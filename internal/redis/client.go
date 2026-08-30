package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// PoolConfig Redis 连接池配置
type PoolConfig struct {
	MaxActive   int           // 最大连接数 (PoolSize)
	MaxIdle     int           // 最大空闲连接数 (MinIdleConns)
	IdleTimeout time.Duration // 空闲连接超时
	PoolTimeout time.Duration // 获取连接超时
}

// DefaultPoolConfig 默认连接池配置
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxActive:   20,
		MaxIdle:     10,
		IdleTimeout: 5 * time.Minute,
		PoolTimeout: 30 * time.Second,
	}
}

// Client Redis 客户端结构体
type Client struct {
	client         *redis.Client
	ctx            context.Context
	circuitBreaker *CircuitBreaker
}

// NewClient 创建新的 Redis 客户端
func NewClient(addr, password string, db int) (*Client, error) {
	return NewClientWithConfig(addr, password, db, DefaultCircuitBreakerConfig())
}

// NewClientWithPool 创建带连接池配置的 Redis 客户端
func NewClientWithPool(addr, password string, db int, pool PoolConfig) (*Client, error) {
	return NewClientWithFullConfig(addr, password, db, pool, DefaultCircuitBreakerConfig())
}

// NewClientWithConfig 创建带熔断器配置的 Redis 客户端
func NewClientWithConfig(addr, password string, db int, cbConfig CircuitBreakerConfig) (*Client, error) {
	return NewClientWithFullConfig(addr, password, db, DefaultPoolConfig(), cbConfig)
}

// NewClientWithFullConfig 创建带完整配置的 Redis 客户端
func NewClientWithFullConfig(addr, password string, db int, pool PoolConfig, cbConfig CircuitBreakerConfig) (*Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		PoolSize:     pool.MaxActive,
		MinIdleConns: pool.MaxIdle,
		MaxRetries:   3,
		PoolTimeout:  pool.PoolTimeout,
		IdleTimeout:  pool.IdleTimeout,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &Client{
		client:         client,
		ctx:            ctx,
		circuitBreaker: NewCircuitBreaker(cbConfig),
	}, nil
}

// NewClientWithURL 使用 URL 格式创建 Redis 客户端
func NewClientWithURL(url string) (*Client, error) {
	return NewClientWithURLAndConfig(url, DefaultCircuitBreakerConfig())
}

// NewClientWithURLAndConfig 使用 URL 和熔断器配置创建 Redis 客户端
func NewClientWithURLAndConfig(url string, cbConfig CircuitBreakerConfig) (*Client, error) {
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
		client:         client,
		ctx:            ctx,
		circuitBreaker: NewCircuitBreaker(cbConfig),
	}, nil
}

// Close 关闭 Redis 连接
func (c *Client) Close() error {
	return c.client.Close()
}

// GetRawClient 获取原始 Redis 客户端
func (c *Client) GetRawClient() *redis.Client {
	return c.client
}

// Context 获取 Redis 客户端使用的上下文
func (c *Client) Context() context.Context {
	return c.ctx
}

// GetCircuitBreaker 获取熔断器
func (c *Client) GetCircuitBreaker() *CircuitBreaker {
	return c.circuitBreaker
}

// checkCircuitBreaker 检查熔断器状态
func (c *Client) checkCircuitBreaker() error {
	if c.circuitBreaker == nil {
		return nil
	}
	if !c.circuitBreaker.Allow() {
		return ErrCircuitBreakerOpen
	}
	return nil
}

// recordSuccess 记录成功
func (c *Client) recordSuccess() {
	if c.circuitBreaker != nil {
		c.circuitBreaker.RecordSuccess()
	}
}

// recordFailure 记录失败
func (c *Client) recordFailure() {
	if c.circuitBreaker != nil {
		c.circuitBreaker.RecordFailure()
	}
}

// Set 设置键值对
func (c *Client) Set(key string, value interface{}, expiration time.Duration) error {
	if err := c.checkCircuitBreaker(); err != nil {
		return err
	}
	err := c.client.Set(c.ctx, key, value, expiration).Err()
	if err != nil {
		c.recordFailure()
		return err
	}
	c.recordSuccess()
	return nil
}

// Get 获取键值
func (c *Client) Get(key string) (string, error) {
	if err := c.checkCircuitBreaker(); err != nil {
		return "", err
	}
	val, err := c.client.Get(c.ctx, key).Result()
	if err == redis.Nil {
		c.recordSuccess()
		return "", nil
	} else if err != nil {
		c.recordFailure()
		return "", err
	}
	c.recordSuccess()
	return val, nil
}

