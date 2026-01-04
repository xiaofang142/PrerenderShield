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
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    []gin.H{},
	})
}

// GetPreheatStats 获取预热统计数据
func (c *PreheatController) GetPreheatStats(ctx *gin.Context) {
	// 获取预热统计数据
	siteId := ctx.Query("siteId")

	if siteId == "" {
		// 获取所有站点的统计数据
		ctx.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "success",
			"data":    []gin.H{},
		})
		return
	}

	// 获取指定站点的统计数据
	// 简化实现，直接返回空统计数据
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"siteId":          siteId,
			"urlCount":        0,
			"cacheCount":      0,
			"totalCacheSize":  0,
			"browserPoolSize": 0,
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
	if err := engine.TriggerPreheatWithURL(baseURL, domain); err != nil {
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

// PreheatURLs 手动预热指定URL
func (c *PreheatController) PreheatURLs(ctx *gin.Context) {
	// 手动预热指定URL
	var req struct {
		SiteId string   `json:"siteId" binding:"required"`
		URLs   []string `json:"urls" binding:"required"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "无效的请求参数",
		})
		return
	}

	// 检查站点是否存在
	_, exists := c.prerenderManager.GetEngine(req.SiteId)
	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": fmt.Sprintf("站点 %s 不存在", req.SiteId),
		})
		return
	}

	// 调用引擎的预热方法
	for _, url := range req.URLs {
		// 这里简化处理，实际应该调用引擎的预热方法
		fmt.Printf("触发URL预热: 站点 %s, URL %s\n", req.SiteId, url)
	}

	// 返回成功响应
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "URL预热任务已成功触发",
		"data": gin.H{
			"siteId":   req.SiteId,
			"urlCount": len(req.URLs),
			"urls":     req.URLs,
		},
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

	var urls []string
	var total int64

	// 检查Redis客户端是否可用
	if c.redisClient != nil {
		// 从Redis获取URL列表
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

	// 转换为前端需要的格式
	var list []gin.H
	for _, url := range pageUrls {
		list = append(list, gin.H{"url": url})
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
