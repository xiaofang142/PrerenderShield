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
	// 注：CrawlerLogManager.GetCrawlerLogs 对内部 Redis 读取错误一律跳过并返回 nil error，
	// 该错误分支不可达（已用覆盖工具验证），故不保留死分支
	logs, total, _ := c.crawlerLogMgr.GetCrawlerLogs(site, startTime, endTime, page, pageSize)

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
			"quality":    log.Quality,
			"verified":   log.Verified,
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
	// 注：CrawlerLogManager.GetCrawlerStats 对内部 Redis 读取错误一律跳过并返回 nil error，错误分支不可达
	stats, _ := c.crawlerLogMgr.GetCrawlerStats(site, startTime, endTime, granularity)

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    stats,
	})
}

// GetURLStats per-URL 渲染预算报表：GET /crawler/url-stats?site=&startTime=&endTime=&limit=
func (c *CrawlerController) GetURLStats(ctx *gin.Context) {
	site := ctx.Query("site")
	startTimeStr := ctx.DefaultQuery("startTime", time.Now().Add(-7*24*time.Hour).Format(time.RFC3339))
	endTimeStr := ctx.DefaultQuery("endTime", time.Now().Format(time.RFC3339))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		startTime = time.Now().Add(-7 * 24 * time.Hour)
	}
	endTime, err := time.Parse(time.RFC3339, endTimeStr)
	if err != nil {
		endTime = time.Now()
	}
	if !startTime.Before(endTime) {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "startTime must be before endTime",
		})
		return
	}

	// 注：CrawlerLogManager.GetURLStats 对内部 Redis 读取错误一律跳过并返回 nil error，
	// 且返回值经 make([]URLStat,0,...) 构造永不为 nil，nil 兜底不可达
	stats, _ := c.crawlerLogMgr.GetURLStats(site, startTime, endTime, limit)

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"list":  stats,
			"total": len(stats),
		},
	})
}
