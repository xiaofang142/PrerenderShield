package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/api/routes"
	"prerender-shield/internal/auth"
	"prerender-shield/internal/cache"
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
	// 构建完整的Redis URL，包括密码和数据库索引
	redisURL := cfg.Cache.RedisURL
	if !strings.HasPrefix(redisURL, "redis://") {
		// 如果不是URL格式，转换为URL格式
		redisURL = "redis://" + redisURL
	}

	// 解析URL
	parsedURL, err := url.Parse(redisURL)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL: %v", err)
	}

	// 设置密码
	if cfg.Cache.RedisPassword != "" {
		parsedURL.User = url.UserPassword("", cfg.Cache.RedisPassword)
	}

	// 设置数据库索引
	if cfg.Cache.RedisDB > 0 {
		parsedURL.Path = fmt.Sprintf("/%d", cfg.Cache.RedisDB)
	}

	// 构建最终的Redis URL
	finalRedisURL := parsedURL.String()

	redisClient, err := redis.NewClientWithURL(finalRedisURL)
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
		} else {
			logging.DefaultLogger.Info("Successfully synced file config to Redis")
		}
	} else {
		// 重新获取更新后的配置
		cfg = configManager.GetConfig()
		logging.DefaultLogger.Info("Using sites configuration from Redis")
	}
	
	// 验证配置完整性
	if len(cfg.Sites) == 0 {
		logging.DefaultLogger.Warn("No sites configured, using fallback configuration")
		// 使用默认站点配置作为回退
		fallbackSite := config.SiteConfig{
			ID:      "fallback-site",
			Name:    "Fallback Site",
			Domains: []string{"localhost"},
			Port:    8080,
			Mode:    "static",
			Firewall: config.FirewallConfig{
				Enabled: false,
			},
			Prerender: config.PrerenderConfig{
				Enabled:  true,
				PoolSize: 3,
				Timeout:  30,
				CacheTTL: 3600,
			},
		}
		cfg.Sites = append(cfg.Sites, fallbackSite)
	}

	// 启动Redis订阅者，监听配置变更（暂时注释掉，因为还没有实现）
	// redisSubscriber := redis.NewSubscriber(redisClient.GetRawClient())
	// 添加配置变更处理
	// redisSubscriber.AddHandler("site:update", func(channel, payload string) {
	// 	log.Printf("Received site update event: %s, payload: %s", channel, payload)
	// 	// 这里可以添加站点更新逻辑
	// })
	// 启动订阅者
	// if err := redisSubscriber.Start(); err != nil {
	// 	log.Printf("Failed to start Redis subscriber: %v", err)
	// }
	// defer redisSubscriber.Stop()

	// 2. 认证模块初始化
	userManager := auth.NewUserManager(cfg.Dirs.DataDir, redisClient)
	jwtManager := auth.NewJWTManager(&auth.JWTConfig{
		SecretKey:  "prerender-shield-secret-key", // 实际项目中应该从配置文件读取
		ExpireTime: 24 * time.Hour,                // 令牌过期时间
	}, redisClient)

	// 3. 防火墙引擎管理器
	firewallManager := firewall.NewEngineManager()

	// 4. 缓存管理器初始化
	cacheManager := cache.NewManager(redisClient)

	// 5. 渲染预热引擎管理器
	prerenderManager := prerender.NewEngineManager(redisClient, cacheManager, 5)

	// 6. 爬虫日志管理器
	crawlerLogManager := logging.NewCrawlerLogManager(finalRedisURL)

	// 7. 访问日志管理器
	visitLogManager := logging.NewVisitLogManager(finalRedisURL)

	// 6.1 GeoIP服务
	geoIPService := services.NewGeoIPService("")

	// 6.2 日志处理器
	logProcessor := services.NewLogProcessor(crawlerLogManager, visitLogManager, geoIPService, configManager, redisClient.GetRawClient())
	logProcessor.Start()

	// 7. 为每个站点创建并启动引擎
	for _, site := range cfg.Sites {
		// 获取或创建站点的渲染引擎
		_, exists := prerenderManager.GetEngine(site.ID)
		if !exists {
			logging.DefaultLogger.Error("Failed to get or create engine for site %s", site.ID)
			log.Fatalf("Failed to get or create engine for site %s", site.ID)
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

	// 8. 初始化健康检查器
	healthChecker := monitoring.NewHealthChecker(redisClient)
	
	// 8.1 添加配置加载失败的回退策略
	// 如果从Redis加载站点配置失败，尝试使用文件配置并记录警告
	if err := configManager.LoadSitesFromRedis(); err != nil {
		logging.DefaultLogger.Warn("Could not load sites from Redis: %v, falling back to file config", err)
		// 保持现有的处理逻辑
		if err := configManager.SaveSitesToRedis(); err != nil {
			logging.DefaultLogger.Error("Failed to sync initial sites to Redis: %v", err)
		}
	} else {
		// 重新获取更新后的配置
		cfg = configManager.GetConfig()
		logging.DefaultLogger.Info("Using sites configuration from Redis")
	}
	
	// 8.2 初始化监控模块
	monitor := monitoring.NewMonitor(monitoring.Config{
		Enabled:           true,
		PrometheusAddress: ":9090",
	})
	if err := monitor.Start(); err != nil {
		logging.DefaultLogger.Error("Failed to start monitoring: %v", err)
		log.Fatalf("Failed to start monitoring: %v", err)
	}
	logging.DefaultLogger.Info("Monitoring service started successfully")
	
	// 8.3 启动健康检查相关服务
	go func() {
		// 定期执行健康检查
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				status := healthChecker.Check()
				isHealthy := status["healthy"].(bool)
				healthyStatus := "healthy"
				if !isHealthy {
					healthyStatus = "unhealthy"
				}
				logging.DefaultLogger.Info("Periodic health check - Overall status: %s", healthyStatus)
				
				// 如果有关键问题，记录警告
				if !isHealthy {
					for checkName, checkResult := range status {
						if checkName == "healthy" || checkName == "timestamp" || checkName == "uptime" {
							continue
						}
						resultMap, ok := checkResult.(map[string]interface{})
						if ok && !resultMap["healthy"].(bool) {
							logging.DefaultLogger.Error("Critical health issue in %s: %s", checkName, resultMap["message"].(string))
						}
					}
				}
			}
		}
	}()

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
	// 检查管理控制台静态目录
	// 强制设置AdminStaticDir为bin/web目录
	// 获取当前工作目录
	currentDir, _ := os.Getwd()
	// 获取二进制文件所在目录
	appDir := filepath.Dir(os.Args[0])
	var webDir string
	if filepath.Base(appDir) == "bin" {
		// 如果二进制文件在bin目录中，直接使用web子目录
		webDir = filepath.Join(appDir, "web")
	} else {
		// 否则使用bin/web目录
		webDir = filepath.Join(currentDir, "bin", "web")
	}
	cfg.Dirs.AdminStaticDir = webDir
	log.Printf("Admin static dir: %s", cfg.Dirs.AdminStaticDir)

	// 检查目录是否存在
	var actualStaticDir string
	if _, err := os.Stat(cfg.Dirs.AdminStaticDir); os.IsNotExist(err) {
		log.Printf("Admin static dir does not exist: %s", cfg.Dirs.AdminStaticDir)
		actualStaticDir = cfg.Dirs.AdminStaticDir
	} else {
		log.Printf("Admin static dir exists: %s", cfg.Dirs.AdminStaticDir)
		// 列出目录内容
		files, _ := os.ReadDir(cfg.Dirs.AdminStaticDir)
		log.Printf("Admin static dir contents: %v", files)

		// 检查dist目录是否在web目录下
		distDir := filepath.Join(cfg.Dirs.AdminStaticDir, "dist")
		if _, err := os.Stat(distDir); err == nil {
			log.Printf("Using dist directory for static files: %s", distDir)
			actualStaticDir = distDir
		} else {
			// 直接使用web目录
			actualStaticDir = cfg.Dirs.AdminStaticDir
		}
	}

	adminMux := http.NewServeMux()

	// 静态文件类型映射，与站点服务器保持一致
	staticExts := []string{".html", ".htm", ".css", ".js", ".jpg", ".jpeg", ".png", ".gif", ".svg", ".ico", ".webp", ".woff", ".woff2", ".ttf", ".eot", ".json"}

	// 检查文件扩展名是否为静态资源
	isStaticFile := func(path string) bool {
		ext := strings.ToLower(filepath.Ext(path))
		return slices.Contains(staticExts, ext)
	}

	// 处理静态资源请求
	adminMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 移除URL中的hash部分，支持SPA路由
		path := strings.Split(r.URL.Path, "#")[0]
		filePath := filepath.Join(actualStaticDir, strings.TrimPrefix(path, "/"))

		log.Printf("Static file request: %s -> %s", r.URL.Path, filePath)

		// 检查是否为静态资源
		if isStaticFile(filePath) {
			// 检查文件是否存在
			if _, err := os.Stat(filePath); err == nil {
				// 根据文件扩展名设置正确的MIME类型
				ext := filepath.Ext(filePath)
				switch ext {
				case ".html", ".htm":
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
				case ".css":
					w.Header().Set("Content-Type", "text/css; charset=utf-8")
				case ".js":
					w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
				case ".json":
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
				case ".png":
					w.Header().Set("Content-Type", "image/png")
				case ".jpg", ".jpeg":
					w.Header().Set("Content-Type", "image/jpeg")
				case ".gif":
					w.Header().Set("Content-Type", "image/gif")
				case ".svg":
					w.Header().Set("Content-Type", "image/svg+xml")
				case ".ico":
					w.Header().Set("Content-Type", "image/x-icon")
				case ".webp":
					w.Header().Set("Content-Type", "image/webp")
				default:
					w.Header().Set("Content-Type", "application/octet-stream")
				}

				// 返回静态文件
				http.ServeFile(w, r, filePath)
				return
			}
		}

		// SPA路由处理：非静态资源请求返回index.html
		indexPath := filepath.Join(actualStaticDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			// 设置正确的Content-Type
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			// 返回index.html
			http.ServeFile(w, r, indexPath)
			return
		}

		// 404处理
		http.NotFound(w, r)
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

// responseRecorder 响应记录器，用于捕获状态码
type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
