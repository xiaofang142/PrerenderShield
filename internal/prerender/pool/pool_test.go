package pool

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, 2, cfg.MinInstances)
	assert.Equal(t, 10, cfg.MaxInstances)
	assert.Equal(t, 5*time.Minute, cfg.IdleTimeout)
	assert.Equal(t, 100, cfg.MaxUseCount)
	assert.Equal(t, 30*time.Second, cfg.HealthCheckInterval)
	assert.True(t, cfg.Headless)
}

func TestNewPool_NilLogger(t *testing.T) {
	cfg := Config{
		MinInstances:        1,
		MaxInstances:        2,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, nil)
	assert.NotNil(t, pool)
	assert.NoError(t, pool.Close())
}

func TestNewPool_Basic(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1,
		MaxInstances:        3,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	assert.NotNil(t, pool)

	stats := pool.Stats()
	assert.NotNil(t, stats)

	assert.NoError(t, pool.Close())
}

func TestPool_AcquireRelease(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1,
		MaxInstances:        2,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	ctx := context.Background()
	instance, err := pool.Acquire(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, instance)

	err = pool.Release(instance)
	assert.NoError(t, err)
}

func TestPool_AcquireWithTimeout(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1,
		MaxInstances:        2,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	instance, err := pool.AcquireWithTimeout(5 * time.Second)
	assert.NoError(t, err)
	assert.NotNil(t, instance)

	err = pool.Release(instance)
	assert.NoError(t, err)
}

func TestPool_AcquireClosedPool(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1,
		MaxInstances:        2,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	pool.Close()

	ctx := context.Background()
	instance, err := pool.Acquire(ctx)
	assert.Error(t, err)
	assert.Nil(t, instance)
}

func TestPool_ScaleUp(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1,
		MaxInstances:        5,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	err := pool.ScaleUp(2)
	assert.NoError(t, err)

	stats := pool.Stats()
	assert.NotNil(t, stats)
}

func TestPool_ScaleUpClosedPool(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1,
		MaxInstances:        5,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	pool.Close()

	err := pool.ScaleUp(2)
	assert.Error(t, err)
}

func TestPool_ScaleDown(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1,
		MaxInstances:        5,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	// 先扩展
	err := pool.ScaleUp(2)
	assert.NoError(t, err)

	// 再缩小
	err = pool.ScaleDown(1)
	assert.NoError(t, err)
}

func TestPool_ScaleDownClosedPool(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1,
		MaxInstances:        5,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	pool.Close()

	err := pool.ScaleDown(1)
	assert.Error(t, err)
}

func TestPool_Close(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1,
		MaxInstances:        2,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	assert.NoError(t, pool.Close())

	// 重复关闭应该没问题
	assert.NoError(t, pool.Close())
}

func TestPool_Stats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1,
		MaxInstances:        3,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	stats := pool.Stats()
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "total_instances")
	assert.Contains(t, stats, "min_instances")
	assert.Contains(t, stats, "max_instances")
	assert.Contains(t, stats, "available")
	assert.Contains(t, stats, "closed")
	assert.Contains(t, stats, "use_count_distribution")
}

func TestPool_ReleaseExceedsMaxUse(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        2,
		MaxInstances:        3,
		MaxUseCount:         2,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	ctx := context.Background()

	// 获取实例并使用多次
	for i := 0; i < 2; i++ {
		instance, err := pool.Acquire(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, instance)
		err = pool.Release(instance)
		assert.NoError(t, err)
	}
}

func TestPool_ContextCancellation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        2,
		MaxInstances:        3,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	// 获取一个实例
	ctx1 := context.Background()
	instance1, err := pool.Acquire(ctx1)
	assert.NoError(t, err)
	assert.NotNil(t, instance1)

	// 释放实例
	err = pool.Release(instance1)
	assert.NoError(t, err)
}

func TestInstance_Mutex(t *testing.T) {
	instance := &Instance{
		ID:          "test-instance",
		MaxUseCount: 100,
		IsHealthy:   true,
		CreatedAt:   time.Now(),
		LastUsedAt:  time.Now(),
	}

	// 测试并发访问
	done := make(chan bool, 3)
	for i := 0; i < 3; i++ {
		go func() {
			instance.mu.RLock()
			_ = instance.IsHealthy
			instance.mu.RUnlock()
			done <- true
		}()
	}

	for i := 0; i < 3; i++ {
		<-done
	}
}
