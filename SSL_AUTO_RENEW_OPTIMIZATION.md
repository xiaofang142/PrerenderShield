# SSL 自动签署与域名证书配置优化方案

**任务 ID:** SSL-20260311-001
**制定日期:** 2026-03-11
**版本:** 1.0

---

## 一、现状分析

### 1.1 当前 SSL 模块功能

| 功能 | 状态 | 说明 |
|------|------|------|
| 证书申请接口 | ⚠️ 简化实现 | `requestCertificateWithACME` 返回自签名证书 |
| 证书续签 | ⚠️ 基础实现 | 依赖 ACME 但未完整实现 |
| 自动续签任务 | ✅ 已实现 | `StartAutoRenewal` 定期检查 |
| 证书导入 | ✅ 已实现 | 支持外部证书导入 |
| 证书状态查询 | ✅ 已实现 | Redis 存储证书信息 |
| HTTP-01 挑战 | ❌ 未实现 | 缺少挑战响应端点 |
| DNS-01 挑战 | ❌ 未实现 | 缺少 DNS API 集成 |
| 多域名支持 | ⚠️ 基础支持 | CSR 支持多域名但未验证 |

### 1.2 存在问题

1. **ACME 流程不完整** - 缺少账户注册、订单创建、挑战验证
2. **挑战响应缺失** - 没有 HTTP-01 挑战的 Web 端点
3. **DNS API 未集成** - 不支持 DNS-01 挑战（通配符证书必需）
4. **证书链不完整** - 未处理中间证书
5. **缺少错误重试** - 申请失败后无重试机制
6. **无 webhook 通知** - 证书更新后无通知机制

---

## 二、优化方案

### 2.1 完整 ACME 证书申请流程

#### 实现架构

```
┌─────────────────────────────────────────────────────────────┐
│                    SSL Certificate Manager                   │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ ACME Client  │  │ HTTP Challenge│  │ DNS Challenge │      │
│  │ (Let's Encrypt)│  │ Server       │  │ Providers    │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
│         │                  │                  │              │
│         ▼                  ▼                  ▼              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              Certificate Lifecycle Manager           │    │
│  │  • Register Account  • Create Order                 │    │
│  │  • Handle Challenges • Download Certificate         │    │
│  │  • Auto Renewal      • Webhook Notification        │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

#### 核心代码实现

```go
// internal/ssl/acme_client.go
package ssl

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
)

// ACMEClient ACME 客户端封装
type ACMEClient struct {
	client      *lego.Client
	certDir     string
	email       string
	domainKey   crypto.PrivateKey
	account     *Account
	challengeProvider ChallengeProvider
}

// Account ACME 账户
type Account struct {
	Email        string                 `json:"email"`
	Registration *registration.Resource `json:"registration"`
	Key          []byte                 `json:"key"`
}

func (a *Account) GetEmail() string {
	return a.Email
}

func (a *Account) GetRegistration() *registration.Resource {
	return a.Registration
}

func (a *Account) GetPrivateKey() crypto.PrivateKey {
	return a.Key
}

// ChallengeProvider 挑战提供者接口
type ChallengeProvider interface {
	// HTTP01 挑战
	PresentHTTP(token, keyAuth string) error
	CleanUpHTTP(token, keyAuth string) error

	// DNS-01 挑战
	PresentDNS(domain, fqdn, value string) error
	CleanUpDNS(domain, fqdn, value string) error
}

// ACMEConfig ACME 配置
type ACMEConfig struct {
	Email       string
	CertDir     string
	Production  bool
	HTTPPort    int
	DNSProvider string  // DNS 服务商：cloudflare, aliyun,.tencentcloud 等
}

