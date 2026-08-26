package push

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// IndexNowClient IndexNow 协议客户端
// 支持 Bing、Yandex、Naver、Seznam 等搜索引擎的即时索引推送
type IndexNowClient struct {
	apiKey  string
	httpCli *http.Client
}

// NewIndexNowClient 创建 IndexNow 客户端
func NewIndexNowClient(apiKey string) *IndexNowClient {
	return &IndexNowClient{
		apiKey: apiKey,
		httpCli: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// indexNowRequest IndexNow API 请求体
type indexNowRequest struct {
	Host        string   `json:"host"`
	Key         string   `json:"key"`
	KeyLocation string   `json:"keyLocation,omitempty"`
	URLList     []string `json:"urlList"`
}

// IndexNowResult IndexNow 推送结果
type IndexNowResult struct {
	StatusCode int
	Message    string
	Success    bool
}

// Push URLs to IndexNow endpoint (Bing)
func (c *IndexNowClient) Push(urls []string, host string) (*IndexNowResult, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("indexnow API key is not configured")
	}
	if len(urls) == 0 {
		return &IndexNowResult{Success: true, Message: "no URLs to push"}, nil
	}

	reqBody := indexNowRequest{
		Host:    host,
		Key:     c.apiKey,
		URLList: urls,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// IndexNow 协议端点
	req, err := http.NewRequest("POST", "https://api.indexnow.org/IndexNow", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	result := &IndexNowResult{StatusCode: resp.StatusCode}

	switch resp.StatusCode {
	case 200:
		result.Success = true
		result.Message = "URLs submitted successfully"
	case 202:
		result.Success = true
		result.Message = "URLs accepted for processing"
	case 422:
		result.Success = false
		result.Message = "invalid request format or key mismatch"
	default:
		result.Success = false
		result.Message = fmt.Sprintf("unexpected status code: %d", resp.StatusCode)
	}

	return result, nil
}
