package controllers

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/redis"
	sitehandler "prerender-shield/internal/site-handler"
	siteserver "prerender-shield/internal/site-server"
)

// SitesController 站点管理控制器
type SitesController struct {
	configManager *config.ConfigManager
	siteServerMgr *siteserver.Manager
	siteHandler   *sitehandler.Handler
	redisClient   *redis.Client
	monitor       *monitoring.Monitor
	crawlerLogMgr *logging.CrawlerLogManager
	visitLogMgr   *logging.VisitLogManager
	cfg           *config.Config
}

// NewSitesController 创建站点管理控制器实例
func NewSitesController(
	configManager *config.ConfigManager,
	siteServerMgr *siteserver.Manager,
	siteHandler *sitehandler.Handler,
	redisClient *redis.Client,
	monitor *monitoring.Monitor,
	crawlerLogMgr *logging.CrawlerLogManager,
	visitLogMgr *logging.VisitLogManager,
	cfg *config.Config,
) *SitesController {
	return &SitesController{
		configManager: configManager,
		siteServerMgr: siteServerMgr,
		siteHandler:   siteHandler,
		redisClient:   redisClient,
		monitor:       monitor,
		crawlerLogMgr: crawlerLogMgr,
		visitLogMgr:   visitLogMgr,
		cfg:           cfg,
	}
}

// GetSites 获取站点列表
func (c *SitesController) GetSites(ctx *gin.Context) {
	// 从配置管理器获取当前配置
	currentConfig := c.configManager.GetConfig()
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    currentConfig.Sites,
	})
}

// GetSite 获取单个站点信息
func (c *SitesController) GetSite(ctx *gin.Context) {
	id := ctx.Param("id")
	// 从配置管理器获取当前配置
	currentConfig := c.configManager.GetConfig()
	for _, site := range currentConfig.Sites {
		if site.ID == id {
			ctx.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "success",
				"data":    site,
			})
			return
		}
	}
	ctx.JSON(http.StatusNotFound, gin.H{
		"code":    404,
		"message": "Site not found",
	})
}

// GetSiteConfig 获取站点的Redis配置（包括预渲染和推送配置）
func (c *SitesController) GetSiteConfig(ctx *gin.Context) {
	id := ctx.Param("id")
	configType := ctx.Query("type") // prerender 或 push
	
	if c.redisClient == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Redis client not available",
		})
		return
	}
	
	var configKey string
	switch configType {
	case "prerender":
		configKey = id + "_prerender"
	case "push":
		configKey = id + "_push"
	case "waf":
		configKey = id + "_waf"
	default:
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid config type. Use 'prerender' or 'push'",
		})
		return
	}
	
	config, err := c.redisClient.GetSiteStats(configKey)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get site config from Redis",
		})
		return
	}
	
	if len(config) == 0 {
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Site config not found in Redis",
		})
		return
	}
	
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    config,
	})
}

