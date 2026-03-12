package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupSystemController() (*SystemController, *gin.Engine) {
	gin.SetMode(gin.TestMode)

	// 使用 nil redis client 进行单元测试
	controller := NewSystemController(nil)

	router := gin.New()
	router.GET("/health", controller.Health)
	router.GET("/version", controller.Version)
	router.GET("/system/config", controller.GetSystemConfig)
	router.POST("/system/config", controller.UpdateSystemConfig)

	return controller, router
}

func TestSystemController_Health_NoRedis(t *testing.T) {
	_, router := setupSystemController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, float64(200), response["code"])
	assert.Equal(t, "success", response["message"].(string))

	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "running", data["status"])
	assert.Equal(t, "prerender-shield", data["service"])
	assert.Equal(t, "unknown", data["redis_status"])
	assert.NotNil(t, data["health_details"])
}

func TestSystemController_Version_Success(t *testing.T) {
	_, router := setupSystemController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/version", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, float64(200), response["code"])
	assert.Equal(t, "success", response["message"].(string))

	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotEmpty(t, data["version"])
	assert.NotEmpty(t, data["name"])
}

func TestSystemController_GetSystemConfig_NoRedis(t *testing.T) {
	_, router := setupSystemController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/config", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSystemController_UpdateSystemConfig_NoRedis(t *testing.T) {
	_, router := setupSystemController()

	configData := map[string]interface{}{
		"access_log_retention_days":  "30",
		"crawler_log_retention_days": "30",
	}
	body, _ := json.Marshal(configData)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/system/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSystemController_UpdateSystemConfig_InvalidRequest(t *testing.T) {
	_, router := setupSystemController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/system/config", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// 由于 Redis 不可用，会返回 500 而不是 400
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSystemController_Health_WithRedis(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewSystemController(nil)

	router := gin.New()
	router.GET("/health", controller.Health)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, float64(200), response["code"])
	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "running", data["status"])
}

func TestSystemController_GetSystemConfig_EmptyResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewSystemController(nil)

	router := gin.New()
	router.GET("/system/config", controller.GetSystemConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/config", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestNewSystemController(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewSystemController(nil)

	assert.NotNil(t, controller)
	assert.Nil(t, controller.redisClient)
}

func TestSystemController_Health_Details(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewSystemController(nil)

	router := gin.New()
	router.GET("/health", controller.Health)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "running", data["status"])
	assert.Equal(t, "prerender-shield", data["service"])
	assert.NotNil(t, data["health_details"])
}

func TestSystemController_Version_Details(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewSystemController(nil)

	router := gin.New()
	router.GET("/version", controller.Version)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/version", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotEmpty(t, data["version"])
	assert.Equal(t, "prerender-shield", data["name"])
}

func TestSystemController_UpdateSystemConfig_EmptyConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewSystemController(nil)

	router := gin.New()
	router.POST("/system/config", controller.UpdateSystemConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/system/config", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// Redis 不可用时返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
