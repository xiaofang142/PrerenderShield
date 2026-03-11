package threatintel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ThreatIntelClient 威胁情报客户端
type ThreatIntelClient struct {
	config      *ThreatIntelConfig
	cache       ThreatIntelCache
	rateLimiter *RateLimiter
	httpClient  *http.Client
	logger      *zap.Logger
	mu          sync.RWMutex
	providers   map[string]ThreatProvider
}

// ThreatIntelConfig 威胁情报配置
type ThreatIntelConfig struct {
	VirusTotalAPIKey  string
	AbuseIPDBAPIKey   string
	AlienVaultAPIKey  string
	EnableVirusTotal  bool
	EnableAbuseIPDB   bool
	EnableAlienVault  bool
	CacheTTL          time.Duration
	CacheMaxSize      int
	RateLimitPerMin   int
	Timeout           time.Duration
}

// ThreatIntelResult 威胁情报查询结果
type ThreatIntelResult struct {
	IP            string                 `json:"ip"`
	Domain        string                 `json:"domain,omitempty"`
	IsMalicious   bool                   `json:"is_malicious"`
	Confidence    float64                `json:"confidence"` // 0-100
	RiskScore     int                    `json:"risk_score"` // 0-100
	Categories    []string               `json:"categories"`
	Provider      string                 `json:"provider"`
	RawData       map[string]interface{} `json:"raw_data"`
	QueryTime     time.Time              `json:"query_time"`
	CacheHit      bool                   `json:"cache_hit"`
	ErrorResponse string                 `json:"error_response,omitempty"`
}

// ThreatProvider 威胁情报提供者接口
type ThreatProvider interface {
	Name() string
	QueryIP(ctx context.Context, ip string) (*ThreatIntelResult, error)
	QueryDomain(ctx context.Context, domain string) (*ThreatIntelResult, error)
	IsEnabled() bool
}

// ThreatIntelCache 威胁情报缓存接口
type ThreatIntelCache interface {
	Get(key string) (*ThreatIntelResult, bool)
	Set(key string, result *ThreatIntelResult, ttl time.Duration)
	Delete(key string)
	Clear()
}

// DefaultThreatIntelConfig 返回默认配置
func DefaultThreatIntelConfig() *ThreatIntelConfig {
	return &ThreatIntelConfig{
		EnableVirusTotal: false,
		EnableAbuseIPDB:  false,
		EnableAlienVault: false,
		CacheTTL:         1 * time.Hour,
		CacheMaxSize:     10000,
		RateLimitPerMin:  60,
		Timeout:          10 * time.Second,
	}
}

