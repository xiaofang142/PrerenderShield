package ddos

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ChallengeManager 挑战管理器
type ChallengeManager struct {
	mu              sync.RWMutex
	challenges      map[string]*ChallengeEntry // key: IP
	challengeDuration time.Duration
	secret          string
}

// ChallengeEntry 挑战条目
type ChallengeEntry struct {
	Token       string
	Timestamp   int64
	IP          string
	Attempts    int
	MaxAttempts int
	ExpiresAt   time.Time
	Verified    bool
}

// ChallengeResult 挑战结果
type ChallengeResult struct {
	Success bool
	Token   string
	Message string
}

// NewChallengeManager 创建挑战管理器
func NewChallengeManager(duration time.Duration) *ChallengeManager {
	secret := generateSecret()
	return &ChallengeManager{
		challenges:      make(map[string]*ChallengeEntry),
		challengeDuration: duration,
		secret:          secret,
	}
}

// StartChallenge 开始挑战
func (c *ChallengeManager) StartChallenge(ip string) *ChallengeEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	token := generateToken()
	timestamp := time.Now().Unix()
	expiresAt := time.Now().Add(c.challengeDuration)

	entry := &ChallengeEntry{
		Token:       token,
		Timestamp:   timestamp,
		IP:          ip,
		Attempts:    0,
		MaxAttempts: 3,
		ExpiresAt:   expiresAt,
		Verified:    false,
	}

	c.challenges[ip] = entry

	return entry
}

// GetStatus 获取挑战状态
func (c *ChallengeManager) GetStatus(ip string) *ChallengeEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.challenges[ip]
	if !exists {
		return nil
	}

	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) {
		return nil
	}

	return entry
}

// VerifyChallenge 验证挑战响应
func (c *ChallengeManager) VerifyChallenge(req *http.Request, ip string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.challenges[ip]
	if !exists {
		return false
	}

	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) {
		delete(c.challenges, ip)
		return false
	}

	// 检查是否已达到最大尝试次数
	if entry.Attempts >= entry.MaxAttempts {
		delete(c.challenges, ip)
		return false
	}

	// 从请求中获取挑战响应
	answer := getChallengeAnswer(req)
	if answer == "" {
		entry.Attempts++
		return false
	}

	// 计算正确答案
	expectedAnswer := c.calculateAnswer(entry.Token, entry.Timestamp)

	if answer == expectedAnswer {
		entry.Verified = true
		return true
	}

	entry.Attempts++
	return false
}

// RemoveChallenge 移除挑战
func (c *ChallengeManager) RemoveChallenge(ip string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.challenges, ip)
}

// CleanupExpired 清理过期挑战
func (c *ChallengeManager) CleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for ip, entry := range c.challenges {
		if now.After(entry.ExpiresAt) || entry.Attempts >= entry.MaxAttempts {
			delete(c.challenges, ip)
		}
	}
}

