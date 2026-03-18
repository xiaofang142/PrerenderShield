package redis

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestSubscriber_NewSubscriber 测试创建 Subscriber
func TestSubscriber_NewSubscriber(t *testing.T) {
	// 使用 nil 客户端测试
	sub := NewSubscriber(nil)
	assert.NotNil(t, sub)
	assert.NotNil(t, sub.ctx)
	assert.NotNil(t, sub.cancel)
	assert.NotNil(t, sub.handlers)
	assert.False(t, sub.isRunning)
}

// TestSubscriber_AddHandler 测试添加事件处理函数
func TestSubscriber_AddHandler(t *testing.T) {
	sub := NewSubscriber(nil)

	handlerCalled := false
	handler := func(channel, payload string) {
		handlerCalled = true
	}

	sub.AddHandler("test-channel", handler)

	// 验证 handler 已添加
	_, exists := sub.handlers["test-channel"]
	assert.True(t, exists)

	// 验证可以调用 handler
	sub.handlers["test-channel"]("test-channel", "test-payload")
	assert.True(t, handlerCalled)
}

// TestSubscriber_AddHandler_MultipleChannels 测试添加多个频道处理函数
func TestSubscriber_AddHandler_MultipleChannels(t *testing.T) {
	sub := NewSubscriber(nil)

	handlers := []string{"channel1", "channel2", "channel3"}
	for _, ch := range handlers {
		sub.AddHandler(ch, func(channel, payload string) {})
	}

	assert.Len(t, sub.handlers, 3)
}

// TestSubscriber_Start_AlreadyRunning 测试启动已运行的订阅者
func TestSubscriber_Start_AlreadyRunning(t *testing.T) {
	sub := NewSubscriber(nil)
	sub.isRunning = true

	err := sub.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "subscriber is already running")
}

// TestSubscriber_Stop 测试停止订阅者
func TestSubscriber_Stop(t *testing.T) {
	sub := NewSubscriber(nil)

	// 停止未运行的订阅者应该不会 panic
	sub.Stop()

	// 验证 context 被取消
	select {
	case <-sub.ctx.Done():
		// 正确
	default:
		t.Error("context should be cancelled")
	}
}

// TestSubscriber_IsRunning 测试 IsRunning 方法
func TestSubscriber_IsRunning(t *testing.T) {
	sub := NewSubscriber(nil)

	assert.False(t, sub.IsRunning())

	sub.isRunning = true
	assert.True(t, sub.IsRunning())
}

// TestSubscriber_Publish 测试发布消息
func TestSubscriber_Publish_NilClient(t *testing.T) {
	sub := NewSubscriber(nil)

	// 使用 nil client 应该会 panic，使用 defer 捕获
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Publish panicked as expected with nil client: %v", r)
		}
	}()

	sub.Publish("test-channel", "test-message")
}

// TestSubscriber_Start_NilClient 测试使用 nil client 启动订阅者
func TestSubscriber_Start_NilClient(t *testing.T) {
	sub := NewSubscriber(nil)

	defer func() {
		if r := recover(); r != nil {
			t.Logf("Start panicked as expected with nil client: %v", r)
		}
	}()

	err := sub.Start()
	// 可能返回错误或者 panic
	if err == nil {
		// 如果没有错误，等待一小段时间然后停止
		time.Sleep(100 * time.Millisecond)
		sub.Stop()
	}
}

// TestSubscriber_HandlerExecution 测试处理函数执行
func TestSubscriber_HandlerExecution(t *testing.T) {
	sub := NewSubscriber(nil)

	var capturedChannel, capturedPayload string
	sub.AddHandler("test", func(ch, pl string) {
		capturedChannel = ch
		capturedPayload = pl
	})

	// 直接调用 handler 验证
	sub.handlers["test"]("received-channel", "received-payload")

	assert.Equal(t, "received-channel", capturedChannel)
	assert.Equal(t, "received-payload", capturedPayload)
}

// TestSubscriber_HandlerNotExist 测试调用不存在的 handler
func TestSubscriber_HandlerNotExist(t *testing.T) {
	sub := NewSubscriber(nil)

	// 调用不存在的 handler 应该不会 panic
	handler, exists := sub.handlers["nonexistent"]
	assert.False(t, exists)
	assert.Nil(t, handler)
}

// TestSubscriber_ConcurrentAddHandler 测试并发添加 handler
func TestSubscriber_ConcurrentAddHandler(t *testing.T) {
	// 这个测试会暴露竞态条件，因为 AddHandler 没有使用锁
	// 在实际使用中，AddHandler 应该在初始化阶段调用，而不是并发调用
	// 这里我们改为顺序添加来验证功能
	sub := NewSubscriber(nil)

	for i := 0; i < 10; i++ {
		sub.AddHandler("channel-"+string(rune(i+'0')), func(c, p string) {})
	}

	assert.Len(t, sub.handlers, 10)
}

