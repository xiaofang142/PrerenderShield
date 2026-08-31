package controllers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"prerender-shield/internal/config"
	"prerender-shield/internal/licensing"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/redis"
	sitehandler "prerender-shield/internal/site-handler"
	siteserver "prerender-shield/internal/site-server"
	"prerender-shield/internal/utils"
)

// validateDomains 校验站点域名格式：
// 非空、无协议前缀、无路径/查询参数、不含空白字符。
// 允许任意合法域名（localhost / IP / 真实域名），站点数量由 licensing 策略另行限制。
func validateDomains(domains []string) error {
	// 站点必须至少绑定一个域名：下游 saveSiteStatsToRedis 依赖 Domains[0]
	if len(domains) == 0 {
		return fmt.Errorf("at least one domain is required")
	}
	for _, domain := range domains {
		d := strings.TrimSpace(strings.ToLower(domain))
		if d == "" {
			return fmt.Errorf("domain cannot be empty")
		}
		if strings.ContainsAny(d, " \t/\\?#") || strings.Contains(d, "://") {
			return fmt.Errorf("invalid domain format: %s", domain)
		}
	}
	return nil
}

// ConfigManagerInterface defines the interface for configuration management
// (P0-3: 新增 Mutate 提供事务性修改语义)
type ConfigManagerInterface interface {
	GetConfig() *config.Config
	UpdateConfig(cfg *config.Config)
	SaveConfig() error
	Mutate(mutate func(c *config.Config) (*config.Config, error)) error
}

// SiteServerManagerInterface defines the interface for site server management
type SiteServerManagerInterface interface {
	StartSiteServer(site config.SiteConfig, serverAddr, staticDir string, crawlerLogMgr *logging.CrawlerLogManager, siteHandler http.Handler)
	StopSiteServer(siteID string) error
	GetSiteServer(siteID string) (*http.Server, bool)
}

// SiteHandlerInterface defines the interface for site handler creation
type SiteHandlerInterface interface {
	CreateSiteHandler(site config.SiteConfig, crawlerLogMgr *logging.CrawlerLogManager, visitLogMgr *logging.VisitLogManager, monitor *monitoring.Monitor, staticDir string) http.Handler
}

// RedisClientInterface defines the interface for Redis operations
type RedisClientInterface interface {
	SetSiteStats(siteID string, stats map[string]interface{}) error
	GetSiteStats(key string) (map[string]string, error)
	DeleteSiteData(siteID string) error
	ClearSiteCache(siteID string) error
	AddURL(siteID, url string) error
}

// MonitorInterface defines the interface for monitoring
type MonitorInterface interface {
	GetStats() map[string]interface{}
}

// CrawlerLogManagerInterface defines the interface for crawler log management
type CrawlerLogManagerInterface interface {
	RecordCrawlerLog(crawlerLog logging.CrawlerLog)
}

// VisitLogManagerInterface defines the interface for visit log management
type VisitLogManagerInterface interface {
	RecordVisitLog(visitLog logging.VisitLog)
}

// SitesController 站点管理控制器
type SitesController struct {
	configManager ConfigManagerInterface
	siteServerMgr SiteServerManagerInterface
	siteHandler   SiteHandlerInterface
	redisClient   RedisClientInterface
	monitor       MonitorInterface
	crawlerLogMgr CrawlerLogManagerInterface
	visitLogMgr   VisitLogManagerInterface
	cfg           configRef

	// Concrete type references for use in wrapper methods
	concreteSiteServerMgr *siteserver.Manager
	concreteSiteHandler   *sitehandler.Handler
	concreteCrawlerLogMgr *logging.CrawlerLogManager
	concreteVisitLogMgr   *logging.VisitLogManager
	concreteMonitor       *monitoring.Monitor
}

// configManagerWrapper wraps config.ConfigManager to implement ConfigManagerInterface
type configManagerWrapper struct {
	cm *config.ConfigManager
}

func (w *configManagerWrapper) GetConfig() *config.Config {
	return w.cm.GetConfig()
}

func (w *configManagerWrapper) UpdateConfig(cfg *config.Config) {
	w.cm.UpdateConfig(cfg)
}

func (w *configManagerWrapper) SaveConfig() error {
	return w.cm.SaveConfig()
}

// Mutate 委托给 config.ConfigManager.Mutate (P0-3)
func (w *configManagerWrapper) Mutate(mutate func(c *config.Config) (*config.Config, error)) error {
	return w.cm.Mutate(mutate)
}

// siteServerMgrWrapper wraps siteserver.Manager to implement SiteServerManagerInterface
type siteServerMgrWrapper struct {
	ctrl *SitesController
}

