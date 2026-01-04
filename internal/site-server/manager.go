package siteserver

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
)

// Manager 站点服务器管理器

type Manager struct {
	siteServers map[string]*http.Server
	monitor     *monitoring.Monitor
}

// NewManager 创建站点服务器管理器实例
func NewManager(monitor *monitoring.Monitor) *Manager {
	return &Manager{
		siteServers: make(map[string]*http.Server),
		monitor:     monitor,
	}
}

// StartSiteServer 启动站点服务器
func (m *Manager) StartSiteServer(site config.SiteConfig, serverAddress string, staticDir string, crawlerLogManager *logging.CrawlerLogManager, siteHandler http.Handler) {
	// 启动站点服务器
	siteAddr := fmt.Sprintf("%s:%d", serverAddress, site.Port)
	siteServer := &http.Server{
		Addr:    siteAddr,
		Handler: siteHandler,
	}

	// 保存站点服务器引用，用于后续管理，使用站点ID作为键
	m.siteServers[site.ID] = siteServer

	// 启动站点服务器
	go func(siteName, siteID, addr string, server *http.Server) {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("站点 %s(%s) 启动失败: %v", siteName, siteID, err)
		}
	}(site.Name, site.ID, siteAddr, siteServer)

	log.Printf("站点 %s(%s) 启动在 %s，模式: %s", site.Name, site.ID, siteAddr, site.Mode)
}

// StopSiteServer 停止站点服务器
func (m *Manager) StopSiteServer(siteID string) error {
	// 检查站点服务器是否存在
	if server, exists := m.siteServers[siteID]; exists {
		// 关闭服务器
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("关闭站点 %s 失败: %v", siteID, err)
			return err
		} else {
			log.Printf("关闭站点 %s 成功", siteID)
			// 从映射中删除服务器
			delete(m.siteServers, siteID)
			return nil
		}
	}
	return nil
}

// GetSiteServer 获取站点服务器实例
func (m *Manager) GetSiteServer(siteID string) (*http.Server, bool) {
	server, exists := m.siteServers[siteID]
	return server, exists
}

// ListSiteServers 列出所有站点服务器
func (m *Manager) ListSiteServers() map[string]*http.Server {
	return m.siteServers
}

// StopAllServers 停止所有站点服务器
func (m *Manager) StopAllServers() {
	for siteName := range m.siteServers {
		if err := m.StopSiteServer(siteName); err != nil {
			log.Printf("停止站点 %s 失败: %v", siteName, err)
		}
	}
}
