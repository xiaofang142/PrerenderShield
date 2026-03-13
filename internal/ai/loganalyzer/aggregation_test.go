package loganalyzer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultAggregatorConfig(t *testing.T) {
	config := DefaultAggregatorConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 1*time.Minute, config.WindowSize)
	assert.Equal(t, 10*time.Second, config.SlideSize)
	assert.Equal(t, 10, config.MaxWindows)
	assert.True(t, config.EnableRealtime)
}

func TestNewAggregator(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	outputChan := make(chan *AggregatedResult, 10)
	config := &AggregatorConfig{
		WindowSize:     30 * time.Second,
		SlideSize:      5 * time.Second,
		MaxWindows:     3,
		EnableRealtime: true,
	}

	agg := NewAggregator(config, outputChan, logger)
	assert.NotNil(t, agg)
	assert.Equal(t, config.WindowSize, agg.windowSize)
	assert.Equal(t, config.SlideSize, agg.slideSize)
	assert.NotNil(t, agg.windows)
	assert.NotNil(t, agg.ctx)
	assert.NotNil(t, agg.cancel)

	// 测试 nil config
	aggNil := NewAggregator(nil, nil, nil)
	assert.NotNil(t, aggNil)
	assert.Equal(t, 1*time.Minute, aggNil.windowSize)

	agg.Close()
	aggNil.Close()
}

func TestAggregator_createWindow(t *testing.T) {
	outputChan := make(chan *AggregatedResult, 10)
	config := &AggregatorConfig{
		WindowSize: 1 * time.Minute,
		SlideSize:  10 * time.Second,
		MaxWindows: 5,
	}
	agg := NewAggregator(config, outputChan, nil)
	defer agg.Close()

	window := agg.createWindow(1000)
	assert.NotNil(t, window)
	assert.Equal(t, int64(1000), window.ID)
	assert.Equal(t, time.Unix(1000, 0), window.StartTime)
	assert.Equal(t, time.Unix(1000, 0).Add(1*time.Minute), window.EndTime)
	assert.Empty(t, window.Entries)
	assert.NotNil(t, window.Aggregated)
}

func TestAggregator_getWindowIDs(t *testing.T) {
	outputChan := make(chan *AggregatedResult, 10)
	config := &AggregatorConfig{
		WindowSize: 1 * time.Minute,
		SlideSize:  10 * time.Second,
		MaxWindows: 5,
	}
	agg := NewAggregator(config, outputChan, nil)
	defer agg.Close()

	now := time.Now()
	ids := agg.getWindowIDs(now)
	assert.NotEmpty(t, ids)
}

func TestAggregator_cleanupExpiredWindows(t *testing.T) {
	outputChan := make(chan *AggregatedResult, 10)
	config := &AggregatorConfig{
		WindowSize: 500 * time.Millisecond,
		SlideSize:  100 * time.Millisecond,
		MaxWindows: 3,
	}
	agg := NewAggregator(config, outputChan, nil)

	// 创建一个过期窗口
	pastTime := time.Now().Add(-2 * time.Second)
	windowID := pastTime.Unix()
	window := agg.createWindow(windowID)
	window.closed = false
	agg.windows[windowID] = window

	// 触发清理
	agg.cleanupExpiredWindows()

	// 窗口应该被标记为已关闭
	assert.True(t, window.closed)

	agg.Close()
}

