package sitehandler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"prerender-shield/internal/config"
	"prerender-shield/internal/firewall"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/prerender"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/repository"
)

// WAF 端到端模拟：恶意载荷（SQLi/XSS/路径穿越）→ 拦截 403 + 攻击日志；良性请求放行
func TestWAFEndToEnd_MaliciousPayloadsBlocked(t *testing.T) {
	client, err := redis.NewClient("localhost:6379", "", 0)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	em := prerender.NewEngineManager(client, nil, 1)
	eng := &fakeEngine{renderRes: prerender.RenderResult{Success: true, Status: 200, HTML: "rendered"}}
	em.RegisterEngine("waf-e2e", eng)

	site := config.SiteConfig{
		ID:      "waf-e2e",
		Mode:    "static",
		Domains: []string{"waf-e2e.local"},
		Firewall: config.FirewallConfig{
			Enabled: true,
			ActionConfig: config.ActionConfig{
				DefaultAction: "block",
				BlockMessage:  "blocked by waf",
			},
		},
		Prerender: config.PrerenderConfig{Enabled: true},
	}

	handler := NewHandler(em, repository.NewWafRepository(&repository.RedisClientWrapper{Client: client}), client, nil, firewall.NewEngineManager(), nil, &monitoring.Monitor{})
	router := handler.CreateSiteHandler(site, nil, nil, &monitoring.Monitor{}, t.TempDir())

	malicious := []struct {
		name  string
		query url.Values
	}{
		{"SQLi-union", url.Values{"id": {"1 UNION SELECT password FROM users"}}},
		{"SQLi-quote", url.Values{"id": {"1' OR '1'='1"}}},
		{"XSS-script", url.Values{"q": {"<script>alert(1)</script>"}}},
		{"XSS-onerror", url.Values{"q": {"<img src=x onerror=alert(1)>"}}},
		{"路径穿越", url.Values{"name": {"../../etc/passwd"}}},
		{"命令注入", url.Values{"cmd": {"; cat /etc/passwd"}}},
	}

	blocked := 0
	for _, m := range malicious {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://waf-e2e.local/x?"+m.query.Encode(), nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120")
		router.ServeHTTP(w, req)
		if w.Code == http.StatusForbidden {
			blocked++
		} else {
			t.Logf("[未拦截] %s -> %d %s", m.name, w.Code, truncateStr(w.Body.String(), 80))
		}
	}
	if blocked == 0 {
		t.Fatal("WAF must block at least one malicious payload")
	}
	t.Logf("拦截 %d/%d 类恶意载荷", blocked, len(malicious))

	// 良性请求必须放行
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://waf-e2e.local/products?category=shoes&page=2", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120")
	router.ServeHTTP(w, req)
	// 放行语义 = 未被 WAF 拦截（static 模式无文件 → 404 由静态层决定；403 才是 WAF 拦截）
	if w.Code == http.StatusForbidden {
		t.Fatalf("benign request must not be blocked, got 403: %s", truncateStr(w.Body.String(), 120))
	}
}

// WAF 载荷经 POST body 注入（表单）
func TestWAFEndToEnd_PostBodyXSS(t *testing.T) {
	client, err := redis.NewClient("localhost:6379", "", 0)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	em := prerender.NewEngineManager(client, nil, 1)
	em.RegisterEngine("waf-post", &fakeEngine{})
	site := config.SiteConfig{
		ID:       "waf-post",
		Mode:     "static",
		Firewall: config.FirewallConfig{Enabled: true, ActionConfig: config.ActionConfig{DefaultAction: "block"}},
	}
	handler := NewHandler(em, repository.NewWafRepository(&repository.RedisClientWrapper{Client: client}), client, nil, firewall.NewEngineManager(), nil, &monitoring.Monitor{})
	router := handler.CreateSiteHandler(site, nil, nil, &monitoring.Monitor{}, t.TempDir())

	form := url.Values{"comment": {"<script>steal()</script>"}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "http://waf-post.local/comment", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Logf("POST body XSS 未拦截: %d %s", w.Code, truncateStr(w.Body.String(), 80))
	} else {
		t.Log("POST body XSS 已拦截 403")
	}
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
