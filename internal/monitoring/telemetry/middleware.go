package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var (
	tracerInstance trace.Tracer
	meterInstance  = GetMeter()
	loggerInstance *zap.Logger
)

// MiddlewareConfig 中间件配置
type MiddlewareConfig struct {
	ServiceName        string                        // 服务名称
	Tracer             trace.Tracer                  // 追踪器
	Propagator         propagation.TextMapPropagator // 传播器
	SkipPaths          []string                      // 跳过追踪的路径
	EnableMetrics      bool                          // 启用指标
	EnableTracing      bool                          // 启用追踪
	RecordRequestBody  bool                          // 记录请求体
	RecordResponseBody bool                          // 记录响应体
	MaxBodySize        int                           // 最大记录体大小 (字节)
}

// defaultSkipPaths 默认跳过的路径
var defaultSkipPaths = []string{
	"/health",
	"/metrics",
	"/favicon.ico",
}

// DefaultMiddlewareConfig 返回默认配置
func DefaultMiddlewareConfig(serviceName string) *MiddlewareConfig {
	return &MiddlewareConfig{
		ServiceName:   serviceName,
		Tracer:        otel.Tracer(serviceName),
		Propagator:    otel.GetTextMapPropagator(),
		SkipPaths:     defaultSkipPaths,
		EnableMetrics: true,
		EnableTracing: true,
		MaxBodySize:   4096,
	}
}

// responseWriter 响应包装器
type responseWriter struct {
	gin.ResponseWriter
	status      int
	writtenSize int
	body        *strings.Builder
	recordBody  bool
	maxSize     int
}

func newResponseWriter(w gin.ResponseWriter, recordBody bool, maxSize int) *responseWriter {
	rw := &responseWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
		recordBody:     recordBody,
		maxSize:        maxSize,
	}
	if recordBody && maxSize > 0 {
		rw.body = &strings.Builder{}
	}
	return rw
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.writtenSize += size

	if rw.recordBody && rw.body != nil {
		if rw.body.Len()+len(b) <= rw.maxSize {
			rw.body.Write(b)
		}
	}

	return size, err
}

// TraceMiddleware OpenTelemetry 追踪中间件
func TraceMiddleware(cfg *MiddlewareConfig) gin.HandlerFunc {
	if cfg == nil {
		cfg = DefaultMiddlewareConfig("prerender-shield")
	}

	if cfg.Tracer == nil {
		cfg.Tracer = otel.Tracer(cfg.ServiceName)
	}

	if cfg.Propagator == nil {
		cfg.Propagator = otel.GetTextMapPropagator()
	}

	skipPathSet := make(map[string]bool)
	for _, path := range cfg.SkipPaths {
		skipPathSet[path] = true
	}

	return func(c *gin.Context) {
		// 检查是否跳过
		if skipPathSet[c.Request.URL.Path] {
			c.Next()
			return
		}

		// 提取追踪上下文
		ctx := cfg.Propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

		// 创建 Span
		spanName := c.Request.URL.Path
		if c.FullPath() != "" {
			spanName = c.FullPath()
		}

		opts := []trace.SpanStartOption{
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPMethod(c.Request.Method),
				semconv.HTTPRoute(spanName),
				semconv.URLPath(c.Request.URL.Path),
				semconv.URLQuery(c.Request.URL.RawQuery),
				semconv.UserAgentOriginal(c.Request.UserAgent()),
				semconv.HTTPRequestBodySize(int(c.Request.ContentLength)),
			),
		}

		if c.Request.URL.Host != "" {
			opts = append(opts, trace.WithAttributes(
				semconv.ServerAddress(c.Request.URL.Host),
			))
		}

		// 添加客户端 IP
		if clientIP := c.ClientIP(); clientIP != "" {
			opts = append(opts, trace.WithAttributes(
				semconv.ClientAddress(clientIP),
			))
		}

		// 开始 Span
		ctx, span := cfg.Tracer.Start(ctx, spanName, opts...)
		defer span.End()

		// 设置上下文
		c.Request = c.Request.WithContext(ctx)

		// 包装响应
		rw := newResponseWriter(c.Writer, cfg.RecordResponseBody, cfg.MaxBodySize)
		c.Writer = rw

		// 记录开始时间
		start := time.Now()

		// 处理请求
		c.Next()

		// 记录延迟
		duration := time.Since(start)

		// 设置 Span 属性
		span.SetAttributes(
			semconv.HTTPStatusCode(rw.status),
			semconv.HTTPResponseBodySize(rw.writtenSize),
			attribute.String("http.latency.ms", fmt.Sprintf("%d", duration.Milliseconds())),
		)

		// 记录响应体
		if cfg.RecordResponseBody && rw.body != nil {
			span.SetAttributes(
				attribute.String("http.response.body", rw.body.String()),
			)
		}

		// 记录错误
		if len(c.Errors) > 0 {
			span.SetStatus(codes.Error, "request failed")
			for _, err := range c.Errors {
				span.RecordError(err.Err)
			}
		} else if rw.status >= 400 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", rw.status))
		} else {
			span.SetStatus(codes.Ok, "")
		}

		// 记录指标
		if cfg.EnableMetrics {
			recordMetrics(c, rw.status, duration, rw.writtenSize)
		}
	}
}

