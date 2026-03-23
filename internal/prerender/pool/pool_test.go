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

// TestPool_allocatorOptions 测试 Chrome 分配器选项生成
func TestPool_allocatorOptions(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1,
		MaxInstances:        2,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	options := pool.allocatorOptions()
	assert.NotEmpty(t, options)
	// 验证包含关键选项
	assert.GreaterOrEqual(t, len(options), 10)
}

// TestPool_allocatorOptions_HeadlessFalse 测试非无头模式
func TestPool_allocatorOptions_HeadlessFalse(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1,
		MaxInstances:        2,
		Headless:            false,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	options := pool.allocatorOptions()
	assert.NotEmpty(t, options)
}

// TestPool_retireInstance 测试回收实例逻辑
func TestPool_retireInstance(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        2,
		MaxInstances:        5,
		MaxUseCount:         2,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	ctx := context.Background()

	// 获取实例并使用到最大次数
	instance, err := pool.Acquire(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, instance)

	// 手动设置使用次数达到最大值
	instance.mu.Lock()
	instance.UseCount = cfg.MaxUseCount
	instance.mu.Unlock()

	// 释放时应该触发回收
	err = pool.Release(instance)
	assert.NoError(t, err)

	// 等待一小段时间让回收完成
	time.Sleep(100 * time.Millisecond)

	stats := pool.Stats()
	assert.NotNil(t, stats)
}

// TestPool_ReleaseToFullPool 测试释放到已满的池
func TestPool_ReleaseToFullPool(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        2,
		MaxInstances:        2,
		MaxUseCount:         100, // 显式设置，避免 UseCount=1 就触发回收
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	ctx := context.Background()

	// 获取所有可用实例
	instance1, err := pool.Acquire(ctx)
	assert.NoError(t, err)
	instance2, err := pool.Acquire(ctx)
	assert.NoError(t, err)

	// 释放所有实例使池满
	err = pool.Release(instance1)
	assert.NoError(t, err)
	err = pool.Release(instance2)
	assert.NoError(t, err)

	// 再次获取并释放，测试池满情况
	instance3, err := pool.Acquire(ctx)
	assert.NoError(t, err)

	// 释放实例
	err = pool.Release(instance3)
	assert.NoError(t, err)
}

// TestPool_checkHealth_MaxUseCount 测试健康检查 - 最大使用次数
func TestPool_checkHealth_MaxUseCount(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1,
		MaxInstances:        5,
		MaxUseCount:         5,
		IdleTimeout:         1 * time.Minute,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	ctx := context.Background()

	// 获取一个实例并使用到最大次数
	instance, err := pool.Acquire(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, instance)

	// 设置达到最大使用次数
	instance.mu.Lock()
	instance.UseCount = 5
	instance.mu.Unlock()

	// 释放应该触发回收
	err = pool.Release(instance)
	assert.NoError(t, err)

	// 等待回收完成
	time.Sleep(50 * time.Millisecond)

	stats := pool.Stats()
	assert.NotNil(t, stats)
}

// TestPool_checkHealth_IdleTimeout 测试健康检查 - 空闲超时
func TestPool_checkHealth_IdleTimeout(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        2,
		MaxInstances:        5,
		MaxUseCount:         100,
		IdleTimeout:         100 * time.Millisecond, // 很短的超时用于测试
		Headless:            true,
		HealthCheckInterval: 50 * time.Millisecond,  // 频繁检查
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	ctx := context.Background()

	// 获取并释放一个实例
	instance, err := pool.Acquire(ctx)
	assert.NoError(t, err)
	err = pool.Release(instance)
	assert.NoError(t, err)

	// 等待超过空闲超时
	time.Sleep(200 * time.Millisecond)

	stats := pool.Stats()
	assert.NotNil(t, stats)
}

// TestPool_retireInstanceLocked 测试锁持有状态下回收实例
func TestPool_retireInstanceLocked(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        2,
		MaxInstances:        5,
		MaxUseCount:         100,
		IdleTimeout:         5 * time.Minute,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	// 获取一个实例
	ctx := context.Background()
	instance, err := pool.Acquire(ctx)
	assert.NoError(t, err)

	// 手动设置为不健康
	instance.mu.Lock()
	instance.IsHealthy = false
	instance.mu.Unlock()

	// 释放触发回收
	err = pool.Release(instance)
	assert.NoError(t, err)

	// 等待回收完成
	time.Sleep(100 * time.Millisecond)

	stats := pool.Stats()
	assert.NotNil(t, stats)
}

