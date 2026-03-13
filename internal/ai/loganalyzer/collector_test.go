package loganalyzer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultCollectorConfig(t *testing.T) {
	config := DefaultCollectorConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 10000, config.BufferSize)
	assert.Equal(t, 1000, config.MaxBatchSize)
	assert.Equal(t, 5*time.Second, config.FlushInterval)
}

func TestNewCollector(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &CollectorConfig{
		BufferSize:    1000,
		MaxBatchSize:  100,
		FlushInterval: 1 * time.Second,
	}

	collector := NewCollector(config, logger)
	assert.NotNil(t, collector)
	assert.Equal(t, config, collector.config)
	assert.NotNil(t, collector.outputChan)
	assert.False(t, collector.running)

	// 测试 nil config
	collectorNil := NewCollector(nil, nil)
	assert.NotNil(t, collectorNil)
}

func TestCollector_AddSource(t *testing.T) {
	collector := NewCollector(nil, nil)

	// 测试添加 nil source
	err := collector.AddSource(nil)
	assert.Error(t, err)

	// 测试添加有效 source
	channelSource := NewChannelLogSource("test", make(<-chan string))
	err = collector.AddSource(channelSource)
	assert.Nil(t, err)
	assert.Len(t, collector.sources, 1)
}

func TestCollector_StartStop(t *testing.T) {
	collector := NewCollector(nil, nil)

	// 测试启动
	err := collector.Start()
	assert.Nil(t, err)
	assert.True(t, collector.running)

	// 测试重复启动
	err = collector.Start()
	assert.Error(t, err)

	// 测试停止
	err = collector.Stop()
	assert.Nil(t, err)
	assert.False(t, collector.running)
}

func TestCollector_OutputChan(t *testing.T) {
	collector := NewCollector(nil, nil)
	outputChan := collector.OutputChan()
	assert.NotNil(t, outputChan)

	// 发送一个条目测试
	entry := &LogEntry{
		ID:         "test-1",
		SourceType: "access",
		Fields:     map[string]interface{}{"test": "value"},
	}
	collector.outputChan <- entry

	received := <-outputChan
	assert.Equal(t, "test-1", received.ID)
}

func TestCollector_ReadSource(t *testing.T) {
	rawChan := make(chan string, 10)
	collector := NewCollector(nil, nil)

	// 创建一个测试 channel source
	channelSource := NewChannelLogSource("test", rawChan)
	collector.AddSource(channelSource)

	// 启动采集器
	err := collector.Start()
	assert.Nil(t, err)

	// 发送日志
	rawChan <- `{"remote_addr":"192.168.1.1","method":"GET"}`
	close(rawChan)

	time.Sleep(100 * time.Millisecond)
	collector.Stop()
}

