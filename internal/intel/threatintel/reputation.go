package threatintel

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// IPReputationService IP 信誉评分服务
type IPReputationService struct {
	config      *IPReputationConfig
	cache       *IPReputationCache
	threatIntel *ThreatIntelClient
	logger      *zap.Logger
	mu          sync.RWMutex
	history     *IPHistoryStore
}

// IPReputationConfig IP 信誉配置
type IPReputationConfig struct {
	// 评分权重
	ThreatIntelWeight float64 // 外部威胁情报权重 (0-1)
	HistoryWeight     float64 // 历史行为权重 (0-1)
	BehaviorWeight    float64 // 实时行为权重 (0-1)

	// 评分阈值
	TrustedThreshold    float64 // 可信阈值
	SuspiciousThreshold float64 // 可疑阈值
	MaliciousThreshold  float64 // 恶意阈值

	// 时间衰减
	DecayFactor    float64       // 衰减因子 (每小时)
	MaxHistoryDays int           // 最大历史记录天数
	CacheTTL       time.Duration // 缓存过期时间

	// 存储配置
	EnableRedis bool
	RedisAddr   string
}

// IPReputation IP 信誉结果
type IPReputation struct {
	IP               string                 `json:"ip"`
	Score            float64                `json:"score"`              // 0-100, 越高越危险
	RiskLevel        RiskLevel              `json:"risk_level"`         // 风险等级
	Confidence       float64                `json:"confidence"`         // 置信度 0-100
	ThreatIntelScore float64                `json:"threat_intel_score"` // 威胁情报评分
	HistoryScore     float64                `json:"history_score"`      // 历史行为评分
	BehaviorScore    float64                `json:"behavior_score"`     // 实时行为评分
	Categories       []string               `json:"categories"`         // 风险类别
	Evidence         []Evidence             `json:"evidence"`           // 证据列表
	LastUpdated      time.Time              `json:"last_updated"`
	RawData          map[string]interface{} `json:"raw_data,omitempty"`
}

// RiskLevel 风险等级
type RiskLevel string

const (
	RiskLevelTrusted    RiskLevel = "trusted"    // 可信
	RiskLevelNormal     RiskLevel = "normal"     // 正常
	RiskLevelSuspicious RiskLevel = "suspicious" // 可疑
	RiskLevelMalicious  RiskLevel = "malicious"  // 恶意
)

// Evidence 证据
type Evidence struct {
	Source    string    `json:"source"`    // 证据来源
	Type      string    `json:"type"`      // 证据类型
	Value     string    `json:"value"`     // 证据值
	Weight    float64   `json:"weight"`    // 权重
	Timestamp time.Time `json:"timestamp"` // 时间戳
}

// IPHistory IP 历史记录
type IPHistory struct {
	IP            string           `json:"ip"`
	FirstSeen     time.Time        `json:"first_seen"`
	LastSeen      time.Time        `json:"last_seen"`
	TotalRequests int64            `json:"total_requests"`
	ErrorRequests int64            `json:"error_requests"`
	BlockedCount  int64            `json:"blocked_count"`
	ThreatEvents  []ThreatEvent    `json:"threat_events"`
	RequestStats  map[string]int64 `json:"request_stats"` // 按状态码统计
}

// ThreatEvent 威胁事件
type ThreatEvent struct {
	Type      string    `json:"type"`      // 事件类型
	Severity  string    `json:"severity"`  // 严重程度
	Timestamp time.Time `json:"timestamp"` // 发生时间
	Details   string    `json:"details"`   // 详细信息
}

// RequestStats 请求统计 (用于实时行为分析)
type RequestStats struct {
	TotalRequests  int64     `json:"total_requests"`
	ErrorRequests  int64     `json:"error_requests"`
	AvgLatency     float64   `json:"avg_latency"`
	RequestsPerMin float64   `json:"requests_per_min"`
	UniqueURIs     int64     `json:"unique_uris"`
	UniqueMethods  int64     `json:"unique_methods"`
	WindowStart    time.Time `json:"window_start"`
	WindowEnd      time.Time `json:"window_end"`
}

