package errors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestError_Error(t *testing.T) {
	// Test with cause
	err := &Error{
		Code:    "TEST_CODE",
		Message: "test message",
		Cause:   assert.AnError,
	}
	assert.Contains(t, err.Error(), "TEST_CODE")
	assert.Contains(t, err.Error(), "test message")
	assert.Contains(t, err.Error(), "assert.AnError")

	// Test without cause
	err = &Error{
		Code:    "TEST_CODE",
		Message: "test message",
	}
	assert.Equal(t, "TEST_CODE: test message", err.Error())
}

func TestError_Unwrap(t *testing.T) {
	cause := assert.AnError
	err := &Error{
		Code:    "TEST_CODE",
		Message: "test message",
		Cause:   cause,
	}
	assert.Equal(t, cause, err.Unwrap())
}

func TestError_WithContext(t *testing.T) {
	err := &Error{
		Code:    "TEST_CODE",
		Message: "test message",
	}

	result := err.WithContext("key1", "value1")
	assert.Equal(t, "value1", result.Context["key1"])

	// Chain multiple contexts
	result.WithContext("key2", 123)
	assert.Equal(t, 123, err.Context["key2"])
}

func TestWrap(t *testing.T) {
	cause := assert.AnError
	err := Wrap(cause, "TEST_CODE", "wrapped message")

	assert.Equal(t, "TEST_CODE", err.Code)
	assert.Equal(t, "wrapped message", err.Message)
	assert.Equal(t, cause, err.Cause)
	assert.ErrorContains(t, err, "TEST_CODE: wrapped message")
}

func TestNew(t *testing.T) {
	err := New("TEST_CODE", "new error message")

	assert.Equal(t, "TEST_CODE", err.Code)
	assert.Equal(t, "new error message", err.Message)
	assert.Nil(t, err.Cause)
}

func TestPredefinedErrors(t *testing.T) {
	assert.Equal(t, ErrInternal, ErrInternalServer.Code)
	assert.Equal(t, ErrInvalidParam, ErrInvalidRequest.Code)
	assert.Equal(t, ErrUnauthorized, ErrUnauthorizedReq.Code)
	assert.Equal(t, ErrForbidden, ErrForbiddenReq.Code)
	assert.Equal(t, ErrNotFound, ErrNotFoundGeneric.Code)
}

func TestErrorCodes(t *testing.T) {
	// Common errors
	assert.Equal(t, "UNKNOWN", ErrUnknown)
	assert.Equal(t, "INTERNAL_ERROR", ErrInternal)
	assert.Equal(t, "INVALID_PARAM", ErrInvalidParam)
	assert.Equal(t, "NOT_FOUND", ErrNotFound)
	assert.Equal(t, "ALREADY_EXISTS", ErrAlreadyExists)
	assert.Equal(t, "UNAUTHORIZED", ErrUnauthorized)
	assert.Equal(t, "FORBIDDEN", ErrForbidden)
	assert.Equal(t, "TIMEOUT", ErrTimeout)

	// WAF errors
	assert.Equal(t, "WAF_BLOCKED", ErrWAFBlocked)
	assert.Equal(t, "WAF_RULE_INVALID", ErrWAFRuleInvalid)
	assert.Equal(t, "WAF_CHECK_FAILED", ErrWAFCheckFailed)

	// Render errors
	assert.Equal(t, "RENDER_TIMEOUT", ErrRenderTimeout)
	assert.Equal(t, "RENDER_FAILED", ErrRenderFailed)
	assert.Equal(t, "CRAWLER_BLOCKED", ErrCrawlerBlocked)

	// Cache errors
	assert.Equal(t, "CACHE_MISS", ErrCacheMiss)
	assert.Equal(t, "CACHE_INVALID", ErrCacheInvalid)

	// Auth errors
	assert.Equal(t, "TOKEN_INVALID", ErrTokenInvalid)
	assert.Equal(t, "TOKEN_EXPIRED", ErrTokenExpired)
	assert.Equal(t, "USER_NOT_FOUND", ErrUserNotFound)
	assert.Equal(t, "INVALID_PASSWORD", ErrInvalidPassword)

	// Site errors
	assert.Equal(t, "SITE_NOT_FOUND", ErrSiteNotFound)
	assert.Equal(t, "SITE_EXISTS", ErrSiteExists)
	assert.Equal(t, "SITE_CONFIG_INVALID", ErrSiteConfigInvalid)
}

func TestError_Is(t *testing.T) {
	err1 := New(ErrNotFound, "resource not found")
	err2 := New(ErrNotFound, "another not found")

	// Both have same code but are different instances
	assert.Equal(t, err1.Code, err2.Code)
	assert.NotEqual(t, err1.Message, err2.Message)
}
