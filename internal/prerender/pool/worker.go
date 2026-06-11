// DEPRECATED: WorkerPool 设计未完成（参见 docs/superpowers/plans/2026-06-10-chrome-pool-fix.md）。
// 生产路径 engine.go → pool.Pool 不使用此模块。问题概述：
//   - Worker.run()/waitForTask() 与 taskDispatcher() 之间缺少任务下发通道
//   - waitForTask() 的 select default 导致立即返回，run() 循环不断 acquire Chrome 实例
//   - dispatcher 仅设置 worker.currentTask 字段，从不调用 executeTask()
// 如需启用，须用 channel 模型重写：worker 阻塞读取 taskChan，dispatcher 写入 taskChan。
package pool

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"go.uber.org/zap"
)

// WorkerStatus 工作进程状态
type WorkerStatus int

const (
	WorkerStatusIdle WorkerStatus = iota
	WorkerStatusBusy
	WorkerStatusStopped
)

func (s WorkerStatus) String() string {
	switch s {
	case WorkerStatusIdle:
		return "idle"
	case WorkerStatusBusy:
		return "busy"
	case WorkerStatusStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// TaskType 任务类型
type TaskType int

const (
	TaskTypeRender TaskType = iota
	TaskTypeScreenshot
	TaskTypePDF
	TaskTypeMetrics
)

func (t TaskType) String() string {
	switch t {
	case TaskTypeRender:
		return "render"
	case TaskTypeScreenshot:
		return "screenshot"
	case TaskTypePDF:
		return "pdf"
	case TaskTypeMetrics:
		return "metrics"
	default:
		return "unknown"
	}
}

// WorkTask 工作任务
type WorkTask struct {
	ID         string
	Type       TaskType
	URL        string
	SiteID     string
	Priority   Priority
	Timeout    time.Duration
	Options    TaskOptions
	ResultChan chan *TaskResult
	CreatedAt  time.Time
}

// TaskOptions 任务选项
type TaskOptions struct {
	WaitUntil      string
	WaitSelector   string
	Headers        map[string]string
	Cookies        []Cookie
	BlockResources bool
	ScreenshotFull bool
	PDFLandscape   bool
	UserAgent      string
}

// Cookie Cookie 结构
type Cookie struct {
	Name     string
	Value    string
	Domain   string
	Path     string
	Expires  time.Time
	Secure   bool
	HttpOnly bool
	SameSite string
}

// TaskResult 任务结果
type TaskResult struct {
	TaskID     string
	Success    bool
	Error      error
	HTML       string
	Screenshot []byte
	PDF        []byte
	Metrics    *PageMetrics
	Duration   time.Duration
}

// PageMetrics 页面性能指标
type PageMetrics struct {
	LoadTime               time.Duration
	DOMContentLoaded       time.Duration
	FirstPaint             time.Duration
	FirstContentfulPaint   time.Duration
	TotalBlockingTime      time.Duration
	LargestContentfulPaint time.Duration
	CumulativeLayoutShift  float64
}

// Priority 优先级
type Priority int

const (
	PriorityLow    Priority = 1
	PriorityNormal Priority = 5
	PriorityHigh   Priority = 8
	PriorityVIP    Priority = 10
)

// Worker 工作进程
type Worker struct {
	ID            string
	pool          *WorkerPool
	instance      *Instance
	status        WorkerStatus
	currentTask   *WorkTask
	taskCount     int64
	successCount  int64
	failCount     int64
	totalDuration time.Duration
	lastTaskAt    time.Time
	createdAt     time.Time
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// WorkerConfig 工作进程配置
type WorkerConfig struct {
	MaxTasksPerWorker int           // 单个 worker 最大处理任务数
	TaskTimeout       time.Duration // 默认任务超时
	CooldownTime      time.Duration // 任务间隔冷却时间
}

func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		MaxTasksPerWorker: 1000,
		TaskTimeout:       30 * time.Second,
		CooldownTime:      100 * time.Millisecond,
	}
}

// WorkerPool 工作进程池
type WorkerPool struct {
	config         Config
	workerConfig   WorkerConfig
	workers        []*Worker
	available      chan *Worker
	taskQueue      chan *WorkTask
	priorityQueues map[Priority]chan *WorkTask
	mu             sync.RWMutex
	closed         bool
	logger         *zap.Logger
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	chromePool     *Pool
	workerCount    int32
}

// NewWorkerPool 创建工作进程池
func NewWorkerPool(config Config, workerConfig WorkerConfig, chromePool *Pool, logger *zap.Logger) *WorkerPool {
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())

	wp := &WorkerPool{
		config:         config,
		workerConfig:   workerConfig,
		workers:        make([]*Worker, 0, config.MaxInstances),
		available:      make(chan *Worker, config.MaxInstances),
		taskQueue:      make(chan *WorkTask, config.MaxInstances*10),
		priorityQueues: make(map[Priority]chan *WorkTask),
		logger:         logger,
		ctx:            ctx,
		cancel:         cancel,
		chromePool:     chromePool,
	}

	// 创建优先级队列
	for p := PriorityVIP; p >= PriorityLow; p-- {
		wp.priorityQueues[p] = make(chan *WorkTask, config.MaxInstances*5)
	}

	// 初始化 worker
	for i := 0; i < config.MinInstances; i++ {
		worker := wp.createWorker()
		if worker != nil {
			wp.workers = append(wp.workers, worker)
			wp.available <- worker
			atomic.AddInt32(&wp.workerCount, 1)
		}
	}

	// 启动任务分发器
	wp.wg.Add(1)
	go wp.taskDispatcher()

	// 启动监控协程
	wp.wg.Add(1)
	go wp.monitor()

	return wp
}

