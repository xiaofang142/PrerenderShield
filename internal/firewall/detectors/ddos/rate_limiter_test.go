package ddos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRateLimiter_New 测试创建速率限制器
func TestRateLimiter_New(t *testing.T) {
	rl := NewRateLimiter(100, 50)
	defer rl.Stop()
	assert.NotNil(t, rl)
	assert.Equal(t, 100, rl.rateThreshold)
	assert.Equal(t, 50, rl.burstThreshold)
	assert.NotNil(t, rl.ipWindows)
}

// TestRateLimiter_RecordRequest 测试记录请求
func TestRateLimiter_RecordRequest(t *testing.T) {
	rl := NewRateLimiter(100, 50)
	defer rl.Stop()

	ip := "192.168.1.1"
	rl.RecordRequest(ip)

	// 验证窗口已创建
	rl.mu.RLock()
	_, exists := rl.ipWindows[ip]
	rl.mu.RUnlock()
	assert.True(t, exists)
}

// TestRateLimiter_IsRateLimited 测试频率限制检查
func TestRateLimiter_IsRateLimited(t *testing.T) {
	rl := NewRateLimiter(5, 3)
	defer rl.Stop()

	ip := "192.168.1.2"

	// 初始不应该被限制
	assert.False(t, rl.IsRateLimited(ip))

	// 发送多个请求
	for i := 0; i < 10; i++ {
		rl.RecordRequest(ip)
	}

	// 现在应该被限制
	assert.True(t, rl.IsRateLimited(ip))
}

// TestRateLimiter_GetRequestRate 测试获取请求速率
func TestRateLimiter_GetRequestRate(t *testing.T) {
	rl := NewRateLimiter(100, 50)
	defer rl.Stop()

	ip := "192.168.1.3"

	// 初始速率为 0
	assert.Equal(t, 0, rl.GetRequestRate(ip))

	// 发送 5 个请求
	for i := 0; i < 5; i++ {
		rl.RecordRequest(ip)
	}

	// 速率应该大于 0
	rate := rl.GetRequestRate(ip)
	assert.Greater(t, rate, 0)
}

// TestRateLimiter_GetBurstCount 测试获取突发请求数
func TestRateLimiter_GetBurstCount(t *testing.T) {
	rl := NewRateLimiter(100, 10)
	defer rl.Stop()

	ip := "192.168.1.4"

	// 初始突发数为 0
	assert.Equal(t, 0, rl.GetBurstCount(ip))

	// 快速发送 5 个请求
	for i := 0; i < 5; i++ {
		rl.RecordRequest(ip)
	}

	// 突发数应该大于 0
	count := rl.GetBurstCount(ip)
	assert.Greater(t, count, 0)
}

// TestRateLimiter_Cleanup 测试清理过期数据
func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(100, 50)
	defer rl.Stop()

	ip := "192.168.1.5"
	rl.RecordRequest(ip)

	// 验证窗口已创建
	rl.mu.RLock()
	_, exists := rl.ipWindows[ip]
	rl.mu.RUnlock()
	assert.True(t, exists)

	// 验证方法存在且不 panic
	assert.NotPanics(t, func() {
		rl.CleanupExpired()
	})
}

// TestRateLimiter_GetStats 测试获取统计信息
func TestRateLimiter_GetStats(t *testing.T) {
	rl := NewRateLimiter(100, 50)
	defer rl.Stop()

	// 发送一些请求
	for i := 0; i < 5; i++ {
		ip := "192.168.1." + string(rune(i+'6'))
		rl.RecordRequest(ip)
	}

	stats := rl.GetStats()
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.TotalIPs, 5)
	assert.GreaterOrEqual(t, stats.Threshold, 100)
	assert.GreaterOrEqual(t, stats.BurstLimit, 50)
}

// TestRateLimiter_ConcurrentAccess 测试并发访问
func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	rl := NewRateLimiter(100, 50)
	defer rl.Stop()

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			ip := "192.168.2." + string(rune(id+'0'))
			for j := 0; j < 10; j++ {
				rl.RecordRequest(ip)
				rl.IsRateLimited(ip)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证没有 panic
	assert.NotNil(t, rl.ipWindows)
}

// TestSlidingWindow 测试 SlidingWindow 结构
func TestSlidingWindow(t *testing.T) {
	window := &SlidingWindow{
		requests: []time.Time{time.Now(), time.Now().Add(-100 * time.Millisecond)},
	}

	assert.NotNil(t, window)
	assert.Len(t, window.requests, 2)
}

// TestTokenBucket_New 测试创建令牌桶
func TestTokenBucket_New(t *testing.T) {
	tb := NewTokenBucket(100, 10)
	assert.NotNil(t, tb)
	assert.Equal(t, 100, tb.capacity)
	assert.Equal(t, float64(100), tb.tokens)
	assert.Equal(t, float64(10), tb.refillRate)
}

// TestTokenBucket_Allow 测试令牌桶允许请求
func TestTokenBucket_Allow(t *testing.T) {
	tb := NewTokenBucket(10, 1)

	// 初始应该允许
	assert.True(t, tb.Allow())

	// 消耗所有令牌
	for i := 0; i < 10; i++ {
		tb.Allow()
	}

	// 应该不允许
	assert.False(t, tb.Allow())
}

// TestTokenBucket_GetTokens 测试获取令牌数
func TestTokenBucket_GetTokens(t *testing.T) {
	tb := NewTokenBucket(100, 10)

	tokens := tb.GetTokens()
	assert.Greater(t, tokens, 0)
	assert.LessOrEqual(t, tokens, 100)
}

// TestTokenBucket_Refill 测试令牌补充
func TestTokenBucket_Refill(t *testing.T) {
	tb := NewTokenBucket(10, 100) // 高速补充

	// 消耗所有令牌
	for i := 0; i < 10; i++ {
		tb.Allow()
	}

	// 等待补充
	time.Sleep(50 * time.Millisecond)

	// 应该有新的令牌
	tokens := tb.GetTokens()
	assert.Greater(t, tokens, 0)
}

// TestRateLimiterStats 测试 RateLimiterStats 结构
func TestRateLimiterStats(t *testing.T) {
	stats := &RateLimiterStats{
		TotalIPs:   100,
		LimitedIPs: 10,
		Threshold:  100,
		BurstLimit: 50,
	}

	assert.Equal(t, 100, stats.TotalIPs)
	assert.Equal(t, 10, stats.LimitedIPs)
	assert.Equal(t, 100, stats.Threshold)
	assert.Equal(t, 50, stats.BurstLimit)
}
