package monitoring

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"

	"prerender-shield/internal/logging"
	"prerender-shield/internal/monitoring/alerting"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/repository"
)

// Metrics 监控指标
var (
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "prerender_requests_total",
			Help: "Total number of requests",
		},
		[]string{"method", "path", "status"},
	)

	responseTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "prerender_response_time_seconds",
			Help:    "Response time in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	crawlerRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "prerender_crawler_requests_total",
			Help: "Total number of crawler requests",
		},
	)

	blockedRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "prerender_blocked_requests_total",
			Help: "Total number of blocked requests",
		},
	)

	cacheHits = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "prerender_cache_hits_total",
			Help: "Total number of cache hits",
		},
	)

	cacheMisses = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "prerender_cache_misses_total",
			Help: "Total number of cache misses",
		},
	)

	activeBrowsers = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "prerender_active_browsers",
			Help: "Number of active browsers",
		},
	)

	renderTime = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "prerender_render_time_seconds",
			Help:    "Render time in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	// 预聚合指标
	cacheHitRate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "prerender_cache_hit_rate",
		Help: "Cache hit rate (0-1)",
	})

	wafBlockRate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "prerender_waf_block_rate",
		Help: "WAF block rate (0-1)",
	})

	renderSuccessRate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "prerender_render_success_rate",
		Help: "Render success rate (0-1)",
	})

	avgRenderTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "prerender_avg_render_time_seconds",
		Help: "Average render time in seconds",
	})

	activeSitesGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "prerender_active_sites",
		Help: "Number of active sites",
	})
)

// CacheHitRateGauge 返回缓存命中率指标
func CacheHitRateGauge() prometheus.Gauge { return cacheHitRate }

// WAFBlockRateGauge 返回 WAF 拦截率指标
func WAFBlockRateGauge() prometheus.Gauge { return wafBlockRate }

// RenderSuccessRateGauge 返回渲染成功率指标
func RenderSuccessRateGauge() prometheus.Gauge { return renderSuccessRate }

// AvgRenderTimeGauge 返回平均渲染时间指标
func AvgRenderTimeGauge() prometheus.Gauge { return avgRenderTime }

// ActiveSitesGauge 返回活跃站点数指标
func ActiveSitesGauge() prometheus.Gauge { return activeSitesGauge }

// Monitor 监控管理器
type Monitor struct {
	isRunning     bool
	config        Config
	redisClient   *redis.Client
	alerts        map[string]*AlertStatus // 告警状态
	alertMutex    sync.RWMutex
	wg            sync.WaitGroup
	stopCh        chan struct{}
	ruleEngine    *alerting.RuleEngine                    // 告警规则引擎
	metricsGetter alerting.MetricsFunc                    // 指标获取函数
	onAlert       func(alert *AlertStatus, status string) // 告警回调（如 WebSocket 广播）
	alertRepo     *repository.AlertRepository             // 告警历史持久化（SetRedisClient 时初始化）
	// notificationSource 动态通知配置来源（bootstrap 注入，控制台改动即时生效免重启）
	notificationSource func() (*alerting.WebhookConfig, *alerting.EmailConfig)
}

// AlertStatus 告警状态
type AlertStatus struct {
	Rule        AlertRule
	IsFiring    bool
	FiredAt     time.Time
	ResolvedAt  time.Time
	LastChecked time.Time
	Value       float64
}

// Config 监控配置
type Config struct {
	Enabled           bool
	PrometheusAddress string
	Alerting          AlertConfig
	// 监控数据持久化配置
	MetricsPersistence MetricsPersistenceConfig
}

// MetricsPersistenceConfig 监控数据持久化配置
type MetricsPersistenceConfig struct {
	Enabled           bool
	Interval          time.Duration // 持久化间隔
	Retention         time.Duration // 数据保留时间
	AggregateEnabled  bool
	AggregateInterval time.Duration // 聚合间隔
}

// AlertConfig 告警配置
type AlertConfig struct {
	Enabled      bool
	AlertRules   []AlertRule
	Notification NotificationConfig
}

// AlertRule 告警规则
type AlertRule struct {
	ID        string
	Name      string
	Metric    string // 监控指标名称
	Operator  string // 操作符: >, <, >=, <=, ==
	Threshold float64
	Duration  time.Duration // 持续时间
	Severity  string        // 严重程度: info, warning, critical
}

// NotificationConfig 通知配置
type NotificationConfig struct {
	Email   EmailConfig
	Webhook WebhookConfig
}

// EmailConfig 邮件配置
type EmailConfig struct {
	Enabled  bool
	SMTPHost string
	SMTPPort int
	Username string
	Password string
	From     string
	To       []string
}

// WebhookConfig Webhook配置
type WebhookConfig struct {
	Enabled bool
	URL     string
	Secret  string
}

