package cache

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

// MockRedisClient for tiered cache tests
type MockRedisClient struct {
	data map[string][]byte
}

func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		data: make(map[string][]byte),
	}
}

func (m *MockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	if val, ok := m.data[key]; ok {
		return redis.NewStringResult(string(val), nil)
	}
	return redis.NewStringResult("", redis.Nil)
}

func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		data = []byte("")
	}
	m.data[key] = data
	return redis.NewStatusResult("", nil)
}

func (m *MockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	deleted := int64(0)
	for _, key := range keys {
		if _, ok := m.data[key]; ok {
			delete(m.data, key)
			deleted++
		}
	}
	return redis.NewIntResult(deleted, nil)
}

func (m *MockRedisClient) Keys(ctx context.Context, pattern string) *redis.StringSliceCmd {
	var keys []string
	for key := range m.data {
		if matchPatternSimple(pattern, key) {
			keys = append(keys, key)
		}
	}
	return redis.NewStringSliceResult(keys, nil)
}

func matchPatternSimple(pattern, key string) bool {
	if pattern == "*" {
		return true
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(key) >= len(prefix) && key[:len(prefix)] == prefix
	}
	return pattern == key
}

// Tests

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.True(t, config.L1Enabled)
	assert.Greater(t, config.L1MaxSize, 0)
	assert.Greater(t, config.L1DefaultTTL, time.Duration(0))
	assert.True(t, config.L2Enabled)
	assert.NotEmpty(t, config.L2Prefix)
	assert.True(t, config.WriteThrough)
	assert.True(t, config.ReadThrough)
}

func TestDefaultPreheaterConfig(t *testing.T) {
	config := DefaultPreheaterConfig()

	assert.True(t, config.Enabled)
	assert.Greater(t, config.MinHits, 0)
	assert.Greater(t, config.PriorityBoost, 0)
	assert.Greater(t, config.PreheatInterval, time.Duration(0))
	assert.Greater(t, config.BatchSize, 0)
}

func TestDefaultInvalidatorConfig(t *testing.T) {
	config := DefaultInvalidatorConfig()

	assert.True(t, config.EnableVersioning)
	assert.Greater(t, config.DefaultTTL, time.Duration(0))
}

func TestTierType_String(t *testing.T) {
	assert.Equal(t, "L1", TierL1.String())
	assert.Equal(t, "L2", TierL2.String())
	assert.Equal(t, "unknown", TierType(99).String())
}

