package behavioranalyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// UEBAEngine 用户和实体行为分析引擎
type UEBAEngine struct {
	config       *UEBAConfig
	userSessions map[string]*UserSession
	riskEngine   *RiskEngine
	patternDB    *PatternDatabase
	mu           sync.RWMutex
	logger       *zap.Logger
	stats        *UEBAStats
}

// UEBAConfig UEBA 配置
type UEBAConfig struct {
	// 会话配置
	SessionTimeout    time.Duration // 会话超时时间
	MaxSessionHistory int           // 最大会话历史数

	// 风险评分配置
	RiskScoreWeights *RiskScoreWeights

	// 检测配置
	EnableSequenceAnalysis   bool    // 启用序列分析
	EnableLateralMovement    bool    // 启用横向移动检测
	EnableInsiderThreat      bool    // 启用内部威胁检测
	MinSequenceLength        int     // 最小序列长度
	LateralMovementThreshold int     // 横向移动阈值
	InsiderThreatThreshold   float64 // 内部威胁阈值

	// 群体基线配置
	EnablePeerGrouping bool // 启用群体分析
	MinGroupSize       int  // 最小群体大小

	// 存储配置
	MaxUsers int
	CacheTTL time.Duration
}

// RiskScoreWeights 风险评分权重
type RiskScoreWeights struct {
	ThreatIntel         float64 // 威胁情报权重
	BehaviorAnomaly     float64 // 行为异常权重
	SequenceAnomaly     float64 // 序列异常权重
	PeerAnomaly         float64 // 群体偏离权重
	PrivilegeEscalation float64 // 权限提升权重
	LateralMovement     float64 // 横向移动权重
}

// UserSession 用户会话
type UserSession struct {
	UserID     string                 `json:"user_id"`
	IP         string                 `json:"ip"`
	Start      time.Time              `json:"start"`
	LastSeen   time.Time              `json:"last_seen"`
	Events     []SessionEvent         `json:"events"`
	EventCount int64                  `json:"event_count"`
	RiskScore  float64                `json:"risk_score"`
	RiskLevel  RiskLevel              `json:"risk_level"`
	Flags      []string               `json:"flags"`
	Metadata   map[string]interface{} `json:"metadata"`
	mu         sync.RWMutex
}

// SessionEvent 会话事件
type SessionEvent struct {
	Timestamp  time.Time              `json:"timestamp"`
	Type       string                 `json:"type"`
	URI        string                 `json:"uri"`
	Method     string                 `json:"method"`
	StatusCode int                    `json:"status_code"`
	SourceIP   string                 `json:"source_ip"`
	UserAgent  string                 `json:"user_agent"`
	ResourceID string                 `json:"resource_id,omitempty"`
	Action     string                 `json:"action,omitempty"`
	Result     string                 `json:"result,omitempty"`
	Extra      map[string]interface{} `json:"extra,omitempty"`
}

// RiskEngine 风险评分引擎
type RiskEngine struct {
	config    *UEBAConfig
	userRisks map[string]*UserRisk
	mu        sync.RWMutex
}

// UserRisk 用户风险
type UserRisk struct {
	UserID        string       `json:"user_id"`
	BaseRisk      float64      `json:"base_risk"`      // 基础风险
	ThreatRisk    float64      `json:"threat_risk"`    // 威胁情报风险
	BehaviorRisk  float64      `json:"behavior_risk"`  // 行为异常风险
	SequenceRisk  float64      `json:"sequence_risk"`  // 序列异常风险
	PeerRisk      float64      `json:"peer_risk"`      // 群体偏离风险
	PrivilegeRisk float64      `json:"privilege_risk"` // 权限提升风险
	LateralRisk   float64      `json:"lateral_risk"`   // 横向移动风险
	TotalRisk     float64      `json:"total_risk"`
	RiskFactors   []RiskFactor `json:"risk_factors"`
	LastUpdated   time.Time    `json:"last_updated"`
}

// RiskFactor 风险因素
type RiskFactor struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"`
	Score       float64   `json:"score"`
	Timestamp   time.Time `json:"timestamp"`
}

// PatternDatabase 行为模式数据库
type PatternDatabase struct {
	patterns       map[string]*BehaviorPattern
	sequenceModels map[string]*SequenceModel
	mu             sync.RWMutex
}

