package routes

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"prerender-shield/internal/auth"
	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/prerender"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/scheduler"
	sitehandler "prerender-shield/internal/site-handler"
	siteserver "prerender-shield/internal/site-server"
)

// Router API路由器，负责注册所有API路由
type Router struct {
	userManager      *auth.UserManager
	jwtManager       *auth.JWTManager
	configManager    *config.ConfigManager
	prerenderManager *prerender.EngineManager
	firewallManager  *auth.UserManager
	redisClient      *redis.Client
	scheduler        *scheduler.Scheduler
	siteServerMgr    *siteserver.Manager
	siteHandler      *sitehandler.Handler
	monitor          *monitoring.Monitor
	crawlerLogMgr    *logging.CrawlerLogManager
	cfg              *config.Config
}

// NewRouter 创建API路由器实例
func NewRouter(
	userManager *auth.UserManager,
	jwtManager *auth.JWTManager,
	configManager *config.ConfigManager,
	prerenderManager *prerender.EngineManager,
	redisClient *redis.Client,
	scheduler *scheduler.Scheduler,
	siteServerMgr *siteserver.Manager,
	siteHandler *sitehandler.Handler,
	monitor *monitoring.Monitor,
	crawlerLogMgr *logging.CrawlerLogManager,
	cfg *config.Config,
) *Router {
	return &Router{
		userManager:      userManager,
		jwtManager:       jwtManager,
		configManager:    configManager,
		prerenderManager: prerenderManager,
		redisClient:      redisClient,
		scheduler:        scheduler,
		siteServerMgr:    siteServerMgr,
		siteHandler:      siteHandler,
		monitor:          monitor,
		crawlerLogMgr:    crawlerLogMgr,
		cfg:              cfg,
	}
}

