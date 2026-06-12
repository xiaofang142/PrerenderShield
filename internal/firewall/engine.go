package firewall

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"

	"prerender-shield/internal/config"
	"prerender-shield/internal/constants"
	"prerender-shield/internal/firewall/detectors"
	"prerender-shield/internal/firewall/detectors/ai"
	"prerender-shield/internal/firewall/types"
	"prerender-shield/internal/logging"
)

// FailStrategy 失败处理策略
type FailStrategy int

const (
	// FailOpen 失败时开放（默认允许）
	FailOpen FailStrategy = iota
	// FailClosed 失败时关闭（默认拒绝）
	FailClosed
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
	redisClient    *redis.Client          // Redis 客户端，用于请求缓存
	cacheTTL       time.Duration          // 请求缓存过期时间
	failStrategy   FailStrategy           // 失败处理策略
}

// OWASPDetector OWASP Top 10 检测器接口
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
	rules            map[string][]types.Rule
	rulesPath        string
	autoUpdate       bool
	updateInterval   time.Duration
	remoteRuleSource string
	redisClient      *redis.Client
	stopChan         chan struct{}
	updateMutex      sync.Mutex
}

// GetRulesByCategory 根据分类获取规则
func (rm *RuleManager) GetRulesByCategory(category string) []types.Rule {
	return rm.rules[category]
}

// ReloadRules 重新加载规则
func (rm *RuleManager) ReloadRules() error {
	rm.updateMutex.Lock()
	defer rm.updateMutex.Unlock()

	var rules map[string][]types.Rule
	var err error

	// 首先尝试从远程源获取规则
	if rm.remoteRuleSource != "" {
		rules, err = rm.fetchRulesFromRemote()
		if err == nil && len(rules) > 0 {
			rm.rules = rules
			// 保存规则到 Redis，便于快速加载
			_ = rm.saveRulesToRedis(rules)
			return nil
		}
	}

	// 尝试从 Redis 加载规则
	if rm.redisClient != nil {
		rules, err = rm.loadRulesFromRedis()
		if err == nil && len(rules) > 0 {
			rm.rules = rules
			return nil
		}
	}

	// 从本地文件加载规则
	if rm.rulesPath != "" {
		rules, err = rm.loadRulesFromFile()
		if err == nil && len(rules) > 0 {
			rm.rules = rules
			return nil
		}
	}

	// 使用默认规则
	rm.rules = rm.getDefaultRules()
	return nil
}

// startAutoUpdate 启动自动更新任务
func (rm *RuleManager) startAutoUpdate() {
	ticker := time.NewTicker(rm.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := rm.ReloadRules()
			if err != nil {
				logging.DefaultLogger.Info("Failed to update rules: %v\n", err)
			}
		case <-rm.stopChan:
			return
		}
	}
}

// StopAutoUpdate 停止自动更新任务
func (rm *RuleManager) StopAutoUpdate() {
	close(rm.stopChan)
}

// fetchRulesFromRemote 从远程源获取规则
func (rm *RuleManager) fetchRulesFromRemote() (map[string][]types.Rule, error) {
	if rm.remoteRuleSource == "" {
		return nil, fmt.Errorf("remote rule source not configured")
	}

	// 创建 HTTP 客户端，设置超时
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 发送请求获取规则
	resp, err := client.Get(rm.remoteRuleSource)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch rules from remote: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote rule source returned status: %d", resp.StatusCode)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read rule response: %w", err)
	}

	// 反序列化规则
	var rules map[string][]types.Rule
	if err := json.Unmarshal(body, &rules); err != nil {
		return nil, fmt.Errorf("failed to parse rules: %w", err)
	}

	return rules, nil
}

// saveRulesToRedis 保存规则到 Redis
func (rm *RuleManager) saveRulesToRedis(rules map[string][]types.Rule) error {
	if rm.redisClient == nil {
		return fmt.Errorf("redis client not set")
	}

	// 序列化规则
	rulesJSON, err := json.Marshal(rules)
	if err != nil {
		return err
	}

	// 保存到 Redis，设置过期时间为 7 天
	err = rm.redisClient.Set(rm.redisClient.Context(), constants.RedisKeyFirewallRules, rulesJSON, 7*24*time.Hour).Err()
	if err != nil {
		return err
	}

	return nil
}

