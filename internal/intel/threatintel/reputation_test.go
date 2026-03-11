package threatintel

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultIPReputationConfig(t *testing.T) {
	config := DefaultIPReputationConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 0.5, config.ThreatIntelWeight)
	assert.Equal(t, 0.3, config.HistoryWeight)
	assert.Equal(t, 0.2, config.BehaviorWeight)
	assert.Equal(t, 80.0, config.MaliciousThreshold)
	assert.Equal(t, 50.0, config.SuspiciousThreshold)
	assert.Equal(t, 20.0, config.TrustedThreshold)
	assert.Equal(t, 0.01, config.DecayFactor)
	assert.Equal(t, 30, config.MaxHistoryDays)
	assert.Equal(t, 30*time.Minute, config.CacheTTL)
}

func TestNewIPReputationService(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultIPReputationConfig()

	service := NewIPReputationService(config, nil, logger)

	assert.NotNil(t, service)
	assert.NotNil(t, service.cache)
	assert.NotNil(t, service.history)
	assert.NotNil(t, service.logger)
}

func TestIPReputationService_GetReputation_EmptyIP(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := NewIPReputationService(DefaultIPReputationConfig(), nil, logger)

	ctx := context.Background()
	rep, err := service.GetReputation(ctx, "")

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "IP 不能为空")
	assert.Nil(t, rep)
}

func TestIPReputationService_GetReputation_NoThreatIntel(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := NewIPReputationService(DefaultIPReputationConfig(), nil, logger)

	ctx := context.Background()
	rep, err := service.GetReputation(ctx, "8.8.8.8")

	assert.Nil(t, err)
	assert.NotNil(t, rep)
	assert.Equal(t, "8.8.8.8", rep.IP)
	// 没有威胁情报时，评分应该很低 (可信)
	assert.Equal(t, RiskLevelTrusted, rep.RiskLevel)
	assert.GreaterOrEqual(t, rep.Score, 0.0)
	assert.LessOrEqual(t, rep.Score, 20.0) // 可信阈值
}

func TestIPReputationService_DetermineRiskLevel(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := NewIPReputationService(DefaultIPReputationConfig(), nil, logger)

	// 测试各风险等级
	tests := []struct {
		score    float64
		expected RiskLevel
	}{
		{0, RiskLevelTrusted},
		{15, RiskLevelTrusted},
		{25, RiskLevelNormal},
		{50, RiskLevelSuspicious},
		{75, RiskLevelSuspicious},
		{80, RiskLevelMalicious},
		{100, RiskLevelMalicious},
	}

	for _, tt := range tests {
		level := service.determineRiskLevel(tt.score)
		assert.Equal(t, tt.expected, level, "score=%f", tt.score)
	}
}

func TestIPReputationService_CalculateFinalScore(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := NewIPReputationService(DefaultIPReputationConfig(), nil, logger)

	// 测试综合评分计算
	score := service.calculateFinalScore(100, 50, 0)
	expected := 100*0.5 + 50*0.3 + 0*0.2
	assert.Equal(t, expected, score)

	// 测试边界
	assert.Equal(t, 0.0, service.calculateFinalScore(0, 0, 0))
	assert.Equal(t, 100.0, service.calculateFinalScore(100, 100, 100))
}

func TestIPReputationService_CalculateConfidence(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := NewIPReputationService(DefaultIPReputationConfig(), nil, logger)

	// 无证据
	confidence := service.calculateConfidence([]Evidence{})
	assert.Equal(t, 0.0, confidence)

	// 单个证据
	evidence := []Evidence{
		{Source: "test", Type: "threat", Weight: 0.5},
	}
	confidence = service.calculateConfidence(evidence)
	assert.Greater(t, confidence, 0.0)

	// 多个证据
	evidence = append(evidence, Evidence{Source: "test2", Type: "history", Weight: 0.3})
	confidence2 := service.calculateConfidence(evidence)
	assert.Greater(t, confidence2, confidence)
}

func TestIPReputationService_RecordEvent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := NewIPReputationService(DefaultIPReputationConfig(), nil, logger)

	ip := "192.168.1.100"
	event := IPEvent{
		Type:       "request",
		StatusCode: 200,
		Latency:    100.0,
		URI:        "/api/test",
		Method:     "GET",
		Timestamp:  time.Now(),
	}

	service.RecordEvent(ip, event)

	history := service.GetIPHistory(ip)
	assert.NotNil(t, history)
	assert.Equal(t, int64(1), history.TotalRequests)
	assert.Equal(t, ip, history.IP)
}

