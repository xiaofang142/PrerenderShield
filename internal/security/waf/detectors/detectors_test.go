package detectors

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"prerender-shield/internal/security/waf/types"
)

// SQLInjectionDetector Tests

func TestNewSQLInjectionDetector(t *testing.T) {
	detector := NewSQLInjectionDetector()
	assert.NotNil(t, detector)
	assert.NotNil(t, detector.patterns)
}

func TestSQLInjectionDetector_Check_SafeQuery(t *testing.T) {
	detector := NewSQLInjectionDetector()

	req := httptest.NewRequest("GET", "/api/users?id=1", nil)
	result := detector.Check(req)
	assert.True(t, result.Allowed)
	assert.False(t, result.Blocked)
}

func TestSQLInjectionDetector_Check_SelectFrom(t *testing.T) {
	detector := NewSQLInjectionDetector()

	req := httptest.NewRequest("GET", "/api/users?query=SELECT+FROM+users", nil)
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
	assert.Equal(t, "sqli-001", result.RuleID)
}

func TestSQLInjectionDetector_Check_UnionSelect(t *testing.T) {
	detector := NewSQLInjectionDetector()

	req := httptest.NewRequest("GET", "/api/users?id=1+UNION+SELECT+passwords", nil)
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
	assert.NotNil(t, result.Threat)
	assert.Equal(t, "query", result.Threat.Source)
}

func TestSQLInjectionDetector_Check_PathInjection(t *testing.T) {
	detector := NewSQLInjectionDetector()

	req := httptest.NewRequest("GET", "/api/users/1%27+OR+%271%27=%271", nil)
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
}

func TestSQLInjectionDetector_Check_EncodedQuotes(t *testing.T) {
	detector := NewSQLInjectionDetector()

	// URL 编码的引号 %27 = '
	req := httptest.NewRequest("GET", "/api/users?id=1%27%20OR%201=1", nil)
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
}

func TestSQLInjectionDetector_Check_CommentInjection(t *testing.T) {
	detector := NewSQLInjectionDetector()

	req := httptest.NewRequest("GET", "/api/users?id=1--", nil)
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
}

func TestSQLInjectionDetector_Check_InsertInto(t *testing.T) {
	detector := NewSQLInjectionDetector()

	req := httptest.NewRequest("POST", "/api/users", nil)
	req.URL.RawQuery = "data=INSERT INTO users VALUES (1)"
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
}

func TestSQLInjectionDetector_Check_UpdateSet(t *testing.T) {
	detector := NewSQLInjectionDetector()

	req := httptest.NewRequest("PUT", "/api/users", nil)
	req.URL.RawQuery = "data=UPDATE users SET admin=1"
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
}

func TestSQLInjectionDetector_Check_DeleteFrom(t *testing.T) {
	detector := NewSQLInjectionDetector()

	req := httptest.NewRequest("DELETE", "/api/users", nil)
	req.URL.RawQuery = "data=DELETE FROM users WHERE 1=1"
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
}

func TestSQLInjectionDetector_Check_DropTable(t *testing.T) {
	detector := NewSQLInjectionDetector()

	req := httptest.NewRequest("GET", "/api/users?q=DROP+TABLE+users", nil)
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
	assert.NotNil(t, result.Threat)
	assert.Equal(t, "high", result.Threat.Severity)
}

func TestSQLInjectionDetector_Check_EmptyQuery(t *testing.T) {
	detector := NewSQLInjectionDetector()

	req := httptest.NewRequest("GET", "/api/users", nil)
	result := detector.Check(req)
	assert.True(t, result.Allowed)
}

func TestSQLInjectionDetector_Check_OrEquals(t *testing.T) {
	detector := NewSQLInjectionDetector()

	req := httptest.NewRequest("GET", "/api/users?id=1+OR+1=1", nil)
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
}

// XSSDetector Tests

func TestNewXSSDetector(t *testing.T) {
	detector := NewXSSDetector()
	assert.NotNil(t, detector)
	assert.NotNil(t, detector.patterns)
}

func TestXSSDetector_Check_SafeQuery(t *testing.T) {
	detector := NewXSSDetector()

	req := httptest.NewRequest("GET", "/api/search?q=hello", nil)
	result := detector.Check(req)
	assert.True(t, result.Allowed)
	assert.False(t, result.Blocked)
}

func TestXSSDetector_Check_ScriptTag(t *testing.T) {
	detector := NewXSSDetector()

	req := httptest.NewRequest("GET", "/api/search?q=%3Cscript%3Ealert(1)%3C/script%3E", nil)
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
	assert.Equal(t, "xss-001", result.RuleID)
}

func TestXSSDetector_Check_JavascriptProtocol(t *testing.T) {
	detector := NewXSSDetector()

	req := httptest.NewRequest("GET", "/api/link?url=javascript:alert(1)", nil)
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
}

