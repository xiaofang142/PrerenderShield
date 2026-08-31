# Prerender Shield 数据模型文档

> 基于代码实际结构的完整数据模型，最后更新: 2026-06-13

---

## 一、核心配置模型

### 1.1 Config (全局配置)

```go
type Config struct {
    Server     ServerConfig     `yaml:"server"`
    Dirs       DirsConfig       `yaml:"dirs"`
    Cache      CacheConfig      `yaml:"cache"`
    Storage    StorageConfig    `yaml:"storage"`
    Monitoring MonitoringConfig `yaml:"monitoring"`
    App        AppConfig        `yaml:"app"`
    SSL        SSLConfig        `yaml:"ssl"`
    Sites      []SiteConfig     `yaml:"sites"`
}
```

### 1.2 ServerConfig (服务器配置)

```go
type ServerConfig struct {
    Address      string `yaml:"address"`       // 监听地址，默认 "0.0.0.0"
    APIPort      int    `yaml:"api_port"`      // API端口，默认 9598
    ConsolePort  int    `yaml:"console_port"`  // 控制台端口，默认 9597
    PublicAPIURL string `yaml:"public_api_url"` // 公开API地址
}
```

### 1.3 DirsConfig (目录配置)

```go
type DirsConfig struct {
    DataDir        string `yaml:"data_dir"`         // 数据目录
    StaticDir      string `yaml:"static_dir"`       // 静态文件目录
    CertsDir       string `yaml:"certs_dir"`        // 证书目录
    AdminStaticDir string `yaml:"admin_static_dir"` // 管理控制台静态文件目录
}
```

### 1.4 CacheConfig (缓存配置)

```go
type CacheConfig struct {
    Type          string          `yaml:"type"`           // "memory" 或 "redis"
    RedisURL      string          `yaml:"redis_url"`      // Redis连接地址
    RedisPassword string          `yaml:"redis_password"`  // Redis密码
    RedisDB       int             `yaml:"redis_db"`       // Redis数据库编号
    MemorySize    int             `yaml:"memory_size"`    // 内存缓存大小(MB)
    RedisPool     RedisPoolConfig `yaml:"redis_pool"`     // Redis连接池配置
}

type RedisPoolConfig struct {
    MaxActive   int           `yaml:"max_active"`    // 最大连接数，默认20
    MaxIdle     int           `yaml:"max_idle"`      // 最大空闲连接，默认10
    IdleTimeout time.Duration `yaml:"idle_timeout"`  // 空闲超时，默认5分钟
    PoolTimeout time.Duration `yaml:"pool_timeout"`  // 获取连接超时，默认30秒
}
```

### 1.5 StorageConfig (存储配置)

```go
type StorageConfig struct {
    Type string `yaml:"type"` // "redis"
}
```

### 1.6 MonitoringConfig (监控配置)

```go
type MonitoringConfig struct {
    Enabled           bool                     `yaml:"enabled"`
    PrometheusAddress string                   `yaml:"prometheus_address"` // 默认 ":9090"
    Alerting          AlertingConfig           `yaml:"alerting"`
    MetricsPersistence MetricsPersistenceConfig `yaml:"metrics_persistence"`
}

type MetricsPersistenceConfig struct {
    Enabled           bool `yaml:"enabled"`
    Interval          int  `yaml:"interval"`            // 持久化间隔(秒)，默认300
    RetentionHours    int  `yaml:"retention_hours"`     // 保留时间(小时)，默认24
    AggregateEnabled  bool `yaml:"aggregate_enabled"`   // 是否启用聚合
    AggregateInterval int  `yaml:"aggregate_interval"`  // 聚合间隔(秒)，默认3600
}

type AlertingConfig struct {
    Enabled       bool                `yaml:"enabled"`
    RulesPath     string              `yaml:"rules_path"`
    Notifications NotificationsConfig `yaml:"notifications"`
}

type NotificationsConfig struct {
    Webhook WebhookNotificationConfig `yaml:"webhook"`
    Email   EmailNotificationConfig   `yaml:"email"`
}

type WebhookNotificationConfig struct {
    Enabled bool   `yaml:"enabled"`
    URL     string `yaml:"url"`
    Secret  string `yaml:"secret"`
}

type EmailNotificationConfig struct {
    Enabled  bool     `yaml:"enabled"`
    SMTPHost string   `yaml:"smtp_host"`
    SMTPPort int      `yaml:"smtp_port"`
    Username string   `yaml:"username"`
    Password string   `yaml:"password"`
    From     string   `yaml:"from"`
    To       []string `yaml:"to"`
}
```

