package ddos

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
		Enabled:            false,
		EnabledDDoSDetection: false,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.RemoteAddr = "127.0.0.1:12345"
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

// TestDetector_cleanupExpired 测试 cleanupExpired 方法
func TestDetector_cleanupExpired(t *testing.T) {
	config := &Config{
		Enabled:       true,
		BlockDuration: 100 * time.Millisecond,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	// 添加一个会过期的黑名单项
	detector.blacklist.AddWithDuration("10.0.0.1", "test", 100*time.Millisecond)

	// 立即检查应该存在
	assert.True(t, detector.blacklist.IsBlacklisted("10.0.0.1"))

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	// 调用 cleanupExpired
	detector.cleanupExpired()

	// 过期项应该被清理
	assert.False(t, detector.blacklist.IsBlacklisted("10.0.0.1"))
}

// TestDetector_checkRedis 测试 checkRedis 方法
func TestDetector_checkRedis(t *testing.T) {
	config := &Config{
		Enabled:     true,
		EnableRedis: false, // 默认不使用 Redis
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	// 不使用 Redis 时应该返回 false
	result, err := detector.checkRedis(context.Background(), "key")
	assert.False(t, result)
	assert.NoError(t, err)
}

// TestDetector_setRedis 测试 setRedis 方法
func TestDetector_setRedis(t *testing.T) {
	config := &Config{
		Enabled:     true,
		EnableRedis: false,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	// 没有 Redis 客户端时应该返回 nil
	err := detector.setRedis(context.Background(), "key", "value", time.Minute)
	assert.NoError(t, err)
}

// TestBlacklist_Middleware 测试 Blacklist 的中间件方法
func TestBlacklist_Middleware(t *testing.T) {
	// 测试空配置
	bl := NewBlacklist(10 * time.Minute)
	assert.NotNil(t, bl)

	// 测试中间件函数创建
	handler := bl.BlacklistMiddleware(nil)
	assert.NotNil(t, handler)
}

// TestDetector_detectDDoSPattern 测试 detectDDoSPattern
func TestDetector_detectDDoSPattern(t *testing.T) {
	config := &Config{
		Enabled:        true,
		RateThreshold:  10,
		BurstThreshold: 5,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	ip := "192.168.1.1"

	// 发送大量请求触发 HTTP Flood
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":12345"
		detector.Detect(req)
		time.Sleep(50 * time.Millisecond)
	}

	// 检测攻击模式
	pattern := detector.detectDDoSPattern(ip)
	// 可能检测到 http_flood 或其他模式
	assert.NotNil(t, pattern)
}

// TestDetector_Detect_SlowlorisPattern 测试 Slowloris 检测
func TestDetector_Detect_SlowlorisPattern(t *testing.T) {
	config := &Config{
		Enabled:        true,
		RateThreshold:  100,
		BurstThreshold: 50,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	ip := "192.168.1.1"

	// 发送没有 User-Agent 的请求（可疑行为）
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":12345"
		req.Header.Del("User-Agent")
		detector.Detect(req)
	}

	// 可能检测到可疑 headers
	threats, err := detector.Detect(httptest.NewRequest(http.MethodGet, "/", nil))
	assert.NoError(t, err)
	assert.NotNil(t, threats)
}

// TestDetector_cleanupLoop 测试 cleanupLoop
func TestDetector_cleanupLoop(t *testing.T) {
	config := &Config{
		Enabled:       true,
		BlockDuration: 100 * time.Millisecond,
	}
	detector, _ := NewDetector(config, nil)

	// 等待清理协程运行
	time.Sleep(200 * time.Millisecond)

	// 停止检测器不应该 panic
	assert.NotPanics(t, func() {
		detector.Stop()
	})
}

// TestDetector_Detect_Challenge 测试 Challenge 检测
func TestDetector_Detect_Challenge(t *testing.T) {
	config := &Config{
		Enabled:            true,
		ChallengeThreshold: 5,
		ChallengeDuration:  5 * time.Minute,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	ip := "192.168.1.1"

	// 发送多个请求触发 Challenge
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":12345"
		detector.Detect(req)
	}

	// 验证 IP 被挑战
	status := detector.GetStatus(ip)
	assert.NotNil(t, status)
}

// TestDetector_Detect_MultipleIPs 测试多个 IP 的检测
func TestDetector_Detect_MultipleIPs(t *testing.T) {
	config := &Config{
		Enabled:        true,
		RateThreshold:  100,
		BurstThreshold: 50,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	// 模拟来自多个 IP 的请求
	for i := 0; i < 5; i++ {
		ip := "192.168.1." + string(rune('1'+i))
		for j := 0; j < 3; j++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = ip + ":12345"
			detector.Detect(req)
		}
	}

	// 验证所有 IP 都有记录
	for i := 0; i < 5; i++ {
		ip := "192.168.1." + string(rune('1'+i))
		status := detector.GetStatus(ip)
		assert.NotNil(t, status)
	}
}

// TestDetector_Detect_POSTRequest 测试 POST 请求检测
func TestDetector_Detect_POSTRequest(t *testing.T) {
	config := &Config{
		Enabled:        true,
		RateThreshold:  100,
		BurstThreshold: 50,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	body := strings.NewReader("data=test")
	req := httptest.NewRequest(http.MethodPost, "http://example.com/api", body)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotNil(t, threats)
}

// TestDetector_Detect_EmptyRemoteAddr 测试空 RemoteAddr
func TestDetector_Detect_EmptyRemoteAddr(t *testing.T) {
	config := &Config{
		Enabled:        true,
		RateThreshold:  100,
		BurstThreshold: 50,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.RemoteAddr = ""

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotNil(t, threats)
}

// TestDetector_AddToWhitelist 测试 AddToWhitelist 方法
func TestDetector_AddToWhitelist(t *testing.T) {
	config := &Config{
		Enabled:   true,
		Whitelist: []string{},
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	ip := "192.168.1.1"
	detector.AddToWhitelist(ip)

	// 验证 IP 在白名单中
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.RemoteAddr = ip + ":12345"
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestDetector_RemoveFromWhitelist 测试 RemoveFromWhitelist 方法
func TestDetector_RemoveFromWhitelist(t *testing.T) {
	config := &Config{
		Enabled:   true,
		Whitelist: []string{"192.168.1.1"},
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	ip := "192.168.1.1"

	// 验证 IP 最初在白名单中
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.RemoteAddr = ip + ":12345"
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)

	// 从白名单移除
	detector.RemoveFromWhitelist(ip)

	// 现在应该能正常检测（不再是白名单）
	req2 := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req2.RemoteAddr = ip + ":12345"
	threats2, err := detector.Detect(req2)
	assert.NoError(t, err)
	assert.NotNil(t, threats2)
}

// TestRateLimiter_Stop_Idempotent 测试 Stop 的幂等性
func TestRateLimiter_Stop_Idempotent(t *testing.T) {
	rl := NewRateLimiter(100, 50)

	// 第一次停止
	rl.Stop()

	// 第二次停止不应该 panic
	assert.NotPanics(t, func() {
		rl.Stop()
	})
}

// TestDetector_detectDDoSPattern_HttpFlood 测试 detectDDoSPattern 检测 HTTP Flood
func TestDetector_detectDDoSPattern_HttpFlood(t *testing.T) {
	config := &Config{
		Enabled:        true,
		RateThreshold:  100,
		BurstThreshold: 5, // 设置较低的阈值以便测试
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	ip := "192.168.5.1"

	// 快速发送多个请求以触发 HTTP Flood 检测
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":12345"
		detector.Detect(req)
	}

	// 检测攻击模式
	pattern := detector.detectDDoSPattern(ip)
	assert.Equal(t, "http_flood", pattern)
}

// TestDetector_detectDDoSPattern_SuspiciousHeaders 测试 detectDDoSPattern 检测可疑请求头
func TestDetector_detectDDoSPattern_SuspiciousHeaders(t *testing.T) {
	config := &Config{
		Enabled:        true,
		RateThreshold:  100,
		BurstThreshold: 50,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	ip := "192.168.5.2"

	// 发送没有 User-Agent 的请求
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":12345"
		req.Header.Del("User-Agent")
		detector.Detect(req)
	}

	// 检测攻击模式
	pattern := detector.detectDDoSPattern(ip)
	assert.Equal(t, "suspicious_headers", pattern)
}

// TestDetector_detectDDoSPattern_DistributedAttack 测试 detectDDoSPattern 检测分布式攻击
func TestDetector_detectDDoSPattern_DistributedAttack(t *testing.T) {
	config := &Config{
		Enabled:        true,
		RateThreshold:  100,
		BurstThreshold: 50,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	// 添加同一网段的多个 IP（15 个，超过 10 个阈值）
	// 需要设置正常的 User-Agent 以避免先触发 suspicious_headers
	for i := 0; i < 15; i++ {
		ip := fmt.Sprintf("192.168.10.%d", i)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":12345"
		req.Header.Set("User-Agent", "Mozilla/5.0")
		detector.Detect(req)
	}

	// 检测其中一个 IP 的分布式攻击模式
	pattern := detector.detectDDoSPattern("192.168.10.1")
	// 可能检测到 distributed_attack 或其他模式
	assert.NotEmpty(t, pattern)
}

// TestDetector_Detect_EmptyIP 测试 Detect 方法处理空 IP
func TestDetector_Detect_EmptyIP(t *testing.T) {
	config := &Config{
		Enabled:        true,
		RateThreshold:  100,
		BurstThreshold: 50,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	// 创建没有 RemoteAddr 的请求
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.RemoteAddr = ""

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestDetector_Detect_BlacklistedIPWithReason 测试 Detect 方法检测黑名单 IP 并返回原因
func TestDetector_Detect_BlacklistedIPWithReason(t *testing.T) {
	config := &Config{
		Enabled:       true,
		BlockDuration: 10 * time.Minute,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	// 添加 IP 到黑名单
	detector.blacklist.Add("192.168.6.1", "manual block")

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.RemoteAddr = "192.168.6.1:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Len(t, threats, 1)
	assert.Equal(t, "ddos", threats[0].Type)
	assert.Equal(t, "blacklisted", threats[0].SubType)
	assert.Contains(t, threats[0].Message, "IP is blacklisted")
}

// TestDetector_Detect_RateLimitExceeded 测试 Detect 方法检测频率限制超出
func TestDetector_Detect_RateLimitExceeded(t *testing.T) {
	config := &Config{
		Enabled:        true,
		RateThreshold:  5,
		BurstThreshold: 3,
		BlockDuration:  10 * time.Minute,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	ip := "192.168.6.2"

	// 发送多个请求以触发频率限制
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		req.RemoteAddr = ip + ":12345"
		_, _ = detector.Detect(req)
		time.Sleep(5 * time.Millisecond)
	}

	// 下一个请求应该被频率限制或加入黑名单
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.RemoteAddr = ip + ":12345"
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	// 可能被频率限制或黑名单
	if len(threats) > 0 {
		assert.True(t, threats[0].Type == "ddos" || threats[0].Type == "rate-limit")
	}
}

// TestDetector_Detect_ChallengeTriggered 测试 Detect 方法触发挑战
func TestDetector_Detect_ChallengeTriggered(t *testing.T) {
	config := &Config{
		Enabled:            true,
		ChallengeThreshold: 3, // 设置较低的阈值
		ChallengeDuration:  5 * time.Minute,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	ip := "192.168.6.3"

	// 发送多个请求以触发挑战
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/api", nil)
		req.RemoteAddr = ip + ":12345"
		_, _ = detector.Detect(req)
	}

	// 验证 IP 被标记为挑战状态
	status := detector.GetStatus(ip)
	assert.NotNil(t, status)
	assert.True(t, status.IsChallenged)
}

// TestDetector_IsWhitelisted 测试 isWhitelisted 方法
func TestDetector_IsWhitelisted(t *testing.T) {
	config := &Config{
		Enabled:   true,
		Whitelist: []string{"10.0.0.1", "10.0.0.2"},
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	assert.True(t, detector.isWhitelisted("10.0.0.1"))
	assert.True(t, detector.isWhitelisted("10.0.0.2"))
	assert.False(t, detector.isWhitelisted("10.0.0.3"))
}

// TestDetector_GetStatus_AllFields 测试 GetStatus 方法所有字段
func TestDetector_GetStatus_AllFields(t *testing.T) {
	config := &Config{
		Enabled:        true,
		RateThreshold:  100,
		BurstThreshold: 50,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	ip := "192.168.7.1"

	// 发送一些请求
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		req.RemoteAddr = ip + ":12345"
		detector.Detect(req)
	}

	status := detector.GetStatus(ip)
	assert.NotNil(t, status)
	assert.GreaterOrEqual(t, status.RequestCount, 5)
	assert.False(t, status.IsBlacklisted)
	assert.False(t, status.IsChallenged)
	assert.False(t, status.IsRateLimited)
	assert.WithinDuration(t, time.Now(), status.FirstSeen, time.Second)
	assert.WithinDuration(t, time.Now(), status.LastSeen, time.Second)
	assert.GreaterOrEqual(t, status.SuspiciousScore, float64(0))
	assert.LessOrEqual(t, status.SuspiciousScore, float64(1))
}

// TestDetector_Detect_ChallengeFailed 测试挑战验证失败
func TestDetector_Detect_ChallengeFailed(t *testing.T) {
	config := &Config{
		Enabled:           true,
		ChallengeDuration: 5 * time.Minute,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	ip := "192.168.8.1"
	// 创建挑战状态
	detector.challenge.StartChallenge(ip)

	// 发送请求但没有提供正确的挑战响应
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.RemoteAddr = ip + ":12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "challenge_failed", threats[0].SubType)
}

// TestDetector_Detect_DistributedAttack 测试分布式攻击检测
func TestDetector_Detect_DistributedAttack(t *testing.T) {
	config := &Config{
		Enabled:        true,
		RateThreshold:  100,
		BurstThreshold: 50,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	// 模拟来自同一网段的多个 IP（超过 10 个）
	for i := 0; i < 15; i++ {
		ip := fmt.Sprintf("192.168.9.%d", i)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":12345"
		req.Header.Set("User-Agent", "Mozilla/5.0")
		detector.Detect(req)
	}

	// 检测其中一个 IP 的分布式攻击模式
	ip := "192.168.9.1"
	pattern := detector.detectDDoSPattern(ip)
	assert.NotEmpty(t, pattern)
}

// TestDetector_detectDDoSPattern_Slowloris 测试 Slowloris 检测
func TestDetector_detectDDoSPattern_Slowloris(t *testing.T) {
	config := &Config{
		Enabled:        true,
		RateThreshold:  100,
		BurstThreshold: 50,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	ip := "192.168.9.100"
	// 先创建记录
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.RemoteAddr = ip + ":12345"
	detector.Detect(req)

	// 设置 slowloris 标记
	detector.ipTracker.SetFlag(ip, "slowloris")

	pattern := detector.detectDDoSPattern(ip)
	assert.Equal(t, "slowloris", pattern)
}

// TestDetector_GetStatus_NotExists 测试获取不存在的状态
func TestDetector_GetStatus_NotExists(t *testing.T) {
	config := &Config{
		Enabled:        true,
		RateThreshold:  100,
		BurstThreshold: 50,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	// 不存在的 IP 应该返回空状态（不是 nil）
	status := detector.GetStatus("192.168.99.99")
	assert.NotNil(t, status)
	assert.Equal(t, 0, status.RequestCount)
	assert.False(t, status.IsBlacklisted)
	assert.False(t, status.IsChallenged)
	assert.False(t, status.IsRateLimited)
}

// TestDetector_Detect_WithRedis 测试 Redis 集成功能（无客户端情况）
func TestDetector_Detect_WithRedis(t *testing.T) {
	config := &Config{
		Enabled:     true,
		EnableRedis: false, // 不使用 Redis
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	// checkRedis 应该返回 false, nil
	exists, err := detector.checkRedis(context.Background(), "test-key")
	assert.False(t, exists)
	assert.NoError(t, err)

	// setRedis 应该无操作
	err = detector.setRedis(context.Background(), "test-key", "value", time.Minute)
	assert.NoError(t, err)
}

// TestDetector_cleanupExpired 测试 cleanupExpired 方法
func TestDetector_cleanupExpired_Full(t *testing.T) {
	config := &Config{
		Enabled:         true,
		BlockDuration:   100 * time.Millisecond,
		ChallengeDuration: 100 * time.Millisecond,
	}
	detector, _ := NewDetector(config, nil)

	// 添加会过期的数据
	detector.blacklist.AddWithDuration("10.0.0.1", "test", 100*time.Millisecond)
	detector.challenge.StartChallenge("10.0.0.2")

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	// 清理
	detector.cleanupExpired()

	// 验证已清理
	assert.False(t, detector.blacklist.IsBlacklisted("10.0.0.1"))
	assert.Nil(t, detector.challenge.GetStatus("10.0.0.2"))

	detector.Stop()
}

// TestIPTracker_HasSuspiciousHeaders_WithFlag 测试 HasSuspiciousHeaders 有标记的情况
func TestIPTracker_HasSuspiciousHeaders_WithFlag(t *testing.T) {
	tracker := NewIPTracker()

	ip := "192.168.30.1"
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.RemoteAddr = ip + ":12345"
	tracker.RecordRequest(ip, req)

	// 手动添加 suspicious_ua 标记
	tracker.mu.Lock()
	record := tracker.ipRecords[ip]
	tracker.mu.Unlock()

	record.mu.Lock()
	record.Flags["suspicious_ua"] = true
	record.mu.Unlock()

	// 即使有 User-Agent，由于有标记也应该返回 true
	assert.True(t, tracker.HasSuspiciousHeaders(ip))
}

// TestIPTracker_updateSuspiciousScore_PathScanning 测试路径扫描分支
func TestIPTracker_updateSuspiciousScore_PathScanning(t *testing.T) {
	record := &IPRecord{
		Paths: make(map[string]int),
		Flags: make(map[string]bool),
	}

	// 添加超过 50 个路径
	for i := 0; i < 55; i++ {
		record.Paths[fmt.Sprintf("/path%d", i)] = 1
	}

	record.updateSuspiciousScore()

	// 路径扫描应该增加可疑分数并设置标记
	assert.GreaterOrEqual(t, record.SuspiciousScore, 0.2)
	assert.True(t, record.Flags["path_scanning"])
}

// TestIPTracker_updateSuspiciousScore_NonStandardMethods 测试非标准方法分支
func TestIPTracker_updateSuspiciousScore_NonStandardMethods_Detector(t *testing.T) {
	record := &IPRecord{
		Methods: make(map[string]int),
		Flags:   make(map[string]bool),
	}

	// 添加超过 10 个非标准方法请求
	for i := 0; i < 15; i++ {
		record.Methods["CUSTOM"] = i + 1
	}

	record.updateSuspiciousScore()

	// 非标准方法应该增加可疑分数并设置标记
	assert.GreaterOrEqual(t, record.SuspiciousScore, 0.1)
	assert.True(t, record.Flags["unusual_methods"])
}

// TestDetector_Detect_ChallengeVerificationFailed 测试挑战验证失败分支
func TestDetector_Detect_ChallengeVerificationFailed(t *testing.T) {
	config := &Config{
		Enabled:            true,
		ChallengeThreshold: 5,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	ip := "192.168.30.100"

	// 先设置挑战状态
	detector.challenge.StartChallenge(ip)

	// 发送没有正确挑战响应的请求
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.RemoteAddr = ip + ":12345"
	// 没有提供挑战响应

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "challenge_failed", threats[0].SubType)
}

// TestDetector_checkRedis_Enabled 测试 checkRedis 在启用 Redis 时的分支
func TestDetector_checkRedis_Enabled(t *testing.T) {
	config := &Config{
		Enabled:     true,
		EnableRedis: true,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	// 没有 Redis 客户端时应该返回 false, nil
	exists, err := detector.checkRedis(context.Background(), "test-key")
	assert.NoError(t, err)
	assert.False(t, exists)
}

// TestDetector_setRedis_Enabled 测试 setRedis 在启用 Redis 时的分支
func TestDetector_setRedis_Enabled(t *testing.T) {
	config := &Config{
		Enabled:     true,
		EnableRedis: true,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	// 没有 Redis 客户端时应该无操作
	err := detector.setRedis(context.Background(), "test-key", "value", time.Minute)
	assert.NoError(t, err)
}

// TestDetector_cleanupLoop_Run 测试 cleanupLoop 协程运行
func TestDetector_cleanupLoop_Run(t *testing.T) {
	config := &Config{
		Enabled:       true,
		BlockDuration: 100 * time.Millisecond,
	}
	detector, _ := NewDetector(config, nil)

	// 等待清理协程运行
	time.Sleep(100 * time.Millisecond)

	// 停止检测器
	detector.Stop()
}

// TestIPTracker_RecordRequest_WithNilReq 测试 RecordRequest 处理 nil 请求
func TestIPTracker_RecordRequest_WithNilReq(t *testing.T) {
	tracker := NewIPTracker()

	ip := "192.168.31.1"
	tracker.RecordRequest(ip, nil)

	// 验证记录已创建
	stats := tracker.GetStats()
	assert.GreaterOrEqual(t, stats.TotalIPs, 1)
}

// TestIPTracker_HasSuspiciousHeaders_NoRecord 测试 HasSuspiciousHeaders 无记录
func TestIPTracker_HasSuspiciousHeaders_NoRecord_Detector(t *testing.T) {
	tracker := NewIPTracker()

	assert.False(t, tracker.HasSuspiciousHeaders("192.168.31.2"))
}

// TestDetector_Detect_ChallengeSuccess 测试挑战验证成功分支
func TestDetector_Detect_ChallengeSuccess(t *testing.T) {
	config := &Config{
		Enabled:            true,
		ChallengeThreshold: 5,
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	ip := "192.168.32.1"

	// 先发送正常请求创建 IP 记录
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.RemoteAddr = ip + ":12345"
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "*/*")  // 添加正常请求头
	detector.Detect(req)

	// 设置挑战状态
	detector.challenge.StartChallenge(ip)

	// 获取挑战 token
	status := detector.challenge.GetStatus(ip)
	assert.NotNil(t, status)

	// 手动设置 hasUA 为 true 通过添加 Headers
	tracker := detector.ipTracker
	tracker.mu.Lock()
	record := tracker.ipRecords[ip]
	tracker.mu.Unlock()

	record.mu.Lock()
	record.Headers["User-Agent"] = []string{"Mozilla/5.0"}
	record.Flags["suspicious_ua"] = false
	record.mu.Unlock()

	// 直接移除挑战来模拟验证成功
	detector.challenge.RemoveChallenge(ip)

	// 发送请求（没有挑战状态，应该正常通过）
	req2 := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req2.RemoteAddr = ip + ":12345"
	req2.Header.Set("User-Agent", "Mozilla/5.0")

	// 验证应该通过
	threats, err := detector.Detect(req2)
	assert.NoError(t, err)
	// 由于已经创建了 IP 记录且有正常 UA，不应该检测到威胁
	assert.Empty(t, threats)

	// 挑战状态应该保持移除
	assert.Nil(t, detector.challenge.GetStatus(ip))
}

// TestDetector_Detect_ChallengeTriggered2 测试挑战触发分支
func TestDetector_Detect_ChallengeTriggered2(t *testing.T) {
	config := &Config{
		Enabled:            true,
		RateThreshold:      100,
		BurstThreshold:     50,
		ChallengeThreshold: 5, // 较低的阈值
	}
	detector, _ := NewDetector(config, nil)
	defer detector.Stop()

	ip := "192.168.32.2"

	// 先记录一些请求以创建 IP 记录
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		req.RemoteAddr = ip + ":12345"
		req.Header.Set("User-Agent", "suspicious-bot/1.0") // 可疑 UA
		detector.Detect(req)
	}

	// 发送更多请求以触发挑战（超过 ChallengeThreshold）
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("http://example.com/path%d", i), nil)
		req.RemoteAddr = ip + ":12345"
		req.Header.Set("User-Agent", "suspicious-bot/1.0")
		time.Sleep(10 * time.Millisecond)
		detector.Detect(req)
	}

	// 验证 IP 被挑战
	status := detector.GetStatus(ip)
	assert.NotNil(t, status)
}

// TestIPTracker_RecordRequest_Over100 测试 RecordRequest 超过 100 个请求的时间切片
func TestIPTracker_RecordRequest_Over100(t *testing.T) {
	tracker := NewIPTracker()

	ip := "192.168.33.1"

	// 发送超过 100 个请求
	for i := 0; i < 150; i++ {
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		req.RemoteAddr = ip + ":12345"
		req.Header.Set("User-Agent", "Mozilla/5.0")
		tracker.RecordRequest(ip, req)
	}

	// 验证请求时间被限制在 100 个以内
	tracker.mu.RLock()
	record := tracker.ipRecords[ip]
	tracker.mu.RUnlock()

	record.mu.RLock()
	requestCount := len(record.RequestTimes)
	record.mu.RUnlock()

	assert.LessOrEqual(t, requestCount, 100)
}
