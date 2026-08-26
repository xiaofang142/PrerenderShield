package prerender

import (
	"sync"
)

// RequestCoalescer (P0-30) 缓存击穿保护
// 当多个 goroutine 同时请求同一个 key (例如同一 URL) 时，
// 只让第一个执行实际工作 (caller 提供的 fn) ，其他等待其结果共享。
//
// 典型场景: 缓存过期瞬间，1000 个并发请求同一 URL
//   - 没有 Coalescer: 1000 次重复渲染，资源耗尽
//   - 有 Coalescer: 仅 1 次渲染，999 个共享结果
type RequestCoalescer struct {
	mu      sync.Mutex
	pending map[string]*call
}

// call 单个进行中的调用
type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

// NewRequestCoalescer 创建新的 coalescer
func NewRequestCoalescer() *RequestCoalescer {
	return &RequestCoalescer{
		pending: make(map[string]*call),
	}
}

// Do (P0-30) 执行 fn，相同 key 的并发请求会合并到同一次 fn 调用
// 返回值会传递给所有等待者
//
// key: 合并键 (例如 cache key)
// fn: 实际工作函数 (只会被第一个请求执行)
func (c *RequestCoalescer) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	c.mu.Lock()
	if existing, ok := c.pending[key]; ok {
		// 已有进行中的调用，等待其完成
		c.mu.Unlock()
		existing.wg.Wait()
		return existing.val, existing.err
	}

	// 创建新调用
	newCall := &call{}
	newCall.wg.Add(1)
	c.pending[key] = newCall
	c.mu.Unlock()

	// 执行实际工作
	newCall.val, newCall.err = fn()
	newCall.wg.Done()

	// 清理 pending map
	c.mu.Lock()
	delete(c.pending, key)
	c.mu.Unlock()

	return newCall.val, newCall.err
}

// PendingCount 返回当前正在进行的调用数 (用于监控/测试)
func (c *RequestCoalescer) PendingCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pending)
}
