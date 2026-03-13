package ai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExtractURLFeatures 测试 URL 特征提取
func TestExtractURLFeatures(t *testing.T) {
	// 测试 nil URL
	features := ExtractURLFeatures(nil)
	assert.Len(t, features, 20)

	// 测试正常 URL
	u, _ := url.Parse("http://example.com/path/to/page?id=123&name=test")
	features = ExtractURLFeatures(u)
	assert.Len(t, features, 20)
	assert.Greater(t, features[0], float32(0)) // URL 长度
	assert.Greater(t, features[1], float32(0)) // 路径深度
	assert.Greater(t, features[2], float32(0)) // 查询参数
}

// TestExtractURLFeatures_SpecialChars 测试特殊字符检测
func TestExtractURLFeatures_SpecialChars(t *testing.T) {
	u, _ := url.Parse("http://example.com/path%20with%20spaces?query=<script>")
	features := ExtractURLFeatures(u)
	assert.Len(t, features, 20)
	assert.Equal(t, float32(1.0), features[4]) // 有编码字符
}

// TestExtractURLFeatures_SuspiciousKeywords 测试可疑关键词
func TestExtractURLFeatures_SuspiciousKeywords(t *testing.T) {
	u, _ := url.Parse("http://example.com/admin/login?password=secret")
	features := ExtractURLFeatures(u)
	assert.Len(t, features, 20)
	assert.Greater(t, features[5], float32(0)) // 有可疑关键词
}

// TestExtractURLFeatures_IPPattern 测试 IP 地址模式
func TestExtractURLFeatures_IPPattern(t *testing.T) {
	u, _ := url.Parse("http://192.168.1.1/admin")
	features := ExtractURLFeatures(u)
	assert.Len(t, features, 20)
	assert.Equal(t, float32(1.0), features[8]) // 有 IP 地址
}

// TestExtractHeaderFeatures 测试 Header 特征提取
func TestExtractHeaderFeatures(t *testing.T) {
	// 测试 nil Header
	features := ExtractHeaderFeatures(nil)
	assert.Len(t, features, 30)

	// 测试正常 Header
	header := http.Header{
		"User-Agent":      []string{"Mozilla/5.0"},
		"Referer":         []string{"http://example.com"},
		"Accept":          []string{"text/html"},
		"Content-Type":    []string{"application/json"},
		"Authorization":   []string{"Bearer token123"},
		"X-Forwarded-For": []string{"1.2.3.4"},
	}
	features = ExtractHeaderFeatures(header)
	assert.Len(t, features, 30)
	assert.Greater(t, features[0], float32(0)) // Header 数量
	assert.Equal(t, float32(1.0), features[1]) // User-Agent 存在
	assert.Equal(t, float32(1.0), features[2]) // Referer 存在
}

// TestExtractHeaderFeatures_SuspiciousUA 测试可疑 User-Agent
func TestExtractHeaderFeatures_SuspiciousUA(t *testing.T) {
	header := http.Header{
		"User-Agent": []string{"sqlmap/1.0"},
	}
	features := ExtractHeaderFeatures(header)
	assert.Equal(t, float32(1.0), features[12]) // 可疑 UA
}

// TestExtractHeaderFeatures_SuspiciousHeaders 测试可疑 Header
func TestExtractHeaderFeatures_SuspiciousHeaders(t *testing.T) {
	header := http.Header{}
	header.Set("X-Original-URL", "/admin")
	header.Set("X-Rewrite-URL", "/config")

	features := ExtractHeaderFeatures(header)
	assert.Len(t, features, 30)
	// 检查可疑 Header 检测 (indices 21-25)
	// X-Original-URL is at index 21, X-Rewrite-URL is at index 22
	assert.Equal(t, float32(1.0), features[21])
	assert.Equal(t, float32(1.0), features[22])
}

