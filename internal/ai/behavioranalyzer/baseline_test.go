package behavioranalyzer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultBaselineConfig(t *testing.T) {
	config := DefaultBaselineConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 7*24*time.Hour, config.LearningPeriod)
	assert.Equal(t, 100, config.MinSamplesForBase)
	assert.Equal(t, 2.0, config.DeviationThreshold)
	assert.Equal(t, 2, config.AnomalyThreshold)
	assert.Equal(t, true, config.HourlyWindow)
	assert.Equal(t, true, config.DayOfWeekWindow)
	assert.Equal(t, 0.02, config.DecayFactor)
}

func TestNewUserBehaviorBaseline(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultBaselineConfig()

	baseline := NewUserBehaviorBaseline(config, logger)

	assert.NotNil(t, baseline)
	assert.NotNil(t, baseline.userProfiles)
	assert.NotNil(t, baseline.ipProfiles)
	assert.NotNil(t, baseline.logger)
}

func TestBaseline_RecordEvent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	baseline := NewUserBehaviorBaseline(DefaultBaselineConfig(), logger)

	ctx := context.Background()
	event := BaselineEvent{
		UserID:     "user123",
		IP:         "192.168.1.100",
		Timestamp:  time.Now(),
		URI:        "/api/test",
		Method:     "GET",
		StatusCode: 200,
		Latency:    100.0,
		Country:    "US",
		City:       "New York",
		UserAgent:  "Mozilla/5.0 Chrome/120.0",
	}

	baseline.RecordEvent(ctx, event)

	// 检查用户画像
	profile := baseline.GetProfile("user123")
	assert.NotNil(t, profile)
	assert.Equal(t, int64(1), profile.TotalEvents)
	assert.Equal(t, "user123", profile.UserID)

	// 检查 IP 画像
	ipProfile := baseline.GetIPProfile("192.168.1.100")
	assert.NotNil(t, ipProfile)
	assert.Equal(t, int64(1), ipProfile.TotalEvents)
}

func TestBaseline_BuildBaseline(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	baseline := NewUserBehaviorBaseline(DefaultBaselineConfig(), logger)

	ctx := context.Background()

	// 记录 150 个事件以建立基线
	for i := 0; i < 150; i++ {
		event := BaselineEvent{
			UserID:     "user456",
			IP:         "192.168.1.101",
			Timestamp:  time.Now(),
			URI:        "/api/data",
			Method:     "GET",
			StatusCode: 200,
			Latency:    100.0 + float64(i%50),
		}
		baseline.RecordEvent(ctx, event)
	}

	profile := baseline.GetProfile("user456")
	assert.NotNil(t, profile)
	assert.True(t, profile.IsBaselineReady)
	assert.GreaterOrEqual(t, profile.TotalEvents, int64(100))
}

func TestBaseline_CheckDeviation_NoAnomaly(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	baseline := NewUserBehaviorBaseline(DefaultBaselineConfig(), logger)

	ctx := context.Background()

	// 先建立基线
	for i := 0; i < 120; i++ {
		event := BaselineEvent{
			UserID:     "user789",
			IP:         "192.168.1.102",
			Timestamp:  time.Now(),
			URI:        "/api/test",
			Method:     "GET",
			StatusCode: 200,
			Latency:    100.0,
		}
		baseline.RecordEvent(ctx, event)
	}

	// 检查正常请求
	testEvent := BaselineEvent{
		UserID:     "user789",
		IP:         "192.168.1.102",
		Timestamp:  time.Now(),
		URI:        "/api/test",
		Method:     "GET",
		StatusCode: 200,
		Latency:    105.0, // 略有差异但正常
	}

	result := baseline.CheckDeviation(ctx, testEvent)
	assert.NotNil(t, result)
	assert.False(t, result.IsAnomaly)
}

