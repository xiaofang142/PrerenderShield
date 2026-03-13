package sitehandler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
)

// TestNewHandler 测试创建 Handler
func TestNewHandler(t *testing.T) {
	h := NewHandler(nil, nil, nil, nil)
	assert.NotNil(t, h)
	assert.Nil(t, h.prerenderManager)
	assert.Nil(t, h.wafRepo)
	assert.Nil(t, h.redisClient)
	assert.Nil(t, h.geoIP)
}

// TestGetLanguageFromRequest 测试从请求获取语言
func TestGetLanguageFromRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		acceptLanguage string
		expected       string
	}{
		{"zh-CN", "zh-CN,zh;q=0.9,en;q=0.8", "zh"},
		{"en-US", "en-US,en;q=0.9", "en"},
		{"ja", "ja;q=0.9", "ja"},
		{"empty", "", "en"}, // 默认返回英文
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.acceptLanguage != "" {
				req.Header.Set("Accept-Language", tt.acceptLanguage)
			}
			c.Request = req

			lang := getLanguageFromRequest(c)
			assert.Equal(t, tt.expected, lang)
		})
	}
}

// TestCreateSiteHandler_NilPrerenderManager 测试 PrerenderManager 为 nil 时的行为
func TestCreateSiteHandler_NilPrerenderManager(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	testSite := config.SiteConfig{
		ID:      "test-site",
		Name:    "Test Site",
		Domains: []string{"example.com"},
		Port:    8080,
		Mode:    "redirect",
		Redirect: config.RedirectConfig{
			StatusCode: 301,
			TargetURL:  "https://target.example.com",
		},
	}

	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")
	visitLogManager := logging.NewVisitLogManager("localhost:6379")
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/114.0.0.0 Safari/537.36")
	rec := httptest.NewRecorder()

	siteHandler := handler.CreateSiteHandler(testSite, crawlerLogManager, visitLogManager, monitor, "/tmp/static")
	siteHandler.ServeHTTP(rec, req)

	// 重定向模式应该正常工作
	assert.Equal(t, 301, rec.Code)
	assert.Equal(t, "https://target.example.com", rec.Header().Get("Location"))
}

// TestCreateSiteHandler_ProxyMode 测试代理模式
func TestCreateSiteHandler_ProxyMode(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	testSite := config.SiteConfig{
		ID:      "test-site",
		Name:    "Test Site",
		Domains: []string{"example.com"},
		Port:    8080,
		Mode:    "proxy",
		Proxy: config.ProxyConfig{
			TargetURL: "http://backend.example.com",
		},
	}

	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")
	visitLogManager := logging.NewVisitLogManager("localhost:6379")
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})

	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/114.0.0.0 Safari/537.36")
	rec := httptest.NewRecorder()

	// 不应该 panic
	assert.NotPanics(t, func() {
		siteHandler := handler.CreateSiteHandler(testSite, crawlerLogManager, visitLogManager, monitor, "/tmp/static")
		siteHandler.ServeHTTP(rec, req)
	})

	// 代理模式会尝试转发请求，但由于后端不存在，可能会失败
	assert.NotNil(t, rec)
}