// NewACMEClient 创建 ACME 客户端
func NewACMEClient(config ACMEConfig, provider ChallengeProvider) (*ACMEClient, error) {
	// 生成账户密钥
	accountKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate account key: %w", err)
	}

	// 创建账户
	account := &Account{
		Email: config.Email,
		Key:   accountKey,
	}

	// 配置 ACME 目录 URL
	directoryURL := lego.LEDirectoryStaging
	if config.Production {
		directoryURL = lego.LEDirectoryProduction
	}

	// 创建 LEGO 客户端配置
	legoConfig := lego.NewConfig()
	legoConfig.CADirURL = directoryURL
	legoConfig.Certificate.KeyType = certcrypto.RSA2048
	legoConfig.UserAgent = "PrerenderShield/1.0"

	// 创建客户端
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

	// 设置挑战提供者
	if provider != nil {
		// HTTP-01 挑战
		httpProvider := NewHTTPProvider(provider, config.HTTPPort)
		client.Challenge.SetHTTP01Provider(httpProvider)

		// DNS-01 挑战
		dnsProvider, err := NewDNSProvider(config.DNSProvider)
		if err != nil {
			return nil, fmt.Errorf("failed to create DNS provider: %w", err)
		}
		client.Challenge.SetDNS01Provider(dnsProvider)
	}

	return &ACMEClient{
		client:    client,
		certDir:   config.CertDir,
		email:     config.Email,
		domainKey: accountKey,
		account:   account,
	}, nil
}

// RequestCertificate 申请证书
func (c *ACMEClient) RequestCertificate(domains []string) (*certificate.Resource, error) {
	// 创建订单
	order, err := c.client.Certificate.ObtainForDomains(domains)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain certificate: %w", err)
	}

	// 保存证书到文件
	if err := c.saveCertificate(domains[0], order); err != nil {
		return nil, fmt.Errorf("failed to save certificate: %w", err)
	}

	return order, nil
}

// RenewCertificate 续签证书
func (c *ACMEClient) RenewCertificate(domain string, cert *certificate.Resource) (*certificate.Resource, error) {
	// 使用 LEGO 的续签功能
	renewedCert, err := c.client.Certificate.Renew(*cert, true, true, "")
	if err != nil {
		return nil, fmt.Errorf("failed to renew certificate: %w", err)
	}

	// 保存新证书
	if err := c.saveCertificate(domain, &renewedCert); err != nil {
		return nil, fmt.Errorf("failed to save renewed certificate: %w", err)
	}

	return &renewedCert, nil
}

// saveCertificate 保存证书到文件
func (c *ACMEClient) saveCertificate(domain string, cert *certificate.Resource) error {
	// 保存证书
	certPath := filepath.Join(c.certDir, fmt.Sprintf("%s.crt", domain))
	if err := ioutil.WriteFile(certPath, cert.Certificate, 0644); err != nil {
		return err
	}

	// 保存私钥
	keyPath := filepath.Join(c.certDir, fmt.Sprintf("%s.key", domain))
	if err := ioutil.WriteFile(keyPath, cert.PrivateKey, 0600); err != nil {
		return err
	}

	// 保存 issuer 证书（中间证书）
	issuerPath := filepath.Join(c.certDir, fmt.Sprintf("%s.issuer.crt", domain))
	if err := ioutil.WriteFile(issuerPath, cert.IssuerCertificate, 0644); err != nil {
		return err
	}

	return nil
}
```

---

### 2.2 HTTP-01 挑战服务器

```go
// internal/ssl/http_challenge.go
package ssl

import (
	"context"
	"fmt"
	"net/http"
	"sync"
)

// HTTPProvider HTTP-01 挑战提供者
type HTTPProvider struct {
	provider ChallengeProvider
	port     int
	server   *http.Server
	tokens   map[string]string
	mu       sync.RWMutex
}

// NewHTTPProvider 创建 HTTP 挑战提供者
func NewHTTPProvider(provider ChallengeProvider, port int) *HTTPProvider {
	h := &HTTPProvider{
		provider: provider,
		port:     port,
		tokens:   make(map[string]string),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/acme-challenge/", h.handleChallenge)

	h.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return h
}

// Present 设置挑战响应
func (h *HTTPProvider) Present(domain, token, keyAuth string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.tokens[token] = keyAuth

	if h.provider != nil {
		return h.provider.PresentHTTP(token, keyAuth)
	}
	return nil
}

// CleanUp 清理挑战响应
func (h *HTTPProvider) CleanUp(domain, token, keyAuth string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.tokens, token)

	if h.provider != nil {
		return h.provider.CleanUpHTTP(token, keyAuth)
	}
	return nil
}

