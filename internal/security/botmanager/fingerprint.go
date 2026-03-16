package botmanager

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// FingerprintEngine 指纹识别引擎
type FingerprintEngine struct {
	config      *BotConfig
	logger      *zap.Logger
	cache       *FingerprintCache
	parser      *UserAgentParser
	tlsAnalyzer *TLSAnalyzer

	// 已知机器人指纹库
	knownBotSignatures []BotSignature
	mu                 sync.RWMutex

	// 统计信息
	stats *FingerprintStats
}

// FingerprintCache 指纹缓存
type FingerprintCache struct {
	data map[string]*FingerprintEntry
	mu   sync.RWMutex
	ttl  time.Duration
	size int
}

// FingerprintEntry 缓存条目
type FingerprintEntry struct {
	Fingerprint *Fingerprint
	ExpiresAt   time.Time
}

// BotSignature 已知机器人签名
type BotSignature struct {
	Name      string   `json:"name"`
	Category  string   `json:"category"` // search_engine, monitoring, malicious, etc.
	Patterns  []string `json:"patterns"` // User-Agent 正则模式
	JA3Hashes []string `json:"ja3_hashes"`
	IsGoodBot bool     `json:"is_good_bot"`
}

// FingerprintStats 指纹统计
type FingerprintStats struct {
	TotalFingerprints int64 `json:"total_fingerprints"`
	BotDetected       int64 `json:"bot_detected"`
	GoodBot           int64 `json:"good_bot"`
	BadBot            int64 `json:"bad_bot"`
	Unknown           int64 `json:"unknown"`
	CacheHits         int64 `json:"cache_hits"`
	CacheMisses       int64 `json:"cache_misses"`
}

// UserAgentParser User-Agent 解析器
type UserAgentParser struct {
	botPatterns     []regexp.Regexp
	browserPatterns map[string]*regexp.Regexp
	osPatterns      map[string]*regexp.Regexp
	devicePatterns  map[string]*regexp.Regexp
}

// TLSAnalyzer TLS 分析器
type TLSAnalyzer struct {
	ja3Hashes map[string]string // hash -> bot name
	knownJa3  []string
	mu        sync.RWMutex
}

// NewFingerprintEngine 创建指纹识别引擎
func NewFingerprintEngine(config *BotConfig, logger *zap.Logger) *FingerprintEngine {
	if config == nil {
		config = DefaultBotConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	engine := &FingerprintEngine{
		config:      config,
		logger:      logger,
		cache:       NewFingerprintCache(config.CacheSize, config.CacheTTL),
		parser:      NewUserAgentParser(),
		tlsAnalyzer: NewTLSAnalyzer(),
		stats:       &FingerprintStats{},
	}

	// 加载已知机器人签名
	engine.loadBotSignatures()

	return engine
}

// NewFingerprintCache 创建指纹缓存
func NewFingerprintCache(size int, ttl time.Duration) *FingerprintCache {
	cache := &FingerprintCache{
		data: make(map[string]*FingerprintEntry, size),
		ttl:  ttl,
		size: size,
	}

	// 启动清理协程
	go cache.cleanupWorker()

	return cache
}

// NewUserAgentParser 创建 User-Agent 解析器
func NewUserAgentParser() *UserAgentParser {
	parser := &UserAgentParser{
		botPatterns:     make([]regexp.Regexp, 0),
		browserPatterns: make(map[string]*regexp.Regexp),
		osPatterns:      make(map[string]*regexp.Regexp),
		devicePatterns:  make(map[string]*regexp.Regexp),
	}

	// 机器人模式
	botPatterns := []string{
		`bot`, `crawler`, `spider`, `scraper`,
		`curl`, `wget`, `httpclient`,
		`python-requests`, `java/`, `go-http-client`,
		`phantomjs`, `selenium`, `headlesschrome`,
		`slurp`, `bingbot`, `googlebot`, `yandexbot`,
		`baiduspider`, `socks`, `axios`, `node-fetch`,
	}
	for _, p := range botPatterns {
		if re, err := regexp.Compile(`(?i)` + p); err == nil {
			parser.botPatterns = append(parser.botPatterns, *re)
		}
	}

	// 浏览器模式
	browserPatterns := map[string]string{
		`chrome`:  `(?i)Chrome/(\d+\.\d+\.\d+\.\d+)`,
		`firefox`: `(?i)Firefox/(\d+\.\d+)`,
		`safari`:  `(?i)Safari/(\d+\.\d+\.\d+)`,
		`edge`:    `(?i)Edg/(\d+\.\d+\.\d+\.\d+)`,
		`opera`:   `(?i)OPR/(\d+\.\d+\.\d+\.\d+)`,
	}
	for name, pattern := range browserPatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			parser.browserPatterns[name] = re
		}
	}

	// 操作系统模式
	osPatterns := map[string]string{
		`windows`: `(?i)Windows NT (\d+\.\d+)`,
		`macos`:   `(?i)Mac OS X (\d+[_.]\d+)`,
		`linux`:   `(?i)Linux`,
		`android`: `(?i)Android (\d+\.\d+)`,
		`ios`:     `(?i)iPhone OS (\d+_\d+)`,
	}
	for name, pattern := range osPatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			parser.osPatterns[name] = re
		}
	}

	// 设备模式
	devicePatterns := map[string]string{
		`mobile`:  `(?i)Mobile|Android|iPhone`,
		`tablet`:  `(?i)Tablet|iPad`,
		`desktop`: `(?i)Windows NT|Mac OS X|Linux`,
	}
	for name, pattern := range devicePatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			parser.devicePatterns[name] = re
		}
	}

	return parser
}

