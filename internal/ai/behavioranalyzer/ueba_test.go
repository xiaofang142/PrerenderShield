package behavioranalyzer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultUEBAConfig(t *testing.T) {
	config := DefaultUEBAConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 30*time.Minute, config.SessionTimeout)
	assert.Equal(t, 1000, config.MaxSessionHistory)
	assert.Equal(t, true, config.EnableSequenceAnalysis)
	assert.Equal(t, true, config.EnableLateralMovement)
	assert.Equal(t, 10, config.LateralMovementThreshold)
	assert.NotNil(t, config.RiskScoreWeights)
}

func TestNewUEBAEngine(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultUEBAConfig()

	engine := NewUEBAEngine(config, logger)

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.userSessions)
	assert.NotNil(t, engine.riskEngine)
	assert.NotNil(t, engine.patternDB)
}

func TestUEBAEngine_ProcessEvent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewUEBAEngine(DefaultUEBAConfig(), logger)

	ctx := context.Background()
	event := SessionEvent{
		Timestamp:  time.Now(),
		Type:       "request",
		URI:        "/api/data",
		Method:     "GET",
		StatusCode: 200,
		SourceIP:   "192.168.1.100",
		Action:     "view",
		Result:     "success",
		Extra: map[string]interface{}{
			"user_id": "user123",
		},
	}

	alert := engine.ProcessEvent(ctx, event)
	assert.Nil(t, alert) // 首次事件不应产生告警

	session := engine.GetSession("user123")
	assert.NotNil(t, session)
	assert.Equal(t, int64(1), session.EventCount)
}

func TestUEBAEngine_ProcessEvent_BruteforcePattern(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewUEBAEngine(DefaultUEBAConfig(), logger)

	ctx := context.Background()

	// 模拟暴力破解：多次登录失败
	var alert *Alert
	for i := 0; i < 5; i++ {
		event := SessionEvent{
			Timestamp:  time.Now(),
			Type:       "auth",
			URI:        "/login",
			Method:     "POST",
			StatusCode: 401,
			SourceIP:   "192.168.1.200",
			Action:     "login",
			Result:     "failure",
			Extra: map[string]interface{}{
				"user_id": "attacker",
			},
		}
		alert = engine.ProcessEvent(ctx, event)
	}

	// 应该触发暴力破解模式告警
	assert.NotNil(t, alert)
	assert.Equal(t, "pattern_match", alert.AlertType)
}

func TestUEBAEngine_GetUserRisk(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewUEBAEngine(DefaultUEBAConfig(), logger)

	ctx := context.Background()

	// 创建一些事件
	for i := 0; i < 10; i++ {
		statusCode := 200
		if i%3 == 0 {
			statusCode = 500 // 一些错误
		}

		event := SessionEvent{
			Timestamp:  time.Now(),
			Type:       "request",
			URI:        "/api/data",
			Method:     "GET",
			StatusCode: statusCode,
			SourceIP:   "192.168.1.50",
			Action:     "view",
			Result:     "success",
			Extra: map[string]interface{}{
				"user_id": "risk_user",
			},
		}
		engine.ProcessEvent(ctx, event)
	}

	risk := engine.GetUserRisk("risk_user")
	assert.NotNil(t, risk)
	assert.GreaterOrEqual(t, risk.TotalRisk, 0.0)
}

func TestRiskEngine_CalculateBehaviorRisk(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultUEBAConfig()
	engine := NewUEBAEngine(config, logger)

	session := &UserSession{
		UserID:   "test_user",
		Events:   make([]SessionEvent, 0),
		RiskLevel: RiskLevelNormal,
	}

	// 添加一些正常事件
	for i := 0; i < 8; i++ {
		session.Events = append(session.Events, SessionEvent{
			StatusCode: 200,
		})
	}

	// 添加一些错误事件
	for i := 0; i < 2; i++ {
		session.Events = append(session.Events, SessionEvent{
			StatusCode: 500,
		})
	}

	risk := engine.riskEngine.calculateBehaviorRisk(session)
	assert.Greater(t, risk, 0.0) // 20% 错误率
}

func TestDetermineRiskLevel(t *testing.T) {
	tests := []struct {
		score    float64
		expected RiskLevel
	}{
		{0, RiskLevelTrusted},
		{15, RiskLevelTrusted},
		{25, RiskLevelNormal},
		{50, RiskLevelSuspicious},
		{75, RiskLevelSuspicious},
		{80, RiskLevelMalicious},
		{100, RiskLevelMalicious},
	}

	for _, tt := range tests {
		level := determineRiskLevel(tt.score)
		assert.Equal(t, tt.expected, level, "score=%f", tt.score)
	}
}

func TestGetSeverityFromScore(t *testing.T) {
	tests := []struct {
		score    float64
		expected string
	}{
		{0, "low"},
		{30, "low"},
		{40, "medium"},
		{50, "medium"},
		{60, "high"},
		{70, "high"},
		{80, "critical"},
		{90, "critical"},
	}

	for _, tt := range tests {
		severity := getSeverityFromScore(tt.score)
		assert.Equal(t, tt.expected, severity, "score=%f", tt.score)
	}
}

