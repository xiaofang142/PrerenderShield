package redis

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/go-redis/redis/v8"
	"prerender-shield/internal/logging"
)

// Subscriber Redis订阅者，用于监听Redis中的配置变更
type Subscriber struct {
	client    *redis.Client
	ctx       context.Context
	cancel    context.CancelFunc
	handlers  map[string]func(string, string)
	pubsub    *redis.PubSub // Start 时创建；Stop 时关闭以解除阻塞的 ReceiveMessage
	isRunning atomic.Bool   // 消费 goroutine 写、IsRunning/Start 读，需原子访问
}

// NewSubscriber 创建Redis订阅者实例
func NewSubscriber(client *redis.Client) *Subscriber {
	ctx, cancel := context.WithCancel(context.Background())
	return &Subscriber{
		client:   client,
		ctx:      ctx,
		cancel:   cancel,
		handlers: make(map[string]func(string, string)),
	}
}

// AddHandler 添加事件处理函数
func (s *Subscriber) AddHandler(channel string, handler func(string, string)) {
	s.handlers[channel] = handler
}

// Start 启动订阅者
func (s *Subscriber) Start() error {
	if s.isRunning.Load() {
		return fmt.Errorf("subscriber is already running")
	}

	// 构建频道列表
	channels := make([]string, 0, len(s.handlers))
	for channel := range s.handlers {
		channels = append(channels, channel)
	}

	// 订阅频道
	pubsub := s.client.Subscribe(s.ctx, channels...)
	s.pubsub = pubsub

	// 启动goroutine处理消息
	go func() {
		defer func() {
			if err := pubsub.Close(); err != nil {
				logging.DefaultLogger.Info("Failed to close pubsub: %v", err)
			}
		}()

		s.isRunning.Store(true)
		logging.DefaultLogger.Info("Redis subscriber started")

		for {
			msg, err := pubsub.ReceiveMessage(s.ctx)
			if err != nil {
				// 注：go-redis v8.11.5 的 PubSub.ReceiveMessage 不经 pool.Get/withConn
				// 且无 ctx.Done select，任何错误都不含 "context canceled"，本分支在
				// 当前依赖版本下不可达；升级 go-redis v9 后 ReceiveMessage 会直接
				// 返回 ctx.Err()（"context canceled"），此分支即成为正常退出路径，
				// 故保留以兼容依赖升级
				if strings.Contains(err.Error(), "context canceled") {
					break
				}
				// Stop 取消 ctx 后，go-redis 可能先抛出底层网络错误
				// （如 use of closed network connection / client is closed）而非
				// context.Canceled——此时必须退出循环，否则 goroutine 空转、IsRunning 永真
				if s.ctx.Err() != nil {
					break
				}
				logging.DefaultLogger.Info("Failed to receive message: %v", err)
				continue
			}

			// 调用对应的处理函数
			if handler, exists := s.handlers[msg.Channel]; exists {
				handler(msg.Channel, msg.Payload)
			}
		}

		s.isRunning.Store(false)
		logging.DefaultLogger.Info("Redis subscriber stopped")
	}()

	return nil
}

// Stop 停止订阅者。
// 仅 cancel 时 go-redis v8 的 pubsub 阻塞读不响应 ctx（持续数秒才超时返回），
// 必须同步关闭 pubsub 连接使 ReceiveMessage 立即报错退出循环。
// PubSub.Close 幂等（内部持锁），与消费 goroutine 的 defer Close 并发安全
func (s *Subscriber) Stop() {
	s.cancel()
	if s.pubsub != nil {
		_ = s.pubsub.Close()
	}
}

// Publish 发布消息到指定频道
func (s *Subscriber) Publish(channel, message string) error {
	return s.client.Publish(s.ctx, channel, message).Err()
}

// IsRunning 检查订阅者是否正在运行
func (s *Subscriber) IsRunning() bool {
	return s.isRunning.Load()
}
