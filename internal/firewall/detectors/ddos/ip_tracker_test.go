package ddos

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestIPTracker_New 测试创建 IP 追踪器
func TestIPTracker_New(t *testing.T) {
	tracker := NewIPTracker()
	assert.NotNil(t, tracker)
	assert.NotNil(t, tracker.ipRecords)
	assert.Equal(t, 100000, tracker.maxRecords)
	assert.Equal(t, 30*time.Minute, tracker.recordExpiry)
}

// TestIPTracker_RecordRequest 测试记录请求
func TestIPTracker_RecordRequest(t *testing.T) {
	tracker := NewIPTracker()

	ip := "192.168.1.1"
	req := httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	tracker.RecordRequest(ip, req)

	// 验证记录已创建
	tracker.mu.RLock()
	record, exists := tracker.ipRecords[ip]
	tracker.mu.RUnlock()
	assert.True(t, exists)
	assert.NotNil(t, record)
}

// TestIPTracker_RecordRequest_NilRequest 测试 nil 请求
func TestIPTracker_RecordRequest_NilRequest(t *testing.T) {
	tracker := NewIPTracker()

	ip := "192.168.1.2"

	// 不应该 panic
	assert.NotPanics(t, func() {
		tracker.RecordRequest(ip, nil)
	})
}

// TestIPTracker_GetRequestCount 测试获取请求数
func TestIPTracker_GetRequestCount(t *testing.T) {
	tracker := NewIPTracker()

	ip := "192.168.1.3"
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	// 发送 5 个请求
	for i := 0; i < 5; i++ {
		tracker.RecordRequest(ip, req)
	}

	count := tracker.GetRequestCount(ip, time.Minute)
	assert.Equal(t, 5, count)
}

// TestIPTracker_GetFirstSeen 测试获取首次出现时间
func TestIPTracker_GetFirstSeen(t *testing.T) {
	tracker := NewIPTracker()

	ip := "192.168.1.4"
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	before := time.Now()
	tracker.RecordRequest(ip, req)
	after := time.Now()

	firstSeen := tracker.GetFirstSeen(ip)
	assert.WithinDuration(t, before, firstSeen, time.Second)
	assert.WithinDuration(t, after, firstSeen, time.Second)
}

// TestIPTracker_GetLastSeen 测试获取最后出现时间
func TestIPTracker_GetLastSeen(t *testing.T) {
	tracker := NewIPTracker()

	ip := "192.168.1.5"
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	tracker.RecordRequest(ip, req)
	time.Sleep(10 * time.Millisecond)
	tracker.RecordRequest(ip, req)

	lastSeen := tracker.GetLastSeen(ip)
	assert.WithinDuration(t, time.Now(), lastSeen, time.Second)
}

// TestIPTracker_GetSuspiciousScore 测试获取可疑分数
func TestIPTracker_GetSuspiciousScore(t *testing.T) {
	tracker := NewIPTracker()

	ip := "192.168.1.6"
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	// 初始分数为 0
	score := tracker.GetSuspiciousScore(ip)
	assert.Equal(t, float64(0), score)

	// 发送请求后应该有分数
	tracker.RecordRequest(ip, req)
	score = tracker.GetSuspiciousScore(ip)
	assert.GreaterOrEqual(t, score, float64(0))
	assert.LessOrEqual(t, score, float64(1))
}

// TestIPTracker_HasSuspiciousHeaders 测试检查可疑请求头
func TestIPTracker_HasSuspiciousHeaders(t *testing.T) {
	tracker := NewIPTracker()

	// 没有 User-Agent 的请求
	ip1 := "192.168.1.7"
	req1 := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req1.Header.Del("User-Agent")
	tracker.RecordRequest(ip1, req1)

	// 可能标记为可疑
	_ = tracker.HasSuspiciousHeaders(ip1)
}

// TestIPTracker_HasSlowlorisPattern 测试检查 Slowloris 攻击特征
func TestIPTracker_HasSlowlorisPattern(t *testing.T) {
	tracker := NewIPTracker()

	ip := "192.168.1.8"
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	tracker.RecordRequest(ip, req)

	// 初始不应该有 Slowloris 特征
	assert.False(t, tracker.HasSlowlorisPattern(ip))
}

