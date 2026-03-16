package detectors

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewBlacklistDetector(t *testing.T) {
	detector := NewBlacklistDetector()
	assert.NotNil(t, detector)
	assert.NotNil(t, detector.blacklistedIPs)
	assert.NotNil(t, detector.blacklistedUA)
}

func TestBlacklistDetector_AddIP(t *testing.T) {
	detector := NewBlacklistDetector()
	detector.AddIP("192.168.1.100")
	detector.AddIP("10.0.0.1")

	detector.mu.RLock()
	assert.True(t, detector.blacklistedIPs["192.168.1.100"])
	assert.True(t, detector.blacklistedIPs["10.0.0.1"])
	assert.Len(t, detector.blacklistedIPs, 2)
	detector.mu.RUnlock()
}

func TestBlacklistDetector_AddUserAgent(t *testing.T) {
	detector := NewBlacklistDetector()
	detector.AddUserAgent("BadBot")
	detector.AddUserAgent("Scraper")

	detector.mu.RLock()
	assert.True(t, detector.blacklistedUA["BadBot"])
	assert.True(t, detector.blacklistedUA["Scraper"])
	assert.Len(t, detector.blacklistedUA, 2)
	detector.mu.RUnlock()
}

func TestBlacklistDetector_Check_Allowed(t *testing.T) {
	detector := NewBlacklistDetector()

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1"

	result := detector.Check(req)
	assert.True(t, result.Allowed)
	assert.False(t, result.Blocked)
	assert.Empty(t, result.Reason)
}

func TestBlacklistDetector_Check_BlockedIP(t *testing.T) {
	detector := NewBlacklistDetector()
	detector.AddIP("192.168.1.100")

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100"

	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
	assert.Equal(t, "IP blacklisted", result.Reason)
	assert.Equal(t, "blacklist-001", result.RuleID)
	assert.NotNil(t, result.Threat)
	assert.Equal(t, "ip", result.Threat.Source)
	assert.Equal(t, "high", result.Threat.Severity)
}

func TestBlacklistDetector_Check_BlockedUA(t *testing.T) {
	detector := NewBlacklistDetector()
	detector.AddUserAgent("BadBot")

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1"
	req.Header.Set("User-Agent", "Mozilla/5.0 BadBot/1.0")

	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
	assert.Equal(t, "User-Agent blacklisted", result.Reason)
	assert.Equal(t, "blacklist-002", result.RuleID)
	assert.NotNil(t, result.Threat)
	assert.Equal(t, "user_agent", result.Threat.Source)
	assert.Equal(t, "medium", result.Threat.Severity)
}

func TestBlacklistDetector_Check_PartialUA(t *testing.T) {
	detector := NewBlacklistDetector()
	detector.AddUserAgent("BadBot")

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1"
	req.Header.Set("User-Agent", "BadBot")

	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
}

func TestBlacklistDetector_ConcurrentAccess(t *testing.T) {
	detector := NewBlacklistDetector()
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			detector.AddIP("192.168.1." + string(rune('0'+id)))
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.1"
			detector.Check(req)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestNewRateLimitDetector(t *testing.T) {
	detector := NewRateLimitDetector(10)
	assert.NotNil(t, detector)
	assert.Equal(t, 10, detector.limit)
	assert.Equal(t, time.Minute, detector.window)
	assert.NotNil(t, detector.requests)
}

func TestRateLimitDetector_Check_UnderLimit(t *testing.T) {
	detector := NewRateLimitDetector(5)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1"
		result := detector.Check(req)
		assert.True(t, result.Allowed)
		assert.False(t, result.Blocked)
	}
}

func TestRateLimitDetector_Check_AtLimit(t *testing.T) {
	detector := NewRateLimitDetector(3)

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1"
		result := detector.Check(req)
		assert.True(t, result.Allowed)
	}

	// 4th request should be blocked
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1"
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
	assert.Equal(t, "Rate limit exceeded", result.Reason)
	assert.Equal(t, "ratelimit-001", result.RuleID)
	assert.NotNil(t, result.Threat)
	assert.Equal(t, "ip", result.Threat.Source)
	assert.Equal(t, "low", result.Threat.Severity)
}

func TestRateLimitDetector_Check_MultipleIPs(t *testing.T) {
	detector := NewRateLimitDetector(2)

	// IP 1: 2 requests
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1"
		result := detector.Check(req)
		assert.True(t, result.Allowed)
	}

	// IP 2: 2 requests
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.2"
		result := detector.Check(req)
		assert.True(t, result.Allowed)
	}

	// Both IPs should be blocked on next request
	for _, ip := range []string{"192.168.1.1", "192.168.1.2"} {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = ip
		result := detector.Check(req)
		assert.False(t, result.Allowed)
		assert.True(t, result.Blocked)
	}
}