func TestAggregator_aggregateWindow(t *testing.T) {
	outputChan := make(chan *AggregatedResult, 10)
	config := &AggregatorConfig{
		WindowSize: 1 * time.Minute,
		SlideSize:  10 * time.Second,
		MaxWindows: 5,
	}
	agg := NewAggregator(config, outputChan, nil)
	defer agg.Close()

	window := agg.createWindow(time.Now().Unix())
	window.Entries = []*LogEntry{
		{
			SourceType: "access",
			Fields: map[string]interface{}{
				"remote_addr":   "192.168.1.1",
				"status":        "200",
				"uri":           "/api/test",
				"country":       "US",
				"site_id":       "site1",
				"body_bytes":    1024,
				"request_time":  0.1,
			},
		},
		{
			SourceType: "access",
			Fields: map[string]interface{}{
				"remote_addr":   "192.168.1.2",
				"status":        "404",
				"uri":           "/api/test",
				"country":       "CN",
				"site_id":       "site1",
				"body_bytes":    512,
				"request_time":  0.2,
			},
		},
		{
			SourceType: "security",
			Fields: map[string]interface{}{
				"remote_addr":   "192.168.1.3",
				"threat_type":   "sql_injection",
				"status":        "403",
				"uri":           "/api/admin",
			},
		},
	}

	agg.aggregateWindow(window)

	result := window.Aggregated
	assert.Equal(t, int64(3), result.TotalCount)
	assert.Equal(t, int64(2), result.ByType["access"])
	assert.Equal(t, int64(1), result.ByType["security"])
	// 状态码统计：200 有 1 个，404 有 1 个，403 有 1 个
	assert.Equal(t, int64(1), result.ByStatus["200"])
	assert.Equal(t, int64(1), result.ByStatus["404"])
	assert.Equal(t, int64(2), result.ErrorCount) // 404 和 403 都是错误
	assert.Equal(t, int64(1), result.ThreatCount)
	assert.NotEmpty(t, result.TopIPs)
	assert.NotEmpty(t, result.TopURIs)
}

func TestAggregator_aggregateWindow_Empty(t *testing.T) {
	outputChan := make(chan *AggregatedResult, 10)
	config := &AggregatorConfig{
		WindowSize: 1 * time.Minute,
		SlideSize:  10 * time.Second,
		MaxWindows: 5,
	}
	agg := NewAggregator(config, outputChan, nil)
	defer agg.Close()

	window := agg.createWindow(time.Now().Unix())
	window.closed = true

	// 已关闭的窗口不应该被聚合
	agg.aggregateWindow(window)
	assert.Equal(t, int64(0), window.Aggregated.TotalCount)
}

func TestAggregator_Close(t *testing.T) {
	outputChan := make(chan *AggregatedResult, 10)
	config := &AggregatorConfig{
		WindowSize: 1 * time.Minute,
		SlideSize:  10 * time.Second,
		MaxWindows: 5,
	}
	agg := NewAggregator(config, outputChan, nil)

	// 添加一些数据
	entry := &LogEntry{
		SourceType: "access",
		Timestamp:  time.Now(),
		Fields: map[string]interface{}{
			"remote_addr": "192.168.1.1",
			"status":      "200",
		},
	}
	agg.Process(entry)

	// 关闭应该输出所有剩余窗口
	agg.Close()
}

func TestTopIPs(t *testing.T) {
	stats := map[string]*IPStat{
		"192.168.1.1": {IP: "192.168.1.1", Count: 10},
		"192.168.1.2": {IP: "192.168.1.2", Count: 30},
		"192.168.1.3": {IP: "192.168.1.3", Count: 20},
	}

	result := topIPs(stats, 2)
	assert.Len(t, result, 2)
	assert.Equal(t, "192.168.1.2", result[0].IP)
	assert.Equal(t, "192.168.1.3", result[1].IP)
}

func TestTopIPs_Limit(t *testing.T) {
	stats := map[string]*IPStat{
		"192.168.1.1": {IP: "192.168.1.1", Count: 10},
	}

	result := topIPs(stats, 5)
	assert.Len(t, result, 1)
}

func TestTopIPs_Empty(t *testing.T) {
	stats := map[string]*IPStat{}
	result := topIPs(stats, 5)
	assert.Empty(t, result)
}

