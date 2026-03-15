package detectors

import (
	"net/http"
	"strings"

	"prerender-shield/internal/firewall/types"
)

// CSRFDetector 跨站请求伪造检测器
type CSRFDetector struct {
	name string
}

// NewCSRFDetector 创建新的 CSRF 检测器
func NewCSRFDetector(ruleProvider interface {
	GetRulesByCategory(category string) []types.Rule
}) *CSRFDetector {
	return &CSRFDetector{
		name: "CSRF",
	}
}

// Name 返回检测器名称
func (d *CSRFDetector) Name() string {
	return d.name
}

// Detect 检测 CSRF 攻击
func (d *CSRFDetector) Detect(req *http.Request) ([]types.Threat, error) {
	threats := make([]types.Threat, 0)

	// 检查非 GET 请求的 CSRF 保护
	if req.Method != http.MethodGet && req.Method != http.MethodHead && req.Method != http.MethodOptions {
		// 检查 CSRF Token
		csrfToken := req.FormValue("csrf_token")
		if csrfToken == "" {
			// 也检查请求头中的 CSRF Token
			csrfToken = req.Header.Get("X-CSRF-Token")
			if csrfToken == "" {
				csrfToken = req.Header.Get("X-XSRF-Token")
			}
		}

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

		// 检查 Origin Header
		origin := req.Header.Get("Origin")
		host := req.Header.Get("Host")
		if origin != "" && host != "" {
			originHost := strings.TrimPrefix(origin, "http://")
			originHost = strings.TrimPrefix(originHost, "https://")
			originHost = strings.Split(originHost, ":")[0]
			hostName := strings.Split(host, ":")[0]

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
	}

	return threats, nil
}
