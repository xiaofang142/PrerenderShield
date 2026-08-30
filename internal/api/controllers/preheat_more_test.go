package controllers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"prerender-shield/internal/cache"
	"prerender-shield/internal/config"
	"prerender-shield/internal/prerender"
	"prerender-shield/internal/scheduler"
)

// fakeEngine 实现 prerender.Engine 接口的假渲染引擎
type fakeEngine struct {
	failInvalidate bool
	failRender     bool
	failCreateTask bool
	failList       bool
	entries        []cache.CacheEntrySummary
}

func (e *fakeEngine) Render(url string, timeout time.Duration) ([]byte, error) {
	return []byte("<html></html>"), nil
}

func (e *fakeEngine) CreatePreheatTask(siteID string, urls []string) (string, error) {
	if e.failCreateTask {
		return "", errors.New("create task failed (simulated)")
	}
	return "task-ctl-123", nil
}

func (e *fakeEngine) GetPreheatTaskStatus(taskID string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (e *fakeEngine) ListPreheatTasks(siteID string) ([]map[string]interface{}, error) {
	return nil, nil
}

func (e *fakeEngine) CancelPreheatTask(taskID string) error { return nil }

func (e *fakeEngine) CleanupPreheatTasks() error { return nil }

func (e *fakeEngine) IsCrawlerRequest(userAgent string) bool { return false }

func (e *fakeEngine) RenderWithContext(c *gin.Context, url string, opts prerender.RenderOptions, userAgent string) (prerender.RenderWithCacheResult, error) {
	return prerender.RenderWithCacheResult{}, nil
}

func (e *fakeEngine) RenderAndCache(req prerender.RenderRequest) (prerender.RenderResult, error) {
	if e.failRender {
		return prerender.RenderResult{Error: "render failed (simulated)"}, errors.New("render failed (simulated)")
	}
	return prerender.RenderResult{Status: 200, RenderTime: 42}, nil
}

func (e *fakeEngine) GetCachedPage(siteID, url, userAgent string) (prerender.PageEnvelope, bool) {
	return prerender.PageEnvelope{}, false
}

func (e *fakeEngine) InvalidatePage(siteID, url string) error {
	if e.failInvalidate {
		return errors.New("invalidate failed (simulated)")
	}
	return nil
}

func (e *fakeEngine) ListCacheEntries(siteID string, limit int) ([]cache.CacheEntrySummary, error) {
	if e.failList {
		return nil, errors.New("list entries failed (simulated)")
	}
	return e.entries, nil
}

func (e *fakeEngine) SetDefaultCacheTTL(seconds int) {}

func (e *fakeEngine) SetPreheatTTLConfig(siteTTL int, rules []config.TTLRule) {}

func (e *fakeEngine) GetPoolSize() int { return 3 }

func (e *fakeEngine) Close() error { return nil }

// setupPreheatWithEngine 构建带 DB15 Redis + 假引擎的预热控制器
func setupPreheatWithEngine(t *testing.T, engine *fakeEngine) (*PreheatController, *gin.Engine, *prerender.EngineManager, string) {
	t.Helper()
	client := newTestRedisDB15(t)

	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{ID: "ctl-preheat", Name: "Preheat Site", Domains: []string{"preheat.example"}, Port: 8080,
				Prerender: config.PrerenderConfig{Timeout: 30}},
			{ID: "ctl-preheat80", Name: "Port80 Site", Domains: []string{"p80.example"}, Port: 80},
			{ID: "ctl-headers", Name: "Headers Site", Prerender: config.PrerenderConfig{
				CrawlerHeaders: []string{"CustomBot/1.0"}}},
		},
	}

	manager := prerender.NewEngineManager(nil, nil, 2)
	if engine != nil {
		manager.RegisterEngine("ctl-preheat", engine)
	}

	controller := NewPreheatController(manager, client, scheduler.NewScheduler(nil, nil, nil), cfg)

	router := ginNewRouter()
	router.GET("/preheat/sites", controller.GetPreheatSites)
	router.GET("/preheat/stats", controller.GetPreheatStats)
	router.POST("/preheat/trigger", controller.TriggerPreheat)
	router.GET("/preheat/urls", controller.GetPreheatUrls)
	router.GET("/preheat/task/status", controller.GetPreheatTaskStatus)
	router.GET("/preheat/crawler-headers", controller.GetCrawlerHeaders)
	router.POST("/preheat/clear-cache", controller.ClearCache)
	router.POST("/preheat/invalidate", controller.InvalidateCache)
	router.POST("/preheat/recache", controller.RecacheURL)
	router.GET("/preheat/entries", controller.ListCacheEntries)
	router.DELETE("/preheat/entries", controller.DeleteCacheEntry)

	// 清理 DB15 中本测试键
	cleanup := func() {
		for _, k := range []string{"site:ctl-preheat:urls", "site:ctl-preheat80:urls",
			"site:ctl-preheat:preheat:current_task"} {
			client.Del(k)
		}
		keys, _ := client.Keys("cache:ctl-preheat*")
		for _, k := range keys {
			client.Del(k)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	return controller, router, manager, "ctl-preheat"
}

func TestPreheatController_GetPreheatStats_AllSitesWithRedis(t *testing.T) {
	controller, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{})

	// 站点 URL 计数
	require.NoError(t, controller.redisClient.AddURL("ctl-preheat", "http://preheat.example/a"))
	// 全局缓存键
	require.NoError(t, controller.redisClient.Set("cache:ctl-preheat:/a", "<html>x</html>", time.Minute))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/preheat/stats", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data []map[string]interface{} `json:"data"`
	}
	require.NoError(t, jsonUnmarshalBody(w, &resp))
	require.NotEmpty(t, resp.Data)
	first := resp.Data[0]
	assert.Equal(t, "ctl-preheat", first["siteId"])
	assert.Equal(t, float64(1), first["urlCount"])
	assert.Equal(t, float64(3), first["browserPoolSize"])
	assert.NotNil(t, first["totalCacheSize"])
}

