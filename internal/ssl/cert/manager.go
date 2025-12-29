package cert

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Manager 证书管理器
type Manager struct {
	certs    map[string]*tls.Certificate
	mutex    sync.RWMutex
	acme     *ACMEManager
	config   *Config
}

// Config 证书配置
type Config struct {
	Enabled          bool
	LetEncrypt       bool
	Domains          []string
	ACMEEmail        string
	ACMEServer       string
	ACMEChallenge    string
	CertPath         string
	KeyPath          string
	CertDir          string
}

// ACMEManager ACME证书管理器
type ACMEManager struct {
	config *Config
	certs  map[string]*CertInfo
}

// CertInfo 证书信息
type CertInfo struct {
	Domain    string
	CertPath  string
	KeyPath   string
	ExpiresAt time.Time
}

// NewManager 创建新的证书管理器
func NewManager(config *Config) (*Manager, error) {
	// 确保证书目录存在
	if err := os.MkdirAll(config.CertDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cert dir: %w", err)
	}
	
	m := &Manager{
		certs:  make(map[string]*tls.Certificate),
		config: config,
	}
	
	// 初始化ACME管理器
	if config.LetEncrypt {
		acme, err := NewACMEManager(config)
		if err != nil {
			return nil, err
		}
		m.acme = acme
	}
	
	// 加载初始证书
	if err := m.loadCertificates(); err != nil {
		return nil, err
	}
	
	return m, nil
}

// NewACMEManager 创建新的ACME管理器
func NewACMEManager(config *Config) (*ACMEManager, error) {
	return &ACMEManager{
		config: config,
		certs:  make(map[string]*CertInfo),
	}, nil
}

// GetCertificate 根据SNI获取证书
func (m *Manager) GetCertificate(h *tls.ClientHelloInfo) (*tls.Certificate, error) {
	// 检查是否存在该域名的证书
	m.mutex.RLock()
	cert, ok := m.certs[h.ServerName]
	m.mutex.RUnlock()
	
	if ok {
		return cert, nil
	}
	
	// 尝试获取默认证书
	m.mutex.RLock()
	cert, ok = m.certs["default"]
	m.mutex.RUnlock()
	
	if ok {
		return cert, nil
	}
	
	// 如果启用了LetEncrypt，尝试自动申请证书
	if m.acme != nil {
		if err := m.acme.IssueCertificate(h.ServerName); err == nil {
			// 重新加载证书
			if err := m.loadCertificates(); err == nil {
				m.mutex.RLock()
				cert, ok = m.certs[h.ServerName]
				m.mutex.RUnlock()
				if ok {
					return cert, nil
				}
			}
		}
	}
	
	return nil, nil
}

// loadCertificates 加载证书
func (m *Manager) loadCertificates() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// 清空现有证书
	m.certs = make(map[string]*tls.Certificate)
	
	// 加载默认证书（如果配置了）
	if m.config.CertPath != "" && m.config.KeyPath != "" {
		cert, err := m.loadCertificateFromFile(m.config.CertPath, m.config.KeyPath)
		if err != nil {
			fmt.Printf("Warning: failed to load default cert: %v, skipping...\n", err)
			// 继续执行，不返回错误
		} else {
			m.certs["default"] = cert
		}
	}
	
	// 加载LetEncrypt证书
	if m.config.LetEncrypt {
		// 扫描证书目录，加载所有证书
		files, err := ioutil.ReadDir(m.config.CertDir)
		if err != nil {
			return fmt.Errorf("failed to read cert dir: %w", err)
		}
		
		// 构建证书文件映射
		certFiles := make(map[string]string)
		keyFiles := make(map[string]string)
		
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			
			filename := file.Name()
			filePath := filepath.Join(m.config.CertDir, filename)
			
			if filepath.Ext(filename) == ".pem" {
				if filepath.Base(filename) == "cert.pem" {
					// 这是一个证书文件，获取域名目录
					domain := filepath.Base(filepath.Dir(filePath))
					certFiles[domain] = filePath
				} else if filepath.Base(filename) == "key.pem" {
					// 这是一个密钥文件，获取域名目录
					domain := filepath.Base(filepath.Dir(filePath))
					keyFiles[domain] = filePath
				}
			}
		}
		
		// 加载所有证书
		for domain, certPath := range certFiles {
			keyPath, ok := keyFiles[domain]
			if !ok {
				continue
			}
			
			cert, err := m.loadCertificateFromFile(certPath, keyPath)
			if err != nil {
				fmt.Printf("Warning: failed to load cert for %s: %v\n", domain, err)
				continue
			}
			
			m.certs[domain] = cert
			
			// 更新ACME管理器中的证书信息
			if m.acme != nil {
				m.acme.certs[domain] = &CertInfo{
					Domain:   domain,
					CertPath: certPath,
					KeyPath:  keyPath,
					// 这里应该解析证书获取过期时间，暂时使用当前时间+30天
					ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
				}
			}
		}
	}
	
	return nil
}

