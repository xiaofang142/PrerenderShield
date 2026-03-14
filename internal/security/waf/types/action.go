package types

// ActionType 动作类型
type ActionType string

const (
	ActionAllow     ActionType = "allow"
	ActionBlock     ActionType = "block"
	ActionChallenge ActionType = "challenge"
	ActionRedirect  ActionType = "redirect"
	ActionRateLimit ActionType = "rate_limit"
)

// Action WAF 动作
type Action struct {
	Type       ActionType        `json:"type"`
	Config     ActionConfig      `json:"config,omitempty"`
	Priority   int               `json:"priority"`
	Conditions []ActionCondition `json:"conditions,omitempty"`
}

// ActionCondition 动作条件
type ActionCondition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// ActionConfig 动作配置
type ActionConfig struct {
	BlockMessage    string         `json:"block_message"`
	RedirectURL     string         `json:"redirect_url"`
	ChallengeType   string         `json:"challenge_type"`
	RateLimitConfig *RateLimitConf `json:"rate_limit_config"`
}

// RateLimitConf 速率限制配置
type RateLimitConf struct {
	Enabled           bool `json:"enabled"`
	RequestsPerMinute int  `json:"requests_per_minute"`
	BurstSize         int  `json:"burst_size"`
}

// ChallengeConfig 挑战配置
type ChallengeConfig struct {
	Type       string `json:"type"`
	Timeout    int    `json:"timeout"`
	MaxRetries int    `json:"max_retries"`
}