// Del 删除键
func (c *Client) Del(key string) error {
	if err := c.checkCircuitBreaker(); err != nil {
		return err
	}
	err := c.client.Del(c.ctx, key).Err()
	if err != nil {
		c.recordFailure()
		return err
	}
	c.recordSuccess()
	return nil
}

// ZAdd 向有序集合添加成员（score 为排序权重）
func (c *Client) ZAdd(key string, score float64, member string) error {
	if err := c.checkCircuitBreaker(); err != nil {
		return err
	}
	err := c.client.ZAdd(c.ctx, key, &redis.Z{Score: score, Member: member}).Err()
	if err != nil {
		c.recordFailure()
		return err
	}
	c.recordSuccess()
	return nil
}

// ZRevRange 按分数从高到低返回有序集合成员
func (c *Client) ZRevRange(key string, start, stop int64) ([]string, error) {
	if err := c.checkCircuitBreaker(); err != nil {
		return nil, err
	}
	vals, err := c.client.ZRevRange(c.ctx, key, start, stop).Result()
	if err != nil {
		c.recordFailure()
		return nil, err
	}
	c.recordSuccess()
	return vals, nil
}

// ZRemRangeByScore 移除有序集合中分数区间内的成员，返回移除数量
func (c *Client) ZRemRangeByScore(key, min, max string) (int64, error) {
	if err := c.checkCircuitBreaker(); err != nil {
		return 0, err
	}
	n, err := c.client.ZRemRangeByScore(c.ctx, key, min, max).Result()
	if err != nil {
		c.recordFailure()
		return 0, err
	}
	c.recordSuccess()
	return n, nil
}

// Exists 检查键是否存在
func (c *Client) Exists(key string) (bool, error) {
	if err := c.checkCircuitBreaker(); err != nil {
		return false, err
	}
	val, err := c.client.Exists(c.ctx, key).Result()
	if err != nil {
		c.recordFailure()
		return false, err
	}
	c.recordSuccess()
	return val > 0, nil
}

// HashSet 设置哈希表字段
func (c *Client) HashSet(key, field string, value interface{}) error {
	if err := c.checkCircuitBreaker(); err != nil {
		return err
	}
	err := c.client.HSet(c.ctx, key, field, value).Err()
	if err != nil {
		c.recordFailure()
		return err
	}
	c.recordSuccess()
	return nil
}

// HashGet 获取哈希表字段
func (c *Client) HashGet(key, field string) (string, error) {
	if err := c.checkCircuitBreaker(); err != nil {
		return "", err
	}
	val, err := c.client.HGet(c.ctx, key, field).Result()
	if err == redis.Nil {
		c.recordSuccess()
		return "", nil
	} else if err != nil {
		c.recordFailure()
		return "", err
	}
	c.recordSuccess()
	return val, nil
}

// HashIncrBy 哈希字段原子自增（不存在时从 0 开始），返回自增后的值。
// 用于并发计数场景（如缓存命中数），替代非原子的 HashGet+HashSet 读改写
func (c *Client) HashIncrBy(key, field string, incr int64) (int64, error) {
	if err := c.checkCircuitBreaker(); err != nil {
		return 0, err
	}
	val, err := c.client.HIncrBy(c.ctx, key, field, incr).Result()
	if err != nil {
		c.recordFailure()
		return 0, err
	}
	c.recordSuccess()
	return val, nil
}

// HashGetAll 获取哈希表所有字段
func (c *Client) HashGetAll(key string) (map[string]string, error) {
	if err := c.checkCircuitBreaker(); err != nil {
		return nil, err
	}
	result, err := c.client.HGetAll(c.ctx, key).Result()
	if err != nil {
		c.recordFailure()
		return nil, err
	}
	c.recordSuccess()
	return result, nil
}

// HashSetAll 设置哈希表所有字段
func (c *Client) HashSetAll(key string, values map[string]interface{}) error {
	if err := c.checkCircuitBreaker(); err != nil {
		return err
	}
	err := c.client.HSet(c.ctx, key, values).Err()
	if err != nil {
		c.recordFailure()
		return err
	}
	c.recordSuccess()
	return nil
}

// ListPush 向列表尾部添加元素
func (c *Client) ListPush(key string, values ...interface{}) error {
	if err := c.checkCircuitBreaker(); err != nil {
		return err
	}
	err := c.client.RPush(c.ctx, key, values...).Err()
	if err != nil {
		c.recordFailure()
		return err
	}
	c.recordSuccess()
	return nil
}

