package detectors

import (
	"fmt"
	"net/http"
	"regexp"
	"sync"

	"prerender-shield/internal/firewall/types"
)

// SensitiveDataDetector 敏感数据泄露检测器
// 支持规则动态更新
type SensitiveDataDetector struct {
	rules         []types.Rule
	compiledRules []compiledRule
	rulesMutex    sync.RWMutex
	name          string
}

// NewSensitiveDataDetector 创建新的敏感数据检测器
func NewSensitiveDataDetector(ruleProvider RuleProvider) *SensitiveDataDetector {
	d := &SensitiveDataDetector{
		name: "SensitiveData",
	}

	if ruleProvider != nil {
		d.rules = ruleProvider.GetRulesByCategory("sensitive-data")
	}

	d.compileRules()

	return d
}

// compileRules 预编译规则
func (d *SensitiveDataDetector) compileRules() {
	// 默认的敏感数据规则
	defaultRules := []types.Rule{
		{ID: "sensitive-001", Name: "Credit Card Number", Category: "sensitive-data", Pattern: `\d{4}-\d{4}-\d{4}-\d{4}|\d{16}`, Severity: "high"},
		{ID: "sensitive-002", Name: "Social Security Number", Category: "sensitive-data", Pattern: `\d{3}-\d{2}-\d{4}`, Severity: "high"},
		{ID: "sensitive-003", Name: "Password in URL", Category: "sensitive-data", Pattern: `password=|pass=|pwd=|secret=`, Severity: "high"},
		{ID: "sensitive-004", Name: "API Key", Category: "sensitive-data", Pattern: `api_key=|api-key=|token=|auth=`, Severity: "high"},
	}

	allRules := append(d.rules, defaultRules...)

	d.compiledRules = make([]compiledRule, 0, len(allRules))
	for _, rule := range allRules {
		if rule.Pattern == "" {
			continue
		}
		re, err := regexp.Compile(`(?i)` + rule.Pattern)
		if err != nil {
			fmt.Printf("Warning: failed to compile sensitive data rule %s: %v\n", rule.ID, err)
			continue
		}
		d.compiledRules = append(d.compiledRules, compiledRule{
			rule:  rule,
			regex: re,
		})
	}
}

// UpdateRules 更新规则
func (d *SensitiveDataDetector) UpdateRules(rules []types.Rule) error {
	d.rulesMutex.Lock()
	defer d.rulesMutex.Unlock()

	d.rules = rules
	d.compileRules()

	return nil
}

// Name 返回检测器名称
func (d *SensitiveDataDetector) Name() string {
	return d.name
}

// Detect 检测敏感数据泄露
func (d *SensitiveDataDetector) Detect(req *http.Request) ([]types.Threat, error) {
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
						Type:      "sensitive-data",
						SubType:   cr.rule.Name,
						Severity:  cr.rule.Severity,
						Message:   "Sensitive data detected: " + cr.rule.Name,
						Parameter: name,
						Value:     "[REDACTED]",
						RuleID:    cr.rule.ID,
						RuleName:  cr.rule.Name,
					})
				}
			}
		}
	}

	return threats, nil
}
