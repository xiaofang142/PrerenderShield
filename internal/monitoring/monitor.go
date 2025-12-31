package monitoring

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	isRunning bool
	config    Config
	wg        sync.WaitGroup
	stopCh    chan struct{}
}

// Config 监控配置
type Config struct {
	Enabled           bool
	PrometheusAddress string
}

// NewMonitor 创建新的监控管理器
func NewMonitor(config Config) *Monitor {
	return &Monitor{
		isRunning: false,
		config:    config,
		stopCh:    make(chan struct{}),
	}
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
	go func() {
		m.wg.Add(1)
		defer m.wg.Done()

		http.Handle("/metrics", promhttp.Handler())
		// 使用配置中的地址，默认使用:9090
		addr := m.config.PrometheusAddress
		if addr == "" {
			addr = ":9090"
		}
		http.ListenAndServe(addr, nil)
	}()

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
}{
	totalRequests:   0,
	crawlerRequests: 0,
	blockedRequests: 0,
	cacheHits:       0,
	cacheMisses:     0,
	activeBrowsers:  0,
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

	return map[string]interface{}{
		"totalRequests":   float64(statsStore.totalRequests),
		"crawlerRequests": float64(statsStore.crawlerRequests),
		"blockedRequests": float64(statsStore.blockedRequests),
		"cacheHits":       float64(statsStore.cacheHits),
		"cacheMisses":     float64(statsStore.cacheMisses),
		"cacheHitRate":    cacheHitRate,
		"activeBrowsers":  float64(statsStore.activeBrowsers),
	}
}
