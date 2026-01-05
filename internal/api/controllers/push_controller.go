package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/config"
	"prerender-shield/internal/prerender/push"
	"prerender-shield/internal/redis"
)

// PushController 推送控制器
type PushController struct {
	pushManager *push.PushManager
	redisClient *redis.Client
	cfg         *config.Config
}

// NewPushController 创建推送控制器实例
func NewPushController(pushManager *push.PushManager, redisClient *redis.Client, cfg *config.Config) *PushController {
	return &PushController{
		pushManager: pushManager,
		redisClient: redisClient,
		cfg:         cfg,
	}
}

// GetSites 获取站点列表
func (c *PushController) GetSites(ctx *gin.Context) {
	// 检查必要的依赖项是否可用
	if c.cfg == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "配置信息不可用",
		})
		return
	}

	// 检查站点列表是否可用
	if c.cfg.Sites == nil {
		c.cfg.Sites = []config.SiteConfig{}
	}

	var sites []gin.H
	for _, site := range c.cfg.Sites {
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
	if c.cfg == nil {
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

	// 检查站点列表是否可用
	if c.cfg.Sites == nil {
		c.cfg.Sites = []config.SiteConfig{}
	}

	siteID := ctx.Query("siteId")

	if siteID == "" {
		// 获取所有站点的统计数据
		var allStats []gin.H
		for _, site := range c.cfg.Sites {
			stats, err := c.pushManager.GetPushStats(site.ID)
			if err != nil {
				// 记录错误但不中断处理
				fmt.Printf("Failed to get push stats for site %s: %v\n", site.ID, err)
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

	// 这里需要获取总数，暂时使用一个模拟值
	total := len(logs) + offset

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
	siteID := ctx.Query("siteId")

	if siteID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "Missing siteId parameter",
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
	siteID := ctx.Query("siteId")

	if siteID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "Missing siteId parameter",
		})
		return
	}

	// 获取推送配置
	config, err := c.pushManager.GetPushConfig(siteID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "Failed to get push config",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    config,
	})
}

// UpdatePushConfig 更新推送配置
func (c *PushController) UpdatePushConfig(ctx *gin.Context) {
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

	// 更新推送配置
	if err := c.pushManager.UpdatePushConfig(req.SiteId, &req.Config); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "Failed to update push config",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Push config updated successfully",
	})
}
