package licensing

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/config"
)

func TestNewPolicyDefaultsToOneFreeSite(t *testing.T) {
	policy := NewPolicy(config.CommercialConfig{})

	assert.Equal(t, FreeSiteLimit, policy.MaxSites)
	assert.Equal(t, "free", policy.Plan)
	assert.Equal(t, 99, policy.SitePriceUSDPerYear)
	assert.Equal(t, 9999, policy.PrivateDeployPriceUSD)
	assert.True(t, policy.AllowsAdditionalSite(0))
	assert.False(t, policy.AllowsAdditionalSite(1))
}

func TestPolicyAllowsConfiguredPaidSiteCount(t *testing.T) {
	policy := NewPolicy(config.CommercialConfig{
		MaxSites:              5,
		Plan:                  "per-site",
		SitePriceUSDPerYear:   99,
		PrivateDeployPriceUSD: 9999,
	})

	assert.True(t, policy.AllowsAdditionalSite(4))
	assert.False(t, policy.AllowsAdditionalSite(5))
}

func TestPolicyAllowsUnlimitedPrivateSourceDeployment(t *testing.T) {
	policy := NewPolicy(config.CommercialConfig{
		MaxSites: -1,
		Plan:     "private-source",
	})

	assert.True(t, policy.AllowsAdditionalSite(1000))
}
