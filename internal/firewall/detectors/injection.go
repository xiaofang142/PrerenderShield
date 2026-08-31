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
	// R12-BUG-3 修复：旧模式为字符级匹配（单字符 ; | & > 即命中），导致主流浏览器
	// User-Agent（Mozilla/5.0 (Macintosh; ...) 等含分号）在 header 检测中被整批 403 误杀。
	// 现改为命令注入组合特征（分隔符+危险命令 / 命令替换 / 敏感路径）。
	defaultRules := []types.Rule{
		{ID: "injection-001", Name: "SQL Injection", Category: "injection", Pattern: `((\x27|\x22)\s*(or|and)\s*(\x27|\x22)?[\w\x27\x22]*\s*=\s*[\w"\x27])|((\x27|\x22)(or|and)(\x27|\x22)\d+(\x27|\x22)=(\x27|\x22)\d)|(\bor\s+\d+\s*=\s*\d)|(union\s+(all\s+)?select)|(--\s*$)|(;\s*drop\s+table)|((\x27|\x22)\s*;)|(select\s+.+\s+from\s+)`, Severity: "high"},
		{ID: "injection-002", Name: "Command Injection", Category: "injection", Pattern: `((\||;|&)\s*(rm|cat|ls|id|pwd|wget|curl|nc|ncat|bash|sh|zsh|ksh|chmod|chown|chgrp|kill|killall|python\d?|perl|php|ruby|lua|whoami|uname|ping|nslookup|dig|grep|egrep|awk|sed|find|touch|mv|cp|dd|echo|env|sleep|head|tail|wc|sort|uniq|xargs|systemctl|service|apt|apt-get|yum|dnf|mkfs|shutdown|reboot|halt|crontab|ssh|scp|ftp|telnet|make|gcc|git)\b)|(\$\([^)]*\))|(\x60[^\x60]*\x60)|(/etc/(passwd|shadow))|(/bin/(ba)?sh)|(\b[\w./-]{1,32}>\s*/?(etc/|tmp/|home/|var/|\w+\.(sh|py|php))|\b(sh|bash|zsh|ksh)\s*<)|(%3B|%7C|%26)`, Severity: "high"},
		{ID: "injection-003", Name: "LDAP Injection", Category: "injection", Pattern: `(\*\)\(\w+=)|(\)\(\s*(&|\||!)\()|(\x2A\)\x28)`, Severity: "medium"},
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
