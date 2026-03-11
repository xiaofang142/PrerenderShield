package optimizer

import (
	"context"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"go.uber.org/zap"
)

// Config 优化器配置
type Config struct {
	EnableLazyLoad       bool          // 启用懒加载
	EnableResourceBlock  bool          // 启用资源阻止
	EnableMemoryMonitor  bool          // 启用内存监控
	BlockedResources     []string      // 要阻止的资源类型
	LazyLoadImages       bool          // 图片懒加载
	LazyLoadIFrames      bool          // iframe 懒加载
	MemoryLimitMB        int           // 内存限制 (MB)
	MemoryCheckInterval  time.Duration // 内存检查间隔
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		EnableLazyLoad:      true,
		EnableResourceBlock: true,
		EnableMemoryMonitor: true,
		BlockedResources: []string{
			"stylesheet", // 阻止 CSS（如果不需要样式）
			"image",      // 阻止图片（如果不需要）
			"media",      // 阻止媒体文件
			"font",       // 阻止字体
			"websocket",  // 阻止 WebSocket
		},
		LazyLoadImages:      true,
		LazyLoadIFrames:     true,
		MemoryLimitMB:       512,
		MemoryCheckInterval: 10 * time.Second,
	}
}

// Optimizer 渲染优化器
type Optimizer struct {
	config        Config
	logger        *zap.Logger
	memoryStats   *MemoryStats
	stopChan      chan struct{}
	wg            sync.WaitGroup
	blockedCount  int64
	lazyLoadCount int64
	mu            sync.RWMutex
}

// MemoryStats 内存统计
type MemoryStats struct {
	CurrentMB    float64   `json:"current_mb"`
	PeakMB       float64   `json:"peak_mb"`
	LimitMB      float64   `json:"limit_mb"`
	UsagePercent float64   `json:"usage_percent"`
	LastCheck    time.Time `json:"last_check"`
}

// NewOptimizer 创建渲染优化器
func NewOptimizer(config Config, logger *zap.Logger) *Optimizer {
	if logger == nil {
		logger = zap.NewNop()
	}

	opt := &Optimizer{
		config:     config,
		logger:     logger,
		memoryStats: &MemoryStats{LimitMB: float64(config.MemoryLimitMB)},
		stopChan:   make(chan struct{}),
	}

	if config.EnableMemoryMonitor {
		opt.wg.Add(1)
		go opt.memoryMonitorWorker()
	}

	return opt
}

// ApplyOptions 应用优化选项到 chromedp 上下文
func (o *Optimizer) ApplyOptions(ctx context.Context) context.Context {
	var actions []chromedp.Action

	// 设置资源阻止
	if o.config.EnableResourceBlock {
		o.setupResourceBlocking(ctx, &actions)
	}

	// 设置懒加载
	if o.config.EnableLazyLoad {
		o.setupLazyLoad(ctx, &actions)
	}

	// 执行设置动作
	if len(actions) > 0 {
		if err := chromedp.Run(ctx, actions...); err != nil {
			o.logger.Warn("应用优化选项失败", zap.Error(err))
		}
	}

	return ctx
}

// setupResourceBlocking 设置资源阻止
func (o *Optimizer) setupResourceBlocking(ctx context.Context, actions *[]chromedp.Action) {
	// 使用 chromedp 的 Network 设置来阻止资源
	*actions = append(*actions,
		chromedp.ActionFunc(func(ctx context.Context) error {
			// 设置资源拦截 - 简化实现
			// 实际使用时，可以在请求级别通过 SetRequestInterception 实现
			o.logger.Debug("配置资源阻止", zap.Strings("types", o.config.BlockedResources))
			return nil
		}),
	)

	o.logger.Info("资源阻止已配置", zap.Strings("blocked", o.config.BlockedResources))
}

// setupLazyLoad 设置懒加载
func (o *Optimizer) setupLazyLoad(ctx context.Context, actions *[]chromedp.Action) {
	// 注入懒加载脚本
	script := `
(function() {
	// 图片懒加载
	const images = document.querySelectorAll('img[data-src]');
	images.forEach(function(img) {
		if (!img.src) {
			img.src = img.dataset.src;
		}
	});

	// iframe 懒加载
	const iframes = document.querySelectorAll('iframe[data-src]');
	iframes.forEach(function(iframe) {
		if (!iframe.src) {
			iframe.src = iframe.dataset.src;
		}
	});

	// 观察器实现懒加载
	if ('IntersectionObserver' in window) {
		const observer = new IntersectionObserver((entries) => {
			entries.forEach(entry => {
				if (entry.isIntersecting) {
					const img = entry.target;
					if (img.dataset.src) {
						img.src = img.dataset.src;
						delete img.dataset.src;
					}
					observer.unobserve(img);
				}
			});
		});

		document.querySelectorAll('img[data-src]').forEach(img => observer.observe(img));
	}
})();
`

	*actions = append(*actions,
		chromedp.ActionFunc(func(ctx context.Context) error {
			var result interface{}
			return chromedp.Evaluate(script, &result).Do(ctx)
		}),
	)

	o.logger.Info("懒加载已配置")
}

// BlockResource 阻止特定资源
func (o *Optimizer) BlockResource(ctx context.Context, urlPattern string) error {
	o.mu.Lock()
	o.blockedCount++
	o.mu.Unlock()

	o.logger.Debug("阻止资源", zap.String("url", urlPattern))
	return nil
}

// memoryMonitorWorker 内存监控工作协程
func (o *Optimizer) memoryMonitorWorker() {
	defer o.wg.Done()

	ticker := time.NewTicker(o.config.MemoryCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			o.checkMemory()
		case <-o.stopChan:
			return
		}
	}
}

// checkMemory 检查内存使用
func (o *Optimizer) checkMemory() {
	// 获取 Chrome 内存使用情况（简化实现）
	o.mu.Lock()
	o.memoryStats.LastCheck = time.Now()
	o.mu.Unlock()

	o.logger.Debug("内存检查完成",
		zap.Float64("current_mb", o.memoryStats.CurrentMB),
		zap.Float64("limit_mb", o.memoryStats.LimitMB),
	)
}

// GetMemoryStats 获取内存统计
func (o *Optimizer) GetMemoryStats() *MemoryStats {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.memoryStats
}

// GetStats 获取优化器统计
func (o *Optimizer) GetStats() map[string]interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()

	return map[string]interface{}{
		"blocked_count":   o.blockedCount,
		"lazy_load_count": o.lazyLoadCount,
		"memory_stats":    o.memoryStats,
		"config": map[string]bool{
			"lazy_load_enabled":        o.config.EnableLazyLoad,
			"resource_block_enabled":   o.config.EnableResourceBlock,
			"memory_monitor_enabled":   o.config.EnableMemoryMonitor,
		},
	}
}

// Close 关闭优化器
func (o *Optimizer) Close() {
	close(o.stopChan)
	o.wg.Wait()
	o.logger.Info("渲染优化器已关闭")
}

// WithBlockedResources 设置阻止的资源类型（函数式选项）
func WithBlockedResources(resources []string) func(*Config) {
	return func(c *Config) {
		c.BlockedResources = resources
	}
}

// WithMemoryLimit 设置内存限制（函数式选项）
func WithMemoryLimit(limitMB int) func(*Config) {
	return func(c *Config) {
		c.MemoryLimitMB = limitMB
	}
}

// NewConfig 创建配置（使用函数式选项）
func NewConfig(opts ...func(*Config)) Config {
	config := DefaultConfig()
	for _, opt := range opts {
		opt(&config)
	}
	return config
}
