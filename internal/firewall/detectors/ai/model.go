package ai

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// 定义模型相关错误
var (
	ErrModelFileNotFound  = errors.New("model file not found")
	ErrInvalidModelFile   = errors.New("invalid model file")
	ErrLabelsFileNotFound = errors.New("labels file not found")
)

// ThreatModel 威胁检测模型接口
type ThreatModel interface {
	Predict(features []float32) *Prediction
	Close()
	GetLabels() []string
	GetVersion() string
}

// Prediction 预测结果
type Prediction struct {
	ThreatType  string
	Confidence  float32
	IsMalicious bool
	AllProbs    []float32 // 所有类别的概率
}

// TensorFlowModel TensorFlow模型实现
type TensorFlowModel struct {
	session   interface{} // *tf.Session
	graph     interface{} // *tf.Graph
	labels    []string
	mu        sync.RWMutex
	version   string
	inputSize int
}

// LoadThreatModel 加载模型
func LoadThreatModel(modelPath string) (ThreatModel, error) {
	// 检查模型目录是否存在
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return nil, ErrModelFileNotFound
	}

	// 加载标签
	labelsPath := filepath.Join(modelPath, "labels.json")
	labels, err := loadLabels(labelsPath)
	if err != nil {
		// 使用默认标签
		labels = getDefaultLabels()
	}

	// 检查模型文件
	modelFile := filepath.Join(modelPath, "model.pb")
	if _, err := os.Stat(modelFile); os.IsNotExist(err) {
		return nil, ErrModelFileNotFound
	}

	// 在实际生产环境中，这里会加载TensorFlow模型
	// 由于TensorFlow Go绑定较复杂，这里提供一个占位实现
	// 实际部署时需要安装 tensorflow/tensorflow/go
	//
	// ⚠️ EXPERIMENTAL: 本检测器为实验性功能——模型加载为占位实现，
	// Predict() 始终回退到 ruleBasedPredict。生产代码路径未启用（AIConfig 无赋值点），
	// 请勿在对外文档中宣称此能力，启用前必须完成真实模型加载实现。

	return &TensorFlowModel{
		labels:    labels,
		version:   "1.0.0",
		inputSize: 128,
	}, nil
}

// Predict 执行预测
func (m *TensorFlowModel) Predict(features []float32) *Prediction {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 在实际生产环境中，这里会执行TensorFlow推理
	// 这里提供一个基于规则的轻量级预测作为后备方案
	return m.ruleBasedPredict(features)
}

// ruleBasedPredict 基于加权特征规则的预测（后备方案）
// 使用多维特征分析，结合特征分布、方差和特定维度权重进行分类
func (m *TensorFlowModel) ruleBasedPredict(features []float32) *Prediction {
	if len(features) == 0 {
		return &Prediction{
			ThreatType:  "benign",
			Confidence:  1.0,
			IsMalicious: false,
		}
	}

	// 计算特征向量的多维统计信息
	var sum, maxVal, minVal, variance float32
	maxVal = features[0]
	minVal = features[0]

	for _, f := range features {
		sum += f
		if f > maxVal {
			maxVal = f
		}
		if f < minVal {
			minVal = f
		}
	}

	avg := sum / float32(len(features))

	// 计算方差（衡量特征波动程度）
	for _, f := range features {
		diff := f - avg
		variance += diff * diff
	}
	variance /= float32(len(features))

	// 特征维度权重分析
	// 假设特征向量按维度编码：
	// [0-15]: SQL 注入特征 (特殊字符频率、关键字匹配等)
	// [16-31]: XSS 特征 (脚本标签、事件处理器等)
	// [32-47]: 路径遍历特征 (../, %2e 等)
	// [48-63]: 命令注入特征 (;|&, $() 等)
	// [64-127]: 其他异常特征

	sqlScore := float32(0)
	xssScore := float32(0)
	pathScore := float32(0)
	cmdScore := float32(0)
	otherScore := float32(0)

	for i, f := range features {
		switch {
		case i < 16:
			sqlScore += f
		case i < 32:
			xssScore += f
		case i < 48:
			pathScore += f
		case i < 64:
			cmdScore += f
		default:
			otherScore += f
		}
	}

	// 归一化各维度得分
	featureSize := float32(16)
	if len(features) < 64 {
		featureSize = float32(len(features)) / 5
		if featureSize < 1 {
			featureSize = 1
		}
	}
	sqlScore /= featureSize
	xssScore /= featureSize
	pathScore /= featureSize
	cmdScore /= featureSize

	// 找出得分最高的威胁类型
	scores := map[string]float32{
		"sql_injection":     sqlScore,
		"xss":               xssScore,
		"path_traversal":    pathScore,
		"command_injection": cmdScore,
	}

	maxScore := float32(0)
	maxThreat := "benign"
	for threat, score := range scores {
		if score > maxScore {
			maxScore = score
			maxThreat = threat
		}
	}

	// 综合判断：结合最高维度得分、整体最大值和方差
	combinedScore := maxScore*0.5 + maxVal*0.3 + variance*0.2

	// 如果综合得分超过阈值，判定为恶意
	if combinedScore > 0.6 {
		return &Prediction{
			ThreatType:  maxThreat,
			Confidence:  minFloat32(combinedScore, 0.99),
			IsMalicious: true,
			AllProbs:    generateProbs(len(m.labels), getLabelIndex(m.labels, maxThreat)),
		}
	}

	// 中等风险：某些维度有异常但不强烈
	if maxScore > 0.4 || avg > 0.3 {
		return &Prediction{
			ThreatType:  maxThreat,
			Confidence:  maxScore * 0.7,
			IsMalicious: maxScore > 0.5,
			AllProbs:    generateProbs(len(m.labels), getLabelIndex(m.labels, maxThreat)),
		}
	}

	// 正常请求
	return &Prediction{
		ThreatType:  "benign",
		Confidence:  1.0 - avg,
		IsMalicious: false,
		AllProbs:    generateProbs(len(m.labels), getLabelIndex(m.labels, "benign")),
	}
}