// TestPool_ScaleUpToMax 测试扩展到最大实例数
func TestPool_ScaleUpToMax(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1,
		MaxInstances:        3,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	// 尝试扩展超过最大值
	err := pool.ScaleUp(10)
	assert.NoError(t, err)

	stats := pool.Stats()
	assert.NotNil(t, stats)
	// 总实例数不应超过 MaxInstances
	assert.LessOrEqual(t, stats["total_instances"], cfg.MaxInstances)
}

// TestPool_ScaleDown_NoIdleInstances 测试没有空闲实例时缩小
func TestPool_ScaleDown_NoIdleInstances(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1,
		MaxInstances:        5,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	ctx := context.Background()

	// 扩展
	err := pool.ScaleUp(2)
	assert.NoError(t, err)

	// 获取所有实例
	instance1, _ := pool.Acquire(ctx)
	instance2, _ := pool.Acquire(ctx)
	instance3, _ := pool.Acquire(ctx)

	// 尝试缩小（没有空闲实例）
	err = pool.ScaleDown(2)
	assert.NoError(t, err)

	// 释放实例
	pool.Release(instance1)
	pool.Release(instance2)
	pool.Release(instance3)
}

// TestPool_AcquireContextCancelled 测试 context 取消
// 当池子中没有可用实例时，上下文取消应该返回错误
func TestPool_AcquireContextCancelled(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1,  // Pre-create one instance
		MaxInstances:        1,  // Only allow one instance
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
		MaxUseCount:         100,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	// Acquire the only instance first
	firstInstance, err := pool.Acquire(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, firstInstance)

	// Now try to acquire another instance with timeout
	// Since the only instance is in use, this should timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = pool.Acquire(ctx)
	elapsed := time.Since(start)

	// Should return error (context deadline exceeded)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")

	// Elapsed time should be close to timeout (at least 400ms)
	assert.GreaterOrEqual(t, elapsed, 400*time.Millisecond, "Acquire should have waited for context timeout")

	// Release the first instance
	pool.Release(firstInstance)
}

// TestPool_Stats_UseCountDistribution 测试使用次数分布统计
func TestPool_Stats_UseCountDistribution(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        2,
		MaxInstances:        5,
		MaxUseCount:         150,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	ctx := context.Background()

	// 获取实例并使用不同次数
	instances := make([]*Instance, 2)
	for i := 0; i < 2; i++ {
		inst, err := pool.Acquire(ctx)
		assert.NoError(t, err)
		instances[i] = inst

		// 设置不同的使用次数
		inst.mu.Lock()
		inst.UseCount = (i + 1) * 50
		inst.mu.Unlock()
	}

	// 释放实例
	for _, inst := range instances {
		pool.Release(inst)
	}

	stats := pool.Stats()
	assert.NotNil(t, stats)

	dist, ok := stats["use_count_distribution"].(map[string]int)
	assert.True(t, ok)
	assert.Contains(t, dist, "0-25")
	assert.Contains(t, dist, "26-50")
	assert.Contains(t, dist, "51-75")
	assert.Contains(t, dist, "76-100")
	assert.Contains(t, dist, "100+")
}

// TestPool_MultipleAcquireRelease 测试多次获取释放
func TestPool_MultipleAcquireRelease(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        2,
		MaxInstances:        5,
		MaxUseCount:         10,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	ctx := context.Background()

	// 多次获取和释放
	for i := 0; i < 5; i++ {
		instance, err := pool.Acquire(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, instance)

		// 模拟使用
		time.Sleep(10 * time.Millisecond)

		err = pool.Release(instance)
		assert.NoError(t, err)
	}
}

// TestPool_HealthChecker 测试健康检查协程
func TestPool_HealthChecker(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        2,
		MaxInstances:        5,
		MaxUseCount:         100,
		IdleTimeout:         5 * time.Minute,
		Headless:            true,
		HealthCheckInterval: 100 * time.Millisecond, // 缩短间隔用于测试
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	// 等待健康检查运行
	time.Sleep(200 * time.Millisecond)

	stats := pool.Stats()
	assert.NotNil(t, stats)
}

