package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/repository"
)

// MonitoringController 监控控制器
type MonitoringController struct {
	monitor     *monitoring.Monitor
	alertRepo   *repository.AlertRepository
	fwRulesRepo *repository.FirewallRulesRepository
	notifyRepo  *repository.NotificationChannelsRepository
}

// NewMonitoringController 创建监控控制器实例
func NewMonitoringController(monitor *monitoring.Monitor, redisClient *redis.Client) *MonitoringController {
	return &MonitoringController{
		monitor:     monitor,
		alertRepo:   repository.NewAlertRepository(redisClient),
		fwRulesRepo: repository.NewFirewallRulesRepository(redisClient),
		notifyRepo:  repository.NewNotificationChannelsRepository(redisClient),
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

// AlertRecord 告警记录（持久化结构由 AlertRepository 统一持有）
type AlertRecord = repository.AlertRecord

// AlertRuleData 前端告警规则数据结构（持久化结构由 AlertRepository 统一持有）
type AlertRuleData = repository.AlertRuleData

// SaveAlertRecord 保存告警记录
// 使用 ZSet（score=时间戳）存储，修复旧方案按 UUID 字典序排序不正确的问题
func (c *MonitoringController) SaveAlertRecord(record AlertRecord) {
	c.alertRepo.AppendAlertHistory(record)
}

// GetAlertHistory 获取告警历史（最新的在前）
func (c *MonitoringController) GetAlertHistory(ctx *gin.Context) {
	limitStr := ctx.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 200 {
		limit = 50
	}
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "success", "data": c.alertRepo.GetAlertHistory(limit)})
}

// GetAlertRules 获取告警规则
func (c *MonitoringController) GetAlertRules(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "success", "data": c.alertRepo.GetAlertRules()})
}

// SaveAlertRule 保存告警规则
func (c *MonitoringController) SaveAlertRule(ctx *gin.Context) {
	var rule AlertRuleData
	if err := ctx.ShouldBindJSON(&rule); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := c.alertRepo.SaveAlertRule(rule); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to save alert rule"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "success"})
}

// DeleteAlertRule 删除告警规则
func (c *MonitoringController) DeleteAlertRule(ctx *gin.Context) {
	ruleID := ctx.Param("id")
	if ruleID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Rule ID is required"})
		return
	}

	if err := c.alertRepo.DeleteAlertRule(ruleID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to delete alert rule"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "success"})
}

// GetFirewallRules 获取防火墙规则
func (c *MonitoringController) GetFirewallRules(ctx *gin.Context) {
	siteID := ctx.Query("site_id")
	if siteID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "site_id is required"})
		return
	}
	data, err := c.fwRulesRepo.Get(siteID)
	if err != nil || data == nil {
		ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "success", "data": gin.H{"rules": []interface{}{}}})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "success", "data": data})
}

// SaveFirewallRules 保存防火墙规则
func (c *MonitoringController) SaveFirewallRules(ctx *gin.Context) {
	var req struct {
		SiteID string        `json:"site_id"`
		Rules  []interface{} `json:"rules"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if req.SiteID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "site_id is required"})
		return
	}

	data, err := json.Marshal(gin.H{"rules": req.Rules})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "failed to serialize firewall rules"})
		return
	}
	if err := c.fwRulesRepo.Save(req.SiteID, data); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to save firewall rules"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "success"})
}

// DeleteFirewallRule 删除防火墙规则
func (c *MonitoringController) DeleteFirewallRule(ctx *gin.Context) {
	ruleID := ctx.Param("id")
	siteID := ctx.Query("site_id")
	if ruleID == "" || siteID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "rule id and site_id are required"})
		return
	}

	if err := c.fwRulesRepo.DeleteRule(siteID, ruleID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to delete firewall rule"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "success"})
}

// NotificationChannelData 通知渠道配置数据
type NotificationChannelData struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

// GetNotificationChannels 获取通知渠道配置
func (c *MonitoringController) GetNotificationChannels(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "success", "data": c.notifyRepo.Get()})
}

// SaveNotificationChannels 保存通知渠道配置
func (c *MonitoringController) SaveNotificationChannels(ctx *gin.Context) {
	var channels []repository.NotificationChannelData
	if err := ctx.ShouldBindJSON(&channels); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := c.notifyRepo.Save(channels); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to save notification channels"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "success"})
}