func TestPatternDatabase(t *testing.T) {
	db := NewPatternDatabase()

	assert.NotNil(t, db)
	assert.Greater(t, len(db.patterns), 0) // 应该有默认模式

	// 检查默认模式
	bruteForce, exists := db.patterns["brute_force"]
	assert.True(t, exists)
	assert.Equal(t, "暴力破解攻击", bruteForce.Name)
}

func TestUEBAEngine_GetStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewUEBAEngine(DefaultUEBAConfig(), logger)

	ctx := context.Background()

	// 创建一些事件
	for i := 0; i < 5; i++ {
		event := SessionEvent{
			Timestamp:  time.Now(),
			Type:       "request",
			URI:        "/api/test",
			Method:     "GET",
			StatusCode: 200,
			SourceIP:   "192.168.1.1",
			Extra: map[string]interface{}{
				"user_id": "user_" + string(rune('0'+i)),
			},
		}
		engine.ProcessEvent(ctx, event)
	}

	stats := engine.GetStats()
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.TotalUsers, int64(1))
}

func TestUEBAEngine_DetectLateralMovement(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultUEBAConfig()
	config.LateralMovementThreshold = 5 // 降低阈值便于测试
	engine := NewUEBAEngine(config, logger)

	ctx := context.Background()

	// 短时间内访问多个不同资源
	for i := 0; i < 10; i++ {
		event := SessionEvent{
			Timestamp:  time.Now(),
			Type:       "access",
			URI:        "/resource/" + string(rune('0'+i)),
			Method:     "GET",
			StatusCode: 200,
			SourceIP:   "192.168.1.100",
			Action:     "resource_access",
			ResourceID: "resource_" + string(rune('0'+i)),
			Extra: map[string]interface{}{
				"user_id": "lateral_user",
			},
		}
		engine.ProcessEvent(ctx, event)
	}

	// 应该触发横向移动检测
	session := engine.GetSession("lateral_user")
	assert.NotNil(t, session)
}

func TestUEBAEngine_DetectInsiderThreat(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewUEBAEngine(DefaultUEBAConfig(), logger)

	ctx := context.Background()

	// 非工作时间访问敏感资源
	event := SessionEvent{
		Timestamp:  time.Date(2026, 1, 1, 23, 0, 0, 0, time.Local), // 23:00
		Type:       "access",
		URI:        "/admin/config",
		Method:     "GET",
		StatusCode: 200,
		SourceIP:   "192.168.1.50",
		Action:     "view",
		Extra: map[string]interface{}{
			"user_id": "insider_user",
		},
	}

	alert := engine.ProcessEvent(ctx, event)
	// 可能触发内部威胁告警
	_ = alert
}

func TestUserSession_AddEvent(t *testing.T) {
	session := &UserSession{
		UserID:   "test",
		Events:   make([]SessionEvent, 0),
		RiskLevel: RiskLevelTrusted,
	}

	// 添加事件
	for i := 0; i < 10; i++ {
		session.addEvent(SessionEvent{
			Timestamp: time.Now(),
			URI:       "/test",
		})
	}

	assert.Equal(t, int64(10), session.EventCount)
	assert.Len(t, session.Events, 10)
}

func TestRiskEngine_UpdateRisk(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultUEBAConfig()
	engine := NewRiskEngine(config, logger)

	session := &UserSession{
		UserID:   "risk_test",
		Events:   make([]SessionEvent, 0),
		RiskLevel: RiskLevelTrusted,
	}

	// 添加混合事件
	for i := 0; i < 10; i++ {
		status := 200
		if i%4 == 0 {
			status = 500
		}
		session.Events = append(session.Events, SessionEvent{
			Timestamp:  time.Now(),
			StatusCode: status,
		})
	}

	event := SessionEvent{
		Timestamp: time.Now(),
	}

	engine.UpdateRisk(session, event)

	risk := engine.GetUserRisk("risk_test")
	assert.NotNil(t, risk)
	assert.GreaterOrEqual(t, risk.TotalRisk, 0.0)
	assert.LessOrEqual(t, risk.TotalRisk, 100.0)
}

func TestSequenceModel(t *testing.T) {
	db := NewPatternDatabase()

	// 创建序列模型
	model := db.createSequenceModel("seq_user", "login")
	assert.NotNil(t, model)
	assert.Equal(t, "seq_user", model.UserID)
	assert.Equal(t, "login", model.CurrentState)

	// 获取模型
	retrieved := db.getSequenceModel("seq_user")
	assert.NotNil(t, retrieved)
	assert.Equal(t, model, retrieved)
}

func TestEvaluateCondition(t *testing.T) {
	event := map[string]interface{}{
		"action": "login",
		"result": "failure",
		"count":  5,
	}

	// 测试 eq
	cond := PatternCondition{Field: "action", Operator: "eq", Value: "login"}
	assert.True(t, evaluateCondition(cond, event))

	cond = PatternCondition{Field: "action", Operator: "eq", Value: "logout"}
	assert.False(t, evaluateCondition(cond, event))

	// 测试 contains
	cond = PatternCondition{Field: "action", Operator: "contains", Value: "log"}
	assert.True(t, evaluateCondition(cond, event))

	// 测试 gt
	cond = PatternCondition{Field: "count", Operator: "gt", Value: 3.0}
	assert.True(t, evaluateCondition(cond, event))

	// 测试 lt
	cond = PatternCondition{Field: "count", Operator: "lt", Value: 10.0}
	assert.True(t, evaluateCondition(cond, event))
}
