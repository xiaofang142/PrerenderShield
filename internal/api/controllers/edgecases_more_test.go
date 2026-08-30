package controllers

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"prerender-shield/internal/auth"
	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/prerender"
	"prerender-shield/internal/redis"
	sitehandler "prerender-shield/internal/site-handler"
	siteserver "prerender-shield/internal/site-server"
)

// ---------------------------------------------------------------------------
// fakeRespServer 最小 RESP 服务端：SCAN 正常返回，DEL 返回错误（触发 DelMultiple 失败分支）
// ---------------------------------------------------------------------------

type fakeRespServer struct {
	listener net.Listener
	keys     []string
}

func newFakeRespServer(t *testing.T, keys []string) *fakeRespServer {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := &fakeRespServer{listener: l, keys: keys}
	go s.serve()
	t.Cleanup(func() { l.Close() })
	return s
}

func (s *fakeRespServer) addr() string { return s.listener.Addr().String() }

func (s *fakeRespServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakeRespServer) handle(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	for {
		cmd, err := readRESPCommand(r)
		if err != nil {
			return
		}
		if len(cmd) == 0 {
			continue
		}
		switch strings.ToUpper(cmd[0]) {
		case "SCAN":
			// 返回 *2 [ "0", [keys...] ]
			out := fmt.Sprintf("*2\r\n$1\r\n0\r\n*%d\r\n", len(s.keys))
			for _, k := range s.keys {
				out += fmt.Sprintf("$%d\r\n%s\r\n", len(k), k)
			}
			conn.Write([]byte(out))
		case "DEL", "FLUSHDB", "FLUSHALL":
			conn.Write([]byte("-ERR simulated DEL failure\r\n"))
		case "MEMORY":
			conn.Write([]byte("$2\r\n64\r\n"))
		default:
			// PING/AUTH/SELECT/CLIENT/HELLO 等一律 +OK
			conn.Write([]byte("+OK\r\n"))
		}
	}
}

func readRESPCommand(r *bufio.Reader) ([]string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimRight(line, "\r\n")
	if len(line) == 0 {
		return nil, nil
	}
	switch line[0] {
	case '*':
		n, _ := strconv.Atoi(line[1:])
		cmd := make([]string, 0, n)
		for i := 0; i < n; i++ {
			hdr, err := r.ReadString('\n')
			if err != nil {
				return nil, err
			}
			hdr = strings.TrimRight(hdr, "\r\n")
			if len(hdr) == 0 || hdr[0] != '$' {
				return nil, fmt.Errorf("expected bulk header, got %q", hdr)
			}
			sz, _ := strconv.Atoi(hdr[1:])
			buf := make([]byte, sz+2)
			if _, err := ioReadFull(r, buf); err != nil {
				return nil, err
			}
			cmd = append(cmd, string(buf[:sz]))
		}
		return cmd, nil
	default:
		// inline command
		return strings.Fields(line), nil
	}
}

