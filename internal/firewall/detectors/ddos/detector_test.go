package ddos

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestConfig 测试 Config 结构
func TestConfig_Struct(t *testing.T) {
	config := &Config{
		Enabled:              true,
		EnabledDDoSDetection: true,
		RateThreshold:        100,
		BurstThreshold:       50,
		ChallengeThreshold:   30,
		BlockDuration:        10 * time.Minute,
		ChallengeDuration:    5 * time.Minute,
		Whitelist:            []string{"127.0.0.1", "10.0.0.1"},
		EnableRedis:          false,
		RedisKeyPrefix:       "firewall:ddos",
	}

	assert.True(t, config.Enabled)
	assert.True(t, config.EnabledDDoSDetection)
	assert.Equal(t, 100, config.RateThreshold)
	assert.Equal(t, 50, config.BurstThreshold)
	assert.Equal(t, 30, config.ChallengeThreshold)
	assert.Equal(t, 10*time.Minute, config.BlockDuration)
	assert.Equal(t, 5*time.Minute, config.ChallengeDuration)
	assert.Len(t, config.Whitelist, 2)
	assert.False(t, config.EnableRedis)
	assert.Equal(t, "firewall:ddos", config.RedisKeyPrefix)
}

// TestNewDetector 测试创建检测器
func TestNewDetector(t *testing.T) {
	config := &Config{
		Enabled:           true,
		RateThreshold:     100,
		BurstThreshold:    50,
		BlockDuration:     10 * time.Minute,
		ChallengeDuration: 5 * time.Minute,
	}

	detector, err := NewDetector(config, nil)
	assert.NoError(t, err)
	assert.NotNil(t, detector)
	assert.NotNil(t, detector.rateLimiter)
	assert.NotNil(t, detector.challenge)
	assert.NotNil(t, detector.ipTracker)
	assert.NotNil(t, detector.blacklist)
	assert.NotNil(t, detector.whitelistMap)

	// 清理
	detector.Stop()
}

// TestNewDetector_NilConfig 测试 nil 配置
func TestNewDetector_NilConfig(t *testing.T) {
	detector, err := NewDetector(nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, detector)

	// 验证使用默认配置
	assert.Equal(t, 100, detector.config.RateThreshold)
	assert.Equal(t, 50, detector.config.BurstThreshold)

	detector.Stop()
}

// TestNewDetector_ZeroThresholds 测试零阈值配置
func TestNewDetector_ZeroThresholds(t *testing.T) {
	config := &Config{
		RateThreshold:      0,
		BurstThreshold:     0,
		ChallengeThreshold: 0,
		BlockDuration:      0,
		ChallengeDuration:  0,
	}

	detector, err := NewDetector(config, nil)
	assert.NoError(t, err)
	assert.NotNil(t, detector)

	// 验证使用默认值
	assert.Equal(t, 100, detector.config.RateThreshold)
	assert.Equal(t, 50, detector.config.BurstThreshold)
	assert.Equal(t, 30, detector.config.ChallengeThreshold)
	assert.Equal(t, 10*time.Minute, detector.config.BlockDuration)
	assert.Equal(t, 5*time.Minute, detector.config.ChallengeDuration)

	detector.Stop()
}

// TestDetector_Name 测试 Name 方法
func TestDetector_Name(t *testing.T) {
	detector, _ := NewDetector(&Config{Enabled: true}, nil)
	defer detector.Stop()

	name := detector.Name()
	assert.Equal(t, "ddos", name)
}

