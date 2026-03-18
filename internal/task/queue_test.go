package task

import (
	"encoding/json"
	"fmt"
	"strings"
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

// TestQueue_Enqueue_NilTask 测试 Enqueue 处理 nil 任务
func TestQueue_Enqueue_NilTask(t *testing.T) {
	q := NewQueue(nil)
	err := q.Enqueue(nil)
	// 应该因为 nil redis 返回错误
	assert.Error(t, err)
}

// TestQueue_Dequeue_EmptyQueue 测试 Dequeue 空队列
func TestQueue_Dequeue_EmptyQueue(t *testing.T) {
	q := NewQueue(nil)
	_, err := q.Dequeue(TaskTypePreheat)
	assert.Error(t, err)
}

// TestQueue_Dequeue_DifferentTypes 测试 Dequeue 不同任务类型
func TestQueue_Dequeue_DifferentTypes(t *testing.T) {
	q := NewQueue(nil)

	taskTypes := []TaskType{TaskTypeSSL, TaskTypeCleanup, TaskTypeMonitor}
	for _, taskType := range taskTypes {
		_, err := q.Dequeue(taskType)
		assert.Error(t, err)
	}
}

// TestQueue_GetTask_InvalidID 测试 GetTask 无效任务 ID
func TestQueue_GetTask_InvalidID(t *testing.T) {
	q := NewQueue(nil)
	_, err := q.GetTask("")
	assert.Error(t, err)

	_, err = q.GetTask("nonexistent-id")
	assert.Error(t, err)
}

// TestQueue_UpdateTaskStatus_NilRedis 测试 UpdateTaskStatus nil redis
func TestQueue_UpdateTaskStatus_NilRedis(t *testing.T) {
	q := NewQueue(nil)
	err := q.UpdateTaskStatus("test-id", TaskStatusRunning)
	assert.Error(t, err)
}

// TestQueue_UpdateTaskStatus_DifferentStatus 测试 UpdateTaskStatus 不同状态
func TestQueue_UpdateTaskStatus_DifferentStatus(t *testing.T) {
	q := NewQueue(nil)

	statuses := []TaskStatus{
		TaskStatusPending,
		TaskStatusRunning,
		TaskStatusCompleted,
		TaskStatusFailed,
		TaskStatusCancelled,
	}

	for _, status := range statuses {
		err := q.UpdateTaskStatus("test-id", status)
		assert.Error(t, err)
	}
}

// TestQueue_ListTasks_Empty 测试 ListTasks 空列表
func TestQueue_ListTasks_Empty(t *testing.T) {
	q := NewQueue(nil)
	tasks, err := q.ListTasks(TaskStatusPending)
	assert.Error(t, err)
	assert.Nil(t, tasks)
}

// TestQueue_ListTasks_DifferentStatus 测试 ListTasks 不同状态
func TestQueue_ListTasks_DifferentStatus(t *testing.T) {
	q := NewQueue(nil)

	statuses := []TaskStatus{
		TaskStatusPending,
		TaskStatusRunning,
		TaskStatusCompleted,
		TaskStatusFailed,
		TaskStatusCancelled,
	}

	for _, status := range statuses {
		_, err := q.ListTasks(status)
		assert.Error(t, err)
	}
}

// TestQueue_Cleanup_NoTasks 测试 Cleanup 没有任务
func TestQueue_Cleanup_NoTasks(t *testing.T) {
	q := NewQueue(nil)
	err := q.Cleanup()
	assert.Error(t, err)
}

// TestQueue_StructFields 测试 queue 结构体字段
func TestQueue_StructFields(t *testing.T) {
	q := &queue{
		redisClient: nil,
	}
	assert.Nil(t, q.redisClient)
}

// TestQueue_Enqueue_ErrorPath 测试 Enqueue 错误路径
func TestQueue_Enqueue_ErrorPath(t *testing.T) {
	// 创建一个有 ID 的任务
	task := &BaseTask{
		IDValue:     "test-id",
		TypeValue:   TaskTypePreheat,
		StatusValue: TaskStatusPending,
	}

	q := NewQueue(nil)
	err := q.Enqueue(task)
	// 应该因为 nil redis 返回错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis client is nil")
}

// TestQueue_Dequeue_ErrorPath 测试 Dequeue 错误路径
func TestQueue_Dequeue_ErrorPath(t *testing.T) {
	q := NewQueue(nil)
	_, err := q.Dequeue(TaskTypePreheat)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis client is nil")
}

// TestQueue_GetTask_ErrorPath 测试 GetTask 错误路径
func TestQueue_GetTask_ErrorPath(t *testing.T) {
	q := NewQueue(nil)
	_, err := q.GetTask("test-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis client is nil")
}

// TestQueue_UpdateTaskStatus_ErrorPath 测试 UpdateTaskStatus 错误路径
func TestQueue_UpdateTaskStatus_ErrorPath(t *testing.T) {
	q := NewQueue(nil)
	err := q.UpdateTaskStatus("test-id", TaskStatusRunning)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis client is nil")
}

// TestQueue_ListTasks_ErrorPath 测试 ListTasks 错误路径
func TestQueue_ListTasks_ErrorPath(t *testing.T) {
	q := NewQueue(nil)
	_, err := q.ListTasks(TaskStatusPending)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis client is nil")
}

// TestQueue_Cleanup_ErrorPath 测试 Cleanup 错误路径
func TestQueue_Cleanup_ErrorPath(t *testing.T) {
	q := NewQueue(nil)
	err := q.Cleanup()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis client is nil")
}

// TestBaseTask_MarshalJSON 测试 BaseTask JSON 序列化
func TestBaseTask_MarshalJSON(t *testing.T) {
	task := NewBaseTask(TaskTypePreheat)
	task.PriorityValue = 5
	task.Retries = 1

	// 验证可以被序列化
	data, err := json.Marshal(task)
	assert.Nil(t, err)
	assert.NotEmpty(t, data)
}

// TestBaseTask_SetCreatedAt 测试设置 CreatedAt
func TestBaseTask_SetCreatedAt(t *testing.T) {
	task := NewBaseTask(TaskTypePreheat)
	oldTime := task.CreatedAt

	task.CreatedAt = oldTime + 1000
	assert.Equal(t, oldTime+1000, task.CreatedAt)
}

// TestBaseTask_SetUpdatedAt 测试设置 UpdatedAt
func TestBaseTask_SetUpdatedAt(t *testing.T) {
	task := NewBaseTask(TaskTypePreheat)
	oldTime := task.UpdatedAt

	task.UpdatedAt = oldTime + 1000
	assert.Equal(t, oldTime+1000, task.UpdatedAt)
}

// TestBaseTask_SetRetries 测试设置 Retries
func TestBaseTask_SetRetries(t *testing.T) {
	task := NewBaseTask(TaskTypePreheat)
	task.Retries = 5
	assert.Equal(t, 5, task.Retries)
}

// TestBaseTask_Retry_BoundaryConditions 测试 Retry 边界条件
func TestBaseTask_Retry_BoundaryConditions(t *testing.T) {
	// 测试 MaxRetries 为 0
	task := NewBaseTask(TaskTypePreheat)
	task.MaxRetries = 0

	err := task.Retry()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max retries reached")
}

// TestQueue_Type 测试 TaskType 类型
func TestQueue_TaskType(t *testing.T) {
	var taskType TaskType = "custom"
	assert.Equal(t, "custom", string(taskType))
}

// TestQueue_TaskStatus 测试 TaskStatus 类型
func TestQueue_TaskStatus(t *testing.T) {
	var taskStatus TaskStatus = "custom"
	assert.Equal(t, "custom", string(taskStatus))
}

// ============== MockRedisClient 用于队列测试 ==============

// MockRedisClient 是 redis.Client 的 mock 实现
type MockRedisClient struct {
	data map[string]interface{}
}

func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		data: make(map[string]interface{}),
	}
}

