package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestExporterConfig_DefaultConfig(t *testing.T) {
	cfg := DefaultExporterConfig()
	assert.NotNil(t, cfg)
	assert.Equal(t, 10*time.Second, cfg.OTLPTimeout)
	assert.True(t, cfg.OTLPCompression)
	assert.Equal(t, 512, cfg.PrometheusBatchSize)
	assert.Equal(t, 5*time.Second, cfg.PrometheusFlushInterval)
	assert.False(t, cfg.LogExport)
	assert.Equal(t, zap.InfoLevel, cfg.LogExportLevel)
	assert.Equal(t, "json", cfg.FileExportFormat)
	assert.Equal(t, 2048, cfg.MaxQueueSize)
	assert.Equal(t, 5*time.Second, cfg.BatchTimeout)
	assert.Equal(t, 512, cfg.MaxExportBatchSize)
	assert.Equal(t, 3, cfg.RetryMaxAttempts)
	assert.Equal(t, time.Second, cfg.RetryInitialInterval)
}

func TestMultiExporter_NoExporters(t *testing.T) {
	cfg := &ExporterConfig{}
	logger, _ := zap.NewDevelopment()

	exporter := NewMultiExporter(cfg, logger)
	assert.NotNil(t, exporter)

	// 没有导出器时，ExportSpans 应该返回 nil
	ctx := context.Background()
	err := exporter.ExportSpans(ctx, nil)
	assert.NoError(t, err)

	// Shutdown 也应该返回 nil
	err = exporter.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestMultiExporter_WithLogExporter(t *testing.T) {
	cfg := &ExporterConfig{
		LogExport:      true,
		LogExportLevel: zap.InfoLevel,
	}
	logger, _ := zap.NewDevelopment()

	exporter := NewMultiExporter(cfg, logger)
	assert.NotNil(t, exporter)
	assert.Len(t, exporter.exporters, 1)

	ctx := context.Background()
	err := exporter.ExportSpans(ctx, nil)
	assert.NoError(t, err)

	err = exporter.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestMultiExporter_WithFileExporter(t *testing.T) {
	cfg := &ExporterConfig{
		FileExportPath: "/tmp/telemetry_test.json",
		FileExportFormat: "json",
	}
	logger, _ := zap.NewDevelopment()

	exporter := NewMultiExporter(cfg, logger)
	// 文件导出器可能因为路径问题创建失败，但不应该 panic
	assert.NotNil(t, exporter)

	ctx := context.Background()
	err := exporter.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestMultiExporter_ConcurrentExport(t *testing.T) {
	cfg := &ExporterConfig{
		LogExport: true,
	}
	logger, _ := zap.NewDevelopment()

	exporter := NewMultiExporter(cfg, logger)
	assert.NotNil(t, exporter)

	ctx := context.Background()

	// 并发导出
	done := make(chan bool, 3)
	for i := 0; i < 3; i++ {
		go func() {
			err := exporter.ExportSpans(ctx, nil)
			assert.NoError(t, err)
			done <- true
		}()
	}

	for i := 0; i < 3; i++ {
		<-done
	}

	err := exporter.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestOTLPExporter_New(t *testing.T) {
	cfg := &ExporterConfig{
		OTLPEndpoint: "http://localhost:4318",
		OTLPTimeout:  5 * time.Second,
		OTLPHeaders:  map[string]string{"Authorization": "Bearer token"},
	}
	logger, _ := zap.NewDevelopment()

	exporter := NewOTLPExporter(cfg, logger)
	assert.NotNil(t, exporter)
	assert.Equal(t, "http://localhost:4318", exporter.endpoint)
	assert.NotNil(t, exporter.client)
	assert.NotNil(t, exporter.headers)
}

func TestOTLPExporter_NilConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	exporter := NewOTLPExporter(nil, logger)
	assert.NotNil(t, exporter)
}

func TestOTLPExporter_ExportSpans(t *testing.T) {
	cfg := &ExporterConfig{
		OTLPEndpoint: "http://localhost:4318",
		OTLPTimeout:  100 * time.Millisecond, // 短时间超时
	}
	logger, _ := zap.NewDevelopment()

	exporter := NewOTLPExporter(cfg, logger)
	assert.NotNil(t, exporter)

	ctx := context.Background()
	// 由于端点不可达，应该返回错误
	err := exporter.ExportSpans(ctx, nil)
	// 不应该 panic，但可能返回错误
	_ = err
}

func TestOTLPExporter_Shutdown(t *testing.T) {
	cfg := &ExporterConfig{
		OTLPEndpoint: "http://localhost:4318",
	}
	logger, _ := zap.NewDevelopment()

	exporter := NewOTLPExporter(cfg, logger)
	ctx := context.Background()
	err := exporter.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestSpansToOTLPProto(t *testing.T) {
	// 使用空的 spans 测试，因为创建有效的 ReadOnlySpan 很复杂
	spans := []sdktrace.ReadOnlySpan{}

	protoSpans := spansToOTLPProto(spans)
	assert.Empty(t, protoSpans)
}

func TestSpansToOTLPProto_Empty(t *testing.T) {
	protoSpans := spansToOTLPProto(nil)
	assert.Empty(t, protoSpans)
}

func TestPrometheusRemoteWriteExporter_New(t *testing.T) {
	cfg := &ExporterConfig{
		PrometheusRemoteWriteURL: "http://localhost:9090/api/v1/write",
		PrometheusBatchSize:      256,
		PrometheusFlushInterval:  2 * time.Second,
		OTLPTimeout:              5 * time.Second,
	}
	logger, _ := zap.NewDevelopment()

	exporter := NewPrometheusRemoteWriteExporter(cfg, logger)
	assert.NotNil(t, exporter)
	assert.Equal(t, "http://localhost:9090/api/v1/write", exporter.url)
	assert.Equal(t, 256, exporter.batchSize)
	assert.NotNil(t, exporter.flushTicker)
}

func TestPrometheusRemoteWriteExporter_Flush(t *testing.T) {
	cfg := &ExporterConfig{
		PrometheusRemoteWriteURL: "http://localhost:9090/api/v1/write",
		PrometheusBatchSize:      10,
		PrometheusFlushInterval:  100 * time.Millisecond,
		OTLPTimeout:              100 * time.Millisecond,
	}
	logger, _ := zap.NewDevelopment()

	exporter := NewPrometheusRemoteWriteExporter(cfg, logger)
	assert.NotNil(t, exporter)

	// 等待自动刷新
	time.Sleep(150 * time.Millisecond)

	ctx := context.Background()
	err := exporter.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestPrometheusRemoteWriteExporter_Shutdown(t *testing.T) {
	cfg := &ExporterConfig{
		PrometheusRemoteWriteURL: "http://localhost:9090/api/v1/write",
		PrometheusFlushInterval:  5 * time.Second,
	}
	logger, _ := zap.NewDevelopment()

	exporter := NewPrometheusRemoteWriteExporter(cfg, logger)
	ctx := context.Background()
	err := exporter.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestLogExporter_New(t *testing.T) {
	cfg := &ExporterConfig{
		LogExport:      true,
		LogExportLevel: zap.DebugLevel,
	}
	logger, _ := zap.NewDevelopment()

	exporter := NewLogExporter(cfg, logger)
	assert.NotNil(t, exporter)
	assert.Equal(t, zap.DebugLevel, exporter.level)
}

func TestLogExporter_ExportSpans(t *testing.T) {
	cfg := &ExporterConfig{
		LogExport:      true,
		LogExportLevel: zap.InfoLevel,
	}
	logger, _ := zap.NewDevelopment()

	exporter := NewLogExporter(cfg, logger)
	ctx := context.Background()

	// 空 spans 不应该 panic
	err := exporter.ExportSpans(ctx, nil)
	assert.NoError(t, err)

	// 有 spans 也不应该 panic - 使用 nil 测试
	err = exporter.ExportSpans(ctx, nil)
	assert.NoError(t, err)
}

func TestLogExporter_ExportSpans_DifferentLevels(t *testing.T) {
	levels := []zapcore.Level{
		zap.DebugLevel,
		zap.InfoLevel,
		zap.WarnLevel,
		zap.ErrorLevel,
	}

	for _, level := range levels {
		cfg := &ExporterConfig{
			LogExport:      true,
			LogExportLevel: level,
		}
		logger, _ := zap.NewDevelopment()
		exporter := NewLogExporter(cfg, logger)

		ctx := context.Background()

		// 不应该 panic
		assert.NotPanics(t, func() {
			err := exporter.ExportSpans(ctx, nil)
			_ = err
		})
	}
}

func TestLogExporter_Shutdown(t *testing.T) {
	cfg := &ExporterConfig{
		LogExport: true,
	}
	logger, _ := zap.NewDevelopment()

	exporter := NewLogExporter(cfg, logger)
	ctx := context.Background()
	err := exporter.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestFileExporter_InvalidPath(t *testing.T) {
	cfg := &ExporterConfig{
		FileExportPath: "/invalid/path/that/does/not/exist/file.json",
		FileExportFormat: "json",
	}
	logger, _ := zap.NewDevelopment()

	exporter := NewFileExporter(cfg, logger)
	// 无效路径应该返回 nil 或处理错误
	if exporter != nil {
		ctx := context.Background()
		err := exporter.Shutdown(ctx)
		assert.NoError(t, err)
	}
}

func TestFileExporter_ValidPath(t *testing.T) {
	cfg := &ExporterConfig{
		FileExportPath: "/tmp/telemetry_test_export.json",
		FileExportFormat: "json",
	}
	logger, _ := zap.NewDevelopment()

	exporter := NewFileExporter(cfg, logger)
	if exporter == nil {
		t.Skip("File exporter is nil, skipping test")
	}

	ctx := context.Background()
	err := exporter.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestFileExporter_TextFormat(t *testing.T) {
	cfg := &ExporterConfig{
		FileExportPath: "/tmp/telemetry_test_export.txt",
		FileExportFormat: "text",
	}
	logger, _ := zap.NewDevelopment()

	exporter := NewFileExporter(cfg, logger)
	if exporter == nil {
		t.Skip("File exporter is nil, skipping test")
	}

	ctx := context.Background()
	err := exporter.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestFileExporter_DefaultFormat(t *testing.T) {
	cfg := &ExporterConfig{
		FileExportPath: "/tmp/telemetry_test_export_default.json",
		FileExportFormat: "", // 空格式应该使用默认值
	}
	logger, _ := zap.NewDevelopment()

	exporter := NewFileExporter(cfg, logger)
	if exporter == nil {
		t.Skip("File exporter is nil, skipping test")
	}

	ctx := context.Background()
	err := exporter.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestAddAttributes(t *testing.T) {
	attrs := make([]attribute.KeyValue, 0)
	AddAttributes(&attrs, "key", "value")
	assert.Len(t, attrs, 1)
	assert.Equal(t, attribute.Key("key"), attrs[0].Key)
	assert.Equal(t, "value", attrs[0].Value.AsString())
}

func TestAddIntAttribute(t *testing.T) {
	attrs := make([]attribute.KeyValue, 0)
	AddIntAttribute(&attrs, "count", 42)
	assert.Len(t, attrs, 1)
	assert.Equal(t, int64(42), attrs[0].Value.AsInt64())
}

func TestAddFloatAttribute(t *testing.T) {
	attrs := make([]attribute.KeyValue, 0)
	AddFloatAttribute(&attrs, "ratio", 0.5)
	assert.Len(t, attrs, 1)
	assert.Equal(t, 0.5, attrs[0].Value.AsFloat64())
}

func TestAddBoolAttribute(t *testing.T) {
	attrs := make([]attribute.KeyValue, 0)
	AddBoolAttribute(&attrs, "enabled", true)
	assert.Len(t, attrs, 1)
	assert.Equal(t, true, attrs[0].Value.AsBool())
}
