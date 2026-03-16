package zerotrust

import (
	"context"
	"crypto/rand"
	"math"
	"math/big"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ContinuousVerificationEngine 持续验证引擎
type ContinuousVerificationEngine struct {
	config     *ContinuousVerificationConfig
	logger     *zap.Logger
	sessions   *SessionStore
	behaviorDB *BehaviorDB
	mu         sync.RWMutex
	stats      *VerificationStats
}

// ContinuousVerificationConfig 持续验证配置
type ContinuousVerificationConfig struct {
	// 会话配置
	SessionTimeout     time.Duration `json:"session_timeout"`
	SessionIdleTimeout time.Duration `json:"session_idle_timeout"`
	MaxSessionDuration time.Duration `json:"max_session_duration"`

	// 验证配置
	VerificationInterval     time.Duration `json:"verification_interval"` // 定期验证间隔
	ReverifyOnIPChange       bool          `json:"reverify_on_ip_change"`
	ReverifyOnUAChange       bool          `json:"reverify_on_ua_change"`
	ReverifyOnLocationChange bool          `json:"reverify_on_location_change"`

	// 行为分析配置
	EnableBehaviorAnalysis bool          `json:"enable_behavior_analysis"`
	BehaviorWindow         time.Duration `json:"behavior_window"`
	AnomalyThreshold       float64       `json:"anomaly_threshold"`

	// 信任配置
	TrustDecayRate float64 `json:"trust_decay_rate"`
	TrustBoostRate float64 `json:"trust_boost_rate"`
	MinTrustScore  float64 `json:"min_trust_score"`
	MaxTrustScore  float64 `json:"max_trust_score"`

	// 阈值
	SuspiciousThreshold float64 `json:"suspicious_threshold"`
	BlockThreshold      float64 `json:"block_threshold"`

	// 缓存配置
	CacheSize int           `json:"cache_size"`
	CacheTTL  time.Duration `json:"cache_ttl"`
}

// DefaultContinuousVerificationConfig 返回默认配置
func DefaultContinuousVerificationConfig() *ContinuousVerificationConfig {
	return &ContinuousVerificationConfig{
		SessionTimeout:           24 * time.Hour,
		SessionIdleTimeout:       30 * time.Minute,
		MaxSessionDuration:       7 * 24 * time.Hour,
		VerificationInterval:     5 * time.Minute,
		ReverifyOnIPChange:       true,
		ReverifyOnUAChange:       true,
		ReverifyOnLocationChange: true,
		EnableBehaviorAnalysis:   true,
		BehaviorWindow:           1 * time.Hour,
		AnomalyThreshold:         0.7,
		TrustDecayRate:           0.01,
		TrustBoostRate:           0.05,
		MinTrustScore:            0,
		MaxTrustScore:            100,
		SuspiciousThreshold:      40,
		BlockThreshold:           20,
		CacheSize:                10000,
		CacheTTL:                 1 * time.Hour,
	}
}

// SessionStore 会话存储
type SessionStore struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	ttl      time.Duration
}

