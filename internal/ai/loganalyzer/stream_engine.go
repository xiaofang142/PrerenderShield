package loganalyzer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// StreamEngine 流处理引擎
type StreamEngine struct {
	config      *StreamConfig
	inputChan   <-chan *LogEntry
	outputChans map[string]chan<- *LogEntry
	processors  []LogProcessor
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	running     bool
	metrics     *StreamMetrics
	logger      *zap.Logger
}

// StreamConfig 流处理配置
type StreamConfig struct {
	WorkerCount     int           // 工作协程数量
	BatchSize       int           // 批量处理大小
	BatchTimeout    time.Duration // 批量超时
	EnableMetrics   bool          // 启用指标
	MetricsInterval time.Duration // 指标上报间隔
}

// LogProcessor 日志处理器接口
type LogProcessor interface {
	Process(ctx context.Context, entry *LogEntry) (*LogEntry, error)
	Name() string
}

// StreamMetrics 流处理指标
type StreamMetrics struct {
	mu              sync.RWMutex
	TotalProcessed  int64
	TotalErrors     int64
	ProcessedPerSec float64
	LastUpdateTime  time.Time
	ProcessorStats  map[string]*ProcessorStats
}

// ProcessorStats 处理器统计
type ProcessorStats struct {
	Name       string
	Processed  int64
	Errors     int64
	AvgLatency time.Duration
}

// DefaultStreamConfig 返回默认配置
func DefaultStreamConfig() *StreamConfig {
	return &StreamConfig{
		WorkerCount:     4,
		BatchSize:       100,
		BatchTimeout:    100 * time.Millisecond,
		EnableMetrics:   true,
		MetricsInterval: 10 * time.Second,
	}
}

// NewStreamEngine 创建流处理引擎
func NewStreamEngine(config *StreamConfig, inputChan <-chan *LogEntry, logger *zap.Logger) *StreamEngine {
	if config == nil {
		config = DefaultStreamConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())

	engine := &StreamEngine{
		config:      config,
		inputChan:   inputChan,
		outputChans: make(map[string]chan<- *LogEntry),
		processors:  make([]LogProcessor, 0),
		ctx:         ctx,
		cancel:      cancel,
		logger:      logger,
		metrics: &StreamMetrics{
			ProcessorStats: make(map[string]*ProcessorStats),
		},
	}

	return engine
}

// AddProcessor 添加处理器
func (e *StreamEngine) AddProcessor(processor LogProcessor) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if processor == nil {
		return fmt.Errorf("处理器不能为空")
	}

	e.processors = append(e.processors, processor)
	e.metrics.mu.Lock()
	e.metrics.ProcessorStats[processor.Name()] = &ProcessorStats{
		Name: processor.Name(),
	}
	e.metrics.mu.Unlock()

	e.logger.Info("添加处理器", zap.String("name", processor.Name()))
	return nil
}

// AddOutput 添加输出通道
func (e *StreamEngine) AddOutput(name string, output chan<- *LogEntry) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.outputChans[name] = output
}

// RemoveOutput 移除输出通道
func (e *StreamEngine) RemoveOutput(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.outputChans, name)
}

// Start 启动流处理引擎
func (e *StreamEngine) Start() error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return fmt.Errorf("流处理引擎已在运行")
	}
	e.running = true
	e.mu.Unlock()

	// 启动工作协程
	for i := 0; i < e.config.WorkerCount; i++ {
		e.wg.Add(1)
		go e.worker(i)
	}

	// 启动指标收集协程
	if e.config.EnableMetrics {
		e.wg.Add(1)
		go e.metricsWorker()
	}

	e.logger.Info("流处理引擎已启动",
		zap.Int("workers", e.config.WorkerCount),
		zap.Int("processors", len(e.processors)))

	return nil
}

// Stop 停止流处理引擎
func (e *StreamEngine) Stop() error {
	e.cancel()
	e.wg.Wait()

	e.mu.Lock()
	e.running = false
	e.mu.Unlock()

	e.logger.Info("流处理引擎已停止")
	return nil
}

