package repository

import (
	"testing"
	"time"

	"prerender-shield/internal/models"
	"prerender-shield/internal/redis"

	"github.com/stretchr/testify/assert"
)

// TestParseInt64 测试 parseInt64 函数
func TestParseInt64(t *testing.T) {
	tests := []struct {
		input  string
		expect int64
	}{
		{"123", 123},
		{"0", 0},
		{"999999", 999999},
		{"", 0},         // 空字符串
		{"abc", 0},      // 无效输入
		{"-123", -123},  // 负数
	}

	for _, tt := range tests {
		result := parseInt64(tt.input)
		assert.Equal(t, tt.expect, result, "input: %s", tt.input)
	}
}

// TestSiteRepository 测试 SiteRepository 接口
func TestSiteRepository_Interface(t *testing.T) {
	// 验证 siteRepository 实现 SiteRepository 接口
	var repo SiteRepository
	mockClient := &redis.Client{}
	repo = NewSiteRepository(mockClient)
	assert.NotNil(t, repo)
}

// TestSiteRepository_Create 测试 Create 方法
func TestSiteRepository_Create(t *testing.T) {
	t.Skip("Requires Redis connection")

	mockClient := &redis.Client{}
	repo := NewSiteRepository(mockClient)

	site := &models.Site{
		Domain:  "example.com",
		Name:    "Test Site",
		Enabled: true,
	}

	err := repo.Create(site)
	assert.NoError(t, err)
	assert.NotEmpty(t, site.ID)
}

// TestSiteRepository_Get 测试 Get 方法
func TestSiteRepository_Get(t *testing.T) {
	t.Skip("Requires Redis connection")

	mockClient := &redis.Client{}
	repo := NewSiteRepository(mockClient)

	site, err := repo.Get("test-id")
	assert.NoError(t, err)
	assert.Nil(t, site) // 不存在的站点
}

// TestSiteRepository_Update 测试 Update 方法
func TestSiteRepository_Update(t *testing.T) {
	t.Skip("Requires Redis connection")
}

// TestSiteRepository_Delete 测试 Delete 方法
func TestSiteRepository_Delete(t *testing.T) {
	t.Skip("Requires Redis connection")
}

// TestSiteRepository_List 测试 List 方法
func TestSiteRepository_List(t *testing.T) {
	t.Skip("Requires Redis connection")
}

// TestSiteRepository_GetByDomain 测试 GetByDomain 方法
func TestSiteRepository_GetByDomain(t *testing.T) {
	t.Skip("Requires Redis connection")
}

// TestWafRepository 测试 WafRepository
func TestWafRepository_Interface(t *testing.T) {
	mockClient := &redis.Client{}
	repo := NewWafRepository(mockClient)
	assert.NotNil(t, repo)
}

// TestWafRepository_GetWafConfigBySiteID 测试 GetWafConfigBySiteID
func TestWafRepository_GetWafConfigBySiteID(t *testing.T) {
	t.Skip("Requires Redis connection")
}

// TestWafRepository_CreateWafConfig 测试 CreateWafConfig
func TestWafRepository_CreateWafConfig(t *testing.T) {
	t.Skip("Requires Redis connection")
}

// TestWafRepository_UpdateWafConfig 测试 UpdateWafConfig
func TestWafRepository_UpdateWafConfig(t *testing.T) {
	t.Skip("Requires Redis connection")
}

// TestWafRepository_UpdateBlockedCountries 测试 UpdateBlockedCountries
func TestWafRepository_UpdateBlockedCountries(t *testing.T) {
	t.Skip("Requires Redis connection")
}

// TestWafRepository_UpdateIPWhitelist 测试 UpdateIPWhitelist
func TestWafRepository_UpdateIPWhitelist(t *testing.T) {
	t.Skip("Requires Redis connection")
}

// TestWafRepository_UpdateIPBlacklist 测试 UpdateIPBlacklist
func TestWafRepository_UpdateIPBlacklist(t *testing.T) {
	t.Skip("Requires Redis connection")
}

// TestWafRepository_GetAccessLogs 测试 GetAccessLogs
func TestWafRepository_GetAccessLogs(t *testing.T) {
	t.Skip("Requires Redis connection")
}

// TestWafRepository_GetAttackLogs 测试 GetAttackLogs
func TestWafRepository_GetAttackLogs(t *testing.T) {
	t.Skip("Requires Redis connection")
}

// TestWafRepository_AddIPToWhitelist 测试 AddIPToWhitelist
func TestWafRepository_AddIPToWhitelist(t *testing.T) {
	t.Skip("Requires Redis connection")
}

// TestWafRepository_AddIPToBlacklist 测试 AddIPToBlacklist
func TestWafRepository_AddIPToBlacklist(t *testing.T) {
	t.Skip("Requires Redis connection")
}

