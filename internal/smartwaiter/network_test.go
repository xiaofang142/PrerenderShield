package smartwaiter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultNetworkConfig(t *testing.T) {
	config := DefaultNetworkConfig()

	assert.NotNil(t, config)
	assert.Equal(t, true, config.EnableDetection)
	assert.Equal(t, 30*time.Second, config.DetectionInterval)
	assert.Equal(t, 100*time.Millisecond, config.GoodLatency)
	assert.Equal(t, 500*time.Millisecond, config.PoorLatency)
	assert.Equal(t, 10.0, config.GoodBandwidth)
	assert.Equal(t, 1.0, config.PoorBandwidth)
}

func TestNewNetworkDetector(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultNetworkConfig()

	detector := NewNetworkDetector(config, logger)

	assert.NotNil(t, detector)
	assert.Equal(t, config, detector.config)
	assert.NotNil(t, detector.metrics)
}

func TestNewNetworkDetector_NilConfig(t *testing.T) {
	detector := NewNetworkDetector(nil, nil)

	assert.NotNil(t, detector)
	assert.Equal(t, true, detector.config.EnableDetection)
}

func TestNetworkDetector_DetectNetwork(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)

	ctx := context.Background()
	metrics := detector.DetectNetwork(ctx)

	assert.NotNil(t, metrics)
	assert.True(t, metrics.IsOnline)
	assert.Greater(t, metrics.QualityScore, 0.0)
	assert.NotEmpty(t, metrics.NetworkType)
}

func TestNetworkDetector_GetNetworkMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)

	// 先检测一次
	ctx := context.Background()
	detector.DetectNetwork(ctx)

	metrics := detector.GetNetworkMetrics()

	assert.NotNil(t, metrics)
	assert.Equal(t, metrics, detector.metrics)
}

func TestNetworkDetector_GetNetworkQuality(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)

	// 模拟优秀网络 (80+ 分)
	detector.SimulateNetworkCondition(NetworkType5G, 20*time.Millisecond, 100.0)
	quality := detector.GetNetworkQuality()
	assert.Equal(t, NetworkQualityExcellent, quality)

	// 模拟良好网络 (60-79 分): 延迟 150-200ms, 带宽 4-5Mbps
	detector.SimulateNetworkCondition(NetworkType4G, 180*time.Millisecond, 4.5)
	quality = detector.GetNetworkQuality()
	assert.Equal(t, NetworkQualityGood, quality)

	// 模拟一般网络 (40-59 分): 延迟 200-400ms, 带宽 3-5Mbps
	detector.SimulateNetworkCondition(NetworkType4G, 300*time.Millisecond, 4.0)
	quality = detector.GetNetworkQuality()
	assert.Equal(t, NetworkQualityFair, quality)

	// 模拟差网络 (20-39 分): 延迟 300-500ms, 带宽 1-2Mbps
	detector.SimulateNetworkCondition(NetworkType3G, 400*time.Millisecond, 1.8)
	quality = detector.GetNetworkQuality()
	assert.Equal(t, NetworkQualityPoor, quality)

	// 模拟很差网络 (1-19 分): 延迟>1000ms, 带宽<0.5Mbps
	detector.SimulateNetworkCondition(NetworkType2G, 1200*time.Millisecond, 0.3)
	quality = detector.GetNetworkQuality()
	assert.Equal(t, NetworkQualityVeryPoor, quality)
}

func TestNetworkDetector_GetNetworkQuality_Offline(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)

	detector.SetOnline(false)
	quality := detector.GetNetworkQuality()

	assert.Equal(t, NetworkQualityOffline, quality)
}

func TestNetworkDetector_GetWaitStrategy(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)

	// 优秀网络 - 无需等待
	detector.SimulateNetworkCondition(NetworkType5G, 20*time.Millisecond, 100.0)
	strategy := detector.GetWaitStrategy()
	assert.Equal(t, WaitStrategyNone, strategy)

	// 一般网络 - 短暂等待
	detector.SimulateNetworkCondition(NetworkType4G, 300*time.Millisecond, 4.0)
	strategy = detector.GetWaitStrategy()
	assert.Equal(t, WaitStrategyShort, strategy)

	// 差网络 - 中等等待
	detector.SimulateNetworkCondition(NetworkType3G, 400*time.Millisecond, 1.8)
	strategy = detector.GetWaitStrategy()
	assert.Equal(t, WaitStrategyMedium, strategy)

	// 很差网络 - 长等待
	detector.SimulateNetworkCondition(NetworkType2G, 1200*time.Millisecond, 0.3)
	strategy = detector.GetWaitStrategy()
	assert.Equal(t, WaitStrategyLong, strategy)
}

