package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func TestNewMetricsDirect(t *testing.T) {
	oldMeterProvider := meterProvider
	oldMeter := meter
	meterProvider = nil
	meter = nil
	t.Cleanup(func() {
		meterProvider = oldMeterProvider
		meter = oldMeter
	})

	cfg := &MetricsConfig{
		ServiceName: "test-new-metrics",
		Endpoint:    ":0",
	}

	res, err := newMetricsResource(cfg)
	if err != nil {
		t.Skipf("Resource error: %v", err)
	}
	meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
	)
	meter = meterProvider.Meter(cfg.ServiceName)

	reg := prometheus.NewRegistry()
	metrics := newMetrics(reg)
	require.NotNil(t, metrics)
	assert.NotNil(t, metrics.HTTPRequestTotal)
	assert.NotNil(t, metrics.HTTPRequestDuration)
	assert.NotNil(t, metrics.HTTPActiveRequests)
	assert.NotNil(t, metrics.HTTPResponseSize)
	assert.NotNil(t, metrics.CrawlerQueueSize)
	assert.NotNil(t, metrics.CacheHitTotal)
	assert.NotNil(t, metrics.WAFBlockedTotal)
	assert.NotNil(t, metrics.DDoSDetectedTotal)
	assert.NotNil(t, metrics.SSLCertExpiryDays)
	assert.NotNil(t, metrics.RenderDuration)
	assert.NotNil(t, metrics.RenderErrorTotal)
}

func TestRecordFunctionsWithMetrics(t *testing.T) {
	oldInstance := metricsInstance
	oldMeterProvider := meterProvider
	oldMeter := meter
	t.Cleanup(func() {
		metricsInstance = oldInstance
		meterProvider = oldMeterProvider
		meter = oldMeter
	})

	cfg := &MetricsConfig{
		ServiceName: "test-record-funcs",
		Endpoint:    ":0",
	}
	res, _ := newMetricsResource(cfg)
	meterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithResource(res))
	meter = meterProvider.Meter(cfg.ServiceName)
	reg := prometheus.NewRegistry()
	metricsInstance = newMetrics(reg)

	assert.NotPanics(t, func() {
		RecordHTTPRequest("GET", "/test", "200", "testHandler", 0.5, 1024)
		RecordWAFBlock("rule-1", "SQL injection", "block")
		RecordDDoSDetection("rate-limiter", "volumetric", "high")
		RecordCacheHit()
		RecordCacheMiss()
		SetCrawlerQueueSize(42)
		RecordRenderDuration("http://example.com", "200", "true", 1.5)
		RecordRenderError("http://example.com", "timeout")
		SetSSLCertExpiryDays("example.com", "LE", 30)
		IncActiveRequests()
		DecActiveRequests()
	})
}

func TestRecordFunctionsNilInstance(t *testing.T) {
	oldInstance := metricsInstance
	metricsInstance = nil
	t.Cleanup(func() { metricsInstance = oldInstance })

	assert.NotPanics(t, func() {
		RecordHTTPRequest("GET", "/test", "200", "h", 0.5, 100)
		RecordWAFBlock("r", "reason", "block")
		RecordDDoSDetection("d", "type", "high")
		RecordCacheHit()
		RecordCacheMiss()
		SetCrawlerQueueSize(10)
		RecordRenderDuration("url", "200", "true", 1.0)
		RecordRenderError("url", "err")
		SetSSLCertExpiryDays("d", "issuer", 30)
		IncActiveRequests()
		DecActiveRequests()
	})
}

func TestShutdownMetricsWithProvider(t *testing.T) {
	old := meterProvider
	meterProvider = sdkmetric.NewMeterProvider()
	t.Cleanup(func() { meterProvider = old })

	err := ShutdownMetrics()
	assert.NoError(t, err)
}

func TestGetMeterInitialized(t *testing.T) {
	oldMeter := meter
	oldProvider := meterProvider

	res, _ := resource.New(context.Background(),
		resource.WithAttributes(semconv.ServiceName("test")),
	)
	meterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithResource(res))
	meter = meterProvider.Meter("test")
	t.Cleanup(func() {
		meter = oldMeter
		meterProvider = oldProvider
	})

	m := GetMeter()
	assert.NotNil(t, m)
}

