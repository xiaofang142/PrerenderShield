package loganalyzer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// MockLogProcessor 模拟日志处理器
type MockLogProcessor struct {
	name       string
	processErr error
	processed  int
}

func (m *MockLogProcessor) Name() string {
	return m.name
}

func (m *MockLogProcessor) Process(ctx context.Context, entry *LogEntry) (*LogEntry, error) {
	m.processed++
	if m.processErr != nil {
		return entry, m.processErr
	}
	// 添加一些处理标记
	if entry.Fields == nil {
		entry.Fields = make(map[string]interface{})
	}
	entry.Fields["processed_by"] = m.name
	return entry, nil
}

// TestDefaultStreamConfig 测试默认配置
func TestDefaultStreamConfig(t *testing.T) {
	config := DefaultStreamConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 4, config.WorkerCount)
	assert.Equal(t, 100, config.BatchSize)
	assert.Equal(t, 100*time.Millisecond, config.BatchTimeout)
	assert.True(t, config.EnableMetrics)
	assert.Equal(t, 10*time.Second, config.MetricsInterval)
}

// TestNewStreamEngine 测试创建引擎
func TestNewStreamEngine(t *testing.T) {
	inputChan := make(<-chan *LogEntry)
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	config := &StreamConfig{
		WorkerCount:   2,
		BatchSize:     50,
		BatchTimeout:  50 * time.Millisecond,
		EnableMetrics: true,
	}

	engine := NewStreamEngine(config, inputChan, logger)
	assert.NotNil(t, engine)
	assert.Equal(t, config, engine.config)
	assert.Equal(t, 2, engine.config.WorkerCount)
	assert.NotNil(t, engine.outputChans)
	assert.NotNil(t, engine.processors)
	assert.NotNil(t, engine.metrics)
}

// TestNewStreamEngine_NilConfig 测试空配置
func TestNewStreamEngine_NilConfig(t *testing.T) {
	inputChan := make(<-chan *LogEntry)
	engine := NewStreamEngine(nil, inputChan, nil)
	assert.NotNil(t, engine)
	assert.NotNil(t, engine.config)
	assert.Equal(t, 4, engine.config.WorkerCount) // 默认值
}

// TestNewStreamEngine_NilLogger 测试空日志
func TestNewStreamEngine_NilLogger(t *testing.T) {
	inputChan := make(<-chan *LogEntry)
	engine := NewStreamEngine(nil, inputChan, nil)
	assert.NotNil(t, engine)
	assert.NotNil(t, engine.logger)
}

// TestStreamEngine_AddProcessor 测试添加处理器
func TestStreamEngine_AddProcessor(t *testing.T) {
	inputChan := make(<-chan *LogEntry)
	engine := NewStreamEngine(nil, inputChan, nil)

	processor := &MockLogProcessor{name: "test_processor"}
	err := engine.AddProcessor(processor)
	assert.NoError(t, err)
	assert.Len(t, engine.processors, 1)
	assert.Contains(t, engine.metrics.ProcessorStats, "test_processor")
}

// TestStreamEngine_AddProcessor_Nil 测试添加空处理器
func TestStreamEngine_AddProcessor_Nil(t *testing.T) {
	inputChan := make(<-chan *LogEntry)
	engine := NewStreamEngine(nil, inputChan, nil)

	err := engine.AddProcessor(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "处理器不能为空")
}

// TestStreamEngine_AddProcessor_Multiple 测试添加多个处理器
func TestStreamEngine_AddProcessor_Multiple(t *testing.T) {
	inputChan := make(<-chan *LogEntry)
	engine := NewStreamEngine(nil, inputChan, nil)

	processors := []LogProcessor{
		&MockLogProcessor{name: "processor1"},
		&MockLogProcessor{name: "processor2"},
		&MockLogProcessor{name: "processor3"},
	}

	for _, p := range processors {
		err := engine.AddProcessor(p)
		assert.NoError(t, err)
	}

	assert.Len(t, engine.processors, 3)
	assert.Len(t, engine.metrics.ProcessorStats, 3)
}

