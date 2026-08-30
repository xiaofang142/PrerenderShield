package seo

import (
	"fmt"
	"os"
	"path/filepath"

	"prerender-shield/internal/config"
)

// SiteRobotsResult 单站点 robots.txt 生成结果
type SiteRobotsResult struct {
	SiteID string `json:"site_id"`
	Path   string `json:"path"`
	URLs   int    `json:"urls"`
}

// GenerateRobotsForAllSites 为所有静态站点生成 robots.txt 到各自静态根目录。
// 修复假性完成：旧实现写到单一 cfg.OutputDir（未配置时落 CWD，任何站点都服务不到）。
// 与 GenerateForAllSites 同款遍历；SitemapURL 未配置时按站点域名补全（scheme://domain/sitemap.xml）。
func GenerateRobotsForAllSites(staticRoot string, sites []config.SiteConfig, cfg config.RobotsSEOConfig) []SiteRobotsResult {
	results := make([]SiteRobotsResult, 0)

	for _, site := range sites {
		staticDir := filepath.Join(staticRoot, site.ID)
		if _, err := os.Stat(staticDir); err != nil {
			continue // 非 static 模式无本地目录，跳过
		}

		rules := make([]RobotsRule, len(cfg.Rules))
		for i, r := range cfg.Rules {
			rules[i] = RobotsRule{
				UserAgent:  r.UserAgent,
				Allow:      r.Allow,
				Disallow:   r.Disallow,
				CrawlDelay: r.CrawlDelay,
			}
		}

		sitemapURL := cfg.SitemapURL
		if sitemapURL == "" && len(site.Domains) > 0 {
			scheme := "https"
			if !site.SSL.Enabled {
				scheme = "http"
			}
			sitemapURL = fmt.Sprintf("%s://%s/sitemap.xml", scheme, site.Domains[0])
		}

		generator := NewRobotsGenerator(RobotsConfig{
			Enabled:    cfg.Enabled,
			OutputDir:  staticDir,
			SitemapURL: sitemapURL,
			Rules:      rules,
		})

		outputPath := filepath.Join(staticDir, "robots.txt")
		if err := generator.WriteToFile(outputPath); err != nil {
			continue
		}
		results = append(results, SiteRobotsResult{SiteID: site.ID, Path: outputPath})
	}
	return results
}
