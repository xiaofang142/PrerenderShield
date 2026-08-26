package prerender

import (
	"fmt"
	"strings"

	"prerender-shield/internal/seo"
)

type SEOInjector struct {
	metaOpt   *seo.MetaTagsOptimizer
	structOpt *seo.StructuredDataOptimizer
	llmOpt    *seo.LLMOptimizer
}

func NewSEOInjector(metaConfig *seo.MetaTagsConfig, structConfig *seo.StructuredDataConfig, llmConfig *seo.LLMConfig) *SEOInjector {
	injector := &SEOInjector{
		metaOpt:   seo.NewMetaTagsOptimizer(metaConfig, nil),
		structOpt: seo.NewStructuredDataOptimizer(structConfig, nil),
	}
	if llmConfig != nil && llmConfig.Enabled {
		injector.llmOpt = seo.NewLLMOptimizer(*llmConfig)
	}
	return injector
}

func (s *SEOInjector) InjectSEOTags(html []byte, pageURL string) ([]byte, error) {
	if s == nil || s.metaOpt == nil {
		return html, nil
	}

	htmlStr := string(html)

	// LLM optimization (if enabled) — runs first, result feeds into rule-based optimizer
	if s.llmOpt != nil {
		if title, err := s.llmOpt.OptimizeTitle(s.metaOpt.OptimizeTitle(htmlStr, nil), ""); err == nil && title != "" {
			htmlStr = replaceTitle(htmlStr, title)
		}
	}

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

func replaceTitle(html, newTitle string) string {
	start := strings.Index(html, "<title>")
	if start < 0 {
		return html
	}
	end := strings.Index(html[start:], "</title>")
	if end < 0 {
		return html
	}
	return html[:start] + "<title>" + newTitle + html[start+end+len("</title>"):]
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