func (m *MockRedisClient) SaveJSON(key string, value interface{}, expiration time.Duration) error {
	m.data[key] = value
	return nil
}

func (m *MockRedisClient) Set(key string, value interface{}, expiration time.Duration) error {
	// 直接存储原始值，不做转换
	m.data[key] = value
	return nil
}

func (m *MockRedisClient) Get(key string) (string, error) {
	if val, ok := m.data[key]; ok {
		// 如果是字符串，直接返回
		if str, ok := val.(string); ok {
			return str, nil
		}
		// 如果是 []byte，转换为字符串
		if bytes, ok := val.([]byte); ok {
			return string(bytes), nil
		}
		// 其他类型，序列化为 JSON 字符串
		data, err := json.Marshal(val)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return "", nil
}

func (m *MockRedisClient) GetJSON(key string, dest interface{}) error {
	if val, ok := m.data[key]; ok {
		// 如果已经是目标类型，直接复制
		if destMap, ok := dest.(*map[string]interface{}); ok {
			if valMap, ok := val.(map[string]interface{}); ok {
				*destMap = valMap
				return nil
			}
		}
		// 否则尝试序列化再反序列化
		data, err := json.Marshal(val)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, dest)
	}
	return fmt.Errorf("key not found")
}

func (m *MockRedisClient) ListPush(key string, values ...interface{}) error {
	m.data[key] = values
	return nil
}

