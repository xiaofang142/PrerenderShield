package redis

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

// Client Redis客户端结构体
// 封装了Redis客户端的核心功能，提供了与应用相关的Redis操作方法
//
// 字段:
//   client: 底层的Redis客户端实例
//   ctx: 上下文，用于管理Redis操作的生命周期

type Client struct {
	client *redis.Client
	ctx    context.Context
}

// NewClient 创建新的Redis客户端
// 支持两种格式的Redis URL:
// 1. 简单格式: localhost:6379
// 2. URL格式: redis://[password@]host:port/db
//
// 参数:
//   redisURL: Redis连接URL
//
// 返回值:
//   *Client: 创建的Redis客户端实例
//   error: 如果创建失败，返回错误信息
//
// 示例:
//   client, err := redis.NewClient("localhost:6379")
//   client, err := redis.NewClient("redis://password@localhost:6379/0")
func NewClient(redisURL string) (*Client, error) {
	// 直接创建Redis客户端选项，不使用ParseURL
	opt := &redis.Options{}

	// 如果redisURL是纯主机名或IP地址，使用默认端口
	if !strings.Contains(redisURL, "://") {
		opt.Addr = redisURL
		if !strings.Contains(opt.Addr, ":") {
			opt.Addr = fmt.Sprintf("%s:6379", opt.Addr)
		}
	} else {
		// 否则尝试解析URL
		parsed, err := url.Parse(redisURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse redis URL: %v", err)
		}

		opt.Addr = parsed.Host
		if !strings.Contains(opt.Addr, ":") {
			opt.Addr = fmt.Sprintf("%s:6379", opt.Addr)
		}

		// 解析密码
		if parsed.User != nil {
			opt.Password, _ = parsed.User.Password()
		}

		// 解析数据库
		db := 0
		if parsed.Path != "" && parsed.Path != "/" {
			fmt.Sscanf(parsed.Path[1:], "%d", &db)
		}
		opt.DB = db
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

// GetRawClient 获取原始Redis客户端实例
func (c *Client) GetRawClient() *redis.Client {
	return c.client
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

// GetUser 获取用户信息
func (c *Client) GetUser(userID string) (map[string]string, error) {
	key := "user:" + userID
	return c.client.HGetAll(c.ctx, key).Result()
}

// GetUserByUsername 通过用户名获取用户ID
func (c *Client) GetUserByUsername(username string) (string, error) {
	key := "username:" + username
	return c.client.Get(c.ctx, key).Result()
}

// GetAllUsers 获取所有用户ID
func (c *Client) GetAllUsers() ([]string, error) {
	keys, err := c.client.Keys(c.ctx, "user:*").Result()
	if err != nil {
		return nil, err
	}
	// 提取用户ID
	userIDs := make([]string, len(keys))
	for i, key := range keys {
		userIDs[i] = key[5:] // 去掉 "user:" 前缀
	}
	return userIDs, nil
}

// SaveUser 保存用户信息到Redis
func (c *Client) SaveUser(userID, username, password string) error {
	// 将用户信息保存到Redis，使用hash结构
	userKey := "user:" + userID
	if err := c.client.HSet(c.ctx, userKey, map[string]interface{}{
		"id":       userID,
		"username": username,
		"password": password,
	}).Err(); err != nil {
		return err
	}

	// 创建用户名到用户ID的映射
	return c.client.Set(c.ctx, "username:"+username, userID, 0).Err()
}
