package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WafConfig represents the firewall configuration for a site
type WafConfig struct {
	ID              string         `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SiteID          string         `gorm:"type:uuid;not null;index" json:"site_id"`
	RateLimitCount  int            `gorm:"default:100" json:"rate_limit_count"`
	RateLimitWindow int            `gorm:"default:5" json:"rate_limit_window"` // in minutes
	CustomBlockPage string         `gorm:"type:text" json:"custom_block_page"`
	Enabled         bool           `gorm:"default:true" json:"enabled"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	BlockedCountries []BlockedCountry `json:"blocked_countries,omitempty"`
	IPWhitelist      []IPWhitelist    `json:"ip_whitelist,omitempty"`
	IPBlacklist      []IPBlacklist    `json:"ip_blacklist,omitempty"`
}

// BeforeCreate hooks into GORM to set UUID
func (w *WafConfig) BeforeCreate(tx *gorm.DB) (err error) {
	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	return
}

// BlockedCountry represents a country blocked by WAF
type BlockedCountry struct {
	ID          string `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	WafConfigID string `gorm:"type:uuid;not null;index" json:"waf_config_id"`
	CountryCode string `gorm:"type:varchar(2);not null" json:"country_code"`
}

// BeforeCreate hooks into GORM to set UUID
func (b *BlockedCountry) BeforeCreate(tx *gorm.DB) (err error) {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return
}

// IPWhitelist represents allowed IPs
type IPWhitelist struct {
	ID          string `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	WafConfigID string `gorm:"type:uuid;not null;index" json:"waf_config_id"`
	IPAddress   string `gorm:"type:inet;not null" json:"ip_address"`
}

// BeforeCreate hooks into GORM to set UUID
func (i *IPWhitelist) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == "" {
		i.ID = uuid.New().String()
	}
	return
}

// IPBlacklist represents blocked IPs
type IPBlacklist struct {
	ID          string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	WafConfigID string    `gorm:"type:uuid;not null;index" json:"waf_config_id"`
	IPAddress   string    `gorm:"type:inet;not null" json:"ip_address"`
	Reason      string    `gorm:"type:varchar(255)" json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
}

// BeforeCreate hooks into GORM to set UUID
func (i *IPBlacklist) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == "" {
		i.ID = uuid.New().String()
	}
	return
}

// AccessLog represents a request log
type AccessLog struct {
	ID          string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SiteID      string    `gorm:"type:uuid;not null;index" json:"site_id"`
	RequestID   string    `gorm:"type:varchar(64);index" json:"request_id"`
	IPAddress   string    `gorm:"type:inet;not null;index" json:"ip_address"`
	Country     string    `gorm:"type:varchar(2)" json:"country"`
	City        string    `gorm:"type:varchar(100)" json:"city"`
	Method      string    `gorm:"type:varchar(10)" json:"method"`
	UserAgent   string    `gorm:"type:text" json:"user_agent"`
	RequestPath string    `gorm:"type:varchar(500)" json:"request_path"`
	StatusCode  int       `json:"status_code"`
	Action      string    `gorm:"type:varchar(20);index" json:"action"` // allow, block, captcha
	RuleID      string    `gorm:"type:varchar(50)" json:"rule_id"`
	Reason      string    `gorm:"type:text" json:"reason"`
	IsCleaned   bool      `gorm:"default:false;index" json:"is_cleaned"`
	CreatedAt   time.Time `gorm:"index:idx_access_logs_created_at,sort:desc" json:"created_at"`
}

// BeforeCreate hooks into GORM to set UUID
func (a *AccessLog) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return
}
