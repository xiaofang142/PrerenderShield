package prerender

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"prerender-shield/internal/redis"
)

// PreheatWorker 预热执行器
type PreheatWorker struct {
	siteName       string
	redisClient    *redis.Client
	concurrency    int
	crawlerHeaders []string
	wg             sync.WaitGroup
	semaphore      chan struct{}
	ctx            context.Context
	cancel         context.CancelFunc
}

// PreheatConfig 预热配置
type PreheatWorkerConfig struct {
	SiteName       string
	RedisClient    *redis.Client
	Concurrency    int
	CrawlerHeaders []string
}

// NewPreheatWorker 创建新的预热执行器
func NewPreheatWorker(config PreheatWorkerConfig) *PreheatWorker {
	ctx, cancel := context.WithCancel(context.Background())

	// 确保并发数至少为1
	concurrency := config.Concurrency
	if concurrency < 1 {
		concurrency = 5
	}

	// 默认爬虫协议头列表
	defaultHeaders := []string{
		"Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)",
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"Mozilla/5.0 (compatible; Sogou spider/4.0; +http://www.sogou.com/docs/help/webmasters.htm#07)",
		"Mozilla/5.0 (compatible; Bytespider; https://zhanzhang.toutiao.com/)",
		"Mozilla/5.0 (compatible; HaosouSpider; http://www.haosou.com/help/help_3_2.html)",
		"Mozilla/5.0 (compatible; YisouSpider/1.0; http://www.yisou.com/help/webmaster/spider_guide.html)",
		"Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
	}

	// 如果没有提供爬虫协议头，使用默认列表
	crawlerHeaders := config.CrawlerHeaders
	if len(crawlerHeaders) == 0 {
		crawlerHeaders = defaultHeaders
	}

	return &PreheatWorker{
		siteName:       config.SiteName,
		redisClient:    config.RedisClient,
		concurrency:    concurrency,
		crawlerHeaders: crawlerHeaders,
		semaphore:      make(chan struct{}, concurrency),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start 开始预热
func (p *PreheatWorker) Start() error {
	// 设置预热状态为运行中
	if err := p.redisClient.SetPreheatRunning(p.siteName, true); err != nil {
		return fmt.Errorf("failed to set preheat running status: %v", err)
	}

	defer func() {
		// 设置预热状态为已停止
		p.redisClient.SetPreheatRunning(p.siteName, false)
	}()

	// 获取站点的所有URL
	urls, err := p.redisClient.GetURLs(p.siteName)
	if err != nil {
		return fmt.Errorf("failed to get URLs from redis: %v", err)
	}

	if len(urls) == 0 {
		return fmt.Errorf("no URLs found for site %s", p.siteName)
	}

	// 并发执行预热任务
	for _, url := range urls {
		// 检查上下文是否已取消
		select {
		case <-p.ctx.Done():
			return nil
		default:
		}

		p.semaphore <- struct{}{}
		p.wg.Add(1)

		go func(url string) {
			defer func() {
				<-p.semaphore
				p.wg.Done()
			}()

			p.preheatURL(url)
		}(url)
	}

	// 等待所有预热任务完成
	p.wg.Wait()

	return nil
}

// Stop 停止预热
func (p *PreheatWorker) Stop() {
	p.cancel()
}

// PreheatURL 预热单个URL
func (p *PreheatWorker) preheatURL(url string) {
	// 检查上下文是否已取消
	select {
	case <-p.ctx.Done():
		return
	default:
	}

	// 为每个URL使用随机的爬虫协议头
	headerIndex := int(time.Now().UnixNano() % int64(len(p.crawlerHeaders)))
	userAgent := p.crawlerHeaders[headerIndex]

	fmt.Printf("Preheating URL: %s with UA: %s\n", url, userAgent)

	// 发送HTTP请求，模拟爬虫访问
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("Failed to create request for %s: %v\n", url, err)
		p.redisClient.SetURLPreheatStatus(p.siteName, url, "failed", 0)
		return
	}

	// 设置请求头
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.8,en-US;q=0.5,en;q=0.3")
	req.Header.Set("Connection", "keep-alive")

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Failed to preheat %s: %v\n", url, err)
		p.redisClient.SetURLPreheatStatus(p.siteName, url, "failed", 0)
		return
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Failed to read response for %s: %v\n", url, err)
		p.redisClient.SetURLPreheatStatus(p.siteName, url, "failed", 0)
		return
	}

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Preheat failed for %s: status code %d\n", url, resp.StatusCode)
		p.redisClient.SetURLPreheatStatus(p.siteName, url, "failed", 0)
		return
	}

	// 记录预热成功
	cacheSize := int64(len(body))
	if err := p.redisClient.SetURLPreheatStatus(p.siteName, url, "cached", cacheSize); err != nil {
		fmt.Printf("Failed to set preheat status for %s: %v\n", url, err)
	}

	fmt.Printf("Successfully preheated URL: %s (size: %d bytes)\n", url, cacheSize)
}

// PreheatURLs 预热指定的URL列表
func (p *PreheatWorker) PreheatURLs(urls []string) error {
	// 设置预热状态为运行中
	if err := p.redisClient.SetPreheatRunning(p.siteName, true); err != nil {
		return fmt.Errorf("failed to set preheat running status: %v", err)
	}

	defer func() {
		// 设置预热状态为已停止
		p.redisClient.SetPreheatRunning(p.siteName, false)
	}()

	if len(urls) == 0 {
		return fmt.Errorf("no URLs provided for preheat")
	}

	// 并发执行预热任务
	for _, url := range urls {
		// 检查上下文是否已取消
		select {
		case <-p.ctx.Done():
			return nil
		default:
		}

		p.semaphore <- struct{}{}
		p.wg.Add(1)

		go func(url string) {
			defer func() {
				<-p.semaphore
				p.wg.Done()
			}()

			p.preheatURL(url)
		}(url)
	}

	// 等待所有预热任务完成
	p.wg.Wait()

	return nil
}

// GetDefaultCrawlerHeaders 获取默认爬虫协议头列表
func GetDefaultCrawlerHeaders() []string {
	return []string{
		"Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)",
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"Mozilla/5.0 (compatible; Sogou spider/4.0; +http://www.sogou.com/docs/help/webmasters.htm#07)",
		"Mozilla/5.0 (compatible; Bytespider; https://zhanzhang.toutiao.com/)",
		"Mozilla/5.0 (compatible; HaosouSpider; http://www.haosou.com/help/help_3_2.html)",
		"Mozilla/5.0 (compatible; YisouSpider/1.0; http://www.yisou.com/help/webmaster/spider_guide.html)",
		"Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
	}
}

// PreheatURLWithHeaders 预热单个URL，使用指定的请求头
func (p *PreheatWorker) PreheatURLWithHeaders(url string, headers map[string]string) error {
	// 检查上下文是否已取消
	select {
	case <-p.ctx.Done():
		return nil
	default:
	}

	// 创建HTTP请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// 设置默认请求头
	req.Header.Set("User-Agent", headers["User-Agent"])
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.8,en-US;q=0.5,en;q=0.3")
	req.Header.Set("Connection", "keep-alive")

	// 添加自定义请求头
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to preheat URL: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("preheat failed with status code: %d", resp.StatusCode)
	}

	// 记录预热成功
	cacheSize := int64(len(body))
	if err := p.redisClient.SetURLPreheatStatus(p.siteName, url, "cached", cacheSize); err != nil {
		return fmt.Errorf("failed to set preheat status: %v", err)
	}

	return nil
}
