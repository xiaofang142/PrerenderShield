package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/utils/redisutil"
)

// Application 应用容器
type Application struct {
	config    *config.Config
	redis     *redis.Client
	servers   []*http.Server
	cleanupFn []func()
	mu        sync.Mutex
	startTime time.Time
}

// New 创建新应用
func New(ctx context.Context, configPath string) (*Application, error) {
	app := &Application{
		startTime: time.Now(),
		servers:   make([]*http.Server, 0),
		cleanupFn: make([]func(), 0),
	}

	// 加载配置
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	app.config = cfg

	// 初始化 Redis (从配置中解析地址、密码、DB)
	redisClient, err := redisutil.ParseRedisURL(cfg.Cache.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("init redis: %w", err)
	}
	app.redis = redisClient
	app.AddCleanup(func() {
		redisClient.Close()
	})

	return app, nil
}

// GetConfig 获取配置
func (a *Application) GetConfig() *config.Config {
	return a.config
}

// GetRedis 获取 Redis 客户端
func (a *Application) GetRedis() *redis.Client {
	return a.redis
}

// AddServer 添加 HTTP 服务器
func (a *Application) AddServer(server *http.Server) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.servers = append(a.servers, server)
}

// AddCleanup 添加清理函数
func (a *Application) AddCleanup(fn func()) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cleanupFn = append(a.cleanupFn, fn)
}

// Run 运行应用
func (a *Application) Run(ctx context.Context) error {
	// 等待关闭信号
	<-ctx.Done()

	// 关闭应用
	return a.Shutdown(ctx)
}

// Shutdown 关闭应用。
// 顺序至关重要：先对 HTTP 服务器排水（完成进行中的请求），
// 再执行清理函数（关停调度器/引擎/监控/Redis 等依赖），
// 否则渲染请求可能打到已关闭的浏览器池、或依赖被提前抽走
func (a *Application) Shutdown(ctx context.Context) error {
	logging.DefaultLogger.Info("Shutting down application...")

	// 快照服务器与清理函数
	a.mu.Lock()
	servers := make([]*http.Server, len(a.servers))
	copy(servers, a.servers)
	cleanupFns := make([]func(), len(a.cleanupFn))
	copy(cleanupFns, a.cleanupFn)
	a.mu.Unlock()

	// 1) 先排水：等待进行中的请求完成
	for _, server := range servers {
		if err := server.Shutdown(ctx); err != nil {
			logging.DefaultLogger.Info("Server shutdown error: %v", err)
		}
	}

	// 2) 再执行模块级清理
	for _, fn := range cleanupFns {
		fn()
	}

	logging.DefaultLogger.Info("Application shutdown complete")
	return nil
}

// GetStartTime 获取启动时间
func (a *Application) GetStartTime() time.Time {
	return a.startTime
}

// GetUptime 获取运行时长
func (a *Application) GetUptime() time.Duration {
	return time.Since(a.startTime)
}
