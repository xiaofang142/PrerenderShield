package detectors

import (
	"net/http"
	"strings"

	"prerender-shield/internal/firewall/types"
)

// CSRFDetector 跨站请求伪造检测器
type CSRFDetector struct {
	rules []types.Rule
}

// NewCSRFDetector 创建新的CSRF检测器
func NewCSRFDetector(ruleManager interface{ GetRulesByCategory(category string) []types.Rule }) *CSRFDetector {
	return &CSRFDetector{
		rules: ruleManager.GetRulesByCategory("csrf"),
	}
}

// Detect 检测CSRF攻击
func (d *CSRFDetector) Detect(req *http.Request) ([]types.Threat, error) {
	threats := make([]types.Threat, 0)

	// 默认的CSRF规则（如果没有从规则文件加载）
	defaultRules := []types.Rule{
		{ID: "csrf-001", Name: "Missing CSRF Token", Category: "csrf", Pattern: "", Severity: "high"},
		{ID: "csrf-002", Name: "Invalid Origin Header", Category: "csrf", Pattern: "", Severity: "high"},
		{ID: "csrf-003", Name: "Invalid Referer Header", Category: "csrf", Pattern: "", Severity: "medium"},
	}

	// 使用默认规则或从规则管理器加载的规则
	rules := d.rules
	if len(rules) == 0 {
		rules = defaultRules
	}

	// 检查非GET请求的CSRF保护
	if req.Method != "GET" && req.Method != "HEAD" && req.Method != "OPTIONS" {
		// 检查CSRF Token
		csrfToken := req.FormValue("csrf_token")
		if csrfToken == "" {
			// 也检查请求头中的CSRF Token
			csrfToken = req.Header.Get("X-CSRF-Token")
			if csrfToken == "" {
				csrfToken = req.Header.Get("X-XSRF-Token")
				if csrfToken == "" {
					threats = append(threats, types.Threat{
						Type:      "csrf",
						SubType:   "Missing CSRF Token",
						Severity:  "high",
						Message:   "Missing CSRF token in request",
						Parameter: "csrf_token",
						Value:     "",
						RuleID:    "csrf-001",
						RuleName:  "Missing CSRF Token",
					})
				}
			}
		}

		// 检查Origin Header
		origin := req.Header.Get("Origin")
		host := req.Header.Get("Host")
		if origin != "" && host != "" {
			// 检查Origin是否与Host匹配
			originHost := strings.TrimPrefix(origin, "http://")
			originHost = strings.TrimPrefix(originHost, "https://")
			originHost = strings.Split(originHost, ":")[0] // 去掉端口

			hostName := strings.Split(host, ":")[0] // 去掉端口

			if originHost != hostName {
				threats = append(threats, types.Threat{
					Type:      "csrf",
					SubType:   "Invalid Origin Header",
					Severity:  "high",
					Message:   "Origin header does not match host",
					Parameter: "Origin",
					Value:     origin,
					RuleID:    "csrf-002",
					RuleName:  "Invalid Origin Header",
				})
			}
		}

		// 检查Referer Header
		referer := req.Header.Get("Referer")
		if referer != "" && host != "" {
			// 检查Referer是否与Host匹配
			refererHost := strings.TrimPrefix(referer, "http://")
			refererHost = strings.TrimPrefix(refererHost, "https://")
			refererHost = strings.Split(refererHost, "/")[0] // 去掉路径
			refererHost = strings.Split(refererHost, ":")[0] // 去掉端口

			hostName := strings.Split(host, ":")[0] // 去掉端口

			if refererHost != hostName {
				threats = append(threats, types.Threat{
					Type:      "csrf",
					SubType:   "Invalid Referer Header",
					Severity:  "medium",
					Message:   "Referer header does not match host",
					Parameter: "Referer",
					Value:     referer,
					RuleID:    "csrf-003",
					RuleName:  "Invalid Referer Header",
				})
			}
		}
	}

	return threats, nil
}

// Name 返回检测器名称
func (d *CSRFDetector) Name() string {
	return "csrf_detector"
}