package seo

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetaTagsOptimizer_OptimizeMetaTags_NoAutoKeywords(t *testing.T) {
	cfg := DefaultMetaTagsConfig()
	cfg.AutoGenerateKeywords = false
	o := NewMetaTagsOptimizer(cfg, nil)

	html := `<html><head><title>这是一个足够长的页面标题示例内容</title></head><body>正文</body></html>`
	target := []string{"seo", "优化"}

	result := o.OptimizeMetaTags(html, target)
	require.NotNil(t, result)
	// 关闭自动生成时，Keywords 应直接使用目标关键词
	assert.Equal(t, target, result.Keywords)
}

func TestMetaTagsOptimizer_AnalyzeDescription_TooLong(t *testing.T) {
	o := NewMetaTagsOptimizer(DefaultMetaTagsConfig(), nil)

	longDesc := strings.Repeat("长", 120) // 360 字节，超过 160 上限
	html := `<html><head><meta name="description" content="` + longDesc + `"></head><body></body></html>`

	analysis := o.analyzeDescription(html, nil)
	require.NotNil(t, analysis)

	found := false
	for _, issue := range analysis.Issues {
		if strings.Contains(issue, "描述太长") {
			found = true
		}
	}
	assert.True(t, found, "should report too-long description, issues=%v", analysis.Issues)
	// 优化后的描述应被追加省略号（truncateString 按 rune 截断，中文字符数未超上限时保留全文）
	assert.Equal(t, strings.Repeat("长", 120)+"...", analysis.Optimized)
}

func TestMetaTagsOptimizer_AnalyzeDescription_HasCTA(t *testing.T) {
	o := NewMetaTagsOptimizer(DefaultMetaTagsConfig(), nil)

	desc := strings.Repeat("了", 60) + "立即免费获取" + strings.Repeat("好", 20)
	html := `<html><head><meta name="description" content="` + desc + `"></head><body></body></html>`

	analysis := o.analyzeDescription(html, nil)
	for _, issue := range analysis.Issues {
		assert.NotContains(t, issue, "号召性用语", "包含 CTA 时不应提示，issues=%v", analysis.Issues)
	}
}

func TestMetaTagsOptimizer_ExtractDescription_AlternateOrder(t *testing.T) {
	o := NewMetaTagsOptimizer(DefaultMetaTagsConfig(), nil)

	// content 属性在 name 属性之前
	html := `<html><head><meta content="备选顺序的描述内容" name="description"></head></html>`
	assert.Equal(t, "备选顺序的描述内容", o.extractDescription(html))
}

func TestMetaTagsOptimizer_GenerateDescriptionFromContent_LongParagraph(t *testing.T) {
	o := NewMetaTagsOptimizer(DefaultMetaTagsConfig(), nil)

	long := strings.Repeat("段", 100) // 300 字节 > 160 上限
	html := `<html><body><p>` + long + `</p></body></html>`

	got := o.generateDescriptionFromContent(html)
	assert.Equal(t, strings.Repeat("段", 100)+"...", got)
}

func TestMetaTagsOptimizer_GenerateDescriptionFromContent_H1Fallback(t *testing.T) {
	o := NewMetaTagsOptimizer(DefaultMetaTagsConfig(), nil)

	// 无 <p> 时回退到 h1
	html := `<html><body><h1>产品主标题</h1><div>内容</div></body></html>`
	got := o.generateDescriptionFromContent(html)
	assert.Equal(t, "产品主标题 - 了解更多详情。", got)
}

func TestMetaTagsOptimizer_GenerateMetaTags_DefaultAuthor(t *testing.T) {
	cfg := DefaultMetaTagsConfig()
	cfg.DefaultAuthor = "测试作者"
	o := NewMetaTagsOptimizer(cfg, nil)

	result := &MetaTagsResult{
		Title:       &TitleAnalysis{Optimized: "标题"},
		Description: &DescriptionAnalysis{Optimized: "描述"},
		MetaTags:    make(map[string]string),
	}

	o.generateMetaTags(result, []string{"kw1", "kw2"})
	assert.Equal(t, "测试作者", result.MetaTags["author"])
	assert.Equal(t, "标题", result.MetaTags["title"])
	assert.Equal(t, "kw1, kw2", result.MetaTags["keywords"])
	assert.Equal(t, "index, follow", result.MetaTags["robots"])
}

func TestMetaTagsOptimizer_BuildOptimizedHTML_ReplaceExisting(t *testing.T) {
	o := NewMetaTagsOptimizer(DefaultMetaTagsConfig(), nil)

	html := `<html><head>` +
		`<meta name="keywords" content="旧关键词">` +
		`<link rel="canonical" href="https://old.example.com/a">` +
		`</head><body></body></html>`

	result := &MetaTagsResult{
		Keywords:     []string{"新关键词一", "新关键词二"},
		CanonicalURL: "https://new.example.com/b",
	}

	got := o.BuildOptimizedHTML(html, result)

	assert.Contains(t, got, `<meta name="keywords" content="新关键词一, 新关键词二">`)
	assert.NotContains(t, got, "旧关键词")
	assert.Contains(t, got, `<link rel="canonical" href="https://new.example.com/b">`)
	assert.NotContains(t, got, "https://old.example.com/a")
}

func TestMetaTagsOptimizer_SetCanonicalURL(t *testing.T) {
	o := NewMetaTagsOptimizer(DefaultMetaTagsConfig(), nil)

	// 不应 panic，并可重复设置
	o.SetCanonicalURL("https://example.com/page")
	o.SetCanonicalURL("")
}
