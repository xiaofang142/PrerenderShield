package controllers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/prerender/push"
	"prerender-shield/internal/redis"
)

// PushController 推送控制器
type PushController struct {
	pushManager *push.PushManager
	redisClient *redis.Client
	cfg         configRef
	sites       *SitesController // 推送配置持久化权威路径（SaveConfig+重启处理器），controller_setup 注入
}

// SetSitesController 注入站点控制器（推送配置落盘与处理器重启复用其权威路径）
func (c *PushController) SetSitesController(sc *SitesController) { c.sites = sc }

// NewPushController 创建推送控制器实例
func NewPushController(pushManager *push.PushManager, redisClient *redis.Client, cfg *config.Config) *PushController {
	return &PushController{
		pushManager: pushManager,
		redisClient: redisClient,
		cfg:         configRef{snapshot: cfg},
	}
}

// GetSites 获取站点列表
func (c *PushController) GetSites(ctx *gin.Context) {
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

	var sites []gin.H
	for _, site := range sitesList {
		// 检查站点域名是否可用
		domain := ""
		if len(site.Domains) > 0 {
			domain = site.Domains[0]
		}

		sites = append(sites, gin.H{
			"id":      site.ID,
			"name":    site.Name,
			"domain":  domain,
			"enabled": site.Prerender.Push.Enabled,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    sites,
	})
}

// GetPushStats 获取推送统计数据
func (c *PushController) GetPushStats(ctx *gin.Context) {
	// 检查必要的依赖项是否可用
	if c.cfg.current() == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "配置信息不可用",
		})
		return
	}

	if c.pushManager == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "推送管理器不可用",
		})
		return
	}

	// 检查站点列表是否可用（空列表局部兜底，不写回共享配置对象避免竞争）
	sitesList := c.cfg.current().Sites
	if sitesList == nil {
		sitesList = []config.SiteConfig{}
	}

	siteID := ctx.Query("siteId")

	if siteID == "" {
		// 获取所有站点的统计数据
		var allStats []gin.H
		for _, site := range sitesList {
			stats, err := c.pushManager.GetPushStats(site.ID)
			if err != nil {
				// 记录错误但不中断处理
				logging.DefaultLogger.Info("Failed to get push stats for site %s: %v\n", site.ID, err)
				continue
			}

			allStats = append(allStats, gin.H{
				"siteId":   site.ID,
				"siteName": site.Name,
				"stats":    stats,
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
	stats, err := c.pushManager.GetPushStats(siteID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": fmt.Sprintf("获取推送统计数据失败: %v", err),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"siteId": siteID,
			"stats":  stats,
		},
	})
}

// GetPushLogs 获取推送日志
func (c *PushController) GetPushLogs(ctx *gin.Context) {
	if c.pushManager == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "Push service not initialized",
		})
		return
	}

	siteID := ctx.Query("siteId")
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 计算偏移量
	offset := (page - 1) * pageSize

	// 获取推送日志
	logs, err := c.pushManager.GetPushLogs(siteID, pageSize, offset)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "Failed to get push logs",
		})
		return
	}

	// 获取推送日志总数
	total, err := c.pushManager.GetPushLogCount(siteID)
	if err != nil {
		total = int64(len(logs) + offset)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"list":     logs,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

// GetPushTrend 获取推送趋势
func (c *PushController) GetPushTrend(ctx *gin.Context) {
	if c.pushManager == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "Push service not initialized",
		})
		return
	}

	siteID := ctx.Query("siteId")

	if siteID == "" {
		ctx.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "success",
			"data":    []interface{}{},
		})
		return
	}

	// 获取推送趋势数据
	trend, err := c.pushManager.GetPushTrend(siteID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "Failed to get push trend",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    trend,
	})
}

