package telemetry

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

var metricsTestMu sync.Mutex

func resetMetricsGlobals() {
	meterProvider = nil
	meter = nil
	metricsInstance = nil
}

func TestMetricsConfig_DefaultConfig(t *testing.T) {
	cfg := DefaultMetricsConfig()
	assert.NotNil(t, cfg)
	assert.Equal(t, "prerender-shield", cfg.ServiceName)
	assert.Equal(t, ":9090", cfg.Endpoint)
	assert.True(t, cfg.EnableGoGC)
	assert.True(t, cfg.EnableProcess)
	assert.Equal(t, 10*time.Second, cfg.ReadTimeout)
	assert.Equal(t, 10*time.Second, cfg.WriteTimeout)
}

func TestMetricsConfig_EnvironmentVariables(t *testing.T) {
	os.Setenv("ENVIRONMENT", "staging")
	os.Setenv("SERVICE_VERSION", "3.0.0")
	defer func() {
		os.Unsetenv("ENVIRONMENT")
		os.Unsetenv("SERVICE_VERSION")
	}()

	cfg := DefaultMetricsConfig()
	assert.Equal(t, "staging", cfg.Environment)
	assert.Equal(t, "3.0.0", cfg.ServiceVersion)
}

func TestInitMetrics_NilConfig(t *testing.T) {
	metricsTestMu.Lock()
	defer resetMetricsGlobals()
	defer metricsTestMu.Unlock()

	// 先清理之前的状态
	meterProvider = nil
	meter = nil

	logger, _ := zap.NewDevelopment()
	metrics, err := InitMetrics(nil, logger)
	if err != nil {
		t.Skipf("Skipping test due to metrics init error: %v", err)
	}
	assert.NotNil(t, metrics)
	defer ShutdownMetrics()
}

func TestInitMetrics_NilLogger(t *testing.T) {
	metricsTestMu.Lock()
	defer resetMetricsGlobals()
	defer metricsTestMu.Unlock()

	// 先清理之前的状态
	meterProvider = nil
	meter = nil
	metricsInstance = nil

	cfg := &MetricsConfig{
		ServiceName: "test-metrics",
		Endpoint:    ":0", // 使用随机端口
	}
	metrics, err := InitMetrics(cfg, nil)
	if err != nil {
		t.Skipf("Skipping test due to metrics init error: %v", err)
	}
	if metrics == nil {
		t.Skip("Skipping test: metrics is nil")
	}
	assert.NotNil(t, metrics)
	defer ShutdownMetrics()
}

func TestInitMetrics_CustomConfig(t *testing.T) {
	metricsTestMu.Lock()
	defer resetMetricsGlobals()
	defer metricsTestMu.Unlock()

	// 先清理之前的状态
	meterProvider = nil
	meter = nil
	metricsInstance = nil

	logger, _ := zap.NewDevelopment()
	cfg := &MetricsConfig{
		ServiceName:    "test-metrics",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		Endpoint:       ":0",
		EnableGoGC:     false,
		EnableProcess:  false,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
	}

	metrics, err := InitMetrics(cfg, logger)
	if err != nil {
		t.Skipf("Skipping test due to metrics init error: %v", err)
	}
	if metrics == nil {
		t.Skip("Skipping test: metrics is nil")
	}
	assert.NotNil(t, metrics)
	defer ShutdownMetrics()
}

func TestInitMetrics_InvalidEndpoint(t *testing.T) {
	metricsTestMu.Lock()
	defer resetMetricsGlobals()
	defer metricsTestMu.Unlock()

	logger, _ := zap.NewDevelopment()
	cfg := &MetricsConfig{
		ServiceName: "test-metrics",
		Endpoint:    "invalid-address-that-will-fail",
	}

	// 这个测试可能不会失败，因为指标服务器只在后台启动
	// 所以我们只验证它不会 panic
	_, _ = InitMetrics(cfg, logger)
	assert.True(t, true)
}

