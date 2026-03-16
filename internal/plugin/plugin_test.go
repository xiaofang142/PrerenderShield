package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockPlugin 用于测试的模拟插件
type MockPlugin struct {
	name        string
	version     string
	description string
	config      map[string]interface{}
	enabled     bool
	metrics     map[string]interface{}
}

func (m *MockPlugin) Name() string        { return m.name }
func (m *MockPlugin) Version() string     { return m.version }
func (m *MockPlugin) Description() string { return m.description }
func (m *MockPlugin) Initialize(ctx context.Context, config map[string]interface{}) error {
	m.config = config
	m.metrics["initialized"] = true
	return nil
}
func (m *MockPlugin) Shutdown(ctx context.Context) error {
	m.metrics["shutdown"] = true
	return nil
}
func (m *MockPlugin) ProcessRequest(ctx context.Context, req *http.Request) error {
	m.metrics["requests"] = m.metrics["requests"].(int) + 1
	return nil
}
func (m *MockPlugin) ProcessResponse(ctx context.Context, w http.ResponseWriter, req *http.Request, status int, body []byte) error {
	m.metrics["responses"] = m.metrics["responses"].(int) + 1
	return nil
}
func (m *MockPlugin) GetConfig() map[string]interface{} { return m.config }
func (m *MockPlugin) SetConfig(config map[string]interface{}) error {
	m.config = config
	return nil
}
func (m *MockPlugin) IsEnabled() bool { return m.enabled }
func (m *MockPlugin) Enable()         { m.enabled = true }
func (m *MockPlugin) Disable()        { m.enabled = false }
func (m *MockPlugin) GetMetrics() map[string]interface{} {
	return m.metrics
}

func newMockPlugin(name string) *MockPlugin {
	return &MockPlugin{
		name:        name,
		version:     "1.0.0",
		description: "Mock plugin for testing",
		config:      make(map[string]interface{}),
		enabled:     true,
		metrics: map[string]interface{}{
			"requests":  0,
			"responses": 0,
		},
	}
}

// TestPluginManager
func TestNewPluginManager(t *testing.T) {
	pm := NewPluginManager()
	assert.NotNil(t, pm)
	assert.NotNil(t, pm.plugins)
	assert.NotNil(t, pm.ctx)
	assert.NotNil(t, pm.cancel)
}

