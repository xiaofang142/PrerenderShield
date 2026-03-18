package prerender

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockRedisClient 模拟 Redis 客户端
type mockRedisClient struct {
	urls    map[string][]string
	status  map[string]map[string]string // route -> status
	mutex   sync.Mutex
	cleared map[string]bool
}

func newMockRedisClient() *mockRedisClient {
	return &mockRedisClient{
		urls:    make(map[string][]string),
		status:  make(map[string]map[string]string),
		cleared: make(map[string]bool),
	}
}

func (m *mockRedisClient) AddURL(site, url string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.urls[site] == nil {
		m.urls[site] = []string{}
	}
	m.urls[site] = append(m.urls[site], url)
	return nil
}

func (m *mockRedisClient) ClearURLs(site string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.urls[site] = []string{}
	m.cleared[site] = true
	return nil
}

func (m *mockRedisClient) SetURLPreheatStatus(site, route, status string, params ...interface{}) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.status[site] == nil {
		m.status[site] = make(map[string]string)
	}
	m.status[site][route] = status
	return nil
}

func (m *mockRedisClient) GetURLs(site string) ([]string, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.urls[site], nil
}

// 其他需要的方法（返回空实现）
func (m *mockRedisClient) SetPushTask(siteID string, task map[string]interface{}) error {
	return nil
}
func (m *mockRedisClient) GetPushOffset(siteID string) (int64, error) {
	return 0, nil
}
func (m *mockRedisClient) SetPushOffset(siteID string, offset int64) error {
	return nil
}
func (m *mockRedisClient) SetLastPushDate(siteID string, date string) error {
	return nil
}
func (m *mockRedisClient) IncrDailyPushCountWithCount(siteID string, count int) error {
	return nil
}
func (m *mockRedisClient) IncrPushStats(siteID string, stat string) error {
	return nil
}
func (m *mockRedisClient) AddPushLogStruct(siteID string, log interface{}) error {
	return nil
}
func (m *mockRedisClient) GetPushStatsWithURLCounts(siteID string) (map[string]interface{}, error) {
	return nil, nil
}
func (m *mockRedisClient) GetLast15DaysPushCount(siteID string) (map[string]int64, error) {
	return nil, nil
}
func (m *mockRedisClient) GetPushLogs(siteID string, limit, offset int) ([]interface{}, error) {
	return nil, nil
}
func (m *mockRedisClient) Close() error {
	return nil
}

// TestNewCrawler 测试创建爬取器
func TestNewCrawler(t *testing.T) {
	mockRedis := newMockRedisClient()
	fetcher := func(url string) (string, error) {
		return "<html></html>", nil
	}

	config := CrawlerConfig{
		SiteName:    "test.com",
		Domain:      "test.com",
		BaseURL:     "http://test.com",
		MaxDepth:    3,
		Concurrency: 5,
		RedisClient: mockRedis,
		Fetcher:     fetcher,
	}

	crawler := NewCrawler(config)

	assert.NotNil(t, crawler)
	assert.Equal(t, "test.com", crawler.siteName)
	assert.Equal(t, "test.com", crawler.domain)
	assert.Equal(t, "http://test.com", crawler.baseURL)
	assert.Equal(t, 3, crawler.maxDepth)
	assert.Equal(t, 5, crawler.concurrency)
	assert.NotNil(t, crawler.fetcher)
	assert.NotNil(t, crawler.ctx)
	assert.NotNil(t, crawler.cancel)
}

// TestNewCrawler_NilConfig 测试 nil 配置
func TestNewCrawler_NilConfig(t *testing.T) {
	config := CrawlerConfig{
		SiteName: "test.com",
		Domain:   "test.com",
		BaseURL:  "http://test.com",
		Fetcher:  func(url string) (string, error) { return "", nil },
	}

	crawler := NewCrawler(config)
	assert.NotNil(t, crawler)
	// 应该使用默认值
	assert.Equal(t, 3, crawler.maxDepth)
	assert.Equal(t, 5, crawler.concurrency)
}

// TestNewCrawler_DefaultDepth 测试默认深度
func TestNewCrawler_DefaultDepth(t *testing.T) {
	mockRedis := newMockRedisClient()

	config := CrawlerConfig{
		SiteName:    "test.com",
		Domain:      "test.com",
		BaseURL:     "http://test.com",
		MaxDepth:    0, // 无效深度
		Concurrency: 0, // 无效并发
		RedisClient: mockRedis,
		Fetcher:     func(url string) (string, error) { return "", nil },
	}

	crawler := NewCrawler(config)

	assert.Equal(t, 3, crawler.maxDepth) // 默认深度
	assert.Equal(t, 5, crawler.concurrency) // 默认并发
}

