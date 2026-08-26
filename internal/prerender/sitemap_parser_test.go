package prerender

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSitemap(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://example.com/</loc>
    <lastmod>2026-01-01</lastmod>
    <changefreq>daily</changefreq>
    <priority>1.0</priority>
  </url>
  <url>
    <loc>https://example.com/page1</loc>
    <lastmod>2026-06-15T10:30:00Z</lastmod>
    <changefreq>weekly</changefreq>
    <priority>0.8</priority>
  </url>
  <url>
    <loc>https://example.com/page2</loc>
  </url>
</urlset>`)

	urls, err := ParseSitemap(xmlData)
	assert.NoError(t, err)
	assert.Len(t, urls, 3)

	assert.Equal(t, "https://example.com/", urls[0].Loc)
	assert.Equal(t, "daily", urls[0].ChangeFreq)
	assert.Equal(t, 1.0, urls[0].Priority)
	assert.False(t, urls[0].LastMod.IsZero())

	assert.Equal(t, "https://example.com/page1", urls[1].Loc)
	assert.Equal(t, "weekly", urls[1].ChangeFreq)
	assert.Equal(t, 0.8, urls[1].Priority)

	assert.Equal(t, "https://example.com/page2", urls[2].Loc)
	assert.True(t, urls[2].LastMod.IsZero())
	assert.Equal(t, 0.0, urls[2].Priority)
}

func TestParseSitemap_InvalidXML(t *testing.T) {
	_, err := ParseSitemap([]byte("invalid xml"))
	assert.Error(t, err)
}

func TestParseSitemap_EmptyURLSet(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"></urlset>`)

	urls, err := ParseSitemap(xmlData)
	assert.NoError(t, err)
	assert.Empty(t, urls)
}

func TestParseSitemapIndex(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap>
    <loc>https://example.com/sitemap1.xml</loc>
    <lastmod>2026-01-01</lastmod>
  </sitemap>
  <sitemap>
    <loc>https://example.com/sitemap2.xml</loc>
  </sitemap>
</sitemapindex>`)

	sitemaps, err := ParseSitemapIndex(xmlData)
	assert.NoError(t, err)
	assert.Len(t, sitemaps, 2)
	assert.Equal(t, "https://example.com/sitemap1.xml", sitemaps[0])
	assert.Equal(t, "https://example.com/sitemap2.xml", sitemaps[1])
}
