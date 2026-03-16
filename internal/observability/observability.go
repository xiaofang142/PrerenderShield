package observability

import (
	"context"
	"time"

	"prerender-shield/internal/eventbus"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/observability/metrics"
)

// Observability 可观测性组件
type Observability struct {
	Logger          *logging.Logger
	MetricsRecorder *metrics.InMemoryRecorder
	EventBus        *eventbus.InMemoryBus
}

// NewObservability 创建可观测性组件
func NewObservability(
	logger *logging.Logger,
	metricsRecorder *metrics.InMemoryRecorder,
	eventBus *eventbus.InMemoryBus,
) *Observability {
	return &Observability{
		Logger:          logger,
		MetricsRecorder: metricsRecorder,
		EventBus:        eventBus,
	}
}

// RecordRequest 记录请求
func (o *Observability) RecordRequest(ctx context.Context, method, path string, status int, duration time.Duration) {
	// 记录指标
	o.MetricsRecorder.RecordRequestDuration(method, path, status, duration)
	o.MetricsRecorder.RecordRequestCount(method, path, status)

	// 记录日志
	o.Logger.Info("Request completed: method=%s path=%s status=%d duration_ms=%d",
		method, path, status, duration.Milliseconds())
}

// RecordError 记录错误
func (o *Observability) RecordError(ctx context.Context, module, errorType string, err error) {
	o.MetricsRecorder.RecordError(module, errorType)
	o.Logger.Error("Error occurred: module=%s error_type=%s error=%v", module, errorType, err)
}

// RecordWAFBlock 记录 WAF 阻断
func (o *Observability) RecordWAFBlock(ctx context.Context, siteID, threatType string) {
	o.MetricsRecorder.RecordWAFBlock(siteID, threatType)
	o.Logger.Warn("WAF blocked request: site_id=%s threat_type=%s", siteID, threatType)

	// 发布事件
	o.EventBus.Publish(ctx, eventbus.TopicWAFBlocked,
		eventbus.NewEvent(eventbus.TopicWAFBlocked, "waf",
			map[string]interface{}{
				"site_id":   siteID,
				"threat_type": threatType,
			}))
}

// RecordCacheHit 记录缓存命中
func (o *Observability) RecordCacheHit(ctx context.Context, siteID string) {
	o.MetricsRecorder.RecordCacheHit(siteID)
}

// RecordCacheMiss 记录缓存未命中
func (o *Observability) RecordCacheMiss(ctx context.Context, siteID string) {
	o.MetricsRecorder.RecordCacheMiss(siteID)
}

// GetMetrics 获取指标
func (o *Observability) GetMetrics() map[string]interface{} {
	return o.MetricsRecorder.GetMetrics()
}