// getLabelIndex 获取标签索引
func getLabelIndex(labels []string, threatType string) int {
	for i, label := range labels {
		if label == threatType {
			return i
		}
	}
	return len(labels) - 1
}

// minFloat32 返回两个 float32 中的较小值
func minFloat32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// generateProbs 生成概率分布
func generateProbs(numClasses, highIdx int) []float32 {
	probs := make([]float32, numClasses)
	for i := range probs {
		if i == highIdx {
			probs[i] = 0.7
		} else {
			probs[i] = 0.3 / float32(numClasses-1)
		}
	}
	return probs
}

// Close 关闭模型
func (m *TensorFlowModel) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	// 在实际环境中关闭TensorFlow会话
}

// GetLabels 获取标签
func (m *TensorFlowModel) GetLabels() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.labels
}

// GetVersion 获取版本
func (m *TensorFlowModel) GetVersion() string {
	return m.version
}

// LightweightModel 轻量级模型（用于演示或无TensorFlow环境）
type LightweightModel struct {
	inputSize int
	labels    []string
	threshold float32
}

// NewLightweightModel 创建轻量级模型
func NewLightweightModel(inputSize int) *LightweightModel {
	return &LightweightModel{
		inputSize: inputSize,
		labels:    getDefaultLabels(),
		threshold: 0.5,
	}
}

// Predict 执行预测
func (m *LightweightModel) Predict(features []float32) *Prediction {
	if len(features) == 0 {
		return &Prediction{
			ThreatType:  "benign",
			Confidence:  1.0,
			IsMalicious: false,
		}
	}

	// 计算异常分数
	score := m.calculateAnomalyScore(features)

	if score > m.threshold {
		// 根据特征模式判断威胁类型
		threatType := m.classifyThreat(features)
		return &Prediction{
			ThreatType:  threatType,
			Confidence:  score,
			IsMalicious: true,
		}
	}

	return &Prediction{
		ThreatType:  "benign",
		Confidence:  1.0 - score,
		IsMalicious: false,
	}
}

// calculateAnomalyScore 计算异常分数
func (m *LightweightModel) calculateAnomalyScore(features []float32) float32 {
	var sum, sqSum float32
	for _, f := range features {
		sum += f
		sqSum += f * f
	}

	n := float32(len(features))
	mean := sum / n
	variance := sqSum/n - mean*mean

	// 使用方差作为异常分数的一部分
	// 高方差通常意味着更异常的行为
	score := variance * 10 // 放大系数

	if score > 1.0 {
		score = 1.0
	}

	return score
}

// classifyThreat 分类威胁类型
func (m *LightweightModel) classifyThreat(features []float32) string {
	if len(features) < 10 {
		return "unknown"
	}

	// 基于特征向量的一些关键维度进行分类
	// 这是简化的演示实现

	// 特征索引映射（与features.go中的提取顺序相关）
	// 0-19: URL特征
	// 20-49: Header特征
	// 50-79: Body特征
	// 80-127: 行为特征

	urlScore := avgSlice(features[0:20])
	headerScore := avgSlice(features[20:50])
	bodyScore := avgSlice(features[50:80])
	behaviorScore := avgSlice(features[80:])

	// 根据不同区域的分数判断威胁类型
	if bodyScore > 0.7 {
		return "sql_injection"
	}
	if headerScore > 0.7 {
		return "xss"
	}
	if urlScore > 0.7 {
		return "path_traversal"
	}
	if behaviorScore > 0.7 {
		return "bot"
	}

	return "unknown"
}

// avgSlice 计算切片平均值
func avgSlice(s []float32) float32 {
	if len(s) == 0 {
		return 0
	}
	var sum float32
	for _, v := range s {
		sum += v
	}
	return sum / float32(len(s))
}

// Close 关闭模型
func (m *LightweightModel) Close() {
	// LightweightModel 无需清理资源，此方法仅为实现接口
}

// GetLabels 获取标签
func (m *LightweightModel) GetLabels() []string {
	return m.labels
}

// GetVersion 获取版本
func (m *LightweightModel) GetVersion() string {
	return "lightweight-1.0.0"
}

// loadLabels 加载标签
func loadLabels(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var labels []string
	if err := json.Unmarshal(data, &labels); err != nil {
		return nil, err
	}

	return labels, nil
}

// getDefaultLabels 获取默认标签
func getDefaultLabels() []string {
	return []string{
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
}

// ModelInfo 模型信息
type ModelInfo struct {
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	InputSize int       `json:"input_size"`
	Labels    []string  `json:"labels"`
	LoadedAt  time.Time `json:"loaded_at"`
}

// GetModelInfo 获取模型信息
func GetModelInfo(model ThreatModel) *ModelInfo {
	if model == nil {
		return nil
	}

	return &ModelInfo{
		Name:      "Threat Detection Model",
		Version:   model.GetVersion(),
		InputSize: 128,
		Labels:    model.GetLabels(),
		LoadedAt:  time.Now(),
	}
}
