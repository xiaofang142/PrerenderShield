package ai

import (
	"time"
)

// Config AI威胁检测器配置
type Config struct {
	// 模型配置
	ModelPath string `yaml:"model_path" json:"model_path"` // 模型文件路径

	// 工作池配置
	WorkerPool int `yaml:"worker_pool" json:"worker_pool"` // 预测工作协程数量

	// 检测阈值
	ConfidenceThreshold float32 `yaml:"confidence_threshold" json:"confidence_threshold"` // 置信度阈值

	// 超时配置
	PredictTimeout time.Duration `yaml:"predict_timeout" json:"predict_timeout"` // 预测超时时间

	// 缓存配置
	CacheSize   int           `yaml:"cache_size" json:"cache_size"`     // 特征缓存大小
	CacheTTL    time.Duration `yaml:"cache_ttl" json:"cache_ttl"`       // 特征缓存过期时间
	FeatureSize int           `yaml:"feature_size" json:"feature_size"` // 特征向量大小

	// 模型更新配置
	AutoUpdate     bool          `yaml:"auto_update" json:"auto_update"`           // 启用自动更新
	UpdateInterval time.Duration `yaml:"update_interval" json:"update_interval"`   // 更新间隔
	RemoteModelURL string        `yaml:"remote_model_url" json:"remote_model_url"` // 远程模型URL

	// 启用标志
	Enabled bool `yaml:"enabled" json:"enabled"` // 是否启用AI检测器
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		ModelPath:           "./data/models/threat_detection",
		WorkerPool:          4,
		ConfidenceThreshold: 0.85,
		PredictTimeout:      50 * time.Millisecond,
		CacheSize:           10000,
		CacheTTL:            5 * time.Minute,
		FeatureSize:         128,
		AutoUpdate:          true,
		UpdateInterval:      24 * time.Hour,
		Enabled:             false,
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.ModelPath == "" {
		return ErrModelPathEmpty
	}
	if c.WorkerPool <= 0 {
		c.WorkerPool = 4
	}
	if c.ConfidenceThreshold <= 0 || c.ConfidenceThreshold > 1 {
		c.ConfidenceThreshold = 0.85
	}
	if c.PredictTimeout <= 0 {
		c.PredictTimeout = 50 * time.Millisecond
	}
	if c.CacheSize <= 0 {
		c.CacheSize = 10000
	}
	if c.FeatureSize <= 0 {
		c.FeatureSize = 128
	}
	return nil
}

// ThreatTypeConfig 威胁类型配置
type ThreatTypeConfig struct {
	Name        string  `yaml:"name" json:"name"`               // 威胁类型名称
	Label       string  `yaml:"label" json:"label"`             // 模型标签
	Threshold   float32 `yaml:"threshold" json:"threshold"`     // 检测阈值
	Severity    string  `yaml:"severity" json:"severity"`       // 严重程度
	Description string  `yaml:"description" json:"description"` // 描述
}

// ThreatTypes 预定义的威胁类型
var ThreatTypes = map[string]ThreatTypeConfig{
	"sql_injection": {
		Name:        "SQL Injection",
		Label:       "sql_injection",
		Threshold:   0.8,
		Severity:    "high",
		Description: "SQL injection attack detected",
	},
	"xss": {
		Name:        "Cross-Site Scripting",
		Label:       "xss",
		Threshold:   0.8,
		Severity:    "high",
		Description: "XSS attack detected",
	},
	"command_injection": {
		Name:        "Command Injection",
		Label:       "command_injection",
		Threshold:   0.85,
		Severity:    "critical",
		Description: "Command injection attack detected",
	},
	"path_traversal": {
		Name:        "Path Traversal",
		Label:       "path_traversal",
		Threshold:   0.8,
		Severity:    "high",
		Description: "Path traversal attack detected",
	},
	"ssrf": {
		Name:        "Server-Side Request Forgery",
		Label:       "ssrf",
		Threshold:   0.75,
		Severity:    "high",
		Description: "SSRF attack detected",
	},
	"xxe": {
		Name:        "XML External Entity",
		Label:       "xxe",
		Threshold:   0.8,
		Severity:    "high",
		Description: "XXE attack detected",
	},
	"bot": {
		Name:        "Malicious Bot",
		Label:       "bot",
		Threshold:   0.7,
		Severity:    "medium",
		Description: "Malicious bot detected",
	},
	"scanner": {
		Name:        "Vulnerability Scanner",
		Label:       "scanner",
		Threshold:   0.7,
		Severity:    "medium",
		Description: "Vulnerability scanner detected",
	},
	"benign": {
		Name:        "Benign",
		Label:       "benign",
		Threshold:   0.0,
		Severity:    "none",
		Description: "Normal request",
	},
}

// GetSeverityByConfidence 根据置信度返回严重程度
func GetSeverityByConfidence(confidence float32) string {
	if confidence >= 0.95 {
		return "critical"
	} else if confidence >= 0.85 {
		return "high"
	} else if confidence >= 0.7 {
		return "medium"
	} else if confidence >= 0.5 {
		return "low"
	}
	return "info"
}
