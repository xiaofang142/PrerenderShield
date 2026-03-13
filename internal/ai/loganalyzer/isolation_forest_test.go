package loganalyzer

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultIFConfig(t *testing.T) {
	config := DefaultIFConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 100, config.NTrees)
	assert.Equal(t, 256, config.SampleSize)
	assert.Equal(t, 8, config.MaxHeight)
	assert.Equal(t, 0.1, config.Contamination)
	assert.Equal(t, 10, config.NumFeatures)
}

func TestNewIsolationForest(t *testing.T) {
	featureNames := []string{"status", "bytes", "latency"}

	// 使用自定义配置
	config := &IFConfig{
		NTrees:        50,
		SampleSize:    128,
		MaxHeight:     6,
		Contamination: 0.05,
		NumFeatures:   3,
	}
	forest := NewIsolationForest(config, featureNames)
	assert.NotNil(t, forest)
	assert.Equal(t, config, forest.config)
	assert.Equal(t, featureNames, forest.featureNames)
	assert.False(t, forest.trained)

	// 使用 nil 配置
	forestNil := NewIsolationForest(nil, featureNames)
	assert.NotNil(t, forestNil)
	assert.Equal(t, 100, forestNil.config.NTrees)

	// 使用 nil MaxHeight
	configZero := &IFConfig{MaxHeight: 0, SampleSize: 256}
	forestZero := NewIsolationForest(configZero, featureNames)
	expectedHeight := int(math.Log2(256))
	assert.Equal(t, expectedHeight, forestZero.config.MaxHeight)
}

func TestIsolationForest_Fit_Empty(t *testing.T) {
	forest := NewIsolationForest(nil, []string{"a", "b"})
	forest.Fit([][]float64{})
	assert.False(t, forest.trained)
}

func TestIsolationForest_Fit(t *testing.T) {
	featureNames := []string{"status", "bytes", "latency", "is_bot", "threat_score", "is_anomaly"}
	forest := NewIsolationForest(&IFConfig{
		NTrees:        10,
		SampleSize:    50,
		MaxHeight:     4,
		Contamination: 0.1,
	}, featureNames)

	// 生成训练数据
	data := make([][]float64, 100)
	for i := 0; i < 100; i++ {
		data[i] = []float64{
			200,
			float64(1000 + i%100),
			float64(50 + i%50),
			0,
			float64(i % 10),
			0,
		}
	}

	forest.Fit(data)
	assert.True(t, forest.trained)
	assert.NotEmpty(t, forest.trees)
}

func TestIsolationForest_Predict(t *testing.T) {
	featureNames := []string{"status", "bytes", "latency"}
	forest := NewIsolationForest(&IFConfig{
		NTrees:        10,
		SampleSize:    50,
		MaxHeight:     4,
		Contamination: 0.1,
	}, featureNames)

	// 训练数据
	data := make([][]float64, 50)
	for i := 0; i < 50; i++ {
		data[i] = []float64{200, 1000, 100}
	}
	forest.Fit(data)

	// 预测正常样本
	normal := []float64{200, 1000, 100}
	result := forest.Predict(normal)
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, result.Score, 0.0)
	assert.LessOrEqual(t, result.Score, 1.0)

	// 预测异常样本
	anomaly := []float64{500, 50000, 5000}
	anomalyResult := forest.Predict(anomaly)
	assert.NotNil(t, anomalyResult)
}

func TestIsolationForest_Predict_Untrained(t *testing.T) {
	forest := NewIsolationForest(nil, []string{"a", "b"})
	result := forest.Predict([]float64{1, 2})
	assert.NotNil(t, result)
	assert.False(t, result.IsAnomaly)
	assert.Equal(t, 0.5, result.Score)
}

func TestAnomalyScore(t *testing.T) {
	forest := NewIsolationForest(&IFConfig{
		NTrees:        10,
		SampleSize:    50,
		MaxHeight:     4,
	}, []string{"a", "b"})

	// 未训练时
	score := forest.anomalyScore([]float64{1, 2})
	assert.Equal(t, 0.5, score)

	// 训练后
	data := make([][]float64, 50)
	for i := 0; i < 50; i++ {
		data[i] = []float64{100, 200}
	}
	forest.Fit(data)

	score = forest.anomalyScore([]float64{100, 200})
	assert.GreaterOrEqual(t, score, 0.0)
	assert.LessOrEqual(t, score, 1.0)
}

func TestPathLength(t *testing.T) {
	forest := NewIsolationForest(nil, []string{"a", "b"})

	// 空节点
	length := forest.pathLength(nil, nil, 0)
	assert.Equal(t, 0.0, length)

	// 叶节点
	leaf := &IFNode{size: 1, depth: 3}
	length = forest.pathLength([]float64{1, 2}, leaf, 3)
	assert.Equal(t, 3.0, length) // c(1) = 0
}

