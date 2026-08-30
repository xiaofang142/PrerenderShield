package ssl

import (
	"errors"
	"testing"
	"time"

	"github.com/go-acme/lego/v4/certificate"

	"prerender-shield/internal/redis"
)

// NewAutoRenewer 禁用态：Start 不启动 goroutine，Stop 幂等安全
func TestAutoRenewer_DisabledLifecycle(t *testing.T) {
	client, err := redis.NewClient("localhost:6379", "", 0)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	r := NewAutoRenewer(nil, client, AutoRenewConfig{Enabled: false})
	r.Start()
	r.Stop()
	r.Stop() // 幂等
}

// ACME 客户端未初始化时 checkAndRenew 只告警不 panic
func TestAutoRenewer_NilACMEClient_NoPanic(t *testing.T) {
	client, err := redis.NewClient("localhost:6379", "", 0)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	r := NewAutoRenewer(nil, client, AutoRenewConfig{
		Enabled:       true,
		CheckInterval: 10 * time.Millisecond,
	})
	// 直接调 checkAndRenew（nil acme → 告警返回）
	r.checkAndRenew()
	// Start 后立即 Stop（run loop 启停路径）
	r.Start()
	time.Sleep(20 * time.Millisecond)
	r.Stop()
}

// NewACMEClient 配置校验：非法参数必须报错而非带病启动
func TestNewACMEClient_InvalidConfig(t *testing.T) {
	if _, err := NewACMEClient(ACMEConfig{Email: ""}); err == nil {
		t.Fatal("empty email must be rejected")
	}
}

// requestCertificate 与 renewCertificate 对 nil/坏参数的防御路径
func TestACMEClient_RequestCertificate_Validation(t *testing.T) {
	c, err := NewACMEClient(ACMEConfig{Email: "test@example.com", CertDir: t.TempDir(), HTTPPort: 8090})
	if err != nil {
		t.Skipf("ACME client init requires environment: %v", err)
	}
	// 空域名列表必须拒绝（不发网络请求）
	if _, err := c.RequestCertificate(nil); err == nil {
		t.Fatal("empty domain list must be rejected")
	}
}

// fakeRenewACME 模拟 ACME：证书列表带 expiresIn，验证续签决策循环
type fakeRenewACME struct {
	certs    []map[string]interface{}
	renewed  []string
	failList bool
}

func (f *fakeRenewACME) ListCertificates() ([]map[string]interface{}, error) {
	if f.failList {
		return nil, errors.New("list failed (simulated)")
	}
	return f.certs, nil
}

func (f *fakeRenewACME) RenewCertificate(domain string) (*certificate.Resource, error) {
	f.renewed = append(f.renewed, domain)
	return &certificate.Resource{Domain: domain}, nil
}

// 续签决策：即将到期(RenewBeforeDays内)→续签；已过期→紧急续签；未到期→跳过
func TestAutoRenewer_CheckAndRenew_DecisionLoop(t *testing.T) {
	client, err := redis.NewClient("localhost:6379", "", 0)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	fake := &fakeRenewACME{certs: []map[string]interface{}{
		{"domain": "expiring.example", "expires_in": 5},    // < RenewBeforeDays → 续签
		{"domain": "expired.example", "expires_in": -1},    // 已过期 → 紧急续签
		{"domain": "fresh.example", "expires_in": 90},      // 未到期 → 跳过
		{"domain": "bad-field.example", "expires_in": "x"}, // 字段类型错 → 跳过
	}}
	r := NewAutoRenewer(fake, client, AutoRenewConfig{
		Enabled:         true,
		RenewBeforeDays: 30,
		CheckInterval:   time.Hour,
	})
	r.checkAndRenew()

	if len(fake.renewed) != 2 {
		t.Fatalf("renewed=%v, want [expiring expired]", fake.renewed)
	}
	if fake.renewed[0] != "expiring.example" || fake.renewed[1] != "expired.example" {
		t.Fatalf("renewal order/targets wrong: %v", fake.renewed)
	}
}

// List 失败：只告警不 panic
func TestAutoRenewer_CheckAndRenew_ListFailure(t *testing.T) {
	client, err := redis.NewClient("localhost:6379", "", 0)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	r := NewAutoRenewer(&fakeRenewACME{failList: true}, client, AutoRenewConfig{Enabled: true})
	r.checkAndRenew()
}
