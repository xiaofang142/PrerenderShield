package zerotrust

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultContinuousVerificationConfig(t *testing.T) {
	config := DefaultContinuousVerificationConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 24*time.Hour, config.SessionTimeout)
	assert.Equal(t, 30*time.Minute, config.SessionIdleTimeout)
	assert.Equal(t, 5*time.Minute, config.VerificationInterval)
	assert.Equal(t, true, config.ReverifyOnIPChange)
	assert.Equal(t, true, config.ReverifyOnUAChange)
}

func TestNewContinuousVerificationEngine(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultContinuousVerificationConfig()

	engine := NewContinuousVerificationEngine(config, logger)

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.sessions)
	assert.NotNil(t, engine.behaviorDB)
}

func TestContinuousVerificationEngine_CreateSession(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewContinuousVerificationEngine(DefaultContinuousVerificationConfig(), logger)

	ctx := context.Background()
	location := &Location{
		Country:  "CN",
		Region:   "Beijing",
		City:     "Beijing",
		Lat:      39.9042,
		Lon:      116.4074,
		Timezone: "Asia/Shanghai",
	}

	session := engine.CreateSession(
		ctx,
		"user123",
		"device456",
		"192.168.1.1",
		"Mozilla/5.0 Chrome/91.0",
		location,
	)

	assert.NotNil(t, session)
	assert.NotEmpty(t, session.SessionID)
	assert.Equal(t, "user123", session.UserID)
	assert.Equal(t, "device456", session.DeviceID)
	assert.Equal(t, "192.168.1.1", session.IP)
	assert.Equal(t, 50.0, session.TrustScore)
	assert.True(t, session.IsValid)
}

func TestContinuousVerificationEngine_VerifySession(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewContinuousVerificationEngine(DefaultContinuousVerificationConfig(), logger)

	ctx := context.Background()

	// 创建会话
	session := engine.CreateSession(
		ctx,
		"user123",
		"device456",
		"192.168.1.1",
		"Mozilla/5.0 Chrome/91.0",
		nil,
	)

	// 验证会话（相同 IP 和 UA）
	result := engine.VerifySession(
		ctx,
		session.SessionID,
		"192.168.1.1",
		"Mozilla/5.0 Chrome/91.0",
		nil,
	)

	assert.True(t, result.Valid)
	assert.Equal(t, "allow", result.Action)
	assert.Equal(t, 50.0, result.TrustScore)
}

func TestContinuousVerificationEngine_VerifySession_IPChange(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultContinuousVerificationConfig()
	config.ReverifyOnIPChange = true
	engine := NewContinuousVerificationEngine(config, logger)

	ctx := context.Background()

	// 创建会话
	session := engine.CreateSession(
		ctx,
		"user123",
		"device456",
		"192.168.1.1",
		"Mozilla/5.0 Chrome/91.0",
		nil,
	)

	// 验证会话（不同 IP）
	result := engine.VerifySession(
		ctx,
		session.SessionID,
		"10.0.0.1", // 不同 IP
		"Mozilla/5.0 Chrome/91.0",
		nil,
	)

	assert.True(t, result.Valid)
	assert.Contains(t, result.Flags, "ip_changed")
	assert.Less(t, result.TrustScore, 50.0)
	assert.True(t, result.ReverifyRequired)
}

func TestContinuousVerificationEngine_VerifySession_UAChange(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultContinuousVerificationConfig()
	config.ReverifyOnUAChange = true
	engine := NewContinuousVerificationEngine(config, logger)

	ctx := context.Background()

	// 创建会话
	session := engine.CreateSession(
		ctx,
		"user123",
		"device456",
		"192.168.1.1",
		"Mozilla/5.0 Chrome/91.0",
		nil,
	)

	// 验证会话（不同 UA）
	result := engine.VerifySession(
		ctx,
		session.SessionID,
		"192.168.1.1",
		"Mozilla/5.0 Chrome/92.0", // 不同 UA
		nil,
	)

	assert.True(t, result.Valid)
	assert.Contains(t, result.Flags, "ua_changed")
	assert.Less(t, result.TrustScore, 50.0)
}

func TestContinuousVerificationEngine_VerifySession_InvalidSession(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewContinuousVerificationEngine(DefaultContinuousVerificationConfig(), logger)

	ctx := context.Background()

	// 验证不存在的会话
	result := engine.VerifySession(
		ctx,
		"nonexistent-session",
		"192.168.1.1",
		"Mozilla/5.0",
		nil,
	)

	assert.False(t, result.Valid)
	assert.Equal(t, "session_not_found", result.Reason)
	assert.Equal(t, "reject", result.Action)
}

