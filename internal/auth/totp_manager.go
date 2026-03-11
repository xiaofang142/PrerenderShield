package auth

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TOTPManager TOTP 管理器
type TOTPManager struct {
	issuer    string
	period    uint
	digits    otp.Digits
	algorithm otp.Algorithm
}

// NewTOTPManager 创建 TOTP 管理器
func NewTOTPManager(issuer string) *TOTPManager {
	return &TOTPManager{
		issuer:    issuer,
		period:    30,
		digits:    otp.DigitsSix,
		algorithm: otp.AlgorithmSHA1,
	}
}

// GenerateSecret 生成新的密钥
func (m *TOTPManager) GenerateSecret() (string, error) {
	key := make([]byte, 20)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("failed to generate random key: %w", err)
	}
	return base32.StdEncoding.EncodeToString(key), nil
}

// GenerateQRCode 生成二维码 URL
func (m *TOTPManager) GenerateQRCode(username, secret string) (string, error) {
	// 解码 Base32 密钥
	secretBytes, err := base32.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", fmt.Errorf("failed to decode secret: %w", err)
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      m.issuer,
		AccountName: username,
		Secret:      secretBytes,
		Period:      m.period,
		Digits:      m.digits,
		Algorithm:   m.algorithm,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate TOTP key: %w", err)
	}
	return key.URL(), nil
}

// ValidateCode 验证 TOTP 码
func (m *TOTPManager) ValidateCode(secret, code string) bool {
	return totp.Validate(code, secret)
}

// ValidateCodeWithSkew 验证 TOTP 码（允许时间偏差）
func (m *TOTPManager) ValidateCodeWithSkew(secret, code string, skew uint) bool {
	valid, _ := totp.ValidateCustom(code, secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    m.period,
		Skew:      skew,
		Digits:    m.digits,
		Algorithm: m.algorithm,
	})
	return valid
}

// GenerateBackupCodes 生成备用码
func (m *TOTPManager) GenerateBackupCodes(count int) ([]string, error) {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		code := make([]byte, 8)
		if _, err := rand.Read(code); err != nil {
			return nil, err
		}
		codes[i] = fmt.Sprintf("%04d-%04d", int(code[0])%10000, int(code[4])%10000)
	}
	return codes, nil
}

// ValidateOpts TOTP 验证选项
type ValidateOpts struct {
	Period    uint
	Skew      uint
	Digits    otp.Digits
	Algorithm otp.Algorithm
}

// VerifyTOTPCustom 自定义 TOTP 验证
func VerifyTOTPCustom(secret, code string, now time.Time, opts ValidateOpts) bool {
	valid, _ := totp.ValidateCustom(code, secret, now, totp.ValidateOpts{
		Period:    opts.Period,
		Skew:      opts.Skew,
		Digits:    opts.Digits,
		Algorithm: opts.Algorithm,
	})
	return valid
}
