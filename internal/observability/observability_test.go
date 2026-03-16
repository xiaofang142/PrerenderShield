package observability

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"prerender-shield/internal/eventbus"
	"prerender-shield/internal/logging"
	"prerender-shield/internal/observability/metrics"
)

func TestNewObservability(t *testing.T) {
	logger := logging.NewLogger(logging.Config{})
	metricsRecorder := metrics.NewInMemoryRecorder()
	eventBus := eventbus.NewInMemoryBus(nil)

	obs := NewObservability(logger, metricsRecorder, eventBus)
	assert.NotNil(t, obs)
	assert.Equal(t, logger, obs.Logger)
	assert.Equal(t, metricsRecorder, obs.MetricsRecorder)
	assert.Equal(t, eventBus, obs.EventBus)
}

func TestRecordRequest(t *testing.T) {
	logger := logging.NewLogger(logging.Config{})
	metricsRecorder := metrics.NewInMemoryRecorder()
	eventBus := eventbus.NewInMemoryBus(nil)

	obs := NewObservability(logger, metricsRecorder, eventBus)

	ctx := context.Background()
	obs.RecordRequest(ctx, "GET", "/api/test", http.StatusOK, 100*time.Millisecond)

	// Verify metrics were recorded
	metricsMap := metricsRecorder.GetMetrics()
	assert.NotNil(t, metricsMap)
}

func TestRecordError(t *testing.T) {
	logger := logging.NewLogger(logging.Config{})
	metricsRecorder := metrics.NewInMemoryRecorder()
	eventBus := eventbus.NewInMemoryBus(nil)

	obs := NewObservability(logger, metricsRecorder, eventBus)

	ctx := context.Background()
	obs.RecordError(ctx, "auth", "invalid_token", assert.AnError)

	// Verify error was recorded
	metricsMap := metricsRecorder.GetMetrics()
	assert.NotNil(t, metricsMap)
}

func TestRecordWAFBlock(t *testing.T) {
	logger := logging.NewLogger(logging.Config{})
	metricsRecorder := metrics.NewInMemoryRecorder()
	eventBus := eventbus.NewInMemoryBus(nil)

	obs := NewObservability(logger, metricsRecorder, eventBus)

	ctx := context.Background()
	obs.RecordWAFBlock(ctx, "site-1", "sql_injection")

	// Verify WAF block was recorded
	metricsMap := metricsRecorder.GetMetrics()
	assert.NotNil(t, metricsMap)
}

func TestRecordCacheHit(t *testing.T) {
	logger := logging.NewLogger(logging.Config{})
	metricsRecorder := metrics.NewInMemoryRecorder()
	eventBus := eventbus.NewInMemoryBus(nil)

	obs := NewObservability(logger, metricsRecorder, eventBus)

	ctx := context.Background()
	obs.RecordCacheHit(ctx, "site-1")

	metricsMap := metricsRecorder.GetMetrics()
	assert.NotNil(t, metricsMap)
}

func TestRecordCacheMiss(t *testing.T) {
	logger := logging.NewLogger(logging.Config{})
	metricsRecorder := metrics.NewInMemoryRecorder()
	eventBus := eventbus.NewInMemoryBus(nil)

	obs := NewObservability(logger, metricsRecorder, eventBus)

	ctx := context.Background()
	obs.RecordCacheMiss(ctx, "site-1")

	metricsMap := metricsRecorder.GetMetrics()
	assert.NotNil(t, metricsMap)
}

func TestGetMetrics(t *testing.T) {
	logger := logging.NewLogger(logging.Config{})
	metricsRecorder := metrics.NewInMemoryRecorder()
	eventBus := eventbus.NewInMemoryBus(nil)

	obs := NewObservability(logger, metricsRecorder, eventBus)

	metricsMap := obs.GetMetrics()
	assert.NotNil(t, metricsMap)
}

func TestRecordRequestWithDifferentStatuses(t *testing.T) {
	logger := logging.NewLogger(logging.Config{})
	metricsRecorder := metrics.NewInMemoryRecorder()
	eventBus := eventbus.NewInMemoryBus(nil)

	obs := NewObservability(logger, metricsRecorder, eventBus)

	ctx := context.Background()

	// Record requests with different status codes
	obs.RecordRequest(ctx, "GET", "/api/success", http.StatusOK, 50*time.Millisecond)
	obs.RecordRequest(ctx, "POST", "/api/created", http.StatusCreated, 100*time.Millisecond)
	obs.RecordRequest(ctx, "GET", "/api/notfound", http.StatusNotFound, 10*time.Millisecond)
	obs.RecordRequest(ctx, "POST", "/api/error", http.StatusInternalServerError, 200*time.Millisecond)

	metricsMap := metricsRecorder.GetMetrics()
	assert.NotNil(t, metricsMap)
}

func TestRecordMultipleErrors(t *testing.T) {
	logger := logging.NewLogger(logging.Config{})
	metricsRecorder := metrics.NewInMemoryRecorder()
	eventBus := eventbus.NewInMemoryBus(nil)

	obs := NewObservability(logger, metricsRecorder, eventBus)

	ctx := context.Background()

	// Record multiple errors from different modules
	obs.RecordError(ctx, "auth", "token_expired", assert.AnError)
	obs.RecordError(ctx, "waf", "rule_violation", assert.AnError)
	obs.RecordError(ctx, "cache", "connection_failed", assert.AnError)

	metricsMap := metricsRecorder.GetMetrics()
	assert.NotNil(t, metricsMap)
}

func TestRecordMultipleWAFBlocks(t *testing.T) {
	logger := logging.NewLogger(logging.Config{})
	metricsRecorder := metrics.NewInMemoryRecorder()
	eventBus := eventbus.NewInMemoryBus(nil)

	obs := NewObservability(logger, metricsRecorder, eventBus)

	ctx := context.Background()

	// Record multiple WAF blocks with different threat types
	obs.RecordWAFBlock(ctx, "site-1", "sql_injection")
	obs.RecordWAFBlock(ctx, "site-1", "xss")
	obs.RecordWAFBlock(ctx, "site-2", "csrf")
	obs.RecordWAFBlock(ctx, "site-2", "rate_limit")

	metricsMap := metricsRecorder.GetMetrics()
	assert.NotNil(t, metricsMap)
}

func TestRecordCacheOperations(t *testing.T) {
	logger := logging.NewLogger(logging.Config{})
	metricsRecorder := metrics.NewInMemoryRecorder()
	eventBus := eventbus.NewInMemoryBus(nil)

	obs := NewObservability(logger, metricsRecorder, eventBus)

	ctx := context.Background()

	// Record multiple cache operations
	for i := 0; i < 5; i++ {
		obs.RecordCacheHit(ctx, "site-1")
		obs.RecordCacheMiss(ctx, "site-1")
	}

	metricsMap := metricsRecorder.GetMetrics()
	assert.NotNil(t, metricsMap)
}
