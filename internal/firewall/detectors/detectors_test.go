package detectors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"prerender-shield/internal/firewall/types"
)

// EmptyRuleManager 返回空规则切片的 mock
type EmptyRuleManager struct{}

func (m *EmptyRuleManager) GetRulesByCategory(category string) []types.Rule {
	return []types.Rule{}
}

// MockRuleManager 模拟规则管理器
type MockRuleManager struct {
	rules map[string][]types.Rule
}

func (m *MockRuleManager) GetRulesByCategory(category string) []types.Rule {
	if m == nil || m.rules == nil {
		return []types.Rule{}
	}
	return m.rules[category]
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
