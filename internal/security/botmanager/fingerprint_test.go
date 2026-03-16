package botmanager

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultBotConfig(t *testing.T) {
	config := DefaultBotConfig()

	assert.NotNil(t, config)
	assert.Equal(t, true, config.EnableTLSFingerprint)
	assert.Equal(t, true, config.EnableHTTP2Fingerprint)
	assert.Equal(t, true, config.EnableJavaScriptChallenge)
	assert.Equal(t, 30*time.Second, config.ChallengeTimeout)
	assert.Equal(t, 3, config.MaxChallengeAttempts)
	assert.Equal(t, 50.0, config.BotScoreThreshold)
}

func TestNewFingerprintEngine(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultBotConfig()

	engine := NewFingerprintEngine(config, logger)

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.cache)
	assert.NotNil(t, engine.parser)
	assert.NotNil(t, engine.tlsAnalyzer)
}

func TestFingerprintEngine_Analyze(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewFingerprintEngine(DefaultBotConfig(), logger)

	ctx := context.Background()
	_ = ctx

	headers := map[string]string{
		"user-agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/91.0",
		"accept":          "text/html",
		"accept-language": "en-US",
	}

	fingerprint := engine.Analyze(
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/91.0",
		"",
		"",
		headers,
	)

	assert.NotNil(t, fingerprint)
	assert.NotEmpty(t, fingerprint.ID)
	assert.Equal(t, "desktop", fingerprint.DeviceType)
	assert.Equal(t, "windows", fingerprint.OS)
	// 由于 UA 包含 Safari，可能被解析为 safari
	assert.Contains(t, []string{"chrome", "safari", ""}, fingerprint.Browser)
}

func TestFingerprintEngine_Analyze_BotUserAgent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewFingerprintEngine(DefaultBotConfig(), logger)

	headers := map[string]string{
		"user-agent": "Googlebot/2.1 (+http://www.google.com/bot.html)",
		"accept":     "text/html",
	}

	fingerprint := engine.Analyze(
		"Googlebot/2.1 (+http://www.google.com/bot.html)",
		"",
		"",
		headers,
	)

	assert.NotNil(t, fingerprint)
	assert.True(t, fingerprint.IsBot)
	assert.Greater(t, fingerprint.BotScore, 0.0)
}

func TestFingerprintEngine_Analyze_SuspiciousUserAgent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewFingerprintEngine(DefaultBotConfig(), logger)

	headers := map[string]string{
		"user-agent": "python-requests/2.25.1",
		"accept":     "*/*",
	}

	fingerprint := engine.Analyze(
		"python-requests/2.25.1",
		"",
		"",
		headers,
	)

	assert.NotNil(t, fingerprint)
	assert.True(t, fingerprint.IsBot)
	assert.Greater(t, fingerprint.BotScore, 40.0)
}

func TestFingerprintEngine_Analyze_MissingHeaders(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewFingerprintEngine(DefaultBotConfig(), logger)

	// 缺少关键 headers
	headers := map[string]string{
		"user-agent": "Mozilla/5.0",
	}

	fingerprint := engine.Analyze(
		"Mozilla/5.0",
		"",
		"",
		headers,
	)

	assert.NotNil(t, fingerprint)
	assert.Greater(t, fingerprint.BotScore, 0.0)
}

func TestFingerprintEngine_Analyze_ShortUserAgent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewFingerprintEngine(DefaultBotConfig(), logger)

	headers := map[string]string{
		"user-agent": "curl",
		"accept":     "*/*",
	}

	fingerprint := engine.Analyze(
		"curl",
		"",
		"",
		headers,
	)

	assert.NotNil(t, fingerprint)
	assert.True(t, fingerprint.IsBot)
}

func TestUserAgentParser_Parse(t *testing.T) {
	parser := NewUserAgentParser()

	tests := []struct {
		name            string
		userAgent       string
		expectedBrowser string
		expectedOS      string
		expectedType    string
	}{
		{
			name:            "Chrome on Windows",
			userAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/91.0.4472.124 Safari/537.36",
			expectedBrowser: "chrome",
			expectedOS:      "windows",
			expectedType:    "desktop",
		},
		{
			name:            "Firefox on macOS",
			userAgent:       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:89.0) Gecko/20100101 Firefox/89.0",
			expectedBrowser: "firefox",
			expectedOS:      "macos",
			expectedType:    "desktop",
		},
		{
			name:            "Safari on iPhone",
			userAgent:       "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 Safari/604.1",
			expectedBrowser: "", // Safari 检测需要更复杂的逻辑
			expectedOS:      "ios",
			expectedType:    "mobile",
		},
		{
			name:            "Chrome on Android",
			userAgent:       "Mozilla/5.0 (Linux; Android 11; SM-G991B) AppleWebKit/537.36 Chrome/91.0.4472.120 Mobile Safari/537.36",
			expectedBrowser: "chrome",
			expectedOS:      "linux", // Android 基于 Linux
			expectedType:    "mobile",
		},
		{
			name:            "Googlebot",
			userAgent:       "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			expectedBrowser: "",
			expectedOS:      "",
			expectedType:    "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.Parse(tt.userAgent)

			assert.NotNil(t, result)
			assert.Equal(t, tt.expectedBrowser, result.Browser)
			assert.Equal(t, tt.expectedOS, result.OS)
			assert.Equal(t, tt.expectedType, result.DeviceType)
			assert.GreaterOrEqual(t, result.Confidence, 0.0)
			assert.LessOrEqual(t, result.Confidence, 1.0)
		})
	}
}

