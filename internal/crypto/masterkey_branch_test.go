package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"
)

// withMasterKey 临时设置主密钥（测试隔离，结束后清理）
func withMasterKey(t *testing.T, key string) {
	t.Helper()
	if err := InitMasterKey(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { keyMu.Lock(); masterKey = nil; keyMu.Unlock() })
}

// Encrypt/Decrypt 全分支：无密钥直通 / 有密钥加解密往返 / 坏 base64 / 坏密文
func TestEncryptDecrypt_MasterKeyBranches(t *testing.T) {
	// 无密钥：直通
	keyMu.Lock()
	masterKey = nil
	keyMu.Unlock()
	plain := []byte("secret-value")
	enc, err := Encrypt(plain)
	if err != nil || string(enc) != "secret-value" {
		t.Fatalf("passthrough broken: %q %v", enc, err)
	}
	dec, err := Decrypt("secret-value")
	if err != nil || string(dec) != "secret-value" {
		t.Fatalf("decrypt passthrough broken: %q %v", dec, err)
	}

	// 有密钥：往返
	withMasterKey(t, "0123456789abcdef0123456789abcdef")
	enc, err = Encrypt(plain)
	if err != nil || string(enc) == "secret-value" {
		t.Fatalf("encrypt broken: %q %v", enc, err)
	}
	dec, err = Decrypt(enc)
	if err != nil || string(dec) != "secret-value" {
		t.Fatalf("roundtrip broken: %q %v", dec, err)
	}

	// 坏 base64
	if _, err := Decrypt("!!!not-base64!!!"); err == nil {
		t.Fatal("invalid base64 must error")
	}
	// 合法 base64 但非 GCM 密文（密文过短）
	if _, err := Decrypt("YWJj"); err == nil {
		t.Fatal("short ciphertext must error")
	}
}

// EncryptConfigField/DecryptConfigField：短值直通 + 标记格式往返
func TestConfigFieldBranches(t *testing.T) {
	// 无密钥：直通
	keyMu.Lock()
	masterKey = nil
	keyMu.Unlock()
	if got, _ := EncryptConfigField("ab"); got != "ab" {
		t.Fatalf("short value passthrough broken: %q", got)
	}

	withMasterKey(t, "0123456789abcdef0123456789abcdef")
	enc, err := EncryptConfigField("my-secret-value")
	if err != nil || !strings.HasPrefix(enc, "!encrypted:") {
		t.Fatalf("config field encrypt broken: %q %v", enc, err)
	}
	dec, err := DecryptConfigField(enc)
	if err != nil || dec != "my-secret-value" {
		t.Fatalf("config field decrypt broken: %q %v", dec, err)
	}
	// 无标记值直通
	if got, _ := DecryptConfigField("plain-value"); got != "plain-value" {
		t.Fatalf("unmarked value passthrough broken: %q", got)
	}
	// 坏密文带标记 → 错误
	if _, err := DecryptConfigField("!encrypted:!!!"); err == nil {
		t.Fatal("marked garbage must error")
	}
}

// padKey 全分支：32/24/16 截取与过短哈希扩展
func TestPadKey_Branches(t *testing.T) {
	if got := padKey(make([]byte, 40)); len(got) != 32 {
		t.Fatalf(">=32 branch: %d", len(got))
	}
	if got := padKey(make([]byte, 30)); len(got) != 24 {
		t.Fatalf("24 branch: %d", len(got))
	}
	if got := padKey(make([]byte, 20)); len(got) != 16 {
		t.Fatalf("16 branch: %d", len(got))
	}
	if got := padKey([]byte("short")); len(got) != 16 {
		t.Fatalf("short-key hash expansion: %d", len(got))
	}
}

