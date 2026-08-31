package routes

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/audit"
	"prerender-shield/internal/auth"
	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/middleware"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/prerender"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/repository"
	"prerender-shield/internal/scheduler"
	sitehandler "prerender-shield/internal/site-handler"
	siteserver "prerender-shield/internal/site-server"
	"prerender-shield/internal/utils"
	"prerender-shield/internal/websocket"
)

// Router API路由器，负责注册所有API路由
type Router struct {
	userManager      *auth.UserManager
	jwtManager       *auth.JWTManager
	configManager    *config.ConfigManager
	prerenderManager *prerender.EngineManager
	redisClient      *redis.Client
	scheduler        *scheduler.Scheduler
	siteServerMgr    *siteserver.Manager
	siteHandler      *sitehandler.Handler
	monitor          *monitoring.Monitor
	crawlerLogMgr    *logging.CrawlerLogManager
	visitLogMgr      *logging.VisitLogManager
	wafRepo          *repository.WafRepository
	cfg              *config.Config
	auditLogger      *audit.Logger
	wsHub            *websocket.Hub
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
	visitLogMgr *logging.VisitLogManager,
	wafRepo *repository.WafRepository,
	auditLogger *audit.Logger,
	cfg *config.Config,
	wsHub *websocket.Hub,
) *Router {
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
		visitLogMgr:      visitLogMgr,
		wafRepo:          wafRepo,
		auditLogger:      auditLogger,
		cfg:              cfg,
		wsHub:            wsHub,
	}
}

// RegisterRoutes 注册所有API路由
func (r *Router) RegisterRoutes(ginRouter *gin.Engine) {
	// 添加全局错误处理中间件 (Recovery)
	ginRouter.Use(gin.Recovery())                  // Gin自带的Recovery
	ginRouter.Use(middleware.GlobalErrorHandler()) // 自定义的错误处理，虽然Gin自带了，但我们可以自定义响应格式

	// 添加安全头中间件
	addSecurityHeaders(ginRouter)

	// R14-BUG-3 修复：CORS/WS 白名单此前硬编码 9597/3000，与可配置的 console_port
	// 脱节——隔离实例(19597)等自定义端口部署时浏览器 Origin 一律被拒，WebSocket
	// 握手 "origin not allowed"。按本进程实际端口动态注入允许来源。
	utils.AddDynamicOrigin(fmt.Sprintf("http://localhost:%d", r.cfg.Server.ConsolePort))
	utils.AddDynamicOrigin(fmt.Sprintf("http://127.0.0.1:%d", r.cfg.Server.ConsolePort))
	utils.AddDynamicOrigin(fmt.Sprintf("http://localhost:%d", r.cfg.Server.APIPort))
	utils.AddDynamicOrigin(fmt.Sprintf("http://127.0.0.1:%d", r.cfg.Server.APIPort))

	// 添加CORS中间件
	addCorsMiddleware(ginRouter)

	// 设置控制器
	controllers := SetupControllers(
		r.userManager,
		r.jwtManager,
		r.configManager,
		r.prerenderManager,
		r.redisClient,
		r.scheduler,
		r.siteServerMgr,
		r.siteHandler,
		r.monitor,
		r.crawlerLogMgr,
		r.visitLogMgr,
		r.wafRepo,
		r.auditLogger,
		r.cfg,
	)

	// WebSocket Hub 由 DI 装配方（bootstrap runner）创建并注入，Router 只负责注册端点
	structuredLogger := logging.NewStructuredLogger(logging.INFO, "")
	if r.wsHub == nil {
		r.wsHub = websocket.NewHub(structuredLogger)
		go r.wsHub.Run()
	}

	// 管理 API 速率限制（必须在路由注册前 Use，否则 Gin 不会将其附加到已注册路由）
	if r.redisClient != nil {
		mgmtRateLimiter := middleware.NewRedisRateLimiter(
			r.redisClient,
			100,  // 每窗口 100 次请求
			60,   // 60 秒窗口
			5*60, // 超限后封禁 5 分钟
		)
		ginRouter.Use(middleware.ManagementRateLimit(mgmtRateLimiter))
	}

	// 管理 API Token 提供器：静态 YAML 配置 + Redis system:config 动态管理（Settings 页生成/吊销）合并
	apiTokenProvider := func() []string {
		hashes := append([]string{}, r.cfg.APITokens...)
		if r.redisClient != nil {
			if sc, err := r.redisClient.GetSystemConfig(); err == nil {
				if raw := sc["api_tokens"]; raw != "" {
					var dynamic []string
					if json.Unmarshal([]byte(raw), &dynamic) == nil {
						hashes = append(hashes, dynamic...)
					}
				}
			}
		}
		return hashes
	}

	// 注册路由
	RegisterAllRoutes(ginRouter, controllers, r.jwtManager, apiTokenProvider)

	// 注册 WebSocket 路由（需要 JWT 认证）
	wsGroup := ginRouter.Group("/ws")
	wsGroup.Use(auth.JWTAuthMiddleware(r.jwtManager, nil))
	{
		wsGroup.GET("/realtime", websocket.HandleWebSocket(r.wsHub, structuredLogger))
	}
}

// GetHub 获取 WebSocket Hub（用于外部模块接入实时广播）
func (r *Router) GetHub() *websocket.Hub {
	return r.wsHub
}
