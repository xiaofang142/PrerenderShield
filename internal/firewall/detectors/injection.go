package detectors

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/prerendershield/internal/firewall/types"
)

// InjectionDetector 注入攻击检测器
type InjectionDetector struct {
	rules []types.Rule
}

// NewInjectionDetector 创建新的注入攻击检测器
func NewInjectionDetector(ruleManager interface{ GetRulesByCategory(category string) []types.Rule }) *InjectionDetector {
	return &InjectionDetector{
		rules: ruleManager.GetRulesByCategory("injection"),
	}
}

// Detect 检测注入攻击
func (d *InjectionDetector) Detect(req *http.Request) ([]types.Threat, error) {
	threats := make([]types.Threat, 0)

	// 默认的注入攻击规则（如果没有从规则文件加载）
	defaultRules := []types.Rule{
		{ID: "injection-001", Name: "SQL Injection", Category: "injection", Pattern: "'|\"|OR\\s+1=1|UNION|SELECT\\s+\\*", Severity: "high"},
		{ID: "injection-002", Name: "Command Injection", Category: "injection", Pattern: ";|\\||&|>|<%3B|<%7C|<%26|<%3E", Severity: "high"},
		{ID: "injection-003", Name: "LDAP Injection", Category: "injection", Pattern: "\\(|\\)|&|\\||!|=|\\*|\\\\|\\/", Severity: "medium"},
	}

	// 使用默认规则或从规则管理器加载的规则
	rules := d.rules
	if len(rules) == 0 {
		rules = defaultRules
	}

	// 检查请求参数
	for name, values := range req.URL.Query() {
		for _, value := range values {
			for _, rule := range rules {
				if matchesInjectionPattern(value, rule.Pattern) {
					threats = append(threats, types.Threat{
						Type:      "injection",
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

	// 检查请求体
	if req.Method == "POST" || req.Method == "PUT" || req.Method == "PATCH" {
		// 解析请求体并检查（这里简化处理，实际实现需要根据Content-Type解析不同格式的请求体）
		// 例如：application/x-www-form-urlencoded, multipart/form-data, application/json等
	}

	// 检查请求头
	for name, values := range req.Header {
		for _, value := range values {
			for _, rule := range rules {
				if matchesInjectionPattern(value, rule.Pattern) {
					threats = append(threats, types.Threat{
						Type:      "injection",
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
func (d *InjectionDetector) Name() string {
	return "injection_detector"
}

// matchesInjectionPattern 检查值是否匹配注入攻击模式
func matchesInjectionPattern(value, pattern string) bool {
	value = strings.ToUpper(value)
	pattern = strings.ToUpper(pattern)

	// 使用正则表达式匹配
	matched, _ := regexp.MatchString(pattern, value)
	return matched
}