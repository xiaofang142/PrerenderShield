package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yml")

	configContent := `server:
  port: 8080
  address: "0.0.0.0"
sites:
  - id: "test-site"
    name: "Test Site"
    domains: ["localhost"]
    port: 8081
    firewall:
      enabled: true
      rules_path: "/tmp/rules"
      action:
        default_action: "block"
        block_message: "Test block message"
`

	// 写入临时配置文件
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NoError(t, err)

	// 测试加载配置
	config, err := LoadConfig(configPath)
	assert.NoError(t, err)
	assert.NotNil(t, config)

	// 验证配置值
	assert.Equal(t, 8080, config.Server.Port)
	assert.Equal(t, "0.0.0.0", config.Server.Address)
	assert.True(t, config.Sites[0].Firewall.Enabled)
	assert.Equal(t, "/tmp/rules", config.Sites[0].Firewall.RulesPath)
	assert.Equal(t, "block", config.Sites[0].Firewall.ActionConfig.DefaultAction)
	assert.Equal(t, "Test block message", config.Sites[0].Firewall.ActionConfig.BlockMessage)
}

func TestLoadConfigWithEnvVars(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yml")

	configContent := `server:
  port: 8080
  address: "0.0.0.0"
`

	// 写入临时配置文件
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NoError(t, err)

	// 设置环境变量覆盖
	err = os.Setenv("SERVER_PORT", "9090")
	assert.NoError(t, err)
	defer os.Unsetenv("SERVER_PORT")

	// 测试加载配置
	config, err := LoadConfig(configPath)
	assert.NoError(t, err)
	assert.NotNil(t, config)

	// 验证环境变量覆盖了配置文件的值
	assert.Equal(t, 9090, config.Server.Port)
	assert.Equal(t, "0.0.0.0", config.Server.Address) // 未被覆盖，使用配置文件值
}

// TestDefaultConfig 测试默认配置
func TestDefaultConfig(t *testing.T) {
	// 测试默认配置通过 LoadConfig 函数加载
	config, err := LoadConfig("")
	assert.NoError(t, err)
	assert.NotNil(t, config)

	// 验证默认值
	assert.Equal(t, 8080, config.Server.Port)
	assert.Equal(t, "0.0.0.0", config.Server.Address)
	assert.True(t, config.Sites[0].Firewall.Enabled)
	assert.Equal(t, "/etc/prerender-shield/rules", config.Sites[0].Firewall.RulesPath)
	assert.Equal(t, "block", config.Sites[0].Firewall.ActionConfig.DefaultAction)
	assert.Equal(t, "Request blocked by firewall", config.Sites[0].Firewall.ActionConfig.BlockMessage)
}
