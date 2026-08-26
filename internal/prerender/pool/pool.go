package pool

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"go.uber.org/zap"
)

// Instance Chromium 实例
type Instance struct {
	ID           string
	AllocCtx     context.Context
	AllocCancel  context.CancelFunc
	ChromeCtx    context.Context
	ChromeCancel context.CancelFunc
	CreatedAt    time.Time
	LastUsedAt   time.Time
	UseCount     int
	MaxUseCount  int
	IsHealthy    bool
	mu           sync.RWMutex
}

// Config 实例池配置
type Config struct {
	MinInstances        int           // 最小实例数
	MaxInstances        int           // 最大实例数
	IdleTimeout         time.Duration // 空闲超时
	MaxUseCount         int           // 单个实例最大使用次数
	HealthCheckInterval time.Duration // 健康检查间隔
	Headless            bool          // 是否无头模式
	ExecPath            string        // Chromium 可执行文件路径（为空则自动查找）
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		MinInstances:        2,
		MaxInstances:        10,
		IdleTimeout:         5 * time.Minute,
		MaxUseCount:         100,
		HealthCheckInterval: 30 * time.Second,
		Headless:            true,
	}
}

// chromiumCandidates 常见 Chromium/Chrome 可执行文件候选（按优先级）
var chromiumCandidates = []string{
	"chromium",
	"chromium-browser",
	"google-chrome",
	"google-chrome-stable",
	"/usr/bin/chromium",
	"/usr/bin/chromium-browser",
	"/usr/bin/google-chrome",
	"/usr/bin/google-chrome-stable",
	"/snap/bin/chromium",
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	"/Applications/Chromium.app/Contents/MacOS/Chromium",
}

// ResolveChromiumPath 解析可用的浏览器路径。
// execPath 非空时校验该路径存在且可执行；为空时按常见候选自动查找。
// 返回解析后的路径，找不到时返回错误。
func ResolveChromiumPath(execPath string) (string, error) {
	if execPath != "" {
		info, err := os.Stat(execPath)
		if err != nil {
			return "", fmt.Errorf("configured chromium path not found: %s", execPath)
		}
		if info.IsDir() {
			return "", fmt.Errorf("configured chromium path is a directory: %s", execPath)
		}
		return execPath, nil
	}

	for _, candidate := range chromiumCandidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no chromium/chrome binary found in PATH (tried: %v); install chromium or set CHROME_PATH/PRERENDER_CHROMIUM_PATH", chromiumCandidates)
}

// Pool Chromium 实例池
type Pool struct {
	config        Config
	instances     []*Instance
	available     chan *Instance
	mu            sync.RWMutex
	closed        bool
	logger        *zap.Logger
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	instanceCount int
}

// NewPool 创建实例池
func NewPool(config Config, logger *zap.Logger) *Pool {
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())

	p := &Pool{
		config:    config,
		instances: make([]*Instance, 0, config.MaxInstances),
		available: make(chan *Instance, config.MaxInstances),
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
	}

	// 初始化最小实例数
	for i := 0; i < config.MinInstances; i++ {
		instance := p.createInstance()
		if instance != nil {
			p.instances = append(p.instances, instance)
			p.available <- instance
			p.instanceCount++
		}
	}

	// 启动健康检查协程
	p.wg.Add(1)
	go p.healthChecker()

	return p
}

// createInstance 创建新实例
func (p *Pool) createInstance() *Instance {
	// 全局进程数硬上限防护：孤儿进程累积或异常扩容时拒绝继续创建，
	// 防止无头浏览器进程拖垮宿主机
	if n := CountChromiumProcesses(); n >= p.HardProcessCap() {
		p.logger.Error("chromium process cap reached, refusing to create instance",
			zap.Int("current_processes", n),
			zap.Int("cap", p.HardProcessCap()),
			zap.String("hint", "restart the service to trigger orphan sweep, or raise PRERENDER_MAX_INSTANCES"))
		return nil
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(p.ctx, p.allocatorOptions()...)
	chromeCtx, chromeCancel := chromedp.NewContext(allocCtx)

	instance := &Instance{
		ID:           fmt.Sprintf("instance-%d", time.Now().UnixNano()),
		AllocCtx:     allocCtx,
		AllocCancel:  allocCancel,
		ChromeCtx:    chromeCtx,
		ChromeCancel: chromeCancel,
		CreatedAt:    time.Now(),
		LastUsedAt:   time.Now(),
		MaxUseCount:  p.config.MaxUseCount,
		IsHealthy:    true,
	}

	p.logger.Debug("created new chromium instance",
		zap.String("id", instance.ID))

	return instance
}

// allocatorOptions 获取 Chrome 分配器选项
func (p *Pool) allocatorOptions() []chromedp.ExecAllocatorOption {
	options := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", p.config.Headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-features", "VizDisplayCompositor"),
		chromedp.Flag("window-size", "1920,1080"),
		chromedp.Flag("js-flags", "--max-old-space-size=512 --max-heap-size=512"),
	)

	// 显式指定浏览器路径（配置或环境变量）
	if p.config.ExecPath != "" {
		options = append(options, chromedp.ExecPath(p.config.ExecPath))
	}

	// 添加性能优化选项
	options = append(options,
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("safebrowsing-disable-auto-update", true),
	)

	return options
}

