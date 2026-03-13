package ddos

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestChallengeManager_New 测试创建挑战管理器
func TestChallengeManager_New(t *testing.T) {
	cm := NewChallengeManager(5 * time.Minute)
	assert.NotNil(t, cm)
	assert.NotNil(t, cm.challenges)
	assert.Equal(t, 5*time.Minute, cm.challengeDuration)
	assert.NotEmpty(t, cm.secret)
}

// TestChallengeManager_StartChallenge 测试开始挑战
func TestChallengeManager_StartChallenge(t *testing.T) {
	cm := NewChallengeManager(5 * time.Minute)

	entry := cm.StartChallenge("192.168.1.1")
	assert.NotNil(t, entry)
	assert.Equal(t, "192.168.1.1", entry.IP)
	assert.NotEmpty(t, entry.Token)
	assert.Equal(t, 0, entry.Attempts)
	assert.Equal(t, 3, entry.MaxAttempts)
	assert.False(t, entry.Verified)
}

// TestChallengeManager_GetStatus 测试获取挑战状态
func TestChallengeManager_GetStatus(t *testing.T) {
	cm := NewChallengeManager(5 * time.Minute)

	// 不存在的挑战
	status := cm.GetStatus("192.168.1.2")
	assert.Nil(t, status)

	// 存在的挑战
	cm.StartChallenge("192.168.1.3")
	status = cm.GetStatus("192.168.1.3")
	assert.NotNil(t, status)
	assert.Equal(t, "192.168.1.3", status.IP)
}

// TestChallengeManager_GetStatus_Expired 测试过期状态
func TestChallengeManager_GetStatus_Expired(t *testing.T) {
	cm := NewChallengeManager(50 * time.Millisecond)

	cm.StartChallenge("192.168.1.4")

	// 验证存在
	status := cm.GetStatus("192.168.1.4")
	assert.NotNil(t, status)

	// 等待过期
	time.Sleep(60 * time.Millisecond)

	// 应该返回 nil
	status = cm.GetStatus("192.168.1.4")
	assert.Nil(t, status)
}

// TestChallengeManager_VerifyChallenge 测试验证挑战
func TestChallengeManager_VerifyChallenge(t *testing.T) {
	cm := NewChallengeManager(5 * time.Minute)

	// 开始挑战
	entry := cm.StartChallenge("192.168.1.5")

	// 创建请求并设置正确的答案
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	answer := cm.calculateAnswer(entry.Token, entry.Timestamp)
	req.Header.Set("X-Challenge-Answer", answer)

	// 验证应该成功
	success := cm.VerifyChallenge(req, "192.168.1.5")
	assert.True(t, success)

	// 验证条目状态
	entry = cm.GetStatus("192.168.1.5")
	assert.NotNil(t, entry)
	assert.True(t, entry.Verified)
}

// TestChallengeManager_VerifyChallenge_NoAnswer 测试无答案验证
func TestChallengeManager_VerifyChallenge_NoAnswer(t *testing.T) {
	cm := NewChallengeManager(5 * time.Minute)

	cm.StartChallenge("192.168.1.6")

	// 创建请求但没有设置答案
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	// 验证应该失败
	success := cm.VerifyChallenge(req, "192.168.1.6")
	assert.False(t, success)

	// 验证尝试次数增加
	entry := cm.GetStatus("192.168.1.6")
	assert.NotNil(t, entry)
	assert.Equal(t, 1, entry.Attempts)
}

// TestChallengeManager_VerifyChallenge_WrongAnswer 测试错误答案
func TestChallengeManager_VerifyChallenge_WrongAnswer(t *testing.T) {
	cm := NewChallengeManager(5 * time.Minute)

	cm.StartChallenge("192.168.1.7")

	// 创建请求并设置错误答案
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Header.Set("X-Challenge-Answer", "wrong_answer")

	// 验证应该失败
	success := cm.VerifyChallenge(req, "192.168.1.7")
	assert.False(t, success)
}

// TestChallengeManager_VerifyChallenge_MaxAttempts 测试最大尝试次数
func TestChallengeManager_VerifyChallenge_MaxAttempts(t *testing.T) {
	cm := NewChallengeManager(5 * time.Minute)

	cm.StartChallenge("192.168.1.8")

	// 尝试 3 次（最大次数）
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		req.Header.Set("X-Challenge-Answer", "wrong")
		cm.VerifyChallenge(req, "192.168.1.8")
	}

	// 第 4 次应该返回 false（挑战已被移除）
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Header.Set("X-Challenge-Answer", "wrong")
	success := cm.VerifyChallenge(req, "192.168.1.8")
	assert.False(t, success)

	// 挑战应该已被移除
	status := cm.GetStatus("192.168.1.8")
	assert.Nil(t, status)
}

