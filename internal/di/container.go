package di

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strconv"
	"time"

	"prerender-shield/internal/audit"
	"prerender-shield/internal/auth"
	"prerender-shield/internal/cache"
	"prerender-shield/internal/config"
	"prerender-shield/internal/firewall"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/middleware"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/prerender"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/repository"
	"prerender-shield/internal/scheduler"
	"prerender-shield/internal/services"
	sitehandler "prerender-shield/internal/site-handler"
	siteserver "prerender-shield/internal/site-server"
	"prerender-shield/internal/ssl"
	"prerender-shield/internal/threatintel"
)

// Container 依赖注入容器
type Container struct {
	Config             *config.Config
	Redis              *redis.Client
	UserManager        *auth.UserManager
	JWTManager         *auth.JWTManager
	FirewallMgr        *firewall.EngineManager
	CacheMgr           cache.Manager
	PrerenderMgr       *prerender.EngineManager
	CrawlerLogMgr      *logging.CrawlerLogManager
	VisitLogMgr        *logging.VisitLogManager
	GeoIPService       services.GeoIPResolver
	Scheduler          *scheduler.Scheduler
	HealthChecker      monitoring.HealthChecker
	Monitor            *monitoring.Monitor
	SiteServerMgr      *siteserver.Manager
	SiteHandler        *sitehandler.Handler
	WafRepo            *repository.WafRepository
	AuditLogger        *audit.Logger
	ThreatIntelFetcher *threatintel.Fetcher
	SSLManager         ssl.Manager
	SSLAutoRenewer     *ssl.AutoRenewer // P0-14: 真正集成自动续期
}

// ContainerDeps 容器依赖项
type ContainerDeps struct {
	Config *config.Config
	Redis  *redis.Client
}

