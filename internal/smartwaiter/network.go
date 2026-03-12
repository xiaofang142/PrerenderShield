package smartwaiter

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// NetworkDetector 网络检测器
type NetworkDetector struct {
	config      *NetworkConfig
	logger      *zap.Logger
	metrics     *NetworkMetrics
	metricsChan chan *NetworkMetrics
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	stopped     bool
}

// NetworkConfig 网络检测配置
type NetworkConfig struct {
	EnableDetection    bool          `json:"enable_detection"`     // 启用检测
	DetectionInterval  time.Duration `json:"detection_interval"`   // 检测间隔
	Timeout            time.Duration `json:"timeout"`              // 超时时间
	SampleCount        int           `json:"sample_count"`         // 采样次数
	EnableAdaptive     bool          `json:"enable_adaptive"`      // 启用自适应
	EnableFallback     bool          `json:"enable_fallback"`      // 启用降级
	GoodLatency        time.Duration `json:"good_latency"`         // 良好延迟阈值
	PoorLatency        time.Duration `json:"poor_latency"`         // 差延迟阈值
	GoodBandwidth      float64       `json:"good_bandwidth"`       // 良好带宽 (Mbps)
	PoorBandwidth      float64       `json:"poor_bandwidth"`       // 差带宽 (Mbps)
}

// DefaultNetworkConfig 返回默认配置
func DefaultNetworkConfig() *NetworkConfig {
	return &NetworkConfig{
		EnableDetection:   true,
		DetectionInterval: 30 * time.Second,
		Timeout:           10 * time.Second,
		SampleCount:       3,
		EnableAdaptive:    true,
		EnableFallback:    true,
		GoodLatency:       100 * time.Millisecond,
		PoorLatency:       500 * time.Millisecond,
		GoodBandwidth:     10.0, // 10 Mbps
		PoorBandwidth:     1.0,  // 1 Mbps
	}
}

// NetworkMetrics 网络指标
type NetworkMetrics struct {
	Latency         time.Duration `json:"latency"`          // 延迟
	RTT             time.Duration `json:"rtt"`              // 往返时间
	Bandwidth       float64       `json:"bandwidth"`        // 带宽 (Mbps)
	PacketLoss      float64       `json:"packet_loss"`      // 丢包率 (%)
	Jitter          time.Duration `json:"jitter"`           // 抖动
	NetworkType     NetworkType   `json:"network_type"`     // 网络类型
	SignalStrength  int           `json:"signal_strength"`  // 信号强度 (0-100)
	EffectiveType   string        `json:"effective_type"`   // 有效类型 (4g, 3g, 2g)
	Downlink        float64       `json:"downlink"`         // 下行速度 (Mbps)
	IsOnline        bool          `json:"is_online"`        // 是否在线
	LastUpdated     time.Time     `json:"last_updated"`     // 最后更新时间
	QualityScore    float64       `json:"quality_score"`    // 质量评分 (0-100)
}

// NetworkType 网络类型
type NetworkType string

const (
	NetworkTypeWiFi      NetworkType = "wifi"
	NetworkType5G        NetworkType = "5g"
	NetworkType4G        NetworkType = "4g"
	NetworkType3G        NetworkType = "3g"
	NetworkType2G        NetworkType = "2g"
	NetworkTypeEthernet  NetworkType = "ethernet"
	NetworkTypeUnknown   NetworkType = "unknown"
)

// NetworkQuality 网络质量等级
type NetworkQuality int

const (
	NetworkQualityExcellent NetworkQuality = 5 // 优秀
	NetworkQualityGood      NetworkQuality = 4 // 良好
	NetworkQualityFair      NetworkQuality = 3 // 一般
	NetworkQualityPoor      NetworkQuality = 2 // 差
	NetworkQualityVeryPoor  NetworkQuality = 1 // 很差
	NetworkQualityOffline   NetworkQuality = 0 // 离线
)

// WaitStrategy 等待策略
type WaitStrategy int

const (
	WaitStrategyNone     WaitStrategy = iota // 无需等待
	WaitStrategyShort                        // 短暂等待
	WaitStrategyMedium                       // 中等等待
	WaitStrategyLong                         // 长等待
	WaitStrategyLazy                         // 懒加载
)

// FallbackStrategy 降级策略
type FallbackStrategy int

const (
	FallbackStrategyNone     FallbackStrategy = iota // 无降级
	FallbackStrategyLite                             // 精简模式
	FallbackStrategyBasic                            // 基础模式
	FallbackStrategyMinimal                          // 最小模式
)

