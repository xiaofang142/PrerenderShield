package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/monitoring"
)

func setupMonitoringController() (*MonitoringController, *gin.Engine) {
	gin.SetMode(gin.TestMode)

	// 创建测试用的 monitor
	monitor := monitoring.NewMonitor(monitoring.Config{
		Enabled:           true,
		PrometheusAddress: ":9090",
	})

	controller := NewMonitoringController(monitor)

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