// createWorker 创建新 worker
func (wp *WorkerPool) createWorker() *Worker {
	ctx, cancel := context.WithCancel(wp.ctx)

	worker := &Worker{
		ID:        fmt.Sprintf("worker-%d", time.Now().UnixNano()),
		pool:      wp,
		status:    WorkerStatusIdle,
		createdAt: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
		stopChan:  make(chan struct{}),
	}

	worker.wg.Add(1)
	go worker.run()

	wp.logger.Debug("created new worker", zap.String("id", worker.ID))

	return worker
}

// Submit 提交任务
func (wp *WorkerPool) Submit(task *WorkTask) error {
	wp.mu.RLock()
	closed := wp.closed
	wp.mu.RUnlock()

	if closed {
		return fmt.Errorf("worker pool is closed")
	}

	if task.ResultChan == nil {
		task.ResultChan = make(chan *TaskResult, 1)
	}

	// 根据优先级投递到对应队列
	select {
	case wp.priorityQueues[task.Priority] <- task:
		wp.logger.Debug("submitted task",
			zap.String("id", task.ID),
			zap.String("type", task.Type.String()),
			zap.Int("priority", int(task.Priority)))
		return nil
	default:
		return fmt.Errorf("task queue is full")
	}
}

// SubmitSync 同步提交任务（阻塞等待结果）
func (wp *WorkerPool) SubmitSync(ctx context.Context, task *WorkTask) (*TaskResult, error) {
	if err := wp.Submit(task); err != nil {
		return nil, err
	}

	select {
	case result := <-task.ResultChan:
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-wp.ctx.Done():
		return nil, fmt.Errorf("worker pool is closed")
	}
}

// taskDispatcher 任务分发器（按优先级处理）
func (wp *WorkerPool) taskDispatcher() {
	defer wp.wg.Done()

	for {
		select {
		case <-wp.ctx.Done():
			return

		default:
			// 按优先级从高到低检查队列
			var task *WorkTask
			for p := PriorityVIP; p >= PriorityLow; p-- {
				select {
				case task = <-wp.priorityQueues[p]:
					goto gotTask
				default:
					continue
				}
			}

			// 所有优先级队列都为空，等待
			select {
			case <-wp.ctx.Done():
				return
			case task = <-wp.priorityQueues[PriorityVIP]:
			case task = <-wp.priorityQueues[PriorityHigh]:
			case task = <-wp.priorityQueues[PriorityNormal]:
			case task = <-wp.priorityQueues[PriorityLow]:
			}

		gotTask:
			if task == nil {
				continue
			}

			// 获取可用 worker
			select {
			case worker := <-wp.available:
				worker.currentTask = task
				wp.logger.Debug("dispatched task to worker",
					zap.String("task_id", task.ID),
					zap.String("worker_id", worker.ID))
			case <-wp.ctx.Done():
				return
			}
		}
	}
}