func TestNewFileLogSource(t *testing.T) {
	// 测试不存在的文件
	_, err := NewFileLogSource("test", "/nonexistent/path.log", "nginx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "文件不存在")

	// 测试创建成功的文件日志源（使用现有文件）
	source, err := NewFileLogSource("test", "/etc/hosts", "nginx")
	assert.Nil(t, err)
	assert.NotNil(t, source)
	assert.Equal(t, "test", source.Name())
}

func TestNewChannelLogSource(t *testing.T) {
	inputChan := make(<-chan string)
	source := NewChannelLogSource("test-channel", inputChan)
	assert.NotNil(t, source)
	assert.Equal(t, "test-channel", source.Name())
}

func TestChannelLogSource_Read(t *testing.T) {
	inputChan := make(chan string, 10)
	outputChan := make(chan string, 10)
	source := NewChannelLogSource("test", inputChan)

	ctx, cancel := context.WithCancel(context.Background())
	errChan := make(chan error, 1)

	go func() {
		errChan <- source.Read(ctx, outputChan)
	}()

	// 发送数据
	inputChan <- "test log line"
	time.Sleep(50 * time.Millisecond)

	// 接收数据
	select {
	case output := <-outputChan:
		assert.Equal(t, "test log line", output)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for output")
	}

	cancel()
	<-errChan
}

func TestChannelLogSource_Close(t *testing.T) {
	source := NewChannelLogSource("test", make(<-chan string))
	err := source.Close()
	assert.Nil(t, err)
}

func TestFileLogSource_Close(t *testing.T) {
	source := &FileLogSource{name: "test", path: "/tmp/test.log"}
	err := source.Close()
	assert.Nil(t, err)
}

func TestGetApacheLogPattern(t *testing.T) {
	re := getApacheLogPattern()
	assert.NotNil(t, re)

	// 测试解析 Apache 日志
	logLine := `192.168.1.1 - - [10/Oct/2024:13:55:36 +0800] "GET /index.html HTTP/1.1" 200 1024 "-" "Mozilla/5.0"`
	matches := re.FindStringSubmatch(logLine)
	assert.NotNil(t, matches)
}

func TestParseLogEntry_Empty(t *testing.T) {
	entry := ParseLogEntry("", "test")
	assert.Nil(t, entry)
}

func TestParseLogEntry_UnknownFormat(t *testing.T) {
	raw := `some unknown log format`
	entry := ParseLogEntry(raw, "test")
	assert.NotNil(t, entry)
	assert.Equal(t, "unknown", entry.SourceType)
	assert.Equal(t, raw, entry.Fields["message"])
}

func TestParseLogEntry_Security(t *testing.T) {
	raw := `{"threat_type":"sql_injection","threat_level":"high"}`
	entry := ParseLogEntry(raw, "test")
	assert.NotNil(t, entry)
	assert.Equal(t, "security", entry.SourceType)
}

func TestParseLogEntry_Render(t *testing.T) {
	raw := `{"render_time":500.0,"cache_hit":true}`
	entry := ParseLogEntry(raw, "test")
	assert.NotNil(t, entry)
	assert.Equal(t, "render", entry.SourceType)
}

func TestTrimNewline(t *testing.T) {
	assert.Equal(t, "test", trimNewline("test\n"))
	assert.Equal(t, "test", trimNewline("test"))
	assert.Equal(t, "", trimNewline("\n"))
}

func TestExtractNamedGroups(t *testing.T) {
	re := getNginxLogPattern()
	matches := re.FindStringSubmatch(`192.168.1.1 - - [10/Oct/2024:13:55:36 +0800] "GET /api HTTP/1.1" 200 1024 "-" "Mozilla/5.0"`)
	result := extractNamedGroups(re, matches)
	assert.NotNil(t, result)
	assert.Contains(t, result, "remote_addr")
	assert.Equal(t, "192.168.1.1", result["remote_addr"])
}

func TestExtractNamedGroups_Empty(t *testing.T) {
	re := getNginxLogPattern()
	result := extractNamedGroups(re, []string{})
	assert.Empty(t, result)
}

func TestGenerateLogID(t *testing.T) {
	id1 := generateLogID()
	assert.Contains(t, id1, "log-")

	time.Sleep(1 * time.Nanosecond)
	id2 := generateLogID()
	assert.NotEqual(t, id1, id2)
}

func TestToString(t *testing.T) {
	assert.Equal(t, "hello", toString("hello"))
	assert.Equal(t, "123", toString(123))
	assert.Equal(t, "123", toString(int64(123)))
	assert.Equal(t, "123.45", toString(123.45))
	assert.Equal(t, "", toString(nil))
}

func TestToInt(t *testing.T) {
	assert.Equal(t, 123, toInt(123))
	assert.Equal(t, 123, toInt(int64(123)))
	assert.Equal(t, 123, toInt(123.45))
	assert.Equal(t, 123, toInt("123"))
	assert.Equal(t, 0, toInt(nil))
	assert.Equal(t, 0, toInt("invalid"))
}

func TestToInt64(t *testing.T) {
	assert.Equal(t, int64(123), toInt64(123))
	assert.Equal(t, int64(123), toInt64(int64(123)))
	assert.Equal(t, int64(123), toInt64(123.45))
	assert.Equal(t, int64(123), toInt64("123"))
	assert.Equal(t, int64(0), toInt64(nil))
	assert.Equal(t, int64(0), toInt64("invalid"))
}

func TestToFloat64(t *testing.T) {
	assert.Equal(t, 123.45, toFloat64(123.45))
	assert.Equal(t, 123.0, toFloat64(123))
	assert.Equal(t, 123.0, toFloat64(int64(123)))
	assert.Equal(t, 123.45, toFloat64("123.45"))
	assert.Equal(t, 0.0, toFloat64(nil))
	assert.Equal(t, 0.0, toFloat64("invalid"))
}
