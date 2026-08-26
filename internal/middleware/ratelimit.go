package middleware

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"

	"prerender-shield/internal/logging"
	pkgredis "prerender-shield/internal/redis"
)

// RedisRateLimiter 基于 Redis 的分布式速率限制器
type RedisRateLimiter struct {
	redisClient *pkgredis.Client
	limit       int64
	window      time.Duration
	banTime     time.Duration
}

// NewRedisRateLimiter 创建基于 Redis 的速率限制器
func NewRedisRateLimiter(redisClient *pkgredis.Client, limit int64, window, banTime time.Duration) *RedisRateLimiter {
	return &RedisRateLimiter{
		redisClient: redisClient,
		limit:       limit,
		window:      window,
		banTime:     banTime,
	}
}

// rateLimitLuaScript 原子性 INCR + EXPIRE，避免竞态条件
var rateLimitLuaScript = redis.NewScript(`
local current = redis.call('INCR', KEYS[1])
if current == 1 then
    redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return current
`)

// Middleware 速率限制中间件（分布式）
func (r *RedisRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		// 检查是否被封禁（Get 内部已将 redis.Nil 转为空串返回，此处 err 必为真实错误）
		bannedKey := "ratelimit:ban:" + clientIP
		isBanned, err := r.redisClient.Get(bannedKey)
		if err != nil {
			logging.DefaultLogger.Warn("Rate limit ban check error for %s: %v", clientIP, err)
		}
		if isBanned != "" {
			c.AbortWithStatusJSON(429, gin.H{
				"code":    429,
				"message": "Too many requests, please try again later",
			})
			return
		}

		// 原子滑动窗口（Lua 脚本保证原子性）
		counterKey := "ratelimit:count:" + clientIP + ":" + time.Now().Truncate(r.window).Format("200601021504")

		rdb := r.redisClient.GetRawClient()
		ctx := r.redisClient.Context()

		count, err := rateLimitLuaScript.Run(ctx, rdb, []string{counterKey}, int(r.window.Seconds())).Int64()
		if err != nil {
			logging.DefaultLogger.Warn("Rate limit counter error for %s: %v", clientIP, err)
			c.Next()
			return
		}

		if count >= r.limit {
			banCtx := context.Background()
			rdb.Set(banCtx, bannedKey, "1", r.banTime)
			c.AbortWithStatusJSON(429, gin.H{
				"code":    429,
				"message": "Too many requests, you have been rate limited",
			})
			return
		}

		c.Next()
	}
}

// ManagementRateLimit 管理 API 速率限制中间件
// 仅对 /api/v1 下的管理端点应用限流，公开端点（health/version/login/first-run）豁免
func ManagementRateLimit(limiter *RedisRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if len(path) >= 7 && path[:7] == "/api/v1" &&
			path != "/api/v1/health" &&
			path != "/api/v1/version" &&
			!strings.HasPrefix(path, "/api/v1/auth/login") &&
			!strings.HasPrefix(path, "/api/v1/auth/first-run") {
			limiter.Middleware()(c)
			return
		}
		c.Next()
	}
}
