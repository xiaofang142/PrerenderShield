package ratelimit

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RateLimiter 速率限制器
type RateLimiter struct {
	config    *RateLimiterConfig
	buckets   map[string]*TokenBucket
	mu        sync.RWMutex
	logger    *zap.Logger
	stats     *RateLimiterStats
	stopChan  chan struct{}
	stopped   bool
	closeMu   sync.Mutex
}

// RateLimiterConfig 速率限制配置
type RateLimiterConfig struct {
	// 全局配置
	RequestsPerSecond int           // 全局每秒请求数
	BurstSize         int           // 突发大小

	// IP 级别配置
	IPRequestsPerSecond int           // 每 IP 每秒请求数
	IPBurstSize         int           // 每 IP 突发大小

	// 用户级别配置
	UserRequestsPerSecond int         // 每用户每秒请求数
	UserBurstSize         int         // 每用户突发大小

	// 端点级别配置
	EndpointLimits map[string]*EndpointLimit // 端点特定限制

	// 存储配置
	BucketTimeout   time.Duration // 桶超时时间
	CleanupInterval time.Duration // 清理间隔
}

// EndpointLimit 端点限制
type EndpointLimit struct {
	RequestsPerSecond int    `json:"requests_per_second"`
	BurstSize         int    `json:"burst_size"`
	Method            string `json:"method"` // GET, POST, *
}

// TokenBucket 令牌桶
type TokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64
	lastRefill time.Time
	mu         sync.Mutex
}

// RateLimiterStats 统计信息
type RateLimiterStats struct {
	TotalRequests   int64 `json:"total_requests"`
	AllowedRequests int64 `json:"allowed_requests"`
	DeniedRequests  int64 `json:"denied_requests"`
	ActiveBuckets   int64 `json:"active_buckets"`
}

// LimitResult 限制结果
type LimitResult struct {
	Allowed   bool    `json:"allowed"`
	Remaining int     `json:"remaining"`
	ResetAfter int64  `json:"reset_after"` // 秒
	RetryAfter  int64  `json:"retry_after"` // 秒
	Reason      string `json:"reason,omitempty"`
}

