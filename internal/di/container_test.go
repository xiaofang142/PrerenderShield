package di

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"prerender-shield/internal/config"
	"prerender-shield/internal/redis"
)

func TestNewContainer(t *testing.T) {
	// Create temporary directories
	tmpDir := t.TempDir()
	dataDir := tmpDir + "/data"
	staticDir := tmpDir + "/static"
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(staticDir, 0755)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Address: "127.0.0.1",
			APIPort: 8080,
		},
		Dirs: config.DirsConfig{
			DataDir:   dataDir,
			StaticDir: staticDir,
		},
		Monitoring: config.MonitoringConfig{
			PrometheusAddress: "",
		},
	}

	// Create a mock Redis client (will fail without real Redis, so we skip full integration)
	// For unit testing, we test the container creation logic without actual Redis connection
	t.Skip("Skipping full integration test - requires Redis connection")

	// This would be the full test if Redis was available:
	redisClient := &redis.Client{}
	deps := ContainerDeps{
		Config: cfg,
		Redis:  redisClient,
	}

	container, err := NewContainer(deps)
	assert.NoError(t, err)
	assert.NotNil(t, container)
	assert.NotNil(t, container.Config)
	assert.NotNil(t, container.UserManager)
	assert.NotNil(t, container.JWTManager)
	assert.NotNil(t, container.FirewallMgr)
	assert.NotNil(t, container.CacheMgr)
	assert.NotNil(t, container.PrerenderMgr)
	assert.NotNil(t, container.CrawlerLogMgr)
	assert.NotNil(t, container.VisitLogMgr)
	assert.NotNil(t, container.Scheduler)
	assert.NotNil(t, container.HealthChecker)
	assert.NotNil(t, container.Monitor)
	assert.NotNil(t, container.SiteServerMgr)
	assert.NotNil(t, container.SiteHandler)
	assert.NotNil(t, container.WafRepo)
}

func TestGetSecretKey(t *testing.T) {
	// Test with environment variable
	os.Setenv("JWT_SECRET", "test-secret-from-env")
	defer os.Unsetenv("JWT_SECRET")

	cfg := &config.Config{}
	key := getSecretKey(cfg)
	assert.Equal(t, "test-secret-from-env", key)
}

func TestGetSecretKeyFromConfig(t *testing.T) {
	// Test with config version (no env var)
	os.Unsetenv("JWT_SECRET")

	cfg := &config.Config{
		App: config.AppConfig{
			Version: "1.0.0",
		},
	}

	key := getSecretKey(cfg)
	assert.NotEmpty(t, key)
	// Key should be a hex-encoded HMAC-SHA256 hash (64 characters)
	assert.Len(t, key, 64)
}

func TestGetSecretKeyDefault(t *testing.T) {
	// Test with no env var and no config version - should generate random key
	os.Unsetenv("JWT_SECRET")

	cfg := &config.Config{}
	key := getSecretKey(cfg)
	// 应生成 64 字符的 hex 编码密钥（32 字节）
	assert.Len(t, key, 64)
	assert.NotEmpty(t, key)
}

func TestGetWorkerCount(t *testing.T) {
	// Test with environment variable
	os.Setenv("PRERENDER_WORKER_COUNT", "10")
	defer os.Unsetenv("PRERENDER_WORKER_COUNT")

	cfg := &config.Config{}
	count := getWorkerCount(cfg)
	assert.Equal(t, 10, count)
}

func TestGetWorkerCountFromConfig(t *testing.T) {
	// Test with config (no env var)
	os.Unsetenv("PRERENDER_WORKER_COUNT")

	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				Prerender: config.PrerenderConfig{
					PoolSize: 8,
				},
			},
		},
	}

	count := getWorkerCount(cfg)
	assert.Equal(t, 8, count)
}

func TestGetWorkerCountDefault(t *testing.T) {
	// Test with no env var and no config
	os.Unsetenv("PRERENDER_WORKER_COUNT")

	cfg := &config.Config{
		Sites: []config.SiteConfig{},
	}

	count := getWorkerCount(cfg)
	assert.Equal(t, 5, count)
}

func TestGetWorkerCountInvalidEnv(t *testing.T) {
	// Test with invalid env var
	os.Setenv("PRERENDER_WORKER_COUNT", "invalid")
	defer os.Unsetenv("PRERENDER_WORKER_COUNT")

	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				Prerender: config.PrerenderConfig{
					PoolSize: 8,
				},
			},
		},
	}

	count := getWorkerCount(cfg)
	// Should fall back to config value
	assert.Equal(t, 8, count)
}

func TestContainer_Close(t *testing.T) {
	// Create a minimal container for testing close
	container := &Container{
		Config: &config.Config{},
	}

	// Close should not panic even with nil fields
	err := container.Close()
	assert.NoError(t, err)
}