// NewMonitor 创建新的监控管理器
func NewMonitor(config Config) *Monitor {
	m := &Monitor{
		isRunning:   false,
		config:      config,
		redisClient: nil,
		alerts:      make(map[string]*AlertStatus),
		stopCh:      make(chan struct{}),
		ruleEngine:  alerting.NewRuleEngine(),
	}
	// 设置指标获取函数
	m.metricsGetter = func(ctx context.Context, metric string) (float64, error) {
		stats := m.GetStats()
		if canonical, ok := metricAliases[metric]; ok {
			metric = canonical
		}
		if value, ok := stats[metric]; ok {
			if floatValue, ok := value.(float64); ok {
				return floatValue, nil
			}
		}
		return 0, fmt.Errorf("metric %s not found", metric)
	}
	return m
}

// resolveMetricKey 将告警规则/文档中可能出现的别名指标名解析为 GetStats 返回的规范键。
// 修复：内置 DefaultRules 使用 system_cpu_usage 等指标名，而 metricsGetter 只查 GetStats
//（键为 cpuUsage 等），导致内置规则因 "metric not found" 永远不触发。此处在引擎查询层
// 统一做一次别名归一，使规则/示例 JSON 两种命名风格都能命中真实指标。
var metricAliases = map[string]string{
	"system_cpu_usage":    "cpuUsage",
	"system_memory_usage": "memoryUsage",
	"system_disk_usage":   "diskUsage",
	"threats_per_minute":  "blockedRequests",
	"render_queue_size":   "renderQueueSize",
}

// LoadAlertRules 从文件加载告警规则
func (m *Monitor) LoadAlertRules(filename string) error {
	return m.ruleEngine.LoadRulesFromFile(filename)
}

// AddAlertHandler 添加告警处理器
func (m *Monitor) AddAlertHandler(handler alerting.AlertHandler) {
	m.ruleEngine.AddHandler(handler)
}

// SetNotificationSource 注入动态通知配置来源（每次告警触发时读取最新配置）
func (m *Monitor) SetNotificationSource(fn func() (*alerting.WebhookConfig, *alerting.EmailConfig)) {
	m.notificationSource = fn
}

// LoadRulesFromStore 从 Redis 规则库同步告警规则到内存引擎（启动时 + API 保存后调用）。
// 修复：控制台保存的规则此前只写 Redis，规则引擎从不读取 → UI 配置的告警永不触发。
func (m *Monitor) LoadRulesFromStore() {
	if m.alertRepo == nil {
		return
	}
	for _, r := range m.alertRepo.GetAlertRules() {
		m.ruleEngine.AddRule(alertRuleDataToEngine(r))
	}
}

// alertRuleDataToEngine AlertRuleData（Redis 形态）→ alerting.Rule（引擎形态）
func alertRuleDataToEngine(r repository.AlertRuleData) *alerting.Rule {
	op := map[string]string{">": "gt", "<": "lt", "==": "eq", ">=": "ge", "<=": "le"}[r.Operator]
	if op == "" {
		op = "gt"
	}
	cooldown := time.Duration(r.Cooldown) * time.Second
	if cooldown <= 0 {
		cooldown = 5 * time.Minute
	}
	return &alerting.Rule{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		Enabled:     r.Enabled,
		Condition:   &alerting.Condition{Metric: r.Metric, Operator: op, Threshold: r.Threshold},
		Severity:    r.Severity,
		Cooldown:    cooldown,
	}
}

// SetOnAlertCallback 设置告警回调（在告警通知发出时同步调用，如 WebSocket 广播）
func (m *Monitor) SetOnAlertCallback(fn func(alert *AlertStatus, status string)) {
	m.onAlert = fn
}

// GetAlertRules 获取所有告警规则
func (m *Monitor) GetAlertRules() []*alerting.Rule {
	return m.ruleEngine.GetRules()
}

// AddAlertRule 添加告警规则
func (m *Monitor) AddAlertRule(rule *alerting.Rule) {
	m.ruleEngine.AddRule(rule)
}

// UpdateAlertRule 更新告警规则
func (m *Monitor) UpdateAlertRule(rule *alerting.Rule) {
	m.ruleEngine.UpdateRule(rule)
}

// DeleteAlertRule 删除告警规则
func (m *Monitor) DeleteAlertRule(ruleID string) {
	m.ruleEngine.RemoveRule(ruleID)
}

// setupDefaultRules 设置默认告警规则
func (m *Monitor) setupDefaultRules() {
	defaultRules := alerting.DefaultRules()
	for _, rule := range defaultRules {
		m.ruleEngine.AddRule(rule)
	}
}

