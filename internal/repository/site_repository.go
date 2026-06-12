package repository

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"prerender-shield/internal/models"
	"prerender-shield/internal/redis"
)

// RedisClient Redis 客户端接口
type RedisClient interface {
	HashSetAll(key string, values map[string]interface{}) error
	Set(key string, value interface{}, expiration time.Duration) error
	SetAdd(key string, members ...interface{}) error
	HashGetAll(key string) (map[string]string, error)
	Del(key string) error
	SetRemove(key string, members ...interface{}) error
	DeleteSiteData(siteID string) error
	Get(key string) (string, error)
	SetMembers(key string) ([]string, error)
}

// 确保 redis.Client 实现 RedisClient 接口
var _ RedisClient = (*redis.Client)(nil)

// SiteRepository 站点存储库接口
type SiteRepository interface {
	Create(site *models.Site) error
	Get(id string) (*models.Site, error)
	Update(site *models.Site) error
	Delete(id string) error
	List() ([]*models.Site, error)
	GetByDomain(domain string) (*models.Site, error)
}

// siteRepository 站点存储库实现
type siteRepository struct {
	redisClient RedisClient
}

// NewSiteRepository 创建新的站点存储库
func NewSiteRepository(redisClient RedisClient) SiteRepository {
	return &siteRepository{
		redisClient: redisClient,
	}
}

// Create 创建新站点
func (r *siteRepository) Create(site *models.Site) error {
	if site.ID == "" {
		site.ID = uuid.New().String()
	}

	now := time.Now()
	if site.CreatedAt.IsZero() {
		site.CreatedAt = now
	}
	site.UpdatedAt = now

	// 存储站点基本信息
	siteKey := fmt.Sprintf("site:%s", site.ID)
	siteData := map[string]interface{}{
		"id":         site.ID,
		"domain":     site.Domain,
		"name":       site.Name,
		"enabled":    site.EnabledInt(),
		"created_at": site.CreatedAt.Unix(),
		"updated_at": site.UpdatedAt.Unix(),
	}

	if err := r.redisClient.HashSetAll(siteKey, siteData); err != nil {
		return fmt.Errorf("failed to save site: %w", err)
	}

	// 存储域名到站点ID的映射
	domainKey := fmt.Sprintf("domain:%s", site.Domain)
	if err := r.redisClient.Set(domainKey, site.ID, 0); err != nil {
		return fmt.Errorf("failed to save domain mapping: %w", err)
	}

	// 将站点ID添加到站点列表
	if err := r.redisClient.SetAdd("sites", site.ID); err != nil {
		return fmt.Errorf("failed to add site to list: %w", err)
	}

	return nil
}

// Get 获取站点信息
func (r *siteRepository) Get(id string) (*models.Site, error) {
	siteKey := fmt.Sprintf("site:%s", id)
	siteData, err := r.redisClient.HashGetAll(siteKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get site: %w", err)
	}

	if len(siteData) == 0 {
		return nil, nil
	}

	site := &models.Site{
		ID:        siteData["id"],
		Domain:    siteData["domain"],
		Name:      siteData["name"],
		Enabled:   siteData["enabled"] == "1",
		CreatedAt: time.Unix(parseInt64(siteData["created_at"]), 0),
		UpdatedAt: time.Unix(parseInt64(siteData["updated_at"]), 0),
	}

	return site, nil
}

// Update 更新站点信息
func (r *siteRepository) Update(site *models.Site) error {
	// 先检查站点是否存在
	existingSite, err := r.Get(site.ID)
	if err != nil {
		return fmt.Errorf("failed to get site: %w", err)
	}
	if existingSite == nil {
		return fmt.Errorf("site not found")
	}

	site.UpdatedAt = time.Now()

	siteKey := fmt.Sprintf("site:%s", site.ID)
	siteData := map[string]interface{}{
		"domain":     site.Domain,
		"name":       site.Name,
		"enabled":    site.EnabledInt(),
		"updated_at": site.UpdatedAt.Unix(),
	}

	if err := r.redisClient.HashSetAll(siteKey, siteData); err != nil {
		return fmt.Errorf("failed to update site: %w", err)
	}

	// 更新域名到站点ID的映射
	domainKey := fmt.Sprintf("domain:%s", site.Domain)
	if err := r.redisClient.Set(domainKey, site.ID, 0); err != nil {
		return fmt.Errorf("failed to update domain mapping: %w", err)
	}

	return nil
}

// Delete 删除站点
func (r *siteRepository) Delete(id string) error {
	// 获取站点信息以删除域名映射
	site, err := r.Get(id)
	if err != nil {
		return fmt.Errorf("failed to get site: %w", err)
	}

	if site != nil {
		// 删除域名到站点ID的映射
		domainKey := fmt.Sprintf("domain:%s", site.Domain)
		if err := r.redisClient.Del(domainKey); err != nil {
			return fmt.Errorf("failed to delete domain mapping: %w", err)
		}
	}

	// 从站点列表中移除
	if err := r.redisClient.Del(fmt.Sprintf("site:%s", id)); err != nil {
		return fmt.Errorf("failed to delete site: %w", err)
	}

	// 从站点集合中移除
	if err := r.redisClient.SetRemove("sites", id); err != nil {
		return fmt.Errorf("failed to remove site from list: %w", err)
	}

	// 删除站点相关数据
	if err := r.redisClient.DeleteSiteData(id); err != nil {
		return fmt.Errorf("failed to delete site data: %w", err)
	}

	return nil
}

// List 获取所有站点
func (r *siteRepository) List() ([]*models.Site, error) {
	siteIDs, err := r.redisClient.SetMembers("sites")
	if err != nil {
		return nil, fmt.Errorf("failed to get site list: %w", err)
	}

	sites := make([]*models.Site, 0, len(siteIDs))
	for _, id := range siteIDs {
		site, err := r.Get(id)
		if err != nil {
			continue
		}
		if site != nil {
			sites = append(sites, site)
		}
	}

	return sites, nil
}

// GetByDomain 根据域名获取站点
func (r *siteRepository) GetByDomain(domain string) (*models.Site, error) {
	domainKey := fmt.Sprintf("domain:%s", domain)
	siteID, err := r.redisClient.Get(domainKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get site ID by domain: %w", err)
	}

	if siteID == "" {
		return nil, nil
	}

	return r.Get(siteID)
}

// parseInt64 将字符串转换为int64
func parseInt64(s string) int64 {
	var i int64
	if _, err := fmt.Sscanf(s, "%d", &i); err != nil {
		return 0
	}
	return i
}
