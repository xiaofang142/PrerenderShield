package loganalyzer

import (
	"context"
	"math"
	"math/rand"
	"sort"
	"sync"
)

// IsolationForest 孤立森林异常检测器
type IsolationForest struct {
	config        *IFConfig
	trees         []*IFTrees
	featureNames  []string
	mu            sync.RWMutex
	trained       bool
	threshold     float64
	normalStats   map[string]*FeatureStats
}

// IFConfig 孤立森林配置
type IFConfig struct {
	NTrees          int     // 树的数量
	SampleSize      int     // 样本大小
	MaxHeight       int     // 树的最大高度
	Contamination   float64 // 异常比例阈值
	NumFeatures     int     // 特征数量
}

// IFTrees 孤立树
type IFTrees struct {
	root    *IFNode
	height  int
	feature int
}

// IFNode 孤立树节点
type IFNode struct {
	left      *IFNode
	right     *IFNode
	splitAttr int
	splitVal  float64
	size      int
	depth     int
}

// FeatureStats 特征统计
type FeatureStats struct {
	Mean   float64
	StdDev float64
	Min    float64
	Max    float64
	Count  int64
}

// IFResult 孤立森林检测结果
type IFResult struct {
	Score       float64 // 异常分数 (0-1), 越接近 1 越异常
	IsAnomaly   bool    // 是否异常
	Confidence  float64 // 置信度
	Reason      string  // 异常原因
}

// DefaultIFConfig 返回默认配置
func DefaultIFConfig() *IFConfig {
	return &IFConfig{
		NTrees:        100,
		SampleSize:    256,
		MaxHeight:     8,
		Contamination: 0.1,
		NumFeatures:   10,
	}
}

// NewIsolationForest 创建孤立森林
func NewIsolationForest(config *IFConfig, featureNames []string) *IsolationForest {
	if config == nil {
		config = DefaultIFConfig()
	}
	if config.MaxHeight <= 0 {
		config.MaxHeight = int(math.Log2(float64(config.SampleSize)))
	}

	return &IsolationForest{
		config:       config,
		featureNames: featureNames,
		trees:        make([]*IFTrees, 0, config.NTrees),
		normalStats:  make(map[string]*FeatureStats),
	}
}

// Fit 训练模型
func (f *IsolationForest) Fit(data [][]float64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(data) == 0 {
		return
	}

	// 计算特征统计
	f.calculateStats(data)

	// 构建孤立树
	f.buildTrees(data)

	// 计算阈值
	f.calculateThreshold(data)

	f.trained = true
}

// calculateStats 计算特征统计
func (f *IsolationForest) calculateStats(data [][]float64) {
	if len(data) == 0 || len(data[0]) == 0 {
		return
	}

	numFeatures := len(data[0])
	for i := 0; i < numFeatures; i++ {
		stats := &FeatureStats{}
		sum := 0.0

		for j, row := range data {
			if j == 0 {
				stats.Min = row[i]
				stats.Max = row[i]
			}
			val := row[i]
			sum += val
			if val < stats.Min {
				stats.Min = val
			}
			if val > stats.Max {
				stats.Max = val
			}
		}

		stats.Count = int64(len(data))
		stats.Mean = sum / float64(len(data))

		// 计算标准差
		variance := 0.0
		for _, row := range data {
			diff := row[i] - stats.Mean
			variance += diff * diff
		}
		stats.StdDev = math.Sqrt(variance / float64(len(data)))

		featureName := "feature_" + string(rune('0'+i))
		if i < len(f.featureNames) {
			featureName = f.featureNames[i]
		}
		f.normalStats[featureName] = stats
	}
}

// buildTrees 构建孤立树
func (f *IsolationForest) buildTrees(data [][]float64) {
	f.trees = make([]*IFTrees, f.config.NTrees)

	for i := 0; i < f.config.NTrees; i++ {
		// 随机采样
		sample := f.randomSample(data, f.config.SampleSize)

		// 构建单棵树
		f.trees[i] = f.buildTree(sample, 0, f.config.MaxHeight)
	}
}

// buildTree 构建单棵孤立树
func (f *IsolationForest) buildTree(data [][]float64, height, maxHeight int) *IFTrees {
	if len(data) <= 1 || height >= maxHeight {
		return &IFTrees{
			root: &IFNode{
				size:  len(data),
				depth: height,
			},
			height: height,
		}
	}

	// 随机选择特征
	numFeatures := len(data[0])
	feature := rand.Intn(numFeatures)

	// 获取特征值范围
	minVal, maxVal := data[0][feature], data[0][feature]
	for _, row := range data {
		if row[feature] < minVal {
			minVal = row[feature]
		}
		if row[feature] > maxVal {
			maxVal = row[feature]
		}
	}

	// 随机选择分割值
	splitVal := minVal + rand.Float64()*(maxVal-minVal)

	// 分割数据
	leftData, rightData := f.splitData(data, feature, splitVal)

	node := &IFNode{
		splitAttr: feature,
		splitVal:  splitVal,
		depth:     height,
	}

	// 递归构建左右子树
	if len(leftData) > 0 {
		node.left = f.buildTree(leftData, height+1, maxHeight).root
	}
	if len(rightData) > 0 {
		node.right = f.buildTree(rightData, height+1, maxHeight).root
	}

	return &IFTrees{
		root:   node,
		height: height,
	}
}

