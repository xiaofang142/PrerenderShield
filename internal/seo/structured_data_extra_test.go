package seo

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStructuredDataOptimizer_OptimizeStructuredData_AllTypes(t *testing.T) {
	o := NewStructuredDataOptimizer(DefaultStructuredDataConfig(), nil)

	tests := []struct {
		name     string
		pageType string
		data     map[string]interface{}
		wantType string
	}{
		{
			name:     "Product",
			pageType: "Product",
			data:     map[string]interface{}{"name": "产品", "type": "Product"},
			wantType: "Product",
		},
		{
			name:     "Organization",
			pageType: "Organization",
			data:     map[string]interface{}{"name": "公司"},
			wantType: "Organization",
		},
		{
			name:     "LocalBusiness",
			pageType: "LocalBusiness",
			data:     map[string]interface{}{"name": "商店", "type": "LocalBusiness"},
			wantType: "LocalBusiness",
		},
		{
			name:     "FAQPage",
			pageType: "FAQPage",
			data:     map[string]interface{}{},
			wantType: "FAQPage",
		},
		{
			name:     "BreadcrumbList",
			pageType: "BreadcrumbList",
			data:     map[string]interface{}{},
			wantType: "BreadcrumbList",
		},
		{
			name:     "WebSite",
			pageType: "WebSite",
			data:     map[string]interface{}{"name": "站点"},
			wantType: "WebSite",
		},
		{
			name:     "unknown type falls back to WebPage schema",
			pageType: "SomeUnknown",
			data:     map[string]interface{}{},
			wantType: "WebPage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := o.OptimizeStructuredData("<html></html>", tt.pageType, tt.data)
			require.NotNil(t, result)
			assert.Equal(t, tt.pageType, result.DetectedType)
			assert.Equal(t, tt.wantType, result.JSONLD["@type"])
		})
	}
}

func TestStructuredDataOptimizer_DetectPageType_FAQAndBreadcrumb(t *testing.T) {
	o := NewStructuredDataOptimizer(DefaultStructuredDataConfig(), nil)

	assert.Equal(t, "FAQPage", o.detectPageType(`<html><body><h2>常见问题</h2></body></html>`))
	assert.Equal(t, "FAQPage", o.detectPageType(`<html><body><details><summary>Q1</summary></details></body></html>`))
	assert.Equal(t, "BreadcrumbList", o.detectPageType(`<html><body><div class="breadcrumb">首页 / 分类</div></body></html>`))
	assert.Equal(t, "BreadcrumbList", o.detectPageType(`<html><body><span itemprop="breadcrumb">路径</span></body></html>`))
}

func TestStructuredDataOptimizer_GenerateArticleSchema_Variants(t *testing.T) {
	o := NewStructuredDataOptimizer(DefaultStructuredDataConfig(), nil)

	data := map[string]interface{}{
		"type":          "Article",
		"id":            "article-1",
		"url":           "https://example.com/a",
		"headline":      "标题",
		"description":   "描述",
		"image":         "https://example.com/img.png", // 单字符串形式
		"datePublished": "2026-01-01T00:00:00Z",
		"dateModified":  "2026-02-02T00:00:00Z",
	}

	schema := o.generateArticleSchema(data)

	assert.Equal(t, "article-1", schema["@id"])
	assert.Equal(t, "https://example.com/a", schema["mainEntityOfPage"])
	assert.Equal(t, []string{"https://example.com/img.png"}, schema["image"])
	assert.Equal(t, "2026-01-01T00:00:00Z", schema["datePublished"])
	assert.Equal(t, "2026-02-02T00:00:00Z", schema["dateModified"])
}

