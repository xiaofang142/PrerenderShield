package seo

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultSitemapConfig(t *testing.T) {
	cfg := DefaultSitemapConfig()

	assert.False(t, cfg.Enabled)
	assert.Equal(t, "daily", cfg.ChangeFreq)
	assert.Equal(t, "0.5", cfg.DefaultPriority)
	assert.Equal(t, []string{"*.html", "*.htm"}, cfg.IncludePatterns)
	assert.Equal(t, []string{"admin/*", "api/*", "login*"}, cfg.ExcludePatterns)
}

func TestNewSitemapGenerator(t *testing.T) {
	cfg := DefaultSitemapConfig()
	g := NewSitemapGenerator(cfg)
	assert.NotNil(t, g)
	assert.Equal(t, cfg, g.config)
}

func TestSitemapGenerator_GenerateFromFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile := func(rel, content string) {
		full := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}

	writeFile("index.html", "<html></html>")
	writeFile("about.html", "<html></html>")
	writeFile("sub/index.html", "<html></html>")
	writeFile("sub/page.htm", "<html></html>")
	writeFile("admin/secret.html", "<html></html>")
	writeFile("notes.txt", "not html")

	g := NewSitemapGenerator(SitemapConfig{
		BaseURL:         "https://example.com/",
		ChangeFreq:      "weekly",
		DefaultPriority: "0.8",
		IncludePatterns: []string{"*.html", "*.htm"},
		ExcludePatterns: []string{"admin/*"},
	})

	sitemap, err := g.GenerateFromFiles(dir)
	require.NoError(t, err)

	locs := make(map[string]SitemapURL)
	for _, u := range sitemap.URLs {
		locs[u.Loc] = u
	}

	// index.html 位于根目录时映射为 "/"
	root, ok := locs["https://example.com/"]
	require.True(t, ok, "root index.html should map to base URL, got %v", locs)
	assert.Equal(t, "0.8", root.Priority)
	// 根 index.html 不应同时出现 /index.html
	_, ok = locs["https://example.com/index.html"]
	assert.False(t, ok)

	about, ok := locs["https://example.com/about.html"]
	require.True(t, ok)
	assert.Equal(t, "weekly", about.ChangeFreq)
	assert.Equal(t, "0.8", about.Priority)
	assert.NotEmpty(t, about.LastMod)
	_, err = time.Parse(time.RFC3339, about.LastMod)
	assert.NoError(t, err, "lastmod should be RFC3339")

	// 子目录 index.html 映射为目录 URL
	_, ok = locs["https://example.com/sub/"]
	assert.True(t, ok, "sub/index.html should map to /sub/")

	// 子目录普通页面
	_, ok = locs["https://example.com/sub/page.htm"]
	assert.True(t, ok)

	// 排除模式命中
	_, ok = locs["https://example.com/admin/secret.html"]
	assert.False(t, ok)

	// 非 HTML 文件被跳过
	assert.Len(t, sitemap.URLs, 4)

	// xmlns 属性
	assert.Equal(t, "http://www.sitemaps.org/schemas/sitemap/0.9", sitemap.Xmlns)
}

func TestSitemapGenerator_GenerateFromFiles_NonexistentDir(t *testing.T) {
	g := NewSitemapGenerator(DefaultSitemapConfig())

	// 不存在的目录：walkFn 吞掉错误，返回空 sitemap 且不报错
	sitemap, err := g.GenerateFromFiles(filepath.Join(t.TempDir(), "missing"))
	require.NoError(t, err)
	assert.Empty(t, sitemap.URLs)
}

func TestSitemapGenerator_GenerateFromURLs(t *testing.T) {
	g := NewSitemapGenerator(SitemapConfig{
		ChangeFreq:      "daily",
		DefaultPriority: "0.5",
		ExcludePatterns: []string{"*admin*"},
	})

	sitemap := g.GenerateFromURLs([]string{
		"https://example.com/a",
		"https://example.com/admin/x",
		"https://example.com/b",
	})

	assert.Len(t, sitemap.URLs, 2)
	assert.Equal(t, "https://example.com/a", sitemap.URLs[0].Loc)
	assert.Equal(t, "https://example.com/b", sitemap.URLs[1].Loc)
	assert.Equal(t, "daily", sitemap.URLs[0].ChangeFreq)
	_, err := time.Parse(time.RFC3339, sitemap.URLs[0].LastMod)
	assert.NoError(t, err)
}

func TestSitemapGenerator_WriteToFile(t *testing.T) {
	dir := t.TempDir()
	g := NewSitemapGenerator(DefaultSitemapConfig())
	sitemap := &Sitemap{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs: []SitemapURL{
			{Loc: "https://example.com/"},
		},
	}

	outputPath := filepath.Join(dir, "out", "sitemap.xml")
	require.NoError(t, g.WriteToFile(sitemap, outputPath))

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	content := string(data)
	assert.True(t, strings.HasPrefix(content, xml.Header))
	assert.Contains(t, content, "<urlset")
	assert.Contains(t, content, "<loc>https://example.com/</loc>")
}

