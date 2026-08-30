package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"prerender-shield/internal/constants"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/repository"
)

// setupMonitoringWithClient 使用 DB15 Redis 构建监控控制器
func setupMonitoringWithClient(t *testing.T) (*MonitoringController, *gin.Engine) {
	t.Helper()
	client := newTestRedisDB15(t)
	cleanupKeys := func() {
		for _, k := range []string{constants.RedisKeyAlertHistory, "monitoring:alert-rules", "monitoring:notification-channels", "firewall:rules:ctl-site"} {
			client.Del(k)
		}
	}
	cleanupKeys()
	t.Cleanup(cleanupKeys)

	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: true})
	controller := NewMonitoringController(monitor, client)

	router := ginNewRouter()
	router.GET("/alerts/history", controller.GetAlertHistory)
	router.GET("/alerts/rules", controller.GetAlertRules)
	router.POST("/alerts/rules", controller.SaveAlertRule)
	router.DELETE("/alerts/rules/:id", controller.DeleteAlertRule)
	router.GET("/firewall-rules", controller.GetFirewallRules)
	router.POST("/firewall-rules", controller.SaveFirewallRules)
	router.DELETE("/firewall-rules/:id", controller.DeleteFirewallRule)
	router.GET("/notify", controller.GetNotificationChannels)
	router.POST("/notify", controller.SaveNotificationChannels)
	return controller, router
}

// setupMonitoringNilRedis 使用 nil Redis 客户端构建监控控制器（触发持久化失败分支）
func setupMonitoringNilRedis(t *testing.T) (*MonitoringController, *gin.Engine) {
	t.Helper()
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: true})
	controller := NewMonitoringController(monitor, nil)

	router := ginNewRouter()
	router.GET("/alerts/rules", controller.GetAlertRules)
	router.POST("/alerts/rules", controller.SaveAlertRule)
	router.DELETE("/alerts/rules/:id", controller.DeleteAlertRule)
	router.GET("/firewall-rules", controller.GetFirewallRules)
	router.POST("/firewall-rules", controller.SaveFirewallRules)
	router.DELETE("/firewall-rules/:id", controller.DeleteFirewallRule)
	router.GET("/notify", controller.GetNotificationChannels)
	router.POST("/notify", controller.SaveNotificationChannels)
	return controller, router
}

func TestMonitoringController_GetAlertHistory_LimitClamp(t *testing.T) {
	_, router := setupMonitoringWithClient(t)

	for _, q := range []string{"limit=abc", "limit=-1", "limit=99999"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/alerts/history?"+q, nil))
		assert.Equal(t, http.StatusOK, w.Code, q)
	}
}

func TestMonitoringController_GetAlertRules(t *testing.T) {
	controller, router := setupMonitoringWithClient(t)

	// 预置一条规则
	require.NoError(t, controller.alertRepo.SaveAlertRule(repository.AlertRuleData{
		ID: "rule-ctl-1", Name: "cpu", Metric: "cpu", Operator: ">", Threshold: 90, Enabled: true,
	}))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/alerts/rules", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data []repository.AlertRuleData `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "rule-ctl-1", resp.Data[0].ID)
}

func TestMonitoringController_SaveAlertRule_InvalidJSON(t *testing.T) {
	_, router := setupMonitoringWithClient(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/alerts/rules", strBody("bad"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMonitoringController_SaveAlertRule_NilRedis(t *testing.T) {
	_, router := setupMonitoringNilRedis(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/alerts/rules", jsonBody(t, map[string]interface{}{"id": "r1", "metric": "cpu"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestMonitoringController_SaveAlertRule_Success(t *testing.T) {
	_, router := setupMonitoringWithClient(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/alerts/rules",
		jsonBody(t, map[string]interface{}{"id": "rule-save-1", "name": "mem", "metric": "memory", "operator": ">", "threshold": 95, "enabled": true}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")
}

func TestMonitoringController_DeleteAlertRule_MissingID(t *testing.T) {
	_, router := setupMonitoringWithClient(t)

	// :id 为空时路由不匹配 → 直接用双斜杠路径仍然 404 路由级；用显式空段命中 handler
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/alerts/rules/", nil))
	// gin 会将 /alerts/rules/ 视为无 :id 的路由 404，这里允许 404 或 400
	assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusBadRequest)
}

func TestMonitoringController_DeleteAlertRule_NilRedis(t *testing.T) {
	_, router := setupMonitoringNilRedis(t)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/alerts/rules/r1", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestMonitoringController_DeleteAlertRule_Success(t *testing.T) {
	controller, router := setupMonitoringWithClient(t)
	require.NoError(t, controller.alertRepo.SaveAlertRule(repository.AlertRuleData{ID: "rule-del-1", Metric: "cpu"}))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/alerts/rules/rule-del-1", nil))
	require.Equal(t, http.StatusOK, w.Code)

	rules := controller.alertRepo.GetAlertRules()
	assert.Empty(t, rules)
}

func TestMonitoringController_GetFirewallRules(t *testing.T) {
	controller, router := setupMonitoringWithClient(t)

	// 缺 site_id → 400
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/firewall-rules", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 无数据 → 空 rules
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/firewall-rules?site_id=ctl-none", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"rules":[]`)

	// 有数据 → 返回保存的结构
	require.NoError(t, controller.fwRulesRepo.Save("ctl-site", []byte(`{"rules":[{"id":"fr1","action":"block"}]}`)))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/firewall-rules?site_id=ctl-site", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "fr1")
}

func TestMonitoringController_SaveFirewallRules(t *testing.T) {
	controller, router := setupMonitoringWithClient(t)

	// 非法 JSON → 400
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/firewall-rules", strBody("bad"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 缺 site_id → 400
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/firewall-rules", jsonBody(t, map[string]interface{}{"rules": []interface{}{}}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 成功保存
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/firewall-rules",
		jsonBody(t, map[string]interface{}{"site_id": "ctl-site", "rules": []interface{}{map[string]interface{}{"id": "fr2"}}}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")

	data, err := controller.fwRulesRepo.Get("ctl-site")
	require.NoError(t, err)
	assert.NotNil(t, data)
}

func TestMonitoringController_DeleteFirewallRule(t *testing.T) {
	controller, router := setupMonitoringWithClient(t)
	require.NoError(t, controller.fwRulesRepo.Save("ctl-site", []byte(`{"rules":[{"id":"fr-del"},{"id":"fr-keep"}]}`)))

	// 缺参数 → 400
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/firewall-rules/fr-del", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 成功删除
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/firewall-rules/fr-del?site_id=ctl-site", nil))
	require.Equal(t, http.StatusOK, w.Code)

	data, _ := controller.fwRulesRepo.Get("ctl-site")
	rules, _ := data["rules"].([]interface{})
	require.Len(t, rules, 1)
}

func TestMonitoringController_GetNotificationChannels(t *testing.T) {
	_, router := setupMonitoringWithClient(t)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/notify", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")
}

func TestMonitoringController_SaveNotificationChannels(t *testing.T) {
	controller, router := setupMonitoringWithClient(t)

	// 非法 JSON → 400
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/notify", strBody("bad"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 成功保存
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/notify",
		jsonBody(t, []repository.NotificationChannelData{{Type: "webhook", URL: "http://hook", Enabled: true}}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	channels := controller.notifyRepo.Get()
	require.Len(t, channels, 1)
	assert.Equal(t, "webhook", channels[0].Type)
}

func TestMonitoringController_SaveNotificationChannels_NilRedis(t *testing.T) {
	_, router := setupMonitoringNilRedis(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/notify", jsonBody(t, []repository.NotificationChannelData{}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
