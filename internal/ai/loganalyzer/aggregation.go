package loganalyzer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Aggregator 窗口聚合器
type Aggregator struct {
	config       *AggregatorConfig
	windowSize   time.Duration
	slideSize    time.Duration
	windows      map[int64]*Window
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	outputChan   chan<- *AggregatedResult
	logger       *zap.Logger
}

// AggregatorConfig 聚合器配置
type AggregatorConfig struct {
	WindowSize    time.Duration // 窗口大小
	SlideSize     time.Duration // 滑动步长
	MaxWindows    int           // 最大窗口数
	EnableRealtime bool         // 启用实时输出
}

// Window 时间窗口
type Window struct {
	ID         int64
	StartTime  time.Time
	EndTime    time.Time
	Entries    []*LogEntry
	Aggregated *AggregatedResult
	mu         sync.RWMutex
	closed     bool
}

// AggregatedResult 聚合结果
type AggregatedResult struct {
	WindowID    int64             `json:"window_id"`
	StartTime   time.Time         `json:"start_time"`
	EndTime     time.Time         `json:"end_time"`
	TotalCount  int64             `json:"total_count"`
	ByType      map[string]int64  `json:"by_type"`
	ByStatus    map[string]int64  `json:"by_status"`
	ByCountry   map[string]int64  `json:"by_country"`
	BySite      map[string]int64  `json:"by_site"`
	TopIPs      []IPStat          `json:"top_ips"`
	TopURIs     []URIStat         `json:"top_uris"`
	ThreatCount int64             `json:"threat_count"`
	ErrorCount  int64             `json:"error_count"`
	AvgLatency  float64           `json:"avg_latency"`
	P95Latency  float64           `json:"p95_latency"`
	P99Latency  float64           `json:"p99_latency"`
}

// IPStat IP 统计
type IPStat struct {
	IP     string `json:"ip"`
	Count  int64  `json:"count"`
	Bytes  int64  `json:"bytes"`
	Errors int64  `json:"errors"`
}

// URIStat URI 统计
type URIStat struct {
	URI    string `json:"uri"`
	Count  int64  `json:"count"`
	AvgTime float64 `json:"avg_time"`
}

// DefaultAggregatorConfig 返回默认配置
func DefaultAggregatorConfig() *AggregatorConfig {
	return &AggregatorConfig{
		WindowSize:     1 * time.Minute,
		SlideSize:      10 * time.Second,
		MaxWindows:     10,
		EnableRealtime: true,
	}
}

// NewAggregator 创建窗口聚合器
func NewAggregator(config *AggregatorConfig, outputChan chan<- *AggregatedResult, logger *zap.Logger) *Aggregator {
	if config == nil {
		config = DefaultAggregatorConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())

	agg := &Aggregator{
		config:     config,
		windowSize: config.WindowSize,
		slideSize:  config.SlideSize,
		windows:    make(map[int64]*Window),
		ctx:        ctx,
		cancel:     cancel,
		outputChan: outputChan,
		logger:     logger,
	}

	// 启动窗口清理协程
	agg.wg.Add(1)
	go agg.cleanupWorker()

	return agg
}

// Process 处理日志条目
func (a *Aggregator) Process(entry *LogEntry) {
	if entry == nil {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// 计算条目所属的窗口
	windowIDs := a.getWindowIDs(entry.Timestamp)

	for _, windowID := range windowIDs {
		window, exists := a.windows[windowID]
		if !exists {
			window = a.createWindow(windowID)
			a.windows[windowID] = window
		}

		window.mu.Lock()
		if !window.closed {
			window.Entries = append(window.Entries, entry)
		}
		window.mu.Unlock()
	}
}

// createWindow 创建窗口
func (a *Aggregator) createWindow(windowID int64) *Window {
	startTime := time.Unix(windowID, 0)
	return &Window{
		ID:        windowID,
		StartTime: startTime,
		EndTime:   startTime.Add(a.windowSize),
		Entries:   make([]*LogEntry, 0),
		Aggregated: &AggregatedResult{
			WindowID:  windowID,
			StartTime: startTime,
			EndTime:   startTime.Add(a.windowSize),
			ByType:    make(map[string]int64),
			ByStatus:  make(map[string]int64),
			ByCountry: make(map[string]int64),
			BySite:    make(map[string]int64),
			TopIPs:    make([]IPStat, 0),
			TopURIs:   make([]URIStat, 0),
		},
	}
}

// getWindowIDs 获取日志条目所属的窗口 ID 列表
func (a *Aggregator) getWindowIDs(t time.Time) []int64 {
	var ids []int64

	// 滑动窗口，一个条目可能属于多个窗口
	now := time.Now()
	windowStart := now.Add(-a.windowSize)

	for windowTime := windowStart; windowTime.Before(now); windowTime = windowTime.Add(a.slideSize) {
		windowID := windowTime.Unix()
		if t.After(windowTime) && t.Before(windowTime.Add(a.windowSize)) {
			ids = append(ids, windowID)
		}
	}

	return ids
}

// cleanupWorker 清理过期窗口
func (a *Aggregator) cleanupWorker() {
	defer a.wg.Done()

	ticker := time.NewTicker(a.slideSize)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.cleanupExpiredWindows()
		}
	}
}