func TestNewMetricsGogenabled(t *testing.T) {
	cfg := &MetricsConfig{
		ServiceName:    fmt.Sprintf("test-gogc-%d", time.Now().UnixNano()),
		Endpoint:       ":0",
		EnableGoGC:     true,
		EnableProcess:  true,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
	}

	res, err := newMetricsResource(cfg)
	if err != nil {
		t.Skipf("Resource error: %v", err)
	}

	_ = res
	metrics := newMetrics(prometheus.NewRegistry())
	assert.NotNil(t, metrics)
}

func TestNewMultiExporterWithOTLP(t *testing.T) {
	cfg := &ExporterConfig{
		OTLPEndpoint: "http://localhost:4318",
		OTLPHeaders:  map[string]string{"X-Auth": "token"},
	}
	exporter := NewMultiExporter(cfg, zap.NewNop())
	assert.NotNil(t, exporter)
	assert.Len(t, exporter.exporters, 1)
}

func TestNewMultiExporterWithPrometheusWrite(t *testing.T) {
	cfg := &ExporterConfig{
		PrometheusRemoteWriteURL: "http://localhost:9090/api/v1/write",
		PrometheusFlushInterval:  1 * time.Hour,
	}
	exporter := NewMultiExporter(cfg, zap.NewNop())
	assert.NotNil(t, exporter)
	assert.Len(t, exporter.exporters, 1)
}

func TestNewMultiExporterAllExporters(t *testing.T) {
	tmpFile := "/tmp/telemetry_all_test.json"
	t.Cleanup(func() { os.Remove(tmpFile) })

	cfg := &ExporterConfig{
		OTLPEndpoint:             "http://localhost:4318",
		PrometheusRemoteWriteURL: "http://localhost:9090/api/v1/write",
		LogExport:                true,
		FileExportPath:           tmpFile,
		PrometheusFlushInterval:  1 * time.Hour,
	}
	exporter := NewMultiExporter(cfg, zap.NewNop())
	assert.NotNil(t, exporter)
	assert.Len(t, exporter.exporters, 4)

	ctx := context.Background()
	exporter.Shutdown(ctx)
}

func TestOTLPExporterExportSpansReal(t *testing.T) {
	cfg := &ExporterConfig{
		OTLPEndpoint: "http://localhost:4318",
		OTLPTimeout:  100 * time.Millisecond,
	}
	exporter := NewOTLPExporter(cfg, zap.NewNop())

	tp := sdktrace.NewTracerProvider()
	_, span := tp.Tracer("test").Start(context.Background(), "test-span")
	span.End()

	err := exporter.ExportSpans(context.Background(), []sdktrace.ReadOnlySpan{span.(sdktrace.ReadOnlySpan)})
	_ = err
}

func TestSpansToOTLPProtoWithData(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	_, span := tp.Tracer("test").Start(context.Background(), "test-span",
		trace.WithAttributes(
			attribute.String("key1", "value1"),
			attribute.Int("key2", 42),
		),
	)
	span.End()

	result := spansToOTLPProto([]sdktrace.ReadOnlySpan{span.(sdktrace.ReadOnlySpan)})
	assert.Len(t, result, 1)
	assert.Equal(t, "test-span", result[0].Name)
	assert.NotEmpty(t, result[0].TraceID)
	assert.NotEmpty(t, result[0].SpanID)
}

func TestSpansToOTLPProtoWithParent(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	ctx, parent := tp.Tracer("test").Start(context.Background(), "parent-span")
	_, child := tp.Tracer("test").Start(ctx, "child-span")
	child.End()
	parent.End()

	result := spansToOTLPProto([]sdktrace.ReadOnlySpan{parent.(sdktrace.ReadOnlySpan), child.(sdktrace.ReadOnlySpan)})
	assert.Len(t, result, 2)
	assert.NotEmpty(t, result[1].ParentSpanID)
}

func TestSpansToOTLPProtoWithStatus(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	_, span := tp.Tracer("test").Start(context.Background(), "error-span")
	span.SetStatus(1, "something went wrong")
	span.End()

	result := spansToOTLPProto([]sdktrace.ReadOnlySpan{span.(sdktrace.ReadOnlySpan)})
	assert.Len(t, result, 1)
	assert.Contains(t, result[0].Status, "Error")
}

