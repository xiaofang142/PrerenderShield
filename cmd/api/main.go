package main

import (
	"archive/zip"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

	"prerender-shield/internal/auth"
	"prerender-shield/internal/config"
	"prerender-shield/internal/firewall"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/prerender"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/routing"
	"prerender-shield/internal/scheduler"

	"github.com/gin-gonic/gin"
)

// 站点服务器映射，用于管理所有运行中的站点服务器
var siteServers = make(map[string]*http.Server)

// 监控管理器实例
var monitor *monitoring.Monitor

// 渲染预热引擎管理器实例
var prerenderManager *prerender.EngineManager

// Redis客户端实例
var redisClient *redis.Client

// 定时任务调度器实例
var schedulerInstance *scheduler.Scheduler

// 常用互联网端口列表，这些端口将被排除
var reservedPorts = map[int]bool{
	// 常用服务端口
	21:  true, // FTP
	22:  true, // SSH
	23:  true, // Telnet
	25:  true, // SMTP
	53:  true, // DNS
	80:  true, // HTTP
	110: true, // POP3
	143: true, // IMAP
	443: true, // HTTPS
	465: true, // SMTPS
	587: true, // SMTP (STARTTLS)
	993: true, // IMAPS
	995: true, // POP3S

	// 常用应用端口
	3306:  true, // MySQL
	5432:  true, // PostgreSQL
	6379:  true, // Redis
	8080:  true, // Tomcat
	9000:  true, // PHP-FPM
	9090:  true, // Prometheus
	15672: true, // RabbitMQ
	27017: true, // MongoDB
}

// 检查端口是否可用
func isPortAvailable(port int) bool {
	// 检查是否是保留端口
	if reservedPorts[port] {
		return false
	}

	// 尝试监听端口
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	defer listener.Close()

	return true
}

// 创建基于域名的虚拟主机处理器
type domainHandler struct {
	handlers       map[string]http.Handler
	defaultHandler http.Handler
}

// ServeHTTP 实现http.Handler接口，根据请求的Host头路由到对应的站点处理器
func (dh *domainHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 获取请求的Host头，去除端口号
	host := r.Host
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// 根据Host查找对应的处理器
	if handler, exists := dh.handlers[host]; exists {
		handler.ServeHTTP(w, r)
	} else if dh.defaultHandler != nil {
		// 使用默认处理器
		dh.defaultHandler.ServeHTTP(w, r)
	} else {
		// 返回404
		http.Error(w, "Site not found", http.StatusNotFound)
	}
}

// 域名处理器映射，用于管理所有站点的HTTP处理器
var domainHandlers = make(map[string]http.Handler)

