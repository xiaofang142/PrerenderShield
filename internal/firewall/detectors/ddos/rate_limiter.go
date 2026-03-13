package ddos

import (
	"sync"
	"sync/atomic"
	"time"
)

// RateLimiter 滑动窗口速率限制器
type RateLimiter struct {
	mu               sync.RWMutex
	rateThreshold    int           // 每秒请求数阈值
	burstThreshold   int           // 突发请求数阈值
	ipWindows        map[string]*SlidingWindow
	cleanupInterval  time.Duration
	lastCleanupTime  time.Time
	stopChan         chan struct{}
	stopped          atomic.Bool
}

// SlidingWindow 滑动窗口
type SlidingWindow struct {
	requests []time.Time // 请求时间戳
	mu       sync.Mutex
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(rateThreshold, burstThreshold int) *RateLimiter {
	rl := &RateLimiter{
		rateThreshold:   rateThreshold,
		burstThreshold:  burstThreshold,
		ipWindows:       make(map[string]*SlidingWindow),
		cleanupInterval: 1 * time.Minute,
		lastCleanupTime: time.Now(),
		stopChan:        make(chan struct{}),
	}

	// 启动后台清理协程
	go rl.startCleanup()

	return rl
}

// RecordRequest 记录请求
func (rl *RateLimiter) RecordRequest(ip string) {
	rl.mu.Lock()
	window, exists := rl.ipWindows[ip]
	if !exists {
		window = &SlidingWindow{
			requests: make([]time.Time, 0, rl.rateThreshold*2),
		}
		rl.ipWindows[ip] = window
	}
	rl.mu.Unlock()

	window.mu.Lock()
	defer window.mu.Unlock()

	now := time.Now()
	window.requests = append(window.requests, now)

	// 清理窗口外的请求（保留最近 1 秒的请求用于速率计算）
	cutoff := now.Add(-time.Second)
	validRequests := make([]time.Time, 0, len(window.requests))
	for _, t := range window.requests {
		if t.After(cutoff) {
			validRequests = append(validRequests, t)
		}
	}
	window.requests = validRequests
}

// IsRateLimited 检查是否超过速率限制
func (rl *RateLimiter) IsRateLimited(ip string) bool {
	rl.mu.RLock()
	window, exists := rl.ipWindows[ip]
	if !exists {
		rl.mu.RUnlock()
		return false
	}
	rl.mu.RUnlock()

	window.mu.Lock()
	defer window.mu.Unlock()

	now := time.Now()

	// 计算最近 1 秒的请求数
	cutoff := now.Add(-time.Second)
	rateCount := 0
	for _, t := range window.requests {
		if t.After(cutoff) {
			rateCount++
		}
	}

	// 检查速率限制
	if rateCount >= rl.rateThreshold {
		return true
	}

	// 检查突发限制（最近 100ms 的请求数）
	burstCutoff := now.Add(-100 * time.Millisecond)
	burstCount := 0
	for _, t := range window.requests {
		if t.After(burstCutoff) {
			burstCount++
		}
	}

	return burstCount >= rl.burstThreshold
}

// GetRequestRate 获取当前请求速率
func (rl *RateLimiter) GetRequestRate(ip string) int {
	rl.mu.RLock()
	window, exists := rl.ipWindows[ip]
	if !exists {
		rl.mu.RUnlock()
		return 0
	}
	rl.mu.RUnlock()

	window.mu.Lock()
	defer window.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Second)
	count := 0
	for _, t := range window.requests {
		if t.After(cutoff) {
			count++
		}
	}

	return count
}

// GetBurstCount 获取突发请求数
func (rl *RateLimiter) GetBurstCount(ip string) int {
	rl.mu.RLock()
	window, exists := rl.ipWindows[ip]
	if !exists {
		rl.mu.RUnlock()
		return 0
	}
	rl.mu.RUnlock()

	window.mu.Lock()
	defer window.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-100 * time.Millisecond)
	count := 0
	for _, t := range window.requests {
		if t.After(cutoff) {
			count++
		}
	}

	return count
}

// CleanupExpired 清理过期数据
func (rl *RateLimiter) CleanupExpired() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for ip, window := range rl.ipWindows {
		window.mu.Lock()
		// 如果窗口为空或所有请求都超过 1 分钟，删除该窗口
		cutoff := now.Add(-time.Minute)
		hasValidRequests := false
		for _, t := range window.requests {
			if t.After(cutoff) {
				hasValidRequests = true
				break
			}
		}
		window.mu.Unlock()

		if !hasValidRequests {
			delete(rl.ipWindows, ip)
		}
	}
}

// startCleanup 启动定期清理
func (rl *RateLimiter) startCleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.CleanupExpired()
		case <-rl.stopChan:
			return
		}
	}
}

// Stop 停止清理协程
func (rl *RateLimiter) Stop() {
	if rl.stopped.Swap(true) {
		return // Already stopped
	}
	close(rl.stopChan)
}

// GetStats 获取统计信息
func (rl *RateLimiter) GetStats() *RateLimiterStats {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	totalIPs := len(rl.ipWindows)
	limitedIPs := 0

	for ip := range rl.ipWindows {
		if rl.IsRateLimited(ip) {
			limitedIPs++
		}
	}

	return &RateLimiterStats{
		TotalIPs:     totalIPs,
		LimitedIPs:   limitedIPs,
		Threshold:    rl.rateThreshold,
		BurstLimit:   rl.burstThreshold,
	}
}

// RateLimiterStats 速率限制器统计
type RateLimiterStats struct {
	TotalIPs   int `json:"total_ips"`
	LimitedIPs int `json:"limited_ips"`
	Threshold  int `json:"threshold"`
	BurstLimit int `json:"burst_limit"`
}

// TokenBucket 令牌桶限流器（备用算法）
type TokenBucket struct {
	mu           sync.Mutex
	capacity     int           // 桶容量
	tokens       float64       // 当前令牌数
	refillRate   float64       // 每秒补充令牌数
	lastRefill   time.Time     // 上次补充时间
}

// NewTokenBucket 创建令牌桶
func NewTokenBucket(capacity, refillRate int) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     float64(capacity),
		refillRate: float64(refillRate),
		lastRefill: time.Now(),
	}
}

// Allow 检查是否允许请求
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.refillRate

	if tb.tokens > float64(tb.capacity) {
		tb.tokens = float64(tb.capacity)
	}

	tb.lastRefill = now

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}

	return false
}

// GetTokens 获取当前令牌数
func (tb *TokenBucket) GetTokens() int {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tokens := tb.tokens + elapsed*tb.refillRate

	if tokens > float64(tb.capacity) {
		tokens = float64(tb.capacity)
	}

	return int(tokens)
}