func TestNetworkDetector_GetFallbackStrategy(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)

	// 优秀网络 - 无降级
	detector.SimulateNetworkCondition(NetworkType5G, 20*time.Millisecond, 100.0)
	strategy := detector.GetFallbackStrategy()
	assert.Equal(t, FallbackStrategyNone, strategy)

	// 一般网络 - 精简模式
	detector.SimulateNetworkCondition(NetworkType4G, 300*time.Millisecond, 4.0)
	strategy = detector.GetFallbackStrategy()
	assert.Equal(t, FallbackStrategyLite, strategy)

	// 差网络 - 基础模式
	detector.SimulateNetworkCondition(NetworkType3G, 400*time.Millisecond, 1.8)
	strategy = detector.GetFallbackStrategy()
	assert.Equal(t, FallbackStrategyBasic, strategy)

	// 很差网络 - 最小模式
	detector.SimulateNetworkCondition(NetworkType2G, 1200*time.Millisecond, 0.3)
	strategy = detector.GetFallbackStrategy()
	assert.Equal(t, FallbackStrategyMinimal, strategy)
}

func TestNetworkDetector_GetWaitDuration(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)

	baseDuration := 1 * time.Second

	// 优秀网络 - 减半
	detector.SimulateNetworkCondition(NetworkType5G, 20*time.Millisecond, 100.0)
	duration := detector.GetWaitDuration(baseDuration)
	assert.LessOrEqual(t, duration, 500*time.Millisecond)

	// 差网络 - 加倍
	detector.SimulateNetworkCondition(NetworkType3G, 400*time.Millisecond, 2.0)
	duration = detector.GetWaitDuration(baseDuration)
	assert.GreaterOrEqual(t, duration, 2*time.Second)
}

func TestNetworkDetector_ShouldLazyLoad(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)

	// 优秀网络 - 不应该懒加载
	detector.SimulateNetworkCondition(NetworkType5G, 20*time.Millisecond, 100.0)
	assert.False(t, detector.ShouldLazyLoad())

	// 良好网络 - 不应该懒加载
	detector.SimulateNetworkCondition(NetworkType4G, 85*time.Millisecond, 6.0)
	assert.False(t, detector.ShouldLazyLoad())

	// 一般网络 - 应该懒加载
	detector.SimulateNetworkCondition(NetworkType4G, 250*time.Millisecond, 3.0)
	assert.True(t, detector.ShouldLazyLoad())

	// 差网络 - 应该懒加载
	detector.SimulateNetworkCondition(NetworkType3G, 450*time.Millisecond, 1.2)
	assert.True(t, detector.ShouldLazyLoad())
}

func TestNetworkDetector_ShouldPreload(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)

	// 优秀网络 - 应该预加载
	detector.SimulateNetworkCondition(NetworkType5G, 20*time.Millisecond, 100.0)
	assert.True(t, detector.ShouldPreload())

	// 良好网络 - 应该预加载
	detector.SimulateNetworkCondition(NetworkType4G, 85*time.Millisecond, 6.0)
	assert.True(t, detector.ShouldPreload())

	// 一般网络 - 不应该预加载
	detector.SimulateNetworkCondition(NetworkType4G, 250*time.Millisecond, 3.0)
	assert.False(t, detector.ShouldPreload())
}

func TestNetworkDetector_IsConnectionGood(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)

	// 良好连接
	detector.SimulateNetworkCondition(NetworkType5G, 50*time.Millisecond, 20.0)
	assert.True(t, detector.IsConnectionGood())

	// 差连接
	detector.SimulateNetworkCondition(NetworkType3G, 600*time.Millisecond, 0.5)
	assert.False(t, detector.IsConnectionGood())
}

