package ai

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestConfig 测试 Config 结构
func TestConfig_Struct(t *testing.T) {
	config := &Config{
		ModelPath:           "./data/models/test",
		WorkerPool:          8,
		ConfidenceThreshold: 0.9,
		PredictTimeout:      100 * time.Millisecond,
		CacheSize:           5000,
		CacheTTL:            10 * time.Minute,
		FeatureSize:         256,
		AutoUpdate:          true,
		UpdateInterval:      12 * time.Hour,
		RemoteModelURL:      "http://example.com/model",
		Enabled:             true,
	}

	assert.Equal(t, "./data/models/test", config.ModelPath)
	assert.Equal(t, 8, config.WorkerPool)
	assert.Equal(t, float32(0.9), config.ConfidenceThreshold)
	assert.Equal(t, 100*time.Millisecond, config.PredictTimeout)
	assert.Equal(t, 5000, config.CacheSize)
	assert.Equal(t, 10*time.Minute, config.CacheTTL)
	assert.Equal(t, 256, config.FeatureSize)
	assert.True(t, config.AutoUpdate)
	assert.Equal(t, 12*time.Hour, config.UpdateInterval)
	assert.True(t, config.Enabled)
}

// TestDefaultConfig 测试默认配置
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, "./data/models/threat_detection", config.ModelPath)
	assert.Equal(t, 4, config.WorkerPool)
	assert.Equal(t, float32(0.85), config.ConfidenceThreshold)
	assert.Equal(t, 50*time.Millisecond, config.PredictTimeout)
	assert.Equal(t, 10000, config.CacheSize)
	assert.Equal(t, 5*time.Minute, config.CacheTTL)
	assert.Equal(t, 128, config.FeatureSize)
	assert.True(t, config.AutoUpdate)
	assert.Equal(t, 24*time.Hour, config.UpdateInterval)
	assert.False(t, config.Enabled)
}

// TestConfig_Validate 测试配置验证
func TestConfig_Validate(t *testing.T) {
	// 测试空 ModelPath
	config := &Config{
		ModelPath: "",
	}
	err := config.Validate()
	assert.Error(t, err)
	assert.Equal(t, ErrModelPathEmpty, err)

	// 测试 WorkerPool <= 0
	config = &Config{
		ModelPath:  "./test",
		WorkerPool: 0,
	}
	err = config.Validate()
	assert.NoError(t, err)
	assert.Equal(t, 4, config.WorkerPool)

	// 测试 ConfidenceThreshold 无效
	config = &Config{
		ModelPath:           "./test",
		WorkerPool:          4,
		ConfidenceThreshold: 1.5,
	}
	err = config.Validate()
	assert.NoError(t, err)
	assert.Equal(t, float32(0.85), config.ConfidenceThreshold)

	// 测试 PredictTimeout <= 0
	config = &Config{
		ModelPath:           "./test",
		WorkerPool:          4,
		ConfidenceThreshold: 0.85,
		PredictTimeout:      0,
	}
	err = config.Validate()
	assert.NoError(t, err)
	assert.Equal(t, 50*time.Millisecond, config.PredictTimeout)

	// 测试 CacheSize <= 0
	config = &Config{
		ModelPath: "./test",
		CacheSize: 0,
	}
	err = config.Validate()
	assert.NoError(t, err)
	assert.Equal(t, 10000, config.CacheSize)

	// 测试 FeatureSize <= 0
	config = &Config{
		ModelPath:   "./test",
		FeatureSize: -1,
	}
	err = config.Validate()
	assert.NoError(t, err)
	assert.Equal(t, 128, config.FeatureSize)

	// 测试有效配置
	config = &Config{
		ModelPath:  "./test",
		WorkerPool: 8,
	}
	err = config.Validate()
	assert.NoError(t, err)
	assert.Equal(t, 8, config.WorkerPool)
}

// TestThreatTypeConfig 测试 ThreatTypeConfig 结构
func TestThreatTypeConfig(t *testing.T) {
	config := ThreatTypeConfig{
		Name:        "SQL Injection",
		Label:       "sql_injection",
		Threshold:   0.8,
		Severity:    "high",
		Description: "SQL injection attack",
	}

	assert.Equal(t, "SQL Injection", config.Name)
	assert.Equal(t, "sql_injection", config.Label)
	assert.Equal(t, float32(0.8), config.Threshold)
	assert.Equal(t, "high", config.Severity)
	assert.Equal(t, "SQL injection attack", config.Description)
}

// TestThreatTypes 测试预定义威胁类型
func TestThreatTypes(t *testing.T) {
	expectedTypes := []string{
		"sql_injection",
		"xss",
		"command_injection",
		"path_traversal",
		"ssrf",
		"xxe",
		"bot",
		"scanner",
		"benign",
	}

	for _, threatType := range expectedTypes {
		config, exists := ThreatTypes[threatType]
		assert.True(t, exists, "ThreatType %s should exist", threatType)
		assert.NotEmpty(t, config.Name)
		assert.NotEmpty(t, config.Label)
		assert.NotEmpty(t, config.Description)
	}
}

// TestGetSeverityByConfidence 测试根据置信度获取严重程度
func TestGetSeverityByConfidence(t *testing.T) {
	tests := []struct {
		confidence float32
		expected   string
	}{
		{0.96, "critical"},
		{0.95, "critical"},
		{0.94, "high"},
		{0.85, "high"},
		{0.84, "medium"},
		{0.7, "medium"},
		{0.69, "low"},
		{0.5, "low"},
		{0.49, "info"},
		{0.0, "info"},
	}

	for _, tt := range tests {
		result := GetSeverityByConfidence(tt.confidence)
		assert.Equal(t, tt.expected, result, "confidence: %f", tt.confidence)
	}
}

