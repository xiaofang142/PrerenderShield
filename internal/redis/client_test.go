package redis

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRedisClient 是redis.Client的mock实现
type MockRedisClient struct {
	mock.Mock
}

// NewMockRedisClient 创建一个新的MockRedisClient
func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{}
}

// TestNewClient 测试创建Redis客户端
func TestNewClient(t *testing.T) {
	// 这个测试需要实际的Redis服务器，我们暂时跳过
	t.Skip("Skipping test that requires actual Redis server")

	// 测试使用默认端口连接
	client, err := NewClient("localhost:6379")
	assert.NoError(t, err)
	assert.NotNil(t, client)

	if client != nil {
		defer client.Close()
	}

	// 测试使用URL格式连接
	client, err = NewClient("redis://localhost:6379/0")
	assert.NoError(t, err)
	assert.NotNil(t, client)

	if client != nil {
		defer client.Close()
	}
}

// TestAddAndRemoveURL 测试添加和移除URL
func TestAddAndRemoveURL(t *testing.T) {
	// 这个测试需要实际的Redis服务器，我们暂时跳过
	t.Skip("Skipping test that requires actual Redis server")

	client, err := NewClient("localhost:6379")
	assert.NoError(t, err)
	assert.NotNil(t, client)
	defer client.Close()

	// 清空测试数据
	err = client.ClearURLs("test-site")
	assert.NoError(t, err)

	// 测试添加URL
	err = client.AddURL("test-site", "http://example.com/page1")
	assert.NoError(t, err)

	// 测试获取URL数量
	count, err := client.GetURLCount("test-site")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// 测试获取URLs
	urls, err := client.GetURLs("test-site")
	assert.NoError(t, err)
	assert.Len(t, urls, 1)
	assert.Contains(t, urls, "http://example.com/page1")

	// 测试移除URL
	err = client.RemoveURL("test-site", "http://example.com/page1")
	assert.NoError(t, err)

	// 测试获取URL数量（应该为0）
	count, err = client.GetURLCount("test-site")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

// TestSetAndGetURLPreheatStatus 测试设置和获取URL预热状态
func TestSetAndGetURLPreheatStatus(t *testing.T) {
	// 这个测试需要实际的Redis服务器，我们暂时跳过
	t.Skip("Skipping test that requires actual Redis server")

	client, err := NewClient("localhost:6379")
	assert.NoError(t, err)
	assert.NotNil(t, client)
	defer client.Close()

	// 测试设置URL预热状态
	err = client.SetURLPreheatStatus("test-site", "http://example.com/page1", "cached", 1024)
	assert.NoError(t, err)

	// 测试获取URL预热状态
	status, err := client.GetURLPreheatStatus("test-site", "http://example.com/page1")
	assert.NoError(t, err)
	assert.Equal(t, "cached", status["status"])
	assert.Equal(t, "1024", status["cache_size"])
}

// TestSetAndGetSiteStats 测试设置和获取站点统计数据
func TestSetAndGetSiteStats(t *testing.T) {
	// 这个测试需要实际的Redis服务器，我们暂时跳过
	t.Skip("Skipping test that requires actual Redis server")

	client, err := NewClient("localhost:6379")
	assert.NoError(t, err)
	assert.NotNil(t, client)
	defer client.Close()

	// 测试设置站点统计数据
	err = client.SetSiteStats("test-site", map[string]interface{}{
		"total_urls":   100,
		"cached_urls":  50,
		"last_updated": time.Now().Unix(),
	})
	assert.NoError(t, err)

	// 测试获取站点统计数据
	stats, err := client.GetSiteStats("test-site")
	assert.NoError(t, err)
	assert.Equal(t, "100", stats["total_urls"])
	assert.Equal(t, "50", stats["cached_urls"])
}

// TestPreheatRunningStatus 测试预热任务运行状态
func TestPreheatRunningStatus(t *testing.T) {
	// 这个测试需要实际的Redis服务器，我们暂时跳过
	t.Skip("Skipping test that requires actual Redis server")

	client, err := NewClient("localhost:6379")
	assert.NoError(t, err)
	assert.NotNil(t, client)
	defer client.Close()

	// 测试设置预热运行状态
	err = client.SetPreheatRunning("test-site", true)
	assert.NoError(t, err)

	// 测试获取预热运行状态
	running, err := client.IsPreheatRunning("test-site")
	assert.NoError(t, err)
	assert.True(t, running)

	// 测试设置预热停止状态
	err = client.SetPreheatRunning("test-site", false)
	assert.NoError(t, err)

	// 测试获取预热停止状态
	running, err = client.IsPreheatRunning("test-site")
	assert.NoError(t, err)
	assert.False(t, running)
}

// TestSaveAndGetUser 测试保存和获取用户信息
func TestSaveAndGetUser(t *testing.T) {
	// 这个测试需要实际的Redis服务器，我们暂时跳过
	t.Skip("Skipping test that requires actual Redis server")

	client, err := NewClient("localhost:6379")
	assert.NoError(t, err)
	assert.NotNil(t, client)
	defer client.Close()

	// 测试保存用户信息
	err = client.SaveUser("user1", "testuser", "password123")
	assert.NoError(t, err)

	// 测试获取用户信息
	user, err := client.GetUser("user1")
	assert.NoError(t, err)
	assert.Equal(t, "user1", user["id"])
	assert.Equal(t, "testuser", user["username"])
	assert.Equal(t, "password123", user["password"])

	// 测试通过用户名获取用户ID
	userID, err := client.GetUserByUsername("testuser")
	assert.NoError(t, err)
	assert.Equal(t, "user1", userID)
}

// TestGetAllUsers 测试获取所有用户
func TestGetAllUsers(t *testing.T) {
	// 这个测试需要实际的Redis服务器，我们暂时跳过
	t.Skip("Skipping test that requires actual Redis server")

	client, err := NewClient("localhost:6379")
	assert.NoError(t, err)
	assert.NotNil(t, client)
	defer client.Close()

	// 测试获取所有用户
	users, err := client.GetAllUsers()
	assert.NoError(t, err)
	// 这里我们只测试是否能正常获取，不验证具体数量，因为可能有其他测试数据
	assert.IsType(t, []string{}, users)
}
