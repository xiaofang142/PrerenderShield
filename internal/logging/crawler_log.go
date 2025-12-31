package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
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
	// 内存存储，用于Redis连接失败时的fallback
	inMemoryLogs   map[string][]CrawlerLog // key: site:date 或 all:date
	redisAvailable bool                    // Redis是否可用
}

// NewCrawlerLogManager 创建爬虫日志管理器
func NewCrawlerLogManager(redisURL string) *CrawlerLogManager {
	// 创建Redis客户端选项，支持两种格式的Redis URL
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
			log.Printf("解析Redis URL失败: %v，将使用内存存储", err)
			opt = &redis.Options{
				Addr: "localhost:6379",
			}
		} else {
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
	}

	// 创建Redis客户端
	client := redis.NewClient(opt)

	// 测试连接
	ctx := context.Background()
	redisAvailable := true
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("连接Redis失败: %v，将使用内存存储", err)
		redisAvailable = false
	}

	// 创建日志管理器
	manager := &CrawlerLogManager{
		redisClient:    client,
		ctx:            ctx,
		logChan:        make(chan CrawlerLog, 1000),   // 缓冲区大小
		inMemoryLogs:   make(map[string][]CrawlerLog), // 初始化内存存储
		redisAvailable: redisAvailable,                // 设置Redis可用标志
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

// saveLog 保存日志到Redis或内存存储
func (clm *CrawlerLogManager) saveLog(crawlerLog CrawlerLog) {
	// 生成ID
	id := fmt.Sprintf("%d_%s", crawlerLog.Time.UnixNano(), crawlerLog.IP)
	crawlerLog.ID = id

	// 生成键名
	dateStr := crawlerLog.Time.Format("2006-01-02")
	siteKey := fmt.Sprintf("crawler_logs:%s:%s", crawlerLog.Site, dateStr)
	totalKey := fmt.Sprintf("crawler_logs:all:%s", dateStr)

	// 如果Redis可用，保存到Redis
	if clm.redisAvailable {
		// 序列化日志
		logJSON, err := json.Marshal(crawlerLog)
		if err != nil {
			log.Printf("序列化日志失败: %v", err)
			return
		}

		// 保存到Redis有序集合，使用时间戳作为分数，便于排序
		if err := clm.redisClient.ZAdd(clm.ctx, siteKey, &redis.Z{
			Score:  float64(crawlerLog.Time.UnixNano()),
			Member: logJSON,
		}).Err(); err != nil {
			log.Printf("保存日志到Redis失败: %v", err)
			// Redis保存失败，降级到内存存储
			clm.saveToMemory(siteKey, crawlerLog)
			clm.saveToMemory(totalKey, crawlerLog)
			return
		}

		// 设置过期时间: 15天
		expireTime := 15 * 24 * time.Hour
		if err := clm.redisClient.Expire(clm.ctx, siteKey, expireTime).Err(); err != nil {
			log.Printf("设置日志过期时间失败: %v", err)
		}

		// 同时添加到总日志集合，用于全局查询
		if err := clm.redisClient.ZAdd(clm.ctx, totalKey, &redis.Z{
			Score:  float64(crawlerLog.Time.UnixNano()),
			Member: logJSON,
		}).Err(); err != nil {
			log.Printf("保存日志到总集合失败: %v", err)
			// Redis保存失败，降级到内存存储
			clm.saveToMemory(totalKey, crawlerLog)
			return
		}

		if err := clm.redisClient.Expire(clm.ctx, totalKey, expireTime).Err(); err != nil {
			log.Printf("设置总日志集合过期时间失败: %v", err)
		}
	} else {
		// Redis不可用，保存到内存存储
		clm.saveToMemory(siteKey, crawlerLog)
		clm.saveToMemory(totalKey, crawlerLog)
	}
}

// saveToMemory 保存日志到内存存储
func (clm *CrawlerLogManager) saveToMemory(key string, crawlerLog CrawlerLog) {
	// 添加到内存存储
	clm.inMemoryLogs[key] = append(clm.inMemoryLogs[key], crawlerLog)
	// 限制内存存储的日志数量，防止内存溢出
	if len(clm.inMemoryLogs[key]) > 10000 {
		// 只保留最新的10000条日志
		clm.inMemoryLogs[key] = clm.inMemoryLogs[key][len(clm.inMemoryLogs[key])-10000:]
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
	// 确定键
	key := fmt.Sprintf("crawler_logs:%s:%s", site, time.Now().Format("2006-01-02"))
	if site == "" {
		key = fmt.Sprintf("crawler_logs:all:%s", time.Now().Format("2006-01-02"))
	}

	// 如果Redis可用，从Redis获取日志
	if clm.redisAvailable {
		// 计算起始和结束时间戳
		startScore := float64(startTime.UnixNano())
		endScore := float64(endTime.UnixNano())

		// 计算分页参数
		offset := (page - 1) * pageSize
		count := int64(pageSize)

		// 获取日志总数
		total, err := clm.redisClient.ZCount(clm.ctx, key, fmt.Sprintf("%f", startScore), fmt.Sprintf("%f", endScore)).Result()
		if err != nil {
			// Redis获取失败，从内存存储获取
			return clm.getLogsFromMemory(site, startTime, endTime, page, pageSize)
		}

		// 获取日志列表（按时间倒序）
		logJSONs, err := clm.redisClient.ZRevRangeByScore(clm.ctx, key, &redis.ZRangeBy{
			Min:    fmt.Sprintf("%f", startScore),
			Max:    fmt.Sprintf("%f", endScore),
			Offset: int64(offset),
			Count:  count,
		}).Result()
		if err != nil {
			// Redis获取失败，从内存存储获取
			return clm.getLogsFromMemory(site, startTime, endTime, page, pageSize)
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
	} else {
		// Redis不可用，从内存存储获取日志
		return clm.getLogsFromMemory(site, startTime, endTime, page, pageSize)
	}
}

// getLogsFromMemory 从内存存储中获取日志
func (clm *CrawlerLogManager) getLogsFromMemory(site string, startTime, endTime time.Time, page, pageSize int) ([]CrawlerLog, int64, error) {
	// 确定键
	key := fmt.Sprintf("crawler_logs:%s:%s", site, time.Now().Format("2006-01-02"))
	if site == "" {
		key = fmt.Sprintf("crawler_logs:all:%s", time.Now().Format("2006-01-02"))
	}

	// 从内存存储获取日志
	allLogs := clm.inMemoryLogs[key]
	if allLogs == nil {
		return []CrawlerLog{}, 0, nil
	}

	// 过滤时间范围内的日志
	var filteredLogs []CrawlerLog
	for _, log := range allLogs {
		if log.Time.After(startTime) && log.Time.Before(endTime) || log.Time.Equal(startTime) || log.Time.Equal(endTime) {
			filteredLogs = append(filteredLogs, log)
		}
	}

	// 按时间倒序排序
	for i := 0; i < len(filteredLogs); i++ {
		for j := i + 1; j < len(filteredLogs); j++ {
			if filteredLogs[i].Time.Before(filteredLogs[j].Time) {
				filteredLogs[i], filteredLogs[j] = filteredLogs[j], filteredLogs[i]
			}
		}
	}

	// 计算总数
	total := int64(len(filteredLogs))

	// 分页处理
	offset := (page - 1) * pageSize
	var pagedLogs []CrawlerLog
	if offset < len(filteredLogs) {
		end := offset + pageSize
		if end > len(filteredLogs) {
			end = len(filteredLogs)
		}
		pagedLogs = filteredLogs[offset:end]
	}

	return pagedLogs, total, nil
}

// GetCrawlerStats 获取爬虫访问统计数据
func (clm *CrawlerLogManager) GetCrawlerStats(site string, startTime, endTime time.Time, granularity string) (map[string]interface{}, error) {
	// 初始化统计数据
	totalRequests := int64(0)
	cacheHits := int64(0)
	topUAs := make(map[string]int64)
	var allLogs []CrawlerLog

	// 确定键前缀
	keyPrefix := "crawler_logs:all:"
	if site != "" {
		keyPrefix = fmt.Sprintf("crawler_logs:%s:", site)
	}

	// 获取时间范围内的所有日期
	days := int(endTime.Sub(startTime).Hours()/24) + 1

	// 遍历所有日期，获取日志
	for i := 0; i < days; i++ {
		date := startTime.AddDate(0, 0, i)
		dateStr := date.Format("2006-01-02")
		key := keyPrefix + dateStr

		// 如果Redis可用，从Redis获取日志
		if clm.redisAvailable {
			// 计算起始和结束时间戳
			startScore := float64(startTime.UnixNano())
			endScore := float64(endTime.UnixNano())

			// 获取当日所有日志
			logJSONs, err := clm.redisClient.ZRangeByScore(clm.ctx, key, &redis.ZRangeBy{
				Min: fmt.Sprintf("%f", startScore),
				Max: fmt.Sprintf("%f", endScore),
			}).Result()
			if err != nil {
				// Redis获取失败，从内存存储获取
				logs, _, _ := clm.getLogsFromMemory(site, startTime, endTime, 1, 10000)
				allLogs = append(allLogs, logs...)
				continue
			}

			// 处理每条日志
			for _, logJSON := range logJSONs {
				var log CrawlerLog
				if err := json.Unmarshal([]byte(logJSON), &log); err != nil {
					continue
				}
				allLogs = append(allLogs, log)
			}
		} else {
			// Redis不可用，从内存存储获取日志
			logs, _, _ := clm.getLogsFromMemory(site, startTime, endTime, 1, 10000)
			allLogs = append(allLogs, logs...)
		}
	}

	// 处理所有日志，统计数据
	for _, log := range allLogs {
		// 统计总请求数
		totalRequests++

		// 统计缓存命中
		if log.HitCache {
			cacheHits++
		}

		// 统计UA
		topUAs[log.UA]++
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
	var trafficData []map[string]interface{}
	switch granularity {
	case "day":
		// 日粒度：返回24小时数据
		trafficData = make([]map[string]interface{}, 24)
		// 根据小时统计数据
		hourlyData := make(map[int]map[string]int64)
		for _, log := range allLogs {
			hour := log.Time.Hour()
			if hourlyData[hour] == nil {
				hourlyData[hour] = map[string]int64{
					"totalRequests": 0,
					"cacheHits":     0,
				}
			}
			hourlyData[hour]["totalRequests"]++
			if log.HitCache {
				hourlyData[hour]["cacheHits"]++
			}
		}
		// 填充数据
		for i := 0; i < 24; i++ {
			data := hourlyData[i]
			if data == nil {
				data = map[string]int64{
					"totalRequests": 0,
					"cacheHits":     0,
				}
			}
			trafficData[i] = map[string]interface{}{
				"time":          fmt.Sprintf("%02d:00", i), // 格式化为HH:00
				"totalRequests": data["totalRequests"],
				"cacheHits":     data["cacheHits"],
				"cacheMisses":   data["totalRequests"] - data["cacheHits"],
				"renderTime":    0.0,
			}
		}
	case "week":
		// 周粒度：返回7天数据
		daysOfWeek := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
		trafficData = make([]map[string]interface{}, 7)
		// 根据星期几统计数据
		weeklyData := make(map[int]map[string]int64)
		for _, log := range allLogs {
			day := int(log.Time.Weekday())
			if weeklyData[day] == nil {
				weeklyData[day] = map[string]int64{
					"totalRequests": 0,
					"cacheHits":     0,
				}
			}
			weeklyData[day]["totalRequests"]++
			if log.HitCache {
				weeklyData[day]["cacheHits"]++
			}
		}
		// 填充数据
		for i := 0; i < 7; i++ {
			data := weeklyData[i]
			if data == nil {
				data = map[string]int64{
					"totalRequests": 0,
					"cacheHits":     0,
				}
			}
			trafficData[i] = map[string]interface{}{
				"time":          daysOfWeek[i],
				"totalRequests": data["totalRequests"],
				"cacheHits":     data["cacheHits"],
				"cacheMisses":   data["totalRequests"] - data["cacheHits"],
				"renderTime":    0.0,
			}
		}
	case "month":
		// 月粒度：返回30天数据
		trafficData = make([]map[string]interface{}, 30)
		// 根据日期统计数据
		monthlyData := make(map[int]map[string]int64)
		for _, log := range allLogs {
			day := log.Time.Day() - 1 // 转换为0-29索引
			if day >= 30 {
				continue // 跳过31日
			}
			if monthlyData[day] == nil {
				monthlyData[day] = map[string]int64{
					"totalRequests": 0,
					"cacheHits":     0,
				}
			}
			monthlyData[day]["totalRequests"]++
			if log.HitCache {
				monthlyData[day]["cacheHits"]++
			}
		}
		// 填充数据
		for i := 0; i < 30; i++ {
			data := monthlyData[i]
			if data == nil {
				data = map[string]int64{
					"totalRequests": 0,
					"cacheHits":     0,
				}
			}
			trafficData[i] = map[string]interface{}{
				"time":          fmt.Sprintf("%d日", i+1),
				"totalRequests": data["totalRequests"],
				"cacheHits":     data["cacheHits"],
				"cacheMisses":   data["totalRequests"] - data["cacheHits"],
				"renderTime":    0.0,
			}
		}
	default:
		// 默认日粒度
		trafficData = make([]map[string]interface{}, 24)
		// 根据小时统计数据
		hourlyData := make(map[int]map[string]int64)
		for _, log := range allLogs {
			hour := log.Time.Hour()
			if hourlyData[hour] == nil {
				hourlyData[hour] = map[string]int64{
					"totalRequests": 0,
					"cacheHits":     0,
				}
			}
			hourlyData[hour]["totalRequests"]++
			if log.HitCache {
				hourlyData[hour]["cacheHits"]++
			}
		}
		// 填充数据
		for i := 0; i < 24; i++ {
			data := hourlyData[i]
			if data == nil {
				data = map[string]int64{
					"totalRequests": 0,
					"cacheHits":     0,
				}
			}
			trafficData[i] = map[string]interface{}{
				"time":          fmt.Sprintf("%02d:00", i), // 格式化为HH:00
				"totalRequests": data["totalRequests"],
				"cacheHits":     data["cacheHits"],
				"cacheMisses":   data["totalRequests"] - data["cacheHits"],
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
