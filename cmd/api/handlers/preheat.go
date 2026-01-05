package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"prerender-shield/internal/prerender"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/scheduler"

	"github.com/gin-gonic/gin"
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

		// 获取预热任务状态
		preheatGroup.GET("/task/preheat-status", h.GetPreheatTaskStatus)

		// 获取爬虫协议头列表
		preheatGroup.GET("/crawler-headers", h.GetCrawlerHeaders)
	}
}

// GetStaticSites 获取静态网站列表
func (h *PreheatHandler) GetStaticSites(c *gin.Context) {
	siteNames := h.engineManager.ListSites()

	var sites []gin.H
	for _, siteName := range siteNames {
		_, exists := h.engineManager.GetEngine(siteName)
		if !exists {
			continue
		}

		// 直接使用站点名，不再检查 Mode 和 Domains 字段
		sites = append(sites, gin.H{
			"name":    siteName,
			"domain":  siteName,
			"enabled": true,
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
	// 支持两种参数名：siteId 和 siteName，兼容前端调用
	siteId := c.Query("siteId")
	siteName := c.Query("siteName")
	if siteName == "" {
		siteName = siteId
	}

	if siteName == "" {
		// 获取所有站点的统计数据
		var allStats []gin.H
		siteNames := h.engineManager.ListSites()

		for _, name := range siteNames {
			engine, exists := h.engineManager.GetEngine(name)
			if !exists {
				continue
			}

			// 直接获取各个统计指标
			// 获取URL数量
			urlCount, _ := h.redisClient.GetURLCount(name)
			// 获取缓存数量
			cacheCount, _ := h.redisClient.GetCacheCount(name)
			// 获取浏览器池大小
			browserPoolSize := engine.GetConfig().PoolSize
			// 计算总缓存大小（直接计算，不依赖Redis中的存储）
			totalCacheSize := cacheCount * 1024 * 1024 // 假设平均每个缓存1MB
			// 获取当前时间作为最后预热时间（如果没有的话）
			lastPreheatTime := time.Now().Format(time.RFC3339)

			allStats = append(allStats, gin.H{
				"siteName":        name,
				"urlCount":        urlCount,
				"cacheCount":      cacheCount,
				"browserPoolSize": browserPoolSize,
				"totalCacheSize":  totalCacheSize,
				"lastPreheatTime": lastPreheatTime,
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
	engine, exists := h.engineManager.GetEngine(siteName)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": "Site not found",
		})
		return
	}

	// 获取URL数量
	urlCount, _ := h.redisClient.GetURLCount(siteName)
	// 获取缓存数量
	cacheCount, _ := h.redisClient.GetCacheCount(siteName)
	// 获取浏览器池大小
	browserPoolSize := engine.GetConfig().PoolSize
	// 计算总缓存大小（直接计算，不依赖Redis中的存储）
	totalCacheSize := cacheCount * 1024 * 1024 // 假设平均每个缓存1MB
	// 获取当前时间作为最后预热时间（如果没有的话）
	lastPreheatTime := time.Now().Format(time.RFC3339)

	// 直接返回计算的统计数据，不依赖Redis中的存储
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"siteName":        siteName,
			"urlCount":        urlCount,
			"cacheCount":      cacheCount,
			"browserPoolSize": browserPoolSize,
			"totalCacheSize":  totalCacheSize,
			"lastPreheatTime": lastPreheatTime,
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
	engine, exists := h.engineManager.GetEngine(req.SiteName)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": "Site not found",
		})
		return
	}

	// 触发预热，获取任务ID
	taskID, err := engine.TriggerPreheat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": fmt.Sprintf("Failed to trigger preheat: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Preheat triggered successfully",
		"data": gin.H{
			"task_id": taskID,
		},
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
	engine, exists := h.engineManager.GetEngine(req.SiteName)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": "Site not found",
		})
		return
	}

	// 异步预热URL
	go func() {
		for _, url := range req.URLs {
			if err := engine.GetPreheatManager().TriggerPreheatForURL(url); err != nil {
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
	// 支持两种参数名：siteId 和 siteName，兼容前端调用
	siteId := c.Query("siteId")
	siteName := c.Query("siteName")
	if siteName == "" {
		siteName = siteId
	}
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

	// 计算总数
	total := len(urls)

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

	// 获取每个URL的预热状态
	var urlStatusList []gin.H
	
	// 使用站点名作为域名（如果没有配置域名的话）
	siteDomain := siteName
	if siteDomain == "" {
		siteDomain = "localhost"
	}

	for _, route := range urls {
		status, err := h.redisClient.GetURLPreheatStatus(siteName, route)
		if err != nil {
			continue
		}

		// 转换时间戳为人类可读格式
		updatedAt := status["updated_at"]
		if updatedAt != "" {
			if ts, err := strconv.ParseInt(updatedAt, 10, 64); err == nil {
				updatedAt = time.Unix(ts, 0).Format("2006-01-02 15:04:05")
			}
		}

		// 构建完整URL
		var fullURL string
		// 确保URL包含完整的域名和路径
		if strings.HasPrefix(route, "http://") || strings.HasPrefix(route, "https://") {
			// 已经是完整URL，直接使用
			fullURL = route
		} else {
			// 构建完整URL
			if !strings.HasPrefix(route, "/") {
				fullURL = "http://" + siteDomain + "/" + route
			} else {
				fullURL = "http://" + siteDomain + route
			}
		}

		urlStatusList = append(urlStatusList, gin.H{
			"url":       fullURL,
			"status":    status["status"],
			"cacheSize": status["cache_size"],
			"updatedAt": updatedAt,
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

// GetPreheatTaskStatus 获取预热任务状态
func (h *PreheatHandler) GetPreheatTaskStatus(c *gin.Context) {
	taskID := c.Query("task_id")
	siteName := c.Query("site_name")

	if taskID == "" || siteName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "Missing task_id or site_name parameter",
		})
		return
	}

	// 获取任务状态
	status, err := h.redisClient.GetPreheatTaskStatus(siteName, taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": fmt.Sprintf("Failed to get preheat task status: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    status,
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
