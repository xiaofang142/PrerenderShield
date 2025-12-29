package detectors

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/prerendershield/internal/firewall/types"
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
	// 创建mock规则管理器
	mockRuleManager := &MockRuleManager{
		rules: make(map[string][]types.Rule),
	}

	// 创建注入检测器
	detector := NewInjectionDetector(mockRuleManager)

	// 测试SQL注入检测 - 直接使用带有注入的请求
	t.Run("SQL Injection Detection", func(t *testing.T) {
		// 直接创建请求，避免URL编码问题
		req := &http.Request{}
		// 手动设置查询参数
		values := url.Values{}
		values.Add("id", "1' OR '1'='1")
		req.URL = &url.URL{
			RawQuery: values.Encode(),
		}

		// 检测SQL注入
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
	// 创建mock规则管理器
	mockRuleManager := &MockRuleManager{
		rules: make(map[string][]types.Rule),
	}

	// 创建注入检测器
	detector := NewInjectionDetector(mockRuleManager)

	// 测试名称返回
	name := detector.Name()
	assert.Equal(t, "injection_detector", name)
}
