package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

	"prerender-shield/internal/monitoring/alerting"
	"prerender-shield/internal/redis"
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
)

// Monitor 监控管理器
type Monitor struct {
	isRunning     bool
	config        Config
	redisClient   *redis.Client
	alerts        map[string]*AlertStatus // 告警状态
	alertMutex    sync.RWMutex
	wg            sync.WaitGroup
	stopCh        chan struct{}
	ruleEngine    *alerting.RuleEngine    // 告警规则引擎
	metricsGetter alerting.MetricsFunc    // 指标获取函数
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
	Enabled         bool
	Interval        time.Duration // 持久化间隔
	Retention       time.Duration // 数据保留时间
	AggregateEnabled bool
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
		if value, ok := stats[metric]; ok {
			if floatValue, ok := value.(float64); ok {
				return floatValue, nil
			}
		}
		return 0, fmt.Errorf("metric %s not found", metric)
	}
	return m
}

// LoadAlertRules 从文件加载告警规则
func (m *Monitor) LoadAlertRules(filename string) error {
	return m.ruleEngine.LoadRulesFromFile(filename)
}

// AddAlertHandler 添加告警处理器
func (m *Monitor) AddAlertHandler(handler alerting.AlertHandler) {
	m.ruleEngine.AddHandler(handler)
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
	// 发送邮件通知
	if m.config.Alerting.Notification.Email.Enabled {
		m.sendEmailNotification(alert, status)
	}

	// 发送Webhook通知
	if m.config.Alerting.Notification.Webhook.Enabled {
		m.sendWebhookNotification(alert, status)
	}
}

// sendEmailNotification 发送邮件通知
func (m *Monitor) sendEmailNotification(alert *AlertStatus, status string) {
	// 邮件发送逻辑
	// 这里只是一个示例，实际实现需要使用SMTP客户端
	emailConfig := m.config.Alerting.Notification.Email
	fmt.Printf("Sending email notification: %s - %s\n", alert.Rule.Name, status)
	fmt.Printf("To: %v\n", emailConfig.To)
	fmt.Printf("Subject: [%s] %s - %s\n", alert.Rule.Severity, alert.Rule.Name, status)
	fmt.Printf("Message: Metric %s is %s threshold %.2f (current value: %.2f)\n",
		alert.Rule.Metric, alert.Rule.Operator, alert.Rule.Threshold, alert.Value)
}

// sendWebhookNotification 发送Webhook通知
func (m *Monitor) sendWebhookNotification(alert *AlertStatus, status string) {
	// Webhook发送逻辑
	// 这里只是一个示例，实际实现需要使用HTTP客户端
	webhookConfig := m.config.Alerting.Notification.Webhook
	fmt.Printf("Sending webhook notification: %s - %s\n", alert.Rule.Name, status)
	fmt.Printf("URL: %s\n", webhookConfig.URL)
	fmt.Printf("Payload: {\"rule\": \"%s\", \"status\": \"%s\", \"severity\": \"%s\", \"value\": %.2f, \"threshold\": %.2f}\n",
		alert.Rule.Name, status, alert.Rule.Severity, alert.Value, alert.Rule.Threshold)
}

// SetRedisClient 设置Redis客户端
func (m *Monitor) SetRedisClient(client *redis.Client) {
	m.redisClient = client
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

	// 从Redis中获取每个键对应的数据
	metrics := make([]map[string]interface{}, 0, len(keys))
	for _, key := range keys {
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
			// 注意：这里假设有 Delete 方法，如果没有需要实现
			// 使用 Keys + Del 模式清理
			deletedCount++
		}
	}

	if deletedCount > 0 {
		log.Printf("Cleaned up %d expired metrics entries", deletedCount)
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

	// 启动定时任务，定期保存监控数据到Redis
	if m.redisClient != nil {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()

			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					err := m.SaveMetricsToRedis()
					if err != nil {
						fmt.Printf("Failed to save metrics to Redis: %v\n", err)
					}
				case <-m.stopCh:
					return
				}
			}
		}()
	}

	// 启动定时任务，定期检查告警
	if m.config.Alerting.Enabled {
		// 设置默认规则
		m.setupDefaultRules()

		// 添加处理器
		if m.config.Alerting.Notification.Webhook.Enabled {
			webhookConfig := &alerting.WebhookConfig{
				URL:        m.config.Alerting.Notification.Webhook.URL,
				Method:     "POST",
				Timeout:    10 * time.Second,
				MaxRetries: 3,
				RetryDelay: 5 * time.Second,
				Secret:     m.config.Alerting.Notification.Webhook.Secret,
			}
			m.AddAlertHandler(alerting.NewWebhookHandler(webhookConfig))
		}

		if m.config.Alerting.Notification.Email.Enabled {
			emailConfig := &alerting.EmailConfig{
				SMTPHost: m.config.Alerting.Notification.Email.SMTPHost,
				SMTPPort: m.config.Alerting.Notification.Email.SMTPPort,
				Username: m.config.Alerting.Notification.Email.Username,
				Password: m.config.Alerting.Notification.Email.Password,
				From:     m.config.Alerting.Notification.Email.From,
				To:       m.config.Alerting.Notification.Email.To,
				UseTLS:   true,
			}
			m.AddAlertHandler(alerting.NewEmailHandler(emailConfig))
		}

		// 启动规则引擎
		m.ruleEngine.Start(m.metricsGetter)
	}

	m.isRunning = true
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
	requestsPerSecond := formatFloat(float64(statsStore.totalRequests) / 1000)

	return map[string]interface{}{
		"totalRequests":   float64(statsStore.totalRequests),
		"crawlerRequests": float64(statsStore.crawlerRequests),
		"blockedRequests": float64(statsStore.blockedRequests),
		"cacheHits":       float64(statsStore.cacheHits),
		"cacheMisses":     float64(statsStore.cacheMisses),
		"cacheHitRate":    cacheHitRate,
		"activeBrowsers":  float64(statsStore.activeBrowsers),
		// 添加系统指标
		"cpuUsage":           cpuUsage,
		"memoryUsage":        memoryInfo.UsagePercent,
		"memoryTotal":        memoryInfo.Total,
		"memoryUsed":         memoryInfo.Used,
		"memoryFree":         memoryInfo.Free,
		"diskUsage":          diskInfo.UsagePercent,
		"diskTotal":          diskInfo.Total,
		"diskUsed":           diskInfo.Used,
		"diskFree":           diskInfo.Free,
		"requestsPerSecond":  requestsPerSecond,
		"networkSent":        netInfo.BytesSent,
		"networkRecv":        netInfo.BytesRecv,
		"networkPacketsSent": netInfo.PacketsSent,
		"networkPacketsRecv": netInfo.PacketsRecv,
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
