package seo

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"go.uber.org/zap"
)

// MetaTagsOptimizer SEO Meta 标签优化器
type MetaTagsOptimizer struct {
	config   *MetaTagsConfig
	logger   *zap.Logger
	keywords []string
	mu       sync.RWMutex
}

// MetaTagsConfig Meta 标签配置
type MetaTagsConfig struct {
	TitleMinLength       int      `json:"title_min_length"`       // 标题最小长度
	TitleMaxLength       int      `json:"title_max_length"`       // 标题最大长度
	DescriptionMinLength int      `json:"description_min_length"` // 描述最小长度
	DescriptionMaxLength int      `json:"description_max_length"` // 描述最大长度
	MaxKeywords          int      `json:"max_keywords"`           // 最大关键词数
	MinKeywordLength     int      `json:"min_keyword_length"`     // 最小关键词长度
	AutoGenerateKeywords bool     `json:"auto_generate_keywords"` // 自动生成关键词
	EnableOpenGraph      bool     `json:"enable_open_graph"`      // 启用 Open Graph
	EnableTwitterCard    bool     `json:"enable_twitter_card"`    // 启用 Twitter Card
	TwitterCardType      string   `json:"twitter_card_type"`      // Twitter Card 类型
	RequiredMetaTags     []string `json:"required_meta_tags"`     // 必需的 Meta 标签
}

// DefaultMetaTagsConfig 返回默认配置
func DefaultMetaTagsConfig() *MetaTagsConfig {
	return &MetaTagsConfig{
		TitleMinLength:       30,
		TitleMaxLength:       60,
		DescriptionMinLength: 120,
		DescriptionMaxLength: 160,
		MaxKeywords:          10,
		MinKeywordLength:     3,
		AutoGenerateKeywords: true,
		EnableOpenGraph:      true,
		EnableTwitterCard:    true,
		TwitterCardType:      "summary_large_image",
		RequiredMetaTags: []string{
			"title",
			"description",
			"viewport",
			"charset",
		},
	}
}

