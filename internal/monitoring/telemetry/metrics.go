package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.uber.org/zap"
)

var (
	meterProvider  *sdkmetric.MeterProvider
	meter          metric.Meter
	metricsInitOnce sync.Once
	metricsLogger  *zap.Logger
)

// MetricsConfig 指标配置
type MetricsConfig struct {
	ServiceName    string        // 服务名称
	ServiceVersion string        // 服务版本
	Environment    string        // 环境
	Endpoint       string        // Prometheus 抓取端点
	EnableGoGC     bool          // 是否启用 Go GC 指标
	EnableProcess  bool          // 是否启用进程指标
	ReadTimeout    time.Duration // 读取超时
	WriteTimeout   time.Duration // 写入超时
}

// DefaultMetricsConfig 返回默认配置
func DefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		ServiceName:    "prerender-shield",
		ServiceVersion: getVersion(),
		Environment:    os.Getenv("ENVIRONMENT"),
		Endpoint:       ":9090",
		EnableGoGC:     true,
		EnableProcess:  true,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
	}
}

// Metrics 应用指标集合
type Metrics struct {
	// HTTP 请求指标
	HTTPRequestTotal    prometheus.CounterVec
	HTTPRequestDuration prometheus.HistogramVec
	HTTPActiveRequests  prometheus.GaugeVec
	HTTPResponseSize    prometheus.HistogramVec

	// 业务指标
	CrawlerQueueSize    prometheus.GaugeVec
	CacheHitTotal       prometheus.CounterVec
	CacheMissTotal      prometheus.CounterVec
	WAFBlockedTotal     prometheus.CounterVec
	DDoSDetectedTotal   prometheus.CounterVec
	SSLCertExpiryDays   prometheus.GaugeVec
	RenderDuration      prometheus.HistogramVec
	RenderErrorTotal    prometheus.CounterVec

	// OTel 指标
	RequestCounter metric.Int64Counter
	LatencyHist    metric.Float64Histogram
	ActiveGauge    metric.Int64ObservableGauge
}

// InitMetrics 初始化指标收集系统
func InitMetrics(cfg *MetricsConfig, log *zap.Logger) (*Metrics, error) {
	if cfg == nil {
		cfg = DefaultMetricsConfig()
	}
	if log == nil {
		log = zap.NewNop()
	}
	metricsLogger = log

	var metrics *Metrics
	var initErr error

	metricsInitOnce.Do(func() {
		// 创建资源
		res, err := newMetricsResource(cfg)
		if err != nil {
			initErr = fmt.Errorf("创建指标资源失败：%w", err)
			return
		}

		// 创建 MeterProvider
		// 使用手动 reader，指标通过 Prometheus 端点暴露
		meterProvider = sdkmetric.NewMeterProvider(
			sdkmetric.WithResource(res),
		)

		otel.SetMeterProvider(meterProvider)

		// 获取 Meter
		meter = meterProvider.Meter(cfg.ServiceName,
			metric.WithInstrumentationVersion(cfg.ServiceVersion),
		)

		// 创建 Prometheus 注册表
		reg := prometheus.NewRegistry()

		// 注册默认收集器
		if cfg.EnableGoGC {
			reg.MustRegister(collectors.NewGoCollector())
		}
		if cfg.EnableProcess {
			reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
		}

		// 创建指标
		metrics = newMetrics(reg)

		// 启动 HTTP 服务器
		if err := startMetricsServer(cfg.Endpoint, reg, cfg.ReadTimeout, cfg.WriteTimeout); err != nil {
			initErr = fmt.Errorf("启动指标服务器失败：%w", err)
			return
		}

		metricsLogger.Info("指标收集系统初始化成功",
			zap.String("service", cfg.ServiceName),
			zap.String("endpoint", cfg.Endpoint),
		)
	})

	return metrics, initErr
}

// newMetricsResource 创建指标资源
func newMetricsResource(cfg *MetricsConfig) (*resource.Resource, error) {
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
			attribute.String("service.instance.id", getInstanceID()),
		),
	)
}

