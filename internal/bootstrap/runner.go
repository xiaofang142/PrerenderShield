package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/api/routes"
	"prerender-shield/internal/config"
	"prerender-shield/internal/di"
	"prerender-shield/internal/firewall"
	"prerender-shield/internal/firewall/detectors"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/prerender/pool"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/scheduler"
	"prerender-shield/internal/seo"
	"prerender-shield/internal/utils"
	"prerender-shield/internal/websocket"
)

// AppRunner 应用运行器
type AppRunner struct {
	app         *Application
	config      *config.Config
	redisClient *redis.Client
	container   *di.Container
	wsHub       *websocket.Hub
	mu          sync.Mutex
	started     bool
}

// NewAppRunner 创建应用运行器
func NewAppRunner(app *Application) *AppRunner {
	return &AppRunner{
		app: app,
	}
}

// Shutdown 优雅关闭 (P0-1)
// 关闭顺序：
//  1. 停止接收新连接 (HTTP servers + 站点服务器)
//  2. 等待进行中的渲染任务完成 (设置硬超时)
//  3. 关闭调度器 (避免新任务被调度)
//  4. 关闭浏览器池 (释放 Chromium 进程)
//  5. 关闭监控/告警/威胁情报
//  6. 关闭 Redis 连接
func (r *AppRunner) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()

	logging.DefaultLogger.Info("AppRunner: starting graceful shutdown...")

	// 1. 关闭站点服务器 (允许已完成请求完成)
	if r.container != nil && r.container.SiteServerMgr != nil {
		servers := r.container.SiteServerMgr.ListSiteServers()
		for siteID, srv := range servers {
			if srv == nil {
				continue
			}
			if err := srv.Shutdown(ctx); err != nil {
				logging.DefaultLogger.Warn("Site server %s shutdown error: %v", siteID, err)
			} else {
				logging.DefaultLogger.Info("Site server %s stopped", siteID)
			}
		}
	}

	// 2. 关闭调度器 (停止派发新任务)
	if r.container != nil && r.container.Scheduler != nil {
		r.container.Scheduler.Stop()
	}

	// 2.5 停止 WebSocket 实时广播 (断开所有客户端，防止向已关停的依赖推送)
	if r.wsHub != nil {
		r.wsHub.Stop()
	}

	// 3. 关闭预渲染引擎 (会清理浏览器池)
	if r.container != nil && r.container.PrerenderMgr != nil {
		if err := r.container.PrerenderMgr.Close(); err != nil {
			logging.DefaultLogger.Warn("Prerender manager close error: %v", err)
		}
	}

	// 4. 关闭容器 (监控/告警/威胁情报/Redis)
	if r.container != nil {
		if err := r.container.Close(); err != nil {
			logging.DefaultLogger.Warn("Container close error: %v", err)
		}
	}

	logging.DefaultLogger.Info("AppRunner: graceful shutdown complete")
	return nil
}

// Initialize 初始化所有模块
func (r *AppRunner) Initialize(ctx context.Context) error {
	r.config = r.app.GetConfig()
	r.redisClient = r.app.GetRedis()

	// 孤儿清扫: 上次运行崩溃/SIGKILL 遗留的无头浏览器进程与临时目录。
	// 必须在实例池创建前执行——此刻存在的 chromedp 进程均为遗留，可安全回收
	if killed, dirs := pool.SweepOrphans(logging.DefaultLogger); killed > 0 || dirs > 0 {
		logging.DefaultLogger.Info("orphan sweep completed: %d processes killed, %d temp dirs removed", killed, dirs)
	}

	// 启动自检: 验证 Chromium 可用（渲染引擎核心依赖，缺失时给出明确指引）
	chromiumPath := utils.FirstNonEmpty(
		os.Getenv("PRERENDER_CHROMIUM_PATH"),
		os.Getenv("CHROME_PATH"),
	)
	if resolved, err := pool.ResolveChromiumPath(chromiumPath); err != nil {
		logging.DefaultLogger.Warn("Chromium not available, prerender engine will fail on render: %v", err)
	} else {
		logging.DefaultLogger.Info("Chromium resolved: %s", resolved)
	}

	// 使用依赖注入容器初始化所有模块
	var err error
	r.container, err = di.NewContainer(di.ContainerDeps{
		Config: r.config,
		Redis:  r.redisClient,
	})
	if err != nil {
		return fmt.Errorf("init container: %w", err)
	}

	return nil
}

