package detectors

import (
	"net/http"
	"regexp"

	"prerender-shield/internal/security/waf/types"
)

// SensitiveDataDetector 敏感数据检测器
type SensitiveDataDetector struct {
	patterns []*regexp.Regexp
}

// NewSensitiveDataDetector 创建敏感数据检测器
func NewSensitiveDataDetector() *SensitiveDataDetector {
	patterns := []string{
		// 身份证号
		`\b[1-9]\d{5}(18|19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`,
		// 手机号
		`\b1[3-9]\d{9}\b`,
		// 银行卡号
		`\b[2-6]\d{13,19}\b`,
		// 密码字段
		`(?i)(password|passwd|pwd|secret|token)\s*[=:]\s*['"]?[^\s'"]+`,
		// API Key
		`(?i)(api[_-]?key|apikey)\s*[=:]\s*['"]?[a-zA-Z0-9]{16,}`,
		// 私有 IP
		`\b(10\.\d{1,3}\.\d{1,3}\.\d{1,3}|172\.(1[6-9]|2[0-9]|3[01])\.\d{1,3}\.\d{1,3}|192\.168\.\d{1,3}\.\d{1,3})\b`,
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}

	return &SensitiveDataDetector{
		patterns: compiled,
	}
}

// Check 检查请求
func (d *SensitiveDataDetector) Check(req *http.Request) *types.CheckResult {
	// 检查 URL 参数
	query := req.URL.RawQuery
	if result := d.checkString(query); result != nil {
		result.Threat.Source = "query"
		return result
	}

	// 检查 URL 路径
	if result := d.checkString(req.URL.Path); result != nil {
		result.Threat.Source = "path"
		return result
	}

	return &types.CheckResult{
		Allowed: true,
	}
}

func (d *SensitiveDataDetector) checkString(s string) *types.CheckResult {
	if s == "" {
		return nil
	}

	decoded := decodeURL(s)

	for _, pattern := range d.patterns {
		if pattern.MatchString(decoded) {
			return &types.CheckResult{
				Allowed: false,
				Blocked: true,
				Reason:  "Sensitive data detected",
				RuleID:  "sensitive-001",
				Threat: &types.Threat{
					Type:     types.ThreatSensitiveData,
					Severity: "medium",
				},
			}
		}
	}

	return nil
}
