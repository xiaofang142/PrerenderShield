package seo

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultStructuredDataConfig(t *testing.T) {
	config := DefaultStructuredDataConfig()

	assert.NotNil(t, config)
	assert.Equal(t, true, config.EnableJSONLD)
	assert.Equal(t, false, config.EnableMicrodata)
	assert.Equal(t, "zh-CN", config.DefaultLanguage)
	assert.Greater(t, len(config.EnabledTypes), 0)
}

func TestNewStructuredDataOptimizer(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultStructuredDataConfig()

	optimizer := NewStructuredDataOptimizer(config, logger)

	assert.NotNil(t, optimizer)
	assert.Equal(t, config, optimizer.config)
}

func TestNewStructuredDataOptimizer_NilConfig(t *testing.T) {
	optimizer := NewStructuredDataOptimizer(nil, nil)

	assert.NotNil(t, optimizer)
	assert.Equal(t, true, optimizer.config.EnableJSONLD)
}

func TestStructuredDataOptimizer_detectPageType(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	// 检测文章页面
	articleHTML := `<html><body><article><h1>Test Article</h1><span class="author">By John</span></article></body></html>`
	pageType := optimizer.detectPageType(articleHTML)
	assert.Equal(t, "Article", pageType)

	// 检测产品页面
	productHTML := `<html><body><div class="product"><span itemprop="price">99.99</span></div></body></html>`
	pageType = optimizer.detectPageType(productHTML)
	assert.Equal(t, "Product", pageType)

	// 检测 FAQ 页面
	faqHTML := `<html><body><div class="faq"><details><summary>Question</summary>Answer</details></div></body></html>`
	pageType = optimizer.detectPageType(faqHTML)
	assert.Equal(t, "FAQPage", pageType)

	// 检测本地商家
	businessHTML := `<html><body><div class="business"><span>地址：北京市朝阳区</span><span>电话：123456789</span><span>营业时间</span></div></body></html>`
	pageType = optimizer.detectPageType(businessHTML)
	assert.Equal(t, "LocalBusiness", pageType)

	// 检测组织页面
	orgHTML := `<html><body><div class="about"><h1>关于我们</h1></div></body></html>`
	pageType = optimizer.detectPageType(orgHTML)
	assert.Equal(t, "Organization", pageType)

	// 默认网页
	defaultHTML := `<html><body><div>Some content</div></body></html>`
	pageType = optimizer.detectPageType(defaultHTML)
	assert.Equal(t, "WebPage", pageType)
}

func TestStructuredDataOptimizer_generateArticleSchema(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	data := map[string]interface{}{
		"type":        "Article",
		"id":          "https://example.com/article/1",
		"url":         "https://example.com/article/1",
		"headline":    "Test Article Headline",
		"description": "Test article description",
		"image":       []string{"https://example.com/image.jpg"},
		"author": map[string]interface{}{
			"name": "John Doe",
			"url":  "https://example.com/author/john",
		},
		"publisher": map[string]interface{}{
			"name": "Example Publisher",
			"logo": map[string]interface{}{
				"@type": "ImageObject",
				"url":   "https://example.com/logo.png",
			},
		},
	}

	schema := optimizer.generateArticleSchema(data)

	assert.NotNil(t, schema)
	assert.Equal(t, "https://schema.org", schema["@context"])
	assert.Equal(t, "Article", schema["@type"])
	assert.Equal(t, "Test Article Headline", schema["headline"])
	assert.NotNil(t, schema["author"])
	assert.NotNil(t, schema["publisher"])
}

func TestStructuredDataOptimizer_generateProductSchema(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	data := map[string]interface{}{
		"name":        "Test Product",
		"description": "Test product description",
		"image":       "https://example.com/product.jpg",
		"sku":         "SKU123456",
		"brand":       "Test Brand",
		"offers": map[string]interface{}{
			"price":        99.99,
			"currency":     "CNY",
			"availability": "InStock",
		},
	}

	schema := optimizer.generateProductSchema(data)

	assert.NotNil(t, schema)
	assert.Equal(t, "Product", schema["@type"])
	assert.Equal(t, "Test Product", schema["name"])
	assert.NotNil(t, schema["offers"])
	assert.NotNil(t, schema["brand"])
}

func TestStructuredDataOptimizer_generateOrganizationSchema(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	data := map[string]interface{}{
		"name": "Test Organization",
		"url":  "https://example.com",
		"logo": "https://example.com/logo.png",
		"sameAs": []string{
			"https://facebook.com/test",
			"https://twitter.com/test",
		},
	}

	schema := optimizer.generateOrganizationSchema(data)

	assert.NotNil(t, schema)
	assert.Equal(t, "Organization", schema["@type"])
	assert.Equal(t, "Test Organization", schema["name"])
	assert.Contains(t, schema["sameAs"], "https://facebook.com/test")
}

