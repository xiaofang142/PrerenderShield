package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"prerender-shield/internal/logging"

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
//
//	redisURL: Redis连接URL
//
// 返回值:
//
//	*Client: 创建的Redis客户端实例
//	error: 如果创建失败，返回错误信息
//
// 示例:
//
//	client, err := redis.NewClient("localhost:6379")
//	client, err := redis.NewClient("redis://password@localhost:6379/0")
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

// Context 获取上下文
func (c *Client) Context() context.Context {
	return c.ctx
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
func (c *Client) AddURL(siteID, url string) error {
	key := fmt.Sprintf("prerender:%s:urls", siteID)
	return c.client.SAdd(c.ctx, key, url).Err()
}

// RemoveURL 从站点的URL集合中移除URL
func (c *Client) RemoveURL(siteID, url string) error {
	key := fmt.Sprintf("prerender:%s:urls", siteID)
	return c.client.SRem(c.ctx, key, url).Err()
}

// GetURLs 获取站点的所有URL
func (c *Client) GetURLs(siteID string) ([]string, error) {
	key := fmt.Sprintf("prerender:%s:urls", siteID)
	urls, err := c.client.SMembers(c.ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get URLs for site %s: %v", siteID, err)
	}
	return urls, nil
}

// GetURLCount 获取站点的URL数量
func (c *Client) GetURLCount(siteID string) (int64, error) {
	key := fmt.Sprintf("prerender:%s:urls", siteID)
	count, err := c.client.SCard(c.ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get URL count for site %s: %v", siteID, err)
	}
	return count, nil
}

// ClearURLs 清空站点的所有URL
func (c *Client) ClearURLs(siteID string) error {
	key := fmt.Sprintf("prerender:%s:urls", siteID)
	if err := c.client.Del(c.ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to clear URLs for site %s: %v", siteID, err)
	}
	return nil
}

// SetURLPreheatStatus 设置URL的预热状态
func (c *Client) SetURLPreheatStatus(siteID, url, status string, cacheSize int64) error {
	key := fmt.Sprintf("prerender:%s:url:%s", siteID, url)
	return c.client.HSet(c.ctx, key, map[string]interface{}{
		"status":     status,
		"cache_size": cacheSize,
		"updated_at": time.Now().Unix(),
	}).Err()
}

// GetURLPreheatStatus 获取URL的预热状态
func (c *Client) GetURLPreheatStatus(siteID, url string) (map[string]string, error) {
	key := fmt.Sprintf("prerender:%s:url:%s", siteID, url)
	return c.client.HGetAll(c.ctx, key).Result()
}

// SetSiteStats 设置站点的统计数据
func (c *Client) SetSiteStats(siteID string, stats map[string]interface{}) error {
	key := fmt.Sprintf("prerender:%s:stats", siteID)

	// 将map转换为键值对切片，并确保所有值都是基本类型
	var values []interface{}
	for k, v := range stats {
		// 根据值的类型进行转换
		switch val := v.(type) {
		case bool:
			// 将bool转换为0或1
			if val {
				values = append(values, k, 1)
			} else {
				values = append(values, k, 0)
			}
		case map[string]interface{}:
			// 跳过嵌套map，避免序列化问题
			continue
		default:
			// 其他基本类型直接使用
			values = append(values, k, val)
		}
	}

	// 使用键值对切片调用HSet
	if len(values) > 0 {
		return c.client.HSet(c.ctx, key, values...).Err()
	}
	return nil
}

// GetSiteStats 获取站点的统计数据
func (c *Client) GetSiteStats(siteID string) (map[string]string, error) {
	key := fmt.Sprintf("prerender:%s:stats", siteID)
	return c.client.HGetAll(c.ctx, key).Result()
}

// GetCacheCount 获取站点的缓存数量
func (c *Client) GetCacheCount(siteID string) (int64, error) {
	// 使用SCAN命令统计所有状态为cached的URL数量
	keyPrefix := fmt.Sprintf("prerender:%s:url:", siteID)
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

// ClearCache 清除站点的所有缓存
func (c *Client) ClearCache(siteID string) (int64, error) {
	// 使用SCAN命令遍历所有以"prerender:{siteID}:url:"为前缀的键
	keyPrefix := fmt.Sprintf("prerender:%s:url:", siteID)
	count := int64(0)

	iter := c.client.Scan(c.ctx, 0, keyPrefix+"*", 100).Iterator() // 使用100作为每次扫描的数量限制
	for iter.Next(c.ctx) {
		// 删除该URL的预热状态，实现清除缓存
		if err := c.client.Del(c.ctx, iter.Val()).Err(); err != nil {
			return count, fmt.Errorf("failed to delete cache key %s for site %s: %v", iter.Val(), siteID, err)
		}
		count++
	}

	if err := iter.Err(); err != nil {
		return count, fmt.Errorf("failed to scan cache keys for site %s: %v", siteID, err)
	}

	return count, nil
}

// SetPreheatRunning 设置预热任务运行状态
func (c *Client) SetPreheatRunning(siteID string, running bool) error {
	key := fmt.Sprintf("prerender:%s:status", siteID)
	return c.client.Set(c.ctx, key, running, 0).Err()
}

// IsPreheatRunning 检查预热任务是否正在运行
func (c *Client) IsPreheatRunning(siteID string) (bool, error) {
	key := fmt.Sprintf("prerender:%s:status", siteID)
	val, err := c.client.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return val == "true", nil
}

// CreatePreheatTask 创建预热任务并返回任务ID
func (c *Client) CreatePreheatTask(siteID string) (string, error) {
	taskID := fmt.Sprintf("%s:%d", siteID, time.Now().UnixNano())
	key := fmt.Sprintf("prerender:%s:task:%s", siteID, taskID)

	// 初始化任务状态
	err := c.client.HSet(c.ctx, key, map[string]interface{}{
		"site_name":  siteID, // Use ID as site identifier
		"task_id":    taskID,
		"status":     "running",
		"total_urls": 0,
		"processed":  0,
		"success":    0,
		"failed":     0,
		"created_at": time.Now().Unix(),
		"updated_at": time.Now().Unix(),
	}).Err()

	if err != nil {
		return "", err
	}

	// 设置任务超时时间（24小时）
	c.client.Expire(c.ctx, key, 24*time.Hour)

	// 更新当前运行的任务ID
	c.client.Set(c.ctx, fmt.Sprintf("prerender:%s:current_task", siteID), taskID, 0)

	return taskID, nil
}

// UpdatePreheatTaskProgress 更新预热任务进度
func (c *Client) UpdatePreheatTaskProgress(siteID, taskID string, total, processed, success, failed int64) error {
	key := fmt.Sprintf("prerender:%s:task:%s", siteID, taskID)
	return c.client.HSet(c.ctx, key, map[string]interface{}{
		"total_urls": total,
		"processed":  processed,
		"success":    success,
		"failed":     failed,
		"updated_at": time.Now().Unix(),
	}).Err()
}

// SetPreheatTaskStatus 设置预热任务状态
func (c *Client) SetPreheatTaskStatus(siteID, taskID, status string) error {
	key := fmt.Sprintf("prerender:%s:task:%s", siteID, taskID)
	return c.client.HSet(c.ctx, key, map[string]interface{}{
		"status":     status,
		"updated_at": time.Now().Unix(),
	}).Err()
}

// GetPreheatTaskStatus 获取预热任务状态
func (c *Client) GetPreheatTaskStatus(siteID, taskID string) (map[string]string, error) {
	key := fmt.Sprintf("prerender:%s:task:%s", siteID, taskID)
	return c.client.HGetAll(c.ctx, key).Result()
}

// GetCurrentPreheatTask 获取当前运行的预热任务ID
func (c *Client) GetCurrentPreheatTask(siteID string) (string, error) {
	key := fmt.Sprintf("prerender:%s:current_task", siteID)
	return c.client.Get(c.ctx, key).Result()
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

// SetPushTask 保存推送任务
func (c *Client) SetPushTask(siteID string, task interface{}) error {
	key := fmt.Sprintf("prerender:%s:push:task", siteID)
	data, err := json.Marshal(task)
	if err != nil {
		return err
	}
	return c.client.Set(c.ctx, key, data, 0).Err()
}

// GetPushTask 获取推送任务
func (c *Client) GetPushTask(siteID string, taskID string) (map[string]string, error) {
	key := fmt.Sprintf("prerender:%s:push:task:%s", siteID, taskID)
	return c.client.HGetAll(c.ctx, key).Result()
}

// IncrPushStats 增加推送统计
func (c *Client) IncrPushStats(siteID string, success, failed int) error {
	key := fmt.Sprintf("prerender:%s:push:stats", siteID)
	pipe := c.client.Pipeline()
	pipe.HIncrBy(c.ctx, key, "total", int64(success+failed))
	pipe.HIncrBy(c.ctx, key, "success", int64(success))
	pipe.HIncrBy(c.ctx, key, "failed", int64(failed))
	_, err := pipe.Exec(c.ctx)
	return err
}

// SetURLPushStatus 设置URL的推送状态
func (c *Client) SetURLPushStatus(siteID, url, status string) error {
	key := fmt.Sprintf("prerender:%s:push:status", siteID)
	return c.client.HSet(c.ctx, key, url, status).Err()
}

// GetURLPushStatus 获取URL的推送状态
func (c *Client) GetURLPushStatus(siteID, url string) (string, error) {
	key := fmt.Sprintf("prerender:%s:push:status", siteID)
	return c.client.HGet(c.ctx, key, url).Result()
}

// GetAllURLPushStatuses 获取站点所有URL的推送状态
func (c *Client) GetAllURLPushStatuses(siteID string) (map[string]string, error) {
	key := fmt.Sprintf("prerender:%s:push:status", siteID)
	return c.client.HGetAll(c.ctx, key).Result()
}

// GetURLPushStats 获取站点的URL推送统计
func (c *Client) GetURLPushStats(siteID string) (map[string]int64, error) {
	// 获取所有URL
	allURLs, err := c.GetURLs(siteID)
	if err != nil {
		return nil, err
	}

	// 获取所有已推送的URL状态
	pushedURLs, err := c.GetAllURLPushStatuses(siteID)
	if err != nil {
		return nil, err
	}

	// 计算统计数据
	stats := map[string]int64{
		"total_urls":      int64(len(allURLs)),
		"pushed_urls":     int64(len(pushedURLs)),
		"not_pushed_urls": int64(len(allURLs) - len(pushedURLs)),
	}

	return stats, nil
}

// GetPushStats 获取推送统计
func (c *Client) GetPushStats(siteID string) (map[string]interface{}, error) {
	key := fmt.Sprintf("prerender:%s:push:stats", siteID)
	stats, err := c.client.HGetAll(c.ctx, key).Result()
	if err != nil {
		// 提供更详细的错误信息
		return nil, fmt.Errorf("failed to get push stats for site %s: %v", siteID, err)
	}

	// 转换为数字类型
	result := make(map[string]interface{})
	for k, v := range stats {
		val, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			// 记录转换错误，但不中断处理
			logging.DefaultLogger.Warn("Failed to parse %s value %s to int: %v", k, v, err)
			result[k] = v
		} else {
			result[k] = val
		}
	}

	// 添加默认值
	if _, exists := result["total"]; !exists {
		result["total"] = 0
	}
	if _, exists := result["success"]; !exists {
		result["success"] = 0
	}
	if _, exists := result["failed"]; !exists {
		result["failed"] = 0
	}

	// 获取URL推送统计，即使失败也不影响主功能
	urlStats, err := c.GetURLPushStats(siteID)
	if err != nil {
		// 记录错误，但不中断处理
		logging.DefaultLogger.Warn("Failed to get URL push stats for site %s: %v", siteID, err)
		// 使用默认值
		result["total_urls"] = 0
		result["pushed_urls"] = 0
		result["not_pushed_urls"] = 0
	} else {
		result["total_urls"] = urlStats["total_urls"]
		result["pushed_urls"] = urlStats["pushed_urls"]
		result["not_pushed_urls"] = urlStats["not_pushed_urls"]
	}

	return result, nil
}

// GetLastPushDate 获取最后推送日期
// 从Redis中获取指定站点的最后推送日期
//
// 参数:
//
//	siteID: 站点ID
//
// 返回值:
//
//	string: 最后推送日期，格式为YYYY-MM-DD
//	error: 错误信息
func (c *Client) GetLastPushDate(siteID string) (string, error) {
	// 构建Redis键
	key := fmt.Sprintf("prerender:%s:push:meta", siteID)

	// 获取最后推送日期
	lastPushDate, err := c.client.HGet(c.ctx, key, "last_push_date").Result()
	if err == redis.Nil {
		return "", nil
	} else if err != nil {
		return "", fmt.Errorf("failed to get last push date: %v", err)
	}

	return lastPushDate, nil
}

// SetLastPushDate 设置最后推送日期
// 将指定站点的最后推送日期保存到Redis
//
// 参数:
//
//	siteID: 站点ID
//	date: 最后推送日期，格式为YYYY-MM-DD
//
// 返回值:
//
//	error: 错误信息
func (c *Client) SetLastPushDate(siteID string, date string) error {
	// 构建Redis键
	key := fmt.Sprintf("prerender:%s:push:meta", siteID)

	// 设置最后推送日期
	_, err := c.client.HSet(c.ctx, key, "last_push_date", date).Result()
	if err != nil {
		return fmt.Errorf("failed to set last push date: %v", err)
	}

	return nil
}

// GetPushOffset 获取推送偏移量
// 从Redis中获取指定站点的推送偏移量
//
// 参数:
//
//	siteID: 站点ID
//
// 返回值:
//
//	int: 推送偏移量
//	error: 错误信息
func (c *Client) GetPushOffset(siteID string) (int, error) {
	// 构建Redis键
	key := fmt.Sprintf("prerender:%s:push:meta", siteID)

	// 获取推送偏移量
	offsetStr, err := c.client.HGet(c.ctx, key, "push_offset").Result()
	if err == redis.Nil {
		return 0, nil
	} else if err != nil {
		return 0, fmt.Errorf("failed to get push offset: %v", err)
	}

	// 转换为整数
	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse push offset: %v", err)
	}

	return offset, nil
}

// SetPushOffset 设置推送偏移量
// 将指定站点的推送偏移量保存到Redis
//
// 参数:
//
//	siteID: 站点ID
//	offset: 推送偏移量
//
// 返回值:
//
//	error: 错误信息
func (c *Client) SetPushOffset(siteID string, offset int) error {
	// 构建Redis键
	key := fmt.Sprintf("prerender:%s:push:meta", siteID)

	// 设置推送偏移量
	_, err := c.client.HSet(c.ctx, key, "push_offset", strconv.Itoa(offset)).Result()
	if err != nil {
		return fmt.Errorf("failed to set push offset: %v", err)
	}

	return nil
}

// AddPushLog 添加推送日志
func (c *Client) AddPushLog(siteID string, log interface{}) error {
	// 使用List存储日志，最新的日志在前面
	key := fmt.Sprintf("prerender:%s:push:logs", siteID)
	data, err := json.Marshal(log)
	if err != nil {
		return err
	}
	// 添加到列表开头
	if err := c.client.LPush(c.ctx, key, data).Err(); err != nil {
		return err
	}
	// 只保留最近1000条日志
	if err := c.client.LTrim(c.ctx, key, 0, 999).Err(); err != nil {
		return err
	}
	// 设置30天过期时间
	return c.client.Expire(c.ctx, key, 30*24*time.Hour).Err()
}

// IncrDailyPushCount 增加每日推送计数
func (c *Client) IncrDailyPushCount(siteID string, count int) error {
	today := time.Now().Format("2006-01-02")
	key := fmt.Sprintf("prerender:%s:push:daily:%s", siteID, today)
	return c.client.IncrBy(c.ctx, key, int64(count)).Err()
}

// GetDailyPushCount 获取每日推送计数
func (c *Client) GetDailyPushCount(siteID string, date string) (int64, error) {
	key := fmt.Sprintf("prerender:%s:push:daily:%s", siteID, date)
	strVal, err := c.client.Get(c.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, err
	}
	return strconv.ParseInt(strVal, 10, 64)
}

// GetLast15DaysPushCount 获取最近15天的推送计数
func (c *Client) GetLast15DaysPushCount(siteID string) (map[string]int64, error) {
	result := make(map[string]int64)

	// 获取最近15天的日期
	for i := 14; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		count, err := c.GetDailyPushCount(siteID, date)
		if err != nil && err != redis.Nil {
			return nil, err
		}
		if err == redis.Nil {
			result[date] = 0
		} else {
			result[date] = count
		}
	}

	return result, nil
}

