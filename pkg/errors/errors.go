package errors

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
)

// AppError 应用错误
type AppError struct {
	Code     int         `json:"code"`
	Message  string      `json:"message"`
	Details  interface{} `json:"details,omitempty"`
	Internal error       `json:"-"`
	Stack    string      `json:"-"`
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	if e.Internal != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Internal)
	}
	return e.Message
}

// Unwrap 返回内部错误
func (e *AppError) Unwrap() error {
	return e.Internal
}

// 新增错误码常量
const (
	CodeSuccess          = 200
	CodeBadRequest       = 400
	CodeUnauthorized     = 401
	CodeForbidden        = 403
	CodeNotFound         = 404
	CodeInternalError    = 500
	CodeServiceUnavailable = 503
	CodeRateLimit        = 429
	CodeValidation       = 422
)

// 预定义错误
var (
	ErrBadRequest = func(msg string) *AppError {
		return &AppError{
			Code:    CodeBadRequest,
			Message: msg,
		}
	}

	ErrUnauthorized = func(msg string) *AppError {
		return &AppError{
			Code:    CodeUnauthorized,
			Message: msg,
		}
	}

	ErrForbidden = func(msg string) *AppError {
		return &AppError{
			Code:    CodeForbidden,
			Message: msg,
		}
	}

	ErrNotFound = func(msg string) *AppError {
		return &AppError{
			Code:    CodeNotFound,
			Message: msg,
		}
	}

	ErrInternal = func(err error) *AppError {
		return &AppError{
			Code:     CodeInternalError,
			Message:  "Internal server error",
			Internal: err,
			Stack:    getStack(),
		}
	}

	ErrServiceUnavailable = func(msg string) *AppError {
		return &AppError{
			Code:    CodeServiceUnavailable,
			Message: msg,
		}
	}

	ErrRateLimit = func(msg string) *AppError {
		return &AppError{
			Code:    CodeRateLimit,
			Message: msg,
		}
	}

	ErrValidation = func(msg string, details interface{}) *AppError {
		return &AppError{
			Code:    CodeValidation,
			Message: msg,
			Details: details,
		}
	}
)

// NewError 创建新错误
func NewError(code int, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// WrapError 包装错误
func WrapError(err error, code int, message string) *AppError {
	return &AppError{
		Code:     code,
		Message:  message,
		Internal: err,
		Stack:    getStack(),
	}
}

// getStack 获取调用栈
func getStack() string {
	var pcs [32]uintptr
	n := runtime.Callers(3, pcs[:])

	var sb strings.Builder
	sb.WriteString("Stack trace:\n")

	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if !more {
			break
		}
		sb.WriteString(fmt.Sprintf("  %s:%d %s\n", frame.File, frame.Line, frame.Function))
	}

	return sb.String()
}

// ErrorHandler 错误处理中间件
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err

			if appErr, ok := err.(*AppError); ok {
				c.JSON(getHTTPStatus(appErr.Code), gin.H{
					"code":    appErr.Code,
					"message": appErr.Message,
					"details": appErr.Details,
				})
				return
			}

			// 未知错误
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    CodeInternalError,
				"message": "Internal server error",
			})
		}
	}
}

// getHTTPStatus 根据错误码获取 HTTP 状态码
func getHTTPStatus(code int) int {
	switch code {
	case CodeSuccess:
		return http.StatusOK
	case CodeBadRequest, CodeValidation:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeRateLimit:
		return http.StatusTooManyRequests
	case CodeServiceUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
	})
}

// Error 错误响应
func Error(c *gin.Context, err *AppError) {
	c.JSON(getHTTPStatus(err.Code), Response{
		Code:    err.Code,
		Message: err.Message,
	})
}

// ErrorWithData 带数据的错误响应
func ErrorWithData(c *gin.Context, err *AppError, details interface{}) {
	err.Details = details
	c.JSON(getHTTPStatus(err.Code), Response{
		Code:    err.Code,
		Message: err.Message,
		Details: details,
	})
}
