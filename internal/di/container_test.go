package di

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"prerender-shield/internal/config"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/repository"
	"prerender-shield/internal/threatintel"
)

// 测试用最小配置（指向隔离 Redis DB15，不触碰运行环境）
func testDIConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Cache.RedisDB = 15
	cfg.Server.Address = "127.0.0.1"
	cfg.Server.APIPort = 19598
	cfg.Server.ConsolePort = 19597
	cfg.Dirs.StaticDir = tTempDir()
	cfg.Dirs.DataDir = tTempDir()
	cfg.Dirs.CertsDir = tTempDir()
	cfg.Monitoring.MetricsPersistence.Enabled = false
	cfg.Monitoring.Alerting.Enabled = false
	cfg.Monitoring.PrometheusAddress = ""
	cfg.Commercial.MaxSites = 1
	return cfg
}

func tTempDir() string {
	d, _ := os.MkdirTemp("", "di-test")
	return d
}

// NewContainer 全装配 + Close 全关闭（覆盖构造与生命周期主体）
func TestNewContainer_FullAssembly(t *testing.T) {
	cfg := testDIConfig()
	// 告警规则文件加载：损坏文件（错误+IsNotExist=false → 记日志分支）
	badRules := filepath.Join(t.TempDir(), "bad-rules.json")
	os.WriteFile(badRules, []byte("corrupt"), 0644)
	cfg.Monitoring.Alerting.RulesPath = badRules
	cfg.Monitoring.Alerting.Enabled = true
	client, err := redis.NewClient("localhost:6379", "", 15)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	container, err := NewContainer(ContainerDeps{Config: cfg, Redis: client})
	if err != nil {
		t.Fatalf("NewContainer: %v", err)
	}
	if container.Monitor == nil || container.Scheduler == nil || container.CacheMgr == nil {
		t.Fatal("core components missing")
	}
	if container.PrerenderMgr == nil {
		t.Fatal("prerender missing")
	}
	if err := container.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// 双重 Close 幂等
	_ = container.Close()

	// 成功加载有效规则文件
	goodRules := filepath.Join(t.TempDir(), "good-rules.json")
	os.WriteFile(goodRules, []byte(`[]`), 0644)
	cfg2 := testDIConfig()
	cfg2.Monitoring.Alerting.RulesPath = goodRules
	cfg2.Monitoring.Alerting.Enabled = true
	client2, err := redis.NewClient("localhost:6379", "", 15)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	container2, err := NewContainer(ContainerDeps{Config: cfg2, Redis: client2})
	if err != nil {
		t.Fatalf("NewContainer(2): %v", err)
	}
	container2.Close()

	// 不存在的文件 → IsNotExist 静默分支
	cfg3 := testDIConfig()
	// RulesPath 留空 → 默认 configs/alert-rules.json（默认路径分支）
	cfg3.Monitoring.Alerting.Enabled = true
	container3, err := NewContainer(ContainerDeps{Config: cfg3, Redis: client2})
	if err != nil {
		t.Fatalf("NewContainer(3): %v", err)
	}
	container3.Close()

	// SSL 启用：ACME 指向本地模拟目录（不发外网请求）→ 续期器接线分支
	t.Setenv("ACME_DIRECTORY_URL", startMiniACME(t))
	cfg4 := testDIConfig()
	cfg4.SSL = config.SSLConfig{Enabled: true, Email: "ops@example.com"}
	container4, err := NewContainer(ContainerDeps{Config: cfg4, Redis: client2})
	if err != nil {
		t.Fatalf("NewContainer(4): %v", err)
	}
	if container4.SSLManager == nil || container4.SSLAutoRenewer == nil {
		t.Fatal("SSL manager and auto-renewer must be wired when ssl.enabled")
	}

	container4.Close()

	// SSL 启用但缺 email → NewManager 失败 → sslMgr=nil 降级分支
	cfg5 := testDIConfig()
	cfg5.SSL = config.SSLConfig{Enabled: true}
	container5, err := NewContainer(ContainerDeps{Config: cfg5, Redis: client2})
	if err != nil {
		t.Fatalf("NewContainer(5): %v", err)
	}
	if container5.SSLManager != nil {
		t.Log("SSL manager unexpectedly initialized without email")
	}
	container5.Close()
}

// getSecretKey 全分支：环境变量 / 随机
func TestGetSecretKey_Branches(t *testing.T) {
	// 无环境变量 → 随机 64 hex 字符
	if got := getSecretKey(); len(got) != 64 {
		t.Fatalf("random secret broken: len=%d", len(got))
	}
	// 有环境变量 → 直接采用
	t.Setenv("JWT_SECRET", "env-secret-0123456789abcdef")
	if got := getSecretKey(); got != "env-secret-0123456789abcdef" {
		t.Fatalf("env secret not honored: %q", got)
	}
}

