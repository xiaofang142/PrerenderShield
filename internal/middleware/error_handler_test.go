package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGlobalErrorHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(GlobalErrorHandler())
	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})
	router.GET("/ok", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 测试正常请求
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 测试恐慌恢复
	req = httptest.NewRequest(http.MethodGet, "/panic", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Internal Server Error")
}

func TestGlobalErrorHandler_NoPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(GlobalErrorHandler())
	router.GET("/normal", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/normal", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")
}
