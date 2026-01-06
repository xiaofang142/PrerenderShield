package prerender

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"prerender-shield/internal/logging"
	"prerender-shield/internal/redis"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/google/uuid"
)

// Engine 渲染预热引擎
type Engine struct {
	SiteName           string
	staticDir          string // 静态文件目录
	config             PrerenderConfig
	browserPool        []*Browser
	idleBrowsers       chan *Browser
	taskQueue          chan *RenderTask
	isRunning          bool
	mutex              sync.RWMutex
	preheatManager     *PreheatManager
	workerWg           sync.WaitGroup
	ctx                context.Context
	cancel             context.CancelFunc
	healthCheckTicker  *time.Ticker
	queueLengthHistory []int        // 任务队列长度历史，用于动态扩容决策
	queueMutex         sync.RWMutex // 队列长度历史互斥锁
	activeTasks        int          // 当前活跃任务数
	taskMutex          sync.RWMutex // 活跃任务数互斥锁
	redisClient        *redis.Client
	// 默认爬虫协议头列表
	defaultCrawlerHeaders []string
}

// EngineManager 渲染预热引擎管理器，管理多个站点的渲染预热引擎
type EngineManager struct {
	mutex             sync.RWMutex
	engines           map[string]*Engine // 站点名 -> 引擎实例
	ctx               context.Context
	cancel            context.CancelFunc
	autoPreheatTicker *time.Ticker // 自动预热检查定时器
	staticDir         string       // 静态文件目录
}

// Browser 浏览器实例
type Browser struct {
	ID         string
	Status     string
	LastUsed   time.Time
	Healthy    bool
	ErrorCount int
	CreatedAt  time.Time
	Instance   *rod.Browser // 实际的浏览器实例
}

// RenderTask 渲染任务
type RenderTask struct {
	ID      string
	URL     string
	Options RenderOptions
	Result  chan *RenderResult
}

// RenderOptions 渲染选项
type RenderOptions struct {
	Timeout   int
	WaitUntil string
}

// RenderResult 渲染结果
type RenderResult struct {
	HTML    string
	Success bool
	Error   string
}

// PrerenderConfig 渲染预热配置
type PrerenderConfig struct {
	Enabled           bool
	PoolSize          int // 初始浏览器池大小
	MinPoolSize       int // 最小浏览器池大小
	MaxPoolSize       int // 最大浏览器池大小
	Timeout           int
	CacheTTL          int
	Preheat           PreheatConfig
	CrawlerHeaders    []string // 爬虫协议头列表
	UseDefaultHeaders bool     // 是否使用默认爬虫协议头
}

// PreheatConfig 缓存预热配置
type PreheatConfig struct {
	Enabled  bool
	MaxDepth int
}

// PreheatManager 缓存预热管理器
type PreheatManager struct {
	config        PrerenderConfig
	engine        *Engine
	redisClient   *redis.Client
	crawler       *Crawler
	preheatWorker *PreheatWorker
	isRunning     bool
	mutex         sync.Mutex
}

// NewPreheatManager 创建新的预热管理器
func NewPreheatManager(engine *Engine, redisClient *redis.Client) *PreheatManager {
	return &PreheatManager{
		config:      engine.config,
		engine:      engine,
		redisClient: redisClient,
		isRunning:   false,
	}
}

// TriggerPreheat 触发缓存预热，默认使用localhost:8081
func (pm *PreheatManager) TriggerPreheat() (string, error) {
	// 默认使用localhost:8081，兼容旧版API
	return pm.TriggerPreheatWithURL("http://localhost:8081", "localhost:8081")
}

