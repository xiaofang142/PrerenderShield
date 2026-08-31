package prerender

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"prerender-shield/internal/cache"
	"prerender-shield/internal/config"
	"prerender-shield/internal/prerender/pool"
	"prerender-shield/internal/prerender/renderkey"
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
	// CacheTTL 渲染结果的业务有效期（秒）；0 表示使用引擎默认 24h。
	// 接线自 site.Prerender.CacheTTL（历史版本此配置未接线，写入端硬编码 24h）。
	CacheTTL int
	// MaxConcurrency 站点级渲染并发预算（同站点同时在渲的页面上限），0 表示默认。
	MaxConcurrency int
}

// RenderRequest 统一渲染管线的入参（实时路径与预热路径共用）
type RenderRequest struct {
	SiteID    string
	URL       string
	Opts      RenderOptions
	UserAgent string
}

// ErrSiteBusy 站点并发预算耗尽（避免单站饿死全局浏览器池）
var ErrSiteBusy = errors.New("site concurrency budget exhausted")

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
	// Status 主文档真实状态码（含 render:status_code 覆盖后的最终值）
	Status int
	// Thin 空壳质检未通过（已注入 noindex，且未写入缓存）
	Thin bool
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
	// RenderAndCache 统一渲染管线：状态码捕获+质量门+SEO注入+信封缓存（预热与实时共用）
	RenderAndCache(req RenderRequest) (RenderResult, error)
	// GetCachedPage 读取任意年龄的信封（新鲜度由调用方用 env.Fresh 判断）
	GetCachedPage(siteID, url, userAgent string) (PageEnvelope, bool)
	// InvalidatePage 删除单 URL 缓存
	InvalidatePage(siteID, url string) error
	// ListCacheEntries 列出站点渲染缓存条目摘要（管理端缓存条目列表用）
	ListCacheEntries(siteID string, limit int) ([]cache.CacheEntrySummary, error)
	// SetDefaultCacheTTL 设置站点级默认业务 TTL（秒），opts.CacheTTL 未传时的兜底。
	SetDefaultCacheTTL(seconds int)
	// SetPreheatTTLConfig 设置预热通道的站点 TTL 与分级规则（预热任务创建方注入）
	SetPreheatTTLConfig(siteTTL int, rules []config.TTLRule)
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

	// siteBudgets 站点级渲染并发预算（站点ID -> 信号量 chan）
	siteBudgets sync.Map

	// defaultCacheTTL 站点默认业务 TTL（秒）；站点配置 CacheTTL 接线（预热通道用）
	defaultCacheTTL int64

	// preheatTTL 站点级 CacheTTL 与分级规则（预热任务创建方注入，preheatTTLRulesMu 保护）
	preheatSiteTTL    int
	preheatTTLRules   []config.TTLRule
	preheatTTLRulesMu sync.RWMutex

	// preheatBaseURL 站点公开基址（scheme://host），用于把 sitemap/crawler 来源的
	// route 形态 URL 补全为绝对地址（修复死键/渲染失败）
	preheatBaseURL string
	preheatBaseMu  sync.RWMutex
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