// BehaviorPattern 行为模式
type BehaviorPattern struct {
	PatternID   string                 `json:"pattern_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Conditions  []PatternCondition     `json:"conditions"`
	RiskScore   float64                `json:"risk_score"`
	Count       int64                  `json:"count"`
	LastMatch   time.Time              `json:"last_match"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// PatternCondition 模式条件
type PatternCondition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"` // eq, contains, regex, gt, lt
	Value    interface{} `json:"value"`
}

// SequenceModel 序列模型
type SequenceModel struct {
	UserID       string           `json:"user_id"`
	Transitions  map[string]int64 `json:"transitions"` // state -> count
	States       []string         `json:"states"`
	CurrentState string           `json:"current_state"`
	AnomalyScore float64          `json:"anomaly_score"`
	LastUpdated  time.Time        `json:"last_updated"`
}

// UEBAStats UEBA 统计
type UEBAStats struct {
	TotalUsers      int64 `json:"total_users"`
	ActiveUsers     int64 `json:"active_users"`
	HighRiskUsers   int64 `json:"high_risk_users"`
	TotalSessions   int64 `json:"total_sessions"`
	ActiveSessions  int64 `json:"active_sessions"`
	PatternsMatched int64 `json:"patterns_matched"`
	AlertsGenerated int64 `json:"alerts_generated"`
}