// monitor 监控协程
func (wp *WorkerPool) monitor() {
	defer wp.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-wp.ctx.Done():
			return
		case <-ticker.C:
			wp.checkWorkers()
		}
	}
}

// checkWorkers 检查 worker 状态
func (wp *WorkerPool) checkWorkers() {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	for _, worker := range wp.workers {
		worker.mu.RLock()
		status := worker.status
		taskCount := worker.taskCount
		worker.mu.RUnlock()

		// 检测卡住的 worker
		if status == WorkerStatusBusy {
			worker.mu.RLock()
			lastTaskAt := worker.lastTaskAt
			worker.mu.RUnlock()

			if time.Since(lastTaskAt) > wp.workerConfig.TaskTimeout*2 {
				wp.logger.Warn("detected stuck worker",
					zap.String("id", worker.ID),
					zap.Duration("since_last_task", time.Since(lastTaskAt)))
			}
		}

		// 检查是否需要替换 worker
		if taskCount >= int64(wp.workerConfig.MaxTasksPerWorker) {
			wp.replaceWorker(worker)
		}
	}
}

// replaceWorker 替换 worker
func (wp *WorkerPool) replaceWorker(oldWorker *Worker) {
	wp.logger.Info("replacing worker", zap.String("id", oldWorker.ID))

	// 停止旧 worker
	oldWorker.stop()

	// 创建新 worker
	newWorker := wp.createWorker()
	if newWorker != nil {
		wp.mu.Lock()
		for i, w := range wp.workers {
			if w.ID == oldWorker.ID {
				wp.workers[i] = newWorker
				break
			}
		}
		wp.available <- newWorker
		wp.mu.Unlock()
	}
}

// Stats 获取工作池统计
func (wp *WorkerPool) Stats() map[string]interface{} {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	stats := map[string]interface{}{
		"total_workers": len(wp.workers),
		"available":     len(wp.available),
		"queue_length":  len(wp.taskQueue),
		"closed":        wp.closed,
	}

	// 按状态统计
	statusCounts := map[string]int{
		"idle":    0,
		"busy":    0,
		"stopped": 0,
	}

	var totalTasks, totalSuccess, totalFail int64
	var totalDuration time.Duration

	for _, worker := range wp.workers {
		worker.mu.RLock()
		statusCounts[worker.status.String()]++
		totalTasks += worker.taskCount
		totalSuccess += worker.successCount
		totalFail += worker.failCount
		totalDuration += worker.totalDuration
		worker.mu.RUnlock()
	}

	stats["status_distribution"] = statusCounts
	stats["total_tasks"] = totalTasks
	stats["total_success"] = totalSuccess
	stats["total_fail"] = totalFail
	if totalTasks > 0 {
		stats["avg_duration"] = totalDuration / time.Duration(totalTasks)
	} else {
		stats["avg_duration"] = 0
	}

	// 优先级队列长度
	queueLengths := make(map[string]int)
	for p, q := range wp.priorityQueues {
		queueLengths[fmt.Sprintf("priority_%d", p)] = len(q)
	}
	stats["priority_queues"] = queueLengths

	return stats
}

// ScaleUp 扩展 worker 池
func (wp *WorkerPool) ScaleUp(count int) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.closed {
		return fmt.Errorf("worker pool is closed")
	}

	added := 0
	for i := 0; i < count && int(atomic.LoadInt32(&wp.workerCount)) < wp.config.MaxInstances; i++ {
		worker := wp.createWorker()
		if worker != nil {
			wp.workers = append(wp.workers, worker)
			wp.available <- worker
			atomic.AddInt32(&wp.workerCount, 1)
			added++
		}
	}

	wp.logger.Info("scaled up worker pool",
		zap.Int("added", added),
		zap.Int32("total", atomic.LoadInt32(&wp.workerCount)))

	return nil
}