// loadRulesFromRedis 从 Redis 加载规则
func (rm *RuleManager) loadRulesFromRedis() (map[string][]types.Rule, error) {
	if rm.redisClient == nil {
		return nil, fmt.Errorf("redis client not set")
	}

	// 从 Redis 获取规则
	rulesJSON, err := rm.redisClient.Get(rm.redisClient.Context(), constants.RedisKeyFirewallRules).Bytes()
	if err != nil {
		return nil, err
	}

	// 反序列化规则
	var rules map[string][]types.Rule
	err = json.Unmarshal(rulesJSON, &rules)
	if err != nil {
		return nil, err
	}

	return rules, nil
}

// loadRulesFromFile 从文件加载规则
func (rm *RuleManager) loadRulesFromFile() (map[string][]types.Rule, error) {
	if rm.rulesPath == "" {
		return nil, fmt.Errorf("rules path not configured")
	}

	// 检查文件是否存在
	if _, err := os.Stat(rm.rulesPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("rules file not found: %s", rm.rulesPath)
	}

	// 读取文件内容
	data, err := os.ReadFile(rm.rulesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file: %w", err)
	}

	// 根据文件扩展名选择解析方式
	var rules map[string][]types.Rule
	ext := strings.ToLower(filepath.Ext(rm.rulesPath))

	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &rules); err != nil {
			return nil, fmt.Errorf("failed to parse JSON rules: %w", err)
		}
	case ".yaml", ".yml":
		// 如果需要使用 YAML，需要引入 gopkg.in/yaml.v3
		// 这里暂时返回错误，表示不支持
		return nil, fmt.Errorf("YAML rules not yet implemented")
	default:
		// 尝试 JSON 格式
		if err := json.Unmarshal(data, &rules); err != nil {
			return nil, fmt.Errorf("unsupported rules file format: %s", ext)
		}
	}

	return rules, nil
}

// getDefaultRules 获取默认规则
func (rm *RuleManager) getDefaultRules() map[string][]types.Rule {
	rules := make(map[string][]types.Rule)

	// 初始化默认规则
	defaultRules := []types.Rule{
		{
			ID:       "rule-1",
			Name:     "SQL Injection Detection",
			Category: "injection",
			Pattern:  `(?i)(SELECT|INSERT|UPDATE|DELETE|DROP|ALTER)\s+.*\s*(FROM|INTO|TABLE|DATABASE)`,
			Severity: "high",
		},
		{
			ID:       "rule-2",
			Name:     "XSS Detection",
			Category: "xss",
			Pattern:  `(?i)<script[^>]*>.*</script>`,
			Severity: "high",
		},
		{
			ID:       "rule-3",
			Name:     "CSRF Detection",
			Category: "csrf",
			Pattern:  `(?i)csrf_token=.*`,
			Severity: "medium",
		},
	}

	// 按分类组织规则
	for _, rule := range defaultRules {
		rules[rule.Category] = append(rules[rule.Category], rule)
	}

	return rules
}

// NewRuleManager 创建新的规则管理器
func NewRuleManager(rulesPath string, autoUpdate bool, updateInterval time.Duration, remoteRuleSource string, redisClient *redis.Client) *RuleManager {
	if updateInterval <= 0 {
		updateInterval = constants.DefaultRuleUpdateInterval
	}

	rm := &RuleManager{
		rules:            make(map[string][]types.Rule),
		rulesPath:        rulesPath,
		autoUpdate:       autoUpdate,
		updateInterval:   updateInterval,
		remoteRuleSource: remoteRuleSource,
		redisClient:      redisClient,
		stopChan:         make(chan struct{}),
	}

	// 初始化加载规则
	rm.ReloadRules()

	// 启动自动更新任务
	if autoUpdate {
		go rm.startAutoUpdate()
	}

	return rm
}

// Logger 日志接口
type Logger interface {
	Error(format string, args ...interface{})
	Info(format string, args ...interface{})
}

// Config 防火墙配置
type Config struct {
	RulesPath           string                      // 规则文件路径
	ActionConfig        ActionConfig                // 动作配置
	CacheTTL            int                         // 请求缓存过期时间（秒）
	StaticDir           string                      // 静态文件目录
	GeoIPConfig         *config.GeoIPConfig         // 地理位置访问控制配置
	RateLimitConfig     *config.RateLimitConfig     // 频率限制配置
	FileIntegrityConfig *config.FileIntegrityConfig // 网页防篡改配置
	Blacklist           []string                    // 静态黑名单
	Whitelist           []string                    // 静态白名单
	RedisClient         *redis.Client               // Redis 客户端
	AIConfig            *AIEngineConfig             // AI 检测器配置
	FailStrategy        FailStrategy                // 失败处理策略
}