// ListPop 从列表头部移除元素
func (c *Client) ListPop(key string) (string, error) {
	if err := c.checkCircuitBreaker(); err != nil {
		return "", err
	}
	val, err := c.client.LPop(c.ctx, key).Result()
	if err == redis.Nil {
		c.recordSuccess()
		return "", nil
	} else if err != nil {
		c.recordFailure()
		return "", err
	}
	c.recordSuccess()
	return val, nil
}

// ListRange 获取列表指定范围的元素
func (c *Client) ListRange(key string, start, stop int64) ([]string, error) {
	if err := c.checkCircuitBreaker(); err != nil {
		return nil, err
	}
	result, err := c.client.LRange(c.ctx, key, start, stop).Result()
	if err != nil {
		c.recordFailure()
		return nil, err
	}
	c.recordSuccess()
	return result, nil
}

// ListLength 获取列表长度
func (c *Client) ListLength(key string) (int64, error) {
	if err := c.checkCircuitBreaker(); err != nil {
		return 0, err
	}
	length, err := c.client.LLen(c.ctx, key).Result()
	if err != nil {
		c.recordFailure()
		return 0, err
	}
	c.recordSuccess()
	return length, nil
}

// GetPushLogCount 获取推送日志总数
func (c *Client) GetPushLogCount(siteID string) (int64, error) {
	key := fmt.Sprintf("site:%s:push:logs", siteID)
	return c.ListLength(key)
}

// SetAdd 向集合添加元素
func (c *Client) SetAdd(key string, members ...interface{}) error {
	if err := c.checkCircuitBreaker(); err != nil {
		return err
	}
	err := c.client.SAdd(c.ctx, key, members...).Err()
	if err != nil {
		c.recordFailure()
		return err
	}
	c.recordSuccess()
	return nil
}

// SetMembers 获取集合所有元素
func (c *Client) SetMembers(key string) ([]string, error) {
	if err := c.checkCircuitBreaker(); err != nil {
		return nil, err
	}
	result, err := c.client.SMembers(c.ctx, key).Result()
	if err != nil {
		c.recordFailure()
		return nil, err
	}
	c.recordSuccess()
	return result, nil
}

// SetContains 检查集合是否包含元素
func (c *Client) SetContains(key string, member interface{}) (bool, error) {
	if err := c.checkCircuitBreaker(); err != nil {
		return false, err
	}
	val, err := c.client.SIsMember(c.ctx, key, member).Result()
	if err != nil {
		c.recordFailure()
		return false, err
	}
	c.recordSuccess()
	return val, nil
}

// SetRemove 从集合中移除元素
func (c *Client) SetRemove(key string, members ...interface{}) error {
	if err := c.checkCircuitBreaker(); err != nil {
		return err
	}
	err := c.client.SRem(c.ctx, key, members...).Err()
	if err != nil {
		c.recordFailure()
		return err
	}
	c.recordSuccess()
	return nil
}

// Incr 递增计数器
func (c *Client) Incr(key string) (int64, error) {
	if err := c.checkCircuitBreaker(); err != nil {
		return 0, err
	}
	val, err := c.client.Incr(c.ctx, key).Result()
	if err != nil {
		c.recordFailure()
		return 0, err
	}
	c.recordSuccess()
	return val, nil
}