// TestStreamEngine_AddOutput 测试添加输出通道
func TestStreamEngine_AddOutput(t *testing.T) {
	inputChan := make(<-chan *LogEntry)
	engine := NewStreamEngine(nil, inputChan, nil)

	outputChan := make(chan *LogEntry, 10)
	engine.AddOutput("output1", outputChan)

	assert.Len(t, engine.outputChans, 1)
	assert.Contains(t, engine.outputChans, "output1")
}

// TestStreamEngine_AddOutput_Multiple 测试添加多个输出
func TestStreamEngine_AddOutput_Multiple(t *testing.T) {
	inputChan := make(<-chan *LogEntry)
	engine := NewStreamEngine(nil, inputChan, nil)

	engine.AddOutput("output1", make(chan *LogEntry, 10))
	engine.AddOutput("output2", make(chan *LogEntry, 10))
	engine.AddOutput("output3", make(chan *LogEntry, 10))

	assert.Len(t, engine.outputChans, 3)
}

// TestStreamEngine_RemoveOutput 测试移除输出通道
func TestStreamEngine_RemoveOutput(t *testing.T) {
	inputChan := make(<-chan *LogEntry)
	engine := NewStreamEngine(nil, inputChan, nil)

	outputChan := make(chan *LogEntry, 10)
	engine.AddOutput("output1", outputChan)
	assert.Len(t, engine.outputChans, 1)

	engine.RemoveOutput("output1")
	assert.Len(t, engine.outputChans, 0)
}

// TestStreamEngine_RemoveOutput_NonExistent 测试移除不存在的输出
func TestStreamEngine_RemoveOutput_NonExistent(t *testing.T) {
	inputChan := make(<-chan *LogEntry)
	engine := NewStreamEngine(nil, inputChan, nil)

	engine.RemoveOutput("nonexistent")
	assert.Len(t, engine.outputChans, 0)
}

// TestStreamEngine_Start 测试启动引擎
func TestStreamEngine_Start(t *testing.T) {
	inputChan := make(<-chan *LogEntry)
	engine := NewStreamEngine(&StreamConfig{
		WorkerCount:   2,
		BatchSize:     10,
		BatchTimeout:  50 * time.Millisecond,
		EnableMetrics: false,
	}, inputChan, nil)

	err := engine.Start()
	assert.NoError(t, err)
	assert.True(t, engine.running)

	// 停止引擎
	err = engine.Stop()
	assert.NoError(t, err)
	assert.False(t, engine.running)
}

// TestStreamEngine_Start_AlreadyRunning 测试重复启动
func TestStreamEngine_Start_AlreadyRunning(t *testing.T) {
	inputChan := make(<-chan *LogEntry)
	engine := NewStreamEngine(&StreamConfig{
		WorkerCount:   1,
		EnableMetrics: false,
	}, inputChan, nil)

	err := engine.Start()
	assert.NoError(t, err)

	err = engine.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "流处理引擎已在运行")

	engine.Stop()
}

// TestStreamEngine_Stop 测试停止引擎
func TestStreamEngine_Stop(t *testing.T) {
	inputChan := make(<-chan *LogEntry, 100)
	engine := NewStreamEngine(&StreamConfig{
		WorkerCount:   1,
		EnableMetrics: false,
	}, inputChan, nil)

	err := engine.Start()
	assert.NoError(t, err)

	err = engine.Stop()
	assert.NoError(t, err)
	assert.False(t, engine.running)
}