// NewTLSAnalyzer 创建 TLS 分析器
func NewTLSAnalyzer() *TLSAnalyzer {
	return &TLSAnalyzer{
		ja3Hashes: make(map[string]string),
		knownJa3:  make([]string, 0),
	}
}

// Analyze 分析请求并生成指纹
func (e *FingerprintEngine) Analyze(
	userAgent string,
	tlsHash string,
	http2Hash string,
	headers map[string]string,
) *Fingerprint {
	e.stats.TotalFingerprints++

	// 检查缓存
	cacheKey := e.generateCacheKey(userAgent, tlsHash)
	if cached := e.cache.Get(cacheKey); cached != nil {
		e.stats.CacheHits++
		return cached
	}

	e.stats.CacheMisses++

	// 解析 User-Agent
	uaInfo := e.parser.Parse(userAgent)

	// 生成指纹
	fingerprint := &Fingerprint{
		ID:             e.generateFingerprintID(userAgent, tlsHash, http2Hash),
		UserAgent:      userAgent,
		UserAgentHash:  e.hashString(userAgent),
		TLSHash:        tlsHash,
		HTTP2Hash:      http2Hash,
		DeviceType:     uaInfo.DeviceType,
		OS:             uaInfo.OS,
		OSVersion:      uaInfo.OSVersion,
		Browser:        uaInfo.Browser,
		BrowserVersion: uaInfo.BrowserVersion,
		Device:         uaInfo.Device,
		Headers:        headers,
		Confidence:     uaInfo.Confidence,
		CreatedAt:      time.Now(),
	}

	// 检测机器人
	fingerprint.IsBot, fingerprint.BotScore = e.detectBot(fingerprint)

	// 确定风险等级
	fingerprint.RiskLevel = e.determineRiskLevel(fingerprint.BotScore)

	// 缓存结果
	e.cache.Set(cacheKey, fingerprint)

	e.logger.Debug("指纹分析完成",
		zap.String("fingerprint_id", fingerprint.ID),
		zap.Bool("is_bot", fingerprint.IsBot),
		zap.Float64("bot_score", fingerprint.BotScore),
		zap.String("risk_level", string(fingerprint.RiskLevel)),
	)

	return fingerprint
}

// detectBot 检测是否为机器人
func (e *FingerprintEngine) detectBot(fp *Fingerprint) (bool, float64) {
	score := 0.0

	// 1. User-Agent 检测
	if e.isBotUserAgent(fp.UserAgent) {
		score += 40
	}

	// 2. JA3 指纹检测
	if fp.TLSHash != "" && e.tlsAnalyzer.IsKnownBot(fp.TLSHash) {
		score += 30
	}

	// 3. HTTP/2 指纹检测
	if fp.HTTP2Hash != "" && e.isBotHTTP2(fp.HTTP2Hash) {
		score += 20
	}

	// 4. 已知机器人签名匹配
	if e.matchesKnownBot(fp.UserAgent) {
		score += 50
	}

	// 5. 异常 User-Agent 检测
	if e.isAnomalousUserAgent(fp.UserAgent) {
		score += 15
	}

	// 6. 缺失或异常的 Headers
	if e.hasAnomalousHeaders(fp.Headers) {
		score += 10
	}

	isBot := score >= e.config.BotScoreThreshold
	if isBot {
		e.stats.BotDetected++
	}

	return isBot, score
}

