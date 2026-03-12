package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/config"
)

func setupSitesController() (*SitesController, *gin.Engine, *config.Config) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				ID:      "test-site-1",
				Name:    "Test Site 1",
				Domains: []string{"127.0.0.1"},
				Port:    8080,
				Mode:    "static",
			},
			{
				ID:      "test-site-2",
				Name:    "Test Site 2",
				Domains: []string{"localhost"},
				Port:    8081,
				Mode:    "static",
			},
		},
		Dirs: config.DirsConfig{
			StaticDir: "/tmp/test-static",
		},
	}

	// 创建测试目录
	os.MkdirAll(cfg.Dirs.StaticDir, 0755)

	controller := NewSitesController(
		nil, // configManager
		nil, // siteServerMgr
		nil, // siteHandler
		nil, // redisClient
		nil, // monitor
		nil, // crawlerLogMgr
		nil, // visitLogMgr
		cfg,
	)

	router := gin.New()
	router.GET("/sites", controller.GetSites)
	router.GET("/sites/:id", controller.GetSite)
	router.GET("/sites/:id/config", controller.GetSiteConfig)
	router.POST("/sites", controller.AddSite)
	router.PUT("/sites/:id", controller.UpdateSite)
	router.DELETE("/sites/:id", controller.DeleteSite)
	router.GET("/sites/:id/static", controller.GetStaticFiles)
	router.POST("/sites/:id/static", controller.UploadStaticFile)
	router.POST("/sites/:id/static/extract", controller.ExtractFile)
	router.DELETE("/sites/:id/static", controller.DeleteStaticFile)
	router.POST("/sites/:id/static/batch-delete", controller.BatchDeleteStaticFiles)

	return controller, router, cfg
}

func TestSitesController_GetSites_Success(t *testing.T) {
	_, router, _ := setupSitesController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sites", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, float64(200), response["code"])
	data, ok := response["data"].([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 2, len(data))
}

func TestSitesController_GetSite_Success(t *testing.T) {
	_, router, _ := setupSitesController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sites/test-site-1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, float64(200), response["code"])
	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "test-site-1", data["id"])
}

