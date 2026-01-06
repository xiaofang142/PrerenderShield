package routes

import (
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

// Controllers 包含所有API控制器实例
type Controllers struct {
	AuthController       *controllers.AuthController
	OverviewController   *controllers.OverviewController
	MonitoringController *controllers.MonitoringController
	FirewallController   *controllers.FirewallController
	CrawlerController    *controllers.CrawlerController
	PreheatController    *controllers.PreheatController
	PushController       *controllers.PushController
	SitesController      *controllers.SitesController
	SystemController     *controllers.SystemController
}

// SetupControllers 创建并配置所有控制器实例
func SetupControllers(
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
	cfg *config.Config,
) *Controllers {
	// 创建推送管理器
	pushManager := push.NewPushManager(cfg, redisClient)

	// 创建控制器实例
	return &Controllers{
		AuthController:       controllers.NewAuthController(userManager, jwtManager),
		OverviewController:   controllers.NewOverviewController(cfg, monitor, visitLogMgr),
		MonitoringController: controllers.NewMonitoringController(monitor),
		FirewallController:   controllers.NewFirewallController(),
		CrawlerController:    controllers.NewCrawlerController(crawlerLogMgr),
		PreheatController:    controllers.NewPreheatController(prerenderManager, redisClient, cfg),
		PushController:       controllers.NewPushController(pushManager, redisClient, cfg),
		SitesController:      controllers.NewSitesController(configManager, siteServerMgr, siteHandler, redisClient, monitor, crawlerLogMgr, visitLogMgr, cfg),
		SystemController:     controllers.NewSystemController(redisClient),
	}
}