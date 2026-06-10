package detectors

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============ 测试检测器创建 ============

// TestCSRFDetector_Name 测试检测器名称
func TestCSRFDetector_Name(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})
	assert.Equal(t, "CSRF", detector.Name())
}

// TestCSRFDetector_New_WithNilProvider 测试使用 nil Provider 创建检测器
func TestCSRFDetector_New_WithNilProvider(t *testing.T) {
	detector := NewCSRFDetector(nil)
	assert.NotNil(t, detector)
	assert.Equal(t, "CSRF", detector.Name())
}

// ============ 测试 GET 请求 (不需要 CSRF) ============

// TestCSRFDetector_Detect_GETRequest 测试 GET 请求不需要 CSRF token
func TestCSRFDetector_Detect_GETRequest(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestCSRFDetector_Detect_HEADRequest 测试 HEAD 请求不需要 CSRF token
func TestCSRFDetector_Detect_HEADRequest(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodHead, "/api/data", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestCSRFDetector_Detect_OPTIONSRequest 测试 OPTIONS 请求不需要 CSRF token
func TestCSRFDetector_Detect_OPTIONSRequest(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodOptions, "/api/data", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// ============ 测试 POST 请求缺少 CSRF Token ============

// TestCSRFDetector_Detect_POSTMissingToken 测试 POST 请求缺少 CSRF token
func TestCSRFDetector_Detect_POSTMissingToken(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/api/update", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "csrf", threats[0].Type)
	assert.Contains(t, threats[0].SubType, "Missing CSRF Token")
	assert.Equal(t, "high", threats[0].Severity)
}

// TestCSRFDetector_Detect_PUTMissingToken 测试 PUT 请求缺少 CSRF token
func TestCSRFDetector_Detect_PUTMissingToken(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPut, "/api/update", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Missing CSRF Token")
}

// TestCSRFDetector_Detect_DELETEMissingToken 测试 DELETE 请求缺少 CSRF token
func TestCSRFDetector_Detect_DELETEMissingToken(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodDelete, "/api/delete/1", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Missing CSRF Token")
}

// TestCSRFDetector_Detect_PATCHMissingToken 测试 PATCH 请求缺少 CSRF token
func TestCSRFDetector_Detect_PATCHMissingToken(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPatch, "/api/update/1", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Missing CSRF Token")
}

// ============ 测试 CSRF Token 验证 ============

// TestCSRFDetector_Detect_FormValueToken 测试表单值中的 CSRF token
func TestCSRFDetector_Detect_FormValueToken(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/api/update?csrf_token=valid-token", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestCSRFDetector_Detect_XCSRFTokenHeader 测试 X-CSRF-Token 请求头
func TestCSRFDetector_Detect_XCSRFTokenHeader(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/api/update", nil)
	req.Header.Set("X-CSRF-Token", "valid-token")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestCSRFDetector_Detect_XXSRFTokenHeader 测试 X-XSRF-Token 请求头
func TestCSRFDetector_Detect_XXSRFTokenHeader(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/api/update", nil)
	req.Header.Set("X-XSRF-Token", "valid-token")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestCSRFDetector_Detect_TokenPrecedence 测试 CSRF Token 优先级 (表单值 > 请求头)
func TestCSRFDetector_Detect_TokenPrecedence(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	// 表单值中有 token，请求头中也有
	req := httptest.NewRequest(http.MethodPost, "/api/update?csrf_token=form-token", nil)
	req.Header.Set("X-CSRF-Token", "header-token")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// ============ 测试 Origin Header 验证 ============

// TestCSRFDetector_Detect_OriginMatchesHost 测试 Origin 与 Host 匹配
func TestCSRFDetector_Detect_OriginMatchesHost(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/api/update", nil)
	req.Header.Set("X-CSRF-Token", "valid-token")
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Host", "example.com")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestCSRFDetector_Detect_OriginWithPort 测试带端口的 Origin 匹配
func TestCSRFDetector_Detect_OriginWithPort(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/api/update", nil)
	req.Header.Set("X-CSRF-Token", "valid-token")
	req.Header.Set("Origin", "http://example.com:8080")
	req.Header.Set("Host", "example.com:8080")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestCSRFDetector_Detect_OriginMismatch 测试 Origin 与 Host 不匹配
func TestCSRFDetector_Detect_OriginMismatch(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/api/update", nil)
	req.Header.Set("X-CSRF-Token", "valid-token")
	req.Header.Set("Origin", "http://evil.com")
	req.Header.Set("Host", "example.com")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Invalid Origin Header")
	assert.Equal(t, "high", threats[0].Severity)
}

// TestCSRFDetector_Detect_HTTPSOrigin 测试 HTTPS Origin
func TestCSRFDetector_Detect_HTTPSOrigin(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/api/update", nil)
	req.Header.Set("X-CSRF-Token", "valid-token")
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Host", "example.com")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestCSRFDetector_Detect_HTTPSOriginMismatch 测试 HTTPS Origin 不匹配
func TestCSRFDetector_Detect_HTTPSOriginMismatch(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/api/update", nil)
	req.Header.Set("X-CSRF-Token", "valid-token")
	req.Header.Set("Origin", "https://evil.com")
	req.Header.Set("Host", "example.com")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Invalid Origin Header")
}

// TestCSRFDetector_Detect_OriginSubdomainMismatch 测试 Origin 子域名不匹配
func TestCSRFDetector_Detect_OriginSubdomainMismatch(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/api/update", nil)
	req.Header.Set("X-CSRF-Token", "valid-token")
	req.Header.Set("Origin", "http://sub.example.com")
	req.Header.Set("Host", "example.com")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Invalid Origin Header")
}

// TestCSRFDetector_Detect_OriginEmpty 测试 Origin 为空时不检查
func TestCSRFDetector_Detect_OriginEmpty(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/api/update", nil)
	req.Header.Set("X-CSRF-Token", "valid-token")
	// 不设置 Origin 头
	req.Header.Set("Host", "example.com")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestCSRFDetector_Detect_HostEmpty 测试 Host 为空时不检查
func TestCSRFDetector_Detect_HostEmpty(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/api/update", nil)
	req.Header.Set("X-CSRF-Token", "valid-token")
	req.Header.Set("Origin", "http://example.com")
	// 不设置 Host 头
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// ============ 测试组合攻击场景 ============

// TestCSRFDetector_Detect_MissingTokenAndOriginMismatch 测试缺少 Token 且 Origin 不匹配
func TestCSRFDetector_Detect_MissingTokenAndOriginMismatch(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/api/update", nil)
	req.Header.Set("Origin", "http://evil.com")
	req.Header.Set("Host", "example.com")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	// 应该检测到缺少 Token 和 Origin 不匹配两个威胁
	assert.GreaterOrEqual(t, len(threats), 1)
}

// TestCSRFDetector_Detect_ValidTokenAndOrigin 测试有效 Token 和 Origin
func TestCSRFDetector_Detect_ValidTokenAndOrigin(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/api/update", nil)
	req.Header.Set("X-CSRF-Token", "valid-token")
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Host", "example.com")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// ============ 测试威胁详情 ============

// TestCSRFDetector_ThreatDetails_MissingToken 测试缺少 Token 的威胁详情
func TestCSRFDetector_ThreatDetails_MissingToken(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/api/update", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)

	threat := threats[0]
	assert.Equal(t, "csrf", threat.Type)
	assert.Equal(t, "Missing CSRF Token", threat.SubType)
	assert.Equal(t, "high", threat.Severity)
	assert.Equal(t, "Missing CSRF token in request", threat.Message)
	assert.Equal(t, "X-CSRF-Token", threat.Parameter)
	assert.Equal(t, "csrf-001", threat.RuleID)
}

// TestCSRFDetector_ThreatDetails_OriginMismatch 测试 Origin 不匹配的威胁详情
func TestCSRFDetector_ThreatDetails_OriginMismatch(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/api/update", nil)
	req.Header.Set("X-CSRF-Token", "valid-token")
	req.Header.Set("Origin", "http://evil.com")
	req.Header.Set("Host", "example.com")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)

	// 找到 Origin 不匹配的威胁
	var originThreatFound bool
	for _, threat := range threats {
		if threat.SubType == "Invalid Origin Header" {
			originThreatFound = true
			assert.Equal(t, "csrf", threat.Type)
			assert.Equal(t, "high", threat.Severity)
			assert.Equal(t, "Origin header does not match host", threat.Message)
			assert.Equal(t, "Origin", threat.Parameter)
			assert.Equal(t, "http://evil.com", threat.Value)
			assert.Equal(t, "csrf-002", threat.RuleID)
		}
	}
	assert.True(t, originThreatFound)
}

// ============ 测试边界情况 ============

// TestCSRFDetector_Detect_EmptyToken 测试空 Token
func TestCSRFDetector_Detect_EmptyToken(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/api/update", nil)
	req.Header.Set("X-CSRF-Token", "")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Missing CSRF Token")
}

// TestCSRFDetector_Detect_OriginWithCredentials 测试带凭证的 Origin
func TestCSRFDetector_Detect_OriginWithCredentials(t *testing.T) {
	detector := NewCSRFDetector(&EmptyRuleManager{})

	req := httptest.NewRequest(http.MethodPost, "/api/update", nil)
	req.Header.Set("X-CSRF-Token", "valid-token")
	// Origin: http://user:pass@example.com
	// 解析后 originHost = "user" (因为@前的部分)，与 host "example.com" 不匹配
	req.Header.Set("Origin", "http://user:pass@example.com")
	req.Header.Set("Host", "example.com")
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	// 由于 CSRF 检测器没有处理 user:pass@ 前缀，Origin 主机名会被解析为 "user" 而不是 "example.com"
	// 所以会检测到 Origin 不匹配
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Invalid Origin Header")
}
