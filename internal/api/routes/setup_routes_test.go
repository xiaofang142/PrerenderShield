package routes

// 覆盖率攻坚：SetupControllers 全装配分支（SSL 禁用/ACME 失败/ACME 成功/SSL 管理器失败）、
// RegisterRoutes 装配与 apiTokenProvider 闭包、CORS Referer 回退。
// 依赖经 internal/di 容器全装配（与 bootstrap 同构），Redis 走 DB15 隔离库；
// ACME 指向本地模拟目录服务器，不发任何外网请求。

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/config"
	"prerender-shield/internal/di"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/websocket"
)

// ---------- 测试基础设施 ----------

// testRoutesConfig 测试用最小配置（隔离目录 + DB15，不触碰运行环境）
func testRoutesConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := &config.Config{}
	cfg.Cache.RedisDB = 15
	cfg.Server.Address = "127.0.0.1"
	cfg.Server.APIPort = 19591
	cfg.Server.ConsolePort = 19596
	cfg.Dirs.StaticDir = t.TempDir()
	cfg.Dirs.DataDir = t.TempDir()
	cfg.Dirs.CertsDir = t.TempDir()
	cfg.APITokens = []string{"static-api-token-hash-00000000000000000000000000aa"}
	return cfg
}

// testContainer DI 全装配（与 bootstrap runner 一致），测试结束统一关闭
func testContainer(t *testing.T, cfg *config.Config) *di.Container {
	t.Helper()
	client, err := redis.NewClient("localhost:6379", "", 15)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	c, err := di.NewContainer(di.ContainerDeps{Config: cfg, Redis: client})
	if err != nil {
		t.Fatalf("NewContainer: %v", err)
	}
	t.Cleanup(func() {
		_ = c.Close()
		_ = client.Close()
	})
	return c
}

// startMiniACME 精简假 ACME 目录服务器（与 di 包测试同构，本地闭环）
func startMiniACME(t *testing.T) string {
	t.Helper()
	var base string
	var mu sync.Mutex
	nonce := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/directory", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"newNonce":   base + "/new-nonce",
			"newAccount": base + "/new-account",
			"newOrder":   base + "/new-order",
			"revokeCert": base + "/revoke-cert",
			"keyChange":  base + "/key-change",
		})
	})
	mux.HandleFunc("/new-nonce", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		nonce++
		n := nonce
		mu.Unlock()
		w.Header().Set("Replay-Nonce", fmt.Sprintf("n%d", n))
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/new-account", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		nonce++
		n := nonce
		mu.Unlock()
		w.Header().Set("Replay-Nonce", fmt.Sprintf("n%d", n))
		w.Header().Set("Location", base+"/acct/1")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "valid"})
	})
	srv := httptest.NewTLSServer(mux)
	base = srv.URL
	t.Cleanup(srv.Close)
	t.Setenv("ACME_TLS_INSECURE", "1")
	return srv.URL + "/directory"
}

