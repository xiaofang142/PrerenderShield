package middleware

// 覆盖率攻坚：补齐 error.go / ratelimit.go / waf.go / waf_log_writer.go 的
// 全部未覆盖语句分支。攻击载荷统一用 url.Values.Encode() 构造，
// 避免 httptest.NewRequest 对原始空格/引号的解析 panic。

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/config"
	"prerender-shield/internal/firewall"
	"prerender-shield/internal/models"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/repository"
)

// ---------- 测试基础设施 ----------

// mustTestRedis 返回 DB15 测试客户端（Redis 不可用则跳过）
func mustTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	client, err := redis.NewClient("localhost:6379", "", 15)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	return client
}

// wafTestIP 生成唯一测试 IP，避免历史封禁键/计数桶残留（TTL 内会立即 429）
func wafTestIP(t *testing.T) string {
	t.Helper()
	n := time.Now().UnixNano()
	return fmt.Sprintf("198.18.%d.%d", (n/1000)%251, n%251)
}

// fakeWafRedis 实现 repository.WafRedisClient，可控成功/失败并统计 LPush 次数
type fakeWafRedis struct {
	mu       sync.Mutex
	count    int
	attempts int
	fail     bool
}

func (f *fakeWafRedis) pushed() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.count
}

func (f *fakeWafRedis) tried() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.attempts
}

func (f *fakeWafRedis) Context() context.Context { return context.Background() }

func (f *fakeWafRedis) Get(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (f *fakeWafRedis) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return nil
}

func (f *fakeWafRedis) LPush(ctx context.Context, key string, value interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attempts++
	if f.fail {
		return errors.New("redis down")
	}
	f.count++
	return nil
}

func (f *fakeWafRedis) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return nil, nil
}

func (f *fakeWafRedis) LLen(ctx context.Context, key string) (int64, error) { return 0, nil }

func (f *fakeWafRedis) LTrim(ctx context.Context, key string, start, stop int64) error { return nil }

func (f *fakeWafRedis) HIncrBy(ctx context.Context, key, field string, incr int64) error {
	return nil
}

func (f *fakeWafRedis) Incr(ctx context.Context, key string) error { return nil }

func (f *fakeWafRedis) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return nil, nil
}

func (f *fakeWafRedis) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return nil
}

// waitUntil 轮询等待异步批量落盘完成
func waitUntil(t *testing.T, cond func() bool, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("condition not met within %v", timeout)
}

func newWafTestRouter(site config.SiteConfig, wafRepo *repository.WafRepository, redisClient *redis.Client, engine *firewall.Engine, logWriter *WafLogWriter) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(WafMiddleware(site, wafRepo, redisClient, nil, engine, logWriter))
	router.GET("/test", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	return router
}

// ---------- error.go ----------

func TestNewAPIError_WithAndWithoutDetails(t *testing.T) {
	with := NewAPIError(400, "bad", "detail-info")
	assert.Equal(t, 400, with.Code)
	assert.Equal(t, "bad", with.Message)
	assert.Equal(t, "detail-info", with.Details)

	without := NewAPIError(500, "oops")
	assert.Equal(t, 500, without.Code)
	assert.Nil(t, without.Details)
}

func TestGlobalErrorHandler_RequestError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(GlobalErrorHandler())
	// 未写响应状态（200）时追加错误 → 状态改写为 500
	router.GET("/e500", func(c *gin.Context) {
		_ = c.Error(errors.New("boom"))
	})
	// 已写非 200 状态 → 沿用该状态
	router.GET("/e404", func(c *gin.Context) {
		c.Status(http.StatusNotFound)
		_ = c.Error(errors.New("missing"))
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/e500", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "boom")

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/e404", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "missing")
}

// ---------- isPathTraversal / atomicIncrWithExpire ----------

func TestIsPathTraversal_Cases(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"safe/page.html", false},
		{"a/../b", true},
		{"%2e%2e%2fsecret", true},
		{"%2e%2e/etc", true},
		{"..%2f..%2fetc", true},
		{"..%5cwin.ini", true},
		{"%252e%252e/decode-twice", true},
		{"../../deep", true},
		{"/etc/passwd", true},
		{"/etc/shadow", true},
		{"/etc/hosts", true},
		{"/proc/self/environ", true},
		{"/proc/version", true},
		{"WEB.CONFIG", true},
		{"app/.env", true},
		{"x/.git/config", true},
		{"/win.ini", true},
		{"/winnt/system32/config", true},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, isPathTraversal(tc.input), "input=%q", tc.input)
	}
}

