package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestJWTManager_GenerateToken(t *testing.T) {
	config := &JWTConfig{
		SecretKey:  "test-secret-key-12345",
		ExpireTime: time.Hour * 24,
	}

	manager := NewJWTManager(config, nil)

	t.Run("GenerateToken_Success", func(t *testing.T) {
		token, err := manager.GenerateToken("user-123", "testuser")

		assert.NoError(t, err)
		assert.NotEmpty(t, token)
		assert.Contains(t, token, ".") // JWT 格式：header.payload.signature
	})

	t.Run("GenerateToken_WithEmptyUsername", func(t *testing.T) {
		token, err := manager.GenerateToken("user-456", "")

		assert.NoError(t, err)
		assert.NotEmpty(t, token)
	})
}

func TestJWTManager_ValidateToken(t *testing.T) {
	config := &JWTConfig{
		SecretKey:  "test-secret-key-12345",
		ExpireTime: time.Hour * 24,
	}

	manager := NewJWTManager(config, nil)

	t.Run("ValidateToken_ValidToken", func(t *testing.T) {
		// 生成有效令牌
		tokenString, err := manager.GenerateToken("user-123", "testuser")
		assert.NoError(t, err)

		// 验证令牌
		claims, err := manager.ValidateToken(tokenString)

		assert.NoError(t, err)
		assert.NotNil(t, claims)
		assert.Equal(t, "user-123", claims.UserID)
		assert.Equal(t, "testuser", claims.Username)
		assert.NotEmpty(t, claims.SessionID)
	})

	t.Run("ValidateToken_InvalidToken", func(t *testing.T) {
		claims, err := manager.ValidateToken("invalid-token")

		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Equal(t, ErrInvalidToken, err)
	})

	t.Run("ValidateToken_EmptyToken", func(t *testing.T) {
		claims, err := manager.ValidateToken("")

		assert.Error(t, err)
		assert.Nil(t, claims)
	})

	t.Run("ValidateToken_WrongSecret", func(t *testing.T) {
		// 用不同的密钥生成令牌
		wrongConfig := &JWTConfig{
			SecretKey:  "wrong-secret-key",
			ExpireTime: time.Hour * 24,
		}
		wrongManager := NewJWTManager(wrongConfig, nil)
		tokenString, _ := wrongManager.GenerateToken("user-123", "testuser")

		// 用正确的密钥验证
		claims, err := manager.ValidateToken(tokenString)

		assert.Error(t, err)
		assert.Nil(t, claims)
	})
}

func TestJWTManager_ValidateToken_Expired(t *testing.T) {
	config := &JWTConfig{
		SecretKey:  "test-secret-key-12345",
		ExpireTime: time.Second * 1, // 1 秒过期
	}

	manager := NewJWTManager(config, nil)

	// 生成令牌
	tokenString, err := manager.GenerateToken("user-123", "testuser")
	assert.NoError(t, err)

	// 等待过期
	time.Sleep(time.Second * 2)

	// 验证过期令牌
	claims, err := manager.ValidateToken(tokenString)

	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Equal(t, ErrExpiredToken, err)
}

func TestJWTManager_RevokeToken(t *testing.T) {
	config := &JWTConfig{
		SecretKey:  "test-secret-key-12345",
		ExpireTime: time.Hour * 24,
	}

	manager := NewJWTManager(config, nil)

	t.Run("RevokeToken_WithoutRedis", func(t *testing.T) {
		tokenString, err := manager.GenerateToken("user-123", "testuser")
		assert.NoError(t, err)

		// 没有 Redis 时，RevokeToken 应该返回 nil（无操作）
		err = manager.RevokeToken(tokenString)
		assert.NoError(t, err)
	})

	t.Run("RevokeToken_InvalidToken", func(t *testing.T) {
		err := manager.RevokeToken("invalid-token")

		assert.Error(t, err)
	})
}

func TestClaims_Struct(t *testing.T) {
	claims := &Claims{
		UserID:    "user-123",
		Username:  "testuser",
		SessionID: "session-456",
	}

	assert.Equal(t, "user-123", claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, "session-456", claims.SessionID)
}

func TestJWTConfig_Defaults(t *testing.T) {
	config := &JWTConfig{
		SecretKey:  "test-key",
		ExpireTime: time.Hour * 24,
	}

	assert.Equal(t, "test-key", config.SecretKey)
	assert.Equal(t, time.Hour*24, config.ExpireTime)
}
