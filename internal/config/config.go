package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ConfigChangeHandler 配置变化处理函数类型
// 当配置发生变化时，会调用所有注册的处理函数
// 参数:
//   *Config: 变化后的新配置

type ConfigChangeHandler func(*Config)

// ConfigManager 配置管理器，用于管理配置和热重载
// 实现了配置的加载、验证、保存、热重载等功能
// 使用单例模式确保全局只有一个配置管理器实例
//
// 字段:
//   mutex: 读写锁，用于保证并发安全
//   config: 当前配置对象
//   configPath: 配置文件路径
//   handlers: 配置变化处理函数列表
//   lastModified: 配置文件最后修改时间
//   watcherRunning: 配置文件监控是否运行
//   closeChan: 关闭监控的通道

type ConfigManager struct {
	mutex          sync.RWMutex
	config         *Config
	configPath     string
	handlers       []ConfigChangeHandler
	lastModified   time.Time
	watcherRunning bool
	closeChan      chan struct{}
}

var (
	instance *ConfigManager
	once     sync.Once
)

// DirsConfig 目录配置结构体
// 定义应用程序所需的各种目录路径
//
// 字段:
//   DataDir: 数据目录，用于存储应用程序数据
//   StaticDir: 静态文件目录，用于存储静态资源
//   CertsDir: 证书目录，用于存储SSL证书
//   AdminStaticDir: 管理控制台静态文件目录，用于存储管理界面的静态资源

type DirsConfig struct {
	DataDir        string `yaml:"data_dir" json:"data_dir"`                 // 数据目录
	StaticDir      string `yaml:"static_dir" json:"static_dir"`             // 静态文件目录
	CertsDir       string `yaml:"certs_dir" json:"certs_dir"`               // 证书目录
	AdminStaticDir string `yaml:"admin_static_dir" json:"admin_static_dir"` // 管理控制台静态文件目录
}

// SiteConfig 站点配置结构体
// 定义单个站点的完整配置信息
//
// 字段:
//   ID: 站点唯一ID，用于标识站点
//   Name: 站点名称，用于显示
//   Domains: 站点绑定的域名列表，支持多个域名
//   Port: 站点监听的端口号
//   Mode: 站点运行模式，可选值：proxy(代理模式), static(静态资源模式), redirect(重定向模式)
//   Proxy: 代理配置，当Mode为proxy时使用
//   Redirect: 重定向配置，当Mode为redirect时使用
//   Firewall: 防火墙配置，站点级别的安全防护设置
//   Prerender: 渲染预热配置，用于SEO优化
//   Routing: 路由配置，用于自定义请求路由
//   FileIntegrityConfig: 网页防篡改配置，用于保护静态资源完整性

type SiteConfig struct {
	// 站点基本信息
	ID      string   `yaml:"id" json:"id"` // 站点唯一ID
	Name    string   `yaml:"name" json:"name"`
	Domains []string `yaml:"domains" json:"domains"` // 支持多个域名解析到同一个站点
	// 站点端口配置，支持一个站点一个端口
	Port int `yaml:"port" json:"port"`
	// 站点模式：proxy(代理已有应用), static(静态资源站), redirect(重定向)
	Mode string `yaml:"mode" json:"mode"`
	// 代理配置
	Proxy ProxyConfig `yaml:"proxy" json:"proxy"`
	// 重定向配置
	Redirect RedirectConfig `yaml:"redirect" json:"redirect"`
	// 防火墙配置
	Firewall FirewallConfig `yaml:"firewall" json:"firewall"`
	// 渲染预热配置
	Prerender PrerenderConfig `yaml:"prerender" json:"prerender"`
	// 路由配置
	Routing RoutingConfig `yaml:"routing" json:"routing"`
	// 网页防篡改配置
	FileIntegrityConfig FileIntegrityConfig `yaml:"file_integrity" json:"FileIntegrityConfig"`
}

// FileIntegrityConfig 网页防篡改配置结构体
// 用于配置网页文件完整性检查
//
// 字段:
//   Enabled: 是否启用网页防篡改检查
//   CheckInterval: 检查间隔，单位为秒
//   HashAlgorithm: 哈希算法，可选值：md5, sha256等

type FileIntegrityConfig struct {
	Enabled       bool   `yaml:"enabled" json:"Enabled"`
	CheckInterval int    `yaml:"check_interval" json:"CheckInterval"` // 检查间隔（秒）
	HashAlgorithm string `yaml:"hash_algorithm" json:"HashAlgorithm"` // 哈希算法（md5, sha256等）
}

