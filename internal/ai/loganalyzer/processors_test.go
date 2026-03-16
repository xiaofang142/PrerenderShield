package loganalyzer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewFieldNormalizerProcessor(t *testing.T) {
	processor := NewFieldNormalizerProcessor()
	assert.NotNil(t, processor)
	assert.Equal(t, "field_normalizer", processor.Name())
}

func TestFieldNormalizerProcessor_Process_NilEntry(t *testing.T) {
	processor := NewFieldNormalizerProcessor()
	result, err := processor.Process(context.Background(), nil)
	assert.Nil(t, err)
	assert.Nil(t, result)
}

func TestFieldNormalizerProcessor_Process(t *testing.T) {
	processor := NewFieldNormalizerProcessor()

	entry := &LogEntry{
		SourceType: "access",
		Fields: map[string]interface{}{
			"time_local":   "10/Oct/2024:13:55:36 +0800",
			"status":       "200",
			"body_bytes":   "1024",
			"request_time": "0.15",
			"remote_addr":  "192.168.1.1",
			"user_agent":   "Mozilla/5.0 (compatible; Googlebot/2.1)",
		},
	}

	result, err := processor.Process(context.Background(), entry)
	assert.Nil(t, err)
	assert.NotNil(t, result)

	// 检查字段是否被正确设置
	assert.Equal(t, int64(1024), entry.Fields["body_bytes_int"])
	assert.Equal(t, 150.0, entry.Fields["request_time_ms"])
	assert.Equal(t, true, entry.Fields["is_bot"])
	assert.Equal(t, true, entry.Fields["is_search_engine"])
	assert.Equal(t, "success", entry.Fields["status_category"])
	assert.Equal(t, true, entry.Fields["ip_valid"])
}

func TestFieldNormalizerProcessor_Process_StatusCategories(t *testing.T) {
	processor := NewFieldNormalizerProcessor()

	testCases := []struct {
		status   string
		expected string
	}{
		{"200", "success"},
		{"301", "redirect"},
		{"404", "client_error"},
		{"500", "server_error"},
	}

	for _, tc := range testCases {
		entry := &LogEntry{
			Fields: map[string]interface{}{
				"status": tc.status,
			},
		}
		_, err := processor.Process(context.Background(), entry)
		assert.Nil(t, err)
		assert.Equal(t, tc.expected, entry.Fields["status_category"])
	}
}

func TestFieldNormalizerProcessor_Process_InvalidIP(t *testing.T) {
	processor := NewFieldNormalizerProcessor()

	entry := &LogEntry{
		Fields: map[string]interface{}{
			"remote_addr": "invalid-ip",
		},
	}

	_, err := processor.Process(context.Background(), entry)
	assert.Nil(t, err)
	assert.Equal(t, false, entry.Fields["ip_valid"])
}

func TestNewSecurityEnrichmentProcessor(t *testing.T) {
	processor := NewSecurityEnrichmentProcessor()
	assert.NotNil(t, processor)
	assert.Equal(t, "security_enrichment", processor.Name())
}

func TestSecurityEnrichmentProcessor_Process_NilEntry(t *testing.T) {
	processor := NewSecurityEnrichmentProcessor()
	result, err := processor.Process(context.Background(), nil)
	assert.Nil(t, err)
	assert.Nil(t, result)
}

func TestSecurityEnrichmentProcessor_Process_NotSecurityType(t *testing.T) {
	processor := NewSecurityEnrichmentProcessor()

	entry := &LogEntry{
		SourceType: "access",
		Fields:     map[string]interface{}{},
	}

	result, err := processor.Process(context.Background(), entry)
	assert.Nil(t, err)
	assert.Equal(t, entry, result)
}