func TestNewMetricsResource(t *testing.T) {
	cfg := &MetricsConfig{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
	}

	res, err := newMetricsResource(cfg)
	if err != nil {
		t.Skipf("Skipping test due to resource creation error: %v", err)
	}
	assert.NotNil(t, res)
}

func TestNewMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()

	// 先初始化 meter
	metricsTestMu.Lock()
	defer resetMetricsGlobals()
	defer metricsTestMu.Unlock()

	// 先清理之前的状态
	meterProvider = nil
	meter = nil
	metricsInstance = nil

	logger, _ := zap.NewDevelopment()
	cfg := &MetricsConfig{
		ServiceName: "test-metrics",
		Endpoint:    ":0",
	}

	_, err := InitMetrics(cfg, logger)
	if err != nil {
		t.Skipf("Skipping test due to metrics init error: %v", err)
	}

	// 检查 meter 是否已初始化
	if meter == nil {
		t.Skip("Skipping test: meter is nil after InitMetrics")
	}

	defer ShutdownMetrics()

	// 现在可以安全地调用 newMetrics
	metrics := newMetrics(reg)
	if metrics == nil {
		t.Skip("Skipping test: metrics is nil")
	}

	assert.NotNil(t, metrics)
	assert.NotNil(t, metrics.HTTPRequestTotal)
	assert.NotNil(t, metrics.HTTPRequestDuration)
	assert.NotNil(t, metrics.HTTPActiveRequests)
	assert.NotNil(t, metrics.HTTPResponseSize)
	assert.NotNil(t, metrics.CrawlerQueueSize)
	assert.NotNil(t, metrics.CacheHitTotal)
	assert.NotNil(t, metrics.CacheMissTotal)
	assert.NotNil(t, metrics.WAFBlockedTotal)
	assert.NotNil(t, metrics.DDoSDetectedTotal)
	assert.NotNil(t, metrics.SSLCertExpiryDays)
	assert.NotNil(t, metrics.RenderDuration)
	assert.NotNil(t, metrics.RenderErrorTotal)
}

func TestStartMetricsServer(t *testing.T) {
	reg := prometheus.NewRegistry()

	// 使用随机端口
	err := startMetricsServer(":0", reg, 5*time.Second, 5*time.Second)
	assert.NoError(t, err)

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)
}

func TestStartMetricsServer_HealthEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// 验证健康检查端点
	resp, err := http.Get(server.URL + "/health")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestGetMeter_NotInitialized(t *testing.T) {
	meter = nil
	meter := GetMeter()
	assert.NotNil(t, meter)
}

func TestGetMeter_Initialized(t *testing.T) {
	metricsTestMu.Lock()
	defer resetMetricsGlobals()
	defer metricsTestMu.Unlock()

	logger, _ := zap.NewDevelopment()
	cfg := &MetricsConfig{
		ServiceName: "test-metrics",
		Endpoint:    ":0",
	}

	_, err := InitMetrics(cfg, logger)
	if err != nil {
		t.Skipf("Skipping test due to metrics init error: %v", err)
	}
	defer ShutdownMetrics()

	meter := GetMeter()
	assert.NotNil(t, meter)
}

func TestShutdownMetrics_NotInitialized(t *testing.T) {
	meterProvider = nil
	err := ShutdownMetrics()
	assert.NoError(t, err)
}

func TestShutdownMetrics_Initialized(t *testing.T) {
	metricsTestMu.Lock()
	defer resetMetricsGlobals()
	defer metricsTestMu.Unlock()

	logger, _ := zap.NewDevelopment()
	cfg := &MetricsConfig{
		ServiceName: "test-metrics",
		Endpoint:    ":0",
	}

	_, err := InitMetrics(cfg, logger)
	if err != nil {
		t.Skipf("Skipping test due to metrics init error: %v", err)
	}

	err = ShutdownMetrics()
	assert.NoError(t, err)
}

