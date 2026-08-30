package sitehandler

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
)

func TestCreateSiteHandler_RedirectMode(t *testing.T) {
	// 创建sitehandler，传递所有必要的参数
	handler := NewHandler(nil, nil, nil, nil, nil, nil, nil)

	// 创建测试站点配置
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

	// 创建实际的监控和日志管理器
	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379") // 使用本地Redis URL
	visitLogManager := logging.NewVisitLogManager("localhost:6379")     // 使用本地Redis URL
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false}) // 禁用监控，避免启动不必要的服务

	// 创建HTTP请求和响应记录器
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36")
	rec := httptest.NewRecorder()

	// 创建站点处理器，传递所有必要的参数
	siteHandler := handler.CreateSiteHandler(testSite, crawlerLogManager, visitLogManager, monitor, "/tmp/static")

	// 处理请求
	siteHandler.ServeHTTP(rec, req)

	// 验证响应状态码和重定向位置
	assert.Equal(t, 301, rec.Code)
	assert.Equal(t, "https://target.example.com", rec.Header().Get("Location"))
}

func TestCreateSiteHandler_IndexNowKeyFile(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil, nil, nil, nil)

	testSite := config.SiteConfig{
		ID:      "test-keyfile-site",
		Name:    "Keyfile Site",
		Domains: []string{"example.com"},
		Port:    8081,
		Mode:    "redirect",
		Redirect: config.RedirectConfig{
			StatusCode: 301,
			TargetURL:  "https://target.example.com",
		},
		Prerender: config.PrerenderConfig{
			Push: config.PushConfig{
				IndexNowKey: "abc123key",
			},
		},
		Firewall: config.FirewallConfig{
			// Enabled: false 时不会创建 WAF 引擎，但仍会注册 WafMiddleware；
			// keyfile 拦截注册在 WafMiddleware 之前，验证顺序正确性由此用例保障
			Enabled: false,
		},
	}

	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")
	visitLogManager := logging.NewVisitLogManager("localhost:6379")
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})
	siteHandler := handler.CreateSiteHandler(testSite, crawlerLogManager, visitLogManager, monitor, "/tmp/static")

	// 精确命中 /{key}.txt → 200 text/plain body=key
	req := httptest.NewRequest("GET", "http://example.com/abc123key.txt", nil)
	rec := httptest.NewRecorder()
	siteHandler.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)
	assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, "abc123key", rec.Body.String())

	// 相邻路径不误命中（key 变体/后缀差异必须落到正常站点逻辑 → redirect）
	req2 := httptest.NewRequest("GET", "http://example.com/abc123key.txt.extra", nil)
	rec2 := httptest.NewRecorder()
	siteHandler.ServeHTTP(rec2, req2)
	assert.Equal(t, 301, rec2.Code)

	// 404.txt 不误命中
	req3 := httptest.NewRequest("GET", "http://example.com/other.txt", nil)
	rec3 := httptest.NewRecorder()
	siteHandler.ServeHTTP(rec3, req3)
	assert.Equal(t, 301, rec3.Code)
}
