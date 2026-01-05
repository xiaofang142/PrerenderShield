package routes

import (
	"archive/zip"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/api/controllers"
	"prerender-shield/internal/auth"
	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/prerender"
	"prerender-shield/internal/prerender/push"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/scheduler"
	sitehandler "prerender-shield/internal/site-handler"
	siteserver "prerender-shield/internal/site-server"
)

// Router API路由器，负责注册所有API路由
type Router struct {
	userManager      *auth.UserManager
	jwtManager       *auth.JWTManager
	configManager    *config.ConfigManager
	prerenderManager *prerender.EngineManager
	firewallManager  *auth.UserManager
	redisClient      *redis.Client
	scheduler        *scheduler.Scheduler
	siteServerMgr    *siteserver.Manager
	siteHandler      *sitehandler.Handler
	monitor          *monitoring.Monitor
	crawlerLogMgr    *logging.CrawlerLogManager
	cfg              *config.Config
	pushManager      *push.PushManager
}

// NewRouter 创建API路由器实例
func NewRouter(
	userManager *auth.UserManager,
	jwtManager *auth.JWTManager,
	configManager *config.ConfigManager,
	prerenderManager *prerender.EngineManager,
	redisClient *redis.Client,
	scheduler *scheduler.Scheduler,
	siteServerMgr *siteserver.Manager,
	siteHandler *sitehandler.Handler,
	monitor *monitoring.Monitor,
	crawlerLogMgr *logging.CrawlerLogManager,
	cfg *config.Config,
) *Router {
	// 创建推送管理器
	pushManager := push.NewPushManager(cfg, redisClient)

	return &Router{
		userManager:      userManager,
		jwtManager:       jwtManager,
		configManager:    configManager,
		prerenderManager: prerenderManager,
		redisClient:      redisClient,
		scheduler:        scheduler,
		siteServerMgr:    siteServerMgr,
		siteHandler:      siteHandler,
		monitor:          monitor,
		crawlerLogMgr:    crawlerLogMgr,
		cfg:              cfg,
		pushManager:      pushManager,
	}
}

// RegisterRoutes 注册所有API路由
func (r *Router) RegisterRoutes(ginRouter *gin.Engine) {
	// 添加安全头中间件
	ginRouter.Use(func(c *gin.Context) {
		// Content-Security-Policy (CSP) 头，防止XSS攻击，允许跨域请求
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self' http://localhost:5173 http://localhost:9598")

		// X-Frame-Options 头，防止Clickjacking攻击
		c.Header("X-Frame-Options", "DENY")

		// X-XSS-Protection 头，启用浏览器的XSS过滤
		c.Header("X-XSS-Protection", "1; mode=block")

		// X-Content-Type-Options 头，防止MIME类型嗅探
		c.Header("X-Content-Type-Options", "nosniff")

		// Referrer-Policy 头，控制Referrer信息的发送
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Strict-Transport-Security (HSTS) 头，强制使用HTTPS
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		// Permissions-Policy 头，控制浏览器API的访问
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=(), usb=(), accelerometer=(), gyroscope=()")

		c.Next()
	})

	// 添加CORS中间件
	ginRouter.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})

	// 创建控制器实例
	authController := controllers.NewAuthController(r.userManager, r.jwtManager)
	overviewController := controllers.NewOverviewController(r.cfg, r.monitor)
	monitoringController := controllers.NewMonitoringController(r.monitor)
	firewallController := controllers.NewFirewallController()
	crawlerController := controllers.NewCrawlerController(r.crawlerLogMgr)
	preheatController := controllers.NewPreheatController(r.prerenderManager, r.redisClient, r.cfg)
	pushController := controllers.NewPushController(r.pushManager, r.redisClient, r.cfg)
	sitesController := controllers.NewSitesController(r.configManager, r.siteServerMgr, r.siteHandler, r.redisClient, r.monitor, r.crawlerLogMgr, r.cfg)
	systemController := controllers.NewSystemController()

	// 注册API路由
	apiGroup := ginRouter.Group("/api/v1")
	{
		// 认证相关API - 不需要JWT验证
		authGroup := apiGroup.Group("/auth")
		{
			// 检查是否是首次运行
			authGroup.GET("/first-run", authController.CheckFirstRun)

			// 用户登录
			authGroup.POST("/login", authController.Login)

			// 用户退出登录
			authGroup.POST("/logout", authController.Logout)
		}

		// 系统相关API - 不需要JWT验证
		apiGroup.GET("/health", systemController.Health)
		apiGroup.GET("/version", systemController.Version)

		// 需要JWT验证的API组
		protectedGroup := apiGroup.Group("/")
		protectedGroup.Use(auth.JWTAuthMiddleware(r.jwtManager))
		{
			// 概览API
			protectedGroup.GET("/overview", overviewController.GetOverview)

			// 监控API
			protectedGroup.GET("/monitoring/stats", monitoringController.GetStats)
			// 防火墙API
			protectedGroup.GET("/firewall/status", firewallController.GetFirewallStatus)

			// 防火墙规则API
			protectedGroup.GET("/firewall/rules", firewallController.GetFirewallRules)

			// 爬虫日志API
			protectedGroup.GET("/crawler/logs", crawlerController.GetCrawlerLogs)
			protectedGroup.GET("/crawler/stats", crawlerController.GetCrawlerStats)

			// 预热API
			protectedGroup.GET("/preheat/sites", preheatController.GetPreheatSites)
			protectedGroup.GET("/preheat/stats", preheatController.GetPreheatStats)
			protectedGroup.POST("/preheat/trigger", preheatController.TriggerPreheat)
			protectedGroup.GET("/preheat/urls", preheatController.GetPreheatUrls)
			protectedGroup.GET("/preheat/task/status", preheatController.GetPreheatTaskStatus)
			protectedGroup.GET("/preheat/crawler-headers", preheatController.GetCrawlerHeaders)

			// 推送API
			protectedGroup.GET("/push/sites", pushController.GetSites)
			protectedGroup.GET("/push/stats", pushController.GetPushStats)
			protectedGroup.GET("/push/logs", pushController.GetPushLogs)
			protectedGroup.POST("/push/trigger", pushController.TriggerPush)
			protectedGroup.GET("/push/config", pushController.GetPushConfig)
			protectedGroup.POST("/push/config", pushController.UpdatePushConfig)

			// 站点管理API
			sitesGroup := protectedGroup.Group("/sites")
			{
				// 获取站点列表
				sitesGroup.GET("", sitesController.GetSites)

				// 获取单个站点信息
				sitesGroup.GET("/:id", sitesController.GetSite)

				// 添加站点
				sitesGroup.POST("", sitesController.AddSite)

				// 更新站点
				sitesGroup.PUT("/:id", sitesController.UpdateSite)

				// 删除站点
				sitesGroup.DELETE("/:id", sitesController.DeleteSite)

				// 静态资源管理API
				// 获取站点的静态资源文件列表
				sitesGroup.GET("/:id/static", sitesController.GetStaticFiles)

				// 上传静态资源文件
				sitesGroup.POST("/:id/static", sitesController.UploadStaticFile)

				// 解压文件
				sitesGroup.POST("/:id/static/extract", sitesController.ExtractFile)

				// 删除静态资源文件
				sitesGroup.DELETE("/:id/static", sitesController.DeleteStaticFile)
			}
		}
	}
}

