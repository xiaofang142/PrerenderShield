package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/monitoring"
)

// MonitoringController 监控控制器
type MonitoringController struct {
	monitor *monitoring.Monitor
}

// NewMonitoringController 创建监控控制器实例
func NewMonitoringController(monitor *monitoring.Monitor) *MonitoringController {
	return &MonitoringController{
		monitor: monitor,
	}
}

// GetStats 获取监控统计数据
func (c *MonitoringController) GetStats(ctx *gin.Context) {
	// 获取监控统计数据
	stats := c.monitor.GetStats()
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    stats,
	})
}
