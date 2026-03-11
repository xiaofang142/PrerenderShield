package ddos

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// IPTracker IP 行为追踪器
type IPTracker struct {
	mu           sync.RWMutex
	ipRecords    map[string]*IPRecord
	maxRecords   int
	recordExpiry time.Duration
}

// IPRecord IP 记录
type IPRecord struct {
	mu              sync.RWMutex
	IP              string
	FirstSeen       time.Time
	LastSeen        time.Time
	RequestTimes    []time.Time
	RequestCount    int
	Paths           map[string]int       // 请求路径统计
	Methods         map[string]int       // 请求方法统计
	Headers         map[string][]string  // 请求头样本
	UserAgents      []string             // User-Agent 列表
	ResponseCodes   map[int]int          // 响应码统计
	SuspiciousScore float64              // 可疑分数
	Flags           map[string]bool      // 行为标记
}

// NewIPTracker 创建 IP 追踪器
func NewIPTracker() *IPTracker {
	return &IPTracker{
		ipRecords:    make(map[string]*IPRecord),
		maxRecords:   100000,
		recordExpiry: 30 * time.Minute,
	}
}

// RecordRequest 记录请求
func (t *IPTracker) RecordRequest(ip string, req *http.Request) {
	t.mu.Lock()
	record, exists := t.ipRecords[ip]
	if !exists {
		record = &IPRecord{
			IP:            ip,
			FirstSeen:     time.Now(),
			LastSeen:      time.Now(),
			RequestTimes:  make([]time.Time, 0, 100),
			RequestCount:  0,
			Paths:         make(map[string]int),
			Methods:       make(map[string]int),
			Headers:       make(map[string][]string),
			UserAgents:    make([]string, 0, 5),
			ResponseCodes: make(map[int]int),
			Flags:         make(map[string]bool),
		}
		t.ipRecords[ip] = record
	}
	t.mu.Unlock()

	record.mu.Lock()
	defer record.mu.Unlock()

	now := time.Now()
	record.LastSeen = now
	record.RequestCount++

	// 记录请求时间（保留最近 100 个）
	record.RequestTimes = append(record.RequestTimes, now)
	if len(record.RequestTimes) > 100 {
		record.RequestTimes = record.RequestTimes[1:]
	}

	// 记录请求路径
	if req != nil {
		path := req.URL.Path
		record.Paths[path]++

		// 记录请求方法
		record.Methods[req.Method]++

		// 记录 User-Agent
		ua := req.Header.Get("User-Agent")
		if ua != "" {
			if len(record.UserAgents) == 0 || record.UserAgents[len(record.UserAgents)-1] != ua {
				record.UserAgents = append(record.UserAgents, ua)
				if len(record.UserAgents) > 5 {
					record.UserAgents = record.UserAgents[1:]
				}
			}
		}

		// 记录关键请求头
		record.recordHeaders(req)
	}

	// 更新可疑分数
	record.updateSuspiciousScore()
}

// recordHeaders 记录关键请求头
func (r *IPRecord) recordHeaders(req *http.Request) {
	keyHeaders := []string{
		"Accept",
		"Accept-Language",
		"Accept-Encoding",
		"Referer",
		"Origin",
		"Content-Type",
	}

	for _, h := range keyHeaders {
		if v := req.Header.Get(h); v != "" {
			if len(r.Headers[h]) < 10 {
				r.Headers[h] = append(r.Headers[h], v)
			}
		}
	}
}

