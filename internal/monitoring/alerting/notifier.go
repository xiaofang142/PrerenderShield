package alerting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// WebhookHandler Webhook 告警处理器
type WebhookHandler struct {
	config  *WebhookConfig
	client  *http.Client
	mu      sync.Mutex
	retryQueue []RetryItem
}

// WebhookConfig Webhook 配置
type WebhookConfig struct {
	URL         string            `json:"url"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	Timeout     time.Duration     `json:"timeout"`
	MaxRetries  int               `json:"max_retries"`
	RetryDelay  time.Duration     `json:"retry_delay"`
}

// RetryItem 重试项
type RetryItem struct {
	Alert     *Alert
	RetryCount int
	NextRetry time.Time
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
	}
}

// Name 返回处理器名称
func (h *WebhookHandler) Name() string {
	return "webhook"
}

// Send 发送告警
func (h *WebhookHandler) Send(ctx context.Context, alert *Alert) error {
	payload, err := h.buildPayload(alert)
	if err != nil {
		return err
	}

	return h.sendWithRetry(ctx, payload)
}

// buildPayload 构建请求体
func (h *WebhookHandler) buildPayload(alert *Alert) ([]byte, error) {
	// 默认格式
	data := map[string]interface{}{
		"alert_id":    alert.ID,
		"rule_id":     alert.RuleID,
		"rule_name":   alert.RuleName,
		"severity":    alert.Severity,
		"message":     alert.Message,
		"timestamp":   alert.Timestamp.Format(time.RFC3339),
		"metric":      alert.Metric,
		"value":       alert.Value,
		"details":     alert.Details,
	}

	// Slack 格式
	if h.isSlackWebhook() {
		return json.Marshal(map[string]interface{}{
			"text": fmt.Sprintf("[%s] %s", alert.Severity, alert.Message),
			"attachments": []map[string]interface{}{
				{
					"color": h.severityToColor(alert.Severity),
					"fields": []map[string]interface{}{
						{"title": "指标", "value": alert.Metric, "short": true},
						{"title": "当前值", "value": fmt.Sprintf("%.2f", alert.Value), "short": true},
						{"title": "阈值", "value": fmt.Sprintf("%.2f", alert.Details["threshold"]), "short": true},
						{"title": "时间", "value": alert.Timestamp.Format("2006-01-02 15:04:05"), "short": true},
					},
				},
			},
		})
	}

	// 钉钉格式
	if h.isDingtalkWebhook() {
		return json.Marshal(map[string]interface{}{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"title": alert.RuleName,
				"text": fmt.Sprintf("## %s\n\n**级别**: %s\n**指标**: %s\n**当前值**: %.2f\n**阈值**: %.2f\n**时间**: %s",
					alert.RuleName,
					h.severityToText(alert.Severity),
					alert.Metric,
					alert.Value,
					alert.Details["threshold"],
					alert.Timestamp.Format("2006-01-02 15:04:05"),
				),
			},
		})
	}

	// 通用 JSON 格式
	return json.Marshal(data)
}

// sendWithRetry 带重试发送
func (h *WebhookHandler) sendWithRetry(ctx context.Context, payload []byte) error {
	var lastErr error

	for i := 0; i <= h.config.MaxRetries; i++ {
		if err := h.send(ctx, payload); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if i < h.config.MaxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(h.config.RetryDelay):
			}
		}
	}

	return fmt.Errorf("发送失败，已重试 %d 次：%w", h.config.MaxRetries, lastErr)
}

// send 发送请求
func (h *WebhookHandler) send(ctx context.Context, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, h.config.Method, h.config.URL, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range h.config.Headers {
		req.Header.Set(k, v)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP 错误：%d", resp.StatusCode)
	}

	return nil
}

// isSlackWebhook 是否是 Slack webhook
func (h *WebhookHandler) isSlackWebhook() bool {
	return h.config.URL != "" && (h.config.URL[:19] == "https://hooks.slack" || h.config.Headers["X-Slack-Signature"] != "")
}

// isDingtalkWebhook 是否是钉钉 webhook
func (h *WebhookHandler) isDingtalkWebhook() bool {
	return h.config.URL != "" && (h.config.URL[:27] == "https://oapi.dingtalk.com" || h.config.Headers["X-DingTalk-Secret"] != "")
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

// StartRetryWorker 启动重试工作协程
func (h *WebhookHandler) StartRetryWorker() {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for range ticker.C {
			h.processRetryQueue()
		}
	}()
}

// processRetryQueue 处理重试队列
func (h *WebhookHandler) processRetryQueue() {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	validQueue := make([]RetryItem, 0)

	for _, item := range h.retryQueue {
		if now.After(item.NextRetry) {
			// 需要重试
			payload, _ := h.buildPayload(item.Alert)
			if err := h.send(context.Background(), payload); err != nil {
				if item.RetryCount < h.config.MaxRetries {
					item.RetryCount++
					item.NextRetry = now.Add(h.config.RetryDelay)
					validQueue = append(validQueue, item)
				}
			}
		} else {
			validQueue = append(validQueue, item)
		}
	}

	h.retryQueue = validQueue
}

// addToRetryQueue 添加到重试队列
func (h *WebhookHandler) addToRetryQueue(alert *Alert) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.retryQueue = append(h.retryQueue, RetryItem{
		Alert:      alert,
		RetryCount: 0,
		NextRetry:  time.Now().Add(h.config.RetryDelay),
	})
}
