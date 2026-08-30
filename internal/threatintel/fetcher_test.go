package threatintel

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"prerender-shield/internal/redis"
)

var hits int32 = 1

func TestIsValidIP(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"1.2.3.4", true},
		{"::1", false},         // 设计：黑名单源仅 IPv4
		{"2001:db8::1", false}, // 同上
		{"999.1.1.1", false},   // 字节越界（回归：曾仅查位数不查范围）
		{"256.1.1.1", false},
		{"not-an-ip", false},
		{"", false},
		{"1.2.3.4/32", false}, // 纯 IP 校验不接受 CIDR
	}
	for _, c := range cases {
		if got := isValidIP(c.in); got != c.want {
			t.Errorf("isValidIP(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestExtractIP(t *testing.T) {
	cases := []struct{ in, want string }{
		{"1.2.3.4", "1.2.3.4"},
		{"1.2.3.4  # comment", "1.2.3.4"},
		{"1.2.3.4:8080", "1.2.3.4"},    // IPv4:port 剥端口
		{"1.2.3.0/24", "1.2.3.0"},      // 剥子网
		{"2001:db8::1", "2001:db8::1"}, // IPv6 无冒号误剥
		{"", ""},
	}
	for _, c := range cases {
		if got := extractIP(c.in); got != c.want {
			t.Errorf("extractIP(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExpandCIDR(t *testing.T) {
	// 大段（/24）：存 CIDR 本体不展开
	got := expandCIDR("1.2.3.0/24")
	if len(got) != 1 || got[0] != "1.2.3.0/24" {
		t.Fatalf("/24 must stay as CIDR, got %v", got)
	}
	// 小段（/30）：展开为 2 个可用主机位
	got = expandCIDR("10.0.0.0/30")
	if len(got) != 2 {
		t.Fatalf("/30 must expand to 2 hosts, got %v", got)
	}
	// 非 CIDR
	if got := expandCIDR("1.2.3.4"); got != nil {
		t.Fatalf("plain ip must return nil, got %v", got)
	}
	if got := expandCIDR("bad/40"); got != nil {
		t.Fatalf("invalid cidr must return nil, got %v", got)
	}
}

func TestParseLine(t *testing.T) {
	cases := []struct{ in, want string }{
		{"1.2.3.4", "1.2.3.4"},
		{"1.2.3.4\t#spam", "1.2.3.4"},
		{"  5.6.7.8  ", "5.6.7.8"},
	}
	for _, c := range cases {
		if got := parseLine(c.in); got != c.want {
			t.Errorf("parseLine(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestIsCIDR(t *testing.T) {
	if !isCIDR("1.2.3.0/24") {
		t.Error("1.2.3.0/24 must be CIDR")
	}
	if isCIDR("1.2.3.4") {
		t.Error("plain ip is not CIDR")
	}
}

func TestSanitizeKey(t *testing.T) {
	f := NewFetcher(Config{}, nil)
	got := f.sanitizeKey("Abuse.ch Feodo Tracker Test")
	if got != "abuse_ch_feodo_tracker_test" {
		t.Fatalf("sanitize broken: %q", got)
	}
	if got := f.sanitizeKey(""); got != "" {
		t.Fatalf("empty name sanitize: %q", got)
	}
}

func TestParseCSV(t *testing.T) {
	f := NewFetcher(Config{MaxIPs: 100}, nil)
	csvData := "first_seen,dst_ip\n2024-01-01,1.2.3.4\n# comment row\n2024-01-02,5.6.7.8\n2024-01-03,999.9.9.9\n"
	ips, err := f.parseCSV(strings.NewReader(csvData), "dst_ip")
	if err != nil {
		t.Fatalf("parseCSV err: %v", err)
	}
	set := map[string]bool{}
	for _, ip := range ips {
		set[ip] = true
	}
	if !set["1.2.3.4"] || !set["5.6.7.8"] {
		t.Fatalf("dst_ip column not parsed: %v", ips)
	}
	if set["999.9.9.9"] {
		t.Fatal("invalid ip must be filtered")
	}
	// 未知字段回退第一列（mapKeys 无序，用集合断言）
	ips, err = f.parseCSV(strings.NewReader("ip,extra\n9.9.9.9,x\n"), "nonexistent")
	set2 := map[string]bool{}
	for _, ip := range ips {
		set2[ip] = true
	}
	if err != nil || len(ips) != 1 || !set2["9.9.9.9"] {
		t.Fatalf("fallback first column broken: %v err=%v", ips, err)
	}
	// MaxIPs 截断
	f2 := NewFetcher(Config{MaxIPs: 2}, nil)
	ips, _ = f2.parseCSV(strings.NewReader("dst_ip\n1.1.1.1\n2.2.2.2\n3.3.3.3\n"), "dst_ip")
	if len(ips) != 2 {
		t.Fatalf("MaxIPs cap broken: %v", ips)
	}
}

func TestParseText(t *testing.T) {
	f := NewFetcher(Config{MaxIPs: 100}, nil)
	text := "# comment\n; also comment\n\n1.2.3.4\n5.6.7.8:80\n9.9.9.9 ;foo\n11.0.0.0/30\n"
	ips, err := f.parseText(strings.NewReader(text))
	if err != nil {
		t.Fatalf("parseText err: %v", err)
	}
	set := map[string]bool{}
	for _, ip := range ips {
		set[ip] = true
	}
	for _, want := range []string{"1.2.3.4", "5.6.7.8", "9.9.9.9"} {
		if !set[want] {
			t.Errorf("missing %q in %v", want, ips)
		}
	}
	if set["5.6.7.8:80"] {
		t.Error("port notation must be stripped")
	}
	if !set["11.0.0.0/30"] {
		t.Errorf("CIDR must be kept as-is, got %v", ips)
	}
}

func TestParseJSON(t *testing.T) {
	f := NewFetcher(Config{MaxIPs: 100}, nil)
	data := `[{"ip":"1.2.3.4"},{"ip":"5.6.7.8"}]`
	ips, err := f.parseJSON(strings.NewReader(data), "ip")
	if err != nil {
		t.Fatalf("parseJSON err: %v", err)
	}
	if len(ips) != 2 {
		t.Fatalf("want 2 ips, got %v", ips)
	}
}

func TestDownloadAndParse_HTTPSource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("1.2.3.4\n5.6.7.8\n"))
	}))
	defer srv.Close()

	f := NewFetcher(Config{MaxIPs: 100}, nil)
	ips, err := f.downloadAndParse(Source{Name: "test", URL: srv.URL, Format: "text"})
	if err != nil {
		t.Fatalf("downloadAndParse err: %v", err)
	}
	if len(ips) != 2 {
		t.Fatalf("want 2 ips, got %v", ips)
	}

	// 服务器 500
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv2.Close()
	if _, err := f.downloadAndParse(Source{Name: "bad", URL: srv2.URL, Format: "text"}); err == nil {
		t.Fatal("500 response must produce error")
	}

	// 未知格式：实现回退 parseText（宽进策略，不报错）
	ips2, err2 := f.downloadAndParse(Source{Name: "x", URL: srv.URL, Format: "xml"})
	if err2 != nil || len(ips2) != 2 {
		t.Fatalf("unknown format must fall back to text parse, got %v err=%v", ips2, err2)
	}
}

func TestIsThreatIP_NilRedis(t *testing.T) {
	f := NewFetcher(Config{}, nil)
	// 无 Redis 时 fail-open：不误报
	if f.IsThreatIP("1.2.3.4") {
		t.Fatal("without redis must fail-open (not threat)")
	}
}

func TestMergeConfig(t *testing.T) {
	f := NewFetcher(Config{Enabled: false, GlobalKey: "k1", MaxIPs: 10}, nil)
	other := Config{
		Enabled:   true,
		GlobalKey: "k2",
		MaxIPs:    20,
		Sources: []Source{
			{Name: "A", Enabled: true, UpdateInterval: time.Hour},
			{Name: "B", Enabled: false},
		},
	}
	f.MergeConfig(other)
	// 实现语义：sources 按名并集、MaxIPs/Concurrency 取大；Enabled/GlobalKey 不动
	if f.config.MaxIPs != 20 {
		t.Fatalf("MaxIPs must take max, got %d", f.config.MaxIPs)
	}
	if f.config.Enabled || f.config.GlobalKey != "k1" {
		t.Fatalf("Enabled/GlobalKey must be untouched: %+v", f.config)
	}
	if len(f.config.Sources) != 2 || !f.config.Sources[0].Enabled {
		t.Fatalf("sources merge broken: %+v", f.config.Sources)
	}
	// 重复名不重复追加
	f.MergeConfig(other)
	if len(f.config.Sources) != 2 {
		t.Fatalf("duplicate source names must be skipped: %+v", f.config.Sources)
	}
}

func TestDefaultConfig_SafeDefaults(t *testing.T) {
	c := DefaultConfig()
	if c.Enabled {
		t.Fatal("threat intel must default to disabled (opt-in)")
	}
	if c.GlobalKey == "" || c.MaxIPs <= 0 {
		t.Fatalf("defaults incomplete: %+v", c)
	}
	for _, s := range c.Sources {
		if s.Enabled {
			t.Fatalf("source %s must default to disabled", s.Name)
		}
		if s.URL == "" || s.Format == "" || s.UpdateInterval <= 0 {
			t.Fatalf("source %s defaults incomplete: %+v", s.Name, s)
		}
	}
}

// 编译期保证 redis.Client 未被误用时接口可用（与 manager 模式一致的防御）
var _ = redis.NewClient

// fetchSource → storeIPs → IsThreatIP 全链（httptest 假源 + 本地 Redis）
func TestFetchSource_StoresAndDetects(t *testing.T) {
	client, err := redis.NewClient("localhost:6379", "", 0)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/404" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte("6.6.6.6\n8.8.8.8\n"))
	}))
	defer srv.Close()

	cfg := Config{Enabled: true, GlobalKey: "threatintel:test:global", MaxIPs: 100, Concurrency: 1}
	f := NewFetcher(cfg, client)

	f.fetchSource(Source{Name: "test-source", URL: srv.URL, Format: "text"})

	// 全局黑名单应包含两个 IP
	for _, ip := range []string{"6.6.6.6", "8.8.8.8"} {
		if !f.IsThreatIP(ip) {
			t.Errorf("IsThreatIP(%q) must be true after fetch", ip)
		}
	}
	if f.IsThreatIP("9.9.9.9") {
		t.Error("unlisted IP must not be flagged")
	}
	// 统计计数
	stats := f.GetStats()
	st, ok := stats["test-source"]
	if !ok || st.FetchCount != 1 || st.ErrorCount != 0 {
		t.Fatalf("stats broken: %+v", st)
	}
	// 失败源计数
	f.fetchSource(Source{Name: "bad-source", URL: srv.URL + "/404", Format: "text"})
	if st2 := f.GetStats()["bad-source"]; st2.ErrorCount != 1 {
		t.Fatalf("error count broken: %+v", st2)
	}
	// 清理
	client.Del(cfg.GlobalKey)
	client.Del("threatintel:source:test_source")
	client.Del("threatintel:source:bad_source")
}

