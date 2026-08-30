package bootstrap

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"prerender-shield/internal/config"
	"prerender-shield/internal/di"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/ssl"
	"prerender-shield/internal/websocket"
)

// silenceFatal 注入 no-op 退出实现并返回 Fatal 调用计数：端口冲突会真实触发
// logging.Fatal（默认 os.Exit）。调用方必须在测试结束前等待 Fatal 计数到位——
// 残留 goroutine 若在 restore 之后调用 Fatal 会拿到真 os.Exit 终止测试进程
func silenceFatal(t *testing.T) *atomic.Int64 {
	t.Helper()
	calls := &atomic.Int64{}
	t.Cleanup(logging.SetFatalExit(func(int) { calls.Add(1) }))
	return calls
}

// freshPrometheus 提供独立的指标注册器与 HTTP mux：
// Monitor.Start 使用全局 MustRegister/http.Handle，同进程多次 Start 需隔离避免 panic
func freshPrometheus(t *testing.T) {
	t.Helper()
	oldReg := prometheus.DefaultRegisterer
	oldMux := http.DefaultServeMux
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	http.DefaultServeMux = http.NewServeMux()
	t.Cleanup(func() {
		prometheus.DefaultRegisterer = oldReg
		http.DefaultServeMux = oldMux
	})
}

// waitListener 轮询等待端口可连接
func waitListener(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("server not listening on %s within %v", addr, timeout)
}

// waitFor 轮询等待条件成立
func waitFor(t *testing.T, cond func() bool, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}

// newHubForTest 创建并运行独立 WebSocket Hub（供 Shutdown/广播分支测试）
func newHubForTest(t *testing.T) *websocket.Hub {
	t.Helper()
	hub := websocket.NewHub(nil)
	go hub.Run()
	t.Cleanup(hub.Stop)
	return hub
}

// bootstrapTestConfig 构造测试配置（Redis DB15、19xxx 高位端口、127.0.0.1）
func bootstrapTestConfig(t *testing.T, apiPort, consolePort int) *config.Config {
	t.Helper()
	cfg := &config.Config{}
	cfg.Server.Address = "127.0.0.1"
	cfg.Server.APIPort = apiPort
	cfg.Server.ConsolePort = consolePort
	cfg.Cache.RedisURL = "redis://localhost:6379/15"
	cfg.Cache.RedisDB = 15
	cfg.Dirs.StaticDir = t.TempDir()
	cfg.Dirs.DataDir = t.TempDir()
	cfg.Dirs.CertsDir = t.TempDir()
	cfg.Monitoring.MetricsPersistence.Enabled = false
	cfg.Monitoring.Alerting.Enabled = false
	cfg.Monitoring.PrometheusAddress = ""
	cfg.Commercial.MaxSites = 1
	return cfg
}

// newRunnerForTest 直接装配 runner（绕过配置文件加载，注入内存 Config）
func newRunnerForTest(t *testing.T, cfg *config.Config) (*AppRunner, *redis.Client) {
	t.Helper()
	client, err := redis.NewClient("localhost:6379", "", 15)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	app := &Application{
		startTime: time.Now(),
		servers:   make([]*http.Server, 0),
		cleanupFn: make([]func(), 0),
	}
	app.config = cfg
	app.redis = client
	app.AddCleanup(func() { client.Close() })
	runner := NewAppRunner(app)
	if err := runner.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	return runner, client
}

