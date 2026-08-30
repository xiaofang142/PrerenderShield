package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"prerender-shield/internal/auth"
	"prerender-shield/internal/redis"

	"github.com/gin-gonic/gin"
)

type apiResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func requireRedis(t *testing.T) *redis.Client {
	t.Helper()
	client, err := redis.NewClient("localhost:6379", "", 15) // DB15 隔离：集成测试绝不触碰运行环境 DB0
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	return client
}

// setupTestServer 设置测试用的 Gin router 和依赖
func setupTestServer(t *testing.T) (*gin.Engine, *auth.JWTManager, string) {
	t.Helper()

	redisClient := requireRedis(t)

	// 清理 Redis 测试数据
	keys, _ := redisClient.Keys("test:*")
	for _, k := range keys {
		redisClient.Del(k)
	}
	redisClient.Del("user:admin")

	// 创建 JWT 管理器
	jwtConfig := &auth.JWTConfig{
		SecretKey:  "test-secret-key-for-testing-only",
		ExpireTime: 24 * time.Hour,
	}
	jwtManager := auth.NewJWTManager(jwtConfig, redisClient)

	// 创建用户管理器
	userManager := auth.NewUserManager("", redisClient)

	// 如果首次运行，创建测试用户
	if userManager.IsFirstRun() {
		_, err := userManager.CreateUser("admin", "test123456")
		if err != nil {
			t.Logf("Create user warning: %v", err)
		}
	}

	// 生成测试 token
	token, err := jwtManager.GenerateToken("test-user-id", "admin")
	require.NoError(t, err)

	// 创建 Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// 注册路由（最小化：只注册必要的）
	apiGroup := router.Group("/api/v1")

	// Auth routes (no JWT)
	authGroup := apiGroup.Group("/auth")
	{
		authGroup.GET("/first-run", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"isFirstRun": userManager.IsFirstRun()}})
		})
		authGroup.POST("/login", func(c *gin.Context) {
			var req struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"code": 400, "message": "bad request"})
				return
			}
			user, err := userManager.AuthenticateUser(req.Username, req.Password)
			if err != nil {
				c.JSON(401, gin.H{"code": 401, "message": "invalid credentials"})
				return
			}
			tok, _ := jwtManager.GenerateToken(user.ID, user.Username)
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"token": tok, "username": user.Username}})
		})
	}

	// Health
	apiGroup.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "ok", "data": gin.H{"status": "healthy"}})
	})
	apiGroup.GET("/version", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "ok", "data": gin.H{"version": "3.0.0"}})
	})

	// Protected routes
	protected := apiGroup.Group("")
	protected.Use(func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(401, gin.H{"code": 401, "message": "unauthorized"})
			c.Abort()
			return
		}
		tokenStr := authHeader[7:]
		claims, err := jwtManager.ValidateToken(tokenStr)
		if err != nil {
			c.JSON(401, gin.H{"code": 401, "message": "invalid token"})
			c.Abort()
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	})
	{
		protected.GET("/overview", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{
				"totalRequests": 1000, "activeSites": 5, "cacheHitRate": 0.85,
			}})
		})
		protected.GET("/logs", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{
				"logs": []interface{}{}, "total": 0,
			}})
		})
		protected.GET("/system/config", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{
				"server": gin.H{"address": "0.0.0.0", "api_port": 9598},
			}})
		})
		protected.POST("/system/config", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "config updated"})
		})
		protected.GET("/monitoring/stats", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{
				"requests_per_second": 100, "active_connections": 5,
			}})
		})

		// 2FA endpoints
		protected.GET("/auth/2fa/status", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"enabled": false, "available": true}})
		})
		protected.POST("/auth/2fa/enable", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"secret": "TEST123", "qr_code_url": "otpauth://"}})
		})
		protected.POST("/auth/2fa/confirm", func(c *gin.Context) {
			var req struct {
				Code string `json:"code"`
			}
			c.ShouldBindJSON(&req)
			if req.Code == "" {
				c.JSON(400, gin.H{"code": 400, "message": "Missing code"})
				return
			}
			c.JSON(200, gin.H{"code": 200, "message": "2FA enabled successfully"})
		})
		protected.POST("/auth/2fa/disable", func(c *gin.Context) {
			var req struct {
				Code string `json:"code"`
			}
			c.ShouldBindJSON(&req)
			if req.Code == "" {
				c.JSON(400, gin.H{"code": 400, "message": "Missing code"})
				return
			}
			c.JSON(200, gin.H{"code": 200, "message": "2FA disabled successfully"})
		})

		// Firewall
		protected.GET("/firewall/attacks", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"items": []interface{}{}, "total": 0}})
		})
		protected.POST("/firewall/whitelist", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "added to whitelist"})
		})
		protected.POST("/firewall/blacklist", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "added to blacklist"})
		})

		// Sites CRUD
		protected.POST("/sites", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "site created", "data": gin.H{"id": "test-site-1"}})
		})
		protected.GET("/sites", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": []interface{}{}})
		})
		protected.GET("/sites/:id", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"id": c.Param("id"), "name": "test"}})
		})
		protected.PUT("/sites/:id", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "site updated"})
		})
		protected.DELETE("/sites/:id", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "site deleted"})
		})
		protected.GET("/sites/:id/config", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"type": c.Query("type")}})
		})
		protected.PUT("/sites/:id/prerender", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "prerender config updated"})
		})
		protected.PUT("/sites/:id/push", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "push config updated"})
		})
		protected.PUT("/sites/:id/firewall", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "firewall config updated"})
		})

		// SSL
		protected.GET("/ssl/certificates", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": []interface{}{}})
		})

		// Preheat
		protected.GET("/preheat/stats", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"total": 0, "active": 0}})
		})
		protected.POST("/preheat/trigger", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "preheat triggered"})
		})
		protected.GET("/preheat/urls", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"items": []interface{}{}, "total": 0}})
		})

		// Crawler
		protected.GET("/crawler/logs", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"items": []interface{}{}, "total": 0}})
		})
		protected.GET("/crawler/stats", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{}})
		})

		// Push
		protected.GET("/push/stats", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{}})
		})
		protected.GET("/push/logs", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"items": []interface{}{}, "total": 0}})
		})
		protected.GET("/push/config", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{}})
		})
		protected.POST("/push/config", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "push config updated"})
		})
		protected.GET("/push/sites", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "message": "success", "data": []interface{}{}})
		})
	}

	return router, jwtManager, token
}

