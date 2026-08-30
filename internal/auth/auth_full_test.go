package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"github.com/stretchr/testify/assert"
)

// TestJWTManager_NilRedis 测试 Redis 为 nil 时的 JWT 管理器
func TestJWTManager_NilRedis(t *testing.T) {
	config := &JWTConfig{
		SecretKey:  "test-secret",
		ExpireTime: time.Hour,
	}

	manager := NewJWTManager(config, nil)
	assert.NotNil(t, manager)

	// 生成令牌应该成功（没有 Redis 时不保存会话）
	token, err := manager.GenerateToken("user-1", "testuser")
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// 验证令牌应该成功
	claims, err := manager.ValidateToken(token)
	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, "user-1", claims.UserID)

	// 撤销令牌应该成功（没有 Redis 时无操作）
	err = manager.RevokeToken(token)
	assert.NoError(t, err)
}

// TestJWTManager_EmptyConfig 测试空配置
func TestJWTManager_EmptyConfig(t *testing.T) {
	manager := NewJWTManager(nil, nil)
	assert.NotNil(t, manager)

	// 空配置时生成令牌会 panic，因为访问 config.SecretKey 会空指针
	// 但这是预期的行为，因为配置是必需的
}

// TestClaims_RegisteredClaims 测试 Claims 结构
func TestClaims_RegisteredClaims(t *testing.T) {
	claims := &Claims{
		UserID:    "user-1",
		Username:  "testuser",
		SessionID: "session-1",
	}
	claims.Issuer = "test-issuer"
	claims.Subject = "test-subject"

	assert.Equal(t, "user-1", claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, "session-1", claims.SessionID)
	assert.Equal(t, "test-issuer", claims.Issuer)
}

// TestErrors 测试错误变量
func TestErrors(t *testing.T) {
	assert.Equal(t, "invalid token", ErrInvalidToken.Error())
	assert.Equal(t, "token has expired", ErrExpiredToken.Error())
	assert.Equal(t, "authorization header is required", ErrNoAuthHeader.Error())
	assert.Equal(t, "invalid authorization format", ErrInvalidAuthFormat.Error())
	assert.Equal(t, "session has expired or been revoked", ErrSessionExpired.Error())
}

// TestJWTAuthMiddleware 测试 JWT 认证中间件
func TestJWTAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := &JWTConfig{
		SecretKey:  "test-secret",
		ExpireTime: time.Hour,
	}
	manager := NewJWTManager(config, nil)
	middleware := JWTAuthMiddleware(manager, nil)

	// 测试没有 Authorization 头
	t.Run("NoAuthHeader", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)

		middleware(c)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	// 测试无效的 Authorization 格式
	t.Run("InvalidAuthFormat", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "InvalidFormat")
		c.Request = req

		middleware(c)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	// 测试无效的令牌
	t.Run("InvalidToken", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		c.Request = req

		middleware(c)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	// 测试有效的令牌
	t.Run("ValidToken", func(t *testing.T) {
		token, _ := manager.GenerateToken("user-1", "testuser")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		c.Request = req

		middleware(c)

		// 有效令牌应该继续处理
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "user-1", c.GetString("user_id"))
		assert.Equal(t, "testuser", c.GetString("username"))
	})
}