func TestCacheEntry_IsExpired(t *testing.T) {
	// Not expired
	entry := &CacheEntry{
		ExpiresAt: time.Now().Add(time.Hour),
	}
	assert.False(t, entry.IsExpired())

	// Expired
	entry = &CacheEntry{
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	assert.True(t, entry.IsExpired())

	// No expiration
	entry = &CacheEntry{
		ExpiresAt: time.Time{},
	}
	assert.False(t, entry.IsExpired())
}

func TestCacheEntry_TTL(t *testing.T) {
	// With expiration
	expiresAt := time.Now().Add(time.Hour)
	entry := &CacheEntry{
		ExpiresAt: expiresAt,
	}
	ttl := entry.TTL()
	assert.Greater(t, ttl, time.Duration(0))
	assert.Less(t, ttl, time.Hour)

	// No expiration
	entry = &CacheEntry{
		ExpiresAt: time.Time{},
	}
	assert.Equal(t, time.Duration(0), entry.TTL())
}

func TestMetrics_RecordL1Hit(t *testing.T) {
	metrics := &Metrics{}
	metrics.RecordL1Hit(10 * time.Millisecond)

	assert.Equal(t, int64(1), metrics.L1Hits)
	assert.Equal(t, int64(1), metrics.TotalReads)
	assert.NotEqual(t, 0.0, metrics.HitRate)
}

func TestMetrics_RecordL1Miss(t *testing.T) {
	metrics := &Metrics{}
	metrics.RecordL1Miss(5 * time.Millisecond)

	assert.Equal(t, int64(1), metrics.L1Misses)
	assert.Equal(t, int64(1), metrics.TotalReads)
}

func TestMetrics_RecordL2Hit(t *testing.T) {
	metrics := &Metrics{}
	metrics.RecordL2Hit(20 * time.Millisecond)

	assert.Equal(t, int64(1), metrics.L2Hits)
	assert.Equal(t, int64(1), metrics.TotalReads)
}

func TestMetrics_RecordL2Miss(t *testing.T) {
	metrics := &Metrics{}
	metrics.RecordL2Miss(15 * time.Millisecond)

	assert.Equal(t, int64(1), metrics.L2Misses)
}

func TestMetrics_RecordL1Write(t *testing.T) {
	metrics := &Metrics{}
	metrics.RecordL1Write(8 * time.Millisecond)

	assert.Equal(t, int64(1), metrics.L1Writes)
	assert.Equal(t, int64(1), metrics.TotalWrites)
}

func TestMetrics_RecordL2Write(t *testing.T) {
	metrics := &Metrics{}
	metrics.RecordL2Write(12 * time.Millisecond)

	assert.Equal(t, int64(1), metrics.L2Writes)
	assert.Equal(t, int64(1), metrics.TotalWrites)
}

func TestMetrics_RecordL2Error(t *testing.T) {
	metrics := &Metrics{}
	metrics.RecordL2Error()

	assert.Equal(t, int64(1), metrics.L2Errors)
}

func TestMetrics_RecordEviction(t *testing.T) {
	metrics := &Metrics{}
	metrics.RecordEviction()

	assert.Equal(t, int64(1), metrics.L1Evictions)
}

func TestMetrics_SetSizes(t *testing.T) {
	metrics := &Metrics{}
	metrics.SetSizes(100, 500)

	assert.Equal(t, 100, metrics.L1Size)
	assert.Equal(t, int64(500), metrics.L2Size)
}

func TestMetrics_GetSnapshot(t *testing.T) {
	metrics := &Metrics{}
	metrics.RecordL1Hit(10 * time.Millisecond)
	metrics.RecordL1Miss(5 * time.Millisecond)
	metrics.RecordEviction()
	metrics.SetSizes(50, 200)

	snapshot := metrics.GetSnapshot()

	assert.NotNil(t, snapshot)
	assert.Equal(t, int64(1), snapshot["l1_hits"])
	assert.Equal(t, int64(1), snapshot["l1_misses"])
	assert.Equal(t, int64(1), snapshot["l1_evictions"])
	assert.Equal(t, 50, snapshot["l1_size"])
	assert.Equal(t, int64(200), snapshot["l2_size"])
	assert.Contains(t, snapshot, "hit_rate")
	assert.Contains(t, snapshot, "avg_read_latency")
}

func TestNewTieredCache(t *testing.T) {
	mockRedis := NewMockRedisClient()
	config := DefaultConfig()

	cache := NewTieredCache(config, mockRedis, nil)

	assert.NotNil(t, cache)
	assert.NotNil(t, cache.metrics)
	assert.NotNil(t, cache.l1items)
	assert.NotNil(t, cache.l1queue)
}

func TestTieredCache_SetAndGet(t *testing.T) {
	mockRedis := NewMockRedisClient()
	config := DefaultConfig()
	config.L2Enabled = false // Disable L2 for simpler test

	cache := NewTieredCache(config, mockRedis, nil)
	defer cache.Close()

	// Set
	err := cache.Set("test-key", []byte("test-value"), time.Minute)
	assert.Nil(t, err)

	// Get
	value, err := cache.Get("test-key")
	assert.Nil(t, err)
	assert.Equal(t, []byte("test-value"), value)
}

func TestTieredCache_Get_Miss(t *testing.T) {
	mockRedis := NewMockRedisClient()
	config := DefaultConfig()

	cache := NewTieredCache(config, mockRedis, nil)
	defer cache.Close()

	// Get non-existent key
	value, err := cache.Get("nonexistent")
	assert.Nil(t, err)
	assert.Nil(t, value)
}

func TestTieredCache_Delete(t *testing.T) {
	mockRedis := NewMockRedisClient()
	config := DefaultConfig()
	config.L2Enabled = false

	cache := NewTieredCache(config, mockRedis, nil)
	defer cache.Close()

	// Set and delete
	cache.Set("test-key", []byte("test-value"), time.Minute)
	err := cache.Delete("test-key")
	assert.Nil(t, err)

	// Verify deleted
	value, _ := cache.Get("test-key")
	assert.Nil(t, value)
}

func TestTieredCache_GetStats(t *testing.T) {
	mockRedis := NewMockRedisClient()
	config := DefaultConfig()
	config.L2Enabled = false

	cache := NewTieredCache(config, mockRedis, nil)
	defer cache.Close()

	cache.Set("key1", []byte("value1"), time.Minute)
	cache.Set("key2", []byte("value2"), time.Minute)

	stats := cache.GetStats()

	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats["l1_count"].(int), 2)
	assert.Contains(t, stats, "l1_max_size")
	assert.Contains(t, stats, "write_queue_size")
}

func TestTieredCache_GetMetrics(t *testing.T) {
	mockRedis := NewMockRedisClient()
	config := DefaultConfig()

	cache := NewTieredCache(config, mockRedis, nil)
	defer cache.Close()

	metrics := cache.GetMetrics()
	assert.NotNil(t, metrics)
}

