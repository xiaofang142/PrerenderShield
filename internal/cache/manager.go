package cache

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"prerender-shield/internal/redis"
)

// CacheEntry 缓存条目结构
type CacheEntry struct {
	Data      []byte      `json:"data"`
	CreatedAt int64       `json:"created_at"`
	ExpiresAt int64       `json:"expires_at"`
	Priority  int         `json:"priority"`  // 优先级 1-5，5 为最高
	HitCount  int64       `json:"hit_count"` // 命中次数
	LastHitAt int64       `json:"last_hit_at"`
}

// MemoryCache 内存缓存结构
type MemoryCache struct {
	mu      sync.RWMutex
	items   map[string]*CacheEntry
	maxSize int
	currentSize int
}

// memoryManager 缓存管理器实现（支持多级缓存）
type memoryManager struct {
	redisClient RedisClientInterface
	memoryCache *MemoryCache
	enabled     bool // 是否启用多级缓存
}

// DefaultTTL 默认缓存过期时间
const DefaultTTL = 1 * time.Hour

// MaxMemoryCacheSize 最大内存缓存条目数
const MaxMemoryCacheSize = 1000

// RedisClientInterface 定义 Redis 客户端接口，便于测试
type RedisClientInterface interface {
	Get(key string) (string, error)
	Set(key string, value interface{}, expiration time.Duration) error
	Del(key string) error
	DelMultiple(keys []string) error
	Keys(pattern string) ([]string, error)
	Incr(key string) (int64, error)
	HashSet(key, field string, value interface{}) error
	HashGet(key, field string) (string, error)
	HashGetAll(key string) (map[string]string, error)
	Expire(key string, expiration time.Duration) error
	TTL(key string) (time.Duration, error)
	ClearCache() error
}

// Manager 缓存管理器接口
type Manager interface {
	Set(siteID, key string, value []byte, expiration time.Duration) error
	Get(siteID, key string) ([]byte, error)
	Delete(siteID, key string) error
	Clear(siteID string) error
	ClearAll() error
	GetStats(siteID string) (map[string]interface{}, error)
	IncrementHit(siteID string) error
	IncrementMiss(siteID string) error
	SetWithPriority(siteID, key string, value []byte, expiration time.Duration, priority int) error
	GetCacheEntry(siteID, key string) (*CacheEntry, error)
	EvictLowPriority(siteID string, count int) error
}

// memoryManager 缓存管理器实现
type manager struct {
	redisClient RedisClientInterface
	memoryCache *MemoryCache
	enabled     bool
}

// NewManager 创建新的缓存管理器（带内存缓存）
func NewManager(redisClient RedisClientInterface) Manager {
	return &manager{
		redisClient: redisClient,
		enabled:     true,
		memoryCache: &MemoryCache{
			items:   make(map[string]*CacheEntry),
			maxSize: MaxMemoryCacheSize,
		},
	}
}

// NewManagerWithClient 使用具体 Redis 客户端创建管理器 (向后兼容)
func NewManagerWithClient(redisClient *redis.Client) Manager {
	return NewManager(redisClient)
}

// getCacheKey 生成缓存键
func getCacheKey(siteID, key string) string {
	return fmt.Sprintf("cache:%s:%s", siteID, key)
}

// getMemoryCacheKey 生成内存缓存键
func getMemoryCacheKey(siteID, key string) string {
	return fmt.Sprintf("%s:%s", siteID, key)
}

// memoryCacheGet 从内存缓存获取
func (m *manager) memoryCacheGet(siteID, key string) ([]byte, bool) {
	if !m.enabled || m.memoryCache == nil {
		return nil, false
	}

	m.memoryCache.mu.RLock()
	defer m.memoryCache.mu.RUnlock()

	cacheKey := getMemoryCacheKey(siteID, key)
	entry, exists := m.memoryCache.items[cacheKey]
	if !exists {
		return nil, false
	}

	// 检查是否过期
	if time.Now().Unix() > entry.ExpiresAt {
		return nil, false
	}

	// 更新命中统计
	entry.HitCount++
	entry.LastHitAt = time.Now().Unix()

	return entry.Data, true
}

