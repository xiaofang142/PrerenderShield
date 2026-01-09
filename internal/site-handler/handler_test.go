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
	handler := NewHandler(nil, nil, nil, nil)

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
	visitLogManager := logging.NewVisitLogManager("localhost:6379") // 使用本地Redis URL
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