// TestNewCrawler_NilFetcher 测试没有提供 Fetcher 的情况
func TestNewCrawler_NilFetcher(t *testing.T) {
	mockRedis := newMockRedisClient()

	config := CrawlerConfig{
		SiteName:    "test.com",
		Domain:      "test.com",
		BaseURL:     "http://test.com",
		RedisClient: mockRedis,
		Fetcher:     nil, // 没有提供 Fetcher
	}

	crawler := NewCrawler(config)
	assert.NotNil(t, crawler)

	// Start 应该返回错误
	err := crawler.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fetcher is required")
}

// TestCrawler_Start_ClearURLsError 测试清空 URL 失败的情况
func TestCrawler_Start_ClearURLsError(t *testing.T) {
	errRedis := &errorRedisClient{addError: true}

	config := CrawlerConfig{
		SiteName:    "test.com",
		Domain:      "test.com",
		BaseURL:     "http://test.com",
		RedisClient: errRedis,
		Fetcher:     func(url string) (string, error) { return "", nil },
	}

	crawler := NewCrawler(config)
	err := crawler.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to clear previous URLs")
}

// errorRedisClient 返回错误的 Redis 客户端
type errorRedisClient struct {
	addError bool
}

func (e *errorRedisClient) ClearURLs(site string) error {
	if e.addError {
		return fmt.Errorf("clear error")
	}
	return nil
}
func (e *errorRedisClient) AddURL(site, url string) error {
	if e.addError {
		return fmt.Errorf("add error")
	}
	return nil
}
func (e *errorRedisClient) SetURLPreheatStatus(site, route, status string, params ...interface{}) error {
	return nil
}
func (e *errorRedisClient) GetURLs(site string) ([]string, error) {
	return nil, nil
}
func (e *errorRedisClient) SetPushTask(siteID string, task map[string]interface{}) error {
	return nil
}
func (e *errorRedisClient) GetPushOffset(siteID string) (int64, error) {
	return 0, nil
}
func (e *errorRedisClient) SetPushOffset(siteID string, offset int64) error {
	return nil
}
func (e *errorRedisClient) SetLastPushDate(siteID string, date string) error {
	return nil
}
func (e *errorRedisClient) IncrDailyPushCountWithCount(siteID string, count int) error {
	return nil
}
func (e *errorRedisClient) IncrPushStats(siteID string, stat string) error {
	return nil
}
func (e *errorRedisClient) AddPushLogStruct(siteID string, log interface{}) error {
	return nil
}
func (e *errorRedisClient) GetPushStatsWithURLCounts(siteID string) (map[string]interface{}, error) {
	return nil, nil
}
func (e *errorRedisClient) GetLast15DaysPushCount(siteID string) (map[string]int64, error) {
	return nil, nil
}
func (e *errorRedisClient) GetPushLogs(siteID string, limit, offset int) ([]interface{}, error) {
	return nil, nil
}
func (e *errorRedisClient) Close() error {
	return nil
}

// TestCrawler_Stop 测试停止爬取
func TestCrawler_Stop(t *testing.T) {
	mockRedis := newMockRedisClient()
	config := CrawlerConfig{
		SiteName:    "test.com",
		Domain:      "test.com",
		BaseURL:     "http://test.com",
		RedisClient: mockRedis,
		Fetcher:     func(url string) (string, error) { return "", nil },
	}

	crawler := NewCrawler(config)
	assert.NotNil(t, crawler.ctx)

	// 调用 Stop 应该取消上下文
	crawler.Stop()

	select {
	case <-crawler.ctx.Done():
		// 正确，上下文已取消
	default:
		t.Error("context should be cancelled after Stop")
	}
}

// TestCrawler_extractLinks 测试提取链接
func TestCrawler_extractLinks(t *testing.T) {
	mockRedis := newMockRedisClient()
	config := CrawlerConfig{
		SiteName:    "test.com",
		Domain:      "test.com",
		BaseURL:     "http://test.com",
		RedisClient: mockRedis,
		Fetcher:     func(url string) (string, error) { return "", nil },
	}

	crawler := NewCrawler(config)

	html := `
	<html>
	<body>
		<a href="http://test.com/page1">Link 1</a>
		<a href="http://test.com/page2">Link 2</a>
		<a href="http://other.com/external">External</a>
		<a href="/relative">Relative</a>
		<a href="">Empty</a>
		<a href="javascript:void(0)">JS</a>
	</body>
	</html>
	`

	links, err := crawler.extractLinks(html)
	assert.NoError(t, err)

	// 应该包含同域名的链接
	assert.GreaterOrEqual(t, len(links), 2)

	// 验证链接包含预期的 URL
	var linkStrings []string
	for _, link := range links {
		linkStrings = append(linkStrings, link)
	}

	// 应该包含 page1 和 page2
	assert.Contains(t, strings.Join(linkStrings, ","), "test.com/page1")
	assert.Contains(t, strings.Join(linkStrings, ","), "test.com/page2")

	// 不应该包含外部域名
	for _, link := range links {
		assert.NotContains(t, link, "other.com")
	}
}

