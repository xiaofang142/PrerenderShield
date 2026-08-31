package controllers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"

	"prerender-shield/internal/utils"
)

// Mock implementations for interfaces

// MockConfigManager implements ConfigManagerInterface
type MockConfigManager struct {
	config      *config.Config
	saveError   error
	mutateError error
	updateFunc  func(*config.Config)
}

func (m *MockConfigManager) GetConfig() *config.Config {
	return m.config
}

func (m *MockConfigManager) UpdateConfig(cfg *config.Config) {
	m.config = cfg
	if m.updateFunc != nil {
		m.updateFunc(cfg)
	}
}

func (m *MockConfigManager) SaveConfig() error {
	return m.saveError
}

func (m *MockConfigManager) Mutate(mutate func(c *config.Config) (*config.Config, error)) error {
	if m.mutateError != nil {
		return m.mutateError
	}
	newCfg, err := mutate(m.config)
	if err != nil {
		return err
	}
	m.config = newCfg
	return nil
}

// MockSiteServerMgr implements SiteServerManagerInterface
type MockSiteServerMgr struct {
	startSiteServerFunc func(site config.SiteConfig, serverAddr, staticDir string, crawlerLogMgr *logging.CrawlerLogManager, siteHandler http.Handler)
	stopSiteServerFunc  func(siteID string) error
	getSiteServerFunc   func(siteID string) (*http.Server, bool)
	servers             map[string]*http.Server
}

func (m *MockSiteServerMgr) StartSiteServer(site config.SiteConfig, serverAddr, staticDir string, crawlerLogMgr *logging.CrawlerLogManager, siteHandler http.Handler) {
	if m.startSiteServerFunc != nil {
		m.startSiteServerFunc(site, serverAddr, staticDir, crawlerLogMgr, siteHandler)
	}
	if m.servers == nil {
		m.servers = make(map[string]*http.Server)
	}
	m.servers[site.ID] = &http.Server{Addr: serverAddr}
}

func (m *MockSiteServerMgr) StopSiteServer(siteID string) error {
	if m.stopSiteServerFunc != nil {
		return m.stopSiteServerFunc(siteID)
	}
	if m.servers != nil {
		delete(m.servers, siteID)
	}
	return nil
}

func (m *MockSiteServerMgr) GetSiteServer(siteID string) (*http.Server, bool) {
	if m.getSiteServerFunc != nil {
		return m.getSiteServerFunc(siteID)
	}
	if m.servers == nil {
		m.servers = make(map[string]*http.Server)
	}
	srv, exists := m.servers[siteID]
	return srv, exists
}

// MockSiteHandler implements SiteHandlerInterface
type MockSiteHandler struct {
	createSiteHandlerFunc func(site config.SiteConfig, crawlerLogMgr *logging.CrawlerLogManager, visitLogMgr *logging.VisitLogManager, monitor *monitoring.Monitor, staticDir string) http.Handler
}

