package sitehandler

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/prerender"
)

func TestCompilePrerenderFilters(t *testing.T) {
	site := config.SiteConfig{ID: "f", Prerender: config.PrerenderConfig{
		IncludePatterns: []string{"^/blog/", "["},     // 第二个非法被忽略
		ExcludePatterns: []string{"^/admin", "(?P<x"}, // 第二个非法被忽略
	}}
	include, exclude := compilePrerenderFilters(site)
	if len(include) != 1 || len(exclude) != 1 {
		t.Fatalf("invalid patterns must be dropped: include=%d exclude=%d", len(include), len(exclude))
	}
}

func TestShouldPrerenderURL(t *testing.T) {
	include := []*regexp.Regexp{regexp.MustCompile("^/blog/")}
	exclude := []*regexp.Regexp{regexp.MustCompile("^/admin")}
	cases := []struct {
		uri  string
		want bool
	}{
		{"/blog/post-1", true},
		{"/admin/x", false}, // exclude 优先（即使 include 也命中）
		{"/pricing", false}, // include 非空且未命中
	}
	for _, c := range cases {
		if got := shouldPrerenderURL(c.uri, include, exclude); got != c.want {
			t.Errorf("shouldPrerenderURL(%q)=%v want %v", c.uri, got, c.want)
		}
	}
	// include 空 = 全放行（exclude 仍生效）
	if !shouldPrerenderURL("/anything", nil, exclude) {
		t.Fatal("empty include must allow all")
	}
	if shouldPrerenderURL("/admin/x", nil, exclude) {
		t.Fatal("exclude must still apply with empty include")
	}
}

func TestSiteBaseURL(t *testing.T) {
	if got := siteBaseURL(config.SiteConfig{Domains: []string{"a.com"}, SSL: config.SiteSSLConfig{Enabled: true}}); got != "https://a.com" {
		t.Fatalf("ssl site=%q", got)
	}
	if got := siteBaseURL(config.SiteConfig{Domains: []string{"a.com"}}); got != "http://a.com" {
		t.Fatalf("plain site=%q", got)
	}
	if got := siteBaseURL(config.SiteConfig{}); got != "" {
		t.Fatalf("no domains=%q", got)
	}
}

func TestPrerenderPolicyFor(t *testing.T) {
	// 空类默认 render
	if got := prerenderPolicyFor(config.SiteConfig{}, ""); got != policyRender {
		t.Fatalf("empty category=%q", got)
	}
	// 默认表：ai=cache_only
	if got := prerenderPolicyFor(config.SiteConfig{}, prerender.CatAI); got != policyCacheOnly {
		t.Fatalf("ai default=%q", got)
	}
	// 站点配置覆盖默认
	site := config.SiteConfig{Prerender: config.PrerenderConfig{CategoryPolicy: map[string]string{prerender.CatAI: "render"}}}
	if got := prerenderPolicyFor(site, prerender.CatAI); got != policyRender {
		t.Fatalf("site override broken: %q", got)
	}
	// 空串配置值回退默认表
	site2 := config.SiteConfig{Prerender: config.PrerenderConfig{CategoryPolicy: map[string]string{prerender.CatAI: ""}}}
	if got := prerenderPolicyFor(site2, prerender.CatAI); got != policyCacheOnly {
		t.Fatalf("empty value must fall back to default: %q", got)
	}
	// 未知类
	if got := prerenderPolicyFor(config.SiteConfig{}, "unknown-cat"); got != policyRender {
		t.Fatalf("unknown category=%q", got)
	}
}

func TestStaleEnabled(t *testing.T) {
	if !staleEnabled(config.SiteConfig{}) {
		t.Fatal("nil = enabled (default)")
	}
	on := true
	off := false
	if !staleEnabled(config.SiteConfig{Prerender: config.PrerenderConfig{StaleWhileRevalidate: &on}}) {
		t.Fatal("explicit true")
	}
	if staleEnabled(config.SiteConfig{Prerender: config.PrerenderConfig{StaleWhileRevalidate: &off}}) {
		t.Fatal("explicit false")
	}
}

func TestRenderErrorHTML(t *testing.T) {
	html := renderErrorHTML("zh", "detail-x")
	if !regexp.MustCompile(`noindex`).Match(html) {
		t.Fatal("error page must carry noindex")
	}
	if !regexp.MustCompile(`detail-x`).Match(html) {
		t.Fatal("detail must be embedded")
	}
}

func TestRecordCrawlerLog_NilManagerNoPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "http://x.local/p", nil)
	recordCrawlerLog(nil, c, crawlerPageMeta{Site: "s", Route: "/p", Status: 200}) // nil 管理器不 panic
}

func TestTriggerAsyncRefresh_NoPanicWithNilEngine(t *testing.T) {
	h := &Handler{}
	// nil 引擎：刷新静默跳过（协程内 recover），不 panic
	h.triggerAsyncRefresh(nil, prerender.RenderRequest{SiteID: "no-site", URL: "http://x.local/"})
	time.Sleep(50 * time.Millisecond)
}

var _ = logging.DefaultLogger