// splitData 分割数据
func (f *IsolationForest) splitData(data [][]float64, feature int, splitVal float64) ([][]float64, [][]float64) {
	left := make([][]float64, 0)
	right := make([][]float64, 0)

	for _, row := range data {
		if row[feature] < splitVal {
			left = append(left, row)
		} else {
			right = append(right, row)
		}
	}

	return left, right
}

// randomSample 随机采样
func (f *IsolationForest) randomSample(data [][]float64, size int) [][]float64 {
	if len(data) <= size {
		return data
	}

	// Fisher-Yates 洗牌算法的简化版
	indices := rand.Perm(len(data))[:size]
	sample := make([][]float64, size)
	for i, idx := range indices {
		sample[i] = data[idx]
	}

	return sample
}

// calculateThreshold 计算异常阈值
func (f *IsolationForest) calculateThreshold(data [][]float64) {
	scores := make([]float64, len(data))
	for i, row := range data {
		scores[i] = f.anomalyScore(row)
	}

	// 排序
	sort.Float64s(scores)

	// 根据 contamination 计算阈值
	idx := int(float64(len(scores)) * (1 - f.config.Contamination))
	if idx >= len(scores) {
		idx = len(scores) - 1
	}
	f.threshold = scores[idx]
}

// Predict 预测单个样本
func (f *IsolationForest) Predict(features []float64) *IFResult {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if !f.trained {
		return &IFResult{
			Score:      0.5,
			IsAnomaly:  false,
			Confidence: 0,
		}
	}

	score := f.anomalyScore(features)
	isAnomaly := score > f.threshold

	// 计算置信度
	confidence := f.calculateConfidence(score)

	// 分析异常原因
	reason := f.analyzeAnomaly(features)

	return &IFResult{
		Score:      score,
		IsAnomaly:  isAnomaly,
		Confidence: confidence,
		Reason:     reason,
	}
}

// anomalyScore 计算异常分数
func (f *IsolationForest) anomalyScore(features []float64) float64 {
	if len(f.trees) == 0 {
		return 0.5
	}

	// 计算平均路径长度
	totalPathLength := 0.0
	for _, tree := range f.trees {
		totalPathLength += f.pathLength(features, tree.root, 0)
	}
	avgPathLength := totalPathLength / float64(len(f.trees))

	// 标准化异常分数
	n := float64(f.config.SampleSize)
	c := 2*(math.Log(n-1)+0.5772156649) - 2*(n-1)/n

	if c == 0 {
		return 0.5
	}

	return math.Pow(2, -avgPathLength/c)
}

// pathLength 计算路径长度
func (f *IsolationForest) pathLength(features []float64, node *IFNode, depth int) float64 {
	if node == nil {
		return float64(depth)
	}

	if node.left == nil && node.right == nil {
		// 叶子节点，使用期望深度调整
		return float64(depth) + f.c(node.size)
	}

	if features[node.splitAttr] < node.splitVal {
		return f.pathLength(features, node.left, depth+1)
	}
	return f.pathLength(features, node.right, depth+1)
}

// c 计算期望深度
func (f *IsolationForest) c(n int) float64 {
	if n <= 1 {
		return 0
	}
	return 2*(math.Log(float64(n-1))+0.5772156649) - 2*float64(n-1)/float64(n)
}

// calculateConfidence 计算置信度
func (f *IsolationForest) calculateConfidence(score float64) float64 {
	// 基于分数与阈值的距离计算置信度
	diff := math.Abs(score - f.threshold)
	confidence := math.Min(1.0, diff*10)
	return confidence
}

// analyzeAnomaly 分析异常原因
func (f *IsolationForest) analyzeAnomaly(features []float64) string {
	reasons := make([]string, 0)

	for i, feature := range features {
		featureName := "feature_" + string(rune('0'+i))
		if i < len(f.featureNames) {
			featureName = f.featureNames[i]
		}

		stats, ok := f.normalStats[featureName]
		if !ok {
			continue
		}

		// 检查是否超出正常范围
		zScore := math.Abs((feature - stats.Mean) / stats.StdDev)
		if zScore > 3 {
			reasons = append(reasons, featureName+" 异常偏离")
		}
	}

	if len(reasons) == 0 {
		return "多特征组合异常"
	}

	return reasons[0]
}

