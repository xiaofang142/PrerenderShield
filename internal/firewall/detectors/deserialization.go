package detectors

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/prerendershield/internal/firewall/types"
)

// DeserializationDetector 不安全的反序列化检测器
type DeserializationDetector struct {
	rules []types.Rule
}

// NewDeserializationDetector 创建新的反序列化检测器
func NewDeserializationDetector(ruleManager interface{ GetRulesByCategory(category string) []types.Rule }) *DeserializationDetector {
	return &DeserializationDetector{
		rules: ruleManager.GetRulesByCategory("deserialization"),
	}
}

// Detect 检测不安全的反序列化
func (d *DeserializationDetector) Detect(req *http.Request) ([]types.Threat, error) {
	threats := make([]types.Threat, 0)

	// 默认的反序列化规则（如果没有从规则文件加载）
	defaultRules := []types.Rule{
		{ID: "deserialization-001", Name: "Java Serialization", Category: "deserialization", Pattern: "\\xac\\xed\\x00\\x05", Severity: "high"},
		{ID: "deserialization-002", Name: "Python Pickle", Category: "deserialization", Pattern: "\\x80\\x03|\\x80\\x04|c:|\\(i|\\(S|\\(V", Severity: "high"},
		{ID: "deserialization-003", Name: "PHP Serialization", Category: "deserialization", Pattern: "O:\\d+:\\|s:\\d+:\\|a:\\d+:\\|i:\\d+:\\|d:\\d+\\.\\d+:\\|b:\\d+:\\|N;", Severity: "high"},
		{ID: "deserialization-004", Name: "JavaScript Serialization", Category: "deserialization", Pattern: "\\{.*\\}|\\[.*\\]|\\\\u[0-9a-fA-F]{4}", Severity: "medium"},
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
				if matchesDeserializationPattern(value, rule.Pattern) {
					threats = append(threats, types.Threat{
						Type:      "deserialization",
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
		// 检查Content-Type
		contentType := req.Header.Get("Content-Type")
		if strings.Contains(contentType, "application/x-www-form-urlencoded") {
			// 解析表单数据
			if err := req.ParseForm(); err == nil {
				for name, values := range req.Form {
					for _, value := range values {
						for _, rule := range rules {
							if matchesDeserializationPattern(value, rule.Pattern) {
								threats = append(threats, types.Threat{
									Type:      "deserialization",
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
			}
		}
	}

	return threats, nil
}

// Name 返回检测器名称
func (d *DeserializationDetector) Name() string {
	return "deserialization_detector"
}

// matchesDeserializationPattern 检查值是否匹配反序列化模式
func matchesDeserializationPattern(value, pattern string) bool {
	// 对于十六进制模式，直接检查字节匹配
	if strings.HasPrefix(pattern, "\\x") {
		// 这里简化处理，实际实现需要将十六进制字符串转换为字节进行匹配
		return false
	}

	// 使用正则表达式匹配
	matched, _ := regexp.MatchString(pattern, value)
	return matched
}