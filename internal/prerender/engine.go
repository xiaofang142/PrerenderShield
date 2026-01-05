package prerender

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"prerender-shield/internal/redis"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/google/uuid"
)

// Engine 渲染预热引擎
type Engine struct {
	SiteName             string
	config               PrerenderConfig
	browserPool          []*Browser
	idleBrowsers         chan *Browser
	taskQueue            chan *RenderTask
	isRunning            bool
	mutex                sync.RWMutex
	preheatManager       *PreheatManager
	workerWg             sync.WaitGroup
	ctx                  context.Context
	cancel               context.CancelFunc
	healthCheckTicker    *time.Ticker
	dynamicScalingTicker *time.Ticker // 动态扩容检查定时器
	queueLengthHistory   []int        // 任务队列长度历史，用于动态扩容决策
	queueMutex           sync.RWMutex // 队列长度历史互斥锁
	activeTasks          int          // 当前活跃任务数
	taskMutex            sync.RWMutex // 活跃任务数互斥锁
	redisClient          *redis.Client
	// 默认爬虫协议头列表
	defaultCrawlerHeaders []string
}

// EngineManager 渲染预热引擎管理器，管理多个站点的渲染预热引擎
type EngineManager struct {
	mutex   sync.RWMutex
	engines map[string]*Engine // 站点名 -> 引擎实例
	ctx     context.Context
	cancel  context.CancelFunc
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
	IdleTimeout       int      // 浏览器空闲超时时间（秒）
	DynamicScaling    bool     // 是否启用动态扩容/缩容
	ScalingFactor     float64  // 扩容因子，如0.5表示每次增加50%
	ScalingInterval   int      // 扩容检查间隔（秒）
	CrawlerHeaders    []string // 爬虫协议头列表
	UseDefaultHeaders bool     // 是否使用默认爬虫协议头
}

