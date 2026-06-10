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

	if req.Method == http.MethodGet || req.Method == http.MethodHead || req.Method == http.MethodOptions {
		return threats, nil
	}

	// SPA 应用通过 Authorization header 携带 JWT，说明这是 JS API 调用
	// 浏览器不会自动附加 Authorization header 到跨站请求，
	// 因此存在有效 Authorization header 即可确认请求来源
	hasAuth := req.Header.Get("Authorization") != ""

	// 支持 URL query 中的 CSRF token (传统表单)
	csrfToken := req.URL.Query().Get("csrf_token")
	if csrfToken == "" {
		csrfToken = req.Header.Get("X-CSRF-Token")
		if csrfToken == "" {
			csrfToken = req.Header.Get("X-XSRF-Token")
		}
	}

	// 如果没有 CSRF token 也没有 Authorization header，报告缺失
	if csrfToken == "" && !hasAuth {
		threats = append(threats, types.Threat{
			Type:      "csrf",
			SubType:   "Missing CSRF Token",
			Severity:  "high",
			Message:   "Missing CSRF token in request",
			Parameter: "X-CSRF-Token",
			Value:     "",
			RuleID:    "csrf-001",
			RuleName:  "Missing CSRF Token",
		})
	}

	// 验证 Origin / Referer 一致性
	origin := req.Header.Get("Origin")
	host := req.Header.Get("Host")
	if origin == "" {
		referer := req.Header.Get("Referer")
		if referer != "" && host != "" {
			refererHost := extractHost(referer)
			if refererHost != "" && refererHost != host {
				threats = append(threats, types.Threat{
					Type:      "csrf",
					SubType:   "Invalid Referer",
					Severity:  "medium",
					Message:   "Referer does not match host",
					Parameter: "Referer",
					Value:     referer,
					RuleID:    "csrf-003",
					RuleName:  "Invalid Referer",
				})
			}
		}
	} else if host != "" {
		originHost := extractHost(origin)
		hostName := extractHost(host)
		if originHost != "" && hostName != "" && originHost != hostName {
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

	return threats, nil
}

// extractHost 从 URL 或 Host 中提取纯主机名
func extractHost(input string) string {
	// 移除协议前缀
	host := strings.TrimPrefix(input, "http://")
	host = strings.TrimPrefix(host, "https://")
	// 移除路径
	if idx := strings.Index(host, "/"); idx > 0 {
		host = host[:idx]
	}
	// 移除端口
	if idx := strings.Index(host, ":"); idx > 0 {
		host = host[:idx]
	}
	return host
}
