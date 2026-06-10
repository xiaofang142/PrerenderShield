package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/api/routes"
	"prerender-shield/internal/config"
	"prerender-shield/internal/di"
	"prerender-shield/internal/firewall"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/logging"
)

// AppRunner 应用运行器
type AppRunner struct {
	app         *Application
	config      *config.Config
	redisClient *redis.Client
	container   *di.Container
}

// NewAppRunner 创建应用运行器
func NewAppRunner(app *Application) *AppRunner {
	return &AppRunner{
		app: app,
	}
}

// Initialize 初始化所有模块
func (r *AppRunner) Initialize(ctx context.Context) error {
	r.config = r.app.GetConfig()
	r.redisClient = r.app.GetRedis()

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

	// 启动监控
	if err := c.Monitor.Start(); err != nil {
		return fmt.Errorf("start monitoring: %w", err)
	}

	// 启动调度器
	c.Scheduler.Start()

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

	// 获取或创建渲染引擎
	if _, exists := c.PrerenderMgr.GetEngine(site.ID); !exists {
		return fmt.Errorf("failed to get engine for site %s", site.ID)
	}

	// 创建防火墙引擎
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
	)

	apiRouter.RegisterRoutes(ginRouter)

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
	// 确定静态目录
	currentDir, _ := os.Getwd()
	appDir := filepath.Dir(os.Args[0])
	var webDir string
	if filepath.Base(appDir) == "bin" {
		webDir = filepath.Join(appDir, "web")
	} else {
		webDir = filepath.Join(currentDir, "bin", "web")
	}

	actualStaticDir := webDir
	if _, err := os.Stat(webDir); err == nil {
		distDir := filepath.Join(webDir, "dist")
		if _, err := os.Stat(distDir); err == nil {
			actualStaticDir = distDir
		}
	}

	staticExts := []string{".html", ".htm", ".css", ".js", ".jpg", ".jpeg", ".png", ".gif", ".svg", ".ico", ".webp", ".woff", ".woff2", ".ttf", ".eot", ".json"}

	isStaticFile := func(path string) bool {
		ext := strings.ToLower(filepath.Ext(path))
		return slices.Contains(staticExts, ext)
	}

	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.Split(r.URL.Path, "#")[0]
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

	// 注册容器清理
	app.AddCleanup(func() {
		if runner.container != nil {
			runner.container.Close()
		}
	})

	// 启动服务
	if err := runner.Start(ctx); err != nil {
		return err
	}

	// 运行
	return app.Run(ctx)
}
