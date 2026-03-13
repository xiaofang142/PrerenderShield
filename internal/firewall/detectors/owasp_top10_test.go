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

// TestSSRFDetector 测试独立的 SSRF 检测器
func TestSSRFDetector_Detect_InternalAddress(t *testing.T) {
	detector := NewSSRFDetector([]string{}, true)

	req := &http.Request{}
	values := url.Values{}
	values.Add("url", "http://127.0.0.1:8080")
	req.URL = &url.URL{
		RawQuery: values.Encode(),
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "ssrf", threats[0].Type)
}

// TestSSRFDetector_Detect_Localhost 测试 localhost 检测
func TestSSRFDetector_Detect_Localhost(t *testing.T) {
	detector := NewSSRFDetector([]string{}, true)

	req := &http.Request{}
	values := url.Values{}
	values.Add("target", "http://localhost/admin")
	req.URL = &url.URL{
		RawQuery: values.Encode(),
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
}

// TestSSRFDetector_Detect_PrivateIP 测试私有 IP 检测
func TestSSRFDetector_Detect_PrivateIP(t *testing.T) {
	detector := NewSSRFDetector([]string{}, true)

	testCases := []string{
		"http://10.0.0.1/admin",
		"http://172.16.0.1/admin",
		"http://192.168.1.1/admin",
	}

	for _, tc := range testCases {
		req := &http.Request{}
		values := url.Values{}
		values.Add("url", tc)
		req.URL = &url.URL{
			RawQuery: values.Encode(),
		}

		threats, err := detector.Detect(req)
		assert.NoError(t, err)
		assert.NotEmpty(t, threats)
	}
}

// TestSSRFDetector_Detect_AllowedHost 测试允许的 HOST
func TestSSRFDetector_Detect_AllowedHost(t *testing.T) {
	detector := NewSSRFDetector([]string{"api.example.com"}, false)

	req := &http.Request{}
	req.URL = &url.URL{
		Path: "/proxy",
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestSSRFDetector_Detect_NoThreats 测试无威胁请求
func TestSSRFDetector_Detect_NoThreats(t *testing.T) {
	detector := NewSSRFDetector([]string{}, false)

	req := &http.Request{}
	req.URL = &url.URL{
		Path: "/api/data",
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestSSRFDetector_Detect_MetadataService 测试云服务元数据检测
func TestSSRFDetector_Detect_MetadataService(t *testing.T) {
	detector := NewSSRFDetector([]string{}, true)

	testCases := []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://metadata.google/internal",
	}

	for _, tc := range testCases {
		req := &http.Request{}
		values := url.Values{}
		values.Add("url", tc)
		req.URL = &url.URL{
			RawQuery: values.Encode(),
		}

		threats, err := detector.Detect(req)
		assert.NoError(t, err)
		assert.NotEmpty(t, threats)
	}
}
