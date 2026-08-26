package types

// RuleType 规则类型枚举
type RuleType string

const (
	// RuleTypeUserAgent User-Agent 检测规则
	RuleTypeUserAgent RuleType = "user_agent"
	// RuleTypeHeader 请求头检测规则
	RuleTypeHeader RuleType = "header"
	// RuleTypeMethod 请求方法检测规则
	RuleTypeMethod RuleType = "method"
	// RuleTypePath 路径检测规则
	RuleTypePath RuleType = "path"
	// RuleTypeBody 请求体检测规则
	RuleTypeBody RuleType = "body"
)

// Rule 规则
type Rule struct {
	ID       string
	Name     string
	Category string
	Pattern  string
	Severity string
}