func (m *MockSiteHandler) CreateSiteHandler(site config.SiteConfig, crawlerLogMgr *logging.CrawlerLogManager, visitLogMgr *logging.VisitLogManager, monitor *monitoring.Monitor, staticDir string) http.Handler {
	if m.createSiteHandlerFunc != nil {
		return m.createSiteHandlerFunc(site, crawlerLogMgr, visitLogMgr, monitor, staticDir)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// MockRedisClient implements RedisClientInterface
type MockRedisClient struct {
	setSiteStatsFunc    func(siteID string, stats map[string]interface{}) error
	getSiteStatsFunc    func(key string) (map[string]string, error)
	deleteSiteDataFunc  func(siteID string) error
	clearSiteCacheFunc  func(siteID string) error
	addURLFunc          func(siteID, url string) error
	storedStats         map[string]map[string]interface{}
}

func (m *MockRedisClient) SetSiteStats(siteID string, stats map[string]interface{}) error {
	if m.setSiteStatsFunc != nil {
		return m.setSiteStatsFunc(siteID, stats)
	}
	if m.storedStats == nil {
		m.storedStats = make(map[string]map[string]interface{})
	}
	m.storedStats[siteID] = stats
	return nil
}

func (m *MockRedisClient) GetSiteStats(key string) (map[string]string, error) {
	if m.getSiteStatsFunc != nil {
		return m.getSiteStatsFunc(key)
	}
	return nil, nil
}

func (m *MockRedisClient) DeleteSiteData(siteID string) error {
	if m.deleteSiteDataFunc != nil {
		return m.deleteSiteDataFunc(siteID)
	}
	return nil
}

func (m *MockRedisClient) ClearSiteCache(siteID string) error {
	if m.clearSiteCacheFunc != nil {
		return m.clearSiteCacheFunc(siteID)
	}
	return nil
}

func (m *MockRedisClient) AddURL(siteID, url string) error {
	if m.addURLFunc != nil {
		return m.addURLFunc(siteID, url)
	}
	return nil
}

// MockMonitor implements MonitorInterface
type MockMonitor struct {
	getStatsFunc func() map[string]interface{}
}

func (m *MockMonitor) GetStats() map[string]interface{} {
	if m.getStatsFunc != nil {
		return m.getStatsFunc()
	}
	return map[string]interface{}{}
}

// MockCrawlerLogMgr implements CrawlerLogManagerInterface
type MockCrawlerLogMgr struct {
	recordCrawlerLogFunc func(crawlerLog logging.CrawlerLog)
	recordedLogs         []logging.CrawlerLog
}

func (m *MockCrawlerLogMgr) RecordCrawlerLog(crawlerLog logging.CrawlerLog) {
	if m.recordCrawlerLogFunc != nil {
		m.recordCrawlerLogFunc(crawlerLog)
	}
	if m.recordedLogs == nil {
		m.recordedLogs = make([]logging.CrawlerLog, 0)
	}
	m.recordedLogs = append(m.recordedLogs, crawlerLog)
}

// MockVisitLogMgr implements VisitLogManagerInterface
type MockVisitLogMgr struct {
	recordVisitLogFunc func(visitLog logging.VisitLog)
	recordedLogs       []logging.VisitLog
}

func (m *MockVisitLogMgr) RecordVisitLog(visitLog logging.VisitLog) {
	if m.recordVisitLogFunc != nil {
		m.recordVisitLogFunc(visitLog)
	}
	if m.recordedLogs == nil {
		m.recordedLogs = make([]logging.VisitLog, 0)
	}
	m.recordedLogs = append(m.recordedLogs, visitLog)
}

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

func setupSitesControllerWithConfigManager() (*SitesController, *gin.Engine, *config.Config) {
	gin.SetMode(gin.TestMode)

	// 重置配置管理器实例
	config.ResetInstance()

	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				ID:      "test-site-1",
				Name:    "Test Site 1",
				Domains: []string{"127.0.0.1"},
				Port:    9999, // 使用不常用的端口
				Mode:    "static",
			},
		},
		Dirs: config.DirsConfig{
			StaticDir: "/tmp/test-static-cm",
		},
	}

	// 创建测试目录
	os.MkdirAll(cfg.Dirs.StaticDir, 0755)

	// 创建 ConfigManager
	configManager := config.GetInstance()
	configManager.UpdateConfig(cfg)

	controller := NewSitesController(
		configManager, // configManager
		nil,           // siteServerMgr
		nil,           // siteHandler
		nil,           // redisClient
		nil,           // monitor
		nil,           // crawlerLogMgr
		nil,           // visitLogMgr
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
	router.POST("/sites/:id/static/upload", controller.UploadStaticFile)
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

func TestSitesController_AddSite_MissingName(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Sites: []config.SiteConfig{},
		Dirs: config.DirsConfig{
			StaticDir: "/tmp/test-static",
		},
	}

	os.MkdirAll(cfg.Dirs.StaticDir, 0755)
	defer os.RemoveAll(cfg.Dirs.StaticDir)

	controller := NewSitesController(
		nil, nil, nil, nil, nil, nil, nil, cfg,
	)

	router := gin.New()
	router.POST("/sites", controller.AddSite)

	site := map[string]interface{}{
		"domains": []string{"127.0.0.1"},
		"port":    9000,
		"mode":    "static",
	}
	body, _ := json.Marshal(site)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/sites", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// 缺少 name 字段返回 400
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSitesController_UpdateSite_MissingName(t *testing.T) {
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
		Dirs: config.DirsConfig{
			StaticDir: "/tmp/test-static",
		},
	}

	os.MkdirAll(cfg.Dirs.StaticDir, 0755)
	defer os.RemoveAll(cfg.Dirs.StaticDir)

	controller := NewSitesController(
		nil, nil, nil, nil, nil, nil, nil, cfg,
	)

	router := gin.New()
	router.PUT("/sites/:id", controller.UpdateSite)

	site := map[string]interface{}{
		"domains": []string{"127.0.0.1"},
		"port":    8080,
		"mode":    "static",
	}
	body, _ := json.Marshal(site)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/sites/test-site-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// R14-BUG-1: 缺 name 现由新校验先拦 → 400（名称必填并入名称合法性校验）
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSitesController_DeleteSite_NoConfigManager(t *testing.T) {
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
	router.DELETE("/sites/:id", controller.DeleteSite)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/sites/test-site-1", nil)
	router.ServeHTTP(w, req)

	// 没有 configManager 时返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSitesController_GetSiteConfig_MissingType(t *testing.T) {
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
	router.GET("/sites/:id/config", controller.GetSiteConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sites/test-site-1/config", nil)
	router.ServeHTTP(w, req)

	// redisClient 为 nil 时返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestIsPortAvailable_PortInRange(t *testing.T) {
	// 端口 0 非法（net.Listen 会将其解释为随机端口），应判定不可用
	assert.False(t, utils.IsPortAvailable(0), "Port 0 should not be available")
	assert.False(t, utils.IsPortAvailable(-1), "Negative port should not be available")
	assert.False(t, utils.IsPortAvailable(65536), "Port out of range should not be available")
	// 测试一个高位端口应该是可用的
	assert.True(t, utils.IsPortAvailable(50000), "Port 50000 should be available")
}

func TestSitesController_GetSites_WithConfigManager(t *testing.T) {
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
		Dirs: config.DirsConfig{
			StaticDir: "/tmp/test-static",
		},
	}

	os.MkdirAll(cfg.Dirs.StaticDir, 0755)
	defer os.RemoveAll(cfg.Dirs.StaticDir)

	controller := NewSitesController(
		nil, nil, nil, nil, nil, nil, nil, cfg,
	)

	router := gin.New()
	router.GET("/sites", controller.GetSites)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sites", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSitesController_GetSite_NotFound_WithConfigManager(t *testing.T) {
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
	router.GET("/sites/:id", controller.GetSite)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sites/non-existent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSitesController_AddSite_ReservedPort(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Sites: []config.SiteConfig{},
		Dirs: config.DirsConfig{
			StaticDir: "/tmp/test-static",
		},
	}

	os.MkdirAll(cfg.Dirs.StaticDir, 0755)
	defer os.RemoveAll(cfg.Dirs.StaticDir)

	controller := NewSitesController(
		nil, nil, nil, nil, nil, nil, nil, cfg,
	)

	router := gin.New()
	router.POST("/sites", controller.AddSite)

	site := map[string]interface{}{
		"name":    "Test Site",
		"domains": []string{"127.0.0.1"},
		"port":    80,
		"mode":    "static",
	}
	body, _ := json.Marshal(site)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/sites", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSitesController_UpdateSite_InvalidDomain(t *testing.T) {
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
		Dirs: config.DirsConfig{
			StaticDir: "/tmp/test-static",
		},
	}

	os.MkdirAll(cfg.Dirs.StaticDir, 0755)
	defer os.RemoveAll(cfg.Dirs.StaticDir)

	controller := NewSitesController(
		nil, nil, nil, nil, nil, nil, nil, cfg,
	)

	router := gin.New()
	router.PUT("/sites/:id", controller.UpdateSite)

	site := map[string]interface{}{
		"name":    "Updated Site",
		"domains": []string{"http://example.com"}, // 含协议前缀，格式非法
		"port":    8080,
		"mode":    "static",
	}
	body, _ := json.Marshal(site)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/sites/test-site-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSitesController_UpdateSite_NoConfigManager(t *testing.T) {
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
	router.PUT("/sites/:id", controller.UpdateSite)

	site := map[string]interface{}{
		"name":    "Updated Site",
		"domains": []string{"127.0.0.1"},
		"port":    8080,
		"mode":    "static",
	}
	body, _ := json.Marshal(site)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/sites/test-site-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// configManager 为 nil 时返回 500
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
		"domains": []string{"http://example.com"}, // 含协议前缀，格式非法
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

// TestSitesController_AddSite_RealDomainAccepted 真实域名应通过校验
// （原实现错误地只允许 127.0.0.1/localhost）。使用保留端口 80 触发端口错误，
// 以此证明请求已通过域名校验进入下一阶段。
func TestSitesController_AddSite_RealDomainAccepted(t *testing.T) {
	_, router, _ := setupSitesController()

	site := map[string]interface{}{
		"name":    "Real Domain Site",
		"domains": []string{"www.example.com"},
		"port":    80, // 保留端口：域名校验通过后才会报端口错误
		"mode":    "static",
	}
	body, _ := json.Marshal(site)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/sites", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Port")
	assert.NotContains(t, w.Body.String(), "domain")
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
	assert.False(t, utils.IsPortAvailable(80), "Port 80 should not be available")
	assert.False(t, utils.IsPortAvailable(443), "Port 443 should not be available")
	assert.False(t, utils.IsPortAvailable(22), "Port 22 should not be available")

	// 测试可用端口（找一个真正可用的）
	// 注意：这个测试可能会失败，因为端口可能已被占用
	// assert.True(t, utils.IsPortAvailable(9999), "Port 9999 should be available")
}

func TestUtilsExtractZIP(t *testing.T) {
	// 创建测试目录
	tmpDir := t.TempDir()
	extractDir := tmpDir + "/extracted"

	// 测试不存在的文件
	err := utils.ExtractZIP("/nonexistent.zip", extractDir)
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

func TestSitesController_UploadStaticFile_NoFile(t *testing.T) {
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
		Dirs: config.DirsConfig{
			StaticDir: "/tmp/test-static",
		},
	}

	os.MkdirAll(cfg.Dirs.StaticDir, 0755)
	defer os.RemoveAll(cfg.Dirs.StaticDir)

	controller := NewSitesController(
		nil, nil, nil, nil, nil, nil, nil, cfg,
	)

	router := gin.New()
	router.POST("/sites/:id/static", controller.UploadStaticFile)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/sites/test-site-1/static", nil)
	router.ServeHTTP(w, req)

	// 没有 configManager 时返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSitesController_UploadStaticFile_NoConfigManager(t *testing.T) {
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
		Dirs: config.DirsConfig{
			StaticDir: "/tmp/test-static",
		},
	}

	os.MkdirAll(cfg.Dirs.StaticDir, 0755)
	defer os.RemoveAll(cfg.Dirs.StaticDir)

	controller := NewSitesController(
		nil, nil, nil, nil, nil, nil, nil, cfg,
	)

	router := gin.New()
	router.POST("/sites/:id/static", controller.UploadStaticFile)

	w := httptest.NewRecorder()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("test content"))
	writer.Close()

	req, _ := http.NewRequest("POST", "/sites/test-site-1/static", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSitesController_ExtractFile_NoConfigManager(t *testing.T) {
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
		Dirs: config.DirsConfig{
			StaticDir: "/tmp/test-static",
		},
	}

	os.MkdirAll(cfg.Dirs.StaticDir, 0755)
	defer os.RemoveAll(cfg.Dirs.StaticDir)

	controller := NewSitesController(
		nil, nil, nil, nil, nil, nil, nil, cfg,
	)

	router := gin.New()
	router.POST("/sites/:id/static/extract", controller.ExtractFile)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/sites/test-site-1/static/extract", bytes.NewBufferString(`{"file": "test.zip"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSitesController_GetSites_NilConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewSitesController(
		nil, nil, nil, nil, nil, nil, nil, nil,
	)

	router := gin.New()
	router.GET("/sites", controller.GetSites)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sites", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSitesController_GetSite_NilConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewSitesController(
		nil, nil, nil, nil, nil, nil, nil, nil,
	)

	router := gin.New()
	router.GET("/sites/:id", controller.GetSite)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sites/test-site", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSitesController_AddSite_NilConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Dirs: config.DirsConfig{
			StaticDir: "/tmp/test-static",
		},
	}

	os.MkdirAll(cfg.Dirs.StaticDir, 0755)
	defer os.RemoveAll(cfg.Dirs.StaticDir)

	controller := NewSitesController(
		nil, nil, nil, nil, nil, nil, nil, cfg,
	)

	router := gin.New()
	router.POST("/sites", controller.AddSite)

	site := map[string]interface{}{
		"name":    "Test Site",
		"domains": []string{"127.0.0.1"},
		"port":    9000,
		"mode":    "static",
	}
	body, _ := json.Marshal(site)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/sites", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// configManager 为 nil 时返回 400（端口验证会失败）
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Note: The following tests are skipped because sites_controller.go uses concrete types
// instead of interfaces, making it difficult to test without all dependencies.
// To properly test these methods, we would need to:
// 1. Refactor SitesController to use interfaces for dependencies
// 2. Create mock implementations of those interfaces
// 3. Or use integration tests with real dependencies

