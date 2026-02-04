package cache

import (
	"fmt"
	"strings"
	"time"

	"prerender-shield/internal/redis"
)

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
}

// manager 缓存管理器实现
type manager struct {
	redisClient *redis.Client
}

// NewManager 创建新的缓存管理器
func NewManager(redisClient *redis.Client) Manager {
	return &manager{
		redisClient: redisClient,
	}
}

// Set 设置缓存
func (m *manager) Set(siteID, key string, value []byte, expiration time.Duration) error {
	cacheKey := fmt.Sprintf("cache:%s:%s", siteID, key)
	return m.redisClient.Set(cacheKey, value, expiration)
}

// Get 获取缓存
func (m *manager) Get(siteID, key string) ([]byte, error) {
	cacheKey := fmt.Sprintf("cache:%s:%s", siteID, key)
	value, err := m.redisClient.Get(cacheKey)
	if err != nil {
		return nil, err
	}
	if value == "" {
		return nil, nil
	}
	return []byte(value), nil
}

// Delete 删除缓存
func (m *manager) Delete(siteID, key string) error {
	cacheKey := fmt.Sprintf("cache:%s:%s", siteID, key)
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

	return map[string]interface{}{
		"hits":      hits,
		"misses":    misses,
		"total":     total,
		"hit_rate":  hitRate,
		"cache_count": cacheCount,
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