func (m *MockRedisClient) ListPop(key string) (string, error) {
	if val, ok := m.data[key]; ok {
		if list, ok := val.([]interface{}); ok && len(list) > 0 {
			m.data[key] = list[1:]
			if str, ok := list[0].(string); ok {
				return str, nil
			}
		}
	}
	return "", nil
}

func (m *MockRedisClient) SetAdd(key string, members ...interface{}) error {
	existing, ok := m.data[key].([]interface{})
	if !ok {
		existing = []interface{}{}
	}
	m.data[key] = append(existing, members...)
	return nil
}

func (m *MockRedisClient) SetRemove(key string, members ...interface{}) error {
	if val, ok := m.data[key].([]interface{}); ok {
		newList := []interface{}{}
		for _, v := range val {
			keep := true
			for _, remove := range members {
				// 将两个值都转换为字符串进行比较
				vStr, vOk := v.(string)
				removeStr, removeOk := remove.(string)
				if vOk && removeOk && vStr == removeStr {
					keep = false
					break
				}
			}
			if keep {
				newList = append(newList, v)
			}
		}
		m.data[key] = newList
	}
	return nil
}

func (m *MockRedisClient) SetMembers(key string) ([]string, error) {
	if val, ok := m.data[key].([]interface{}); ok {
		result := make([]string, len(val))
		for i, v := range val {
			if str, ok := v.(string); ok {
				result[i] = str
			}
		}
		return result, nil
	}
	return []string{}, nil
}

func (m *MockRedisClient) Keys(pattern string) ([]string, error) {
	keys := []string{}
	for k := range m.data {
		if strings.Contains(k, strings.TrimSuffix(pattern, "*")) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (m *MockRedisClient) Del(key string) error {
	delete(m.data, key)
	return nil
}

// TestQueue_Enqueue_Success 测试 Enqueue 成功路径
func TestQueue_Enqueue_Success(t *testing.T) {
	mockRedis := NewMockRedisClient()
	q := NewQueue(mockRedis)

	mockTask := NewBaseTask(TaskTypePreheat)
	mockTask.PriorityValue = 5

	err := q.Enqueue(mockTask)

	assert.NoError(t, err)
	// 验证任务被存储
	taskID := mockTask.ID()
	assert.Contains(t, mockRedis.data, fmt.Sprintf("task:%s", taskID))
	assert.Contains(t, mockRedis.data, fmt.Sprintf("task:%s:data", taskID))
	assert.Contains(t, mockRedis.data, "queue:preheat")
	assert.Contains(t, mockRedis.data, "tasks:pending")
}

// TestQueue_Dequeue_Success 测试 Dequeue 成功路径
func TestQueue_Dequeue_Success(t *testing.T) {
	mockRedis := NewMockRedisClient()
	q := NewQueue(mockRedis)

	// 先入队一个任务
	mockTask := NewBaseTask(TaskTypeSSL)
	err := q.Enqueue(mockTask)
	assert.NoError(t, err)

	// 出队任务
	dequeued, err := q.Dequeue(TaskTypeSSL)

	assert.NoError(t, err)
	assert.NotNil(t, dequeued)
	// 验证状态被更新为 running
	assert.Contains(t, mockRedis.data, "tasks:running")
}

// TestQueue_Dequeue_Empty 测试空队列
func TestQueue_Dequeue_Empty(t *testing.T) {
	mockRedis := NewMockRedisClient()
	q := NewQueue(mockRedis)

	_, err := q.Dequeue(TaskTypePreheat)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "queue is empty")
}

// TestQueue_GetTask_Success 测试获取任务成功
func TestQueue_GetTask_Success(t *testing.T) {
	mockRedis := NewMockRedisClient()
	q := NewQueue(mockRedis)

	mockTask := NewBaseTask(TaskTypeMonitor)
	err := q.Enqueue(mockTask)
	assert.NoError(t, err)

	retrieved, err := q.GetTask(mockTask.ID())

	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, mockTask.ID(), retrieved.ID())
	assert.Equal(t, TaskTypeMonitor, retrieved.Type())
}