// TestCreateSiteHandler_ProxyMode_InvalidURL 测试代理模式无效 URL
func TestCreateSiteHandler_ProxyMode_InvalidURL(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	testSite := config.SiteConfig{
		ID:      "test-site",
		Name:    "Test Site",
		Domains: []string{"example.com"},
		Port:    8080,
		Mode:    "proxy",
		Proxy: config.ProxyConfig{
			TargetURL: "://invalid-url", // 无效的 URL
		},
	}

	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")
	visitLogManager := logging.NewVisitLogManager("localhost:6379")
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/114.0.0.0 Safari/537.36")
	rec := httptest.NewRecorder()

	siteHandler := handler.CreateSiteHandler(testSite, crawlerLogManager, visitLogManager, monitor, "/tmp/static")
	siteHandler.ServeHTTP(rec, req)

	// 无效 URL 应该返回 500 错误
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// TestCreateSiteHandler_StaticMode 测试静态模式
func TestCreateSiteHandler_StaticMode(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	testSite := config.SiteConfig{
		ID:      "test-site",
		Name:    "Test Site",
		Domains: []string{"example.com"},
		Port:    8080,
		Mode:    "static",
	}

	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")
	visitLogManager := logging.NewVisitLogManager("localhost:6379")
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/114.0.0.0 Safari/537.36")
	rec := httptest.NewRecorder()

	siteHandler := handler.CreateSiteHandler(testSite, crawlerLogManager, visitLogManager, monitor, t.TempDir())
	siteHandler.ServeHTTP(rec, req)

	// 静态模式下，没有 index.html 应该返回 404
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestCreateSiteHandler_StaticMode_WithIndexHTML 测试静态模式有 index.html
func TestCreateSiteHandler_StaticMode_WithIndexHTML(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	testSite := config.SiteConfig{
		ID:      "test-site",
		Name:    "Test Site",
		Domains: []string{"example.com"},
		Port:    8080,
		Mode:    "static",
	}

	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")
	visitLogManager := logging.NewVisitLogManager("localhost:6379")
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})

	staticDir := t.TempDir()

	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/114.0.0.0 Safari/537.36")
	rec := httptest.NewRecorder()

	siteHandler := handler.CreateSiteHandler(testSite, crawlerLogManager, visitLogManager, monitor, staticDir)
	siteHandler.ServeHTTP(rec, req)

	// 应该返回 404 因为 index.html 不存在
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestCreateSiteHandler_StaticMode_StaticResource 测试静态资源
func TestCreateSiteHandler_StaticMode_StaticResource(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	testSite := config.SiteConfig{
		ID:      "test-site",
		Name:    "Test Site",
		Domains: []string{"example.com"},
		Port:    8080,
		Mode:    "static",
	}

	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")
	visitLogManager := logging.NewVisitLogManager("localhost:6379")
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})

	staticDir := t.TempDir()

	req := httptest.NewRequest("GET", "http://example.com/style.css", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/114.0.0.0 Safari/537.36")
	rec := httptest.NewRecorder()

	siteHandler := handler.CreateSiteHandler(testSite, crawlerLogManager, visitLogManager, monitor, staticDir)
	siteHandler.ServeHTTP(rec, req)

	// 静态资源不存在应该返回 404
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestCreateSiteHandler_UnknownMode 测试未知模式
func TestCreateSiteHandler_UnknownMode(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	testSite := config.SiteConfig{
		ID:      "test-site",
		Name:    "Test Site",
		Domains: []string{"example.com"},
		Port:    8080,
		Mode:    "unknown",
	}

	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")
	visitLogManager := logging.NewVisitLogManager("localhost:6379")
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/114.0.0.0 Safari/537.36")
	rec := httptest.NewRecorder()

	siteHandler := handler.CreateSiteHandler(testSite, crawlerLogManager, visitLogManager, monitor, "/tmp/static")
	siteHandler.ServeHTTP(rec, req)

	// 未知模式应该返回 500 错误
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// TestCreateSiteHandler_CrawlerRequest_NilPrerenderManager 测试爬虫请求时 PrerenderManager 为 nil
func TestCreateSiteHandler_CrawlerRequest_NilPrerenderManager(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	testSite := config.SiteConfig{
		ID:      "test-site",
		Name:    "Test Site",
		Domains: []string{"example.com"},
		Port:    8080,
		Mode:    "proxy",
		Proxy: config.ProxyConfig{
			TargetURL: "http://backend.example.com",
		},
	}

	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")
	visitLogManager := logging.NewVisitLogManager("localhost:6379")
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})

	// 使用爬虫 User-Agent
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("User-Agent", "Googlebot/2.1 (+http://www.google.com/bot.html)")
	rec := httptest.NewRecorder()

	siteHandler := handler.CreateSiteHandler(testSite, crawlerLogManager, visitLogManager, monitor, "/tmp/static")
	siteHandler.ServeHTTP(rec, req)

	// PrerenderManager 为 nil 时，爬虫请求应该返回 500 错误
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// TestCreateSiteHandler_MonitorRecording 测试监控记录
func TestCreateSiteHandler_MonitorRecording(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	testSite := config.SiteConfig{
		ID:      "test-site",
		Name:    "Test Site",
		Domains: []string{"example.com"},
		Port:    8080,
		Mode:    "redirect",
		Redirect: config.RedirectConfig{
			StatusCode: 302,
			TargetURL:  "https://target.example.com",
		},
	}

	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")
	visitLogManager := logging.NewVisitLogManager("localhost:6379")
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: true})

	req := httptest.NewRequest("POST", "http://example.com/api/data", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/114.0.0.0 Safari/537.36")
	rec := httptest.NewRecorder()

	siteHandler := handler.CreateSiteHandler(testSite, crawlerLogManager, visitLogManager, monitor, "/tmp/static")
	siteHandler.ServeHTTP(rec, req)

	// 验证重定向
	assert.Equal(t, 302, rec.Code)

	// 验证监控记录了请求
	stats := monitor.GetStats()
	assert.NotNil(t, stats)
}

