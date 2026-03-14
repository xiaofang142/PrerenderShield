package detectors

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"prerender-shield/internal/security/waf/types"
)

// BlacklistDetector 黑名单检测器
type BlacklistDetector struct {
	blacklistedIPs map[string]bool
	blacklistedUA  map[string]bool
	mu             sync.RWMutex
}

// NewBlacklistDetector 创建黑名单检测器
func NewBlacklistDetector() *BlacklistDetector {
	return &BlacklistDetector{
		blacklistedIPs: make(map[string]bool),
		blacklistedUA:  make(map[string]bool),
	}
}

// AddIP 添加 IP 到黑名单
func (d *BlacklistDetector) AddIP(ip string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.blacklistedIPs[ip] = true
}

// AddUserAgent 添加 User Agent 到黑名单
func (d *BlacklistDetector) AddUserAgent(ua string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.blacklistedUA[ua] = true
}

// Check 检查请求
func (d *BlacklistDetector) Check(req *http.Request) *types.CheckResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// 检查 IP
	if d.blacklistedIPs[req.RemoteAddr] {
		return &types.CheckResult{
			Allowed: false,
			Blocked: true,
			Reason:  "IP blacklisted",
			RuleID:  "blacklist-001",
			Threat: &types.Threat{
				Type:     types.ThreatMaliciousIP,
				Severity: "high",
				Source:   "ip",
			},
		}
	}

	// 检查 User Agent
	ua := req.Header.Get("User-Agent")
	for blockedUA := range d.blacklistedUA {
		if strings.Contains(ua, blockedUA) {
			return &types.CheckResult{
				Allowed: false,
				Blocked: true,
				Reason:  "User-Agent blacklisted",
				RuleID:  "blacklist-002",
				Threat: &types.Threat{
					Type:     types.ThreatMaliciousIP,
					Severity: "medium",
					Source:   "user_agent",
				},
			}
		}
	}

	return &types.CheckResult{
		Allowed: true,
	}
}

// RateLimitDetector 速率限制检测器
type RateLimitDetector struct {
	requests      map[string][]time.Time
	limit         int
	window        time.Duration
	mu            sync.Mutex
}

// NewRateLimitDetector 创建速率限制检测器
func NewRateLimitDetector(requestsPerMinute int) *RateLimitDetector {
	return &RateLimitDetector{
		requests: make(map[string][]time.Time),
		limit:    requestsPerMinute,
		window:   time.Minute,
	}
}

// Check 检查请求
func (d *RateLimitDetector) Check(req *http.Request) *types.CheckResult {
	d.mu.Lock()
	defer d.mu.Unlock()

	ip := req.RemoteAddr
	now := time.Now()

	// 清理过期记录
	if requests, ok := d.requests[ip]; ok {
		var valid []time.Time
		for _, t := range requests {
			if now.Sub(t) < d.window {
				valid = append(valid, t)
			}
		}
		d.requests[ip] = valid
	}

	// 添加新请求
	d.requests[ip] = append(d.requests[ip], now)

	// 检查是否超限
	if len(d.requests[ip]) > d.limit {
		return &types.CheckResult{
			Allowed: false,
			Blocked: true,
			Reason:  "Rate limit exceeded",
			RuleID:  "ratelimit-001",
			Threat: &types.Threat{
				Type:     types.ThreatRateLimit,
				Severity: "low",
				Source:   "ip",
			},
		}
	}

	return &types.CheckResult{
		Allowed: true,
	}
}
