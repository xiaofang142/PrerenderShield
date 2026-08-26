package seo

import (
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// StructuredDataOptimizer 结构化数据优化器
type StructuredDataOptimizer struct {
	config *StructuredDataConfig
	logger *zap.Logger
	mu     sync.RWMutex
}

// StructuredDataConfig 结构化数据配置
type StructuredDataConfig struct {
	EnableJSONLD       bool     `json:"enable_json_ld"`       // 启用 JSON-LD
	EnableMicrodata    bool     `json:"enable_microdata"`     // 启用微数据
	EnableRDFa         bool     `json:"enable_rdfa"`          // 启用 RDFa
	EnabledTypes       []string `json:"enabled_types"`        // 启用的类型
	AutoDetectType     bool     `json:"auto_detect_type"`     // 自动检测页面类型
	IncludeLogo        bool     `json:"include_logo"`         // 包含 Logo
	IncludeSocialLinks bool     `json:"include_social_links"` // 包含社交链接
	DefaultLanguage    string   `json:"default_language"`     // 默认语言
}

// DefaultStructuredDataConfig 返回默认配置
func DefaultStructuredDataConfig() *StructuredDataConfig {
	return &StructuredDataConfig{
		EnableJSONLD:       true,
		EnableMicrodata:    false, // JSON-LD 是推荐格式
		EnableRDFa:         false,
		EnabledTypes:       []string{"Article", "Product", "Organization", "LocalBusiness", "FAQPage", "BreadcrumbList"},
		AutoDetectType:     true,
		IncludeLogo:        true,
		IncludeSocialLinks: true,
		DefaultLanguage:    "zh-CN",
	}
}

// NewStructuredDataOptimizer 创建结构化数据优化器
func NewStructuredDataOptimizer(config *StructuredDataConfig, logger *zap.Logger) *StructuredDataOptimizer {
	if config == nil {
		config = DefaultStructuredDataConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &StructuredDataOptimizer{
		config: config,
		logger: logger,
	}
}

// StructuredDataResult 结构化数据结果
type StructuredDataResult struct {
	JSONLD       map[string]interface{} `json:"json_ld"`       // JSON-LD 格式
	Microdata    string                 `json:"microdata"`     // 微数据格式（HTML）
	DetectedType string                 `json:"detected_type"` // 检测到的页面类型
	Valid        bool                   `json:"valid"`         // 是否有效
	Errors       []string               `json:"errors"`        // 错误列表
	Warnings     []string               `json:"warnings"`      // 警告列表
}

// ArticleSchema Article 类型结构化数据
type ArticleSchema struct {
	Context       string              `json:"@context"`
	Type          string              `json:"@type"`
	ID            string              `json:"@id,omitempty"`
	MainEntity    string              `json:"mainEntityOfPage,omitempty"`
	Headline      string              `json:"headline"`
	Description   string              `json:"description"`
	Image         []string            `json:"image,omitempty"`
	DatePublished string              `json:"datePublished,omitempty"`
	DateModified  string              `json:"dateModified,omitempty"`
	Author        *AuthorSchema       `json:"author,omitempty"`
	Publisher     *OrganizationSchema `json:"publisher,omitempty"`
}

// AuthorSchema 作者
type AuthorSchema struct {
	Type string `json:"@type"`
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// OrganizationSchema 组织
type OrganizationSchema struct {
	Type string             `json:"@type"`
	Name string             `json:"name"`
	Logo *ImageObjectSchema `json:"logo,omitempty"`
	URL  string             `json:"url,omitempty"`
}

// ImageObjectSchema 图片
type ImageObjectSchema struct {
	Type   string `json:"@type"`
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// ProductSchema 产品
type ProductSchema struct {
	Context     string       `json:"@context"`
	Type        string       `json:"@type"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Image       []string     `json:"image,omitempty"`
	Offers      *OfferSchema `json:"offers,omitempty"`
	Brand       *BrandSchema `json:"brand,omitempty"`
	SKU         string       `json:"sku,omitempty"`
	MPN         string       `json:"mpn,omitempty"`
}

// OfferSchema 报价
type OfferSchema struct {
	Type          string  `json:"@type"`
	Price         float64 `json:"price"`
	PriceCurrency string  `json:"priceCurrency"`
	Availability  string  `json:"availability,omitempty"`
	URL           string  `json:"url,omitempty"`
}

// BrandSchema 品牌
type BrandSchema struct {
	Type string `json:"@type"`
	Name string `json:"name"`
}

// LocalBusinessSchema 本地商家
type LocalBusinessSchema struct {
	Context      string               `json:"@context"`
	Type         string               `json:"@type"`
	Name         string               `json:"name"`
	Description  string               `json:"description"`
	Image        []string             `json:"image,omitempty"`
	Telephone    string               `json:"telephone,omitempty"`
	Address      *PostalAddressSchema `json:"address,omitempty"`
	Geo          *GeoSchema           `json:"geo,omitempty"`
	OpeningHours []string             `json:"openingHours,omitempty"`
	PriceRange   string               `json:"priceRange,omitempty"`
}

// PostalAddressSchema 地址
type PostalAddressSchema struct {
	Type            string `json:"@type"`
	StreetAddress   string `json:"streetAddress"`
	AddressLocality string `json:"addressLocality"`
	AddressRegion   string `json:"addressRegion,omitempty"`
	PostalCode      string `json:"postalCode,omitempty"`
	AddressCountry  string `json:"addressCountry"`
}

// GeoSchema 地理位置
type GeoSchema struct {
	Type      string  `json:"@type"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// FAQSchema FAQ 页面
type FAQSchema struct {
	Context    string          `json:"@context"`
	Type       string          `json:"@type"`
	MainEntity []FAQItemSchema `json:"mainEntity"`
}

// FAQItemSchema FAQ 项目
type FAQItemSchema struct {
	Type         string        `json:"@type"`
	Name         string        `json:"name"`
	AcceptAnswer *AnswerSchema `json:"acceptedAnswer"`
}

// AnswerSchema 答案
type AnswerSchema struct {
	Type string `json:"@type"`
	Text string `json:"text"`
}

// BreadcrumbListSchema 面包屑
type BreadcrumbListSchema struct {
	Context         string           `json:"@context"`
	Type            string           `json:"@type"`
	ItemListElement []ListItemSchema `json:"itemListElement"`
}

// ListItemSchema 列表项
type ListItemSchema struct {
	Type     string    `json:"@type"`
	Position int       `json:"position"`
	Item     *ListItem `json:"item"`
}

// ListItem 列表项详情
type ListItem struct {
	Type string `json:"@type"`
	ID   string `json:"@id"`
	Name string `json:"name"`
}

// OrganizationWithSocialMedia 带社交媒体的组织
type OrganizationWithSocialMedia struct {
	Type         string               `json:"@type"`
	Name         string               `json:"name"`
	URL          string               `json:"url"`
	Logo         interface{}          `json:"logo,omitempty"`
	SameAs       []string             `json:"sameAs,omitempty"`
	ContactPoint []ContactPointSchema `json:"contactPoint,omitempty"`
}

// ContactPointSchema 联系点
type ContactPointSchema struct {
	Type              string `json:"@type"`
	Telephone         string `json:"telephone"`
	ContactType       string `json:"contactType"`
	AvailableLanguage string `json:"availableLanguage,omitempty"`
}

// OptimizeStructuredData 优化结构化数据
func (o *StructuredDataOptimizer) OptimizeStructuredData(html string, pageType string, data map[string]interface{}) *StructuredDataResult {
	o.mu.Lock()
	defer o.mu.Unlock()

	result := &StructuredDataResult{
		JSONLD:   make(map[string]interface{}),
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	// 自动检测页面类型
	if pageType == "" && o.config.AutoDetectType {
		pageType = o.detectPageType(html)
	}
	result.DetectedType = pageType

	// 根据页面类型生成结构化数据
	switch pageType {
	case "Article", "BlogPosting", "NewsArticle":
		result.JSONLD = o.generateArticleSchema(data)
	case "Product":
		result.JSONLD = o.generateProductSchema(data)
	case "Organization":
		result.JSONLD = o.generateOrganizationSchema(data)
	case "LocalBusiness":
		result.JSONLD = o.generateLocalBusinessSchema(data)
	case "FAQPage":
		result.JSONLD = o.generateFAQSchema(data)
	case "BreadcrumbList":
		result.JSONLD = o.generateBreadcrumbSchema(data)
	case "WebSite":
		result.JSONLD = o.generateWebSiteSchema(data)
	default:
		// 默认使用 WebPage
		result.JSONLD = o.generateWebPageSchema(data)
	}

	// 验证结构化数据
	result.Valid, result.Errors, result.Warnings = o.validateStructuredData(result.JSONLD)

	return result
}

// detectPageType 检测页面类型
func (o *StructuredDataOptimizer) detectPageType(html string) string {
	htmlLower := strings.ToLower(html)

	// 检测文章页面
	if regexp.MustCompile(`<article`).MatchString(html) ||
		regexp.MustCompile(`<h1[^>]*>.*文章.*</h1>`).MatchString(html) ||
		regexp.MustCompile(`by\s*<span|author`).MatchString(htmlLower) {
		return "Article"
	}

	// 检测产品页面
	if regexp.MustCompile(`product|价格 | 库存|¥|\$`).MatchString(htmlLower) ||
		regexp.MustCompile(`itemprop=["']price["']`).MatchString(html) {
		return "Product"
	}

	// 检测 FAQ 页面
	if regexp.MustCompile(`faq|常见问题|问答`).MatchString(htmlLower) ||
		regexp.MustCompile(`<details[^>]*>.*<summary>`).MatchString(html) {
		return "FAQPage"
	}

	// 检测面包屑
	if regexp.MustCompile(`breadcrumb|面包屑|导航`).MatchString(htmlLower) ||
		regexp.MustCompile(`itemprop=["']breadcrumb["']`).MatchString(html) {
		return "BreadcrumbList"
	}

	// 检测本地商家
	if regexp.MustCompile(`address|电话 | 地址|营业时间`).MatchString(htmlLower) {
		return "LocalBusiness"
	}

	// 检测组织/关于我们
	if regexp.MustCompile(`about|关于我们|公司简介`).MatchString(htmlLower) {
		return "Organization"
	}

	return "WebPage"
}

// generateArticleSchema 生成文章结构化数据
func (o *StructuredDataOptimizer) generateArticleSchema(data map[string]interface{}) map[string]interface{} {
	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    data["type"],
	}

	if v, ok := data["id"]; ok {
		schema["@id"] = v
	}
	if v, ok := data["url"]; ok {
		schema["mainEntityOfPage"] = v
	}
	if v, ok := data["headline"]; ok {
		schema["headline"] = v
	}
	if v, ok := data["description"]; ok {
		schema["description"] = v
	}
	if v, ok := data["image"]; ok {
		if images, ok := v.([]string); ok {
			schema["image"] = images
		} else if img, ok := v.(string); ok {
			schema["image"] = []string{img}
		}
	}
	if v, ok := data["datePublished"]; ok {
		schema["datePublished"] = v
	} else {
		schema["datePublished"] = time.Now().Format(time.RFC3339)
	}
	if v, ok := data["dateModified"]; ok {
		schema["dateModified"] = v
	}

	// 作者
	if author, ok := data["author"].(map[string]interface{}); ok {
		authorSchema := map[string]interface{}{
			"@type": "Person",
			"name":  author["name"],
		}
		if url, ok := author["url"].(string); ok {
			authorSchema["url"] = url
		}
		schema["author"] = authorSchema
	}

	// 发布者
	if publisher, ok := data["publisher"].(map[string]interface{}); ok {
		pubSchema := map[string]interface{}{
			"@type": "Organization",
			"name":  publisher["name"],
		}
		if logo, ok := publisher["logo"].(map[string]interface{}); ok {
			pubSchema["logo"] = logo
		}
		schema["publisher"] = pubSchema
	}

	return schema
}

// generateProductSchema 生成产品结构化数据
func (o *StructuredDataOptimizer) generateProductSchema(data map[string]interface{}) map[string]interface{} {
	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "Product",
	}

	if v, ok := data["name"]; ok {
		schema["name"] = v
	}
	if v, ok := data["description"]; ok {
		schema["description"] = v
	}
	if v, ok := data["image"]; ok {
		if images, ok := v.([]string); ok {
			schema["image"] = images
		} else if img, ok := v.(string); ok {
			schema["image"] = []string{img}
		}
	}
	if v, ok := data["sku"]; ok {
		schema["sku"] = v
	}
	if v, ok := data["brand"]; ok {
		if brand, ok := v.(map[string]interface{}); ok {
			schema["brand"] = brand
		} else if brandName, ok := v.(string); ok {
			schema["brand"] = map[string]interface{}{
				"@type": "Brand",
				"name":  brandName,
			}
		}
	}

	// 报价
	if offers, ok := data["offers"].(map[string]interface{}); ok {
		offerSchema := map[string]interface{}{
			"@type": "Offer",
		}
		if price, ok := offers["price"]; ok {
			offerSchema["price"] = price
		}
		if currency, ok := offers["currency"]; ok {
			offerSchema["priceCurrency"] = currency
		}
		if availability, ok := offers["availability"]; ok {
			offerSchema["availability"] = availability
		}
		schema["offers"] = offerSchema
	}

	return schema
}

// generateOrganizationSchema 生成组织结构化数据
func (o *StructuredDataOptimizer) generateOrganizationSchema(data map[string]interface{}) map[string]interface{} {
	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "Organization",
	}

	if v, ok := data["name"]; ok {
		schema["name"] = v
	}
	if v, ok := data["url"]; ok {
		schema["url"] = v
	}
	if v, ok := data["logo"]; ok {
		schema["logo"] = v
	}
	if v, ok := data["sameAs"]; ok {
		if links, ok := v.([]string); ok {
			schema["sameAs"] = links
		}
	}
	if v, ok := data["contactPoint"]; ok {
		schema["contactPoint"] = v
	}

	return schema
}

// generateLocalBusinessSchema 生成本地商家结构化数据
func (o *StructuredDataOptimizer) generateLocalBusinessSchema(data map[string]interface{}) map[string]interface{} {
	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    data["type"],
	}

	if v, ok := data["name"]; ok {
		schema["name"] = v
	}
	if v, ok := data["description"]; ok {
		schema["description"] = v
	}
	if v, ok := data["image"]; ok {
		schema["image"] = v
	}
	if v, ok := data["telephone"]; ok {
		schema["telephone"] = v
	}
	if v, ok := data["address"]; ok {
		schema["address"] = v
	}
	if v, ok := data["geo"]; ok {
		schema["geo"] = v
	}
	if v, ok := data["openingHours"]; ok {
		schema["openingHours"] = v
	}
	if v, ok := data["priceRange"]; ok {
		schema["priceRange"] = v
	}

	return schema
}

// generateFAQSchema 生成 FAQ 结构化数据
func (o *StructuredDataOptimizer) generateFAQSchema(data map[string]interface{}) map[string]interface{} {
	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "FAQPage",
	}

	if questions, ok := data["questions"].([]map[string]interface{}); ok {
		mainEntity := make([]map[string]interface{}, 0, len(questions))
		for _, q := range questions {
			item := map[string]interface{}{
				"@type": "Question",
				"name":  q["question"],
				"acceptedAnswer": map[string]interface{}{
					"@type": "Answer",
					"text":  q["answer"],
				},
			}
			mainEntity = append(mainEntity, item)
		}
		schema["mainEntity"] = mainEntity
	}

	return schema
}

// generateBreadcrumbSchema 生成面包屑结构化数据
func (o *StructuredDataOptimizer) generateBreadcrumbSchema(data map[string]interface{}) map[string]interface{} {
	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "BreadcrumbList",
	}

	if items, ok := data["items"].([]map[string]interface{}); ok {
		itemListElement := make([]map[string]interface{}, 0, len(items))
		for i, item := range items {
			listItem := map[string]interface{}{
				"@type":    "ListItem",
				"position": i + 1,
				"item": map[string]interface{}{
					"@id":  item["url"],
					"name": item["name"],
				},
			}
			itemListElement = append(itemListElement, listItem)
		}
		schema["itemListElement"] = itemListElement
	}

	return schema
}