func TestC(t *testing.T) {
	forest := NewIsolationForest(nil, []string{})

	// c(1) = 0
	assert.Equal(t, 0.0, forest.c(1))

	// c(2) = 2*(ln(1)+0.577...) - 2*1/2 ≈ 0.154
	c2 := forest.c(2)
	assert.Greater(t, c2, 0.0)
	assert.Less(t, c2, 1.0)

	// c(n) for n > 2
	c10 := forest.c(10)
	assert.Greater(t, c10, 0.0)
}

func TestCalculateConfidence(t *testing.T) {
	forest := NewIsolationForest(&IFConfig{
		NTrees:        10,
		SampleSize:    50,
		Contamination: 0.1,
	}, []string{"a", "b"})

	// 训练以设置阈值
	data := make([][]float64, 50)
	for i := 0; i < 50; i++ {
		data[i] = []float64{100, 200}
	}
	forest.Fit(data)

	// 测试置信度计算
	confidence := forest.calculateConfidence(0.5)
	assert.GreaterOrEqual(t, confidence, 0.0)
	assert.LessOrEqual(t, confidence, 1.0)
}

func TestAnalyzeAnomaly(t *testing.T) {
	forest := NewIsolationForest(&IFConfig{
		NTrees:        10,
		SampleSize:    50,
	}, []string{"status", "bytes", "latency"})

	// 训练
	data := make([][]float64, 50)
	for i := 0; i < 50; i++ {
		data[i] = []float64{200, 1000, 100}
	}
	forest.Fit(data)

	sample := []float64{200, 50000, 5000}
	reason := forest.analyzeAnomaly(sample)
	assert.NotEmpty(t, reason)
}

func TestAnalyzeAnomaly_NoStats(t *testing.T) {
	forest := NewIsolationForest(nil, []string{"a", "b"})
	reason := forest.analyzeAnomaly([]float64{1, 2})
	// 当没有 stats 时，返回默认原因
	assert.Equal(t, "多特征组合异常", reason)
}

func TestUpdateStats(t *testing.T) {
	forest := NewIsolationForest(nil, []string{"a", "b"})

	features := []float64{100, 200}
	forest.UpdateStats(features)
	assert.NotEmpty(t, forest.normalStats)
	assert.Len(t, forest.normalStats, 2)
}

func TestGetStats(t *testing.T) {
	forest := NewIsolationForest(nil, []string{"a", "b"})

	stats := forest.GetStats()
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "trained")
	assert.Contains(t, stats, "num_trees")
	assert.Contains(t, stats, "threshold")
}

func TestLogEntryToFeatures(t *testing.T) {
	entry := &LogEntry{
		SourceType: "access",
		Fields: map[string]interface{}{
			"status_int":      200,
			"body_bytes_int":  1024,
			"request_time_ms": 150.5,
			"is_bot":          true,
			"threat_score":    25.0,
			"is_anomaly":      false,
		},
	}

	features := LogEntryToFeatures(entry)
	assert.NotEmpty(t, features)
	assert.GreaterOrEqual(t, len(features), 1)
	assert.Equal(t, float64(200), features[0])
}

func TestLogEntryToFeatures_MissingFields(t *testing.T) {
	entry := &LogEntry{
		SourceType: "access",
		Fields:     map[string]interface{}{},
	}

	features := LogEntryToFeatures(entry)
	assert.NotEmpty(t, features)
	// 所有字段都应该是默认值 0 或缺失
}

func TestNewAnomalyDetectorProcessor(t *testing.T) {
	features := []string{"status", "bytes", "latency", "is_bot", "threat", "anomaly"}
	processor := NewAnomalyDetectorProcessor(features)
	assert.NotNil(t, processor)
	assert.Equal(t, "anomaly_detector", processor.Name())
}

func TestAnomalyDetectorProcessor_Process(t *testing.T) {
	features := []string{"status", "bytes", "latency", "is_bot", "threat", "anomaly"}
	processor := NewAnomalyDetectorProcessor(features)

	// 发送一些数据进行训练
	for i := 0; i < 100; i++ {
		entry := &LogEntry{
			SourceType: "access",
			Fields: map[string]interface{}{
				"status_int":      200,
				"body_bytes_int":  1000 + i,
				"request_time_ms": 100.0,
				"is_bot":          false,
				"threat_score":    0.0,
			},
		}
		result, err := processor.Process(context.Background(), entry)
		assert.Nil(t, err)
		assert.NotNil(t, result)
	}

	// 检查模型是否已训练
	stats := processor.GetModelStats()
	assert.NotNil(t, stats)
}

func TestAnomalyDetectorProcessor_GetModelStats(t *testing.T) {
	features := []string{"a", "b"}
	processor := NewAnomalyDetectorProcessor(features)

	stats := processor.GetModelStats()
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "trained")
}

