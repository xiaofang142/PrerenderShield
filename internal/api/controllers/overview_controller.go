package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/config"
	"prerender-shield/internal/monitoring"
)

// OverviewController 概览控制器
type OverviewController struct {
	cfg     *config.Config
	monitor *monitoring.Monitor
}

// NewOverviewController 创建概览控制器实例
func NewOverviewController(cfg *config.Config, monitor *monitoring.Monitor) *OverviewController {
	return &OverviewController{
		cfg:     cfg,
		monitor: monitor,
	}
}

// GetOverview 获取概览信息
func (c *OverviewController) GetOverview(ctx *gin.Context) {
	// 计算总防火墙和渲染预热启用状态
	firewallEnabled := false
	prerenderEnabled := false
	for _, site := range c.cfg.Sites {
		if site.Firewall.Enabled {
			firewallEnabled = true
		}
		if site.Prerender.Enabled {
			prerenderEnabled = true
		}
	}

	// 获取真实监控数据
	stats := c.monitor.GetStats()

	// 获取站点统计数据
	activeSites := len(c.cfg.Sites)
	sslCertificates := 0 // SSL功能已移除

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"totalRequests":    int64(stats["totalRequests"].(float64)),
			"crawlerRequests":  int64(stats["crawlerRequests"].(float64)),
			"blockedRequests":  int64(stats["blockedRequests"].(float64)),
			"cacheHitRate":     float64(int(stats["cacheHitRate"].(float64)*100)) / 100, // 保留两位小数
			"activeBrowsers":   int(stats["activeBrowsers"].(float64)),
			"activeSites":      activeSites,
			"sslCertificates":  sslCertificates,
			"firewallEnabled":  firewallEnabled,
			"prerenderEnabled": prerenderEnabled,
			// 暂时保留模拟数据，后续可以替换为真实数据
			"geoData": gin.H{
				"countryData": []gin.H{
					{"country": "中国", "count": 891800, "color": "#1890ff"},
					{"country": "美国", "count": 2300, "color": "#52c41a"},
					{"country": "爱尔兰", "count": 461, "color": "#faad14"},
					{"country": "澳大利亚", "count": 361, "color": "#f5222d"},
					{"country": "新加坡", "count": 221, "color": "#722ed1"},
					{"country": "印度", "count": 157, "color": "#fa8c16"},
					{"country": "日本", "count": 133, "color": "#eb2f96"},
				},
				"mapData": []gin.H{
					{"name": "中国", "value": 891800},
					{"name": "美国", "value": 2300},
					{"name": "爱尔兰", "value": 461},
					{"name": "澳大利亚", "value": 361},
					{"name": "新加坡", "value": 221},
					{"name": "印度", "value": 157},
					{"name": "日本", "value": 133},
				},
			},
			"trafficData": []gin.H{
				{"time": "00:00", "totalRequests": int(stats["totalRequests"].(float64)) / 6, "crawlerRequests": int(stats["crawlerRequests"].(float64)) / 6, "blockedRequests": int(stats["blockedRequests"].(float64)) / 6},
				{"time": "04:00", "totalRequests": int(stats["totalRequests"].(float64)) / 8, "crawlerRequests": int(stats["crawlerRequests"].(float64)) / 8, "blockedRequests": int(stats["blockedRequests"].(float64)) / 8},
				{"time": "08:00", "totalRequests": int(stats["totalRequests"].(float64)) / 5, "crawlerRequests": int(stats["crawlerRequests"].(float64)) / 5, "blockedRequests": int(stats["blockedRequests"].(float64)) / 5},
				{"time": "12:00", "totalRequests": int(stats["totalRequests"].(float64)) / 3, "crawlerRequests": int(stats["crawlerRequests"].(float64)) / 3, "blockedRequests": int(stats["blockedRequests"].(float64)) / 3},
				{"time": "16:00", "totalRequests": int(stats["totalRequests"].(float64)) / 2, "crawlerRequests": int(stats["crawlerRequests"].(float64)) / 2, "blockedRequests": int(stats["blockedRequests"].(float64)) / 2},
				{"time": "20:00", "totalRequests": int(stats["totalRequests"].(float64)) / 4, "crawlerRequests": int(stats["crawlerRequests"].(float64)) / 4, "blockedRequests": int(stats["blockedRequests"].(float64)) / 4},
			},
			"accessStats": gin.H{
				"pv": int(stats["totalRequests"].(float64)) * 50,
				"uv": 100 + int(stats["totalRequests"].(float64))/100,
				"ip": 50 + int(stats["totalRequests"].(float64))/50,
			},
		},
	})
}