// generateWebSiteSchema 生成网站结构化数据
func (o *StructuredDataOptimizer) generateWebSiteSchema(data map[string]interface{}) map[string]interface{} {
	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "WebSite",
	}

	if v, ok := data["name"]; ok {
		schema["name"] = v
	}
	if v, ok := data["url"]; ok {
		schema["url"] = v
	}
	if v, ok := data["description"]; ok {
		schema["description"] = v
	}
	if v, ok := data["publisher"]; ok {
		schema["publisher"] = v
	}

	// 潜在搜索操作
	if v, ok := data["search_url"]; ok {
		schema["potentialAction"] = map[string]interface{}{
			"@type":       "SearchAction",
			"target":      v,
			"query-input": "required name=search_term_string",
		}
	}

	return schema
}

// generateWebPageSchema 生成网页结构化数据
func (o *StructuredDataOptimizer) generateWebPageSchema(data map[string]interface{}) map[string]interface{} {
	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "WebPage",
	}

	if v, ok := data["name"]; ok {
		schema["name"] = v
	}
	if v, ok := data["description"]; ok {
		schema["description"] = v
	}
	if v, ok := data["url"]; ok {
		schema["url"] = v
	}

	return schema
}

// validateStructuredData 验证结构化数据
func (o *StructuredDataOptimizer) validateStructuredData(schema map[string]interface{}) (bool, []string, []string) {
	errors := make([]string, 0)
	warnings := make([]string, 0)

	// 检查必需字段
	if _, ok := schema["@context"]; !ok {
		errors = append(errors, "缺少必需字段：@context")
	}
	if _, ok := schema["@type"]; !ok {
		errors = append(errors, "缺少必需字段：@type")
	}

	// 根据类型检查特定字段
	if schemaType, ok := schema["@type"].(string); ok {
		switch schemaType {
		case "Article", "BlogPosting", "NewsArticle":
			if _, ok := schema["headline"]; !ok {
				warnings = append(warnings, "Article 类型建议包含 headline 字段")
			}
		case "Product":
			if _, ok := schema["name"]; !ok {
				errors = append(errors, "Product 类型必需包含 name 字段")
			}
			if _, ok := schema["offers"]; !ok {
				warnings = append(warnings, "Product 类型建议包含 offers 字段")
			}
		case "LocalBusiness":
			if _, ok := schema["name"]; !ok {
				errors = append(errors, "LocalBusiness 类型必需包含 name 字段")
			}
		}
	}

	valid := len(errors) == 0
	return valid, errors, warnings
}