// Start 全装配真实启动：站点 HTTP 服务 + API + 控制台 + 调度器/监控/WS，
// 验证端口可连、API 路由可达、Shutdown 全链路清理
func TestAppRunner_Start_FullLifecycle(t *testing.T) {
	silenceFatal(t)
	freshPrometheus(t)
	cfg := bootstrapTestConfig(t, 19601, 19602)
	site := config.SiteConfig{
		ID: "boot-site", Name: "BootSite", Port: 19603, Mode: "static", Domains: []string{"boot.local"},
		Firewall: config.FirewallConfig{
			CCProtection: config.CCProtectionConfig{
				Enabled: true,
				Rules: []config.CCProtectionRule{{
					Name: "burst", Path: "/api", Method: "GET", Dimensions: []string{"ip"},
					Requests: 100, Window: 10, BanTime: 60, Enabled: true,
				}},
			},
			ThreatIntel: config.ThreatIntelConfig{Enabled: true},
		},
	}
	cfg.Sites = []config.SiteConfig{site}
	// SEO 全开：sitemap + robots（含规则映射与坏 interval 分支在 seo 包内消化）
	cfg.SEO.Sitemap = config.SitemapSEOConfig{Enabled: true, BaseURL: "https://boot.local"}
	cfg.SEO.Robots = config.RobotsSEOConfig{
		Enabled: true, OutputDir: cfg.Dirs.StaticDir,
		Rules: []config.RobotsRuleSEO{{UserAgent: "*", Disallow: []string{"/private"}}},
	}
	// LLM SEO 开启（合法 timeout → ParseDuration 成功分支）
	cfg.SEO.LLM = config.LLMSEOConfig{
		Enabled: true, Provider: "zhipu", APIKey: "test-key", APIURL: "http://127.0.0.1:1/v1",
		Model: "glm-4", MaxTokens: 100, Temperature: 0.3, Timeout: "5s", MaxRetries: 1,
	}
	// SSL 开启 + 假 ACME 目录服务器 → SSLManager/ACME/自动续期器全接线
	t.Setenv("ACME_DIRECTORY_URL", startMiniACMEForBootstrap(t))
	cfg.SSL = config.SSLConfig{Enabled: true, Email: "ops@example.com", HTTPPort: 19671}

	runner, _ := newRunnerForTest(t, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := runner.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// 站点 / API / 控制台端口均监听
	waitListener(t, "127.0.0.1:19603", 5*time.Second)
	waitListener(t, "127.0.0.1:19601", 5*time.Second)
	waitListener(t, "127.0.0.1:19602", 5*time.Second)

	// API 路由可达（健康检查端点）
	resp, err := http.Get("http://127.0.0.1:19601/api/v1/health")
	if err != nil {
		t.Fatalf("api server not serving: %v", err)
	}
	resp.Body.Close()

	// 控制台：无静态目录 → SPA 兜底 404（index.html 不存在 → http.NotFound）
	resp2, err := http.Get("http://127.0.0.1:19602/")
	if err != nil {
		t.Fatalf("console server not serving: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("console no-static-dir must 404, got %d", resp2.StatusCode)
	}

	// 触发 SEO 回调（regenSEOFiles 闭包内容）与检查回调
	if err := runner.regenSEOFiles(); err != nil {
		t.Fatalf("regenSEOFiles: %v", err)
	}
	if err := runner.checkRedisHealth(); err != nil {
		t.Fatalf("checkRedisHealth: %v", err)
	}

	// 优雅关闭全链路（Shutdown：站点→调度器→WS→引擎→容器）
	shutdownCtx, sc := context.WithTimeout(context.Background(), 15*time.Second)
	defer sc()
	if err := runner.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	time.Sleep(300 * time.Millisecond) // 等待 ListenAndServe goroutine 退出记录覆盖
}

// Start 幂等路径 + API/控制台端口冲突 Fatal 分支
func TestAppRunner_StartPortConflicts(t *testing.T) {
	fatalCalls := silenceFatal(t)
	freshPrometheus(t)
	cfg := bootstrapTestConfig(t, 19611, 19612)

	// 预占 API 端口 → startAPIServer 的 Fatal 分支
	lnAPI, err := net.Listen("tcp", "127.0.0.1:19611")
	if err != nil {
		t.Skipf("port 19611 unavailable: %v", err)
	}
	defer lnAPI.Close()
	// 预占控制台端口 → startConsoleServer 的 Fatal 分支
	lnConsole, err := net.Listen("tcp", "127.0.0.1:19612")
	if err != nil {
		t.Skipf("port 19612 unavailable: %v", err)
	}
	defer lnConsole.Close()

	runner, _ := newRunnerForTest(t, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Start 恒 nil；监听失败在 goroutine 内触发 Fatal（已注入 no-op）
	if err := runner.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// 等待两处 Fatal（API+控制台）真实触发完成，避免 restore 后残留 goroutine 拿到 os.Exit
	waitFor(t, func() bool { return fatalCalls.Load() >= 2 }, 5*time.Second)

	// 关停：Monitor/调度器等
	shutdownCtx, sc := context.WithTimeout(context.Background(), 10*time.Second)
	defer sc()
	_ = runner.Shutdown(shutdownCtx)
	time.Sleep(300 * time.Millisecond)
}

// Shutdown 未启动（started=false）→ 直通返回；wsHub 已建但未启动 → Hub.Stop 分支
func TestAppRunner_Shutdown_NotStarted(t *testing.T) {
	cfg := bootstrapTestConfig(t, 19621, 19622)
	runner, _ := newRunnerForTest(t, cfg)
	// 模拟 startAPIServer 已建 hub 但 Start 从未执行
	runner.wsHub = newHubForTest(t)
	if err := runner.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown before Start must be nil: %v", err)
	}
}

// onAlertFired：告警回调 → WS 广播（hub 运行中注册假客户端，验证广播路径不 panic 且送达缓冲）
func TestAppRunner_OnAlertFired(t *testing.T) {
	cfg := bootstrapTestConfig(t, 19623, 19624)
	runner, _ := newRunnerForTest(t, cfg)
	runner.wsHub = newHubForTest(t)
	defer runner.wsHub.Stop()

	alert := &monitoring.AlertStatus{
		Rule:        monitoring.AlertRule{ID: "r1", Name: "cpu", Metric: "cpu", Operator: ">", Threshold: 90, Severity: "high"},
		IsFiring:    true,
		FiredAt:     time.Now(),
		LastChecked: time.Now(),
		Value:       95.5,
	}
	// 无客户端：广播静默丢弃（不 panic）
	runner.onAlertFired(alert, "firing")
	time.Sleep(100 * time.Millisecond)

	// 有客户端注册：广播路径经 hub 事件循环送达客户端发送缓冲
	client := websocket.NewClient(runner.wsHub, nil, "tester")
	runner.wsHub.RegisterClient(client)
	waitFor(t, func() bool { return runner.wsHub.GetClientCount() == 1 }, 2*time.Second)
	runner.onAlertFired(alert, "firing")
	time.Sleep(100 * time.Millisecond)
}

// checkSSLExpiry 全分支：无 SSL 管理器 → nil；有管理器 → CheckExpiration 调用
func TestAppRunner_CheckSSLExpiry(t *testing.T) {
	cfg := bootstrapTestConfig(t, 19625, 19626)
	runner, _ := newRunnerForTest(t, cfg)

	// 无 SSLManager → nil
	if err := runner.checkSSLExpiry(); err != nil {
		t.Fatalf("no ssl manager must be nil: %v", err)
	}

	// 注入 SSL 管理器（证书目录临时目录）→ 走 CheckExpiration 分支
	mgr, err := ssl.NewManager(runner.redisClient, cfg.Dirs.CertsDir, "ops@example.com", false)
	if err != nil {
		t.Fatalf("ssl manager: %v", err)
	}
	runner.container.SSLManager = mgr
	if err := runner.checkSSLExpiry(); err != nil {
		t.Fatalf("ssl expiry check: %v", err)
	}
}

// generateSEOFiles 全分支：sitemap 开/关、robots 开/关、robots 写入成功
func TestAppRunner_GenerateSEOFiles(t *testing.T) {
	cfg := bootstrapTestConfig(t, 19627, 19628)
	staticDir := cfg.Dirs.StaticDir
	siteDir := filepath.Join(staticDir, "seo-site")
	if err := os.MkdirAll(siteDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "index.html"), []byte("<html>seo</html>"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg.Sites = []config.SiteConfig{{ID: "seo-site", Name: "SEO", Mode: "static"}}
	cfg.SEO.Sitemap = config.SitemapSEOConfig{Enabled: true, BaseURL: "https://seo.example"}
	cfg.SEO.Robots = config.RobotsSEOConfig{
		Enabled:    true,
		OutputDir:  staticDir,
		SitemapURL: "https://seo.example/sitemap.xml",
		Rules: []config.RobotsRuleSEO{
			{UserAgent: "Baiduspider", Allow: []string{"/"}, CrawlDelay: 1},
			{UserAgent: "*", Disallow: []string{"/admin"}},
		},
	}

	runner, _ := newRunnerForTest(t, cfg)
	runner.generateSEOFiles()

	if _, err := os.Stat(filepath.Join(staticDir, "robots.txt")); err != nil {
		t.Fatalf("robots.txt not generated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(siteDir, "sitemap.xml")); err != nil {
		t.Fatalf("sitemap.xml not generated: %v", err)
	}

	// 关闭态：直通不产出
	cfg2 := bootstrapTestConfig(t, 19629, 19630)
	runner2, _ := newRunnerForTest(t, cfg2)
	runner2.generateSEOFiles()
}

// generateSitemap 无静态目录 → Warn 分支（results 为空）
func TestAppRunner_GenerateSitemap_Empty(t *testing.T) {
	cfg := bootstrapTestConfig(t, 19631, 19632)
	cfg.Sites = []config.SiteConfig{{ID: "ghost-site", Name: "Ghost", Mode: "proxy"}}
	runner, _ := newRunnerForTest(t, cfg)
	runner.generateSitemap(config.SitemapSEOConfig{Enabled: true, BaseURL: "https://ghost.example"})
}

// Run 一站式：配置文件加载 → Initialize → Start → ctx 取消 → app.Run 返回 → cleanup 执行
func TestRun_FullCycle(t *testing.T) {
	silenceFatal(t)
	freshPrometheus(t)
	tmpDir := t.TempDir()
	staticDir := filepath.Join(tmpDir, "static")
	if err := os.MkdirAll(staticDir, 0755); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := fmt.Sprintf(`
server:
  address: 127.0.0.1
  api_port: 19641
  console_port: 19642
dirs:
  static_dir: %s
  data_dir: %s
  certs_dir: %s
cache:
  redis_url: redis://localhost:6379/15
monitoring:
  metrics_persistence:
    enabled: false
  alerting:
    enabled: false
sites: []
`, staticDir, filepath.Join(tmpDir, "data"), filepath.Join(tmpDir, "certs"))
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, configFile)
	}()

	// 等服务起来
	waitListener(t, "127.0.0.1:19641", 10*time.Second)
	waitListener(t, "127.0.0.1:19642", 10*time.Second)

	// 控制台静态目录兜底路径：static 目录存在但无 index.html → SPA fallback 404
	resp, err := http.Get("http://127.0.0.1:19642/some/route")
	if err != nil {
		t.Fatalf("console not serving: %v", err)
	}
	resp.Body.Close()

	// API 代理：/api/* 转发到 API 服务器（Director 分支）
	resp2, err := http.Get("http://127.0.0.1:19642/api/v1/health")
	if err != nil {
		t.Fatalf("console proxy broken: %v", err)
	}
	resp2.Body.Close()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
	time.Sleep(300 * time.Millisecond)
}

// Run 配置加载失败 → 错误返回
func TestRun_InvalidConfig(t *testing.T) {
	if err := Run(context.Background(), "/non-existent/config.yaml"); err == nil {
		t.Fatal("Run must fail on invalid config")
	}
}

// startConsoleServer 静态文件服务：候选目录命中（cwd/web/dist）→
// 静态扩展名直出 / index.html 直出 / 非静态路径 SPA fallback
func TestAppRunner_ConsoleStaticServing(t *testing.T) {
	silenceFatal(t)
	cfg := bootstrapTestConfig(t, 19651, 19652)

	// startConsoleServer 候选目录含 cwd/web/dist；测试进程 cwd 即包目录 → 预先创建使命中
	wd, _ := os.Getwd()
	localWeb := filepath.Join(wd, "web", "dist")
	if err := os.MkdirAll(localWeb, 0755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(filepath.Join(wd, "web"))
	if err := os.WriteFile(filepath.Join(localWeb, "index.html"), []byte("<html>spa</html>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localWeb, "app.js"), []byte("console.log(1)"), 0644); err != nil {
		t.Fatal(err)
	}

	runner, _ := newRunnerForTest(t, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = runner.startConsoleServer(ctx)
	waitListener(t, "127.0.0.1:19652", 5*time.Second)

	// 静态 js 直出
	resp, err := http.Get("http://127.0.0.1:19652/app.js")
	if err != nil {
		t.Fatalf("static serve broken: %v", err)
	}
	buf := make([]byte, 64)
	n, _ := resp.Body.Read(buf)
	resp.Body.Close()
	if !strings.Contains(string(buf[:n]), "console.log") {
		t.Fatalf("static js content broken: %q", string(buf[:n]))
	}

	// index.html 直出
	resp2, err := http.Get("http://127.0.0.1:19652/index.html")
	if err != nil {
		t.Fatalf("index serve broken: %v", err)
	}
	resp2.Body.Close()

	// 非静态路径 → SPA fallback index.html
	resp3, err := http.Get("http://127.0.0.1:19652/dashboard")
	if err != nil {
		t.Fatalf("spa fallback broken: %v", err)
	}
	resp3.Body.Close()

	_ = runner.Shutdown(context.Background())
	time.Sleep(300 * time.Millisecond)
}

// startAPIServer 的 WS 指标广播协程全分支：
// 第一跳（~10s）无客户端 → continue；注册客户端后第二跳 → BroadcastMonitor；
// ctx 取消 → 协程退出
func TestAppRunner_MetricsTickerBroadcast(t *testing.T) {
	cfg := bootstrapTestConfig(t, 19657, 19658)
	runner, _ := newRunnerForTest(t, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	_ = runner.startAPIServer(ctx)

	// 第一跳：无客户端 → continue 分支
	time.Sleep(11 * time.Second)

	// 注册客户端 → 第二跳：BroadcastMonitor 分支
	client := websocket.NewClient(runner.wsHub, nil, "metrics")
	runner.wsHub.RegisterClient(client)
	waitFor(t, func() bool { return runner.wsHub.GetClientCount() == 1 }, 2*time.Second)
	time.Sleep(11 * time.Second)

	// ctx 取消 → select ctx.Done 分支
	cancel()
	time.Sleep(200 * time.Millisecond)
	_ = runner.Shutdown(context.Background())
	time.Sleep(200 * time.Millisecond)
}

// 控制台代理 /ws/ 前缀 → 转发分支（对端非 WS 端点，验证代理转发执行）
func TestAppRunner_ConsoleWSProxy(t *testing.T) {
	silenceFatal(t)
	cfg := bootstrapTestConfig(t, 19655, 19656)
	runner, _ := newRunnerForTest(t, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = runner.startAPIServer(ctx)
	_ = runner.startConsoleServer(ctx)
	waitListener(t, "127.0.0.1:19655", 5*time.Second)
	waitListener(t, "127.0.0.1:19656", 5*time.Second)

	// /ws/xxx 代理到 API 服务器（非升级请求 → API 返回非 101 即可证明转发发生）
	resp, err := http.Get("http://127.0.0.1:19656/ws/notifications")
	if err != nil {
		t.Fatalf("ws proxy broken: %v", err)
	}
	resp.Body.Close()

	_ = runner.Shutdown(context.Background())
	time.Sleep(300 * time.Millisecond)
}

// startMiniACMEForBootstrap 精简假 ACME 目录服务器（SSL 接线用，不发外网请求）
func startMiniACMEForBootstrap(t *testing.T) string {
	t.Helper()
	var base string
	mux := http.NewServeMux()
	mux.HandleFunc("/directory", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"newNonce":%q,"newAccount":%q,"newOrder":%q,"revokeCert":%q,"keyChange":%q}`,
			base+"/new-nonce", base+"/new-account", base+"/new-order", base+"/revoke-cert", base+"/key-change")
	})
	mux.HandleFunc("/new-nonce", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Replay-Nonce", "bs-nonce")
		w.WriteHeader(200)
	})
	mux.HandleFunc("/new-account", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Replay-Nonce", "bs-nonce")
		w.Header().Set("Location", base+"/acct/1")
		w.WriteHeader(201)
		fmt.Fprint(w, `{"status":"valid"}`)
	})
	srv := httptest.NewTLSServer(mux)
	base = srv.URL
	t.Cleanup(srv.Close)
	t.Setenv("ACME_TLS_INSECURE", "1")
	return srv.URL + "/directory"
}

// Shutdown 分支补齐：注册表含 nil 服务器（continue）与带活动连接的真实服务器
// （canceled ctx → Shutdown 排水超时立即返回 ctx.Err → 错误日志分支）
func TestAppRunner_Shutdown_Branches(t *testing.T) {
	cfg := bootstrapTestConfig(t, 19681, 19682)
	runner, _ := newRunnerForTest(t, cfg)

	// 模拟已启动 + 站点服务器注册表：一个带活动连接的真实服务器 + 一个 nil 条目
	runner.started = true
	release := make(chan struct{})
	site := config.SiteConfig{ID: "shutdown-site", Name: "SS", Port: 19683}
	runner.container.SiteServerMgr.StartSiteServer(site, "127.0.0.1", cfg.Dirs.StaticDir, nil,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			<-release
		}))
	waitListener(t, "127.0.0.1:19683", 5*time.Second)
	runner.container.SiteServerMgr.ListSiteServers()["ghost"] = nil

	// 活动连接（挂起的 handler）：无连接的服务器 Shutdown 恒返回 nil，触发不了错误分支
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Get("http://127.0.0.1:19683/")
	if err != nil {
		t.Skipf("cannot establish active connection: %v", err)
	}
	defer resp.Body.Close()

	// canceled ctx → 真实服务器 Shutdown 返回错误（错误日志分支），nil 条目走 continue
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runner.Shutdown(canceled); err != nil {
		t.Fatalf("Shutdown must swallow per-server errors: %v", err)
	}
	// 强制回收挂起的连接与服务器
	close(release)
	if srv, ok := runner.container.SiteServerMgr.GetSiteServer("shutdown-site"); ok {
		_ = srv.Close()
	}
	// 二次关闭：正常 ctx（此时监听已关，幂等清理）
	if err := runner.Shutdown(context.Background()); err != nil {
		t.Fatalf("second shutdown: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
}

// Application.Shutdown：服务器排水错误分支（活动连接 + canceled ctx → 立即返回 ctx.Err）
// + cleanup 仍然执行。注意：无连接的服务器 Shutdown 恒返回 nil（closeIdleConns 直接成功），
// 必须有进行中的请求才能触发排水等待路径
func TestApplication_Shutdown_ServerErrorBranch(t *testing.T) {
	release := make(chan struct{})
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			<-release
		}),
	}
	ln, err := net.Listen("tcp", "127.0.0.1:19695")
	if err != nil {
		t.Skipf("port 19695 unavailable: %v", err)
	}
	defer ln.Close()
	go func() { _ = srv.Serve(ln) }()
	waitListener(t, "127.0.0.1:19695", 3*time.Second)

	a := &Application{
		startTime: time.Now(),
		servers:   make([]*http.Server, 0),
		cleanupFn: make([]func(), 0),
	}
	a.AddServer(srv)
	called := false
	a.AddCleanup(func() { called = true })

	// 建立活动连接（挂起的 handler）
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Get("http://127.0.0.1:19695/")
	if err != nil {
		t.Skipf("cannot establish active connection: %v", err)
	}
	defer resp.Body.Close()

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.Shutdown(canceled); err != nil {
		t.Fatalf("Shutdown must swallow server drain error: %v", err)
	}
	if !called {
		t.Fatal("cleanup must run even when server drain fails")
	}
	close(release)
	_ = srv.Close()
}

// Initialize：Chromium 不可用 → 解析失败告警分支
func TestAppRunner_Initialize_ChromiumUnavailable(t *testing.T) {
	cfg := bootstrapTestConfig(t, 19685, 19686)
	t.Setenv("PRERENDER_CHROMIUM_PATH", "/nonexistent/chromium-binary")
	runner, _ := newRunnerForTest(t, cfg)
	if runner.container == nil {
		t.Fatal("container must be built even when chromium unavailable")
	}
}

// Initialize：孤儿清扫命中（>1h 的 chromedp-runner 临时目录被回收）→ Info 分支
func TestAppRunner_Initialize_OrphanSweepHit(t *testing.T) {
	cfg := bootstrapTestConfig(t, 19687, 19688)
	stale := filepath.Join(os.TempDir(), "chromedp-runner-coverage-stale")
	if err := os.MkdirAll(stale, 0755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(stale)

	runner, _ := newRunnerForTest(t, cfg)
	if runner.container == nil {
		t.Fatal("container must be built")
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("stale chromedp dir must be swept")
	}
}

// startSite：LLM 启用但 timeout 非法 → ParseDuration 失败跳过（不中断启动）
func TestAppRunner_StartSite_LLMTimeoutInvalid(t *testing.T) {
	cfg := bootstrapTestConfig(t, 19689, 19690)
	cfg.SEO.LLM = config.LLMSEOConfig{
		Enabled: true, Provider: "zhipu", APIKey: "k", APIURL: "http://127.0.0.1:1/v1",
		Model: "glm-4", Timeout: "not-a-duration",
	}
	cfg.Sites = []config.SiteConfig{{ID: "llm-site", Name: "LLM", Port: 19691, Mode: "static", Domains: []string{"llm.local"}}}

	runner, _ := newRunnerForTest(t, cfg)
	if err := runner.startSite(cfg.Sites[0]); err != nil {
		t.Fatalf("startSite: %v", err)
	}
	waitListener(t, "127.0.0.1:19691", 5*time.Second)
	_ = runner.Shutdown(context.Background())
	time.Sleep(200 * time.Millisecond)
}

// fakeSSLForRunner 模拟 SSL 管理器（checkSSLExpiry 错误/循环分支）
type fakeSSLForRunner struct {
	err      error
	expiring []string
}

func (f *fakeSSLForRunner) RequestCertificate(string) error                { return nil }
func (f *fakeSSLForRunner) RenewCertificate(string) error                  { return nil }
func (f *fakeSSLForRunner) ImportCertificate(string, string, string) error { return nil }
func (f *fakeSSLForRunner) GetCertificate(string) (*tls.Certificate, error) {
	return nil, errors.New("none")
}
func (f *fakeSSLForRunner) GetCertificateStatus(string) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeSSLForRunner) ListCertificates() (map[string]map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeSSLForRunner) DeleteCertificate(string) error     { return nil }
func (f *fakeSSLForRunner) SetACMEClient(*ssl.ACMEClient)      {}
func (f *fakeSSLForRunner) CheckExpiration() ([]string, error) { return f.expiring, f.err }

// checkSSLExpiry：CheckExpiration 错误返回 / 过期域名循环
func TestAppRunner_CheckSSLExpiry_ErrorAndLoop(t *testing.T) {
	// 错误分支
	r1 := &AppRunner{container: &di.Container{SSLManager: &fakeSSLForRunner{err: errors.New("redis down")}}}
	if err := r1.checkSSLExpiry(); err == nil {
		t.Fatal("checkSSLExpiry must propagate CheckExpiration error")
	}
	// 过期域名循环分支
	r2 := &AppRunner{container: &di.Container{SSLManager: &fakeSSLForRunner{expiring: []string{"a.example", "b.example"}}}}
	if err := r2.checkSSLExpiry(); err != nil {
		t.Fatalf("checkSSLExpiry with expiring domains: %v", err)
	}
}

// checkRedisHealth：Redis 不可用（客户端已关闭）→ 错误包装分支
func TestAppRunner_CheckRedisHealth_Error(t *testing.T) {
	client, err := redis.NewClient("localhost:6379", "", 15)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	client.Close()
	r := &AppRunner{redisClient: client}
	if err := r.checkRedisHealth(); err == nil {
		t.Fatal("closed redis must fail health check")
	}
}

// generateRobotsTxt：OutputDir 为普通文件 → 写入失败 Warn 分支
func TestAppRunner_GenerateRobotsTxt_WriteError(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "file-not-dir")
	if err := os.WriteFile(notADir, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	r := &AppRunner{}
	r.generateRobotsTxt(config.RobotsSEOConfig{
		Enabled:   true,
		OutputDir: notADir,
		Rules:     []config.RobotsRuleSEO{{UserAgent: "*", Disallow: []string{"/"}}},
	})
}
