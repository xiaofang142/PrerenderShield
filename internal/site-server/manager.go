package siteserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring"
	"prerender-shield/internal/ssl"
)

// Manager 站点服务器管理器

type Manager struct {
	mu          sync.RWMutex
	siteServers map[string]*http.Server
	sslManager  ssl.Manager
	monitor     *monitoring.Monitor
}

// NewManager 创建站点服务器管理器实例
func NewManager(monitor *monitoring.Monitor, sslManager ssl.Manager) *Manager {
	return &Manager{
		siteServers: make(map[string]*http.Server),
		sslManager:  sslManager,
		monitor:     monitor,
	}
}

// StartSiteServer 启动站点服务器
func (m *Manager) StartSiteServer(site config.SiteConfig, serverAddress string, staticDir string, crawlerLogManager *logging.CrawlerLogManager, siteHandler http.Handler) {
	// Check if SSL is enabled for this site
	if site.SSL.Enabled {
		m.startTLSServer(site, serverAddress, siteHandler)
	} else {
		m.startHTTPServer(site, serverAddress, siteHandler)
	}
}

// startHTTPServer starts a plain HTTP server
func (m *Manager) startHTTPServer(site config.SiteConfig, serverAddress string, handler http.Handler) {
	siteAddr := fmt.Sprintf("%s:%d", serverAddress, site.Port)
	siteServer := &http.Server{
		Addr:    siteAddr,
		Handler: handler,
	}

	m.mu.Lock()
	m.siteServers[site.ID] = siteServer
	m.mu.Unlock()

	go func() {
		if err := siteServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.DefaultLogger.Fatal("站点 %s(%s) 启动失败: %v", site.Name, site.ID, err)
		}
	}()

	logging.DefaultLogger.Info("站点 %s(%s) 启动在 %s，模式: %s", site.Name, site.ID, siteAddr, site.Mode)
}

// startTLSServer starts an HTTPS server with TLS
func (m *Manager) startTLSServer(site config.SiteConfig, serverAddress string, handler http.Handler) {
	// Determine TLS config
	var tlsConfig *tls.Config

	if site.SSL.CertFile != "" && site.SSL.KeyFile != "" {
		// Use provided cert/key files
		cert, err := tls.LoadX509KeyPair(site.SSL.CertFile, site.SSL.KeyFile)
		if err != nil {
			logging.DefaultLogger.Error("Failed to load SSL certificate for site %s: %v. Falling back to HTTP.", site.ID, err)
			m.startHTTPServer(site, serverAddress, handler)
			return
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
	} else if m.sslManager != nil {
		// Try to get certificate from SSL manager
		cert, err := m.getSSLCertForSite(site)
		if err != nil {
			logging.DefaultLogger.Warn("No SSL certificate for site %s: %v. Starting HTTP only.", site.ID, err)
			m.startHTTPServer(site, serverAddress, handler)
			return
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{*cert},
			MinVersion:   tls.VersionTLS12,
		}
	} else {
		logging.DefaultLogger.Warn("SSL enabled but no certificate source for site %s. Starting HTTP only.", site.ID)
		m.startHTTPServer(site, serverAddress, handler)
		return
	}

	// HTTPS server
	httpsAddr := fmt.Sprintf("%s:%d", serverAddress, site.Port)
	httpsServer := &http.Server{
		Addr:      httpsAddr,
		Handler:   handler,
		TLSConfig: tlsConfig,
	}

	m.siteServers[site.ID] = httpsServer

	go func() {
		if err := httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			logging.DefaultLogger.Fatal("站点 %s(%s) HTTPS 启动失败: %v", site.Name, site.ID, err)
		}
	}()

	logging.DefaultLogger.Info("站点 %s(%s) HTTPS 启动在 %s，模式: %s", site.Name, site.ID, httpsAddr, site.Mode)

	// Start HTTP server for redirect and ACME challenges if HTTPPort is configured
	if site.SSL.HTTPPort > 0 {
		m.startHTTPRedirectServer(site, serverAddress, handler)
	}
}

// startHTTPRedirectServer starts HTTP server for redirects and ACME challenges
func (m *Manager) startHTTPRedirectServer(site config.SiteConfig, serverAddress string, handler http.Handler) {
	httpAddr := fmt.Sprintf("%s:%d", serverAddress, site.SSL.HTTPPort)

	var httpHandler http.Handler
	if site.SSL.ForceHTTPS {
		// Redirect all HTTP to HTTPS
		httpHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			target := "https://" + r.Host + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
		})
	} else {
		// Use the same handler (for ACME challenges)
		httpHandler = handler
	}

	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: httpHandler,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.DefaultLogger.Warn("站点 %s HTTP 重定向服务器启动失败: %v", site.ID, err)
		}
	}()

	logging.DefaultLogger.Info("站点 %s HTTP 重定向服务器启动在 %s", site.ID, httpAddr)
}

// getSSLCertForSite gets SSL certificate for a site from SSL manager
func (m *Manager) getSSLCertForSite(site config.SiteConfig) (*tls.Certificate, error) {
	// Try each domain
	for _, domain := range site.Domains {
		cert, err := m.sslManager.GetCertificate(domain)
		if err == nil {
			return cert, nil
		}
	}
	return nil, fmt.Errorf("no SSL certificate found for site domains: %v", site.Domains)
}

// AddHSTSHeaders adds HSTS headers to response
func AddHSTSHeaders(w http.ResponseWriter, maxAge int) {
	if maxAge <= 0 {
		maxAge = 31536000 // 1 year default
	}
	w.Header().Set("Strict-Transport-Security", fmt.Sprintf("max-age=%d; includeSubDomains", maxAge))
}

// StopSiteServer 停止站点服务器
func (m *Manager) StopSiteServer(siteID string) error {
	if server, exists := m.siteServers[siteID]; exists {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logging.DefaultLogger.Info("关闭站点 %s 失败: %v", siteID, err)
			return err
		}
		logging.DefaultLogger.Info("关闭站点 %s 成功", siteID)
		delete(m.siteServers, siteID)
		return nil
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
			logging.DefaultLogger.Info("停止站点 %s 失败: %v", siteName, err)
		}
	}
}

// GetDomains returns all domains for a site
func GetDomains(site config.SiteConfig) []string {
	return site.Domains
}

// MatchDomain checks if a request host matches any of the site's domains
func MatchDomain(host string, domains []string) bool {
	// Remove port from host
	if idx := strings.Index(host, ":"); idx > 0 {
		host = host[:idx]
	}
	for _, domain := range domains {
		if host == domain {
			return true
		}
		// Support wildcard matching
		if strings.HasPrefix(domain, "*.") {
			suffix := domain[1:] // .example.com
			if strings.HasSuffix(host, suffix) {
				return true
			}
		}
	}
	return false
}
