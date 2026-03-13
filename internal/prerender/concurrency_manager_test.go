package prerender

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewConcurrencyManager(t *testing.T) {
	manager := NewConcurrencyManager(1, 10, 5)
	assert.NotNil(t, manager)
	assert.Equal(t, 1, manager.minLimit)
	assert.Equal(t, 10, manager.maxLimit)
	assert.Equal(t, 5, manager.currentLimit)
	assert.Equal(t, 0, manager.activeCount)
}

func TestNewConcurrencyManager_InvalidValues(t *testing.T) {
	// minLimit < 1
	manager := NewConcurrencyManager(0, 10, 5)
	assert.Equal(t, 1, manager.minLimit)

	// maxLimit < minLimit
	manager = NewConcurrencyManager(5, 3, 5)
	assert.Equal(t, 25, manager.maxLimit) // minLimit * 5

	// initialLimit < minLimit
	manager = NewConcurrencyManager(5, 10, 2)
	assert.Equal(t, 5, manager.currentLimit)

	// initialLimit > maxLimit
	manager = NewConcurrencyManager(5, 10, 20)
	assert.Equal(t, 5, manager.currentLimit)
}

func TestConcurrencyManager_Acquire(t *testing.T) {
	manager := NewConcurrencyManager(1, 3, 2)

	// 第一次获取应该成功
	result := manager.Acquire()
	assert.True(t, result)
	assert.Equal(t, 1, manager.GetActiveCount())

	// 第二次获取应该成功（未达到限制）
	result = manager.Acquire()
	assert.True(t, result)
	assert.Equal(t, 2, manager.GetActiveCount())

	// 第三次获取应该失败（超过限制）
	result = manager.Acquire()
	assert.False(t, result)
	assert.Equal(t, 1, manager.waitingCount)
}

func TestConcurrencyManager_Release(t *testing.T) {
	manager := NewConcurrencyManager(1, 3, 2)

	manager.Acquire()
	manager.Acquire()
	assert.Equal(t, 2, manager.GetActiveCount())

	manager.Release()
	assert.Equal(t, 1, manager.GetActiveCount())

	manager.Release()
	assert.Equal(t, 0, manager.GetActiveCount())
}

func TestConcurrencyManager_RecordSuccess(t *testing.T) {
	manager := NewConcurrencyManager(1, 5, 3)
	manager.SetAdjustInterval(1 * time.Millisecond)

	// 记录成功
	manager.RecordSuccess(1.5)
	assert.Equal(t, int64(1), manager.successCount)
	assert.Equal(t, 1.5, manager.avgRenderTime)

	// 等待调整间隔
	time.Sleep(10 * time.Millisecond)

	// 再次记录成功，应该触发调整
	manager.RecordSuccess(2.0)
	assert.Equal(t, int64(2), manager.successCount)
}

func TestConcurrencyManager_RecordFailure(t *testing.T) {
	manager := NewConcurrencyManager(1, 5, 3)

	initialLimit := manager.GetCurrentLimit()
	manager.RecordFailure()

	assert.Equal(t, initialLimit-1, manager.GetCurrentLimit())
	assert.Equal(t, int64(1), manager.failureCount)
}

func TestConcurrencyManager_RecordFailure_MinLimit(t *testing.T) {
	manager := NewConcurrencyManager(3, 5, 3)

	// 连续失败直到达到最小限制
	for i := 0; i < 10; i++ {
		manager.RecordFailure()
	}

	assert.Equal(t, 3, manager.GetCurrentLimit()) // 不会低于 minLimit
}

func TestConcurrencyManager_AddRenderTime(t *testing.T) {
	manager := NewConcurrencyManager(1, 5, 3)

	// 添加渲染时间
	for i := 0; i < 50; i++ {
		manager.addRenderTime(float64(i + 1))
	}

	// 检查平均渲染时间
	assert.Greater(t, manager.avgRenderTime, 0.0)

	// 添加超过 100 条记录
	for i := 0; i < 60; i++ {
		manager.addRenderTime(100.0)
	}

	// 应该只保留最近的 100 条
	assert.LessOrEqual(t, len(manager.renderTimes), 100)
}