func TestSitesController_GetSite_NotFound(t *testing.T) {
	_, router, _ := setupSitesController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sites/non-existent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSitesController_GetSiteConfig_NoRedis(t *testing.T) {
	_, router, _ := setupSitesController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sites/test-site-1/config?type=prerender", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSitesController_GetSiteConfig_InvalidType(t *testing.T) {
	_, router, _ := setupSitesController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sites/test-site-1/config?type=invalid", nil)
	router.ServeHTTP(w, req)

	// 由于没有 redisClient，会先返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSitesController_AddSite_InvalidRequest(t *testing.T) {
	_, router, _ := setupSitesController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/sites", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSitesController_AddSite_InvalidDomain(t *testing.T) {
	_, router, _ := setupSitesController()

	site := map[string]interface{}{
		"name":    "Invalid Site",
		"domains": []string{"example.com"}, // 只允许 127.0.0.1 或 localhost
		"port":    9000,
		"mode":    "static",
	}
	body, _ := json.Marshal(site)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/sites", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSitesController_AddSite_InvalidPort(t *testing.T) {
	_, router, _ := setupSitesController()

	site := map[string]interface{}{
		"name":    "Invalid Port Site",
		"domains": []string{"127.0.0.1"},
		"port":    80, // 保留端口
		"mode":    "static",
	}
	body, _ := json.Marshal(site)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/sites", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSitesController_UpdateSite_InvalidRequest(t *testing.T) {
	_, router, _ := setupSitesController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/sites/test-site-1", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSitesController_UpdateSite_NotFound(t *testing.T) {
	_, router, cfg := setupSitesController()

	site := map[string]interface{}{
		"name":    "Updated Site",
		"domains": []string{"127.0.0.1"},
		"port":    8080,
		"mode":    "static",
	}
	body, _ := json.Marshal(site)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/sites/non-existent", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// 由于没有 configManager，会返回 500
	// 这是预期行为，因为我们的测试设置中没有提供 configManager
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// 清理测试目录
	os.RemoveAll(cfg.Dirs.StaticDir)
}

func TestSitesController_DeleteSite(t *testing.T) {
	_, router, cfg := setupSitesController()

	// 由于没有 configManager，删除会返回 500
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/sites/non-existent", nil)
	router.ServeHTTP(w, req)

	// 没有 configManager 时返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// 清理测试目录
	os.RemoveAll(cfg.Dirs.StaticDir)
}

func TestSitesController_GetStaticFiles_NoConfigManager(t *testing.T) {
	_, router, cfg := setupSitesController()

	// 创建测试目录
	testDir := cfg.Dirs.StaticDir + "/test-site-1"
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(cfg.Dirs.StaticDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sites/test-site-1/static?path=/", nil)
	router.ServeHTTP(w, req)

	// 没有 configManager 时返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSitesController_DeleteStaticFile_NoConfigManager(t *testing.T) {
	_, router, cfg := setupSitesController()

	// 创建测试文件
	testDir := cfg.Dirs.StaticDir + "/test-site-1"
	os.MkdirAll(testDir, 0755)
	testFile := testDir + "/test.txt"
	os.WriteFile(testFile, []byte("test"), 0644)
	defer os.RemoveAll(cfg.Dirs.StaticDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/sites/test-site-1/static?path=test.txt", nil)
	router.ServeHTTP(w, req)

	// 没有 configManager 时返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSitesController_BatchDeleteStaticFiles_InvalidRequest(t *testing.T) {
	_, router, _ := setupSitesController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/sites/test-site-1/static/batch-delete", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSitesController_BatchDeleteStaticFiles_MissingPaths(t *testing.T) {
	_, router, _ := setupSitesController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/sites/test-site-1/static/batch-delete", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestIsPortAvailable(t *testing.T) {
	// 测试保留端口
	assert.False(t, isPortAvailable(80), "Port 80 should not be available")
	assert.False(t, isPortAvailable(443), "Port 443 should not be available")
	assert.False(t, isPortAvailable(22), "Port 22 should not be available")

	// 测试可用端口（找一个真正可用的）
	// 注意：这个测试可能会失败，因为端口可能已被占用
	// assert.True(t, isPortAvailable(9999), "Port 9999 should be available")
}

func TestExtractZIP(t *testing.T) {
	// 创建测试目录
	tmpDir := t.TempDir()
	extractDir := tmpDir + "/extracted"

	// 测试不存在的文件
	err := ExtractZIP("/nonexistent.zip", extractDir)
	assert.Error(t, err)
}

func TestSitesController_UpdateSitePrerenderConfig_InvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				ID:      "test-site-1",
				Name:    "Test Site 1",
				Domains: []string{"127.0.0.1"},
				Port:    8080,
				Mode:    "static",
			},
		},
	}

	controller := NewSitesController(
		nil, nil, nil, nil, nil, nil, nil, cfg,
	)

	router := gin.New()
	router.PUT("/sites/:id/prerender", controller.UpdateSitePrerenderConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/sites/test-site-1/prerender", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSitesController_UpdateSitePrerenderConfig_NoConfigManager(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				ID:      "test-site-1",
				Name:    "Test Site 1",
				Domains: []string{"127.0.0.1"},
				Port:    8080,
				Mode:    "static",
			},
		},
	}

	controller := NewSitesController(
		nil, nil, nil, nil, nil, nil, nil, cfg,
	)

	router := gin.New()
	router.PUT("/sites/:id/prerender", controller.UpdateSitePrerenderConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/sites/test-site-1/prerender", bytes.NewBufferString(`{"enabled": true}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// 没有 configManager 时返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSitesController_UpdateSitePushConfig_InvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				ID:      "test-site-1",
				Name:    "Test Site 1",
				Domains: []string{"127.0.0.1"},
				Port:    8080,
				Mode:    "static",
			},
		},
	}

	controller := NewSitesController(
		nil, nil, nil, nil, nil, nil, nil, cfg,
	)

	router := gin.New()
	router.PUT("/sites/:id/push", controller.UpdateSitePushConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/sites/test-site-1/push", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSitesController_UpdateSitePushConfig_NoConfigManager(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				ID:      "test-site-1",
				Name:    "Test Site 1",
				Domains: []string{"127.0.0.1"},
				Port:    8080,
				Mode:    "static",
			},
		},
	}

	controller := NewSitesController(
		nil, nil, nil, nil, nil, nil, nil, cfg,
	)

	router := gin.New()
	router.PUT("/sites/:id/push", controller.UpdateSitePushConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/sites/test-site-1/push", bytes.NewBufferString(`{"enabled": true}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSitesController_UpdateSiteFirewallConfig_InvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				ID:      "test-site-1",
				Name:    "Test Site 1",
				Domains: []string{"127.0.0.1"},
				Port:    8080,
				Mode:    "static",
			},
		},
	}

	controller := NewSitesController(
		nil, nil, nil, nil, nil, nil, nil, cfg,
	)

	router := gin.New()
	router.PUT("/sites/:id/firewall", controller.UpdateSiteFirewallConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/sites/test-site-1/firewall", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSitesController_UpdateSiteFirewallConfig_NoConfigManager(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				ID:      "test-site-1",
				Name:    "Test Site 1",
				Domains: []string{"127.0.0.1"},
				Port:    8080,
				Mode:    "static",
			},
		},
	}

	controller := NewSitesController(
		nil, nil, nil, nil, nil, nil, nil, cfg,
	)

	router := gin.New()
	router.PUT("/sites/:id/firewall", controller.UpdateSiteFirewallConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/sites/test-site-1/firewall", bytes.NewBufferString(`{"enabled": true}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