// CheckAlerts 检查告警
func (m *Monitor) CheckAlerts() {
	if !m.config.Alerting.Enabled {
		return
	}

	// 获取当前统计数据
	stats := m.GetStats()

	// 检查每个告警规则
	for _, rule := range m.config.Alerting.AlertRules {
		m.checkAlertRule(rule, stats)
	}
}

// checkAlertRule 检查单个告警规则
func (m *Monitor) checkAlertRule(rule AlertRule, stats map[string]interface{}) {
	// 获取指标值
	value, ok := stats[rule.Metric]
	if !ok {
		return
	}

	// 转换为float64
	floatValue, ok := value.(float64)
	if !ok {
		return
	}

	// 检查是否触发告警
	isFiring := false
	switch rule.Operator {
	case ">":
		isFiring = floatValue > rule.Threshold
	case "<":
		isFiring = floatValue < rule.Threshold
	case ">=":
		isFiring = floatValue >= rule.Threshold
	case "<=":
		isFiring = floatValue <= rule.Threshold
	case "==":
		isFiring = floatValue == rule.Threshold
	}

	// 更新告警状态
	m.alertMutex.Lock()
	defer m.alertMutex.Unlock()

	alertStatus, exists := m.alerts[rule.ID]
	if !exists {
		alertStatus = &AlertStatus{
			Rule:        rule,
			IsFiring:    false,
			LastChecked: time.Now(),
			Value:       floatValue,
		}
		m.alerts[rule.ID] = alertStatus
	}

	// 更新最后检查时间和值
	alertStatus.LastChecked = time.Now()
	alertStatus.Value = floatValue

	// 处理告警状态变化
	if isFiring && !alertStatus.IsFiring {
		// 告警触发
		alertStatus.IsFiring = true
		alertStatus.FiredAt = time.Now()
		m.sendAlertNotification(alertStatus, "firing")
	} else if !isFiring && alertStatus.IsFiring {
		// 告警恢复
		alertStatus.IsFiring = false
		alertStatus.ResolvedAt = time.Now()
		m.sendAlertNotification(alertStatus, "resolved")
	}
}

// sendAlertNotification 发送告警通知
func (m *Monitor) sendAlertNotification(alert *AlertStatus, status string) {
	// 外部回调（WebSocket 实时广播）
	if m.onAlert != nil {
		m.onAlert(alert, status)
	}

	// 保存告警记录到 Redis
	m.saveAlertToRedis(alert, status)

	// 发送邮件通知
	if m.config.Alerting.Notification.Email.Enabled {
		m.sendEmailNotification(alert, status)
	}

	// 发送Webhook通知
	if m.config.Alerting.Notification.Webhook.Enabled {
		m.sendWebhookNotification(alert, status)
	}
}

// saveAlertToRedis 保存告警记录到 Redis
// 委托 AlertRepository 统一持有告警历史的键名/格式/裁剪逻辑，避免双处维护
func (m *Monitor) saveAlertToRedis(alert *AlertStatus, status string) {
	if m.redisClient == nil {
		return
	}
	now := time.Now()
	m.alertRepo.AppendAlertHistory(repository.AlertRecord{
		ID:        fmt.Sprintf("alert_%d", now.UnixNano()),
		Level:     alert.Rule.Severity,
		Rule:      alert.Rule.Name,
		Message:   fmt.Sprintf("%s is %s threshold %.2f (current: %.2f)", alert.Rule.Metric, alert.Rule.Operator, alert.Rule.Threshold, alert.Value),
		Value:     alert.Value,
		Threshold: alert.Rule.Threshold,
		Status:    status,
		Timestamp: now,
	})
}

// sendEmailNotification 发送邮件通知
func (m *Monitor) sendEmailNotification(alert *AlertStatus, status string) {
	emailConfig := m.config.Alerting.Notification.Email
	if emailConfig.SMTPHost == "" {
		logging.DefaultLogger.Warn("SMTP not configured, skipping email notification")
		return
	}

	subject := fmt.Sprintf("[%s] %s - %s", alert.Rule.Severity, alert.Rule.Name, status)
	body := fmt.Sprintf("Metric %s is %s threshold %.2f (current value: %.2f)\n",
		alert.Rule.Metric, alert.Rule.Operator, alert.Rule.Threshold, alert.Value)

	addr := fmt.Sprintf("%s:%d", emailConfig.SMTPHost, emailConfig.SMTPPort)
	auth := smtp.PlainAuth("", emailConfig.Username, emailConfig.Password, emailConfig.SMTPHost)

	for _, to := range emailConfig.To {
		msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
			emailConfig.From, to, subject, body))
		err := smtp.SendMail(addr, auth, emailConfig.From, []string{to}, msg)
		if err != nil {
			logging.DefaultLogger.Warn("Failed to send email to %s: %v", to, err)
		}
	}
}

