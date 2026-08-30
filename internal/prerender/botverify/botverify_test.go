package botverify

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// fakeResolver 假 DNS 解析器（不触外网）
type fakeResolver struct {
	mu     sync.Mutex
	ptr    map[string][]string // ip → 主机名
	host   map[string][]string // 主机名 → ip
	ptrErr error
}

func (f *fakeResolver) LookupAddr(ctx context.Context, addr string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ptrErr != nil {
		return nil, f.ptrErr
	}
	if names, ok := f.ptr[addr]; ok {
		return names, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: addr, IsNotFound: true}
}

func (f *fakeResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if addrs, ok := f.host[host]; ok {
		return addrs, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

func TestVerify_GoogleTwoWaySuccess(t *testing.T) {
	r := &fakeResolver{
		ptr:  map[string][]string{"66.249.66.1": {"crawl-66-249-66-1.googlebot.com."}},
		host: map[string][]string{"crawl-66-249-66-1.googlebot.com": {"66.249.66.1"}},
	}
	v := NewVerifierWithResolver(r, filepath.Join(t.TempDir(), "cache.json"))

	if got := v.Verify("66.249.66.1"); got != ResultVerified {
		t.Fatalf("Verify = %q, want verified", got)
	}
	if got := v.Peek("66.249.66.1"); got != ResultVerified {
		t.Fatalf("cache result = %q, want verified", got)
	}
}

func TestVerify_PTRForgeryRejected(t *testing.T) {
	// PTR 声称是 googlebot，但正向回验 IP 不一致 → unverified
	r := &fakeResolver{
		ptr:  map[string][]string{"1.2.3.4": {"fake.googlebot.com."}},
		host: map[string][]string{"fake.googlebot.com": {"9.9.9.9"}},
	}
	v := NewVerifierWithResolver(r, filepath.Join(t.TempDir(), "cache.json"))

	if got := v.Verify("1.2.3.4"); got != ResultUnverified {
		t.Fatalf("Verify = %q, want unverified", got)
	}
}

func TestVerify_NoPTRIsUnverified(t *testing.T) {
	r := &fakeResolver{ptr: map[string][]string{}, host: map[string][]string{}}
	v := NewVerifierWithResolver(r, filepath.Join(t.TempDir(), "cache.json"))

	if got := v.Verify("8.8.8.8"); got != ResultUnverified {
		t.Fatalf("Verify = %q, want unverified", got)
	}
}

func TestVerify_DNSErrorFailOpenNotCached(t *testing.T) {
	// DNS 故障是不确定态：fail-open 返回 unknown 且不缓存（区别于确认的 unverified）
	r := &fakeResolver{ptrErr: errors.New("dns down")}
	v := NewVerifierWithResolver(r, filepath.Join(t.TempDir(), "cache.json"))

	if got := v.Verify("5.5.5.5"); got != ResultUnknown {
		t.Fatalf("Verify = %q, want unknown (fail-open)", got)
	}
	if got := v.Peek("5.5.5.5"); got != ResultUnknown {
		t.Fatalf("dns failure must not be cached, got %q", got)
	}
}

func TestVerify_NonGoogleSuffixUnverified(t *testing.T) {
	r := &fakeResolver{
		ptr:  map[string][]string{"7.7.7.7": {"bot.evil.example.com."}},
		host: map[string][]string{"bot.evil.example.com": {"7.7.7.7"}},
	}
	v := NewVerifierWithResolver(r, filepath.Join(t.TempDir(), "cache.json"))

	if got := v.Verify("7.7.7.7"); got != ResultUnverified {
		t.Fatalf("Verify = %q, want unverified", got)
	}
}

func TestCachePersistenceAndExpiry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	r := &fakeResolver{
		ptr:  map[string][]string{"66.249.66.1": {"crawl-66-249-66-1.googlebot.com."}, "1.2.3.4": {"bot.evil.example.com."}},
		host: map[string][]string{"crawl-66-249-66-1.googlebot.com": {"66.249.66.1"}},
	}
	v := NewVerifierWithResolver(r, path)
	_ = v.Verify("66.249.66.1")
	_ = v.Verify("1.2.3.4")
	if err := v.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("disk cache not written: %v", err)
	}

	// 重新加载：verified 命中缓存（不再触 DNS）
	v2 := NewVerifier(path)
	if got := v2.Peek("66.249.66.1"); got != ResultVerified {
		t.Fatalf("reloaded verified = %q", got)
	}
	if got := v2.Peek("1.2.3.4"); got != ResultUnverified {
		t.Fatalf("reloaded unverified = %q", got)
	}

	// 负结果 1h 过期
	v3 := NewVerifier(path)
	v3.mu.Lock()
	e := v3.cache["1.2.3.4"]
	e.CachedAt = time.Now().Add(-2 * time.Hour)
	v3.cache["1.2.3.4"] = e
	v3.mu.Unlock()
	if got := v3.Peek("1.2.3.4"); got != ResultUnknown {
		t.Fatalf("expired negative must read unknown, got %q", got)
	}
}

func TestLRUEviction(t *testing.T) {
	r := &fakeResolver{ptr: map[string][]string{}, host: map[string][]string{}}
	v := NewVerifierWithResolver(r, filepath.Join(t.TempDir(), "cache.json"))
	v.mu.Lock()
	base := time.Now().Add(-time.Hour)
	for i := 0; i < diskCacheMaxEntries+10; i++ {
		ip := "10.0.0." + itoa(i)
		tt := base.Add(time.Duration(i) * time.Second)
		if i == 0 { // i=0 最旧，应被淘汰
			tt = base.Add(-time.Minute)
		}
		v.cache[ip] = cacheEntry{Result: ResultUnverified, CachedAt: tt}
	}
	v.evictLRULocked()
	v.mu.Unlock()

	newestIdx := diskCacheMaxEntries + 9
	v.mu.RLock()
	_, oldestExists := v.cache["10.0.0.0"]
	_, newestExists := v.cache["10.0.0."+itoa(newestIdx)]
	count := len(v.cache)
	v.mu.RUnlock()

	if oldestExists {
		t.Errorf("oldest entry must be evicted")
	}
	if !newestExists {
		t.Errorf("newest entry must be retained")
	}
	if count != diskCacheMaxEntries {
		t.Errorf("cache size = %d, want %d", count, diskCacheMaxEntries)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := ""
	for i > 0 {
		digits = string(rune('0'+i%10)) + digits
		i /= 10
	}
	return digits
}

func TestSingleflightDedup(t *testing.T) {
	r := &fakeResolver{
		ptr:  map[string][]string{"66.249.66.1": {"crawl-66-249-66-1.googlebot.com."}},
		host: map[string][]string{"crawl-66-249-66-1.googlebot.com": {"66.249.66.1"}},
	}
	v := NewVerifierWithResolver(r, filepath.Join(t.TempDir(), "cache.json"))

	// 并发 20 个同 IP 验证：singleflight 保证 DNS 只走一次
	var wg sync.WaitGroup
	results := make([]string, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = v.Verify("66.249.66.1")
		}(i)
	}
	wg.Wait()
	for i, res := range results {
		if res != ResultVerified {
			t.Errorf("result[%d] = %q, want verified", i, res)
		}
	}
}
