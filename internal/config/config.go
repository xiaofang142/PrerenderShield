package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/redis"
	"regexp"
	"strconv"
	"strings"
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
	redisClient    *redis.Client
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
	FileIntegrityConfig FileIntegrityConfig `yaml:"file_integrity" json:"file_integrity"`
}

// FileIntegrityConfig 网页防篡改配置结构体
// 用于配置网页文件完整性检查
//
// 字段:
//   Enabled: 是否启用网页防篡改检查
//   CheckInterval: 检查间隔，单位为秒
//   HashAlgorithm: 哈希算法，可选值：md5, sha256等

type FileIntegrityConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	CheckInterval int    `yaml:"check_interval" json:"check_interval"` // 检查间隔（秒）
	HashAlgorithm string `yaml:"hash_algorithm" json:"hash_algorithm"` // 哈希算法（md5, sha256等）
}

// RedirectConfig 重定向配置结构体
// 用于配置站点重定向规则
//
// 字段:
//   StatusCode: 重定向状态码，如301(永久重定向), 302(临时重定向)
//   TargetURL: 重定向目标URL

type RedirectConfig struct {
	StatusCode int    `yaml:"status_code" json:"status_code"`
	TargetURL  string `yaml:"target_url" json:"target_url"`
}

// ProxyConfig 代理配置结构体
// 用于配置站点代理规则
//
// 字段:
//   TargetURL: 代理目标URL，即真实后端服务地址

type ProxyConfig struct {
	TargetURL string `yaml:"target_url" json:"target_url"`
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
	// 应用配置
	App AppConfig `yaml:"app"`
	// SSL 配置
	SSL SSLConfig `yaml:"ssl"`
	// 站点列表
	Sites []SiteConfig `yaml:"sites"`
}

// SSLConfig SSL 证书配置
type SSLConfig struct {
	Enabled         bool          `yaml:"enabled" json:"enabled"`                     // 是否启用 SSL
	AutoRenew       bool          `yaml:"auto_renew" json:"auto_renew"`               // 自动续签
	Email           string        `yaml:"email" json:"email"`                         // ACME 账户邮箱
	Production      bool          `yaml:"production" json:"production"`               // 是否使用生产环境（Let's Encrypt）
	HTTPPort        int           `yaml:"http_port" json:"http_port"`                 // HTTP-01 挑战监听端口
	CheckInterval   time.Duration `yaml:"check_interval" json:"check_interval"`       // 检查间隔
	RenewBeforeDays int           `yaml:"renew_before_days" json:"renew_before_days"` // 提前多少天续签
	MaxRetries      int           `yaml:"max_retries" json:"max_retries"`             // 最大重试次数
	RetryDelay      time.Duration `yaml:"retry_delay" json:"retry_delay"`             // 重试间隔
	WebhookURL      string        `yaml:"webhook_url" json:"webhook_url"`             // 通知 Webhook
	DNS             DNSConfig     `yaml:"dns" json:"dns"`                             // DNS 配置（用于 DNS-01 挑战）
}

// DNSConfig DNS 配置
type DNSConfig struct {
	Provider    string            `yaml:"provider" json:"provider"`       // DNS 服务商：cloudflare, aliyun, tencentcloud 等
	Credentials map[string]string `yaml:"credentials" json:"credentials"` // API 凭证
}