// GetPushStatsWithURLCounts 获取包含URL计数的推送统计
func (c *Client) GetPushStatsWithURLCounts(siteID string) (map[string]interface{}, error) {
	key := fmt.Sprintf("prerender:%s:push:stats", siteID)
	stats, err := c.client.HGetAll(c.ctx, key).Result()
	if err != nil {
		return nil, err
	}

	// 转换为数字类型
	result := make(map[string]interface{})
	for k, v := range stats {
		val, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			result[k] = v
		} else {
			result[k] = val
		}
	}

	// 添加默认值
	if _, exists := result["total"]; !exists {
		result["total"] = 0
	}
	if _, exists := result["success"]; !exists {
		result["success"] = 0
	}
	if _, exists := result["failed"]; !exists {
		result["failed"] = 0
	}

	// 获取所有URL
	allURLs, err := c.GetURLs(siteID)
	if err != nil {
		return nil, err
	}

	// 获取所有已推送的URL状态
	pushedURLs, err := c.GetAllURLPushStatuses(siteID)
	if err != nil {
		return nil, err
	}

	// 添加URL计数
	totalURLs := int64(len(allURLs))
	pushed := int64(len(pushedURLs))
	result["total_urls"] = totalURLs
	result["pushed_urls"] = pushed
	result["not_pushed_urls"] = totalURLs - pushed

	return result, nil
}