// Acquire 获取可用实例（阻塞），自动跳过已关闭的陈旧实例
func (p *Pool) Acquire(ctx context.Context) (*Instance, error) {
	for {
		p.mu.RLock()
		if p.closed {
			p.mu.RUnlock()
			return nil, fmt.Errorf("pool is closed")
		}
		canCreateMore := p.instanceCount < p.config.MaxInstances
		p.mu.RUnlock()

		select {
		case instance := <-p.available:
			// 跳过已关闭的陈旧实例（上下文已被 cancel）
			instance.mu.RLock()
			closed := instance.ChromeCancel == nil
			instance.mu.RUnlock()

			if closed {
				continue
			}

			instance.mu.Lock()
			instance.LastUsedAt = time.Now()
			instance.UseCount++
			instance.mu.Unlock()

			return instance, nil

		case <-ctx.Done():
			return nil, ctx.Err()

		case <-p.ctx.Done():
			return nil, fmt.Errorf("pool is closed")

		default:
			if canCreateMore {
				p.mu.Lock()
				if p.instanceCount < p.config.MaxInstances && !p.closed {
					instance := p.createInstance()
					if instance != nil {
						p.instances = append(p.instances, instance)
						p.instanceCount++
						p.mu.Unlock()

						instance.mu.Lock()
						instance.UseCount++
						instance.mu.Unlock()

						return instance, nil
					}
				}
				p.mu.Unlock()
			}

			select {
			case instance := <-p.available:
				instance.mu.RLock()
				closed := instance.ChromeCancel == nil
				instance.mu.RUnlock()

				if closed {
					continue
				}

				instance.mu.Lock()
				instance.LastUsedAt = time.Now()
				instance.UseCount++
				instance.mu.Unlock()

				return instance, nil

			case <-ctx.Done():
				return nil, ctx.Err()

			case <-p.ctx.Done():
				return nil, fmt.Errorf("pool is closed")
			}
		}
	}
}

// AcquireWithTimeout 获取可用实例（带超时）
func (p *Pool) AcquireWithTimeout(timeout time.Duration) (*Instance, error) {
	ctx, cancel := context.WithTimeout(p.ctx, timeout)
	defer cancel()

	return p.Acquire(ctx)
}

// Release 释放实例回池
func (p *Pool) Release(instance *Instance) error {
	instance.mu.RLock()
	useCount := instance.UseCount
	health := instance.IsHealthy
	instance.mu.RUnlock()

	// 检查是否需要回收实例
	needsRetire := useCount >= instance.MaxUseCount || !health

	if needsRetire {
		p.retireInstance(instance)
		return nil
	}

	p.mu.RLock()
	closed := p.closed
	p.mu.RUnlock()

	if !closed {
		select {
		case p.available <- instance:
			p.logger.Debug("released instance back to pool",
				zap.String("id", instance.ID))
		default:
			// 池已满，关闭实例
			p.closeInstance(instance)
		}
	} else {
		p.closeInstance(instance)
	}

	return nil
}

// retireInstance 回收实例
func (p *Pool) retireInstance(instance *Instance) {
	p.mu.Lock()

	p.logger.Info("retiring instance",
		zap.String("id", instance.ID),
		zap.Int("use_count", instance.UseCount))

	for i, inst := range p.instances {
		if inst.ID == instance.ID {
			p.instances = append(p.instances[:i], p.instances[i+1:]...)
			p.instanceCount--
			break
		}
	}

	needsNewInstance := p.instanceCount < p.config.MinInstances

	p.mu.Unlock()

	// 在锁外关闭实例，避免阻塞
	p.closeInstance(instance)

	// 在锁外创建新实例
	if needsNewInstance {
		newInstance := p.createInstance()
		if newInstance != nil {
			p.mu.Lock()
			p.instances = append(p.instances, newInstance)
			p.instanceCount++
			p.mu.Unlock()

			select {
			case p.available <- newInstance:
			default:
				p.mu.Lock()
				for i, inst := range p.instances {
					if inst.ID == newInstance.ID {
						p.instances = append(p.instances[:i], p.instances[i+1:]...)
						p.instanceCount--
						break
					}
				}
				p.mu.Unlock()
				p.closeInstance(newInstance)
			}
		}
	}
}

