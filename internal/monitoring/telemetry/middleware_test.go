package telemetry

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func TestMiddlewareConfig_DefaultConfig(t *testing.T) {
	cfg := DefaultMiddlewareConfig("test-service")
	assert.NotNil(t, cfg)
	assert.Equal(t, "test-service", cfg.ServiceName)
	assert.True(t, cfg.EnableMetrics)
	assert.True(t, cfg.EnableTracing)
	assert.Contains(t, cfg.SkipPaths, "/health")
	assert.Contains(t, cfg.SkipPaths, "/metrics")
	assert.Contains(t, cfg.SkipPaths, "/favicon.ico")
}

func TestTraceMiddleware_NilConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceMiddleware(nil))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTraceMiddleware_SkipPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceMiddleware(&MiddlewareConfig{
		SkipPaths: []string{"/health", "/metrics"},
	}))
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTraceMiddleware_CustomTracer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceMiddleware(&MiddlewareConfig{
		ServiceName: "test-service",
		Tracer:      trace.NewNoopTracerProvider().Tracer("noop"),
	}))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTraceMiddleware_WithErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceMiddleware(nil))
	router.GET("/error", func(c *gin.Context) {
		c.Error(assert.AnError)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/error", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestTraceMiddleware_ClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceMiddleware(nil))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"clientIP": c.ClientIP()})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMetricsMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(MetricsMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMetricsMiddleware_HandlerName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(MetricsMiddleware())

	handler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
	router.GET("/test", handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCombinedMiddleware_NilConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CombinedMiddleware(nil))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCombinedMiddleware_WithTracing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CombinedMiddleware(&MiddlewareConfig{
		EnableTracing:      true,
		EnableMetrics:      true,
		ServiceName:        "test-service",
		RecordResponseBody: true,
		MaxBodySize:        1024,
	}))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCombinedMiddleware_WithoutTracing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CombinedMiddleware(&MiddlewareConfig{
		EnableTracing: false,
		EnableMetrics: true,
	}))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWAFMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(WAFMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDDoSMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(DDoSMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSetTelemetryGlobals(t *testing.T) {
	tracer := trace.NewNoopTracerProvider().Tracer("test")
	logger, _ := zap.NewDevelopment()

	SetTelemetryGlobals(tracer, logger)
	assert.NotNil(t, tracerInstance)
	assert.NotNil(t, loggerInstance)
}

func TestSetTelemetryGlobals_NilTracer(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	SetTelemetryGlobals(nil, logger)
	assert.NotNil(t, tracerInstance)
}

func TestSetTelemetryGlobals_NilLogger(t *testing.T) {
	tracer := trace.NewNoopTracerProvider().Tracer("test")
	SetTelemetryGlobals(tracer, nil)
	assert.NotNil(t, loggerInstance)
}

func TestGetTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/test", nil)

	traceID := GetTraceID(c)
	assert.NotEmpty(t, traceID)
}

func TestGetSpanID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/test", nil)

	spanID := GetSpanID(c)
	assert.NotEmpty(t, spanID)
}

func TestInjectHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/test", nil)

	headers := http.Header{}
	InjectHeaders(c, headers)
	// 不验证具体内容，只确保不 panic
	assert.NotNil(t, headers)
}

func TestExtractTraceID(t *testing.T) {
	headers := http.Header{}
	traceID := ExtractTraceID(headers)
	assert.NotEmpty(t, traceID)
}

func TestRecordMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/test", nil)

	// 不应该 panic
	assert.NotPanics(t, func() {
		recordMetrics(c, http.StatusOK, 100*time.Millisecond, 1024)
	})
}

func TestResponseWriter_Interface(t *testing.T) {
	rw := newResponseWriter(nil, false, 0)
	assert.Implements(t, (*gin.ResponseWriter)(nil), rw)
}

func TestTraceMiddleware_FullPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceMiddleware(nil))

	// 使用具名路由
	router.GET("/test/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test/123", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTraceMiddleware_QueryParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceMiddleware(nil))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test?foo=bar&baz=qux", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTraceMiddleware_UserAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceMiddleware(nil))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Test Browser")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCombinedMiddleware_CustomPropagator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CombinedMiddleware(&MiddlewareConfig{
		ServiceName:   "test-service",
		EnableTracing: true,
		EnableMetrics: false,
	}))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