// AddSite 添加站点
func (c *SitesController) AddSite(ctx *gin.Context) {
	var site config.SiteConfig
	if err := ctx.ShouldBindJSON(&site); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request",
		})
		return
	}

	// 验证域名：只允许127.0.0.1或localhost
	for _, domain := range site.Domains {
		if domain != "127.0.0.1" && domain != "localhost" {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "Only 127.0.0.1 or localhost are allowed as domains",
			})
			return
		}
	}

	// 验证端口是否可用
	if !isPortAvailable(site.Port) {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Port is either reserved or already in use",
		})
		return
	}

	// 为新站点生成唯一ID
	site.ID = uuid.New().String()

	// 从配置管理器获取当前配置并更新
	currentConfig := c.configManager.GetConfig()
	currentConfig.Sites = append(currentConfig.Sites, site)

	// 保存配置到文件
	if err := c.configManager.SaveConfig(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to save site configuration",
		})
		return
	}

	// 启动新站点的服务器实例
	siteHandler := c.siteHandler.CreateSiteHandler(site, c.crawlerLogMgr, c.visitLogMgr, c.monitor, c.cfg.Dirs.StaticDir)

	// 启动站点服务器
	c.siteServerMgr.StartSiteServer(site, c.cfg.Server.Address, c.cfg.Dirs.StaticDir, c.crawlerLogMgr, siteHandler)

	// 保存站点配置到Redis
	if c.redisClient != nil {
		// 保存站点统计信息
		stats := map[string]interface{}{
			"name":   site.Name,
			"domain": site.Domains[0],
			"port":   site.Port,
			"mode":   site.Mode,
		}
		if err := c.redisClient.SetSiteStats(site.ID, stats); err != nil {
			logging.DefaultLogger.Warn("Failed to save site stats to Redis: %v", err)
		}

		// 保存预渲染配置（扁平化结构，不使用嵌套map）
		preheatConfig := map[string]interface{}{
			"enabled":             site.Prerender.Enabled,
			"pool_size":           site.Prerender.PoolSize,
			"min_pool_size":       site.Prerender.MinPoolSize,
			"max_pool_size":       site.Prerender.MaxPoolSize,
			"timeout":             site.Prerender.Timeout,
			"cache_ttl":           site.Prerender.CacheTTL,
			"idle_timeout":        site.Prerender.IdleTimeout,
			"preheat_enabled":     site.Prerender.Preheat.Enabled,
			"preheat_sitemap_url": site.Prerender.Preheat.SitemapURL,
			"preheat_schedule":    site.Prerender.Preheat.Schedule,
			"preheat_concurrency": site.Prerender.Preheat.Concurrency,
			"preheat_max_depth":   site.Prerender.Preheat.MaxDepth,
		}
		if err := c.redisClient.SetSiteStats(site.ID+"_prerender", preheatConfig); err != nil {
			logging.DefaultLogger.Warn("Failed to save prerender config to Redis: %v", err)
		}

		// 保存推送配置
		pushConfig := map[string]interface{}{
			"enabled":           site.Prerender.Push.Enabled,
			"baidu_api":         site.Prerender.Push.BaiduAPI,
			"baidu_token":       site.Prerender.Push.BaiduToken,
			"bing_api":          site.Prerender.Push.BingAPI,
			"bing_token":        site.Prerender.Push.BingToken,
			"baidu_daily_limit": site.Prerender.Push.BaiduDailyLimit,
			"bing_daily_limit":  site.Prerender.Push.BingDailyLimit,
			"push_domain":       site.Prerender.Push.PushDomain,
		}
		if err := c.redisClient.SetSiteStats(site.ID+"_push", pushConfig); err != nil {
			logging.DefaultLogger.Warn("Failed to save push config to Redis: %v", err)
		}

		// 保存WAF配置
		wafConfig := map[string]interface{}{
			"firewall_enabled":    site.Firewall.Enabled,
			"default_action":      site.Firewall.ActionConfig.DefaultAction,
			"block_message":       site.Firewall.ActionConfig.BlockMessage,
			"geoip_enabled":       site.Firewall.GeoIPConfig.Enabled,
			"geoip_block_list":    strings.Join(site.Firewall.GeoIPConfig.BlockList, ","),
			"ratelimit_enabled":   site.Firewall.RateLimitConfig.Enabled,
			"ratelimit_requests":  site.Firewall.RateLimitConfig.Requests,
			"ratelimit_window":    site.Firewall.RateLimitConfig.Window,
			"ratelimit_ban_time":  site.Firewall.RateLimitConfig.BanTime,
			"blacklist":           strings.Join(site.Firewall.Blacklist, ","),
			"whitelist":           strings.Join(site.Firewall.Whitelist, ","),
		}
		if err := c.redisClient.SetSiteStats(site.ID+"_waf", wafConfig); err != nil {
			logging.DefaultLogger.Warn("Failed to save WAF config to Redis: %v", err)
		}
	}

	// 记录系统日志
	logging.DefaultLogger.LogAdminAction(
		"admin",
		ctx.ClientIP(),
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

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Site added successfully",
		"data":    site,
	})
}

