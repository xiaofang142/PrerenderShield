package alerting

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// RuleEngine 告警规则引擎
type RuleEngine struct {
	rules    []Rule
	handlers []AlertHandler
	mu       sync.RWMutex
	stopChan chan struct{}
}

// Rule 告警规则
type Rule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Enabled     bool          `json:"enabled"`
	Condition   *Condition    `json:"condition"`
	Severity    string        `json:"severity"` // critical, warning, info
	Handlers    []string      `json:"handlers"`
	Cooldown    time.Duration `json:"cooldown"` // 冷却时间
	lastTriggered time.Time
	mu          sync.Mutex
}

// Condition 告警条件
type Condition struct {
	Metric    string      `json:"metric"`    // 指标名称
	Operator  string      `json:"operator"`  // gt, lt, eq, ge, le
	Threshold float64     `json:"threshold"` // 阈值
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
		rules:    make([]Rule, 0),
		handlers: make([]AlertHandler, 0),
		stopChan: make(chan struct{}),
	}
}

// AddRule 添加规则
func (e *RuleEngine) AddRule(rule Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
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
	rules := make([]Rule, len(e.rules))
	copy(rules, e.rules)
	handlers := e.handlers
	e.mu.RUnlock()

	ctx := context.Background()

	for i := range rules {
		rule := &rules[i]
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
							fmt.Printf("发送告警失败 [%s]: %v\n", h.Name(), err)
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

// LoadRulesFromFile 从文件加载规则
func (e *RuleEngine) LoadRulesFromFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var rules []Rule
	if err := json.Unmarshal(data, &rules); err != nil {
		return err
	}

	for _, rule := range rules {
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
func DefaultRules() []Rule {
	return []Rule{
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
