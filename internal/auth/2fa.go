package auth

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"prerender-shield/internal/redis"
)

// TwoFactorAuth 双因素认证管理器
type TwoFactorAuth struct {
	redisClient *redis.Client
	totpManager *TOTPManager
}

// NewTwoFactorAuth 创建双因素认证管理器
func NewTwoFactorAuth(redisClient *redis.Client, issuer string) *TwoFactorAuth {
	return &TwoFactorAuth{
		redisClient: redisClient,
		totpManager: NewTOTPManager(issuer),
	}
}

// Enable2FA 启用 2FA
func (t *TwoFactorAuth) Enable2FA(userID string) (secret, qrCodeURL string, err error) {
	// 生成密钥
	secret, err = t.totpManager.GenerateSecret()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate secret: %w", err)
	}

	// 生成二维码 URL
	qrCodeURL, err = t.totpManager.GenerateQRCode(userID, secret)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate QR code: %w", err)
	}

	// 临时存储密钥（等待用户确认）
	key := fmt.Sprintf("2fa:pending:%s", userID)
	if err := t.redisClient.Set(key, secret, 15*time.Minute); err != nil {
		return "", "", fmt.Errorf("failed to store secret: %w", err)
	}

	return secret, qrCodeURL, nil
}

// Confirm2FA 确认启用 2FA
func (t *TwoFactorAuth) Confirm2FA(userID, code string) error {
	// 获取临时密钥
	key := fmt.Sprintf("2fa:pending:%s", userID)
	secret, err := t.redisClient.Get(key)
	if err != nil || secret == "" {
		return fmt.Errorf("2FA setup not initiated or expired")
	}

	// 验证 TOTP 码
	if !t.totpManager.ValidateCode(secret, code) {
		return fmt.Errorf("invalid verification code")
	}

	// 保存 2FA 状态
	userKey := fmt.Sprintf("user:%s", userID)
	if err := t.redisClient.HashSet(userKey, "2fa_enabled", "true"); err != nil {
		return fmt.Errorf("failed to save 2FA status: %w", err)
	}

	// 保存密钥
	secretKey := fmt.Sprintf("2fa:secret:%s", userID)
	if err := t.redisClient.Set(secretKey, secret, 0); err != nil {
		return fmt.Errorf("failed to save secret: %w", err)
	}

	// 删除临时密钥
	t.redisClient.Del(key)

	return nil
}

// Disable2FA 禁用 2FA
func (t *TwoFactorAuth) Disable2FA(userID, code string) error {
	// 验证当前 TOTP 码
	if err := t.Verify2FA(userID, code); err != nil {
		return fmt.Errorf("invalid verification code: %w", err)
	}

	// 删除 2FA 状态
	userKey := fmt.Sprintf("user:%s", userID)
	if err := t.redisClient.HashSet(userKey, "2fa_enabled", "false"); err != nil {
		return fmt.Errorf("failed to disable 2FA: %w", err)
	}

	// 删除密钥
	secretKey := fmt.Sprintf("2fa:secret:%s", userID)
	t.redisClient.Del(secretKey)

	return nil
}

// Verify2FA 验证 2FA 码
func (t *TwoFactorAuth) Verify2FA(userID, code string) error {
	// 获取用户 2FA 状态
	userKey := fmt.Sprintf("user:%s", userID)
	enabled, _ := t.redisClient.HashGet(userKey, "2fa_enabled")
	if enabled != "true" {
		return nil // 未启用 2FA
	}

	// 获取密钥
	secretKey := fmt.Sprintf("2fa:secret:%s", userID)
	secret, err := t.redisClient.Get(secretKey)
	if err != nil || secret == "" {
		return fmt.Errorf("2FA secret not found")
	}

	// 验证 TOTP 码（允许±1 个时间窗口偏差）
	if !t.totpManager.ValidateCodeWithSkew(secret, code, 1) {
		return fmt.Errorf("invalid verification code")
	}

	return nil
}

// Is2FAEnabled 检查用户是否启用 2FA
func (t *TwoFactorAuth) Is2FAEnabled(userID string) (bool, error) {
	userKey := fmt.Sprintf("user:%s", userID)
	enabled, err := t.redisClient.HashGet(userKey, "2fa_enabled")
	if err != nil {
		return false, err
	}
	return enabled == "true", nil
}

// GenerateBackupCodes 生成备用码
func (t *TwoFactorAuth) GenerateBackupCodes(userID string) ([]string, error) {
	// 验证 2FA 状态
	enabled, err := t.Is2FAEnabled(userID)
	if err != nil || !enabled {
		return nil, fmt.Errorf("2FA not enabled")
	}

	// 生成备用码
	codes, err := t.totpManager.GenerateBackupCodes(10)
	if err != nil {
		return nil, fmt.Errorf("failed to generate backup codes: %w", err)
	}

	// 存储备用码（哈希）
	backupKey := fmt.Sprintf("2fa:backup:%s", userID)
	backupData := make(map[string]interface{})
	for i, code := range codes {
		backupData[fmt.Sprintf("code_%d", i)] = code
	}
	if err := t.redisClient.SaveJSON(backupKey, backupData, 0); err != nil {
		return nil, fmt.Errorf("failed to save backup codes: %w", err)
	}

	return codes, nil
}

// VerifyBackupCode 验证备用码
func (t *TwoFactorAuth) VerifyBackupCode(userID, code string) error {
	backupKey := fmt.Sprintf("2fa:backup:%s", userID)
	backupData := make(map[string]interface{})
	if err := t.redisClient.GetJSON(backupKey, &backupData); err != nil {
		return fmt.Errorf("backup codes not found")
	}

	// 查找并删除匹配的备用码
	for k, v := range backupData {
		if vStr, ok := v.(string); ok && subtle.ConstantTimeCompare([]byte(vStr), []byte(code)) == 1 {
			delete(backupData, k)
			if err := t.redisClient.SaveJSON(backupKey, backupData, 0); err != nil {
				return fmt.Errorf("failed to update backup codes: %w", err)
			}
			return nil
		}
	}

	return fmt.Errorf("invalid backup code")
}

// Require2FA 2FA 验证中间件
func (t *TwoFactorAuth) Require2FA() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Unauthorized",
			})
			return
		}

		// 检查是否需要 2FA
		enabled, err := t.Is2FAEnabled(userID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "Failed to check 2FA status",
			})
			return
		}

		if enabled {
			// 检查是否已通过 2FA 验证
			verified := c.GetBool("2fa_verified")
			if !verified {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"code":    403,
					"message": "2FA verification required",
				})
				return
			}
		}

		c.Next()
	}
}

// VerifyTOTPCode 通用 TOTP 码验证函数
func VerifyTOTPCode(secret, code string) bool {
	return VerifyTOTPCustom(secret, code, time.Now().UTC(), ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
}
