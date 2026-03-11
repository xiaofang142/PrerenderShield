package plugin

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Plugin 插件接口
// 所有插件都需要实现此接口
//
// 方法:
//   Name(): 返回插件名称
//   Version(): 返回插件版本
//   Description(): 返回插件描述
//   Initialize(): 初始化插件
//   Shutdown(): 关闭插件
//   ProcessRequest(): 处理HTTP请求
//   ProcessResponse(): 处理HTTP响应
//   GetConfig(): 获取插件配置
//   SetConfig(): 设置插件配置
//   IsEnabled(): 检查插件是否启用
//   Enable(): 启用插件
//   Disable(): 禁用插件
//   GetMetrics(): 获取插件指标

type Plugin interface {
	// 插件基本信息
	Name() string
	Version() string
	Description() string

	// 插件生命周期管理
	Initialize(ctx context.Context, config map[string]interface{}) error
	Shutdown(ctx context.Context) error

	// 请求和响应处理
	ProcessRequest(ctx context.Context, req *http.Request) error
	ProcessResponse(ctx context.Context, w http.ResponseWriter, req *http.Request, status int, body []byte) error

	// 配置管理
	GetConfig() map[string]interface{}
	SetConfig(config map[string]interface{}) error

	// 插件状态管理
	IsEnabled() bool
	Enable()
	Disable()

	// 插件指标
	GetMetrics() map[string]interface{}
}

// PluginManager 插件管理器
// 用于管理所有插件的生命周期和操作
//
// 字段:
//   plugins: 插件映射，键为插件名称
//   mutex: 互斥锁，用于并发安全
//   ctx: 上下文，用于插件初始化和关闭
//   cancel: 上下文取消函数
//
// 方法:
//   RegisterPlugin(): 注册插件
//   UnregisterPlugin(): 注销插件
//   GetPlugin(): 获取插件
//   ListPlugins(): 列出所有插件
//   InitializeAll(): 初始化所有插件
//   ShutdownAll(): 关闭所有插件
//   ProcessRequestAll(): 处理所有插件的请求
//   ProcessResponseAll(): 处理所有插件的响应
//   EnablePlugin(): 启用插件
//   DisablePlugin(): 禁用插件
//   GetPluginMetrics(): 获取插件指标

type PluginManager struct {
	plugins map[string]Plugin
	mutex   sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewPluginManager 创建新的插件管理器
func NewPluginManager() *PluginManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &PluginManager{
		plugins: make(map[string]Plugin),
		mutex:   sync.RWMutex{},
		ctx:     ctx,
		cancel:  cancel,
	}
}

// RegisterPlugin 注册插件
func (pm *PluginManager) RegisterPlugin(plugin Plugin) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	name := plugin.Name()
	if _, exists := pm.plugins[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}

	pm.plugins[name] = plugin
	return nil
}

// UnregisterPlugin 注销插件
func (pm *PluginManager) UnregisterPlugin(name string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	plugin, exists := pm.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	// 关闭插件
	err := plugin.Shutdown(pm.ctx)
	if err != nil {
		return fmt.Errorf("failed to shutdown plugin %s: %v", name, err)
	}

	// 从映射中删除
	delete(pm.plugins, name)
	return nil
}

// GetPlugin 获取插件
func (pm *PluginManager) GetPlugin(name string) (Plugin, bool) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	plugin, exists := pm.plugins[name]
	return plugin, exists
}

// ListPlugins 列出所有插件
func (pm *PluginManager) ListPlugins() []string {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	plugins := make([]string, 0, len(pm.plugins))
	for name := range pm.plugins {
		plugins = append(plugins, name)
	}

	return plugins
}

// InitializeAll 初始化所有插件
func (pm *PluginManager) InitializeAll() error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	for name, plugin := range pm.plugins {
		err := plugin.Initialize(pm.ctx, plugin.GetConfig())
		if err != nil {
			return fmt.Errorf("failed to initialize plugin %s: %v", name, err)
		}
	}

	return nil
}

