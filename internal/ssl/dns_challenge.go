package ssl

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/providers/dns"
	"prerender-shield/internal/logging"
)

var (
	dnsEnvMu sync.Mutex
)

// DNSConfig DNS 配置
type DNSConfig struct {
	Provider    string            `yaml:"provider" json:"provider"`
	Credentials map[string]string `yaml:"credentials" json:"credentials"`
}

// NewDNSProvider 创建 DNS 提供者
func NewDNSProvider(name string, credentials map[string]string) (challenge.Provider, error) {
	// 支持的 DNS 服务商列表
	supportedProviders := map[string]bool{
		"cloudflare":   true,
		"aliyun":       true,
		"tencentcloud": true,
		"aws":          true,
		"godaddy":      true,
		"namecheap":    true,
		"manual":       true,
	}

	if !supportedProviders[strings.ToLower(name)] {
		return nil, fmt.Errorf("unsupported DNS provider: %s (supported: cloudflare, aliyun, tencentcloud, aws, godaddy, namecheap, manual)", name)
	}

	// 线程安全地设置环境变量，并保存旧值用于恢复
	dnsEnvMu.Lock()
	oldEnv := make(map[string]string, len(credentials))
	for key, value := range credentials {
		if oldVal, existed := lookupEnv(key); existed {
			oldEnv[key] = oldVal
		}
		os.Setenv(key, value)
	}
	dnsEnvMu.Unlock()

	// 使用 LEGO 的 DNS 提供者
	provider, err := dns.NewDNSChallengeProviderByName(name)
	if err != nil {
		// 恢复环境变量
		dnsEnvMu.Lock()
		for key := range credentials {
			if oldVal, ok := oldEnv[key]; ok {
				os.Setenv(key, oldVal)
			} else {
				os.Unsetenv(key)
			}
		}
		dnsEnvMu.Unlock()
		return nil, fmt.Errorf("failed to create DNS provider: %w", err)
	}

	logging.DefaultLogger.Info("DNS provider '%s' initialized successfully", name)

	return provider, nil
}

// lookupEnv 查找环境变量，os.LookupEnv is available in Go 1.x
func lookupEnv(key string) (string, bool) {
	val := os.Getenv(key)
	return val, val != ""
}

// SetDNSProvider 为 ACME 客户端设置 DNS 提供者
func (c *ACMEClient) SetDNSProvider(name string, credentials map[string]string) error {
	provider, err := NewDNSProvider(name, credentials)
	if err != nil {
		return err
	}

	// 配置 DNS-01 挑战
	err = c.client.Challenge.SetDNS01Provider(
		provider,
		dns01.AddDNSTimeout(10*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to set DNS01 provider: %w", err)
	}

	logging.DefaultLogger.Info("DNS-01 challenge enabled for provider: %s", name)

	return nil
}

// RequestWildcardCertificate 申请通配符证书
func (c *ACMEClient) RequestWildcardCertificate(baseDomain string, subdomains []string) (*certificate.Resource, error) {
	// 构建域名列表（包含通配符）
	domains := []string{fmt.Sprintf("*.%s", baseDomain)}
	if baseDomain != "" {
		domains = append(domains, baseDomain) // 通常也包含根域名
	}
	domains = append(domains, subdomains...)

	logging.DefaultLogger.Info("Requesting wildcard certificate for: %s", baseDomain)

	// 通配符证书必须使用 DNS-01 挑战
	request := certificate.ObtainRequest{
		Domains: domains,
		Bundle:  true,
	}

	cert, err := c.client.Certificate.Obtain(request)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain wildcard certificate: %w", err)
	}

	logging.DefaultLogger.Info("Wildcard certificate obtained for: %s", baseDomain)

	// 保存证书
	err = c.saveCertificate(baseDomain, cert)
	if err != nil {
		return nil, fmt.Errorf("failed to save wildcard certificate: %w", err)
	}

	return cert, nil
}