// RenderWithContext 渲染页面（带 gin.Context 参数的版本）。
// 语义：命中缓存直接回；未命中走统一管线渲染并写缓存。降级供数（软过期兜底）
// 由上层 handler 编排，此方法保持旧行为供既有调用方使用。
func (e *engine) RenderWithContext(c *gin.Context, url string, opts RenderOptions, userAgent string) (RenderWithCacheResult, error) {
	siteID := "default"
	if c != nil {
		siteID = c.GetString("site_id")
		if siteID == "" {
			siteID = "default"
		}
	}

	// 单桶读（设备分桶收敛，见 RenderAndCache 写入注释）+ 存量回退链：
	// @desktop → 无后缀旧键 → 存量 @mobile（一次性过渡，@mobile 键随 TTL 自然过期）
	if raw, err := e.cacheManager.Get(siteID, e.cacheKey(url, "desktop")); err == nil && len(raw) > 0 {
		env, _ := unmarshalPageEnvelope(raw)
		return RenderWithCacheResult{
			Result: RenderResult{
				HTML:    env.HTML,
				Success: true,
				URL:     url,
			},
			HitCache: true,
		}, nil
	}
	for _, fallbackKey := range []string{e.legacyCacheKey(url), e.cacheKey(url, "mobile")} {
		if raw, err := e.cacheManager.Get(siteID, fallbackKey); err == nil && len(raw) > 0 {
			env, _ := unmarshalPageEnvelope(raw)
			return RenderWithCacheResult{
				Result: RenderResult{
					HTML:    env.HTML,
					Success: true,
					URL:     url,
				},
				HitCache: true,
			}, nil
		}
	}

	req := RenderRequest{SiteID: siteID, URL: url, Opts: opts, UserAgent: userAgent}
	res, err := e.RenderAndCache(req)
	if err != nil {
		return RenderWithCacheResult{
			Result: RenderResult{
				HTML:       res.HTML,
				Success:    false,
				Error:      res.Error,
				RenderTime: res.RenderTime,
				URL:        url,
			},
			HitCache: false,
		}, err
	}
	return RenderWithCacheResult{Result: res, HitCache: false}, nil
}

// RenderAndCache 统一渲染管线（实时与预热共用唯一写入路径）：
// 站点并发预算 → Chromium 渲染（网络事件捕获主文档状态码 + smartWaitJS 智能等待）
// → render:status_code 自声明覆盖 → SEO 注入 → 空壳质检（最多重试1次）
// → 信封缓存写入（仅 Success 且状态<500 且非空壳；TTL 接线 site 配置，存储期含降级窗口）。
func (e *engine) RenderAndCache(req RenderRequest) (RenderResult, error) {
	budget := e.budgetFor(req.SiteID, req.Opts.MaxConcurrency)
	if budget != nil {
		select {
		case budget <- struct{}{}:
			defer func() { <-budget }()
		default:
			return RenderResult{Success: false, Error: ErrSiteBusy.Error(), URL: req.URL}, ErrSiteBusy
		}
	}

	timeout := req.Opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	start := time.Now()
	const maxAttempts = 2
	html := ""
	docStatus := 200
	thin := false

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		h, st, err := e.chromedpRender(req.URL, timeout)
		if err != nil {
			if attempt == 1 && isEmptyShell(h) {
				// 导航成功但产出空壳且报错（如等待超时），再试一次
				continue
			}
			return RenderResult{
				Success:    false,
				Error:      fmt.Sprintf("failed to render page: %v", err),
				RenderTime: time.Since(start).Seconds(),
				URL:        req.URL,
			}, fmt.Errorf("failed to render page: %w", err)
		}
		html, docStatus = h, st
		if !isEmptyShell(html) {
			break
		}
		if attempt == maxAttempts {
			thin = true
		}
	}
	renderTime := time.Since(start).Seconds()

	// 主文档真实状态码优先；render:status_code 页面自声明可覆盖（3xx/4xx/5xx 才有意义）
	if docStatus < 200 || docStatus > 599 {
		docStatus = 200
	}
	if decl := extractDeclaredStatus(html); decl >= 300 && decl != docStatus {
		docStatus = decl
	}

	// SEO 注入仅在最终产物上进行一次
	if e.seoInjector != nil {
		if optimized, seoErr := e.seoInjector.InjectSEOTags([]byte(html), req.URL); seoErr == nil {
			html = string(optimized)
		}
	}

	// 空壳终态：注入 noindex 后照常响应，但绝不入缓存（坏缓存比无缓存更毒）
	if thin {
		html = injectNoindexMeta(html)
	}

	// R12-BUG-5 修复：缓存只收 2xx。此前 docStatus<500 即缓存——WAF 拦截的 403 页、
	// 上游 404/403 页面渲染结果会以完整 HTML 结构通过空壳质量门进入缓存，导致后续
	// 真实爬虫持续命中"被拦截页"的投毒缓存（实测：403 响应被缓存后爬虫回放）。
	cacheable := !thin && docStatus >= 200 && docStatus < 300
	if cacheable {
		ttlSecs := req.Opts.CacheTTL
		if ttlSecs <= 0 && e.defaultCacheTTL > 0 {
			ttlSecs = int(e.defaultCacheTTL)
		}
		ttl := time.Duration(ttlSecs) * time.Second
		if ttl <= 0 {
			ttl = 24 * time.Hour
		}
		env := PageEnvelope{Status: docStatus, HTML: html, ExpiresAt: time.Now().Add(ttl).Unix(), CreatedAt: time.Now().Unix()}
		// 单桶写入：当前渲染器输出与 UA 无关（无移动视口仿真），设备分桶只会产出
		// 逐字节相同的副本并使 mobile 爬虫永不命中预热缓存（调研：响应式站发桌面
		// HTML 为 Google 官方推荐）。DeviceBucket 保留供未来自适应渲染使用。
		_ = e.cacheManager.Set(req.SiteID, e.cacheKey(req.URL, "desktop"), marshalPageEnvelope(env), staleRetention(ttl))
	}

	return RenderResult{
		HTML:       html,
		Success:    true,
		RenderTime: renderTime,
		URL:        req.URL,
		Status:     docStatus,
		Thin:       thin,
	}, nil
}