// ShutdownAll 关闭所有插件
func (pm *PluginManager) ShutdownAll() error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	for name, plugin := range pm.plugins {
		err := plugin.Shutdown(pm.ctx)
		if err != nil {
			return fmt.Errorf("failed to shutdown plugin %s: %v", name, err)
		}
	}

	// 取消上下文
	pm.cancel()
	return nil
}

// ProcessRequestAll 处理所有插件的请求
func (pm *PluginManager) ProcessRequestAll(ctx context.Context, req *http.Request) error {
	pm.mutex.RLock()
	plugins := make(map[string]Plugin)
	for name, plugin := range pm.plugins {
		if plugin.IsEnabled() {
			plugins[name] = plugin
		}
	}
	pm.mutex.RUnlock()

	for name, plugin := range plugins {
		err := plugin.ProcessRequest(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to process request with plugin %s: %v", name, err)
		}
	}

	return nil
}

// ProcessResponseAll 处理所有插件的响应
func (pm *PluginManager) ProcessResponseAll(ctx context.Context, w http.ResponseWriter, req *http.Request, status int, body []byte) error {
	pm.mutex.RLock()
	plugins := make(map[string]Plugin)
	for name, plugin := range pm.plugins {
		if plugin.IsEnabled() {
			plugins[name] = plugin
		}
	}
	pm.mutex.RUnlock()

	for name, plugin := range plugins {
		err := plugin.ProcessResponse(ctx, w, req, status, body)
		if err != nil {
			return fmt.Errorf("failed to process response with plugin %s: %v", name, err)
		}
	}

	return nil
}

// EnablePlugin 启用插件
func (pm *PluginManager) EnablePlugin(name string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	plugin, exists := pm.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	plugin.Enable()
	return nil
}

// DisablePlugin 禁用插件
func (pm *PluginManager) DisablePlugin(name string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	plugin, exists := pm.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	plugin.Disable()
	return nil
}

// GetPluginMetrics 获取插件指标
func (pm *PluginManager) GetPluginMetrics() map[string]map[string]interface{} {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	metrics := make(map[string]map[string]interface{})
	for name, plugin := range pm.plugins {
		metrics[name] = plugin.GetMetrics()
	}

	return metrics
}

// PluginLoader 插件加载器
// 用于加载和管理插件
//
// 字段:
//   pluginDir: 插件目录
//   pluginManager: 插件管理器
//
// 方法:
//   LoadPlugins(): 加载所有插件
//   UnloadPlugins(): 卸载所有插件
//   LoadPlugin(): 加载单个插件
//   UnloadPlugin(): 卸载单个插件

type PluginLoader struct {
	pluginDir     string
	pluginManager *PluginManager
	loadedPlugins map[string]bool
	loadMutex     sync.Mutex
}

// NewPluginLoader 创建新的插件加载器
func NewPluginLoader(pluginDir string, pluginManager *PluginManager) *PluginLoader {
	return &PluginLoader{
		pluginDir:     pluginDir,
		pluginManager: pluginManager,
		loadedPlugins: make(map[string]bool),
		loadMutex:     sync.Mutex{},
	}
}

// LoadPlugins 加载所有插件
func (pl *PluginLoader) LoadPlugins() error {
	pl.loadMutex.Lock()
	defer pl.loadMutex.Unlock()

	// 这里实现插件加载逻辑
	// 例如从目录加载插件
	// 暂时返回空，实际实现需要根据具体的插件加载方式进行调整
	return nil
}

// UnloadPlugins 卸载所有插件
func (pl *PluginLoader) UnloadPlugins() error {
	pl.loadMutex.Lock()
	defer pl.loadMutex.Unlock()

	// 卸载所有加载的插件
	for name := range pl.loadedPlugins {
		err := pl.pluginManager.UnregisterPlugin(name)
		if err != nil {
			return err
		}
		delete(pl.loadedPlugins, name)
	}

	return nil
}

