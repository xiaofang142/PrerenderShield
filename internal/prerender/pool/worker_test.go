package pool

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// createMockChromePool 创建一个模拟的 Chrome pool 用于测试
func createMockChromePool() *Pool {
	// 创建一个最小化的 Pool 实例用于测试
	// 由于 WorkerPool 只是持有 pool 引用，我们只需要一个非 nil 的实例
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		MinInstances:        0,
		MaxInstances:        1,
		Headless:            true,
		HealthCheckInterval: 30 * time.Second,
	}
	return NewPool(cfg, logger)
}

// TestWorkerStatus_String 测试 WorkerStatus 字符串转换
func TestWorkerStatus_String(t *testing.T) {
	tests := []struct {
		status   WorkerStatus
		expected string
	}{
		{WorkerStatusIdle, "idle"},
		{WorkerStatusBusy, "busy"},
		{WorkerStatusStopped, "stopped"},
		{WorkerStatus(999), "unknown"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.status.String())
	}
}

// TestTaskType_String 测试 TaskType 字符串转换
func TestTaskType_String(t *testing.T) {
	tests := []struct {
		taskType TaskType
		expected string
	}{
		{TaskTypeRender, "render"},
		{TaskTypeScreenshot, "screenshot"},
		{TaskTypePDF, "pdf"},
		{TaskTypeMetrics, "metrics"},
		{TaskType(999), "unknown"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.taskType.String())
	}
}

// TestPriority_Constants 测试优先级常量
func TestPriority_Constants(t *testing.T) {
	assert.Equal(t, Priority(1), PriorityLow)
	assert.Equal(t, Priority(5), PriorityNormal)
	assert.Equal(t, Priority(8), PriorityHigh)
	assert.Equal(t, Priority(10), PriorityVIP)
}

// TestDefaultWorkerConfig 测试默认 Worker 配置
func TestDefaultWorkerConfig(t *testing.T) {
	cfg := DefaultWorkerConfig()
	assert.Equal(t, 1000, cfg.MaxTasksPerWorker)
	assert.Equal(t, 30*time.Second, cfg.TaskTimeout)
	assert.Equal(t, 100*time.Millisecond, cfg.CooldownTime)
}

// TestNewWorkerPool_NilLogger 测试 nil logger 处理
func TestNewWorkerPool_NilLogger(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 3,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	wp := NewWorkerPool(cfg, workerCfg, chromePool, nil)
	assert.NotNil(t, wp)
	assert.NotNil(t, wp.logger)

	err := wp.Close()
	assert.NoError(t, err)
}

// TestNewWorkerPool_Basic 测试基本创建
func TestNewWorkerPool_Basic(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 3,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	assert.NotNil(t, wp)
	assert.NotNil(t, wp.workers)
	assert.NotNil(t, wp.available)
	assert.NotNil(t, wp.taskQueue)
	assert.NotNil(t, wp.priorityQueues)

	err := wp.Close()
	assert.NoError(t, err)
}

// TestNewWorkerPool_WithMinInstances 测试最小实例数
func TestNewWorkerPool_WithMinInstances(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 2,
		MaxInstances: 5,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	// 等待 worker 初始化
	time.Sleep(100 * time.Millisecond)

	stats := wp.Stats()
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats["total_workers"], 2)
}

// TestWorkerPool_Submit 测试提交任务
func TestWorkerPool_Submit(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 2,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	task := &WorkTask{
		ID:         "test-task-1",
		Type:       TaskTypeRender,
		URL:        "https://example.com",
		SiteID:     "test-site",
		Priority:   PriorityNormal,
		Timeout:    10 * time.Second,
		ResultChan: make(chan *TaskResult, 1),
		CreatedAt:  time.Now(),
	}

	err := wp.Submit(task)
	assert.NoError(t, err)
}

// TestWorkerPool_Submit_ClosedPool 测试关闭后提交任务
func TestWorkerPool_Submit_ClosedPool(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 2,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	wp.Close()

	task := &WorkTask{
		ID:       "test-task-1",
		Type:     TaskTypeRender,
		URL:      "https://example.com",
		Priority: PriorityNormal,
	}

	err := wp.Submit(task)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "worker pool is closed")
}

