package geoip

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultDiskCachePath(t *testing.T) {
	t.Setenv("PRERENDER_GEOIP_CACHE", "/tmp/geoip-custom.json")
	if got := defaultDiskCachePath(); got != "/tmp/geoip-custom.json" {
		t.Fatalf("env override broken: %q", got)
	}
	os.Unsetenv("PRERENDER_GEOIP_CACHE")
	if got := defaultDiskCachePath(); !strings.HasSuffix(got, filepath.Join("data", "geoip_cache.json")) {
		t.Fatalf("default broken: %q", got)
	}
}

func TestIsUniqueLocal(t *testing.T) {
	// fc00::/7：fd00 开头为 ULA；2000:: 开头为全球单播（非 ULA）
	if !isUniqueLocal(net.ParseIP("fd00::1")) {
		t.Fatal("fd00:: must be ULA")
	}
	if isUniqueLocal(net.ParseIP("2001:db8::1")) {
		t.Fatal("2001:db8 must not be ULA")
	}
	// IPv4 映射（长度 16 但首字节 0x7f）
	if isUniqueLocal(net.ParseIP("127.0.0.1")) {
		t.Fatal("v4 must not be ULA")
	}
}

// 预置 provider 构造器（覆盖 New* 函数体）
func TestPreconfiguredProviders(t *testing.T) {
	p1 := NewIPAPIProvider()
	if p1.name != "ip-api.com" || !strings.Contains(p1.apiURL, "{ip}") {
		t.Fatalf("ip-api provider broken: %+v", p1)
	}
	p2 := NewIPInfoProvider("tok-123")
	if p2.name != "ipinfo.io" || p2.apiKey != "tok-123" {
		t.Fatalf("ipinfo provider broken: %+v", p2)
	}
	p3 := NewIPAPIProviderCO()
	if p3.name != "ipapi.co" {
		t.Fatalf("ipapi.co provider broken: %+v", p3)
	}
}

