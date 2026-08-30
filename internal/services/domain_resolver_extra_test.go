package services

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errRedisBoom 通用模拟 Redis 错误
var errRedisBoom = errors.New("redis boom")

// MockRedisClientSelective 可按 key / 方法粒度注入错误的 Redis 模拟客户端
type MockRedisClientSelective struct {
	MockRedisClientForDomainResolver
	failGet        map[string]bool
	failSet        bool
	failDel        bool
	failSetAdd     bool
	failSetMembers bool
	failSetRemove  bool
}

func (m *MockRedisClientSelective) Get(key string) (string, error) {
	if m.failGet[key] {
		return "", errRedisBoom
	}
	return m.MockRedisClientForDomainResolver.Get(key)
}

func (m *MockRedisClientSelective) Set(key string, value interface{}, expiration time.Duration) error {
	if m.failSet {
		return errRedisBoom
	}
	return m.MockRedisClientForDomainResolver.Set(key, value, expiration)
}

func (m *MockRedisClientSelective) Del(key string) error {
	if m.failDel {
		return errRedisBoom
	}
	return m.MockRedisClientForDomainResolver.Del(key)
}

func (m *MockRedisClientSelective) SetAdd(key string, members ...interface{}) error {
	if m.failSetAdd {
		return errRedisBoom
	}
	return m.MockRedisClientForDomainResolver.SetAdd(key, members...)
}

func (m *MockRedisClientSelective) SetMembers(key string) ([]string, error) {
	if m.failSetMembers {
		return nil, errRedisBoom
	}
	return m.MockRedisClientForDomainResolver.SetMembers(key)
}

func (m *MockRedisClientSelective) SetRemove(key string, members ...interface{}) error {
	if m.failSetRemove {
		return errRedisBoom
	}
	return m.MockRedisClientForDomainResolver.SetRemove(key, members...)
}

func TestNewDomainResolverWithClient(t *testing.T) {
	client := newTestRedisClient(t)

	resolver := NewDomainResolverWithClient(client)
	assert.NotNil(t, resolver)

	// 真实 Redis 上的端到端解析
	domain := fmt.Sprintf("withclient-%d.example.com", time.Now().UnixNano())
	require.NoError(t, resolver.AddMapping(domain, "site-rc"))

	siteID, err := resolver.Resolve(domain)
	require.NoError(t, err)
	assert.Equal(t, "site-rc", siteID)

	require.NoError(t, resolver.RemoveMapping(domain))
	siteID, err = resolver.Resolve(domain)
	require.NoError(t, err)
	assert.Empty(t, siteID)
}

func TestDomainResolver_Resolve_WildcardGetErrorContinues(t *testing.T) {
	mock := &MockRedisClientSelective{
		MockRedisClientForDomainResolver: *NewMockRedisClientForDomainResolver(),
		failGet: map[string]bool{
			"domain:*.sub.example.com": true, // 第一级通配符查询失败
		},
	}
	mock.data["domain:*.example.com"] = "site-deep"

	resolver := NewDomainResolver(mock)

	// 通配符查询出错时应 continue 到下一级，最终命中更深层通配符
	siteID, err := resolver.Resolve("deep.sub.example.com")
	require.NoError(t, err)
	assert.Equal(t, "site-deep", siteID)
}

func TestDomainResolver_AddMapping_SetAddError(t *testing.T) {
	mock := &MockRedisClientSelective{
		MockRedisClientForDomainResolver: *NewMockRedisClientForDomainResolver(),
		failSetAdd:                       true,
	}

	resolver := NewDomainResolver(mock)
	err := resolver.AddMapping("example.com", "site-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to add domain to list")
}

func TestDomainResolver_RemoveMapping_SetRemoveError(t *testing.T) {
	mock := &MockRedisClientSelective{
		MockRedisClientForDomainResolver: *NewMockRedisClientForDomainResolver(),
		failSetRemove:                    true,
	}

	resolver := NewDomainResolver(mock)
	err := resolver.RemoveMapping("example.com")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove domain from list")
}

func TestDomainResolver_ListMappings_GetErrorSkipsDomain(t *testing.T) {
	mock := &MockRedisClientSelective{
		MockRedisClientForDomainResolver: *NewMockRedisClientForDomainResolver(),
		failGet:                          map[string]bool{"domain:bad.com": true},
	}
	mock.data["domain:good.com"] = "site-good"
	mock.sets["domains"] = map[string]bool{"good.com": true, "bad.com": true}

	resolver := NewDomainResolver(mock)
	mappings, err := resolver.ListMappings()

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"good.com": "site-good"}, mappings)
}
