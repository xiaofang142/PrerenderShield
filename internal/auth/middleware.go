package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// JWTAuthMiddleware JWT认证中间件
func JWTAuthMiddleware(jwtManager *JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取Authorization头
		authHeader := c.GetHeader("Authorization")
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