// loadCertificateFromFile 从文件加载证书
func (m *Manager) loadCertificateFromFile(certPath, keyPath string) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	
	return &cert, nil
}

// Start 启动证书管理器
func (m *Manager) Start() {
	// 启动证书更新定时器
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		
		for range ticker.C {
			if m.acme != nil {
				m.acme.RenewCertificates()
				m.loadCertificates()
			}
		}
	}()
}

// IssueCertificate 申请证书
func (a *ACMEManager) IssueCertificate(domain string) error {
	// 检查是否已存在该域名的证书
	if _, exists := a.certs[domain]; exists {
		return nil
	}
	
	// 创建域名证书目录
	domainDir := filepath.Join(a.config.CertDir, domain)
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		return fmt.Errorf("failed to create domain dir: %w", err)
	}
	
	// 这里应该实现真正的ACME证书申请逻辑
	// 暂时生成自签名证书
	certPath := filepath.Join(domainDir, "cert.pem")
	keyPath := filepath.Join(domainDir, "key.pem")
	
	// 生成自签名证书（模拟）
	if err := a.generateSelfSignedCert(domain, certPath, keyPath); err != nil {
		return fmt.Errorf("failed to generate self-signed cert: %w", err)
	}
	
	// 更新证书信息
	a.certs[domain] = &CertInfo{
		Domain:   domain,
		CertPath: certPath,
		KeyPath:  keyPath,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}
	
	return nil
}

// RenewCertificates 续签证书
func (a *ACMEManager) RenewCertificates() {
	// 检查所有证书的过期时间，提前7天续签
	now := time.Now()
	for domain, certInfo := range a.certs {
		if now.Add(7 * 24 * time.Hour).After(certInfo.ExpiresAt) {
			// 需要续签证书
			if err := a.IssueCertificate(domain); err != nil {
				fmt.Printf("Warning: failed to renew cert for %s: %v\n", domain, err)
			}
		}
	}
}

// generateSelfSignedCert 生成自签名证书（模拟）
func (a *ACMEManager) generateSelfSignedCert(domain, certPath, keyPath string) error {
	// 这里应该实现真正的自签名证书生成逻辑
	// 暂时创建空文件作为模拟
	if err := ioutil.WriteFile(certPath, []byte("-----BEGIN CERTIFICATE-----\nMII...\n-----END CERTIFICATE-----\n"), 0644); err != nil {
		return err
	}
	
	if err := ioutil.WriteFile(keyPath, []byte("-----BEGIN PRIVATE KEY-----\nMII...\n-----END PRIVATE KEY-----\n"), 0600); err != nil {
		return err
	}
	
	return nil
}

// AddDomain 添加域名
func (m *Manager) AddDomain(domain string) error {
	// 如果启用了LetEncrypt，申请证书
	if m.config.LetEncrypt && m.acme != nil {
		if err := m.acme.IssueCertificate(domain); err != nil {
			return err
		}
		
		// 重新加载证书
		return m.loadCertificates()
	}
	
	return nil
}

// RemoveDomain 移除域名
func (m *Manager) RemoveDomain(domain string) error {
	// 移除证书文件
	domainDir := filepath.Join(m.config.CertDir, domain)
	if err := os.RemoveAll(domainDir); err != nil {
		return err
	}
	
	// 从内存中移除证书
	m.mutex.Lock()
	delete(m.certs, domain)
	m.mutex.Unlock()
	
	// 从ACME管理器中移除
	if m.acme != nil {
		delete(m.acme.certs, domain)
	}
	
	return nil
}

// GetDomains 获取所有域名
func (m *Manager) GetDomains() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	domains := make([]string, 0, len(m.certs))
	for domain := range m.certs {
		if domain != "default" {
			domains = append(domains, domain)
		}
	}
	
	return domains
}

// IsCertExpiring 检查证书是否即将过期
func (m *Manager) IsCertExpiring(domain string, days int) bool {
	if m.acme == nil {
		return false
	}
	
	certInfo, exists := m.acme.certs[domain]
	if !exists {
		return true
	}
	
	return time.Now().Add(time.Duration(days) * 24 * time.Hour).After(certInfo.ExpiresAt)
}