func TestSecurityEnrichmentProcessor_Process(t *testing.T) {
	processor := NewSecurityEnrichmentProcessor()

	entry := &LogEntry{
		SourceType: "security",
		Fields: map[string]interface{}{
			"threat_level": "high",
			"matched_data": "SELECT * FROM users; DROP TABLE users;--",
			"session_id":   "sess-123",
			"user_id":      "user-456",
			"remote_addr":  "192.168.1.1",
		},
	}

	result, err := processor.Process(context.Background(), entry)
	assert.Nil(t, err)
	assert.NotNil(t, result)

	assert.Equal(t, 75.0, entry.Fields["threat_score"])
	assert.Contains(t, entry.Fields["attack_patterns"], "sql_injection")
	assert.Equal(t, "session:sess-123", entry.Fields["session_key"])
	assert.Equal(t, "user:user-456", entry.Fields["user_key"])
	assert.Equal(t, "ip:192.168.1.1", entry.Fields["ip_key"])
}

func TestSecurityEnrichmentProcessor_threatLevelToScore(t *testing.T) {
	processor := NewSecurityEnrichmentProcessor()

	testCases := []struct {
		level    string
		expected float64
	}{
		{"critical", 100.0},
		{"high", 75.0},
		{"medium", 50.0},
		{"low", 25.0},
		{"unknown", 0.0},
		{"", 0.0},
	}

	for _, tc := range testCases {
		score := processor.threatLevelToScore(tc.level)
		assert.Equal(t, tc.expected, score, "Level: %s", tc.level)
	}
}

func TestSecurityEnrichmentProcessor_extractAttackPatterns(t *testing.T) {
	processor := NewSecurityEnrichmentProcessor()

	testCases := []struct {
		data     string
		expected []string
	}{
		{"SELECT * FROM users", []string{"sql_injection"}},
		{"<script>alert(1)</script>", []string{"xss"}},
		{"../../etc/passwd", []string{"path_traversal"}},
		{"ls -la | grep test", []string{"command_injection"}},
		{"normal text", []string{}},
	}

	for _, tc := range testCases {
		patterns := processor.extractAttackPatterns(tc.data)
		for _, expectedPattern := range tc.expected {
			assert.Contains(t, patterns, expectedPattern, "Data: %s", tc.data)
		}
	}
}

func TestNewRenderEnrichmentProcessor(t *testing.T) {
	processor := NewRenderEnrichmentProcessor()
	assert.NotNil(t, processor)
	assert.Equal(t, "render_enrichment", processor.Name())
}

func TestRenderEnrichmentProcessor_Process_NilEntry(t *testing.T) {
	processor := NewRenderEnrichmentProcessor()
	result, err := processor.Process(context.Background(), nil)
	assert.Nil(t, err)
	assert.Nil(t, result)
}

func TestRenderEnrichmentProcessor_Process_NotRenderType(t *testing.T) {
	processor := NewRenderEnrichmentProcessor()

	entry := &LogEntry{
		SourceType: "access",
		Fields:     map[string]interface{}{},
	}

	result, err := processor.Process(context.Background(), entry)
	assert.Nil(t, err)
	assert.Equal(t, entry, result)
}

func TestRenderEnrichmentProcessor_Process(t *testing.T) {
	processor := NewRenderEnrichmentProcessor()

	entry := &LogEntry{
		SourceType: "render",
		Fields: map[string]interface{}{
			"render_time": 500.0,
			"cache_hit":   true,
		},
	}

	result, err := processor.Process(context.Background(), entry)
	assert.Nil(t, err)
	assert.NotNil(t, result)

	assert.Equal(t, "excellent", entry.Fields["performance_level"])
	assert.Equal(t, "HIT", entry.Fields["cache_result"])
}

func TestRenderEnrichmentProcessor_Process_PerformanceLevels(t *testing.T) {
	processor := NewRenderEnrichmentProcessor()

	testCases := []struct {
		renderTime float64
		expected   string
	}{
		{500, "excellent"},
		{2000, "good"},
		{4000, "fair"},
		{6000, "poor"},
	}

	for _, tc := range testCases {
		entry := &LogEntry{
			SourceType: "render",
			Fields: map[string]interface{}{
				"render_time": tc.renderTime,
			},
		}
		_, err := processor.Process(context.Background(), entry)
		assert.Nil(t, err)
		assert.Equal(t, tc.expected, entry.Fields["performance_level"])
	}
}

