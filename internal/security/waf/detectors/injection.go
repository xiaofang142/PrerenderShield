package detectors

import (
	"net/http"
	"regexp"
	"strings"

	"prerender-shield/internal/security/waf/types"
)

// SQLInjectionDetector SQL 注入检测器
type SQLInjectionDetector struct {
	patterns []*regexp.Regexp
}

// NewSQLInjectionDetector 创建 SQL 注入检测器
func NewSQLInjectionDetector() *SQLInjectionDetector {
	patterns := []string{
		`(?i)(\bunion\b.*\bselect\b)`,
		`(?i)(\bselect\b.*\bfrom\b)`,
		`(?i)(\binsert\b.*\binto\b)`,
		`(?i)(\bupdate\b.*\bset\b)`,
		`(?i)(\bdelete\b.*\bfrom\b)`,
		`(?i)(\bdrop\b.*\b(table|database)\b)`,
		`(?i)(--|\#|\/\*|\*\/)`,
		`(?i)(\bor\b.*=.*|\band\b.*=.*)`,
		`(?i)(;.*\b(or|select|union|drop)\b)`,
		`(?i)('.*'.*=.*')`,
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}

	return &SQLInjectionDetector{
		patterns: compiled,
	}
}

// Check 检查请求
func (d *SQLInjectionDetector) Check(req *http.Request) *types.CheckResult {
	// 检查 URL 参数
	query := req.URL.RawQuery
	if d.checkString(query) {
		return &types.CheckResult{
			Allowed: false,
			Blocked: true,
			Reason:  "SQL injection detected in query string",
			RuleID:  "sqli-001",
			Threat: &types.Threat{
				Type:     types.ThreatSQLInjection,
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
			Reason:  "SQL injection detected in path",
			RuleID:  "sqli-002",
			Threat: &types.Threat{
				Type:     types.ThreatSQLInjection,
				Severity: "high",
				Source:   "path",
			},
		}
	}

	return &types.CheckResult{
		Allowed: true,
	}
}

func (d *SQLInjectionDetector) checkString(s string) bool {
	if s == "" {
		return false
	}

	// URL 解码
	decoded := decodeURL(s)

	for _, pattern := range d.patterns {
		if pattern.MatchString(decoded) {
			return true
		}
	}

	return false
}

func decodeURL(s string) string {
	r := strings.NewReplacer(
		"%27", "'",
		"%22", "\"",
		"%3D", "=",
		"%26", "&",
		"%3B", ";",
		"%2D", "-",
		"%20", " ",
		"%2B", "+",
		"%3C", "<",
		"%3E", ">",
	)
	return r.Replace(s)
}
