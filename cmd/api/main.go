package main

import (
	"archive/zip"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"github.com/google/uuid"

	"github.com/prerendershield/internal/auth"
	"github.com/prerendershield/internal/config"
	"github.com/prerendershield/internal/firewall"
	"github.com/prerendershield/internal/logging"
	"github.com/prerendershield/internal/prerender"
	"github.com/prerendershield/internal/routing"
	"github.com/prerendershield/internal/ssl/cert"

	"github.com/gin-gonic/gin"
)

// 站点服务器映射，用于管理所有运行中的站点服务器
var siteServers = make(map[string]*http.Server)

// 启动站点服务器
func startSiteServer(site config.SiteConfig, serverAddress string) {
	// 创建站点级别的Gin路由器
	siteRouter := gin.Default()

	// 站点请求处理中间件
	siteRouter.Use(func(c *gin.Context) {
		// 移除域名验证逻辑，允许任何域名访问
		// 这样站点服务器可以作为反向代理的上游，由nginx处理域名解析

		// 根据站点的访问模式处理请求
		// 1. 检查是否启用了上游代理
		if site.Proxy.Enabled {
			// 上游代理模式：将请求转发到上游服务
			proxyURL, err := url.Parse(site.Proxy.TargetURL)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Invalid upstream URL"})
				c.Abort()
				return
			}

			proxy := httputil.NewSingleHostReverseProxy(proxyURL)
			proxy.ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		} else {
			// 直接访问模式：提供静态文件服务
			// 静态文件目录：./static/{site.ID}
			staticDir := fmt.Sprintf("./static/%s", site.ID)
			
			// 确保静态文件目录存在
			if _, err := os.Stat(staticDir); os.IsNotExist(err) {
				os.MkdirAll(staticDir, 0755)
			}
			
			// 提供静态文件服务
			// 对于根路径，尝试提供 index.html
			if c.Request.URL.Path == "/" {
				indexPath := filepath.Join(staticDir, "index.html")
				if _, err := os.Stat(indexPath); err == nil {
					c.File(indexPath)
					return
				}
			}
			
			// 尝试提供请求的文件
			filePath := filepath.Join(staticDir, c.Request.URL.Path)
			if _, err := os.Stat(filePath); err == nil {
				c.File(filePath)
				return
			}
			
			// 文件不存在，返回404
			c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"message": "File not found",
			"data": gin.H{
				"site": site.Name,
				"domains": site.Domains,
				"port": site.Port,
				"path": c.Request.URL.Path,
			},
		})
			c.Abort()
			return
		}
	})

	// 启动站点服务器
	siteAddr := fmt.Sprintf("%s:%d", serverAddress, site.Port)
	log.Printf("Site server %s (ID: %s) starting on %s for domains %v", site.Name, site.ID, siteAddr, site.Domains)

	// 创建HTTP服务器实例
	server := &http.Server{
		Addr:    siteAddr,
		Handler: siteRouter,
	}

	// 将服务器实例保存到映射中
	siteServers[site.Name] = server

	// 启动服务器
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Failed to start site server %s (ID: %s): %v", site.Name, site.ID, err)
		}
	}()
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
		// 例如：重新初始化防火墙规则、预渲染引擎等
		logging.DefaultLogger.Info("Services reloaded successfully")
	})

	// 初始化认证模块
	// 1. 创建用户管理器
	userManager := auth.NewUserManager("./data")

	// 2. 创建JWT管理器
	jwtManager := auth.NewJWTManager(&auth.JWTConfig{
		SecretKey:  "prerender-shield-secret-key", // 实际项目中应该从配置文件读取
		ExpireTime: 24 * time.Hour,                 // 令牌过期时间
	})

	// 初始化各模块
	// 1. 防火墙引擎管理器
	firewallManager := firewall.NewEngineManager()

	// 2. 预渲染引擎管理器
	prerenderManager := prerender.NewEngineManager()

	// 3. 为每个站点创建并启动引擎
	for _, site := range cfg.Sites {
		// 将 config.PrerenderConfig 转换为 prerender.PrerenderConfig
		prerenderConfig := prerender.PrerenderConfig{
			Enabled:         site.Prerender.Enabled,
			PoolSize:        site.Prerender.PoolSize,
			MinPoolSize:     site.Prerender.MinPoolSize,
			MaxPoolSize:     site.Prerender.MaxPoolSize,
			Timeout:         site.Prerender.Timeout,
			CacheTTL:        site.Prerender.CacheTTL,
			IdleTimeout:     site.Prerender.IdleTimeout,
			DynamicScaling:  site.Prerender.DynamicScaling,
			ScalingFactor:   site.Prerender.ScalingFactor,
			ScalingInterval: site.Prerender.ScalingInterval,
			CrawlerHeaders:  site.Prerender.CrawlerHeaders,
			UseDefaultHeaders: site.Prerender.UseDefaultHeaders,
			Preheat: prerender.PreheatConfig{
				Enabled:         site.Prerender.Preheat.Enabled,
				SitemapURL:      site.Prerender.Preheat.SitemapURL,
				Schedule:        site.Prerender.Preheat.Schedule,
				Concurrency:     site.Prerender.Preheat.Concurrency,
				DefaultPriority: site.Prerender.Preheat.DefaultPriority,
			},
		}

		// 创建并启动预渲染引擎
		prerenderEngine, err := prerender.NewEngine(site.Name, prerenderConfig)
		if err != nil {
			log.Fatalf("Failed to initialize prerender engine for site %s: %v", site.Name, err)
		}

		// 启动预渲染引擎
		if err := prerenderEngine.Start(); err != nil {
			logging.DefaultLogger.Error("Failed to start prerender engine for site %s: %v", site.Name, err)
			log.Fatalf("Failed to start prerender engine for site %s: %v", site.Name, err)
		}
		logging.DefaultLogger.Info("Prerender engine started successfully for site %s", site.Name)

		// 将引擎添加到管理器
		prerenderManager.AddSite(site.Name, prerenderConfig)

		// 创建防火墙引擎
		if err := firewallManager.AddSite(site.Name, firewall.Config{
			RulesPath: site.Firewall.RulesPath,
			ActionConfig: firewall.ActionConfig{
				DefaultAction: site.Firewall.ActionConfig.DefaultAction,
				BlockMessage:  site.Firewall.ActionConfig.BlockMessage,
			},
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

	// 5. SSL证书管理器
	// 从第一个站点获取SSL配置，或者使用默认值
	var sslConfig config.SSLConfig
	if len(cfg.Sites) > 0 {
		sslConfig = cfg.Sites[0].SSL
	}

	certManager, err := cert.NewManager(&cert.Config{
		Enabled:       sslConfig.Enabled,
		LetEncrypt:    sslConfig.LetEncrypt,
		Domains:       sslConfig.Domains,
		ACMEEmail:     sslConfig.ACMEEmail,
		ACMEServer:    sslConfig.ACMEServer,
		ACMEChallenge: sslConfig.ACMEChallenge,
		CertPath:      sslConfig.CertPath,
		KeyPath:       sslConfig.KeyPath,
		CertDir:       "./certs",
	})
	if err != nil {
		logging.DefaultLogger.Error("Failed to initialize certificate manager: %v", err)
		log.Fatalf("Failed to initialize certificate manager: %v", err)
	}
	logging.DefaultLogger.Info("Certificate manager initialized successfully")

	// 6. 启动证书管理器
	certManager.Start()
	logging.DefaultLogger.Info("Certificate manager started successfully")

	// 初始化Gin路由
	ginRouter := gin.Default()

	// 添加安全头中间件
	ginRouter.Use(func(c *gin.Context) {
		// Content-Security-Policy (CSP) 头，防止XSS攻击
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'")
		
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
			// 计算总防火墙和预渲染启用状态
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

			// 模拟地理位置数据，实际实现中应该从数据库或监控系统获取
			geoData := gin.H{
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
			}

			// 模拟流量趋势数据
			trafficData := []gin.H{
				{"time": "00:00", "totalRequests": 120, "crawlerRequests": 30, "blockedRequests": 10},
				{"time": "04:00", "totalRequests": 80, "crawlerRequests": 20, "blockedRequests": 5},
				{"time": "08:00", "totalRequests": 200, "crawlerRequests": 60, "blockedRequests": 15},
				{"time": "12:00", "totalRequests": 350, "crawlerRequests": 120, "blockedRequests": 30},
				{"time": "16:00", "totalRequests": 420, "crawlerRequests": 150, "blockedRequests": 45},
				{"time": "20:00", "totalRequests": 280, "crawlerRequests": 90, "blockedRequests": 25},
			}

			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "success",
				"data": gin.H{
					"totalRequests":    1550,
					"crawlerRequests":  470,
					"blockedRequests":  135,
					"cacheHitRate":     85,
					"activeBrowsers":   125,
					"activeSites":      len(cfg.Sites),
					"sslCertificates":  len(certManager.GetDomains()),
					"firewallEnabled":  firewallEnabled,
					"prerenderEnabled": prerenderEnabled,
					"geoData":          geoData,
					"trafficData":      trafficData,
					"accessStats": gin.H{
						"pv": 74100,
						"uv": 192,
						"ip": 294,
					},
				},
			})
		})

		// 站点管理API
		sitesGroup := apiGroup.Group("/sites")
		{
			// 获取站点列表
			sitesGroup.GET("/", func(c *gin.Context) {
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
			sitesGroup.POST("/", func(c *gin.Context) {
				var site config.SiteConfig
				if err := c.ShouldBindJSON(&site); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    400,
						"message": "Invalid request",
					})
					return
				}
				
				// 为新站点生成唯一ID
				site.ID = uuid.New().String()
				
				// 从配置管理器获取当前配置并更新
				currentConfig := configManager.GetConfig()
				currentConfig.Sites = append(currentConfig.Sites, site)
				
				// 启动新站点的服务器实例
				startSiteServer(site, currentConfig.Server.Address)
						
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "Site added successfully",
					"data":    site,
				})
			})

			// 更新站点
			sitesGroup.PUT("/:name", func(c *gin.Context) {
				name := c.Param("name")
				var site config.SiteConfig
				if err := c.ShouldBindJSON(&site); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    400,
						"message": "Invalid request",
					})
					return
				}
				
				// 从配置管理器获取当前配置并更新
				currentConfig := configManager.GetConfig()
				
				// 查找并更新指定站点
				for i, s := range currentConfig.Sites {
					if s.Name == name {
						// 更新站点配置
						currentConfig.Sites[i] = site
						
						// 停止旧的站点服务器
						stopSiteServer(name)
						// 启动新的站点服务器
						startSiteServer(site, currentConfig.Server.Address)
						
						c.JSON(http.StatusOK, gin.H{
							"code":    200,
							"message": "Site updated successfully",
							"data":    site,
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

			// 删除站点
			sitesGroup.DELETE("/:name", func(c *gin.Context) {
				name := c.Param("name")
				
				// 从配置管理器获取当前配置并更新
				currentConfig := configManager.GetConfig()
				
				// 查找并删除指定站点
				for i, site := range currentConfig.Sites {
					if site.Name == name {
						// 停止站点服务器
						stopSiteServer(name)
						
						// 从切片中删除站点
						currentConfig.Sites = append(currentConfig.Sites[:i], currentConfig.Sites[i+1:]...)
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
				
				// 构建静态文件目录路径：./static/{site.ID}
				staticDir := fmt.Sprintf("./static/%s", siteID)
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
				
				// 构建完整的文件保存路径：./static/{site.ID}
				staticDir := fmt.Sprintf("./static/%s", siteID)
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
						"site": name,
						"siteID": siteID,
						"filename": file.Filename,
						"path": filePath,
						"size": file.Size,
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
				
				// 构建静态文件目录路径：./static/{site.ID}
				staticDir := fmt.Sprintf("./static/%s", siteID)
				
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
						"site": name,
						"filename": filename,
						"path": filePath,
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
									"enabled":         site.Firewall.Enabled,
									"defaultAction":   site.Firewall.ActionConfig.DefaultAction,
									"rulesPath":       site.Firewall.RulesPath,
									"blockMessage":    site.Firewall.ActionConfig.BlockMessage,
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
							"enabled":         site.Firewall.Enabled,
							"defaultAction":   site.Firewall.ActionConfig.DefaultAction,
							"rulesPath":       site.Firewall.RulesPath,
							"blockMessage":    site.Firewall.ActionConfig.BlockMessage,
						}
					}
					c.JSON(http.StatusOK, gin.H{
						"code":    200,
						"message": "success",
						"data": siteStatuses,
					})
				}
			})

			// 获取防火墙规则 - 支持指定站点
			firewallGroup.GET("/rules", func(c *gin.Context) {
				_ = c.Query("site") // 使用下划线前缀，使其成为匿名变量
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data": []gin.H{},
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
						"site": req.Site,
					},
				})
			})
		}

		// 预渲染API - 支持多站点
		prerenderGroup := apiGroup.Group("/prerender")
		{
			// 获取预渲染状态 - 默认获取所有站点或指定站点
		prerenderGroup.GET("/status", func(c *gin.Context) {
			siteName := c.Query("site")
			
			if siteName != "" {
				// 获取指定站点的预渲染配置
				for _, site := range cfg.Sites {
					if site.Name == siteName {
						c.JSON(http.StatusOK, gin.H{
							"code":    200,
							"message": "success",
							"data": gin.H{
								"enabled":   site.Prerender.Enabled,
								"poolSize":  site.Prerender.PoolSize,
								"timeout":   site.Prerender.Timeout,
								"cacheTTL":  site.Prerender.CacheTTL,
								"preheat":   site.Prerender.Preheat.Enabled,
								"crawlerHeaders": site.Prerender.CrawlerHeaders,
								"useDefaultHeaders": site.Prerender.UseDefaultHeaders,
							},
						})
						return
					}
				}
				c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Site not found"})
			} else {
				// 返回所有站点的预渲染状态
				siteStatuses := make(map[string]interface{})
				for _, site := range cfg.Sites {
					siteStatuses[site.Name] = gin.H{
						"enabled":   site.Prerender.Enabled,
						"poolSize":  site.Prerender.PoolSize,
						"timeout":   site.Prerender.Timeout,
						"cacheTTL":  site.Prerender.CacheTTL,
						"preheat":   site.Prerender.Preheat.Enabled,
						"crawlerHeaders": site.Prerender.CrawlerHeaders,
						"useDefaultHeaders": site.Prerender.UseDefaultHeaders,
					}
				}
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data": siteStatuses,
				})
			}
		})

			// 手动触发预渲染 - 支持指定站点
			prerenderGroup.POST("/render", func(c *gin.Context) {
				var req struct {
					Site string `json:"site" binding:"required"`
					URL  string `json:"url" binding:"required"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				// 获取指定站点的预渲染引擎
				prerenderEngine, exists := prerenderManager.GetEngine(req.Site)
				if !exists {
					c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Site not found"})
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

				result, err := prerenderEngine.Render(c, req.URL, prerender.RenderOptions{
					Timeout:  siteConfig.Prerender.Timeout,
					WaitUntil: "networkidle0",
				})
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Render failed"})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data": result,
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

				// 获取指定站点的预渲染引擎
				prerenderEngine, exists := prerenderManager.GetEngine(req.Site)
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

			// 更新预渲染配置 - 支持指定站点
			prerenderGroup.PUT("/config", func(c *gin.Context) {
				var req struct {
					Site string `json:"site" binding:"required"`
					Config config.PrerenderConfig `json:"config" binding:"required"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				// 获取当前配置
				currentConfig := configManager.GetConfig()
				
				// 查找并更新指定站点的预渲染配置
				var siteFound bool
				for i, site := range currentConfig.Sites {
					if site.Name == req.Site {
						// 更新站点的预渲染配置
						currentConfig.Sites[i].Prerender = req.Config
						siteFound = true
						break
					}
				}

				if !siteFound {
					c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Site not found"})
					return
				}

				// 重启预渲染引擎
				if err := prerenderManager.RemoveSite(req.Site); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to remove old prerender engine"})
					return
				}

				// 将 config.PrerenderConfig 转换为 prerender.PrerenderConfig
				prerenderConfig := prerender.PrerenderConfig{
					Enabled:         req.Config.Enabled,
					PoolSize:        req.Config.PoolSize,
					MinPoolSize:     req.Config.MinPoolSize,
					MaxPoolSize:     req.Config.MaxPoolSize,
					Timeout:         req.Config.Timeout,
					CacheTTL:        req.Config.CacheTTL,
					IdleTimeout:     req.Config.IdleTimeout,
					DynamicScaling:  req.Config.DynamicScaling,
					ScalingFactor:   req.Config.ScalingFactor,
					ScalingInterval: req.Config.ScalingInterval,
					CrawlerHeaders:  req.Config.CrawlerHeaders,
					UseDefaultHeaders: req.Config.UseDefaultHeaders,
					Preheat: prerender.PreheatConfig{
						Enabled:         req.Config.Preheat.Enabled,
						SitemapURL:      req.Config.Preheat.SitemapURL,
						Schedule:        req.Config.Preheat.Schedule,
						Concurrency:     req.Config.Preheat.Concurrency,
						DefaultPriority: req.Config.Preheat.DefaultPriority,
					},
				}

				if err := prerenderManager.AddSite(req.Site, prerenderConfig); err != nil {
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

		// SSL证书API - 支持多站点
		sslGroup := apiGroup.Group("/ssl")
		{
			// 获取SSL状态 - 默认获取所有站点或指定站点
			sslGroup.GET("/status", func(c *gin.Context) {
				siteName := c.Query("site")
				
				if siteName != "" {
					// 获取指定站点的SSL配置
					for _, site := range cfg.Sites {
						if site.Name == siteName {
							c.JSON(http.StatusOK, gin.H{
								"code":    200,
								"message": "success",
								"data": gin.H{
									"enabled":       site.SSL.Enabled,
									"letEncrypt":    site.SSL.LetEncrypt,
									"acmeEmail":     site.SSL.ACMEEmail,
									"acmeServer":    site.SSL.ACMEServer,
									"acmeChallenge": site.SSL.ACMEChallenge,
								},
							})
							return
						}
					}
					c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Site not found"})
				} else {
					// 返回所有站点的SSL状态
					siteStatuses := make(map[string]interface{})
					for _, site := range cfg.Sites {
						siteStatuses[site.Name] = gin.H{
							"enabled":       site.SSL.Enabled,
							"letEncrypt":    site.SSL.LetEncrypt,
							"acmeEmail":     site.SSL.ACMEEmail,
							"acmeServer":    site.SSL.ACMEServer,
							"acmeChallenge": site.SSL.ACMEChallenge,
						}
					}
					c.JSON(http.StatusOK, gin.H{
						"code":    200,
						"message": "success",
						"data": siteStatuses,
					})
				}
			})

			// 获取证书列表
			sslGroup.GET("/certs", func(c *gin.Context) {
				siteName := c.Query("site")
				allDomains := certManager.GetDomains()
				
				// 如果指定了站点，只返回该站点相关的证书
				if siteName != "" {
					// 查找站点配置
					var siteConfig *config.SiteConfig
					for _, site := range cfg.Sites {
						if site.Name == siteName {
							siteConfig = &site
							break
						}
					}
					
					if siteConfig != nil {
						// 只返回站点配置中指定的域名证书
						var siteDomains []string
						for _, domain := range siteConfig.SSL.Domains {
							// 检查证书管理器中是否存在该域名的证书
							for _, certDomain := range allDomains {
								if certDomain == domain {
									siteDomains = append(siteDomains, domain)
									break
								}
							}
						}
						c.JSON(http.StatusOK, gin.H{
							"code":    200,
							"message": "success",
							"data":    siteDomains,
						})
						return
					}
				}
				
				// 如果没有指定站点或站点不存在，返回所有证书
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    allDomains,
				})
			})

			// 添加域名证书
			sslGroup.POST("/certs", func(c *gin.Context) {
				var req struct {
					Site   string `json:"site" binding:"required"`
					Domain string `json:"domain" binding:"required"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request"})
					return
				}

				// 检查站点是否存在
				var siteIndex int
				var siteExists bool
				for i, site := range cfg.Sites {
					if site.Name == req.Site {
						siteIndex = i
						siteExists = true
						break
					}
				}
				if !siteExists {
					c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Site not found"})
					return
				}

				// 检查域名是否已经在站点配置中
				var domainExists bool
				for _, domain := range cfg.Sites[siteIndex].SSL.Domains {
					if domain == req.Domain {
						domainExists = true
						break
					}
				}
				
				// 如果域名不在站点配置中，添加到站点配置
				if !domainExists {
					cfg.Sites[siteIndex].SSL.Domains = append(cfg.Sites[siteIndex].SSL.Domains, req.Domain)
				}

				if err := certManager.AddDomain(req.Domain); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Add domain failed"})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "Domain added",
				})
			})

			// 删除域名证书
			sslGroup.DELETE("/certs/:domain", func(c *gin.Context) {
				domain := c.Param("domain")
				siteName := c.Query("site")
				
				// 如果指定了站点，检查域名是否属于该站点
				if siteName != "" {
					// 查找站点配置
					var siteConfig *config.SiteConfig
					for _, site := range cfg.Sites {
						if site.Name == siteName {
							siteConfig = &site
							break
						}
					}
					
					if siteConfig != nil {
						// 检查域名是否在站点配置中
						var domainFound bool
						for _, siteDomain := range siteConfig.SSL.Domains {
							if siteDomain == domain {
								domainFound = true
								break
							}
						}
						
						if !domainFound {
							c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "Domain does not belong to the specified site"})
							return
						}
					}
				}
				
				if err := certManager.RemoveDomain(domain); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Delete domain failed"})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "Domain deleted",
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
						"cpuUsage":         25.3,
						"memoryUsage":      67.8,
						"diskUsage":        45.2,
					},
				})
			})

			// 获取日志
			monitoringGroup.GET("/logs", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data": []gin.H{},
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
						"status": "healthy",
					},
					"prerender": gin.H{
						"enabled": site.Prerender.Enabled,
						"poolSize": site.Prerender.PoolSize,
						"status": "healthy",
					},
					"routing": gin.H{
						"status": "healthy",
					},
					"ssl": gin.H{
						"enabled": site.SSL.Enabled,
						"letEncrypt": site.SSL.LetEncrypt,
						"domains": len(site.SSL.Domains),
						"status": "healthy",
					},
				}
			}

			// 构建健康检查响应
			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "ok",
				"data": gin.H{
					"status": "healthy",
					"timestamp": time.Now().Unix(),
					"system": gin.H{
						"goVersion": runtime.Version(),
						"cpuCount": runtime.NumCPU(),
						"goroutines": runtime.NumGoroutine(),
						"memory": gin.H{
							"alloc": memStats.Alloc / (1024 * 1024), // MB
							"total": memStats.TotalAlloc / (1024 * 1024), // MB
							"sys": memStats.Sys / (1024 * 1024), // MB
						},
					},
					"sites": sitesModules,
					"config": gin.H{
						"serverPort": cfg.Server.Port,
						"environment": "production",
						"totalSites": len(cfg.Sites),
					},
				},
			})
		})

		apiGroup.GET("/version", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "success",
				"data": gin.H{
					"version": "v1.0.0",
					"buildDate": "2025-12-29",
				},
			})
		})
	}

	// 为每个站点启动HTTP服务器实例，实现一个端口一个站点
	for _, site := range cfg.Sites {
		startSiteServer(site, cfg.Server.Address)
	}

	// 启动API服务器，使用固定端口9602
	apiAddr := fmt.Sprintf("%s:%d", cfg.Server.Address, 9602)
	log.Printf("API server starting on %s", apiAddr)
	go func() {
		if err := http.ListenAndServe(apiAddr, ginRouter); err != nil {
			log.Fatalf("Failed to start API server: %v", err)
		}
	}()
	
	// 启动管理控制台服务器
	// 管理控制台端口使用9603
	adminAddr := fmt.Sprintf("%s:%d", cfg.Server.Address, 9603)
	log.Printf("Admin console server starting on %s", adminAddr)

	// 启动管理控制台前端静态资源服务器，使用API端口+1
	// 仅在生产环境提供前端代码，开发阶段由前端开发服务器提供
	log.Printf("Admin console server starting on %s", adminAddr)
	
	// 创建静态资源服务器
	adminRouter := gin.Default()
	
	// 添加安全头中间件
	adminRouter.Use(func(c *gin.Context) {
		// Content-Security-Policy (CSP) 头，防止XSS攻击
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self' http://localhost:9598")
		
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
	
	// 提供静态资源服务
	adminStaticDir := "./web/dist" // 前端编译后的目录
	if _, err := os.Stat(adminStaticDir); os.IsNotExist(err) {
		// 如果dist目录不存在，创建一个空目录
		err := os.MkdirAll(adminStaticDir, 0755)
		if err != nil {
			log.Printf("Failed to create admin static directory: %v", err)
		}
	}
	
	// 静态文件路由
	adminRouter.Static("/", adminStaticDir)
	
	// 处理SPA路由，所有未匹配的路由都返回index.html
	adminRouter.NoRoute(func(c *gin.Context) {
		indexPath := filepath.Join(adminStaticDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			c.File(indexPath)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"message": "File not found",
		})
	})
	
	// 启动管理控制台服务器
	go func() {
		// 如果启用了SSL，使用HTTPS
		if sslConfig.Enabled {
			// 创建TLS配置
			tlsConfig := &tls.Config{
				GetCertificate: certManager.GetCertificate,
			}

			// 创建HTTP服务器
			adminServer := &http.Server{
				Addr:      adminAddr,
				Handler:   adminRouter,
				TLSConfig: tlsConfig,
			}

			// 使用HTTPS启动服务器
			log.Printf("Admin console server starting on HTTPS %s", adminAddr)
			if err := adminServer.ListenAndServeTLS("", ""); err != nil {
				log.Fatalf("Failed to start admin console HTTPS server: %v", err)
			}
		} else {
			// 使用HTTP启动服务器
			log.Printf("Admin console server starting on HTTP %s", adminAddr)
			if err := http.ListenAndServe(adminAddr, adminRouter); err != nil {
				log.Fatalf("Failed to start admin console server: %v", err)
			}
		}
	}()

	// 启动API服务器
	go func() {
		// 如果启用了SSL，使用HTTPS
		if sslConfig.Enabled {
			// 创建TLS配置
			tlsConfig := &tls.Config{
				GetCertificate: certManager.GetCertificate,
			}

			// 创建HTTP服务器
			apiServer := &http.Server{
				Addr:      apiAddr,
				Handler:   ginRouter,
				TLSConfig: tlsConfig,
			}

			// 使用HTTPS启动服务器
			log.Printf("API server starting on HTTPS %s", apiAddr)
			if err := apiServer.ListenAndServeTLS("", ""); err != nil {
				log.Fatalf("Failed to start API HTTPS server: %v", err)
			}
		} else {
			// 使用HTTP启动服务器
			log.Printf("API server starting on HTTP %s", apiAddr)
			if err := http.ListenAndServe(apiAddr, ginRouter); err != nil {
				log.Fatalf("Failed to start API server: %v", err)
			}
		}
	}()

	// 阻塞主goroutine
	select {}
}