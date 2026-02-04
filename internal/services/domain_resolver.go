package services

import (
	"fmt"
	"strings"

	"prerender-shield/internal/redis"
)

// DomainResolver 域名解析器接口
type DomainResolver interface {
	Resolve(domain string) (string, error)
	AddMapping(domain, siteID string) error
	RemoveMapping(domain string) error
	ListMappings() (map[string]string, error)
}

// domainResolver 域名解析器实现
type domainResolver struct {
	redisClient *redis.Client
}

// NewDomainResolver 创建新的域名解析器
func NewDomainResolver(redisClient *redis.Client) DomainResolver {
	return &domainResolver{
		redisClient: redisClient,
	}
}

// Resolve 解析域名到站点ID
func (r *domainResolver) Resolve(domain string) (string, error) {
	// 首先尝试精确匹配
	siteID, err := r.redisClient.Get(fmt.Sprintf("domain:%s", domain))
	if err != nil {
		return "", fmt.Errorf("failed to resolve domain: %w", err)
	}

	if siteID != "" {
		return siteID, nil
	}

	// 尝试通配符匹配
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		subdomain := "*." + strings.Join(parts[i:], ".")
		siteID, err := r.redisClient.Get(fmt.Sprintf("domain:%s", subdomain))
		if err != nil {
			continue
		}
		if siteID != "" {
			return siteID, nil
		}
	}

	return "", nil
}

// AddMapping 添加域名到站点ID的映射
func (r *domainResolver) AddMapping(domain, siteID string) error {
	domainKey := fmt.Sprintf("domain:%s", domain)
	if err := r.redisClient.Set(domainKey, siteID, 0); err != nil {
		return fmt.Errorf("failed to add domain mapping: %w", err)
	}

	// 将域名添加到域名列表
	if err := r.redisClient.SetAdd("domains", domain); err != nil {
		return fmt.Errorf("failed to add domain to list: %w", err)
	}

	return nil
}

// RemoveMapping 移除域名到站点ID的映射
func (r *domainResolver) RemoveMapping(domain string) error {
	domainKey := fmt.Sprintf("domain:%s", domain)
	if err := r.redisClient.Del(domainKey); err != nil {
		return fmt.Errorf("failed to remove domain mapping: %w", err)
	}

	// 从域名列表中移除
	if err := r.redisClient.SetRemove("domains", domain); err != nil {
		return fmt.Errorf("failed to remove domain from list: %w", err)
	}

	return nil
}

// ListMappings 列出所有域名映射
func (r *domainResolver) ListMappings() (map[string]string, error) {
	domains, err := r.redisClient.SetMembers("domains")
	if err != nil {
		return nil, fmt.Errorf("failed to get domain list: %w", err)
	}

	mappings := make(map[string]string)
	for _, domain := range domains {
		siteID, err := r.redisClient.Get(fmt.Sprintf("domain:%s", domain))
		if err != nil {
			continue
		}
		if siteID != "" {
			mappings[domain] = siteID
		}
	}

	return mappings, nil
}