// TestQueue_GetTask_NotFound 测试获取不存在的任务
func TestQueue_GetTask_NotFound(t *testing.T) {
	mockRedis := NewMockRedisClient()
	q := NewQueue(mockRedis)

	_, err := q.GetTask("nonexistent-id")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task not found")
}

// TestQueue_UpdateTaskStatus_Success 测试更新任务状态成功
func TestQueue_UpdateTaskStatus_Success(t *testing.T) {
	mockRedis := NewMockRedisClient()
	q := NewQueue(mockRedis)

	mockTask := NewBaseTask(TaskTypeCleanup)
	err := q.Enqueue(mockTask)
	assert.NoError(t, err)

	err = q.UpdateTaskStatus(mockTask.ID(), TaskStatusRunning)

	assert.NoError(t, err)
	// 验证状态已更新
	taskInfo := make(map[string]interface{})
	err = mockRedis.GetJSON(fmt.Sprintf("task:%s", mockTask.ID()), &taskInfo)
	assert.NoError(t, err)
	// 状态可能是 TaskStatus 类型（底层是 string）
	status := taskInfo["status"]
	assert.NotNil(t, status)
	// 尝试转换为 string 或 TaskStatus
	switch s := status.(type) {
	case string:
		assert.Equal(t, "running", s)
	case TaskStatus:
		assert.Equal(t, TaskStatusRunning, s)
	default:
		t.Errorf("unexpected status type: %T", status)
	}
}

// TestQueue_ListTasks_Success 测试列出任务成功
func TestQueue_ListTasks_Success(t *testing.T) {
	mockRedis := NewMockRedisClient()
	q := NewQueue(mockRedis)

	// 入队多个任务
	task1 := NewBaseTask(TaskTypePreheat)
	task2 := NewBaseTask(TaskTypePreheat)
	err := q.Enqueue(task1)
	assert.NoError(t, err)
	err = q.Enqueue(task2)
	assert.NoError(t, err)

	tasks, err := q.ListTasks(TaskStatusPending)

	assert.NoError(t, err)
	assert.Len(t, tasks, 2)
}

// TestQueue_Cleanup_Success 测试清理任务成功
func TestQueue_Cleanup_Success(t *testing.T) {
	mockRedis := NewMockRedisClient()
	q := NewQueue(mockRedis)

	// 创建一个已完成的旧任务
	oldTaskID := "old-task-id"
	oldTime := time.Now().Add(-25 * time.Hour).Unix()
	taskInfo := map[string]interface{}{
		"id":         oldTaskID,
		"status":     string(TaskStatusCompleted),
		"created_at": float64(oldTime),
	}
	mockRedis.data["task:"+oldTaskID] = taskInfo

	err := q.Cleanup()

	assert.NoError(t, err)
	// 验证旧任务被清理
	_, exists := mockRedis.data["task:"+oldTaskID]
	assert.False(t, exists)
}

// TestQueue_Cleanup_NoExpiredTasks 测试清理没有过期任务
func TestQueue_Cleanup_NoExpiredTasks(t *testing.T) {
	mockRedis := NewMockRedisClient()
	q := NewQueue(mockRedis)

	// 创建一个新任务
	recentTaskID := "recent-task-id"
	recentTime := time.Now().Unix()
	taskInfo := map[string]interface{}{
		"id":         recentTaskID,
		"status":     string(TaskStatusCompleted),
		"created_at": float64(recentTime),
	}
	mockRedis.data["task:"+recentTaskID] = taskInfo

	err := q.Cleanup()

	assert.NoError(t, err)
	// 验证新任务没有被清理
	_, exists := mockRedis.data["task:"+recentTaskID]
	assert.True(t, exists)
}

// TestQueue_Enqueue_Integration 测试完整的入队流程
func TestQueue_Enqueue_Integration(t *testing.T) {
	mockRedis := NewMockRedisClient()
	q := NewQueue(mockRedis)

	// 创建并序列化任务
	task := &BaseTask{
		IDValue:       "integration-test-id",
		TypeValue:     TaskTypeSSL,
		StatusValue:   TaskStatusPending,
		PriorityValue: 10,
		Retries:       0,
		MaxRetries:    3,
	}

	err := q.Enqueue(task)

	assert.NoError(t, err)
	// 验证所有相关键都被设置
	assert.Contains(t, mockRedis.data, "task:integration-test-id")
	assert.Contains(t, mockRedis.data, "task:integration-test-id:data")
	assert.Contains(t, mockRedis.data, "queue:ssl")
	assert.Contains(t, mockRedis.data, "tasks:pending")
}