// TestCrawler_extractLinks_EmptyHTML 测试空 HTML
func TestCrawler_extractLinks_EmptyHTML(t *testing.T) {
	mockRedis := newMockRedisClient()
	config := CrawlerConfig{
		SiteName:    "test.com",
		Domain:      "test.com",
		BaseURL:     "http://test.com",
		RedisClient: mockRedis,
		Fetcher:     func(url string) (string, error) { return "", nil },
	}

	crawler := NewCrawler(config)

	links, err := crawler.extractLinks("")
	// 空字符串会被 html.Parse 解析为有效文档（只有根节点）
	assert.NoError(t, err)
	assert.Empty(t, links)
}

// TestCrawler_extractLinks_InvalidHTML 测试无效 HTML
func TestCrawler_extractLinks_InvalidHTML(t *testing.T) {
	mockRedis := newMockRedisClient()
	config := CrawlerConfig{
		SiteName:    "test.com",
		Domain:      "test.com",
		BaseURL:     "http://test.com",
		RedisClient: mockRedis,
		Fetcher:     func(url string) (string, error) { return "", nil },
	}

	crawler := NewCrawler(config)

	links, err := crawler.extractLinks("<html><a href>")
	// html.Parse 对不完整 HTML 有一定容错性
	assert.NoError(t, err)
	assert.Empty(t, links)
}

// TestCrawler_extractRoute 测试提取路由
func TestCrawler_extractRoute(t *testing.T) {
	mockRedis := newMockRedisClient()
	config := CrawlerConfig{
		SiteName:    "test.com",
		Domain:      "test.com",
		BaseURL:     "http://test.com",
		RedisClient: mockRedis,
		Fetcher:     func(url string) (string, error) { return "", nil },
	}

	crawler := NewCrawler(config)

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"with path", "http://test.com/page1", "/page1"},
		{"with query", "http://test.com/page?q=1", "/page?q=1"}, // extractRoute 保留查询参数
		{"with fragment", "http://test.com/page#section", "/page#section"},
		{"with query and fragment", "http://test.com/page?q=1#section", "/page?q=1#section"},
		{"root path", "http://test.com/", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := crawler.extractRoute(tt.url)
			assert.Equal(t, tt.expected, route)
		})
	}
}

// TestCrawler_extractRoute_InvalidURL 测试无效 URL
func TestCrawler_extractRoute_InvalidURL(t *testing.T) {
	mockRedis := newMockRedisClient()
	config := CrawlerConfig{
		SiteName:    "test.com",
		Domain:      "test.com",
		BaseURL:     "http://test.com",
		RedisClient: mockRedis,
		Fetcher:     func(url string) (string, error) { return "", nil },
	}

	crawler := NewCrawler(config)

	route := crawler.extractRoute("://invalid")
	assert.Equal(t, "://invalid", route) // 返回原始字符串
}

