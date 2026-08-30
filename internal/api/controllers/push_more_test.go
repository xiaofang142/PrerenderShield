package controllers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"prerender-shield/internal/config"
	"prerender-shield/internal/prerender/push"
)

// fakePushRedis 实现 push.RedisClient 接口
type fakePushRedis struct {
	statsErr bool
	trendErr bool
	logsErr  bool
	countErr bool
	stats    map[string]interface{}
	trend    map[string]int64
	logs     []interface{}
	logCount int64
}

func (f *fakePushRedis) GetURLs(siteID string) ([]string, error)                      { return nil, nil }
func (f *fakePushRedis) SetPushTask(siteID string, task map[string]interface{}) error { return nil }
func (f *fakePushRedis) GetPushOffset(siteID string) (int64, error)                   { return 0, nil }
func (f *fakePushRedis) SetPushOffset(siteID string, offset int64) error              { return nil }
func (f *fakePushRedis) SetLastPushDate(siteID string, date string) error             { return nil }
func (f *fakePushRedis) IncrDailyPushCountWithCount(siteID string, count int) error   { return nil }
func (f *fakePushRedis) IncrPushStats(siteID string, stat string) error               { return nil }
func (f *fakePushRedis) AddPushLogStruct(siteID string, log interface{}) error        { return nil }

func (f *fakePushRedis) GetPushStatsWithURLCounts(siteID string) (map[string]interface{}, error) {
	if f.statsErr {
		return nil, errors.New("stats failed")
	}
	if f.stats != nil {
		return f.stats, nil
	}
	return map[string]interface{}{"success": int64(5), "failed": int64(1)}, nil
}

func (f *fakePushRedis) GetLast15DaysPushCount(siteID string) (map[string]int64, error) {
	if f.trendErr {
		return nil, errors.New("trend failed")
	}
	if f.trend != nil {
		return f.trend, nil
	}
	return map[string]int64{"2026-01-01": 3}, nil
}

func (f *fakePushRedis) GetPushLogs(siteID string, limit, offset int) ([]interface{}, error) {
	if f.logsErr {
		return nil, errors.New("logs failed")
	}
	return f.logs, nil
}

func (f *fakePushRedis) GetPushLogCount(siteID string) (int64, error) {
	if f.countErr {
		return 0, errors.New("count failed")
	}
	return f.logCount, nil
}

// setupPushController 构建带假 pushManager 的推送控制器
func setupPushWithFake(t *testing.T, fake *fakePushRedis, withSites bool) (*PushController, *gin.Engine, *MockConfigManager, string) {
	t.Helper()

	staticRoot := t.TempDir()
	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{ID: "ctl-push", Name: "Push Site", Domains: []string{"push.example"},
				Prerender: config.PrerenderConfig{Push: config.PushConfig{IndexNowKey: "oldkey"}}},
		},
		Dirs: config.DirsConfig{StaticDir: staticRoot},
	}

	var manager *push.PushManager
	if fake != nil {
		manager = push.NewPushManager(cfg, fake)
	}

	controller := NewPushController(manager, nil, cfg)

	var mockCM *MockConfigManager
	if withSites {
		mockCM = &MockConfigManager{config: cfg}
		sites := NewSitesController(mockCM, nil, &MockSiteHandler{}, &MockRedisClient{}, nil, nil, nil, cfg)
		controller.SetSitesController(sites)
	}

	router := ginNewRouter()
	router.GET("/push/sites", controller.GetSites)
	router.GET("/push/stats", controller.GetPushStats)
	router.GET("/push/logs", controller.GetPushLogs)
	router.GET("/push/trend", controller.GetPushTrend)
	router.GET("/push/config", controller.GetPushConfig)
	router.PUT("/push/config", controller.UpdatePushConfig)
	return controller, router, mockCM, staticRoot
}

func TestPushController_GetSites_NilSitesList(t *testing.T) {
	cfg := &config.Config{} // Sites 为 nil
	controller := NewPushController(nil, nil, cfg)
	router := ginNewRouter()
	router.GET("/push/sites", controller.GetSites)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/push/sites", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"data":null`)
}