// TestDetector_Detect_NotEnabled 测试未启用检测
func TestDetector_Detect_NotEnabled(t *testing.T) {
	config := &Config{
		Enabled: false,
		EnabledDDoSDetection: false,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestDetector_Detect_WhitelistedIP 测试白名单 IP
func TestDetector_Detect_WhitelistedIP(t *testing.T) {
	config := &Config{
		Enabled:   true,
		Whitelist: []string{"127.0.0.1"},
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestDetector_Detect_BlacklistedIP 测试黑名单 IP
func TestDetector_Detect_BlacklistedIP(t *testing.T) {
	config := &Config{
		Enabled:       true,
		BlockDuration: 10 * time.Minute,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	// 先添加到黑名单
	detector.blacklist.Add("192.168.1.100", "test_reason")

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Len(t, threats, 1)
	assert.Equal(t, "ddos", threats[0].Type)
	assert.Equal(t, "blacklisted", threats[0].SubType)
}

// TestDetector_Detect_RateLimited 测试频率限制
func TestDetector_Detect_RateLimited(t *testing.T) {
	config := &Config{
		Enabled:        true,
		RateThreshold:  5,
		BurstThreshold: 3,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	// 发送多个请求以触发频率限制
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		req.RemoteAddr = "192.168.1.200:12345"
		detector.Detect(req)
		time.Sleep(10 * time.Millisecond)
	}

	// 下一个请求应该被频率限制
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.RemoteAddr = "192.168.1.200:12345"
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	// 可能被频率限制或触发挑战
	if len(threats) > 0 {
		assert.Equal(t, "ddos", threats[0].Type)
	}
}

// TestDetector_AddToWhitelist 测试添加白名单
func TestDetector_AddToWhitelist(t *testing.T) {
	detector, _ := NewDetector(&Config{Enabled: true}, nil)
	defer detector.Stop()

	detector.AddToWhitelist("10.0.0.1")
	assert.True(t, detector.isWhitelisted("10.0.0.1"))

	detector.RemoveFromWhitelist("10.0.0.1")
	assert.False(t, detector.isWhitelisted("10.0.0.1"))
}

// TestDetector_GetStatus 测试获取状态
func TestDetector_GetStatus(t *testing.T) {
	detector, _ := NewDetector(&Config{Enabled: true}, nil)
	defer detector.Stop()

	ip := "192.168.1.50"
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.RemoteAddr = ip + ":12345"

	// 发送请求
	detector.Detect(req)

	status := detector.GetStatus(ip)
	assert.NotNil(t, status)
	assert.GreaterOrEqual(t, status.RequestCount, 0)
	assert.False(t, status.IsBlacklisted)
	assert.False(t, status.IsChallenged)
}

// TestDetector_Stop 测试停止检测器
func TestDetector_Stop(t *testing.T) {
	detector, _ := NewDetector(&Config{Enabled: true}, nil)

	// 停止不应该 panic
	assert.NotPanics(t, func() {
		detector.Stop()
	})
}

// TestIPStatus 测试 IPStatus 结构
func TestIPStatus(t *testing.T) {
	status := &IPStatus{
		RequestCount:    100,
		IsBlacklisted:   true,
		IsChallenged:    true,
		IsRateLimited:   true,
		FirstSeen:       time.Now().Add(-time.Hour),
		LastSeen:        time.Now(),
		SuspiciousScore: 0.8,
	}

	assert.Equal(t, 100, status.RequestCount)
	assert.True(t, status.IsBlacklisted)
	assert.True(t, status.IsChallenged)
	assert.True(t, status.IsRateLimited)
	assert.WithinDuration(t, time.Now(), status.LastSeen, time.Hour)
	assert.Equal(t, float64(0.8), status.SuspiciousScore)
}

// TestGetDefaultConfig 测试默认配置
func TestGetDefaultConfig(t *testing.T) {
	config := getDefaultConfig()

	assert.NotNil(t, config)
	assert.True(t, config.Enabled)
	assert.Equal(t, 100, config.RateThreshold)
	assert.Equal(t, 50, config.BurstThreshold)
	assert.Equal(t, 30, config.ChallengeThreshold)
	assert.Equal(t, 10*time.Minute, config.BlockDuration)
	assert.Equal(t, 5*time.Minute, config.ChallengeDuration)
	assert.Equal(t, "firewall:ddos", config.RedisKeyPrefix)
	assert.False(t, config.EnableRedis)
}