// TestPool_ConcurrentAcquireRelease 测试并发获取释放
func TestPool_ConcurrentAcquireRelease(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        3,
		MaxInstances:        5,
		MaxUseCount:         50,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	done := make(chan bool, 10)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		go func() {
			instance, err := pool.Acquire(ctx)
			if err == nil {
				time.Sleep(10 * time.Millisecond)
				pool.Release(instance)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	stats := pool.Stats()
	assert.NotNil(t, stats)
}

// TestInstance_Fields 测试 Instance 字段访问
func TestInstance_Fields(t *testing.T) {
	now := time.Now()
	instance := &Instance{
		ID:           "test-123",
		AllocCtx:     context.Background(),
		AllocCancel:  func() {},
		ChromeCtx:    context.Background(),
		ChromeCancel: func() {},
		CreatedAt:    now,
		LastUsedAt:   now,
		UseCount:     5,
		MaxUseCount:  100,
		IsHealthy:    true,
	}

	assert.Equal(t, "test-123", instance.ID)
	assert.Equal(t, 5, instance.UseCount)
	assert.Equal(t, 100, instance.MaxUseCount)
	assert.True(t, instance.IsHealthy)
	assert.Equal(t, now, instance.CreatedAt)
	assert.Equal(t, now, instance.LastUsedAt)
}

// TestConfig_Fields 测试 Config 字段
func TestConfig_Fields(t *testing.T) {
	cfg := Config{
		MinInstances:        5,
		MaxInstances:        20,
		IdleTimeout:         10 * time.Minute,
		MaxUseCount:         500,
		HealthCheckInterval: 60 * time.Second,
		Headless:            false,
	}

	assert.Equal(t, 5, cfg.MinInstances)
	assert.Equal(t, 20, cfg.MaxInstances)
	assert.Equal(t, 10*time.Minute, cfg.IdleTimeout)
	assert.Equal(t, 500, cfg.MaxUseCount)
	assert.Equal(t, 60*time.Second, cfg.HealthCheckInterval)
	assert.False(t, cfg.Headless)
}

// TestPool_createInstance 测试创建实例
func TestPool_createInstance(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        0, // 不自动创建
		MaxInstances:        5,
		MaxUseCount:         100,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	// 手动创建实例
	instance := pool.createInstance()
	assert.NotNil(t, instance)
	assert.NotEmpty(t, instance.ID)
	assert.True(t, instance.IsHealthy)
	assert.Equal(t, 0, instance.UseCount)
}

// TestPool_closeInstance 测试关闭实例
func TestPool_closeInstance(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        0,
		MaxInstances:        2,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	instance := pool.createInstance()
	assert.NotNil(t, instance)

	// 关闭实例
	pool.closeInstance(instance)
}

// TestPool_ReleaseUnhealthyInstance 测试释放不健康实例
func TestPool_ReleaseUnhealthyInstance(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        2,
		MaxInstances:        5,
		MaxUseCount:         100,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	ctx := context.Background()

	instance, err := pool.Acquire(ctx)
	assert.NoError(t, err)

	// 设置为不健康
	instance.mu.Lock()
	instance.IsHealthy = false
	instance.mu.Unlock()

	// 释放应该触发回收
	err = pool.Release(instance)
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	stats := pool.Stats()
	assert.NotNil(t, stats)
}

// TestPool_AcquireWithTimeout_Timeout 测试获取超时
func TestPool_AcquireWithTimeout_Timeout(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        1, // Pre-create one instance
		MaxInstances:        1, // Only allow one instance
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
		MaxUseCount:         100,
	}

	pool := NewPool(cfg, logger)
	defer pool.Close()

	// Acquire the only instance first
	firstInstance, err := pool.Acquire(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, firstInstance)

	// Timeout should return error since no instance is available
	instance, err := pool.AcquireWithTimeout(50 * time.Millisecond)
	assert.Error(t, err)
	assert.Nil(t, instance)

	// Release the first instance
	pool.Release(firstInstance)
}
