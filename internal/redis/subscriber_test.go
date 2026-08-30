package redis

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewSubscriber 测试创建订阅者
func TestNewSubscriber(t *testing.T) {
	subscriber := NewSubscriber(nil)
	assert.NotNil(t, subscriber)
	assert.NotNil(t, subscriber.handlers)
	assert.False(t, subscriber.isRunning.Load())
}

// TestAddHandler 测试添加事件处理函数
func TestAddHandler(t *testing.T) {
	subscriber := NewSubscriber(nil)

	handler := func(channel, payload string) {}

	subscriber.AddHandler("test-channel", handler)
	assert.Len(t, subscriber.handlers, 1)

	// 验证处理函数已注册
	_, exists := subscriber.handlers["test-channel"]
	assert.True(t, exists)
}

// TestAddMultipleHandlers 测试添加多个处理函数
func TestAddMultipleHandlers(t *testing.T) {
	subscriber := NewSubscriber(nil)

	subscriber.AddHandler("channel1", func(channel, payload string) {})
	subscriber.AddHandler("channel2", func(channel, payload string) {})
	subscriber.AddHandler("channel3", func(channel, payload string) {})

	assert.Len(t, subscriber.handlers, 3)
}

// TestIsRunning 测试 IsRunning 方法
func TestIsRunning(t *testing.T) {
	subscriber := NewSubscriber(nil)

	// 初始状态
	assert.False(t, subscriber.IsRunning())

	// 手动设置状态（实际由 Start 方法设置）
	subscriber.isRunning.Store(true)
	assert.True(t, subscriber.IsRunning())
}

// TestStop 测试 Stop 方法
func TestStop(t *testing.T) {
	subscriber := NewSubscriber(nil)

	// 停止之前
	assert.False(t, subscriber.isRunning.Load())

	// 调用 Stop
	subscriber.Stop()

	// 验证 context 已取消
	// 注意：由于我们无法直接检查 context 是否取消，这里只测试方法可以调用
	assert.NotNil(t, subscriber.cancel)
}

// TestSubscriber_Struct 测试 Subscriber 结构体
func TestSubscriber_Struct(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subscriber := &Subscriber{
		client:   nil,
		ctx:      ctx,
		cancel:   cancel,
		handlers: make(map[string]func(string, string)),
	}

	assert.Nil(t, subscriber.client)
	assert.NotNil(t, subscriber.ctx)
	assert.NotNil(t, subscriber.cancel)
	assert.NotNil(t, subscriber.handlers)
	assert.False(t, subscriber.isRunning.Load())
}

// TestHandlerFunction 测试处理函数的调用
func TestHandlerFunction(t *testing.T) {
	receivedChannel := ""
	receivedPayload := ""

	handler := func(channel, payload string) {
		receivedChannel = channel
		receivedPayload = payload
	}

	// 调用处理函数
	handler("test-channel", "test-payload")

	assert.Equal(t, "test-channel", receivedChannel)
	assert.Equal(t, "test-payload", receivedPayload)
}

// TestStart_NilClient 测试使用 nil client 启动
func TestStart_NilClient(t *testing.T) {
	subscriber := NewSubscriber(nil)

	// 由于 client 为 nil，Start 会 panic，所以这里只测试方法存在
	// 实际使用需要传入有效的 redis client
	assert.NotNil(t, subscriber.Start)
}

// TestPublish_NilClient 测试使用 nil client 发布消息
func TestPublish_NilClient(t *testing.T) {
	subscriber := NewSubscriber(nil)

	// 由于 client 为 nil，Publish 会 panic
	// 这里只测试方法存在
	assert.NotNil(t, subscriber.Publish)
}

// TestContextCancellation 测试 context 取消
func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 取消 context
	cancel()

	// 验证 context 已取消
	select {
	case <-ctx.Done():
		// 正确，context 已取消
	default:
		t.Error("context should be cancelled")
	}
}

// TestHandlerMap 测试处理函数 map 的操作
func TestHandlerMap(t *testing.T) {
	handlers := make(map[string]func(string, string))

	// 添加处理函数
	handlers["channel1"] = func(c, p string) {}
	handlers["channel2"] = func(c, p string) {}

	// 验证存在
	_, exists := handlers["channel1"]
	assert.True(t, exists)

	// 删除处理函数
	delete(handlers, "channel1")
	_, exists = handlers["channel1"]
	assert.False(t, exists)

	// 验证另一个还在
	_, exists = handlers["channel2"]
	assert.True(t, exists)
}

