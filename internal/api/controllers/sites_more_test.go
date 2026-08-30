package controllers

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/site-handler"
	siteserver "prerender-shield/internal/site-server"
)

// freePort 找一个当前可用的 TCP 端口
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	return port
}

// TestConfigManagerWrapper_Delegates configManagerWrapper 四个委托方法
func TestConfigManagerWrapper_Delegates(t *testing.T) {
	config.ResetInstance()
	defer config.ResetInstance()

	w := &configManagerWrapper{cm: config.GetInstance()}
	require.NotNil(t, w.GetConfig())

	cfg := w.GetConfig()
	cfg.Sites = []config.SiteConfig{{ID: "wrap-site", Name: "Wrap"}}
	w.UpdateConfig(cfg)
	assert.Equal(t, "wrap-site", w.GetConfig().Sites[0].ID)

	// 默认实例无 configPath → SaveConfig 返回 nil
	assert.NoError(t, w.SaveConfig())

	// Mutate 事务性修改
	err := w.Mutate(func(c *config.Config) (*config.Config, error) {
		c.Sites = append(c.Sites, config.SiteConfig{ID: "wrap-site-2"})
		return c, nil
	})
	assert.NoError(t, err)
	assert.Len(t, w.GetConfig().Sites, 2)
}

// TestRedisClientWrapper_Delegates redisClientWrapper 委托方法（DB15）
func TestRedisClientWrapper_Delegates(t *testing.T) {
	client := newTestRedisDB15(t)
	w := &redisClientWrapper{client: client}

	require.NoError(t, w.SetSiteStats("ctl-wrap", map[string]interface{}{"name": "W"}))
	vals, err := w.GetSiteStats("ctl-wrap")
	require.NoError(t, err)
	assert.Equal(t, "W", vals["name"])

	require.NoError(t, w.AddURL("ctl-wrap", "http://x/1"))
	t.Cleanup(func() {
		client.Del("site:ctl-wrap:stats")
		client.Del("site:ctl-wrap:urls")
		client.Del("site:ctl-wrap:prerender")
		client.Del("site:ctl-wrap:push")
		client.Del("site:ctl-wrap:waf")
		client.Del("site:ctl-del:stats")
	})

	require.NoError(t, w.DeleteSiteData("ctl-wrap"))
}

// TestMonitorAndLogWrappers monitor/crawler/visit 包装器委托
func TestMonitorAndLogWrappers(t *testing.T) {
	client := newTestRedisDB15(t)

	monitorCtrl := &SitesController{concreteMonitor: monitoring.NewMonitor(monitoring.Config{Enabled: true})}
	assert.NotNil(t, (&monitorWrapper{ctrl: monitorCtrl}).GetStats())

	crawlerCtrl := &SitesController{concreteCrawlerLogMgr: logging.NewCrawlerLogManagerWithClient(client.GetRawClient())}
	(&crawlerLogMgrWrapper{ctrl: crawlerCtrl}).RecordCrawlerLog(logging.CrawlerLog{ID: "wrap-c1", Route: "/w"})

	visitCtrl := &SitesController{concreteVisitLogMgr: logging.NewVisitLogManagerWithClient(client.GetRawClient())}
	(&visitLogMgrWrapper{ctrl: visitCtrl}).RecordVisitLog(logging.VisitLog{ID: "wrap-v1", URL: "/w"})
}

