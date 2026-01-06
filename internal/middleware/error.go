package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"prerender-shield/internal/logging"
)

// GlobalErrorHandler 全局错误处理中间件
func GlobalErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 记录堆栈信息
				logging.DefaultLogger.Error("Panic recovered: %v\nStack: %s", err, string(debug.Stack()))

				// 返回 500 错误
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    500,
					"message": "Internal Server Error",
					"error":   "An unexpected error occurred",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}
