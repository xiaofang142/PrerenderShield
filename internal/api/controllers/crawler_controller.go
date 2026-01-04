package controllers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/logging"
)

// CrawlerController 爬虫控制器
type CrawlerController struct {
	crawlerLogMgr *logging.CrawlerLogManager
}

// NewCrawlerController 创建爬虫控制器实例
func NewCrawlerController(crawlerLogMgr *logging.CrawlerLogManager) *CrawlerController {
	return &CrawlerController{
		crawlerLogMgr: crawlerLogMgr,
	}
}

// GetCrawlerLogs 获取爬虫日志
func (c *CrawlerController) GetCrawlerLogs(ctx *gin.Context) {
	// 获取爬虫日志
	site := ctx.Query("site")
	startTimeStr := ctx.DefaultQuery("startTime", time.Now().Add(-24*time.Hour).Format(time.RFC3339))
	endTimeStr := ctx.DefaultQuery("endTime", time.Now().Format(time.RFC3339))
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "10"))

	// 解析时间
	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		startTime = time.Now().Add(-24 * time.Hour)
	}
	endTime, err := time.Parse(time.RFC3339, endTimeStr)
	if err != nil {
		endTime = time.Now()
	}

	// 获取日志
	logs, total, err := c.crawlerLogMgr.GetCrawlerLogs(site, startTime, endTime, page, pageSize)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "Failed to get crawler logs",
		})
		return
	}

	// 转换为前端需要的格式
	var items []gin.H
	for _, log := range logs {
		items = append(items, gin.H{
			"id":         log.ID,
			"site":       log.Site,
			"ip":         log.IP,
			"time":       log.Time.Format(time.RFC3339),
			"hitCache":   log.HitCache,
			"route":      log.Route,
			"ua":         log.UA,
			"status":     log.Status,
			"method":     log.Method,
			"cacheTTL":   log.CacheTTL,
			"renderTime": log.RenderTime,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"items":    items,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

// GetCrawlerStats 获取爬虫统计数据
func (c *CrawlerController) GetCrawlerStats(ctx *gin.Context) {
	// 获取爬虫统计数据
	site := ctx.Query("site")
	startTimeStr := ctx.DefaultQuery("startTime", time.Now().Add(-24*time.Hour).Format(time.RFC3339))
	endTimeStr := ctx.DefaultQuery("endTime", time.Now().Format(time.RFC3339))
	granularity := ctx.DefaultQuery("granularity", "hour")

	// 解析时间
	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		startTime = time.Now().Add(-24 * time.Hour)
	}
	endTime, err := time.Parse(time.RFC3339, endTimeStr)
	if err != nil {
		endTime = time.Now()
	}

	// 获取统计数据
	stats, err := c.crawlerLogMgr.GetCrawlerStats(site, startTime, endTime, granularity)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "Failed to get crawler stats",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    stats,
	})
}