func TestXSSDetector_Check_OnHandler(t *testing.T) {
	detector := NewXSSDetector()

	req := httptest.NewRequest("GET", "/api/search?q=%3Cimg+onerror=alert(1)%3E", nil)
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
}

func TestXSSDetector_Check_Iframe(t *testing.T) {
	detector := NewXSSDetector()

	req := httptest.NewRequest("GET", "/api/content?html=%3Ciframe+src='evil.com'%3E", nil)
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
}

func TestXSSDetector_Check_PathXSS(t *testing.T) {
	detector := NewXSSDetector()

	req := httptest.NewRequest("GET", "/api/page/%3Cscript%3Ealert(1)%3C/script%3E", nil)
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
}

func TestXSSDetector_Check_DocumentCookie(t *testing.T) {
	detector := NewXSSDetector()

	req := httptest.NewRequest("GET", "/api/search?q=%3Cscript%3Efetch('/steal?c='+document.cookie)%3C/script%3E", nil)
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
	assert.NotNil(t, result.Threat)
	assert.Equal(t, types.ThreatXSS, result.Threat.Type)
}

func TestXSSDetector_Check_Eval(t *testing.T) {
	detector := NewXSSDetector()

	req := httptest.NewRequest("GET", "/api/search?q=eval(atob('YWxlcnQoMSk='))", nil)
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
}

func TestXSSDetector_Check_EmptyPath(t *testing.T) {
	detector := NewXSSDetector()

	req := httptest.NewRequest("GET", "/api/search", nil)
	result := detector.Check(req)
	assert.True(t, result.Allowed)
}

// CSRFDetector Tests

func TestNewCSRFDetector(t *testing.T) {
	detector := NewCSRFDetector()
	assert.NotNil(t, detector)
	assert.NotNil(t, detector.allowedOrigins)
}

func TestCSRFDetector_Check_GETRequest(t *testing.T) {
	detector := NewCSRFDetector()

	req := httptest.NewRequest("GET", "/api/data", nil)
	result := detector.Check(req)
	assert.True(t, result.Allowed) // GET 请求应该被允许
}

func TestCSRFDetector_Check_HEADRequest(t *testing.T) {
	detector := NewCSRFDetector()

	req := httptest.NewRequest("HEAD", "/api/data", nil)
	result := detector.Check(req)
	assert.True(t, result.Allowed) // HEAD 请求应该被允许
}

func TestCSRFDetector_Check_POSTNoOrigin(t *testing.T) {
	detector := NewCSRFDetector()

	req := httptest.NewRequest("POST", "/api/submit", nil)
	result := detector.Check(req)
	// 没有 Origin 头时，检查 Referer 和 Token
	// 由于都没有，返回 Challenge 状态
	assert.True(t, result.Allowed)
	assert.True(t, result.Challenge)
}

func TestCSRFDetector_Check_POSTWithToken(t *testing.T) {
	detector := NewCSRFDetector()

	req := httptest.NewRequest("POST", "/api/submit", nil)
	req.Header.Set("X-CSRF-Token", "valid-token")
	result := detector.Check(req)
	assert.True(t, result.Allowed)
}

func TestCSRFDetector_Check_POSTWithFormToken(t *testing.T) {
	detector := NewCSRFDetector()

	req := httptest.NewRequest("POST", "/api/submit?csrf_token=valid-token", nil)
	result := detector.Check(req)
	assert.True(t, result.Allowed)
}

func TestCSRFDetector_Check_SetAllowedOrigins(t *testing.T) {
	detector := NewCSRFDetector()
	detector.SetAllowedOrigins([]string{"https://example.com", "https://trusted.com"})

	assert.Len(t, detector.allowedOrigins, 2)
}

func TestCSRFDetector_Check_InvalidOrigin(t *testing.T) {
	detector := NewCSRFDetector()
	detector.SetAllowedOrigins([]string{"https://example.com"})

	req := httptest.NewRequest("POST", "/api/submit", nil)
	req.Header.Set("Origin", "https://evil.com")
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
}

func TestCSRFDetector_Check_InvalidReferer(t *testing.T) {
	detector := NewCSRFDetector()
	detector.SetAllowedOrigins([]string{"https://example.com"})

	req := httptest.NewRequest("POST", "/api/submit", nil)
	req.Header.Set("Referer", "https://evil.com/page")
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
}

func TestCSRFDetector_Check_ValidOrigin(t *testing.T) {
	detector := NewCSRFDetector()
	detector.SetAllowedOrigins([]string{"https://example.com"})

	req := httptest.NewRequest("POST", "/api/submit", nil)
	req.Header.Set("Origin", "https://example.com")
	result := detector.Check(req)
	assert.True(t, result.Allowed)
}

