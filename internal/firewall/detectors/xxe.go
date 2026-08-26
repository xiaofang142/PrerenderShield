package detectors

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"

	"prerender-shield/internal/firewall/types"
	"prerender-shield/internal/logging"
)

// XXEDetector XXE 攻击检测器
type XXEDetector struct {
	rules         []types.Rule
	compiledRules []compiledRule
	rulesMutex    sync.RWMutex
	name          string
}

// NewXXEDetector 创建 XXE 检测器
func NewXXEDetector(ruleProvider RuleProvider) *XXEDetector {
	d := &XXEDetector{
		name: "XXE",
	}

	if ruleProvider != nil {
		d.rules = ruleProvider.GetRulesByCategory("xxe")
	}

	d.compileRules()
	return d
}

func (d *XXEDetector) compileRules() {
	defaultRules := []types.Rule{
		{ID: "xxe-001", Name: "XML Entity Declaration", Category: "xxe", Pattern: `<!ENTITY\s+\w+\s+`, Severity: "high"},
		{ID: "xxe-002", Name: "XML External Entity", Category: "xxe", Pattern: `SYSTEM\s+["']?file:|SYSTEM\s+["']?http:|SYSTEM\s+["']?ftp:`, Severity: "critical"},
		{ID: "xxe-003", Name: "XML DOCTYPE with ENTITY", Category: "xxe", Pattern: `<!DOCTYPE[^>]*\[`, Severity: "high"},
		{ID: "xxe-004", Name: "XML Parameter Entity", Category: "xxe", Pattern: `%\w+\s+SYSTEM`, Severity: "high"},
		{ID: "xxe-005", Name: "PHP XXE Wrapper", Category: "xxe", Pattern: `php://filter|php://input|expect://`, Severity: "critical"},
	}

	allRules := append(d.rules, defaultRules...)
	d.compiledRules = make([]compiledRule, 0, len(allRules))
	for _, rule := range allRules {
		if rule.Pattern == "" {
			continue
		}
		re, err := regexp.Compile(`(?i)` + rule.Pattern)
		if err != nil {
			logging.DefaultLogger.Info("Warning: failed to compile XXE rule %s: %v\n", rule.ID, err)
			continue
		}
		d.compiledRules = append(d.compiledRules, compiledRule{rule: rule, regex: re})
	}
}

func (d *XXEDetector) UpdateRules(rules []types.Rule) error {
	d.rulesMutex.Lock()
	defer d.rulesMutex.Unlock()
	d.rules = rules
	d.compileRules()
	return nil
}

func (d *XXEDetector) Name() string { return d.name }

func (d *XXEDetector) Detect(req *http.Request) ([]types.Threat, error) {
	d.rulesMutex.RLock()
	compiledRules := make([]compiledRule, len(d.compiledRules))
	copy(compiledRules, d.compiledRules)
	d.rulesMutex.RUnlock()

	// 检查 Content-Type 是否为 XML
	contentType := req.Header.Get("Content-Type")
	isXML := len(contentType) > 0 && (regexp.MustCompile(`(?i)xml`).MatchString(contentType))

	threats := checkHTTPInputs(req, compiledRules, "xxe")

	// 如果是 XML 请求，额外检查 Body 中的 XXE 特征
	if isXML && req.Body != nil {
		// 读取 Body 内容进行 XXE 检测
		bodyBytes, err := io.ReadAll(io.LimitReader(req.Body, 1<<20)) // 限制 1MB 防止 OOM
		if err != nil {
			logging.DefaultLogger.Warn("Failed to read request body for XXE detection: %v", err)
		} else {
			// 恢复 Body 供后续中间件使用
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			bodyStr := string(bodyBytes)
			for _, cr := range compiledRules {
				if cr.regex.MatchString(bodyStr) {
					threats = append(threats, types.Threat{
						Type:     "xxe",
						SubType:  cr.rule.Name,
						Severity: cr.rule.Severity,
						Message:  fmt.Sprintf("XXE threat detected in XML body: %s", cr.rule.Name),
						RuleID:   cr.rule.ID,
						RuleName: cr.rule.Name,
					})
				}
			}
		}
	}

	return threats, nil
}
