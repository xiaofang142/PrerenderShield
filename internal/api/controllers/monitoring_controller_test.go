package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"prerender-shield/internal/constants"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/redis"
)

func setupMonitoringController() (*MonitoringController, *gin.Engine) {
	gin.SetMode(gin.TestMode)

	// 创建测试用的 monitor
	monitor := monitoring.NewMonitor(monitoring.Config{
		Enabled:           true,
		PrometheusAddress: ":9090",
	})

	controller := NewMonitoringController(monitor, nil)

	router := gin.New()
	router.GET("/monitoring/stats", controller.GetStats)

	return controller, router
}

func TestMonitoringController_GetStats_Success(t *testing.T) {
	_, router := setupMonitoringController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/monitoring/stats", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, float64(200), response["code"])
	assert.Equal(t, "success", response["message"].(string))
	assert.NotNil(t, response["data"])

	// 验证返回的数据包含预期的字段
	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, data["totalRequests"])
	assert.NotNil(t, data["blockedRequests"])
	assert.NotNil(t, data["cacheHitRate"])
	assert.NotNil(t, data["activeBrowsers"])
}

// TestMonitoringController_AlertHistory_ZSetOrdering 验证告警历史按时间倒序返回（最新在前）
func TestMonitoringController_AlertHistory_ZSetOrdering(t *testing.T) {
	gin.SetMode(gin.TestMode)

	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer redisClient.Close()

	redisClient.Del(constants.RedisKeyAlertHistory)
	defer redisClient.Del(constants.RedisKeyAlertHistory)

	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: true})
	controller := NewMonitoringController(monitor, redisClient)

	router := gin.New()
	router.GET("/alerts/history", controller.GetAlertHistory)

	base := time.Now()
	// 故意乱序写入，验证 ZSet score 排序
	inputs := []struct {
		msg string
		ts  time.Time
	}{
		{"old", base.Add(-2 * time.Hour)},
		{"newest", base},
		{"middle", base.Add(-1 * time.Hour)},
	}
	for _, in := range inputs {
		controller.SaveAlertRecord(AlertRecord{
			ID:        "id-" + in.msg,
			Level:     "warning",
			Rule:      "cpu",
			Message:   in.msg,
			Value:     90,
			Threshold: 80,
			Timestamp: in.ts,
		})
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/alerts/history?limit=10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Code int           `json:"code"`
		Data []AlertRecord `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Data, 3)

	assert.Equal(t, "newest", response.Data[0].Message)
	assert.Equal(t, "middle", response.Data[1].Message)
	assert.Equal(t, "old", response.Data[2].Message)

	// limit 截断验证
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/alerts/history?limit=1", nil)
	router.ServeHTTP(w2, req2)
	var resp2 struct {
		Data []AlertRecord `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))
	require.Len(t, resp2.Data, 1)
	assert.Equal(t, "newest", resp2.Data[0].Message)
}