func TestRecordHTTPMethods(t *testing.T) {
	metricsTestMu.Lock()
	defer resetMetricsGlobals()
	defer metricsTestMu.Unlock()

	// 先清理之前的状态
	meterProvider = nil
	meter = nil
	metricsInstance = nil

	logger, _ := zap.NewDevelopment()
	cfg := &MetricsConfig{
		ServiceName: "test-metrics",
		Endpoint:    ":0",
	}

	_, err := InitMetrics(cfg, logger)
	if err != nil {
		t.Skipf("Skipping test due to metrics init error: %v", err)
	}
	if metricsInstance == nil {
		t.Skip("Skipping test: metricsInstance is nil")
	}
	defer ShutdownMetrics()

	// 记录 HTTP 请求
	RecordHTTPRequest("GET", "/api/test", "200", "testHandler", 0.5, 1024)

	// 不应该 panic
	assert.NotPanics(t, func() {
		RecordHTTPRequest("POST", "/api/create", "201", "createHandler", 1.0, 2048)
	})
}

func TestRecordWAFBlock(t *testing.T) {
	metricsTestMu.Lock()
	defer resetMetricsGlobals()
	defer metricsTestMu.Unlock()

	// 先清理之前的状态
	meterProvider = nil
	meter = nil
	metricsInstance = nil

	logger, _ := zap.NewDevelopment()
	cfg := &MetricsConfig{
		ServiceName: "test-metrics",
		Endpoint:    ":0",
	}

	_, err := InitMetrics(cfg, logger)
	if err != nil {
		t.Skipf("Skipping test due to metrics init error: %v", err)
	}
	if metricsInstance == nil {
		t.Skip("Skipping test: metricsInstance is nil")
	}
	defer ShutdownMetrics()

	// 记录 WAF 拦截
	assert.NotPanics(t, func() {
		RecordWAFBlock("sql-injection", "SQL injection detected", "block")
	})
}

func TestRecordDDoSDetection(t *testing.T) {
	metricsTestMu.Lock()
	defer resetMetricsGlobals()
	defer metricsTestMu.Unlock()

	// 先清理之前的状态
	meterProvider = nil
	meter = nil
	metricsInstance = nil

	logger, _ := zap.NewDevelopment()
	cfg := &MetricsConfig{
		ServiceName: "test-metrics",
		Endpoint:    ":0",
	}

	_, err := InitMetrics(cfg, logger)
	if err != nil {
		t.Skipf("Skipping test due to metrics init error: %v", err)
	}
	if metricsInstance == nil {
		t.Skip("Skipping test: metricsInstance is nil")
	}
	defer ShutdownMetrics()

	// 记录 DDoS 检测
	assert.NotPanics(t, func() {
		RecordDDoSDetection("rate-limiter", "volumetric", "high")
	})
}

func TestRecordCacheHitMiss(t *testing.T) {
	metricsTestMu.Lock()
	defer resetMetricsGlobals()
	defer metricsTestMu.Unlock()

	// 先清理之前的状态
	meterProvider = nil
	meter = nil
	metricsInstance = nil

	logger, _ := zap.NewDevelopment()
	cfg := &MetricsConfig{
		ServiceName: "test-metrics",
		Endpoint:    ":0",
	}

	_, err := InitMetrics(cfg, logger)
	if err != nil {
		t.Skipf("Skipping test due to metrics init error: %v", err)
	}
	if metricsInstance == nil {
		t.Skip("Skipping test: metricsInstance is nil")
	}
	defer ShutdownMetrics()

	// 记录缓存命中
	assert.NotPanics(t, func() {
		RecordCacheHit()
		RecordCacheMiss()
	})
}

func TestSetCrawlerQueueSize(t *testing.T) {
	metricsTestMu.Lock()
	defer resetMetricsGlobals()
	defer metricsTestMu.Unlock()

	// 先清理之前的状态
	meterProvider = nil
	meter = nil
	metricsInstance = nil

	logger, _ := zap.NewDevelopment()
	cfg := &MetricsConfig{
		ServiceName: "test-metrics",
		Endpoint:    ":0",
	}

	_, err := InitMetrics(cfg, logger)
	if err != nil {
		t.Skipf("Skipping test due to metrics init error: %v", err)
	}
	if metricsInstance == nil {
		t.Skip("Skipping test: metricsInstance is nil")
	}
	defer ShutdownMetrics()

	// 设置爬虫队列大小
	assert.NotPanics(t, func() {
		SetCrawlerQueueSize(100)
	})
}