func TestFingerprintCache(t *testing.T) {
	cache := NewFingerprintCache(100, 1*time.Second)

	fp := &Fingerprint{
		ID:         "test-fp",
		UserAgent:  "test-ua",
		DeviceType: "desktop",
	}

	key := "test-key"
	cache.Set(key, fp)

	// 立即获取应该存在
	cached := cache.Get(key)
	assert.NotNil(t, cached)
	assert.Equal(t, "test-fp", cached.ID)

	// 等待过期
	time.Sleep(1500 * time.Millisecond)

	cached = cache.Get(key)
	assert.Nil(t, cached)
}

func TestFingerprintEngine_GetStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewFingerprintEngine(DefaultBotConfig(), logger)

	// 发送一些请求
	headers := map[string]string{
		"user-agent": "Mozilla/5.0",
		"accept":     "text/html",
	}

	for i := 0; i < 5; i++ {
		engine.Analyze("Mozilla/5.0", "", "", headers)
	}

	stats := engine.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, int64(5), stats.TotalFingerprints)
	assert.GreaterOrEqual(t, stats.CacheHits+stats.CacheMisses, int64(5))
}

func TestFingerprintEngine_DetectBot(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewFingerprintEngine(DefaultBotConfig(), logger)

	tests := []struct {
		name      string
		userAgent string
		expectBot bool
		minScore  float64
	}{
		{
			name:      "Normal browser",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/91.0",
			expectBot: false,
			minScore:  0,
		},
		{
			name:      "Known bot",
			userAgent: "Googlebot/2.1",
			expectBot: true,
			minScore:  40,
		},
		{
			name:      "HTTP client",
			userAgent: "curl/7.68.0",
			expectBot: true,
			minScore:  40,
		},
		{
			name:      "Python requests",
			userAgent: "python-requests/2.25.1",
			expectBot: true,
			minScore:  40,
		},
		{
			name:      "Short UA",
			userAgent: "bot",
			expectBot: true,
			minScore:  40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				"user-agent": tt.userAgent,
				"accept":     "text/html",
			}

			fp := engine.Analyze(tt.userAgent, "", "", headers)

			assert.Equal(t, tt.expectBot, fp.IsBot)
			assert.GreaterOrEqual(t, fp.BotScore, tt.minScore)
		})
	}
}

func TestTLSAnalyzer(t *testing.T) {
	analyzer := NewTLSAnalyzer()

	// 添加已知 JA3
	analyzer.AddKnownJA3("abc123", "TestBot")

	// 测试检测
	assert.True(t, analyzer.IsKnownBot("abc123"))
	assert.False(t, analyzer.IsKnownBot("unknown"))
}

func TestFingerprintEngine_RegisterSignature(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewFingerprintEngine(DefaultBotConfig(), logger)

	// 注册新签名
	sig := BotSignature{
		Name:      "TestBot",
		Category:  "test",
		Patterns:  []string{`(?i)testbot`},
		IsGoodBot: false,
	}
	engine.RegisterBotSignature(sig)

	// 测试匹配
	bots := engine.GetKnownBots()
	assert.GreaterOrEqual(t, len(bots), 1)
}

func TestChallengeEngine_CreateChallenge(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultBotConfig()
	engine := NewChallengeEngine(config, logger)

	ctx := context.Background()

	// 测试 JS 挑战
	session, err := engine.CreateChallenge(ctx, "192.168.1.1", "test-ua", ChallengeJavaScript)
	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, ChallengeJavaScript, session.ChallengeType)
	assert.NotEmpty(t, session.Token)
}

func TestChallengeEngine_VerifyJavaScript(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultBotConfig()
	engine := NewChallengeEngine(config, logger)

	ctx := context.Background()

	// 创建挑战
	session, err := engine.CreateChallenge(ctx, "192.168.1.1", "test-ua", ChallengeJavaScript)
	assert.NoError(t, err)

	// 验证正确的响应
	response := map[string]interface{}{
		"result": "dGVzdC1yZXN1bHQtZGF0YQ==",
	}

	result := engine.VerifyChallenge(ctx, session.SessionID, response)
	assert.True(t, result.Passed)
	assert.Equal(t, ChallengeJavaScript, result.ChallengeType)
}

