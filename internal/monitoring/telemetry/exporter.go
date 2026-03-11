package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ExporterConfig 导出器配置
type ExporterConfig struct {
	// OTLP 配置
	OTLPEndpoint     string
	OTLPHeaders      map[string]string
	OTLPTimeout      time.Duration
	OTLPCompression  bool

	// Prometheus 远程写入配置
	PrometheusRemoteWriteURL     string
	PrometheusRemoteWriteHeaders map[string]string
	PrometheusBatchSize          int
	PrometheusFlushInterval      time.Duration

	// 日志导出配置
	LogExport      bool
	LogExportLevel zapcore.Level // zapcore.Level 类型

	// 文件导出配置
	FileExportPath   string
	FileExportFormat string // json, text

	// 通用配置
	MaxQueueSize       int
	BatchTimeout       time.Duration
	MaxExportBatchSize int
	RetryMaxAttempts   int
	RetryInitialInterval time.Duration
}

// DefaultExporterConfig 返回默认配置
func DefaultExporterConfig() *ExporterConfig {
	return &ExporterConfig{
		OTLPTimeout:        10 * time.Second,
		OTLPCompression:    true,
		PrometheusBatchSize: 512,
		PrometheusFlushInterval: 5 * time.Second,
		LogExport:          false,
		LogExportLevel:     zap.InfoLevel,
		FileExportFormat:   "json",
		MaxQueueSize:       2048,
		BatchTimeout:       5 * time.Second,
		MaxExportBatchSize: 512,
		RetryMaxAttempts:   3,
		RetryInitialInterval: time.Second,
	}
}

// Exporter 统一导出器接口
type Exporter interface {
	ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error
	Shutdown(ctx context.Context) error
}

// MultiExporter 多路导出器
type MultiExporter struct {
	exporters []Exporter
	logger    *zap.Logger
}

// NewMultiExporter 创建多路导出器
func NewMultiExporter(cfg *ExporterConfig, log *zap.Logger) *MultiExporter {
	if log == nil {
		log = zap.NewNop()
	}

	multi := &MultiExporter{
		exporters: make([]Exporter, 0),
		logger:    log,
	}

	// 添加 OTLP 导出器
	if cfg.OTLPEndpoint != "" {
		otlpExporter := NewOTLPExporter(cfg, log)
		multi.exporters = append(multi.exporters, otlpExporter)
		log.Info("启用 OTLP 导出器", zap.String("endpoint", cfg.OTLPEndpoint))
	}

	// 添加 Prometheus 远程写入导出器
	if cfg.PrometheusRemoteWriteURL != "" {
		promExporter := NewPrometheusRemoteWriteExporter(cfg, log)
		multi.exporters = append(multi.exporters, promExporter)
		log.Info("启用 Prometheus 远程写入导出器", zap.String("url", cfg.PrometheusRemoteWriteURL))
	}

	// 添加日志导出器
	if cfg.LogExport {
		logExporter := NewLogExporter(cfg, log)
		multi.exporters = append(multi.exporters, logExporter)
		log.Info("启用日志导出器", zap.String("level", cfg.LogExportLevel.String()))
	}

	// 添加文件导出器
	if cfg.FileExportPath != "" {
		fileExporter := NewFileExporter(cfg, log)
		multi.exporters = append(multi.exporters, fileExporter)
		log.Info("启用文件导出器", zap.String("path", cfg.FileExportPath))
	}

	return multi
}

// ExportSpans 导出 Span 到所有导出器
func (m *MultiExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	if len(m.exporters) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(m.exporters))

	for _, exporter := range m.exporters {
		wg.Add(1)
		go func(exp Exporter) {
			defer wg.Done()
			if err := exp.ExportSpans(ctx, spans); err != nil {
				errChan <- err
			}
		}(exporter)
	}

	wg.Wait()
	close(errChan)

	// 收集错误
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("导出错误：%v", errs)
	}

	return nil
}

// Shutdown 关闭所有导出器
func (m *MultiExporter) Shutdown(ctx context.Context) error {
	var errs []error
	for _, exporter := range m.exporters {
		if err := exporter.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("关闭导出器错误：%v", errs)
	}

	return nil
}