// TestSiteServerMgrWrapper_Delegates 站点服务器包装器（真实启动+停止，端口 0）
func TestSiteServerMgrWrapper_Delegates(t *testing.T) {
	mgr := siteserver.NewManager(nil, nil)
	ctrl := &SitesController{concreteSiteServerMgr: mgr}
	w := &siteServerMgrWrapper{ctrl: ctrl}

	// 未启动时查询/停止均为空操作
	_, exists := w.GetSiteServer("never")
	assert.False(t, exists)
	assert.NoError(t, w.StopSiteServer("never"))

	// 启动一个真实站点服务器（随机端口），随后停止
	site := config.SiteConfig{ID: "ctl-wrap-srv", Name: "WrapSrv", Port: freePort(t)}
	staticDir := t.TempDir()
	w.StartSiteServer(site, "127.0.0.1", staticDir, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	srv, exists := w.GetSiteServer("ctl-wrap-srv")
	require.True(t, exists)
	require.NotNil(t, srv)
	require.NoError(t, w.StopSiteServer("ctl-wrap-srv"))
}

// TestSiteHandlerWrapper_Delegates siteHandler 包装器委托
func TestSiteHandlerWrapper_Delegates(t *testing.T) {
	ctrl := &SitesController{concreteSiteHandler: &sitehandler.Handler{}}
	w := &siteHandlerWrapper{ctrl: ctrl}
	handler := w.CreateSiteHandler(config.SiteConfig{ID: "ctl-wrap-h"}, nil, nil, nil, t.TempDir())
	require.NotNil(t, handler)
}

// TestNewSitesControllerWithConcreteDeps 全依赖注入构造 + GetSites/GetSite 走 configManager 路径
func TestNewSitesControllerWithConcreteDeps(t *testing.T) {
	config.ResetInstance()
	defer config.ResetInstance()

	cfg := &config.Config{
		Sites: []config.SiteConfig{{ID: "ctl-concrete", Name: "Concrete", Domains: []string{"127.0.0.1"}, Port: freePort(t)}},
		Dirs:  config.DirsConfig{StaticDir: t.TempDir()},
	}
	cm := config.GetInstance()
	cm.UpdateConfig(cfg)

	controller := NewSitesControllerWithConcreteDeps(
		cm,
		siteserver.NewManager(nil, nil),
		&sitehandler.Handler{},
		nil, nil, nil, nil,
		cfg,
	)
	require.NotNil(t, controller)
	require.NotNil(t, controller.configManager)

	router := ginNewRouter()
	router.GET("/sites", controller.GetSites)
	router.GET("/sites/:id", controller.GetSite)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ctl-concrete")

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites/ctl-concrete", nil))
	require.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites/ghost", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestSitesController_StartSite 启动站点：已运行 / 启动成功 / 站点不存在
func TestSitesController_StartSite(t *testing.T) {
	cfg := &config.Config{
		Sites:  []config.SiteConfig{{ID: "ctl-start", Name: "Start", Domains: []string{"127.0.0.1"}, Port: freePort(t)}},
		Dirs:   config.DirsConfig{StaticDir: t.TempDir()},
		Server: config.ServerConfig{Address: "127.0.0.1"},
	}
	mockCM := &MockConfigManager{config: cfg}
	mockSSM := &MockSiteServerMgr{servers: map[string]*http.Server{}}
	controller := NewSitesController(mockCM, mockSSM, &MockSiteHandler{}, nil, nil, nil, nil, cfg)

	router := ginNewRouter()
	router.POST("/sites/:id/start", controller.StartSite)

	// 站点不存在 → 404
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/sites/ghost/start", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)

	// 站点已在运行 → already running
	mockSSM.servers["ctl-start"] = &http.Server{}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/sites/ctl-start/start", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "already running")

	// 未运行 → 启动成功
	delete(mockSSM.servers, "ctl-start")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/sites/ctl-start/start", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Site started successfully")
	_, running := mockSSM.GetSiteServer("ctl-start")
	assert.True(t, running)
}

// TestSitesController_StopSite 停止站点：失败 / 成功 / 站点不存在
func TestSitesController_StopSite(t *testing.T) {
	cfg := &config.Config{Sites: []config.SiteConfig{{ID: "ctl-stop", Name: "Stop"}}}
	mockCM := &MockConfigManager{config: cfg}
	mockSSM := &MockSiteServerMgr{servers: map[string]*http.Server{}}
	controller := NewSitesController(mockCM, mockSSM, &MockSiteHandler{}, nil, nil, nil, nil, cfg)

	router := ginNewRouter()
	router.POST("/sites/:id/stop", controller.StopSite)

	// 站点不存在 → 404
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/sites/ghost/stop", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)

	// 停止失败 → 500
	mockSSM.stopSiteServerFunc = func(siteID string) error {
		return fmt.Errorf("shutdown failed")
	}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/sites/ctl-stop/stop", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// 停止成功 → 200
	mockSSM.stopSiteServerFunc = nil
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/sites/ctl-stop/stop", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Site stopped successfully")
}

// TestSitesController_AddSite_Success 全链路添加站点（事务 Mutate + 处理器创建 + Redis 持久化）
func TestSitesController_AddSite_Success(t *testing.T) {
	staticDir := t.TempDir()
	cfg := &config.Config{
		Sites:  []config.SiteConfig{},
		Dirs:   config.DirsConfig{StaticDir: staticDir},
		Server: config.ServerConfig{Address: "127.0.0.1"},
	}
	port := freePort(t)

	mockCM := &MockConfigManager{config: cfg}
	mockRedis := &MockRedisClient{storedStats: map[string]map[string]interface{}{}}
	controller := NewSitesController(mockCM, &MockSiteServerMgr{}, &MockSiteHandler{}, mockRedis, nil, nil, nil, cfg)

	router := ginNewRouter()
	router.POST("/sites", controller.AddSite)

	body := map[string]interface{}{
		"name":    "Added Site",
		"domains": []string{"added.example"},
		"port":    port,
		"mode":    "static",
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sites", jsonBody(t, body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Site added successfully")

	// 站点已入配置，且带生成的 UUID
	require.Len(t, mockCM.config.Sites, 1)
	assert.NotEmpty(t, mockCM.config.Sites[0].ID)

	// Redis 站点配置持久化被调用
	assert.NotEmpty(t, mockRedis.storedStats)
}

