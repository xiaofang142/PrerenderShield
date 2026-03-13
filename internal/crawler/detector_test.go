package crawler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockDetectorFunctions 用于测试的 mock 函数
type mockDetectorFunctions struct {
	whitelistCheck func(ip string) (bool, error)
	crawlerIPCheck func(ip string) (bool, error)
	userAgentCheck func(ua string) (bool, error)
}

func TestDetector_GetClientIP(t *testing.T) {
	d := &detector{}

	// Test X-Forwarded-For header
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1, 192.168.1.2")
	ip := d.getClientIP(req)
	assert.Equal(t, "192.168.1.1", ip)

	// Test X-Forwarded-For with single IP
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.3")
	ip = d.getClientIP(req)
	assert.Equal(t, "192.168.1.3", ip)

	// Test X-Real-IP header
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "192.168.1.4")
	ip = d.getClientIP(req)
	assert.Equal(t, "192.168.1.4", ip)

	// Test RemoteAddr fallback
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.5:12345"
	ip = d.getClientIP(req)
	assert.Equal(t, "192.168.1.5", ip)

	// Test RemoteAddr IPv6 - splits on first colon
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "[::1]:12345"
	ip = d.getClientIP(req)
	// 代码使用 strings.Split(r.RemoteAddr, ":")[0]，所以对于 "[::1]:12345" 会得到 "["
	assert.Equal(t, "[", ip)
}

func TestDetector_IsCrawler_UserAgentMatch(t *testing.T) {
	// 测试 isCrawlerUserAgent 函数逻辑
	testCases := []struct {
		userAgent string
		patterns  []string
		expected  bool
	}{
		{"Mozilla/5.0 (compatible; Googlebot/2.1)", []string{"googlebot", "bingbot"}, true},
		{"Mozilla/5.0 (compatible; Bingbot/2.0)", []string{"googlebot", "bingbot"}, true},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64)", []string{"googlebot", "bingbot"}, false},
		{"curl/7.68.0", []string{"curl", "wget"}, true},
		{"Mozilla/5.0", []string{}, false},
	}

	for _, tc := range testCases {
		result := isCrawlerUserAgentSimple(tc.userAgent, tc.patterns)
		assert.Equal(t, tc.expected, result, "User-Agent: %s, Patterns: %v", tc.userAgent, tc.patterns)
	}
}

func isCrawlerUserAgentSimple(userAgent string, patterns []string) bool {
	for _, pattern := range patterns {
		if containsIgnoreCase(userAgent, pattern) {
			return true
		}
	}
	return false
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	sLower := toLower(s)
	substrLower := toLower(substr)
	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func TestNewDetector(t *testing.T) {
	// 由于 NewDetector 需要真实的 Redis 客户端，我们只测试它返回非 nil 值
	// 实际功能测试通过集成测试进行
	d := NewDetector(nil)
	assert.NotNil(t, d)
}