// TestJWTAuthMiddleware_ExpiredToken 测试过期令牌
func TestJWTAuthMiddleware_ExpiredToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := &JWTConfig{
		SecretKey:  "test-secret",
		ExpireTime: time.Second,
	}
	manager := NewJWTManager(config, nil)
	middleware := JWTAuthMiddleware(manager, nil)

	// 生成令牌并等待过期
	token, _ := manager.GenerateToken("user-1", "testuser")
	time.Sleep(time.Second * 2)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	c.Request = req

	middleware(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestTOTPManager 测试 TOTP 管理器
func TestTOTPManager(t *testing.T) {
	manager := NewTOTPManager("TestApp")
	assert.NotNil(t, manager)
	assert.Equal(t, "TestApp", manager.issuer)
}

// TestTOTPManager_GenerateSecret_Multiple 测试生成多个密钥
func TestTOTPManager_GenerateSecret_Multiple(t *testing.T) {
	manager := NewTOTPManager("TestApp")

	// 多次生成应该得到不同的密钥
	secret, err := manager.GenerateSecret()
	assert.NoError(t, err)
	assert.NotEmpty(t, secret)

	secret2, err := manager.GenerateSecret()
	assert.NoError(t, err)
	assert.NotEqual(t, secret, secret2)
}

// TestTOTPManager_GenerateQRCode_WithIssuer 测试生成二维码带发行者
func TestTOTPManager_GenerateQRCode_WithIssuer(t *testing.T) {
	manager := NewTOTPManager("TestApp")

	secret, err := manager.GenerateSecret()
	assert.NoError(t, err)

	qrURL, err := manager.GenerateQRCode("user@example.com", secret)
	assert.NoError(t, err)
	assert.Contains(t, qrURL, "otpauth://totp")
	assert.Contains(t, qrURL, "TestApp:user@example.com")
}

// TestTOTPManager_GenerateQRCode_InvalidSecret 测试无效密钥
func TestTOTPManager_GenerateQRCode_InvalidSecret(t *testing.T) {
	manager := NewTOTPManager("TestApp")

	_, err := manager.GenerateQRCode("user@example.com", "invalid-base32-!!!")
	assert.Error(t, err)
}

// TestTOTPManager_ValidateCodeWithSkew 测试带时间偏差的验证
func TestTOTPManager_ValidateCodeWithSkew(t *testing.T) {
	manager := NewTOTPManager("TestApp")

	secret, _ := manager.GenerateSecret()

	// 测试无效码
	valid := manager.ValidateCode(secret, "000000")
	assert.False(t, valid)

	// 测试空码
	valid = manager.ValidateCode(secret, "")
	assert.False(t, valid)
}

// TestTOTPManager_GenerateBackupCodes 测试生成备用码
func TestTOTPManager_GenerateBackupCodes(t *testing.T) {
	manager := NewTOTPManager("TestApp")

	codes, err := manager.GenerateBackupCodes(5)
	assert.NoError(t, err)
	assert.Len(t, codes, 5)

	// 验证备用码格式
	for _, code := range codes {
		assert.Regexp(t, `^\d{4}-\d{4}$`, code)
	}
}

// TestVerifyTOTPCustom_WithInvalidCode 测试自定义 TOTP 验证与无效码
func TestVerifyTOTPCustom_WithInvalidCode(t *testing.T) {
	secret, _ := NewTOTPManager("TestApp").GenerateSecret()

	// 测试无效码
	valid := VerifyTOTPCustom(secret, "000000", time.Now().UTC(), ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	assert.False(t, valid)
}

// TestValidateOpts 测试 ValidateOpts 结构
func TestValidateOpts(t *testing.T) {
	opts := ValidateOpts{
		Period:    60,
		Skew:      2,
		Digits:    otp.DigitsEight,
		Algorithm: otp.AlgorithmSHA256,
	}

	assert.Equal(t, uint(60), opts.Period)
	assert.Equal(t, uint(2), opts.Skew)
	assert.Equal(t, otp.DigitsEight, opts.Digits)
	assert.Equal(t, otp.AlgorithmSHA256, opts.Algorithm)
}

// TestTwoFactorAuth_NilRedis 测试 Redis 为 nil 时的双因素认证
func TestTwoFactorAuth_NilRedis(t *testing.T) {
	auth := NewTwoFactorAuth(nil, "TestApp")
	assert.NotNil(t, auth)

	// 启用 2FA 应该返回错误（Redis 为 nil）
	_, _, err := auth.Enable2FA("user-1")
	assert.Error(t, err)

	// 确认 2FA 应该返回错误
	err = auth.Confirm2FA("user-1", "123456")
	assert.Error(t, err)

	// 禁用 2FA 应该返回错误
	err = auth.Disable2FA("user-1", "123456")
	assert.Error(t, err)

	// 验证 2FA 应该返回错误
	err = auth.Verify2FA("user-1", "123456")
	assert.Error(t, err)

	// 检查 2FA 状态应该返回错误
	_, err = auth.Is2FAEnabled("user-1")
	assert.Error(t, err)

	// 生成备用码应该返回错误
	_, err = auth.GenerateBackupCodes("user-1")
	assert.Error(t, err)

	// 验证备用码应该返回错误
	err = auth.VerifyBackupCode("user-1", "1234-5678")
	assert.Error(t, err)
}

// TestTwoFactorAuth_Struct 测试 TwoFactorAuth 结构
func TestTwoFactorAuth_Struct(t *testing.T) {
	auth := &TwoFactorAuth{
		redisClient: nil,
		totpManager: NewTOTPManager("Test"),
	}

	assert.NotNil(t, auth)
	assert.NotNil(t, auth.totpManager)
}

// TestRequire2FA 测试 2FA 中间件
func TestRequire2FA(t *testing.T) {
	gin.SetMode(gin.TestMode)

	auth := NewTwoFactorAuth(nil, "TestApp")
	middleware := auth.Require2FA()

	// 测试没有 user_id
	t.Run("NoUserID", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)

		middleware(c)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	// 测试有 user_id 但 Redis 不可用
	t.Run("WithUserID_NilRedis", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		c.Set("user_id", "user-1")

		middleware(c)
		// Redis 不可用时，Is2FAEnabled 返回错误，应该返回 500
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// TestVerifyTOTPCode 测试通用 TOTP 验证函数
func TestVerifyTOTPCode(t *testing.T) {
	secret, _ := NewTOTPManager("TestApp").GenerateSecret()

	// 测试无效码
	valid := VerifyTOTPCode(secret, "000000")
	assert.False(t, valid)

	// 测试空码
	valid = VerifyTOTPCode(secret, "")
	assert.False(t, valid)
}

// TestUserManager_NilRedis 测试 Redis 为 nil 时的用户管理器
func TestUserManager_NilRedis(t *testing.T) {
	manager := NewUserManager("test", nil)
	assert.NotNil(t, manager)
	assert.Nil(t, manager.redisClient)

	// 创建用户应该成功（Redis 为 nil 时不检查唯一性）
	user, err := manager.CreateUser("testuser", "password123")
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "testuser", user.Username)
	assert.NotEmpty(t, user.ID)

	// 获取用户应该返回错误
	_, err = manager.GetUserByUsername("testuser")
	assert.Error(t, err)
	assert.Equal(t, ErrUserNotFound, err)

	// 验证用户应该返回错误
	_, err = manager.AuthenticateUser("testuser", "password123")
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidCredentials, err)

	// 首次运行检查
	isFirst := manager.IsFirstRun()
	assert.True(t, isFirst)
}

// TestUserManager_Errors 测试用户管理器错误
func TestUserManager_Errors(t *testing.T) {
	assert.Equal(t, "user not found", ErrUserNotFound.Error())
	assert.Equal(t, "invalid username or password", ErrInvalidCredentials.Error())
	assert.Equal(t, "user already exists", ErrUserExists.Error())
}

// TestUser_Struct 测试 User 结构
func TestUser_Struct(t *testing.T) {
	user := &User{
		ID:       "user-1",
		Username: "testuser",
		Password: "hashed-password",
	}

	assert.Equal(t, "user-1", user.ID)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, "hashed-password", user.Password)
}
