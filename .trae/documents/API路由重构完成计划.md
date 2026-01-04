# API路由重构完成计划

## 当前状态
- 已创建所有必要的控制器文件
- 已开始重构routes.go文件，但尚未完成
- 系统处于plan mode，需要提交完整计划后才能继续执行

## 剩余工作

### 1. 完成routes.go文件的重构
- 替换整个RegisterRoutes函数，使用控制器实例处理所有API请求
- 移除所有不再需要的辅助函数（如isPortAvailable, ExtractZIP等）
- 确保所有控制器实例正确初始化和使用

### 2. 清理routes.go文件
- 删除所有未使用的导入
- 移除所有内嵌的处理函数
- 确保代码结构清晰，只包含路由注册逻辑

### 3. 验证重构后的代码
- 确保所有路由正确映射到对应的控制器方法
- 检查控制器依赖注入是否正确
- 确保中间件配置保持不变

## 具体实现步骤

### 步骤1：替换RegisterRoutes函数
将当前的RegisterRoutes函数替换为使用控制器的版本：
```go
// RegisterRoutes 注册所有API路由
func (r *Router) RegisterRoutes(ginRouter *gin.Engine) {
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

	// 创建控制器实例
	authController := controllers.NewAuthController(r.userManager, r.jwtManager)
	overviewController := controllers.NewOverviewController(r.cfg, r.monitor)
	monitoringController := controllers.NewMonitoringController(r.monitor)
	firewallController := controllers.NewFirewallController()
	crawlerController := controllers.NewCrawlerController(r.crawlerLogMgr)
	preheatController := controllers.NewPreheatController(r.prerenderManager, r.redisClient, r.cfg)
	sitesController := controllers.NewSitesController(
		r.configManager,
		r.siteServerMgr,
		r.siteHandler,
		r.redisClient,
		r.monitor,
		r.crawlerLogMgr,
		r.cfg,
	)

	// 注册API路由
	apiGroup := ginRouter.Group("/api/v1")
	{
		// 认证相关API - 不需要JWT验证
		authGroup := apiGroup.Group("/auth")
		{
			// 检查是否是首次运行
			authGroup.GET("/first-run", authController.CheckFirstRun)
			// 用户登录
			authGroup.POST("/login", authController.Login)
			// 用户退出登录
			authGroup.POST("/logout", authController.Logout)
		}

		// 需要JWT验证的API组
		protectedGroup := apiGroup.Group("/")
		protectedGroup.Use(auth.JWTAuthMiddleware(r.jwtManager))
		{
			// 概览API
			protectedGroup.GET("/overview", overviewController.GetOverview)

			// 监控API
			protectedGroup.GET("/monitoring/stats", monitoringController.GetStats)
			
			// 防火墙API
			protectedGroup.GET("/firewall/status", firewallController.GetFirewallStatus)
			protectedGroup.GET("/firewall/rules", firewallController.GetFirewallRules)

			// 爬虫日志API
			protectedGroup.GET("/crawler/logs", crawlerController.GetCrawlerLogs)
			protectedGroup.GET("/crawler/stats", crawlerController.GetCrawlerStats)

			// 预热API
			protectedGroup.GET("/preheat/sites", preheatController.GetPreheatSites)
			protectedGroup.GET("/preheat/stats", preheatController.GetPreheatStats)
			protectedGroup.POST("/preheat/trigger", preheatController.TriggerPreheat)
			protectedGroup.POST("/preheat/url", preheatController.PreheatURLs)
			protectedGroup.GET("/preheat/urls", preheatController.GetPreheatUrls)
			protectedGroup.GET("/preheat/task/status", preheatController.GetPreheatTaskStatus)
			protectedGroup.GET("/preheat/crawler-headers", preheatController.GetCrawlerHeaders)

			// 站点管理API
			sitesGroup := protectedGroup.Group("/sites")
			{
				// 获取站点列表
				sitesGroup.GET("", sitesController.GetSites)
				// 获取单个站点信息
				sitesGroup.GET("/:id", sitesController.GetSite)
				// 添加站点
				sitesGroup.POST("", sitesController.AddSite)
				// 更新站点
				sitesGroup.PUT("/:id", sitesController.UpdateSite)
				// 删除站点
				sitesGroup.DELETE("/:id", sitesController.DeleteSite)
				// 获取站点的静态资源文件列表
				sitesGroup.GET("/:id/static", sitesController.GetStaticFiles)
				// 上传静态资源文件
				sitesGroup.POST("/:id/static", sitesController.UploadStaticFile)
				// 解压文件
				sitesGroup.POST("/:id/static/extract", sitesController.ExtractFile)
				// 删除静态资源文件
				sitesGroup.DELETE("/:id/static", sitesController.DeleteStaticFile)
			}
		}
	}
}
```

### 步骤2：清理routes.go文件
- 删除所有未使用的导入（如time, strconv, strings, os, filepath, io, archive/zip, log, net等）
- 移除所有内嵌的辅助函数（isPortAvailable, ExtractZIP等）
- 确保只保留必要的导入和路由注册逻辑

### 步骤3：验证重构
- 检查所有控制器实例是否正确初始化
- 确保所有路由正确映射到对应的控制器方法
- 验证中间件配置保持不变
- 确保没有遗漏任何API路由

## 预期结果
- routes.go文件将只负责API声明和路由注册
- 所有路由处理逻辑将迁移到对应的控制器中
- 代码结构更加清晰，符合良好的架构设计
- 便于后续维护和扩展

## 风险评估
- 可能存在控制器依赖注入错误
- 可能遗漏某些API路由
- 可能存在导入错误

## 缓解措施
- 仔细检查每个控制器的初始化和依赖注入
- 对比原始路由和重构后的路由，确保没有遗漏
- 使用IDE的自动导入功能确保导入正确
- 在完成重构后进行测试，确保所有API正常工作

现在我已经准备好执行这个计划，完成API路由的重构工作。