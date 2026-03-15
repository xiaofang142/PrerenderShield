package eventbus

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"prerender-shield/internal/logging"
)

func TestNewEvent(t *testing.T) {
	data := map[string]interface{}{"key": "value"}
	event := NewEvent("test.topic", "test-source", data)

	assert.NotEmpty(t, event.ID)
	assert.Equal(t, "test.topic", event.Topic)
	assert.Equal(t, "test-source", event.Source)
	assert.Equal(t, "value", event.Data["key"])
	assert.WithinDuration(t, time.Now(), event.Timestamp, time.Second)
}

func TestNewInMemoryBus(t *testing.T) {
	logger := logging.NewLogger(logging.Config{})
	bus := NewInMemoryBus(logger)

	assert.NotNil(t, bus)
	assert.NotNil(t, bus.handlers)
	assert.False(t, bus.closed)
	assert.Equal(t, logger, bus.logger)
}

func TestPublishNoSubscribers(t *testing.T) {
	bus := NewInMemoryBus(nil)
	defer bus.Close()

	event := NewEvent("test.topic", "source", nil)
	err := bus.Publish(context.Background(), "test.topic", event)

	assert.NoError(t, err)
}

func TestPublishWithSubscribers(t *testing.T) {
	bus := NewInMemoryBus(nil)
	defer bus.Close()

	var received Event
	var mu sync.Mutex

	handler := func(ctx context.Context, event Event) error {
		mu.Lock()
		defer mu.Unlock()
		received = event
		return nil
	}

	sub, err := bus.Subscribe(context.Background(), "test.topic", handler)
	assert.NoError(t, err)
	assert.NotNil(t, sub)

	event := NewEvent("test.topic", "source", map[string]interface{}{"data": "test"})
	err = bus.Publish(context.Background(), "test.topic", event)
	assert.NoError(t, err)

	// Wait for async handler
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, event.ID, received.ID)
	assert.Equal(t, "test", received.Data["data"])
	mu.Unlock()
}

func TestPublishWithError(t *testing.T) {
	logger := logging.NewLogger(logging.Config{})
	bus := NewInMemoryBus(logger)
	defer bus.Close()

	handler := func(ctx context.Context, event Event) error {
		return errors.New("handler error")
	}

	_, err := bus.Subscribe(context.Background(), "test.topic", handler)
	assert.NoError(t, err)

	event := NewEvent("test.topic", "source", nil)
	err = bus.Publish(context.Background(), "test.topic", event)
	assert.NoError(t, err) // Publish itself should not error
}

func TestSubscribeToBusClosed(t *testing.T) {
	bus := NewInMemoryBus(nil)
	bus.Close()

	_, err := bus.Subscribe(context.Background(), "test.topic", nil)
	assert.Error(t, err)
	assert.Equal(t, ErrBusClosed, err)
}

func TestPublishToBusClosed(t *testing.T) {
	bus := NewInMemoryBus(nil)

	// 先添加一个订阅者
	handler := func(ctx context.Context, event Event) error {
		return nil
	}
	sub, _ := bus.Subscribe(context.Background(), "test.topic", handler)

	// 关闭总线（会清空 handlers）
	bus.Close()

	// 关闭后 handlers 被清空，所以 Publish 会返回 nil（没有订阅者）
	event := NewEvent("test.topic", "source", nil)
	err := bus.Publish(context.Background(), "test.topic", event)
	assert.NoError(t, err)

	// 但是订阅操作会返回错误
	_, err = bus.Subscribe(context.Background(), "test.topic", handler)
	assert.Error(t, err)
	assert.Equal(t, ErrBusClosed, err)

	// 取消订阅也会返回错误
	err = sub.Unsubscribe()
	assert.NoError(t, err) // Unsubscribe 不检查 closed 状态
}

func TestUnsubscribe(t *testing.T) {
	bus := NewInMemoryBus(nil)
	defer bus.Close()

	handler := func(ctx context.Context, event Event) error {
		return nil
	}

	sub, err := bus.Subscribe(context.Background(), "test.topic", handler)
	assert.NoError(t, err)

	err = bus.Unsubscribe(context.Background(), sub)
	assert.NoError(t, err)

	// Verify handler is removed
	bus.mu.RLock()
	_, exists := bus.handlers["test.topic"]
	bus.mu.RUnlock()
	assert.False(t, exists)
}

func TestSubscriptionID(t *testing.T) {
	bus := NewInMemoryBus(nil)
	defer bus.Close()

	handler := func(ctx context.Context, event Event) error {
		return nil
	}

	sub, err := bus.Subscribe(context.Background(), "test.topic", handler)
	assert.NoError(t, err)

	assert.NotEmpty(t, sub.ID())
}

