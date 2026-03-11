package threatintel

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultThreatIntelConfig(t *testing.T) {
	config := DefaultThreatIntelConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 1*time.Hour, config.CacheTTL)
	assert.Equal(t, 10000, config.CacheMaxSize)
	assert.Equal(t, 60, config.RateLimitPerMin)
	assert.Equal(t, 10*time.Second, config.Timeout)
	assert.False(t, config.EnableVirusTotal)
	assert.False(t, config.EnableAbuseIPDB)
	assert.False(t, config.EnableAlienVault)
}

func TestNewThreatIntelClient(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	config := &ThreatIntelConfig{
		EnableVirusTotal: false,
		EnableAbuseIPDB:  false,
		EnableAlienVault: false,
		CacheTTL:         30 * time.Minute,
		CacheMaxSize:     5000,
		RateLimitPerMin:  30,
	}

	client := NewThreatIntelClient(config, logger)

	assert.NotNil(t, client)
	assert.NotNil(t, client.cache)
	assert.NotNil(t, client.rateLimiter)
	assert.NotNil(t, client.httpClient)
	assert.Equal(t, 0, len(client.providers)) // 没有启用任何提供者

	stats := client.GetStats()
	assert.Equal(t, 0, stats["provider_count"])
	assert.Equal(t, 0, stats["enabled_count"])
}

func TestThreatIntelClient_QueryIP_EmptyProviders(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := NewThreatIntelClient(DefaultThreatIntelConfig(), logger)

	ctx := context.Background()
	result, err := client.QueryIP(ctx, "8.8.8.8")

	assert.Nil(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "8.8.8.8", result.IP)
	assert.False(t, result.IsMalicious)
	assert.Equal(t, 0, result.RiskScore)
}

func TestThreatIntelClient_QueryDomain_EmptyProviders(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := NewThreatIntelClient(DefaultThreatIntelConfig(), logger)

	ctx := context.Background()
	result, err := client.QueryDomain(ctx, "google.com")

	assert.Nil(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "google.com", result.Domain)
	assert.False(t, result.IsMalicious)
	assert.Equal(t, 0, result.RiskScore)
}

func TestThreatIntelClient_QueryIP_InvalidIP(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := NewThreatIntelClient(DefaultThreatIntelConfig(), logger)

	ctx := context.Background()
	result, err := client.QueryIP(ctx, "")

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "IP 不能为空")
	assert.Nil(t, result)
}

func TestThreatIntelClient_QueryDomain_InvalidDomain(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := NewThreatIntelClient(DefaultThreatIntelConfig(), logger)

	ctx := context.Background()
	result, err := client.QueryDomain(ctx, "")

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "域名不能为空")
	assert.Nil(t, result)
}

func TestMemoryCache(t *testing.T) {
	cache := NewMemoryCache(100)

	result := &ThreatIntelResult{
		IP:          "8.8.8.8",
		IsMalicious: false,
		RiskScore:   0,
		Confidence:  50.0,
		Provider:    "test",
	}

	// 测试 Set 和 Get
	cache.Set("ip:8.8.8.8", result, 1*time.Hour)

	cached, found := cache.Get("ip:8.8.8.8")
	assert.True(t, found)
	assert.Equal(t, "8.8.8.8", cached.IP)
	assert.Equal(t, 1, cache.Size())

	// 测试 Delete
	cache.Delete("ip:8.8.8.8")
	cached, found = cache.Get("ip:8.8.8.8")
	assert.False(t, found)
	assert.Nil(t, cached)

	// 测试 Clear
	cache.Set("ip:1.1.1.1", result, 1*time.Hour)
	cache.Set("ip:2.2.2.2", result, 1*time.Hour)
	assert.Equal(t, 2, cache.Size())

	cache.Clear()
	assert.Equal(t, 0, cache.Size())
}

func TestMemoryCache_Expire(t *testing.T) {
	cache := NewMemoryCache(100)

	result := &ThreatIntelResult{
		IP:          "8.8.8.8",
		IsMalicious: false,
		RiskScore:   0,
	}

	// 设置很短的过期时间
	cache.Set("ip:8.8.8.8", result, 100*time.Millisecond)

	// 立即获取应该存在
	cached, found := cache.Get("ip:8.8.8.8")
	assert.True(t, found)
	assert.NotNil(t, cached)

	// 等待过期
	time.Sleep(200 * time.Millisecond)

	// 过期后应该不存在
	cached, found = cache.Get("ip:8.8.8.8")
	assert.False(t, found)
	assert.Nil(t, cached)
}

func TestRateLimiter(t *testing.T) {
	// 每分钟 60 个令牌 = 每秒 1 个
	limiter := NewRateLimiter(60)

	// 初始应该有 60 个令牌
	assert.GreaterOrEqual(t, limiter.Remaining(), 59)

	// 消耗一个令牌
	allowed := limiter.Allow()
	assert.True(t, allowed)

	// 快速消耗所有令牌
	limiter2 := NewRateLimiter(5)
	time.Sleep(100 * time.Millisecond) // 等待令牌补充

	for i := 0; i < 5; i++ {
		assert.True(t, limiter2.Allow())
	}

	// 第 6 次应该被限制
	assert.False(t, limiter2.Allow())
}

func TestAggregateResults_Empty(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := NewThreatIntelClient(DefaultThreatIntelConfig(), logger)

	result := client.aggregateResults([]*ThreatIntelResult{})

	assert.NotNil(t, result)
	assert.False(t, result.IsMalicious)
	assert.Equal(t, 0, result.RiskScore)
	assert.Equal(t, 0.0, result.Confidence)
	assert.Empty(t, result.Categories)
}

func TestAggregateResults_Single(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := NewThreatIntelClient(DefaultThreatIntelConfig(), logger)

	input := &ThreatIntelResult{
		IP:          "8.8.8.8",
		IsMalicious: true,
		RiskScore:   80,
		Confidence:  90.0,
		Categories:  []string{"malicious"},
		Provider:    "test",
	}

	result := client.aggregateResults([]*ThreatIntelResult{input})

	assert.Equal(t, "8.8.8.8", result.IP)
	assert.True(t, result.IsMalicious)
	assert.Equal(t, 80, result.RiskScore)
	assert.Equal(t, "test", result.Provider)
}

func TestAggregateResults_Multiple(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := NewThreatIntelClient(DefaultThreatIntelConfig(), logger)

	// 模拟两个提供者的结果
	input1 := &ThreatIntelResult{
		IP:          "8.8.8.8",
		IsMalicious: true,
		RiskScore:   80,
		Confidence:  90.0,
		Categories:  []string{"malicious", "scanner"},
		Provider:    "virustotal",
	}

	input2 := &ThreatIntelResult{
		IP:          "8.8.8.8",
		IsMalicious: false,
		RiskScore:   30,
		Confidence:  50.0,
		Categories:  []string{"scanner"},
		Provider:    "abuseipdb",
	}

	result := client.aggregateResults([]*ThreatIntelResult{input1, input2})

	assert.True(t, result.IsMalicious) // 只要有一个标记为恶意就是恶意
	assert.Greater(t, result.RiskScore, 30)
	assert.Less(t, result.RiskScore, 80)
	assert.Len(t, result.Categories, 2) // malicious 和 scanner
}

func TestGetProviders(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// 不启用任何提供者
	client := NewThreatIntelClient(DefaultThreatIntelConfig(), logger)
	providers := client.GetProviders()
	assert.Empty(t, providers)
}