// PreheatConfig 缓存预热配置
type PreheatConfig struct {
	Enabled         bool
	SitemapURL      string
	Schedule        string
	Concurrency     int
	DefaultPriority int
	MaxDepth        int
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

	// 异步执行预热
	go func() {
		defer func() {
			pm.mutex.Lock()
			pm.isRunning = false
			pm.mutex.Unlock()
		}()

		// 获取所有URL后执行预热
		urls, err := pm.redisClient.GetURLs(pm.engine.SiteName)
		if err != nil {
			pm.redisClient.SetPreheatTaskStatus(pm.engine.SiteName, taskID, "failed")
			fmt.Printf("Failed to get URLs for preheat: %v\n", err)
			return
		}

		// 更新任务的总URL数
		totalURLs := int64(len(urls))
		pm.redisClient.UpdatePreheatTaskProgress(pm.engine.SiteName, taskID, totalURLs, 0, 0, 0)

		// 创建预热执行器配置
		preheatConfig := PreheatWorkerConfig{
			SiteName:       pm.engine.SiteName,
			RedisClient:    pm.redisClient,
			Concurrency:    pm.config.Preheat.Concurrency,
			CrawlerHeaders: pm.config.CrawlerHeaders,
		}

		// 创建预热执行器实例
		preheatWorker := NewPreheatWorker(preheatConfig)

		// 开始预热
		if err := preheatWorker.Start(); err != nil {
			pm.redisClient.SetPreheatTaskStatus(pm.engine.SiteName, taskID, "failed")
			fmt.Printf("Failed to preheat URLs: %v\n", err)
			return
		}

		// 更新统计数据
		pm.updateStats()

		// 标记任务完成
		pm.redisClient.SetPreheatTaskStatus(pm.engine.SiteName, taskID, "completed")
		fmt.Printf("Preheat completed for site: %s\n", pm.engine.SiteName)
	}()

	// 1. 首先爬取站点的所有链接
	fmt.Printf("Starting URL crawler for site: %s with baseURL: %s\n", pm.engine.SiteName, baseURL)

	// 创建爬虫配置
	crawlerConfig := CrawlerConfig{
		SiteName:    pm.engine.SiteName,
		Domain:      domain,
		BaseURL:     baseURL,
		MaxDepth:    pm.config.Preheat.MaxDepth,
		Concurrency: pm.config.Preheat.Concurrency,
		RedisClient: pm.redisClient,
	}

	// 创建爬虫实例
	crawler := NewCrawler(crawlerConfig)

	// 开始爬取
	go func() {
		if err := crawler.Start(); err != nil {
			pm.redisClient.SetPreheatTaskStatus(pm.engine.SiteName, taskID, "failed")
			fmt.Printf("Failed to crawl URLs: %v\n", err)
			pm.mutex.Lock()
			pm.isRunning = false
			pm.mutex.Unlock()
			return
		}
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

	// 创建预热执行器配置
	preheatConfig := PreheatWorkerConfig{
		SiteName:       pm.engine.SiteName,
		RedisClient:    pm.redisClient,
		Concurrency:    1,
		CrawlerHeaders: pm.config.CrawlerHeaders,
	}

	// 创建预热执行器实例
	preheatWorker := NewPreheatWorker(preheatConfig)

	// 预热单个URL
	return preheatWorker.PreheatURLWithHeaders(url, map[string]string{
		"User-Agent": pm.config.CrawlerHeaders[0], // 使用第一个爬虫协议头
	})
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
func NewEngine(siteName string, config PrerenderConfig, redisClient *redis.Client) (*Engine, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// 设置默认值
	if config.MinPoolSize == 0 {
		config.MinPoolSize = 2 // 默认最小浏览器数
	}
	if config.MaxPoolSize == 0 {
		config.MaxPoolSize = config.PoolSize * 2 // 默认最大浏览器数为初始值的2倍
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 300 // 默认空闲超时5分钟
	}
	if config.DynamicScaling {
		if config.ScalingFactor == 0 {
			config.ScalingFactor = 0.5 // 默认扩容因子50%
		}
		if config.ScalingInterval == 0 {
			config.ScalingInterval = 60 // 默认每分钟检查一次
		}
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
func NewEngineManager() *EngineManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &EngineManager{
		engines: make(map[string]*Engine),
		mutex:   sync.RWMutex{},
		ctx:     ctx,
		cancel:  cancel,
	}
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
	engine, err := NewEngine(siteName, config, redisClient)
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

	em.cancel()
	return nil
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

	// 启动动态扩容检查（如果启用）
	if e.config.DynamicScaling {
		e.startDynamicScaling()
	}

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

	// 停止动态扩容检查
	if e.dynamicScalingTicker != nil {
		e.dynamicScalingTicker.Stop()
		e.dynamicScalingTicker = nil
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
			// 不再将结果存入缓存，直接返回
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

// GetCrawlerHeaders 获取完整的爬虫协议头列表（包括默认和自定义的）
func (e *Engine) GetCrawlerHeaders() []string {
	// 合并默认和自定义爬虫协议头
	allHeaders := make([]string, 0)

	// 如果启用了默认爬虫协议头，添加默认列表
	if e.config.UseDefaultHeaders {
		allHeaders = append(allHeaders, e.defaultCrawlerHeaders...)
	}

	// 添加自定义爬虫协议头
	if len(e.config.CrawlerHeaders) > 0 {
		allHeaders = append(allHeaders, e.config.CrawlerHeaders...)
	}

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
		rodBrowser := rod.New().ControlURL(browserURL).MustConnect()

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
			browser.Instance.MustClose()
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
	rodBrowser := rod.New().ControlURL(browserURL).MustConnect()

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
		oldBrowser.Instance.MustClose()
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

	// 创建带超时的上下文
	taskCtx, taskCancel := context.WithTimeout(e.ctx, timeout)
	defer taskCancel()

	// 结果变量
	result := &RenderResult{
		Success: false,
		Error:   "",
	}

	// 执行渲染
	func() {
		// 使用defer恢复panic
		defer func() {
			if r := recover(); r != nil {
				result.Error = fmt.Sprintf("render panic: %v", r)
				// 标记浏览器为不健康
				e.mutex.Lock()
				browser.Healthy = false
				browser.ErrorCount++
				e.mutex.Unlock()
			}
		}()

		// 创建新页面
		page := browser.Instance.MustPage()
		defer page.MustClose()

		// 导航到URL
		page.MustNavigate(task.URL)

		// 等待页面加载完成
		page.MustWaitLoad()

		// 检查URL是否包含hash
		isHashURL := strings.Contains(task.URL, "#")

		// 根据WaitUntil选项和是否为hash URL决定等待策略
		switch task.Options.WaitUntil {
		case "networkidle0":
			// 等待网络空闲（0个网络连接）- 使用简单的等待方式
			if isHashURL {
				// hash路由需要额外等待，因为hash处理是在客户端JavaScript中完成的
				time.Sleep(4 * time.Second)
			} else {
				time.Sleep(3 * time.Second)
			}
		case "networkidle2":
			// 等待网络空闲（最多2个网络连接）- 使用简单的等待方式
			if isHashURL {
				// hash路由需要额外等待
				time.Sleep(3 * time.Second)
			} else {
				time.Sleep(2 * time.Second)
			}
		case "domcontentloaded":
			// 已经通过page.MustWaitLoad()等待了DOM内容加载
			// 额外等待1秒确保JavaScript执行
			time.Sleep(1 * time.Second)
			// 对于hash URL，再额外等待1秒确保hash路由处理完成
			if isHashURL {
				time.Sleep(1 * time.Second)
			}
		case "load":
			// 已经通过page.MustWaitLoad()等待了页面加载
			// 额外等待2秒确保JavaScript执行和页面渲染
			time.Sleep(2 * time.Second)
			// 对于hash URL，再额外等待1秒确保hash路由处理完成
			if isHashURL {
				time.Sleep(1 * time.Second)
			}
		default:
			// 默认等待策略：等待3秒确保JavaScript执行和页面渲染
			time.Sleep(3 * time.Second)
			// 对于hash URL，再额外等待1秒确保hash路由处理完成
			if isHashURL {
				time.Sleep(1 * time.Second)
			}
		}

		// 最后再等待一小段时间，确保所有异步操作完成
		time.Sleep(500 * time.Millisecond)

		// 获取完整的HTML内容
		html, err := page.HTML()
		if err != nil {
			result.Error = fmt.Sprintf("failed to get html: %v", err)
			return
		}

		// 验证HTML内容
		if html == "" {
			result.Error = "empty html content"
			return
		}

		// 检查是否包含基本的HTML结构
		lowerHTML := strings.ToLower(html)
		if !strings.Contains(lowerHTML, "<html") || !strings.Contains(lowerHTML, "<body") {
			result.Error = "incomplete html structure"
			return
		}

		// 检查body是否为空
		bodyStart := strings.Index(lowerHTML, "<body")
		bodyEnd := strings.LastIndex(lowerHTML, "</body>")
		if bodyStart == -1 || bodyEnd == -1 || bodyEnd <= bodyStart {
			result.Error = "empty body content"
			return
		}

		// 检查body内容是否只有空白字符
		bodyContent := strings.TrimSpace(html[bodyStart:bodyEnd])
		if bodyContent == "" || bodyContent == "<body>" || bodyContent == "<body></body>" || bodyContent == "<body />" {
			result.Error = "empty body content"
			return
		}

		// 成功获取HTML
		result.HTML = html
		result.Success = true
	}()

	// 更新浏览器状态并返回结果
	e.mutex.Lock()
	browser.Status = "available"
	// 如果浏览器不健康，不将其放回池中
	if browser.ErrorCount > 5 {
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
			// 如果通道已满，忽略
		}
	} else {
		// 如果浏览器不健康，关闭并替换它
		if browser.Instance != nil {
			browser.Instance.MustClose()
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

	// 发送结果
	select {
	case task.Result <- result:
	case <-taskCtx.Done():
		// 如果任务上下文已取消，忽略结果
	}
	close(task.Result)
}

// startDynamicScaling 启动动态扩容检查
func (e *Engine) startDynamicScaling() {
	// 每config.ScalingInterval秒检查一次
	e.dynamicScalingTicker = time.NewTicker(time.Duration(e.config.ScalingInterval) * time.Second)
	go func() {
		for {
			select {
			case <-e.dynamicScalingTicker.C:
				e.adjustPoolSize()
			case <-e.ctx.Done():
				return
			}
		}
	}()
}

// adjustPoolSize 根据当前负载调整浏览器池大小
func (e *Engine) adjustPoolSize() {
	// 获取当前队列长度
	queueLen := len(e.taskQueue)

	// 记录队列长度历史
	e.queueMutex.Lock()
	e.queueLengthHistory = append(e.queueLengthHistory, queueLen)
	// 只保留最近10个记录
	if len(e.queueLengthHistory) > 10 {
		e.queueLengthHistory = e.queueLengthHistory[len(e.queueLengthHistory)-10:]
	}
	e.queueMutex.Unlock()

	// 获取当前浏览器池大小和活跃任务数
	e.mutex.RLock()
	currentSize := len(e.browserPool)
	e.mutex.RUnlock()

	e.taskMutex.RLock()
	activeTasks := e.activeTasks
	e.taskMutex.RUnlock()

	// 计算空闲浏览器数
	idleCount := currentSize - activeTasks

	// 扩容策略：如果队列中有任务且没有空闲浏览器，且当前大小小于最大限制，则扩容
	needScaleUp := queueLen > 0 && idleCount == 0 && currentSize < e.config.MaxPoolSize

	// 缩容策略：如果空闲浏览器数超过当前大小的30%，且当前大小大于最小限制，则缩容
	needScaleDown := float64(idleCount) > float64(currentSize)*0.3 && currentSize > e.config.MinPoolSize

	if needScaleUp {
		// 计算需要增加的浏览器数
		addCount := int(float64(currentSize) * e.config.ScalingFactor)
		if addCount == 0 {
			addCount = 1 // 至少增加1个
		}
		// 确保不超过最大限制
		newSize := currentSize + addCount
		if newSize > e.config.MaxPoolSize {
			addCount = e.config.MaxPoolSize - currentSize
		}

		if addCount > 0 {
			for i := 0; i < addCount; i++ {
				e.addBrowser()
			}
		}
	} else if needScaleDown {
		// 计算需要减少的浏览器数
		removeCount := int(float64(idleCount) * e.config.ScalingFactor)
		if removeCount == 0 {
			removeCount = 1 // 至少减少1个
		}
		// 确保不低于最小限制
		newSize := currentSize - removeCount
		if newSize < e.config.MinPoolSize {
			removeCount = currentSize - e.config.MinPoolSize
		}

		if removeCount > 0 {
			for i := 0; i < removeCount; i++ {
				e.removeIdleBrowser()
			}
		}
	}
}

// addBrowser 添加一个浏览器到池
func (e *Engine) addBrowser() {
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
		return // 如果启动失败，直接返回
	}

	// 连接到浏览器
	rodBrowser := rod.New().ControlURL(browserURL).MustConnect()

	e.mutex.Lock()
	defer e.mutex.Unlock()

	browser := &Browser{
		ID:         fmt.Sprintf("browser-%d", time.Now().UnixNano()),
		Status:     "available",
		LastUsed:   time.Now(),
		Healthy:    true,
		ErrorCount: 0,
		CreatedAt:  time.Now(),
		Instance:   rodBrowser,
	}

	e.browserPool = append(e.browserPool, browser)
	// 添加到空闲浏览器通道
	select {
	case e.idleBrowsers <- browser:
	default:
		// 如果通道已满，忽略
	}
}

// removeIdleBrowser 移除一个空闲浏览器
func (e *Engine) removeIdleBrowser() {
	// 从空闲浏览器通道获取一个浏览器
	select {
	case browser := <-e.idleBrowsers:
		e.mutex.Lock()
		// 从浏览器池中移除
		for i, b := range e.browserPool {
			if b.ID == browser.ID {
				// 移除该浏览器
				e.browserPool = append(e.browserPool[:i], e.browserPool[i+1:]...)
				break
			}
		}
		e.mutex.Unlock()
	default:
		// 如果没有空闲浏览器，忽略
	}
}
