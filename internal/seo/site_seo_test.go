package seo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"prerender-shield/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildStaticSite 在 staticRoot 下创建一个站点目录及页面文件
func buildStaticSite(t *testing.T, staticRoot, siteID string, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(staticRoot, siteID)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	return dir
}

func TestGenerateForAllSites(t *testing.T) {
	staticRoot := t.TempDir()
	outputDir := t.TempDir()

	buildStaticSite(t, staticRoot, "s1", map[string]string{
		"index.html": "<html></html>",
		"about.html": "<html></html>",
	})
	s2Dir := buildStaticSite(t, staticRoot, "s2", map[string]string{
		"page.html": "<html></html>",
	})
	// 预置一个同名目录，使 os.Create 失败，覆盖写失败跳过分支
	require.NoError(t, os.MkdirAll(filepath.Join(s2Dir, "sitemap.xml"), 0o755))
	buildStaticSite(t, staticRoot, "default", map[string]string{
		"index.html": "<html></html>",
	})

	sites := []config.SiteConfig{
		{ID: "s1", Domains: []string{"s1.example.com"}, SSL: config.SiteSSLConfig{Enabled: true}},
		{ID: "s2", Domains: []string{"s2.example.com"}, SSL: config.SiteSSLConfig{Enabled: false}},
		{ID: "default", Domains: []string{"default.example.com"}, SSL: config.SiteSSLConfig{Enabled: true}},
		{ID: "ghost", Domains: []string{"ghost.example.com"}}, // 无静态目录，跳过
	}

	cfg := config.SitemapSEOConfig{
		Enabled:         true,
		BaseURL:         "", // 未配置 → 回退到站点域名
		OutputDir:       outputDir,
		ChangeFreq:      "daily",
		DefaultPriority: "0.5",
		IncludePatterns: []string{"*.html"},
	}

	results := GenerateForAllSites(staticRoot, sites, cfg)

	bySite := make(map[string]SiteSitemapResult)
	for _, r := range results {
		bySite[r.SiteID] = r
	}

	// s1：成功生成，BaseURL 回退为 https://域名
	r1, ok := bySite["s1"]
	require.True(t, ok, "s1 should be generated, results=%v", results)
	assert.Equal(t, filepath.Join(staticRoot, "s1", "sitemap.xml"), r1.OutputPath)
	assert.Equal(t, 2, r1.URLCount)

	content, err := os.ReadFile(r1.OutputPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "https://s1.example.com/")

	// s2：sitemap.xml 写失败，被跳过
	_, ok = bySite["s2"]
	assert.False(t, ok, "s2 write failure should be skipped")

	// default：写入配置的 OutputDir
	r3, ok := bySite["default"]
	require.True(t, ok)
	assert.Equal(t, filepath.Join(outputDir, "sitemap.xml"), r3.OutputPath)
	assert.Equal(t, 1, r3.URLCount)

	// ghost：无静态目录，被跳过
	_, ok = bySite["ghost"]
	assert.False(t, ok)
}

func TestGenerateForAllSites_ExampleBaseURLFallback(t *testing.T) {
	staticRoot := t.TempDir()
	buildStaticSite(t, staticRoot, "only", map[string]string{"page.html": "<html></html>"})

	sites := []config.SiteConfig{
		{ID: "only", Domains: []string{"only.example.com"}, SSL: config.SiteSSLConfig{Enabled: false}},
	}

	// 示例值 BaseURL 同样触发域名回退，且 SSL 未启用时使用 http
	cfg := config.SitemapSEOConfig{
		Enabled:   true,
		BaseURL:   "https://example.com",
		OutputDir: t.TempDir(),
	}

	results := GenerateForAllSites(staticRoot, sites, cfg)
	require.Len(t, results, 1)

	content, err := os.ReadFile(results[0].OutputPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "http://only.example.com/page.html")
}

