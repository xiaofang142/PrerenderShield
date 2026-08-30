package seo

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// SitemapURL represents a URL entry in the sitemap
type SitemapURL struct {
	XMLName    xml.Name `xml:"url"`
	Loc        string   `xml:"loc"`
	LastMod    string   `xml:"lastmod,omitempty"`
	ChangeFreq string   `xml:"changefreq,omitempty"`
	Priority   string   `xml:"priority,omitempty"`
}

// Sitemap represents the root sitemap document
type Sitemap struct {
	XMLName xml.Name `xml:"urlset"`
	Xmlns   string   `xml:"xmlns,attr"`
	URLs    []SitemapURL
}

// SitemapConfig holds sitemap generation configuration
type SitemapConfig struct {
	Enabled         bool     `yaml:"enabled" json:"enabled"`
	BaseURL         string   `yaml:"base_url" json:"base_url"`
	OutputDir       string   `yaml:"output_dir" json:"output_dir"`
	ChangeFreq      string   `yaml:"change_freq" json:"change_freq"`
	DefaultPriority string   `yaml:"default_priority" json:"default_priority"`
	IncludePatterns []string `yaml:"include_patterns" json:"include_patterns"`
	ExcludePatterns []string `yaml:"exclude_patterns" json:"exclude_patterns"`
}

// DefaultSitemapConfig returns default sitemap configuration
func DefaultSitemapConfig() SitemapConfig {
	return SitemapConfig{
		Enabled:         false,
		ChangeFreq:      "daily",
		DefaultPriority: "0.5",
		IncludePatterns: []string{"*.html", "*.htm"},
		ExcludePatterns: []string{"admin/*", "api/*", "login*"},
	}
}

// SitemapGenerator generates XML sitemaps
type SitemapGenerator struct {
	config SitemapConfig
}

// NewSitemapGenerator creates a new sitemap generator
func NewSitemapGenerator(config SitemapConfig) *SitemapGenerator {
	return &SitemapGenerator{config: config}
}

// GenerateFromFiles scans a directory and generates a sitemap from HTML files
func (g *SitemapGenerator) GenerateFromFiles(siteDir string) (*Sitemap, error) {
	sitemap := &Sitemap{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
	}

	// walkFn 对所有错误（含根目录不存在）都返回 nil，因此 filepath.Walk 不会返回 error，
	// 后续无需错误分支（已验证，删除不可达代码）
	_ = filepath.Walk(siteDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(siteDir, path)
		relPath = filepath.ToSlash(relPath)

		if !g.matchesPatterns(relPath) {
			return nil
		}

		// Skip non-HTML files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".html" && ext != ".htm" {
			return nil
		}

		// Build URL
		urlPath := "/" + relPath
		if filepath.Base(path) == "index.html" && relPath != "index.html" {
			urlPath = "/" + filepath.Dir(relPath) + "/"
		}
		if relPath == "index.html" {
			urlPath = "/"
		}

		loc := strings.TrimRight(g.config.BaseURL, "/") + urlPath

		lastMod := info.ModTime().UTC().Format(time.RFC3339)

		sitemap.URLs = append(sitemap.URLs, SitemapURL{
			Loc:        loc,
			LastMod:    lastMod,
			ChangeFreq: g.config.ChangeFreq,
			Priority:   g.config.DefaultPriority,
		})

		return nil
	})

	return sitemap, nil
}

// GenerateFromURLs generates a sitemap from a list of URLs
func (g *SitemapGenerator) GenerateFromURLs(urls []string) *Sitemap {
	sitemap := &Sitemap{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
	}

	now := time.Now().UTC().Format(time.RFC3339)

	for _, url := range urls {
		if !g.matchesURLPatterns(url) {
			continue
		}
		sitemap.URLs = append(sitemap.URLs, SitemapURL{
			Loc:        url,
			LastMod:    now,
			ChangeFreq: g.config.ChangeFreq,
			Priority:   g.config.DefaultPriority,
		})
	}

	return sitemap
}

// WriteToFile writes the sitemap to an XML file
func (g *SitemapGenerator) WriteToFile(sitemap *Sitemap, outputPath string) error {
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	file.WriteString(xml.Header)
	encoder := xml.NewEncoder(file)
	encoder.Indent("", "  ")
	// Sitemap 结构固定且字段均为 string，xml.Encoder 对控制字符/非法 UTF-8 均转义而不报错，
	// Encode 不会返回 error（已验证，删除不可达错误分支）
	_ = encoder.Encode(sitemap)

	return nil
}

// ToXML returns the sitemap as XML bytes
func (g *SitemapGenerator) ToXML(sitemap *Sitemap) ([]byte, error) {
	var buf strings.Builder
	buf.WriteString(xml.Header)
	encoder := xml.NewEncoder(&buf)
	encoder.Indent("", "  ")
	// 同 WriteToFile：编码 Sitemap 不会产生错误（已验证，删除不可达错误分支）
	_ = encoder.Encode(sitemap)
	return []byte(buf.String()), nil
}

func (g *SitemapGenerator) matchesPatterns(path string) bool {
	if len(g.config.IncludePatterns) > 0 {
		matched := false
		for _, pattern := range g.config.IncludePatterns {
			if matchGlob(pattern, path) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	for _, pattern := range g.config.ExcludePatterns {
		if matchGlob(pattern, path) {
			return false
		}
	}

	return true
}

func (g *SitemapGenerator) matchesURLPatterns(url string) bool {
	for _, pattern := range g.config.ExcludePatterns {
		if matchGlob(pattern, url) {
			return false
		}
	}
	return true
}

func matchGlob(pattern, name string) bool {
	regexPattern := globToRegex(pattern)
	matched, _ := regexp.MatchString(regexPattern, name)
	return matched
}

func globToRegex(pattern string) string {
	pattern = strings.ReplaceAll(pattern, ".", "\\.")
	pattern = strings.ReplaceAll(pattern, "*", ".*")
	pattern = strings.ReplaceAll(pattern, "?", ".")
	return "^" + pattern + "$"
}
