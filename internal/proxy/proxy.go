package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"prerender-shield/internal/redis"
	"prerender-shield/internal/services"
)

// Proxy 反向代理接口
type Proxy interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	AddBackend(siteID, backendURL string) error
	RemoveBackend(siteID string) error
	GetBackend(siteID string) (string, error)
}

// proxy 反向代理实现
type proxy struct {
	domainResolver services.DomainResolver
	redisClient    *redis.Client
	backends       map[string]*url.URL
	reverseProxies map[string]*httputil.ReverseProxy
	mutex          sync.RWMutex
	transport      *http.Transport
}

// NewProxy 创建新的反向代理
func NewProxy(domainResolver services.DomainResolver, redisClient *redis.Client) Proxy {
	// 创建HTTP传输池
	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	p := &proxy{
		domainResolver: domainResolver,
		redisClient:    redisClient,
		backends:       make(map[string]*url.URL),
		reverseProxies: make(map[string]*httputil.ReverseProxy),
		transport:      transport,
	}

	// 从Redis加载后端配置
	p.loadBackendsFromRedis()

	return p
}

// loadBackendsFromRedis 从Redis加载后端配置
func (p *proxy) loadBackendsFromRedis() {
	// 获取所有后端配置
	backendKeys, err := p.redisClient.Keys("backend:*")
	if err != nil {
		return
	}

	for _, key := range backendKeys {
		siteID := strings.TrimPrefix(key, "backend:")
		backendURL, err := p.redisClient.Get(key)
		if err != nil || backendURL == "" {
			continue
		}

		backend, err := url.Parse(backendURL)
		if err != nil {
			continue
		}

		p.backends[siteID] = backend
	}
}

// ServeHTTP 处理HTTP请求
func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 获取请求域名
	domain := r.Host
	if domain == "" {
		http.Error(w, "Host header is required", http.StatusBadRequest)
		return
	}

	// 解析域名到站点ID
	siteID, err := p.domainResolver.Resolve(domain)
	if err != nil || siteID == "" {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	// 获取后端URL
	backendURL, err := p.GetBackend(siteID)
	if err != nil || backendURL == "" {
		http.Error(w, "Backend not found", http.StatusNotFound)
		return
	}

	// 获取或创建反向代理实例
	reverseProxy, err := p.getOrCreateReverseProxy(siteID, backendURL)
	if err != nil {
		http.Error(w, "Failed to create reverse proxy", http.StatusInternalServerError)
		return
	}

	// 处理请求
	reverseProxy.ServeHTTP(w, r)
}

// getOrCreateReverseProxy 获取或创建反向代理实例
func (p *proxy) getOrCreateReverseProxy(siteID, backendURL string) (*httputil.ReverseProxy, error) {
	// 尝试从缓存获取
	p.mutex.RLock()
	reverseProxy, ok := p.reverseProxies[siteID]
	p.mutex.RUnlock()

	if ok {
		return reverseProxy, nil
	}

	// 解析后端URL
	backend, err := url.Parse(backendURL)
	if err != nil {
		return nil, fmt.Errorf("invalid backend URL: %w", err)
	}

	// 创建新的反向代理
	newReverseProxy := httputil.NewSingleHostReverseProxy(backend)

	// 设置自定义传输
	newReverseProxy.Transport = p.transport

	// 自定义请求修改
	newReverseProxy.Director = func(req *http.Request) {
		// 修改请求URL
		req.URL.Scheme = backend.Scheme
		req.URL.Host = backend.Host
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/")

		// 添加唯一凭证
		req.Header.Set("X-Site-ID", siteID)
		req.Header.Set("X-Proxy-ID", "prerender-shield")

		// 保留原始请求头
		// 注意：这里不需要手动复制，httputil.ReverseProxy会自动处理

		// 修改Host头
		req.Host = backend.Host
	}

	// 自定义响应处理
	newReverseProxy.ModifyResponse = func(resp *http.Response) error {
		// 添加代理响应头
		resp.Header.Set("X-Proxy-By", "prerender-shield")
		return nil
	}

	// 添加到缓存
	p.mutex.Lock()
	p.reverseProxies[siteID] = newReverseProxy
	p.mutex.Unlock()

	return newReverseProxy, nil
}

// AddBackend 添加后端URL
func (p *proxy) AddBackend(siteID, backendURL string) error {
	backend, err := url.Parse(backendURL)
	if err != nil {
		return fmt.Errorf("invalid backend URL: %w", err)
	}

	// 更新内存中的后端配置
	p.mutex.Lock()
	p.backends[siteID] = backend
	// 删除旧的反向代理实例，以便创建新的
	delete(p.reverseProxies, siteID)
	p.mutex.Unlock()

	// 持久化到Redis
	if p.redisClient != nil {
		backendKey := fmt.Sprintf("backend:%s", siteID)
		if err := p.redisClient.Set(backendKey, backendURL, 0); err != nil {
			return fmt.Errorf("failed to save backend to Redis: %w", err)
		}
	}

	return nil
}

// RemoveBackend 移除后端URL
func (p *proxy) RemoveBackend(siteID string) error {
	// 从内存中移除后端配置
	p.mutex.Lock()
	delete(p.backends, siteID)
	delete(p.reverseProxies, siteID)
	p.mutex.Unlock()

	// 从Redis中移除
	if p.redisClient != nil {
		backendKey := fmt.Sprintf("backend:%s", siteID)
		if err := p.redisClient.Del(backendKey); err != nil {
			return fmt.Errorf("failed to delete backend from Redis: %w", err)
		}
	}

	return nil
}

// GetBackend 获取后端URL
func (p *proxy) GetBackend(siteID string) (string, error) {
	p.mutex.RLock()
	backend, ok := p.backends[siteID]
	p.mutex.RUnlock()

	if !ok {
		return "", fmt.Errorf("backend not found for site: %s", siteID)
	}

	return backend.String(), nil
}
