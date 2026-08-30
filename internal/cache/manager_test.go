package cache

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRedisClient 模拟 Redis 客户端 (简化版本用于测试)
type MockRedisClient struct {
	data map[string]string
}

func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		data: make(map[string]string),
	}
}

// 实现 redis.Client 的基本方法
func (m *MockRedisClient) Get(key string) (string, error) {
	if val, ok := m.data[key]; ok {
		return val, nil
	}
	return "", nil
}

func (m *MockRedisClient) Set(key string, value interface{}, expiration time.Duration) error {
	switch v := value.(type) {
	case string:
		m.data[key] = v
	case []byte:
		m.data[key] = string(v)
	default:
		m.data[key] = ""
	}
	return nil
}

func (m *MockRedisClient) Del(key string) error {
	delete(m.data, key)
	return nil
}

func (m *MockRedisClient) DelMultiple(keys []string) error {
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

func (m *MockRedisClient) Keys(pattern string) ([]string, error) {
	// 通用通配：* 可出现在任意位置（转义其余正则元字符），支持 cache:site:*:meta 中段通配
	var keys []string
	rePattern := "^" + regexp.QuoteMeta(pattern) + "$"
	rePattern = strings.ReplaceAll(rePattern, regexp.QuoteMeta("*"), ".*")
	re, err := regexp.Compile(rePattern)
	if err != nil {
		return nil, err
	}
	for key := range m.data {
		if re.MatchString(key) {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func (m *MockRedisClient) ClearCache() error {
	m.data = make(map[string]string)
	return nil
}

func (m *MockRedisClient) HashSetAll(key string, values map[string]interface{}) error {
	// 物化 hash 主键：真实 Redis 中 hash 是独立键，Keys(pattern) 必须能扫到
	if _, ok := m.data[key]; !ok {
		m.data[key] = ""
	}
	for k, v := range values {
		m.data[key+"::"+k] = fmt.Sprintf("%v", v)
	}
	return nil
}

func (m *MockRedisClient) Incr(key string) (int64, error) {
	return 1, nil
}

func (m *MockRedisClient) HashSet(key, field string, value interface{}) error {
	if _, ok := m.data[key]; !ok {
		m.data[key] = ""
	}
	m.data[key+"::"+field] = fmt.Sprintf("%v", value)
	return nil
}

func (m *MockRedisClient) HashGet(key, field string) (string, error) {
	return m.data[key+"::"+field], nil
}

func (m *MockRedisClient) HashIncrBy(key, field string, incr int64) (int64, error) {
	return incr, nil
}

func (m *MockRedisClient) HashGetAll(key string) (map[string]string, error) {
	return nil, nil
}

func (m *MockRedisClient) Expire(key string, expiration time.Duration) error {
	return nil
}

func (m *MockRedisClient) TTL(key string) (time.Duration, error) {
	return time.Hour, nil
}

// 其他必需方法（空实现）
func (m *MockRedisClient) SaveSession(sessionID string, data map[string]interface{}, expiration time.Duration) error {
	return nil
}
func (m *MockRedisClient) CheckSessionExists(sessionID string) (bool, error) {
	return true, nil
}
func (m *MockRedisClient) DeleteSession(sessionID string) error {
	return nil
}
func (m *MockRedisClient) GetAllSessions() ([]map[string]interface{}, error) {
	return nil, nil
}
func (m *MockRedisClient) SavePreheatTask(siteID, url, status string, metadata map[string]interface{}) error {
	return nil
}
func (m *MockRedisClient) GetPreheatTask(url string) (map[string]interface{}, error) {
	return nil, nil
}
func (m *MockRedisClient) UpdatePreheatTaskStatus(url, status string) error {
	return nil
}
func (m *MockRedisClient) GetPreheatTasksByStatus(status string) ([]map[string]interface{}, error) {
	return nil, nil
}
func (m *MockRedisClient) DeletePreheatTask(url string) error {
	return nil
}
func (m *MockRedisClient) SaveSiteStats(siteID string, stats map[string]interface{}, expiration time.Duration) error {
	return nil
}
func (m *MockRedisClient) GetSiteStats(siteID string) (map[string]interface{}, error) {
	return nil, nil
}
func (m *MockRedisClient) SaveUser(user map[string]interface{}, expiration time.Duration) error {
	return nil
}
func (m *MockRedisClient) GetUser(username string) (map[string]interface{}, error) {
	return nil, nil
}
func (m *MockRedisClient) GetAllUsers() ([]map[string]interface{}, error) {
	return nil, nil
}
func (m *MockRedisClient) CheckUserExists(username string) (bool, error) {
	return false, nil
}
func (m *MockRedisClient) DeleteUser(username string) error {
	return nil
}
func (m *MockRedisClient) SaveWafConfig(siteID string, config interface{}, expiration time.Duration) error {
	return nil
}
func (m *MockRedisClient) GetWafConfig(siteID string) (interface{}, error) {
	return nil, nil
}

func TestCacheManager_Set(t *testing.T) {
	mockRedis := NewMockRedisClient()
	manager := NewManager(mockRedis)

	err := manager.Set("site-123", "test-key", []byte("test-value"), time.Hour)

	assert.NoError(t, err)
	assert.Equal(t, "test-value", mockRedis.data["cache:site-123:test-key"])
}

func TestCacheManager_Get(t *testing.T) {
	mockRedis := NewMockRedisClient()
	manager := NewManager(mockRedis)

	// 先设置值
	mockRedis.data["cache:site-123:test-key"] = "test-value"

	// 获取值
	value, err := manager.Get("site-123", "test-key")

	assert.NoError(t, err)
	assert.Equal(t, []byte("test-value"), value)
}

func TestCacheManager_Get_NotFound(t *testing.T) {
	mockRedis := NewMockRedisClient()
	manager := NewManager(mockRedis)

	value, err := manager.Get("site-123", "nonexistent-key")

	assert.NoError(t, err)
	assert.Nil(t, value)
}

func TestCacheManager_Delete(t *testing.T) {
	mockRedis := NewMockRedisClient()
	manager := NewManager(mockRedis)

	// 先设置值
	mockRedis.data["cache:site-123:test-key"] = "test-value"

	// 删除
	err := manager.Delete("site-123", "test-key")

	assert.NoError(t, err)
	_, exists := mockRedis.data["cache:site-123:test-key"]
	assert.False(t, exists)
}

func TestCacheManager_Clear(t *testing.T) {
	mockRedis := NewMockRedisClient()
	manager := NewManager(mockRedis)

	// 设置多个值
	mockRedis.data["cache:site-123:key1"] = "value1"
	mockRedis.data["cache:site-123:key2"] = "value2"
	mockRedis.data["cache:site-456:key3"] = "value3"

	// 清除 site-123 的缓存
	err := manager.Clear("site-123")

	assert.NoError(t, err)
	_, exists1 := mockRedis.data["cache:site-123:key1"]
	_, exists2 := mockRedis.data["cache:site-123:key2"]
	_, exists3 := mockRedis.data["cache:site-456:key3"]
	assert.False(t, exists1)
	assert.False(t, exists2)
	assert.True(t, exists3) // site-456 不受影响
}

func TestCacheManager_ClearAll(t *testing.T) {
	mockRedis := NewMockRedisClient()
	manager := NewManager(mockRedis)

	// 设置多个值
	mockRedis.data["cache:site-123:key1"] = "value1"
	mockRedis.data["cache:site-456:key2"] = "value2"

	// 清除所有
	err := manager.ClearAll()

	assert.NoError(t, err)
	assert.Empty(t, mockRedis.data)
}

func TestCacheManager_GetStats(t *testing.T) {
	mockRedis := NewMockRedisClient()
	manager := NewManager(mockRedis)

	stats, err := manager.GetStats("site-123")

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, "redis", stats["type"])
	assert.Equal(t, "site-123", stats["site_id"])
}

func TestCacheManager_IncrementHit(t *testing.T) {
	mockRedis := NewMockRedisClient()
	manager := NewManager(mockRedis)

	err := manager.IncrementHit("site-123")

	assert.NoError(t, err)
}

func TestCacheManager_IncrementMiss(t *testing.T) {
	mockRedis := NewMockRedisClient()
	manager := NewManager(mockRedis)

	err := manager.IncrementMiss("site-123")

	assert.NoError(t, err)
}

func TestNewManager(t *testing.T) {
	mockRedis := NewMockRedisClient()
	manager := NewManager(mockRedis)

	assert.NotNil(t, manager)
}

func TestCacheManager_ListEntries(t *testing.T) {
	mockRedis := NewMockRedisClient()
	manager := NewManager(mockRedis)

	// 写入信封格式条目（"s"/"h"/"e"/"c" 字段）
	env := `{"s":200,"h":"<html>page body</html>","e":9999999999,"c":1700000000}`
	require.NoError(t, mockRedis.Set("cache:site1:prerender:host/path/@desktop", env, time.Hour))
	// meta 键必须被跳过
	require.NoError(t, mockRedis.Set("cache:site1:prerender:host/path/@desktop:meta", `{"priority":2}`, time.Hour))
	// 类型错误信封跳过（非 JSON 形态会被当作 legacy 裸 HTML 兼容解析，不算非法）
	require.NoError(t, mockRedis.Set("cache:site1:prerender:bad@desktop", `{"s":"wrong-type"}`, time.Hour))

	entries, err := manager.ListEntries("site1", 10)
	require.NoError(t, err)
	if len(entries) != 1 {
		t.Fatalf("want 1 valid entry (meta/invalid skipped), got %d: %+v", len(entries), entries)
	}
	e := entries[0]
	if e.URL != "host/path/" {
		t.Fatalf("display URL stripped: %q", e.URL)
	}
	if e.Status != 200 || e.Device != "desktop" || e.SizeBytes != len(env) || !e.Fresh {
		t.Fatalf("entry fields wrong: %+v", e)
	}
}

func TestCacheManager_ListEntries_Limit(t *testing.T) {
	mockRedis := NewMockRedisClient()
	manager := NewManager(mockRedis)
	for i := 0; i < 5; i++ {
		env := `{"s":200,"h":"x","e":9999999999}`
		require.NoError(t, mockRedis.Set(fmt.Sprintf("cache:site1:prerender:p%d/@desktop", i), env, time.Hour))
	}
	entries, err := manager.ListEntries("site1", 3)
	require.NoError(t, err)
	if len(entries) != 3 {
		t.Fatalf("limit broken: %d", len(entries))
	}
}

func TestCacheManager_GetCacheEntry(t *testing.T) {
	mockRedis := NewMockRedisClient()
	manager := NewManager(mockRedis)
	require.NoError(t, mockRedis.Set("cache:site1:key1", "value1", time.Hour))

	entry, err := manager.GetCacheEntry("site1", "key1")
	require.NoError(t, err)
	if entry == nil || string(entry.Data) != "value1" {
		t.Fatalf("GetCacheEntry broken: %+v", entry)
	}
	// 不存在
	entry, err = manager.GetCacheEntry("site1", "nope")
	require.NoError(t, err)
	if entry != nil {
		t.Fatalf("missing key must return nil entry, got %+v", entry)
	}
}

func TestCacheManager_SetWithPriority_And_EvictLowPriority(t *testing.T) {
	mockRedis := NewMockRedisClient()
	manager := NewManager(mockRedis)

	// priority: 1=低 2=中 3=高
	require.NoError(t, manager.SetWithPriority("site1", "low", []byte("a"), time.Hour, 1))
	require.NoError(t, manager.SetWithPriority("site1", "mid", []byte("b"), time.Hour, 2))
	require.NoError(t, manager.SetWithPriority("site1", "high", []byte("c"), time.Hour, 3))

	// meta 中记录 priority（供驱逐决策）
	lowMeta := "cache:site1:low:meta"
	require.NoError(t, mockRedis.HashSet(lowMeta, "priority", "1"))
	midMeta := "cache:site1:mid:meta"
	require.NoError(t, mockRedis.HashSet(midMeta, "priority", "2"))
	highMeta := "cache:site1:high:meta"
	require.NoError(t, mockRedis.HashSet(highMeta, "priority", "3"))

	require.NoError(t, manager.EvictLowPriority("site1", 2))

	// 低/中被驱逐（Get 对缺失键返回 nil 值），高保留
	lowVal, _ := manager.Get("site1", "low")
	if lowVal != nil {
		t.Fatal("low priority must be evicted")
	}
	midVal, _ := manager.Get("site1", "mid")
	if midVal != nil {
		t.Fatal("mid priority must be evicted")
	}
	highVal, err := manager.Get("site1", "high")
	require.NoError(t, err)
	if string(highVal) != "c" {
		t.Fatal("high priority must survive")
	}
}

func TestUnmarshalEnvelope(t *testing.T) {
	// 非 JSON 形态 = legacy 裸 HTML 兼容解析（Status 默认 200）
	env, ok := unmarshalEnvelope([]byte("junk html"))
	if !ok || env.Status != 200 {
		t.Fatalf("legacy raw html must parse with status 200, got ok=%v %+v", ok, env)
	}
	// JSON 字段类型错误必须失败
	if _, ok := unmarshalEnvelope([]byte(`{"s":"not-int"}`)); ok {
		t.Fatal("wrong-type envelope must fail")
	}
	// 合法信封
	env, ok = unmarshalEnvelope([]byte(`{"s":404,"h":"x","e":123}`))
	if !ok || env.Status != 404 {
		t.Fatalf("valid envelope broken: ok=%v %+v", ok, env)
	}
}

func TestEnvelopeDevice(t *testing.T) {
	if envelopeDevice("prerender:x/@mobile") != "mobile" {
		t.Fatal("mobile bucket detect broken")
	}
	if envelopeDevice("prerender:x/@desktop") != "desktop" {
		t.Fatal("desktop bucket detect broken")
	}
	if envelopeDevice("prerender:x") != "desktop" {
		t.Fatal("legacy key defaults desktop")
	}
}

func TestDisplayURL(t *testing.T) {
	if got := displayURL("prerender:host/path/@desktop"); got != "host/path/" {
		t.Fatalf("displayURL=%q", got)
	}
	if got := displayURL("prerender:host/path/@mobile"); got != "host/path/" {
		t.Fatalf("displayURL mobile=%q", got)
	}
	if got := displayURL("weird-form"); got != "weird-form" {
		t.Fatalf("fallback must return raw: %q", got)
	}
}