// RedirectConfig 重定向配置结构体
// 用于配置站点重定向规则
//
// 字段:
//   StatusCode: 重定向状态码，如301(永久重定向), 302(临时重定向)
//   TargetURL: 重定向目标URL

type RedirectConfig struct {
	StatusCode int    `yaml:"status_code" json:"StatusCode"`
	TargetURL  string `yaml:"target_url" json:"TargetURL"`
}

// ProxyConfig 代理配置结构体
// 用于配置站点代理规则
//
// 字段:
//   TargetURL: 代理目标URL，即真实后端服务地址

type ProxyConfig struct {
	TargetURL string `yaml:"target_url" json:"TargetURL"`
}

// Config 应用全局配置结构体
// 定义整个应用程序的全局配置
//
// 字段:
//   Server: 服务器配置，如监听地址、端口等
//   Dirs: 目录配置，定义应用程序使用的各种目录
//   Cache: 缓存配置，定义缓存类型和相关参数
//   Storage: 存储配置，定义数据存储类型和相关参数
//   Monitoring: 监控配置，定义监控相关参数
//   Sites: 站点列表，包含所有配置的站点

type Config struct {
	// 服务器配置
	Server ServerConfig `yaml:"server"`
	// 目录配置
	Dirs DirsConfig `yaml:"dirs"`
	// 缓存配置
	Cache CacheConfig `yaml:"cache"`
	// 存储配置
	Storage StorageConfig `yaml:"storage"`
	// 监控配置
	Monitoring MonitoringConfig `yaml:"monitoring"`
	// 站点列表
	Sites []SiteConfig `yaml:"sites"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Address     string `yaml:"address"`
	APIPort     int    `yaml:"api_port"`
	ConsolePort int    `yaml:"console_port"`
}

// FirewallConfig 防火墙配置
type FirewallConfig struct {
	Enabled         bool            `yaml:"enabled" json:"Enabled"`
	RulesPath       string          `yaml:"rules_path" json:"RulesPath"`
	ActionConfig    ActionConfig    `yaml:"action" json:"ActionConfig"`
	GeoIPConfig     GeoIPConfig     `yaml:"geoip" json:"GeoIPConfig"`
	RateLimitConfig RateLimitConfig `yaml:"rate_limit" json:"RateLimitConfig"`
}

// GeoIPConfig 地理位置访问控制配置
type GeoIPConfig struct {
	Enabled   bool     `yaml:"enabled" json:"Enabled"`
	AllowList []string `yaml:"allow_list" json:"AllowList"` // 允许的国家/地区代码列表
	BlockList []string `yaml:"block_list" json:"BlockList"` // 阻止的国家/地区代码列表
}

// RateLimitConfig 频率限制配置
type RateLimitConfig struct {
	Enabled  bool `yaml:"enabled" json:"Enabled"`
	Requests int  `yaml:"requests" json:"Requests"` // 时间窗口内允许的请求数
	Window   int  `yaml:"window" json:"Window"`     // 时间窗口（秒）
	BanTime  int  `yaml:"ban_time" json:"BanTime"`  // 封禁时间（秒）
}

// ActionConfig 防火墙动作配置
type ActionConfig struct {
	DefaultAction string `yaml:"default_action" json:"DefaultAction"`
	BlockMessage  string `yaml:"block_message" json:"BlockMessage"`
}

// PrerenderConfig 渲染预热配置
type PrerenderConfig struct {
	Enabled           bool          `yaml:"enabled" json:"Enabled"`
	PoolSize          int           `yaml:"pool_size" json:"PoolSize"`
	MinPoolSize       int           `yaml:"min_pool_size" json:"MinPoolSize"`
	MaxPoolSize       int           `yaml:"max_pool_size" json:"MaxPoolSize"`
	Timeout           int           `yaml:"timeout" json:"Timeout"`
	CacheTTL          int           `yaml:"cache_ttl" json:"CacheTTL"`
	IdleTimeout       int           `yaml:"idle_timeout" json:"IdleTimeout"`
	DynamicScaling    bool          `yaml:"dynamic_scaling" json:"DynamicScaling"`
	ScalingFactor     float64       `yaml:"scaling_factor" json:"ScalingFactor"`
	ScalingInterval   int           `yaml:"scaling_interval" json:"ScalingInterval"`
	Preheat           PreheatConfig `yaml:"preheat" json:"Preheat"`
	CrawlerHeaders    []string      `yaml:"crawler_headers" json:"CrawlerHeaders"`        // 爬虫协议头列表
	UseDefaultHeaders bool          `yaml:"use_default_headers" json:"UseDefaultHeaders"` // 是否使用默认爬虫协议头
}

// PreheatConfig 缓存预热配置
type PreheatConfig struct {
	Enabled         bool   `yaml:"enabled" json:"Enabled"`
	SitemapURL      string `yaml:"sitemap_url" json:"SitemapURL"`
	Schedule        string `yaml:"schedule" json:"Schedule"`
	Concurrency     int    `yaml:"concurrency" json:"Concurrency"`
	DefaultPriority int    `yaml:"default_priority" json:"DefaultPriority"`
	MaxDepth        int    `yaml:"max_depth" json:"MaxDepth"` // 爬取深度
}

// RoutingConfig 路由配置
type RoutingConfig struct {
	Rules []RouteRule `yaml:"rules" json:"Rules"`
}

// RouteRule 路由规则
type RouteRule struct {
	ID       string `yaml:"id" json:"ID"`
	Pattern  string `yaml:"pattern" json:"Pattern"`
	Action   string `yaml:"action" json:"Action"`
	Priority int    `yaml:"priority" json:"Priority"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Type       string `yaml:"type"`
	RedisURL   string `yaml:"redis_url"`
	MemorySize int    `yaml:"memory_size"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	Type        string `yaml:"type"`
	PostgresURL string `yaml:"postgres_url"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	Enabled           bool   `yaml:"enabled"`
	PrometheusAddress string `yaml:"prometheus_address"`
}