// AppConfig 应用全局配置
type AppConfig struct {
	Version     string `yaml:"version" json:"version"`
	OfficialURL string `yaml:"official_url" json:"official_url"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Address      string `yaml:"address"`
	APIPort      int    `yaml:"api_port"`
	ConsolePort  int    `yaml:"console_port"`
	PublicAPIURL string `yaml:"public_api_url"`
}

// FirewallConfig 防火墙配置
type FirewallConfig struct {
	Enabled         bool            `yaml:"enabled" json:"enabled"`
	RulesPath       string          `yaml:"rules_path" json:"rules_path"`
	ActionConfig    ActionConfig    `yaml:"action" json:"action"`
	GeoIPConfig     GeoIPConfig     `yaml:"geoip" json:"geoip"`
	RateLimitConfig RateLimitConfig `yaml:"rate_limit" json:"rate_limit"`
	Blacklist       []string        `yaml:"blacklist" json:"blacklist"`
	Whitelist       []string        `yaml:"whitelist" json:"whitelist"`
}

// GeoIPConfig 地理位置访问控制配置
type GeoIPConfig struct {
	Enabled   bool     `yaml:"enabled" json:"enabled"`
	AllowList []string `yaml:"allow_list" json:"allow_list"` // 允许的国家/地区代码列表
	BlockList []string `yaml:"block_list" json:"block_list"` // 阻止的国家/地区代码列表
}

// RateLimitConfig 频率限制配置
type RateLimitConfig struct {
	Enabled  bool `yaml:"enabled" json:"enabled"`
	Requests int  `yaml:"requests" json:"requests"` // 时间窗口内允许的请求数
	Window   int  `yaml:"window" json:"window"`     // 时间窗口（秒）
	BanTime  int  `yaml:"ban_time" json:"ban_time"` // 封禁时间（秒）
}

// ActionConfig 防火墙动作配置
type ActionConfig struct {
	DefaultAction string `yaml:"default_action" json:"default_action"`
	BlockMessage  string `yaml:"block_message" json:"block_message"`
}

// PrerenderConfig 渲染预热配置
type PrerenderConfig struct {
	Enabled           bool          `yaml:"enabled" json:"enabled"`
	PoolSize          int           `yaml:"pool_size" json:"pool_size"`
	MinPoolSize       int           `yaml:"min_pool_size" json:"min_pool_size"`
	MaxPoolSize       int           `yaml:"max_pool_size" json:"max_pool_size"`
	Timeout           int           `yaml:"timeout" json:"timeout"`
	CacheTTL          int           `yaml:"cache_ttl" json:"cache_ttl"`
	IdleTimeout       int           `yaml:"idle_timeout" json:"idle_timeout"`
	DynamicScaling    bool          `yaml:"dynamic_scaling" json:"dynamic_scaling"`
	ScalingFactor     float64       `yaml:"scaling_factor" json:"scaling_factor"`
	ScalingInterval   int           `yaml:"scaling_interval" json:"scaling_interval"`
	Preheat           PreheatConfig `yaml:"preheat" json:"preheat"`
	Push              PushConfig    `yaml:"push" json:"push"`
	CrawlerHeaders    []string      `yaml:"crawler_headers" json:"crawler_headers"`         // 爬虫协议头列表
	UseDefaultHeaders bool          `yaml:"use_default_headers" json:"use_default_headers"` // 是否使用默认爬虫协议头
}

// PreheatConfig 缓存预热配置
type PreheatConfig struct {
	Enabled         bool   `yaml:"enabled" json:"enabled"`
	SitemapURL      string `yaml:"sitemap_url" json:"sitemap_url"`
	Schedule        string `yaml:"schedule" json:"schedule"`
	Concurrency     int    `yaml:"concurrency" json:"concurrency"`
	DefaultPriority int    `yaml:"default_priority" json:"default_priority"`
	MaxDepth        int    `yaml:"max_depth" json:"max_depth"` // 爬取深度
}

// PushConfig 搜索引擎推送配置
type PushConfig struct {
	Enabled         bool   `yaml:"enabled" json:"enabled"`
	BaiduAPI        string `yaml:"baidu_api" json:"baidu_api"`
	BaiduToken      string `yaml:"baidu_token" json:"baidu_token"`
	BingAPI         string `yaml:"bing_api" json:"bing_api"`
	BingToken       string `yaml:"bing_token" json:"bing_token"`
	BaiduDailyLimit int    `yaml:"baidu_daily_limit" json:"baidu_daily_limit"`
	BingDailyLimit  int    `yaml:"bing_daily_limit" json:"bing_daily_limit"`
	PushDomain      string `yaml:"push_domain" json:"push_domain"`
}

// RoutingConfig 路由配置
type RoutingConfig struct {
	Rules []RouteRule `yaml:"rules" json:"rules"`
}

// RouteRule 路由规则
type RouteRule struct {
	ID       string `yaml:"id" json:"id"`
	Pattern  string `yaml:"pattern" json:"pattern"`
	Action   string `yaml:"action" json:"action"`
	Priority int    `yaml:"priority" json:"priority"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Type          string `yaml:"type"`
	RedisURL      string `yaml:"redis_url"`
	RedisPassword string `yaml:"redis_password"`
	RedisDB       int    `yaml:"redis_db"`
	MemorySize    int    `yaml:"memory_size"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	Type string `yaml:"type"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	Enabled           bool         `yaml:"enabled"`
	PrometheusAddress string       `yaml:"prometheus_address"`
	Alerting          AlertingConfig `yaml:"alerting"`
	// 监控数据持久化配置
	MetricsPersistence MetricsPersistenceConfig `yaml:"metrics_persistence"`
}

// MetricsPersistenceConfig 监控数据持久化配置
type MetricsPersistenceConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Interval       int    `yaml:"interval"` // 持久化间隔（秒），默认 300 秒
	RetentionHours int    `yaml:"retention_hours"` // 数据保留时间（小时），默认 24 小时
	AggregateEnabled bool `yaml:"aggregate_enabled"` // 是否启用数据聚合
	AggregateInterval int `yaml:"aggregate_interval"` // 聚合间隔（秒），默认 3600 秒（1 小时）
}

// AlertingConfig 告警配置
type AlertingConfig struct {
	Enabled     bool   `yaml:"enabled"`
	RulesPath   string `yaml:"rules_path"`
	Notifications NotificationsConfig `yaml:"notifications"`
}

// NotificationsConfig 通知配置
type NotificationsConfig struct {
	Webhook WebhookNotificationConfig `yaml:"webhook"`
	Email   EmailNotificationConfig   `yaml:"email"`
}

