package monitoring

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ============== 辅助函数 ==============

// ============== NewMetricsCollector 测试 ==============

func TestNewMetricsCollector(t *testing.T) {
	collector := NewMetricsCollector(nil)

	assert.NotNil(t, collector)
	assert.NotNil(t, collector.(*metricsCollector).counters)
	assert.NotNil(t, collector.(*metricsCollector).gauges)
	assert.NotNil(t, collector.(*metricsCollector).timers)
	assert.NotZero(t, collector.(*metricsCollector).startTime)
}

func TestNewMetricsCollector_NilRedis(t *testing.T) {
	collector := NewMetricsCollector(nil)

	assert.NotNil(t, collector)
	assert.Nil(t, collector.(*metricsCollector).redisClient)
}

// ============== IncrementCounter 测试 ==============

func TestMetricsCollector_IncrementCounter(t *testing.T) {
	collector := NewMetricsCollector(nil).(*metricsCollector)

	collector.IncrementCounter("requests", 1)
	collector.IncrementCounter("requests", 5)
	collector.IncrementCounter("errors", 2)

	collector.mutex.RLock()
	defer collector.mutex.RUnlock()

	assert.Equal(t, int64(6), collector.counters["requests"])
	assert.Equal(t, int64(2), collector.counters["errors"])
}

func TestMetricsCollector_IncrementCounter_WithRedis(t *testing.T) {
	// Redis 测试在实际环境中进行，这里只测试内存操作
	collector := NewMetricsCollector(nil).(*metricsCollector)

	collector.IncrementCounter("test_counter", 10)

	collector.mutex.RLock()
	defer collector.mutex.RUnlock()

	assert.Equal(t, int64(10), collector.counters["test_counter"])
}

func TestMetricsCollector_IncrementCounter_Concurrent(t *testing.T) {
	collector := NewMetricsCollector(nil).(*metricsCollector)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			collector.IncrementCounter("concurrent", 1)
		}()
	}

	wg.Wait()

	collector.mutex.RLock()
	defer collector.mutex.RUnlock()

	assert.Equal(t, int64(100), collector.counters["concurrent"])
}

// ============== SetGauge 测试 ==============

func TestMetricsCollector_SetGauge(t *testing.T) {
	collector := NewMetricsCollector(nil).(*metricsCollector)

	collector.SetGauge("cpu_usage", 75.5)
	collector.SetGauge("memory_usage", 82.3)
	collector.SetGauge("cpu_usage", 50.0) // 覆盖

	collector.mutex.RLock()
	defer collector.mutex.RUnlock()

	assert.Equal(t, 50.0, collector.gauges["cpu_usage"])
	assert.Equal(t, 82.3, collector.gauges["memory_usage"])
}

func TestMetricsCollector_SetGauge_NegativeValue(t *testing.T) {
	collector := NewMetricsCollector(nil).(*metricsCollector)

	collector.SetGauge("temperature", -10.5)

	collector.mutex.RLock()
	defer collector.mutex.RUnlock()

	assert.Equal(t, -10.5, collector.gauges["temperature"])
}

func TestMetricsCollector_SetGauge_ZeroValue(t *testing.T) {
	collector := NewMetricsCollector(nil).(*metricsCollector)

	collector.SetGauge("active_connections", 0)

	collector.mutex.RLock()
	defer collector.mutex.RUnlock()

	assert.Equal(t, float64(0), collector.gauges["active_connections"])
}

func TestMetricsCollector_SetGauge_Concurrent(t *testing.T) {
	collector := NewMetricsCollector(nil).(*metricsCollector)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(val float64) {
			defer wg.Done()
			collector.SetGauge("concurrent_gauge", val)
		}(float64(i))
	}

	wg.Wait()

	// 最后一个值应该是 99
	collector.mutex.RLock()
	defer collector.mutex.RUnlock()

	assert.NotZero(t, collector.gauges["concurrent_gauge"])
}

// ============== RecordTimer 测试 ==============

