package crawler

import (
	"fmt"
	"net/http"
	"strings"

	"prerender-shield/internal/redis"
)

// Detector 爬虫检测器接口
type Detector interface {
	IsCrawler(r *http.Request) (bool, error)
	AddCrawlerUserAgent(pattern string) error
	RemoveCrawlerUserAgent(pattern string) error
	AddCrawlerIP(ip string) error
	RemoveCrawlerIP(ip string) error
	AddWhitelistIP(ip string) error
	RemoveWhitelistIP(ip string) error
	ListCrawlerUserAgents() ([]string, error)
	ListCrawlerIPs() ([]string, error)
	ListWhitelistIPs() ([]string, error)
}

// detector 爬虫检测器实现
type detector struct {
	redisClient *redis.Client
}

// NewDetector 创建新的爬虫检测器
func NewDetector(redisClient *redis.Client) Detector {
	return &detector{
		redisClient: redisClient,
	}
}

// IsCrawler 检测请求是否来自爬虫
func (d *detector) IsCrawler(r *http.Request) (bool, error) {
	// 获取客户端IP
	clientIP := d.getClientIP(r)

	// 检查是否在白名单中
	isWhitelisted, err := d.isWhitelistedIP(clientIP)
	if err != nil {
		return false, fmt.Errorf("failed to check whitelist: %w", err)
	}
	if isWhitelisted {
		return false, nil
	}

	// 检查是否在爬虫IP列表中
	isCrawlerIP, err := d.isCrawlerIP(clientIP)
	if err != nil {
		return false, fmt.Errorf("failed to check crawler IP: %w", err)
	}
	if isCrawlerIP {
		return true, nil
	}

	// 检查User-Agent
	userAgent := r.Header.Get("User-Agent")
	isCrawlerUA, err := d.isCrawlerUserAgent(userAgent)
	if err != nil {
		return false, fmt.Errorf("failed to check crawler User-Agent: %w", err)
	}
	if isCrawlerUA {
		return true, nil
	}

	return false, nil
}

// getClientIP 获取客户端IP
func (d *detector) getClientIP(r *http.Request) string {
	// 从X-Forwarded-For头获取
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// 从X-Real-IP头获取
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// 从RemoteAddr获取
	return strings.Split(r.RemoteAddr, ":")[0]
}

// isWhitelistedIP 检查IP是否在白名单中
func (d *detector) isWhitelistedIP(ip string) (bool, error) {
	return d.redisClient.SetContains("crawler:whitelist:ips", ip)
}

// isCrawlerIP 检查IP是否在爬虫IP列表中
func (d *detector) isCrawlerIP(ip string) (bool, error) {
	return d.redisClient.SetContains("crawler:ips", ip)
}

// isCrawlerUserAgent 检查User-Agent是否匹配爬虫规则
func (d *detector) isCrawlerUserAgent(userAgent string) (bool, error) {
	patterns, err := d.redisClient.SetMembers("crawler:user_agents")
	if err != nil {
		return false, err
	}

	for _, pattern := range patterns {
		if strings.Contains(strings.ToLower(userAgent), strings.ToLower(pattern)) {
			return true, nil
		}
	}

	return false, nil
}

// AddCrawlerUserAgent 添加爬虫User-Agent模式
func (d *detector) AddCrawlerUserAgent(pattern string) error {
	return d.redisClient.SetAdd("crawler:user_agents", pattern)
}

// RemoveCrawlerUserAgent 移除爬虫User-Agent模式
func (d *detector) RemoveCrawlerUserAgent(pattern string) error {
	return d.redisClient.SetRemove("crawler:user_agents", pattern)
}

// AddCrawlerIP 添加爬虫IP
func (d *detector) AddCrawlerIP(ip string) error {
	return d.redisClient.SetAdd("crawler:ips", ip)
}

// RemoveCrawlerIP 移除爬虫IP
func (d *detector) RemoveCrawlerIP(ip string) error {
	return d.redisClient.SetRemove("crawler:ips", ip)
}

// AddWhitelistIP 添加白名单IP
func (d *detector) AddWhitelistIP(ip string) error {
	return d.redisClient.SetAdd("crawler:whitelist:ips", ip)
}

// RemoveWhitelistIP 移除白名单IP
func (d *detector) RemoveWhitelistIP(ip string) error {
	return d.redisClient.SetRemove("crawler:whitelist:ips", ip)
}

// ListCrawlerUserAgents 列出所有爬虫User-Agent模式
func (d *detector) ListCrawlerUserAgents() ([]string, error) {
	return d.redisClient.SetMembers("crawler:user_agents")
}

// ListCrawlerIPs 列出所有爬虫IP
func (d *detector) ListCrawlerIPs() ([]string, error) {
	return d.redisClient.SetMembers("crawler:ips")
}

// ListWhitelistIPs 列出所有白名单IP
func (d *detector) ListWhitelistIPs() ([]string, error) {
	return d.redisClient.SetMembers("crawler:whitelist:ips")
}
