package eventbus

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"prerender-shield/internal/logging"
)

// Event 事件结构
type Event struct {
	ID        string                 `json:"id"`
	Topic     string                 `json:"topic"`
	Source    string                 `json:"source"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// NewEvent 创建新事件
func NewEvent(topic, source string, data map[string]interface{}) Event {
	return Event{
		ID:        uuid.New().String(),
		Topic:     topic,
		Source:    source,
		Timestamp: time.Now(),
		Data:      data,
	}
}

// EventHandler 事件处理函数
type EventHandler func(ctx context.Context, event Event) error

// Subscription 订阅接口
type Subscription interface {
	Unsubscribe() error
	ID() string
}

// EventBus 事件总线接口
type EventBus interface {
	Publish(ctx context.Context, topic string, event Event) error
	Subscribe(ctx context.Context, topic string, handler EventHandler) (Subscription, error)
	Unsubscribe(ctx context.Context, subscription Subscription) error
	Close() error
}

// InMemoryBus 内存事件总线实现（带错误处理和日志）
type InMemoryBus struct {
	handlers map[string]map[string]handlerEntry
	mu       sync.RWMutex
	closed   bool
	logger   *logging.Logger
	sem      chan struct{} // 限制并发 goroutine 数量
}

type handlerEntry struct {
	id      string
	handler EventHandler
}

const maxConcurrentHandlers = 100

// NewInMemoryBus 创建内存事件总线
func NewInMemoryBus(logger *logging.Logger) *InMemoryBus {
	return &InMemoryBus{
		handlers: make(map[string]map[string]handlerEntry),
		logger:   logger,
		sem:      make(chan struct{}, maxConcurrentHandlers),
	}
}

// Publish 发布事件
func (b *InMemoryBus) Publish(ctx context.Context, topic string, event Event) error {
	b.mu.RLock()
	handlers, ok := b.handlers[topic]
	if !ok {
		b.mu.RUnlock()
		return nil // 没有订阅者
	}

	// 复制处理器列表，减少锁持有时间
	handlerCopy := make([]handlerEntry, 0, len(handlers))
	for _, entry := range handlers {
		handlerCopy = append(handlerCopy, entry)
	}
	b.mu.RUnlock()

	if b.closed {
		return ErrBusClosed
	}

	// 异步通知所有订阅者，使用信号量限制并发数
	for _, entry := range handlerCopy {
		b.sem <- struct{}{} // 获取信号量
		go func(h handlerEntry) {
			defer func() { <-b.sem }() // 释放信号量

			if err := h.handler(ctx, event); err != nil {
				if b.logger != nil {
					b.logger.Error("Event handler error: handler_id=%s, topic=%s, error=%v", h.id, topic, err)
				}
			}
		}(entry)
	}

	return nil
}

// Subscribe 订阅事件
func (b *InMemoryBus) Subscribe(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, ErrBusClosed
	}

	if _, ok := b.handlers[topic]; !ok {
		b.handlers[topic] = make(map[string]handlerEntry)
	}

	id := uuid.New().String()
	b.handlers[topic][id] = handlerEntry{
		id:      id,
		handler: handler,
	}

	return &subscription{
		bus:   b,
		topic: topic,
		id:    id,
	}, nil
}

// Unsubscribe 取消订阅
func (b *InMemoryBus) Unsubscribe(ctx context.Context, subscription Subscription) error {
	return subscription.Unsubscribe()
}

// Close 关闭事件总线
func (b *InMemoryBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.closed = true
	b.handlers = make(map[string]map[string]handlerEntry)
	return nil
}

// subscription 订阅实现
type subscription struct {
	bus   *InMemoryBus
	topic string
	id    string
}

func (s *subscription) Unsubscribe() error {
	s.bus.mu.Lock()
	defer s.bus.mu.Unlock()

	if handlers, ok := s.bus.handlers[s.topic]; ok {
		delete(handlers, s.id)
		if len(handlers) == 0 {
			delete(s.bus.handlers, s.topic)
		}
	}
	return nil
}

func (s *subscription) ID() string {
	return s.id
}

// 预定义事件主题
const (
	// 站点事件
	TopicSiteCreated = "site.created"
	TopicSiteUpdated = "site.updated"
	TopicSiteDeleted = "site.deleted"

	// WAF 事件
	TopicWAFAttackDetected = "waf.attack_detected"
	TopicWAFRuleUpdated    = "waf.rule_updated"
	TopicWAFBlocked        = "waf.blocked"

	// 渲染事件
	TopicRenderComplete = "render.complete"
	TopicRenderFailed   = "render.failed"
	TopicCacheUpdated   = "cache.updated"
	TopicCacheCleared   = "cache.cleared"

	// 认证事件
	TopicUserLogin    = "auth.login"
	TopicUserLogout   = "auth.logout"
	TopicUserCreated  = "auth.user_created"
	TopicTokenRefresh = "auth.token_refresh"

	// 系统事件
	TopicConfigUpdated = "system.config_updated"
	TopicShutdown      = "system.shutdown"
	TopicHealthAlert   = "system.health_alert"
)

// 错误
var ErrBusClosed = &EventError{Code: "BUS_CLOSED", Message: "event bus is closed"}

// EventError 事件错误
type EventError struct {
	Code    string
	Message string
}

func (e *EventError) Error() string {
	return e.Code + ": " + e.Message
}