// GetInstance 获取配置管理器实例
type ConfigManagerInterface interface {
	GetConfig() *Config
	AddConfigChangeHandler(handler ConfigChangeHandler)
	StartWatching() error
	StopWatching()
}

// GetInstance 获取配置管理器实例
func GetInstance() *ConfigManager {
	once.Do(func() {
		instance = &ConfigManager{
			config:    defaultConfig(),
			closeChan: make(chan struct{}),
		}
	})
	return instance
}

// LoadConfig 从环境变量和YAML配置文件加载配置
func LoadConfig(configPath string) (*Config, error) {
	manager := GetInstance()
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	// 创建默认配置
	cfg := defaultConfig()

	// 如果指定了配置文件路径，从文件加载配置
	if configPath != "" {
		file, err := ioutil.ReadFile(configPath)
		if err != nil {
			return nil, err
		}

		if err := yaml.Unmarshal(file, cfg); err != nil {
			return nil, err
		}

		// 保存配置文件路径和修改时间
		manager.configPath = configPath
		info, err := os.Stat(configPath)
		if err == nil {
			manager.lastModified = info.ModTime()
		}
	}

	// 从环境变量加载配置，覆盖文件配置
	loadFromEnv(cfg)

	// 更新配置
	manager.config = cfg

	return cfg, nil
}

// GetConfig 获取当前配置
func (cm *ConfigManager) GetConfig() *Config {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.config
}

// ValidateConfig 验证配置的合法性
func (cm *ConfigManager) ValidateConfig(config *Config) error {
	// 验证服务器配置
	if config.Server.Address == "" {
		config.Server.Address = "0.0.0.0" // 使用默认地址
	}

	// 验证站点配置
	for i, site := range config.Sites {
		// 验证站点ID
		if site.ID == "" {
			return fmt.Errorf("site at index %d has no ID", i)
		}

		// 验证站点名称
		if site.Name == "" {
			return fmt.Errorf("site %s has no name", site.ID)
		}

		// 验证站点域名
		if len(site.Domains) == 0 {
			return fmt.Errorf("site %s has no domains", site.ID)
		}

		// 验证站点模式
		validModes := map[string]bool{"proxy": true, "static": true, "redirect": true}
		if !validModes[site.Mode] {
			return fmt.Errorf("site %s has invalid mode: %s", site.ID, site.Mode)
		}

		// 根据站点模式验证特定配置
		switch site.Mode {
		case "proxy":
			if site.Proxy.TargetURL == "" {
				return fmt.Errorf("site %s is in proxy mode but has no target URL", site.ID)
			}
		case "redirect":
			if site.Redirect.TargetURL == "" {
				return fmt.Errorf("site %s is in redirect mode but has no target URL", site.ID)
			}
		}

		// 验证渲染预热配置
		if site.Prerender.Enabled {
			if site.Prerender.PoolSize < 1 {
				site.Prerender.PoolSize = 1 // 使用默认值
			}
		}
	}

	return nil
}