// cleanupExpiredWindows 清理过期窗口
func (a *Aggregator) cleanupExpiredWindows() {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-a.windowSize * 2)

	for id, window := range a.windows {
		if window.EndTime.Before(cutoff) {
			// 窗口已过期，输出结果并删除
			if !window.closed {
				a.aggregateWindow(window)
				window.closed = true

				// 输出结果
				if a.outputChan != nil {
					select {
					case a.outputChan <- window.Aggregated:
					case <-a.ctx.Done():
					default:
						a.logger.Warn("输出通道已满，丢弃聚合结果")
					}
				}
			}

			// 限制窗口数量
			if len(a.windows) > a.config.MaxWindows {
				delete(a.windows, id)
			}
		}
	}
}

// aggregateWindow 聚合窗口数据
func (a *Aggregator) aggregateWindow(window *Window) {
	window.mu.Lock()
	defer window.mu.Unlock()

	if window.closed {
		return
	}

	agg := window.Aggregated
	agg.TotalCount = int64(len(window.Entries))

	ipStats := make(map[string]*IPStat)
	uriStats := make(map[string]*URIStat)
	latencies := make([]float64, 0)

	for _, entry := range window.Entries {
		// 按类型统计
		agg.ByType[entry.SourceType]++

		// 按状态码统计
		if status, ok := entry.Fields["status"]; ok {
			statusStr := toString(status)
			agg.ByStatus[statusStr]++

			// 统计错误
			if statusInt := toInt(status); statusInt >= 400 {
				agg.ErrorCount++
			}
		}

		// 按国家统计
		if country, ok := entry.Fields["country"]; ok {
			agg.ByCountry[toString(country)]++
		}

		// 按站点统计
		if siteID, ok := entry.Fields["site_id"]; ok {
			agg.BySite[toString(siteID)]++
		}

		// IP 统计
		if ip, ok := entry.Fields["remote_addr"]; ok {
			ipStr := toString(ip)
			if _, exists := ipStats[ipStr]; !exists {
				ipStats[ipStr] = &IPStat{IP: ipStr}
			}
			ipStats[ipStr].Count++

			if bytes, ok := entry.Fields["body_bytes"]; ok {
				ipStats[ipStr].Bytes += toInt64(bytes)
			}
		}

		// URI 统计
		if uri, ok := entry.Fields["uri"]; ok {
			uriStr := toString(uri)
			if _, exists := uriStats[uriStr]; !exists {
				uriStats[uriStr] = &URIStat{URI: uriStr}
			}
			uriStats[uriStr].Count++

			if reqTime, ok := entry.Fields["request_time"]; ok {
				uriStats[uriStr].AvgTime += toFloat64(reqTime)
			}
		}

		// 延迟统计
		if reqTime, ok := entry.Fields["request_time"]; ok {
			latencies = append(latencies, toFloat64(reqTime))
		}

		// 威胁统计
		if threatType, ok := entry.Fields["threat_type"]; ok {
			if toString(threatType) != "" {
				agg.ThreatCount++
			}
		}
	}

	// 计算 Top IPs
	agg.TopIPs = topIPs(ipStats, 10)

	// 计算 Top URIs
	agg.TopURIs = topURIs(uriStats, 10)

	// 计算延迟百分位
	if len(latencies) > 0 {
		agg.AvgLatency = average(latencies)
		agg.P95Latency = percentile(latencies, 0.95)
		agg.P99Latency = percentile(latencies, 0.99)
	}

	a.logger.Debug("窗口聚合完成",
		zap.Int64("window_id", window.ID),
		zap.Int64("total_count", agg.TotalCount),
		zap.Int64("threat_count", agg.ThreatCount))
}

// GetWindowStats 获取窗口统计
func (a *Aggregator) GetWindowStats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return map[string]interface{}{
		"window_count":  len(a.windows),
		"window_size":   a.windowSize.String(),
		"slide_size":    a.slideSize.String(),
	}
}

// Close 关闭聚合器
func (a *Aggregator) Close() {
	a.cancel()
	a.wg.Wait()

	// 输出所有剩余窗口
	a.mu.Lock()
	for _, window := range a.windows {
		if !window.closed {
			a.aggregateWindow(window)
			if a.outputChan != nil {
				a.outputChan <- window.Aggregated
			}
		}
	}
	a.mu.Unlock()
}

// 辅助函数

func topIPs(stats map[string]*IPStat, limit int) []IPStat {
	result := make([]IPStat, 0, limit)
	for _, s := range stats {
		result = append(result, *s)
	}

	// 简单排序（按 Count 降序）
	for i := 0; i < len(result) && i < limit; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Count > result[i].Count {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	if len(result) > limit {
		result = result[:limit]
	}

	return result
}

func topURIs(stats map[string]*URIStat, limit int) []URIStat {
	result := make([]URIStat, 0, limit)
	for _, s := range stats {
		// 计算平均值
		if s.Count > 0 {
			s.AvgTime = s.AvgTime / float64(s.Count)
		}
		result = append(result, *s)
	}

	// 简单排序（按 Count 降序）
	for i := 0; i < len(result) && i < limit; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Count > result[i].Count {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	if len(result) > limit {
		result = result[:limit]
	}

	return result
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// 排序
	sorted := make([]float64, len(values))
	copy(sorted, values)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	idx := int(float64(len(values)-1) * p)
	return sorted[idx]
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case fmt.Stringer:
		return val.String()
	default:
		return fmt.Sprintf("%v", val)
	}
}

func toInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case string:
		var i int
		fmt.Sscanf(val, "%d", &i)
		return i
	default:
		return 0
	}
}

func toInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return int64(val)
	case int64:
		return val
	case float64:
		return int64(val)
	case string:
		var i int64
		fmt.Sscanf(val, "%d", &i)
		return i
	default:
		return 0
	}
}

func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	default:
		return 0
	}
}
