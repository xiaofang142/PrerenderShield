package detectors

import (
	"net/http"
	"regexp"

	"prerender-shield/internal/security/waf/types"
)

// XSSDetector XSS 检测器
type XSSDetector struct {
	patterns []*regexp.Regexp
}

// NewXSSDetector 创建 XSS 检测器
func NewXSSDetector() *XSSDetector {
	patterns := []string{
		`(?i)(<script.*?>)`,
		`(?i)(</script>)`,
		`(?i)(javascript:)`,
		`(?i)(on\w+\s*=)`,
		`(?i)(<img.*?onerror)`,
		`(?i)(<svg.*?onload)`,
		`(?i)(<iframe)`,
		`(?i)(<object)`,
		`(?i)(<embed)`,
		`(?i)(document\.(cookie|write|location))`,
		`(?i)(window\.(location|open))`,
		`(?i)(eval\s*\()`,
		`(?i)(alert\s*\()`,
		`(?i)(prompt\s*\()`,
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}

	return &XSSDetector{
		patterns: compiled,
	}
}

// Check 检查请求
func (d *XSSDetector) Check(req *http.Request) *types.CheckResult {
	// 检查 URL 参数
	query := req.URL.RawQuery
	if d.checkString(query) {
		return &types.CheckResult{
			Allowed: false,
			Blocked: true,
			Reason:  "XSS detected in query string",
			RuleID:  "xss-001",
			Threat: &types.Threat{
				Type:     types.ThreatXSS,
				Severity: "high",
				Source:   "query",
			},
		}
	}

	// 检查 URL 路径
	if d.checkString(req.URL.Path) {
		return &types.CheckResult{
			Allowed: false,
			Blocked: true,
			Reason:  "XSS detected in path",
			RuleID:  "xss-002",
			Threat: &types.Threat{
				Type:     types.ThreatXSS,
				Severity: "high",
				Source:   "path",
			},
		}
	}

	return &types.CheckResult{
		Allowed: true,
	}
}

func (d *XSSDetector) checkString(s string) bool {
	if s == "" {
		return false
	}

	decoded := decodeURL(s)

	for _, pattern := range d.patterns {
		if pattern.MatchString(decoded) {
			return true
		}
	}

	return false
}
