package licensing

import (
	"fmt"

	"prerender-shield/internal/config"
)

const FreeSiteLimit = 1

// Policy captures the commercial rule: one site is free with all features,
// every additional site requires a paid per-site license.
type Policy struct {
	MaxSites              int    `json:"max_sites"`
	Plan                  string `json:"plan"`
	SitePriceUSDPerYear   int    `json:"site_price_usd_per_year"`
	PrivateDeployPriceUSD int    `json:"private_deploy_price_usd"`
}

// NewPolicy builds a normalized commercial policy from config.
func NewPolicy(cfg config.CommercialConfig) Policy {
	p := Policy{
		MaxSites:              cfg.MaxSites,
		Plan:                  cfg.Plan,
		SitePriceUSDPerYear:   cfg.SitePriceUSDPerYear,
		PrivateDeployPriceUSD: cfg.PrivateDeployPriceUSD,
	}
	if p.MaxSites == 0 {
		p.MaxSites = FreeSiteLimit
	}
	if p.Plan == "" {
		p.Plan = "free"
	}
	if p.SitePriceUSDPerYear == 0 {
		p.SitePriceUSDPerYear = 99
	}
	if p.PrivateDeployPriceUSD == 0 {
		p.PrivateDeployPriceUSD = 9999
	}
	return p
}

// AllowsAdditionalSite reports whether one more site may be added.
// A negative MaxSites means unlimited, intended for private source delivery.
func (p Policy) AllowsAdditionalSite(currentSites int) bool {
	if p.MaxSites < 0 {
		return true
	}
	return currentSites < p.MaxSites
}

func (p Policy) UpgradeMessage(currentSites int) string {
	return fmt.Sprintf(
		"One site is free with all features. You currently have %d site(s); additional sites are licensed at $%d/year per site, or $%d for private source deployment.",
		currentSites,
		p.SitePriceUSDPerYear,
		p.PrivateDeployPriceUSD,
	)
}