func TestLogExporterWithRealSpans(t *testing.T) {
	cfg := &ExporterConfig{
		LogExport:      true,
		LogExportLevel: zap.DebugLevel,
	}
	exporter := NewLogExporter(cfg, zap.NewNop())

	tp := sdktrace.NewTracerProvider()
	_, span := tp.Tracer("test").Start(context.Background(), "test-log-span",
		trace.WithAttributes(attribute.String("test", "value")),
	)
	span.End()

	err := exporter.ExportSpans(context.Background(), []sdktrace.ReadOnlySpan{span.(sdktrace.ReadOnlySpan)})
	assert.NoError(t, err)
}

func TestFileExporterExportRealSpansJSON(t *testing.T) {
	tmpFile := "/tmp/telemetry_file_json_test.json"
	t.Cleanup(func() { os.Remove(tmpFile) })

	cfg := &ExporterConfig{
		FileExportPath:   tmpFile,
		FileExportFormat: "json",
	}
	exporter := NewFileExporter(cfg, zap.NewNop())
	if exporter == nil {
		t.Skip("File exporter is nil")
	}

	tp := sdktrace.NewTracerProvider()
	_, span := tp.Tracer("test").Start(context.Background(), "test-file-span",
		trace.WithAttributes(
			attribute.String("key1", "val1"),
			attribute.Float64("key2", 3.14),
		),
	)
	span.End()

	err := exporter.ExportSpans(context.Background(), []sdktrace.ReadOnlySpan{span.(sdktrace.ReadOnlySpan)})
	assert.NoError(t, err)

	data, err := os.ReadFile(tmpFile)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "test-file-span")
}

func TestFileExporterTextFormatFile(t *testing.T) {
	tmpFile := "/tmp/telemetry_text_test_file.txt"
	t.Cleanup(func() { os.Remove(tmpFile) })

	cfg := &ExporterConfig{
		FileExportPath:   tmpFile,
		FileExportFormat: "text",
	}
	exporter := NewFileExporter(cfg, zap.NewNop())
	if exporter == nil {
		t.Skip("File exporter is nil")
	}

	tp := sdktrace.NewTracerProvider()
	_, span := tp.Tracer("test").Start(context.Background(), "text-span-test-file")
	span.End()

	err := exporter.ExportSpans(context.Background(), []sdktrace.ReadOnlySpan{span.(sdktrace.ReadOnlySpan)})
	assert.NoError(t, err)

	data, err := os.ReadFile(tmpFile)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "text-span-test-file")
}

func TestSpanToJSONMethod(t *testing.T) {
	exporter := &FileExporter{format: "json"}
	tp := sdktrace.NewTracerProvider()
	_, span := tp.Tracer("test").Start(context.Background(), "json-span-test",
		trace.WithAttributes(attribute.String("key", "val")),
	)
	span.End()

	data, err := exporter.spanToJSON(span.(sdktrace.ReadOnlySpan))
	assert.NoError(t, err)
	assert.Contains(t, string(data), "json-span-test")

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)
	assert.Equal(t, "json-span-test", parsed["name"])
}

func TestSpanToTextMethod(t *testing.T) {
	exporter := &FileExporter{format: "text"}
	tp := sdktrace.NewTracerProvider()
	_, span := tp.Tracer("test").Start(context.Background(), "text-span-test")
	span.End()

	data := exporter.spanToText(span.(sdktrace.ReadOnlySpan))
	assert.Contains(t, string(data), "text-span-test")
}

func TestPrometheusExporterExportSpansData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), "trace_span_duration_seconds")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &ExporterConfig{
		PrometheusRemoteWriteURL: server.URL,
		PrometheusBatchSize:      10,
		PrometheusFlushInterval:  1 * time.Hour,
	}
	exporter := NewPrometheusRemoteWriteExporter(cfg, zap.NewNop())
	exporter.url = server.URL

	tp := sdktrace.NewTracerProvider()
	_, span := tp.Tracer("test").Start(context.Background(), "prom-test-span",
		trace.WithAttributes(attribute.String("service", "test")),
	)
	span.End()

	err := exporter.ExportSpans(context.Background(), []sdktrace.ReadOnlySpan{span.(sdktrace.ReadOnlySpan)})
	assert.NoError(t, err)
	exporter.Shutdown(context.Background())
}