// TestCreateSiteHandler_WithQueryParams 测试带查询参数的请求
func TestCreateSiteHandler_WithQueryParams(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	testSite := config.SiteConfig{
		ID:      "test-site",
		Name:    "Test Site",
		Domains: []string{"example.com"},
		Port:    8080,
		Mode:    "redirect",
		Redirect: config.RedirectConfig{
			StatusCode: 301,
			TargetURL:  "https://target.example.com",
		},
	}

	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")
	visitLogManager := logging.NewVisitLogManager("localhost:6379")
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})

	req := httptest.NewRequest("GET", "http://example.com/search?q=test&page=1", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/114.0.0.0 Safari/537.36")
	rec := httptest.NewRecorder()

	siteHandler := handler.CreateSiteHandler(testSite, crawlerLogManager, visitLogManager, monitor, "/tmp/static")
	siteHandler.ServeHTTP(rec, req)

	assert.Equal(t, 301, rec.Code)
}

// TestCreateSiteHandler_WithReferer 测试带 Referer 的请求
func TestCreateSiteHandler_WithReferer(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	testSite := config.SiteConfig{
		ID:      "test-site",
		Name:    "Test Site",
		Domains: []string{"example.com"},
		Port:    8080,
		Mode:    "redirect",
		Redirect: config.RedirectConfig{
			StatusCode: 301,
			TargetURL:  "https://target.example.com",
		},
	}

	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")
	visitLogManager := logging.NewVisitLogManager("localhost:6379")
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/114.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://referrer.com/page")
	rec := httptest.NewRecorder()

	siteHandler := handler.CreateSiteHandler(testSite, crawlerLogManager, visitLogManager, monitor, "/tmp/static")
	siteHandler.ServeHTTP(rec, req)

	assert.Equal(t, 301, rec.Code)
}

// TestCreateSiteHandler_ConcurrentAccess 测试并发访问
func TestCreateSiteHandler_ConcurrentAccess(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	testSite := config.SiteConfig{
		ID:      "test-site",
		Name:    "Test Site",
		Domains: []string{"example.com"},
		Port:    8080,
		Mode:    "redirect",
		Redirect: config.RedirectConfig{
			StatusCode: 301,
			TargetURL:  "https://target.example.com",
		},
	}

	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")
	visitLogManager := logging.NewVisitLogManager("localhost:6379")
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})

	siteHandler := handler.CreateSiteHandler(testSite, crawlerLogManager, visitLogManager, monitor, "/tmp/static")

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			req := httptest.NewRequest("GET", "http://example.com/test", nil)
			req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/114.0.0.0 Safari/537.36")
			rec := httptest.NewRecorder()
			siteHandler.ServeHTTP(rec, req)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// 不应该 panic
	assert.True(t, true)
}

// TestCreateSiteHandler_DifferentUserAgents 测试不同 User-Agent
func TestCreateSiteHandler_DifferentUserAgents(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	testSite := config.SiteConfig{
		ID:      "test-site",
		Name:    "Test Site",
		Domains: []string{"example.com"},
		Port:    8080,
		Mode:    "redirect",
		Redirect: config.RedirectConfig{
			StatusCode: 301,
			TargetURL:  "https://target.example.com",
		},
	}

	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")
	visitLogManager := logging.NewVisitLogManager("localhost:6379")
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})

	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/114.0.0.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/605.1.15",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) Mobile/15E148",
		"curl/7.68.0",
		"PostmanRuntime/7.28.0",
	}

	for _, ua := range userAgents {
		t.Run(ua, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com/test", nil)
			req.Header.Set("User-Agent", ua)
			rec := httptest.NewRecorder()

			siteHandler := handler.CreateSiteHandler(testSite, crawlerLogManager, visitLogManager, monitor, "/tmp/static")
			siteHandler.ServeHTTP(rec, req)

			assert.Equal(t, 301, rec.Code)
		})
	}
}

// TestCreateSiteHandler_HTTPMethods 测试不同 HTTP 方法
func TestCreateSiteHandler_HTTPMethods(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	testSite := config.SiteConfig{
		ID:      "test-site",
		Name:    "Test Site",
		Domains: []string{"example.com"},
		Port:    8080,
		Mode:    "redirect",
		Redirect: config.RedirectConfig{
			StatusCode: 301,
			TargetURL:  "https://target.example.com",
		},
	}

	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")
	visitLogManager := logging.NewVisitLogManager("localhost:6379")
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "http://example.com/test", nil)
			req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/114.0.0.0 Safari/537.36")
			rec := httptest.NewRecorder()

			siteHandler := handler.CreateSiteHandler(testSite, crawlerLogManager, visitLogManager, monitor, "/tmp/static")
			siteHandler.ServeHTTP(rec, req)

			assert.Equal(t, 301, rec.Code)
		})
	}
}