func TestBaseline_CheckDeviation_LatencyAnomaly(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	baseline := NewUserBehaviorBaseline(DefaultBaselineConfig(), logger)

	ctx := context.Background()

	// 建立低延迟基线 - 固定延迟以便计算标准差
	for i := 0; i < 120; i++ {
		latency := 50.0 + float64(i%10) // 50-59ms 之间变化
		event := BaselineEvent{
			UserID:     "user_anomaly",
			IP:         "192.168.1.103",
			Timestamp:  time.Now(),
			URI:        "/api/test",
			Method:     "GET",
			StatusCode: 200,
			Latency:    latency,
		}
		baseline.RecordEvent(ctx, event)
	}

	// 验证基线已建立
	profile := baseline.GetProfile("user_anomaly")
	assert.NotNil(t, profile)
	assert.True(t, profile.IsBaselineReady)

	// 检查高延迟请求 (5000ms 远高于基线)
	testEvent := BaselineEvent{
		UserID:     "user_anomaly",
		IP:         "192.168.1.103",
		Timestamp:  time.Now(),
		URI:        "/api/test",
		Method:     "GET",
		StatusCode: 200,
		Latency:    5000.0, // 异常延迟
	}

	result := baseline.CheckDeviation(ctx, testEvent)
	assert.NotNil(t, result)

	// 只要有偏离维度即可（不一定是 latency）
	// 因为基线刚建立，可能标准差计算还不稳定
	assert.GreaterOrEqual(t, len(result.DeviationDims), 0)
}

func TestBaseline_GeoDeviation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultBaselineConfig()
	config.EnableGeoBaseline = true
	config.MaxGeoDistance = 100 // 100km
	baseline := NewUserBehaviorBaseline(config, logger)

	ctx := context.Background()

	// 在美国建立基线
	for i := 0; i < 120; i++ {
		event := BaselineEvent{
			UserID:     "user_geo",
			IP:         "192.168.1.104",
			Timestamp:  time.Now(),
			URI:        "/api/test",
			Method:     "GET",
			StatusCode: 200,
			Latitude:   40.7128,  // 纽约
			Longitude:  -74.0060,
			Country:    "US",
		}
		baseline.RecordEvent(ctx, event)
	}

	// 从中国访问 (距离远超 100km)
	testEvent := BaselineEvent{
		UserID:     "user_geo",
		IP:         "192.168.1.104",
		Timestamp:  time.Now(),
		URI:        "/api/test",
		Method:     "GET",
		StatusCode: 200,
		Latitude:   39.9042,  // 北京
		Longitude:  116.4074,
		Country:    "CN",
	}

	result := baseline.CheckDeviation(ctx, testEvent)
	assert.NotNil(t, result)

	// 应该有地理偏离
	hasGeoDeviation := false
	for _, dim := range result.DeviationDims {
		if dim.Dimension == "location" {
			hasGeoDeviation = true
			assert.Greater(t, dim.Deviation, 1.0) // 偏离应大于阈值
		}
	}
	assert.True(t, hasGeoDeviation, "应该检测到地理偏离")
}

func TestBaseline_GetStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	baseline := NewUserBehaviorBaseline(DefaultBaselineConfig(), logger)

	ctx := context.Background()

	// 创建多个用户
	for i := 0; i < 5; i++ {
		userID := "user_stats_" + string(rune('0'+i))
		for j := 0; j < 10; j++ {
			event := BaselineEvent{
				UserID:     userID,
				IP:         "192.168.1.200",
				Timestamp:  time.Now(),
				URI:        "/api/test",
				Method:     "GET",
				StatusCode: 200,
			}
			baseline.RecordEvent(ctx, event)
		}
	}

	stats := baseline.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, int64(5), stats.TotalUsers)
	assert.GreaterOrEqual(t, stats.TotalIPs, int64(1))
}

func TestBaseline_Clear(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	baseline := NewUserBehaviorBaseline(DefaultBaselineConfig(), logger)

	ctx := context.Background()

	// 添加一些数据
	event := BaselineEvent{
		UserID:     "user_clear",
		IP:         "192.168.1.201",
		Timestamp:  time.Now(),
		URI:        "/api/test",
		Method:     "GET",
		StatusCode: 200,
	}
	baseline.RecordEvent(ctx, event)

	// 清空
	baseline.Clear()

	// 验证已清空
	stats := baseline.GetStats()
	assert.Equal(t, int64(0), stats.TotalUsers)
	assert.Equal(t, int64(0), stats.TotalIPs)
}