// DefaultIPReputationConfig 返回默认配置
func DefaultIPReputationConfig() *IPReputationConfig {
	return &IPReputationConfig{
		ThreatIntelWeight:   0.5,
		HistoryWeight:       0.3,
		BehaviorWeight:      0.2,
		TrustedThreshold:    20,
		SuspiciousThreshold: 50,
		MaliciousThreshold:  80,
		DecayFactor:         0.01, // 每小时衰减 1%
		MaxHistoryDays:      30,
		CacheTTL:            30 * time.Minute,
		EnableRedis:         false,
	}
}

// NewIPReputationService 创建 IP 信誉评分服务
func NewIPReputationService(config *IPReputationConfig, threatIntel *ThreatIntelClient, logger *zap.Logger) *IPReputationService {
	if config == nil {
		config = DefaultIPReputationConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	service := &IPReputationService{
		config:      config,
		cache:       NewIPReputationCache(config.CacheTTL),
		threatIntel: threatIntel,
		logger:      logger,
		history:     NewIPHistoryStore(),
	}

	// 权重归一化
	totalWeight := config.ThreatIntelWeight + config.HistoryWeight + config.BehaviorWeight
	if totalWeight > 0 {
		service.config.ThreatIntelWeight /= totalWeight
		service.config.HistoryWeight /= totalWeight
		service.config.BehaviorWeight /= totalWeight
	}

	return service
}

// GetReputation 获取 IP 信誉评分
func (s *IPReputationService) GetReputation(ctx context.Context, ip string) (*IPReputation, error) {
	if ip == "" {
		return nil, fmt.Errorf("IP 不能为空")
	}

	// 检查缓存
	if rep, found := s.cache.Get(ip); found {
		s.logger.Debug("IP 信誉缓存命中", zap.String("ip", ip))
		return rep, nil
	}

	// 并行获取各维度评分
	var (
		threatScore      float64
		historyScore     float64
		behaviorScore    float64
		threatCategories []string
		threatEvidence   []Evidence
		mu               sync.Mutex
		wg               sync.WaitGroup
	)

	// 1. 获取外部威胁情报
	wg.Add(1)
	go func() {
		defer wg.Done()
		if s.threatIntel != nil {
			result, err := s.threatIntel.QueryIP(ctx, ip)
			if err == nil && result != nil {
				mu.Lock()
				defer mu.Unlock()
				threatScore = float64(result.RiskScore)
				threatCategories = result.Categories
				if result.IsMalicious {
					threatEvidence = append(threatEvidence, Evidence{
						Source:    result.Provider,
						Type:      "threat_intel",
						Value:     fmt.Sprintf("RiskScore:%d", result.RiskScore),
						Weight:    s.config.ThreatIntelWeight,
						Timestamp: time.Now(),
					})
				}
			}
		}
	}()

	// 2. 获取历史行为评分
	wg.Add(1)
	go func() {
		defer wg.Done()
		history := s.history.Get(ip)
		if history != nil {
			mu.Lock()
			defer mu.Unlock()
			historyScore = s.calculateHistoryScore(history)
		}
	}()

	// 3. 获取实时行为评分 (从窗口统计)
	wg.Add(1)
	go func() {
		defer wg.Done()
		stats := s.getRecentRequestStats(ip)
		if stats != nil {
			mu.Lock()
			defer mu.Unlock()
			behaviorScore = s.calculateBehaviorScore(stats)
		}
	}()

	wg.Wait()

	// 计算综合评分
	finalScore := s.calculateFinalScore(threatScore, historyScore, behaviorScore)
	riskLevel := s.determineRiskLevel(finalScore)
	confidence := s.calculateConfidence(threatEvidence)

	// 构建证据列表
	allEvidence := threatEvidence
	historyEvidence := s.getHistoryEvidence(ip)
	allEvidence = append(allEvidence, historyEvidence...)

	reputation := &IPReputation{
		IP:               ip,
		Score:            finalScore,
		RiskLevel:        riskLevel,
		Confidence:       confidence,
		ThreatIntelScore: threatScore,
		HistoryScore:     historyScore,
		BehaviorScore:    behaviorScore,
		Categories:       threatCategories,
		Evidence:         allEvidence,
		LastUpdated:      time.Now(),
	}

	// 写入缓存
	s.cache.Set(ip, reputation)

	return reputation, nil
}

// RecordEvent 记录 IP 事件
func (s *IPReputationService) RecordEvent(ip string, event IPEvent) {
	s.history.Record(ip, event)
	s.logger.Debug("记录 IP 事件",
		zap.String("ip", ip),
		zap.String("type", event.Type))
}

// IPEvent IP 事件
type IPEvent struct {
	Type       string                 `json:"type"`                  // 事件类型：request/error/block/threat
	StatusCode int                    `json:"status_code"`           // HTTP 状态码
	Latency    float64                `json:"latency"`               // 延迟 ms
	URI        string                 `json:"uri"`                   // 请求 URI
	Method     string                 `json:"method"`                // HTTP 方法
	ThreatType string                 `json:"threat_type,omitempty"` // 威胁类型
	Details    map[string]interface{} `json:"details,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

// calculateHistoryScore 计算历史行为评分
func (s *IPReputationService) calculateHistoryScore(history *IPHistory) float64 {
	if history.TotalRequests == 0 {
		return 0
	}

	score := 0.0

	// 错误率评分 (0-40 分)
	errorRate := float64(history.ErrorRequests) / float64(history.TotalRequests)
	score += errorRate * 40

	// 被封锁次数评分 (0-30 分)
	blockRate := float64(history.BlockedCount) / float64(history.TotalRequests)
	score += blockRate * 30

	// 威胁事件评分 (0-30 分)
	if len(history.ThreatEvents) > 0 {
		threatRate := float64(len(history.ThreatEvents)) / float64(history.TotalRequests) * 100
		if threatRate > 30 {
			threatRate = 30
		}
		score += threatRate
	}

	// 时间衰减
	age := time.Since(history.LastSeen).Hours()
	decay := 1.0 - (age * s.config.DecayFactor)
	if decay < 0.1 {
		decay = 0.1
	}

	return score * decay
}

// calculateBehaviorScore 计算实时行为评分
func (s *IPReputationService) calculateBehaviorScore(stats *RequestStats) float64 {
	score := 0.0

	// 错误率评分 (0-40 分)
	if stats.TotalRequests > 0 {
		errorRate := float64(stats.ErrorRequests) / float64(stats.TotalRequests)
		score += errorRate * 40
	}

	// 请求频率评分 (0-30 分)
	if stats.RequestsPerMin > 100 {
		score += 30
	} else if stats.RequestsPerMin > 50 {
		score += 15
	}

	// 请求多样性评分 (0-30 分)
	// 如果访问大量不同 URI，可能是扫描行为
	if stats.UniqueURIs > 50 {
		score += 30
	} else if stats.UniqueURIs > 20 {
		score += 15
	}

	return score
}

// calculateFinalScore 计算最终评分
func (s *IPReputationService) calculateFinalScore(threatScore, historyScore, behaviorScore float64) float64 {
	finalScore := threatScore*s.config.ThreatIntelWeight +
		historyScore*s.config.HistoryWeight +
		behaviorScore*s.config.BehaviorWeight

	if finalScore > 100 {
		finalScore = 100
	}
	if finalScore < 0 {
		finalScore = 0
	}

	return finalScore
}

// determineRiskLevel 确定风险等级
func (s *IPReputationService) determineRiskLevel(score float64) RiskLevel {
	if score >= s.config.MaliciousThreshold {
		return RiskLevelMalicious
	}
	if score >= s.config.SuspiciousThreshold {
		return RiskLevelSuspicious
	}
	if score <= s.config.TrustedThreshold {
		return RiskLevelTrusted
	}
	return RiskLevelNormal
}

// calculateConfidence 计算置信度
func (s *IPReputationService) calculateConfidence(evidence []Evidence) float64 {
	if len(evidence) == 0 {
		return 0
	}

	// 基于证据数量和质量计算置信度
	baseConfidence := float64(len(evidence)) * 20
	if baseConfidence > 100 {
		baseConfidence = 100
	}

	// 证据权重加成
	weightSum := 0.0
	for _, e := range evidence {
		weightSum += e.Weight
	}

	weightBonus := weightSum * 10
	if weightBonus > 20 {
		weightBonus = 20
	}

	confidence := baseConfidence + weightBonus
	if confidence > 100 {
		confidence = 100
	}

	return confidence
}

// getHistoryEvidence 获取历史证据
func (s *IPReputationService) getHistoryEvidence(ip string) []Evidence {
	history := s.history.Get(ip)
	if history == nil {
		return nil
	}

	evidence := make([]Evidence, 0)

	if history.BlockedCount > 0 {
		evidence = append(evidence, Evidence{
			Source:    "history",
			Type:      "blocked_count",
			Value:     fmt.Sprintf("%d", history.BlockedCount),
			Weight:    s.config.HistoryWeight,
			Timestamp: history.LastSeen,
		})
	}

	if len(history.ThreatEvents) > 0 {
		evidence = append(evidence, Evidence{
			Source:    "history",
			Type:      "threat_events",
			Value:     fmt.Sprintf("%d events", len(history.ThreatEvents)),
			Weight:    s.config.HistoryWeight,
			Timestamp: history.ThreatEvents[len(history.ThreatEvents)-1].Timestamp,
		})
	}

	return evidence
}

// getRecentRequestStats 获取最近的请求统计
func (s *IPReputationService) getRecentRequestStats(ip string) *RequestStats {
	return s.history.GetRecentStats(ip, 5*time.Minute)
}

// GetIPHistory 获取 IP 历史记录
func (s *IPReputationService) GetIPHistory(ip string) *IPHistory {
	return s.history.Get(ip)
}

// ClearCache 清除缓存
func (s *IPReputationService) ClearCache() {
	s.cache.Clear()
}

// GetStats 获取统计信息
func (s *IPReputationService) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"cache_size":  s.cache.Size(),
		"history_ips": s.history.Count(),
		"config": map[string]interface{}{
			"threat_weight":        s.config.ThreatIntelWeight,
			"history_weight":       s.config.HistoryWeight,
			"behavior_weight":      s.config.BehaviorWeight,
			"malicious_threshold":  s.config.MaliciousThreshold,
			"suspicious_threshold": s.config.SuspiciousThreshold,
		},
	}
}

// IPReputationCache IP 信誉缓存
type IPReputationCache struct {
	data       map[string]*IPReputation
	mu         sync.RWMutex
	defaultTTL time.Duration
}

// NewIPReputationCache 创建 IP 信誉缓存
func NewIPReputationCache(ttl time.Duration) *IPReputationCache {
	cache := &IPReputationCache{
		data:       make(map[string]*IPReputation),
		defaultTTL: ttl,
	}

	// 启动清理协程
	go cache.cleanup()

	return cache
}

// Get 获取缓存
func (c *IPReputationCache) Get(ip string) (*IPReputation, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	rep, ok := c.data[ip]
	if !ok {
		return nil, false
	}

	// 检查是否过期
	if time.Since(rep.LastUpdated) > c.defaultTTL {
		return nil, false
	}

	return rep, true
}

// Set 设置缓存
func (c *IPReputationCache) Set(ip string, rep *IPReputation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[ip] = rep
}

// Clear 清空缓存
func (c *IPReputationCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]*IPReputation)
}

// Size 获取缓存大小
func (c *IPReputationCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}

// cleanup 定期清理过期缓存
func (c *IPReputationCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for ip, rep := range c.data {
			if now.Sub(rep.LastUpdated) > c.defaultTTL {
				delete(c.data, ip)
			}
		}
		c.mu.Unlock()
	}
}

// IPHistoryStore IP 历史记录存储
type IPHistoryStore struct {
	data  map[string]*IPHistory
	mu    sync.RWMutex
	stats map[string][]RequestStats // 按 IP 存储请求统计窗口
}

// NewIPHistoryStore 创建 IP 历史记录存储
func NewIPHistoryStore() *IPHistoryStore {
	store := &IPHistoryStore{
		data:  make(map[string]*IPHistory),
		stats: make(map[string][]RequestStats),
	}

	// 启动清理协程
	go store.cleanup()

	return store
}

// Get 获取 IP 历史记录
func (s *IPHistoryStore) Get(ip string) *IPHistory {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[ip]
}

// Record 记录 IP 事件
func (s *IPHistoryStore) Record(ip string, event IPEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	history, exists := s.data[ip]
	if !exists {
		history = &IPHistory{
			IP:           ip,
			FirstSeen:    event.Timestamp,
			RequestStats: make(map[string]int64),
		}
		s.data[ip] = history
	}

	history.LastSeen = event.Timestamp
	history.TotalRequests++

	if event.StatusCode >= 400 {
		history.ErrorRequests++
	}

	// 更新状态码统计
	statusKey := fmt.Sprintf("%d", event.StatusCode)
	history.RequestStats[statusKey]++

	// 记录威胁事件
	if event.Type == "threat" || event.Type == "block" {
		history.BlockedCount++
		history.ThreatEvents = append(history.ThreatEvents, ThreatEvent{
			Type:      event.Type,
			Severity:  getEventSeverity(event.Type),
			Timestamp: event.Timestamp,
			Details:   event.ThreatType,
		})

		// 限制威胁事件数量
		if len(history.ThreatEvents) > 100 {
			history.ThreatEvents = history.ThreatEvents[1:]
		}
	}

	// 更新请求统计窗口
	s.updateRequestStats(ip, event)
}

// GetRecentStats 获取最近的请求统计
func (s *IPHistoryStore) GetRecentStats(ip string, window time.Duration) *RequestStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statsList, exists := s.stats[ip]
	if !exists || len(statsList) == 0 {
		return nil
	}

	// 聚合窗口内的统计
	now := time.Now()
	windowStart := now.Add(-window)

	var aggregated RequestStats
	aggregated.WindowStart = windowStart
	aggregated.WindowEnd = now

	for _, stats := range statsList {
		if stats.WindowEnd.After(windowStart) {
			aggregated.TotalRequests += stats.TotalRequests
			aggregated.ErrorRequests += stats.ErrorRequests
			aggregated.UniqueURIs += stats.UniqueURIs
			aggregated.UniqueMethods += stats.UniqueMethods

			// 加权平均延迟
			if aggregated.TotalRequests > 0 {
				aggregated.AvgLatency = (aggregated.AvgLatency*float64(aggregated.TotalRequests-stats.TotalRequests) +
					stats.AvgLatency*float64(stats.TotalRequests)) / float64(aggregated.TotalRequests)
			}
		}
	}

	// 计算每分钟请求数
	duration := window.Minutes()
	if duration > 0 {
		aggregated.RequestsPerMin = float64(aggregated.TotalRequests) / duration
	}

	return &aggregated
}

// updateRequestStats 更新请求统计
func (s *IPHistoryStore) updateRequestStats(ip string, event IPEvent) {
	windowSize := 1 * time.Minute

	// 获取或创建当前窗口
	var currentStats *RequestStats
	statsList := s.stats[ip]
	if len(statsList) > 0 {
		lastStats := &statsList[len(statsList)-1]
		if time.Since(lastStats.WindowStart) < windowSize {
			currentStats = lastStats
		}
	}

	// 创建新窗口
	if currentStats == nil {
		currentStats = &RequestStats{
			WindowStart: time.Now(),
			WindowEnd:   time.Now(),
		}
		s.stats[ip] = append(s.stats[ip], *currentStats)
		currentStats = &s.stats[ip][len(s.stats[ip])-1]
	}

	// 更新统计
	currentStats.TotalRequests++
	if event.StatusCode >= 400 {
		currentStats.ErrorRequests++
	}
	currentStats.WindowEnd = time.Now()

	// 更新平均延迟
	currentStats.AvgLatency = (currentStats.AvgLatency*float64(currentStats.TotalRequests-1) +
		event.Latency) / float64(currentStats.TotalRequests)
}

// Count 获取存储的 IP 数量
func (s *IPHistoryStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// cleanup 定期清理过期数据
func (s *IPHistoryStore) cleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		maxAge := 30 * 24 * time.Hour // 30 天

		for ip, history := range s.data {
			if now.Sub(history.LastSeen) > maxAge {
				delete(s.data, ip)
				delete(s.stats, ip)
			}
		}
		s.mu.Unlock()
	}
}

func getEventSeverity(eventType string) string {
	switch eventType {
	case "threat":
		return "high"
	case "block":
		return "medium"
	case "error":
		return "low"
	default:
		return "info"
	}
}

// MarshalJSON 序列化 IPReputation
func (r *IPReputation) MarshalJSON() ([]byte, error) {
	type Alias IPReputation
	return json.Marshal(&struct {
		*Alias
		RiskLevel string `json:"risk_level"`
	}{
		Alias:     (*Alias)(r),
		RiskLevel: string(r.RiskLevel),
	})
}
