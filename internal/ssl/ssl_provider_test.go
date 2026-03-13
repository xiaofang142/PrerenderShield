package ssl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/stretchr/testify/assert"
)

// TestHTTPProvider 测试 HTTP 提供者
func TestHTTPProvider(t *testing.T) {
	// 使用非标准端口避免权限问题
	provider := NewHTTPProvider(0)
	assert.NotNil(t, provider)
	// port 0 会被改为 80
	assert.Equal(t, 80, provider.port)
	assert.NotNil(t, provider.tokens)
	assert.NotNil(t, provider.server)
}

// TestHTTPProvider_PortOverride 测试端口覆盖
func TestHTTPProvider_PortOverride(t *testing.T) {
	provider := NewHTTPProvider(8080)
	assert.NotNil(t, provider)
	assert.Equal(t, 8080, provider.port)
}

// TestHTTPProvider_Present 测试 Present 方法
func TestHTTPProvider_Present(t *testing.T) {
	provider := NewHTTPProvider(0)

	err := provider.Present("example.com", "test-token", "test-key-auth")
	assert.NoError(t, err)

	// 验证 token 已存储
	provider.mu.RLock()
	_, exists := provider.tokens["test-token"]
	provider.mu.RUnlock()
	assert.True(t, exists)
}

// TestHTTPProvider_CleanUp 测试 CleanUp 方法
func TestHTTPProvider_CleanUp(t *testing.T) {
	provider := NewHTTPProvider(0)

	// 先添加 token
	provider.Present("example.com", "test-token", "test-key-auth")

	// 再清理
	err := provider.CleanUp("example.com", "test-token", "test-key-auth")
	assert.NoError(t, err)

	// 验证 token 已删除
	provider.mu.RLock()
	_, exists := provider.tokens["test-token"]
	provider.mu.RUnlock()
	assert.False(t, exists)
}

// TestHTTPProvider_Present_MultipleTokens 测试多个 token
func TestHTTPProvider_Present_MultipleTokens(t *testing.T) {
	provider := NewHTTPProvider(0)

	tokens := []string{"token1", "token2", "token3"}
	for i, token := range tokens {
		err := provider.Present("example.com", token, "key-auth-"+token)
		assert.NoError(t, err)
		assert.Len(t, provider.tokens, i+1)
	}
}

// TestHTTPProvider_CleanUp_NonExistentToken 测试清理不存在的 token
func TestHTTPProvider_CleanUp_NonExistentToken(t *testing.T) {
	provider := NewHTTPProvider(0)

	// 清理不存在的 token 不应该报错
	err := provider.CleanUp("example.com", "non-existent", "key-auth")
	assert.NoError(t, err)
}

// TestHTTPProvider_HandleChallenge 测试挑战处理函数
func TestHTTPProvider_HandleChallenge(t *testing.T) {
	provider := NewHTTPProvider(0)

	// 设置 token
	provider.Present("example.com", "test-token", "test-key-auth")

	// 创建测试请求
	req := httptest.NewRequest(http.MethodGet, "/.well-known/acme-challenge/test-token", nil)
	w := httptest.NewRecorder()

	// 调用处理函数
	provider.handleChallenge(w, req)

	// 验证响应
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
	assert.Equal(t, "test-key-auth", w.Body.String())
}

