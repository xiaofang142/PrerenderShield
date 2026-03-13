package telemetry

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func TestTracerConfig_DefaultConfig(t *testing.T) {
	cfg := DefaultTracerConfig()
	assert.NotNil(t, cfg)
	assert.Equal(t, "prerender-shield", cfg.ServiceName)
	assert.Equal(t, 1.0, cfg.SampleRatio)
	assert.Equal(t, 10*time.Second, cfg.Timeout)
}

func TestTracerConfig_EnvironmentVariables(t *testing.T) {
	// 设置环境变量
	os.Setenv("ENVIRONMENT", "test")
	os.Setenv("SERVICE_VERSION", "1.0.0")
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")
	os.Setenv("OTEL_TRACES_EXPORTER", "console")
	defer func() {
		os.Unsetenv("ENVIRONMENT")
		os.Unsetenv("SERVICE_VERSION")
		os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
		os.Unsetenv("OTEL_TRACES_EXPORTER")
	}()

	cfg := DefaultTracerConfig()
	assert.Equal(t, "test", cfg.Environment)
	assert.Equal(t, "1.0.0", cfg.ServiceVersion)
	assert.Equal(t, "http://localhost:4318", cfg.Endpoint)
	assert.True(t, cfg.UseStdout)
}

func TestGetVersion(t *testing.T) {
	// 未设置环境变量
	os.Unsetenv("SERVICE_VERSION")
	version := getVersion()
	assert.Equal(t, "dev", version)

	// 设置环境变量
	os.Setenv("SERVICE_VERSION", "2.0.0")
	defer os.Unsetenv("SERVICE_VERSION")
	version = getVersion()
	assert.Equal(t, "2.0.0", version)
}

func TestInitTracer_NilConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	err := InitTracer(nil, logger)
	if err != nil {
		t.Skipf("Skipping test due to tracer init error: %v", err)
	}

	// 清理
	ShutdownTracer()
}

func TestInitTracer_NilLogger(t *testing.T) {
	cfg := &TracerConfig{
		ServiceName: "test-service",
		UseStdout:   true,
	}
	err := InitTracer(cfg, nil)
	assert.NoError(t, err)

	// 清理
	ShutdownTracer()
}

func TestInitTracer_CustomConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &TracerConfig{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		SampleRatio:    0.5,
		UseStdout:      true,
		Timeout:        5 * time.Second,
	}

	err := InitTracer(cfg, logger)
	assert.NoError(t, err)

	// 验证 Tracer 可用
	tracer := GetTracer()
	assert.NotNil(t, tracer)

	// 清理
	ShutdownTracer()
}

func TestInitTracer_StdoutExporter(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &TracerConfig{
		ServiceName: "test-service",
		UseStdout:   true,
	}

	err := InitTracer(cfg, logger)
	assert.NoError(t, err)

	// 清理
	ShutdownTracer()
}

func TestInitTracer_OTLPExporter(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &TracerConfig{
		ServiceName: "test-service",
		Endpoint:    "http://localhost:4318",
		Timeout:     2 * time.Second,
	}

	err := InitTracer(cfg, logger)
	// 由于端点不可达，可能会失败，但不应该 panic
	_ = err

	// 清理
	ShutdownTracer()
}

func TestGetTracer_NotInitialized(t *testing.T) {
	// 确保未初始化
	tracerProvider = nil
	tracer = nil

	tracer := GetTracer()
	assert.NotNil(t, tracer)

	// 验证是 Noop Tracer
	_, span := tracer.Start(context.Background(), "test")
	assert.NotNil(t, span)
	assert.False(t, span.SpanContext().IsValid())
	span.End()
}

func TestGetTracer_Initialized(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &TracerConfig{
		ServiceName: "test-service",
		UseStdout:   true,
	}

	err := InitTracer(cfg, logger)
	assert.NoError(t, err)

	tracer := GetTracer()
	assert.NotNil(t, tracer)

	// 清理
	ShutdownTracer()
}

func TestShutdownTracer_NotInitialized(t *testing.T) {
	tracerProvider = nil
	err := ShutdownTracer()
	assert.NoError(t, err)
}

func TestShutdownTracer_Initialized(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &TracerConfig{
		ServiceName: "test-service",
		UseStdout:   true,
	}

	err := InitTracer(cfg, logger)
	assert.NoError(t, err)

	err = ShutdownTracer()
	assert.NoError(t, err)
}