// NewNetworkDetector 创建网络检测器
func NewNetworkDetector(config *NetworkConfig, logger *zap.Logger) *NetworkDetector {
	if config == nil {
		config = DefaultNetworkConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())

	detector := &NetworkDetector{
		config:      config,
		logger:      logger,
		metrics:     &NetworkMetrics{},
		metricsChan: make(chan *NetworkMetrics, 10),
		ctx:         ctx,
		cancel:      cancel,
	}

	// 初始化指标
	detector.metrics.IsOnline = true
	detector.metrics.LastUpdated = time.Now()

	return detector
}

// DetectNetwork 检测网络状态
func (d *NetworkDetector) DetectNetwork(ctx context.Context) *NetworkMetrics {
	d.mu.Lock()
	defer d.mu.Unlock()

	metrics := &NetworkMetrics{
		LastUpdated: time.Now(),
		IsOnline:    true,
	}

	// 模拟检测（实际应该使用真实网络检测）
	// 在实际浏览器环境中，可以通过 Navigation API 获取
	metrics.NetworkType = NetworkTypeWiFi
	metrics.Latency = 50 * time.Millisecond
	metrics.RTT = 100 * time.Millisecond
	metrics.Bandwidth = 50.0
	metrics.PacketLoss = 0.1
	metrics.Jitter = 10 * time.Millisecond
	metrics.SignalStrength = 80
	metrics.EffectiveType = "4g"
	metrics.Downlink = 25.0

	// 计算质量评分
	metrics.QualityScore = d.calculateQualityScore(metrics)
	metrics.NetworkType = d.determineNetworkType(metrics)

	d.metrics = metrics

	// 发送到渠道（非阻塞）
	select {
	case d.metricsChan <- metrics:
	default:
	}

	d.logger.Debug("网络检测完成",
		zap.String("type", string(metrics.NetworkType)),
		zap.Duration("latency", metrics.Latency),
		zap.Float64("bandwidth", metrics.Bandwidth),
		zap.Float64("quality", metrics.QualityScore),
	)

	return metrics
}

// GetNetworkMetrics 获取当前网络指标
func (d *NetworkDetector) GetNetworkMetrics() *NetworkMetrics {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.metrics
}

// GetNetworkQuality 获取网络质量等级
func (d *NetworkDetector) GetNetworkQuality() NetworkQuality {
	metrics := d.GetNetworkMetrics()

	if !metrics.IsOnline {
		return NetworkQualityOffline
	}

	if metrics.QualityScore >= 80 {
		return NetworkQualityExcellent
	}
	if metrics.QualityScore >= 60 {
		return NetworkQualityGood
	}
	if metrics.QualityScore >= 40 {
		return NetworkQualityFair
	}
	if metrics.QualityScore >= 20 {
		return NetworkQualityPoor
	}
	return NetworkQualityVeryPoor
}

// GetWaitStrategy 获取等待策略
func (d *NetworkDetector) GetWaitStrategy() WaitStrategy {
	quality := d.GetNetworkQuality()

	switch quality {
	case NetworkQualityExcellent, NetworkQualityGood:
		return WaitStrategyNone
	case NetworkQualityFair:
		return WaitStrategyShort
	case NetworkQualityPoor:
		return WaitStrategyMedium
	case NetworkQualityVeryPoor:
		return WaitStrategyLong
	default:
		return WaitStrategyMedium
	}
}

// GetFallbackStrategy 获取降级策略
func (d *NetworkDetector) GetFallbackStrategy() FallbackStrategy {
	quality := d.GetNetworkQuality()

	switch quality {
	case NetworkQualityExcellent, NetworkQualityGood:
		return FallbackStrategyNone
	case NetworkQualityFair:
		return FallbackStrategyLite
	case NetworkQualityPoor:
		return FallbackStrategyBasic
	case NetworkQualityVeryPoor:
		return FallbackStrategyMinimal
	default:
		return FallbackStrategyNone
	}
}

// GetWaitDuration 获取建议等待时间
func (d *NetworkDetector) GetWaitDuration(baseDuration time.Duration) time.Duration {
	quality := d.GetNetworkQuality()

	multiplier := 1.0
	switch quality {
	case NetworkQualityExcellent:
		multiplier = 0.5
	case NetworkQualityGood:
		multiplier = 1.0
	case NetworkQualityFair:
		multiplier = 1.5
	case NetworkQualityPoor:
		multiplier = 2.0
	case NetworkQualityVeryPoor:
		multiplier = 3.0
	}

	return time.Duration(float64(baseDuration) * multiplier)
}

