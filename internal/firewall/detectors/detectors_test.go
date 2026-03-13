package detectors

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"prerender-shield/internal/config"
	"prerender-shield/internal/firewall/types"
	"github.com/stretchr/testify/assert"
)

// EmptyRuleManager 返回空规则切片的 mock
type EmptyRuleManager struct{}

func (m *EmptyRuleManager) GetRulesByCategory(category string) []types.Rule {
	return []types.Rule{}
}

func TestXSSDetector_Detect_NoXSS(t *testing.T) {
	detector := NewXSSDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodGet, "/search?q=hello+world", nil)
	threats, err := detector.Detect(req)

	assert.Nil(t, err)
	assert.Empty(t, threats)
}

func TestXSSDetector_Detect_ScriptTag(t *testing.T) {
	detector := NewXSSDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodGet, "/search?q=<script>alert(1)</script>", nil)
	threats, err := detector.Detect(req)

	assert.Nil(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "xss", threats[0].Type)
	assert.Contains(t, threats[0].SubType, "HTML Tag Injection")
}

func TestXSSDetector_Detect_JavascriptProtocol(t *testing.T) {
	detector := NewXSSDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodGet, "/search?q=javascript:alert(1)", nil)
	threats, err := detector.Detect(req)

	assert.Nil(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "xss", threats[0].Type)
}

func TestXSSDetector_Detect_EventHanlder(t *testing.T) {
	detector := NewXSSDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodGet, "/search?q=onload=alert(1)", nil)
	threats, err := detector.Detect(req)

	assert.Nil(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "JavaScript Event Handler")
}

func TestXSSDetector_Detect_InHeader(t *testing.T) {
	detector := NewXSSDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Custom", "<script>alert(1)</script>")
	threats, err := detector.Detect(req)

	assert.Nil(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "xss", threats[0].Type)
	assert.Contains(t, threats[0].Message, "header")
}

func TestXSSDetector_Name(t *testing.T) {
	detector := NewXSSDetector(&EmptyRuleManager{})
	assert.Equal(t, "xss_detector", detector.Name())
}

func TestMatchesXSSPattern(t *testing.T) {
	tests := []struct {
		value    string
		pattern  string
		expected bool
	}{
		{"<script>alert(1)</script>", "<script", true},
		{"<SCRIPT>alert(1)</SCRIPT>", "<script", true},
		{"javascript:alert(1)", "javascript:", true},
		{"hello world", "<script", false},
		{"normal text", "onload=", false},
	}

	for _, tt := range tests {
		result := matchesXSSPattern(tt.value, tt.pattern)
		assert.Equal(t, tt.expected, result, "Value: %s, Pattern: %s", tt.value, tt.pattern)
	}
}

func TestNewCSRFDetector(t *testing.T) {
	mockManager := &MockRuleManager{
		rules: map[string][]types.Rule{
			"csrf": {
				{ID: "csrf-001", Name: "Test CSRF", Category: "csrf", Pattern: "", Severity: "high"},
			},
		},
	}

	detector := NewCSRFDetector(mockManager)
	assert.NotNil(t, detector)
	assert.Len(t, detector.rules, 1)
}

func TestCSRFDetector_Detect_GETRequest(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	threats, err := detector.Detect(req)

	assert.Nil(t, err)
	assert.Empty(t, threats) // GET 请求不需要 CSRF token
}

func TestCSRFDetector_Detect_POSTWithToken(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-CSRF-Token", "valid-token")
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Host", "example.com")
	threats, err := detector.Detect(req)

	assert.Nil(t, err)
	assert.Empty(t, threats)
}

func TestCSRFDetector_Detect_POSTMissingToken(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Host", "example.com")
	threats, err := detector.Detect(req)

	assert.Nil(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "csrf", threats[0].Type)
	assert.Contains(t, threats[0].SubType, "Missing CSRF Token")
}

func TestCSRFDetector_Detect_InvalidOrigin(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-CSRF-Token", "valid-token")
	req.Header.Set("Origin", "http://evil.com")
	req.Header.Set("Host", "example.com")
	threats, err := detector.Detect(req)

	assert.Nil(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Invalid Origin Header")
}

func TestCSRFDetector_Detect_InvalidReferer(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-CSRF-Token", "valid-token")
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Referer", "http://evil.com/page")
	req.Header.Set("Host", "example.com")
	threats, err := detector.Detect(req)

	assert.Nil(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Invalid Referer Header")
}

func TestCSRFDetector_Name(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})
	assert.Equal(t, "csrf_detector", detector.Name())
}