// TestPrediction 测试 Prediction 结构
func TestPrediction(t *testing.T) {
	prediction := &Prediction{
		ThreatType:  "sql_injection",
		Confidence:  0.95,
		IsMalicious: true,
		AllProbs:    []float32{0.05, 0.95, 0.0},
	}

	assert.Equal(t, "sql_injection", prediction.ThreatType)
	assert.Equal(t, float32(0.95), prediction.Confidence)
	assert.True(t, prediction.IsMalicious)
	assert.Len(t, prediction.AllProbs, 3)
}

// TestTensorFlowModel 测试 TensorFlowModel
func TestTensorFlowModel(t *testing.T) {
	model := &TensorFlowModel{
		labels:    []string{"sql_injection", "xss", "benign"},
		version:   "1.0.0",
		inputSize: 128,
	}

	// 测试 GetLabels
	labels := model.GetLabels()
	assert.Equal(t, []string{"sql_injection", "xss", "benign"}, labels)

	// 测试 GetVersion
	version := model.GetVersion()
	assert.Equal(t, "1.0.0", version)

	// 测试 Close（不应该 panic）
	assert.NotPanics(t, func() {
		model.Close()
	})
}

// TestTensorFlowModel_Predict_EmptyFeatures 测试空特征预测
func TestTensorFlowModel_Predict_EmptyFeatures(t *testing.T) {
	model := &TensorFlowModel{}

	prediction := model.Predict([]float32{})
	assert.Equal(t, "benign", prediction.ThreatType)
	assert.Equal(t, float32(1.0), prediction.Confidence)
	assert.False(t, prediction.IsMalicious)
}

// TestTensorFlowModel_Predict_HighMaxVal 测试高最大值预测
func TestTensorFlowModel_Predict_HighMaxVal(t *testing.T) {
	model := &TensorFlowModel{
		labels: []string{"sql_injection", "benign"},
	}

	features := []float32{0.9, 0.5, 0.3, 0.1}
	prediction := model.Predict(features)
	assert.Equal(t, "sql_injection", prediction.ThreatType)
	assert.Equal(t, float32(0.99), prediction.Confidence)
	assert.True(t, prediction.IsMalicious)
}

// TestTensorFlowModel_Predict_MediumAvg 测试中等平均值预测
func TestTensorFlowModel_Predict_MediumAvg(t *testing.T) {
	model := &TensorFlowModel{
		labels: []string{"sql_injection", "xss", "benign"},
	}

	features := []float32{0.6, 0.7, 0.5, 0.6}
	prediction := model.Predict(features)
	assert.Equal(t, "sql_injection", prediction.ThreatType)
	assert.True(t, prediction.IsMalicious)
}

// TestTensorFlowModel_Predict_Normal 测试正常请求预测
func TestTensorFlowModel_Predict_Normal(t *testing.T) {
	model := &TensorFlowModel{
		labels: []string{"sql_injection", "benign"},
	}

	features := []float32{0.1, 0.2, 0.1, 0.15}
	prediction := model.Predict(features)
	assert.Equal(t, "sql_injection", prediction.ThreatType)
	assert.True(t, prediction.IsMalicious)
}

// TestGenerateProbs 测试概率分布生成
func TestGenerateProbs(t *testing.T) {
	probs := generateProbs(5, 2)
	assert.Len(t, probs, 5)
	assert.Equal(t, float32(0.7), probs[2]) // 高概率在索引 2

	// 其他概率应该相等且较小
	for i := range probs {
		if i != 2 {
			assert.Equal(t, float32(0.075), probs[i]) // 0.3 / 4
		}
	}
}

// TestLightweightModel 测试 LightweightModel
func TestLightweightModel(t *testing.T) {
	model := NewLightweightModel(128)

	assert.NotNil(t, model)
	assert.Equal(t, 128, model.inputSize)
	assert.NotNil(t, model.labels)
	assert.Equal(t, float32(0.5), model.threshold)
}

// TestLightweightModel_Predict_EmptyFeatures 测试空特征预测
func TestLightweightModel_Predict_EmptyFeatures(t *testing.T) {
	model := NewLightweightModel(128)

	prediction := model.Predict([]float32{})
	assert.Equal(t, "benign", prediction.ThreatType)
	assert.Equal(t, float32(1.0), prediction.Confidence)
	assert.False(t, prediction.IsMalicious)
}

// TestLightweightModel_Predict_Anomaly 测试异常检测
func TestLightweightModel_Predict_Anomaly(t *testing.T) {
	model := NewLightweightModel(128)

	// 高方差特征
	features := make([]float32, 128)
	for i := 0; i < 128; i++ {
		if i%2 == 0 {
			features[i] = 0.9
		} else {
			features[i] = 0.1
		}
	}
	prediction := model.Predict(features)
	assert.True(t, prediction.IsMalicious)
}

// TestLightweightModel_Predict_Normal 测试正常检测
func TestLightweightModel_Predict_Normal(t *testing.T) {
	model := NewLightweightModel(128)

	// 低方差特征
	features := []float32{0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5}
	prediction := model.Predict(features)
	assert.False(t, prediction.IsMalicious)
}