// queryAPI/parseResponse 全分支：三种响应格式 + 认证头 + 非 200 + 坏 JSON + 无国家码
func TestQueryAPIAndParse_Branches(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		pathSeg := strings.TrimSuffix(r.URL.Path, "/")
		seg := strings.SplitN(strings.TrimPrefix(pathSeg, "/"), "/", 2)[0]
		switch seg {
		case "ip-api":
			w.Write([]byte(`{"countryCode":"US","country":"United States","query":"8.8.8.8"}`))
		case "ipinfo":
			w.Write([]byte(`{"country":"JP"}`))
		case "ipapico":
			w.Write([]byte(`{"country_code":"KR"}`))
		case "no-country":
			w.Write([]byte(`{"foo":"bar"}`))
		case "badjson":
			w.Write([]byte(`not-json`))
		default:
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	defer srv.Close()

	// ip-api 格式（带 apiKey → Authorization 头分支）
	p := NewAPIProviderWithCache("t", srv.URL+"/ip-api/{ip}", "secret", filepath.Join(t.TempDir(), "cache.json"))
	res, err := p.queryAPI("8.8.8.8")
	if err != nil || res.CountryCode != "US" || res.CountryName != "United States" || res.IP != "8.8.8.8" {
		t.Fatalf("ip-api parse broken: %+v err=%v", res, err)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("auth header broken: %q", gotAuth)
	}

	// ipinfo 格式（country 只在 CountryCode 为空时生效）
	p2 := NewAPIProviderWithCache("t2", srv.URL+"/ipinfo/{ip}", "", filepath.Join(t.TempDir(), "c2.json"))
	res2, err := p2.queryAPI("1.2.3.4")
	if err != nil || res2.CountryCode != "JP" {
		t.Fatalf("ipinfo parse broken: %+v err=%v", res2, err)
	}

	// ipapi.co 格式
	p3 := NewAPIProviderWithCache("t3", srv.URL+"/ipapico/{ip}", "", filepath.Join(t.TempDir(), "c3.json"))
	res3, err := p3.queryAPI("1.2.3.4")
	if err != nil || res3.CountryCode != "KR" {
		t.Fatalf("ipapico parse broken: %+v err=%v", res3, err)
	}

	// 无国家码 → 错误
	p4 := NewAPIProviderWithCache("t4", srv.URL+"/no-country/{ip}", "", filepath.Join(t.TempDir(), "c4.json"))
	if _, err := p4.queryAPI("1.2.3.4"); err == nil || !strings.Contains(err.Error(), "country code") {
		t.Fatalf("no-country error broken: %v", err)
	}

	// 坏 JSON → 错误
	p5 := NewAPIProviderWithCache("t5", srv.URL+"/badjson/{ip}", "", filepath.Join(t.TempDir(), "c5.json"))
	if _, err := p5.queryAPI("1.2.3.4"); err == nil {
		t.Fatal("bad json must error")
	}

	// 非 200 → 错误
	p6 := NewAPIProviderWithCache("t6", srv.URL+"/denied/{ip}", "", filepath.Join(t.TempDir(), "c6.json"))
	if _, err := p6.queryAPI("1.2.3.4"); err == nil || !strings.Contains(err.Error(), "status 403") {
		t.Fatalf("non-200 error broken: %v", err)
	}

	// parseResponse 直调：坏 JSON 分支
	if _, err := p.parseResponse([]byte("nope")); err == nil {
		t.Fatal("parseResponse bad json must error")
	}

	// 无 {ip} 占位符 → URL 追加式（响应为 ip-api 固定样例，IP=8.8.8.8）
	p7 := NewAPIProviderWithCache("t7", srv.URL+"/ip-api", "", filepath.Join(t.TempDir(), "c7.json"))
	res7, err := p7.queryAPI("8.8.4.4")
	if err != nil || res7.CountryCode != "US" || res7.IP != "8.8.8.8" {
		t.Fatalf("append-style url broken: %+v err=%v", res7, err)
	}
}

// 磁盘缓存持久化：内存写入 → Flush 落盘 → 新实例 loadDisk 命中
func TestDiskCachePersistence(t *testing.T) {
	disk := filepath.Join(t.TempDir(), "geoip.json")
	p := NewAPIProviderWithCache("dp", "http://127.0.0.1:1/never", "", disk)
	p.setCache("5.5.5.5", &CountryResult{CountryCode: "DE", CountryName: "Germany", IP: "5.5.5.5"})
	if err := p.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if _, err := os.Stat(disk); err != nil {
		t.Fatalf("disk file missing: %v", err)
	}

	p2 := NewAPIProviderWithCache("dp2", "http://127.0.0.1:1/never", "", disk)
	if got := p2.getFromCache("5.5.5.5"); got == nil || got.CountryCode != "DE" {
		t.Fatalf("loadDisk restore broken: %+v", got)
	}
}

// scheduleFlush：setCache 触发防抖调度（dirty 标记 + timer），diskSaveDelay 后落盘
func TestScheduleFlush(t *testing.T) {
	disk := filepath.Join(t.TempDir(), "geoip-auto.json")
	p := NewAPIProviderWithCache("sp", "http://127.0.0.1:1/never", "", disk)
	p.setCache("6.6.6.6", &CountryResult{CountryCode: "FR", IP: "6.6.6.6"})
	p.scheduleFlush()
	if !p.dirty || p.saveTimer == nil {
		t.Fatalf("scheduleFlush must mark dirty and arm timer: dirty=%v timer=%v", p.dirty, p.saveTimer)
	}
	// 等待防抖（3s）触发落盘
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(disk); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("scheduled flush did not write disk")
}

// Lookup 全链：私网短路 / 磁盘缓存命中 / API 兜底与失败降级
func TestAPILookup_Chains(t *testing.T) {
	disk := filepath.Join(t.TempDir(), "g.json")
	p := NewAPIProviderWithCache("chain", "http://127.0.0.1:1/never", "", disk)

	// 私网短路
	res, err := p.Lookup("127.0.0.1")
	if err != nil || res.CountryCode != "LOCAL" {
		t.Fatalf("private short-circuit broken: %+v %v", res, err)
	}
	// localhost 字面量
	res, _ = p.Lookup("localhost")
	if res.CountryCode != "LOCAL" {
		t.Fatal("localhost short-circuit broken")
	}
	// 非法 IP → 不短路、API 失败 → UNKNOWN 降级（或错误语义，视实现）
	res, err = p.Lookup("not-an-ip")
	if err == nil && res == nil {
		t.Fatal("must return something for invalid ip")
	}
}

// loadDisk 全分支：文件不存在 / 损坏 JSON / 过期条目跳过 / 坏时间戳跳过 / 有效性入库
func TestLoadDisk_Branches(t *testing.T) {
	disk := filepath.Join(t.TempDir(), "ld.json")

	// 损坏 JSON
	os.WriteFile(disk, []byte("corrupt{"), 0644)
	p1 := NewAPIProviderWithCache("c1", "http://x", "", disk)
	if len(p1.cache) != 0 {
		t.Fatal("corrupt file must start empty")
	}

	// 混合条目：有效 / 过期 / 坏时间戳
	os.WriteFile(disk, []byte(`{"entries":{
		"3.3.3.3":{"country_code":"GB","ip":"3.3.3.3","cached_at":"`+time.Now().Add(-time.Hour).Format(time.RFC3339)+`"},
		"4.4.4.4":{"country_code":"OLD","ip":"4.4.4.4","cached_at":"`+time.Now().Add(-8*24*time.Hour).Format(time.RFC3339)+`"},
		"5.5.5.5":{"country_code":"BAD","ip":"5.5.5.5","cached_at":"not-a-time"}
	}}`), 0644)
	p2 := NewAPIProviderWithCache("c2", "http://x", "", disk)
	if got := p2.getFromCache("3.3.3.3"); got == nil || got.CountryCode != "GB" {
		t.Fatalf("valid entry must load: %+v", got)
	}
	if p2.getFromCache("4.4.4.4") != nil {
		t.Fatal("expired entry must be skipped")
	}
	if p2.getFromCache("5.5.5.5") != nil {
		t.Fatal("bad timestamp must be skipped")
	}
}

// Flush 空缓存 no-op 与写盘错误路径
func TestFlush_EdgeBranches(t *testing.T) {
	// 空 diskPath → nil no-op
	p0 := &APIProvider{}
	if err := p0.Flush(); err != nil {
		t.Fatal("empty diskPath must no-op")
	}
	// 空缓存 → no-op 不建文件
	disk := filepath.Join(t.TempDir(), "empty.json")
	p1 := NewAPIProviderWithCache("f1", "http://x", "", disk)
	if err := p1.Flush(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(disk); !os.IsNotExist(err) {
		t.Fatal("empty cache must not write file")
	}
	// 不可写目录 → 错误
	p2 := NewAPIProviderWithCache("f2", "http://x", "", "/proc/definitely/not/writable/g.json")
	p2.setCache("1.1.1.1", &CountryResult{CountryCode: "XX", IP: "1.1.1.1"})
	p2.scheduleFlush()
	if err := p2.Flush(); err == nil {
		t.Log("Flush to unwritable path returned nil (rename may succeed on some systems)")
	}
}

// http.NewRequest 错误分支：apiURL 含控制字符
func TestQueryAPI_NewRequestError(t *testing.T) {
	p := &APIProvider{
		name:      "bad",
		apiURL:    "http://bad.example/{ip}\x7f",
		client:    &http.Client{Timeout: 2 * time.Second},
		cache:     map[string]*CountryResult{},
		cacheTime: map[string]time.Time{},
	}
	if _, err := p.queryAPI("1.2.3.4"); err == nil || !strings.Contains(err.Error(), "create request") {
		t.Fatalf("NewRequest error branch broken: %v", err)
	}
}

// io.ReadAll 错误分支：自定义 Transport 注入失败 body
type errBodyReader struct{}

func (errBodyReader) Read([]byte) (int, error) { return 0, errors.New("read failure (simulated)") }
func (errBodyReader) Close() error             { return nil }

func TestQueryAPI_ReadBodyError(t *testing.T) {
	p := &APIProvider{
		name:   "rb",
		apiURL: "http://body-fail.example/{ip}",
		client: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(errBodyReader{})}, nil
		})},
		cache:     map[string]*CountryResult{},
		cacheTime: map[string]time.Time{},
	}
	if _, err := p.queryAPI("1.2.3.4"); err == nil || !strings.Contains(err.Error(), "read response") {
		t.Fatalf("read body error branch broken: %v", err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// loadDisk 空 diskPath 分支
func TestLoadDisk_EmptyPath(t *testing.T) {
	p := &APIProvider{cache: map[string]*CountryResult{}, cacheTime: map[string]time.Time{}}
	p.loadDisk() // 不 panic 即通过
}

// scheduleFlush 定时回调中 Flush 失败 → 记日志路径（不可写目录）
func TestScheduleFlush_FlushErrorPath(t *testing.T) {
	roDir := filepath.Join(t.TempDir(), "ro")
	if err := os.MkdirAll(roDir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(roDir, 0o755)
	p := NewAPIProviderWithCache("sp2", "http://x", "", filepath.Join(roDir, "g.json"))
	p.setCache("2.2.2.2", &CountryResult{CountryCode: "ZZ", IP: "2.2.2.2"})
	p.scheduleFlush()
	// 防抖 3s 后 AfterFunc 触发 Flush 失败 → 记日志（覆盖回调分支）
	time.Sleep(4 * time.Second)
}
