package siteserver

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
)

// silenceFatal 注入 no-op 退出实现并返回 Fatal 调用计数：端口冲突会真实触发
// logging.Fatal（默认 os.Exit）。调用方必须在测试结束前等待 Fatal 计数到位——
// 残留 goroutine 若在 restore 之后调用 Fatal 会拿到真 os.Exit 终止测试进程
func silenceFatal(t *testing.T) *atomic.Int64 {
	t.Helper()
	calls := &atomic.Int64{}
	t.Cleanup(logging.SetFatalExit(func(int) { calls.Add(1) }))
	return calls
}

// waitForFatal 轮询等待 Fatal 计数到位
func waitForFatal(t *testing.T, calls *atomic.Int64, want int64, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if calls.Load() >= want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected %d fatal calls within %v, got %d", want, timeout, calls.Load())
}

// waitListener 轮询等待端口可连接
func waitListener(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("server not listening on %s within %v", addr, timeout)
}

// StartSiteServer 入口分流（SSL enabled → startTLSServer）+ 管理器证书三分支 + 重定向：
// 1) CertFile/KeyFile 有效 → HTTPS
// 2) sslManager 命中域名 → 证书来自管理器 → HTTPS + HTTPPort>0 → 重定向服务器
// 3) ForceHTTPS=true → 真实请求验证 301
// 4) sslManager 全部未命中 → 降级 HTTP + ForceHTTPS=false → 透传 handler
func TestStartSiteServer_ViaManager_TLSAndRedirect(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := generateTestCert(t, dir)
	echo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("via-manager-ok"))
	})
	tlsClient := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // nolint: gosec — 测试自签证书
		},
	}

	// 1. 证书文件有效（经 StartSiteServer 入口）→ HTTPS
	m1 := NewManager(nil, nil)
	site1 := config.SiteConfig{
		ID: "via-ssl1", Name: "V1", Port: 19451, Mode: "static",
		SSL: config.SiteSSLConfig{Enabled: true, CertFile: certPath, KeyFile: keyPath},
	}
	m1.StartSiteServer(site1, "127.0.0.1", dir, nil, echo)
	waitListener(t, "127.0.0.1:19451", 3*time.Second)
	resp, err := tlsClient.Get("https://127.0.0.1:19451/")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("https site broken: %v %+v", err, resp)
	}
	resp.Body.Close()

	// 2. sslManager 命中域名 → 管理器证书 → HTTPS + HTTPPort>0 → 重定向服务器
	m2 := NewManager(nil, &fakeSSLManager{okDomain: "ssl2.local"})
	site2 := config.SiteConfig{
		ID: "via-ssl2", Name: "V2", Port: 19452, Mode: "static", Domains: []string{"ssl2.local"},
		SSL: config.SiteSSLConfig{Enabled: true, HTTPPort: 19453, ForceHTTPS: true},
	}
	m2.StartSiteServer(site2, "127.0.0.1", dir, nil, echo)
	waitListener(t, "127.0.0.1:19452", 3*time.Second)
	waitListener(t, "127.0.0.1:19453", 3*time.Second)

	// 3. ForceHTTPS=true：HTTP 请求 → 301 到 HTTPS
	noRedirect := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp2, err := noRedirect.Get("http://127.0.0.1:19453/page?a=1")
	if err != nil {
		t.Fatalf("redirect server broken: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusMovedPermanently ||
		!strings.HasPrefix(resp2.Header.Get("Location"), "https://127.0.0.1:19453/page?a=1") {
		t.Fatalf("force https redirect broken: %d %q", resp2.StatusCode, resp2.Header.Get("Location"))
	}

	// 4. sslManager 全部未命中 → 降级 HTTP（fallback 路径不启动重定向服务器，
	// startTLSServer 在降级分支提前 return）
	m3 := NewManager(nil, &fakeSSLManager{okDomain: "other.example"})
	site3 := config.SiteConfig{
		ID: "via-ssl3", Name: "V3", Port: 19454, Mode: "static", Domains: []string{"nomatch.example"},
		SSL: config.SiteSSLConfig{Enabled: true, HTTPPort: 19455},
	}
	m3.StartSiteServer(site3, "127.0.0.1", dir, nil, echo)
	waitListener(t, "127.0.0.1:19454", 3*time.Second)
	resp3, err := http.Get("http://127.0.0.1:19454/")
	if err != nil || resp3.StatusCode != 200 {
		t.Fatalf("fallback http site broken: %v %+v", err, resp3)
	}
	resp3.Body.Close()

	m1.StopAllServers()
	m2.StopAllServers()
	m3.StopAllServers()
	time.Sleep(200 * time.Millisecond) // 等待 ListenAndServe goroutine 退出并记录覆盖
}

// 端口冲突 → ListenAndServe 立即失败 → Fatal 分支（HTTP/TLS/重定向 Warn）
func TestSiteServer_PortConflict(t *testing.T) {
	fatalCalls := silenceFatal(t)
	dir := t.TempDir()
	certPath, keyPath := generateTestCert(t, dir)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	// 1. HTTP 端口被占 → startHTTPServer 的 Fatal 分支
	ln1, err := net.Listen("tcp", "127.0.0.1:19456")
	if err != nil {
		t.Skipf("port 19456 unavailable: %v", err)
	}
	defer ln1.Close()
	m := NewManager(nil, nil)
	m.startHTTPServer(config.SiteConfig{ID: "pc1", Name: "PC1", Port: 19456}, "127.0.0.1", handler)

	// 2. TLS 端口被占 → startTLSServer 的 Fatal 分支（证书来自管理器，确保走 HTTPS 路径）
	ln2, err := net.Listen("tcp", "127.0.0.1:19457")
	if err != nil {
		t.Skipf("port 19457 unavailable: %v", err)
	}
	defer ln2.Close()
	m2 := NewManager(nil, &fakeSSLManager{okDomain: "pc.local"})
	m2.startTLSServer(config.SiteConfig{
		ID: "pc2", Name: "PC2", Port: 19457, Domains: []string{"pc.local"},
		SSL: config.SiteSSLConfig{Enabled: true},
	}, "127.0.0.1", handler)

	// 3. 重定向端口被占 → startHTTPRedirectServer 的 Warn 分支
	ln3, err := net.Listen("tcp", "127.0.0.1:19458")
	if err != nil {
		t.Skipf("port 19458 unavailable: %v", err)
	}
	defer ln3.Close()
	m3 := NewManager(nil, nil)
	m3.startTLSServer(config.SiteConfig{
		ID: "pc3", Name: "PC3", Port: 19459,
		SSL: config.SiteSSLConfig{Enabled: true, CertFile: certPath, KeyFile: keyPath, HTTPPort: 19458},
	}, "127.0.0.1", handler)

	waitListener(t, "127.0.0.1:19459", 3*time.Second) // HTTPS 本体正常，重定向冲突
	// 等待两处 Fatal（HTTP+TLS）真实触发完成，避免 restore 后残留 goroutine 拿到 os.Exit
	waitForFatal(t, fatalCalls, 2, 5*time.Second)

	m.StopAllServers()
	m2.StopAllServers()
	m3.StopAllServers()
	time.Sleep(200 * time.Millisecond)
}

// StopAllServers：活动连接（handler 未返回）→ Shutdown 5s 超时 →
// StopSiteServer 错误分支 + StopAllServers 错误日志分支
func TestStopAllServers_TimeoutError(t *testing.T) {
	release := make(chan struct{})
	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-release // 挂起 handler → 连接保持活动状态
	})

	m := NewManager(nil, nil)
	m.startHTTPServer(config.SiteConfig{ID: "slow", Name: "SLOW", Port: 19460}, "127.0.0.1", slow)
	waitListener(t, "127.0.0.1:19460", 3*time.Second)

	// 建立并保持一个活动连接（不读完响应体）
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:19460/", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		t.Skipf("cannot establish active connection: %v", err)
	}
	defer resp.Body.Close()

	start := time.Now()
	m.StopAllServers() // StopSiteServer 超时 → 错误分支 + StopAllServers 日志分支
	elapsed := time.Since(start)
	if elapsed < 4*time.Second {
		t.Fatalf("StopAllServers must block on active connection until timeout, took %v", elapsed)
	}
	if _, ok := m.GetSiteServer("slow"); !ok {
		t.Fatal("failed server must remain registered after shutdown error")
	}

	// 清理：直接强制关闭挂起的服务器与连接
	if srv, ok := m.GetSiteServer("slow"); ok {
		_ = srv.Close()
	}
	close(release)
	_ = m.StopSiteServer("slow")
	time.Sleep(100 * time.Millisecond)
}

