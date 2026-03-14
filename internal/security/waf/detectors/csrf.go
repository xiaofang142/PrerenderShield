package detectors

import (
	"net/http"
	"strings"

	"prerender-shield/internal/security/waf/types"
)

// CSRFDetector CSRF 检测器
type CSRFDetector struct {
	allowedOrigins []string
}

// NewCSRFDetector 创建 CSRF 检测器
func NewCSRFDetector() *CSRFDetector {
	return &CSRFDetector{
		allowedOrigins: make([]string, 0),
	}
}

// SetAllowedOrigins 设置允许的源
func (d *CSRFDetector) SetAllowedOrigins(origins []string) {
	d.allowedOrigins = origins
}

// Check 检查请求
func (d *CSRFDetector) Check(req *http.Request) *types.CheckResult {
	// 只检查修改性请求
	if req.Method == "GET" || req.Method == "HEAD" || req.Method == "OPTIONS" {
		return &types.CheckResult{
			Allowed: true,
		}
	}

	// 检查 Origin 头
	origin := req.Header.Get("Origin")
	if origin != "" {
		if !d.isAllowedOrigin(origin) {
			return &types.CheckResult{
				Allowed: false,
				Blocked: true,
				Reason:  "CSRF detected: invalid origin",
				RuleID:  "csrf-001",
				Threat: &types.Threat{
					Type:     types.ThreatCSRF,
					Severity: "medium",
					Source:   "origin",
				},
			}
		}
	}

	// 检查 Referer 头
	referer := req.Header.Get("Referer")
	if referer != "" {
		if !d.isAllowedOrigin(referer) {
			return &types.CheckResult{
				Allowed: false,
				Blocked: true,
				Reason:  "CSRF detected: invalid referer",
				RuleID:  "csrf-002",
				Threat: &types.Threat{
					Type:     types.ThreatCSRF,
					Severity: "medium",
					Source:   "referer",
				},
			}
		}
	}

	// 检查 CSRF Token
	token := req.Header.Get("X-CSRF-Token")
	if token == "" {
		token = req.FormValue("csrf_token")
	}
	if token == "" {
		// 如果没有 token，标记为需要验证
		return &types.CheckResult{
			Allowed:   true,
			Challenge: true,
			Reason:    "CSRF token required",
		}
	}

	return &types.CheckResult{
		Allowed: true,
	}
}

func (d *CSRFDetector) isAllowedOrigin(origin string) bool {
	if len(d.allowedOrigins) == 0 {
		return true
	}

	for _, allowed := range d.allowedOrigins {
		if strings.EqualFold(origin, allowed) {
			return true
		}
	}

	return false
}