func (w *siteServerMgrWrapper) StartSiteServer(site config.SiteConfig, serverAddr, staticDir string, crawlerLogMgr *logging.CrawlerLogManager, siteHandler http.Handler) {
	w.ctrl.concreteSiteServerMgr.StartSiteServer(site, serverAddr, staticDir, crawlerLogMgr, siteHandler)
}

func (w *siteServerMgrWrapper) StopSiteServer(siteID string) error {
	return w.ctrl.concreteSiteServerMgr.StopSiteServer(siteID)
}

func (w *siteServerMgrWrapper) GetSiteServer(siteID string) (*http.Server, bool) {
	return w.ctrl.concreteSiteServerMgr.GetSiteServer(siteID)
}

// siteHandlerWrapper wraps sitehandler.Handler to implement SiteHandlerInterface
type siteHandlerWrapper struct {
	ctrl *SitesController
}

func (w *siteHandlerWrapper) CreateSiteHandler(site config.SiteConfig, crawlerLogMgr *logging.CrawlerLogManager, visitLogMgr *logging.VisitLogManager, monitor *monitoring.Monitor, staticDir string) http.Handler {
	return w.ctrl.concreteSiteHandler.CreateSiteHandler(site, crawlerLogMgr, visitLogMgr, monitor, staticDir)
}

// redisClientWrapper wraps redis.Client to implement RedisClientInterface
type redisClientWrapper struct {
	client *redis.Client
}

func (w *redisClientWrapper) SetSiteStats(siteID string, stats map[string]interface{}) error {
	return w.client.SetSiteStats(siteID, stats)
}

func (w *redisClientWrapper) GetSiteStats(key string) (map[string]string, error) {
	return w.client.GetSiteStats(key)
}

func (w *redisClientWrapper) DeleteSiteData(siteID string) error {
	return w.client.DeleteSiteData(siteID)
}

func (w *redisClientWrapper) ClearSiteCache(siteID string) error {
	return w.client.ClearSiteCache(siteID)
}

func (w *redisClientWrapper) AddURL(siteID, url string) error {
	return w.client.AddURL(siteID, url)
}

// monitorWrapper wraps monitoring.Monitor to implement MonitorInterface
type monitorWrapper struct {
	ctrl *SitesController
}

func (w *monitorWrapper) GetStats() map[string]interface{} {
	return w.ctrl.concreteMonitor.GetStats()
}

// crawlerLogMgrWrapper wraps logging.CrawlerLogManager to implement CrawlerLogManagerInterface
type crawlerLogMgrWrapper struct {
	ctrl *SitesController
}

func (w *crawlerLogMgrWrapper) RecordCrawlerLog(crawlerLog logging.CrawlerLog) {
	w.ctrl.concreteCrawlerLogMgr.RecordCrawlerLog(crawlerLog)
}

// visitLogMgrWrapper wraps logging.VisitLogManager to implement VisitLogManagerInterface
type visitLogMgrWrapper struct {
	ctrl *SitesController
}

func (w *visitLogMgrWrapper) RecordVisitLog(visitLog logging.VisitLog) {
	w.ctrl.concreteVisitLogMgr.RecordVisitLog(visitLog)
}

// NewSitesController 创建站点管理控制器实例
func NewSitesController(
	configManager ConfigManagerInterface,
	siteServerMgr SiteServerManagerInterface,
	siteHandler SiteHandlerInterface,
	redisClient RedisClientInterface,
	monitor MonitorInterface,
	crawlerLogMgr CrawlerLogManagerInterface,
	visitLogMgr VisitLogManagerInterface,
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
		cfg:           configRef{snapshot: cfg},
	}
}