// AIEngineConfig AI 检测器引擎配置
type AIEngineConfig struct {
	Enabled             bool    // 是否启用 AI 检测器
	ModelPath           string  // 模型文件路径
	WorkerPool          int     // 工作池大小
	ConfidenceThreshold float32 // 置信度阈值
	TimeoutMs           int     // 预测超时时间 (毫秒)
	CacheSize           int     // 特征缓存大小
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
	// 创建规则管理器，启用自动更新
	ruleManager := NewRuleManager(
		config.RulesPath,
		true, // 启用自动更新
		constants.DefaultRuleUpdateInterval,
		"",                 // 远程规则源，暂时为空
		config.RedisClient, // Redis 客户端
	)

	// 设置默认缓存 TTL 为 60 秒
	cacheTTL := 60 * time.Second
	if config.CacheTTL > 0 {
		cacheTTL = time.Duration(config.CacheTTL) * time.Second
	}

	// 设置默认失败策略为 FailClosed（安全优先）
	failStrategy := config.FailStrategy
	if failStrategy == 0 {
		failStrategy = FailClosed
	}

	// 创建引擎实例
	e := &Engine{
		SiteName:       siteName,
		owaspDetectors: make(map[string]OWASPDetector),
		coreDetectors:  make([]CoreDetector, 0),
		ruleManager:    ruleManager,
		redisClient:    config.RedisClient,
		cacheTTL:       cacheTTL,
		failStrategy:   failStrategy,
	}

	// 初始化动作处理器
	e.actionHandler = NewDefaultActionHandler(config.ActionConfig, config.StaticDir, siteName)

	// 初始化 OWASP Top 10 检测器
	e.owaspDetectors["injection"] = detectors.NewInjectionDetector(ruleManager)
	e.owaspDetectors["xss"] = detectors.NewXSSDetector(ruleManager)
	e.owaspDetectors["csrf"] = detectors.NewCSRFDetector(ruleManager)
	e.owaspDetectors["deserialization"] = detectors.NewDeserializationDetector(ruleManager)
	e.owaspDetectors["sensitive-data"] = detectors.NewSensitiveDataDetector(ruleManager)
	e.owaspDetectors["xxe"] = detectors.NewXXEDetector(ruleManager)

	// 初始化 User-Agent 检测器
	e.owaspDetectors["user-agent"] = detectors.NewUserAgentDetector()

	// 初始化核心检测器
	// 检查 GeoIP 配置是否为 nil
	if config.GeoIPConfig != nil {
		e.coreDetectors = append(e.coreDetectors, detectors.NewGeoIPDetector(config.GeoIPConfig))
	}

	// 检查 RateLimit 配置是否为 nil
	if config.RateLimitConfig != nil {
		e.coreDetectors = append(e.coreDetectors, detectors.NewRateLimitDetector(config.RateLimitConfig))
	}

	// 检查 FileIntegrity 配置是否为 nil
	if config.FileIntegrityConfig != nil {
		e.coreDetectors = append(e.coreDetectors, detectors.NewFileIntegrityDetector(config.StaticDir, config.FileIntegrityConfig))
	}

	// 初始化黑名单检测器
	if config.RedisClient != nil {
		e.coreDetectors = append(e.coreDetectors, detectors.NewBlacklistDetector(config.RedisClient, siteName, config.Blacklist, config.Whitelist))
	} else {
		e.coreDetectors = append(e.coreDetectors, detectors.NewBlacklistDetector(nil, siteName, config.Blacklist, config.Whitelist))
	}

	// 初始化 AI 威胁检测器（如果启用）
	if config.AIConfig != nil && config.AIConfig.Enabled {
		aiConfig := &ai.Config{
			ModelPath:           config.AIConfig.ModelPath,
			WorkerPool:          config.AIConfig.WorkerPool,
			ConfidenceThreshold: config.AIConfig.ConfidenceThreshold,
			PredictTimeout:      time.Duration(config.AIConfig.TimeoutMs) * time.Millisecond,
			CacheSize:           config.AIConfig.CacheSize,
			Enabled:             true,
		}

		// 如果配置值无效，使用默认值
		if aiConfig.WorkerPool <= 0 {
			aiConfig.WorkerPool = 4
		}
		if aiConfig.ConfidenceThreshold <= 0 {
			aiConfig.ConfidenceThreshold = 0.85
		}
		if aiConfig.PredictTimeout <= 0 {
			aiConfig.PredictTimeout = 50 * time.Millisecond
		}
		if aiConfig.CacheSize <= 0 {
			aiConfig.CacheSize = 10000
		}

		aiDetector, err := ai.NewAIDetector(aiConfig)
		if err != nil {
			// AI 检测器初始化失败，记录错误但不影响其他检测器
			logging.DefaultLogger.Info("AI detector initialization failed: %v\n", err)
		} else {
			e.owaspDetectors["ai"] = aiDetector
		}
	}

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

	// 并行执行 OWASP Top 10 检测
	var wg sync.WaitGroup

	// 执行 OWASP 检测器
	e.mutex.RLock()
	owaspDetectors := make(map[string]OWASPDetector)
	for k, v := range e.owaspDetectors {
		owaspDetectors[k] = v
	}
	coreDetectors := make([]CoreDetector, len(e.coreDetectors))
	copy(coreDetectors, e.coreDetectors)
	e.mutex.RUnlock()

	// 启动 OWASP 检测器协程
	for name, detector := range owaspDetectors {
		wg.Add(1)
		go func(det OWASPDetector, detectorName string) {
			defer wg.Done()
			threats, err := det.Detect(req)
			if err != nil {
				errChan <- fmt.Errorf("detector %s error: %w", detectorName, err)
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
				errChan <- fmt.Errorf("core detector %s error: %w", det.Name(), err)
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

	// 收集错误，关键检测器错误会影响结果
	var criticalErrors []error
	for err := range errChan {
		if e.logger != nil {
			e.logger.Error("Detector error: %s", err.Error())
		}
		criticalErrors = append(criticalErrors, err)
	}

	// 如果有严重错误，根据失败策略处理
	if len(criticalErrors) > 0 && e.failStrategy == FailClosed {
		// FailClosed 策略：检测器失败时拒绝请求
		result.Allow = false
		result.Threats = append(result.Threats, types.Threat{
			Type:     "detector_error",
			SubType:  "Security Detector Failure",
			Severity: "critical",
			Message:  fmt.Sprintf("Security detector failed (%d errors), request blocked by fail-closed policy", len(criticalErrors)),
			RuleID:   "system-failclosed",
			RuleName: "Fail-Closed Policy",
		})
	}

	// 评估威胁严重度：只要有一个 high/critical 威胁就阻断
	hasHighThreat := false
	hasThreat := false
	for _, t := range result.Threats {
		hasThreat = true
		if t.Severity == "high" || t.Severity == "critical" || t.Severity == "" {
			// 空 Severity 也视为高危（fail secure）
			hasHighThreat = true
		}
	}

	if hasHighThreat {
		result.Allow = false
	} else if hasThreat {
		// 低危威胁：记录日志但不阻断
		result.Allow = true
		if e.logger != nil {
			for _, t := range result.Threats {
				e.logger.Info("Low-severity threat logged: %s - %s", t.Type, t.Message)
			}
		}
	}

	// 将结果添加到缓存
	e.addToCache(cacheKey, result)

	return result, nil
}

// HandleRequest 处理请求
// HandleRequest 处理请求并根据检查结果执行相应动作
// 参数:
//
//	w: HTTP 响应写入器
//	req: HTTP 请求
//
// 返回值:
//
//	bool: true 表示允许请求通过，false 表示拒绝
func (e *Engine) HandleRequest(w http.ResponseWriter, req *http.Request) bool {
	// 检查请求
	result, err := e.CheckRequest(req)
	if err != nil {
		if e.logger != nil {
			e.logger.Error("Check request error: %s, fail_strategy=%v", err.Error(), e.failStrategy)
		}
		// 根据失败策略决定
		if e.failStrategy == FailClosed {
			// FailClosed: 错误时拒绝请求
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Security check unavailable"))
			return false
		}
		// FailOpen: 错误时允许请求
		return true
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
// 使用 URL 规范化 + Header 指纹 + 请求体哈希的组合方式
func (e *Engine) generateRequestCacheKey(req *http.Request) string {
	// URL 规范化
	normalizedURL := normalizeURL(req.URL)

	// 获取真实客户端 IP（考虑代理）
	clientIP := getClientIP(req)

	// 计算请求体哈希（如果有）
	bodyHash := e.calculateBodyHash(req)

	// 组合缓存键
	key := fmt.Sprintf("%s|%s|%s|%s", req.Method, normalizedURL, clientIP, bodyHash)
	return key
}

// normalizeURL 规范化 URL，防止绕过
func normalizeURL(u *url.URL) string {
	// 复制 URL 以避免修改原始对象
	normalized := &url.URL{
		Scheme:   strings.ToLower(u.Scheme),
		Host:     strings.ToLower(u.Host),
		Path:     u.Path,
		RawQuery: normalizeQuery(u.RawQuery),
		Fragment: u.Fragment,
	}

	// 清理路径中的 .. 和 .
	normalized.Path = filepath.Clean(normalized.Path)

	// 如果路径为空，设置为/
	if normalized.Path == "" || normalized.Path == "." {
		normalized.Path = "/"
	}

	return normalized.String()
}

// normalizeQuery 规范化查询参数
func normalizeQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}

	// 解析查询参数
	params, err := url.ParseQuery(rawQuery)
	if err != nil {
		return rawQuery
	}

	// 对参数键进行排序，确保相同参数生成相同的查询字符串
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}

	// 构建规范化的查询字符串
	var normalized strings.Builder
	for i, key := range keys {
		if i > 0 {
			normalized.WriteByte('&')
		}
		values := params[key]
		for j, value := range values {
			if j > 0 {
				normalized.WriteByte('&')
			}
			normalized.WriteString(url.QueryEscape(key))
			normalized.WriteByte('=')
			normalized.WriteString(url.QueryEscape(value))
		}
	}

	return normalized.String()
}

// getClientIP 获取真实客户端 IP
func getClientIP(req *http.Request) string {
	// 检查 X-Forwarded-For 头
	xff := req.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For 可能包含多个 IP，取第一个
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// 检查 X-Real-IP 头
	xri := req.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// 使用 RemoteAddr
	return req.RemoteAddr
}

// calculateBodyHash 计算请求体哈希
func (e *Engine) calculateBodyHash(req *http.Request) string {
	if req.Body == nil {
		return ""
	}

	// 读取请求体
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return ""
	}

	// 关闭原始 Body 并重新设置，以便后续处理
	req.Body = io.NopCloser(strings.NewReader(string(body)))

	// 解析 Form 数据 (包含 URL 参数和 POST body)
	// 让 detectors 能通过 req.Form 访问所有输入
	if len(body) > 0 {
		if err := req.ParseForm(); err != nil {
			logging.DefaultLogger.Warn("Failed to parse form data: %v", err)
		}
		if err := req.ParseMultipartForm(32 << 20); err != nil {
			// 非 multipart 请求会返回错误，这是正常的
			_ = err
		}
	}

	if len(body) == 0 {
		return ""
	}

	// 计算 SHA256 哈希
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:])
}