func TestStructuredDataOptimizer_GenerateArticleSchema_AuthorAndPublisher(t *testing.T) {
	o := NewStructuredDataOptimizer(DefaultStructuredDataConfig(), nil)

	data := map[string]interface{}{
		"type": "Article",
		"author": map[string]interface{}{
			"name": "张三",
			"url":  "https://example.com/zhang",
		},
		"publisher": map[string]interface{}{
			"name": "示例出版社",
			"logo": map[string]interface{}{"url": "https://example.com/logo.png"},
		},
	}

	schema := o.generateArticleSchema(data)

	author, ok := schema["author"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Person", author["@type"])
	assert.Equal(t, "张三", author["name"])
	assert.Equal(t, "https://example.com/zhang", author["url"])

	pub, ok := schema["publisher"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Organization", pub["@type"])
	assert.Equal(t, "示例出版社", pub["name"])
	assert.Equal(t, map[string]interface{}{"url": "https://example.com/logo.png"}, pub["logo"])
}

func TestStructuredDataOptimizer_GenerateProductSchema_Variants(t *testing.T) {
	o := NewStructuredDataOptimizer(DefaultStructuredDataConfig(), nil)

	data := map[string]interface{}{
		"name":        "产品",
		"description": "产品描述",
		"image":       "https://example.com/p.png", // 单字符串
		"sku":         "SKU-1",
		"brand":       map[string]interface{}{"@type": "Brand", "name": "品牌"}, // map 形式
		"offers": map[string]interface{}{
			"price":        99.9,
			"currency":     "CNY",
			"availability": "InStock",
		},
	}

	schema := o.generateProductSchema(data)

	assert.Equal(t, []string{"https://example.com/p.png"}, schema["image"])
	assert.Equal(t, map[string]interface{}{"@type": "Brand", "name": "品牌"}, schema["brand"])

	offers, ok := schema["offers"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Offer", offers["@type"])
	assert.Equal(t, 99.9, offers["price"])
	assert.Equal(t, "CNY", offers["priceCurrency"])
	assert.Equal(t, "InStock", offers["availability"])
}

func TestStructuredDataOptimizer_GenerateOrganizationSchema_SameAs(t *testing.T) {
	o := NewStructuredDataOptimizer(DefaultStructuredDataConfig(), nil)

	data := map[string]interface{}{
		"name": "公司",
		"sameAs": []string{
			"https://twitter.com/example",
			"https://github.com/example",
		},
		"contactPoint": map[string]interface{}{"telephone": "+86-10-00000000"},
	}

	schema := o.generateOrganizationSchema(data)
	assert.Equal(t, []string{"https://twitter.com/example", "https://github.com/example"}, schema["sameAs"])
	assert.Equal(t, map[string]interface{}{"telephone": "+86-10-00000000"}, schema["contactPoint"])
}

func TestStructuredDataOptimizer_GenerateLocalBusinessSchema_Variants(t *testing.T) {
	o := NewStructuredDataOptimizer(DefaultStructuredDataConfig(), nil)

	data := map[string]interface{}{
		"type":         "LocalBusiness",
		"name":         "商家",
		"description":  "商家描述",
		"image":        "https://example.com/shop.png",
		"telephone":    "+86-10-11111111",
		"address":      map[string]interface{}{"streetAddress": "某街 1 号"},
		"geo":          map[string]interface{}{"latitude": 39.9, "longitude": 116.4},
		"openingHours": []string{"Mo-Fr 09:00-18:00"},
		"priceRange":   "￥￥",
	}

	schema := o.generateLocalBusinessSchema(data)

	assert.Equal(t, "LocalBusiness", schema["@type"])
	assert.Equal(t, "商家描述", schema["description"])
	assert.Equal(t, "https://example.com/shop.png", schema["image"])
	assert.Equal(t, "+86-10-11111111", schema["telephone"])
	assert.Equal(t, map[string]interface{}{"streetAddress": "某街 1 号"}, schema["address"])
	assert.Equal(t, map[string]interface{}{"latitude": 39.9, "longitude": 116.4}, schema["geo"])
	assert.Equal(t, []string{"Mo-Fr 09:00-18:00"}, schema["openingHours"])
	assert.Equal(t, "￥￥", schema["priceRange"])
}

func TestStructuredDataOptimizer_GenerateWebSiteSchema(t *testing.T) {
	o := NewStructuredDataOptimizer(DefaultStructuredDataConfig(), nil)

	data := map[string]interface{}{
		"name":        "示例网站",
		"url":         "https://example.com",
		"description": "站点描述",
		"publisher":   map[string]interface{}{"name": "出版方"},
		"search_url":  "https://example.com/search?q={search_term_string}",
	}

	schema := o.generateWebSiteSchema(data)

	assert.Equal(t, "WebSite", schema["@type"])
	assert.Equal(t, "示例网站", schema["name"])
	assert.Equal(t, "https://example.com", schema["url"])
	assert.Equal(t, "站点描述", schema["description"])
	assert.Equal(t, map[string]interface{}{"name": "出版方"}, schema["publisher"])

	action, ok := schema["potentialAction"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "SearchAction", action["@type"])
	assert.Equal(t, "https://example.com/search?q={search_term_string}", action["target"])
	assert.Equal(t, "required name=search_term_string", action["query-input"])
}

func TestStructuredDataOptimizer_GenerateWebSiteSchema_Empty(t *testing.T) {
	o := NewStructuredDataOptimizer(DefaultStructuredDataConfig(), nil)

	schema := o.generateWebSiteSchema(map[string]interface{}{})
	assert.Equal(t, "WebSite", schema["@type"])
	assert.NotContains(t, schema, "potentialAction")
}

func TestStructuredDataOptimizer_ValidateStructuredData_LocalBusinessMissingName(t *testing.T) {
	o := NewStructuredDataOptimizer(DefaultStructuredDataConfig(), nil)

	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "LocalBusiness",
	}

	valid, errs, _ := o.validateStructuredData(schema)
	assert.False(t, valid)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0], "LocalBusiness")
}