// Alert UEBA 告警
type Alert struct {
	AlertID     string                 `json:"alert_id"`
	UserID      string                 `json:"user_id"`
	AlertType   string                 `json:"alert_type"`
	Severity    string                 `json:"severity"`
	Score       float64                `json:"score"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Evidence    []Evidence             `json:"evidence"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// DefaultUEBAConfig 返回默认配置
func DefaultUEBAConfig() *UEBAConfig {
	return &UEBAConfig{
		SessionTimeout:           30 * time.Minute,
		MaxSessionHistory:        1000,
		EnableSequenceAnalysis:   true,
		EnableLateralMovement:    true,
		EnableInsiderThreat:      true,
		MinSequenceLength:        5,
		LateralMovementThreshold: 10,
		InsiderThreatThreshold:   0.7,
		EnablePeerGrouping:       true,
		MinGroupSize:             10,
		MaxUsers:                 100000,
		CacheTTL:                 1 * time.Hour,
		RiskScoreWeights: &RiskScoreWeights{
			ThreatIntel:         0.2,
			BehaviorAnomaly:     0.2,
			SequenceAnomaly:     0.15,
			PeerAnomaly:         0.15,
			PrivilegeEscalation: 0.15,
			LateralMovement:     0.15,
		},
	}
}

// NewUEBAEngine 创建 UEBA 引擎
func NewUEBAEngine(config *UEBAConfig, logger *zap.Logger) *UEBAEngine {
	if config == nil {
		config = DefaultUEBAConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	engine := &UEBAEngine{
		config:       config,
		userSessions: make(map[string]*UserSession),
		riskEngine:   NewRiskEngine(config, logger),
		patternDB:    NewPatternDatabase(),
		logger:       logger,
		stats:        &UEBAStats{},
	}

	// 启动后台清理协程
	go engine.cleanupWorker()

	return engine
}

// NewRiskEngine 创建风险评分引擎
func NewRiskEngine(config *UEBAConfig, logger *zap.Logger) *RiskEngine {
	return &RiskEngine{
		config:    config,
		userRisks: make(map[string]*UserRisk),
	}
}

// NewPatternDatabase 创建模式数据库
func NewPatternDatabase() *PatternDatabase {
	db := &PatternDatabase{
		patterns:       make(map[string]*BehaviorPattern),
		sequenceModels: make(map[string]*SequenceModel),
	}

	// 注册默认模式
	db.registerDefaultPatterns()

	return db
}

// ProcessEvent 处理事件
func (e *UEBAEngine) ProcessEvent(ctx context.Context, event SessionEvent) *Alert {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 获取或创建会话
	session := e.getOrCreateSession(event)

	// 添加事件到会话
	session.addEvent(event)

	// 更新风险评分
	alert := e.riskEngine.UpdateRisk(session, event)

	// 检测行为模式
	if patternAlert := e.detectPatterns(session, event); patternAlert != nil {
		alert = patternAlert
	}

	// 检测序列异常
	if e.config.EnableSequenceAnalysis {
		if seqAlert := e.detectSequenceAnomaly(session, event); seqAlert != nil {
			alert = seqAlert
		}
	}

	// 检测横向移动
	if e.config.EnableLateralMovement {
		if latAlert := e.detectLateralMovement(session, event); latAlert != nil {
			alert = latAlert
		}
	}

	// 检测内部威胁
	if e.config.EnableInsiderThreat {
		if insAlert := e.detectInsiderThreat(session, event); insAlert != nil {
			alert = insAlert
		}
	}

	// 更新统计
	e.updateStats(alert)

	return alert
}

// GetSession 获取用户会话
func (e *UEBAEngine) GetSession(userID string) *UserSession {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.userSessions[userID]
}

// GetUserRisk 获取用户风险
func (e *UEBAEngine) GetUserRisk(userID string) *UserRisk {
	return e.riskEngine.GetUserRisk(userID)
}

// GetStats 获取统计
func (e *UEBAEngine) GetStats() *UEBAStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	highRisk := int64(0)
	for _, session := range e.userSessions {
		if session.RiskLevel == RiskLevelMalicious || session.RiskLevel == RiskLevelSuspicious {
			highRisk++
		}
	}

	return &UEBAStats{
		TotalUsers:      int64(len(e.userSessions)),
		ActiveUsers:     e.getActiveUserCount(),
		HighRiskUsers:   highRisk,
		TotalSessions:   e.stats.TotalSessions,
		ActiveSessions:  e.stats.ActiveSessions,
		PatternsMatched: e.stats.PatternsMatched,
		AlertsGenerated: e.stats.AlertsGenerated,
	}
}

// addEvent 添加事件到会话
func (s *UserSession) addEvent(event SessionEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Events = append(s.Events, event)
	s.EventCount++
	s.LastSeen = event.Timestamp

	// 限制事件数量
	if len(s.Events) > 1000 {
		s.Events = s.Events[500:]
	}
}

// updateStats 更新统计
func (e *UEBAEngine) updateStats(alert *Alert) {
	e.stats.ActiveSessions = int64(len(e.userSessions))
	if alert != nil {
		e.stats.AlertsGenerated++
	}
}

// getOrCreateSession 获取或创建会话
func (e *UEBAEngine) getOrCreateSession(event SessionEvent) *UserSession {
	userID, ok := event.Extra["user_id"].(string)
	if !ok || userID == "" {
		return nil
	}

	session, exists := e.userSessions[userID]
	if !exists {
		session = &UserSession{
			UserID:    userID,
			IP:        event.SourceIP,
			Start:     event.Timestamp,
			LastSeen:  event.Timestamp,
			Events:    make([]SessionEvent, 0),
			Metadata:  make(map[string]interface{}),
			RiskLevel: RiskLevelTrusted,
		}
		e.userSessions[userID] = session
		e.stats.TotalSessions++
	}
	return session
}

// getActiveUserCount 获取活跃用户数
func (e *UEBAEngine) getActiveUserCount() int64 {
	count := int64(0)
	now := time.Now()
	for _, session := range e.userSessions {
		if now.Sub(session.LastSeen) < e.config.SessionTimeout {
			count++
		}
	}
	return count
}

// cleanupWorker 清理过期会话
func (e *UEBAEngine) cleanupWorker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		e.mu.Lock()
		e.cleanupExpired()
		e.mu.Unlock()
	}
}

// cleanupExpired 清理过期会话
func (e *UEBAEngine) cleanupExpired() {
	now := time.Now()
	for userID, session := range e.userSessions {
		if now.Sub(session.LastSeen) > e.config.SessionTimeout*2 {
			delete(e.userSessions, userID)
		}
	}
}