func TestSitemapGenerator_WriteToFile_MkdirError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))

	g := NewSitemapGenerator(DefaultSitemapConfig())
	err := g.WriteToFile(&Sitemap{}, filepath.Join(blocker, "sitemap.xml"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create directory")
}

func TestSitemapGenerator_WriteToFile_CreateError(t *testing.T) {
	dir := t.TempDir()
	// 目标路径是一个目录，os.Create 失败
	target := filepath.Join(dir, "sitemap.xml")
	require.NoError(t, os.MkdirAll(target, 0o755))

	g := NewSitemapGenerator(DefaultSitemapConfig())
	err := g.WriteToFile(&Sitemap{}, target)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create file")
}

func TestSitemapGenerator_ToXML(t *testing.T) {
	g := NewSitemapGenerator(DefaultSitemapConfig())
	sitemap := &Sitemap{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs: []SitemapURL{
			{Loc: "https://example.com/page", LastMod: "2026-08-30T00:00:00Z"},
		},
	}

	data, err := g.ToXML(sitemap)
	require.NoError(t, err)

	content := string(data)
	assert.True(t, strings.HasPrefix(content, xml.Header))
	assert.Contains(t, content, "<loc>https://example.com/page</loc>")
	assert.Contains(t, content, "<lastmod>2026-08-30T00:00:00Z</lastmod>")
	assert.Contains(t, content, `xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"`)
}

func TestSitemapGenerator_MatchesPatterns(t *testing.T) {
	tests := []struct {
		name string
		cfg  SitemapConfig
		path string
		want bool
	}{
		{
			name: "include hit",
			cfg:  SitemapConfig{IncludePatterns: []string{"*.html"}},
			path: "page.html",
			want: true,
		},
		{
			name: "include miss",
			cfg:  SitemapConfig{IncludePatterns: []string{"*.html"}},
			path: "page.txt",
			want: false,
		},
		{
			name: "no include patterns accepts all",
			cfg:  SitemapConfig{},
			path: "anything.bin",
			want: true,
		},
		{
			name: "exclude hit wins",
			cfg:  SitemapConfig{ExcludePatterns: []string{"admin/*"}},
			path: "admin/x.html",
			want: false,
		},
		{
			name: "exclude miss keeps path",
			cfg:  SitemapConfig{ExcludePatterns: []string{"admin/*"}},
			path: "public/x.html",
			want: true,
		},
		{
			name: "multiple includes any match",
			cfg:  SitemapConfig{IncludePatterns: []string{"*.md", "*.html"}},
			path: "doc.html",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewSitemapGenerator(tt.cfg)
			assert.Equal(t, tt.want, g.matchesPatterns(tt.path))
		})
	}
}

func TestSitemapGenerator_MatchesURLPatterns(t *testing.T) {
	g := NewSitemapGenerator(SitemapConfig{ExcludePatterns: []string{"*login*"}})

	assert.False(t, g.matchesURLPatterns("https://example.com/login"))
	assert.True(t, g.matchesURLPatterns("https://example.com/home"))
}

func TestMatchGlob(t *testing.T) {
	assert.True(t, matchGlob("*.html", "page.html"))
	assert.True(t, matchGlob("page?.html", "page1.html"))
	assert.False(t, matchGlob("page?.html", "page12.html"))
	assert.False(t, matchGlob("*.html", "page.htm"))
	// "." 应被转义为字面量
	assert.False(t, matchGlob("a.html", "aXhtml"))
}

func TestGlobToRegex(t *testing.T) {
	assert.Equal(t, `^a\.html$`, globToRegex("a.html"))
	assert.Equal(t, `^a.*$`, globToRegex("a*"))
	assert.Equal(t, `^a.$`, globToRegex("a?"))
	assert.Equal(t, `^$`, globToRegex(""))
}

func TestSitemapGenerator_GenerateFromFiles_SkipsNonHTMLWhenNoIncludePatterns(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("text"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "page.html"), []byte("<html></html>"), 0o644))

	// IncludePatterns 为空时所有文件都通过模式匹配，但非 .html/.htm 文件仍被扩展名过滤
	g := NewSitemapGenerator(SitemapConfig{BaseURL: "https://example.com"})

	sitemap, err := g.GenerateFromFiles(dir)
	require.NoError(t, err)
	require.Len(t, sitemap.URLs, 1)
	assert.Equal(t, "https://example.com/page.html", sitemap.URLs[0].Loc)
}