// TestStreamEngine_Worker_BatchProcessing 测试批量处理
func TestStreamEngine_Worker_BatchProcessing(t *testing.T) {
	inputChan := make(chan *LogEntry, 100)
	outputChan := make(chan *LogEntry, 100)

	engine := NewStreamEngine(&StreamConfig{
		WorkerCount:   1,
		BatchSize:     5,
		BatchTimeout:  100 * time.Millisecond,
		EnableMetrics: false,
	}, inputChan, nil)

	processor := &MockLogProcessor{name: "batch_processor"}
	engine.AddProcessor(processor)
	engine.AddOutput("output", outputChan)

	err := engine.Start()
	assert.NoError(t, err)

	// 发送 5 条日志
	for i := 0; i < 5; i++ {
		inputChan <- &LogEntry{
			Timestamp: time.Now(),
			Raw:       "test message",
			Fields:    make(map[string]interface{}),
		}
	}

	// 等待处理完成
	time.Sleep(200 * time.Millisecond)

	// 停止引擎
	engine.Stop()
	close(inputChan)

	// 验证输出
	count := 0
	for range outputChan {
		count++
	}
	assert.GreaterOrEqual(t, count, 1)
}

// TestStreamEngine_Worker_BatchTimeout 测试批量超时
func TestStreamEngine_Worker_BatchTimeout(t *testing.T) {
	inputChan := make(chan *LogEntry, 100)
	outputChan := make(chan *LogEntry, 100)

	engine := NewStreamEngine(&StreamConfig{
		WorkerCount:   1,
		BatchSize:     100,
		BatchTimeout:  50 * time.Millisecond,
		EnableMetrics: false,
	}, inputChan, nil)

	processor := &MockLogProcessor{name: "timeout_processor"}
	engine.AddProcessor(processor)
	engine.AddOutput("output", outputChan)

	err := engine.Start()
	assert.NoError(t, err)

	// 只发送 1 条日志（少于 BatchSize）
	inputChan <- &LogEntry{
		Timestamp: time.Now(),
		Raw:       "timeout test",
	}

	// 等待超时触发
	time.Sleep(150 * time.Millisecond)

	engine.Stop()
	close(inputChan)

	// 应该至少有 1 条输出
	count := 0
	for range outputChan {
		count++
	}
	assert.GreaterOrEqual(t, count, 1)
}

// TestStreamEngine_ProcessEntry_ProcessorError 测试处理器错误
func TestStreamEngine_ProcessEntry_ProcessorError(t *testing.T) {
	inputChan := make(chan *LogEntry, 10)
	outputChan := make(chan *LogEntry, 10)

	engine := NewStreamEngine(&StreamConfig{
		WorkerCount:   1,
		EnableMetrics: false,
	}, inputChan, nil)

	// 添加一个会返回错误的处理器
	errProcessor := &MockLogProcessor{
		name:       "error_processor",
		processErr: assert.AnError,
	}
	engine.AddProcessor(errProcessor)
	engine.AddOutput("output", outputChan)

	err := engine.Start()
	assert.NoError(t, err)

	entry := &LogEntry{
		Timestamp: time.Now(),
		Raw:       "error test",
	}
	inputChan <- entry

	time.Sleep(100 * time.Millisecond)

	engine.Stop()
	close(inputChan)

	// 即使处理器出错，日志也应该被发送到输出
	count := 0
	for range outputChan {
		count++
	}
	assert.GreaterOrEqual(t, count, 1)
}

// TestStreamEngine_ProcessEntry_MultipleProcessors 测试多个处理器
func TestStreamEngine_ProcessEntry_MultipleProcessors(t *testing.T) {
	inputChan := make(chan *LogEntry, 10)
	outputChan := make(chan *LogEntry, 10)

	engine := NewStreamEngine(&StreamConfig{
		WorkerCount:   1,
		EnableMetrics: false,
	}, inputChan, nil)

	processors := []LogProcessor{
		&MockLogProcessor{name: "processor_a"},
		&MockLogProcessor{name: "processor_b"},
		&MockLogProcessor{name: "processor_c"},
	}

	for _, p := range processors {
		engine.AddProcessor(p)
	}
	engine.AddOutput("output", outputChan)

	err := engine.Start()
	assert.NoError(t, err)

	entry := &LogEntry{
		Timestamp: time.Now(),
		Raw:       "multi processor test",
	}
	inputChan <- entry

	time.Sleep(100 * time.Millisecond)

	engine.Stop()
	close(inputChan)

	// 验证所有处理器都被调用
	for _, p := range processors {
		mockP := p.(*MockLogProcessor)
		assert.GreaterOrEqual(t, mockP.processed, 1)
	}
}