// registerDefaultPatterns 注册默认模式
func (db *PatternDatabase) registerDefaultPatterns() {
	// 暴力破解模式
	db.patterns["brute_force"] = &BehaviorPattern{
		PatternID:   "brute_force",
		Name:        "暴力破解攻击",
		Description: "短时间内多次登录失败",
		Conditions: []PatternCondition{
			{Field: "action", Operator: "eq", Value: "login"},
			{Field: "result", Operator: "eq", Value: "failure"},
		},
		RiskScore: 80,
		Metadata:  map[string]interface{}{"type": "security"},
	}

	// 权限提升模式
	db.patterns["privilege_escalation"] = &BehaviorPattern{
		PatternID:   "privilege_escalation",
		Name:        "权限提升尝试",
		Description: "用户尝试访问高权限资源",
		Conditions: []PatternCondition{
			{Field: "action", Operator: "contains", Value: "admin"},
		},
		RiskScore: 70,
		Metadata:  map[string]interface{}{"type": "security"},
	}

	// 数据导出模式
	db.patterns["data_exfiltration"] = &BehaviorPattern{
		PatternID:   "data_exfiltration",
		Name:        "数据导出异常",
		Description: "大量数据下载或导出",
		Conditions: []PatternCondition{
			{Field: "action", Operator: "eq", Value: "export"},
		},
		RiskScore: 60,
		Metadata:  map[string]interface{}{"type": "dlp"},
	}
}

// detectPatterns 检测行为模式
func (e *UEBAEngine) detectPatterns(session *UserSession, event SessionEvent) *Alert {
	for _, pattern := range e.patternDB.patterns {
		if e.matchPattern(pattern, event) {
			pattern.Count++
			pattern.LastMatch = event.Timestamp

			return &Alert{
				AlertID:     fmt.Sprintf("pattern_%s_%s", pattern.PatternID, time.Now().Format("20060102150405")),
				UserID:      session.UserID,
				AlertType:   "pattern_match",
				Severity:    getSeverityFromScore(pattern.RiskScore),
				Score:       pattern.RiskScore,
				Title:       pattern.Name,
				Description: pattern.Description,
				Evidence: []Evidence{
					{
						Source:    "pattern_db",
						Type:      pattern.PatternID,
						Value:     event.URI,
						Timestamp: event.Timestamp,
					},
				},
				Timestamp: event.Timestamp,
			}
		}
	}
	return nil
}

// matchPattern 匹配模式
func (e *UEBAEngine) matchPattern(pattern *BehaviorPattern, event SessionEvent) bool {
	eventMap := eventToMap(event)
	for _, cond := range pattern.Conditions {
		if !evaluateCondition(cond, eventMap) {
			return false
		}
	}
	return true
}

// evaluateCondition 评估条件
func evaluateCondition(cond PatternCondition, event map[string]interface{}) bool {
	value, exists := event[cond.Field]
	if !exists {
		return false
	}

	switch cond.Operator {
	case "eq":
		return fmt.Sprintf("%v", value) == fmt.Sprintf("%v", cond.Value)
	case "contains":
		return containsStr(fmt.Sprintf("%v", value), fmt.Sprintf("%v", cond.Value))
	case "gt":
		return toFloat64(value) > toFloat64(cond.Value)
	case "lt":
		return toFloat64(value) < toFloat64(cond.Value)
	default:
		return false
	}
}

// eventToMap 转换事件为 map
func eventToMap(event SessionEvent) map[string]interface{} {
	m := map[string]interface{}{
		"type":      event.Type,
		"uri":       event.URI,
		"method":    event.Method,
		"status":    event.StatusCode,
		"source_ip": event.SourceIP,
		"action":    event.Action,
		"result":    event.Result,
	}
	for k, v := range event.Extra {
		m[k] = v
	}
	return m
}

// detectSequenceAnomaly 检测序列异常
func (e *UEBAEngine) detectSequenceAnomaly(session *UserSession, event SessionEvent) *Alert {
	model := e.patternDB.getSequenceModel(session.UserID)
	currentState := event.Action

	if model != nil {
		key := model.CurrentState + "->" + currentState
		count, exists := model.Transitions[key]

		// 计算转移概率
		total := int64(0)
		for _, c := range model.Transitions {
			total += c
		}

		if total > 0 {
			prob := float64(count) / float64(total)
			if exists && prob < 0.05 {
				// 低概率转移，标记异常
				return &Alert{
					AlertID:     fmt.Sprintf("sequence_%s_%s", session.UserID, time.Now().Format("20060102150405")),
					UserID:      session.UserID,
					AlertType:   "sequence_anomaly",
					Severity:    "medium",
					Score:       50,
					Title:       "异常行为序列",
					Description: fmt.Sprintf("检测到罕见的行为序列：%s", key),
					Evidence: []Evidence{
						{
							Source:    "sequence_model",
							Type:      "transition",
							Value:     key,
							Timestamp: event.Timestamp,
						},
					},
					Timestamp: event.Timestamp,
				}
			}
		}

		// 更新转移计数
		model.Transitions[key]++
		model.CurrentState = currentState
	} else {
		// 创建新模型
		e.patternDB.createSequenceModel(session.UserID, currentState)
	}

	return nil
}