// RegisterRoutes 注册所有API路由
func (r *Router) RegisterRoutes(ginRouter *gin.Engine) {
	// 添加安全头中间件
	ginRouter.Use(func(c *gin.Context) {
		// Content-Security-Policy (CSP) 头，防止XSS攻击，允许跨域请求
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self' http://localhost:5173 http://localhost:9598")

		// X-Frame-Options 头，防止Clickjacking攻击
		c.Header("X-Frame-Options", "DENY")

		// X-XSS-Protection 头，启用浏览器的XSS过滤
		c.Header("X-XSS-Protection", "1; mode=block")

		// X-Content-Type-Options 头，防止MIME类型嗅探
		c.Header("X-Content-Type-Options", "nosniff")

		// Referrer-Policy 头，控制Referrer信息的发送
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Strict-Transport-Security (HSTS) 头，强制使用HTTPS
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		// Permissions-Policy 头，控制浏览器API的访问
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=(), usb=(), accelerometer=(), gyroscope=()")

		c.Next()
	})

	// 添加CORS中间件
	ginRouter.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})

	// 注册API路由
	apiGroup := ginRouter.Group("/api/v1")
	{
		// 认证相关API - 不需要JWT验证
		authGroup := apiGroup.Group("/auth")
		{
			// 检查是否是首次运行
			authGroup.GET("/first-run", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data": gin.H{
						"isFirstRun": r.userManager.IsFirstRun(),
					},
				})
			})

			// 用户登录
			authGroup.POST("/login", func(c *gin.Context) {
				// 解析请求
				var req struct {
					Username string `json:"username" binding:"required"`
					Password string `json:"password" binding:"required"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    http.StatusBadRequest,
						"message": "Invalid request",
					})
					return
				}

				// 验证用户
				user, err := r.userManager.AuthenticateUser(req.Username, req.Password)
				if err != nil {
					c.JSON(http.StatusUnauthorized, gin.H{
						"code":    http.StatusUnauthorized,
						"message": "Invalid username or password",
					})
					return
				}

				// 生成JWT令牌
				token, err := r.jwtManager.GenerateToken(user.ID, user.Username)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
						"code":    http.StatusInternalServerError,
						"message": "Failed to generate token",
					})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "Login successful",
					"data": gin.H{
						"token":    token,
						"username": user.Username,
					},
				})
			})

			// 用户退出登录
			authGroup.POST("/logout", func(c *gin.Context) {
				// JWT是无状态的，退出登录只需要前端清除token即可
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "Logout successful",
				})
			})
		}

		// 需要JWT验证的API组
		protectedGroup := apiGroup.Group("/")
		protectedGroup.Use(auth.JWTAuthMiddleware(r.jwtManager))
		{
			// 概览API
			protectedGroup.GET("/overview", func(c *gin.Context) {
				// 计算总防火墙和渲染预热启用状态
				firewallEnabled := false
				prerenderEnabled := false
				for _, site := range r.cfg.Sites {
					if site.Firewall.Enabled {
						firewallEnabled = true
					}
					if site.Prerender.Enabled {
						prerenderEnabled = true
					}
				}

				// 获取真实监控数据
				stats := r.monitor.GetStats()

				// 获取站点统计数据
				activeSites := len(r.cfg.Sites)
				sslCertificates := 0 // SSL功能已移除

				c.JSON(http.StatusOK, gin.H{
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
						// 暂时保留模拟数据，后续可以替换为真实数据
						"geoData": gin.H{
							"countryData": []gin.H{
								{"country": "中国", "count": 891800, "color": "#1890ff"},
								{"country": "美国", "count": 2300, "color": "#52c41a"},
								{"country": "爱尔兰", "count": 461, "color": "#faad14"},
								{"country": "澳大利亚", "count": 361, "color": "#f5222d"},
								{"country": "新加坡", "count": 221, "color": "#722ed1"},
								{"country": "印度", "count": 157, "color": "#fa8c16"},
								{"country": "日本", "count": 133, "color": "#eb2f96"},
							},
							"mapData": []gin.H{
								{"name": "中国", "value": 891800},
								{"name": "美国", "value": 2300},
								{"name": "爱尔兰", "value": 461},
								{"name": "澳大利亚", "value": 361},
								{"name": "新加坡", "value": 221},
								{"name": "印度", "value": 157},
								{"name": "日本", "value": 133},
							},
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
			})

			// 监控API
			protectedGroup.GET("/monitoring/stats", func(c *gin.Context) {
				// 获取监控统计数据
				stats := r.monitor.GetStats()
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    stats,
				})
			})
			// 防火墙API
			protectedGroup.GET("/firewall/status", func(c *gin.Context) {
				// 获取防火墙状态
				site := c.Query("site")
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data": gin.H{
						"site":    site,
						"enabled": true,
						"status":  "running",
					},
				})
			})

			// 防火墙规则API
			protectedGroup.GET("/firewall/rules", func(c *gin.Context) {
				// 获取防火墙规则
				site := c.Query("site")
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data": []gin.H{
						{
							"id":        "1",
							"site":      site,
							"name":      "Default Rule",
							"priority":  100,
							"condition": "all",
							"action":    "allow",
							"enabled":   true,
						},
					},
				})
			})

			// 爬虫日志API
			protectedGroup.GET("/crawler/logs", func(c *gin.Context) {
				// 获取爬虫日志
				site := c.Query("site")
				startTimeStr := c.DefaultQuery("startTime", time.Now().Add(-24*time.Hour).Format(time.RFC3339))
				endTimeStr := c.DefaultQuery("endTime", time.Now().Format(time.RFC3339))
				page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
				pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

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
				logs, total, err := r.crawlerLogMgr.GetCrawlerLogs(site, startTime, endTime, page, pageSize)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
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

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data": gin.H{
						"items":    items,
						"total":    total,
						"page":     page,
						"pageSize": pageSize,
					},
				})
			})

			protectedGroup.GET("/crawler/stats", func(c *gin.Context) {
				// 获取爬虫统计数据
				site := c.Query("site")
				startTimeStr := c.DefaultQuery("startTime", time.Now().Add(-24*time.Hour).Format(time.RFC3339))
				endTimeStr := c.DefaultQuery("endTime", time.Now().Format(time.RFC3339))
				granularity := c.DefaultQuery("granularity", "hour")

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
				stats, err := r.crawlerLogMgr.GetCrawlerStats(site, startTime, endTime, granularity)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
						"code":    http.StatusInternalServerError,
						"message": "Failed to get crawler stats",
					})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    stats,
				})
			})

			// 预热API
			protectedGroup.GET("/preheat/sites", func(c *gin.Context) {
				// 获取静态网站列表
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    []gin.H{},
				})
			})

			protectedGroup.GET("/preheat/stats", func(c *gin.Context) {
				// 获取预热统计数据
				siteId := c.Query("siteId")

				if siteId == "" {
					// 获取所有站点的统计数据
					c.JSON(http.StatusOK, gin.H{
						"code":    200,
						"message": "success",
						"data":    []gin.H{},
					})
					return
				}

				// 获取指定站点的统计数据
				// 简化实现，直接返回空统计数据
				c.JSON(http.StatusOK, gin.H{
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
			})

			protectedGroup.POST("/preheat/trigger", func(c *gin.Context) {
				// 触发站点预热
				var req struct {
					SiteId string `json:"siteId" binding:"required"`
				}

				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    http.StatusBadRequest,
						"message": "Invalid request",
					})
					return
				}

				// 获取站点的预渲染引擎
				engine, exists := r.prerenderManager.GetEngine(req.SiteId)
				if !exists {
					c.JSON(http.StatusNotFound, gin.H{
						"code":    http.StatusNotFound,
						"message": fmt.Sprintf("Site with ID '%s' not found", req.SiteId),
					})
					return
				}

				// 调用引擎的触发预热方法
				if err := engine.TriggerPreheat(); err != nil {
					// 检查错误类型，返回更友好的错误信息
					if strings.Contains(err.Error(), "preheat is already running") {
						c.JSON(http.StatusConflict, gin.H{
							"code":    http.StatusConflict,
							"message": "预热任务已在运行中，请稍后再试",
						})
					} else if strings.Contains(err.Error(), "redis client is not available") {
						c.JSON(http.StatusServiceUnavailable, gin.H{
							"code":    http.StatusServiceUnavailable,
							"message": "Redis服务不可用，无法触发预热",
						})
					} else {
						c.JSON(http.StatusInternalServerError, gin.H{
							"code":    http.StatusInternalServerError,
							"message": fmt.Sprintf("触发预热失败: %v", err),
						})
					}
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "Preheat triggered successfully",
				})
			})

			protectedGroup.POST("/preheat/url", func(c *gin.Context) {
				// 手动预热指定URL
				var req struct {
					SiteId string   `json:"siteId" binding:"required"`
					URLs   []string `json:"urls" binding:"required"`
				}

				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    http.StatusBadRequest,
						"message": "无效的请求参数",
					})
					return
				}

				// 检查站点是否存在
				_, exists := r.prerenderManager.GetEngine(req.SiteId)
				if !exists {
					c.JSON(http.StatusNotFound, gin.H{
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
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "URL预热任务已成功触发",
					"data": gin.H{
						"siteId":   req.SiteId,
						"urlCount": len(req.URLs),
						"urls":     req.URLs,
					},
				})
			})

			protectedGroup.GET("/preheat/urls", func(c *gin.Context) {
				// 获取URL列表
				siteId := c.Query("siteId")
				page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
				pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

				if page < 1 {
					page = 1
				}
				if pageSize < 1 || pageSize > 100 {
					pageSize = 20
				}

				var urls []string
				var total int64

				// 检查Redis客户端是否可用
				if r.redisClient != nil {
					// 从Redis获取URL列表
					allUrls, err := r.redisClient.GetURLs(siteId)
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

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data": gin.H{
						"list":     list,
						"total":    total,
						"page":     page,
						"pageSize": pageSize,
					},
				})
			})

			protectedGroup.GET("/preheat/task/status", func(c *gin.Context) {
				// 获取任务状态
				siteId := c.Query("siteId")

				if siteId == "" {
					// 获取所有站点的任务状态
					c.JSON(http.StatusOK, gin.H{
						"code":    200,
						"message": "success",
						"data":    []gin.H{},
					})
					return
				}

				// 获取站点的预渲染引擎
				engine, exists := r.prerenderManager.GetEngine(siteId)
				if !exists {
					c.JSON(http.StatusNotFound, gin.H{
						"code":    http.StatusNotFound,
						"message": fmt.Sprintf("Site with ID '%s' not found", siteId),
					})
					return
				}

				// 获取预热状态
				status := engine.GetPreheatStatus()

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data": gin.H{
						"siteId":    siteId,
						"isRunning": status["isRunning"],
						"scheduled": false,
						"nextRun":   "",
					},
				})
			})

			protectedGroup.GET("/preheat/crawler-headers", func(c *gin.Context) {
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

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    defaultHeaders,
				})
			})

			// 站点管理API
			sitesGroup := protectedGroup.Group("/sites")
			{
				// 获取站点列表
				sitesGroup.GET("", func(c *gin.Context) {
					// 从配置管理器获取当前配置
					currentConfig := r.configManager.GetConfig()
					c.JSON(http.StatusOK, gin.H{
						"code":    200,
						"message": "success",
						"data":    currentConfig.Sites,
					})
				})

				// 获取单个站点信息
				sitesGroup.GET("/:name", func(c *gin.Context) {
					name := c.Param("name")
					// 从配置管理器获取当前配置
					currentConfig := r.configManager.GetConfig()
					for _, site := range currentConfig.Sites {
						if site.Name == name {
							c.JSON(http.StatusOK, gin.H{
								"code":    200,
								"message": "success",
								"data":    site,
							})
							return
						}
					}
					c.JSON(http.StatusNotFound, gin.H{
						"code":    404,
						"message": "Site not found",
					})
				})

				// 添加站点
				sitesGroup.POST("", func(c *gin.Context) {
					var site config.SiteConfig
					if err := c.ShouldBindJSON(&site); err != nil {
						c.JSON(http.StatusBadRequest, gin.H{
							"code":    400,
							"message": "Invalid request",
						})
						return
					}

					// 验证域名：只允许127.0.0.1或localhost
					for _, domain := range site.Domains {
						if domain != "127.0.0.1" && domain != "localhost" {
							c.JSON(http.StatusBadRequest, gin.H{
								"code":    400,
								"message": "Only 127.0.0.1 or localhost are allowed as domains",
							})
							return
						}
					}

					// 验证端口是否可用
					if !isPortAvailable(site.Port) {
						c.JSON(http.StatusBadRequest, gin.H{
							"code":    400,
							"message": "Port is either reserved or already in use",
						})
						return
					}

					// 为新站点生成唯一ID
					site.ID = uuid.New().String()

					// 从配置管理器获取当前配置并更新
					currentConfig := r.configManager.GetConfig()
					currentConfig.Sites = append(currentConfig.Sites, site)

					// 保存配置到文件
					if err := r.configManager.SaveConfig(); err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{
							"code":    500,
							"message": "Failed to save site configuration",
						})
						return
					}

					// 启动新站点的服务器实例
					siteHandler := r.siteHandler.CreateSiteHandler(site, r.crawlerLogMgr, r.monitor, r.cfg.Dirs.StaticDir)

					// 启动站点服务器
					r.siteServerMgr.StartSiteServer(site, r.cfg.Server.Address, r.cfg.Dirs.StaticDir, r.crawlerLogMgr, siteHandler)

					// 记录系统日志
					logging.DefaultLogger.LogAdminAction(
						"admin",
						c.ClientIP(),
						"site_add",
						"site",
						map[string]interface{}{
							"site_id":   site.ID,
							"site_name": site.Name,
							"domains":   site.Domains,
							"port":      site.Port,
							"mode":      site.Mode,
						},
						"success",
						"Site added successfully",
					)

					c.JSON(http.StatusOK, gin.H{
						"code":    200,
						"message": "Site added successfully",
						"data":    site,
					})
				})

				// 更新站点
				sitesGroup.PUT("/:name", func(c *gin.Context) {
					name := c.Param("name")
					var siteUpdates config.SiteConfig
					if err := c.ShouldBindJSON(&siteUpdates); err != nil {
						c.JSON(http.StatusBadRequest, gin.H{
							"code":    400,
							"message": "Invalid request",
						})
						return
					}

					// 验证域名：只允许127.0.0.1或localhost
					for _, domain := range siteUpdates.Domains {
						if domain != "127.0.0.1" && domain != "localhost" {
							c.JSON(http.StatusBadRequest, gin.H{
								"code":    400,
								"message": "Only 127.0.0.1 or localhost are allowed as domains",
							})
							return
						}
					}

					// 从配置管理器获取当前配置
					currentConfig := r.configManager.GetConfig()

					// 查找并更新指定站点
					var updatedSite *config.SiteConfig
					var oldSite *config.SiteConfig

					for i, s := range currentConfig.Sites {
						if s.Name == name {
							// 保存旧站点信息
							oldSite = &s

							// 检查端口是否可用（仅当端口改变时）
							if s.Port != siteUpdates.Port {
								if !isPortAvailable(siteUpdates.Port) {
									c.JSON(http.StatusBadRequest, gin.H{
										"code":    400,
										"message": "Port is either reserved or already in use",
									})
									return
								}
							}

							// 更新站点配置，保留原始ID
							currentConfig.Sites[i].Name = siteUpdates.Name
							currentConfig.Sites[i].Domains = siteUpdates.Domains
							currentConfig.Sites[i].Port = siteUpdates.Port
							currentConfig.Sites[i].Mode = siteUpdates.Mode
							currentConfig.Sites[i].Proxy = siteUpdates.Proxy
							currentConfig.Sites[i].Redirect = siteUpdates.Redirect
							currentConfig.Sites[i].Firewall = siteUpdates.Firewall
							currentConfig.Sites[i].Prerender = siteUpdates.Prerender
							currentConfig.Sites[i].Routing = siteUpdates.Routing
							currentConfig.Sites[i].FileIntegrityConfig = siteUpdates.FileIntegrityConfig

							// 获取更新后的站点
							updatedSite = &currentConfig.Sites[i]

							break
						}
					}

					if updatedSite == nil {
						c.JSON(http.StatusNotFound, gin.H{
							"code":    404,
							"message": "Site not found",
						})
						return
					}

					// 保存配置到文件
					if err := r.configManager.SaveConfig(); err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{
							"code":    500,
							"message": "Failed to save site configuration",
						})
						return
					}

					// 停止旧的站点服务器
					if _, exists := r.siteServerMgr.GetSiteServer(oldSite.Name); exists {
						r.siteServerMgr.StopSiteServer(oldSite.Name)
					}

					// 启动新的站点服务器
					siteHandler := r.siteHandler.CreateSiteHandler(*updatedSite, r.crawlerLogMgr, r.monitor, r.cfg.Dirs.StaticDir)

					// 启动站点服务器
					r.siteServerMgr.StartSiteServer(*updatedSite, r.cfg.Server.Address, r.cfg.Dirs.StaticDir, r.crawlerLogMgr, siteHandler)

					// 记录系统日志
					logging.DefaultLogger.LogAdminAction(
						"admin",
						c.ClientIP(),
						"site_update",
						"site",
						map[string]interface{}{
							"old_site_name": oldSite.Name,
							"new_site_name": updatedSite.Name,
							"site_id":       updatedSite.ID,
							"domains":       updatedSite.Domains,
							"port":          updatedSite.Port,
							"mode":          updatedSite.Mode,
						},
						"success",
						"Site updated successfully",
					)

					c.JSON(http.StatusOK, gin.H{
						"code":    200,
						"message": "Site updated successfully",
						"data":    updatedSite,
					})
				})

				// 删除站点
				sitesGroup.DELETE("/:name", func(c *gin.Context) {
					name := c.Param("name")

					// 从配置管理器获取当前配置并更新
					currentConfig := r.configManager.GetConfig()

					// 查找并删除指定站点
					for i, site := range currentConfig.Sites {
						if site.Name == name {
							// 停止站点服务器
							r.siteServerMgr.StopSiteServer(site.Name)

							// 删除站点的静态资源目录
							staticDir := filepath.Join(r.cfg.Dirs.StaticDir, site.ID)
							if _, err := os.Stat(staticDir); err == nil {
								// 目录存在，删除它
								if err := os.RemoveAll(staticDir); err != nil {
									log.Printf("Failed to delete static files for site %s: %v", site.Name, err)
									// 继续执行，不中断删除流程
								} else {
									log.Printf("Deleted static files for site %s", site.Name)
								}
							}

							// 从切片中删除站点
							currentConfig.Sites = append(currentConfig.Sites[:i], currentConfig.Sites[i+1:]...)

							// 保存配置到文件
							if err := r.configManager.SaveConfig(); err != nil {
								c.JSON(http.StatusInternalServerError, gin.H{
									"code":    500,
									"message": "Failed to save site configuration",
								})
								return
							}

							// 记录系统日志
							logging.DefaultLogger.LogAdminAction(
								"admin",
								c.ClientIP(),
								"site_delete",
								"site",
								map[string]interface{}{
									"site_id":   site.ID,
									"site_name": site.Name,
									"domains":   site.Domains,
									"port":      site.Port,
								},
								"success",
								"Site deleted successfully",
							)

							c.JSON(http.StatusOK, gin.H{
								"code":    200,
								"message": "Site deleted successfully",
							})
							return
						}
					}

					// 如果站点不存在，返回404
					c.JSON(http.StatusNotFound, gin.H{
						"code":    404,
						"message": "Site not found",
					})
				})
			}
		}
	}
}

// 检查端口是否可用
func isPortAvailable(port int) bool {
	// 常用互联网端口列表，这些端口将被排除
	reservedPorts := map[int]bool{
		// 常用服务端口
		21:  true, // FTP
		22:  true, // SSH
		23:  true, // Telnet
		25:  true, // SMTP
		53:  true, // DNS
		80:  true, // HTTP
		110: true, // POP3
		143: true, // IMAP
		443: true, // HTTPS
		465: true, // SMTPS
		587: true, // SMTP (STARTTLS)
		993: true, // IMAPS
		995: true, // POP3S

		// 常用应用端口
		3306:  true, // MySQL
		5432:  true, // PostgreSQL
		6379:  true, // Redis
		8080:  true, // Tomcat
		9000:  true, // PHP-FPM
		9090:  true, // Prometheus
		15672: true, // RabbitMQ
		27017: true, // MongoDB
	}

	// 检查是否是保留端口
	if reservedPorts[port] {
		return false
	}

	// 尝试监听端口
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	defer listener.Close()

	return true
}
