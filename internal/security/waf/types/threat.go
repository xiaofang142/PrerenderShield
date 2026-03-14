package types

import "time"

// ThreatType 威胁类型
type ThreatType string

const (
	ThreatSQLInjection     ThreatType = "sql_injection"
	ThreatXSS              ThreatType = "xss"
	ThreatCSRF             ThreatType = "csrf"
	ThreatRCE              ThreatType = "rce"
	ThreatPathTraversal    ThreatType = "path_traversal"
	ThreatFileInclusion    ThreatType = "file_inclusion"
	ThreatSensitiveData    ThreatType = "sensitive_data"
	ThreatMaliciousIP      ThreatType = "malicious_ip"
	ThreatRateLimit        ThreatType = "rate_limit"
	ThreatGeoIPBlock       ThreatType = "geo_ip_block"
)

// Threat 威胁信息
type Threat struct {
	ID        string            `json:"id"`
	Type      ThreatType        `json:"type"`
	Severity  string            `json:"severity"`
	Source    string            `json:"source"`
	Rules     []string          `json:"rules"`
	Timestamp time.Time         `json:"timestamp"`
	Details   map[string]string `json:"details,omitempty"`
}

// Rule WAF 规则
type Rule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Severity    string            `json:"severity"`
	Pattern     string            `json:"pattern"`
	Action      string            `json:"action"`
	Enabled     bool              `json:"enabled"`
	Tags        []string          `json:"tags"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// CheckResult 检查结果
type CheckResult struct {
	Allowed   bool     `json:"allowed"`
	Blocked   bool     `json:"blocked"`
	Challenge bool     `json:"challenge"`
	Reason    string   `json:"reason,omitempty"`
	RuleID    string   `json:"rule_id,omitempty"`
	Threat    *Threat  `json:"threat,omitempty"`
}

// GeoIPConfig GeoIP 配置
type GeoIPConfig struct {
	Enabled          bool     `json:"enabled"`
	DatabasePath     string   `json:"database_path"`
	AllowedCountries []string `json:"allowed_countries"`
	BlockedCountries []string `json:"blocked_countries"`
}

// RateLimitConfig 速率限制配置
type RateLimitConfig struct {
	Enabled           bool `json:"enabled"`
	RequestsPerMinute int  `json:"requests_per_minute"`
	BurstSize         int  `json:"burst_size"`
}

// FileIntegrityConfig 文件完整性配置
type FileIntegrityConfig struct {
	Enabled       bool     `json:"enabled"`
	WatchPaths    []string `json:"watch_paths"`
	HashAlgorithm string   `json:"hash_algorithm"`
	CheckInterval int      `json:"check_interval"`
}
