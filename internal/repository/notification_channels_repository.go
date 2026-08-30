package repository

import (
	"encoding/json"
	"fmt"

	"prerender-shield/internal/redis"
)

const notificationChannelsKey = "monitoring:notification-channels"

// NotificationChannelData 通知渠道配置
type NotificationChannelData struct {
	Type    string `json:"type"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

// NotificationChannelsRepository 通知渠道配置（monitoring:notification-channels）的持久化仓储
type NotificationChannelsRepository struct {
	client *redis.Client
}

// NewNotificationChannelsRepository 创建通知渠道仓储
func NewNotificationChannelsRepository(client *redis.Client) *NotificationChannelsRepository {
	return &NotificationChannelsRepository{client: client}
}

// Get 获取通知渠道配置；无数据时返回空切片
func (r *NotificationChannelsRepository) Get() []NotificationChannelData {
	channels := []NotificationChannelData{}
	if r.client == nil {
		return channels
	}
	val, err := r.client.Get(notificationChannelsKey)
	if err != nil || val == "" {
		return channels
	}
	if err := json.Unmarshal([]byte(val), &channels); err != nil {
		return []NotificationChannelData{}
	}
	return channels
}

// Save 保存通知渠道配置
func (r *NotificationChannelsRepository) Save(channels []NotificationChannelData) error {
	if r.client == nil {
		return fmt.Errorf("notification channels repository: redis client not available")
	}
	// 死分支证明：NotificationChannelData 仅含 string/bool 字段，
	// 均为 JSON 可序列化类型，json.Marshal 恒成功
	data, _ := json.Marshal(channels)
	return r.client.Set(notificationChannelsKey, string(data), 0)
}
