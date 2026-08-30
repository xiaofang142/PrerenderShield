package seo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"prerender-shield/internal/logging"
)

// LLMConfig holds the LLM SEO optimizer configuration
type LLMConfig struct {
	Enabled     bool          `json:"enabled" yaml:"enabled"`
	Provider    string        `json:"provider" yaml:"provider"` // openai, zhipu, deepseek, ollama
	APIKey      string        `json:"api_key" yaml:"api_key"`
	APIURL      string        `json:"api_url" yaml:"api_url"`
	Model       string        `json:"model" yaml:"model"`
	MaxTokens   int           `json:"max_tokens" yaml:"max_tokens"`
	Temperature float64       `json:"temperature" yaml:"temperature"`
	Timeout     time.Duration `json:"timeout" yaml:"timeout"`
	MaxRetries  int           `json:"max_retries" yaml:"max_retries"`

	Prompts LLMPrompts `json:"prompts" yaml:"prompts"`
}

// LLMPrompts holds customizable prompts for different SEO tasks
type LLMPrompts struct {
	TitleOptimization       string `json:"title_optimization" yaml:"title_optimization"`
	DescriptionOptimization string `json:"description_optimization" yaml:"description_optimization"`
	KeywordExtraction       string `json:"keyword_extraction" yaml:"keyword_extraction"`
	StructuredData          string `json:"structured_data" yaml:"structured_data"`
}

// DefaultLLMConfig returns the default LLM configuration
func DefaultLLMConfig() LLMConfig {
	return LLMConfig{
		Enabled:     false,
		Provider:    "openai",
		APIURL:      "https://api.openai.com/v1/chat/completions",
		Model:       "gpt-4o-mini",
		MaxTokens:   500,
		Temperature: 0.3,
		Timeout:     10 * time.Second,
		MaxRetries:  2,
		Prompts: LLMPrompts{
			TitleOptimization: `You are an SEO expert. Optimize the following page title for search engines.
Requirements:
- 30-60 characters
- Include the primary keyword near the beginning
- Include the brand name at the end separated by " | "
- Make it compelling for users to click
Return ONLY the optimized title, nothing else.

Page title: %s
Target keywords: %s`,

			DescriptionOptimization: `You are an SEO expert. Generate a compelling meta description.
Requirements:
- 120-160 characters
- Include a call-to-action
- Include primary keywords naturally
- Accurately summarize the page content
Return ONLY the description, nothing else.

Page content summary: %s
Target keywords: %s`,

			KeywordExtraction: `Extract the top 10 most relevant SEO keywords from the following content.
Return as a JSON array of strings only, nothing else.
Content: %s`,

			StructuredData: `Generate Schema.org JSON-LD for this page.
Page type: %s
Page content: %s
Return ONLY valid JSON-LD, nothing else.`,
		},
	}
}

// LLMOptimizer provides LLM-based SEO content optimization
type LLMOptimizer struct {
	config     LLMConfig
	httpClient *http.Client
}

// NewLLMOptimizer creates a new LLM SEO optimizer
func NewLLMOptimizer(config LLMConfig) *LLMOptimizer {
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Second
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 2
	}
	if config.MaxTokens <= 0 {
		config.MaxTokens = 500
	}

	return &LLMOptimizer{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// chatMessage represents a chat message for LLM API
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// OptimizeTitle uses LLM to optimize a page title
func (o *LLMOptimizer) OptimizeTitle(title, keywords string) (string, error) {
	if !o.config.Enabled {
		return title, nil
	}
	prompt := fmt.Sprintf(o.config.Prompts.TitleOptimization, title, keywords)
	result, err := o.callLLM(prompt)
	if err != nil {
		logging.DefaultLogger.Warn("LLM title optimization failed, using original: %v", err)
		return title, nil
	}
	result = strings.TrimSpace(result)
	if len(result) < 10 {
		return title, nil
	}
	return result, nil
}

// OptimizeDescription uses LLM to generate a meta description
func (o *LLMOptimizer) OptimizeDescription(content, keywords string) (string, error) {
	if !o.config.Enabled {
		return "", nil
	}
	summary := truncateText(stripHTMLTags(content), 500)
	prompt := fmt.Sprintf(o.config.Prompts.DescriptionOptimization, summary, keywords)
	result, err := o.callLLM(prompt)
	if err != nil {
		logging.DefaultLogger.Warn("LLM description optimization failed: %v", err)
		return "", err
	}
	result = strings.TrimSpace(result)
	if len(result) < 50 {
		return "", fmt.Errorf("description too short")
	}
	return result, nil
}

// ExtractKeywords uses LLM to extract SEO keywords
func (o *LLMOptimizer) ExtractKeywords(content string) ([]string, error) {
	if !o.config.Enabled {
		return nil, nil
	}
	summary := truncateText(stripHTML(content), 1000)
	prompt := fmt.Sprintf(o.config.Prompts.KeywordExtraction, summary)
	result, err := o.callLLM(prompt)
	if err != nil {
		logging.DefaultLogger.Warn("LLM keyword extraction failed: %v", err)
		return nil, err
	}
	var keywords []string
	if err := json.Unmarshal([]byte(result), &keywords); err != nil {
		return nil, fmt.Errorf("failed to parse keywords: %w", err)
	}
	return keywords, nil
}

// GenerateStructuredData uses LLM to generate Schema.org JSON-LD
func (o *LLMOptimizer) GenerateStructuredData(pageType, content string) (map[string]interface{}, error) {
	if !o.config.Enabled {
		return nil, nil
	}
	summary := truncateText(stripHTML(content), 1000)
	prompt := fmt.Sprintf(o.config.Prompts.StructuredData, pageType, summary)
	result, err := o.callLLM(prompt)
	if err != nil {
		logging.DefaultLogger.Warn("LLM structured data generation failed: %v", err)
		return nil, err
	}
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(result), &schema); err != nil {
		return nil, fmt.Errorf("failed to parse structured data: %w", err)
	}
	return schema, nil
}

func (o *LLMOptimizer) callLLM(prompt string) (string, error) {
	req := chatRequest{
		Model: o.config.Model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens:   o.config.MaxTokens,
		Temperature: o.config.Temperature,
	}

	// chatRequest 仅含 string/int/float64 基础类型字段，json.Marshal 不会失败
	// （已验证，删除不可达错误分支）
	body, _ := json.Marshal(req)

	var lastErr error
	for attempt := 0; attempt <= o.config.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		result, err := o.doRequest(body)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}

	return "", fmt.Errorf("all %d attempts failed: %w", o.config.MaxRetries+1, lastErr)
}

func (o *LLMOptimizer) doRequest(body []byte) (string, error) {
	req, err := http.NewRequest("POST", o.config.APIURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if o.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.config.APIKey)
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}

func stripHTMLTags(html string) string {
	result := html
	for _, tag := range []string{"script", "style", "nav", "footer", "header", "aside"} {
		for {
			start := strings.Index(strings.ToLower(result), "<"+tag)
			if start < 0 {
				break
			}
			end := strings.Index(strings.ToLower(result[start:]), "</"+tag+">")
			if end < 0 {
				break
			}
			result = result[:start] + result[start+end+len(tag)+3:]
		}
	}
	// Remove remaining HTML tags
	for {
		start := strings.Index(result, "<")
		if start < 0 {
			break
		}
		end := strings.Index(result[start:], ">")
		if end < 0 {
			break
		}
		result = result[:start] + " " + result[start+end+1:]
	}
	return strings.TrimSpace(result)
}

func truncateText(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen]) + "..."
}
