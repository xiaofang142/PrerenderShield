package seo

import (
	"regexp"
	"strings"
)

// AEOConfig AI 爬虫引擎优化配置
type AEOConfig struct {
	EnableAECrawlerDetection bool     // 启用 AI 爬虫检测
	EnableAnswerExtraction   bool     // 启用 AI 答案提取（为 LLM 提供纯净内容）
	EnableStructuredData     bool     // 启用结构化数据（AI 爬虫优先读取）
	SupportedAICrawlers      []string // 支持的 AI 爬虫列表
	AnswerFormats            []string // 答案格式: summary, qa, bullet
}

// DefaultAEOConfig 默认 AEO 配置
func DefaultAEOConfig() *AEOConfig {
	return &AEOConfig{
		EnableAECrawlerDetection: true,
		EnableAnswerExtraction:   true,
		EnableStructuredData:     true,
		SupportedAICrawlers: []string{
			"gptbot",          // OpenAI ChatGPT
			"claudebot",       // Anthropic Claude
			"perplexitybot",   // Perplexity AI
			"google-extended", // Google AI (Gemini)
			"cohere-ai",       // Cohere
			"facebookbot",     // Meta AI
			"applebot",        // Apple AI
			"bytespider",      // ByteDance
		},
		AnswerFormats: []string{"summary", "bullet"},
	}
}

// AICrawlerInfo AI 爬虫信息
type AICrawlerInfo struct {
	Name     string `json:"name"`
	Company  string `json:"company"`
	BotToken string `json:"bot_token"`
	Purpose  string `json:"purpose"` // training, search, indexing
}

// KnownAICrawlers 已知 AI 爬虫清单
var KnownAICrawlers = []AICrawlerInfo{
	{Name: "GPTBot", Company: "OpenAI", BotToken: "gptbot", Purpose: "training"},
	{Name: "ClaudeBot", Company: "Anthropic", BotToken: "claudebot", Purpose: "training"},
	{Name: "Claude-Web", Company: "Anthropic", BotToken: "claude-web", Purpose: "search"},
	{Name: "PerplexityBot", Company: "Perplexity AI", BotToken: "perplexitybot", Purpose: "search"},
	{Name: "Perplexity-User", Company: "Perplexity AI", BotToken: "perplexity-user", Purpose: "search"},
	{Name: "Google-Extended", Company: "Google (Gemini)", BotToken: "google-extended", Purpose: "training"},
	{Name: "Cohere-AI", Company: "Cohere", BotToken: "cohere-ai", Purpose: "training"},
	{Name: "FacebookBot", Company: "Meta AI", BotToken: "facebookbot", Purpose: "training"},
	{Name: "AppleBot", Company: "Apple", BotToken: "applebot", Purpose: "search"},
	{Name: "Bytespider", Company: "ByteDance", BotToken: "bytespider", Purpose: "training"},
}

// AEResult AI 引擎优化结果
type AEResult struct {
	IsAICrawler     bool              `json:"is_ai_crawler"`
	CrawlerName     string            `json:"crawler_name,omitempty"`
	CrawlerCompany  string            `json:"crawler_company,omitempty"`
	Purpose         string            `json:"purpose,omitempty"`
	ExtractedAnswer string            `json:"extracted_answer,omitempty"`
	StructuredData  map[string]string `json:"structured_data,omitempty"`
}

// IsAICrawler 判断 User-Agent 是否为 AI 爬虫
func IsAICrawler(userAgent string) *AICrawlerInfo {
	if userAgent == "" {
		return nil
	}
	ua := strings.ToLower(userAgent)
	for _, crawler := range KnownAICrawlers {
		if strings.Contains(ua, strings.ToLower(crawler.BotToken)) {
			return &crawler
		}
	}
	return nil
}

// ExtractAnswer 从 HTML 中提取纯净答案（供 AI 爬虫消费）
// 移除导航、广告、页脚等非内容元素
func ExtractAnswer(html string, format string) string {
	if html == "" {
		return ""
	}
	content := html

	for _, tag := range []string{"<script", "<style", "<nav", "<footer", "<header"} {
		endTag := "</" + tag[1:]
		for {
			start := strings.Index(content, tag)
			if start == -1 {
				break
			}
			end := strings.Index(content[start:], endTag)
			if end == -1 {
				break
			}
			content = content[:start] + content[start+end+len(endTag):]
		}
	}

	text := stripTags(content)
	text = collapseWhitespace(text)

	switch format {
	case "summary":
		return truncateToSentences(text, 3)
	case "bullet":
		return formatAsBullets(text)
	case "qa":
		return formatAsQA(text)
	default:
		return text
	}
}

func stripTags(s string) string {
	tagPattern := regexp.MustCompile(`<[^>]+>`)
	return tagPattern.ReplaceAllString(s, "")
}

func collapseWhitespace(s string) string {
	wsPattern := regexp.MustCompile(`\s+`)
	s = wsPattern.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func truncateToSentences(text string, maxSentences int) string {
	sentences := strings.FieldsFunc(text, func(r rune) bool {
		return r == '.' || r == '!' || r == '?'
	})
	if len(sentences) > maxSentences {
		sentences = sentences[:maxSentences]
	}
	result := strings.Join(sentences, ". ")
	if len(result) > 0 && !strings.HasSuffix(result, ".") {
		result += "."
	}
	return result
}

func formatAsBullets(text string) string {
	sentences := strings.FieldsFunc(text, func(r rune) bool {
		return r == '.' || r == '!' || r == '?'
	})
	var bullets []string
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if s != "" {
			bullets = append(bullets, "- "+s)
		}
	}
	return strings.Join(bullets, "\n")
}

func formatAsQA(text string) string {
	return "A: " + strings.TrimSpace(text)
}