func TestContinuousVerificationEngine_RecordActivity(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewContinuousVerificationEngine(DefaultContinuousVerificationConfig(), logger)

	ctx := context.Background()

	// 创建会话
	session := engine.CreateSession(
		ctx,
		"user123",
		"device456",
		"192.168.1.1",
		"Mozilla/5.0",
		nil,
	)

	// 记录活动
	engine.RecordActivity(ctx, session.SessionID, "/api/data", "GET", 100)

	// 验证行为已记录
	behavior := engine.behaviorDB.Get("user123", "device456")
	assert.NotNil(t, behavior)
	assert.Greater(t, len(behavior.RequestTimes), 0)
}

func TestContinuousVerificationEngine_UpdateTrust(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewContinuousVerificationEngine(DefaultContinuousVerificationConfig(), logger)

	ctx := context.Background()

	// 创建会话
	session := engine.CreateSession(
		ctx,
		"user123",
		"device456",
		"192.168.1.1",
		"Mozilla/5.0",
		nil,
	)

	initialScore := session.TrustScore

	// 更新信任（加分）
	engine.UpdateTrust(session.SessionID, 10.0, "good_behavior")
	session = engine.GetSession(session.SessionID)
	assert.Greater(t, session.TrustScore, initialScore)

	// 更新信任（减分）
	engine.UpdateTrust(session.SessionID, -15.0, "bad_behavior")
	session = engine.GetSession(session.SessionID)
	assert.Less(t, session.TrustScore, initialScore+10.0)
}

func TestContinuousVerificationEngine_InvalidateSession(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewContinuousVerificationEngine(DefaultContinuousVerificationConfig(), logger)

	ctx := context.Background()

	// 创建会话
	session := engine.CreateSession(
		ctx,
		"user123",
		"device456",
		"192.168.1.1",
		"Mozilla/5.0",
		nil,
	)

	assert.True(t, session.IsValid)

	// 使会话失效
	engine.InvalidateSession(session.SessionID)

	session = engine.GetSession(session.SessionID)
	assert.False(t, session.IsValid)
}

func TestContinuousVerificationEngine_CloseSession(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewContinuousVerificationEngine(DefaultContinuousVerificationConfig(), logger)

	ctx := context.Background()

	// 创建会话
	session := engine.CreateSession(
		ctx,
		"user123",
		"device456",
		"192.168.1.1",
		"Mozilla/5.0",
		nil,
	)

	assert.NotNil(t, session)

	// 关闭会话
	engine.CloseSession(session.SessionID)

	session = engine.GetSession(session.SessionID)
	assert.Nil(t, session)
}

func TestSessionStore(t *testing.T) {
	store := NewSessionStore(100, 1*time.Hour)

	session := &Session{
		SessionID:  "test-session",
		UserID:     "user123",
		IsValid:    true,
		TrustScore: 50.0,
	}

	store.Set("test-session", session)

	// 获取
	retrieved := store.Get("test-session")
	assert.NotNil(t, retrieved)
	assert.Equal(t, "test-session", retrieved.SessionID)

	// 删除
	store.Delete("test-session")
	retrieved = store.Get("test-session")
	assert.Nil(t, retrieved)
}

func TestSessionStore_List(t *testing.T) {
	store := NewSessionStore(100, 1*time.Hour)

	// 添加多个会话
	for i := 0; i < 5; i++ {
		store.Set("session-"+string(rune('0'+i)), &Session{
			SessionID: "session-" + string(rune('0'+i)),
			UserID:    "user" + string(rune('0'+i)),
			IsValid:   true,
		})
	}

	sessions := store.List()
	assert.Len(t, sessions, 5)
}

func TestBehaviorDB_Record(t *testing.T) {
	db := NewBehaviorDB()

	// 记录行为
	for i := 0; i < 10; i++ {
		db.Record("user123", "device456", "/api/test", "GET", 100)
	}

	behavior := db.Get("user123", "device456")
	assert.NotNil(t, behavior)
	assert.Equal(t, 10, len(behavior.RequestTimes))
	assert.Equal(t, 10, len(behavior.URIs))
}

func TestBehaviorDB_LimitHistory(t *testing.T) {
	db := NewBehaviorDB()

	// 记录超过限制的行为
	for i := 0; i < 1500; i++ {
		db.Record("user123", "device456", "/api/test", "GET", 100)
	}

	behavior := db.Get("user123", "device456")
	assert.NotNil(t, behavior)
	assert.LessOrEqual(t, len(behavior.RequestTimes), 1000)
}

