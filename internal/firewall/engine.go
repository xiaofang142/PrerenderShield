package firewall

import (
	"net/http"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"

	"prerender-shield/internal/config"
	"prerender-shield/internal/firewall/detectors"
	"prerender-shield/internal/firewall/types"
)

// Engine 防火墙引擎
type Engine struct {
	SiteName       string // 站点名称
	mutex          sync.RWMutex
	owaspDetectors map[string]OWASPDetector
	coreDetectors  []CoreDetector
	actionHandler  ActionHandler
	ruleManager    *RuleManager
	logger         Logger
	requestCache   map[string]*CheckResult // 请求缓存，用于相同请求快速返回结果
	cacheMutex     sync.RWMutex            // 请求缓存互斥锁
	cacheTTL       time.Duration           // 请求缓存过期时间
}

// OWASPDetector OWASP Top 10检测器接口
type OWASPDetector interface {
	Detect(req *http.Request) ([]types.Threat, error)
	Name() string
}

// CoreDetector 核心安全检测器接口
type CoreDetector interface {
	Detect(req *http.Request) ([]types.Threat, error)
	Name() string
}

// ActionHandler 动作处理器接口
type ActionHandler interface {
	Handle(w http.ResponseWriter, req *http.Request, result *CheckResult) bool
}

// RuleManager 规则管理器
type RuleManager struct {
	rules map[string][]types.Rule
}

// GetRulesByCategory 根据分类获取规则
func (rm *RuleManager) GetRulesByCategory(category string) []types.Rule {
	return rm.rules[category]
}

// ReloadRules 重新加载规则
func (rm *RuleManager) ReloadRules() error {
	// 实现规则重新加载逻辑
	return nil
}

// NewRuleManager 创建新的规则管理器
func NewRuleManager() *RuleManager {
	return &RuleManager{
		rules: make(map[string][]types.Rule),
	}
}

// Logger 日志接口
type Logger interface {
	Error(format string, args ...interface{})
	Info(format string, args ...interface{})
}

// Config 防火墙配置
type Config struct {
	RulesPath           string
	ActionConfig        ActionConfig
	CacheTTL            int                         // 请求缓存过期时间（秒）
	StaticDir           string                      // 静态文件目录
	GeoIPConfig         *config.GeoIPConfig         // 地理位置访问控制配置
	RateLimitConfig     *config.RateLimitConfig     // 频率限制配置
	FileIntegrityConfig *config.FileIntegrityConfig // 网页防篡改配置
	Blacklist           []string                    // 静态黑名单
	Whitelist           []string                    // 静态白名单
	RedisClient         *redis.Client               // Redis客户端
}

// ActionConfig 动作配置
type ActionConfig struct {
	DefaultAction string
	BlockMessage  string
}

// CheckResult 检查结果
type CheckResult struct {
	Threats   []types.Threat
	CreatedAt time.Time
	Allow     bool
}

// EngineManager 防火墙引擎管理器，用于管理多个站点的防火墙引擎
type EngineManager struct {
	mutex   sync.RWMutex
	engines map[string]*Engine
}

// NewEngineManager 创建新的防火墙引擎管理器
func NewEngineManager() *EngineManager {
	return &EngineManager{
		engines: make(map[string]*Engine),
	}
}

// AddSite 添加站点并创建对应的防火墙引擎
func (em *EngineManager) AddSite(siteName string, config Config) error {
	em.mutex.Lock()
	defer em.mutex.Unlock()

	// 检查站点是否已存在
	if _, exists := em.engines[siteName]; exists {
		return nil // 站点已存在，无需重复创建
	}

	// 创建新的防火墙引擎
	engine, err := NewEngine(siteName, config)
	if err != nil {
		return err
	}

	em.engines[siteName] = engine
	return nil
}

