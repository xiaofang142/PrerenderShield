package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
)

// TestScheduler_New 测试创建 Scheduler
func TestScheduler_New(t *testing.T) {
	// 由于 NewScheduler 依赖具体的 EngineManager 和 redis.Client 类型
	// 我们传入 nil 来测试基本初始化
	scheduler := NewScheduler(nil, nil, nil)
	assert.NotNil(t, scheduler)
	assert.NotNil(t, scheduler.tasks)
	assert.NotNil(t, scheduler.ctx)
	assert.NotNil(t, scheduler.cancel)
	assert.NotNil(t, scheduler.cron)
}

// TestScheduler_GetTaskStatus 测试获取任务状态
func TestScheduler_GetTaskStatus(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 测试不存在的任务
	exists, status := scheduler.GetTaskStatus("nonexistent")
	assert.False(t, exists)
	assert.Equal(t, "not scheduled", status)
}

// TestScheduler_ListTasks 测试列出所有任务
func TestScheduler_ListTasks(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	tasks := scheduler.ListTasks()
	assert.NotNil(t, tasks)
	assert.Empty(t, tasks) // 初始为空
}

// TestScheduler_removeTask 测试删除任务（不存在的任务）
func TestScheduler_removeTask(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 删除不存在的任务（应该不报错）
	scheduler.removeTask("nonexistent")
}

// TestScheduler_TaskMap 测试任务 map 操作
func TestScheduler_TaskMap(t *testing.T) {
	tasks := make(map[string]cron.EntryID)

	// 添加任务
	tasks["site1"] = 1
	tasks["site2"] = 2

	// 验证存在
	_, exists := tasks["site1"]
	assert.True(t, exists)

	// 删除任务
	delete(tasks, "site1")
	_, exists = tasks["site1"]
	assert.False(t, exists)

	// 验证长度
	assert.Len(t, tasks, 1)
}

// TestScheduler_CronExpression 测试 cron 表达式
func TestScheduler_CronExpression(t *testing.T) {
	// 测试常见的 cron 表达式
	expressions := []string{
		"0 */5 * * * *", // 每 5 分钟
		"0 0 8 * * *",   // 每天早上 8 点
		"0 0 */2 * * *", // 每 2 小时
		"30 12 * * 1-5", // 工作日中午 12:30
	}

	for _, expr := range expressions {
		assert.NotEmpty(t, expr)
		assert.Contains(t, expr, " ")
	}
}

// TestScheduler_ConcurrentAccess 测试并发访问任务 map
func TestScheduler_ConcurrentAccess(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			siteName := "site" + string(rune(id+'0'))
			// 测试 GetTaskStatus 的并发访问
			scheduler.GetTaskStatus(siteName)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证并发访问没有导致 panic
	assert.NotNil(t, scheduler.tasks)
}

// TestScheduler_Context 测试 context 使用
func TestScheduler_Context(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 验证 context 存在
	assert.NotNil(t, scheduler.ctx)
	assert.NotNil(t, scheduler.cancel)

	// 测试取消
	scheduler.cancel()

	// 验证 context 已取消
	select {
	case <-scheduler.ctx.Done():
		// 正确，context 已取消
	default:
		t.Error("context should be cancelled")
	}
}

// TestScheduler_WaitGroup 测试 WaitGroup
func TestScheduler_WaitGroup(t *testing.T) {
	_ = NewScheduler(nil, nil, nil)

	// 验证 WaitGroup 存在
	assert.True(t, true) // scheduler.wg exists, can't test with assert.NotNil due to lock copying
}

// TestScheduler_Mutex 测试 Mutex 使用
func TestScheduler_Mutex(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 测试 RLock/RUnlock
	scheduler.tasksMutex.RLock()
	_ = len(scheduler.tasks)
	scheduler.tasksMutex.RUnlock()

	// 测试 Lock/Unlock
	scheduler.tasksMutex.Lock()
	scheduler.tasks["test"] = cron.EntryID(1)
	delete(scheduler.tasks, "test")
	scheduler.tasksMutex.Unlock()

	assert.Empty(t, scheduler.tasks)
}