// ==================== OTLP 导出器 ====================

// OTLPExporter OTLP 导出器
type OTLPExporter struct {
	client   *http.Client
	endpoint string
	headers  map[string]string
	logger   *zap.Logger
}

// NewOTLPExporter 创建 OTLP 导出器
func NewOTLPExporter(cfg *ExporterConfig, log *zap.Logger) *OTLPExporter {
	return &OTLPExporter{
		client: &http.Client{
			Timeout: cfg.OTLPTimeout,
		},
		endpoint: cfg.OTLPEndpoint,
		headers:  cfg.OTLPHeaders,
		logger:   log,
	}
}

// ExportSpans 导出 Span
func (e *OTLPExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	// 转换为 OTLP 格式
	protoSpans := spansToOTLPProto(spans)

	data, err := json.Marshal(protoSpans)
	if err != nil {
		return fmt.Errorf("序列化 Span 失败：%w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("创建请求失败：%w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range e.headers {
		req.Header.Set(k, v)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败：%w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OTLP 导出失败，状态码：%d, 响应：%s", resp.StatusCode, string(body))
	}

	return nil
}

// Shutdown 关闭导出器
func (e *OTLPExporter) Shutdown(ctx context.Context) error {
	e.client.CloseIdleConnections()
	return nil
}

// OTLP Span 数据结构
type otlpSpan struct {
	TraceID           string            `json:"trace_id"`
	SpanID            string            `json:"span_id"`
	ParentSpanID      string            `json:"parent_span_id,omitempty"`
	Name              string            `json:"name"`
	Kind              string            `json:"kind"`
	StartTimeUnixNano int64             `json:"start_time_unix_nano"`
	EndTimeUnixNano   int64             `json:"end_time_unix_nano"`
	Attributes        map[string]string `json:"attributes,omitempty"`
	Status            string            `json:"status,omitempty"`
	StatusMessage     string            `json:"status_message,omitempty"`
}

func spansToOTLPProto(spans []sdktrace.ReadOnlySpan) []otlpSpan {
	result := make([]otlpSpan, 0, len(spans))
	for _, span := range spans {
		attrs := make(map[string]string)
		for _, attr := range span.Attributes() {
			attrs[string(attr.Key)] = attr.Value.AsString()
		}

		s := otlpSpan{
			TraceID:           span.SpanContext().TraceID().String(),
			SpanID:            span.SpanContext().SpanID().String(),
			Name:              span.Name(),
			Kind:              span.SpanKind().String(),
			StartTimeUnixNano: span.StartTime().UnixNano(),
			EndTimeUnixNano:   span.EndTime().UnixNano(),
			Attributes:        attrs,
		}

		if span.Parent().IsValid() {
			s.ParentSpanID = span.Parent().SpanID().String()
		}

		status := span.Status()
		if status.Code != 0 {
			s.Status = status.Code.String()
			s.StatusMessage = status.Description
		}

		result = append(result, s)
	}

	return result
}

// ==================== Prometheus 远程写入导出器 ====================

// PrometheusRemoteWriteExporter Prometheus 远程写入导出器
type PrometheusRemoteWriteExporter struct {
	client       *http.Client
	url          string
	headers      map[string]string
	batchSize    int
	flushTicker  *time.Ticker
	metricBuffer []prometheusMetric
	mu           sync.Mutex
	logger       *zap.Logger
	done         chan struct{}
}

type prometheusMetric struct {
	Name      string
	Labels    map[string]string
	Value     float64
	Timestamp int64
}

// NewPrometheusRemoteWriteExporter 创建 Prometheus 远程写入导出器
func NewPrometheusRemoteWriteExporter(cfg *ExporterConfig, log *zap.Logger) *PrometheusRemoteWriteExporter {
	exporter := &PrometheusRemoteWriteExporter{
		client: &http.Client{
			Timeout: cfg.OTLPTimeout,
		},
		url:          cfg.PrometheusRemoteWriteURL,
		headers:      cfg.PrometheusRemoteWriteHeaders,
		batchSize:    cfg.PrometheusBatchSize,
		metricBuffer: make([]prometheusMetric, 0, cfg.PrometheusBatchSize),
		logger:       log,
		done:         make(chan struct{}),
	}

	// 启动定时刷新
	exporter.flushTicker = time.NewTicker(cfg.PrometheusFlushInterval)
	go exporter.flushLoop()

	return exporter
}

func (e *PrometheusRemoteWriteExporter) flushLoop() {
	for {
		select {
		case <-e.flushTicker.C:
			e.flush()
		case <-e.done:
			e.flushTicker.Stop()
			return
		}
	}
}

func (e *PrometheusRemoteWriteExporter) flush() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.metricBuffer) == 0 {
		return
	}

	// 构建 Prometheus 远程写入格式
	var buf bytes.Buffer
	for _, m := range e.metricBuffer {
		labels := make([]string, 0, len(m.Labels))
		for k, v := range m.Labels {
			labels = append(labels, fmt.Sprintf("%s=%q", k, v))
		}
		fmt.Fprintf(&buf, "%s{%s} %v %d\n", m.Name, labels, m.Value, m.Timestamp)
	}

	// 发送请求
	req, err := http.NewRequest("POST", e.url, &buf)
	if err != nil {
		e.logger.Error("创建 Prometheus 请求失败", zap.Error(err))
		return
	}

	req.Header.Set("Content-Type", "text/plain")
	for k, v := range e.headers {
		req.Header.Set(k, v)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		e.logger.Error("发送 Prometheus 请求失败", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		e.logger.Error("Prometheus 远程写入失败",
			zap.Int("status", resp.StatusCode),
			zap.String("response", string(body)),
		)
		return
	}

	e.metricBuffer = e.metricBuffer[:0]
	e.logger.Debug("Prometheus 远程写入成功", zap.Int("metrics", len(e.metricBuffer)))
}

// ExportSpans 导出 Span (转换为指标)
func (e *PrometheusRemoteWriteExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, span := range spans {
		// 将 Span 转换为指标
		metric := prometheusMetric{
			Name:      "trace_span_duration_seconds",
			Labels:    make(map[string]string),
			Value:     span.EndTime().Sub(span.StartTime()).Seconds(),
			Timestamp: span.EndTime().UnixMilli(),
		}

		metric.Labels["span_name"] = span.Name()
		metric.Labels["span_kind"] = span.SpanKind().String()

		for _, attr := range span.Attributes() {
			metric.Labels[string(attr.Key)] = attr.Value.AsString()
		}

		e.metricBuffer = append(e.metricBuffer, metric)

		// 批量发送
		if len(e.metricBuffer) >= e.batchSize {
			e.mu.Unlock()
			e.flush()
			e.mu.Lock()
		}
	}

	return nil
}

// Shutdown 关闭导出器
func (e *PrometheusRemoteWriteExporter) Shutdown(ctx context.Context) error {
	close(e.done)
	e.flush()
	e.client.CloseIdleConnections()
	return nil
}

// ==================== 日志导出器 ====================

// LogExporter 日志导出器
type LogExporter struct {
	logger *zap.Logger
	level  zapcore.Level
}

// NewLogExporter 创建日志导出器
func NewLogExporter(cfg *ExporterConfig, log *zap.Logger) *LogExporter {
	return &LogExporter{
		logger: log,
		level:  cfg.LogExportLevel,
	}
}

// ExportSpans 导出 Span 到日志
func (e *LogExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	for _, span := range spans {
		fields := []zap.Field{
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.String("span_id", span.SpanContext().SpanID().String()),
			zap.String("name", span.Name()),
			zap.String("kind", span.SpanKind().String()),
			zap.Duration("duration", span.EndTime().Sub(span.StartTime())),
		}

		// 添加属性
		for _, attr := range span.Attributes() {
			fields = append(fields, zap.String(string(attr.Key), attr.Value.AsString()))
		}

		// 添加状态
		status := span.Status()
		if status.Code != 0 {
			fields = append(fields, zap.String("status", status.Code.String()))
			fields = append(fields, zap.String("status_message", status.Description))
		}

		switch e.level {
		case zapcore.DebugLevel:
			e.logger.Debug("Span", fields...)
		case zapcore.InfoLevel:
			e.logger.Info("Span", fields...)
		case zapcore.WarnLevel:
			e.logger.Warn("Span", fields...)
		case zapcore.ErrorLevel:
			e.logger.Error("Span", fields...)
		}
	}

	return nil
}

// Shutdown 关闭导出器
func (e *LogExporter) Shutdown(ctx context.Context) error {
	return nil
}

// ==================== 文件导出器 ====================

// FileExporter 文件导出器
type FileExporter struct {
	file   *os.File
	format string
	logger *zap.Logger
	mu     sync.Mutex
}

// NewFileExporter 创建文件导出器
func NewFileExporter(cfg *ExporterConfig, log *zap.Logger) *FileExporter {
	file, err := os.OpenFile(cfg.FileExportPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Error("创建文件导出器失败", zap.Error(err))
		return nil
	}

	return &FileExporter{
		file:   file,
		format: cfg.FileExportFormat,
		logger: log,
	}
}

// ExportSpans 导出 Span 到文件
func (e *FileExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, span := range spans {
		var data []byte
		var err error

		switch e.format {
		case "json":
			data, err = e.spanToJSON(span)
		case "text":
			data = e.spanToText(span)
		default:
			data, err = e.spanToJSON(span)
		}

		if err != nil {
			e.logger.Error("序列化 Span 失败", zap.Error(err))
			continue
		}

		data = append(data, '\n')
		if _, err := e.file.Write(data); err != nil {
			e.logger.Error("写入文件失败", zap.Error(err))
			return err
		}
	}

	return nil
}

func (e *FileExporter) spanToJSON(span sdktrace.ReadOnlySpan) ([]byte, error) {
	type spanJSON struct {
		TraceID    string            `json:"trace_id"`
		SpanID     string            `json:"span_id"`
		Name       string            `json:"name"`
		Kind       string            `json:"kind"`
		StartTime  time.Time         `json:"start_time"`
		EndTime    time.Time         `json:"end_time"`
		Duration   time.Duration     `json:"duration"`
		Attributes map[string]string `json:"attributes,omitempty"`
	}

	attrs := make(map[string]string)
	for _, attr := range span.Attributes() {
		attrs[string(attr.Key)] = attr.Value.AsString()
	}

	s := spanJSON{
		TraceID:    span.SpanContext().TraceID().String(),
		SpanID:     span.SpanContext().SpanID().String(),
		Name:       span.Name(),
		Kind:       span.SpanKind().String(),
		StartTime:  span.StartTime(),
		EndTime:    span.EndTime(),
		Duration:   span.EndTime().Sub(span.StartTime()),
		Attributes: attrs,
	}

	return json.Marshal(s)
}

func (e *FileExporter) spanToText(span sdktrace.ReadOnlySpan) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "[%s] trace_id=%s span_id=%s name=%s kind=%s duration=%v",
		span.StartTime().Format(time.RFC3339),
		span.SpanContext().TraceID().String(),
		span.SpanContext().SpanID().String(),
		span.Name(),
		span.SpanKind().String(),
		span.EndTime().Sub(span.StartTime()),
	)

	for _, attr := range span.Attributes() {
		fmt.Fprintf(&buf, " %s=%s", attr.Key, attr.Value.AsString())
	}

	return buf.Bytes()
}

// Shutdown 关闭导出器
func (e *FileExporter) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.file != nil {
		return e.file.Close()
	}
	return nil
}

// AddAttributes 添加属性到 Span
func AddAttributes(attrs *[]attribute.KeyValue, key string, value string) {
	*attrs = append(*attrs, attribute.String(key, value))
}

func AddIntAttribute(attrs *[]attribute.KeyValue, key string, value int64) {
	*attrs = append(*attrs, attribute.Int64(key, value))
}

func AddFloatAttribute(attrs *[]attribute.KeyValue, key string, value float64) {
	*attrs = append(*attrs, attribute.Float64(key, value))
}

func AddBoolAttribute(attrs *[]attribute.KeyValue, key string, value bool) {
	*attrs = append(*attrs, attribute.Bool(key, value))
}
