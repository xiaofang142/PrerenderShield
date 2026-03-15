package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewInMemoryRecorder(t *testing.T) {
	recorder := NewInMemoryRecorder()
	assert.NotNil(t, recorder)
	assert.NotNil(t, recorder.requestDuration)
	assert.NotNil(t, recorder.requestCount)
	assert.NotNil(t, recorder.cacheHits)
	assert.NotNil(t, recorder.cacheMisses)
	assert.NotNil(t, recorder.wafBlocks)
	assert.NotNil(t, recorder.errors)
	assert.True(t, recorder.startTime.Before(time.Now()))
}

func TestRecordRequestDuration(t *testing.T) {
	recorder := NewInMemoryRecorder()
	recorder.RecordRequestDuration("GET", "/api/test", 200, 100*time.Millisecond)
	recorder.RecordRequestDuration("GET", "/api/test", 200, 200*time.Millisecond)

	recorder.mu.RLock()
	durations := recorder.requestDuration["GET:/api/test:200"]
	recorder.mu.RUnlock()

	assert.Len(t, durations, 2)
}

func TestRecordRequestDurationMaxLimit(t *testing.T) {
	recorder := NewInMemoryRecorder()

	// Record more than 1000 durations
	for i := 0; i < 1010; i++ {
		recorder.RecordRequestDuration("GET", "/api/test", 200, time.Duration(i)*time.Millisecond)
	}

	recorder.mu.RLock()
	durations := recorder.requestDuration["GET:/api/test:200"]
	recorder.mu.RUnlock()

	// Should be capped at 1000
	assert.Len(t, durations, 1000)
	// First 10 should be removed (oldest removed first)
	assert.Equal(t, int64(10), durations[0].Milliseconds())
}

func TestRecordRequestCount(t *testing.T) {
	recorder := NewInMemoryRecorder()
	recorder.RecordRequestCount("GET", "/api/test", 200)
	recorder.RecordRequestCount("GET", "/api/test", 200)
	recorder.RecordRequestCount("POST", "/api/test", 201)

	recorder.mu.RLock()
	count1 := recorder.requestCount["GET:/api/test:200"]
	count2 := recorder.requestCount["POST:/api/test:201"]
	total := recorder.totalRequests
	recorder.mu.RUnlock()

	assert.Equal(t, int64(2), count1)
	assert.Equal(t, int64(1), count2)
	assert.Equal(t, int64(3), total)
}

func TestRecordCacheHit(t *testing.T) {
	recorder := NewInMemoryRecorder()
	recorder.RecordCacheHit("site-1")
	recorder.RecordCacheHit("site-1")
	recorder.RecordCacheHit("site-2")

	recorder.mu.RLock()
	site1Hits := recorder.cacheHits["site-1"]
	site2Hits := recorder.cacheHits["site-2"]
	totalHits := recorder.totalCacheHits
	recorder.mu.RUnlock()

	assert.Equal(t, int64(2), site1Hits)
	assert.Equal(t, int64(1), site2Hits)
	assert.Equal(t, int64(3), totalHits)
}

func TestRecordCacheMiss(t *testing.T) {
	recorder := NewInMemoryRecorder()
	recorder.RecordCacheMiss("site-1")
	recorder.RecordCacheMiss("site-1")
	recorder.RecordCacheMiss("site-2")

	recorder.mu.RLock()
	site1Misses := recorder.cacheMisses["site-1"]
	site2Misses := recorder.cacheMisses["site-2"]
	totalMisses := recorder.totalCacheMisses
	recorder.mu.RUnlock()

	assert.Equal(t, int64(2), site1Misses)
	assert.Equal(t, int64(1), site2Misses)
	assert.Equal(t, int64(3), totalMisses)
}

func TestRecordWAFBlock(t *testing.T) {
	recorder := NewInMemoryRecorder()
	recorder.RecordWAFBlock("site-1", "sql_injection")
	recorder.RecordWAFBlock("site-1", "sql_injection")
	recorder.RecordWAFBlock("site-2", "xss")

	recorder.mu.RLock()
	site1Blocks := recorder.wafBlocks["site-1:sql_injection"]
	site2Blocks := recorder.wafBlocks["site-2:xss"]
	totalBlocks := recorder.totalWAFBlocks
	recorder.mu.RUnlock()

	assert.Equal(t, int64(2), site1Blocks)
	assert.Equal(t, int64(1), site2Blocks)
	assert.Equal(t, int64(3), totalBlocks)
}

