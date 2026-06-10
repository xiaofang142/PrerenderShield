package prerender

import (
	"encoding/json"
	"fmt"
	"time"

	"prerender-shield/internal/redis"
)

type PersistentQueue struct {
	redisCli *redis.Client
	prefix   string
}

func NewPersistentQueue(redisCli *redis.Client, prefix string) *PersistentQueue {
	return &PersistentQueue{redisCli: redisCli, prefix: prefix}
}

func (pq *PersistentQueue) Enqueue(task *RenderTask) error {
	if task == nil {
		return fmt.Errorf("task is nil")
	}
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshal task: %w", err)
	}
	key := fmt.Sprintf("%s:task:%s", pq.prefix, task.ID)
	return pq.redisCli.Set(key, string(data), 2*time.Hour)
}

func (pq *PersistentQueue) Dequeue() (*RenderTask, error) {
	pattern := fmt.Sprintf("%s:task:*", pq.prefix)
	keys, err := pq.redisCli.Keys(pattern)
	if err != nil || len(keys) == 0 {
		return nil, nil
	}
	key := keys[0]
	val, err := pq.redisCli.Get(key)
	if err != nil || val == "" {
		return nil, nil
	}
	var task RenderTask
	if err := json.Unmarshal([]byte(val), &task); err != nil {
		return nil, err
	}
	pq.redisCli.Del(key)
	return &task, nil
}
