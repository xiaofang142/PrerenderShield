package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"prerender-shield/internal/audit"
	"prerender-shield/internal/auth"
	"prerender-shield/internal/redis"
)

// userCtxMiddleware 注入 user_id（模拟 JWT 中间件）
func userCtxMiddleware(userID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	}
}

var authTestSeq int64

func uniqueUsername(prefix string) string {
	authTestSeq++
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), authTestSeq)
}

// createAuthUser 在 DB15 中创建认证用户（走 CreateUser 权威路径），返回用户 ID 并注册清理
func createAuthUser(t *testing.T, client *redis.Client, username, password string) string {
	t.Helper()
	client.Del("username:" + username)
	um := auth.NewUserManager("", client)
	user, err := um.CreateUser(username, password)
	require.NoError(t, err)
	t.Cleanup(func() {
		client.Del("user:" + user.ID)
		client.Del("username:" + username)
	})
	return user.ID
}

// mustAuthController 构建带 DB15 依赖的 AuthController（审计开启，无 2FA）
func mustAuthController(t *testing.T, client *redis.Client) *AuthController {
	t.Helper()
	userManager := auth.NewUserManager("", client)
	jwtManager := auth.NewJWTManager(&auth.JWTConfig{
		SecretKey:  "test-secret-key-for-testing-only-32bytes",
		ExpireTime: 3600000000000,
	}, client)
	auditLogger := audit.NewLogger(client, audit.Config{})
	return NewAuthController(userManager, jwtManager, auditLogger, nil)
}

// setupAuthWithRedis 使用 DB15 Redis 构建完整认证控制器（含审计与 2FA），注入唯一 user_id
func setupAuthWithRedis(t *testing.T) (*AuthController, *gin.Engine, *redis.Client, string) {
	t.Helper()
	client := newTestRedisDB15(t)

	userID := "ctl-user-" + uniqueUsername("u")

	cleanup := func() {
		for _, k := range []string{"user:" + userID, "2fa:pending:" + userID, "2fa:secret:" + userID} {
			client.Del(k)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	userManager := auth.NewUserManager("", client)
	jwtManager := auth.NewJWTManager(&auth.JWTConfig{
		SecretKey:  "test-secret-key-for-testing-only-32bytes",
		ExpireTime: 3600000000000,
	}, client)
	auditLogger := audit.NewLogger(client, audit.Config{})
	twoFactor := auth.NewTwoFactorAuth(client, "prerender-shield-test")

	controller := NewAuthController(userManager, jwtManager, auditLogger, twoFactor)

	router := ginNewRouter()
	authed := router.Group("/", userCtxMiddleware(userID))
	authed.GET("/auth/2fa/status", controller.Get2FAStatus)
	authed.POST("/auth/2fa/enable", controller.Enable2FA)
	authed.POST("/auth/2fa/confirm", controller.Confirm2FA)
	authed.POST("/auth/2fa/disable", controller.Disable2FA)
	authed.POST("/auth/change-password", controller.ChangePassword)
	return controller, router, client, userID
}

func TestAuthController_Enable2FA_NotConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewAuthController(auth.NewUserManager("", nil), nil, nil, nil)
	router := ginNewRouter()
	router.POST("/auth/2fa/enable", controller.Enable2FA)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/2fa/enable", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "2FA not configured")
}

func TestAuthController_Get2FAStatus_NotConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewAuthController(auth.NewUserManager("", nil), nil, nil, nil)
	router := ginNewRouter()
	router.GET("/auth/2fa/status", controller.Get2FAStatus)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/2fa/status", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"available":false`)
}

func TestAuthController_2FA_FullFlow(t *testing.T) {
	_, router, _, _ := setupAuthWithRedis(t)

	// 1. 状态查询：未启用
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/2fa/status", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"enabled":false`)
	assert.Contains(t, w.Body.String(), `"available":true`)

	// 2. 开启 2FA
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/2fa/enable", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var enableResp struct {
		Data struct {
			Secret string `json:"secret"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &enableResp))
	require.NotEmpty(t, enableResp.Data.Secret)

	// 3. 错误码确认 → 400
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/2fa/confirm", jsonBody(t, map[string]string{"code": "000000"}))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 4. 缺 code → 400
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/auth/2fa/confirm", jsonBody(t, map[string]string{}))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 5. 正确 TOTP 码确认 → 200
	code, err := totp.GenerateCode(enableResp.Data.Secret, time.Now())
	require.NoError(t, err)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/auth/2fa/confirm", jsonBody(t, map[string]string{"code": code}))
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "2FA enabled successfully")

	// 6. 状态查询：已启用
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/2fa/status", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"enabled":true`)

	// 7. 关闭 2FA：缺 code → 400
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/auth/2fa/disable", jsonBody(t, map[string]string{}))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 8. 关闭 2FA：错误码 → 400
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/auth/2fa/disable", jsonBody(t, map[string]string{"code": "000000"}))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 9. 关闭 2FA：正确码 → 200
	code, err = totp.GenerateCode(enableResp.Data.Secret, time.Now())
	require.NoError(t, err)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/auth/2fa/disable", jsonBody(t, map[string]string{"code": code}))
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "2FA disabled successfully")

	// 10. 未发起 setup 时确认 → 400（无 pending secret）
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/auth/2fa/confirm", jsonBody(t, map[string]string{"code": "123456"}))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "not initiated")
}