// buildThreatIntelConfig 映射（含坏 interval 回退分支）
func TestBuildThreatIntelConfig(t *testing.T) {
	tiCfg := buildThreatIntelConfig(config.ThreatIntelConfig{
		Enabled:     true,
		GlobalKey:   "custom:key",
		MaxIPs:      100,
		Concurrency: 2,
		Sources: []config.ThreatIntelSource{
			{Name: "s1", URL: "http://x", Format: "text", UpdateInterval: "1h", Enabled: true, IPField: "ip"},
			{Name: "s2", URL: "http://y", Format: "csv", UpdateInterval: "not-a-duration", Enabled: false},
		},
	})
	if !tiCfg.Enabled || tiCfg.GlobalKey != "custom:key" || tiCfg.MaxIPs != 100 || tiCfg.Concurrency != 2 {
		t.Fatalf("threat intel config mapping broken: %+v", tiCfg)
	}
	if len(tiCfg.Sources) != 2 || tiCfg.Sources[0].UpdateInterval != time.Hour {
		t.Fatalf("sources mapping broken: %+v", tiCfg.Sources)
	}
	// 坏 interval 回退 6h
	if tiCfg.Sources[1].UpdateInterval != 6*time.Hour {
		t.Fatalf("bad interval fallback broken: %v", tiCfg.Sources[1].UpdateInterval)
	}
	_ = threatintel.DefaultConfig
}

// buildNotificationSource 全分支：渠道禁用/启用×webhook×email(host:port解析)/文件兜底/字段合并
func TestBuildNotificationSource_Branches(t *testing.T) {
	client, err := redis.NewClient("localhost:6379", "", 15)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	repo := repository.NewNotificationChannelsRepository(client)

	fileNotify := config.NotificationsConfig{}
	fn := buildNotificationSource(repo, fileNotify)

	// 无渠道无文件 → 全 nil
	wc, ec := fn()
	if wc != nil || ec != nil {
		t.Fatalf("empty source must yield nils: %+v %+v", wc, ec)
	}

	// 渠道：webhook + email（host:port 解析）
	_ = repo.Save([]repository.NotificationChannelData{
		{Type: "webhook", URL: "http://hook.example/x", Enabled: true},
		{Type: "email", URL: "127.0.0.1:9925", Enabled: true},
		{Type: "webhook", URL: "", Enabled: true},       // 空 URL 跳过
		{Type: "weird", URL: "http://y", Enabled: true}, // 未知类型跳过
	})
	wc, ec = fn()
	if wc == nil || wc.URL != "http://hook.example/x" {
		t.Fatalf("webhook channel broken: %+v", wc)
	}
	if ec == nil || ec.SMTPHost != "127.0.0.1" || ec.SMTPPort != 9925 {
		t.Fatalf("email channel host:port parse broken: %+v", ec)
	}
	// email 渠道无 To → 文件配置补齐
	fileNotify.Email = config.EmailNotificationConfig{Enabled: true, SMTPHost: "file-host", SMTPPort: 25, From: "f@x", To: []string{"ops@x"}}
	fn = buildNotificationSource(repo, fileNotify)
	_, ec = fn()
	if len(ec.To) != 1 || ec.From != "alert@shield.local" {
		t.Fatalf("field-level merge broken: %+v", ec)
	}
	_ = wc

	// 渠道 email 为空 URL → 文件兜底完整生效
	fileNotify.Webhook = config.WebhookNotificationConfig{Enabled: true, URL: "http://file-hook.example", Secret: "s3cret"}
	_ = repo.Save([]repository.NotificationChannelData{{Type: "email", URL: "", Enabled: true}})
	fn = buildNotificationSource(repo, fileNotify) // 重新捕获：闭包按值持有 fileNotify
	wc, ec = fn()
	if ec == nil || ec.SMTPHost != "file-host" || len(ec.To) != 1 || !ec.UseTLS {
		t.Fatalf("file fallback broken: %+v", ec)
	}
	// webhook 文件兜底（渠道无 webhook）
	if wc == nil || wc.URL != fileNotify.Webhook.URL {
		t.Fatalf("webhook file fallback broken: %+v wc=%+v", wc, fileNotify.Webhook)
	}
	_ = wc

	// 渠道 email 有 To 场景不可表达 → 文件 From 已有值时保留渠道 From
	repo.Save([]repository.NotificationChannelData{{Type: "email", URL: "h:25", Enabled: true}})
	_, ec = fn()
	if ec.From != "alert@shield.local" {
		t.Fatalf("channel From priority broken: %q", ec.From)
	}
	client.Del("monitoring:notification-channels")
}

// startMiniACME 精简假 ACME 目录（供 container SSL 接线测试；与 ssl 包内假实现同构）
func startMiniACME(t *testing.T) string {
	t.Helper()
	var base string
	mux := http.NewServeMux()
	nonce := 0
	var mu sync.Mutex
	mux.HandleFunc("/directory", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
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
		w.WriteHeader(200)
	})
	mux.HandleFunc("/new-account", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		nonce++
		n := nonce
		mu.Unlock()
		w.Header().Set("Replay-Nonce", fmt.Sprintf("n%d", n))
		w.Header().Set("Location", base+"/acct/1")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "valid"})
	})
	srv := httptest.NewTLSServer(mux)
	base = srv.URL
	t.Cleanup(srv.Close)
	t.Setenv("ACME_TLS_INSECURE", "1")
	return srv.URL + "/directory"
}