// LoadPlugin 加载单个插件
func (pl *PluginLoader) LoadPlugin(name string) error {
	pl.loadMutex.Lock()
	defer pl.loadMutex.Unlock()

	// 检查插件是否已加载
	if pl.loadedPlugins[name] {
		return fmt.Errorf("plugin %s already loaded", name)
	}

	// 这里实现加载单个插件的逻辑
	// 暂时返回空，实际实现需要根据具体的插件加载方式进行调整
	return nil
}

// UnloadPlugin 卸载单个插件
func (pl *PluginLoader) UnloadPlugin(name string) error {
	pl.loadMutex.Lock()
	defer pl.loadMutex.Unlock()

	// 检查插件是否已加载
	if !pl.loadedPlugins[name] {
		return fmt.Errorf("plugin %s not loaded", name)
	}

	// 卸载插件
	err := pl.pluginManager.UnregisterPlugin(name)
	if err != nil {
		return err
	}

	delete(pl.loadedPlugins, name)
	return nil
}

// DefaultPlugin 基础插件实现
// 提供插件接口的默认实现，方便插件开发者继承
//
// 字段:
//   name: 插件名称
//   version: 插件版本
//   description: 插件描述
//   config: 插件配置
//   enabled: 插件是否启用
//   metrics: 插件指标
//   mutex: 互斥锁，用于并发安全
//
// 方法:
//   实现Plugin接口的所有方法

type DefaultPlugin struct {
	name        string
	version     string
	description string
	config      map[string]interface{}
	enabled     bool
	metrics     map[string]interface{}
	mutex       sync.RWMutex
}

// NewDefaultPlugin 创建新的默认插件
func NewDefaultPlugin(name, version, description string) *DefaultPlugin {
	return &DefaultPlugin{
		name:        name,
		version:     version,
		description: description,
		config:      make(map[string]interface{}),
		enabled:     true,
		metrics:     make(map[string]interface{}),
		mutex:       sync.RWMutex{},
	}
}

// Name 返回插件名称
func (p *DefaultPlugin) Name() string {
	return p.name
}

// Version 返回插件版本
func (p *DefaultPlugin) Version() string {
	return p.version
}

// Description 返回插件描述
func (p *DefaultPlugin) Description() string {
	return p.description
}

// Initialize 初始化插件
func (p *DefaultPlugin) Initialize(ctx context.Context, config map[string]interface{}) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if config != nil {
		p.config = config
	}

	// 初始化插件指标
	p.metrics["initialized_at"] = time.Now().Unix()
	p.metrics["requests_processed"] = 0
	p.metrics["responses_processed"] = 0

	return nil
}

// Shutdown 关闭插件
func (p *DefaultPlugin) Shutdown(ctx context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 清理插件资源
	p.metrics["shutdown_at"] = time.Now().Unix()

	return nil
}

// ProcessRequest 处理HTTP请求
func (p *DefaultPlugin) ProcessRequest(ctx context.Context, req *http.Request) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 更新请求处理计数
	if val, ok := p.metrics["requests_processed"].(int); ok {
		p.metrics["requests_processed"] = val + 1
	} else {
		p.metrics["requests_processed"] = 1
	}

	return nil
}

// ProcessResponse 处理HTTP响应
func (p *DefaultPlugin) ProcessResponse(ctx context.Context, w http.ResponseWriter, req *http.Request, status int, body []byte) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 更新响应处理计数
	if val, ok := p.metrics["responses_processed"].(int); ok {
		p.metrics["responses_processed"] = val + 1
	} else {
		p.metrics["responses_processed"] = 1
	}

	return nil
}

// GetConfig 获取插件配置
func (p *DefaultPlugin) GetConfig() map[string]interface{} {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.config
}

// SetConfig 设置插件配置
func (p *DefaultPlugin) SetConfig(config map[string]interface{}) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.config = config
	return nil
}

// IsEnabled 检查插件是否启用
func (p *DefaultPlugin) IsEnabled() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.enabled
}

// Enable 启用插件
func (p *DefaultPlugin) Enable() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.enabled = true
}

// Disable 禁用插件
func (p *DefaultPlugin) Disable() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.enabled = false
}

// GetMetrics 获取插件指标
func (p *DefaultPlugin) GetMetrics() map[string]interface{} {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.metrics
}
