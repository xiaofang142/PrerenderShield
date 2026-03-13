package monitoring

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestMonitor_Struct 测试 Monitor 结构体
func TestMonitor_Struct(t *testing.T) {
	m := &Monitor{
		alerts: make(map[string]*AlertStatus),
		stopCh: make(chan struct{}),
	}
	assert.NotNil(t, m)
}

// TestNewMonitor 测试创建 Monitor
func TestNewMonitor(t *testing.T) {
	config := Config{
		Enabled: true,
		Alerting: AlertConfig{
			Enabled: true,
		},
	}
	m := NewMonitor(config)
	assert.NotNil(t, m)
	assert.True(t, m.config.Enabled)
	assert.False(t, m.isRunning)
	assert.Nil(t, m.redisClient)
}

// TestNewMonitor_DefaultConfig 测试使用默认配置创建 Monitor
func TestNewMonitor_DefaultConfig(t *testing.T) {
	m := NewMonitor(Config{})
	assert.NotNil(t, m)
	assert.False(t, m.config.Enabled)
}

// TestMonitor_CheckAlerts_AlertingDisabled 测试告警禁用时的行为
func TestMonitor_CheckAlerts_AlertingDisabled(t *testing.T) {
	m := NewMonitor(Config{
		Enabled: true,
		Alerting: AlertConfig{
			Enabled: false,
		},
	})

	// 不应该 panic
	assert.NotPanics(t, func() {
		m.CheckAlerts()
	})
}

// TestMonitor_CheckAlerts_WithRules 测试有告警规则时的行为
func TestMonitor_CheckAlerts_WithRules(t *testing.T) {
	m := NewMonitor(Config{
		Enabled: true,
		Alerting: AlertConfig{
			Enabled: true,
			AlertRules: []AlertRule{
				{
					ID:        "test-rule",
					Name:      "Test Rule",
					Metric:    "error_rate",
					Operator:  ">",
					Threshold: 0.1,
					Duration:  time.Minute,
					Severity:  "warning",
				},
			},
		},
	})

	// 不应该 panic
	assert.NotPanics(t, func() {
		m.CheckAlerts()
	})
}

// TestMonitor_SetRedisClient 测试设置 Redis 客户端
func TestMonitor_SetRedisClient(t *testing.T) {
	m := NewMonitor(Config{})
	// redisClient 为 nil 时不应该 panic
	assert.NotPanics(t, func() {
		m.SetRedisClient(nil)
	})
}

// TestMonitor_SaveMetricsToRedis_NilRedis 测试在 Redis 为 nil 时保存指标
func TestMonitor_SaveMetricsToRedis_NilRedis(t *testing.T) {
	m := NewMonitor(Config{})

	err := m.SaveMetricsToRedis()
	// 应该返回错误而不是 panic
	assert.Error(t, err)
}

// TestMonitor_GetMetricsFromRedis_NilRedis 测试在 Redis 为 nil 时获取指标
func TestMonitor_GetMetricsFromRedis_NilRedis(t *testing.T) {
	m := NewMonitor(Config{})

	metrics, err := m.GetMetricsFromRedis(0, 0)
	// 应该返回错误而不是 panic
	assert.Error(t, err)
	assert.Nil(t, metrics)
}

// TestMonitor_Start_NilRedis 测试 Start 方法
func TestMonitor_Start_NilRedis(t *testing.T) {
	m := NewMonitor(Config{
		Enabled: true,
	})

	// 不应该 panic
	assert.NotPanics(t, func() {
		m.Start()
	})
}

// TestMonitor_Stop 测试 Stop 方法
func TestMonitor_Stop(t *testing.T) {
	m := NewMonitor(Config{})

	// 不应该 panic
	assert.NotPanics(t, func() {
		m.Stop()
	})
}

// TestMonitor_isStaticResource 测试静态资源检测
func TestMonitor_isStaticResource(t *testing.T) {
	// isStaticResource 是包级私有函数，直接测试
	assert.True(t, isStaticResource("/static/js/app.js"))
	assert.True(t, isStaticResource("/static/style.css"))
	assert.True(t, isStaticResource("/images/logo.png"))
	assert.True(t, isStaticResource("/fonts/arial.woff"))

	// 测试非静态资源
	assert.False(t, isStaticResource("/api/users"))
	assert.False(t, isStaticResource("/prerender/page"))
	assert.False(t, isStaticResource("/"))
}

// TestMonitor_isStaticResource_EmptyPath 测试空路径
func TestMonitor_isStaticResource_EmptyPath(t *testing.T) {
	assert.False(t, isStaticResource(""))
}

// TestMonitor_RecordRequest 测试记录请求
func TestMonitor_RecordRequest(t *testing.T) {
	m := NewMonitor(Config{})

	// 不应该 panic
	assert.NotPanics(t, func() {
		m.RecordRequest("GET", "/api/test", 200, 100*time.Millisecond)
	})
}

// TestMonitor_RecordCrawlerRequest 测试记录爬虫请求
func TestMonitor_RecordCrawlerRequest(t *testing.T) {
	m := NewMonitor(Config{})

	// 不应该 panic
	assert.NotPanics(t, func() {
		m.RecordCrawlerRequest()
	})
}