// WebhookNotificationConfig Webhook 通知配置
type WebhookNotificationConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	Secret  string `yaml:"secret"`
}

// EmailNotificationConfig 邮件通知配置
type EmailNotificationConfig struct {
	Enabled  bool   `yaml:"enabled"`
	SMTPHost string `yaml:"smtp_host"`
	SMTPPort int    `yaml:"smtp_port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
	To       []string `yaml:"to"`
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

// ResetInstance 重置配置管理器实例，仅用于测试
// 测试完成后需要调用 GetInstance() 重新初始化
func ResetInstance() {
	instance = nil
	once = sync.Once{}
}

// LoadConfig 从环境变量和YAML配置文件加载配置
func LoadConfig(configPath string) (*Config, error) {
	manager := GetInstance()
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	// 创建默认配置
	cfg := defaultConfig()

	// 获取应用程序二进制文件的位置
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("无法获取应用程序路径: %v", err)
	}
	// 获取二进制文件所在目录
	appDir := filepath.Dir(exePath)

	// 如果是在 go run 模式下运行（可执行文件在临时目录），使用当前工作目录
	// 这确保了在开发环境下重启后静态资源路径不变
	var wd string
	if wd, err = os.Getwd(); err != nil {
		wd = appDir
	}

	if strings.Contains(exePath, "/var/folders") || strings.Contains(exePath, "/tmp") || strings.Contains(exePath, "T\\go-build") {
		appDir = wd
	}

	// 如果没有指定配置文件路径，尝试从默认位置加载
	if configPath == "" {
		// 尝试从项目根目录下的configs/config.yml加载
		projectConfigPath := filepath.Join(wd, "configs", "config.yml")
		if _, err := os.Stat(projectConfigPath); err == nil {
			configPath = projectConfigPath
		} else {
			// 尝试从当前目录下的config.yml加载
			localConfigPath := filepath.Join(".", "config.yml")
			if _, err := os.Stat(localConfigPath); err == nil {
				configPath = localConfigPath
			} else {
				// 尝试从应用程序所在目录下的config.yml加载
				appConfigPath := filepath.Join(appDir, "config.yml")
				if _, err := os.Stat(appConfigPath); err == nil {
					configPath = appConfigPath
				} else {
					// 尝试从应用程序所在目录下的config/config.yml加载
					appConfigDirPath := filepath.Join(appDir, "config", "config.yml")
					if _, err := os.Stat(appConfigDirPath); err == nil {
						configPath = appConfigDirPath
					}
				}
			}
		}
	}

	// 如果找到配置文件路径，从文件加载配置
	if configPath != "" {
		// 检查配置文件是否存在
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("配置文件不存在: %s", configPath)
		}

		file, err := ioutil.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("读取配置文件失败: %s, 错误: %v", configPath, err)
		}

		if err := yaml.Unmarshal(file, cfg); err != nil {
			return nil, fmt.Errorf("解析配置文件失败: %s, 错误: %v", configPath, err)
		}

		// 替换配置中的环境变量占位符
		replaceConfigEnvVars(cfg)

		// 保存配置文件路径和修改时间
		manager.configPath = configPath
		info, err := os.Stat(configPath)
		if err != nil {
			fmt.Printf("无法获取配置文件修改时间: %s, 错误: %v\n", configPath, err)
			manager.lastModified = time.Now()
		} else {
			manager.lastModified = info.ModTime()
		}
	}

	// 从环境变量加载配置，覆盖文件配置
	loadFromEnv(cfg)

	// 确保所有目录路径都是绝对路径
	// 处理静态目录
	if !filepath.IsAbs(cfg.Dirs.StaticDir) {
		// 检查路径是否已经包含"bin/"，如果包含则直接使用简单的相对路径
		if strings.HasPrefix(cfg.Dirs.StaticDir, "bin/") {
			// 如果路径是bin/darwin-arm64/static，直接使用static
			cfg.Dirs.StaticDir = filepath.Join(wd, "static")
		} else {
			cfg.Dirs.StaticDir = filepath.Join(wd, cfg.Dirs.StaticDir)
		}
	}

	// 处理数据目录
	if !filepath.IsAbs(cfg.Dirs.DataDir) {
		// 检查路径是否已经包含"bin/"，如果包含则直接使用简单的相对路径
		if strings.HasPrefix(cfg.Dirs.DataDir, "bin/") {
			// 如果路径是bin/darwin-arm64/data，直接使用data
			cfg.Dirs.DataDir = filepath.Join(wd, "data")
		} else {
			cfg.Dirs.DataDir = filepath.Join(wd, cfg.Dirs.DataDir)
		}
	}

	// 处理证书目录
	if !filepath.IsAbs(cfg.Dirs.CertsDir) {
		// 检查路径是否已经包含"bin/"，如果包含则直接使用简单的相对路径
		if strings.HasPrefix(cfg.Dirs.CertsDir, "bin/") {
			// 如果路径是bin/darwin-arm64/certs，直接使用certs
			cfg.Dirs.CertsDir = filepath.Join(wd, "certs")
		} else {
			cfg.Dirs.CertsDir = filepath.Join(wd, cfg.Dirs.CertsDir)
		}
	}

	// 处理管理控制台静态目录
	if !filepath.IsAbs(cfg.Dirs.AdminStaticDir) {
		// 如果配置的是相对路径 ./web，则转换为 bin/web
		if cfg.Dirs.AdminStaticDir == "./web" {
			// 检查当前目录是否已经是bin目录
			if filepath.Base(appDir) == "bin" {
				// 如果当前目录是bin目录，直接使用web子目录
				cfg.Dirs.AdminStaticDir = filepath.Join(appDir, "web")
			} else {
				// 否则使用bin/web目录
				cfg.Dirs.AdminStaticDir = filepath.Join(appDir, "bin", "web")
			}
		} else {
			// 否则使用原来的逻辑
			cfg.Dirs.AdminStaticDir = filepath.Join(wd, cfg.Dirs.AdminStaticDir)
		}
	}

	// 创建必要的目录
	for _, dir := range []string{cfg.Dirs.DataDir, cfg.Dirs.StaticDir, cfg.Dirs.CertsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("无法创建目录: %s, 错误: %v", dir, err)
		}
	}

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
// 关键配置错误会返回错误，非关键配置会使用默认值并记录警告
func (cm *ConfigManager) ValidateConfig(config *Config) error {
	var errs []error
	var warnings []string

	// 验证服务器配置
	if config.Server.Address == "" {
		config.Server.Address = "0.0.0.0" // 使用默认地址
		warnings = append(warnings, "Server address is empty, using default: 0.0.0.0")
	}

	// 验证端口号 - 关键配置，错误时返回错误
	if config.Server.APIPort <= 0 || config.Server.APIPort > 65535 {
		errs = append(errs, fmt.Errorf("invalid API port: %d, must be between 1 and 65535", config.Server.APIPort))
	}

	if config.Server.ConsolePort <= 0 || config.Server.ConsolePort > 65535 {
		errs = append(errs, fmt.Errorf("invalid Console port: %d, must be between 1 and 65535", config.Server.ConsolePort))
	}

	// 验证 PublicAPIURL
	if config.Server.PublicAPIURL == "" {
		config.Server.PublicAPIURL = fmt.Sprintf("http://%s:%d", config.Server.Address, config.Server.APIPort)
		warnings = append(warnings, "Public API URL is empty, using default")
	}

	// 验证目录配置 - 非关键配置，使用默认值
	if config.Dirs.DataDir == "" {
		config.Dirs.DataDir = "./data"
		warnings = append(warnings, "Data directory is empty, using default: ./data")
	}

	if config.Dirs.StaticDir == "" {
		config.Dirs.StaticDir = "./static"
		warnings = append(warnings, "Static directory is empty, using default: ./static")
	}

	if config.Dirs.CertsDir == "" {
		config.Dirs.CertsDir = "./certs"
		warnings = append(warnings, "Certs directory is empty, using default: ./certs")
	}

	if config.Dirs.AdminStaticDir == "" {
		config.Dirs.AdminStaticDir = "./web"
		warnings = append(warnings, "Admin static directory is empty, using default: ./web")
	}

	// 验证缓存配置
	validCacheTypes := map[string]bool{"memory": true, "redis": true}
	if !validCacheTypes[config.Cache.Type] {
		config.Cache.Type = "memory"
		warnings = append(warnings, fmt.Sprintf("Invalid cache type '%s', using default: memory", config.Cache.Type))
	}

	if config.Cache.Type == "redis" {
		if config.Cache.RedisURL == "" {
			config.Cache.RedisURL = "localhost:6379"
			warnings = append(warnings, "Redis URL is empty, using default: localhost:6379")
		}
	}

	if config.Cache.MemorySize <= 0 {
		config.Cache.MemorySize = 1000
		warnings = append(warnings, "Memory size is invalid, using default: 1000")
	}

	// 验证存储配置
	validStorageTypes := map[string]bool{"redis": true, "memory": true}
	if !validStorageTypes[config.Storage.Type] {
		config.Storage.Type = "redis"
		warnings = append(warnings, fmt.Sprintf("Invalid storage type '%s', using default: redis", config.Storage.Type))
	}

	// 验证监控配置
	if config.Monitoring.PrometheusAddress == "" {
		config.Monitoring.PrometheusAddress = ":9090"
		warnings = append(warnings, "Prometheus address is empty, using default: :9090")
	}

	// 验证应用配置
	if config.App.Version == "" {
		config.App.Version = "1.0.0"
		warnings = append(warnings, "App version is empty, using default: 1.0.0")
	}

	// 输出警告日志
	for _, w := range warnings {
		fmt.Printf("[WARN] %s\n", w)
	}

	// 如果有严重错误，返回错误
	if len(errs) > 0 {
		return fmt.Errorf("configuration validation failed: %v", errs)
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

		// 验证域名格式
		for _, domain := range site.Domains {
			if domain == "" {
				return fmt.Errorf("site %s has empty domain", site.ID)
			}
		}

		// 验证站点端口
		if site.Port <= 0 || site.Port > 65535 {
			return fmt.Errorf("site %s has invalid port: %d", site.ID, site.Port)
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

			if site.Redirect.StatusCode < 300 || site.Redirect.StatusCode >= 400 {
				warnings = append(warnings, fmt.Sprintf("Site %s has invalid redirect status code %d, using default: 301", site.ID, site.Redirect.StatusCode))
				site.Redirect.StatusCode = 301
			}
		}

		// 验证渲染预热配置
		if site.Prerender.Enabled {
			if site.Prerender.PoolSize < 1 {
				site.Prerender.PoolSize = 5 // 使用默认池大小
			}

			if site.Prerender.MinPoolSize < 0 {
				site.Prerender.MinPoolSize = 2 // 使用默认最小池大小
			}

			if site.Prerender.MaxPoolSize < site.Prerender.PoolSize {
				site.Prerender.MaxPoolSize = 20 // 使用默认最大池大小
			}

			if site.Prerender.Timeout <= 0 {
				site.Prerender.Timeout = 30 // 使用默认超时时间
			}

			if site.Prerender.CacheTTL <= 0 {
				site.Prerender.CacheTTL = 3600 // 使用默认缓存TTL
			}

			if site.Prerender.IdleTimeout <= 0 {
				site.Prerender.IdleTimeout = 300 // 使用默认空闲超时时间
			}

			if site.Prerender.ScalingInterval <= 0 {
				site.Prerender.ScalingInterval = 60 // 使用默认缩放间隔
			}

			if site.Prerender.Preheat.Enabled {
				if site.Prerender.Preheat.MaxDepth <= 0 {
					site.Prerender.Preheat.MaxDepth = 3 // 使用默认爬取深度
				}

				if site.Prerender.Preheat.Concurrency <= 0 {
					site.Prerender.Preheat.Concurrency = 5 // 使用默认并发数
				}
			}
		}

		// 验证防火墙配置
		if site.Firewall.Enabled {
			if site.Firewall.RulesPath == "" {
				warnings = append(warnings, fmt.Sprintf("Site %s firewall rules path is empty, using default: ./rules", site.ID))
				site.Firewall.RulesPath = "./rules"
			}

			validActions := map[string]bool{"block": true, "allow": true}
			if !validActions[site.Firewall.ActionConfig.DefaultAction] {
				warnings = append(warnings, fmt.Sprintf("Site %s firewall action is invalid, using default: block", site.ID))
				site.Firewall.ActionConfig.DefaultAction = "block"
			}

			if site.Firewall.ActionConfig.BlockMessage == "" {
				site.Firewall.ActionConfig.BlockMessage = "Request blocked by firewall"
			}

			if site.Firewall.RateLimitConfig.Enabled {
				if site.Firewall.RateLimitConfig.Requests <= 0 {
					warnings = append(warnings, fmt.Sprintf("Site %s rate limit requests is invalid, using default: 100", site.ID))
					site.Firewall.RateLimitConfig.Requests = 100
				}

				if site.Firewall.RateLimitConfig.Window <= 0 {
					warnings = append(warnings, fmt.Sprintf("Site %s rate limit window is invalid, using default: 60", site.ID))
					site.Firewall.RateLimitConfig.Window = 60
				}

				if site.Firewall.RateLimitConfig.BanTime <= 0 {
					warnings = append(warnings, fmt.Sprintf("Site %s rate limit ban time is invalid, using default: 3600", site.ID))
					site.Firewall.RateLimitConfig.BanTime = 3600
				}
			}
		}

		// 验证路由配置
		for j, rule := range site.Routing.Rules {
			if rule.ID == "" {
				return fmt.Errorf("site %s has route rule at index %d with no ID", site.ID, j)
			}

			if rule.Pattern == "" {
				return fmt.Errorf("site %s has route rule %s with no pattern", site.ID, rule.ID)
			}

			if rule.Action == "" {
				return fmt.Errorf("site %s has route rule %s with no action", site.ID, rule.ID)
			}
		}

		// 验证网页防篡改配置
		if site.FileIntegrityConfig.Enabled {
			if site.FileIntegrityConfig.CheckInterval <= 0 {
				warnings = append(warnings, fmt.Sprintf("Site %s file integrity check interval is invalid, using default: 300", site.ID))
				site.FileIntegrityConfig.CheckInterval = 300
			}

			validHashAlgorithms := map[string]bool{"md5": true, "sha256": true}
			if !validHashAlgorithms[site.FileIntegrityConfig.HashAlgorithm] {
				warnings = append(warnings, fmt.Sprintf("Site %s hash algorithm is invalid, using default: sha256", site.ID))
				site.FileIntegrityConfig.HashAlgorithm = "sha256"
			}
		}
	}

	// 输出所有警告
	for _, w := range warnings {
		fmt.Printf("[WARN] %s\n", w)
	}

	return nil
}

// SetRedisClient 设置Redis客户端
func (cm *ConfigManager) SetRedisClient(client *redis.Client) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.redisClient = client
}

// SaveSitesToRedis 保存站点配置到Redis
func (cm *ConfigManager) SaveSitesToRedis() error {
	if cm.redisClient == nil {
		return fmt.Errorf("redis client is not set")
	}

	// 序列化站点配置
	data, err := yaml.Marshal(cm.config.Sites)
	if err != nil {
		return err
	}

	// 保存到Redis
	// 使用 GetRawClient 获取原始客户端进行操作
	err = cm.redisClient.GetRawClient().Set(cm.redisClient.Context(), "prerender:config:sites", data, 0).Err()
	if err != nil {
		return err
	}

	logging.DefaultLogger.Info("Sites configuration saved to Redis")
	return nil
}

// LoadSitesFromRedis 从Redis加载站点配置
func (cm *ConfigManager) LoadSitesFromRedis() error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.redisClient == nil {
		return fmt.Errorf("redis client is not set")
	}

	// 从Redis获取配置
	data, err := cm.redisClient.GetRawClient().Get(cm.redisClient.Context(), "prerender:config:sites").Bytes()
	if err != nil {
		// 如果key不存在，说明是首次运行或没有Redis配置
		return err
	}

	// 反序列化配置
	var sites []SiteConfig
	if err := yaml.Unmarshal(data, &sites); err != nil {
		return err
	}

	// 更新配置
	cm.config.Sites = sites
	logging.DefaultLogger.Info("Sites configuration loaded from Redis: %d sites", len(sites))
	return nil
}

// SaveConfig 保存配置到文件和Redis
func (cm *ConfigManager) SaveConfig() error {
	cm.mutex.Lock()
	// 注意：SaveSitesToRedis 内部也需要加锁，这里需要小心死锁
	// 但是 SaveSitesToRedis 的实现中已经加了锁，所以我们不能在这里调用它
	// 我们需要提取保存逻辑或者在 SaveConfig 中直接操作
	defer cm.mutex.Unlock()

	// 1. 保存到 Redis (如果可用)
	if cm.redisClient != nil {
		data, err := yaml.Marshal(cm.config.Sites)
		if err == nil {
			// 使用 context.Background() 避免依赖
			ctx := cm.redisClient.Context()
			if err := cm.redisClient.GetRawClient().Set(ctx, "prerender:config:sites", data, 0).Err(); err != nil {
				logging.DefaultLogger.Error("Failed to save sites to Redis: %v", err)
			} else {
				logging.DefaultLogger.Info("Sites configuration saved to Redis")
			}
		} else {
			logging.DefaultLogger.Error("Failed to marshal sites config: %v", err)
		}
	}

	if cm.configPath == "" {
		return nil // 没有配置文件路径，无法保存到文件
	}
	// ... continues with file saving logic ...

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
		logging.DefaultLogger.Error("重新读取配置文件失败: %s, 错误: %v", cm.configPath, err)
		return
	}

	if err := yaml.Unmarshal(file, cfg); err != nil {
		logging.DefaultLogger.Error("重新读取配置文件失败: %s, 错误: %v", cm.configPath, err)
		return
	}

	// 替换配置中的环境变量占位符
	replaceConfigEnvVars(cfg)

	// 从环境变量加载配置，覆盖文件配置
	loadFromEnv(cfg)

	// 验证配置
	if err := cm.ValidateConfig(cfg); err != nil {
		logging.DefaultLogger.Error("配置验证失败: %v", err)
		return
	}

	// 保存修改时间
	info, err := os.Stat(cm.configPath)
	if err != nil {
		logging.DefaultLogger.Warn("无法获取配置文件修改时间: %s, 错误: %v", cm.configPath, err)
		cm.lastModified = time.Now()
	} else {
		cm.lastModified = info.ModTime()
	}

	// 更新配置
	cm.config = cfg

	// 通知所有配置变化处理函数
	for _, handler := range cm.handlers {
		go func(handler ConfigChangeHandler, config *Config) {
			defer func() {
				if r := recover(); r != nil {
					logging.DefaultLogger.Error("配置变化处理函数panic: %v", r)
				}
			}()
			handler(config)
		}(handler, cfg) // 异步调用，避免阻塞
	}

	logging.DefaultLogger.Info("配置文件已重新加载: %s", cm.configPath)
}

// defaultConfig 创建默认配置
func defaultConfig() *Config {
	// 默认站点配置
	defaultSite := SiteConfig{
		ID:      "default", // 默认站点ID
		Name:    "默认站点",
		Domains: []string{"localhost"}, // 支持多个域名
		Port:    8084,                  // 默认端口
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
			RulesPath: "./rules",
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
			Enabled:         true,
			PoolSize:        5,
			MinPoolSize:     2,
			MaxPoolSize:     20,
			Timeout:         30,
			CacheTTL:        3600,
			IdleTimeout:     300,
			DynamicScaling:  true,
			ScalingFactor:   0.5,
			ScalingInterval: 60,
			// 默认包含市面上常见的爬虫协议头
			CrawlerHeaders: []string{
				"Googlebot",
				"Bingbot",
				"Slurp",
				"DuckDuckBot",
				"Baiduspider",
				"Sogou spider",
				"YandexBot",
				"Exabot",
				"FacebookBot",
				"Twitterbot",
				"LinkedInBot",
				"WhatsAppBot",
				"TelegramBot",
				"DiscordBot",
				"PinterestBot",
				"InstagramBot",
				"Google-InspectionTool",
				"Google-Site-Verification",
				"AhrefsBot",
				"SEMrushBot",
				"Majestic",
				"Yahoo! Slurp",
				"Applebot",
				"Mediapartners-Google",
				"AdsBot-Google",
				"Feedfetcher-Google",
				"Googlebot-Image",
				"Googlebot-News",
				"Googlebot-Video",
				"Googlebot-Extended",
				"bingbot/2.0",
				"msnbot",
				"MSNbot-Media",
				"bingbot/1.0",
				"msnbot-media/1.1",
				"adidxbot",
				"BingPreview",
				"BingSiteAuth",
				"BingLocalSearchBot",
				"Baiduspider-image",
				"Baiduspider-video",
				"Baiduspider-mobile",
				"Baiduspider-news",
				"Baiduspider-favo",
				"Baiduspider-cpro",
				"Baiduspider-ads",
				"Sogou web spider",
				"Sogou inst spider",
				"Sogou spider2",
				"Sogou blog",
				"Sogou News Spider",
				"Sogou Orion spider",
				"Sogou video spider",
				"Sogou image spider",
				"YandexBot/3.0",
				"YandexMobileBot",
				"YandexImages",
				"YandexVideo",
				"YandexMedia",
				"YandexBlogs",
				"YandexNews",
				"YandexCatalog",
			},
			UseDefaultHeaders: false, // 不再使用默认爬虫协议头，直接使用配置的CrawlerHeaders
			Preheat: PreheatConfig{
				Enabled:         false,
				SitemapURL:      "",
				Schedule:        "0 0 * * *",
				Concurrency:     5,
				DefaultPriority: 0,
				MaxDepth:        3, // 默认爬取深度为3
			},
			Push: PushConfig{
				Enabled:         false,
				BaiduAPI:        "http://data.zz.baidu.com/urls",
				BaiduToken:      "",
				BingAPI:         "https://ssl.bing.com/webmaster/api.svc/json/SubmitUrl",
				BingToken:       "",
				BaiduDailyLimit: 1000,
				BingDailyLimit:  1000,
				PushDomain:      "",
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
			Address:      "0.0.0.0",
			APIPort:      9598,
			ConsolePort:  9597,
			PublicAPIURL: "http://localhost:9598",
		},
		Dirs: DirsConfig{
			DataDir:        "./data",   // 数据目录
			StaticDir:      "./static", // 静态文件目录
			CertsDir:       "./certs",  // 证书目录
			AdminStaticDir: "./web",    // 管理控制台静态文件目录
		},
		Cache: CacheConfig{
			Type:          "memory",
			RedisURL:      "localhost:6379",
			RedisPassword: "",
			RedisDB:       0,
			MemorySize:    1000,
		},
		Storage: StorageConfig{
			Type: "redis",
		},
		Monitoring: MonitoringConfig{
			Enabled:           true,
			PrometheusAddress: ":9090",
		},
		App: AppConfig{
			Version:     "1.0.1",
			OfficialURL: "https://prerender.websitetool.cn",
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
	cfg.Server.PublicAPIURL = getEnv("API_PUBLIC_URL", cfg.Server.PublicAPIURL)

	// 目录配置
	cfg.Dirs.DataDir = getEnv("DIRS_DATA_DIR", cfg.Dirs.DataDir)
	cfg.Dirs.StaticDir = getEnv("DIRS_STATIC_DIR", cfg.Dirs.StaticDir)
	cfg.Dirs.CertsDir = getEnv("DIRS_CERTS_DIR", cfg.Dirs.CertsDir)
	cfg.Dirs.AdminStaticDir = getEnv("DIRS_ADMIN_STATIC_DIR", cfg.Dirs.AdminStaticDir)

	// 缓存配置
	cfg.Cache.Type = getEnv("CACHE_TYPE", cfg.Cache.Type)

	// 支持细粒度的Redis环境变量
	redisHost := getEnv("REDIS_HOST", "")
	if redisHost != "" {
		redisPort := getEnv("REDIS_PORT", "6379")
		redisPassword := getEnv("REDIS_PASSWORD", "")
		redisDB := getEnv("REDIS_DB", "0")

		authPart := ""
		if redisPassword != "" {
			authPart = fmt.Sprintf(":%s@", redisPassword)
		}

		cfg.Cache.RedisURL = fmt.Sprintf("redis://%s%s:%s/%s", authPart, redisHost, redisPort, redisDB)
	} else {
		// 回退到 CACHE_REDIS_URL
		cfg.Cache.RedisURL = getEnv("CACHE_REDIS_URL", cfg.Cache.RedisURL)
	}

	cfg.Cache.MemorySize = getEnvAsInt("CACHE_MEMORY_SIZE", cfg.Cache.MemorySize)

	// 存储配置
	cfg.Storage.Type = getEnv("STORAGE_TYPE", cfg.Storage.Type)

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

// replaceEnvVars 替换字符串中的环境变量占位符
// 支持 `${VAR}` 和 `${VAR:-default}` 格式
func replaceEnvVars(s string) string {
	// 正则表达式匹配环境变量占位符
	pattern := regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]+))?\}`)

	return pattern.ReplaceAllStringFunc(s, func(m string) string {
		// 提取环境变量名和默认值
		matches := pattern.FindStringSubmatch(m)
		if len(matches) < 2 {
			return m
		}

		key := matches[1]
		defaultValue := ""
		if len(matches) > 2 {
			defaultValue = matches[2]
		}

		// 查找环境变量
		if value, exists := os.LookupEnv(key); exists {
			return value
		}

		// 返回默认值
		return defaultValue
	})
}