// TestExtractBodyFeatures 测试 Body 特征提取
func TestExtractBodyFeatures(t *testing.T) {
	// 测试 nil Body
	features, err := ExtractBodyFeatures(nil)
	assert.NoError(t, err)
	assert.Len(t, features, 30)

	// 测试空 Body
	features, err = ExtractBodyFeatures(http.NoBody)
	assert.NoError(t, err)
	assert.Len(t, features, 30)

	// 测试包含 SQL 注入特征的 Body
	sqlBody := "SELECT * FROM users WHERE id=1 OR '1'='1'"
	features, err = ExtractBodyFeatures(io.NopCloser(strings.NewReader(sqlBody)))
	assert.NoError(t, err)
	assert.Len(t, features, 30)
	assert.Equal(t, float32(1.0), features[6]) // SQL 关键词
}

// TestExtractBodyFeatures_XSS 测试 XSS 特征
func TestExtractBodyFeatures_XSS(t *testing.T) {
	xssBody := `<script>alert('xss')</script>`
	features, err := ExtractBodyFeatures(io.NopCloser(strings.NewReader(xssBody)))
	assert.NoError(t, err)
	assert.Len(t, features, 30)
	assert.Equal(t, float32(1.0), features[11]) // <script>标签
}

// TestExtractBodyFeatures_PathTraversal 测试路径遍历特征
func TestExtractBodyFeatures_PathTraversal(t *testing.T) {
	ptBody := "../../../etc/passwd"
	features, err := ExtractBodyFeatures(io.NopCloser(strings.NewReader(ptBody)))
	assert.NoError(t, err)
	assert.Len(t, features, 30)
	assert.Equal(t, float32(1.0), features[16]) // ../
}

// TestExtractBodyFeatures_CommandInjection 测试命令注入特征
func TestExtractBodyFeatures_CommandInjection(t *testing.T) {
	cmdBody := "127.0.0.1; cat /etc/passwd"
	features, err := ExtractBodyFeatures(io.NopCloser(strings.NewReader(cmdBody)))
	assert.NoError(t, err)
	assert.Len(t, features, 30)
	assert.Greater(t, features[25], float32(0)) // 命令关键词
}

// TestExtractBehaviorFeatures 测试行为特征提取
func TestExtractBehaviorFeatures(t *testing.T) {
	// 测试 nil Request
	features := ExtractBehaviorFeatures(nil)
	assert.Len(t, features, 48)

	// 测试正常 GET 请求
	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/users", nil)
	features = ExtractBehaviorFeatures(req)
	assert.Len(t, features, 48)
	assert.Equal(t, float32(1.0), features[0]) // GET 方法

	// 测试 POST 请求
	req = httptest.NewRequest(http.MethodPost, "http://example.com/api/users", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	features = ExtractBehaviorFeatures(req)
	assert.Equal(t, float32(1.0), features[1]) // POST 方法
	assert.Equal(t, float32(1.0), features[8]) // 有 Body
}

// TestExtractBehaviorFeatures_HTTPS 测试 HTTPS 特征
func TestExtractBehaviorFeatures_HTTPS(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://example.com/secure", nil)
	features := ExtractBehaviorFeatures(req)
	assert.Equal(t, float32(1.0), features[7]) // HTTPS
}

// TestExtractBehaviorFeatures_AdminPath 测试管理员路径
func TestExtractBehaviorFeatures_AdminPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/admin/dashboard", nil)
	features := ExtractBehaviorFeatures(req)
	assert.Equal(t, float32(1.0), features[29]) // Admin 路径
}

// TestExtractBehaviorFeatures_SensitivePath 测试敏感路径
func TestExtractBehaviorFeatures_SensitivePath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/config", nil)
	features := ExtractBehaviorFeatures(req)
	assert.Equal(t, float32(1.0), features[30]) // 敏感路径
}

// TestNormalizeFeatures 测试特征规范化
func TestNormalizeFeatures_DifferentSizes(t *testing.T) {
	// 测试从小到大的规范化
	small := make([]float32, 10)
	result := NormalizeFeatures(small, 20)
	assert.Len(t, result, 20)
	assert.Equal(t, small, result[:10])
	assert.Equal(t, make([]float32, 10), result[10:])

	// 测试从大到小的规范化
	large := make([]float32, 30)
	for i := range large {
		large[i] = float32(i)
	}
	result = NormalizeFeatures(large, 15)
	assert.Len(t, result, 15)
	assert.Equal(t, large[:15], result)
}

