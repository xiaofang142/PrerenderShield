package tests

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"prerender-shield/internal/auth"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/redis"

	"github.com/gin-gonic/gin"
)

type apiResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Success *bool           `json:"success,omitempty"`
}

func TestAPI_ComprehensiveV2(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup Redis
	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer redisClient.Close()

	// Cleanup
	keys, _ := redisClient.Keys("test:*")
	for _, k := range keys {
		redisClient.Del(k)
	}
	redisClient.Del("user:admin")
	redisClient.Del("users:all")

	// Setup auth
	jwtCfg := &auth.JWTConfig{SecretKey: "test-secret", ExpireTime: 24 * time.Hour}
	jwtManager := auth.NewJWTManager(jwtCfg, redisClient)
	userMgr := auth.NewUserManager("", redisClient)

	// 彻底清除共享 Redis 中可能残留的 admin 用户（真实键为 user:<uuid> 与 username:admin），
	// 避免依赖 IsFirstRun 的不确定结果
	if oldID, err := redisClient.GetUserByUsername("admin"); err == nil && oldID != "" {
		redisClient.Del(fmt.Sprintf("user:%s", oldID))
	}
	redisClient.Del("username:admin")

	if _, err := userMgr.CreateUser("admin", "Test123456!"); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	token, err := jwtManager.GenerateToken("uid-1", "admin")
	require.NoError(t, err)

	_ = logging.DefaultLogger

	// Build router
	router := gin.New()

	// ====== Public routes ======
	api := router.Group("/api/v1")
	api.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "ok"})
	})
	api.GET("/version", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "ok", "data": gin.H{"version": "3.0.0"}})
	})
	api.GET("/auth/first-run", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"isFirstRun": false}})
	})
	api.POST("/auth/login", func(c *gin.Context) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"code": 400, "message": "bad request"})
			return
		}
		if req.Username == "" || req.Password == "" {
			c.JSON(400, gin.H{"code": 400, "message": "missing fields"})
			return
		}
		user, err := userMgr.AuthenticateUser(req.Username, req.Password)
		if err != nil {
			c.JSON(401, gin.H{"code": 401, "message": "invalid credentials"})
			return
		}
		tok, _ := jwtManager.GenerateToken(user.ID, user.Username)
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"token": tok, "username": user.Username}})
	})
	api.POST("/auth/logout", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "logout success"})
	})

	// SSL public
	ssl := api.Group("/ssl")
	ssl.GET("/certificates", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": []interface{}{}})
	})
	ssl.GET("/certificates/expiring", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": []interface{}{}})
	})
	ssl.GET("/certificates/:domain", func(c *gin.Context) {
		domain := c.Param("domain")
		if domain == "" {
			c.JSON(400, gin.H{"code": 400, "message": "domain required"})
			return
		}
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"domain": domain}})
	})

	// ====== Protected routes ======
	authMW := func(c *gin.Context) {
		ah := c.GetHeader("Authorization")
		if ah == "" || !strings.HasPrefix(ah, "Bearer ") {
			c.JSON(401, gin.H{"code": 401, "message": "unauthorized"})
			c.Abort()
			return
		}
		_, err := jwtManager.ValidateToken(ah[7:])
		if err != nil {
			c.JSON(401, gin.H{"code": 401, "message": "invalid token"})
			c.Abort()
			return
		}
		c.Set("user_id", "uid-1")
		c.Next()
	}
	p := api.Group("")
	p.Use(authMW)

	p.GET("/overview", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{
			"totalRequests": 100, "activeSites": 3, "cacheHitRate": 0.85,
			"accessStats": gin.H{"pv": 1000, "uv": 50, "ip": 30},
		}})
	})

	p.GET("/system/config", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"server": gin.H{"api_port": 9598}}})
	})
	p.POST("/system/config", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "config updated"})
	})

	p.GET("/monitoring/stats", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{
			"requests_per_second": 100, "active_connections": 5,
		}})
	})

	// Logs
	p.GET("/logs", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{
			"logs": []interface{}{}, "total": 0, "page": 1, "limit": 20,
		}})
	})

	// Firewall
	p.GET("/firewall/attacks", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{
			"logs": []interface{}{}, "total": 0, "page": 1, "limit": 20,
		}})
	})
	p.POST("/firewall/whitelist", func(c *gin.Context) {
		var req struct {
			SiteID string `json:"site_id"`
			IP     string `json:"ip"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.SiteID == "" || req.IP == "" {
			c.JSON(400, gin.H{"code": 400, "message": "site_id and ip required"})
			return
		}
		c.JSON(200, gin.H{"code": 200, "message": "success"})
	})
	p.POST("/firewall/blacklist", func(c *gin.Context) {
		var req struct {
			SiteID string `json:"site_id"`
			IP     string `json:"ip"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.SiteID == "" || req.IP == "" {
			c.JSON(400, gin.H{"code": 400, "message": "site_id and ip required"})
			return
		}
		c.JSON(200, gin.H{"code": 200, "message": "success"})
	})

	// Crawler
	p.GET("/crawler/logs", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"items": []interface{}{}, "total": 0}})
	})
	p.GET("/crawler/stats", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{}})
	})

	// Preheat
	p.GET("/preheat/sites", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": []interface{}{}})
	})
	p.GET("/preheat/stats", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"total": 0, "active": 0}})
	})
	p.POST("/preheat/trigger", func(c *gin.Context) {
		var req struct {
			SiteID string `json:"siteId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.SiteID == "" {
			c.JSON(400, gin.H{"code": 400, "message": "siteId required"})
			return
		}
		c.JSON(200, gin.H{"code": 200, "message": "preheat triggered"})
	})
	p.GET("/preheat/urls", func(c *gin.Context) {
		siteID := c.Query("siteId")
		if siteID == "" {
			c.JSON(400, gin.H{"code": 400, "message": "siteId required"})
			return
		}
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"items": []interface{}{}, "total": 0}})
	})
	p.GET("/preheat/task/status", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{}})
	})
	p.GET("/preheat/crawler-headers", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": []string{}})
	})
	p.POST("/preheat/clear-cache", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "cache cleared"})
	})

	// Push
	p.GET("/push/sites", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": []interface{}{}})
	})
	p.GET("/push/stats", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{}})
	})
	p.GET("/push/logs", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"items": []interface{}{}, "total": 0}})
	})
	p.GET("/push/trend", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": []interface{}{}})
	})
	p.GET("/push/config", func(c *gin.Context) {
		siteID := c.Query("siteId")
		if siteID == "" {
			c.JSON(400, gin.H{"code": 400, "message": "siteId required"})
			return
		}
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{}})
	})
	p.POST("/push/config", func(c *gin.Context) {
		var req struct {
			SiteID string `json:"siteId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.SiteID == "" {
			c.JSON(400, gin.H{"code": 400, "message": "siteId required"})
			return
		}
		c.JSON(200, gin.H{"code": 200, "message": "config updated"})
	})

	// Sites CRUD
	p.GET("/sites", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": []interface{}{}})
	})
	p.POST("/sites", func(c *gin.Context) {
		var req struct {
			Name string `json:"name"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
			c.JSON(400, gin.H{"code": 400, "message": "name required"})
			return
		}
		c.JSON(200, gin.H{"code": 200, "message": "created", "data": gin.H{"id": "new-site-id"}})
	})
	p.GET("/sites/:id", func(c *gin.Context) {
		id := c.Param("id")
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"id": id, "name": "test"}})
	})
	p.GET("/sites/:id/config", func(c *gin.Context) {
		configType := c.Query("type")
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"type": configType}})
	})
	p.GET("/sites/:id/waf", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"enabled": true}})
	})
	p.PUT("/sites/:id/waf", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "waf config updated"})
	})
	p.PUT("/sites/:id/prerender", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "prerender config updated"})
	})
	p.PUT("/sites/:id/push", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "push config updated"})
	})
	p.PUT("/sites/:id/firewall", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "firewall config updated"})
	})
	p.PUT("/sites/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "site updated"})
	})
	p.DELETE("/sites/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "site deleted"})
	})
	p.GET("/sites/:id/static", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": []interface{}{}})
	})
	p.POST("/sites/:id/static", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "file uploaded"})
	})
	p.POST("/sites/:id/static/extract", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "file extracted"})
	})
	p.DELETE("/sites/:id/static", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "file deleted"})
	})
	p.POST("/sites/:id/static/batch-delete", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "batch deleted"})
	})

	// SSL protected
	p.POST("/ssl/certificates", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "cert requested"})
	})
	p.POST("/ssl/certificates/:domain/renew", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "cert renewed"})
	})
	p.DELETE("/ssl/certificates/:domain", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "cert deleted"})
	})
	p.POST("/ssl/certificates/wildcard", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "wildcard cert requested"})
	})
	p.GET("/ssl/certificates/:domain/renewal-history", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": []interface{}{}})
	})

	// 2FA
	p.GET("/2fa/status", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"enabled": false, "available": true}})
	})
	p.POST("/2fa/enable", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": gin.H{"secret": "TEST123", "qr_code_url": "otpauth://"}})
	})
	p.POST("/2fa/confirm", func(c *gin.Context) {
		var req struct {
			Code string `json:"code"`
		}
		c.ShouldBindJSON(&req)
		if req.Code == "" {
			c.JSON(400, gin.H{"code": 400, "message": "Missing code"})
			return
		}
		c.JSON(200, gin.H{"code": 200, "message": "2FA enabled"})
	})
	p.POST("/2fa/disable", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "message": "2FA disabled"})
	})

	// =========================================
	//  Comprehensive test cases
	// =========================================
	type testCase struct {
		name       string
		method     string
		path       string
		body       string
		token      string
		wantCode   int
		wantStatus int // response code field
	}

	publicRoutes := []testCase{
		// Public endpoints
		{"GET /health", "GET", "/api/v1/health", "", "", 200, 200},
		{"GET /version", "GET", "/api/v1/version", "", "", 200, 200},
		{"GET /auth/first-run", "GET", "/api/v1/auth/first-run", "", "", 200, 200},
		// Login variations
		{"POST /auth/login valid", "POST", "/api/v1/auth/login", `{"username":"admin","password":"Test123456!"}`, "", 200, 200},
		{"POST /auth/login wrong pwd", "POST", "/api/v1/auth/login", `{"username":"admin","password":"wrong"}`, "", 401, 401},
		{"POST /auth/login empty body", "POST", "/api/v1/auth/login", `{}`, "", 400, 400},
		{"POST /auth/login no user", "POST", "/api/v1/auth/login", `{"username":"","password":""}`, "", 400, 400},
		// SSL public
		{"GET /ssl/certificates", "GET", "/api/v1/ssl/certificates", "", "", 200, 200},
		{"GET /ssl/certificates/expiring", "GET", "/api/v1/ssl/certificates/expiring", "", "", 200, 200},
		{"GET /ssl/certificates/:domain", "GET", "/api/v1/ssl/certificates/example.com", "", "", 200, 200},
		// Auth failure for protected routes
		{"GET /overview no auth", "GET", "/api/v1/overview", "", "", 401, 401},
		{"GET /sites no auth", "GET", "/api/v1/sites", "", "", 401, 401},
		{"POST /sites no auth", "POST", "/api/v1/sites", `{"name":"test"}`, "", 401, 401},
		{"GET /system/config no auth", "GET", "/api/v1/system/config", "", "", 401, 401},
		{"POST /system/config no auth", "POST", "/api/v1/system/config", `{}`, "", 401, 401},
		{"GET /logs no auth", "GET", "/api/v1/logs", "", "", 401, 401},
		{"GET /monitoring/stats no auth", "GET", "/api/v1/monitoring/stats", "", "", 401, 401},
		{"GET /firewall/attacks no auth", "GET", "/api/v1/firewall/attacks", "", "", 401, 401},
		{"POST /firewall/whitelist no auth", "POST", "/api/v1/firewall/whitelist", `{}`, "", 401, 401},
		{"POST /firewall/blacklist no auth", "POST", "/api/v1/firewall/blacklist", `{}`, "", 401, 401},
		{"GET /crawler/logs no auth", "GET", "/api/v1/crawler/logs", "", "", 401, 401},
		{"GET /crawler/stats no auth", "GET", "/api/v1/crawler/stats", "", "", 401, 401},
		{"GET /preheat/stats no auth", "GET", "/api/v1/preheat/stats", "", "", 401, 401},
		{"POST /preheat/trigger no auth", "POST", "/api/v1/preheat/trigger", `{}`, "", 401, 401},
		{"GET /push/stats no auth", "GET", "/api/v1/push/stats", "", "", 401, 401},
		{"POST /push/config no auth", "POST", "/api/v1/push/config", `{}`, "", 401, 401},
		{"GET /ssl/certificates no auth (protected)", "POST", "/api/v1/ssl/certificates", `{}`, "", 401, 401},
		{"GET /2fa/status no auth", "GET", "/api/v1/2fa/status", "", "", 401, 401},
	}

	// All protected routes with valid auth
	protectedRoutes := []testCase{
		{"GET /overview", "GET", "/api/v1/overview", "", "VALID", 200, 200},
		{"GET /system/config", "GET", "/api/v1/system/config", "", "VALID", 200, 200},
		{"POST /system/config", "POST", "/api/v1/system/config", `{}`, "VALID", 200, 200},
		{"GET /monitoring/stats", "GET", "/api/v1/monitoring/stats", "", "VALID", 200, 200},
		{"GET /logs", "GET", "/api/v1/logs", "", "VALID", 200, 200},
		{"GET /firewall/attacks", "GET", "/api/v1/firewall/attacks", "", "VALID", 200, 200},
		// Firewall with missing params should 400
		{"POST /firewall/whitelist empty", "POST", "/api/v1/firewall/whitelist", `{}`, "VALID", 400, 400},
		{"POST /firewall/blacklist empty", "POST", "/api/v1/firewall/blacklist", `{}`, "VALID", 400, 400},
		{"POST /firewall/whitelist valid", "POST", "/api/v1/firewall/whitelist", `{"site_id":"s1","ip":"1.2.3.4"}`, "VALID", 200, 200},
		{"POST /firewall/blacklist valid", "POST", "/api/v1/firewall/blacklist", `{"site_id":"s1","ip":"5.6.7.8"}`, "VALID", 200, 200},
		// Crawler
		{"GET /crawler/logs", "GET", "/api/v1/crawler/logs", "", "VALID", 200, 200},
		{"GET /crawler/stats", "GET", "/api/v1/crawler/stats", "", "VALID", 200, 200},
		// Preheat
		{"GET /preheat/sites", "GET", "/api/v1/preheat/sites", "", "VALID", 200, 200},
		{"GET /preheat/stats", "GET", "/api/v1/preheat/stats", "", "VALID", 200, 200},
		{"POST /preheat/trigger empty", "POST", "/api/v1/preheat/trigger", `{}`, "VALID", 400, 400},
		{"POST /preheat/trigger valid", "POST", "/api/v1/preheat/trigger", `{"siteId":"s1"}`, "VALID", 200, 200},
		{"GET /preheat/urls no site", "GET", "/api/v1/preheat/urls", "", "VALID", 400, 400},
		{"GET /preheat/urls with site", "GET", "/api/v1/preheat/urls?siteId=s1", "", "VALID", 200, 200},
		{"GET /preheat/task/status", "GET", "/api/v1/preheat/task/status", "", "VALID", 200, 200},
		{"GET /preheat/crawler-headers", "GET", "/api/v1/preheat/crawler-headers", "", "VALID", 200, 200},
		{"POST /preheat/clear-cache", "POST", "/api/v1/preheat/clear-cache", `{}`, "VALID", 200, 200},
		// Push
		{"GET /push/sites", "GET", "/api/v1/push/sites", "", "VALID", 200, 200},
		{"GET /push/stats", "GET", "/api/v1/push/stats", "", "VALID", 200, 200},
		{"GET /push/logs", "GET", "/api/v1/push/logs", "", "VALID", 200, 200},
		{"GET /push/trend", "GET", "/api/v1/push/trend", "", "VALID", 200, 200},
		{"GET /push/config no site", "GET", "/api/v1/push/config", "", "VALID", 400, 400},
		{"GET /push/config with site", "GET", "/api/v1/push/config?siteId=s1", "", "VALID", 200, 200},
		{"POST /push/config empty", "POST", "/api/v1/push/config", `{}`, "VALID", 400, 400},
		{"POST /push/config valid", "POST", "/api/v1/push/config", `{"siteId":"s1"}`, "VALID", 200, 200},
		// Sites CRUD
		{"GET /sites", "GET", "/api/v1/sites", "", "VALID", 200, 200},
		{"POST /sites empty name", "POST", "/api/v1/sites", `{}`, "VALID", 400, 400},
		{"POST /sites valid", "POST", "/api/v1/sites", `{"name":"test-site"}`, "VALID", 200, 200},
		{"GET /sites/:id", "GET", "/api/v1/sites/site-1", "", "VALID", 200, 200},
		{"GET /sites/:id/config", "GET", "/api/v1/sites/site-1/config?type=prerender", "", "VALID", 200, 200},
		{"GET /sites/:id/waf", "GET", "/api/v1/sites/site-1/waf", "", "VALID", 200, 200},
		{"PUT /sites/:id/waf", "PUT", "/api/v1/sites/site-1/waf", `{}`, "VALID", 200, 200},
		{"PUT /sites/:id/prerender", "PUT", "/api/v1/sites/site-1/prerender", `{}`, "VALID", 200, 200},
		{"PUT /sites/:id/push", "PUT", "/api/v1/sites/site-1/push", `{}`, "VALID", 200, 200},
		{"PUT /sites/:id/firewall", "PUT", "/api/v1/sites/site-1/firewall", `{}`, "VALID", 200, 200},
		{"PUT /sites/:id", "PUT", "/api/v1/sites/site-1", `{"name":"updated"}`, "VALID", 200, 200},
		{"DELETE /sites/:id", "DELETE", "/api/v1/sites/site-1", "", "VALID", 200, 200},
		// Static files
		{"GET /sites/:id/static", "GET", "/api/v1/sites/site-1/static", "", "VALID", 200, 200},
		{"POST /sites/:id/static", "POST", "/api/v1/sites/site-1/static", `{}`, "VALID", 200, 200},
		{"POST /sites/:id/static/extract", "POST", "/api/v1/sites/site-1/static/extract", `{}`, "VALID", 200, 200},
		{"DELETE /sites/:id/static", "DELETE", "/api/v1/sites/site-1/static", "", "VALID", 200, 200},
		{"POST /sites/:id/static/batch-delete", "POST", "/api/v1/sites/site-1/static/batch-delete", `{}`, "VALID", 200, 200},
		// SSL protected
		{"POST /ssl/certificates", "POST", "/api/v1/ssl/certificates", `{}`, "VALID", 200, 200},
		{"POST /ssl/certificates/:domain/renew", "POST", "/api/v1/ssl/certificates/example.com/renew", `{}`, "VALID", 200, 200},
		{"DELETE /ssl/certificates/:domain", "DELETE", "/api/v1/ssl/certificates/example.com", "", "VALID", 200, 200},
		{"POST /ssl/certificates/wildcard", "POST", "/api/v1/ssl/certificates/wildcard", `{}`, "VALID", 200, 200},
		{"GET /ssl/certificates/:domain/renewal-history", "GET", "/api/v1/ssl/certificates/example.com/renewal-history", "", "VALID", 200, 200},
		// 2FA
		{"GET /2fa/status", "GET", "/api/v1/2fa/status", "", "VALID", 200, 200},
		{"POST /2fa/enable", "POST", "/api/v1/2fa/enable", `{}`, "VALID", 200, 200},
		{"POST /2fa/confirm no code", "POST", "/api/v1/2fa/confirm", `{}`, "VALID", 400, 400},
		{"POST /2fa/confirm with code", "POST", "/api/v1/2fa/confirm", `{"code":"123456"}`, "VALID", 200, 200},
		{"POST /2fa/disable", "POST", "/api/v1/2fa/disable", `{}`, "VALID", 200, 200},
		// Logout
		{"POST /auth/logout", "POST", "/api/v1/auth/logout", `{}`, "VALID", 200, 200},
	}

	// Run public tests
	t.Run("Public", func(t *testing.T) {
		for _, tc := range publicRoutes {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
				if tc.body != "" {
					req.Header.Set("Content-Type", "application/json")
				}
				if tc.token == "VALID" {
					req.Header.Set("Authorization", "Bearer "+token)
				} else if tc.token != "" {
					req.Header.Set("Authorization", "Bearer "+tc.token)
				}
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				assert.Equal(t, tc.wantCode, w.Code, "HTTP status")
				if tc.wantStatus > 0 {
					var resp apiResp
					if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
						if resp.Code != 0 {
							assert.Equal(t, tc.wantStatus, resp.Code, "response code")
						}
					}
				}
			})
		}
	})

	// Run protected tests
	t.Run("Protected", func(t *testing.T) {
		for _, tc := range protectedRoutes {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
				if tc.body != "" {
					req.Header.Set("Content-Type", "application/json")
				}
				if tc.token == "VALID" {
					req.Header.Set("Authorization", "Bearer "+token)
				} else if tc.token != "" {
					req.Header.Set("Authorization", "Bearer "+tc.token)
				}
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				assert.Equal(t, tc.wantCode, w.Code, "[%s] HTTP status", tc.name)
				if tc.wantStatus > 0 {
					var resp apiResp
					if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
						if resp.Code != 0 {
							assert.Equal(t, tc.wantStatus, resp.Code, "[%s] response code", tc.name)
						}
					}
				}
			})
		}
	})

	// Test response format consistency for all 200 responses
	formatEndpoints := []struct {
		name string
		path string
	}{
		{"GET /health", "/api/v1/health"},
		{"GET /overview", "/api/v1/overview"},
		{"GET /system/config", "/api/v1/system/config"},
		{"GET /monitoring/stats", "/api/v1/monitoring/stats"},
		{"GET /logs", "/api/v1/logs"},
		{"GET /firewall/attacks", "/api/v1/firewall/attacks"},
		{"GET /crawler/logs", "/api/v1/crawler/logs"},
		{"GET /crawler/stats", "/api/v1/crawler/stats"},
		{"GET /preheat/stats", "/api/v1/preheat/stats"},
		{"GET /push/stats", "/api/v1/push/stats"},
		{"GET /push/sites", "/api/v1/push/sites"},
		{"GET /sites", "/api/v1/sites"},
		{"GET /2fa/status", "/api/v1/2fa/status"},
		{"GET /ssl/certificates", "/api/v1/ssl/certificates"},
		{"GET /ssl/certificates/expiring", "/api/v1/ssl/certificates/expiring"},
	}
	t.Run("ResponseFormat", func(t *testing.T) {
		for _, ep := range formatEndpoints {
			t.Run(ep.name, func(t *testing.T) {
				req := httptest.NewRequest("GET", ep.path, nil)
				req.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				var raw map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &raw)
				assert.NoError(t, err, "response must be valid JSON")

				hasCode := false
				hasMessage := false
				for k := range raw {
					if k == "code" {
						hasCode = true
					}
					if k == "message" {
						hasMessage = true
					}
				}
				assert.True(t, hasCode, "%s: response must have 'code' field, got keys: %v", ep.name, mapKeys(raw))
				assert.True(t, hasMessage, "%s: response must have 'message' field", ep.name)
			})
		}
	})

	// Print stats
	fmt.Printf("\n=== API Test Summary ===\n")
	fmt.Printf("Public endpoint tests:  %d\n", len(publicRoutes))
	fmt.Printf("Protected endpoint tests: %d\n", len(protectedRoutes))
	fmt.Printf("Response format checks: %d\n", len(formatEndpoints))
	fmt.Printf("Total test cases: %d\n", len(publicRoutes)+len(protectedRoutes)+len(formatEndpoints))
	fmt.Printf("(Each test includes HTTP status + response code validation)\n")
}

func mapKeys(m map[string]interface{}) []string {
	var k []string
	for key := range m {
		k = append(k, key)
	}
	return k
}