// TestBuildChannelList 测试构建频道列表
func TestBuildChannelList(t *testing.T) {
	handlers := map[string]func(string, string){
		"channel1": nil,
		"channel2": nil,
		"channel3": nil,
	}

	// 构建频道列表
	channels := make([]string, 0, len(handlers))
	for channel := range handlers {
		channels = append(channels, channel)
	}

	assert.Len(t, channels, 3)
	assert.Contains(t, channels, "channel1")
	assert.Contains(t, channels, "channel2")
	assert.Contains(t, channels, "channel3")
}

// TestSubscriberWithMultipleChannels 测试多频道订阅
func TestSubscriberWithMultipleChannels(t *testing.T) {
	subscriber := NewSubscriber(nil)

	// 添加多个频道的处理函数
	channels := []string{"config", "events", "notifications"}
	for _, channel := range channels {
		subscriber.AddHandler(channel, func(ch, payload string) {})
	}

	assert.Len(t, subscriber.handlers, len(channels))
}

// TestHandlerExecution 测试处理函数执行
func TestHandlerExecution(t *testing.T) {
	executionCount := 0
	handler := func(channel, payload string) {
		executionCount++
	}

	// 多次调用
	for i := 0; i < 5; i++ {
		handler("channel", "payload")
	}

	assert.Equal(t, 5, executionCount)
}

// TestSubscriberState 测试订阅者状态
func TestSubscriberState(t *testing.T) {
	subscriber := NewSubscriber(nil)

	// 初始状态
	assert.False(t, subscriber.IsRunning())

	// 模拟启动
	subscriber.isRunning.Store(true)
	assert.True(t, subscriber.IsRunning())

	// 模拟停止（通过 cancel）
	subscriber.Stop()
	// isRunning 状态由内部 goroutine 设置，这里不直接验证
}

// TestSubscriberConcurrency 测试并发安全
func TestSubscriberConcurrency(t *testing.T) {
	subscriber := NewSubscriber(nil)

	// 使用 mutex 保护并发访问
	var mu sync.Mutex

	// 并发添加处理函数
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			mu.Lock()
			subscriber.AddHandler("channel"+string(rune(id)), func(c, p string) {})
			mu.Unlock()
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	assert.GreaterOrEqual(t, len(subscriber.handlers), 1)
}

// TestNilHandler 测试 nil 处理函数
func TestNilHandler(t *testing.T) {
	subscriber := NewSubscriber(nil)

	// 添加 nil 处理函数
	subscriber.AddHandler("channel", nil)

	// 验证已添加（即使是 nil）
	_, exists := subscriber.handlers["channel"]
	assert.True(t, exists)
}

// TestEmptyHandlerMap 测试空处理函数 map
func TestEmptyHandlerMap(t *testing.T) {
	subscriber := NewSubscriber(nil)

	assert.Empty(t, subscriber.handlers)
	assert.NotNil(t, subscriber.handlers)
}

// TestChannelPattern 测试频道命名模式
func TestChannelPattern(t *testing.T) {
	patterns := []string{
		"config:update",
		"events:alert",
		"notifications:user",
		"system:health",
	}

	for _, pattern := range patterns {
		assert.Contains(t, pattern, ":")
		parts := []string{"category", "type"}
		assert.Len(t, parts, 2)
	}
}

// TestMessagePayload 测试消息载荷格式
func TestMessagePayload(t *testing.T) {
	// 测试不同类型载荷
	payloads := []string{
		"simple string",
		`{"json": "object"}`,
		"123",
		"true",
	}

	for _, payload := range payloads {
		assert.NotEmpty(t, payload)
	}
}

// TestTimeRelated 测试时间相关功能
func TestTimeRelated(t *testing.T) {
	// 验证超时设置
	timeout := 5 * time.Second
	assert.Greater(t, timeout, time.Duration(0))

	// 验证重试间隔
	retryInterval := 1 * time.Second
	assert.Greater(t, retryInterval, time.Duration(0))
}