// TestWorkerPool_Submit_NilResultChan 测试自动创建 ResultChan
func TestWorkerPool_Submit_NilResultChan(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 2,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	task := &WorkTask{
		ID:       "test-task-1",
		Type:     TaskTypeRender,
		URL:      "https://example.com",
		Priority: PriorityNormal,
	}

	err := wp.Submit(task)
	assert.NoError(t, err)
	assert.NotNil(t, task.ResultChan)
}

// TestWorkerPool_SubmitSync 测试同步提交任务
func TestWorkerPool_SubmitSync(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 2,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	task := &WorkTask{
		ID:        "test-task-1",
		Type:      TaskTypeRender,
		URL:       "https://example.com",
		Priority:  PriorityNormal,
		Timeout:   10 * time.Second,
		CreatedAt: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 注意：由于没有实际的 Chrome 实例，任务会失败，但方法应该能被调用
	_, err := wp.SubmitSync(ctx, task)
	// 允许超时或上下文取消错误
	assert.Error(t, err)
}

// TestWorkerPool_SubmitSync_ClosedPool 测试关闭后同步提交
func TestWorkerPool_SubmitSync_ClosedPool(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 2,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	wp.Close()

	task := &WorkTask{
		ID:       "test-task-1",
		Type:     TaskTypeRender,
		URL:      "https://example.com",
		Priority: PriorityNormal,
	}

	ctx := context.Background()
	_, err := wp.SubmitSync(ctx, task)
	assert.Error(t, err)
}

// TestWorkerPool_SubmitSync_ContextCancelled 测试上下文取消
func TestWorkerPool_SubmitSync_ContextCancelled(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 2,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	task := &WorkTask{
		ID:       "test-task-1",
		Type:     TaskTypeRender,
		URL:      "https://example.com",
		Priority: PriorityNormal,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	_, err := wp.SubmitSync(ctx, task)
	assert.Error(t, err)
}

// TestWorkerPool_Stats 测试统计信息
func TestWorkerPool_Stats(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 3,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	stats := wp.Stats()
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "total_workers")
	assert.Contains(t, stats, "available")
	assert.Contains(t, stats, "queue_length")
	assert.Contains(t, stats, "closed")
	assert.Contains(t, stats, "status_distribution")
	assert.Contains(t, stats, "total_tasks")
	assert.Contains(t, stats, "total_success")
	assert.Contains(t, stats, "total_fail")
	assert.Contains(t, stats, "priority_queues")
}

// TestWorkerPool_ScaleUp 测试扩展
func TestWorkerPool_ScaleUp(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 5,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	err := wp.ScaleUp(2)
	assert.NoError(t, err)

	// 等待 worker 创建
	time.Sleep(100 * time.Millisecond)

	stats := wp.Stats()
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats["total_workers"], 1)
}

// TestWorkerPool_ScaleUp_ClosedPool 测试关闭后扩展
func TestWorkerPool_ScaleUp_ClosedPool(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 5,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	wp.Close()

	err := wp.ScaleUp(2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "worker pool is closed")
}

// TestWorkerPool_ScaleUp_ToMax 测试扩展到最大值
func TestWorkerPool_ScaleUp_ToMax(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 3,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	// 尝试扩展超过最大值
	err := wp.ScaleUp(10)
	assert.NoError(t, err)

	// 等待 worker 创建
	time.Sleep(100 * time.Millisecond)

	stats := wp.Stats()
	assert.NotNil(t, stats)
	// 总数不应超过 MaxInstances
	assert.LessOrEqual(t, stats["total_workers"], cfg.MaxInstances)
}

// TestWorkerPool_ScaleDown 测试缩小
func TestWorkerPool_ScaleDown(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 5,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	// 先扩展
	err := wp.ScaleUp(2)
	assert.NoError(t, err)

	// 等待 worker 创建
	time.Sleep(100 * time.Millisecond)

	// 再缩小
	err = wp.ScaleDown(1)
	assert.NoError(t, err)
}

// TestWorkerPool_ScaleDown_ClosedPool 测试关闭后缩小
func TestWorkerPool_ScaleDown_ClosedPool(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 5,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	wp.Close()

	err := wp.ScaleDown(1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "worker pool is closed")
}

// TestWorkerPool_ScaleDown_NoIdleInstances 测试没有空闲实例时缩小
func TestWorkerPool_ScaleDown_NoIdleInstances(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 2,
		MaxInstances: 5,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	// 等待初始化
	time.Sleep(100 * time.Millisecond)

	// 尝试缩小到低于 MinInstances
	err := wp.ScaleDown(5)
	assert.NoError(t, err)
}