// worker 工作协程
func (e *StreamEngine) worker(id int) {
	defer e.wg.Done()

	batch := make([]*LogEntry, 0, e.config.BatchSize)
	flushTimer := time.NewTimer(e.config.BatchTimeout)
	defer flushTimer.Stop()

	for {
		select {
		case <-e.ctx.Done():
			// 处理剩余日志
			if len(batch) > 0 {
				e.processBatch(batch)
			}
			return

		case entry, ok := <-e.inputChan:
			if !ok {
				if len(batch) > 0 {
					e.processBatch(batch)
				}
				return
			}

			batch = append(batch, entry)

			// 批量大小达到阈值，立即处理
			if len(batch) >= e.config.BatchSize {
				if !flushTimer.Stop() {
					<-flushTimer.C
				}
				e.processBatch(batch)
				batch = batch[:0]
				flushTimer.Reset(e.config.BatchTimeout)
			}

		case <-flushTimer.C:
			if len(batch) > 0 {
				e.processBatch(batch)
				batch = batch[:0]
			}
			flushTimer.Reset(e.config.BatchTimeout)
		}
	}
}

// processBatch 处理批量日志
func (e *StreamEngine) processBatch(batch []*LogEntry) {
	for _, entry := range batch {
		e.processEntry(entry)
	}
}

// processEntry 处理单条日志
func (e *StreamEngine) processEntry(entry *LogEntry) *LogEntry {
	startTime := time.Now()

	// 通过所有处理器
	for _, processor := range e.processors {
		select {
		case <-e.ctx.Done():
			return entry
		default:
		}

		processed, err := processor.Process(e.ctx, entry)
		e.updateProcessorStats(processor.Name(), err, time.Since(startTime))

		if err != nil {
			e.logger.Debug("处理器处理失败",
				zap.String("processor", processor.Name()),
				zap.Error(err))
			// 继续下一个处理器
			continue
		}

		if processed != nil {
			entry = processed
		}
	}

	// 发送到输出通道
	e.sendToOutputs(entry)

	e.metrics.mu.Lock()
	e.metrics.TotalProcessed++
	e.metrics.mu.Unlock()

	return entry
}

// sendToOutputs 发送到输出通道
func (e *StreamEngine) sendToOutputs(entry *LogEntry) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for name, output := range e.outputChans {
		select {
		case output <- entry:
		default:
			// 通道已满，跳过
			e.logger.Debug("输出通道已满", zap.String("name", name))
		}
	}
}

// updateProcessorStats 更新处理器统计
func (e *StreamEngine) updateProcessorStats(name string, err error, latency time.Duration) {
	e.metrics.mu.Lock()
	defer e.metrics.mu.Unlock()

	stats, ok := e.metrics.ProcessorStats[name]
	if !ok {
		return
	}

	stats.Processed++
	if err != nil {
		stats.Errors++
		e.metrics.TotalErrors++
	}

	// 简单移动平均
	stats.AvgLatency = (stats.AvgLatency*time.Duration(stats.Processed-1) + latency) / time.Duration(stats.Processed)
}

// GetMetrics 获取指标
func (e *StreamEngine) GetMetrics() *StreamMetrics {
	e.metrics.mu.RLock()
	defer e.metrics.mu.RUnlock()

	// 计算每秒处理数
	now := time.Now()
	if !e.metrics.LastUpdateTime.IsZero() {
		duration := now.Sub(e.metrics.LastUpdateTime).Seconds()
		if duration > 0 {
			e.metrics.ProcessedPerSec = float64(e.metrics.TotalProcessed) / duration
		}
	}
	e.metrics.LastUpdateTime = now

	// 深拷贝返回
	stats := make(map[string]*ProcessorStats)
	for k, v := range e.metrics.ProcessorStats {
		s := *v
		stats[k] = &s
	}

	return &StreamMetrics{
		TotalProcessed:  e.metrics.TotalProcessed,
		TotalErrors:     e.metrics.TotalErrors,
		ProcessedPerSec: e.metrics.ProcessedPerSec,
		LastUpdateTime:  e.metrics.LastUpdateTime,
		ProcessorStats:  stats,
	}
}

// metricsWorker 指标收集协程
func (e *StreamEngine) metricsWorker() {
	defer e.wg.Done()

	interval := e.config.MetricsInterval
	if interval <= 0 {
		interval = 10 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			metrics := e.GetMetrics()
			e.logger.Debug("流处理指标",
				zap.Int64("total_processed", metrics.TotalProcessed),
				zap.Int64("total_errors", metrics.TotalErrors),
				zap.Float64("processed_per_sec", metrics.ProcessedPerSec))
		}
	}
}

// GetStats 获取统计信息
func (e *StreamEngine) GetStats() map[string]interface{} {
	metrics := e.GetMetrics()
	return map[string]interface{}{
		"total_processed":   metrics.TotalProcessed,
		"total_errors":      metrics.TotalErrors,
		"processed_per_sec": metrics.ProcessedPerSec,
		"running":           e.running,
		"worker_count":      e.config.WorkerCount,
		"processor_count":   len(e.processors),
	}
}
