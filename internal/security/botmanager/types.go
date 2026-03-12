package botmanager

import "time"

// Fingerprint 设备指纹
type Fingerprint struct {
	ID             string                 `json:"id"`
	TLSHash        string                 `json:"tls_hash,omitempty"`        // JA3 指纹
	HTTP2Hash      string                 `json:"http2_hash,omitempty"`      // HTTP/2 指纹
	TCPHash        string                 `json:"tcp_hash,omitempty"`        // TCP 栈指纹
	UserAgent      string                 `json:"user_agent"`
	UserAgentHash  string                 `json:"user_agent_hash"`
	DeviceType     string                 `json:"device_type"`               // desktop, mobile, tablet, bot
	OS             string                 `json:"os"`
	OSVersion      string                 `json:"os_version"`
	Browser        string                 `json:"browser"`
	BrowserVersion string                 `json:"browser_version"`
	Device         string                 `json:"device"`
	ScreenRes      string                 `json:"screen_res,omitempty"`
	Timezone       string                 `json:"timezone,omitempty"`
	Language       string                 `json:"language,omitempty"`
	Headers        map[string]string      `json:"headers,omitempty"`
	TLSVersions    []string               `json:"tls_versions,omitempty"`
	TLSCiphers     []string               `json:"tls_ciphers,omitempty"`
	TLSExtensions  []string               `json:"tls_extensions,omitempty"`
	HTTP2Settings  map[string]interface{} `json:"http2_settings,omitempty"`
	TCPWindow      int                    `json:"tcp_window,omitempty"`
	TCPMSS         int                    `json:"tcp_mss,omitempty"`
	TCPOptions     []string               `json:"tcp_options,omitempty"`
	Confidence     float64                `json:"confidence"`                // 置信度 0-1
	IsBot          bool                   `json:"is_bot"`
	BotScore       float64                `json:"bot_score"`                 // 0-100, 越高风险越大
	RiskLevel      RiskLevel              `json:"risk_level"`
	CreatedAt      time.Time              `json:"created_at"`
}

// RiskLevel 风险等级
type RiskLevel string

const (
	RiskLevelTrusted  RiskLevel = "trusted"
	RiskLevelNormal   RiskLevel = "normal"
	RiskLevelSuspicious RiskLevel = "suspicious"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelCritical RiskLevel = "critical"
)

// ChallengeType 挑战类型
type ChallengeType string

const (
	ChallengeJavaScript ChallengeType = "javascript"
	ChallengeCookie     ChallengeType = "cookie"
	ChallengePoW        ChallengeType = "pow"        // Proof of Work
	ChallengeCaptcha    ChallengeType = "captcha"    // 人机验证
)