func TestPreheatController_GetPreheatStats_SingleSiteWithEngine(t *testing.T) {
	_, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/preheat/stats?siteId=ctl-preheat", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ctl-preheat")
	assert.NotContains(t, w.Body.String(), "siteName")
}

func TestPreheatController_GetPreheatStats_SiteNotFound(t *testing.T) {
	_, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/preheat/stats?siteId=ghost", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestPreheatController_TriggerPreheat_EngineLazilyCreated GetEngine 懒创建引擎：
// 站点存在但未注册引擎时不再 404，URL 列表为空返回 "No URLs to preheat"
func TestPreheatController_TriggerPreheat_EngineLazilyCreated(t *testing.T) {
	_, router, _, _ := setupPreheatWithEngine(t, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/preheat/trigger", jsonBody(t, map[string]string{"siteId": "ctl-preheat"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "No URLs to preheat")
}

func TestPreheatController_TriggerPreheat_NoURLs(t *testing.T) {
	_, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{})

	// site:ctl-preheat:urls 为空 → GetURLs 出错或空 → "No URLs to preheat"
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/preheat/trigger", jsonBody(t, map[string]string{"siteId": "ctl-preheat"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "No URLs to preheat")
}

func TestPreheatController_TriggerPreheat_CreateTaskError(t *testing.T) {
	controller, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{failCreateTask: true})

	require.NoError(t, controller.redisClient.AddURL("ctl-preheat", "/page-a"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/preheat/trigger", jsonBody(t, map[string]string{"siteId": "ctl-preheat"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPreheatController_TriggerPreheat_Success(t *testing.T) {
	controller, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{})

	require.NoError(t, controller.redisClient.AddURL("ctl-preheat", "/page-a"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/preheat/trigger", jsonBody(t, map[string]string{"siteId": "ctl-preheat"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Preheat triggered successfully")
}

func TestPreheatController_GetPreheatUrls_WithRedis(t *testing.T) {
	controller, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{})

	// 三种形态：完整 URL / 无斜杠相对路径 / 带斜杠路径
	require.NoError(t, controller.redisClient.AddURL("ctl-preheat", "https://full.example/page"))
	require.NoError(t, controller.redisClient.AddURL("ctl-preheat", "relative/path"))
	require.NoError(t, controller.redisClient.AddURL("ctl-preheat", "/leading-slash"))
	// 预热状态
	require.NoError(t, controller.redisClient.SetURLPreheatStatus("ctl-preheat", "/leading-slash", "done", 0))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/preheat/urls?siteId=ctl-preheat&page=1&pageSize=20", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data struct {
			List  []map[string]interface{} `json:"list"`
			Total int                      `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, jsonUnmarshalBody(w, &resp))
	require.Equal(t, 3, resp.Data.Total)

	byURL := map[string]string{}
	for _, item := range resp.Data.List {
		byURL[item["url"].(string)] = item["updatedAt"].(string)
	}
	assert.Contains(t, byURL, "https://full.example/page")
	assert.Contains(t, byURL, "http://preheat.example:8080/relative/path")
	assert.Contains(t, byURL, "http://preheat.example:8080/leading-slash")
	// 有预热状态的 URL 更新时间非 "-"
	assert.NotEqual(t, "-", byURL["http://preheat.example:8080/leading-slash"])
}

// TestPreheatController_GetPreheatUrls_Port80 端口 80 时不拼接端口
func TestPreheatController_GetPreheatUrls_Port80(t *testing.T) {
	controller, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{})

	require.NoError(t, controller.redisClient.AddURL("ctl-preheat80", "/hello"))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/preheat/urls?siteId=ctl-preheat80", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "http://p80.example/hello")
	assert.NotContains(t, w.Body.String(), "p80.example:80")
}

func TestPreheatController_GetPreheatUrls_PaginationClamp(t *testing.T) {
	controller, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{})

	require.NoError(t, controller.redisClient.AddURL("ctl-preheat", "/page-a"))

	// page=0&pageSize=0 回退默认；超出页码返回空列表
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/preheat/urls?siteId=ctl-preheat&page=0&pageSize=0", nil))
	require.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/preheat/urls?siteId=ctl-preheat&page=99&pageSize=999", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"total":1`)
}

