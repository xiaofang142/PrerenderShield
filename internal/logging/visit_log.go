package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

// VisitLog 正常用户访问日志结构体
type VisitLog struct {
	ID       string    `json:"id"`
	Site     string    `json:"site"`
	IP       string    `json:"ip"`
	Time     time.Time `json:"time"`
	Method   string    `json:"method"`
	URL      string    `json:"url"`
	Status   int       `json:"status"`
	UA       string    `json:"ua"`
	Duration float64   `json:"duration"` // 请求耗时（秒）
	Referer  string    `json:"referer"`

	// GeoIP fields
	Country   string  `json:"country,omitempty"`
	City      string  `json:"city,omitempty"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	Washed    bool    `json:"washed"` // 是否已清洗
}

// VisitLogManager 访问日志管理器
type VisitLogManager struct {
	redisClient *redis.Client
	ctx         context.Context
	logChan     chan VisitLog
}

// NewVisitLogManager 创建访问日志管理器
func NewVisitLogManager(redisURL string) *VisitLogManager {
	opt := &redis.Options{}
	if !strings.Contains(redisURL, "://") {
		opt.Addr = redisURL
		if !strings.Contains(opt.Addr, ":") {
			opt.Addr = fmt.Sprintf("%s:6379", opt.Addr)
		}
	} else {
		parsed, err := url.Parse(redisURL)
		if err != nil {
			log.Printf("解析Redis URL失败: %v", err)
			opt = &redis.Options{Addr: "localhost:6379"}
		} else {
			opt.Addr = parsed.Host
			if !strings.Contains(opt.Addr, ":") {
				opt.Addr = fmt.Sprintf("%s:6379", opt.Addr)
			}
			if parsed.User != nil {
				opt.Password, _ = parsed.User.Password()
			}
			db := 0
			if parsed.Path != "" && parsed.Path != "/" {
				fmt.Sscanf(parsed.Path[1:], "%d", &db)
			}
			opt.DB = db
		}
	}

	client := redis.NewClient(opt)
	ctx := context.Background()

	manager := &VisitLogManager{
		redisClient: client,
		ctx:         ctx,
		logChan:     make(chan VisitLog, 2000), // Larger buffer for visit logs
	}

	go manager.processLogs()
	go manager.startCleanupTask()

	return manager
}

// RecordVisitLog 记录访问日志
func (vlm *VisitLogManager) RecordVisitLog(visitLog VisitLog) {
	if visitLog.Time.IsZero() {
		visitLog.Time = time.Now()
	}

	// 简单的日志清洗/增强
	// 在没有独立清洗服务的情况下，我们在记录时直接进行简单的GeoIP处理
	if visitLog.IP != "" {
		// 模拟GeoIP解析
		if visitLog.IP == "127.0.0.1" || visitLog.IP == "::1" || visitLog.IP == "localhost" {
			visitLog.Country = "China"
			visitLog.City = "Local"
			visitLog.Latitude = 39.9042
			visitLog.Longitude = 116.4074
		} else {
			// 默认值
			visitLog.Country = "Unknown"
		}
		// 标记为已清洗，以便在统计中显示
		visitLog.Washed = true
	}

	select {
	case vlm.logChan <- visitLog:
	default:
		vlm.saveLog(visitLog)
	}
}

func (vlm *VisitLogManager) processLogs() {
	for visitLog := range vlm.logChan {
		vlm.saveLog(visitLog)
	}
}

func (vlm *VisitLogManager) saveLog(visitLog VisitLog) {
	id := fmt.Sprintf("%d_%s", visitLog.Time.UnixNano(), visitLog.IP)
	visitLog.ID = id

	dateStr := visitLog.Time.Format("2006-01-02")
	siteKey := fmt.Sprintf("visit_logs:%s:%s", visitLog.Site, dateStr)
	totalKey := fmt.Sprintf("visit_logs:all:%s", dateStr)

	logJSON, err := json.Marshal(visitLog)
	if err != nil {
		return
	}

	score := float64(visitLog.Time.UnixNano())

	pipe := vlm.redisClient.Pipeline()
	pipe.ZAdd(vlm.ctx, siteKey, &redis.Z{Score: score, Member: logJSON})
	pipe.Expire(vlm.ctx, siteKey, 15*24*time.Hour)
	pipe.ZAdd(vlm.ctx, totalKey, &redis.Z{Score: score, Member: logJSON})
	pipe.Expire(vlm.ctx, totalKey, 15*24*time.Hour)

	if !visitLog.Washed {
		pipe.RPush(vlm.ctx, "visit_logs:unwashed", logJSON)
	}

	// 更新统计数据 (PV/UV/IP/Hourly)
	// PV (Hourly)
	hourStr := visitLog.Time.Format("15") // 00-23
	hourlyKey := fmt.Sprintf("stats:hourly:%s", dateStr)
	pipe.HIncrBy(vlm.ctx, hourlyKey, hourStr, 1)
	pipe.Expire(vlm.ctx, hourlyKey, 15*24*time.Hour)

	// IP (HyperLogLog for Daily)
	if visitLog.IP != "" {
		ipKey := fmt.Sprintf("stats:daily_ip:%s", dateStr)
		pipe.PFAdd(vlm.ctx, ipKey, visitLog.IP)
		pipe.Expire(vlm.ctx, ipKey, 15*24*time.Hour)
	}

	// UV (HyperLogLog for Daily, using IP+UA hash approximation)
	if visitLog.IP != "" && visitLog.UA != "" {
		uvKey := fmt.Sprintf("stats:daily_uv:%s", dateStr)
		uvIdentifier := fmt.Sprintf("%s|%s", visitLog.IP, visitLog.UA)
		pipe.PFAdd(vlm.ctx, uvKey, uvIdentifier)
		pipe.Expire(vlm.ctx, uvKey, 15*24*time.Hour)
	}

	pipe.Exec(vlm.ctx)
}

// GetAccessStats 获取访问统计 (PV, UV, IP)
func (vlm *VisitLogManager) GetAccessStats(startTime, endTime time.Time) (int64, int64, int64) {
	var totalPV, totalUV, totalIP int64

	days := int(endTime.Sub(startTime).Hours()/24) + 1
	for i := 0; i < days; i++ {
		dateStr := startTime.AddDate(0, 0, i).Format("2006-01-02")

		// 1. PV (Page View)
		// 优先使用 visit_logs:all:YYYY-MM-DD 的 ZSet 长度，这是最准确的（包含历史数据）
		logKey := fmt.Sprintf("visit_logs:all:%s", dateStr)
		if pv, err := vlm.redisClient.ZCard(vlm.ctx, logKey).Result(); err == nil && pv > 0 {
			totalPV += pv
		} else {
			// 回退到 stats:hourly:* (虽然 ZCard 应该总是准确的)
			hourlyKey := fmt.Sprintf("stats:hourly:%s", dateStr)
			if hourlyData, err := vlm.redisClient.HGetAll(vlm.ctx, hourlyKey).Result(); err == nil {
				for _, countStr := range hourlyData {
					if count, err := strconv.ParseInt(countStr, 10, 64); err == nil {
						// 注意：如果 ZCard 失败了（key不存在），这里可能也为0
						// 但如果 ZCard 成功，我们就不需要这个了
						// 这里作为双重保险，但要小心不要重复计算
						if totalPV == 0 {
							totalPV += count
						}
					}
				}
			}
		}

		// 2. IP (Unique IP)
		ipKey := fmt.Sprintf("stats:daily_ip:%s", dateStr)
		if count, err := vlm.redisClient.PFCount(vlm.ctx, ipKey).Result(); err == nil && count > 0 {
			totalIP += count
		} else if totalPV > 0 && totalPV < 10000 {
			// 如果没有统计数据但有日志（且数量不多），尝试从日志中恢复
			// 注意：这只针对当天，且日志量较小的情况
			if logs, err := vlm.redisClient.ZRange(vlm.ctx, logKey, 0, -1).Result(); err == nil {
				uniqueIPs := make(map[string]bool)
				for _, logJSON := range logs {
					var l VisitLog
					if json.Unmarshal([]byte(logJSON), &l) == nil && l.IP != "" {
						uniqueIPs[l.IP] = true
					}
				}
				totalIP += int64(len(uniqueIPs))

				// 可选：回写统计数据
				if len(uniqueIPs) > 0 {
					pipe := vlm.redisClient.Pipeline()
					for ip := range uniqueIPs {
						pipe.PFAdd(vlm.ctx, ipKey, ip)
					}
					pipe.Expire(vlm.ctx, ipKey, 15*24*time.Hour)
					pipe.Exec(vlm.ctx)
				}
			}
		}

		// 3. UV (Unique Visitor)
		uvKey := fmt.Sprintf("stats:daily_uv:%s", dateStr)
		if count, err := vlm.redisClient.PFCount(vlm.ctx, uvKey).Result(); err == nil && count > 0 {
			totalUV += count
		} else if totalPV > 0 && totalPV < 10000 {
			// 同上，尝试恢复
			if logs, err := vlm.redisClient.ZRange(vlm.ctx, logKey, 0, -1).Result(); err == nil {
				uniqueUVs := make(map[string]bool)
				for _, logJSON := range logs {
					var l VisitLog
					if json.Unmarshal([]byte(logJSON), &l) == nil && l.IP != "" {
						uvID := fmt.Sprintf("%s|%s", l.IP, l.UA)
						uniqueUVs[uvID] = true
					}
				}
				totalUV += int64(len(uniqueUVs))

				// 可选：回写统计数据
				if len(uniqueUVs) > 0 {
					pipe := vlm.redisClient.Pipeline()
					for uv := range uniqueUVs {
						pipe.PFAdd(vlm.ctx, uvKey, uv)
					}
					pipe.Expire(vlm.ctx, uvKey, 15*24*time.Hour)
					pipe.Exec(vlm.ctx)
				}
			}
		}
	}

	return totalPV, totalUV, totalIP
}

// GetTrafficTrend 获取流量趋势
type TrafficData struct {
	Time            string `json:"time"`
	TotalRequests   int64  `json:"totalRequests"`
	CrawlerRequests int64  `json:"crawlerRequests"` // 这里暂时无法区分爬虫，除非我们也统计爬虫
	BlockedRequests int64  `json:"blockedRequests"` // WAF blocked
}

func (vlm *VisitLogManager) GetTrafficTrend(startTime, endTime time.Time) []TrafficData {
	// 获取当天的每小时数据
	// 为了简化，我们只返回当天的
	dateStr := time.Now().Format("2006-01-02")
	hourlyKey := fmt.Sprintf("stats:hourly:%s", dateStr)

	hourlyData, err := vlm.redisClient.HGetAll(vlm.ctx, hourlyKey).Result()
	if err != nil {
		return []TrafficData{}
	}

	// 初始化24小时数据
	result := make([]TrafficData, 0)
	// 4小时一个间隔，或者1小时一个间隔，前端是每4小时
	// 前端 trafficData: 00:00, 04:00, 08:00, 12:00, 16:00, 20:00

	points := []string{"00", "04", "08", "12", "16", "20"}

	for _, p := range points {
		// 聚合4小时窗口
		startHour, _ := strconv.Atoi(p)
		var total int64
		for i := 0; i < 4; i++ {
			h := fmt.Sprintf("%02d", startHour+i)
			if val, ok := hourlyData[h]; ok {
				if v, err := strconv.ParseInt(val, 10, 64); err == nil {
					total += v
				}
			}
		}

		result = append(result, TrafficData{
			Time:          fmt.Sprintf("%s:00", p),
			TotalRequests: total,
			// 爬虫和拦截暂时无法从visit log stats中获取，需要crawler log stats 和 waf stats
			// 这里先填0或从其他地方获取
			CrawlerRequests: 0,
			BlockedRequests: 0,
		})
	}

	return result
}

// GetUnwashedLogs 获取待清洗日志
func (vlm *VisitLogManager) GetUnwashedLogs(count int64) ([]VisitLog, error) {
	unwashedKey := "visit_logs:unwashed"
	var logs []VisitLog

	// 使用Pipeline提高效率? LPop不能pipeline read and return immediately easily in loop
	// Just loop
	for i := int64(0); i < count; i++ {
		logJSON, err := vlm.redisClient.LPop(vlm.ctx, unwashedKey).Result()
		if err == redis.Nil {
			break
		}
		if err != nil {
			return logs, err
		}
		var l VisitLog
		if err := json.Unmarshal([]byte(logJSON), &l); err != nil {
			continue
		}
		logs = append(logs, l)
	}
	return logs, nil
}

// UpdateLog 更新日志
func (vlm *VisitLogManager) UpdateLog(oldLog, newLog VisitLog) error {
	dateStr := oldLog.Time.Format("2006-01-02")
	siteKey := fmt.Sprintf("visit_logs:%s:%s", oldLog.Site, dateStr)
	totalKey := fmt.Sprintf("visit_logs:all:%s", dateStr)

	oldJSON, err := json.Marshal(oldLog)
	if err != nil {
		return err
	}
	newJSON, err := json.Marshal(newLog)
	if err != nil {
		return err
	}

	score := float64(newLog.Time.UnixNano())
	pipe := vlm.redisClient.Pipeline()
	pipe.ZRem(vlm.ctx, siteKey, oldJSON)
	pipe.ZAdd(vlm.ctx, siteKey, &redis.Z{Score: score, Member: newJSON})
	pipe.ZRem(vlm.ctx, totalKey, oldJSON)
	pipe.ZAdd(vlm.ctx, totalKey, &redis.Z{Score: score, Member: newJSON})
	_, err = pipe.Exec(vlm.ctx)
	return err
}

func (vlm *VisitLogManager) startCleanupTask() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	vlm.cleanupOldLogs()
	for range ticker.C {
		vlm.cleanupOldLogs()
	}
}

func (vlm *VisitLogManager) cleanupOldLogs() {
	// 1. 获取配置
	config, err := vlm.redisClient.HGetAll(vlm.ctx, "config:system").Result()
	if err != nil {
		log.Printf("Failed to get system config for log cleanup: %v", err)
		return
	}

	retentionDays := 7
	if val, ok := config["access_log_retention_days"]; ok {
		if days, err := strconv.Atoi(val); err == nil && days > 0 {
			retentionDays = days
		}
	}

	maxSizeMB := 128
	if val, ok := config["access_log_max_size"]; ok {
		if size, err := strconv.Atoi(val); err == nil && size > 0 {
			maxSizeMB = size
		}
	}

	// 2. 按天数清理
	// 扫描所有访问日志key
	// Pattern: visit_logs:all:*
	iter := vlm.redisClient.Scan(vlm.ctx, 0, "visit_logs:all:*", 0).Iterator()
	var allLogKeys []string
	for iter.Next(vlm.ctx) {
		allLogKeys = append(allLogKeys, iter.Val())
	}

	for _, key := range allLogKeys {
		// key format: visit_logs:all:2023-01-01
		parts := strings.Split(key, ":")
		if len(parts) < 3 {
			continue
		}
		dateStr := parts[len(parts)-1]
		logDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		// 检查是否超过保留天数
		if time.Since(logDate).Hours() > float64(retentionDays*24) {
			// 删除总日志
			vlm.redisClient.Del(vlm.ctx, key)
			log.Printf("Deleted old access log: %s", key)

			// 删除该日期的所有站点日志
			// Pattern: visit_logs:*:dateStr
			siteLogIter := vlm.redisClient.Scan(vlm.ctx, 0, fmt.Sprintf("visit_logs:*:%s", dateStr), 0).Iterator()
			for siteLogIter.Next(vlm.ctx) {
				vlm.redisClient.Del(vlm.ctx, siteLogIter.Val())
			}
		}
	}

	// 3. 按大小清理
	// 获取最近保留天数内的所有日志key，按时间倒序排列（最新的在前）
	var validLogKeys []string
	for i := 0; i < retentionDays; i++ {
		dateStr := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		key := fmt.Sprintf("visit_logs:all:%s", dateStr)
		validLogKeys = append(validLogKeys, key)
	}

	currentSize := int64(0)
	maxSizeBytes := int64(maxSizeMB) * 1024 * 1024

	for _, key := range validLogKeys {
		// 获取key的内存占用
		usage, err := vlm.redisClient.MemoryUsage(vlm.ctx, key).Result()
		if err != nil {
			continue
		}

		// 如果累加大小超过限制，删除该日志及更早的日志
		if currentSize+usage > maxSizeBytes {
			// 删除总日志
			vlm.redisClient.Del(vlm.ctx, key)

			// Extract date from key
			parts := strings.Split(key, ":")
			if len(parts) > 0 {
				dateStr := parts[len(parts)-1]
				siteLogIter := vlm.redisClient.Scan(vlm.ctx, 0, fmt.Sprintf("visit_logs:*:%s", dateStr), 0).Iterator()
				for siteLogIter.Next(vlm.ctx) {
					vlm.redisClient.Del(vlm.ctx, siteLogIter.Val())
				}
				log.Printf("Deleted access log due to size limit: %s", key)
			}
		} else {
			currentSize += usage
		}
	}
}

// GetVisitStats 获取访问统计 (3D图所需数据)
// 返回 GeoJSON 格式或 简单的 Location Count 格式
func (vlm *VisitLogManager) GetVisitStats(site string, startTime, endTime time.Time) ([]map[string]interface{}, error) {
	// Aggregate washed logs by lat/lon or city
	// This is heavy if logs are many.
	// For "3D globe", we need lat/lon and magnitude.

	// We iterate logs in range
	days := int(endTime.Sub(startTime).Hours()/24) + 1
	keyPrefix := "visit_logs:all:"
	if site != "" {
		keyPrefix = fmt.Sprintf("visit_logs:%s:", site)
	}

	startScore := float64(startTime.UnixNano())
	endScore := float64(endTime.UnixNano())

	geoStats := make(map[string]map[string]interface{}) // "lat,lon" -> {lat, lon, count}

	for i := 0; i < days; i++ {
		dateStr := startTime.AddDate(0, 0, i).Format("2006-01-02")
		key := keyPrefix + dateStr

		logJSONs, err := vlm.redisClient.ZRangeByScore(vlm.ctx, key, &redis.ZRangeBy{
			Min: fmt.Sprintf("%f", startScore),
			Max: fmt.Sprintf("%f", endScore),
		}).Result()
		if err != nil {
			continue
		}

		for _, logJSON := range logJSONs {
			var l VisitLog
			if err := json.Unmarshal([]byte(logJSON), &l); err != nil {
				continue
			}
			if l.Washed && l.Latitude != 0 && l.Longitude != 0 {
				geoKey := fmt.Sprintf("%.2f,%.2f", l.Latitude, l.Longitude)
				if _, ok := geoStats[geoKey]; !ok {
					geoStats[geoKey] = map[string]interface{}{
						"lat":     l.Latitude,
						"lng":     l.Longitude,
						"count":   int64(0),
						"city":    l.City,
						"country": l.Country,
					}
				}
				geoStats[geoKey]["count"] = geoStats[geoKey]["count"].(int64) + 1
			}
		}
	}

	var result []map[string]interface{}
	for _, v := range geoStats {
		result = append(result, v)
	}
	return result, nil
}
