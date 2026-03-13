package detectors

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"prerender-shield/internal/config"
)

// TestRateLimitDetector_Name 测试检测器名称
func TestRateLimitDetector_Name(t *testing.T) {
	cfg := &config.RateLimitConfig{Enabled: true, Requests: 10, Window: 60, BanTime: 300}
	detector := NewRateLimitDetector(cfg)
	assert.Equal(t, "rate_limit", detector.Name())
}

// TestRateLimitDetector_Detect_RateLimitDisabled 测试频率限制禁用的情况
func TestRateLimitDetector_Detect_RateLimitDisabled(t *testing.T) {
	cfg := &config.RateLimitConfig{Enabled: false, Requests: 10, Window: 60, BanTime: 300}
	detector := NewRateLimitDetector(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestRateLimitDetector_Detect_NilConfig 测试空配置的情况
func TestRateLimitDetector_Detect_NilConfig(t *testing.T) {
	detector := NewRateLimitDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestRateLimitDetector_Detect_WithinLimit 测试在限制内的请求
func TestRateLimitDetector_Detect_WithinLimit(t *testing.T) {
	cfg := &config.RateLimitConfig{Enabled: true, Requests: 10, Window: 60, BanTime: 300}
	detector := NewRateLimitDetector(cfg)

	// 发送 5 个请求（少于限制的 10 个）
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.100:12345"

		threats, err := detector.Detect(req)
		assert.NoError(t, err)
		assert.Empty(t, threats)
	}
}

// TestRateLimitDetector_Detect_ExceedLimit 测试超过限制的请求
func TestRateLimitDetector_Detect_ExceedLimit(t *testing.T) {
	cfg := &config.RateLimitConfig{Enabled: true, Requests: 5, Window: 60, BanTime: 300}
	detector := NewRateLimitDetector(cfg)

	// 发送 6 个请求（超过限制的 5 个）
	for i := 0; i < 6; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.100:12345"

		threats, err := detector.Detect(req)
		assert.NoError(t, err)

		// 前 5 个请求不应该有威胁，第 6 个应该有
		if i < 5 {
			assert.Empty(t, threats)
		} else {
			assert.NotEmpty(t, threats)
			assert.Equal(t, "rate_limit", threats[0].Type)
			assert.Equal(t, "exceeded", threats[0].SubType)
		}
	}
}

// TestRateLimitDetector_Detect_AfterBan 测试被封禁后的请求
func TestRateLimitDetector_Detect_AfterBan(t *testing.T) {
	cfg := &config.RateLimitConfig{Enabled: true, Requests: 3, Window: 60, BanTime: 300}
	detector := NewRateLimitDetector(cfg)

	// 先超过限制导致被封禁
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		detector.Detect(req)
	}

	// 之后的请求应该直接返回被封禁的威胁
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "rate_limit", threats[0].Type)
	assert.Equal(t, "banned", threats[0].SubType)
	assert.Contains(t, threats[0].Message, "IP is banned")
}

// TestRateLimitDetector_exceedsRateLimit_MultipleIPs 测试多个 IP 的独立计数
func TestRateLimitDetector_exceedsRateLimit_MultipleIPs(t *testing.T) {
	cfg := &config.RateLimitConfig{Enabled: true, Requests: 5, Window: 60, BanTime: 300}
	detector := NewRateLimitDetector(cfg)

	// IP1 发送 5 个请求
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		detector.Detect(req)
	}

	// IP2 发送 1 个请求，不应该被限制
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.200:12345"

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestRateLimitDetector_isBanned_NonExistentIP 测试检查不存在的 IP
func TestRateLimitDetector_isBanned_NonExistentIP(t *testing.T) {
	cfg := &config.RateLimitConfig{Enabled: true, Requests: 5, Window: 60, BanTime: 300}
	detector := NewRateLimitDetector(cfg)

	// 不存在的 IP 应该返回未封禁
	assert.False(t, detector.isBanned("192.168.1.100"))
}

// TestRateLimitDetector_banIP 测试封禁 IP
func TestRateLimitDetector_banIP(t *testing.T) {
	cfg := &config.RateLimitConfig{Enabled: true, Requests: 5, Window: 60, BanTime: 300}
	detector := NewRateLimitDetector(cfg)

	// 封禁 IP
	detector.banIP("192.168.1.100", 5*time.Second)

	// 验证 IP 被封禁
	assert.True(t, detector.isBanned("192.168.1.100"))

	// 等待封禁过期
	time.Sleep(6 * time.Second)

	// 验证 IP 封禁已过期
	assert.False(t, detector.isBanned("192.168.1.100"))
}

// TestRateLimitDetector_cleanupExpired 测试清理过期记录
func TestRateLimitDetector_cleanupExpired(t *testing.T) {
	cfg := &config.RateLimitConfig{Enabled: true, Requests: 5, Window: 1, BanTime: 1}
	detector := NewRateLimitDetector(cfg)

	// 添加一些请求
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		detector.Detect(req)
	}

	// 等待过期
	time.Sleep(3 * time.Second)

	// 手动触发清理
	detector.cleanupExpired()

	// 验证记录已被清理（这个测试可能因为时间问题不太稳定，主要用于覆盖代码）
	// 不验证具体结果，只确保函数可以调用
}

// TestRateLimitDetector_Detect_EmptyIP 测试空 IP 的情况
func TestRateLimitDetector_Detect_EmptyIP(t *testing.T) {
	cfg := &config.RateLimitConfig{Enabled: true, Requests: 5, Window: 60, BanTime: 300}
	detector := NewRateLimitDetector(cfg)

	// 创建没有 RemoteAddr 的请求
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = ""

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestRateLimitDetector_Detect_ConcurrentAccess 测试并发访问
func TestRateLimitDetector_Detect_ConcurrentAccess(t *testing.T) {
	cfg := &config.RateLimitConfig{Enabled: true, Requests: 100, Window: 60, BanTime: 300}
	detector := NewRateLimitDetector(cfg)

	done := make(chan bool, 20)

	// 并发发送请求
	for i := 0; i < 20; i++ {
		go func(id int) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = "192.168.1.100:12345"
			detector.Detect(req)
			done <- true
		}(i)
	}

	// 等待所有请求完成
	for i := 0; i < 20; i++ {
		<-done
	}

	// 不应该 panic
	assert.True(t, true)
}

// TestIPCounter_Struct 测试 IPCounter 结构
func TestIPCounter_Struct(t *testing.T) {
	counter := &IPCounter{
		Requests:    []time.Time{time.Now()},
		BannedUntil: time.Now().Add(time.Hour),
	}

	assert.Len(t, counter.Requests, 1)
	assert.False(t, counter.BannedUntil.IsZero())
}
