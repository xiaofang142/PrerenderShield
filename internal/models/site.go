package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Site represents a website managed by the system
type Site struct {
	ID        string         `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Domain    string         `gorm:"type:varchar(255);unique;not null" json:"domain"`
	Name      string         `gorm:"type:varchar(100);not null" json:"name"`
	Enabled   bool           `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	WafConfig  *WafConfig  `json:"waf_config,omitempty"`
	AccessLogs []AccessLog `json:"-"`
}

// BeforeCreate hooks into GORM to set UUID
func (s *Site) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return
}
