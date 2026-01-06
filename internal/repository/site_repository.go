package repository

import (
	"errors"
	"prerender-shield/internal/db"
	"prerender-shield/internal/models"

	"gorm.io/gorm"
)

type SiteRepository struct {
	db *gorm.DB
}

func NewSiteRepository() *SiteRepository {
	return &SiteRepository{
		db: db.GetDB(),
	}
}

func (r *SiteRepository) GetSiteByID(id string) (*models.Site, error) {
	var site models.Site
	err := r.db.Where("id = ?", id).First(&site).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &site, err
}

func (r *SiteRepository) CreateSite(site *models.Site) error {
	return r.db.Create(site).Error
}

func (r *SiteRepository) UpdateSite(site *models.Site) error {
	return r.db.Save(site).Error
}

// SyncSite ensures the site exists in the database (upsert)
func (r *SiteRepository) SyncSite(site *models.Site) error {
	var existing models.Site
	err := r.db.Where("id = ?", site.ID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.CreateSite(site)
	} else if err != nil {
		return err
	}
	// Update fields if needed
	existing.Name = site.Name
	existing.Domain = site.Domain
	return r.db.Save(&existing).Error
}
