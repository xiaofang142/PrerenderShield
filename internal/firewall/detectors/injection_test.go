package detectors

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/firewall/types"
)

// MockRuleManager 用于测试的规则管理器模拟
type MockRuleManager struct {
	rules map[string][]types.Rule
}

// GetRulesByCategory 根据分类获取规则
func (m *MockRuleManager) GetRulesByCategory(category string) []types.Rule {
	return m.rules[category]
}

func TestInjectionDetector_Detect(t *testing.T) {
	// 创建 mock 规则管理器
	mockRuleManager := &MockRuleManager{
		rules: make(map[string][]types.Rule),
	}

	// 创建注入检测器
	detector := NewInjectionDetector(mockRuleManager)

	// 测试 SQL 注入检测 - 直接使用带有注入的请求
	t.Run("SQL Injection Detection", func(t *testing.T) {
		// 直接创建请求，避免 URL 编码问题
		req := &http.Request{}
		// 手动设置查询参数
		values := url.Values{}
		values.Add("id", "1' OR '1'='1")
		req.URL = &url.URL{
			RawQuery: values.Encode(),
		}

		// 检测 SQL 注入
		threats, err := detector.Detect(req)
		assert.NoError(t, err)
		assert.NotEmpty(t, threats)
		assert.Equal(t, "injection", threats[0].Type)
	})

	// 测试正常请求
	t.Run("Normal Request", func(t *testing.T) {
		req := &http.Request{}
		values := url.Values{}
		values.Add("id", "123")
		req.URL = &url.URL{
			RawQuery: values.Encode(),
		}

		threats, err := detector.Detect(req)
		assert.NoError(t, err)
		assert.Empty(t, threats)
	})
}

func TestInjectionDetector_Name(t *testing.T) {
	// 创建 mock 规则管理器
	mockRuleManager := &MockRuleManager{
		rules: make(map[string][]types.Rule),
	}

	// 创建注入检测器
	detector := NewInjectionDetector(mockRuleManager)

	// 测试名称返回
	name := detector.Name()
	assert.Equal(t, "injection_detector", name)
}

// TestInjectionDetector_Detect_CommandInjection 测试命令注入检测
func TestInjectionDetector_Detect_CommandInjection(t *testing.T) {
	mockRuleManager := &MockRuleManager{
		rules: make(map[string][]types.Rule),
	}
	detector := NewInjectionDetector(mockRuleManager)

	req := &http.Request{}
	values := url.Values{}
	values.Add("cmd", "ls; cat /etc/passwd")
	req.URL = &url.URL{
		RawQuery: values.Encode(),
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "injection", threats[0].Type)
}

// TestInjectionDetector_Detect_LDAPInjection 测试 LDAP 注入检测
func TestInjectionDetector_Detect_LDAPInjection(t *testing.T) {
	mockRuleManager := &MockRuleManager{
		rules: make(map[string][]types.Rule),
	}
	detector := NewInjectionDetector(mockRuleManager)

	req := &http.Request{}
	values := url.Values{}
	// LDAP 注入特征：包含 ( | ) & = 等字符
	values.Add("filter", "(|(uid=admin)(cn=user))")
	req.URL = &url.URL{
		RawQuery: values.Encode(),
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	// LDAP 规则使用默认规则，可能被识别为 Command Injection 或 LDAP Injection
	assert.NotEmpty(t, threats)
}

// TestInjectionDetector_Detect_HeaderInjection 测试请求头中的注入检测
func TestInjectionDetector_Detect_HeaderInjection(t *testing.T) {
	mockRuleManager := &MockRuleManager{
		rules: make(map[string][]types.Rule),
	}
	detector := NewInjectionDetector(mockRuleManager)

	req := &http.Request{}
	values := url.Values{}
	req.URL = &url.URL{
		RawQuery: values.Encode(),
	}
	req.Header = make(http.Header)
	req.Header.Set("X-Custom-Header", "'; DROP TABLE users--")

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Contains(t, threats[0].Message, "header")
}

// TestInjectionDetector_Detect_CustomRules 测试自定义规则
func TestInjectionDetector_Detect_CustomRules(t *testing.T) {
	mockRuleManager := &MockRuleManager{
		rules: map[string][]types.Rule{
			"injection": {
				{ID: "custom-001", Name: "Custom Injection", Category: "injection", Pattern: "CUSTOM_INJECT", Severity: "high"},
			},
		},
	}
	detector := NewInjectionDetector(mockRuleManager)

	req := &http.Request{}
	values := url.Values{}
	values.Add("param", "CUSTOM_INJECT")
	req.URL = &url.URL{
		RawQuery: values.Encode(),
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
	assert.Equal(t, "custom-001", threats[0].RuleID)
}

// TestInjectionDetector_Detect_MultipleParams 测试多个参数同时检测
func TestInjectionDetector_Detect_MultipleParams(t *testing.T) {
	mockRuleManager := &MockRuleManager{
		rules: make(map[string][]types.Rule),
	}
	detector := NewInjectionDetector(mockRuleManager)

	req := &http.Request{}
	values := url.Values{}
	values.Add("id", "1' OR 1=1")
	values.Add("name", "admin")
	values.Add("cmd", "test;ls")
	req.URL = &url.URL{
		RawQuery: values.Encode(),
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, threats)
}

// TestInjectionDetector_Detect_EmptyQuery 测试空查询参数
func TestInjectionDetector_Detect_EmptyQuery(t *testing.T) {
	mockRuleManager := &MockRuleManager{
		rules: make(map[string][]types.Rule),
	}
	detector := NewInjectionDetector(mockRuleManager)

	req := &http.Request{}
	req.URL = &url.URL{
		RawQuery: "",
	}

	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Empty(t, threats)
}

// TestInjectionDetector_Detect_CaseInsensitive 测试大小写不敏感
func TestInjectionDetector_Detect_CaseInsensitive(t *testing.T) {
	mockRuleManager := &MockRuleManager{
		rules: make(map[string][]types.Rule),
	}
	detector := NewInjectionDetector(mockRuleManager)

	testCases := []string{
		"SELECT * FROM users",
		"select * from users",
		"SeLeCt * FrOm users",
	}

	for _, value := range testCases {
		req := &http.Request{}
		values := url.Values{}
		values.Add("query", value)
		req.URL = &url.URL{
			RawQuery: values.Encode(),
		}

		threats, err := detector.Detect(req)
		assert.NoError(t, err)
		assert.NotEmpty(t, threats)
	}
}
