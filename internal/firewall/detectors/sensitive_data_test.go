package detectors

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/firewall/types"
)

// ============ 测试检测器创建 ============

// TestSensitiveDataDetector_Name 测试检测器名称
func TestSensitiveDataDetector_Name(t *testing.T) {
	mockProvider := &MockRuleProvider{
		rules: make(map[string][]types.Rule),
	}
	detector := NewSensitiveDataDetector(mockProvider)
	assert.Equal(t, "SensitiveData", detector.Name())
}

// TestSensitiveDataDetector_New_WithNilProvider 测试使用 nil Provider 创建检测器
func TestSensitiveDataDetector_New_WithNilProvider(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)
	assert.NotNil(t, detector)
	assert.Equal(t, "SensitiveData", detector.Name())
}

// ============ 测试信用卡号检测 ============

// TestSensitiveDataDetector_Detect_CreditCard_Formatted 测试格式化信用卡号
func TestSensitiveDataDetector_Detect_CreditCard_Formatted(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/payment?card=1234-5678-9012-3456", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "sensitive-data", threats[0].Type)
	assert.Contains(t, threats[0].SubType, "Credit Card")
}

// TestSensitiveDataDetector_Detect_CreditCard_Raw 测试原始 16 位信用卡号
func TestSensitiveDataDetector_Detect_CreditCard_Raw(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/payment?card=1234567890123456", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Credit Card")
}

