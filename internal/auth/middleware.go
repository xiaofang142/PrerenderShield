package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// apiTokenPathPrefix 管理 API Token 仅可用于该前缀下的端点（缓存自动化运维，如 CI 发布钩子）
const apiTokenPathPrefix = "/api/v1/preheat/"

// JWTAuthMiddleware JWT认证中间件。
// apiTokenProvider 返回管理 API Token 的 sha256 hex 列表（nil/空返回值=禁用回退鉴权）：
// JWT 校验失败时，若请求命中 /api/v1/preheat/ 前缀且 Bearer Token 命中任一哈希，则放行
// 并标记 auth_via=api_token。WebSocket 组应显式传 nil 保持仅 JWT。
func JWTAuthMiddleware(jwtManager *JWTManager, apiTokenProvider func() []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取Authorization头
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" && strings.HasPrefix(c.Request.URL.Path, "/ws/") {
			// WebSocket 场景：浏览器 WS API 无法携带 Authorization 头，
			// 仅 /ws 端点允许 ?token= 查询参数（避免 JWT 泄入普通 API 访问日志）
			if qt := c.Query("token"); qt != "" {
				authHeader = "Bearer " + qt
			}
		}
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": ErrNoAuthHeader.Error(),
			})
			c.Abort()
			return
		}

		// 验证Authorization格式
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": ErrInvalidAuthFormat.Error(),
			})
			c.Abort()
			return
		}

		// 验证令牌
		claims, err := jwtManager.ValidateToken(parts[1])
		if err != nil {
			// 管理 API Token 回退：仅限 /preheat/ 运维端点，避免 Token 泄露放大为全 API 权限
			if apiTokenProvider != nil &&
				strings.HasPrefix(c.Request.URL.Path, apiTokenPathPrefix) &&
				VerifyToken(parts[1], apiTokenProvider()) {
				c.Set("auth_via", "api_token")
				c.Next()
				return
			}
			statusCode := http.StatusUnauthorized
			if err == ErrExpiredToken {
				statusCode = http.StatusUnauthorized
			}
			c.JSON(statusCode, gin.H{
				"code":    statusCode,
				"message": err.Error(),
			})
			c.Abort()
			return
		}

		// 将用户信息存储到上下文
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)

		c.Next()
	}
}
