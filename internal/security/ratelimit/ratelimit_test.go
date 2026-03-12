package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultRateLimiterConfig(t *testing.T) {
	config := DefaultRateLimiterConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 1000, config.RequestsPerSecond)
	assert.Equal(t, 100, config.IPRequestsPerSecond)
	assert.Equal(t, 50, config.UserRequestsPerSecond)
	assert.Contains(t, config.EndpointLimits, "/api/login")
}

func TestNewRateLimiter(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultRateLimiterConfig()

	limiter := NewRateLimiter(config, logger)

	assert.NotNil(t, limiter)
	assert.NotNil(t, limiter.buckets)
	assert.NotNil(t, limiter.stats)
}

func TestRateLimiter_Allow(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &RateLimiterConfig{
		IPRequestsPerSecond:   10,
		IPBurstSize:           20,
		UserRequestsPerSecond: 5,
		UserBurstSize:         10,
		BucketTimeout:         1 * time.Minute,
		CleanupInterval:       30 * time.Second,
	}

	limiter := NewRateLimiter(config, logger)

	ctx := context.Background()

	// 前 10 个请求应该允许
	for i := 0; i < 10; i++ {
		result := limiter.Allow(ctx, "192.168.1.1", "user1", "/api/test", "GET")
		assert.True(t, result.Allowed, "请求 %d 应该被允许", i)
	}

	// 第 11 个请求可能被限制（取决于 IP 限制）
	result := limiter.Allow(ctx, "192.168.1.1", "user1", "/api/test", "GET")
	// 由于 burst size，可能仍然允许
	_ = result
}

func TestRateLimiter_EndpointLimit(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &RateLimiterConfig{
		IPRequestsPerSecond: 100,
		IPBurstSize:         200,
		EndpointLimits: map[string]*EndpointLimit{
			"/api/login": {RequestsPerSecond: 2, BurstSize: 3, Method: "POST"},
		},
		BucketTimeout:   1 * time.Minute,
		CleanupInterval: 30 * time.Second,
	}

	limiter := NewRateLimiter(config, logger)
	ctx := context.Background()

	// 登录端点限制为 2 请求/秒，burst 3
	for i := 0; i < 3; i++ {
		result := limiter.Allow(ctx, "192.168.1.2", "", "/api/login", "POST")
		assert.True(t, result.Allowed, "登录请求 %d 应该被允许", i)
	}

	// 第 4 个请求应该被限制
	result := limiter.Allow(ctx, "192.168.1.2", "", "/api/login", "POST")
	assert.False(t, result.Allowed, "登录请求应该被限制")
	assert.Equal(t, "endpoint_limit", result.Reason)
}

func TestRateLimiter_UserLimit(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &RateLimiterConfig{
		IPRequestsPerSecond:   100,
		IPBurstSize:           200,
		UserRequestsPerSecond: 3,
		UserBurstSize:         5,
		BucketTimeout:         1 * time.Minute,
		CleanupInterval:       30 * time.Second,
	}

	limiter := NewRateLimiter(config, logger)
	ctx := context.Background()

	// 同一用户的请求
	for i := 0; i < 5; i++ {
		result := limiter.Allow(ctx, "192.168.1.3", "user_limited", "/api/test", "GET")
		if i < 5 {
			assert.True(t, result.Allowed, "用户请求 %d 应该被允许", i)
		}
	}

	// 超出用户限制
	result := limiter.Allow(ctx, "192.168.1.3", "user_limited", "/api/test", "GET")
	assert.False(t, result.Allowed)
	assert.Equal(t, "user_limit", result.Reason)
}

func TestTokenBucket_Allow(t *testing.T) {
	// 创建桶：最大 10 个令牌，每秒补充 5 个
	bucket := NewTokenBucket(10, 5)

	// 前 10 次应该允许
	for i := 0; i < 10; i++ {
		allowed, remaining := bucket.Allow()
		assert.True(t, allowed)
		assert.Equal(t, 9-i, remaining)
	}

	// 第 11 次应该拒绝
	allowed, remaining := bucket.Allow()
	assert.False(t, allowed)
	assert.Equal(t, 0, remaining)
}

func TestTokenBucket_Refill(t *testing.T) {
	// 创建桶：最大 5 个令牌，每秒补充 10 个
	bucket := NewTokenBucket(5, 10)

	// 消耗所有令牌
	for i := 0; i < 5; i++ {
		bucket.Allow()
	}

	// 等待补充
	time.Sleep(200 * time.Millisecond)

	// 应该有一些令牌了
	allowed, _ := bucket.Allow()
	assert.True(t, allowed)
}

func TestRateLimiter_GetStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultRateLimiterConfig()
	limiter := NewRateLimiter(config, logger)

	ctx := context.Background()

	// 发送一些请求
	for i := 0; i < 5; i++ {
		limiter.Allow(ctx, "192.168.1.100", "", "/api/test", "GET")
	}

	stats := limiter.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, int64(5), stats.TotalRequests)
	assert.GreaterOrEqual(t, stats.ActiveBuckets, int64(1))
}

func TestRateLimiter_Reset(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultRateLimiterConfig()
	limiter := NewRateLimiter(config, logger)

	ctx := context.Background()

	// 发送一些请求
	for i := 0; i < 5; i++ {
		limiter.Allow(ctx, "192.168.1.100", "", "/api/test", "GET")
	}

	// 重置
	limiter.Reset()

	stats := limiter.GetStats()
	assert.Equal(t, int64(0), stats.TotalRequests)
	assert.Equal(t, int64(0), stats.ActiveBuckets)
}

func TestRateLimiter_ResetKey(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultRateLimiterConfig()
	limiter := NewRateLimiter(config, logger)

	ctx := context.Background()

	// 发送一些请求
	limiter.Allow(ctx, "192.168.1.100", "", "/api/test", "GET")

	// 重置特定键
	limiter.ResetKey("ip:192.168.1.100")

	// 再次请求应该允许（因为桶被重置了）
	result := limiter.Allow(ctx, "192.168.1.100", "", "/api/test", "GET")
	assert.True(t, result.Allowed)
}

func TestRateLimiter_SetEndpointLimit(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultRateLimiterConfig()
	limiter := NewRateLimiter(config, logger)

	// 设置新的端点限制
	limit := &EndpointLimit{
		RequestsPerSecond: 10,
		BurstSize:         20,
		Method:            "GET",
	}
	limiter.SetEndpointLimit("/api/new", limit)

	ctx := context.Background()
	result := limiter.Allow(ctx, "192.168.1.1", "", "/api/new", "GET")
	assert.NotNil(t, result)
}

func TestRateLimiter_RemoveEndpointLimit(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultRateLimiterConfig()
	limiter := NewRateLimiter(config, logger)

	// 移除现有端点限制
	limiter.RemoveEndpointLimit("/api/login")

	// 登录端点不再有限制
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		result := limiter.Allow(ctx, "192.168.1.1", "", "/api/login", "POST")
		assert.True(t, result.Allowed) // 只受 IP 限制
	}
}

func TestLimitResult(t *testing.T) {
	result := &LimitResult{
		Allowed:    false,
		Remaining:  0,
		ResetAfter: 60,
		RetryAfter: 10,
		Reason:     "ip_limit",
	}

	assert.False(t, result.Allowed)
	assert.Equal(t, 0, result.Remaining)
	assert.Equal(t, int64(60), result.ResetAfter)
	assert.Equal(t, int64(10), result.RetryAfter)
	assert.Equal(t, "ip_limit", result.Reason)
}