// TestNormalize 测试 normalizeLength 函数
func TestNormalize(t *testing.T) {
	assert.Equal(t, float32(0.5), normalizeLength(50, 100))
	assert.Equal(t, float32(1.0), normalizeLength(100, 100))
	assert.Equal(t, float32(1.0), normalizeLength(200, 100))
	assert.Equal(t, float32(0.0), normalizeLength(0, 100))
}

// TestBoolToFloat 测试 boolToFloat 函数
func TestBoolToFloat(t *testing.T) {
	assert.Equal(t, float32(1.0), boolToFloat(true))
	assert.Equal(t, float32(0.0), boolToFloat(false))
}

// TestHasSpecialChars 测试 hasSpecialChars 函数
func TestHasSpecialChars(t *testing.T) {
	assert.Greater(t, hasSpecialChars("'; DROP TABLE users;--"), float32(0))
	assert.Greater(t, hasSpecialChars("test<string>"), float32(0))
	assert.Equal(t, float32(0.0), hasSpecialChars("normal text"))
}

// TestHasEncodedChars 测试 hasEncodedChars 函数
func TestHasEncodedChars(t *testing.T) {
	assert.Equal(t, float32(1.0), hasEncodedChars("test%20value"))
	assert.Equal(t, float32(1.0), hasEncodedChars("test&amp;value"))
	// Base64 pattern needs at least 20 chars
	assert.Equal(t, float32(1.0), hasEncodedChars("SGVsbG8gV29ybGQhISBUaGlzIGlzIGEgdGVzdA=="))
	assert.Equal(t, float32(0.0), hasEncodedChars("normal text"))
}

// TestHasSuspiciousKeywords 测试 hasSuspiciousKeywords 函数
func TestHasSuspiciousKeywords(t *testing.T) {
	assert.Equal(t, float32(1.0), hasSuspiciousKeywords("/admin/login/password/secret/token"))
	assert.Greater(t, hasSuspiciousKeywords("/api/password/reset"), float32(0))
	assert.Equal(t, float32(0.0), hasSuspiciousKeywords("/normal/path"))
}

// TestHasDigits 测试 hasDigits 函数
func TestHasDigits(t *testing.T) {
	assert.Equal(t, float32(1.0), hasDigits("test123"))
	assert.Equal(t, float32(0.0), hasDigits("nodigits"))
}

// TestHasIPPattern 测试 hasIPPattern 函数
func TestHasIPPattern(t *testing.T) {
	assert.Equal(t, float32(1.0), hasIPPattern("http://192.168.1.1/admin"))
	assert.Equal(t, float32(0.0), hasIPPattern("http://example.com/admin"))
}

// TestHasFileExtension 测试 hasFileExtension 函数
func TestHasFileExtension(t *testing.T) {
	assert.True(t, hasFileExtension("/path/to/file.html"))
	assert.True(t, hasFileExtension("/path/to/file.php"))
	assert.True(t, hasFileExtension("/path/to/file.json"))
	assert.False(t, hasFileExtension("/path/to/file"))
	assert.False(t, hasFileExtension("/path/to/file.unknown"))
}

// TestIsSuspiciousUA 测试 isSuspiciousUA 函数
func TestIsSuspiciousUA(t *testing.T) {
	assert.Equal(t, float32(1.0), isSuspiciousUA("sqlmap/1.0"))
	assert.Equal(t, float32(1.0), isSuspiciousUA("Mozilla/5.0 (compatible; Nikto)"))
	assert.Equal(t, float32(1.0), isSuspiciousUA("curl/7.68.0 (bot)"))
	assert.Equal(t, float32(0.0), isSuspiciousUA("Mozilla/5.0 (Windows NT 10.0; Win64; x64)"))
}

// TestHasSQLKeywords 测试 hasSQLKeywords 函数
func TestHasSQLKeywords(t *testing.T) {
	assert.Equal(t, float32(1.0), hasSQLKeywords("SELECT * FROM users WHERE DELETE DROP"))
	assert.Greater(t, hasSQLKeywords("SELECT id, name"), float32(0))
}

// TestHasSQLPatterns 测试 hasSQLPatterns 函数
func TestHasSQLPatterns(t *testing.T) {
	assert.Equal(t, float32(1.0), hasSQLPatterns("OR 1=1"))
	assert.Equal(t, float32(1.0), hasSQLPatterns("' OR '1'='1"))
	assert.Equal(t, float32(1.0), hasSQLPatterns("test--"))
	assert.Equal(t, float32(1.0), hasSQLPatterns("test/* comment */"))
	assert.Equal(t, float32(0.0), hasSQLPatterns("normal text"))
}

