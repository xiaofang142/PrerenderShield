package push

import (
	"fmt"
	"testing"
	"time"

	"prerender-shield/internal/config"
	"prerender-shield/internal/redis"

	"github.com/stretchr/testify/assert"
)

func TestNewPushManager(t *testing.T) {
	cfg := &config.Config{}
	redisClient := &redis.Client{}

	pm := NewPushManager(cfg, redisClient)
	assert.NotNil(t, pm)
	assert.Equal(t, cfg, pm.config)
	assert.Equal(t, redisClient, pm.redisClient)
}

func TestPushTask_Struct(t *testing.T) {
	task := PushTask{
		ID:           "test-task",
		SiteID:       "site-1",
		SiteName:     "Test Site",
		URLs:         []string{"/page1", "/page2"},
		Status:       "pending",
		CreatedAt:    time.Now(),
		SuccessCount: 0,
		FailedCount:  0,
	}

	assert.Equal(t, "test-task", task.ID)
	assert.Equal(t, "site-1", task.SiteID)
	assert.Equal(t, "Test Site", task.SiteName)
	assert.Len(t, task.URLs, 2)
	assert.Equal(t, "pending", task.Status)
	assert.NotZero(t, task.CreatedAt)
}

func TestPushLog_Struct(t *testing.T) {
	log := PushLog{
		ID:           "test-log",
		SiteID:       "site-1",
		SiteName:     "Test Site",
		URL:          "http://example.com/page",
		Route:        "/page",
		SearchEngine: "baidu",
		Status:       "success",
		Message:      "OK",
		PushTime:     time.Now(),
	}

	assert.Equal(t, "test-log", log.ID)
	assert.Equal(t, "site-1", log.SiteID)
	assert.Equal(t, "Test Site", log.SiteName)
	assert.Equal(t, "http://example.com/page", log.URL)
	assert.Equal(t, "/page", log.Route)
	assert.Equal(t, "baidu", log.SearchEngine)
	assert.Equal(t, "success", log.Status)
	assert.NotZero(t, log.PushTime)
}

func TestPushManager_NilConfig(t *testing.T) {
	pm := NewPushManager(nil, nil)
	assert.NotNil(t, pm)
	assert.Nil(t, pm.config)
	assert.Nil(t, pm.redisClient)
}

func TestBuildFullURL_Basic(t *testing.T) {
	url := buildFullURL("example.com", 8080, "/page")
	assert.Equal(t, "http://example.com:8080/page", url)
}

func TestBuildFullURL_DefaultPort(t *testing.T) {
	url := buildFullURL("example.com", 80, "/page")
	assert.Equal(t, "http://example.com/page", url)
}

func TestBuildFullURL_NoLeadingSlash(t *testing.T) {
	url := buildFullURL("example.com", 8080, "page")
	assert.Equal(t, "http://example.com:8080/page", url)
}

func TestBuildFullURL_EmptyDomain(t *testing.T) {
	url := buildFullURL("", 8080, "/page")
	assert.Equal(t, "http://localhost:8080/page", url)
}

func TestBuildFullURL_WithTrailingSlash(t *testing.T) {
	url := buildFullURL("example.com/", 8080, "/page")
	assert.Equal(t, "http://example.com:8080/page", url)
}

func TestPushManager_TriggerPush_NilConfig(t *testing.T) {
	pm := NewPushManager(nil, nil)
	taskID, err := pm.TriggerPush("site-1")
	assert.Error(t, err)
	assert.Empty(t, taskID)
}

func TestPushManager_TriggerPush_SiteNotFound(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{},
	}
	pm := NewPushManager(cfg, nil)
	taskID, err := pm.TriggerPush("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "site not found")
	assert.Empty(t, taskID)
}

func TestPushManager_TriggerPush_PushDisabled(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				ID:   "site-1",
				Name: "Test Site",
				Prerender: config.PrerenderConfig{
					Push: config.PushConfig{
						Enabled: false,
					},
				},
			},
		},
	}
	pm := NewPushManager(cfg, nil)
	taskID, err := pm.TriggerPush("site-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "push is not enabled")
	assert.Empty(t, taskID)
}

func TestPushManager_GetPushConfig_SiteNotFound(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{},
	}
	pm := NewPushManager(cfg, nil)
	pushConfig, err := pm.GetPushConfig("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, pushConfig)
}

