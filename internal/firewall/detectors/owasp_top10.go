package detectors

import (
	"net/http"
	"regexp"
	"strings"
	"sync"

	"prerender-shield/internal/firewall/types"
)

// compiledRule 预编译的规则
type compiledRule struct {
	rule    types.Rule
	regex   *regexp.Regexp
}

// OWASPTop10Detector OWASP Top 10 攻击检测器
type OWASPTop10Detector struct {
	rules         []types.Rule
	compiledRules []compiledRule
	initOnce      sync.Once
}

// NewOWASPTop10Detector 创建 OWASP Top 10 检测器
func NewOWASPTop10Detector(ruleManager interface {
	GetRulesByCategory(category string) []types.Rule
}) *OWASPTop10Detector {
	d := &OWASPTop10Detector{
		rules: ruleManager.GetRulesByCategory("owasp_top10"),
	}
	d.precompileRules()
	return d
}

// precompileRules 预编译正则表达式
func (d *OWASPTop10Detector) precompileRules() {
	d.initOnce.Do(func() {
		// OWASP Top 10 2021 规则
		owaspRules := []types.Rule{
			// A01: Broken Access Control
			{ID: "owasp-a01", Name: "Path Traversal", Category: "owasp_top10", Pattern: `\.\\./|%2e%2e%2f|%2e%2e/|\\.\\./|%2e%2e\\`, Severity: "high"},
			{ID: "owasp-a01-2", Name: "Admin Access Attempt", Category: "owasp_top10", Pattern: `(/admin|/administrator|/wp-admin|/phpmyadmin|/manager)`, Severity: "medium"},

			// A02: Cryptographic Failures
			{ID: "owasp-a02", Name: "Sensitive Data Exposure", Category: "owasp_top10", Pattern: `(passwd|password|secret|private|credential|token)\\s*=\\s*[^&]+`, Severity: "high"},

			// A04: Insecure Design
			{ID: "owasp-a04", Name: "Mass Assignment", Category: "owasp_top10", Pattern: `(__proto__|prototype|constructor)`, Severity: "medium"},

			// A05: Security Misconfiguration
			{ID: "owasp-a05", Name: "Server Info Disclosure", Category: "owasp_top10", Pattern: `(server-tokens|X-Powered-By|PHP/)`, Severity: "low"},

			// A06: Vulnerable and Outdated Components
			{ID: "owasp-a06", Name: "Vulnerability Scanner", Category: "owasp_top10", Pattern: `(nikto|nmap|nessus|openvas|w3af|wpscan|sqlmap|burp)`, Severity: "high"},

			// A08: Software and Data Integrity Failures
			{ID: "owasp-a08", Name: "Untrusted Data", Category: "owasp_top10", Pattern: `(php://|file://|gopher://|dict://)`, Severity: "high"},

			// A10: Server-Side Request Forgery (SSRF)
			{ID: "owasp-a10", Name: "SSRF Attempt", Category: "owasp_top10", Pattern: `(127\\.0\\.0\\.1|localhost|0\\.0\\.0\\.0|169\\.254\\.|\\[::1\\]|metadata\\.google)`, Severity: "critical"},
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
				continue
			}
			d.compiledRules = append(d.compiledRules, compiledRule{
				rule:  rule,
				regex: re,
			})
		}
	})
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

// Detect 检测 OWASP Top 10 攻击
func (d *OWASPTop10Detector) Detect(req *http.Request) ([]types.Threat, error) {
	threats := make([]types.Threat, 0)

	// 确保规则已预编译
	d.precompileRules()

	// 检查 URL
	urlStr := req.URL.String()
	for _, cr := range d.compiledRules {
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
			for _, cr := range d.compiledRules {
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
	for _, cr := range d.compiledRules {
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

// SSRFDetector SSRF 攻击检测器
type SSRFDetector struct {
	allowedHosts map[string]bool
	denyInternal bool
}

// NewSSRFDetector 创建 SSRF 检测器
func NewSSRFDetector(allowedHosts []string, denyInternal bool) *SSRFDetector {
	hostMap := make(map[string]bool)
	for _, h := range allowedHosts {
		hostMap[h] = true
	}
	return &SSRFDetector{
		allowedHosts: hostMap,
		denyInternal: denyInternal,
	}
}

// Detect 检测 SSRF 攻击
func (d *SSRFDetector) Detect(req *http.Request) ([]types.Threat, error) {
	threats := make([]types.Threat, 0)

	// 检查 URL 参数中是否有可疑的内部地址
	params := req.URL.Query()
	for name, values := range params {
		for _, value := range values {
			if d.isInternalAddress(value) {
				threats = append(threats, types.Threat{
					Type:      "ssrf",
					SubType:   "Internal Address Access",
					Severity:  "critical",
					Message:   "SSRF: Attempt to access internal address",
					Parameter: name,
					Value:     value,
					RuleID:    "ssrf-001",
					RuleName:  "SSRF Internal Access",
				})
			}
		}
	}

	return threats, nil
}

// isInternalAddress 检查是否是内部地址
var internalRegexes []*regexp.Regexp

func init() {
	internalPatterns := []string{
		`127\.0\.0\.1`,
		`localhost`,
		`0\.0\.0\.0`,
		`169\.254\.`,
		`10\.\d+\.\d+\.\d+`,
		`172\.(1[6-9]|2[0-9]|3[0-1])\.\d+\.\d+`,
		`192\.168\.\d+\.\d+`,
		`\[::1\]`,
		`metadata\.google`,
		`169\.254\.169\.254`,
	}
	for _, pattern := range internalPatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			internalRegexes = append(internalRegexes, re)
		}
	}
}

func (d *SSRFDetector) isInternalAddress(url string) bool {
	for _, re := range internalRegexes {
		if re.MatchString(url) {
			return true
		}
	}
	return false
}
