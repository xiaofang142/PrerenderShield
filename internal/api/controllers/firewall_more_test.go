package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"prerender-shield/internal/models"
)

// fakeWafRepo 可编程 WAF 仓储 mock：逐方法注入错误/返回值
type fakeWafRepo struct {
	config      *models.WafConfig
	getErr      bool
	updateErr   bool
	accessErr   bool
	attackErr   bool
	blackErr    bool
	whiteErr    bool
	addWhiteErr bool
	addBlackErr bool

	blacklist  []string
	whitelist  []string
	accessLogs []models.AccessLog
	attackLogs []models.AccessLog
}

func (f *fakeWafRepo) GetWafConfigBySiteID(siteID string) (*models.WafConfig, error) {
	if f.getErr {
		return nil, errors.New("db down")
	}
	return f.config, nil
}

func (f *fakeWafRepo) UpdateWafConfig(config *models.WafConfig) error {
	if f.updateErr {
		return errors.New("db write failed")
	}
	f.config = config
	return nil
}

func (f *fakeWafRepo) GetAccessLogs(siteID string, page, limit int) ([]models.AccessLog, int64, error) {
	if f.accessErr {
		return nil, 0, errors.New("db read failed")
	}
	return f.accessLogs, int64(len(f.accessLogs)), nil
}

func (f *fakeWafRepo) GetAttackLogs(siteID string, page, limit int) ([]models.AccessLog, int64, error) {
	if f.attackErr {
		return nil, 0, errors.New("db read failed")
	}
	return f.attackLogs, int64(len(f.attackLogs)), nil
}

func (f *fakeWafRepo) AddIPToWhitelist(siteID, ip string) error {
	if f.addWhiteErr {
		return errors.New("insert failed")
	}
	return nil
}

func (f *fakeWafRepo) AddIPToBlacklist(siteID, ip string) error {
	if f.addBlackErr {
		return errors.New("insert failed")
	}
	return nil
}

func (f *fakeWafRepo) GetBlacklist(siteID string) ([]string, error) {
	if f.blackErr {
		return nil, errors.New("query failed")
	}
	return f.blacklist, nil
}

func (f *fakeWafRepo) GetWhitelist(siteID string) ([]string, error) {
	if f.whiteErr {
		return nil, errors.New("query failed")
	}
	return f.whitelist, nil
}

func setupFakeFirewall(repo *fakeWafRepo) (*FirewallController, *gin.Engine) {
	controller := NewFirewallController(repo)
	router := ginNewRouter()
	router.GET("/sites/:id/waf", controller.GetWafConfig)
	router.PUT("/sites/:id/waf", controller.UpdateWafConfig)
	router.GET("/logs", controller.GetAccessLogs)
	router.GET("/attacks", controller.GetAttackLogs)
	router.GET("/blacklist", controller.GetBlacklist)
	router.GET("/whitelist", controller.GetWhitelist)
	router.POST("/whitelist", controller.AddToWhitelist)
	router.POST("/blacklist", controller.AddToBlacklist)
	router.GET("/export", controller.ExportLogs)
	return controller, router
}

func TestFirewallController_GetWafConfig_RepoError(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{getErr: true})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites/s1/waf", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestFirewallController_GetWafConfig_Found(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{config: &models.WafConfig{SiteID: "s1", Enabled: true}})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites/s1/waf", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"enabled":true`)
}

func TestFirewallController_UpdateWafConfig_GetError(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{getErr: true})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/sites/s1/waf", jsonBody(t, map[string]interface{}{"enabled": true}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestFirewallController_UpdateWafConfig_UpdateError(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{updateErr: true})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/sites/s1/waf", jsonBody(t, map[string]interface{}{"enabled": true}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to update WAF config")
}

func TestFirewallController_GetAccessLogs_RepoError(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{accessErr: true})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/logs?site_id=s1", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestFirewallController_GetAttackLogs_RepoError(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{attackErr: true})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/attacks?site_id=s1", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestFirewallController_GetLogs_PaginationClamp page/limit 非法回退默认值
func TestFirewallController_GetLogs_PaginationClamp(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/logs?page=0&limit=0", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data struct {
			Page  int `json:"page"`
			Limit int `json:"limit"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Data.Page)
	assert.Equal(t, 20, resp.Data.Limit)
}

func TestFirewallController_GetBlacklist_MissingSiteID(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/blacklist", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFirewallController_GetBlacklist_RepoError(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{blackErr: true})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/blacklist?site_id=s1", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestFirewallController_GetBlacklist_Success(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{blacklist: []string{"10.0.0.1", "10.0.0.2"}})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/blacklist?site_id=s1", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "10.0.0.1")
}

func TestFirewallController_GetWhitelist_MissingSiteID(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/whitelist", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFirewallController_GetWhitelist_RepoError(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{whiteErr: true})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/whitelist?site_id=s1", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestFirewallController_GetWhitelist_Success(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{whitelist: []string{"192.168.0.1"}})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/whitelist?site_id=s1", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "192.168.0.1")
}

func TestFirewallController_AddToWhitelist_InvalidJSON(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/whitelist", strBody("bad json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFirewallController_AddToWhitelist_RepoError(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{addWhiteErr: true})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/whitelist", jsonBody(t, map[string]string{"site_id": "s1", "ip": "1.2.3.4"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestFirewallController_AddToBlacklist_InvalidJSON(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/blacklist", strBody("bad json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFirewallController_AddToBlacklist_RepoError(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{addBlackErr: true})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/blacklist", jsonBody(t, map[string]string{"site_id": "s1", "ip": "1.2.3.4"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestFirewallController_ExportLogs_MissingSiteID(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{})

	// site_id 缺省 → "default" 兜底，正常导出表头
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/export", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ID,IP,Method,Path,Status,Action,Reason,CreatedAt")
}

func TestFirewallController_ExportLogs_RepoError(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{accessErr: true})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/export?site_id=s1", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestFirewallController_ExportLogs_WithRows(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{accessLogs: []models.AccessLog{
		{ID: "log1", IPAddress: "1.1.1.1", Method: "GET", RequestPath: "/a", StatusCode: 403, Action: "block", Reason: "blacklist"},
	}})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/export?site_id=s1", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "log1,1.1.1.1,GET,/a,403,block,blacklist")
}