// NewSitesControllerWithConcreteDeps creates a SitesController from concrete implementations
func NewSitesControllerWithConcreteDeps(
	configManager *config.ConfigManager,
	siteServerMgr *siteserver.Manager,
	siteHandler *sitehandler.Handler,
	redisClient *redis.Client,
	monitor *monitoring.Monitor,
	crawlerLogMgr *logging.CrawlerLogManager,
	visitLogMgr *logging.VisitLogManager,
	cfg *config.Config,
) *SitesController {
	var cm ConfigManagerInterface
	var ssm SiteServerManagerInterface
	var sh SiteHandlerInterface
	var rc RedisClientInterface
	var m MonitorInterface
	var clm CrawlerLogManagerInterface
	var vlm VisitLogManagerInterface

	if configManager != nil {
		cm = &configManagerWrapper{cm: configManager}
	}
	if siteServerMgr != nil {
		ssm = &siteServerMgrWrapper{ctrl: nil} // Will set ctrl after creating controller
	}
	if siteHandler != nil {
		sh = &siteHandlerWrapper{ctrl: nil}
	}
	if redisClient != nil {
		rc = &redisClientWrapper{client: redisClient}
	}
	if monitor != nil {
		m = &monitorWrapper{ctrl: nil}
	}
	if crawlerLogMgr != nil {
		clm = &crawlerLogMgrWrapper{ctrl: nil}
	}
	if visitLogMgr != nil {
		vlm = &visitLogMgrWrapper{ctrl: nil}
	}

	controller := &SitesController{
		configManager:         cm,
		siteServerMgr:         ssm,
		siteHandler:           sh,
		redisClient:           rc,
		monitor:               m,
		crawlerLogMgr:         clm,
		visitLogMgr:           vlm,
		cfg:                   configRef{snapshot: cfg},
		concreteSiteServerMgr: siteServerMgr,
		concreteSiteHandler:   siteHandler,
		concreteCrawlerLogMgr: crawlerLogMgr,
		concreteVisitLogMgr:   visitLogMgr,
		concreteMonitor:       monitor,
	}

	// Set controller references in wrappers
	if ssm != nil {
		ssm.(*siteServerMgrWrapper).ctrl = controller
	}
	if sh != nil {
		sh.(*siteHandlerWrapper).ctrl = controller
	}
	if m != nil {
		m.(*monitorWrapper).ctrl = controller
	}
	if clm != nil {
		clm.(*crawlerLogMgrWrapper).ctrl = controller
	}
	if vlm != nil {
		vlm.(*visitLogMgrWrapper).ctrl = controller
	}

	return controller
}

