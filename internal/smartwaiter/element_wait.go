package smartwaiter

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ElementWaiter 元素等待器
type ElementWaiter struct {
	config   *ElementWaitConfig
	logger   *zap.Logger
	mu       sync.RWMutex
	detector *NetworkDetector
}

// ElementWaitConfig 元素等待配置
type ElementWaitConfig struct {
	EnableWait        bool          `json:"enable_wait"`         // 启用等待
	DefaultTimeout    time.Duration `json:"default_timeout"`     // 默认超时时间
	PollInterval      time.Duration `json:"poll_interval"`       // 轮询间隔
	EnableVisibility  bool          `json:"enable_visibility"`   // 启用可见性检测
	EnableIntersection bool         `json:"enable_intersection"` // 启用交叉观察
	MaxWaitElements   int           `json:"max_wait_elements"`   // 最大等待元素数量
}

// DefaultElementWaitConfig 返回默认配置
func DefaultElementWaitConfig() *ElementWaitConfig {
	return &ElementWaitConfig{
		EnableWait:        true,
		DefaultTimeout:    30 * time.Second,
		PollInterval:      100 * time.Millisecond,
		EnableVisibility:  true,
		EnableIntersection: true,
		MaxWaitElements:   10,
	}
}

// ElementWaitResult 元素等待结果
type ElementWaitResult struct {
	Selector     string        `json:"selector"`      // 元素选择器
	Found        bool          `json:"found"`         // 是否找到
	Visible      bool          `json:"visible"`       // 是否可见
	InViewport   bool          `json:"in_viewport"`   // 是否在视口内
	WaitDuration time.Duration `json:"wait_duration"` // 等待时长
	Error        string        `json:"error,omitempty"`
}

// WaitForElementOptions 等待元素选项
type WaitForElementOptions struct {
	Timeout       time.Duration `json:"timeout"`        // 超时时间
	Visible       bool          `json:"visible"`        // 是否需要可见
	InViewport    bool          `json:"in_viewport"`    // 是否需要在视口内
	WaitForRender bool          `json:"wait_for_render"` // 等待渲染完成
}

// NewElementWaiter 创建元素等待器
func NewElementWaiter(config *ElementWaitConfig, logger *zap.Logger, detector *NetworkDetector) *ElementWaiter {
	if config == nil {
		config = DefaultElementWaitConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	if detector == nil {
		// 创建默认网络检测器
		detector = NewNetworkDetector(nil, logger)
	}

	return &ElementWaiter{
		config:   config,
		logger:   logger,
		detector: detector,
	}
}

// WaitForElement 等待元素出现
func (w *ElementWaiter) WaitForElement(ctx context.Context, selector string, opts *WaitForElementOptions) *ElementWaitResult {
	// 只在访问配置时加锁
	w.mu.RLock()
	enableWait := w.config.EnableWait
	defaultTimeout := w.config.DefaultTimeout
	enableVisibility := w.config.EnableVisibility
	enableIntersection := w.config.EnableIntersection
	pollInterval := w.config.PollInterval
	w.mu.RUnlock()

	if !enableWait {
		return &ElementWaitResult{
			Selector: selector,
			Found:    true, // 禁用等待时直接返回找到
			Visible:  true,
		}
	}

	if opts == nil {
		opts = &WaitForElementOptions{
			Timeout:    defaultTimeout,
			Visible:    enableVisibility,
			InViewport: enableIntersection,
		}
	}

	// 根据网络质量调整超时时间
	baseTimeout := opts.Timeout
	if w.detector != nil {
		baseTimeout = w.detector.GetWaitDuration(baseTimeout)
	}

	result := &ElementWaitResult{
		Selector: selector,
	}

	startTime := time.Now()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	timeoutCtx, cancel := context.WithTimeout(ctx, baseTimeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			result.Error = "timeout"
			result.WaitDuration = time.Since(startTime)
			w.logger.Debug("元素等待超时",
				zap.String("selector", selector),
				zap.Duration("duration", result.WaitDuration),
			)
			return result

		case <-ctx.Done():
			result.Error = "cancelled"
			result.WaitDuration = time.Since(startTime)
			return result

		case <-ticker.C:
			// 模拟元素检测（实际应该使用浏览器 DOM API）
			found, visible, inViewport := w.simulateElementCheck(selector, opts)

			result.Found = found
			result.Visible = visible
			result.InViewport = inViewport

			// 检查是否满足条件
			if found {
				if opts.Visible && !visible {
					continue
				}
				if opts.InViewport && !inViewport {
					continue
				}

				result.WaitDuration = time.Since(startTime)
				w.logger.Debug("元素等待完成",
					zap.String("selector", selector),
					zap.Duration("duration", result.WaitDuration),
					zap.Bool("visible", visible),
					zap.Bool("in_viewport", inViewport),
				)
				return result
			}
		}
	}
}