// Start 启动所有服务
func (r *AppRunner) Start(ctx context.Context) error {
	c := r.container

	// P0-1: 标记已启动，使 Shutdown 知道是否需要执行清理
	r.mu.Lock()
	r.started = true
	r.mu.Unlock()

	// 启动监控
	if err := c.Monitor.Start(); err != nil {
		return fmt.Errorf("start monitoring: %w", err)
	}

	// 启动威胁情报拉取器
	if c.ThreatIntelFetcher != nil {
		c.ThreatIntelFetcher.Start()
	}

	// 生成 Sitemap 和 robots.txt
	r.generateSEOFiles()

	// 注册全局定时任务（SSL检查/日志清理/统计聚合/健康检查/SEO重生成）
	c.Scheduler.RegisterGlobalTasks(scheduler.GlobalTaskOptions{
		SSLCheckFn: func() error {
			if c.SSLManager != nil {
				expiring, err := c.SSLManager.CheckExpiration()
				if err != nil {
					return err
				}
				for _, domain := range expiring {
					logging.DefaultLogger.Warn("SSL certificate expiring soon: %s", domain)
				}
			}
			return nil
		},
		HealthCheckFn: func() error {
			// 检查Redis连接（通过简单操作验证可用性）
			if _, err := r.redisClient.Get("health:check"); err != nil {
				return fmt.Errorf("redis health check failed: %w", err)
			}
			return nil
		},
		SEORegenFn: func() error {
			r.generateSEOFiles()
			return nil
		},
	})

	// 启动调度器
	c.Scheduler.Start()

	// P0-14: 启动 SSL 自动续期器 (与 scheduler 并行, 立即触发一次检查)
	if c.SSLAutoRenewer != nil {
		c.SSLAutoRenewer.Start()
		logging.DefaultLogger.Info("SSL auto-renewer started")
	}

	// 为每个站点启动服务
	for _, site := range r.config.Sites {
		if err := r.startSite(site); err != nil {
			return fmt.Errorf("start site %s: %w", site.ID, err)
		}
	}

	// 启动 API 服务器
	if err := r.startAPIServer(ctx); err != nil {
		return fmt.Errorf("start API server: %w", err)
	}

	// 启动管理控制台
	if err := r.startConsoleServer(ctx); err != nil {
		return fmt.Errorf("start console server: %w", err)
	}

	return nil
}

