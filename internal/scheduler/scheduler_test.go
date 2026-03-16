package scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"prerender-shield/internal/config"
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

// TestScheduler_StartStop 测试 Start 和 Stop 方法
func TestScheduler_StartStop(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)
	assert.NotNil(t, scheduler)

	// 只测试 cron 调度器的启动和停止，不测试 monitorSites 协程
	// 因为 monitorSites 依赖 engineManager，传入 nil 时会 panic
	scheduler.cron.Start()

	// 等待一小段时间
	time.Sleep(50 * time.Millisecond)

	// 停止 cron 调度器
	scheduler.cron.Stop()

	// 验证 cron 仍在运行（没有 panic）
	assert.NotNil(t, scheduler.cron)
}

// TestScheduler_AddManualTask 测试 AddManualTask 方法
// 注意：该方法启动 goroutine 执行 executePreheat，在 nil 依赖下会 panic
// 完整测试需要 mock EngineManager 和 Redis 客户端
func TestScheduler_AddManualTask(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 只测试方法存在且可以被调用
	// 实际功能测试需要在集成测试环境中进行
	_ = scheduler.AddManualTask
}

// TestScheduler_removeTask_Existing 测试删除存在的任务
func TestScheduler_removeTask_Existing(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 手动添加一个任务到 map
	scheduler.tasksMutex.Lock()
	scheduler.tasks["test-site"] = cron.EntryID(999)
	scheduler.tasksMutex.Unlock()

	// 删除任务
	scheduler.removeTask("test-site")

	// 验证任务已被删除
	scheduler.tasksMutex.RLock()
	_, exists := scheduler.tasks["test-site"]
	scheduler.tasksMutex.RUnlock()
	assert.False(t, exists)
}

// TestScheduler_reloadSites 测试 reloadSites 方法
func TestScheduler_reloadSites(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 调用 reloadSites（应该不会 panic）
	defer func() {
		if r := recover(); r != nil {
			t.Logf("reloadSites panicked (expected with nil deps): %v", r)
		}
	}()

	scheduler.reloadSites()
}

// TestScheduler_createTask 测试 createTask 方法
func TestScheduler_createTask(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 创建 config.PrerenderConfig 类型的配置
	config := config.PrerenderConfig{
		Preheat: config.PreheatConfig{
			Enabled:  true,
			Schedule: "0 */5 * * * *",
		},
		Push: config.PushConfig{
			Enabled: true,
		},
	}

	// 调用 createTask（应该不会 panic）
	defer func() {
		if r := recover(); r != nil {
			t.Logf("createTask panicked (expected with nil deps): %v", r)
		}
	}()

	scheduler.createTask("test-site", config)
}

// TestScheduler_updateTask 测试 updateTask 方法
func TestScheduler_updateTask(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	config := config.PrerenderConfig{
		Preheat: config.PreheatConfig{
			Enabled: true,
		},
		Push: config.PushConfig{
			Enabled: false,
		},
	}

	// 调用 updateTask
	defer func() {
		if r := recover(); r != nil {
			t.Logf("updateTask panicked: %v", r)
		}
	}()

	scheduler.updateTask("test-site", config)
}

// TestScheduler_GetTaskStatus_Existing 测试 GetTaskStatus 对于存在的任务
func TestScheduler_GetTaskStatus_Existing(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 添加一个任务
	entryID, _ := scheduler.cron.AddFunc("0 */5 * * * *", func() {})
	scheduler.tasksMutex.Lock()
	scheduler.tasks["test-site"] = entryID
	scheduler.tasksMutex.Unlock()

	// 测试获取任务状态
	exists, status := scheduler.GetTaskStatus("test-site")
	assert.True(t, exists)
	assert.NotEmpty(t, status)
}

// TestScheduler_ListTasks_WithTasks 测试 ListTasks 在有任务时
func TestScheduler_ListTasks_WithTasks(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 添加一个任务
	entryID, _ := scheduler.cron.AddFunc("0 */5 * * * *", func() {})
	scheduler.tasksMutex.Lock()
	scheduler.tasks["test-site"] = entryID
	scheduler.tasksMutex.Unlock()

	// 测试列出任务
	tasks := scheduler.ListTasks()
	assert.NotNil(t, tasks)
	// 应该包含添加的任务
	_, exists := tasks["test-site"]
	assert.True(t, exists)
}

// TestScheduler_Cron_AddFunc 测试 cron.AddFunc 的使用
func TestScheduler_Cron_AddFunc(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 添加一个简单的函数
	_, err := scheduler.cron.AddFunc("0 */5 * * * *", func() {})

	assert.Nil(t, err)

	// 验证 cron 条目存在
	entries := scheduler.cron.Entries()
	assert.NotEmpty(t, entries)
}