func TestNewGeoIPDetector(t *testing.T) {
	geoIPConfig := &config.GeoIPConfig{
		Enabled:   true,
		BlockList: []string{"RU", "KP"},
		AllowList: []string{"US", "CN"},
	}

	detector := NewGeoIPDetector(geoIPConfig)
	assert.NotNil(t, detector)
	assert.Equal(t, geoIPConfig, detector.geoIPConfig)
}

func TestGeoIPDetector_Detect_Disabled(t *testing.T) {
	geoIPConfig := &config.GeoIPConfig{
		Enabled: false,
	}

	detector := NewGeoIPDetector(geoIPConfig)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	threats, err := detector.Detect(req)

	assert.Nil(t, err)
	assert.Empty(t, threats)
}

func TestGeoIPDetector_Detect_Localhost(t *testing.T) {
	geoIPConfig := &config.GeoIPConfig{
		Enabled:   true,
		BlockList: []string{"RU"},
		AllowList: []string{"CN"}, // 只允许中国
	}

	detector := NewGeoIPDetector(geoIPConfig)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	threats, err := detector.Detect(req)

	assert.Nil(t, err)
	// 127.0.0.1 被模拟为 CN，在允许列表中
	assert.Empty(t, threats)
}

func TestGeoIPDetector_Detect_BlockedCountry(t *testing.T) {
	geoIPConfig := &config.GeoIPConfig{
		Enabled:   true,
		BlockList: []string{"CN"}, // 阻止中国
	}

	detector := NewGeoIPDetector(geoIPConfig)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	threats, err := detector.Detect(req)

	assert.Nil(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "geoip", threats[0].Type)
	assert.Contains(t, threats[0].SubType, "country_block")
}

func TestGeoIPDetector_Detect_NotInAllowList(t *testing.T) {
	geoIPConfig := &config.GeoIPConfig{
		Enabled:   true,
		AllowList: []string{"US"}, // 只允许美国
	}

	detector := NewGeoIPDetector(geoIPConfig)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	threats, err := detector.Detect(req)

	assert.Nil(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "country_allow")
}

func TestGeoIPDetector_Name(t *testing.T) {
	geoIPConfig := &config.GeoIPConfig{Enabled: true}
	detector := NewGeoIPDetector(geoIPConfig)
	assert.Equal(t, "geoip", detector.Name())
}

func TestGetClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1, 192.168.1.2, 192.168.1.3")

	ip := getClientIP(req)
	assert.Equal(t, "192.168.1.1", ip)
}

func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "192.168.1.100")

	ip := getClientIP(req)
	assert.Equal(t, "192.168.1.100", ip)
}

func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.200:12345"

	ip := getClientIP(req)
	assert.Equal(t, "192.168.1.200", ip)
}

func TestGetClientIP_RemoteAddrIPv6(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "[::1]:12345"

	ip := getClientIP(req)
	assert.Equal(t, "[::1]", ip)
}

// TestRule struct tests
func TestRuleStruct(t *testing.T) {
	rule := types.Rule{
		ID:       "test-001",
		Name:     "Test Rule",
		Category: "test",
		Pattern:  "pattern",
		Severity: "high",
	}

	assert.Equal(t, "test-001", rule.ID)
	assert.Equal(t, "Test Rule", rule.Name)
	assert.Equal(t, "test", rule.Category)
	assert.Equal(t, "high", rule.Severity)
}

func TestThreatStruct(t *testing.T) {
	threat := types.Threat{
		Type:      "xss",
		SubType:   "script_injection",
		Severity:  "high",
		Message:   "XSS detected",
		Parameter: "query",
		Value:     "<script>",
		RuleID:    "xss-001",
		RuleName:  "XSS Rule",
		SourceIP:  "192.168.1.1",
		Details:   map[string]interface{}{"key": "value"},
	}

	assert.Equal(t, "xss", threat.Type)
	assert.Equal(t, "high", threat.Severity)
	assert.Equal(t, "XSS detected", threat.Message)
	assert.Equal(t, "192.168.1.1", threat.SourceIP)
}