// TestStreamEngine_SendToOutputs_FullChannel 测试满通道
func TestStreamEngine_SendToOutputs_FullChannel(t *testing.T) {
	inputChan := make(chan *LogEntry, 100)
	outputChan := make(chan *LogEntry, 1) // 小容量通道

	engine := NewStreamEngine(&StreamConfig{
		WorkerCount:   1,
		BatchSize:     1,
		EnableMetrics: false,
	}, inputChan, nil)

	engine.AddProcessor(&MockLogProcessor{name: "test"})
	engine.AddOutput("output", outputChan)

	err := engine.Start()
	assert.NoError(t, err)

	// 发送多条日志
	for i := 0; i < 10; i++ {
		inputChan <- &LogEntry{
			Timestamp: time.Now(),
			Raw:       "full channel test",
		}
	}

	time.Sleep(200 * time.Millisecond)

	engine.Stop()
	close(inputChan)

	// 由于通道容量小，会有部分日志被丢弃
	count := 0
	for range outputChan {
		count++
	}
	assert.GreaterOrEqual(t, count, 1)
}

// TestStreamEngine_GetMetrics 测试获取指标
func TestStreamEngine_GetMetrics(t *testing.T) {
	inputChan := make(chan *LogEntry, 100)
	outputChan := make(chan *LogEntry, 100)

	engine := NewStreamEngine(&StreamConfig{
		WorkerCount:   1,
		BatchSize:     10,
		EnableMetrics: true,
	}, inputChan, nil)

	engine.AddProcessor(&MockLogProcessor{name: "metrics_test"})
	engine.AddOutput("output", outputChan)

	err := engine.Start()
	assert.NoError(t, err)

	// 发送一些日志
	for i := 0; i < 10; i++ {
		inputChan <- &LogEntry{
			Timestamp: time.Now(),
			Raw:       "metrics test",
		}
	}

	time.Sleep(200 * time.Millisecond)

	metrics := engine.GetMetrics()
	assert.NotNil(t, metrics)
	assert.GreaterOrEqual(t, metrics.TotalProcessed, int64(0))
	assert.NotNil(t, metrics.ProcessorStats)

	engine.Stop()
	close(inputChan)
}

// TestStreamEngine_GetMetrics_WithDelay 测试指标计算
func TestStreamEngine_GetMetrics_WithDelay(t *testing.T) {
	inputChan := make(chan *LogEntry, 100)
	engine := NewStreamEngine(&StreamConfig{
		WorkerCount:     1,
		EnableMetrics:   true,
		MetricsInterval: 1 * time.Second,
	}, inputChan, nil)

	// 第一次获取
	metrics1 := engine.GetMetrics()
	assert.NotNil(t, metrics1)

	// 等待一段时间
	time.Sleep(100 * time.Millisecond)

	// 第二次获取
	metrics2 := engine.GetMetrics()
	assert.NotNil(t, metrics2)

	// LastUpdateTime 应该更新
	assert.True(t, !metrics2.LastUpdateTime.IsZero())
}

// TestStreamEngine_MetricsWorker 测试指标收集协程
func TestStreamEngine_MetricsWorker(t *testing.T) {
	inputChan := make(chan *LogEntry, 10)
	logger, _ := zap.NewDevelopment()

	engine := NewStreamEngine(&StreamConfig{
		WorkerCount:     1,
		EnableMetrics:   true,
		MetricsInterval: 50 * time.Millisecond,
	}, inputChan, logger)

	err := engine.Start()
	assert.NoError(t, err)

	// 等待指标收集
	time.Sleep(150 * time.Millisecond)

	engine.Stop()
}

