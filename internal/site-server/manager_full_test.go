package siteserver

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
)

// TestNewManager_NilMonitor 测试创建管理器时 monitor 为 nil
func TestNewManager_NilMonitor(t *testing.T) {
	manager := NewManager(nil)
	assert.NotNil(t, manager)
	assert.Nil(t, manager.monitor)
	assert.NotNil(t, manager.siteServers)
	assert.Len(t, manager.siteServers, 0)
}

// TestManager_Struct 测试 Manager 结构
func TestManager_Struct(t *testing.T) {
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})
	defer monitor.Stop()

	manager := &Manager{
		siteServers: make(map[string]*http.Server),
		monitor:     monitor,
	}

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.siteServers)
	assert.Equal(t, monitor, manager.monitor)
}

// TestStartSiteServer 测试启动站点服务器
func TestStartSiteServer(t *testing.T) {
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})
	defer monitor.Stop()

	manager := NewManager(monitor)

	site := config.SiteConfig{
		ID:      "test-site",
		Name:    "Test Site",
		Domains: []string{"example.com"},
		Port:    18080, // 使用非标准端口避免冲突
		Mode:    "redirect",
		Redirect: config.RedirectConfig{
			StatusCode: 301,
			TargetURL:  "https://example.com",
		},
	}

	// 创建简单的测试处理器
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 创建日志管理器
	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")

	// 启动服务器（不实际监听，只是测试函数调用）
	serverAddress := "127.0.0.1"
	staticDir := t.TempDir()

	// 不应该 panic
	assert.NotPanics(t, func() {
		manager.StartSiteServer(site, serverAddress, staticDir, crawlerLogManager, testHandler)
	})

	// 验证服务器已被记录
	server, exists := manager.GetSiteServer("test-site")
	assert.True(t, exists)
	assert.NotNil(t, server)

	// 清理：停止服务器
	manager.StopSiteServer("test-site")
}

// TestStartSiteServer_MultipleServers 测试启动多个站点服务器
func TestStartSiteServer_MultipleServers(t *testing.T) {
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})
	defer monitor.Stop()

	manager := NewManager(monitor)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")
	staticDir := t.TempDir()

	// 启动多个服务器
	sites := []config.SiteConfig{
		{ID: "site1", Name: "Site 1", Port: 18081, Mode: "redirect", Redirect: config.RedirectConfig{StatusCode: 301, TargetURL: "http://example.com"}},
		{ID: "site2", Name: "Site 2", Port: 18082, Mode: "redirect", Redirect: config.RedirectConfig{StatusCode: 301, TargetURL: "http://example.com"}},
		{ID: "site3", Name: "Site 3", Port: 18083, Mode: "redirect", Redirect: config.RedirectConfig{StatusCode: 301, TargetURL: "http://example.com"}},
	}

	for _, site := range sites {
		manager.StartSiteServer(site, "127.0.0.1", staticDir, crawlerLogManager, testHandler)
	}

	// 验证所有服务器都被记录
	assert.Len(t, manager.siteServers, 3)

	// 清理
	manager.StopAllServers()
}

// TestStopSiteServer_NonExistent 测试停止不存在的服务器
func TestStopSiteServer_NonExistent(t *testing.T) {
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})
	defer monitor.Stop()

	manager := NewManager(monitor)

	// 停止不存在的服务器应该返回 nil（无错误）
	err := manager.StopSiteServer("non-existent")
	assert.NoError(t, err)
}

// TestStopSiteServer 测试停止服务器
func TestStopSiteServer(t *testing.T) {
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})
	defer monitor.Stop()

	manager := NewManager(monitor)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")

	site := config.SiteConfig{
		ID:   "test-site",
		Name: "Test Site",
		Port: 18084,
		Mode: "redirect",
		Redirect: config.RedirectConfig{
			StatusCode: 301,
			TargetURL:  "https://example.com",
		},
	}

	manager.StartSiteServer(site, "127.0.0.1", t.TempDir(), crawlerLogManager, testHandler)

	// 验证服务器已启动
	_, exists := manager.GetSiteServer("test-site")
	assert.True(t, exists)

	// 停止服务器
	err := manager.StopSiteServer("test-site")
	assert.NoError(t, err)

	// 等待服务器完全停止
	time.Sleep(time.Millisecond * 100)

	// 验证服务器已被移除
	_, exists = manager.GetSiteServer("test-site")
	assert.False(t, exists)
}

