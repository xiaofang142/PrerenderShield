package detectors

import (
	"net/http"
	"sync"
	"time"

	"github.com/prerendershield/internal/firewall/types"
)

// RateLimitDetector 频率限制检测器
type RateLimitDetector struct {
	mutex      sync.RWMutex
	ipCounters map[string]*IPCounter
}

// IPCounter IP请求计数器
type IPCounter struct {
	Requests    []time.Time
	BannedUntil time.Time
}

// NewRateLimitDetector 创建新的频率限制检测器
func NewRateLimitDetector() *RateLimitDetector {
	d := &RateLimitDetector{
		ipCounters: make(map[string]*IPCounter),
	}

	// 启动清理过期请求的协程
	go d.cleanupLoop()

	return d
}

// Detect 检测请求是否超过频率限制
func (d *RateLimitDetector) Detect(req *http.Request) ([]types.Threat, error) {
	threats := make([]types.Threat, 0)

	// 获取请求IP地址
	ip := getClientIP(req)
	if ip == "" {
		return threats, nil
	}

	// 检查是否被封禁
	if d.isBanned(ip) {
		threats = append(threats, types.Threat{
			Type:     "rate_limit",
			SubType:  "banned",
			Severity: "high",
			Message:  "IP is banned due to excessive requests",
			SourceIP: ip,
			Details: map[string]interface{}{
				"reason": "banned",
			},
		})
		return threats, nil
	}

	// 检查是否超过频率限制
	// 注意：实际实现中，应该从配置中获取requests和window
	// 这里只是一个示例：100次请求/60秒
	maxRequests := 100
	window := 60 * time.Second
	banTime := 3600 * time.Second

	if d.exceedsRateLimit(ip, maxRequests, window) {
		// 封禁IP
		d.banIP(ip, banTime)

		threats = append(threats, types.Threat{
			Type:     "rate_limit",
			SubType:  "exceeded",
			Severity: "high",
			Message:  "Exceeded request rate limit",
			SourceIP: ip,
			Details: map[string]interface{}{
				"max_requests": maxRequests,
				"window":       window.Seconds(),
				"ban_time":     banTime.Seconds(),
			},
		})
	}

	return threats, nil
}

// Name 返回检测器名称
func (d *RateLimitDetector) Name() string {
	return "rate_limit"
}

// exceedsRateLimit 检查IP是否超过频率限制
func (d *RateLimitDetector) exceedsRateLimit(ip string, maxRequests int, window time.Duration) bool {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// 获取或创建IP计数器
	counter, exists := d.ipCounters[ip]
	if !exists {
		counter = &IPCounter{
			Requests: make([]time.Time, 0),
		}
		d.ipCounters[ip] = counter
	}

	// 添加当前请求时间
	now := time.Now()
	counter.Requests = append(counter.Requests, now)

	// 移除过期的请求
	cutoff := now.Add(-window)
	validRequests := make([]time.Time, 0)
	for _, reqTime := range counter.Requests {
		if reqTime.After(cutoff) {
			validRequests = append(validRequests, reqTime)
		}
	}
	counter.Requests = validRequests

	// 更新计数器
	d.ipCounters[ip] = counter

	// 检查是否超过限制
	return len(validRequests) > maxRequests
}

// isBanned 检查IP是否被封禁
func (d *RateLimitDetector) isBanned(ip string) bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	counter, exists := d.ipCounters[ip]
	if !exists {
		return false
	}

	return !counter.BannedUntil.IsZero() && time.Now().Before(counter.BannedUntil)
}

// banIP 封禁IP
func (d *RateLimitDetector) banIP(ip string, duration time.Duration) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	counter, exists := d.ipCounters[ip]
	if !exists {
		counter = &IPCounter{
			Requests: make([]time.Time, 0),
		}
		d.ipCounters[ip] = counter
	}

	counter.BannedUntil = time.Now().Add(duration)
	// 清空请求记录
	counter.Requests = make([]time.Time, 0)
}

// cleanupLoop 定期清理过期的请求记录
func (d *RateLimitDetector) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		d.cleanupExpired()
	}
}

// cleanupExpired 清理过期的请求记录
func (d *RateLimitDetector) cleanupExpired() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	now := time.Now()
	for ip, counter := range d.ipCounters {
		// 检查是否需要清理
		if len(counter.Requests) == 0 && (counter.BannedUntil.IsZero() || now.After(counter.BannedUntil)) {
			delete(d.ipCounters, ip)
		}
	}
}