// TestCrawler_isSameDomain 测试域名匹配
func TestCrawler_isSameDomain(t *testing.T) {
	mockRedis := newMockRedisClient()
	config := CrawlerConfig{
		SiteName:    "test.com",
		Domain:      "test.com",
		BaseURL:     "http://test.com",
		RedisClient: mockRedis,
		Fetcher:     func(url string) (string, error) { return "", nil },
	}

	crawler := NewCrawler(config)

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"same domain http", "http://test.com/page", true},
		{"same domain https", "https://test.com/page", true},
		{"same domain with port", "http://test.com:8080/page", true}, // 端口不匹配时仍会匹配主机名
		{"different domain", "http://other.com/page", false},
		{"subdomain", "http://sub.test.com/page", false},
		{"invalid url", "://invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := crawler.isSameDomain(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCrawler_isSameDomain_WithPort 测试带端口的域名
func TestCrawler_isSameDomain_WithPort(t *testing.T) {
	mockRedis := newMockRedisClient()
	config := CrawlerConfig{
		SiteName:    "test.com",
		Domain:      "test.com:8080",
		BaseURL:     "http://test.com:8080",
		RedisClient: mockRedis,
		Fetcher:     func(url string) (string, error) { return "", nil },
	}

	crawler := NewCrawler(config)

	assert.True(t, crawler.isSameDomain("http://test.com:8080/page"))
	assert.False(t, crawler.isSameDomain("http://test.com/page"))
}

// TestCrawler_isValidURL 测试 URL 有效性
func TestCrawler_isValidURL(t *testing.T) {
	mockRedis := newMockRedisClient()
	config := CrawlerConfig{
		SiteName:    "test.com",
		Domain:      "test.com",
		BaseURL:     "http://test.com",
		RedisClient: mockRedis,
		Fetcher:     func(url string) (string, error) { return "", nil },
	}

	crawler := NewCrawler(config)

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"valid http", "http://test.com/page", true},
		{"valid https", "https://test.com/page", true},
		{"invalid scheme ftp", "ftp://test.com/file", false},
		{"invalid scheme javascript", "javascript:void(0)", false},
		{"no host", "http://", false},
		{"invalid url", "://invalid", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := crawler.isValidURL(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCrawler_resolveURL 测试 URL 解析
func TestCrawler_resolveURL(t *testing.T) {
	mockRedis := newMockRedisClient()
	config := CrawlerConfig{
		SiteName:    "test.com",
		Domain:      "test.com",
		BaseURL:     "http://test.com",
		RedisClient: mockRedis,
		Fetcher:     func(url string) (string, error) { return "", nil },
	}

	crawler := NewCrawler(config)

	// 测试相对路径解析
	links, err := crawler.extractLinks(`<html><body><a href="/page1">Link</a></body></html>`)
	assert.NoError(t, err)
	assert.Len(t, links, 1)
	assert.Contains(t, links[0], "test.com/page1")
}

// TestCrawler_isVisited 测试访问检查
func TestCrawler_isVisited(t *testing.T) {
	mockRedis := newMockRedisClient()
	config := CrawlerConfig{
		SiteName:    "test.com",
		Domain:      "test.com",
		BaseURL:     "http://test.com",
		RedisClient: mockRedis,
		Fetcher:     func(url string) (string, error) { return "", nil },
	}

	crawler := NewCrawler(config)

	url := "http://test.com/page1"

	// 初始未访问
	assert.False(t, crawler.isVisited(url))

	// 标记为已访问
	crawler.markVisited(url)

	// 检查已访问
	assert.True(t, crawler.isVisited(url))
}

// TestCrawler_Integration 集成测试
func TestCrawler_Integration(t *testing.T) {
	// 创建测试服务器
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Write([]byte(`<html><body><a href="/page1">Page 1</a><a href="/page2">Page 2</a></body></html>`))
		case "/page1":
			w.Write([]byte(`<html><body><a href="/">Home</a></body></html>`))
		case "/page2":
			w.Write([]byte(`<html><body><a href="/">Home</a></body></html>`))
		default:
			http.NotFound(w, r)
		}
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	mockRedis := newMockRedisClient()

	// 创建 Fetcher 函数
	fetcher := func(url string) (string, error) {
		resp, err := http.Get(url)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		buf := make([]byte, 1024*10) // 10KB buffer
		n, err := resp.Body.Read(buf)
		return string(buf[:n]), err
	}

	config := CrawlerConfig{
		SiteName:    "test.com",
		Domain:      strings.TrimPrefix(server.URL, "http://"),
		BaseURL:     server.URL,
		MaxDepth:    2,
		Concurrency: 2,
		RedisClient: mockRedis,
		Fetcher:     fetcher,
	}

	crawler := NewCrawler(config)

	// 创建一个带超时的上下文，防止测试卡住
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	crawler.ctx = ctx

	err := crawler.Start()
	assert.NoError(t, err)

	// 验证 URL 已被添加
	urls, err := mockRedis.GetURLs("test.com")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(urls), 1)
}

// TestCrawler_FetcherError 测试 Fetcher 错误处理
func TestCrawler_FetcherError(t *testing.T) {
	mockRedis := newMockRedisClient()

	// 创建总是返回错误的 Fetcher
	errorFetcher := func(url string) (string, error) {
		return "", fmt.Errorf("fetch error")
	}

	config := CrawlerConfig{
		SiteName:    "test.com",
		Domain:      "test.com",
		BaseURL:     "http://test.com",
		MaxDepth:    2,
		Concurrency: 2,
		RedisClient: mockRedis,
		Fetcher:     errorFetcher,
	}

	crawler := NewCrawler(config)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	crawler.ctx = ctx

	// Start 应该完成（虽然会有错误日志）
	err := crawler.Start()
	assert.NoError(t, err) // Start 本身不会返回 fetch 错误，只会记录日志
}

// TestCrawler_PanicRecovery 测试 panic 恢复
func TestCrawler_PanicRecovery(t *testing.T) {
	mockRedis := newMockRedisClient()

	// 创建会 panic 的 Fetcher
	panicFetcher := func(url string) (string, error) {
		panic("test panic")
	}

	config := CrawlerConfig{
		SiteName:    "test.com",
		Domain:      "test.com",
		BaseURL:     "http://test.com",
		MaxDepth:    1,
		Concurrency: 1,
		RedisClient: mockRedis,
		Fetcher:     panicFetcher,
	}

	crawler := NewCrawler(config)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	crawler.ctx = ctx

	// 应该不会崩溃
	err := crawler.Start()
	assert.NoError(t, err)
}
