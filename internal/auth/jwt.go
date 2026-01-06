package auth

import (
	"errors"
	"fmt"
	"prerender-shield/internal/redis"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	// 错误定义
	ErrInvalidToken      = errors.New("invalid token")
	ErrExpiredToken      = errors.New("token has expired")
	ErrNoAuthHeader      = errors.New("authorization header is required")
	ErrInvalidAuthFormat = errors.New("invalid authorization format")
	ErrSessionExpired    = errors.New("session has expired or been revoked")
)

// JWTConfig JWT配置
type JWTConfig struct {
	SecretKey  string        `yaml:"secret_key"`
	ExpireTime time.Duration `yaml:"expire_time"`
}

// Claims JWT声明
type Claims struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	SessionID string `json:"session_id"` // 添加SessionID
	jwt.RegisteredClaims
}

// JWTManager JWT管理器
type JWTManager struct {
	config      *JWTConfig
	redisClient *redis.Client // 添加Redis客户端
}

// NewJWTManager 创建JWT管理器
func NewJWTManager(config *JWTConfig, redisClient *redis.Client) *JWTManager {
	return &JWTManager{
		config:      config,
		redisClient: redisClient,
	}
}

// GenerateToken 生成JWT令牌
func (m *JWTManager) GenerateToken(userID, username string) (string, error) {
	// 生成唯一的SessionID
	sessionID := uuid.New().String()

	// 创建声明
	claims := &Claims{
		UserID:    userID,
		Username:  username,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.config.ExpireTime)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "prerender-shield",
			Subject:   userID,
			ID:        sessionID, // JTI
		},
	}

	// 创建令牌
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 签名令牌
	tokenString, err := token.SignedString([]byte(m.config.SecretKey))
	if err != nil {
		return "", err
	}

	// 如果Redis客户端可用，保存会话到Redis
	if m.redisClient != nil {
		err := m.redisClient.SaveSession(sessionID, userID, m.config.ExpireTime)
		if err != nil {
			return "", err
		}
	}

	return tokenString, nil
}

// ValidateToken 验证JWT令牌
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	// 解析令牌
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(m.config.SecretKey), nil
	})

	if err != nil {
		// 检查是否是过期错误
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	// 验证令牌是否有效
	if !token.Valid {
		return nil, ErrInvalidToken
	}

	// 获取声明
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidToken
	}

	// 如果Redis客户端可用，检查会话是否存在（实现服务端注销和会话管理）
	if m.redisClient != nil {
		exists, err := m.redisClient.CheckSessionExists(claims.SessionID)
		if err != nil {
			// 如果Redis出错，暂时允许通过（降级策略），或者返回错误
			// 这里选择安全策略：如果无法验证会话，则认为无效
			return nil, fmt.Errorf("failed to verify session: %v", err)
		}
		if !exists {
			return nil, ErrSessionExpired
		}
	}

	return claims, nil
}

// RevokeToken 撤销令牌（注销）
func (m *JWTManager) RevokeToken(tokenString string) error {
	claims, err := m.ValidateToken(tokenString)
	if err != nil {
		return err
	}
	
	if m.redisClient != nil {
		return m.redisClient.DeleteSession(claims.SessionID)
	}
	return nil
}