func TestGenerateRobotsForAllSites(t *testing.T) {
	staticRoot := t.TempDir()

	buildStaticSite(t, staticRoot, "r1", map[string]string{})
	buildStaticSite(t, staticRoot, "r2", map[string]string{})
	r3Dir := buildStaticSite(t, staticRoot, "r3", map[string]string{})
	// 预置同名目录，使 robots.txt 写入失败
	require.NoError(t, os.MkdirAll(filepath.Join(r3Dir, "robots.txt"), 0o755))
	buildStaticSite(t, staticRoot, "r4", map[string]string{})

	sites := []config.SiteConfig{
		{ID: "r1", Domains: []string{"r1.example.com"}, SSL: config.SiteSSLConfig{Enabled: true}},
		{ID: "r2", Domains: []string{"r2.example.com"}, SSL: config.SiteSSLConfig{Enabled: false}},
		{ID: "r3", Domains: []string{"r3.example.com"}, SSL: config.SiteSSLConfig{Enabled: true}},
		{ID: "r4"}, // 无域名：sitemapURL 保持为空
		{ID: "ghost", Domains: []string{"ghost.example.com"}}, // 无静态目录，跳过
	}

	cfg := config.RobotsSEOConfig{
		Enabled:    true,
		OutputDir:  t.TempDir(),
		SitemapURL: "", // 空 → 按站点域名补全
		Rules: []config.RobotsRuleSEO{
			{UserAgent: "*", Allow: []string{"/"}, Disallow: []string{"/admin/"}},
		},
	}

	results := GenerateRobotsForAllSites(staticRoot, sites, cfg)

	bySite := make(map[string]SiteRobotsResult)
	for _, r := range results {
		bySite[r.SiteID] = r
	}

	// r1：SSL 启用 → https sitemap
	r1, ok := bySite["r1"]
	require.True(t, ok, "r1 should be generated, results=%v", results)
	r1Content, err := os.ReadFile(r1.Path)
	require.NoError(t, err)
	assert.Contains(t, string(r1Content), "Sitemap: https://r1.example.com/sitemap.xml")
	assert.Contains(t, string(r1Content), "Disallow: /admin/")

	// r2：SSL 未启用 → http sitemap
	r2, ok := bySite["r2"]
	require.True(t, ok)
	r2Content, err := os.ReadFile(r2.Path)
	require.NoError(t, err)
	assert.Contains(t, string(r2Content), "Sitemap: http://r2.example.com/sitemap.xml")

	// r3：写失败被跳过
	_, ok = bySite["r3"]
	assert.False(t, ok)

	// r4：无域名，robots.txt 中不含 Sitemap 行
	r4, ok := bySite["r4"]
	require.True(t, ok)
	r4Content, err := os.ReadFile(r4.Path)
	require.NoError(t, err)
	assert.False(t, strings.Contains(string(r4Content), "Sitemap:"))

	// ghost：无静态目录被跳过
	_, ok = bySite["ghost"]
	assert.False(t, ok)
}

func TestGenerateRobotsForAllSites_ConfiguredSitemapURL(t *testing.T) {
	staticRoot := t.TempDir()
	buildStaticSite(t, staticRoot, "r1", map[string]string{})

	sites := []config.SiteConfig{
		{ID: "r1", Domains: []string{"r1.example.com"}, SSL: config.SiteSSLConfig{Enabled: true}},
	}

	// 已配置全局 SitemapURL 时直接使用，不再按域名补全
	cfg := config.RobotsSEOConfig{
		Enabled:    true,
		SitemapURL: "https://cdn.example.com/sitemap.xml",
		Rules:      []config.RobotsRuleSEO{{UserAgent: "*"}},
	}

	results := GenerateRobotsForAllSites(staticRoot, sites, cfg)
	require.Len(t, results, 1)

	content, err := os.ReadFile(results[0].Path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "Sitemap: https://cdn.example.com/sitemap.xml")
	assert.NotContains(t, string(content), "r1.example.com/sitemap.xml")
}
