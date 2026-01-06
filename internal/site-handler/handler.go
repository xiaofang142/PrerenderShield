package sitehandler

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/middleware"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/prerender"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/repository"
)

// Handler 站点处理器，负责处理站点的HTTP请求
// 管理站点的请求路由、爬虫检测和响应处理
//
// 字段:
//   prerenderManager: 渲染预热引擎管理器，用于处理爬虫请求的渲染
//   wafRepo: WAF仓库，用于记录WAF日志
//   redisClient: Redis客户端，用于限流
type Handler struct {
	prerenderManager *prerender.EngineManager
	wafRepo          *repository.WafRepository
	redisClient      *redis.Client
}

// NewHandler 创建站点处理器实例
//
// 参数:
//
//	prerenderManager: 渲染预热引擎管理器，用于处理爬虫请求的渲染
//	wafRepo: WAF仓库
//	redisClient: Redis客户端
//
// 返回值:
//
//	*Handler: 创建的站点处理器实例
//
// 示例:
//
//	handler := sitehandler.NewHandler(prerenderManager, wafRepo, redisClient)
func NewHandler(prerenderManager *prerender.EngineManager, wafRepo *repository.WafRepository, redisClient *redis.Client) *Handler {
	return &Handler{
		prerenderManager: prerenderManager,
		wafRepo:          wafRepo,
		redisClient:      redisClient,
	}
}

