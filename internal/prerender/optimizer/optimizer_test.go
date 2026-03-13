package optimizer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.True(t, cfg.EnableLazyLoad)
	assert.True(t, cfg.EnableResourceBlock)
	assert.True(t, cfg.EnableMemoryMonitor)
	assert.Len(t, cfg.BlockedResources, 5)
	assert.Contains(t, cfg.BlockedResources, "stylesheet")
	assert.Contains(t, cfg.BlockedResources, "image")
	assert.Contains(t, cfg.BlockedResources, "media")
	assert.Contains(t, cfg.BlockedResources, "font")
	assert.Contains(t, cfg.BlockedResources, "websocket")
	assert.True(t, cfg.LazyLoadImages)
	assert.True(t, cfg.LazyLoadIFrames)
	assert.Equal(t, 512, cfg.MemoryLimitMB)
	assert.Equal(t, 10*time.Second, cfg.MemoryCheckInterval)
}

func TestNewOptimizer_NilLogger(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableMemoryMonitor = false // 禁用内存监控以避免后台协程

	opt := NewOptimizer(cfg, nil)
	assert.NotNil(t, opt)
}

func TestNewOptimizer_CustomConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		EnableLazyLoad:      false,
		EnableResourceBlock: false,
		EnableMemoryMonitor: false,
		MemoryLimitMB:       256,
	}

	opt := NewOptimizer(cfg, logger)
	assert.NotNil(t, opt)
	assert.Equal(t, cfg, opt.config)
}

func TestNewOptimizer_WithMemoryMonitor(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		EnableMemoryMonitor: true,
		MemoryLimitMB:       256,
		MemoryCheckInterval: 100 * time.Millisecond,
	}

	opt := NewOptimizer(cfg, logger)
	assert.NotNil(t, opt)
	assert.NotNil(t, opt.memoryStats)

	// 等待内存监控启动
	time.Sleep(50 * time.Millisecond)

	// 关闭
	close(opt.stopChan)
	opt.wg.Wait()
}

func TestOptimizer_ApplyOptions_NilConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		EnableLazyLoad:      false,
		EnableResourceBlock: false,
		EnableMemoryMonitor: false,
	}

	opt := NewOptimizer(cfg, logger)
	ctx := context.Background()
	resultCtx := opt.ApplyOptions(ctx)
	assert.NotNil(t, resultCtx)
}

func TestOptimizer_ApplyOptions_WithResourceBlock(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		EnableLazyLoad:      false,
		EnableResourceBlock: true,
		EnableMemoryMonitor: false,
		BlockedResources:    []string{"image", "font"},
	}

	opt := NewOptimizer(cfg, logger)
	ctx := context.Background()
	resultCtx := opt.ApplyOptions(ctx)
	assert.NotNil(t, resultCtx)
}

func TestOptimizer_ApplyOptions_WithLazyLoad(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		EnableLazyLoad:      true,
		EnableResourceBlock: false,
		EnableMemoryMonitor: false,
		LazyLoadImages:      true,
		LazyLoadIFrames:     true,
	}

	opt := NewOptimizer(cfg, logger)
	ctx := context.Background()
	resultCtx := opt.ApplyOptions(ctx)
	assert.NotNil(t, resultCtx)
}

func TestMemoryStats_Struct(t *testing.T) {
	stats := &MemoryStats{
		CurrentMB:    100.5,
		PeakMB:       200.0,
		LimitMB:      512.0,
		UsagePercent: 39.06,
		LastCheck:    time.Now(),
	}

	assert.Equal(t, 100.5, stats.CurrentMB)
	assert.Equal(t, 200.0, stats.PeakMB)
	assert.Equal(t, 512.0, stats.LimitMB)
	assert.Equal(t, 39.06, stats.UsagePercent)
	assert.NotZero(t, stats.LastCheck)
}

func TestOptimizer_MemoryMonitorWorker(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		EnableMemoryMonitor: true,
		MemoryLimitMB:       256,
		MemoryCheckInterval: 50 * time.Millisecond,
	}

	opt := NewOptimizer(cfg, logger)
	assert.NotNil(t, opt)

	// 等待内存监控运行
	time.Sleep(100 * time.Millisecond)

	// 停止
	close(opt.stopChan)
	opt.wg.Wait()
}

func TestOptimizer_ConcurrentAccess(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		EnableMemoryMonitor: false,
	}

	opt := NewOptimizer(cfg, logger)
	ctx := context.Background()

	// 并发调用 ApplyOptions
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			_ = opt.ApplyOptions(ctx)
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}
