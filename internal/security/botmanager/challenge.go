package botmanager

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ChallengeEngine 挑战引擎
type ChallengeEngine struct {
	config   *BotConfig
	logger   *zap.Logger
	sessions *ChallengeSessionStore
	stats    *ChallengeStats
}

// ChallengeSessionStore 挑战会话存储
type ChallengeSessionStore struct {
	sessions map[string]*ChallengeSession
	mu       sync.RWMutex
	ttl      time.Duration
}

// ChallengeSession 挑战会话
type ChallengeSession struct {
	SessionID   string
	IP          string
	UserAgent   string
	ChallengeType ChallengeType
	CreatedAt   time.Time
	ExpiresAt   time.Time
	Attempts    int
	Passed      bool
	Token       string
	PoWParams   *PoWParams
}

// PoWParams Proof of Work 参数
type PoWParams struct {
	Challenge string `json:"challenge"`
	Difficulty int   `json:"difficulty"` // 前缀零的个数
	Salt       string `json:"salt"`
}

// PoWResult Proof of Work 结果
type PoWResult struct {
	Nonce     int    `json:"nonce"`
	Solution  string `json:"solution"`
	ElapsedMs int64  `json:"elapsed_ms"`
}

// ChallengeStats 挑战统计
type ChallengeStats struct {
	TotalChallenges   int64 `json:"total_challenges"`
	PassedChallenges  int64 `json:"passed_challenges"`
	FailedChallenges  int64 `json:"failed_challenges"`
	TimeoutChallenges int64 `json:"timeout_challenges"`
	JSCallenges       int64 `json:"js_challenges"`
	CookieChallenges  int64 `json:"cookie_challenges"`
	PoWChallenges     int64 `json:"pow_challenges"`
	CaptchaChallenges int64 `json:"captcha_challenges"`
}

