package errors

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
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

// ErrorHandler 中间件：AppError 分支 + 未知错误分支
func TestErrorHandler_AppErrorAndUnknown(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// AppError：映射 HTTP 状态码
	r1 := gin.New()
	r1.Use(ErrorHandler())
	r1.GET("/bad", func(c *gin.Context) {
		_ = c.Error(NewError(CodeBadRequest, "bad input"))
	})
	w := httptest.NewRecorder()
	r1.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/bad", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("AppError mapping broken: %d", w.Code)
	}

	// 未知错误 → 500
	r2 := gin.New()
	r2.Use(ErrorHandler())
	r2.GET("/boom", func(c *gin.Context) {
		_ = c.Error(errors.New("raw failure"))
	})
	w = httptest.NewRecorder()
	r2.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("unknown error must 500, got %d", w.Code)
	}

	// 无错误 → 直通
	w = httptest.NewRecorder()
	r2.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))
	_ = w
}

// Success / Error 响应辅助
func TestSuccessAndErrorResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ok", func(c *gin.Context) { Success(c, map[string]string{"k": "v"}) })
	r.GET("/err", func(c *gin.Context) { Error(c, NewError(CodeNotFound, "missing")) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != CodeSuccess || resp.Data == nil {
		t.Fatalf("Success broken: %+v", resp)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/err", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("Error status mapping broken: %d", w.Code)
	}
}

// getHTTPStatus 全分支
func TestGetHTTPStatus_AllBranches(t *testing.T) {
	cases := []struct {
		code int
		want int
	}{
		{CodeSuccess, http.StatusOK},
		{CodeBadRequest, http.StatusBadRequest},
		{CodeValidation, http.StatusBadRequest},
		{CodeUnauthorized, http.StatusUnauthorized},
		{CodeForbidden, http.StatusForbidden},
		{CodeNotFound, http.StatusNotFound},
		{CodeRateLimit, http.StatusTooManyRequests},
		{CodeServiceUnavailable, http.StatusServiceUnavailable},
		{999999, http.StatusInternalServerError},
	}
	for _, c := range cases {
		if got := getHTTPStatus(c.code); got != c.want {
			t.Errorf("getHTTPStatus(%d)=%d want %d", c.code, got, c.want)
		}
	}
}

// WrapError 携带堆栈与内部错误
func TestWrapError_StackAndInternal(t *testing.T) {
	inner := errors.New("root cause")
	appErr := WrapError(inner, CodeInternalError, "wrapped")
	if appErr.Internal != inner {
		t.Fatal("internal error lost")
	}
	if !strings.Contains(appErr.Stack, "Stack trace:") {
		t.Fatalf("stack missing: %q", appErr.Stack)
	}
}

func TestErrorWithData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ed", func(c *gin.Context) {
		ErrorWithData(c, NewError(CodeValidation, "fields"), map[string]string{"f1": "bad"})
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ed", nil))
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != CodeValidation || resp.Details == nil {
		t.Fatalf("ErrorWithData broken: %+v", resp)
	}
}
