package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/repository"
)

func TestOverviewController_GetOverview(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{ID: "site1", Name: "Test Site 1", Firewall: config.FirewallConfig{Enabled: true}, Prerender: config.PrerenderConfig{Enabled: true}},
			{ID: "site2", Name: "Test Site 2", Firewall: config.FirewallConfig{Enabled: false}, Prerender: config.PrerenderConfig{Enabled: false}},
		},
	}

	monitor := monitoring.NewMonitor(monitoring.Config{})
	visitLogMgr := logging.NewVisitLogManager("")

	controller := &OverviewController{
		cfg:         cfg,
		monitor:     monitor,
		visitLogMgr: visitLogMgr,
		wafRepo:     nil, // 使用 nil wafRepo 进行测试
	}

	router := gin.New()
	router.GET("/overview", controller.GetOverview)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/overview", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, float64(200), response["code"])
	assert.Equal(t, "success", response["message"])

	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)

	// 验证基本字段存在
	assert.NotNil(t, data["totalRequests"])
	assert.NotNil(t, data["crawlerRequests"])
	assert.NotNil(t, data["blockedRequests"])
	assert.NotNil(t, data["cacheHitRate"])
	assert.NotNil(t, data["activeBrowsers"])
	assert.Equal(t, float64(2), data["activeSites"])
	assert.Equal(t, float64(0), data["sslCertificates"])
	assert.Equal(t, true, data["firewallEnabled"])
	assert.Equal(t, true, data["prerenderEnabled"])

	// 验证 geoData
	geoData, ok := data["geoData"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, geoData["countryData"])
	assert.NotNil(t, geoData["mapData"])
	assert.NotNil(t, geoData["globeData"])

	// 验证 accessStats
	accessStats, ok := data["accessStats"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, accessStats["pv"])
	assert.NotNil(t, accessStats["uv"])
	assert.NotNil(t, accessStats["ip"])
}

func TestOverviewController_GetOverview_EmptySites(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Sites: []config.SiteConfig{},
	}

	monitor := monitoring.NewMonitor(monitoring.Config{})
	visitLogMgr := logging.NewVisitLogManager("")

	controller := &OverviewController{
		cfg:         cfg,
		monitor:     monitor,
		visitLogMgr: visitLogMgr,
	}

	router := gin.New()
	router.GET("/overview", controller.GetOverview)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/overview", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(0), data["activeSites"])
	assert.Equal(t, false, data["firewallEnabled"])
	assert.Equal(t, false, data["prerenderEnabled"])
}

func TestOverviewController_GetOverview_NilWafRepo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{ID: "site1", Name: "Test Site 1", Firewall: config.FirewallConfig{Enabled: true}},
		},
	}

	monitor := monitoring.NewMonitor(monitoring.Config{})
	visitLogMgr := logging.NewVisitLogManager("")

	controller := &OverviewController{
		cfg:         cfg,
		monitor:     monitor,
		visitLogMgr: visitLogMgr,
		wafRepo:     nil,
	}

	router := gin.New()
	router.GET("/overview", controller.GetOverview)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/overview", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNewOverviewController(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{ID: "site1", Name: "Test Site"},
		},
	}

	monitor := monitoring.NewMonitor(monitoring.Config{})
	visitLogMgr := logging.NewVisitLogManager("")
	wafRepo := &repository.WafRepository{}

	controller := NewOverviewController(cfg, monitor, visitLogMgr, wafRepo)

	assert.NotNil(t, controller)
	assert.Equal(t, cfg, controller.cfg)
	assert.Equal(t, monitor, controller.monitor)
	assert.Equal(t, visitLogMgr, controller.visitLogMgr)
	assert.Equal(t, wafRepo, controller.wafRepo)
}

func TestOverviewController_GetOverview_WithWafRepo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{ID: "site1", Name: "Test Site 1", Firewall: config.FirewallConfig{Enabled: true}, Prerender: config.PrerenderConfig{Enabled: true}},
		},
	}

	monitor := monitoring.NewMonitor(monitoring.Config{})
	visitLogMgr := logging.NewVisitLogManager("")

	controller := NewOverviewController(cfg, monitor, visitLogMgr, nil)

	router := gin.New()
	router.GET("/overview", controller.GetOverview)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/overview", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, float64(200), response["code"])
}

func TestOverviewController_GetOverview_MultipleSites(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{ID: "site1", Name: "Site 1", Firewall: config.FirewallConfig{Enabled: true}, Prerender: config.PrerenderConfig{Enabled: false}},
			{ID: "site2", Name: "Site 2", Firewall: config.FirewallConfig{Enabled: false}, Prerender: config.PrerenderConfig{Enabled: true}},
			{ID: "site3", Name: "Site 3", Firewall: config.FirewallConfig{Enabled: true}, Prerender: config.PrerenderConfig{Enabled: true}},
		},
	}

	monitor := monitoring.NewMonitor(monitoring.Config{})
	visitLogMgr := logging.NewVisitLogManager("")

	controller := NewOverviewController(cfg, monitor, visitLogMgr, nil)

	router := gin.New()
	router.GET("/overview", controller.GetOverview)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/overview", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(3), data["activeSites"])
	assert.Equal(t, true, data["firewallEnabled"])
	assert.Equal(t, true, data["prerenderEnabled"])
}