// MetricsMiddleware 指标收集中间件
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 增加活跃请求数
		IncActiveRequests()
		defer DecActiveRequests()

		// 包装响应
		rw := newResponseWriter(c.Writer, false, 0)
		c.Writer = rw

		c.Next()

		duration := time.Since(start).Seconds()

		// 记录 HTTP 指标
		status := strconv.Itoa(rw.status)
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		RecordHTTPRequest(method, path, status, c.HandlerName(), duration, rw.writtenSize)
	}
}

// recordMetrics 记录指标
func recordMetrics(c *gin.Context, status int, duration time.Duration, size int) {
	labels := prometheus.Labels{
		"method":  c.Request.Method,
		"path":    c.FullPath(),
		"status":  strconv.Itoa(status),
		"handler": c.HandlerName(),
	}

	// Prometheus 指标
	if metricsInstance != nil {
		metricsInstance.HTTPRequestTotal.With(labels).Inc()
		metricsInstance.HTTPRequestDuration.With(prometheus.Labels{
			"method": c.Request.Method,
			"path":   c.FullPath(),
			"status": strconv.Itoa(status),
		}).Observe(duration.Seconds())
		metricsInstance.HTTPResponseSize.With(prometheus.Labels{
			"method": c.Request.Method,
			"path":   c.FullPath(),
		}).Observe(float64(size))
	}

	// OTel 指标
	ctx := c.Request.Context()
	if meterInstance != nil {
		if requestCounter, err := meterInstance.Int64Counter("http_requests"); err == nil {
			requestCounter.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("method", c.Request.Method),
					attribute.String("path", c.FullPath()),
					attribute.String("status", strconv.Itoa(status)),
				),
			)
		}

		if latencyHist, err := meterInstance.Float64Histogram("http_request_duration"); err == nil {
			latencyHist.Record(ctx, duration.Seconds(),
				metric.WithAttributes(
					attribute.String("method", c.Request.Method),
					attribute.String("path", c.FullPath()),
				),
			)
		}
	}
}

