package renderkey

import (
	"net/url"
	"sort"
	"strings"
)

// Normalize 归一化 URL 作为渲染缓存键的规范形态：
// - 去除 fragment（hash 路由片段对 HTTP 服务端无意义，保留会污染缓存键）
// - host 转小写；省略 scheme 差异（同一资源 http/https 共用缓存）
// - path 保持转义原样
// - query 参数按名称排序，消除参数顺序差异导致的重复键
func Normalize(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "/"
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	if u.Host != "" {
		u.Host = strings.ToLower(u.Host)
	}
	u.Scheme = ""
	u.Fragment = ""
	u.RawFragment = ""

	if len(u.Query()) > 0 {
		keys := make([]string, 0, len(u.Query()))
		for k := range u.Query() {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		b.WriteByte('?')
		for i, k := range keys {
			vs := u.Query()[k]
			sort.Strings(vs)
			for _, v := range vs {
				if i > 0 || b.Len() > 1 {
					b.WriteByte('&')
				}
				b.WriteString(k)
				if v != "" {
					b.WriteByte('=')
					b.WriteString(v)
				}
			}
		}
		u.RawQuery = b.String()[1:]
	} else {
		u.RawQuery = ""
	}

	norm := u.String()
	if u.Host != "" {
		// scheme 已清空，u.String() 产生 "//host/path?query" 形态
		return strings.TrimPrefix(norm, "//")
	}
	return strings.TrimPrefix(norm, "/")
}

// FromPath 补全 host 构造完整 URL。preheat 的 sitemap/crawler 来源常是纯 route 形态。
func FromPath(scheme, host, pathWithQuery string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	scheme = strings.TrimSpace(scheme)
	if scheme == "" {
		scheme = "http"
	}
	if !strings.HasPrefix(pathWithQuery, "/") {
		pathWithQuery = "/" + pathWithQuery
	}
	if host == "" {
		return pathWithQuery
	}
	return scheme + "://" + host + pathWithQuery
}

// BuildCacheKey 渲染缓存的业务键（不含 cache:<site>: 外层前缀，该前缀由 cache.Manager 统一拼装）。
func BuildCacheKey(normalizedURL string) string {
	return "prerender:" + normalizedURL
}

// mobileUAKeywords 移动设备 UA 关键词（小写匹配）
var mobileUAKeywords = []string{
	"mobile", "android", "iphone", "ipad", "ipod",
	"opera mini", "iemobile", "blackberry", "webos",
}

// DeviceBucket 按 UA 判定设备分桶：mobile / desktop。
// 空 UA 或无法识别一律 desktop（预热通道传空 UA，落在 desktop 桶；
// 移动版爬虫未命中 desktop 键时走实时渲染补齐 mobile 桶）。
func DeviceBucket(userAgent string) string {
	ua := strings.ToLower(userAgent)
	if ua == "" {
		return "desktop"
	}
	for _, kw := range mobileUAKeywords {
		if strings.Contains(ua, kw) {
			return "mobile"
		}
	}
	return "desktop"
}

// WithDeviceBucket 在归一化 URL 业务键尾追加设备桶后缀。
// desktop 加后缀（键形态统一），读侧对无后缀旧键做一次性回退兼容。
func WithDeviceBucket(normalizedURL, device string) string {
	if device == "mobile" {
		return "prerender:" + normalizedURL + "@mobile"
	}
	return "prerender:" + normalizedURL + "@desktop"
}

// NormalizeFlexible 兼容两种输入形态的键归一化：
//   - 完整 URL（含 ://）：走 Normalize；
//   - 归一化展示形态（管理端 entries 列表回传，如 "host:port/path"）：
//     Normalize 会把 host 误判为 scheme（url.Parse("localhost:29901/") → "29901/"），
//     故仅对含 "://" 的输入归一化，其余原样返回。
func NormalizeFlexible(u string) string {
	if strings.Contains(u, "://") {
		return Normalize(u)
	}
	return strings.TrimSpace(u)
}

// ToAbsolute 把归一化形态补全为可导航的绝对 URL（重渲通道用）。
// 归一化键丢弃 scheme，补全统一按 defaultScheme（默认 http）；
// scheme 与缓存命中无关（Normalize 本就抹平 http/https 差异）。
func ToAbsolute(u, defaultScheme string) string {
	u = strings.TrimSpace(u)
	if u == "" || strings.Contains(u, "://") {
		return u
	}
	if defaultScheme == "" {
		defaultScheme = "http"
	}
	if strings.HasPrefix(u, "/") {
		// 仅路径形态无 host，无法补全，交由调用方兜底
		return u
	}
	return defaultScheme + "://" + u
}

// StripBizKey 还原业务键为展示用归一化 URL（去 "prerender:" 前缀与 "@desktop/@mobile" 后缀）。
// 返回 (归一化URL, 设备桶)。管理端缓存条目列表展示用；
// 还原后的 URL 传回 InvalidatePage 时经 Normalize 幂等还原，删除键保持一致。
func StripBizKey(bizKey string) (normalizedURL, device string) {
	norm := strings.TrimPrefix(bizKey, "prerender:")
	device = "desktop"
	if strings.HasSuffix(norm, "@mobile") {
		device = "mobile"
		norm = strings.TrimSuffix(norm, "@mobile")
	} else if strings.HasSuffix(norm, "@desktop") {
		norm = strings.TrimSuffix(norm, "@desktop")
	}
	return norm, device
}
