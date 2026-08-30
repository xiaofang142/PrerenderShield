package controllers

import "prerender-shield/internal/config"

// configRef 控制器配置引用。
// 背景：ConfigManager.Mutate/UpdateConfig 采用 copy-on-write 换指针实现事务性更新，
// 启动时注入的 *Config 快照在首次 Mutate 后即陈旧（站点列表、系统配置均不可见）。
// 生产环境由 controller_setup 通过 setConfigProvider 注入 configManager.GetConfig，
// 每请求读取最新配置；测试直接构造未注入 provider 时回退 snapshot 字段，行为与旧版一致。
type configRef struct {
	snapshot *config.Config
	provider func() *config.Config
}

func (r *configRef) current() *config.Config {
	if r.provider != nil {
		return r.provider()
	}
	return r.snapshot
}

func (r *configRef) setProvider(fn func() *config.Config) {
	r.provider = fn
}

// SetConfigProvider 注入每请求新鲜配置来源（controller_setup 生产装配用）。
type configProviderSetter interface {
	SetConfigProvider(fn func() *config.Config)
}

func (c *OverviewController) SetConfigProvider(fn func() *config.Config) { c.cfg.setProvider(fn) }
func (c *PreheatController) SetConfigProvider(fn func() *config.Config)  { c.cfg.setProvider(fn) }
func (c *PushController) SetConfigProvider(fn func() *config.Config)     { c.cfg.setProvider(fn) }
func (c *SitesController) SetConfigProvider(fn func() *config.Config)    { c.cfg.setProvider(fn) }
func (c *SEOController) SetConfigProvider(fn func() *config.Config)      { c.cfg.setProvider(fn) }