// TestHasCommandKeywords 测试 hasCommandKeywords 函数
func TestHasCommandKeywords(t *testing.T) {
	assert.Equal(t, float32(1.0), hasCommandKeywords("cat /etc/passwd bash shell"))
	assert.Greater(t, hasCommandKeywords("curl http://example.com"), float32(0))
	assert.Equal(t, float32(0.0), hasCommandKeywords("normal text"))
}

// TestIsWebRequest 测试 isWebRequest 函数
func TestIsWebRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	assert.True(t, isWebRequest(req))

	req.Header.Set("Accept", "application/json")
	assert.False(t, isWebRequest(req))
}

// TestIsAPIRequest 测试 isAPIRequest 函数
func TestIsAPIRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/api", nil)
	req.Header.Set("Content-Type", "application/json")
	assert.True(t, isAPIRequest(req))

	req.Header.Del("Content-Type")
	req.Header.Set("Accept", "application/json")
	assert.True(t, isAPIRequest(req))

	req.Header.Del("Accept")
	assert.False(t, isAPIRequest(req))
}

// TestIsStaticResource 测试 isStaticResource 函数
func TestIsStaticResource(t *testing.T) {
	staticPaths := []string{
		"/static/style.css",
		"/js/app.js",
		"/images/logo.png",
		"/favicon.ico",
		"/fonts/roboto.woff2",
	}
	for _, path := range staticPaths {
		req := httptest.NewRequest(http.MethodGet, "http://example.com"+path, nil)
		assert.True(t, isStaticResource(req), "path: %s", path)
	}

	nonStaticPaths := []string{
		"/api/users",
		"/admin/dashboard",
		"/page.html",
	}
	for _, path := range nonStaticPaths {
		req := httptest.NewRequest(http.MethodGet, "http://example.com"+path, nil)
		assert.False(t, isStaticResource(req), "path: %s", path)
	}
}

// TestIsAdminPath 测试 isAdminPath 函数
func TestIsAdminPath(t *testing.T) {
	adminPaths := []string{
		"/admin",
		"/admin/users",
		"/administrator",
		"/manage/dashboard",
		"/backend/config",
		"/console",
	}
	for _, path := range adminPaths {
		assert.True(t, isAdminPath(path), "path: %s", path)
	}

	nonAdminPaths := []string{
		"/user/profile",
		"/public",
	}
	for _, path := range nonAdminPaths {
		assert.False(t, isAdminPath(path), "path: %s", path)
	}
}

// TestIsSensitivePath 测试 isSensitivePath 函数
func TestIsSensitivePath(t *testing.T) {
	sensitivePaths := []string{
		"/api/users",
		"/config/settings",
		"/user/profile",
		"/account/settings",
		"/password/reset",
		"/login",
	}
	for _, path := range sensitivePaths {
		assert.True(t, isSensitivePath(path), "path: %s", path)
	}

	nonSensitivePaths := []string{
		"/public",
		"/about",
		"/contact",
	}
	for _, path := range nonSensitivePaths {
		assert.False(t, isSensitivePath(path), "path: %s", path)
	}
}

// TestHasDebugParams 测试 hasDebugParams 函数
func TestHasDebugParams(t *testing.T) {
	debugQueries := []string{
		"?debug=true",
		"?test=1",
		"?dev=1",
		"?trace=full",
	}
	for _, query := range debugQueries {
		u, _ := url.Parse("http://example.com" + query)
		assert.True(t, hasDebugParams(u.Query()), "query: %s", query)
	}

	nonDebugQueries := []string{
		"?name=test",
		"?id=123",
	}
	for _, query := range nonDebugQueries {
		u, _ := url.Parse("http://example.com" + query)
		assert.False(t, hasDebugParams(u.Query()), "query: %s", query)
	}
}

