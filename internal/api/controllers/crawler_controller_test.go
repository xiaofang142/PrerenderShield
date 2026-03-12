package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/logging"
)

func setupCrawlerController() (*CrawlerController, *gin.Engine) {
	gin.SetMode(gin.TestMode)

	// 创建测试用的 crawler log manager
	crawlerLogMgr := logging.NewCrawlerLogManager("")

	controller := NewCrawlerController(crawlerLogMgr)

	router := gin.New()
	router.GET("/crawler/logs", controller.GetCrawlerLogs)
	router.GET("/crawler/stats", controller.GetCrawlerStats)

	return controller, router
}

func TestCrawlerController_GetCrawlerLogs_Success(t *testing.T) {
	_, router := setupCrawlerController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/crawler/logs?page=1&pageSize=10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, float64(200), response["code"])
	assert.Equal(t, "success", response["message"].(string))

	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)
	// items 可能为 nil 当没有数据时
	assert.NotNil(t, data)
}

func TestCrawlerController_GetCrawlerLogs_WithFilters(t *testing.T) {
	_, router := setupCrawlerController()

	startTime := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	endTime := time.Now().Format(time.RFC3339)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/crawler/logs?site=test-site&startTime="+startTime+"&endTime="+endTime+"&page=1&pageSize=20", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, float64(200), response["code"])
}

func TestCrawlerController_GetCrawlerLogs_InvalidTimeFormat(t *testing.T) {
	_, router := setupCrawlerController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/crawler/logs?startTime=invalid&endTime=invalid", nil)
	router.ServeHTTP(w, req)

	// 应该使用默认时间范围，返回成功
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCrawlerController_GetCrawlerStats_Success(t *testing.T) {
	_, router := setupCrawlerController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/crawler/stats?site=test-site&granularity=hour", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, float64(200), response["code"])
	assert.Equal(t, "success", response["message"].(string))
	assert.NotNil(t, response["data"])
}

func TestCrawlerController_GetCrawlerStats_DefaultGranularity(t *testing.T) {
	_, router := setupCrawlerController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/crawler/stats", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, float64(200), response["code"])
}

func TestCrawlerController_GetCrawlerLogs_InvalidPagination(t *testing.T) {
	_, router := setupCrawlerController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/crawler/logs?page=invalid&pageSize=invalid", nil)
	router.ServeHTTP(w, req)

	// 应该使用默认分页参数
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCrawlerController_GetCrawlerStats_InvalidGranularity(t *testing.T) {
	_, router := setupCrawlerController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/crawler/stats?granularity=invalid", nil)
	router.ServeHTTP(w, req)

	// 应该返回错误或使用默认值
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNewCrawlerController(t *testing.T) {
	gin.SetMode(gin.TestMode)

	crawlerLogMgr := logging.NewCrawlerLogManager("")
	controller := NewCrawlerController(crawlerLogMgr)

	assert.NotNil(t, controller)
	assert.NotNil(t, controller.crawlerLogMgr)
}
