package prerender

import (
	"sync"
	"time"

	"prerender-shield/internal/cache"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/seo"
	"prerender-shield/internal/prerender/pool"
)

// 确保 redis.Client 实现 RedisClient 接口
var _ RedisClient = (*redis.Client)(nil)

// EngineManager 渲染引擎管理器
// 管理多个渲染引擎实例，为不同站点提供独立的渲染能力
// 支持引擎的创建、获取和销毁
//
// 字段:
//
//	engines: 引擎实例映射，键为站点ID，值为对应的渲染引擎
//	redisClient: Redis客户端，用于共享数据
//	cacheManager: 缓存管理器，用于共享缓存
//	mutex: 互斥锁，用于并发安全
//	maxConcurrent: 最大并发渲染数
//	timeout: 默认渲染超时时间
//	seoConfigs: 站点SEO配置映射
//
// 方法:
//
//	NewEngineManager: 创建引擎管理器实例
//	GetEngine: 获取或创建指定站点的渲染引擎
//	RemoveEngine: 移除指定站点的渲染引擎
//	Cleanup: 清理所有引擎实例
//	SetMaxConcurrent: 设置最大并发渲染数
//	SetTimeout: 设置默认渲染超时时间
//	SetSEOConfig: 设置站点SEO配置
//	IsCrawlerRequest: 检测请求是否来自爬虫
//	Render: 渲染指定URL
//
// 示例:
//
//	manager := NewEngineManager(redisClient, cacheManager, 5)
//	manager.SetSEOConfig("site1", seoConfig)
//	engine, _ := manager.GetEngine("site1")
//	result, _ := engine.Render("https://example.com")
type EngineManager struct {
	engines       map[string]Engine
	sharedPool    *pool.Pool // 多站点共享浏览器池（懒创建，统一生命周期）
	redisClient   RedisClient
	cacheManager  cache.Manager
	mutex         sync.RWMutex
	maxConcurrent int
	timeout       time.Duration
	seoConfigs    map[string]*seo.LLMConfig
}

// NewEngineManager 创建渲染引擎管理器实例
//
// 参数:
//
//	redisClient: Redis客户端，用于共享数据
//	cacheManager: 缓存管理器，用于共享缓存
//	maxConcurrent: 最大并发渲染数
//
// 返回值:
//
//	*EngineManager: 渲染引擎管理器实例
//
// 示例:
//
//	manager := NewEngineManager(redisClient, cacheManager, 5)
func NewEngineManager(redisClient RedisClient, cacheManager cache.Manager, maxConcurrent int) *EngineManager {
	return &EngineManager{
		engines:       make(map[string]Engine),
		redisClient:   redisClient,
		cacheManager:  cacheManager,
		maxConcurrent: maxConcurrent,
		timeout:       30 * time.Second,
		seoConfigs:    make(map[string]*seo.LLMConfig),
	}
}

// SetSEOConfig 设置站点的SEO配置
func (m *EngineManager) SetSEOConfig(siteID string, llmConfig *seo.LLMConfig) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.seoConfigs[siteID] = llmConfig
}

// GetEngine 获取或创建指定站点的渲染引擎
//
// 参数:
//
//	siteID: 站点ID
//
// 返回值:
//
//	Engine: 渲染引擎实例
//	error: 错误信息
//
// 示例:
//
//	engine, err := manager.GetEngine("site1")
//	if err != nil {
//		log.Fatal(err)
//	}
func (m *EngineManager) GetEngine(siteID string) (Engine, bool) {
	m.mutex.RLock()
	engine, exists := m.engines[siteID]
	llmConfig := m.seoConfigs[siteID]
	m.mutex.RUnlock()

	if exists {
		return engine, true
	}

	// 创建新的渲染引擎
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 再次检查，避免并发创建
	if engine, exists := m.engines[siteID]; exists {
		return engine, true
	}

	if m.sharedPool == nil {
		m.sharedPool = defaultBrowserPool()
	}
	engine = NewEngineWithSharedPool(m.redisClient, m.cacheManager, m.maxConcurrent, llmConfig, m.sharedPool)
	m.engines[siteID] = engine

	return engine, true
}

// RemoveEngine 移除并关闭指定站点的渲染引擎
//
// 参数:
//
//	siteID: 站点ID
//
// 示例:
//
//	manager.RemoveEngine("site1")
func (m *EngineManager) RemoveEngine(siteID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if engine, ok := m.engines[siteID]; ok {
		engine.Close()
		delete(m.engines, siteID)
	}
}

// Close 关闭并清理所有引擎实例 (P0-1: 作为 Cleanup 的语义化别名)
//
// 示例:
//
//	manager.Close()
func (m *EngineManager) Close() error {
	m.Cleanup()
	return nil
}

// Cleanup 关闭并清理所有引擎实例，最后关闭共享浏览器池
//
// 示例:
//
//	manager.Cleanup()
func (m *EngineManager) Cleanup() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for id, engine := range m.engines {
		engine.Close()
		delete(m.engines, id)
	}
	// 共享池由 Manager 统一关闭（各 Engine 的 Close 不会触碰它）
	if m.sharedPool != nil {
		m.sharedPool.Close()
		m.sharedPool = nil
	}
}

// SetMaxConcurrent 设置最大并发渲染数
//
// 参数:
//
//	maxConcurrent: 最大并发渲染数
//
// 示例:
//
//	manager.SetMaxConcurrent(10)
func (m *EngineManager) SetMaxConcurrent(maxConcurrent int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.maxConcurrent = maxConcurrent
}

// SetTimeout 设置默认渲染超时时间
//
// 参数:
//
//	timeout: 默认渲染超时时间
//
// 示例:
//
//	manager.SetTimeout(60 * time.Second)
func (m *EngineManager) SetTimeout(timeout time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.timeout = timeout
}

// IsCrawlerRequest 检测请求是否来自爬虫
//
// 参数:
//
//	userAgent: 用户代理字符串
//
// 返回值:
//
//	bool: 是否为爬虫请求
//
// 示例:
//
//	isCrawler := manager.IsCrawlerRequest("Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html")
func (m *EngineManager) IsCrawlerRequest(userAgent string) bool {
	// 委托到共享实现，避免与 engine.IsCrawlerRequest 逻辑重复导致维护不一致
	return isCrawlerUserAgent(userAgent)
}

// ListSites 列出所有已创建的引擎的站点ID
//
// 返回值:
//
//	[]string: 站点ID列表
//
// 示例:
//
//	siteIDs := manager.ListSites()
//	for _, siteID := range siteIDs {
//		fmt.Println(siteID)
//	}
func (m *EngineManager) ListSites() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	siteIDs := make([]string, 0, len(m.engines))
	for siteID := range m.engines {
		siteIDs = append(siteIDs, siteID)
	}

	return siteIDs
}