// memoryCacheSet 设置内存缓存
func (m *manager) memoryCacheSet(siteID, key string, data []byte, expiration time.Duration, priority int) {
	if !m.enabled || m.memoryCache == nil {
		return
	}

	cacheKey := getMemoryCacheKey(siteID, key)

	m.memoryCache.mu.Lock()
	defer m.memoryCache.mu.Unlock()

	// 如果缓存已满，删除最低优先级的条目
	if m.memoryCache.currentSize >= m.memoryCache.maxSize {
		m.memoryCache.removeOldestLowPriorityLocked()
	}

	expiresAt := time.Now().Add(expiration).Unix()
	entry := &CacheEntry{
		Data:      data,
		CreatedAt: time.Now().Unix(),
		ExpiresAt: expiresAt,
		Priority:  priority,
		HitCount:  0,
		LastHitAt: 0,
	}

	m.memoryCache.items[cacheKey] = entry
	m.memoryCache.currentSize++
}

// memoryCacheDelete 删除内存缓存
func (m *manager) memoryCacheDelete(siteID, key string) {
	if !m.enabled || m.memoryCache == nil {
		return
	}

	cacheKey := getMemoryCacheKey(siteID, key)

	m.memoryCache.mu.Lock()
	defer m.memoryCache.mu.Unlock()

	if _, exists := m.memoryCache.items[cacheKey]; exists {
		delete(m.memoryCache.items, cacheKey)
		m.memoryCache.currentSize--
	}
}

// removeOldestLowPriorityLocked 删除最旧的低优先级条目（已持有锁）
func (m *MemoryCache) removeOldestLowPriorityLocked() {
	// 找到优先级最低且最久未使用的条目
	var targetKey string
	var targetPriority int = 6
	var targetLastHitAt int64 = time.Now().Unix()

	for key, entry := range m.items {
		if entry.Priority < targetPriority || (entry.Priority == targetPriority && entry.LastHitAt < targetLastHitAt) {
			targetKey = key
			targetPriority = entry.Priority
			targetLastHitAt = entry.LastHitAt
		}
	}

	if targetKey != "" {
		delete(m.items, targetKey)
		m.currentSize--
	}
}

// Set 设置缓存（默认优先级）
func (m *manager) Set(siteID, key string, value []byte, expiration time.Duration) error {
	return m.SetWithPriority(siteID, key, value, expiration, 3)
}

// SetWithPriority 设置缓存（带优先级）
func (m *manager) SetWithPriority(siteID, key string, value []byte, expiration time.Duration, priority int) error {
	cacheKey := getCacheKey(siteID, key)

	// 先设置到内存缓存
	m.memoryCacheSet(siteID, key, value, expiration, priority)

	// 再设置到 Redis
	return m.redisClient.Set(cacheKey, value, expiration)
}

// Get 获取缓存（多级缓存）
func (m *manager) Get(siteID, key string) ([]byte, error) {
	// 先从内存缓存获取
	if data, hit := m.memoryCacheGet(siteID, key); hit {
		return data, nil
	}

	// 从 Redis 获取
	cacheKey := getCacheKey(siteID, key)
	value, err := m.redisClient.Get(cacheKey)
	if err != nil {
		return nil, err
	}
	if value == "" {
		return nil, nil
	}

	data := []byte(value)

	// 回填到内存缓存
	m.memoryCacheSet(siteID, key, data, 30*time.Minute, 3)

	return data, nil
}

// GetCacheEntry 获取缓存条目详情
func (m *manager) GetCacheEntry(siteID, key string) (*CacheEntry, error) {
	cacheKey := getMemoryCacheKey(siteID, key)

	m.memoryCache.mu.RLock()
	defer m.memoryCache.mu.RUnlock()

	entry, exists := m.memoryCache.items[cacheKey]
	if !exists {
		return nil, fmt.Errorf("cache entry not found")
	}

	return entry, nil
}

