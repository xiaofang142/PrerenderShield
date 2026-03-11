package ai

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// FeatureCache 特征缓存
type FeatureCache struct {
	data    map[string]*cacheItem
	mu      sync.RWMutex
	maxSize int
	ttl     time.Duration
}

type cacheItem struct {
	features  []float32
	expiresAt time.Time
}

// NewFeatureCache 创建特征缓存
func NewFeatureCache(maxSize int, ttl time.Duration) *FeatureCache {
	return &FeatureCache{
		data:    make(map[string]*cacheItem),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get 获取缓存
func (c *FeatureCache) Get(key string) ([]float32, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.data[key]
	if !exists || time.Now().After(item.expiresAt) {
		return nil, false
	}

	return item.features, true
}

// Set 设置缓存
func (c *FeatureCache) Set(key string, features []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// LRU淘汰
	if len(c.data) >= c.maxSize {
		c.evictLRU()
	}

	c.data[key] = &cacheItem{
		features:  features,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// evictLRU 淘汰最近最少使用
func (c *FeatureCache) evictLRU() {
	// 简单实现：删除最早过期的项
	var oldestKey string
	oldestTime := time.Now().Add(24 * time.Hour) // 最大可能的过期时间

	for key, item := range c.data {
		if item.expiresAt.Before(oldestTime) {
			oldestTime = item.expiresAt
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(c.data, oldestKey)
	}
}

// Cleanup 清理过期缓存
func (c *FeatureCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, item := range c.data {
		if now.After(item.expiresAt) {
			delete(c.data, key)
		}
	}
}

// Clear 清空缓存
func (c *FeatureCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]*cacheItem)
}

// Size 获取缓存大小
func (c *FeatureCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}

// ExtractURLFeatures 提取URL特征
func ExtractURLFeatures(u *url.URL) []float32 {
	features := make([]float32, 20)

	if u == nil {
		return features
	}

	// 0: URL长度归一化
	urlLen := len(u.String())
	features[0] = normalizeLength(urlLen, 2000)

	// 1: 路径深度
	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
	features[1] = float32(len(pathParts)) / 10.0

	// 2: 查询参数数量
	query := u.Query()
	features[2] = float32(len(query)) / 20.0

	// 3: 是否有特殊字符
	features[3] = hasSpecialChars(u.String())

	// 4: 是否有编码字符
	features[4] = hasEncodedChars(u.String())

	// 5: 是否有可疑关键词
	features[5] = hasSuspiciousKeywords(u.String())

	// 6: 是否有文件扩展名
	features[6] = boolToFloat(hasFileExtension(u.Path))

	// 7: 是否有数字
	features[7] = hasDigits(u.String())

	// 8: 是否有IP地址格式
	features[8] = hasIPPattern(u.String())

	// 9: 域名长度归一化
	features[9] = normalizeLength(len(u.Hostname()), 100)

	// 10-19: URL模式特征
	features[10] = float32(strings.Count(u.String(), "/")) / 20.0
	features[11] = float32(strings.Count(u.String(), "?")) / 5.0
	features[12] = float32(strings.Count(u.String(), "&")) / 20.0
	features[13] = float32(strings.Count(u.String(), "=")) / 20.0
	features[14] = float32(strings.Count(u.String(), "%")) / 20.0
	features[15] = float32(strings.Count(u.String(), ".")) / 10.0
	features[16] = float32(strings.Count(u.String(), "-")) / 10.0
	features[17] = float32(strings.Count(u.String(), "_")) / 10.0
	features[18] = boolToFloat(strings.Contains(u.String(), ".."))
	features[19] = boolToFloat(strings.Contains(u.String(), "//"))

	return features
}

// ExtractHeaderFeatures 提取Header特征
func ExtractHeaderFeatures(header http.Header) []float32 {
	features := make([]float32, 30)

	if header == nil {
		return features
	}

	// 0: Header数量归一化
	features[0] = float32(len(header)) / 30.0

	// 1-10: 常见Header存在性
	commonHeaders := []string{
		"User-Agent",
		"Referer",
		"Accept",
		"Accept-Language",
		"Accept-Encoding",
		"Cookie",
		"Authorization",
		"X-Forwarded-For",
		"X-Real-IP",
		"Content-Type",
	}

	for i, h := range commonHeaders {
		features[i+1] = boolToFloat(header.Get(h) != "")
	}

	// 11: User-Agent长度
	ua := header.Get("User-Agent")
	features[11] = normalizeLength(len(ua), 500)

	// 12: User-Agent是否可疑
	features[12] = isSuspiciousUA(ua)

	// 13: Referer是否存在
	referer := header.Get("Referer")
	features[13] = boolToFloat(referer != "")

	// 14: Referer长度
	features[14] = normalizeLength(len(referer), 500)

	// 15: Cookie数量
	cookies := header.Get("Cookie")
	cookieCount := len(strings.Split(cookies, ";"))
	features[15] = float32(cookieCount) / 20.0

	// 16: Cookie长度
	features[16] = normalizeLength(len(cookies), 4000)

	// 17: 是否有Authorization头
	features[17] = boolToFloat(header.Get("Authorization") != "")

	// 18: Content-Type是否JSON
	contentType := header.Get("Content-Type")
	features[18] = boolToFloat(strings.Contains(contentType, "application/json"))

	// 19: Content-Type是否表单
	features[19] = boolToFloat(strings.Contains(contentType, "application/x-www-form-urlencoded"))

	// 20: Content-Type是否multipart
	features[20] = boolToFloat(strings.Contains(contentType, "multipart/form-data"))

	// 21-25: 可疑Header检测
	suspiciousHeaders := []string{
		"X-Original-URL",
		"X-Rewrite-URL",
		"X-Custom-IP-Authorization",
		"X-Forwarded-Host",
		"X-Host",
	}

	for i, h := range suspiciousHeaders {
		features[i+21] = boolToFloat(header.Get(h) != "")
	}

	// 26-29: Header值特征
	features[26] = countSpecialCharsInHeaders(header)
	features[27] = countEncodedCharsInHeaders(header)
	features[28] = hasSuspiciousKeywordsInHeaders(header)
	features[29] = hasSQLKeywordsInHeaders(header)

	return features
}

// ExtractBodyFeatures 提取Body特征
func ExtractBodyFeatures(body io.ReadCloser) ([]float32, error) {
	features := make([]float32, 30)

	if body == nil {
		return features, nil
	}

	// 读取body内容
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return features, err
	}

	// 恢复body供后续使用
	body = io.NopCloser(bytes.NewReader(bodyBytes))

	bodyStr := string(bodyBytes)

	// 0: Body长度归一化
	features[0] = normalizeLength(len(bodyStr), 10000)

	// 1-5: JSON/XML/HTML/Form特征
	features[1] = boolToFloat(strings.Contains(bodyStr, "{") && strings.Contains(bodyStr, "}"))
	features[2] = boolToFloat(strings.Contains(bodyStr, "<?xml"))
	features[3] = boolToFloat(strings.Contains(bodyStr, "<!DOCTYPE") || strings.Contains(bodyStr, "<html"))
	features[4] = boolToFloat(strings.Contains(bodyStr, "=") && strings.Contains(bodyStr, "&"))
	features[5] = boolToFloat(strings.HasPrefix(strings.TrimSpace(bodyStr), "--"))

	// 6-10: SQL注入特征
	features[6] = hasSQLKeywords(bodyStr)
	features[7] = hasSQLPatterns(bodyStr)
	features[8] = float32(strings.Count(bodyStr, "'")) / 20.0
	features[9] = float32(strings.Count(bodyStr, "\"")) / 20.0
	features[10] = float32(strings.Count(bodyStr, ";")) / 20.0

	// 11-15: XSS特征
	features[11] = boolToFloat(strings.Contains(bodyStr, "<script"))
	features[12] = boolToFloat(strings.Contains(bodyStr, "javascript:"))
	features[13] = boolToFloat(strings.Contains(bodyStr, "onerror="))
	features[14] = boolToFloat(strings.Contains(bodyStr, "onload="))
	features[15] = boolToFloat(strings.Contains(bodyStr, "eval("))

	// 16-20: 路径遍历特征
	features[16] = boolToFloat(strings.Contains(bodyStr, "../"))
	features[17] = boolToFloat(strings.Contains(bodyStr, "..\\"))
	features[18] = boolToFloat(strings.Contains(bodyStr, "/etc/"))
	features[19] = boolToFloat(strings.Contains(bodyStr, "c:\\"))
	features[20] = boolToFloat(strings.Contains(bodyStr, "file://"))

	// 21-25: 命令注入特征
	features[21] = boolToFloat(strings.Contains(bodyStr, "|"))
	features[22] = boolToFloat(strings.Contains(bodyStr, "&&"))
	features[23] = boolToFloat(strings.Contains(bodyStr, "`"))
	features[24] = boolToFloat(strings.Contains(bodyStr, "$("))
	features[25] = hasCommandKeywords(bodyStr)

	// 26-29: 其他特征
	features[26] = hasSpecialChars(bodyStr)
	features[27] = hasEncodedChars(bodyStr)
	features[28] = float32(strings.Count(bodyStr, "\n")) / 100.0
	features[29] = float32(strings.Count(bodyStr, "\r")) / 100.0

	return features, nil
}

// ExtractBehaviorFeatures 提取行为特征
func ExtractBehaviorFeatures(req *http.Request) []float32 {
	features := make([]float32, 48)

	if req == nil {
		return features
	}

	// 0: HTTP方法特征
	methods := map[string]int{
		"GET":     0,
		"POST":    1,
		"PUT":     2,
		"DELETE":  3,
		"PATCH":   4,
		"HEAD":    5,
		"OPTIONS": 6,
	}

	if idx, ok := methods[req.Method]; ok {
		features[idx] = 1.0
	}

	// 7: 是否HTTPS
	features[7] = boolToFloat(req.URL.Scheme == "https")

	// 8: 是否有Body
	features[8] = boolToFloat(req.Body != nil && req.ContentLength > 0)

	// 9: Content-Length归一化
	features[9] = normalizeLength(int(req.ContentLength), 10000)

	// 10-17: 请求属性
	features[10] = boolToFloat(req.Close)
	features[11] = boolToFloat(req.Referer() != "")
	features[12] = normalizeLength(len(req.Host), 100)
	features[13] = normalizeLength(len(req.RemoteAddr), 50)
	features[14] = boolToFloat(req.TransferEncoding != nil)
	features[15] = boolToFloat(req.Trailer != nil)
	features[16] = boolToFloat(len(req.Cookies()) > 0)
	features[17] = float32(len(req.Cookies())) / 20.0

	// 18-25: 协议特征
	features[18] = boolToFloat(req.ProtoMajor == 1)
	features[19] = boolToFloat(req.ProtoMajor == 2)
	features[20] = boolToFloat(req.ProtoMinor == 0)
	features[21] = boolToFloat(req.ProtoMinor == 1)

	// 22-25: URL特征补充
	features[22] = boolToFloat(req.URL.IsAbs())
	features[23] = boolToFloat(req.URL.User != nil)
	features[24] = boolToFloat(req.URL.Fragment != "")
	features[25] = boolToFloat(req.URL.ForceQuery)

	// 26-33: 可疑行为模式
	features[26] = boolToFloat(isWebRequest(req))
	features[27] = boolToFloat(isAPIRequest(req))
	features[28] = boolToFloat(isStaticResource(req))
	features[29] = boolToFloat(isAdminPath(req.URL.Path))
	features[30] = boolToFloat(isSensitivePath(req.URL.Path))
	features[31] = boolToFloat(hasDebugParams(req.URL.Query()))
	features[32] = boolToFloat(hasAdminParams(req.URL.Query()))
	features[33] = boolToFloat(hasSensitiveParams(req.URL.Query()))

	// 34-47: 组合特征
	features[34] = features[6] * features[8] // POST + Body
	features[35] = features[1] * features[8] // POST + Body
	features[36] = features[29] * features[33] // Admin path + Sensitive params
	features[37] = features[30] * features[6] // Sensitive path + DELETE
	features[38] = features[26] * features[7] // Web request + HTTPS
	features[39] = features[27] * features[8] // API request + Body
	features[40] = features[16] * features[17] // Has cookies * cookie count
	features[41] = features[11] * features[12] // Has referer * host length
	features[42] = features[9] * features[8]   // Content length * Has body
	features[43] = features[0] + features[1] + features[2] // Method score
	features[44] = features[29] + features[30] + features[31] // Suspicious path score
	features[45] = features[32] + features[33] // Suspicious params score
	features[46] = features[26] + features[27] + features[28] // Request type score
	features[47] = (features[43] + features[44] + features[45]) / 3.0 // Overall suspicious score

	return features
}

// NormalizeFeatures 规范化特征向量大小
func NormalizeFeatures(features []float32, targetSize int) []float32 {
	currentLen := len(features)
	
	if currentLen == targetSize {
		return features
	}
	
	if currentLen < targetSize {
		// 填充0
		result := make([]float32, targetSize)
		copy(result, features)
		return result
	}
	
	// 截断
	return features[:targetSize]
}

// 辅助函数

func normalizeLength(length, max int) float32 {
	if length >= max {
		return 1.0
	}
	return float32(length) / float32(max)
}

func boolToFloat(b bool) float32 {
	if b {
		return 1.0
	}
	return 0.0
}

func hasSpecialChars(s string) float32 {
	specialChars := []string{"'", "\"", "<", ">", "{", "}", "[", "]", "|", ";", ":", "`"}
	count := 0
	for _, c := range specialChars {
		count += strings.Count(s, c)
	}
	if count > 10 {
		return 1.0
	}
	return float32(count) / 10.0
}

func hasEncodedChars(s string) float32 {
	// URL编码
	if strings.Contains(s, "%") {
		return 1.0
	}
	// HTML实体
	if strings.Contains(s, "&#") || strings.Contains(s, "&amp;") {
		return 1.0
	}
	// Base64模式
	if base64Pattern.MatchString(s) {
		return 1.0
	}
	return 0.0
}

var base64Pattern = regexp.MustCompile(`[A-Za-z0-9+/]{20,}={0,2}`)

func hasSuspiciousKeywords(s string) float32 {
	keywords := []string{
		"admin", "root", "password", "passwd", "secret", "token",
		"api_key", "apikey", "auth", "login", "logout", "session",
		"config", "backup", "dump", "export", "import",
	}
	sLower := strings.ToLower(s)
	count := 0
	for _, kw := range keywords {
		if strings.Contains(sLower, kw) {
			count++
		}
	}
	if count > 3 {
		return 1.0
	}
	return float32(count) / 3.0
}

func hasDigits(s string) float32 {
	for _, c := range s {
		if c >= '0' && c <= '9' {
			return 1.0
		}
	}
	return 0.0
}

func hasIPPattern(s string) float32 {
	ipPattern := regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
	if ipPattern.MatchString(s) {
		return 1.0
	}
	return 0.0
}

func hasFileExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	commonExts := []string{".html", ".htm", ".php", ".asp", ".aspx", ".jsp", ".json", ".xml", ".txt", ".pdf"}
	for _, e := range commonExts {
		if ext == e {
			return true
		}
	}
	return false
}

func isSuspiciousUA(ua string) float32 {
	suspiciousUAs := []string{
		"sqlmap", "nikto", "nmap", "masscan", "dirbuster",
		"gobuster", "wfuzz", "burp", "zap", "scanner",
		"bot", "crawler", "spider", "scraper",
	}
	uaLower := strings.ToLower(ua)
	for _, s := range suspiciousUAs {
		if strings.Contains(uaLower, s) {
			return 1.0
		}
	}
	return 0.0
}

func countSpecialCharsInHeaders(header http.Header) float32 {
	var count int
	for _, values := range header {
		for _, v := range values {
			count += strings.Count(v, "'") + strings.Count(v, "\"") + strings.Count(v, "<") + strings.Count(v, ">")
		}
	}
	if count > 20 {
		return 1.0
	}
	return float32(count) / 20.0
}

func countEncodedCharsInHeaders(header http.Header) float32 {
	var count int
	for _, values := range header {
		for _, v := range values {
			count += strings.Count(v, "%")
		}
	}
	if count > 10 {
		return 1.0
	}
	return float32(count) / 10.0
}

func hasSuspiciousKeywordsInHeaders(header http.Header) float32 {
	for _, values := range header {
		for _, v := range values {
			if hasSuspiciousKeywords(v) > 0 {
				return 1.0
			}
		}
	}
	return 0.0
}

func hasSQLKeywordsInHeaders(header http.Header) float32 {
	for _, values := range header {
		for _, v := range values {
			if hasSQLKeywords(v) > 0 {
				return 1.0
			}
		}
	}
	return 0.0
}

func hasSQLKeywords(s string) float32 {
	keywords := []string{
		"SELECT", "INSERT", "UPDATE", "DELETE", "DROP", "ALTER",
		"CREATE", "TRUNCATE", "UNION", "JOIN", "WHERE", "FROM",
		"INTO", "VALUES", "SET", "AND", "OR", "NOT", "NULL",
	}
	sUpper := strings.ToUpper(s)
	count := 0
	for _, kw := range keywords {
		if strings.Contains(sUpper, kw) {
			count++
		}
	}
	if count > 3 {
		return 1.0
	}
	return float32(count) / 3.0
}

func hasSQLPatterns(s string) float32 {
	patterns := []string{
		"OR 1=1", "OR '1'='1", "OR \"1\"=\"1",
		"--", "/*", "*/", "@@", "CHAR(", "NCHAR(",
		"EXEC(", "EXECUTE(", "SP_", "XP_",
	}
	sUpper := strings.ToUpper(s)
	for _, p := range patterns {
		if strings.Contains(sUpper, strings.ToUpper(p)) {
			return 1.0
		}
	}
	return 0.0
}

func hasCommandKeywords(s string) float32 {
	keywords := []string{
		"cmd", "bash", "sh", "powershell", "python", "perl",
		"ruby", "php", "wget", "curl", "nc", "netcat",
		"cat", "ls", "dir", "pwd", "whoami", "id",
	}
	sLower := strings.ToLower(s)
	count := 0
	for _, kw := range keywords {
		if strings.Contains(sLower, kw) {
			count++
		}
	}
	if count > 2 {
		return 1.0
	}
	return float32(count) / 2.0
}

func isWebRequest(req *http.Request) bool {
	accept := req.Header.Get("Accept")
	return strings.Contains(accept, "text/html") || strings.Contains(accept, "application/xhtml+xml")
}

func isAPIRequest(req *http.Request) bool {
	contentType := req.Header.Get("Content-Type")
	accept := req.Header.Get("Accept")
	return strings.Contains(contentType, "application/json") || strings.Contains(accept, "application/json")
}

func isStaticResource(req *http.Request) bool {
	path := strings.ToLower(req.URL.Path)
	staticExts := []string{".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".woff", ".woff2", ".ttf", ".eot"}
	for _, ext := range staticExts {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

func isAdminPath(path string) bool {
	adminPaths := []string{"/admin", "/administrator", "/manage", "/backend", "/console", "/dashboard", "/control"}
	pathLower := strings.ToLower(path)
	for _, p := range adminPaths {
		if strings.HasPrefix(pathLower, p) {
			return true
		}
	}
	return false
}

func isSensitivePath(path string) bool {
	sensitivePaths := []string{"/api", "/config", "/settings", "/user", "/account", "/password", "/login", "/logout", "/register"}
	pathLower := strings.ToLower(path)
	for _, p := range sensitivePaths {
		if strings.HasPrefix(pathLower, p) {
			return true
		}
	}
	return false
}

func hasDebugParams(query url.Values) bool {
	debugParams := []string{"debug", "test", "dev", "trace", "profile", "benchmark"}
	for param := range query {
		paramLower := strings.ToLower(param)
		for _, dp := range debugParams {
			if strings.Contains(paramLower, dp) {
				return true
			}
		}
	}
	return false
}

func hasAdminParams(query url.Values) bool {
	adminParams := []string{"admin", "root", "super", "master", "god", "sudo"}
	for param := range query {
		paramLower := strings.ToLower(param)
		for _, ap := range adminParams {
			if strings.Contains(paramLower, ap) {
				return true
			}
		}
	}
	return false
}

func hasSensitiveParams(query url.Values) bool {
	sensitiveParams := []string{"password", "passwd", "pwd", "secret", "token", "key", "auth", "session"}
	for param := range query {
		paramLower := strings.ToLower(param)
		for _, sp := range sensitiveParams {
			if strings.Contains(paramLower, sp) {
				return true
			}
		}
	}
	return false
}