func TestNetworkDetector_calculateQualityScore(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)

	// 优秀网络
	metrics := &NetworkMetrics{
		Latency:    30 * time.Millisecond,
		Bandwidth:  50.0,
		PacketLoss: 0.01,
	}
	score := detector.calculateQualityScore(metrics)
	assert.Greater(t, score, 80.0)

	// 差网络
	metrics = &NetworkMetrics{
		Latency:    1000 * time.Millisecond,
		Bandwidth:  0.5,
		PacketLoss: 10.0,
	}
	score = detector.calculateQualityScore(metrics)
	assert.Less(t, score, 40.0)
}

func TestNetworkDetector_determineNetworkType(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)

	// 5G
	metrics := &NetworkMetrics{
		Latency:   20 * time.Millisecond,
		Bandwidth: 100.0,
	}
	networkType := detector.determineNetworkType(metrics)
	assert.Equal(t, NetworkType5G, networkType)

	// 4G
	metrics = &NetworkMetrics{
		Latency:   80 * time.Millisecond,
		Bandwidth: 20.0,
	}
	networkType = detector.determineNetworkType(metrics)
	assert.Equal(t, NetworkType4G, networkType)

	// 3G
	metrics = &NetworkMetrics{
		Latency:   200 * time.Millisecond,
		Bandwidth: 3.0,
	}
	networkType = detector.determineNetworkType(metrics)
	assert.Equal(t, NetworkType3G, networkType)

	// 2G
	metrics = &NetworkMetrics{
		Latency:   1000 * time.Millisecond,
		Bandwidth: 0.5,
	}
	networkType = detector.determineNetworkType(metrics)
	assert.Equal(t, NetworkType2G, networkType)
}

func TestNetworkDetector_Subscribe(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)

	ctx := context.Background()
	ch := detector.Subscribe()

	assert.NotNil(t, ch)

	// 检测网络应该发送指标到渠道
	detector.DetectNetwork(ctx)

	// 从渠道接收
	select {
	case metrics := <-ch:
		assert.NotNil(t, metrics)
	case <-time.After(100 * time.Millisecond):
		t.Error("Should receive metrics from channel")
	}
}

func TestNetworkDetector_SetOnline(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)

	ctx := context.Background()
	detector.DetectNetwork(ctx)

	detector.SetOnline(false)

	metrics := detector.GetNetworkMetrics()
	assert.False(t, metrics.IsOnline)
	assert.Equal(t, 0.0, metrics.QualityScore)
}

func TestNetworkDetector_GetEffectiveConnectionType(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)

	// 优秀网络 (80+ 分) -> "4g"：延迟<80ms，带宽>=7Mbps
	detector.SimulateNetworkCondition(NetworkType5G, 50*time.Millisecond, 15.0)
	assert.Equal(t, "4g", detector.GetEffectiveConnectionType())

	// 良好网络 (60-79 分) -> "3g"：延迟 80-250ms，带宽 3-7Mbps
	detector.SimulateNetworkCondition(NetworkType4G, 120*time.Millisecond, 6.0)
	assert.Equal(t, "3g", detector.GetEffectiveConnectionType())

	// 一般网络 (40-59 分) -> "2g"：延迟 250-500ms，带宽 1.5-3Mbps
	detector.SimulateNetworkCondition(NetworkType4G, 200*time.Millisecond, 4.0)
	assert.Equal(t, "2g", detector.GetEffectiveConnectionType())

	// 差网络 (20-39 分) -> "slow-2g"：延迟 300-500ms，带宽 1-1.5Mbps
	detector.SimulateNetworkCondition(NetworkType3G, 400*time.Millisecond, 1.2)
	assert.Equal(t, "slow-2g", detector.GetEffectiveConnectionType())

	// offline
	detector.SetOnline(false)
	assert.Equal(t, "offline", detector.GetEffectiveConnectionType())
}