// ChallengeResult 挑战结果
type ChallengeResult struct {
	Passed      bool                 `json:"passed"`
	ChallengeType ChallengeType      `json:"challenge_type"`
	Score       float64              `json:"score"`
	Duration    time.Duration        `json:"duration"`
	Attempts    int                  `json:"attempts"`
	Error       string               `json:"error,omitempty"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

// DeviceTrust 设备信任状态
type DeviceTrust struct {
	DeviceID      string    `json:"device_id"`
	FingerprintID string    `json:"fingerprint_id"`
	TrustScore    float64   `json:"trust_score"` // 0-100
	TrustLevel    string    `json:"trust_level"` // trusted, verified, unknown, suspicious
	FirstSeen     time.Time `json:"first_seen"`
	LastSeen      time.Time `json:"last_seen"`
	TotalSessions int64     `json:"total_sessions"`
	FailedChallenges int64  `json:"failed_challenges"`
	PassedChallenges int64  `json:"passed_challenges"`
	LastChallenge time.Time `json:"last_challenge,omitempty"`
	LastIP        string    `json:"last_ip"`
	LastUserAgent string    `json:"last_user_agent"`
	IsKnown       bool      `json:"is_known"`
	IsBlocked     bool      `json:"is_blocked"`
	Tags          []string  `json:"tags,omitempty"`
}

// SessionState 会话状态
type SessionState struct {
	SessionID       string                 `json:"session_id"`
	DeviceID        string                 `json:"device_id"`
	UserID          string                 `json:"user_id,omitempty"`
	IP              string                 `json:"ip"`
	UserAgent       string                 `json:"user_agent"`
	StartTime       time.Time              `json:"start_time"`
	LastActivity    time.Time              `json:"last_activity"`
	RequestCount    int64                  `json:"request_count"`
	ChallengePassed bool                   `json:"challenge_passed"`
	ChallengeType   ChallengeType          `json:"challenge_type,omitempty"`
	TrustScore      float64                `json:"trust_score"`
	RiskScore       float64                `json:"risk_score"`
	BehaviorFlags   []string               `json:"behavior_flags,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// BotConfig 机器人管理配置
type BotConfig struct {
	// 指纹配置
	EnableTLSFingerprint    bool          `json:"enable_tls_fingerprint"`
	EnableHTTP2Fingerprint  bool          `json:"enable_http2_fingerprint"`
	EnableTCPFingerprint    bool          `json:"enable_tcp_fingerprint"`
	EnableUserAgentAnalysis bool          `json:"enable_user_agent_analysis"`

	// 挑战配置
	EnableJavaScriptChallenge bool          `json:"enable_javascript_challenge"`
	EnableCookieChallenge     bool          `json:"enable_cookie_challenge"`
	EnablePoWChallenge        bool          `json:"enable_pow_challenge"`
	EnableCaptcha             bool          `json:"enable_captcha"`
	ChallengeTimeout          time.Duration `json:"challenge_timeout"`
	MaxChallengeAttempts      int           `json:"max_challenge_attempts"`

	// 信任配置
	EnableDeviceTrust       bool          `json:"enable_device_trust"`
	TrustDecayRate          float64       `json:"trust_decay_rate"`     // 信任衰减率
	TrustBoostRate          float64       `json:"trust_boost_rate"`     // 信任提升率
	MinTrustScore           float64       `json:"min_trust_score"`      // 最低信任分数
	MaxTrustScore           float64       `json:"max_trust_score"`      // 最高信任分数

	// 阈值配置
	BotScoreThreshold       float64       `json:"bot_score_threshold"`  // 机器人分数阈值
	SuspiciousThreshold     float64       `json:"suspicious_threshold"` // 可疑阈值
	CriticalThreshold       float64       `json:"critical_threshold"`   // 严重阈值

	// 缓存配置
	CacheSize               int           `json:"cache_size"`
	CacheTTL                time.Duration `json:"cache_ttl"`

	// 日志配置
	EnableLogging           bool          `json:"enable_logging"`
}

// DefaultBotConfig 返回默认机器人管理配置
func DefaultBotConfig() *BotConfig {
	return &BotConfig{
		// 指纹配置
		EnableTLSFingerprint:    true,
		EnableHTTP2Fingerprint:  true,
		EnableTCPFingerprint:    false,  // TCP 指纹需要原始 socket 权限
		EnableUserAgentAnalysis: true,

		// 挑战配置
		EnableJavaScriptChallenge: true,
		EnableCookieChallenge:     true,
		EnablePoWChallenge:        true,
		EnableCaptcha:             false,  // 需要第三方服务
		ChallengeTimeout:          30 * time.Second,
		MaxChallengeAttempts:      3,

		// 信任配置
		EnableDeviceTrust: true,
		TrustDecayRate:    0.01,  // 每次请求衰减 1%
		TrustBoostRate:    0.05,  // 通过挑战提升 5%
		MinTrustScore:     0,
		MaxTrustScore:     100,

		// 阈值配置
		BotScoreThreshold:   50,
		SuspiciousThreshold: 70,
		CriticalThreshold:   90,

		// 缓存配置
		CacheSize: 10000,
		CacheTTL:  24 * time.Hour,

		// 日志配置
		EnableLogging: true,
	}
}
