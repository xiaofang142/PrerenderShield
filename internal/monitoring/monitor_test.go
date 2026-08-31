package monitoring

import (
	"context"
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

// TestMonitor_MetricsGetter_Aliases 验证 metricsGetter 能将内置/文档别名指标名解析为 GetStats 规范键。
// 回归测试（Issue #1）：DefaultRules 用 system_cpu_usage 等，metricsGetter 键为 cpuUsage 等，
// 解析不一致会导致内置告警规则永不触发。
func TestMonitor_MetricsGetter_Aliases(t *testing.T) {
	m := NewMonitor(Config{Enabled: true})
	m.SetRenderQueueSize(55)
	m.SetActiveBrowsers(12)
	ctx := context.Background()

	cases := map[string]string{
		"system_cpu_usage":    "cpuUsage",
		"system_memory_usage": "memoryUsage",
		"system_disk_usage":   "diskUsage",
		"threats_per_minute":  "blockedRequests",
		"render_queue_size":   "renderQueueSize",
	}
	for alias, canonical := range cases {
		v, err := m.metricsGetter(ctx, alias)
		assert.NoError(t, err, "alias %s 应可解析", alias)
		// 仅对稳定指标断言与规范键结果一致（cpu/mem 为实时系统指标，两次采样可能不同）
		if alias == "threats_per_minute" || alias == "render_queue_size" {
			cv, err := m.metricsGetter(ctx, canonical)
			assert.NoError(t, err, "canonical %s 应可解析", canonical)
			assert.Equal(t, cv, v, "alias %s 应与 %s 结果一致", alias, canonical)
		}
	}

	// render_queue_size 应反映 SetRenderQueueSize 喂入的值
	v, err := m.metricsGetter(ctx, "render_queue_size")
	assert.NoError(t, err)
	assert.Equal(t, 55.0, v)

	// 未知指标应报错（不影响到正常路径）
	_, err = m.metricsGetter(ctx, "no_such_metric")
	assert.Error(t, err)
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

// ============== 告警操作符测试 ==============

// TestMonitor_checkAlertRule_AllOperators 测试所有告警操作符
func TestMonitor_checkAlertRule_AllOperators(t *testing.T) {
	tests := []struct {
		name           string
		operator       string
		value          float64
		threshold      float64
		expectedFiring bool
	}{
		{"GreaterThan triggered", ">", 85.0, 80.0, true},
		{"GreaterThan not triggered", ">", 75.0, 80.0, false},
		{"GreaterThan equal", ">", 80.0, 80.0, false},
		{"LessThan triggered", "<", 75.0, 80.0, true},
		{"LessThan not triggered", "<", 85.0, 80.0, false},
		{"LessThan equal", "<", 80.0, 80.0, false},
		{"GreaterThanOrEqual triggered", ">=", 80.0, 80.0, true},
		{"GreaterThanOrEqual not triggered", ">=", 75.0, 80.0, false},
		{"LessThanOrEqual triggered", "<=", 80.0, 80.0, true},
		{"LessThanOrEqual not triggered", "<=", 85.0, 80.0, false},
		{"Equals triggered", "==", 80.0, 80.0, true},
		{"Equals not triggered", "==", 85.0, 80.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMonitor(Config{
				Enabled: true,
				Alerting: AlertConfig{
					Enabled: true,
				},
			})

			rule := AlertRule{
				ID:        "test_rule",
				Name:      "Test Rule",
				Metric:    "testMetric",
				Operator:  tt.operator,
				Threshold: tt.threshold,
				Severity:  "warning",
			}

			stats := map[string]interface{}{
				"testMetric": tt.value,
			}

			m.checkAlertRule(rule, stats)

			m.alertMutex.RLock()
			status, exists := m.alerts["test_rule"]
			m.alertMutex.RUnlock()

			assert.True(t, exists)
			assert.Equal(t, tt.expectedFiring, status.IsFiring, "告警状态不匹配")
		})
	}
}

// TestMonitor_checkAlertRule_MissingMetric 测试缺失指标的情况
func TestMonitor_checkAlertRule_MissingMetric(t *testing.T) {
	m := NewMonitor(Config{
		Enabled: true,
		Alerting: AlertConfig{
			Enabled: true,
		},
	})

	rule := AlertRule{
		ID:        "test_rule",
		Name:      "Test Rule",
		Metric:    "missingMetric",
		Operator:  ">",
		Threshold: 80.0,
		Severity:  "warning",
	}

	stats := map[string]interface{}{
		"otherMetric": 100.0,
	}

	// 不应该 panic
	assert.NotPanics(t, func() {
		m.checkAlertRule(rule, stats)
	})

	// 告警不应该被创建
	m.alertMutex.RLock()
	_, exists := m.alerts["test_rule"]
	m.alertMutex.RUnlock()
	assert.False(t, exists)
}

// TestMonitor_checkAlertRule_InvalidType 测试无效类型
func TestMonitor_checkAlertRule_InvalidType(t *testing.T) {
	m := NewMonitor(Config{
		Enabled: true,
		Alerting: AlertConfig{
			Enabled: true,
		},
	})

	rule := AlertRule{
		ID:        "test_rule",
		Name:      "Test Rule",
		Metric:    "stringMetric",
		Operator:  ">",
		Threshold: 80.0,
		Severity:  "warning",
	}

	stats := map[string]interface{}{
		"stringMetric": "not a number",
	}

	// 不应该 panic
	assert.NotPanics(t, func() {
		m.checkAlertRule(rule, stats)
	})
}

// ============== 告警状态转换测试 ==============

// TestMonitor_checkAlertRule_StateTransition 测试告警状态转换
func TestMonitor_checkAlertRule_StateTransition(t *testing.T) {
	m := NewMonitor(Config{
		Enabled: true,
		Alerting: AlertConfig{
			Enabled: true,
		},
	})

	rule := AlertRule{
		ID:        "cpu_high",
		Name:      "CPU High",
		Metric:    "cpuUsage",
		Operator:  ">",
		Threshold: 80.0,
		Severity:  "warning",
	}

	// 第一次检查：告警触发
	stats1 := map[string]interface{}{"cpuUsage": 85.0}
	m.checkAlertRule(rule, stats1)

	m.alertMutex.RLock()
	status1, exists := m.alerts["cpu_high"]
	m.alertMutex.RUnlock()
	assert.True(t, exists)
	assert.True(t, status1.IsFiring)
	assert.False(t, status1.FiredAt.IsZero())   // Firing 时 FiredAt 应该被设置
	assert.True(t, status1.ResolvedAt.IsZero()) // Firing 时 ResolvedAt 应该为零值

	// 第二次检查：告警恢复
	stats2 := map[string]interface{}{"cpuUsage": 50.0}
	m.checkAlertRule(rule, stats2)

	m.alertMutex.RLock()
	status2, exists := m.alerts["cpu_high"]
	m.alertMutex.RUnlock()
	assert.True(t, exists)
	assert.False(t, status2.IsFiring)
	assert.False(t, status2.ResolvedAt.IsZero()) // 恢复时 ResolvedAt 应该有值
}

// ============== 通知测试 ==============

// TestMonitor_sendEmailNotification 测试发送邮件通知
func TestMonitor_sendEmailNotification(t *testing.T) {
	m := NewMonitor(Config{
		Enabled: true,
		Alerting: AlertConfig{
			Enabled: true,
			Notification: NotificationConfig{
				Email: EmailConfig{
					Enabled: true,
					To:      []string{"admin@example.com"},
				},
			},
		},
	})

	alert := &AlertStatus{
		Rule: AlertRule{
			Name:      "Test Alert",
			Severity:  "warning",
			Metric:    "cpuUsage",
			Threshold: 80.0,
		},
		Value: 85.0,
	}

	// 不应该 panic
	assert.NotPanics(t, func() {
		m.sendEmailNotification(alert, "firing")
	})
}

// TestMonitor_sendWebhookNotification 测试发送 Webhook 通知
func TestMonitor_sendWebhookNotification(t *testing.T) {
	m := NewMonitor(Config{
		Enabled: true,
		Alerting: AlertConfig{
			Enabled: true,
			Notification: NotificationConfig{
				Webhook: WebhookConfig{
					Enabled: true,
					URL:     "https://hooks.example.com/alert",
				},
			},
		},
	})

	alert := &AlertStatus{
		Rule: AlertRule{
			Name:      "Test Alert",
			Severity:  "critical",
			Metric:    "diskUsage",
			Threshold: 90.0,
		},
		Value: 95.0,
	}

	// 不应该 panic
	assert.NotPanics(t, func() {
		m.sendWebhookNotification(alert, "firing")
	})
}

// TestMonitor_sendAlertNotification_BothEnabled 测试同时启用邮件和 Webhook 通知
func TestMonitor_sendAlertNotification_BothEnabled(t *testing.T) {
	m := NewMonitor(Config{
		Enabled: true,
		Alerting: AlertConfig{
			Enabled: true,
			Notification: NotificationConfig{
				Email: EmailConfig{
					Enabled: true,
					To:      []string{"admin@example.com"},
				},
				Webhook: WebhookConfig{
					Enabled: true,
					URL:     "https://hooks.example.com/alert",
				},
			},
		},
	})

	alert := &AlertStatus{
		Rule: AlertRule{
			Name:     "Test Alert",
			Severity: "warning",
		},
	}

	// 不应该 panic
	assert.NotPanics(t, func() {
		m.sendAlertNotification(alert, "firing")
	})
}

// ============== 静态资源测试 ==============

// TestIsStaticResource_AllExtensions 测试所有静态资源扩展名
func TestIsStaticResource_AllExtensions(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		// 图片
		{"/images/photo.jpg", true},
		{"/images/photo.jpeg", true},
		{"/images/icon.png", true},
		{"/images/anim.gif", true},
		{"/images/photo.webp", true},
		{"/images/icon.svg", true},
		{"/images/favicon.ico", true},
		// 样式表
		{"/css/main.css", true},
		{"/styles/theme.less", true},
		{"/sass/main.sass", true},
		{"/scss/main.scss", true},
		// JavaScript
		{"/js/app.js", true},
		{"/js/bundle.ts", true},
		{"/js/app.jsx", true},
		{"/js/app.tsx", true},
		// 字体
		{"/fonts/roboto.woff", true},
		{"/fonts/roboto.woff2", true},
		{"/fonts/roboto.ttf", true},
		{"/fonts/roboto.eot", true},
		// 文档
		{"/docs/readme.txt", true},
		{"/docs/data.json", true},
		{"/docs/sitemap.xml", true},
		{"/docs/manual.pdf", true},
		// 压缩文件
		{"/files/archive.zip", true},
		{"/files/archive.rar", true},
		// 媒体
		{"/media/video.mp4", true},
		{"/media/audio.mp3", true},
		{"/media/video.avi", true},
		{"/media/video.mov", true},
		{"/media/video.wmv", true},
		// Office
		{"/docs/data.csv", true},
		{"/docs/report.xls", true},
		{"/docs/report.xlsx", true},
		{"/docs/doc.doc", true},
		{"/docs/doc.docx", true},
		// 非静态资源
		{"/api/users", false},
		{"/pages/home", false},
		{"/", false},
		{"/static/app", false}, // 没有扩展名
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isStaticResource(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============== Record 方法测试 ==============

// TestMonitor_RecordRequest_StaticResourceSkipped 测试静态资源被跳过
func TestMonitor_RecordRequest_StaticResourceSkipped(t *testing.T) {
	m := NewMonitor(Config{})

	initialStats := m.GetStats()
	initialTotal := initialStats["totalRequests"].(float64)

	// 记录静态资源请求（应该被跳过）
	m.RecordRequest("GET", "/static/app.js", 200, 10*time.Millisecond)
	m.RecordRequest("GET", "/images/logo.png", 200, 5*time.Millisecond)
	m.RecordRequest("GET", "/styles/main.css", 200, 8*time.Millisecond)

	stats := m.GetStats()
	// 静态资源不应该计入总请求数
	assert.Equal(t, initialTotal, stats["totalRequests"])
}

// TestMonitor_RecordRequest_DynamicResource 测试动态资源被记录
func TestMonitor_RecordRequest_DynamicResource(t *testing.T) {
	m := NewMonitor(Config{})

	initialStats := m.GetStats()
	initialTotal := initialStats["totalRequests"].(float64)

	// 记录动态请求
	m.RecordRequest("GET", "/api/users", 200, 50*time.Millisecond)
	m.RecordRequest("POST", "/api/data", 201, 100*time.Millisecond)

	stats := m.GetStats()
	assert.Equal(t, initialTotal+2, stats["totalRequests"])
}

// ============== formatFloat 测试 ==============

// TestFormatFloat 测试浮点数格式化
func TestFormatFloat(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
		name     string
	}{
		{12.345, 12.34, "two decimal places down"},
		{12.346, 12.34, "two decimal places up"},
		{0.999, 0.99, "fractional"},
		{100.0, 100.0, "whole number"},
		{0.0, 0.0, "zero"},
		{-5.555, -5.55, "negative"},
		{99.999, 99.99, "near hundred"},
		{1.234567, 1.23, "many decimals"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatFloat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============== 系统信息结构测试 ==============

// TestMemoryInfo_Struct 测试内存信息结构体
func TestMemoryInfo_Struct(t *testing.T) {
	info := MemoryInfo{
		Total:        16 * 1024 * 1024 * 1024,
		Used:         8 * 1024 * 1024 * 1024,
		Free:         8 * 1024 * 1024 * 1024,
		UsagePercent: 50.0,
	}

	assert.Equal(t, uint64(16*1024*1024*1024), info.Total)
	assert.Equal(t, uint64(8*1024*1024*1024), info.Used)
	assert.Equal(t, uint64(8*1024*1024*1024), info.Free)
	assert.Equal(t, 50.0, info.UsagePercent)
}

// TestDiskInfo_Struct 测试磁盘信息结构体
func TestDiskInfo_Struct(t *testing.T) {
	info := DiskInfo{
		Total:        512 * 1024 * 1024 * 1024,
		Used:         256 * 1024 * 1024 * 1024,
		Free:         256 * 1024 * 1024 * 1024,
		UsagePercent: 50.0,
	}

	assert.Equal(t, uint64(512*1024*1024*1024), info.Total)
	assert.Equal(t, uint64(256*1024*1024*1024), info.Used)
	assert.Equal(t, uint64(256*1024*1024*1024), info.Free)
	assert.Equal(t, 50.0, info.UsagePercent)
}

// TestNetworkInfo_Struct 测试网络信息结构体
func TestNetworkInfo_Struct(t *testing.T) {
	info := NetworkInfo{
		BytesSent:   1024 * 1024 * 100,
		BytesRecv:   1024 * 1024 * 500,
		PacketsSent: 100000,
		PacketsRecv: 500000,
	}

	assert.Equal(t, uint64(1024*1024*100), info.BytesSent)
	assert.Equal(t, uint64(1024*1024*500), info.BytesRecv)
	assert.Equal(t, uint64(100000), info.PacketsSent)
	assert.Equal(t, uint64(500000), info.PacketsRecv)
}

// ============== 系统信息获取测试 ==============

// TestGetCPUUsage 测试获取 CPU 使用率
func TestGetCPUUsage(t *testing.T) {
	usage, err := getCPUUsage()

	// 不应该返回错误（在正常系统上）
	// 某些系统可能无法获取，所以只验证类型
	assert.IsType(t, 0.0, usage)
	// 如果成功，值应该在 0-100 之间
	if err == nil {
		assert.GreaterOrEqual(t, usage, 0.0)
		assert.LessOrEqual(t, usage, 100.0)
	}
}

// TestGetMemoryInfo 测试获取内存信息
func TestGetMemoryInfo(t *testing.T) {
	info, err := getMemoryInfo()

	if err != nil {
		assert.Nil(t, info)
	} else {
		assert.NotNil(t, info)
		assert.Greater(t, info.Total, uint64(0))
		assert.LessOrEqual(t, info.Used, info.Total)
		assert.GreaterOrEqual(t, info.UsagePercent, 0.0)
		assert.LessOrEqual(t, info.UsagePercent, 100.0)
	}
}

// TestGetDiskInfo 测试获取磁盘信息
func TestGetDiskInfo(t *testing.T) {
	info, err := getDiskInfo()

	if err != nil {
		assert.Nil(t, info)
	} else {
		assert.NotNil(t, info)
		assert.Greater(t, info.Total, uint64(0))
		assert.LessOrEqual(t, info.Used, info.Total)
		assert.GreaterOrEqual(t, info.UsagePercent, 0.0)
		assert.LessOrEqual(t, info.UsagePercent, 100.0)
	}
}

// TestGetNetworkInfo 测试获取网络信息
func TestGetNetworkInfo(t *testing.T) {
	info, err := getNetworkInfo()

	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.GreaterOrEqual(t, info.BytesSent, uint64(0))
	assert.GreaterOrEqual(t, info.BytesRecv, uint64(0))
	assert.GreaterOrEqual(t, info.PacketsSent, uint64(0))
	assert.GreaterOrEqual(t, info.PacketsRecv, uint64(0))
}

// ============== GetStats 完整测试 ==============

// TestMonitor_GetStats_Complete 测试完整的统计数据
func TestMonitor_GetStats_Complete(t *testing.T) {
	m := NewMonitor(Config{})

	// 记录一些数据
	m.RecordRequest("GET", "/api/test", 200, 100*time.Millisecond)
	m.RecordRequest("POST", "/api/test", 201, 200*time.Millisecond)
	m.RecordCacheHit()
	m.RecordCacheHit()
	m.RecordCacheMiss()
	m.SetActiveBrowsers(5)

	stats := m.GetStats()

	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats["totalRequests"], float64(2))
	assert.GreaterOrEqual(t, stats["cacheHits"], float64(2))
	assert.GreaterOrEqual(t, stats["cacheMisses"], float64(1))
	assert.Equal(t, float64(5), stats["activeBrowsers"])

	// 验证缓存命中率计算
	cacheHitRate := stats["cacheHitRate"].(float64)
	assert.Greater(t, cacheHitRate, 0.0)
	assert.LessOrEqual(t, cacheHitRate, 100.0)
}

// TestMonitor_GetStats_EmptyCache 测试空缓存命中率
func TestMonitor_GetStats_EmptyCache(t *testing.T) {
	// 重置全局状态
	statsStore.mu.Lock()
	statsStore.totalRequests = 0
	statsStore.crawlerRequests = 0
	statsStore.blockedRequests = 0
	statsStore.cacheHits = 0
	statsStore.cacheMisses = 0
	statsStore.activeBrowsers = 0
	statsStore.mu.Unlock()

	m := NewMonitor(Config{})

	stats := m.GetStats()

	assert.NotNil(t, stats)
	// 缓存命中率为 0（没有请求）
	assert.Equal(t, 0.0, stats["cacheHitRate"])
	// 验证其他指标也为 0
	assert.Equal(t, float64(0), stats["totalRequests"])
	assert.Equal(t, float64(0), stats["cacheHits"])
	assert.Equal(t, float64(0), stats["cacheMisses"])
}