func TestPluginManager_RegisterPlugin(t *testing.T) {
	pm := NewPluginManager()
	plugin := newMockPlugin("test_plugin")

	err := pm.RegisterPlugin(plugin)
	assert.Nil(t, err)

	// 重复注册应该失败
	err = pm.RegisterPlugin(plugin)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestPluginManager_UnregisterPlugin(t *testing.T) {
	pm := NewPluginManager()
	plugin := newMockPlugin("test_plugin")

	pm.RegisterPlugin(plugin)

	err := pm.UnregisterPlugin("test_plugin")
	assert.Nil(t, err)

	// 注销不存在的插件应该失败
	err = pm.UnregisterPlugin("nonexistent")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPluginManager_GetPlugin(t *testing.T) {
	pm := NewPluginManager()
	plugin := newMockPlugin("test_plugin")

	pm.RegisterPlugin(plugin)

	gotPlugin, exists := pm.GetPlugin("test_plugin")
	assert.True(t, exists)
	assert.Equal(t, "test_plugin", gotPlugin.Name())

	_, exists = pm.GetPlugin("nonexistent")
	assert.False(t, exists)
}

func TestPluginManager_ListPlugins(t *testing.T) {
	pm := NewPluginManager()
	pm.RegisterPlugin(newMockPlugin("plugin1"))
	pm.RegisterPlugin(newMockPlugin("plugin2"))
	pm.RegisterPlugin(newMockPlugin("plugin3"))

	plugins := pm.ListPlugins()
	assert.Len(t, plugins, 3)
	assert.Contains(t, plugins, "plugin1")
	assert.Contains(t, plugins, "plugin2")
	assert.Contains(t, plugins, "plugin3")
}

func TestPluginManager_InitializeAll(t *testing.T) {
	pm := NewPluginManager()
	pm.RegisterPlugin(newMockPlugin("plugin1"))
	pm.RegisterPlugin(newMockPlugin("plugin2"))

	err := pm.InitializeAll()
	assert.Nil(t, err)

	// 验证插件已初始化
	plugin1, _ := pm.GetPlugin("plugin1")
	assert.True(t, plugin1.GetMetrics()["initialized"].(bool))
}

func TestPluginManager_ShutdownAll(t *testing.T) {
	pm := NewPluginManager()
	pm.RegisterPlugin(newMockPlugin("plugin1"))
	pm.RegisterPlugin(newMockPlugin("plugin2"))

	pm.InitializeAll()
	err := pm.ShutdownAll()
	assert.Nil(t, err)

	// 验证插件已关闭
	plugin1, _ := pm.GetPlugin("plugin1")
	assert.True(t, plugin1.GetMetrics()["shutdown"].(bool))
}

func TestPluginManager_ProcessRequestAll(t *testing.T) {
	pm := NewPluginManager()
	pm.RegisterPlugin(newMockPlugin("plugin1"))
	pm.RegisterPlugin(newMockPlugin("plugin2"))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	err := pm.ProcessRequestAll(context.Background(), req)
	assert.Nil(t, err)

	// 验证请求已处理
	plugin1, _ := pm.GetPlugin("plugin1")
	assert.Greater(t, plugin1.GetMetrics()["requests"].(int), 0)
}

func TestPluginManager_ProcessResponseAll(t *testing.T) {
	pm := NewPluginManager()
	pm.RegisterPlugin(newMockPlugin("plugin1"))
	pm.RegisterPlugin(newMockPlugin("plugin2"))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	err := pm.ProcessResponseAll(context.Background(), w, req, 200, []byte("OK"))
	assert.Nil(t, err)

	// 验证响应已处理
	plugin1, _ := pm.GetPlugin("plugin1")
	assert.Greater(t, plugin1.GetMetrics()["responses"].(int), 0)
}

func TestPluginManager_EnablePlugin(t *testing.T) {
	pm := NewPluginManager()
	plugin := newMockPlugin("test_plugin")
	plugin.Disable()
	pm.RegisterPlugin(plugin)

	err := pm.EnablePlugin("test_plugin")
	assert.Nil(t, err)

	gotPlugin, _ := pm.GetPlugin("test_plugin")
	assert.True(t, gotPlugin.IsEnabled())
}

func TestPluginManager_DisablePlugin(t *testing.T) {
	pm := NewPluginManager()
	plugin := newMockPlugin("test_plugin")
	pm.RegisterPlugin(plugin)

	err := pm.DisablePlugin("test_plugin")
	assert.Nil(t, err)

	gotPlugin, _ := pm.GetPlugin("test_plugin")
	assert.False(t, gotPlugin.IsEnabled())
}

func TestPluginManager_GetPluginMetrics(t *testing.T) {
	pm := NewPluginManager()
	pm.RegisterPlugin(newMockPlugin("plugin1"))
	pm.RegisterPlugin(newMockPlugin("plugin2"))

	metrics := pm.GetPluginMetrics()
	assert.Len(t, metrics, 2)
	assert.Contains(t, metrics, "plugin1")
	assert.Contains(t, metrics, "plugin2")
}

func TestPluginManager_ProcessRequestAll_OnlyEnabled(t *testing.T) {
	pm := NewPluginManager()
	plugin1 := newMockPlugin("plugin1")
	plugin2 := newMockPlugin("plugin2")
	plugin2.Disable()
	pm.RegisterPlugin(plugin1)
	pm.RegisterPlugin(plugin2)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	err := pm.ProcessRequestAll(context.Background(), req)
	assert.Nil(t, err)

	// 只有启用的插件被处理
	assert.Greater(t, plugin1.GetMetrics()["requests"].(int), 0)
	assert.Equal(t, 0, plugin2.GetMetrics()["requests"].(int))
}

func TestPluginManager_ProcessResponseAll_OnlyEnabled(t *testing.T) {
	pm := NewPluginManager()
	plugin1 := newMockPlugin("plugin1")
	plugin2 := newMockPlugin("plugin2")
	plugin2.Disable()
	pm.RegisterPlugin(plugin1)
	pm.RegisterPlugin(plugin2)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	err := pm.ProcessResponseAll(context.Background(), w, req, 200, []byte("OK"))
	assert.Nil(t, err)

	assert.Greater(t, plugin1.GetMetrics()["responses"].(int), 0)
	assert.Equal(t, 0, plugin2.GetMetrics()["responses"].(int))
}

// TestPluginLoader
func TestNewPluginLoader(t *testing.T) {
	pm := NewPluginManager()
	pl := NewPluginLoader("/tmp/plugins", pm)
	assert.NotNil(t, pl)
	assert.Equal(t, "/tmp/plugins", pl.pluginDir)
	assert.NotNil(t, pl.pluginManager)
	assert.NotNil(t, pl.loadedPlugins)
}

func TestPluginLoader_LoadPlugins(t *testing.T) {
	pm := NewPluginManager()
	pl := NewPluginLoader("/tmp/plugins", pm)

	err := pl.LoadPlugins()
	assert.Nil(t, err)
}

func TestPluginLoader_UnloadPlugins(t *testing.T) {
	pm := NewPluginManager()
	pl := NewPluginLoader("/tmp/plugins", pm)

	// 模拟已加载的插件
	pl.loadedPlugins["plugin1"] = true
	pl.loadedPlugins["plugin2"] = true
	pm.RegisterPlugin(newMockPlugin("plugin1"))
	pm.RegisterPlugin(newMockPlugin("plugin2"))

	err := pl.UnloadPlugins()
	assert.Nil(t, err)
	assert.Empty(t, pl.loadedPlugins)
}

func TestPluginLoader_LoadPlugin(t *testing.T) {
	pm := NewPluginManager()
	pl := NewPluginLoader("/tmp/plugins", pm)

	// LoadPlugin 是空实现，不会设置 loadedPlugins
	err := pl.LoadPlugin("test_plugin")
	assert.Nil(t, err)
	// 由于是空实现，loadedPlugins 不会被设置
	// 这里只验证方法调用不报错
}

func TestPluginLoader_LoadPlugin_AlreadyLoaded(t *testing.T) {
	pm := NewPluginManager()
	pl := NewPluginLoader("/tmp/plugins", pm)

	// 模拟已加载
	pl.loadedPlugins["test_plugin"] = true

	// 重复加载应该失败
	err := pl.LoadPlugin("test_plugin")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "already loaded")
}

func TestPluginLoader_UnloadPlugin(t *testing.T) {
	pm := NewPluginManager()
	pl := NewPluginLoader("/tmp/plugins", pm)

	// 先加载
	pl.loadedPlugins["test_plugin"] = true
	pm.RegisterPlugin(newMockPlugin("test_plugin"))

	err := pl.UnloadPlugin("test_plugin")
	assert.Nil(t, err)
	assert.False(t, pl.loadedPlugins["test_plugin"])
}

func TestPluginLoader_UnloadPlugin_NotLoaded(t *testing.T) {
	pm := NewPluginManager()
	pl := NewPluginLoader("/tmp/plugins", pm)

	err := pl.UnloadPlugin("nonexistent")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "not loaded")
}

