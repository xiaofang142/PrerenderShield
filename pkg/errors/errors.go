package errors

import (
	"fmt"
	"net/http"
)

// Error 统一错误类型
type Error struct {
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	HTTPStatus int                    `json:"-"`
	Cause      error                  `json:"-"`
	Context    map[string]interface{} `json:"context,omitempty"`
}

// Error implements error interface
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause
func (e *Error) Unwrap() error {
	return e.Cause
}

// WithContext adds context to the error
func (e *Error) WithContext(key string, value interface{}) *Error {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// Wrap wraps an error with code and message
func Wrap(err error, code string, message string) *Error {
	return &Error{
		Code:       code,
		Message:    message,
		Cause:      err,
		HTTPStatus: http.StatusInternalServerError,
	}
}

// New creates a new error with code and message
func New(code string, message string) *Error {
	return &Error{
		Code:       code,
		Message:    message,
		HTTPStatus: http.StatusInternalServerError,
	}
}

// Pre-defined error codes
const (
	// Common errors
	ErrUnknown       = "UNKNOWN"
	ErrInternal      = "INTERNAL_ERROR"
	ErrInvalidParam  = "INVALID_PARAM"
	ErrNotFound      = "NOT_FOUND"
	ErrAlreadyExists = "ALREADY_EXISTS"
	ErrUnauthorized  = "UNAUTHORIZED"
	ErrForbidden     = "FORBIDDEN"
	ErrTimeout       = "TIMEOUT"

	// WAF errors
	ErrWAFBlocked     = "WAF_BLOCKED"
	ErrWAFRuleInvalid = "WAF_RULE_INVALID"
	ErrWAFCheckFailed = "WAF_CHECK_FAILED"

	// Render errors
	ErrRenderTimeout  = "RENDER_TIMEOUT"
	ErrRenderFailed   = "RENDER_FAILED"
	ErrCrawlerBlocked = "CRAWLER_BLOCKED"

	// Cache errors
	ErrCacheMiss    = "CACHE_MISS"
	ErrCacheInvalid = "CACHE_INVALID"

	// Auth errors
	ErrTokenInvalid    = "TOKEN_INVALID"
	ErrTokenExpired    = "TOKEN_EXPIRED"
	ErrUserNotFound    = "USER_NOT_FOUND"
	ErrInvalidPassword = "INVALID_PASSWORD"

	// Site errors
	ErrSiteNotFound      = "SITE_NOT_FOUND"
	ErrSiteExists        = "SITE_EXISTS"
	ErrSiteConfigInvalid = "SITE_CONFIG_INVALID"
)

// Common error instances
var (
	ErrInternalServer  = New(ErrInternal, "internal server error")
	ErrInvalidRequest  = New(ErrInvalidParam, "invalid request parameters")
	ErrUnauthorizedReq = New(ErrUnauthorized, "unauthorized request")
	ErrForbiddenReq    = New(ErrForbidden, "forbidden request")
	ErrNotFoundGeneric = New(ErrNotFound, "resource not found")
)