func TestPrometheusExporterFlushWithData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &ExporterConfig{
		PrometheusRemoteWriteURL: server.URL,
		PrometheusBatchSize:      5,
		PrometheusFlushInterval:  1 * time.Hour,
	}
	exporter := NewPrometheusRemoteWriteExporter(cfg, zap.NewNop())

	exporter.mu.Lock()
	exporter.metricBuffer = append(exporter.metricBuffer, prometheusMetric{
		Name:      "test_metric",
		Labels:    map[string]string{"key": "value"},
		Value:     1.0,
		Timestamp: time.Now().UnixMilli(),
	})
	exporter.mu.Unlock()

	exporter.flush()

	exporter.mu.Lock()
	assert.Empty(t, exporter.metricBuffer)
	exporter.mu.Unlock()

	exporter.Shutdown(context.Background())
}

func TestPrometheusExporterFlushServerError(t *testing.T) {
	cfg := &ExporterConfig{
		PrometheusRemoteWriteURL: "http://localhost:9999",
		PrometheusFlushInterval:  1 * time.Hour,
	}
	exporter := NewPrometheusRemoteWriteExporter(cfg, zap.NewNop())

	exporter.mu.Lock()
	exporter.metricBuffer = append(exporter.metricBuffer, prometheusMetric{
		Name:  "error_metric",
		Value: 1.0,
	})
	exporter.mu.Unlock()

	assert.NotPanics(t, func() { exporter.flush() })
	exporter.Shutdown(context.Background())
}

func TestPrometheusExporterFlushBadResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	cfg := &ExporterConfig{
		PrometheusRemoteWriteURL: server.URL,
		PrometheusFlushInterval:  1 * time.Hour,
	}
	exporter := NewPrometheusRemoteWriteExporter(cfg, zap.NewNop())

	exporter.mu.Lock()
	exporter.metricBuffer = append(exporter.metricBuffer, prometheusMetric{
		Name:  "bad_resp_metric",
		Value: 1.0,
	})
	exporter.mu.Unlock()

	assert.NotPanics(t, func() { exporter.flush() })
	exporter.Shutdown(context.Background())
}

func TestFileExporterShutdownNoFile(t *testing.T) {
	exporter := &FileExporter{file: nil, format: "json"}
	err := exporter.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestMultiExporterExportSpansError(t *testing.T) {
	cfg := &ExporterConfig{LogExport: true}
	exporter := NewMultiExporter(cfg, zap.NewNop())

	err := exporter.ExportSpans(context.Background(), nil)
	assert.NoError(t, err)
	exporter.Shutdown(context.Background())
}

func TestOTLPExporterExportEmpty(t *testing.T) {
	cfg := &ExporterConfig{OTLPEndpoint: "http://localhost:4318"}
	exporter := NewOTLPExporter(cfg, zap.NewNop())
	err := exporter.ExportSpans(context.Background(), nil)
	assert.Error(t, err)
}

func TestSpanToJSONWithErrorStatus(t *testing.T) {
	exporter := &FileExporter{format: "json"}
	tp := sdktrace.NewTracerProvider()
	_, span := tp.Tracer("test").Start(context.Background(), "error-json-span")
	span.SetStatus(1, "test error")
	span.End()

	data, err := exporter.spanToJSON(span.(sdktrace.ReadOnlySpan))
	assert.NoError(t, err)
	assert.Contains(t, string(data), "error-json-span")
}

func TestMultiExporterConcurrentShutdown(t *testing.T) {
	cfg := &ExporterConfig{}
	exporter := NewMultiExporter(cfg, zap.NewNop())

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			exporter.Shutdown(context.Background())
		}()
	}
	wg.Wait()
}

func TestInitTracerWithOTLPTimeout(t *testing.T) {
	cfg := &TracerConfig{
		ServiceName: "test-otlp-timeout",
		Endpoint:    "http://localhost:4318",
		Timeout:     100 * time.Millisecond,
	}
	_ = InitTracer(cfg, zap.NewNop())
	ShutdownTracer()
}