// TestHasAdminParams 测试 hasAdminParams 函数
func TestHasAdminParams(t *testing.T) {
	adminQueries := []string{
		"?admin=true",
		"?root=1",
		"?super=user",
		"?sudo=yes",
	}
	for _, query := range adminQueries {
		u, _ := url.Parse("http://example.com" + query)
		assert.True(t, hasAdminParams(u.Query()), "query: %s", query)
	}

	nonAdminQueries := []string{
		"?name=admin_user", // 包含 admin 但在值中
		"?id=123",
	}
	for _, query := range nonAdminQueries {
		u, _ := url.Parse("http://example.com" + query)
		// 注意：hasAdminParams 检查参数名而非参数值
		assert.False(t, hasAdminParams(u.Query()), "query: %s", query)
	}
}

// TestHasSensitiveParams 测试 hasSensitiveParams 函数
func TestHasSensitiveParams(t *testing.T) {
	sensitiveQueries := []string{
		"?password=secret",
		"?token=abc123",
		"?key=apikey",
		"?auth=bearer",
	}
	for _, query := range sensitiveQueries {
		u, _ := url.Parse("http://example.com" + query)
		assert.True(t, hasSensitiveParams(u.Query()), "query: %s", query)
	}

	nonSensitiveQueries := []string{
		"?name=test",
		"?id=123",
	}
	for _, query := range nonSensitiveQueries {
		u, _ := url.Parse("http://example.com" + query)
		assert.False(t, hasSensitiveParams(u.Query()), "query: %s", query)
	}
}

// TestCountSpecialCharsInHeaders 测试 countSpecialCharsInHeaders 函数
func TestCountSpecialCharsInHeaders(t *testing.T) {
	header := http.Header{
		"X-Custom": []string{"'; DROP TABLE users;--"},
	}
	result := countSpecialCharsInHeaders(header)
	assert.Greater(t, result, float32(0))

	normalHeader := http.Header{
		"X-Custom": []string{"normal value"},
	}
	result = countSpecialCharsInHeaders(normalHeader)
	assert.Equal(t, float32(0), result)
}

// TestCountEncodedCharsInHeaders 测试 countEncodedCharsInHeaders 函数
func TestCountEncodedCharsInHeaders(t *testing.T) {
	header := http.Header{
		"X-Custom": []string{"test%20value%20encoded%20more%20and%20more"},
	}
	result := countEncodedCharsInHeaders(header)
	assert.Greater(t, result, float32(0))

	normalHeader := http.Header{
		"X-Custom": []string{"normal value"},
	}
	result = countEncodedCharsInHeaders(normalHeader)
	assert.Equal(t, float32(0), result)
}

// TestHasSuspiciousKeywordsInHeaders 测试 hasSuspiciousKeywordsInHeaders 函数
func TestHasSuspiciousKeywordsInHeaders(t *testing.T) {
	header := http.Header{
		"X-Custom": []string{"admin password secret"},
	}
	result := hasSuspiciousKeywordsInHeaders(header)
	assert.Equal(t, float32(1.0), result)

	normalHeader := http.Header{
		"X-Custom": []string{"normal value"},
	}
	result = hasSuspiciousKeywordsInHeaders(normalHeader)
	assert.Equal(t, float32(0), result)
}

// TestHasSQLKeywordsInHeaders 测试 hasSQLKeywordsInHeaders 函数
func TestHasSQLKeywordsInHeaders(t *testing.T) {
	header := http.Header{
		"X-Custom": []string{"SELECT * FROM users"},
	}
	result := hasSQLKeywordsInHeaders(header)
	assert.Equal(t, float32(1.0), result)

	normalHeader := http.Header{
		"X-Custom": []string{"normal value"},
	}
	result = hasSQLKeywordsInHeaders(normalHeader)
	assert.LessOrEqual(t, result, float32(1.0))
}

// TestAvgSlice_EdgeCases 测试 avgSlice 边界情况
func TestAvgSlice_EdgeCases(t *testing.T) {
	// 空切片
	assert.Equal(t, float32(0), avgSlice([]float32{}))

	// 单个元素
	assert.Equal(t, float32(5), avgSlice([]float32{5}))

	// 多个元素
	assert.Equal(t, float32(3), avgSlice([]float32{1, 2, 3, 4, 5}))

	// 负数
	assert.Equal(t, float32(0), avgSlice([]float32{-1, 0, 1}))
}