// Session 会话
type Session struct {
	SessionID         string                 `json:"session_id"`
	UserID            string                 `json:"user_id"`
	DeviceID          string                 `json:"device_id"`
	IP                string                 `json:"ip"`
	UserAgent         string                 `json:"user_agent"`
	Location          *Location              `json:"location,omitempty"`
	StartTime         time.Time              `json:"start_time"`
	LastActivity      time.Time              `json:"last_activity"`
	LastVerification  time.Time              `json:"last_verification"`
	TrustScore        float64                `json:"trust_score"`
	RiskScore         float64                `json:"risk_score"`
	BehaviorFlags     []string               `json:"behavior_flags,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	IsValid           bool                   `json:"is_valid"`
	IsVerified        bool                   `json:"is_verified"`
	VerificationCount int64                  `json:"verification_count"`
}

// BehaviorDB 行为数据库
type BehaviorDB struct {
	_behaviors map[string]*UserBehavior
	mu         sync.RWMutex
}

// UserBehavior 用户行为
type UserBehavior struct {
	UserID        string      `json:"user_id"`
	DeviceID      string      `json:"device_id"`
	RequestTimes  []time.Time `json:"request_times"`
	URIs          []string    `json:"uris,omitempty"`
	Methods       []string    `json:"methods,omitempty"`
	ResponseTimes []int64     `json:"response_times,omitempty"` // 毫秒
	AvgRPM        float64     `json:"avg_rpm"`                  // 平均每分钟请求数
	MaxRPM        float64     `json:"max_rpm"`
	LastUpdated   time.Time   `json:"last_updated"`
}

// VerificationStats 验证统计
type VerificationStats struct {
	TotalSessions      int64 `json:"total_sessions"`
	ActiveSessions     int64 `json:"active_sessions"`
	VerifiedSessions   int64 `json:"verified_sessions"`
	SuspiciousSessions int64 `json:"suspicious_sessions"`
	BlockedSessions    int64 `json:"blocked_sessions"`
	IPChanges          int64 `json:"ip_changes"`
	UAChanges          int64 `json:"ua_changes"`
	LocationChanges    int64 `json:"location_changes"`
	BehaviorAnomalies  int64 `json:"behavior_anomalies"`
	TimeoutSessions    int64 `json:"timeout_sessions"`
}

// NewContinuousVerificationEngine 创建持续验证引擎
func NewContinuousVerificationEngine(config *ContinuousVerificationConfig, logger *zap.Logger) *ContinuousVerificationEngine {
	if config == nil {
		config = DefaultContinuousVerificationConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	engine := &ContinuousVerificationEngine{
		config:     config,
		logger:     logger,
		sessions:   NewSessionStore(config.CacheSize, config.CacheTTL),
		behaviorDB: NewBehaviorDB(),
		stats:      &VerificationStats{},
	}

	// 启动监控协程
	go engine.monitorWorker()

	return engine
}

// NewSessionStore 创建会话存储
func NewSessionStore(size int, ttl time.Duration) *SessionStore {
	store := &SessionStore{
		sessions: make(map[string]*Session, size),
		ttl:      ttl,
	}

	// 启动清理协程
	go store.cleanupWorker()

	return store
}

// NewBehaviorDB 创建行为数据库
func NewBehaviorDB() *BehaviorDB {
	return &BehaviorDB{
		_behaviors: make(map[string]*UserBehavior),
	}
}

// CreateSession 创建会话
func (e *ContinuousVerificationEngine) CreateSession(
	ctx context.Context,
	userID string,
	deviceID string,
	ip string,
	userAgent string,
	location *Location,
) *Session {
	sessionID := e.generateSessionID()

	session := &Session{
		SessionID:         sessionID,
		UserID:            userID,
		DeviceID:          deviceID,
		IP:                ip,
		UserAgent:         userAgent,
		Location:          location,
		StartTime:         time.Now(),
		LastActivity:      time.Now(),
		LastVerification:  time.Now(),
		TrustScore:        50.0,
		RiskScore:         0.0,
		BehaviorFlags:     make([]string, 0),
		Metadata:          make(map[string]interface{}),
		IsValid:           true,
		IsVerified:        false,
		VerificationCount: 0,
	}

	e.sessions.Set(sessionID, session)
	e.stats.TotalSessions++
	e.stats.ActiveSessions++

	e.logger.Info("创建会话",
		zap.String("session_id", sessionID),
		zap.String("user_id", userID),
		zap.String("device_id", deviceID),
		zap.String("ip", ip),
	)

	return session
}

// VerifySession 验证会话
func (e *ContinuousVerificationEngine) VerifySession(
	ctx context.Context,
	sessionID string,
	currentIP string,
	currentUA string,
	currentLocation *Location,
) *VerificationResult {
	session := e.sessions.Get(sessionID)
	if session == nil {
		return &VerificationResult{
			Valid:  false,
			Reason: "session_not_found",
			Action: "reject",
		}
	}

	result := &VerificationResult{
		Valid:      true,
		Action:     "allow",
		TrustScore: session.TrustScore,
		RiskScore:  session.RiskScore,
	}

	// 1. 检查会话是否有效
	if !session.IsValid {
		result.Valid = false
		result.Reason = "session_invalid"
		result.Action = "reject"
		return result
	}

	// 2. 检查会话超时
	if time.Now().After(session.StartTime.Add(e.config.MaxSessionDuration)) {
		session.IsValid = false
		e.stats.TimeoutSessions++
		result.Valid = false
		result.Reason = "session_expired"
		result.Action = "reauth"
		return result
	}

	// 3. 检查空闲超时
	if time.Now().After(session.LastActivity.Add(e.config.SessionIdleTimeout)) {
		session.IsValid = false
		e.stats.TimeoutSessions++
		result.Valid = false
		result.Reason = "session_idle_timeout"
		result.Action = "reverify"
		return result
	}

	// 4. 检查 IP 变化
	if e.config.ReverifyOnIPChange && session.IP != currentIP {
		session.BehaviorFlags = append(session.BehaviorFlags, "ip_changed")
		e.stats.IPChanges++
		result.Flags = append(result.Flags, "ip_changed")
		result.TrustScore -= 15
		result.ReverifyRequired = true

		e.logger.Warn("检测到 IP 变化",
			zap.String("session_id", sessionID),
			zap.String("old_ip", session.IP),
			zap.String("new_ip", currentIP),
		)
	}

	// 5. 检查 User-Agent 变化
	if e.config.ReverifyOnUAChange && session.UserAgent != currentUA {
		session.BehaviorFlags = append(session.BehaviorFlags, "ua_changed")
		e.stats.UAChanges++
		result.Flags = append(result.Flags, "ua_changed")
		result.TrustScore -= 10
		result.ReverifyRequired = true

		e.logger.Warn("检测到 User-Agent 变化",
			zap.String("session_id", sessionID),
		)
	}

	// 6. 检查位置变化
	if e.config.ReverifyOnLocationChange && currentLocation != nil && session.Location != nil {
		distance := calculateDistance(
			session.Location.Lat, session.Location.Lon,
			currentLocation.Lat, currentLocation.Lon,
		)
		if distance > 100 { // 100km 阈值
			session.BehaviorFlags = append(session.BehaviorFlags, "location_changed")
			e.stats.LocationChanges++
			result.Flags = append(result.Flags, "location_changed")
			result.TrustScore -= 20
			result.ReverifyRequired = true

			e.logger.Warn("检测到位置变化",
				zap.String("session_id", sessionID),
				zap.Float64("distance_km", distance),
			)
		}
	}

	// 7. 行为分析
	if e.config.EnableBehaviorAnalysis {
		anomalyScore := e.analyzeBehavior(session.UserID, session.DeviceID)
		if anomalyScore > e.config.AnomalyThreshold {
			session.BehaviorFlags = append(session.BehaviorFlags, "behavior_anomaly")
			e.stats.BehaviorAnomalies++
			result.Flags = append(result.Flags, "behavior_anomaly")
			result.TrustScore -= 25
		}
	}

	// 更新会话
	session.LastActivity = time.Now()
	session.TrustScore = result.TrustScore

	// 确定风险等级和风险分数
	session.RiskScore = 100 - session.TrustScore
	result.RiskScore = session.RiskScore

	// 根据信任分数决定动作
	if session.TrustScore < e.config.BlockThreshold {
		session.IsValid = false
		result.Valid = false
		result.Reason = "trust_too_low"
		result.Action = "block"
		e.stats.BlockedSessions++
	} else if session.TrustScore < e.config.SuspiciousThreshold {
		result.Valid = true
		result.Reason = "suspicious"
		result.Action = "challenge"
		e.stats.SuspiciousSessions++
	} else if result.ReverifyRequired {
		result.Action = "reverify"
	}

	session.VerificationCount++
	session.LastVerification = time.Now()

	// 更新统计
	if session.IsVerified {
		e.stats.VerifiedSessions++
	}

	e.logger.Debug("会话验证完成",
		zap.String("session_id", sessionID),
		zap.Bool("valid", result.Valid),
		zap.String("action", result.Action),
		zap.Float64("trust_score", result.TrustScore),
	)

	return result
}

// RecordActivity 记录活动
func (e *ContinuousVerificationEngine) RecordActivity(
	ctx context.Context,
	sessionID string,
	uri string,
	method string,
	responseTimeMs int64,
) {
	session := e.sessions.Get(sessionID)
	if session == nil {
		return
	}

	session.LastActivity = time.Now()

	// 记录行为
	e.behaviorDB.Record(session.UserID, session.DeviceID, uri, method, responseTimeMs)
}

// UpdateTrust 更新信任分数
func (e *ContinuousVerificationEngine) UpdateTrust(sessionID string, delta float64, reason string) {
	session := e.sessions.Get(sessionID)
	if session == nil {
		return
	}

	session.TrustScore += delta
	if session.TrustScore > e.config.MaxTrustScore {
		session.TrustScore = e.config.MaxTrustScore
	}
	if session.TrustScore < e.config.MinTrustScore {
		session.TrustScore = e.config.MinTrustScore
	}

	e.logger.Debug("更新信任分数",
		zap.String("session_id", sessionID),
		zap.Float64("delta", delta),
		zap.String("reason", reason),
		zap.Float64("new_score", session.TrustScore),
	)
}

// analyzeBehavior 分析行为
func (e *ContinuousVerificationEngine) analyzeBehavior(userID, deviceID string) float64 {
	behavior := e.behaviorDB.Get(userID, deviceID)
	if behavior == nil {
		return 0
	}

	anomalyScore := 0.0

	// 1. 检查请求频率
	if behavior.AvgRPM > 100 { // 每分钟超过 100 请求
		anomalyScore += 0.3
	}

	// 2. 检查请求模式突变
	if len(behavior.RequestTimes) > 10 {
		// 检查是否有突发流量
		recentCount := 0
		now := time.Now()
		for _, t := range behavior.RequestTimes {
			if now.Sub(t) < 1*time.Minute {
				recentCount++
			}
		}
		if recentCount > int(behavior.AvgRPM*2) {
			anomalyScore += 0.4
		}
	}

	return anomalyScore
}

// monitorWorker 监控协程
func (e *ContinuousVerificationEngine) monitorWorker() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		e.monitor()
	}
}

// monitor 监控所有会话
func (e *ContinuousVerificationEngine) monitor() {
	sessions := e.sessions.List()
	for _, session := range sessions {
		// 应用信任衰减
		if session.TrustScore > 50 {
			session.TrustScore -= e.config.TrustDecayRate
		}

		// 检查是否过期
		if time.Now().After(session.StartTime.Add(e.config.MaxSessionDuration)) {
			session.IsValid = false
		}
	}
}

// Get 获取会话
func (s *SessionStore) Get(sessionID string) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[sessionID]
}

// Set 设置会话
func (s *SessionStore) Set(sessionID string, session *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = session
}

// Delete 删除会话
func (s *SessionStore) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

// List 列出所有会话
func (s *SessionStore) List() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		result = append(result, session)
	}
	return result
}

// cleanupWorker 清理工
func (s *SessionStore) cleanupWorker() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanup()
	}
}

// cleanup 清理过期会话
func (s *SessionStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	deleted := 0

	for id, session := range s.sessions {
		if now.After(session.StartTime.Add(24 * time.Hour)) {
			delete(s.sessions, id)
			deleted++
		}
	}

	// 日志在调用者中记录
	_ = deleted
}

// Record 记录行为
func (db *BehaviorDB) Record(userID, deviceID, uri, method string, responseTimeMs int64) {
	db.mu.Lock()
	defer db.mu.Unlock()

	key := userID + "|" + deviceID
	behavior, exists := db._behaviors[key]

	if !exists {
		behavior = &UserBehavior{
			UserID:        userID,
			DeviceID:      deviceID,
			RequestTimes:  make([]time.Time, 0),
			URIs:          make([]string, 0),
			Methods:       make([]string, 0),
			ResponseTimes: make([]int64, 0),
			LastUpdated:   time.Now(),
		}
		db._behaviors[key] = behavior
	}

	// 添加记录
	behavior.RequestTimes = append(behavior.RequestTimes, time.Now())
	behavior.URIs = append(behavior.URIs, uri)
	behavior.Methods = append(behavior.Methods, method)
	behavior.ResponseTimes = append(behavior.ResponseTimes, responseTimeMs)

	// 限制历史记录大小
	maxHistory := 1000
	if len(behavior.RequestTimes) > maxHistory {
		behavior.RequestTimes = behavior.RequestTimes[len(behavior.RequestTimes)-maxHistory:]
	}
	if len(behavior.URIs) > maxHistory {
		behavior.URIs = behavior.URIs[len(behavior.URIs)-maxHistory:]
	}
	if len(behavior.Methods) > maxHistory {
		behavior.Methods = behavior.Methods[len(behavior.Methods)-maxHistory:]
	}
	if len(behavior.ResponseTimes) > maxHistory {
		behavior.ResponseTimes = behavior.ResponseTimes[len(behavior.ResponseTimes)-maxHistory:]
	}

	// 计算平均 RPM
	if len(behavior.RequestTimes) > 1 {
		firstTime := behavior.RequestTimes[0]
		lastTime := behavior.RequestTimes[len(behavior.RequestTimes)-1]
		duration := lastTime.Sub(firstTime).Minutes()
		if duration > 0 {
			behavior.AvgRPM = float64(len(behavior.RequestTimes)) / duration
		}
	}

	behavior.LastUpdated = time.Now()
}

// Get 获取行为
func (db *BehaviorDB) Get(userID, deviceID string) *UserBehavior {
	db.mu.RLock()
	defer db.mu.RUnlock()
	key := userID + "|" + deviceID
	return db._behaviors[key]
}

// VerificationResult 验证结果
type VerificationResult struct {
	Valid            bool     `json:"valid"`
	Reason           string   `json:"reason,omitempty"`
	Action           string   `json:"action"` // allow, challenge, reverify, block, reject
	TrustScore       float64  `json:"trust_score"`
	RiskScore        float64  `json:"risk_score"`
	Flags            []string `json:"flags,omitempty"`
	ReverifyRequired bool     `json:"reverify_required"`
	ChallengeType    string   `json:"challenge_type,omitempty"`
}

// generateSessionID 生成会话 ID
func (e *ContinuousVerificationEngine) generateSessionID() string {
	return "sess_" + time.Now().Format("20060102150405") + "_" + randomString(8)
}

// calculateDistance 计算两点间距离（Haversine 公式）
func calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371 // 地球半径 km

	dLat := deg2rad(lat2 - lat1)
	dLon := deg2rad(lon2 - lon1)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(deg2rad(lat1))*math.Cos(deg2rad(lat2))*
			math.Sin(dLon/2)*math.Sin(dLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

func deg2rad(deg float64) float64 {
	return deg * math.Pi / 180
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			// fallback to insecure random if crypto/rand fails
			num = big.NewInt(int64(time.Now().Nanosecond() % len(letters)))
		}
		b[i] = letters[num.Int64()]
	}
	return string(b)
}

// GetStats 获取统计
func (e *ContinuousVerificationEngine) GetStats() *VerificationStats {
	e.stats.ActiveSessions = int64(len(e.sessions.sessions))
	return e.stats
}

// InvalidateSession 使会话失效
func (e *ContinuousVerificationEngine) InvalidateSession(sessionID string) {
	session := e.sessions.Get(sessionID)
	if session != nil {
		session.IsValid = false
	}
}

// GetSession 获取会话
func (e *ContinuousVerificationEngine) GetSession(sessionID string) *Session {
	return e.sessions.Get(sessionID)
}

// CloseSession 关闭会话
func (e *ContinuousVerificationEngine) CloseSession(sessionID string) {
	e.sessions.Delete(sessionID)
}
