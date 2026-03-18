package controllers

import (
	"context"
	"net/http"
	appConfig "prerender-shield/internal/config"
	"prerender-shield/internal/redis"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	redisV8 "github.com/go-redis/redis/v8"
)

// SystemRedisClient defines the interface for system Redis operations
type SystemRedisClient interface {
	Context() context.Context
	GetRawClient() (RawRedisClient, error)
	SetMembers(key string) ([]string, error)
	GetJSON(key string, value interface{}) error
	GetSystemConfig() (map[string]string, error)
	SaveSystemConfig(config map[string]interface{}) error
}

// RawRedisClient defines the interface for raw Redis client operations
type RawRedisClient interface {
	Ping(ctx context.Context) RedisStatus
}

// RedisStatus defines the interface for Redis ping status
type RedisStatus interface {
	Err() error
}

// systemRedisWrapper wraps redis.Client to implement SystemRedisClient
type systemRedisWrapper struct {
	client *redis.Client
}

func (w *systemRedisWrapper) Context() context.Context {
	return w.client.Context()
}

type rawRedisWrapper struct {
	client *redisV8.Client
}

func (w *rawRedisWrapper) Ping(ctx context.Context) RedisStatus {
	return w.client.Ping(ctx)
}

func (w *systemRedisWrapper) GetRawClient() (RawRedisClient, error) {
	return &rawRedisWrapper{client: w.client.GetRawClient()}, nil
}

func (w *systemRedisWrapper) SetMembers(key string) ([]string, error) {
	return w.client.SetMembers(key)
}

func (w *systemRedisWrapper) GetJSON(key string, value interface{}) error {
	return w.client.GetJSON(key, value)
}

func (w *systemRedisWrapper) GetSystemConfig() (map[string]string, error) {
	return w.client.GetSystemConfig()
}

func (w *systemRedisWrapper) SaveSystemConfig(config map[string]interface{}) error {
	return w.client.SaveSystemConfig(config)
}

// SystemController 系统控制器
type SystemController struct {
	redisClient SystemRedisClient
}

// NewSystemController 创建系统控制器实例
func NewSystemController(redisClient *redis.Client) *SystemController {
	var wrapper SystemRedisClient
	if redisClient != nil {
		wrapper = &systemRedisWrapper{client: redisClient}
	}
	return &SystemController{
		redisClient: wrapper,
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
		rawClient, err := c.redisClient.GetRawClient()
		if err == nil && rawClient != nil {
			if pingErr := rawClient.Ping(c.redisClient.Context()).Err(); pingErr != nil {
				redisStatus = "disconnected"
				status = "degraded"
			} else {
				redisStatus = "connected"
			}
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