// EvictLowPriority 驱逐低优先级缓存
func (m *manager) EvictLowPriority(siteID string, count int) error {
	if !m.enabled || m.memoryCache == nil {
		return nil
	}

	m.memoryCache.mu.Lock()
	defer m.memoryCache.mu.Unlock()

	evicted := 0
	for key, entry := range m.memoryCache.items {
		if evicted >= count {
			break
		}
		// 只驱逐优先级低于 3 的条目
		if entry.Priority < 3 {
			delete(m.memoryCache.items, key)
			m.memoryCache.currentSize--
			evicted++
		}
	}

	return nil
}

// Delete 删除缓存
func (m *manager) Delete(siteID, key string) error {
	// 删除内存缓存
	m.memoryCacheDelete(siteID, key)

	// 删除 Redis 缓存
	cacheKey := getCacheKey(siteID, key)
	return m.redisClient.Del(cacheKey)
}

// Clear 清理站点缓存
func (m *manager) Clear(siteID string) error {
	cachePattern := fmt.Sprintf("cache:%s:*", siteID)
	keys, err := m.redisClient.Keys(cachePattern)
	if err != nil {
		return fmt.Errorf("failed to get cache keys: %w", err)
	}
	if len(keys) > 0 {
		return m.redisClient.DelMultiple(keys)
	}
	return nil
}

// ClearAll 清理所有缓存
func (m *manager) ClearAll() error {
	return m.redisClient.ClearCache()
}

// GetStats 获取缓存统计信息
func (m *manager) GetStats(siteID string) (map[string]interface{}, error) {
	// 获取缓存命中率
	hitsStr, err := m.redisClient.Get(fmt.Sprintf("cache:%s:hits", siteID))
	if err != nil {
		hitsStr = "0"
	}
	hits := parseInt64(hitsStr)

	missesStr, err := m.redisClient.Get(fmt.Sprintf("cache:%s:misses", siteID))
	if err != nil {
		missesStr = "0"
	}
	misses := parseInt64(missesStr)

	// 计算命中率
	total := hits + misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	// 获取缓存数量
	cachePattern := fmt.Sprintf("cache:%s:*", siteID)
	cacheKeys, err := m.redisClient.Keys(cachePattern)
	if err != nil {
		return nil, fmt.Errorf("failed to get cache keys: %w", err)
	}

	// 过滤掉统计键
	cacheCount := 0
	for _, key := range cacheKeys {
		if !strings.Contains(key, ":hits") && !strings.Contains(key, ":misses") {
			cacheCount++
		}
	}

	// 获取内存缓存统计
	memoryCount := 0
	memoryHits := int64(0)
	if m.memoryCache != nil {
		m.memoryCache.mu.RLock()
		for key, entry := range m.memoryCache.items {
			if strings.HasPrefix(key, siteID+":") {
				memoryCount++
				memoryHits += entry.HitCount
			}
		}
		m.memoryCache.mu.RUnlock()
	}

	return map[string]interface{}{
		"hits":              hits,
		"misses":            misses,
		"total":             total,
		"hit_rate":          hitRate,
		"cache_count":       cacheCount,
		"memory_cache_count": memoryCount,
		"memory_cache_hits":  memoryHits,
		"memory_cache_size":  m.memoryCache.currentSize,
		"memory_cache_max":   m.memoryCache.maxSize,
	}, nil
}

// IncrementHit 增加缓存命中计数
func (m *manager) IncrementHit(siteID string) error {
	_, err := m.redisClient.Incr(fmt.Sprintf("cache:%s:hits", siteID))
	return err
}

// IncrementMiss 增加缓存未命中计数
func (m *manager) IncrementMiss(siteID string) error {
	_, err := m.redisClient.Incr(fmt.Sprintf("cache:%s:misses", siteID))
	return err
}

// parseInt64 将字符串转换为int64
func parseInt64(s string) int64 {
	var i int64
	fmt.Sscanf(s, "%d", &i)
	return i
}