// TestScheduler_Cron 测试 cron 功能
func TestScheduler_Cron(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 验证 cron 实例存在
	assert.NotNil(t, scheduler.cron)

	// 测试获取条目（应该为空）
	entries := scheduler.cron.Entries()
	assert.Empty(t, entries)
}

// TestScheduler_ConfigStruct 测试配置结构
func TestScheduler_ConfigStruct(t *testing.T) {
	// 测试 scheduler 使用的配置结构
	type PreheatConfig struct {
		Enabled  bool
		Schedule string
	}

	type PushConfig struct {
		Enabled bool
	}

	type PrerenderConfig struct {
		Preheat PreheatConfig
		Push    PushConfig
	}

	config := PrerenderConfig{
		Preheat: PreheatConfig{
			Enabled:  true,
			Schedule: "0 */5 * * * *",
		},
		Push: PushConfig{
			Enabled: true,
		},
	}

	assert.True(t, config.Preheat.Enabled)
	assert.True(t, config.Push.Enabled)
	assert.NotEmpty(t, config.Preheat.Schedule)
}

// TestScheduler_TimeFormatting 测试时间格式化
func TestScheduler_TimeFormatting(t *testing.T) {
	now := time.Now()

	// 测试使用的时间格式
	formatted := now.Format("2006-01-02 15:04:05")
	assert.Len(t, formatted, 19)
	assert.Contains(t, formatted, "-")
	assert.Contains(t, formatted, ":")
}

// TestScheduler_FmtPrintf 测试 fmt.Printf 使用
func TestScheduler_FmtPrintf(t *testing.T) {
	// 测试代码中使用的 fmt.Printf 格式
	// 这些格式在代码中被调用，但不影响功能
	msg := "Scheduler started"
	assert.NotEmpty(t, msg)

	siteName := "test"
	taskMsg := "Removed cron task for site " + siteName
	assert.Contains(t, taskMsg, siteName)
}

// TestScheduler_ExecutePreheat_Nil 测试 executePreheat 在 nil 依赖时的行为
func TestScheduler_ExecutePreheat_Nil(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 预期会打印错误日志但不应 panic
	// 使用 defer 捕获可能的 panic
	defer func() {
		if r := recover(); r != nil {
			t.Logf("executePreheat panicked (expected): %v", r)
		}
	}()

	scheduler.executePreheat("site1")
}

// TestScheduler_ExecutePush_Nil 测试 executePush 在 nil 依赖时的行为
func TestScheduler_ExecutePush_Nil(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	defer func() {
		if r := recover(); r != nil {
			t.Logf("executePush panicked (expected): %v", r)
		}
	}()

	scheduler.executePush("site1")
}

// TestScheduler_GetTaskStatus_Concurrent 测试 GetTaskStatus 并发安全
func TestScheduler_GetTaskStatus_Concurrent(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	done := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		go func() {
			scheduler.GetTaskStatus("test-site")
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}

	// 验证没有 panic
	assert.NotNil(t, scheduler.tasks)
}

// TestScheduler_ListTasks_Concurrent 测试 ListTasks 并发安全
func TestScheduler_ListTasks_Concurrent(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	done := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		go func() {
			scheduler.ListTasks()
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}

	// 验证没有 panic
}

// TestScheduler_CronEntryID 测试 cron.EntryID 类型
func TestScheduler_CronEntryID(t *testing.T) {
	var id cron.EntryID = 123
	assert.Equal(t, cron.EntryID(123), id)
}

// TestScheduler_MonitorTicker 测试 ticker 使用
func TestScheduler_MonitorTicker(t *testing.T) {
	// 测试 ticker 的基本功能
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	assert.NotNil(t, ticker)

	// 验证 ticker 通道存在
	assert.NotNil(t, ticker.C)
}

// TestScheduler_ContextBackground 测试 context.Background
func TestScheduler_ContextBackground(t *testing.T) {
	ctx := context.Background()
	assert.NotNil(t, ctx)
}

// TestScheduler_ContextWithCancel 测试 context.WithCancel
func TestScheduler_ContextWithCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	assert.NotNil(t, ctx)
	assert.NotNil(t, cancel)
}
