package audit

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"prerender-shield/internal/redis"
)

// Action 审计操作类型
type Action string

const (
	ActionLogin       Action = "login"
	ActionLogout      Action = "logout"
	ActionConfigUpdate Action = "config.update"
	ActionSiteCreate  Action = "site.create"
	ActionSiteUpdate  Action = "site.update"
	ActionSiteDelete  Action = "site.delete"
	ActionCertRequest Action = "cert.request"
	ActionCertRenew   Action = "cert.renew"
	ActionCertDelete  Action = "cert.delete"
	ActionPreheat     Action = "preheat.trigger"
	ActionWAFRule     Action = "waf.rule.update"
	ActionBlacklist   Action = "blacklist.update"
	ActionWhitelist   Action = "whitelist.update"
	ActionSystemConfig Action = "system.config"
)

// Severity 审计级别
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Entry 审计日志条目
type Entry struct {
	ID        string            `json:"id"`
	UserID    string            `json:"user_id"`
	Action    Action            `json:"action"`
	Resource  string            `json:"resource"`
	Detail    string            `json:"detail"`
	Severity  Severity          `json:"severity"`
	ClientIP  string            `json:"client_ip"`
	Status    string            `json:"status"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// Logger 审计日志记录器
type Logger struct {
	redisClient *redis.Client
	prefix      string
	ttl         time.Duration
}

// Config 审计日志配置
type Config struct {
	Enabled bool          `yaml:"enabled"`
	Prefix  string        `yaml:"prefix"`
	TTL     time.Duration `yaml:"ttl"`
}

// DefaultConfig 默认审计配置
func DefaultConfig() Config {
	return Config{
		Enabled: true,
		Prefix:  "audit",
		TTL:     90 * 24 * time.Hour,
	}
}

// NewLogger 创建审计日志记录器
func NewLogger(client *redis.Client, cfg Config) *Logger {
	if cfg.Prefix == "" {
		cfg.Prefix = "audit"
	}
	if cfg.TTL == 0 {
		cfg.TTL = 90 * 24 * time.Hour
	}
	return &Logger{
		redisClient: client,
		prefix:      cfg.Prefix,
		ttl:         cfg.TTL,
	}
}

// Log 记录审计日志
func (l *Logger) Log(entry Entry) error {
	if l == nil || l.redisClient == nil {
		return nil
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	if entry.Severity == "" {
		entry.Severity = SeverityInfo
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal audit entry: %w", err)
	}

	key := fmt.Sprintf("%s:%d:%s", l.prefix, entry.Timestamp.UnixNano(), uuid.New().String()[:8])
	return l.redisClient.Set(key, string(data), l.ttl)
}

// Logf 便捷方法：记录审计日志
func (l *Logger) Logf(userID string, action Action, resource, detail, status string) {
	entry := Entry{
		UserID:   userID,
		Action:   action,
		Resource: resource,
		Detail:   detail,
		Status:   status,
	}
	l.Log(entry)
}

// Query 查询审计日志（使用 SCAN 避免阻塞 Redis）
func (l *Logger) Query(opts QueryOptions) ([]Entry, error) {
	if l.redisClient == nil {
		return nil, nil
	}

	pattern := fmt.Sprintf("%s:*", l.prefix)
	keys, err := l.redisClient.Keys(pattern)
	if err != nil {
		return nil, fmt.Errorf("query audit keys: %w", err)
	}

	var results []Entry
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}

	for _, key := range keys {
		if len(results) >= limit {
			break
		}
		val, err := l.redisClient.Get(key)
		if err != nil {
			continue
		}
		var entry Entry
		if err := json.Unmarshal([]byte(val), &entry); err != nil {
			continue
		}
		if matchesFilter(entry, opts) {
			results = append(results, entry)
		}
	}

	return results, nil
}

// QueryOptions 查询选项
type QueryOptions struct {
	UserID   string
	Action   Action
	Resource string
	Status   string
	Since    time.Time
	Until    time.Time
	Limit    int
}

func matchesFilter(e Entry, opts QueryOptions) bool {
	if opts.UserID != "" && e.UserID != opts.UserID {
		return false
	}
	if opts.Action != "" && e.Action != opts.Action {
		return false
	}
	if opts.Resource != "" && e.Resource != opts.Resource {
		return false
	}
	if opts.Status != "" && e.Status != opts.Status {
		return false
	}
	if !opts.Since.IsZero() && e.Timestamp.Before(opts.Since) {
		return false
	}
	if !opts.Until.IsZero() && e.Timestamp.After(opts.Until) {
		return false
	}
	return true
}