// TriggerPreheatWithURL 触发缓存预热，支持自定义baseURL和Domain
func (pm *PreheatManager) TriggerPreheatWithURL(baseURL, domain string) (string, error) {
	pm.mutex.Lock()
	if pm.isRunning {
		pm.mutex.Unlock()
		return "", fmt.Errorf("preheat is already running")
	}
	pm.isRunning = true
	pm.mutex.Unlock()

	// 检查Redis客户端是否可用
	if pm.redisClient == nil {
		pm.mutex.Lock()
		pm.isRunning = false
		pm.mutex.Unlock()
		return "", fmt.Errorf("redis client is not available, preheat cannot be triggered")
	}

	// 创建预热任务
	taskID, err := pm.redisClient.CreatePreheatTask(pm.engine.SiteName)
	if err != nil {
		pm.mutex.Lock()
		pm.isRunning = false
		pm.mutex.Unlock()
		return "", fmt.Errorf("failed to create preheat task: %v", err)
	}

	// 异步执行预热流程，包括爬虫和渲染
	go func() {
		defer func() {
			pm.mutex.Lock()
			pm.isRunning = false
			pm.mutex.Unlock()
		}()

		// 1. 首先爬取站点的所有链接
		logging.DefaultLogger.Info("Starting URL crawler for site: %s with baseURL: %s", pm.engine.SiteName, baseURL)

		// 创建爬虫配置
		crawlerConfig := CrawlerConfig{
			SiteName:    pm.engine.SiteName,
			Domain:      domain,
			BaseURL:     baseURL,
			MaxDepth:    pm.config.Preheat.MaxDepth,
			Concurrency: 3, // 降低爬虫并发度，减少资源消耗
			RedisClient: pm.redisClient,
		}

		// 创建爬虫实例
		crawler := NewCrawler(crawlerConfig)

		// 同步执行爬虫，确保先完成URL爬取
		if err := crawler.Start(); err != nil {
			pm.redisClient.SetPreheatTaskStatus(pm.engine.SiteName, taskID, "failed")
			logging.DefaultLogger.Error("Failed to crawl URLs: %v", err)
			return
		}

		// 2. 获取所有URL后执行预热
		urls, err := pm.redisClient.GetURLs(pm.engine.SiteName)
		if err != nil {
			pm.redisClient.SetPreheatTaskStatus(pm.engine.SiteName, taskID, "failed")
			logging.DefaultLogger.Error("Failed to get URLs for preheat: %v", err)
			return
		}

		// 防御性编程：限制最大URL数量，防止资源耗尽
		const MaxPreheatURLs = 1000
		if len(urls) > MaxPreheatURLs {
			logging.DefaultLogger.Warn("Too many URLs to preheat, limiting to %d (total: %d)", MaxPreheatURLs, len(urls))
			urls = urls[:MaxPreheatURLs]
		}

		// 更新任务的总URL数
		totalURLs := int64(len(urls))
		pm.redisClient.UpdatePreheatTaskProgress(pm.engine.SiteName, taskID, totalURLs, 0, 0, 0)

		// 初始化进度统计
		var (
			success     int64 = 0
			failed      int64 = 0
			processed   int64 = 0
			progressMux sync.Mutex
		)

		// 获取浏览器池大小，基于池大小动态调整并发度
		maxConcurrency := pm.engine.config.PoolSize
		if maxConcurrency < 1 {
			maxConcurrency = 1
		}
		if maxConcurrency > 10 {
			maxConcurrency = 10 // 限制最大并发度，防止资源耗尽
		}

		// 创建并发控制信号量
		semaphore := make(chan struct{}, maxConcurrency)
		var wg sync.WaitGroup

		// 并发执行渲染预热
		for _, url := range urls {
			// 检查预热是否已被停止
			pm.mutex.Lock()
			if !pm.isRunning {
				pm.mutex.Unlock()
				break
			}
			pm.mutex.Unlock()

			wg.Add(1)
			semaphore <- struct{}{}

			go func(url string) {
				defer func() {
					wg.Done()
					<-semaphore

					// 更新进度
					progressMux.Lock()
					processed++
					pm.redisClient.UpdatePreheatTaskProgress(pm.engine.SiteName, taskID, totalURLs, processed, success, failed)
					progressMux.Unlock()
				}()

				// 使用渲染引擎进行真正的缓存预热
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second) // 缩短超时时间
				defer cancel()

				logging.DefaultLogger.Debug("Starting preheat for URL: %s", url)

				// 调用引擎的Render方法，这将自动缓存渲染结果
				resultWithCache, err := pm.engine.Render(ctx, url, RenderOptions{
					Timeout:   20,
					WaitUntil: "networkidle0",
				})

				if err != nil {
					logging.DefaultLogger.Error("Preheat failed for URL %s: %v", url, err)
					progressMux.Lock()
					failed++
					progressMux.Unlock()
					// 更新URL状态为failed
					pm.redisClient.SetURLPreheatStatus(pm.engine.SiteName, url, "failed", 0)
					return
				}

				if !resultWithCache.Result.Success {
					logging.DefaultLogger.Error("Render failed for URL %s: %s", url, resultWithCache.Result.Error)
					progressMux.Lock()
					failed++
					progressMux.Unlock()
					// 更新URL状态为failed
					pm.redisClient.SetURLPreheatStatus(pm.engine.SiteName, url, "failed", 0)
					return
				}

				// 渲染成功，更新成功计数和URL状态
				logging.DefaultLogger.Debug("Successfully preheated URL: %s", url)
				progressMux.Lock()
				success++
				progressMux.Unlock()
				// 更新URL状态为cached
				cacheSize := int64(len(resultWithCache.Result.HTML))
				pm.redisClient.SetURLPreheatStatus(pm.engine.SiteName, url, "cached", cacheSize)
			}(url)
		}

		// 等待所有预热任务完成
		wg.Wait()

		// 更新统计数据
		pm.updateStats()

		// 标记任务完成
		pm.redisClient.SetPreheatTaskStatus(pm.engine.SiteName, taskID, "completed")
		logging.DefaultLogger.Info("Preheat completed for site: %s", pm.engine.SiteName)
		logging.DefaultLogger.Info("Preheat summary: total=%d, success=%d, failed=%d", totalURLs, success, failed)
	}()

	return taskID, nil
}

// updateStats 更新站点统计数据
func (pm *PreheatManager) updateStats() error {
	// 检查Redis客户端是否可用
	if pm.redisClient == nil {
		return fmt.Errorf("redis client is not available, cannot update stats")
	}

	// 获取URL数量
	urlCount, err := pm.redisClient.GetURLCount(pm.engine.SiteName)
	if err != nil {
		return err
	}

	// 获取缓存数量
	cacheCount, err := pm.redisClient.GetCacheCount(pm.engine.SiteName)
	if err != nil {
		return err
	}

	// 计算总缓存大小（简化实现，实际可能需要遍历所有URL的缓存大小）
	// 这里使用缓存数量 * 平均缓存大小来估算
	totalCacheSize := cacheCount * 1024 * 1024 // 假设平均每个缓存1MB

	// 更新统计数据
	stats := map[string]interface{}{
		"url_count":         urlCount,
		"cache_count":       cacheCount,
		"total_cache_size":  totalCacheSize,
		"last_preheat_time": time.Now().Unix(),
	}

	return pm.redisClient.SetSiteStats(pm.engine.SiteName, stats)
}