func TestStructuredDataOptimizer_GenerateJSONLD_Error(t *testing.T) {
	o := NewStructuredDataOptimizer(DefaultStructuredDataConfig(), nil)

	// chan 无法被 JSON 序列化，应返回错误
	schema := map[string]interface{}{"bad": make(chan int)}

	got, err := o.GenerateJSONLD(schema)
	require.Error(t, err)
	assert.Empty(t, got)
}

func TestStructuredDataOptimizer_BuildStructuredDataHTML_Error(t *testing.T) {
	o := NewStructuredDataOptimizer(DefaultStructuredDataConfig(), nil)

	schema := map[string]interface{}{"bad": make(chan int)}

	got, err := o.BuildStructuredDataHTML(schema)
	require.Error(t, err)
	assert.Empty(t, got)
}

func TestStructuredDataOptimizer_InjectStructuredData_Error(t *testing.T) {
	o := NewStructuredDataOptimizer(DefaultStructuredDataConfig(), nil)

	schema := map[string]interface{}{"bad": make(chan int)}

	got, err := o.InjectStructuredData("<html></html>", schema)
	require.Error(t, err)
	assert.Equal(t, "<html></html>", got)
}

func TestStructuredDataOptimizer_InjectStructuredData_Disabled(t *testing.T) {
	cfg := DefaultStructuredDataConfig()
	cfg.EnableJSONLD = false
	o := NewStructuredDataOptimizer(cfg, nil)

	html := "<html><head></head></html>"
	got, err := o.InjectStructuredData(html, map[string]interface{}{"@type": "WebPage"})
	require.NoError(t, err)
	assert.Equal(t, html, got)
}

func TestStructuredDataOptimizer_OptimizeStructuredData_NoAutoDetect(t *testing.T) {
	cfg := DefaultStructuredDataConfig()
	cfg.AutoDetectType = false
	o := NewStructuredDataOptimizer(cfg, nil)

	// 关闭自动检测且未指定类型时，直接走 default 分支（WebPage）
	result := o.OptimizeStructuredData(strings.Repeat("<p>内容</p>", 5), "", map[string]interface{}{})
	assert.Equal(t, "", result.DetectedType)
	assert.Equal(t, "WebPage", result.JSONLD["@type"])
}

func TestStructuredDataOptimizer_GenerateProductSchema_ImageList(t *testing.T) {
	o := NewStructuredDataOptimizer(DefaultStructuredDataConfig(), nil)

	data := map[string]interface{}{
		"name":  "产品",
		"image": []string{"https://example.com/1.png", "https://example.com/2.png"},
	}

	schema := o.generateProductSchema(data)
	assert.Equal(t, []string{"https://example.com/1.png", "https://example.com/2.png"}, schema["image"])
}