// Keys 获取匹配模式的键（使用 SCAN 替代 KEYS 避免阻塞 Redis）
func (c *Client) Keys(pattern string) ([]string, error) {
	if err := c.checkCircuitBreaker(); err != nil {
		return nil, err
	}
	var result []string
	var cursor uint64
	for {
		keys, nextCursor, err := c.client.Scan(c.ctx, cursor, pattern, 100).Result()
		if err != nil {
			c.recordFailure()
			return nil, err
		}
		result = append(result, keys...)
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	c.recordSuccess()
	return result, nil
}

// DelMultiple 删除多个键
func (c *Client) DelMultiple(keys []string) error {
	if err := c.checkCircuitBreaker(); err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	err := c.client.Del(c.ctx, keys...).Err()
	if err != nil {
		c.recordFailure()
		return err
	}
	c.recordSuccess()
	return nil
}

// Decr 递减计数器
func (c *Client) Decr(key string) (int64, error) {
	if err := c.checkCircuitBreaker(); err != nil {
		return 0, err
	}
	val, err := c.client.Decr(c.ctx, key).Result()
	if err != nil {
		c.recordFailure()
		return 0, err
	}
	c.recordSuccess()
	return val, nil
}

// Expire 设置键过期时间
func (c *Client) Expire(key string, expiration time.Duration) error {
	if err := c.checkCircuitBreaker(); err != nil {
		return err
	}
	err := c.client.Expire(c.ctx, key, expiration).Err()
	if err != nil {
		c.recordFailure()
		return err
	}
	c.recordSuccess()
	return nil
}

// TTL 获取键剩余过期时间
func (c *Client) TTL(key string) (time.Duration, error) {
	if err := c.checkCircuitBreaker(); err != nil {
		return 0, err
	}
	val, err := c.client.TTL(c.ctx, key).Result()
	if err != nil {
		c.recordFailure()
		return 0, err
	}
	c.recordSuccess()
	return val, nil
}

// Publish 发布消息到频道
func (c *Client) Publish(channel string, message interface{}) error {
	if err := c.checkCircuitBreaker(); err != nil {
		return err
	}
	err := c.client.Publish(c.ctx, channel, message).Err()
	if err != nil {
		c.recordFailure()
		return err
	}
	c.recordSuccess()
	return nil
}

// Subscribe 订阅频道（返回 PubSub，调用方必须在完成后调用 Close()）
func (c *Client) Subscribe(channels ...string) *redis.PubSub {
	if err := c.checkCircuitBreaker(); err != nil {
		return nil
	}
	return c.client.Subscribe(c.ctx, channels...)
}

// SubscribeWithContext 使用指定 context 订阅频道
func (c *Client) SubscribeWithContext(ctx context.Context, channels ...string) *redis.PubSub {
	return c.client.Subscribe(ctx, channels...)
}

// SaveJSON 保存 JSON 数据
func (c *Client) SaveJSON(key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}
	return c.Set(key, data, expiration)
}

// GetJSON 获取 JSON 数据
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

// AddURL 添加 URL 到站点
func (c *Client) AddURL(siteID, url string) error {
	key := fmt.Sprintf("site:%s:urls", siteID)
	return c.SetAdd(key, url)
}

// RemoveURL 从站点移除 URL
func (c *Client) RemoveURL(siteID, url string) error {
	key := fmt.Sprintf("site:%s:urls", siteID)
	return c.SetRemove(key, url)
}

// GetURLs 获取站点所有 URL
func (c *Client) GetURLs(siteID string) ([]string, error) {
	key := fmt.Sprintf("site:%s:urls", siteID)
	return c.SetMembers(key)
}

// GetURLCount 获取站点 URL 数量
func (c *Client) GetURLCount(siteID string) (int64, error) {
	if err := c.checkCircuitBreaker(); err != nil {
		return 0, err
	}
	key := fmt.Sprintf("site:%s:urls", siteID)
	val, err := c.client.SCard(c.ctx, key).Result()
	if err != nil {
		c.recordFailure()
		return 0, err
	}
	c.recordSuccess()
	return val, nil
}

// SetURLPreheatStatus 设置 URL 预热状态
func (c *Client) SetURLPreheatStatus(siteID, url, status string, params ...interface{}) error {
	key := fmt.Sprintf("site:%s:preheat:%s", siteID, url)
	return c.Set(key, status, 24*time.Hour)
}

// GetURLPreheatStatusMap 获取 URL 预热状态（返回 map）
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
	if err := c.checkCircuitBreaker(); err != nil {
		return false, err
	}
	key := fmt.Sprintf("site:%s:preheat:running", siteID)
	val, err := c.Get(key)
	if err != nil {
		return false, err
	}
	return val == "1", nil
}

// ClearURLs 清空站点的 URL 记录
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
	if err := c.checkCircuitBreaker(); err != nil {
		return "", err
	}
	key := fmt.Sprintf("site:%s:preheat:current_task", siteID)
	return c.Get(key)
}

// UpdatePreheatTaskProgress 更新预热任务进度
func (c *Client) UpdatePreheatTaskProgress(taskID string, progress int, params ...interface{}) error {
	if err := c.checkCircuitBreaker(); err != nil {
		return err
	}
	key := fmt.Sprintf("task:preheat:%s", taskID)
	err := c.HashSet(key, "progress", progress)
	if err != nil {
		return err
	}
	return nil
}

// SetPushTask 设置推送任务
func (c *Client) SetPushTask(siteID string, task map[string]interface{}) error {
	key := fmt.Sprintf("site:%s:push:task", siteID)
	return c.SaveJSON(key, task, 24*time.Hour)
}