// chromedpRender 单次浏览器渲染，返回 HTML 与主文档 HTTP 状态码。
func (e *engine) chromedpRender(target string, timeout time.Duration) (string, int, error) {
	instance, err := e.browserPool.AcquireWithTimeout(timeout)
	if err != nil {
		return "", 0, fmt.Errorf("acquire browser: %w", err)
	}
	defer e.browserPool.Release(instance)

	ctx, cancel := context.WithTimeout(instance.ChromeCtx, timeout)
	defer cancel()

	var html string
	var docStatus int64
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if rs, ok := ev.(*network.EventResponseReceived); ok &&
			rs.Type == network.ResourceTypeDocument && rs.Response.Status >= 200 {
			if docStatus == 0 || int(rs.Response.Status) != int(docStatus) {
				docStatus = rs.Response.Status
			}
		}
	})
	err = chromedp.Run(ctx,
		network.Enable(),
		chromedp.ActionFunc(func(c context.Context) error {
			// R12-BUG-5 修复②：渲染回环导航标记为内部请求，WafMiddleware 对其放行，
			// 否则 WAF 开启的站点中渲染器抓自家页面会被自家 403 页污染（并进一步写缓存）。
			return network.SetExtraHTTPHeaders(map[string]interface{}{
				"X-Prerender-Internal": "1",
			}).Do(c)
		}),
		chromedp.Navigate(target),
		chromedp.WaitVisible("body"),
		chromedp.ActionFunc(func(c context.Context) error {
			return chromedp.Evaluate(smartWaitJS(500, 30000), nil).Do(c)
		}),
		chromedp.OuterHTML("html", &html),
	)
	if err != nil {
		// 渲染失败即标记不健康：Release 走回收路径，污染实例绝不回池复用
		// （防 chromedp 分配超时后同 context 二次 Allocate 的上游 double-close panic）
		instance.MarkUnhealthy()
		return html, int(docStatus), err
	}
	if docStatus == 0 {
		docStatus = 200
	}
	return html, int(docStatus), nil
}

// budgetFor 获取站点并发预算信号量；上限<=0 表示不限制（返回 nil），沿用全局池自约束。
func (e *engine) budgetFor(siteID string, maxConcurrency int) chan struct{} {
	size := maxConcurrency
	if size <= 0 {
		return nil
	}
	if b, ok := e.siteBudgets.Load(siteID); ok {
		return b.(chan struct{})
	}
	b := make(chan struct{}, size)
	actual, loaded := e.siteBudgets.LoadOrStore(siteID, b)
	if loaded {
		return actual.(chan struct{})
	}
	return b
}

