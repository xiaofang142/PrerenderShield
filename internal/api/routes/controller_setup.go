package routes

import (
	"prerender-shield/internal/api/controllers"
	"prerender-shield/internal/audit"
	"prerender-shield/internal/auth"
	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/prerender"
	"prerender-shield/internal/prerender/push"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/repository"
	"prerender-shield/internal/scheduler"
	sitehandler "prerender-shield/internal/site-handler"
	siteserver "prerender-shield/internal/site-server"
	"prerender-shield/internal/ssl"
)

// Controllers 包含所有 API 控制器实例
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
	SSLController        *controllers.SSLController
	SEOController        *controllers.SEOController
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
	wafRepo *repository.WafRepository,
	auditLogger *audit.Logger,
	cfg *config.Config,
) *Controllers {
	// 创建推送管理器
	pushManager := push.NewPushManager(cfg, redisClient)
	pushManager.SetConfigProvider(func() *config.Config { return configManager.GetConfig() })

	// 创建 SSL 管理器和 ACME 客户端
	sslConfig := ssl.ACMEConfig{
		Email:      cfg.SSL.Email,
		CertDir:    cfg.Dirs.CertsDir,
		Production: cfg.SSL.Production,
		HTTPPort:   cfg.SSL.HTTPPort,
	}

	var acmeClient *ssl.ACMEClient
	var autoRenewer *ssl.AutoRenewer

	// 如果 SSL 启用，则初始化 ACME 客户端
	if cfg.SSL.Enabled {
		var err error
		acmeClient, err = ssl.NewACMEClient(sslConfig)
		if err != nil {
			logging.DefaultLogger.Warn("Failed to initialize ACME client: %v", err)
		} else {
			logging.DefaultLogger.Info("ACME client initialized successfully")

			// 如果配置了 DNS 提供商，设置 DNS 挑战
			if cfg.SSL.DNS.Provider != "" {
				err = acmeClient.SetDNSProvider(cfg.SSL.DNS.Provider, cfg.SSL.DNS.Credentials)
				if err != nil {
					logging.DefaultLogger.Warn("Failed to set DNS provider: %v", err)
				}
			}

			// 创建自动续签器
			autoRenewConfig := ssl.AutoRenewConfig{
				Enabled:         cfg.SSL.AutoRenew,
				CheckInterval:   cfg.SSL.CheckInterval,
				RenewBeforeDays: cfg.SSL.RenewBeforeDays,
				MaxRetries:      cfg.SSL.MaxRetries,
				RetryDelay:      cfg.SSL.RetryDelay,
				WebhookURL:      cfg.SSL.WebhookURL,
			}
			autoRenewer = ssl.NewAutoRenewer(acmeClient, redisClient, autoRenewConfig)
			autoRenewer.Start()
		}
	}

	// 创建 2FA 管理器
	twoFactorAuth := auth.NewTwoFactorAuth(redisClient, "Prerender Shield")

	// 创建 SSL 管理器（用于概览页面展示证书状态）
	var sslMgr ssl.Manager
	if mgr, err := ssl.NewManager(redisClient, cfg.Dirs.CertsDir, cfg.SSL.Email, cfg.SSL.Production); err == nil {
		sslMgr = mgr
	} else {
		logging.DefaultLogger.Warn("Failed to create SSL manager: %v", err)
	}

	// 创建 SSL 控制器 — 没有 ACME 时传 nil（接口 nil 而非类型 nil）
	var sslController *controllers.SSLController
	if cfg.SSL.Enabled && acmeClient != nil {
		sslController = controllers.NewSSLController(acmeClient, autoRenewer)
	} else {
		sslController = controllers.NewSSLController(nil, nil)
	}

	// 创建控制器实例
	// configProvider：每请求返回最新配置。Mutate 为 copy-on-write 换指针，
	// 若控制器持有启动快照，站点增删改后其视图将永久陈旧（缺陷：clear-cache 报 Site not found）。
	configProvider := func() *config.Config { return configManager.GetConfig() }
	controllersSet := &Controllers{
		AuthController:       controllers.NewAuthController(userManager, jwtManager, auditLogger, twoFactorAuth),
		OverviewController:   controllers.NewOverviewController(cfg, monitor, visitLogMgr, crawlerLogMgr, wafRepo, sslMgr),
		MonitoringController: controllers.NewMonitoringController(monitor, redisClient),
		FirewallController:   controllers.NewFirewallController(wafRepo),
		CrawlerController:    controllers.NewCrawlerController(crawlerLogMgr),
		PreheatController:    controllers.NewPreheatController(prerenderManager, redisClient, scheduler, cfg),
		PushController:       controllers.NewPushController(pushManager, redisClient, cfg),
		// WithConcreteDeps：同时设置 concreteCrawlerLogMgr 等具体依赖——
		// 站点增改后经 CreateSiteHandler 重建的处理器靠它记录爬虫/访问日志；
		// 旧 NewSitesController 不设具体依赖，控制台建站的站点日志永远为空（实测发现的缺陷）
		SitesController:  controllers.NewSitesControllerWithConcreteDeps(configManager, siteServerMgr, siteHandler, redisClient, monitor, crawlerLogMgr, visitLogMgr, cfg),
		SystemController: controllers.NewSystemController(redisClient),
		SSLController:    sslController,
		SEOController:    controllers.NewSEOController(cfg),
	}
	controllersSet.OverviewController.SetConfigProvider(configProvider)
	controllersSet.PreheatController.SetConfigProvider(configProvider)
	controllersSet.PushController.SetConfigProvider(configProvider)
	controllersSet.PushController.SetSitesController(controllersSet.SitesController)
	controllersSet.SitesController.SetConfigProvider(configProvider)
	controllersSet.SEOController.SetConfigProvider(configProvider)
	return controllersSet
}
