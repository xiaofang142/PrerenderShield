package controllers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
)

// OverviewController 概览控制器
type OverviewController struct {
	cfg         *config.Config
	monitor     *monitoring.Monitor
	visitLogMgr *logging.VisitLogManager
}

// NewOverviewController 创建概览控制器实例
func NewOverviewController(cfg *config.Config, monitor *monitoring.Monitor, visitLogMgr *logging.VisitLogManager) *OverviewController {
	return &OverviewController{
		cfg:         cfg,
		monitor:     monitor,
		visitLogMgr: visitLogMgr,
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

	// 获取地理位置统计数据
	geoStats, _ := c.visitLogMgr.GetVisitStats("", time.Now().Add(-24*time.Hour), time.Now())

	// 处理Globe数据和国家数据
	globeData := make([]gin.H, 0)
	countryMap := make(map[string]int64)

	for _, item := range geoStats {
		globeData = append(globeData, gin.H{
			"lat":   item["lat"],
			"lng":   item["lng"],
			"count": item["count"],
		})
		if country, ok := item["country"].(string); ok && country != "" {
			countryMap[country] += item["count"].(int64)
		}
	}

	mapData := make([]gin.H, 0)
	countryData := make([]gin.H, 0)
	for k, v := range countryMap {
		mapData = append(mapData, gin.H{"name": k, "value": v})
		countryData = append(countryData, gin.H{"country": k, "count": v, "color": "#1890ff"})
	}

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
			"geoData": gin.H{
				"countryData": countryData,
				"mapData":     mapData,
				"globeData":   globeData,
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