// cacheKey 归一化后的渲染缓存业务键（含设备分桶后缀 @mobile/@desktop）
func (e *engine) cacheKey(u, device string) string {
	return renderkey.WithDeviceBucket(renderkey.Normalize(u), device)
}

// legacyCacheKey 存量无分桶后缀旧键（设备分桶上线前的键形态，读侧一次性回退兼容）
func (e *engine) legacyCacheKey(u string) string {
	return renderkey.BuildCacheKey(renderkey.Normalize(u))
}

// GetCachedPage 读取任意年龄的信封；不存在返回 ok=false。
// 单桶读 + 存量回退链：@desktop → 无后缀旧键 → 存量 @mobile（一次性过渡）。
func (e *engine) GetCachedPage(siteID, u, userAgent string) (PageEnvelope, bool) {
	for _, key := range []string{e.cacheKey(u, "desktop"), e.legacyCacheKey(u), e.cacheKey(u, "mobile")} {
		raw, err := e.cacheManager.Get(siteID, key)
		if err != nil || len(raw) == 0 {
			continue
		}
		env, _ := unmarshalPageEnvelope(raw)
		return env, true
	}
	return PageEnvelope{}, false
}

// InvalidatePage 删除单 URL 的渲染缓存（覆盖 mobile/desktop 两分桶及存量无后缀旧键，
// 失效语义=全设备失效，避免管理端只删到一半设备桶造成脏读）
func (e *engine) InvalidatePage(siteID, u string) error {
	// NormalizeFlexible：管理端回传归一化展示形态（host:port/path）时
	// Normalize 会把 host 误判为 scheme 删除错键，故按形态分流
	norm := renderkey.NormalizeFlexible(u)
	var firstErr error
	for _, key := range []string{
		renderkey.WithDeviceBucket(norm, "desktop"),
		renderkey.WithDeviceBucket(norm, "mobile"),
		e.legacyCacheKey(u),
	} {
		if err := e.cacheManager.Delete(siteID, key); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// ListCacheEntries 列出站点渲染缓存条目摘要（委托 cacheManager）
func (e *engine) ListCacheEntries(siteID string, limit int) ([]cache.CacheEntrySummary, error) {
	return e.cacheManager.ListEntries(siteID, limit)
}

// SetDefaultCacheTTL 设置站点默认业务 TTL（秒）
func (e *engine) SetDefaultCacheTTL(seconds int) {
	if seconds > 0 {
		e.defaultCacheTTL = int64(seconds)
	}
}

// SetPreheatTTLConfig 设置预热通道站点 TTL 与分级规则
func (e *engine) SetPreheatTTLConfig(siteTTL int, rules []config.TTLRule) {
	e.preheatTTLRulesMu.Lock()
	defer e.preheatTTLRulesMu.Unlock()
	e.preheatSiteTTL = siteTTL
	e.preheatTTLRules = rules
}

// preheatEffectiveTTL 预热 URL 有效 TTL：规则首中 > 站点 TTL > defaultCacheTTL（0 时引擎再兜底 24h）
func (e *engine) preheatEffectiveTTL(rawURL string) int {
	e.preheatTTLRulesMu.RLock()
	defer e.preheatTTLRulesMu.RUnlock()
	for _, rule := range e.preheatTTLRules {
		if rule.Matches(rawURL) {
			return rule.TTLSeconds
		}
	}
	if e.preheatSiteTTL > 0 {
		return e.preheatSiteTTL
	}
	return int(e.defaultCacheTTL)
}

// SetPreheatBaseURL 设置站点公开基址，供预热任务把相对路径补全为绝对 URL
func (e *engine) SetPreheatBaseURL(base string) {
	e.preheatBaseMu.Lock()
	e.preheatBaseURL = base
	e.preheatBaseMu.Unlock()
}

func (e *engine) getPreheatBaseURL() string {
	e.preheatBaseMu.RLock()
	defer e.preheatBaseMu.RUnlock()
	return e.preheatBaseURL
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