func TestNewExporterStdoutExporter(t *testing.T) {
	cfg := &TracerConfig{UseStdout: true}
	exporter, err := newExporter(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exporter)
}

func TestNewExporterEmptyEndpoint(t *testing.T) {
	cfg := &TracerConfig{Endpoint: ""}
	exporter, err := newExporter(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exporter)
}

func TestGetTracerNoopWhenNil(t *testing.T) {
	oldTracer := tracer
	tracer = nil
	t.Cleanup(func() { tracer = oldTracer })

	tr := GetTracer()
	assert.NotNil(t, tr)
	_, span := tr.Start(context.Background(), "noop-test")
	assert.False(t, span.SpanContext().IsValid())
	span.End()
}

func TestStartSpanNoopTracer(t *testing.T) {
	oldTracer := tracer
	tracer = nil
	t.Cleanup(func() { tracer = oldTracer })

	ctx, span := StartSpan(context.Background(), "noop-span")
	assert.NotNil(t, span)
	assert.NotNil(t, ctx)
	span.End()
}

func TestErrorRecorderProcessorWithSpan(t *testing.T) {
	processor := &errorRecorderSpanProcessor{}
	tp := sdktrace.NewTracerProvider()
	_, span := tp.Tracer("test").Start(context.Background(), "test-processor-span")
	span.End()

	ctx := context.Background()
	assert.NotPanics(t, func() {
		processor.OnStart(ctx, span.(sdktrace.ReadWriteSpan))
		processor.OnEnd(span.(sdktrace.ReadOnlySpan))
	})
}

func TestCombinedMiddlewareFullFeatures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CombinedMiddleware(&MiddlewareConfig{
		ServiceName:        "test",
		EnableTracing:      true,
		EnableMetrics:      true,
		RecordResponseBody: true,
		MaxBodySize:        4096,
		SkipPaths:          []string{"/skip"},
	}))
	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": "hello"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/test", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCombinedMiddlewareWithError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CombinedMiddleware(&MiddlewareConfig{
		EnableTracing: true,
		EnableMetrics: true,
	}))
	router.GET("/error", func(c *gin.Context) {
		c.Error(assert.AnError)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/error", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestCombinedMiddlewareMetricsOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CombinedMiddleware(&MiddlewareConfig{
		ServiceName:   "metrics-only",
		EnableTracing: false,
		EnableMetrics: true,
	}))
	router.GET("/metrics-only", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/metrics-only", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTraceMiddlewareWithHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceMiddleware(&MiddlewareConfig{
		EnableTracing: true,
		ServiceName:   "test",
	}))
	router.GET("/host-test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"host": c.Request.Host})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/host-test", nil)
	req.Host = "example.com:8080"
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTraceMiddlewareRecordResponseBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceMiddleware(&MiddlewareConfig{
		EnableTracing:      true,
		RecordResponseBody: true,
		MaxBodySize:        50,
	}))
	router.GET("/response-body", func(c *gin.Context) {
		c.String(http.StatusOK, "this is the response body content")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/response-body", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestResponseWriterSize(t *testing.T) {
	mock := &mockResponseWriter{}
	rw := newResponseWriter(mock, false, 0)

	n, err := rw.Write([]byte("hello world"))
	assert.NoError(t, err)
	assert.Equal(t, 11, n)
	assert.Equal(t, 11, rw.writtenSize)
}

func TestResponseWriterWriteHeader(t *testing.T) {
	mock := &mockResponseWriter{}
	rw := newResponseWriter(mock, true, 100)
	rw.WriteHeader(http.StatusNotFound)
	assert.Equal(t, http.StatusNotFound, rw.status)
}

func TestInjectHeadersFromRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/test", nil)

	headers := make(http.Header)
	assert.NotPanics(t, func() {
		InjectHeaders(c, headers)
	})
}

func TestMultiExporterShutdownWithLock(t *testing.T) {
	cfg := &ExporterConfig{
		OTLPEndpoint: "http://localhost:4318",
	}
	exporter := NewMultiExporter(cfg, zap.NewNop())
	ctx := context.Background()
	err := exporter.Shutdown(ctx)
	assert.NoError(t, err)
}
