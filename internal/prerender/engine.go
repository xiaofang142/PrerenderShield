package prerender

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"prerender-shield/internal/cache"
	"prerender-shield/internal/redis"
)

// RenderOptions 渲染选项
type RenderOptions struct {
	Timeout        time.Duration
	WaitUntil      string
	Headers        map[string]string
	Cookies        []Cookie
	Proxy          string
	BlockResources bool
}

// Cookie Cookie结构
type Cookie struct {
	Name     string
	Value    string
	Domain   string
	Path     string
	Expires  time.Time
	Secure   bool
	HttpOnly bool
	SameSite string
}

// RenderResult 渲染结果
type RenderResult struct {
	HTML       string
	Success    bool
	Error      string
	RenderTime float64
	URL        string
}

// RenderWithCacheResult 带缓存的渲染结果
type RenderWithCacheResult struct {
	Result   RenderResult
	HitCache bool
	CacheTTL int
}

// Engine 渲染引擎接口
type Engine interface {
	Render(url string, timeout time.Duration) ([]byte, error)
	CreatePreheatTask(siteID string, urls []string) (string, error)
	GetPreheatTaskStatus(taskID string) (map[string]interface{}, error)
	ListPreheatTasks(siteID string) ([]map[string]interface{}, error)
	CancelPreheatTask(taskID string) error
	CleanupPreheatTasks() error
	IsCrawlerRequest(userAgent string) bool
	RenderWithContext(c *gin.Context, url string, opts RenderOptions, userAgent string) (RenderWithCacheResult, error)
}

// engine 渲染引擎实现
type engine struct {
	redisClient          *redis.Client
	cacheManager         cache.Manager
	maxConcurrent        int
	concurrencyManager   *ConcurrencyManager
}

// NewEngine 创建新的渲染引擎
func NewEngine(redisClient *redis.Client, cacheManager cache.Manager, maxConcurrent int) Engine {
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}

	// 创建动态并发管理器
	concurrencyManager := NewConcurrencyManager(2, maxConcurrent*2, maxConcurrent)

	return &engine{
		redisClient:        redisClient,
		cacheManager:       cacheManager,
		maxConcurrent:      maxConcurrent,
		concurrencyManager: concurrencyManager,
	}
}

// Render 渲染页面
func (e *engine) Render(url string, timeout time.Duration) ([]byte, error) {
	// 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 创建Chrome实例
	options := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, options...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// 导航到页面
	var html string
	err := chromedp.Run(chromeCtx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("body"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// 等待页面加载完成
			time.Sleep(2 * time.Second)
			return nil
		}),
		chromedp.OuterHTML("html", &html),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to render page: %w", err)
	}

	return []byte(html), nil
}

// CreatePreheatTask 创建预热任务
func (e *engine) CreatePreheatTask(siteID string, urls []string) (string, error) {
	// 生成任务ID
	taskID := uuid.New().String()

	// 创建任务信息
	task := map[string]interface{}{
		"task_id":        taskID,
		"site_id":        siteID,
		"urls":           urls,
		"total_urls":     len(urls),
		"completed_urls": 0,
		"failed_urls":    0,
		"status":         "pending",
		"created_at":     time.Now().Unix(),
		"updated_at":     time.Now().Unix(),
	}

	// 存储任务信息到Redis
	if err := e.redisClient.SaveJSON(fmt.Sprintf("task:preheat:%s", taskID), task, 24*time.Hour); err != nil {
		return "", fmt.Errorf("failed to save task: %w", err)
	}

	// 将任务添加到站点任务列表
	if err := e.redisClient.SetAdd(fmt.Sprintf("site:%s:tasks", siteID), taskID); err != nil {
		return "", fmt.Errorf("failed to add task to site: %w", err)
	}

	// 开始执行任务
	go e.executePreheatTask(taskID)

	return taskID, nil
}

// executePreheatTask 执行预热任务
func (e *engine) executePreheatTask(taskID string) {
	// 获取任务信息
	task := make(map[string]interface{})
	if err := e.redisClient.GetJSON(fmt.Sprintf("task:preheat:%s", taskID), &task); err != nil {
		return
	}

	// 更新任务状态为运行中
	task["status"] = "running"
	task["updated_at"] = time.Now().Unix()
	e.redisClient.SaveJSON(fmt.Sprintf("task:preheat:%s", taskID), task, 24*time.Hour)

	// 提取任务参数
	siteID, _ := task["site_id"].(string)
	urls, _ := task["urls"].([]interface{})

	// 执行渲染任务
	completed := 0
	failed := 0

	for _, urlInterface := range urls {
		url, ok := urlInterface.(string)
		if !ok {
			failed++
			continue
		}

		// 渲染页面
		html, err := e.Render(url, 30*time.Second)
		if err != nil {
			// 更新URL预热状态为失败
			e.redisClient.SetURLPreheatStatus(siteID, url, "failed")
			failed++
			continue
		}

		// 存储渲染结果到缓存
		cacheKey := fmt.Sprintf("prerender:%s", url)
		if err := e.cacheManager.Set(siteID, cacheKey, html, 24*time.Hour); err != nil {
			// 更新URL预热状态为失败
			e.redisClient.SetURLPreheatStatus(siteID, url, "failed")
			failed++
			continue
		}

		// 更新URL预热状态为成功
		e.redisClient.SetURLPreheatStatus(siteID, url, "success")
		completed++

		// 更新任务进度
		task["completed_urls"] = completed
		task["failed_urls"] = failed
		task["updated_at"] = time.Now().Unix()
		task["progress"] = int(float64(completed+failed) / float64(len(urls)) * 100)
		e.redisClient.SaveJSON(fmt.Sprintf("task:preheat:%s", taskID), task, 24*time.Hour)

		// 限制并发
		time.Sleep(1 * time.Second)
	}

	// 更新任务状态为完成
	finalStatus := "completed"
	if failed > 0 {
		finalStatus = "completed_with_errors"
	}

	task["status"] = finalStatus
	task["updated_at"] = time.Now().Unix()
	task["progress"] = 100
	e.redisClient.SaveJSON(fmt.Sprintf("task:preheat:%s", taskID), task, 24*time.Hour)
}

