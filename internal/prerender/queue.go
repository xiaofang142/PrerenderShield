package prerender

import (
	"container/heap"
	"strings"
	"sync"
	"time"
)

// Priority 优先级类型
type Priority int

const (
	PriorityLow    Priority = 1
	PriorityNormal Priority = 5
	PriorityHigh   Priority = 8
	PriorityVIP    Priority = 10
)

// RenderTask 渲染任务
type RenderTask struct {
	index     int           // heap 需要的索引
	ID        string        `json:"id"`
	URL       string        `json:"url"`
	SiteID    string        `json:"site_id"`
	Priority  Priority      `json:"priority"`
	CreatedAt time.Time     `json:"created_at"`
	Timeout   time.Duration `json:"timeout"`
	UserAgent string        `json:"user_agent"`
	Callback  chan<- TaskRenderResult
}

// TaskRenderResult 渲染结果（队列专用）
type TaskRenderResult struct {
	TaskID     string
	HTML       string
	Error      error
	RenderTime time.Duration
}

// RenderQueue 渲染优先级队列
type RenderQueue struct {
	mu       sync.Mutex
	tasks    PriorityQueue
	taskMap  map[string]*RenderTask
	cond     *sync.Cond
	closed   bool
	maxSize  int
}

// PriorityQueue 实现 heap.Interface
type PriorityQueue []*RenderTask

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// 优先级高的在前
	if pq[i].Priority != pq[j].Priority {
		return pq[i].Priority > pq[j].Priority
	}
	// 优先级相同，先创建的在前
	return pq[i].CreatedAt.Before(pq[j].CreatedAt)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	task := x.(*RenderTask)
	task.index = n
	*pq = append(*pq, task)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	task := old[n-1]
	old[n-1] = nil
	task.index = -1
	*pq = old[:n-1]
	return task
}

// NewRenderQueue 创建渲染队列
func NewRenderQueue(maxSize int) *RenderQueue {
	rq := &RenderQueue{
		tasks:   make(PriorityQueue, 0),
		taskMap: make(map[string]*RenderTask),
		maxSize: maxSize,
	}
	rq.cond = sync.NewCond(&rq.mu)
	heap.Init(&rq.tasks)
	return rq
}

// Enqueue 添加任务到队列
func (rq *RenderQueue) Enqueue(task *RenderTask) error {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	// 检查队列是否已满
	if len(rq.tasks) >= rq.maxSize {
		// 队列已满，移除低优先级任务
		rq.evictLowPriority()
	}

	// 添加任务
	heap.Push(&rq.tasks, task)
	rq.taskMap[task.ID] = task

	// 通知等待的消费者
	rq.cond.Signal()

	return nil
}

// Dequeue 从队列获取任务（阻塞）
func (rq *RenderQueue) Dequeue() *RenderTask {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	for len(rq.tasks) == 0 && !rq.closed {
		rq.cond.Wait()
	}

	if rq.closed && len(rq.tasks) == 0 {
		return nil
	}

	task := heap.Pop(&rq.tasks).(*RenderTask)
	delete(rq.taskMap, task.ID)

	return task
}

// DequeueNonBlocking 从队列获取任务（非阻塞）
func (rq *RenderQueue) DequeueNonBlocking() *RenderTask {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	if len(rq.tasks) == 0 {
		return nil
	}

	task := heap.Pop(&rq.tasks).(*RenderTask)
	delete(rq.taskMap, task.ID)

	return task
}

// evictLowPriority 驱逐低优先级任务
func (rq *RenderQueue) evictLowPriority() {
	// 找到最低优先级的任务
	if len(rq.tasks) == 0 {
		return
	}

	// 从队尾开始查找（优先级最低）
	for i := len(rq.tasks) - 1; i >= 0; i-- {
		task := rq.tasks[i]
		if task.Priority <= PriorityLow {
			// 移除低优先级任务
			heap.Remove(&rq.tasks, i)
			delete(rq.taskMap, task.ID)
			return
		}
	}

	// 如果没有低优先级任务，移除队尾任务
	task := heap.Pop(&rq.tasks).(*RenderTask)
	delete(rq.taskMap, task.ID)
}

// GetTask 获取指定任务
func (rq *RenderQueue) GetTask(taskID string) *RenderTask {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	return rq.taskMap[taskID]
}

// CancelTask 取消任务
func (rq *RenderQueue) CancelTask(taskID string) bool {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	task, exists := rq.taskMap[taskID]
	if !exists {
		return false
	}

	// 从堆中移除
	heap.Remove(&rq.tasks, task.index)
	delete(rq.taskMap, taskID)

	return true
}

// Close 关闭队列
func (rq *RenderQueue) Close() {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	rq.closed = true
	rq.cond.Broadcast()
}

// Len 获取队列长度
func (rq *RenderQueue) Len() int {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	return len(rq.tasks)
}

// Stats 获取队列统计
func (rq *RenderQueue) Stats() map[string]interface{} {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	stats := map[string]interface{}{
		"total_tasks": len(rq.tasks),
		"closed":      rq.closed,
		"max_size":    rq.maxSize,
	}

	// 按优先级统计
	priorityCounts := make(map[string]int)
	for _, task := range rq.tasks {
		switch task.Priority {
		case PriorityVIP:
			priorityCounts["vip"]++
		case PriorityHigh:
			priorityCounts["high"]++
		case PriorityNormal:
			priorityCounts["normal"]++
		case PriorityLow:
			priorityCounts["low"]++
		}
	}
	stats["by_priority"] = priorityCounts

	return stats
}

// PriorityOptions 优先级选项
type PriorityOptions struct {
	SiteID         string
	URL            string
	IsVIP          bool
	IsPreheat      bool
	IsUserTriggered bool
	UserAgent      string
}

// CalculatePriority 计算任务优先级
func CalculatePriority(opts PriorityOptions) Priority {
	priority := PriorityNormal

	// VIP 站点
	if opts.IsVIP {
		return PriorityVIP
	}

	// 预热任务优先级较低
	if opts.IsPreheat {
		return PriorityLow
	}

	// 用户触发的请求优先级较高
	if opts.IsUserTriggered {
		priority = PriorityHigh
	}

	// 来自搜索引擎爬虫的请求优先级高
	if isSearchEngineBot(opts.UserAgent) {
		priority = PriorityHigh
	}

	return priority
}

// isSearchEngineBot 检查是否是搜索引擎爬虫
func isSearchEngineBot(userAgent string) bool {
	uaLower := strings.ToLower(userAgent)
	bots := []string{"googlebot", "bingbot", "baiduspider", "sogouspider", "yandexbot"}
	for _, bot := range bots {
		if strings.Contains(uaLower, bot) {
			return true
		}
	}
	return false
}