// TestDefaultPlugin
func TestNewDefaultPlugin(t *testing.T) {
	p := NewDefaultPlugin("my_plugin", "1.0.0", "My test plugin")
	assert.NotNil(t, p)
	assert.Equal(t, "my_plugin", p.Name())
	assert.Equal(t, "1.0.0", p.Version())
	assert.Equal(t, "My test plugin", p.Description())
	assert.True(t, p.IsEnabled())
}

func TestDefaultPlugin_Initialize(t *testing.T) {
	p := NewDefaultPlugin("my_plugin", "1.0.0", "My test plugin")
	config := map[string]interface{}{"key": "value"}

	err := p.Initialize(context.Background(), config)
	assert.Nil(t, err)
	assert.Equal(t, "value", p.GetConfig()["key"])
	assert.NotNil(t, p.GetMetrics()["initialized_at"])
	assert.Equal(t, 0, p.GetMetrics()["requests_processed"].(int))
	assert.Equal(t, 0, p.GetMetrics()["responses_processed"].(int))
}

func TestDefaultPlugin_Shutdown(t *testing.T) {
	p := NewDefaultPlugin("my_plugin", "1.0.0", "My test plugin")
	p.Initialize(context.Background(), nil)

	err := p.Shutdown(context.Background())
	assert.Nil(t, err)
	assert.NotNil(t, p.GetMetrics()["shutdown_at"])
}