func TestAtomicIncrWithExpire(t *testing.T) {
	client := mustTestRedis(t)
	defer client.Close()

	key := fmt.Sprintf("ratelimit:atomic-test:%d", time.Now().UnixNano())
	ctx := client.Context()
	rdb := client.GetRawClient()
	defer rdb.Del(ctx, key)

	n1, err := atomicIncrWithExpire(ctx, rdb, key, 60*time.Second)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), n1)

	n2, err := atomicIncrWithExpire(ctx, rdb, key, 60*time.Second)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), n2)
}

func TestAtomicIncrWithExpire_RedisError(t *testing.T) {
	client := mustTestRedis(t)
	if err := client.Close(); err != nil {
		t.Skipf("close failed: %v", err)
	}
	_, err := atomicIncrWithExpire(context.Background(), client.GetRawClient(),
		"ratelimit:atomic-closed", 60*time.Second)
	assert.Error(t, err)
}

// ---------- WafMiddleware：限流分支 ----------

func TestWafMiddleware_RateLimit_PassThenBlock(t *testing.T) {
	client := mustTestRedis(t)
	defer client.Close()

	siteID := fmt.Sprintf("site-rl-%d", time.Now().UnixNano())
	ip := wafTestIP(t)
	site := config.SiteConfig{
		ID: siteID,
		Firewall: config.FirewallConfig{
			Enabled: true,
			RateLimitConfig: config.RateLimitConfig{
				Enabled:  true,
				Requests: 1,
				Window:   60,
			},
			ActionConfig: config.ActionConfig{BlockMessage: "slow down"},
		},
	}
	key := fmt.Sprintf("ratelimit:%s:%s", siteID, ip)
	defer client.GetRawClient().Del(context.Background(), key)

	router := newWafTestRouter(site, nil, client, nil, nil)

	// 第 1 次：count=1 未超限 → 放行
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = ip + ":1234"
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 第 2 次：count=2 > 1 → 403 Rate limit exceeded
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = ip + ":1234"
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Rate limit exceeded")
}