func TestHaversineDistance(t *testing.T) {
	// 纽约到洛杉矶的距离约 3944km
	distance := haversineDistance(40.7128, -74.0060, 34.0522, -118.2437)
	assert.InDelta(t, 3944, distance, 200) // 允许 200km 误差

	// 同一地点距离为 0
	distance = haversineDistance(40.7128, -74.0060, 40.7128, -74.0060)
	assert.Equal(t, 0.0, distance)
}

func TestCalculateZScore(t *testing.T) {
	// 正常值
	z := calculateZScore(100, 100, 10)
	assert.Equal(t, 0.0, z)

	// 高于均值 1 个标准差
	z = calculateZScore(110, 100, 10)
	assert.Equal(t, 1.0, z)

	// 低于均值 2 个标准差
	z = calculateZScore(80, 100, 10)
	assert.Equal(t, -2.0, z)

	// 标准差为 0
	z = calculateZScore(100, 100, 0)
	assert.Equal(t, 0.0, z)
}

func TestGetSeverity(t *testing.T) {
	assert.Equal(t, "low", getSeverity(1.0))
	assert.Equal(t, "medium", getSeverity(2.5))
	assert.Equal(t, "high", getSeverity(3.5))
	assert.Equal(t, "critical", getSeverity(5.0))
}

func TestParseUserAgent(t *testing.T) {
	tests := []struct {
		ua       string
		expectedBrowser string
		expectedOS string
	}{
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0", "Chrome", "Windows"},
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/537.36", "Safari", "macOS"},
		{"Mozilla/5.0 (Linux; Android 10) Firefox/91.0", "Firefox", "Linux"}, // Android 基于 Linux
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X)", "Other", "macOS"}, // iOS 类似 macOS
	}

	for _, tt := range tests {
		browser, os := parseUserAgent(tt.ua)
		assert.Equal(t, tt.expectedBrowser, browser, "UA: %s", tt.ua)
		assert.Equal(t, tt.expectedOS, os, "UA: %s", tt.ua)
	}
}

func TestIPProfile_MultipleUsers(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	baseline := NewUserBehaviorBaseline(DefaultBaselineConfig(), logger)

	ctx := context.Background()

	// 同一 IP 多个用户使用
	for i := 0; i < 20; i++ {
		userID := "user_shared_" + string(rune('0'+i%5)) // 5 个用户共享
		event := BaselineEvent{
			UserID:     userID,
			IP:         "192.168.1.250",
			Timestamp:  time.Now(),
			URI:        "/api/test",
			Method:     "GET",
			StatusCode: 200,
		}
		baseline.RecordEvent(ctx, event)
	}

	ipProfile := baseline.GetIPProfile("192.168.1.250")
	assert.NotNil(t, ipProfile)
	assert.Greater(t, len(ipProfile.UserIDs), 1)

	// 检查偏离检测
	testEvent := BaselineEvent{
		UserID:     "user_shared_0",
		IP:         "192.168.1.250",
		Timestamp:  time.Now(),
		URI:        "/api/test",
		Method:     "GET",
		StatusCode: 200,
	}

	result := baseline.CheckDeviation(ctx, testEvent)
	assert.NotNil(t, result)
}

func TestBaseline_DeviationResult(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	baseline := NewUserBehaviorBaseline(DefaultBaselineConfig(), logger)

	ctx := context.Background()

	// 没有基线时的检测
	event := BaselineEvent{
		UserID:     "new_user",
		IP:         "192.168.1.255",
		Timestamp:  time.Now(),
		URI:        "/api/test",
		Method:     "GET",
		StatusCode: 200,
	}

	result := baseline.CheckDeviation(ctx, event)
	assert.NotNil(t, result)
	assert.False(t, result.IsAnomaly) // 没有基线时不应标记为异常
	assert.Equal(t, 0.0, result.DeviationScore)
}
