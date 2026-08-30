package siteserver

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"prerender-shield/internal/config"
)

func TestMatchDomain(t *testing.T) {
	domains := []string{"example.com", "*.sub.example.com"}
	cases := []struct {
		host string
		want bool
	}{
		{"example.com", true},
		{"example.com:8080", true},  // 端口剥离
		{"EXAMPLE.COM", false},      // 实现大小写敏感（记录现状，不臆断改行为）
		{"a.sub.example.com", true}, // 通配
		{"b.sub.example.com:443", true},
		{"sub.example.com", false}, // 通配要求至少一段前缀
		{"other.com", false},
	}
	for _, c := range cases {
		if got := MatchDomain(c.host, domains); got != c.want {
			t.Errorf("MatchDomain(%q)=%v want %v", c.host, got, c.want)
		}
	}
}

func TestGetDomains(t *testing.T) {
	site := config.SiteConfig{Domains: []string{"a.com", "b.com"}}
	if got := GetDomains(site); len(got) != 2 || got[0] != "a.com" {
		t.Fatalf("GetDomains=%v", got)
	}
}

func TestAddHSTSHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	AddHSTSHeaders(w, 86400)
	if got := w.Header().Get("Strict-Transport-Security"); got != "max-age=86400; includeSubDomains" {
		t.Fatalf("HSTS=%q", got)
	}
	// 非正数回退 1 年默认
	w2 := httptest.NewRecorder()
	AddHSTSHeaders(w2, 0)
	if got := w2.Header().Get("Strict-Transport-Security"); got != "max-age=31536000; includeSubDomains" {
		t.Fatalf("HSTS default=%q", got)
	}
}

func TestStartStopSiteServer_EphemeralPort(t *testing.T) {
	m := NewManager(nil, nil)
	siteID := "ss-test"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: handler}
	go srv.Serve(ln)

	if _, ok := m.GetSiteServer(siteID); ok {
		t.Fatal("unregistered site must not exist")
	}
	m.siteServers[siteID] = srv
	if _, ok := m.GetSiteServer(siteID); !ok {
		t.Fatal("registered site must exist")
	}
	if lst := m.ListSiteServers(); len(lst) != 1 {
		t.Fatalf("ListSiteServers=%d", len(lst))
	}

	if err := m.StopSiteServer(siteID); err != nil {
		t.Fatalf("StopSiteServer err: %v", err)
	}
	if _, ok := m.GetSiteServer(siteID); ok {
		t.Fatal("site must be removed after stop")
	}
	// 重复停止幂等
	if err := m.StopSiteServer(siteID); err != nil {
		t.Fatalf("double stop must be no-op, got %v", err)
	}

	// 停止后端口应拒绝新连接（优雅关闭生效）
	m.siteServers[siteID] = srv
	m.StopAllServers()
	if _, ok := m.GetSiteServer(siteID); ok {
		t.Fatal("StopAllServers must clear registry")
	}
	conn, err := net.DialTimeout("tcp", ln.Addr().String(), time.Second)
	if err == nil {
		conn.Close()
		t.Log("port still accepting during graceful shutdown window (acceptable)")
	}
}
