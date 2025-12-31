package detectors

import (
	"net/http"
	"regexp"
	"strings"

	"prerender-shield/internal/firewall/types"
)

// XSSDetector 跨站脚本攻击检测器
type XSSDetector struct {
	rules []types.Rule
}

// NewXSSDetector 创建新的XSS检测器
func NewXSSDetector(ruleManager interface{ GetRulesByCategory(category string) []types.Rule }) *XSSDetector {
	return &XSSDetector{
		rules: ruleManager.GetRulesByCategory("xss"),
	}
}

// Detect 检测XSS攻击
func (d *XSSDetector) Detect(req *http.Request) ([]types.Threat, error) {
	threats := make([]types.Threat, 0)

	// 默认的XSS规则（如果没有从规则文件加载）
	defaultRules := []types.Rule{
		{ID: "xss-001", Name: "HTML Tag Injection", Category: "xss", Pattern: "<script|</script>|<iframe|</iframe>|<object|</object>|<embed|</embed>", Severity: "high"},
		{ID: "xss-002", Name: "JavaScript Event Handler", Category: "xss", Pattern: "onload=|onerror=|onclick=|onmouseover=|onfocus=|onblur=", Severity: "high"},
		{ID: "xss-003", Name: "JavaScript Protocol", Category: "xss", Pattern: "javascript:|vbscript:|data:", Severity: "high"},
		{ID: "xss-004", Name: "HTML Attribute Injection", Category: "xss", Pattern: "'|\"|>|\\||/|<%3C|<%3E|<%27|<%22", Severity: "medium"},
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
				if matchesXSSPattern(value, rule.Pattern) {
					threats = append(threats, types.Threat{
						Type:      "xss",
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
				if matchesXSSPattern(value, rule.Pattern) {
					threats = append(threats, types.Threat{
						Type:      "xss",
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
func (d *XSSDetector) Name() string {
	return "xss_detector"
}

// matchesXSSPattern 检查值是否匹配XSS模式
func matchesXSSPattern(value, pattern string) bool {
	// 将值转换为小写以便不区分大小写匹配
	value = strings.ToLower(value)
	pattern = strings.ToLower(pattern)

	// 使用正则表达式匹配
	matched, _ := regexp.MatchString(pattern, value)
	return matched
}