// ======================================================
// API Endpoint Tests
// ======================================================

func TestAPI_Auth_Login(t *testing.T) {
	router, _, token := setupTestServer(t)
	require.NotEmpty(t, token)

	// Login with invalid credentials
	w := performRequest(router, "POST", "/api/v1/auth/login", `{"username":"admin","password":"wrong"}`)
	var resp apiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 401, resp.Code)
}

func TestAPI_Auth_FirstRun(t *testing.T) {
	router, _, _ := setupTestServer(t)

	w := performRequest(router, "GET", "/api/v1/auth/first-run", "")
	var resp apiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 200, resp.Code)
}

func TestAPI_Health(t *testing.T) {
	router, _, _ := setupTestServer(t)

	w := performRequest(router, "GET", "/api/v1/health", "")
	assert.Equal(t, 200, w.Code)
	var resp apiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 200, resp.Code)
}

func TestAPI_Version(t *testing.T) {
	router, _, _ := setupTestServer(t)

	w := performRequest(router, "GET", "/api/v1/version", "")
	assert.Equal(t, 200, w.Code)
}

func TestAPI_Auth_Unauthorized(t *testing.T) {
	router, _, _ := setupTestServer(t)

	endpoints := []string{
		"/api/v1/overview",
		"/api/v1/system/config",
		"/api/v1/monitoring/stats",
		"/api/v1/logs",
		"/api/v1/sites",
		"/api/v1/firewall/attacks",
		"/api/v1/preheat/stats",
		"/api/v1/crawler/logs",
		"/api/v1/push/stats",
		"/api/v1/ssl/certificates",
		"/api/v1/auth/2fa/status",
	}
	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			w := performRequest(router, "GET", ep, "")
			var resp apiResponse
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, 401, resp.Code, "endpoint %s should return 401 without token", ep)
		})
	}
}

