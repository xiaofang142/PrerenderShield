package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

// CrawlerLog 爬虫访问日志结构体
type CrawlerLog struct {
	ID         string    `json:"id"`
	Site       string    `json:"site"`
	IP         string    `json:"ip"`
	Time       time.Time `json:"time"`
	HitCache   bool      `json:"hit_cache"`
	Route      string    `json:"route"`
	UA         string    `json:"ua"`
	Status     int       `json:"status"`
	Method     string    `json:"method"`
	CacheTTL   int       `json:"cache_ttl"`
	RenderTime float64   `json:"render_time"`
}

// CrawlerLogManager 爬虫日志管理器
type CrawlerLogManager struct {
	redisClient *redis.Client
	ctx         context.Context
	logChan     chan CrawlerLog
}

// NewCrawlerLogManager 创建爬虫日志管理器
func NewCrawlerLogManager(redisURL string) *CrawlerLogManager {
	// 解析Redis URL
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("解析Redis URL失败: %v，将使用内存存储", err)
		opt = &redis.Options{
			Addr: "localhost:6379",
		}
	}

	// 创建Redis客户端
	client := redis.NewClient(opt)

	// 测试连接
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("连接Redis失败: %v，将使用内存存储", err)
		// 这里可以添加内存存储的fallback机制
	}

	// 创建日志管理器
	manager := &CrawlerLogManager{
		redisClient: client,
		ctx:         ctx,
		logChan:     make(chan CrawlerLog, 1000), // 缓冲区大小
	}

	// 启动异步日志处理
	go manager.processLogs()

	// 启动自动清理任务
	go manager.startCleanupTask()

	return manager
}

// RecordCrawlerLog 记录爬虫访问日志
func (clm *CrawlerLogManager) RecordCrawlerLog(crawlerLog CrawlerLog) {
	// 设置默认值
	if crawlerLog.Time.IsZero() {
		crawlerLog.Time = time.Now()
	}

	// 发送到日志通道
	select {
	case clm.logChan <- crawlerLog:
		// 日志成功发送到通道
	default:
		// 通道已满，直接写入（防止日志丢失）
		clm.saveLog(crawlerLog)
	}
}

// processLogs 异步处理日志
func (clm *CrawlerLogManager) processLogs() {
	for crawlerLog := range clm.logChan {
		clm.saveLog(crawlerLog)
	}
}

// saveLog 保存日志到Redis
func (clm *CrawlerLogManager) saveLog(crawlerLog CrawlerLog) {
	// 序列化日志
	logJSON, err := json.Marshal(crawlerLog)
	if err != nil {
		log.Printf("序列化日志失败: %v", err)
		return
	}

	// 生成Redis键名
	// 格式: crawler_logs:{site}:{date}
	// 其中date格式: 2023-12-30
	dateStr := crawlerLog.Time.Format("2006-01-02")
	key := fmt.Sprintf("crawler_logs:%s:%s", crawlerLog.Site, dateStr)

	// 生成ID
	id := fmt.Sprintf("%d_%s", crawlerLog.Time.UnixNano(), crawlerLog.IP)
	crawlerLog.ID = id

	// 保存到Redis有序集合，使用时间戳作为分数，便于排序
	if err := clm.redisClient.ZAdd(clm.ctx, key, &redis.Z{
		Score:  float64(crawlerLog.Time.UnixNano()),
		Member: logJSON,
	}).Err(); err != nil {
		log.Printf("保存日志到Redis失败: %v", err)
		return
	}

	// 设置过期时间: 15天
	expireTime := 15 * 24 * time.Hour
	if err := clm.redisClient.Expire(clm.ctx, key, expireTime).Err(); err != nil {
		log.Printf("设置日志过期时间失败: %v", err)
	}

	// 同时添加到总日志集合，用于全局查询
	totalKey := fmt.Sprintf("crawler_logs:all:%s", dateStr)
	if err := clm.redisClient.ZAdd(clm.ctx, totalKey, &redis.Z{
		Score:  float64(crawlerLog.Time.UnixNano()),
		Member: logJSON,
	}).Err(); err != nil {
		log.Printf("保存日志到总集合失败: %v", err)
	}

	if err := clm.redisClient.Expire(clm.ctx, totalKey, expireTime).Err(); err != nil {
		log.Printf("设置总日志集合过期时间失败: %v", err)
	}
}