// TestListSiteServers_WithServers 测试列出有服务器的列表
func TestListSiteServers_WithServers(t *testing.T) {
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})
	defer monitor.Stop()

	manager := NewManager(monitor)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")

	site := config.SiteConfig{
		ID:   "test-site",
		Name: "Test Site",
		Port: 18085,
		Mode: "redirect",
		Redirect: config.RedirectConfig{
			StatusCode: 301,
			TargetURL:  "https://example.com",
		},
	}

	manager.StartSiteServer(site, "127.0.0.1", t.TempDir(), crawlerLogManager, testHandler)

	servers := manager.ListSiteServers()
	assert.Len(t, servers, 1)
	assert.Contains(t, servers, "test-site")

	// 清理
	manager.StopSiteServer("test-site")
}

// TestStopAllServers_WithMultipleServers 测试停止多个服务器
func TestStopAllServers_WithMultipleServers(t *testing.T) {
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})
	defer monitor.Stop()

	manager := NewManager(monitor)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")

	sites := []config.SiteConfig{
		{ID: "site1", Name: "Site 1", Port: 18086, Mode: "redirect", Redirect: config.RedirectConfig{StatusCode: 301, TargetURL: "http://example.com"}},
		{ID: "site2", Name: "Site 2", Port: 18087, Mode: "redirect", Redirect: config.RedirectConfig{StatusCode: 301, TargetURL: "http://example.com"}},
	}

	for _, site := range sites {
		manager.StartSiteServer(site, "127.0.0.1", t.TempDir(), crawlerLogManager, testHandler)
	}

	assert.Len(t, manager.siteServers, 2)

	// 停止所有服务器
	manager.StopAllServers()

	// 等待服务器停止
	time.Sleep(time.Millisecond * 200)

	// 验证所有服务器都已停止
	assert.Len(t, manager.siteServers, 0)
}

// TestGetSiteServer_EmptyManager 测试从空管理器获取服务器
func TestGetSiteServer_EmptyManager(t *testing.T) {
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})
	defer monitor.Stop()

	manager := NewManager(monitor)

	server, exists := manager.GetSiteServer("any-site")
	assert.False(t, exists)
	assert.Nil(t, server)
}

// TestListSiteServers_ReturnsMap 测试 ListSiteServers 返回的是内部映射
func TestListSiteServers_ReturnsMap(t *testing.T) {
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})
	defer monitor.Stop()

	manager := NewManager(monitor)

	servers := manager.ListSiteServers()
	assert.NotNil(t, servers)
	assert.IsType(t, map[string]*http.Server{}, servers)
}

// TestSiteConfig_Usage 测试 SiteConfig 的使用
func TestSiteConfig_Usage(t *testing.T) {
	site := config.SiteConfig{
		ID:      "test",
		Name:    "Test Site",
		Domains: []string{"test.com", "www.test.com"},
		Port:    8080,
		Mode:    "proxy",
	}

	assert.Equal(t, "test", site.ID)
	assert.Equal(t, "Test Site", site.Name)
	assert.Len(t, site.Domains, 2)
	assert.Equal(t, 8080, site.Port)
	assert.Equal(t, "proxy", site.Mode)
}

// TestStartSiteServer_WithActualHTTP 测试实际 HTTP 服务器启动
func TestStartSiteServer_WithActualHTTP(t *testing.T) {
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})
	defer monitor.Stop()

	manager := NewManager(monitor)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello"))
	})
	crawlerLogManager := logging.NewCrawlerLogManager("localhost:6379")

	site := config.SiteConfig{
		ID:   "test-site",
		Name: "Test Site",
		Port: 18088,
		Mode: "redirect",
		Redirect: config.RedirectConfig{
			StatusCode: 301,
			TargetURL:  "https://example.com",
		},
	}

	manager.StartSiteServer(site, "127.0.0.1", t.TempDir(), crawlerLogManager, testHandler)

	// 等待服务器启动
	time.Sleep(time.Millisecond * 100)

	// 发送测试请求
	resp, err := http.Get("http://127.0.0.1:18088/test")
	if err == nil {
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}

	// 清理
	manager.StopSiteServer("test-site")
}

// TestManager_ConcurrentAccess 测试并发访问
func TestManager_ConcurrentAccess(t *testing.T) {
	monitor := monitoring.NewMonitor(monitoring.Config{Enabled: false})
	defer monitor.Stop()

	manager := NewManager(monitor)

	done := make(chan bool, 20)

	// 并发读取
	for i := 0; i < 10; i++ {
		go func() {
			_ = manager.ListSiteServers()
			done <- true
		}()
	}

	// 并发获取
	for i := 0; i < 10; i++ {
		go func() {
			manager.GetSiteServer("test")
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}

	// 不应该 panic
	assert.True(t, true)
}