// Helper function to create controller with all mocks
func setupSitesControllerWithMocks() (*SitesController, *gin.Engine, *MockConfigManager, *MockSiteServerMgr, *MockRedisClient) {
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
		Dirs: config.DirsConfig{
			StaticDir: "/tmp/test-static-mocks",
		},
		Server: config.ServerConfig{
			Address: "127.0.0.1",
		},
	}

	os.MkdirAll(cfg.Dirs.StaticDir, 0755)
	os.MkdirAll("/tmp/test-static-mocks/test-site-1", 0755)

	mockConfigMgr := &MockConfigManager{config: cfg}
	mockSiteServerMgr := &MockSiteServerMgr{servers: make(map[string]*http.Server)}
	mockSiteHandler := &MockSiteHandler{}
	mockRedisClient := &MockRedisClient{storedStats: make(map[string]map[string]interface{})}
	mockMonitor := &MockMonitor{}
	mockCrawlerLogMgr := &MockCrawlerLogMgr{}
	mockVisitLogMgr := &MockVisitLogMgr{}

	controller := NewSitesController(
		mockConfigMgr,
		mockSiteServerMgr,
		mockSiteHandler,
		mockRedisClient,
		mockMonitor,
		mockCrawlerLogMgr,
		mockVisitLogMgr,
		cfg,
	)

	router := gin.New()
	router.GET("/sites/:id/static", controller.GetStaticFiles)
	router.POST("/sites/:id/static/upload", controller.UploadStaticFile)
	router.POST("/sites/:id/static/extract", controller.ExtractFile)
	router.DELETE("/sites/:id/static", controller.DeleteStaticFile)
	router.POST("/sites/:id/static/batch-delete", controller.BatchDeleteStaticFiles)
	router.DELETE("/sites/:id", controller.DeleteSite)
	router.PUT("/sites/:id", controller.UpdateSite)
	router.PUT("/sites/:id/prerender", controller.UpdateSitePrerenderConfig)
	router.PUT("/sites/:id/push", controller.UpdateSitePushConfig)
	router.PUT("/sites/:id/firewall", controller.UpdateSiteFirewallConfig)

	return controller, router, mockConfigMgr, mockSiteServerMgr, mockRedisClient
}

