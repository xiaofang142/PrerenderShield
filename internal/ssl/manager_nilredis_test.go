package ssl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewManager_NilRedis 测试创建 Manager 时 Redis 为 nil 的情况
func TestNewManager_NilRedis(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(nil, tempDir, "test@example.com", false)
	assert.NoError(t, err)
	assert.NotNil(t, manager)
}

// TestManager_RequestCertificate_NilRedis 测试 Redis 为 nil 时 RequestCertificate 的行为
func TestManager_RequestCertificate_NilRedis(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(nil, tempDir, "test@example.com", false)
	assert.NoError(t, err)

	err = manager.RequestCertificate("test.example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis client is nil")
}

// TestManager_RenewCertificate_NilRedis 测试 Redis 为 nil 时 RenewCertificate 的行为
func TestManager_RenewCertificate_NilRedis(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(nil, tempDir, "test@example.com", false)
	assert.NoError(t, err)

	err = manager.RenewCertificate("test.example.com")
	assert.Error(t, err)
}

// TestManager_ImportCertificate_NilRedis 测试 Redis 为 nil 时 ImportCertificate 的行为
func TestManager_ImportCertificate_NilRedis(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(nil, tempDir, "test@example.com", false)
	assert.NoError(t, err)

	err = manager.ImportCertificate("test.example.com", "/path/to/cert.crt", "/path/to/key.key")
	assert.Error(t, err)
}

// TestManager_GetCertificate_NilRedis 测试 Redis 为 nil 时 GetCertificate 的行为
func TestManager_GetCertificate_NilRedis(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(nil, tempDir, "test@example.com", false)
	assert.NoError(t, err)

	_, err = manager.GetCertificate("test.example.com")
	assert.Error(t, err)
}

// TestManager_GetCertificateStatus_NilRedis 测试 Redis 为 nil 时 GetCertificateStatus 的行为
func TestManager_GetCertificateStatus_NilRedis(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(nil, tempDir, "test@example.com", false)
	assert.NoError(t, err)

	_, err = manager.GetCertificateStatus("test.example.com")
	assert.Error(t, err)
}

// TestManager_ListCertificates_NilRedis 测试 Redis 为 nil 时 ListCertificates 的行为
func TestManager_ListCertificates_NilRedis(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(nil, tempDir, "test@example.com", false)
	assert.NoError(t, err)

	_, err = manager.ListCertificates()
	assert.Error(t, err)
}

// TestManager_DeleteCertificate_NilRedis 测试 Redis 为 nil 时 DeleteCertificate 的行为
func TestManager_DeleteCertificate_NilRedis(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(nil, tempDir, "test@example.com", false)
	assert.NoError(t, err)

	err = manager.DeleteCertificate("test.example.com")
	assert.Error(t, err)
}

// TestManager_CheckExpiration_NilRedis 测试 Redis 为 nil 时 CheckExpiration 的行为
func TestManager_CheckExpiration_NilRedis(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(nil, tempDir, "test@example.com", false)
	assert.NoError(t, err)

	_, err = manager.CheckExpiration()
	assert.Error(t, err)
}

// TestGeneratePrivateKey 测试生成私钥
func TestGeneratePrivateKey(t *testing.T) {
	key, err := generatePrivateKey()
	assert.NoError(t, err)
	assert.NotNil(t, key)
	assert.Equal(t, 2048, key.N.BitLen())
}

// TestGenerateCSR 测试生成 CSR
func TestGenerateCSR(t *testing.T) {
	key, err := generatePrivateKey()
	assert.NoError(t, err)

	csr, err := generateCSR(key, []string{"test.example.com"})
	assert.NoError(t, err)
	assert.NotNil(t, csr)
	assert.Equal(t, "test.example.com", csr.Subject.CommonName)
}

// TestSaveAndLoadPrivateKey 测试保存和加载私钥
func TestSaveAndLoadPrivateKey(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := tempDir + "/test.key"

	key, err := generatePrivateKey()
	assert.NoError(t, err)

	// 保存私钥
	err = savePrivateKey(keyPath, key)
	assert.NoError(t, err)

	// 验证文件存在
	assert.FileExists(t, keyPath)
}

// TestSaveAndLoadCertificate 测试保存和加载证书
func TestSaveAndLoadCertificate(t *testing.T) {
	tempDir := t.TempDir()
	certPath := tempDir + "/test.crt"

	key, err := generatePrivateKey()
	assert.NoError(t, err)

	// 创建自签名证书
	mgr, err := NewManager(nil, tempDir, "test@example.com", false)
	assert.NoError(t, err)
	manager := mgr.(*manager)

	certPEM, err := manager.createSelfSignedCertificate("test.example.com", key)
	assert.NoError(t, err)

	// 保存证书
	err = saveCertificate(certPath, certPEM)
	assert.NoError(t, err)

	// 验证文件存在
	assert.FileExists(t, certPath)
}

// TestCreateSelfSignedCertificate 测试创建自签名证书
func TestCreateSelfSignedCertificate(t *testing.T) {
	tempDir := t.TempDir()
	mgr, err := NewManager(nil, tempDir, "test@example.com", false)
	assert.NoError(t, err)
	manager := mgr.(*manager)

	key, err := generatePrivateKey()
	assert.NoError(t, err)

	certPEM, err := manager.createSelfSignedCertificate("test.example.com", key)
	assert.NoError(t, err)
	assert.NotEmpty(t, certPEM)
	assert.Contains(t, string(certPEM), "BEGIN CERTIFICATE")
}

// TestManager_Interface 测试 Manager 接口实现
func TestManager_Interface(t *testing.T) {
	var _ Manager = (*manager)(nil)
}