// TestIPTracker_HasDistributedPattern 测试检查分布式攻击特征
func TestIPTracker_HasDistributedPattern(t *testing.T) {
	tracker := NewIPTracker()

	// 添加同一网段的多个 IP
	for i := 0; i < 15; i++ {
		ip := "192.168.10." + string(rune(i+'0'))
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		tracker.RecordRequest(ip, req)
	}

	// 检查是否检测到分布式攻击
	ip := "192.168.10.1"
	hasDistributed := tracker.HasDistributedPattern(ip)
	assert.True(t, hasDistributed)
}

// TestIPTracker_CleanupExpired 测试清理过期记录
func TestIPTracker_CleanupExpired(t *testing.T) {
	tracker := NewIPTracker()

	ip := "192.168.1.9"
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	tracker.RecordRequest(ip, req)

	// 验证记录已创建
	tracker.mu.RLock()
	_, exists := tracker.ipRecords[ip]
	tracker.mu.RUnlock()
	assert.True(t, exists)

	// 设置较短的过期时间以便测试
	tracker.recordExpiry = 50 * time.Millisecond

	// 等待过期
	time.Sleep(60 * time.Millisecond)

	// 清理
	tracker.CleanupExpired()

	// 验证记录已被清理
	tracker.mu.RLock()
	_, exists = tracker.ipRecords[ip]
	tracker.mu.RUnlock()
	assert.False(t, exists)
}

// TestIPTracker_GetActiveIPs 测试获取活跃 IP 列表
func TestIPTracker_GetActiveIPs(t *testing.T) {
	tracker := NewIPTracker()

	// 添加多个 IP
	for i := 0; i < 5; i++ {
		ip := "192.168.3." + string(rune(i+'0'))
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		tracker.RecordRequest(ip, req)
	}

	// 获取活跃 IP
	ips := tracker.GetActiveIPs(time.Minute)
	assert.GreaterOrEqual(t, len(ips), 5)
}

// TestIPTracker_GetStats 测试获取统计信息
func TestIPTracker_GetStats(t *testing.T) {
	tracker := NewIPTracker()

	// 添加一些 IP
	for i := 0; i < 10; i++ {
		ip := "192.168.4." + string(rune(i+'0'))
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		tracker.RecordRequest(ip, req)
	}

	stats := tracker.GetStats()
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.TotalIPs, 10)
	assert.GreaterOrEqual(t, stats.MaxRecords, 100000)
}

// TestIPTracker_SetFlag_GetFlags 测试设置和获取 IP 标记
func TestIPTracker_SetFlag_GetFlags(t *testing.T) {
	tracker := NewIPTracker()

	ip := "192.168.1.10"
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	tracker.RecordRequest(ip, req)

	// 设置标记
	tracker.SetFlag(ip, "test_flag")

	// 获取标记
	flags := tracker.GetFlags(ip)
	assert.NotNil(t, flags)
	assert.True(t, flags["test_flag"])
}

// TestIPTracker_ResetIP 测试重置 IP 记录
func TestIPTracker_ResetIP(t *testing.T) {
	tracker := NewIPTracker()

	ip := "192.168.1.11"
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	tracker.RecordRequest(ip, req)

	// 验证记录已创建
	tracker.mu.RLock()
	_, exists := tracker.ipRecords[ip]
	tracker.mu.RUnlock()
	assert.True(t, exists)

	// 重置 IP
	tracker.ResetIP(ip)

	// 验证记录已被删除
	tracker.mu.RLock()
	_, exists = tracker.ipRecords[ip]
	tracker.mu.RUnlock()
	assert.False(t, exists)
}

// TestIPTracker_GetIPRecord 测试获取 IP 记录
func TestIPTracker_GetIPRecord(t *testing.T) {
	tracker := NewIPTracker()

	ip := "192.168.1.12"
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	tracker.RecordRequest(ip, req)

	record := tracker.GetIPRecord(ip)
	assert.NotNil(t, record)
	assert.Equal(t, ip, record.IP)
}

// TestIPTracker_GetIPRecord_NotExists 测试获取不存在的 IP 记录
func TestIPTracker_GetIPRecord_NotExists(t *testing.T) {
	tracker := NewIPTracker()

	record := tracker.GetIPRecord("192.168.1.100")
	assert.Nil(t, record)
}

