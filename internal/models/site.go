package models

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

const (
	RenderModeProxy    = "proxy"
	RenderModeStatic   = "static"
	RenderModeRedirect = "redirect"
)

var (
	ErrDomainEmpty          = errors.New("domain is required")
	ErrModeInvalid          = errors.New("invalid render mode")
	ErrTargetURLEmpty       = errors.New("target_url is required for proxy/redirect mode")
	ErrPortInvalid          = errors.New("port must be between 1 and 65535")
	ErrPatternInvalid       = errors.New("invalid regex pattern")
	ErrWAFRuleActionInvalid = errors.New("invalid WAF rule action")
	ErrWAFRuleNameEmpty     = errors.New("rule name is required")
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

// SiteConfig represents the site configuration
type SiteConfig struct {
	Domain    string `json:"domain"`
	Mode      string `json:"mode"`
	TargetURL string `json:"target_url,omitempty"`
	Port      int    `json:"port,omitempty"`
}

// Validate validates the site configuration
func (c *SiteConfig) Validate() error {
	if c.Domain == "" {
		return ErrDomainEmpty
	}

	validModes := map[string]bool{
		RenderModeProxy:    true,
		RenderModeStatic:   true,
		RenderModeRedirect: true,
	}

	if !validModes[c.Mode] {
		return ErrModeInvalid
	}

	if c.Port < 0 || c.Port > 65535 {
		return ErrPortInvalid
	}

	if (c.Mode == RenderModeProxy || c.Mode == RenderModeRedirect) && c.TargetURL == "" {
		return ErrTargetURLEmpty
	}

	return nil
}

// WAFRule represents a WAF rule
type WAFRule struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Pattern  string `json:"pattern"`
	Action   string `json:"action"`
	Priority int    `json:"priority"`
	Enabled  bool   `json:"enabled"`
}

const (
	WAFActionBlock     = "block"
	WAFActionAllow     = "allow"
	WAFActionChallenge = "challenge"
	WAFActionLog       = "log"
)

var validActions = map[string]bool{
	WAFActionBlock:     true,
	WAFActionAllow:     true,
	WAFActionChallenge: true,
	WAFActionLog:       true,
}

// Validate validates the WAF rule
func (r *WAFRule) Validate() error {
	if r.Name == "" {
		return ErrWAFRuleNameEmpty
	}

	if r.Pattern == "" {
		return nil
	}

	if _, err := regexp.Compile(r.Pattern); err != nil {
		return fmt.Errorf("%w: %v", ErrPatternInvalid, err)
	}

	if r.Action != "" && !validActions[r.Action] {
		return ErrWAFRuleActionInvalid
	}

	return nil
}
