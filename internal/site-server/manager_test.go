package siteserver

import (
	"testing"

	"prerender-shield/internal/monitoring"
)

// TestNewManager 测试创建站点服务器管理器
func TestNewManager(t *testing.T) {
	// 创建监控器
	monitor := monitoring.NewMonitor(monitoring.Config{
		Enabled:           true,
		PrometheusAddress: ":0", // 使用随机端口
	})
	defer monitor.Stop()

	// 创建站点服务器管理器
	manager := NewManager(monitor)
	if manager == nil {
		t.Error("NewManager returned nil")
	}

	// 验证站点服务器映射被正确初始化
	if len(manager.siteServers) != 0 {
		t.Errorf("Expected empty siteServers map, got %d entries", len(manager.siteServers))
	}
}

// TestListSiteServers 测试列出站点服务器
func TestListSiteServers(t *testing.T) {
	// 创建监控器
	monitor := monitoring.NewMonitor(monitoring.Config{
		Enabled:           true,
		PrometheusAddress: ":0", // 使用随机端口
	})
	defer monitor.Stop()

	// 创建站点服务器管理器
	manager := NewManager(monitor)

	// 测试列出空的站点服务器列表
	servers := manager.ListSiteServers()
	if len(servers) != 0 {
		t.Errorf("Expected empty servers list, got %d entries", len(servers))
	}

	// 注意：我们不测试StartSiteServer和StopSiteServer，因为它们需要实际的网络端口
	// 这些功能在集成测试中会被更好地测试
}

// TestStopAllServers 测试停止所有站点服务器
func TestStopAllServers(t *testing.T) {
	// 创建监控器
	monitor := monitoring.NewMonitor(monitoring.Config{
		Enabled:           true,
		PrometheusAddress: ":0", // 使用随机端口
	})
	defer monitor.Stop()

	// 创建站点服务器管理器
	manager := NewManager(monitor)

	// 测试停止空的站点服务器列表
	manager.StopAllServers()

	// 验证没有错误发生
	// 这个测试主要是为了验证StopAllServers在没有服务器时不会崩溃
}

// TestGetSiteServer 测试获取站点服务器
func TestGetSiteServer(t *testing.T) {
	// 创建监控器
	monitor := monitoring.NewMonitor(monitoring.Config{
		Enabled:           true,
		PrometheusAddress: ":0", // 使用随机端口
	})
	defer monitor.Stop()

	// 创建站点服务器管理器
	manager := NewManager(monitor)

	// 测试获取不存在的站点服务器
	server, exists := manager.GetSiteServer("non_existent_site")
	if exists {
		t.Error("Expected server not to exist")
	}
	if server != nil {
		t.Error("Expected nil server for non-existent site")
	}
}