// SaveConfig 保存配置到文件
func (cm *ConfigManager) SaveConfig() error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.configPath == "" {
		return nil // 没有配置文件路径，无法保存
	}

	// 验证配置
	if err := cm.ValidateConfig(cm.config); err != nil {
		return err
	}

	// 序列化配置为YAML
	content, err := yaml.Marshal(cm.config)
	if err != nil {
		return err
	}

	// 写入配置文件
	if err := os.WriteFile(cm.configPath, content, 0644); err != nil {
		return err
	}

	// 更新配置文件修改时间
	info, err := os.Stat(cm.configPath)
	if err == nil {
		cm.lastModified = info.ModTime()
	}

	return nil
}

// UpdateConfig 更新配置
func (cm *ConfigManager) UpdateConfig(newConfig *Config) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// 更新配置
	cm.config = newConfig

	// 通知所有配置变化处理函数
	for _, handler := range cm.handlers {
		go handler(newConfig) // 异步调用，避免阻塞
	}
}

// AddConfigChangeHandler 添加配置变化处理函数
func (cm *ConfigManager) AddConfigChangeHandler(handler ConfigChangeHandler) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.handlers = append(cm.handlers, handler)
}

// StartWatching 开始监控配置文件变化
func (cm *ConfigManager) StartWatching() error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.configPath == "" {
		return nil // 没有配置文件，无需监控
	}

	if cm.watcherRunning {
		return nil // 已经在监控
	}

	cm.watcherRunning = true
	go cm.watchConfig()
	return nil
}

// StopWatching 停止监控配置文件变化
func (cm *ConfigManager) StopWatching() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if !cm.watcherRunning {
		return
	}

	cm.watcherRunning = false
	close(cm.closeChan)
	cm.closeChan = make(chan struct{}) // 重置通道
}

// watchConfig 监控配置文件变化
func (cm *ConfigManager) watchConfig() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.checkAndReload()
		case <-cm.closeChan:
			return
		}
	}
}

// checkAndReload 检查配置文件是否变化，如果变化则重新加载
func (cm *ConfigManager) checkAndReload() {
	cm.mutex.RLock()
	configPath := cm.configPath
	lastModified := cm.lastModified
	cm.mutex.RUnlock()

	if configPath == "" {
		return
	}

	// 检查文件是否存在
	info, err := os.Stat(configPath)
	if err != nil {
		return
	}

	// 检查文件是否被修改
	if !info.ModTime().After(lastModified) {
		return
	}

	// 重新加载配置
	cm.reloadConfig()
}

// reloadConfig 重新加载配置
func (cm *ConfigManager) reloadConfig() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// 创建默认配置
	cfg := defaultConfig()

	// 从文件加载配置
	file, err := ioutil.ReadFile(cm.configPath)
	if err != nil {
		return
	}

	if err := yaml.Unmarshal(file, cfg); err != nil {
		return
	}

	// 从环境变量加载配置，覆盖文件配置
	loadFromEnv(cfg)

	// 验证配置
	if err := cm.ValidateConfig(cfg); err != nil {
		fmt.Printf("Config validation failed: %v\n", err)
		return
	}

	// 保存修改时间
	info, _ := os.Stat(cm.configPath)
	if info != nil {
		cm.lastModified = info.ModTime()
	}

	// 更新配置
	cm.config = cfg

	// 通知所有配置变化处理函数
	for _, handler := range cm.handlers {
		go handler(cfg) // 异步调用，避免阻塞
	}
}

