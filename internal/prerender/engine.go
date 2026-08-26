package prerender

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"prerender-shield/internal/cache"
	"prerender-shield/internal/prerender/pool"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/seo"
	"prerender-shield/internal/utils"
)

// RedisClient 是 redis.Client 的接口，用于测试 mock
type RedisClient interface {
	SaveJSON(key string, value interface{}, ttl time.Duration) error
	GetJSON(key string, value interface{}) error
	SetAdd(key string, members ...interface{}) error
	SetMembers(key string) ([]string, error)
	Del(key string) error
	Keys(pattern string) ([]string, error)
	SetURLPreheatStatus(site, route, status string, params ...interface{}) error
}

// 确保 redis.Client 实现 RedisClient 接口
var _ RedisClient = (*redis.Client)(nil)

// RenderOptions 渲染选项
type RenderOptions struct {
	Timeout        time.Duration
	WaitUntil      string
	Headers        map[string]string
	Cookies        []Cookie
	Proxy          string
	BlockResources bool
}

// Cookie Cookie结构
type Cookie struct {
	Name     string
	Value    string
	Domain   string
	Path     string
	Expires  time.Time
	Secure   bool
	HttpOnly bool
	SameSite string
}

// RenderResult 渲染结果
type RenderResult struct {
	HTML       string
	Success    bool
	Error      string
	RenderTime float64
	URL        string
}

// RenderWithCacheResult 带缓存的渲染结果
type RenderWithCacheResult struct {
	Result   RenderResult
	HitCache bool
	CacheTTL int
}

// Engine 渲染引擎接口
type Engine interface {
	Render(url string, timeout time.Duration) ([]byte, error)
	CreatePreheatTask(siteID string, urls []string) (string, error)
	GetPreheatTaskStatus(taskID string) (map[string]interface{}, error)
	ListPreheatTasks(siteID string) ([]map[string]interface{}, error)
	CancelPreheatTask(taskID string) error
	CleanupPreheatTasks() error
	IsCrawlerRequest(userAgent string) bool
	RenderWithContext(c *gin.Context, url string, opts RenderOptions, userAgent string) (RenderWithCacheResult, error)
	GetPoolSize() int
	Close() error
}

// engine 渲染引擎实现
type engine struct {
	redisClient        RedisClient
	cacheManager       cache.Manager
	maxConcurrent      int
	concurrencyManager *ConcurrencyManager
	browserPool        *pool.Pool
	ownsPool           bool // 是否拥有浏览器池的生命周期（共享池为 false）
	seoInjector        *SEOInjector
}

// NewEngine 创建新的渲染引擎
func NewEngine(redisClient RedisClient, cacheManager cache.Manager, maxConcurrent int) Engine {
	return NewEngineWithSEO(redisClient, cacheManager, maxConcurrent, nil)
}

// NewEngineWithSEO 创建带SEO配置的渲染引擎（自建私有浏览器池）
func NewEngineWithSEO(redisClient RedisClient, cacheManager cache.Manager, maxConcurrent int, llmConfig *seo.LLMConfig) Engine {
	return newEngineWithPool(redisClient, cacheManager, maxConcurrent, llmConfig, defaultBrowserPool(), true)
}

// NewEngineWithSharedPool 创建使用共享浏览器池的渲染引擎。
// 多站点场景下所有 Engine 复用同一个实例池，避免 N 站点 × MinInstances 的进程爆炸；
// 共享池由 EngineManager 统一创建与关闭，引擎自身不负责其生命周期。
func NewEngineWithSharedPool(redisClient RedisClient, cacheManager cache.Manager, maxConcurrent int, llmConfig *seo.LLMConfig, shared *pool.Pool) Engine {
	return newEngineWithPool(redisClient, cacheManager, maxConcurrent, llmConfig, shared, false)
}

// defaultBrowserPool 按环境变量构建默认浏览器池配置
func defaultBrowserPool() *pool.Pool {
	poolCfg := pool.DefaultConfig()
	if envMax := os.Getenv("PRERENDER_MAX_INSTANCES"); envMax != "" {
		if n, err := strconv.Atoi(envMax); err == nil && n > 0 && n <= 100 {
			poolCfg.MaxInstances = n
		}
	}
	if envMin := os.Getenv("PRERENDER_MIN_INSTANCES"); envMin != "" {
		if n, err := strconv.Atoi(envMin); err == nil && n >= 0 {
			poolCfg.MinInstances = n
		}
	}
	// 浏览器路径: PRERENDER_CHROMIUM_PATH 优先，其次 CHROME_PATH（Docker 镜像内置）
	poolCfg.ExecPath = utils.FirstNonEmpty(
		os.Getenv("PRERENDER_CHROMIUM_PATH"),
		os.Getenv("CHROME_PATH"),
	)
	return pool.NewPool(poolCfg, zap.NewNop())
}

