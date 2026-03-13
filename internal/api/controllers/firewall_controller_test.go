package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/models"
	"prerender-shield/internal/repository"
)

func setupFirewallController() (*FirewallController, *gin.Engine) {
	gin.SetMode(gin.TestMode)

	// 创建内存数据库的 repository
	wafRepo := repository.NewWafRepositoryInMemory()
	controller := NewFirewallController(wafRepo)

	router := gin.New()
	router.GET("/sites/:id/waf", controller.GetWafConfig)
	router.PUT("/sites/:id/waf", controller.UpdateWafConfig)
	router.GET("/logs", controller.GetAccessLogs)
	router.GET("/firewall/attacks", controller.GetAttackLogs)
	router.POST("/firewall/whitelist", controller.AddToWhitelist)
	router.POST("/firewall/blacklist", controller.AddToBlacklist)

	return controller, router
}

func TestFirewallController_GetWafConfig_NoSiteID(t *testing.T) {
	_, router := setupFirewallController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sites//waf", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFirewallController_GetWafConfig_NotFound(t *testing.T) {
	_, router := setupFirewallController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sites/nonexistent/waf", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, true, response["success"])
}

func TestFirewallController_UpdateWafConfig_InvalidJSON(t *testing.T) {
	_, router := setupFirewallController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/sites/test-site/waf", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFirewallController_UpdateWafConfig_Success(t *testing.T) {
	_, router := setupFirewallController()

	configData := map[string]interface{}{
		"enabled":           true,
		"rate_limit_count":  100,
		"rate_limit_window": 60,
		"blocked_countries": []string{"US", "CN"},
		"whitelist_ips":     []string{"192.168.1.1"},
		"blacklist_ips":     []string{"10.0.0.1"},
		"custom_block_page": "<h1>Blocked</h1>",
	}
	body, _ := json.Marshal(configData)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/sites/test-site/waf", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, true, response["success"])
}

func TestFirewallController_GetAccessLogs(t *testing.T) {
	_, router := setupFirewallController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/logs?site_id=test-site&page=1&limit=10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, true, response["success"])
}

func TestFirewallController_GetAttackLogs(t *testing.T) {
	_, router := setupFirewallController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/firewall/attacks?site_id=test-site&page=1&limit=10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, true, response["success"])
}

func TestFirewallController_AddToWhitelist_Success(t *testing.T) {
	_, router := setupFirewallController()

	data := map[string]string{
		"site_id": "test-site",
		"ip":      "192.168.1.100",
	}
	body, _ := json.Marshal(data)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/firewall/whitelist", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, true, response["success"])
}

func TestFirewallController_AddToWhitelist_MissingFields(t *testing.T) {
	_, router := setupFirewallController()

	data := map[string]string{
		"site_id": "",
		"ip":      "",
	}
	body, _ := json.Marshal(data)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/firewall/whitelist", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFirewallController_AddToBlacklist_Success(t *testing.T) {
	_, router := setupFirewallController()

	data := map[string]string{
		"site_id": "test-site",
		"ip":      "10.0.0.100",
	}
	body, _ := json.Marshal(data)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/firewall/blacklist", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, true, response["success"])
}

func TestFirewallController_AddToBlacklist_MissingFields(t *testing.T) {
	_, router := setupFirewallController()

	data := map[string]string{
		"site_id": "",
		"ip":      "",
	}
	body, _ := json.Marshal(data)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/firewall/blacklist", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFirewallController_AddToWhitelist_MissingIP(t *testing.T) {
	_, router := setupFirewallController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/firewall/whitelist",
		bytes.NewBufferString(`{"siteId":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFirewallController_AddToWhitelist_InvalidIP(t *testing.T) {
	_, router := setupFirewallController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/firewall/whitelist",
		bytes.NewBufferString(`{"siteId":"test","ip":"invalid"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFirewallController_AddToBlacklist_MissingIP(t *testing.T) {
	_, router := setupFirewallController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/firewall/blacklist",
		bytes.NewBufferString(`{"siteId":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFirewallController_AddToBlacklist_InvalidIP(t *testing.T) {
	_, router := setupFirewallController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/firewall/blacklist",
		bytes.NewBufferString(`{"siteId":"test","ip":"invalid"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFirewallController_GetAccessLogs_NoSiteID(t *testing.T) {
	_, router := setupFirewallController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/logs", nil)
	router.ServeHTTP(w, req)

	// 没有 siteId 时返回空列表 200
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFirewallController_GetAttackLogs_NoSiteID(t *testing.T) {
	_, router := setupFirewallController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/firewall/attacks", nil)
	router.ServeHTTP(w, req)

	// 没有 siteId 时返回空列表 200
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNewFirewallController(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := repository.NewWafRepositoryInMemory()
	controller := NewFirewallController(repo)

	assert.NotNil(t, controller)
	assert.NotNil(t, controller.wafRepo)
}

func TestFirewallController_GetWafConfig_EmptySiteID(t *testing.T) {
	_, router := setupFirewallController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sites//waf", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFirewallController_UpdateWafConfig_NoSiteID(t *testing.T) {
	_, router := setupFirewallController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/sites//waf", bytes.NewBufferString(`{"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFirewallController_GetAccessLogs_WithPagination(t *testing.T) {
	_, router := setupFirewallController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/logs?site=test&page=1&pageSize=10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFirewallController_GetAttackLogs_WithPagination(t *testing.T) {
	_, router := setupFirewallController()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/firewall/attacks?site=test&page=1&pageSize=10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
func TestWafRepositoryInMemory(t *testing.T) {
	repo := repository.NewWafRepositoryInMemory()

	// Test GetWafConfigBySiteID - empty
	config, err := repo.GetWafConfigBySiteID("test-site")
	assert.NoError(t, err)
	assert.Nil(t, config)

	// Test UpdateWafConfig
	newConfig := &models.WafConfig{
		SiteID:          "test-site",
		Enabled:         true,
		RateLimitCount:  100,
		RateLimitWindow: 60,
		CustomBlockPage: "<h1>Blocked</h1>",
	}
	err = repo.UpdateWafConfig(newConfig)
	assert.NoError(t, err)

	// Test GetWafConfigBySiteID - should return config
	config, err = repo.GetWafConfigBySiteID("test-site")
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, true, config.Enabled)
	assert.Equal(t, 100, config.RateLimitCount)

	// Test AddIPToWhitelist
	err = repo.AddIPToWhitelist("test-site", "192.168.1.1")
	assert.NoError(t, err)

	// Test AddIPToBlacklist
	err = repo.AddIPToBlacklist("test-site", "10.0.0.1")
	assert.NoError(t, err)

	// Test GetAccessLogs
	logs, total, err := repo.GetAccessLogs("test-site", 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, logs)

	// Test GetAttackLogs
	attackLogs, total, err := repo.GetAttackLogs("test-site", 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, attackLogs)
}