// InitMasterKey 参数校验
func TestInitMasterKey_Validation(t *testing.T) {
	if err := InitMasterKey("short"); err == nil {
		t.Fatal("short key must be rejected")
	}
	if err := InitMasterKey("0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("valid key rejected: %v", err)
	}
	// 环境变量路径
	t.Setenv("PRERENDER_MASTER_KEY", "")
	if err := InitMasterKeyFromEnv(); err != nil {
		t.Fatal("empty env must be no-op success")
	}
	t.Setenv("PRERENDER_MASTER_KEY", "0123456789abcdef0123456789abcdef")
	if err := InitMasterKeyFromEnv(); err != nil {
		t.Fatalf("env init broken: %v", err)
	}
	keyMu.Lock()
	masterKey = nil
	keyMu.Unlock()
	_ = os.Unsetenv(envMasterKey)
	_ = envMasterKey
}

// 防御分支覆盖：非法长度主密钥注入 → NewCipher/GCM/nonce 错误路径
func TestEncrypt_DefensiveBranches(t *testing.T) {
	withMasterKey(t, "0123456789abcdef0123456789abcdef")
	keyMu.Lock()
	masterKey = []byte("bad") // 非法长度：aes.NewCipher 必失败
	keyMu.Unlock()
	if _, err := Encrypt([]byte("x")); err == nil {
		t.Fatal("invalid key must error on Encrypt")
	}
	if _, err := Decrypt("YWJj"); err == nil {
		t.Fatal("invalid key must error on Decrypt")
	}
	// EncryptConfigField 错误传播（长值进入加密路径）
	if _, err := EncryptConfigField("long-enough-value"); err == nil {
		t.Fatal("EncryptConfigField must propagate encrypt error under invalid key")
	}
	// 恢复
	keyMu.Lock()
	masterKey = nil
	keyMu.Unlock()
}

// Encryptor 防御分支：非法长度 key 注入 → cipher 错误路径
func TestEncryptor_DefensiveBranches(t *testing.T) {
	e := &Encryptor{key: []byte("bad")} // 非法长度
	if _, err := e.Encrypt("x"); err == nil {
		t.Fatal("invalid key must error on Encrypt")
	}
	if _, err := e.Decrypt("YWJj"); err == nil {
		t.Fatal("invalid key must error on Decrypt")
	}
	if _, err := e.EncryptField("f", "x"); err == nil {
		t.Fatal("EncryptField must propagate cipher error")
	}
	if _, err := e.DecryptField("f", "YWJj"); err == nil {
		t.Fatal("DecryptField must propagate cipher error")
	}
}

// nonce 生成失败分支：rand.Reader 故障注入（Restore 交由 t.Cleanup）
type failingRand struct{}

func (failingRand) Read(p []byte) (int, error) { return 0, errors.New("rand failure (simulated)") }

func TestEncrypt_NonceFailure(t *testing.T) {
	withMasterKey(t, "0123456789abcdef0123456789abcdef")
	orig := rand.Reader
	rand.Reader = failingRand{}
	t.Cleanup(func() { rand.Reader = orig })

	if _, err := Encrypt([]byte("x")); err == nil {
		t.Fatal("nonce failure must error on Encrypt")
	}
}

// SensitiveConfig.Set 加密错误传播（非法 key 注入）
func TestSensitiveConfig_SetError(t *testing.T) {
	c := &SensitiveConfig{encryptor: &Encryptor{key: []byte("bad")}, data: map[string]string{}}
	if err := c.Set("k", "v"); err == nil {
		t.Fatal("Set must propagate encrypt error")
	}
}

func TestDecrypt_NewCipherDefensive(t *testing.T) {
	keyMu.Lock()
	masterKey = []byte("bad-len")
	keyMu.Unlock()
	defer func() { keyMu.Lock(); masterKey = nil; keyMu.Unlock() }()
	// 合法 base64 → aes.NewCipher(3 字节) 报错
	if _, err := Decrypt("YWJj"); err == nil {
		t.Fatal("NewCipher defensive branch must error")
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	withMasterKey(t, "0123456789abcdef0123456789abcdef")
	enc, err := Encrypt([]byte("tamper-me-please"))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.StdEncoding.DecodeString(enc)
	raw[len(raw)-1] ^= 0xFF // 篡改认证标签
	tampered := base64.StdEncoding.EncodeToString(raw)
	if _, err := Decrypt(tampered); err == nil {
		t.Fatal("tampered ciphertext must fail GCM auth")
	}
}
