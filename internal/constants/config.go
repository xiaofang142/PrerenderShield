package constants

import "time"

// Redis Key 常量
const (
	RedisKeyFirewallRules = "prerender:firewall:rules"
	RedisKeyConfigSites   = "prerender:config:sites"
	RedisKeyCachePrefix   = "prerender:cache:"
)

// 默认配置常量
const (
	DefaultConfigCheckInterval        = 5 * time.Second
	DefaultPrerenderTimeout           = 30 * time.Second
	DefaultCacheTTL                   = 3600 * time.Second
	DefaultIdleTimeout                = 300 * time.Second
	DefaultFileIntegrityCheckInterval = 300 * time.Second
	DefaultRuleUpdateInterval         = 24 * time.Hour
	DefaultCacheCleanInterval         = 5 * time.Minute
	DefaultCacheBatchSize             = 100
)

// 默认端口和地址
const (
	DefaultServerAddress = "0.0.0.0"
	DefaultAPIPort       = 9598
	DefaultConsolePort   = 9597
)

// 默认缓存配置
const (
	DefaultCacheType       = "memory"
	DefaultCacheMemorySize = 1000
	DefaultRedisDB         = 0
)

// 默认防火墙配置
const (
	DefaultFirewallAction       = "block"
	DefaultFirewallBlockMessage = "Request blocked by firewall"
	DefaultRateLimitRequests    = 100
	DefaultRateLimitWindow      = 60
	DefaultRateLimitBanTime     = 3600
)

// 默认预热配置
const (
	DefaultPreheatConcurrency     = 5
	DefaultPreheatMaxDepth        = 3
	DefaultPreheatScalingInterval = 60
)

// 默认 SSL 配置
const (
	DefaultHashAlgorithm = "sha256"
)

// RedisKeyAlertHistory 告警历史 ZSet 键（score=Unix 秒时间戳）
// 写入方：monitoring.Monitor.saveAlertToRedis / controllers.SaveAlertRecord
// 读取方：controllers.GetAlertHistory
const RedisKeyAlertHistory = "alert:history"