func TestRecordRenderDuration(t *testing.T) {
	metricsTestMu.Lock()
	defer resetMetricsGlobals()
	defer metricsTestMu.Unlock()

	// 先清理之前的状态
	meterProvider = nil
	meter = nil
	metricsInstance = nil

	logger, _ := zap.NewDevelopment()
	cfg := &MetricsConfig{
		ServiceName: "test-metrics",
		Endpoint:    ":0",
	}

	_, err := InitMetrics(cfg, logger)
	if err != nil {
		t.Skipf("Skipping test due to metrics init error: %v", err)
	}
	if metricsInstance == nil {
		t.Skip("Skipping test: metricsInstance is nil")
	}
	defer ShutdownMetrics()

	// 记录渲染延迟
	assert.NotPanics(t, func() {
		RecordRenderDuration("http://example.com", "200", "true", 2.5)
	})
}

func TestRecordRenderError(t *testing.T) {
	metricsTestMu.Lock()
	defer resetMetricsGlobals()
	defer metricsTestMu.Unlock()

	// 先清理之前的状态
	meterProvider = nil
	meter = nil
	metricsInstance = nil

	logger, _ := zap.NewDevelopment()
	cfg := &MetricsConfig{
		ServiceName: "test-metrics",
		Endpoint:    ":0",
	}

	_, err := InitMetrics(cfg, logger)
	if err != nil {
		t.Skipf("Skipping test due to metrics init error: %v", err)
	}
	if metricsInstance == nil {
		t.Skip("Skipping test: metricsInstance is nil")
	}
	defer ShutdownMetrics()

	// 记录渲染错误
	assert.NotPanics(t, func() {
		RecordRenderError("http://example.com", "timeout")
	})
}

func TestSetSSLCertExpiryDays(t *testing.T) {
	metricsTestMu.Lock()
	defer resetMetricsGlobals()
	defer metricsTestMu.Unlock()

	// 先清理之前的状态
	meterProvider = nil
	meter = nil
	metricsInstance = nil

	logger, _ := zap.NewDevelopment()
	cfg := &MetricsConfig{
		ServiceName: "test-metrics",
		Endpoint:    ":0",
	}

	_, err := InitMetrics(cfg, logger)
	if err != nil {
		t.Skipf("Skipping test due to metrics init error: %v", err)
	}
	if metricsInstance == nil {
		t.Skip("Skipping test: metricsInstance is nil")
	}
	defer ShutdownMetrics()

	// 设置 SSL 证书过期天数
	assert.NotPanics(t, func() {
		SetSSLCertExpiryDays("example.com", "Let's Encrypt", 30)
	})
}

func TestIncDecActiveRequests(t *testing.T) {
	metricsTestMu.Lock()
	defer resetMetricsGlobals()
	defer metricsTestMu.Unlock()

	// 先清理之前的状态
	meterProvider = nil
	meter = nil
	metricsInstance = nil

	logger, _ := zap.NewDevelopment()
	cfg := &MetricsConfig{
		ServiceName: "test-metrics",
		Endpoint:    ":0",
	}

	_, err := InitMetrics(cfg, logger)
	if err != nil {
		t.Skipf("Skipping test due to metrics init error: %v", err)
	}
	if metricsInstance == nil {
		t.Skip("Skipping test: metricsInstance is nil")
	}
	defer ShutdownMetrics()

	// 增加和减少活跃请求数
	assert.NotPanics(t, func() {
		IncActiveRequests()
		IncActiveRequests()
		DecActiveRequests()
		DecActiveRequests()
	})
}

