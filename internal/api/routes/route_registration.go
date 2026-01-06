package routes

import (
	"prerender-shield/internal/auth"

	"github.com/gin-gonic/gin"
)

// RegisterAllRoutes 注册所有API路由
func RegisterAllRoutes(ginRouter *gin.Engine, controllers *Controllers, jwtManager *auth.JWTManager) {
	// 注册API路由
	apiGroup := ginRouter.Group("/api/v1")
	{
		// 认证相关API - 不需要JWT验证
		authGroup := apiGroup.Group("/auth")
		{
			// 检查是否是首次运行
			authGroup.GET("/first-run", controllers.AuthController.CheckFirstRun)

			// 用户登录
			authGroup.POST("/login", controllers.AuthController.Login)

			// 用户退出登录
			authGroup.POST("/logout", controllers.AuthController.Logout)
		}

		// 系统相关API - 不需要JWT验证
		apiGroup.GET("/health", controllers.SystemController.Health)
		apiGroup.GET("/version", controllers.SystemController.Version)

		// 需要JWT验证的API组
		protectedGroup := apiGroup.Group("")
		protectedGroup.Use(auth.JWTAuthMiddleware(jwtManager))
		{
			// 系统配置API
			protectedGroup.GET("/system/config", controllers.SystemController.GetSystemConfig)
			protectedGroup.POST("/system/config", controllers.SystemController.UpdateSystemConfig)

			// 概览API
			protectedGroup.GET("/overview", controllers.OverviewController.GetOverview)

			// 监控API
			protectedGroup.GET("/monitoring/stats", controllers.MonitoringController.GetStats)

			// 访问日志API
			protectedGroup.GET("/logs", controllers.FirewallController.GetAccessLogs)

			// 爬虫日志API
			protectedGroup.GET("/crawler/logs", controllers.CrawlerController.GetCrawlerLogs)
			protectedGroup.GET("/crawler/stats", controllers.CrawlerController.GetCrawlerStats)

			// 预热API
			protectedGroup.GET("/preheat/sites", controllers.PreheatController.GetPreheatSites)
			protectedGroup.GET("/preheat/stats", controllers.PreheatController.GetPreheatStats)
			protectedGroup.POST("/preheat/trigger", controllers.PreheatController.TriggerPreheat)
			protectedGroup.GET("/preheat/urls", controllers.PreheatController.GetPreheatUrls)
			protectedGroup.GET("/preheat/task/status", controllers.PreheatController.GetPreheatTaskStatus)
			protectedGroup.GET("/preheat/crawler-headers", controllers.PreheatController.GetCrawlerHeaders)
			protectedGroup.POST("/preheat/clear-cache", controllers.PreheatController.ClearCache)

			// 推送API
			protectedGroup.GET("/push/sites", controllers.PushController.GetSites)
			protectedGroup.GET("/push/stats", controllers.PushController.GetPushStats)
			protectedGroup.GET("/push/logs", controllers.PushController.GetPushLogs)
			protectedGroup.GET("/push/trend", controllers.PushController.GetPushTrend)
			protectedGroup.GET("/push/config", controllers.PushController.GetPushConfig)
			protectedGroup.POST("/push/config", controllers.PushController.UpdatePushConfig)

			// 站点管理API
			sitesGroup := protectedGroup.Group("/sites")
			{
				// 获取站点列表
				sitesGroup.GET("", controllers.SitesController.GetSites)

				// 获取单个站点信息
				sitesGroup.GET("/:id", controllers.SitesController.GetSite)

				// 获取站点的Redis配置（预渲染或推送配置）
				sitesGroup.GET("/:id/config", controllers.SitesController.GetSiteConfig)

				// WAF Configuration
				sitesGroup.GET("/:id/waf", controllers.FirewallController.GetWafConfig)
				sitesGroup.PUT("/:id/waf", controllers.FirewallController.UpdateWafConfig)

				// 添加站点
				sitesGroup.POST("", controllers.SitesController.AddSite)

				// 更新站点
				sitesGroup.PUT("/:id", controllers.SitesController.UpdateSite)

				// 删除站点
				sitesGroup.DELETE("/:id", controllers.SitesController.DeleteSite)

				// 静态资源管理API
				// 获取站点的静态资源文件列表
				sitesGroup.GET("/:id/static", controllers.SitesController.GetStaticFiles)

				// 上传静态资源文件
				sitesGroup.POST("/:id/static", controllers.SitesController.UploadStaticFile)

				// 解压文件
				sitesGroup.POST("/:id/static/extract", controllers.SitesController.ExtractFile)

				// 删除静态资源文件
				sitesGroup.DELETE("/:id/static", controllers.SitesController.DeleteStaticFile)
	
		}
		}
	}
}