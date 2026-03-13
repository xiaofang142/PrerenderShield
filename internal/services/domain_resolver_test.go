package services

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockRedisClientForDomainResolver 模拟 Redis 客户端用于域名解析器测试
type MockRedisClientForDomainResolver struct {
	data        map[string]string
	sets        map[string]map[string]bool // 用于集合操作
	setAddCalls []struct {
		key     string
		members []interface{}
	}
	setRemoveCalls []struct {
		key     string
		members []interface{}
	}
}

func NewMockRedisClientForDomainResolver() *MockRedisClientForDomainResolver {
	return &MockRedisClientForDomainResolver{
		data: make(map[string]string),
		sets: make(map[string]map[string]bool),
	}
}

func (m *MockRedisClientForDomainResolver) Get(key string) (string, error) {
	if val, ok := m.data[key]; ok {
		return val, nil
	}
	return "", nil
}

func (m *MockRedisClientForDomainResolver) Set(key string, value interface{}, expiration time.Duration) error {
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

func (m *MockRedisClientForDomainResolver) Del(key string) error {
	delete(m.data, key)
	return nil
}

func (m *MockRedisClientForDomainResolver) SetAdd(key string, members ...interface{}) error {
	if m.sets[key] == nil {
		m.sets[key] = make(map[string]bool)
	}
	for _, member := range members {
		if s, ok := member.(string); ok {
			m.sets[key][s] = true
		}
	}
	m.setAddCalls = append(m.setAddCalls, struct {
		key     string
		members []interface{}
	}{key, members})
	return nil
}

func (m *MockRedisClientForDomainResolver) SetMembers(key string) ([]string, error) {
	if members, ok := m.sets[key]; ok {
		result := make([]string, 0, len(members))
		for member := range members {
			result = append(result, member)
		}
		return result, nil
	}
	return []string{}, nil
}

func (m *MockRedisClientForDomainResolver) SetRemove(key string, members ...interface{}) error {
	if m.sets[key] != nil {
		for _, member := range members {
			if s, ok := member.(string); ok {
				delete(m.sets[key], s)
			}
		}
	}
	m.setRemoveCalls = append(m.setRemoveCalls, struct {
		key     string
		members []interface{}
	}{key, members})
	return nil
}

// 其他必需方法（空实现）
func (m *MockRedisClientForDomainResolver) DelMultiple(keys []string) error {
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

func (m *MockRedisClientForDomainResolver) Keys(pattern string) ([]string, error) {
	return []string{}, nil
}

func (m *MockRedisClientForDomainResolver) ClearCache() error {
	m.data = make(map[string]string)
	return nil
}

func (m *MockRedisClientForDomainResolver) Incr(key string) (int64, error) {
	return 1, nil
}

func (m *MockRedisClientForDomainResolver) HashSet(key, field string, value interface{}) error {
	return nil
}

func (m *MockRedisClientForDomainResolver) HashGet(key, field string) (string, error) {
	return "", nil
}

func (m *MockRedisClientForDomainResolver) HashGetAll(key string) (map[string]string, error) {
	return nil, nil
}

func (m *MockRedisClientForDomainResolver) Expire(key string, expiration time.Duration) error {
	return nil
}

func (m *MockRedisClientForDomainResolver) TTL(key string) (time.Duration, error) {
	return time.Hour, nil
}

func (m *MockRedisClientForDomainResolver) SaveSession(sessionID string, data map[string]interface{}, expiration time.Duration) error {
	return nil
}

func (m *MockRedisClientForDomainResolver) CheckSessionExists(sessionID string) (bool, error) {
	return false, nil
}

func (m *MockRedisClientForDomainResolver) DeleteSession(sessionID string) error {
	return nil
}

func (m *MockRedisClientForDomainResolver) GetAllSessions() ([]map[string]interface{}, error) {
	return nil, nil
}

func (m *MockRedisClientForDomainResolver) SavePreheatTask(siteID, url, status string, metadata map[string]interface{}) error {
	return nil
}

func (m *MockRedisClientForDomainResolver) GetPreheatTask(url string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *MockRedisClientForDomainResolver) UpdatePreheatTaskStatus(url, status string) error {
	return nil
}

func (m *MockRedisClientForDomainResolver) GetPreheatTasksByStatus(status string) ([]map[string]interface{}, error) {
	return nil, nil
}

func (m *MockRedisClientForDomainResolver) DeletePreheatTask(url string) error {
	return nil
}

func (m *MockRedisClientForDomainResolver) SaveSiteStats(siteID string, stats map[string]interface{}, expiration time.Duration) error {
	return nil
}

func (m *MockRedisClientForDomainResolver) GetSiteStats(siteID string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *MockRedisClientForDomainResolver) SaveUser(user map[string]interface{}, expiration time.Duration) error {
	return nil
}

func (m *MockRedisClientForDomainResolver) GetUser(username string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *MockRedisClientForDomainResolver) GetAllUsers() ([]map[string]interface{}, error) {
	return nil, nil
}

func (m *MockRedisClientForDomainResolver) CheckUserExists(username string) (bool, error) {
	return false, nil
}

func (m *MockRedisClientForDomainResolver) DeleteUser(username string) error {
	return nil
}

func (m *MockRedisClientForDomainResolver) SaveWafConfig(siteID string, config interface{}, expiration time.Duration) error {
	return nil
}

func (m *MockRedisClientForDomainResolver) GetWafConfig(siteID string) (interface{}, error) {
	return nil, nil
}

func (m *MockRedisClientForDomainResolver) SetContains(key string, member interface{}) (bool, error) {
	if members, ok := m.sets[key]; ok {
		if s, ok := member.(string); ok {
			return members[s], nil
		}
	}
	return false, nil
}

// Tests for DomainResolver

func TestNewDomainResolver(t *testing.T) {
	mockRedis := NewMockRedisClientForDomainResolver()
	resolver := NewDomainResolver(mockRedis)

	assert.NotNil(t, resolver)
}

func TestDomainResolver_Resolve_ExactMatch(t *testing.T) {
	mockRedis := NewMockRedisClientForDomainResolver()
	mockRedis.data["domain:example.com"] = "site-123"

	resolver := NewDomainResolver(mockRedis)
	siteID, err := resolver.Resolve("example.com")

	assert.Nil(t, err)
	assert.Equal(t, "site-123", siteID)
}

func TestDomainResolver_Resolve_WildcardMatch(t *testing.T) {
	mockRedis := NewMockRedisClientForDomainResolver()
	mockRedis.data["domain:*.example.com"] = "site-456"

	resolver := NewDomainResolver(mockRedis)
	siteID, err := resolver.Resolve("sub.example.com")

	assert.Nil(t, err)
	assert.Equal(t, "site-456", siteID)
}

func TestDomainResolver_Resolve_NoMatch(t *testing.T) {
	mockRedis := NewMockRedisClientForDomainResolver()

	resolver := NewDomainResolver(mockRedis)
	siteID, err := resolver.Resolve("unknown.com")

	assert.Nil(t, err)
	assert.Equal(t, "", siteID)
}

func TestDomainResolver_Resolve_Priority_ExactOverWildcard(t *testing.T) {
	mockRedis := NewMockRedisClientForDomainResolver()
	mockRedis.data["domain:example.com"] = "site-exact"
	mockRedis.data["domain:*.example.com"] = "site-wildcard"

	resolver := NewDomainResolver(mockRedis)
	siteID, err := resolver.Resolve("example.com")

	assert.Nil(t, err)
	assert.Equal(t, "site-exact", siteID) // 应该优先使用精确匹配
}

func TestDomainResolver_Resolve_WildcardSubdomain(t *testing.T) {
	mockRedis := NewMockRedisClientForDomainResolver()
	mockRedis.data["domain:*.example.com"] = "site-wildcard"

	resolver := NewDomainResolver(mockRedis)

	// 测试多级子域名
	siteID, err := resolver.Resolve("deep.sub.example.com")
	assert.Nil(t, err)
	assert.Equal(t, "site-wildcard", siteID)
}

// MockRedisClientWithError 模拟返回错误的 Redis 客户端
type MockRedisClientWithError struct{}

func (m *MockRedisClientWithError) Get(key string) (string, error) {
	return "", fmt.Errorf("redis error")
}

func (m *MockRedisClientWithError) Set(key string, value interface{}, expiration time.Duration) error {
	return fmt.Errorf("redis error")
}

func (m *MockRedisClientWithError) Del(key string) error {
	return fmt.Errorf("redis error")
}

func (m *MockRedisClientWithError) SetAdd(key string, members ...interface{}) error {
	return fmt.Errorf("redis error")
}

func (m *MockRedisClientWithError) SetMembers(key string) ([]string, error) {
	return nil, fmt.Errorf("redis error")
}

func (m *MockRedisClientWithError) SetRemove(key string, members ...interface{}) error {
	return fmt.Errorf("redis error")
}

func TestDomainResolver_Resolve_RedisError(t *testing.T) {
	mockRedis := &MockRedisClientWithError{}
	resolver := NewDomainResolver(mockRedis)
	siteID, err := resolver.Resolve("example.com")

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "redis error")
	assert.Equal(t, "", siteID)
}

func TestDomainResolver_AddMapping_Success(t *testing.T) {
	mockRedis := NewMockRedisClientForDomainResolver()

	resolver := NewDomainResolver(mockRedis)
	err := resolver.AddMapping("example.com", "site-789")

	assert.Nil(t, err)
	assert.Equal(t, "site-789", mockRedis.data["domain:example.com"])
	assert.Contains(t, mockRedis.sets["domains"], "example.com")
}

func TestDomainResolver_AddMapping_RedisError(t *testing.T) {
	mockRedis := &MockRedisClientWithError{}
	resolver := NewDomainResolver(mockRedis)
	err := resolver.AddMapping("example.com", "site-789")

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "redis error")
}

