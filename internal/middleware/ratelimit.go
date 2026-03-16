package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"prerender-shield/internal/redis"
)

// RateLimiter 速率限制器
type RateLimiter struct {
	redisClient *redis.Client
	mu          sync.RWMutex
	clients     map[string]*ClientRateLimit
	limit       int64                // 请求次数限制
	window      time.Duration        // 时间窗口
	banTime     time.Duration        // 封禁时长
	bannedIPs   map[string]time.Time // 被封禁的 IP
}

// ClientRateLimit 客户端速率限制信息
type ClientRateLimit struct {
	Count       int64
	WindowStart time.Time
	LastAccess  time.Time
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(redisClient *redis.Client, limit int64, window, banTime time.Duration) *RateLimiter {
	return &RateLimiter{
		redisClient: redisClient,
		clients:     make(map[string]*ClientRateLimit),
		limit:       limit,
		window:      window,
		banTime:     banTime,
		bannedIPs:   make(map[string]time.Time),
	}
}

// Middleware 速率限制中间件
func (r *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		// 检查是否被封禁
		if r.isBanned(clientIP) {
			c.AbortWithStatusJSON(429, gin.H{
				"code":        429,
				"message":     "Too many requests, please try again later",
				"retry_after": r.banTime.String(),
			})
			return
		}

		// 检查速率限制
		if !r.Allow(clientIP) {
			// 超过限制，封禁
			r.Ban(clientIP)
			c.AbortWithStatusJSON(429, gin.H{
				"code":        429,
				"message":     "Too many requests, you have been rate limited",
				"retry_after": r.banTime.String(),
			})
			return
		}

		c.Next()
	}
}

// isBanned 检查 IP 是否被封禁
func (r *RateLimiter) isBanned(ip string) bool {
	r.mu.RLock()
	banTime, exists := r.bannedIPs[ip]
	r.mu.RUnlock()

	if !exists {
		return false
	}

	// 检查封禁是否已过期
	if time.Now().After(banTime) {
		r.mu.Lock()
		delete(r.bannedIPs, ip)
		r.mu.Unlock()
		return false
	}

	return true
}

// Ban 封禁 IP
func (r *RateLimiter) Ban(ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.bannedIPs[ip] = time.Now().Add(r.banTime)
}

// Allow 检查是否允许请求
func (r *RateLimiter) Allow(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	client, exists := r.clients[ip]
	if !exists {
		// 新客户端
		r.clients[ip] = &ClientRateLimit{
			Count:       1,
			WindowStart: now,
			LastAccess:  now,
		}
		return true
	}

	// 检查时间窗口
	if now.Sub(client.WindowStart) > r.window {
		// 重置窗口
		client.Count = 1
		client.WindowStart = now
		client.LastAccess = now
		return true
	}

	// 检查是否超过限制
	if client.Count >= r.limit {
		return false
	}

	// 增加计数
	client.Count++
	client.LastAccess = now
	return true
}

// Cleanup 清理过期数据
func (r *RateLimiter) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	// 清理过期的客户端记录
	for ip, client := range r.clients {
		if now.Sub(client.LastAccess) > r.window*2 {
			delete(r.clients, ip)
		}
	}

	// 清理过期的封禁
	for ip, banTime := range r.bannedIPs {
		if now.After(banTime) {
			delete(r.bannedIPs, ip)
		}
	}
}

// StartCleanup 启动清理任务
func (r *RateLimiter) StartCleanup(interval time.Duration, stopChan <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.Cleanup()
		case <-stopChan:
			return
		}
	}
}

// GetStats 获取速率限制统计
func (r *RateLimiter) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return map[string]interface{}{
		"total_clients": len(r.clients),
		"banned_ips":    len(r.bannedIPs),
		"limit":         r.limit,
		"window":        r.window.String(),
		"ban_time":      r.banTime.String(),
	}
}

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