// TriggerPreheatForURL 触发单个URL的预热
func (pm *PreheatManager) TriggerPreheatForURL(url string) error {
	// 检查Redis客户端是否可用
	if pm.redisClient == nil {
		return fmt.Errorf("redis client is not available, cannot preheat URL")
	}

	// 创建上下文，设置30秒超时
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logging.DefaultLogger.Info("Starting preheat for single URL: %s", url)

	// 调用引擎的Render方法，这将自动缓存渲染结果
	resultWithCache, err := pm.engine.Render(ctx, url, RenderOptions{
		Timeout:   30,
		WaitUntil: "networkidle0",
	})

	if err != nil {
		logging.DefaultLogger.Error("Preheat failed for URL %s: %v", url, err)
		// 更新URL状态为failed
		pm.redisClient.SetURLPreheatStatus(pm.engine.SiteName, url, "failed", 0)
		return err
	}

	if !resultWithCache.Result.Success {
		logging.DefaultLogger.Error("Render failed for URL %s: %s", url, resultWithCache.Result.Error)
		// 更新URL状态为failed
		pm.redisClient.SetURLPreheatStatus(pm.engine.SiteName, url, "failed", 0)
		return fmt.Errorf("render failed: %s", resultWithCache.Result.Error)
	}

	// 渲染成功，更新URL状态为cached
	cacheSize := int64(len(resultWithCache.Result.HTML))
	pm.redisClient.SetURLPreheatStatus(pm.engine.SiteName, url, "cached", cacheSize)

	logging.DefaultLogger.Info("Successfully preheated URL: %s (size: %d bytes)", url, cacheSize)
	return nil
}

// GetStats 获取站点预热统计数据
func (pm *PreheatManager) GetStats() (map[string]string, error) {
	// 检查Redis客户端是否可用
	if pm.redisClient == nil {
		// 返回空统计数据而不是报错，避免前端崩溃
		return map[string]string{
			"url_count":        "0",
			"cache_count":      "0",
			"total_cache_size": "0",
		}, nil
	}

	return pm.redisClient.GetSiteStats(pm.engine.SiteName)
}

// IsRunning 检查预热是否正在运行
func (pm *PreheatManager) IsRunning() bool {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	return pm.isRunning
}

// GetStatus 获取预热状态
func (pm *PreheatManager) GetStatus() map[string]interface{} {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	return map[string]interface{}{
		"isRunning": pm.isRunning,
	}
}

// Stop 停止预热
func (pm *PreheatManager) Stop() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if pm.crawler != nil {
		pm.crawler.Stop()
	}

	if pm.preheatWorker != nil {
		pm.preheatWorker.Stop()
	}

	pm.isRunning = false
}

// NewEngine 创建新的渲染预热引擎
func NewEngine(siteName string, config PrerenderConfig, redisClient *redis.Client, staticDir string) (*Engine, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// 设置默认值
	if config.MinPoolSize == 0 {
		config.MinPoolSize = 2 // 默认最小浏览器数
	}
	if config.MaxPoolSize == 0 {
		config.MaxPoolSize = config.PoolSize * 2 // 默认最大浏览器数为初始值的2倍
	}

	// 默认爬虫协议头列表
	defaultCrawlerHeaders := []string{
		"Googlebot",
		"Bingbot",
		"Slurp",
		"DuckDuckBot",
		"Baiduspider",
		"Sogou spider",
		"YandexBot",
		"Exabot",
		"FacebookBot",
		"Twitterbot",
		"LinkedInBot",
		"WhatsAppBot",
		"TelegramBot",
		"DiscordBot",
		"PinterestBot",
		"InstagramBot",
		"Google-InspectionTool",
		"Google-Site-Verification",
		"AhrefsBot",
		"SEMrushBot",
		"Majestic",
		"Yahoo! Slurp",
	}

	engine := &Engine{
		SiteName:              siteName,
		staticDir:             staticDir,
		config:                config,
		taskQueue:             make(chan *RenderTask, 100),
		idleBrowsers:          make(chan *Browser, config.MaxPoolSize),
		isRunning:             false,
		ctx:                   ctx,
		cancel:                cancel,
		queueLengthHistory:    make([]int, 0, 10), // 保存最近10次的队列长度
		activeTasks:           0,
		defaultCrawlerHeaders: defaultCrawlerHeaders,
		redisClient:           redisClient,
	}

	return engine, nil
}

// NewEngineManager 创建渲染预热引擎管理器
func NewEngineManager(staticDir string) *EngineManager {
	ctx, cancel := context.WithCancel(context.Background())
	manager := &EngineManager{
		engines:   make(map[string]*Engine),
		mutex:     sync.RWMutex{},
		ctx:       ctx,
		cancel:    cancel,
		staticDir: staticDir,
	}
	// Start the auto-preheating daemon
	manager.startAutoPreheating()
	return manager
}

// AddSite 添加新站点
func (em *EngineManager) AddSite(siteName string, config PrerenderConfig, redisClient *redis.Client) error {
	em.mutex.Lock()
	defer em.mutex.Unlock()

	// 检查站点是否已存在
	if _, exists := em.engines[siteName]; exists {
		return fmt.Errorf("site %s already exists", siteName)
	}

	// 创建新引擎实例
	engine, err := NewEngine(siteName, config, redisClient, em.staticDir)
	if err != nil {
		return err
	}

	// 启动引擎
	if err := engine.Start(); err != nil {
		return err
	}

	em.engines[siteName] = engine
	return nil
}

// RemoveSite 移除站点
func (em *EngineManager) RemoveSite(siteName string) error {
	em.mutex.Lock()
	defer em.mutex.Unlock()

	engine, exists := em.engines[siteName]
	if !exists {
		return fmt.Errorf("site %s not found", siteName)
	}

	// 停止引擎
	engine.Stop()

	// 移除引擎
	delete(em.engines, siteName)
	return nil
}

// GetEngine 获取指定站点的引擎实例
func (em *EngineManager) GetEngine(siteName string) (*Engine, bool) {
	em.mutex.RLock()
	defer em.mutex.RUnlock()

	engine, exists := em.engines[siteName]
	return engine, exists
}