func TestPreheatController_GetPreheatTaskStatus_WithRedisAndScheduler(t *testing.T) {
	_, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/preheat/task/status?siteId=ctl-preheat", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, jsonUnmarshalBody(w, &resp))
	assert.Equal(t, "ctl-preheat", resp.Data["siteId"])
	assert.Equal(t, false, resp.Data["isRunning"])
	// 无任务时调度器返回未调度
	assert.Equal(t, false, resp.Data["scheduled"])
}

func TestPreheatController_ClearCache_WithKeys(t *testing.T) {
	controller, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{})

	require.NoError(t, controller.redisClient.Set("cache:ctl-preheat:/a", "x", time.Minute))
	require.NoError(t, controller.redisClient.Set("cache:ctl-preheat:/b", "y", time.Minute))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/preheat/clear-cache", jsonBody(t, map[string]string{"siteId": "ctl-preheat"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"clearedCount":2`)

	// 键已被删除
	keys, _ := controller.redisClient.Keys("cache:ctl-preheat:*")
	assert.Empty(t, keys)
}

func TestPreheatController_ClearCache_NoKeys(t *testing.T) {
	_, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/preheat/clear-cache", jsonBody(t, map[string]string{"siteId": "ctl-preheat"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"clearedCount":0`)
}

// TestPreheatController_ClearCache_RedisClosed 已关闭客户端 → Keys 报错 → 500
func TestPreheatController_ClearCache_RedisClosed(t *testing.T) {
	closed := closedTestRedisDB15(t)
	cfg := &config.Config{Sites: []config.SiteConfig{{ID: "ctl-preheat"}}}
	controller := NewPreheatController(nil, closed, nil, cfg)
	router := ginNewRouter()
	router.POST("/preheat/clear-cache", controller.ClearCache)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/preheat/clear-cache", jsonBody(t, map[string]string{"siteId": "ctl-preheat"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "扫描缓存键失败")
}

func TestPreheatController_GetCrawlerHeaders_Configured(t *testing.T) {
	_, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/preheat/crawler-headers", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "CustomBot/1.0")
}