// handleChallenge 处理 ACME 挑战请求
func (h *HTTPProvider) handleChallenge(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Path[len("/.well-known/acme-challenge/"):]

	h.mu.RLock()
	keyAuth, exists := h.tokens[token]
	h.mu.RUnlock()

	if !exists {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(keyAuth))
}

// Start 启动挑战服务器
func (h *HTTPProvider) Start() error {
	go func() {
		if err := h.server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("HTTP challenge server error: %v", err)
		}
	}()
	return nil
}

// Stop 停止挑战服务器
func (h *HTTPProvider) Stop(ctx context.Context) error {
	return h.server.Shutdown(ctx)
}
```

---

### 2.3 DNS-01 挑战提供者（支持多 DNS 服务商）

```go
// internal/ssl/dns_challenge.go
package ssl

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/providers/dns"
)

// DNSProvider DNS 挑战提供者
type DNSProvider struct {
	provider dns01.Provider
	name     string
}

// NewDNSProvider 创建 DNS 提供者
func NewDNSProvider(name string) (dns01.Provider, error) {
	// 支持的 DNS 服务商列表
	supportedProviders := map[string]bool{
		"cloudflare":    true,
		"aliyun":        true,
		"tencentcloud":  true,
		"aws":           true,
		"godaddy":       true,
		"namecheap":     true,
		"manual":        true,
	}

	if !supportedProviders[strings.ToLower(name)] {
		return nil, fmt.Errorf("unsupported DNS provider: %s", name)
	}

	// 使用 LEGO 的 DNS 提供者
	return dns.NewDNSProvider(name)
}

// DNSConfig DNS 配置
type DNSConfig struct {
	Provider    string            `yaml:"provider" json:"provider"`       // DNS 服务商
	Credentials map[string]string `yaml:"credentials" json:"credentials"` // API 凭证
}

// 示例：在配置文件中
/*
ssl:
  dns:
    provider: cloudflare
    credentials:
      CLOUDFLARE_DNS_API_TOKEN: "your-api-token"
      CLOUDFLARE_ENVIRONMENT: production
*/
```

---

### 2.4 证书自动续签增强

```go
// internal/ssl/auto_renew.go
package ssl

import (
	"context"
	"fmt"
	"time"

	"prerender-shield/internal/logging"
)

// AutoRenewConfig 自动续签配置
type AutoRenewConfig struct {
	Enabled         bool          `yaml:"enabled" json:"enabled"`
	CheckInterval   time.Duration `yaml:"check_interval" json:"check_interval"`     // 检查间隔
	RenewBeforeDays int           `yaml:"renew_before_days" json:"renew_before_days"` // 提前多少天续签
	MaxRetries      int           `yaml:"max_retries" json:"max_retries"`           // 最大重试次数
	RetryDelay      time.Duration `yaml:"retry_delay" json:"retry_delay"`           // 重试间隔
	WebhookURL      string        `yaml:"webhook_url" json:"webhook_url"`           // 通知 Webhook
}

// AutoRenewer 自动续签器
type AutoRenewer struct {
	acmeClient *ACMEClient
	config     AutoRenewConfig
	redisClient *redis.Client
	cancel     context.CancelFunc
	ctx        context.Context
}

// NewAutoRenewer 创建自动续签器
func NewAutoRenewer(acmeClient *ACMEClient, redisClient *redis.Client, config AutoRenewConfig) *AutoRenewer {
	return &AutoRenewer{
		acmeClient:  acmeClient,
		redisClient: redisClient,
		config:      config,
	}
}

// Start 启动自动续签
func (r *AutoRenewer) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	r.ctx = ctx
	r.cancel = cancel

	go r.run()
}

// Stop 停止自动续签
func (r *AutoRenewer) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
}

func (r *AutoRenewer) run() {
	ticker := time.NewTicker(r.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.checkAndRenew()
		}
	}
}

