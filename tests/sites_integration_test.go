package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	"prerender-shield/internal/api/controllers"
	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
	sitehandler "prerender-shield/internal/site-handler"
	siteserver "prerender-shield/internal/site-server"
)

func setupTestEnv(t *testing.T) (*gin.Engine, *controllers.SitesController, string) {
	// Create temporary directory for config and static files
	tmpDir, err := os.MkdirTemp("", "prerender-shield-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create config file
	configFile := filepath.Join(tmpDir, "config.yaml")
	staticDir := filepath.Join(tmpDir, "static")
	os.MkdirAll(staticDir, 0755)

	// Initialize Config
	cfg := &config.Config{
		Server: config.ServerConfig{
			Address: "127.0.0.1",
		},
		Dirs: config.DirsConfig{
			StaticDir: staticDir,
		},
		Sites: []config.SiteConfig{},
	}

	// Save initial config to file (YAML)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	os.WriteFile(configFile, data, 0644)

	// Initialize ConfigManager
	// We use LoadConfig to initialize the singleton and load the file
	loadedCfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	configManager := config.GetInstance()

	// Initialize Dependencies
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})
	
	// Use empty string to avoid connection attempts if not needed, 
	// or localhost if we want to try (but might fail)
	crawlerLogMgr := logging.NewCrawlerLogManager("") 
	visitLogMgr := logging.NewVisitLogManager("")

	// Initialize SiteHandler with nil dependencies (PrerenderManager, WafRepo, RedisClient, GeoIP)
	// This is risky but works for basic CRUD tests where we don't trigger WAF blocking or Prerender
	siteHandler := sitehandler.NewHandler(nil, nil, nil, nil)
	
	siteServerMgr := siteserver.NewManager(monitor)

	// Initialize Controller
	sitesController := controllers.NewSitesController(
		configManager,
		siteServerMgr,
		siteHandler,
		nil, // RedisClient
		monitor,
		crawlerLogMgr,
		visitLogMgr,
		loadedCfg,
	)

	// Setup Gin
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	
	// Register Routes
	r.GET("/api/v1/sites", sitesController.GetSites)
	r.POST("/api/v1/sites", sitesController.AddSite)
	r.PUT("/api/v1/sites/:id", sitesController.UpdateSite)
	r.DELETE("/api/v1/sites/:id", sitesController.DeleteSite)

	return r, sitesController, tmpDir
}

func TestSitesCRUD(t *testing.T) {
	router, _, tmpDir := setupTestEnv(t)
	defer os.RemoveAll(tmpDir)

	// Use a random high port to avoid conflicts
	testPort := 50000 + (time.Now().UnixNano() % 10000)

	// 1. Test Add Site
	newSite := config.SiteConfig{
		Name:    "Test Site",
		Domains: []string{"localhost"},
		Port:    int(testPort),
		Mode:    "static",
	}
	body, _ := json.Marshal(newSite)
	req, _ := http.NewRequest("POST", "/api/v1/sites", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert Add Site
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 200.0, response["code"])
	
	siteData := response["data"].(map[string]interface{})
	siteID := siteData["id"].(string)
	assert.NotEmpty(t, siteID)
	assert.Equal(t, "Test Site", siteData["name"])

	// 2. Test Get Sites
	req, _ = http.NewRequest("GET", "/api/v1/sites", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	json.Unmarshal(w.Body.Bytes(), &response)
	sites := response["data"].([]interface{})
	assert.Equal(t, 1, len(sites))

	// 3. Test Update Site
	// Use a new port for update
	updatedPort := testPort + 1
	updateSite := config.SiteConfig{
		Name:    "Updated Test Site",
		Domains: []string{"localhost"},
		Port:    int(updatedPort),
		Mode:    "static",
	}
	body, _ = json.Marshal(updateSite)
	req, _ = http.NewRequest("PUT", "/api/v1/sites/"+siteID, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	json.Unmarshal(w.Body.Bytes(), &response)
	siteData = response["data"].(map[string]interface{})
	assert.Equal(t, "Updated Test Site", siteData["name"])

	// 4. Test Delete Site
	req, _ = http.NewRequest("DELETE", "/api/v1/sites/"+siteID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	
	// Verify deletion
	req, _ = http.NewRequest("GET", "/api/v1/sites", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &response)
	sites = response["data"].([]interface{})
	assert.Equal(t, 0, len(sites))
}
