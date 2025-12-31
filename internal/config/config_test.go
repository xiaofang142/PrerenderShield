package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	// 创建临时配置文件
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config.yaml")

	// 写入测试配置
	testConfig := `
server:
  address: 127.0.0.1
  api_port: 8080
  console_port: 8081
`
	err := os.WriteFile(configFile, []byte(testConfig), 0644)
	assert.NoError(t, err)

	// 加载配置
	cfg, err := LoadConfig(configFile)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "127.0.0.1", cfg.Server.Address)
	assert.Equal(t, 8080, cfg.Server.APIPort)
	assert.Equal(t, 8081, cfg.Server.ConsolePort)
}

func TestGetInstance(t *testing.T) {
	// 测试单例模式
	instance1 := GetInstance()
	instance2 := GetInstance()
	assert.Equal(t, instance1, instance2)
}

func TestValidateConfig(t *testing.T) {
	manager := GetInstance()

	// 测试无效配置（缺少站点ID）
	invalidConfig := &Config{
		Sites: []SiteConfig{
			{
				Name:    "Test Site",
				Domains: []string{"example.com"},
				Mode:    "static",
			},
		},
	}
	err := manager.ValidateConfig(invalidConfig)
	assert.Error(t, err)

	// 测试有效配置
	validConfig := &Config{
		Sites: []SiteConfig{
			{
				ID:      "test-site",
				Name:    "Test Site",
				Domains: []string{"example.com"},
				Mode:    "static",
			},
		},
	}
	err = manager.ValidateConfig(validConfig)
	assert.NoError(t, err)
}

func TestUpdateConfig(t *testing.T) {
	manager := GetInstance()

	// 添加配置变化处理函数
	configUpdated := false
	manager.AddConfigChangeHandler(func(cfg *Config) {
		configUpdated = true
	})

	// 更新配置
	newConfig := defaultConfig()
	newConfig.Server.APIPort = 9090
	manager.UpdateConfig(newConfig)

	// 检查配置是否更新
	cfg := manager.GetConfig()
	assert.Equal(t, 9090, cfg.Server.APIPort)

	// 等待配置变化处理函数执行
	time.Sleep(100 * time.Millisecond)
	assert.True(t, configUpdated)
}

func TestStartAndStopWatching(t *testing.T) {
	// 创建临时配置文件
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config.yaml")

	// 写入初始配置
	testConfig := `
server:
  address: 127.0.0.1
  api_port: 8080
`
	err := os.WriteFile(configFile, []byte(testConfig), 0644)
	assert.NoError(t, err)

	// 加载配置
	_, err = LoadConfig(configFile)
	assert.NoError(t, err)

	manager := GetInstance()

	// 启动配置监控
	err = manager.StartWatching()
	assert.NoError(t, err)

	// 停止配置监控
	manager.StopWatching()
}

func TestGetEnv(t *testing.T) {
	// 测试获取不存在的环境变量
	result := getEnv("NON_EXISTENT_ENV", "default")
	assert.Equal(t, "default", result)

	// 测试获取存在的环境变量
	os.Setenv("TEST_ENV", "test_value")
	result = getEnv("TEST_ENV", "default")
	assert.Equal(t, "test_value", result)
	os.Unsetenv("TEST_ENV")
}

func TestGetEnvAsInt(t *testing.T) {
	// 测试获取不存在的环境变量
	result := getEnvAsInt("NON_EXISTENT_ENV", 123)
	assert.Equal(t, 123, result)

	// 测试获取存在的环境变量
	os.Setenv("TEST_INT_ENV", "456")
	result = getEnvAsInt("TEST_INT_ENV", 123)
	assert.Equal(t, 456, result)
	os.Unsetenv("TEST_INT_ENV")

	// 测试获取无效的整数环境变量
	os.Setenv("TEST_INT_ENV", "invalid")
	result = getEnvAsInt("TEST_INT_ENV", 123)
	assert.Equal(t, 123, result)
	os.Unsetenv("TEST_INT_ENV")
}

func TestGetEnvAsBool(t *testing.T) {
	// 测试获取不存在的环境变量
	result := getEnvAsBool("NON_EXISTENT_ENV", true)
	assert.True(t, result)

	// 测试获取存在的环境变量（true）
	os.Setenv("TEST_BOOL_ENV", "true")
	result = getEnvAsBool("TEST_BOOL_ENV", false)
	assert.True(t, result)
	os.Unsetenv("TEST_BOOL_ENV")

	// 测试获取存在的环境变量（false）
	os.Setenv("TEST_BOOL_ENV", "false")
	result = getEnvAsBool("TEST_BOOL_ENV", true)
	assert.False(t, result)
	os.Unsetenv("TEST_BOOL_ENV")

	// 测试获取无效的布尔环境变量
	os.Setenv("TEST_BOOL_ENV", "invalid")
	result = getEnvAsBool("TEST_BOOL_ENV", true)
	assert.True(t, result)
	os.Unsetenv("TEST_BOOL_ENV")
}

func TestGetEnvAsFloat(t *testing.T) {
	// 测试获取不存在的环境变量
	result := getEnvAsFloat("NON_EXISTENT_ENV", 123.45)
	assert.Equal(t, 123.45, result)

	// 测试获取存在的环境变量
	os.Setenv("TEST_FLOAT_ENV", "678.90")
	result = getEnvAsFloat("TEST_FLOAT_ENV", 123.45)
	assert.Equal(t, 678.90, result)
	os.Unsetenv("TEST_FLOAT_ENV")

	// 测试获取无效的浮点数环境变量
	os.Setenv("TEST_FLOAT_ENV", "invalid")
	result = getEnvAsFloat("TEST_FLOAT_ENV", 123.45)
	assert.Equal(t, 123.45, result)
	os.Unsetenv("TEST_FLOAT_ENV")
}