package waf

import (
	"net/http"
	"sync"
	"time"

	"prerender-shield/internal/logging"
	"prerender-shield/internal/security/waf/types"
)

// Engine WAF 引擎实现
type Engine struct {
	siteName      string
	config        *Config
	logger        *logging.Logger
	ruleManager   *RuleManager
	actionHandler ActionHandler
	requestCache  map[string]*cacheEntry
	stats         *WAFStats
	mu            sync.RWMutex
}

type cacheEntry struct {
	result    *types.CheckResult
	expiredAt time.Time
}

// WAFStats WAF 统计
type WAFStats struct {
	TotalRequests   int64
	BlockedRequests int64
	AllowedRequests int64
	mu              sync.RWMutex
}

// Config WAF 配置
type Config struct {
	RulesPath           string
	ActionConfig        types.ActionConfig
	StaticDir           string
	GeoIPConfig         *types.GeoIPConfig
	RateLimitConfig     *types.RateLimitConfig
	FileIntegrityConfig *types.FileIntegrityConfig
	Blacklist           []string
	Whitelist           []string
	RedisClient         interface{}
}

// GeoIPConfig GeoIP 配置
type GeoIPConfig struct {
	Enabled          bool     `json:"enabled"`
	DatabasePath     string   `json:"database_path"`
	AllowedCountries []string `json:"allowed_countries"`
	BlockedCountries []string `json:"blocked_countries"`
}

// RateLimitConfig 速率限制配置
type RateLimitConfig struct {
	Enabled           bool `json:"enabled"`
	RequestsPerMinute int  `json:"requests_per_minute"`
	BurstSize         int  `json:"burst_size"`
}

// FileIntegrityConfig 文件完整性配置
type FileIntegrityConfig struct {
	Enabled       bool     `json:"enabled"`
	WatchPaths    []string `json:"watch_paths"`
	HashAlgorithm string   `json:"hash_algorithm"`
	CheckInterval int      `json:"check_interval"`
}

// Check implements WebApplicationFirewall interface
func (e *Engine) Check(req *http.Request) (*types.CheckResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 检查缓存
	cacheKey := e.getCacheKey(req)
	if entry, ok := e.requestCache[cacheKey]; ok {
		if time.Now().Before(entry.expiredAt) {
			return entry.result, nil
		}
	}

	// 执行检查
	result := &types.CheckResult{
		Allowed: true,
	}

	// 检查白名单
	if e.isWhitelisted(req) {
		result.Allowed = true
		e.cacheResult(cacheKey, result)
		return result, nil
	}

	// 检查黑名单
	if e.isBlacklisted(req) {
		result.Allowed = false
		result.Blocked = true
		result.Reason = "IP blacklisted"
		e.stats.BlockedRequests++
		e.cacheResult(cacheKey, result)
		return result, nil
	}

	e.stats.AllowedRequests++
	e.cacheResult(cacheKey, result)
	return result, nil
}

func (e *Engine) getCacheKey(req *http.Request) string {
	return req.RemoteAddr + ":" + req.URL.Path
}

func (e *Engine) cacheResult(key string, result *types.CheckResult) {
	e.requestCache[key] = &cacheEntry{
		result:    result,
		expiredAt: time.Now().Add(5 * time.Second),
	}
}

func (e *Engine) isWhitelisted(req *http.Request) bool {
	for _, ip := range e.config.Whitelist {
		if ip == req.RemoteAddr {
			return true
		}
	}
	return false
}

func (e *Engine) isBlacklisted(req *http.Request) bool {
	for _, ip := range e.config.Blacklist {
		if ip == req.RemoteAddr {
			return true
		}
	}
	return false
}

// AddRule 添加规则
func (e *Engine) AddRule(rule interface{}) error {
	return e.ruleManager.AddRule(rule)
}

// RemoveRule 删除规则
func (e *Engine) RemoveRule(id string) error {
	return e.ruleManager.RemoveRule(id)
}

// UpdateRules 更新规则
func (e *Engine) UpdateRules() error {
	return e.ruleManager.LoadRules()
}

// Close 关闭引擎
func (e *Engine) Close() error {
	return nil
}

// NewEngine 创建新引擎
func NewEngine(siteName string, config Config, logger *logging.Logger) *Engine {
	ruleManager := NewRuleManager(config.RulesPath)
	actionHandler := NewDefaultActionHandler(config.ActionConfig)

	engine := &Engine{
		siteName:      siteName,
		config:        &config,
		logger:        logger,
		ruleManager:   ruleManager,
		actionHandler: actionHandler,
		requestCache:  make(map[string]*cacheEntry),
		stats:         &WAFStats{},
	}

	// 加载规则
	if err := ruleManager.LoadRules(); err != nil {
		logger.Warn("Failed to load WAF rules: %v", err)
	}

	return engine
}