// TestWorkerPool_Close 测试关闭
func TestWorkerPool_Close(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 2,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)

	err := wp.Close()
	assert.NoError(t, err)

	// 重复关闭应该没问题
	err = wp.Close()
	assert.NoError(t, err)
}

// TestWorkerPool_Close_MultipleTimes 测试多次关闭
func TestWorkerPool_Close_MultipleTimes(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 2,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)

	// 多次关闭
	for i := 0; i < 3; i++ {
		err := wp.Close()
		assert.NoError(t, err)
	}
}

// TestWorkerPool_PriorityQueues 测试优先级队列
func TestWorkerPool_PriorityQueues(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 2,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	// 测试不同优先级
	priorities := []Priority{PriorityLow, PriorityNormal, PriorityHigh, PriorityVIP}
	for _, p := range priorities {
		task := &WorkTask{
			ID:       "task-" + string(rune(p)),
			Type:     TaskTypeRender,
			URL:      "https://example.com",
			Priority: p,
		}
		err := wp.Submit(task)
		assert.NoError(t, err)
	}
}

// TestWorkerPool_DifferentTaskTypes 测试不同任务类型
func TestWorkerPool_DifferentTaskTypes(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 2,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	taskTypes := []TaskType{TaskTypeRender, TaskTypeScreenshot, TaskTypePDF, TaskTypeMetrics}
	for _, tt := range taskTypes {
		task := &WorkTask{
			ID:       "task-" + string(rune(tt)),
			Type:     tt,
			URL:      "https://example.com",
			Priority: PriorityNormal,
		}
		err := wp.Submit(task)
		assert.NoError(t, err)
	}
}

// TestWorkTask_Struct 测试 WorkTask 结构
func TestWorkTask_Struct(t *testing.T) {
	task := &WorkTask{
		ID:        "test-123",
		Type:      TaskTypeRender,
		URL:       "https://example.com",
		SiteID:    "test-site",
		Priority:  PriorityHigh,
		Timeout:   30 * time.Second,
		CreatedAt: time.Now(),
		Options: TaskOptions{
			WaitUntil:      "networkidle0",
			WaitSelector:   "#content",
			Headers:        map[string]string{"X-Custom": "value"},
			BlockResources: true,
			ScreenshotFull: true,
			PDFLandscape:   true,
			UserAgent:      "Custom-Agent",
		},
	}

	assert.Equal(t, "test-123", task.ID)
	assert.Equal(t, TaskTypeRender, task.Type)
	assert.Equal(t, "https://example.com", task.URL)
	assert.Equal(t, PriorityHigh, task.Priority)
}

// TestTaskOptions_Struct 测试 TaskOptions 结构
func TestTaskOptions_Struct(t *testing.T) {
	opts := TaskOptions{
		WaitUntil:      "domcontentloaded",
		WaitSelector:   ".main",
		Headers:        make(map[string]string),
		Cookies:        []Cookie{{Name: "test", Value: "value"}},
		BlockResources: false,
		ScreenshotFull: false,
		PDFLandscape:   false,
		UserAgent:      "Test-Agent",
	}

	assert.Equal(t, "domcontentloaded", opts.WaitUntil)
	assert.Equal(t, ".main", opts.WaitSelector)
	assert.NotNil(t, opts.Headers)
	assert.Len(t, opts.Cookies, 1)
}

// TestCookie_Struct 测试 Cookie 结构
func TestCookie_Struct(t *testing.T) {
	cookie := Cookie{
		Name:     "session",
		Value:    "abc123",
		Domain:   "example.com",
		Path:     "/",
		Expires:  time.Now().Add(time.Hour),
		Secure:   true,
		HttpOnly: true,
		SameSite: "Strict",
	}

	assert.Equal(t, "session", cookie.Name)
	assert.Equal(t, "abc123", cookie.Value)
	assert.True(t, cookie.Secure)
	assert.True(t, cookie.HttpOnly)
}

// TestTaskResult_Struct 测试 TaskResult 结构
func TestTaskResult_Struct(t *testing.T) {
	result := &TaskResult{
		TaskID:     "test-123",
		Success:    true,
		Error:      nil,
		HTML:       "<html>test</html>",
		Screenshot: []byte("image-data"),
		PDF:        []byte("pdf-data"),
		Metrics:    &PageMetrics{},
		Duration:   100 * time.Millisecond,
	}

	assert.Equal(t, "test-123", result.TaskID)
	assert.True(t, result.Success)
	assert.Nil(t, result.Error)
	assert.Equal(t, "<html>test</html>", result.HTML)
}