func TestMetricsCollector_RecordTimer(t *testing.T) {
	collector := NewMetricsCollector(nil).(*metricsCollector)

	collector.RecordTimer("request_duration", 100*time.Millisecond)
	collector.RecordTimer("request_duration", 200*time.Millisecond)
	collector.RecordTimer("request_duration", 300*time.Millisecond)

	collector.mutex.RLock()
	defer collector.mutex.RUnlock()

	assert.Len(t, collector.timers["request_duration"], 3)
}

func TestMetricsCollector_RecordTimer_Limit100(t *testing.T) {
	collector := NewMetricsCollector(nil).(*metricsCollector)

	// 记录 150 个计时器值
	for i := 0; i < 150; i++ {
		collector.RecordTimer("many_timers", time.Duration(i)*time.Millisecond)
	}

	collector.mutex.RLock()
	defer collector.mutex.RUnlock()

	// 应该只保留最后 100 个
	assert.Len(t, collector.timers["many_timers"], 100)

	// 第一个值应该是 50ms（第 51 个记录的值）
	assert.Equal(t, 50*time.Millisecond, collector.timers["many_timers"][0])
}

func TestMetricsCollector_RecordTimer_MultipleTimers(t *testing.T) {
	collector := NewMetricsCollector(nil).(*metricsCollector)

	collector.RecordTimer("timer1", 10*time.Millisecond)
	collector.RecordTimer("timer1", 20*time.Millisecond)
	collector.RecordTimer("timer2", 100*time.Millisecond)

	collector.mutex.RLock()
	defer collector.mutex.RUnlock()

	assert.Len(t, collector.timers["timer1"], 2)
	assert.Len(t, collector.timers["timer2"], 1)
}

func TestMetricsCollector_RecordTimer_ZeroDuration(t *testing.T) {
	collector := NewMetricsCollector(nil).(*metricsCollector)

	collector.RecordTimer("zero_timer", 0)

	collector.mutex.RLock()
	defer collector.mutex.RUnlock()

	assert.Len(t, collector.timers["zero_timer"], 1)
	assert.Equal(t, time.Duration(0), collector.timers["zero_timer"][0])
}

// ============== GetMetrics 测试 ==============

func TestMetricsCollector_GetMetrics_Empty(t *testing.T) {
	collector := NewMetricsCollector(nil).(*metricsCollector)

	metrics := collector.GetMetrics()

	assert.NotNil(t, metrics)
	assert.Contains(t, metrics, "counters")
	assert.Contains(t, metrics, "gauges")
	assert.Contains(t, metrics, "timers")
	assert.Contains(t, metrics, "system")

	assert.Empty(t, metrics["counters"].(map[string]int64))
	assert.Empty(t, metrics["gauges"].(map[string]float64))
	assert.Empty(t, metrics["timers"].(map[string]interface{}))
	assert.NotNil(t, metrics["system"].(map[string]interface{})["uptime_seconds"])
}

func TestMetricsCollector_GetMetrics_WithData(t *testing.T) {
	collector := NewMetricsCollector(nil).(*metricsCollector)

	collector.IncrementCounter("requests", 100)
	collector.IncrementCounter("errors", 5)
	collector.SetGauge("cpu", 75.5)
	collector.RecordTimer("latency", 50*time.Millisecond)
	collector.RecordTimer("latency", 100*time.Millisecond)
	collector.RecordTimer("latency", 150*time.Millisecond)

	metrics := collector.GetMetrics()

	counters := metrics["counters"].(map[string]int64)
	assert.Equal(t, int64(100), counters["requests"])
	assert.Equal(t, int64(5), counters["errors"])

	gauges := metrics["gauges"].(map[string]float64)
	assert.Equal(t, 75.5, gauges["cpu"])

	timers := metrics["timers"].(map[string]interface{})
	latencyStats := timers["latency"].(map[string]interface{})
	assert.Equal(t, 3, latencyStats["count"])
	assert.Equal(t, float64(100), latencyStats["avg_ms"]) // (50+100+150)/3 = 100
	assert.Equal(t, float64(50), latencyStats["min_ms"])
	assert.Equal(t, float64(150), latencyStats["max_ms"])
}