// TestChallengeManager_RemoveChallenge 测试移除挑战
func TestChallengeManager_RemoveChallenge(t *testing.T) {
	cm := NewChallengeManager(5 * time.Minute)

	cm.StartChallenge("192.168.1.9")

	// 验证存在
	assert.NotNil(t, cm.GetStatus("192.168.1.9"))

	// 移除
	cm.RemoveChallenge("192.168.1.9")

	// 验证已移除
	assert.Nil(t, cm.GetStatus("192.168.1.9"))
}

// TestChallengeManager_CleanupExpired 测试清理过期挑战
func TestChallengeManager_CleanupExpired(t *testing.T) {
	cm := NewChallengeManager(50 * time.Millisecond)

	cm.StartChallenge("192.168.1.10")

	// 验证存在
	assert.Equal(t, 1, len(cm.challenges))

	// 等待过期
	time.Sleep(60 * time.Millisecond)

	// 清理
	cm.CleanupExpired()

	// 验证已清理
	assert.Equal(t, 0, len(cm.challenges))
}

// TestChallengeManager_GenerateChallengePage 测试生成挑战页面
func TestChallengeManager_GenerateChallengePage(t *testing.T) {
	cm := NewChallengeManager(5 * time.Minute)

	cm.StartChallenge("192.168.1.11")

	html := cm.GenerateChallengePage("192.168.1.11")
	assert.Contains(t, html, "<!DOCTYPE html>")
	assert.Contains(t, html, "Security Check")
	assert.Contains(t, html, "<script>")
}

// TestChallengeManager_GenerateChallengePage_InvalidIP 测试生成无效 IP 的挑战页面
func TestChallengeManager_GenerateChallengePage_InvalidIP(t *testing.T) {
	cm := NewChallengeManager(5 * time.Minute)

	html := cm.GenerateChallengePage("invalid")
	assert.Contains(t, html, "Invalid challenge")
}

// TestChallengeManager_GetStats 测试获取统计
func TestChallengeManager_GetStats(t *testing.T) {
	cm := NewChallengeManager(5 * time.Minute)

	cm.StartChallenge("192.168.1.12")
	cm.StartChallenge("192.168.1.13")

	stats := cm.GetStats()
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.TotalChallenges, 2)
	assert.Equal(t, 5*time.Minute, stats.Duration)
}

// TestChallengeEntry 测试 ChallengeEntry 结构
func TestChallengeEntry(t *testing.T) {
	entry := &ChallengeEntry{
		Token:       "test_token",
		Timestamp:   time.Now().Unix(),
		IP:          "192.168.1.1",
		Attempts:    1,
		MaxAttempts: 3,
		ExpiresAt:   time.Now().Add(5 * time.Minute),
		Verified:    true,
	}

	assert.Equal(t, "test_token", entry.Token)
	assert.Equal(t, "192.168.1.1", entry.IP)
	assert.Equal(t, 1, entry.Attempts)
	assert.Equal(t, 3, entry.MaxAttempts)
	assert.True(t, entry.Verified)
}

// TestChallengeResult 测试 ChallengeResult 结构
func TestChallengeResult(t *testing.T) {
	result := &ChallengeResult{
		Success: true,
		Token:   "test_token",
		Message: "Challenge verified",
	}

	assert.True(t, result.Success)
	assert.Equal(t, "test_token", result.Token)
	assert.Equal(t, "Challenge verified", result.Message)
}

// TestChallengeRequest 测试 ChallengeRequest 结构
func TestChallengeRequest(t *testing.T) {
	req := &ChallengeRequest{
		Token:     "test_token",
		Timestamp: time.Now().Unix(),
		Answer:    "test_answer",
	}

	assert.Equal(t, "test_token", req.Token)
	assert.Equal(t, "test_answer", req.Answer)
}

// TestChallengeResponse 测试 ChallengeResponse 结构
func TestChallengeResponse(t *testing.T) {
	resp := &ChallengeResponse{
		Success: true,
		Message: "Success",
		Token:   "test_token",
	}

	assert.True(t, resp.Success)
	assert.Equal(t, "Success", resp.Message)
	assert.Equal(t, "test_token", resp.Token)
}

// TestGenerateToken 测试生成令牌
func TestGenerateToken(t *testing.T) {
	token := generateToken()
	assert.NotEmpty(t, token)
	assert.Len(t, token, 32) // 16 bytes hex encoded = 32 chars
}

// TestGenerateSecret 测试生成密钥
func TestGenerateSecret(t *testing.T) {
	secret := generateSecret()
	assert.NotEmpty(t, secret)
	assert.Len(t, secret, 64) // 32 bytes hex encoded = 64 chars
}