func TestRecordError(t *testing.T) {
	recorder := NewInMemoryRecorder()
	recorder.RecordError("auth", "token_expired")
	recorder.RecordError("auth", "token_expired")
	recorder.RecordError("waf", "rule_violation")

	recorder.mu.RLock()
	authErrors := recorder.errors["auth:token_expired"]
	wafErrors := recorder.errors["waf:rule_violation"]
	totalErrors := recorder.totalErrors
	recorder.mu.RUnlock()

	assert.Equal(t, int64(2), authErrors)
	assert.Equal(t, int64(1), wafErrors)
	assert.Equal(t, int64(3), totalErrors)
}

func TestGetMetrics(t *testing.T) {
	recorder := NewInMemoryRecorder()

	// Record some data
	recorder.RecordRequestCount("GET", "/api/test", 200)
	recorder.RecordRequestDuration("GET", "/api/test", 200, 100*time.Millisecond)
	recorder.RecordRequestDuration("GET", "/api/test", 200, 200*time.Millisecond)
	recorder.RecordCacheHit("site-1")
	recorder.RecordCacheMiss("site-1")
	recorder.RecordWAFBlock("site-1", "sql_injection")
	recorder.RecordError("auth", "token_expired")

	metrics := recorder.GetMetrics()

	assert.NotNil(t, metrics["uptime_seconds"])
	assert.Equal(t, int64(1), metrics["total_requests"])
	assert.Equal(t, int64(1), metrics["cache_hits"])
	assert.Equal(t, int64(1), metrics["cache_misses"])
	assert.Equal(t, int64(1), metrics["waf_blocks"])
	assert.Equal(t, int64(1), metrics["total_errors"])

	// Cache hit rate should be 50% (1 hit, 1 miss)
	assert.Equal(t, 0.5, metrics["cache_hit_rate"])

	// Average response time should be 150ms ((100+200)/2)
	assert.Equal(t, 150.0, metrics["avg_response_time_ms"])
}

func TestGetMetricsEmptyData(t *testing.T) {
	recorder := NewInMemoryRecorder()
	metrics := recorder.GetMetrics()

	assert.NotNil(t, metrics)
	assert.Equal(t, int64(0), metrics["total_requests"])
	assert.Equal(t, int64(0), metrics["cache_hits"])
	assert.Equal(t, int64(0), metrics["cache_misses"])
	assert.Equal(t, int64(0), metrics["waf_blocks"])
	assert.Equal(t, int64(0), metrics["total_errors"])
	assert.Equal(t, 0.0, metrics["avg_response_time_ms"])
}

func TestGetMetricsNoCache(t *testing.T) {
	recorder := NewInMemoryRecorder()
	recorder.RecordRequestCount("GET", "/api/test", 200)

	metrics := recorder.GetMetrics()

	// No cache operations, so cache_hit_rate should not be set
	_, exists := metrics["cache_hit_rate"]
	assert.False(t, exists)
}

func TestCalculateAvgResponseTime(t *testing.T) {
	recorder := NewInMemoryRecorder()

	// No data
	assert.Equal(t, 0.0, recorder.calculateAvgResponseTime())

	// Single value
	recorder.RecordRequestDuration("GET", "/api/test", 200, 100*time.Millisecond)
	assert.Equal(t, 100.0, recorder.calculateAvgResponseTime())

	// Multiple values
	recorder.RecordRequestDuration("GET", "/api/test", 200, 200*time.Millisecond)
	recorder.RecordRequestDuration("POST", "/api/test", 201, 300*time.Millisecond)
	// (100 + 200 + 300) / 3 = 200
	assert.Equal(t, 200.0, recorder.calculateAvgResponseTime())
}

func TestMetricKey(t *testing.T) {
	key := metricKey("GET", "/api/test", 200)
	assert.Equal(t, "GET:/api/test:200", key)

	key2 := metricKey("POST", "/api/users", 201)
	assert.Equal(t, "POST:/api/users:201", key2)
}

func TestConcurrentAccess(t *testing.T) {
	recorder := NewInMemoryRecorder()
	done := make(chan bool, 10)

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				recorder.RecordRequestCount("GET", "/api/test", 200)
				recorder.RecordCacheHit("site-1")
				recorder.RecordWAFBlock("site-1", "sql_injection")
				recorder.RecordError("auth", "token_expired")
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to finish
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify counts
	metrics := recorder.GetMetrics()
	assert.Equal(t, int64(1000), metrics["total_requests"])
	assert.Equal(t, int64(1000), metrics["cache_hits"])
	assert.Equal(t, int64(1000), metrics["waf_blocks"])
	assert.Equal(t, int64(1000), metrics["total_errors"])
}