func TestStructuredDataOptimizer_generateLocalBusinessSchema(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	data := map[string]interface{}{
		"type":        "Restaurant",
		"name":        "Test Restaurant",
		"description": "Best food in town",
		"telephone":   "123-456-7890",
		"address": map[string]interface{}{
			"@type":           "PostalAddress",
			"streetAddress":   "123 Main St",
			"addressLocality": "Beijing",
			"addressCountry":  "CN",
		},
		"openingHours": []string{"Mo-Fr 09:00-17:00"},
		"priceRange":   "$$",
	}

	schema := optimizer.generateLocalBusinessSchema(data)

	assert.NotNil(t, schema)
	assert.Equal(t, "Restaurant", schema["@type"])
	assert.Equal(t, "Test Restaurant", schema["name"])
	assert.NotNil(t, schema["address"])
}

func TestStructuredDataOptimizer_generateFAQSchema(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	data := map[string]interface{}{
		"questions": []map[string]interface{}{
			{
				"question": "What is SEO?",
				"answer":   "SEO stands for Search Engine Optimization.",
			},
			{
				"question": "Why is SEO important?",
				"answer":   "SEO helps your website rank higher in search results.",
			},
		},
	}

	schema := optimizer.generateFAQSchema(data)

	assert.NotNil(t, schema)
	assert.Equal(t, "FAQPage", schema["@type"])
	assert.NotNil(t, schema["mainEntity"])
	mainEntity := schema["mainEntity"].([]map[string]interface{})
	assert.Greater(t, len(mainEntity), 0)
	assert.Equal(t, "What is SEO?", mainEntity[0]["name"])
}

func TestStructuredDataOptimizer_generateBreadcrumbSchema(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	data := map[string]interface{}{
		"items": []map[string]interface{}{
			{"name": "Home", "url": "https://example.com"},
			{"name": "Products", "url": "https://example.com/products"},
			{"name": "Product 1", "url": "https://example.com/products/1"},
		},
	}

	schema := optimizer.generateBreadcrumbSchema(data)

	assert.NotNil(t, schema)
	assert.Equal(t, "BreadcrumbList", schema["@type"])
	assert.NotNil(t, schema["itemListElement"])
	itemList := schema["itemListElement"].([]map[string]interface{})
	assert.Len(t, itemList, 3)
	assert.Equal(t, 1, itemList[0]["position"])
	assert.Equal(t, 2, itemList[1]["position"])
}

func TestStructuredDataOptimizer_generateWebPageSchema(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	data := map[string]interface{}{
		"name":        "Test Page",
		"description": "Test page description",
		"url":         "https://example.com/page",
	}

	schema := optimizer.generateWebPageSchema(data)

	assert.NotNil(t, schema)
	assert.Equal(t, "WebPage", schema["@type"])
	assert.Equal(t, "Test Page", schema["name"])
}

func TestStructuredDataOptimizer_validateStructuredData(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	// 有效的结构化数据
	validSchema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "Article",
		"headline": "Test",
	}
	valid, errs, warns := optimizer.validateStructuredData(validSchema)
	assert.True(t, valid)
	assert.Empty(t, errs)
	assert.Empty(t, warns)

	// 缺少 @context
	invalidSchema := map[string]interface{}{
		"@type": "Article",
	}
	valid, errs, warns = optimizer.validateStructuredData(invalidSchema)
	assert.False(t, valid)
	assert.NotEmpty(t, errs)
	assert.Contains(t, errs[0], "@context")

	// 缺少 @type
	invalidSchema2 := map[string]interface{}{
		"@context": "https://schema.org",
	}
	valid, errs, warns = optimizer.validateStructuredData(invalidSchema2)
	assert.False(t, valid)
	assert.NotEmpty(t, errs)
	assert.Contains(t, errs[0], "@type")
}

func TestStructuredDataOptimizer_OptimizeStructuredData(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	html := `<html><head><title>Test</title></head><body><article><h1>Article</h1></article></body></html>`
	data := map[string]interface{}{
		"headline": "Test Article",
	}

	result := optimizer.OptimizeStructuredData(html, "", data)

	assert.NotNil(t, result)
	assert.Equal(t, "Article", result.DetectedType)
	assert.NotNil(t, result.JSONLD)
	assert.NotEmpty(t, result.JSONLD["@context"])
}

func TestStructuredDataOptimizer_OptimizeStructuredData_ExplicitType(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	data := map[string]interface{}{
		"name": "Test Product",
		"price": 99.99,
	}

	result := optimizer.OptimizeStructuredData("", "Product", data)

	assert.NotNil(t, result)
	assert.Equal(t, "Product", result.DetectedType)
	assert.Equal(t, "Product", result.JSONLD["@type"])
}