// TestPageMetrics_Struct 测试 PageMetrics 结构
func TestPageMetrics_Struct(t *testing.T) {
	metrics := &PageMetrics{
		LoadTime:               1 * time.Second,
		DOMContentLoaded:       500 * time.Millisecond,
		FirstPaint:             100 * time.Millisecond,
		FirstContentfulPaint:   200 * time.Millisecond,
		TotalBlockingTime:      50 * time.Millisecond,
		LargestContentfulPaint: 800 * time.Millisecond,
		CumulativeLayoutShift:  0.1,
	}

	assert.Equal(t, 1*time.Second, metrics.LoadTime)
	assert.Equal(t, 500*time.Millisecond, metrics.DOMContentLoaded)
	assert.Equal(t, 0.1, metrics.CumulativeLayoutShift)
}

// TestWorker_Struct 测试 Worker 结构
func TestWorker_Struct(t *testing.T) {
	worker := &Worker{
		ID:           "worker-test",
		status:       WorkerStatusIdle,
		taskCount:    100,
		successCount: 95,
		failCount:    5,
		createdAt:    time.Now(),
		lastTaskAt:   time.Now(),
	}

	assert.Equal(t, "worker-test", worker.ID)
	assert.Equal(t, WorkerStatusIdle, worker.status)
	assert.Equal(t, int64(100), worker.taskCount)
	assert.Equal(t, int64(95), worker.successCount)
	assert.Equal(t, int64(5), worker.failCount)
}

// TestWorkerConfig_Struct 测试 WorkerConfig 结构
func TestWorkerConfig_Struct(t *testing.T) {
	cfg := WorkerConfig{
		MaxTasksPerWorker: 500,
		TaskTimeout:       60 * time.Second,
		CooldownTime:      200 * time.Millisecond,
	}

	assert.Equal(t, 500, cfg.MaxTasksPerWorker)
	assert.Equal(t, 60*time.Second, cfg.TaskTimeout)
	assert.Equal(t, 200*time.Millisecond, cfg.CooldownTime)
}