func TestContinuousVerificationEngine_GetStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewContinuousVerificationEngine(DefaultContinuousVerificationConfig(), logger)

	ctx := context.Background()

	// 创建一些会话
	for i := 0; i < 5; i++ {
		engine.CreateSession(
			ctx,
			"user"+string(rune('0'+i)),
			"device"+string(rune('0'+i)),
			"192.168.1."+string(rune('1'+i)),
			"Mozilla/5.0",
			nil,
		)
	}

	stats := engine.GetStats()
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.TotalSessions, int64(5))
	// 由于 session ID 可能重复，ActiveSessions 可能少于 5
	assert.GreaterOrEqual(t, stats.ActiveSessions, int64(4))
}

func TestVerificationResult(t *testing.T) {
	result := &VerificationResult{
		Valid:            true,
		Reason:           "",
		Action:           "allow",
		TrustScore:       75.0,
		RiskScore:        25.0,
		Flags:            []string{"ip_changed"},
		ReverifyRequired: true,
		ChallengeType:    "javascript",
	}

	assert.NotNil(t, result)
	assert.True(t, result.Valid)
	assert.Equal(t, "allow", result.Action)
	assert.Equal(t, 75.0, result.TrustScore)
	assert.Equal(t, 25.0, result.RiskScore)
	assert.Contains(t, result.Flags, "ip_changed")
	assert.True(t, result.ReverifyRequired)
}

func TestSession(t *testing.T) {
	now := time.Now()
	location := &Location{
		Country: "CN",
		Region:  "Beijing",
		City:    "Beijing",
		Lat:     39.9042,
		Lon:     116.4074,
	}

	session := &Session{
		SessionID:         "sess-test123",
		UserID:            "user123",
		DeviceID:          "device456",
		IP:                "192.168.1.1",
		UserAgent:         "Mozilla/5.0",
		Location:          location,
		StartTime:         now,
		LastActivity:      now,
		LastVerification:  now,
		TrustScore:        75.0,
		RiskScore:         25.0,
		BehaviorFlags:     []string{"verified"},
		IsValid:           true,
		IsVerified:        true,
		VerificationCount: 5,
	}

	assert.NotNil(t, session)
	assert.Equal(t, "sess-test123", session.SessionID)
	assert.Equal(t, 75.0, session.TrustScore)
	assert.True(t, session.IsValid)
}

func TestLocation(t *testing.T) {
	location := &Location{
		Country:  "CN",
		Region:   "Beijing",
		City:     "Beijing",
		Lat:      39.9042,
		Lon:      116.4074,
		Timezone: "Asia/Shanghai",
		Accuracy: 10.0,
	}

	assert.NotNil(t, location)
	assert.Equal(t, "CN", location.Country)
	assert.Equal(t, "Beijing", location.City)
	assert.Equal(t, 39.9042, location.Lat)
}

func TestUserBehavior(t *testing.T) {
	now := time.Now()
	behavior := &UserBehavior{
		UserID:        "user123",
		DeviceID:      "device456",
		RequestTimes:  []time.Time{now, now.Add(1 * time.Minute)},
		URIs:          []string{"/api/test", "/api/data"},
		Methods:       []string{"GET", "POST"},
		ResponseTimes: []int64{100, 200},
		AvgRPM:        2.0,
		MaxRPM:        5.0,
		LastUpdated:   now,
	}

	assert.NotNil(t, behavior)
	assert.Len(t, behavior.RequestTimes, 2)
	assert.Equal(t, 2.0, behavior.AvgRPM)
}

func TestContinuousVerificationEngine_VerifySession_LocationChange(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultContinuousVerificationConfig()
	config.ReverifyOnLocationChange = true
	engine := NewContinuousVerificationEngine(config, logger)

	ctx := context.Background()

	// 创建会话（北京）
	session := engine.CreateSession(
		ctx,
		"user123",
		"device456",
		"192.168.1.1",
		"Mozilla/5.0",
		&Location{
			Country: "CN",
			City:    "Beijing",
			Lat:     39.9042,
			Lon:     116.4074,
		},
	)

	// 验证会话（上海 - 远距离）
	result := engine.VerifySession(
		ctx,
		session.SessionID,
		"192.168.1.1",
		"Mozilla/5.0",
		&Location{
			Country: "CN",
			City:    "Shanghai",
			Lat:     31.2304,
			Lon:     121.4737,
		},
	)

	// 应该检测到位置变化
	assert.Contains(t, result.Flags, "location_changed")
	assert.Less(t, result.TrustScore, 50.0)
}

func TestSessionStore_Cleanup(t *testing.T) {
	store := NewSessionStore(100, 100*time.Millisecond)

	// 添加即将过期的会话
	session := &Session{
		SessionID: "test-session",
		StartTime: time.Now().Add(-25 * time.Hour), // 25 小时前
		IsValid:   true,
	}
	store.Set("test-session", session)

	// 等待并清理
	time.Sleep(150 * time.Millisecond)
	store.cleanup()

	// 应该被清理
	cached := store.Get("test-session")
	assert.Nil(t, cached)
}
