package loganalyzer

import (
	"context"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// StatisticalDetector 统计异常检测器
type StatisticalDetector struct {
	config     *StatisticalConfig
	metrics    map[string]*MetricStats
	mu         sync.RWMutex
	minSamples int
}

// StatisticalConfig 统计检测配置
type StatisticalConfig struct {
	ZScoreThreshold    float64 // Z-Score 阈值
	ModifiedZThreshold float64 // 改进 Z-Score 阈值
	IQRMultiplier      float64 // IQR 倍数
	EnableIQR          bool    // 是否启用 IQR 检测
	EnableZScore       bool    // 是否启用 Z-Score 检测
}

// MetricStats 指标统计
type MetricStats struct {
	Count    int64
	Sum      float64
	SumSq    float64
	Min      float64
	Max      float64
	Mean     float64
	StdDev   float64
	Q1       float64 // 第一四分位数
	Q3       float64 // 第三四分位数
	IQR      float64 // 四分位距
	Values   []float64 // 用于计算分位数
	lastUpdate time.Time
}

// StatisticalResult 统计检测结果
type StatisticalResult struct {
	Method      string  // 检测方法
	ZScore      float64 // Z-Score
	ModifiedZ   float64 // 改进 Z-Score
	IsIQRNormal bool    // IQR 检测是否正常
	IsZNormal   bool    // Z-Score 检测是否正常
	IsAnomaly   bool    // 是否异常
	Severity    string  // 严重程度
}

// DefaultStatisticalConfig 返回默认配置
func DefaultStatisticalConfig() *StatisticalConfig {
	return &StatisticalConfig{
		ZScoreThreshold:    3.0,
		ModifiedZThreshold: 3.5,
		IQRMultiplier:      1.5,
		EnableIQR:          true,
		EnableZScore:       true,
	}
}

// NewStatisticalDetector 创建统计异常检测器
func NewStatisticalDetector(config *StatisticalConfig) *StatisticalDetector {
	if config == nil {
		config = DefaultStatisticalConfig()
	}

	return &StatisticalDetector{
		config:     config,
		metrics:    make(map[string]*MetricStats),
		minSamples: 30, // 最少需要 30 个样本
	}
}

// Process 处理日志条目
func (d *StatisticalDetector) Process(ctx context.Context, entry *LogEntry) (*LogEntry, error) {
	if entry == nil {
		return nil, nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// 提取数值指标
	metrics := d.extractMetrics(entry)

	results := make([]StatisticalResult, 0)

	for metricName, value := range metrics {
		// 更新统计
		stats := d.updateStats(metricName, value)

		// 如果样本不足，跳过检测
		if stats.Count < int64(d.minSamples) {
			continue
		}

		// 检测异常
		result := d.detectAnomaly(metricName, value, stats)
		if result.IsAnomaly {
			results = append(results, result)
		}
	}

	// 如果有异常，标记日志条目
	if len(results) > 0 {
		entry.Fields["statistical_anomalies"] = results
		entry.Fields["has_statistical_anomaly"] = true

		// 设置严重程度
		maxSeverity := "low"
		for _, r := range results {
			if r.Severity == "critical" {
				maxSeverity = "critical"
				break
			}
			if r.Severity == "high" {
				maxSeverity = "high"
			}
		}
		entry.Fields["anomaly_severity"] = maxSeverity

		if maxSeverity == "critical" || maxSeverity == "high" {
			entry.Level = "error"
		} else {
			entry.Level = "warn"
		}
	}

	return entry, nil
}

// extractMetrics 提取数值指标
func (d *StatisticalDetector) extractMetrics(entry *LogEntry) map[string]float64 {
	metrics := make(map[string]float64)

	// 请求延迟
	if reqTime, ok := entry.Fields["request_time_ms"]; ok {
		metrics["request_latency"] = toFloat64(reqTime)
	}

	// 响应大小
	if bytes, ok := entry.Fields["body_bytes_int"]; ok {
		metrics["response_size"] = float64(toInt64(bytes))
	}

	// 状态码
	if status, ok := entry.Fields["status_int"]; ok {
		metrics["status_code"] = float64(toInt(status))
	}

	// 渲染时间
	if renderTime, ok := entry.Fields["render_time"]; ok {
		metrics["render_time"] = toFloat64(renderTime)
	}

	// 威胁评分
	if threatScore, ok := entry.Fields["threat_score"]; ok {
		metrics["threat_score"] = toFloat64(threatScore)
	}

	return metrics
}

// updateStats 更新统计
func (d *StatisticalDetector) updateStats(name string, value float64) *MetricStats {
	stats, ok := d.metrics[name]
	if !ok {
		stats = &MetricStats{
			Min:    value,
			Max:    value,
			Values: make([]float64, 0, 1000),
		}
		d.metrics[name] = stats
	}

	// 更新基础统计
	stats.Count++
	stats.Sum += value
	stats.SumSq += value * value

	if value < stats.Min {
		stats.Min = value
	}
	if value > stats.Max {
		stats.Max = value
	}

	// 更新均值
	stats.Mean = stats.Sum / float64(stats.Count)

	// 更新标准差
	if stats.Count > 1 {
		variance := (stats.SumSq - (stats.Sum * stats.Sum / float64(stats.Count))) / float64(stats.Count-1)
		if variance > 0 {
			stats.StdDev = math.Sqrt(variance)
		}
	}

	// 保存值用于分位数计算（限制大小）
	if len(stats.Values) < 1000 {
		stats.Values = append(stats.Values, value)
	} else {
		// 随机替换一个旧值
		idx := rand.Intn(len(stats.Values))
		stats.Values[idx] = value
	}

	stats.lastUpdate = time.Now()

	return stats
}

// detectAnomaly 检测异常
func (d *StatisticalDetector) detectAnomaly(metric string, value float64, stats *MetricStats) StatisticalResult {
	result := StatisticalResult{
		Method:      "statistical",
		IsIQRNormal: true,
		IsZNormal:   true,
		IsAnomaly:   false,
		Severity:    "low",
	}

	// Z-Score 检测
	if d.config.EnableZScore && stats.StdDev > 0 {
		zScore := (value - stats.Mean) / stats.StdDev
		result.ZScore = zScore

		if math.Abs(zScore) > d.config.ZScoreThreshold {
			result.IsZNormal = false
			result.IsAnomaly = true

			// 根据 Z-Score 设置严重程度
			if math.Abs(zScore) > 5 {
				result.Severity = "critical"
			} else if math.Abs(zScore) > 4 {
				result.Severity = "high"
			} else {
				result.Severity = "medium"
			}
		}
	}

	// IQR 检测
	if d.config.EnableIQR && len(stats.Values) >= 10 {
		// 计算四分位数
		sorted := make([]float64, len(stats.Values))
		copy(sorted, stats.Values)
		sort.Float64s(sorted)

		q1Idx := len(sorted) / 4
		q3Idx := 3 * len(sorted) / 4

		stats.Q1 = sorted[q1Idx]
		stats.Q3 = sorted[q3Idx]
		stats.IQR = stats.Q3 - stats.Q1

		lowerBound := stats.Q1 - d.config.IQRMultiplier*stats.IQR
		upperBound := stats.Q3 + d.config.IQRMultiplier*stats.IQR

		if value < lowerBound || value > upperBound {
			result.IsIQRNormal = false
			result.IsAnomaly = true

			if result.Severity == "low" {
				result.Severity = "medium"
			}
		}
	}

	return result
}

// GetMetricStats 获取指标统计
func (d *StatisticalDetector) GetMetricStats(metric string) (*MetricStats, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	stats, ok := d.metrics[metric]
	return stats, ok
}

// GetAllMetrics 获取所有指标统计
func (d *StatisticalDetector) GetAllMetrics() map[string]*MetricStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make(map[string]*MetricStats)
	for k, v := range d.metrics {
		result[k] = v
	}
	return result
}

// Reset 重置统计
func (d *StatisticalDetector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.metrics = make(map[string]*MetricStats)
}

// GetStats 获取检测器统计
func (d *StatisticalDetector) GetStats() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	metricCount := len(d.metrics)
	readyCount := 0

	for _, stats := range d.metrics {
		if stats.Count >= int64(d.minSamples) {
			readyCount++
		}
	}

	return map[string]interface{}{
		"total_metrics":   metricCount,
		"ready_metrics":   readyCount,
		"min_samples":     d.minSamples,
		"z_threshold":     d.config.ZScoreThreshold,
		"iqr_multiplier":  d.config.IQRMultiplier,
	}
}

// StatisticalDetectorProcessor 统计异常检测处理器（包装器）
type StatisticalDetectorProcessor struct {
	name       string
	detector   *StatisticalDetector
}

// NewStatisticalDetectorProcessor 创建统计异常检测处理器
func NewStatisticalDetectorProcessor(config *StatisticalConfig) *StatisticalDetectorProcessor {
	return &StatisticalDetectorProcessor{
		name:     "statistical_detector",
		detector: NewStatisticalDetector(config),
	}
}

// Name 返回处理器名称
func (p *StatisticalDetectorProcessor) Name() string {
	return p.name
}

// Process 处理日志条目
func (p *StatisticalDetectorProcessor) Process(ctx context.Context, entry *LogEntry) (*LogEntry, error) {
	return p.detector.Process(ctx, entry)
}

// GetProcessorStats 获取处理器统计
func (p *StatisticalDetectorProcessor) GetProcessorStats() map[string]interface{} {
	return p.detector.GetStats()
}