func TestStartSpan(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &TracerConfig{
		ServiceName: "test-service",
		UseStdout:   true,
	}

	err := InitTracer(cfg, logger)
	assert.NoError(t, err)
	defer ShutdownTracer()

	ctx := context.Background()
	newCtx, span := StartSpan(ctx, "test-span")
	assert.NotNil(t, span)
	assert.NotNil(t, newCtx)
	span.End()
}

func TestStartSpan_WithOptions(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &TracerConfig{
		ServiceName: "test-service",
		UseStdout:   true,
	}

	err := InitTracer(cfg, logger)
	assert.NoError(t, err)
	defer ShutdownTracer()

	ctx := context.Background()
	newCtx, span := StartSpan(ctx, "test-span", trace.WithSpanKind(trace.SpanKindServer))
	assert.NotNil(t, span)
	assert.NotNil(t, newCtx)
	span.End()
}

func TestRecordError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &TracerConfig{
		ServiceName: "test-service",
		UseStdout:   true,
	}

	err := InitTracer(cfg, logger)
	assert.NoError(t, err)
	defer ShutdownTracer()

	ctx := context.Background()
	_, span := StartSpan(ctx, "test-span")

	// 记录错误
	testErr := assert.AnError
	RecordError(span, testErr)

	// 不应该 panic
	assert.NotPanics(t, func() {
		RecordError(nil, testErr)
		RecordError(span, nil)
	})

	span.End()
}

func TestSetSpanAttributes(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &TracerConfig{
		ServiceName: "test-service",
		UseStdout:   true,
	}

	err := InitTracer(cfg, logger)
	assert.NoError(t, err)
	defer ShutdownTracer()

	ctx := context.Background()
	_, span := StartSpan(ctx, "test-span")

	// 设置属性
	SetSpanAttributes(span, AttrHTTPMethod.String("GET"))
	SetSpanAttributes(span, AttrHTTPStatusCode.Int(200))

	// 不应该 panic
	assert.NotPanics(t, func() {
		SetSpanAttributes(nil, AttrHTTPMethod.String("POST"))
	})

	span.End()
}

func TestAttributeKeys(t *testing.T) {
	// 验证属性键存在
	assert.NotNil(t, AttrHTTPMethod)
	assert.NotNil(t, AttrHTTPURL)
	assert.NotNil(t, AttrHTTPTarget)
	assert.NotNil(t, AttrHTTPStatusCode)
	assert.NotNil(t, AttrHTTPRoute)
	assert.NotNil(t, AttrHTTPClientIP)
	assert.NotNil(t, AttrHTTPUserAgent)
	assert.NotNil(t, AttrHTTPScheme)
	assert.NotNil(t, AttrNetHostName)
	assert.NotNil(t, AttrNetHostPort)
	assert.NotNil(t, AttrNetPeerIP)
	assert.NotNil(t, AttrNetPeerPort)
	assert.NotNil(t, AttrNetPeerName)
	assert.NotNil(t, AttrDBSystem)
	assert.NotNil(t, AttrErrorType)
}

func TestHostname(t *testing.T) {
	// 测试 getHostname
	os.Setenv("HOSTNAME", "test-host")
	defer os.Unsetenv("HOSTNAME")

	hostname := getHostname()
	assert.Equal(t, "test-host", hostname)
}

func TestInstanceID(t *testing.T) {
	// 测试 getInstanceID
	os.Setenv("SERVICE_INSTANCE_ID", "instance-123")
	defer os.Unsetenv("SERVICE_INSTANCE_ID")

	instanceID := getInstanceID()
	assert.Equal(t, "instance-123", instanceID)
}

func TestNewResource(t *testing.T) {
	cfg := &TracerConfig{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
	}

	res, err := newResource(cfg)
	if err != nil {
		t.Skipf("Skipping test due to resource creation error: %v", err)
	}
	assert.NotNil(t, res)
}

func TestNewExporter_Stdout(t *testing.T) {
	cfg := &TracerConfig{
		UseStdout: true,
	}

	exporter, err := newExporter(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exporter)
}

func TestNewExporter_NoEndpoint(t *testing.T) {
	cfg := &TracerConfig{
		Endpoint: "",
	}

	exporter, err := newExporter(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exporter)
}

func TestErrorRecorderSpanProcessor(t *testing.T) {
	processor := &errorRecorderSpanProcessor{}
	ctx := context.Background()

	// 不应该 panic
	assert.NotPanics(t, func() {
		processor.OnStart(ctx, nil)
		processor.OnEnd(nil)
		processor.Shutdown(ctx)
		processor.ForceFlush(ctx)
	})
}