func newEngineWithPool(redisClient RedisClient, cacheManager cache.Manager, maxConcurrent int, llmConfig *seo.LLMConfig, browserPool *pool.Pool, ownsPool bool) Engine {
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}

	// 创建动态并发管理器
	concurrencyManager := NewConcurrencyManager(2, maxConcurrent*2, maxConcurrent)

	// 创建 SEO 注入器
	var seoInjector *SEOInjector
	if llmConfig != nil && llmConfig.Enabled {
		seoInjector = NewSEOInjector(nil, nil, llmConfig)
	} else {
		seoInjector = NewSEOInjector(nil, nil, nil)
	}

	return &engine{
		redisClient:        redisClient,
		cacheManager:       cacheManager,
		maxConcurrent:      maxConcurrent,
		concurrencyManager: concurrencyManager,
		browserPool:        browserPool,
		ownsPool:           ownsPool,
		seoInjector:        seoInjector,
	}
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  500 * time.Millisecond,
		MaxDelay:   5 * time.Second,
	}
}

// Render 渲染页面（带指数退避重试）
func (e *engine) Render(url string, timeout time.Duration) ([]byte, error) {
	return e.renderWithRetry(url, timeout, DefaultRetryConfig())
}

// renderWithRetry 带指数退避重试的渲染
func (e *engine) renderWithRetry(url string, timeout time.Duration, retry RetryConfig) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= retry.MaxRetries; attempt++ {
		html, err := e.renderOnce(url, timeout)
		if err == nil {
			return html, nil
		}

		lastErr = err
		if attempt < retry.MaxRetries {
			delay := retry.BaseDelay * (1 << attempt)
			if delay > retry.MaxDelay {
				delay = retry.MaxDelay
			}
			time.Sleep(delay)
		}
	}

	return nil, fmt.Errorf("render failed after %d retries: %w", retry.MaxRetries, lastErr)
}

// renderOnce 单次渲染
func (e *engine) renderOnce(url string, timeout time.Duration) ([]byte, error) {
	instance, err := e.browserPool.AcquireWithTimeout(timeout)
	if err != nil {
		return nil, fmt.Errorf("acquire browser: %w", err)
	}
	defer e.browserPool.Release(instance)

	ctx, cancel := context.WithTimeout(instance.ChromeCtx, timeout)
	defer cancel()

	var html string
	err = chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.OuterHTML("html", &html),
	)

	if err != nil {
		return nil, fmt.Errorf("render page: %w", err)
	}

	return []byte(html), nil
}

// RenderWithGzip 渲染并返回gzip压缩结果
func (e *engine) RenderWithGzip(url string, timeout time.Duration) ([]byte, error) {
	html, err := e.Render(url, timeout)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(html); err != nil {
		return nil, fmt.Errorf("gzip write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("gzip close: %w", err)
	}

	return buf.Bytes(), nil
}

// IsCrawlerRequest 检测请求是否来自爬虫
func (e *engine) IsCrawlerRequest(userAgent string) bool {
	return isCrawlerUserAgent(userAgent)
}

// RenderWithContext 渲染页面（带 gin.Context 参数的版本）
func (e *engine) RenderWithContext(c *gin.Context, url string, opts RenderOptions, userAgent string) (RenderWithCacheResult, error) {
	// #52: 先检查缓存，避免每次请求都启动 Chrome 渲染
	cacheKey := fmt.Sprintf("prerender:%s", url)
	siteID := ""
	if c != nil {
		siteID = c.GetString("site_id")
	}
	if siteID == "" {
		siteID = "default"
	}

	if cached, err := e.cacheManager.Get(siteID, cacheKey); err == nil && len(cached) > 0 {
		return RenderWithCacheResult{
			Result: RenderResult{
				HTML:    string(cached),
				Success: true,
				URL:     url,
			},
			HitCache: true,
		}, nil
	}

	// 从池获取实例
	instance, err := e.browserPool.AcquireWithTimeout(opts.Timeout)
	if err != nil {
		return RenderWithCacheResult{
			Result: RenderResult{
				HTML:    "",
				Success: false,
				Error:   err.Error(),
			},
			HitCache: false,
		}, fmt.Errorf("failed to acquire browser instance: %w", err)
	}
	defer e.browserPool.Release(instance)

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(instance.ChromeCtx, opts.Timeout)
	defer cancel()

	// #51: 计算渲染时间
	renderStart := time.Now()

	// P0-8: 智能等待 - 使用 Activity Tracker 正确实现网络空闲检测
	// 跟踪活跃请求数，当活跃数 = 0 且持续 idleMs 时 resolve
	// 硬上限 hardCapMs 防止无限等待
	var html string
	err = chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("body"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(smartWaitJS(500, 30000), nil).Do(ctx)
		}),
		chromedp.OuterHTML("html", &html),
	)

	renderTime := time.Since(renderStart).Seconds()

	if err != nil {
		return RenderWithCacheResult{
			Result: RenderResult{
				HTML:       "",
				Success:    false,
				Error:      err.Error(),
				RenderTime: renderTime,
			},
			HitCache: false,
		}, fmt.Errorf("failed to render page: %w", err)
	}

	// 注入 SEO 优化（Meta 标签 / 结构化数据 / canonical）
	// 仅对实时渲染的结果注入；缓存命中时已包含 SEO 标签，无需重复处理
	if e.seoInjector != nil {
		if optimized, seoErr := e.seoInjector.InjectSEOTags([]byte(html), url); seoErr == nil {
			html = string(optimized)
		}
	}

	// 缓存渲染结果
	_ = e.cacheManager.Set(siteID, cacheKey, []byte(html), 24*time.Hour)

	return RenderWithCacheResult{
		Result: RenderResult{
			HTML:       html,
			Success:    true,
			Error:      "",
			RenderTime: renderTime,
			URL:        url,
		},
		HitCache: false,
	}, nil
}