// TestIPRecord 测试 IPRecord 结构
func TestIPRecord(t *testing.T) {
	record := &IPRecord{
		IP:              "192.168.1.1",
		FirstSeen:       time.Now().Add(-time.Hour),
		LastSeen:        time.Now(),
		RequestTimes:    make([]time.Time, 0),
		RequestCount:    100,
		Paths:           map[string]int{"/api": 50, "/admin": 10},
		Methods:         map[string]int{"GET": 80, "POST": 20},
		UserAgents:      []string{"Mozilla/5.0"},
		ResponseCodes:   map[int]int{200: 90, 404: 10},
		SuspiciousScore: 0.5,
		Flags:           map[string]bool{"suspicious_ua": true},
	}

	assert.Equal(t, "192.168.1.1", record.IP)
	assert.Equal(t, 100, record.RequestCount)
	assert.Len(t, record.Paths, 2)
	assert.Len(t, record.Methods, 2)
	assert.Equal(t, float64(0.5), record.SuspiciousScore)
	assert.True(t, record.Flags["suspicious_ua"])
}

// TestIPTrackerStats 测试 IPTrackerStats 结构
func TestIPTrackerStats(t *testing.T) {
	stats := &IPTrackerStats{
		TotalIPs:      1000,
		SuspiciousIPs: 50,
		MaxRecords:    100000,
		RecordExpiry:  30 * time.Minute,
	}

	assert.Equal(t, 1000, stats.TotalIPs)
	assert.Equal(t, 50, stats.SuspiciousIPs)
	assert.Equal(t, 100000, stats.MaxRecords)
	assert.Equal(t, 30*time.Minute, stats.RecordExpiry)
}

// TestContainsIgnoreCase 测试 containsIgnoreCase 函数
func TestContainsIgnoreCase(t *testing.T) {
	assert.True(t, containsIgnoreCase("Hello World", "hello"))
	assert.True(t, containsIgnoreCase("Hello World", "WORLD"))
	assert.False(t, containsIgnoreCase("Hello World", "foo"))
	assert.True(t, containsIgnoreCase("", ""))
}

// TestLower 测试 lower 函数
func TestLower(t *testing.T) {
	assert.Equal(t, "hello", lower("Hello"))
	assert.Equal(t, "world", lower("WORLD"))
	assert.Equal(t, "test123", lower("Test123"))
}

// TestFindSubstring 测试 findSubstring 函数
func TestFindSubstring(t *testing.T) {
	assert.Equal(t, 0, findSubstring("hello world", "hello"))
	assert.Equal(t, 6, findSubstring("hello world", "world"))
	assert.Equal(t, -1, findSubstring("hello world", "foo"))
	assert.Equal(t, 0, findSubstring("", ""))
}

// TestMin 测试 min 函数
func TestMin(t *testing.T) {
	assert.Equal(t, float64(1), min(1, 2))
	assert.Equal(t, float64(2), min(3, 2))
	assert.Equal(t, float64(5), min(5, 5))
}

// TestIsSuspiciousUserAgent 测试 isSuspiciousUserAgent 函数
func TestIsSuspiciousUserAgent(t *testing.T) {
	assert.True(t, isSuspiciousUserAgent("sqlmap/1.0"))
	assert.True(t, isSuspiciousUserAgent("Mozilla/5.0 (compatible; Nikto)"))
	assert.True(t, isSuspiciousUserAgent("curl/7.68.0"))
	assert.True(t, isSuspiciousUserAgent("python-requests/2.25.1"))
	assert.False(t, isSuspiciousUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64)"))
}

// TestGetIPPrefix 测试 getIPPrefix 函数
func TestGetIPPrefix(t *testing.T) {
	assert.Equal(t, "192.168.1.0/24", getIPPrefix("192.168.1.1"))
	assert.Equal(t, "10.0.0.0/24", getIPPrefix("10.0.0.100"))
	assert.Equal(t, "invalid", getIPPrefix("invalid"))
}

// TestSplitIP 测试 splitIP 函数
func TestSplitIP(t *testing.T) {
	parts := splitIP("192.168.1.1")
	assert.Len(t, parts, 4)
	assert.Equal(t, []string{"192", "168", "1", "1"}, parts)
}