func TestRenderEnrichmentProcessor_Process_ErrorClassification(t *testing.T) {
	processor := NewRenderEnrichmentProcessor()

	testCases := []struct {
		errMsg   string
		expected string
	}{
		{"timeout error", "timeout"},
		{"context cancelled", "context_cancelled"},
		{"connection refused", "network"},
		{"dns lookup failed", "dns"},
		{"ssl handshake failed", "ssl"},
		{"javascript error", "javascript"},
		{"unknown error", "unknown"},
	}

	for _, tc := range testCases {
		entry := &LogEntry{
			SourceType: "render",
			Fields: map[string]interface{}{
				"error": tc.errMsg,
			},
		}
		_, err := processor.Process(context.Background(), entry)
		assert.Nil(t, err)
		assert.Equal(t, tc.expected, entry.Fields["error_category"])
		assert.Equal(t, true, entry.Fields["has_error"])
	}
}

func TestRenderEnrichmentProcessor_classifyError(t *testing.T) {
	processor := NewRenderEnrichmentProcessor()

	assert.Equal(t, "timeout", processor.classifyError("request timeout"))
	assert.Equal(t, "context_cancelled", processor.classifyError("context deadline exceeded"))
	assert.Equal(t, "network", processor.classifyError("connection reset"))
	assert.Equal(t, "dns", processor.classifyError("dns resolution failed"))
	assert.Equal(t, "ssl", processor.classifyError("tls handshake error"))
	assert.Equal(t, "javascript", processor.classifyError("javascript exception"))
	assert.Equal(t, "unknown", processor.classifyError("some random error"))
}

func TestNewAnomalyDetectionProcessor(t *testing.T) {
	processor := NewAnomalyDetectionProcessor(nil)
	assert.NotNil(t, processor)
	assert.Equal(t, "anomaly_detection", processor.Name())

	thresholds := &AnomalyThresholds{
		RPMThreshold:       500,
		ErrorRateThreshold: 0.05,
		LatencyThreshold:   3000,
	}
	processorWithThreshold := NewAnomalyDetectionProcessor(thresholds)
	assert.NotNil(t, processorWithThreshold)
	assert.Equal(t, 500, processorWithThreshold.thresholds.RPMThreshold)
}

func TestAnomalyDetectionProcessor_Process_NilEntry(t *testing.T) {
	processor := NewAnomalyDetectionProcessor(nil)
	result, err := processor.Process(context.Background(), nil)
	assert.Nil(t, err)
	assert.Nil(t, result)
}

func TestAnomalyDetectionProcessor_Process(t *testing.T) {
	processor := NewAnomalyDetectionProcessor(&AnomalyThresholds{
		RPMThreshold:       100,
		ErrorRateThreshold: 0.1,
		LatencyThreshold:   5000,
	})

	// 发送正常请求
	entry := &LogEntry{
		Fields: map[string]interface{}{
			"remote_addr":     "192.168.1.1",
			"site_id":         "site-1",
			"status_int":      200,
			"request_time_ms": 100.0,
		},
	}

	_, err := processor.Process(context.Background(), entry)
	assert.Nil(t, err)
	// 第一次请求，不应该有异常（is_anomaly 可能不存在或为 false）
	if isAnomaly, ok := entry.Fields["is_anomaly"].(bool); ok {
		assert.False(t, isAnomaly)
	}

	// 发送高频请求触发 RPM 异常
	for i := 0; i < 150; i++ {
		entry := &LogEntry{
			Fields: map[string]interface{}{
				"remote_addr":     "192.168.1.2",
				"site_id":         "site-1",
				"status_int":      200,
				"request_time_ms": 100.0,
			},
		}
		_, err := processor.Process(context.Background(), entry)
		assert.Nil(t, err)
	}

	// 第 151 次请求应该触发异常
	entry2 := &LogEntry{
		Fields: map[string]interface{}{
			"remote_addr":     "192.168.1.2",
			"site_id":         "site-1",
			"status_int":      200,
			"request_time_ms": 100.0,
		},
	}
	_, err = processor.Process(context.Background(), entry2)
	assert.Nil(t, err)
	assert.True(t, entry2.Fields["is_anomaly"].(bool))
	assert.Contains(t, entry2.Fields["anomalies"], "high_request_rate")
}

