package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/config"
)

func setupPushController() (*PushController, *gin.Engine) {
	gin.SetMode(gin.TestMode)

	// 创建测试配置
	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				ID:      "test-site-1",
				Name:    "Test Site 1",
				Domains: []string{"localhost"},
				Port:    8080,
				Prerender: config.PrerenderConfig{
					Push: config.PushConfig{
						Enabled: true,
					},
				},
			},
		},
	}

	// 使用 nil 依赖进行单元测试
	controller := NewPushController(nil, nil, cfg)

	router := gin.New()
	router.GET("/push/sites", controller.GetSites)
	router.GET("/push/stats", controller.GetPushStats)
	router.GET("/push/logs", controller.GetPushLogs)
	router.GET("/push/trend", controller.GetPushTrend)
	router.GET("/push/config", controller.GetPushConfig)
	router.POST("/push/config", controller.UpdatePushConfig)

	return controller, router
}

func TestPushController_GetSites_Success(t *testing.T) {
	_, router := setupPushController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/push/sites", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, float64(200), response["code"])
	assert.Equal(t, "success", response["message"].(string))

	data, ok := response["data"].([]interface{})
	assert.True(t, ok)
	assert.GreaterOrEqual(t, len(data), 1)
}

func TestPushController_GetSites_NilConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewPushController(nil, nil, nil)

	router := gin.New()
	router.GET("/push/sites", controller.GetSites)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/push/sites", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPushController_GetPushStats_NilConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewPushController(nil, nil, nil)

	router := gin.New()
	router.GET("/push/stats", controller.GetPushStats)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/push/stats", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPushController_GetPushStats_NilPushManager(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				ID:      "test-site-1",
				Name:    "Test Site 1",
				Domains: []string{"localhost"},
				Port:    8080,
			},
		},
	}

	gin.SetMode(gin.TestMode)
	controller := NewPushController(nil, nil, cfg)

	router := gin.New()
	router.GET("/push/stats", controller.GetPushStats)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/push/stats?siteId=test-site-1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPushController_GetPushLogs_NilPushManager(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				ID:      "test-site-1",
				Name:    "Test Site 1",
				Domains: []string{"localhost"},
				Port:    8080,
			},
		},
	}

	gin.SetMode(gin.TestMode)
	controller := NewPushController(nil, nil, cfg)

	router := gin.New()
	router.GET("/push/logs", controller.GetPushLogs)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/push/logs?siteId=test-site-1", nil)
	router.ServeHTTP(w, req)

	// pushManager 为 nil 时返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPushController_GetPushTrend_MissingSiteId(t *testing.T) {
	_, router := setupPushController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/push/trend", nil)
	router.ServeHTTP(w, req)

	// pushManager 为 nil 时返回 500，不会到 validation 那一步
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPushController_GetPushTrend_NilPushManager(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				ID:      "test-site-1",
				Name:    "Test Site 1",
				Domains: []string{"localhost"},
				Port:    8080,
			},
		},
	}

	gin.SetMode(gin.TestMode)
	controller := NewPushController(nil, nil, cfg)

	router := gin.New()
	router.GET("/push/trend", controller.GetPushTrend)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/push/trend?siteId=test-site-1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPushController_GetPushConfig_MissingSiteId(t *testing.T) {
	_, router := setupPushController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/push/config", nil)
	router.ServeHTTP(w, req)

	// pushManager 为 nil 时返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPushController_GetPushConfig_NilPushManager(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				ID:      "test-site-1",
				Name:    "Test Site 1",
				Domains: []string{"localhost"},
				Port:    8080,
			},
		},
	}

	gin.SetMode(gin.TestMode)
	controller := NewPushController(nil, nil, cfg)

	router := gin.New()
	router.GET("/push/config", controller.GetPushConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/push/config?siteId=test-site-1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPushController_UpdatePushConfig_MissingSiteId(t *testing.T) {
	_, router := setupPushController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/push/config",
		bytes.NewBufferString(`{"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// pushManager 为 nil 时返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPushController_UpdatePushConfig_NilPushManager(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				ID:      "test-site-1",
				Name:    "Test Site 1",
				Domains: []string{"localhost"},
				Port:    8080,
			},
		},
	}

	gin.SetMode(gin.TestMode)
	controller := NewPushController(nil, nil, cfg)

	router := gin.New()
	router.POST("/push/config", controller.UpdatePushConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/push/config?siteId=test-site-1",
		bytes.NewBufferString(`{"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPushController_GetPushLogs_MissingSiteId(t *testing.T) {
	_, router := setupPushController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/push/logs", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPushController_GetPushLogs_InvalidLimit(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				ID:      "test-site-1",
				Name:    "Test Site 1",
				Domains: []string{"localhost"},
				Port:    8080,
			},
		},
	}

	gin.SetMode(gin.TestMode)
	controller := NewPushController(nil, nil, cfg)

	router := gin.New()
	router.GET("/push/logs", controller.GetPushLogs)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/push/logs?siteId=test-site-1&limit=invalid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPushController_UpdatePushConfig_InvalidJSON(t *testing.T) {
	_, router := setupPushController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/push/config",
		bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPushController_GetPushStats_MissingSiteId(t *testing.T) {
	_, router := setupPushController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/push/stats", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