// WaitForElements 批量等待多个元素
func (w *ElementWaiter) WaitForElements(ctx context.Context, selectors []string, opts *WaitForElementOptions) []*ElementWaitResult {
	// 限制最大等待元素数量
	if len(selectors) > w.config.MaxWaitElements {
		selectors = selectors[:w.config.MaxWaitElements]
	}

	results := make([]*ElementWaitResult, 0, len(selectors))

	// 简单实现：顺序等待
	for _, selector := range selectors {
		result := w.WaitForElement(ctx, selector, opts)
		results = append(results, result)
	}

	return results
}

// simulateElementCheck 模拟元素检测（实际应该使用浏览器 DOM API）
func (w *ElementWaiter) simulateElementCheck(selector string, opts *WaitForElementOptions) (found, visible, inViewport bool) {
	// 在实际浏览器环境中，这里应该使用 JavaScript 执行 DOM 查询
	// 例如：document.querySelector(selector)

	// 模拟：假设所有元素最终都会被找到
	// 实际实现应该与浏览器集成
	found = true
	visible = !opts.Visible || true // 简化：假设可见
	inViewport = !opts.InViewport || true // 简化：假设在视口内

	return found, visible, inViewport
}

// ShouldWaitForElement 判断是否需要等待元素
func (w *ElementWaiter) ShouldWaitForElement(elementType string) bool {
	if !w.config.EnableWait {
		return false
	}

	// 根据网络质量决定是否等待
	if w.detector != nil {
		quality := w.detector.GetNetworkQuality()

		// 优秀网络：等待所有元素
		if quality >= NetworkQualityExcellent {
			return true
		}
		// 良好网络：等待关键元素
		if quality >= NetworkQualityGood {
			return w.isCriticalElement(elementType)
		}
		// 一般或差网络：只等待必要元素
		if quality >= NetworkQualityFair {
			return w.isEssentialElement(elementType)
		}
		// 差网络：不等待，使用懒加载
		return false
	}

	return true
}

// isCriticalElement 判断是否是关键元素
func (w *ElementWaiter) isCriticalElement(elementType string) bool {
	criticalTypes := []string{
		"above-fold-content",
		"main-image",
		"primary-button",
		"navigation",
		"hero-section",
	}

	for _, t := range criticalTypes {
		if t == elementType {
			return true
		}
	}
	return false
}

// isEssentialElement 判断是否是必要元素
func (w *ElementWaiter) isEssentialElement(elementType string) bool {
	essentialTypes := []string{
		"above-fold-content",
		"main-content",
	}

	for _, t := range essentialTypes {
		if t == elementType {
			return true
		}
	}
	return false
}

// GetAdaptiveTimeout 获取自适应超时时间
func (w *ElementWaiter) GetAdaptiveTimeout(baseTimeout time.Duration) time.Duration {
	if w.detector == nil {
		return baseTimeout
	}
	return w.detector.GetWaitDuration(baseTimeout)
}

// GetConfig 获取配置
func (w *ElementWaiter) GetConfig() *ElementWaitConfig {
	return w.config
}

// GetDetector 获取网络检测器
func (w *ElementWaiter) GetDetector() *NetworkDetector {
	return w.detector
}
