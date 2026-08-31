package di

import (
	"testing"

	"prerender-shield/internal/config"
	"prerender-shield/internal/redis"
)

// TestNewContainer_ConfigManagerRedisInjected 回归测试（R11-BUG-3）：
// 生产装配必须把 Redis 客户端注入 ConfigManager，否则站点配置的 Redis 副本
// 同步在 Mutate 中静默跳过，SaveSitesToRedis/LoadSitesFromRedis 恒报
// "redis client is not set"，"站点配置存 Redis" 链路整体休眠。
func TestNewContainer_ConfigManagerRedisInjected(t *testing.T) {
	cfg := testDIConfig()
	client, err := redis.NewClient("localhost:6379", "", 15)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	defer client.Close()

	container, err := NewContainer(ContainerDeps{Config: cfg, Redis: client})
	if err != nil {
		t.Fatalf("NewContainer: %v", err)
	}
	t.Cleanup(func() { _ = container.Close() })

	cm := config.GetInstance()
	if cm == nil {
		t.Fatal("config manager instance missing")
	}
	if err := cm.SaveSitesToRedis(); err != nil {
		t.Fatalf("SaveSitesToRedis after DI assembly: %v (redis client must be injected)", err)
	}
}

// TestConfigManager_MutateWritesRedisCopy 端到端验证 Mutate 的 Redis 副本同步
func TestConfigManager_MutateWritesRedisCopy(t *testing.T) {
	cfg := testDIConfig()

	client, err := redis.NewClient("localhost:6379", "", 15)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	defer client.Close()

	container, err := NewContainer(ContainerDeps{Config: cfg, Redis: client})
	if err != nil {
		t.Fatalf("NewContainer: %v", err)
	}
	t.Cleanup(func() { _ = container.Close() })

	cm := config.GetInstance()
	if err := cm.Mutate(func(c *config.Config) (*config.Config, error) {
		c.Sites = append(c.Sites, config.SiteConfig{ID: "redis-copy-check", Name: "redis-copy-check", Mode: "static"})
		return c, nil
	}); err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	// 重启语义验证：Mutate 已同步副本 → LoadSitesFromRedis 应能取回
	if err := cm.LoadSitesFromRedis(); err != nil {
		t.Fatalf("LoadSitesFromRedis (restart simulation): %v", err)
	}
}
