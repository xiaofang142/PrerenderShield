package geoip

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAPIProvider_Name(t *testing.T) {
	p := NewAPIProvider("test-src", "http://example.com/api", "key")
	if p.Name() != "test-src" {
		t.Fatalf("expected name test-src, got %s", p.Name())
	}
}

func TestAPIProvider_Lookup_PrivateIP_NoHTTPCall(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	p := NewAPIProvider("t", srv.URL, "")
	res, err := p.Lookup("192.168.1.1")
	if err != nil {
		t.Fatalf("private IP lookup should not error, got %v", err)
	}
	if called {
		t.Fatal("private IP must be answered from cache/local, without HTTP call")
	}
	if res == nil || res.CountryCode == "" {
		t.Fatalf("expected a local result for private IP, got %+v", res)
	}
}

func TestAPIProvider_parseResponse_IPAPIFormat(t *testing.T) {
	p := NewIPAPIProvider()
	res, err := p.parseResponse([]byte(`{"countryCode":"DE","country":"Germany","query":"8.8.8.8"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.CountryCode != "DE" || res.IP != "8.8.8.8" {
		t.Fatalf("unexpected parse result: %+v", res)
	}
}

func TestAPIProvider_CacheRoundTrip(t *testing.T) {
	p := NewAPIProviderWithCache("t", "http://unused", "", filepath.Join(t.TempDir(), "cache.json"))
	p.setCache("9.9.9.9", &CountryResult{CountryCode: "JP", IP: "9.9.9.9"})
	got := p.getFromCache("9.9.9.9")
	if got == nil || got.CountryCode != "JP" {
		t.Fatalf("cache round-trip failed: %+v", got)
	}
	p.ClearCache()
	if p.getFromCache("9.9.9.9") != nil {
		t.Fatal("cache should be empty after ClearCache")
	}
}

// C6-3: API 失败时回退到超过 memoryTTL 但仍在 diskTTL 内的历史缓存
func TestAPIProvider_Lookup_FallsBackToStaleCache(t *testing.T) {
	p := NewAPIProviderWithCache("t", "http://127.0.0.1:1/unreachable", "", filepath.Join(t.TempDir(), "cache.json"))
	// 注入一条 2 天前的旧结果（已超 24h 内存 TTL，未超 7 天磁盘 TTL）
	old := time.Now().Add(-48 * time.Hour)
	p.cacheMu.Lock()
	p.cache["8.8.8.8"] = &CountryResult{CountryCode: "US", IP: "8.8.8.8"}
	p.cacheTime["8.8.8.8"] = old
	p.cacheMu.Unlock()

	res, err := p.Lookup("8.8.8.8")
	if err != nil {
		t.Fatalf("expected stale fallback, got error: %v", err)
	}
	if res == nil || res.CountryCode != "US" {
		t.Fatalf("unexpected stale fallback result: %+v", res)
	}
}

// 超过 diskTTL 的旧结果不得作为兜底
func TestAPIProvider_Lookup_StaleCacheExpired(t *testing.T) {
	p := NewAPIProviderWithCache("t", "http://127.0.0.1:1/unreachable", "", filepath.Join(t.TempDir(), "cache.json"))
	tooOld := time.Now().Add(-8 * 24 * time.Hour)
	p.cacheMu.Lock()
	p.cache["8.8.8.8"] = &CountryResult{CountryCode: "US", IP: "8.8.8.8"}
	p.cacheTime["8.8.8.8"] = tooOld
	p.cacheMu.Unlock()

	if _, err := p.Lookup("8.8.8.8"); err == nil {
		t.Fatal("expected error when only an expired cache entry exists")
	}
}

// C7-2: 缓存超过容量上限时按最早解析时间 LRU 淘汰
func TestAPIProvider_LRUEviction(t *testing.T) {
	p := NewAPIProviderWithCache("t", "http://unused", "", filepath.Join(t.TempDir(), "cache.json"))

	p.cacheMu.Lock()
	for i := 0; i <= diskCacheMaxEntries; i++ {
		ip := fmt.Sprintf("10.%d.%d.%d", i>>16&0xff, i>>8&0xff, i&0xff)
		p.cache[ip] = &CountryResult{CountryCode: "XX", IP: ip}
		p.cacheTime[ip] = time.Now().Add(time.Duration(i) * time.Millisecond)
	}
	p.evictLRULocked()
	size := len(p.cache)
	_, oldestKept := p.cache["10.0.0.0"]
	_, newestKept := p.cache[fmt.Sprintf("10.%d.%d.%d", diskCacheMaxEntries>>16&0xff, diskCacheMaxEntries>>8&0xff, diskCacheMaxEntries&0xff)]
	p.cacheMu.Unlock()

	if size != diskCacheMaxEntries {
		t.Fatalf("cache size after eviction = %d, want %d", size, diskCacheMaxEntries)
	}
	if oldestKept {
		t.Fatal("oldest entry should have been evicted")
	}
	if !newestKept {
		t.Fatal("newest entry must be retained")
	}
}

// 磁盘持久化：成功解析后落盘，新实例（模拟重启）在 API 不可用时仍可命中
func TestAPIProvider_DiskPersistenceAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache.json")

	var apiCalled int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalled++
		w.Write([]byte(`{"countryCode":"DE","country":"Germany","query":"8.8.4.4"}`))
	}))

	p1 := NewAPIProviderWithCache("t", srv.URL+"/{ip}", "", cachePath)
	res, err := p1.Lookup("8.8.4.4")
	if err != nil || res.CountryCode != "DE" {
		t.Fatalf("first lookup failed: %v %+v", err, res)
	}
	srv.Close()

	// 直接落盘（不依赖防抖定时器时序）
	if err := p1.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	raw, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("cache file missing after flush: %v", err)
	}
	var file diskFile
	if err := json.Unmarshal(raw, &file); err != nil || len(file.Entries) == 0 {
		t.Fatalf("invalid cache file: %v entries=%d", err, len(file.Entries))
	}

	// 模拟进程重启：API 已关闭，应从磁盘缓存命中
	p2 := NewAPIProviderWithCache("t", srv.URL+"/{ip}", "", cachePath)
	res2, err := p2.Lookup("8.8.4.4")
	if err != nil {
		t.Fatalf("lookup after restart should hit disk cache, got: %v", err)
	}
	if res2.CountryCode != "DE" {
		t.Fatalf("unexpected result after restart: %+v", res2)
	}
	if apiCalled != 1 {
		t.Fatalf("restart must not call API, got %d calls", apiCalled)
	}
}

func TestIsPrivateIP(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1":     true,
		"10.0.0.5":      true,
		"172.16.3.4":    true,
		"192.168.0.1":   true,
		"::1":           true,
		"fe80::1":       true,
		"8.8.8.8":       false,
		"1.1.1.1":       false,
		"not-an-ip":     false,
		"172.32.0.1":    false, // 172.16-31 才是私网
	}
	for ip, want := range cases {
		if got := isPrivateIP(ip); got != want {
			t.Errorf("isPrivateIP(%q) = %v, want %v", ip, got, want)
		}
	}
}