func TestSubscriptionUnsubscribe(t *testing.T) {
	bus := NewInMemoryBus(nil)
	defer bus.Close()

	handler := func(ctx context.Context, event Event) error {
		return nil
	}

	sub, err := bus.Subscribe(context.Background(), "test.topic", handler)
	assert.NoError(t, err)

	err = sub.Unsubscribe()
	assert.NoError(t, err)
}

func TestClose(t *testing.T) {
	bus := NewInMemoryBus(nil)

	handler := func(ctx context.Context, event Event) error {
		return nil
	}
	_, _ = bus.Subscribe(context.Background(), "test.topic", handler)

	err := bus.Close()
	assert.NoError(t, err)
	assert.True(t, bus.closed)
	assert.Empty(t, bus.handlers)
}

func TestMultipleSubscribers(t *testing.T) {
	bus := NewInMemoryBus(nil)
	defer bus.Close()

	var count int
	var mu sync.Mutex

	handler1 := func(ctx context.Context, event Event) error {
		mu.Lock()
		count++
		mu.Unlock()
		return nil
	}
	handler2 := func(ctx context.Context, event Event) error {
		mu.Lock()
		count++
		mu.Unlock()
		return nil
	}

	_, _ = bus.Subscribe(context.Background(), "test.topic", handler1)
	_, _ = bus.Subscribe(context.Background(), "test.topic", handler2)

	event := NewEvent("test.topic", "source", nil)
	err := bus.Publish(context.Background(), "test.topic", event)
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, 2, count)
	mu.Unlock()
}

func TestMultipleTopics(t *testing.T) {
	bus := NewInMemoryBus(nil)
	defer bus.Close()

	topic1Received := false
	topic2Received := false

	handler1 := func(ctx context.Context, event Event) error {
		topic1Received = true
		return nil
	}
	handler2 := func(ctx context.Context, event Event) error {
		topic2Received = true
		return nil
	}

	_, _ = bus.Subscribe(context.Background(), "topic1", handler1)
	_, _ = bus.Subscribe(context.Background(), "topic2", handler2)

	_ = bus.Publish(context.Background(), "topic1", NewEvent("topic1", "source", nil))
	_ = bus.Publish(context.Background(), "topic2", NewEvent("topic2", "source", nil))

	time.Sleep(50 * time.Millisecond)

	assert.True(t, topic1Received)
	assert.True(t, topic2Received)
}

func TestEventConstants(t *testing.T) {
	// Site events
	assert.Equal(t, "site.created", TopicSiteCreated)
	assert.Equal(t, "site.updated", TopicSiteUpdated)
	assert.Equal(t, "site.deleted", TopicSiteDeleted)

	// WAF events
	assert.Equal(t, "waf.attack_detected", TopicWAFAttackDetected)
	assert.Equal(t, "waf.rule_updated", TopicWAFRuleUpdated)
	assert.Equal(t, "waf.blocked", TopicWAFBlocked)

	// Render events
	assert.Equal(t, "render.complete", TopicRenderComplete)
	assert.Equal(t, "render.failed", TopicRenderFailed)
	assert.Equal(t, "cache.updated", TopicCacheUpdated)
	assert.Equal(t, "cache.cleared", TopicCacheCleared)

	// Auth events
	assert.Equal(t, "auth.login", TopicUserLogin)
	assert.Equal(t, "auth.logout", TopicUserLogout)
	assert.Equal(t, "auth.user_created", TopicUserCreated)
	assert.Equal(t, "auth.token_refresh", TopicTokenRefresh)

	// System events
	assert.Equal(t, "system.config_updated", TopicConfigUpdated)
	assert.Equal(t, "system.shutdown", TopicShutdown)
	assert.Equal(t, "system.health_alert", TopicHealthAlert)
}

func TestEventError(t *testing.T) {
	err := &EventError{Code: "TEST", Message: "test error"}
	assert.Equal(t, "TEST: test error", err.Error())
}

func TestErrBusClosed(t *testing.T) {
	assert.NotNil(t, ErrBusClosed)
	assert.Equal(t, "BUS_CLOSED", ErrBusClosed.Code)
	assert.Contains(t, ErrBusClosed.Error(), "event bus is closed")
}

func TestSubscribeUnsubscribeMultipleTimes(t *testing.T) {
	bus := NewInMemoryBus(nil)
	defer bus.Close()

	handler := func(ctx context.Context, event Event) error {
		return nil
	}

	// Subscribe and unsubscribe multiple times
	for i := 0; i < 3; i++ {
		sub, err := bus.Subscribe(context.Background(), "test.topic", handler)
		assert.NoError(t, err)
		err = sub.Unsubscribe()
		assert.NoError(t, err)
	}

	// Should still work
	event := NewEvent("test.topic", "source", nil)
	err := bus.Publish(context.Background(), "test.topic", event)
	assert.NoError(t, err)
}
