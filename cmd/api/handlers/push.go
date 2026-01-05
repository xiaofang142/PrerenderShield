package handlers

import (
	"net/http"
	"strconv"

	"prerender-shield/internal/config"
	"prerender-shield/internal/prerender/push"
	"prerender-shield/internal/redis"

	"github.com/gin-gonic/gin"
)

// PushHandler 推送API处理器
type PushHandler struct {
	pushManager *push.PushManager
	redisClient *redis.Client
	config      *config.Config
}

// NewPushHandler 创建新的推送API处理器
func NewPushHandler(pushManager *push.PushManager, redisClient *redis.Client, config *config.Config) *PushHandler {
	return &PushHandler{
		pushManager: pushManager,
		redisClient: redisClient,
		config:      config,
	}
}

// RegisterRoutes 注册推送相关路由
func (h *PushHandler) RegisterRoutes(router *gin.RouterGroup) {
	pushGroup := router.Group("/push")
	{
		// 获取站点列表
		pushGroup.GET("/sites", h.GetSites)

		// 获取推送统计数据
		pushGroup.GET("/stats", h.GetPushStats)

		// 获取推送日志
		pushGroup.GET("/logs", h.GetPushLogs)

		// 手动触发推送
		pushGroup.POST("/trigger", h.TriggerPush)

		// 获取推送配置
		pushGroup.GET("/config", h.GetPushConfig)

		// 更新推送配置
		pushGroup.POST("/config", h.UpdatePushConfig)
	}
}

// GetSites 获取站点列表
func (h *PushHandler) GetSites(c *gin.Context) {
	var sites []gin.H
	for _, site := range h.config.Sites {
		sites = append(sites, gin.H{
			"id":      site.ID,
			"name":    site.Name,
			"domain":  site.Domains[0],
			"enabled": site.Prerender.Push.Enabled,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    sites,
	})
}

// GetPushStats 获取推送统计数据
func (h *PushHandler) GetPushStats(c *gin.Context) {
	siteID := c.Query("siteId")

	if siteID == "" {
		// 获取所有站点的统计数据
		var allStats []gin.H
		for _, site := range h.config.Sites {
			stats, err := h.pushManager.GetPushStats(site.ID)
			if err != nil {
				continue
			}

			allStats = append(allStats, gin.H{
				"siteId":  site.ID,
				"siteName": site.Name,
				"stats":   stats,
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
	stats, err := h.pushManager.GetPushStats(siteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "Failed to get push stats",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"siteId": siteID,
			"stats":  stats,
		},
	})
}

// GetPushLogs 获取推送日志
func (h *PushHandler) GetPushLogs(c *gin.Context) {
	siteID := c.Query("siteId")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 计算偏移量
	offset := (page - 1) * pageSize

	// 获取推送日志
	logs, err := h.pushManager.GetPushLogs(siteID, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "Failed to get push logs",
		})
		return
	}

	// 这里需要获取总数，暂时使用一个模拟值
	total := len(logs) + offset

	c.JSON(http.StatusOK, gin.H{
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

// TriggerPush 手动触发推送
func (h *PushHandler) TriggerPush(c *gin.Context) {
	var req struct {
		SiteID string `json:"siteId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "Invalid request",
		})
		return
	}

	// 触发推送，获取任务ID
	taskID, err := h.pushManager.TriggerPush(req.SiteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Push triggered successfully",
		"data": gin.H{
			"taskId": taskID,
		},
	})
}

// GetPushConfig 获取推送配置
func (h *PushHandler) GetPushConfig(c *gin.Context) {
	siteID := c.Query("siteId")

	if siteID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "Missing siteId parameter",
		})
		return
	}

	// 获取推送配置
	config, err := h.pushManager.GetPushConfig(siteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "Failed to get push config",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    config,
	})
}

// UpdatePushConfig 更新推送配置
func (h *PushHandler) UpdatePushConfig(c *gin.Context) {
	var req struct {
		SiteID string           `json:"siteId" binding:"required"`
		Config config.PushConfig `json:"config" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "Invalid request",
		})
		return
	}

	// 更新推送配置
	if err := h.pushManager.UpdatePushConfig(req.SiteID, &req.Config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "Failed to update push config",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Push config updated successfully",
	})
}
