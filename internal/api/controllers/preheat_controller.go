package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/config"
	"prerender-shield/internal/prerender"
	"prerender-shield/internal/redis"
)

// PreheatController 预热控制器
type PreheatController struct {
	prerenderManager *prerender.EngineManager
	redisClient      *redis.Client
	cfg              *config.Config
}

// NewPreheatController 创建预热控制器实例
func NewPreheatController(
	prerenderManager *prerender.EngineManager,
	redisClient *redis.Client,
	cfg *config.Config,
) *PreheatController {
	return &PreheatController{
		prerenderManager: prerenderManager,
		redisClient:      redisClient,
		cfg:              cfg,
	}
}

// GetPreheatSites 获取静态网站列表
func (c *PreheatController) GetPreheatSites(ctx *gin.Context) {
	// 获取配置中的所有站点
	var sites []gin.H
	for _, site := range c.cfg.Sites {
		// 为每个站点构建完整的域名
		var domain string
		if len(site.Domains) > 0 {
			domain = site.Domains[0]
		} else {
			domain = "localhost"
		}

		// 构建站点信息
		sites = append(sites, gin.H{
			"id":      site.ID,
			"name":    site.Name,
			"domain":  domain,
			"Domains": site.Domains,
			"enabled": true,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    sites,
	})
}

// GetPreheatStats 获取预热统计数据
func (c *PreheatController) GetPreheatStats(ctx *gin.Context) {
	// 获取预热统计数据
	siteId := ctx.Query("siteId")

	if siteId == "" {
		// 获取所有站点的统计数据
		var allStats []gin.H

		for _, site := range c.cfg.Sites {
			// 初始化统计数据
			urlCount := int64(0)
			cacheCount := int64(0)
			totalCacheSize := int64(0)
			browserPoolSize := int64(0)

			// 从引擎获取浏览器池大小，使用站点ID作为siteName
			engine, exists := c.prerenderManager.GetEngine(site.ID)
			if exists {
				browserPoolSize = int64(engine.GetConfig().PoolSize)
			}

			// 检查Redis客户端是否可用
			if c.redisClient != nil {
				// 从Redis获取URL总数，使用站点ID作为siteName
				urlCount, _ = c.redisClient.GetURLCount(site.ID)

				// 从Redis获取缓存数
				cacheCount, _ = c.redisClient.GetCacheCount(site.ID)

				// 直接计算总缓存大小
				totalCacheSize = cacheCount * 1024 * 1024 // 假设平均每个缓存1MB
			}

			// 构建站点统计信息
			allStats = append(allStats, gin.H{
				"siteId":          site.ID,
				"siteName":        site.Name,
				"urlCount":        urlCount,
				"cacheCount":      cacheCount,
				"totalCacheSize":  totalCacheSize,
				"browserPoolSize": browserPoolSize,
			})
		}

		ctx.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "success",
			"data":    allStats,
		})
		return
	}

	// 获取指定站点的统计数据
	// 首先根据siteId查找对应的站点配置
	var siteConfig *config.SiteConfig
	for _, site := range c.cfg.Sites {
		if site.ID == siteId {
			siteConfig = &site
			break
		}
	}

	if siteConfig == nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": fmt.Sprintf("Site with ID '%s' not found", siteId),
		})
		return
	}

	// 初始化统计数据
	urlCount := int64(0)
	cacheCount := int64(0)
	totalCacheSize := int64(0)
	browserPoolSize := int64(0)

	// 从引擎获取浏览器池大小，使用站点ID作为siteName
	engine, exists := c.prerenderManager.GetEngine(siteId)
	if exists {
		browserPoolSize = int64(engine.GetConfig().PoolSize)
	}

	// 检查Redis客户端是否可用
	if c.redisClient != nil {
		// 从Redis获取URL总数，使用站点ID作为siteName
		urlCount, _ = c.redisClient.GetURLCount(siteId)

		// 从Redis获取缓存数
		cacheCount, _ = c.redisClient.GetCacheCount(siteId)

		// 直接计算总缓存大小
		totalCacheSize = cacheCount * 1024 * 1024 // 假设平均每个缓存1MB
	}

	// 返回实际统计数据
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"siteId":          siteId,
			"urlCount":        urlCount,
			"cacheCount":      cacheCount,
			"totalCacheSize":  totalCacheSize,
			"browserPoolSize": browserPoolSize,
		},
	})
}

