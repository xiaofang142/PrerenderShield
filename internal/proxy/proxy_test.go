package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"

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

func TestProxy(t *testing.T) {
	// 创建Redis客户端（使用内存模式或测试Redis）
	redisClient, err := redis.NewClient("localhost:6379", "", 0)
	assert.NoError(t, err)
	defer redisClient.Close()

	// 创建模拟域名解析器
	resolver := &mockDomainResolver{
		resolveMap: make(map[string]string),
	}

	// 创建测试站点ID和域名
	testSiteID := "test-site-1"
	testDomain := "test.example.com"
	testBackendURL := "http://localhost:8080"

	// 添加域名映射
	resolver.AddMapping(testDomain, testSiteID)

	// 创建代理
	proxyInstance := NewProxy(resolver, redisClient)

	// 测试1: 添加后端URL
	t.Run("AddBackend", func(t *testing.T) {
		err := proxyInstance.AddBackend(testSiteID, testBackendURL)
		assert.NoError(t, err)

		// 检查后端URL是否存储到Redis
		backendKey := "backend:" + testSiteID
		backendURL, err := redisClient.Get(backendKey)
		assert.NoError(t, err)
		assert.Equal(t, testBackendURL, backendURL)
	})

	// 测试2: 获取后端URL
	t.Run("GetBackend", func(t *testing.T) {
		backendURL, err := proxyInstance.GetBackend(testSiteID)
		assert.NoError(t, err)
		assert.Equal(t, testBackendURL, backendURL)
	})

	// 测试3: 移除后端URL
	t.Run("RemoveBackend", func(t *testing.T) {
		err := proxyInstance.RemoveBackend(testSiteID)
		assert.NoError(t, err)

		// 检查后端URL是否从Redis删除
		backendKey := "backend:" + testSiteID
		backendURL, err := redisClient.Get(backendKey)
		assert.NoError(t, err)
		assert.Empty(t, backendURL)
	})

	// 测试4: 测试从Redis加载后端配置
	t.Run("LoadBackendsFromRedis", func(t *testing.T) {
		// 先添加后端配置到Redis
		testBackendURL := "http://localhost:8081"
		backendKey := "backend:" + testSiteID
		err := redisClient.Set(backendKey, testBackendURL, 0)
		assert.NoError(t, err)

		// 创建新的代理实例，应该从Redis加载配置
		proxyInstance := NewProxy(resolver, redisClient)

		// 检查后端URL是否加载成功
		backendURL, err := proxyInstance.GetBackend(testSiteID)
		assert.NoError(t, err)
		assert.Equal(t, testBackendURL, backendURL)
	})
}
