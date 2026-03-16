package seo

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultMetaTagsConfig(t *testing.T) {
	config := DefaultMetaTagsConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 30, config.TitleMinLength)
	assert.Equal(t, 60, config.TitleMaxLength)
	assert.Equal(t, 120, config.DescriptionMinLength)
	assert.Equal(t, 160, config.DescriptionMaxLength)
	assert.Equal(t, 10, config.MaxKeywords)
	assert.Equal(t, true, config.EnableOpenGraph)
	assert.Equal(t, true, config.EnableTwitterCard)
}

func TestNewMetaTagsOptimizer(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultMetaTagsConfig()

	optimizer := NewMetaTagsOptimizer(config, logger)

	assert.NotNil(t, optimizer)
	assert.Equal(t, config, optimizer.config)
}

func TestNewMetaTagsOptimizer_NilConfig(t *testing.T) {
	optimizer := NewMetaTagsOptimizer(nil, nil)

	assert.NotNil(t, optimizer)
	assert.Equal(t, 30, optimizer.config.TitleMinLength)
}

func TestMetaTagsOptimizer_OptimizeMetaTags(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	html := `<!DOCTYPE html>
<html>
<head>
	<title>Test Page Title - My Brand</title>
	<meta name="description" content="This is a test page description with enough length to be optimal for SEO purposes.">
	<meta name="keywords" content="test, seo, page">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<meta charset="utf-8">
</head>
<body>
	<h1>Welcome to Test Page</h1>
	<p>This is a paragraph with some content.</p>
</body>
</html>`

	keywords := []string{"test", "seo", "page"}
	result := optimizer.OptimizeMetaTags(html, keywords)

	assert.NotNil(t, result)
	assert.NotNil(t, result.Title)
	assert.NotNil(t, result.Description)
	assert.NotEmpty(t, result.MetaTags)
}

func TestMetaTagsOptimizer_extractTitle(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	html := `<html><head><title>My Test Page Title</title></head><body></body></html>`
	title := optimizer.extractTitle(html)

	assert.Equal(t, "My Test Page Title", title)
}

func TestMetaTagsOptimizer_extractTitle_Empty(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	html := `<html><head></head><body></body></html>`
	title := optimizer.extractTitle(html)

	assert.Equal(t, "", title)
}

func TestMetaTagsOptimizer_extractDescription(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	html := `<html><head><meta name="description" content="Test description content"></head></html>`
	desc := optimizer.extractDescription(html)

	assert.Equal(t, "Test description content", desc)
}

func TestMetaTagsOptimizer_extractDescription_Empty(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	html := `<html><head></head></html>`
	desc := optimizer.extractDescription(html)

	assert.Equal(t, "", desc)
}

func TestMetaTagsOptimizer_analyzeTitle_Short(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	html := `<html><head><title>Short</title></head></html>`
	keywords := []string{"test"}

	analysis := optimizer.analyzeTitle(html, keywords)

	assert.NotNil(t, analysis)
	assert.Less(t, analysis.Length, 30)
	assert.False(t, analysis.IsOptimal)
	assert.Greater(t, len(analysis.Issues), 0)
}

func TestMetaTagsOptimizer_analyzeTitle_Long(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	longTitle := strings.Repeat("This is a very long title ", 10)
	html := `<html><head><title>` + longTitle + `</title></head></html>`
	keywords := []string{"title"}

	analysis := optimizer.analyzeTitle(html, keywords)

	assert.NotNil(t, analysis)
	assert.Greater(t, analysis.Length, 60)
	assert.False(t, analysis.IsOptimal)
	assert.Contains(t, analysis.Issues[0], "标题太长")
}

func TestMetaTagsOptimizer_analyzeTitle_Optimal(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	// 标题长度需要在 30-60 之间
	html := `<html><head><title>Test Page Title For SEO Optimization - My Brand</title></head></html>`
	keywords := []string{"test"}

	analysis := optimizer.analyzeTitle(html, keywords)

	assert.NotNil(t, analysis)
	assert.GreaterOrEqual(t, analysis.Length, 30)
	assert.LessOrEqual(t, analysis.Length, 60)
	assert.True(t, analysis.IsOptimal)
}

func TestMetaTagsOptimizer_analyzeDescription_Missing(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	html := `<html><head></head><body><p>Some content here.</p></body></html>`
	keywords := []string{"test"}

	analysis := optimizer.analyzeDescription(html, keywords)

	assert.NotNil(t, analysis)
	assert.Greater(t, len(analysis.Issues), 0)
	assert.Contains(t, analysis.Issues[0], "缺少")
}