// GetPushLogs 获取推送日志
func (c *Client) GetPushLogs(siteID string, limit, offset int) ([]interface{}, error) {
	key := fmt.Sprintf("prerender:%s:push:logs", siteID)
	// 获取指定范围的日志
	logs, err := c.client.LRange(c.ctx, key, int64(offset), int64(offset+limit-1)).Result()
	if err != nil {
		return nil, err
	}

	// 解析日志
	result := make([]interface{}, len(logs))
	for i, log := range logs {
		var data interface{}
		if err := json.Unmarshal([]byte(log), &data); err != nil {
			result[i] = log
		} else {
			result[i] = data
		}
	}

	return result, nil
}

// DeleteSiteData 删除站点的所有Redis数据
func (c *Client) DeleteSiteData(siteID string) error {
	// 使用Scan查找并删除相关key
	var keys []string

	// Pattern 1: prerender:{siteID}:*
	iter := c.client.Scan(c.ctx, 0, fmt.Sprintf("prerender:%s:*", siteID), 0).Iterator()
	for iter.Next(c.ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return err
	}

	// Pattern 2: prerender:{siteID}_* (for the config keys I added)
	iter2 := c.client.Scan(c.ctx, 0, fmt.Sprintf("prerender:%s_*", siteID), 0).Iterator()
	for iter2.Next(c.ctx) {
		keys = append(keys, iter2.Val())
	}
	if err := iter2.Err(); err != nil {
		return err
	}

	if len(keys) > 0 {
		return c.client.Del(c.ctx, keys...).Err()
	}
	return nil
}

