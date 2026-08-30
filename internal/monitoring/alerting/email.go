package alerting

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// EmailNotifier 邮件告警通知
type EmailNotifier struct {
	config EmailConfig
	// provider 非空时每次告警读取最新配置（控制台改动即时生效，免重启）
	provider func() *EmailConfig
}

// SetConfigProvider 注入动态配置来源
func (n *EmailNotifier) SetConfigProvider(fn func() *EmailConfig) { n.provider = fn }

// currentConfig 当前生效配置
func (n *EmailNotifier) currentConfig() EmailConfig {
	if n.provider != nil {
		if c := n.provider(); c != nil {
			return *c
		}
	}
	return n.config
}

// NewEmailNotifier 创建邮件通知器
func NewEmailNotifier(cfg EmailConfig) *EmailNotifier {
	return &EmailNotifier{config: cfg}
}

// Name 返回通知器名称
func (n *EmailNotifier) Name() string { return "email" }

// Send 发送告警邮件（实现 AlertHandler 接口）
func (n *EmailNotifier) Send(ctx context.Context, alert *Alert) error {
	cfg := n.currentConfig()
	if !cfg.Enabled || len(cfg.To) == 0 {
		return nil
	}

	val := alert.Value
	subject := fmt.Sprintf("[Prerender Shield %s] %s", alert.Severity, alert.RuleName)
	body := fmt.Sprintf(`<html>
<body style="font-family: Arial, sans-serif; padding: 20px;">
<h2 style="color: %s;">%s 告警</h2>
<table border="1" cellpadding="8" cellspacing="0" style="border-collapse:collapse;">
<tr><td><b>规则</b></td><td>%s</td></tr>
<tr><td><b>级别</b></td><td>%s</td></tr>
<tr><td><b>指标</b></td><td>%s</td></tr>
<tr><td><b>当前值</b></td><td>%.2f</td></tr>
<tr><td><b>时间</b></td><td>%s</td></tr>
<tr><td><b>消息</b></td><td>%s</td></tr>
</table>
<p style="color:#888; font-size:12px;">此邮件由 Prerender Shield 自动发送</p>
</body></html>`,
		colorForSeverity(alert.Severity), alert.Severity,
		alert.RuleName, alert.Severity, alert.Metric,
		val, alert.Timestamp.Format("2006-01-02 15:04:05"),
		alert.Message,
	)

	return n.sendSMTP(subject, body)
}

func colorForSeverity(severity string) string {
	switch severity {
	case "critical":
		return "#FF0000"
	case "warning":
		return "#FFA500"
	default:
		return "#3498DB"
	}
}

func (n *EmailNotifier) sendSMTP(subject, body string) error {
	cfg := n.currentConfig()
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	// 未配置凭证时跳过 AUTH（中继服务器无需认证场景；此前空凭证仍发 AUTH 导致
	// "server doesn't support AUTH" 拒发）
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	}

	msg := buildMessage(cfg.From, cfg.To, subject, body)

	if cfg.UseTLS {
		return n.sendTLS(addr, auth, msg)
	}
	return smtp.SendMail(addr, auth, cfg.From, cfg.To, msg)
}

func (n *EmailNotifier) sendTLS(addr string, auth smtp.Auth, msg []byte) error {
	tlsConfig := &tls.Config{
		ServerName: n.config.SMTPHost,
	}

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	tlsConn := tls.Client(conn, tlsConfig)
	client, err := smtp.NewClient(tlsConn, n.config.SMTPHost)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	if err = client.Mail(n.config.From); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	for _, to := range n.config.To {
		if err = client.Rcpt(to); err != nil {
			return fmt.Errorf("rcpt %s: %w", to, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return w.Close()
}

func buildMessage(from string, to []string, subject, body string) []byte {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("From: %s\r\n", from))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(body)
	return buf.Bytes()
}
