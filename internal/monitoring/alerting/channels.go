package alerting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

// ChannelType 通知渠道类型
type ChannelType string

const (
	ChannelWebhook    ChannelType = "webhook"
	ChannelEmail     ChannelType = "email"
	ChannelDingTalk  ChannelType = "dingtalk"
	ChannelWeChat    ChannelType = "wechat"
	ChannelSlack     ChannelType = "slack"
	ChannelFeishu    ChannelType = "feishu"
)

// MultiChannelNotifier 多渠道通知器
type MultiChannelNotifier struct {
	channels []Channel
	logger   *zap.Logger
}

// Channel 通知渠道接口
type Channel interface {
	Name() string
	Type() ChannelType
	Send(alert Alert) error
}

// ChannelConfig 渠道配置
type ChannelConfig struct {
	Type      ChannelType `yaml:"type" json:"type"`
	Name      string      `yaml:"name" json:"name"`
	WebhookURL string     `yaml:"webhook_url" json:"webhook_url"`
	Secret    string      `yaml:"secret" json:"secret"`
	Enabled   bool        `yaml:"enabled" json:"enabled"`
}

// NewMultiChannelNotifier 创建多渠道通知器
func NewMultiChannelNotifier(configs []ChannelConfig, logger *zap.Logger) *MultiChannelNotifier {
	if logger == nil {
		logger = zap.NewNop()
	}

	n := &MultiChannelNotifier{
		channels: make([]Channel, 0),
		logger:   logger,
	}

	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		ch := createChannel(cfg)
		if ch != nil {
			n.channels = append(n.channels, ch)
			logger.Info("Alert channel registered", zap.String("name", cfg.Name), zap.String("type", string(cfg.Type)))
		}
	}

	return n
}

func createChannel(cfg ChannelConfig) Channel {
	switch cfg.Type {
	case ChannelWebhook:
		return &genericWebhookChannel{name: cfg.Name, url: cfg.WebhookURL}
	case ChannelDingTalk:
		return &dingTalkChannel{name: cfg.Name, webhookURL: cfg.WebhookURL}
	case ChannelWeChat:
		return &weChatChannel{name: cfg.Name, webhookURL: cfg.WebhookURL, secret: cfg.Secret}
	case ChannelSlack:
		return &slackChannel{name: cfg.Name, webhookURL: cfg.WebhookURL}
	case ChannelFeishu:
		return &feishuChannel{name: cfg.Name, webhookURL: cfg.WebhookURL}
	default:
		return nil
	}
}

// NotifyAll 通知所有渠道
func (n *MultiChannelNotifier) NotifyAll(alert Alert) {
	for _, ch := range n.channels {
		if err := ch.Send(alert); err != nil {
			n.logger.Error("Alert channel send failed",
				zap.String("channel", ch.Name()),
				zap.Error(err),
			)
		}
	}
}

// Count 渠道数量
func (n *MultiChannelNotifier) Count() int {
	return len(n.channels)
}

// genericWebhookChannel 通用 Webhook 渠道
type genericWebhookChannel struct {
	name string
	url  string
}

func (c *genericWebhookChannel) Name() string          { return c.name }
func (c *genericWebhookChannel) Type() ChannelType      { return ChannelWebhook }
func (c *genericWebhookChannel) Send(alert Alert) error { return postJSON(c.url, alert) }

// dingTalkChannel 钉钉渠道
type dingTalkChannel struct {
	name        string
	webhookURL  string
}

func (c *dingTalkChannel) Name() string     { return c.name }
func (c *dingTalkChannel) Type() ChannelType { return ChannelDingTalk }
func (c *dingTalkChannel) Send(alert Alert) error {
	val := alert.Value
	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": fmt.Sprintf("[%s] %s", alert.Severity, alert.RuleName),
			"text":  fmt.Sprintf("### %s 告警\n- **级别**: %s\n- **指标**: %s\n- **当前值**: %.2f\n- **时间**: %s",
				alert.RuleName, alert.Severity, alert.Metric, val, alert.Timestamp.Format("2006-01-02 15:04:05")),
		},
	}
	return postJSON(c.webhookURL, payload)
}

// weChatChannel 企业微信渠道
type weChatChannel struct {
	name        string
	webhookURL  string
	secret      string
}

func (c *weChatChannel) Name() string     { return c.name }
func (c *weChatChannel) Type() ChannelType { return ChannelWeChat }
func (c *weChatChannel) Send(alert Alert) error {
	val := alert.Value
	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": fmt.Sprintf("## %s 告警\n> 级别: %s\n> 指标: %s\n> 当前值: %.2f",
				alert.RuleName, alert.Severity, alert.Metric, val),
		},
	}
	return postJSON(c.webhookURL, payload)
}

// slackChannel Slack 渠道
type slackChannel struct {
	name        string
	webhookURL  string
}

func (c *slackChannel) Name() string     { return c.name }
func (c *slackChannel) Type() ChannelType { return ChannelSlack }
func (c *slackChannel) Send(alert Alert) error {
	color := "#FF0000"
	if alert.Severity == "warning" {
		color = "#FFA500"
	} else if alert.Severity == "info" {
		color = "#3498DB"
	}
	val := alert.Value
	payload := map[string]interface{}{
		"attachments": []map[string]interface{}{
			{
				"color": color,
				"title": fmt.Sprintf("[%s] %s", alert.Severity, alert.RuleName),
				"fields": []map[string]interface{}{
					{"title": "Metric", "value": alert.Metric, "short": true},
					{"title": "Current", "value": fmt.Sprintf("%.2f", val), "short": true},
					{"title": "Time", "value": alert.Timestamp.Format("2006-01-02 15:04:05"), "short": false},
				},
			},
		},
	}
	return postJSON(c.webhookURL, payload)
}

// feishuChannel 飞书渠道
type feishuChannel struct {
	name        string
	webhookURL  string
}

func (c *feishuChannel) Name() string     { return c.name }
func (c *feishuChannel) Type() ChannelType { return ChannelFeishu }
func (c *feishuChannel) Send(alert Alert) error {
	val := alert.Value
	content := fmt.Sprintf("**指标**: %s\n**当前值**: %.2f\n**时间**: %s",
		alert.Metric, val, alert.Timestamp.Format("2006-01-02 15:04:05"))
	payload := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]string{"tag": "plain_text", "content": fmt.Sprintf("[%s] %s", alert.Severity, alert.RuleName)},
			},
			"elements": []map[string]interface{}{
				{"tag": "div", "text": map[string]string{"tag": "lark_md", "content": content}},
			},
		},
	}
	return postJSON(c.webhookURL, payload)
}

func postJSON(url string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}

// validateChannelConfig 校验渠道配置
func validateChannelConfig(cfg ChannelConfig) error {
	if !cfg.Enabled {
		return nil
	}
	switch cfg.Type {
	case ChannelWebhook, ChannelDingTalk, ChannelWeChat, ChannelSlack, ChannelFeishu:
		if cfg.WebhookURL == "" {
			return fmt.Errorf("webhook_url required for channel type: %s", cfg.Type)
		}
		return nil
	default:
		return fmt.Errorf("unsupported channel type: %s", cfg.Type)
	}
}

// FilterSensitive 过滤敏感信息（用于日志展示）
func FilterSensitive(cfg ChannelConfig) ChannelConfig {
	if cfg.WebhookURL != "" {
		if idx := strings.LastIndex(cfg.WebhookURL, "/"); idx > 0 {
			cfg.WebhookURL = cfg.WebhookURL[:idx] + "/***"
		}
	}
	if cfg.Secret != "" {
		cfg.Secret = "****"
	}
	return cfg
}