// === 会话管理 ===

// SaveSession 保存会话
func (c *Client) SaveSession(sessionID, userID string, expiration time.Duration) error {
	key := fmt.Sprintf("session:%s", sessionID)
	// 使用Hash存储更多会话信息
	err := c.client.HSet(c.ctx, key, map[string]interface{}{
		"user_id":    userID,
		"created_at": time.Now().Unix(),
		"expires_at": time.Now().Add(expiration).Unix(),
	}).Err()
	if err != nil {
		return err
	}
	// 设置过期时间
	return c.client.Expire(c.ctx, key, expiration).Err()
}

// GetSession 获取会话信息
func (c *Client) GetSession(sessionID string) (map[string]string, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	return c.client.HGetAll(c.ctx, key).Result()
}

// DeleteSession 删除会话
func (c *Client) DeleteSession(sessionID string) error {
	key := fmt.Sprintf("session:%s", sessionID)
	return c.client.Del(c.ctx, key).Err()
}

// CheckSessionExists 检查会话是否存在
func (c *Client) CheckSessionExists(sessionID string) (bool, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	val, err := c.client.Exists(c.ctx, key).Result()
	if err != nil {
		return false, err
	}
	return val > 0, nil
}

// === 系统配置管理 ===

// SaveSystemConfig 保存系统配置
func (c *Client) SaveSystemConfig(config map[string]interface{}) error {
	key := "config:system"
	return c.client.HSet(c.ctx, key, config).Err()
}

// GetSystemConfig 获取系统配置
func (c *Client) GetSystemConfig() (map[string]string, error) {
	key := "config:system"
	return c.client.HGetAll(c.ctx, key).Result()
}