// UpdateSite 更新站点
func (c *SitesController) UpdateSite(ctx *gin.Context) {
	id := ctx.Param("id")
	var siteUpdates config.SiteConfig
	if err := ctx.ShouldBindJSON(&siteUpdates); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request",
		})
		return
	}

	// 验证域名：只允许127.0.0.1或localhost
	for _, domain := range siteUpdates.Domains {
		if domain != "127.0.0.1" && domain != "localhost" {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "Only 127.0.0.1 or localhost are allowed as domains",
			})
			return
		}
	}

	// 从配置管理器获取当前配置
	currentConfig := c.configManager.GetConfig()

	// 查找并更新指定站点
	var updatedSite *config.SiteConfig
	var oldSite *config.SiteConfig

	for i, s := range currentConfig.Sites {
		if s.ID == id {
			// 保存旧站点信息
			oldSite = &s

			// 检查端口是否可用（仅当端口改变时）
			if s.Port != siteUpdates.Port {
				if !isPortAvailable(siteUpdates.Port) {
					ctx.JSON(http.StatusBadRequest, gin.H{
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
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Site not found",
		})
		return
	}

	// 保存配置到文件
	if err := c.configManager.SaveConfig(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to save site configuration",
		})
		return
	}

	// 停止旧的站点服务器
	if _, exists := c.siteServerMgr.GetSiteServer(oldSite.ID); exists {
		c.siteServerMgr.StopSiteServer(oldSite.ID)
	}

	// 启动新的站点服务器
	siteHandler := c.siteHandler.CreateSiteHandler(*updatedSite, c.crawlerLogMgr, c.visitLogMgr, c.monitor, c.cfg.Dirs.StaticDir)

	// 启动站点服务器
	c.siteServerMgr.StartSiteServer(*updatedSite, c.cfg.Server.Address, c.cfg.Dirs.StaticDir, c.crawlerLogMgr, siteHandler)

	// 保存站点配置到Redis
	if c.redisClient != nil {
		// 保存站点统计信息
		stats := map[string]interface{}{
			"name":   updatedSite.Name,
			"domain": updatedSite.Domains[0],
			"port":   updatedSite.Port,
			"mode":   updatedSite.Mode,
		}
		if err := c.redisClient.SetSiteStats(updatedSite.ID, stats); err != nil {
			logging.DefaultLogger.Warn("Failed to save site stats to Redis: %v", err)
		}

		// 保存预渲染配置（扁平化结构，不使用嵌套map）
		preheatConfig := map[string]interface{}{
			"enabled":             updatedSite.Prerender.Enabled,
			"pool_size":           updatedSite.Prerender.PoolSize,
			"min_pool_size":       updatedSite.Prerender.MinPoolSize,
			"max_pool_size":       updatedSite.Prerender.MaxPoolSize,
			"timeout":             updatedSite.Prerender.Timeout,
			"cache_ttl":           updatedSite.Prerender.CacheTTL,
			"idle_timeout":        updatedSite.Prerender.IdleTimeout,
			"preheat_enabled":     updatedSite.Prerender.Preheat.Enabled,
			"preheat_sitemap_url": updatedSite.Prerender.Preheat.SitemapURL,
			"preheat_schedule":    updatedSite.Prerender.Preheat.Schedule,
			"preheat_concurrency": updatedSite.Prerender.Preheat.Concurrency,
			"preheat_max_depth":   updatedSite.Prerender.Preheat.MaxDepth,
		}
		if err := c.redisClient.SetSiteStats(updatedSite.ID+"_prerender", preheatConfig); err != nil {
			logging.DefaultLogger.Error("Failed to save prerender config to Redis: %v", err)
		} else {
			logging.DefaultLogger.Info("Pre-render config saved to Redis successfully")
		}

		// 保存推送配置
		pushConfig := map[string]interface{}{
			"enabled":           updatedSite.Prerender.Push.Enabled,
			"baidu_api":         updatedSite.Prerender.Push.BaiduAPI,
			"baidu_token":       updatedSite.Prerender.Push.BaiduToken,
			"bing_api":          updatedSite.Prerender.Push.BingAPI,
			"bing_token":        updatedSite.Prerender.Push.BingToken,
			"baidu_daily_limit": updatedSite.Prerender.Push.BaiduDailyLimit,
			"bing_daily_limit":  updatedSite.Prerender.Push.BingDailyLimit,
			"push_domain":       updatedSite.Prerender.Push.PushDomain,
		}
		if err := c.redisClient.SetSiteStats(updatedSite.ID+"_push", pushConfig); err != nil {
			logging.DefaultLogger.Warn("Failed to save push config to Redis: %v", err)
		}

		// 保存WAF配置
		wafConfig := map[string]interface{}{
			"firewall_enabled":    updatedSite.Firewall.Enabled,
			"default_action":      updatedSite.Firewall.ActionConfig.DefaultAction,
			"block_message":       updatedSite.Firewall.ActionConfig.BlockMessage,
			"geoip_enabled":       updatedSite.Firewall.GeoIPConfig.Enabled,
			"geoip_block_list":    strings.Join(updatedSite.Firewall.GeoIPConfig.BlockList, ","),
			"ratelimit_enabled":   updatedSite.Firewall.RateLimitConfig.Enabled,
			"ratelimit_requests":  updatedSite.Firewall.RateLimitConfig.Requests,
			"ratelimit_window":    updatedSite.Firewall.RateLimitConfig.Window,
			"ratelimit_ban_time":  updatedSite.Firewall.RateLimitConfig.BanTime,
			"blacklist":           strings.Join(updatedSite.Firewall.Blacklist, ","),
			"whitelist":           strings.Join(updatedSite.Firewall.Whitelist, ","),
		}
		if err := c.redisClient.SetSiteStats(updatedSite.ID+"_waf", wafConfig); err != nil {
			logging.DefaultLogger.Warn("Failed to save WAF config to Redis: %v", err)
		}
	}

	// 记录系统日志
	logging.DefaultLogger.LogAdminAction(
		"admin",
		ctx.ClientIP(),
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

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Site updated successfully",
		"data":    updatedSite,
	})
}

