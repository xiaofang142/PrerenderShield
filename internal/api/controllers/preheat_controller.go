package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/cache"
	"prerender-shield/internal/config"
	"prerender-shield/internal/prerender"
	"prerender-shield/internal/prerender/renderkey"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/scheduler"
)

// PreheatController 预热控制器
type PreheatController struct {
	prerenderManager *prerender.EngineManager
	redisClient      *redis.Client
	scheduler        *scheduler.Scheduler
	cfg              configRef
}

// NewPreheatController 创建预热控制器实例
func NewPreheatController(
	prerenderManager *prerender.EngineManager,
	redisClient *redis.Client,
	scheduler *scheduler.Scheduler,
	cfg *config.Config,
) *PreheatController {
	return &PreheatController{
		prerenderManager: prerenderManager,
		redisClient:      redisClient,
		scheduler:        scheduler,
		cfg:              configRef{snapshot: cfg},
	}
}

// getCacheTotalSize 采样估算缓存总大小
func (c *PreheatController) getCacheTotalSize(cacheCount int64) int64 {
	if c.redisClient == nil || cacheCount == 0 {
		return 0
	}
	rawClient := c.redisClient.GetRawClient()
	ctx := rawClient.Context()

	sampleSize := 50
	var totalSampleSize int64
	var sampled int

	var cursor uint64
	for sampled < sampleSize {
		keys, nextCursor, err := rawClient.Scan(ctx, cursor, "cache:*", int64(sampleSize-sampled)).Result()
		if err != nil {
			break
		}
		cursor = nextCursor
		for _, key := range keys {
			size, err := rawClient.MemoryUsage(ctx, key).Result()
			if err == nil {
				totalSampleSize += size
				sampled++
			}
			if sampled >= sampleSize {
				break
			}
		}
		if cursor == 0 {
			break
		}
	}

	if sampled == 0 {
		return cacheCount * 1024 * 1024
	}

	avgSize := totalSampleSize / int64(sampled)
	return avgSize * cacheCount
}

// collectSiteStats 收集单个站点的预热统计数据
// cacheCount/totalCacheSize 为全局量，由调用方预计算一次传入，避免 N 站点循环重复查询 Redis
func (c *PreheatController) collectSiteStats(site config.SiteConfig, cacheCount, totalCacheSize int64) gin.H {
	urlCount := int64(0)
	browserPoolSize := int64(0)

	if c.redisClient != nil {
		urlCount, _ = c.redisClient.GetURLCount(site.ID)
	}

	if engine, exists := c.prerenderManager.GetEngine(site.ID); exists {
		browserPoolSize = int64(engine.GetPoolSize())
	}

	// 站点渲染状态字段（/prerender 页状态卡消费；缺失导致页面 status.preheat.enabled 读取 undefined 崩溃）
	return gin.H{
		"siteId":          site.ID,
		"siteName":        site.Name,
		"urlCount":        urlCount,
		"cacheCount":      cacheCount,
		"totalCacheSize":  totalCacheSize,
		"browserPoolSize": browserPoolSize,
		"enabled":         site.Prerender.Enabled,
		"poolSize":        site.Prerender.PoolSize,
		"timeout":         site.Prerender.Timeout,
		"cacheTTL":        site.Prerender.CacheTTL,
		"preheat": gin.H{
			"enabled":    site.Prerender.Preheat.Enabled,
			"sitemapURL": site.Prerender.Preheat.SitemapURL,
			"schedule":   site.Prerender.Preheat.Schedule,
		},
	}
}