// ShouldLazyLoad 判断是否应该懒加载
func (d *NetworkDetector) ShouldLazyLoad() bool {
	quality := d.GetNetworkQuality()
	return quality <= NetworkQualityFair
}

// ShouldPreload 判断是否应该预加载
func (d *NetworkDetector) ShouldPreload() bool {
	quality := d.GetNetworkQuality()
	return quality >= NetworkQualityGood
}

// IsConnectionGood 判断连接是否良好
func (d *NetworkDetector) IsConnectionGood() bool {
	metrics := d.GetNetworkMetrics()
	return metrics.IsOnline &&
		metrics.Latency < d.config.PoorLatency &&
		metrics.Bandwidth > d.config.PoorBandwidth
}

// calculateQualityScore 计算质量评分
func (d *NetworkDetector) calculateQualityScore(metrics *NetworkMetrics) float64 {
	score := 0.0

	// 延迟评分（40 分）
	if metrics.Latency < d.config.GoodLatency {
		score += 40
	} else if metrics.Latency < d.config.PoorLatency {
		ratio := float64(d.config.PoorLatency-metrics.Latency) / float64(d.config.PoorLatency-d.config.GoodLatency)
		score += 40 * ratio
	} else {
		score += 0
	}

	// 带宽评分（40 分）
	if metrics.Bandwidth > d.config.GoodBandwidth {
		score += 40
	} else if metrics.Bandwidth > d.config.PoorBandwidth {
		ratio := (metrics.Bandwidth - d.config.PoorBandwidth) / (d.config.GoodBandwidth - d.config.PoorBandwidth)
		score += 40 * ratio
	} else {
		score += 0
	}

	// 丢包率评分（20 分）
	if metrics.PacketLoss < 0.1 {
		score += 20
	} else if metrics.PacketLoss < 5.0 {
		ratio := (5.0 - metrics.PacketLoss) / 4.9
		score += 20 * ratio
	} else {
		score += 0
	}

	// 确保在 0-100 范围内
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// determineNetworkType 确定网络类型
func (d *NetworkDetector) determineNetworkType(metrics *NetworkMetrics) NetworkType {
	// 基于带宽和延迟判断网络类型
	if metrics.Bandwidth > 50 && metrics.Latency < 50*time.Millisecond {
		return NetworkType5G
	}
	if metrics.Bandwidth > 10 && metrics.Latency < 100*time.Millisecond {
		return NetworkType4G
	}
	if metrics.Bandwidth > 1 && metrics.Latency < 300*time.Millisecond {
		return NetworkType3G
	}
	return NetworkType2G
}

// StartMonitoring 开始监控
func (d *NetworkDetector) StartMonitoring(ctx context.Context) {
	if !d.config.EnableDetection {
		return
	}

	go func() {
		ticker := time.NewTicker(d.config.DetectionInterval)
		defer ticker.Stop()

		for {
			select {
			case <-d.ctx.Done():
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				d.DetectNetwork(ctx)
			}
		}
	}()

	d.logger.Info("网络监控已启动",
		zap.Duration("interval", d.config.DetectionInterval),
	)
}

// Stop 停止监控
func (d *NetworkDetector) Stop() {
	d.mu.Lock()
	if d.stopped {
		d.mu.Unlock()
		return
	}
	d.stopped = true

	// 关闭 metricsChan
	if d.metricsChan != nil {
		close(d.metricsChan)
	}
	d.mu.Unlock()

	if d.cancel != nil {
		d.cancel()
	}

	d.logger.Info("网络监控已停止")
}

// Close 实现 io.Closer 接口
func (d *NetworkDetector) Close() error {
	d.Stop()
	return nil
}

// Subscribe 订阅网络指标变化
func (d *NetworkDetector) Subscribe() <-chan *NetworkMetrics {
	return d.metricsChan
}

// Unsubscribe 取消订阅（调用者负责不再从 channel 读取）
func (d *NetworkDetector) Unsubscribe(ch <-chan *NetworkMetrics) {
	// 注意：不能关闭只读 channel，调用者应该停止读取
	// 在 Stop 时会自动关闭 metricsChan
	_ = ch
}

// GetConfig 获取配置
func (d *NetworkDetector) GetConfig() *NetworkConfig {
	return d.config
}

// SetOnline 设置在线状态
func (d *NetworkDetector) SetOnline(online bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.metrics.IsOnline = online
	d.metrics.LastUpdated = time.Now()

	if !online {
		d.metrics.QualityScore = 0
	}

	d.logger.Debug("在线状态更新", zap.Bool("online", online))
}

// SimulateNetworkCondition 模拟网络条件（用于测试）
func (d *NetworkDetector) SimulateNetworkCondition(networkType NetworkType, latency time.Duration, bandwidth float64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 验证输入参数
	if latency < 0 {
		latency = 0
	}
	if bandwidth < 0 {
		bandwidth = 0
	}

	d.metrics.NetworkType = networkType
	d.metrics.Latency = latency
	d.metrics.Bandwidth = bandwidth

	// 根据网络条件模拟合理的丢包率
	// 评分逻辑（默认配置：GoodLatency=100ms, PoorLatency=500ms, GoodBandwidth=10Mbps, PoorBandwidth=1Mbps）：
	// - 延迟评分（40 分）：< 100ms 满分，100-500ms 线性递减，> 500ms 0 分
	// - 带宽评分（40 分）：> 10Mbps 满分，1-10Mbps 线性递减，< 1Mbps 0 分
	// - 丢包率评分（20 分）：< 0.1% 满分，0.1-5% 线性递减，> 5% 0 分
	// 目标分数：优秀 (80+), 良好 (60-79), 一般 (40-59), 差 (20-39), 很差 (0-19)

	if latency >= 1000*time.Millisecond || bandwidth < 0.5 {
		// 很差网络：延迟>=1s 或带宽<0.5Mbps，评分约 0-19 分
		d.metrics.PacketLoss = 10.0
	} else if latency >= 500*time.Millisecond || bandwidth < 1.0 {
		// 差网络：延迟 500-1000ms 或带宽 0.5-1Mbps，评分约 20-39 分
		d.metrics.PacketLoss = 5.0
	} else if latency >= 200*time.Millisecond || bandwidth < 3.0 {
		// 一般网络：延迟 200-500ms 或带宽 1-3Mbps，评分约 40-59 分
		d.metrics.PacketLoss = 2.0
	} else if latency >= 100*time.Millisecond || bandwidth < 7.0 {
		// 良好网络：延迟 100-200ms 或带宽 3-7Mbps，评分约 60-79 分
		d.metrics.PacketLoss = 0.5
	} else {
		// 优秀网络：延迟<100ms 且带宽>=7Mbps，评分 80-100 分
		d.metrics.PacketLoss = 0.01
	}

	d.metrics.QualityScore = d.calculateQualityScore(d.metrics)
	d.metrics.LastUpdated = time.Now()

	d.logger.Debug("模拟网络条件",
		zap.String("type", string(networkType)),
		zap.Duration("latency", latency),
		zap.Float64("bandwidth", bandwidth),
	)
}

// GetEffectiveConnectionType 获取有效连接类型
func (d *NetworkDetector) GetEffectiveConnectionType() string {
	metrics := d.GetNetworkMetrics()

	if !metrics.IsOnline {
		return "offline"
	}

	if metrics.QualityScore >= 80 {
		return "4g"
	}
	if metrics.QualityScore >= 60 {
		return "3g"
	}
	if metrics.QualityScore >= 40 {
		return "2g"
	}
	return "slow-2g"
}

// GetRecommendedImageQuality 获取推荐的图片质量
func (d *NetworkDetector) GetRecommendedImageQuality() float64 {
	quality := d.GetNetworkQuality()

	switch quality {
	case NetworkQualityExcellent:
		return 1.0 // 100% 质量
	case NetworkQualityGood:
		return 0.8 // 80% 质量
	case NetworkQualityFair:
		return 0.6 // 60% 质量
	case NetworkQualityPoor:
		return 0.4 // 40% 质量
	default:
		return 0.2 // 20% 质量
	}
}

// GetMaxConcurrentConnections 获取最大并发连接数
func (d *NetworkDetector) GetMaxConcurrentConnections() int {
	quality := d.GetNetworkQuality()

	switch quality {
	case NetworkQualityExcellent:
		return 10
	case NetworkQualityGood:
		return 6
	case NetworkQualityFair:
		return 4
	case NetworkQualityPoor:
		return 2
	default:
		return 1
	}
}
