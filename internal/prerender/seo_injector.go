package prerender

import (
	"fmt"
	"strings"

	"prerender-shield/internal/seo"
)

type SEOInjector struct {
	metaOpt   *seo.MetaTagsOptimizer
	structOpt *seo.StructuredDataOptimizer
}

func NewSEOInjector(metaConfig *seo.MetaTagsConfig, structConfig *seo.StructuredDataConfig) *SEOInjector {
	return &SEOInjector{
		metaOpt:   seo.NewMetaTagsOptimizer(metaConfig, nil),
		structOpt: seo.NewStructuredDataOptimizer(structConfig, nil),
	}
}

func (s *SEOInjector) InjectSEOTags(html []byte, pageURL string) ([]byte, error) {
	if s == nil || s.metaOpt == nil {
		return html, nil
	}

	htmlStr := string(html)
	result := s.metaOpt.OptimizeMetaTags(htmlStr, nil)
	if result == nil {
		return html, nil
	}

	optimized := s.metaOpt.BuildOptimizedHTML(htmlStr, result)

	if s.structOpt != nil && pageURL != "" {
		schema := map[string]interface{}{
			"@type":       "WebPage",
			"name":        result.Title,
			"description": result.Description,
			"url":         pageURL,
		}
		schemaHTML, err := s.structOpt.BuildStructuredDataHTML(schema)
		if err == nil && schemaHTML != "" {
			optimized = strings.Replace(optimized, "</head>", schemaHTML+"\n</head>", 1)
		}
	}

	if pageURL != "" {
		canonical := fmt.Sprintf(`<link rel="canonical" href="%s">`, escapeHTML(pageURL))
		if !strings.Contains(optimized, `rel="canonical"`) {
			optimized = strings.Replace(optimized, "</head>", canonical+"\n</head>", 1)
		}
	}

	return []byte(optimized), nil
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
