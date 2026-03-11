package prerender

import (
	"container/heap"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPriorityConstants(t *testing.T) {
	assert.Equal(t, Priority(1), PriorityLow)
	assert.Equal(t, Priority(5), PriorityNormal)
	assert.Equal(t, Priority(8), PriorityHigh)
	assert.Equal(t, Priority(10), PriorityVIP)
}

func TestPriorityQueue_Len(t *testing.T) {
	pq := make(PriorityQueue, 0)
	assert.Equal(t, 0, pq.Len())

	pq = append(pq, &RenderTask{ID: "1"})
	assert.Equal(t, 1, pq.Len())
}

func TestPriorityQueue_Less(t *testing.T) {
	now := time.Now()
	pq := PriorityQueue{
		&RenderTask{ID: "1", Priority: PriorityLow, CreatedAt: now},
		&RenderTask{ID: "2", Priority: PriorityHigh, CreatedAt: now},
	}

	// 高优先级应该在前
	assert.True(t, pq.Less(1, 0))

	// 相同优先级，先创建的在前
	pq = PriorityQueue{
		&RenderTask{ID: "1", Priority: PriorityNormal, CreatedAt: now.Add(time.Hour)},
		&RenderTask{ID: "2", Priority: PriorityNormal, CreatedAt: now},
	}
	assert.True(t, pq.Less(1, 0))
}

func TestPriorityQueue_PushAndPop(t *testing.T) {
	pq := make(PriorityQueue, 0)

	// Push
	heap.Push(&pq, &RenderTask{ID: "1", Priority: PriorityNormal})
	assert.Equal(t, 1, len(pq))

	// Pop
	task := heap.Pop(&pq).(*RenderTask)
	assert.Equal(t, "1", task.ID)
	assert.Equal(t, 0, len(pq))
}

func TestRenderQueue_EnqueueAndDequeue(t *testing.T) {
	rq := NewRenderQueue(100)

	task := &RenderTask{
		ID:       "test-1",
		URL:      "https://example.com",
		SiteID:   "site-1",
		Priority: PriorityNormal,
	}

	// Enqueue
	err := rq.Enqueue(task)
	assert.NoError(t, err)
	assert.Equal(t, 1, rq.Len())

	// Dequeue
	dequeued := rq.DequeueNonBlocking()
	assert.NotNil(t, dequeued)
	assert.Equal(t, "test-1", dequeued.ID)
	assert.Equal(t, 0, rq.Len())
}

func TestRenderQueue_PriorityOrder(t *testing.T) {
	rq := NewRenderQueue(100)

	// 按不同优先级添加任务
	rq.Enqueue(&RenderTask{ID: "low", Priority: PriorityLow})
	rq.Enqueue(&RenderTask{ID: "high", Priority: PriorityHigh})
	rq.Enqueue(&RenderTask{ID: "vip", Priority: PriorityVIP})
	rq.Enqueue(&RenderTask{ID: "normal", Priority: PriorityNormal})

	// 应该按优先级顺序出队
	assert.Equal(t, "vip", rq.DequeueNonBlocking().ID)
	assert.Equal(t, "high", rq.DequeueNonBlocking().ID)
	assert.Equal(t, "normal", rq.DequeueNonBlocking().ID)
	assert.Equal(t, "low", rq.DequeueNonBlocking().ID)
}

func TestRenderQueue_CancelTask(t *testing.T) {
	rq := NewRenderQueue(100)

	task := &RenderTask{ID: "to-cancel", Priority: PriorityNormal}
	rq.Enqueue(task)

	// Cancel
	success := rq.CancelTask("to-cancel")
	assert.True(t, success)
	assert.Equal(t, 0, rq.Len())

	// Cancel non-existent
	success = rq.CancelTask("non-existent")
	assert.False(t, success)
}

func TestRenderQueue_GetTask(t *testing.T) {
	rq := NewRenderQueue(100)

	task := &RenderTask{ID: "test-task", Priority: PriorityNormal}
	rq.Enqueue(task)

	// Get
	found := rq.GetTask("test-task")
	assert.NotNil(t, found)
	assert.Equal(t, "test-task", found.ID)

	// Get non-existent
	found = rq.GetTask("non-existent")
	assert.Nil(t, found)
}

func TestRenderQueue_Stats(t *testing.T) {
	rq := NewRenderQueue(100)

	rq.Enqueue(&RenderTask{ID: "1", Priority: PriorityVIP})
	rq.Enqueue(&RenderTask{ID: "2", Priority: PriorityHigh})
	rq.Enqueue(&RenderTask{ID: "3", Priority: PriorityNormal})

	stats := rq.Stats()
	assert.Equal(t, 3, stats["total_tasks"])
	assert.Equal(t, false, stats["closed"])
	assert.Equal(t, 100, stats["max_size"])
}

func TestRenderQueue_Close(t *testing.T) {
	rq := NewRenderQueue(100)

	rq.Close()
	assert.Equal(t, true, rq.Stats()["closed"])
}

func TestCalculatePriority(t *testing.T) {
	// VIP 站点
	opts := PriorityOptions{IsVIP: true}
	assert.Equal(t, PriorityVIP, CalculatePriority(opts))

	// 预热任务
	opts = PriorityOptions{IsPreheat: true}
	assert.Equal(t, PriorityLow, CalculatePriority(opts))

	// 用户触发
	opts = PriorityOptions{IsUserTriggered: true}
	assert.Equal(t, PriorityHigh, CalculatePriority(opts))

	// 搜索引擎爬虫
	opts = PriorityOptions{UserAgent: "Mozilla/5.0 Googlebot"}
	assert.Equal(t, PriorityHigh, CalculatePriority(opts))

	// 普通
	opts = PriorityOptions{}
	assert.Equal(t, PriorityNormal, CalculatePriority(opts))
}

func TestIsSearchEngineBot(t *testing.T) {
	assert.True(t, isSearchEngineBot("Mozilla/5.0 Googlebot/2.1"))
	assert.True(t, isSearchEngineBot("Bingbot/2.0"))
	assert.True(t, isSearchEngineBot("Baiduspider/2.0"))
	assert.False(t, isSearchEngineBot("Mozilla/5.0 Chrome"))
}