func TestNetworkDetector_GetRecommendedImageQuality(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)

	// 优秀网络 (80+ 分) - 100% 质量
	detector.SimulateNetworkCondition(NetworkType5G, 50*time.Millisecond, 15.0)
	assert.Equal(t, 1.0, detector.GetRecommendedImageQuality())

	// 良好网络 (60-79 分) - 80% 质量
	detector.SimulateNetworkCondition(NetworkType4G, 120*time.Millisecond, 6.0)
	assert.Equal(t, 0.8, detector.GetRecommendedImageQuality())

	// 一般网络 (40-59 分) - 60% 质量
	detector.SimulateNetworkCondition(NetworkType4G, 200*time.Millisecond, 4.0)
	assert.Equal(t, 0.6, detector.GetRecommendedImageQuality())

	// 差网络 (20-39 分): 延迟 300-500ms, 带宽 1-1.5Mbps - 40% 质量
	detector.SimulateNetworkCondition(NetworkType3G, 400*time.Millisecond, 1.2)
	assert.Equal(t, 0.4, detector.GetRecommendedImageQuality())

	// 很差网络 (0-19 分): 延迟>=1000ms 或带宽<0.5Mbps - 20% 质量
	detector.SimulateNetworkCondition(NetworkType2G, 1200*time.Millisecond, 0.3)
	assert.Equal(t, 0.2, detector.GetRecommendedImageQuality())
}

func TestNetworkDetector_GetMaxConcurrentConnections(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)

	// 优秀网络 (80+ 分) - 10 个连接
	detector.SimulateNetworkCondition(NetworkType5G, 50*time.Millisecond, 15.0)
	assert.Equal(t, 10, detector.GetMaxConcurrentConnections())

	// 良好网络 (60-79 分) - 6 个连接
	detector.SimulateNetworkCondition(NetworkType4G, 120*time.Millisecond, 6.0)
	assert.Equal(t, 6, detector.GetMaxConcurrentConnections())

	// 一般网络 (40-59 分) - 4 个连接
	detector.SimulateNetworkCondition(NetworkType4G, 200*time.Millisecond, 4.0)
	assert.Equal(t, 4, detector.GetMaxConcurrentConnections())

	// 差网络 (20-39 分): 延迟 300-500ms, 带宽 1-1.5Mbps - 2 个连接
	detector.SimulateNetworkCondition(NetworkType3G, 400*time.Millisecond, 1.2)
	assert.Equal(t, 2, detector.GetMaxConcurrentConnections())

	// 很差网络 (0-19 分): 延迟>=1000ms 或带宽<0.5Mbps - 1 个连接
	detector.SimulateNetworkCondition(NetworkType2G, 1200*time.Millisecond, 0.3)
	assert.Equal(t, 1, detector.GetMaxConcurrentConnections())
}

func TestNetworkDetector_GetConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)

	config := detector.GetConfig()

	assert.NotNil(t, config)
	assert.Equal(t, true, config.EnableDetection)
}

func TestNetworkType_Constants(t *testing.T) {
	assert.Equal(t, NetworkType("wifi"), NetworkTypeWiFi)
	assert.Equal(t, NetworkType("5g"), NetworkType5G)
	assert.Equal(t, NetworkType("4g"), NetworkType4G)
	assert.Equal(t, NetworkType("3g"), NetworkType3G)
	assert.Equal(t, NetworkType("2g"), NetworkType2G)
	assert.Equal(t, NetworkType("ethernet"), NetworkTypeEthernet)
	assert.Equal(t, NetworkType("unknown"), NetworkTypeUnknown)
}

func TestNetworkQuality_Constants(t *testing.T) {
	assert.Equal(t, NetworkQuality(5), NetworkQualityExcellent)
	assert.Equal(t, NetworkQuality(4), NetworkQualityGood)
	assert.Equal(t, NetworkQuality(3), NetworkQualityFair)
	assert.Equal(t, NetworkQuality(2), NetworkQualityPoor)
	assert.Equal(t, NetworkQuality(1), NetworkQualityVeryPoor)
	assert.Equal(t, NetworkQuality(0), NetworkQualityOffline)
}

func TestWaitStrategy_Constants(t *testing.T) {
	assert.Equal(t, WaitStrategy(0), WaitStrategyNone)
	assert.Equal(t, WaitStrategy(1), WaitStrategyShort)
	assert.Equal(t, WaitStrategy(2), WaitStrategyMedium)
	assert.Equal(t, WaitStrategy(3), WaitStrategyLong)
	assert.Equal(t, WaitStrategy(4), WaitStrategyLazy)
}

func TestFallbackStrategy_Constants(t *testing.T) {
	assert.Equal(t, FallbackStrategy(0), FallbackStrategyNone)
	assert.Equal(t, FallbackStrategy(1), FallbackStrategyLite)
	assert.Equal(t, FallbackStrategy(2), FallbackStrategyBasic)
	assert.Equal(t, FallbackStrategy(3), FallbackStrategyMinimal)
}
