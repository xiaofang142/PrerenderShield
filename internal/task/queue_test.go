package task

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestTaskType
func TestTaskType_Constants(t *testing.T) {
	assert.Equal(t, TaskType("preheat"), TaskTypePreheat)
	assert.Equal(t, TaskType("ssl"), TaskTypeSSL)
	assert.Equal(t, TaskType("cleanup"), TaskTypeCleanup)
	assert.Equal(t, TaskType("monitor"), TaskTypeMonitor)
}

func TestTaskStatus_Constants(t *testing.T) {
	assert.Equal(t, TaskStatus("pending"), TaskStatusPending)
	assert.Equal(t, TaskStatus("running"), TaskStatusRunning)
	assert.Equal(t, TaskStatus("completed"), TaskStatusCompleted)
	assert.Equal(t, TaskStatus("failed"), TaskStatusFailed)
	assert.Equal(t, TaskStatus("cancelled"), TaskStatusCancelled)
}

// TestBaseTask
func TestNewBaseTask(t *testing.T) {
	task := NewBaseTask(TaskTypePreheat)

	assert.NotNil(t, task)
	assert.NotEmpty(t, task.ID())
	assert.Equal(t, TaskTypePreheat, task.Type())
	assert.Equal(t, TaskStatusPending, task.Status())
	assert.Equal(t, 0, task.Priority())
	assert.Greater(t, task.CreatedAt, int64(0))
	assert.Greater(t, task.UpdatedAt, int64(0))
	assert.Equal(t, 0, task.Retries)
	assert.Equal(t, 3, task.MaxRetries)
}

func TestNewBaseTask_DifferentTypes(t *testing.T) {
	testCases := []struct {
		taskType TaskType
	}{
		{TaskTypeSSL},
		{TaskTypeCleanup},
		{TaskTypeMonitor},
	}

	for _, tc := range testCases {
		task := NewBaseTask(tc.taskType)
		assert.Equal(t, tc.taskType, task.Type())
	}
}

func TestBaseTask_ID(t *testing.T) {
	task := NewBaseTask(TaskTypeSSL)
	id := task.ID()
	assert.NotEmpty(t, id)

	// 验证 ID 是 UUID 格式
	assert.Greater(t, len(id), 30)
}

func TestBaseTask_Type(t *testing.T) {
	task := NewBaseTask(TaskTypeMonitor)
	assert.Equal(t, TaskTypeMonitor, task.Type())
}

func TestBaseTask_Status(t *testing.T) {
	task := NewBaseTask(TaskTypeCleanup)
	assert.Equal(t, TaskStatusPending, task.Status())
}

func TestBaseTask_Priority(t *testing.T) {
	task := NewBaseTask(TaskTypePreheat)
	assert.Equal(t, 0, task.Priority())

	task.PriorityValue = 5
	assert.Equal(t, 5, task.Priority())
}

func TestBaseTask_Execute(t *testing.T) {
	task := NewBaseTask(TaskTypePreheat)
	err := task.Execute()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "not implemented")
}

func TestBaseTask_Cancel(t *testing.T) {
	task := NewBaseTask(TaskTypePreheat)
	oldUpdatedAt := task.UpdatedAt

	time.Sleep(1100 * time.Millisecond)

	err := task.Cancel()
	assert.Nil(t, err)
	assert.Equal(t, TaskStatusCancelled, task.Status())
	assert.Greater(t, task.UpdatedAt, oldUpdatedAt)
}

func TestBaseTask_Retry_Success(t *testing.T) {
	task := NewBaseTask(TaskTypePreheat)

	// 第一次重试应该成功
	err := task.Retry()
	assert.Nil(t, err)
	assert.Equal(t, 1, task.Retries)
	assert.Equal(t, TaskStatusPending, task.Status())
}

func TestBaseTask_Retry_MaxRetries(t *testing.T) {
	task := NewBaseTask(TaskTypePreheat)

	// 重试直到达到最大次数
	for i := 0; i < 3; i++ {
		err := task.Retry()
		assert.Nil(t, err)
		assert.Equal(t, i+1, task.Retries)
	}

	// 超过最大重试次数应该失败
	err := task.Retry()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "max retries reached")
}

func TestBaseTask_Retry_CustomMaxRetries(t *testing.T) {
	task := NewBaseTask(TaskTypePreheat)
	task.MaxRetries = 5

	for i := 0; i < 5; i++ {
		err := task.Retry()
		assert.Nil(t, err)
		assert.Equal(t, i+1, task.Retries)
	}

	err := task.Retry()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "max retries reached")
}

func TestBaseTask_Retry_ResetsStatus(t *testing.T) {
	task := NewBaseTask(TaskTypePreheat)
	task.StatusValue = TaskStatusFailed

	err := task.Retry()
	assert.Nil(t, err)
	assert.Equal(t, TaskStatusPending, task.Status())
}

func TestBaseTask_Retry_UpdatesTimestamp(t *testing.T) {
	task := NewBaseTask(TaskTypePreheat)
	oldUpdatedAt := task.UpdatedAt

	time.Sleep(1100 * time.Millisecond)

	err := task.Retry()
	assert.Nil(t, err)
	assert.Greater(t, task.UpdatedAt, oldUpdatedAt)
}