### 1.7 AppConfig (应用配置)

```go
type AppConfig struct {
    Version     string `yaml:"version"`      // "1.0.1"
    OfficialURL string `yaml:"official_url"` // "https://prerender.websitetool.cn"
}
```

### 1.8 SSLConfig (SSL配置)

```go
type SSLConfig struct {
    Enabled         bool          `yaml:"enabled"`
    AutoRenew       bool          `yaml:"auto_renew"`
    Email           string        `yaml:"email"`              // ACME账户邮箱
    Production      bool          `yaml:"production"`         // 是否生产环境
    HTTPPort        int           `yaml:"http_port"`          // HTTP-01挑战端口
    CheckInterval   time.Duration `yaml:"check_interval"`     // 检查间隔
    RenewBeforeDays int           `yaml:"renew_before_days"`  // 提前续签天数
    MaxRetries      int           `yaml:"max_retries"`        // 最大重试次数
    RetryDelay      time.Duration `yaml:"retry_delay"`        // 重试间隔
    WebhookURL      string        `yaml:"webhook_url"`        // 通知Webhook
    DNS             DNSConfig     `yaml:"dns"`                // DNS-01挑战配置
}

type DNSConfig struct {
    Provider    string            `yaml:"provider"`     // cloudflare/aliyun/tencentcloud
    Credentials map[string]string `yaml:"credentials"`  // API凭证
}
```

---

## 二、站点模型

### 2.1 SiteConfig (站点配置)

```go
type SiteConfig struct {
    ID      string   `yaml:"id" json:"id"`           // 站点唯一ID
    Name    string   `yaml:"name" json:"name"`        // 站点名称
    Domains []string `yaml:"domains" json:"domains"`  // 绑定域名列表
    Port    int      `yaml:"port" json:"port"`        // 监听端口
    Mode    string   `yaml:"mode" json:"mode"`        // proxy/static/redirect
    
    Proxy              ProxyConfig         `yaml:"proxy" json:"proxy"`
    Redirect           RedirectConfig      `yaml:"redirect" json:"redirect"`
    Firewall           FirewallConfig      `yaml:"firewall" json:"firewall"`
    Prerender          PrerenderConfig     `yaml:"prerender" json:"prerender"`
    Routing            RoutingConfig       `yaml:"routing" json:"routing"`
    FileIntegrityConfig FileIntegrityConfig `yaml:"file_integrity" json:"file_integrity"`
}

type ProxyConfig struct {
    TargetURL string `yaml:"target_url" json:"target_url"`
}

type RedirectConfig struct {
    StatusCode int    `yaml:"status_code" json:"status_code"` // 301/302
    TargetURL  string `yaml:"target_url" json:"target_url"`
}

type FileIntegrityConfig struct {
    Enabled       bool   `yaml:"enabled" json:"enabled"`
    CheckInterval int    `yaml:"check_interval" json:"check_interval"` // 秒
    HashAlgorithm string `yaml:"hash_algorithm" json:"hash_algorithm"` // md5/sha256
}
```

### 2.2 FirewallConfig (防火墙配置)

