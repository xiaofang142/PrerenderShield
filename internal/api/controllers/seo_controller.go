package controllers

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/config"
	"prerender-shield/internal/seo"
)

// SEOController SEO 管理控制器
type SEOController struct {
	cfg configRef
}

// NewSEOController 创建 SEO 控制器
func NewSEOController(cfg *config.Config) *SEOController {
	return &SEOController{cfg: configRef{snapshot: cfg}}
}

// GenerateSitemap 生成 Sitemap
// POST /api/v1/seo/sitemap/generate
func (c *SEOController) GenerateSitemap(ctx *gin.Context) {
	cfg := c.cfg.current().SEO.Sitemap

	if !cfg.Enabled {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Sitemap generation is disabled in config",
		})
		return
	}

	// 与启动时任务共用按站点生成逻辑：各静态站点输出到自身静态根目录
	results := seo.GenerateForAllSites(c.cfg.current().Dirs.StaticDir, c.cfg.current().Sites, cfg)
	if len(results) == 0 {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "No static site directories found to generate sitemap",
		})
		return
	}

	totalURLs := 0
	paths := make([]string, 0, len(results))
	for _, res := range results {
		totalURLs += res.URLCount
		paths = append(paths, res.OutputPath)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Sitemap generated successfully",
		"data": gin.H{
			"sites":     len(results),
			"outputs":   paths,
			"url_count": totalURLs,
		},
	})
}

// GetSitemap 获取 Sitemap XML
// GET /api/v1/seo/sitemap
func (c *SEOController) GetSitemap(ctx *gin.Context) {
	cfg := c.cfg.current().SEO.Sitemap

	generator := seo.NewSitemapGenerator(seo.SitemapConfig{
		Enabled:         cfg.Enabled,
		BaseURL:         cfg.BaseURL,
		OutputDir:       cfg.OutputDir,
		ChangeFreq:      cfg.ChangeFreq,
		DefaultPriority: cfg.DefaultPriority,
		IncludePatterns: cfg.IncludePatterns,
		ExcludePatterns: cfg.ExcludePatterns,
	})

	staticDir := filepath.Join(c.cfg.current().Dirs.StaticDir, "default")
	// 注：GenerateFromFiles 的 walkFn 吞掉一切遍历错误并恒返回 nil error（见 seo/sitemap.go），
	// 错误分支不可达
	sitemap, _ := generator.GenerateFromFiles(staticDir)

	// 注：Sitemap 仅含 string/[]struct 字段，xml.MarshalIndent 不可能失败
	xmlData, _ := generator.ToXML(sitemap)

	ctx.Data(http.StatusOK, "application/xml", xmlData)
}

// GenerateRobotsTxt 生成 robots.txt
// POST /api/v1/seo/robots/generate
func (c *SEOController) GenerateRobotsTxt(ctx *gin.Context) {
	cfg := c.cfg.current().SEO.Robots

	if !cfg.Enabled {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "robots.txt generation is disabled in config",
		})
		return
	}

	// 按站点生成到各自静态根（与 sitemap 同款遍历）。
	// 修复假性完成：旧实现写单一 cfg.OutputDir，未配置时落 CWD —— 任何站点都服务不到。
	results := seo.GenerateRobotsForAllSites(c.cfg.current().Dirs.StaticDir, c.cfg.current().Sites, cfg)
	if len(results) == 0 {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "No static site directories found to generate robots.txt",
		})
		return
	}

	paths := make([]string, 0, len(results))
	for _, r := range results {
		paths = append(paths, r.Path)
	}
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "robots.txt generated successfully",
		"data": gin.H{
			"output_paths": paths,
		},
	})
}

// GetRobotsTxt 获取 robots.txt
// GET /api/v1/seo/robots
func (c *SEOController) GetRobotsTxt(ctx *gin.Context) {
	cfg := c.cfg.current().SEO.Robots

	rules := make([]seo.RobotsRule, len(cfg.Rules))
	for i, r := range cfg.Rules {
		rules[i] = seo.RobotsRule{
			UserAgent:  r.UserAgent,
			Allow:      r.Allow,
			Disallow:   r.Disallow,
			CrawlDelay: r.CrawlDelay,
		}
	}

	generator := seo.NewRobotsGenerator(seo.RobotsConfig{
		Enabled:    cfg.Enabled,
		OutputDir:  cfg.OutputDir,
		SitemapURL: cfg.SitemapURL,
		Rules:      rules,
	})

	content := generator.GenerateWithTimestamp()
	ctx.Data(http.StatusOK, "text/plain", []byte(content))
}

// GetSEOConfig 获取 SEO 配置
// GET /api/v1/seo/config
func (c *SEOController) GetSEOConfig(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": c.cfg.current().SEO,
	})
}

// UpdateSEOConfig 更新 SEO 配置
// PUT /api/v1/seo/config
func (c *SEOController) UpdateSEOConfig(ctx *gin.Context) {
	var newCfg config.SEOConfig
	if err := ctx.ShouldBindJSON(&newCfg); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request body: " + err.Error(),
		})
		return
	}

	// Update config
	c.cfg.current().SEO = newCfg

	// Save to config manager
	if err := config.GetInstance().SaveConfig(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to save config: " + err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "SEO config updated successfully",
	})
}
