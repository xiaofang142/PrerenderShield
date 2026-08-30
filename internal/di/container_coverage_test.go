package di

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"prerender-shield/internal/config"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/repository"
)

// getWorkerCount 全分支：环境变量优先 / 非法回退 / 站点 PoolSize 最大值 / 默认 5
func TestGetWorkerCount_Branches(t *testing.T) {
	// 环境变量合法 → 直接采用
	t.Setenv("PRERENDER_WORKER_COUNT", "7")
	if got := getWorkerCount(&config.Config{}); got != 7 {
		t.Fatalf("env worker count not honored: %d", got)
	}

	// 环境变量非法（非数字）→ 回退站点配置
	t.Setenv("PRERENDER_WORKER_COUNT", "abc")
	cfg := &config.Config{}
	cfg.Sites = []config.SiteConfig{
		{ID: "a", Prerender: config.PrerenderConfig{PoolSize: 3}},
		{ID: "b", Prerender: config.PrerenderConfig{PoolSize: 8}},
	}
	if got := getWorkerCount(cfg); got != 8 {
		t.Fatalf("max pool size not honored: %d", got)
	}

	// 环境变量为空 → 无站点 → 默认 5
	t.Setenv("PRERENDER_WORKER_COUNT", "")
	if got := getWorkerCount(&config.Config{}); got != 5 {
		t.Fatalf("default worker count broken: %d", got)
	}
}

// NewContainer SSL 分支补齐：
// 1) 证书目录为普通文件 → NewManager 失败 → sslMgr=nil 降级
// 2) ACME 目录服务器不可达 → NewACMEClient 失败（sslMgr 存在但续期器不接线）
func TestNewContainer_SSLInitFailureBranches(t *testing.T) {
	client, err := redis.NewClient("localhost:6379", "", 15)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}

	// NewManager 失败：certDir 为已存在普通文件 → MkdirAll 报错
	notADir := filepath.Join(t.TempDir(), "file-not-dir")
	if err := os.WriteFile(notADir, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := testDIConfig()
	cfg.Dirs.CertsDir = notADir
	cfg.SSL = config.SSLConfig{Enabled: true, Email: "ops@example.com"}
	c1, err := NewContainer(ContainerDeps{Config: cfg, Redis: client})
	if err != nil {
		t.Fatalf("NewContainer: %v", err)
	}
	if c1.SSLManager != nil {
		t.Fatal("ssl manager must be nil when cert dir creation fails")
	}
	c1.Close()

	// NewACMEClient 失败：目录服务器连接拒绝（127.0.0.1:1）→ ACME 接线分支 Warn
	t.Setenv("ACME_DIRECTORY_URL", "http://127.0.0.1:1/directory")
	cfg2 := testDIConfig()
	cfg2.SSL = config.SSLConfig{Enabled: true, Email: "ops@example.com"}
	c2, err := NewContainer(ContainerDeps{Config: cfg2, Redis: client})
	if err != nil {
		t.Fatalf("NewContainer(2): %v", err)
	}
	if c2.SSLManager == nil {
		t.Fatal("ssl manager must be wired even when ACME registration fails")
	}
	if c2.SSLAutoRenewer != nil {
		t.Fatal("auto renewer must not be wired when ACME client fails")
	}
	c2.Close()
}

// NewContainer 威胁情报拉取器装配 + 健康检查站点闭包 + Close 关停拉取器：
// 多站点（TI 启用×2：首个 NewFetcher、次个 MergeConfig；TI 禁用×1 跳过），
// HealthChecker.Check() 触发 siteServerChecker 闭包（running<total → 明细拼接）
func TestNewContainer_ThreatIntelAndHealthChecker(t *testing.T) {
	client, err := redis.NewClient("localhost:6379", "", 15)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	cfg := testDIConfig()
	cfg.Sites = []config.SiteConfig{
		{ID: "ti-a", Name: "TIA", Firewall: config.FirewallConfig{
			ThreatIntel: config.ThreatIntelConfig{Enabled: true, GlobalKey: "custom:key", MaxIPs: 10, Concurrency: 2},
		}},
		{ID: "ti-b", Name: "TIB", Firewall: config.FirewallConfig{
			ThreatIntel: config.ThreatIntelConfig{Enabled: true},
		}},
		{ID: "ti-off", Name: "TIOFF"},
	}
	container, err := NewContainer(ContainerDeps{Config: cfg, Redis: client})
	if err != nil {
		t.Fatalf("NewContainer: %v", err)
	}
	if container.ThreatIntelFetcher == nil {
		t.Fatal("threat intel fetcher must be wired when any site enables it")
	}

	// 触发健康检查闭包：3 站点均未启动 → running<total → detail 含全部站点名与分隔符
	results := container.HealthChecker.Check()
	siteCheck, ok := results["site_servers"].(map[string]interface{})
	if !ok {
		t.Fatalf("site_servers check missing: %+v", results)
	}
	if healthy, _ := siteCheck["healthy"].(bool); healthy {
		t.Fatal("site_servers must be unhealthy when no server started")
	}
	msg, _ := siteCheck["message"].(string)
	for _, want := range []string{"TIA (stopped)", "TIB (stopped)", "TIOFF (stopped)"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("detail broken: %q (missing %q)", msg, want)
		}
	}
	if !strings.Contains(msg, ", ") {
		t.Fatalf("detail separator missing: %q", msg)
	}

	// Close 关停 TI 拉取器（324-326 分支）；注意 close(stopChan) 非幂等，仅关一次
	if err := container.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// buildNotificationSource：禁用渠道 → continue 分支
func TestBuildNotificationSource_DisabledChannel(t *testing.T) {
	client, err := redis.NewClient("localhost:6379", "", 15)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	repo := repository.NewNotificationChannelsRepository(client)
	if err := repo.Save([]repository.NotificationChannelData{
		{Type: "webhook", URL: "http://disabled.example", Enabled: false},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	fn := buildNotificationSource(repo, config.NotificationsConfig{})
	wc, ec := fn()
	if wc != nil || ec != nil {
		t.Fatalf("disabled channel must be skipped: %+v %+v", wc, ec)
	}
	client.Del("monitoring:notification-channels")
}