func TestStructuredDataOptimizer_GenerateJSONLD(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "Article",
		"headline": "Test",
	}

	jsonLD, err := optimizer.GenerateJSONLD(schema)

	assert.NoError(t, err)
	assert.Contains(t, jsonLD, "@context")
	assert.Contains(t, jsonLD, "https://schema.org")
	assert.Contains(t, jsonLD, "Article")
}

func TestStructuredDataOptimizer_BuildStructuredDataHTML(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "Article",
	}

	html, err := optimizer.BuildStructuredDataHTML(schema)

	assert.NoError(t, err)
	assert.Contains(t, html, `<script type="application/ld+json">`)
	assert.Contains(t, html, "</script>")
}

func TestStructuredDataOptimizer_BuildStructuredDataHTML_Disabled(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &StructuredDataConfig{
		EnableJSONLD: false,
	}
	optimizer := NewStructuredDataOptimizer(config, logger)

	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "Article",
	}

	html, err := optimizer.BuildStructuredDataHTML(schema)

	assert.NoError(t, err)
	assert.Empty(t, html)
}

func TestStructuredDataOptimizer_InjectStructuredData(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	html := `<!DOCTYPE html>
<html>
<head>
	<title>Test</title>
</head>
<body></body>
</html>`

	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "Article",
		"headline": "Injected Article",
	}

	resultHTML, err := optimizer.InjectStructuredData(html, schema)

	assert.NoError(t, err)
	assert.Contains(t, resultHTML, `<script type="application/ld+json">`)
	assert.Contains(t, resultHTML, "Injected Article")
}

func TestStructuredDataOptimizer_InjectStructuredData_NoHead(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	html := `<html><body>Content</body></html>`

	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "Article",
	}

	resultHTML, err := optimizer.InjectStructuredData(html, schema)

	assert.NoError(t, err)
	assert.Contains(t, resultHTML, `<script type="application/ld+json">`)
}

func TestStructuredDataOptimizer_GetConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	config := optimizer.GetConfig()

	assert.NotNil(t, config)
	assert.Equal(t, true, config.EnableJSONLD)
}

func TestStructuredDataOptimizer_validateStructuredData_ProductWarnings(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	// Product 缺少 name
	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "Product",
	}
	valid, errors, warnings := optimizer.validateStructuredData(schema)
	assert.False(t, valid)
	assert.NotEmpty(t, errors)
	assert.Contains(t, errors[0], "name")

	// Product 有 name 但缺少 offers
	schema = map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "Product",
		"name":     "Test Product",
	}
	valid, errors, warnings = optimizer.validateStructuredData(schema)
	assert.True(t, valid)
	assert.NotEmpty(t, warnings)
	assert.Contains(t, warnings[0], "offers")
}

func TestStructuredDataOptimizer_validateStructuredData_ArticleWarnings(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	// Article 缺少 headline
	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "Article",
	}
	valid, _, warnings := optimizer.validateStructuredData(schema)
	assert.True(t, valid)
	assert.NotEmpty(t, warnings)
	assert.Contains(t, warnings[0], "headline")
}

func TestStructuredDataOptimizer_OptimizeStructuredData_ResultValidation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	data := map[string]interface{}{
		"type": "Article",
	}

	result := optimizer.OptimizeStructuredData("", "Article", data)

	assert.NotNil(t, result)
	assert.NotNil(t, result.JSONLD)
	// 应该有 warnings 因为缺少 headline
	assert.GreaterOrEqual(t, len(result.Warnings), 0)
}

func TestStructuredDataOptimizer_InjectStructuredData_ValidJSON(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	html := `<!DOCTYPE html><html><head><title>Test</title></head><body></body></html>`

	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "Organization",
		"name":     "Test Org",
		"url":      "https://example.com",
	}

	resultHTML, err := optimizer.InjectStructuredData(html, schema)

	assert.NoError(t, err)

	// 提取 JSON-LD 并验证
	startIdx := strings.Index(resultHTML, `<script type="application/ld+json">`)
	endIdx := strings.Index(resultHTML, `</script>`)
	assert.Greater(t, endIdx, startIdx)

	jsonStr := resultHTML[startIdx+len(`<script type="application/ld+json">`):endIdx]

	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(jsonStr), &parsed)
	assert.NoError(t, err)
	assert.Equal(t, "Organization", parsed["@type"])
}

func TestStructuredDataOptimizer_generateArticleSchema_AutoDate(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	optimizer := NewStructuredDataOptimizer(nil, logger)

	data := map[string]interface{}{
		"type":     "Article",
		"headline": "Test",
	}

	schema := optimizer.generateArticleSchema(data)

	assert.NotNil(t, schema)
	assert.NotEmpty(t, schema["datePublished"])

	// 验证日期格式
	dateStr := schema["datePublished"].(string)
	_, err := time.Parse(time.RFC3339, dateStr)
	assert.NoError(t, err)
}