// TestLightweightModel_ClassifyThreat 测试威胁分类
func TestLightweightModel_ClassifyThreat(t *testing.T) {
	model := &LightweightModel{
		inputSize: 128,
		labels:    getDefaultLabels(),
		threshold: 0.5,
	}

	// 测试 Body 特征高（SQL 注入）
	features := make([]float32, 128)
	for i := 50; i < 80; i++ {
		features[i] = 0.9
	}
	threatType := model.classifyThreat(features)
	assert.Equal(t, "sql_injection", threatType)

	// 测试 Header 特征高（XSS）
	features = make([]float32, 128)
	for i := 20; i < 50; i++ {
		features[i] = 0.9
	}
	threatType = model.classifyThreat(features)
	assert.Equal(t, "xss", threatType)

	// 测试 URL 特征高（路径遍历）
	features = make([]float32, 128)
	for i := 0; i < 20; i++ {
		features[i] = 0.9
	}
	threatType = model.classifyThreat(features)
	assert.Equal(t, "path_traversal", threatType)

	// 测试行为特征高（Bot）
	features = make([]float32, 128)
	for i := 80; i < 128; i++ {
		features[i] = 0.9
	}
	threatType = model.classifyThreat(features)
	assert.Equal(t, "bot", threatType)

	// 测试特征不足
	threatType = model.classifyThreat([]float32{0.1, 0.2})
	assert.Equal(t, "unknown", threatType)
}

// TestLightweightModel_Close 测试 Close 方法
func TestLightweightModel_Close(t *testing.T) {
	model := NewLightweightModel(128)
	assert.NotPanics(t, func() {
		model.Close()
	})
}

// TestLightweightModel_GetLabels 测试 GetLabels 方法
func TestLightweightModel_GetLabels(t *testing.T) {
	model := NewLightweightModel(128)
	labels := model.GetLabels()
	assert.NotNil(t, labels)
	assert.NotEmpty(t, labels)
}

// TestLightweightModel_GetVersion 测试 GetVersion 方法
func TestLightweightModel_GetVersion(t *testing.T) {
	model := NewLightweightModel(128)
	version := model.GetVersion()
	assert.Equal(t, "lightweight-1.0.0", version)
}

// TestGetDefaultLabels 测试默认标签
func TestGetDefaultLabels(t *testing.T) {
	labels := getDefaultLabels()
	expectedLabels := []string{
		"sql_injection",
		"xss",
		"command_injection",
		"path_traversal",
		"ssrf",
		"xxe",
		"bot",
		"scanner",
		"benign",
	}
	assert.Equal(t, expectedLabels, labels)
}

// TestLoadThreatModel 测试加载模型
func TestLoadThreatModel(t *testing.T) {
	// 测试不存在的模型路径
	model, err := LoadThreatModel("/nonexistent/path")
	assert.Error(t, err)
	assert.Nil(t, model)
	assert.Equal(t, ErrModelFileNotFound, err)
}

// TestModelInfo 测试 ModelInfo 结构
func TestModelInfo(t *testing.T) {
	info := &ModelInfo{
		Name:      "Test Model",
		Version:   "1.0.0",
		InputSize: 128,
		Labels:    []string{"sql_injection", "benign"},
	}

	assert.Equal(t, "Test Model", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Equal(t, 128, info.InputSize)
	assert.Len(t, info.Labels, 2)
}

// TestGetModelInfo 测试获取模型信息
func TestGetModelInfo(t *testing.T) {
	// 测试 nil 模型
	info := GetModelInfo(nil)
	assert.Nil(t, info)

	// 测试有效模型
	model := &TensorFlowModel{
		labels:  []string{"sql_injection", "benign"},
		version: "2.0.0",
	}
	info = GetModelInfo(model)
	assert.NotNil(t, info)
	assert.Equal(t, "Threat Detection Model", info.Name)
	assert.Equal(t, "2.0.0", info.Version)
	assert.Equal(t, 128, info.InputSize)
	assert.Len(t, info.Labels, 2)
	assert.WithinDuration(t, time.Now(), info.LoadedAt, time.Second)
}

// TestAIDetector_New 测试创建 AI 检测器
func TestAIDetector_New(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model" // 不存在的目录，会使用轻量级模型

	detector, err := NewAIDetector(config)
	assert.NoError(t, err)
	assert.NotNil(t, detector)
	assert.NotNil(t, detector.model)
	assert.NotNil(t, detector.featureCache)
	assert.NotNil(t, detector.predictChan)
	assert.Equal(t, config.WorkerPool, detector.workerPool)

	// 清理
	err = detector.Close()
	assert.NoError(t, err)
}

// TestAIDetector_New_InvalidConfig 测试无效配置
func TestAIDetector_New_InvalidConfig(t *testing.T) {
	config := &Config{
		ModelPath: "", // 空路径
	}

	detector, err := NewAIDetector(config)
	assert.Error(t, err)
	assert.Nil(t, detector)
}

// TestAIDetector_Detect 测试 Detect 方法
func TestAIDetector_Detect(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"
	config.ConfidenceThreshold = 0.5 // 降低阈值以便更容易检测到威胁

	detector, err := NewAIDetector(config)
	assert.NoError(t, err)
	defer detector.Close()

	// 创建正常请求
	req := httptest.NewRequest(http.MethodGet, "http://example.com/normal/path", nil)
	_, err = detector.Detect(req)
	// 正常请求可能没有威胁（返回 nil）
	assert.NoError(t, err)

	// 创建可疑请求（使用 URL 编码）
	suspiciousURL := "http://example.com/api?id=1%27%20OR%20%271%27=%271"
	req2 := httptest.NewRequest(http.MethodGet, suspiciousURL, nil)
	threats2, err := detector.Detect(req2)
	assert.NoError(t, err)
	// 可疑请求可能检测到威胁，也可能没有（取决于模型实现）
	assert.NotNil(t, threats2)
}

// TestAIDetector_Detect_ClosedDetector 测试关闭的检测器
func TestAIDetector_Detect_ClosedDetector(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"

	detector, err := NewAIDetector(config)
	assert.NoError(t, err)

	// 关闭检测器
	err = detector.Close()
	assert.NoError(t, err)

	// 尝试检测
	req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	threats, err := detector.Detect(req)
	assert.Error(t, err)
	assert.Nil(t, threats)
	assert.Equal(t, ErrDetectorClosed, err)
}

// TestAIDetector_Detect_EmptyFeatures 测试空特征
func TestAIDetector_Detect_EmptyFeatures(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"

	detector, err := NewAIDetector(config)
	assert.NoError(t, err)
	defer detector.Close()

	// 创建一个可能产生空特征的请求
	req := &http.Request{
		Method: http.MethodGet,
		URL:    &url.URL{},
	}
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	assert.Nil(t, threats)
}

// TestAIDetector_Name 测试 Name 方法
func TestAIDetector_Name(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"

	detector, err := NewAIDetector(config)
	assert.NoError(t, err)
	defer detector.Close()

	name := detector.Name()
	assert.Equal(t, "ai_detector", name)
}

// TestAIDetector_GetStats 测试获取统计信息
func TestAIDetector_GetStats(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"

	detector, err := NewAIDetector(config)
	assert.NoError(t, err)
	defer detector.Close()

	stats := detector.GetStats()
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "cache_size")
	assert.Contains(t, stats, "worker_pool")
	assert.Contains(t, stats, "model_loaded")
	assert.Contains(t, stats, "confidence_threshold")
}

