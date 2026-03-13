package ddos

import (
	"context"
	"net/http"
	"sync"
	"time"

	"prerender-shield/internal/firewall/types"
	"prerender-shield/internal/logging"

	"github.com/go-redis/redis/v8"
)

// Config DDoS 检测器配置
type Config struct {
	Enabled              bool          // 是否启用 DDoS 检测
	EnabledDDoSDetection bool          // 是否启用 DDoS 检测（同 Enabled，用于兼容）
	RateThreshold        int           // 每秒请求数阈值
	BurstThreshold       int           // 突发请求数阈值
	ChallengeThreshold   int           // 触发挑战的请求数阈值
	BlockDuration        time.Duration // 封禁持续时间
	ChallengeDuration    time.Duration // 挑战持续时间
	Whitelist            []string      // 白名单 IP 列表
	EnableRedis          bool          // 是否启用 Redis 分布式检测
	RedisKeyPrefix       string        // Redis 键前缀
}

// Detector DDoS 检测器
type Detector struct {
	config         *Config
	rateLimiter    *RateLimiter
	challenge      *ChallengeManager
	ipTracker      *IPTracker
	blacklist      *Blacklist
	redisClient    *redis.Client
	whitelistMap   map[string]bool
	mutex          sync.RWMutex
	cleanupStopSig chan struct{}
}

// NewDetector 创建 DDoS 检测器
func NewDetector(config *Config, redisClient *redis.Client) (*Detector, error) {
	if config == nil {
		config = getDefaultConfig()
	}

	// 使用默认值填充未设置的配置
	if config.RateThreshold <= 0 {
		config.RateThreshold = 100
	}
	if config.BurstThreshold <= 0 {
		config.BurstThreshold = 50
	}
	if config.ChallengeThreshold <= 0 {
		config.ChallengeThreshold = 30
	}
	if config.BlockDuration <= 0 {
		config.BlockDuration = 10 * time.Minute
	}
	if config.ChallengeDuration <= 0 {
		config.ChallengeDuration = 5 * time.Minute
	}
	if config.RedisKeyPrefix == "" {
		config.RedisKeyPrefix = "firewall:ddos"
	}

	// 构建白名单映射
	whitelistMap := make(map[string]bool)
	for _, ip := range config.Whitelist {
		whitelistMap[ip] = true
	}

	d := &Detector{
		config:         config,
		rateLimiter:    NewRateLimiter(config.RateThreshold, config.BurstThreshold),
		challenge:      NewChallengeManager(config.ChallengeDuration),
		ipTracker:      NewIPTracker(),
		blacklist:      NewBlacklist(config.BlockDuration),
		redisClient:    redisClient,
		whitelistMap:   whitelistMap,
		cleanupStopSig: make(chan struct{}),
	}

	// 启动清理协程
	go d.cleanupLoop()

	return d, nil
}

// Name 返回检测器名称
func (d *Detector) Name() string {
	return "ddos"
}

// Detect 检测请求是否为 DDoS 攻击
func (d *Detector) Detect(req *http.Request) ([]types.Threat, error) {
	threats := make([]types.Threat, 0)

	// 如果 DDoS 检测未启用，直接返回
	if !d.config.Enabled && !d.config.EnabledDDoSDetection {
		return threats, nil
	}

	// 获取请求 IP 地址
	ip := logging.GetClientIP(req)
	if ip == "" {
		return threats, nil
	}

	// 检查白名单
	if d.isWhitelisted(ip) {
		return threats, nil
	}

	// 1. 检查是否在黑名单中
	if d.blacklist.IsBlacklisted(ip) {
		threats = append(threats, types.Threat{
			Type:     "ddos",
			SubType:  "blacklisted",
			Severity: "critical",
			Message:  "IP is blacklisted due to DDoS attack detection",
			SourceIP: ip,
			Details: map[string]interface{}{
				"reason": "blacklist",
			},
		})
		return threats, nil
	}

	// 2. 检查是否处于挑战状态
	if challengeStatus := d.challenge.GetStatus(ip); challengeStatus != nil {
		// 验证挑战响应
		if !d.challenge.VerifyChallenge(req, ip) {
			threats = append(threats, types.Threat{
				Type:     "ddos",
				SubType:  "challenge_failed",
				Severity: "high",
				Message:  "Challenge verification failed",
				SourceIP: ip,
				Details: map[string]interface{}{
					"challenge_token": challengeStatus.Token,
				},
			})
			return threats, nil
		}
		// 挑战成功，移除挑战状态
		d.challenge.RemoveChallenge(ip)
		return threats, nil
	}

	// 3. 检查请求频率
	if d.rateLimiter.IsRateLimited(ip) {
		// 超过频率限制，添加黑名单
		d.blacklist.Add(ip, "rate_limit_exceeded")

		threats = append(threats, types.Threat{
			Type:     "ddos",
			SubType:  "rate_limit_exceeded",
			Severity: "high",
			Message:  "Request rate limit exceeded",
			SourceIP: ip,
			Details: map[string]interface{}{
				"rate_threshold": d.config.RateThreshold,
			},
		})
		return threats, nil
	}

	// 4. 更新 IP 行为追踪
	d.ipTracker.RecordRequest(ip, req)

	// 5. 检查是否有 DDoS 攻击特征
	if attackType := d.detectDDoSPattern(ip); attackType != "" {
		// 触发挑战
		if d.ipTracker.GetRequestCount(ip, 10*time.Second) >= d.config.ChallengeThreshold {
			d.challenge.StartChallenge(ip)
			threats = append(threats, types.Threat{
				Type:     "ddos",
				SubType:  "challenge_required",
				Severity: "medium",
				Message:  "Challenge required due to suspicious behavior",
				SourceIP: ip,
				Details: map[string]interface{}{
					"attack_type": attackType,
					"challenge":   true,
				},
			})
			return threats, nil
		}

		threats = append(threats, types.Threat{
			Type:     "ddos",
			SubType:  attackType,
			Severity: "high",
			Message:  "DDoS attack pattern detected",
			SourceIP: ip,
			Details: map[string]interface{}{
				"attack_type": attackType,
			},
		})
		return threats, nil
	}

	// 6. 记录正常请求到速率限制器
	d.rateLimiter.RecordRequest(ip)

	return threats, nil
}