// isBotUserAgent 检查 User-Agent 是否为机器人
func (e *FingerprintEngine) isBotUserAgent(ua string) bool {
	ua = strings.ToLower(ua)
	for _, pattern := range e.parser.botPatterns {
		if pattern.MatchString(ua) {
			return true
		}
	}
	return false
}

// matchesKnownBot 匹配已知机器人
func (e *FingerprintEngine) matchesKnownBot(ua string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, sig := range e.knownBotSignatures {
		for _, pattern := range sig.Patterns {
			if re, err := regexp.Compile(pattern); err == nil {
				if re.MatchString(ua) {
					return true
				}
			}
		}
	}
	return false
}

// isAnomalousUserAgent 检测异常 User-Agent
func (e *FingerprintEngine) isAnomalousUserAgent(ua string) bool {
	// 过短的 User-Agent
	if len(ua) < 10 {
		return true
	}

	// 过长的 User-Agent
	if len(ua) > 500 {
		return true
	}

	// 包含乱码
	if strings.Contains(ua, "\\x") || strings.Contains(ua, "\\u") {
		return true
	}

	return false
}

// hasAnomalousHeaders 检测异常 Headers
func (e *FingerprintEngine) hasAnomalousHeaders(headers map[string]string) bool {
	if headers == nil {
		return true
	}

	// 检查关键 header
	criticalHeaders := []string{"user-agent", "accept", "accept-language"}
	for _, h := range criticalHeaders {
		if _, exists := headers[h]; !exists {
			return true
		}
	}

	return false
}

// determineRiskLevel 确定风险等级
func (e *FingerprintEngine) determineRiskLevel(score float64) RiskLevel {
	if score >= e.config.CriticalThreshold {
		return RiskLevelCritical
	}
	if score >= e.config.SuspiciousThreshold {
		return RiskLevelSuspicious
	}
	if score >= e.config.BotScoreThreshold {
		return RiskLevelHigh
	}
	if score > 0 {
		return RiskLevelNormal
	}
	return RiskLevelTrusted
}

// generateFingerprintID 生成指纹 ID
func (e *FingerprintEngine) generateFingerprintID(ua, tlsHash, http2Hash string) string {
	data := fmt.Sprintf("%s|%s|%s", ua, tlsHash, http2Hash)
	return e.hashString(data)
}

// generateCacheKey 生成缓存键
func (e *FingerprintEngine) generateCacheKey(ua, tlsHash string) string {
	return fmt.Sprintf("%s|%s", ua, tlsHash)
}

// hashString 生成哈希
func (e *FingerprintEngine) hashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

// loadBotSignatures 加载已知机器人签名
func (e *FingerprintEngine) loadBotSignatures() {
	e.knownBotSignatures = []BotSignature{
		// 搜索引擎机器人（好机器人）
		{
			Name:      "Googlebot",
			Category:  "search_engine",
			Patterns:  []string{`(?i)Googlebot`},
			JA3Hashes: []string{},
			IsGoodBot: true,
		},
		{
			Name:      "Bingbot",
			Category:  "search_engine",
			Patterns:  []string{`(?i)bingbot`},
			JA3Hashes: []string{},
			IsGoodBot: true,
		},
		{
			Name:      "Baiduspider",
			Category:  "search_engine",
			Patterns:  []string{`(?i)Baiduspider`},
			JA3Hashes: []string{},
			IsGoodBot: true,
		},
		// 监控工具
		{
			Name:      "UptimeRobot",
			Category:  "monitoring",
			Patterns:  []string{`(?i)UptimeRobot`},
			JA3Hashes: []string{},
			IsGoodBot: true,
		},
		// 恶意爬虫
		{
			Name:      "Generic Scraper",
			Category:  "scraper",
			Patterns:  []string{`(?i)scraper|scrapy`},
			JA3Hashes: []string{},
			IsGoodBot: false,
		},
	}
}

// GetStats 获取统计信息
func (e *FingerprintEngine) GetStats() *FingerprintStats {
	return e.stats
}

// Get 获取缓存
func (c *FingerprintCache) Get(key string) *Fingerprint {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.data[key]
	if !exists {
		return nil
	}

	if time.Now().After(entry.ExpiresAt) {
		return nil
	}

	return entry.Fingerprint
}

