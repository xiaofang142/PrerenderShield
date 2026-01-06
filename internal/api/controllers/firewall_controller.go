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
		Enabled          bool     `json:"enabled"`
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
		// Create immediately to ensure ID is generated if needed, 
		// but UpdateWafConfig below will save it anyway.
	}

	// 2. Update fields
	config.Enabled = req.Enabled
	config.RateLimitCount = req.RateLimitCount
	config.RateLimitWindow = req.RateLimitWindow
	config.CustomBlockPage = req.CustomBlockPage

	// Update Relations
	// Blocked Countries
	var blockedCountries []models.BlockedCountry
	for _, code := range req.BlockedCountries {
		blockedCountries = append(blockedCountries, models.BlockedCountry{
			CountryCode: code,
			WafConfigID: config.ID,
		})
	}
	config.BlockedCountries = blockedCountries

	// Whitelist IPs
	var whitelistIPs []models.IPWhitelist
	for _, ip := range req.WhitelistIPs {
		whitelistIPs = append(whitelistIPs, models.IPWhitelist{
			IPAddress:   ip,
			WafConfigID: config.ID,
		})
	}
	config.IPWhitelist = whitelistIPs

	// Blacklist IPs
	var blacklistIPs []models.IPBlacklist
	for _, ip := range req.BlacklistIPs {
		blacklistIPs = append(blacklistIPs, models.IPBlacklist{
			IPAddress:   ip,
			WafConfigID: config.ID,
		})
	}
	config.IPBlacklist = blacklistIPs
	
	if err := c.wafRepo.UpdateWafConfig(config); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update WAF config"})
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

// GetAttackLogs returns attack logs
func (c *FirewallController) GetAttackLogs(ctx *gin.Context) {
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

	logs, total, err := c.wafRepo.GetAttackLogs(siteID, page, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get attack logs"})
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

// AddToWhitelist adds an IP to the whitelist
func (c *FirewallController) AddToWhitelist(ctx *gin.Context) {
	var req struct {
		SiteID string `json:"site_id"`
		IP     string `json:"ip"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.SiteID == "" || req.IP == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Site ID and IP are required"})
		return
	}

	if err := c.wafRepo.AddIPToWhitelist(req.SiteID, req.IP); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add to whitelist"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"success": true})
}

// AddToBlacklist adds an IP to the blacklist
func (c *FirewallController) AddToBlacklist(ctx *gin.Context) {
	var req struct {
		SiteID string `json:"site_id"`
		IP     string `json:"ip"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.SiteID == "" || req.IP == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Site ID and IP are required"})
		return
	}

	if err := c.wafRepo.AddIPToBlacklist(req.SiteID, req.IP); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add to blacklist"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"success": true})
}
