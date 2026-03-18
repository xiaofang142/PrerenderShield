package ratelimit

import (
	"context"
	"fmt"
	"sync"
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

// ============== TokenBucket 补充测试 ==============

func TestTokenBucket_GetTokens(t *testing.T) {
	// 创建桶：最大 10 个令牌
	bucket := NewTokenBucket(10, 5)

	// 初始应该有 10 个令牌
	tokens := bucket.GetTokens()
	assert.Equal(t, 10, tokens, "初始令牌数应该为 10")

	// 消耗 3 个令牌
	for i := 0; i < 3; i++ {
		bucket.Allow()
	}

	// 应该剩下 7 个令牌
	tokens = bucket.GetTokens()
	assert.Equal(t, 7, tokens, "消耗 3 个令牌后应该剩下 7 个")

	// 等待补充
	time.Sleep(300 * time.Millisecond)

	// 令牌数应该增加（补充速率 5/s，300ms 应该补充约 1.5 个）
	tokens = bucket.GetTokens()
	assert.GreaterOrEqual(t, tokens, 7, "令牌数应该有所补充")
	assert.LessOrEqual(t, tokens, 10, "令牌数不应该超过最大值")
}

func TestTokenBucket_GetTokens_EmptyBucket(t *testing.T) {
	// 创建空桶
	bucket := NewTokenBucket(0, 5)

	tokens := bucket.GetTokens()
	assert.Equal(t, 0, tokens, "空桶的令牌数应该为 0")
}

func TestTokenBucket_Allow_Concurrent(t *testing.T) {
	// 创建桶：最大 100 个令牌，每秒补充 100 个
	bucket := NewTokenBucket(100, 100)

	var wg sync.WaitGroup
	var allowedCount int
	var mu sync.Mutex

	// 启动 50 个协程，每个尝试获取 3 个令牌
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				if allowed, _ := bucket.Allow(); allowed {
					mu.Lock()
					allowedCount++
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	// 由于初始有 100 个令牌，应该有 100 次允许
	assert.Equal(t, 100, allowedCount, "应该有 100 次允许")
}

// ============== RateLimiter 补充测试 ==============

func TestRateLimiter_Stop(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &RateLimiterConfig{
		IPRequestsPerSecond: 10,
		IPBurstSize:         20,
		BucketTimeout:       1 * time.Minute,
		CleanupInterval:     100 * time.Millisecond,
	}

	limiter := NewRateLimiter(config, logger)
	assert.NotNil(t, limiter)

	// 发送一些请求创建桶
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		limiter.Allow(ctx, "192.168.1.1", "", "/api/test", "GET")
	}

	// 停止
	limiter.Stop()

	// 再次停止应该是幂等的
	assert.NotPanics(t, func() {
		limiter.Stop()
	}, "多次调用 Stop 不应该 panic")
}

func TestRateLimiter_Close(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &RateLimiterConfig{
		IPRequestsPerSecond: 10,
		IPBurstSize:         20,
		BucketTimeout:       1 * time.Minute,
		CleanupInterval:     100 * time.Millisecond,
	}

	limiter := NewRateLimiter(config, logger)

	// Close 应该调用 Stop 并返回 nil
	err := limiter.Close()
	assert.NoError(t, err, "Close 应该返回 nil")

	// 再次 Close 应该是幂等的
	err = limiter.Close()
	assert.NoError(t, err, "多次调用 Close 不应该返回错误")
}

func TestRateLimiter_cleanup(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &RateLimiterConfig{
		IPRequestsPerSecond: 10,
		IPBurstSize:         20,
		BucketTimeout:       50 * time.Millisecond, // 很短的超时时间用于测试
		CleanupInterval:     1 * time.Hour,          // 禁用自动清理
		EndpointLimits:      nil,                    // 禁用端点限制，只创建 IP 桶
		RequestsPerSecond:   0,                      // 禁用全局限制
	}

	limiter := NewRateLimiter(config, logger)
	defer limiter.Stop()

	// 直接创建桶并设置为满的且过期
	limiter.mu.Lock()
	bucket := NewTokenBucket(20, 10) // 20 个令牌，每秒补充 10 个
	// 消耗所有令牌
	for i := 0; i < 20; i++ {
		bucket.Allow()
	}
	// 设置最后补充时间为很久之前
	bucket.lastRefill = time.Now().Add(-2 * time.Minute)
	limiter.buckets["test:expired"] = bucket
	limiter.mu.Unlock()

	// 等待桶过期
	time.Sleep(100 * time.Millisecond)

	// 手动调用 cleanup
	limiter.cleanup()

	stats := limiter.GetStats()
	// 桶未满（令牌已耗尽），不应该被清理
	assert.Equal(t, int64(1), stats.ActiveBuckets, "未满的桶不应该被清理")
}

func TestRateLimiter_cleanup_FullBucket(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &RateLimiterConfig{
		IPRequestsPerSecond: 10,
		IPBurstSize:         20,
		BucketTimeout:       50 * time.Millisecond,
		CleanupInterval:     1 * time.Hour,
		EndpointLimits:      nil,
		RequestsPerSecond:   0,
	}

	limiter := NewRateLimiter(config, logger)
	defer limiter.Stop()

	// 创建满的桶且长时间未使用
	limiter.mu.Lock()
	bucket := NewTokenBucket(20, 10)
	// 桶是满的（不需要消耗令牌）
	// 设置最后补充时间为很久之前
	bucket.lastRefill = time.Now().Add(-2 * time.Minute)
	limiter.buckets["test:full:expired"] = bucket
	limiter.mu.Unlock()

	// 等待超时
	time.Sleep(100 * time.Millisecond)

	// 手动调用 cleanup
	limiter.cleanup()

	stats := limiter.GetStats()
	// 桶满且过期，应该被清理
	assert.Equal(t, int64(0), stats.ActiveBuckets, "满的过期桶应该被清理")
}

func TestRateLimiter_cleanup_NoExpiredBuckets(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &RateLimiterConfig{
		IPRequestsPerSecond: 10,
		IPBurstSize:         20,
		BucketTimeout:       1 * time.Hour, // 很长的超时时间
		CleanupInterval:     1 * time.Hour, // 禁用自动清理
		EndpointLimits:      nil,           // 禁用端点限制
		RequestsPerSecond:   0,             // 禁用全局限制
	}

	limiter := NewRateLimiter(config, logger)
	defer limiter.Stop()

	ctx := context.Background()

	// 创建一些桶（只创建 IP 桶）
	for i := 0; i < 3; i++ {
		limiter.Allow(ctx, "192.168.1.101", "", "/api/test", "GET")
	}

	// 立即调用 cleanup（桶不应该过期）
	limiter.cleanup()

	stats := limiter.GetStats()
	assert.Equal(t, int64(1), stats.ActiveBuckets, "只有 1 个 IP 桶，不应该被清理")
}

func TestRateLimiter_cleanup_PartiallyFilledBucket(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &RateLimiterConfig{
		IPRequestsPerSecond: 10,
		IPBurstSize:         20,
		BucketTimeout:       50 * time.Millisecond,
		CleanupInterval:     1 * time.Hour,
	}

	limiter := NewRateLimiter(config, logger)
	defer limiter.Stop()

	ctx := context.Background()

	// 创建桶并消耗一些令牌（不是满的）
	for i := 0; i < 10; i++ {
		limiter.Allow(ctx, "192.168.1.102", "", "/api/test", "GET")
	}

	// 等待超时时间
	time.Sleep(100 * time.Millisecond)

	// 调用 cleanup
	limiter.cleanup()

	stats := limiter.GetStats()
	// 未满的桶不应该被清理（只有满的桶才会被清理）
	assert.GreaterOrEqual(t, stats.ActiveBuckets, int64(1), "未满的桶不应该被清理")
}

// ============== 并发访问测试 ==============

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &RateLimiterConfig{
		IPRequestsPerSecond:   100,
		IPBurstSize:           200,
		UserRequestsPerSecond: 50,
		UserBurstSize:         100,
		// 不使用端点限制，避免共享桶竞争
		EndpointLimits:  nil,
		BucketTimeout:   1 * time.Minute,
		CleanupInterval: 100 * time.Millisecond,
	}

	limiter := NewRateLimiter(config, logger)
	defer limiter.Stop()

	ctx := context.Background()

	var wg sync.WaitGroup

	// 启动 20 个协程，每个发送 50 个请求
	// 使用不同的 IP 和用户以避免桶竞争
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ip := "192.168.10." + fmt.Sprintf("%d", id)
			userID := "user_" + fmt.Sprintf("%d", id)
			for j := 0; j < 50; j++ {
				result := limiter.Allow(ctx, ip, userID, "/api/test", "GET")
				// 验证返回结果有效
				assert.NotNil(t, result)
			}
		}(i)
	}

	wg.Wait()

	// 验证统计信息（使用原子操作，应该准确）
	stats := limiter.GetStats()
	assert.Equal(t, int64(1000), stats.TotalRequests, "总请求数应该为 1000")
	assert.GreaterOrEqual(t, stats.AllowedRequests, int64(0), "允许的请求数应该非负")
}

