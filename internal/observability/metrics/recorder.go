package metrics

import (
	"strconv"
	"sync"
	"time"
)

// Recorder 指标记录器接口
type Recorder interface {
	RecordRequestDuration(method, path string, status int, duration time.Duration)
	RecordRequestCount(method, path string, status int)
	RecordCacheHit(siteID string)
	RecordCacheMiss(siteID string)
	RecordWAFBlock(siteID, threatType string)
	RecordError(module, errorType string)
	GetMetrics() map[string]interface{}
}

// InMemoryRecorder 内存指标记录器
type InMemoryRecorder struct {
	mu sync.RWMutex

	// 详细数据（用于计算平均值）
	requestDuration map[string][]time.Duration

	// 运行计数器（避免 GetMetrics 时遍历）
	totalRequests    int64
	totalCacheHits   int64
	totalCacheMisses int64
	totalWAFBlocks   int64
	totalErrors      int64

	// 详细分类数据
	requestCount map[string]int64
	cacheHits    map[string]int64
	cacheMisses  map[string]int64
	wafBlocks    map[string]int64
	errors       map[string]int64

	startTime time.Time
}

// NewInMemoryRecorder 创建内存指标记录器
func NewInMemoryRecorder() *InMemoryRecorder {
	return &InMemoryRecorder{
		requestDuration: make(map[string][]time.Duration),
		requestCount:    make(map[string]int64),
		cacheHits:       make(map[string]int64),
		cacheMisses:     make(map[string]int64),
		wafBlocks:       make(map[string]int64),
		errors:          make(map[string]int64),
		startTime:       time.Now(),
	}
}

func (r *InMemoryRecorder) RecordRequestDuration(method, path string, status int, duration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := metricKey(method, path, status)
	r.requestDuration[key] = append(r.requestDuration[key], duration)

	if len(r.requestDuration[key]) > 1000 {
		r.requestDuration[key] = r.requestDuration[key][1:]
	}
}

func (r *InMemoryRecorder) RecordRequestCount(method, path string, status int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := metricKey(method, path, status)
	r.requestCount[key]++
	r.totalRequests++
}

func (r *InMemoryRecorder) RecordCacheHit(siteID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cacheHits[siteID]++
	r.totalCacheHits++
}

func (r *InMemoryRecorder) RecordCacheMiss(siteID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cacheMisses[siteID]++
	r.totalCacheMisses++
}

func (r *InMemoryRecorder) RecordWAFBlock(siteID, threatType string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := siteID + ":" + threatType
	r.wafBlocks[key]++
	r.totalWAFBlocks++
}

func (r *InMemoryRecorder) RecordError(module, errorType string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := module + ":" + errorType
	r.errors[key]++
	r.totalErrors++
}

func (r *InMemoryRecorder) GetMetrics() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metrics := make(map[string]interface{})

	metrics["uptime_seconds"] = int64(time.Since(r.startTime).Seconds())
	metrics["total_requests"] = r.totalRequests
	metrics["cache_hits"] = r.totalCacheHits
	metrics["cache_misses"] = r.totalCacheMisses
	metrics["waf_blocks"] = r.totalWAFBlocks
	metrics["total_errors"] = r.totalErrors

	// 计算缓存命中率
	totalCache := r.totalCacheHits + r.totalCacheMisses
	if totalCache > 0 {
		metrics["cache_hit_rate"] = float64(r.totalCacheHits) / float64(totalCache)
	}

	metrics["avg_response_time_ms"] = r.calculateAvgResponseTime()

	return metrics
}

func (r *InMemoryRecorder) calculateAvgResponseTime() float64 {
	total := 0
	count := 0
	for _, durations := range r.requestDuration {
		for _, d := range durations {
			total += int(d.Milliseconds())
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return float64(total) / float64(count)
}

func metricKey(method, path string, status int) string {
	return method + ":" + path + ":" + strconv.Itoa(status)
}
