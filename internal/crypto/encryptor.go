package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// Encryptor 加密器
type Encryptor struct {
	key []byte
}

// NewEncryptor 创建加密器
func NewEncryptor(secretKey string) (*Encryptor, error) {
	// 密钥必须是 16、24 或 32 字节（对应 AES-128、AES-192、AES-256）
	key := []byte(secretKey)
	if len(key) < 16 {
		// 如果密钥太短，使用 SHA256 哈希扩展
		key = hashKey(secretKey)
	}
	if len(key) > 32 {
		key = key[:32]
	}

	return &Encryptor{
		key: padKey(key),
	}, nil
}

// Encrypt 加密数据
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	// 创建 AES 密码块
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher block: %w", err)
	}

	// 将明文转换为字节
	plaintextBytes := []byte(plaintext)

	// GCM 模式需要追加 nonce
	// NewGCM 仅在 BlockSize≠16 时报错；aes.NewCipher 保证 AES blockSize=16，此分支不可达
	aesGCM, _ := cipher.NewGCM(block)

	// 生成随机 nonce
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// 加密数据
	ciphertext := aesGCM.Seal(nonce, nonce, plaintextBytes, nil)

	// 使用 Base64 编码
	encoded := base64.StdEncoding.EncodeToString(ciphertext)

	return encoded, nil
}

// Decrypt 解密数据
func (e *Encryptor) Decrypt(ciphertext string) (string, error) {
	// Base64 解码
	decoded, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// 创建 AES 密码块
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher block: %w", err)
	}

	// GCM 模式
	// NewGCM 仅在 BlockSize≠16 时报错；aes.NewCipher 保证 AES blockSize=16，此分支不可达
	aesGCM, _ := cipher.NewGCM(block)

	// 检查数据长度
	nonceSize := aesGCM.NonceSize()
	if len(decoded) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	// 提取 nonce 和密文
	nonce, ciphertextBytes := decoded[:nonceSize], decoded[nonceSize:]

	// 解密数据
	plaintextBytes, err := aesGCM.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintextBytes), nil
}

// hashKey 使用 SHA-256 将密钥扩展到 32 字节
func hashKey(key string) []byte {
	hash := sha256.Sum256([]byte(key))
	return hash[:]
}

// padKey 填充密钥到有效长度（使用最大可用长度）
func padKey(key []byte) []byte {
	validLengths := []int{32, 24, 16}

	for _, validLen := range validLengths {
		if len(key) >= validLen {
			return key[:validLen]
		}
	}

	// 密钥太短时使用 SHA-256 哈希扩展
	hashed := hashKey(string(key))
	return hashed[:16]
}

// EncryptField 加密敏感字段
func (e *Encryptor) EncryptField(fieldName, value string) (string, error) {
	if value == "" {
		return "", nil
	}

	encrypted, err := e.Encrypt(value)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt %s: %w", fieldName, err)
	}

	return encrypted, nil
}

// DecryptField 解密敏感字段
func (e *Encryptor) DecryptField(fieldName, value string) (string, error) {
	if value == "" {
		return "", nil
	}

	decrypted, err := e.Decrypt(value)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt %s: %w", fieldName, err)
	}

	return decrypted, nil
}

// SensitiveConfig 敏感配置结构
type SensitiveConfig struct {
	encryptor *Encryptor
	data      map[string]string
}

// NewSensitiveConfig 创建敏感配置
func NewSensitiveConfig(secretKey string) (*SensitiveConfig, error) {
	// NewEncryptor 当前实现永不返回错误（密钥自动规范化），分支不可达；
	// 保留 error 签名以兼容调用方
	encryptor, _ := NewEncryptor(secretKey)
	return &SensitiveConfig{
		encryptor: encryptor,
		data:      make(map[string]string),
	}, nil
}

// Set 设置加密值
func (c *SensitiveConfig) Set(key, value string) error {
	encrypted, err := c.encryptor.Encrypt(value)
	if err != nil {
		return err
	}

	c.data[key] = encrypted
	return nil
}

// Get 获取解密值
func (c *SensitiveConfig) Get(key string) (string, error) {
	encrypted, exists := c.data[key]
	if !exists {
		return "", fmt.Errorf("key not found: %s", key)
	}

	return c.encryptor.Decrypt(encrypted)
}

// SetRaw 设置原始值（不加密）
func (c *SensitiveConfig) SetRaw(key, value string) {
	c.data[key] = value
}

// GetRaw 获取原始值
func (c *SensitiveConfig) GetRaw(key string) string {
	return c.data[key]
}

// Export 导出加密数据
func (c *SensitiveConfig) Export() map[string]string {
	exported := make(map[string]string)
	for k, v := range c.data {
		exported[k] = v
	}
	return exported
}

// Import 导入加密数据
func (c *SensitiveConfig) Import(data map[string]string) {
	for k, v := range data {
		c.data[k] = v
	}
}
