package cache

import (
	"sync"
	"time"
)

// Stats 缓存统计
type Stats struct {
	Hits        int64   `json:"hits"`
	Misses      int64   `json:"misses"`
	Evictions   int64   `json:"evictions"`
	Size        int     `json:"size"`
	MaxSize     int     `json:"max_size"`
	HitRate     float64 `json:"hit_rate"`
	AvgGetTime  float64 `json:"avg_get_time"`
	AvgSetTime  float64 `json:"avg_set_time"`
	TotalKeys   int     `json:"total_keys"`
	MemoryUsage int64   `json:"memory_usage"`
}

// StatsCollector 统计收集器
type StatsCollector struct {
	hits      int64
	misses    int64
	evictions int64
	size      int
	maxSize   int
	getTimes  []time.Duration
	setTimes  []time.Duration
	mu        sync.RWMutex
	startTime time.Time
}

// NewStatsCollector 创建统计收集器
func NewStatsCollector(maxSize int) *StatsCollector {
	return &StatsCollector{
		maxSize:   maxSize,
		getTimes:  make([]time.Duration, 0, 1000),
		setTimes:  make([]time.Duration, 0, 1000),
		startTime: time.Now(),
	}
}

// RecordHit 记录命中
func (sc *StatsCollector) RecordHit() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.hits++
}

// RecordMiss 记录未命中
func (sc *StatsCollector) RecordMiss() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.misses++
}

// RecordEviction 记录驱逐
func (sc *StatsCollector) RecordEviction() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.evictions++
}

// SetSize 设置当前大小
func (sc *StatsCollector) SetSize(size int) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.size = size
}

// RecordGetTime 记录获取耗时
func (sc *StatsCollector) RecordGetTime(d time.Duration) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if len(sc.getTimes) >= 1000 {
		// 保留最近1000条记录
		sc.getTimes = sc.getTimes[1:]
	}
	sc.getTimes = append(sc.getTimes, d)
}

// RecordSetTime 记录设置耗时
func (sc *StatsCollector) RecordSetTime(d time.Duration) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if len(sc.setTimes) >= 1000 {
		sc.setTimes = sc.setTimes[1:]
	}
	sc.setTimes = append(sc.setTimes, d)
}

// GetStats 获取统计信息
func (sc *StatsCollector) GetStats() Stats {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	total := sc.hits + sc.misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(sc.hits) / float64(total) * 100
	}

	return Stats{
		Hits:       sc.hits,
		Misses:     sc.misses,
		Evictions:  sc.evictions,
		Size:       sc.size,
		MaxSize:    sc.maxSize,
		HitRate:    hitRate,
		AvgGetTime: sc.avgDuration(sc.getTimes),
		AvgSetTime: sc.avgDuration(sc.setTimes),
		TotalKeys:  sc.size,
	}
}

// avgDuration 计算平均耗时
func (sc *StatsCollector) avgDuration(durations []time.Duration) float64 {
	if len(durations) == 0 {
		return 0
	}

	var total time.Duration
	for _, d := range durations {
		total += d
	}

	return float64(total.Milliseconds()) / float64(len(durations))
}

// Reset 重置统计
func (sc *StatsCollector) Reset() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.hits = 0
	sc.misses = 0
	sc.evictions = 0
	sc.getTimes = sc.getTimes[:0]
	sc.setTimes = sc.setTimes[:0]
	sc.startTime = time.Now()
}

// Uptime 运行时间
func (sc *StatsCollector) Uptime() time.Duration {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return time.Since(sc.startTime)
}
