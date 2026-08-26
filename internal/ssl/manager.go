package ssl

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"prerender-shield/internal/logging"
	"prerender-shield/internal/redis"
)

// Manager SSL证书管理器接口
type Manager interface {
	RequestCertificate(domain string) error
	RenewCertificate(domain string) error
	ImportCertificate(domain, certPath, keyPath string) error
	GetCertificate(domain string) (*tls.Certificate, error)
	GetCertificateStatus(domain string) (map[string]interface{}, error)
	ListCertificates() (map[string]map[string]interface{}, error)
	DeleteCertificate(domain string) error
	CheckExpiration() ([]string, error)
	SetACMEClient(client *ACMEClient)
}

// manager SSL证书管理器实现
// P0-12: 删除未使用的 acmeClient (golang.org/x/crypto/acme) 字段
// 统一使用 acmeClientWrapper (lego/v4)
type manager struct {
	redisClient       *redis.Client
	acmeClientWrapper *ACMEClient
	certDir           string
	email             string
	production        bool
}

// SetACMEClient 设置 ACME 客户端（用于真实证书申请）
func (m *manager) SetACMEClient(client *ACMEClient) {
	m.acmeClientWrapper = client
}

// NewManager 创建新的SSL证书管理器
func NewManager(redisClient *redis.Client, certDir, email string, production bool) (Manager, error) {
	// 确保证书目录存在
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cert directory: %w", err)
	}

	return &manager{
		redisClient: redisClient,
		certDir:     certDir,
		email:       email,
		production:  production,
	}, nil
}

// RequestCertificate 申请SSL证书
func (m *manager) RequestCertificate(domain string) error {
	if m.redisClient == nil {
		return fmt.Errorf("redis client is nil")
	}
	// 生成域名密钥
	domainKey, err := generatePrivateKey()
	if err != nil {
		return fmt.Errorf("failed to generate domain key: %w", err)
	}

	// 创建CSR
	csr, err := generateCSR(domainKey, []string{domain})
	if err != nil {
		return fmt.Errorf("failed to generate CSR: %w", err)
	}

	// 保存证书和密钥
	certPath := filepath.Join(m.certDir, fmt.Sprintf("%s.crt", domain))
	keyPath := filepath.Join(m.certDir, fmt.Sprintf("%s.key", domain))

	if err := savePrivateKey(keyPath, domainKey); err != nil {
		return fmt.Errorf("failed to save domain key: %w", err)
	}

	// 尝试使用ACME申请证书
	certPEM, err := m.requestCertificateWithACME(domain, domainKey, csr)
	if err != nil {
		// ACME 申请失败时不再回退到自签名证书，直接返回错误
		m.redisClient.Set(fmt.Sprintf("ssl:acme:error:%s", domain), err.Error(), 24*time.Hour)
		return fmt.Errorf("failed to obtain certificate via ACME for domain %s: %w", domain, err)
	}

	if err := saveCertificate(certPath, certPEM); err != nil {
		return fmt.Errorf("failed to save certificate: %w", err)
	}

	// 从实际证书解析过期时间，避免硬编码
	expiresAt := time.Now().Add(90 * 24 * time.Hour) // 默认值作为回退
	if parsedExpiry, parseErr := extractCertExpiry(certPEM); parseErr == nil {
		expiresAt = parsedExpiry
	}

	// 存储证书信息到Redis
	certInfo := map[string]interface{}{
		"domain":     domain,
		"cert_path":  certPath,
		"key_path":   keyPath,
		"created_at": time.Now().Unix(),
		"expires_at": expiresAt.Unix(),
		"status":     "active",
	}

	if err := m.redisClient.SaveJSON(fmt.Sprintf("ssl:cert:%s", domain), certInfo, 0); err != nil {
		return err
	}

	// 将域名添加到证书集合
	return m.redisClient.SetAdd("ssl:certs", domain)
}

