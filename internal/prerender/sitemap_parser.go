package prerender

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SitemapURL 表示 sitemap 中的一个 URL 条目
type SitemapURL struct {
	Loc        string
	LastMod    time.Time
	ChangeFreq string
	Priority   float64
}

// sitemapURLSet 对应 sitemap.xml 的根结构
type sitemapURLSet struct {
	URLs []struct {
		Loc        string `xml:"loc"`
		LastMod    string `xml:"lastmod"`
		ChangeFreq string `xml:"changefreq"`
		Priority   string `xml:"priority"`
	} `xml:"url"`
}

// sitemapIndex 对应 sitemap index.xml 的根结构
type sitemapIndex struct {
	Sitemaps []struct {
		Loc     string `xml:"loc"`
		LastMod string `xml:"lastmod"`
	} `xml:"sitemap"`
}

// ParseSitemap 解析 sitemap.xml 内容，返回 URL 列表
func ParseSitemap(data []byte) ([]SitemapURL, error) {
	var urlSet sitemapURLSet
	if err := xml.Unmarshal(data, &urlSet); err != nil {
		return nil, fmt.Errorf("failed to parse sitemap XML: %w", err)
	}

	urls := make([]SitemapURL, 0, len(urlSet.URLs))
	for _, u := range urlSet.URLs {
		su := SitemapURL{
			Loc:        strings.TrimSpace(u.Loc),
			ChangeFreq: u.ChangeFreq,
		}
		if u.LastMod != "" {
			if t, err := time.Parse(time.RFC3339, u.LastMod); err == nil {
				su.LastMod = t
			} else if t, err := time.Parse("2006-01-02", u.LastMod); err == nil {
				su.LastMod = t
			}
		}
		if u.Priority != "" {
			var p float64
			if _, err := fmt.Sscanf(u.Priority, "%f", &p); err == nil {
				su.Priority = p
			}
		}
		if su.Loc != "" {
			urls = append(urls, su)
		}
	}

	return urls, nil
}

// ParseSitemapIndex 解析 sitemap index.xml，返回子 sitemap 的 URL 列表
func ParseSitemapIndex(data []byte) ([]string, error) {
	var idx sitemapIndex
	if err := xml.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("failed to parse sitemap index XML: %w", err)
	}

	urls := make([]string, 0, len(idx.Sitemaps))
	for _, s := range idx.Sitemaps {
		loc := strings.TrimSpace(s.Loc)
		if loc != "" {
			urls = append(urls, loc)
		}
	}

	return urls, nil
}

// FetchAndParseSitemap 从指定 URL 获取并解析 sitemap.xml
// 如果是 sitemap index，会递归获取所有子 sitemap
func FetchAndParseSitemap(sitemapURL string) ([]SitemapURL, error) {
	data, err := fetchURL(sitemapURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sitemap from %s: %w", sitemapURL, err)
	}

	// 先尝试作为 sitemap index 解析
	subSitemaps, err := ParseSitemapIndex(data)
	if err == nil && len(subSitemaps) > 0 {
		// 是 sitemap index，递归获取子 sitemap
		var allURLs []SitemapURL
		for _, subURL := range subSitemaps {
			urls, err := FetchAndParseSitemap(subURL)
			if err != nil {
				continue
			}
			allURLs = append(allURLs, urls...)
		}
		return allURLs, nil
	}

	// 作为普通 sitemap 解析
	return ParseSitemap(data)
}

// fetchURL 获取 URL 内容
func fetchURL(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