// NewChallengeEngine 创建挑战引擎
func NewChallengeEngine(config *BotConfig, logger *zap.Logger) *ChallengeEngine {
	if config == nil {
		config = DefaultBotConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &ChallengeEngine{
		config:   config,
		logger:   logger,
		sessions: NewChallengeSessionStore(config.ChallengeTimeout),
		stats:    &ChallengeStats{},
	}
}

// NewChallengeSessionStore 创建挑战会话存储
func NewChallengeSessionStore(ttl time.Duration) *ChallengeSessionStore {
	store := &ChallengeSessionStore{
		sessions: make(map[string]*ChallengeSession),
		ttl:      ttl,
	}

	// 启动清理协程
	go store.cleanupWorker()

	return store
}

// CreateChallenge 创建挑战
func (e *ChallengeEngine) CreateChallenge(ctx context.Context, ip, userAgent string, challengeType ChallengeType) (*ChallengeSession, error) {
	sessionID := e.generateSessionID()
	now := time.Now()

	session := &ChallengeSession{
		SessionID:   sessionID,
		IP:          ip,
		UserAgent:   userAgent,
		ChallengeType: challengeType,
		CreatedAt:   now,
		ExpiresAt:   now.Add(e.config.ChallengeTimeout),
		Attempts:    0,
		Passed:      false,
	}

	// 根据挑战类型生成特定参数
	switch challengeType {
	case ChallengePoW:
		session.PoWParams = e.generatePoWParams()
		session.Token = session.PoWParams.Challenge
	case ChallengeJavaScript:
		session.Token = e.generateJSToken()
	case ChallengeCookie:
		session.Token = e.generateCookieToken()
	case ChallengeCaptcha:
		session.Token = e.generateCaptchaID()
	}

	e.sessions.Set(sessionID, session)
	e.stats.TotalChallenges++

	e.logger.Debug("创建挑战",
		zap.String("session_id", sessionID),
		zap.String("challenge_type", string(challengeType)),
		zap.String("ip", ip),
	)

	return session, nil
}

// VerifyChallenge 验证挑战
func (e *ChallengeEngine) VerifyChallenge(ctx context.Context, sessionID string, response interface{}) *ChallengeResult {
	startTime := time.Now()

	session := e.sessions.Get(sessionID)
	if session == nil {
		return &ChallengeResult{
			Passed: false,
			Error:  "challenge_not_found",
		}
	}

	// 检查是否过期
	if time.Now().After(session.ExpiresAt) {
		e.stats.TimeoutChallenges++
		e.sessions.Delete(sessionID)
		return &ChallengeResult{
			Passed:      false,
			ChallengeType: session.ChallengeType,
			Error:       "challenge_expired",
		}
	}

	// 检查尝试次数
	if session.Attempts >= e.config.MaxChallengeAttempts {
		e.stats.FailedChallenges++
		e.sessions.Delete(sessionID)
		return &ChallengeResult{
			Passed:      false,
			ChallengeType: session.ChallengeType,
			Error:       "max_attempts_exceeded",
		}
	}

	session.Attempts++

	// 根据挑战类型验证
	var result *ChallengeResult
	switch session.ChallengeType {
	case ChallengeJavaScript:
		result = e.verifyJavaScript(response)
	case ChallengeCookie:
		result = e.verifyCookie(response)
	case ChallengePoW:
		result = e.verifyPoW(session.PoWParams, response)
	case ChallengeCaptcha:
		result = e.verifyCaptcha(response)
	}

	if result == nil {
		result = &ChallengeResult{
			Passed: false,
			Error:  "unknown_challenge_type",
		}
	}

	result.ChallengeType = session.ChallengeType
	result.Duration = time.Since(startTime)
	result.Attempts = session.Attempts

	if result.Passed {
		session.Passed = true
		e.stats.PassedChallenges++
		e.sessions.Delete(sessionID)
	} else {
		e.stats.FailedChallenges++
		if session.Attempts >= e.config.MaxChallengeAttempts {
			e.sessions.Delete(sessionID)
		}
	}

	e.logger.Debug("验证挑战",
		zap.String("session_id", sessionID),
		zap.Bool("passed", result.Passed),
		zap.String("error", result.Error),
	)

	return result
}

// verifyJavaScript 验证 JavaScript 挑战
func (e *ChallengeEngine) verifyJavaScript(response interface{}) *ChallengeResult {
	// JavaScript 挑战：客户端需要执行一段 JS 代码并返回计算结果
	// 响应应该包含正确的计算结果
	respMap, ok := response.(map[string]interface{})
	if !ok {
		return &ChallengeResult{
			Passed: false,
			Error:  "invalid_response_format",
		}
	}

	result, ok := respMap["result"].(string)
	if !ok || result == "" {
		return &ChallengeResult{
			Passed: false,
			Error:  "missing_result",
		}
	}

	// 简单验证：结果不为空且长度合理
	if len(result) < 10 || len(result) > 1000 {
		return &ChallengeResult{
			Passed: false,
			Error:  "invalid_result_length",
		}
	}

	return &ChallengeResult{
		Passed: true,
		Score:  0.8,
		Extra: map[string]interface{}{
			"challenge_type": "javascript",
		},
	}
}

// verifyCookie 验证 Cookie 挑战
func (e *ChallengeEngine) verifyCookie(response interface{}) *ChallengeResult {
	// Cookie 挑战：客户端需要在指定时间内获得并返回 Cookie
	respMap, ok := response.(map[string]interface{})
	if !ok {
		return &ChallengeResult{
			Passed: false,
			Error:  "invalid_response_format",
		}
	}

	cookie, ok := respMap["cookie"].(string)
	if !ok || cookie == "" {
		return &ChallengeResult{
			Passed: false,
			Error:  "missing_cookie",
		}
	}

	// 验证 Cookie 格式
	if len(cookie) < 10 {
		return &ChallengeResult{
			Passed: false,
			Error:  "invalid_cookie",
		}
	}

	return &ChallengeResult{
		Passed: true,
		Score:  0.6,
		Extra: map[string]interface{}{
			"challenge_type": "cookie",
		},
	}
}

// verifyPoW 验证 Proof of Work
func (e *ChallengeEngine) verifyPoW(params *PoWParams, response interface{}) *ChallengeResult {
	if params == nil {
		return &ChallengeResult{
			Passed: false,
			Error:  "missing_pow_params",
		}
	}

	respMap, ok := response.(map[string]interface{})
	if !ok {
		return &ChallengeResult{
			Passed: false,
			Error:  "invalid_response_format",
		}
	}

	nonceFloat, ok := respMap["nonce"].(float64)
	if !ok {
		return &ChallengeResult{
			Passed: false,
			Error:  "missing_nonce",
		}
	}
	nonce := int(nonceFloat)

	// 验证解决方案
	expectedPrefix := ""
	for i := 0; i < params.Difficulty; i++ {
		expectedPrefix += "0"
	}

	input := fmt.Sprintf("%s%s%d", params.Challenge, params.Salt, nonce)
	hash := e.hashString(input)

	if len(hash) > len(expectedPrefix) && hash[:params.Difficulty] == expectedPrefix {
		return &ChallengeResult{
			Passed: true,
			Score:  1.0, // PoW 通过表示高置信度
			Extra: map[string]interface{}{
				"challenge_type": "pow",
				"difficulty":     params.Difficulty,
				"nonce":          nonce,
			},
		}
	}

	return &ChallengeResult{
		Passed: false,
		Error:  "invalid_solution",
	}
}

// verifyCaptcha 验证人机验证
func (e *ChallengeEngine) verifyCaptcha(response interface{}) *ChallengeResult {
	// 实际应用中需要调用第三方验证码服务
	// 这里简化处理
	respMap, ok := response.(map[string]interface{})
	if !ok {
		return &ChallengeResult{
			Passed: false,
			Error:  "invalid_response_format",
		}
	}

	token, ok := respMap["token"].(string)
	if !ok || token == "" {
		return &ChallengeResult{
			Passed: false,
			Error:  "missing_token",
		}
	}

	// 简化：假设有 token 就通过
	return &ChallengeResult{
		Passed: true,
		Score:  0.95,
		Extra: map[string]interface{}{
			"challenge_type": "captcha",
		},
	}
}

// generatePoWParams 生成 PoW 参数
func (e *ChallengeEngine) generatePoWParams() *PoWParams {
	challenge := e.generateRandomString(32)
	salt := e.generateRandomString(16)
	difficulty := 4 // 默认需要 4 个前导零

	return &PoWParams{
		Challenge:  challenge,
		Difficulty: difficulty,
		Salt:       salt,
	}
}

// generateJSToken 生成 JS 挑战令牌
func (e *ChallengeEngine) generateJSToken() string {
	return e.generateRandomString(32)
}

// generateCookieToken 生成 Cookie 挑战令牌
func (e *ChallengeEngine) generateCookieToken() string {
	return e.generateRandomString(24)
}

// generateCaptchaID 生成验证码 ID
func (e *ChallengeEngine) generateCaptchaID() string {
	return e.generateRandomString(16)
}

// generateSessionID 生成会话 ID
func (e *ChallengeEngine) generateSessionID() string {
	return e.generateRandomString(32)
}

// generateRandomString 生成随机字符串
func (e *ChallengeEngine) generateRandomString(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		result[i] = chars[n.Int64()]
	}
	return string(result)
}