// NewContainer 创建依赖容器
func NewContainer(deps ContainerDeps) (*Container, error) {
	cfg := deps.Config
	redisClient := deps.Redis

	// 使用 wrapper 适配 redis.Client 到 WafRedisClient 接口
	wafRedisClient := &repository.RedisClientWrapper{Client: redisClient}

	// 初始化仓库
	wafRepo := repository.NewWafRepository(wafRedisClient)

	// 初始化认证模块
	userManager := auth.NewUserManager(cfg.Dirs.DataDir, redisClient)
	jwtManager := auth.NewJWTManager(&auth.JWTConfig{
		SecretKey:  getSecretKey(cfg),
		ExpireTime: 24 * time.Hour,
	}, redisClient)

	// 初始化防火墙
	firewallMgr := firewall.NewEngineManager()

	// 初始化缓存
	cacheMgr := cache.NewManager(redisClient)

	// 初始化预渲染
	prerenderMgr := prerender.NewEngineManager(redisClient, cacheMgr, getWorkerCount(cfg))

	// 初始化日志管理（复用主 Redis 连接）
	crawlerLogMgr := logging.NewCrawlerLogManagerWithClient(redisClient.GetRawClient())
	visitLogMgr := logging.NewVisitLogManagerWithClient(redisClient.GetRawClient())

	// 初始化 GeoIP
	geoIPService := services.NewGeoIPService("")

	// 初始化监控
	monitor := monitoring.NewMonitor(monitoring.Config{
		Enabled:           true,
		PrometheusAddress: cfg.Monitoring.PrometheusAddress,
		Alerting: monitoring.AlertConfig{
			Enabled: cfg.Monitoring.Alerting.Enabled,
		},
		MetricsPersistence: monitoring.MetricsPersistenceConfig{
			Enabled:           cfg.Monitoring.MetricsPersistence.Enabled,
			Interval:          time.Duration(cfg.Monitoring.MetricsPersistence.Interval) * time.Second,
			Retention:         time.Duration(cfg.Monitoring.MetricsPersistence.RetentionHours) * time.Hour,
			AggregateEnabled:  cfg.Monitoring.MetricsPersistence.AggregateEnabled,
			AggregateInterval: time.Duration(cfg.Monitoring.MetricsPersistence.AggregateInterval) * time.Second,
		},
	})

	// 加载告警规则配置文件
	alertRulesPath := cfg.Monitoring.Alerting.RulesPath
	if alertRulesPath == "" {
		alertRulesPath = "configs/alert-rules.json"
	}
	if err := monitor.LoadAlertRules(alertRulesPath); err != nil {
		// 如果配置文件不存在，使用默认规则
		if !os.IsNotExist(err) {
			// 其他错误需要记录日志但不中断启动
			logging.DefaultLogger.Info("Warning: failed to load alert rules from %s: %v", alertRulesPath, err)
		}
	}

	// 设置 Redis 客户端用于监控数据持久化
	monitor.SetRedisClient(redisClient)

	// 初始化健康检查
	healthChecker := monitoring.NewHealthChecker(redisClient)

	// 初始化 SSL 管理器
	var (
		sslMgr             ssl.Manager
		sslAutoRenewer     *ssl.AutoRenewer
		acmeClientForRenew *ssl.ACMEClient
	)
	if cfg.SSL.Enabled {
		var err error
		sslMgr, err = ssl.NewManager(redisClient, cfg.Dirs.CertsDir, cfg.SSL.Email, cfg.SSL.Production)
		if err != nil {
			logging.DefaultLogger.Warn("Failed to initialize SSL manager: %v", err)
			sslMgr = nil
		} else {
			acmeCfg := ssl.ACMEConfig{
				Email:      cfg.SSL.Email,
				CertDir:    cfg.Dirs.CertsDir,
				Production: cfg.SSL.Production,
				HTTPPort:   cfg.SSL.HTTPPort,
			}
			acmeClient, acmeErr := ssl.NewACMEClient(acmeCfg)
			if acmeErr != nil {
				logging.DefaultLogger.Warn("Failed to initialize ACME client for manager: %v", acmeErr)
			} else {
				sslMgr.SetACMEClient(acmeClient)
				acmeClientForRenew = acmeClient
				logging.DefaultLogger.Info("ACME client wired into SSL manager")
			}
		}

		// P0-14: 真正集成自动续期器 (替代之前无人调用的 checkAndRenewCertificates)
		if acmeClientForRenew != nil {
			sslAutoRenewer = ssl.NewAutoRenewer(acmeClientForRenew, redisClient, ssl.AutoRenewConfig{
				Enabled:         true,
				CheckInterval:   6 * time.Hour, // 每 6 小时检查一次
				RenewBeforeDays: 30,            // 到期前 30 天开始续签
				MaxRetries:      3,
				RetryDelay:      30 * time.Second, // 失败后 30s 重试
			})
		}
	}

	// 初始化站点管理器
	siteServerMgr := siteserver.NewManager(monitor, sslMgr)

	// 注册站点服务器状态健康检查
	healthChecker.SetSiteServerChecker(func() (int, int, string) {
		servers := siteServerMgr.ListSiteServers()
		total := len(cfg.Sites)
		running := len(servers)
		detail := ""
		if running < total {
			// 找出未运行的站点
			for _, site := range cfg.Sites {
				if _, ok := servers[site.ID]; !ok {
					if detail != "" {
						detail += ", "
					}
					detail += site.Name + " (stopped)"
				}
			}
		}
		return total, running, detail
	})

	// 初始化站点处理器（传入批量日志写入器）
	wafLogWriter := middleware.NewWafLogWriter(wafRepo, 50, 5*time.Second)
	siteHandler := sitehandler.NewHandler(prerenderMgr, wafRepo, redisClient, geoIPService, firewallMgr, wafLogWriter)

	// 初始化调度器
	schedulerInstance := scheduler.NewScheduler(prerenderMgr, redisClient, cfg)

	// 初始化审计日志
	auditLogger := audit.NewLogger(redisClient, audit.DefaultConfig())

	// 初始化威胁情报拉取器（全局单例）：
	// 聚合所有启用站点配置的并集——威胁情报本质是全局黑名单数据源，
	// 任一站点启用即应拉取；不同站点选择源的差异化由各站点检测器自行过滤
	var threatIntelFetcher *threatintel.Fetcher
	if cfg.Sites != nil && len(cfg.Sites) > 0 {
		for _, site := range cfg.Sites {
			if site.Firewall.ThreatIntel.Enabled {
				tiCfg := buildThreatIntelConfig(site.Firewall.ThreatIntel)
				if threatIntelFetcher == nil {
					threatIntelFetcher = threatintel.NewFetcher(tiCfg, redisClient)
				} else {
					threatIntelFetcher.MergeConfig(tiCfg)
				}
			}
		}
	}

	return &Container{
		Config:             cfg,
		Redis:              redisClient,
		UserManager:        userManager,
		JWTManager:         jwtManager,
		FirewallMgr:        firewallMgr,
		CacheMgr:           cacheMgr,
		PrerenderMgr:       prerenderMgr,
		CrawlerLogMgr:      crawlerLogMgr,
		VisitLogMgr:        visitLogMgr,
		GeoIPService:       geoIPService,
		Scheduler:          schedulerInstance,
		HealthChecker:      healthChecker,
		Monitor:            monitor,
		SiteServerMgr:      siteServerMgr,
		SiteHandler:        siteHandler,
		WafRepo:            wafRepo,
		AuditLogger:        auditLogger,
		ThreatIntelFetcher: threatIntelFetcher,
		SSLManager:         sslMgr,
		SSLAutoRenewer:     sslAutoRenewer,
	}, nil
}