// newMetrics 创建指标集合
func newMetrics(reg *prometheus.Registry) *Metrics {
	m := &Metrics{
		// HTTP 请求总数
		HTTPRequestTotal: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "HTTP 请求总数",
			},
			[]string{"method", "path", "status", "handler"},
		),
		// HTTP 请求延迟
		HTTPRequestDuration: *prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP 请求延迟 (秒)",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"method", "path", "status"},
		),
		// 活跃请求数
		HTTPActiveRequests: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "http_requests_active",
				Help: "当前活跃请求数",
			},
			[]string{},
		),
		// HTTP 响应大小
		HTTPResponseSize: *prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_response_size_bytes",
				Help:    "HTTP 响应大小 (字节)",
				Buckets: []float64{100, 500, 1000, 5000, 10000, 50000, 100000, 500000, 1000000},
			},
			[]string{"method", "path"},
		),
		// 爬虫队列大小
		CrawlerQueueSize: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "crawler_queue_size",
				Help: "爬虫队列当前大小",
			},
			[]string{},
		),
		// 缓存命中数
		CacheHitTotal: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_hits_total",
				Help: "缓存命中总数",
			},
			[]string{},
		),
		// 缓存未命中数
		CacheMissTotal: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_misses_total",
				Help: "缓存未命中总数",
			},
			[]string{},
		),
		// WAF 拦截数
		WAFBlockedTotal: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "waf_blocked_total",
				Help: "WAF 拦截请求总数",
			},
			[]string{"rule", "reason", "action"},
		),
		// DDoS 检测数
		DDoSDetectedTotal: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ddos_detected_total",
				Help: "DDoS 攻击检测总数",
			},
			[]string{"detector", "attack_type", "severity"},
		),
		// SSL 证书过期天数
		SSLCertExpiryDays: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ssl_cert_expiry_days",
				Help: "SSL 证书过期剩余天数",
			},
			[]string{"domain", "issuer"},
		),
		// 渲染延迟
		RenderDuration: *prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "render_duration_seconds",
				Help:    "页面渲染延迟 (秒)",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
			},
			[]string{"url", "status", "cache_hit"},
		),
		// 渲染错误数
		RenderErrorTotal: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "render_errors_total",
				Help: "渲染错误总数",
			},
			[]string{"url", "error_type"},
		),
	}

	// 注册所有指标
	reg.MustRegister(
		&m.HTTPRequestTotal,
		&m.HTTPRequestDuration,
		&m.HTTPActiveRequests,
		&m.HTTPResponseSize,
		&m.CrawlerQueueSize,
		m.CacheHitTotal,
		m.CacheMissTotal,
		&m.WAFBlockedTotal,
		&m.DDoSDetectedTotal,
		&m.SSLCertExpiryDays,
		&m.RenderDuration,
		&m.RenderErrorTotal,
	)

	// 创建 OTel 指标
	var err error
	m.RequestCounter, err = meter.Int64Counter("http_requests",
		metric.WithDescription("HTTP 请求总数"),
		metric.WithUnit("1"),
	)
	if err != nil {
		metricsLogger.Warn("创建请求计数器失败", zap.Error(err))
	}

	m.LatencyHist, err = meter.Float64Histogram("http_request_duration",
		metric.WithDescription("HTTP 请求延迟"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	)
	if err != nil {
		metricsLogger.Warn("创建延迟直方图失败", zap.Error(err))
	}

	m.ActiveGauge, err = meter.Int64ObservableGauge("http_requests_active",
		metric.WithDescription("当前活跃请求数"),
		metric.WithUnit("1"),
	)
	if err != nil {
		metricsLogger.Warn("创建活跃请求 Gauge 失败", zap.Error(err))
	}

	return m
}

// startMetricsServer 启动指标 HTTP 服务器
func startMetricsServer(endpoint string, reg *prometheus.Registry, readTimeout, writeTimeout time.Duration) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))

	// 健康检查端点
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:         endpoint,
		Handler:      mux,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	go func() {
		metricsLogger.Info("启动指标 HTTP 服务器", zap.String("endpoint", endpoint))
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			metricsLogger.Error("指标服务器异常", zap.Error(err))
		}
	}()

	return nil
}