// RemoveSite 移除站点及其防火墙引擎
func (em *EngineManager) RemoveSite(siteName string) {
	em.mutex.Lock()
	defer em.mutex.Unlock()

	delete(em.engines, siteName)
}

// GetEngine 获取指定站点的防火墙引擎
func (em *EngineManager) GetEngine(siteName string) (*Engine, bool) {
	em.mutex.RLock()
	defer em.mutex.RUnlock()

	engine, exists := em.engines[siteName]
	return engine, exists
}

// ListSites 列出所有站点
func (em *EngineManager) ListSites() []string {
	em.mutex.RLock()
	defer em.mutex.RUnlock()

	sites := make([]string, 0, len(em.engines))
	for siteName := range em.engines {
		sites = append(sites, siteName)
	}

	return sites
}

// NewEngine 创建新的防火墙引擎
func NewEngine(siteName string, config Config) (*Engine, error) {
	// 创建规则管理器
	ruleManager := NewRuleManager()

	// 设置默认缓存TTL为60秒
	cacheTTL := 60 * time.Second
	if config.CacheTTL > 0 {
		cacheTTL = time.Duration(config.CacheTTL) * time.Second
	}

	// 创建引擎实例
	e := &Engine{
		SiteName:       siteName,
		owaspDetectors: make(map[string]OWASPDetector),
		coreDetectors:  make([]CoreDetector, 0),
		ruleManager:    ruleManager,
		requestCache:   make(map[string]*CheckResult),
		cacheTTL:       cacheTTL,
	}

	// 初始化动作处理器
	e.actionHandler = NewDefaultActionHandler(config.ActionConfig, config.StaticDir, siteName)

	// 初始化OWASP Top 10检测器
	e.owaspDetectors["injection"] = detectors.NewInjectionDetector(ruleManager)
	e.owaspDetectors["xss"] = detectors.NewXSSDetector(ruleManager)
	e.owaspDetectors["csrf"] = detectors.NewCSRFDetector(ruleManager)
	e.owaspDetectors["deserialization"] = detectors.NewDeserializationDetector(ruleManager)
	e.owaspDetectors["sensitive-data"] = detectors.NewSensitiveDataDetector(ruleManager)

	// 初始化核心检测器
	e.coreDetectors = append(e.coreDetectors, detectors.NewGeoIPDetector(config.GeoIPConfig))
	e.coreDetectors = append(e.coreDetectors, detectors.NewRateLimitDetector(config.RateLimitConfig))
	e.coreDetectors = append(e.coreDetectors, detectors.NewFileIntegrityDetector(config.StaticDir, config.FileIntegrityConfig))
	e.coreDetectors = append(e.coreDetectors, detectors.NewBlacklistDetector(config.RedisClient, siteName, config.Blacklist, config.Whitelist))

	// 启动缓存清理协程
	go e.cleanCacheLoop()

	return e, nil
}