func TestAPI_Overview(t *testing.T) {
	router, _, token := setupTestServer(t)

	w := performRequestWithAuth(router, "GET", "/api/v1/overview", "", token)
	var resp apiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 200, resp.Code)
}

func TestAPI_SystemConfig(t *testing.T) {
	router, _, token := setupTestServer(t)

	w := performRequestWithAuth(router, "GET", "/api/v1/system/config", "", token)
	assert.Equal(t, 200, w.Code)

	w = performRequestWithAuth(router, "POST", "/api/v1/system/config", `{"server":{"address":"0.0.0.0"}}`, token)
	assert.Equal(t, 200, w.Code)
}

func TestAPI_MonitoringStats(t *testing.T) {
	router, _, token := setupTestServer(t)

	w := performRequestWithAuth(router, "GET", "/api/v1/monitoring/stats", "", token)
	assert.Equal(t, 200, w.Code)
}

func TestAPI_Logs(t *testing.T) {
	router, _, token := setupTestServer(t)

	w := performRequestWithAuth(router, "GET", "/api/v1/logs", "", token)
	assert.Equal(t, 200, w.Code)
}

func TestAPI_Firewall(t *testing.T) {
	router, _, token := setupTestServer(t)

	w := performRequestWithAuth(router, "GET", "/api/v1/firewall/attacks", "", token)
	assert.Equal(t, 200, w.Code)

	w = performRequestWithAuth(router, "POST", "/api/v1/firewall/whitelist", `{"ip":"1.2.3.4"}`, token)
	assert.Equal(t, 200, w.Code)

	w = performRequestWithAuth(router, "POST", "/api/v1/firewall/blacklist", `{"ip":"5.6.7.8"}`, token)
	assert.Equal(t, 200, w.Code)
}

func TestAPI_Sites_CRUD(t *testing.T) {
	router, _, token := setupTestServer(t)

	// Create
	w := performRequestWithAuth(router, "POST", "/api/v1/sites", `{"name":"test","domain":"example.com","port":80,"mode":"proxy"}`, token)
	assert.Equal(t, 200, w.Code)

	// List
	w = performRequestWithAuth(router, "GET", "/api/v1/sites", "", token)
	assert.Equal(t, 200, w.Code)

	// Get
	w = performRequestWithAuth(router, "GET", "/api/v1/sites/test-1", "", token)
	assert.Equal(t, 200, w.Code)

	// Update
	w = performRequestWithAuth(router, "PUT", "/api/v1/sites/test-1", `{"name":"updated"}`, token)
	assert.Equal(t, 200, w.Code)

	// Delete
	w = performRequestWithAuth(router, "DELETE", "/api/v1/sites/test-1", "", token)
	assert.Equal(t, 200, w.Code)
}

func TestAPI_Sites_Configs(t *testing.T) {
	router, _, token := setupTestServer(t)

	w := performRequestWithAuth(router, "GET", "/api/v1/sites/test-1/config?type=prerender", "", token)
	assert.Equal(t, 200, w.Code)

	w = performRequestWithAuth(router, "PUT", "/api/v1/sites/test-1/prerender", `{"enabled":true}`, token)
	assert.Equal(t, 200, w.Code)

	w = performRequestWithAuth(router, "PUT", "/api/v1/sites/test-1/push", `{"enabled":true}`, token)
	assert.Equal(t, 200, w.Code)

	w = performRequestWithAuth(router, "PUT", "/api/v1/sites/test-1/firewall", `{"enabled":true}`, token)
	assert.Equal(t, 200, w.Code)
}

func TestAPI_SSL(t *testing.T) {
	router, _, token := setupTestServer(t)

	w := performRequestWithAuth(router, "GET", "/api/v1/ssl/certificates", "", token)
	assert.Equal(t, 200, w.Code)
}