func TestMetricsCollector_GetMetrics_SystemInfo(t *testing.T) {
	collector := NewMetricsCollector(nil).(*metricsCollector)

	metrics := collector.GetMetrics()

	system := metrics["system"].(map[string]interface{})
	assert.Contains(t, system, "uptime_seconds")
	assert.Contains(t, system, "timestamp")

	uptime := system["uptime_seconds"].(float64)
	assert.Greater(t, uptime, float64(0))

	timestamp := system["timestamp"].(int64)
	assert.Greater(t, timestamp, int64(0))
}

func TestMetricsCollector_GetMetrics_Concurrent(t *testing.T) {
	collector := NewMetricsCollector(nil).(*metricsCollector)

	var wg sync.WaitGroup
	done := make(chan bool, 10)

	// 启动多个 goroutine 同时读写
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			collector.IncrementCounter("concurrent_test", 1)
			collector.SetGauge("concurrent_gauge", float64(val))
			collector.RecordTimer("concurrent_timer", time.Duration(val)*time.Millisecond)
			collector.GetMetrics()
			done <- true
		}(i)
	}

	wg.Wait()
	close(done)

	// 验证所有操作都完成了
	metrics := collector.GetMetrics()
	counters := metrics["counters"].(map[string]int64)
	assert.Equal(t, int64(10), counters["concurrent_test"])
}

// ============== ServeHTTP 测试 ==============

func TestMetricsCollector_ServeHTTP_ContentType(t *testing.T) {
	collector := NewMetricsCollector(nil).(*metricsCollector)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()

	collector.ServeHTTP(rr, req)

	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestMetricsCollector_ServeHTTP_EmptyMetrics(t *testing.T) {
	collector := NewMetricsCollector(nil).(*metricsCollector)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()

	collector.ServeHTTP(rr, req)

	body := rr.Body.String()
	assert.Contains(t, body, `"metrics"`)
	assert.Contains(t, body, `"counters"`)
	assert.Contains(t, body, `"gauges"`)
	assert.Contains(t, body, `"timers"`)
	assert.Contains(t, body, `"system"`)
	assert.Contains(t, body, `"uptime_seconds"`)
	assert.Contains(t, body, `"timestamp"`)
}

func TestMetricsCollector_ServeHTTP_WithData(t *testing.T) {
	collector := NewMetricsCollector(nil).(*metricsCollector)

	collector.IncrementCounter("total_requests", 1000)
	collector.SetGauge("active_users", 50)
	collector.RecordTimer("response_time", 100*time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()

	collector.ServeHTTP(rr, req)

	body := rr.Body.String()
	assert.Contains(t, body, `"total_requests"`)
	assert.Contains(t, body, "1000")
	assert.Contains(t, body, `"active_users"`)
	assert.Contains(t, body, "50.0")
	assert.Contains(t, body, `"response_time"`)
}

func TestMetricsCollector_ServeHTTP_ValidJSON(t *testing.T) {
	collector := NewMetricsCollector(nil).(*metricsCollector)

	collector.IncrementCounter("requests", 10)
	collector.SetGauge("cpu", 75.5)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()

	collector.ServeHTTP(rr, req)

	body := rr.Body.String()
	// 验证 JSON 格式是否正确（简单的括号匹配检查）
	assert.Contains(t, body, "{")
	assert.Contains(t, body, "}")
	// 验证是有效的 JSON
	var result map[string]interface{}
	err := unmarshalJSON([]byte(body), &result)
	assert.NoError(t, err)
	assert.Contains(t, result, "metrics")
}

func TestMetricsCollector_ServeHTTP_MultipleEntries(t *testing.T) {
	collector := NewMetricsCollector(nil).(*metricsCollector)

	collector.IncrementCounter("counter1", 10)
	collector.IncrementCounter("counter2", 20)
	collector.IncrementCounter("counter3", 30)
	collector.SetGauge("gauge1", 1.1)
	collector.SetGauge("gauge2", 2.2)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()

	collector.ServeHTTP(rr, req)

	body := rr.Body.String()
	assert.Contains(t, body, `"counter1"`)
	assert.Contains(t, body, `"counter2"`)
	assert.Contains(t, body, `"counter3"`)
	assert.Contains(t, body, `"gauge1"`)
	assert.Contains(t, body, `"gauge2"`)
}

// 辅助函数：解析 JSON
func unmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
