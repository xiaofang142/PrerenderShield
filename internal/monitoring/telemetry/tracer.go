package telemetry

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var (
	tracerProvider *sdktrace.TracerProvider
	tracer         trace.Tracer
	initOnce       sync.Once
	logger         *zap.Logger
)

// TracerConfig 追踪器配置
type TracerConfig struct {
	ServiceName    string        // 服务名称
	ServiceVersion string        // 服务版本
	Environment    string        // 环境 (development, staging, production)
	Endpoint       string        // OTLP 导出端点
	UseStdout      bool          // 是否使用标准输出导出
	SampleRatio    float64       // 采样率 (0.0-1.0)
	Timeout        time.Duration // 导出超时时间
}

// DefaultTracerConfig 返回默认配置
func DefaultTracerConfig() *TracerConfig {
	return &TracerConfig{
		ServiceName:    "prerender-shield",
		ServiceVersion: getVersion(),
		Environment:    os.Getenv("ENVIRONMENT"),
		Endpoint:       os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		UseStdout:      os.Getenv("OTEL_TRACES_EXPORTER") == "console",
		SampleRatio:    1.0,
		Timeout:        10 * time.Second,
	}
}

func getVersion() string {
	if version := os.Getenv("SERVICE_VERSION"); version != "" {
		return version
	}
	return "dev"
}

// InitTracer 初始化分布式追踪系统
func InitTracer(cfg *TracerConfig, log *zap.Logger) error {
	if cfg == nil {
		cfg = DefaultTracerConfig()
	}
	if log == nil {
		log = zap.NewNop()
	}
	logger = log

	var initErr error
	initOnce.Do(func() {
		// 创建资源
		res, err := newResource(cfg)
		if err != nil {
			initErr = fmt.Errorf("创建 OTel 资源失败：%w", err)
			return
		}

		// 创建采样器
		sampler := sdktrace.ParentBased(
			sdktrace.TraceIDRatioBased(cfg.SampleRatio),
		)

		// 创建导出器
		exporter, err := newExporter(cfg)
		if err != nil {
			initErr = fmt.Errorf("创建导出器失败：%w", err)
			return
		}

		// 创建 TracerProvider
		tracerProvider = sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sampler),
			sdktrace.WithBatcher(exporter,
				sdktrace.WithBatchTimeout(5*time.Second),
				sdktrace.WithMaxExportBatchSize(512),
			),
			sdktrace.WithSpanProcessor(&errorRecorderSpanProcessor{}),
		)

		// 设置全局 TracerProvider
		otel.SetTracerProvider(tracerProvider)

		// 设置全局 Propagator
		otel.SetTextMapPropagator(
			propagation.NewCompositeTextMapPropagator(
				propagation.TraceContext{},
				propagation.Baggage{},
			),
		)

		// 获取 Tracer
		tracer = otel.Tracer(cfg.ServiceName,
			trace.WithInstrumentationVersion(cfg.ServiceVersion),
		)

		logger.Info("分布式追踪初始化成功",
			zap.String("service", cfg.ServiceName),
			zap.String("version", cfg.ServiceVersion),
			zap.String("environment", cfg.Environment),
			zap.String("endpoint", cfg.Endpoint),
			zap.Float64("sampleRatio", cfg.SampleRatio),
		)
	})

	return initErr
}

// newResource 创建 OTel 资源
func newResource(cfg *TracerConfig) (*resource.Resource, error) {
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
			semconv.HostName(getHostname()),
			attribute.String("service.instance.id", getInstanceID()),
		),
	)
}

func getHostname() string {
	if hostname := os.Getenv("HOSTNAME"); hostname != "" {
		return hostname
	}
	h, _ := os.Hostname()
	if h == "" {
		h = "unknown"
	}
	return h
}

func getInstanceID() string {
	if id := os.Getenv("SERVICE_INSTANCE_ID"); id != "" {
		return id
	}
	return getHostname()
}

// newExporter 创建导出器
func newExporter(cfg *TracerConfig) (sdktrace.SpanExporter, error) {
	if cfg.UseStdout {
		return stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
		)
	}

	if cfg.Endpoint == "" {
		// 如果没有配置端点，使用标准输出
		return stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.Endpoint),
		otlptracehttp.WithTimeout(cfg.Timeout),
	)
	if err != nil {
		return nil, err
	}

	return exporter, nil
}

// GetTracer 获取全局 Tracer
func GetTracer() trace.Tracer {
	if tracer == nil {
		// 如果未初始化，返回一个 Noop Tracer
		return trace.NewNoopTracerProvider().Tracer("noop")
	}
	return tracer
}

// ShutdownTracer 关闭追踪器
func ShutdownTracer() error {
	if tracerProvider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return tracerProvider.Shutdown(ctx)
	}
	return nil
}

// StartSpan 开始一个新的 Span
func StartSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return GetTracer().Start(ctx, spanName, opts...)
}

// RecordError 记录错误到 Span
func RecordError(span trace.Span, err error, opts ...trace.EventOption) {
	if span == nil || err == nil {
		return
	}
	span.RecordError(err, opts...)
	span.SetStatus(1, err.Error()) // Error status
}

// SetSpanAttributes 设置 Span 属性
func SetSpanAttributes(span trace.Span, attributes ...attribute.KeyValue) {
	if span == nil {
		return
	}
	span.SetAttributes(attributes...)
}

// 常用属性键
var (
	AttrHTTPMethod     = semconv.HTTPMethodKey
	AttrHTTPURL        = semconv.HTTPURLKey
	AttrHTTPTarget     = semconv.HTTPTargetKey
	AttrHTTPStatusCode = semconv.HTTPStatusCodeKey
	AttrHTTPRoute      = semconv.HTTPRouteKey
	AttrHTTPClientIP   = semconv.ClientAddressKey
	AttrHTTPUserAgent  = semconv.UserAgentOriginalKey
	AttrHTTPScheme     = semconv.URLSchemeKey
	AttrNetHostName    = semconv.ServerAddressKey
	AttrNetHostPort    = semconv.ServerPortKey
	AttrNetPeerIP      = semconv.NetworkPeerAddressKey
	AttrNetPeerPort    = semconv.NetworkPeerPortKey
	AttrNetPeerName    = semconv.NetPeerNameKey
	AttrDBSystem       = semconv.DBSystemKey
	AttrErrorType      = semconv.ErrorTypeKey
)

// errorRecorderSpanProcessor 错误记录 Span 处理器
type errorRecorderSpanProcessor struct{}

func (p *errorRecorderSpanProcessor) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {}

func (p *errorRecorderSpanProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	// 可以在 Span 结束时进行额外处理
}

func (p *errorRecorderSpanProcessor) Shutdown(ctx context.Context) error {
	return nil
}

func (p *errorRecorderSpanProcessor) ForceFlush(ctx context.Context) error {
	return nil
}