func TestCSRFDetector_Check_NoAllowedOrigins(t *testing.T) {
	detector := NewCSRFDetector()

	req := httptest.NewRequest("POST", "/api/submit", nil)
	req.Header.Set("Origin", "https://any-origin.com")
	result := detector.Check(req)
	assert.True(t, result.Allowed) // 没有设置允许的源时，应该允许
}

// SensitiveDataDetector Tests

func TestNewSensitiveDataDetector(t *testing.T) {
	detector := NewSensitiveDataDetector()
	assert.NotNil(t, detector)
	assert.NotNil(t, detector.patterns)
}

func TestSensitiveDataDetector_Check_SafeQuery(t *testing.T) {
	detector := NewSensitiveDataDetector()

	req := httptest.NewRequest("GET", "/api/users?name=john", nil)
	result := detector.Check(req)
	assert.True(t, result.Allowed)
	assert.False(t, result.Blocked)
}

func TestSensitiveDataDetector_Check_IDCardNumber(t *testing.T) {
	detector := NewSensitiveDataDetector()

	req := httptest.NewRequest("GET", "/api/search?id=110101199001011234", nil)
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
	assert.Equal(t, "sensitive-001", result.RuleID)
}

func TestSensitiveDataDetector_Check_PhoneNumber(t *testing.T) {
	detector := NewSensitiveDataDetector()

	req := httptest.NewRequest("GET", "/api/search?phone=13800138000", nil)
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
	assert.NotNil(t, result.Threat)
	assert.Equal(t, "query", result.Threat.Source)
}

func TestSensitiveDataDetector_Check_BankCardNumber(t *testing.T) {
	detector := NewSensitiveDataDetector()

	req := httptest.NewRequest("GET", "/api/search?card=6222021234567890123", nil)
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
}

func TestSensitiveDataDetector_Check_PasswordField(t *testing.T) {
	detector := NewSensitiveDataDetector()

	req := httptest.NewRequest("POST", "/api/login", nil)
	req.URL.RawQuery = "password=secret123"
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
}

func TestSensitiveDataDetector_Check_APIKey(t *testing.T) {
	detector := NewSensitiveDataDetector()

	req := httptest.NewRequest("GET", "/api/search?api_key=sk1234567890abcdef", nil)
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
}

func TestSensitiveDataDetector_Check_PrivateIP(t *testing.T) {
	detector := NewSensitiveDataDetector()

	req := httptest.NewRequest("GET", "/api/search?ip=192.168.1.1", nil)
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
}

func TestSensitiveDataDetector_Check_PathSensitive(t *testing.T) {
	detector := NewSensitiveDataDetector()

	req := httptest.NewRequest("GET", "/api/data/13800138000", nil)
	result := detector.Check(req)
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
}

func TestSensitiveDataDetector_Check_EmptyQuery(t *testing.T) {
	detector := NewSensitiveDataDetector()

	req := httptest.NewRequest("GET", "/api/data", nil)
	result := detector.Check(req)
	assert.True(t, result.Allowed)
}

// Utility Tests

func TestDecodeURL_URLDecoding(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello%20world", "hello world"},
		{"test%27quote", "test'quote"},
		{"%3Cscript%3E", "<script>"},
		{"normal", "normal"},
		{"%ZZinvalid", "%ZZinvalid"}, // 无效编码应该返回原字符串
	}

	for _, tt := range tests {
		result := decodeURL(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestCheckResult_Allowed(t *testing.T) {
	result := &types.CheckResult{
		Allowed: true,
		Blocked: false,
	}
	assert.True(t, result.Allowed)
	assert.False(t, result.Blocked)
	assert.Empty(t, result.Reason)
}

func TestCheckResult_Blocked(t *testing.T) {
	result := &types.CheckResult{
		Allowed: false,
		Blocked: true,
		Reason:  "Blocked by rule",
		RuleID:  "test-001",
	}
	assert.False(t, result.Allowed)
	assert.True(t, result.Blocked)
	assert.Equal(t, "Blocked by rule", result.Reason)
	assert.Equal(t, "test-001", result.RuleID)
}

func TestCheckResult_Challenge(t *testing.T) {
	result := &types.CheckResult{
		Allowed:   false,
		Challenge: true,
		Reason:    "Need verification",
	}
	assert.False(t, result.Allowed)
	assert.True(t, result.Challenge)
	assert.Equal(t, "Need verification", result.Reason)
}

func TestThreatTypes(t *testing.T) {
	// 测试 Threat 结构
	threat := &types.Threat{
		Type:     types.ThreatSQLInjection,
		Severity: "high",
		Source:   "query",
	}
	assert.Equal(t, types.ThreatSQLInjection, threat.Type)
	assert.Equal(t, "high", threat.Severity)
	assert.Equal(t, "query", threat.Source)
}