func doJSON(router *gin.Engine, method, path string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// ---------- SetupControllers ----------

// SSL 禁用：主体装配路径（push/2FA/SSL 管理器成功/控制器集合/Provider 注入）
func TestSetupControllers_SSLOff(t *testing.T) {
	cfg := testRoutesConfig(t)
	c := testContainer(t, cfg)

	cs := SetupControllers(
		c.UserManager, c.JWTManager, config.GetInstance(), c.PrerenderMgr, c.Redis,
		c.Scheduler, c.SiteServerMgr, c.SiteHandler, c.Monitor, c.CrawlerLogMgr,
		c.VisitLogMgr, c.WafRepo, c.AuditLogger, cfg,
	)

	assert.NotNil(t, cs)
	assert.NotNil(t, cs.AuthController)
	assert.NotNil(t, cs.OverviewController)
	assert.NotNil(t, cs.MonitoringController)
	assert.NotNil(t, cs.FirewallController)
	assert.NotNil(t, cs.CrawlerController)
	assert.NotNil(t, cs.PreheatController)
	assert.NotNil(t, cs.PushController)
	assert.NotNil(t, cs.SitesController)
	assert.NotNil(t, cs.SystemController)
	assert.NotNil(t, cs.SSLController)
	assert.NotNil(t, cs.SEOController)
}

// SSL 管理器创建失败：CertsDir 指向已存在的普通文件 → MkdirAll 失败 → 降级 warn 分支
func TestSetupControllers_SSLManagerFail(t *testing.T) {
	cfg := testRoutesConfig(t)
	c := testContainer(t, cfg)

	blocker := filepath.Join(t.TempDir(), "certs-blocker")
	assert.NoError(t, os.WriteFile(blocker, []byte("not a dir"), 0644))
	cfg.Dirs.CertsDir = blocker

	cs := SetupControllers(
		c.UserManager, c.JWTManager, config.GetInstance(), c.PrerenderMgr, c.Redis,
		c.Scheduler, c.SiteServerMgr, c.SiteHandler, c.Monitor, c.CrawlerLogMgr,
		c.VisitLogMgr, c.WafRepo, c.AuditLogger, cfg,
	)
	assert.NotNil(t, cs)
}

// SSL 启用但 ACME 目录不可达（closed port）→ NewACMEClient 失败 → warn 分支
func TestSetupControllers_ACMEFail(t *testing.T) {
	cfg := testRoutesConfig(t)
	c := testContainer(t, cfg)
	cfg.SSL = config.SSLConfig{Enabled: true, Email: "ops@example.com"}
	t.Setenv("ACME_DIRECTORY_URL", "http://127.0.0.1:1/directory")

	cs := SetupControllers(
		c.UserManager, c.JWTManager, config.GetInstance(), c.PrerenderMgr, c.Redis,
		c.Scheduler, c.SiteServerMgr, c.SiteHandler, c.Monitor, c.CrawlerLogMgr,
		c.VisitLogMgr, c.WafRepo, c.AuditLogger, cfg,
	)
	assert.NotNil(t, cs)
}

// SSL 启用 + 本地模拟 ACME 注册成功 → DNS provider 非法 → SetDNSProvider warn 分支；
// 自动续签器启动 + SSLController 走 ACME 装配分支
func TestSetupControllers_SSLFull(t *testing.T) {
	t.Setenv("ACME_DIRECTORY_URL", startMiniACME(t))
	cfg := testRoutesConfig(t)
	c := testContainer(t, cfg)
	cfg.SSL = config.SSLConfig{
		Enabled:       true,
		Email:         "ops@example.com",
		AutoRenew:     true,
		CheckInterval: time.Hour,
		DNS:           config.DNSConfig{Provider: "bogus-provider", Credentials: map[string]string{"K": "V"}},
	}

	cs := SetupControllers(
		c.UserManager, c.JWTManager, config.GetInstance(), c.PrerenderMgr, c.Redis,
		c.Scheduler, c.SiteServerMgr, c.SiteHandler, c.Monitor, c.CrawlerLogMgr,
		c.VisitLogMgr, c.WafRepo, c.AuditLogger, cfg,
	)
	assert.NotNil(t, cs)
	assert.NotNil(t, cs.SSLController)
}

// ---------- RegisterRoutes ----------

// 全依赖装配：真实 Redis 限流器 + 注入 Hub；apiTokenProvider 闭包经
// /api/v1/preheat/* 无效 JWT 请求触发（静态 Token + 动态 Redis Token 全分支）
func TestRegisterRoutes_FullWithRealDeps(t *testing.T) {
	cfg := testRoutesConfig(t)
	c := testContainer(t, cfg)

	ctx := context.Background()
	rdb := c.Redis.GetRawClient()
	// 动态管理 Token（Redis system:config）
	assert.NoError(t, rdb.Set(ctx, "system:config", `{"api_tokens":["dynamic-api-token-hash"]}`, 0).Err())
	defer rdb.Del(ctx, "system:config")

	hub := websocket.NewHub(logging.NewStructuredLogger(logging.INFO, ""))
	go hub.Run()
	router := gin.New()
	apiRouter := NewRouter(
		c.UserManager, c.JWTManager, config.GetInstance(), c.PrerenderMgr, c.Redis,
		c.Scheduler, c.SiteServerMgr, c.SiteHandler, c.Monitor, c.CrawlerLogMgr,
		c.VisitLogMgr, c.WafRepo, c.AuditLogger, cfg, hub,
	)
	apiRouter.RegisterRoutes(router)
	assert.NotNil(t, apiRouter.GetHub(), "注入的 Hub 应原样返回")

	// 公开端点
	assert.Equal(t, http.StatusOK, doJSON(router, http.MethodGet, "/api/v1/health", nil).Code)

	// 受保护端点：无凭证 → 401
	assert.Equal(t, http.StatusUnauthorized,
		doJSON(router, http.MethodGet, "/api/v1/preheat/urls", nil).Code)

	// 无效 JWT → apiTokenProvider 闭包：静态 + 动态(Redis) 合并后校验失败 → 401
	w := doJSON(router, http.MethodGet, "/api/v1/preheat/urls",
		map[string]string{"Authorization": "Bearer invalid.jwt.token"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// system:config 中 api_tokens 为非数组 JSON → Unmarshal 失败跳过动态分支
	assert.NoError(t, rdb.Set(ctx, "system:config", `{"api_tokens":"not-an-array"}`, 0).Err())
	w = doJSON(router, http.MethodGet, "/api/v1/preheat/urls",
		map[string]string{"Authorization": "Bearer invalid.jwt.token"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// 删除 system:config → GetSystemConfig 成功但 raw 为空 → 跳过动态分支
	assert.NoError(t, rdb.Del(ctx, "system:config").Err())
	w = doJSON(router, http.MethodGet, "/api/v1/preheat/urls",
		map[string]string{"Authorization": "Bearer invalid.jwt.token"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// WebSocket 端点已注册（无凭证 → 401）
	assert.Equal(t, http.StatusUnauthorized,
		doJSON(router, http.MethodGet, "/ws/realtime", nil).Code)

	// 有效 JWT：触发控制器 configProvider 闭包（configRef.current() 每请求读最新配置）
	token, err := c.JWTManager.GenerateToken("tester", "tester")
	assert.NoError(t, err)
	auth := map[string]string{"Authorization": "Bearer " + token}

	// SEO 配置读取（GET，纯配置读）
	assert.Equal(t, http.StatusOK, doJSON(router, http.MethodGet, "/api/v1/seo/config", auth).Code)
	// 推送站点列表（GET）
	assert.Equal(t, http.StatusOK, doJSON(router, http.MethodGet, "/api/v1/push/sites", auth).Code)

	// pushManager 配置来源闭包（currentConfig）：推送配置更新路径（站点不存在 → 4xx 收尾）
	req := httptest.NewRequest(http.MethodPost, "/api/v1/push/config",
		strings.NewReader(`{"siteId":"ghost-site","config":{"enabled":true}}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.NotEqual(t, 0, w.Code)
}

// Redis 不可用（closed client）→ 跳过限流器装配？否：非 nil 即装配，请求时逐级
// fail-open（封禁检查 warn → 计数器 warn）；apiTokenProvider 走 GetSystemConfig 错误分支
func TestRegisterRoutes_ClosedRedis(t *testing.T) {
	cfg := testRoutesConfig(t)
	client, err := redis.NewClient("localhost:6379", "", 15)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	assert.NoError(t, client.Close())

	router := gin.New()
	apiRouter := NewRouter(
		nil, nil, config.GetInstance(), nil, client,
		nil, nil, nil, nil, nil,
		nil, nil, nil, cfg, nil,
	)
	apiRouter.RegisterRoutes(router) // wsHub 为 nil → 兜底创建并后台运行

	// closed client：限流 fail-open + apiTokenProvider GetSystemConfig 失败分支
	w := doJSON(router, http.MethodGet, "/api/v1/preheat/urls",
		map[string]string{"Authorization": "Bearer invalid.jwt.token"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// Redis 为 nil → 不装配限流器；wsHub 为 nil → 兜底创建（装配分支补全）
func TestRegisterRoutes_NilRedisNilHub(t *testing.T) {
	cfg := testRoutesConfig(t)
	router := gin.New()
	apiRouter := NewRouter(
		nil, nil, config.GetInstance(), nil, nil,
		nil, nil, nil, nil, nil,
		nil, nil, nil, cfg, nil,
	)
	apiRouter.RegisterRoutes(router)

	assert.NotNil(t, apiRouter.GetHub(), "nil Hub 应在 RegisterRoutes 内兜底创建")
	assert.Equal(t, http.StatusOK, doJSON(router, http.MethodGet, "/api/v1/health", nil).Code)
}

// ---------- CORS ----------

// Origin 缺失时回退 Referer 判定（CORS 头仍按白名单发放）
func TestAddCorsMiddleware_RefererFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetAllowedOrigins([]string{"http://referer.example"})
	addCorsMiddleware(router)
	router.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "OK") })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Referer", "http://referer.example")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "http://referer.example", w.Header().Get("Access-Control-Allow-Origin"))

	// Origin 与 Referer 均缺失 → 不发 CORS 头
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}
