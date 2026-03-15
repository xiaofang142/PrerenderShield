package detectors

import (
	"fmt"
	"net/http"
	"regexp"
	"sync"

	"prerender-shield/internal/firewall/types"
)

// XSSDetector 跨站脚本攻击检测器
// 支持规则动态更新
type XSSDetector struct {
	rules         []types.Rule
	compiledRules []compiledRule
	rulesMutex    sync.RWMutex
	name          string
}

// NewXSSDetector 创建新的 XSS 检测器
func NewXSSDetector(ruleProvider RuleProvider) *XSSDetector {
	d := &XSSDetector{
		name: "XSS",
	}

	if ruleProvider != nil {
		d.rules = ruleProvider.GetRulesByCategory("xss")
	}

	d.compileRules()

	return d
}

// compileRules 预编译规则
func (d *XSSDetector) compileRules() {
	// 默认的 XSS 规则
	defaultRules := []types.Rule{
		{ID: "xss-001", Name: "HTML Tag Injection", Category: "xss", Pattern: `<script|</script>|<iframe|</iframe>|<object|</object>|<embed|</embed>`, Severity: "high"},
		{ID: "xss-002", Name: "JavaScript Event Handler", Category: "xss", Pattern: `onload=|onerror=|onclick=|onmouseover=|onfocus=|onblur=`, Severity: "high"},
		{ID: "xss-003", Name: "JavaScript Protocol", Category: "xss", Pattern: `javascript:|vbscript:|data:`, Severity: "high"},
		{ID: "xss-004", Name: "HTML Attribute Injection", Category: "xss", Pattern: `'|"|>|\||/|<%3C|<%3E|<%27|<%22`, Severity: "medium"},
	}

	allRules := append(d.rules, defaultRules...)

	d.compiledRules = make([]compiledRule, 0, len(allRules))
	for _, rule := range allRules {
		if rule.Pattern == "" {
			continue
		}
		re, err := regexp.Compile(`(?i)` + rule.Pattern)
		if err != nil {
			fmt.Printf("Warning: failed to compile XSS rule %s: %v\n", rule.ID, err)
			continue
		}
		d.compiledRules = append(d.compiledRules, compiledRule{
			rule:  rule,
			regex: re,
		})
	}
}

// UpdateRules 更新规则
func (d *XSSDetector) UpdateRules(rules []types.Rule) error {
	d.rulesMutex.Lock()
	defer d.rulesMutex.Unlock()

	d.rules = rules
	d.compileRules()

	return nil
}

// Name 返回检测器名称
func (d *XSSDetector) Name() string {
	return d.name
}

// Detect 检测 XSS 攻击
func (d *XSSDetector) Detect(req *http.Request) ([]types.Threat, error) {
	d.rulesMutex.RLock()
	compiledRules := make([]compiledRule, len(d.compiledRules))
	copy(compiledRules, d.compiledRules)
	d.rulesMutex.RUnlock()

	threats := make([]types.Threat, 0)

	// 检查请求参数
	for name, values := range req.URL.Query() {
		for _, value := range values {
			for _, cr := range compiledRules {
				if cr.regex.MatchString(value) {
					threats = append(threats, types.Threat{
						Type:      "xss",
						SubType:   cr.rule.Name,
						Severity:  cr.rule.Severity,
						Message:   cr.rule.Name + " detected",
						Parameter: name,
						Value:     value,
						RuleID:    cr.rule.ID,
						RuleName:  cr.rule.Name,
					})
				}
			}
		}
	}

	// 检查请求头
	for name, values := range req.Header {
		for _, value := range values {
			for _, cr := range compiledRules {
				if cr.regex.MatchString(value) {
					threats = append(threats, types.Threat{
						Type:      "xss",
						SubType:   cr.rule.Name,
						Severity:  cr.rule.Severity,
						Message:   cr.rule.Name + " detected in header",
						Parameter: name,
						Value:     value,
						RuleID:    cr.rule.ID,
						RuleName:  cr.rule.Name,
					})
				}
			}
		}
	}

	return threats, nil
}