// GetSites 获取站点列表
func (c *SitesController) GetSites(ctx *gin.Context) {
	// 从配置管理器获取当前配置
	if c.configManager == nil {
		// 如果没有 configManager，使用 cfg 中的 Sites
		sites := []config.SiteConfig{}
		if c.cfg.current() != nil {
			sites = c.cfg.current().Sites
		}
		ctx.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "success",
			"data":    sites,
		})
		return
	}
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
	var sites []config.SiteConfig
	if c.configManager == nil {
		sites = []config.SiteConfig{}
		if c.cfg.current() != nil {
			sites = c.cfg.current().Sites
		}
	} else {
		currentConfig := c.configManager.GetConfig()
		sites = currentConfig.Sites
	}
	for _, site := range sites {
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

	// 验证站点名称
	if site.Name == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Site name is required",
		})
		return
	}

	// R14-BUG-1 修复：站点名会流入 sitemap XML、控制台与日志。拒绝 HTML/控制字符，
	// 防止存储型 XSS 与注入面（名称只允许可见安全字符，长度上限 64）。
	if len(site.Name) > 64 || strings.ContainsAny(site.Name, "<>&\"'`\n\r\t") {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid site name: must be <=64 chars and contain no HTML/control characters",
		})
		return
	}

	// 验证域名格式
	if err := validateDomains(site.Domains); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// 验证端口是否可用
	if !utils.IsPortAvailable(site.Port) {
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
	policy := licensing.NewPolicy(currentConfig.Commercial)
	if !policy.AllowsAdditionalSite(len(currentConfig.Sites)) {
		ctx.JSON(http.StatusPaymentRequired, gin.H{
			"code":    402,
			"message": policy.UpgradeMessage(len(currentConfig.Sites)),
			"data": gin.H{
				"current_sites":            len(currentConfig.Sites),
				"max_sites":                policy.MaxSites,
				"site_price_usd_per_year":  policy.SitePriceUSDPerYear,
				"private_deploy_price_usd": policy.PrivateDeployPriceUSD,
				"billing_model":            "one_site_free_all_features_then_99_usd_per_site_per_year",
			},
		})
		return
	}

	// P0-3: 使用事务性 Mutate，持久化失败时不会污染内存中的配置
	if err := c.configManager.Mutate(func(c *config.Config) (*config.Config, error) {
		c.Sites = append(c.Sites, site)
		return c, nil
	}); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to save site configuration: " + err.Error(),
		})
		return
	}

	// 启动新站点的服务器实例
	siteHandler := c.siteHandler.CreateSiteHandler(site, c.concreteCrawlerLogMgr, c.concreteVisitLogMgr, c.concreteMonitor, c.cfg.current().Dirs.StaticDir)

	// 启动站点服务器
	c.siteServerMgr.StartSiteServer(site, c.cfg.current().Server.Address, c.cfg.current().Dirs.StaticDir, c.concreteCrawlerLogMgr, siteHandler)

	// 保存站点配置到Redis
	c.persistSiteConfigToRedis(&site)

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

	// 验证域名格式
	if err := validateDomains(siteUpdates.Domains); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// R14-BUG-1：更新路径同样拒绝 HTML/控制字符站点名（与 AddSite 一致）
	if siteUpdates.Name == "" || len(siteUpdates.Name) > 64 || strings.ContainsAny(siteUpdates.Name, "<>&\"'`\n\r\t") {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid site name: must be non-empty, <=64 chars, no HTML/control characters",
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
			// 保存旧站点信息
			oldSite = &s

			// 检查端口是否可用（仅当端口改变时）
			if s.Port != siteUpdates.Port {
				if !utils.IsPortAvailable(siteUpdates.Port) {
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
	siteHandler := c.siteHandler.CreateSiteHandler(*updatedSite, c.concreteCrawlerLogMgr, c.concreteVisitLogMgr, c.concreteMonitor, c.cfg.current().Dirs.StaticDir)

	// 启动站点服务器
	c.siteServerMgr.StartSiteServer(*updatedSite, c.cfg.current().Server.Address, c.cfg.current().Dirs.StaticDir, c.concreteCrawlerLogMgr, siteHandler)

	// 保存站点配置到Redis
	c.persistSiteConfigToRedis(updatedSite)

	// 站点服务配置变更（代理目标/预渲染开关/模式等）后失效爬虫渲染缓存，
	// 避免爬虫命中旧配置下的陈旧渲染内容（若变更未测到则不产生额外副作用）。
	c.clearSiteRenderCache(updatedSite)

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
	if c.configManager == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Configuration manager not initialized",
		})
		return
	}
	currentConfig := c.configManager.GetConfig()

	// 查找并删除指定站点
	for i, site := range currentConfig.Sites {
		if site.ID == id {
			// 停止站点服务器
			c.siteServerMgr.StopSiteServer(site.ID)

			// 删除 Redis 中的站点数据
			if c.redisClient != nil {
				if err := c.redisClient.DeleteSiteData(site.ID); err != nil {
					logging.DefaultLogger.Warn("Failed to delete site data from Redis for site %s: %v", site.Name, err)
				} else {
					logging.DefaultLogger.Info("Deleted site data from Redis for site %s", site.Name)
				}
			}

			// 删除站点的静态资源目录
			staticDir := filepath.Join(c.cfg.current().Dirs.StaticDir, site.ID)
			if _, err := os.Stat(staticDir); err == nil {
				// 目录存在，删除它
				if err := os.RemoveAll(staticDir); err != nil {
					logging.DefaultLogger.Info("Failed to delete static files for site %s: %v", site.Name, err)
					// 继续执行，不中断删除流程
				} else {
					logging.DefaultLogger.Info("Deleted static files for site %s", site.Name)
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

// StartSite 启动指定站点的服务器
func (c *SitesController) StartSite(ctx *gin.Context) {
	id := ctx.Param("id")

	currentConfig := c.configManager.GetConfig()
	for _, site := range currentConfig.Sites {
		if site.ID == id {
			// 检查站点服务器是否已在运行
			if _, running := c.siteServerMgr.GetSiteServer(site.ID); running {
				ctx.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "Site server is already running",
				})
				return
			}

			// 创建站点处理器并启动服务器
			siteHandler := c.siteHandler.CreateSiteHandler(site, c.concreteCrawlerLogMgr, c.concreteVisitLogMgr, c.concreteMonitor, c.cfg.current().Dirs.StaticDir)
			c.siteServerMgr.StartSiteServer(site, c.cfg.current().Server.Address, c.cfg.current().Dirs.StaticDir, c.concreteCrawlerLogMgr, siteHandler)

			logging.DefaultLogger.LogAdminAction(
				"admin", ctx.ClientIP(), "site_start", "site",
				map[string]interface{}{"site_id": site.ID, "site_name": site.Name},
				"success", "Site started successfully",
			)

			ctx.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "Site started successfully",
			})
			return
		}
	}

	ctx.JSON(http.StatusNotFound, gin.H{
		"code":    404,
		"message": "Site not found",
	})
}

// StopSite 停止指定站点的服务器
func (c *SitesController) StopSite(ctx *gin.Context) {
	id := ctx.Param("id")

	currentConfig := c.configManager.GetConfig()
	for _, site := range currentConfig.Sites {
		if site.ID == id {
			if err := c.siteServerMgr.StopSiteServer(site.ID); err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{
					"code":    500,
					"message": "Failed to stop site server: " + err.Error(),
				})
				return
			}

			logging.DefaultLogger.LogAdminAction(
				"admin", ctx.ClientIP(), "site_stop", "site",
				map[string]interface{}{"site_id": site.ID, "site_name": site.Name},
				"success", "Site stopped successfully",
			)

			ctx.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "Site stopped successfully",
			})
			return
		}
	}

	ctx.JSON(http.StatusNotFound, gin.H{
		"code":    404,
		"message": "Site not found",
	})
}

// GetStaticFiles 获取站点的静态资源文件列表
// UploadStaticFile 上传静态资源文件
// ExtractFile 解压文件
// DeleteStaticFile 删除静态资源文件
// BatchDeleteStaticFiles 批量删除静态资源文件