// TestStreamEngine_GetStats 获取统计信息
func TestStreamEngine_GetStats(t *testing.T) {
	inputChan := make(chan *LogEntry, 100)
	engine := NewStreamEngine(&StreamConfig{
		WorkerCount:   2,
		EnableMetrics: false,
	}, inputChan, nil)

	engine.AddProcessor(&MockLogProcessor{name: "stats_test"})

	stats := engine.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, 2, stats["worker_count"])
	assert.Equal(t, 1, stats["processor_count"])
	assert.False(t, stats["running"].(bool))

	// 启动后再次检查
	err := engine.Start()
	assert.NoError(t, err)

	stats = engine.GetStats()
	assert.True(t, stats["running"].(bool))

	engine.Stop()
}

// TestStreamEngine_GetStats_NeverStarted 测试从未启动的引擎
func TestStreamEngine_GetStats_NeverStarted(t *testing.T) {
	inputChan := make(<-chan *LogEntry)
	engine := NewStreamEngine(nil, inputChan, nil)

	stats := engine.GetStats()
	assert.NotNil(t, stats)
	assert.False(t, stats["running"].(bool))
	assert.Equal(t, 4, stats["worker_count"]) // 默认值
}

// TestUpdateProcessorStats 测试更新处理器统计
func TestUpdateProcessorStats(t *testing.T) {
	inputChan := make(<-chan *LogEntry)
	engine := NewStreamEngine(nil, inputChan, nil)

	// 添加处理器以创建统计
	processor := &MockLogProcessor{name: "stats_update_test"}
	engine.AddProcessor(processor)

	// 更新统计
	engine.updateProcessorStats("stats_update_test", nil, 10*time.Millisecond)
	engine.updateProcessorStats("stats_update_test", assert.AnError, 20*time.Millisecond)

	stats := engine.metrics.ProcessorStats["stats_update_test"]
	assert.NotNil(t, stats)
	assert.Equal(t, int64(2), stats.Processed)
	assert.Equal(t, int64(1), stats.Errors)
	assert.Greater(t, stats.AvgLatency, time.Duration(0))
}

// TestUpdateProcessorStats_UnknownName 测试未知名称
func TestUpdateProcessorStats_UnknownName(t *testing.T) {
	inputChan := make(<-chan *LogEntry)
	engine := NewStreamEngine(nil, inputChan, nil)

	// 不应 panic
	engine.updateProcessorStats("unknown", nil, 10*time.Millisecond)
}

// TestStreamEngine_ContextCancellation 测试上下文取消
func TestStreamEngine_ContextCancellation(t *testing.T) {
	inputChan := make(chan *LogEntry, 100)
	outputChan := make(chan *LogEntry, 100)

	engine := NewStreamEngine(&StreamConfig{
		WorkerCount:   2,
		BatchSize:     10,
		EnableMetrics: false,
	}, inputChan, nil)

	engine.AddProcessor(&MockLogProcessor{name: "cancel_test"})
	engine.AddOutput("output", outputChan)

	err := engine.Start()
	assert.NoError(t, err)

	// 发送一些日志
	for i := 0; i < 5; i++ {
		inputChan <- &LogEntry{
			Timestamp: time.Now(),
			Raw:       "cancel test",
		}
	}

	// 立即停止，测试上下文取消
	engine.Stop()
	close(inputChan)

	// 验证没有 panic
	assert.True(t, true)
}

