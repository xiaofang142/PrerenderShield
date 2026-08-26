package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuleTypeConstants(t *testing.T) {
	// 验证架构图声明的 5 种规则类型常量都存在且有值
	assert.Equal(t, RuleType("user_agent"), RuleTypeUserAgent)
	assert.Equal(t, RuleType("header"), RuleTypeHeader)
	assert.Equal(t, RuleType("method"), RuleTypeMethod)
	assert.Equal(t, RuleType("path"), RuleTypePath)
	assert.Equal(t, RuleType("body"), RuleTypeBody)
}

func TestRuleStruct(t *testing.T) {
	r := Rule{
		ID:       "test-001",
		Name:     "Test Rule",
		Category: "injection",
		Pattern:  `(?i)union.*select`,
		Severity: "high",
	}

	assert.Equal(t, "test-001", r.ID)
	assert.Equal(t, "Test Rule", r.Name)
	assert.Equal(t, "injection", r.Category)
	assert.Equal(t, "high", r.Severity)
}
