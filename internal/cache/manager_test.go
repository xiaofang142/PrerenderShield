package cache

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	// 简单实现，支持 cache:* 和 cache:site:* 模式
	var keys []string
	for key := range m.data {
		if pattern == "cache:*" {
			keys = append(keys, key)
		} else if strings.HasSuffix(pattern, "*") {
			// 处理 cache:site:* 这样的模式
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(key, prefix) {
				keys = append(keys, key)
			}
		} else if key == pattern {
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
	for k, v := range values {
		m.data[key+":"+k] = ""
		_ = v
	}
	return nil
}

func (m *MockRedisClient) Incr(key string) (int64, error) {
	return 1, nil
}

func (m *MockRedisClient) HashSet(key, field string, value interface{}) error {
	return nil
}

func (m *MockRedisClient) HashGet(key, field string) (string, error) {
	return "", nil
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