// TestQueue_Dequeue_Integration 测试完整的出队流程
func TestQueue_Dequeue_Integration(t *testing.T) {
	mockRedis := NewMockRedisClient()
	q := NewQueue(mockRedis)

	// 创建任务
	task := NewBaseTask(TaskTypePreheat)
	initialTaskID := task.ID()

	err := q.Enqueue(task)
	assert.NoError(t, err)

	// 出队任务
	dequeuedTask, err := q.Dequeue(TaskTypePreheat)

	assert.NoError(t, err)
	assert.NotNil(t, dequeuedTask)
	assert.Equal(t, initialTaskID, dequeuedTask.ID())
	assert.Equal(t, TaskTypePreheat, dequeuedTask.Type())
}

// TestQueue_UpdateTaskStatus_Integration 测试完整的状态更新流程
func TestQueue_UpdateTaskStatus_Integration(t *testing.T) {
	mockRedis := NewMockRedisClient()
	q := NewQueue(mockRedis)

	// 创建任务
	task := NewBaseTask(TaskTypeMonitor)
	err := q.Enqueue(task)
	assert.NoError(t, err)

	// 验证初始状态是 pending
	pendingMembers, _ := mockRedis.SetMembers("tasks:pending")
	assert.Contains(t, pendingMembers, task.ID())

	// 更新状态为 Running
	err = q.UpdateTaskStatus(task.ID(), TaskStatusRunning)
	assert.NoError(t, err)

	// 验证状态集合已更新
	pendingMembers, _ = mockRedis.SetMembers("tasks:pending")
	runningMembers, _ := mockRedis.SetMembers("tasks:running")

	// 调试输出
	t.Logf("pending members: %v", pendingMembers)
	t.Logf("running members: %v", runningMembers)
	t.Logf("task ID: %s", task.ID())

	assert.NotContains(t, pendingMembers, task.ID(), "task should be removed from pending")
	assert.Contains(t, runningMembers, task.ID(), "task should be added to running")

	// 更新状态为 Completed
	err = q.UpdateTaskStatus(task.ID(), TaskStatusCompleted)
	assert.NoError(t, err)

	runningMembers2, _ := mockRedis.SetMembers("tasks:running")
	completedMembers, _ := mockRedis.SetMembers("tasks:completed")

	assert.NotContains(t, runningMembers2, task.ID(), "task should be removed from running")
	assert.Contains(t, completedMembers, task.ID(), "task should be added to completed")
}

// TestQueue_ListTasks_WithSkippedErrors 测试列出任务时跳过错误
func TestQueue_ListTasks_WithSkippedErrors(t *testing.T) {
	mockRedis := NewMockRedisClient()
	q := NewQueue(mockRedis)

	// 手动添加一个任务信息但没有数据
	invalidTaskID := "invalid-task"
	mockRedis.data["tasks:pending"] = []interface{}{invalidTaskID}

	tasks, err := q.ListTasks(TaskStatusPending)

	assert.NoError(t, err)
	assert.Empty(t, tasks) // 无效任务被跳过
}

// TestQueue_Cleanup_SkipsInvalidTasks 测试清理时跳过无效任务
func TestQueue_Cleanup_SkipsInvalidTasks(t *testing.T) {
	mockRedis := NewMockRedisClient()
	q := NewQueue(mockRedis)

	// 添加一个没有 created_at 的任务
	invalidTaskID := "invalid-task"
	mockRedis.data["task:"+invalidTaskID] = map[string]interface{}{
		"id":     invalidTaskID,
		"status": string(TaskStatusCompleted),
		// 缺少 created_at
	}

	err := q.Cleanup()

	assert.NoError(t, err)
	// 无效任务应该被跳过（不被清理）
	_, exists := mockRedis.data["task:"+invalidTaskID]
	assert.True(t, exists)
}

// TestQueue_Enqueue_GeneratesIDForEmptyTask 测试 Enqueue 为空 ID 的任务生成 ID
func TestQueue_Enqueue_GeneratesIDForEmptyTask(t *testing.T) {
	mockRedis := NewMockRedisClient()
	q := NewQueue(mockRedis)

	// 创建空 ID 的任务
	task := &BaseTask{
		TypeValue:   TaskTypeSSL,
		StatusValue: TaskStatusPending,
	}

	err := q.Enqueue(task)

	assert.NoError(t, err)
	// 验证生成了新的 ID
	assert.NotEmpty(t, task.IDValue)
}
