package push

import (
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
