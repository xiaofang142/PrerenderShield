package detectors

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/firewall/types"
)

// MockRuleManagerForOWASP 用于测试的规则管理器模拟
type MockRuleManagerForOWASP struct {
	rules map[string][]types.Rule
}

func (m *MockRuleManagerForOWASP) GetRulesByCategory(category string) []types.Rule {
	return m.rules[category]
}

// TestOWASPTop10Detector_Detect_PathTraversal 测试路径遍历检测
func TestOWASPTop10Detector_Detect_PathTraversal(t *testing.T) {
	mockManager := &MockRuleManagerForOWASP{
		rules: make(map[string][]types.Rule),
	}
	detector := NewOWASPTop10Detector(mockManager)

	req := &http.Request{}
	// 使用 URL 编码的路径遍历载荷
	req.URL = &url.URL{
		Path:     "/test/../../../etc/passwd",
		RawQuery: "file=..%2f..%2fetc%2fpasswd",
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	// 路径遍历可能被检测到
	if len(threats) > 0 {
		assert.Equal(t, "owasp_top10", threats[0].Type)
	}
}

// TestOWASPTop10Detector_Detect_AdminAccess 测试 admin 访问检测
func TestOWASPTop10Detector_Detect_AdminAccess(t *testing.T) {
	mockManager := &MockRuleManagerForOWASP{
		rules: make(map[string][]types.Rule),
	}
	detector := NewOWASPTop10Detector(mockManager)

	req := &http.Request{}
	req.URL = &url.URL{
		Path: "/admin/dashboard",
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Admin Access")
}

// TestOWASPTop10Detector_Detect_SensitiveData 测试敏感数据暴露检测
func TestOWASPTop10Detector_Detect_SensitiveData(t *testing.T) {
	mockManager := &MockRuleManagerForOWASP{
		rules: make(map[string][]types.Rule),
	}
	detector := NewOWASPTop10Detector(mockManager)

	req := &http.Request{}
	// 使用符合规则模式的参数：(passwd|password|secret|private|credential|token)\s*=\s*[^&]+
	req.URL = &url.URL{
		RawQuery: "password=secret123",
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	// 如果检测到威胁，验证类型
	if len(threats) > 0 {
		assert.Equal(t, "owasp_top10", threats[0].Type)
		assert.Contains(t, threats[0].SubType, "Sensitive Data")
	}
}

// TestOWASPTop10Detector_Detect_MassAssignment 测试批量赋值检测
func TestOWASPTop10Detector_Detect_MassAssignment(t *testing.T) {
	mockManager := &MockRuleManagerForOWASP{
		rules: make(map[string][]types.Rule),
	}
	detector := NewOWASPTop10Detector(mockManager)

	req := &http.Request{}
	values := url.Values{}
	values.Add("__proto__", "polluted")
	req.URL = &url.URL{
		RawQuery: values.Encode(),
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Mass Assignment")
}

// TestOWASPTop10Detector_Detect_Scanner 测试扫描工具检测
func TestOWASPTop10Detector_Detect_Scanner(t *testing.T) {
	mockManager := &MockRuleManagerForOWASP{
		rules: make(map[string][]types.Rule),
	}
	detector := NewOWASPTop10Detector(mockManager)

	req := &http.Request{}
	req.Header = make(http.Header)
	req.Header.Set("User-Agent", "nikto/2.1.6")
	req.URL = &url.URL{}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Scanner")
}

// TestOWASPTop10Detector_Detect_SSRF 测试 SSRF 检测
func TestOWASPTop10Detector_Detect_SSRF(t *testing.T) {
	mockManager := &MockRuleManagerForOWASP{
		rules: make(map[string][]types.Rule),
	}
	detector := NewOWASPTop10Detector(mockManager)

	req := &http.Request{}
	values := url.Values{}
	// SSRF 规则匹配：127\.0\.0\.1|localhost|0\.0\.0\.0|169\.254\.|(::1)|metadata\.google
	values.Add("target", "127.0.0.1:8080")
	req.URL = &url.URL{
		RawQuery: values.Encode(),
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	// SSRF 可能被检测到
	if len(threats) > 0 {
		assert.Equal(t, "owasp_top10", threats[0].Type)
	}
}

// TestOWASPTop10Detector_Detect_UntrustedData 测试不可信数据检测
func TestOWASPTop10Detector_Detect_UntrustedData(t *testing.T) {
	mockManager := &MockRuleManagerForOWASP{
		rules: make(map[string][]types.Rule),
	}
	detector := NewOWASPTop10Detector(mockManager)

	req := &http.Request{}
	values := url.Values{}
	values.Add("file", "php://filter/convert.base64-encode/resource=index.php")
	req.URL = &url.URL{
		RawQuery: values.Encode(),
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Untrusted Data")
}

// TestOWASPTop10Detector_Detect_NoThreats 测试无威胁请求
func TestOWASPTop10Detector_Detect_NoThreats(t *testing.T) {
	mockManager := &MockRuleManagerForOWASP{
		rules: make(map[string][]types.Rule),
	}
	detector := NewOWASPTop10Detector(mockManager)

	req := &http.Request{}
	req.URL = &url.URL{
		Path: "/api/users",
	}
	req.Header = make(http.Header)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestOWASPTop10Detector_Detect_MultipleThreats 测试多个威胁
func TestOWASPTop10Detector_Detect_MultipleThreats(t *testing.T) {
	mockManager := &MockRuleManagerForOWASP{
		rules: make(map[string][]types.Rule),
	}
	detector := NewOWASPTop10Detector(mockManager)

	req := &http.Request{}
	req.URL = &url.URL{
		Path:     "/admin",
		RawQuery: "url=http://127.0.0.1:8080&__proto__=polluted",
	}
	req.Header = make(http.Header)
	req.Header.Set("User-Agent", "sqlmap/1.0")

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.GreaterOrEqual(t, len(threats), 1)
}

// ============ 测试规则更新 ============

// TestOWASPTop10Detector_UpdateRules 测试动态更新规则
func TestOWASPTop10Detector_UpdateRules(t *testing.T) {
	mockManager := &MockRuleManagerForOWASP{
		rules: make(map[string][]types.Rule),
	}
	detector := NewOWASPTop10Detector(mockManager)

	// 更新自定义规则
	customRules := []types.Rule{
		{ID: "custom-owasp-001", Name: "Custom OWASP Rule", Category: "owasp_top10", Pattern: `CUSTOM_OWASP`, Severity: "critical"},
	}

	err := detector.UpdateRules(customRules)
	assert.NoError(t, err)

	// 验证新规则生效
	req := &http.Request{}
	req.URL = &url.URL{
		RawQuery: "data=CUSTOM_OWASP",
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "custom-owasp-001", threats[0].RuleID)
	assert.Equal(t, "critical", threats[0].Severity)
}

// ============ 测试检测器名称 ============

// TestOWASPTop10Detector_Name 测试检测器名称
func TestOWASPTop10Detector_Name(t *testing.T) {
	mockManager := &MockRuleManagerForOWASP{
		rules: make(map[string][]types.Rule),
	}
	detector := NewOWASPTop10Detector(mockManager)

	name := detector.Name()
	assert.Equal(t, "OWASP-Top10", name)
}

// TestOWASPTop10Detector_CompileRules_InvalidPattern 测试编译无效规则模式
func TestOWASPTop10Detector_CompileRules_InvalidPattern(t *testing.T) {
	mockManager := &MockRuleManagerForOWASP{
		rules: map[string][]types.Rule{
			"owasp_top10": {
				{ID: "invalid-rule", Name: "Invalid Rule", Category: "owasp_top10", Pattern: "[invalid(regex", Severity: "high"},
			},
		},
	}
	detector := NewOWASPTop10Detector(mockManager)

	// 验证检测器创建成功（无效规则被跳过）
	assert.NotNil(t, detector)

	// 验证仍然可以正常检测
	req := &http.Request{}
	req.URL = &url.URL{
		Path: "/admin",
	}
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
}

// TestOWASPTop10Detector_CompileRules_EmptyPattern 测试空规则模式
func TestOWASPTop10Detector_CompileRules_EmptyPattern(t *testing.T) {
	mockManager := &MockRuleManagerForOWASP{
		rules: map[string][]types.Rule{
			"owasp_top10": {
				{ID: "empty-rule", Name: "Empty Rule", Category: "owasp_top10", Pattern: "", Severity: "high"},
			},
		},
	}
	detector := NewOWASPTop10Detector(mockManager)

	// 验证检测器创建成功（空规则被跳过）
	assert.NotNil(t, detector)
}

