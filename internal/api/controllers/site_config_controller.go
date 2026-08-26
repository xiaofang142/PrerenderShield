package controllers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
)

// persistSiteConfigToRedis 将站点完整配置（基础信息+预渲染+推送+WAF）持久化到 Redis
func (c *SitesController) persistSiteConfigToRedis(site *config.SiteConfig) {
	c.saveSiteStatsToRedis(site)
	c.savePrerenderConfigToRedis(site)
	c.savePushConfigToRedis(site)
	c.saveWafConfigToRedis(site)
}

// saveSiteStatsToRedis 保存站点基础统计信息
func (c *SitesController) saveSiteStatsToRedis(site *config.SiteConfig) {
	if c.redisClient == nil {
		return
	}
	// Domains 由 validateDomains 保证非空，但本函数还有其他调用路径，防御性兜底
	domain := ""
	if len(site.Domains) > 0 {
		domain = site.Domains[0]
	}
	stats := map[string]interface{}{
		"name":   site.Name,
		"domain": domain,
		"port":   site.Port,
		"mode":   site.Mode,
	}
	if err := c.redisClient.SetSiteStats(site.ID, stats); err != nil {
		logging.DefaultLogger.Warn("Failed to save site stats to Redis: %v", err)
	}
}

// savePrerenderConfigToRedis 保存预渲染配置（扁平化结构，不使用嵌套map）
func (c *SitesController) savePrerenderConfigToRedis(site *config.SiteConfig) {
	if c.redisClient == nil {
		return
	}
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
		"crawler_headers":     strings.Join(site.Prerender.CrawlerHeaders, "\n"),
	}
	if err := c.redisClient.SetSiteStats(site.ID+"_prerender", preheatConfig); err != nil {
		logging.DefaultLogger.Warn("Failed to save prerender config to Redis: %v", err)
	} else {
		logging.DefaultLogger.Info("Pre-render config saved to Redis successfully")
	}
}

// savePushConfigToRedis 保存推送配置
func (c *SitesController) savePushConfigToRedis(site *config.SiteConfig) {
	if c.redisClient == nil {
		return
	}
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
}

// saveWafConfigToRedis 保存WAF配置
func (c *SitesController) buildWafConfigMap(site *config.SiteConfig) map[string]interface{} {
	return map[string]interface{}{
		"firewall_enabled":   site.Firewall.Enabled,
		"default_action":     site.Firewall.ActionConfig.DefaultAction,
		"block_message":      site.Firewall.ActionConfig.BlockMessage,
		"geoip_enabled":      site.Firewall.GeoIPConfig.Enabled,
		"geoip_block_list":   strings.Join(site.Firewall.GeoIPConfig.BlockList, ","),
		"ratelimit_enabled":  site.Firewall.RateLimitConfig.Enabled,
		"ratelimit_requests": site.Firewall.RateLimitConfig.Requests,
		"ratelimit_window":   site.Firewall.RateLimitConfig.Window,
		"ratelimit_ban_time": site.Firewall.RateLimitConfig.BanTime,
		"blacklist":          strings.Join(site.Firewall.Blacklist, ","),
		"whitelist":          strings.Join(site.Firewall.Whitelist, ","),
	}
}

func (c *SitesController) saveWafConfigToRedis(site *config.SiteConfig) {
	if c.redisClient == nil {
		return
	}
	wafConfig := c.buildWafConfigMap(site)
	if err := c.redisClient.SetSiteStats(site.ID+"_waf", wafConfig); err != nil {
		logging.DefaultLogger.Warn("Failed to save WAF config to Redis: %v", err)
	}
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
		// Redis 中无定制配置时，回退到站点静态配置中的对应部分，
		// 避免新装环境 GET 配置直接 404（前端无需再自行兜底）
		for i := range c.cfg.Sites {
			if c.cfg.Sites[i].ID != id {
				continue
			}
			site := c.cfg.Sites[i]
			var fallback interface{}
			switch configType {
			case "prerender":
				fallback = site.Prerender
			case "push":
				fallback = site.Prerender.Push
			case "waf":
				// 与 saveWafConfigToRedis 保持相同字段形状，前端无需区分来源
				fallback = c.buildWafConfigMap(&site)
			}
			if fallback != nil {
				ctx.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    fallback,
				})
				return
			}
			break
		}
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

