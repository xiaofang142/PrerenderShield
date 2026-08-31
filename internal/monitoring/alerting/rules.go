package alerting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"prerender-shield/internal/logging"
	"sync"
	"time"
)

// RuleEngine 告警规则引擎
type RuleEngine struct {
	rules    []*Rule
	handlers []AlertHandler
	mu       sync.RWMutex
	stopChan chan struct{}
}

// Rule 告警规则
type Rule struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	Enabled       bool          `json:"enabled"`
	Condition     *Condition    `json:"condition"`
	Severity      string        `json:"severity"` // critical, warning, info
	Handlers      []string      `json:"handlers"`
	Cooldown      time.Duration `json:"cooldown"` // 冷却时间
	lastTriggered time.Time
	mu            sync.Mutex
}

// Condition 告警条件
type Condition struct {
	Metric    string        `json:"metric"`    // 指标名称
	Operator  string        `json:"operator"`  // gt, lt, eq, ge, le
	Threshold float64       `json:"threshold"` // 阈值
	Duration  time.Duration `json:"duration"`  // 持续时间
}

// Alert 告警
type Alert struct {
	ID        string                 `json:"id"`
	RuleID    string                 `json:"rule_id"`
	RuleName  string                 `json:"rule_name"`
	Severity  string                 `json:"severity"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Metric    string                 `json:"metric"`
	Value     float64                `json:"value"`
	Details   map[string]interface{} `json:"details"`
}

// AlertHandler 告警处理器接口
type AlertHandler interface {
	Send(ctx context.Context, alert *Alert) error
	Name() string
}

// MetricsFunc 指标获取函数
type MetricsFunc func(ctx context.Context, metric string) (float64, error)

// NewRuleEngine 创建规则引擎
func NewRuleEngine() *RuleEngine {
	return &RuleEngine{
		rules:    make([]*Rule, 0),
		handlers: make([]AlertHandler, 0),
		stopChan: make(chan struct{}),
	}
}

// AddRule 添加规则
func (e *RuleEngine) AddRule(rule *Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = append(e.rules, rule)
}

// UpdateRule 原子更新规则：同 ID 就地替换，不存在则追加。
// 替代先 Remove 后 Add 的两步操作，避免中间态窗口内评估到缺失规则
func (e *RuleEngine) UpdateRule(rule *Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, r := range e.rules {
		if r.ID == rule.ID {
			e.rules[i] = rule
			return
		}
	}
	e.rules = append(e.rules, rule)
}

// RemoveRule 移除规则
func (e *RuleEngine) RemoveRule(ruleID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, r := range e.rules {
		if r.ID == ruleID {
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			return
		}
	}
}

// GetRules 获取所有规则
func (e *RuleEngine) GetRules() []*Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*Rule, len(e.rules))
	copy(result, e.rules)
	return result
}

// AddHandler 添加处理器
func (e *RuleEngine) AddHandler(handler AlertHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlers = append(e.handlers, handler)
}

// Start 启动规则引擎
func (e *RuleEngine) Start(getMetric MetricsFunc) {
	go e.evaluateLoop(getMetric)
}

// Stop 停止规则引擎
func (e *RuleEngine) Stop() {
	close(e.stopChan)
}

// evaluateLoop 评估循环
func (e *RuleEngine) evaluateLoop(getMetric MetricsFunc) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.evaluateAll(getMetric)
		case <-e.stopChan:
			return
		}
	}
}

// evaluateAll 评估所有规则
func (e *RuleEngine) evaluateAll(getMetric MetricsFunc) {
	e.mu.RLock()
	rules := make([]*Rule, len(e.rules))
	copy(rules, e.rules)
	handlers := e.handlers
	e.mu.RUnlock()

	ctx := context.Background()

	for i := range rules {
		rule := rules[i]
		if !rule.Enabled {
			continue
		}

		// 检查冷却时间
		rule.mu.Lock()
		if time.Since(rule.lastTriggered) < rule.Cooldown {
			rule.mu.Unlock()
			continue
		}
		rule.mu.Unlock()

		// 获取指标值
		value, err := getMetric(ctx, rule.Condition.Metric)
		if err != nil {
			continue
		}

		// 评估条件
		if e.evaluateCondition(rule.Condition, value) {
			alert := &Alert{
				ID:        fmt.Sprintf("alert-%s-%d", rule.ID, time.Now().Unix()),
				RuleID:    rule.ID,
				RuleName:  rule.Name,
				Severity:  rule.Severity,
				Message:   fmt.Sprintf("%s: %s = %.2f", rule.Name, rule.Condition.Metric, value),
				Timestamp: time.Now(),
				Metric:    rule.Condition.Metric,
				Value:     value,
				Details:   map[string]interface{}{"threshold": rule.Condition.Threshold},
			}

			// 发送告警
			for _, handler := range handlers {
				if contains(rule.Handlers, handler.Name()) || len(rule.Handlers) == 0 {
					go func(h AlertHandler) {
						if err := h.Send(ctx, alert); err != nil {
							logging.DefaultLogger.Info("发送告警失败 [%s]: %v\n", h.Name(), err)
						}
					}(handler)
				}
			}

			rule.mu.Lock()
			rule.lastTriggered = time.Now()
			rule.mu.Unlock()
		}
	}
}

// evaluateCondition 评估条件
func (e *RuleEngine) evaluateCondition(cond *Condition, value float64) bool {
	switch cond.Operator {
	case "gt":
		return value > cond.Threshold
	case "lt":
		return value < cond.Threshold
	case "eq":
		return value == cond.Threshold
	case "ge":
		return value >= cond.Threshold
	case "le":
		return value <= cond.Threshold
	default:
		return false
	}
}

// durationValue 兼容两种 JSON 时长编码：Go time.Duration 数值（纳秒）或人类可读字符串（如 "5m"、"1h30s"）。
// 修复：configs/alert-rules.example.json 以字符串时长（"1m"/"5m"）书写，而具名 Rule.Cooldown/Condition.Duration
// 通过 time.Duration 反序列化只接受整数纳秒，导致模板不可被 LoadRulesFromFile 解析。
type durationValue time.Duration

func (d *durationValue) UnmarshalJSON(b []byte) error {
	bs := bytes.TrimSpace(b)
	if len(bs) == 0 || bytes.Equal(bs, []byte("null")) || bytes.Equal(bs, []byte(`""`)) {
		return nil
	}
	// 字符串形式（如 "1m"、"5m"）
	if bs[0] == '"' {
		var s string
		if err := json.Unmarshal(bs, &s); err != nil {
			return err
		}
		if s == "" {
			return nil
		}
		dur, err := time.ParseDuration(s)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", string(bs), err)
		}
		*d = durationValue(dur)
		return nil
	}
	// 数值形式（纳秒）
	var ns int64
	if err := json.Unmarshal(bs, &ns); err != nil {
		return fmt.Errorf("invalid duration %q", string(bs))
	}
	*d = durationValue(ns)
	return nil
}

func (d durationValue) Duration() time.Duration { return time.Duration(d) }

// ruleFile / conditionFile 为 LoadRulesFromFile 的中间结构体，
// 将 Cooldown/Duration 用 durationValue 承载，加载后再转换为逻辑 Rule。
type ruleFile struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Enabled     bool           `json:"enabled"`
	Condition   *conditionFile `json:"condition"`
	Severity    string         `json:"severity"`
	Handlers    []string       `json:"handlers"`
	Cooldown    durationValue  `json:"cooldown"`
}

type conditionFile struct {
	Metric    string        `json:"metric"`
	Operator  string        `json:"operator"`
	Threshold float64       `json:"threshold"`
	Duration  durationValue `json:"duration"`
}

// LoadRulesFromFile 从文件加载规则
func (e *RuleEngine) LoadRulesFromFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var files []ruleFile
	// 兼容两种顶层格式：裸规则数组 [...] 或包装对象 {"rules": [...], "notifications": {...}}
	var wrapper struct {
		Rules []ruleFile `json:"rules"`
	}
	if err := json.Unmarshal(data, &wrapper); err == nil && wrapper.Rules != nil {
		files = wrapper.Rules
	} else {
		if err := json.Unmarshal(data, &files); err != nil {
			return err
		}
	}

	for i := range files {
		f := files[i]
		rule := &Rule{
			ID:          f.ID,
			Name:        f.Name,
			Description: f.Description,
			Enabled:     f.Enabled,
			Severity:    f.Severity,
			Handlers:    f.Handlers,
			Cooldown:    f.Cooldown.Duration(),
		}
		if f.Condition != nil {
			rule.Condition = &Condition{
				Metric:    f.Condition.Metric,
				Operator:  f.Condition.Operator,
				Threshold: f.Condition.Threshold,
				Duration:  f.Condition.Duration.Duration(),
			}
		}
		e.AddRule(rule)
	}

	return nil
}

// SaveRulesToFile 保存规则到文件
func (e *RuleEngine) SaveRulesToFile(filename string) error {
	e.mu.RLock()
	data, err := json.MarshalIndent(e.rules, "", "  ")
	e.mu.RUnlock()
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// 内置规则
func DefaultRules() []*Rule {
	return []*Rule{
		{
			ID:          "cpu_high",
			Name:        "CPU 使用率过高",
			Description: "当 CPU 使用率超过 90% 时触发告警",
			Enabled:     true,
			Condition: &Condition{
				Metric:    "system_cpu_usage",
				Operator:  "gt",
				Threshold: 90,
				Duration:  time.Minute,
			},
			Severity: "warning",
			Cooldown: 5 * time.Minute,
		},
		{
			ID:          "memory_high",
			Name:        "内存使用率过高",
			Description: "当内存使用率超过 85% 时触发告警",
			Enabled:     true,
			Condition: &Condition{
				Metric:    "system_memory_usage",
				Operator:  "gt",
				Threshold: 85,
				Duration:  time.Minute,
			},
			Severity: "warning",
			Cooldown: 5 * time.Minute,
		},
		{
			ID:          "threat_spike",
			Name:        "威胁检测激增",
			Description: "当每分钟检测到的威胁数超过 100 时触发告警",
			Enabled:     true,
			Condition: &Condition{
				Metric:    "threats_per_minute",
				Operator:  "gt",
				Threshold: 100,
				Duration:  30 * time.Second,
			},
			Severity: "critical",
			Cooldown: 10 * time.Minute,
		},
		{
			ID:          "render_queue_backlog",
			Name:        "渲染队列积压",
			Description: "当渲染队列积压超过 50 时触发告警",
			Enabled:     true,
			Condition: &Condition{
				Metric:    "render_queue_size",
				Operator:  "gt",
				Threshold: 50,
				Duration:  2 * time.Minute,
			},
			Severity: "warning",
			Cooldown: 5 * time.Minute,
		},
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return len(slice) == 0
}
