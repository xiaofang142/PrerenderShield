package routes

import (
	"prerender-shield/internal/auth"

	"github.com/gin-gonic/gin"
)

// RegisterAllRoutes 注册所有 API 路由
func RegisterAllRoutes(ginRouter *gin.Engine, controllers *Controllers, jwtManager *auth.JWTManager) {
	// 注册 API 路由
	apiGroup := ginRouter.Group("/api/v1")
	{
		// 认证相关 API - 不需要 JWT 验证
		authGroup := apiGroup.Group("/auth")
		{
			// 检查是否是首次运行
			authGroup.GET("/first-run", controllers.AuthController.CheckFirstRun)

			// 用户登录
			authGroup.POST("/login", controllers.AuthController.Login)

			// 用户退出登录
			authGroup.POST("/logout", controllers.AuthController.Logout)
		}

		// 系统相关 API - 不需要 JWT 验证
		apiGroup.GET("/health", controllers.SystemController.Health)
		apiGroup.GET("/version", controllers.SystemController.Version)

		// SSL 证书管理 API - 部分不需要 JWT 验证
		sslGroup := apiGroup.Group("/ssl")
		{
			// 公开 API - 获取证书状态
			sslGroup.GET("/certificates/:domain", controllers.SSLController.GetCertStatus)
			sslGroup.GET("/certificates", controllers.SSLController.ListCerts)
			sslGroup.GET("/certificates/expiring", controllers.SSLController.GetExpiringCerts)

			// 需要 JWT 验证的 API
			sslGroupProtected := sslGroup.Group("")
			sslGroupProtected.Use(auth.JWTAuthMiddleware(jwtManager))
			{
				// 申请证书
				sslGroupProtected.POST("/certificates", controllers.SSLController.RequestCert)
				// 续签证书
				sslGroupProtected.POST("/certificates/:domain/renew", controllers.SSLController.RenewCert)
				// 删除证书
				sslGroupProtected.DELETE("/certificates/:domain", controllers.SSLController.DeleteCert)
				// 申请通配符证书
				sslGroupProtected.POST("/certificates/wildcard", controllers.SSLController.RequestWildcardCert)
				// 获取续签历史
				sslGroupProtected.GET("/certificates/:domain/renewal-history", controllers.SSLController.GetRenewalHistory)
			}
		}

		// 需要 JWT 验证的 API 组
		protectedGroup := apiGroup.Group("")
		protectedGroup.Use(auth.JWTAuthMiddleware(jwtManager))
		{
			// 系统配置 API
			protectedGroup.GET("/system/config", controllers.SystemController.GetSystemConfig)
			protectedGroup.POST("/system/config", controllers.SystemController.UpdateSystemConfig)

			// 备份恢复 API
			protectedGroup.POST("/system/backup", controllers.SystemController.BackupConfig)
			protectedGroup.POST("/system/restore", controllers.SystemController.RestoreConfig)
			protectedGroup.GET("/system/backups", controllers.SystemController.ListBackups)

			// 修改密码
			protectedGroup.POST("/auth/change-password", controllers.AuthController.ChangePassword)

			// 2FA API (独立前缀避免 Gin 路由冲突)
			protectedGroup.GET("/2fa/status", controllers.AuthController.Get2FAStatus)
			protectedGroup.POST("/2fa/enable", controllers.AuthController.Enable2FA)
			protectedGroup.POST("/2fa/confirm", controllers.AuthController.Confirm2FA)
			protectedGroup.POST("/2fa/disable", controllers.AuthController.Disable2FA)

			// 概览 API
			protectedGroup.GET("/overview", controllers.OverviewController.GetOverview)

			// 监控 API
			protectedGroup.GET("/monitoring/stats", controllers.MonitoringController.GetStats)
			protectedGroup.GET("/monitoring/alerts/history", controllers.MonitoringController.GetAlertHistory)

			// 告警规则 CRUD API
			protectedGroup.GET("/monitoring/alert-rules", controllers.MonitoringController.GetAlertRules)
			protectedGroup.POST("/monitoring/alert-rules", controllers.MonitoringController.SaveAlertRule)
			protectedGroup.DELETE("/monitoring/alert-rules/:id", controllers.MonitoringController.DeleteAlertRule)

			// 告警规则别名路由（兼容前端 /monitoring/alerts/rules）
			protectedGroup.GET("/monitoring/alerts/rules", controllers.MonitoringController.GetAlertRules)
			protectedGroup.POST("/monitoring/alerts/rules", controllers.MonitoringController.SaveAlertRule)

			// 通知渠道 API
			protectedGroup.GET("/monitoring/alerts/channels", controllers.MonitoringController.GetNotificationChannels)
			protectedGroup.POST("/monitoring/alerts/channels", controllers.MonitoringController.SaveNotificationChannels)

			// 防火墙规则 API
			protectedGroup.GET("/firewall/rules", controllers.MonitoringController.GetFirewallRules)
			protectedGroup.POST("/firewall/rules", controllers.MonitoringController.SaveFirewallRules)
			protectedGroup.DELETE("/firewall/rules/:id", controllers.MonitoringController.DeleteFirewallRule)

			// 访问日志 API
			protectedGroup.GET("/logs", controllers.FirewallController.GetAccessLogs)
			protectedGroup.GET("/logs/export", controllers.FirewallController.ExportLogs)
			protectedGroup.GET("/firewall/attacks", controllers.FirewallController.GetAttackLogs)
			protectedGroup.POST("/firewall/whitelist", controllers.FirewallController.AddToWhitelist)
			protectedGroup.POST("/firewall/blacklist", controllers.FirewallController.AddToBlacklist)
			protectedGroup.GET("/firewall/blacklist", controllers.FirewallController.GetBlacklist)
			protectedGroup.GET("/firewall/whitelist", controllers.FirewallController.GetWhitelist)

			// 爬虫日志 API
			protectedGroup.GET("/crawler/logs", controllers.CrawlerController.GetCrawlerLogs)
			protectedGroup.GET("/crawler/stats", controllers.CrawlerController.GetCrawlerStats)

			// SEO API
			seoGroup := protectedGroup.Group("/seo")
			{
				seoGroup.GET("/config", controllers.SEOController.GetSEOConfig)
				seoGroup.PUT("/config", controllers.SEOController.UpdateSEOConfig)
				seoGroup.GET("/sitemap", controllers.SEOController.GetSitemap)
				seoGroup.POST("/sitemap/generate", controllers.SEOController.GenerateSitemap)
				seoGroup.GET("/robots", controllers.SEOController.GetRobotsTxt)
				seoGroup.POST("/robots/generate", controllers.SEOController.GenerateRobotsTxt)
			}

			// 预热 API
			protectedGroup.GET("/preheat/sites", controllers.PreheatController.GetPreheatSites)
			protectedGroup.GET("/preheat/stats", controllers.PreheatController.GetPreheatStats)
			protectedGroup.POST("/preheat/trigger", controllers.PreheatController.TriggerPreheat)
			protectedGroup.GET("/preheat/urls", controllers.PreheatController.GetPreheatUrls)
			protectedGroup.GET("/preheat/task/status", controllers.PreheatController.GetPreheatTaskStatus)
			protectedGroup.GET("/preheat/crawler-headers", controllers.PreheatController.GetCrawlerHeaders)
			protectedGroup.POST("/preheat/clear-cache", controllers.PreheatController.ClearCache)

			// 推送 API
			protectedGroup.GET("/push/sites", controllers.PushController.GetSites)
			protectedGroup.GET("/push/stats", controllers.PushController.GetPushStats)
			protectedGroup.GET("/push/logs", controllers.PushController.GetPushLogs)
			protectedGroup.GET("/push/trend", controllers.PushController.GetPushTrend)
			protectedGroup.GET("/push/config", controllers.PushController.GetPushConfig)
			protectedGroup.POST("/push/config", controllers.PushController.UpdatePushConfig)

			// 站点管理 API
			sitesGroup := protectedGroup.Group("/sites")
			{
				// 获取站点列表
				sitesGroup.GET("", controllers.SitesController.GetSites)

				// 获取单个站点信息
				sitesGroup.GET("/:id", controllers.SitesController.GetSite)

				// 获取站点的 Redis 配置（预渲染或推送配置）
				sitesGroup.GET("/:id/config", controllers.SitesController.GetSiteConfig)

				// WAF Configuration
				sitesGroup.GET("/:id/waf", controllers.FirewallController.GetWafConfig)
				sitesGroup.PUT("/:id/waf", controllers.FirewallController.UpdateWafConfig)

				// Independent Config Updates
				sitesGroup.PUT("/:id/prerender", controllers.SitesController.UpdateSitePrerenderConfig)
				sitesGroup.PUT("/:id/push", controllers.SitesController.UpdateSitePushConfig)
				sitesGroup.PUT("/:id/firewall", controllers.SitesController.UpdateSiteFirewallConfig)

				// 添加站点
				sitesGroup.POST("", controllers.SitesController.AddSite)

				// 更新站点
				sitesGroup.PUT("/:id", controllers.SitesController.UpdateSite)

				// 删除站点
				sitesGroup.DELETE("/:id", controllers.SitesController.DeleteSite)

				// 启动/停止站点服务器
				sitesGroup.POST("/:id/start", controllers.SitesController.StartSite)
				sitesGroup.POST("/:id/stop", controllers.SitesController.StopSite)

				// 静态资源管理 API
				// 获取站点的静态资源文件列表
				sitesGroup.GET("/:id/static", controllers.SitesController.GetStaticFiles)

				// 上传静态资源文件
				sitesGroup.POST("/:id/static", controllers.SitesController.UploadStaticFile)

				// 解压文件
				sitesGroup.POST("/:id/static/extract", controllers.SitesController.ExtractFile)

				// 删除静态资源文件
				sitesGroup.DELETE("/:id/static", controllers.SitesController.DeleteStaticFile)

				// 批量删除静态资源文件
				sitesGroup.POST("/:id/static/batch-delete", controllers.SitesController.BatchDeleteStaticFiles)

			}
		}
	}
}
