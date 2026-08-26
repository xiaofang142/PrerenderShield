package seo

import (
	"fmt"
	"os"
	"path/filepath"

	"prerender-shield/internal/config"
)

// SiteSitemapResult 单站点 sitemap 生成结果
type SiteSitemapResult struct {
	SiteID     string
	OutputPath string
	URLCount   int
}

// GenerateForAllSites 按站点生成 sitemap 的共享实现。
// 每个静态站点从自己的静态目录扫描 URL，输出到该站点的静态根目录，
// 由站点服务器直接对外提供 /sitemap.xml；default 站点保持使用配置的
// OutputDir（向后兼容）。bootstrap 启动任务与 SEO API 共用此逻辑。
func GenerateForAllSites(staticRoot string, sites []config.SiteConfig, cfg config.SitemapSEOConfig) []SiteSitemapResult {
	results := make([]SiteSitemapResult, 0)

	for _, site := range sites {
		staticDir := filepath.Join(staticRoot, site.ID)
		if _, err := os.Stat(staticDir); err != nil {
			// 非静态站点（proxy/redirect 模式）无本地静态目录，跳过
			continue
		}

		siteCfg := cfg
		// BaseURL 未配置或为示例值时回退到站点第一个域名
		if siteCfg.BaseURL == "" || siteCfg.BaseURL == "https://example.com" {
			if len(site.Domains) > 0 {
				scheme := "https"
				if !site.SSL.Enabled {
					scheme = "http"
				}
				siteCfg.BaseURL = fmt.Sprintf("%s://%s", scheme, site.Domains[0])
			}
		}

		outputDir := cfg.OutputDir
		if site.ID != "default" {
			outputDir = staticDir
		}

		generator := NewSitemapGenerator(SitemapConfig{
			Enabled:         siteCfg.Enabled,
			BaseURL:         siteCfg.BaseURL,
			OutputDir:       outputDir,
			ChangeFreq:      siteCfg.ChangeFreq,
			DefaultPriority: siteCfg.DefaultPriority,
			IncludePatterns: siteCfg.IncludePatterns,
			ExcludePatterns: siteCfg.ExcludePatterns,
		})

		sitemap, err := generator.GenerateFromFiles(staticDir)
		if err != nil {
			continue
		}

		outputPath := filepath.Join(outputDir, "sitemap.xml")
		if err := generator.WriteToFile(sitemap, outputPath); err != nil {
			continue
		}

		results = append(results, SiteSitemapResult{
			SiteID:     site.ID,
			OutputPath: outputPath,
			URLCount:   len(sitemap.URLs),
		})
	}

	return results
}
