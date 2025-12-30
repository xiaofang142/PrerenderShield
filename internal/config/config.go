package config

import (
	"io/ioutil"
	"os"
	"strconv"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ConfigChangeHandler 配置变化处理函数

type ConfigChangeHandler func(*Config)

// ConfigManager 配置管理器，用于管理配置和热重载
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

// DirsConfig 目录配置
type DirsConfig struct {
	DataDir        string `yaml:"data_dir" json:"data_dir"`                 // 数据目录
	StaticDir      string `yaml:"static_dir" json:"static_dir"`             // 静态文件目录
	CertsDir       string `yaml:"certs_dir" json:"certs_dir"`               // 证书目录
	AdminStaticDir string `yaml:"admin_static_dir" json:"admin_static_dir"` // 管理控制台静态文件目录
}

// SiteConfig 站点配置
type SiteConfig struct {
	// 站点基本信息
	ID      string   `yaml:"id" json:"id"` // 站点唯一ID
	Name    string   `yaml:"name" json:"name"`
	Domains []string `yaml:"domains" json:"domains"` // 支持多个域名解析到同一个站点
	// 站点端口配置，支持一个站点一个端口
	Port int `yaml:"port" json:"port"`
	// 代理配置
	Proxy ProxyConfig `yaml:"proxy" json:"proxy"`
	// 防火墙配置
	Firewall FirewallConfig `yaml:"firewall" json:"firewall"`
	// 预渲染配置
	Prerender PrerenderConfig `yaml:"prerender" json:"prerender"`
	// 路由配置
	Routing RoutingConfig `yaml:"routing" json:"routing"`
	// SSL配置
	SSL SSLConfig `yaml:"ssl" json:"ssl"`
	// 网页防篡改配置
	FileIntegrityConfig FileIntegrityConfig `yaml:"file_integrity" json:"FileIntegrityConfig"`
}

// FileIntegrityConfig 网页防篡改配置
type FileIntegrityConfig struct {
	Enabled       bool   `yaml:"enabled" json:"Enabled"`
	CheckInterval int    `yaml:"check_interval" json:"CheckInterval"` // 检查间隔（秒）
	HashAlgorithm string `yaml:"hash_algorithm" json:"HashAlgorithm"` // 哈希算法（md5, sha256等）
}

// ProxyConfig 代理配置
type ProxyConfig struct {
	Enabled   bool   `yaml:"enabled" json:"Enabled"`
	TargetURL string `yaml:"target_url" json:"TargetURL"`
	Type      string `yaml:"type" json:"Type"` // direct: 直接解析域名, proxy: 反向代理
}

// Config 应用全局配置
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
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
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

// PrerenderConfig 预渲染配置
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

// SSLConfig SSL配置
type SSLConfig struct {
	Enabled       bool     `yaml:"enabled" json:"Enabled"`
	LetEncrypt    bool     `yaml:"let_encrypt" json:"LetEncrypt"`
	Domains       []string `yaml:"domains" json:"Domains"`
	ACMEEmail     string   `yaml:"acme_email" json:"ACMEEmail"`
	ACMEServer    string   `yaml:"acme_server" json:"ACMEServer"`
	ACMEChallenge string   `yaml:"acme_challenge" json:"ACMEChallenge"`
	CertPath      string   `yaml:"cert_path" json:"CertPath"`
	KeyPath       string   `yaml:"key_path" json:"KeyPath"`
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
			},
		},
		Routing: RoutingConfig{
			Rules: []RouteRule{},
		},
		SSL: SSLConfig{
			Enabled:       false,
			LetEncrypt:    false,
			Domains:       []string{},
			ACMEEmail:     "",
			ACMEServer:    "https://acme-v02.api.letsencrypt.org/directory",
			ACMEChallenge: "http01",
			CertPath:      "/etc/prerender-shield/certs/cert.pem",
			KeyPath:       "/etc/prerender-shield/certs/key.pem",
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
			Address: "0.0.0.0",
			Port:    8080,
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
	cfg.Server.Port = getEnvAsInt("SERVER_PORT", cfg.Server.Port)

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
