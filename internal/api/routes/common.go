package routes

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/utils"
)

// 添加安全头中间件
func addSecurityHeaders(ginRouter *gin.Engine) {
	ginRouter.Use(func(c *gin.Context) {
		// Content-Security-Policy (CSP) 头，防止XSS攻击
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'")

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
}

var (
	customAllowedOriginsMu sync.RWMutex
	// customAllowedOrigins 运行时可被 SetAllowedOrigins 重写（配置热更新），
	// 而 CORS 中间件每请求并发读，必须用 RWMutex 保护
	customAllowedOrigins map[string]bool
)

func SetAllowedOrigins(origins []string) {
	utils.SetAllowedOrigins(origins)
}

func isOriginAllowed(origin string) bool {
	if allowedOriginsStatic[origin] {
		return true
	}
	customAllowedOriginsMu.RLock()
	defer customAllowedOriginsMu.RUnlock()
	return customAllowedOrigins[origin]
}

// allowedOriginsStatic 内置允许来源（不可变，无需加锁）
var allowedOriginsStatic = map[string]bool{
	"http://localhost:9597": true,
	"http://localhost:3000": true,
	"http://127.0.0.1:9597": true,
	"http://127.0.0.1:3000": true,
}

// 添加CORS中间件
func addCorsMiddleware(ginRouter *gin.Engine) {
	ginRouter.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			origin = c.Request.Header.Get("Referer")
		}

		if utils.IsOriginAllowed(origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, Authorization")
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})
}