// sendWebhookNotification 发送Webhook通知
func (m *Monitor) sendWebhookNotification(alert *AlertStatus, status string) {
	webhookConfig := m.config.Alerting.Notification.Webhook
	if webhookConfig.URL == "" {
		return
	}

	payload := map[string]interface{}{
		"rule":      alert.Rule.Name,
		"status":    status,
		"severity":  alert.Rule.Severity,
		"value":     alert.Value,
		"threshold": alert.Rule.Threshold,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		logging.DefaultLogger.Warn("Failed to marshal webhook payload: %v", err)
		return
	}

	resp, err := http.Post(webhookConfig.URL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		logging.DefaultLogger.Warn("Failed to send webhook: %v", err)
		return
	}
	defer resp.Body.Close()
}

// SetRedisClient 设置Redis客户端
func (m *Monitor) SetRedisClient(client *redis.Client) {
	m.redisClient = client
	m.alertRepo = repository.NewAlertRepository(client)
}

// SaveMetricsToRedis 保存监控指标到Redis
func (m *Monitor) SaveMetricsToRedis() error {
	if m.redisClient == nil {
		return fmt.Errorf("redis client is not set")
	}

	// 获取当前统计数据
	stats := m.GetStats()

	// 序列化统计数据
	statsJSON, err := json.Marshal(stats)
	if err != nil {
		return err
	}

	// 保存到Redis，使用时间戳作为键的一部分
	timestamp := time.Now().Unix()
	key := fmt.Sprintf("prerender:metrics:%d", timestamp)

	// 使用Redis的SET命令保存数据，设置过期时间为24小时
	err = m.redisClient.Set(key, string(statsJSON), 24*time.Hour)
	if err != nil {
		return err
	}

	return nil
}

// GetMetricsFromRedis 从Redis获取监控指标
func (m *Monitor) GetMetricsFromRedis(startTime, endTime int64) ([]map[string]interface{}, error) {
	if m.redisClient == nil {
		return nil, fmt.Errorf("redis client is not set")
	}

	// 获取所有指标键
	keys, err := m.redisClient.Keys("prerender:metrics:*")
	if err != nil {
		return nil, err
	}

	// 从Redis中获取每个键对应的数据，按时间范围过滤
	metrics := make([]map[string]interface{}, 0, len(keys))
	for _, key := range keys {
		// 跳过聚合键
		if strings.Contains(key, ":agg:") {
			continue
		}

		// 从键中提取时间戳进行范围过滤
		parts := strings.Split(key, ":")
		if len(parts) < 3 {
			continue
		}
		timestamp, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
		if err != nil {
			continue
		}
		if timestamp < startTime || timestamp > endTime {
			continue
		}

		data, err := m.redisClient.Get(key)
		if err != nil || data == "" {
			continue
		}

		var stat map[string]interface{}
		if err := json.Unmarshal([]byte(data), &stat); err != nil {
			continue
		}

		metrics = append(metrics, stat)
	}

	return metrics, nil
}

// AggregateMetricsToRedis 聚合监控数据到 Redis
// 将指定时间范围内的监控数据进行聚合，计算平均值、最大值、最小值等统计信息
func (m *Monitor) AggregateMetricsToRedis() error {
	if m.redisClient == nil {
		return fmt.Errorf("redis client is not set")
	}

	// 获取过去一小时的监控数据
	now := time.Now()
	hourAgo := now.Add(-time.Hour).Unix()
	nowUnix := now.Unix()

	metrics, err := m.GetMetricsFromRedis(hourAgo, nowUnix)
	if err != nil {
		return err
	}

	if len(metrics) == 0 {
		return nil
	}

	// 计算聚合统计
	aggStats := make(map[string][]float64)
	metricKeys := []string{
		"totalRequests", "crawlerRequests", "blockedRequests",
		"cacheHits", "cacheMisses", "cacheHitRate",
		"activeBrowsers", "cpuUsage", "memoryUsage", "diskUsage",
	}

	for _, metric := range metrics {
		for _, key := range metricKeys {
			if value, ok := metric[key].(float64); ok {
				aggStats[key] = append(aggStats[key], value)
			}
		}
	}

	// 计算聚合值
	aggregated := make(map[string]interface{})
	for key, values := range aggStats {
		if len(values) == 0 {
			continue
		}

		var sum, min, max float64
		min = values[0]
		max = values[0]

		for _, v := range values {
			sum += v
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
		}

		avg := sum / float64(len(values))

		aggregated[key+"_avg"] = formatFloat(avg)
		aggregated[key+"_min"] = formatFloat(min)
		aggregated[key+"_max"] = formatFloat(max)
	}

	// 添加时间戳和聚合类型
	aggregated["timestamp"] = nowUnix
	aggregated["aggregation_period"] = "hourly"

	// 保存聚合数据到 Redis
	aggKey := fmt.Sprintf("prerender:metrics:agg:%d", nowUnix/3600*3600)
	aggJSON, err := json.Marshal(aggregated)
	if err != nil {
		return err
	}

	// 设置过期时间为 30 天
	err = m.redisClient.Set(aggKey, string(aggJSON), 30*24*time.Hour)
	if err != nil {
		return err
	}

	return nil
}