// startSite 启动单个站点
func (r *AppRunner) startSite(site config.SiteConfig) error {
	c := r.container

	// 设置 SEO 配置（在创建引擎之前）
	if r.config.SEO.LLM.Enabled {
		llmConfig := &seo.LLMConfig{
			Enabled:     r.config.SEO.LLM.Enabled,
			Provider:    r.config.SEO.LLM.Provider,
			APIKey:      r.config.SEO.LLM.APIKey,
			APIURL:      r.config.SEO.LLM.APIURL,
			Model:       r.config.SEO.LLM.Model,
			MaxTokens:   r.config.SEO.LLM.MaxTokens,
			Temperature: r.config.SEO.LLM.Temperature,
			MaxRetries:  r.config.SEO.LLM.MaxRetries,
			Prompts: seo.LLMPrompts{
				TitleOptimization:       r.config.SEO.LLM.Prompts.TitleOptimization,
				DescriptionOptimization: r.config.SEO.LLM.Prompts.DescriptionOptimization,
				KeywordExtraction:       r.config.SEO.LLM.Prompts.KeywordExtraction,
				StructuredData:          r.config.SEO.LLM.Prompts.StructuredData,
			},
		}
		if timeout, err := time.ParseDuration(r.config.SEO.LLM.Timeout); err == nil {
			llmConfig.Timeout = timeout
		}
		c.PrerenderMgr.SetSEOConfig(site.ID, llmConfig)
	}

	// 获取或创建渲染引擎
	if _, exists := c.PrerenderMgr.GetEngine(site.ID); !exists {
		return fmt.Errorf("failed to get engine for site %s", site.ID)
	}

	// 创建防火墙引擎
	// 构建 CC 防护配置
	var ccCfg *detectors.CCProtectionConfig
	if site.Firewall.CCProtection.Enabled {
		rules := make([]detectors.CCProtectionRule, len(site.Firewall.CCProtection.Rules))
		for i, r := range site.Firewall.CCProtection.Rules {
			rules[i] = detectors.CCProtectionRule{
				Name:       r.Name,
				Path:       r.Path,
				Method:     r.Method,
				Dimensions: r.Dimensions,
				Requests:   r.Requests,
				Window:     r.Window,
				BanTime:    r.BanTime,
				Enabled:    r.Enabled,
			}
		}
		ccCfg = &detectors.CCProtectionConfig{
			Enabled: site.Firewall.CCProtection.Enabled,
			Rules:   rules,
		}
	}

	// 构建威胁情报配置
	var tiCfg *detectors.ThreatIntelConfig
	if site.Firewall.ThreatIntel.Enabled {
		tiCfg = &detectors.ThreatIntelConfig{
			Enabled:   true,
			GlobalKey: site.Firewall.ThreatIntel.GlobalKey,
		}
		if tiCfg.GlobalKey == "" {
			tiCfg.GlobalKey = "threatintel:global:blacklist"
		}
	}

	if err := c.FirewallMgr.AddSite(site.Name, firewall.Config{
		RulesPath: site.Firewall.RulesPath,
		ActionConfig: firewall.ActionConfig{
			DefaultAction: site.Firewall.ActionConfig.DefaultAction,
			BlockMessage:  site.Firewall.ActionConfig.BlockMessage,
		},
		StaticDir:           r.config.Dirs.StaticDir,
		GeoIPConfig:         &site.Firewall.GeoIPConfig,
		RateLimitConfig:     &site.Firewall.RateLimitConfig,
		FileIntegrityConfig: &site.FileIntegrityConfig,
		CCProtectionConfig:  ccCfg,
		ThreatIntelConfig:   tiCfg,
		Blacklist:           site.Firewall.Blacklist,
		Whitelist:           site.Firewall.Whitelist,
		RedisClient:         r.redisClient.GetRawClient(),
	}); err != nil {
		return fmt.Errorf("init firewall for site %s: %w", site.Name, err)
	}

	// 创建站点处理器
	siteHTTPHandler := c.SiteHandler.CreateSiteHandler(site, c.CrawlerLogMgr, c.VisitLogMgr, c.Monitor, r.config.Dirs.StaticDir)

	// 启动站点服务器
	c.SiteServerMgr.StartSiteServer(site, r.config.Server.Address, r.config.Dirs.StaticDir, c.CrawlerLogMgr, siteHTTPHandler)

	logging.DefaultLogger.Info("Site server started: %s (%s:%d)", site.Name, r.config.Server.Address, site.Port)
	return nil
}

