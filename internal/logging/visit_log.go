package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

// VisitLog 正常用户访问日志结构体
type VisitLog struct {
	ID        string    `json:"id"`
	Site      string    `json:"site"`
	IP        string    `json:"ip"`
	Time      time.Time `json:"time"`
	Method    string    `json:"method"`
	URL       string    `json:"url"`
	Status    int       `json:"status"`
	UA        string    `json:"ua"`
	Duration  float64   `json:"duration"` // 请求耗时（秒）
	Referer   string    `json:"referer"`

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
	
	pipe.Exec(vlm.ctx)
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
	if err != nil { return err }
	newJSON, err := json.Marshal(newLog)
	if err != nil { return err }

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
	fifteenDaysAgo := time.Now().AddDate(0, 0, -15)
	dateStr := fifteenDaysAgo.Format("2006-01-02")
	key := fmt.Sprintf("visit_logs:all:%s", dateStr)
	vlm.redisClient.Del(vlm.ctx, key)
	// Note: site specific keys are harder to clean without iterating sites.
	// Since we set TTL on keys, they should expire automatically!
	// The explicit delete here is just extra cleanup for the 'all' key.
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
		if err != nil { continue }
		
		for _, logJSON := range logJSONs {
			var l VisitLog
			if err := json.Unmarshal([]byte(logJSON), &l); err != nil { continue }
			if l.Washed && l.Latitude != 0 && l.Longitude != 0 {
				geoKey := fmt.Sprintf("%.2f,%.2f", l.Latitude, l.Longitude)
				if _, ok := geoStats[geoKey]; !ok {
					geoStats[geoKey] = map[string]interface{}{
						"lat": l.Latitude,
						"lng": l.Longitude,
						"count": int64(0),
						"city": l.City,
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