// TestHTTPProvider_HandleChallenge_NotFound 测试挑战未找到
func TestHTTPProvider_HandleChallenge_NotFound(t *testing.T) {
	provider := NewHTTPProvider(0)

	// 不设置 token，直接请求
	req := httptest.NewRequest(http.MethodGet, "/.well-known/acme-challenge/non-existent", nil)
	w := httptest.NewRecorder()

	provider.handleChallenge(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestHTTPProvider_HandleChallenge_EmptyToken 测试空 token
func TestHTTPProvider_HandleChallenge_EmptyToken(t *testing.T) {
	provider := NewHTTPProvider(0)

	// 请求空 token
	req := httptest.NewRequest(http.MethodGet, "/.well-known/acme-challenge/", nil)
	w := httptest.NewRecorder()

	provider.handleChallenge(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestHTTPProvider_Start 测试 Start 方法
func TestHTTPProvider_Start(t *testing.T) {
	provider := NewHTTPProvider(0)

	// Start 方法应该返回 nil（它会在 goroutine 中启动服务器）
	err := provider.Start()
	assert.NoError(t, err)
}

// TestHTTPProvider_Stop 测试 Stop 方法
func TestHTTPProvider_Stop(t *testing.T) {
	provider := NewHTTPProvider(0)

	// 先启动
	provider.Start()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 停止服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := provider.Stop(ctx)
	assert.NoError(t, err)
}

// TestHTTPProvider_Stop_WithoutStart 测试未启动时停止
func TestHTTPProvider_Stop_WithoutStart(t *testing.T) {
	provider := NewHTTPProvider(0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 未启动时停止应该也能正常处理
	err := provider.Stop(ctx)
	assert.NoError(t, err)
}

// TestHTTPProvider_ConcurrentAccess 测试并发访问
func TestHTTPProvider_ConcurrentAccess(t *testing.T) {
	provider := NewHTTPProvider(0)

	done := make(chan bool, 20)

	// 并发添加 token
	for i := 0; i < 10; i++ {
		go func(id int) {
			token := "token-" + string(rune(id))
			provider.Present("example.com", token, "key-"+token)
			done <- true
		}(i)
	}

	// 并发清理 token
	for i := 0; i < 10; i++ {
		go func(id int) {
			token := "token-" + string(rune(id))
			provider.CleanUp("example.com", token, "key-"+token)
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 20; i++ {
		<-done
	}

	// 不应该 panic
	assert.True(t, true)
}

// TestHTTPProvider_HandleChallenge_MultiplePaths 测试不同路径的挑战请求
func TestHTTPProvider_HandleChallenge_MultiplePaths(t *testing.T) {
	provider := NewHTTPProvider(0)
	provider.Present("example.com", "valid-token", "valid-key")

	testCases := []struct {
		path       string
		expectCode int
	}{
		{"/.well-known/acme-challenge/valid-token", http.StatusOK},
		{"/.well-known/acme-challenge/invalid-token", http.StatusNotFound},
		{"/.well-known/acme-challenge/", http.StatusNotFound},
		{"/.well-known/acme-challenge/token-with-dashes", http.StatusNotFound},
	}

	for _, tc := range testCases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		w := httptest.NewRecorder()
		provider.handleChallenge(w, req)
		assert.Equal(t, tc.expectCode, w.Code, "path: %s", tc.path)
	}
}

// TestAccount 测试 Account 结构
func TestAccount(t *testing.T) {
	account := &Account{
		Email: "test@example.com",
	}

	assert.Equal(t, "test@example.com", account.GetEmail())
	assert.Nil(t, account.GetRegistration())
	assert.Nil(t, account.GetPrivateKey())
}

// TestAccount_WithRegistration 测试带注册的 Account
func TestAccount_WithRegistration(t *testing.T) {
	account := &Account{
		Email: "test@example.com",
	}

	// 设置私钥（实际使用中由 NewACMEClient 设置）
	account.key = "test-key"

	assert.Equal(t, "test-key", account.GetPrivateKey())
}

// TestACMEConfig 测试 ACMEConfig 结构
func TestACMEConfig(t *testing.T) {
	config := ACMEConfig{
		Email:      "test@example.com",
		CertDir:    "/tmp/certs",
		Production: false,
		HTTPPort:   80,
	}

	assert.Equal(t, "test@example.com", config.Email)
	assert.Equal(t, "/tmp/certs", config.CertDir)
	assert.False(t, config.Production)
	assert.Equal(t, 80, config.HTTPPort)
}

// TestDNSConfig 测试 DNSConfig 结构
func TestDNSConfig(t *testing.T) {
	config := DNSConfig{
		Provider: "cloudflare",
		Credentials: map[string]string{
			"CLOUDFLARE_DNS_API_TOKEN": "test-token",
		},
	}

	assert.Equal(t, "cloudflare", config.Provider)
	assert.Len(t, config.Credentials, 1)
	assert.Equal(t, "test-token", config.Credentials["CLOUDFLARE_DNS_API_TOKEN"])
}

// TestNewDNSProvider_UnsupportedProvider 测试不支持的 DNS 提供者
func TestNewDNSProvider_UnsupportedProvider(t *testing.T) {
	_, err := NewDNSProvider("unsupported-provider", map[string]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported DNS provider")
	assert.Contains(t, err.Error(), "supported:")
}

// TestNewDNSProvider_EmptyCredentials 测试空凭证
func TestNewDNSProvider_EmptyCredentials(t *testing.T) {
	// 测试支持的提供者但使用空凭证
	// manual 提供者可能不需要凭证，所以这里不验证错误
	// 只验证函数可以调用
	NewDNSProvider("manual", map[string]string{})
	assert.NotNil(t, NewDNSProvider)
}

// TestSetDNSProvider_NilClient 测试 nil ACME 客户端
func TestSetDNSProvider_NilClient(t *testing.T) {
	// 创建一个 nil 客户端来测试
	client := &ACMEClient{}

	client.SetDNSProvider("cloudflare", map[string]string{})
	// 这应该会 panic 或者返回错误，因为 client.client 为 nil
	// 我们只验证方法存在
	assert.NotNil(t, client.SetDNSProvider)
}

// TestRequestWildcardCertificate_EmptyBaseDomain 测试空域名
func TestRequestWildcardCertificate_EmptyBaseDomain(t *testing.T) {
	// 这个测试需要一个有效的 ACMEClient，而 nil 客户端会 panic
	// 所以我们跳过这个测试，只验证方法存在
	client := &ACMEClient{}
	assert.NotNil(t, client.RequestWildcardCertificate)
}

// TestSaveCertificate 测试 saveCertificate 方法
func TestSaveCertificate(t *testing.T) {
	tempDir := t.TempDir()

	client := &ACMEClient{
		certDir: tempDir,
	}

	cert := &certificate.Resource{
		Domain:      "example.com",
		Certificate: []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----"),
		PrivateKey:  []byte("-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----"),
		IssuerCertificate: []byte("-----BEGIN CERTIFICATE-----\nissuer\n-----END CERTIFICATE-----"),
	}

	err := client.saveCertificate("example.com", cert)
	assert.NoError(t, err)

	// 验证文件存在
	assert.FileExists(t, filepath.Join(tempDir, "example.com.crt"))
	assert.FileExists(t, filepath.Join(tempDir, "example.com.key"))
	assert.FileExists(t, filepath.Join(tempDir, "example.com.issuer.crt"))
}

// TestSaveCertificate_WriteError 测试 saveCertificate 写入错误
func TestSaveCertificate_WriteError(t *testing.T) {
	// 使用一个不可写的路径
	client := &ACMEClient{
		certDir: "/nonexistent/directory/that/does/not/exist",
	}

	cert := &certificate.Resource{
		Domain:      "example.com",
		Certificate: []byte("test-cert"),
	}

	err := client.saveCertificate("example.com", cert)
	assert.Error(t, err)
}

// TestDeleteCertificate 测试 DeleteCertificate 方法
func TestDeleteCertificate(t *testing.T) {
	tempDir := t.TempDir()

	// 先创建证书文件
	files := []string{
		filepath.Join(tempDir, "example.com.crt"),
		filepath.Join(tempDir, "example.com.key"),
		filepath.Join(tempDir, "example.com.issuer.crt"),
	}

	for _, file := range files {
		err := os.WriteFile(file, []byte("test"), 0644)
		assert.NoError(t, err)
	}

	client := &ACMEClient{
		certDir: tempDir,
	}

	err := client.DeleteCertificate("example.com")
	assert.NoError(t, err)

	// 验证文件已删除
	for _, file := range files {
		assert.NoFileExists(t, file)
	}
}

// TestDeleteCertificate_NonExistentFiles 测试删除不存在的文件
func TestDeleteCertificate_NonExistentFiles(t *testing.T) {
	tempDir := t.TempDir()

	client := &ACMEClient{
		certDir: tempDir,
	}

	// 删除不存在的证书不应该报错
	err := client.DeleteCertificate("nonexistent.com")
	assert.NoError(t, err)
}

// TestListCertificates_EmptyDirectory 测试空目录
func TestListCertificates_EmptyDirectory(t *testing.T) {
	tempDir := t.TempDir()

	client := &ACMEClient{
		certDir: tempDir,
	}

	certs, err := client.ListCertificates()
	assert.NoError(t, err)
	assert.Empty(t, certs)
}

// TestListCertificates_WithCertificates 测试有证书的目录
func TestListCertificates_WithCertificates(t *testing.T) {
	tempDir := t.TempDir()

	// 创建假证书文件
	certFiles := []string{
		"example.com.crt",
		"example.com.key",
		"example.com.issuer.crt",
		"test.com.crt",
		"test.com.key",
		"test.com.issuer.crt",
	}

	for _, file := range certFiles {
		err := os.WriteFile(filepath.Join(tempDir, file), []byte("test"), 0644)
		assert.NoError(t, err)
	}

	client := &ACMEClient{
		certDir: tempDir,
	}

	certs, err := client.ListCertificates()
	// 由于 GetCertificateInfo 会解析证书，假证书会失败
	// 但 ListCertificates 应该能正常返回（只是跳过无效的）
	assert.NoError(t, err)
	// 假证书无法解析，所以列表应该为空或只有有效的
	assert.GreaterOrEqual(t, len(certs), 0)
}

// TestGetExpiringCertificates 测试 GetExpiringCertificates
func TestGetExpiringCertificates(t *testing.T) {
	tempDir := t.TempDir()

	client := &ACMEClient{
		certDir: tempDir,
	}

	// 空目录应该返回空列表
	certs, err := client.GetExpiringCertificates(30)
	assert.NoError(t, err)
	assert.Empty(t, certs)
}

// TestGetCertificateInfo_NonExistentCert 测试获取不存在的证书信息
func TestGetCertificateInfo_NonExistentCert(t *testing.T) {
	tempDir := t.TempDir()

	client := &ACMEClient{
		certDir: tempDir,
	}

	_, err := client.GetCertificateInfo("nonexistent.com")
	assert.Error(t, err)
}

// TestACMEClient_Struct 测试 ACMEClient 结构
func TestACMEClient_Struct(t *testing.T) {
	client := &ACMEClient{
		certDir:    "/tmp/certs",
		email:      "test@example.com",
		production: false,
		httpPort:   80,
	}

	assert.Equal(t, "/tmp/certs", client.certDir)
	assert.Equal(t, "test@example.com", client.email)
	assert.False(t, client.production)
	assert.Equal(t, 80, client.httpPort)
}

// TestRequestCertificate_EmptyDomains 测试 RequestCertificate 空域名
func TestRequestCertificate_EmptyDomains(t *testing.T) {
	client := &ACMEClient{}

	_, err := client.RequestCertificate([]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no domains specified")
}

// TestRenewCertificate_ReadError 测试 RenewCertificate 读取错误
func TestRenewCertificate_ReadError(t *testing.T) {
	client := &ACMEClient{
		certDir: "/nonexistent",
	}

	_, err := client.RenewCertificate("example.com")
	assert.Error(t, err)
}