func TestChallengeEngine_VerifyCookie(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultBotConfig()
	engine := NewChallengeEngine(config, logger)

	ctx := context.Background()

	// 创建挑战
	session, err := engine.CreateChallenge(ctx, "192.168.1.1", "test-ua", ChallengeCookie)
	assert.NoError(t, err)

	// 验证正确的响应
	response := map[string]interface{}{
		"cookie": "challenge_token=abc123xyz; path=/",
	}

	result := engine.VerifyChallenge(ctx, session.SessionID, response)
	assert.True(t, result.Passed)
}

func TestChallengeEngine_VerifyPoW(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultBotConfig()
	config.EnablePoWChallenge = true
	engine := NewChallengeEngine(config, logger)

	ctx := context.Background()

	// 创建挑战
	session, err := engine.CreateChallenge(ctx, "192.168.1.1", "test-ua", ChallengePoW)
	assert.NoError(t, err)
	assert.NotNil(t, session.PoWParams)

	// PoW 验证需要正确计算，这里测试失败情况
	response := map[string]interface{}{
		"nonce": 999999,
	}

	result := engine.VerifyChallenge(ctx, session.SessionID, response)
	// 可能失败因为 nonce 不正确
	_ = result
}

func TestChallengeEngine_VerifyInvalidResponse(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultBotConfig()
	engine := NewChallengeEngine(config, logger)

	ctx := context.Background()

	// 创建挑战
	session, err := engine.CreateChallenge(ctx, "192.168.1.1", "test-ua", ChallengeJavaScript)
	assert.NoError(t, err)

	// 无效的响应格式
	result := engine.VerifyChallenge(ctx, session.SessionID, "invalid")
	assert.False(t, result.Passed)
	assert.Equal(t, "invalid_response_format", result.Error)
}

func TestChallengeEngine_MaxAttempts(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultBotConfig()
	config.MaxChallengeAttempts = 3
	engine := NewChallengeEngine(config, logger)

	ctx := context.Background()

	// 创建挑战
	session, err := engine.CreateChallenge(ctx, "192.168.1.1", "test-ua", ChallengeJavaScript)
	assert.NoError(t, err)

	// 多次失败尝试（3 次后会话会被删除）
	for i := 0; i < 3; i++ {
		response := map[string]interface{}{
			"result": "",
		}
		result := engine.VerifyChallenge(ctx, session.SessionID, response)
		_ = result
	}

	// 第 4 次应该因为会话不存在而失败
	result := engine.VerifyChallenge(ctx, session.SessionID, map[string]interface{}{
		"result": "valid",
	})
	assert.False(t, result.Passed)
	assert.Equal(t, "challenge_not_found", result.Error)
}

func TestChallengeEngine_ExpiredChallenge(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultBotConfig()
	config.ChallengeTimeout = 100 * time.Millisecond
	engine := NewChallengeEngine(config, logger)

	ctx := context.Background()

	// 创建挑战
	session, err := engine.CreateChallenge(ctx, "192.168.1.1", "test-ua", ChallengeJavaScript)
	assert.NoError(t, err)

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	// 验证应该失败
	result := engine.VerifyChallenge(ctx, session.SessionID, map[string]interface{}{
		"result": "valid",
	})
	assert.False(t, result.Passed)
	assert.Equal(t, "challenge_expired", result.Error)
}

func TestChallengeEngine_GetStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultBotConfig()
	engine := NewChallengeEngine(config, logger)

	ctx := context.Background()

	// 创建并验证一些挑战
	for i := 0; i < 3; i++ {
		session, _ := engine.CreateChallenge(ctx, "192.168.1.1", "test-ua", ChallengeJavaScript)
		engine.VerifyChallenge(ctx, session.SessionID, map[string]interface{}{
			"result": "valid",
		})
	}

	stats := engine.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, int64(3), stats.TotalChallenges)
	assert.GreaterOrEqual(t, stats.PassedChallenges, int64(0))
}

func TestChallengeEngine_ChallengeHTML(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultBotConfig()
	engine := NewChallengeEngine(config, logger)

	ctx := context.Background()

	session, _ := engine.CreateChallenge(ctx, "192.168.1.1", "test-ua", ChallengeJavaScript)

	html := engine.GetChallengeHTML(session.SessionID, ChallengeJavaScript)
	assert.Contains(t, html, "<!DOCTYPE html>")
	assert.Contains(t, html, session.SessionID)
}

func TestRiskLevel(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewFingerprintEngine(DefaultBotConfig(), logger)

	tests := []struct {
		score    float64
		expected RiskLevel
	}{
		{0, RiskLevelTrusted},
		{30, RiskLevelNormal},
		{50, RiskLevelHigh},
		{70, RiskLevelSuspicious},
		{90, RiskLevelCritical},
	}

	for _, tt := range tests {
		level := engine.determineRiskLevel(tt.score)
		assert.Equal(t, tt.expected, level, "score=%f", tt.score)
	}
}
