//go:build wireinject
// +build wireinject

package di

import (
	"context"

	"github.com/google/wire"
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
	"prerender-shield/internal/utils/redisutil"
)

// ProviderSet 提供者集合
var ProviderSet = wire.NewSet(
	// Redis
	NewRedis,

	// Config
	NewConfig,

	// Repositories
	repository.NewWafRepository,

	// Auth
	auth.NewUserManager,
	auth.NewJWTManager,

	// Firewall
	firewall.NewEngineManager,

	// Cache
	cache.NewManager,

	// Prerender
	prerender.NewEngineManager,

	// Logging
	logging.NewCrawlerLogManager,
	logging.NewVisitLogManager,

	// Services
	services.NewGeoIPService,

	// Monitoring
	monitoring.NewMonitor,
	monitoring.NewHealthChecker,

	// Site
	siteserver.NewManager,
	sitehandler.NewHandler,

	// Scheduler
	scheduler.NewScheduler,

	// Event Bus
	event.NewInMemoryBus,

	// Metrics
	metrics.NewInMemoryRecorder,

	// Container
	wire.Struct(new(Container), "*"),
)

// NewRedis 创建 Redis 客户端（Wire 提供者）
func NewRedis(cfg *config.Config) (*redis.Client, error) {
	return redisutil.ParseRedisURL(cfg.Cache.RedisURL)
}

// NewConfig 创建配置（Wire 提供者）
func NewConfig() *config.Config {
	return config.GetInstance().GetConfig()
}

// InitializeContainer 初始化容器（由 Wire 生成）
func InitializeContainer(ctx context.Context, cfg *config.Config) (*Container, error) {
	wire.Build(ProviderSet)
	return &Container{}, nil
}
