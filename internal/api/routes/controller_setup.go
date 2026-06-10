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
	var twoFactorAuth *auth.TwoFactorAuth
	if cfg.SSL.Enabled || true { // 默认启用 2FA（可配置）
		twoFactorAuth = auth.NewTwoFactorAuth(redisClient, "Prerender Shield")
	}

	// 创建 SSL 控制器 — 没有 ACME 时传 nil（接口 nil 而非类型 nil）
	var sslController *controllers.SSLController
	if cfg.SSL.Enabled && acmeClient != nil {
		sslController = controllers.NewSSLController(acmeClient, autoRenewer)
	} else {
		sslController = controllers.NewSSLController(nil, nil)
	}

	// 创建控制器实例
	return &Controllers{
		AuthController:       controllers.NewAuthController(userManager, jwtManager, auditLogger, twoFactorAuth),
		OverviewController:   controllers.NewOverviewController(cfg, monitor, visitLogMgr, crawlerLogMgr, wafRepo),
		MonitoringController: controllers.NewMonitoringController(monitor),
		FirewallController:   controllers.NewFirewallController(wafRepo),
		CrawlerController:    controllers.NewCrawlerController(crawlerLogMgr),
		PreheatController:    controllers.NewPreheatController(prerenderManager, redisClient, cfg),
		PushController:       controllers.NewPushController(pushManager, redisClient, cfg),
		SitesController:      controllers.NewSitesController(configManager, siteServerMgr, siteHandler, redisClient, monitor, crawlerLogMgr, visitLogMgr, cfg),
		SystemController:     controllers.NewSystemController(redisClient),
		SSLController:        sslController,
	}
}
