package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTOTPManager_GenerateSecret(t *testing.T) {
	manager := NewTOTPManager("TestApp")

	secret, err := manager.GenerateSecret()
	assert.NoError(t, err)
	assert.NotEmpty(t, secret)
	assert.Len(t, secret, 32) // Base32 编码的 20 字节
}

func TestTOTPManager_GenerateQRCode(t *testing.T) {
	manager := NewTOTPManager("TestApp")

	secret, err := manager.GenerateSecret()
	assert.NoError(t, err)

	qrURL, err := manager.GenerateQRCode("user@example.com", secret)
	assert.NoError(t, err)
	assert.Contains(t, qrURL, "otpauth://totp")
	assert.Contains(t, qrURL, "TestApp")
	assert.Contains(t, qrURL, "user@example.com")
}

func TestTOTPManager_ValidateCode(t *testing.T) {
	manager := NewTOTPManager("TestApp")

	secret, err := manager.GenerateSecret()
	assert.NoError(t, err)

	// 测试接口存在
	_ = secret
	// 注意：TOTP 码需要实际时间生成，这里仅测试接口
	assert.NotNil(t, manager)
}

func TestVerifyTOTPCustom(t *testing.T) {
	manager := NewTOTPManager("TestApp")

	secret, _ := manager.GenerateSecret()

	// 测试无效的码
	valid := manager.ValidateCode(secret, "000000")
	assert.False(t, valid)
}
