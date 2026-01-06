package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"prerender-shield/internal/models"
	"prerender-shield/internal/repository"
)

// FirewallController handles WAF configuration requests
type FirewallController struct {
	wafRepo *repository.WafRepository
}

// NewFirewallController creates a new FirewallController
func NewFirewallController(wafRepo *repository.WafRepository) *FirewallController {
	return &FirewallController{
		wafRepo: wafRepo,
	}
}

// GetWafConfig returns the WAF configuration for a site
func (c *FirewallController) GetWafConfig(ctx *gin.Context) {
	siteID := ctx.Param("id")
	if siteID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Site ID is required"})
		return
	}

	config, err := c.wafRepo.GetWafConfigBySiteID(siteID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get WAF config"})
		return
	}

	if config == nil {
		// Return empty default or 404? 
		// Return a default structure if not found, or create one on the fly.
		// For now return empty object
		ctx.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    gin.H{},
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// UpdateWafConfig updates the WAF configuration for a site
func (c *FirewallController) UpdateWafConfig(ctx *gin.Context) {
	siteID := ctx.Param("id")
	if siteID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Site ID is required"})
		return
	}

	var req struct {
		RateLimitCount   int      `json:"rate_limit_count"`
		RateLimitWindow  int      `json:"rate_limit_window"`
		BlockedCountries []string `json:"blocked_countries"`
		WhitelistIPs     []string `json:"whitelist_ips"`
		BlacklistIPs     []string `json:"blacklist_ips"`
		CustomBlockPage  string   `json:"custom_block_page"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. Get or Create WafConfig
	config, err := c.wafRepo.GetWafConfigBySiteID(siteID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing config"})
		return
	}

	if config == nil {
		config = &models.WafConfig{
			SiteID: siteID,
		}
		if err := c.wafRepo.CreateWafConfig(config); err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create WAF config"})
			return
		}
	}

	// 2. Update fields
	config.RateLimitCount = req.RateLimitCount
	config.RateLimitWindow = req.RateLimitWindow
	config.CustomBlockPage = req.CustomBlockPage
	
	if err := c.wafRepo.UpdateWafConfig(config); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update WAF config"})
		return
	}

	// 3. Update relations
	if err := c.wafRepo.UpdateBlockedCountries(config.ID, req.BlockedCountries); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update blocked countries"})
		return
	}
	if err := c.wafRepo.UpdateIPWhitelist(config.ID, req.WhitelistIPs); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update whitelist IPs"})
		return
	}
	if err := c.wafRepo.UpdateIPBlacklist(config.ID, req.BlacklistIPs); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update blacklist IPs"})
		return
	}

	// Refetch to return full object
	updatedConfig, _ := c.wafRepo.GetWafConfigBySiteID(siteID)

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updatedConfig,
	})
}

// GetAccessLogs returns access logs
func (c *FirewallController) GetAccessLogs(ctx *gin.Context) {
	siteID := ctx.Query("site_id")
	pageStr := ctx.DefaultQuery("page", "1")
	limitStr := ctx.DefaultQuery("limit", "20")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	logs, total, err := c.wafRepo.GetAccessLogs(siteID, page, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get logs"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"logs":  logs,
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}