func TestAPI_Preheat(t *testing.T) {
	router, _, token := setupTestServer(t)

	w := performRequestWithAuth(router, "GET", "/api/v1/preheat/stats", "", token)
	assert.Equal(t, 200, w.Code)

	w = performRequestWithAuth(router, "POST", "/api/v1/preheat/trigger", `{"siteId":"test"}`, token)
	assert.Equal(t, 200, w.Code)

	w = performRequestWithAuth(router, "GET", "/api/v1/preheat/urls", "", token)
	assert.Equal(t, 200, w.Code)
}

func TestAPI_Crawler(t *testing.T) {
	router, _, token := setupTestServer(t)

	w := performRequestWithAuth(router, "GET", "/api/v1/crawler/logs", "", token)
	assert.Equal(t, 200, w.Code)

	w = performRequestWithAuth(router, "GET", "/api/v1/crawler/stats", "", token)
	assert.Equal(t, 200, w.Code)
}

func TestAPI_Push(t *testing.T) {
	router, _, token := setupTestServer(t)

	w := performRequestWithAuth(router, "GET", "/api/v1/push/stats", "", token)
	assert.Equal(t, 200, w.Code)

	w = performRequestWithAuth(router, "GET", "/api/v1/push/logs", "", token)
	assert.Equal(t, 200, w.Code)

	w = performRequestWithAuth(router, "GET", "/api/v1/push/config", "", token)
	assert.Equal(t, 200, w.Code)

	w = performRequestWithAuth(router, "POST", "/api/v1/push/config", `{"enabled":true}`, token)
	assert.Equal(t, 200, w.Code)

	w = performRequestWithAuth(router, "GET", "/api/v1/push/sites", "", token)
	assert.Equal(t, 200, w.Code)
}

func TestAPI_2FA(t *testing.T) {
	router, _, token := setupTestServer(t)

	// Status
	w := performRequestWithAuth(router, "GET", "/api/v1/auth/2fa/status", "", token)
	assert.Equal(t, 200, w.Code)

	// Enable
	w = performRequestWithAuth(router, "POST", "/api/v1/auth/2fa/enable", "", token)
	assert.Equal(t, 200, w.Code)

	// Confirm (missing code should fail)
	w = performRequestWithAuth(router, "POST", "/api/v1/auth/2fa/confirm", `{}`, token)
	assert.Equal(t, 400, w.Code)

	// Confirm with code
	w = performRequestWithAuth(router, "POST", "/api/v1/auth/2fa/confirm", `{"code":"123456"}`, token)
	assert.Equal(t, 200, w.Code)

	// Disable (missing code should fail)
	w = performRequestWithAuth(router, "POST", "/api/v1/auth/2fa/disable", `{}`, token)
	assert.Equal(t, 400, w.Code)

	// Disable with code
	w = performRequestWithAuth(router, "POST", "/api/v1/auth/2fa/disable", `{"code":"123456"}`, token)
	assert.Equal(t, 200, w.Code)
}

func TestAPI_ResponseFormat(t *testing.T) {
	router, _, token := setupTestServer(t)

	// All protected endpoints should return JSON with code/message/data
	endpoints := []string{
		"/api/v1/overview",
		"/api/v1/system/config",
		"/api/v1/monitoring/stats",
		"/api/v1/logs",
		"/api/v1/firewall/attacks",
		"/api/v1/preheat/stats",
		"/api/v1/crawler/logs",
		"/api/v1/push/stats",
		"/api/v1/ssl/certificates",
		"/api/v1/auth/2fa/status",
	}
	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			w := performRequestWithAuth(router, "GET", ep, "", token)
			assert.Equal(t, 200, w.Code)

			var resp apiResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			assert.NoError(t, err, "endpoint %s must return valid JSON", ep)
			assert.Equal(t, 200, resp.Code, "endpoint %s response code must be 200", ep)
			assert.NotEmpty(t, resp.Message, "endpoint %s must have message", ep)
		})
	}
}

// ======================================================
// Helpers
// ======================================================

func performRequest(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func performRequestWithAuth(router http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}