func TestAnomalyDetectionProcessor_Process_HighErrorRate(t *testing.T) {
	processor := NewAnomalyDetectionProcessor(&AnomalyThresholds{
		RPMThreshold:       1000,
		ErrorRateThreshold: 0.1,
		LatencyThreshold:   5000,
	})

	// 发送 10 个请求，其中 9 个失败
	for i := 0; i < 9; i++ {
		entry := &LogEntry{
			Fields: map[string]interface{}{
				"remote_addr":     "192.168.1.1",
				"site_id":         "site-error",
				"status_int":      500,
				"request_time_ms": 100.0,
			},
		}
		_, err := processor.Process(context.Background(), entry)
		assert.Nil(t, err)
	}

	entry := &LogEntry{
		Fields: map[string]interface{}{
			"remote_addr":     "192.168.1.1",
			"site_id":         "site-error",
			"status_int":      200,
			"request_time_ms": 100.0,
		},
	}
	_, err := processor.Process(context.Background(), entry)
	assert.Nil(t, err)

	// 错误率 90% 应该触发异常
	assert.True(t, entry.Fields["is_anomaly"].(bool))
	assert.Contains(t, entry.Fields["anomalies"], "high_error_rate")
}

func TestAnomalyDetectionProcessor_Process_HighLatency(t *testing.T) {
	processor := NewAnomalyDetectionProcessor(&AnomalyThresholds{
		RPMThreshold:       1000,
		ErrorRateThreshold: 0.1,
		LatencyThreshold:   5000,
	})

	entry := &LogEntry{
		Fields: map[string]interface{}{
			"remote_addr":     "192.168.1.1",
			"site_id":         "site-1",
			"status_int":      200,
			"request_time_ms": 6000.0, // 超过 5000ms 阈值
		},
	}

	_, err := processor.Process(context.Background(), entry)
	assert.Nil(t, err)
	assert.True(t, entry.Fields["is_anomaly"].(bool))
	assert.Contains(t, entry.Fields["anomalies"], "high_latency")
}

func TestAnomalyDetectionProcessor_updateStats(t *testing.T) {
	processor := NewAnomalyDetectionProcessor(nil)

	entry := &LogEntry{
		Fields: map[string]interface{}{
			"remote_addr":     "192.168.1.1",
			"site_id":         "site-1",
			"status_int":      500,
			"request_time_ms": 200.0,
		},
	}

	processor.updateStats(entry)

	processor.stats.mu.RLock()
	ipStats := processor.stats.ipStats["192.168.1.1"]
	siteStats := processor.stats.siteStats["site-1"]
	processor.stats.mu.RUnlock()

	assert.NotNil(t, ipStats)
	assert.Equal(t, int64(1), ipStats.Count)
	assert.Equal(t, int64(1), ipStats.Errors)
	assert.Equal(t, 200.0, ipStats.TotalLatency)

	assert.NotNil(t, siteStats)
	assert.Equal(t, int64(1), siteStats.Count)
	assert.Equal(t, int64(1), siteStats.Errors)
}