func TestDomainResolver_RemoveMapping_Success(t *testing.T) {
	mockRedis := NewMockRedisClientForDomainResolver()
	mockRedis.data["domain:example.com"] = "site-789"
	mockRedis.sets["domains"] = map[string]bool{"example.com": true}

	resolver := NewDomainResolver(mockRedis)
	err := resolver.RemoveMapping("example.com")

	assert.Nil(t, err)
	assert.NotContains(t, mockRedis.data, "domain:example.com")
	assert.NotContains(t, mockRedis.sets["domains"], "example.com")
}

func TestDomainResolver_RemoveMapping_RedisError(t *testing.T) {
	mockRedis := &MockRedisClientWithError{}
	resolver := NewDomainResolver(mockRedis)
	err := resolver.RemoveMapping("example.com")

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "redis error")
}

func TestDomainResolver_ListMappings_Success(t *testing.T) {
	mockRedis := NewMockRedisClientForDomainResolver()
	mockRedis.data["domain:example1.com"] = "site-1"
	mockRedis.data["domain:example2.com"] = "site-2"
	mockRedis.sets["domains"] = map[string]bool{
		"example1.com": true,
		"example2.com": true,
	}

	resolver := NewDomainResolver(mockRedis)
	mappings, err := resolver.ListMappings()

	assert.Nil(t, err)
	assert.Len(t, mappings, 2)
	assert.Equal(t, "site-1", mappings["example1.com"])
	assert.Equal(t, "site-2", mappings["example2.com"])
}

