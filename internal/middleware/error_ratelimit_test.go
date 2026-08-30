package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"prerender-shield/internal/models"
	"prerender-shield/internal/redis"
)

func TestNewAPIResponse(t *testing.T) {
	resp := NewAPIResponse(map[string]string{"k": "v"})
	if resp.Code != 0 || resp.Message != "success" || resp.Data == nil {
		t.Fatalf("APIResponse broken: %+v", resp)
	}
}

func TestNewErrorResponse(t *testing.T) {
	errResp := NewErrorResponse(400, "bad input", "detail1")
	if errResp.Code != 400 || errResp.Message != "bad input" {
		t.Fatalf("ErrorResponse broken: %+v", errResp)
	}
}

func TestGlobalErrorHandler_RecoversPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(GlobalErrorHandler())
	router.GET("/boom", func(c *gin.Context) { panic("explode") })
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("panic must yield 500, got %d", w.Code)
	}
}

func TestNewRedisRateLimiter_Middleware(t *testing.T) {
	client, err := redis.NewClient("localhost:6379", "", 0)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	gin.SetMode(gin.TestMode)
	rl := NewRedisRateLimiter(client, 3, 60*time.Second, 30*time.Second) // 3 次/分钟，封 30s

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	// 唯一 IP：避开历史封禁键/计数桶残留（TTL 内会立即 429）
	nano := time.Now().UnixNano()
	clientIP := fmt.Sprintf("172.%d.%d.%d:1234", (nano/1e9)%200+20, (nano/1e5)%250+1, nano%250+2)
	codes := []int{}
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = clientIP
		router.ServeHTTP(w, req)
		codes = append(codes, w.Code)
	}
	// 前 3 次通过，之后限流（429）
	if codes[0] != http.StatusOK || codes[1] != http.StatusOK || codes[2] != http.StatusOK {
		t.Fatalf("first three must pass: %v", codes)
	}
	if codes[3] == http.StatusOK || codes[4] == http.StatusOK {
		t.Fatalf("4th/5th must be limited: %v", codes)
	}
}

func TestManagementRateLimit(t *testing.T) {
	client, err := redis.NewClient("localhost:6379", "", 0)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	rl := NewRedisRateLimiter(client, 5, 60*time.Second, 30*time.Second)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ManagementRateLimit(rl))
	router.POST("/auth/login", func(c *gin.Context) { c.Status(http.StatusOK) })
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		req.RemoteAddr = "10.8.8.8:1234"
		router.ServeHTTP(w, req)
	}
}

func TestWafLogWriter_Write(t *testing.T) {
	lw := NewWafLogWriter(nil, 1, 10*time.Millisecond)
	lw.Write(models.AccessLog{ID: "t1", IPAddress: "1.2.3.4", RuleID: "test"})
	lw.Stop()
}