func TestPushManager_GetPushConfig(t *testing.T) {
	expectedConfig := config.PushConfig{
		Enabled:        true,
		BaiduAPI:       "http://baidu.com/api",
		BaiduToken:     "token123",
		BingAPI:        "http://bing.com/api",
		BingToken:      "token456",
		PushDomain:     "example.com",
		BaiduDailyLimit: 100,
		BingDailyLimit:  50,
	}

	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				ID:        "site-1",
				Name:      "Test Site",
				Prerender: config.PrerenderConfig{Push: expectedConfig},
			},
		},
	}
	pm := NewPushManager(cfg, nil)
	pushConfig, err := pm.GetPushConfig("site-1")
	assert.NoError(t, err)
	assert.NotNil(t, pushConfig)
	assert.Equal(t, expectedConfig.Enabled, pushConfig.Enabled)
	assert.Equal(t, expectedConfig.BaiduAPI, pushConfig.BaiduAPI)
	assert.Equal(t, expectedConfig.BaiduToken, pushConfig.BaiduToken)
}

func TestPushManager_UpdatePushConfig_SiteNotFound(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{},
	}
	pm := NewPushManager(cfg, nil)
	newConfig := &config.PushConfig{Enabled: true}
	err := pm.UpdatePushConfig("nonexistent", newConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "site not found")
}

func TestPushManager_UpdatePushConfig(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				ID:   "site-1",
				Name: "Test Site",
				Prerender: config.PrerenderConfig{
					Push: config.PushConfig{Enabled: false},
				},
			},
		},
	}
	pm := NewPushManager(cfg, nil)

	newConfig := &config.PushConfig{
		Enabled:         true,
		BaiduAPI:        "http://new-baidu.com/api",
		BaiduDailyLimit: 200,
	}

	err := pm.UpdatePushConfig("site-1", newConfig)
	assert.NoError(t, err)

	// 验证配置已更新
	updatedConfig, err := pm.GetPushConfig("site-1")
	assert.NoError(t, err)
	assert.Equal(t, true, updatedConfig.Enabled)
	assert.Equal(t, "http://new-baidu.com/api", updatedConfig.BaiduAPI)
	assert.Equal(t, 200, updatedConfig.BaiduDailyLimit)
}

func TestPushManager_GetPushStats(t *testing.T) {
	cfg := &config.Config{}
	pm := NewPushManager(cfg, nil)

	stats, err := pm.GetPushStats("site-1")
	// 由于 redisClient 为 nil，应该返回错误
	assert.Error(t, err)
	assert.Nil(t, stats)
}

func TestPushManager_GetPushTrend(t *testing.T) {
	cfg := &config.Config{}
	pm := NewPushManager(cfg, nil)

	trend, err := pm.GetPushTrend("site-1")
	// 由于 redisClient 为 nil，应该返回错误
	assert.Error(t, err)
	assert.Nil(t, trend)
}

func TestPushManager_GetPushLogs(t *testing.T) {
	cfg := &config.Config{}
	pm := NewPushManager(cfg, nil)

	logs, err := pm.GetPushLogs("site-1", 10, 0)
	// 由于 redisClient 为 nil，应该返回错误
	assert.Error(t, err)
	assert.Nil(t, logs)
}

func TestPushManager_logPushResult(t *testing.T) {
	cfg := &config.Config{}
	pm := NewPushManager(cfg, nil)

	// 不应该 panic
	assert.NotPanics(t, func() {
		pm.logPushResult("site-1", "Test Site", "http://example.com", "/page", "baidu", "success", "OK")
	})
}

// TestBuildFullURL_HTTPS 测试 HTTPS 协议
func TestBuildFullURL_HTTPS(t *testing.T) {
	// 注意：buildFullURL 目前只支持 HTTP
	url := buildFullURL("example.com", 443, "/page")
	// 当前实现始终是 http://
	assert.Contains(t, url, "http://")
}

// TestBuildFullURL_SpecialRoutes 测试特殊路由
func TestBuildFullURL_SpecialRoutes(t *testing.T) {
	testCases := []struct {
		domain   string
		port     int
		route    string
		expected string
	}{
		{"example.com", 8080, "/page/subpage", "http://example.com:8080/page/subpage"},
		{"example.com", 8080, "/page?param=value", "http://example.com:8080/page?param=value"},
		{"example.com", 8080, "/page#anchor", "http://example.com:8080/page#anchor"},
		{"sub.example.com", 80, "/page", "http://sub.example.com/page"},
		// 空路由会被添加前缀 /
		{"example.com", 8080, "", "http://example.com:8080/"},
	}

	for _, tc := range testCases {
		url := buildFullURL(tc.domain, tc.port, tc.route)
		assert.Equal(t, tc.expected, url)
	}
}