// ListSites 列出所有站点
func (em *EngineManager) ListSites() []string {
	em.mutex.RLock()
	defer em.mutex.RUnlock()

	var sites []string
	for site := range em.engines {
		sites = append(sites, site)
	}
	return sites
}

// StartAll 启动所有站点的引擎
func (em *EngineManager) StartAll() error {
	em.mutex.RLock()
	engines := make(map[string]*Engine, len(em.engines))
	for k, v := range em.engines {
		engines[k] = v
	}
	em.mutex.RUnlock()

	for siteName, engine := range engines {
		if err := engine.Start(); err != nil {
			return fmt.Errorf("failed to start engine for site %s: %v", siteName, err)
		}
	}

	return nil
}

// StopAll 停止所有站点的引擎
func (em *EngineManager) StopAll() error {
	em.mutex.RLock()
	engines := make(map[string]*Engine, len(em.engines))
	for k, v := range em.engines {
		engines[k] = v
	}
	em.mutex.RUnlock()

	for _, engine := range engines {
		engine.Stop()
	}

	// Stop the auto-preheating daemon
	if em.autoPreheatTicker != nil {
		em.autoPreheatTicker.Stop()
	}

	em.cancel()
	return nil
}

// startAutoPreheating 启动自动预热守护进程
func (em *EngineManager) startAutoPreheating() {
	// Run every minute
	em.autoPreheatTicker = time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {
			case <-em.autoPreheatTicker.C:
				em.checkAutoPreheat()
			case <-em.ctx.Done():
				return
			}
		}
	}()
}

// checkAutoPreheat 检查所有站点的自动预热配置
func (em *EngineManager) checkAutoPreheat() {
	em.mutex.RLock()
	engines := make(map[string]*Engine, len(em.engines))
	for k, v := range em.engines {
		engines[k] = v
	}
	em.mutex.RUnlock()

	for siteName, engine := range engines {
		// Check if auto-preheating is enabled for this site
		if !engine.config.Preheat.Enabled {
			continue
		}

		// Check all URLs for this site
		urls, err := engine.redisClient.GetURLs(siteName)
		if err != nil {
			continue
		}

		for _, url := range urls {
			// Check if cache is about to expire or already expired
			if em.shouldPreheatURL(engine, url) {
				// Trigger preheating for this URL
				go func(url string) {
					if err := engine.preheatManager.TriggerPreheatForURL(url); err != nil {
						// Log error but continue with other URLs
						logging.DefaultLogger.Error("Auto-preheat failed for URL %s: %v", url, err)
					}
				}(url)
			}
		}
	}
}

// shouldPreheatURL 检查URL是否需要预热
func (em *EngineManager) shouldPreheatURL(engine *Engine, url string) bool {
	// Get cache TTL from configuration
	cacheTTL := engine.config.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = 3600 // Default to 1 hour
	}

	// Check cache status
	status, err := engine.redisClient.GetURLPreheatStatus(engine.SiteName, url)
	if err != nil {
		// If we can't get the status, assume it needs preheating
		return true
	}

	// Get last updated time
	updatedAtStr, exists := status["updated_at"]
	if !exists {
		// If no updated time, assume it needs preheating
		return true
	}

	updatedAt, err := strconv.ParseInt(updatedAtStr, 10, 64)
	if err != nil {
		// If invalid updated time, assume it needs preheating
		return true
	}

	// Calculate time elapsed since last update
	elapsed := time.Now().Unix() - updatedAt
	// Check if cache is within 10 minutes of expiration or already expired
	return elapsed >= int64(cacheTTL)-600 || elapsed >= int64(cacheTTL)
}

// Start 启动渲染预热引擎
func (e *Engine) Start() error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if e.isRunning {
		return nil
	}

	// 初始化浏览器池
	if err := e.initBrowserPool(); err != nil {
		return err
	}

	// 初始化预热管理器
	e.preheatManager = &PreheatManager{
		config:      e.config,
		engine:      e,
		redisClient: e.redisClient,
	}

	// 启动任务处理器
	e.startWorkers()

	// 启动浏览器健康检查
	e.startHealthCheck()

	e.isRunning = true
	return nil
}

// Stop 停止渲染预热引擎
func (e *Engine) Stop() error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if !e.isRunning {
		return nil
	}

	// 停止健康检查
	if e.healthCheckTicker != nil {
		e.healthCheckTicker.Stop()
		e.healthCheckTicker = nil
	}

	// 取消上下文
	e.cancel()

	// 等待工作协程结束
	e.workerWg.Wait()

	// 关闭浏览器池
	e.closeBrowserPool()

	e.isRunning = false
	return nil
}

// isStaticResource 检查URL是否为静态资源
func (e *Engine) isStaticResource(url string) bool {
	// 静态资源文件扩展名列表
	staticExtensions := []string{
		".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg",
		".css", ".less", ".sass", ".scss",
		".js", ".jsx", ".ts", ".tsx",
		".woff", ".woff2", ".ttf", ".eot",
		".ico", ".txt", ".json", ".xml", ".pdf", ".zip", ".rar",
		".mp4", ".mp3", ".avi", ".mov", ".wmv",
		".csv", ".xls", ".xlsx", ".doc", ".docx",
	}

	// 检查URL是否以静态资源扩展名结尾
	for _, ext := range staticExtensions {
		if len(url) >= len(ext) && url[len(url)-len(ext):] == ext {
			return true
		}
	}

	return false
}

// isPaymentReturn 检查URL是否为支付返回页面
func (e *Engine) isPaymentReturn(url string) bool {
	// 支付返回页面的常见路径模式
	paymentPaths := []string{
		"/payment/return", "/pay/callback", "/order/notify", "/payment/callback",
		"/pay/return", "/checkout/return", "/checkout/callback",
	}

	// 检查URL是否包含支付返回路径
	for _, path := range paymentPaths {
		if strings.Contains(url, path) {
			return true
		}
	}

	// 检查URL是否包含支付相关的查询参数
	paymentParams := []string{
		"?type=payment", "&type=payment", "?action=return", "&action=return",
		"?notify=1", "&notify=1", "?callback=1", "&callback=1",
	}

	for _, param := range paymentParams {
		if strings.Contains(url, param) {
			return true
		}
	}

	return false
}