func TestPushController_GetPushStats_AllSitesWithFake(t *testing.T) {
	_, router, _, _ := setupPushWithFake(t, &fakePushRedis{}, false)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/push/stats", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ctl-push")
}

func TestPushController_GetPushStats_PerSiteError(t *testing.T) {
	_, router, _, _ := setupPushWithFake(t, &fakePushRedis{statsErr: true}, false)

	// 单站点查询失败 → 500
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/push/stats?siteId=ctl-push", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// 全站点循环：失败站点跳过，返回 200 空列表
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/push/stats", nil))
	require.Equal(t, http.StatusOK, w.Code)
}

func TestPushController_GetPushStats_NilSitesList(t *testing.T) {
	cfg := &config.Config{}
	manager := push.NewPushManager(cfg, &fakePushRedis{})
	controller := NewPushController(manager, nil, cfg)
	router := ginNewRouter()
	router.GET("/push/stats", controller.GetPushStats)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/push/stats", nil))
	require.Equal(t, http.StatusOK, w.Code)
}

func TestPushController_GetPushLogs(t *testing.T) {
	_, router, _, _ := setupPushWithFake(t, &fakePushRedis{
		logs: []interface{}{
			map[string]interface{}{
				"id": "log1", "siteId": "ctl-push", "siteName": "Push Site", "url": "http://x/a",
				"route": "/a", "searchEngine": "baidu", "status": "success", "message": "ok",
				"pushTime": "2026-01-01T00:00:00Z",
			},
		},
		logCount: 1,
	}, false)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/push/logs?siteId=ctl-push&page=1&pageSize=20", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"total":1`)

	// 分页 clamp
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/push/logs?siteId=ctl-push&page=0&pageSize=1000", nil))
	require.Equal(t, http.StatusOK, w.Code)
}

func TestPushController_GetPushLogs_Error(t *testing.T) {
	_, router, _, _ := setupPushWithFake(t, &fakePushRedis{logsErr: true}, false)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/push/logs?siteId=ctl-push", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPushController_GetPushLogs_CountErrorFallback(t *testing.T) {
	_, router, _, _ := setupPushWithFake(t, &fakePushRedis{
		logs:     []interface{}{},
		countErr: true,
	}, false)

	// GetPushLogCount 失败 → total 回退 len(logs)+offset
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/push/logs?siteId=ctl-push&page=2&pageSize=20", nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"total":20`)
}

func TestPushController_GetPushTrend(t *testing.T) {
	_, router, _, _ := setupPushWithFake(t, &fakePushRedis{}, false)

	// 缺 siteId → 空列表
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/push/trend", nil))
	require.Equal(t, http.StatusOK, w.Code)

}

// TestPushController_GetPushTrend_Success 趋势数据返回
func TestPushController_GetPushTrend_Success(t *testing.T) {
	_, router, _, _ := setupPushWithFake(t, &fakePushRedis{}, false)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/push/trend?siteId=ctl-push", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "2026-01-01")
}

func TestPushController_GetPushTrend_Error(t *testing.T) {
	_, router, _, _ := setupPushWithFake(t, &fakePushRedis{trendErr: true}, false)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/push/trend?siteId=ctl-push", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPushController_GetPushConfig(t *testing.T) {
	_, router, _, _ := setupPushWithFake(t, &fakePushRedis{}, false)

	// 缺 siteId → 400
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/push/config", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 站点不存在 → 404
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/push/config?siteId=ghost", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)

	// 成功
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/push/config?siteId=ctl-push", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "oldkey")
}

