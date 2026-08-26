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
	cfg *config.Config
}

// NewSEOController 创建 SEO 控制器
func NewSEOController(cfg *config.Config) *SEOController {
	return &SEOController{cfg: cfg}
}

// GenerateSitemap 生成 Sitemap
// POST /api/v1/seo/sitemap/generate
func (c *SEOController) GenerateSitemap(ctx *gin.Context) {
	cfg := c.cfg.SEO.Sitemap

	if !cfg.Enabled {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Sitemap generation is disabled in config",
		})
		return
	}

	// 与启动时任务共用按站点生成逻辑：各静态站点输出到自身静态根目录
	results := seo.GenerateForAllSites(c.cfg.Dirs.StaticDir, c.cfg.Sites, cfg)
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
	cfg := c.cfg.SEO.Sitemap

	generator := seo.NewSitemapGenerator(seo.SitemapConfig{
		Enabled:         cfg.Enabled,
		BaseURL:         cfg.BaseURL,
		OutputDir:       cfg.OutputDir,
		ChangeFreq:      cfg.ChangeFreq,
		DefaultPriority: cfg.DefaultPriority,
		IncludePatterns: cfg.IncludePatterns,
		ExcludePatterns: cfg.ExcludePatterns,
	})

	staticDir := filepath.Join(c.cfg.Dirs.StaticDir, "default")
	sitemap, err := generator.GenerateFromFiles(staticDir)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to generate sitemap: " + err.Error(),
		})
		return
	}

	xmlData, err := generator.ToXML(sitemap)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to encode sitemap: " + err.Error(),
		})
		return
	}

	ctx.Data(http.StatusOK, "application/xml", xmlData)
}

// GenerateRobotsTxt 生成 robots.txt
// POST /api/v1/seo/robots/generate
func (c *SEOController) GenerateRobotsTxt(ctx *gin.Context) {
	cfg := c.cfg.SEO.Robots

	if !cfg.Enabled {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "robots.txt generation is disabled in config",
		})
		return
	}

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

	outputPath := filepath.Join(cfg.OutputDir, "robots.txt")
	if err := generator.WriteToFile(outputPath); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to write robots.txt: " + err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "robots.txt generated successfully",
		"data": gin.H{
			"output_path": outputPath,
		},
	})
}

// GetRobotsTxt 获取 robots.txt
// GET /api/v1/seo/robots
func (c *SEOController) GetRobotsTxt(ctx *gin.Context) {
	cfg := c.cfg.SEO.Robots

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
		"data": c.cfg.SEO,
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
	c.cfg.SEO = newCfg

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