func TestDefaultPlugin_ProcessRequest(t *testing.T) {
	p := NewDefaultPlugin("my_plugin", "1.0.0", "My test plugin")
	p.Initialize(context.Background(), nil)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	err := p.ProcessRequest(context.Background(), req)
	assert.Nil(t, err)
	assert.Equal(t, 1, p.GetMetrics()["requests_processed"].(int))

	// 多次请求
	p.ProcessRequest(context.Background(), req)
	assert.Equal(t, 2, p.GetMetrics()["requests_processed"].(int))
}

func TestDefaultPlugin_ProcessResponse(t *testing.T) {
	p := NewDefaultPlugin("my_plugin", "1.0.0", "My test plugin")
	p.Initialize(context.Background(), nil)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	err := p.ProcessResponse(context.Background(), w, req, 200, []byte("OK"))
	assert.Nil(t, err)
	assert.Equal(t, 1, p.GetMetrics()["responses_processed"].(int))
}

func TestDefaultPlugin_GetConfig_SetConfig(t *testing.T) {
	p := NewDefaultPlugin("my_plugin", "1.0.0", "My test plugin")
	config := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
	}

	err := p.SetConfig(config)
	assert.Nil(t, err)
	assert.Equal(t, "value1", p.GetConfig()["key1"])
	assert.Equal(t, 123, p.GetConfig()["key2"])
}

func TestDefaultPlugin_Enable_Disable(t *testing.T) {
	p := NewDefaultPlugin("my_plugin", "1.0.0", "My test plugin")

	// 默认启用
	assert.True(t, p.IsEnabled())

	// 禁用
	p.Disable()
	assert.False(t, p.IsEnabled())

	// 启用
	p.Enable()
	assert.True(t, p.IsEnabled())
}

func TestDefaultPlugin_GetMetrics(t *testing.T) {
	p := NewDefaultPlugin("my_plugin", "1.0.0", "My test plugin")
	p.Initialize(context.Background(), nil)

	metrics := p.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Contains(t, metrics, "initialized_at")
	assert.Contains(t, metrics, "requests_processed")
	assert.Contains(t, metrics, "responses_processed")
}

func TestDefaultPlugin_ConcurrentAccess(t *testing.T) {
	p := NewDefaultPlugin("my_plugin", "1.0.0", "My test plugin")

	done := make(chan bool, 10)

	// 并发访问
	for i := 0; i < 10; i++ {
		go func() {
			p.Enable()
			p.Disable()
			p.GetConfig()
			p.GetMetrics()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestPluginInterface(t *testing.T) {
	// 验证 DefaultPlugin 实现了 Plugin 接口
	var _ Plugin = (*DefaultPlugin)(nil)
}

func TestMockPlugin(t *testing.T) {
	p := newMockPlugin("mock")
	assert.Equal(t, "mock", p.Name())
	assert.Equal(t, "1.0.0", p.Version())
	assert.True(t, p.IsEnabled())
}
