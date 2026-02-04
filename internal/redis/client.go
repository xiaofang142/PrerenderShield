package redis

import (
	"context"
	"encoding/json"
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
func NewClient(addr, password string, db int) (*Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		PoolSize:     20,
		MinIdleConns: 10,
		MaxRetries:   3,
		PoolTimeout:  30 * time.Second,
		IdleTimeout:  5 * time.Minute,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &Client{
		client: client,
		ctx:    ctx,
	}, nil
}

// NewClientWithURL 使用URL格式创建Redis客户端
func NewClientWithURL(url string) (*Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis url: %w", err)
	}

	client := redis.NewClient(opts)
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
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

// GetRawClient 获取原始Redis客户端
func (c *Client) GetRawClient() *redis.Client {
	return c.client
}

// Context 获取Redis客户端使用的上下文
func (c *Client) Context() context.Context {
	return c.ctx
}

// Set 设置键值对
func (c *Client) Set(key string, value interface{}, expiration time.Duration) error {
	return c.client.Set(c.ctx, key, value, expiration).Err()
}

// Get 获取键值
func (c *Client) Get(key string) (string, error) {
	val, err := c.client.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return val, nil
}

// Del 删除键
func (c *Client) Del(key string) error {
	return c.client.Del(c.ctx, key).Err()
}

// Exists 检查键是否存在
func (c *Client) Exists(key string) (bool, error) {
	val, err := c.client.Exists(c.ctx, key).Result()
	if err != nil {
		return false, err
	}
	return val > 0, nil
}

// HashSet 设置哈希表字段
func (c *Client) HashSet(key, field string, value interface{}) error {
	return c.client.HSet(c.ctx, key, field, value).Err()
}

// HashGet 获取哈希表字段
func (c *Client) HashGet(key, field string) (string, error) {
	val, err := c.client.HGet(c.ctx, key, field).Result()
	if err == redis.Nil {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return val, nil
}

// HashGetAll 获取哈希表所有字段
func (c *Client) HashGetAll(key string) (map[string]string, error) {
	return c.client.HGetAll(c.ctx, key).Result()
}

// HashSetAll 设置哈希表所有字段
func (c *Client) HashSetAll(key string, values map[string]interface{}) error {
	return c.client.HSet(c.ctx, key, values).Err()
}

// ListPush 向列表尾部添加元素
func (c *Client) ListPush(key string, values ...interface{}) error {
	return c.client.RPush(c.ctx, key, values...).Err()
}

// ListPop 从列表头部移除元素
func (c *Client) ListPop(key string) (string, error) {
	val, err := c.client.LPop(c.ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return val, nil
}

// ListRange 获取列表指定范围的元素
func (c *Client) ListRange(key string, start, stop int64) ([]string, error) {
	return c.client.LRange(c.ctx, key, start, stop).Result()
}

// SetAdd 向集合添加元素
func (c *Client) SetAdd(key string, members ...interface{}) error {
	return c.client.SAdd(c.ctx, key, members...).Err()
}

// SetMembers 获取集合所有元素
func (c *Client) SetMembers(key string) ([]string, error) {
	return c.client.SMembers(c.ctx, key).Result()
}

// SetContains 检查集合是否包含元素
func (c *Client) SetContains(key string, member interface{}) (bool, error) {
	val, err := c.client.SIsMember(c.ctx, key, member).Result()
	if err != nil {
		return false, err
	}
	return val, nil
}

// SetRemove 从集合中移除元素
func (c *Client) SetRemove(key string, members ...interface{}) error {
	return c.client.SRem(c.ctx, key, members...).Err()
}

// Incr 递增计数器
func (c *Client) Incr(key string) (int64, error) {
	return c.client.Incr(c.ctx, key).Result()
}

// Keys 获取匹配模式的键
func (c *Client) Keys(pattern string) ([]string, error) {
	return c.client.Keys(c.ctx, pattern).Result()
}

// DelMultiple 删除多个键
func (c *Client) DelMultiple(keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(c.ctx, keys...).Err()
}

// Decr 递减计数器
func (c *Client) Decr(key string) (int64, error) {
	return c.client.Decr(c.ctx, key).Result()
}

// Expire 设置键过期时间
func (c *Client) Expire(key string, expiration time.Duration) error {
	return c.client.Expire(c.ctx, key, expiration).Err()
}

// TTL 获取键剩余过期时间
func (c *Client) TTL(key string) (time.Duration, error) {
	return c.client.TTL(c.ctx, key).Result()
}

// Publish 发布消息到频道
func (c *Client) Publish(channel string, message interface{}) error {
	return c.client.Publish(c.ctx, channel, message).Err()
}

// Subscribe 订阅频道
func (c *Client) Subscribe(channels ...string) *redis.PubSub {
	return c.client.Subscribe(c.ctx, channels...)
}

// SaveJSON 保存JSON数据
func (c *Client) SaveJSON(key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}
	return c.Set(key, data, expiration)
}

// GetJSON 获取JSON数据
func (c *Client) GetJSON(key string, dest interface{}) error {
	data, err := c.Get(key)
	if err != nil {
		return err
	}
	if data == "" {
		return nil
	}
	return json.Unmarshal([]byte(data), dest)
}

// AddURL 添加URL到站点
func (c *Client) AddURL(siteID, url string) error {
	key := fmt.Sprintf("site:%s:urls", siteID)
	return c.SetAdd(key, url)
}

// RemoveURL 从站点移除URL
func (c *Client) RemoveURL(siteID, url string) error {
	key := fmt.Sprintf("site:%s:urls", siteID)
	return c.SetRemove(key, url)
}

// GetURLs 获取站点所有URL
func (c *Client) GetURLs(siteID string) ([]string, error) {
	key := fmt.Sprintf("site:%s:urls", siteID)
	return c.SetMembers(key)
}

// GetURLCount 获取站点URL数量
func (c *Client) GetURLCount(siteID string) (int64, error) {
	key := fmt.Sprintf("site:%s:urls", siteID)
	return c.client.SCard(c.ctx, key).Result()
}

// SetURLPreheatStatus 设置URL预热状态
func (c *Client) SetURLPreheatStatus(siteID, url, status string, params ...interface{}) error {
	key := fmt.Sprintf("site:%s:preheat:%s", siteID, url)
	return c.Set(key, status, 24*time.Hour)
}

// GetURLPreheatStatusMap 获取URL预热状态（返回map）
func (c *Client) GetURLPreheatStatusMap(siteID, url string) (map[string]string, error) {
	key := fmt.Sprintf("site:%s:preheat:%s", siteID, url)
	val, err := c.Get(key)
	if err != nil {
		return nil, err
	}
	return map[string]string{"status": val}, nil
}

// IsPreheatRunning 检查预热是否正在运行
func (c *Client) IsPreheatRunning(siteID string) (bool, error) {
	key := fmt.Sprintf("site:%s:preheat:running", siteID)
	val, err := c.Get(key)
	if err != nil {
		return false, err
	}
	return val == "1", nil
}

// ClearURLs 清空站点的URL记录
func (c *Client) ClearURLs(siteID string) error {
	key := fmt.Sprintf("site:%s:urls", siteID)
	return c.Del(key)
}

// SetPreheatRunning 设置预热运行状态
func (c *Client) SetPreheatRunning(siteID string, running bool) error {
	key := fmt.Sprintf("site:%s:preheat:running", siteID)
	value := "0"
	if running {
		value = "1"
	}
	return c.Set(key, value, 24*time.Hour)
}

// GetCurrentPreheatTask 获取当前预热任务
func (c *Client) GetCurrentPreheatTask(siteID string) (string, error) {
	key := fmt.Sprintf("site:%s:preheat:current_task", siteID)
	return c.Get(key)
}

// UpdatePreheatTaskProgress 更新预热任务进度
func (c *Client) UpdatePreheatTaskProgress(taskID string, progress int, params ...interface{}) error {
	key := fmt.Sprintf("task:preheat:%s", taskID)
	return c.HashSet(key, "progress", progress)
}

// SetPushTask 设置推送任务
func (c *Client) SetPushTask(siteID string, task map[string]interface{}) error {
	key := fmt.Sprintf("site:%s:push:task", siteID)
	return c.SaveJSON(key, task, 24*time.Hour)
}

// GetPushOffset 获取推送偏移量
func (c *Client) GetPushOffset(siteID string) (int64, error) {
	key := fmt.Sprintf("site:%s:push:offset", siteID)
	value, err := c.Get(key)
	if err != nil || value == "" {
		return 0, nil
	}
	var offset int64
	fmt.Sscanf(value, "%d", &offset)
	return offset, nil
}

// SetPushOffset 设置推送偏移量
func (c *Client) SetPushOffset(siteID string, offset int64) error {
	key := fmt.Sprintf("site:%s:push:offset", siteID)
	return c.Set(key, offset, 24*time.Hour)
}

// SetLastPushDate 设置最后推送日期
func (c *Client) SetLastPushDate(siteID string, date string) error {
	key := fmt.Sprintf("site:%s:push:last_date", siteID)
	return c.Set(key, date, 24*time.Hour)
}

// IncrDailyPushCount 增加每日推送计数
func (c *Client) IncrDailyPushCount(siteID string) error {
	key := fmt.Sprintf("site:%s:push:daily_count", siteID)
	_, err := c.Incr(key)
	return err
}

// IncrPushStats 增加推送统计
func (c *Client) IncrPushStats(siteID string, stat string) error {
	key := fmt.Sprintf("site:%s:push:stats:%s", siteID, stat)
	_, err := c.Incr(key)
	return err
}

// AddPushLog 添加推送日志
func (c *Client) AddPushLog(siteID string, log string) error {
	key := fmt.Sprintf("site:%s:push:logs", siteID)
	return c.ListPush(key, log)
}

// GetURLPreheatStatus 获取URL预热状态
func (c *Client) GetURLPreheatStatus(siteID, url string) (string, error) {
	key := fmt.Sprintf("site:%s:preheat:%s", siteID, url)
	return c.Get(key)
}

// SetSiteStats 设置站点统计信息
func (c *Client) SetSiteStats(siteID string, stats map[string]interface{}) error {
	key := fmt.Sprintf("site:%s:stats", siteID)
	return c.HashSetAll(key, stats)
}

// GetSiteStats 获取站点统计信息
func (c *Client) GetSiteStats(siteID string) (map[string]string, error) {
	key := fmt.Sprintf("site:%s:stats", siteID)
	return c.HashGetAll(key)
}

// GetCacheCount 获取缓存数量
func (c *Client) GetCacheCount() (int64, error) {
	count, err := c.client.Keys(c.ctx, "cache:*").Result()
	if err != nil {
		return 0, err
	}
	return int64(len(count)), nil
}

// ClearCache 清理缓存
func (c *Client) ClearCache() error {
	keys, err := c.client.Keys(c.ctx, "cache:*").Result()
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		return c.client.Del(c.ctx, keys...).Err()
	}
	return nil
}

// CreatePreheatTask 创建预热任务
func (c *Client) CreatePreheatTask(taskID string, task map[string]interface{}) error {
	key := fmt.Sprintf("task:preheat:%s", taskID)
	return c.SaveJSON(key, task, 24*time.Hour)
}



// SaveUser 保存用户信息
func (c *Client) SaveUser(userID string, user map[string]interface{}) error {
	key := fmt.Sprintf("user:%s", userID)
	err := c.HashSetAll(key, user)
	if err != nil {
		return err
	}
	
	// 保存用户名到用户ID的映射
	if username, ok := user["username"].(string); ok {
		usernameKey := fmt.Sprintf("username:%s", username)
		err = c.Set(usernameKey, userID, 0)
	}
	
	return err
}

// SaveUserWithCredentials 保存用户信息（带用户名和密码）
func (c *Client) SaveUserWithCredentials(userID, username, password string) error {
	user := map[string]interface{}{
		"id":       userID,
		"username": username,
		"password": password,
	}
	return c.SaveUser(userID, user)
}

// GetUser 获取用户信息
func (c *Client) GetUser(userID string) (map[string]string, error) {
	key := fmt.Sprintf("user:%s", userID)
	return c.HashGetAll(key)
}

// GetUserByUsername 通过用户名获取用户ID
func (c *Client) GetUserByUsername(username string) (string, error) {
	key := fmt.Sprintf("username:%s", username)
	return c.Get(key)
}

// GetAllUsers 获取所有用户
func (c *Client) GetAllUsers() ([]string, error) {
	keys, err := c.Keys("user:*")
	if err != nil {
		return nil, err
	}
	
	users := make([]string, 0, len(keys))
	for _, key := range keys {
		userID := key[5:] // 移除 "user:" 前缀
		users = append(users, userID)
	}
	
	return users, nil
}

// SaveSession 保存会话信息
func (c *Client) SaveSession(sessionID string, session map[string]interface{}, expiration time.Duration) error {
	key := fmt.Sprintf("session:%s", sessionID)
	return c.SaveJSON(key, session, expiration)
}

// GetSession 获取会话信息
func (c *Client) GetSession(sessionID string, dest interface{}) error {
	key := fmt.Sprintf("session:%s", sessionID)
	return c.GetJSON(key, dest)
}

// CheckSessionExists 检查会话是否存在
func (c *Client) CheckSessionExists(sessionID string) (bool, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	return c.Exists(key)
}

// DeleteSession 删除会话
func (c *Client) DeleteSession(sessionID string) error {
	key := fmt.Sprintf("session:%s", sessionID)
	return c.Del(key)
}

// IncrDailyPushCount 增加每日推送计数（带数量参数）
func (c *Client) IncrDailyPushCountWithCount(siteID string, count int) error {
	key := fmt.Sprintf("site:%s:push:daily_count", siteID)
	_, err := c.client.IncrBy(c.ctx, key, int64(count)).Result()
	return err
}

// IncrPushStats 增加推送统计（带计数参数）
func (c *Client) IncrPushStatsWithCount(siteID string, stat string, count int) error {
	key := fmt.Sprintf("site:%s:push:stats:%s", siteID, stat)
	_, err := c.client.IncrBy(c.ctx, key, int64(count)).Result()
	return err
}

// AddPushLog 添加推送日志（支持结构体）
func (c *Client) AddPushLogStruct(siteID string, log interface{}) error {
	key := fmt.Sprintf("site:%s:push:logs", siteID)
	logJSON, err := json.Marshal(log)
	if err != nil {
		return err
	}
	return c.ListPush(key, string(logJSON))
}

// GetPushStatsWithURLCounts 获取推送统计（包含URL数量）
func (c *Client) GetPushStatsWithURLCounts(siteID string) (map[string]interface{}, error) {
	// 获取URL数量
	urlCount, err := c.GetURLCount(siteID)
	if err != nil {
		urlCount = 0
	}

	// 获取成功和失败统计
	successCount, _ := c.client.Get(c.ctx, fmt.Sprintf("site:%s:push:stats:success", siteID)).Int64()
	failedCount, _ := c.client.Get(c.ctx, fmt.Sprintf("site:%s:push:stats:failed", siteID)).Int64()

	return map[string]interface{}{
		"urlCount":     urlCount,
		"successCount": successCount,
		"failedCount":  failedCount,
		"totalCount":   successCount + failedCount,
	}, nil
}

// GetLast15DaysPushCount 获取最近15天的推送计数
func (c *Client) GetLast15DaysPushCount(siteID string) (map[string]int64, error) {
	result := make(map[string]int64)
	
	// 生成最近15天的日期
	for i := 0; i < 15; i++ {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		key := fmt.Sprintf("site:%s:push:daily:%s", siteID, date)
		count, _ := c.client.Get(c.ctx, key).Int64()
		result[date] = count
	}
	
	return result, nil
}

// GetPushLogs 获取推送日志
func (c *Client) GetPushLogs(siteID string, limit, offset int) ([]interface{}, error) {
	key := fmt.Sprintf("site:%s:push:logs", siteID)
	
	// 获取日志列表
	logs, err := c.client.LRange(c.ctx, key, int64(offset), int64(offset+limit-1)).Result()
	if err != nil {
		return nil, err
	}
	
	// 转换为interface{}切片
	result := make([]interface{}, len(logs))
	for i, log := range logs {
		var logMap map[string]interface{}
		if err := json.Unmarshal([]byte(log), &logMap); err == nil {
			result[i] = logMap
		} else {
			result[i] = log
		}
	}
	
	return result, nil
}

// DeleteSiteData 删除站点数据
func (c *Client) DeleteSiteData(siteID string) error {
	keys := []string{
		fmt.Sprintf("site:%s:*", siteID),
		fmt.Sprintf("cache:%s:*", siteID),
		fmt.Sprintf("task:preheat:%s:*", siteID),
	}
	for _, pattern := range keys {
		siteKeys, err := c.client.Keys(c.ctx, pattern).Result()
		if err != nil {
			return err
		}
		if len(siteKeys) > 0 {
			if err := c.client.Del(c.ctx, siteKeys...).Err(); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetSystemConfig 获取系统配置
func (c *Client) GetSystemConfig() (map[string]string, error) {
	key := "system:config"
	return c.HashGetAll(key)
}

// SaveSystemConfig 保存系统配置
func (c *Client) SaveSystemConfig(config map[string]interface{}) error {
	key := "system:config"
	return c.HashSetAll(key, config)
}
