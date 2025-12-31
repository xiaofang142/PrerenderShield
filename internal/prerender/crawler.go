package prerender

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/go-rod/rod"
	"prerender-shield/internal/redis"
)

// Crawler 链接爬取器
type Crawler struct {
	siteName     string
	domain       string
	baseURL      string
	depth        int
	maxDepth     int
	concurrency  int
	redisClient  *redis.Client
	visited      map[string]bool
	visitedMutex sync.Mutex
	wg           sync.WaitGroup
	semaphore    chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
}

// CrawlerConfig 爬取器配置
type CrawlerConfig struct {
	SiteName    string
	Domain      string
	BaseURL     string
	MaxDepth    int
	Concurrency int
	RedisClient *redis.Client
}

// NewCrawler 创建新的链接爬取器
func NewCrawler(config CrawlerConfig) *Crawler {
	ctx, cancel := context.WithCancel(context.Background())

	// 确保最大深度至少为1
	maxDepth := config.MaxDepth
	if maxDepth < 1 {
		maxDepth = 3
	}

	// 确保并发数至少为1
	concurrency := config.Concurrency
	if concurrency < 1 {
		concurrency = 5
	}

	return &Crawler{
		siteName:    config.SiteName,
		domain:      config.Domain,
		baseURL:     config.BaseURL,
		depth:       0,
		maxDepth:    maxDepth,
		concurrency: concurrency,
		redisClient: config.RedisClient,
		visited:     make(map[string]bool),
		semaphore:   make(chan struct{}, concurrency),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start 开始爬取
func (c *Crawler) Start() error {
	// 清空之前的URL记录
	if err := c.redisClient.ClearURLs(c.siteName); err != nil {
		return fmt.Errorf("failed to clear previous URLs: %v", err)
	}

	// 标记起始URL为已访问
	c.markVisited(c.baseURL)

	// 添加到Redis
	if err := c.redisClient.AddURL(c.siteName, c.baseURL); err != nil {
		return fmt.Errorf("failed to add initial URL to redis: %v", err)
	}

	// 开始递归爬取
	c.wg.Add(1)
	go c.crawl(c.baseURL, 0)

	// 等待所有爬取任务完成
	c.wg.Wait()

	return nil
}

// Stop 停止爬取
func (c *Crawler) Stop() {
	c.cancel()
}

// crawl 递归爬取URL
func (c *Crawler) crawl(urlStr string, depth int) {
	defer c.wg.Done()

	// 检查上下文是否已取消
	select {
	case <-c.ctx.Done():
		return
	default:
	}

	// 检查是否达到最大深度
	if depth >= c.maxDepth {
		return
	}

	// 获取浏览器实例
	browser := rod.New().MustConnect()
	defer browser.MustClose()

	// 创建页面
	page := browser.MustPage()

	// 导航到URL
	fmt.Printf("Crawling: %s (depth: %d)\n", urlStr, depth)

	if err := page.Navigate(urlStr); err != nil {
		fmt.Printf("Failed to navigate to %s: %v\n", urlStr, err)
		return
	}

	// 等待页面加载完成
	if err := page.WaitLoad(); err != nil {
		fmt.Printf("Failed to wait for page load %s: %v\n", urlStr, err)
		return
	}

	// 提取所有链接
	links, err := c.extractLinks(page)
	if err != nil {
		fmt.Printf("Failed to extract links from %s: %v\n", urlStr, err)
		return
	}

	// 处理每个链接
	for _, link := range links {
		// 检查上下文是否已取消
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// 检查是否已访问
		if c.isVisited(link) {
			continue
		}

		// 标记为已访问
		c.markVisited(link)

		// 添加到Redis
		if err := c.redisClient.AddURL(c.siteName, link); err != nil {
			fmt.Printf("Failed to add URL to redis %s: %v\n", link, err)
			continue
		}

		// 递归爬取，使用信号量控制并发
		c.semaphore <- struct{}{}
		c.wg.Add(1)
		go func(link string, depth int) {
			defer func() {
				<-c.semaphore
				c.wg.Done()
			}()
			c.crawl(link, depth+1)
		}(link, depth)
	}
}

// extractLinks 从页面中提取所有链接
func (c *Crawler) extractLinks(page *rod.Page) ([]string, error) {
	// 查找所有<a>标签
	elements, err := page.Elements("a")
	if err != nil {
		return nil, err
	}

	var links []string

	for _, elem := range elements {
		// 获取href属性
		href, err := elem.Attribute("href")
		if err != nil {
			continue
		}

		if href == nil || *href == "" {
			continue
		}

		// 解析URL
		fullURL, err := c.resolveURL(*href)
		if err != nil {
			continue
		}

		// 检查是否为目标域名
		if !c.isSameDomain(fullURL) {
			continue
		}

		// 检查是否为有效URL格式
		if !c.isValidURL(fullURL) {
			continue
		}

		// 去重
		links = append(links, fullURL)
	}

	// 自定义去重逻辑
	uniqueLinks := make([]string, 0, len(links))
	seen := make(map[string]bool)
	for _, link := range links {
		if !seen[link] {
			seen[link] = true
			uniqueLinks = append(uniqueLinks, link)
		}
	}

	return uniqueLinks, nil
}

// resolveURL 解析相对URL为绝对URL
func (c *Crawler) resolveURL(href string) (string, error) {
	// 处理锚点链接
	if strings.HasPrefix(href, "#") {
		return c.baseURL, nil
	}

	// 处理相对路径
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}

	relative, err := url.Parse(href)
	if err != nil {
		return "", err
	}

	fullURL := base.ResolveReference(relative).String()

	// 移除URL查询参数和片段
	parsed, err := url.Parse(fullURL)
	if err != nil {
		return "", err
	}

	parsed.RawQuery = ""
	parsed.Fragment = ""

	return parsed.String(), nil
}

// isSameDomain 检查URL是否与目标域名相同
func (c *Crawler) isSameDomain(urlStr string) bool {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	return strings.EqualFold(parsed.Hostname(), c.domain)
}

// isValidURL 检查URL是否为有效的HTTP/HTTPS URL
func (c *Crawler) isValidURL(urlStr string) bool {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// 只允许HTTP和HTTPS协议
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}

	// 确保URL有主机名
	if parsed.Hostname() == "" {
		return false
	}

	return true
}

// isVisited 检查URL是否已访问
func (c *Crawler) isVisited(urlStr string) bool {
	c.visitedMutex.Lock()
	defer c.visitedMutex.Unlock()
	return c.visited[urlStr]
}

// markVisited 标记URL为已访问
func (c *Crawler) markVisited(urlStr string) {
	c.visitedMutex.Lock()
	defer c.visitedMutex.Unlock()
	c.visited[urlStr] = true
}