// GetPoolSize 获取浏览器池当前实例数
func (e *engine) GetPoolSize() int {
	if e.browserPool == nil {
		return 0
	}
	stats := e.browserPool.Stats()
	if total, ok := stats["total_instances"].(int); ok {
		return total
	}
	return 0
}

// Close 关闭渲染引擎，释放资源。
// 使用共享池时仅解除引用，池生命周期归 EngineManager 管理
func (e *engine) Close() error {
	if e.browserPool != nil && e.ownsPool {
		return e.browserPool.Close()
	}
	e.browserPool = nil
	return nil
}

// containsIgnoreCase 忽略大小写检查字符串是否包含子串
func containsIgnoreCase(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

// equalFold 忽略大小写比较两个字符串是否相等
func equalFold(s, t string) bool {
	if len(s) != len(t) {
		return false
	}
	for i := 0; i < len(s); i++ {
		a, b := s[i], t[i]
		if a >= 'A' && a <= 'Z' {
			a += 'a' - 'A'
		}
		if b >= 'A' && b <= 'Z' {
			b += 'a' - 'A'
		}
		if a != b {
			return false
		}
	}
	return true
}

// smartWaitJS (P0-8) 返回等待网络空闲的 JS 代码
// 实现原理：
//   - 维护活跃请求计数器 (activeRequests)
//   - 包装 fetch / XMLHttpRequest / WebSocket 跟踪请求
//   - 当 activeRequests === 0 时启动 idle 定时器
//   - 持续 idleMs 无新请求则 resolve
//   - hardCapMs 硬上限防止无限等待
func smartWaitJS(idleMs, hardCapMs int) string {
	return fmt.Sprintf(`
(function() {
	return new Promise(function(resolve) {
		var activeRequests = 0;
		var idleTimer = null;
		var hardTimer = null;
		var resolved = false;

		function tryResolve() {
			if (resolved) return;
			if (activeRequests === 0) {
				if (idleTimer) clearTimeout(idleTimer);
				idleTimer = setTimeout(function() {
					resolved = true;
					resolve();
				}, %d);
			} else if (idleTimer) {
				clearTimeout(idleTimer);
				idleTimer = null;
			}
		}

		// 硬上限
		hardTimer = setTimeout(function() {
			resolved = true;
			resolve();
		}, %d);

		// 包装 fetch
		if (window.fetch) {
			var origFetch = window.fetch;
			window.fetch = function() {
				activeRequests++;
				return origFetch.apply(this, arguments).finally(function() {
					activeRequests--;
					tryResolve();
				});
			};
		}

		// 包装 XMLHttpRequest
		var origXHROpen = XMLHttpRequest.prototype.open;
		var origXHRSend = XMLHttpRequest.prototype.send;
		XMLHttpRequest.prototype.open = function() {
			this.__tracked = true;
			return origXHROpen.apply(this, arguments);
		};
		XMLHttpRequest.prototype.send = function() {
			if (this.__tracked) {
				activeRequests++;
				this.addEventListener('loadend', function() {
					activeRequests--;
					tryResolve();
				});
			}
			return origXHRSend.apply(this, arguments);
		};

		// 包装 WebSocket (send 调用视为请求)
		if (window.WebSocket) {
			var OrigWS = window.WebSocket;
			window.WebSocket = function(url, protocols) {
				var ws = protocols !== undefined ? new OrigWS(url, protocols) : new OrigWS(url);
				var origWSSend = ws.send;
				ws.send = function() {
					activeRequests++;
					try { ws.addEventListener('close', function() { activeRequests--; tryResolve(); }); } catch(e) {}
					try { ws.addEventListener('error', function() { activeRequests--; tryResolve(); }); } catch(e) {}
					return origWSSend.apply(this, arguments);
				};
				return ws;
			};
			window.WebSocket.prototype = OrigWS.prototype;
		}

		// 初始检测 (DOMContentLoaded 时可能已有请求)
		tryResolve();
	});
})()
	`, idleMs, hardCapMs)
}
