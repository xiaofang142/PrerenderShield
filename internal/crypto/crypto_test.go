package crypto

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncryptDecrypt(t *testing.T) {
	err := InitMasterKey("this-is-32-char-test-key-for-aes!")
	assert.NoError(t, err)

	plaintext := []byte("sensitive-data-123")
	encrypted, err := Encrypt(plaintext)
	assert.NoError(t, err)
	assert.NotEqual(t, string(plaintext), encrypted)

	decrypted, err := Decrypt(encrypted)
	assert.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncryptNoKey(t *testing.T) {
	masterKey = nil
	plaintext := []byte("test")
	result, err := Encrypt(plaintext)
	assert.NoError(t, err)
	assert.Equal(t, string(plaintext), result)
}

func TestDecryptNoKey(t *testing.T) {
	masterKey = nil
	result, err := Decrypt("dGVzdA==")
	assert.NoError(t, err)
	assert.Equal(t, "dGVzdA==", string(result))
}

func TestInitMasterKeyTooShort(t *testing.T) {
	err := InitMasterKey("short")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least 16")
}

func TestEncryptConfigField(t *testing.T) {
	_ = InitMasterKey("this-is-32-char-test-key-for-aes!")
	result, err := EncryptConfigField("my-password")
	assert.NoError(t, err)
	assert.Contains(t, result, "!encrypted:")
}

func TestEncryptConfigFieldEmpty(t *testing.T) {
	result, err := EncryptConfigField("")
	assert.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestDecryptConfigField(t *testing.T) {
	_ = InitMasterKey("this-is-32-char-test-key-for-aes!")
	encrypted, _ := EncryptConfigField("my-password")
	decrypted, err := DecryptConfigField(encrypted)
	assert.NoError(t, err)
	assert.Equal(t, "my-password", decrypted)
}

func TestDecryptConfigFieldPlain(t *testing.T) {
	result, err := DecryptConfigField("plain-text-without-prefix")
	assert.NoError(t, err)
	assert.Equal(t, "plain-text-without-prefix", result)
}

func TestInitMasterKeyFromEnv(t *testing.T) {
	os.Setenv("PRERENDER_MASTER_KEY", "env-based-key-32-chars-long!!!")
	defer os.Unsetenv("PRERENDER_MASTER_KEY")
	err := InitMasterKeyFromEnv()
	assert.NoError(t, err)
}

func TestInitMasterKeyFromEnvEmpty(t *testing.T) {
	os.Unsetenv("PRERENDER_MASTER_KEY")
	err := InitMasterKeyFromEnv()
	assert.NoError(t, err)
}

func TestDecryptInvalidBase64(t *testing.T) {
	masterKey = []byte("this-is-32-char-test-key-for-aes!")
	_, err := Decrypt("!!!invalid-base64!!!")
	assert.Error(t, err)
}

func TestDecryptInvalidCiphertext(t *testing.T) {
	masterKey = []byte("this-is-32-char-test-key-for-aes!")
	_, err := Decrypt("YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXo=")
	assert.Error(t, err)
}