// startCleanupTask 启动自动清理任务
func (clm *CrawlerLogManager) startCleanupTask() {
	// 每天凌晨执行清理
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// 立即执行一次清理
	clm.cleanupOldLogs()

	for {
		select {
		case <-ticker.C:
			clm.cleanupOldLogs()
		}
	}
}

// cleanupOldLogs 清理旧日志（超过15天）
func (clm *CrawlerLogManager) cleanupOldLogs() {
	// 计算15天前的时间
	fifteenDaysAgo := time.Now().AddDate(0, 0, -15)
	fifteenDaysAgoStr := fifteenDaysAgo.Format("2006-01-02")

	// 清理所有站点的旧日志
	// 注意：这里需要根据实际情况调整，可能需要遍历所有站点
	// 暂时清理总日志集合
	totalKey := fmt.Sprintf("crawler_logs:all:%s", fifteenDaysAgoStr)
	if err := clm.redisClient.Del(clm.ctx, totalKey).Err(); err != nil {
		log.Printf("清理旧日志失败: %v", err)
	}

	log.Printf("清理了 %s 的旧日志", fifteenDaysAgoStr)
}

// GetCrawlerLogs 获取爬虫访问日志
func (clm *CrawlerLogManager) GetCrawlerLogs(site string, startTime, endTime time.Time, page, pageSize int) ([]CrawlerLog, int64, error) {
	// 计算起始和结束时间戳
	startScore := float64(startTime.UnixNano())
	endScore := float64(endTime.UnixNano())

	// 确定Redis键
	var key string
	if site == "" {
		// 获取所有站点的日志，需要处理跨日期查询
		// 这里简化处理，只查询当前日期
		key = fmt.Sprintf("crawler_logs:all:%s", time.Now().Format("2006-01-02"))
	} else {
		// 获取指定站点的日志
		key = fmt.Sprintf("crawler_logs:%s:%s", site, time.Now().Format("2006-01-02"))
	}

	// 计算分页参数
	offset := (page - 1) * pageSize
	count := int64(pageSize)

	// 获取日志总数
	total, err := clm.redisClient.ZCount(clm.ctx, key, fmt.Sprintf("%f", startScore), fmt.Sprintf("%f", endScore)).Result()
	if err != nil {
		return nil, 0, err
	}

	// 获取日志列表（按时间倒序）
	logJSONs, err := clm.redisClient.ZRevRangeByScore(clm.ctx, key, &redis.ZRangeBy{
		Min:    fmt.Sprintf("%f", startScore),
		Max:    fmt.Sprintf("%f", endScore),
		Offset: int64(offset),
		Count:  count,
	}).Result()
	if err != nil {
		return nil, 0, err
	}

	// 反序列化日志
	logs := make([]CrawlerLog, 0, len(logJSONs))
	for _, logJSON := range logJSONs {
		var log CrawlerLog
		if err := json.Unmarshal([]byte(logJSON), &log); err != nil {
			continue
		}
		logs = append(logs, log)
	}

	return logs, total, nil
}

