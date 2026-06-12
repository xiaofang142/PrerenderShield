package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"prerender-shield/internal/redis"
)

// RedisRateLimiter 基于 Redis 的分布式速率限制器
type RedisRateLimiter struct {
	redisClient *redis.Client
	limit       int64
	window      time.Duration
	banTime     time.Duration
}

// NewRedisRateLimiter 创建基于 Redis 的速率限制器
func NewRedisRateLimiter(redisClient *redis.Client, limit int64, window, banTime time.Duration) *RedisRateLimiter {
	return &RedisRateLimiter{
		redisClient: redisClient,
		limit:       limit,
		window:      window,
		banTime:     banTime,
	}
}

// Middleware 速率限制中间件（分布式）
func (r *RedisRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		// 检查是否被封禁
		bannedKey := "ratelimit:ban:" + clientIP
		isBanned, _ := r.redisClient.Get(bannedKey)
		if isBanned != "" {
			c.AbortWithStatusJSON(429, gin.H{
				"code":    429,
				"message": "Too many requests, please try again later",
			})
			return
		}

		// 使用滑动窗口计数
		counterKey := "ratelimit:count:" + clientIP
		windowKey := "ratelimit:window:" + clientIP

		// 获取当前窗口开始时间
		windowStart, _ := r.redisClient.Get(windowKey)

		now := time.Now()
		if windowStart == "" || now.Sub(parseTime(windowStart)) > r.window {
			// 新窗口
			r.redisClient.Set(windowKey, now.Format(time.RFC3339), r.window)
			r.redisClient.Set(counterKey, 1, r.window)
		} else {
			// 当前窗口内计数
			count, _ := r.redisClient.Incr(counterKey)
			if count >= r.limit {
				// 超过限制，封禁
				r.redisClient.Set(bannedKey, "1", r.banTime)
				c.AbortWithStatusJSON(429, gin.H{
					"code":    429,
					"message": "Too many requests, you have been rate limited",
				})
				return
			}
		}

		c.Next()
	}
}

// parseTime 解析时间字符串
func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
