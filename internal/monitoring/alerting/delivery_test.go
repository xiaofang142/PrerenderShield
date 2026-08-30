package alerting

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// send 直连测试：httptest 接收端验证 payload/headers
func TestWebhookHandler_Send_DeliversPayload(t *testing.T) {
	type received struct {
		body    string
		agent   string
		success bool
	}
	var mu sync.Mutex
	got := received{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		mu.Lock()
		got.body = string(buf)
		got.agent = r.Header.Get("User-Agent")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := NewWebhookHandler(&WebhookConfig{URL: srv.URL, Method: "POST", Timeout: 5 * time.Second, MaxRetries: 1, RetryDelay: 10 * time.Millisecond})
	alert := &Alert{ID: "a1", RuleName: "cpu", Severity: "critical", Metric: "cpuUsage", Value: 99, Timestamp: time.Now(), Message: "m"}
	if err := h.Send(context.Background(), alert); err != nil {
		t.Fatalf("Send err: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(got.body, "cpu") {
		t.Fatalf("payload missing alert content: %q", got.body)
	}
	if got.agent == "" {
		t.Fatal("User-Agent missing")
	}
}

// 发送失败 → 重试 → 仍失败返回错误（重试计数验证）
func TestWebhookHandler_SendWithRetry_Retries(t *testing.T) {
	attempts := 0
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	h := NewWebhookHandler(&WebhookConfig{URL: srv.URL, Method: "POST", Timeout: 2 * time.Second, MaxRetries: 2, RetryDelay: 5 * time.Millisecond})
	if err := h.Send(context.Background(), &Alert{ID: "a", Timestamp: time.Now()}); err == nil {
		t.Fatal("persistent failure must return error")
	}
	mu.Lock()
	defer mu.Unlock()
	// 1 次初始 + 2 次重试
	if attempts != 3 {
		t.Fatalf("attempts=%d want 3", attempts)
	}
}

// Slack 格式分支
func TestWebhookHandler_SlackFormat(t *testing.T) {
	var mu sync.Mutex
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		mu.Lock()
		body = string(buf)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := NewWebhookHandler(&WebhookConfig{URL: srv.URL + "/services/T000/B000/XXXX", Method: "POST", Timeout: 5 * time.Second})
	if err := h.Send(context.Background(), &Alert{ID: "a", RuleName: "r", Severity: "warning", Message: "slack-msg", Timestamp: time.Now()}); err != nil {
		t.Fatalf("Send err: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	// 通用 JSON payload（非 Slack 域名走默认格式）
	if !strings.Contains(body, "slack-msg") || !strings.Contains(body, "rule_name") {
		t.Fatalf("generic payload broken: %q", body)
	}
	// Slack URL 判定逻辑单独验证（格式化分支不触网）
	h2 := NewWebhookHandler(&WebhookConfig{URL: "https://hooks.slack.com/services/T/B/X"})
	if !h2.isSlackWebhook() {
		t.Fatal("hooks.slack URL must be detected as slack")
	}
}

// miniSMTPServer 模拟 SMTP 服务器（25/587 明文路径，无 AUTH）
type miniSMTPServer struct {
	ln       net.Listener
	mu       sync.Mutex
	mailFrom string
	rcpts    []string
	data     string
}

func newMiniSMTP(t *testing.T) *miniSMTPServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &miniSMTPServer{ln: ln}
	go s.serve()
	t.Cleanup(func() { ln.Close() })
	return s
}

func (s *miniSMTPServer) addr() string { return s.ln.Addr().String() }

func (s *miniSMTPServer) serve() {
	conn, err := s.ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	w := bufio.NewWriter(conn)
	r := bufio.NewReader(conn)
	writeLine := func(l string) { w.WriteString(l + "\r\n"); w.Flush() }
	writeLine("220 mini-smtp ready")
	inData := false
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		trimmed := strings.TrimRight(line, "\r\n")
		switch {
		case inData:
			if trimmed == "." {
				writeLine("250 ok")
				inData = false
			} else {
				s.mu.Lock()
				s.data += trimmed + "\n"
				s.mu.Unlock()
			}
		case strings.HasPrefix(strings.ToUpper(trimmed), "EHLO"), strings.HasPrefix(strings.ToUpper(trimmed), "HELO"):
			writeLine("250 mini-smtp")
		case strings.HasPrefix(strings.ToUpper(trimmed), "MAIL FROM:"):
			s.mu.Lock()
			s.mailFrom = trimmed[10:]
			s.mu.Unlock()
			writeLine("250 ok")
		case strings.HasPrefix(strings.ToUpper(trimmed), "RCPT TO:"):
			s.mu.Lock()
			s.rcpts = append(s.rcpts, trimmed[8:])
			s.mu.Unlock()
			writeLine("250 ok")
		case strings.HasPrefix(strings.ToUpper(trimmed), "DATA"):
			writeLine("354 end with .")
			inData = true
		case strings.HasPrefix(strings.ToUpper(trimmed), "QUIT"):
			writeLine("221 bye")
			return
		default:
			writeLine("250 ok")
		}
	}
}

// EmailNotifier 经真实 SMTP 协议投递（模拟服务器）——UseTLS=false 路径
func TestEmailNotifier_Send_RealSMTPProtocol(t *testing.T) {
	smtpSrv := newMiniSMTP(t)
	host, portStr, _ := net.SplitHostPort(smtpSrv.addr())
	port, _ := strconv.Atoi(portStr)

	n := NewEmailNotifier(EmailConfig{
		Enabled:  true,
		SMTPHost: host,
		SMTPPort: port,
		From:     "alert@shield.local",
		To:       []string{"ops@shield.local"},
		UseTLS:   false, // 明文 SMTP（无 AUTH 无 TLS）路径
	})
	err := n.Send(context.Background(), &Alert{
		ID: "a", RuleName: "cpu-high", Severity: "critical",
		Metric: "cpuUsage", Value: 99.5, Timestamp: time.Now(), Message: "cpu over threshold",
	})
	if err != nil {
		t.Fatalf("Send via mini SMTP: %v", err)
	}
	if smtpSrv.mailFrom == "" || len(smtpSrv.rcpts) == 0 {
		t.Fatalf("envelope broken: from=%q rcpts=%v", smtpSrv.mailFrom, smtpSrv.rcpts)
	}
	if !strings.Contains(smtpSrv.data, "Subject:") || !strings.Contains(smtpSrv.data, "cpu-high") {
		t.Fatalf("mail content broken: %q", smtpSrv.data)
	}
}
