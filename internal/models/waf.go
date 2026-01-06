package models

import (
	"time"
)

// WafConfig represents the firewall configuration for a site
type WafConfig struct {
	ID              string    `json:"id"`
	SiteID          string    `json:"site_id"`
	RateLimitCount  int       `json:"rate_limit_count"`
	RateLimitWindow int       `json:"rate_limit_window"` // in minutes
	CustomBlockPage string    `json:"custom_block_page"`
	Enabled         bool      `json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	// Relations
	BlockedCountries []BlockedCountry `json:"blocked_countries,omitempty"`
	IPWhitelist      []IPWhitelist    `json:"ip_whitelist,omitempty"`
	IPBlacklist      []IPBlacklist    `json:"ip_blacklist,omitempty"`
}

// BlockedCountry represents a country blocked by WAF
type BlockedCountry struct {
	ID          string `json:"id"`
	WafConfigID string `json:"waf_config_id"`
	CountryCode string `json:"country_code"`
}

// IPWhitelist represents allowed IPs
type IPWhitelist struct {
	ID          string `json:"id"`
	WafConfigID string `json:"waf_config_id"`
	IPAddress   string `json:"ip_address"`
}

// IPBlacklist represents blocked IPs
type IPBlacklist struct {
	ID          string    `json:"id"`
	WafConfigID string    `json:"waf_config_id"`
	IPAddress   string    `json:"ip_address"`
	Reason      string    `json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
}

// AccessLog represents a request log
type AccessLog struct {
	ID          string    `json:"id"`
	SiteID      string    `json:"site_id"`
	RequestID   string    `json:"request_id"`
	IPAddress   string    `json:"ip_address"`
	Country     string    `json:"country"`
	City        string    `json:"city"`
	Method      string    `json:"method"`
	UserAgent   string    `json:"user_agent"`
	RequestPath string    `json:"request_path"`
	StatusCode  int       `json:"status_code"`
	Action      string    `json:"action"` // allow, block, captcha
	RuleID      string    `json:"rule_id"`
	Reason      string    `json:"reason"`
	IsCleaned   bool      `json:"is_cleaned"`
	CreatedAt   time.Time `json:"created_at"`
}