func ioReadFull(r *bufio.Reader, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// TestPreheatController_ClearCache_DelFails Redis DEL 被模拟拒绝 → 500
func TestPreheatController_ClearCache_DelFails(t *testing.T) {
	srv := newFakeRespServer(t, []string{"cache:ctl-del:/a", "cache:ctl-del:/b"})
	client, err := redis.NewClient(srv.addr(), "", 0)
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	cfg := &config.Config{Sites: []config.SiteConfig{{ID: "ctl-del"}}}
	controller := NewPreheatController(nil, client, nil, cfg)
	router := ginNewRouter()
	router.POST("/preheat/clear-cache", controller.ClearCache)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/preheat/clear-cache", jsonBody(t, map[string]string{"siteId": "ctl-del"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "清除缓存失败")
}

// TestPreheatController_GetCacheTotalSize_50Keys 采样满 50 触发内层 break
func TestPreheatController_GetCacheTotalSize_50Keys(t *testing.T) {
	client := newTestRedisDB15(t)
	for i := 0; i < 55; i++ {
		key := fmt.Sprintf("cache:ctl-size50:/%d", i)
		require.NoError(t, client.Set(key, strings.Repeat("x", 32), time.Minute))
		t.Cleanup(func() { client.Del(key) })
	}
	c := NewPreheatController(nil, client, nil, &config.Config{})
	size := c.getCacheTotalSize(100)
	assert.Greater(t, size, int64(0))
}

// TestPreheatController_EdgeFallbacks 站点无域名/站点列表为 nil 的兜底分支
func TestPreheatController_EdgeFallbacks(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{{ID: "ctl-nodomain", Name: "NoDomain"}},
	}
	manager := newFakeManager(t)
	controller := NewPreheatController(manager, nil, nil, cfg)

	// GetPreheatSites：无域名站点回退 localhost
	router := ginNewRouter()
	router.GET("/preheat/sites", controller.GetPreheatSites)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/preheat/sites", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "localhost")

	// GetPreheatStats：无域名站点单查
	router2 := ginNewRouter()
	router2.GET("/preheat/stats", controller.GetPreheatStats)
	w = httptest.NewRecorder()
	router2.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/preheat/stats?siteId=ctl-nodomain", nil))
	require.Equal(t, http.StatusOK, w.Code)

	// GetPreheatUrls：无域名站点回退 siteId 作为域名（注入 URL 后验证拼接结果）
	client := newTestRedisDB15(t)
	require.NoError(t, client.AddURL("ctl-nodomain", "/page"))
	t.Cleanup(func() { client.Del("site:ctl-nodomain:urls") })
	redisCtrl := NewPreheatController(manager, client, nil, cfg)
	router3 := ginNewRouter()
	router3.GET("/preheat/urls", redisCtrl.GetPreheatUrls)
	w = httptest.NewRecorder()
	router3.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/preheat/urls?siteId=ctl-nodomain", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "http://ctl-nodomain/page")

	// nil Sites 列表兜底
	nilCfg := NewPreheatController(manager, nil, nil, &config.Config{})
	router4 := ginNewRouter()
	router4.GET("/preheat/sites", nilCfg.GetPreheatSites)
	router4.GET("/preheat/stats", nilCfg.GetPreheatStats)
	w = httptest.NewRecorder()
	router4.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/preheat/sites", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	w = httptest.NewRecorder()
	router4.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/preheat/stats", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestPreheatController_EngineNotFound_404s manager 为 nil 时 findEngine 返回 false → 404
func TestPreheatController_EngineNotFound_404s(t *testing.T) {
	cfg := &config.Config{Sites: []config.SiteConfig{{ID: "ctl-eng"}}}
	controller := NewPreheatController(nil, nil, nil, cfg)
	router := ginNewRouter()
	router.POST("/preheat/invalidate", controller.InvalidateCache)
	router.POST("/preheat/recache", controller.RecacheURL)

	for _, path := range []string{"/preheat/invalidate", "/preheat/recache"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path,
			jsonBody(t, map[string]string{"siteId": "ctl-eng", "url": "/p"}))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code, path)
		assert.Contains(t, w.Body.String(), "engine not found")
	}
}