```go
type FirewallConfig struct {
    Enabled         bool            `yaml:"enabled" json:"enabled"`
    RulesPath       string          `yaml:"rules_path" json:"rules_path"`
    ActionConfig    ActionConfig    `yaml:"action" json:"action"`
    GeoIPConfig     GeoIPConfig     `yaml:"geoip" json:"geoip"`
    RateLimitConfig RateLimitConfig `yaml:"rate_limit" json:"rate_limit"`
    Blacklist       []string        `yaml:"blacklist" json:"blacklist"`
    Whitelist       []string        `yaml:"whitelist" json:"whitelist"`
}

type ActionConfig struct {
    DefaultAction string `yaml:"default_action" json:"default_action"` // block/allow
    BlockMessage  string `yaml:"block_message" json:"block_message"`
}

type GeoIPConfig struct {
    Enabled   bool     `yaml:"enabled" json:"enabled"`
    AllowList []string `yaml:"allow_list" json:"allow_list"` // 国家代码列表
    BlockList []string `yaml:"block_list" json:"block_list"` // 国家代码列表
}

type RateLimitConfig struct {
    Enabled  bool `yaml:"enabled" json:"enabled"`
    Requests int  `yaml:"requests" json:"requests"` // 窗口内允许请求数
    Window   int  `yaml:"window" json:"window"`     // 时间窗口(秒)
    BanTime  int  `yaml:"ban_time" json:"ban_time"` // 封禁时间(秒)
}
```

### 2.3 PrerenderConfig (预渲染配置)

```go
type PrerenderConfig struct {
    Enabled           bool          `yaml:"enabled" json:"enabled"`
    PoolSize          int           `yaml:"pool_size" json:"pool_size"`           // 默认5
    MinPoolSize       int           `yaml:"min_pool_size" json:"min_pool_size"`   // 默认2
    MaxPoolSize       int           `yaml:"max_pool_size" json:"max_pool_size"`   // 默认20
    Timeout           int           `yaml:"timeout" json:"timeout"`               // 默认30秒
    CacheTTL          int           `yaml:"cache_ttl" json:"cache_ttl"`           // 默认3600秒
    IdleTimeout       int           `yaml:"idle_timeout" json:"idle_timeout"`     // 默认300秒
    DynamicScaling    bool          `yaml:"dynamic_scaling" json:"dynamic_scaling"`
    ScalingFactor     float64       `yaml:"scaling_factor" json:"scaling_factor"`   // 默认0.5
    ScalingInterval   int           `yaml:"scaling_interval" json:"scaling_interval"` // 默认60秒
    Preheat           PreheatConfig `yaml:"preheat" json:"preheat"`
    Push              PushConfig    `yaml:"push" json:"push"`
    CrawlerHeaders    []string      `yaml:"crawler_headers" json:"crawler_headers"`
    UseDefaultHeaders bool          `yaml:"use_default_headers" json:"use_default_headers"`
}

type PreheatConfig struct {
    Enabled         bool   `yaml:"enabled" json:"enabled"`
    SitemapURL      string `yaml:"sitemap_url" json:"sitemap_url"`
    Schedule        string `yaml:"schedule" json:"schedule"`             // Cron表达式
    Concurrency     int    `yaml:"concurrency" json:"concurrency"`       // 默认5
    DefaultPriority int    `yaml:"default_priority" json:"default_priority"`
    MaxDepth        int    `yaml:"max_depth" json:"max_depth"`           // 默认3
}

type PushConfig struct {
    Enabled         bool   `yaml:"enabled" json:"enabled"`
    BaiduAPI        string `yaml:"baidu_api" json:"baidu_api"`
    BaiduToken      string `yaml:"baidu_token" json:"baidu_token"`
    BingAPI         string `yaml:"bing_api" json:"bing_api"`
    BingToken       string `yaml:"bing_token" json:"bing_token"`
    BaiduDailyLimit int    `yaml:"baidu_daily_limit" json:"baidu_daily_limit"` // 默认1000
    BingDailyLimit  int    `yaml:"bing_daily_limit" json:"bing_daily_limit"`   // 默认1000
    PushDomain      string `yaml:"push_domain" json:"push_domain"`
}
```

### 2.4 RoutingConfig (路由配置)

