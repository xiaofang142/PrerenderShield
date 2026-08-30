package sitehandler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"prerender-shield/internal/cache"
	"prerender-shield/internal/config"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/prerender"
	"prerender-shield/internal/redis"
)

// fakeEngine 可编程假引擎：驱动渲染热路径全分支（不启动 chromium）
type fakeEngine struct {
	env         prerender.PageEnvelope // GetCachedPage 返回值
	hasCache    bool
	renderRes   prerender.RenderResult
	renderErr   error
	renderCalls atomic.Int32
}

func (f *fakeEngine) Render(url string, timeout time.Duration) ([]byte, error) {
	return []byte(f.renderRes.HTML), f.renderErr
}
func (f *fakeEngine) CreatePreheatTask(siteID string, urls []string) (string, error) { return "t", nil }
func (f *fakeEngine) GetPreheatTaskStatus(taskID string) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeEngine) ListPreheatTasks(siteID string) ([]map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeEngine) CancelPreheatTask(taskID string) error { return nil }
func (f *fakeEngine) CleanupPreheatTasks() error            { return nil }
func (f *fakeEngine) IsCrawlerRequest(userAgent string) bool {
	return prerender.ClassifyUserAgent(userAgent) != ""
}
func (f *fakeEngine) RenderWithContext(c *gin.Context, url string, opts prerender.RenderOptions, userAgent string) (prerender.RenderWithCacheResult, error) {
	return prerender.RenderWithCacheResult{}, nil
}
func (f *fakeEngine) RenderAndCache(req prerender.RenderRequest) (prerender.RenderResult, error) {
	f.renderCalls.Add(1)
	return f.renderRes, f.renderErr
}
func (f *fakeEngine) GetCachedPage(siteID, url, userAgent string) (prerender.PageEnvelope, bool) {
	return f.env, f.hasCache
}
func (f *fakeEngine) InvalidatePage(siteID, url string) error { return nil }
func (f *fakeEngine) ListCacheEntries(siteID string, limit int) ([]cache.CacheEntrySummary, error) {
	return nil, nil
}
func (f *fakeEngine) SetDefaultCacheTTL(seconds int)                          {}
func (f *fakeEngine) SetPreheatTTLConfig(siteTTL int, rules []config.TTLRule) {}
func (f *fakeEngine) GetPoolSize() int                                        { return 0 }
func (f *fakeEngine) Close() error                                            { return nil }

func newBranchTestServer(t *testing.T, site config.SiteConfig, eng *fakeEngine) (http.Handler, *Handler) {
	t.Helper()
	client, err := redis.NewClient("localhost:6379", "", 0)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	em := prerender.NewEngineManager(client, nil, 1)
	em.RegisterEngine(site.ID, eng)
	h := NewHandler(em, nil, client, nil, nil, nil, &monitoring.Monitor{})
	return h.CreateSiteHandler(site, nil, nil, &monitoring.Monitor{}, t.TempDir()), h
}

func crawlerReq(url string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Googlebot/2.1)")
	return req
}

// 分支1：fresh 缓存命中 → 直接供数 + ETag + hit 头
func TestRenderHotPath_FreshCacheHit(t *testing.T) {
	eng := &fakeEngine{
		hasCache: true,
		env:      prerender.PageEnvelope{Status: 200, HTML: "<html>fresh-page-body</html>", ExpiresAt: time.Now().Add(time.Hour).Unix(), CreatedAt: time.Now().Unix()},
	}
	site := config.SiteConfig{ID: "hp1", Mode: "static", Prerender: config.PrerenderConfig{Enabled: true}}
	router, _ := newBranchTestServer(t, site, eng)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, crawlerReq("http://hp1.local/"))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if w.Header().Get("X-Prerender-Hit") != "fresh" {
		t.Fatalf("hit header=%q", w.Header().Get("X-Prerender-Hit"))
	}
	if !strings.Contains(w.Body.String(), "fresh-page-body") {
		t.Fatalf("fresh html missing: %s", w.Body.String())
	}
	if w.Header().Get("ETag") == "" {
		t.Fatal("ETag missing on fresh hit")
	}
}

