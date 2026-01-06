package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"prerender-shield/internal/config"
	"prerender-shield/internal/models"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/repository"
	"prerender-shield/internal/services"
)

// WafMiddleware implements the Web Application Firewall logic
func WafMiddleware(site config.SiteConfig, wafRepo *repository.WafRepository, redisClient *redis.Client, geoIP services.GeoIPResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !site.Firewall.Enabled {
			c.Next()
			return
		}

		clientIP := c.ClientIP()
		requestPath := c.Request.URL.Path
		userAgent := c.Request.UserAgent()
		method := c.Request.Method
		requestID := uuid.New().String()

		// Helper to log and block
		block := func(reason, ruleID string) {
			// Log to DB
			log := models.AccessLog{
				ID:          uuid.New().String(),
				SiteID:      site.ID,
				RequestID:   requestID,
				IPAddress:   clientIP,
				Method:      method,
				RequestPath: requestPath,
				UserAgent:   userAgent,
				StatusCode:  403,
				Action:      "block",
				RuleID:      ruleID,
				Reason:      reason,
				CreatedAt:   time.Now(),
			}
			// Use a goroutine to avoid blocking the response
			go func() {
				if wafRepo != nil {
					if err := wafRepo.CreateAccessLog(&log); err != nil {
						fmt.Printf("Failed to create access log: %v\n", err)
					}
				}
			}()

			// Return response
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": site.Firewall.ActionConfig.BlockMessage,
				"reason":  reason,
			})
			c.Abort()
		}

		// 1. Whitelist Check
		for _, ip := range site.Firewall.Whitelist {
			if ip == clientIP {
				// Allowed, skip other checks
				c.Next()
				return
			}
		}

		// 2. Blacklist Check
		for _, ip := range site.Firewall.Blacklist {
			if ip == clientIP {
				block("IP is in blacklist", "ip_blacklist")
				return
			}
		}

		// 3. GeoIP Check
		if site.Firewall.GeoIPConfig.Enabled && geoIP != nil {
			countryCode, err := geoIP.LookupCountryISO(clientIP)
			if err == nil && countryCode != "" {
				// Check BlockList
				for _, blockedCode := range site.Firewall.GeoIPConfig.BlockList {
					if blockedCode == countryCode {
						block("Country is blocked: "+countryCode, "geoip_block")
						return
					}
				}

				// Check AllowList (only if configured)
				if len(site.Firewall.GeoIPConfig.AllowList) > 0 {
					allowed := false
					for _, allowedCode := range site.Firewall.GeoIPConfig.AllowList {
						if allowedCode == countryCode {
							allowed = true
							break
						}
					}
					if !allowed {
						block("Country not in allow list: "+countryCode, "geoip_allow")
						return
					}
				}
			}
		}

		// 4. Rate Limiting
		if site.Firewall.RateLimitConfig.Enabled && redisClient != nil {
			limit := site.Firewall.RateLimitConfig.Requests
			window := site.Firewall.RateLimitConfig.Window
			// banTime := site.Firewall.RateLimitConfig.BanTime

			key := fmt.Sprintf("ratelimit:%s:%s", site.ID, clientIP)

			// Simple counter implementation
			// In production, use a sliding window or token bucket
			// Use GetRawClient() to access underlying redis client methods
			rdb := redisClient.GetRawClient()
			ctx := redisClient.Context()

			count, err := rdb.Incr(ctx, key).Result()
			if err == nil {
				if count == 1 {
					rdb.Expire(ctx, key, time.Duration(window)*time.Second)
				}
				if int(count) > limit {
					block("Rate limit exceeded", "rate_limit")
					return
				}
			}
		}

		// If passed all checks
		c.Next()
	}
}