// DefaultRateLimiterConfig 返回默认配置
func DefaultRateLimiterConfig() *RateLimiterConfig {
	return &RateLimiterConfig{
		RequestsPerSecond:     1000,
		BurstSize:             2000,
		IPRequestsPerSecond:   100,
		IPBurstSize:           200,
		UserRequestsPerSecond: 50,
		UserBurstSize:         100,
		EndpointLimits: map[string]*EndpointLimit{
			"/api/login":    {RequestsPerSecond: 5, BurstSize: 10, Method: "POST"},
			"/api/register": {RequestsPerSecond: 3, BurstSize: 5, Method: "POST"},
			"/api/search":   {RequestsPerSecond: 20, BurstSize: 40, Method: "GET"},
		},
		BucketTimeout:   10 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(config *RateLimiterConfig, logger *zap.Logger) *RateLimiter {
	if config == nil {
		config = DefaultRateLimiterConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	limiter := &RateLimiter{
		config:   config,
		buckets:  make(map[string]*TokenBucket),
		logger:   logger,
		stats:    &RateLimiterStats{},
		stopChan: make(chan struct{}),
	}

	// 启动清理协程
	go limiter.cleanupWorker()

	return limiter
}

// Allow 检查是否允许请求
func (r *RateLimiter) Allow(ctx context.Context, ip, userID, endpoint, method string) *LimitResult {
	r.stats.TotalRequests++

	// 检查端点限制
	if endpointLimit, ok := r.config.EndpointLimits[endpoint]; ok {
		if endpointLimit.Method == "*" || endpointLimit.Method == method {
			result := r.checkLimit(endpoint, endpointLimit.RequestsPerSecond, endpointLimit.BurstSize)
			if !result.Allowed {
				r.stats.DeniedRequests++
				result.Reason = "endpoint_limit"
				return result
			}
		}
	}

	// 检查 IP 限制
	ipResult := r.checkLimit("ip:"+ip, r.config.IPRequestsPerSecond, r.config.IPBurstSize)
	if !ipResult.Allowed {
		r.stats.DeniedRequests++
		ipResult.Reason = "ip_limit"
		return ipResult
	}

	// 检查用户限制
	if userID != "" {
		userResult := r.checkLimit("user:"+userID, r.config.UserRequestsPerSecond, r.config.UserBurstSize)
		if !userResult.Allowed {
			r.stats.DeniedRequests++
			userResult.Reason = "user_limit"
			return userResult
		}
	}

	// 检查全局限制（如果配置了）
	if r.config.RequestsPerSecond > 0 && r.config.BurstSize > 0 {
		globalResult := r.checkLimit("global", r.config.RequestsPerSecond, r.config.BurstSize)
		if !globalResult.Allowed {
			r.stats.DeniedRequests++
			globalResult.Reason = "global_limit"
			return globalResult
		}
	}

	r.stats.AllowedRequests++
	return ipResult // 返回 IP 限制结果作为通用结果
}

// checkLimit 检查特定限制
func (r *RateLimiter) checkLimit(key string, rate, burst int) *LimitResult {
	bucket := r.getOrCreateBucket(key, rate, burst)

	allowed, remaining := bucket.Allow()
	resetAfter := int(bucket.maxTokens / bucket.refillRate)

	if !allowed {
		retryAfter := int((1 - bucket.tokens) / bucket.refillRate)
		if retryAfter < 1 {
			retryAfter = 1
		}
		return &LimitResult{
			Allowed:    false,
			Remaining:  0,
			ResetAfter: int64(resetAfter),
			RetryAfter: int64(retryAfter),
		}
	}

	return &LimitResult{
		Allowed:    true,
		Remaining:  remaining,
		ResetAfter: int64(resetAfter),
	}
}

// getOrCreateBucket 获取或创建桶
func (r *RateLimiter) getOrCreateBucket(key string, rate, burst int) *TokenBucket {
	r.mu.RLock()
	bucket, exists := r.buckets[key]
	r.mu.RUnlock()

	if exists {
		return bucket
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 双重检查
	if bucket, exists = r.buckets[key]; exists {
		return bucket
	}

	bucket = NewTokenBucket(float64(burst), float64(rate))
	r.buckets[key] = bucket
	r.stats.ActiveBuckets++

	return bucket
}

// NewTokenBucket 创建令牌桶
func NewTokenBucket(maxTokens, refillRate float64) *TokenBucket {
	return &TokenBucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow 尝试获取令牌
func (b *TokenBucket) Allow() (bool, int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// 补充令牌
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastRefill = now

	// 消耗令牌
	if b.tokens >= 1 {
		b.tokens--
		return true, int(b.tokens)
	}

	return false, 0
}

// GetTokens 获取当前令牌数
func (b *TokenBucket) GetTokens() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return int(b.tokens)
}

// cleanupWorker 清理过期桶
func (r *RateLimiter) cleanupWorker() {
	ticker := time.NewTicker(r.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.cleanup()
		case <-r.stopChan:
			return
		}
	}
}

// Stop 停止清理协程
func (r *RateLimiter) Stop() {
	r.closeMu.Lock()
	defer r.closeMu.Unlock()

	if r.stopped {
		return
	}
	r.stopped = true
	close(r.stopChan)
}

// Close 实现 io.Closer 接口
func (r *RateLimiter) Close() error {
	r.Stop()
	return nil
}

// cleanup 清理过期桶
func (r *RateLimiter) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	deleted := 0

	for key, bucket := range r.buckets {
		bucket.mu.Lock()
		// 如果桶已满且长时间未使用，删除
		if bucket.tokens >= bucket.maxTokens && now.Sub(bucket.lastRefill) > r.config.BucketTimeout {
			delete(r.buckets, key)
			deleted++
		}
		bucket.mu.Unlock()
	}

	if deleted > 0 {
		r.stats.ActiveBuckets -= int64(deleted)
		r.logger.Debug("清理过期桶", zap.Int("count", deleted))
	}
}

// GetStats 获取统计信息
func (r *RateLimiter) GetStats() *RateLimiterStats {
	r.stats.ActiveBuckets = int64(len(r.buckets))
	return r.stats
}

// Reset 重置所有限制
func (r *RateLimiter) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buckets = make(map[string]*TokenBucket)
	r.stats = &RateLimiterStats{}
}

// ResetKey 重置特定键的限制
func (r *RateLimiter) ResetKey(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.buckets, key)
}

// SetEndpointLimit 设置端点限制
func (r *RateLimiter) SetEndpointLimit(endpoint string, limit *EndpointLimit) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.config.EndpointLimits == nil {
		r.config.EndpointLimits = make(map[string]*EndpointLimit)
	}
	r.config.EndpointLimits[endpoint] = limit
}

// RemoveEndpointLimit 移除端点限制
func (r *RateLimiter) RemoveEndpointLimit(endpoint string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.config.EndpointLimits, endpoint)
}