// getFromCache 从 Redis 获取缓存结果
func (e *Engine) getFromCache(key string) *CheckResult {
	if e.redisClient == nil {
		return nil
	}

	cacheKey := fmt.Sprintf("firewall:cache:%s:%s", e.SiteName, key)
	data, err := e.redisClient.Get(e.redisClient.Context(), cacheKey).Bytes()
	if err != nil {
		return nil
	}

	var result CheckResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return &result
}

// addToCache 将结果添加到 Redis 缓存
func (e *Engine) addToCache(key string, result *CheckResult) {
	if e.redisClient == nil {
		return
	}

	cacheKey := fmt.Sprintf("firewall:cache:%s:%s", e.SiteName, key)
	data, err := json.Marshal(result)
	if err != nil {
		return
	}
	e.redisClient.Set(e.redisClient.Context(), cacheKey, data, e.cacheTTL)
}

// clearCache 清空缓存
func (e *Engine) clearCache() {
	if e.redisClient == nil {
		return
	}

	pattern := fmt.Sprintf("firewall:cache:%s:*", e.SiteName)
	keys, err := e.redisClient.Keys(e.redisClient.Context(), pattern).Result()
	if err != nil {
		return
	}
	if len(keys) > 0 {
		e.redisClient.Del(e.redisClient.Context(), keys...)
	}
}
