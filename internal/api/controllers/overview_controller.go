package controllers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/repository"
	"prerender-shield/internal/services"
	"prerender-shield/internal/ssl"
	"prerender-shield/internal/utils/country"
)

// OverviewController 概览控制器
type OverviewController struct {
	cfg           *config.Config
	monitor       *monitoring.Monitor
	visitLogMgr   *logging.VisitLogManager
	crawlerLogMgr *logging.CrawlerLogManager
	wafStatsSvc   *services.OverviewService
	sslMgr        ssl.Manager
}

// NewOverviewController 创建概览控制器实例
func NewOverviewController(cfg *config.Config, monitor *monitoring.Monitor, visitLogMgr *logging.VisitLogManager, crawlerLogMgr *logging.CrawlerLogManager, wafStatsSvc *repository.WafRepository, sslMgr ssl.Manager) *OverviewController {
	return &OverviewController{
		cfg:           cfg,
		monitor:       monitor,
		visitLogMgr:   visitLogMgr,
		crawlerLogMgr: crawlerLogMgr,
		wafStatsSvc:   services.NewOverviewService(wafStatsSvc),
		sslMgr:        sslMgr,
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

	// 获取 WAF 统计数据 (最近 24 小时)
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	totalRequests := int64(0)
	if v, ok := stats["totalRequests"].(float64); ok {
		totalRequests = int64(v)
	}

	// 如果 WAF stats 可用，优先使用 DB 中的数据
	if c.wafStatsSvc != nil {
		wafStats, err := c.wafStatsSvc.GetWafGlobalStats(startTime.Format("2006-01-02 15:04:05"), endTime.Format("2006-01-02 15:04:05"))
		if err == nil && wafStats != nil {
			totalRequests = wafStats.TotalRequests
		}
	}

	// 获取站点统计数据
	activeSites := len(c.cfg.Sites)

	// 从 SSL 管理器获取实际证书数量
	sslCertificates := 0
	if c.sslMgr != nil {
		if certs, err := c.sslMgr.ListCertificates(); err == nil {
			sslCertificates = len(certs)
		}
	}

	// 获取地理位置统计数据
	geoStats, _ := c.visitLogMgr.GetVisitStats("", time.Now().Add(-24*time.Hour), time.Now())

	// 获取PV/UV/IP统计数据（过去24小时）
	pv, uv, ip := c.visitLogMgr.GetAccessStats(time.Now().Add(-24*time.Hour), time.Now())

	// 获取流量趋势数据
	trafficData := c.visitLogMgr.GetTrafficTrend(time.Now(), time.Now())

	// 使用 CrawlerLogManager 获取真实的爬虫和拦截统计数据
	var crawlerTotal, blockedTotal int64
	if c.crawlerLogMgr != nil {
		crawlerTrafficData := c.crawlerLogMgr.GetTrafficTrend(startTime, endTime)

		// 计算爬虫请求总数和拦截总数
		for _, data := range crawlerTrafficData {
			crawlerTotal += data.CrawlerRequests
			blockedTotal += data.BlockedRequests
		}

		// 合并流量趋势数据
		if len(trafficData) == len(crawlerTrafficData) {
			for i := range trafficData {
				trafficData[i].CrawlerRequests = crawlerTrafficData[i].CrawlerRequests
				trafficData[i].BlockedRequests = crawlerTrafficData[i].BlockedRequests
			}
		} else if len(trafficData) > 0 && len(crawlerTrafficData) > 0 {
			// 如果长度不匹配，使用第一个数据作为参考
			for i := range trafficData {
				if i < len(crawlerTrafficData) {
					trafficData[i].CrawlerRequests = crawlerTrafficData[i].CrawlerRequests
					trafficData[i].BlockedRequests = crawlerTrafficData[i].BlockedRequests
				}
			}
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
			if count, ok := item["count"].(int64); ok {
				countryMap[countryName] += count
			}
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
			"totalRequests":   totalRequests,
			"crawlerRequests": crawlerTotal,
			"blockedRequests": blockedTotal,
			"cacheHitRate": func() float64 {
				if v, ok := stats["cacheHitRate"].(float64); ok {
					return float64(int(v*100)) / 100
				}
				return 0
			}(),
			"activeBrowsers": func() int {
				if v, ok := stats["activeBrowsers"].(float64); ok {
					return int(v)
				}
				return 0
			}(),
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
