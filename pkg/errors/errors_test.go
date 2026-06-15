package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppError_Error(t *testing.T) {
	// Test with internal error
	err := &AppError{
		Code:     CodeInternalError,
		Message:  "test message",
		Internal: errors.New("internal cause"),
	}
	assert.Contains(t, err.Error(), "test message")
	assert.Contains(t, err.Error(), "internal cause")

	// Test without internal error
	err = &AppError{
		Code:    CodeBadRequest,
		Message: "bad request",
	}
	assert.Equal(t, "bad request", err.Error())
}

func TestAppError_Unwrap(t *testing.T) {
	cause := errors.New("original error")
	err := &AppError{
		Code:     CodeInternalError,
		Message:  "test message",
		Internal: cause,
	}
	assert.Equal(t, cause, err.Unwrap())
}

func TestNewError(t *testing.T) {
	err := NewError(CodeBadRequest, "new error message")
	assert.Equal(t, CodeBadRequest, err.Code)
	assert.Equal(t, "new error message", err.Message)
	assert.Nil(t, err.Internal)
}

func TestWrapError(t *testing.T) {
	cause := errors.New("original error")
	err := WrapError(cause, CodeInternalError, "wrapped message")

	assert.Equal(t, CodeInternalError, err.Code)
	assert.Equal(t, "wrapped message", err.Message)
	assert.Equal(t, cause, err.Internal)
	assert.NotEmpty(t, err.Stack)
}

func TestPredefinedErrors(t *testing.T) {
	// ErrBadRequest
	err := ErrBadRequest("bad input")
	assert.Equal(t, CodeBadRequest, err.Code)
	assert.Equal(t, "bad input", err.Message)

	// ErrUnauthorized
	err = ErrUnauthorized("unauthorized")
	assert.Equal(t, CodeUnauthorized, err.Code)

	// ErrForbidden
	err = ErrForbidden("forbidden")
	assert.Equal(t, CodeForbidden, err.Code)

	// ErrNotFound
	err = ErrNotFound("not found")
	assert.Equal(t, CodeNotFound, err.Code)

	// ErrInternal
	err = ErrInternal(errors.New("internal"))
	assert.Equal(t, CodeInternalError, err.Code)
	assert.NotEmpty(t, err.Stack)

	// ErrServiceUnavailable
	err = ErrServiceUnavailable("down")
	assert.Equal(t, CodeServiceUnavailable, err.Code)

	// ErrRateLimit
	err = ErrRateLimit("too many")
	assert.Equal(t, CodeRateLimit, err.Code)

	// ErrValidation
	err = ErrValidation("invalid", map[string]string{"field": "name"})
	assert.Equal(t, CodeValidation, err.Code)
	assert.NotNil(t, err.Details)
}

func TestErrorCodes(t *testing.T) {
	assert.Equal(t, 200, CodeSuccess)
	assert.Equal(t, 400, CodeBadRequest)
	assert.Equal(t, 401, CodeUnauthorized)
	assert.Equal(t, 403, CodeForbidden)
	assert.Equal(t, 404, CodeNotFound)
	assert.Equal(t, 429, CodeRateLimit)
	assert.Equal(t, 422, CodeValidation)
	assert.Equal(t, 500, CodeInternalError)
	assert.Equal(t, 503, CodeServiceUnavailable)
}

func TestGetHTTPStatus(t *testing.T) {
	assert.Equal(t, 200, getHTTPStatus(CodeSuccess))
	assert.Equal(t, 400, getHTTPStatus(CodeBadRequest))
	assert.Equal(t, 401, getHTTPStatus(CodeUnauthorized))
	assert.Equal(t, 403, getHTTPStatus(CodeForbidden))
	assert.Equal(t, 404, getHTTPStatus(CodeNotFound))
	assert.Equal(t, 429, getHTTPStatus(CodeRateLimit))
	assert.Equal(t, 503, getHTTPStatus(CodeServiceUnavailable))
	assert.Equal(t, 500, getHTTPStatus(99999)) // unknown code defaults to 500
}

func TestResponse_Structure(t *testing.T) {
	resp := Response{
		Code:    CodeSuccess,
		Message: "success",
		Data:    map[string]string{"key": "value"},
	}
	assert.Equal(t, CodeSuccess, resp.Code)
	assert.Equal(t, "success", resp.Message)
	assert.NotNil(t, resp.Data)
}

func TestResponse_WithDetails(t *testing.T) {
	resp := Response{
		Code:    CodeBadRequest,
		Message: "validation failed",
		Details: []string{"field1 is required", "field2 is invalid"},
	}
	assert.Equal(t, CodeBadRequest, resp.Code)
	assert.NotNil(t, resp.Details)
}

func TestGetStack(t *testing.T) {
	stack := getStack()
	assert.NotEmpty(t, stack)
	assert.Contains(t, stack, "Stack trace:")
}