// 检查端口是否可用
func isPortAvailable(port int) bool {
	// 常用互联网端口列表，这些端口将被排除
	reservedPorts := map[int]bool{
		// 常用服务端口
		21:  true, // FTP
		22:  true, // SSH
		23:  true, // Telnet
		25:  true, // SMTP
		53:  true, // DNS
		80:  true, // HTTP
		110: true, // POP3
		143: true, // IMAP
		443: true, // HTTPS
		465: true, // SMTPS
		587: true, // SMTP (STARTTLS)
		993: true, // IMAPS
		995: true, // POP3S

		// 常用应用端口
		3306:  true, // MySQL
		5432:  true, // PostgreSQL
		6379:  true, // Redis
		8080:  true, // Tomcat
		9000:  true, // PHP-FPM
		9090:  true, // Prometheus
		15672: true, // RabbitMQ
		27017: true, // MongoDB
	}

	// 检查是否是保留端口
	if reservedPorts[port] {
		return false
	}

	// 尝试监听端口
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	defer listener.Close()

	return true
}

// ExtractZIP 解压ZIP文件，导出供测试使用
func ExtractZIP(filePath, destDir string) error {
	// 打开ZIP文件
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	// 确保目标目录存在
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// 遍历ZIP文件中的所有文件
	for _, file := range reader.File {
		// 构建目标文件路径
		destFilePath := filepath.Join(destDir, file.Name)

		// 检查文件是否是目录
		if file.FileInfo().IsDir() {
			// 创建目录
			if err := os.MkdirAll(destFilePath, file.Mode()); err != nil {
				return err
			}
			continue
		}

		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(destFilePath), 0755); err != nil {
			return err
		}

		// 创建目标文件
		destFile, err := os.OpenFile(destFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}
		// 不使用defer，而是立即关闭文件

		// 获取ZIP文件中的文件
		zipFile, err := file.Open()
		if err != nil {
			destFile.Close() // 确保文件关闭
			return err
		}

		// 复制文件内容
		if _, err := io.Copy(destFile, zipFile); err != nil {
			zipFile.Close()
			destFile.Close() // 确保文件关闭
			return err
		}

		// 立即关闭文件，避免资源泄漏和文件锁定问题
		zipFile.Close()
		destFile.Close()
	}

	return nil
}