func TestAuthController_ChangePassword_InvalidBody(t *testing.T) {
	_, router, _, _ := setupAuthWithRedis(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/change-password", strBody("bad"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthController_ChangePassword_WeakPassword(t *testing.T) {
	_, router, _, _ := setupAuthWithRedis(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/change-password",
		jsonBody(t, map[string]string{"old_password": "old", "new_password": "abc"}))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthController_ChangePassword_UserNotFound(t *testing.T) {
	_, router, _, _ := setupAuthWithRedis(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/change-password",
		jsonBody(t, map[string]string{"old_password": "oldpass1", "new_password": "newpass1"}))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestAuthController_ChangePassword_Flow 真实用户：错旧密码 → 400；正确旧密码 → 200
func TestAuthController_ChangePassword_Flow(t *testing.T) {
	client := newTestRedisDB15(t)
	username := uniqueUsername("ctluser")
	realUserID := createAuthUser(t, client, username, "oldpass1")

	controller := mustAuthController(t, client)
	router := ginNewRouter()
	router.POST("/auth/change-password", userCtxMiddleware(realUserID), controller.ChangePassword)

	// 错误旧密码 → 400 + 提示
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/change-password",
		jsonBody(t, map[string]string{"old_password": "wrongold1", "new_password": "newpass1"}))
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Current password is incorrect")

	// 正确旧密码 → 200
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/auth/change-password",
		jsonBody(t, map[string]string{"old_password": "oldpass1", "new_password": "newpass9"}))
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Password changed successfully")
}

// TestAuthController_Login_FirstRun_CreateFailed 首跑创建用户失败（弱密码）→ 500 + 审计
func TestAuthController_Login_FirstRun_CreateFailed(t *testing.T) {
	client := newTestRedisDB15(t)
	// nil redis 的 UserManager：IsFirstRun 恒为 true，CreateUser 在密码强度校验处失败
	um := auth.NewUserManager("", nil)
	jwtManager := auth.NewJWTManager(&auth.JWTConfig{
		SecretKey:  "test-secret-key-for-testing-only-32bytes",
		ExpireTime: 3600000000000,
	}, nil)
	auditLogger := audit.NewLogger(client, audit.Config{})
	controller := NewAuthController(um, jwtManager, auditLogger, nil)

	router := ginNewRouter()
	router.POST("/auth/login", controller.Login)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/login", jsonBody(t, map[string]string{"username": "admin", "password": "weak"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestAuthController_Login_AuthFailed 非首跑 + 错误凭证 → 401 + 审计
func TestAuthController_Login_AuthFailed(t *testing.T) {
	client := newTestRedisDB15(t)
	username := uniqueUsername("ctluser-authfail")
	createAuthUser(t, client, username, "realpass1")

	jwtManager := auth.NewJWTManager(&auth.JWTConfig{
		SecretKey:  "test-secret-key-for-testing-only-32bytes",
		ExpireTime: 3600000000000,
	}, nil)
	auditLogger := audit.NewLogger(client, audit.Config{})
	controller := NewAuthController(auth.NewUserManager("", client), jwtManager, auditLogger, nil)

	router := ginNewRouter()
	router.POST("/auth/login", controller.Login)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/login", jsonBody(t, map[string]string{"username": username, "password": "wrongpass1"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestAuthController_Login_Success_Audited 登录成功 + 审计 + force_change_password 分支 + 登出
func TestAuthController_Login_Success_Audited(t *testing.T) {
	client := newTestRedisDB15(t)
	username := uniqueUsername("ctluser-ok")
	createAuthUser(t, client, username, "realpass1")

	jwtManager := auth.NewJWTManager(&auth.JWTConfig{
		SecretKey:  "test-secret-key-for-testing-only-32bytes",
		ExpireTime: 3600000000000,
	}, client)
	auditLogger := audit.NewLogger(client, audit.Config{})
	controller := NewAuthController(auth.NewUserManager("", client), jwtManager, auditLogger, nil)

	router := ginNewRouter()
	router.POST("/auth/login", controller.Login)
	router.POST("/auth/logout", controller.Logout)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/login", jsonBody(t, map[string]string{"username": username, "password": "realpass1"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data struct {
			Token             string `json:"token"`
			Username          string `json:"username"`
			ForceChangePasswd bool   `json:"force_change_password"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Data.Token)
	assert.Equal(t, username, resp.Data.Username)

	// Logout 携带 token（RevokeToken 落 Redis）
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req2.Header.Set("Authorization", "Bearer "+resp.Data.Token)
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

// TestAuthController_Login_TokenSaveFailed 会话写 Redis 失败 → 500
func TestAuthController_Login_TokenSaveFailed(t *testing.T) {
	client := newTestRedisDB15(t)
	username := uniqueUsername("ctluser-tokfail")
	createAuthUser(t, client, username, "realpass1")

	// jwtManager 使用已关闭的客户端 → SaveSession 失败 → GenerateToken 返回错误
	closed := closedTestRedisDB15(t)
	jwtManager := auth.NewJWTManager(&auth.JWTConfig{
		SecretKey:  "test-secret-key-for-testing-only-32bytes",
		ExpireTime: 3600000000000,
	}, closed)
	controller := NewAuthController(auth.NewUserManager("", client), jwtManager, nil, nil)

	router := ginNewRouter()
	router.POST("/auth/login", controller.Login)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/login", jsonBody(t, map[string]string{"username": username, "password": "realpass1"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to generate token")
}
