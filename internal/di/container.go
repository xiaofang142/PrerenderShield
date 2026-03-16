package di

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strconv"
	"time"

	"prerender-shield/internal/auth"
	"prerender-shield/internal/cache"
	"prerender-shield/internal/config"
	"prerender-shield/internal/eventbus"
	"prerender-shield/internal/firewall"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/observability/metrics"
	"prerender-shield/internal/prerender"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/repository"
	"prerender-shield/internal/scheduler"
	"prerender-shield/internal/services"
	sitehandler "prerender-shield/internal/site-handler"
	siteserver "prerender-shield/internal/site-server"
)

// Container 依赖注入容器
type Container struct {
	Config          *config.Config
	Redis           *redis.Client
	UserManager     *auth.UserManager
	JWTManager      *auth.JWTManager
	FirewallMgr     *firewall.EngineManager
	CacheMgr        cache.Manager
	PrerenderMgr    *prerender.EngineManager
	CrawlerLogMgr   *logging.CrawlerLogManager
	VisitLogMgr     *logging.VisitLogManager
	GeoIPService    services.GeoIPResolver
	Scheduler       *scheduler.Scheduler
	HealthChecker   monitoring.HealthChecker
	Monitor         *monitoring.Monitor
	SiteServerMgr   *siteserver.Manager
	SiteHandler     *sitehandler.Handler
	WafRepo         *repository.WafRepository
	EventBus        *eventbus.InMemoryBus
	MetricsRecorder *metrics.InMemoryRecorder
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

	// 初始化仓库
	wafRepo := repository.NewWafRepository(redisClient)

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

	// 初始化日志管理
	crawlerLogMgr := logging.NewCrawlerLogManager(cfg.Cache.RedisURL)
	visitLogMgr := logging.NewVisitLogManager(cfg.Cache.RedisURL)

	// 初始化 GeoIP
	geoIPService := services.NewGeoIPService("")

	// 初始化监控
	monitor := monitoring.NewMonitor(monitoring.Config{
		Enabled:           true,
		PrometheusAddress: cfg.Monitoring.PrometheusAddress,
	})

	// 初始化健康检查
	healthChecker := monitoring.NewHealthChecker(redisClient)

	// 初始化站点管理器
	siteServerMgr := siteserver.NewManager(monitor)

	// 初始化站点处理器
	siteHandler := sitehandler.NewHandler(prerenderMgr, wafRepo, redisClient, geoIPService)

	// 初始化调度器
	schedulerInstance := scheduler.NewScheduler(prerenderMgr, redisClient, cfg)

	// 初始化事件总线
	eventBus := eventbus.NewInMemoryBus(nil)

	// 初始化指标记录器
	metricsRecorder := metrics.NewInMemoryRecorder()

	return &Container{
		Config:          cfg,
		Redis:           redisClient,
		UserManager:     userManager,
		JWTManager:      jwtManager,
		FirewallMgr:     firewallMgr,
		CacheMgr:        cacheMgr,
		PrerenderMgr:    prerenderMgr,
		CrawlerLogMgr:   crawlerLogMgr,
		VisitLogMgr:     visitLogMgr,
		GeoIPService:    geoIPService,
		Scheduler:       schedulerInstance,
		HealthChecker:   healthChecker,
		Monitor:         monitor,
		SiteServerMgr:   siteServerMgr,
		SiteHandler:     siteHandler,
		WafRepo:         wafRepo,
		EventBus:        eventBus,
		MetricsRecorder: metricsRecorder,
	}, nil
}

// getSecretKey 获取 JWT 密钥
// 优先从环境变量 JWT_SECRET 读取，如果不存在则使用 HMAC-SHA256 生成安全密钥
func getSecretKey(cfg *config.Config) string {
	// 1. 优先从环境变量读取
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		return secret
	}

	// 2. 从配置读取（如果配置中有）
	if cfg.App.Version != "" {
		// 使用 HMAC-SHA256 生成安全密钥
		h := hmac.New(sha256.New, []byte(cfg.App.Version))
		h.Write([]byte("prerender-shield-jwt-secret"))
		return hex.EncodeToString(h.Sum(nil))
	}

	// 3. 默认密钥（仅用于开发环境，生产环境必须设置 JWT_SECRET）
	return "prerender-shield-secret-dev-only"
}

// getWorkerCount 获取预渲染工作线程数
func getWorkerCount(cfg *config.Config) int {
	// 1. 优先从环境变量读取
	if workers := os.Getenv("PRERENDER_WORKER_COUNT"); workers != "" {
		if n, err := strconv.Atoi(workers); err == nil && n > 0 {
			return n
		}
	}

	// 2. 从站点配置读取
	if len(cfg.Sites) > 0 && cfg.Sites[0].Prerender.PoolSize > 0 {
		return cfg.Sites[0].Prerender.PoolSize
	}

	return 5 // 默认值
}

// Close 关闭所有资源
func (c *Container) Close() error {
	// 关闭调度器
	if c.Scheduler != nil {
		c.Scheduler.Stop()
	}

	// 关闭事件总线
	if c.EventBus != nil {
		c.EventBus.Close()
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
