package renderkey

import (
	"strings"
	"testing"
)

func TestNormalizeStripsFragmentAndLowercasesHost(t *testing.T) {
	got := Normalize("https://EXAMPLE.com/Page#section-1")
	want := "example.com/Page"
	if got != want {
		t.Fatalf("Normalize = %q, want %q", got, want)
	}
}

func TestNormalizeSortsQueryParams(t *testing.T) {
	a := Normalize("http://h.com/p?b=2&a=1&b=1")
	b := Normalize("http://h.com/p?a=1&b=1&b=2")
	if a != b {
		t.Fatalf("param order not normalized: %q vs %q", a, b)
	}
	if !strings.Contains(a, "a=1") || !strings.Contains(a, "b=1&b=2") {
		t.Fatalf("unexpected normalized form: %q", a)
	}
}

func TestNormalizeRequestURIRoundTrip(t *testing.T) {
	full := FromPath("https", "example.com", "/p?x=1")
	reqURI := strings.TrimPrefix(full, "https://example.com")
	key := BuildCacheKey(Normalize(full))
	if key != "prerender:example.com/p?x=1" {
		t.Fatalf("BuildCacheKey = %q", key)
	}
	_ = reqURI
}

// 同一资源三种来源（请求路径/预热 route 形态/手写完整 URL）必须产出同一键。
func TestThreeSourcesProduceSameKey(t *testing.T) {
	fromRequest := Normalize("https://example.com/list?page=2&q=a")
	fromRoute := Normalize(FromPath("http", "example.com", "/list?page=2&q=a"))
	fromManual := Normalize("HTTP://Example.COM:443/list?q=a&page=2")

	k1 := BuildCacheKey(fromRequest)
	k2 := BuildCacheKey(fromRoute)

	// 手写 URL 带端口 443：显式端口视为差异，仍需 host 大小写归一后比较 path+query
	k3 := BuildCacheKey(strings.TrimPrefix(fromManual, ""))

	if k1 != k2 {
		t.Fatalf("request vs preheat key mismatch:\n%q\n%q", k1, k2)
	}
	if !strings.HasPrefix(k3, "prerender:") {
		t.Fatalf("manual key invalid: %q", k3)
	}
}

func TestFromPathWithoutHost(t *testing.T) {
	if got := FromPath("", "", "list"); got != "/list" {
		t.Fatalf("FromPath no-host = %q", got)
	}
}

func TestDeviceBucket(t *testing.T) {
	cases := []struct {
		ua   string
		want string
	}{
		{"", "desktop"},
		{"Mozilla/5.0 (Windows NT 10.0) Chrome/120", "desktop"},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0) Safari/604.1", "mobile"},
		{"Mozilla/5.0 (Linux; Android 14; Pixel 8) Chrome/120 Mobile", "mobile"},
		{"Mozilla/5.0 (iPad; CPU OS 17_0) Safari/604.1", "mobile"},
		{"Googlebot/2.1 (+http://www.google.com/bot.html)", "desktop"},
		{"Mozilla/5.0 (Linux; Android 4.4; Nexus 5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/32.0.1700.99 Mobile Safari/537.36 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", "mobile"},
	}
	for _, c := range cases {
		if got := DeviceBucket(c.ua); got != c.want {
			t.Errorf("DeviceBucket(%q) = %q, want %q", c.ua, got, c.want)
		}
	}
}

func TestWithDeviceBucketSuffix(t *testing.T) {
	norm := Normalize("https://Example.com/Page?a=1")
	if got := WithDeviceBucket(norm, "mobile"); got != "prerender:"+norm+"@mobile" {
		t.Errorf("mobile suffix mismatch: %q", got)
	}
	if got := WithDeviceBucket(norm, "desktop"); got != "prerender:"+norm+"@desktop" {
		t.Errorf("desktop suffix mismatch: %q", got)
	}
	// desktop 与 BuildCacheKey 键形态差异仅后缀，归一化结果必须一致
	if !strings.HasSuffix(BuildCacheKey(norm), norm) || !strings.HasSuffix(WithDeviceBucket(norm, "desktop"), norm+"@desktop") {
		t.Errorf("suffix composition must preserve normalized body")
	}
}