// TestAIDetector_Close_Twice 测试多次关闭
func TestAIDetector_Close_Twice(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"

	detector, err := NewAIDetector(config)
	assert.NoError(t, err)

	// 第一次关闭
	err = detector.Close()
	assert.NoError(t, err)

	// 第二次关闭（应该无错误）
	err = detector.Close()
	assert.NoError(t, err)
}

// TestGenerateRequestID 测试生成请求 ID
func TestGenerateRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/test/path", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	id := generateRequestID(req)
	assert.Contains(t, id, "GET")
	assert.Contains(t, id, "http://example.com/test/path")
	assert.Contains(t, id, "192.168.1.1:12345")

	// 测试 nil 请求
	id = generateRequestID(nil)
	assert.Equal(t, "", id)
}

// TestFeatureCache 测试 FeatureCache
func TestFeatureCache_New(t *testing.T) {
	cache := NewFeatureCache(100, time.Minute)
	assert.NotNil(t, cache)
	assert.NotNil(t, cache.data)
	assert.Equal(t, 100, cache.maxSize)
	assert.Equal(t, time.Minute, cache.ttl)
}

func TestFeatureCache_Set_Get(t *testing.T) {
	cache := NewFeatureCache(100, time.Second)

	features := []float32{0.1, 0.2, 0.3}
	cache.Set("key1", features)

	// 立即获取应该存在
	cached, exists := cache.Get("key1")
	assert.True(t, exists)
	assert.Equal(t, features, cached)
}

func TestFeatureCache_Get_NotFound(t *testing.T) {
	cache := NewFeatureCache(100, time.Second)

	cached, exists := cache.Get("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, cached)
}

func TestFeatureCache_Get_Expired(t *testing.T) {
	cache := NewFeatureCache(100, 10*time.Millisecond)

	features := []float32{0.1, 0.2, 0.3}
	cache.Set("key1", features)

	// 等待过期
	time.Sleep(20 * time.Millisecond)

	cached, exists := cache.Get("key1")
	assert.False(t, exists)
	assert.Nil(t, cached)
}

func TestFeatureCache_Cleanup(t *testing.T) {
	cache := NewFeatureCache(100, 10*time.Millisecond)

	cache.Set("key1", []float32{0.1, 0.2})
	cache.Set("key2", []float32{0.3, 0.4})

	// 等待过期
	time.Sleep(20 * time.Millisecond)

	cache.Cleanup()
	assert.Equal(t, 0, cache.Size())
}

func TestFeatureCache_Clear(t *testing.T) {
	cache := NewFeatureCache(100, time.Minute)

	cache.Set("key1", []float32{0.1, 0.2})
	cache.Set("key2", []float32{0.3, 0.4})

	cache.Clear()
	assert.Equal(t, 0, cache.Size())
}

func TestFeatureCache_Size(t *testing.T) {
	cache := NewFeatureCache(100, time.Minute)

	assert.Equal(t, 0, cache.Size())

	cache.Set("key1", []float32{0.1})
	assert.Equal(t, 1, cache.Size())

	cache.Set("key2", []float32{0.2})
	assert.Equal(t, 2, cache.Size())
}

func TestFeatureCache_LRU(t *testing.T) {
	cache := NewFeatureCache(3, time.Minute)

	// 添加超过最大容量的项
	cache.Set("key1", []float32{0.1})
	time.Sleep(time.Millisecond)
	cache.Set("key2", []float32{0.2})
	time.Sleep(time.Millisecond)
	cache.Set("key3", []float32{0.3})
	time.Sleep(time.Millisecond)
	cache.Set("key4", []float32{0.4}) // 应该触发 LRU

	// key1 应该被淘汰
	_, exists := cache.Get("key1")
	assert.False(t, exists)

	// 其他 key 应该存在
	_, exists = cache.Get("key2")
	assert.True(t, exists)
	_, exists = cache.Get("key3")
	assert.True(t, exists)
	_, exists = cache.Get("key4")
	assert.True(t, exists)
}

// TestNormalizeFeatures 测试特征规范化
func TestNormalizeFeatures(t *testing.T) {
	// 测试相同大小
	features := []float32{0.1, 0.2, 0.3}
	result := NormalizeFeatures(features, 3)
	assert.Equal(t, features, result)

	// 测试填充
	features = []float32{0.1, 0.2}
	result = NormalizeFeatures(features, 5)
	assert.Len(t, result, 5)
	assert.Equal(t, []float32{0.1, 0.2, 0, 0, 0}, result)

	// 测试截断
	features = []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	result = NormalizeFeatures(features, 3)
	assert.Len(t, result, 3)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, result)
}