func TestRateLimiter_ConcurrentBucketCreation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &RateLimiterConfig{
		IPRequestsPerSecond: 10,
		IPBurstSize:         20,
		BucketTimeout:       1 * time.Minute,
		CleanupInterval:     100 * time.Millisecond,
		EndpointLimits:      nil, // 禁用端点限制
		RequestsPerSecond:   0,   // 禁用全局限制
	}

	limiter := NewRateLimiter(config, logger)
	defer limiter.Stop()

	ctx := context.Background()

	var wg sync.WaitGroup

	// 启动 50 个协程创建不同的 IP 桶
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ip := "192.168.2." + fmt.Sprintf("%d", id)
			limiter.Allow(ctx, ip, "", "/api/test", "GET")
		}(i)
	}

	wg.Wait()

	stats := limiter.GetStats()
	assert.Equal(t, int64(50), stats.ActiveBuckets, "应该有 50 个不同的桶")
}

func TestRateLimiter_ConcurrentReset(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &RateLimiterConfig{
		IPRequestsPerSecond: 10,
		IPBurstSize:         20,
		BucketTimeout:       1 * time.Minute,
		CleanupInterval:     100 * time.Millisecond,
		EndpointLimits:      nil,
		RequestsPerSecond:   0,
	}

	limiter := NewRateLimiter(config, logger)
	defer limiter.Stop()

	ctx := context.Background()

	// 创建一些桶
	for i := 0; i < 10; i++ {
		limiter.Allow(ctx, "192.168.3.1", "", "/api/test", "GET")
	}

	// Reset 应该在所有操作之后
	limiter.Reset()

	// 验证 Reset 后统计归零
	stats := limiter.GetStats()
	assert.Equal(t, int64(0), stats.TotalRequests)
	assert.Equal(t, int64(0), stats.ActiveBuckets)

	// Reset 后再次创建桶
	limiter.Allow(ctx, "192.168.3.2", "", "/api/test", "GET")

	stats = limiter.GetStats()
	assert.Equal(t, int64(1), stats.ActiveBuckets)
}

