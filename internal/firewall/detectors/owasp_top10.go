package detectors

import (
	"net/http"
	"regexp"
	"strings"
	"sync"

	"prerender-shield/internal/firewall/types"
	"prerender-shield/internal/logging"
)

// compiledRule 预编译的规则
type compiledRule struct {
	rule  types.Rule
	regex *regexp.Regexp
}

// RuleProvider 规则提供者接口
type RuleProvider interface {
	GetRulesByCategory(category string) []types.Rule
}

// OWASPTop10Detector OWASP Top 10 攻击检测器
// 支持规则动态更新，使用读写锁保证并发安全
type OWASPTop10Detector struct {
	rules         []types.Rule
	compiledRules []compiledRule
	rulesMutex    sync.RWMutex
	name          string
}

// NewOWASPTop10Detector 创建 OWASP Top 10 检测器
func NewOWASPTop10Detector(ruleProvider RuleProvider) *OWASPTop10Detector {
	d := &OWASPTop10Detector{
		name: "OWASP-Top10",
	}

	// 从规则提供者获取规则
	if ruleProvider != nil {
		d.rules = ruleProvider.GetRulesByCategory("owasp_top10")
	}

	// 预编译规则
	d.compileRules()

	return d
}

// compileRules 预编译正则表达式（内部方法，需要持有锁）
func (d *OWASPTop10Detector) compileRules() {
	// OWASP Top 10 2021 规则 - 内置默认规则
	owaspRules := []types.Rule{
		// A01: Broken Access Control
		{ID: "owasp-a01", Name: "Path Traversal", Category: "owasp_top10", Pattern: `\.\\./|%2e%2e%2f|%2e%2e/|\\.\\./|%2e%2e\\`, Severity: "high"},
		{ID: "owasp-a01-2", Name: "Admin Access Attempt", Category: "owasp_top10", Pattern: `(/admin|/administrator|/wp-admin|/phpmyadmin|/manager)`, Severity: "medium"},

		// A02: Cryptographic Failures
		{ID: "owasp-a02", Name: "Sensitive Data Exposure", Category: "owasp_top10", Pattern: `(passwd|password|secret|private|credential|token)\s*=\s*[^&]+`, Severity: "high"},

		// A04: Insecure Design
		{ID: "owasp-a04", Name: "Mass Assignment", Category: "owasp_top10", Pattern: `(__proto__|prototype|constructor)`, Severity: "medium"},

		// A05: Security Misconfiguration
		{ID: "owasp-a05", Name: "Server Info Disclosure", Category: "owasp_top10", Pattern: `(server-tokens|X-Powered-By|PHP/)`, Severity: "low"},

		// A06: Vulnerable and Outdated Components
		{ID: "owasp-a06", Name: "Vulnerability Scanner", Category: "owasp_top10", Pattern: `(nikto|nmap|nessus|openvas|w3af|wpscan|sqlmap|burp)`, Severity: "high"},

		// A08: Software and Data Integrity Failures
		{ID: "owasp-a08", Name: "Untrusted Data", Category: "owasp_top10", Pattern: `(php://|file://|gopher://|dict://)`, Severity: "high"},

		// A10: Server-Side Request Forgery (SSRF)
		{ID: "owasp-a10", Name: "SSRF Attempt", Category: "owasp_top10", Pattern: `(127\.0\.0\.1|localhost|0\.0\.0\.0|169\.254\.|\[::1\]|metadata\.google)`, Severity: "critical"},
	}

	// 合并规则
	allRules := append(d.rules, owaspRules...)

	// 预编译所有正则
	d.compiledRules = make([]compiledRule, 0, len(allRules))
	for _, rule := range allRules {
		if rule.Pattern == "" {
			continue
		}
		re, err := regexp.Compile(`(?i)` + rule.Pattern)
		if err != nil {
			// 规则编译失败，记录但不影响其他规则
			logging.DefaultLogger.Info("Warning: failed to compile rule %s: %v\n", rule.ID, err)
			continue
		}
		d.compiledRules = append(d.compiledRules, compiledRule{
			rule:  rule,
			regex: re,
		})
	}
}

// UpdateRules 动态更新规则（支持热更新）
func (d *OWASPTop10Detector) UpdateRules(rules []types.Rule) error {
	d.rulesMutex.Lock()
	defer d.rulesMutex.Unlock()

	d.rules = rules
	d.compileRules()

	return nil
}

// createThreat 创建威胁对象
func (d *OWASPTop10Detector) createThreat(rule types.Rule, parameter, value, message string) types.Threat {
	return types.Threat{
		Type:      "owasp_top10",
		SubType:   rule.Name,
		Severity:  rule.Severity,
		Message:   message,
		Parameter: parameter,
		Value:     value,
		RuleID:    rule.ID,
		RuleName:  rule.Name,
	}
}

// Name 返回检测器名称
func (d *OWASPTop10Detector) Name() string {
	return d.name
}

// Detect 检测 OWASP Top 10 攻击
func (d *OWASPTop10Detector) Detect(req *http.Request) ([]types.Threat, error) {
	// 使用读锁获取编译后的规则
	d.rulesMutex.RLock()
	compiledRules := make([]compiledRule, len(d.compiledRules))
	copy(compiledRules, d.compiledRules)
	d.rulesMutex.RUnlock()

	threats := make([]types.Threat, 0)

	// 检查 URL
	urlStr := req.URL.String()
	for _, cr := range compiledRules {
		if cr.regex.MatchString(urlStr) {
			threats = append(threats, d.createThreat(
				cr.rule,
				"url",
				urlStr,
				"OWASP Top 10: "+cr.rule.Name+" detected in URL",
			))
		}
	}

	// 检查查询参数
	for name, values := range req.URL.Query() {
		for _, value := range values {
			for _, cr := range compiledRules {
				if cr.regex.MatchString(value) {
					threats = append(threats, d.createThreat(
						cr.rule,
						name,
						value,
						"OWASP Top 10: "+cr.rule.Name+" detected",
					))
				}
			}
		}
	}

	// 检查请求头 - 仅检测扫描工具
	userAgent := req.Header.Get("User-Agent")
	for _, cr := range compiledRules {
		if strings.Contains(cr.rule.Name, "Scanner") {
			if cr.regex.MatchString(userAgent) {
				threats = append(threats, d.createThreat(
					cr.rule,
					"User-Agent",
					userAgent,
					"OWASP Top 10: "+cr.rule.Name+" detected in User-Agent",
				))
			}
		}
	}

	return threats, nil
}