// TestSitesController_AddSite_LicenseLimit 免费版仅 1 站点 → 402
func TestSitesController_AddSite_LicenseLimit(t *testing.T) {
	cfg := &config.Config{
		Sites:      []config.SiteConfig{{ID: "existing", Name: "Existing", Domains: []string{"a.example"}}},
		Commercial: config.CommercialConfig{MaxSites: 1},
		Dirs:       config.DirsConfig{StaticDir: t.TempDir()},
	}
	mockCM := &MockConfigManager{config: cfg}
	controller := NewSitesController(mockCM, nil, &MockSiteHandler{}, nil, nil, nil, nil, cfg)

	router := ginNewRouter()
	router.POST("/sites", controller.AddSite)

	body := map[string]interface{}{
		"name":    "Second Site",
		"domains": []string{"b.example"},
		"port":    freePort(t),
		"mode":    "static",
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sites", jsonBody(t, body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusPaymentRequired, w.Code)
	assert.Contains(t, w.Body.String(), "licensed")
}

// TestSitesController_AddSite_NoDomains 域名缺失 → 400
func TestSitesController_AddSite_NoDomains(t *testing.T) {
	cfg := &config.Config{Dirs: config.DirsConfig{StaticDir: t.TempDir()}}
	mockCM := &MockConfigManager{config: cfg}
	controller := NewSitesController(mockCM, nil, &MockSiteHandler{}, nil, nil, nil, nil, cfg)

	router := ginNewRouter()
	router.POST("/sites", controller.AddSite)

	body := map[string]interface{}{"name": "X", "domains": []string{}, "port": freePort(t), "mode": "static"}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sites", jsonBody(t, body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestSitesController_UpdateSite_Branches 端口被占/站点不存在/保存失败/服务器重启
func TestSitesController_UpdateSite_Branches(t *testing.T) {
	staticDir := t.TempDir()
	cfg := &config.Config{
		Sites: []config.SiteConfig{{ID: "ctl-upd", Name: "Old", Domains: []string{"old.example"}, Port: freePort(t)}},
		Dirs:  config.DirsConfig{StaticDir: staticDir},
	}
	mockCM := &MockConfigManager{config: cfg}
	mockSSM := &MockSiteServerMgr{servers: map[string]*http.Server{"ctl-upd": {}}}
	mockRedis := &MockRedisClient{storedStats: map[string]map[string]interface{}{}}
	controller := NewSitesController(mockCM, mockSSM, &MockSiteHandler{}, mockRedis, nil, nil, nil, cfg)

	router := ginNewRouter()
	router.PUT("/sites/:id", controller.UpdateSite)

	// 端口变更为被占用端口 → 400（与 IsPortAvailable 相同方式绑定全部接口）
	busyPort := freePort(t)
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", busyPort))
	require.NoError(t, err)
	defer l.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/sites/ctl-upd",
		jsonBody(t, map[string]interface{}{"name": "New", "domains": []string{"new.example"}, "port": busyPort, "mode": "static"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 站点不存在 → 404
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/sites/ghost",
		jsonBody(t, map[string]interface{}{"name": "New", "domains": []string{"new.example"}, "port": freePort(t), "mode": "static"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// SaveConfig 失败 → 500
	mockCM.saveError = fmt.Errorf("disk full")
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/sites/ctl-upd",
		jsonBody(t, map[string]interface{}{"name": "New", "domains": []string{"new.example"}, "port": freePort(t), "mode": "static"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockCM.saveError = nil

	// 成功：服务器先停后启 + Redis 配置刷新
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/sites/ctl-upd",
		jsonBody(t, map[string]interface{}{"name": "New", "domains": []string{"new.example"}, "port": freePort(t), "mode": "static"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "New", mockCM.config.Sites[0].Name)
	assert.NotEmpty(t, mockRedis.storedStats)
}

// TestSitesController_DeleteSite_Branches Redis 删除失败/静态目录清理/保存失败/站点不存在
func TestSitesController_DeleteSite_Branches(t *testing.T) {
	staticRoot := t.TempDir()
	siteStaticDir := filepath.Join(staticRoot, "ctl-del")
	require.NoError(t, os.MkdirAll(filepath.Join(siteStaticDir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(siteStaticDir, "index.html"), []byte("<html/>"), 0o644))

	cfg := &config.Config{
		Sites: []config.SiteConfig{{ID: "ctl-del", Name: "Del", Domains: []string{"del.example"}}},
		Dirs:  config.DirsConfig{StaticDir: staticRoot},
	}
	mockCM := &MockConfigManager{config: cfg}
	mockSSM := &MockSiteServerMgr{servers: map[string]*http.Server{"ctl-del": {}}}

	router := ginNewRouter()

	// 1) Redis 删除失败（错误被记录，流程继续）→ 200
	mockRedisErr := &MockRedisClient{deleteSiteDataFunc: func(siteID string) error {
		return fmt.Errorf("redis unavailable")
	}}
	c1 := NewSitesController(mockCM, mockSSM, &MockSiteHandler{}, mockRedisErr, nil, nil, nil, cfg)
	router.DELETE("/sites/:id", c1.DeleteSite)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/sites/ctl-del", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0, len(mockCM.config.Sites))

	// 2) 站点不存在 → 404
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/sites/ghost", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)

	// 3) SaveConfig 失败 → 500（重新灌入站点）
	mockCM.config = &config.Config{
		Sites: []config.SiteConfig{{ID: "ctl-del2", Name: "Del2", Domains: []string{"d2.example"}}},
		Dirs:  config.DirsConfig{StaticDir: staticRoot},
	}
	mockCM.saveError = fmt.Errorf("disk full")
	c3 := NewSitesController(mockCM, mockSSM, &MockSiteHandler{}, &MockRedisClient{}, nil, nil, nil, mockCM.config)
	router2 := ginNewRouter()
	router2.DELETE("/sites/:id", c3.DeleteSite)
	w = httptest.NewRecorder()
	router2.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/sites/ctl-del2", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockCM.saveError = nil
}

// TestSitesController_GetSiteConfig_PushFallback push 配置缺失时回退站点静态配置
func TestSitesController_GetSiteConfig_PushFallback(t *testing.T) {
	staticRoot := t.TempDir()
	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{ID: "ctl-fb", Name: "FB", Domains: []string{"fb.example"},
				Prerender: config.PrerenderConfig{Push: config.PushConfig{BaiduToken: "tok123"}}},
		},
		Dirs: config.DirsConfig{StaticDir: staticRoot},
	}
	mockCM := &MockConfigManager{config: cfg}
	mockRedis := &MockRedisClient{getSiteStatsFunc: func(key string) (map[string]string, error) {
		return map[string]string{}, nil // 空配置 → 走回退
	}}
	controller := NewSitesController(mockCM, nil, &MockSiteHandler{}, mockRedis, nil, nil, nil, cfg)

	router := ginNewRouter()
	router.GET("/sites/:id/config", controller.GetSiteConfig)

	// push 类型回退到站点配置中的推送段
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites/ctl-fb/config?type=push", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "tok123")

	// waf 类型回退到扁平 WAF map
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites/ctl-fb/config?type=waf", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "firewall_enabled")

	// 站点不存在 → 404
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites/ghost/config?type=push", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestSitesController_GetSiteConfig_RedisError Redis 读配置失败 → 500
func TestSitesController_GetSiteConfig_RedisError(t *testing.T) {
	cfg := &config.Config{Sites: []config.SiteConfig{{ID: "ctl-err"}}}
	mockRedis := &MockRedisClient{getSiteStatsFunc: func(key string) (map[string]string, error) {
		return nil, fmt.Errorf("redis down")
	}}
	controller := NewSitesController(&MockConfigManager{config: cfg}, nil, &MockSiteHandler{}, mockRedis, nil, nil, nil, cfg)

	router := ginNewRouter()
	router.GET("/sites/:id/config", controller.GetSiteConfig)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites/ctl-err/config?type=push", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestSitesController_GetSiteConfig_InvalidType 非法配置类型 → 400
func TestSitesController_GetSiteConfig_InvalidType(t *testing.T) {
	cfg := &config.Config{Sites: []config.SiteConfig{{ID: "ctl-err"}}}
	controller := NewSitesController(&MockConfigManager{config: cfg}, nil, &MockSiteHandler{}, &MockRedisClient{}, nil, nil, nil, cfg)

	router := ginNewRouter()
	router.GET("/sites/:id/config", controller.GetSiteConfig)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites/ctl-err/config?type=bogus", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestSitesController_GetSiteConfig_PrerenderFromConfig prerender 类型直接读站点配置 struct
func TestSitesController_GetSiteConfig_PrerenderFromConfig(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{{ID: "ctl-pr", Prerender: config.PrerenderConfig{Enabled: true, CacheTTL: 3600}}},
	}
	controller := NewSitesController(&MockConfigManager{config: cfg}, nil, &MockSiteHandler{}, &MockRedisClient{}, nil, nil, nil, cfg)

	router := ginNewRouter()
	router.GET("/sites/:id/config", controller.GetSiteConfig)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites/ctl-pr/config?type=prerender", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"cache_ttl":3600`)

	// prerender 站点不存在 → 404
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites/ghost/config?type=prerender", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
}