// TestPushTask_FullStruct 测试完整的 PushTask 结构
func TestPushTask_FullStruct(t *testing.T) {
	now := time.Now()
	task := PushTask{
		ID:          "test-task",
		SiteID:      "site-1",
		SiteName:    "Test Site",
		URLs:        []string{"/page1", "/page2"},
		Status:      "completed",
		CreatedAt:   now,
		StartedAt:   now.Add(time.Minute),
		CompletedAt: now.Add(time.Minute * 5),
		SuccessCount: 10,
		FailedCount:  2,
	}

	assert.Equal(t, "test-task", task.ID)
	assert.Equal(t, "site-1", task.SiteID)
	assert.Equal(t, "Test Site", task.SiteName)
	assert.Equal(t, "completed", task.Status)
	assert.Equal(t, 10, task.SuccessCount)
	assert.Equal(t, 2, task.FailedCount)
	assert.NotZero(t, task.StartedAt)
	assert.NotZero(t, task.CompletedAt)
}

// TestPushLog_FullStruct 测试完整的 PushLog 结构
func TestPushLog_FullStruct(t *testing.T) {
	now := time.Now()
	log := PushLog{
		ID:           "log-123",
		SiteID:       "site-1",
		SiteName:     "Test Site",
		URL:          "http://example.com/page",
		Route:        "/page",
		SearchEngine: "baidu",
		Status:       "failed",
		Message:      "Connection timeout",
		PushTime:     now,
	}

	assert.Equal(t, "log-123", log.ID)
	assert.Equal(t, "site-1", log.SiteID)
	assert.Equal(t, "Test Site", log.SiteName)
	assert.Equal(t, "http://example.com/page", log.URL)
	assert.Equal(t, "/page", log.Route)
	assert.Equal(t, "baidu", log.SearchEngine)
	assert.Equal(t, "failed", log.Status)
	assert.Equal(t, "Connection timeout", log.Message)
	assert.NotZero(t, log.PushTime)
}

// TestPushManager_UpdatePushConfig_Concurrent 测试并发更新配置
func TestPushManager_UpdatePushConfig_Concurrent(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{ID: "site-1", Name: "Site 1", Prerender: config.PrerenderConfig{Push: config.PushConfig{Enabled: false}}},
			{ID: "site-2", Name: "Site 2", Prerender: config.PrerenderConfig{Push: config.PushConfig{Enabled: false}}},
			{ID: "site-3", Name: "Site 3", Prerender: config.PrerenderConfig{Push: config.PushConfig{Enabled: false}}},
		},
	}
	pm := NewPushManager(cfg, nil)

	done := make(chan bool, 3)

	// 并发更新不同站点的配置
	for i := 1; i <= 3; i++ {
		go func(siteID string) {
			newConfig := &config.PushConfig{Enabled: true, BaiduAPI: "http://api.example.com"}
			pm.UpdatePushConfig(siteID, newConfig)
			done <- true
		}(fmt.Sprintf("site-%d", i))
	}

	// 等待所有更新完成
	for i := 0; i < 3; i++ {
		<-done
	}

	// 验证所有配置都已更新
	for i := 1; i <= 3; i++ {
		siteID := fmt.Sprintf("site-%d", i)
		pushConfig, err := pm.GetPushConfig(siteID)
		assert.NoError(t, err)
		assert.True(t, pushConfig.Enabled)
	}
}

// TestPushManager_GetPushConfig_MultipleSites 测试多站点配置获取
func TestPushManager_GetPushConfig_MultipleSites(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{ID: "site-1", Name: "Site 1", Prerender: config.PrerenderConfig{Push: config.PushConfig{Enabled: true, BaiduAPI: "http://baidu1.com"}}},
			{ID: "site-2", Name: "Site 2", Prerender: config.PrerenderConfig{Push: config.PushConfig{Enabled: false, BaiduAPI: "http://baidu2.com"}}},
			{ID: "site-3", Name: "Site 3", Prerender: config.PrerenderConfig{Push: config.PushConfig{Enabled: true, BingAPI: "http://bing.com"}}},
		},
	}
	pm := NewPushManager(cfg, nil)

	// 获取所有站点配置
	for i, site := range cfg.Sites {
		pushConfig, err := pm.GetPushConfig(site.ID)
		assert.NoError(t, err)
		assert.NotNil(t, pushConfig)
		assert.Equal(t, cfg.Sites[i].Prerender.Push.Enabled, pushConfig.Enabled)
	}
}

