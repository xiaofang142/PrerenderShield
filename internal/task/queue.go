package task

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// redisClient 定义需要的 Redis 操作接口（用于测试）
type redisClient interface {
	Set(key string, value interface{}, expiration time.Duration) error
	SaveJSON(key string, value interface{}, expiration time.Duration) error
	Get(key string) (string, error)
	GetJSON(key string, dest interface{}) error
	ListPush(key string, values ...interface{}) error
	ListPop(key string) (string, error)
	SetAdd(key string, members ...interface{}) error
	SetRemove(key string, members ...interface{}) error
	SetMembers(key string) ([]string, error)
	Keys(pattern string) ([]string, error)
	Del(key string) error
}

// TaskType 任务类型
type TaskType string

// 任务类型常量
const (
	TaskTypePreheat TaskType = "preheat"
	TaskTypeSSL     TaskType = "ssl"
	TaskTypeCleanup TaskType = "cleanup"
	TaskTypeMonitor TaskType = "monitor"
)

// TaskStatus 任务状态
type TaskStatus string

// 任务状态常量
const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// Task 任务接口
type Task interface {
	ID() string
	Type() TaskType
	Status() TaskStatus
	Priority() int
	Execute() error
	Cancel() error
	Retry() error
}

// Queue 任务队列接口
type Queue interface {
	Enqueue(task Task) error
	Dequeue(taskType TaskType) (Task, error)
	GetTask(taskID string) (Task, error)
	UpdateTaskStatus(taskID string, status TaskStatus) error
	ListTasks(status TaskStatus) ([]Task, error)
	Cleanup() error
}

// queue 任务队列实现
type queue struct {
	redisClient redisClient
}

// NewQueue 创建新的任务队列
func NewQueue(redisClient redisClient) Queue {
	return &queue{
		redisClient: redisClient,
	}
}

// Enqueue 入队任务
func (q *queue) Enqueue(task Task) error {
	if q.redisClient == nil {
		return fmt.Errorf("redis client is nil")
	}
	// 生成任务ID
	taskID := task.ID()
	if taskID == "" {
		taskID = uuid.New().String()
		// 如果任务是 *BaseTask 类型，更新其 ID
		if baseTask, ok := task.(*BaseTask); ok {
			baseTask.IDValue = taskID
		}
	}

	// 创建任务信息
	taskInfo := map[string]interface{}{
		"id":         taskID,
		"type":       task.Type(),
		"status":     task.Status(),
		"priority":   task.Priority(),
		"created_at": time.Now().Unix(),
		"updated_at": time.Now().Unix(),
	}

	// 序列化任务
	taskData, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	// 存储任务信息到Redis
	if err := q.redisClient.SaveJSON(fmt.Sprintf("task:%s", taskID), taskInfo, 24*time.Hour); err != nil {
		return fmt.Errorf("failed to save task info: %w", err)
	}

	// 存储任务数据到Redis
	if err := q.redisClient.Set(fmt.Sprintf("task:%s:data", taskID), taskData, 24*time.Hour); err != nil {
		return fmt.Errorf("failed to save task data: %w", err)
	}

	// 将任务添加到队列
	queueKey := fmt.Sprintf("queue:%s", task.Type())
	if err := q.redisClient.ListPush(queueKey, taskID); err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	// 将任务添加到状态集合
	statusKey := fmt.Sprintf("tasks:%s", task.Status())
	if err := q.redisClient.SetAdd(statusKey, taskID); err != nil {
		return fmt.Errorf("failed to add task to status set: %w", err)
	}

	return nil
}

// Dequeue 出队任务
func (q *queue) Dequeue(taskType TaskType) (Task, error) {
	if q.redisClient == nil {
		return nil, fmt.Errorf("redis client is nil")
	}
	// 从队列中获取任务
	queueKey := fmt.Sprintf("queue:%s", taskType)
	taskID, err := q.redisClient.ListPop(queueKey)
	if err != nil {
		return nil, fmt.Errorf("failed to dequeue task: %w", err)
	}

	if taskID == "" {
		return nil, fmt.Errorf("queue is empty")
	}

	// 获取任务
	task, err := q.GetTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	// 更新任务状态为运行中
	if err := q.UpdateTaskStatus(taskID, TaskStatusRunning); err != nil {
		return nil, fmt.Errorf("failed to update task status: %w", err)
	}

	return task, nil
}

// GetTask 获取任务
func (q *queue) GetTask(taskID string) (Task, error) {
	if q.redisClient == nil {
		return nil, fmt.Errorf("redis client is nil")
	}
	// 获取任务数据
	taskData, err := q.redisClient.Get(fmt.Sprintf("task:%s:data", taskID))
	if err != nil {
		return nil, fmt.Errorf("failed to get task data: %w", err)
	}

	if taskData == "" {
		return nil, fmt.Errorf("task not found")
	}

	// 反序列化任务到 BaseTask 具体类型
	baseTask := &BaseTask{}
	if err := json.Unmarshal([]byte(taskData), baseTask); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	return baseTask, nil
}

