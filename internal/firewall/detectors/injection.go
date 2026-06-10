package detectors

import (
	"net/http"
	"regexp"
	"sync"

	"prerender-shield/internal/firewall/types"
	"prerender-shield/internal/logging"
)

// InjectionDetector 注入攻击检测器
// 支持规则动态更新
type InjectionDetector struct {
	rules         []types.Rule
	compiledRules []compiledRule
	rulesMutex    sync.RWMutex
	name          string
}

// NewInjectionDetector 创建新的注入攻击检测器
func NewInjectionDetector(ruleProvider RuleProvider) *InjectionDetector {
	d := &InjectionDetector{
		name: "Injection",
	}

	if ruleProvider != nil {
		d.rules = ruleProvider.GetRulesByCategory("injection")
	}

	d.compileRules()

	return d
}

// compileRules 预编译规则
func (d *InjectionDetector) compileRules() {
	// 默认的注入攻击规则
	defaultRules := []types.Rule{
		{ID: "injection-001", Name: "SQL Injection", Category: "injection", Pattern: `'|"|OR\s+1=1|UNION|SELECT\s+\*`, Severity: "high"},
		{ID: "injection-002", Name: "Command Injection", Category: "injection", Pattern: `;|\||&|>|<%3B|<%7C|<%26|<%3E`, Severity: "high"},
		{ID: "injection-003", Name: "LDAP Injection", Category: "injection", Pattern: `\(|\)|&|\||!|=|\*|\\|/`, Severity: "medium"},
	}

	allRules := append(d.rules, defaultRules...)

	d.compiledRules = make([]compiledRule, 0, len(allRules))
	for _, rule := range allRules {
		if rule.Pattern == "" {
			continue
		}
		re, err := regexp.Compile(`(?i)` + rule.Pattern)
		if err != nil {
			logging.DefaultLogger.Info("Warning: failed to compile injection rule %s: %v\n", rule.ID, err)
			continue
		}
		d.compiledRules = append(d.compiledRules, compiledRule{
			rule:  rule,
			regex: re,
		})
	}
}

// UpdateRules 更新规则
func (d *InjectionDetector) UpdateRules(rules []types.Rule) error {
	d.rulesMutex.Lock()
	defer d.rulesMutex.Unlock()

	d.rules = rules
	d.compileRules()

	return nil
}

// Name 返回检测器名称
func (d *InjectionDetector) Name() string {
	return d.name
}

// Detect 检测注入攻击
func (d *InjectionDetector) Detect(req *http.Request) ([]types.Threat, error) {
	d.rulesMutex.RLock()
	compiledRules := make([]compiledRule, len(d.compiledRules))
	copy(compiledRules, d.compiledRules)
	d.rulesMutex.RUnlock()

	return checkHTTPInputs(req, compiledRules, "injection"), nil
}

// matchesPattern 检查值是否匹配正则表达式
func matchesPattern(value string, re *regexp.Regexp) bool {
	return re.MatchString(value)
}