// ScaleDown 缩小 worker 池
func (wp *WorkerPool) ScaleDown(count int) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.closed {
		return fmt.Errorf("worker pool is closed")
	}

	removed := 0
	for i := 0; i < count && len(wp.workers) > wp.config.MinInstances; i++ {
		// 尝试移除空闲 worker
		select {
		case worker := <-wp.available:
			worker.stop()
			for j, w := range wp.workers {
				if w.ID == worker.ID {
					wp.workers = append(wp.workers[:j], wp.workers[j+1:]...)
					atomic.AddInt32(&wp.workerCount, -1)
					removed++
					break
				}
			}
		default:
			break
		}
	}

	wp.logger.Info("scaled down worker pool",
		zap.Int("removed", removed),
		zap.Int32("total", atomic.LoadInt32(&wp.workerCount)))

	return nil
}

// Close 关闭工作进程池
func (wp *WorkerPool) Close() error {
	wp.mu.Lock()
	if wp.closed {
		wp.mu.Unlock()
		return nil
	}
	wp.closed = true
	wp.mu.Unlock()

	wp.cancel()

	// 停止所有 worker
	wp.mu.Lock()
	for _, worker := range wp.workers {
		worker.stop()
	}
	wp.workers = nil
	wp.mu.Unlock()

	// 等待所有协程退出
	wp.wg.Wait()

	wp.logger.Info("worker pool closed")

	return nil
}

func (w *Worker) run() {
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.stopChan:
			return
		default:
			// DEPRECATED: WorkerPool 未完成，此循环不应获取 Chrome 实例。
			// 仅释放 CPU 防止空转，不做实际操作。
			runtime.Gosched()
		}
	}
}

func (w *Worker) waitForTask() {
	// 获取实例
	instance, err := w.pool.chromePool.Acquire(w.ctx)
	if err != nil {
		return
	}

	w.instance = instance
	w.mu.Lock()
	w.status = WorkerStatusIdle
	w.mu.Unlock()

	// 等待任务分配
	select {
	case <-w.ctx.Done():
		w.pool.chromePool.Release(instance)
		return
	case <-w.stopChan:
		w.pool.chromePool.Release(instance)
		return
	default:
		// 任务由 taskDispatcher 分配，这里只负责持有实例
	}
}

// executeTask 执行任务
func (w *Worker) executeTask(task *WorkTask) {
	w.mu.Lock()
	w.status = WorkerStatusBusy
	w.lastTaskAt = time.Now()
	w.mu.Unlock()

	startTime := time.Now()
	result := &TaskResult{
		TaskID: task.ID,
	}

	defer func() {
		result.Duration = time.Since(startTime)
		w.mu.Lock()
		w.taskCount++
		w.totalDuration += result.Duration
		if result.Success {
			w.successCount++
		} else {
			w.failCount++
		}
		w.status = WorkerStatusIdle
		w.currentTask = nil
		w.lastTaskAt = time.Now()
		w.mu.Unlock()

		// 释放实例
		if w.instance != nil {
			w.pool.chromePool.Release(w.instance)
			w.instance = nil
		}

		// 发送结果
		task.ResultChan <- result

		// 冷却时间
		if w.pool.workerConfig.CooldownTime > 0 {
			time.Sleep(w.pool.workerConfig.CooldownTime)
		}
	}()

	// 执行任务
	switch task.Type {
	case TaskTypeRender:
		result.HTML, result.Error = w.render(task)
	case TaskTypeScreenshot:
		result.Screenshot, result.Error = w.screenshot(task)
	case TaskTypePDF:
		result.PDF, result.Error = w.pdf(task)
	case TaskTypeMetrics:
		result.Metrics, result.Error = w.collectMetrics(task)
	default:
		result.Error = fmt.Errorf("unknown task type: %s", task.Type)
	}

	result.Success = result.Error == nil
}

// render 渲染页面
func (w *Worker) render(task *WorkTask) (string, error) {
	var html string
	actions := []chromedp.Action{
		chromedp.Navigate(task.URL),
		chromedp.WaitVisible("body", chromedp.NodeVisible),
	}

	if task.Options.WaitSelector != "" {
		actions = append(actions, chromedp.WaitVisible(task.Options.WaitSelector, chromedp.NodeVisible))
	}

	actions = append(actions, chromedp.OuterHTML("html", &html))

	if err := chromedp.Run(w.instance.ChromeCtx, actions...); err != nil {
		return "", fmt.Errorf("render failed: %w", err)
	}

	return html, nil
}

