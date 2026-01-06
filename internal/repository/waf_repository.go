package repository

import (
	"errors"
	"prerender-shield/internal/db"
	"prerender-shield/internal/models"

	"gorm.io/gorm"
)

// WafRepository handles WAF related database operations
type WafRepository struct {
	db *gorm.DB
}

// NewWafRepository creates a new WafRepository
func NewWafRepository() *WafRepository {
	return &WafRepository{
		db: db.GetDB(),
	}
}

// GetWafConfigBySiteID retrieves the WAF configuration for a specific site
func (r *WafRepository) GetWafConfigBySiteID(siteID string) (*models.WafConfig, error) {
	var config models.WafConfig
	err := r.db.Preload("BlockedCountries").
		Preload("IPWhitelist").
		Preload("IPBlacklist").
		Where("site_id = ?", siteID).
		First(&config).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// If not found, create a default one? Or return error?
		// Usually return nil or error. Let's return error for now.
		return nil, nil
	}
	return &config, err
}

// CreateWafConfig creates a new WAF configuration
func (r *WafRepository) CreateWafConfig(config *models.WafConfig) error {
	return r.db.Create(config).Error
}

// UpdateWafConfig updates an existing WAF configuration
func (r *WafRepository) UpdateWafConfig(config *models.WafConfig) error {
	return r.db.Save(config).Error
}

// UpdateBlockedCountries replaces the list of blocked countries
func (r *WafRepository) UpdateBlockedCountries(wafConfigID string, countries []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Delete existing
		if err := tx.Where("waf_config_id = ?", wafConfigID).Delete(&models.BlockedCountry{}).Error; err != nil {
			return err
		}

		// Add new
		for _, code := range countries {
			bc := models.BlockedCountry{
				WafConfigID: wafConfigID,
				CountryCode: code,
			}
			if err := tx.Create(&bc).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateIPWhitelist replaces the IP whitelist
func (r *WafRepository) UpdateIPWhitelist(wafConfigID string, ips []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("waf_config_id = ?", wafConfigID).Delete(&models.IPWhitelist{}).Error; err != nil {
			return err
		}
		for _, ip := range ips {
			wl := models.IPWhitelist{
				WafConfigID: wafConfigID,
				IPAddress:   ip,
			}
			if err := tx.Create(&wl).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateIPBlacklist replaces the IP blacklist
func (r *WafRepository) UpdateIPBlacklist(wafConfigID string, ips []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Note: We might want to preserve reasons, but for simple list update this is fine.
		// If the input is just strings, we lose 'reason'.
		// The API spec in architecture.md says `blacklist_ips: string[]`, so we lose reasons in that specific API.
		if err := tx.Where("waf_config_id = ?", wafConfigID).Delete(&models.IPBlacklist{}).Error; err != nil {
			return err
		}
		for _, ip := range ips {
			bl := models.IPBlacklist{
				WafConfigID: wafConfigID,
				IPAddress:   ip,
			}
			if err := tx.Create(&bl).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetAccessLogs retrieves access logs with pagination and filters
func (r *WafRepository) GetAccessLogs(siteID string, page, limit int) ([]models.AccessLog, int64, error) {
	var logs []models.AccessLog
	var total int64

	query := r.db.Model(&models.AccessLog{}).Where("site_id = ?", siteID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.Order("created_at desc").Limit(limit).Offset(offset).Find(&logs).Error
	return logs, total, err
}

// CreateAccessLog creates a new access log entry
func (r *WafRepository) CreateAccessLog(log *models.AccessLog) error {
	return r.db.Create(log).Error
}

// WafStats represents aggregated WAF statistics
type WafStats struct {
	TotalRequests   int64 `json:"total_requests"`
	BlockedRequests int64 `json:"blocked_requests"`
	AttackRequests  int64 `json:"attack_requests"`
}

// GetGlobalStats returns global WAF statistics for a given duration
func (r *WafRepository) GetGlobalStats(startTime, endTime string) (*WafStats, error) {
	var stats WafStats

	// Total Requests
	if err := r.db.Model(&models.AccessLog{}).
		Where("created_at BETWEEN ? AND ?", startTime, endTime).
		Count(&stats.TotalRequests).Error; err != nil {
		return nil, err
	}

	// Blocked Requests (Action = 'block')
	if err := r.db.Model(&models.AccessLog{}).
		Where("created_at BETWEEN ? AND ?", startTime, endTime).
		Where("action = ?", "block").
		Count(&stats.BlockedRequests).Error; err != nil {
		return nil, err
	}

	// Attack Requests (Assuming same as blocked for now)
	stats.AttackRequests = stats.BlockedRequests

	return &stats, nil
}

// GetTrafficStats returns traffic statistics grouped by time
func (r *WafRepository) GetTrafficStats(startTime, endTime string) ([]map[string]interface{}, error) {
	// Group by hour for the last 24 hours, or appropriate interval
	// For simplicity, let's just return a list of counts per hour
	// This requires DB specific SQL (Postgres uses date_trunc)

	type Result struct {
		TimeBucket string `gorm:"column:time_bucket"`
		Count      int64  `gorm:"column:count"`
		Blocked    int64  `gorm:"column:blocked"`
	}

	var results []Result

	// Postgres query
	err := r.db.Raw(`
		SELECT 
			date_trunc('hour', created_at) as time_bucket,
			COUNT(*) as count,
			COUNT(CASE WHEN action = 'block' THEN 1 END) as blocked
		FROM access_logs 
		WHERE created_at BETWEEN ? AND ?
		GROUP BY time_bucket
		ORDER BY time_bucket
	`, startTime, endTime).Scan(&results).Error

	if err != nil {
		return nil, err
	}

	var data []map[string]interface{}
	for _, res := range results {
		data = append(data, map[string]interface{}{
			"time":            res.TimeBucket,
			"totalRequests":   res.Count,
			"blockedRequests": res.Blocked,
		})
	}
	return data, nil
}