// updateSuspiciousScore 更新可疑分数
func (r *IPRecord) updateSuspiciousScore() {
	score := 0.0

	// 检查请求频率异常
	if len(r.RequestTimes) >= 2 {
		intervals := make([]time.Duration, 0, len(r.RequestTimes)-1)
		for i := 1; i < len(r.RequestTimes); i++ {
			intervals = append(intervals, r.RequestTimes[i].Sub(r.RequestTimes[i-1]))
		}

		// 计算平均间隔
		var total time.Duration
		for _, d := range intervals {
			total += d
		}
		avgInterval := total / time.Duration(len(intervals))

		// 间隔过短（<10ms）增加可疑分数
		if avgInterval < 10*time.Millisecond {
			score += 0.3
		} else if avgInterval < 50*time.Millisecond {
			score += 0.1
		}
	}

	// 检查 User-Agent 异常
	if len(r.UserAgents) == 0 {
		score += 0.2 // 无 User-Agent
	} else {
		// 检查是否使用已知扫描工具
		for _, ua := range r.UserAgents {
			if isSuspiciousUserAgent(ua) {
				score += 0.3
				r.Flags["suspicious_ua"] = true
				break
			}
		}
	}

	// 检查路径扫描行为（访问大量不同路径）
	if len(r.Paths) > 50 {
		score += 0.2
		r.Flags["path_scanning"] = true
	}

	// 检查请求方法异常（大量非 GET/POST）
	nonStandardMethods := 0
	for method, count := range r.Methods {
		if method != "GET" && method != "POST" && method != "HEAD" {
			nonStandardMethods += count
		}
	}
	if nonStandardMethods > 10 {
		score += 0.1
		r.Flags["unusual_methods"] = true
	}

	r.SuspiciousScore = min(score, 1.0)
}

// isSuspiciousUserAgent 检查是否为可疑 User-Agent
func isSuspiciousUserAgent(ua string) bool {
	suspiciousPatterns := []string{
		"sqlmap",
		"nikto",
		"nmap",
		"masscan",
		"zgrab",
		"curl/",
		"wget/",
		"python-requests",
		"go-http-client",
		"scanner",
		"crawler",
		"bot",
	}

	for _, pattern := range suspiciousPatterns {
		if containsIgnoreCase(ua, pattern) {
			return true
		}
	}
	return false
}

// GetRequestCount 获取指定时间范围内的请求数
func (t *IPTracker) GetRequestCount(ip string, duration time.Duration) int {
	t.mu.RLock()
	record, exists := t.ipRecords[ip]
	if !exists {
		return 0
	}
	t.mu.RUnlock()

	record.mu.RLock()
	defer record.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	count := 0
	for _, t := range record.RequestTimes {
		if t.After(cutoff) {
			count++
		}
	}

	return count
}

// GetFirstSeen 获取首次见到时间
func (t *IPTracker) GetFirstSeen(ip string) time.Time {
	t.mu.RLock()
	record, exists := t.ipRecords[ip]
	if !exists {
		return time.Time{}
	}
	t.mu.RUnlock()

	record.mu.RLock()
	defer record.mu.RUnlock()
	return record.FirstSeen
}

// GetLastSeen 获取最后见到时间
func (t *IPTracker) GetLastSeen(ip string) time.Time {
	t.mu.RLock()
	record, exists := t.ipRecords[ip]
	if !exists {
		return time.Time{}
	}
	t.mu.RUnlock()

	record.mu.RLock()
	defer record.mu.RUnlock()
	return record.LastSeen
}

// GetSuspiciousScore 获取可疑分数
func (t *IPTracker) GetSuspiciousScore(ip string) float64 {
	t.mu.RLock()
	record, exists := t.ipRecords[ip]
	if !exists {
		return 0
	}
	t.mu.RUnlock()

	record.mu.RLock()
	defer record.mu.RUnlock()
	return record.SuspiciousScore
}

// HasSuspiciousHeaders 检查是否有可疑请求头
func (t *IPTracker) HasSuspiciousHeaders(ip string) bool {
	t.mu.RLock()
	record, exists := t.ipRecords[ip]
	if !exists {
		return false
	}
	t.mu.RUnlock()

	record.mu.RLock()
	defer record.mu.RUnlock()

	// 检查是否有 User-Agent
	hasUA := false
	for k, v := range record.Headers {
		if k == "User-Agent" && len(v) > 0 {
			hasUA = true
			break
		}
	}

	return !hasUA || record.Flags["suspicious_ua"]
}

// HasSlowlorisPattern 检查是否有 Slowloris 攻击特征
func (t *IPTracker) HasSlowlorisPattern(ip string) bool {
	t.mu.RLock()
	record, exists := t.ipRecords[ip]
	if !exists {
		return false
	}
	t.mu.RUnlock()

	record.mu.RLock()
	defer record.mu.RUnlock()

	// Slowloris 特征：长时间保持连接，发送不完整请求头
	// 这里通过检查是否有大量部分请求来判断
	return record.Flags["slowloris"] || record.SuspiciousScore > 0.7
}

