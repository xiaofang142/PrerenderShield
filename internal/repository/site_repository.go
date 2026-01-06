package repository

import (
	"encoding/json"
	"fmt"
	"time"

	"prerender-shield/internal/models"
	redisPkg "prerender-shield/internal/redis"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

type SiteRepository struct {
	client *redisPkg.Client
}

func NewSiteRepository(client *redisPkg.Client) *SiteRepository {
	return &SiteRepository{
		client: client,
	}
}

func (r *SiteRepository) GetSiteByID(id string) (*models.Site, error) {
	ctx := r.client.Context()
	key := fmt.Sprintf("site:%s", id)

	data, err := r.client.GetRawClient().Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var site models.Site
	if err := json.Unmarshal([]byte(data), &site); err != nil {
		return nil, err
	}

	return &site, nil
}

func (r *SiteRepository) CreateSite(site *models.Site) error {
	if site.ID == "" {
		site.ID = uuid.New().String()
	}
	now := time.Now()
	if site.CreatedAt.IsZero() {
		site.CreatedAt = now
	}
	site.UpdatedAt = now

	return r.saveSite(site)
}

func (r *SiteRepository) UpdateSite(site *models.Site) error {
	site.UpdatedAt = time.Now()
	return r.saveSite(site)
}

func (r *SiteRepository) saveSite(site *models.Site) error {
	ctx := r.client.Context()
	key := fmt.Sprintf("site:%s", site.ID)

	data, err := json.Marshal(site)
	if err != nil {
		return err
	}

	return r.client.GetRawClient().Set(ctx, key, data, 0).Err()
}

// SyncSite ensures the site exists in the database (upsert)
func (r *SiteRepository) SyncSite(site *models.Site) error {
	existing, err := r.GetSiteByID(site.ID)
	if err != nil {
		return err
	}

	if existing == nil {
		return r.CreateSite(site)
	}

	// Update fields
	existing.Name = site.Name
	existing.Domain = site.Domain
	existing.UpdatedAt = time.Now()

	return r.saveSite(existing)
}