func TestDomainResolver_ListMappings_Empty(t *testing.T) {
	mockRedis := NewMockRedisClientForDomainResolver()

	resolver := NewDomainResolver(mockRedis)
	mappings, err := resolver.ListMappings()

	assert.Nil(t, err)
	assert.Empty(t, mappings)
}

func TestDomainResolver_ListMappings_RedisError(t *testing.T) {
	mockRedis := &MockRedisClientWithError{}
	resolver := NewDomainResolver(mockRedis)
	mappings, err := resolver.ListMappings()

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "redis error")
	assert.Nil(t, mappings)
}

func TestDomainResolver_ListMappings_SkipMissingEntries(t *testing.T) {
	mockRedis := NewMockRedisClientForDomainResolver()
	mockRedis.sets["domains"] = map[string]bool{
		"example1.com": true,
		"example2.com": true, // 这个没有对应的 siteID
	}
	mockRedis.data["domain:example1.com"] = "site-1"
	// example2.com 没有对应的值

	resolver := NewDomainResolver(mockRedis)
	mappings, err := resolver.ListMappings()

	assert.Nil(t, err)
	assert.Len(t, mappings, 1)
	assert.Equal(t, "site-1", mappings["example1.com"])
	assert.NotContains(t, mappings, "example2.com")
}

func TestDomainResolverInterface(t *testing.T) {
	// 验证 domainResolver 实现了 DomainResolver 接口
	var _ DomainResolver = (*domainResolver)(nil)
}