// closeInstance 关闭实例，标记已关闭（ChromeCancel 置 nil）
func (p *Pool) closeInstance(instance *Instance) {
	instance.mu.Lock()
	defer instance.mu.Unlock()

	if instance.ChromeCancel != nil {
		instance.ChromeCancel()
		instance.ChromeCancel = nil
	}
	if instance.AllocCancel != nil {
		instance.AllocCancel()
		instance.AllocCancel = nil
	}

	p.logger.Debug("closed instance", zap.String("id", instance.ID))
}

// healthChecker 健康检查协程
func (p *Pool) healthChecker() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.checkHealth()
		}
	}
}

// checkHealth 检查实例健康状态
func (p *Pool) checkHealth() {
	p.mu.Lock()

	now := time.Now()
	var toRetire []*Instance

	for _, instance := range p.instances {
		instance.mu.Lock()

		unhealthy := false
		if instance.UseCount >= instance.MaxUseCount {
			unhealthy = true
			p.logger.Info("instance unhealthy: max use count reached",
				zap.String("id", instance.ID),
				zap.Int("use_count", instance.UseCount))
		}

		if !unhealthy && now.Sub(instance.LastUsedAt) > p.config.IdleTimeout {
			if len(p.instances) > p.config.MinInstances {
				unhealthy = true
				p.logger.Info("instance unhealthy: idle timeout",
					zap.String("id", instance.ID),
					zap.Duration("idle_time", now.Sub(instance.LastUsedAt)))
			}
		}

		instance.mu.Unlock()

		if unhealthy {
			toRetire = append(toRetire, instance)
		}
	}

	p.mu.Unlock()

	// 锁外回收，解除对 available channel 的持锁发送
	for _, instance := range toRetire {
		p.retireInstance(instance)
	}
}

// ScaleUp 扩展实例池
func (p *Pool) ScaleUp(count int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return fmt.Errorf("pool is closed")
	}

	added := 0
	for i := 0; i < count && p.instanceCount < p.config.MaxInstances; i++ {
		instance := p.createInstance()
		if instance != nil {
			p.instances = append(p.instances, instance)
			p.available <- instance
			p.instanceCount++
			added++
		}
	}

	p.logger.Info("scaled up pool",
		zap.Int("added", added),
		zap.Int("total", p.instanceCount))

	return nil
}

// ScaleDown 缩小实例池
func (p *Pool) ScaleDown(count int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return fmt.Errorf("pool is closed")
	}

	removed := 0
	for i := 0; i < count && len(p.instances) > p.config.MinInstances; i++ {
		// 尝试移除空闲实例
		select {
		case instance := <-p.available:
			for j, inst := range p.instances {
				if inst.ID == instance.ID {
					p.instances = append(p.instances[:j], p.instances[j+1:]...)
					p.instanceCount--
					go p.closeInstance(instance)
					removed++
					break
				}
			}
		default:
			// 没有空闲实例可移除
			break
		}
	}

	p.logger.Info("scaled down pool",
		zap.Int("removed", removed),
		zap.Int("total", p.instanceCount))

	return nil
}

// Stats 获取池统计
func (p *Pool) Stats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := map[string]interface{}{
		"total_instances": p.instanceCount,
		"min_instances":   p.config.MinInstances,
		"max_instances":   p.config.MaxInstances,
		"available":       len(p.available),
		"closed":          p.closed,
	}

	// 按使用次数统计
	useCountRanges := map[string]int{
		"0-25":   0,
		"26-50":  0,
		"51-75":  0,
		"76-100": 0,
		"100+":   0,
	}

	for _, instance := range p.instances {
		instance.mu.RLock()
		uc := instance.UseCount
		health := instance.IsHealthy
		instance.mu.RUnlock()

		switch {
		case uc <= 25:
			useCountRanges["0-25"]++
		case uc <= 50:
			useCountRanges["26-50"]++
		case uc <= 75:
			useCountRanges["51-75"]++
		case uc <= 100:
			useCountRanges["76-100"]++
		default:
			useCountRanges["100+"]++
		}

		if !health {
			useCountRanges["unhealthy"]++
		}
	}

	stats["use_count_distribution"] = useCountRanges

	return stats
}

// Close 关闭实例池
func (p *Pool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	// 取消上下文
	p.cancel()

	// 同步关闭所有实例：cancel 链路负责终止浏览器进程树，
	// 必须在本函数返回前完成，否则进程退出后子浏览器变孤儿
	p.mu.Lock()
	for _, instance := range p.instances {
		p.closeInstance(instance)
	}
	p.instances = nil
	p.mu.Unlock()

	// 等待协程退出
	p.wg.Wait()

	p.logger.Info("pool closed")

	return nil
}