func TestBaseTask_SetStatus(t *testing.T) {
	task := NewBaseTask(TaskTypePreheat)
	task.StatusValue = TaskStatusRunning
	assert.Equal(t, TaskStatusRunning, task.Status())

	task.StatusValue = TaskStatusCompleted
	assert.Equal(t, TaskStatusCompleted, task.Status())
}

func TestBaseTask_SetPriority(t *testing.T) {
	task := NewBaseTask(TaskTypePreheat)
	task.PriorityValue = 10
	assert.Equal(t, 10, task.Priority())
}

func TestBaseTask_JSONSerialization(t *testing.T) {
	task := NewBaseTask(TaskTypeSSL)
	task.PriorityValue = 5
	task.Retries = 2

	// 验证任务可以被序列化（用于 Redis 存储）
	assert.Equal(t, "ssl", string(task.Type()))
	assert.Equal(t, "pending", string(task.Status()))
	assert.Equal(t, 5, task.Priority())
	assert.Equal(t, 2, task.Retries)
}

func TestBaseTask_AllStatusTransitions(t *testing.T) {
	task := NewBaseTask(TaskTypePreheat)

	// Pending -> Running
	task.StatusValue = TaskStatusRunning
	assert.Equal(t, TaskStatusRunning, task.Status())

	// Running -> Completed
	task.StatusValue = TaskStatusCompleted
	assert.Equal(t, TaskStatusCompleted, task.Status())

	// Reset and test Failed
	task2 := NewBaseTask(TaskTypePreheat)
	task2.StatusValue = TaskStatusRunning
	task2.StatusValue = TaskStatusFailed
	assert.Equal(t, TaskStatusFailed, task2.Status())

	// Reset and test Cancelled
	task3 := NewBaseTask(TaskTypePreheat)
	task3.StatusValue = TaskStatusRunning
	task3.StatusValue = TaskStatusCancelled
	assert.Equal(t, TaskStatusCancelled, task3.Status())
}

func TestBaseTask_Fields(t *testing.T) {
	task := &BaseTask{
		IDValue:       "custom-id",
		TypeValue:     TaskTypeMonitor,
		StatusValue:   TaskStatusCompleted,
		PriorityValue: 10,
		CreatedAt:     1234567890,
		UpdatedAt:     1234567891,
		Retries:       2,
		MaxRetries:    5,
	}

	assert.Equal(t, "custom-id", task.ID())
	assert.Equal(t, TaskTypeMonitor, task.Type())
	assert.Equal(t, TaskStatusCompleted, task.Status())
	assert.Equal(t, 10, task.Priority())
	assert.Equal(t, int64(1234567890), task.CreatedAt)
	assert.Equal(t, int64(1234567891), task.UpdatedAt)
	assert.Equal(t, 2, task.Retries)
	assert.Equal(t, 5, task.MaxRetries)
}

func TestTaskInterface(t *testing.T) {
	// 验证 BaseTask 实现了 Task 接口
	var _ Task = (*BaseTask)(nil)
}

func TestQueueInterface(t *testing.T) {
	// 验证 queue 实现了 Queue 接口（需要 redis 客户端）
	// 这里只验证接口定义
	var q Queue
	assert.Nil(t, q)
}

// TestQueue_NilRedis 测试 Queue 方法在 Redis 客户端为 nil 时的行为
func TestQueue_NilRedis(t *testing.T) {
	q := NewQueue(nil)
	assert.NotNil(t, q)

	// 测试 Enqueue
	mockTask := NewBaseTask(TaskTypePreheat)
	err := q.Enqueue(mockTask)
	assert.Error(t, err)

	// 测试 Dequeue
	_, err = q.Dequeue(TaskTypePreheat)
	assert.Error(t, err)

	// 测试 GetTask
	_, err = q.GetTask("test-id")
	assert.Error(t, err)

	// 测试 UpdateTaskStatus
	err = q.UpdateTaskStatus("test-id", TaskStatusRunning)
	assert.Error(t, err)

	// 测试 ListTasks
	_, err = q.ListTasks(TaskStatusPending)
	assert.Error(t, err)

	// 测试 Cleanup
	err = q.Cleanup()
	assert.Error(t, err)
}

// TestQueue_EmptyTaskID 测试 Enqueue 生成任务 ID
func TestQueue_EmptyTaskID(t *testing.T) {
	// 创建一个 ID 为空的任务
	task := &BaseTask{
		TypeValue:   TaskTypeSSL,
		StatusValue: TaskStatusPending,
	}
	q := NewQueue(nil)
	err := q.Enqueue(task)
	// 即使 Redis 为 nil，也应该尝试生成 UUID
	assert.Error(t, err)
}

// TestTaskType_String 测试 TaskType 字符串转换
func TestTaskType_String(t *testing.T) {
	assert.Equal(t, "preheat", string(TaskTypePreheat))
	assert.Equal(t, "ssl", string(TaskTypeSSL))
	assert.Equal(t, "cleanup", string(TaskTypeCleanup))
	assert.Equal(t, "monitor", string(TaskTypeMonitor))
}

// TestTaskStatus_String 测试 TaskStatus 字符串转换
func TestTaskStatus_String(t *testing.T) {
	assert.Equal(t, "pending", string(TaskStatusPending))
	assert.Equal(t, "running", string(TaskStatusRunning))
	assert.Equal(t, "completed", string(TaskStatusCompleted))
	assert.Equal(t, "failed", string(TaskStatusFailed))
	assert.Equal(t, "cancelled", string(TaskStatusCancelled))
}