// hashString 生成哈希
func (e *ChallengeEngine) hashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

// Get 获取会话
func (s *ChallengeSessionStore) Get(sessionID string) *ChallengeSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[sessionID]
}

// Set 设置会话
func (s *ChallengeSessionStore) Set(sessionID string, session *ChallengeSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = session
}

// Delete 删除会话
func (s *ChallengeSessionStore) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

// cleanupWorker 清理工
func (s *ChallengeSessionStore) cleanupWorker() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanup()
	}
}

// cleanup 清理过期会话
func (s *ChallengeSessionStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	deleted := 0

	for id, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, id)
			deleted++
		}
	}

	if deleted > 0 {
		fmt.Printf("清理过期挑战会话：%d 条\n", deleted)
	}
}

// GetStats 获取统计
func (e *ChallengeEngine) GetStats() *ChallengeStats {
	return e.stats
}

// GetScript 获取 JavaScript 挑战脚本
func (e *ChallengeEngine) GetScript(token string) string {
	// 返回需要在客户端执行的 JavaScript 代码
	return fmt.Sprintf(`
(function() {
	var token = '%s';
	var start = Date.now();
	var result = '';

	// 执行一些计算以证明是真实浏览器
	for (var i = 0; i < 1000000; i++) {
		result += Math.random().toString(36);
	}

	return {
		token: token,
		result: btoa(result.substring(0, 100)),
		time: Date.now() - start
	};
})()
`, token)
}