// screenshot 截取屏幕
func (w *Worker) screenshot(task *WorkTask) ([]byte, error) {
	var buf []byte
	actions := []chromedp.Action{
		chromedp.Navigate(task.URL),
		chromedp.WaitVisible("body", chromedp.NodeVisible),
	}

	if task.Options.WaitSelector != "" {
		actions = append(actions, chromedp.WaitVisible(task.Options.WaitSelector, chromedp.NodeVisible))
	}

	if task.Options.ScreenshotFull {
		actions = append(actions, chromedp.FullScreenshot(&buf, 90))
	} else {
		actions = append(actions, chromedp.FullScreenshot(&buf, 100))
	}

	if err := chromedp.Run(w.instance.ChromeCtx, actions...); err != nil {
		return nil, fmt.Errorf("screenshot failed: %w", err)
	}

	return buf, nil
}

// pdf 生成 PDF
func (w *Worker) pdf(task *WorkTask) ([]byte, error) {
	var buf []byte

	actions := []chromedp.Action{
		chromedp.Navigate(task.URL),
		chromedp.WaitVisible("body", chromedp.NodeVisible),
	}

	if task.Options.WaitSelector != "" {
		actions = append(actions, chromedp.WaitVisible(task.Options.WaitSelector, chromedp.NodeVisible))
	}

	actions = append(actions, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		buf, _, err = page.PrintToPDF().
			WithLandscape(task.Options.PDFLandscape).
			WithPrintBackground(true).
			WithPaperWidth(8.5).
			WithPaperHeight(11).
			WithMarginTop(0.5).
			WithMarginBottom(0.5).
			WithMarginLeft(0.5).
			WithMarginRight(0.5).
			Do(ctx)
		return err
	}))

	if err := chromedp.Run(w.instance.ChromeCtx, actions...); err != nil {
		return nil, fmt.Errorf("pdf generation failed: %w", err)
	}

	return buf, nil
}

// collectMetrics 收集页面性能指标
func (w *Worker) collectMetrics(task *WorkTask) (*PageMetrics, error) {
	metrics := &PageMetrics{}

	var jsResult map[string]interface{}
	err := chromedp.Run(w.instance.ChromeCtx,
		chromedp.Navigate(task.URL),
		chromedp.WaitVisible("body", chromedp.NodeVisible),
		chromedp.Evaluate(`() => {
			const timing = performance.timing;
			const entries = performance.getEntriesByType('navigation')[0];
			const paintEntries = performance.getEntriesByType('paint');

			let firstPaint = 0;
			let fcp = 0;
			for (const entry of paintEntries) {
				if (entry.name === 'first-paint') firstPaint = entry.startTime;
				if (entry.name === 'first-contentful-paint') fcp = entry.startTime;
			}

			return {
				loadTime: timing.loadEventEnd - timing.navigationStart,
				domContentLoaded: timing.domContentLoadedEventEnd - timing.navigationStart,
				firstPaint: firstPaint,
				firstContentfulPaint: fcp,
				largestContentfulPaint: entries ? entries.loadEventEnd - entries.startTime : 0
			};
		}`, &jsResult),
	)

	if err != nil {
		return nil, fmt.Errorf("collect metrics failed: %w", err)
	}

	if v, ok := jsResult["loadTime"].(float64); ok {
		metrics.LoadTime = time.Duration(v) * time.Millisecond
	}
	if v, ok := jsResult["domContentLoaded"].(float64); ok {
		metrics.DOMContentLoaded = time.Duration(v) * time.Millisecond
	}
	if v, ok := jsResult["firstPaint"].(float64); ok {
		metrics.FirstPaint = time.Duration(v) * time.Millisecond
	}
	if v, ok := jsResult["firstContentfulPaint"].(float64); ok {
		metrics.FirstContentfulPaint = time.Duration(v) * time.Millisecond
	}
	if v, ok := jsResult["largestContentfulPaint"].(float64); ok {
		metrics.LargestContentfulPaint = time.Duration(v) * time.Millisecond
	}

	return metrics, nil
}

// stop 停止 worker
func (w *Worker) stop() {
	w.mu.Lock()
	if w.status == WorkerStatusStopped {
		w.mu.Unlock()
		return
	}
	w.status = WorkerStatusStopped
	w.mu.Unlock()

	close(w.stopChan)
	w.cancel()
}