// GetMeter 获取全局 Meter
func GetMeter() metric.Meter {
	if meter == nil {
		return noop.NewMeterProvider().Meter("")
	}
	return meter
}

// ShutdownMetrics 关闭指标系统
func ShutdownMetrics() error {
	if meterProvider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return meterProvider.Shutdown(ctx)
	}
	return nil
}

// GetMetricsRegistry 获取 Prometheus 注册表 (用于测试)
func GetMetricsRegistry(reg *prometheus.Registry) *prometheus.Registry {
	return reg
}

// 便捷方法 - HTTP 指标记录

// RecordHTTPRequest 记录 HTTP 请求指标
func RecordHTTPRequest(method, path, status, handler string, duration float64, size int) {
	labels := prometheus.Labels{
		"method":  method,
		"path":    path,
		"status":  status,
		"handler": handler,
	}

	// Prometheus 指标
	metricsInstance.HTTPRequestTotal.With(labels).Inc()
	metricsInstance.HTTPRequestDuration.With(prometheus.Labels{
		"method": method,
		"path":   path,
		"status": status,
	}).Observe(duration)

	// OTel 指标
	ctx := context.Background()
	metricsInstance.RequestCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("method", method),
			attribute.String("path", path),
			attribute.String("status", status),
		),
	)
	metricsInstance.LatencyHist.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("method", method),
			attribute.String("path", path),
		),
	)
}

// RecordWAFBlock 记录 WAF 拦截事件
func RecordWAFBlock(rule, reason, action string) {
	metricsInstance.WAFBlockedTotal.With(prometheus.Labels{
		"rule":   rule,
		"reason": reason,
		"action": action,
	}).Inc()
}

// RecordDDoSDetection 记录 DDoS 检测事件
func RecordDDoSDetection(detector, attackType, severity string) {
	metricsInstance.DDoSDetectedTotal.With(prometheus.Labels{
		"detector":   detector,
		"attack_type": attackType,
		"severity":   severity,
	}).Inc()
}

// RecordCacheHit 记录缓存命中
func RecordCacheHit() {
	metricsInstance.CacheHitTotal.With(prometheus.Labels{}).Inc()
}

// RecordCacheMiss 记录缓存未命中
func RecordCacheMiss() {
	metricsInstance.CacheMissTotal.With(prometheus.Labels{}).Inc()
}

// SetCrawlerQueueSize 设置爬虫队列大小
func SetCrawlerQueueSize(size int) {
	metricsInstance.CrawlerQueueSize.With(prometheus.Labels{}).Set(float64(size))
}

// RecordRenderDuration 记录渲染延迟
func RecordRenderDuration(url, status, cacheHit string, duration float64) {
	metricsInstance.RenderDuration.With(prometheus.Labels{
		"url":       url,
		"status":    status,
		"cache_hit": cacheHit,
	}).Observe(duration)
}

// RecordRenderError 记录渲染错误
func RecordRenderError(url, errorType string) {
	metricsInstance.RenderErrorTotal.With(prometheus.Labels{
		"url":       url,
		"error_type": errorType,
	}).Inc()
}

// SetSSLCertExpiryDays 设置 SSL 证书过期天数
func SetSSLCertExpiryDays(domain, issuer string, days float64) {
	metricsInstance.SSLCertExpiryDays.With(prometheus.Labels{
		"domain": domain,
		"issuer": issuer,
	}).Set(days)
}

// IncActiveRequests 增加活跃请求数
func IncActiveRequests() {
	metricsInstance.HTTPActiveRequests.With(prometheus.Labels{}).Inc()
}

// DecActiveRequests 减少活跃请求数
func DecActiveRequests() {
	metricsInstance.HTTPActiveRequests.With(prometheus.Labels{}).Dec()
}

var metricsInstance *Metrics