func TestRateLimitDetector_Check_WindowCleanup(t *testing.T) {
	detector := NewRateLimitDetector(5)

	// Make some requests
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1"
		detector.Check(req)
	}

	// Manually set old timestamps to simulate expiration
	detector.mu.Lock()
	oldTime := time.Now().Add(-2 * time.Minute)
	detector.requests["192.168.1.1"] = []time.Time{oldTime, oldTime, oldTime}
	detector.mu.Unlock()

	// Next request should clean up old entries and be allowed
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1"
	result := detector.Check(req)
	assert.True(t, result.Allowed)
}

func TestRateLimitDetector_ConcurrentAccess(t *testing.T) {
	detector := NewRateLimitDetector(100)
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "192.168.1.1"
				detector.Check(req)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestRateLimitDetector_DifferentIPs 测试不同 IP 的独立计数
func TestRateLimitDetector_DifferentIPs(t *testing.T) {
	detector := NewRateLimitDetector(2)

	// IP1 用尽限制
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1"
		assert.True(t, detector.Check(req).Allowed)
	}

	// IP1 的第三个请求应该被阻止
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "10.0.0.1"
	assert.False(t, detector.Check(req1).Allowed)

	// IP2 仍然被允许
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "10.0.0.2"
	assert.True(t, detector.Check(req2).Allowed)
	assert.True(t, detector.Check(req2).Allowed)

	// IP2 的第三个请求也应该被阻止
	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.RemoteAddr = "10.0.0.2"
	assert.False(t, detector.Check(req3).Allowed)
}

// TestRateLimitDetector_ExactlyAtLimit 测试恰好在限制边界
func TestRateLimitDetector_ExactlyAtLimit(t *testing.T) {
	detector := NewRateLimitDetector(1)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1"

	// 第一个请求应该被允许
	assert.True(t, detector.Check(req).Allowed)

	// 第二个请求应该被阻止
	assert.False(t, detector.Check(req).Allowed)
}

// TestBlacklistDetector_EmptyState 测试空状态
func TestBlacklistDetector_EmptyState(t *testing.T) {
	detector := NewBlacklistDetector()

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "any-ip"

	result := detector.Check(req)
	assert.True(t, result.Allowed)
	assert.False(t, result.Blocked)
	assert.Nil(t, result.Threat)
}

// TestBlacklistDetector_MultipleIPs 测试多个 IP 添加
func TestBlacklistDetector_MultipleIPs(t *testing.T) {
	detector := NewBlacklistDetector()
	ips := []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}

	for _, ip := range ips {
		detector.AddIP(ip)
	}

	for _, ip := range ips {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = ip
		result := detector.Check(req)
		assert.False(t, result.Allowed)
	}

	// 不在黑名单的 IP 应该被允许
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "4.4.4.4"
	assert.True(t, detector.Check(req).Allowed)
}

// TestBlacklistDetector_MultipleUAs 测试多个 UA 添加
func TestBlacklistDetector_MultipleUAs(t *testing.T) {
	detector := NewBlacklistDetector()
	uas := []string{"Bot1", "Bot2", "Bot3"}

	for _, ua := range uas {
		detector.AddUserAgent(ua)
	}

	for _, ua := range uas {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("User-Agent", ua)
		result := detector.Check(req)
		assert.False(t, result.Allowed)
	}
}

// TestBlacklistDetector_CaseSensitive 测试大小写敏感
func TestBlacklistDetector_CaseSensitive(t *testing.T) {
	detector := NewBlacklistDetector()
	detector.AddUserAgent("BadBot")

	// 完全匹配
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "BadBot")
	assert.False(t, detector.Check(req).Allowed)

	// 小写不匹配（因为是子字符串匹配，实际上会匹配）
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("User-Agent", "badbot")
	// 由于是 Contains 匹配，小写不会匹配
	assert.True(t, detector.Check(req2).Allowed)
}

// TestRateLimitDetector_LargeLimit 测试大限制值
func TestRateLimitDetector_LargeLimit(t *testing.T) {
	detector := NewRateLimitDetector(1000)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1"

	// 前 100 个请求应该都被允许
	for i := 0; i < 100; i++ {
		assert.True(t, detector.Check(req).Allowed)
	}
}

// TestBlacklistDetector_WildcardUA 测试通配符 UA 匹配
func TestBlacklistDetector_WildcardUA(t *testing.T) {
	detector := NewBlacklistDetector()
	detector.AddUserAgent("Bot")

	// 所有包含 "Bot" 的 UA 都应该被阻止
	testCases := []string{
		"Bot/1.0",
		"Mozilla/5.0 (compatible; Bot)",
		"MyBot/2.0",
		"Bot",
	}

	for _, ua := range testCases {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("User-Agent", ua)
		result := detector.Check(req)
		assert.False(t, result.Allowed, "UA '%s' should be blocked", ua)
	}
}

// TestRateLimitDetector_ZeroLimit 测试零限制
func TestRateLimitDetector_ZeroLimit(t *testing.T) {
	detector := NewRateLimitDetector(0)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1"

	// 第一个请求就应该被阻止
	assert.False(t, detector.Check(req).Allowed)
}