// GenerateChallengePage 生成挑战页面 HTML
func (c *ChallengeManager) GenerateChallengePage(ip string) string {
	entry := c.GetStatus(ip)
	if entry == nil {
		return "<html><body>Invalid challenge</body></html>"
	}

	// 生成客户端挑战脚本
	script := c.generateClientScript(entry.Token, entry.Timestamp)

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Security Challenge</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
        }
        .container {
            background: white;
            padding: 40px;
            border-radius: 10px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.2);
            text-align: center;
        }
        h1 { color: #333; }
        p { color: #666; }
        .spinner {
            border: 4px solid #f3f3f3;
            border-top: 4px solid #667eea;
            border-radius: 50%%;
            width: 40px;
            height: 40px;
            animation: spin 1s linear infinite;
            margin: 20px auto;
        }
        @keyframes spin {
            0%% { transform: rotate(0deg); }
            100%% { transform: rotate(360deg); }
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Security Check</h1>
        <p>Verifying your request...</p>
        <div class="spinner"></div>
        <p>Please wait while we verify you're human.</p>
        %s
    </div>
</body>
</html>`, script)
}

// generateClientScript 生成客户端验证脚本
func (c *ChallengeManager) generateClientScript(token string, timestamp int64) string {
	answer := c.calculateAnswer(token, timestamp)

	return fmt.Sprintf(`<script>
(function() {
    var token = "%s";
    var timestamp = %d;

    // 计算挑战答案
    var challenge = token + timestamp + "%s";

    // 使用 SHA256 计算（简化版本，实际应该用 crypto-js）
    fetch(window.location.href, {
        method: 'GET',
        headers: {
            'X-Challenge-Token': token,
            'X-Challenge-Timestamp': timestamp.toString(),
            'X-Challenge-Answer': "%s"
        }
    }).then(function(response) {
        if (response.ok) {
            window.location.reload();
        }
    });
})();
</script>`, token, timestamp, c.secret, answer)
}

// calculateAnswer 计算挑战答案
func (c *ChallengeManager) calculateAnswer(token string, timestamp int64) string {
	challenge := fmt.Sprintf("%s%d%s", token, timestamp, c.secret)
	hash := sha256.Sum256([]byte(challenge))
	return hex.EncodeToString(hash[:])
}

// GetStats 获取挑战统计
func (c *ChallengeManager) GetStats() *ChallengeStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := len(c.challenges)
	verified := 0
	expired := 0

	now := time.Now()
	for _, entry := range c.challenges {
		if entry.Verified {
			verified++
		}
		if now.After(entry.ExpiresAt) {
			expired++
		}
	}

	return &ChallengeStats{
		TotalChallenges:   total,
		VerifiedChallenges: verified,
		ExpiredChallenges: expired,
		Duration:          c.challengeDuration,
	}
}

// ChallengeStats 挑战统计
type ChallengeStats struct {
	TotalChallenges    int           `json:"total_challenges"`
	VerifiedChallenges int           `json:"verified_challenges"`
	ExpiredChallenges  int           `json:"expired_challenges"`
	Duration           time.Duration `json:"duration"`
}

// generateToken 生成随机令牌
func generateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// generateSecret 生成随机密钥
func generateSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// getChallengeAnswer 从请求中获取挑战答案
func getChallengeAnswer(req *http.Request) string {
	// 从 Header 中获取
	answer := req.Header.Get("X-Challenge-Answer")
	if answer != "" {
		return answer
	}

	// 从 Cookie 中获取
	cookie, err := req.Cookie("challenge_answer")
	if err == nil {
		return cookie.Value
	}

	// 从 URL 参数中获取
	answer = req.URL.Query().Get("challenge_answer")
	if answer != "" {
		return answer
	}

	// 尝试从表单中获取（POST 请求）
	if req.Method == http.MethodPost {
		req.ParseForm()
		answer = req.FormValue("challenge_answer")
		if answer != "" {
			return answer
		}
	}

	return ""
}

// ChallengeMiddleware 创建挑战中间件
func ChallengeMiddleware(manager *ChallengeManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)

			// 检查是否处于挑战状态
			entry := manager.GetStatus(ip)
			if entry != nil && !entry.Verified {
				// 验证挑战
				if manager.VerifyChallenge(r, ip) {
					// 验证成功，继续处理
					next.ServeHTTP(w, r)
					return
				}

				// 验证失败，返回挑战页面
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(manager.GenerateChallengePage(ip)))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ChallengeRequest JSON 挑战请求
type ChallengeRequest struct {
	Token     string `json:"token"`
	Timestamp int64  `json:"timestamp"`
	Answer    string `json:"answer"`
}

// ChallengeResponse JSON 挑战响应
type ChallengeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Token   string `json:"token,omitempty"`
}

// HandleChallengeAPI 处理挑战 API 请求
func HandleChallengeAPI(manager *ChallengeManager, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req ChallengeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChallengeResponse{
			Success: false,
			Message: "Invalid request",
		})
		return
	}

	// 这里可以添加额外的验证逻辑
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ChallengeResponse{
		Success: true,
		Message: "Challenge verified",
	})
}

// getClientIP 获取客户端 IP
func getClientIP(r *http.Request) string {
	// 检查 X-Forwarded-For
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		return xff
	}

	// 检查 X-Real-IP
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// 使用 RemoteAddr
	ip := r.RemoteAddr
	// 去除端口
	if idx := lastindex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

func lastindex(s, sep string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}