// TestWafRepository_CreateAccessLog 测试 CreateAccessLog
func TestWafRepository_CreateAccessLog(t *testing.T) {
	t.Skip("Requires Redis connection")
}

// TestWafRepository_GetGlobalStats 测试 GetGlobalStats
func TestWafRepository_GetGlobalStats(t *testing.T) {
	t.Skip("Requires Redis connection")
}

// TestWafRepository_GetTrafficStats 测试 GetTrafficStats
func TestWafRepository_GetTrafficStats(t *testing.T) {
	t.Skip("Requires Redis connection")
}

// TestWafRepositoryInMemory 测试内存实现
func TestWafRepositoryInMemory_New(t *testing.T) {
	repo := NewWafRepositoryInMemory()
	assert.NotNil(t, repo)
	assert.NotNil(t, repo.configs)
	assert.NotNil(t, repo.accessLogs)
	assert.NotNil(t, repo.attackLogs)
	assert.NotNil(t, repo.ipWhitelists)
	assert.NotNil(t, repo.ipBlacklists)
}

// TestWafRepositoryInMemory_GetWafConfigBySiteID 测试内存实现的 GetWafConfigBySiteID
func TestWafRepositoryInMemory_GetWafConfigBySiteID(t *testing.T) {
	repo := NewWafRepositoryInMemory()

	// 测试不存在的配置
	config, err := repo.GetWafConfigBySiteID("site1")
	assert.NoError(t, err)
	assert.Nil(t, config)

	// 测试存在的配置
	expectedConfig := &models.WafConfig{
		ID:     "config1",
		SiteID: "site1",
		Enabled: true,
	}
	repo.configs["site1"] = expectedConfig

	config, err = repo.GetWafConfigBySiteID("site1")
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "site1", config.SiteID)
}

// TestWafRepositoryInMemory_UpdateWafConfig 测试内存实现的 UpdateWafConfig
func TestWafRepositoryInMemory_UpdateWafConfig(t *testing.T) {
	repo := NewWafRepositoryInMemory()

	config := &models.WafConfig{
		ID:     "config1",
		SiteID: "site1",
		Enabled: true,
	}

	err := repo.UpdateWafConfig(config)
	assert.NoError(t, err)

	// 验证已保存
	savedConfig, err := repo.GetWafConfigBySiteID("site1")
	assert.NoError(t, err)
	assert.NotNil(t, savedConfig)
	assert.Equal(t, "config1", savedConfig.ID)
}

// TestWafRepositoryInMemory_GetAccessLogs 测试内存实现的 GetAccessLogs
func TestWafRepositoryInMemory_GetAccessLogs(t *testing.T) {
	repo := NewWafRepositoryInMemory()

	// 测试空日志
	logs, total, err := repo.GetAccessLogs("site1", 1, 10)
	assert.NoError(t, err)
	assert.Empty(t, logs)
	assert.Equal(t, int64(0), total)

	// 添加测试日志
	repo.accessLogs["site1"] = []models.AccessLog{
		{ID: "log1", SiteID: "site1", IPAddress: "1.2.3.4"},
		{ID: "log2", SiteID: "site1", IPAddress: "5.6.7.8"},
		{ID: "log3", SiteID: "site1", IPAddress: "9.10.11.12"},
	}

	// 测试分页
	logs, total, err = repo.GetAccessLogs("site1", 1, 2)
	assert.NoError(t, err)
	assert.Len(t, logs, 2)
	assert.Equal(t, int64(3), total)

	// 测试第二页
	logs, total, err = repo.GetAccessLogs("site1", 2, 2)
	assert.NoError(t, err)
	assert.Len(t, logs, 1)
}

// TestWafRepositoryInMemory_GetAttackLogs 测试内存实现的 GetAttackLogs
func TestWafRepositoryInMemory_GetAttackLogs(t *testing.T) {
	repo := NewWafRepositoryInMemory()

	// 测试空日志
	logs, total, err := repo.GetAttackLogs("site1", 1, 10)
	assert.NoError(t, err)
	assert.Empty(t, logs)
	assert.Equal(t, int64(0), total)

	// 添加测试攻击日志
	repo.attackLogs["site1"] = []models.AccessLog{
		{ID: "attack1", SiteID: "site1", Action: "block"},
		{ID: "attack2", SiteID: "site1", Action: "block"},
	}

	logs, total, err = repo.GetAttackLogs("site1", 1, 10)
	assert.NoError(t, err)
	assert.Len(t, logs, 2)
	assert.Equal(t, int64(2), total)
}