// Set 设置缓存
func (c *FingerprintCache) Set(key string, fp *Fingerprint) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果缓存已满，删除最旧的
	if len(c.data) >= c.size {
		c.evictOldest()
	}

	c.data[key] = &FingerprintEntry{
		Fingerprint: fp,
		ExpiresAt:   time.Now().Add(c.ttl),
	}
}

// evictOldest 删除最旧的条目
func (c *FingerprintCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.data {
		if oldestKey == "" || entry.ExpiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.ExpiresAt
		}
	}

	if oldestKey != "" {
		delete(c.data, oldestKey)
	}
}

// cleanupWorker 清理工
func (c *FingerprintCache) cleanupWorker() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup 清理过期条目
func (c *FingerprintCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	deleted := 0

	for key, entry := range c.data {
		if now.After(entry.ExpiresAt) {
			delete(c.data, key)
			deleted++
		}
	}

	if deleted > 0 {
		fmt.Printf("清理过期指纹缓存：%d 条\n", deleted)
	}
}

// UAInfo User-Agent 解析结果
type UAInfo struct {
	Browser        string
	BrowserVersion string
	OS             string
	OSVersion      string
	Device         string
	DeviceType     string
	Confidence     float64
}

// Parse 解析 User-Agent
func (p *UserAgentParser) Parse(ua string) *UAInfo {
	info := &UAInfo{
		Confidence: 0.5,
	}

	// 检测浏览器
	for name, pattern := range p.browserPatterns {
		if matches := pattern.FindStringSubmatch(ua); matches != nil {
			info.Browser = name
			info.BrowserVersion = matches[1]
			info.Confidence += 0.2
			break
		}
	}

	// 检测操作系统
	for name, pattern := range p.osPatterns {
		if matches := pattern.FindStringSubmatch(ua); matches != nil {
			info.OS = name
			if len(matches) > 1 {
				info.OSVersion = strings.Replace(matches[1], "_", ".", -1)
			}
			info.Confidence += 0.15
			break
		}
	}

	// 检测设备类型
	if p.devicePatterns["mobile"].MatchString(ua) {
		info.DeviceType = "mobile"
		info.Confidence += 0.1
	} else if p.devicePatterns["tablet"].MatchString(ua) {
		info.DeviceType = "tablet"
		info.Confidence += 0.1
	} else if p.devicePatterns["desktop"].MatchString(ua) {
		info.DeviceType = "desktop"
		info.Confidence += 0.1
	} else {
		info.DeviceType = "unknown"
	}

	// 置信度上限
	if info.Confidence > 1.0 {
		info.Confidence = 1.0
	}

	return info
}

// IsKnownBot 检查 JA3 哈希是否为已知机器人
func (t *TLSAnalyzer) IsKnownBot(ja3Hash string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, exists := t.ja3Hashes[ja3Hash]
	return exists
}

// AddKnownJA3 添加已知 JA3 哈希
func (t *TLSAnalyzer) AddKnownJA3(hash, botName string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ja3Hashes[hash] = botName
	t.knownJa3 = append(t.knownJa3, hash)
}

// isBotHTTP2 检查 HTTP/2 指纹是否为机器人
func (e *FingerprintEngine) isBotHTTP2(hash string) bool {
	// 简单的实现：检查是否包含已知的机器人 HTTP/2 特征
	// 实际应用中需要维护一个 HTTP/2 指纹库
	return false
}

// RegisterBotSignature 注册机器人签名
func (e *FingerprintEngine) RegisterBotSignature(sig BotSignature) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.knownBotSignatures = append(e.knownBotSignatures, sig)
}

// GetKnownBots 获取已知机器人列表
func (e *FingerprintEngine) GetKnownBots() []BotSignature {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]BotSignature, len(e.knownBotSignatures))
	copy(result, e.knownBotSignatures)
	return result
}

// AnalyzeUserAgent 分析 User-Agent
func (e *FingerprintEngine) AnalyzeUserAgent(ua string) *UAInfo {
	return e.parser.Parse(ua)
}

// ParseUserAgent 解析 User-Agent（别名）
func ParseUserAgent(ua string) *UAInfo {
	parser := NewUserAgentParser()
	return parser.Parse(ua)
}

// sortKeys 排序 map 键
func sortKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