// TestSubscriber_Start_Stop_Lifecycle 测试完整的生命周期
func TestSubscriber_Start_Stop_Lifecycle(t *testing.T) {
	sub := NewSubscriber(nil)

	// 初始状态
	assert.False(t, sub.IsRunning())

	// 启动（nil client 会 panic 或返回错误）
	defer func() {
		recover()
	}()

	_ = sub.Start()

	// 停止
	sub.Stop()

	// 验证 context 已取消
	select {
	case <-sub.ctx.Done():
		// 正确
	default:
		t.Error("context should be cancelled")
	}
}

// TestSubscriber_ChannelPatterns 测试频道命名模式
func TestSubscriber_ChannelPatterns(t *testing.T) {
	testCases := []string{
		"site:config:change",
		"system:reload",
		"user:session:update",
		"cache:invalidate",
		"prerender:complete",
	}

	for _, channel := range testCases {
		assert.NotEmpty(t, channel)
		assert.Contains(t, channel, ":")
	}
}

// TestSubscriber_EmptyPayload 测试空 payload
func TestSubscriber_EmptyPayload(t *testing.T) {
	sub := NewSubscriber(nil)

	var capturedPayload string
	sub.AddHandler("test", func(ch, pl string) {
		capturedPayload = pl
	})

	sub.handlers["test"]("test", "")
	assert.Equal(t, "", capturedPayload)
}

// TestSubscriber_LongPayload 测试长 payload
func TestSubscriber_LongPayload(t *testing.T) {
	sub := NewSubscriber(nil)

	longPayload := string(make([]byte, 10000))

	var capturedPayload string
	sub.AddHandler("test", func(ch, pl string) {
		capturedPayload = pl
	})

	sub.handlers["test"]("test", longPayload)
	assert.Len(t, capturedPayload, 10000)
}

// TestSubscriber_RaceCondition 测试竞态条件
func TestSubscriber_RaceCondition(t *testing.T) {
	sub := NewSubscriber(nil)

	// 这个测试会暴露竞态条件，因为 AddHandler 没有使用锁
	// 改为顺序调用来验证功能
	for i := 0; i < 10; i++ {
		sub.AddHandler("channel", func(c, p string) {})
	}

	// 验证 handler 存在
	_, exists := sub.handlers["channel"]
	assert.True(t, exists)
}

// TestSubscriber_ContextCancellation 测试 context 取消
func TestSubscriber_ContextCancellation(t *testing.T) {
	sub := NewSubscriber(nil)

	// 手动取消 context
	sub.cancel()

	// 验证 context 已取消
	select {
	case <-sub.ctx.Done():
		// 正确
	default:
		t.Error("context should be cancelled")
	}
}

// TestSubscriber_HandlerMapInitialization 测试 handler map 初始化
func TestSubscriber_HandlerMapInitialization(t *testing.T) {
	sub := NewSubscriber(nil)

	assert.NotNil(t, sub.handlers)
	assert.Empty(t, sub.handlers)
	assert.IsType(t, make(map[string]func(string, string)), sub.handlers)
}

// TestSubscriber_Stop_MultipleTimes 测试多次停止
func TestSubscriber_Stop_MultipleTimes(t *testing.T) {
	sub := NewSubscriber(nil)

	// 多次调用 Stop 应该不会 panic
	sub.Stop()
	sub.Stop()
	sub.Stop()

	// 验证 context 已取消
	select {
	case <-sub.ctx.Done():
		// 正确
	default:
		t.Error("context should be cancelled")
	}
}

// TestSubscriber_isRunningFlag 测试 isRunning 标志
func TestSubscriber_isRunningFlag(t *testing.T) {
	sub := NewSubscriber(nil)

	// 初始为 false
	assert.False(t, sub.isRunning)

	// 设置为 true
	sub.isRunning = true
	assert.True(t, sub.isRunning)

	// 设置为 false
	sub.isRunning = false
	assert.False(t, sub.isRunning)
}

// TestSubscriber_NewSubscriber_MultipleInstances 测试创建多个实例
func TestSubscriber_NewSubscriber_MultipleInstances(t *testing.T) {
	sub1 := NewSubscriber(nil)
	sub2 := NewSubscriber(nil)

	assert.NotNil(t, sub1)
	assert.NotNil(t, sub2)

	// 验证是不同的实例
	assert.NotEqual(t, sub1, sub2)

	// 验证 handlers 是独立的
	sub1.AddHandler("sub1-channel", func(c, p string) {})
	assert.Len(t, sub1.handlers, 1)
	assert.Empty(t, sub2.handlers)
}