func (r *AutoRenewer) checkAndRenew() {
	// 获取所有证书
	certDomains, err := r.redisClient.SetMembers("ssl:certs")
	if err != nil {
		logging.DefaultLogger.Error("Failed to get cert domains: %v", err)
		return
	}

	for _, domain := range certDomains {
		r.renewCertificate(domain)
	}
}

func (r *AutoRenewer) renewCertificate(domain string) {
	// 获取证书信息
	certInfo := make(map[string]interface{})
	if err := r.redisClient.GetJSON(fmt.Sprintf("ssl:cert:%s", domain), &certInfo); err != nil {
		return
	}

	// 检查过期时间
	expiresAt, ok := certInfo["expires_at"].(float64)
	if !ok {
		return
	}

	expiryTime := time.Unix(int64(expiresAt), 0)
	remainingDays := int(time.Until(expiryTime).Hours() / 24)

	// 不需要续签
	if remainingDays > r.config.RenewBeforeDays {
		return
	}

	logging.DefaultLogger.Info("Certificate %s expires in %d days, starting renewal", domain, remainingDays)

	// 尝试续签（带重试）
	var err error
	for attempt := 0; attempt < r.config.MaxRetries; attempt++ {
		err = r.doRenew(domain, certInfo)
		if err == nil {
			logging.DefaultLogger.Info("Certificate %s renewed successfully", domain)

			// 发送 webhook 通知
			if r.config.WebhookURL != "" {
				sendWebhook(r.config.WebhookURL, "renewal_success", domain)
			}
			return
		}

		logging.DefaultLogger.Warn("Renewal attempt %d failed: %v", attempt+1, err)
		time.Sleep(r.config.RetryDelay)
	}

	// 所有重试失败
	logging.DefaultLogger.Error("Failed to renew certificate %s after %d attempts: %v", domain, r.config.MaxRetries, err)

	if r.config.WebhookURL != "" {
		sendWebhook(r.config.WebhookURL, "renewal_failed", domain)
	}
}

func (r *AutoRenewer) doRenew(domain string, certInfo map[string]interface{}) error {
	// 加载现有证书
	certPath := certInfo["cert_path"].(string)
	keyPath := certInfo["key_path"].(string)

	cert, err := certificate.NewCertFromFile(certPath, keyPath)
	if err != nil {
		return fmt.Errorf("failed to load certificate: %w", err)
	}

	// 续签
	newCert, err := r.acmeClient.RenewCertificate(domain, cert)
	if err != nil {
		return err
	}

	// 更新证书信息
	certInfo["updated_at"] = time.Now().Unix()
	certInfo["expires_at"] = getCertExpiry(newCert.Certificate).Unix()
	return r.redisClient.SaveJSON(fmt.Sprintf("ssl:cert:%s", domain), certInfo, 0)
}

func getCertExpiry(certDER []byte) time.Time {
	certs, err := parseCertsFromPEM(certDER)
	if err != nil {
		return time.Now()
	}
	return certs[0].NotAfter
}

func sendWebhook(url, event, domain string) {
	// 发送 webhook 通知
	payload := map[string]interface{}{
		"event":     event,
		"domain":    domain,
		"timestamp": time.Now().Unix(),
	}
	// HTTP POST 实现...
}
```

---

### 2.5 站点 SSL 配置增强

```go
// internal/config/config.go - 新增 SSL 配置

// SSLConfig SSL 证书配置
type SSLConfig struct {
	Enabled       bool          `yaml:"enabled" json:"enabled"`               // 是否启用 SSL
	AutoRenew     bool          `yaml:"auto_renew" json:"auto_renew"`         // 自动续签
	Email         string        `yaml:"email" json:"email"`                   // ACME 账户邮箱
	Provider      string        `yaml:"provider" json:"provider"`             // 证书提供商：letsencrypt, zerossl, manual
	KeyType       string        `yaml:"key_type" json:"key_type"`             // 密钥类型：rsa2048, rsa4096, ec256, ec384
	PreferredChain string       `yaml:"preferred_chain" json:"preferred_chain"` // 首选证书链
	DNS           DNSConfig     `yaml:"dns" json:"dns"`                       // DNS 配置（用于 DNS-01 挑战）
	CertFile      string        `yaml:"cert_file" json:"cert_file"`           // 证书文件路径（手动模式）
	KeyFile       string        `yaml:"key_file" json:"key_file"`             // 私钥文件路径（手动模式）
}

