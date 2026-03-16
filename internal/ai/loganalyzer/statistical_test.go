package loganalyzer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultStatisticalConfig(t *testing.T) {
	config := DefaultStatisticalConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 3.0, config.ZScoreThreshold)
	assert.Equal(t, 3.5, config.ModifiedZThreshold)
	assert.Equal(t, 1.5, config.IQRMultiplier)
	assert.True(t, config.EnableIQR)
	assert.True(t, config.EnableZScore)
}

func TestNewStatisticalDetector(t *testing.T) {
	config := &StatisticalConfig{
		ZScoreThreshold:    2.5,
		ModifiedZThreshold: 3.0,
		IQRMultiplier:      1.5,
		EnableIQR:          true,
		EnableZScore:       true,
	}
	detector := NewStatisticalDetector(config)
	assert.NotNil(t, detector)
	assert.Equal(t, config, detector.config)
	assert.NotNil(t, detector.metrics)
	assert.Equal(t, 30, detector.minSamples)

	detectorNil := NewStatisticalDetector(nil)
	assert.NotNil(t, detectorNil)
	assert.Equal(t, 3.0, detectorNil.config.ZScoreThreshold)
}

func TestStatisticalDetector_Process_NilEntry(t *testing.T) {
	detector := NewStatisticalDetector(nil)
	result, err := detector.Process(context.Background(), nil)
	assert.Nil(t, err)
	assert.Nil(t, result)
}

func TestStatisticalDetector_Process(t *testing.T) {
	detector := NewStatisticalDetector(&StatisticalConfig{
		ZScoreThreshold:    3.0,
		ModifiedZThreshold: 3.5,
		IQRMultiplier:      1.5,
		EnableIQR:          true,
		EnableZScore:       true,
	})

	for i := 0; i < 50; i++ {
		entry := &LogEntry{
			SourceType: "access",
			Fields: map[string]interface{}{
				"status_int":      200,
				"body_bytes_int":  1024,
				"request_time_ms": 100.0,
			},
		}
		result, err := detector.Process(context.Background(), entry)
		assert.Nil(t, err)
		assert.NotNil(t, result)
	}
}

func TestStatisticalDetector_extractMetrics(t *testing.T) {
	detector := NewStatisticalDetector(nil)

	entry := &LogEntry{
		Fields: map[string]interface{}{
			"request_time_ms": 150.0,
			"body_bytes_int":  1024,
			"status_int":      200,
			"render_time":     500.0,
			"threat_score":    25.0,
		},
	}

	metrics := detector.extractMetrics(entry)
	assert.NotNil(t, metrics)
	assert.Contains(t, metrics, "request_latency")
	assert.Contains(t, metrics, "response_size")
	assert.Contains(t, metrics, "status_code")
	assert.Contains(t, metrics, "render_time")
	assert.Contains(t, metrics, "threat_score")
}

func TestStatisticalDetector_extractMetrics_Empty(t *testing.T) {
	detector := NewStatisticalDetector(nil)

	entry := &LogEntry{
		Fields: map[string]interface{}{},
	}

	metrics := detector.extractMetrics(entry)
	assert.NotNil(t, metrics)
	assert.Empty(t, metrics)
}

func TestStatisticalDetector_updateStats(t *testing.T) {
	detector := NewStatisticalDetector(nil)

	stats := detector.updateStats("test_metric", 100.0)
	assert.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.Count)
	assert.Equal(t, 100.0, stats.Mean)
	assert.Equal(t, 100.0, stats.Min)
	assert.Equal(t, 100.0, stats.Max)

	stats = detector.updateStats("test_metric", 200.0)
	assert.Equal(t, int64(2), stats.Count)
	assert.Equal(t, 150.0, stats.Mean)
	assert.Equal(t, 100.0, stats.Min)
	assert.Equal(t, 200.0, stats.Max)
}

func TestStatisticalDetector_detectAnomaly_ZScore(t *testing.T) {
	detector := NewStatisticalDetector(&StatisticalConfig{
		ZScoreThreshold: 3.0,
		EnableIQR:       false,
		EnableZScore:    true,
	})

	stats := &MetricStats{
		Mean:   100.0,
		StdDev: 10.0,
	}

	// 正常值
	result := detector.detectAnomaly("test", 105.0, stats)
	assert.False(t, result.IsAnomaly)
	assert.True(t, result.IsZNormal)

	// 异常值 (Z-Score > 3 but < 4)
	result = detector.detectAnomaly("test", 135.0, stats)
	assert.True(t, result.IsAnomaly)
	assert.False(t, result.IsZNormal)
	assert.Equal(t, "medium", result.Severity)
}

func TestStatisticalDetector_detectAnomaly_IQR(t *testing.T) {
	detector := NewStatisticalDetector(&StatisticalConfig{
		IQRMultiplier: 1.5,
		EnableIQR:     true,
		EnableZScore:  false,
	})

	// 创建有足够数据的 stats
	stats := &MetricStats{
		Values: []float64{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
	}

	// 正常值
	result := detector.detectAnomaly("test", 15.0, stats)
	assert.False(t, result.IsAnomaly)

	// 异常值（远高于 Q3）
	result = detector.detectAnomaly("test", 100.0, stats)
	assert.True(t, result.IsAnomaly)
	assert.False(t, result.IsIQRNormal)
}

func TestStatisticalDetector_detectAnomaly_Severity(t *testing.T) {
	detector := NewStatisticalDetector(&StatisticalConfig{
		ZScoreThreshold: 3.0,
		EnableIQR:       false,
		EnableZScore:    true,
	})

	stats := &MetricStats{
		Mean:   100.0,
		StdDev: 10.0,
	}

	// Z-Score > 5, severity = critical
	result := detector.detectAnomaly("test", 200.0, stats)
	assert.Equal(t, "critical", result.Severity)
}

func TestStatisticalDetector_GetMetricStats(t *testing.T) {
	detector := NewStatisticalDetector(nil)

	detector.updateStats("test_metric", 100.0)

	stats, ok := detector.GetMetricStats("test_metric")
	assert.True(t, ok)
	assert.NotNil(t, stats)

	_, ok = detector.GetMetricStats("nonexistent")
	assert.False(t, ok)
}

func TestStatisticalDetector_GetAllMetrics(t *testing.T) {
	detector := NewStatisticalDetector(nil)

	detector.updateStats("metric1", 100.0)
	detector.updateStats("metric2", 200.0)

	allMetrics := detector.GetAllMetrics()
	assert.Len(t, allMetrics, 2)
	assert.Contains(t, allMetrics, "metric1")
	assert.Contains(t, allMetrics, "metric2")
}

func TestMetricStats(t *testing.T) {
	stats := &MetricStats{
		Count:      100,
		Sum:        1000.0,
		SumSq:      15000.0,
		Min:        5.0,
		Max:        25.0,
		Mean:       10.0,
		StdDev:     3.0,
		Q1:         7.5,
		Q3:         12.5,
		IQR:        5.0,
		Values:     []float64{1, 2, 3},
		lastUpdate: time.Now(),
	}

	assert.Equal(t, int64(100), stats.Count)
	assert.Equal(t, 1000.0, stats.Sum)
}

func TestStatisticalResult(t *testing.T) {
	result := &StatisticalResult{
		Method:      "z_score",
		ZScore:      2.5,
		ModifiedZ:   3.0,
		IsIQRNormal: true,
		IsZNormal:   true,
		IsAnomaly:   false,
		Severity:    "low",
	}

	assert.Equal(t, "z_score", result.Method)
	assert.Equal(t, 2.5, result.ZScore)
	assert.False(t, result.IsAnomaly)
}