// GenerateJSONLD 生成 JSON-LD 字符串
func (o *StructuredDataOptimizer) GenerateJSONLD(schema map[string]interface{}) (string, error) {
	jsonBytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// BuildStructuredDataHTML 构建结构化数据 HTML
func (o *StructuredDataOptimizer) BuildStructuredDataHTML(schema map[string]interface{}) (string, error) {
	if !o.config.EnableJSONLD {
		return "", nil
	}

	jsonLD, err := o.GenerateJSONLD(schema)
	if err != nil {
		return "", err
	}

	return `<script type="application/ld+json">` + jsonLD + `</script>`, nil
}

// InjectStructuredData 将结构化数据注入 HTML
func (o *StructuredDataOptimizer) InjectStructuredData(html string, schema map[string]interface{}) (string, error) {
	structuredHTML, err := o.BuildStructuredDataHTML(schema)
	if err != nil {
		return html, err
	}

	if structuredHTML == "" {
		return html, nil
	}

	// 在 </head> 前插入
	if strings.Contains(html, "</head>") {
		html = strings.Replace(html, "</head>", structuredHTML+"</head>", 1)
	} else {
		// 如果没有 head，在开头添加
		html = structuredHTML + html
	}

	return html, nil
}

// GetConfig 获取配置
func (o *StructuredDataOptimizer) GetConfig() *StructuredDataConfig {
	return o.config
}