// 分支2：stale + SWR 开 → 立即回旧值 + 异步重渲（引擎渲染被异步调用）
func TestRenderHotPath_StaleWithSWR(t *testing.T) {
	eng := &fakeEngine{
		hasCache:  true,
		env:       prerender.PageEnvelope{Status: 200, HTML: "<html>stale-body</html>", ExpiresAt: time.Now().Add(-time.Minute).Unix(), CreatedAt: time.Now().Add(-time.Hour).Unix()},
		renderRes: prerender.RenderResult{Success: true, Status: 200, HTML: "refreshed"},
	}
	site := config.SiteConfig{ID: "hp2", Mode: "static", Prerender: config.PrerenderConfig{Enabled: true}} // SWR 默认开
	router, _ := newBranchTestServer(t, site, eng)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, crawlerReq("http://hp2.local/"))
	if w.Header().Get("X-Prerender-Hit") != "stale" {
		t.Fatalf("hit=%q want stale", w.Header().Get("X-Prerender-Hit"))
	}
	if !strings.Contains(w.Body.String(), "stale-body") {
		t.Fatalf("stale body must be served: %s", w.Body.String())
	}
	// 异步刷新最终触发
	deadline := time.Now().Add(2 * time.Second)
	for eng.renderCalls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if eng.renderCalls.Load() == 0 {
		t.Fatal("async refresh must call RenderAndCache")
	}
}

// 分支3：stale + SWR 关 → 同步重渲成功替换
func TestRenderHotPath_StaleNoSWR_RenderSuccess(t *testing.T) {
	eng := &fakeEngine{
		hasCache:  true,
		env:       prerender.PageEnvelope{Status: 200, HTML: "<html>stale-old</html>", ExpiresAt: time.Now().Add(-time.Minute).Unix()},
		renderRes: prerender.RenderResult{Success: true, Status: 200, HTML: "<html>brand-new</html>"},
	}
	off := false
	site := config.SiteConfig{ID: "hp3", Mode: "static", Prerender: config.PrerenderConfig{Enabled: true, StaleWhileRevalidate: &off}}
	router, _ := newBranchTestServer(t, site, eng)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, crawlerReq("http://hp3.local/"))
	if w.Header().Get("X-Prerender-Hit") != "miss" {
		t.Fatalf("hit=%q want miss (sync re-render)", w.Header().Get("X-Prerender-Hit"))
	}
	if !strings.Contains(w.Body.String(), "brand-new") {
		t.Fatalf("new render must be served: %s", w.Body.String())
	}
}

// 分支4：stale + SWR 关 + 重渲失败 → 旧值兜底供数
func TestRenderHotPath_StaleNoSWR_RenderFailFallback(t *testing.T) {
	eng := &fakeEngine{
		hasCache:  true,
		env:       prerender.PageEnvelope{Status: 200, HTML: "<html>fallback-old</html>", ExpiresAt: time.Now().Add(-time.Minute).Unix()},
		renderErr: errors.New("chromium crashed (simulated)"),
	}
	off := false
	site := config.SiteConfig{ID: "hp4", Mode: "static", Prerender: config.PrerenderConfig{Enabled: true, StaleWhileRevalidate: &off}}
	router, _ := newBranchTestServer(t, site, eng)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, crawlerReq("http://hp4.local/"))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "fallback-old") {
		t.Fatalf("stale fallback must serve old html, got %d %s", w.Code, w.Body.String())
	}
}

// 分支5：完全未命中 + 渲染成功
func TestRenderHotPath_MissRenderSuccess(t *testing.T) {
	eng := &fakeEngine{
		renderRes: prerender.RenderResult{Success: true, Status: 200, HTML: "<html>rendered-now</html>"},
	}
	site := config.SiteConfig{ID: "hp5", Mode: "static", Prerender: config.PrerenderConfig{Enabled: true}}
	router, _ := newBranchTestServer(t, site, eng)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, crawlerReq("http://hp5.local/"))
	if w.Header().Get("X-Prerender-Hit") != "miss" || !strings.Contains(w.Body.String(), "rendered-now") {
		t.Fatalf("miss render broken: hit=%q body=%s", w.Header().Get("X-Prerender-Hit"), w.Body.String())
	}
}

