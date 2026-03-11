package ssl

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	"prerender-shield/internal/logging"
)

// ACMEConfig ACME 配置
type ACMEConfig struct {
	Email      string
	CertDir    string
	Production bool
	HTTPPort   int
}

// ACMEClient ACME 客户端封装
type ACMEClient struct {
	client     *lego.Client
	certDir    string
	email      string
	account    *Account
	production bool
	httpPort   int
}

// Account ACME 账户
type Account struct {
	Email        string                 `json:"email"`
	Registration *registration.Resource `json:"registration"`
	key          crypto.PrivateKey
}

func (a *Account) GetEmail() string {
	return a.Email
}

func (a *Account) GetRegistration() *registration.Resource {
	return a.Registration
}

func (a *Account) GetPrivateKey() crypto.PrivateKey {
	return a.key
}

// NewACMEClient 创建 ACME 客户端
func NewACMEClient(config ACMEConfig) (*ACMEClient, error) {
	// 生成账户密钥
	accountKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate account key: %w", err)
	}

	// 创建账户
	account := &Account{
		Email: config.Email,
		key:   accountKey,
	}

	// 配置 ACME 目录 URL
	directoryURL := lego.LEDirectoryStaging
	if config.Production {
		directoryURL = lego.LEDirectoryProduction
	}

	// 创建 LEGO 客户端配置（先创建临时配置用于注册）
	legoConfig := &lego.Config{
		CADirURL: directoryURL,
		UserAgent: "PrerenderShield/1.0",
	}

	// 创建客户端（用于注册）
	client, err := lego.NewClient(legoConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create lego client: %w", err)
	}

	// 注册账户
	reg, err := client.Registration.Register(registration.RegisterOptions{
		TermsOfServiceAgreed: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to register account: %w", err)
	}
	account.Registration = reg

	// 重新创建客户端配置（使用已注册的账户）
	legoConfig = &lego.Config{
		CADirURL: directoryURL,
		UserAgent: "PrerenderShield/1.0",
	}
	legoConfig.Certificate.KeyType = certcrypto.RSA2048

	// 设置 HTTP-01 挑战提供者
	httpProvider := NewHTTPProvider(config.HTTPPort)
	err = client.Challenge.SetHTTP01Provider(httpProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to set HTTP01 provider: %w", err)
	}

	// 启动 HTTP 挑战服务器
	err = httpProvider.Start()
	if err != nil {
		logging.DefaultLogger.Warn("Failed to start HTTP challenge server: %v", err)
	}

	// 确保证书目录存在
	if err := os.MkdirAll(config.CertDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cert directory: %w", err)
	}

	return &ACMEClient{
		client:     client,
		certDir:    config.CertDir,
		email:      config.Email,
		account:    account,
		production: config.Production,
		httpPort:   config.HTTPPort,
	}, nil
}

// RequestCertificate 申请证书
func (c *ACMEClient) RequestCertificate(domains []string) (*certificate.Resource, error) {
	if len(domains) == 0 {
		return nil, errors.New("no domains specified")
	}

	logging.DefaultLogger.Info("Requesting certificate for domains: %v", domains)

	// 创建订单
	request := certificate.ObtainRequest{
		Domains: domains,
		Bundle:  true,
	}

	cert, err := c.client.Certificate.Obtain(request)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain certificate: %w", err)
	}

	logging.DefaultLogger.Info("Certificate obtained for domain: %s", cert.Domain)

	// 保存证书到文件
	err = c.saveCertificate(domains[0], cert)
	if err != nil {
		return nil, fmt.Errorf("failed to save certificate: %w", err)
	}

	return cert, nil
}

// RenewCertificate 续签证书
func (c *ACMEClient) RenewCertificate(domain string) (*certificate.Resource, error) {
	// 加载现有证书
	certPath := filepath.Join(c.certDir, fmt.Sprintf("%s.crt", domain))
	keyPath := filepath.Join(c.certDir, fmt.Sprintf("%s.key", domain))

	// 读取证书文件
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate: %w", err)
	}

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	// 从 PEM 数据加载证书（LEGO v4 没有 NewCertFromPEM，手动解析）
	cert := &certificate.Resource{
		Domain:        domain,
		Certificate:   certData,
		PrivateKey:    keyData,
		IssuerCertificate: certData, // 简化处理，使用相同数据
	}

	logging.DefaultLogger.Info("Renewing certificate for domain: %s", domain)

	// 续签证书
	renewedCert, err := c.client.Certificate.Renew(*cert, true, true, "")
	if err != nil {
		return nil, fmt.Errorf("failed to renew certificate: %w", err)
	}

	logging.DefaultLogger.Info("Certificate renewed for domain: %s", domain)

	// 保存新证书
	err = c.saveCertificate(domain, renewedCert)
	if err != nil {
		return nil, fmt.Errorf("failed to save renewed certificate: %w", err)
	}

	return renewedCert, nil
}

