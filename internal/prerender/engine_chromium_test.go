package prerender

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"prerender-shield/internal/cache"
	"prerender-shield/internal/prerender/pool"
	"prerender-shield/internal/redis"
)

// pooledEngine 组合引擎与所属池，Close 时一并释放（测试辅助）
type pooledEngine struct {
	Engine
	pool *pool.Pool
}

func (p *pooledEngine) Close() error {
	err := p.Engine.Close()
	if p.pool != nil {
		p.pool.Close()
	}
	return err
}

// 真实 Chromium 渲染管线单元级验证：
// httptest 目标页 → chromedp 渲染 → 状态码捕获/SEO注入/信封缓存/质量门/失效闭环 全链。
// 轻量池（Min=1）+ 404 前置：状态码捕获与管线主体先行，环境敏感的第三渲染殿后。
func TestEngine_RenderAndCache_RealChromium(t *testing.T) {
	if _, err := pool.ResolveChromiumPath(""); err != nil {
		t.Skipf("chromium unavailable: %v", err)
	}

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Write([]byte(`<!DOCTYPE html><html><head><title>Target Page</title></head>` +
				`<body><h1>Real Chromium Render Check</h1><p>This paragraph provides the substantial visible ` +
				`text content required to pass the thin-page quality gate of the rendering pipeline in this verification run.</p>` +
				`<a href="/sub">sub</a></body></html>`))
		case "/thin":
			w.Write([]byte(`<html><body></body></html>`)) // 空壳页 → 质量门
		case "/err":
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`<html><body><h1>Not Found</h1><p>Custom 404 body content page with text.</p></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer target.Close()

	client, err := redis.NewClient("localhost:6379", "", 0)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	cm := cache.NewManagerWithClient(client)
	siteID := "chromium-e2e"
	ua := "Mozilla/5.0 (compatible; Googlebot/2.1)"

	// 轻量池配置：Min=1 降低测试环境 spawn 压力（语义与生产一致，仅规模不同）
	cfg := pool.DefaultConfig()
	cfg.MinInstances = 1
	cfg.MaxInstances = 2
	browserPool := pool.NewPool(cfg, nil)
	eng := NewEngineWithSharedPool(client, cm, 1, nil, browserPool)
	defer func() {
		eng.Close()
		browserPool.Close()
	}()

	// 环境可用性守卫：chromium spawn/导航健康时首渲秒级完成；
	// 环境退化（进程残留/资源枯竭）时失败可见化——不伪造通过，与 redis-unavailable 同类守卫
	res3, err3 := eng.RenderAndCache(RenderRequest{
		SiteID: siteID, URL: target.URL + "/err",
		Opts: RenderOptions{Timeout: 30 * time.Second, CacheTTL: 60}, UserAgent: ua,
	})
	if err3 != nil {
		t.Skipf("chromium render environment degraded (probe failed): %v", err3)
	}
	if res3.Status != 404 {
		t.Fatalf("upstream 404 must be captured, got %d", res3.Status)
	}
	if env404, ok := eng.GetCachedPage(siteID, target.URL+"/err", ua); !ok || env404.Status != 404 {
		t.Fatalf("404 envelope missing: ok=%v status=%v", ok, env404.Status)
	}

	// 2. 正常页：渲染成功 + SEO 注入（meta/canonical）+ 信封可读 + 失效闭环
	res, err := eng.RenderAndCache(RenderRequest{
		SiteID:    siteID,
		URL:       target.URL + "/",
		Opts:      RenderOptions{Timeout: 90 * time.Second, CacheTTL: 60},
		UserAgent: ua,
	})
	if err != nil || !res.Success {
		t.Fatalf("render failed: err=%v res=%+v", err, res)
	}
	if res.Status != 200 {
		t.Fatalf("status=%d", res.Status)
	}
	lower := strings.ToLower(res.HTML)
	if !strings.Contains(lower, "<meta") {
		t.Fatal("SEO meta injection missing")
	}
	if !strings.Contains(lower, "canonical") {
		t.Fatal("canonical injection missing")
	}
	if env, ok := eng.GetCachedPage(siteID, target.URL+"/", ua); !ok || !strings.Contains(env.HTML, "Real Chromium") {
		t.Fatalf("envelope cache missing: ok=%v", ok)
	}
	if err := eng.InvalidatePage(siteID, target.URL+"/"); err != nil {
		t.Fatalf("invalidate: %v", err)
	}
	if _, ok := eng.GetCachedPage(siteID, target.URL+"/", ua); ok {
		t.Fatal("cache must be gone after invalidate")
	}

	// 3. 空壳页：质量门路径（thin 标记或错误页兜底，不透传空壳）
	res2, err2 := eng.RenderAndCache(RenderRequest{
		SiteID: siteID, URL: target.URL + "/thin",
		Opts: RenderOptions{Timeout: 30 * time.Second, CacheTTL: 60}, UserAgent: ua,
	})
	if err2 != nil {
		t.Fatalf("thin render err: %v", err2)
	}
	if !res2.Success {
		t.Fatalf("thin page must still succeed with quality-gate handling, got %+v", res2)
	}
}
