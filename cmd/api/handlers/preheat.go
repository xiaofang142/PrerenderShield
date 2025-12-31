package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"prerender-shield/internal/prerender"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/scheduler"
)

// PreheatHandler 预热API处理器
type PreheatHandler struct {
	engineManager *prerender.EngineManager
	scheduler     *scheduler.Scheduler
	redisClient   *redis.Client
}

// NewPreheatHandler 创建新的预热API处理器
func NewPreheatHandler(engineManager *prerender.EngineManager, scheduler *scheduler.Scheduler, redisClient *redis.Client) *PreheatHandler {
	return &PreheatHandler{
		engineManager: engineManager,
		scheduler:     scheduler,
		redisClient:   redisClient,
	}
}

// RegisterRoutes 注册预热相关路由
func (h *PreheatHandler) RegisterRoutes(router *gin.RouterGroup) {
	preheatGroup := router.Group("/preheat")
	{
		// 获取站点列表（仅静态网站）
		preheatGroup.GET("/sites", h.GetStaticSites)

		// 获取预热统计数据
		preheatGroup.GET("/stats", h.GetPreheatStats)

		// 触发站点预热
		preheatGroup.POST("/trigger", h.TriggerPreheat)

		// 手动预热指定URL
		preheatGroup.POST("/url", h.PreheatURL)

		// 获取URL列表
		preheatGroup.GET("/urls", h.GetURLs)

		// 获取任务状态
		preheatGroup.GET("/task/status", h.GetTaskStatus)

		// 获取爬虫协议头列表
		preheatGroup.GET("/crawler-headers", h.GetCrawlerHeaders)
	}
}

// GetStaticSites 获取静态网站列表
func (h *PreheatHandler) GetStaticSites(c *gin.Context) {
	engines := h.engineManager.GetEngines()

	var sites []gin.H
	for siteName, engine := range engines {
		// 只返回静态模式的站点
		if engine.GetConfig().Mode != "static" {
			continue
		}

		sites = append(sites, gin.H{
			"name":    siteName,
			"domain":  engine.GetConfig().Domains[0],
			"enabled": engine.GetConfig().Enabled,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    sites,
	})
}

// GetPreheatStats 获取预热统计数据
func (h *PreheatHandler) GetPreheatStats(c *gin.Context) {
	siteName := c.Query("siteName")

	if siteName == "" {
		// 获取所有站点的统计数据
		var allStats []gin.H
		engines := h.engineManager.GetEngines()

		for name, engine := range engines {
			if engine.GetConfig().Mode != "static" {
				continue
			}

			stats, err := engine.GetPreheatManager().GetStats()
			if err != nil {
				continue
			}

			allStats = append(allStats, gin.H{
				"siteName":        name,
				"urlCount":        stats["url_count"],
				"cacheCount":      stats["cache_count"],
				"totalCacheSize":  stats["total_cache_size"],
				"lastPreheatTime": stats["last_preheat_time"],
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "success",
			"data":    allStats,
		})
		return
	}

	// 获取指定站点的统计数据
	engine := h.engineManager.GetEngine(siteName)
	if engine == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": "Site not found",
		})
		return
	}

	stats, err := engine.GetPreheatManager().GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "Failed to get preheat stats",
		})
		return
	}

	// 获取浏览器池大小
	browserPoolSize := engine.GetConfig().PoolSize

	// 获取缓存数量
	cacheCount, _ := h.redisClient.GetCacheCount(siteName)

	// 获取URL数量
	urlCount, _ := h.redisClient.GetURLCount(siteName)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"siteName":        siteName,
			"urlCount":        urlCount,
			"cacheCount":      cacheCount,
			"browserPoolSize": browserPoolSize,
			"totalCacheSize":  stats["total_cache_size"],
			"lastPreheatTime": stats["last_preheat_time"],
		},
	})
}

// TriggerPreheat 触发站点预热
func (h *PreheatHandler) TriggerPreheat(c *gin.Context) {
	var req struct {
		SiteName string `json:"siteName" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "Invalid request",
		})
		return
	}

	// 检查站点是否存在
	engine := h.engineManager.GetEngine(req.SiteName)
	if engine == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": "Site not found",
		})
		return
	}

	// 异步触发预热
	go func() {
		if err := engine.GetPreheatManager().TriggerPreheat(); err != nil {
			// 预热失败，记录日志
			// 可以考虑添加错误通知机制
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Preheat triggered successfully",
	})
}

// PreheatURL 手动预热指定URL
func (h *PreheatHandler) PreheatURL(c *gin.Context) {
	var req struct {
		SiteName string   `json:"siteName" binding:"required"`
		URLs     []string `json:"urls" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "Invalid request",
		})
		return
	}

	// 检查站点是否存在
	engine := h.engineManager.GetEngine(req.SiteName)
	if engine == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": "Site not found",
		})
		return
	}

	// 异步预热URL
	go func() {
		preheatManager := engine.GetPreheatManager()
		for _, url := range req.URLs {
			if err := preheatManager.TriggerPreheatForURL(url); err != nil {
				// 预热失败，记录日志
			}
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "URL preheat triggered successfully",
	})
}

// GetURLs 获取URL列表
func (h *PreheatHandler) GetURLs(c *gin.Context) {
	siteName := c.Query("siteName")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 获取URL列表
	urls, err := h.redisClient.GetURLs(siteName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "Failed to get URLs",
		})
		return
	}

	// 分页处理
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= len(urls) {
		urls = []string{}
	} else if end > len(urls) {
		urls = urls[start:]
	} else {
		urls = urls[start:end]
	}

	// 计算总数
	total := len(urls)

	// 获取每个URL的预热状态
	var urlStatusList []gin.H
	for _, url := range urls {
		status, err := h.redisClient.GetURLPreheatStatus(siteName, url)
		if err != nil {
			continue
		}

		urlStatusList = append(urlStatusList, gin.H{
			"url":       url,
			"status":    status["status"],
			"cacheSize": status["cache_size"],
			"updatedAt": status["updated_at"],
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"list":     urlStatusList,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

// GetTaskStatus 获取任务状态
func (h *PreheatHandler) GetTaskStatus(c *gin.Context) {
	siteName := c.Query("siteName")

	if siteName == "" {
		// 获取所有站点的任务状态
		allStatus := h.scheduler.ListTasks()

		var tasks []gin.H
		for name, nextRun := range allStatus {
			tasks = append(tasks, gin.H{
				"siteName": name,
				"nextRun":  nextRun,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "success",
			"data":    tasks,
		})
		return
	}

	// 获取指定站点的任务状态
	scheduled, nextRun := h.scheduler.GetTaskStatus(siteName)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"siteName":  siteName,
			"scheduled": scheduled,
			"nextRun":   nextRun,
		},
	})
}

// GetCrawlerHeaders 获取爬虫协议头列表
func (h *PreheatHandler) GetCrawlerHeaders(c *gin.Context) {
	// 获取默认爬虫协议头
	defaultHeaders := prerender.GetDefaultCrawlerHeaders()

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    defaultHeaders,
	})
}
