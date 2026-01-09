package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/api/routes"
	"prerender-shield/internal/auth"
	"prerender-shield/internal/config"
	"prerender-shield/internal/firewall"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/prerender"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/repository"
	"prerender-shield/internal/scheduler"
	"prerender-shield/internal/services"
	sitehandler "prerender-shield/internal/site-handler"
	siteserver "prerender-shield/internal/site-server"
)

func main() {
	// 解析命令行参数
	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to the YAML configuration file")
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 获取配置管理器实例
	configManager := config.GetInstance()

	// 启动配置文件监控
	if err := configManager.StartWatching(); err != nil {
		log.Printf("Failed to start config watching: %v", err)
	} else {
		log.Println("Config watching started")
	}

	// 添加配置变化处理函数
	configManager.AddConfigChangeHandler(func(newConfig *config.Config) {
		logging.DefaultLogger.Info("Config updated, reloading services...")
		// 记录配置变更审计日志
		logging.DefaultLogger.LogAdminAction("system", "localhost", "config_update", "global_config", map[string]interface{}{"source": "config_file"}, "success", "Configuration updated from file")
		// 这里可以添加需要重新加载的服务逻辑
		// 例如：重新初始化防火墙规则、渲染预热引擎等
		logging.DefaultLogger.Info("Services reloaded successfully")
	})

	// 6. 初始化各模块
	// 1. Redis客户端初始化
	redisClient, err := redis.NewClient(cfg.Cache.RedisURL)
	if err != nil {
		log.Fatalf("Failed to initialize Redis client: %v", err)
	}

	// 0.1 初始化WAF仓库
	wafRepo := repository.NewWafRepository(redisClient)

	// 注入Redis客户端到配置管理器
	configManager.SetRedisClient(redisClient)

	// 从Redis加载站点配置
	// 如果Redis中有配置，将覆盖文件配置
	if err := configManager.LoadSitesFromRedis(); err != nil {
		// 如果加载失败（可能是key不存在），则将当前文件配置同步到Redis
		logging.DefaultLogger.Info("Could not load sites from Redis (first run?), syncing file config to Redis: %v", err)
		if err := configManager.SaveSitesToRedis(); err != nil {
			logging.DefaultLogger.Error("Failed to sync initial sites to Redis: %v", err)
		}
	} else {
		// 重新获取更新后的配置
		cfg = configManager.GetConfig()
		logging.DefaultLogger.Info("Using sites configuration from Redis")
	}

	// 启动Redis订阅者，监听配置变更
	redisSubscriber := redis.NewSubscriber(redisClient.GetRawClient())
	// 添加配置变更处理
	redisSubscriber.AddHandler("site:update", func(channel, payload string) {
		log.Printf("Received site update event: %s, payload: %s", channel, payload)
		// 这里可以添加站点更新逻辑
	})
	// 启动订阅者
	if err := redisSubscriber.Start(); err != nil {
		log.Printf("Failed to start Redis subscriber: %v", err)
	}
	defer redisSubscriber.Stop()

	// 2. 认证模块初始化
	userManager := auth.NewUserManager(cfg.Dirs.DataDir, redisClient)
	jwtManager := auth.NewJWTManager(&auth.JWTConfig{
		SecretKey:  "prerender-shield-secret-key", // 实际项目中应该从配置文件读取
		ExpireTime: 24 * time.Hour,                // 令牌过期时间
	}, redisClient)

	// 3. 防火墙引擎管理器
	firewallManager := firewall.NewEngineManager()

	// 4. 渲染预热引擎管理器
	prerenderManager := prerender.NewEngineManager(cfg.Dirs.StaticDir)

	// 5. 爬虫日志管理器
	crawlerLogManager := logging.NewCrawlerLogManager(cfg.Cache.RedisURL)

	// 6. 访问日志管理器
	visitLogManager := logging.NewVisitLogManager(cfg.Cache.RedisURL)

	// 6.1 GeoIP服务
	geoIPService := services.NewGeoIPService("")

	// 6.2 日志处理器
	logProcessor := services.NewLogProcessor(crawlerLogManager, visitLogManager, geoIPService, configManager, redisClient.GetRawClient())
	logProcessor.Start()

	// 7. 为每个站点创建并启动引擎
	for _, site := range cfg.Sites {
		// 将 config.PrerenderConfig 转换为 prerender.PrerenderConfig
		prerenderConfig := prerender.PrerenderConfig{
			Enabled:           site.Prerender.Enabled,
			PoolSize:          site.Prerender.PoolSize,
			MinPoolSize:       site.Prerender.MinPoolSize,
			MaxPoolSize:       site.Prerender.MaxPoolSize,
			Timeout:           site.Prerender.Timeout,
			CacheTTL:          site.Prerender.CacheTTL,
			CrawlerHeaders:    site.Prerender.CrawlerHeaders,
			UseDefaultHeaders: site.Prerender.UseDefaultHeaders,
			Preheat: prerender.PreheatConfig{
				Enabled:  site.Prerender.Preheat.Enabled,
				MaxDepth: site.Prerender.Preheat.MaxDepth,
			},
		}

		// 将引擎添加到管理器
		// AddSite 方法会自动创建并启动引擎
		if err := prerenderManager.AddSite(site.ID, prerenderConfig, redisClient); err != nil {
			logging.DefaultLogger.Error("Failed to add site to prerender manager: %v", err)
			log.Fatalf("Failed to add site to prerender manager: %v", err)
		}
		logging.DefaultLogger.Info("Prerender engine started successfully for site %s (ID: %s)", site.Name, site.ID)

		// 创建防火墙引擎
		if err := firewallManager.AddSite(site.Name, firewall.Config{
			RulesPath: site.Firewall.RulesPath,
			ActionConfig: firewall.ActionConfig{
				DefaultAction: site.Firewall.ActionConfig.DefaultAction,
				BlockMessage:  site.Firewall.ActionConfig.BlockMessage,
			},
			StaticDir:           cfg.Dirs.StaticDir,
			GeoIPConfig:         &site.Firewall.GeoIPConfig,
			RateLimitConfig:     &site.Firewall.RateLimitConfig,
			FileIntegrityConfig: &site.FileIntegrityConfig,
			Blacklist:           site.Firewall.Blacklist,
			Whitelist:           site.Firewall.Whitelist,
			RedisClient:         redisClient.GetRawClient(),
		}); err != nil {
			logging.DefaultLogger.Error("Failed to initialize firewall engine for site %s: %v", site.Name, err)
			log.Fatalf("Failed to initialize firewall engine for site %s: %v", site.Name, err)
		}
		logging.DefaultLogger.Info("Firewall engine initialized successfully for site %s", site.Name)
	}

	// 记录站点数量
	logging.DefaultLogger.Info("Initialized %d sites", len(cfg.Sites))

	// 5. 定时任务调度器初始化
	schedulerInstance := scheduler.NewScheduler(prerenderManager, redisClient, cfg)
	schedulerInstance.Start()
	defer schedulerInstance.Stop()

	// 8. 初始化监控模块
	monitor := monitoring.NewMonitor(monitoring.Config{
		Enabled:           true,
		PrometheusAddress: ":9090",
	})
	if err := monitor.Start(); err != nil {
		logging.DefaultLogger.Error("Failed to start monitoring: %v", err)
		log.Fatalf("Failed to start monitoring: %v", err)
	}
	logging.DefaultLogger.Info("Monitoring service started successfully")

	// 9. 初始化站点服务器管理器
	siteServerManager := siteserver.NewManager(monitor)

	// 10. 初始化站点处理器
	siteHandler := sitehandler.NewHandler(prerenderManager, wafRepo, redisClient, geoIPService)

	// 11. 为每个站点启动服务器
	for _, site := range cfg.Sites {
		// 创建站点处理器
		siteHTTPHandler := siteHandler.CreateSiteHandler(site, crawlerLogManager, visitLogManager, monitor, cfg.Dirs.StaticDir)
		// 启动站点服务器
		siteServerManager.StartSiteServer(site, cfg.Server.Address, cfg.Dirs.StaticDir, crawlerLogManager, siteHTTPHandler)
		log.Printf("站点服务器启动成功: %s (%s:%d)", site.Name, cfg.Server.Address, site.Port)
	}

	// 13. 初始化Gin路由
	ginRouter := gin.Default()

	// 14. 初始化API路由器
	apiRouter := routes.NewRouter(
		userManager,
		jwtManager,
		configManager,
		prerenderManager,
		redisClient,
		schedulerInstance,
		siteServerManager,
		siteHandler,
		monitor,
		crawlerLogManager,
		visitLogManager,
		wafRepo,
		cfg,
	)

	// 14. 注册API路由
	apiRouter.RegisterRoutes(ginRouter)

	// 15. 启动主API服务器
	apiServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Address, cfg.Server.APIPort),
		Handler: ginRouter,
	}

	// 16. 启动API服务器
	go func() {
		log.Printf("API server starting on %s:%d", cfg.Server.Address, cfg.Server.APIPort)
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start API server: %v", err)
		}
	}()

	// 17. 启动管理控制台服务器
	// 只在发行阶段启动管理控制台静态资源服务器，开发阶段（go run）不启动
	// 检查是否是在go run模式下运行
	exePath, _ := os.Executable()
	isDevMode := strings.Contains(exePath, "/var/folders") || strings.Contains(exePath, "/tmp") || strings.Contains(exePath, "T\\go-build")

	// 创建静态文件服务器，用于提供管理控制台前端页面
	log.Printf("Admin static dir: %s", cfg.Dirs.AdminStaticDir)
	// 检查目录是否存在
	if _, err := os.Stat(cfg.Dirs.AdminStaticDir); os.IsNotExist(err) {
		log.Printf("Admin static dir does not exist: %s", cfg.Dirs.AdminStaticDir)
	} else {
		log.Printf("Admin static dir exists: %s", cfg.Dirs.AdminStaticDir)
		// 列出目录内容
		files, _ := os.ReadDir(cfg.Dirs.AdminStaticDir)
		log.Printf("Admin static dir contents: %v", files)
	}

	adminMux := http.NewServeMux()

	// 简化的SPA路由处理
	adminMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 检查请求是否指向一个具体的文件
		filePath := path.Join(cfg.Dirs.AdminStaticDir, r.URL.Path)

		// 如果文件不存在，返回index.html
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			// 记录日志
			log.Printf("File not found, serving index.html: %s", r.URL.Path)
			// 返回index.html
			http.ServeFile(w, r, path.Join(cfg.Dirs.AdminStaticDir, "index.html"))
			return
		}

		// 如果文件存在，使用默认的文件服务器处理
		http.FileServer(http.Dir(cfg.Dirs.AdminStaticDir)).ServeHTTP(w, r)
	})
	// 启动管理控制台服务器
	adminServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Address, cfg.Server.ConsolePort),
		Handler: adminMux,
	}

	go func() {
		log.Printf("Admin console server starting on %s:%d", cfg.Server.Address, cfg.Server.ConsolePort)
		if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start admin console server: %v", err)
		}
	}()

	// 18. 优雅关闭管理控制台服务器
	defer func() {
		if err := adminServer.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down admin console server: %v", err)
		}
	}()

	if isDevMode {
		log.Printf("Running in development mode, admin console server started from static files")
		log.Printf("Use 'npm run dev' in web directory for hot-reload development")
	}

	// 16. 处理信号，优雅关闭服务
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	log.Println("Server started successfully, waiting for signals...")
	<-quit

	log.Println("Shutting down server...")

	// 17. 关闭API服务器
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := apiServer.Shutdown(ctx); err != nil {
		log.Fatalf("API server forced to shutdown: %v", err)
	}

	// 18. 关闭站点服务器
	siteServerManager.StopAllServers()

	log.Println("Server exited")
}