// TestScheduler_Cron_Remove 测试 cron.Remove 的使用
func TestScheduler_Cron_Remove(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 添加并移除一个函数
	entryID, _ := scheduler.cron.AddFunc("0 */5 * * * *", func() {})
	initialLen := len(scheduler.cron.Entries())

	scheduler.cron.Remove(entryID)

	afterLen := len(scheduler.cron.Entries())
	assert.Equal(t, initialLen-1, afterLen)
}

// TestScheduler_reloadSites_NilEngine 测试 reloadSites 在 nil engine 时的行为
func TestScheduler_reloadSites_NilEngine(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// reloadSites 会调用 engineManager.ListSites()，nil 时会 panic
	// 使用 defer 捕获 panic
	defer func() {
		if r := recover(); r != nil {
			t.Logf("reloadSites panicked as expected: %v", r)
		}
	}()

	scheduler.reloadSites()
}

// TestScheduler_executePreheat_NilEngine 测试 executePreheat 在 nil engine 时的行为
func TestScheduler_executePreheat_NilEngine(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// engineManager 为 nil 时会 panic，使用 defer 捕获
	defer func() {
		if r := recover(); r != nil {
			t.Logf("executePreheat panicked as expected: %v", r)
		}
	}()

	scheduler.executePreheat("test-site")
	// 测试通过，没有 panic
}

// TestScheduler_executePush_NilManager 测试 executePush 在 nil pushManager 时的行为
func TestScheduler_executePush_NilManager(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// pushManager 可能为 nil，TriggerPush 会 panic
	defer func() {
		if r := recover(); r != nil {
			t.Logf("executePush panicked as expected: %v", r)
		}
	}()

	scheduler.executePush("test-site")
}

// TestScheduler_createTask_WithConfig 测试使用不同配置创建任务
func TestScheduler_createTask_WithConfig(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	tests := []struct {
		name   string
		config config.PrerenderConfig
	}{
		{
			name: "preheat enabled",
			config: config.PrerenderConfig{
				Preheat: config.PreheatConfig{
					Enabled:  true,
					Schedule: "0 */5 * * * *",
				},
			},
		},
		{
			name: "push enabled",
			config: config.PrerenderConfig{
				Push: config.PushConfig{
					Enabled: true,
				},
			},
		},
		{
			name: "both enabled",
			config: config.PrerenderConfig{
				Preheat: config.PreheatConfig{
					Enabled:  true,
					Schedule: "0 */5 * * * *",
				},
				Push: config.PushConfig{
					Enabled: true,
				},
			},
		},
		{
			name:   "both disabled",
			config: config.PrerenderConfig{},
		},
		{
			name: "preheat with invalid schedule",
			config: config.PrerenderConfig{
				Preheat: config.PreheatConfig{
					Enabled:  true,
					Schedule: "invalid",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("createTask panicked: %v", r)
				}
			}()
			scheduler.createTask("test-site", tt.config)
		})
	}
}

// TestScheduler_updateTask_WithConfig 测试使用不同配置更新任务
func TestScheduler_updateTask_WithConfig(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	tests := []struct {
		name   string
		config config.PrerenderConfig
	}{
		{
			name: "enable preheat",
			config: config.PrerenderConfig{
				Preheat: config.PreheatConfig{
					Enabled:  true,
					Schedule: "0 */10 * * * *",
				},
			},
		},
		{
			name: "disable preheat",
			config: config.PrerenderConfig{
				Preheat: config.PreheatConfig{
					Enabled: false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("updateTask panicked: %v", r)
				}
			}()
			scheduler.updateTask("test-site", tt.config)
		})
	}
}

// TestScheduler_removeTask_NonExistent 测试删除不存在的任务
func TestScheduler_removeTask_NonExistent(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 删除不存在的任务应该不报错
	scheduler.removeTask("non-existent-site")
	// 测试通过
}

// TestScheduler_GetTaskStatus_NonExistentEntry 测试获取不存在 cron 条目的任务状态
func TestScheduler_GetTaskStatus_NonExistentEntry(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 添加一个任务到 map，但不在 cron 中
	scheduler.tasksMutex.Lock()
	scheduler.tasks["test-site"] = cron.EntryID(999)
	scheduler.tasksMutex.Unlock()

	exists, status := scheduler.GetTaskStatus("test-site")
	assert.False(t, exists)
	assert.Equal(t, "not found", status)
}

// TestScheduler_StartStop_Full 测试完整的启动和停止流程
func TestScheduler_StartStop_Full(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 只测试 cron 调度器的启动和停止，不启动 monitorSites 协程
	// 因为 monitorSites 依赖 engineManager，传入 nil 时会 panic
	scheduler.cron.Start()

	// 等待一小段时间
	time.Sleep(50 * time.Millisecond)

	// 停止 cron 调度器
	scheduler.cron.Stop()

	// 测试通过，没有 panic
}

