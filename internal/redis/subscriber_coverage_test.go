package redis

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// waitForCond 轮询等待条件成立（最长 5 秒）。
// 注意：Subscriber.Stop 取消 ctx 后，go-redis v8 的 pubsub 阻塞读约 2s 才返回错误，
// 2s 窗口在负载下必然越界，故放宽至 5s
func waitForCond(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within 5s")
}

// TestCovSubscriber_Start_FullLifecycle 覆盖 Subscriber.Start 的完整生命周期：
// 订阅频道、收到消息并调用 handler、Stop 后退出 goroutine
func TestCovSubscriber_Start_FullLifecycle(t *testing.T) {
	cl := newCovClient(t)

	sub := NewSubscriber(cl.GetRawClient())
	got := make(chan string, 4)
	sub.AddHandler("cov-test-chan", func(channel, payload string) {
		got <- payload
	})

	assert.NoError(t, sub.Start())
	waitForCond(t, sub.IsRunning)

	// 发布消息，等待 handler 被调用
	err := cl.Publish("cov-test-chan", "hello-world")
	assert.NoError(t, err)
	select {
	case payload := <-got:
		assert.Equal(t, "hello-world", payload)
	case <-time.After(3 * time.Second):
		t.Fatal("handler was not invoked within 3s")
	}

	// 停止订阅者，goroutine 因 context canceled 退出
	sub.Stop()
	waitForCond(t, func() bool { return !sub.IsRunning() })
}

// TestCovSubscriber_Start_AlreadyRunningFlag 覆盖 Start 的重复启动错误分支
func TestCovSubscriber_Start_AlreadyRunningFlag(t *testing.T) {
	cl := newCovClient(t)
	sub := NewSubscriber(cl.GetRawClient())
	sub.AddHandler("cov-test-chan2", func(channel, payload string) {})

	// 直接置位运行标记，触发 "already running" 错误
	sub.isRunning.Store(true)
	err := sub.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
	sub.isRunning.Store(false)
}

// TestCovSubscriber_Start_ClosedClient 覆盖接收消息出错（非 context canceled）的
// continue 重试分支：客户端已关闭时 ReceiveMessage 持续报错，Stop 后退出
func TestCovSubscriber_Start_ClosedClient(t *testing.T) {
	cl := newCovClient(t)
	raw := cl.GetRawClient()
	_ = cl.Close() // 先关闭，制造接收错误

	sub := NewSubscriber(raw)
	sub.AddHandler("cov-test-chan3", func(channel, payload string) {})

	assert.NoError(t, sub.Start())
	waitForCond(t, sub.IsRunning)
	time.Sleep(100 * time.Millisecond) // 让 goroutine 进入错误重试分支

	sub.Stop()
	waitForCond(t, func() bool { return !sub.IsRunning() })
}
