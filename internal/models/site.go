package models

import (
	"time"
)

// Site represents a website managed by the system
type Site struct {
	ID        string    `json:"id"`
	Domain    string    `json:"domain"`
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	WafConfig  *WafConfig  `json:"waf_config,omitempty"`
	AccessLogs []AccessLog `json:"-"`
}

// EnabledInt returns 1 if Enabled is true, 0 otherwise
func (s *Site) EnabledInt() int {
	if s.Enabled {
		return 1
	}
	return 0
}