func TestIPReputationService_RecordErrorEvent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := NewIPReputationService(DefaultIPReputationConfig(), nil, logger)

	ip := "192.168.1.101"

	// 记录多个错误请求
	for i := 0; i < 5; i++ {
		event := IPEvent{
			Type:       "request",
			StatusCode: 500,
			Latency:    1000.0,
			URI:        "/api/error",
			Method:     "GET",
			Timestamp:  time.Now(),
		}
		service.RecordEvent(ip, event)
	}

	history := service.GetIPHistory(ip)
	assert.NotNil(t, history)
	assert.Equal(t, int64(5), history.TotalRequests)
	assert.Equal(t, int64(5), history.ErrorRequests)
}

func TestIPReputationService_RecordThreatEvent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := NewIPReputationService(DefaultIPReputationConfig(), nil, logger)

	ip := "192.168.1.102"

	event := IPEvent{
		Type:       "threat",
		StatusCode: 403,
		Latency:    50.0,
		URI:        "/admin",
		Method:     "POST",
		ThreatType: "sql_injection",
		Timestamp:  time.Now(),
	}

	service.RecordEvent(ip, event)

	history := service.GetIPHistory(ip)
	assert.NotNil(t, history)
	assert.Equal(t, int64(1), history.BlockedCount)
	assert.Len(t, history.ThreatEvents, 1)
	assert.Equal(t, "sql_injection", history.ThreatEvents[0].Details)
}

func TestIPHistoryStore(t *testing.T) {
	store := NewIPHistoryStore()

	ip := "10.0.0.1"
	event := IPEvent{
		Type:       "request",
		StatusCode: 200,
		Latency:    100.0,
		Timestamp:  time.Now(),
	}

	store.Record(ip, event)

	history := store.Get(ip)
	assert.NotNil(t, history)
	assert.Equal(t, int64(1), history.TotalRequests)

	count := store.Count()
	assert.GreaterOrEqual(t, count, 1)
}

func TestIPHistoryStore_GetRecentStats(t *testing.T) {
	store := NewIPHistoryStore()

	ip := "10.0.0.2"

	// 记录多个请求
	for i := 0; i < 10; i++ {
		event := IPEvent{
			Type:       "request",
			StatusCode: 200,
			Latency:    float64(100 + i*10),
			Timestamp:  time.Now(),
		}
		store.Record(ip, event)
	}

	stats := store.GetRecentStats(ip, 5*time.Minute)
	assert.NotNil(t, stats)
	assert.Equal(t, int64(10), stats.TotalRequests)
	assert.Greater(t, stats.RequestsPerMin, 0.0)
}

func TestIPReputationCache(t *testing.T) {
	cache := NewIPReputationCache(1 * time.Hour)

	rep := &IPReputation{
		IP:          "8.8.8.8",
		Score:       50.0,
		RiskLevel:   RiskLevelNormal,
		LastUpdated: time.Now(),
	}

	// 测试 Set 和 Get
	cache.Set("8.8.8.8", rep)
	cached, found := cache.Get("8.8.8.8")
	assert.True(t, found)
	assert.Equal(t, 50.0, cached.Score)
	assert.Equal(t, 1, cache.Size())

	// 测试 Clear
	cache.Clear()
	assert.Equal(t, 0, cache.Size())
}

func TestIPReputationService_GetStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := NewIPReputationService(DefaultIPReputationConfig(), nil, logger)

	// 记录一些事件
	for i := 0; i < 5; i++ {
		event := IPEvent{
			Type:       "request",
			StatusCode: 200,
			Timestamp:  time.Now(),
		}
		service.RecordEvent("192.168.1.1", event)
	}

	stats := service.GetStats()
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats["history_ips"], 1)
	assert.NotNil(t, stats["config"])
}

func TestCalculateHistoryScore(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := NewIPReputationService(DefaultIPReputationConfig(), nil, logger)

	history := &IPHistory{
		TotalRequests: 100,
		ErrorRequests: 20,
		BlockedCount:  5,
		ThreatEvents:  []ThreatEvent{{Type: "threat", Severity: "high"}},
		LastSeen:      time.Now(),
	}

	score := service.calculateHistoryScore(history)
	assert.Greater(t, score, 0.0)
	assert.LessOrEqual(t, score, 100.0)
}

func TestCalculateBehaviorScore(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := NewIPReputationService(DefaultIPReputationConfig(), nil, logger)

	stats := &RequestStats{
		TotalRequests:  1000,
		ErrorRequests:  100,
		RequestsPerMin: 150,
		UniqueURIs:     100,
	}

	score := service.calculateBehaviorScore(stats)
	assert.Greater(t, score, 0.0)
	assert.LessOrEqual(t, score, 100.0)
}