func TestPreheatController_InvalidateCache_Success(t *testing.T) {
	_, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/preheat/invalidate",
		jsonBody(t, map[string]string{"siteId": "ctl-preheat", "url": "preheat.example/page"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")
}

func TestPreheatController_InvalidateCache_EngineError(t *testing.T) {
	_, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{failInvalidate: true})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/preheat/invalidate",
		jsonBody(t, map[string]string{"siteId": "ctl-preheat", "url": "/page"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPreheatController_RecacheURL_Success(t *testing.T) {
	_, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{})

	// 归一化形态（host:port/path）→ 补全 scheme；站点 Timeout=30 生效
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/preheat/recache",
		jsonBody(t, map[string]string{"siteId": "ctl-preheat", "url": "preheat.example:8080/page"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, jsonUnmarshalBody(w, &resp))
	assert.Equal(t, "preheat.example:8080/page", resp.Data["url"])
	assert.Equal(t, float64(200), resp.Data["status"])
	assert.Equal(t, float64(42), resp.Data["renderTime"])
}

func TestPreheatController_RecacheURL_RenderError(t *testing.T) {
	_, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{failRender: true})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/preheat/recache",
		jsonBody(t, map[string]string{"siteId": "ctl-preheat", "url": "/page"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPreheatController_ListCacheEntries_Success(t *testing.T) {
	_, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{entries: []cache.CacheEntrySummary{
		{URL: "preheat.example:8080/page", Status: 200, SizeBytes: 1024, Fresh: true},
	}})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/preheat/entries?siteId=ctl-preheat&limit=10", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"total":1`)
}

func TestPreheatController_ListCacheEntries_NilEntries(t *testing.T) {
	_, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/preheat/entries?siteId=ctl-preheat", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"list":[]`)
}

func TestPreheatController_ListCacheEntries_EngineError(t *testing.T) {
	_, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{failList: true})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/preheat/entries?siteId=ctl-preheat", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPreheatController_DeleteCacheEntry_Success(t *testing.T) {
	_, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/preheat/entries?siteId=ctl-preheat&url=/page", nil))
	require.Equal(t, http.StatusOK, w.Code)
}

func TestPreheatController_DeleteCacheEntry_EngineError(t *testing.T) {
	_, router, _, _ := setupPreheatWithEngine(t, &fakeEngine{failInvalidate: true})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/preheat/entries?siteId=ctl-preheat&url=/page", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestPreheatController_GetCacheTotalSize 采样估算：nil / 零计数 / 扫描失败 / 正常采样
func TestPreheatController_GetCacheTotalSize(t *testing.T) {
	client := newTestRedisDB15(t)

	// 1. nil client → 0
	c1 := &PreheatController{}
	assert.Equal(t, int64(0), c1.getCacheTotalSize(100))

	// 2. 计数为 0 → 0
	c2 := NewPreheatController(nil, client, nil, &config.Config{})
	assert.Equal(t, int64(0), c2.getCacheTotalSize(0))

	// 3. 已关闭 client：GetCacheCount 返回 0，直接传正数计数模拟计数非零但扫描失败 → count * 1MB
	c3 := NewPreheatController(nil, closedTestRedisDB15(t), nil, &config.Config{})
	assert.Equal(t, int64(5*1024*1024), c3.getCacheTotalSize(5))

	// 4. 正常采样：3 个缓存键，采样均值 × 总数
	require.NoError(t, client.Set("cache:ctl-size:/a", "aaaaaaaaaa", time.Minute))
	require.NoError(t, client.Set("cache:ctl-size:/b", "bbbbbbbbbb", time.Minute))
	require.NoError(t, client.Set("cache:ctl-size:/c", "cccccccccc", time.Minute))
	t.Cleanup(func() {
		client.Del("cache:ctl-size:/a")
		client.Del("cache:ctl-size:/b")
		client.Del("cache:ctl-size:/c")
	})
	c4 := NewPreheatController(nil, client, nil, &config.Config{})
	size := c4.getCacheTotalSize(4)
	assert.Greater(t, size, int64(0))
}

// TestPreheatController_CollectSiteStats 直接构造站点统计（含引擎池大小）
func TestPreheatController_CollectSiteStats(t *testing.T) {
	client := newTestRedisDB15(t)
	cfg := &config.Config{Sites: []config.SiteConfig{{ID: "ctl-cs", Name: "CS", Prerender: config.PrerenderConfig{Enabled: true, PoolSize: 2, Timeout: 30, CacheTTL: 3600}}}}
	manager := prerender.NewEngineManager(nil, nil, 2)
	manager.RegisterEngine("ctl-cs", &fakeEngine{})
	controller := NewPreheatController(manager, client, nil, cfg)

	require.NoError(t, client.AddURL("ctl-cs", "/x"))
	t.Cleanup(func() { client.Del("site:ctl-cs:urls") })

	stats := controller.collectSiteStats(cfg.Sites[0], 10, 10240)
	assert.Equal(t, "ctl-cs", stats["siteId"])
	assert.Equal(t, int64(1), stats["urlCount"])
	assert.Equal(t, int64(10), stats["cacheCount"])
	assert.Equal(t, int64(10240), stats["totalCacheSize"])
	assert.Equal(t, int64(3), stats["browserPoolSize"])
}