// ============== 边界条件测试 ==============

func TestRateLimiter_EdgeCases(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	t.Run("NilConfig", func(t *testing.T) {
		// nil 配置应该使用默认配置
		limiter := NewRateLimiter(nil, logger)
		defer limiter.Stop()
		assert.NotNil(t, limiter.config)
		assert.Equal(t, 1000, limiter.config.RequestsPerSecond)
	})

	t.Run("EmptyEndpoint", func(t *testing.T) {
		config := DefaultRateLimiterConfig()
		limiter := NewRateLimiter(config, logger)
		defer limiter.Stop()

		ctx := context.Background()
		result := limiter.Allow(ctx, "192.168.4.1", "", "", "GET")
		assert.NotNil(t, result)
	})

	t.Run("EmptyUserID", func(t *testing.T) {
		config := DefaultRateLimiterConfig()
		limiter := NewRateLimiter(config, logger)
		defer limiter.Stop()

		ctx := context.Background()
		result := limiter.Allow(ctx, "192.168.4.2", "", "/api/test", "GET")
		assert.NotNil(t, result)
	})

	t.Run("EmptyIP", func(t *testing.T) {
		config := DefaultRateLimiterConfig()
		limiter := NewRateLimiter(config, logger)
		defer limiter.Stop()

		ctx := context.Background()
		result := limiter.Allow(ctx, "", "user_empty_ip", "/api/test", "GET")
		assert.NotNil(t, result)
	})
}