// StopSiteServer 成功路径：注册 → 关停 → 注销
func TestStopSiteServer_Success(t *testing.T) {
	m := NewManager(nil, nil)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	site := config.SiteConfig{ID: "stop-ok", Name: "SO", Port: 19461}
	m.startHTTPServer(site, "127.0.0.1", handler)
	waitListener(t, "127.0.0.1:19461", 3*time.Second)
	if err := m.StopSiteServer("stop-ok"); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if _, ok := m.GetSiteServer("stop-ok"); ok {
		t.Fatal("stopped server must be unregistered")
	}
	// 不存在的站点 → 幂等 nil
	if err := m.StopSiteServer("never-existed"); err != nil {
		t.Fatalf("stop missing site must be nil: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
}

// AddHSTSHeaders：默认 max-age 与自定义 max-age
func TestAddHSTSHeaders_Branches(t *testing.T) {
	// 自定义 max-age
	rec := httptest.NewRecorder()
	AddHSTSHeaders(rec, 600)
	if got := rec.Header().Get("Strict-Transport-Security"); got != "max-age=600; includeSubDomains" {
		t.Fatalf("custom max-age broken: %q", got)
	}
	// 非正值 → 默认一年
	rec2 := httptest.NewRecorder()
	AddHSTSHeaders(rec2, 0)
	if got := rec2.Header().Get("Strict-Transport-Security"); got != "max-age=31536000; includeSubDomains" {
		t.Fatalf("default max-age broken: %q", got)
	}
}

// GetDomains / MatchDomain：通配符与端口剥离分支
func TestMatchDomain_Branches(t *testing.T) {
	site := config.SiteConfig{Domains: []string{"a.example", "*.wild.example"}}
	if got := GetDomains(site); len(got) != 2 {
		t.Fatalf("GetDomains broken: %v", got)
	}
	cases := []struct {
		host string
		want bool
	}{
		{"a.example:8443", true},      // 端口剥离
		{"a.example", true},           // 精确
		{"sub.wild.example", true},    // 通配符
		{"sub.wild.example:80", true}, // 通配符+端口
		{"b.example", false},          // 未收录
		{"evil-a.example", false},     // 后缀伪造不匹配
		{"", false},                   // 空主机
	}
	for _, tc := range cases {
		if got := MatchDomain(tc.host, site.Domains); got != tc.want {
			t.Fatalf("MatchDomain(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}