// getSecretKey 获取 JWT 密钥
// 优先从环境变量 JWT_SECRET 读取，不存在则自动生成随机密钥
func getSecretKey(cfg *config.Config) string {
	// 1. 优先从环境变量读取
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		return secret
	}

	// 2. 自动生成随机密钥（单机部署足够安全）
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		logging.DefaultLogger.Error("Failed to generate random JWT secret: %v", err)
		// 极端情况回退：基于版本+时间生成
		h := hmac.New(sha256.New, []byte(cfg.App.Version+"-"+time.Now().String()))
		h.Write([]byte("prerender-shield-jwt-secret"))
		return hex.EncodeToString(h.Sum(nil))
	}
	return hex.EncodeToString(b)
}

// getWorkerCount 获取预渲染工作线程数
func getWorkerCount(cfg *config.Config) int {
	// 1. 优先从环境变量读取
	if workers := os.Getenv("PRERENDER_WORKER_COUNT"); workers != "" {
		if n, err := strconv.Atoi(workers); err == nil && n > 0 {
			return n
		}
	}

	// 2. 从站点配置读取：取所有站点中的最大 PoolSize，
	// 保证最大配置的站点不会因其他站点配置偏小而被限流
	maxPool := 0
	for _, site := range cfg.Sites {
		if site.Prerender.PoolSize > maxPool {
			maxPool = site.Prerender.PoolSize
		}
	}
	if maxPool > 0 {
		return maxPool
	}

	return 5 // 默认值
}

// Close 关闭所有资源
func (c *Container) Close() error {
	// 关闭 SSL 自动续期器 (P0-14)
	if c.SSLAutoRenewer != nil {
		c.SSLAutoRenewer.Stop()
	}

	// 关闭威胁情报拉取器
	if c.ThreatIntelFetcher != nil {
		c.ThreatIntelFetcher.Stop()
	}

	// 关闭调度器
	if c.Scheduler != nil {
		c.Scheduler.Stop()
	}

	// 关闭监控
	if c.Monitor != nil {
		c.Monitor.Stop()
	}

	// 关闭 Redis 连接
	if c.Redis != nil {
		c.Redis.Close()
	}

	return nil
}

// buildThreatIntelConfig converts config ThreatIntelConfig to threatintel.Config
func buildThreatIntelConfig(cfg config.ThreatIntelConfig) threatintel.Config {
	sources := make([]threatintel.Source, len(cfg.Sources))
	for i, s := range cfg.Sources {
		interval, err := time.ParseDuration(s.UpdateInterval)
		if err != nil {
			interval = 6 * time.Hour
		}
		sources[i] = threatintel.Source{
			Name:           s.Name,
			URL:            s.URL,
			Format:         s.Format,
			UpdateInterval: interval,
			Enabled:        s.Enabled,
			IPField:        s.IPField,
		}
	}

	return threatintel.Config{
		Enabled:     cfg.Enabled,
		Sources:     sources,
		GlobalKey:   cfg.GlobalKey,
		MaxIPs:      cfg.MaxIPs,
		Concurrency: cfg.Concurrency,
	}
}