// RenewCertificate 续签SSL证书
func (m *manager) RenewCertificate(domain string) error {
	if m.redisClient == nil {
		return fmt.Errorf("redis client is nil")
	}
	// 检查证书是否存在
	certInfo := make(map[string]interface{})
	if err := m.redisClient.GetJSON(fmt.Sprintf("ssl:cert:%s", domain), &certInfo); err != nil {
		return fmt.Errorf("certificate not found: %w", err)
	}

	// 重新申请证书
	err := m.RequestCertificate(domain)
	if err == nil {
		// 从证书文件读取实际过期时间
		certPath := filepath.Join(m.certDir, fmt.Sprintf("%s.crt", domain))
		certData, readErr := os.ReadFile(certPath)
		expiresAt := time.Now().Add(90 * 24 * time.Hour) // 默认回退
		if readErr == nil {
			if parsedExpiry, parseErr := extractCertExpiry(certData); parseErr == nil {
				expiresAt = parsedExpiry
			}
		}
		// 更新证书信息
		certInfo["updated_at"] = time.Now().Unix()
		certInfo["expires_at"] = expiresAt.Unix()
		m.redisClient.SaveJSON(fmt.Sprintf("ssl:cert:%s", domain), certInfo, 0)
	}

	return err
}

// ImportCertificate 导入SSL证书
func (m *manager) ImportCertificate(domain, certPath, keyPath string) error {
	if m.redisClient == nil {
		return fmt.Errorf("redis client is nil")
	}
	// 读取证书文件
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("failed to read certificate file: %w", err)
	}

	// 读取密钥文件
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read key file: %w", err)
	}

	// 解析证书
	block, _ := pem.Decode(certData)
	if block == nil || block.Type != "CERTIFICATE" {
		return fmt.Errorf("invalid certificate file")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// 复制证书文件到证书目录
	newCertPath := filepath.Join(m.certDir, fmt.Sprintf("%s.crt", domain))
	newKeyPath := filepath.Join(m.certDir, fmt.Sprintf("%s.key", domain))

	if err := os.WriteFile(newCertPath, certData, 0644); err != nil {
		return fmt.Errorf("failed to write certificate file: %w", err)
	}

	if err := os.WriteFile(newKeyPath, keyData, 0600); err != nil {
		return fmt.Errorf("failed to write key file: %w", err)
	}

	// 存储证书信息到Redis
	certInfo := map[string]interface{}{
		"domain":     domain,
		"cert_path":  newCertPath,
		"key_path":   newKeyPath,
		"created_at": time.Now().Unix(),
		"expires_at": cert.NotAfter.Unix(),
		"status":     "active",
		"imported":   true,
	}

	if err := m.redisClient.SaveJSON(fmt.Sprintf("ssl:cert:%s", domain), certInfo, 0); err != nil {
		return err
	}

	// 将域名添加到证书集合
	return m.redisClient.SetAdd("ssl:certs", domain)
}

// GetCertificate 获取SSL证书
func (m *manager) GetCertificate(domain string) (*tls.Certificate, error) {
	if m.redisClient == nil {
		return nil, fmt.Errorf("redis client is nil")
	}
	// 检查证书是否存在
	certInfo := make(map[string]interface{})
	if err := m.redisClient.GetJSON(fmt.Sprintf("ssl:cert:%s", domain), &certInfo); err != nil {
		return nil, fmt.Errorf("certificate not found: %w", err)
	}

	// 获取证书路径
	certPath, ok := certInfo["cert_path"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid cert_path")
	}

	keyPath, ok := certInfo["key_path"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid key_path")
	}

	// 读取证书
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %w", err)
	}

	return &cert, nil
}

// GetCertificateStatus 获取SSL证书状态
func (m *manager) GetCertificateStatus(domain string) (map[string]interface{}, error) {
	if m.redisClient == nil {
		return nil, fmt.Errorf("redis client is nil")
	}
	// 检查证书是否存在
	certInfo := make(map[string]interface{})
	if err := m.redisClient.GetJSON(fmt.Sprintf("ssl:cert:%s", domain), &certInfo); err != nil {
		return nil, fmt.Errorf("certificate not found: %w", err)
	}

	// 检查证书是否过期
	expiresAt, ok := certInfo["expires_at"].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid expires_at")
	}

	expiryTime := time.Unix(int64(expiresAt), 0)
	remaining := time.Until(expiryTime)

	certInfo["remaining_days"] = int(remaining.Hours() / 24)
	certInfo["expired"] = remaining < 0

	return certInfo, nil
}

// ListCertificates 列出所有SSL证书
func (m *manager) ListCertificates() (map[string]map[string]interface{}, error) {
	if m.redisClient == nil {
		return nil, fmt.Errorf("redis client is nil")
	}
	// 获取所有证书域名
	certDomains, err := m.redisClient.SetMembers("ssl:certs")
	if err != nil {
		return nil, fmt.Errorf("failed to get cert domains: %w", err)
	}

	certificates := make(map[string]map[string]interface{})
	for _, domain := range certDomains {
		status, err := m.GetCertificateStatus(domain)
		if err != nil {
			continue
		}
		certificates[domain] = status
	}

	return certificates, nil
}