// TestPreheatController_RecacheURL_InvalidateFail 重渲前失效失败 → 500
func TestPreheatController_RecacheURL_InvalidateFail(t *testing.T) {
	cfg := &config.Config{Sites: []config.SiteConfig{{ID: "ctl-ri"}}}
	manager := newFakeManagerWithEngine(t, &fakeEngine{failInvalidate: true}, "ctl-ri")
	controller := NewPreheatController(manager, nil, nil, cfg)
	router := ginNewRouter()
	router.POST("/preheat/recache", controller.RecacheURL)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/preheat/recache",
		jsonBody(t, map[string]string{"siteId": "ctl-ri", "url": "/p"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "失效旧缓存失败")
}

// TestPreheatController_TaskStatus_RedisClosed closed client 下 IsPreheatRunning 报错回退 false
func TestPreheatController_TaskStatus_RedisClosed(t *testing.T) {
	closed := closedTestRedisDB15(t)
	controller := NewPreheatController(nil, closed, nil, &config.Config{})
	router := ginNewRouter()
	router.GET("/preheat/task/status", controller.GetPreheatTaskStatus)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/preheat/task/status?siteId=s1", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"isRunning":false`)
}

// TestPreheatController_ClearCache_SiteNotFound 站点不存在 → 404
func TestPreheatController_ClearCache_SiteNotFound(t *testing.T) {
	client := newTestRedisDB15(t)
	controller := NewPreheatController(nil, client, nil, &config.Config{Sites: []config.SiteConfig{{ID: "real"}}})
	router := ginNewRouter()
	router.POST("/preheat/clear-cache", controller.ClearCache)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/preheat/clear-cache", jsonBody(t, map[string]string{"siteId": "ghost"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestPreheatController_ListCacheEntries_LimitClamp limit 越界回退 200
func TestPreheatController_ListCacheEntries_LimitClamp(t *testing.T) {
	cfg := &config.Config{Sites: []config.SiteConfig{{ID: "ctl-cl"}}}
	manager := newFakeManagerWithEngine(t, &fakeEngine{}, "ctl-cl")
	controller := NewPreheatController(manager, nil, nil, cfg)
	router := ginNewRouter()
	router.GET("/preheat/entries", controller.ListCacheEntries)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/preheat/entries?siteId=ctl-cl&limit=99999", nil))
	require.Equal(t, http.StatusOK, w.Code)
}

// ---------------------------------------------------------------------------
// auth 补齐
// ---------------------------------------------------------------------------

// TestAuthController_Enable2FA_Error 2FA 服务 Redis 不可用 → 500
func TestAuthController_Enable2FA_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	twoFactor := auth.NewTwoFactorAuth(nil, "test")
	controller := NewAuthController(nil, nil, nil, twoFactor)
	router := ginNewRouter()
	router.POST("/auth/2fa/enable", controller.Enable2FA)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/2fa/enable", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestAuthController_ConfirmDisable2FA_NotConfigured 2FA 未装配 → 400
func TestAuthController_ConfirmDisable2FA_NotConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewAuthController(nil, nil, nil, nil)
	router := ginNewRouter()
	router.POST("/auth/2fa/confirm", controller.Confirm2FA)
	router.POST("/auth/2fa/disable", controller.Disable2FA)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/2fa/confirm", jsonBody(t, map[string]string{"code": "123456"}))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/auth/2fa/disable", jsonBody(t, map[string]string{"code": "123456"}))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// firewall / ssl / system 补齐
// ---------------------------------------------------------------------------

// TestFirewallController_GetAttackLogs_PaginationClamp 分页参数回退
func TestFirewallController_GetAttackLogs_PaginationClamp(t *testing.T) {
	_, router := setupFakeFirewall(&fakeWafRepo{})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/attacks?page=0&limit=0", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data struct {
			Page  int `json:"page"`
			Limit int `json:"limit"`
		} `json:"data"`
	}
	require.NoError(t, jsonUnmarshalBody(w, &resp))
	assert.Equal(t, 1, resp.Data.Page)
	assert.Equal(t, 20, resp.Data.Limit)
}

// TestSSLController_BindErrorsWithClient 非 nil 客户端下的请求体校验分支
func TestSSLController_BindErrorsWithClient(t *testing.T) {
	controller := NewSSLController(&fakeACMEClient{certs: map[string]map[string]interface{}{}}, nil)
	router := ginNewRouter()
	router.POST("/ssl/certificates", controller.RequestCert)
	router.POST("/ssl/certificates/wildcard", controller.RequestWildcardCert)
	router.GET("/ssl/certificates/expiring", controller.GetExpiringCerts)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ssl/certificates", strBody("bad json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/ssl/certificates/wildcard", strBody("bad json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ssl/certificates/expiring?days=notanumber", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"expiring_in_days":30`)
}