// SiteConfig 新增 SSL 字段
type SiteConfig struct {
	// ... 现有字段 ...
	SSL SSLConfig `yaml:"ssl" json:"ssl"`
}

// 配置文件示例
/*
sites:
  - id: example-com
    name: Example Site
    domains:
      - example.com
      - www.example.com
    port: 443
    ssl:
      enabled: true
      auto_renew: true
      email: admin@example.com
      provider: letsencrypt
      key_type: ec256
      dns:
        provider: cloudflare
        credentials:
          CLOUDFLARE_DNS_API_TOKEN: "xxx"
*/
```

---

### 2.6 SSL 管理 API 控制器

```go
// internal/api/controllers/ssl_controller.go
package controllers

import (
	"net/http"
	"prerender-shield/internal/ssl"

	"github.com/gin-gonic/gin"
)

// SSLController SSL 控制器
type SSLController struct {
	sslManager *ssl.Manager
}

// NewSSLController 创建 SSL 控制器
func NewSSLController(sslManager *ssl.Manager) *SSLController {
	return &SSLController{sslManager: sslManager}
}

// RequestCert 申请证书
// POST /api/v1/ssl/certificates
// Body: { "domain": "example.com", "sans": ["www.example.com"] }
func (c *SSLController) RequestCert(ctx *gin.Context) {
	var req struct {
		Domain string   `json:"domain" binding:"required"`
		SANs   []string `json:"sans"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	domains := append([]string{req.Domain}, req.SANs...)

	if err := c.sslManager.RequestCertificate(domains); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Certificate requested successfully",
		"domain":  req.Domain,
	})
}

// RenewCert 续签证书
// POST /api/v1/ssl/certificates/:domain/renew
func (c *SSLController) RenewCert(ctx *gin.Context) {
	domain := ctx.Param("domain")

	if err := c.sslManager.RenewCertificate(domain); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Certificate renewed successfully",
		"domain":  domain,
	})
}

// GetCertStatus 获取证书状态
// GET /api/v1/ssl/certificates/:domain
func (c *SSLController) GetCertStatus(ctx *gin.Context) {
	domain := ctx.Param("domain")

	status, err := c.sslManager.GetCertificateStatus(domain)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"domain": domain,
		"status": status,
	})
}

// ListCerts 列出所有证书
// GET /api/v1/ssl/certificates
func (c *SSLController) ListCerts(ctx *gin.Context) {
	certs, err := c.sslManager.ListCertificates()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"certificates": certs,
	})
}

// DeleteCert 删除证书
// DELETE /api/v1/ssl/certificates/:domain
func (c *SSLController) DeleteCert(ctx *gin.Context) {
	domain := ctx.Param("domain")

	if err := c.sslManager.DeleteCertificate(domain); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Certificate deleted successfully",
		"domain":  domain,
	})
}

// GetExpiringCerts 获取即将过期的证书
// GET /api/v1/ssl/certificates/expiring
func (c *SSLController) GetExpiringCerts(ctx *gin.Context) {
	days := ctx.DefaultQuery("days", "30")

	expiring, err := c.sslManager.CheckExpiration()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"expiring_in_days": days,
		"domains":          expiring,
	})
}
```

---

## 三、配置文件示例

### 3.1 完整 SSL 配置示例

```yaml
# configs/config.yml

server:
  address: "0.0.0.0"
  api_port: 9598
  console_port: 9597

ssl:
  # 全局 SSL 配置
  enabled: true
  auto_renew: true
  check_interval: 24h
  renew_before_days: 30
  email: admin@example.com
  provider: letsencrypt  # letsencrypt, zerossl, manual
  production: false  # false = 使用 Let's Encrypt 测试环境

  # DNS 挑战配置（用于通配符证书）
  dns:
    provider: cloudflare  # cloudflare, aliyun, tencentcloud, aws
    credentials:
      CLOUDFLARE_DNS_API_TOKEN: "your-api-token"
      CLOUDFLARE_ENVIRONMENT: production

  # HTTP 挑战配置
  http:
    port: 80  # HTTP-01 挑战监听端口

  # Webhook 通知配置
  webhook:
    url: "http://localhost:9597/api/v1/webhook/ssl"
    events: ["renewal_success", "renewal_failed", "expiring_soon"]

sites:
  # 站点 1 - 自动申请证书
  - id: example-com
    name: Example Site
    domains:
      - example.com
      - www.example.com
    port: 443
    ssl:
      enabled: true
      auto_renew: true
      email: admin@example.com
      provider: letsencrypt

  # 站点 2 - 通配符证书（需要 DNS 挑战）
  - id: example-org
    name: Example Org
    domains:
      - "*.example.org"
    port: 443
    ssl:
      enabled: true
      auto_renew: true
      provider: letsencrypt
      dns:
        provider: cloudflare
        credentials:
          CLOUDFLARE_DNS_API_TOKEN: "xxx"

  # 站点 3 - 手动证书
  - id: manual-site
    name: Manual SSL Site
    domains:
      - manual.example.com
    port: 443
    ssl:
      enabled: true
      cert_file: "/path/to/cert.pem"
      key_file: "/path/to/key.pem"
```

---

## 四、依赖更新

### 4.1 go.mod 新增依赖

```go
require (
	github.com/go-acme/lego/v4 v4.15.0  // ACME/Let's Encrypt 客户端
)
```

### 4.2 安装命令

```bash
go get github.com/go-acme/lego/v4
```

---

## 五、实施步骤

### Phase 1: 核心 ACME 功能 (8 小时)
- [ ] 实现 `ACMEClient` 完整功能
- [ ] 实现 HTTP-01 挑战服务器
- [ ] 集成 LEGO DNS 提供者
- [ ] 测试证书申请流程

### Phase 2: 自动续签 (4 小时)
- [ ] 实现 `AutoRenewer`
- [ ] 添加重试机制
- [ ] 实现 Webhook 通知

### Phase 3: API 和管理界面 (4 小时)
- [ ] 实现 SSL 控制器 API
- [ ] 前端管理界面集成
- [ ] 证书状态展示

### Phase 4: 测试和文档 (4 小时)
- [ ] 单元测试
- [ ] 集成测试（使用 Let's Encrypt Staging）
- [ ] 文档更新

**总工时:** 20 小时

---

## 六、测试命令

```bash
# 运行 SSL 模块测试
go test -v ./internal/ssl/...

# 运行集成测试（使用 Staging 环境）
SSL_TEST_STAGING=true go test -v ./internal/ssl/integration_test.go

# 测试证书申请
curl -X POST http://localhost:9598/api/v1/ssl/certificates \
  -H "Content-Type: application/json" \
  -d '{"domain": "test.example.com"}'

# 查看证书状态
curl http://localhost:9598/api/v1/ssl/certificates/test.example.com
```

---

## 七、安全注意事项

1. **生产环境前使用 Staging** - 先用 Let's Encrypt Staging 环境测试
2. **保护私钥** - 私钥文件权限设为 0600
3. **API 认证** - SSL 管理 API 需要管理员认证
4. **DNS 凭证安全** - DNS API 凭证存储在环境变量或加密配置中
5. **速率限制** - 遵循 Let's Encrypt 的速率限制

---

## 八、常见问题

### Q1: 申请失败怎么办？
A: 检查以下几点：
- 域名 DNS 解析是否指向服务器
- 80 端口是否可访问（HTTP-01）
- DNS API 凭证是否正确（DNS-01）
- 查看日志：`tail -f logs/ssl.log`

### Q2: 通配符证书如何申请？
A: 必须使用 DNS-01 挑战：
```yaml
ssl:
  provider: letsencrypt
  dns:
    provider: cloudflare
```

### Q3: 证书续签失败？
A: 检查：
- 自动续签服务是否运行
- Redis 是否可连接
- 网络是否可访问 ACME 服务器

---

**文档维护:** 实施过程中更新
**最后更新:** 2026-03-11