// UpdateStats 在线更新统计
func (f *IsolationForest) UpdateStats(features []float64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i, feature := range features {
		featureName := "feature_" + string(rune('0'+i))
		if i < len(f.featureNames) {
			featureName = f.featureNames[i]
		}

		stats, ok := f.normalStats[featureName]
		if !ok {
			stats = &FeatureStats{}
			f.normalStats[featureName] = stats
		}

		// 在线更新均值和方差
		stats.Count++
		stats.Mean = stats.Mean + (feature-stats.Mean)/float64(stats.Count)
		stats.StdDev = math.Sqrt(stats.StdDev*stats.StdDev + (feature-stats.Mean)*(feature-stats.Mean)/float64(stats.Count+1))

		if feature < stats.Min {
			stats.Min = feature
		}
		if feature > stats.Max {
			stats.Max = feature
		}
	}
}

// GetStats 获取统计信息
func (f *IsolationForest) GetStats() map[string]interface{} {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return map[string]interface{}{
		"trained":        f.trained,
		"num_trees":      len(f.trees),
		"threshold":      f.threshold,
		"num_features":   len(f.featureNames),
		"contamination":  f.config.Contamination,
	}
}

// LogEntryToFeatures 将日志条目转换为特征向量
func LogEntryToFeatures(entry *LogEntry) []float64 {
	features := make([]float64, 0, 10)

	// 提取数值特征
	if status, ok := entry.Fields["status_int"]; ok {
		features = append(features, float64(toInt(status)))
	} else {
		features = append(features, 0)
	}

	if bodyBytes, ok := entry.Fields["body_bytes_int"]; ok {
		features = append(features, float64(toInt64(bodyBytes)))
	} else {
		features = append(features, 0)
	}

	if reqTime, ok := entry.Fields["request_time_ms"]; ok {
		features = append(features, toFloat64(reqTime))
	} else {
		features = append(features, 0)
	}

	if isBot, ok := entry.Fields["is_bot"]; ok {
		if toBool(isBot) {
			features = append(features, 1)
		} else {
			features = append(features, 0)
		}
	} else {
		features = append(features, 0)
	}

	// 威胁相关特征
	if threatScore, ok := entry.Fields["threat_score"]; ok {
		features = append(features, toFloat64(threatScore))
	} else {
		features = append(features, 0)
	}

	if isAnomaly, ok := entry.Fields["is_anomaly"]; ok {
		if toBool(isAnomaly) {
			features = append(features, 1)
		} else {
			features = append(features, 0)
		}
	} else {
		features = append(features, 0)
	}

	return features
}

// AnomalyDetectorProcessor 异常检测处理器
type AnomalyDetectorProcessor struct {
	name    string
	forest  *IsolationForest
	window  []*LogEntry
	mu      sync.RWMutex
}

// NewAnomalyDetectorProcessor 创建异常检测处理器
func NewAnomalyDetectorProcessor(featureNames []string) *AnomalyDetectorProcessor {
	return &AnomalyDetectorProcessor{
		name:   "anomaly_detector",
		forest: NewIsolationForest(DefaultIFConfig(), featureNames),
		window: make([]*LogEntry, 0, 1000),
	}
}

// Name 返回处理器名称
func (p *AnomalyDetectorProcessor) Name() string {
	return p.name
}

// Process 处理日志条目
func (p *AnomalyDetectorProcessor) Process(ctx context.Context, entry *LogEntry) (*LogEntry, error) {
	if entry == nil {
		return nil, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// 转换为特征向量
	features := LogEntryToFeatures(entry)

	// 如果已训练，进行预测
	if p.forest.trained {
		result := p.forest.Predict(features)
		entry.Fields["anomaly_score"] = result.Score
		entry.Fields["is_ml_anomaly"] = result.IsAnomaly
		entry.Fields["anomaly_confidence"] = result.Confidence
		entry.Fields["anomaly_reason"] = result.Reason

		if result.IsAnomaly {
			entry.Level = "warn"
		}
	}

	// 加入训练窗口
	p.window = append(p.window, entry)

	// 定期重新训练
	if len(p.window) >= 1000 && !p.forest.trained {
		p.retrain()
	} else if len(p.window) >= 5000 {
		// 增量更新
		p.forest.UpdateStats(features)
	}

	return entry, nil
}

// retrain 重新训练模型
func (p *AnomalyDetectorProcessor) retrain() {
	data := make([][]float64, 0, len(p.window))
	for _, entry := range p.window {
		features := LogEntryToFeatures(entry)
		data = append(data, features)
	}

	p.forest.Fit(data)

	// 保留最近的 1000 条
	if len(p.window) > 1000 {
		p.window = p.window[len(p.window)-1000:]
	}
}

// GetModelStats 获取模型统计
func (p *AnomalyDetectorProcessor) GetModelStats() map[string]interface{} {
	return p.forest.GetStats()
}

// Train 手动训练模型
func (p *AnomalyDetectorProcessor) Train(data [][]float64) {
	p.forest.Fit(data)
}
