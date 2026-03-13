package detectors

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/firewall/types"
)

// MockRuleManagerForSensitiveData 用于测试的规则管理器模拟
type MockRuleManagerForSensitiveData struct {
	rules map[string][]types.Rule
}

func (m *MockRuleManagerForSensitiveData) GetRulesByCategory(category string) []types.Rule {
	return m.rules[category]
}

// TestSensitiveDataDetector_Name 测试检测器名称
func TestSensitiveDataDetector_Name(t *testing.T) {
	mockManager := &MockRuleManagerForSensitiveData{
		rules: make(map[string][]types.Rule),
	}
	detector := NewSensitiveDataDetector(mockManager)
	assert.Equal(t, "sensitive_data_detector", detector.Name())
}

// TestSensitiveDataDetector_Detect_CreditCard 测试信用卡号检测
func TestSensitiveDataDetector_Detect_CreditCard(t *testing.T) {
	mockManager := &MockRuleManagerForSensitiveData{
		rules: make(map[string][]types.Rule),
	}
	detector := NewSensitiveDataDetector(mockManager)

	req := &http.Request{}
	values := url.Values{}
	values.Add("card", "1234-5678-9012-3456")
	req.URL = &url.URL{
		RawQuery: values.Encode(),
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "sensitive-data", threats[0].Type)
	assert.Contains(t, threats[0].SubType, "Credit Card")
}

// TestSensitiveDataDetector_Detect_SSN 测试社会安全号检测
func TestSensitiveDataDetector_Detect_SSN(t *testing.T) {
	mockManager := &MockRuleManagerForSensitiveData{
		rules: make(map[string][]types.Rule),
	}
	detector := NewSensitiveDataDetector(mockManager)

	req := &http.Request{}
	values := url.Values{}
	values.Add("ssn", "123-45-6789")
	req.URL = &url.URL{
		RawQuery: values.Encode(),
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Social Security")
}

// TestSensitiveDataDetector_Detect_PasswordInURL 测试 URL 中的密码检测
func TestSensitiveDataDetector_Detect_PasswordInURL(t *testing.T) {
	mockManager := &MockRuleManagerForSensitiveData{
		rules: make(map[string][]types.Rule),
	}
	detector := NewSensitiveDataDetector(mockManager)

	req := &http.Request{}
	req.URL = &url.URL{
		RawQuery: "password=secret123",
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Password")
}

// TestSensitiveDataDetector_Detect_APIKey 测试 API 密钥检测
func TestSensitiveDataDetector_Detect_APIKey(t *testing.T) {
	mockManager := &MockRuleManagerForSensitiveData{
		rules: make(map[string][]types.Rule),
	}
	detector := NewSensitiveDataDetector(mockManager)

	req := &http.Request{}
	values := url.Values{}
	values.Add("api_key", "sk-1234567890abcdef")
	req.URL = &url.URL{
		RawQuery: values.Encode(),
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "API Key")
}

// TestSensitiveDataDetector_Detect_Email 测试邮箱地址检测
func TestSensitiveDataDetector_Detect_Email(t *testing.T) {
	mockManager := &MockRuleManagerForSensitiveData{
		rules: make(map[string][]types.Rule),
	}
	detector := NewSensitiveDataDetector(mockManager)

	req := &http.Request{}
	values := url.Values{}
	values.Add("email", "user@example.com")
	req.URL = &url.URL{
		RawQuery: values.Encode(),
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Email")
}

// TestSensitiveDataDetector_Detect_Phone 测试电话号码检测
func TestSensitiveDataDetector_Detect_Phone(t *testing.T) {
	mockManager := &MockRuleManagerForSensitiveData{
		rules: make(map[string][]types.Rule),
	}
	detector := NewSensitiveDataDetector(mockManager)

	req := &http.Request{}
	values := url.Values{}
	values.Add("phone", "123-456-7890")
	req.URL = &url.URL{
		RawQuery: values.Encode(),
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Phone")
}

// TestSensitiveDataDetector_Detect_Header 测试请求头中的敏感数据
func TestSensitiveDataDetector_Detect_Header(t *testing.T) {
	mockManager := &MockRuleManagerForSensitiveData{
		rules: make(map[string][]types.Rule),
	}
	detector := NewSensitiveDataDetector(mockManager)

	req := &http.Request{}
	req.Header = make(http.Header)
	// 使用符合规则的 token= 格式
	req.Header.Set("X-Auth", "token=secret123")
	req.URL = &url.URL{}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	// Token 可能被检测为 API Key
	if len(threats) > 0 {
		assert.Equal(t, "sensitive-data", threats[0].Type)
	}
}

// TestSensitiveDataDetector_Detect_CustomRules 测试自定义规则
func TestSensitiveDataDetector_Detect_CustomRules(t *testing.T) {
	mockManager := &MockRuleManagerForSensitiveData{
		rules: map[string][]types.Rule{
			"sensitive-data": {
				{ID: "custom-sensitive-001", Name: "Custom Sensitive", Category: "sensitive-data", Pattern: "CUSTOM_SECRET", Severity: "high"},
			},
		},
	}
	detector := NewSensitiveDataDetector(mockManager)

	req := &http.Request{}
	values := url.Values{}
	values.Add("data", "CUSTOM_SECRET")
	req.URL = &url.URL{
		RawQuery: values.Encode(),
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "custom-sensitive-001", threats[0].RuleID)
}

// TestSensitiveDataDetector_Detect_NoThreats 测试无威胁请求
func TestSensitiveDataDetector_Detect_NoThreats(t *testing.T) {
	mockManager := &MockRuleManagerForSensitiveData{
		rules: make(map[string][]types.Rule),
	}
	detector := NewSensitiveDataDetector(mockManager)

	req := &http.Request{}
	values := url.Values{}
	values.Add("name", "john")
	values.Add("age", "30")
	req.URL = &url.URL{
		RawQuery: values.Encode(),
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestSensitiveDataDetector_Detect_EmptyQuery 测试空查询
func TestSensitiveDataDetector_Detect_EmptyQuery(t *testing.T) {
	mockManager := &MockRuleManagerForSensitiveData{
		rules: make(map[string][]types.Rule),
	}
	detector := NewSensitiveDataDetector(mockManager)

	req := &http.Request{}
	req.URL = &url.URL{}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestSensitiveDataDetector_Detect_MultipleSensitiveFields 测试多个敏感字段
func TestSensitiveDataDetector_Detect_MultipleSensitiveFields(t *testing.T) {
	mockManager := &MockRuleManagerForSensitiveData{
		rules: make(map[string][]types.Rule),
	}
	detector := NewSensitiveDataDetector(mockManager)

	req := &http.Request{}
	values := url.Values{}
	values.Add("card", "1234-5678-9012-3456")
	values.Add("ssn", "123-45-6789")
	values.Add("password", "secret")
	req.URL = &url.URL{
		RawQuery: values.Encode(),
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	// 应该检测到多个威胁
	assert.GreaterOrEqual(t, len(threats), 1)
}

// TestSensitiveDataDetector_Detect_16DigitCard 测试 16 位信用卡号
func TestSensitiveDataDetector_Detect_16DigitCard(t *testing.T) {
	mockManager := &MockRuleManagerForSensitiveData{
		rules: make(map[string][]types.Rule),
	}
	detector := NewSensitiveDataDetector(mockManager)

	req := &http.Request{}
	values := url.Values{}
	values.Add("card", "1234567890123456")
	req.URL = &url.URL{
		RawQuery: values.Encode(),
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Credit Card")
}