func TestStoreIPs_NilRedisError(t *testing.T) {
	f := NewFetcher(Config{}, nil)
	if err := f.storeIPs("k", []string{"1.2.3.4"}); err == nil {
		t.Fatal("nil redis must error")
	}
}

// 调度循环模拟：Start + 短间隔 + httptest 源 → fetchAll 定时拉取 → Stop 收敛
func TestFetcher_Start_FetchAll_Stop(t *testing.T) {
	client, err := redis.NewClient("localhost:6379", "", 0)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("7.7.7.7\n"))
	}))
	defer srv.Close()

	cfg := Config{
		Enabled:     true,
		GlobalKey:   "threatintel:test:loop",
		MaxIPs:      10,
		Concurrency: 1,
		Sources: []Source{{
			Name: "loop-src", URL: srv.URL, Format: "text",
			UpdateInterval: 30 * time.Millisecond,
			Enabled:        true,
		}},
	}
	f := NewFetcher(cfg, client)
	f.Start()
	time.Sleep(150 * time.Millisecond) // 至少经历多个周期
	f.Stop()

	// 循环已把源 IP 写入全局黑名单
	if !f.IsThreatIP("7.7.7.7") {
		t.Fatal("scheduled fetch must populate global blacklist")
	}
	if st := f.GetStats()["loop-src"]; st == nil || st.FetchCount == 0 {
		t.Fatalf("scheduled source stats missing: %+v", st)
	}
	client.Del(cfg.GlobalKey)
	client.Del("threatintel:source:loop_src")
}

// Start 空转：无启用源时循环安全
func TestFetcher_Start_NoSources(t *testing.T) {
	client, err := redis.NewClient("localhost:6379", "", 0)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	f := NewFetcher(Config{Enabled: true, GlobalKey: "threatintel:test:empty", Sources: nil}, client)
	f.Start()
	time.Sleep(30 * time.Millisecond)
	f.Stop()
}
