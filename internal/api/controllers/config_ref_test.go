package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/config"
)

// TestConfigRef_ProviderPriority provider 存在时优先于 snapshot
func TestConfigRef_ProviderPriority(t *testing.T) {
	snap := &config.Config{Sites: []config.SiteConfig{{ID: "snap-site"}}}
	fresh := &config.Config{Sites: []config.SiteConfig{{ID: "fresh-site"}}}

	ref := &configRef{snapshot: snap}
	assert.Same(t, snap, ref.current())

	ref.setProvider(func() *config.Config { return fresh })
	assert.Same(t, fresh, ref.current())
}

// TestConfigRef_SetConfigProvider_AllControllers 五个控制器的 SetConfigProvider 装配入口
func TestConfigRef_SetConfigProvider_AllControllers(t *testing.T) {
	fresh := &config.Config{Sites: []config.SiteConfig{{ID: "fresh"}}}
	provider := func() *config.Config { return fresh }

	overview := &OverviewController{cfg: configRef{snapshot: &config.Config{}}}
	overview.SetConfigProvider(provider)
	assert.Same(t, fresh, overview.cfg.current())

	preheat := &PreheatController{}
	preheat.SetConfigProvider(provider)
	assert.Same(t, fresh, preheat.cfg.current())

	push := &PushController{}
	push.SetConfigProvider(provider)
	assert.Same(t, fresh, push.cfg.current())

	sites := &SitesController{}
	sites.SetConfigProvider(provider)
	assert.Same(t, fresh, sites.cfg.current())

	seo := &SEOController{}
	seo.SetConfigProvider(provider)
	assert.Same(t, fresh, seo.cfg.current())
}