// TestWafRepositoryInMemory_AddIPToWhitelist 测试内存实现的 AddIPToWhitelist
func TestWafRepositoryInMemory_AddIPToWhitelist(t *testing.T) {
	repo := NewWafRepositoryInMemory()

	err := repo.AddIPToWhitelist("site1", "1.2.3.4")
	assert.NoError(t, err)

	assert.Len(t, repo.ipWhitelists["site1"], 1)
	assert.Equal(t, "1.2.3.4", repo.ipWhitelists["site1"][0].IPAddress)
}

// TestWafRepositoryInMemory_AddIPToBlacklist 测试内存实现的 AddIPToBlacklist
func TestWafRepositoryInMemory_AddIPToBlacklist(t *testing.T) {
	repo := NewWafRepositoryInMemory()

	err := repo.AddIPToBlacklist("site1", "5.6.7.8")
	assert.NoError(t, err)

	assert.Len(t, repo.ipBlacklists["site1"], 1)
	assert.Equal(t, "5.6.7.8", repo.ipBlacklists["site1"][0].IPAddress)
}

// TestWafRepositoryInMemory_AddIPToWhitelist_RemovesFromBlacklist 测试白名单移除黑名单
func TestWafRepositoryInMemory_AddIPToWhitelist_RemovesFromBlacklist(t *testing.T) {
	repo := NewWafRepositoryInMemory()

	// 先添加到黑名单
	repo.ipBlacklists["site1"] = []models.IPBlacklist{
		{IPAddress: "1.2.3.4"},
	}

	// 再添加到白名单
	err := repo.AddIPToWhitelist("site1", "1.2.3.4")
	assert.NoError(t, err)

	// 验证已从黑名单移除
	assert.Empty(t, repo.ipBlacklists["site1"])
}

// TestWafRepositoryInMemory_AddIPToBlacklist_RemovesFromWhitelist 测试黑名单移除白名单
func TestWafRepositoryInMemory_AddIPToBlacklist_RemovesFromWhitelist(t *testing.T) {
	repo := NewWafRepositoryInMemory()

	// 先添加到白名单
	repo.ipWhitelists["site1"] = []models.IPWhitelist{
		{IPAddress: "5.6.7.8"},
	}

	// 再添加到黑名单
	err := repo.AddIPToBlacklist("site1", "5.6.7.8")
	assert.NoError(t, err)

	// 验证已从白名单移除
	assert.Empty(t, repo.ipWhitelists["site1"])
}

// TestWafRepositoryInMemory_ConcurrentAccess 测试并发访问
func TestWafRepositoryInMemory_ConcurrentAccess(t *testing.T) {
	repo := NewWafRepositoryInMemory()

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			siteID := "site" + string(rune(id%2+'0'))
			repo.AddIPToWhitelist(siteID, "1.2.3."+string(rune(id+'0')))
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证并发访问没有导致 panic
	assert.NotNil(t, repo.ipWhitelists)
}

// TestWafStats 测试 WafStats 结构
func TestWafStats(t *testing.T) {
	stats := &WafStats{
		TotalRequests:   100,
		BlockedRequests: 10,
		AttackRequests:  10,
	}

	assert.Equal(t, int64(100), stats.TotalRequests)
	assert.Equal(t, int64(10), stats.BlockedRequests)
	assert.Equal(t, int64(10), stats.AttackRequests)
}

// TestModels 测试模型结构
func TestSiteModel(t *testing.T) {
	site := &models.Site{
		ID:      "site1",
		Domain:  "example.com",
		Name:    "Test Site",
		Enabled: true,
	}

	assert.Equal(t, "site1", site.ID)
	assert.Equal(t, "example.com", site.Domain)
	assert.Equal(t, "Test Site", site.Name)
	assert.True(t, site.Enabled)
}

func TestWafConfigModel(t *testing.T) {
	config := &models.WafConfig{
		ID:              "config1",
		SiteID:          "site1",
		RateLimitCount:  100,
		RateLimitWindow: 60,
		Enabled:         true,
	}

	assert.Equal(t, "config1", config.ID)
	assert.Equal(t, "site1", config.SiteID)
	assert.Equal(t, 100, config.RateLimitCount)
	assert.Equal(t, 60, config.RateLimitWindow)
	assert.True(t, config.Enabled)
}

func TestAccessLogModel(t *testing.T) {
	log := &models.AccessLog{
		ID:          "log1",
		SiteID:      "site1",
		IPAddress:   "1.2.3.4",
		Country:     "US",
		City:        "New York",
		Method:      "GET",
		RequestPath: "/api/test",
		StatusCode:  200,
		Action:      "allow",
		CreatedAt:   time.Now(),
	}

	assert.Equal(t, "log1", log.ID)
	assert.Equal(t, "site1", log.SiteID)
	assert.Equal(t, "1.2.3.4", log.IPAddress)
	assert.Equal(t, "GET", log.Method)
	assert.Equal(t, 200, log.StatusCode)
	assert.Equal(t, "allow", log.Action)
}