```go
type RoutingConfig struct {
    Rules []RouteRule `yaml:"rules" json:"rules"`
}

type RouteRule struct {
    ID       string `yaml:"id" json:"id"`
    Pattern  string `yaml:"pattern" json:"pattern"`   // 正则表达式
    Action   string `yaml:"action" json:"action"`     // 路由动作
    Priority int    `yaml:"priority" json:"priority"`  // 优先级
}
```

---

## 三、认证模型

### 3.1 User (管理员)

```go
type User struct {
    ID       string `json:"id"`       // UUID
    Username string `json:"username"`
    Password string `json:"password"` // bcrypt加密
}

// UserManager — 单管理员模式
// 系统仅维护一个管理员账户，通过 IsFirstRun() 检测首次运行
// CreateUser() 仅允许在首次运行时创建
// ChangePassword() 支持管理员自行修改密码
```

### 3.2 JWT相关

```go
type JWTConfig struct {
    SecretKey  string        `yaml:"secret_key"`
    ExpireTime time.Duration `yaml:"expire_time"` // 默认24小时
}

type Claims struct {
    UserID    string `json:"user_id"`
    Username  string `json:"username"`
    SessionID string `json:"session_id"`
    jwt.RegisteredClaims  // exp, iat, nbf, iss, sub, jti
}
```

---

## 四、防火墙模型

### 4.1 引擎核心

```go
type Engine struct {
    SiteName       string
    owaspDetectors map[string]OWASPDetector  // OWASP检测器集合
    coreDetectors  []CoreDetector            // 核心检测器列表
    actionHandler  ActionHandler             // 动作处理器
    ruleManager    *RuleManager              // 规则管理器
    redisClient    *redis.Client             // Redis客户端
    cacheTTL       time.Duration             // 请求缓存TTL
    failStrategy   FailStrategy              // 失败策略 (FailOpen/FailClosed)
}

type FailStrategy int
const (
    FailOpen   FailStrategy = iota  // 失败时允许
    FailClosed                       // 失败时拒绝
)
```

### 4.2 检测器接口

```go
type OWASPDetector interface {
    Detect(req *http.Request) ([]types.Threat, error)
    Name() string
}

type CoreDetector interface {
    Detect(req *http.Request) ([]types.Threat, error)
    Name() string
}

type ActionHandler interface {
    Handle(w http.ResponseWriter, req *http.Request, result *CheckResult) bool
}
```

### 4.3 威胁与规则

```go
// types/threat.go
type Threat struct {
    Type        string    // 威胁类型
    Severity    string    // 严重程度: critical/high/medium/low
    Description string    // 描述
    RuleID      string    // 触发的规则ID
    Details     map[string]interface{} // 详细信息
}

// types/rule.go
type Rule struct {
    ID          string    // 规则ID
    Name        string    // 规则名称
    Category    string    // 分类: injection/xss/csrf/...
    Pattern     string    // 匹配模式(正则)
    Action      string    // 动作: block/allow/challenge/log
    Priority    int       // 优先级
    Enabled     bool      // 是否启用
    Description string    // 描述
}
```

---

## 五、渲染引擎模型

### 5.1 渲染核心

```go
type Engine interface {
    Render(url string, timeout time.Duration) ([]byte, error)
    CreatePreheatTask(siteID string, urls []string) (string, error)
    GetPreheatTaskStatus(taskID string) (map[string]interface{}, error)
    ListPreheatTasks(siteID string) ([]map[string]interface{}, error)
    CancelPreheatTask(taskID string) error
}

type RenderOptions struct {
    Timeout        time.Duration
    WaitUntil      string            // "load"/"domcontentloaded"/"networkidle"
    Headers        map[string]string
    Cookies        []Cookie
    Proxy          string
    BlockResources bool
}

type Cookie struct {
    Name     string
    Value    string
    Domain   string
    Path     string
    Expires  time.Time
    Secure   bool
    HttpOnly bool
    SameSite string
}

type RenderResult struct {
    HTML       string
    Success    bool
    Error      string
    RenderTime float64  // 渲染耗时(秒)
    URL        string
}

type RenderWithCacheResult struct {
    Result   RenderResult
    HitCache bool
    CacheTTL int
}
```