// 分支6：未命中 + 渲染失败 + 无旧值 → 503 noindex 错误页
func TestRenderHotPath_MissRenderFail_503(t *testing.T) {
	eng := &fakeEngine{renderErr: errors.New("pool exhausted (simulated)")}
	site := config.SiteConfig{ID: "hp6", Mode: "static", Prerender: config.PrerenderConfig{Enabled: true}}
	router, _ := newBranchTestServer(t, site, eng)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, crawlerReq("http://hp6.local/"))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want 503", w.Code)
	}
	if !strings.Contains(strings.ToLower(w.Body.String()), "noindex") {
		t.Fatal("503 page must carry noindex")
	}
}

// 分支7：cache_only 策略（AI 爬虫默认）——命中缓存供数
func TestRenderHotPath_CacheOnly_CacheHit(t *testing.T) {
	eng := &fakeEngine{
		hasCache: true,
		env:      prerender.PageEnvelope{Status: 200, HTML: "<html>cached-for-ai</html>", ExpiresAt: time.Now().Add(time.Hour).Unix()},
	}
	site := config.SiteConfig{ID: "hp7", Mode: "static", Prerender: config.PrerenderConfig{Enabled: true}}
	router, _ := newBranchTestServer(t, site, eng)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://hp7.local/", nil)
	req.Header.Set("User-Agent", "GPTBot/1.0")
	router.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "cached-for-ai") {
		t.Fatalf("cache_only must serve cache: %s", w.Body.String())
	}
	if eng.renderCalls.Load() != 0 {
		t.Fatal("cache_only must NOT render")
	}
}

// 分支8：cache_only 未命中 → 透传（static 模式无上游 → 404 由静态层决定；此处验证不渲染）
func TestRenderHotPath_CacheOnly_MissNoRender(t *testing.T) {
	eng := &fakeEngine{}
	site := config.SiteConfig{ID: "hp8", Mode: "static", Prerender: config.PrerenderConfig{Enabled: true}}
	router, _ := newBranchTestServer(t, site, eng)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://hp8.local/", nil)
	req.Header.Set("User-Agent", "ClaudeBot/1.0")
	router.ServeHTTP(w, req)
	if eng.renderCalls.Load() != 0 {
		t.Fatal("cache_only miss must not render")
	}
}

// 分支9：include/exclude 过滤 → 爬虫也透传不渲染
func TestRenderHotPath_ExcludePatternPassthrough(t *testing.T) {
	eng := &fakeEngine{renderRes: prerender.RenderResult{Success: true, Status: 200, HTML: "should-not-render"}}
	site := config.SiteConfig{ID: "hp9", Mode: "static", Prerender: config.PrerenderConfig{
		Enabled:         true,
		ExcludePatterns: []string{"^/api/"},
	}}
	router, _ := newBranchTestServer(t, site, eng)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, crawlerReq("http://hp9.local/api/data"))
	if eng.renderCalls.Load() != 0 {
		t.Fatal("excluded path must not render")
	}
	if w.Header().Get("X-Prerender-Hit") != "" {
		t.Fatalf("excluded path must bypass prerender, got hit=%q", w.Header().Get("X-Prerender-Hit"))
	}
}

// 分支10：include 白名单未命中 → 不渲染
func TestRenderHotPath_IncludeNotMatched(t *testing.T) {
	eng := &fakeEngine{renderRes: prerender.RenderResult{Success: true, Status: 200, HTML: "nope"}}
	site := config.SiteConfig{ID: "hp10", Mode: "static", Prerender: config.PrerenderConfig{
		Enabled:         true,
		IncludePatterns: []string{"^/docs/"},
	}}
	router, _ := newBranchTestServer(t, site, eng)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, crawlerReq("http://hp10.local/blog/x"))
	if eng.renderCalls.Load() != 0 {
		t.Fatal("include-missed path must not render")
	}
}