// TestSystemController_UpdateSystemConfig_BindError 非 nil Redis 客户端下非法请求体 → 400
func TestSystemController_UpdateSystemConfig_BindError(t *testing.T) {
	_, router := setupFakeSystem(&fakeSystemRedis{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/system/config", strBody("bad json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestSystemController_RestoreConfig_NilRedis / SaveError
func TestSystemController_RestoreConfig_NilRedis(t *testing.T) {
	controller := NewSystemController(nil)
	router := ginNewRouter()
	router.POST("/system/restore", controller.RestoreConfig)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/system/restore", jsonBody(t, map[string]string{"backup_key": "k"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSystemController_RestoreConfig_SaveError(t *testing.T) {
	payload, _ := jsonMarshal(map[string]interface{}{"config": map[string]interface{}{"a": "b"}})
	_, router := setupFakeSystem(&fakeSystemRedis{
		values:  map[string]string{"bk": string(payload)},
		saveErr: true,
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/system/restore", jsonBody(t, map[string]string{"backup_key": "bk"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestSystemController_ListBackups_EmptyValue 值为空的备份键被跳过
func TestSystemController_ListBackups_EmptyValue(t *testing.T) {
	_, router := setupFakeSystem(&fakeSystemRedis{
		keys:   []string{"system:backup:empty"},
		values: map[string]string{"system:backup:empty": ""},
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/system/backups", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"data":null`)
}

// ---------------------------------------------------------------------------
// push 补齐
// ---------------------------------------------------------------------------

// TestPushController_GetPushStats_SingleSiteSuccess 单站点统计成功
func TestPushController_GetPushStats_SingleSiteSuccess(t *testing.T) {
	_, router, _, _ := setupPushWithFake(t, &fakePushRedis{}, false)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/push/stats?siteId=ctl-push", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"success":5`)
}

// TestPushController_SyncKeyFile_Errors 目录创建失败 / 文件写入失败分支
func TestPushController_SyncKeyFile_Errors(t *testing.T) {
	staticRoot := t.TempDir()
	cfg := &config.Config{
		Sites: []config.SiteConfig{{ID: "ctl-syncerr"}},
		Dirs:  config.DirsConfig{StaticDir: staticRoot},
	}
	controller := NewPushController(nil, nil, cfg)

	// siteRoot 位置被文件占用 → MkdirAll 失败
	blocker := filepath.Join(staticRoot, "ctl-syncerr")
	require.NoError(t, os.WriteFile(blocker, []byte("file"), 0o644))
	controller.syncIndexNowKeyFile("ctl-syncerr", "", "keyX") // mkdir 失败仅记日志

	// key 文件名与现有目录冲突 → WriteFile 失败
	require.NoError(t, os.Remove(blocker))
	require.NoError(t, os.MkdirAll(filepath.Join(staticRoot, "ctl-syncerr", "dirkey.txt"), 0o755))
	controller.syncIndexNowKeyFile("ctl-syncerr", "", "dirkey") // write 失败仅记日志
}

// ---------------------------------------------------------------------------
// site_config 直接调用补齐
// ---------------------------------------------------------------------------

// TestSitesController_SaveConfigToRedis_Branches 四类配置持久化的 nil/错误/成功分支
func TestSitesController_SaveConfigToRedis_Branches(t *testing.T) {
	site := &config.SiteConfig{ID: "ctl-save", Name: "S", Domains: []string{"s.example"}}

	// nil redisClient → 全部直接返回
	nilCtrl := NewSitesController(nil, nil, nil, nil, nil, nil, nil, nil)
	nilCtrl.persistSiteConfigToRedis(site)

	// SetSiteStats 报错 → 仅记日志
	errCtrl := NewSitesController(nil, nil, nil, &MockRedisClient{
		setSiteStatsFunc: func(siteID string, stats map[string]interface{}) error {
			return errors.New("redis write failed")
		},
	}, nil, nil, nil, nil)
	errCtrl.persistSiteConfigToRedis(site)

	// 成功路径（含 Info 日志分支）
	okCtrl := NewSitesController(nil, nil, nil, &MockRedisClient{
		storedStats: map[string]map[string]interface{}{},
	}, nil, nil, nil, nil)
	okCtrl.persistSiteConfigToRedis(site)
	assert.NotEmpty(t, okCtrl.redisClient.(*MockRedisClient).storedStats)
}

// TestSitesController_UpdateSitePrerenderConfig_Branches 站点不存在/保存失败/服务器重启
func TestSitesController_UpdateSitePrerenderConfig_Branches(t *testing.T) {
	cfg := &config.Config{Sites: []config.SiteConfig{{ID: "ctl-up", Name: "UP"}}}
	mockCM := &MockConfigManager{config: cfg}
	mockSSM := &MockSiteServerMgr{servers: map[string]*http.Server{"ctl-up": {}}}
	controller := NewSitesController(mockCM, mockSSM, &MockSiteHandler{}, &MockRedisClient{}, nil, nil, nil, cfg)

	router := ginNewRouter()
	router.PUT("/sites/:id/prerender", controller.UpdateSitePrerenderConfig)

	// 站点不存在 → 404
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/sites/ghost/prerender", jsonBody(t, map[string]interface{}{"enabled": true}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// SaveConfig 失败 → 500
	mockCM.saveError = errors.New("disk full")
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/sites/ctl-up/prerender", jsonBody(t, map[string]interface{}{"enabled": true}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockCM.saveError = nil

	// 成功（服务器存在 → 先停后启）
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/sites/ctl-up/prerender", jsonBody(t, map[string]interface{}{"enabled": true}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

// TestSitesController_UpdateSitePushConfig_Branches 保存失败分支
func TestSitesController_UpdateSitePushConfig_Branches(t *testing.T) {
	cfg := &config.Config{Sites: []config.SiteConfig{{ID: "ctl-pushup"}}}
	mockCM := &MockConfigManager{config: cfg, saveError: errors.New("disk full")}
	controller := NewSitesController(mockCM, nil, &MockSiteHandler{}, &MockRedisClient{}, nil, nil, nil, cfg)

	router := ginNewRouter()
	router.PUT("/sites/:id/push", controller.UpdateSitePushConfig)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/sites/ctl-pushup/push", jsonBody(t, map[string]interface{}{"enabled": true}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestSitesController_UpdateSiteFirewallConfig_Branches 站点不存在/保存失败/成功
func TestSitesController_UpdateSiteFirewallConfig_Branches(t *testing.T) {
	cfg := &config.Config{Sites: []config.SiteConfig{{ID: "ctl-fwup"}}}
	mockCM := &MockConfigManager{config: cfg}
	mockSSM := &MockSiteServerMgr{servers: map[string]*http.Server{"ctl-fwup": {}}}
	controller := NewSitesController(mockCM, mockSSM, &MockSiteHandler{}, &MockRedisClient{}, nil, nil, nil, cfg)

	router := ginNewRouter()
	router.PUT("/sites/:id/firewall", controller.UpdateSiteFirewallConfig)

	// 站点不存在 → 404
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/sites/ghost/firewall", jsonBody(t, map[string]interface{}{"enabled": true}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// SaveConfig 失败 → 500
	mockCM.saveError = errors.New("disk full")
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/sites/ctl-fwup/firewall", jsonBody(t, map[string]interface{}{"enabled": true}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockCM.saveError = nil

	// 成功
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/sites/ctl-fwup/firewall", jsonBody(t, map[string]interface{}{"enabled": true}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

// ---------------------------------------------------------------------------
// sites 补齐
// ---------------------------------------------------------------------------

// TestSitesController_AddSite_MutateError Mutate 持久化失败 → 500
func TestSitesController_AddSite_MutateError(t *testing.T) {
	cfg := &config.Config{Sites: []config.SiteConfig{}, Dirs: config.DirsConfig{StaticDir: t.TempDir()}}
	mockCM := &MockConfigManager{config: cfg, mutateError: errors.New("fs error")}
	controller := NewSitesController(mockCM, &MockSiteServerMgr{}, &MockSiteHandler{}, &MockRedisClient{}, nil, nil, nil, cfg)

	router := ginNewRouter()
	router.POST("/sites", controller.AddSite)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sites",
		jsonBody(t, map[string]interface{}{"name": "X", "domains": []string{"x.example"}, "port": freePort(t), "mode": "static"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestSitesController_NewConcreteDeps_AllWrappers 全部依赖注入（含 crawler/visit log）
func TestSitesController_NewConcreteDeps_AllWrappers(t *testing.T) {
	client := newTestRedisDB15(t)
	config.ResetInstance()
	defer config.ResetInstance()

	cfg := &config.Config{
		Sites: []config.SiteConfig{{ID: "ctl-all", Name: "All", Domains: []string{"127.0.0.1"}, Port: freePort(t)}},
		Dirs:  config.DirsConfig{StaticDir: t.TempDir()},
	}
	cm := config.GetInstance()
	cm.UpdateConfig(cfg)

	controller := NewSitesControllerWithConcreteDeps(
		cm,
		siteserverNewManager(t),
		sitehandlerNewHandler(),
		client,
		monitorNewMonitor(),
		logging.NewCrawlerLogManagerWithClient(client.GetRawClient()),
		logging.NewVisitLogManagerWithClient(client.GetRawClient()),
		cfg,
	)

	// 触发全部 wrapper ctrl 回填（通过 GetSites 走 configManager 路径）
	router := ginNewRouter()
	router.GET("/sites", controller.GetSites)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites", nil))
	require.Equal(t, http.StatusOK, w.Code)
}

// TestSitesController_DeleteSite_RemoveAllFails 静态目录删除失败仅记日志
func TestSitesController_DeleteSite_RemoveAllFails(t *testing.T) {
	staticRoot := t.TempDir()
	siteDir := filepath.Join(staticRoot, "ctl-rmfail")
	inner := filepath.Join(siteDir, "locked")
	require.NoError(t, os.MkdirAll(inner, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(inner, "f.txt"), []byte("x"), 0o644))
	// 去掉内层目录写权限 → RemoveAll 失败
	require.NoError(t, os.Chmod(inner, 0o555))
	t.Cleanup(func() { _ = os.Chmod(inner, 0o755) })

	cfg := &config.Config{
		Sites: []config.SiteConfig{{ID: "ctl-rmfail", Name: "RM", Domains: []string{"r.example"}}},
		Dirs:  config.DirsConfig{StaticDir: staticRoot},
	}
	mockCM := &MockConfigManager{config: cfg}
	controller := NewSitesController(mockCM, &MockSiteServerMgr{}, &MockSiteHandler{}, &MockRedisClient{}, nil, nil, nil, cfg)

	router := ginNewRouter()
	router.DELETE("/sites/:id", controller.DeleteSite)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/sites/ctl-rmfail", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0, len(mockCM.config.Sites))
}

// jsonMarshal 独立于 *testing.T 的 JSON 序列化
func jsonMarshal(v interface{}) ([]byte, error) { return json.Marshal(v) }

// newFakeManager 构建空引擎管理器
func newFakeManager(t *testing.T) *prerender.EngineManager {
	t.Helper()
	return prerender.NewEngineManager(nil, nil, 2)
}

// newFakeManagerWithEngine 注册假引擎的引擎管理器
func newFakeManagerWithEngine(t *testing.T, engine *fakeEngine, siteID string) *prerender.EngineManager {
	t.Helper()
	m := prerender.NewEngineManager(nil, nil, 2)
	m.RegisterEngine(siteID, engine)
	return m
}

// siteserverNewManager 真实站点服务器管理器
func siteserverNewManager(t *testing.T) *siteserver.Manager {
	return siteserver.NewManager(nil, nil)
}

// sitehandlerNewHandler 零值站点处理器
func sitehandlerNewHandler() *sitehandler.Handler { return &sitehandler.Handler{} }

// monitorNewMonitor 默认监控器
func monitorNewMonitor() *monitoring.Monitor {
	return monitoring.NewMonitor(monitoring.Config{Enabled: true})
}

// TestMonitoringController_SaveFirewallRules_NilRedis nil 客户端保存防火墙规则 → 500
func TestMonitoringController_SaveFirewallRules_NilRedis(t *testing.T) {
	_, router := setupMonitoringNilRedis(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/firewall-rules",
		jsonBody(t, map[string]interface{}{"site_id": "s1", "rules": []interface{}{}}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestPreheatController_RecacheURL_InvalidJSON 非法请求体 → 400
func TestPreheatController_RecacheURL_InvalidJSON(t *testing.T) {
	cfg := &config.Config{Sites: []config.SiteConfig{{ID: "s1"}}}
	controller := NewPreheatController(newFakeManager(t), nil, nil, cfg)
	router := ginNewRouter()
	router.POST("/preheat/recache", controller.RecacheURL)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/preheat/recache", strBody("bad json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestPushController_UpdatePushConfig_BindError 非法请求体 → 400
func TestPushController_UpdatePushConfig_BindError(t *testing.T) {
	_, router, _, _ := setupPushWithFake(t, &fakePushRedis{}, false)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/push/config", strBody("bad json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestSitesController_GetSiteConfig_RedisHit Redis 命中定制配置 → 直接返回
func TestSitesController_GetSiteConfig_RedisHit(t *testing.T) {
	cfg := &config.Config{Sites: []config.SiteConfig{{ID: "ctl-hit"}}}
	mockRedis := &MockRedisClient{getSiteStatsFunc: func(key string) (map[string]string, error) {
		return map[string]string{"enabled": "true", "baidu_token": "t"}, nil
	}}
	controller := NewSitesController(&MockConfigManager{config: cfg}, nil, &MockSiteHandler{}, mockRedis, nil, nil, nil, cfg)

	router := ginNewRouter()
	router.GET("/sites/:id/config", controller.GetSiteConfig)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites/ctl-hit/config?type=push", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "baidu_token")
}

// TestSitesController_UpdatePushConfigByID_ServerExists 已有站点服务器时先停后启
func TestSitesController_UpdatePushConfigByID_ServerExists(t *testing.T) {
	cfg := &config.Config{
		Sites:  []config.SiteConfig{{ID: "ctl-pu", Name: "PU", Domains: []string{"p.example"}}},
		Dirs:   config.DirsConfig{StaticDir: t.TempDir()},
		Server: config.ServerConfig{Address: "127.0.0.1"},
	}
	mockCM := &MockConfigManager{config: cfg}
	mockSSM := &MockSiteServerMgr{servers: map[string]*http.Server{"ctl-pu": {}}}
	controller := NewSitesController(mockCM, mockSSM, &MockSiteHandler{}, &MockRedisClient{}, nil, nil, nil, cfg)

	status, msg := controller.updatePushConfigByID("ctl-pu", config.PushConfig{Enabled: true})
	assert.Equal(t, http.StatusOK, status)
	assert.Contains(t, msg, "successfully")
	_, exists := mockSSM.GetSiteServer("ctl-pu")
	assert.True(t, exists)
}

// TestSitesController_IndexExtractedFiles_EarlyReturn nil Redis / 无域名站点直接返回
func TestSitesController_IndexExtractedFiles_EarlyReturn(t *testing.T) {
	// nil redis
	nilCtrl := NewSitesController(nil, nil, nil, nil, nil, nil, nil, nil)
	nilCtrl.indexExtractedFiles(&config.SiteConfig{ID: "s", Domains: []string{"d.example"}}, t.TempDir())

	// 无域名站点
	client := newTestRedisDB15(t)
	domCtrl := NewSitesController(nil, nil, nil, &MockRedisClient{}, nil, nil, nil, nil)
	domCtrl.indexExtractedFiles(&config.SiteConfig{ID: "s2"}, t.TempDir())
	_ = client
}

// TestSEOController_UpdateSEOConfig_SaveError SaveConfig 落盘失败 → 500。
// 全局单例的 configPath 为未导出字段，通过反射注入指向只读文件的路径，
// 复现生产环境中已加载配置文件场景下的落盘失败。
func TestSEOController_UpdateSEOConfig_SaveError(t *testing.T) {
	config.ResetInstance()
	defer config.ResetInstance()

	cm := config.GetInstance()
	badDir := t.TempDir()
	badPath := filepath.Join(badDir, "config.yaml")
	require.NoError(t, os.WriteFile(badPath, []byte("app: {}\n"), 0o444))

	field := reflect.ValueOf(cm).Elem().FieldByName("configPath")
	require.True(t, field.CanAddr())
	ptr := field.Addr().UnsafePointer()
	reflect.NewAt(field.Type(), ptr).Elem().SetString(badPath)

	controller := NewSEOController(&config.Config{})
	router := ginNewRouter()
	router.PUT("/seo/config", controller.UpdateSEOConfig)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/seo/config", strBody(`{"sitemap":{"enabled":true}}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