// defaultConfig 创建默认配置
func defaultConfig() *Config {
	// 默认站点配置
	defaultSite := SiteConfig{
		ID:      "default", // 默认站点ID
		Name:    "默认站点",
		Domains: []string{"localhost"}, // 支持多个域名
		Port:    8081,                  // 默认端口
		Mode:    "static",              // 默认模式：静态资源站
		// 代理配置
		Proxy: ProxyConfig{
			TargetURL: "",
		},
		// 重定向配置
		Redirect: RedirectConfig{
			StatusCode: 301,
			TargetURL:  "",
		},
		Firewall: FirewallConfig{
			Enabled:   true,
			RulesPath: "/etc/prerender-shield/rules",
			ActionConfig: ActionConfig{
				DefaultAction: "block",
				BlockMessage:  "Request blocked by firewall",
			},
			GeoIPConfig: GeoIPConfig{
				Enabled:   false,
				AllowList: []string{},
				BlockList: []string{},
			},
			RateLimitConfig: RateLimitConfig{
				Enabled:  false,
				Requests: 100,
				Window:   60,
				BanTime:  3600,
			},
		},
		Prerender: PrerenderConfig{
			Enabled:           true,
			PoolSize:          5,
			MinPoolSize:       2,
			MaxPoolSize:       20,
			Timeout:           30,
			CacheTTL:          3600,
			IdleTimeout:       300,
			DynamicScaling:    true,
			ScalingFactor:     0.5,
			ScalingInterval:   60,
			CrawlerHeaders:    []string{}, // 默认空列表
			UseDefaultHeaders: true,       // 默认使用默认爬虫协议头
			Preheat: PreheatConfig{
				Enabled:         false,
				SitemapURL:      "",
				Schedule:        "0 0 * * *",
				Concurrency:     5,
				DefaultPriority: 0,
				MaxDepth:        3, // 默认爬取深度为3
			},
		},
		Routing: RoutingConfig{
			Rules: []RouteRule{},
		},
		// 网页防篡改配置
		FileIntegrityConfig: FileIntegrityConfig{
			Enabled:       false,
			CheckInterval: 300, // 5分钟检查一次
			HashAlgorithm: "sha256",
		},
	}

	return &Config{
		Server: ServerConfig{
			Address:     "0.0.0.0",
			APIPort:     9598,
			ConsolePort: 9597,
		},
		Dirs: DirsConfig{
			DataDir:        "./data",     // 数据目录
			StaticDir:      "./static",   // 静态文件目录
			CertsDir:       "./certs",    // 证书目录
			AdminStaticDir: "./web/dist", // 管理控制台静态文件目录
		},
		Cache: CacheConfig{
			Type:       "memory",
			RedisURL:   "localhost:6379",
			MemorySize: 1000,
		},
		Storage: StorageConfig{
			Type:        "postgres",
			PostgresURL: "postgres://prerender:prerender@localhost:5432/prerender?sslmode=disable",
		},
		Monitoring: MonitoringConfig{
			Enabled:           true,
			PrometheusAddress: ":9090",
		},
		Sites: []SiteConfig{defaultSite},
	}
}

// loadFromEnv 从环境变量加载配置，覆盖现有配置
func loadFromEnv(cfg *Config) {
	// 服务器配置
	cfg.Server.Address = getEnv("SERVER_ADDRESS", cfg.Server.Address)
	cfg.Server.APIPort = getEnvAsInt("SERVER_API_PORT", cfg.Server.APIPort)
	cfg.Server.ConsolePort = getEnvAsInt("SERVER_CONSOLE_PORT", cfg.Server.ConsolePort)

	// 目录配置
	cfg.Dirs.DataDir = getEnv("DIRS_DATA_DIR", cfg.Dirs.DataDir)
	cfg.Dirs.StaticDir = getEnv("DIRS_STATIC_DIR", cfg.Dirs.StaticDir)
	cfg.Dirs.CertsDir = getEnv("DIRS_CERTS_DIR", cfg.Dirs.CertsDir)
	cfg.Dirs.AdminStaticDir = getEnv("DIRS_ADMIN_STATIC_DIR", cfg.Dirs.AdminStaticDir)

	// 缓存配置
	cfg.Cache.Type = getEnv("CACHE_TYPE", cfg.Cache.Type)
	cfg.Cache.RedisURL = getEnv("CACHE_REDIS_URL", cfg.Cache.RedisURL)
	cfg.Cache.MemorySize = getEnvAsInt("CACHE_MEMORY_SIZE", cfg.Cache.MemorySize)

	// 存储配置
	cfg.Storage.Type = getEnv("STORAGE_TYPE", cfg.Storage.Type)
	cfg.Storage.PostgresURL = getEnv("STORAGE_POSTGRES_URL", cfg.Storage.PostgresURL)

	// 监控配置
	cfg.Monitoring.Enabled = getEnvAsBool("MONITORING_ENABLED", cfg.Monitoring.Enabled)
	cfg.Monitoring.PrometheusAddress = getEnv("MONITORING_PROMETHEUS_ADDRESS", cfg.Monitoring.PrometheusAddress)

	// 注意：站点配置主要通过 YAML 文件管理，环境变量加载暂不支持站点级配置
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvAsInt 获取环境变量并转换为整数，如果不存在或转换失败则返回默认值
func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvAsBool 获取环境变量并转换为布尔值，如果不存在或转换失败则返回默认值
func getEnvAsBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// getEnvAsFloat 获取环境变量并转换为float64类型，如果不存在或转换失败则返回默认值
func getEnvAsFloat(key string, defaultValue float64) float64 {
	if value, exists := os.LookupEnv(key); exists {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}