// RenderResultWithCache 包含缓存命中信息的渲染结果
type RenderResultWithCache struct {
	Result   *RenderResult
	HitCache bool
}

// Render 执行渲染任务
func (e *Engine) Render(ctx context.Context, url string, options RenderOptions) (*RenderResultWithCache, error) {
	// 检查是否为静态资源或支付返回页面，如果是则跳过渲染预热
	if e.isStaticResource(url) || e.isPaymentReturn(url) {
		// 直接返回空结果或跳过渲染预热，这里返回一个特殊的成功结果表示不需要渲染预热
		return &RenderResultWithCache{
			Result: &RenderResult{
				HTML:    "",
				Success: true,
				Error:   "",
			},
			HitCache: false,
		}, nil
	}

	// 构建缓存键
	cacheKey := fmt.Sprintf("prerender:%s:content:%s", e.SiteName, url)

	// 尝试从Redis获取缓存
	if e.redisClient != nil {
		// 获取缓存的HTML内容
		cachedHTML, err := e.redisClient.GetRawClient().Get(e.ctx, cacheKey).Result()
		if err == nil {
			// 缓存命中，直接返回
			return &RenderResultWithCache{
				Result: &RenderResult{
					HTML:    cachedHTML,
					Success: true,
					Error:   "",
				},
				HitCache: true,
			}, nil
		}
	}

	// 对于静态模式的站点，直接读取静态文件而不是使用浏览器渲染，避免循环依赖
	// 检查URL是否指向本地静态资源
	isLocalURL := strings.Contains(url, "localhost:8081") || strings.Contains(url, "127.0.0.1:8081")
	if isLocalURL {
		// 尝试直接读取静态文件
		// 移除协议和域名，只保留路径
		path := url
		if idx := strings.Index(url, "/"); idx != -1 {
			path = url[idx:]
		}

		// 默认站点的静态文件目录，使用配置文件中的路径
		staticDir := filepath.Join(e.staticDir, e.SiteName)

		// 处理URL，移除hash部分并获取实际路径
		getActualPath := func(urlPath string) string {
			// 移除URL中的hash部分，因为hash不会发送到服务器
			if hashIndex := strings.Index(urlPath, "#"); hashIndex != -1 {
				return urlPath[:hashIndex]
			}
			return urlPath
		}

		// 获取实际路径（移除hash部分）
		actualPath := getActualPath(path)

		// 构建文件路径
		filePath := staticDir + actualPath

		// 如果路径是目录或不存在，尝试添加index.html
		if info, err := os.Stat(filePath); err != nil || info.IsDir() {
			filePath = filepath.Join(staticDir, actualPath, "index.html")
		}

		// 读取文件内容
		htmlContent, err := os.ReadFile(filePath)
		if err == nil {
			// 成功读取文件，将内容返回并缓存
			htmlStr := string(htmlContent)
			if e.redisClient != nil {
				// 将渲染结果存入Redis缓存
				cacheTTL := time.Duration(e.config.CacheTTL) * time.Second
				e.redisClient.GetRawClient().Set(e.ctx, cacheKey, htmlStr, cacheTTL).Err()
				// 更新URL状态为cached
				e.redisClient.SetURLPreheatStatus(e.SiteName, url, "cached", int64(len(htmlStr)))
			}
			return &RenderResultWithCache{
				Result: &RenderResult{
					HTML:    htmlStr,
					Success: true,
					Error:   "",
				},
				HitCache: false,
			}, nil
		}
	}

	// 创建渲染任务
	task := &RenderTask{
		ID:      uuid.New().String(),
		URL:     url,
		Options: options,
		Result:  make(chan *RenderResult, 1),
	}

	// 发送到任务队列
	select {
	case e.taskQueue <- task:
		// 等待结果
		select {
		case result := <-task.Result:
			if result.Success && result.HTML != "" && e.redisClient != nil {
				// 将渲染结果存入Redis缓存
				cacheTTL := time.Duration(e.config.CacheTTL) * time.Second
				e.redisClient.GetRawClient().Set(e.ctx, cacheKey, result.HTML, cacheTTL).Err()
				// 更新URL状态为cached
				e.redisClient.SetURLPreheatStatus(e.SiteName, url, "cached", int64(len(result.HTML)))
			}
			return &RenderResultWithCache{
				Result:   result,
				HitCache: false,
			}, nil
		case <-ctx.Done():
			return &RenderResultWithCache{
				Result:   &RenderResult{Success: false, Error: "context canceled"},
				HitCache: false,
			}, ctx.Err()
		case <-e.ctx.Done():
			return &RenderResultWithCache{
				Result:   &RenderResult{Success: false, Error: "engine stopped"},
				HitCache: false,
			}, nil
		}
	case <-ctx.Done():
		return &RenderResultWithCache{
			Result:   &RenderResult{Success: false, Error: "context canceled"},
			HitCache: false,
		}, ctx.Err()
	case <-e.ctx.Done():
		return &RenderResultWithCache{
			Result:   &RenderResult{Success: false, Error: "engine stopped"},
			HitCache: false,
		}, nil
	}
}

// TriggerPreheat 触发缓存预热
func (e *Engine) TriggerPreheat() (string, error) {
	if e.preheatManager == nil {
		return "", nil
	}
	// 默认使用localhost:8081，兼容旧版API
	return e.preheatManager.TriggerPreheatWithURL("http://localhost:8081", "localhost:8081")
}