// CheckRequest 检查请求
func (e *Engine) CheckRequest(req *http.Request) (*CheckResult, error) {
	// 生成请求缓存键
	cacheKey := e.generateRequestCacheKey(req)

	// 检查请求缓存
	if cachedResult := e.getFromCache(cacheKey); cachedResult != nil {
		return cachedResult, nil
	}

	// 创建结果通道
	threatsChan := make(chan []types.Threat, len(e.owaspDetectors)+len(e.coreDetectors))
	errChan := make(chan error, len(e.owaspDetectors)+len(e.coreDetectors))

	// 并行执行OWASP Top 10检测
	var wg sync.WaitGroup

	// 执行OWASP检测器
	e.mutex.RLock()
	owaspDetectors := make(map[string]OWASPDetector)
	for k, v := range e.owaspDetectors {
		owaspDetectors[k] = v
	}
	coreDetectors := make([]CoreDetector, len(e.coreDetectors))
	copy(coreDetectors, e.coreDetectors)
	e.mutex.RUnlock()

	// 启动OWASP检测器协程
	for name, detector := range owaspDetectors {
		wg.Add(1)
		go func(det OWASPDetector, detectorName string) {
			defer wg.Done()
			threats, err := det.Detect(req)
			if err != nil {
				errChan <- err
				return
			}
			threatsChan <- threats
		}(detector, name)
	}

	// 启动核心检测器协程
	for _, detector := range coreDetectors {
		wg.Add(1)
		go func(det CoreDetector) {
			defer wg.Done()
			threats, err := det.Detect(req)
			if err != nil {
				errChan <- err
				return
			}
			threatsChan <- threats
		}(detector)
	}

	// 等待所有检测器完成
	go func() {
		wg.Wait()
		close(threatsChan)
		close(errChan)
	}()

	// 收集检测结果
	result := &CheckResult{
		Threats:   make([]types.Threat, 0),
		CreatedAt: time.Now(),
		Allow:     true,
	}

	// 收集威胁
	for threats := range threatsChan {
		result.Threats = append(result.Threats, threats...)
	}

	// 收集错误
	for err := range errChan {
		if e.logger != nil {
			e.logger.Error("Detector error: %s", err.Error())
		}
	}

	// 如果有威胁，设置Allow为false
	if len(result.Threats) > 0 {
		result.Allow = false
	}

	// 将结果添加到缓存
	e.addToCache(cacheKey, result)

	return result, nil
}

// HandleRequest 处理请求
func (e *Engine) HandleRequest(w http.ResponseWriter, req *http.Request) bool {
	// 检查请求
	result, err := e.CheckRequest(req)
	if err != nil {
		if e.logger != nil {
			e.logger.Error("Check request error: %s", err.Error())
		}
		return true // 出错时默认允许请求通过
	}

	// 如果检测到威胁，执行相应动作
	if len(result.Threats) > 0 {
		// 记录安全事件
		// 执行动作
		if e.actionHandler != nil {
			return e.actionHandler.Handle(w, req, result)
		}
		return false // 没有动作处理器，默认阻止
	}

	return true // 允许请求通过
}

// UpdateRules 更新规则
func (e *Engine) UpdateRules() error {
	// 更新规则
	if err := e.ruleManager.ReloadRules(); err != nil {
		return err
	}

	// 清空请求缓存，因为规则更新后，之前的缓存结果可能不再有效
	e.clearCache()

	return nil
}

// generateRequestCacheKey 生成请求缓存键
func (e *Engine) generateRequestCacheKey(req *http.Request) string {
	// 简单的缓存键生成，实际生产环境中可以使用更复杂的算法
	return req.Method + "|" + req.URL.String() + "|" + req.RemoteAddr
}

// getFromCache 从缓存获取结果
func (e *Engine) getFromCache(key string) *CheckResult {
	e.cacheMutex.RLock()
	defer e.cacheMutex.RUnlock()

	result, exists := e.requestCache[key]
	if !exists {
		return nil
	}

	// 检查缓存是否过期
	if time.Since(result.CreatedAt) > e.cacheTTL {
		return nil
	}

	return result
}

// addToCache 将结果添加到缓存
func (e *Engine) addToCache(key string, result *CheckResult) {
	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()

	e.requestCache[key] = result
}

// clearCache 清空缓存
func (e *Engine) clearCache() {
	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()

	e.requestCache = make(map[string]*CheckResult)
}

// cleanCacheLoop 定期清理过期缓存
func (e *Engine) cleanCacheLoop() {
	// 每5分钟清理一次过期缓存
	ticker := time.NewTicker(5 * time.Minute)
	for {
		<-ticker.C
		e.cleanExpiredCache()
	}
}

// cleanExpiredCache 清理过期缓存
func (e *Engine) cleanExpiredCache() {
	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()

	now := time.Now()
	for key, result := range e.requestCache {
		if now.Sub(result.CreatedAt) > e.cacheTTL {
			delete(e.requestCache, key)
		}
	}
}