// replaceConfigEnvVars 递归替换配置中的环境变量占位符
func replaceConfigEnvVars(cfg *Config) {
	// 替换服务器配置中的环境变量
	cfg.Server.PublicAPIURL = replaceEnvVars(cfg.Server.PublicAPIURL)

	// 替换目录配置中的环境变量
	cfg.Dirs.DataDir = replaceEnvVars(cfg.Dirs.DataDir)
	cfg.Dirs.StaticDir = replaceEnvVars(cfg.Dirs.StaticDir)
	cfg.Dirs.CertsDir = replaceEnvVars(cfg.Dirs.CertsDir)
	cfg.Dirs.AdminStaticDir = replaceEnvVars(cfg.Dirs.AdminStaticDir)

	// 替换缓存配置中的环境变量
	cfg.Cache.RedisURL = replaceEnvVars(cfg.Cache.RedisURL)
	cfg.Cache.RedisPassword = replaceEnvVars(cfg.Cache.RedisPassword)

	// 替换应用配置中的环境变量
	cfg.App.Version = replaceEnvVars(cfg.App.Version)
	cfg.App.OfficialURL = replaceEnvVars(cfg.App.OfficialURL)

	// 替换站点配置中的环境变量
	for i := range cfg.Sites {
		site := &cfg.Sites[i]
		site.Name = replaceEnvVars(site.Name)

		// 替换域名列表中的环境变量
		for j := range site.Domains {
			site.Domains[j] = replaceEnvVars(site.Domains[j])
		}

		// 替换代理配置中的环境变量
		site.Proxy.TargetURL = replaceEnvVars(site.Proxy.TargetURL)

		// 替换重定向配置中的环境变量
		site.Redirect.TargetURL = replaceEnvVars(site.Redirect.TargetURL)

		// 替换防火墙配置中的环境变量
		site.Firewall.RulesPath = replaceEnvVars(site.Firewall.RulesPath)
		site.Firewall.ActionConfig.BlockMessage = replaceEnvVars(site.Firewall.ActionConfig.BlockMessage)

		// 替换渲染预热配置中的环境变量
		site.Prerender.Preheat.SitemapURL = replaceEnvVars(site.Prerender.Preheat.SitemapURL)
		site.Prerender.Push.BaiduAPI = replaceEnvVars(site.Prerender.Push.BaiduAPI)
		site.Prerender.Push.BaiduToken = replaceEnvVars(site.Prerender.Push.BaiduToken)
		site.Prerender.Push.BingAPI = replaceEnvVars(site.Prerender.Push.BingAPI)
		site.Prerender.Push.BingToken = replaceEnvVars(site.Prerender.Push.BingToken)
		site.Prerender.Push.PushDomain = replaceEnvVars(site.Prerender.Push.PushDomain)

		// 替换爬虫协议头中的环境变量
		for j := range site.Prerender.CrawlerHeaders {
			site.Prerender.CrawlerHeaders[j] = replaceEnvVars(site.Prerender.CrawlerHeaders[j])
		}

		// 替换路由规则中的环境变量
		for j := range site.Routing.Rules {
			rule := &site.Routing.Rules[j]
			rule.Pattern = replaceEnvVars(rule.Pattern)
			rule.Action = replaceEnvVars(rule.Action)
		}
	}
}