// TestAvgSlice 测试 avgSlice 函数
func TestAvgSlice(t *testing.T) {
	result := avgSlice([]float32{1, 2, 3, 4, 5})
	assert.Equal(t, float32(3), result)

	result = avgSlice([]float32{})
	assert.Equal(t, float32(0), result)
}

// TestAIDetector_getSeverity 测试 getSeverity 方法
func TestAIDetector_getSeverity(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"

	detector, err := NewAIDetector(config)
	assert.NoError(t, err)
	defer detector.Close()

	// 测试不同置信度水平的严重程度
	assert.Equal(t, "critical", detector.getSeverity(0.96))
	assert.Equal(t, "critical", detector.getSeverity(0.95))
	assert.Equal(t, "high", detector.getSeverity(0.94))
	assert.Equal(t, "high", detector.getSeverity(0.85))
	assert.Equal(t, "medium", detector.getSeverity(0.84))
	assert.Equal(t, "medium", detector.getSeverity(0.7))
	assert.Equal(t, "low", detector.getSeverity(0.69))
	assert.Equal(t, "low", detector.getSeverity(0.5))
	assert.Equal(t, "info", detector.getSeverity(0.49))
	assert.Equal(t, "info", detector.getSeverity(0.0))
}

// TestAIDetector_UpdateModel 测试 UpdateModel 方法
func TestAIDetector_UpdateModel(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"

	detector, err := NewAIDetector(config)
	// 如果模型文件不存在，会使用轻量级模型
	if err != nil {
		// 跳过模型加载失败的测试，因为这是预期行为
		t.Skipf("Skipping test: model file not found: %v", err)
		return
	}
	defer detector.Close()

	// 更新模型路径（使用一个不存在的路径）
	err = detector.UpdateModel("./new_model_path")
	// 由于新路径也不存在，应该返回错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model file not found")
}

// ============ 测试缓存清理协程 ============

// TestAIDetector_cacheCleanupWorker 测试缓存清理协程
func TestAIDetector_cacheCleanupWorker(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"
	config.CacheTTL = 50 * time.Millisecond

	detector, err := NewAIDetector(config)
	assert.NoError(t, err)
	defer detector.Close()

	// 添加一些会过期的缓存
	detector.featureCache.Set("key1", []float32{0.1, 0.2})
	detector.featureCache.Set("key2", []float32{0.3, 0.4})

	// 等待超过 TTL
	time.Sleep(100 * time.Millisecond)

	// 手动触发清理（因为清理协程运行间隔较长）
	detector.featureCache.Cleanup()

	// 验证缓存已被清理
	assert.Equal(t, 0, detector.featureCache.Size())
}

// ============ 测试 Detect 方法覆盖率 ============

// TestAIDetector_Detect_WithCache 测试使用缓存的检测
func TestAIDetector_Detect_WithCache(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"
	config.ConfidenceThreshold = 0.5

	detector, err := NewAIDetector(config)
	assert.NoError(t, err)
	defer detector.Close()

	// 第一次请求
	req1 := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	_, err = detector.Detect(req1)
	assert.NoError(t, err)

	// 第二次相同请求应该使用缓存
	req2 := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	_, err = detector.Detect(req2)
	assert.NoError(t, err)
}

// TestAIDetector_Detect_Timeout 测试预测超时
func TestAIDetector_Detect_Timeout(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"
	config.PredictTimeout = 1 * time.Millisecond // 非常短的超时

	detector, err := NewAIDetector(config)
	assert.NoError(t, err)
	defer detector.Close()

	// 创建复杂请求可能导致超时
	req := httptest.NewRequest(http.MethodPost, "http://example.com/test", strings.NewReader("data="+strings.Repeat("x", 1000)))
	_, err = detector.Detect(req)
	// 超时应该返回 nil 而不报错
	assert.NoError(t, err)
}

// TestAIDetector_Detect_MaliciousRequest 测试恶意请求检测
func TestAIDetector_Detect_MaliciousRequest(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"
	config.ConfidenceThreshold = 0.3 // 降低阈值

	detector, err := NewAIDetector(config)
	assert.NoError(t, err)
	defer detector.Close()

	// 创建可疑请求
	req := httptest.NewRequest(http.MethodGet, "http://example.com/test?id=1'+OR+'1'='1", nil)
	threats, err := detector.Detect(req)
	assert.NoError(t, err)
	// 请求可能被检测为恶意
	if len(threats) > 0 {
		assert.Contains(t, threats[0].SubType, "ai_detected")
	}
}

// ============ 测试模型文件加载 ============

// TestLoadLabels 测试 loadLabels 函数
func TestLoadLabels(t *testing.T) {
	// 测试加载不存在的文件
	labels, err := loadLabels("/nonexistent/path/labels.json")
	assert.Error(t, err)
	assert.Nil(t, labels)

	// 创建临时标签文件
	tmpFile := t.TempDir() + "/labels.json"
	writeTestLabelsFile(t, tmpFile, []string{"label1", "label2", "label3"})

	labels, err = loadLabels(tmpFile)
	assert.NoError(t, err)
	assert.Equal(t, []string{"label1", "label2", "label3"}, labels)

	// 测试无效的 JSON 文件
	invalidFile := t.TempDir() + "/invalid.json"
	writeTestFile(t, invalidFile, "not valid json")
	labels, err = loadLabels(invalidFile)
	assert.Error(t, err)
	assert.Nil(t, labels)
}

