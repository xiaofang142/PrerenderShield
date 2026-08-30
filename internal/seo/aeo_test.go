package seo

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultAEOConfig(t *testing.T) {
	cfg := DefaultAEOConfig()

	assert.NotNil(t, cfg)
	assert.True(t, cfg.EnableAECrawlerDetection)
	assert.True(t, cfg.EnableAnswerExtraction)
	assert.True(t, cfg.EnableStructuredData)
	assert.Len(t, cfg.SupportedAICrawlers, 8)
	assert.Contains(t, cfg.SupportedAICrawlers, "gptbot")
	assert.Contains(t, cfg.SupportedAICrawlers, "bytespider")
	assert.Equal(t, []string{"summary", "bullet"}, cfg.AnswerFormats)
}

func TestKnownAICrawlers(t *testing.T) {
	assert.NotEmpty(t, KnownAICrawlers)
	for _, c := range KnownAICrawlers {
		assert.NotEmpty(t, c.Name)
		assert.NotEmpty(t, c.Company)
		assert.NotEmpty(t, c.BotToken)
		assert.NotEmpty(t, c.Purpose)
	}
}

func TestIsAICrawler(t *testing.T) {
	tests := []struct {
		name        string
		userAgent   string
		wantNil     bool
		wantName    string
		wantCompany string
	}{
		{
			name:      "empty user agent",
			userAgent: "",
			wantNil:   true,
		},
		{
			name:        "gptbot mixed case",
			userAgent:   "Mozilla/5.0 (compatible; GPTBot/1.0; +https://openai.com/gptbot)",
			wantNil:     false,
			wantName:    "GPTBot",
			wantCompany: "OpenAI",
		},
		{
			name:      "claudebot plain token",
			userAgent: "claudebot",
			wantNil:   false,
			wantName:  "ClaudeBot",
		},
		{
			name:      "google-extended",
			userAgent: "Mozilla/5.0 Google-Extended",
			wantNil:   false,
			wantName:  "Google-Extended",
		},
		{
			name:      "unknown crawler",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0",
			wantNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAICrawler(tt.userAgent)
			if tt.wantNil {
				assert.Nil(t, got)
				return
			}
			assert.NotNil(t, got)
			assert.Equal(t, tt.wantName, got.Name)
			if tt.wantCompany != "" {
				assert.Equal(t, tt.wantCompany, got.Company)
			}
		})
	}
}

func TestExtractAnswer_Empty(t *testing.T) {
	assert.Equal(t, "", ExtractAnswer("", "summary"))
}

func TestExtractAnswer_RemovesNonContentElements(t *testing.T) {
	html := `<html><head><script>var tracking = 1;</script>` +
		`<style>.ad { color: red; }</style></head>` +
		`<body><header>站点导航栏头部</header>` +
		`<nav>导航链接</nav>` +
		`<p>这是正文第一段。</p>` +
		`<footer>版权所有</footer></body></html>`

	got := ExtractAnswer(html, "default")
	assert.NotContains(t, got, "tracking")
	assert.NotContains(t, got, "color")
	assert.NotContains(t, got, "导航链接")
	assert.NotContains(t, got, "版权所有")
	assert.Contains(t, got, "这是正文第一段")
}

func TestExtractAnswer_UnclosedTag(t *testing.T) {
	// 未闭合的 <script> 应触发内部 break，剩余文本仍被提取
	html := `<script>alert(1)<p>剩余正文内容`
	got := ExtractAnswer(html, "default")
	assert.Contains(t, got, "剩余正文内容")
}

func TestExtractAnswer_FormatSummary(t *testing.T) {
	html := "<p>第一句话内容.第二句话内容.第三句话内容.第四句话内容.</p>"
	got := ExtractAnswer(html, "summary")
	assert.Equal(t, "第一句话内容. 第二句话内容. 第三句话内容.", got)
}

func TestExtractAnswer_FormatBullet(t *testing.T) {
	html := "<p>要点一.要点二.要点三.</p>"
	got := ExtractAnswer(html, "bullet")
	assert.Equal(t, "- 要点一\n- 要点二\n- 要点三", got)
}

func TestExtractAnswer_FormatBullet_SkipsBlankSentences(t *testing.T) {
	// 句点之间仅剩空白时，空白句应被过滤
	got := formatAsBullets(" . . ")
	assert.Empty(t, got)
}

func TestExtractAnswer_FormatQA(t *testing.T) {
	got := ExtractAnswer("<p>标准答案内容</p>", "qa")
	assert.Equal(t, "A: 标准答案内容", got)
}

func TestTruncateToSentences(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		maxSentences int
		want         string
	}{
		{
			name:         "more sentences than limit",
			text:         "一.二.三.四.五.",
			maxSentences: 3,
			want:         "一. 二. 三.",
		},
		{
			name:         "fewer sentences than limit",
			text:         "唯一一句话",
			maxSentences: 3,
			want:         "唯一一句话.",
		},
		{
			name:         "exactly at limit",
			text:         "一.二.",
			maxSentences: 2,
			want:         "一. 二.",
		},
		{
			name:         "empty text",
			text:         "",
			maxSentences: 3,
			want:         "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, truncateToSentences(tt.text, tt.maxSentences))
		})
	}
}

func TestFormatAsBullets(t *testing.T) {
	got := formatAsBullets("apple. banana! cherry?")
	lines := strings.Split(got, "\n")
	assert.Equal(t, []string{"- apple", "- banana", "- cherry"}, lines)
}

func TestFormatAsQA(t *testing.T) {
	assert.Equal(t, "A: hello", formatAsQA("  hello  "))
}

func TestStripTags(t *testing.T) {
	assert.Equal(t, "hello world", stripTags("<b>hello</b> <i>world</i>"))
	assert.Equal(t, "", stripTags("<br/><hr>"))
}

func TestCollapseWhitespace(t *testing.T) {
	assert.Equal(t, "a b c", collapseWhitespace("  a\t\nb   c  "))
}