---

## 六、数据模型 (models/)

### 6.1 Site (站点)

```go
type Site struct {
    ID        string    `json:"id"`
    Domain    string    `json:"domain"`
    Name      string    `json:"name"`
    Enabled   bool      `json:"enabled"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    WafConfig  *WafConfig  `json:"waf_config,omitempty"`
    AccessLogs []AccessLog `json:"-"`
}

type SiteConfig struct {
    Domain    string `json:"domain"`
    Mode      string `json:"mode"`       // proxy/static/redirect
    TargetURL string `json:"target_url,omitempty"`
    Port      int    `json:"port,omitempty"`
}
```

### 6.2 WAF配置

```go
type WafConfig struct {
    ID              string    `json:"id"`
    SiteID          string    `json:"site_id"`
    RateLimitCount  int       `json:"rate_limit_count"`
    RateLimitWindow int       `json:"rate_limit_window"`  // 分钟
    CustomBlockPage string    `json:"custom_block_page"`
    Enabled         bool      `json:"enabled"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
    BlockedCountries []BlockedCountry `json:"blocked_countries,omitempty"`
    IPWhitelist      []IPWhitelist    `json:"ip_whitelist,omitempty"`
    IPBlacklist      []IPBlacklist    `json:"ip_blacklist,omitempty"`
}

type BlockedCountry struct {
    ID          string `json:"id"`
    WafConfigID string `json:"waf_config_id"`
    CountryCode string `json:"country_code"`
}

type IPWhitelist struct {
    ID          string `json:"id"`
    WafConfigID string `json:"waf_config_id"`
    IPAddress   string `json:"ip_address"`
}

type IPBlacklist struct {
    ID          string    `json:"id"`
    WafConfigID string    `json:"waf_config_id"`
    IPAddress   string    `json:"ip_address"`
    Reason      string    `json:"reason"`
    CreatedAt   time.Time `json:"created_at"`
}
```

### 6.3 WAF规则

```go
type WAFRule struct {
    ID       string `json:"id"`
    Name     string `json:"name"`
    Pattern  string `json:"pattern"`  // 正则表达式
    Action   string `json:"action"`   // block/allow/challenge/log
    Priority int    `json:"priority"`
    Enabled  bool   `json:"enabled"`
}

const (
    WAFActionBlock     = "block"
    WAFActionAllow     = "allow"
    WAFActionChallenge = "challenge"
    WAFActionLog       = "log"
)
```

### 6.4 访问日志

```go
type AccessLog struct {
    ID          string    `json:"id"`
    SiteID      string    `json:"site_id"`
    RequestID   string    `json:"request_id"`
    IPAddress   string    `json:"ip_address"`
    Country     string    `json:"country"`
    City        string    `json:"city"`
    Method      string    `json:"method"`
    UserAgent   string    `json:"user_agent"`
    RequestPath string    `json:"request_path"`
    StatusCode  int       `json:"status_code"`
    Action      string    `json:"action"`    // allow/block/captcha
    RuleID      string    `json:"rule_id"`
    Reason      string    `json:"reason"`
    IsCleaned   bool      `json:"is_cleaned"`
    CreatedAt   time.Time `json:"created_at"`
}
```

---

## 七、错误模型

```go
type AppError struct {
    Code     int         `json:"code"`              // HTTP状态码
    Message  string      `json:"message"`            // 错误消息
    Details  interface{} `json:"details,omitempty"`  // 详细信息
    Internal error       `json:"-"`                  // 内部错误
    Stack    string      `json:"-"`                  // 调用栈
}

type Response struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
    Details interface{} `json:"details,omitempty"`
}

// 错误码常量
const (
    CodeSuccess           = 200
    CodeBadRequest        = 400
    CodeUnauthorized      = 401
    CodeForbidden         = 403
    CodeNotFound          = 404
    CodeRateLimit         = 429
    CodeValidation        = 422
    CodeInternalError     = 500
    CodeServiceUnavailable = 503
)
```

---

## 八、Redis Key设计

| Key Pattern | 类型 | 说明 | TTL |
|------------|------|------|-----|
| `user:{id}` | Hash | 用户数据 {id, username, password} | 永久 |
| `username:{name}` | String | 用户名→用户ID映射 | 永久 |
| `users:all` | Set | 所有用户ID集合 | 永久 |
| `session:{id}` | Hash | JWT会话 {user_id, username, session_id, created_at} | 24h |
| `prerender:config:sites` | String | 站点配置YAML | 永久 |
| `prerender:cache:{hash}` | String | 渲染缓存HTML | CacheTTL |
| `prerender:firewall:rules` | String | 防火墙规则 | 永久 |
| `system:backup:{timestamp}` | String | 配置备份 | 永久 |
| `site:{id}:stats` | Hash | 站点统计 | 永久 |
| `site:{id}_prerender` | Hash | 预渲染配置 | 永久 |
| `site:{id}:urls` | Set | 预热URL集合 | 永久 |
| `push:{site}:daily:{date}` | String | 推送日计数 | 24h |

---

## 九、默认值常量

```go
// 端口
DefaultServerAddress = "0.0.0.0"
DefaultAPIPort       = 9598
DefaultConsolePort   = 9597

// 缓存
DefaultCacheType       = "memory"
DefaultCacheMemorySize = 1000
DefaultCacheTTL        = 3600s
DefaultRedisDB         = 0

// 防火墙
DefaultFirewallAction       = "block"
DefaultFirewallBlockMessage = "Request blocked by firewall"
DefaultRateLimitRequests    = 100
DefaultRateLimitWindow      = 60s
DefaultRateLimitBanTime     = 3600s

// 预热
DefaultPreheatConcurrency     = 5
DefaultPreheatMaxDepth        = 3
DefaultPreheatScalingInterval = 60s

// 渲染
DefaultPrerenderTimeout = 30s
DefaultIdleTimeout      = 300s

// 文件完整性
DefaultFileIntegrityCheckInterval = 300s
DefaultHashAlgorithm             = "sha256"

// 配置
DefaultConfigCheckInterval = 5s
DefaultRuleUpdateInterval  = 24h
DefaultCacheCleanInterval  = 5min
DefaultCacheBatchSize      = 100
```

---

## 十、SEO数据模型

### 10.1 Meta标签优化

```go
type MetaTagsConfig struct {
    TitleMinLength       int      // 标题最小长度，默认30
    TitleMaxLength       int      // 标题最大长度，默认60
    DescriptionMinLength int      // 描述最小长度，默认120
    DescriptionMaxLength int      // 描述最大长度，默认160
    MaxKeywords          int      // 最大关键词数，默认10
    MinKeywordLength     int      // 最小关键词长度，默认3
    AutoGenerateKeywords bool     // 自动生成关键词
    EnableOpenGraph      bool     // 启用Open Graph
    EnableTwitterCard    bool     // 启用Twitter Card
    TwitterCardType      string   // Twitter Card类型，默认"summary_large_image"
    RequiredMetaTags     []string // 必需标签: title/description/viewport/charset
}

type MetaTagsResult struct {
    Title           *TitleAnalysis
    Description     *DescriptionAnalysis
    Keywords        []string
    MetaTags        map[string]string  // 生成的meta标签
    OpenGraph       map[string]string  // og:title/og:description/og:type/og:url
    TwitterCard     map[string]string  // twitter:card/twitter:title/twitter:description
    CanonicalURL    string
    MissingTags     []string           // 缺失的标签列表
    Recommendations []string           // 优化建议
}
```

### 10.2 结构化数据 (Schema.org)

```go
type StructuredDataConfig struct {
    EnableJSONLD       bool     // 启用JSON-LD，默认true
    EnableMicrodata    bool     // 启用微数据，默认false
    EnableRDFa         bool     // 启用RDFa，默认false
    EnabledTypes       []string // Article/Product/Organization/LocalBusiness/FAQPage/BreadcrumbList
    AutoDetectType     bool     // 自动检测页面类型
    IncludeLogo        bool
    IncludeSocialLinks bool
    DefaultLanguage    string   // 默认"zh-CN"
}

// 支持的Schema类型:
// - ArticleSchema: headline, description, image, datePublished, author, publisher
// - ProductSchema: name, description, image, offers(price/currency/availability), brand, sku
// - OrganizationSchema: name, url, logo, sameAs, contactPoint
// - LocalBusinessSchema: name, telephone, address, geo, openingHours, priceRange
// - FAQSchema: mainEntity[] (Question + Answer)
// - BreadcrumbListSchema: itemListElement[] (position + item)
```

### 10.3 AI爬虫引擎优化 (AEO)

```go
type AEOConfig struct {
    EnableAECrawlerDetection bool     // AI爬虫检测
    EnableAnswerExtraction   bool     // 纯净内容提取
    EnableStructuredData     bool     // 结构化数据优先
    SupportedAICrawlers      []string // gptbot/claudebot/perplexitybot/google-extended/cohere-ai/facebookbot/applebot/bytespider
    AnswerFormats            []string // summary/bullet
}

type AICrawlerInfo struct {
    Name     string  // GPTBot/ClaudeBot/PerplexityBot...
    Company  string  // OpenAI/Anthropic/Perplexity AI...
    BotToken string  // gptbot/claudebot/perplexitybot...
    Purpose  string  // training/search/indexing
}
```

---

## 十一、任务与调度模型

### 11.1 任务队列

```go
type TaskType string
const (
    TaskTypePreheat TaskType = "preheat"  // 预热任务
    TaskTypeSSL     TaskType = "ssl"      // SSL证书任务
    TaskTypeCleanup TaskType = "cleanup"  // 清理任务
    TaskTypeMonitor TaskType = "monitor"  // 监控任务
)

type TaskStatus string
const (
    TaskStatusPending   TaskStatus = "pending"
    TaskStatusRunning   TaskStatus = "running"
    TaskStatusCompleted TaskStatus = "completed"
    TaskStatusFailed    TaskStatus = "failed"
    TaskStatusCancelled TaskStatus = "cancelled"
)

type Task interface {
    ID() string
    Type() TaskType
    Status() TaskStatus
    Priority() int
    Execute() error
    Cancel() error
    Retry() error
}
```

### 11.2 调度器

```go
type Scheduler struct {
    cron          *cron.Cron           // robfig/cron，秒级精度
    engineManager *prerender.EngineManager
    pushManager   *push.PushManager
    redisClient   *redis.Client
    tasks         map[string]cron.EntryID  // 站点→任务映射
}
```

---

## 十二、审计模型

```go
type Action string
const (
    ActionLogin        Action = "login"
    ActionLogout       Action = "logout"
    ActionConfigUpdate Action = "config.update"
    ActionSiteCreate   Action = "site.create"
    ActionSiteUpdate   Action = "site.update"
    ActionSiteDelete   Action = "site.delete"
    ActionCertRequest  Action = "cert.request"
    ActionCertRenew    Action = "cert.renew"
    ActionCertDelete   Action = "cert.delete"
    ActionPreheat      Action = "preheat.trigger"
    ActionWAFRule      Action = "waf.rule.update"
    ActionBlacklist    Action = "blacklist.update"
    ActionWhitelist    Action = "whitelist.update"
    ActionUserManage   Action = "user.manage"
    ActionSystemConfig Action = "system.config"
)

type Severity string
const (
    SeverityInfo     Severity = "info"
    SeverityWarning  Severity = "warning"
    SeverityCritical Severity = "critical"
)

type Entry struct {
    ID        string            // UUID
    UserID    string
    Action    Action
    Resource  string
    Detail    string
    Severity  Severity
    ClientIP  string
    Status    string
    Metadata  map[string]string
    Timestamp time.Time
}
```

---

## 十三、加密模型

```go
type Encryptor struct {
    key []byte  // AES密钥 (16/24/32字节)
}

// AES-256-GCM加密
func (e *Encryptor) Encrypt(plaintext string) (string, error)
func (e *Encryptor) Decrypt(ciphertext string) (string, error)
```

---

## 十四、依赖注入容器

```go
type Container struct {
    Config          *config.Config
    Redis           *redis.Client
    UserManager     *auth.UserManager
    JWTManager      *auth.JWTManager
    FirewallMgr     *firewall.EngineManager
    CacheMgr        cache.Manager
    PrerenderMgr    *prerender.EngineManager
    CrawlerLogMgr   *logging.CrawlerLogManager
    VisitLogMgr     *logging.VisitLogManager
    GeoIPService    services.GeoIPResolver
    Scheduler       *scheduler.Scheduler
    HealthChecker   monitoring.HealthChecker
    Monitor         *monitoring.Monitor
    SiteServerMgr   *siteserver.Manager
    SiteHandler     *sitehandler.Handler
    WafRepo         *repository.WafRepository
    AuditLogger     *audit.Logger
}
```

---

## 十五、日志模型

```go
type LogLevel int
const (
    DEBUG LogLevel = iota
    INFO
    WARN
    ERROR
    FATAL
)

type Logger struct {
    debugLogger  *log.Logger
    infoLogger   *log.Logger
    warnLogger   *log.Logger
    errorLogger  *log.Logger
    fatalLogger  *log.Logger
    auditLogger  *log.Logger
    auditLogs    []AuditLogEntry
    level        LogLevel
    auditEnabled bool
    maxAuditLogs int
}

type AuditLogEntry struct {
    Timestamp time.Time
    Level     string
    EventType string              // "admin_action"
    User      string
    IP        string
    Action    string
    Resource  string
    Details   map[string]interface{}
    Result    string              // "success"/"failure"
    Message   string
}
```

---

## 十六、GeoIP服务模型

```go
type GeoLocation struct {
    Country     string  // 国家名称
    CountryCode string  // ISO 3166-1 alpha-2
    City        string
    Latitude    float64
    Longitude   float64
}

type GeoIPResolver interface {
    LookupCountryISO(ip string) (string, error)
}

type GeoIPService struct {
    client         *http.Client      // 5秒超时
    serverLocation *GeoLocation      // 本机位置(内网IP回退)
    cache          sync.Map          // IP→Location内存缓存
}

// API提供商轮询顺序:
// 1. ip-api.com (免费, 45请求/分钟)
// 2. ipapi.co (免费, 1000请求/天)
// 3. get.geojs.io (免费, 无限制)
```

---

## 十七、域名解析模型

```go
type DomainResolver interface {
    Resolve(domain string) (string, error)       // 域名→站点ID
    AddMapping(domain, siteID string) error       // 添加映射
    RemoveMapping(domain string) error            // 移除映射
    ListMappings() (map[string]string, error)     // 列出所有映射
}

// Redis Key设计:
// domain:{domain} → siteID (精确匹配)
// 支持通配符: *.example.com
```

---

## 十八、反向代理模型

```go
type Proxy interface {
    ServeHTTP(w http.ResponseWriter, r *http.Request)
    AddBackend(siteID, backendURL string) error
    RemoveBackend(siteID string) error
    GetBackend(siteID string) (string, error)
}

// HTTP连接池配置:
// MaxIdleConns: 100
// MaxIdleConnsPerHost: 20
// IdleConnTimeout: 90s
// TLSHandshakeTimeout: 10s
```

---

## 十九、智能路由模型

```go
type Matcher interface {
    Match(req *http.Request, rule *RouteRule) bool
}

type Cache interface {
    Get(key string) interface{}
    Set(key string, value interface{}, ttl int) error
    Clear() error
}

type MemoryCache struct {
    cache map[string]cacheItem
}

type cacheItem struct {
    value      interface{}
    expiration time.Time
}
```