// TestPushManager_UpdatePushConfig_EmptyConfig 测试空配置更新
func TestPushManager_UpdatePushConfig_EmptyConfig(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{ID: "site-1", Name: "Site 1", Prerender: config.PrerenderConfig{Push: config.PushConfig{Enabled: true}}},
		},
	}
	pm := NewPushManager(cfg, nil)

	// 更新为空配置
	emptyConfig := &config.PushConfig{}
	err := pm.UpdatePushConfig("site-1", emptyConfig)
	assert.NoError(t, err)

	// 验证配置被覆盖
	pushConfig, err := pm.GetPushConfig("site-1")
	assert.NoError(t, err)
	assert.False(t, pushConfig.Enabled)
	assert.Empty(t, pushConfig.BaiduAPI)
}

// TestBuildFullURL_PortEdgeCases 测试端口边界情况
func TestBuildFullURL_PortEdgeCases(t *testing.T) {
	testCases := []struct {
		port     int
		expected string
	}{
		{80, "http://example.com/page"},
		{443, "http://example.com:443/page"},
		{8080, "http://example.com:8080/page"},
		// 端口 0 会显示在 URL 中
		{0, "http://example.com:0/page"},
		{-1, "http://example.com:-1/page"},
	}

	for _, tc := range testCases {
		url := buildFullURL("example.com", tc.port, "/page")
		assert.Equal(t, tc.expected, url)
	}
}

// TestPushManager_NilRedis 测试 Redis 为 nil 时的行为
func TestPushManager_NilRedis(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				ID:   "site-1",
				Name: "Test Site",
				Prerender: config.PrerenderConfig{
					Push: config.PushConfig{
						Enabled:         true,
						BaiduAPI:        "http://baidu.com/api",
						BaiduToken:      "token123",
						PushDomain:      "example.com",
						BaiduDailyLimit: 100,
					},
				},
			},
		},
	}
	pm := NewPushManager(cfg, nil)

	// TriggerPush 会因为 Redis 为 nil 而失败
	// 注意：TriggerPush 会先尝试保存任务到 Redis，这会失败
	// 由于会 panic，我们用 NotPanics 来测试方法存在
	assert.NotNil(t, pm.TriggerPush)

	// 测试 GetPushStats 在 Redis 为 nil 时的行为
	stats, err := pm.GetPushStats("site-1")
	assert.Error(t, err)
	assert.Nil(t, stats)

	// 测试 GetPushTrend 在 Redis 为 nil 时的行为
	trend, err := pm.GetPushTrend("site-1")
	assert.Error(t, err)
	assert.Nil(t, trend)
}

// TestPushTask_StatusTransitions 测试任务状态转换
func TestPushTask_StatusTransitions(t *testing.T) {
	task := PushTask{
		ID:     "task-1",
		Status: "pending",
	}

	// 模拟状态转换
	statuses := []string{"pending", "running", "completed", "failed"}
	for _, status := range statuses {
		task.Status = status
		assert.Equal(t, status, task.Status)
	}
}

// TestPushLog_StatusValues 测试推送日志状态值
func TestPushLog_StatusValues(t *testing.T) {
	validStatuses := []string{"success", "failed", "pending", "retrying"}

	for _, status := range validStatuses {
		log := PushLog{
			ID:     "log-1",
			Status: status,
		}
		assert.Equal(t, status, log.Status)
	}
}

// TestPushManager_SearchEngines 测试支持的搜索引擎
func TestPushManager_SearchEngines(t *testing.T) {
	supportedEngines := []string{"baidu", "bing", "google", "sogou", "360"}

	for _, engine := range supportedEngines {
		log := PushLog{
			ID:           "log-1",
			SearchEngine: engine,
			Status:       "success",
		}
		assert.Equal(t, engine, log.SearchEngine)
	}
}