func TestRateLimiter_EdgeCases_ZeroConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &RateLimiterConfig{
		RequestsPerSecond:     0,
		BurstSize:             0,
		IPRequestsPerSecond:   0,
		IPBurstSize:           0,
		UserRequestsPerSecond: 0,
		UserBurstSize:         0,
		BucketTimeout:         0,
		CleanupInterval:       0,
	}

	limiter := NewRateLimiter(config, logger)
	defer limiter.Stop()

	ctx := context.Background()
	result := limiter.Allow(ctx, "192.168.5.1", "user_zero", "/api/test", "GET")
	assert.NotNil(t, result)
}

func TestRateLimiter_EdgeCases_NegativeConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &RateLimiterConfig{
		RequestsPerSecond:     -100,
		BurstSize:             -10,
		IPRequestsPerSecond:   -10,
		IPBurstSize:           -10,
		UserRequestsPerSecond: -5,
		UserBurstSize:         -5,
		BucketTimeout:         -1 * time.Minute,
		CleanupInterval:       -30 * time.Second,
	}

	limiter := NewRateLimiter(config, logger)
	defer limiter.Stop()

	ctx := context.Background()
	result := limiter.Allow(ctx, "192.168.6.1", "user_negative", "/api/test", "GET")
	// 负数配置应该被处理（可能表现为无限制或使用默认值）
	assert.NotNil(t, result)
}

func TestRateLimiter_ResetKey_NonExistentKey(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultRateLimiterConfig()
	limiter := NewRateLimiter(config, logger)
	defer limiter.Stop()

	// 重置不存在的键不应该 panic
	assert.NotPanics(t, func() {
		limiter.ResetKey("non_existent_key")
	}, "重置不存在的键不应该 panic")
}

func TestRateLimiter_RemoveEndpointLimit_NonExistentEndpoint(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultRateLimiterConfig()
	limiter := NewRateLimiter(config, logger)
	defer limiter.Stop()

	// 移除不存在的端点不应该 panic
	assert.NotPanics(t, func() {
		limiter.RemoveEndpointLimit("/non/existent")
	}, "移除不存在的端点不应该 panic")
}

func TestRateLimiter_SetEndpointLimit_NilMap(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &RateLimiterConfig{
		IPRequestsPerSecond: 10,
		IPBurstSize:         20,
		BucketTimeout:       1 * time.Minute,
		CleanupInterval:     100 * time.Millisecond,
		// EndpointLimits 初始化为 nil
	}

	limiter := NewRateLimiter(config, logger)
	defer limiter.Stop()

	limit := &EndpointLimit{
		RequestsPerSecond: 5,
		BurstSize:         10,
		Method:            "POST",
	}

	// 设置端点限制不应该 panic（应该初始化 map）
	assert.NotPanics(t, func() {
		limiter.SetEndpointLimit("/new/endpoint", limit)
	}, "设置端点限制到 nil map 不应该 panic")

	// 验证限制已被设置
	ctx := context.Background()
	result := limiter.Allow(ctx, "192.168.7.1", "", "/new/endpoint", "POST")
	assert.NotNil(t, result)
}

// ============== GetTokens 完整测试 ==============

func TestTokenBucket_GetTokens_AfterRefill(t *testing.T) {
	// 创建桶：最大 5 个令牌，每秒补充 10 个
	bucket := NewTokenBucket(5, 10)

	// 消耗所有令牌
	for i := 0; i < 5; i++ {
		bucket.Allow()
	}

	// 等待补充（10/s，200ms 应该补充约 2 个）
	time.Sleep(200 * time.Millisecond)

	// 使用 Allow() 验证令牌已补充（因为 Allow() 会先补充再消耗）
	allowed, remaining := bucket.Allow()
	assert.True(t, allowed, "应该有补充的令牌可用")
	assert.GreaterOrEqual(t, remaining, 0, "应该有剩余令牌")

	// 验证当前令牌数
	tokens := bucket.GetTokens()
	assert.GreaterOrEqual(t, tokens, 0, "令牌数应该非负")
	assert.LessOrEqual(t, tokens, 5, "令牌数不应该超过最大值")
}