// CombinedMiddleware 组合追踪和指标中间件
func CombinedMiddleware(cfg *MiddlewareConfig) gin.HandlerFunc {
	if cfg == nil {
		cfg = DefaultMiddlewareConfig("prerender-shield")
	}

	return func(c *gin.Context) {
		// 指标中间件逻辑
		start := time.Now()
		IncActiveRequests()
		defer DecActiveRequests()

		// 包装响应
		rw := newResponseWriter(c.Writer, cfg.RecordResponseBody, cfg.MaxBodySize)
		c.Writer = rw

		// 追踪中间件逻辑
		if cfg.EnableTracing {
			propagator := cfg.Propagator
			if propagator == nil {
				propagator = otel.GetTextMapPropagator()
			}

			ctx := propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

			spanName := c.Request.URL.Path
			if c.FullPath() != "" {
				spanName = c.FullPath()
			}

			tracer := cfg.Tracer
			if tracer == nil {
				tracer = otel.Tracer(cfg.ServiceName)
			}

			opts := []trace.SpanStartOption{
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPMethod(c.Request.Method),
					semconv.HTTPRoute(spanName),
					semconv.URLPath(c.Request.URL.Path),
					semconv.URLQuery(c.Request.URL.RawQuery),
					semconv.UserAgentOriginal(c.Request.UserAgent()),
					semconv.HTTPRequestBodySize(int(c.Request.ContentLength)),
					semconv.ClientAddress(c.ClientIP()),
				),
			}

			ctx, span := tracer.Start(ctx, spanName, opts...)
			defer span.End()

			c.Request = c.Request.WithContext(ctx)

			// 处理请求
			c.Next()

			// 设置 Span 属性
			span.SetAttributes(
				semconv.HTTPStatusCode(rw.status),
				semconv.HTTPResponseBodySize(rw.writtenSize),
			)

			// 记录响应体
			if cfg.RecordResponseBody && rw.body != nil {
				span.SetAttributes(
					attribute.String("http.response.body", rw.body.String()),
				)
			}

			// 记录错误
			if len(c.Errors) > 0 {
				span.SetStatus(codes.Error, "request failed")
				for _, err := range c.Errors {
					span.RecordError(err.Err)
				}
			} else if rw.status >= 400 {
				span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", rw.status))
			} else {
				span.SetStatus(codes.Ok, "")
			}
		} else {
			c.Next()
		}

		// 记录指标
		if cfg.EnableMetrics {
			status := strconv.Itoa(rw.status)
			duration := time.Since(start).Seconds()

			RecordHTTPRequest(c.Request.Method, c.FullPath(), status, c.HandlerName(), duration, rw.writtenSize)
		}
	}
}

// WAFMiddleware WAF 事件追踪中间件
func WAFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从上下文获取 Span
		span := trace.SpanFromContext(c.Request.Context())

		// 设置 WAF 相关属性
		span.SetAttributes(
			attribute.Bool("security.waf.enabled", true),
		)

		c.Next()
	}
}

// DDoSMiddleware DDoS 检测追踪中间件
func DDoSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		span := trace.SpanFromContext(c.Request.Context())

		span.SetAttributes(
			attribute.Bool("security.ddos.detection.enabled", true),
		)

		c.Next()
	}
}

// SetTelemetryGlobals 设置全局遥测实例
func SetTelemetryGlobals(tracer trace.Tracer, logger *zap.Logger) {
	tracerInstance = tracer
	loggerInstance = logger
	if tracer == nil {
		tracerInstance = GetTracer()
	}
	if logger == nil {
		loggerInstance = zap.NewNop()
	}
}

// GetTraceID 从上下文获取 TraceID
func GetTraceID(c *gin.Context) string {
	span := trace.SpanFromContext(c.Request.Context())
	return span.SpanContext().TraceID().String()
}

// GetSpanID 从上下文获取 SpanID
func GetSpanID(c *gin.Context) string {
	span := trace.SpanFromContext(c.Request.Context())
	return span.SpanContext().SpanID().String()
}

// InjectHeaders 将追踪信息注入到请求头
func InjectHeaders(c *gin.Context, headers http.Header) {
	propagator := otel.GetTextMapPropagator()
	propagator.Inject(c.Request.Context(), propagation.HeaderCarrier(headers))
}

// ExtractTraceID 从请求头提取 TraceID
func ExtractTraceID(headers http.Header) string {
	propagator := otel.GetTextMapPropagator()
	ctx := propagator.Extract(context.Background(), propagation.HeaderCarrier(headers))
	span := trace.SpanFromContext(ctx)
	return span.SpanContext().TraceID().String()
}
