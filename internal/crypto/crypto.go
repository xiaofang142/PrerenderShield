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
	"os"
	"sync"
)

var (
	masterKey []byte
	keyMu     sync.RWMutex
)

const (
	envMasterKey = "PRERENDER_MASTER_KEY"
	keyLength    = 32
)

// Encrypt 加密明文
// 使用 AES-256-GCM, 输出格式: base64(nonce + ciphertext)
// 当主密钥未初始化时，返回明文（直通模式，用于无需加密的场景）
func Encrypt(plaintext []byte) (string, error) {
	key := getMasterKey()
	if key == nil {
		return string(plaintext), nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}

	// NewGCM 仅在 BlockSize≠16 时报错；aes.NewCipher 保证 AES blockSize=16，此分支不可达
	gcm, _ := cipher.NewGCM(block)

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密密文
// 当主密钥未初始化时，返回原始字符串（直通模式，用于无需加密的场景）
func Decrypt(encoded string) ([]byte, error) {
	key := getMasterKey()
	if key == nil {
		return []byte(encoded), nil
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}

	// NewGCM 仅在 BlockSize≠16 时报错；aes.NewCipher 保证 AES blockSize=16，此分支不可达
	gcm, _ := cipher.NewGCM(block)

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// InitMasterKey 初始化主密钥
// 使用 SHA-256 哈希将任意长度密钥扩展到 32 字节，确保密钥材料均匀分布
func InitMasterKey(key string) error {
	keyMu.Lock()
	defer keyMu.Unlock()

	if len(key) < 16 {
		return errors.New("master key must be at least 16 characters")
	}

	// 使用 SHA-256 哈希密钥材料，生成固定 32 字节密钥
	hash := sha256.Sum256([]byte(key))
	masterKey = hash[:]
	return nil
}

// InitMasterKeyFromEnv 从环境变量初始化主密钥
func InitMasterKeyFromEnv() error {
	key := os.Getenv(envMasterKey)
	if key == "" {
		return nil
	}
	return InitMasterKey(key)
}

func getMasterKey() []byte {
	keyMu.RLock()
	defer keyMu.RUnlock()
	return masterKey
}

// EncryptConfigField 加密配置字段（标记 !encrypted 格式）
func EncryptConfigField(value string) (string, error) {
	if value == "" || len(value) < 4 {
		return value, nil
	}
	encrypted, err := Encrypt([]byte(value))
	if err != nil {
		return "", err
	}
	return "!encrypted:" + encrypted, nil
}

// DecryptConfigField 解密配置字段
func DecryptConfigField(value string) (string, error) {
	if len(value) < 12 || value[:11] != "!encrypted:" {
		return value, nil
	}
	decrypted, err := Decrypt(value[11:])
	if err != nil {
		return "", err
	}
	return string(decrypted), nil
}