// CreateSiteHandler 创建基于站点配置的HTTP处理器
// 根据站点配置创建对应的HTTP处理器，支持proxy、static和redirect三种模式
//
// 参数:
//
//	site: 站点配置，包含站点的基本信息、运行模式、路由规则等
//	crawlerLogManager: 爬虫日志管理器，用于记录爬虫访问日志
//	monitor: 监控管理器，用于记录请求指标
//	staticDir: 静态文件目录，用于static模式下的文件服务
//
// 返回值:
//
//	http.Handler: 创建的HTTP处理器，可直接用于HTTP服务器
//
// 示例:
//
//	httpHandler := handler.CreateSiteHandler(siteConfig, crawlerLogManager, visitLogManager, monitor, "/static")
//	http.ListenAndServe(":8080", httpHandler)
func (h *Handler) CreateSiteHandler(site config.SiteConfig, crawlerLogManager *logging.CrawlerLogManager, visitLogManager *logging.VisitLogManager, monitor *monitoring.Monitor, staticDir string) http.Handler {
	// 创建站点级别的Gin路由器
	siteRouter := gin.Default()

	// WAF中间件 - 最先执行，保护后续处理
	siteRouter.Use(middleware.WafMiddleware(site, h.wafRepo, h.redisClient))

	// 爬虫检测中间件 - 第一个执行，确保爬虫请求得到正确处理
	siteRouter.Use(func(c *gin.Context) {
		// 获取请求的User-Agent
		userAgent := c.Request.UserAgent()

		// 检测爬虫
		isCrawler := false

		// 只有当prerenderManager不为nil时才使用引擎的检测方法
		if h.prerenderManager != nil {
			prerenderEngine, _ := h.prerenderManager.GetEngine(site.ID)
			if prerenderEngine != nil {
				isCrawler = prerenderEngine.IsCrawlerRequest(userAgent)
			} else {
				// 降级方案：使用默认的爬虫UA检测
				lowerUA := strings.ToLower(userAgent)
				isCrawler = strings.Contains(lowerUA, "baiduspider") ||
					strings.Contains(lowerUA, "googlebot") ||
					strings.Contains(lowerUA, "bingbot") ||
					strings.Contains(lowerUA, "yandexbot") ||
					strings.Contains(lowerUA, "sogou")
			}
		}

		if isCrawler {
			// 如果prerenderManager为nil，无法处理爬虫请求，返回500错误
			if h.prerenderManager == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Prerender engine not available"})
				monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusInternalServerError, 0)
				c.Abort()
				return
			}

			// 记录爬虫请求开始时间
			startTime := time.Now()

			// 记录爬虫请求
			monitor.RecordCrawlerRequest()

			// 构建完整的URL
			var scheme string
			if c.Request.TLS != nil {
				scheme = "https"
			} else {
				scheme = "http"
			}
			fullURL := fmt.Sprintf("%s://%s%s", scheme, c.Request.Host, c.Request.URL.Path)
			if c.Request.URL.RawQuery != "" {
				fullURL += "?" + c.Request.URL.RawQuery
			}

			// 获取当前站点的渲染预热引擎实例
			prerenderEngine, exists := h.prerenderManager.GetEngine(site.ID)
			if !exists {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Prerender engine not found"})
				monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusInternalServerError, 0)
				c.Abort()
				return
			}

			// 使用渲染预热引擎渲染页面
			resultWithCache, err := prerenderEngine.Render(c, fullURL, prerender.RenderOptions{
				Timeout:   site.Prerender.Timeout,
				WaitUntil: "networkidle0",
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Prerender failed"})
				monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusInternalServerError, 0)
				c.Abort()
				return
			}

			result := resultWithCache.Result
			if !result.Success {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Prerender result failed"})
				monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusInternalServerError, 0)
				c.Abort()
				return
			}

			// 计算渲染时间
			renderTime := time.Since(startTime).Seconds()

			// 记录爬虫访问日志
			crawlerLog := logging.CrawlerLog{
				Site:       site.ID,
				IP:         logging.GetClientIP(c.Request),
				Time:       time.Now(),
				HitCache:   resultWithCache.HitCache, // 使用实际的缓存命中状态
				Route:      c.Request.URL.Path,
				UA:         userAgent,
				Status:     http.StatusOK,
				Method:     c.Request.Method,
				CacheTTL:   site.Prerender.CacheTTL,
				RenderTime: float64(int(renderTime*100)) / 100, // 保留两位小数
			}
			crawlerLogManager.RecordCrawlerLog(crawlerLog)

			// 返回渲染后的HTML响应
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(result.HTML))
			// 记录请求
			monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusOK, time.Duration(renderTime*float64(time.Second)))
			// 终止请求处理，避免后续处理器覆盖我们的响应
			c.Abort()
			return
		}

		// 非爬虫请求，继续处理
		c.Next()
	})

	// 非爬虫请求处理中间件
	siteRouter.Use(func(c *gin.Context) {
		startTime := time.Now()

		// 记录正常访问日志
		defer func() {
			visitLog := logging.VisitLog{
				Site:     site.ID,
				IP:       logging.GetClientIP(c.Request),
				Time:     startTime,
				Method:   c.Request.Method,
				URL:      c.Request.URL.String(),
				Status:   c.Writer.Status(),
				UA:       c.Request.UserAgent(),
				Duration: time.Since(startTime).Seconds(),
				Referer:  c.Request.Referer(),
				Washed:   false,
			}
			visitLogManager.RecordVisitLog(visitLog)
		}()

		// 根据站点模式处理请求
		switch site.Mode {
		case "proxy":
			// 代理已有应用模式：将请求转发到上游服务
			proxyURL, err := url.Parse(site.Proxy.TargetURL)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Invalid upstream URL"})
				monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusInternalServerError, time.Since(startTime))
				c.Abort()
				return
			}

			proxy := httputil.NewSingleHostReverseProxy(proxyURL)
			proxy.ServeHTTP(c.Writer, c.Request)
			monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusOK, time.Since(startTime))
			c.Abort()
			return

		case "static":
			// 静态资源站模式：提供静态文件服务
			// 静态文件目录：{staticDir}/{site.ID}
			siteStaticDir := filepath.Join(staticDir, site.ID)

			// 确保静态文件目录存在
			if _, err := os.Stat(siteStaticDir); os.IsNotExist(err) {
				os.MkdirAll(siteStaticDir, 0755)
			}

			// 处理URL，移除hash部分并获取实际路径
			getActualPath := func(urlPath string) string {
				// 移除URL中的hash部分，因为hash不会发送到服务器
				if hashIndex := strings.Index(urlPath, "#"); hashIndex != -1 {
					return urlPath[:hashIndex]
				}
				return urlPath
			}

			// 检查请求的路径是否为静态资源
			isStaticResource := func(path string) bool {
				staticExtensions := []string{
					".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg",
					".css", ".less", ".sass", ".scss",
					".js", ".jsx", ".ts", ".tsx",
					".woff", ".woff2", ".ttf", ".eot",
					".ico", ".txt", ".json", ".xml", ".pdf", ".zip", ".rar",
					".mp4", ".mp3", ".avi", ".mov", ".wmv",
					".csv", ".xls", ".xlsx", ".doc", ".docx",
				}
				for _, ext := range staticExtensions {
					if len(path) >= len(ext) && path[len(path)-len(ext):] == ext {
						return true
					}
				}
				return false
			}

			// 获取实际路径（移除hash部分）
			actualPath := getActualPath(c.Request.URL.Path)

			// 对于静态资源，尝试直接提供文件
			if isStaticResource(actualPath) {
				filePath := filepath.Join(siteStaticDir, actualPath)
				if _, err := os.Stat(filePath); err == nil {
					c.File(filePath)
					monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusOK, time.Since(startTime))
					return
				}
			}

			// 对于非静态资源，返回index.html（SPA路由处理）
			indexPath := filepath.Join(siteStaticDir, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				c.File(indexPath)
				monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusOK, time.Since(startTime))
				return
			}

			// 文件不存在，返回404
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "File not found",
				"data": gin.H{
					"site":    site.Name,
					"domains": site.Domains,
					"port":    site.Port,
					"path":    c.Request.URL.Path,
				},
			})
			monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusNotFound, time.Since(startTime))
			c.Abort()
			return

		case "redirect":
			// 重定向模式：返回重定向响应
			c.Redirect(site.Redirect.StatusCode, site.Redirect.TargetURL)
			monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, site.Redirect.StatusCode, time.Since(startTime))
			c.Abort()
			return

		default:
			// 未知模式，返回500错误
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "Invalid site mode",
			})
			monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusInternalServerError, time.Since(startTime))
			c.Abort()
			return
		}
	})

	// 返回站点路由器作为HTTP处理器
	return siteRouter
}
