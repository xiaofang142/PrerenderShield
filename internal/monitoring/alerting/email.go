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
}

// NewEmailNotifier 创建邮件通知器
func NewEmailNotifier(cfg EmailConfig) *EmailNotifier {
	return &EmailNotifier{config: cfg}
}

// Name 返回通知器名称
func (n *EmailNotifier) Name() string { return "email" }

// Send 发送告警邮件（实现 AlertHandler 接口）
func (n *EmailNotifier) Send(ctx context.Context, alert *Alert) error {
	if !n.config.Enabled || len(n.config.To) == 0 {
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
	addr := fmt.Sprintf("%s:%d", n.config.SMTPHost, n.config.SMTPPort)
	auth := smtp.PlainAuth("", n.config.Username, n.config.Password, n.config.SMTPHost)

	msg := buildMessage(n.config.From, n.config.To, subject, body)

	if n.config.UseTLS {
		return n.sendTLS(addr, auth, msg)
	}
	return smtp.SendMail(addr, auth, n.config.From, n.config.To, msg)
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
