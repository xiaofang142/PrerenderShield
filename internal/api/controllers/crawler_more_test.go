package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"prerender-shield/internal/logging"
	"prerender-shield/internal/redis"
)

// setupCrawlerControllerWithClient 使用指定 Redis(DB15) 客户端构建爬虫控制器
func setupCrawlerControllerWithClient(t *testing.T) (*CrawlerController, *gin.Engine, *redis.Client) {
	t.Helper()

	client := newTestRedisDB15(t)

	mgr := logging.NewCrawlerLogManagerWithClient(client.GetRawClient())
	controller := NewCrawlerController(mgr)

	router := ginNewRouter()
	router.GET("/crawler/logs", controller.GetCrawlerLogs)
	router.GET("/crawler/stats", controller.GetCrawlerStats)
	router.GET("/crawler/url-stats", controller.GetURLStats)
	return controller, router, client
}

// TestCrawlerController_GetCrawlerStats_InvalidTimeFallback 非法时间回退默认窗口
func TestCrawlerController_GetCrawlerStats_InvalidTimeFallback(t *testing.T) {
	_, router, _ := setupCrawlerControllerWithClient(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/crawler/stats?startTime=bad&endTime=bad", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestCrawlerController_GetCrawlerStats_SeededData 有数据时的统计聚合
func TestCrawlerController_GetCrawlerStats_SeededData(t *testing.T) {
	_, router, client := setupCrawlerControllerWithClient(t)

	now := time.Now()
	logs := []logging.CrawlerLog{
		{ID: "u1", Site: "ctl-stats", Route: "/a", HitCache: true, Status: 200, Time: now, UA: "test-ua"},
		{ID: "u2", Site: "ctl-stats", Route: "/a", HitCache: false, Status: 200, Time: now, UA: "test-ua"},
	}
	seedCrawlerLogs(t, client, "ctl-stats", logs)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/crawler/stats?site=ctl-stats&granularity=day", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp.Data)
}

func seedCrawlerLogs(t *testing.T, client *redis.Client, site string, logs []logging.CrawlerLog) {
	t.Helper()
	day := time.Now().Format("2006-01-02")
	key := fmt.Sprintf("crawler_logs:%s:%s", site, day)
	for _, l := range logs {
		if l.Time.IsZero() {
			l.Time = time.Now()
		}
		data, err := json.Marshal(l)
		require.NoError(t, err)
		require.NoError(t, client.ZAdd(key, float64(l.Time.UnixNano()), string(data)))
	}
	t.Cleanup(func() { client.Del(key) })
}

// TestCrawlerController_GetURLStats_Success 真实数据的 per-URL 报表
func TestCrawlerController_GetURLStats_Success(t *testing.T) {
	_, router, client := setupCrawlerControllerWithClient(t)

	now := time.Now()
	logs := []logging.CrawlerLog{
		{ID: "us1", Site: "ctl-urlstats", Route: "/render-me", HitCache: false, RenderTime: 1.25, Status: 200, Time: now},
		{ID: "us2", Site: "ctl-urlstats", Route: "/render-me", HitCache: false, RenderTime: 0.75, Status: 200, Time: now},
		{ID: "us3", Site: "ctl-urlstats", Route: "/cached", HitCache: true, Status: 200, Time: now},
	}
	seedCrawlerLogs(t, client, "ctl-urlstats", logs)

	start := now.Add(-time.Hour).Format(time.RFC3339)
	end := now.Add(time.Hour).Format(time.RFC3339)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/crawler/url-stats?site=ctl-urlstats&startTime="+start+"&endTime="+end+"&limit=10", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			List  []logging.URLStat `json:"list"`
			Total int               `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, 2, resp.Data.Total)
	// 渲染次数降序：/render-me 在前
	if len(resp.Data.List) == 2 {
		assert.Equal(t, "/render-me", resp.Data.List[0].Route)
		assert.Equal(t, int64(2), resp.Data.List[0].Renders)
		assert.Equal(t, int64(0), resp.Data.List[0].CacheHits)
		assert.Equal(t, "/cached", resp.Data.List[1].Route)
		assert.Equal(t, int64(1), resp.Data.List[1].CacheHits)
	}
}

// TestCrawlerController_GetURLStats_EmptyResult 无数据时返回空列表而非 null
func TestCrawlerController_GetURLStats_EmptyResult(t *testing.T) {
	_, router, _ := setupCrawlerControllerWithClient(t)

	start := time.Now().Add(-time.Hour).Format(time.RFC3339)
	end := time.Now().Format(time.RFC3339)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/crawler/url-stats?site=ctl-empty&startTime="+start+"&endTime="+end, nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"list":[]`)
	assert.Contains(t, w.Body.String(), `"total":0`)
}

// TestCrawlerController_GetURLStats_InvalidTimes 非法时间回退默认窗口
func TestCrawlerController_GetURLStats_InvalidTimes(t *testing.T) {
	_, router, _ := setupCrawlerControllerWithClient(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/crawler/url-stats?site=ctl-badtime&startTime=bad&endTime=bad&limit=999", nil)
	router.ServeHTTP(w, req)

	// 回退后默认窗口 start < end，正常返回
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestCrawlerController_GetURLStats_TimeRangeRejected startTime >= endTime 返回 400
func TestCrawlerController_GetURLStats_TimeRangeRejected(t *testing.T) {
	_, router, _ := setupCrawlerControllerWithClient(t)

	now := time.Now().UTC().Format(time.RFC3339)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/crawler/url-stats?site=x&startTime="+now+"&endTime="+now, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "startTime must be before endTime")
}

// TestCrawlerController_GetURLStats_LimitClamp limit 越界回退 20
func TestCrawlerController_GetURLStats_LimitClamp(t *testing.T) {
	_, router, _ := setupCrawlerControllerWithClient(t)

	start := time.Now().Add(-time.Hour).Format(time.RFC3339)
	end := time.Now().Format(time.RFC3339)

	for _, q := range []string{"limit=0", "limit=-5", "limit=10000"} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/crawler/url-stats?site=ctl-clamp&startTime="+start+"&endTime="+end+"&"+q, nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, q)
	}
}
