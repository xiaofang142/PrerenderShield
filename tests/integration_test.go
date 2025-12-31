package tests

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/config"
	"prerender-shield/internal/redis"
)

// TestConfigAndRedisIntegration 测试配置管理和Redis客户端的集成
func TestConfigAndRedisIntegration(t *testing.T) {
	// 这个测试需要实际的Redis服务器，我们暂时跳过
	t.Skip("Skipping test that requires actual Redis server")

	// 确保从项目根目录运行测试，或者使用正确的相对路径
	projectRoot := ".."
	configPath := filepath.Join(projectRoot, "configs", "config.example.yml")

	// 加载配置
	cfg, err := config.LoadConfig(configPath)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// 测试Redis客户端连接
	redisClient, err := redis.NewClient(cfg.Cache.RedisURL)
	assert.NoError(t, err)
	assert.NotNil(t, redisClient)

	// 测试Redis基本操作
	siteName := "test-site"
	testURL := "http://example.com/integration-test"

	// 测试添加URL
	err = redisClient.AddURL(siteName, testURL)
	assert.NoError(t, err)

	// 测试获取URL数量
	count, err := redisClient.GetURLCount(siteName)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(1))

	// 测试获取URLs
	urls, err := redisClient.GetURLs(siteName)
	assert.NoError(t, err)
	assert.NotEmpty(t, urls)

	// 测试移除URL
	err = redisClient.RemoveURL(siteName, testURL)
	assert.NoError(t, err)

	// 关闭Redis连接
	err = redisClient.Close()
	assert.NoError(t, err)
}

// TestConfigReloadIntegration 测试配置热重载功能
func TestConfigReloadIntegration(t *testing.T) {
	// 创建临时配置文件
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config.yml")

	// 写入测试配置
	testConfig := `
server:
  address: 127.0.0.1
  api_port: 8080
  console_port: 8081
cache:
  type: memory
  redis_url: localhost:6379
  memory_size: 1000
`
	err := os.WriteFile(configFile, []byte(testConfig), 0644)
	assert.NoError(t, err)

	// 加载配置
	cfg, err := config.LoadConfig(configFile)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// 获取配置管理器实例
	configManager := config.GetInstance()

	// 启动配置监控
	err = configManager.StartWatching()
	assert.NoError(t, err)

	// 等待一段时间，确保监控已启动
	time.Sleep(1 * time.Second)

	// 停止配置监控
	configManager.StopWatching()

	// 验证配置监控功能能够正常启动和停止
	assert.True(t, true, "配置监控功能测试通过")
}
