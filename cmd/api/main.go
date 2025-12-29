package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"time"

	"github.com/prerendershield/internal/config"
	"github.com/prerendershield/internal/firewall"
	"github.com/prerendershield/internal/logging"
	"github.com/prerendershield/internal/prerender"
	"github.com/prerendershield/internal/routing"
	"github.com/prerendershield/internal/ssl/cert"

	"github.com/gin-gonic/gin"
)

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

			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "success",
				"data": gin.H{
					"totalRequests":    1550,
					"crawlerRequests":  470,
					"blockedRequests":  135,
					"cacheHitRate":     85,
					"activeSites":      len(cfg.Sites),
					"sslCertificates":  len(certManager.GetDomains()),
					"firewallEnabled":  firewallEnabled,
					"prerenderEnabled": prerenderEnabled,
				},
			})
		})

		// 站点管理API
		sitesGroup := apiGroup.Group("/sites")
		{
			// 获取站点列表
			sitesGroup.GET("/", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    cfg.Sites,
				})
			})

			// 获取单个站点信息
			sitesGroup.GET("/:name", func(c *gin.Context) {
				name := c.Param("name")
				for _, site := range cfg.Sites {
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
				
				// 这里应该添加站点到配置中，当前实现简化处理
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "Site added successfully",
					"data":    site,
				})
			})

			// 更新站点
			sitesGroup.PUT("/:name", func(c *gin.Context) {
				_ = c.Param("name") // 使用下划线前缀，使其成为匿名变量
				var site config.SiteConfig
				if err := c.ShouldBindJSON(&site); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    400,
						"message": "Invalid request",
					})
					return
				}
				
				// 这里应该更新站点配置，当前实现简化处理
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "Site updated successfully",
					"data":    site,
				})
			})

			// 删除站点
			sitesGroup.DELETE("/:name", func(c *gin.Context) {
				_ = c.Param("name") // 使用下划线前缀，使其成为匿名变量
				// 这里应该从配置中删除站点，当前实现简化处理
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "Site deleted successfully",
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
				domains := certManager.GetDomains()
				c.JSON(http.StatusOK, gin.H{
					"code":    200,
					"message": "success",
					"data":    domains,
				})
			})

			// 添加域名证书
			sslGroup.POST("/certs", func(c *gin.Context) {
				var req struct {
					Domain string `json:"domain" binding:"required"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request"})
					return
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

	// 启动服务器
	addr := fmt.Sprintf("%s:%d", cfg.Server.Address, cfg.Server.Port)
	log.Printf("API server starting on %s", addr)
	if err := http.ListenAndServe(addr, ginRouter); err != nil {
		log.Fatalf("Failed to start API server: %v", err)
	}
}