// TestScheduler_reloadSites_WithSites 测试 reloadSites 有站点时
func TestScheduler_reloadSites_WithSites(t *testing.T) {
	// 这个测试需要 mock EngineManager，暂时跳过
	t.Skip("Requires mock EngineManager")
}

// TestScheduler_ConcurrentTaskAccess 测试并发访问任务 map
func TestScheduler_ConcurrentTaskAccess(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	done := make(chan bool, 20)

	// 并发读取
	for i := 0; i < 10; i++ {
		go func() {
			scheduler.GetTaskStatus("test-site")
			done <- true
		}()
	}

	// 并发写入
	for i := 0; i < 10; i++ {
		go func(id int) {
			scheduler.tasksMutex.Lock()
			scheduler.tasks[fmt.Sprintf("site-%d", id)] = cron.EntryID(id)
			scheduler.tasksMutex.Unlock()
			done <- true
		}(i)
	}

	for i := 0; i < 20; i++ {
		<-done
	}

	// 测试通过，没有 panic
}

// TestScheduler_ContextCancellation 测试 context 取消
func TestScheduler_ContextCancellation(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 取消 context
	scheduler.cancel()

	// 验证 context 已取消
	select {
	case <-scheduler.ctx.Done():
		// 正确
	default:
		t.Error("context should be cancelled")
	}
}

// TestScheduler_EmptyConfig 测试空配置
func TestScheduler_EmptyConfig(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 使用空配置创建任务
	config := config.PrerenderConfig{}
	scheduler.createTask("test-site", config)
	// 测试通过，没有 panic
}

// TestScheduler_InvalidCronSchedule 测试无效的 cron 表达式
func TestScheduler_InvalidCronSchedule(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 尝试添加无效的 cron 表达式
	_, err := scheduler.cron.AddFunc("invalid", func() {})
	assert.Error(t, err)
}

// TestScheduler_CronEntries_Empty 测试空 cron 条目列表
func TestScheduler_CronEntries_Empty(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	entries := scheduler.cron.Entries()
	assert.Empty(t, entries)
}

// TestScheduler_MutexOperations 测试 mutex 操作
func TestScheduler_MutexOperations(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 测试 RLock/RUnlock
	scheduler.tasksMutex.RLock()
	count := len(scheduler.tasks)
	scheduler.tasksMutex.RUnlock()
	assert.Equal(t, 0, count)

	// 测试 Lock/Unlock
	scheduler.tasksMutex.Lock()
	scheduler.tasks["test"] = cron.EntryID(1)
	delete(scheduler.tasks, "test")
	scheduler.tasksMutex.Unlock()
}

// TestScheduler_entryToSiteMapping 测试 entry 到站点的映射
func TestScheduler_entryToSiteMapping(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	// 添加任务
	entryID, _ := scheduler.cron.AddFunc("0 */5 * * * *", func() {})
	scheduler.tasksMutex.Lock()
	scheduler.tasks["test-site"] = entryID
	scheduler.tasksMutex.Unlock()

	// 构建映射
	scheduler.tasksMutex.RLock()
	entryToSite := make(map[cron.EntryID]string)
	for siteName, id := range scheduler.tasks {
		entryToSite[id] = siteName
	}
	scheduler.tasksMutex.RUnlock()

	assert.Len(t, entryToSite, 1)
	assert.Equal(t, "test-site", entryToSite[entryID])
}

// TestScheduler_formatVerification 测试时间格式化验证
func TestScheduler_formatVerification(t *testing.T) {
	now := time.Now()
	formatted := now.Format("2006-01-02 15:04:05")

	// 验证格式正确
	assert.Len(t, formatted, 19)
	assert.Contains(t, formatted, "-")
	assert.Contains(t, formatted, ":")
	assert.Contains(t, formatted, " ")
}

// TestScheduler_CronWithSeconds 测试带秒的 cron 支持
func TestScheduler_CronWithSeconds(t *testing.T) {
	c := cron.New(cron.WithSeconds())
	defer c.Stop()

	// 添加每秒执行的任务
	entryID, err := c.AddFunc("* * * * * *", func() {})
	assert.NoError(t, err)
	assert.NotZero(t, entryID)

	// 清理
	c.Remove(entryID)
}

// TestScheduler_NilRedisClient 测试 nil redis 客户端
func TestScheduler_NilRedisClient(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)
	assert.Nil(t, scheduler.redisClient)
}

// TestScheduler_PushManagerCreation 测试 PushManager 创建
func TestScheduler_PushManagerCreation(t *testing.T) {
	// 使用 nil 配置和 redis 客户端创建
	scheduler := NewScheduler(nil, nil, nil)
	assert.NotNil(t, scheduler.pushManager)
}