// TestSensitiveDataDetector_Detect_CreditCard_Visa 测试 Visa 卡号
func TestSensitiveDataDetector_Detect_CreditCard_Visa(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	// Visa 卡号示例
	req := httptest.NewRequest(http.MethodGet, "/payment?card=4111111111111111", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Credit Card")
}

// TestSensitiveDataDetector_Detect_CreditCard_Mastercard 测试 Mastercard 卡号
func TestSensitiveDataDetector_Detect_CreditCard_Mastercard(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	// Mastercard 卡号示例
	req := httptest.NewRequest(http.MethodGet, "/payment?card=5500000000000004", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Credit Card")
}

// ============ 测试社会安全号检测 ============

// TestSensitiveDataDetector_Detect_SSN 测试社会安全号
func TestSensitiveDataDetector_Detect_SSN(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/user?ssn=123-45-6789", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Social Security Number")
}

// TestSensitiveDataDetector_Detect_SSN_Multiple 测试多个社会安全号
func TestSensitiveDataDetector_Detect_SSN_Multiple(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/users?ssn1=123-45-6789&ssn2=987-65-4321", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(threats), 1)
	assert.Contains(t, threats[0].SubType, "Social Security Number")
}

// ============ 测试 URL 中密码检测 ============

// TestSensitiveDataDetector_Detect_PasswordInURL_Equals 测试 URL 中的 password= 参数
func TestSensitiveDataDetector_Detect_PasswordInURL_Equals(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	// 使用 url.Values 确保=被正确编码为参数值的一部分
	values := url.Values{}
	values.Set("param", "password=secret123")
	req := httptest.NewRequest(http.MethodGet, "/login?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Password in URL")
}

// TestSensitiveDataDetector_Detect_PasswordInURL_Pass 测试 URL 中的 pass= 参数
func TestSensitiveDataDetector_Detect_PasswordInURL_Pass(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	// 使用 url.Values 确保=被正确编码为参数值的一部分
	values := url.Values{}
	values.Set("param", "pass=secret123")
	req := httptest.NewRequest(http.MethodGet, "/login?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Password in URL")
}

// TestSensitiveDataDetector_Detect_PasswordInURL_Pwd 测试 URL 中的 pwd= 参数
func TestSensitiveDataDetector_Detect_PasswordInURL_Pwd(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	// 使用 url.Values 确保=被正确编码为参数值的一部分
	values := url.Values{}
	values.Set("param", "pwd=secret123")
	req := httptest.NewRequest(http.MethodGet, "/login?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Password in URL")
}

// TestSensitiveDataDetector_Detect_PasswordInURL_Secret 测试 URL 中的 secret= 参数
func TestSensitiveDataDetector_Detect_PasswordInURL_Secret(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	// 使用 url.Values 确保=被正确编码为参数值的一部分
	values := url.Values{}
	values.Set("param", "secret=mysecretkey")
	req := httptest.NewRequest(http.MethodGet, "/config?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Password in URL")
}

// TestSensitiveDataDetector_Detect_PasswordInURL_CaseInsensitive 测试密码检测大小写不敏感
func TestSensitiveDataDetector_Detect_PasswordInURL_CaseInsensitive(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	// 使用 url.Values 确保=被正确编码为参数值的一部分
	values := url.Values{}
	values.Set("param", "PASSWORD=secret123")
	req := httptest.NewRequest(http.MethodGet, "/login?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "Password in URL")
}

// ============ 测试 API 密钥检测 ============

// TestSensitiveDataDetector_Detect_APIKey_api_key 测试参数值中包含 api_key=模式
func TestSensitiveDataDetector_Detect_APIKey_api_key(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	// 检测参数值中包含 api_key= 模式的情况
	values := url.Values{}
	values.Set("redirect", "/callback?api_key=sk-1234567890")
	req := httptest.NewRequest(http.MethodGet, "/oauth?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "API Key")
}

// TestSensitiveDataDetector_Detect_APIKey_api_dash_key 测试参数值中包含 api-key=模式
func TestSensitiveDataDetector_Detect_APIKey_api_dash_key(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	values := url.Values{}
	values.Set("redirect", "/callback?api-key=sk-1234567890")
	req := httptest.NewRequest(http.MethodGet, "/oauth?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "API Key")
}

// TestSensitiveDataDetector_Detect_APIKey_token 测试参数值中包含 token=模式
func TestSensitiveDataDetector_Detect_APIKey_token(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	values := url.Values{}
	values.Set("redirect", "/callback?token=Bearer_xyz123")
	req := httptest.NewRequest(http.MethodGet, "/oauth?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "API Key")
}

// TestSensitiveDataDetector_Detect_APIKey_auth 测试参数值中包含 auth=模式
func TestSensitiveDataDetector_Detect_APIKey_auth(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	values := url.Values{}
	values.Set("redirect", "/callback?auth=secret_auth_token")
	req := httptest.NewRequest(http.MethodGet, "/oauth?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "API Key")
}

// TestSensitiveDataDetector_Detect_APIKey_CaseInsensitive 测试 API 密钥检测大小写不敏感
func TestSensitiveDataDetector_Detect_APIKey_CaseInsensitive(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	values := url.Values{}
	values.Set("redirect", "/callback?API_KEY=sk-1234567890")
	req := httptest.NewRequest(http.MethodGet, "/oauth?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].SubType, "API Key")
}

// ============ 测试规则更新 ============

// TestSensitiveDataDetector_UpdateRules 测试动态更新规则
func TestSensitiveDataDetector_UpdateRules(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	customRules := []types.Rule{
		{ID: "custom-sensitive-001", Name: "Custom Secret", Category: "sensitive-data", Pattern: `MY_SECRET_PATTERN`, Severity: "critical"},
	}

	err := detector.UpdateRules(customRules)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/test?data=MY_SECRET_PATTERN", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "custom-sensitive-001", threats[0].RuleID)
	assert.Equal(t, "critical", threats[0].Severity)
}

// TestSensitiveDataDetector_UpdateRules_WithProvider 测试使用 Provider 更新规则
func TestSensitiveDataDetector_UpdateRules_WithProvider(t *testing.T) {
	mockProvider := &MockRuleProvider{
		rules: map[string][]types.Rule{
			"sensitive-data": {
				{ID: "provider-sensitive-001", Name: "Provider Secret", Category: "sensitive-data", Pattern: `PROVIDER_SECRET`, Severity: "high"},
			},
		},
	}
	detector := NewSensitiveDataDetector(mockProvider)

	req := httptest.NewRequest(http.MethodGet, "/test?data=PROVIDER_SECRET", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "provider-sensitive-001", threats[0].RuleID)
}

// ============ 测试多个参数 ============

// TestSensitiveDataDetector_Detect_MultipleParams 测试多个参数同时检测
func TestSensitiveDataDetector_Detect_MultipleParams(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	values := url.Values{}
	values.Add("card", "1234-5678-9012-3456")
	values.Add("ssn", "123-45-6789")
	values.Add("redirect", "/callback?api_key=sk-1234567890")
	values.Add("safe", "normal_value")

	req := httptest.NewRequest(http.MethodGet, "/submit?"+values.Encode(), nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.GreaterOrEqual(t, len(threats), 3)
}

// TestSensitiveDataDetector_Detect_MultipleThreatsInSameParam 测试同一参数中多个威胁
func TestSensitiveDataDetector_Detect_MultipleThreatsInSameParam(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	// 参数值包含多个敏感模式
	req := httptest.NewRequest(http.MethodGet, "/test?q=password=secret123&api_key=sk-1234", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
}

// ============ 测试边界情况 ============

// TestSensitiveDataDetector_Detect_EmptyQuery 测试空查询参数
func TestSensitiveDataDetector_Detect_EmptyQuery(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestSensitiveDataDetector_Detect_NormalRequest 测试正常请求
func TestSensitiveDataDetector_Detect_NormalRequest(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/users?page=1&limit=10", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestSensitiveDataDetector_Detect_SafeData 测试安全数据
func TestSensitiveDataDetector_Detect_SafeData(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	// 包含数字但不是敏感数据
	req := httptest.NewRequest(http.MethodGet, "/user?age=30&count=100", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// ============ 测试威胁详情 ============

// TestSensitiveDataDetector_ThreatDetails 测试威胁详情完整性
func TestSensitiveDataDetector_ThreatDetails(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/test?card=1234-5678-9012-3456", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)

	threat := threats[0]
	assert.Equal(t, "sensitive-data", threat.Type)
	assert.NotEmpty(t, threat.SubType)
	assert.NotEmpty(t, threat.Severity)
	assert.NotEmpty(t, threat.Message)
	assert.NotEmpty(t, threat.Parameter)
	// 敏感数据值会保留用于审计，由上层决定是否 redact
	assert.Equal(t, "1234-5678-9012-3456", threat.Value)
	assert.NotEmpty(t, threat.RuleID)
	assert.NotEmpty(t, threat.RuleName)
}

// TestSensitiveDataDetector_ThreatMessage 测试威胁消息格式
func TestSensitiveDataDetector_ThreatMessage(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	req := httptest.NewRequest(http.MethodGet, "/test?card=1234-5678-9012-3456", nil)
	threats, err := detector.Detect(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].Message, "detected")
}

// ============ 测试并发安全 ============

// TestSensitiveDataDetector_ConcurrentAccess 测试并发访问安全性
func TestSensitiveDataDetector_ConcurrentAccess(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	done := make(chan bool)
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			req := httptest.NewRequest(http.MethodGet, "/test?card=1234-5678-9012-3456", nil)
			_, err := detector.Detect(req)
			if err != nil {
				errors <- err
			}
		}()
	}

	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case err := <-errors:
			t.Errorf("Concurrent access error: %v", err)
		}
	}
}

// TestSensitiveDataDetector_ConcurrentUpdateAndDetect 测试并发更新和检测
func TestSensitiveDataDetector_ConcurrentUpdateAndDetect(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	done := make(chan bool)
	errors := make(chan error, 10)

	go func() {
		defer func() { done <- true }()
		for i := 0; i < 5; i++ {
			customRules := []types.Rule{
				{ID: "custom-" + string(rune(i)), Name: "Custom", Category: "sensitive-data", Pattern: `CUSTOM`, Severity: "high"},
			}
			err := detector.UpdateRules(customRules)
			if err != nil {
				errors <- err
			}
		}
	}()

	go func() {
		defer func() { done <- true }()
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test?card=1234-5678-9012-3456", nil)
			_, err := detector.Detect(req)
			if err != nil {
				errors <- err
			}
		}
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case err := <-errors:
			t.Errorf("Concurrent update/detect error: %v", err)
		}
	}
}

// ============ 测试 compileRules 边缘情况 ============

// TestSensitiveDataDetector_CompileRules_InvalidPattern 测试编译无效规则模式
func TestSensitiveDataDetector_CompileRules_InvalidPattern(t *testing.T) {
	mockProvider := &MockRuleProvider{
		rules: map[string][]types.Rule{
			"sensitive-data": {
				{ID: "invalid-rule", Name: "Invalid Rule", Category: "sensitive-data", Pattern: "[invalid(regex", Severity: "high"},
			},
		},
	}
	detector := NewSensitiveDataDetector(mockProvider)

	// 验证检测器创建成功（无效规则被跳过）
	assert.NotNil(t, detector)

	// 验证仍然可以正常检测
	req := httptest.NewRequest(http.MethodGet, "/test?card=1234-5678-9012-3456", nil)
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
}

// TestSensitiveDataDetector_CompileRules_EmptyPattern 测试空规则模式
func TestSensitiveDataDetector_CompileRules_EmptyPattern(t *testing.T) {
	mockProvider := &MockRuleProvider{
		rules: map[string][]types.Rule{
			"sensitive-data": {
				{ID: "empty-rule", Name: "Empty Rule", Category: "sensitive-data", Pattern: "", Severity: "high"},
			},
		},
	}
	detector := NewSensitiveDataDetector(mockProvider)

	// 验证检测器创建成功（空规则被跳过）
	assert.NotNil(t, detector)
}

// TestSensitiveDataDetector_CompileRules_PrecompiledRules 测试预编译规则数量
func TestSensitiveDataDetector_CompileRules_PrecompiledRules(t *testing.T) {
	detector := NewSensitiveDataDetector(nil)

	// 验证默认规则已预编译
	req := httptest.NewRequest(http.MethodGet, "/search?q=normal", nil)
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats) // 正常请求无威胁
}
