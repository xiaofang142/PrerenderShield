package detectors

import (
	"net/http"
	"regexp"

	"prerender-shield/internal/firewall/types"
)

// SensitiveDataDetector 敏感数据泄露检测器
type SensitiveDataDetector struct {
	rules []types.Rule
}

// NewSensitiveDataDetector 创建新的敏感数据检测器
func NewSensitiveDataDetector(ruleManager interface{ GetRulesByCategory(category string) []types.Rule }) *SensitiveDataDetector {
	return &SensitiveDataDetector{
		rules: ruleManager.GetRulesByCategory("sensitive-data"),
	}
}

// Detect 检测敏感数据泄露
func (d *SensitiveDataDetector) Detect(req *http.Request) ([]types.Threat, error) {
	threats := make([]types.Threat, 0)

	// 默认的敏感数据规则（如果没有从规则文件加载）
	defaultRules := []types.Rule{
		{ID: "sensitive-001", Name: "Credit Card Number", Category: "sensitive-data", Pattern: "\\d{4}-\\d{4}-\\d{4}-\\d{4}|\\d{16}", Severity: "high"},
		{ID: "sensitive-002", Name: "Social Security Number", Category: "sensitive-data", Pattern: "\\d{3}-\\d{2}-\\d{4}", Severity: "high"},
		{ID: "sensitive-003", Name: "Password in URL", Category: "sensitive-data", Pattern: "password=|pass=|pwd=|secret=", Severity: "high"},
		{ID: "sensitive-004", Name: "API Key", Category: "sensitive-data", Pattern: "api_key=|api-key=|token=|auth=", Severity: "high"},
		{ID: "sensitive-005", Name: "Email Address", Category: "sensitive-data", Pattern: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}", Severity: "medium"},
		{ID: "sensitive-006", Name: "Phone Number", Category: "sensitive-data", Pattern: "\\+?\\d{10,15}|\\d{3}-\\d{3}-\\d{4}", Severity: "medium"},
	}

	// 使用默认规则或从规则管理器加载的规则
	rules := d.rules
	if len(rules) == 0 {
		rules = defaultRules
	}

	// 检查请求URL
	url := req.URL.String()
	for _, rule := range rules {
		if matchesSensitiveDataPattern(url, rule.Pattern) {
			threats = append(threats, types.Threat{
				Type:      "sensitive-data",
				SubType:   rule.Name,
				Severity:  rule.Severity,
				Message:   rule.Name + " detected in URL",
				Parameter: "URL",
				Value:     url,
				RuleID:    rule.ID,
				RuleName:  rule.Name,
			})
		}
	}

	// 检查请求参数
	for name, values := range req.URL.Query() {
		for _, value := range values {
			for _, rule := range rules {
				if matchesSensitiveDataPattern(value, rule.Pattern) {
					threats = append(threats, types.Threat{
						Type:      "sensitive-data",
						SubType:   rule.Name,
						Severity:  rule.Severity,
						Message:   rule.Name + " detected",
						Parameter: name,
						Value:     value,
						RuleID:    rule.ID,
						RuleName:  rule.Name,
					})
				}
			}
		}
	}

	// 检查请求头
	for name, values := range req.Header {
		for _, value := range values {
			for _, rule := range rules {
				if matchesSensitiveDataPattern(value, rule.Pattern) {
					threats = append(threats, types.Threat{
						Type:      "sensitive-data",
						SubType:   rule.Name,
						Severity:  rule.Severity,
						Message:   rule.Name + " detected in header",
						Parameter: name,
						Value:     value,
						RuleID:    rule.ID,
						RuleName:  rule.Name,
					})
				}
			}
		}
	}

	return threats, nil
}

// Name 返回检测器名称
func (d *SensitiveDataDetector) Name() string {
	return "sensitive_data_detector"
}

// matchesSensitiveDataPattern 检查值是否匹配敏感数据模式
func matchesSensitiveDataPattern(value, pattern string) bool {
	// 使用正则表达式匹配
	matched, _ := regexp.MatchString(pattern, value)
	return matched
}