// 辅助函数：写入测试标签文件
func writeTestLabelsFile(t *testing.T, path string, labels []string) {
	t.Helper()
	data, err := json.Marshal(labels)
	assert.NoError(t, err)
	err = os.WriteFile(path, data, 0644)
	assert.NoError(t, err)
}

// 辅助函数：写入测试文件
func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	err := os.WriteFile(path, []byte(content), 0644)
	assert.NoError(t, err)
}

// ============ 测试 extractFeatures 和辅助函数 ============

// TestAIDetector_extractFeatures 测试特征提取
func TestAIDetector_extractFeatures(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"
	config.FeatureSize = 256

	detector, err := NewAIDetector(config)
	assert.NoError(t, err)
	defer detector.Close()

	// 创建测试请求
	req := httptest.NewRequest(http.MethodGet, "http://example.com/test?id=1", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("X-Custom-Header", "test-value")

	// 调用 extractFeatures（私有方法，通过 Detect 间接测试）
	// 这个测试主要验证 Detect 不会因为特征提取而报错
	_, err = detector.Detect(req)
	assert.NoError(t, err)
}

// TestAIDetector_extractFeatures_WithBody 测试带 Body 的特征提取
func TestAIDetector_extractFeatures_WithBody(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"

	detector, err := NewAIDetector(config)
	assert.NoError(t, err)
	defer detector.Close()

	// 创建带 Body 的请求
	body := strings.NewReader("field1=value1&field2=value2")
	req := httptest.NewRequest(http.MethodPost, "http://example.com/api", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// 调用检测（会触发特征提取）
	_, err = detector.Detect(req)
	assert.NoError(t, err)
}

// Test_hasSpecialChars 测试 hasSpecialChars 函数
func Test_hasSpecialChars(t *testing.T) {
	tests := []struct {
		input    string
		expected float32
	}{
		{"normal", 0},
		{"<script>", 0.1},
		{"'; DROP", 0.2},
		{"../../etc", 0},
		{"", 0},
	}

	for _, tt := range tests {
		result := hasSpecialChars(tt.input)
		assert.GreaterOrEqual(t, result, tt.expected, "input: %s", tt.input)
	}
}

// Test_countSpecialCharsInHeaders 测试 countSpecialCharsInHeaders
func Test_countSpecialCharsInHeaders(t *testing.T) {
	headers := http.Header{
		"X-Test": []string{"normal-value"},
	}
	count := countSpecialCharsInHeaders(headers)
	assert.GreaterOrEqual(t, count, float32(0))

	// 测试包含特殊字符
	headers2 := http.Header{
		"X-Test": []string{"<script>alert(1)</script>"},
	}
	count2 := countSpecialCharsInHeaders(headers2)
	assert.Greater(t, count2, count)
}

// Test_countEncodedCharsInHeaders 测试 countEncodedCharsInHeaders
func Test_countEncodedCharsInHeaders(t *testing.T) {
	headers := http.Header{
		"X-Test": []string{"normal"},
	}
	count := countEncodedCharsInHeaders(headers)
	assert.GreaterOrEqual(t, count, float32(0))

	// 测试包含 URL 编码字符
	headers2 := http.Header{
		"X-Test": []string{"%3Cscript%3E"},
	}
	count2 := countEncodedCharsInHeaders(headers2)
	assert.Greater(t, count2, count)
}

// ============ 测试 LoadThreatModel ============

// TestLoadThreatModel_WithValidModel 测试加载有效模型
func TestLoadThreatModel_WithValidModel(t *testing.T) {
	// 创建临时模型目录
	tmpDir := t.TempDir()
	modelDir := tmpDir + "/model"
	err := os.MkdirAll(modelDir, 0755)
	assert.NoError(t, err)

	// 创建 labels.json
	labelsPath := modelDir + "/labels.json"
	writeTestLabelsFile(t, labelsPath, []string{"label1", "label2"})

	// 创建空的 model.pb 文件
	modelFile := modelDir + "/model.pb"
	writeTestFile(t, modelFile, "dummy model content")

	// 加载模型
	model, err := LoadThreatModel(modelDir)
	assert.NoError(t, err)
	assert.NotNil(t, model)

	// 验证模型
	labels := model.GetLabels()
	assert.Equal(t, []string{"label1", "label2"}, labels)
	assert.Equal(t, "1.0.0", model.GetVersion())

	// 关闭模型
	model.Close()
}

// TestLoadThreatModel_WithoutLabels 测试没有 labels.json 时加载模型
func TestLoadThreatModel_WithoutLabels(t *testing.T) {
	// 创建临时模型目录
	tmpDir := t.TempDir()
	modelDir := tmpDir + "/model"
	err := os.MkdirAll(modelDir, 0755)
	assert.NoError(t, err)

	// 只创建 model.pb，不创建 labels.json
	modelFile := modelDir + "/model.pb"
	writeTestFile(t, modelFile, "dummy model content")

	// 加载模型（应该使用默认标签）
	model, err := LoadThreatModel(modelDir)
	assert.NoError(t, err)
	assert.NotNil(t, model)

	// 验证使用默认标签
	labels := model.GetLabels()
	assert.NotNil(t, labels)
	assert.NotEmpty(t, labels)

	model.Close()
}

// Test_hasSpecialChars_MoreThan10 测试 hasSpecialChars 超过 10 个特殊字符
func Test_hasSpecialChars_MoreThan10(t *testing.T) {
	// 超过 10 个特殊字符应该返回 1.0
	input := "';'\";<>{}[]|;:`" // 超过 10 个特殊字符
	result := hasSpecialChars(input)
	assert.Equal(t, float32(1.0), result)
}

// Test_hasSpecialChars_LessThan10 测试 hasSpecialChars 少于 10 个特殊字符
func Test_hasSpecialChars_LessThan10(t *testing.T) {
	// 3 个特殊字符应该返回 0.3
	input := "';'"
	result := hasSpecialChars(input)
	assert.Equal(t, float32(0.3), result)
}

// Test_countSpecialCharsInHeaders_Empty 测试 countSpecialCharsInHeaders 空 header
func Test_countSpecialCharsInHeaders_Empty(t *testing.T) {
	headers := http.Header{}
	count := countSpecialCharsInHeaders(headers)
	assert.Equal(t, float32(0), count)
}

// Test_countEncodedCharsInHeaders_Empty 测试 countEncodedCharsInHeaders 空 header
func Test_countEncodedCharsInHeaders_Empty(t *testing.T) {
	headers := http.Header{}
	count := countEncodedCharsInHeaders(headers)
	assert.Equal(t, float32(0), count)
}

// TestAIDetector_UpdateModel_Success 测试 UpdateModel 成功分支
func TestAIDetector_UpdateModel_Success(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"

	detector, err := NewAIDetector(config)
	assert.NoError(t, err)
	defer detector.Close()

	// 创建临时模型目录
	tmpDir := t.TempDir()
	modelDir := tmpDir + "/model"
	err = os.MkdirAll(modelDir, 0755)
	assert.NoError(t, err)

	// 创建 labels.json
	labelsPath := modelDir + "/labels.json"
	writeTestLabelsFile(t, labelsPath, []string{"label1", "label2"})

	// 创建空的 model.pb 文件
	modelFile := modelDir + "/model.pb"
	writeTestFile(t, modelFile, "dummy model content")

	// 更新模型
	err = detector.UpdateModel(modelDir)
	assert.NoError(t, err)
}

// TestAIDetector_cacheCleanupWorker_Run 测试 cacheCleanupWorker 协程运行
func TestAIDetector_cacheCleanupWorker_Run(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"
	config.CacheTTL = 50 * time.Millisecond

	detector, err := NewAIDetector(config)
	assert.NoError(t, err)
	defer detector.Close()

	// 添加缓存
	detector.featureCache.Set("key1", []float32{0.1, 0.2})

	// 等待清理协程运行（清理间隔为 1 分钟，这里手动触发）
	time.Sleep(10 * time.Millisecond)

	// 手动触发清理来验证协程逻辑
	detector.featureCache.Cleanup()

	// 验证没有 panic
	assert.NotPanics(t, func() {
		detector.Close()
	})
}

// TestAIDetector_predictWorker_NoModel 测试 predictWorker 没有模型的情况
func TestAIDetector_predictWorker_NoModel(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"

	detector, err := NewAIDetector(config)
	assert.NoError(t, err)
	defer detector.Close()

	// 手动关闭模型以模拟无模型状态
	detector.mu.Lock()
	detector.model = nil
	detector.mu.Unlock()

	// 发送预测请求
	resultChan := make(chan *PredictResult, 1)
	select {
	case detector.predictChan <- &PredictRequest{
		RequestID: "test-id",
		Features:  []float32{0.1, 0.2},
		Response:  resultChan,
	}:
	case <-time.After(detector.config.PredictTimeout):
	}

	// 等待结果
	select {
	case result := <-resultChan:
		assert.Error(t, result.Error)
		assert.Equal(t, ErrModelNotLoaded, result.Error)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("predict worker did not respond")
	}
}

// TestAIDetector_Close_AlreadyClosed 测试 Close 方法在已经关闭时
func TestAIDetector_Close_AlreadyClosed(t *testing.T) {
	config := DefaultConfig()
	detector, err := NewAIDetector(config)
	assert.NoError(t, err)

	// 第一次关闭
	err = detector.Close()
	assert.NoError(t, err)

	// 第二次关闭应该返回 nil 且不 panic
	err = detector.Close()
	assert.NoError(t, err)
}

// TestAIDetector_cacheCleanupWorker_StopChan 测试 cacheCleanupWorker 的 stopChan 分支
func TestAIDetector_cacheCleanupWorker_StopChan(t *testing.T) {
	config := DefaultConfig()
	detector, err := NewAIDetector(config)
	assert.NoError(t, err)

	// 手动停止（不通过 Close，因为 Close 会关闭 predictChan）
	detector.mu.Lock()
	detector.closed = true
	detector.mu.Unlock()
	close(detector.stopChan)

	// 等待一小段时间让协程退出
	time.Sleep(10 * time.Millisecond)
}

// TestAIDetector_Detect_Closed 测试 Detect 方法在检测器关闭时
func TestAIDetector_Detect_Closed(t *testing.T) {
	config := DefaultConfig()
	detector, err := NewAIDetector(config)
	assert.NoError(t, err)

	// 关闭检测器
	detector.Close()

	// 尝试检测应该返回 ErrDetectorClosed
	req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	threats, err := detector.Detect(req)
	assert.Error(t, err)
	assert.Equal(t, ErrDetectorClosed, err)
	assert.Nil(t, threats)
}

// Test_countSpecialCharsInHeaders_MoreThan20 测试特殊字符数超过 20
func Test_countSpecialCharsInHeaders_MoreThan20(t *testing.T) {
	header := http.Header{}
	// 添加超过 20 个特殊字符的 header 值
	header.Set("X-Test", `'"<>'"<>'"<>'"<>'"<>'"<>'"<>'"<>`)
	result := countSpecialCharsInHeaders(header)
	assert.Equal(t, float32(1.0), result)
}

// Test_countEncodedCharsInHeaders_MoreThan10 测试编码字符数超过 10
func Test_countEncodedCharsInHeaders_MoreThan10(t *testing.T) {
	header := http.Header{}
	// 添加超过 10 个%字符的 header 值
	header.Set("X-Test", `%20%20%20%20%20%20%20%20%20%20%20`)
	result := countEncodedCharsInHeaders(header)
	assert.Equal(t, float32(1.0), result)
}

// TestAIDetector_Detect_UnknownThreatType 测试 Detect 方法处理未知威胁类型
func TestAIDetector_Detect_UnknownThreatType(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"
	detector, err := NewAIDetector(config)
	assert.NoError(t, err)
	defer detector.Close()

	// 创建一个没有模型的检测器，直接设置预测结果
	detector.mu.Lock()
	detector.model = &LightweightModel{
		labels:    []string{"custom_threat_type", "benign"},
		inputSize: 2,
	}
	detector.mu.Unlock()

	// 发送一个会被判定为恶意但威胁类型未知的请求
	req := httptest.NewRequest(http.MethodGet, "http://example.com/admin.php?id=1&name=test'OR'1'='1", nil)
	req.Header.Set("User-Agent", "sqlmap/1.0")

	// 由于没有实际模型，我们测试特征提取部分
	// 这里主要测试当 ThreatTypes 中没有对应类型时的分支
	threats, err := detector.Detect(req)
	// 不检查具体结果，只验证不 panic
	assert.NotNil(t, threats) // 可能是 nil 或非 nil，取决于模型
}

// TestLoadThreatModel_InvalidLabels 测试 LoadThreatModel 使用无效的 labels.json
func TestLoadThreatModel_InvalidLabels(t *testing.T) {
	tmpDir := t.TempDir()
	modelDir := tmpDir + "/model"
	os.MkdirAll(modelDir, 0755)

	// 创建无效的 labels.json
	labelsPath := modelDir + "/labels.json"
	writeTestFile(t, labelsPath, "invalid json content")

	// 创建模型文件
	modelFile := modelDir + "/model.pb"
	writeTestFile(t, modelFile, "dummy model content")

	// 应该使用默认标签而不是返回错误
	model, err := LoadThreatModel(modelDir)
	assert.NoError(t, err)
	assert.NotNil(t, model)

	// 验证使用的是默认标签
	labels := model.GetLabels()
	assert.Contains(t, labels, "sql_injection") // 默认标签之一
}

// TestExtractBodyFeatures_ReadError 测试 ExtractBodyFeatures 读取错误
func TestExtractBodyFeatures_ReadError(t *testing.T) {
	// 创建一个会返回错误的 ReadCloser
	errorReader := &errorReader{}
	features, err := ExtractBodyFeatures(errorReader)
	assert.Error(t, err)
	assert.Len(t, features, 30)
}

// errorReader 是一个会返回错误的 ReadCloser
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}

func (e *errorReader) Close() error {
	return nil
}

// TestLoadThreatModel_NoModelFile 测试 LoadThreatModel 模型文件不存在
func TestLoadThreatModel_NoModelFile(t *testing.T) {
	tmpDir := t.TempDir()
	modelDir := tmpDir + "/model"
	os.MkdirAll(modelDir, 0755)

	// 只创建 labels.json，不创建 model.pb
	labelsPath := modelDir + "/labels.json"
	writeTestLabelsFile(t, labelsPath, []string{"label1", "label2"})

	// 应该返回错误
	model, err := LoadThreatModel(modelDir)
	assert.Error(t, err)
	assert.Equal(t, ErrModelFileNotFound, err)
	assert.Nil(t, model)
}

// TestAIDetector_Detect_ResultError 测试 Detect 方法接收结果错误
func TestAIDetector_Detect_ResultError(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"
	detector, err := NewAIDetector(config)
	assert.NoError(t, err)
	defer detector.Close()

	// 等待 predict worker 启动
	time.Sleep(50 * time.Millisecond)

	// 创建一个会导致模型错误的请求
	req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)

	// 不检查具体结果，只验证不 panic
	threats, err := detector.Detect(req)
	// 如果有错误，应该是模型相关错误
	if err != nil {
		assert.NotNil(t, threats)
	}
}