func TestTopURIs(t *testing.T) {
	stats := map[string]*URIStat{
		"/api/test":  {URI: "/api/test", Count: 10, AvgTime: 100},
		"/api/admin": {URI: "/api/admin", Count: 30, AvgTime: 200},
		"/api/users": {URI: "/api/users", Count: 20, AvgTime: 150},
	}

	result := topURIs(stats, 2)
	assert.Len(t, result, 2)
	assert.Equal(t, "/api/admin", result[0].URI)
	assert.Equal(t, "/api/users", result[1].URI)
	// 检查 AvgTime 是否被正确计算为平均值
	assert.Equal(t, 200.0/30, result[0].AvgTime)
}

func TestTopURIs_Limit(t *testing.T) {
	stats := map[string]*URIStat{
		"/api/test": {URI: "/api/test", Count: 10, AvgTime: 100},
	}

	result := topURIs(stats, 5)
	assert.Len(t, result, 1)
}

func TestTopURIs_Empty(t *testing.T) {
	stats := map[string]*URIStat{}
	result := topURIs(stats, 5)
	assert.Empty(t, result)
}

func TestAverage(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	assert.Equal(t, 3.0, average(values))

	// 空数组
	assert.Equal(t, 0.0, average([]float64{}))

	// 单个值
	assert.Equal(t, 5.0, average([]float64{5}))
}

func TestPercentile(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	// percentile 使用 idx := int(float64(len(values)-1) * p)
	// p=0.4: idx = int(9 * 0.4) = int(3.6) = 3, values[3] = 4
	assert.Equal(t, 4.0, percentile(values, 0.4))
	// p=0.9: idx = int(9 * 0.9) = int(8.1) = 8, values[8] = 9
	assert.Equal(t, 9.0, percentile(values, 0.9))

	// 空数组
	assert.Equal(t, 0.0, percentile([]float64{}, 0.5))

	// 单个值
	assert.Equal(t, 5.0, percentile([]float64{5}, 0.5))
}

func TestToString_NilCases(t *testing.T) {
	assert.Equal(t, "", toString(nil))
	assert.Equal(t, "test", toString("test"))
}

func TestToInt_EdgeCases(t *testing.T) {
	assert.Equal(t, 0, toInt(nil))
	assert.Equal(t, 0, toInt("invalid"))
}

func TestToInt64_EdgeCases(t *testing.T) {
	assert.Equal(t, int64(0), toInt64(nil))
	assert.Equal(t, int64(0), toInt64("invalid"))
}

func TestToFloat64_EdgeCases(t *testing.T) {
	assert.Equal(t, 0.0, toFloat64(nil))
	assert.Equal(t, 0.0, toFloat64("invalid"))
}

func TestWindow(t *testing.T) {
	window := &Window{
		ID:        1,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(1 * time.Minute),
		Entries:   make([]*LogEntry, 0),
		closed:    false,
	}

	assert.Equal(t, int64(1), window.ID)
	assert.False(t, window.closed)
}

func TestAggregatedResult(t *testing.T) {
	result := &AggregatedResult{
		WindowID:   1,
		StartTime:  time.Now(),
		EndTime:    time.Now().Add(1 * time.Minute),
		TotalCount: 100,
		ByType:     map[string]int64{"access": 80, "security": 20},
		ByStatus:   map[string]int64{"200": 90, "404": 10},
		TopIPs:     []IPStat{{IP: "192.168.1.1", Count: 50}},
		TopURIs:    []URIStat{{URI: "/api/test", Count: 30}},
	}

	assert.Equal(t, int64(100), result.TotalCount)
	assert.Equal(t, int64(80), result.ByType["access"])
}

func TestIPStat(t *testing.T) {
	stat := IPStat{
		IP:     "192.168.1.1",
		Count:  100,
		Bytes:  10240,
		Errors: 5,
	}

	assert.Equal(t, "192.168.1.1", stat.IP)
	assert.Equal(t, int64(100), stat.Count)
}

func TestURIStat(t *testing.T) {
	stat := URIStat{
		URI:     "/api/test",
		Count:   50,
		AvgTime: 0.15,
	}

	assert.Equal(t, "/api/test", stat.URI)
	assert.Equal(t, int64(50), stat.Count)
}
