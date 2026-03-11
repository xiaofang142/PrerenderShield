package loganalyzer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseLogEntry_JSON(t *testing.T) {
	raw := `{"remote_addr":"192.168.1.1","method":"GET","uri":"/api/test","status":200,"body_bytes":1024}`
	entry := ParseLogEntry(raw, "test")

	assert.NotNil(t, entry)
	assert.Equal(t, "access", entry.SourceType)
	assert.Equal(t, "192.168.1.1", entry.Fields["remote_addr"])
}

func TestParseLogEntry_Nginx(t *testing.T) {
	raw := `192.168.1.1 - - [10/Oct/2024:13:55:36 +0800] "GET /api/test HTTP/1.1" 200 1024 "-" "Mozilla/5.0"`
	entry := ParseLogEntry(raw, "test")

	assert.NotNil(t, entry)
	assert.Equal(t, "access", entry.SourceType)
	assert.Equal(t, "192.168.1.1", entry.Fields["remote_addr"])
	assert.Equal(t, "GET", entry.Fields["method"])
	assert.Equal(t, "/api/test", entry.Fields["uri"])
	assert.Equal(t, "200", entry.Fields["status"])
}

func TestFieldNormalizerProcessor(t *testing.T) {
	processor := NewFieldNormalizerProcessor()
	entry := &LogEntry{
		SourceType: "access",
		Fields: map[string]interface{}{
			"remote_addr":  "192.168.1.1",
			"status":       "200",
			"body_bytes":   "1024",
			"request_time": "0.123",
			"user_agent":   "Mozilla/5.0 (compatible; Googlebot/2.1)",
		},
	}

	result, err := processor.Process(context.Background(), entry)
	assert.Nil(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "success", result.Fields["status_category"])
	assert.Equal(t, true, result.Fields["is_search_engine"])
}

func TestSecurityEnrichmentProcessor(t *testing.T) {
	processor := NewSecurityEnrichmentProcessor()
	entry := &LogEntry{
		SourceType: "security",
		Fields: map[string]interface{}{
			"threat_level": "high",
			"matched_data": "SELECT * FROM users",
			"remote_addr":  "192.168.1.1",
		},
	}

	result, err := processor.Process(context.Background(), entry)
	assert.Nil(t, err)
	assert.Equal(t, 75.0, result.Fields["threat_score"])
	assert.Contains(t, result.Fields["attack_patterns"], "sql_injection")
}

func TestRenderEnrichmentProcessor(t *testing.T) {
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
	assert.Equal(t, "excellent", result.Fields["performance_level"])
	assert.Equal(t, "HIT", result.Fields["cache_result"])
}

func TestAnomalyDetectionProcessor(t *testing.T) {
	thresholds := &AnomalyThresholds{
		RPMThreshold:       100,
		ErrorRateThreshold: 0.1,
		LatencyThreshold:   5000,
	}
	processor := NewAnomalyDetectionProcessor(thresholds)

	// 正常请求
	entry := &LogEntry{
		SourceType: "access",
		Fields: map[string]interface{}{
			"remote_addr":    "192.168.1.1",
			"status_int":     200,
			"request_time_ms": 100.0,
		},
	}

	result, err := processor.Process(context.Background(), entry)
	assert.Nil(t, err)
	assert.False(t, toBool(result.Fields["is_anomaly"]))
}

func TestAggregator_Window(t *testing.T) {
	outputChan := make(chan *AggregatedResult, 10)
	config := &AggregatorConfig{
		WindowSize: 1 * time.Minute,
		SlideSize:  10 * time.Second,
		MaxWindows: 5,
	}

	agg := NewAggregator(config, outputChan, nil)
	defer agg.Close()

	entry := &LogEntry{
		SourceType: "access",
		Timestamp:  time.Now(),
		Fields: map[string]interface{}{
			"remote_addr": "192.168.1.1",
			"status":      "200",
			"uri":         "/api/test",
		},
	}

	agg.Process(entry)

	stats := agg.GetWindowStats()
	assert.GreaterOrEqual(t, stats["window_count"], 0)
}

func TestStreamEngine(t *testing.T) {
	inputChan := make(chan *LogEntry, 10)
	config := &StreamConfig{
		WorkerCount:   2,
		BatchSize:     10,
		BatchTimeout:  100 * time.Millisecond,
		EnableMetrics: true,
	}

	engine := NewStreamEngine(config, inputChan, nil)
	engine.AddProcessor(NewFieldNormalizerProcessor())

	err := engine.Start()
	assert.Nil(t, err)
	defer engine.Stop()

	// 发送测试日志
	entry := &LogEntry{
		SourceType: "access",
		Fields: map[string]interface{}{
			"remote_addr": "192.168.1.1",
			"status":      "200",
		},
	}

	inputChan <- entry
	time.Sleep(200 * time.Millisecond)

	stats := engine.GetStats()
	assert.GreaterOrEqual(t, stats["total_processed"], int64(1))
}

func TestIsBot(t *testing.T) {
	assert.True(t, isBot("Googlebot/2.1"))
	assert.True(t, isBot("curl/7.68.0"))
	assert.False(t, isBot("Mozilla/5.0 (Windows NT 10.0; Win64; x64)"))
}

func TestIsSearchEngine(t *testing.T) {
	assert.True(t, isSearchEngine("Googlebot/2.1"))
	assert.True(t, isSearchEngine("Bingbot/2.0"))
	assert.False(t, isSearchEngine("curl/7.68.0"))
}

func TestPercentile(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	assert.Equal(t, 4.0, percentile(values, 0.4))
	assert.Equal(t, 9.0, percentile(values, 0.9))
}

func TestAverage(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	assert.Equal(t, 3.0, average(values))
}