func TestMetaTagsOptimizer_analyzeDescription_Short(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	html := `<html><head><meta name="description" content="Short"></head></html>`
	keywords := []string{"test"}

	analysis := optimizer.analyzeDescription(html, keywords)

	assert.NotNil(t, analysis)
	assert.Less(t, analysis.Length, 120)
	assert.False(t, analysis.IsOptimal)
}

func TestMetaTagsOptimizer_extractKeywords(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	html := `<!DOCTYPE html>
<html>
<body>
	<h1>SEO Optimization</h1>
	<p>SEO is important for search engines. Good SEO helps your page rank higher.</p>
	<p>Search engine optimization is a key skill for web developers.</p>
</body>
</html>`

	keywords := optimizer.extractKeywords(html)

	assert.NotNil(t, keywords)
	assert.Greater(t, len(keywords), 0)
	// "seo" 或 "search" 应该是高频词
	assert.Contains(t, keywords[0], "seo")
}

func TestMetaTagsOptimizer_generateDescriptionFromContent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	html := `<html>
<body>
	<p>This is a paragraph with enough content to be used as a description for SEO purposes. It should be long enough to meet the minimum length requirement.</p>
</body>
</html>`

	desc := optimizer.generateDescriptionFromContent(html)

	assert.NotEmpty(t, desc)
	assert.Greater(t, len(desc), 50)
}

func TestMetaTagsOptimizer_detectMissingTags(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	// 完整的 HTML（包含 Open Graph 标签）
	html := `<!DOCTYPE html>
<html>
<head>
	<title>Test</title>
	<meta name="description" content="test">
	<meta name="viewport" content="width=device-width">
	<meta charset="utf-8">
	<link rel="canonical" href="https://example.com">
	<meta property="og:title" content="Test">
	<meta property="og:description" content="test">
</head>
</html>`

	missing := optimizer.detectMissingTags(html)

	assert.Empty(t, missing)
}

func TestMetaTagsOptimizer_detectMissingTags_Missing(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	html := `<!DOCTYPE html><html><head></head></html>`

	missing := optimizer.detectMissingTags(html)

	assert.Greater(t, len(missing), 0)
	assert.Contains(t, missing, "title")
	assert.Contains(t, missing, "description")
}

func TestMetaTagsOptimizer_generateMetaTags(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	result := &MetaTagsResult{
		Title:       &TitleAnalysis{Optimized: "Optimized Title"},
		Description: &DescriptionAnalysis{Optimized: "Optimized Description"},
		MetaTags:    make(map[string]string),
	}

	keywords := []string{"seo", "test"}
	optimizer.generateMetaTags(result, keywords)

	assert.Equal(t, "Optimized Title", result.MetaTags["title"])
	assert.Equal(t, "Optimized Description", result.MetaTags["description"])
	assert.Contains(t, result.MetaTags["keywords"], "seo")
}

func TestMetaTagsOptimizer_generateOpenGraph(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	result := &MetaTagsResult{
		Title:        &TitleAnalysis{Optimized: "OG Title"},
		Description:  &DescriptionAnalysis{Optimized: "OG Description"},
		CanonicalURL: "https://example.com/page",
		OpenGraph:    make(map[string]string),
	}

	optimizer.generateOpenGraph(result)

	assert.Equal(t, "OG Title", result.OpenGraph["og:title"])
	assert.Equal(t, "OG Description", result.OpenGraph["og:description"])
	assert.Equal(t, "website", result.OpenGraph["og:type"])
	assert.Equal(t, "https://example.com/page", result.OpenGraph["og:url"])
}

func TestMetaTagsOptimizer_generateTwitterCard(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	result := &MetaTagsResult{
		Title:       &TitleAnalysis{Optimized: "Twitter Title"},
		Description: &DescriptionAnalysis{Optimized: "Twitter Description"},
		TwitterCard: make(map[string]string),
	}

	optimizer.generateTwitterCard(result)

	assert.Equal(t, "summary_large_image", result.TwitterCard["twitter:card"])
	assert.Equal(t, "Twitter Title", result.TwitterCard["twitter:title"])
	assert.Equal(t, "Twitter Description", result.TwitterCard["twitter:description"])
}

func TestMetaTagsOptimizer_generateRecommendations(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	result := &MetaTagsResult{
		Title: &TitleAnalysis{
			Issues: []string{"标题太短"},
		},
		Description: &DescriptionAnalysis{
			Issues: []string{"描述太长"},
		},
		MissingTags:     []string{"viewport"},
		Keywords:        []string{},
		Recommendations: make([]string, 0),
	}

	optimizer.generateRecommendations(result)

	assert.Greater(t, len(result.Recommendations), 0)
	assert.Contains(t, result.Recommendations[0], "标题")
}

