package prerender

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"prerender-shield/internal/logging"
	"prerender-shield/internal/redis"

	"golang.org/x/net/html"
)

// FetcherFunc 定义获取页面内容的函数类型
type FetcherFunc func(url string) (string, error)

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
	fetcher      FetcherFunc // 用于获取页面内容的函数
}

// CrawlerConfig 爬取器配置
type CrawlerConfig struct {
	SiteName    string
	Domain      string
	BaseURL     string
	MaxDepth    int
	Concurrency int
	RedisClient *redis.Client
	Fetcher     FetcherFunc // 必须提供
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
		fetcher:     config.Fetcher,
	}
}

// Start 开始爬取
func (c *Crawler) Start() error {
	// 如果没有提供Fetcher，报错
	if c.fetcher == nil {
		return fmt.Errorf("fetcher is required for crawler")
	}

	// 清空之前的URL记录
	if err := c.redisClient.ClearURLs(c.siteName); err != nil {
		return fmt.Errorf("failed to clear previous URLs: %v", err)
	}

	// 标记起始URL为已访问
	c.markVisited(c.baseURL)

	// 提取初始URL的路由部分
	initialRoute := c.extractRoute(c.baseURL)

	// 添加到Redis，只存储路由部分
	if err := c.redisClient.AddURL(c.siteName, initialRoute); err != nil {
		return fmt.Errorf("failed to add initial URL to redis: %v", err)
	}
	
	// 设置初始URL的初始状态和更新时间
	if err := c.redisClient.SetURLPreheatStatus(c.siteName, initialRoute, "pending", 0); err != nil {
		// 记录错误但不中断爬取
		logging.DefaultLogger.Warn("Failed to set initial URL preheat status %s: %v", initialRoute, err)
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
	// 添加panic恢复机制，防止单个爬取任务崩溃整个服务
	defer func() {
		if r := recover(); r != nil {
			logging.DefaultLogger.Error("Panic recovered in crawl %s: %v", urlStr, r)
		}
		c.wg.Done()
	}()

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

	// 使用Fetcher获取页面内容
	logging.DefaultLogger.Debug("Fetching %s (depth: %d)", urlStr, depth)
	
	htmlContent, err := c.fetcher(urlStr)
	if err != nil {
		logging.DefaultLogger.Error("Failed to fetch %s: %v", urlStr, err)
		return
	}

	logging.DefaultLogger.Debug("Page HTML length: %d", len(htmlContent))

	// 提取所有链接
	links, err := c.extractLinks(htmlContent)
	if err != nil {
		logging.DefaultLogger.Error("Failed to extract links from %s: %v", urlStr, err)
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

		// 提取URL的路由部分（去除域名）
		route := c.extractRoute(link)
		
		// 添加到Redis，只存储路由部分
		if err := c.redisClient.AddURL(c.siteName, route); err != nil {
			logging.DefaultLogger.Warn("Failed to add URL to redis %s: %v", route, err)
			continue
		}
		
		// 设置URL的初始状态和更新时间
		if err := c.redisClient.SetURLPreheatStatus(c.siteName, route, "pending", 0); err != nil {
			logging.DefaultLogger.Warn("Failed to set URL preheat status %s: %v", route, err)
			// 不中断流程，继续处理
		}

		// 递归爬取，使用信号量控制并发
		c.semaphore <- struct{}{}
		c.wg.Add(1)
		go func(link string, depth int) {
			defer func() {
				if r := recover(); r != nil {
					logging.DefaultLogger.Error("Panic recovered in recursive crawl %s: %v", link, r)
				}
				<-c.semaphore
				c.wg.Done()
			}()
			c.crawl(link, depth+1)
		}(link, depth)
	}
}

// extractLinks 从HTML内容中提取所有链接
func (c *Crawler) extractLinks(htmlContent string) ([]string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	var links []string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					if a.Val != "" {
						links = append(links, a.Val)
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	// 处理链接
	var validLinks []string
	for _, href := range links {
		// 直接使用原始href值，不进行resolveURL处理，以保留完整的hash信息
		fullURL := href

		// 如果是相对路径，解析为绝对URL
		if !strings.HasPrefix(fullURL, "http://") && !strings.HasPrefix(fullURL, "https://") {
			// 处理相对路径
			base, err := url.Parse(c.baseURL)
			if err != nil {
				continue
			}

			relative, err := url.Parse(fullURL)
			if err != nil {
				continue
			}

			fullURL = base.ResolveReference(relative).String()
		}

		// 检查是否为目标域名
		if !c.isSameDomain(fullURL) {
			continue
		}

		// 检查是否为有效URL格式
		if !c.isValidURL(fullURL) {
			continue
		}

		validLinks = append(validLinks, fullURL)
	}

	// 自定义去重逻辑
	uniqueLinks := make([]string, 0, len(validLinks))
	seen := make(map[string]bool)
	for _, link := range validLinks {
		if !seen[link] {
			seen[link] = true
			uniqueLinks = append(uniqueLinks, link)
		}
	}

	return uniqueLinks, nil
}

// resolveURL 解析相对URL为绝对URL
func (c *Crawler) resolveURL(href string) (string, error) {
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

	// 解析完整URL
	parsed, err := url.Parse(fullURL)
	if err != nil {
		return "", err
	}

	// 移除URL查询参数，但保留片段（用于支持hash模式路由）
	parsed.RawQuery = ""

	return parsed.String(), nil
}

// isSameDomain 检查URL是否与目标域名相同
func (c *Crawler) isSameDomain(urlStr string) bool {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// 比较完整的host（包含端口号）或仅主机名
	targetURL, err := url.Parse(fmt.Sprintf("http://%s", c.domain))
	if err != nil {
		// 如果目标域名解析失败，尝试仅比较主机名
		return strings.EqualFold(parsed.Hostname(), c.domain)
	}

	// 如果目标URL有端口号，比较完整的host
	if targetURL.Port() != "" {
		return strings.EqualFold(parsed.Host, c.domain)
	}

	// 否则仅比较主机名
	return strings.EqualFold(parsed.Hostname(), targetURL.Hostname())
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

// extractRoute 从URL中提取路由部分（去除域名）
func (c *Crawler) extractRoute(urlStr string) string {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return urlStr
	}
	
	// 保留路由部分，包括path、rawquery和fragment
	route := parsed.EscapedPath()
	if parsed.RawQuery != "" {
		route += "?" + parsed.RawQuery
	}
	if parsed.Fragment != "" {
		route += "#" + parsed.Fragment
	}
	
	// 确保路由以/开头
	if !strings.HasPrefix(route, "/") {
		route = "/" + route
	}
	
	return route
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