// TestWorkerPool_ConcurrentAccess 测试并发访问
func TestWorkerPool_ConcurrentAccess(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 2,
		MaxInstances: 5,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	done := make(chan bool, 10)

	// 并发提交任务
	for i := 0; i < 5; i++ {
		go func(id int) {
			task := &WorkTask{
				ID:       "task-" + string(rune('0'+id)),
				Type:     TaskTypeRender,
				URL:      "https://example.com",
				Priority: PriorityNormal,
			}
			wp.Submit(task)
			done <- true
		}(i)
	}

	// 并发获取统计
	for i := 0; i < 5; i++ {
		go func() {
			wp.Stats()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestWorkerPool_createWorker 测试创建 worker
func TestWorkerPool_createWorker(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 0, // 不自动创建
		MaxInstances: 5,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	worker := wp.createWorker()
	assert.NotNil(t, worker)
	assert.NotEmpty(t, worker.ID)
	assert.Equal(t, WorkerStatusIdle, worker.status)
}

// TestWorkerPool_checkWorkers 测试检查 worker 状态
func TestWorkerPool_checkWorkers(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 3,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()
	workerCfg.TaskTimeout = 100 * time.Millisecond // 缩短超时用于测试

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	// 等待 worker 初始化
	time.Sleep(100 * time.Millisecond)

	// 调用 checkWorkers 应该不会 panic
	wp.checkWorkers()
}

// TestWorkerPool_replaceWorker 测试替换 worker
func TestWorkerPool_replaceWorker(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 3,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	// 等待 worker 初始化
	time.Sleep(100 * time.Millisecond)

	wp.mu.RLock()
	if len(wp.workers) > 0 {
		oldWorker := wp.workers[0]
		wp.mu.RUnlock()

		// 替换 worker
		wp.replaceWorker(oldWorker)

		// 等待替换完成
		time.Sleep(100 * time.Millisecond)

		stats := wp.Stats()
		assert.NotNil(t, stats)
	} else {
		wp.mu.RUnlock()
	}
}

// TestWorker_stop 测试停止 worker
func TestWorker_stop(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 0,
		MaxInstances: 2,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	worker := wp.createWorker()
	assert.NotNil(t, worker)

	// 多次停止应该不会 panic
	worker.stop()
	worker.stop()
}

// TestWorkerPool_monitor 测试监控协程
func TestWorkerPool_monitor(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 3,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()
	workerCfg.TaskTimeout = 100 * time.Millisecond

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	// 等待监控协程运行
	time.Sleep(200 * time.Millisecond)

	stats := wp.Stats()
	assert.NotNil(t, stats)
}

// TestWorkerPool_taskDispatcher 测试任务分发器
func TestWorkerPool_taskDispatcher(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 2,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	// 提交不同优先级的任务
	priorities := []Priority{PriorityVIP, PriorityHigh, PriorityNormal, PriorityLow}
	for _, p := range priorities {
		task := &WorkTask{
			ID:       "task-" + string(rune(p)),
			Type:     TaskTypeRender,
			URL:      "https://example.com",
			Priority: p,
		}
		err := wp.Submit(task)
		assert.NoError(t, err)
	}

	// 等待任务分发
	time.Sleep(100 * time.Millisecond)

	stats := wp.Stats()
	assert.NotNil(t, stats)
}

// TestPriority_String 测试 Priority 字符串表示
func TestPriority_String(t *testing.T) {
	assert.Equal(t, "render", TaskTypeRender.String())
	assert.Equal(t, "screenshot", TaskTypeScreenshot.String())
	assert.Equal(t, "pdf", TaskTypePDF.String())
	assert.Equal(t, "metrics", TaskTypeMetrics.String())
}

// TestWorkerPool_QueueFull 测试队列满的情况
func TestWorkerPool_QueueFull(t *testing.T) {
	chromePool := createMockChromePool()


	cfg := Config{
		MinInstances: 1,
		MaxInstances: 2,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	// 每个优先级队列的容量是 MaxInstances*5 = 10
	// 填满所有优先级队列（PriorityVIP, PriorityHigh, PriorityNormal, PriorityLow）
	queueCapacity := cfg.MaxInstances * 5
	for p := PriorityVIP; p >= PriorityLow; p-- {
		for i := 0; i < queueCapacity; i++ {
			task := &WorkTask{
				ID:       "task-p" + string(rune(p)) + "-" + string(rune('0'+i%10)),
				Type:     TaskTypeRender,
				URL:      "https://example.com",
				Priority: p,
			}
			wp.Submit(task)
		}
	}

	// 再提交应该失败
	task := &WorkTask{
		ID:       "overflow-task",
		Type:     TaskTypeRender,
		URL:      "https://example.com",
		Priority: PriorityLow,
	}
	err := wp.Submit(task)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task queue is full")
}

// TestWorkerPool_Close_WithPendingTasks 测试关闭时有待处理任务
func TestWorkerPool_Close_WithPendingTasks(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 1,
		MaxInstances: 2,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)

	// 提交一些任务
	for i := 0; i < 5; i++ {
		task := &WorkTask{
			ID:       "task-" + string(rune('0'+i)),
			Type:     TaskTypeRender,
			URL:      "https://example.com",
			Priority: PriorityNormal,
		}
		wp.Submit(task)
	}

	// 立即关闭
	err := wp.Close()
	assert.NoError(t, err)
}

// TestWorkerPool_Stats_Distribution 测试统计分布
func TestWorkerPool_Stats_Distribution(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 2,
		MaxInstances: 5,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	// 等待初始化
	time.Sleep(100 * time.Millisecond)

	stats := wp.Stats()
	assert.NotNil(t, stats)

	statusDist, ok := stats["status_distribution"].(map[string]int)
	assert.True(t, ok)
	assert.Contains(t, statusDist, "idle")
	assert.Contains(t, statusDist, "busy")
	assert.Contains(t, statusDist, "stopped")
}

// TestWorkerPool_EmptyConfig 测试空配置
func TestWorkerPool_EmptyConfig(t *testing.T) {
	chromePool := createMockChromePool()
	

	cfg := Config{
		MinInstances: 0,
		MaxInstances: 1,
		Headless:     true,
	}
	workerCfg := DefaultWorkerConfig()

	logger, _ := zap.NewDevelopment()
	wp := NewWorkerPool(cfg, workerCfg, chromePool, logger)
	defer wp.Close()

	stats := wp.Stats()
	assert.NotNil(t, stats)
	assert.Equal(t, 0, stats["total_workers"])
}
