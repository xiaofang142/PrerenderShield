package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// FirewallController 防火墙控制器
type FirewallController struct {}

// NewFirewallController 创建防火墙控制器实例
func NewFirewallController() *FirewallController {
	return &FirewallController{}
}

// GetFirewallStatus 获取防火墙状态
func (c *FirewallController) GetFirewallStatus(ctx *gin.Context) {
	// 获取防火墙状态
	site := ctx.Query("site")
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"site":    site,
			"enabled": true,
			"status":  "running",
		},
	})
}

// GetFirewallRules 获取防火墙规则
func (c *FirewallController) GetFirewallRules(ctx *gin.Context) {
	// 获取防火墙规则
	site := ctx.Query("site")
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": []gin.H{
			{
				"id":        "1",
				"site":      site,
				"name":      "Default Rule",
				"priority":  100,
				"condition": "all",
				"action":    "allow",
				"enabled":   true,
			},
		},
	})
}