// TriggerPreheatWithURL 触发缓存预热，支持自定义baseURL和Domain
func (e *Engine) TriggerPreheatWithURL(baseURL, domain string) (string, error) {
	if e.preheatManager == nil {
		return "", nil
	}
	return e.preheatManager.TriggerPreheatWithURL(baseURL, domain)
}

// GetPreheatStatus 获取预热状态
func (e *Engine) GetPreheatStatus() map[string]interface{} {
	if e.preheatManager == nil {
		return map[string]interface{}{
			"isRunning": false,
		}
	}
	return e.preheatManager.GetStatus()
}

// GetConfig 获取引擎配置
func (e *Engine) GetConfig() PrerenderConfig {
	return e.config
}

// GetPreheatManager 获取预热管理器
func (e *Engine) GetPreheatManager() *PreheatManager {
	return e.preheatManager
}

// GetCrawlerHeaders 获取完整的爬虫协议头列表
func (e *Engine) GetCrawlerHeaders() []string {
	// 合并默认爬虫协议头和配置中的爬虫协议头
	allHeaders := append(e.defaultCrawlerHeaders, e.config.CrawlerHeaders...)

	// 去重
	uniqueHeaders := make([]string, 0, len(allHeaders))
	seen := make(map[string]bool)
	for _, header := range allHeaders {
		if !seen[header] {
			seen[header] = true
			uniqueHeaders = append(uniqueHeaders, header)
		}
	}

	return uniqueHeaders
}

// IsCrawlerRequest 检查请求是否来自爬虫
func (e *Engine) IsCrawlerRequest(userAgent string) bool {
	crawlerHeaders := e.GetCrawlerHeaders()

	// 如果没有配置爬虫协议头，默认返回false
	if len(crawlerHeaders) == 0 {
		return false
	}

	// 检查User-Agent是否包含任何爬虫协议头
	for _, header := range crawlerHeaders {
		if strings.Contains(strings.ToLower(userAgent), strings.ToLower(header)) {
			return true
		}
	}

	return false
}

// initBrowserPool 初始化浏览器池
func (e *Engine) initBrowserPool() error {
	e.browserPool = make([]*Browser, 0, e.config.PoolSize)

	for i := 0; i < e.config.PoolSize; i++ {
		// 启动一个新的浏览器实例
		launchOpts := launcher.New()
		launchOpts.Set("headless")
		launchOpts.Set("no-sandbox")
		launchOpts.Set("disable-dev-shm-usage")
		launchOpts.Set("disable-gpu")
		launchOpts.Set("disable-setuid-sandbox")
		launchOpts.Set("single-process")
		launchOpts.Set("disable-accelerated-2d-canvas")
		launchOpts.Set("disable-javascript-harmony")
		launchOpts.Set("disable-features", "site-per-process")
		launchOpts.Set("ignore-certificate-errors")
		launchOpts.Set("disable-web-security")

		// 启动浏览器
		browserURL, err := launchOpts.Launch()
		if err != nil {
			return fmt.Errorf("failed to launch browser: %v", err)
		}

		// 连接到浏览器
		rodBrowser := rod.New().ControlURL(browserURL)
		if err := rodBrowser.Connect(); err != nil {
			return fmt.Errorf("failed to connect to browser: %v", err)
		}

		// 创建浏览器实例
		browser := &Browser{
			ID:         fmt.Sprintf("browser-%d", i),
			Status:     "available",
			LastUsed:   time.Now(),
			Healthy:    true,
			ErrorCount: 0,
			CreatedAt:  time.Now(),
			Instance:   rodBrowser,
		}
		e.browserPool = append(e.browserPool, browser)
		// 添加到空闲浏览器通道
		e.idleBrowsers <- browser
	}

	return nil
}

// closeBrowserPool 关闭浏览器池
func (e *Engine) closeBrowserPool() {
	// 关闭所有浏览器实例
	for i, browser := range e.browserPool {
		browser.Status = "closed"
		browser.Healthy = false

		// 关闭实际的浏览器实例
		if browser.Instance != nil {
			if err := browser.Instance.Close(); err != nil {
				logging.DefaultLogger.Warn("Failed to close browser %s: %v", browser.ID, err)
			}
		}

		e.browserPool[i] = browser
	}

	// 关闭空闲浏览器通道
	close(e.idleBrowsers)
	e.browserPool = nil
}

// startWorkers 启动工作协程
func (e *Engine) startWorkers() {
	// 启动任务分发器
	e.workerWg.Add(1)
	go e.taskDispatcher()
}

// startHealthCheck 启动浏览器健康检查
func (e *Engine) startHealthCheck() {
	// 每30秒检查一次浏览器健康状态
	e.healthCheckTicker = time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-e.healthCheckTicker.C:
				e.checkBrowsersHealth()
			case <-e.ctx.Done():
				return
			}
		}
	}()
}

// checkBrowsersHealth 检查浏览器健康状态
func (e *Engine) checkBrowsersHealth() {
	e.mutex.RLock()
	browsers := make([]*Browser, len(e.browserPool))
	copy(browsers, e.browserPool)
	e.mutex.RUnlock()

	for i, browser := range browsers {
		// 检查浏览器是否超过最大生命周期（2小时）
		if time.Since(browser.CreatedAt) > 2*time.Hour {
			e.replaceBrowser(i, browser)
			continue
		}

		// 检查浏览器错误计数是否超过阈值
		if browser.ErrorCount > 5 {
			e.replaceBrowser(i, browser)
			continue
		}

		// 检查浏览器是否长时间未使用（30分钟）
		if browser.Status == "available" && time.Since(browser.LastUsed) > 30*time.Minute {
			e.replaceBrowser(i, browser)
			continue
		}
	}
}

