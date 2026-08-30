package sitehandler

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"prerender-shield/internal/config"
	"prerender-shield/internal/prerender"
	"prerender-shield/internal/prerender/botverify"
)

// localFakeResolver 假 DNS 解析器（不触外网；与 botverify.fakeResolver 同构）
type localFakeResolver struct {
	ptr  map[string][]string
	host map[string][]string
}

func (f *localFakeResolver) LookupAddr(ctx context.Context, addr string) ([]string, error) {
	if names, ok := f.ptr[addr]; ok {
		return names, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: addr, IsNotFound: true}
}

func (f *localFakeResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	if addrs, ok := f.host[host]; ok {
		return addrs, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

func newBotVerifyTestHandler(t *testing.T, ptr map[string][]string, host map[string][]string) *Handler {
	t.Helper()
	v := botverify.NewVerifierWithResolver(&localFakeResolver{ptr: ptr, host: host}, filepath.Join(t.TempDir(), "bv.json"))
	return &Handler{botVerifier: v}
}

func TestBotVerifyFor_LogMode(t *testing.T) {
	// 确认伪造的 IP（PTR 指向非 Google 域）
	h := newBotVerifyTestHandler(t,
		map[string][]string{"1.2.3.4": {"evil.example.com."}},
		map[string][]string{"evil.example.com": {"1.2.3.4"}})
	site := config.SiteConfig{Firewall: config.FirewallConfig{BotVerify: config.BotVerifyConfig{Enabled: true, Mode: "log"}}}

	got := h.botVerifyFor(site, prerender.CatSearch, "Mozilla/5.0 (compatible; Googlebot/2.1)", "1.2.3.4")
	// log 模式：未缓存时本次不打标（异步回填），返回空
	if got != "" {
		t.Fatalf("log mode first hit must not block/label, got %q", got)
	}
	// 异步验证回填后可 Peek 到确定性结果（轮询等待）
	for i := 0; i < 100; i++ {
		if r := h.botVerifier.Peek("1.2.3.4"); r == botverify.ResultUnverified {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("async verification did not backfill cache")
}

func TestBotVerifyFor_BlockModeVerdicts(t *testing.T) {
	// 真实 Google：PTR → googlebot.com 且正向回验一致
	h := newBotVerifyTestHandler(t,
		map[string][]string{"66.249.66.1": {"crawl-66-249-66-1.googlebot.com."}},
		map[string][]string{"crawl-66-249-66-1.googlebot.com": {"66.249.66.1"}})
	blockSite := config.SiteConfig{Firewall: config.FirewallConfig{BotVerify: config.BotVerifyConfig{Enabled: true, Mode: "block"}}}

	if got := h.botVerifyFor(blockSite, prerender.CatSearch, "Googlebot/2.1", "66.249.66.1"); got != botverify.ResultVerified {
		t.Fatalf("genuine googlebot must be verified, got %q", got)
	}

	// 确认伪造：PTR 存在但指向非官方域
	h2 := newBotVerifyTestHandler(t,
		map[string][]string{"1.2.3.4": {"fake.example.com."}},
		map[string][]string{"fake.example.com": {"1.2.3.4"}})
	if got := h2.botVerifyFor(blockSite, prerender.CatSearch, "Googlebot/2.1", "1.2.3.4"); got != botverify.ResultUnverified {
		t.Fatalf("fake googlebot must be unverified, got %q", got)
	}

	// 非搜索类爬虫不触发验证（AI 类零 DNS 成本）
	if got := h2.botVerifyFor(blockSite, prerender.CatAI, "GPTBot/1.0", "1.2.3.4"); got != "" {
		t.Fatalf("non-search category must skip verification, got %q", got)
	}

	// 未启用直接跳过
	off := config.SiteConfig{}
	if got := h2.botVerifyFor(off, prerender.CatSearch, "Googlebot/2.1", "1.2.3.4"); got != "" {
		t.Fatalf("disabled bot_verify must skip, got %q", got)
	}
}