// DeleteSite 删除站点
func (c *SitesController) DeleteSite(ctx *gin.Context) {
	id := ctx.Param("id")

	// 从配置管理器获取当前配置并更新
	currentConfig := c.configManager.GetConfig()

	// 查找并删除指定站点
	for i, site := range currentConfig.Sites {
		if site.ID == id {
			// 停止站点服务器
			c.siteServerMgr.StopSiteServer(site.ID)

			// 删除Redis中的站点数据
			if c.redisClient != nil {
				if err := c.redisClient.DeleteSiteData(site.ID); err != nil {
					logging.DefaultLogger.Warn("Failed to delete site data from Redis for site %s: %v", site.Name, err)
				} else {
					logging.DefaultLogger.Info("Deleted site data from Redis for site %s", site.Name)
				}
			}

			// 删除站点的静态资源目录
			staticDir := filepath.Join(c.cfg.Dirs.StaticDir, site.ID)
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
			if err := c.configManager.SaveConfig(); err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{
					"code":    500,
					"message": "Failed to save site configuration",
				})
				return
			}

			// 记录系统日志
			logging.DefaultLogger.LogAdminAction(
				"admin",
				ctx.ClientIP(),
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

			ctx.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "Site deleted successfully",
			})
			return
		}
	}

	// 如果站点不存在，返回404
	ctx.JSON(http.StatusNotFound, gin.H{
		"code":    404,
		"message": "Site not found",
	})
}