// TestPushController_UpdatePushConfig_Success 完整链路：SaveConfig + IndexNow keyfile 落盘
func TestPushController_UpdatePushConfig_Success(t *testing.T) {
	_, router, _, staticRoot := setupPushWithFake(t, &fakePushRedis{}, true)

	body := map[string]interface{}{
		"siteId": "ctl-push",
		"config": map[string]interface{}{
			"enabled":           true,
			"indexnow_key":      "newkey123",
			"baidu_api":         "https://zz.bdstatic.com/linksubmit/push",
			"baidu_token":       "tok",
			"baidu_daily_limit": 100,
		},
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/push/config", jsonBody(t, body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "updated successfully")

	// 新 key 文件已写入站点静态根
	data, err := os.ReadFile(filepath.Join(staticRoot, "ctl-push", "newkey123.txt"))
	require.NoError(t, err)
	assert.Equal(t, "newkey123", string(data))

	// 旧 key 文件不存在（未创建过）→ 不需要断言
}

// TestPushController_UpdatePushConfig_ClearKey key 清空时删除旧文件
func TestPushController_UpdatePushConfig_ClearKey(t *testing.T) {
	_, router, _, staticRoot := setupPushWithFake(t, &fakePushRedis{}, true)

	// 预置旧 key 文件
	siteRoot := filepath.Join(staticRoot, "ctl-push")
	require.NoError(t, os.MkdirAll(siteRoot, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(siteRoot, "oldkey.txt"), []byte("oldkey"), 0o644))

	body := map[string]interface{}{
		"siteId": "ctl-push",
		"config": map[string]interface{}{"enabled": false, "indexnow_key": ""},
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/push/config", jsonBody(t, body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	_, err := os.Stat(filepath.Join(siteRoot, "oldkey.txt"))
	assert.True(t, os.IsNotExist(err))
}

func TestPushController_UpdatePushConfig_NilSites(t *testing.T) {
	_, router, _, _ := setupPushWithFake(t, &fakePushRedis{}, false)

	body := map[string]interface{}{
		"siteId": "ctl-push",
		"config": map[string]interface{}{"enabled": true},
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/push/config", jsonBody(t, body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Sites controller not available")
}

func TestPushController_UpdatePushConfig_SiteNotFound(t *testing.T) {
	_, router, _, _ := setupPushWithFake(t, &fakePushRedis{}, true)

	body := map[string]interface{}{
		"siteId": "ghost",
		"config": map[string]interface{}{"enabled": true},
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/push/config", jsonBody(t, body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestPushController_SyncIndexNowKeyFile 直接验证 keyfile 落盘分支
func TestPushController_SyncIndexNowKeyFile(t *testing.T) {
	staticRoot := t.TempDir()
	cfg := &config.Config{
		Sites: []config.SiteConfig{{ID: "ctl-sync"}},
		Dirs:  config.DirsConfig{StaticDir: staticRoot},
	}
	controller := NewPushController(nil, nil, cfg)

	// cfg 为 nil 的控制器：直接 return
	nilCfg := NewPushController(nil, nil, nil)
	nilCfg.syncIndexNowKeyFile("s", "old", "new") // 不应 panic

	// 新 key 写入
	controller.syncIndexNowKeyFile("ctl-sync", "", "keyA")
	data, err := os.ReadFile(filepath.Join(staticRoot, "ctl-sync", "keyA.txt"))
	require.NoError(t, err)
	assert.Equal(t, "keyA", string(data))

	// key 变更：旧文件删除、新文件写入
	controller.syncIndexNowKeyFile("ctl-sync", "keyA", "keyB")
	_, err = os.Stat(filepath.Join(staticRoot, "ctl-sync", "keyA.txt"))
	assert.True(t, os.IsNotExist(err))
	data, err = os.ReadFile(filepath.Join(staticRoot, "ctl-sync", "keyB.txt"))
	require.NoError(t, err)
	assert.Equal(t, "keyB", string(data))

	// 空 key：只清理
	controller.syncIndexNowKeyFile("ctl-sync", "keyB", "")
	_, err = os.Stat(filepath.Join(staticRoot, "ctl-sync", "keyB.txt"))
	assert.True(t, os.IsNotExist(err))
}

// TestPushController_GetPushTrend_NilManager pushManager 未初始化 → 500
func TestPushController_GetPushTrend_NilManager(t *testing.T) {
	_, router, _, _ := setupPushWithFake(t, nil, false)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/push/trend?siteId=ctl-push", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