// NewThreatIntelClient 创建威胁情报客户端
func NewThreatIntelClient(config *ThreatIntelConfig, logger *zap.Logger) *ThreatIntelClient {
	if config == nil {
		config = DefaultThreatIntelConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	client := &ThreatIntelClient{
		config: config,
		cache:  NewMemoryCache(config.CacheMaxSize),
		rateLimiter: NewRateLimiter(config.RateLimitPerMin),
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		logger:    logger,
		providers: make(map[string]ThreatProvider),
	}

	// 注册提供者
	if config.EnableVirusTotal && config.VirusTotalAPIKey != "" {
		client.providers["virustotal"] = NewVirusTotalProvider(config.VirusTotalAPIKey, client.httpClient)
	}
	if config.EnableAbuseIPDB && config.AbuseIPDBAPIKey != "" {
		client.providers["abuseipdb"] = NewAbuseIPDBProvider(config.AbuseIPDBAPIKey, client.httpClient)
	}
	if config.EnableAlienVault && config.AlienVaultAPIKey != "" {
		client.providers["alienvault"] = NewAlienVaultProvider(config.AlienVaultAPIKey, client.httpClient)
	}

	return client
}

// QueryIP 查询 IP 威胁情报
func (c *ThreatIntelClient) QueryIP(ctx context.Context, ip string) (*ThreatIntelResult, error) {
	if ip == "" {
		return nil, fmt.Errorf("IP 不能为空")
	}

	// 检查缓存
	cacheKey := "ip:" + ip
	if result, found := c.cache.Get(cacheKey); found {
		c.logger.Debug("威胁情报缓存命中", zap.String("ip", ip))
		result.CacheHit = true
		return result, nil
	}

	// 速率限制
	if !c.rateLimiter.Allow() {
		c.logger.Warn("威胁情报查询触发速率限制", zap.String("ip", ip))
		return &ThreatIntelResult{
			IP:            ip,
			IsMalicious:   false,
			Confidence:    0,
			RiskScore:     0,
			Provider:      "rate_limited",
			QueryTime:     time.Now(),
			ErrorResponse: "rate limited",
		}, nil
	}

	// 查询所有启用的提供者
	results := c.queryAllProviders(ctx, ip, "")

	// 聚合结果
	aggregated := c.aggregateResults(results)
	aggregated.IP = ip
	aggregated.QueryTime = time.Now()

	// 写入缓存
	if aggregated.RiskScore > 0 {
		c.cache.Set(cacheKey, aggregated, c.config.CacheTTL)
	}

	return aggregated, nil
}

// QueryDomain 查询域名威胁情报
func (c *ThreatIntelClient) QueryDomain(ctx context.Context, domain string) (*ThreatIntelResult, error) {
	if domain == "" {
		return nil, fmt.Errorf("域名不能为空")
	}

	// 检查缓存
	cacheKey := "domain:" + domain
	if result, found := c.cache.Get(cacheKey); found {
		c.logger.Debug("威胁情报缓存命中", zap.String("domain", domain))
		result.CacheHit = true
		return result, nil
	}

	// 速率限制
	if !c.rateLimiter.Allow() {
		c.logger.Warn("威胁情报查询触发速率限制", zap.String("domain", domain))
		return &ThreatIntelResult{
			Domain:        domain,
			IsMalicious:   false,
			Confidence:    0,
			RiskScore:     0,
			Provider:      "rate_limited",
			QueryTime:     time.Now(),
			ErrorResponse: "rate limited",
		}, nil
	}

	// 查询所有启用的提供者
	results := c.queryAllProviders(ctx, "", domain)

	// 聚合结果
	aggregated := c.aggregateResults(results)
	aggregated.Domain = domain
	aggregated.QueryTime = time.Now()

	// 写入缓存
	if aggregated.RiskScore > 0 {
		c.cache.Set(cacheKey, aggregated, c.config.CacheTTL)
	}

	return aggregated, nil
}

// queryAllProviders 查询所有提供者
func (c *ThreatIntelClient) queryAllProviders(ctx context.Context, ip, domain string) []*ThreatIntelResult {
	results := make([]*ThreatIntelResult, 0, len(c.providers))
	mu := sync.Mutex{}
	wg := sync.WaitGroup{}

	for name, provider := range c.providers {
		if !provider.IsEnabled() {
			continue
		}

		wg.Add(1)
		go func(name string, p ThreatProvider) {
			defer wg.Done()

			var result *ThreatIntelResult
			var err error

			if ip != "" {
				result, err = p.QueryIP(ctx, ip)
			} else if domain != "" {
				result, err = p.QueryDomain(ctx, domain)
			}

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				c.logger.Debug("威胁情报提供者查询失败",
					zap.String("provider", name),
					zap.Error(err))
				return
			}

			if result != nil {
				results = append(results, result)
			}
		}(name, provider)
	}

	wg.Wait()
	return results
}

// aggregateResults 聚合多个提供者的结果
func (c *ThreatIntelClient) aggregateResults(results []*ThreatIntelResult) *ThreatIntelResult {
	if len(results) == 0 {
		return &ThreatIntelResult{
			IsMalicious: false,
			Confidence:  0,
			RiskScore:   0,
			Categories:  []string{},
			RawData:     make(map[string]interface{}),
		}
	}

	if len(results) == 1 {
		return results[0]
	}

	// 多提供者聚合
	aggregated := &ThreatIntelResult{
		IsMalicious:  false,
		Confidence:   0,
		RiskScore:    0,
		Categories:   make([]string, 0),
		RawData:      make(map[string]interface{}),
		Provider:     "aggregated",
	}

	categorySet := make(map[string]bool)
	totalWeight := 0.0

	for _, r := range results {
		// 加权平均风险评分
		weight := 1.0
		if r.Confidence > 80 {
			weight = 1.5
		}

		aggregated.RiskScore += int(float64(r.RiskScore) * weight)
		totalWeight += weight

		// 合并类别
		for _, cat := range r.Categories {
			if !categorySet[cat] {
				categorySet[cat] = true
				aggregated.Categories = append(aggregated.Categories, cat)
			}
		}

		// 只要有一个提供者标记为恶意，就认为恶意
		if r.IsMalicious {
			aggregated.IsMalicious = true
		}

		// 合并原始数据
		aggregated.RawData[r.Provider] = r.RawData
	}

	// 计算平均风险评分
	if totalWeight > 0 {
		aggregated.RiskScore = aggregated.RiskScore / int(totalWeight)
	}

	// 计算置信度
	aggregated.Confidence = float64(len(results)) / float64(len(c.providers)) * 100

	return aggregated
}

