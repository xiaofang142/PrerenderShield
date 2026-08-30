package alerting

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// RetryItem 重试项
type RetryItem struct {
	Alert      *Alert
	RetryCount int
	NextRetry  time.Time
}

// WebhookHandler Webhook 告警处理器
type WebhookHandler struct {
	config     *WebhookConfig
	client     *http.Client
	mu         sync.Mutex
	retryQueue []RetryItem
	stats      *WebhookStats
	// provider 非空时每次告警读取最新配置（控制台改动即时生效，免重启）
	provider func() *WebhookConfig
}

// SetConfigProvider 注入动态配置来源
func (h *WebhookHandler) SetConfigProvider(fn func() *WebhookConfig) { h.provider = fn }

// currentConfig 当前生效配置（nil 接收者安全）
func (h *WebhookHandler) currentConfig() *WebhookConfig {
	if h == nil {
		return nil
	}
	if h.provider != nil {
		if c := h.provider(); c != nil {
			return c
		}
	}
	return h.config
}

// WebhookConfig Webhook 配置
type WebhookConfig struct {
	URL        string            `json:"url"`
	Method     string            `json:"method"`
	Headers    map[string]string `json:"headers"`
	Timeout    time.Duration     `json:"timeout"`
	MaxRetries int               `json:"max_retries"`
	RetryDelay time.Duration     `json:"retry_delay"`
	Secret     string            `json:"secret"` // 用于签名
}

// WebhookStats Webhook 统计
type WebhookStats struct {
	TotalSent     int64     `json:"total_sent"`
	SuccessCount  int64     `json:"success_count"`
	FailureCount  int64     `json:"failure_count"`
	LastSentAt    time.Time `json:"last_sent_at"`
	LastSuccessAt time.Time `json:"last_success_at"`
	LastFailureAt time.Time `json:"last_failure_at"`
}

// NewWebhookHandler 创建 Webhook 处理器
func NewWebhookHandler(config *WebhookConfig) *WebhookHandler {
	if config == nil {
		config = &WebhookConfig{
			Method:     "POST",
			Timeout:    10 * time.Second,
			MaxRetries: 3,
			RetryDelay: 5 * time.Second,
		}
	}

	return &WebhookHandler{
		config:     config,
		client:     &http.Client{Timeout: config.Timeout},
		retryQueue: make([]RetryItem, 0),
		stats:      &WebhookStats{},
	}
}

// Name 返回处理器名称
func (h *WebhookHandler) Name() string {
	return "webhook"
}

// Send 发送告警
func (h *WebhookHandler) Send(ctx context.Context, alert *Alert) error {
	if cfg := h.currentConfig(); cfg == nil || cfg.URL == "" {
		return nil // 未配置 webhook：no-op
	}
	payload, contentType, err := h.buildPayload(alert)
	if err != nil {
		return err
	}

	return h.sendWithRetry(ctx, payload, contentType)
}

// buildPayload 构建请求体
func (h *WebhookHandler) buildPayload(alert *Alert) ([]byte, string, error) {
	// Slack 格式
	if h.isSlackWebhook() {
		data := map[string]interface{}{
			"text": fmt.Sprintf("[%s] %s", h.severityToEmoji(alert.Severity), alert.RuleName),
			"attachments": []map[string]interface{}{
				{
					"color": h.severityToColor(alert.Severity),
					"fields": []map[string]interface{}{
						{"title": "指标", "value": alert.Metric, "short": true},
						{"title": "当前值", "value": fmt.Sprintf("%.2f", alert.Value), "short": true},
						{"title": "阈值", "value": fmt.Sprintf("%.2f", alert.Details["threshold"]), "short": true},
						{"title": "时间", "value": alert.Timestamp.Format("2006-01-02 15:04:05"), "short": true},
					},
					"footer": "Prerender Shield Alerting",
				},
			},
		}
		body, err := json.Marshal(data)
		return body, "application/json", err
	}

	// 钉钉格式
	if h.isDingtalkWebhook() {
		data := map[string]interface{}{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"title": alert.RuleName,
				"text": fmt.Sprintf("## %s %s\n\n**级别**: %s\n\n| 项目 | 值 |\n|------|------|\n| 指标 | %s |\n| 当前值 | %.2f |\n| 阈值 | %.2f |\n| 时间 | %s |\n\n> %s",
					h.severityToEmoji(alert.Severity),
					alert.RuleName,
					h.severityToText(alert.Severity),
					alert.Metric,
					alert.Value,
					alert.Details["threshold"],
					alert.Timestamp.Format("2006-01-02 15:04:05"),
					alert.Message,
				),
			},
		}
		body, err := json.Marshal(data)
		return body, "application/json", err
	}

	// 通用 JSON 格式
	data := map[string]interface{}{
		"alert_id":  alert.ID,
		"rule_id":   alert.RuleID,
		"rule_name": alert.RuleName,
		"severity":  alert.Severity,
		"message":   alert.Message,
		"timestamp": alert.Timestamp.Format(time.RFC3339),
		"metric":    alert.Metric,
		"value":     alert.Value,
		"details":   alert.Details,
	}
	body, err := json.Marshal(data)
	return body, "application/json", err
}