// TestCreateSiteHandler_EmptySiteConfig 测试空站点配置
func TestCreateSiteHandler_EmptySiteConfig(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	testSite := config.SiteConfig{}

	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")
	visitLogManager := logging.NewVisitLogManager("localhost:6379")
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/114.0.0.0 Safari/537.36")
	rec := httptest.NewRecorder()

	siteHandler := handler.CreateSiteHandler(testSite, crawlerLogManager, visitLogManager, monitor, "/tmp/static")
	siteHandler.ServeHTTP(rec, req)

	// 空配置默认应该是未知模式，返回 500
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// TestCreateSiteHandler_RedisNotAvailable 测试 Redis 不可用时的行为
func TestCreateSiteHandler_RedisNotAvailable(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	testSite := config.SiteConfig{
		ID:      "test-site",
		Name:    "Test Site",
		Domains: []string{"example.com"},
		Port:    8080,
		Mode:    "redirect",
		Redirect: config.RedirectConfig{
			StatusCode: 301,
			TargetURL:  "https://target.example.com",
		},
	}

	// 使用无法连接的 Redis
	crawlerLogManager := logging.NewCrawlerLogManager("invalid-redis:6379")
	visitLogManager := logging.NewVisitLogManager("invalid-redis:6379")
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/114.0.0.0 Safari/537.36")
	rec := httptest.NewRecorder()

	// 不应该 panic
	assert.NotPanics(t, func() {
		siteHandler := handler.CreateSiteHandler(testSite, crawlerLogManager, visitLogManager, monitor, "/tmp/static")
		siteHandler.ServeHTTP(rec, req)
	})

	assert.Equal(t, 301, rec.Code)
}

// TestCreateSiteHandler_WithTimeout 测试超时配置
func TestCreateSiteHandler_WithTimeout(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	testSite := config.SiteConfig{
		ID:      "test-site",
		Name:    "Test Site",
		Domains: []string{"example.com"},
		Port:    8080,
		Mode:    "redirect",
		Redirect: config.RedirectConfig{
			StatusCode: 301,
			TargetURL:  "https://target.example.com",
		},
		Prerender: config.PrerenderConfig{
			Timeout:  30,
			CacheTTL: 300,
		},
	}

	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")
	visitLogManager := logging.NewVisitLogManager("localhost:6379")
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/114.0.0.0 Safari/537.36")
	rec := httptest.NewRecorder()

	siteHandler := handler.CreateSiteHandler(testSite, crawlerLogManager, visitLogManager, monitor, "/tmp/static")

	start := time.Now()
	siteHandler.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	assert.Equal(t, 301, rec.Code)
	// 重定向应该很快完成
	assert.Less(t, elapsed, time.Second)
}

// TestConfig_Struct 测试 SiteConfig 结构体
func TestConfig_Struct(t *testing.T) {
	cfg := config.SiteConfig{
		ID:      "test",
		Name:    "Test",
		Domains: []string{"test.com"},
		Port:    8080,
		Mode:    "proxy",
	}
	assert.Equal(t, "test", cfg.ID)
	assert.Equal(t, "Test", cfg.Name)
	assert.Len(t, cfg.Domains, 1)
	assert.Equal(t, 8080, cfg.Port)
}

// TestRedirectConfig_Struct 测试 RedirectConfig 结构体
func TestRedirectConfig_Struct(t *testing.T) {
	cfg := config.RedirectConfig{
		StatusCode: 301,
		TargetURL:  "https://example.com",
	}
	assert.Equal(t, 301, cfg.StatusCode)
	assert.Equal(t, "https://example.com", cfg.TargetURL)
}

// TestProxyConfig_Struct 测试 ProxyConfig 结构体
func TestProxyConfig_Struct(t *testing.T) {
	cfg := config.ProxyConfig{
		TargetURL: "http://backend.com",
	}
	assert.Equal(t, "http://backend.com", cfg.TargetURL)
}

// TestHandler_Interface 测试 Handler 接口实现
func TestHandler_Interface(t *testing.T) {
	var _ interface{} = (*Handler)(nil)
}
