package smartwaiter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultElementWaitConfig(t *testing.T) {
	config := DefaultElementWaitConfig()

	assert.NotNil(t, config)
	assert.Equal(t, true, config.EnableWait)
	assert.Equal(t, 30*time.Second, config.DefaultTimeout)
	assert.Equal(t, 100*time.Millisecond, config.PollInterval)
	assert.Equal(t, true, config.EnableVisibility)
	assert.Equal(t, true, config.EnableIntersection)
	assert.Greater(t, config.MaxWaitElements, 0)
}

func TestNewElementWaiter(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultElementWaitConfig()
	detector := NewNetworkDetector(nil, logger)

	waiter := NewElementWaiter(config, logger, detector)

	assert.NotNil(t, waiter)
	assert.Equal(t, config, waiter.config)
	assert.NotNil(t, waiter.detector)
}

func TestNewElementWaiter_NilConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	waiter := NewElementWaiter(nil, logger, nil)

	assert.NotNil(t, waiter)
	assert.Equal(t, true, waiter.config.EnableWait)
	assert.NotNil(t, waiter.detector)
}

func TestElementWaiter_WaitForElement(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	waiter := NewElementWaiter(nil, logger, nil)

	ctx := context.Background()
	result := waiter.WaitForElement(ctx, ".test-element", nil)

	assert.NotNil(t, result)
	assert.Equal(t, ".test-element", result.Selector)
	assert.Equal(t, true, result.Found)
}

func TestElementWaiter_WaitForElement_Disabled(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &ElementWaitConfig{
		EnableWait: false,
	}
	waiter := NewElementWaiter(config, logger, nil)

	ctx := context.Background()
	result := waiter.WaitForElement(ctx, ".test-element", nil)

	assert.NotNil(t, result)
	assert.Equal(t, true, result.Found)
}

func TestElementWaiter_WaitForElement_WithContext(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	waiter := NewElementWaiter(nil, logger, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := waiter.WaitForElement(ctx, ".test-element", &WaitForElementOptions{
		Timeout: 50 * time.Millisecond,
	})

	assert.NotNil(t, result)
	// 可能超时或取消
	assert.Contains(t, []string{"timeout", "cancelled", ""}, result.Error)
}

func TestElementWaiter_WaitForElements(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	waiter := NewElementWaiter(nil, logger, nil)

	ctx := context.Background()
	selectors := []string{".element-1", ".element-2", ".element-3"}

	results := waiter.WaitForElements(ctx, selectors, nil)

	assert.NotNil(t, results)
	assert.Len(t, results, 3)
	for i, result := range results {
		assert.Equal(t, selectors[i], result.Selector)
	}
}

func TestElementWaiter_WaitForElements_MaxLimit(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &ElementWaitConfig{
		EnableWait:     true,
		PollInterval:   100 * time.Millisecond,
		MaxWaitElements: 5,
	}
	waiter := NewElementWaiter(config, logger, nil)

	ctx := context.Background()
	selectors := make([]string, 10)
	for i := 0; i < 10; i++ {
		selectors[i] = ".element-" + string(rune('a'+i))
	}

	results := waiter.WaitForElements(ctx, selectors, nil)

	assert.NotNil(t, results)
	assert.Len(t, results, 5) // 应该被限制在 5 个
}

func TestElementWaiter_ShouldWaitForElement(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)
	waiter := NewElementWaiter(nil, logger, detector)

	// 优秀网络 - 等待所有元素
	detector.SimulateNetworkCondition(NetworkType5G, 20*time.Millisecond, 100.0)
	assert.True(t, waiter.ShouldWaitForElement("any-element"))

	// 良好网络 - 等待关键元素
	detector.SimulateNetworkCondition(NetworkType4G, 180*time.Millisecond, 4.5)
	assert.True(t, waiter.ShouldWaitForElement("above-fold-content"))
	assert.True(t, waiter.ShouldWaitForElement("main-image"))
	assert.True(t, waiter.ShouldWaitForElement("primary-button"))

	// 一般网络 - 只等待必要元素
	detector.SimulateNetworkCondition(NetworkType4G, 300*time.Millisecond, 4.0)
	assert.True(t, waiter.ShouldWaitForElement("above-fold-content"))
	assert.True(t, waiter.ShouldWaitForElement("main-content"))
}