// replaceBrowser 替换不健康的浏览器
func (e *Engine) replaceBrowser(index int, oldBrowser *Browser) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// 检查浏览器是否仍在池中和位置是否正确
	if index >= len(e.browserPool) || e.browserPool[index] != oldBrowser {
		return
	}

	// 启动一个新的浏览器实例
	launchOpts := launcher.New()
	launchOpts.Set("headless")
	launchOpts.Set("no-sandbox")
	launchOpts.Set("disable-dev-shm-usage")
	launchOpts.Set("disable-gpu")
	launchOpts.Set("disable-setuid-sandbox")
	launchOpts.Set("single-process")
	launchOpts.Set("disable-accelerated-2d-canvas")
	launchOpts.Set("disable-javascript-harmony")
	launchOpts.Set("disable-features", "site-per-process")
	launchOpts.Set("ignore-certificate-errors")
	launchOpts.Set("disable-web-security")

	// 启动浏览器
	browserURL, err := launchOpts.Launch()
	if err != nil {
		// 如果启动失败，标记原浏览器为健康并返回
		oldBrowser.Healthy = true
		oldBrowser.ErrorCount = 0
		return
	}

	// 连接到浏览器
	rodBrowser := rod.New().ControlURL(browserURL)
	if err := rodBrowser.Connect(); err != nil {
		// 如果连接失败，标记原浏览器为健康并返回
		oldBrowser.Healthy = true
		oldBrowser.ErrorCount = 0
		logging.DefaultLogger.Error("Failed to connect to browser: %v", err)
		return
	}

	// 创建新浏览器
	newBrowser := &Browser{
		ID:         fmt.Sprintf("browser-%d", time.Now().UnixNano()),
		Status:     "available",
		LastUsed:   time.Now(),
		Healthy:    true,
		ErrorCount: 0,
		CreatedAt:  time.Now(),
		Instance:   rodBrowser,
	}

	// 关闭旧浏览器实例
	if oldBrowser.Instance != nil {
		if err := oldBrowser.Instance.Close(); err != nil {
			logging.DefaultLogger.Warn("Failed to close old browser %s: %v", oldBrowser.ID, err)
		}
	}

	// 替换浏览器
	e.browserPool[index] = newBrowser

	// 将新浏览器添加到空闲通道
	select {
	case e.idleBrowsers <- newBrowser:
	default:
		// 如果通道已满，忽略
	}
}

// taskDispatcher 任务分发器，将任务分配给空闲浏览器
func (e *Engine) taskDispatcher() {
	defer e.workerWg.Done()

	for {
		select {
		case task := <-e.taskQueue:
			// 从空闲浏览器通道获取一个浏览器
			select {
			case browser := <-e.idleBrowsers:
				// 启动工作协程处理任务
				e.workerWg.Add(1)
				go e.processTask(browser, task)
			case <-e.ctx.Done():
				return
			}
		case <-e.ctx.Done():
			return
		}
	}
}

