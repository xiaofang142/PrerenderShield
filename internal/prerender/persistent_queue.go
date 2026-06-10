package prerender

import (
	"encoding/json"
	"fmt"

	"prerender-shield/internal/redis"
)

type PersistentQueue struct {
	redisCli *redis.Client
	prefix   string
}

func NewPersistentQueue(redisCli *redis.Client, prefix string) *PersistentQueue {
	return &PersistentQueue{redisCli: redisCli, prefix: prefix}
}

func (pq *PersistentQueue) persistKey() string {
	return fmt.Sprintf("%s:list", pq.prefix)
}

func (pq *PersistentQueue) Enqueue(task *RenderTask) error {
	if task == nil {
		return fmt.Errorf("task is nil")
	}
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshal task: %w", err)
	}
	return pq.redisCli.ListPush(pq.persistKey(), string(data))
}

func (pq *PersistentQueue) Dequeue() (*RenderTask, error) {
	val, err := pq.redisCli.ListPop(pq.persistKey())
	if err != nil || val == "" {
		return nil, nil
	}
	var task RenderTask
	if err := json.Unmarshal([]byte(val), &task); err != nil {
		return nil, err
	}
	return &task, nil
}

func (pq *PersistentQueue) Clear() error {
	return pq.redisCli.Del(pq.persistKey())
}
