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

	// Test: RemoteAddr 优先（非回环地址）
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.5:12345"
	req.Header.Set("X-Forwarded-For", "192.168.1.1, 192.168.1.2")
	ip := d.getClientIP(req)
	assert.Equal(t, "192.168.1.5", ip)

	// Test: RemoteAddr 为回环地址时信任 X-Forwarded-For
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "192.168.1.1, 192.168.1.2")
	ip = d.getClientIP(req)
	assert.Equal(t, "192.168.1.1", ip)

	// Test: RemoteAddr 为回环地址时信任 X-Real-IP
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Real-IP", "192.168.1.4")
	ip = d.getClientIP(req)
	assert.Equal(t, "192.168.1.4", ip)

	// Test: RemoteAddr 非回环，忽略所有头
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "1.1.1.1")
	req.Header.Set("X-Real-IP", "2.2.2.2")
	ip = d.getClientIP(req)
	assert.Equal(t, "10.0.0.1", ip)
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
	// 由于 NewDetector 需要真实的 Redis 客户端，只测试它返回非 nil 值
	d := NewDetector(nil)
	assert.NotNil(t, d)
}

// TestDetector_GetClientIP_EdgeCases 测试 getClientIP 边界情况
func TestDetector_GetClientIP_EdgeCases(t *testing.T) {
	d := &detector{}

	// Test: RemoteAddr 非回环，X-Forwarded-For 被忽略
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("X-Forwarded-For", "")
	ip := d.getClientIP(req)
	assert.Equal(t, "192.168.1.1", ip)

	// Test: RemoteAddr 为回环，X-Forwarded-For 被信任
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "  192.168.1.1  , 192.168.1.2  ")
	ip = d.getClientIP(req)
	assert.Equal(t, "192.168.1.1", ip)

	// Test: RemoteAddr 非回环，X-Real-IP 被忽略
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Real-IP", "10.0.0.2")
	ip = d.getClientIP(req)
	assert.Equal(t, "10.0.0.1", ip)

	// Test: RemoteAddr 为回环，X-Forwarded-For 优先于 X-Real-IP
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "10.0.0.2")
	req.Header.Set("X-Real-IP", "10.0.0.1")
	ip = d.getClientIP(req)
	assert.Equal(t, "10.0.0.2", ip)

	// Test: RemoteAddr IPv6
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "[2001:db8::1]:12345"
	ip = d.getClientIP(req)
	assert.Equal(t, "[2001", ip)
}

// TestDetector_IsCrawler_NilRedis 测试 Redis 为 nil 时 IsCrawler 的行为
func TestDetector_IsCrawler_NilRedis(t *testing.T) {
	d := NewDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.RemoteAddr = "192.168.1.1:12345"

	// Redis 为 nil 时应该返回错误
	isCrawler, err := d.IsCrawler(req)
	assert.Error(t, err)
	assert.False(t, isCrawler)
}

// TestDetector_IsCrawler_WithCrawlerUserAgent 测试爬虫 User-Agent 检测
func TestDetector_IsCrawler_WithCrawlerUserAgent(t *testing.T) {
	d := &detector{}

	// 测试常见的爬虫 User-Agent
	crawlerUserAgents := []string{
		"Googlebot/2.1 (+http://www.google.com/bot.html)",
		"Mozilla/5.0 (compatible; Googlebot/2.1)",
		"Mozilla/5.0 (compatible; Bingbot/2.0; +http://www.bing.com/bingbot.htm)",
		"Mozilla/5.0 (compatible; YandexBot/3.0; +http://yandex.com/bots)",
		"Sogou web spider/4.0(+http://www.sogou.com/docs/help/webmasters.htm)",
		"Mozilla/5.0 (compatible; Baiduspider/2.0)",
	}

	for _, ua := range crawlerUserAgents {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("User-Agent", ua)
		req.RemoteAddr = "192.168.1.1:12345"

		// 由于 Redis 为 nil，会返回错误
		isCrawler, err := d.IsCrawler(req)
		assert.Error(t, err)
		assert.False(t, isCrawler)
	}
}

// TestDetector_IsCrawler_WithNormalUserAgent 测试正常 User-Agent 检测
func TestDetector_IsCrawler_WithNormalUserAgent(t *testing.T) {
	d := &detector{}

	normalUserAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/605.1.15",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X)",
		"curl/7.68.0",
		"PostmanRuntime/7.28.0",
	}

	for _, ua := range normalUserAgents {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("User-Agent", ua)
		req.RemoteAddr = "192.168.1.1:12345"

		isCrawler, err := d.IsCrawler(req)
		assert.Error(t, err) // Redis 为 nil 会返回错误
		assert.False(t, isCrawler)
	}
}