// saveCertificate 保存证书到文件
func (c *ACMEClient) saveCertificate(domain string, cert *certificate.Resource) error {
	// 保存证书（包含完整证书链）
	certPath := filepath.Join(c.certDir, fmt.Sprintf("%s.crt", domain))
	if err := os.WriteFile(certPath, cert.Certificate, 0644); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	// 保存私钥
	keyPath := filepath.Join(c.certDir, fmt.Sprintf("%s.key", domain))
	if err := os.WriteFile(keyPath, cert.PrivateKey, 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	// 保存 issuer 证书（中间证书）
	issuerPath := filepath.Join(c.certDir, fmt.Sprintf("%s.issuer.crt", domain))
	if err := os.WriteFile(issuerPath, cert.IssuerCertificate, 0644); err != nil {
		return fmt.Errorf("failed to write issuer certificate: %w", err)
	}

	logging.DefaultLogger.Info("Certificate saved: %s", certPath)

	return nil
}

// GetCertificateInfo 获取证书信息
func (c *ACMEClient) GetCertificateInfo(domain string) (map[string]interface{}, error) {
	certPath := filepath.Join(c.certDir, fmt.Sprintf("%s.crt", domain))

	certData, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate: %w", err)
	}

	// 解析证书
	certs, err := certcrypto.ParsePEMBundle(certData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	if len(certs) == 0 {
		return nil, errors.New("no certificates found")
	}

	// 获取终端实体证书（通常是第一个）
	cert := certs[0]

	info := map[string]interface{}{
		"domain":     domain,
		"subject":    cert.Subject.CommonName,
		"issuer":     cert.Issuer.CommonName,
		"not_before": cert.NotBefore,
		"not_after":  cert.NotAfter,
		"dns_names":  cert.DNSNames,
		"expires_in": int(time.Until(cert.NotAfter).Hours() / 24),
		"expired":    time.Now().After(cert.NotAfter),
	}

	return info, nil
}

// DeleteCertificate 删除证书
func (c *ACMEClient) DeleteCertificate(domain string) error {
	files := []string{
		filepath.Join(c.certDir, fmt.Sprintf("%s.crt", domain)),
		filepath.Join(c.certDir, fmt.Sprintf("%s.key", domain)),
		filepath.Join(c.certDir, fmt.Sprintf("%s.issuer.crt", domain)),
	}

	for _, file := range files {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete %s: %w", file, err)
		}
	}

	logging.DefaultLogger.Info("Certificate deleted: %s", domain)
	return nil
}

// ListCertificates 列出所有证书
func (c *ACMEClient) ListCertificates() ([]map[string]interface{}, error) {
	entries, err := os.ReadDir(c.certDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read cert directory: %w", err)
	}

	var certificates []map[string]interface{}
	domains := make(map[string]bool)

	// 从.crt 文件提取域名
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) > 4 && name[len(name)-4:] == ".crt" && name[len(name)-12:] != ".issuer.crt" {
			domain := name[:len(name)-4]
			domains[domain] = true
		}
	}

	// 获取每个域名的证书信息
	for domain := range domains {
		info, err := c.GetCertificateInfo(domain)
		if err != nil {
			logging.DefaultLogger.Warn("Failed to get cert info for %s: %v", domain, err)
			continue
		}
		certificates = append(certificates, info)
	}

	return certificates, nil
}

// GetExpiringCertificates 获取即将过期的证书
func (c *ACMEClient) GetExpiringCertificates(days int) ([]map[string]interface{}, error) {
	certs, err := c.ListCertificates()
	if err != nil {
		return nil, err
	}

	var expiring []map[string]interface{}
	for _, cert := range certs {
		if expiresIn, ok := cert["expires_in"].(int); ok {
			if expiresIn <= days && expiresIn > 0 {
				expiring = append(expiring, cert)
			}
		}
	}

	return expiring, nil
}
