package redis

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/go-redis/redis/v8"
)

// Subscriber Redis订阅者，用于监听Redis中的配置变更
type Subscriber struct {
	client    *redis.Client
	ctx       context.Context
	cancel    context.CancelFunc
	handlers  map[string]func(string, string)
	isRunning bool
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
	if s.isRunning {
		return fmt.Errorf("subscriber is already running")
	}

	// 构建频道列表
	channels := make([]string, 0, len(s.handlers))
	for channel := range s.handlers {
		channels = append(channels, channel)
	}

	// 订阅频道
	pubsub := s.client.Subscribe(s.ctx, channels...)

	// 启动goroutine处理消息
	go func() {
		defer func() {
			if err := pubsub.Close(); err != nil {
				log.Printf("Failed to close pubsub: %v", err)
			}
		}()

		s.isRunning = true
		log.Println("Redis subscriber started")

		for {
			msg, err := pubsub.ReceiveMessage(s.ctx)
			if err != nil {
				if strings.Contains(err.Error(), "context canceled") {
					break
				}
				log.Printf("Failed to receive message: %v", err)
				continue
			}

			// 调用对应的处理函数
			if handler, exists := s.handlers[msg.Channel]; exists {
				handler(msg.Channel, msg.Payload)
			}
		}

		s.isRunning = false
		log.Println("Redis subscriber stopped")
	}()

	return nil
}

// Stop 停止订阅者
func (s *Subscriber) Stop() {
	s.cancel()
}

// Publish 发布消息到指定频道
func (s *Subscriber) Publish(channel, message string) error {
	return s.client.Publish(s.ctx, channel, message).Err()
}

// IsRunning 检查订阅者是否正在运行
func (s *Subscriber) IsRunning() bool {
	return s.isRunning
}
