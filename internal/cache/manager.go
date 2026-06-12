package cache

import (
	"fmt"
	"strings"
	"time"

	"prerender-shield/internal/redis"
)

// CacheEntry 缓存条目结构
type CacheEntry struct {
	Data      []byte `json:"data"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
	Priority  int    `json:"priority"`
	HitCount  int64  `json:"hit_count"`
	LastHitAt int64  `json:"last_hit_at"`
}

// manager 缓存管理器（纯 Redis）
type manager struct {
	redisClient RedisClientInterface
}

// NewManager 创建基于 Redis 的缓存管理器
func NewManager(redisClient RedisClientInterface) Manager {
	return &manager{redisClient: redisClient}
}

// NewManagerWithClient 使用具体 Redis 客户端创建
func NewManagerWithClient(redisClient *redis.Client) Manager {
	return NewManager(redisClient)
}

// DefaultTTL 默认缓存过期时间
const DefaultTTL = 1 * time.Hour

// RedisClientInterface Redis 客户端接口
type RedisClientInterface interface {
	Get(key string) (string, error)
	Set(key string, value interface{}, expiration time.Duration) error
	Del(key string) error
	DelMultiple(keys []string) error
	Keys(pattern string) ([]string, error)
	Incr(key string) (int64, error)
	HashSet(key, field string, value interface{}) error
	HashSetAll(key string, values map[string]interface{}) error
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

func getCacheKey(siteID, key string) string {
	return fmt.Sprintf("cache:%s:%s", siteID, key)
}

// Set 设置缓存
func (m *manager) Set(siteID, key string, value []byte, expiration time.Duration) error {
	return m.SetWithPriority(siteID, key, value, expiration, 3)
}

// SetWithPriority 设置缓存（带优先级，Redis Hash 存储）
func (m *manager) SetWithPriority(siteID, key string, value []byte, expiration time.Duration, priority int) error {
	cacheKey := getCacheKey(siteID, key)
	now := time.Now()

	if err := m.redisClient.Set(cacheKey, string(value), expiration); err != nil {
		return err
	}

	metaKey := fmt.Sprintf("%s:meta", cacheKey)
	meta := map[string]interface{}{
		"created_at":  now.Unix(),
		"expires_at":  now.Add(expiration).Unix(),
		"priority":    priority,
		"hit_count":   0,
		"last_hit_at": 0,
	}
	m.redisClient.HashSetAll(metaKey, meta)
	m.redisClient.Expire(metaKey, expiration)
	return nil
}

// Get 获取缓存
func (m *manager) Get(siteID, key string) ([]byte, error) {
	cacheKey := getCacheKey(siteID, key)
	value, err := m.redisClient.Get(cacheKey)
	if err != nil {
		return nil, err
	}
	if value == "" {
		return nil, nil
	}

	metaKey := fmt.Sprintf("%s:meta", cacheKey)
	if hc, err := m.redisClient.HashGet(metaKey, "hit_count"); err == nil && hc != "" {
		var count int64
		fmt.Sscanf(hc, "%d", &count)
		m.redisClient.HashSet(metaKey, "hit_count", count+1)
	} else {
		m.redisClient.HashSet(metaKey, "hit_count", 1)
	}
	m.redisClient.HashSet(metaKey, "last_hit_at", time.Now().Unix())

	return []byte(value), nil
}

// GetCacheEntry 获取缓存条目详情（从 Redis meta）
func (m *manager) GetCacheEntry(siteID, key string) (*CacheEntry, error) {
	cacheKey := getCacheKey(siteID, key)
	value, err := m.redisClient.Get(cacheKey)
	if err != nil || value == "" {
		return nil, err
	}

	metaKey := fmt.Sprintf("%s:meta", cacheKey)
	meta, err := m.redisClient.HashGetAll(metaKey)
	if err != nil {
		meta = make(map[string]string)
	}

	entry := &CacheEntry{Data: []byte(value)}
	if ct, ok := meta["created_at"]; ok {
		fmt.Sscanf(ct, "%d", &entry.CreatedAt)
	}
	if et, ok := meta["expires_at"]; ok {
		fmt.Sscanf(et, "%d", &entry.ExpiresAt)
	}
	if pr, ok := meta["priority"]; ok {
		fmt.Sscanf(pr, "%d", &entry.Priority)
	}
	if hc, ok := meta["hit_count"]; ok {
		fmt.Sscanf(hc, "%d", &entry.HitCount)
	}
	if lh, ok := meta["last_hit_at"]; ok {
		fmt.Sscanf(lh, "%d", &entry.LastHitAt)
	}
	return entry, nil
}

// Delete 删除缓存
func (m *manager) Delete(siteID, key string) error {
	cacheKey := getCacheKey(siteID, key)
	metaKey := fmt.Sprintf("%s:meta", cacheKey)
	m.redisClient.Del(metaKey)
	return m.redisClient.Del(cacheKey)
}

// Clear 清除站点所有缓存
func (m *manager) Clear(siteID string) error {
	pattern := fmt.Sprintf("cache:%s:*", siteID)
	return m.deleteByPattern(pattern)
}

// ClearAll 清除所有缓存
func (m *manager) ClearAll() error {
	return m.deleteByPattern("cache:*")
}

func (m *manager) deleteByPattern(pattern string) error {
	keys, err := m.redisClient.Keys(pattern)
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		return m.redisClient.DelMultiple(keys)
	}
	return nil
}

// EvictLowPriority 淘汰低优先级缓存（Redis 中通过 TTL 自然过期，此操作手动触发）
func (m *manager) EvictLowPriority(siteID string, count int) error {
	pattern := fmt.Sprintf("cache:%s:*:meta", siteID)
	keys, err := m.redisClient.Keys(pattern)
	if err != nil {
		return err
	}

	evicted := 0
	for _, metaKey := range keys {
		if evicted >= count {
			break
		}
		prStr, _ := m.redisClient.HashGet(metaKey, "priority")
		if prStr == "1" || prStr == "2" {
			cacheKey := strings.TrimSuffix(metaKey, ":meta")
			m.redisClient.Del(cacheKey)
			m.redisClient.Del(metaKey)
			evicted++
		}
	}
	return nil
}

// GetStats 获取缓存统计
func (m *manager) GetStats(siteID string) (map[string]interface{}, error) {
	pattern := fmt.Sprintf("cache:%s:*", siteID)
	keys, err := m.redisClient.Keys(pattern)
	if err != nil {
		return nil, err
	}

	totalKeys := 0
	for _, k := range keys {
		if !strings.HasSuffix(k, ":meta") {
			totalKeys++
		}
	}

	stats := map[string]interface{}{
		"total_keys": totalKeys,
		"site_id":    siteID,
		"type":       "redis",
	}
	return stats, nil
}

// IncrementHit 增加命中计数
func (m *manager) IncrementHit(siteID string) error {
	key := fmt.Sprintf("stats:hits:%s", siteID)
	_, err := m.redisClient.Incr(key)
	return err
}

// IncrementMiss 增加未命中计数
func (m *manager) IncrementMiss(siteID string) error {
	key := fmt.Sprintf("stats:miss:%s", siteID)
	_, err := m.redisClient.Incr(key)
	return err
}