// CleanupExpiredMetrics 清理过期的监控数据
func (m *Monitor) CleanupExpiredMetrics() error {
	if m.redisClient == nil {
		return fmt.Errorf("redis client is not set")
	}

	// 获取保留时间，默认 24 小时
	retention := m.config.MetricsPersistence.Retention
	if retention == 0 {
		retention = 24 * time.Hour
	}

	cutoffTime := time.Now().Add(-retention).Unix()

	// 获取所有指标键
	keys, err := m.redisClient.Keys("prerender:metrics:*")
	if err != nil {
		return err
	}

	// 清理过期数据
	deletedCount := 0
	for _, key := range keys {
		// 跳过聚合数据（聚合数据有更长的保留时间）
		if strings.Contains(key, ":agg:") {
			continue
		}

		// 从键中提取时间戳
		parts := strings.Split(key, ":")
		if len(parts) < 3 {
			continue
		}

		timestamp, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
		if err != nil {
			continue
		}

		// 如果数据过期，删除它
		if timestamp < cutoffTime {
			if err := m.redisClient.Del(key); err != nil {
				logging.DefaultLogger.Warn("Failed to delete expired metrics key %s: %v", key, err)
				continue
			}
			deletedCount++
		}
	}

	if deletedCount > 0 {
		logging.DefaultLogger.Info("Cleaned up %d expired metrics entries", deletedCount)
	}

	return nil
}

// Start 启动监控服务
func (m *Monitor) Start() error {
	if m.isRunning {
		return nil
	}

	// 注册指标
	prometheus.MustRegister(
		requestsTotal,
		responseTime,
		crawlerRequests,
		blockedRequests,
		cacheHits,
		cacheMisses,
		activeBrowsers,
		renderTime,
		cacheHitRate,
		wafBlockRate,
		renderSuccessRate,
		avgRenderTime,
		activeSitesGauge,
	)

	// 启动Prometheus服务器
	m.wg.Add(1)
	go func() {

		defer m.wg.Done()

		http.Handle("/metrics", promhttp.Handler())
		// 使用配置中的地址，默认使用:9090
		addr := m.config.PrometheusAddress
		if addr == "" {
			addr = ":9090"
		}
		http.ListenAndServe(addr, nil)
	}()

	// 启动定时任务：持久化/聚合/清理监控数据到 Redis。
	// 修复：此前硬编码 5 分钟且永续运行，metrics_persistence.{enabled,interval,aggregate_enabled,
	// aggregate_interval,retention_hours} 均为暴露但未接线的预留字段；聚合与清理函数虽已实现却从未调度。
	if m.redisClient != nil && m.config.MetricsPersistence.Enabled {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()

			persistInterval := m.config.MetricsPersistence.Interval
			if persistInterval <= 0 {
				persistInterval = 5 * time.Minute
			}
			aggregateInterval := m.config.MetricsPersistence.AggregateInterval
			if aggregateInterval <= 0 {
				aggregateInterval = time.Hour
			}
			cleanupInterval := time.Hour

			persist := time.NewTicker(persistInterval)
			defer persist.Stop()
			aggregate := time.NewTicker(aggregateInterval)
			defer aggregate.Stop()
			cleanup := time.NewTicker(cleanupInterval)
			defer cleanup.Stop()

			for {
				select {
				case <-persist.C:
					if err := m.SaveMetricsToRedis(); err != nil {
						logging.DefaultLogger.Info("Failed to save metrics to Redis: %v\n", err)
					}
				case <-aggregate.C:
					if m.config.MetricsPersistence.AggregateEnabled {
						if err := m.AggregateMetricsToRedis(); err != nil {
							logging.DefaultLogger.Info("Failed to aggregate metrics to Redis: %v\n", err)
						}
					}
				case <-cleanup.C:
					if err := m.CleanupExpiredMetrics(); err != nil {
						logging.DefaultLogger.Info("Failed to cleanup expired metrics: %v\n", err)
					}
				case <-m.stopCh:
					return
				}
			}
		}()
	}

	// 启动规则引擎（常启：UI 配置的规则/渠道需要引擎在场才能生效；
	// 无规则无渠道时评估为空转、处理器 no-op，成本可忽略。文件 alerting.enabled
	// 开关在 provider 模式下已无实际意义——历史缺陷：该开关为 false 时 UI 规则永不评估）
	{
		// 设置默认规则
		m.setupDefaultRules()

		// 添加处理器（provider 模式：每次告警读取最新配置；无配置时处理器 no-op，
		// 因此注册不再依赖文件配置开关——UI 渠道/系统配置改动即时生效）
		webhookConfig := &alerting.WebhookConfig{
			URL:        m.config.Alerting.Notification.Webhook.URL,
			Method:     "POST",
			Timeout:    10 * time.Second,
			MaxRetries: 3,
			RetryDelay: 5 * time.Second,
			Secret:     m.config.Alerting.Notification.Webhook.Secret,
		}
		wh := alerting.NewWebhookHandler(webhookConfig)
		if m.notificationSource != nil {
			wh.SetConfigProvider(func() *alerting.WebhookConfig {
				if wc, _ := m.notificationSource(); wc != nil {
					return wc
				}
				return nil
			})
		}
		m.AddAlertHandler(wh)

		emailConfig := &alerting.EmailConfig{
			SMTPHost: m.config.Alerting.Notification.Email.SMTPHost,
			SMTPPort: m.config.Alerting.Notification.Email.SMTPPort,
			Username: m.config.Alerting.Notification.Email.Username,
			Password: m.config.Alerting.Notification.Email.Password,
			From:     m.config.Alerting.Notification.Email.From,
			To:       m.config.Alerting.Notification.Email.To,
			UseTLS:   true,
		}
		eh := alerting.NewEmailHandler(emailConfig)
		if m.notificationSource != nil {
			if notifier, ok := eh.(interface {
				SetConfigProvider(func() *alerting.EmailConfig)
			}); ok {
				notifier.SetConfigProvider(func() *alerting.EmailConfig {
					if _, ec := m.notificationSource(); ec != nil {
						return ec
					}
					return nil
				})
			}
		}
		m.AddAlertHandler(eh)

		// 修复（R11-BUG-4）：告警历史此前只有从未被调度的 legacy CheckAlerts 链路才会写入，
		// UI/文件规则经规则引擎触发后通知可发出但 Alert History 恒空。此处将历史持久化
		// 挂接为引擎 handler，与触发同链路（引擎已含冷却去重）。
		if m.alertRepo != nil {
			m.AddAlertHandler(&alertHistoryHandler{repo: m.alertRepo})
		}

		// 启动规则引擎
		m.ruleEngine.Start(m.metricsGetter)
	}

	m.isRunning = true
	return nil
}