// startAPIServer 启动 API 服务器
func (r *AppRunner) startAPIServer(ctx context.Context) error {
	c := r.container
	ginRouter := gin.Default()

	// WebSocket Hub 在 DI 装配阶段创建（Router 只注入使用，避免重复创建泄漏）
	wsHub := websocket.NewHub(logging.NewStructuredLogger(logging.INFO, ""))
	go wsHub.Run()
	r.wsHub = wsHub

	apiRouter := routes.NewRouter(
		c.UserManager,
		c.JWTManager,
		config.GetInstance(),
		c.PrerenderMgr,
		r.redisClient,
		c.Scheduler,
		c.SiteServerMgr,
		c.SiteHandler,
		c.Monitor,
		c.CrawlerLogMgr,
		c.VisitLogMgr,
		c.WafRepo,
		c.AuditLogger,
		r.config,
		wsHub,
	)

	apiRouter.RegisterRoutes(ginRouter)

	// WebSocket 实时广播接线：
	// 1) 告警触发时通过 WS 推送；2) 每 10s 推送一次监控指标快照
	if wsHub := apiRouter.GetHub(); wsHub != nil {
		c.Monitor.SetOnAlertCallback(func(alert *monitoring.AlertStatus, status string) {
			wsHub.BroadcastAlert(map[string]interface{}{
				"status":    status,
				"rule":      alert.Rule,
				"value":     alert.Value,
				"fired_at":  alert.FiredAt.Unix(),
				"last_seen": alert.LastChecked.Unix(),
			})
		})

		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if wsHub.GetClientCount() == 0 {
						continue
					}
					wsHub.BroadcastMonitor(c.Monitor.GetStats())
				}
			}
		}()
		logging.DefaultLogger.Info("WebSocket realtime broadcast enabled (alerts + metrics every 10s)")
	}

	addr := fmt.Sprintf("%s:%d", r.config.Server.Address, r.config.Server.APIPort)
	apiServer := &http.Server{
		Addr:    addr,
		Handler: ginRouter,
	}

	go func() {
		logging.DefaultLogger.Info("API server starting on %s", addr)
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.DefaultLogger.Fatal("API server error: %v", err)
		}
	}()

	r.app.AddServer(apiServer)
	return nil
}

// startConsoleServer 启动管理控制台
func (r *AppRunner) startConsoleServer(ctx context.Context) error {
	// 确定静态目录 - 按优先级尝试多个路径
	appDir := filepath.Dir(os.Args[0])
	currentDir, _ := os.Getwd()

	candidates := []string{
		filepath.Join(appDir, "web", "dist"),            // /opt/prerender-shield/web/dist
		filepath.Join(appDir, "web"),                    // /opt/prerender-shield/web
		filepath.Join(appDir, "bin", "web", "dist"),     // /opt/prerender-shield/bin/web/dist
		filepath.Join(appDir, "bin", "web"),             // /opt/prerender-shield/bin/web
		filepath.Join(currentDir, "web", "dist"),        // ./web/dist
		filepath.Join(currentDir, "web"),                // ./web
		filepath.Join(currentDir, "bin", "web", "dist"), // ./bin/web/dist
		filepath.Join(currentDir, "bin", "web"),         // ./bin/web
	}

	var actualStaticDir string
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			actualStaticDir = dir
			break
		}
	}
	if actualStaticDir == "" {
		actualStaticDir = candidates[0]
	}

	staticExts := []string{".html", ".htm", ".css", ".js", ".jpg", ".jpeg", ".png", ".gif", ".svg", ".ico", ".webp", ".woff", ".woff2", ".ttf", ".eot", ".json"}

	isStaticFile := func(path string) bool {
		ext := strings.ToLower(filepath.Ext(path))
		return slices.Contains(staticExts, ext)
	}

	// API 代理地址
	apiAddr := fmt.Sprintf("http://%s:%d", r.config.Server.Address, r.config.Server.APIPort)

	// 创建反向代理
	apiProxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			target, _ := url.Parse(apiAddr)
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		},
	}

	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.Split(r.URL.Path, "#")[0]

		// 代理 /api 请求到 API 服务器；/ws WebSocket 升级请求同样转发
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/ws/") {
			apiProxy.ServeHTTP(w, r)
			return
		}

		filePath := filepath.Join(actualStaticDir, strings.TrimPrefix(path, "/"))

		if isStaticFile(filePath) {
			if _, err := os.Stat(filePath); err == nil {
				http.ServeFile(w, r, filePath)
				return
			}
		}

		indexPath := filepath.Join(actualStaticDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.ServeFile(w, r, indexPath)
			return
		}

		http.NotFound(w, r)
	})

	addr := fmt.Sprintf("%s:%d", r.config.Server.Address, r.config.Server.ConsolePort)
	adminServer := &http.Server{
		Addr:    addr,
		Handler: adminMux,
	}

	go func() {
		logging.DefaultLogger.Info("Admin console starting on %s", addr)
		if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.DefaultLogger.Fatal("Admin console error: %v", err)
		}
	}()

	r.app.AddServer(adminServer)
	return nil
}