// GetStaticFiles 获取站点的静态资源文件列表
func (c *SitesController) GetStaticFiles(ctx *gin.Context) {
	id := ctx.Param("id")
	path := ctx.Query("path")

	// 从配置管理器获取当前配置
	currentConfig := c.configManager.GetConfig()

	// 查找指定站点
	var site *config.SiteConfig
	for i, s := range currentConfig.Sites {
		if s.ID == id {
			site = &currentConfig.Sites[i]
			break
		}
	}

	if site == nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Site not found",
		})
		return
	}

	// 构建静态资源目录路径
	siteStaticDir := filepath.Join(c.cfg.Dirs.StaticDir, site.ID)

	// 构建完整的文件路径
	filePath := filepath.Join(siteStaticDir, path)

	// 检查文件路径是否存在
	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		// 如果是目录路径不存在，返回空文件列表
		if strings.HasSuffix(path, "/") {
			ctx.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "success",
				"data":    []gin.H{},
			})
			return
		}
		// 如果是文件路径不存在，返回404
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "File not found",
		})
		return
	}

	// 如果是文件，直接返回文件内容
	if fileInfo.IsDir() {
		// 如果是目录，返回目录下的文件列表
		files, err := os.ReadDir(filePath)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "Failed to read directory",
			})
			return
		}

		var fileList []gin.H
		for _, file := range files {
			fileInfo, _ := file.Info()
			fileType := "file"
			if file.IsDir() {
				fileType = "dir"
			}
			var filePath string
			if path == "" || path == "/" {
				filePath = "/" + file.Name()
			} else {
				filePath = filepath.Join(path, file.Name())
			}
			var key string
			if file.IsDir() {
				key = file.Name() + "/"
			} else {
				key = file.Name()
			}
			fileList = append(fileList, gin.H{
				"key":   key,
				"name":  file.Name(),
				"type":  fileType,
				"size":  fileInfo.Size(),
				"isDir": file.IsDir(),
				"path":  filePath,
			})
		}

		ctx.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "success",
			"data":    fileList,
		})
	} else {
		// 返回文件内容
		ctx.File(filePath)
	}
}

// UploadStaticFile 上传静态资源文件
func (c *SitesController) UploadStaticFile(ctx *gin.Context) {
	id := ctx.Param("id")
	path := ctx.PostForm("path")

	// 从配置管理器获取当前配置
	currentConfig := c.configManager.GetConfig()

	// 查找指定站点
	var site *config.SiteConfig
	for i, s := range currentConfig.Sites {
		if s.ID == id {
			site = &currentConfig.Sites[i]
			break
		}
	}

	if site == nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Site not found",
		})
		return
	}

	// 构建静态资源目录路径
	siteStaticDir := filepath.Join(c.cfg.Dirs.StaticDir, site.ID)

	// 保存上传的文件
	file, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Failed to get file",
		})
		return
	}

	// 构建完整的文件路径，包含文件名
	filePath := filepath.Join(siteStaticDir, path, file.Filename)

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to create directory",
		})
		return
	}

	// 保存文件
	if err := ctx.SaveUploadedFile(file, filePath); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to save file",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "File uploaded successfully",
	})
}