// UpdateSitePrerenderConfig 独立更新渲染预热配置
func (c *SitesController) UpdateSitePrerenderConfig(ctx *gin.Context) {
	id := ctx.Param("id")
	var prerenderUpdates config.PrerenderConfig
	if err := ctx.ShouldBindJSON(&prerenderUpdates); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request",
		})
		return
	}

	// 从配置管理器获取当前配置
	if c.configManager == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Configuration manager not available",
		})
		return
	}
	currentConfig := c.configManager.GetConfig()

	// 查找并更新指定站点
	var updatedSite *config.SiteConfig
	var oldSite *config.SiteConfig

	for i, s := range currentConfig.Sites {
		if s.ID == id {
			oldSite = &s

			// 仅更新预渲染相关配置，保留推送配置(Push)
			// 注意：前端传来的 prerenderUpdates 中 Push 可能为空或默认值，所以我们需要手动保留原有的 Push 配置
			originalPush := currentConfig.Sites[i].Prerender.Push
			currentConfig.Sites[i].Prerender = prerenderUpdates
			currentConfig.Sites[i].Prerender.Push = originalPush

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

	// 重启站点服务器
	if _, exists := c.siteServerMgr.GetSiteServer(oldSite.ID); exists {
		c.siteServerMgr.StopSiteServer(oldSite.ID)
	}
	siteHandler := c.siteHandler.CreateSiteHandler(*updatedSite, c.concreteCrawlerLogMgr, c.concreteVisitLogMgr, c.concreteMonitor, c.cfg.Dirs.StaticDir)
	c.siteServerMgr.StartSiteServer(*updatedSite, c.cfg.Server.Address, c.cfg.Dirs.StaticDir, c.concreteCrawlerLogMgr, siteHandler)

	// 保存预渲染配置到Redis
	c.savePrerenderConfigToRedis(updatedSite)

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Prerender configuration updated successfully",
		"data":    updatedSite.Prerender,
	})
}

// UpdateSitePushConfig 独立更新推送配置
func (c *SitesController) UpdateSitePushConfig(ctx *gin.Context) {
	id := ctx.Param("id")
	var pushUpdates config.PushConfig
	if err := ctx.ShouldBindJSON(&pushUpdates); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request",
		})
		return
	}

	if c.configManager == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Configuration manager not available",
		})
		return
	}
	currentConfig := c.configManager.GetConfig()
	var updatedSite *config.SiteConfig
	var oldSite *config.SiteConfig

	for i, s := range currentConfig.Sites {
		if s.ID == id {
			oldSite = &s
			currentConfig.Sites[i].Prerender.Push = pushUpdates
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

	if err := c.configManager.SaveConfig(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to save site configuration",
		})
		return
	}

	if _, exists := c.siteServerMgr.GetSiteServer(oldSite.ID); exists {
		c.siteServerMgr.StopSiteServer(oldSite.ID)
	}
	siteHandler := c.siteHandler.CreateSiteHandler(*updatedSite, c.concreteCrawlerLogMgr, c.concreteVisitLogMgr, c.concreteMonitor, c.cfg.Dirs.StaticDir)
	c.siteServerMgr.StartSiteServer(*updatedSite, c.cfg.Server.Address, c.cfg.Dirs.StaticDir, c.concreteCrawlerLogMgr, siteHandler)

	c.savePushConfigToRedis(updatedSite)

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Push configuration updated successfully",
		"data":    updatedSite.Prerender.Push,
	})
}

// UpdateSiteFirewallConfig 独立更新防火墙配置
func (c *SitesController) UpdateSiteFirewallConfig(ctx *gin.Context) {
	id := ctx.Param("id")
	var firewallUpdates config.FirewallConfig
	if err := ctx.ShouldBindJSON(&firewallUpdates); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request",
		})
		return
	}

	if c.configManager == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Configuration manager not available",
		})
		return
	}
	currentConfig := c.configManager.GetConfig()
	var updatedSite *config.SiteConfig
	var oldSite *config.SiteConfig

	for i, s := range currentConfig.Sites {
		if s.ID == id {
			oldSite = &s
			currentConfig.Sites[i].Firewall = firewallUpdates
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

	if err := c.configManager.SaveConfig(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to save site configuration",
		})
		return
	}

	if _, exists := c.siteServerMgr.GetSiteServer(oldSite.ID); exists {
		c.siteServerMgr.StopSiteServer(oldSite.ID)
	}
	siteHandler := c.siteHandler.CreateSiteHandler(*updatedSite, c.concreteCrawlerLogMgr, c.concreteVisitLogMgr, c.concreteMonitor, c.cfg.Dirs.StaticDir)
	c.siteServerMgr.StartSiteServer(*updatedSite, c.cfg.Server.Address, c.cfg.Dirs.StaticDir, c.concreteCrawlerLogMgr, siteHandler)

	c.saveWafConfigToRedis(updatedSite)

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Firewall configuration updated successfully",
		"data":    updatedSite.Firewall,
	})
}
