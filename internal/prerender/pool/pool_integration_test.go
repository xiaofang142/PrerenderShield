package pool

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestPool_InstanceReuse(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1,
		MaxInstances:        2,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
		MaxUseCount:         100,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	ctx := context.Background()

	instance1, err := pool.Acquire(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, instance1)

	firstID := instance1.ID

	err = pool.Release(instance1)
	assert.NoError(t, err)

	instance2, err := pool.Acquire(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, instance2)

	assert.Equal(t, firstID, instance2.ID)

	pool.Release(instance2)
}

func TestPool_MaxInstancesLimit(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1,
		MaxInstances:        2,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
		MaxUseCount:         100, // Set reasonable max use count
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	ctx := context.Background()

	instances := make([]*Instance, 3)
	var err error

	instances[0], err = pool.Acquire(ctx)
	assert.NoError(t, err)

	instances[1], err = pool.Acquire(ctx)
	assert.NoError(t, err)

	acquireCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	instances[2], err = pool.Acquire(acquireCtx)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Nil(t, instances[2])
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)

	pool.Release(instances[0])
	pool.Release(instances[1])
}

func TestPool_IdleTimeoutRecycle(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        2,
		MaxInstances:        5,
		IdleTimeout:         200 * time.Millisecond,
		Headless:            true,
		HealthCheckInterval: 100 * time.Millisecond,
		MaxUseCount:         100,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	ctx := context.Background()

	instance, err := pool.Acquire(ctx)
	assert.NoError(t, err)

	firstID := instance.ID

	err = pool.Release(instance)
	assert.NoError(t, err)

	time.Sleep(400 * time.Millisecond)

	newInstance, err := pool.Acquire(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, newInstance)

	assert.NotEqual(t, firstID, newInstance.ID)

	pool.Release(newInstance)
}

func TestPool_ConcurrentAcquire(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        3,
		MaxInstances:        5,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
		MaxUseCount:         100, // Set reasonable max use count
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ctx := context.Background()
			instance, err := pool.Acquire(ctx)
			if err != nil {
				errors <- err
				return
			}

			time.Sleep(10 * time.Millisecond)

			err = pool.Release(instance)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		assert.Fail(t, "concurrent acquire failed", err)
	}
}

func TestPool_InstanceUseCount(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1,
		MaxInstances:        2,
		MaxUseCount:         5,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	ctx := context.Background()

	instance, err := pool.Acquire(ctx)
	assert.NoError(t, err)

	assert.Equal(t, 1, instance.UseCount)

	err = pool.Release(instance)
	assert.NoError(t, err)

	instance2, err := pool.Acquire(ctx)
	assert.NoError(t, err)

	assert.Equal(t, 2, instance2.UseCount)

	pool.Release(instance2)
}

func TestPool_RenderWithPool(t *testing.T) {
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

	assert.NotNil(t, instance.ChromeCtx)
	assert.NotNil(t, instance.AllocCtx)

	var title string
	ctxRender, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = chromedp.Run(ctxRender,
		chromedp.Navigate("data:text/html,<html><head><title>Test</title></head><body>Hello</body></html>"),
		chromedp.Title(&title),
	)

	if err == nil {
		assert.Equal(t, "Test", title)
	}

	err = pool.Release(instance)
	assert.NoError(t, err)
}

func TestPool_ScaleOperations(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1,
		MaxInstances:        5,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	initialStats := pool.Stats()
	assert.NotNil(t, initialStats)

	err := pool.ScaleUp(2)
	assert.NoError(t, err)

	afterScaleUp := pool.Stats()
	assert.GreaterOrEqual(t, afterScaleUp["total_instances"], initialStats["total_instances"])

	err = pool.ScaleDown(1)
	assert.NoError(t, err)

	afterScaleDown := pool.Stats()
	assert.LessOrEqual(t, afterScaleDown["total_instances"], afterScaleUp["total_instances"])
}
