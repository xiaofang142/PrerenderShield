package controllers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/repository"
	"prerender-shield/internal/utils/country"
)

// OverviewController 概览控制器
type OverviewController struct {
	cfg         *config.Config
	monitor     *monitoring.Monitor
	visitLogMgr *logging.VisitLogManager
	wafRepo     *repository.WafRepository
}

// NewOverviewController 创建概览控制器实例
func NewOverviewController(cfg *config.Config, monitor *monitoring.Monitor, visitLogMgr *logging.VisitLogManager, wafRepo *repository.WafRepository) *OverviewController {
	return &OverviewController{
		cfg:         cfg,
		monitor:     monitor,
		visitLogMgr: visitLogMgr,
		wafRepo:     wafRepo,
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

	// 获取WAF统计数据 (最近24小时)
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)
	wafStats, err := c.wafRepo.GetGlobalStats(startTime.Format("2006-01-02 15:04:05"), endTime.Format("2006-01-02 15:04:05"))
	
	totalRequests := int64(stats["totalRequests"].(float64))
	blockedRequests := int64(stats["blockedRequests"].(float64))
	
	// 如果WAF stats可用，优先使用 DB 中的数据
	if err == nil && wafStats != nil {
		totalRequests = wafStats.TotalRequests
		blockedRequests = wafStats.BlockedRequests
	}

	// 获取站点统计数据
	activeSites := len(c.cfg.Sites)
	sslCertificates := 0 // SSL功能已移除

	// 获取地理位置统计数据
	geoStats, _ := c.visitLogMgr.GetVisitStats("", time.Now().Add(-24*time.Hour), time.Now())

	// 获取PV/UV/IP统计数据
	pv, uv, ip := c.visitLogMgr.GetAccessStats(time.Now(), time.Now())

	// 获取流量趋势数据
	trafficData := c.visitLogMgr.GetTrafficTrend(time.Now(), time.Now())
	
	// 简单的流量趋势补充：爬虫和拦截请求（按比例分配或者简单的平均，因为暂时没有小时级的爬虫/拦截统计）
	// TODO: 实现小时级的爬虫和拦截统计
	crawlerTotal := int64(stats["crawlerRequests"].(float64))
	blockedTotal := blockedRequests
	
	// 将总数分配到各个时间段（平滑分配，仅作为展示）
	// 注意：这是一个临时的展示策略，直到我们有真实的时间序列数据
	if len(trafficData) > 0 {
		avgCrawler := crawlerTotal / int64(len(trafficData))
		avgBlocked := blockedTotal / int64(len(trafficData))
		for i := range trafficData {
			// 如果该时段有总请求，则显示爬虫和拦截（但不超过总请求）
			// 这里仅仅是简单的模拟分布，真实数据需要 CrawlerLogManager 支持 GetTrafficTrend
			trafficData[i].CrawlerRequests = avgCrawler
			trafficData[i].BlockedRequests = avgBlocked
		}
	}

	// 处理Globe数据和国家数据
	globeData := make([]gin.H, 0)
	countryMap := make(map[string]int64)

	for _, item := range geoStats {
		globeData = append(globeData, gin.H{
			"lat":   item["lat"],
			"lng":   item["lng"],
			"count": item["count"],
		})
		
		var countryName string
		// 优先使用 CountryCode 进行映射
		if code, ok := item["country_code"].(string); ok && code != "" {
			countryName = country.GetCountryName(code)
		} else if name, ok := item["country"].(string); ok && name != "" {
			// 回退到使用 Country Name
			countryName = country.GetCountryName(name)
		}

		if countryName != "" {
			countryMap[countryName] += item["count"].(int64)
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
			"totalRequests":    totalRequests,
			"crawlerRequests":  crawlerTotal,
			"blockedRequests":  blockedTotal,
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
			"trafficData": trafficData,
			"accessStats": gin.H{
				"pv": pv,
				"uv": uv,
				"ip": ip,
			},
		},
	})
}