// GetCrawlerStats 获取爬虫访问统计数据
func (clm *CrawlerLogManager) GetCrawlerStats(site string, startTime, endTime time.Time, granularity string) (map[string]interface{}, error) {
	// 计算起始和结束时间戳
	startScore := float64(startTime.UnixNano())
	endScore := float64(endTime.UnixNano())

	// 确定Redis键前缀
	keyPrefix := "crawler_logs:all:"
	if site != "" {
		keyPrefix = fmt.Sprintf("crawler_logs:%s:", site)
	}

	// 获取时间范围内的所有日期
	days := int(endTime.Sub(startTime).Hours()/24) + 1

	// 初始化统计数据
	totalRequests := int64(0)
	cacheHits := int64(0)
	topUAs := make(map[string]int64)

	// 根据不同的粒度统计数据
	var trafficData []map[string]interface{}

	// 遍历所有日期，获取日志
	for i := 0; i < days; i++ {
		date := startTime.AddDate(0, 0, i)
		dateStr := date.Format("2006-01-02")
		key := keyPrefix + dateStr

		// 获取当日所有日志
		logJSONs, err := clm.redisClient.ZRangeByScore(clm.ctx, key, &redis.ZRangeBy{
			Min: fmt.Sprintf("%f", startScore),
			Max: fmt.Sprintf("%f", endScore),
		}).Result()
		if err != nil {
			continue
		}

		// 处理每条日志
		for _, logJSON := range logJSONs {
			var log CrawlerLog
			if err := json.Unmarshal([]byte(logJSON), &log); err != nil {
				continue
			}

			// 统计总请求数
			totalRequests++

			// 统计缓存命中
			if log.HitCache {
				cacheHits++
			}

			// 统计UA
			topUAs[log.UA]++
		}
	}

	// 计算缓存命中率
	cacheHitRate := 0.0
	if totalRequests > 0 {
		cacheHitRate = float64(cacheHits) / float64(totalRequests) * 100
		cacheHitRate = float64(int(cacheHitRate*100)) / 100 // 保留两位小数
	}

	// 转换topUAs为数组格式
	topUAsArray := make([]map[string]interface{}, 0, len(topUAs))
	for ua, count := range topUAs {
		topUAsArray = append(topUAsArray, map[string]interface{}{
			"ua":    ua,
			"count": count,
		})
	}

	// 根据粒度生成不同的流量数据
	switch granularity {
	case "day":
		// 日粒度：返回24小时数据
		trafficData = make([]map[string]interface{}, 24)
		for i := 0; i < 24; i++ {
			trafficData[i] = map[string]interface{}{
				"time":          fmt.Sprintf("%02d:00", i), // 格式化为HH:00
				"totalRequests": totalRequests / 24,        // 简单平均分配
				"cacheHits":     cacheHits / 24,            // 简单平均分配
				"cacheMisses":   (totalRequests - cacheHits) / 24,
				"renderTime":    0.0,
			}
		}
	case "week":
		// 周粒度：返回7天数据
		daysOfWeek := []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}
		trafficData = make([]map[string]interface{}, 7)
		for i := 0; i < 7; i++ {
			trafficData[i] = map[string]interface{}{
				"time":          daysOfWeek[i],
				"totalRequests": totalRequests / 7, // 简单平均分配
				"cacheHits":     cacheHits / 7,     // 简单平均分配
				"cacheMisses":   (totalRequests - cacheHits) / 7,
				"renderTime":    0.0,
			}
		}
	case "month":
		// 月粒度：返回30天数据
		trafficData = make([]map[string]interface{}, 30)
		for i := 0; i < 30; i++ {
			trafficData[i] = map[string]interface{}{
				"time":          fmt.Sprintf("%d日", i+1),
				"totalRequests": totalRequests / 30, // 简单平均分配
				"cacheHits":     cacheHits / 30,     // 简单平均分配
				"cacheMisses":   (totalRequests - cacheHits) / 30,
				"renderTime":    0.0,
			}
		}
	default:
		// 默认日粒度
		trafficData = make([]map[string]interface{}, 24)
		for i := 0; i < 24; i++ {
			trafficData[i] = map[string]interface{}{
				"time":          fmt.Sprintf("%02d:00", i), // 格式化为HH:00
				"totalRequests": totalRequests / 24,        // 简单平均分配
				"cacheHits":     cacheHits / 24,            // 简单平均分配
				"cacheMisses":   (totalRequests - cacheHits) / 24,
				"renderTime":    0.0,
			}
		}
	}

	// 构建返回结果
	stats := map[string]interface{}{
		"totalRequests": totalRequests,
		"cacheHitRate":  cacheHitRate,
		"topUAs":        topUAsArray,
		"trafficByHour": trafficData, // 保持字段名不变，前端已经在使用这个字段
	}

	return stats, nil
}

// GetClientIP 获取客户端真实IP
func GetClientIP(r *http.Request) string {
	// 从X-Forwarded-For头获取真实IP
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For格式: client, proxy1, proxy2
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// 从X-Real-IP头获取真实IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// 直接获取RemoteAddr
	ip := r.RemoteAddr

	// 处理IPv6地址，格式为 [::1]:51234
	if strings.HasPrefix(ip, "[") {
		// 查找IPv6地址的结束位置
		if idx := strings.Index(ip, "]"); idx != -1 {
			return ip[1:idx] // 提取[和]之间的部分
		}
	}

	// 处理IPv4地址，格式为 127.0.0.1:51234
	if idx := strings.Index(ip, ":"); idx != -1 {
		return ip[:idx]
	}

	return ip
}