// DeleteCertificate 删除SSL证书
func (m *manager) DeleteCertificate(domain string) error {
	if m.redisClient == nil {
		return fmt.Errorf("redis client is nil")
	}
	// 检查证书是否存在
	certInfo := make(map[string]interface{})
	if err := m.redisClient.GetJSON(fmt.Sprintf("ssl:cert:%s", domain), &certInfo); err != nil {
		return fmt.Errorf("certificate not found: %w", err)
	}

	// 删除证书文件
	if certPath, ok := certInfo["cert_path"].(string); ok {
		if err := os.Remove(certPath); err != nil && !os.IsNotExist(err) {
			logging.DefaultLogger.Warn("Failed to delete certificate file %s: %v", certPath, err)
		}
	}

	if keyPath, ok := certInfo["key_path"].(string); ok {
		if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
			logging.DefaultLogger.Warn("Failed to delete key file %s: %v", keyPath, err)
		}
	}

	// 从Redis中删除证书信息
	if err := m.redisClient.Del(fmt.Sprintf("ssl:cert:%s", domain)); err != nil {
		return fmt.Errorf("failed to delete cert info: %w", err)
	}

	// 从证书列表中移除
	return m.redisClient.SetRemove("ssl:certs", domain)
}

// CheckExpiration 检查过期的证书
func (m *manager) CheckExpiration() ([]string, error) {
	if m.redisClient == nil {
		return nil, fmt.Errorf("redis client is nil")
	}
	// 获取所有证书
	certificates, err := m.ListCertificates()
	if err != nil {
		return nil, fmt.Errorf("failed to list certificates: %w", err)
	}

	expiring := []string{}
	for domain, info := range certificates {
		if remaining, ok := info["remaining_days"].(int); ok && remaining <= 30 {
			expiring = append(expiring, domain)
		}
	}

	return expiring, nil
}

// generatePrivateKey 生成私钥
func generatePrivateKey() (*rsa.PrivateKey, error) {
	// 生成2048位RSA私钥
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}
	return privateKey, nil
}

// generateCSR 生成CSR
func generateCSR(key *rsa.PrivateKey, domains []string) (*x509.CertificateRequest, error) {
	// 创建CSR模板
	template := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: domains[0],
		},
		DNSNames: domains,
	}

	// 生成CSR
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &template, key)
	if err != nil {
		return nil, fmt.Errorf("failed to create CSR: %w", err)
	}

	// 解析CSR
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSR: %w", err)
	}

	return csr, nil
}

// savePrivateKey 保存私钥
func savePrivateKey(path string, key *rsa.PrivateKey) error {
	// 将私钥转换为PEM格式
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(key)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// 保存私钥到文件
	if err := os.WriteFile(path, privateKeyPEM, 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	return nil
}

// saveCertificate 保存证书
func saveCertificate(path string, certData []byte) error {
	// 保存证书到文件
	if err := os.WriteFile(path, certData, 0644); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	return nil
}

// createSelfSignedCertificate 创建自签名证书（仅用于开发和测试）
func (m *manager) createSelfSignedCertificate(domain string, privateKey *rsa.PrivateKey) ([]byte, error) {
	// 使用随机序列号，避免固定值
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}
	// 创建证书模板
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: domain,
		},
		DNSNames:              []string{domain},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(90 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// 生成证书
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// 转换为PEM格式
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	return certPEM, nil
}

// requestCertificateWithACME 使用ACME协议申请证书
// 优先使用注入的 ACMEClient（lego/v4），否则返回错误
func (m *manager) requestCertificateWithACME(domain string, privateKey *rsa.PrivateKey, csr *x509.CertificateRequest) ([]byte, error) {
	if m.acmeClientWrapper == nil {
		return nil, fmt.Errorf("ACME client not configured")
	}
	cert, err := m.acmeClientWrapper.RequestCertificate([]string{domain})
	if err != nil {
		return nil, err
	}
	return cert.Certificate, nil
}

// extractCertExpiry 从 PEM 编码的证书中解析过期时间
func extractCertExpiry(certPEM []byte) (time.Time, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return time.Time{}, fmt.Errorf("failed to decode PEM block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse certificate: %w", err)
	}
	return cert.NotAfter, nil
}