func TestWafMiddleware_RateLimit_RedisError(t *testing.T) {
	client := mustTestRedis(t)
	if err := client.Close(); err != nil {
		t.Skipf("close failed: %v", err)
	}
	site := config.SiteConfig{
		ID: "site-rl-closed",
		Firewall: config.FirewallConfig{
			Enabled: true,
			RateLimitConfig: config.RateLimitConfig{
				Enabled:  true,
				Requests: 100,
				Window:   60,
			},
		},
	}
	// closed client → Lua 脚本执行失败 → 不阻断（fail-open）
	router := newWafTestRouter(site, nil, client, nil, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = wafTestIP(t) + ":1234"
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------- WafMiddleware：路径遍历 / 日志落盘分支 ----------

// 路径遍历命中（UserAgent 携带载荷）+ logWriter 日志分支
func TestWafMiddleware_PathTraversal_LogWriter(t *testing.T) {
	fw := &fakeWafRedis{}
	logWriter := NewWafLogWriter(repository.NewWafRepository(fw), 1, time.Hour)
	defer logWriter.Stop()

	site := config.SiteConfig{
		ID:       "site-traversal",
		Firewall: config.FirewallConfig{Enabled: true, ActionConfig: config.ActionConfig{BlockMessage: "denied"}},
	}
	router := newWafTestRouter(site, nil, nil, nil, logWriter)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("User-Agent", "wget/1.0 ../../etc/passwd")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Path traversal detected")
	waitUntil(t, func() bool { return fw.pushed() >= 1 }, time.Second)
}

// logWriter 为 nil 但 wafRepo 非 nil → 直接落库分支
func TestWafMiddleware_BlockViaWafRepo(t *testing.T) {
	fw := &fakeWafRedis{}
	repo := repository.NewWafRepository(fw)

	site := config.SiteConfig{
		ID: "site-repo",
		Firewall: config.FirewallConfig{
			Enabled:      true,
			Blacklist:    []string{"203.0.113.9"},
			ActionConfig: config.ActionConfig{BlockMessage: "denied"},
		},
	}
	router := newWafTestRouter(site, repo, nil, nil, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	waitUntil(t, func() bool { return fw.pushed() >= 1 }, time.Second)
}

// ---------- WafMiddleware：OWASP 引擎分支 ----------

func TestWafMiddleware_OWASP_HighSeverityBlocks(t *testing.T) {
	engine, err := firewall.NewEngine("waf-owasp-high", firewall.Config{})
	assert.NoError(t, err)

	site := config.SiteConfig{
		ID:       "site-owasp",
		Firewall: config.FirewallConfig{Enabled: true, ActionConfig: config.ActionConfig{BlockMessage: "denied"}},
	}
	router := newWafTestRouter(site, nil, nil, engine, nil)

	// SQL 注入载荷（'）→ injection-001 高危威胁 → 阻断
	q := url.Values{"q": []string{"1' OR '1'='1"}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test?"+q.Encode(), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "OWASP threat")
}

func TestWafMiddleware_OWASP_LowSeverityPasses(t *testing.T) {
	engine, err := firewall.NewEngine("waf-owasp-low", firewall.Config{})
	assert.NoError(t, err)

	site := config.SiteConfig{
		ID:       "site-owasp-low",
		Firewall: config.FirewallConfig{Enabled: true},
	}
	router := newWafTestRouter(site, nil, nil, engine, nil)

	// 缺失 User-Agent 仅产生 low 威胁 → 不阻断
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------- RedisRateLimiter ----------

func TestRedisRateLimiter_BannedIP(t *testing.T) {
	client := mustTestRedis(t)
	defer client.Close()

	ip := wafTestIP(t)
	ctx := context.Background()
	banKey := "ratelimit:ban:" + ip
	client.GetRawClient().Set(ctx, banKey, "1", 30*time.Second)
	defer client.GetRawClient().Del(ctx, banKey)

	rl := NewRedisRateLimiter(client, 5, 60*time.Second, 30*time.Second)
	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = ip + ":1234"
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), "please try again later")
}

func TestRedisRateLimiter_ClosedRedis(t *testing.T) {
	client := mustTestRedis(t)
	if err := client.Close(); err != nil {
		t.Skipf("close failed: %v", err)
	}
	// closed client：封禁检查报错（warn）→ 计数器报错（warn）→ fail-open 放行
	rl := NewRedisRateLimiter(client, 5, 60*time.Second, 30*time.Second)
	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = wafTestIP(t) + ":1234"
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ManagementRateLimit：/api/v1 前缀走限流器，豁免端点直接放行
func TestManagementRateLimit_ApiPathRouting(t *testing.T) {
	client := mustTestRedis(t)
	defer client.Close()

	ip := wafTestIP(t)
	ctx := context.Background()
	defer client.GetRawClient().Del(ctx, "ratelimit:ban:"+ip)
	defer client.GetRawClient().Del(ctx,
		"ratelimit:count:"+ip+":"+time.Now().Truncate(time.Minute).Format("200601021504"))

	rl := NewRedisRateLimiter(client, 2, time.Minute, 30*time.Second)
	router := gin.New()
	router.Use(ManagementRateLimit(rl))
	router.GET("/api/v1/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/api/v1/system/config", func(c *gin.Context) { c.Status(http.StatusOK) })

	do := func(path string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = ip + ":1234"
		router.ServeHTTP(w, req)
		return w.Code
	}

	// 豁免端点：不受限流影响
	for i := 0; i < 3; i++ {
		assert.Equal(t, http.StatusOK, do("/api/v1/health"))
	}
	// 管理端点：前 2 次通过（limit=2），第 3 次超限封禁
	assert.Equal(t, http.StatusOK, do("/api/v1/system/config"))
	assert.Equal(t, http.StatusOK, do("/api/v1/system/config"))
	assert.Equal(t, http.StatusTooManyRequests, do("/api/v1/system/config"))
}

// ---------- WafLogWriter ----------

func TestWafLogWriter_Defaults(t *testing.T) {
	// batchSize/flushInterval 非法值 → 默认值分支
	lw := NewWafLogWriter(nil, 0, 0)
	lw.Stop()
}

func TestWafLogWriter_ChannelFullDrops(t *testing.T) {
	// Stop 后无消费者：写满 500 容量 → 第 501 条走丢弃分支
	lw := NewWafLogWriter(nil, 1000, time.Hour)
	lw.Stop()
	for i := 0; i < 500; i++ {
		lw.Write(models.AccessLog{ID: strconv.Itoa(i)})
	}
	lw.Write(models.AccessLog{ID: "overflow"})
}

func TestWafLogWriter_BatchSizeFlush(t *testing.T) {
	fw := &fakeWafRedis{}
	lw := NewWafLogWriter(repository.NewWafRepository(fw), 1, time.Hour)

	lw.Write(models.AccessLog{ID: "a"})
	lw.Write(models.AccessLog{ID: "b"})
	waitUntil(t, func() bool { return fw.pushed() >= 2 }, time.Second)
	lw.Stop()
	assert.GreaterOrEqual(t, fw.pushed(), 2)
}

func TestWafLogWriter_TickerFlush(t *testing.T) {
	fw := &fakeWafRedis{}
	lw := NewWafLogWriter(repository.NewWafRepository(fw), 100, 20*time.Millisecond)

	lw.Write(models.AccessLog{ID: "ticker"})
	waitUntil(t, func() bool { return fw.pushed() >= 1 }, time.Second)
	lw.Stop()
	assert.GreaterOrEqual(t, fw.pushed(), 1)
}

func TestWafLogWriter_StopDrainsPending(t *testing.T) {
	fw := &fakeWafRedis{}
	lw := NewWafLogWriter(repository.NewWafRepository(fw), 100, time.Hour)

	lw.Write(models.AccessLog{ID: "p1"})
	lw.Write(models.AccessLog{ID: "p2"})
	// Stop 内部排空通道并刷写剩余日志（同步保证）
	lw.Stop()
	assert.Equal(t, 2, fw.pushed())
}

func TestWafLogWriter_FlushError(t *testing.T) {
	fw := &fakeWafRedis{fail: true}
	lw := NewWafLogWriter(repository.NewWafRepository(fw), 1, time.Hour)

	lw.Write(models.AccessLog{ID: "will-fail"})
	waitUntil(t, func() bool { return fw.tried() >= 1 }, time.Second)
	lw.Stop()
	assert.Equal(t, 1, fw.tried())
}

// ---------- 防回归：黑名单 + logWriter 组合下 block 响应字段 ----------

func TestWafMiddleware_BlockResponseFields(t *testing.T) {
	site := config.SiteConfig{
		ID: "site-fields",
		Firewall: config.FirewallConfig{
			Enabled:      true,
			Blacklist:    []string{"203.0.113.77"},
			ActionConfig: config.ActionConfig{BlockMessage: "denied"},
		},
	}
	router := newWafTestRouter(site, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "203.0.113.77:1234"
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "denied")
	assert.Contains(t, body, "reason")
	assert.Contains(t, body, "request_id")
}