func TestTieredCache_Clear(t *testing.T) {
	mockRedis := NewMockRedisClient()
	config := DefaultConfig()
	config.L2Enabled = false

	cache := NewTieredCache(config, mockRedis, nil)
	defer cache.Close()

	cache.Set("key1", []byte("value1"), time.Minute)
	cache.Set("key2", []byte("value2"), time.Minute)

	err := cache.Clear()
	assert.Nil(t, err)

	stats := cache.GetStats()
	assert.Equal(t, 0, stats["l1_count"])
}

func TestTieredCache_Flush(t *testing.T) {
	mockRedis := NewMockRedisClient()
	config := DefaultConfig()

	cache := NewTieredCache(config, mockRedis, nil)
	defer cache.Close()

	cache.Set("key1", []byte("value1"), time.Minute)

	err := cache.Flush()
	assert.Nil(t, err)
}

func TestTieredCache_GetWithMetadata(t *testing.T) {
	mockRedis := NewMockRedisClient()
	config := DefaultConfig()
	config.L2Enabled = false

	cache := NewTieredCache(config, mockRedis, nil)
	defer cache.Close()

	metadata := map[string]interface{}{"version": 1}
	err := cache.SetWithMetadata("key1", []byte("value1"), time.Minute, metadata)
	assert.Nil(t, err)

	entry, err := cache.GetWithMetadata("key1")
	assert.Nil(t, err)
	assert.NotNil(t, entry)
	assert.Equal(t, []byte("value1"), entry.Value)
}

func TestPreheater_RecordHit(t *testing.T) {
	mockRedis := NewMockRedisClient()
	config := DefaultPreheaterConfig()
	config.MinHits = 3 // Lower threshold for testing

	cache := NewTieredCache(DefaultConfig(), mockRedis, nil)
	preheater := NewPreheater(config, cache, nil)
	defer preheater.Close()

	// Record hits
	for i := 0; i < 5; i++ {
		preheater.RecordHit("test-key")
	}

	// Check stats
	stats := preheater.GetStats()
	assert.Greater(t, stats["total_hits"].(int), 0)
}

func TestPreheater_Preheat(t *testing.T) {
	mockRedis := NewMockRedisClient()
	config := DefaultPreheaterConfig()

	cache := NewTieredCache(DefaultConfig(), mockRedis, nil)
	preheater := NewPreheater(config, cache, nil)
	defer preheater.Close()

	// Manual preheat
	preheater.Preheat([]string{"key1", "key2"}, 5)

	// Give some time for processing
	time.Sleep(50 * time.Millisecond)

	stats := preheater.GetStats()
	assert.NotNil(t, stats)
}

func TestPreheater_GetHotKeys(t *testing.T) {
	mockRedis := NewMockRedisClient()
	config := DefaultPreheaterConfig()
	config.MinHits = 2

	cache := NewTieredCache(DefaultConfig(), mockRedis, nil)
	preheater := NewPreheater(config, cache, nil)
	defer preheater.Close()

	// Make keys hot
	preheater.RecordHit("hot-key1")
	preheater.RecordHit("hot-key1")
	preheater.RecordHit("hot-key2")
	preheater.RecordHit("hot-key2")

	hotKeys := preheater.GetHotKeys()
	assert.GreaterOrEqual(t, len(hotKeys), 2)
}

func TestNewInvalidator(t *testing.T) {
	mockRedis := NewMockRedisClient()
	config := DefaultInvalidatorConfig()

	cache := NewTieredCache(DefaultConfig(), mockRedis, nil)
	invalidator := NewInvalidator(config, cache, nil)

	assert.NotNil(t, invalidator)
	assert.NotNil(t, invalidator.rules)
	assert.NotNil(t, invalidator.versionMap)
}

func TestInvalidator_AddRule(t *testing.T) {
	mockRedis := NewMockRedisClient()
	cache := NewTieredCache(DefaultConfig(), mockRedis, nil)
	invalidator := NewInvalidator(nil, cache, nil)

	rule := InvalidationRule{
		Pattern: "test*",
		OnInvalidate: func(key string) {
			_ = true
		},
	}

	invalidator.AddRule(rule)

	stats := invalidator.GetStats()
	assert.Equal(t, 1, stats["rules_count"])
}