func TestSitesController_DeleteSite_Success(t *testing.T) {
	_, router, mockConfigMgr, mockSiteServerMgr, mockRedisClient := setupSitesControllerWithMocks()
	defer os.RemoveAll("/tmp/test-static-mocks")

	// Set up delete funcs
	mockSiteServerMgr.stopSiteServerFunc = func(siteID string) error {
		return nil
	}
	mockRedisClient.deleteSiteDataFunc = func(siteID string) error {
		return nil
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/sites/test-site-1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, float64(200), response["code"])

	// Verify site was removed from config
	assert.Len(t, mockConfigMgr.config.Sites, 0)
}

func TestSitesController_GetStaticFiles_Success(t *testing.T) {
	_, router, _, _, _ := setupSitesControllerWithMocks()
	defer os.RemoveAll("/tmp/test-static-mocks")

	// Create test static files
	staticDir := "/tmp/test-static-mocks/test-site-1"
	os.MkdirAll(staticDir, 0755)
	os.WriteFile(staticDir+"/index.html", []byte("<html></html>"), 0644)
	os.WriteFile(staticDir+"/style.css", []byte("body{}"), 0644)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sites/test-site-1/static", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, float64(200), response["code"])

	// data is an array of file info
	data, ok := response["data"].([]interface{})
	assert.True(t, ok)
	assert.GreaterOrEqual(t, len(data), 2) // index.html and style.css
}

