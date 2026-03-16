package controllers

import (
	"net/http"
	appConfig "prerender-shield/internal/config"
	"prerender-shield/internal/redis"
	"runtime"
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
	sslStatus := "unknown"
	expiringCerts := 0

	if c.redisClient != nil {
		// 检查 Redis 连接
		if err := c.redisClient.GetRawClient().Ping(c.redisClient.Context()).Err(); err != nil {
			redisStatus = "disconnected"
			status = "degraded"
		} else {
			redisStatus = "connected"
		}

		// 检查 SSL 证书状态
		certKeys, err := c.redisClient.SetMembers("ssl:certs")
		if err != nil {
			sslStatus = "not_configured"
		} else if len(certKeys) == 0 {
			sslStatus = "no_certificates"
		} else {
			sslStatus = "active"
			// 检查即将过期的证书
			for _, domain := range certKeys {
				certInfo := make(map[string]interface{})
				if err := c.redisClient.GetJSON("ssl:cert:"+domain, &certInfo); err == nil {
					if expiresAt, ok := certInfo["expires_at"].(float64); ok {
						expiryTime := time.Unix(int64(expiresAt), 0)
						if time.Until(expiryTime).Hours()/24 <= 30 {
							expiringCerts++
							if status == "running" {
								status = "warning"
							}
						}
					}
				}
			}
		}
	}

	// 检查系统资源使用情况
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 获取 goroutine 数量
	goroutines := runtime.NumGoroutine()

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"status":         status,
			"service":        "prerender-shield",
			"redis_status":   redisStatus,
			"ssl_status":     sslStatus,
			"expiring_certs": expiringCerts,
			"timestamp":      time.Now().Unix(),
			"health_details": gin.H{
				"memory_allocated":   m.Alloc,
				"memory_total_alloc": m.TotalAlloc,
				"memory_sys":         m.Sys,
				"num_goroutines":     goroutines,
				"gc_cycles":          m.NumGC,
			},
		},
	})
}

// Version 版本信息接口
func (c *SystemController) Version(ctx *gin.Context) {
	cfg := appConfig.GetInstance().GetConfig()
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"version":      cfg.App.Version,
			"official_url": cfg.App.OfficialURL,
			"name":         "prerender-shield",
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
			"access_log_retention_days":  "7",
			"access_log_max_size":        "128",
			"crawler_log_retention_days": "7",
			"crawler_log_max_size":       "128",
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