// GetProviders 获取所有提供者信息
func (c *ThreatIntelClient) GetProviders() []map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	providers := make([]map[string]interface{}, 0, len(c.providers))
	for name, provider := range c.providers {
		providers = append(providers, map[string]interface{}{
			"name":      name,
			"enabled":   provider.IsEnabled(),
			"type":      fmt.Sprintf("%T", provider),
		})
	}
	return providers
}

// GetStats 获取统计信息
func (c *ThreatIntelClient) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"provider_count":  len(c.providers),
		"enabled_count":   c.countEnabledProviders(),
		"cache_size":      c.cache.(*MemoryCache).Size(),
		"rate_limit_remaining": c.rateLimiter.Remaining(),
	}
}

func (c *ThreatIntelClient) countEnabledProviders() int {
	count := 0
	for _, p := range c.providers {
		if p.IsEnabled() {
			count++
		}
	}
	return count
}

// MemoryCache 内存缓存实现
type MemoryCache struct {
	data     map[string]*cacheItem
	mu       sync.RWMutex
	maxSize  int
}

type cacheItem struct {
	result   *ThreatIntelResult
	expireAt time.Time
}

// NewMemoryCache 创建内存缓存
func NewMemoryCache(maxSize int) *MemoryCache {
	cache := &MemoryCache{
		data:    make(map[string]*cacheItem),
		maxSize: maxSize,
	}

	// 启动清理协程
	go cache.cleanup()

	return cache
}

// Get 获取缓存
func (c *MemoryCache) Get(key string) (*ThreatIntelResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.data[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(item.expireAt) {
		return nil, false
	}

	return item.result, true
}

// Set 设置缓存
func (c *MemoryCache) Set(key string, result *ThreatIntelResult, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果缓存已满，删除最旧的 10%
	if len(c.data) >= c.maxSize {
		c.evictOldest(10)
	}

	c.data[key] = &cacheItem{
		result:   result,
		expireAt: time.Now().Add(ttl),
	}
}

// Delete 删除缓存
func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}

// Clear 清空缓存
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]*cacheItem)
}

// Size 获取缓存大小
func (c *MemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}

// cleanup 定期清理过期缓存
func (c *MemoryCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, item := range c.data {
			if now.After(item.expireAt) {
				delete(c.data, key)
			}
		}
		c.mu.Unlock()
	}
}

// evictOldest 驱逐最旧的条目
func (c *MemoryCache) evictOldest(percent int) {
	if len(c.data) == 0 {
		return
	}

	count := len(c.data) * percent / 100
	if count < 1 {
		count = 1
	}

	// 找到最旧的 count 个条目
	keys := make([]string, 0, len(c.data))
	for k := range c.data {
		keys = append(keys, k)
	}

	// 简单删除前 count 个
	for i := 0; i < count && i < len(keys); i++ {
		delete(c.data, keys[i])
	}
}

// RateLimiter 简单令牌桶速率限制器
type RateLimiter struct {
	tokens     float64
	maxTokens  float64
	refillRate float64
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(tokensPerMin int) *RateLimiter {
	return &RateLimiter{
		tokens:     float64(tokensPerMin),
		maxTokens:  float64(tokensPerMin),
		refillRate: float64(tokensPerMin) / 60.0, // 每秒补充的令牌
		lastRefill: time.Now(),
	}
}

// Allow 检查是否允许请求
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 补充令牌
	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.tokens += elapsed * r.refillRate
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}
	r.lastRefill = now

	// 消耗令牌
	if r.tokens >= 1 {
		r.tokens--
		return true
	}

	return false
}

// Remaining 获取剩余令牌数
func (r *RateLimiter) Remaining() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return int(r.tokens)
}

// Helper functions for JSON handling
func toJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func safeGetFloat(m map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case int:
			return float64(val)
		case string:
			var f float64
			if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
				return f
			}
		}
	}
	return defaultVal
}

func safeGetInt(m map[string]interface{}, key string, defaultVal int) int {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		case string:
			var i int
			if _, err := fmt.Sscanf(val, "%d", &i); err == nil {
				return i
			}
		}
	}
	return defaultVal
}

func safeGetString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func safeGetBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}