// GetPushOffset 获取推送偏移量
func (c *Client) GetPushOffset(siteID string) (int64, error) {
	if err := c.checkCircuitBreaker(); err != nil {
		return 0, err
	}
	key := fmt.Sprintf("site:%s:push:offset", siteID)
	value, err := c.Get(key)
	if err != nil || value == "" {
		return 0, nil
	}
	var offset int64
	if _, err := fmt.Sscanf(value, "%d", &offset); err != nil {
		return 0, nil
	}
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

// GetURLPreheatStatus 获取 URL 预热状态
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

// GetCacheCount 获取缓存数量（SCAN 增量遍历，避免 KEYS 阻塞 Redis）
func (c *Client) GetCacheCount() (int64, error) {
	keys, err := c.Keys("cache:*")
	if err != nil {
		return 0, err
	}
	return int64(len(keys)), nil
}

// ClearCache 清理全部渲染缓存（SCAN 增量遍历）
func (c *Client) ClearCache() error {
	keys, err := c.Keys("cache:*")
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		return c.DelMultiple(keys)
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

	// 保存用户名到用户 ID 的映射
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

// GetUserByUsername 通过用户名获取用户 ID
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

// IncrDailyPushCountWithCount 增加每日推送计数（带数量参数）
func (c *Client) IncrDailyPushCountWithCount(siteID string, count int) error {
	key := fmt.Sprintf("site:%s:push:daily_count", siteID)
	_, err := c.client.IncrBy(c.ctx, key, int64(count)).Result()
	if err != nil {
		c.recordFailure()
		return err
	}
	c.recordSuccess()
	return nil
}

// IncrPushStatsWithCount 增加推送统计（带计数参数）
func (c *Client) IncrPushStatsWithCount(siteID string, stat string, count int) error {
	key := fmt.Sprintf("site:%s:push:stats:%s", siteID, stat)
	_, err := c.client.IncrBy(c.ctx, key, int64(count)).Result()
	if err != nil {
		c.recordFailure()
		return err
	}
	c.recordSuccess()
	return nil
}

// AddPushLogStruct 添加推送日志（支持结构体）
func (c *Client) AddPushLogStruct(siteID string, log interface{}) error {
	key := fmt.Sprintf("site:%s:push:logs", siteID)
	logJSON, err := json.Marshal(log)
	if err != nil {
		return err
	}
	return c.ListPush(key, string(logJSON))
}

// GetPushStatsWithURLCounts 获取推送统计（包含 URL 数量）
func (c *Client) GetPushStatsWithURLCounts(siteID string) (map[string]interface{}, error) {
	// 获取 URL 数量
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

// GetLast15DaysPushCount 获取最近 15 天的推送计数
func (c *Client) GetLast15DaysPushCount(siteID string) (map[string]int64, error) {
	result := make(map[string]int64)

	// 生成最近 15 天的日期
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
	if err := c.checkCircuitBreaker(); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("site:%s:push:logs", siteID)

	// 获取日志列表
	logs, err := c.client.LRange(c.ctx, key, int64(offset), int64(offset+limit-1)).Result()
	if err != nil {
		c.recordFailure()
		return nil, err
	}
	c.recordSuccess()

	// 转换为 interface{} 切片
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
	if err := c.checkCircuitBreaker(); err != nil {
		return err
	}
	keys := []string{
		fmt.Sprintf("site:%s:*", siteID),
		fmt.Sprintf("cache:%s:*", siteID),
		fmt.Sprintf("task:preheat:%s:*", siteID),
	}
	for _, pattern := range keys {
		siteKeys, err := c.Keys(pattern)
		if err != nil {
			return err
		}
		if len(siteKeys) > 0 {
			if err := c.client.Del(c.ctx, siteKeys...).Err(); err != nil {
				c.recordFailure()
				return err
			}
		}
	}
	c.recordSuccess()
	return nil
}

// GetSystemConfig 获取系统配置
func (c *Client) GetSystemConfig() (map[string]string, error) {
	key := "system:config"
	val, err := c.Get(key)
	if err != nil {
		return nil, err
	}
	if val == "" {
		return map[string]string{}, nil
	}
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(val), &config); err != nil {
		return nil, err
	}
	result := make(map[string]string, len(config))
	for k, v := range config {
		switch val := v.(type) {
		case string:
			result[k] = val
		default:
			b, _ := json.Marshal(val)
			result[k] = string(b)
		}
	}
	return result, nil
}

// SaveSystemConfig 保存系统配置（带基础校验）
func (c *Client) SaveSystemConfig(config map[string]interface{}) error {
	// 基础校验：不允许覆盖关键系统字段
	blockedKeys := map[string]bool{
		"jwt_secret":     true,
		"redis_url":      true,
		"admin_password": true,
	}
	for k := range config {
		if blockedKeys[k] {
			return fmt.Errorf("system config key '%s' is read-only and cannot be modified via API", k)
		}
	}

	key := "system:config"
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal system config: %w", err)
	}
	return c.Set(key, string(data), 0)
}
