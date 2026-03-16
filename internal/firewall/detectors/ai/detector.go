package ai

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"prerender-shield/internal/firewall/types"
)

// 定义错误
var (
	ErrModelNotLoaded    = errors.New("model not loaded")
	ErrModelPathEmpty    = errors.New("model path is empty")
	ErrPredictionTimeout = errors.New("prediction timeout")
	ErrInvalidFeatures   = errors.New("invalid features")
	ErrDetectorClosed    = errors.New("detector is closed")
)

// AIDetector AI威胁检测器
type AIDetector struct {
	model        ThreatModel
	featureCache *FeatureCache
	config       *Config
	predictChan  chan *PredictRequest
	workerPool   int
	mu           sync.RWMutex
	closed       bool
	stopChan     chan struct{}
}

// PredictRequest 预测请求
type PredictRequest struct {
	RequestID string
	Features  []float32
	Response  chan *PredictResult
}

// PredictResult 预测结果
type PredictResult struct {
	RequestID   string
	ThreatType  string
	Confidence  float32
	IsMalicious bool
	Error       error
}

// NewAIDetector 创建AI检测器
func NewAIDetector(config *Config) (*AIDetector, error) {
	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// 加载预训练模型
	model, err := LoadThreatModel(config.ModelPath)
	if err != nil {
		// 如果模型加载失败，创建一个轻量级模型用于演示
		model = NewLightweightModel(config.FeatureSize)
	}

	detector := &AIDetector{
		model:        model,
		featureCache: NewFeatureCache(config.CacheSize, config.CacheTTL),
		config:       config,
		predictChan:  make(chan *PredictRequest, 1000),
		workerPool:   config.WorkerPool,
		stopChan:     make(chan struct{}),
	}

	// 启动预测工作池
	for i := 0; i < config.WorkerPool; i++ {
		go detector.predictWorker(i)
	}

	// 启动缓存清理协程
	go detector.cacheCleanupWorker()

	return detector, nil
}

// Detect 实现OWASPDetector接口
func (d *AIDetector) Detect(req *http.Request) ([]types.Threat, error) {
	d.mu.RLock()
	if d.closed {
		d.mu.RUnlock()
		return nil, ErrDetectorClosed
	}
	d.mu.RUnlock()

	// 提取特征
	features, err := d.extractFeatures(req)
	if err != nil {
		return nil, err
	}

	// 检查特征是否有效
	if len(features) == 0 {
		return nil, nil
	}

	// 发送预测请求
	resultChan := make(chan *PredictResult, 1)
	select {
	case d.predictChan <- &PredictRequest{
		RequestID: generateRequestID(req),
		Features:  features,
		Response:  resultChan,
	}:
	case <-time.After(d.config.PredictTimeout):
		// 发送请求超时，返回空
		return nil, nil
	}

	// 等待结果（带超时）
	select {
	case result := <-resultChan:
		if result.Error != nil {
			return nil, result.Error
		}
		if result.IsMalicious && result.Confidence > d.config.ConfidenceThreshold {
			threatType, ok := ThreatTypes[result.ThreatType]
			if !ok {
				threatType = ThreatTypeConfig{
					Name:        result.ThreatType,
					Label:       result.ThreatType,
					Severity:    GetSeverityByConfidence(result.Confidence),
					Description: "AI detected potential threat",
				}
			}

			return []types.Threat{{
				Type:     threatType.Label,
				Severity: threatType.Severity,
				Message:  threatType.Description,
				SubType:  "ai_detected",
				Details: map[string]interface{}{
					"confidence": result.Confidence,
					"source":     "ai_detector",
				},
			}}, nil
		}
	case <-time.After(d.config.PredictTimeout):
		// 超时则返回空，继续其他检测
		return nil, nil
	}

	return nil, nil
}

// predictWorker 预测工作协程
func (d *AIDetector) predictWorker(workerID int) {
	for {
		select {
		case req := <-d.predictChan:
			if req == nil {
				continue
			}

			if d.model == nil {
				req.Response <- &PredictResult{
					RequestID: req.RequestID,
					Error:     ErrModelNotLoaded,
				}
				continue
			}

			// 模型推理
			prediction := d.model.Predict(req.Features)

			req.Response <- &PredictResult{
				RequestID:   req.RequestID,
				ThreatType:  prediction.ThreatType,
				Confidence:  prediction.Confidence,
				IsMalicious: prediction.IsMalicious,
			}

		case <-d.stopChan:
			return
		}
	}
}

// extractFeatures 特征提取
func (d *AIDetector) extractFeatures(req *http.Request) ([]float32, error) {
	// 检查缓存
	cacheKey := generateRequestID(req)
	if cached, ok := d.featureCache.Get(cacheKey); ok {
		return cached, nil
	}

	features := make([]float32, 0, d.config.FeatureSize)

	// URL特征
	urlFeatures := ExtractURLFeatures(req.URL)
	features = append(features, urlFeatures...)

	// Header特征
	headerFeatures := ExtractHeaderFeatures(req.Header)
	features = append(features, headerFeatures...)

	// Body特征（如果有）
	if req.Body != nil {
		bodyFeatures, err := ExtractBodyFeatures(req.Body)
		if err == nil {
			features = append(features, bodyFeatures...)
		}
	}

	// 行为特征
	behaviorFeatures := ExtractBehaviorFeatures(req)
	features = append(features, behaviorFeatures...)

	// 规范化特征向量大小
	features = NormalizeFeatures(features, d.config.FeatureSize)

	// 缓存特征
	d.featureCache.Set(cacheKey, features)

	return features, nil
}

// getSeverity 根据置信度返回严重程度
func (d *AIDetector) getSeverity(confidence float32) string {
	return GetSeverityByConfidence(confidence)
}

// Name 实现接口
func (d *AIDetector) Name() string {
	return "ai_detector"
}

// Close 关闭检测器
func (d *AIDetector) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return nil
	}

	d.closed = true
	close(d.stopChan)
	close(d.predictChan)

	if d.model != nil {
		d.model.Close()
	}

	return nil
}

// cacheCleanupWorker 缓存清理协程
func (d *AIDetector) cacheCleanupWorker() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.featureCache.Cleanup()
		case <-d.stopChan:
			return
		}
	}
}

// UpdateModel 更新模型
func (d *AIDetector) UpdateModel(modelPath string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 加载新模型
	newModel, err := LoadThreatModel(modelPath)
	if err != nil {
		return err
	}

	// 关闭旧模型
	if d.model != nil {
		d.model.Close()
	}

	d.model = newModel
	d.featureCache.Clear()

	return nil
}

// GetStats 获取统计信息
func (d *AIDetector) GetStats() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return map[string]interface{}{
		"cache_size":           d.featureCache.Size(),
		"worker_pool":          d.workerPool,
		"model_loaded":         d.model != nil,
		"confidence_threshold": d.config.ConfidenceThreshold,
	}
}

// generateRequestID 生成请求ID
func generateRequestID(req *http.Request) string {
	if req == nil {
		return ""
	}
	return req.Method + "|" + req.URL.String() + "|" + req.RemoteAddr
}