// TestMonitor_RecordBlockedRequest 测试记录被拦截的请求
func TestMonitor_RecordBlockedRequest(t *testing.T) {
	m := NewMonitor(Config{})

	// 不应该 panic
	assert.NotPanics(t, func() {
		m.RecordBlockedRequest()
	})
}

// TestMonitor_RecordCacheHit 测试记录缓存命中
func TestMonitor_RecordCacheHit(t *testing.T) {
	m := NewMonitor(Config{})

	// 不应该 panic
	assert.NotPanics(t, func() {
		m.RecordCacheHit()
	})
}

// TestMonitor_RecordCacheMiss 测试记录缓存未命中
func TestMonitor_RecordCacheMiss(t *testing.T) {
	m := NewMonitor(Config{})

	// 不应该 panic
	assert.NotPanics(t, func() {
		m.RecordCacheMiss()
	})
}

// TestMonitor_SetActiveBrowsers 测试设置活跃浏览器数量
func TestMonitor_SetActiveBrowsers(t *testing.T) {
	m := NewMonitor(Config{})

	// 不应该 panic
	assert.NotPanics(t, func() {
		m.SetActiveBrowsers(5)
	})
}

// TestMonitor_RecordRenderTime 测试记录渲染时间
func TestMonitor_RecordRenderTime(t *testing.T) {
	m := NewMonitor(Config{})

	// 不应该 panic
	assert.NotPanics(t, func() {
		m.RecordRenderTime(2 * time.Second)
	})
}

// TestMonitor_GetStats 测试获取统计数据
func TestMonitor_GetStats(t *testing.T) {
	m := NewMonitor(Config{})

	stats := m.GetStats()
	assert.NotNil(t, stats)

	// 验证包含预期的键 (camelCase 格式)
	assert.Contains(t, stats, "totalRequests")
	assert.Contains(t, stats, "crawlerRequests")
	assert.Contains(t, stats, "blockedRequests")
	assert.Contains(t, stats, "cacheHits")
	assert.Contains(t, stats, "cacheMisses")
	assert.Contains(t, stats, "activeBrowsers")
	assert.Contains(t, stats, "cacheHitRate")
	assert.Contains(t, stats, "cpuUsage")
	assert.Contains(t, stats, "memoryUsage")
}

// TestAlertRule_Struct 测试 AlertRule 结构体
// 注意：formatFloat、getCPUUsage、getMemoryInfo、getDiskInfo、getNetworkInfo 是私有函数
// 无法直接从测试文件调用，因此不测试这些函数
func TestAlertRule_Struct(t *testing.T) {
	rule := AlertRule{
		ID:        "test-rule",
		Name:      "Test Rule",
		Metric:    "cpu_usage",
		Operator:  ">",
		Threshold: 80.0,
		Duration:  5 * time.Minute,
		Severity:  "critical",
	}

	assert.Equal(t, "test-rule", rule.ID)
	assert.Equal(t, "Test Rule", rule.Name)
	assert.Equal(t, "cpu_usage", rule.Metric)
	assert.Equal(t, ">", rule.Operator)
	assert.Equal(t, 80.0, rule.Threshold)
	assert.Equal(t, 5*time.Minute, rule.Duration)
	assert.Equal(t, "critical", rule.Severity)
}

// TestAlertStatus_Struct 测试 AlertStatus 结构体
func TestAlertStatus_Struct(t *testing.T) {
	status := AlertStatus{
		Rule:        AlertRule{ID: "test-rule"},
		IsFiring:    true,
		FiredAt:     time.Now(),
		LastChecked: time.Now(),
		Value:       85.5,
	}

	assert.True(t, status.IsFiring)
	assert.NotZero(t, status.FiredAt)
	assert.Equal(t, 85.5, status.Value)
}

// TestConfig_Struct 测试 Config 结构体
func TestConfig_Struct(t *testing.T) {
	config := Config{
		Enabled:           true,
		PrometheusAddress: ":9090",
		Alerting: AlertConfig{
			Enabled: true,
		},
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, ":9090", config.PrometheusAddress)
	assert.True(t, config.Alerting.Enabled)
}

// TestEmailConfig_Struct 测试 EmailConfig 结构体
func TestEmailConfig_Struct(t *testing.T) {
	config := EmailConfig{
		Enabled:  true,
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		Username: "user@example.com",
		Password: "password",
		From:     "alert@example.com",
		To:       []string{"admin@example.com"},
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "smtp.example.com", config.SMTPHost)
	assert.Equal(t, 587, config.SMTPPort)
	assert.Len(t, config.To, 1)
}

// TestWebhookConfig_Struct 测试 WebhookConfig 结构体
func TestWebhookConfig_Struct(t *testing.T) {
	config := WebhookConfig{
		Enabled: true,
		URL:     "https://example.com/webhook",
		Secret:  "secret-key",
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "https://example.com/webhook", config.URL)
	assert.Equal(t, "secret-key", config.Secret)
}

// TestNotificationConfig_Struct 测试 NotificationConfig 结构体
func TestNotificationConfig_Struct(t *testing.T) {
	config := NotificationConfig{
		Email:   EmailConfig{Enabled: true},
		Webhook: WebhookConfig{Enabled: true},
	}

	assert.True(t, config.Email.Enabled)
	assert.True(t, config.Webhook.Enabled)
}
