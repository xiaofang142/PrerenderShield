package cache

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"prerender-shield/internal/prerender/renderkey"
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

// CacheEntrySummary 渲染缓存条目摘要（从页面信封解出，管理端列表用）
type CacheEntrySummary struct {
	URL       string `json:"url"`
	Status    int    `json:"status"`
	ExpiresAt int64  `json:"expires_at"`
	CreatedAt int64  `json:"created_at"`
	SizeBytes int    `json:"size_bytes"`
	Fresh     bool   `json:"fresh"`
	Device    string `json:"device"`
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
	HashIncrBy(key, field string, incr int64) (int64, error)
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
	// ListEntries 列出站点渲染缓存条目摘要（SCAN pattern 键，过滤 meta，解信封提取状态/过期/大小）
	ListEntries(siteID string, limit int) ([]CacheEntrySummary, error)
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
	// 原子自增命中计数：此前 HashGet+HashSet 读改写在并发下会丢失计数
	m.redisClient.HashIncrBy(metaKey, "hit_count", 1)
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

// ListEntries 列出站点渲染缓存条目摘要。
// 只列主键（过滤 :meta 后缀）；信封字段与读路径同源（JSON 信封解析），
// 避免 meta 副键双源不一致。limit<=0 时取默认 200。
func (m *manager) ListEntries(siteID string, limit int) ([]CacheEntrySummary, error) {
	if limit <= 0 {
		limit = 200
	}
	pattern := fmt.Sprintf("cache:%s:*", siteID)
	keys, err := m.redisClient.Keys(pattern)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	entries := make([]CacheEntrySummary, 0, len(keys))
	for _, k := range keys {
		if strings.HasSuffix(k, ":meta") {
			continue
		}
		if len(entries) >= limit {
			break
		}
		// cache:<siteID>:<bizKey> → bizKey 即 renderkey 归一化 URL
		bizKey := strings.TrimPrefix(k, "cache:"+siteID+":")
		raw, err := m.redisClient.Get(k)
		if err != nil || raw == "" {
			continue
		}
		env, ok := unmarshalEnvelope([]byte(raw))
		if !ok {
			continue
		}
		created := int64(0)
		if ttl, err := m.redisClient.TTL(k); err == nil && ttl > 0 {
			created = now.Unix() - int64(ttl.Seconds())
		}
		entries = append(entries, CacheEntrySummary{
			URL:       displayURL(bizKey),
			Status:    env.Status,
			ExpiresAt: env.ExpiresAt,
			CreatedAt: created,
			SizeBytes: len(raw),
			Fresh:     env.fresh(now),
			Device:    envelopeDevice(bizKey),
		})
	}
	return entries, nil
}

// pageEnvelopeLite 页面信封的本地轻量解析（与 prerender.PageEnvelope 字段一致）
type pageEnvelopeLite struct {
	Status    int    `json:"s"`
	HTML      string `json:"h"`
	ExpiresAt int64  `json:"e"`
}

func (e pageEnvelopeLite) fresh(now time.Time) bool {
	return e.ExpiresAt == 0 || now.Unix() < e.ExpiresAt
}

func unmarshalEnvelope(raw []byte) (pageEnvelopeLite, bool) {
	var env pageEnvelopeLite
	s := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(s, "{") {
		// legacy 裸 HTML
		return pageEnvelopeLite{Status: 200}, true
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return pageEnvelopeLite{}, false
	}
	if env.Status == 0 {
		env.Status = 200
	}
	return env, true
}

// envelopeDevice 从业务键提取设备分桶后缀（@mobile/@desktop，无后缀=desktop 兼容旧键）
func envelopeDevice(bizKey string) string {
	if strings.HasSuffix(bizKey, "@mobile") {
		return "mobile"
	}
	return "desktop"
}

// displayURL 业务键转展示 URL：renderkey.StripBizKey 去 prerender: 前缀与设备后缀；
// 异常形态兜底原样返回（不因展示层转换丢失条目）。
func displayURL(bizKey string) string {
	if url, _ := renderkey.StripBizKey(bizKey); url != "" {
		return url
	}
	return bizKey
}