// detectDDoSPattern 检测 DDoS 攻击模式
func (d *Detector) detectDDoSPattern(ip string) string {
	// 检查 HTTP Flood 攻击（短时间内大量请求）
	if d.ipTracker.GetRequestCount(ip, time.Second) > d.config.BurstThreshold {
		return "http_flood"
	}

	// 检查 Slowloris 攻击（长时间保持连接）
	if d.ipTracker.HasSlowlorisPattern(ip) {
		return "slowloris"
	}

	// 检查请求头是否异常（缺少 User-Agent 等）
	if d.ipTracker.HasSuspiciousHeaders(ip) {
		return "suspicious_headers"
	}

	// 检查是否有分布式攻击特征（同一 IP 段多个 IP 同时请求）
	if d.ipTracker.HasDistributedPattern(ip) {
		return "distributed_attack"
	}

	return ""
}

// isWhitelisted 检查 IP 是否在白名单中
func (d *Detector) isWhitelisted(ip string) bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.whitelistMap[ip]
}

// AddToWhitelist 添加 IP 到白名单
func (d *Detector) AddToWhitelist(ip string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.whitelistMap[ip] = true
}

// RemoveFromWhitelist 从白名单移除 IP
func (d *Detector) RemoveFromWhitelist(ip string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	delete(d.whitelistMap, ip)
}

// GetStatus 获取 DDoS 检测器状态
func (d *Detector) GetStatus(ip string) *IPStatus {
	return &IPStatus{
		RequestCount:    d.ipTracker.GetRequestCount(ip, time.Minute),
		IsBlacklisted:   d.blacklist.IsBlacklisted(ip),
		IsChallenged:    d.challenge.GetStatus(ip) != nil,
		IsRateLimited:   d.rateLimiter.IsRateLimited(ip),
		FirstSeen:       d.ipTracker.GetFirstSeen(ip),
		LastSeen:        d.ipTracker.GetLastSeen(ip),
		SuspiciousScore: d.ipTracker.GetSuspiciousScore(ip),
	}
}

// IPStatus IP 状态信息
type IPStatus struct {
	RequestCount    int       // 请求次数
	IsBlacklisted   bool      // 是否被黑名单
	IsChallenged    bool      // 是否需要挑战
	IsRateLimited   bool      // 是否被频率限制
	FirstSeen       time.Time // 首次见到时间
	LastSeen        time.Time // 最后见到时间
	SuspiciousScore float64   // 可疑分数
}

// cleanupLoop 定期清理过期数据
func (d *Detector) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.cleanupExpired()
		case <-d.cleanupStopSig:
			return
		}
	}
}

// cleanupExpired 清理过期数据
func (d *Detector) cleanupExpired() {
	d.blacklist.CleanupExpired()
	d.challenge.CleanupExpired()
	d.ipTracker.CleanupExpired()
	d.rateLimiter.CleanupExpired()
}

// Stop 停止检测器
func (d *Detector) Stop() {
	close(d.cleanupStopSig)
	if d.rateLimiter != nil {
		d.rateLimiter.Stop()
	}
}

// getDefaultConfig 获取默认配置
func getDefaultConfig() *Config {
	return &Config{
		Enabled:              true,
		RateThreshold:        100,
		BurstThreshold:       50,
		ChallengeThreshold:   30,
		BlockDuration:        10 * time.Minute,
		ChallengeDuration:    5 * time.Minute,
		RedisKeyPrefix:       "firewall:ddos",
		EnableRedis:          false,
	}
}

// checkRedis 检查 Redis 中的状态（用于分布式场景）
func (d *Detector) checkRedis(ctx context.Context, key string) (bool, error) {
	if !d.config.EnableRedis || d.redisClient == nil {
		return false, nil
	}

	fullKey := d.config.RedisKeyPrefix + ":" + key
	exists, err := d.redisClient.Exists(ctx, fullKey).Result()
	if err != nil {
		return false, err
	}

	return exists > 0, nil
}

// setRedis 设置 Redis 状态（用于分布式场景）
func (d *Detector) setRedis(ctx context.Context, key string, value string, expiration time.Duration) error {
	if !d.config.EnableRedis || d.redisClient == nil {
		return nil
	}

	fullKey := d.config.RedisKeyPrefix + ":" + key
	return d.redisClient.Set(ctx, fullKey, value, expiration).Err()
}