// GetPreheatSites 获取静态网站列表
func (c *PreheatController) GetPreheatSites(ctx *gin.Context) {
	// 检查必要的依赖项是否可用
	if c.cfg.current() == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "配置信息不可用",
		})
		return
	}

	// 检查站点列表是否可用（空列表局部兜底，不写回共享配置对象避免竞争）
	sitesList := c.cfg.current().Sites
	if sitesList == nil {
		sitesList = []config.SiteConfig{}
	}

	// 获取配置中的所有站点
	var sites []gin.H
	for _, site := range sitesList {
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

	// 检查必要的依赖项是否可用
	if c.cfg.current() == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "配置信息不可用",
		})
		return
	}

	if c.prerenderManager == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "渲染引擎管理器不可用",
		})
		return
	}

	// 检查站点列表是否可用（空列表局部兜底，不写回共享配置对象避免竞争）
	sitesList := c.cfg.current().Sites
	if sitesList == nil {
		sitesList = []config.SiteConfig{}
	}

	if siteId == "" {
		// 缓存计数为全局量，仅计算一次
		globalCacheCount, globalCacheSize := int64(0), int64(0)
		if c.redisClient != nil {
			globalCacheCount, _ = c.redisClient.GetCacheCount()
			globalCacheSize = c.getCacheTotalSize(globalCacheCount)
		}

		// 获取所有站点的统计数据
		allStats := make([]gin.H, 0, len(sitesList))
		for _, site := range sitesList {
			allStats = append(allStats, c.collectSiteStats(site, globalCacheCount, globalCacheSize))
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
	for _, site := range c.cfg.current().Sites {
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

	// 返回实际统计数据
	singleCacheCount, singleCacheSize := int64(0), int64(0)
	if c.redisClient != nil {
		singleCacheCount, _ = c.redisClient.GetCacheCount()
		singleCacheSize = c.getCacheTotalSize(singleCacheCount)
	}
	stats := c.collectSiteStats(*siteConfig, singleCacheCount, singleCacheSize)
	delete(stats, "siteName") // 保持单站点响应结构与历史版本一致

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    stats,
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
	for _, site := range c.cfg.current().Sites {
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
	// 注：EngineManager.GetEngine 对未注册站点会懒创建引擎并恒返回 exists=true，
	// 该错误分支不可达（引擎级失败由后续 CreatePreheatTask 报错承担）
	engine, _ := c.prerenderManager.GetEngine(req.SiteId)

	// 从Redis获取站点的URL列表
	urls, err := c.redisClient.GetURLs(req.SiteId)
	if err != nil || len(urls) == 0 {
		ctx.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "No URLs to preheat",
		})
		return
	}

	// 注入预热通道 TTL 配置（分级规则首中 > 站点 CacheTTL > 引擎默认）
	engine.SetPreheatTTLConfig(siteConfig.Prerender.CacheTTL, siteConfig.Prerender.TTLRules)

	// 调用引擎的创建预热任务方法
	taskID, err := engine.CreatePreheatTask(req.SiteId, urls)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": fmt.Sprintf("触发预热失败: %v", err),
		})
		return
	}

	// 存储当前预热任务ID
	c.redisClient.Set(fmt.Sprintf("site:%s:preheat:current_task", req.SiteId), taskID, 24*time.Hour)

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
	for _, site := range c.cfg.current().Sites {
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
			status, err := c.redisClient.GetURLPreheatStatus(siteId, route)
			if err == nil && status != "" {
				updatedAt = time.Now().Format(time.RFC3339)
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

	var isRunning bool
	var err error

	// 从 Redis 获取预热运行状态
	if c.redisClient != nil {
		isRunning, err = c.redisClient.IsPreheatRunning(siteId)
		if err != nil {
			isRunning = false
		}
	} else {
		isRunning = false
	}

	// 从调度器获取实际的调度状态和下次执行时间
	var scheduled bool
	var nextRun string
	if c.scheduler != nil {
		scheduled, nextRun = c.scheduler.GetTaskStatus(siteId)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"siteId":    siteId,
			"isRunning": isRunning,
			"scheduled": scheduled,
			"nextRun":   nextRun,
		},
	})
}

// GetCrawlerHeaders 获取爬虫协议头列表
func (c *PreheatController) GetCrawlerHeaders(ctx *gin.Context) {
	var headers []string

	// 从配置中读取爬虫头列表
	if c.cfg.current() != nil && len(c.cfg.current().Sites) > 0 {
		for _, site := range c.cfg.current().Sites {
			if len(site.Prerender.CrawlerHeaders) > 0 {
				headers = site.Prerender.CrawlerHeaders
				break
			}
		}
	}

	// 如果配置中没有，使用默认列表
	if len(headers) == 0 {
		headers = []string{
			"Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)",
			"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			"Mozilla/5.0 (compatible; Sogou spider/4.0; +http://www.sogou.com/docs/help/webmasters.htm#07)",
			"Mozilla/5.0 (compatible; Bytespider; https://zhanzhang.toutiao.com/)",
			"Mozilla/5.0 (compatible; HaosouSpider; http://www.haosou.com/help/help_3_2.html)",
			"Mozilla/5.0 (compatible; YisouSpider/1.0; http://www.yisou.com/help/webmaster/spider_guide.html)",
			"Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    headers,
	})
}

// ClearCache 清除站点缓存
func (c *PreheatController) ClearCache(ctx *gin.Context) {
	// 清除站点缓存
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
	for _, site := range c.cfg.current().Sites {
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

	// 检查Redis客户端是否可用
	if c.redisClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    http.StatusServiceUnavailable,
			"message": "Redis服务不可用，无法清除缓存",
		})
		return
	}

	// G3-1 修复：站点级精确清理（cache:<siteId>:*），不再误删全部站点缓存
	keys, err := c.redisClient.Keys(fmt.Sprintf("cache:%s:*", req.SiteId))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": fmt.Sprintf("扫描缓存键失败: %v", err),
		})
		return
	}

	if len(keys) > 0 {
		if err := c.redisClient.DelMultiple(keys); err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"code":    http.StatusInternalServerError,
				"message": fmt.Sprintf("清除缓存失败: %v", err),
			})
			return
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": fmt.Sprintf("站点 %s 缓存清除成功", req.SiteId),
		"data": gin.H{
			"clearedCount": len(keys),
		},
	})
}