// processTask 处理渲染任务
func (e *Engine) processTask(browser *Browser, task *RenderTask) {
	defer e.workerWg.Done()

	// 增加活跃任务数
	e.taskMutex.Lock()
	e.activeTasks++
	e.taskMutex.Unlock()

	defer func() {
		// 减少活跃任务数
		e.taskMutex.Lock()
		e.activeTasks--
		e.taskMutex.Unlock()
	}()

	// 更新浏览器状态
	e.mutex.Lock()
	browser.Status = "working"
	browser.LastUsed = time.Now()
	e.mutex.Unlock()

	// 实现超时控制
	timeout := time.Duration(task.Options.Timeout) * time.Second
	if timeout == 0 {
		timeout = time.Duration(e.config.Timeout) * time.Second
	}
	// 限制最大超时时间，防止单个任务占用资源过久
	if timeout > 30*time.Second {
		timeout = 30 * time.Second
	}

	// 创建带超时的上下文
	taskCtx, taskCancel := context.WithTimeout(e.ctx, timeout)
	defer taskCancel()

	// 结果变量
	result := &RenderResult{
		Success: false,
		Error:   "",
	}

	// 执行渲染，使用双重defer防护
	func() {
		// 最外层panic恢复，确保无论发生什么都能正常释放资源
		defer func() {
			if r := recover(); r != nil {
				result.Error = fmt.Sprintf("render panic: %v", r)
				// 标记浏览器为不健康
				e.mutex.Lock()
				browser.Healthy = false
				browser.ErrorCount++
				e.mutex.Unlock()
				logging.DefaultLogger.Error("Render panic for URL %s: %v", task.URL, r)
			}
		}()

		// 检查浏览器是否健康
		e.mutex.RLock()
		if !browser.Healthy || browser.Instance == nil {
			e.mutex.RUnlock()
			result.Error = "browser is not healthy"
			return
		}
		e.mutex.RUnlock()

		// 创建新页面，增加重试机制
		var page *rod.Page
		var err error
		for i := 0; i < 2; i++ {
			page, err = browser.Instance.Page(proto.TargetCreateTarget{})
			if err == nil {
				break
			}
			// 短暂等待后重试
			time.Sleep(500 * time.Millisecond)
		}
		if err != nil {
			result.Error = fmt.Sprintf("failed to create page: %v", err)
			// 标记浏览器为不健康
			e.mutex.Lock()
			browser.Healthy = false
			browser.ErrorCount++
			e.mutex.Unlock()
			logging.DefaultLogger.Error("Failed to create page for URL %s: %v", task.URL, err)
			return
		}

		// 页面关闭防护，确保资源释放
		pageClosed := false
		defer func() {
			if !pageClosed {
				// 异步关闭页面，避免阻塞主流程
				go func() {
					if err := page.Close(); err != nil {
						logging.DefaultLogger.Warn("Failed to close page: %v", err)
					}
				}()
			}
		}()

		// 导航到URL，增加超时控制
		navigateDone := make(chan bool)
		go func() {
			defer close(navigateDone)
			if err := page.Navigate(task.URL); err != nil {
				select {
				case <-taskCtx.Done():
					// 上下文已取消，忽略错误
				default:
					err = fmt.Errorf("failed to navigate to %s: %v", task.URL, err)
				}
			}
		}()

		select {
		case <-navigateDone:
		case <-taskCtx.Done():
			result.Error = "navigation timeout"
			return
		}

		if err != nil {
			result.Error = err.Error()
			return
		}

		// 等待页面加载完成，使用更安全的等待策略
		waitDone := make(chan bool)
		go func() {
			defer close(waitDone)
			// 使用多个等待策略，提高成功率
			waitErr := page.WaitLoad()
			if waitErr != nil {
				logging.DefaultLogger.Warn("WaitLoad failed for %s, trying to wait for network idle: %v", task.URL, waitErr)
				// 使用简单的等待策略，适用于hash模式
				time.Sleep(1 * time.Second)
			}
		}()

		select {
		case <-waitDone:
		case <-taskCtx.Done():
			result.Error = "page load timeout"
			return
		}

		// 检查URL是否包含hash
		isHashURL := strings.Contains(task.URL, "#")

		// 根据WaitUntil选项和是否为hash URL决定等待策略，缩短等待时间
		baseWaitTime := 1 * time.Second
		if isHashURL {
			baseWaitTime = 2 * time.Second
		}

		switch task.Options.WaitUntil {
		case "networkidle0":
			// 等待网络空闲（0个网络连接）
			time.Sleep(baseWaitTime + 1*time.Second)
		case "networkidle2":
			// 等待网络空闲（最多2个网络连接）
			time.Sleep(baseWaitTime)
		case "domcontentloaded":
			// 已经通过page.WaitLoad()等待了DOM内容加载
			time.Sleep(500 * time.Millisecond)
		case "load":
			// 已经通过page.WaitLoad()等待了页面加载
			time.Sleep(baseWaitTime)
		default:
			// 默认等待策略
			time.Sleep(baseWaitTime)
		}

		// 获取完整的HTML内容，增加超时控制
		htmlDone := make(chan struct {
			html string
			err  error
		})
		go func() {
			defer close(htmlDone)
			html, err := page.HTML()
			htmlDone <- struct {
				html string
				err  error
			}{html, err}
		}()

		var html string
		select {
		case res := <-htmlDone:
			html, err = res.html, res.err
		case <-taskCtx.Done():
			result.Error = "html extraction timeout"
			return
		}

		if err != nil {
			result.Error = fmt.Sprintf("failed to get html: %v", err)
			return
		}

		// 验证HTML内容
		if html == "" {
			result.Error = "empty html content"
			return
		}

		// 检查是否包含基本的HTML结构，放宽验证条件，提高容错性
		lowerHTML := strings.ToLower(html)
		if !strings.Contains(lowerHTML, "<html") {
			// 如果没有完整HTML结构，尝试提取body内容
			bodyStart := strings.Index(lowerHTML, "<body")
			if bodyStart == -1 {
				result.Error = "incomplete html structure"
				return
			}
			// 允许只有body的情况
		} else if !strings.Contains(lowerHTML, "<body") {
			// 如果有html但没有body，也允许通过
			logging.DefaultLogger.Warn("HTML missing body tag for URL %s", task.URL)
		}

		// 标记页面已关闭，避免重复关闭
		pageClosed = true
		if err := page.Close(); err != nil {
			logging.DefaultLogger.Warn("Failed to close page: %v", err)
		}

		// 成功获取HTML
		result.HTML = html
		result.Success = true
	}()

	// 更新浏览器状态并返回结果
	e.mutex.Lock()
	browser.Status = "available"
	// 降低错误计数阈值，更快替换不健康的浏览器
	if browser.ErrorCount > 3 {
		browser.Healthy = false
	} else {
		browser.Healthy = true
	}
	e.mutex.Unlock()

	// 将浏览器放回空闲通道（仅当健康时）
	if browser.Healthy {
		select {
		case e.idleBrowsers <- browser:
		default:
			// 如果通道已满，关闭该浏览器并创建新的
			if browser.Instance != nil {
				if err := browser.Instance.Close(); err != nil {
					logging.DefaultLogger.Warn("Failed to close extra browser %s: %v", browser.ID, err)
				}
			}
			// 异步替换浏览器
			go func() {
				e.mutex.Lock()
				defer e.mutex.Unlock()
				for i, b := range e.browserPool {
					if b.ID == browser.ID {
						e.replaceBrowser(i, browser)
						break
					}
				}
			}()
		}
	} else {
		// 如果浏览器不健康，关闭并替换它
		if browser.Instance != nil {
			// 异步关闭浏览器，避免阻塞主流程
			go func() {
				if err := browser.Instance.Close(); err != nil {
					logging.DefaultLogger.Warn("Failed to close unhealthy browser %s: %v", browser.ID, err)
				}
			}()
		}
		// 异步替换浏览器
		go func() {
			// 查找浏览器在池中位置
			e.mutex.Lock()
			defer e.mutex.Unlock()
			for i, b := range e.browserPool {
				if b.ID == browser.ID {
					e.replaceBrowser(i, browser)
					break
				}
			}
		}()
	}

	// 发送结果，使用非阻塞方式
	select {
	case task.Result <- result:
	case <-taskCtx.Done():
		// 如果任务上下文已取消，忽略结果
		logging.DefaultLogger.Warn("Task context canceled, result ignored for URL %s", task.URL)
	}
	close(task.Result)
}
