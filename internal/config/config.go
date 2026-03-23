package config

import (
	"fmt"
	"os"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/redis"
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
	Enabled           bool           `yaml:"enabled"`
	PrometheusAddress string         `yaml:"prometheus_address"`
	Alerting          AlertingConfig `yaml:"alerting"`
	// 监控数据持久化配置
	MetricsPersistence MetricsPersistenceConfig `yaml:"metrics_persistence"`
}

// MetricsPersistenceConfig 监控数据持久化配置
type MetricsPersistenceConfig struct {
	Enabled           bool `yaml:"enabled"`
	Interval          int  `yaml:"interval"`           // 持久化间隔（秒），默认 300 秒
	RetentionHours    int  `yaml:"retention_hours"`    // 数据保留时间（小时），默认 24 小时
	AggregateEnabled  bool `yaml:"aggregate_enabled"`  // 是否启用数据聚合
	AggregateInterval int  `yaml:"aggregate_interval"` // 聚合间隔（秒），默认 3600 秒（1 小时）
}

// AlertingConfig 告警配置
type AlertingConfig struct {
	Enabled       bool                `yaml:"enabled"`
	RulesPath     string              `yaml:"rules_path"`
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
	Enabled  bool     `yaml:"enabled"`
	SMTPHost string   `yaml:"smtp_host"`
	SMTPPort int      `yaml:"smtp_port"`
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
	From     string   `yaml:"from"`
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

// GetConfig 获取当前配置
func (cm *ConfigManager) GetConfig() *Config {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.config
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