func TestConcurrencyManager_AdjustLimitLocked_HighSuccess(t *testing.T) {
	manager := NewConcurrencyManager(1, 5, 3)
	manager.SetAdjustInterval(1 * time.Nanosecond)

	// 模拟高成功率
	for i := 0; i < 100; i++ {
		manager.successCount++
	}

	manager.adjustLimitLocked()

	// 由于成功率高，应该增加限制
	assert.Greater(t, manager.currentLimit, 3)
}

func TestConcurrencyManager_AdjustLimitLocked_LowSuccess(t *testing.T) {
	manager := NewConcurrencyManager(1, 5, 3)

	// 模拟低成功率
	manager.successCount = 50
	manager.failureCount = 50

	manager.adjustLimitLocked()

	// 由于成功率低于 80%，应该降低限制
	assert.Less(t, manager.currentLimit, 3)
}

func TestConcurrencyManager_AdjustLimitLocked_HighLatency(t *testing.T) {
	manager := NewConcurrencyManager(1, 5, 3)

	// 模拟高延迟
	manager.avgRenderTime = 20.0

	manager.adjustLimitLocked()

	// 由于延迟高于 15s，应该降低限制
	assert.LessOrEqual(t, manager.currentLimit, 3)
}

func TestConcurrencyManager_GetCurrentLimit(t *testing.T) {
	manager := NewConcurrencyManager(1, 10, 5)
	assert.Equal(t, 5, manager.GetCurrentLimit())
}

func TestConcurrencyManager_GetActiveCount(t *testing.T) {
	manager := NewConcurrencyManager(1, 10, 5)

	manager.Acquire()
	manager.Acquire()

	assert.Equal(t, 2, manager.GetActiveCount())
}

func TestConcurrencyManager_GetWaitingCount(t *testing.T) {
	manager := NewConcurrencyManager(1, 3, 2)

	manager.Acquire()
	manager.Acquire()
	manager.Acquire() // 这个会进入等待

	assert.Equal(t, 1, manager.GetWaitingCount())
}

func TestConcurrencyManager_GetStats(t *testing.T) {
	manager := NewConcurrencyManager(1, 10, 5)

	manager.Acquire()
	manager.RecordSuccess(2.5)
	// 不记录失败，避免触发并发限制降低

	stats := manager.GetStats()

	assert.NotNil(t, stats)
	assert.Equal(t, 5, stats["current_limit"])
	assert.Equal(t, 1, stats["min_limit"])
	assert.Equal(t, 10, stats["max_limit"])
	assert.Equal(t, 1, stats["active_count"])
	assert.Equal(t, int64(1), stats["success_count"])
	assert.Greater(t, stats["avg_render_time"], 0.0)
	assert.Greater(t, stats["success_rate"], 0.0)
}

func TestConcurrencyManager_SetAdjustInterval(t *testing.T) {
	manager := NewConcurrencyManager(1, 10, 5)

	manager.SetAdjustInterval(60 * time.Second)
	assert.Equal(t, 60*time.Second, manager.adjustInterval)
}

func TestConcurrencyManager_Reset(t *testing.T) {
	manager := NewConcurrencyManager(1, 10, 5)

	manager.RecordSuccess(2.5)
	manager.RecordFailure()
	manager.addRenderTime(3.0)

	manager.Reset()

	assert.Equal(t, int64(0), manager.successCount)
	assert.Equal(t, int64(0), manager.failureCount)
	assert.Equal(t, 0.0, manager.avgRenderTime)
	assert.Empty(t, manager.renderTimes)
}

func TestConcurrencyManager_Concurrent(t *testing.T) {
	manager := NewConcurrencyManager(1, 5, 3)

	done := make(chan bool, 10)

	// 启动多个协程
	for i := 0; i < 10; i++ {
		go func() {
			manager.Acquire()
			time.Sleep(10 * time.Millisecond)
			manager.Release()
			done <- true
		}()
	}

	// 等待所有协程完成
	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, 0, manager.GetActiveCount())
}