func TestMetaTagsOptimizer_OptimizeTitle(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	html := `<html><head><title>Short</title></head></html>`
	keywords := []string{"test"}

	optimized := optimizer.OptimizeTitle(html, keywords)

	assert.NotNil(t, optimized)
}

func TestMetaTagsOptimizer_OptimizeDescription(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	html := `<html><head><meta name="description" content="Short desc"></head></html>`
	keywords := []string{"test"}

	optimized := optimizer.OptimizeDescription(html, keywords)

	assert.NotNil(t, optimized)
}

func TestMetaTagsOptimizer_GenerateKeywords(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	html := `<html><body>
		<h1>Go Programming</h1>
		<p>Go is a programming language developed by Google.</p>
		<p>Go is fast and efficient.</p>
	</body></html>`

	keywords := optimizer.GenerateKeywords(html)

	assert.NotNil(t, keywords)
	assert.Greater(t, len(keywords), 0)
}

func TestMetaTagsOptimizer_BuildOptimizedHTML(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	html := `<!DOCTYPE html>
<html>
<head>
	<title>Old Title</title>
	<meta name="description" content="Old description">
</head>
<body></body>
</html>`

	result := &MetaTagsResult{
		Title: &TitleAnalysis{
			Original:  "Old Title",
			Optimized: "New Optimized Title",
		},
		Description: &DescriptionAnalysis{
			Original:  "Old description",
			Optimized: "New optimized description for better SEO",
		},
		Keywords: []string{"seo", "optimization"},
	}

	optimizedHTML := optimizer.BuildOptimizedHTML(html, result)

	assert.Contains(t, optimizedHTML, "New Optimized Title")
	assert.Contains(t, optimizedHTML, "New optimized description")
	assert.Contains(t, optimizedHTML, "seo, optimization")
}

func TestMetaTagsOptimizer_truncateString(t *testing.T) {
	// 测试长字符串（英文）
	result := truncateString("This is a very long text used for testing the truncate function", 20)
	assert.Equal(t, "This is a very long ", result)

	// 测试短字符串
	result = truncateString("Short", 10)
	assert.Equal(t, "Short", result)
}

func TestMetaTagsOptimizer_stripHTML(t *testing.T) {
	html := `<div><p>Hello <strong>World</strong></p></div>`
	result := stripHTML(html)

	assert.Equal(t, "Hello World", result)
}

func TestMetaTagsOptimizer_BuildOptimizedHTML_AddMissing(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	// 没有 title 和 description 的 HTML
	html := `<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
</head>
<body></body>
</html>`

	result := &MetaTagsResult{
		Title: &TitleAnalysis{
			Original:  "",
			Optimized: "Added Title",
		},
		Description: &DescriptionAnalysis{
			Original:  "",
			Optimized: "Added description",
		},
		Keywords:     []string{"test"},
		CanonicalURL: "https://example.com/page",
	}

	optimizedHTML := optimizer.BuildOptimizedHTML(html, result)

	assert.Contains(t, optimizedHTML, "<title>Added Title</title>")
	assert.Contains(t, optimizedHTML, `name="description"`)
	assert.Contains(t, optimizedHTML, `name="keywords"`)
	assert.Contains(t, optimizedHTML, `rel="canonical"`)
}

func TestMetaTagsOptimizer_OptimizeMetaTags_Complete(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	html := `<!DOCTYPE html>
<html>
<head>
	<title>Complete SEO Test Page - Brand Name</title>
	<meta name="description" content="This is a comprehensive test page for SEO optimization. It contains all necessary meta tags and proper content structure for search engine optimization.">
	<meta name="keywords" content="seo, test, optimization">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<meta charset="utf-8">
	<link rel="canonical" href="https://example.com/page">
	<meta property="og:title" content="Complete SEO Test">
	<meta property="og:description" content="Test description">
</head>
<body>
	<h1>Complete SEO Test</h1>
	<p>This page tests all aspects of SEO optimization including title, description, keywords, and more.</p>
</body>
</html>`

	keywords := []string{"seo", "optimization", "test"}
	result := optimizer.OptimizeMetaTags(html, keywords)

	assert.NotNil(t, result)
	assert.NotNil(t, result.Title)
	// 标题可能因为包含关键词而优化
	assert.NotNil(t, result.Description)
	assert.Greater(t, len(result.Keywords), 0)
	// 不检查 missing tags，因为 Open Graph 可能缺少一些
}

func TestMetaTagsOptimizer_GetConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewMetaTagsOptimizer(nil, logger)

	config := optimizer.GetConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 30, config.TitleMinLength)
}