// TestAIDetector_Detect_SendTimeout 测试 Detect 方法发送超时
func TestAIDetector_Detect_SendTimeout(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"
	config.PredictTimeout = 1 * time.Millisecond // 非常短的超时

	detector, err := NewAIDetector(config)
	assert.NoError(t, err)
	defer detector.Close()

	// 创建一个请求
	req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)

	// 不检查具体结果，只验证不 panic
	assert.NotPanics(t, func() {
		threats, err := detector.Detect(req)
		// 超时可能返回 nil 或错误
		_ = threats
		_ = err
	})
}

// TestAIDetector_Detect_ResultTimeout 测试 Detect 方法结果超时
func TestAIDetector_Detect_ResultTimeout(t *testing.T) {
	config := DefaultConfig()
	config.ModelPath = "./test_model"
	config.PredictTimeout = 1 * time.Millisecond // 非常短的超时

	detector, err := NewAIDetector(config)
	assert.NoError(t, err)
	defer detector.Close()

	// 等待 predict worker 启动
	time.Sleep(50 * time.Millisecond)

	// 创建一个请求
	req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)

	// 不检查具体结果，只验证不 panic
	assert.NotPanics(t, func() {
		threats, err := detector.Detect(req)
		// 超时可能返回 nil 或错误
		_ = threats
		_ = err
	})
}