// GetPushConfig 获取推送配置
func (c *PushController) GetPushConfig(ctx *gin.Context) {
	if c.pushManager == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "Push service not initialized",
		})
		return
	}

	siteID := ctx.Query("siteId")

	if siteID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "Missing siteId parameter",
		})
		return
	}

	// 站点存在性校验：不存在的站点返回 404 而非 500（与 /sites/:id 错误语义一致）
	if siteID != "" {
		siteExists := false
		if c.cfg.current() != nil {
			for _, site := range c.cfg.current().Sites {
				if site.ID == siteID {
					siteExists = true
					break
				}
			}
		}
		if !siteExists {
			ctx.JSON(http.StatusNotFound, gin.H{
				"code":    http.StatusNotFound,
				"message": "Site not found",
			})
			return
		}
	}

	// 权威读：站点配置中的推送段（pushManager 持有启动快照，对本会话新建站点会误报 site not found → 500）
	// 注：能走到这里说明上方存在性校验已确认站点存在于同一 cfg.current()，循环必然命中并返回，
	// 末尾兜底 404 不可达，故移除死分支
	for _, site := range c.cfg.current().Sites {
		if site.ID == siteID {
			pushCfg := site.Prerender.Push
			ctx.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "success",
				"data":    &pushCfg,
			})
			return
		}
	}
}

// UpdatePushConfig 更新推送配置
func (c *PushController) UpdatePushConfig(ctx *gin.Context) {
	if c.pushManager == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "Push service not initialized",
		})
		return
	}

	var req struct {
		SiteId string            `json:"siteId" binding:"required"`
		Config config.PushConfig `json:"config" binding:"required"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "Invalid request",
		})
		return
	}

	// 修复：必须在 updatePushConfigByID 改写共享配置之前读取旧 key，
	// 否则 oldKey 恒等于 newKey，旧 key 文件清理成为死逻辑
	oldKey := ""
	if cfgSnap := c.cfg.current(); cfgSnap != nil {
		for _, site := range cfgSnap.Sites {
			if site.ID == req.SiteId {
				oldKey = site.Prerender.Push.IndexNowKey
				break
			}
		}
	}

	// 同步 PushManager 内存态（供推送任务读配置；对本会话新建站点会因启动快照缺失失败，忽略）
	if c.pushManager != nil {
		_ = c.pushManager.UpdatePushConfig(req.SiteId, &req.Config)
	}

	// 权威持久化：SaveConfig 落盘 + 重启站点处理器（keyfile 拦截器即时生效）+ Redis 副本
	if c.sites == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "Sites controller not available",
		})
		return
	}
	status, message := c.sites.updatePushConfigByID(req.SiteId, req.Config)
	if status != http.StatusOK {
		ctx.JSON(status, gin.H{
			"code":    status,
			"message": message,
		})
		return
	}

	// IndexNow key file 同步落盘到站点静态根（冗余便利副本；
	// 权威来源是 site-handler 对 /{key}.txt 的拦截应答，写失败不影响配置生效）
	c.syncIndexNowKeyFile(req.SiteId, oldKey, req.Config.IndexNowKey)

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Push config updated successfully",
	})
}

// syncIndexNowKeyFile 将 IndexNow key 文件写入站点静态目录；
// oldKey != newKey 时清理旧文件（含 key 清空场景）。
func (c *PushController) syncIndexNowKeyFile(siteID, oldKey, newKey string) {
	if c.cfg.current() == nil || c.cfg.current().Dirs.StaticDir == "" {
		return
	}
	siteRoot := filepath.Join(c.cfg.current().Dirs.StaticDir, siteID)

	// 清理旧 key 文件（key 变更或清空时）
	if oldKey != "" && oldKey != newKey {
		_ = os.Remove(filepath.Join(siteRoot, oldKey+".txt"))
	}

	if newKey == "" {
		return
	}
	if err := os.MkdirAll(siteRoot, 0o755); err != nil {
		logging.DefaultLogger.Info("indexnow keyfile mkdir failed for site %s: %v", siteID, err)
		return
	}
	if err := os.WriteFile(filepath.Join(siteRoot, newKey+".txt"), []byte(newKey), 0o644); err != nil {
		logging.DefaultLogger.Info("indexnow keyfile write failed for site %s: %v", siteID, err)
	}
}
