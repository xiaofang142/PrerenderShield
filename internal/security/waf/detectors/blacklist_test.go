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