// HasDistributedPattern 检查是否有分布式攻击特征
func (t *IPTracker) HasDistributedPattern(ip string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// 获取 IP 段
	ipPrefix := getIPPrefix(ip)

	// 统计同一 IP 段的 IP 数量
	count := 0
	for recordedIP := range t.ipRecords {
		if getIPPrefix(recordedIP) == ipPrefix {
			count++
			if count >= 10 {
				return true
			}
		}
	}

	return false
}

// getIPPrefix 获取 IP 前缀（/24 网段）
func getIPPrefix(ip string) string {
	// 解析 IP
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return ip
	}

	// 对于 IPv4，返回前 3 段
	if parsedIP.To4() != nil {
		parts := splitIP(ip)
		if len(parts) >= 3 {
			return parts[0] + "." + parts[1] + "." + parts[2] + ".0/24"
		}
	}

	return ip
}

// splitIP 分割 IP 地址
func splitIP(ip string) []string {
	parts := make([]string, 0, 4)
	start := 0
	for i := 0; i < len(ip); i++ {
		if ip[i] == '.' {
			parts = append(parts, ip[start:i])
			start = i + 1
		}
	}
	parts = append(parts, ip[start:])
	return parts
}

// GetIPRecord 获取 IP 记录
func (t *IPTracker) GetIPRecord(ip string) *IPRecord {
	t.mu.RLock()
	record, exists := t.ipRecords[ip]
	if !exists {
		return nil
	}
	t.mu.RUnlock()

	return record
}

// GetActiveIPs 获取活跃 IP 列表
func (t *IPTracker) GetActiveIPs(duration time.Duration) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	ips := make([]string, 0)

	for ip, record := range t.ipRecords {
		record.mu.RLock()
		if record.LastSeen.After(cutoff) {
			ips = append(ips, ip)
		}
		record.mu.RUnlock()
	}

	return ips
}

// CleanupExpired 清理过期记录
func (t *IPTracker) CleanupExpired() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-t.recordExpiry)

	for ip, record := range t.ipRecords {
		record.mu.RLock()
		shouldDelete := record.LastSeen.Before(cutoff)
		record.mu.RUnlock()

		if shouldDelete {
			delete(t.ipRecords, ip)
		}
	}
}

// GetStats 获取追踪器统计
func (t *IPTracker) GetStats() *IPTrackerStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	totalIPs := len(t.ipRecords)
	suspiciousIPs := 0

	for _, record := range t.ipRecords {
		record.mu.RLock()
		if record.SuspiciousScore > 0.5 {
			suspiciousIPs++
		}
		record.mu.RUnlock()
	}

	return &IPTrackerStats{
		TotalIPs:       totalIPs,
		SuspiciousIPs:  suspiciousIPs,
		MaxRecords:     t.maxRecords,
		RecordExpiry:   t.recordExpiry,
	}
}

// IPTrackerStats IP 追踪器统计
type IPTrackerStats struct {
	TotalIPs      int           `json:"total_ips"`
	SuspiciousIPs int           `json:"suspicious_ips"`
	MaxRecords    int           `json:"max_records"`
	RecordExpiry  time.Duration `json:"record_expiry"`
}

// SetFlag 设置 IP 标记
func (t *IPTracker) SetFlag(ip, flag string) {
	t.mu.RLock()
	record, exists := t.ipRecords[ip]
	if !exists {
		t.mu.RUnlock()
		return
	}
	t.mu.RUnlock()

	record.mu.Lock()
	record.Flags[flag] = true
	record.mu.Unlock()
}

// GetFlags 获取 IP 标记
func (t *IPTracker) GetFlags(ip string) map[string]bool {
	t.mu.RLock()
	record, exists := t.ipRecords[ip]
	if !exists {
		return nil
	}
	t.mu.RUnlock()

	record.mu.RLock()
	defer record.mu.RUnlock()

	flags := make(map[string]bool)
	for k, v := range record.Flags {
		flags[k] = v
	}
	return flags
}

// ResetIP 重置 IP 记录
func (t *IPTracker) ResetIP(ip string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.ipRecords, ip)
}

// ContainsIgnoreCase 检查字符串是否包含子串（忽略大小写）
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || lower(s) == lower(substr) ||
		 findSubstring(lower(s), lower(substr)) >= 0)
}

func lower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func findSubstring(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
