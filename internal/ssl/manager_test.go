package ssl

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/redis"
)

func TestSSLManager(t *testing.T) {
	// 创建临时目录用于测试
	tempDir, err := ioutil.TempDir("", "ssl-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 创建Redis客户端（使用内存模式或测试Redis）
	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	// 创建SSL管理器
	manager, err := NewManager(redisClient, tempDir, "test@example.com", false)
	assert.NoError(t, err)

	// 测试域名
	testDomain := "test.example.com"

	// 测试1: 申请证书
	t.Run("RequestCertificate", func(t *testing.T) {
		err := manager.RequestCertificate(testDomain)
		assert.NoError(t, err)

		// 检查证书文件是否创建
		certPath := filepath.Join(tempDir, testDomain+".crt")
		keyPath := filepath.Join(tempDir, testDomain+".key")

		assert.FileExists(t, certPath)
		assert.FileExists(t, keyPath)

		// 检查证书信息是否存储到Redis
		certInfo := make(map[string]interface{})
		err = redisClient.GetJSON("ssl:cert:"+testDomain, &certInfo)
		assert.NoError(t, err)
		assert.Equal(t, testDomain, certInfo["domain"])
		assert.Equal(t, certPath, certInfo["cert_path"])
		assert.Equal(t, keyPath, certInfo["key_path"])
		assert.Equal(t, "active", certInfo["status"])
	})

	// 测试2: 获取证书
	t.Run("GetCertificate", func(t *testing.T) {
		cert, err := manager.GetCertificate(testDomain)
		assert.NoError(t, err)
		assert.NotNil(t, cert)
	})

	// 测试3: 获取证书状态
	t.Run("GetCertificateStatus", func(t *testing.T) {
		status, err := manager.GetCertificateStatus(testDomain)
		assert.NoError(t, err)
		assert.NotEmpty(t, status)
		assert.Equal(t, testDomain, status["domain"])
		assert.Equal(t, "active", status["status"])
		assert.NotZero(t, status["remaining_days"])
		assert.False(t, status["expired"].(bool))
	})

	// 测试4: 列出所有证书
	t.Run("ListCertificates", func(t *testing.T) {
		certs, err := manager.ListCertificates()
		assert.NoError(t, err)
		assert.NotEmpty(t, certs)
		assert.Contains(t, certs, testDomain)
	})

	// 测试5: 删除证书
	t.Run("DeleteCertificate", func(t *testing.T) {
		err := manager.DeleteCertificate(testDomain)
		assert.NoError(t, err)

		// 检查证书文件是否删除
		certPath := filepath.Join(tempDir, testDomain+".crt")
		keyPath := filepath.Join(tempDir, testDomain+".key")

		assert.NoFileExists(t, certPath)
		assert.NoFileExists(t, keyPath)

		// 检查证书信息是否从Redis删除
		certInfo := make(map[string]interface{})
		err = redisClient.GetJSON("ssl:cert:"+testDomain, &certInfo)
		assert.NoError(t, err)
		assert.Empty(t, certInfo)
	})

	// 测试6: 检查过期证书
	t.Run("CheckExpiration", func(t *testing.T) {
		expiring, err := manager.CheckExpiration()
		assert.NoError(t, err)
		// 由于我们刚刚删除了测试证书，这里应该返回空列表
		assert.Empty(t, expiring)
	})
}