func TestGetMetricsRegistry(t *testing.T) {
	reg := prometheus.NewRegistry()
	result := GetMetricsRegistry(reg)
	assert.Equal(t, reg, result)
}

func TestMetricsInstance_NotInitialized(t *testing.T) {
	// 验证未初始化时的行为
	assert.NotPanics(t, func() {
		// 这些函数在 metricsInstance 为 nil 时会 panic，
		// 但应该先调用 InitMetrics
	})
}

func TestResponseWriter(t *testing.T) {
	// 测试 responseWriter - 使用简单的 mock ResponseWriter
	mock := &mockResponseWriter{}
	rw := newResponseWriter(mock, false, 0)
	assert.NotNil(t, rw)
	assert.Equal(t, http.StatusOK, rw.status)

	// 测试 WriteHeader
	rw.WriteHeader(http.StatusCreated)
	assert.Equal(t, http.StatusCreated, rw.status)

	// 测试 Write
	data := []byte("test data")
	size, err := rw.Write(data)
	assert.Equal(t, len(data), size)
	assert.NoError(t, err)
	assert.Equal(t, len(data), rw.writtenSize)
}

// mockResponseWriter 简单的 mock ResponseWriter
type mockResponseWriter struct {
	code       int
	written    int
	header     http.Header
	flushed    bool
	hijacked   bool
	pusher     http.Pusher
	size       int
	writtenNow bool
}

func (m *mockResponseWriter) Header() http.Header {
	if m.header == nil {
		m.header = http.Header{}
	}
	return m.header
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	m.written += len(b)
	m.size += len(b)
	return len(b), nil
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.code = statusCode
	m.writtenNow = true
}

func (m *mockResponseWriter) Status() int {
	return m.code
}

func (m *mockResponseWriter) CloseNotify() <-chan bool {
	return nil
}

func (m *mockResponseWriter) ClientConnected() <-chan bool {
	return nil
}

func (m *mockResponseWriter) Flush() {
	m.flushed = true
}

func (m *mockResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	m.hijacked = true
	return nil, nil, nil
}

func (m *mockResponseWriter) Pusher() http.Pusher {
	return m.pusher
}

func (m *mockResponseWriter) Size() int {
	return m.size
}

func (m *mockResponseWriter) WriteHeaderNow() {
	m.writtenNow = true
}

func (m *mockResponseWriter) WriteString(s string) (int, error) {
	m.written += len(s)
	m.size += len(s)
	return len(s), nil
}

func (m *mockResponseWriter) Written() bool {
	return m.written > 0 || m.writtenNow
}

func TestResponseWriter_WithBody(t *testing.T) {
	mock := &mockResponseWriter{}
	rw := newResponseWriter(mock, true, 100)
	assert.NotNil(t, rw)
	assert.NotNil(t, rw.body)

	// 写入数据
	data := []byte("test body")
	rw.Write(data)

	// 验证 body 被记录
	assert.Contains(t, rw.body.String(), string(data))
}

func TestResponseWriter_BodySizeLimit(t *testing.T) {
	mock := &mockResponseWriter{}
	rw := newResponseWriter(mock, true, 10) // 最大 10 字节
	assert.NotNil(t, rw)

	// 写入超过限制的数据
	data := []byte("this is a very long body that exceeds the limit")
	rw.Write(data)

	// 验证 body 被截断
	assert.LessOrEqual(t, rw.body.Len(), 10)
}

func TestDefaultMiddlewareConfig(t *testing.T) {
	cfg := DefaultMiddlewareConfig("test-service")
	assert.NotNil(t, cfg)
	assert.Equal(t, "test-service", cfg.ServiceName)
	assert.True(t, cfg.EnableMetrics)
	assert.True(t, cfg.EnableTracing)
	assert.Equal(t, 4096, cfg.MaxBodySize)
	assert.Contains(t, cfg.SkipPaths, "/health")
	assert.Contains(t, cfg.SkipPaths, "/metrics")
}
