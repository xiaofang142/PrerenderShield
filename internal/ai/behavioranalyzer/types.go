package behavioranalyzer

import "time"

// RiskLevel 风险等级
type RiskLevel string

const (
	RiskLevelTrusted    RiskLevel = "trusted"    // 可信
	RiskLevelNormal     RiskLevel = "normal"     // 正常
	RiskLevelSuspicious RiskLevel = "suspicious" // 可疑
	RiskLevelMalicious  RiskLevel = "malicious"  // 恶意
)

// Evidence 证据
type Evidence struct {
	Source    string    `json:"source"`    // 证据来源
	Type      string    `json:"type"`      // 证据类型
	Value     string    `json:"value"`     // 证据值
	Weight    float64   `json:"weight"`    // 权重
	Timestamp time.Time `json:"timestamp"` // 时间戳
}
