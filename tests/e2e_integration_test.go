package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/cache"
	"prerender-shield/internal/proxy"
	"prerender-shield/internal/redis"
)

// 模拟域名解析器
type mockDomainResolver struct {
	resolveMap map[string]string
}

func (m *mockDomainResolver) Resolve(domain string) (string, error) {
	return m.resolveMap[domain], nil
}

func (m *mockDomainResolver) AddMapping(domain, siteID string) error {
	m.resolveMap[domain] = siteID
	return nil
}

func (m *mockDomainResolver) RemoveMapping(domain string) error {
	delete(m.resolveMap, domain)
	return nil
}

func (m *mockDomainResolver) ListMappings() (map[string]string, error) {
	return m.resolveMap, nil
}

// 模拟后端服务器
func createMockBackendServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Test Page</title></head><body><h1>Hello from backend</h1></body></html>`))
	}))
}

func TestEndToEndFlow(t *testing.T) {
	// 创建Redis客户端
	redisClient, err := redis.NewClient("localhost:6379", "", 15) // DB15 隔离：集成测试绝不触碰运行环境 DB0
	assert.NoError(t, err)
	defer redisClient.Close()

	// 创建缓存管理器
	cacheManager := cache.NewManager(redisClient)

	// 创建域名解析器
	resolver := &mockDomainResolver{
		resolveMap: make(map[string]string),
	}

	// 创建测试站点和域名
	testSiteID := "test-site-e2e"
	testDomain := "e2e-test.example.com"

	// 添加域名映射
	resolver.AddMapping(testDomain, testSiteID)

	// 创建模拟后端服务器
	backendServer := createMockBackendServer()
	defer backendServer.Close()

	// 创建代理
	proxyInstance := proxy.NewProxy(resolver, redisClient)

	// 添加后端URL
	err = proxyInstance.AddBackend(testSiteID, backendServer.URL)
	assert.NoError(t, err)

	// 测试1: 正常请求流程
	t.Run("NormalRequest", func(t *testing.T) {
		// 创建测试请求
		req, err := http.NewRequest("GET", "/", nil)
		assert.NoError(t, err)
		req.Host = testDomain

		// 创建响应记录器
		w := httptest.NewRecorder()

		// 处理请求
		proxyInstance.ServeHTTP(w, req)

		// 检查响应
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Hello from backend")
	})

	// 测试2: 爬虫请求流程
	t.Run("CrawlerRequest", func(t *testing.T) {
		// 创建测试请求
		req, err := http.NewRequest("GET", "/", nil)
		assert.NoError(t, err)
		req.Host = testDomain
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)")

		// 创建响应记录器
		w := httptest.NewRecorder()

		// 处理请求
		proxyInstance.ServeHTTP(w, req)

		// 检查响应
		assert.Equal(t, http.StatusOK, w.Code)
	})

	// 测试3: 缓存功能
	t.Run("CacheFunctionality", func(t *testing.T) {
		// 设置缓存
		cacheKey := "test-page"
		cacheValue := []byte("Cached content")
		err := cacheManager.Set(testSiteID, cacheKey, cacheValue, 1*time.Hour)
		assert.NoError(t, err)

		// 获取缓存
		cachedContent, err := cacheManager.Get(testSiteID, cacheKey)
		assert.NoError(t, err)
		assert.Equal(t, cacheValue, cachedContent)

		// 清理缓存
		err = cacheManager.Delete(testSiteID, cacheKey)
		assert.NoError(t, err)

		// 验证缓存已删除
		cachedContent, err = cacheManager.Get(testSiteID, cacheKey)
		assert.NoError(t, err)
		assert.Nil(t, cachedContent)
	})

	// 测试4: 域名解析失败
	t.Run("DomainResolutionFailure", func(t *testing.T) {
		// 创建测试请求，使用未映射的域名
		req, err := http.NewRequest("GET", "/", nil)
		assert.NoError(t, err)
		req.Host = "unknown.example.com"

		// 创建响应记录器
		w := httptest.NewRecorder()

		// 处理请求
		proxyInstance.ServeHTTP(w, req)

		// 检查响应
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	// 测试5: 后端未找到
	t.Run("BackendNotFound", func(t *testing.T) {
		// 创建新的站点ID，不添加后端
		newSiteID := "test-site-no-backend"
		newDomain := "no-backend.example.com"
		resolver.AddMapping(newDomain, newSiteID)

		// 创建测试请求
		req, err := http.NewRequest("GET", "/", nil)
		assert.NoError(t, err)
		req.Host = newDomain

		// 创建响应记录器
		w := httptest.NewRecorder()

		// 处理请求
		proxyInstance.ServeHTTP(w, req)

		// 检查响应
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