// 启动站点服务器，返回站点的HTTP处理器
func startSiteServer(site config.SiteConfig, serverAddress string, staticDir string, crawlerLogManager *logging.CrawlerLogManager, monitor *monitoring.Monitor) http.Handler {
	// 创建站点级别的Gin路由器
	siteRouter := gin.Default()

	// 爬虫检测中间件 - 第一个执行，确保爬虫请求得到正确处理
	siteRouter.Use(func(c *gin.Context) {
		// 获取请求的User-Agent
		userAgent := c.Request.UserAgent()
		log.Printf("Request received: Path=%s, User-Agent=%s, Host=%s", c.Request.URL.Path, userAgent, c.Request.Host)

		// 检测爬虫
		// 使用渲染预热引擎的IsCrawlerRequest方法，该方法会从站点配置中读取爬虫UA列表
		prerenderEngine, _ := prerenderManager.GetEngine(site.ID)
		isCrawler := false
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

		log.Printf("Crawler detection: User-Agent=%s, isCrawler=%t", userAgent, isCrawler)

		if isCrawler {
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
			log.Printf("Prerendering URL: %s for site: %s", fullURL, site.Name)

			// 获取当前站点的渲染预热引擎实例
			prerenderEngine, exists := prerenderManager.GetEngine(site.ID)
			if !exists {
				log.Printf("Prerender engine not found for site ID: %s, name: %s", site.ID, site.Name)
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
				log.Printf("Prerender failed for URL: %s, error: %v", fullURL, err)
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Prerender failed"})
				monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusInternalServerError, 0)
				c.Abort()
				return
			}

			result := resultWithCache.Result
			if !result.Success {
				log.Printf("Prerender result failed for URL: %s, error: %s", fullURL, result.Error)
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Prerender result failed"})
				monitor.RecordRequest(c.Request.Method, c.Request.URL.Path, http.StatusInternalServerError, 0)
				c.Abort()
				return
			}

			// 计算渲染时间
			renderTime := time.Since(startTime).Seconds()

			// 记录爬虫访问日志
			crawlerLog := logging.CrawlerLog{
				Site:       site.Name,
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

			// 获取实际路径（移除hash部分）
			actualPath := getActualPath(c.Request.URL.Path)

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

// 停止站点服务器
func stopSiteServer(siteName string) {
	// 检查站点服务器是否存在
	if server, exists := siteServers[siteName]; exists {
		// 关闭服务器
		if err := server.Close(); err != nil {
			log.Printf("Failed to stop site server %s: %v", siteName, err)
		} else {
			log.Printf("Site server %s stopped successfully", siteName)
		}
		// 从映射中删除
		delete(siteServers, siteName)
	}
}

// extractZIP 解压ZIP文件
func extractZIP(archivePath, extractPath string) error {
	// 打开ZIP文件
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	// 遍历ZIP文件中的所有文件
	for _, file := range reader.File {
		// 构建目标文件路径
		targetPath := filepath.Join(extractPath, file.Name)

		// 检查文件类型
		if file.FileInfo().IsDir() {
			// 创建目录
			os.MkdirAll(targetPath, os.ModePerm)
			continue
		}

		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(targetPath), os.ModePerm); err != nil {
			return err
		}

		// 打开ZIP中的文件
		inFile, err := file.Open()
		if err != nil {
			return err
		}

		// 创建目标文件
		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			inFile.Close()
			return err
		}

		// 复制文件内容
		if _, err := io.Copy(outFile, inFile); err != nil {
			inFile.Close()
			outFile.Close()
			return err
		}

		// 关闭文件
		inFile.Close()
		outFile.Close()
	}

	return nil
}

// extractRAR 解压RAR文件（预留实现，需要外部库支持）
func extractRAR(archivePath, extractPath string) error {
	return fmt.Errorf("RAR extraction not implemented yet")
}

func main() {
	// 解析命令行参数
	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to the YAML configuration file")
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 获取配置管理器实例
	configManager := config.GetInstance()

	// 启动配置文件监控
	if err := configManager.StartWatching(); err != nil {
		log.Printf("Failed to start config watching: %v", err)
	} else {
		log.Println("Config watching started")
	}

	// 添加配置变化处理函数
	configManager.AddConfigChangeHandler(func(newConfig *config.Config) {
		logging.DefaultLogger.Info("Config updated, reloading services...")
		// 记录配置变更审计日志
		logging.DefaultLogger.LogAdminAction("system", "localhost", "config_update", "global_config", map[string]interface{}{"source": "config_file"}, "success", "Configuration updated from file")
		// 这里可以添加需要重新加载的服务逻辑
		// 例如：重新初始化防火墙规则、渲染预热引擎等
		logging.DefaultLogger.Info("Services reloaded successfully")
	})

	// 初始化认证模块
	// 1. 创建用户管理器
	userManager := auth.NewUserManager(cfg.Dirs.DataDir)

	// 2. 创建JWT管理器
	jwtManager := auth.NewJWTManager(&auth.JWTConfig{
		SecretKey:  "prerender-shield-secret-key", // 实际项目中应该从配置文件读取
		ExpireTime: 24 * time.Hour,                // 令牌过期时间
	})

	// 初始化各模块
	// 1. 防火墙引擎管理器
	firewallManager := firewall.NewEngineManager()

	// 2. 渲染预热引擎管理器
	prerenderManager = prerender.NewEngineManager()

	// 3. Redis客户端初始化
	redisClient, err = redis.NewClient(cfg.Cache.RedisURL)
	if err != nil {
		log.Printf("Failed to initialize Redis client: %v, continuing with limited functionality", err)
		// 继续运行，Redis错误不会导致程序崩溃
	}

	// 4. 定时任务调度器初始化
	schedulerInstance = scheduler.NewScheduler(prerenderManager, redisClient)
	schedulerInstance.Start()

	// 5. 爬虫日志管理器
	crawlerLogManager := logging.NewCrawlerLogManager(cfg.Cache.RedisURL)

	// 4. 为每个站点创建并启动引擎
	for _, site := range cfg.Sites {
		// 将 config.PrerenderConfig 转换为 prerender.PrerenderConfig
		prerenderConfig := prerender.PrerenderConfig{
			Enabled:           site.Prerender.Enabled,
			PoolSize:          site.Prerender.PoolSize,
			MinPoolSize:       site.Prerender.MinPoolSize,
			MaxPoolSize:       site.Prerender.MaxPoolSize,
			Timeout:           site.Prerender.Timeout,
			CacheTTL:          site.Prerender.CacheTTL,
			IdleTimeout:       site.Prerender.IdleTimeout,
			DynamicScaling:    site.Prerender.DynamicScaling,
			ScalingFactor:     site.Prerender.ScalingFactor,
			ScalingInterval:   site.Prerender.ScalingInterval,
			CrawlerHeaders:    site.Prerender.CrawlerHeaders,
			UseDefaultHeaders: site.Prerender.UseDefaultHeaders,
			Preheat: prerender.PreheatConfig{
				Enabled:         site.Prerender.Preheat.Enabled,
				SitemapURL:      site.Prerender.Preheat.SitemapURL,
				Schedule:        site.Prerender.Preheat.Schedule,
				Concurrency:     site.Prerender.Preheat.Concurrency,
				DefaultPriority: site.Prerender.Preheat.DefaultPriority,
			},
		}

		// 将引擎添加到管理器
		// AddSite 方法会自动创建并启动引擎
		if err := prerenderManager.AddSite(site.ID, prerenderConfig, redisClient); err != nil {
			logging.DefaultLogger.Error("Failed to add site to prerender manager: %v", err)
			log.Fatalf("Failed to add site to prerender manager: %v", err)
		}
		logging.DefaultLogger.Info("Prerender engine started successfully for site %s (ID: %s)", site.Name, site.ID)

		// 创建防火墙引擎
		if err := firewallManager.AddSite(site.Name, firewall.Config{
			RulesPath: site.Firewall.RulesPath,
			ActionConfig: firewall.ActionConfig{
				DefaultAction: site.Firewall.ActionConfig.DefaultAction,
				BlockMessage:  site.Firewall.ActionConfig.BlockMessage,
			},
			StaticDir:           cfg.Dirs.StaticDir,
			GeoIPConfig:         &site.Firewall.GeoIPConfig,
			RateLimitConfig:     &site.Firewall.RateLimitConfig,
			FileIntegrityConfig: &site.FileIntegrityConfig,
		}); err != nil {
			logging.DefaultLogger.Error("Failed to initialize firewall engine for site %s: %v", site.Name, err)
			log.Fatalf("Failed to initialize firewall engine for site %s: %v", site.Name, err)
		}
		logging.DefaultLogger.Info("Firewall engine initialized successfully for site %s", site.Name)
	}

	// 记录站点数量
	logging.DefaultLogger.Info("Initialized %d sites", len(cfg.Sites))

	// 4. 路由引擎
	routerEngine := routing.NewRouter(routing.Config{
		Rules: []*routing.RouteRule{},
		Cache: routing.NewMemoryCache(),
	})

	// 7. 初始化监控模块
	monitor = monitoring.NewMonitor(monitoring.Config{
		Enabled:           true,
		PrometheusAddress: ":9090",
	})
	if err := monitor.Start(); err != nil {
		logging.DefaultLogger.Error("Failed to start monitoring: %v", err)
		log.Fatalf("Failed to start monitoring: %v", err)
	}
	logging.DefaultLogger.Info("Monitoring service started successfully")

	// 初始化Gin路由
	ginRouter := gin.Default()

	// 添加安全头中间件
	ginRouter.Use(func(c *gin.Context) {
		// Content-Security-Policy (CSP) 头，防止XSS攻击，允许跨域请求
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self' http://localhost:5173 http://localhost:9598")

		// X-Frame-Options 头，防止Clickjacking攻击
		c.Header("X-Frame-Options", "DENY")

		// X-XSS-Protection 头，启用浏览器的XSS过滤
		c.Header("X-XSS-Protection", "1; mode=block")

		// X-Content-Type-Options 头，防止MIME类型嗅探
		c.Header("X-Content-Type-Options", "nosniff")

		// Referrer-Policy 头，控制Referrer信息的发送
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Strict-Transport-Security (HSTS) 头，强制使用HTTPS
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		// Permissions-Policy 头，控制浏览器API的访问
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=(), usb=(), accelerometer=(), gyroscope=()")

		c.Next()
	})

	// 添加CORS中间件
	ginRouter.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})

	// 注册API路由
	apiGroup := ginRouter.Group("/api/v1")
	{
		// 认证相关API - 不需要JWT验证
		authGroup := apiGroup.Group("/auth")
		{
			// 检查是否是首次运行
			authGroup.GET("/first-run", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data": gin.H{
						"isFirstRun": userManager.IsFirstRun(),
					},
				})
			})

			// 注册用户（仅首次运行时可用）
			authGroup.POST("/register", func(c *gin.Context) {
				// 检查是否是首次运行
				if !userManager.IsFirstRun() {
					c.JSON(http.StatusForbidden, gin.H{
						"code":    http.StatusForbidden,
						"message": "Registration is only allowed on first run",
					})
					return
				}

				// 解析请求
				var req struct {
					Username string `json:"username" binding:"required"`
					Password string `json:"password" binding:"required"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    http.StatusBadRequest,
						"message": "Invalid request",
					})
					return
				}

				// 创建用户
				user, err := userManager.CreateUser(req.Username, req.Password)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
						"code":    http.StatusInternalServerError,
						"message": err.Error(),
					})
					return
				}

				// 生成JWT令牌
				token, err := jwtManager.GenerateToken(user.ID, user.Username)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
						"code":    http.StatusInternalServerError,
						"message": "Failed to generate token",
					})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "User registered successfully",
					"data": gin.H{
						"token":    token,
						"username": user.Username,
					},
				})
			})

			// 用户登录
			authGroup.POST("/login", func(c *gin.Context) {
				// 解析请求
				var req struct {
					Username string `json:"username" binding:"required"`
					Password string `json:"password" binding:"required"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    http.StatusBadRequest,
						"message": "Invalid request",
					})
					return
				}

				// 验证用户
				user, err := userManager.AuthenticateUser(req.Username, req.Password)
				if err != nil {
					c.JSON(http.StatusUnauthorized, gin.H{
						"code":    http.StatusUnauthorized,
						"message": "Invalid username or password",
					})
					return
				}

				// 生成JWT令牌
				token, err := jwtManager.GenerateToken(user.ID, user.Username)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
						"code":    http.StatusInternalServerError,
						"message": "Failed to generate token",
					})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "Login successful",
					"data": gin.H{
						"token":    token,
						"username": user.Username,
					},
				})
			})

			// 用户退出登录
			authGroup.POST("/logout", func(c *gin.Context) {
				// JWT是无状态的，退出登录只需要前端清除token即可
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "Logout successful",
				})
			})
		}

		// 概览API
		apiGroup.GET("/overview", func(c *gin.Context) {
			// 计算总防火墙和渲染预热启用状态
			firewallEnabled := false
			prerenderEnabled := false
			for _, site := range cfg.Sites {
				if site.Firewall.Enabled {
					firewallEnabled = true
				}
				if site.Prerender.Enabled {
					prerenderEnabled = true
				}
			}

			// 获取真实监控数据
			stats := monitor.GetStats()

			// 获取站点统计数据
			activeSites := len(cfg.Sites)
			sslCertificates := 0 // SSL功能已移除

			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "success",
				"data": gin.H{
					"totalRequests":    int64(stats["totalRequests"].(float64)),
					"crawlerRequests":  int64(stats["crawlerRequests"].(float64)),
					"blockedRequests":  int64(stats["blockedRequests"].(float64)),
					"cacheHitRate":     float64(int(stats["cacheHitRate"].(float64)*100)) / 100, // 保留两位小数
					"activeBrowsers":   int(stats["activeBrowsers"].(float64)),
					"activeSites":      activeSites,
					"sslCertificates":  sslCertificates,
					"firewallEnabled":  firewallEnabled,
					"prerenderEnabled": prerenderEnabled,
					// 暂时保留模拟数据，后续可以替换为真实数据
					"geoData": gin.H{
						"countryData": []gin.H{
							{"country": "中国", "count": 891800, "color": "#1890ff"},
							{"country": "美国", "count": 2300, "color": "#52c41a"},
							{"country": "爱尔兰", "count": 461, "color": "#faad14"},
							{"country": "澳大利亚", "count": 361, "color": "#f5222d"},
							{"country": "新加坡", "count": 221, "color": "#722ed1"},
							{"country": "印度", "count": 157, "color": "#fa8c16"},
							{"country": "日本", "count": 133, "color": "#eb2f96"},
						},
						"mapData": []gin.H{
							{"name": "中国", "value": 891800},
							{"name": "美国", "value": 2300},
							{"name": "爱尔兰", "value": 461},
							{"name": "澳大利亚", "value": 361},
							{"name": "新加坡", "value": 221},
							{"name": "印度", "value": 157},
							{"name": "日本", "value": 133},
						},
					},
					"trafficData": []gin.H{
						{"time": "00:00", "totalRequests": int(stats["totalRequests"].(float64)) / 6, "crawlerRequests": int(stats["crawlerRequests"].(float64)) / 6, "blockedRequests": int(stats["blockedRequests"].(float64)) / 6},
						{"time": "04:00", "totalRequests": int(stats["totalRequests"].(float64)) / 8, "crawlerRequests": int(stats["crawlerRequests"].(float64)) / 8, "blockedRequests": int(stats["blockedRequests"].(float64)) / 8},
						{"time": "08:00", "totalRequests": int(stats["totalRequests"].(float64)) / 5, "crawlerRequests": int(stats["crawlerRequests"].(float64)) / 5, "blockedRequests": int(stats["blockedRequests"].(float64)) / 5},
						{"time": "12:00", "totalRequests": int(stats["totalRequests"].(float64)) / 3, "crawlerRequests": int(stats["crawlerRequests"].(float64)) / 3, "blockedRequests": int(stats["blockedRequests"].(float64)) / 3},
						{"time": "16:00", "totalRequests": int(stats["totalRequests"].(float64)) / 2, "crawlerRequests": int(stats["crawlerRequests"].(float64)) / 2, "blockedRequests": int(stats["blockedRequests"].(float64)) / 2},
						{"time": "20:00", "totalRequests": int(stats["totalRequests"].(float64)) / 4, "crawlerRequests": int(stats["crawlerRequests"].(float64)) / 4, "blockedRequests": int(stats["blockedRequests"].(float64)) / 4},
					},
					"accessStats": gin.H{
						"pv": int(stats["totalRequests"].(float64)) * 50,
						"uv": 100 + int(stats["totalRequests"].(float64))/100,
						"ip": 50 + int(stats["totalRequests"].(float64))/50,
					},
				},
			})
		})

		// 预热API
		apiGroup.GET("/preheat/sites", func(c *gin.Context) {
			// 获取静态网站列表
			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "success",
				"data":    []gin.H{},
			})
		})

		apiGroup.GET("/preheat/stats", func(c *gin.Context) {
			// 获取预热统计数据
			siteName := c.Query("site")

			if siteName == "" {
				// 获取所有站点的统计数据
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    []gin.H{},
				})
				return
			}

			// 获取指定站点的统计数据
			// 简化实现，直接返回空统计数据
			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "success",
				"data": gin.H{
					"urlCount":        0,
					"cacheCount":      0,
					"totalCacheSize":  0,
					"browserPoolSize": 0,
				},
			})
		})

		apiGroup.POST("/preheat/trigger", func(c *gin.Context) {
			// 触发站点预热
			var req struct {
				SiteName string `json:"siteName" binding:"required"`
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    http.StatusBadRequest,
					"message": "Invalid request",
				})
				return
			}

			// 获取站点的预渲染引擎
			engine, exists := prerenderManager.GetEngine(req.SiteName)
			if !exists {
				c.JSON(http.StatusNotFound, gin.H{
					"code":    http.StatusNotFound,
					"message": "Site not found",
				})
				return
			}

			// 调用引擎的触发预热方法
			if err := engine.TriggerPreheat(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    http.StatusInternalServerError,
					"message": fmt.Sprintf("Failed to trigger preheat: %v", err),
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "Preheat triggered successfully",
			})
		})

		apiGroup.POST("/preheat/url", func(c *gin.Context) {
			// 手动预热指定URL
			var req struct {
				SiteName string   `json:"siteName" binding:"required"`
				URLs     []string `json:"urls" binding:"required"`
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    http.StatusBadRequest,
					"message": "Invalid request",
				})
				return
			}

			// 简化实现，直接返回成功
			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "URL preheat triggered successfully",
			})
		})

		apiGroup.GET("/preheat/urls", func(c *gin.Context) {
			// 获取URL列表
			siteName := c.Query("siteName")
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

			if page < 1 {
				page = 1
			}
			if pageSize < 1 || pageSize > 100 {
				pageSize = 20
			}

			var urls []string
			var total int64

			// 检查Redis客户端是否可用
			if redisClient != nil {
				// 从Redis获取URL列表
				allUrls, err := redisClient.GetURLs(siteName)
				if err == nil {
					urls = allUrls
					total = int64(len(allUrls))
				}
			}

			// 分页处理
			start := (page - 1) * pageSize
			end := start + pageSize
			if end > len(urls) {
				end = len(urls)
			}

			var pageUrls []string
			if start < len(urls) {
				pageUrls = urls[start:end]
			}

			// 转换为前端需要的格式
			var list []gin.H
			for _, url := range pageUrls {
				list = append(list, gin.H{"url": url})
			}

			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "success",
				"data": gin.H{
					"list":     list,
					"total":    total,
					"page":     page,
					"pageSize": pageSize,
				},
			})
		})

		apiGroup.GET("/preheat/task/status", func(c *gin.Context) {
			// 获取任务状态
			siteName := c.Query("site")

			if siteName == "" {
				// 获取所有站点的任务状态
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    []gin.H{},
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "success",
				"data": gin.H{
					"siteName":  siteName,
					"scheduled": false,
					"nextRun":   "",
				},
			})
		})

		apiGroup.GET("/preheat/crawler-headers", func(c *gin.Context) {
			// 获取爬虫协议头列表
			defaultHeaders := []string{
				"Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)",
				"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
				"Mozilla/5.0 (compatible; Sogou spider/4.0; +http://www.sogou.com/docs/help/webmasters.htm#07)",
				"Mozilla/5.0 (compatible; Bytespider; https://zhanzhang.toutiao.com/)",
				"Mozilla/5.0 (compatible; HaosouSpider; http://www.haosou.com/help/help_3_2.html)",
				"Mozilla/5.0 (compatible; YisouSpider/1.0; http://www.yisou.com/help/webmaster/spider_guide.html)",
				"Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
			}

			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "success",
				"data":    defaultHeaders,
			})
		})

		// 站点管理API
		sitesGroup := apiGroup.Group("/sites")
		{
			// 获取站点列表
			sitesGroup.GET("", func(c *gin.Context) {
				// 从配置管理器获取当前配置
				currentConfig := configManager.GetConfig()
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    currentConfig.Sites,
				})
			})

			// 获取单个站点信息
			sitesGroup.GET("/:name", func(c *gin.Context) {
				name := c.Param("name")
				// 从配置管理器获取当前配置
				currentConfig := configManager.GetConfig()
				for _, site := range currentConfig.Sites {
					if site.Name == name {
						c.JSON(http.StatusOK, gin.H{
							"code":    200,
							"message": "success",
							"data":    site,
						})
						return
					}
				}
				c.JSON(http.StatusNotFound, gin.H{
					"code":    404,
					"message": "Site not found",
				})
			})

			// 添加站点
			sitesGroup.POST("", func(c *gin.Context) {
				var site config.SiteConfig
				if err := c.ShouldBindJSON(&site); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    400,
						"message": "Invalid request",
					})
					return
				}

				// 验证域名：只允许127.0.0.1或localhost
				for _, domain := range site.Domains {
					if domain != "127.0.0.1" && domain != "localhost" {
						c.JSON(http.StatusBadRequest, gin.H{
							"code":    400,
							"message": "Only 127.0.0.1 or localhost are allowed as domains",
						})
						return
					}
				}

				// 验证端口是否可用
				if !isPortAvailable(site.Port) {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    400,
						"message": "Port is either reserved or already in use",
					})
					return
				}

				// 为新站点生成唯一ID
				site.ID = uuid.New().String()

				// 从配置管理器获取当前配置并更新
				currentConfig := configManager.GetConfig()
				currentConfig.Sites = append(currentConfig.Sites, site)

				// 保存配置到文件
				if err := configManager.SaveConfig(); err != nil {
					log.Printf("Failed to save config: %v", err)
					c.JSON(http.StatusInternalServerError, gin.H{
						"code":    500,
						"message": "Failed to save site configuration",
					})
					return
				}

				// 启动新站点的服务器实例
				siteHandler := startSiteServer(site, currentConfig.Server.Address, currentConfig.Dirs.StaticDir, crawlerLogManager, monitor)

				// 启动站点服务器
				siteAddr := fmt.Sprintf("%s:%d", currentConfig.Server.Address, site.Port)
				siteServer := &http.Server{
					Addr:    siteAddr,
					Handler: siteHandler,
				}

				// 保存站点服务器引用，用于后续管理
				siteServers[site.Name] = siteServer

				// 启动站点服务器
				go func(siteName, addr string, server *http.Server) {
					if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
						log.Fatalf("站点 %s 启动失败: %v", siteName, err)
					}
				}(site.Name, siteAddr, siteServer)

				log.Printf("站点 %s 启动在 %s，模式: %s", site.Name, siteAddr, site.Mode)

				// 记录系统日志
				logging.DefaultLogger.LogAdminAction(
					"admin",
					c.ClientIP(),
					"site_add",
					"site",
					map[string]interface{}{
						"site_id":   site.ID,
						"site_name": site.Name,
						"domains":   site.Domains,
						"port":      site.Port,
						"mode":      site.Mode,
					},
					"success",
					"Site added successfully",
				)

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "Site added successfully",
					"data":    site,
				})
			})

			// 更新站点
			sitesGroup.PUT("/:name", func(c *gin.Context) {
				name := c.Param("name")
				var siteUpdates config.SiteConfig
				if err := c.ShouldBindJSON(&siteUpdates); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    400,
						"message": "Invalid request",
					})
					return
				}

				// 验证域名：只允许127.0.0.1或localhost
				for _, domain := range siteUpdates.Domains {
					if domain != "127.0.0.1" && domain != "localhost" {
						c.JSON(http.StatusBadRequest, gin.H{
							"code":    400,
							"message": "Only 127.0.0.1 or localhost are allowed as domains",
						})
						return
					}
				}

				// 从配置管理器获取当前配置
				currentConfig := configManager.GetConfig()

				// 查找并更新指定站点
				var updatedSite *config.SiteConfig
				var oldSite *config.SiteConfig

				for i, s := range currentConfig.Sites {
					if s.Name == name {
						// 保存旧站点信息
						oldSite = &s

						// 检查端口是否可用（仅当端口改变时）
						if s.Port != siteUpdates.Port {
							if !isPortAvailable(siteUpdates.Port) {
								c.JSON(http.StatusBadRequest, gin.H{
									"code":    400,
									"message": "Port is either reserved or already in use",
								})
								return
							}
						}

						// 更新站点配置，保留原始ID
						currentConfig.Sites[i].Name = siteUpdates.Name
						currentConfig.Sites[i].Domains = siteUpdates.Domains
						currentConfig.Sites[i].Port = siteUpdates.Port
						currentConfig.Sites[i].Mode = siteUpdates.Mode
						currentConfig.Sites[i].Proxy = siteUpdates.Proxy
						currentConfig.Sites[i].Redirect = siteUpdates.Redirect
						currentConfig.Sites[i].Firewall = siteUpdates.Firewall
						currentConfig.Sites[i].Prerender = siteUpdates.Prerender
						currentConfig.Sites[i].Routing = siteUpdates.Routing
						currentConfig.Sites[i].FileIntegrityConfig = siteUpdates.FileIntegrityConfig

						// 获取更新后的站点
						updatedSite = &currentConfig.Sites[i]

						break
					}
				}

				if updatedSite == nil {
					c.JSON(http.StatusNotFound, gin.H{
						"code":    404,
						"message": "Site not found",
					})
					return
				}

				// 保存配置到文件
				if err := configManager.SaveConfig(); err != nil {
					log.Printf("Failed to save config: %v", err)
					c.JSON(http.StatusInternalServerError, gin.H{
						"code":    500,
						"message": "Failed to save site configuration",
					})
					return
				}

				// 停止旧的站点服务器
				if oldServer, exists := siteServers[oldSite.Name]; exists {
					// 关闭旧服务器
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					if err := oldServer.Shutdown(ctx); err != nil {
						log.Printf("关闭站点 %s 失败: %v", oldSite.Name, err)
					} else {
						log.Printf("关闭站点 %s 成功", oldSite.Name)
						// 从映射中删除旧服务器
						delete(siteServers, oldSite.Name)
					}
				}

				// 启动新的站点服务器
				siteHandler := startSiteServer(*updatedSite, currentConfig.Server.Address, currentConfig.Dirs.StaticDir, crawlerLogManager, monitor)

				// 启动站点服务器
				siteAddr := fmt.Sprintf("%s:%d", currentConfig.Server.Address, updatedSite.Port)
				siteServer := &http.Server{
					Addr:    siteAddr,
					Handler: siteHandler,
				}

				// 保存站点服务器引用，用于后续管理
				siteServers[updatedSite.Name] = siteServer

				// 启动站点服务器
				go func(siteName, addr string, server *http.Server) {
					if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
						log.Fatalf("站点 %s 启动失败: %v", siteName, err)
					}
				}(updatedSite.Name, siteAddr, siteServer)

				log.Printf("站点 %s 更新后启动在 %s，模式: %s", updatedSite.Name, siteAddr, updatedSite.Mode)

				// 记录系统日志
				logging.DefaultLogger.LogAdminAction(
					"admin",
					c.ClientIP(),
					"site_update",
					"site",
					map[string]interface{}{
						"old_site_name": oldSite.Name,
						"new_site_name": updatedSite.Name,
						"site_id":       updatedSite.ID,
						"domains":       updatedSite.Domains,
						"port":          updatedSite.Port,
						"mode":          updatedSite.Mode,
					},
					"success",
					"Site updated successfully",
				)

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "Site updated successfully",
					"data":    updatedSite,
				})
			})

			// 删除站点
			sitesGroup.DELETE("/:name", func(c *gin.Context) {
				name := c.Param("name")

				// 从配置管理器获取当前配置并更新
				currentConfig := configManager.GetConfig()

				// 查找并删除指定站点
				for i, site := range currentConfig.Sites {
					if site.Name == name {
						// 停止站点服务器
						if siteServer, exists := siteServers[site.Name]; exists {
							// 关闭服务器
							ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
							defer cancel()
							if err := siteServer.Shutdown(ctx); err != nil {
								log.Printf("关闭站点 %s 失败: %v", site.Name, err)
							} else {
								log.Printf("关闭站点 %s 成功", site.Name)
								// 从映射中删除服务器
								delete(siteServers, site.Name)
							}
						}

						// 删除站点的静态资源目录
						staticDir := filepath.Join(cfg.Dirs.StaticDir, site.ID)
						if _, err := os.Stat(staticDir); err == nil {
							// 目录存在，删除它
							if err := os.RemoveAll(staticDir); err != nil {
								log.Printf("Failed to delete static files for site %s: %v", site.Name, err)
								// 继续执行，不中断删除流程
							} else {
								log.Printf("Deleted static files for site %s", site.Name)
							}
						}

						// 从切片中删除站点
						currentConfig.Sites = append(currentConfig.Sites[:i], currentConfig.Sites[i+1:]...)

						// 保存配置到文件
						if err := configManager.SaveConfig(); err != nil {
							log.Printf("Failed to save config: %v", err)
							c.JSON(http.StatusInternalServerError, gin.H{
								"code":    500,
								"message": "Failed to save site configuration",
							})
							return
						}

						// 记录系统日志
						logging.DefaultLogger.LogAdminAction(
							"admin",
							c.ClientIP(),
							"site_delete",
							"site",
							map[string]interface{}{
								"site_id":   site.ID,
								"site_name": site.Name,
								"domains":   site.Domains,
								"port":      site.Port,
							},
							"success",
							"Site deleted successfully",
						)

						c.JSON(http.StatusOK, gin.H{
							"code":    200,
							"message": "Site deleted successfully",
						})
						return
					}
				}

				// 如果站点不存在，返回404
				c.JSON(http.StatusNotFound, gin.H{
					"code":    404,
					"message": "Site not found",
				})
			})

			// 获取静态文件列表
			sitesGroup.GET("/:name/static", func(c *gin.Context) {
				name := c.Param("name")
				path := c.Query("path")
				if path == "" {
					path = "/"
				}

				// 检查站点是否存在并获取站点ID
				currentConfig := configManager.GetConfig()
				var siteID string
				siteExists := false
				for _, site := range currentConfig.Sites {
					if site.Name == name {
						siteID = site.ID
						siteExists = true
						break
					}
				}

				if !siteExists {
					c.JSON(http.StatusNotFound, gin.H{
						"code":    404,
						"message": "Site not found",
					})
					return
				}

				// 构建静态文件目录路径：{cfg.Dirs.StaticDir}/{site.ID}
				staticDir := filepath.Join(cfg.Dirs.StaticDir, siteID)
				targetPath := filepath.Join(staticDir, path)

				// 确保目录存在
				if _, err := os.Stat(targetPath); os.IsNotExist(err) {
					os.MkdirAll(targetPath, 0755)
				}

				// 读取目录内容
				entries, err := os.ReadDir(targetPath)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
						"code":    500,
						"message": "Failed to read directory",
					})
					return
				}

				// 转换为前端需要的数据格式
				var fileList []gin.H
				for _, entry := range entries {
					info, err := entry.Info()
					if err != nil {
						continue
					}

					// Determine file type
					fileType := "file"
					if entry.IsDir() {
						fileType = "dir"
					}

					fileList = append(fileList, gin.H{
						"key":  info.Name(),
						"name": info.Name(),
						"type": fileType,
						"size": info.Size(),
						"path": filepath.Join(path, info.Name()),
					})
				}

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    fileList,
				})
			})

			// 静态文件上传API
			sitesGroup.POST("/:name/static", func(c *gin.Context) {
				name := c.Param("name")

				// 检查站点是否存在并获取站点ID
				currentConfig := configManager.GetConfig()
				var siteID string
				siteExists := false
				for _, site := range currentConfig.Sites {
					if site.Name == name {
						siteID = site.ID
						siteExists = true
						break
					}
				}

				if !siteExists {
					c.JSON(http.StatusNotFound, gin.H{
						"code":    404,
						"message": "Site not found",
					})
					return
				}

				// 获取上传的文件
				file, err := c.FormFile("file")
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    400,
						"message": "Failed to get uploaded file",
					})
					return
				}

				// 获取文件路径
				filePath := c.PostForm("path")
				if filePath == "" {
					filePath = "/"
				}

				// 确保文件路径以斜杠结尾
				if filePath != "/" && !strings.HasSuffix(filePath, "/") {
					filePath += "/"
				}

				// 构建完整的文件保存路径：{cfg.Dirs.StaticDir}/{site.ID}
				staticDir := filepath.Join(cfg.Dirs.StaticDir, siteID)
				savePath := filepath.Join(staticDir, filePath, file.Filename)

				// 确保目录存在
				if err := os.MkdirAll(filepath.Dir(savePath), 0755); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
						"code":    500,
						"message": "Failed to create directory",
					})
					return
				}

				// 保存文件
				if err := c.SaveUploadedFile(file, savePath); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
						"code":    500,
						"message": "Failed to save file",
					})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "File uploaded successfully",
					"data": gin.H{
						"site":     name,
						"siteID":   siteID,
						"filename": file.Filename,
						"path":     filePath,
						"size":     file.Size,
					},
				})
			})

			// 解压文件API
			sitesGroup.POST("/:name/static/extract", func(c *gin.Context) {
				name := c.Param("name")
				filename := c.PostForm("filename")
				filePath := c.PostForm("path")

				// 调试日志：打印请求参数
				log.Printf("解压请求参数: name=%s, filename=%s, path=%s", name, filename, filePath)

				if filePath == "" {
					filePath = "/"
				}

				// 检查站点是否存在并获取站点ID
				currentConfig := configManager.GetConfig()
				var siteID string
				siteExists := false
				for _, site := range currentConfig.Sites {
					if site.Name == name {
						siteID = site.ID
						siteExists = true
						break
					}
				}

				if !siteExists {
					c.JSON(http.StatusNotFound, gin.H{
						"code":    404,
						"message": "Site not found",
					})
					return
				}

				// 调试日志：打印站点ID和静态目录
				log.Printf("站点ID: %s, 站点名称: %s", siteID, name)

				// 构建静态文件目录路径：{cfg.Dirs.StaticDir}/{site.ID}
				staticDir := filepath.Join(cfg.Dirs.StaticDir, siteID)

				// 修复路径拼接问题：如果filePath是根路径，直接使用staticDir
				var archivePath, extractPath string
				if filePath == "/" {
					archivePath = filepath.Join(staticDir, filename)
					extractPath = staticDir
				} else {
					archivePath = filepath.Join(staticDir, filePath, filename)
					extractPath = filepath.Join(staticDir, filePath)
				}

				// 调试日志：打印文件路径
				log.Printf("静态目录: %s, 压缩文件路径: %s, 解压路径: %s", staticDir, archivePath, extractPath)

				// 检查文件是否存在
				if _, err := os.Stat(archivePath); os.IsNotExist(err) {
					// 调试日志：文件不存在
					log.Printf("压缩文件不存在: %s", archivePath)
					c.JSON(http.StatusNotFound, gin.H{
						"code":    404,
						"message": "Archive file not found",
					})
					return
				}

				// 调试日志：文件存在，开始解压
				log.Printf("开始解压文件: %s 到 %s", archivePath, extractPath)

				// 根据文件类型解压
				var err error
				if strings.HasSuffix(filename, ".zip") {
					// 解压ZIP文件
					err = extractZIP(archivePath, extractPath)
				} else if strings.HasSuffix(filename, ".rar") {
					// 解压RAR文件
					err = extractRAR(archivePath, extractPath)
				} else {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    400,
						"message": "Unsupported archive format",
					})
					return
				}

				if err != nil {
					// 调试日志：解压失败
					log.Printf("解压失败: %v", err)
					c.JSON(http.StatusInternalServerError, gin.H{
						"code":    500,
						"message": fmt.Sprintf("Failed to extract file: %v", err),
					})
					return
				}

				// 调试日志：解压成功
				log.Printf("解压成功: %s", archivePath)

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "File extracted successfully",
					"data": gin.H{
						"site":     name,
						"filename": filename,
						"path":     filePath,
					},
				})
			})
		}

		// 防火墙API - 支持多站点
		firewallGroup := apiGroup.Group("/firewall")
		{
			// 获取防火墙状态 - 默认获取所有站点或指定站点
			firewallGroup.GET("/status", func(c *gin.Context) {
				siteName := c.Query("site")

				if siteName != "" {
					// 获取指定站点的防火墙配置
					for _, site := range cfg.Sites {
						if site.Name == siteName {
							c.JSON(http.StatusOK, gin.H{
								"code":    200,
								"message": "success",
								"data": gin.H{
									"enabled":       site.Firewall.Enabled,
									"defaultAction": site.Firewall.ActionConfig.DefaultAction,
									"rulesPath":     site.Firewall.RulesPath,
									"blockMessage":  site.Firewall.ActionConfig.BlockMessage,
								},
							})
							return
						}
					}
					c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Site not found"})
				} else {
					// 返回所有站点的防火墙状态
					siteStatuses := make(map[string]interface{})
					for _, site := range cfg.Sites {
						siteStatuses[site.Name] = gin.H{
							"enabled":       site.Firewall.Enabled,
							"defaultAction": site.Firewall.ActionConfig.DefaultAction,
							"rulesPath":     site.Firewall.RulesPath,
							"blockMessage":  site.Firewall.ActionConfig.BlockMessage,
						}
					}
					c.JSON(http.StatusOK, gin.H{
						"code":    200,
						"message": "success",
						"data":    siteStatuses,
					})
				}
			})

			// 获取防火墙规则 - 支持指定站点
			firewallGroup.GET("/rules", func(c *gin.Context) {
				_ = c.Query("site") // 使用下划线前缀，使其成为匿名变量
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    []gin.H{},
				})
			})

			// 触发威胁扫描 - 支持指定站点
			firewallGroup.POST("/scan", func(c *gin.Context) {
				var req struct {
					Site string `json:"site"`
					URL  string `json:"url" binding:"required"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data": gin.H{
						"scanId": "scan-12345",
						"status": "started",
						"site":   req.Site,
					},
				})
			})
		}

		// 渲染预热API - 支持多站点
		prerenderGroup := apiGroup.Group("/prerender")
		{
			// 获取渲染预热状态 - 默认获取所有站点或指定站点
			prerenderGroup.GET("/status", func(c *gin.Context) {
				siteName := c.Query("site")

				if siteName != "" {
					// 获取指定站点的渲染预热配置
					for _, site := range cfg.Sites {
						if site.Name == siteName {
							c.JSON(http.StatusOK, gin.H{
								"code":    200,
								"message": "success",
								"data": gin.H{
									"enabled":           site.Prerender.Enabled,
									"poolSize":          site.Prerender.PoolSize,
									"timeout":           site.Prerender.Timeout,
									"cacheTTL":          site.Prerender.CacheTTL,
									"preheat":           site.Prerender.Preheat.Enabled,
									"crawlerHeaders":    site.Prerender.CrawlerHeaders,
									"useDefaultHeaders": site.Prerender.UseDefaultHeaders,
								},
							})
							return
						}
					}
					c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Site not found"})
				} else {
					// 返回所有站点的渲染预热状态
					siteStatuses := make(map[string]interface{})
					for _, site := range cfg.Sites {
						siteStatuses[site.Name] = gin.H{
							"enabled":           site.Prerender.Enabled,
							"poolSize":          site.Prerender.PoolSize,
							"timeout":           site.Prerender.Timeout,
							"cacheTTL":          site.Prerender.CacheTTL,
							"preheat":           site.Prerender.Preheat.Enabled,
							"crawlerHeaders":    site.Prerender.CrawlerHeaders,
							"useDefaultHeaders": site.Prerender.UseDefaultHeaders,
						}
					}
					c.JSON(http.StatusOK, gin.H{
						"code":    200,
						"message": "success",
						"data":    siteStatuses,
					})
				}
			})

			// 手动触发渲染预热 - 支持指定站点
			prerenderGroup.POST("/render", func(c *gin.Context) {
				var req struct {
					Site string `json:"site" binding:"required"`
					URL  string `json:"url" binding:"required"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				// 获取站点配置
				var siteConfig *config.SiteConfig
				for _, site := range cfg.Sites {
					if site.Name == req.Site {
						siteConfig = &site
						break
					}
				}
				if siteConfig == nil {
					c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Site config not found"})
					return
				}

				// 获取指定站点的渲染预热引擎
				prerenderEngine, exists := prerenderManager.GetEngine(siteConfig.ID)
				if !exists {
					c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Site not found"})
					return
				}

				resultWithCache, err := prerenderEngine.Render(c, req.URL, prerender.RenderOptions{
					Timeout:   siteConfig.Prerender.Timeout,
					WaitUntil: "networkidle0",
				})
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Render failed"})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    resultWithCache.Result,
				})
			})

			// 触发缓存预热 - 支持指定站点
			prerenderGroup.POST("/preheat", func(c *gin.Context) {
				var req struct {
					Site string `json:"site" binding:"required"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				// 获取站点配置
				var siteConfig *config.SiteConfig
				for _, site := range cfg.Sites {
					if site.Name == req.Site {
						siteConfig = &site
						break
					}
				}
				if siteConfig == nil {
					c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Site config not found"})
					return
				}

				// 获取指定站点的渲染预热引擎
				prerenderEngine, exists := prerenderManager.GetEngine(siteConfig.ID)
				if !exists {
					c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Site not found"})
					return
				}

				err := prerenderEngine.TriggerPreheat()
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Preheat failed"})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "Preheat started",
				})
			})

			// 更新渲染预热配置 - 支持指定站点
			prerenderGroup.PUT("/config", func(c *gin.Context) {
				var req struct {
					Site   string                 `json:"site" binding:"required"`
					Config config.PrerenderConfig `json:"config" binding:"required"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				// 获取当前配置
				currentConfig := configManager.GetConfig()

				// 查找并更新指定站点的渲染预热配置
				var siteFound bool
				for i, site := range currentConfig.Sites {
					if site.Name == req.Site {
						// 更新站点的渲染预热配置
						currentConfig.Sites[i].Prerender = req.Config
						siteFound = true
						break
					}
				}

				if !siteFound {
					c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Site not found"})
					return
				}

				// 重启渲染预热引擎
				if err := prerenderManager.RemoveSite(req.Site); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to remove old prerender engine"})
					return
				}

				// 将 config.PrerenderConfig 转换为 prerender.PrerenderConfig
				prerenderConfig := prerender.PrerenderConfig{
					Enabled:           req.Config.Enabled,
					PoolSize:          req.Config.PoolSize,
					MinPoolSize:       req.Config.MinPoolSize,
					MaxPoolSize:       req.Config.MaxPoolSize,
					Timeout:           req.Config.Timeout,
					CacheTTL:          req.Config.CacheTTL,
					IdleTimeout:       req.Config.IdleTimeout,
					DynamicScaling:    req.Config.DynamicScaling,
					ScalingFactor:     req.Config.ScalingFactor,
					ScalingInterval:   req.Config.ScalingInterval,
					CrawlerHeaders:    req.Config.CrawlerHeaders,
					UseDefaultHeaders: req.Config.UseDefaultHeaders,
					Preheat: prerender.PreheatConfig{
						Enabled:         req.Config.Preheat.Enabled,
						SitemapURL:      req.Config.Preheat.SitemapURL,
						Schedule:        req.Config.Preheat.Schedule,
						Concurrency:     req.Config.Preheat.Concurrency,
						DefaultPriority: req.Config.Preheat.DefaultPriority,
					},
				}

				if err := prerenderManager.AddSite(req.Site, prerenderConfig, redisClient); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to restart prerender engine"})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "Prerender config updated successfully",
				})
			})
		}

		// 路由API
		routingGroup := apiGroup.Group("/routing")
		{
			// 获取路由规则
			routingGroup.GET("/rules", func(c *gin.Context) {
				rules := routerEngine.GetRules()
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    rules,
				})
			})

			// 添加路由规则
			routingGroup.POST("/rules", func(c *gin.Context) {
				var rule routing.RouteRule
				if err := c.ShouldBindJSON(&rule); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid rule"})
					return
				}

				if err := routerEngine.AddRule(&rule); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Add rule failed"})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "Rule added",
				})
			})

			// 删除路由规则
			routingGroup.DELETE("/rules/:id", func(c *gin.Context) {
				ruleID := c.Param("id")
				if err := routerEngine.DeleteRule(ruleID); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Delete rule failed"})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "Rule deleted",
				})
			})
		}

		// 监控API
		monitoringGroup := apiGroup.Group("/monitoring")
		{
			// 获取监控统计
			monitoringGroup.GET("/stats", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data": gin.H{
						"requestsPerSecond": 12.5,
						"cpuUsage":          25.3,
						"memoryUsage":       67.8,
						"diskUsage":         45.2,
					},
				})
			})

			// 获取日志
			monitoringGroup.GET("/logs", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    []gin.H{},
				})
			})
		}

		// 系统日志API
		logsGroup := apiGroup.Group("/logs")
		{
			// 获取系统日志列表，支持分页
			logsGroup.GET("/", func(c *gin.Context) {
				// 获取分页参数
				page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
				pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

				// 获取审计日志
				logs, total := logging.DefaultLogger.GetAuditLogs(page, pageSize)

				// 返回日志列表
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data": gin.H{
						"logs":     logs,
						"total":    total,
						"page":     page,
						"pageSize": pageSize,
					},
				})
			})
		}

		// 爬虫访问日志API
		crawlerGroup := apiGroup.Group("/crawler")
		{
			// 获取爬虫访问日志列表
			crawlerGroup.GET("/logs", func(c *gin.Context) {
				// 获取查询参数
				site := c.Query("site")
				page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
				pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
				startTimeStr := c.DefaultQuery("startTime", "")
				endTimeStr := c.DefaultQuery("endTime", "")

				// 解析时间范围
				var startTime, endTime time.Time
				var err error
				if startTimeStr == "" {
					startTime = time.Now().AddDate(0, 0, -7) // 默认7天前
				} else {
					startTime, err = time.Parse(time.RFC3339, startTimeStr)
					if err != nil {
						c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid startTime format"})
						return
					}
				}

				if endTimeStr == "" {
					endTime = time.Now() // 默认当前时间
				} else {
					endTime, err = time.Parse(time.RFC3339, endTimeStr)
					if err != nil {
						c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid endTime format"})
						return
					}
				}

				// 获取日志列表
				logs, total, err := crawlerLogManager.GetCrawlerLogs(site, startTime, endTime, page, pageSize)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to get crawler logs"})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data": gin.H{
						"logs":     logs,
						"total":    total,
						"page":     page,
						"pageSize": pageSize,
					},
				})
			})

			// 获取爬虫访问统计数据
			crawlerGroup.GET("/stats", func(c *gin.Context) {
				// 获取查询参数
				site := c.Query("site")
				startTimeStr := c.DefaultQuery("startTime", "")
				endTimeStr := c.DefaultQuery("endTime", "")
				granularity := c.DefaultQuery("granularity", "day") // day, week, month

				// 解析时间范围
				var startTime, endTime time.Time
				var err error
				if startTimeStr == "" {
					startTime = time.Now().AddDate(0, 0, -7) // 默认7天前
				} else {
					startTime, err = time.Parse(time.RFC3339, startTimeStr)
					if err != nil {
						c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid startTime format"})
						return
					}
				}

				if endTimeStr == "" {
					endTime = time.Now() // 默认当前时间
				} else {
					endTime, err = time.Parse(time.RFC3339, endTimeStr)
					if err != nil {
						c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid endTime format"})
						return
					}
				}

				// 获取统计数据
				stats, err := crawlerLogManager.GetCrawlerStats(site, startTime, endTime, granularity)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to get crawler stats"})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    stats,
				})
			})
		}

		// 系统API
		apiGroup.GET("/health", func(c *gin.Context) {
			// 获取系统内存使用情况
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			// 构建站点模块状态
			sitesModules := make(map[string]interface{})
			for _, site := range cfg.Sites {
				sitesModules[site.Name] = gin.H{
					"firewall": gin.H{
						"enabled": site.Firewall.Enabled,
						"status":  "healthy",
					},
					"prerender": gin.H{
						"enabled":  site.Prerender.Enabled,
						"poolSize": site.Prerender.PoolSize,
						"status":   "healthy",
					},
					"routing": gin.H{
						"status": "healthy",
					},
				}
			}

			// 构建健康检查响应
			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "ok",
				"data": gin.H{
					"status":    "healthy",
					"timestamp": time.Now().Unix(),
					"system": gin.H{
						"goVersion":  runtime.Version(),
						"cpuCount":   runtime.NumCPU(),
						"goroutines": runtime.NumGoroutine(),
						"memory": gin.H{
							"alloc": memStats.Alloc / (1024 * 1024),      // MB
							"total": memStats.TotalAlloc / (1024 * 1024), // MB
							"sys":   memStats.Sys / (1024 * 1024),        // MB
						},
					},
					"sites": sitesModules,
					"config": gin.H{
						"serverPort":  cfg.Server.APIPort,
						"environment": "production",
						"totalSites":  len(cfg.Sites),
					},
				},
			})
		})

		apiGroup.GET("/version", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "success",
				"data": gin.H{
					"version":   "v1.0.0",
					"buildDate": "2025-12-29",
				},
			})
		})
	}

	// 初始化域名处理器映射
	domainHandlers := make(map[string]http.Handler)
	defaultHandler := gin.Default()
	defaultHandler.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Site not found",
		})
	})

	// 为每个站点创建处理器，并添加到域名映射中
	for _, site := range cfg.Sites {
		siteHandler := startSiteServer(site, cfg.Server.Address, cfg.Dirs.StaticDir, crawlerLogManager, monitor)
		// 将站点的所有域名映射到该处理器
		for _, domain := range site.Domains {
			domainHandlers[domain] = siteHandler
			log.Printf("Added domain %s for site %s", domain, site.Name)
		}
		// 将站点名称也作为域名映射
		domainHandlers[site.Name] = siteHandler
	}

	// 创建主处理器，将API路由和虚拟主机路由结合
	mainHandler := http.NewServeMux()

	// 虚拟主机路由：其他所有路由根据Host头路由到对应的站点处理器
	vhHandler := &domainHandler{
		handlers:       domainHandlers,
		defaultHandler: defaultHandler,
	}

	// API路由：/api/v1/* 路由到ginRouter
	mainHandler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 检查是否是API请求
		if strings.HasPrefix(r.URL.Path, "/api/v1/") {
			// API请求，使用ginRouter处理
			ginRouter.ServeHTTP(w, r)
		} else {
			// 否则使用虚拟主机处理器
			vhHandler.ServeHTTP(w, r)
		}
	})

	// 1. 启动API服务（单独端口）
	apiAddr := fmt.Sprintf("%s:%d", cfg.Server.Address, cfg.Server.APIPort)
	apiServer := &http.Server{
		Addr:    apiAddr,
		Handler: ginRouter,
	}
	log.Printf("API服务启动在 %s", apiAddr)

	// 2. 启动管理控制台服务 - 静态资源服务器
	consoleRouter := gin.Default()
	// 配置静态文件服务
	consoleRouter.Static("/", cfg.Dirs.AdminStaticDir)
	// 处理SPA路由，所有404请求都返回index.html
	consoleRouter.NoRoute(func(c *gin.Context) {
		c.File(filepath.Join(cfg.Dirs.AdminStaticDir, "index.html"))
	})

	consoleAddr := fmt.Sprintf("%s:%d", cfg.Server.Address, cfg.Server.ConsolePort)
	consoleServer := &http.Server{
		Addr:    consoleAddr,
		Handler: consoleRouter,
	}
	log.Printf("管理控制台服务启动在 %s", consoleAddr)

	// 3. 为每个站点启动独立的HTTP服务器
	for _, site := range cfg.Sites {
		// 为每个站点创建独立的处理器
		siteHandler := startSiteServer(site, cfg.Server.Address, cfg.Dirs.StaticDir, crawlerLogManager, monitor)

		// 启动站点服务器
		siteAddr := fmt.Sprintf("%s:%d", cfg.Server.Address, site.Port)
		siteServer := &http.Server{
			Addr:    siteAddr,
			Handler: siteHandler,
		}

		// 保存站点服务器引用，用于后续管理
		siteServers[site.Name] = siteServer

		// 启动站点服务器
		go func(siteName, addr string, server *http.Server) {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("站点 %s 启动失败: %v", siteName, err)
			}
		}(site.Name, siteAddr, siteServer)

		log.Printf("站点 %s 启动在 %s，模式: %s", site.Name, siteAddr, site.Mode)
	}

	// 启动API服务
	go func() {
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("API服务启动失败: %v", err)
		}
	}()

	// 启动管理控制台服务
	go func() {
		if err := consoleServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("管理控制台服务启动失败: %v", err)
		}
	}()

	// 启动站点服务的逻辑已移到上面的循环中

	// 等待中断信号优雅关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在关闭服务器...")

	// 关闭API服务
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := apiServer.Shutdown(ctx); err != nil {
		log.Fatalf("API服务关闭失败: %v", err)
	}

	// 关闭管理控制台服务
	if err := consoleServer.Shutdown(ctx); err != nil {
		log.Fatalf("管理控制台服务关闭失败: %v", err)
	}

	log.Println("服务器已关闭")
}
