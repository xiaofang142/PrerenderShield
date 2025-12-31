package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// Client Redis客户端结构体
type Client struct {
	client *redis.Client
	ctx    context.Context
}

// NewClient 创建新的Redis客户端
func NewClient(redisURL string) (*Client, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %v", err)
	}

	client := redis.NewClient(opt)
	ctx := context.Background()

	// 测试连接
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %v", err)
	}

	return &Client{
		client: client,
		ctx:    ctx,
	}, nil
}

// Close 关闭Redis连接
func (c *Client) Close() error {
	return c.client.Close()
}

// AddURL 添加URL到站点的URL集合
func (c *Client) AddURL(siteName, url string) error {
	key := fmt.Sprintf("prerender:%s:urls", siteName)
	return c.client.SAdd(c.ctx, key, url).Err()
}

// RemoveURL 从站点的URL集合中移除URL
func (c *Client) RemoveURL(siteName, url string) error {
	key := fmt.Sprintf("prerender:%s:urls", siteName)
	return c.client.SRem(c.ctx, key, url).Err()
}

// GetURLs 获取站点的所有URL
func (c *Client) GetURLs(siteName string) ([]string, error) {
	key := fmt.Sprintf("prerender:%s:urls", siteName)
	return c.client.SMembers(c.ctx, key).Result()
}

// GetURLCount 获取站点的URL数量
func (c *Client) GetURLCount(siteName string) (int64, error) {
	key := fmt.Sprintf("prerender:%s:urls", siteName)
	return c.client.SCard(c.ctx, key).Result()
}

// ClearURLs 清空站点的所有URL
func (c *Client) ClearURLs(siteName string) error {
	key := fmt.Sprintf("prerender:%s:urls", siteName)
	return c.client.Del(c.ctx, key).Err()
}

// SetURLPreheatStatus 设置URL的预热状态
func (c *Client) SetURLPreheatStatus(siteName, url, status string, cacheSize int64) error {
	key := fmt.Sprintf("prerender:%s:url:%s", siteName, url)
	return c.client.HSet(c.ctx, key, map[string]interface{}{
		"status":     status,
		"cache_size": cacheSize,
		"updated_at": time.Now().Unix(),
	}).Err()
}

// GetURLPreheatStatus 获取URL的预热状态
func (c *Client) GetURLPreheatStatus(siteName, url string) (map[string]string, error) {
	key := fmt.Sprintf("prerender:%s:url:%s", siteName, url)
	return c.client.HGetAll(c.ctx, key).Result()
}

// SetSiteStats 设置站点的统计数据
func (c *Client) SetSiteStats(siteName string, stats map[string]interface{}) error {
	key := fmt.Sprintf("prerender:%s:stats", siteName)
	return c.client.HSet(c.ctx, key, stats).Err()
}

// GetSiteStats 获取站点的统计数据
func (c *Client) GetSiteStats(siteName string) (map[string]string, error) {
	key := fmt.Sprintf("prerender:%s:stats", siteName)
	return c.client.HGetAll(c.ctx, key).Result()
}

// GetCacheCount 获取站点的缓存数量
func (c *Client) GetCacheCount(siteName string) (int64, error) {
	// 使用SCAN命令统计所有状态为cached的URL数量
	keyPrefix := fmt.Sprintf("prerender:%s:url:", siteName)
	count := int64(0)
	
	iter := c.client.Scan(c.ctx, 0, keyPrefix+"*", 0).Iterator()
	for iter.Next(c.ctx) {
		status, err := c.client.HGet(c.ctx, iter.Val(), "status").Result()
		if err == nil && status == "cached" {
			count++
		}
	}
	
	if err := iter.Err(); err != nil {
		return 0, err
	}
	
	return count, nil
}

// SetPreheatRunning 设置预热任务运行状态
func (c *Client) SetPreheatRunning(siteName string, running bool) error {
	key := fmt.Sprintf("prerender:%s:status", siteName)
	return c.client.Set(c.ctx, key, running, 0).Err()
}

// IsPreheatRunning 检查预热任务是否正在运行
func (c *Client) IsPreheatRunning(siteName string) (bool, error) {
	key := fmt.Sprintf("prerender:%s:status", siteName)
	val, err := c.client.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return val == "true", nil
}