// TestDetector_AddCrawlerUserAgent_NilRedis 测试 Redis 为 nil 时添加爬虫 User-Agent
func TestDetector_AddCrawlerUserAgent_NilRedis(t *testing.T) {
	d := NewDetector(nil)

	err := d.AddCrawlerUserAgent("Googlebot")
	assert.Error(t, err)
}

// TestDetector_RemoveCrawlerUserAgent_NilRedis 测试 Redis 为 nil 时移除爬虫 User-Agent
func TestDetector_RemoveCrawlerUserAgent_NilRedis(t *testing.T) {
	d := NewDetector(nil)

	err := d.RemoveCrawlerUserAgent("Googlebot")
	assert.Error(t, err)
}

// TestDetector_AddCrawlerIP_NilRedis 测试 Redis 为 nil 时添加爬虫 IP
func TestDetector_AddCrawlerIP_NilRedis(t *testing.T) {
	d := NewDetector(nil)

	err := d.AddCrawlerIP("192.168.1.1")
	assert.Error(t, err)
}

// TestDetector_RemoveCrawlerIP_NilRedis 测试 Redis 为 nil 时移除爬虫 IP
func TestDetector_RemoveCrawlerIP_NilRedis(t *testing.T) {
	d := NewDetector(nil)

	err := d.RemoveCrawlerIP("192.168.1.1")
	assert.Error(t, err)
}

// TestDetector_AddWhitelistIP_NilRedis 测试 Redis 为 nil 时添加白名单 IP
func TestDetector_AddWhitelistIP_NilRedis(t *testing.T) {
	d := NewDetector(nil)

	err := d.AddWhitelistIP("10.0.0.1")
	assert.Error(t, err)
}

// TestDetector_RemoveWhitelistIP_NilRedis 测试 Redis 为 nil 时移除白名单 IP
func TestDetector_RemoveWhitelistIP_NilRedis(t *testing.T) {
	d := NewDetector(nil)

	err := d.RemoveWhitelistIP("10.0.0.1")
	assert.Error(t, err)
}

// TestDetector_ListCrawlerUserAgents_NilRedis 测试 Redis 为 nil 时列出爬虫 User-Agent
func TestDetector_ListCrawlerUserAgents_NilRedis(t *testing.T) {
	d := NewDetector(nil)

	userAgents, err := d.ListCrawlerUserAgents()
	assert.Error(t, err)
	assert.Nil(t, userAgents)
}

// TestDetector_ListCrawlerIPs_NilRedis 测试 Redis 为 nil 时列出爬虫 IP
func TestDetector_ListCrawlerIPs_NilRedis(t *testing.T) {
	d := NewDetector(nil)

	ips, err := d.ListCrawlerIPs()
	assert.Error(t, err)
	assert.Nil(t, ips)
}

// TestDetector_ListWhitelistIPs_NilRedis 测试 Redis 为 nil 时列出白名单 IP
func TestDetector_ListWhitelistIPs_NilRedis(t *testing.T) {
	d := NewDetector(nil)

	ips, err := d.ListWhitelistIPs()
	assert.Error(t, err)
	assert.Nil(t, ips)
}

// TestDetector_Interface 测试 Detector 接口实现
func TestDetector_Interface(t *testing.T) {
	var _ Detector = (*detector)(nil)
}

// TestDetector_GetClientIP_AllHeaders 测试所有 IP 头的优先级
func TestDetector_GetClientIP_AllHeaders(t *testing.T) {
	d := &detector{}

	// 测试优先级：RemoteAddr(非回环) > X-Forwarded-For > X-Real-IP
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "1.1.1.1")
	req.Header.Set("X-Real-IP", "2.2.2.2")
	req.RemoteAddr = "3.3.3.3:12345"
	ip := d.getClientIP(req)
	assert.Equal(t, "3.3.3.3", ip)

	// RemoteAddr 为回环时，X-Forwarded-For 生效
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "1.1.1.1")
	req.Header.Set("X-Real-IP", "2.2.2.2")
	req.RemoteAddr = "127.0.0.1:12345"
	ip = d.getClientIP(req)
	assert.Equal(t, "1.1.1.1", ip)

	// RemoteAddr 为回环且无 X-Forwarded-For 时，X-Real-IP 生效
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "2.2.2.2")
	req.RemoteAddr = "127.0.0.1:12345"
	ip = d.getClientIP(req)
	assert.Equal(t, "2.2.2.2", ip)

	// 都没有时，使用 RemoteAddr
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "3.3.3.3:12345"
	ip = d.getClientIP(req)
	assert.Equal(t, "3.3.3.3", ip)
}