// findEngine 按站点 ID 查找配置与引擎（缓存管理端点共用）
func (c *PreheatController) findEngine(siteID string) (*config.SiteConfig, prerender.Engine, bool) {
	var siteConfig *config.SiteConfig
	for _, site := range c.cfg.current().Sites {
		if site.ID == siteID {
			siteConfig = &site
			break
		}
	}
	if siteConfig == nil {
		return nil, nil, false
	}
	if c.prerenderManager == nil {
		return siteConfig, nil, false
	}
	// 注：GetEngine 懒创建并恒返回 exists=true，无需检查第二个返回值
	engine, _ := c.prerenderManager.GetEngine(siteID)
	return siteConfig, engine, true
}

// InvalidateCache 单 URL 缓存失效：POST /preheat/invalidate {siteId,url}
func (c *PreheatController) InvalidateCache(ctx *gin.Context) {
	var req struct {
		SiteId string `json:"siteId" binding:"required"`
		URL    string `json:"url" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "Invalid request",
		})
		return
	}

	siteConfig, engine, ok := c.findEngine(req.SiteId)
	if siteConfig == nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": fmt.Sprintf("Site with ID '%s' not found", req.SiteId),
		})
		return
	}
	if !ok {
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": fmt.Sprintf("Site with ID '%s' engine not found", req.SiteId),
		})
		return
	}

	// 走 renderkey 归一化，保证与缓存写入键一致
	if err := engine.InvalidatePage(req.SiteId, req.URL); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": fmt.Sprintf("失效缓存失败: %v", err),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"siteId": req.SiteId,
			"url":    req.URL,
		},
	})
}

// RecacheURL 单 URL 强制重渲并替换缓存：POST /preheat/recache {siteId,url}
// 同步返回结果（单 URL 渲染秒级，无需异步任务轮询）。
func (c *PreheatController) RecacheURL(ctx *gin.Context) {
	var req struct {
		SiteId string `json:"siteId" binding:"required"`
		URL    string `json:"url" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "Invalid request",
		})
		return
	}

	siteConfig, engine, ok := c.findEngine(req.SiteId)
	if siteConfig == nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": fmt.Sprintf("Site with ID '%s' not found", req.SiteId),
		})
		return
	}
	if !ok {
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": fmt.Sprintf("Site with ID '%s' engine not found", req.SiteId),
		})
		return
	}

	// 先失效再重渲，保证替换语义
	if err := engine.InvalidatePage(req.SiteId, req.URL); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": fmt.Sprintf("失效旧缓存失败: %v", err),
		})
		return
	}

	// entries 列表回传的是归一化形态（host:port/path，无 scheme），
	// 渲染需要可导航的绝对 URL，按形态补全
	renderURL := renderkey.ToAbsolute(req.URL, "http")

	opts := prerender.RenderOptions{}
	if siteConfig.Prerender.Timeout > 0 {
		opts.Timeout = time.Duration(siteConfig.Prerender.Timeout) * time.Second
	}
	opts.CacheTTL = siteConfig.EffectiveCacheTTL(renderURL)
	opts.MaxConcurrency = siteConfig.Prerender.MaxConcurrency

	res, err := engine.RenderAndCache(prerender.RenderRequest{
		SiteID: req.SiteId,
		URL:    renderURL,
		Opts:   opts,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": fmt.Sprintf("重渲失败: %v", err),
			"data": gin.H{
				"url":   req.URL,
				"error": res.Error,
			},
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"url":        req.URL,
			"status":     res.Status,
			"renderTime": res.RenderTime,
			"thin":       res.Thin,
		},
	})
}

// ListCacheEntries 列出站点缓存条目：GET /preheat/entries?siteId=&limit=
func (c *PreheatController) ListCacheEntries(ctx *gin.Context) {
	siteId := ctx.Query("siteId")
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "200"))
	if limit <= 0 || limit > 1000 {
		limit = 200
	}

	if _, _, ok := c.findEngine(siteId); !ok {
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": fmt.Sprintf("Site with ID '%s' not found", siteId),
		})
		return
	}

	engine, _ := c.prerenderManager.GetEngine(siteId)
	entries, err := engine.ListCacheEntries(siteId, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": fmt.Sprintf("列出缓存条目失败: %v", err),
		})
		return
	}
	if entries == nil {
		entries = []cache.CacheEntrySummary{}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"list":  entries,
			"total": len(entries),
		},
	})
}

// DeleteCacheEntry 删除单条缓存条目：DELETE /preheat/entries?siteId=&url=
func (c *PreheatController) DeleteCacheEntry(ctx *gin.Context) {
	siteId := ctx.Query("siteId")
	urlParam := ctx.Query("url")
	if siteId == "" || urlParam == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "siteId and url are required",
		})
		return
	}

	if _, engine, ok := c.findEngine(siteId); !ok || engine == nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": fmt.Sprintf("Site with ID '%s' not found", siteId),
		})
		return
	}

	engine, _ := c.prerenderManager.GetEngine(siteId)
	if err := engine.InvalidatePage(siteId, urlParam); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": fmt.Sprintf("删除缓存条目失败: %v", err),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
	})
}