// Run 运行应用（一站式）
func Run(ctx context.Context, configPath string) error {
	// 创建应用
	app, err := New(ctx, configPath)
	if err != nil {
		return err
	}

	// 创建运行器
	runner := NewAppRunner(app)

	// 初始化
	if err := runner.Initialize(ctx); err != nil {
		return err
	}

	// P0-1: 注册 Graceful Shutdown hook (使用 runner.Shutdown 替代 container.Close)
	// 优先级更高，会先关闭站点服务器和预渲染引擎
	app.AddCleanup(func() {
		// 使用 30s 硬超时，确保即使 Shutdown 卡住也不会无限等待
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = runner.Shutdown(shutdownCtx)
	})

	// 启动服务
	if err := runner.Start(ctx); err != nil {
		return err
	}

	// 运行
	return app.Run(ctx)
}

// generateSEOFiles generates sitemap.xml and robots.txt based on config
func (r *AppRunner) generateSEOFiles() {
	seoCfg := r.config.SEO

	// Generate sitemap if enabled
	if seoCfg.Sitemap.Enabled {
		r.generateSitemap(seoCfg.Sitemap)
	}

	// Generate robots.txt if enabled
	if seoCfg.Robots.Enabled {
		r.generateRobotsTxt(seoCfg.Robots)
	}
}

// generateSitemap generates sitemap.xml
// 委托给 internal/seo 的共享实现（与 SEO API 保持一致），按站点生成
func (r *AppRunner) generateSitemap(cfg config.SitemapSEOConfig) {
	results := seo.GenerateForAllSites(r.config.Dirs.StaticDir, r.config.Sites, cfg)
	for _, res := range results {
		logging.DefaultLogger.Info("Sitemap generated for site %s: %s (%d URLs)", res.SiteID, res.OutputPath, res.URLCount)
	}
	if len(results) == 0 {
		logging.DefaultLogger.Warn("Sitemap generation skipped: no static site directories found under %s", r.config.Dirs.StaticDir)
	}
}

// generateRobotsTxt generates robots.txt
func (r *AppRunner) generateRobotsTxt(cfg config.RobotsSEOConfig) {
	rules := make([]seo.RobotsRule, len(cfg.Rules))
	for i, r := range cfg.Rules {
		rules[i] = seo.RobotsRule{
			UserAgent:  r.UserAgent,
			Allow:      r.Allow,
			Disallow:   r.Disallow,
			CrawlDelay: r.CrawlDelay,
		}
	}

	generator := seo.NewRobotsGenerator(seo.RobotsConfig{
		Enabled:    cfg.Enabled,
		OutputDir:  cfg.OutputDir,
		SitemapURL: cfg.SitemapURL,
		Rules:      rules,
	})

	outputPath := filepath.Join(cfg.OutputDir, "robots.txt")
	if err := generator.WriteToFile(outputPath); err != nil {
		logging.DefaultLogger.Warn("Failed to write robots.txt: %v", err)
		return
	}

	logging.DefaultLogger.Info("robots.txt generated: %s", outputPath)
}