func TestElementWaiter_isCriticalElement(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	waiter := NewElementWaiter(nil, logger, nil)

	criticalElements := []string{
		"above-fold-content",
		"main-image",
		"primary-button",
		"navigation",
		"hero-section",
	}

	for _, elementType := range criticalElements {
		assert.True(t, waiter.isCriticalElement(elementType))
	}

	assert.False(t, waiter.isCriticalElement("footer"))
	assert.False(t, waiter.isCriticalElement("sidebar"))
}

func TestElementWaiter_isEssentialElement(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	waiter := NewElementWaiter(nil, logger, nil)

	essentialElements := []string{
		"above-fold-content",
		"main-content",
	}

	for _, elementType := range essentialElements {
		assert.True(t, waiter.isEssentialElement(elementType))
	}

	assert.False(t, waiter.isEssentialElement("footer"))
	assert.False(t, waiter.isEssentialElement("advertisement"))
}

func TestElementWaiter_GetAdaptiveTimeout(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)
	waiter := NewElementWaiter(nil, logger, detector)

	baseTimeout := 1 * time.Second

	// 优秀网络 - 超时减半
	detector.SimulateNetworkCondition(NetworkType5G, 20*time.Millisecond, 100.0)
	timeout := waiter.GetAdaptiveTimeout(baseTimeout)
	assert.LessOrEqual(t, timeout, 500*time.Millisecond)

	// 差网络 - 超时加倍
	detector.SimulateNetworkCondition(NetworkType3G, 600*time.Millisecond, 1.8)
	timeout = waiter.GetAdaptiveTimeout(baseTimeout)
	assert.GreaterOrEqual(t, timeout, 2*time.Second)
}

func TestElementWaiter_GetConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &ElementWaitConfig{
		EnableWait:     true,
		DefaultTimeout: 60 * time.Second,
	}
	waiter := NewElementWaiter(config, logger, nil)

	retrievedConfig := waiter.GetConfig()

	assert.NotNil(t, retrievedConfig)
	assert.Equal(t, 60*time.Second, retrievedConfig.DefaultTimeout)
}

func TestElementWaiter_GetDetector(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNetworkDetector(nil, logger)
	waiter := NewElementWaiter(nil, logger, detector)

	retrievedDetector := waiter.GetDetector()

	assert.NotNil(t, retrievedDetector)
	assert.Equal(t, detector, retrievedDetector)
}

func TestElementWaiter_WaitForElementOptions(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	waiter := NewElementWaiter(nil, logger, nil)

	ctx := context.Background()

	// 测试需要可见的选项
	result := waiter.WaitForElement(ctx, ".visible-element", &WaitForElementOptions{
		Timeout:    5 * time.Second,
		Visible:    true,
		InViewport: false,
	})

	assert.NotNil(t, result)
	assert.Equal(t, ".visible-element", result.Selector)

	// 测试需要在视口的选项
	result = waiter.WaitForElement(ctx, ".viewport-element", &WaitForElementOptions{
		Timeout:    5 * time.Second,
		Visible:    true,
		InViewport: true,
	})

	assert.NotNil(t, result)
	assert.Equal(t, ".viewport-element", result.Selector)
}

func TestElementWaiter_WaitForResult_Fields(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	waiter := NewElementWaiter(nil, logger, nil)

	ctx := context.Background()
	result := waiter.WaitForElement(ctx, ".test-element", nil)

	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Selector)
	assert.GreaterOrEqual(t, result.WaitDuration, time.Duration(0))
}

func TestElementWaitConfig_Fields(t *testing.T) {
	config := &ElementWaitConfig{
		EnableWait:        true,
		DefaultTimeout:    60 * time.Second,
		PollInterval:      200 * time.Millisecond,
		EnableVisibility:  true,
		EnableIntersection: true,
		MaxWaitElements:   20,
	}

	assert.Equal(t, true, config.EnableWait)
	assert.Equal(t, 60*time.Second, config.DefaultTimeout)
	assert.Equal(t, 200*time.Millisecond, config.PollInterval)
	assert.Equal(t, true, config.EnableVisibility)
	assert.Equal(t, true, config.EnableIntersection)
	assert.Equal(t, 20, config.MaxWaitElements)
}