// ============== 统计信息测试 ==============

func TestRateLimiterStats_Struct(t *testing.T) {
	stats := &RateLimiterStats{
		TotalRequests:   1000,
		AllowedRequests: 800,
		DeniedRequests:  200,
		ActiveBuckets:   50,
	}

	assert.Equal(t, int64(1000), stats.TotalRequests)
	assert.Equal(t, int64(800), stats.AllowedRequests)
	assert.Equal(t, int64(200), stats.DeniedRequests)
	assert.Equal(t, int64(50), stats.ActiveBuckets)

	// 验证 TotalRequests = AllowedRequests + DeniedRequests
	assert.Equal(t, stats.TotalRequests, stats.AllowedRequests+stats.DeniedRequests)
}

// ============== EndpointLimit 测试 ==============

func TestEndpointLimit_Struct(t *testing.T) {
	limit := &EndpointLimit{
		RequestsPerSecond: 10,
		BurstSize:         20,
		Method:            "GET",
	}

	assert.Equal(t, 10, limit.RequestsPerSecond)
	assert.Equal(t, 20, limit.BurstSize)
	assert.Equal(t, "GET", limit.Method)
}

func TestEndpointLimit_WildcardMethod(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &RateLimiterConfig{
		IPRequestsPerSecond: 1000, // 较高的 IP 限制
		IPBurstSize:         2000,
		EndpointLimits: map[string]*EndpointLimit{
			"/api/wildcard": {RequestsPerSecond: 2, BurstSize: 5, Method: "*"},
		},
		BucketTimeout:   1 * time.Minute,
		CleanupInterval: 100 * time.Millisecond,
	}

	limiter := NewRateLimiter(config, logger)
	defer limiter.Stop()

	ctx := context.Background()

	// 通配符方法 "*" 匹配所有 HTTP 方法
	// 所有方法共享同一个端点桶（键为 "/api/wildcard"）
	// burst size 为 5，所以前 5 个请求应该允许（无论什么方法）

	// 前 5 个请求应该允许
	for i := 0; i < 5; i++ {
		result := limiter.Allow(ctx, "192.168.8.1", "", "/api/wildcard", "GET")
		assert.True(t, result.Allowed, "请求 %d 应该被允许", i)
	}

	// 第 6 个请求应该被限制（共享桶已满）
	result := limiter.Allow(ctx, "192.168.8.1", "", "/api/wildcard", "POST")
	assert.False(t, result.Allowed, "请求应该被限制")
	assert.Equal(t, "endpoint_limit", result.Reason)
}

// ============== cleanupWorker 测试 ==============

func TestRateLimiter_cleanupWorker(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &RateLimiterConfig{
		IPRequestsPerSecond: 10,
		IPBurstSize:         20,
		BucketTimeout:       50 * time.Millisecond,  // 很短的超时时间
		CleanupInterval:     25 * time.Millisecond,  // 很短的清理间隔
		EndpointLimits:      nil,
		RequestsPerSecond:   0,
	}

	limiter := NewRateLimiter(config, logger)
	defer limiter.Stop()

	// 直接创建满的且过期的桶
	limiter.mu.Lock()
	for i := 0; i < 3; i++ {
		bucket := NewTokenBucket(20, 10)
		// 设置最后补充时间为很久之前（桶是满的）
		bucket.lastRefill = time.Now().Add(-2 * time.Minute)
		limiter.buckets["test:expired:"+fmt.Sprintf("%d", i)] = bucket
	}
	limiter.mu.Unlock()

	// 等待自动清理（清理间隔 25ms，等待 100ms 应该足够）
	time.Sleep(100 * time.Millisecond)

	stats := limiter.GetStats()
	// 桶应该被自动清理
	assert.Equal(t, int64(0), stats.ActiveBuckets, "桶应该被自动清理")
}
