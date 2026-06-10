package detectors

import (
	"net/http"
	"regexp"
	"sync"

	"prerender-shield/internal/firewall/types"
	"prerender-shield/internal/logging"
)

// DeserializationDetector 不安全的反序列化检测器
// 支持规则动态更新
type DeserializationDetector struct {
	rules         []types.Rule
	compiledRules []compiledRule
	rulesMutex    sync.RWMutex
	name          string
}

// NewDeserializationDetector 创建新的反序列化检测器
func NewDeserializationDetector(ruleProvider RuleProvider) *DeserializationDetector {
	d := &DeserializationDetector{
		name: "Deserialization",
	}

	if ruleProvider != nil {
		d.rules = ruleProvider.GetRulesByCategory("deserialization")
	}

	d.compileRules()

	return d
}

// compileRules 预编译规则
func (d *DeserializationDetector) compileRules() {
	// 默认的反序列化规则
	defaultRules := []types.Rule{
		{ID: "deserialization-001", Name: "Java Serialization", Category: "deserialization", Pattern: `%AC%ED%00%05|rO0ABX|aced0005`, Severity: "high"},
		{ID: "deserialization-002", Name: "Python Pickle", Category: "deserialization", Pattern: `%80%04|%80%03|c:|\(i|\(S|\(V`, Severity: "high"},
		{ID: "deserialization-003", Name: "PHP Serialization", Category: "deserialization", Pattern: `O:\d+:|s:\d+:|a:\d+:`, Severity: "high"},
	}

	allRules := append(d.rules, defaultRules...)

	d.compiledRules = make([]compiledRule, 0, len(allRules))
	for _, rule := range allRules {
		if rule.Pattern == "" {
			continue
		}
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			logging.DefaultLogger.Info("Warning: failed to compile deserialization rule %s: %v\n", rule.ID, err)
			continue
		}
		d.compiledRules = append(d.compiledRules, compiledRule{
			rule:  rule,
			regex: re,
		})
	}
}

// UpdateRules 更新规则
func (d *DeserializationDetector) UpdateRules(rules []types.Rule) error {
	d.rulesMutex.Lock()
	defer d.rulesMutex.Unlock()

	d.rules = rules
	d.compileRules()

	return nil
}

// Name 返回检测器名称
func (d *DeserializationDetector) Name() string {
	return d.name
}

// Detect 检测不安全的反序列化
func (d *DeserializationDetector) Detect(req *http.Request) ([]types.Threat, error) {
	d.rulesMutex.RLock()
	compiledRules := make([]compiledRule, len(d.compiledRules))
	copy(compiledRules, d.compiledRules)
	d.rulesMutex.RUnlock()

	return checkHTTPInputs(req, compiledRules, "deserialization"), nil
}