func TestAnomalyDetectorProcessor_Train(t *testing.T) {
	features := []string{"a", "b"}
	processor := NewAnomalyDetectorProcessor(features)

	data := make([][]float64, 50)
	for i := 0; i < 50; i++ {
		data[i] = []float64{float64(i), float64(i * 2)}
	}

	processor.Train(data)

	stats := processor.GetModelStats()
	assert.True(t, stats["trained"].(bool))
}

func TestRetrain(t *testing.T) {
	features := []string{"a", "b"}
	processor := NewAnomalyDetectorProcessor(features)

	// 添加数据到窗口
	for i := 0; i < 1000; i++ {
		entry := &LogEntry{
			SourceType: "access",
			Fields: map[string]interface{}{
				"status_int":      200,
				"body_bytes_int":  1000 + i,
				"request_time_ms": 100.0,
			},
		}
		processor.Process(context.Background(), entry)
	}

	// 检查是否已训练
	stats := processor.GetModelStats()
	assert.NotNil(t, stats)
}

func TestSplitData(t *testing.T) {
	forest := NewIsolationForest(nil, []string{"a", "b"})

	data := [][]float64{
		{1, 2},
		{3, 4},
		{5, 6},
		{7, 8},
	}

	left, right := forest.splitData(data, 0, 4)
	assert.NotEmpty(t, left)
	assert.NotEmpty(t, right)
}

func TestBuildTree(t *testing.T) {
	forest := NewIsolationForest(&IFConfig{
		NTrees:     10,
		SampleSize: 50,
		MaxHeight:  4,
	}, []string{"a", "b"})

	data := make([][]float64, 50)
	for i := 0; i < 50; i++ {
		data[i] = []float64{float64(i), float64(i * 2)}
	}

	tree := forest.buildTree(data, 0, 4)
	assert.NotNil(t, tree)
	assert.NotNil(t, tree.root)
}

func TestBuildTrees(t *testing.T) {
	forest := NewIsolationForest(&IFConfig{
		NTrees:     5,
		SampleSize: 50,
		MaxHeight:  4,
	}, []string{"a", "b"})

	data := make([][]float64, 100)
	for i := 0; i < 100; i++ {
		data[i] = []float64{float64(i), float64(i * 2)}
	}

	forest.buildTrees(data)
	assert.Len(t, forest.trees, 5)
}

func TestRandomSample(t *testing.T) {
	forest := NewIsolationForest(nil, []string{"a", "b"})

	data := make([][]float64, 100)
	for i := 0; i < 100; i++ {
		data[i] = []float64{float64(i), float64(i * 2)}
	}

	sample := forest.randomSample(data, 50)
	assert.Len(t, sample, 50)
}

func TestRandomSample_SmallData(t *testing.T) {
	forest := NewIsolationForest(nil, []string{"a", "b"})

	data := [][]float64{{1, 2}, {3, 4}}
	sample := forest.randomSample(data, 50)
	assert.Len(t, sample, 2) // 数据不足时返回全部
}

func TestCalculateThreshold(t *testing.T) {
	forest := NewIsolationForest(&IFConfig{
		NTrees:        10,
		SampleSize:    50,
		Contamination: 0.1,
	}, []string{"a", "b"})

	data := make([][]float64, 100)
	for i := 0; i < 100; i++ {
		data[i] = []float64{float64(i), float64(i * 2)}
	}

	forest.Fit(data)
	assert.Greater(t, forest.threshold, 0.0)
}

func TestIFNode(t *testing.T) {
	node := &IFNode{
		left:      nil,
		right:     nil,
		splitAttr: 0,
		splitVal:  5.0,
		size:      10,
		depth:     3,
	}

	assert.Nil(t, node.left)
	assert.Nil(t, node.right)
	assert.Equal(t, 0, node.splitAttr)
	assert.Equal(t, 5.0, node.splitVal)
	assert.Equal(t, 10, node.size)
	assert.Equal(t, 3, node.depth)
}

func TestIFTrees(t *testing.T) {
	tree := &IFTrees{
		root:    nil,
		height:  5,
		feature: 0,
	}

	assert.Equal(t, 5, tree.height)
	assert.Equal(t, 0, tree.feature)
}

func TestFeatureStats(t *testing.T) {
	stats := &FeatureStats{
		Mean:   50.0,
		StdDev: 10.0,
		Min:    20.0,
		Max:    80.0,
		Count:  100,
	}

	assert.Equal(t, 50.0, stats.Mean)
	assert.Equal(t, 10.0, stats.StdDev)
	assert.Equal(t, 20.0, stats.Min)
	assert.Equal(t, 80.0, stats.Max)
	assert.Equal(t, int64(100), stats.Count)
}

func TestIFResult(t *testing.T) {
	result := &IFResult{
		Score:      0.85,
		IsAnomaly:  true,
		Confidence: 0.9,
		Reason:     "High latency detected",
	}

	assert.Equal(t, 0.85, result.Score)
	assert.True(t, result.IsAnomaly)
	assert.Equal(t, 0.9, result.Confidence)
	assert.Equal(t, "High latency detected", result.Reason)
}