// detectLateralMovement 检测横向移动
func (e *UEBAEngine) detectLateralMovement(session *UserSession, event SessionEvent) *Alert {
	if event.Action != "resource_access" {
		return nil
	}

	// 统计最近访问的资源数量
	uniqueResources := make(map[string]bool)
	windowStart := event.Timestamp.Add(-5 * time.Minute)

	for _, e := range session.Events {
		if e.Timestamp.After(windowStart) && e.ResourceID != "" {
			uniqueResources[e.ResourceID] = true
		}
	}

	if len(uniqueResources) >= e.config.LateralMovementThreshold {
		return &Alert{
			AlertID:     fmt.Sprintf("lateral_%s_%s", session.UserID, time.Now().Format("20060102150405")),
			UserID:      session.UserID,
			AlertType:   "lateral_movement",
			Severity:    "high",
			Score:       75,
			Title:       "潜在横向移动检测",
			Description: fmt.Sprintf("用户在 5 分钟内访问了 %d 个不同资源", len(uniqueResources)),
			Evidence: []Evidence{
				{
					Source:    "lateral_detection",
					Type:      "resource_count",
					Value:     fmt.Sprintf("%d", len(uniqueResources)),
					Timestamp: event.Timestamp,
				},
			},
			Timestamp: event.Timestamp,
		}
	}

	return nil
}

// detectInsiderThreat 检测内部威胁
func (e *UEBAEngine) detectInsiderThreat(session *UserSession, event SessionEvent) *Alert {
	// 内部威胁检测逻辑
	// 1. 非工作时间访问
	// 2. 访问敏感资源
	// 3. 异常下载行为

	riskFactors := 0

	// 检测非工作时间 (22:00 - 06:00)
	hour := event.Timestamp.Hour()
	if hour >= 22 || hour <= 6 {
		riskFactors++
	}

	// 检测敏感资源访问
	sensitivePatterns := []string{"/admin", "/config", "/backup", "/api/internal"}
	for _, pattern := range sensitivePatterns {
		if containsStr(event.URI, pattern) {
			riskFactors++
			break
		}
	}

	// 检测批量操作
	if event.Action == "batch_operation" {
		riskFactors++
	}

	// 计算风险分数
	riskScore := float64(riskFactors) * 30

	if riskScore >= e.config.InsiderThreatThreshold*100 {
		return &Alert{
			AlertID:     fmt.Sprintf("insider_%s_%s", session.UserID, time.Now().Format("20060102150405")),
			UserID:      session.UserID,
			AlertType:   "insider_threat",
			Severity:    getSeverityFromScore(riskScore),
			Score:       riskScore,
			Title:       "潜在内部威胁",
			Description: fmt.Sprintf("检测到 %d 个风险因素", riskFactors),
			Evidence: []Evidence{
				{
					Source:    "insider_detection",
					Type:      "risk_factors",
					Value:     fmt.Sprintf("%d", riskFactors),
					Timestamp: event.Timestamp,
				},
			},
			Timestamp: event.Timestamp,
		}
	}

	return nil
}