// UpdateTaskStatus 更新任务状态
func (q *queue) UpdateTaskStatus(taskID string, status TaskStatus) error {
	if q.redisClient == nil {
		return fmt.Errorf("redis client is nil")
	}
	// 获取任务信息
	taskInfo := make(map[string]interface{})
	if err := q.redisClient.GetJSON(fmt.Sprintf("task:%s", taskID), &taskInfo); err != nil {
		return fmt.Errorf("failed to get task info: %w", err)
	}

	// 获取旧状态
	var oldStatus string
	switch s := taskInfo["status"].(type) {
	case string:
		oldStatus = s
	case TaskStatus:
		oldStatus = string(s)
	}

	// 更新任务状态
	taskInfo["status"] = status
	taskInfo["updated_at"] = time.Now().Unix()

	// 保存任务信息
	if err := q.redisClient.SaveJSON(fmt.Sprintf("task:%s", taskID), taskInfo, 24*time.Hour); err != nil {
		return fmt.Errorf("failed to save task info: %w", err)
	}

	// 从旧状态集合中移除
	if oldStatus != "" {
		oldStatusKey := fmt.Sprintf("tasks:%s", oldStatus)
		q.redisClient.SetRemove(oldStatusKey, taskID)
	}

	// 添加到新状态集合
	newStatusKey := fmt.Sprintf("tasks:%s", status)
	if err := q.redisClient.SetAdd(newStatusKey, taskID); err != nil {
		return fmt.Errorf("failed to add task to status set: %w", err)
	}

	return nil
}

// ListTasks 列出任务
func (q *queue) ListTasks(status TaskStatus) ([]Task, error) {
	if q.redisClient == nil {
		return nil, fmt.Errorf("redis client is nil")
	}
	// 获取状态集合中的任务ID
	statusKey := fmt.Sprintf("tasks:%s", status)
	taskIDs, err := q.redisClient.SetMembers(statusKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get task IDs: %w", err)
	}

	// 获取每个任务
	tasks := []Task{}
	for _, taskID := range taskIDs {
		task, err := q.GetTask(taskID)
		if err != nil {
			continue
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// Cleanup 清理任务
func (q *queue) Cleanup() error {
	if q.redisClient == nil {
		return fmt.Errorf("redis client is nil")
	}
	// 获取所有任务
	keys, err := q.redisClient.Keys("task:*")
	if err != nil {
		return fmt.Errorf("failed to get task keys: %w", err)
	}

	// 清理24小时前创建的已完成或失败的任务
	now := time.Now().Unix()
	for _, key := range keys {
		// 跳过任务数据键
		if strings.Contains(key, ":data") {
			continue
		}

		taskInfo := make(map[string]interface{})
		if err := q.redisClient.GetJSON(key, &taskInfo); err != nil {
			continue
		}

		createdAt, ok := taskInfo["created_at"].(float64)
		if !ok {
			continue
		}

		status, _ := taskInfo["status"].(string)
		if (status == string(TaskStatusCompleted) || status == string(TaskStatusFailed) || status == string(TaskStatusCancelled)) && now-int64(createdAt) > 24*3600 {
			// 删除任务信息
			taskID := strings.TrimPrefix(key, "task:")
			q.redisClient.Del(key)
			q.redisClient.Del(fmt.Sprintf("task:%s:data", taskID))

			// 从状态集合中移除
			statusKey := fmt.Sprintf("tasks:%s", status)
			q.redisClient.SetRemove(statusKey, taskID)
		}
	}

	return nil
}

// BaseTask 基础任务结构
type BaseTask struct {
	IDValue       string     `json:"id"`
	TypeValue     TaskType   `json:"type"`
	StatusValue   TaskStatus `json:"status"`
	PriorityValue int        `json:"priority"`
	CreatedAt     int64      `json:"created_at"`
	UpdatedAt     int64      `json:"updated_at"`
	Retries       int        `json:"retries"`
	MaxRetries    int        `json:"max_retries"`
}

// NewBaseTask 创建新的基础任务
func NewBaseTask(taskType TaskType) *BaseTask {
	return &BaseTask{
		IDValue:       uuid.New().String(),
		TypeValue:     taskType,
		StatusValue:   TaskStatusPending,
		PriorityValue: 0,
		CreatedAt:     time.Now().Unix(),
		UpdatedAt:     time.Now().Unix(),
		Retries:       0,
		MaxRetries:    3,
	}
}

// ID 获取任务ID
func (t *BaseTask) ID() string {
	return t.IDValue
}

// Type 获取任务类型
func (t *BaseTask) Type() TaskType {
	return t.TypeValue
}

// Status 获取任务状态
func (t *BaseTask) Status() TaskStatus {
	return t.StatusValue
}

// Priority 获取任务优先级
func (t *BaseTask) Priority() int {
	return t.PriorityValue
}

// Execute 执行任务
func (t *BaseTask) Execute() error {
	return fmt.Errorf("not implemented")
}

// Cancel 取消任务
func (t *BaseTask) Cancel() error {
	t.StatusValue = TaskStatusCancelled
	t.UpdatedAt = time.Now().Unix()
	return nil
}

// Retry 重试任务
func (t *BaseTask) Retry() error {
	if t.Retries >= t.MaxRetries {
		return fmt.Errorf("max retries reached")
	}

	t.Retries++
	t.StatusValue = TaskStatusPending
	t.UpdatedAt = time.Now().Unix()
	return nil
}