func TestInvalidator_Invalidate(t *testing.T) {
	mockRedis := NewMockRedisClient()
	cache := NewTieredCache(DefaultConfig(), mockRedis, nil)
	invalidator := NewInvalidator(nil, cache, nil)

	// Add a rule
	rule := InvalidationRule{
		Pattern: "test*",
	}
	invalidator.AddRule(rule)

	// Set and invalidate
	cache.Set("test-key", []byte("value"), time.Minute)
	err := invalidator.Invalidate("test-key")
	assert.Nil(t, err)
}

func TestInvalidator_InvalidatePattern(t *testing.T) {
	mockRedis := NewMockRedisClient()
	cache := NewTieredCache(DefaultConfig(), mockRedis, nil)
	invalidator := NewInvalidator(nil, cache, nil)

	// Set some keys
	cache.Set("prefix-key1", []byte("value1"), time.Minute)
	cache.Set("prefix-key2", []byte("value2"), time.Minute)

	err := invalidator.InvalidatePattern("prefix*")
	assert.Nil(t, err)
}

func TestInvalidator_Versioning(t *testing.T) {
	mockRedis := NewMockRedisClient()
	cache := NewTieredCache(DefaultConfig(), mockRedis, nil)
	invalidator := NewInvalidator(nil, cache, nil)

	// Initial version should be 0
	version := invalidator.GetVersion("key1")
	assert.Equal(t, 0, version)

	// Increment
	newVersion := invalidator.IncrementVersion("key1")
	assert.Equal(t, 1, newVersion)

	// Check again
	version = invalidator.GetVersion("key1")
	assert.Equal(t, 1, version)
}

func TestInvalidator_SetWithVersion(t *testing.T) {
	mockRedis := NewMockRedisClient()
	config := DefaultConfig()
	config.L2Enabled = false

	cache := NewTieredCache(config, mockRedis, nil)
	invalidator := NewInvalidator(nil, cache, nil)

	// Increment version first
	invalidator.IncrementVersion("key1")

	// Set with version
	err := invalidator.SetWithVersion(context.Background(), "key1", []byte("value"), time.Minute)
	assert.Nil(t, err)
}

func TestMatchPattern(t *testing.T) {
	// Exact match
	assert.True(t, matchPattern("test", "test"))

	// Wildcard
	assert.True(t, matchPattern("*", "anything"))

	// Prefix with wildcard
	assert.True(t, matchPattern("test*", "test-key"))
	assert.True(t, matchPattern("test*", "test"))
	assert.False(t, matchPattern("test*", "other"))

	// No match
	assert.False(t, matchPattern("test", "other"))
}

func TestTieredCache_SetWithMetadata(t *testing.T) {
	mockRedis := NewMockRedisClient()
	config := DefaultConfig()
	config.L2Enabled = false

	cache := NewTieredCache(config, mockRedis, nil)
	defer cache.Close()

	metadata := map[string]interface{}{
		"version": 1,
		"source":  "test",
	}

	err := cache.SetWithMetadata("key1", []byte("value1"), time.Minute, metadata)
	assert.Nil(t, err)

	entry, err := cache.GetWithMetadata("key1")
	assert.Nil(t, err)
	assert.NotNil(t, entry.Metadata)
}

func TestCacheEntry_Struct(t *testing.T) {
	entry := &CacheEntry{
		Key:       "test-key",
		Value:     []byte("test-value"),
		Metadata:  map[string]interface{}{"version": 1},
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		HitCount:  10,
		LastHitAt: time.Now(),
		Priority:  5,
		Tier:      TierL1,
	}

	assert.Equal(t, "test-key", entry.Key)
	assert.Equal(t, []byte("test-value"), entry.Value)
	assert.Equal(t, int64(10), entry.HitCount)
	assert.Equal(t, 5, entry.Priority)
	assert.Equal(t, TierL1, entry.Tier)
}

func TestPreheatTask_Struct(t *testing.T) {
	task := &PreheatTask{
		Key:      "test-key",
		Priority: 5,
		Source:   "manual",
	}

	assert.Equal(t, "test-key", task.Key)
	assert.Equal(t, 5, task.Priority)
	assert.Equal(t, "manual", task.Source)
}

func TestHeatEntry_Struct(t *testing.T) {
	entry := &HeatEntry{
		Key:      "test-key",
		HitCount: 10,
		LastHit:  time.Now(),
		IsHot:    true,
		Priority: 5,
	}

	assert.Equal(t, "test-key", entry.Key)
	assert.Equal(t, 10, entry.HitCount)
	assert.True(t, entry.IsHot)
}

func TestWriteBackTask_Struct(t *testing.T) {
	task := &writeBackTask{
		key:       "test-key",
		entry:     &CacheEntry{Key: "test-key"},
		timestamp: time.Now(),
	}

	assert.Equal(t, "test-key", task.key)
	assert.NotNil(t, task.entry)
}