func TestAnomalyDetectionProcessor_detectAnomalies(t *testing.T) {
	processor := NewAnomalyDetectionProcessor(&AnomalyThresholds{
		RPMThreshold:       10,
		ErrorRateThreshold: 0.1,
		LatencyThreshold:   5000,
	})

	entry := &LogEntry{
		Fields: map[string]interface{}{
			"remote_addr":     "192.168.1.1",
			"site_id":         "site-1",
			"status_int":      200,
			"request_time_ms": 100.0,
		},
	}

	// 初始没有异常
	anomalies := processor.detectAnomalies(entry)
	assert.Empty(t, anomalies)

	// 触发高频
	for i := 0; i < 15; i++ {
		processor.updateStats(entry)
	}

	anomalies = processor.detectAnomalies(entry)
	assert.Contains(t, anomalies, "high_request_rate")
}

func TestIsBot(t *testing.T) {
	testCases := []struct {
		userAgent string
		expected  bool
	}{
		{"Mozilla/5.0 (compatible; Googlebot/2.1)", true},
		{"curl/7.68.0", true},
		{"python-requests/2.25.1", true},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64)", false},
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)", false},
	}

	for _, tc := range testCases {
		result := isBot(tc.userAgent)
		assert.Equal(t, tc.expected, result, "User-Agent: %s", tc.userAgent)
	}
}

func TestIsSearchEngine(t *testing.T) {
	testCases := []struct {
		userAgent string
		expected  bool
	}{
		{"Mozilla/5.0 (compatible; Googlebot/2.1)", true},
		{"Mozilla/5.0 (compatible; Bingbot/2.0)", true},
		{"Mozilla/5.0 (compatible; Baiduspider/2.0)", true},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64)", false},
		{"curl/7.68.0", false},
	}

	for _, tc := range testCases {
		result := isSearchEngine(tc.userAgent)
		assert.Equal(t, tc.expected, result, "User-Agent: %s", tc.userAgent)
	}
}

func TestParseTimeLogFormat(t *testing.T) {
	timeStr := "10/Oct/2024:13:55:36 +0800"
	parsed, err := parseTimeLogFormat(timeStr)
	assert.Nil(t, err)
	assert.Equal(t, 2024, parsed.Year())
	assert.Equal(t, time.October, parsed.Month())
	assert.Equal(t, 10, parsed.Day())
}

func TestToBool(t *testing.T) {
	testCases := []struct {
		input    interface{}
		expected bool
	}{
		{true, true},
		{false, false},
		{"true", true},
		{"false", false},
		{"1", true},
		{"0", false},
		{1, true},
		{0, false},
		{1.0, true},
		{0.0, false},
		{nil, false},
		{"", false},
	}

	for _, tc := range testCases {
		result := toBool(tc.input)
		assert.Equal(t, tc.expected, result, "Input: %v", tc.input)
	}
}

func TestIPWindowStats(t *testing.T) {
	now := time.Now()
	stats := &IPWindowStats{
		Count:        100,
		Errors:       5,
		TotalLatency: 1500.0,
		FirstSeen:    now,
		LastSeen:     now.Add(10 * time.Second),
	}

	assert.Equal(t, int64(100), stats.Count)
	assert.Equal(t, int64(5), stats.Errors)
	assert.Equal(t, 1500.0, stats.TotalLatency)
}

func TestSiteWindowStats(t *testing.T) {
	stats := &SiteWindowStats{
		Count:        200,
		Errors:       10,
		TotalLatency: 3000.0,
	}

	assert.Equal(t, int64(200), stats.Count)
	assert.Equal(t, int64(10), stats.Errors)
	assert.Equal(t, 3000.0, stats.TotalLatency)
}

func TestAnomalyThresholds(t *testing.T) {
	thresholds := &AnomalyThresholds{
		RPMThreshold:       500,
		ErrorRateThreshold: 0.05,
		LatencyThreshold:   3000,
	}

	assert.Equal(t, 500, thresholds.RPMThreshold)
	assert.Equal(t, 0.05, thresholds.ErrorRateThreshold)
	assert.Equal(t, 3000.0, thresholds.LatencyThreshold)
}
