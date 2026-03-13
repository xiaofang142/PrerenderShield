package logging

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestCrawlerLog_Struct 测试 CrawlerLog 结构体
func TestCrawlerLog_Struct(t *testing.T) {
	log := CrawlerLog{
		ID:          "test-id",
		Site:        "example.com",
		IP:          "192.168.1.1",
		Time:        time.Now(),
		HitCache:    true,
		Route:       "/api/test",
		UA:          "Mozilla/5.0",
		Status:      200,
		Method:      "GET",
		CacheTTL:    3600,
		RenderTime:  0.5,
		Country:     "US",
		CountryCode: "US",
		City:        "New York",
		Latitude:    40.7128,
		Longitude:   -74.0060,
		Washed:      true,
	}

	assert.Equal(t, "test-id", log.ID)
	assert.Equal(t, "example.com", log.Site)
	assert.Equal(t, "192.168.1.1", log.IP)
	assert.True(t, log.HitCache)
	assert.Equal(t, "/api/test", log.Route)
	assert.Equal(t, 200, log.Status)
	assert.Equal(t, "GET", log.Method)
	assert.Equal(t, 3600, log.CacheTTL)
	assert.Equal(t, 0.5, log.RenderTime)
	assert.True(t, log.Washed)
}

// TestCrawlerLog_JSON 测试 CrawlerLog 的 JSON 序列化
func TestCrawlerLog_JSON(t *testing.T) {
	log := CrawlerLog{
		ID:       "test-id",
		Site:     "example.com",
		IP:       "192.168.1.1",
		Time:     time.Now(),
		HitCache: true,
		Route:    "/api/test",
		Status:   200,
		Method:   "GET",
		Washed:   true,
	}

	data, err := json.Marshal(log)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "test-id")
	assert.Contains(t, string(data), "example.com")

	// 测试反序列化
	var decoded CrawlerLog
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, log.ID, decoded.ID)
}

// TestVisitLog_Struct 测试 VisitLog 结构体
func TestVisitLog_Struct(t *testing.T) {
	log := VisitLog{
		ID:          "test-id",
		Site:        "example.com",
		IP:          "192.168.1.1",
		Time:        time.Now(),
		Method:      "POST",
		URL:         "/api/users",
		Status:      201,
		UA:          "curl/7.68.0",
		Duration:    0.25,
		Referer:     "https://example.com",
		Country:     "CN",
		CountryCode: "CN",
		City:        "Beijing",
		Latitude:    39.9042,
		Longitude:   116.4074,
		Washed:      false,
	}

	assert.Equal(t, "test-id", log.ID)
	assert.Equal(t, "POST", log.Method)
	assert.Equal(t, "/api/users", log.URL)
	assert.Equal(t, 201, log.Status)
	assert.Equal(t, 0.25, log.Duration)
	assert.False(t, log.Washed)
}

// TestVisitLog_JSON 测试 VisitLog 的 JSON 序列化
func TestVisitLog_JSON(t *testing.T) {
	log := VisitLog{
		ID:       "test-id",
		Site:     "example.com",
		IP:       "192.168.1.1",
		Time:     time.Now(),
		Method:   "GET",
		URL:      "/index.html",
		Status:   200,
		Duration: 0.1,
		Washed:   true,
	}

	data, err := json.Marshal(log)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "/index.html")

	// 测试反序列化
	var decoded VisitLog
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, log.ID, decoded.ID)
}

// TestCrawlerLogManager_NilRedis 测试 CrawlerLogManager 创建时 Redis 不可用的情况
func TestCrawlerLogManager_NilRedis(t *testing.T) {
	// 使用无效的 Redis URL，应该不会 panic
	manager := NewCrawlerLogManager("invalid://redis:6379")
	assert.NotNil(t, manager)
}

// TestVisitLogManager_NilRedis 测试 VisitLogManager 创建时 Redis 不可用的情况
func TestVisitLogManager_NilRedis(t *testing.T) {
	// 使用无效的 Redis URL，应该不会 panic
	manager := NewVisitLogManager("invalid://redis:6379")
	assert.NotNil(t, manager)
}

// TestCrawlerLogManager_EmptyRedisURL 测试 CrawlerLogManager 使用空 Redis URL
func TestCrawlerLogManager_EmptyRedisURL(t *testing.T) {
	manager := NewCrawlerLogManager("")
	assert.NotNil(t, manager)
}

// TestVisitLogManager_EmptyRedisURL 测试 VisitLogManager 使用空 Redis URL
func TestVisitLogManager_EmptyRedisURL(t *testing.T) {
	manager := NewVisitLogManager("")
	assert.NotNil(t, manager)
}

// TestCrawlerLogManager_InvalidRedisURL 测试 CrawlerLogManager 使用无效的 Redis URL
func TestCrawlerLogManager_InvalidRedisURL(t *testing.T) {
	manager := NewCrawlerLogManager("://invalid")
	assert.NotNil(t, manager)
}

// TestVisitLogManager_InvalidRedisURL 测试 VisitLogManager 使用无效的 Redis URL
func TestVisitLogManager_InvalidRedisURL(t *testing.T) {
	manager := NewVisitLogManager("://invalid")
	assert.NotNil(t, manager)
}

// TestCrawlerLogManager_WithPort 测试 CrawlerLogManager 使用带端口的 Redis 地址
func TestCrawlerLogManager_WithPort(t *testing.T) {
	manager := NewCrawlerLogManager("localhost:6380")
	assert.NotNil(t, manager)
}

// TestVisitLogManager_WithPort 测试 VisitLogManager 使用带端口的 Redis 地址
func TestVisitLogManager_WithPort(t *testing.T) {
	manager := NewVisitLogManager("localhost:6380")
	assert.NotNil(t, manager)
}

// TestCrawlerLogManager_WithPassword 测试 CrawlerLogManager 使用带密码的 Redis URL
func TestCrawlerLogManager_WithPassword(t *testing.T) {
	manager := NewCrawlerLogManager("redis://user:password@localhost:6379/1")
	assert.NotNil(t, manager)
}

// TestVisitLogManager_WithPassword 测试 VisitLogManager 使用带密码的 Redis URL
func TestVisitLogManager_WithPassword(t *testing.T) {
	manager := NewVisitLogManager("redis://user:password@localhost:6379/1")
	assert.NotNil(t, manager)
}

// TestCrawlerLogManager_WithDB 测试 CrawlerLogManager 使用指定数据库
func TestCrawlerLogManager_WithDB(t *testing.T) {
	manager := NewCrawlerLogManager("redis://localhost:6379/5")
	assert.NotNil(t, manager)
}

// TestVisitLogManager_WithDB 测试 VisitLogManager 使用指定数据库
func TestVisitLogManager_WithDB(t *testing.T) {
	manager := NewVisitLogManager("redis://localhost:6379/5")
	assert.NotNil(t, manager)
}