// GetPreheatTaskStatus 获取预热任务状态
func (e *engine) GetPreheatTaskStatus(taskID string) (map[string]interface{}, error) {
	task := make(map[string]interface{})
	if err := e.redisClient.GetJSON(fmt.Sprintf("task:preheat:%s", taskID), &task); err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	return task, nil
}

// ListPreheatTasks 列出站点的预热任务
func (e *engine) ListPreheatTasks(siteID string) ([]map[string]interface{}, error) {
	// 获取站点的任务ID列表
	taskIDs, err := e.redisClient.SetMembers(fmt.Sprintf("site:%s:tasks", siteID))
	if err != nil {
		return nil, fmt.Errorf("failed to get task IDs: %w", err)
	}

	// 获取每个任务的详细信息
	tasks := []map[string]interface{}{}
	for _, taskID := range taskIDs {
		task, err := e.GetPreheatTaskStatus(taskID)
		if err != nil {
			continue
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// CancelPreheatTask 取消预热任务
func (e *engine) CancelPreheatTask(taskID string) error {
	// 获取任务信息
	task := make(map[string]interface{})
	if err := e.redisClient.GetJSON(fmt.Sprintf("task:preheat:%s", taskID), &task); err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// 检查任务状态
	status, _ := task["status"].(string)
	if status == "completed" || status == "completed_with_errors" || status == "cancelled" {
		return fmt.Errorf("task cannot be cancelled")
	}

	// 更新任务状态为取消
	task["status"] = "cancelled"
	task["updated_at"] = time.Now().Unix()

	return e.redisClient.SaveJSON(fmt.Sprintf("task:preheat:%s", taskID), task, 24*time.Hour)
}

// IsCrawlerRequest 检测请求是否来自爬虫
func (e *engine) IsCrawlerRequest(userAgent string) bool {
	// 简单的爬虫检测逻辑
	crawlerKeywords := []string{
		"googlebot",
		"bingbot",
		"baiduspider",
		"yandexbot",
		"sogou",
		"yahoo! slurp",
		"duckduckbot",
		"facebookexternalhit",
		"linkedinbot",
		"twitterbot",
		"pinterest",
		"slackbot",
		"telegrambot",
		"whatsapp",
		"embed",
		"bot",
		"spider",
		"crawler",
		"robot",
		"curl",
		"wget",
		"fetch",
	}

	lowerUA := userAgent
	for _, keyword := range crawlerKeywords {
		if containsIgnoreCase(lowerUA, keyword) {
			return true
		}
	}

	return false
}

// RenderWithContext 渲染页面（带gin.Context参数的版本）
func (e *engine) RenderWithContext(c *gin.Context, url string, opts RenderOptions, userAgent string) (RenderWithCacheResult, error) {
	// 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	// 创建Chrome实例
	options := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, options...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// 导航到页面
	var html string
	err := chromedp.Run(chromeCtx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("body"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// 等待页面加载完成
			time.Sleep(2 * time.Second)
			return nil
		}),
		chromedp.OuterHTML("html", &html),
	)

	if err != nil {
		return RenderWithCacheResult{
			Result: RenderResult{
				HTML:    "",
				Success: false,
				Error:   err.Error(),
			},
			HitCache: false,
		}, fmt.Errorf("failed to render page: %w", err)
	}

	return RenderWithCacheResult{
		Result: RenderResult{
			HTML:       html,
			Success:    true,
			Error:      "",
			RenderTime: 0,
			URL:        url,
		},
		HitCache: false,
	}, nil
}

// CleanupPreheatTasks 清理过期的预热任务
func (e *engine) CleanupPreheatTasks() error {
	// 获取所有预热任务
	keys, err := e.redisClient.Keys("task:preheat:*")
	if err != nil {
		return fmt.Errorf("failed to get task keys: %w", err)
	}

	// 清理24小时前创建的已完成任务
	now := time.Now().Unix()
	for _, key := range keys {
		task := make(map[string]interface{})
		if err := e.redisClient.GetJSON(key, &task); err != nil {
			continue
		}

		createdAt, ok := task["created_at"].(float64)
		if !ok {
			continue
		}

		status, _ := task["status"].(string)
		if (status == "completed" || status == "completed_with_errors" || status == "cancelled") && now-int64(createdAt) > 24*3600 {
			e.redisClient.Del(key)
		}
	}

	return nil
}

// containsIgnoreCase 忽略大小写检查字符串是否包含子串
func containsIgnoreCase(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

// equalFold 忽略大小写比较两个字符串是否相等
func equalFold(s, t string) bool {
	if len(s) != len(t) {
		return false
	}
	for i := 0; i < len(s); i++ {
		a, b := s[i], t[i]
		if a >= 'A' && a <= 'Z' {
			a += 'a' - 'A'
		}
		if b >= 'A' && b <= 'Z' {
			b += 'a' - 'A'
		}
		if a != b {
			return false
		}
	}
	return true
}