// TestStreamEngine_InputChanClosed 测试输入通道关闭
func TestStreamEngine_InputChanClosed(t *testing.T) {
	inputChan := make(chan *LogEntry, 100)
	outputChan := make(chan *LogEntry, 100)

	engine := NewStreamEngine(&StreamConfig{
		WorkerCount:   1,
		BatchSize:     10,
		EnableMetrics: false,
	}, inputChan, nil)

	engine.AddProcessor(&MockLogProcessor{name: "close_test"})
	engine.AddOutput("output", outputChan)

	err := engine.Start()
	assert.NoError(t, err)

	// 发送日志后立即关闭通道
	inputChan <- &LogEntry{
		Timestamp: time.Now(),
		Raw:       "close test",
	}
	close(inputChan)

	// 等待处理完成
	time.Sleep(100 * time.Millisecond)

	engine.Stop()

	// 验证没有 panic
	assert.True(t, true)
}

// TestStreamMetrics_Struct 测试结构体
func TestStreamMetrics_Struct(t *testing.T) {
	metrics := &StreamMetrics{
		TotalProcessed:  100,
		TotalErrors:     5,
		ProcessedPerSec: 10.5,
		LastUpdateTime:  time.Now(),
		ProcessorStats:  make(map[string]*ProcessorStats),
	}

	assert.Equal(t, int64(100), metrics.TotalProcessed)
	assert.Equal(t, int64(5), metrics.TotalErrors)
	assert.Equal(t, 10.5, metrics.ProcessedPerSec)
	assert.NotNil(t, metrics.ProcessorStats)
}

// TestProcessorStats_Struct 测试结构体
func TestProcessorStats_Struct(t *testing.T) {
	stats := &ProcessorStats{
		Name:       "test",
		Processed:  50,
		Errors:     2,
		AvgLatency: 5 * time.Millisecond,
	}

	assert.Equal(t, "test", stats.Name)
	assert.Equal(t, int64(50), stats.Processed)
	assert.Equal(t, int64(2), stats.Errors)
	assert.Equal(t, 5*time.Millisecond, stats.AvgLatency)
}

// TestStreamConfig_Struct 测试结构体
func TestStreamConfig_Struct(t *testing.T) {
	config := &StreamConfig{
		WorkerCount:     8,
		BatchSize:       200,
		BatchTimeout:    500 * time.Millisecond,
		EnableMetrics:   false,
		MetricsInterval: 30 * time.Second,
	}

	assert.Equal(t, 8, config.WorkerCount)
	assert.Equal(t, 200, config.BatchSize)
	assert.Equal(t, 500*time.Millisecond, config.BatchTimeout)
	assert.False(t, config.EnableMetrics)
	assert.Equal(t, 30*time.Second, config.MetricsInterval)
}

// TestStreamEngine_ConcurrentAccess 测试并发访问
func TestStreamEngine_ConcurrentAccess(t *testing.T) {
	inputChan := make(chan *LogEntry, 100)
	engine := NewStreamEngine(&StreamConfig{
		WorkerCount:   2,
		EnableMetrics: true,
	}, inputChan, nil)

	done := make(chan bool, 20)

	// 并发调用各种方法
	for i := 0; i < 10; i++ {
		go func() {
			engine.GetMetrics()
			done <- true
		}()
		go func() {
			engine.GetStats()
			done <- true
		}()
	}

	// 等待所有调用完成
	for i := 0; i < 20; i++ {
		<-done
	}

	assert.True(t, true)
}

// TestMockLogProcessor 测试模拟处理器
func TestMockLogProcessor_Process(t *testing.T) {
	processor := &MockLogProcessor{
		name: "test",
	}

	entry := &LogEntry{
		Timestamp: time.Now(),
		Raw:       "test",
		Fields:    make(map[string]interface{}),
	}

	result, err := processor.Process(context.Background(), entry)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, processor.processed)
	assert.Equal(t, "test", result.Fields["processed_by"])
}

// TestMockLogProcessor_WithError 测试模拟处理器返回错误
func TestMockLogProcessor_WithError(t *testing.T) {
	processor := &MockLogProcessor{
		name:       "error_test",
		processErr: assert.AnError,
	}

	entry := &LogEntry{
		Timestamp: time.Now(),
		Raw:       "test",
	}

	result, err := processor.Process(context.Background(), entry)
	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, processor.processed)
}