// NewMetaTagsOptimizer 创建 Meta 标签优化器
func NewMetaTagsOptimizer(config *MetaTagsConfig, logger *zap.Logger) *MetaTagsOptimizer {
	if config == nil {
		config = DefaultMetaTagsConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &MetaTagsOptimizer{
		config:   config,
		logger:   logger,
		keywords: make([]string, 0),
	}
}

// TitleAnalysis 标题分析结果
type TitleAnalysis struct {
	Original       string   `json:"original"`
	Optimized      string   `json:"optimized"`
	Length         int      `json:"length"`
	IsOptimal      bool     `json:"is_optimal"`
	Issues         []string `json:"issues"`
	KeywordDensity float64  `json:"keyword_density"`
}

// DescriptionAnalysis 描述分析结果
type DescriptionAnalysis struct {
	Original       string   `json:"original"`
	Optimized      string   `json:"optimized"`
	Length         int      `json:"length"`
	IsOptimal      bool     `json:"is_optimal"`
	Issues         []string `json:"issues"`
	KeywordDensity float64  `json:"keyword_density"`
}

// MetaTagsResult Meta 标签优化结果
type MetaTagsResult struct {
	Title           *TitleAnalysis       `json:"title"`
	Description     *DescriptionAnalysis `json:"description"`
	Keywords        []string             `json:"keywords"`
	MetaTags        map[string]string    `json:"meta_tags"`
	OpenGraph       map[string]string    `json:"open_graph"`
	TwitterCard     map[string]string    `json:"twitter_card"`
	CanonicalURL    string               `json:"canonical_url"`
	MissingTags     []string             `json:"missing_tags"`
	Recommendations []string             `json:"recommendations"`
}

// OptimizeMetaTags 优化 Meta 标签
func (o *MetaTagsOptimizer) OptimizeMetaTags(html string, targetKeywords []string) *MetaTagsResult {
	o.mu.Lock()
	defer o.mu.Unlock()

	result := &MetaTagsResult{
		MetaTags:        make(map[string]string),
		OpenGraph:       make(map[string]string),
		TwitterCard:     make(map[string]string),
		Recommendations: make([]string, 0),
	}

	// 1. 分析并优化标题
	result.Title = o.analyzeTitle(html, targetKeywords)

	// 2. 分析并优化描述
	result.Description = o.analyzeDescription(html, targetKeywords)

	// 3. 提取/生成关键词
	if o.config.AutoGenerateKeywords {
		result.Keywords = o.extractKeywords(html)
	} else {
		result.Keywords = targetKeywords
	}

	// 4. 检测缺失的标签
	result.MissingTags = o.detectMissingTags(html)

	// 5. 生成 Meta 标签
	o.generateMetaTags(result, targetKeywords)

	// 6. 生成 Open Graph 标签
	if o.config.EnableOpenGraph {
		o.generateOpenGraph(result)
	}

	// 7. 生成 Twitter Card 标签
	if o.config.EnableTwitterCard {
		o.generateTwitterCard(result)
	}

	// 8. 生成建议
	o.generateRecommendations(result)

	return result
}

// analyzeTitle 分析并优化标题
func (o *MetaTagsOptimizer) analyzeTitle(html string, keywords []string) *TitleAnalysis {
	title := o.extractTitle(html)

	analysis := &TitleAnalysis{
		Original:  title,
		Optimized: title,
		Length:    len(title),
		Issues:    make([]string, 0),
	}

	// 检查长度
	if analysis.Length < o.config.TitleMinLength {
		analysis.Issues = append(analysis.Issues, fmt.Sprintf("标题太短（%d 字符），建议至少 %d 字符", analysis.Length, o.config.TitleMinLength))
	}
	if analysis.Length > o.config.TitleMaxLength {
		analysis.Issues = append(analysis.Issues, fmt.Sprintf("标题太长（%d 字符），建议不超过 %d 字符", analysis.Length, o.config.TitleMaxLength))
		// 截断标题
		analysis.Optimized = truncateString(title, o.config.TitleMaxLength-3) + "..."
	}

	// 检查是否包含关键词
	for _, kw := range keywords {
		if strings.Contains(strings.ToLower(title), strings.ToLower(kw)) {
			analysis.KeywordDensity++
		}
	}

	// 检查是否在开头包含关键词
	if len(keywords) > 0 && len(title) > 0 {
		firstWord := strings.Fields(title)[0]
		if strings.ToLower(firstWord) == strings.ToLower(keywords[0]) {
			analysis.KeywordDensity += 0.5 // 加分
		}
	}

	// 检查是否包含品牌词（假设最后一个 | 或 - 后面的是品牌）
	if strings.Contains(title, "|") || strings.Contains(title, "-") {
		// 已经有品牌词
	} else if len(title) < o.config.TitleMaxLength-10 {
		// 可以添加品牌词
		analysis.Issues = append(analysis.Issues, "建议在标题末尾添加品牌名")
	}

	analysis.IsOptimal = len(analysis.Issues) == 0 &&
		analysis.Length >= o.config.TitleMinLength &&
		analysis.Length <= o.config.TitleMaxLength

	return analysis
}

// analyzeDescription 分析并优化描述
func (o *MetaTagsOptimizer) analyzeDescription(html string, keywords []string) *DescriptionAnalysis {
	description := o.extractDescription(html)

	analysis := &DescriptionAnalysis{
		Original:  description,
		Optimized: description,
		Length:    len(description),
		Issues:    make([]string, 0),
	}

	if description == "" {
		analysis.Issues = append(analysis.Issues, "缺少 meta description 标签")
		// 尝试从内容生成
		analysis.Optimized = o.generateDescriptionFromContent(html)
		analysis.Length = len(analysis.Optimized)
		return analysis
	}

	// 检查长度
	if analysis.Length < o.config.DescriptionMinLength {
		analysis.Issues = append(analysis.Issues, fmt.Sprintf("描述太短（%d 字符），建议至少 %d 字符", analysis.Length, o.config.DescriptionMinLength))
	}
	if analysis.Length > o.config.DescriptionMaxLength {
		analysis.Issues = append(analysis.Issues, fmt.Sprintf("描述太长（%d 字符），建议不超过 %d 字符", analysis.Length, o.config.DescriptionMaxLength))
		analysis.Optimized = truncateString(description, o.config.DescriptionMaxLength-3) + "..."
	}

	// 计算关键词密度
	wordCount := len(strings.Fields(description))
	if wordCount > 0 {
		for _, kw := range keywords {
			count := strings.Count(strings.ToLower(description), strings.ToLower(kw))
			analysis.KeywordDensity += float64(count) / float64(wordCount) * 100
		}
	}

	// 检查是否包含号召性用语
	ctaWords := []string{"立即", "免费", "开始", "了解", "发现", "获取", "查看"}
	hasCTA := false
	for _, cta := range ctaWords {
		if strings.Contains(description, cta) {
			hasCTA = true
			break
		}
	}
	if !hasCTA {
		analysis.Issues = append(analysis.Issues, "建议添加号召性用语以提高点击率")
	}

	analysis.IsOptimal = len(analysis.Issues) == 0 &&
		analysis.Length >= o.config.DescriptionMinLength &&
		analysis.Length <= o.config.DescriptionMaxLength

	return analysis
}

// extractTitle 提取标题
func (o *MetaTagsOptimizer) extractTitle(html string) string {
	titlePattern := regexp.MustCompile(`(?s)<title[^>]*>(.*?)</title>`)
	match := titlePattern.FindStringSubmatch(html)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

// extractDescription 提取描述
func (o *MetaTagsOptimizer) extractDescription(html string) string {
	descPattern := regexp.MustCompile(`<meta[^>]+name=["']description["'][^>]+content=["']([^"']+)["'][^>]*>`)
	match := descPattern.FindStringSubmatch(html)
	if len(match) > 1 {
		return match[1]
	}

	// 尝试另一种格式
	descPattern2 := regexp.MustCompile(`<meta[^>]+content=["']([^"']+)["'][^>]+name=["']description["'][^>]*>`)
	match = descPattern2.FindStringSubmatch(html)
	if len(match) > 1 {
		return match[1]
	}

	return ""
}

// extractKeywords 从内容中提取关键词
func (o *MetaTagsOptimizer) extractKeywords(html string) []string {
	// 移除 HTML 标签
	textPattern := regexp.MustCompile(`>[^<]+<`)
	textMatches := textPattern.FindAllString(html, -1)

	text := ""
	for _, m := range textMatches {
		text += m[1:len(m)-1] + " "
	}

	// 分词（简化为按非字母数字字符分割）
	wordPattern := regexp.MustCompile(`[a-zA-Z0-9]+`)
	words := wordPattern.FindAllString(text, -1)

	// 统计词频
	wordFreq := make(map[string]int)
	for _, word := range words {
		word = strings.ToLower(word)
		if len(word) >= o.config.MinKeywordLength {
			wordFreq[word]++
		}
	}

	// 按频率排序
	type wordCount struct {
		word  string
		count int
	}
	sorted := make([]wordCount, 0, len(wordFreq))
	for w, c := range wordFreq {
		sorted = append(sorted, wordCount{w, c})
	}

	// 使用 sort.Slice 进行排序（更高效）
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	// 取前 N 个
	keywords := make([]string, 0)
	for i := 0; i < len(sorted) && i < o.config.MaxKeywords; i++ {
		keywords = append(keywords, sorted[i].word)
	}

	return keywords
}

// generateDescriptionFromContent 从内容生成描述
func (o *MetaTagsOptimizer) generateDescriptionFromContent(html string) string {
	// 尝试提取第一个段落
	pPattern := regexp.MustCompile(`(?s)<p[^>]*>(.*?)</p>`)
	match := pPattern.FindStringSubmatch(html)
	if len(match) > 1 {
		content := stripHTML(match[1])
		content = strings.TrimSpace(content)
		if len(content) > o.config.DescriptionMaxLength {
			return truncateString(content, o.config.DescriptionMaxLength-3) + "..."
		}
		if len(content) >= o.config.DescriptionMinLength {
			return content
		}
	}

	// 尝试提取 h1 后的内容
	h1Pattern := regexp.MustCompile(`(?s)<h1[^>]*>(.*?)</h1>`)
	h1Match := h1Pattern.FindStringSubmatch(html)
	if len(h1Match) > 1 {
		content := stripHTML(h1Match[1])
		content = strings.TrimSpace(content)
		return content + " - 了解更多详情。"
	}

	return "欢迎访问我们的网站，了解更多精彩内容。"
}

// detectMissingTags 检测缺失的标签
func (o *MetaTagsOptimizer) detectMissingTags(html string) []string {
	missing := make([]string, 0)

	// 检查 title
	if !regexp.MustCompile(`(?s)<title[^>]*>.*?</title>`).MatchString(html) {
		missing = append(missing, "title")
	}

	// 检查 description
	if !regexp.MustCompile(`<meta[^>]+name=["']description["']`).MatchString(html) {
		missing = append(missing, "description")
	}

	// 检查 viewport
	if !regexp.MustCompile(`<meta[^>]+name=["']viewport["']`).MatchString(html) {
		missing = append(missing, "viewport")
	}

	// 检查 charset
	if !regexp.MustCompile(`<meta[^>]+charset=["']?utf-8["']?`).MatchString(html) {
		missing = append(missing, "charset")
	}

	// 检查 canonical
	if !regexp.MustCompile(`<link[^>]+rel=["']canonical["']`).MatchString(html) {
		missing = append(missing, "canonical")
	}

	// 检查 og:title
	if o.config.EnableOpenGraph {
		if !regexp.MustCompile(`<meta[^>]+property=["']og:title["']`).MatchString(html) {
			missing = append(missing, "og:title")
		}
	}

	return missing
}

// generateMetaTags 生成 Meta 标签
func (o *MetaTagsOptimizer) generateMetaTags(result *MetaTagsResult, keywords []string) {
	// Title
	if result.Title != nil && result.Title.Optimized != "" {
		result.MetaTags["title"] = result.Title.Optimized
	}

	// Description
	if result.Description != nil && result.Description.Optimized != "" {
		result.MetaTags["description"] = result.Description.Optimized
	}

	// Keywords
	if len(keywords) > 0 {
		result.MetaTags["keywords"] = strings.Join(keywords, ", ")
	}

	// Author (如果有)
	result.MetaTags["author"] = "Site Author"

	// Robots
	result.MetaTags["robots"] = "index, follow"
}

// generateOpenGraph 生成 Open Graph 标签
func (o *MetaTagsOptimizer) generateOpenGraph(result *MetaTagsResult) {
	if result.Title != nil {
		result.OpenGraph["og:title"] = result.Title.Optimized
	}

	if result.Description != nil {
		result.OpenGraph["og:description"] = result.Description.Optimized
	}

	result.OpenGraph["og:type"] = "website"
	result.OpenGraph["og:locale"] = "zh_CN"

	// 如果有 canonical URL
	if result.CanonicalURL != "" {
		result.OpenGraph["og:url"] = result.CanonicalURL
	}
}

// generateTwitterCard 生成 Twitter Card 标签
func (o *MetaTagsOptimizer) generateTwitterCard(result *MetaTagsResult) {
	result.TwitterCard["twitter:card"] = o.config.TwitterCardType

	if result.Title != nil {
		result.TwitterCard["twitter:title"] = result.Title.Optimized
	}

	if result.Description != nil {
		result.TwitterCard["twitter:description"] = result.Description.Optimized
	}
}

// generateRecommendations 生成优化建议
func (o *MetaTagsOptimizer) generateRecommendations(result *MetaTagsResult) {
	// 基于标题问题
	if result.Title != nil {
		for _, issue := range result.Title.Issues {
			result.Recommendations = append(result.Recommendations, "标题："+issue)
		}
	}

	// 基于描述问题
	if result.Description != nil {
		for _, issue := range result.Description.Issues {
			result.Recommendations = append(result.Recommendations, "描述："+issue)
		}
	}

	// 基于缺失标签
	for _, tag := range result.MissingTags {
		result.Recommendations = append(result.Recommendations, "缺失标签：建议添加 "+tag)
	}

	// 关键词建议
	if len(result.Keywords) == 0 {
		result.Recommendations = append(result.Recommendations, "关键词：建议添加页面关键词")
	}
}

// OptimizeTitle 单独优化标题
func (o *MetaTagsOptimizer) OptimizeTitle(html string, keywords []string) string {
	analysis := o.analyzeTitle(html, keywords)
	return analysis.Optimized
}

// OptimizeDescription 单独优化描述
func (o *MetaTagsOptimizer) OptimizeDescription(html string, keywords []string) string {
	analysis := o.analyzeDescription(html, keywords)
	return analysis.Optimized
}

// GenerateKeywords 生成关键词
func (o *MetaTagsOptimizer) GenerateKeywords(html string) []string {
	return o.extractKeywords(html)
}

// BuildOptimizedHTML 构建优化后的 HTML
func (o *MetaTagsOptimizer) BuildOptimizedHTML(html string, result *MetaTagsResult) string {
	// 优化标题
	if result.Title != nil && result.Title.Optimized != result.Title.Original {
		titlePattern := regexp.MustCompile(`(?s)<title[^>]*>.*?</title>`)
		newTitle := "<title>" + result.Title.Optimized + "</title>"
		if titlePattern.MatchString(html) {
			html = titlePattern.ReplaceAllString(html, newTitle)
		} else {
			// 在 head 内添加
			html = strings.Replace(html, "<head>", "<head>"+newTitle, 1)
		}
	}

	// 优化/添加描述
	if result.Description != nil {
		descTag := `<meta name="description" content="` + result.Description.Optimized + `">`
		descPattern := regexp.MustCompile(`<meta[^>]+name=["']description["'][^>]*>`)
		if descPattern.MatchString(html) {
			html = descPattern.ReplaceAllString(html, descTag)
		} else {
			html = strings.Replace(html, "<head>", "<head>"+descTag, 1)
		}
	}

	// 添加关键词
	if len(result.Keywords) > 0 {
		keywordTag := `<meta name="keywords" content="` + strings.Join(result.Keywords, ", ") + `">`
		keywordPattern := regexp.MustCompile(`<meta[^>]+name=["']keywords["'][^>]*>`)
		if keywordPattern.MatchString(html) {
			html = keywordPattern.ReplaceAllString(html, keywordTag)
		} else {
			html = strings.Replace(html, "<head>", "<head>"+keywordTag, 1)
		}
	}

	// 添加规范链接
	if result.CanonicalURL != "" {
		canonicalTag := `<link rel="canonical" href="` + result.CanonicalURL + `">`
		canonicalPattern := regexp.MustCompile(`<link[^>]+rel=["']canonical["'][^>]*>`)
		if canonicalPattern.MatchString(html) {
			html = canonicalPattern.ReplaceAllString(html, canonicalTag)
		} else {
			html = strings.Replace(html, "<head>", "<head>"+canonicalTag, 1)
		}
	}

	return html
}

// SetCanonicalURL 设置规范链接
func (o *MetaTagsOptimizer) SetCanonicalURL(url string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	// 用于后续生成
}

// GetConfig 获取配置
func (o *MetaTagsOptimizer) GetConfig() *MetaTagsConfig {
	return o.config
}

// 辅助函数

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen])
	}
	return s
}

func stripHTML(html string) string {
	tagPattern := regexp.MustCompile(`<[^>]+>`)
	return tagPattern.ReplaceAllString(html, "")
}
