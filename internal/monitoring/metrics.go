package monitoring

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"prerender-shield/internal/redis"
)

// MetricsCollector 指标收集器接口
type MetricsCollector interface {
	IncrementCounter(name string, value int64)
	SetGauge(name string, value float64)
	RecordTimer(name string, duration time.Duration)
	GetMetrics() map[string]interface{}
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// metricsCollector 指标收集器实现
type metricsCollector struct {
	redisClient *redis.Client
	counters    map[string]int64
	gauges      map[string]float64
	timers      map[string][]time.Duration
	mutex       sync.RWMutex
	startTime   time.Time
}

// NewMetricsCollector 创建新的指标收集器
func NewMetricsCollector(redisClient *redis.Client) MetricsCollector {
	return &metricsCollector{
		redisClient: redisClient,
		counters:    make(map[string]int64),
		gauges:      make(map[string]float64),
		timers:      make(map[string][]time.Duration),
		startTime:   time.Now(),
	}
}

// IncrementCounter 增加计数器
func (m *metricsCollector) IncrementCounter(name string, value int64) {
	m.mutex.Lock()
	m.counters[name] += value
	m.mutex.Unlock()

	// 可选：持久化到Redis
	if m.redisClient != nil {
		key := fmt.Sprintf("metrics:counter:%s", name)
		m.redisClient.Incr(key)
	}
}

// SetGauge 设置 gauge 值
func (m *metricsCollector) SetGauge(name string, value float64) {
	m.mutex.Lock()
	m.gauges[name] = value
	m.mutex.Unlock()

	// 可选：持久化到Redis
	if m.redisClient != nil {
		key := fmt.Sprintf("metrics:gauge:%s", name)
		m.redisClient.Set(key, fmt.Sprintf("%f", value), 5*time.Minute)
	}
}

// RecordTimer 记录计时器值
func (m *metricsCollector) RecordTimer(name string, duration time.Duration) {
	m.mutex.Lock()
	if _, ok := m.timers[name]; !ok {
		m.timers[name] = make([]time.Duration, 0, 100)
	}
	m.timers[name] = append(m.timers[name], duration)
	// 限制计时器数据量
	if len(m.timers[name]) > 100 {
		m.timers[name] = m.timers[name][len(m.timers[name])-100:]
	}
	m.mutex.Unlock()
}

// GetMetrics 获取所有指标
func (m *metricsCollector) GetMetrics() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	metrics := make(map[string]interface{})

	// 计数器
	metrics["counters"] = m.counters

	// Gauges
	metrics["gauges"] = m.gauges

	// 计时器统计
	timerStats := make(map[string]interface{})
	for name, durations := range m.timers {
		if len(durations) > 0 {
			avg := time.Duration(0)
			min := durations[0]
			max := durations[0]

			for _, d := range durations {
				avg += d
				if d < min {
					min = d
				}
				if d > max {
					max = d
				}
			}

			avg /= time.Duration(len(durations))

			timerStats[name] = map[string]interface{}{
				"count":  len(durations),
				"avg_ms": float64(avg.Milliseconds()),
				"min_ms": float64(min.Milliseconds()),
				"max_ms": float64(max.Milliseconds()),
			}
		}
	}
	metrics["timers"] = timerStats

	// 系统指标
	metrics["system"] = map[string]interface{}{
		"uptime_seconds": time.Since(m.startTime).Seconds(),
		"timestamp":      time.Now().Unix(),
	}

	return metrics
}

// ServeHTTP 处理指标请求
func (m *metricsCollector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	metrics := m.GetMetrics()

	w.Header().Set("Content-Type", "application/json")

	// 构建JSON响应
	response := `{
	"metrics": {
`

	// 计数器
	response += `		"counters": {
`
	first := true
	for name, value := range metrics["counters"].(map[string]int64) {
		if !first {
			response += `,`
		}
		first = false
		response += fmt.Sprintf(`			"%s": %d`, name, value)
	}
	response += `
		},
`

	// Gauges
	response += `		"gauges": {
`
	first = true
	for name, value := range metrics["gauges"].(map[string]float64) {
		if !first {
			response += `,`
		}
		first = false
		response += fmt.Sprintf(`			"%s": %f`, name, value)
	}
	response += `
		},
`

	// 计时器
	response += `		"timers": {
`
	first = true
	for name, stats := range metrics["timers"].(map[string]interface{}) {
		if !first {
			response += `,`
		}
		first = false
		timerStat := stats.(map[string]interface{})
		response += fmt.Sprintf(`			"%s": {
				"count": %d,
				"avg_ms": %f,
				"min_ms": %f,
				"max_ms": %f
			}`,
			name,
			timerStat["count"],
			timerStat["avg_ms"],
			timerStat["min_ms"],
			timerStat["max_ms"])
	}
	response += `
		},
`

	// 系统指标
	system := metrics["system"].(map[string]interface{})
	response += fmt.Sprintf(`		"system": {
			"uptime_seconds": %f,
			"timestamp": %d
		}
`,
		system["uptime_seconds"],
		system["timestamp"])

	response += `
	}
}`

	w.Write([]byte(response))
}