// UpdateRisk 更新用户风险评分
func (e *RiskEngine) UpdateRisk(session *UserSession, event SessionEvent) *Alert {
	e.mu.Lock()
	defer e.mu.Unlock()

	risk, exists := e.userRisks[session.UserID]
	if !exists {
		risk = &UserRisk{
			UserID:      session.UserID,
			RiskFactors: make([]RiskFactor, 0),
		}
		e.userRisks[session.UserID] = risk
	}

	// 计算各维度风险
	risk.BehaviorRisk = e.calculateBehaviorRisk(session)
	risk.SequenceRisk = e.calculateSequenceRisk(session)
	risk.LateralRisk = e.calculateLateralRisk(session)

	// 计算总分
	risk.TotalRisk = risk.BehaviorRisk*e.config.RiskScoreWeights.BehaviorAnomaly +
		risk.SequenceRisk*e.config.RiskScoreWeights.SequenceAnomaly +
		risk.LateralRisk*e.config.RiskScoreWeights.LateralMovement

	// 限制在 0-100
	if risk.TotalRisk > 100 {
		risk.TotalRisk = 100
	}

	risk.LastUpdated = event.Timestamp
	session.RiskScore = risk.TotalRisk
	session.RiskLevel = determineRiskLevel(risk.TotalRisk)

	return nil
}

// calculateBehaviorRisk 计算行为风险
func (e *RiskEngine) calculateBehaviorRisk(session *UserSession) float64 {
	// 基于错误率计算
	errors := 0
	for _, event := range session.Events {
		if event.StatusCode >= 400 {
			errors++
		}
	}

	if len(session.Events) == 0 {
		return 0
	}

	errorRate := float64(errors) / float64(len(session.Events))
	return errorRate * 100
}

// calculateSequenceRisk 计算序列风险
func (e *RiskEngine) calculateSequenceRisk(session *UserSession) float64 {
	// 基于事件序列的异常程度
	if len(session.Events) < 3 {
		return 0
	}

	// 检测快速连续操作
	windowSize := 5
	if len(session.Events) < windowSize {
		windowSize = len(session.Events)
	}
	recentEvents := session.Events[len(session.Events)-windowSize:]
	if len(recentEvents) >= 3 {
		duration := recentEvents[len(recentEvents)-1].Timestamp.Sub(recentEvents[0].Timestamp)
		if duration < 2*time.Second {
			return 50 // 快速操作，中等风险
		}
	}

	return 0
}

// calculateLateralRisk 计算横向移动风险
func (e *RiskEngine) calculateLateralRisk(session *UserSession) float64 {
	// 统计唯 URI 数量
	uniqueURIs := make(map[string]bool)
	for _, event := range session.Events {
		uniqueURIs[event.URI] = true
	}

	if len(uniqueURIs) > 20 {
		return float64(len(uniqueURIs)) * 2
	}
	return 0
}

// GetUserRisk 获取用户风险
func (e *RiskEngine) GetUserRisk(userID string) *UserRisk {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.userRisks[userID]
}

// getSequenceModel 获取序列模型
func (db *PatternDatabase) getSequenceModel(userID string) *SequenceModel {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.sequenceModels[userID]
}

// createSequenceModel 创建序列模型
func (db *PatternDatabase) createSequenceModel(userID string, initialState string) *SequenceModel {
	db.mu.Lock()
	defer db.mu.Unlock()

	model := &SequenceModel{
		UserID:       userID,
		Transitions:  make(map[string]int64),
		States:       []string{initialState},
		CurrentState: initialState,
		LastUpdated:  time.Now(),
	}
	db.sequenceModels[userID] = model
	return model
}

// determineRiskLevel 确定风险等级
func determineRiskLevel(score float64) RiskLevel {
	if score >= 80 {
		return RiskLevelMalicious
	}
	if score >= 50 {
		return RiskLevelSuspicious
	}
	if score <= 20 {
		return RiskLevelTrusted
	}
	return RiskLevelNormal
}

// getSeverityFromScore 根据分数获取严重程度
func getSeverityFromScore(score float64) string {
	if score >= 80 {
		return "critical"
	}
	if score >= 60 {
		return "high"
	}
	if score >= 40 {
		return "medium"
	}
	return "low"
}

// containsStr 检查字符串包含
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// toFloat64 转换为 float64
func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}

// sort.Float64s 需要导入 sort 包，已导入
var _ = sort.Float64s

// MarshalJSON 序列化 UserSession
func (s *UserSession) MarshalJSON() ([]byte, error) {
	type Alias UserSession
	return json.Marshal(&struct {
		*Alias
		RiskLevel string `json:"risk_level"`
	}{
		Alias:     (*Alias)(s),
		RiskLevel: string(s.RiskLevel),
	})
}
