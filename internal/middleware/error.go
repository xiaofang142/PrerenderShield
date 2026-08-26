package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"prerender-shield/internal/logging"
)

// APIError 统一 API 错误响应
type APIError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// NewAPIError 创建 API 错误
func NewAPIError(code int, message string, details ...interface{}) APIError {
	err := APIError{Code: code, Message: message}
	if len(details) > 0 {
		err.Details = details[0]
	}
	return err
}

// APIResponse 统一 API 成功响应
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewAPIResponse 创建 API 成功响应
func NewAPIResponse(data interface{}) APIResponse {
	return APIResponse{Code: 0, Message: "success", Data: data}
}

// GlobalErrorHandler 全局错误处理中间件
func GlobalErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logging.DefaultLogger.Error("Panic recovered: %v\nStack: %s", err, string(debug.Stack()))
				c.JSON(http.StatusInternalServerError, NewAPIError(
					http.StatusInternalServerError,
					"Internal Server Error",
				))
				c.Abort()
			}
		}()
		c.Next()

		if len(c.Errors) > 0 {
			lastErr := c.Errors.Last()
			status := c.Writer.Status()
			if status == 0 || status == http.StatusOK {
				status = http.StatusInternalServerError
			}
			logging.DefaultLogger.Error("Request error: %v", lastErr)
			c.JSON(status, NewAPIError(status, lastErr.Error()))
		}
	}
}

// ErrorCode 业务错误码
type ErrorCode int

const (
	ErrCodeBadRequest     ErrorCode = 40000
	ErrCodeUnauthorized   ErrorCode = 40100
	ErrCodeForbidden      ErrorCode = 40300
	ErrCodeNotFound       ErrorCode = 40400
	ErrCodeRateLimit      ErrorCode = 42900
	ErrCodeInternal       ErrorCode = 50000
	ErrCodeValidation     ErrorCode = 40001
	ErrCodeDuplicate      ErrorCode = 40900
	ErrCodeDependencyFail ErrorCode = 50200
)

// ErrorResponse 带业务码的错误响应
type ErrorResponse struct {
	Code    ErrorCode   `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// NewErrorResponse 创建业务错误响应
func NewErrorResponse(code ErrorCode, message string, details ...interface{}) ErrorResponse {
	resp := ErrorResponse{Code: code, Message: message}
	if len(details) > 0 {
		resp.Details = details[0]
	}
	return resp
}
