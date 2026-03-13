package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestNewRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(nil, 10, 1*time.Minute, 5*time.Minute)
	assert.NotNil(t, limiter)
	assert.Equal(t, int64(10), limiter.limit)
	assert.Equal(t, 1*time.Minute, limiter.window)
	assert.Equal(t, 5*time.Minute, limiter.banTime)
	assert.NotNil(t, limiter.clients)
	assert.NotNil(t, limiter.bannedIPs)
}

func TestRateLimiter_Allow(t *testing.T) {
	limiter := NewRateLimiter(nil, 3, 1*time.Minute, 5*time.Minute)

	ip := "192.168.1.1"

	// 前 3 次请求应该允许
	assert.True(t, limiter.Allow(ip))
	assert.True(t, limiter.Allow(ip))
	assert.True(t, limiter.Allow(ip))

	// 第 4 次请求应该被拒绝
	assert.False(t, limiter.Allow(ip))
}

func TestRateLimiter_Allow_WindowReset(t *testing.T) {
	limiter := NewRateLimiter(nil, 3, 100*time.Millisecond, 5*time.Minute)

	ip := "192.168.1.2"

	// 用完配额
	limiter.Allow(ip)
	limiter.Allow(ip)
	limiter.Allow(ip)
	assert.False(t, limiter.Allow(ip))

	// 等待窗口过期
	time.Sleep(150 * time.Millisecond)

	// 应该允许新的请求
	assert.True(t, limiter.Allow(ip))
}

func TestRateLimiter_Ban(t *testing.T) {
	limiter := NewRateLimiter(nil, 3, 1*time.Minute, 5*time.Minute)

	ip := "192.168.1.3"
	limiter.Ban(ip)

	assert.True(t, limiter.isBanned(ip))
}

func TestRateLimiter_isBanned_NotBanned(t *testing.T) {
	limiter := NewRateLimiter(nil, 3, 1*time.Minute, 5*time.Minute)

	ip := "192.168.1.4"
	assert.False(t, limiter.isBanned(ip))
}

func TestRateLimiter_isBanned_Expired(t *testing.T) {
	limiter := NewRateLimiter(nil, 3, 1*time.Minute, 100*time.Millisecond)

	ip := "192.168.1.5"
	limiter.Ban(ip)

	// 等待封禁过期
	time.Sleep(150 * time.Millisecond)

	assert.False(t, limiter.isBanned(ip))
}

func TestRateLimiter_Middleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := NewRateLimiter(nil, 2, 1*time.Minute, 5*time.Minute)

	router := gin.New()
	router.Use(limiter.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	ip := "192.168.1.6"

	// 前两次请求应该成功
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", ip)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// 第三次请求应该被速率限制
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", ip)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestRateLimiter_Middleware_Banned(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := NewRateLimiter(nil, 2, 1*time.Minute, 5*time.Minute)

	// 先封禁一个 IP
	limiter.Ban("192.168.1.7")

	router := gin.New()
	router.Use(limiter.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.7")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestRateLimiter_Cleanup(t *testing.T) {
	limiter := NewRateLimiter(nil, 3, 100*time.Millisecond, 100*time.Millisecond)

	// 添加一些客户端
	limiter.Allow("192.168.1.8")
	limiter.Allow("192.168.1.9")

	// 封禁一个 IP
	limiter.Ban("192.168.1.10")

	// 等待过期
	time.Sleep(250 * time.Millisecond)

	// 清理
	limiter.Cleanup()

	stats := limiter.GetStats()
	assert.Equal(t, 0, stats["total_clients"])
	assert.Equal(t, 0, stats["banned_ips"])
}

func TestRateLimiter_GetStats(t *testing.T) {
	limiter := NewRateLimiter(nil, 10, 1*time.Minute, 5*time.Minute)

	limiter.Allow("192.168.1.11")
	limiter.Ban("192.168.1.12")

	stats := limiter.GetStats()
	assert.Equal(t, 1, stats["total_clients"])
	assert.Equal(t, 1, stats["banned_ips"])
	assert.Equal(t, int64(10), stats["limit"])
	assert.Equal(t, "1m0s", stats["window"])
	assert.Equal(t, "5m0s", stats["ban_time"])
}

func TestRateLimiter_StartCleanup(t *testing.T) {
	limiter := NewRateLimiter(nil, 3, 100*time.Millisecond, 100*time.Millisecond)

	stopChan := make(chan struct{})

	go limiter.StartCleanup(50*time.Millisecond, stopChan)

	// 添加一些数据
	limiter.Allow("192.168.1.13")

	// 等待清理
	time.Sleep(200 * time.Millisecond)

	// 停止清理协程
	close(stopChan)

	// 等待一小段时间确保协程退出
	time.Sleep(50 * time.Millisecond)
}

func TestRedisRateLimiter_New(t *testing.T) {
	limiter := NewRedisRateLimiter(nil, 10, 1*time.Minute, 5*time.Minute)
	assert.NotNil(t, limiter)
	assert.Equal(t, int64(10), limiter.limit)
	assert.Equal(t, 1*time.Minute, limiter.window)
	assert.Equal(t, 5*time.Minute, limiter.banTime)
}

// TestRedisRateLimiter_Middleware_NoBan 跳过 - 需要真实的 Redis 客户端
// func TestRedisRateLimiter_Middleware_NoBan(t *testing.T) {
// 	t.Skip("需要真实的 Redis 客户端")
// 	gin.SetMode(gin.TestMode)
// 	limiter := NewRedisRateLimiter(nil, 5, 1*time.Minute, 5*time.Minute)
//
// 	router := gin.New()
// 	router.Use(limiter.Middleware())
// 	router.GET("/test", func(c *gin.Context) {
// 		c.JSON(200, gin.H{"status": "ok"})
// 	})
//
// 	req := httptest.NewRequest(http.MethodGet, "/test", nil)
// 	req.Header.Set("X-Forwarded-For", "192.168.1.14")
// 	w := httptest.NewRecorder()
// 	router.ServeHTTP(w, req)
//
// 	assert.Equal(t, http.StatusOK, w.Code)
// }

func TestParseTime(t *testing.T) {
	now := time.Now()
	parsed := parseTime(now.Format(time.RFC3339))
	assert.Equal(t, now.Year(), parsed.Year())
	assert.Equal(t, now.Month(), parsed.Month())
	assert.Equal(t, now.Day(), parsed.Day())
}

func TestParseTime_Invalid(t *testing.T) {
	parsed := parseTime("invalid-time")
	assert.Equal(t, time.Time{}, parsed)
}

func TestClientRateLimit_Struct(t *testing.T) {
	client := &ClientRateLimit{
		Count:       5,
		WindowStart: time.Now(),
		LastAccess:  time.Now(),
	}
	assert.Equal(t, int64(5), client.Count)
	assert.NotNil(t, client.WindowStart)
	assert.NotNil(t, client.LastAccess)
}
