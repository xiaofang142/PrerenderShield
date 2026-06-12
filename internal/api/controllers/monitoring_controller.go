package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/redis"
)

// MonitoringController 监控控制器
type MonitoringController struct {
	monitor     *monitoring.Monitor
	redisClient *redis.Client
}

// NewMonitoringController 创建监控控制器实例
func NewMonitoringController(monitor *monitoring.Monitor, redisClient *redis.Client) *MonitoringController {
	return &MonitoringController{
		monitor:     monitor,
		redisClient: redisClient,
	}
}

// GetStats 获取监控统计数据
func (c *MonitoringController) GetStats(ctx *gin.Context) {
	stats := c.monitor.GetStats()
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    stats,
	})
}

// AlertRecord 告警记录
type AlertRecord struct {
	ID        string    `json:"id"`
	Level     string    `json:"level"`
	Rule      string    `json:"rule"`
	Message   string    `json:"message"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Timestamp time.Time `json:"timestamp"`
}

// SaveAlertRecord 保存告警记录
func (c *MonitoringController) SaveAlertRecord(record AlertRecord) {
	if c.redisClient == nil {
		return
	}
	data, _ := json.Marshal(record)
	key := "alert:history:" + record.Timestamp.Format("20060102150405")
	c.redisClient.Set(key, string(data), 30*24*time.Hour)
}

// GetAlertHistory 获取告警历史
func (c *MonitoringController) GetAlertHistory(ctx *gin.Context) {
	if c.redisClient == nil {
		ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "success", "data": []interface{}{}})
		return
	}
	limitStr := ctx.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	keys, err := c.redisClient.Keys("alert:history:*")
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "success", "data": []interface{}{}})
		return
	}

	// 倒序排列（最新的在前）
	if len(keys) > limit {
		keys = keys[len(keys)-limit:]
	}

	var records []AlertRecord
	for i := len(keys) - 1; i >= 0; i-- {
		val, err := c.redisClient.Get(keys[i])
		if err != nil || val == "" {
			continue
		}
		var record AlertRecord
		if err := json.Unmarshal([]byte(val), &record); err == nil {
			records = append(records, record)
		}
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "success", "data": records})
}
