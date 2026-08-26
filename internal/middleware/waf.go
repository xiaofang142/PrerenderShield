package middleware

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"

	"prerender-shield/internal/config"
	"prerender-shield/internal/firewall"
	"prerender-shield/internal/firewall/types"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/models"
	pkgredis "prerender-shield/internal/redis"
	"prerender-shield/internal/repository"
	"prerender-shield/internal/services"
)

// WafMiddleware implements the Web Application Firewall logic
func WafMiddleware(site config.SiteConfig, wafRepo *repository.WafRepository, redisClient *pkgredis.Client, geoIP services.GeoIPResolver, wafEngine *firewall.Engine, logWriter *WafLogWriter) gin.HandlerFunc {
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
		startTime := time.Now()

		// Helper to log and block
		block := func(reason, ruleID string, threat *types.Threat) {
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

			// 异步批量写入日志
			if logWriter != nil {
				logWriter.Write(log)
			} else if wafRepo != nil {
				wafRepo.CreateAccessLog(&log)
			}

			// 记录请求持续时间
			duration := time.Since(startTime).Milliseconds()

			// 返回响应
			c.JSON(http.StatusForbidden, gin.H{
				"code":       403,
				"message":    site.Firewall.ActionConfig.BlockMessage,
				"reason":     reason,
				"request_id": requestID,
			})
			c.Abort()

			// 输出日志
			logging.DefaultLogger.Info("[WAF Blocked] [%dms] %s %s from %s - %s (Rule: %s)\n",
				duration, method, requestPath, clientIP, reason, ruleID)
		}

		// 1. Whitelist Check - 白名单直接放行
		for _, ip := range site.Firewall.Whitelist {
			if ip == clientIP {
				c.Next()
				return
			}
		}

		// 2. Blacklist Check - 黑名单直接阻断
		for _, ip := range site.Firewall.Blacklist {
			if ip == clientIP {
				block("IP is in blacklist", "ip_blacklist", nil)
				return
			}
		}

		// 3. GeoIP Check - 地理位置检查
		if site.Firewall.GeoIPConfig.Enabled && geoIP != nil {
			countryCode, err := geoIP.LookupCountryISO(clientIP)
			if err == nil && countryCode != "" {
				// Check BlockList
				for _, blockedCode := range site.Firewall.GeoIPConfig.BlockList {
					if blockedCode == countryCode {
						block("Country is blocked: "+countryCode, "geoip_block", nil)
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
						block("Country not in allow list: "+countryCode, "geoip_allow", nil)
						return
					}
				}
			}
		}

		// 4. Path Traversal Detection - 路径遍历检测
		if isPathTraversal(requestPath) || isPathTraversal(userAgent) {
			block("Path traversal detected", "path_traversal", nil)
			return
		}

		// 5. Rate Limiting - 频率限制（原子操作，使用 Lua 脚本避免竞态）
		if site.Firewall.RateLimitConfig.Enabled && redisClient != nil {
			limit := site.Firewall.RateLimitConfig.Requests
			window := site.Firewall.RateLimitConfig.Window

			key := fmt.Sprintf("ratelimit:%s:%s", site.ID, clientIP)
			rdb := redisClient.GetRawClient()
			ctx := redisClient.Context()

			count, err := atomicIncrWithExpire(ctx, rdb, key, time.Duration(window)*time.Second)
			if err == nil && int(count) > limit {
				block("Rate limit exceeded", "rate_limit", nil)
				return
			}
		}

		// 5. OWASP Content Detection - 内容威胁检测
		if wafEngine != nil {
			result, err := wafEngine.CheckRequest(c.Request)
			if err == nil && result != nil && !result.Allow {
				for _, t := range result.Threats {
					if t.Severity == "high" || t.Severity == "critical" || t.Severity == "" {
						block(fmt.Sprintf("OWASP threat: %s - %s", t.Type, t.Message), t.RuleID, &t)
						return
					}
				}
			}
		}

		// If passed all checks - 通过所有检查，放行
		c.Next()
	}
}

// pathTraversalPatterns 路径遍历检测正则
var pathTraversalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\.\./|\.\.\\`),
	regexp.MustCompile(`%2e%2e%2f|%2e%2e/|%2e%2e%5c`),
	regexp.MustCompile(`\.\.%2f|\.\.%5c`),
	regexp.MustCompile(`%252e%252e`),
	regexp.MustCompile(`\.\./\.\./`),
}

// sensitivePatterns 敏感文件路径
var sensitivePatterns = []string{
	"/etc/passwd", "/etc/shadow", "/etc/hosts",
	"/proc/self", "/proc/version",
	"web.config", ".env", ".git/",
	"/win.ini", "/winnt/system32",
}

// atomicIncrWithExpire 使用 Lua 脚本原子性地执行 INCR + EXPIRE，避免竞态条件
var rateLimitScript = redis.NewScript(`
local current = redis.call('INCR', KEYS[1])
if current == 1 then
    redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return current
`)

func atomicIncrWithExpire(ctx context.Context, rdb *redis.Client, key string, ttl time.Duration) (int64, error) {
	result, err := rateLimitScript.Run(ctx, rdb, []string{key}, int(ttl.Seconds())).Int64()
	return result, err
}

// isPathTraversal 检测路径遍历攻击
func isPathTraversal(path string) bool {
	if path == "" {
		return false
	}
	lower := strings.ToLower(path)
	for _, re := range pathTraversalPatterns {
		if re.MatchString(lower) {
			return true
		}
	}
	for _, sensitive := range sensitivePatterns {
		if strings.Contains(lower, sensitive) {
			return true
		}
	}
	return false
}