// alertHistoryHandler 将引擎触发的告警写入历史存储（R11-BUG-4 修复组件）。
// 复用 AlertRepository 的键名/格式/裁剪逻辑，保证与 legacy 链路记录形态一致。
type alertHistoryHandler struct {
	repo *repository.AlertRepository
}

func (h *alertHistoryHandler) Name() string { return "history" }

func (h *alertHistoryHandler) Send(_ context.Context, alert *alerting.Alert) error {
	now := time.Now()
	h.repo.AppendAlertHistory(repository.AlertRecord{
		ID:        fmt.Sprintf("alert_%d", now.UnixNano()),
		Level:     alert.Severity,
		Rule:      alert.RuleName,
		Message:   alert.Message,
		Value:     alert.Value,
		Threshold: alert.Details["threshold"].(float64),
		Status:    "firing",
		Timestamp: now,
	})
	return nil
}

// Stop 停止监控服务
func (m *Monitor) Stop() error {
	if !m.isRunning {
		return nil
	}

	close(m.stopCh)
	m.wg.Wait()

	// 停止规则引擎
	if m.ruleEngine != nil {
		m.ruleEngine.Stop()
	}

	m.isRunning = false
	return nil
}

// isStaticResource 检查路径是否为静态资源
func isStaticResource(path string) bool {
	// 静态资源文件扩展名列表
	staticExtensions := []string{
		".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg",
		".css", ".less", ".sass", ".scss",
		".js", ".jsx", ".ts", ".tsx",
		".woff", ".woff2", ".ttf", ".eot",
		".ico", ".txt", ".json", ".xml", ".pdf", ".zip", ".rar",
		".mp4", ".mp3", ".avi", ".mov", ".wmv",
		".csv", ".xls", ".xlsx", ".doc", ".docx",
	}

	// 检查路径是否以静态资源扩展名结尾
	for _, ext := range staticExtensions {
		if len(path) >= len(ext) && path[len(path)-len(ext):] == ext {
			return true
		}
	}

	return false
}