// sendWithRetry 带重试发送
func (h *WebhookHandler) sendWithRetry(ctx context.Context, payload []byte, contentType string) error {
	h.updateStatsSend()

	var lastErr error
	for i := 0; i <= h.currentConfig().MaxRetries; i++ {
		if err := h.send(ctx, payload, contentType); err == nil {
			h.updateStatsSuccess()
			return nil
		} else {
			lastErr = err
			h.updateStatsFailure()
		}

		if i < h.currentConfig().MaxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(h.currentConfig().RetryDelay):
			}
		}
	}

	return fmt.Errorf("发送失败，已重试 %d 次：%w", h.currentConfig().MaxRetries, lastErr)
}

// send 发送请求
func (h *WebhookHandler) send(ctx context.Context, payload []byte, contentType string) error {
	req, err := http.NewRequestWithContext(ctx, h.currentConfig().Method, h.currentConfig().URL, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", contentType)
	for k, v := range h.currentConfig().Headers {
		req.Header.Set(k, v)
	}

	// 添加签名（如果配置了密钥）
	if h.currentConfig().Secret != "" {
		signature := h.signPayload(payload)
		req.Header.Set("X-Signature", signature)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP 错误：%d, 响应：%s", resp.StatusCode, string(body))
	}

	return nil
}

// signPayload 签名载荷
func (h *WebhookHandler) signPayload(payload []byte) string {
	mac := hmac.New(sha256.New, []byte(h.currentConfig().Secret))
	mac.Write(payload)
	return fmt.Sprintf("sha256=%x", mac.Sum(nil))
}

// isSlackWebhook 是否是 Slack webhook
func (h *WebhookHandler) isSlackWebhook() bool {
	return h.currentConfig().URL != "" && (len(h.currentConfig().URL) >= 19 && h.currentConfig().URL[:19] == "https://hooks.slack")
}

// isDingtalkWebhook 是否是钉钉 webhook
func (h *WebhookHandler) isDingtalkWebhook() bool {
	return h.currentConfig().URL != "" && (len(h.currentConfig().URL) >= 25 && h.currentConfig().URL[:25] == "https://oapi.dingtalk.com")
}

// severityToColor 严重程度转颜色
func (h *WebhookHandler) severityToColor(severity string) string {
	switch severity {
	case "critical":
		return "#e94560"
	case "warning":
		return "#ffc107"
	default:
		return "#4ecca3"
	}
}

// severityToEmoji 严重程度转 Emoji
func (h *WebhookHandler) severityToEmoji(severity string) string {
	switch severity {
	case "critical":
		return "🚨"
	case "warning":
		return "⚠️"
	default:
		return "ℹ️"
	}
}

// severityToText 严重程度转文本
func (h *WebhookHandler) severityToText(severity string) string {
	switch severity {
	case "critical":
		return "严重"
	case "warning":
		return "警告"
	default:
		return "信息"
	}
}

// GetStats 获取统计
func (h *WebhookHandler) GetStats() *WebhookStats {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stats
}

func (h *WebhookHandler) updateStatsSend() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stats.TotalSent++
	h.stats.LastSentAt = time.Now()
}

func (h *WebhookHandler) updateStatsSuccess() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stats.SuccessCount++
	h.stats.LastSuccessAt = time.Now()
}

func (h *WebhookHandler) updateStatsFailure() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stats.FailureCount++
	h.stats.LastFailureAt = time.Now()
}

// EmailHandler 邮件告警处理器（简化实现）
type EmailHandler struct {
	config *EmailConfig
}

// EmailConfig 邮件配置
type EmailConfig struct {
	Enabled  bool     `yaml:"enabled" json:"enabled"`
	SMTPHost string   `yaml:"smtp_host" json:"smtp_host"`
	SMTPPort int      `yaml:"smtp_port" json:"smtp_port"`
	Username string   `yaml:"username" json:"username"`
	Password string   `yaml:"password" json:"password"`
	From     string   `yaml:"from" json:"from"`
	To       []string `yaml:"to" json:"to"`
	UseTLS   bool     `yaml:"use_tls" json:"use_tls"`
}

// NewEmailHandler 创建邮件处理器
func NewEmailHandler(config *EmailConfig) AlertHandler {
	return NewEmailNotifier(*config)
}
