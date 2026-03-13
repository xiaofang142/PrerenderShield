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

func setupPreheatController() (*PreheatController, *gin.Engine) {
	gin.SetMode(gin.TestMode)

	// 创建测试配置
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

	// 使用 nil 依赖进行单元测试
	controller := NewPreheatController(nil, nil, cfg)

	router := gin.New()
	router.GET("/preheat/sites", controller.GetPreheatSites)
	router.GET("/preheat/stats", controller.GetPreheatStats)
	router.POST("/preheat/trigger", controller.TriggerPreheat)
	router.GET("/preheat/urls", controller.GetPreheatUrls)
	router.GET("/preheat/task/status", controller.GetPreheatTaskStatus)
	router.GET("/preheat/crawler-headers", controller.GetCrawlerHeaders)
	router.POST("/preheat/clear-cache", controller.ClearCache)

	return controller, router
}

func TestPreheatController_GetPreheatSites_Success(t *testing.T) {
	_, router := setupPreheatController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/preheat/sites", nil)
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

func TestPreheatController_GetPreheatSites_NilConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewPreheatController(nil, nil, nil)

	router := gin.New()
	router.GET("/preheat/sites", controller.GetPreheatSites)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/preheat/sites", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPreheatController_GetPreheatStats_NilConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewPreheatController(nil, nil, nil)

	router := gin.New()
	router.GET("/preheat/stats", controller.GetPreheatStats)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/preheat/stats", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPreheatController_GetPreheatStats_NilPrerenderManager(t *testing.T) {
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
	controller := NewPreheatController(nil, nil, cfg)

	router := gin.New()
	router.GET("/preheat/stats", controller.GetPreheatStats)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/preheat/stats?siteId=test-site-1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPreheatController_GetCrawlerHeaders_Success(t *testing.T) {
	_, router := setupPreheatController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/preheat/crawler-headers", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, float64(200), response["code"])
}

func TestPreheatController_TriggerPreheat_InvalidRequest(t *testing.T) {
	_, router := setupPreheatController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/preheat/trigger", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPreheatController_TriggerPreheat_SiteNotFound(t *testing.T) {
	_, router := setupPreheatController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/preheat/trigger",
		bytes.NewBufferString(`{"siteId":"non-existent"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPreheatController_GetPreheatUrls_NilRedis(t *testing.T) {
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
	controller := NewPreheatController(nil, nil, cfg)

	router := gin.New()
	router.GET("/preheat/urls", controller.GetPreheatUrls)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/preheat/urls?siteId=test-site-1", nil)
	router.ServeHTTP(w, req)

	// redisClient 为 nil 时返回空列表 200
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPreheatController_GetPreheatTaskStatus_NilPrerenderManager(t *testing.T) {
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
	controller := NewPreheatController(nil, nil, cfg)

	router := gin.New()
	router.GET("/preheat/task/status", controller.GetPreheatTaskStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/preheat/task/status?siteId=test-site-1", nil)
	router.ServeHTTP(w, req)

	// prerenderManager 为 nil 时返回默认状态 200
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPreheatController_ClearCache_NilRedis(t *testing.T) {
	_, router := setupPreheatController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/preheat/clear-cache",
		bytes.NewBufferString(`{"siteId":"test-site-1"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// 由于 redisClient 为 nil，会返回 503
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPreheatController_GetPreheatStats_InvalidSiteId(t *testing.T) {
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
	controller := NewPreheatController(nil, nil, cfg)

	router := gin.New()
	router.GET("/preheat/stats", controller.GetPreheatStats)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/preheat/stats?siteId=non-existent", nil)
	router.ServeHTTP(w, req)

	// prerenderManager 为 nil 时返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPreheatController_TriggerPreheat_InvalidJSON(t *testing.T) {
	_, router := setupPreheatController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/preheat/trigger",
		bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPreheatController_GetPreheatUrls_MissingSiteId(t *testing.T) {
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
	controller := NewPreheatController(nil, nil, cfg)

	router := gin.New()
	router.GET("/preheat/urls", controller.GetPreheatUrls)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/preheat/urls", nil)
	router.ServeHTTP(w, req)

	// siteId 缺失时返回 404
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPreheatController_GetPreheatTaskStatus_MissingSiteId(t *testing.T) {
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
	controller := NewPreheatController(nil, nil, cfg)

	router := gin.New()
	router.GET("/preheat/task/status", controller.GetPreheatTaskStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/preheat/task/status", nil)
	router.ServeHTTP(w, req)

	// prerenderManager 为 nil 时返回 200
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPreheatController_ClearCache_InvalidJSON(t *testing.T) {
	_, router := setupPreheatController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/preheat/clear-cache",
		bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPreheatController_ClearCache_MissingSiteId(t *testing.T) {
	_, router := setupPreheatController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/preheat/clear-cache",
		bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPreheatController_GetPreheatStats_AllSites(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				ID:      "test-site-1",
				Name:    "Test Site 1",
				Domains: []string{"localhost"},
				Port:    8080,
			},
			{
				ID:      "test-site-2",
				Name:    "Test Site 2",
				Domains: []string{"localhost"},
				Port:    8081,
			},
		},
	}

	gin.SetMode(gin.TestMode)
	controller := NewPreheatController(nil, nil, cfg)

	router := gin.New()
	router.GET("/preheat/stats", controller.GetPreheatStats)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/preheat/stats?siteId=non-existent", nil)
	router.ServeHTTP(w, req)

	// prerenderManager 为 nil 时返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPreheatController_TriggerPreheat_MissingSiteId(t *testing.T) {
	_, router := setupPreheatController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/preheat/trigger", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPreheatController_GetPreheatStats_SingleSite(t *testing.T) {
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
	controller := NewPreheatController(nil, nil, cfg)

	router := gin.New()
	router.GET("/preheat/stats", controller.GetPreheatStats)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/preheat/stats?siteId=test-site-1", nil)
	router.ServeHTTP(w, req)

	// prerenderManager 为 nil 时返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPreheatController_GetPreheatStats_NonExistentSite(t *testing.T) {
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
	controller := NewPreheatController(nil, nil, cfg)

	router := gin.New()
	router.GET("/preheat/stats", controller.GetPreheatStats)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/preheat/stats?siteId=non-existent", nil)
	router.ServeHTTP(w, req)

	// prerenderManager 为 nil 时返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
