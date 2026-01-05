package routes

import (
	"github.com/gin-gonic/gin"

	"prerender-shield/internal/auth"
	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/prerender"
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
	redisClient      *redis.Client
	scheduler        *scheduler.Scheduler
	siteServerMgr    *siteserver.Manager
	siteHandler      *sitehandler.Handler
	monitor          *monitoring.Monitor
	crawlerLogMgr    *logging.CrawlerLogManager
	cfg              *config.Config
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
	}
}

// RegisterRoutes 注册所有API路由
func (r *Router) RegisterRoutes(ginRouter *gin.Engine) {
	// 添加安全头中间件
	addSecurityHeaders(ginRouter)

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
		r.cfg,
	)

	// 注册路由
	RegisterAllRoutes(ginRouter, controllers, r.jwtManager)
}