// ExtractFile 解压文件
func (c *SitesController) ExtractFile(ctx *gin.Context) {
	id := ctx.Param("id")
	fileName := ctx.PostForm("filename")
	path := ctx.PostForm("path")

	// 从配置管理器获取当前配置
	currentConfig := c.configManager.GetConfig()

	// 查找指定站点
	var site *config.SiteConfig
	for i, s := range currentConfig.Sites {
		if s.ID == id {
			site = &currentConfig.Sites[i]
			break
		}
	}

	if site == nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Site not found",
		})
		return
	}

	// 构建静态资源目录路径
	siteStaticDir := filepath.Join(c.cfg.Dirs.StaticDir, site.ID)

	// 清理路径，移除前导斜杠，确保filepath.Join工作正常
	cleanPath := strings.TrimPrefix(path, "/")
	if cleanPath == "" {
		cleanPath = "."
	}

	// 构建完整的文件路径
	filePath := filepath.Join(siteStaticDir, cleanPath, fileName)

	// 检查文件是否存在
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": fmt.Sprintf("File not found at path: %s", filePath),
		})
		return
	}

	// 构建解压目标目录
	destDir := filepath.Join(siteStaticDir, cleanPath)

	// 根据文件扩展名选择解压方法
	if !strings.HasSuffix(strings.ToLower(fileName), ".zip") {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Only ZIP files are supported for extraction",
		})
		return
	}

	// 确保目标目录存在
	if err := os.MkdirAll(destDir, 0755); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": fmt.Sprintf("Failed to create destination directory: %v", err),
		})
		return
	}
	// 解压ZIP文件
	if err := ExtractZIP(filePath, destDir); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": fmt.Sprintf("Failed to extract ZIP file: %v", err),
		})
		return
	}

	// 将提取的文件信息存储到Redis中
	if c.redisClient != nil {
		// 遍历解压目录，收集所有HTML文件的URL
		var htmlFiles []string
		walkErr := filepath.Walk(destDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".html") {
				// 构建相对URL路径
				relPath, err := filepath.Rel(destDir, path)
				if err != nil {
					return err
				}
				// 构建完整URL
				baseURL := fmt.Sprintf("http://%s:%d", site.Domains[0], site.Port)
				fileURL := fmt.Sprintf("%s/%s", baseURL, strings.ReplaceAll(relPath, "\\", "/"))
				htmlFiles = append(htmlFiles, fileURL)
			}
			return nil
		})

		if walkErr != nil {
			log.Printf("Failed to walk extracted files: %v", walkErr)
		} else {
			// 将收集到的URL存储到Redis中
			for _, url := range htmlFiles {
				if err := c.redisClient.AddURL(site.ID, url); err != nil {
					log.Printf("Failed to add URL to Redis: %v", err)
					continue
				}
				log.Printf("Added URL to Redis: %s", url)
			}
			// 更新站点统计信息
			if len(htmlFiles) > 0 {
				// 创建新的统计信息
				stats := map[string]interface{}{
					"url_count": len(htmlFiles),
				}
				if err := c.redisClient.SetSiteStats(site.ID, stats); err != nil {
					log.Printf("Failed to update site stats: %v", err)
				}
			}
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "File extracted successfully",
	})
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

// ExtractZIP 解压ZIP文件，导出供测试使用
func ExtractZIP(filePath, destDir string) error {
	// 打开ZIP文件
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	// 确保目标目录存在
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// 遍历ZIP文件中的所有文件
	for _, file := range reader.File {
		// 构建目标文件路径
		destFilePath := filepath.Join(destDir, file.Name)

		// 检查文件是否是目录
		if file.FileInfo().IsDir() {
			// 创建目录
			if err := os.MkdirAll(destFilePath, file.Mode()); err != nil {
				return err
			}
			continue
		}

		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(destFilePath), 0755); err != nil {
			return err
		}

		// 创建目标文件
		destFile, err := os.OpenFile(destFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}
		// 不使用defer，而是立即关闭文件

		// 获取ZIP文件中的文件
		zipFile, err := file.Open()
		if err != nil {
			destFile.Close() // 确保文件关闭
			return err
		}

		// 复制文件内容
		if _, err := io.Copy(destFile, zipFile); err != nil {
			zipFile.Close()
			destFile.Close() // 确保文件关闭
			return err
		}

		// 立即关闭文件，避免资源泄漏和文件锁定问题
		zipFile.Close()
		destFile.Close()
	}

	return nil
}

// DeleteStaticFile 删除静态资源文件
func (c *SitesController) DeleteStaticFile(ctx *gin.Context) {
	id := ctx.Param("id")
	path := ctx.Query("path")

	// 从配置管理器获取当前配置
	currentConfig := c.configManager.GetConfig()

	// 查找指定站点
	var site *config.SiteConfig
	for i, s := range currentConfig.Sites {
		if s.ID == id {
			site = &currentConfig.Sites[i]
			break
		}
	}

	if site == nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Site not found",
		})
		return
	}

	// 构建静态资源目录路径
	siteStaticDir := filepath.Join(c.cfg.Dirs.StaticDir, site.ID)

	// 构建完整的文件路径
	filePath := filepath.Join(siteStaticDir, path)

	// 删除文件或目录
	if err := os.RemoveAll(filePath); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to delete file",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "File deleted successfully",
	})
}