// RecordRequest 记录请求，排除静态资源
func (m *Monitor) RecordRequest(method, path string, status int, duration time.Duration) {
	// 检查是否为静态资源，如果是则跳过记录
	if isStaticResource(path) {
		return
	}

	// 更新Prometheus指标，使用正确的字符串转换
	statusStr := fmt.Sprintf("%d", status)
	requestsTotal.WithLabelValues(method, path, statusStr).Inc()
	responseTime.WithLabelValues(method, path).Observe(duration.Seconds())

	// 更新实时统计数据
	statsStore.mu.Lock()
	statsStore.totalRequests++
	now := time.Now()
	if statsStore.firstRequestTime.IsZero() {
		statsStore.firstRequestTime = now
	}
	statsStore.lastRequestTime = now
	statsStore.mu.Unlock()
}

// RecordCrawlerRequest 记录爬虫请求
func (m *Monitor) RecordCrawlerRequest() {
	// 更新Prometheus指标
	crawlerRequests.Inc()

	// 更新实时统计数据
	statsStore.mu.Lock()
	statsStore.crawlerRequests++
	statsStore.mu.Unlock()
}

// RecordBlockedRequest 记录被阻止的请求
func (m *Monitor) RecordBlockedRequest() {
	// 更新Prometheus指标
	blockedRequests.Inc()

	// 更新实时统计数据
	statsStore.mu.Lock()
	statsStore.blockedRequests++
	statsStore.mu.Unlock()
}

// RecordCacheHit 记录缓存命中
func (m *Monitor) RecordCacheHit() {
	// 更新Prometheus指标
	cacheHits.Inc()

	// 更新实时统计数据
	statsStore.mu.Lock()
	statsStore.cacheHits++
	statsStore.mu.Unlock()
}

// RecordCacheMiss 记录缓存未命中
func (m *Monitor) RecordCacheMiss() {
	// 更新Prometheus指标
	cacheMisses.Inc()

	// 更新实时统计数据
	statsStore.mu.Lock()
	statsStore.cacheMisses++
	statsStore.mu.Unlock()
}

// SetActiveBrowsers 设置活跃浏览器数量
func (m *Monitor) SetActiveBrowsers(count int) {
	// 更新Prometheus指标
	activeBrowsers.Set(float64(count))

	// 更新实时统计数据
	statsStore.mu.Lock()
	statsStore.activeBrowsers = count
	statsStore.mu.Unlock()
}

// SetRenderQueueSize 设置渲染队列积压数量（供渲染引擎喂入，默认 0）
func (m *Monitor) SetRenderQueueSize(count int) {
	statsStore.mu.Lock()
	statsStore.renderQueueSize = count
	statsStore.mu.Unlock()
}

// RecordRenderTime 记录渲染时间
func (m *Monitor) RecordRenderTime(duration time.Duration) {
	renderTime.Observe(duration.Seconds())
}

// 实时统计数据存储
var statsStore = struct {
	mu              sync.Mutex
	totalRequests   int64
	crawlerRequests int64
	blockedRequests int64
	cacheHits       int64
	cacheMisses     int64
	activeBrowsers  int
	// 系统指标
	cpuUsage          float64
	memoryUsage       float64
	diskUsage         float64
	requestsPerSecond float64
	// 渲染队列
	renderQueueSize int
	// 时间跟踪
	firstRequestTime time.Time
	lastRequestTime  time.Time
}{
	totalRequests:   0,
	crawlerRequests: 0,
	blockedRequests: 0,
	cacheHits:       0,
	cacheMisses:     0,
	activeBrowsers:  0,
	// 系统指标初始化
	cpuUsage:          0,
	memoryUsage:       0,
	diskUsage:         0,
	requestsPerSecond: 0,
	renderQueueSize:   0,
}

// formatFloat 格式化浮点数，保留两位小数
func formatFloat(f float64) float64 {
	return float64(int(f*100)) / 100
}

