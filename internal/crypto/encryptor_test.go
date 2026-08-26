package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEncryptor(t *testing.T) {
	// 测试短密钥
	encryptor, err := NewEncryptor("short")
	assert.NoError(t, err)
	assert.NotNil(t, encryptor)
	assert.Len(t, encryptor.key, 32)

	// 测试正常密钥
	encryptor, err = NewEncryptor("this-is-a-16-char-key!")
	assert.NoError(t, err)
	assert.NotNil(t, encryptor)

	// 测试长密钥
	encryptor, err = NewEncryptor("this-is-a-very-long-secret-key-that-is-more-than-32-chars")
	assert.NoError(t, err)
	assert.NotNil(t, encryptor)
	// 密钥会被截断到 32 字节
	assert.GreaterOrEqual(t, len(encryptor.key), 16)
}

func TestEncryptor_EncryptAndDecrypt(t *testing.T) {
	encryptor, err := NewEncryptor("test-secret-key-1234")
	assert.NoError(t, err)

	plaintext := "Hello, World!"

	// 加密
	encrypted, err := encryptor.Encrypt(plaintext)
	assert.NoError(t, err)
	assert.NotEmpty(t, encrypted)
	assert.NotEqual(t, plaintext, encrypted)

	// 解密
	decrypted, err := encryptor.Decrypt(encrypted)
	assert.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncryptor_DecryptInvalid(t *testing.T) {
	encryptor, err := NewEncryptor("test-secret-key-1234")
	assert.NoError(t, err)

	// 测试无效的 Base64
	_, err = encryptor.Decrypt("invalid-base64!")
	assert.Error(t, err)

	// 测试无效的密文
	_, err = encryptor.Decrypt("YQ==") // "a" 的 Base64
	assert.Error(t, err)
}

func TestEncryptor_EmptyValue(t *testing.T) {
	encryptor, err := NewEncryptor("test-secret-key-1234")
	assert.NoError(t, err)

	// 测试空字符串加密
	encrypted, err := encryptor.Encrypt("")
	assert.NoError(t, err)
	assert.NotEmpty(t, encrypted)

	decrypted, err := encryptor.Decrypt(encrypted)
	assert.NoError(t, err)
	assert.Empty(t, decrypted)
}

func TestEncryptor_EncryptField(t *testing.T) {
	encryptor, err := NewEncryptor("test-secret-key-1234")
	assert.NoError(t, err)

	// 测试加密字段
	encrypted, err := encryptor.EncryptField("password", "secret123")
	assert.NoError(t, err)
	assert.NotEmpty(t, encrypted)

	// 测试解密字段
	decrypted, err := encryptor.DecryptField("password", encrypted)
	assert.NoError(t, err)
	assert.Equal(t, "secret123", decrypted)
}

func TestSensitiveConfig(t *testing.T) {
	config, err := NewSensitiveConfig("test-secret-key-1234")
	assert.NoError(t, err)

	// 测试设置加密值
	err = config.Set("api_key", "my-secret-api-key")
	assert.NoError(t, err)

	// 测试获取解密值
	value, err := config.Get("api_key")
	assert.NoError(t, err)
	assert.Equal(t, "my-secret-api-key", value)

	// 测试获取不存在的键
	_, err = config.Get("nonexistent")
	assert.Error(t, err)

	// 测试设置原始值
	config.SetRaw("public_key", "public-value")
	assert.Equal(t, "public-value", config.GetRaw("public_key"))

	// 测试导出导入
	exported := config.Export()
	assert.NotEmpty(t, exported)

	newConfig, _ := NewSensitiveConfig("test-secret-key-1234")
	newConfig.Import(exported)
	value, _ = newConfig.Get("api_key")
	assert.Equal(t, "my-secret-api-key", value)
}

func TestEncryptor_DifferentKeys(t *testing.T) {
	encryptor1, _ := NewEncryptor("key-1-secret")
	encryptor2, _ := NewEncryptor("key-2-secret")

	plaintext := "sensitive-data"

	// 用 key1 加密
	encrypted1, _ := encryptor1.Encrypt(plaintext)
	encrypted2, _ := encryptor2.Encrypt(plaintext)

	// 确保不同密钥加密结果不同
	assert.NotEqual(t, encrypted1, encrypted2)

	// 用 key1 解密 key2 的密文应该失败
	_, err := encryptor1.Decrypt(encrypted2)
	assert.Error(t, err)
}