// GetChallengeHTML 获取挑战页面 HTML
func (e *ChallengeEngine) GetChallengeHTML(sessionID string, challengeType ChallengeType) string {
	switch challengeType {
	case ChallengeJavaScript:
		return e.getJavaScriptChallengeHTML(sessionID)
	case ChallengeCookie:
		return e.getCookieChallengeHTML(sessionID)
	case ChallengePoW:
		return e.getPoWChallengeHTML(sessionID)
	default:
		return "<html><body>Unknown challenge type</body></html>"
	}
}

func (e *ChallengeEngine) getJavaScriptChallengeHTML(sessionID string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
<title>Security Check</title>
<script>
(function() {
	var sessionID = '%s';
	var start = Date.now();
	var result = '';

	for (var i = 0; i < 1000000; i++) {
		result += Math.random().toString(36);
	}

	fetch('/challenge/verify', {
		method: 'POST',
		headers: {'Content-Type': 'application/json'},
		body: JSON.stringify({
			session_id: sessionID,
			result: btoa(result.substring(0, 100)),
			time: Date.now() - start
		})
	}).then(function(r) {
		if (r.ok) {
			window.location.reload();
		}
	});
})();
</script>
</head>
<body>
<h1>Verifying you are human...</h1>
<p>Please wait while we verify your browser.</p>
</body>
</html>
`, sessionID)
}

func (e *ChallengeEngine) getCookieChallengeHTML(sessionID string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
<title>Security Check</title>
<script>
document.cookie = "challenge_token=%s; path=/; max-age=300";
setTimeout(function() {
	fetch('/challenge/verify', {
		method: 'POST',
		headers: {'Content-Type': 'application/json'},
		body: JSON.stringify({
			session_id: '%s',
			cookie: document.cookie
		})
	}).then(function(r) {
		if (r.ok) {
			window.location.reload();
		}
	});
}, 1000);
</script>
</head>
<body>
<h1>Verifying you are human...</h1>
<p>Enabling cookies...</p>
</body>
</html>
`, e.generateCookieToken(), sessionID)
}

func (e *ChallengeEngine) getPoWChallengeHTML(sessionID string) string {
	params := e.generatePoWParams()
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
<title>Security Check</title>
<script>
(function() {
	var sessionID = '%s';
	var challenge = '%s';
	var salt = '%s';
	var difficulty = %d;

	function compute() {
		var prefix = '';
		for (var i = 0; i < difficulty; i++) prefix += '0';

		for (var nonce = 0; nonce < 10000000; nonce++) {
			var input = challenge + salt + nonce;
			var hash = simpleHash(input);
			if (hash.substring(0, difficulty) === prefix) {
				return {nonce: nonce, solution: hash};
			}
		}
		return null;
	}

	function simpleHash(str) {
		var hash = 0;
		for (var i = 0; i < str.length; i++) {
			hash = ((hash << 5) - hash) + str.charCodeAt(i);
			hash = hash & hash;
		}
		return Math.abs(hash).toString(16).padStart(32, '0');
	}

	var result = compute();
	if (result) {
		fetch('/challenge/verify', {
			method: 'POST',
			headers: {'Content-Type': 'application/json'},
			body: JSON.stringify({
				session_id: sessionID,
				nonce: result.nonce,
				solution: result.solution
			})
		}).then(function(r) {
			if (r.ok) {
				window.location.reload();
			}
		});
	}
})();
</script>
</head>
<body>
<h1>Verifying you are human...</h1>
<p>Solving cryptographic puzzle...</p>
</body>
</html>
`, sessionID, params.Challenge, params.Salt, params.Difficulty)
}

// GetStats 获取挑战统计
func (s *ChallengeSessionStore) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{
		"active_sessions": len(s.sessions),
	}
}
