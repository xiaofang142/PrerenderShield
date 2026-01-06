package controllers

import (
	"net/http"
	"prerender-shield/internal/redis"
	"time"

	"github.com/gin-gonic/gin"
)

// SystemController 系统控制器
type SystemController struct {
	redisClient *redis.Client
}

// NewSystemController 创建系统控制器实例
func NewSystemController(redisClient *redis.Client) *SystemController {
	return &SystemController{
		redisClient: redisClient,
	}
}

// Health 健康检查接口
func (c *SystemController) Health(ctx *gin.Context) {
	status := "running"
	redisStatus := "unknown"
	
	if c.redisClient != nil {
		// 检查Redis连接
		if err := c.redisClient.GetRawClient().Ping(c.redisClient.Context()).Err(); err != nil {
			redisStatus = "disconnected"
			status = "degraded"
		} else {
			redisStatus = "connected"
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"status":       status,
			"service":      "prerender-shield",
			"redis_status": redisStatus,
			"timestamp":    time.Now().Unix(),
		},
	})
}

// Version 版本信息接口
func (c *SystemController) Version(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"version": "1.0.0",
			"name":    "prerender-shield",
		},
	})
}

// GetSystemConfig 获取系统配置
func (c *SystemController) GetSystemConfig(ctx *gin.Context) {
	if c.redisClient == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Redis client not available",
		})
		return
	}

	config, err := c.redisClient.GetSystemConfig()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get system config",
		})
		return
	}

	// 如果没有配置，返回默认值
	if len(config) == 0 {
		config = map[string]string{
			"max_users":          "1",
			"allow_registration": "false",
			"maintenance_mode":   "false",
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    config,
	})
}

// UpdateSystemConfig 更新系统配置
func (c *SystemController) UpdateSystemConfig(ctx *gin.Context) {
	if c.redisClient == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Redis client not available",
		})
		return
	}

	var req map[string]interface{}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request",
		})
		return
	}

	if err := c.redisClient.SaveSystemConfig(req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to save system config",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "System config updated successfully",
	})
}
