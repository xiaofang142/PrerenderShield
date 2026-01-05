package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SystemController 系统控制器
type SystemController struct{}

// NewSystemController 创建系统控制器实例
func NewSystemController() *SystemController {
	return &SystemController{}
}

// Health 健康检查接口
func (c *SystemController) Health(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"status":  "running",
			"service": "prerender-shield",
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