// GetStats 获取统计数据
func (m *Monitor) GetStats() map[string]interface{} {
	statsStore.mu.Lock()
	defer statsStore.mu.Unlock()

	// 计算缓存命中率
	var cacheHitRate float64 = 0
	if statsStore.cacheHits+statsStore.cacheMisses > 0 {
		cacheHitRate = (float64(statsStore.cacheHits) / float64(statsStore.cacheHits+statsStore.cacheMisses)) * 100
		cacheHitRate = formatFloat(cacheHitRate)
	}

	// 获取真实系统指标数据
	cpuUsage, _ := getCPUUsage()
	memoryInfo, _ := getMemoryInfo()
	diskInfo, _ := getDiskInfo()
	netInfo, _ := getNetworkInfo()

	// 计算请求每秒
	var requestsPerSecond float64
	if !statsStore.firstRequestTime.IsZero() && statsStore.totalRequests > 0 {
		elapsed := time.Since(statsStore.firstRequestTime).Seconds()
		if elapsed > 0 {
			requestsPerSecond = formatFloat(float64(statsStore.totalRequests) / elapsed)
		}
	}

	// 安全获取内存指标
	var memoryUsage, memoryTotal, memoryUsed, memoryFree float64
	if memoryInfo != nil {
		memoryUsage = memoryInfo.UsagePercent
		memoryTotal = float64(memoryInfo.Total)
		memoryUsed = float64(memoryInfo.Used)
		memoryFree = float64(memoryInfo.Free)
	}

	// 安全获取磁盘指标
	var diskUsage, diskTotal, diskUsed, diskFree float64
	if diskInfo != nil {
		diskUsage = diskInfo.UsagePercent
		diskTotal = float64(diskInfo.Total)
		diskUsed = float64(diskInfo.Used)
		diskFree = float64(diskInfo.Free)
	}

	// 安全获取网络指标
	var networkSent, networkRecv, networkPacketsSent, networkPacketsRecv uint64
	if netInfo != nil {
		networkSent = netInfo.BytesSent
		networkRecv = netInfo.BytesRecv
		networkPacketsSent = netInfo.PacketsSent
		networkPacketsRecv = netInfo.PacketsRecv
	}

	return map[string]interface{}{
		"totalRequests":   float64(statsStore.totalRequests),
		"crawlerRequests": float64(statsStore.crawlerRequests),
		"blockedRequests": float64(statsStore.blockedRequests),
		"cacheHits":       float64(statsStore.cacheHits),
		"cacheMisses":     float64(statsStore.cacheMisses),
		"cacheHitRate":    cacheHitRate,
		"activeBrowsers":  float64(statsStore.activeBrowsers),
		"renderQueueSize": float64(statsStore.renderQueueSize),
		// 添加系统指标
		"cpuUsage":           cpuUsage,
		"memoryUsage":        memoryUsage,
		"memoryTotal":        memoryTotal,
		"memoryUsed":         memoryUsed,
		"memoryFree":         memoryFree,
		"diskUsage":          diskUsage,
		"diskTotal":          diskTotal,
		"diskUsed":           diskUsed,
		"diskFree":           diskFree,
		"requestsPerSecond":  requestsPerSecond,
		"networkSent":        networkSent,
		"networkRecv":        networkRecv,
		"networkPacketsSent": networkPacketsSent,
		"networkPacketsRecv": networkPacketsRecv,
	}
}

// getCPUUsage 获取CPU使用率
func getCPUUsage() (float64, error) {
	percent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return 0, err
	}
	if len(percent) > 0 {
		return formatFloat(percent[0]), nil
	}
	return 0, nil
}

// MemoryInfo 内存信息

type MemoryInfo struct {
	Total        uint64  `json:"total"`
	Used         uint64  `json:"used"`
	Free         uint64  `json:"free"`
	UsagePercent float64 `json:"usagePercent"`
}

// getMemoryInfo 获取内存信息
func getMemoryInfo() (*MemoryInfo, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	return &MemoryInfo{
		Total:        v.Total,
		Used:         v.Used,
		Free:         v.Free,
		UsagePercent: formatFloat(v.UsedPercent),
	}, nil
}

// DiskInfo 磁盘信息

type DiskInfo struct {
	Total        uint64  `json:"total"`
	Used         uint64  `json:"used"`
	Free         uint64  `json:"free"`
	UsagePercent float64 `json:"usagePercent"`
}

// getDiskInfo 获取磁盘信息
func getDiskInfo() (*DiskInfo, error) {
	// 获取根目录磁盘信息
	d, err := disk.Usage("/")
	if err != nil {
		return nil, err
	}
	return &DiskInfo{
		Total:        d.Total,
		Used:         d.Used,
		Free:         d.Free,
		UsagePercent: formatFloat(d.UsedPercent),
	}, nil
}

// NetworkInfo 网络信息

type NetworkInfo struct {
	BytesSent   uint64 `json:"bytesSent"`
	BytesRecv   uint64 `json:"bytesRecv"`
	PacketsSent uint64 `json:"packetsSent"`
	PacketsRecv uint64 `json:"packetsRecv"`
}

// getNetworkInfo 获取网络信息
func getNetworkInfo() (*NetworkInfo, error) {
	// 获取所有网络接口的IO统计
	ioCounters, err := net.IOCounters(false)
	if err != nil {
		return nil, err
	}
	if len(ioCounters) > 0 {
		return &NetworkInfo{
			BytesSent:   ioCounters[0].BytesSent,
			BytesRecv:   ioCounters[0].BytesRecv,
			PacketsSent: ioCounters[0].PacketsSent,
			PacketsRecv: ioCounters[0].PacketsRecv,
		}, nil
	}
	return &NetworkInfo{}, nil
}