func TestSitesController_DeleteStaticFile_Success(t *testing.T) {
	_, router, _, _, _ := setupSitesControllerWithMocks()
	defer os.RemoveAll("/tmp/test-static-mocks")

	// Create test static file
	staticDir := "/tmp/test-static-mocks/test-site-1"
	os.MkdirAll(staticDir, 0755)
	os.WriteFile(staticDir+"/test.txt", []byte("test content"), 0644)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/sites/test-site-1/static?path=test.txt", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify file was deleted
	_, err := os.Stat(staticDir + "/test.txt")
	assert.True(t, os.IsNotExist(err))
}

func TestSitesController_BatchDeleteStaticFiles_Success(t *testing.T) {
	_, router, _, _, _ := setupSitesControllerWithMocks()
	defer os.RemoveAll("/tmp/test-static-mocks")

	// Create test static files
	staticDir := "/tmp/test-static-mocks/test-site-1"
	os.MkdirAll(staticDir, 0755)
	os.WriteFile(staticDir+"/file1.txt", []byte("content1"), 0644)
	os.WriteFile(staticDir+"/file2.txt", []byte("content2"), 0644)
	os.WriteFile(staticDir+"/file3.txt", []byte("content3"), 0644)

	deleteReq := map[string][]string{
		"paths": {"/file1.txt", "/file2.txt"},
	}
	body, _ := json.Marshal(deleteReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/sites/test-site-1/static/batch-delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify files were deleted
	_, err := os.Stat(staticDir + "/file1.txt")
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(staticDir + "/file2.txt")
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(staticDir + "/file3.txt")
	assert.NoError(t, err) // file3.txt should still exist
}

func TestSitesController_UploadStaticFile_Success(t *testing.T) {
	_, router, _, _, _ := setupSitesControllerWithMocks()
	defer os.RemoveAll("/tmp/test-static-mocks")

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	fileWriter, _ := writer.CreateFormFile("file", "uploaded.txt")
	fileWriter.Write([]byte("uploaded content"))
	writer.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/sites/test-site-1/static/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify file was uploaded
	staticDir := "/tmp/test-static-mocks/test-site-1"
	_, err := os.Stat(staticDir + "/uploaded.txt")
	assert.NoError(t, err)
}

func TestSitesController_ExtractFile_Success(t *testing.T) {
	_, router, _, _, _ := setupSitesControllerWithMocks()
	defer os.RemoveAll("/tmp/test-static-mocks")

	// Create a test zip file
	staticDir := "/tmp/test-static-mocks/test-site-1"
	os.MkdirAll(staticDir, 0755)

	zipPath := staticDir + "/test.zip"
	zipFile, _ := os.Create(zipPath)
	zipWriter := zip.NewWriter(zipFile)

	// Add a file to the zip
	fileWriter, _ := zipWriter.Create("extracted.txt")
	fileWriter.Write([]byte("extracted content"))
	zipWriter.Close()
	zipFile.Close()

	// Create form with filename and path
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("filename", "test.zip")
	writer.WriteField("path", "")
	writer.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/sites/test-site-1/static/extract", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify file was extracted
	_, err := os.Stat(staticDir + "/extracted.txt")
	assert.NoError(t, err)

	// Clean up
	os.Remove(zipPath)
}

func TestSitesController_UpdateSite_Success(t *testing.T) {
	_, router, mockConfigMgr, mockSiteServerMgr, _ := setupSitesControllerWithMocks()
	defer os.RemoveAll("/tmp/test-static-mocks")

	mockSiteServerMgr.stopSiteServerFunc = func(siteID string) error {
		return nil
	}

	updateReq := map[string]interface{}{
		"name":    "Updated Site Name",
		"domains": []string{"127.0.0.1"},
		"port":    19091, // Use port 19091 to avoid conflicts with common services
		"mode":    "static",
	}
	body, _ := json.Marshal(updateReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/sites/test-site-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify site was updated
	assert.Len(t, mockConfigMgr.config.Sites, 1)
	assert.Equal(t, "Updated Site Name", mockConfigMgr.config.Sites[0].Name)
}

func TestSitesController_UpdateSitePrerenderConfig_Success(t *testing.T) {
	_, router, _, mockSiteServerMgr, mockRedisClient := setupSitesControllerWithMocks()
	defer os.RemoveAll("/tmp/test-static-mocks")

	mockSiteServerMgr.stopSiteServerFunc = func(siteID string) error {
		return nil
	}

	updateReq := map[string]interface{}{
		"enabled":       true,
		"pool_size":     5,
		"min_pool_size": 2,
		"max_pool_size": 10,
		"timeout":       30,
		"cache_ttl":     3600,
	}
	body, _ := json.Marshal(updateReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/sites/test-site-1/prerender", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify Redis was called with prerender config
	assert.NotEmpty(t, mockRedisClient.storedStats)
	_, hasPrerenderConfig := mockRedisClient.storedStats["test-site-1_prerender"]
	assert.True(t, hasPrerenderConfig)
}

func TestSitesController_UpdateSitePushConfig_Success(t *testing.T) {
	_, router, _, mockSiteServerMgr, mockRedisClient := setupSitesControllerWithMocks()
	defer os.RemoveAll("/tmp/test-static-mocks")

	mockSiteServerMgr.stopSiteServerFunc = func(siteID string) error {
		return nil
	}

	updateReq := map[string]interface{}{
		"enabled":           true,
		"baidu_api":         "https://example.baidu.com",
		"baidu_token":       "test-token",
		"baidu_daily_limit": 1000,
	}
	body, _ := json.Marshal(updateReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/sites/test-site-1/push", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify Redis was called with push config
	_, hasPushConfig := mockRedisClient.storedStats["test-site-1_push"]
	assert.True(t, hasPushConfig)
}

func TestSitesController_UpdateSiteFirewallConfig_Success(t *testing.T) {
	_, router, _, mockSiteServerMgr, mockRedisClient := setupSitesControllerWithMocks()
	defer os.RemoveAll("/tmp/test-static-mocks")

	mockSiteServerMgr.stopSiteServerFunc = func(siteID string) error {
		return nil
	}

	updateReq := map[string]interface{}{
		"enabled": true,
		"action_config": map[string]interface{}{
			"default_action": "block",
			"block_message":  "Access denied",
		},
		"ratelimit_config": map[string]interface{}{
			"enabled":  true,
			"requests": 100,
			"window":   60,
			"ban_time": 300,
		},
	}
	body, _ := json.Marshal(updateReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/sites/test-site-1/firewall", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify Redis was called with firewall config
	_, hasWafConfig := mockRedisClient.storedStats["test-site-1_waf"]
	assert.True(t, hasWafConfig)
}

// TestValidateDomains_Format 域名格式校验表驱动测试
func TestValidateDomains_Format(t *testing.T) {
	valid := [][]string{
		{"localhost"},
		{"127.0.0.1"},
		{"www.example.com"},
		{"8.8.8.8"},
		{"sub.domain.example.cn", "example.com"},
		{"MySite.Example.COM"}, // 大小写不敏感，仅做格式校验
	}
	for _, domains := range valid {
		assert.NoError(t, validateDomains(domains), "should accept: %v", domains)
	}

	invalid := [][]string{
		{},
		nil,
		{""},
		{"  "},
		{"http://example.com"},
		{"example.com/path"},
		{"bad domain"},
		{"example.com?q=1"},
		{"ok", ""},
	}
	for _, domains := range invalid {
		assert.Error(t, validateDomains(domains), "should reject: %v", domains)
	}
}

// TestSitesController_UpdateSite_RealDomainAccepted 真实域名应通过校验，
// 随后在 configManager 为 nil 处返回 500（证明已越过域名校验）
func TestSitesController_UpdateSite_RealDomainAccepted(t *testing.T) {
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
	router.PUT("/sites/:id", controller.UpdateSite)

	site := map[string]interface{}{
		"name":    "Updated Site",
		"domains": []string{"production.example.com"},
		"port":    8080,
		"mode":    "static",
	}
	body, _ := json.Marshal(site)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/sites/test-site-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "domain format")
}