// TriggerPreheat 触发站点预热
func (c *PreheatController) TriggerPreheat(ctx *gin.Context) {
	// 触发站点预热
	var req struct {
		SiteId string `json:"siteId" binding:"required"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "Invalid request",
		})
		return
	}

	// 获取站点配置
	var siteConfig *config.SiteConfig
	for _, site := range c.cfg.Sites {
		if site.ID == req.SiteId {
			siteConfig = &site
			break
		}
	}

	if siteConfig == nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": fmt.Sprintf("Site with ID '%s' not found", req.SiteId),
		})
		return
	}

	// 获取站点的预渲染引擎
	engine, exists := c.prerenderManager.GetEngine(req.SiteId)
	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": fmt.Sprintf("Site with ID '%s' not found", req.SiteId),
		})
		return
	}

	// 构建正确的baseURL和Domain
	var baseURL, domain string
	switch siteConfig.Mode {
	case "proxy":
		// 代理模式下，使用代理的目标URL
		baseURL = siteConfig.Proxy.TargetURL
		domain = siteConfig.Proxy.TargetURL
	case "redirect":
		// 重定向模式下，使用重定向的目标URL
		baseURL = siteConfig.Redirect.TargetURL
		domain = siteConfig.Redirect.TargetURL
	default:
		// 静态模式下，使用站点的域名和端口
		if len(siteConfig.Domains) > 0 {
			domain = fmt.Sprintf("%s:%d", siteConfig.Domains[0], siteConfig.Port)
			baseURL = fmt.Sprintf("http://%s", domain)
		} else {
			// 默认使用localhost
			domain = fmt.Sprintf("localhost:%d", siteConfig.Port)
			baseURL = fmt.Sprintf("http://%s", domain)
		}
	}

	// 调用引擎的触发预热方法，传递正确的baseURL和Domain
	_, err := engine.TriggerPreheatWithURL(baseURL, domain)
	if err != nil {
		// 检查错误类型，返回更友好的错误信息
		if strings.Contains(err.Error(), "preheat is already running") {
			ctx.JSON(http.StatusConflict, gin.H{
				"code":    http.StatusConflict,
				"message": "预热任务已在运行中，请稍后再试",
			})
		} else if strings.Contains(err.Error(), "redis client is not available") {
			ctx.JSON(http.StatusServiceUnavailable, gin.H{
				"code":    http.StatusServiceUnavailable,
				"message": "Redis服务不可用，无法触发预热",
			})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"code":    http.StatusInternalServerError,
				"message": fmt.Sprintf("触发预热失败: %v", err),
			})
		}
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Preheat triggered successfully",
	})
}

// GetPreheatUrls 获取URL列表
func (c *PreheatController) GetPreheatUrls(ctx *gin.Context) {
	// 获取URL列表
	siteId := ctx.Query("siteId")
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 获取站点配置
	var siteConfig *config.SiteConfig
	for _, site := range c.cfg.Sites {
		if site.ID == siteId {
			siteConfig = &site
			break
		}
	}

	if siteConfig == nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": fmt.Sprintf("Site with ID '%s' not found", siteId),
		})
		return
	}

	var urls []string
	var total int64

	// 检查Redis客户端是否可用
	if c.redisClient != nil {
		// 从Redis获取URL列表，使用站点ID作为siteName
		allUrls, err := c.redisClient.GetURLs(siteId)
		if err == nil {
			urls = allUrls
			total = int64(len(allUrls))
		}
	}

	// 分页处理
	start := (page - 1) * pageSize
	end := start + pageSize
	if end > len(urls) {
		end = len(urls)
	}

	var pageUrls []string
	if start < len(urls) {
		pageUrls = urls[start:end]
	}

	// 构建完整的站点域名（使用站点配置的域名，不是推送配置的域名）
	var siteDomain string
	var baseURL string
	if siteConfig != nil && len(siteConfig.Domains) > 0 {
		// 使用站点配置的第一个域名
		siteDomain = siteConfig.Domains[0]
		// 构建基础URL，包含域名和端口
		if siteConfig.Port != 80 {
			baseURL = fmt.Sprintf("http://%s:%d", siteDomain, siteConfig.Port)
		} else {
			baseURL = fmt.Sprintf("http://%s", siteDomain)
		}
	} else {
		// 默认使用站点ID作为域名
		siteDomain = siteId
		baseURL = fmt.Sprintf("http://%s", siteDomain)
	}

	// 转换为前端需要的格式
	var list []gin.H
	for _, route := range pageUrls {
		// 检查路由是否已经是完整URL
		var fullURL string
		if strings.HasPrefix(route, "http://") || strings.HasPrefix(route, "https://") {
			// 如果已经是完整URL，直接使用
			fullURL = route
		} else {
			// 确保路由以/开头
			normalizedRoute := route
			if !strings.HasPrefix(normalizedRoute, "/") {
				normalizedRoute = "/" + normalizedRoute
			}

			// 构建完整URL，确保包含完整的域名和路径
			fullURL = baseURL + normalizedRoute
		}

		// 获取URL的预热状态
		var updatedAt string
		if c.redisClient != nil {
			// 使用站点ID作为siteName，路由作为URL
			urlStatus, err := c.redisClient.GetURLPreheatStatus(siteId, route)
			if err == nil {
				updatedAt = urlStatus["updated_at"]
			}
		}

		// 保持原始时间戳格式，不进行格式化
		// 前端会将时间戳转换为可读格式
		if updatedAt == "" {
			// 为没有更新时间的URL设置默认值
			updatedAt = "-"
		}

		// 将完整URL添加到列表中，移除status和cacheSize字段
		list = append(list, gin.H{
			"url":       fullURL,
			"updatedAt": updatedAt,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"list":     list,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

// GetPreheatTaskStatus 获取任务状态
func (c *PreheatController) GetPreheatTaskStatus(ctx *gin.Context) {
	// 获取任务状态
	siteId := ctx.Query("siteId")

	if siteId == "" {
		// 获取所有站点的任务状态
		ctx.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "success",
			"data":    []gin.H{},
		})
		return
	}

	// 获取站点的预渲染引擎
	engine, exists := c.prerenderManager.GetEngine(siteId)
	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": fmt.Sprintf("Site with ID '%s' not found", siteId),
		})
		return
	}

	// 获取预热状态
	status := engine.GetPreheatStatus()

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"siteId":    siteId,
			"isRunning": status["isRunning"],
			"scheduled": false,
			"nextRun":   "",
		},
	})
}

// GetCrawlerHeaders 获取爬虫协议头列表
func (c *PreheatController) GetCrawlerHeaders(ctx *gin.Context) {
	// 获取爬虫协议头列表
	defaultHeaders := []string{
		"Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)",
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"Mozilla/5.0 (compatible; Sogou spider/4.0; +http://www.sogou.com/docs/help/webmasters.htm#07)",
		"Mozilla/5.0 (compatible; Bytespider; https://zhanzhang.toutiao.com/)",
		"Mozilla/5.0 (compatible; HaosouSpider; http://www.haosou.com/help/help_3_2.html)",
		"Mozilla/5.0 (compatible; YisouSpider/1.0; http://www.yisou.com/help/webmaster/spider_guide.html)",
		"Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    defaultHeaders,
	})
}