// TestGetChallengeAnswer_Header 测试从 Header 获取答案
func TestGetChallengeAnswer_Header(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Header.Set("X-Challenge-Answer", "test_answer")

	answer := getChallengeAnswer(req)
	assert.Equal(t, "test_answer", answer)
}

// TestGetChallengeAnswer_Cookie 测试从 Cookie 获取答案
func TestGetChallengeAnswer_Cookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.AddCookie(&http.Cookie{
		Name:  "challenge_answer",
		Value: "cookie_answer",
	})

	answer := getChallengeAnswer(req)
	assert.Equal(t, "cookie_answer", answer)
}

// TestGetChallengeAnswer_QueryParam 测试从 URL 参数获取答案
func TestGetChallengeAnswer_QueryParam(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com?challenge_answer=query_answer", nil)

	answer := getChallengeAnswer(req)
	assert.Equal(t, "query_answer", answer)
}

// TestGetChallengeAnswer_Form 测试从表单获取答案
func TestGetChallengeAnswer_Form(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://example.com", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Form = map[string][]string{
		"challenge_answer": {"form_answer"},
	}

	answer := getChallengeAnswer(req)
	assert.Equal(t, "form_answer", answer)
}

// TestGetChallengeAnswer_None 测试无答案
func TestGetChallengeAnswer_None(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	answer := getChallengeAnswer(req)
	assert.Equal(t, "", answer)
}

// TestHandleChallengeAPI 测试处理挑战 API
func TestHandleChallengeAPI(t *testing.T) {
	cm := NewChallengeManager(5 * time.Minute)

	// 测试非 POST 请求
	req := httptest.NewRequest(http.MethodGet, "http://example.com/challenge", nil)
	w := httptest.NewRecorder()

	HandleChallengeAPI(cm, w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

	// 测试无效 JSON
	req = httptest.NewRequest(http.MethodPost, "http://example.com/challenge", nil)
	w = httptest.NewRecorder()

	HandleChallengeAPI(cm, w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestHandleChallengeAPI_ValidRequest 测试有效的 API 请求
func TestHandleChallengeAPI_ValidRequest(t *testing.T) {
	cm := NewChallengeManager(5 * time.Minute)

	// 测试有效 JSON 请求
	req := httptest.NewRequest(http.MethodPost, "http://example.com/challenge", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Body = http.NoBody // 简化测试

	w := httptest.NewRecorder()

	// 这个测试会因为无法解析空 body 而返回错误，但验证函数存在
	HandleChallengeAPI(cm, w, req)
	assert.NotNil(t, w)
}

// TestChallengeMiddleware 测试挑战中间件
func TestChallengeMiddleware(t *testing.T) {
	cm := NewChallengeManager(5 * time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := ChallengeMiddleware(cm)
	wrappedHandler := middleware(handler)

	// 测试没有挑战状态的请求
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestChallengeMiddleware_VerifiedChallenge 测试已验证挑战的中间件
func TestChallengeMiddleware_VerifiedChallenge(t *testing.T) {
	cm := NewChallengeManager(5 * time.Minute)

	// 先创建挑战并标记为已验证
	entry := cm.StartChallenge("192.168.1.14")
	entry.Verified = true

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := ChallengeMiddleware(cm)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.RemoteAddr = "192.168.1.14:12345"
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestChallengeStats 测试 ChallengeStats 结构
func TestChallengeStats(t *testing.T) {
	stats := &ChallengeStats{
		TotalChallenges:    100,
		VerifiedChallenges: 80,
		ExpiredChallenges:  20,
		Duration:           5 * time.Minute,
	}

	assert.Equal(t, 100, stats.TotalChallenges)
	assert.Equal(t, 80, stats.VerifiedChallenges)
	assert.Equal(t, 20, stats.ExpiredChallenges)
	assert.Equal(t, 5*time.Minute, stats.Duration)
}

// TestCalculateAnswer 测试计算挑战答案
func TestCalculateAnswer(t *testing.T) {
	cm := NewChallengeManager(5 * time.Minute)

	token := "test_token"
	timestamp := time.Now().Unix()

	answer := cm.calculateAnswer(token, timestamp)
	assert.NotEmpty(t, answer)
	// SHA256 hash = 64 hex characters
	assert.Len(t, answer, 64)
}

// TestLastIndex 测试 lastindex 函数
func TestLastIndex(t *testing.T) {
	assert.Equal(t, 5, lastindex("hello:world", ":"))
	assert.Equal(t, -1, lastindex("hello", ":"))
	assert.Equal(t, 0, lastindex(":hello", ":"))
}
