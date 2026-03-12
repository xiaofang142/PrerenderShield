package streaming

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type testFlushWriter struct {
	*httptest.ResponseRecorder
}

func (t *testFlushWriter) Flush() error {
	return nil
}

func TestDefaultChunkedRendererConfig(t *testing.T) {
	config := DefaultChunkedRendererConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 4096, config.ChunkSize)
	assert.Equal(t, 100*time.Millisecond, config.FlushInterval)
	assert.Equal(t, 30*time.Second, config.Timeout)
}

func TestNewChunkedRenderer(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultChunkedRendererConfig()

	renderer := NewChunkedRenderer(config, logger)

	assert.NotNil(t, renderer)
	assert.NotNil(t, renderer.chunks)
	assert.NotNil(t, renderer.stats)
}

func TestChunkedRenderer_Render(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewChunkedRenderer(DefaultChunkedRendererConfig(), logger)

	ctx := context.Background()
	content := strings.NewReader("Hello, World! This is a test content for streaming rendering.")

	recorder := httptest.NewRecorder()
	writer := &testFlushWriter{recorder}

	chunkCount := 0
	err := renderer.Render(ctx, content, writer, func(chunk *RenderChunk) {
		chunkCount++
	})

	assert.NoError(t, err)
	assert.Greater(t, chunkCount, 0)

	stats := renderer.GetStats()
	assert.Greater(t, stats.TotalChunks, int64(0))
	assert.Greater(t, stats.TotalBytes, int64(0))
}

func TestChunkedRenderer_RenderHTML(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewChunkedRenderer(DefaultChunkedRendererConfig(), logger)

	ctx := context.Background()
	html := "<html><head><title>Test</title></head><body><h1>Hello</h1></body></html>"

	recorder := httptest.NewRecorder()
	writer := &testFlushWriter{recorder}

	err := renderer.RenderHTML(ctx, html, writer)

	assert.NoError(t, err)
	assert.Contains(t, recorder.Body.String(), "<!DOCTYPE html>")
	assert.Contains(t, recorder.Body.String(), "Hello")

	stats := renderer.GetStats()
	assert.Greater(t, stats.TotalChunks, int64(0))
}

func TestChunkedRenderer_Render_Timeout(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultChunkedRendererConfig()
	config.Timeout = 10 * time.Millisecond
	renderer := NewChunkedRenderer(config, logger)

	ctx := context.Background()

	// 创建一个慢速 reader，数据量大于 0 以便实际读取
	slowReader := &slowReader{data: make([]byte, 1000), delay: 50 * time.Millisecond}

	recorder := httptest.NewRecorder()
	writer := &testFlushWriter{recorder}

	err := renderer.Render(ctx, slowReader, writer, nil)

	// 不检查具体错误消息，只检查是否出错
	assert.Error(t, err)
}

func TestChunkedRenderer_Render_WriteError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewChunkedRenderer(DefaultChunkedRendererConfig(), logger)

	ctx := context.Background()
	content := strings.NewReader("test content")

	// 错误写入器
	errorWriter := &errorWriter{}

	err := renderer.Render(ctx, content, errorWriter, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "写入分块失败")

	stats := renderer.GetStats()
	assert.Greater(t, stats.Errors, int64(0))
}

func TestChunkedTransferEncoder(t *testing.T) {
	var buf bytes.Buffer
	encoder := NewChunkedTransferEncoder(&buf)

	// 编码分块
	data := []byte("Hello, Chunked Transfer!")
	err := encoder.Encode(data)

	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "18") // 16 进制的 24

	// 完成编码
	err = encoder.Finalize()

	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "0\r\n\r\n")
}

func TestBufferPool(t *testing.T) {
	pool := NewBufferPool(4096)

	// 获取缓冲区
	buf1 := pool.Get()
	assert.NotNil(t, buf1)

	// 使用缓冲区
	buf1.WriteString("test data")

	// 放回
	pool.Put(buf1)

	// 再次获取（应该被重用）
	buf2 := pool.Get()
	assert.NotNil(t, buf2)
	assert.Equal(t, 0, buf2.Len()) // 应该已重置
}

func TestStreamingResponseWriter(t *testing.T) {
	var buf bytes.Buffer
	flushCalled := false

	flushFunc := func() error {
		flushCalled = true
		return nil
	}

	writer := NewStreamingResponseWriter(&buf, flushFunc, 10)

	// 写入小数据（不应刷新）
	n, err := writer.Write([]byte("small"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.False(t, flushCalled)

	// 写入更多数据触发刷新
	n, err = writer.Write([]byte(" more data to exceed chunk size"))
	assert.NoError(t, err)
	assert.True(t, flushCalled)

	assert.Greater(t, writer.WrittenBytes(), 0)

	// 关闭应该刷新
	err = writer.Close()
	assert.NoError(t, err)
}

func TestStreamingResponseWriter_FlushError(t *testing.T) {
	errorFlush := func() error {
		return fmt.Errorf("flush error")
	}

	var buf bytes.Buffer
	writer := NewStreamingResponseWriter(&buf, errorFlush, 10)

	// 写入触发刷新
	_, err := writer.Write([]byte("data that exceeds chunk size"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "flush error")
}

func TestChunkedRenderer_ConcurrentRendering(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewChunkedRenderer(DefaultChunkedRendererConfig(), logger)

	ctx := context.Background()
	done := make(chan bool, 5)

	for i := 0; i < 5; i++ {
		go func(idx int) {
			content := strings.NewReader(fmt.Sprintf("Content from goroutine %d", idx))
			recorder := httptest.NewRecorder()
			writer := &testFlushWriter{recorder}

			err := renderer.Render(ctx, content, writer, nil)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// 等待所有完成
	for i := 0; i < 5; i++ {
		<-done
	}

	stats := renderer.GetStats()
	assert.GreaterOrEqual(t, stats.TotalChunks, int64(5))
}

func TestRenderChunk(t *testing.T) {
	chunk := &RenderChunk{
		ID:        1,
		Data:      []byte("test data"),
		IsFirst:   true,
		IsLast:    false,
		Timestamp: time.Now(),
		Latency:   100,
	}

	assert.NotNil(t, chunk)
	assert.Equal(t, 1, chunk.ID)
	assert.Equal(t, []byte("test data"), chunk.Data)
	assert.True(t, chunk.IsFirst)
	assert.False(t, chunk.IsLast)
}

func TestRendererStats(t *testing.T) {
	stats := &RendererStats{
		TotalChunks:     100,
		TotalBytes:      409600,
		AvgChunkLatency: 50,
		MaxChunkLatency: 200,
		FlushCount:      100,
		Errors:          0,
	}

	assert.NotNil(t, stats)
	assert.Equal(t, int64(100), stats.TotalChunks)
	assert.Equal(t, int64(409600), stats.TotalBytes)
	assert.Equal(t, int64(50), stats.AvgChunkLatency)
	assert.Equal(t, int64(0), stats.Errors)
}

func TestRenderContext(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewChunkedRenderer(DefaultChunkedRendererConfig(), logger)

	ctx := context.Background()
	renderCtx := renderer.NewRenderContext(ctx)

	assert.NotNil(t, renderCtx)
	assert.NotNil(t, renderCtx.Context)
	assert.NotNil(t, renderCtx.Cancel)
	assert.Nil(t, renderCtx.Context.Err())

	// 取消
	renderCtx.Cancel()

	select {
	case <-renderCtx.Context.Done():
		// 预期行为
	default:
		t.Error("Context should be done after cancel")
	}
}

func TestChunkedRenderer_Stream(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewChunkedRenderer(DefaultChunkedRendererConfig(), logger)

	// 启动消费者
	go func() {
		for chunk := range renderer.Stream() {
			_ = chunk
		}
	}()

	// 创建测试内容
	ctx := context.Background()
	content := strings.NewReader("test content for streaming")
	recorder := httptest.NewRecorder()
	writer := &testFlushWriter{recorder}

	err := renderer.Render(ctx, content, writer, nil)
	assert.NoError(t, err)

	// 关闭渲染器
	renderer.Close()
}

func TestChunkedRenderer_EmptyContent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewChunkedRenderer(DefaultChunkedRendererConfig(), logger)

	ctx := context.Background()
	content := strings.NewReader("")

	recorder := httptest.NewRecorder()
	writer := &testFlushWriter{recorder}

	chunkCount := 0
	err := renderer.Render(ctx, content, writer, func(chunk *RenderChunk) {
		chunkCount++
	})

	assert.NoError(t, err)
	assert.Equal(t, 0, chunkCount)
}

func TestChunkedTransferEncoder_Empty(t *testing.T) {
	var buf bytes.Buffer
	encoder := NewChunkedTransferEncoder(&buf)

	// 编码空数据
	err := encoder.Encode([]byte{})
	assert.NoError(t, err)

	// 完成
	err = encoder.Finalize()
	assert.NoError(t, err)
	// 空数据也会产生分块：0\r\n\r\n (empty chunk) + 0\r\n\r\n (final)
	assert.Contains(t, buf.String(), "0\r\n\r\n")
}

func TestStreamingResponseWriter_LargeWrite(t *testing.T) {
	var buf bytes.Buffer
	flushCount := 0

	flushFunc := func() error {
		flushCount++
		return nil
	}

	writer := NewStreamingResponseWriter(&buf, flushFunc, 100)

	// 写入大数据
	largeData := make([]byte, 1000)
	for i := range largeData {
		largeData[i] = byte('a' + (i % 26))
	}

	n, err := writer.Write(largeData)
	assert.NoError(t, err)
	assert.Equal(t, 1000, n)

	// 应该触发多次刷新
	assert.Greater(t, flushCount, 0)

	writer.Close()
}

func TestNewChunkedRenderer_NilConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewChunkedRenderer(nil, logger)

	assert.NotNil(t, renderer)
	assert.Equal(t, 4096, renderer.config.ChunkSize)
}

func TestNewChunkedRenderer_NilLogger(t *testing.T) {
	renderer := NewChunkedRenderer(DefaultChunkedRendererConfig(), nil)

	assert.NotNil(t, renderer)
}

// slowReader 慢速读取器
type slowReader struct {
	data  []byte
	pos   int
	delay time.Duration
}

func (r *slowReader) Read(p []byte) (int, error) {
	time.Sleep(r.delay)
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// errorWriter 错误写入器
type errorWriter struct{}

func (w *errorWriter) Write(p []byte) (int, error) {
	return 0, fmt.Errorf("write error")
}

func (w *errorWriter) Flush() error {
	return fmt.Errorf("flush error")
}

// 实现 http.Flusher 接口
type testHTTPResponseWriter struct {
	*httptest.ResponseRecorder
}

func (w *testHTTPResponseWriter) Flush() {
}

func TestChunkedRenderer_WithRealHTTP(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewChunkedRenderer(DefaultChunkedRendererConfig(), logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 设置分块传输头
		w.Header().Set("Transfer-Encoding", "chunked")
		w.Header().Set("Content-Type", "text/html")

		// 刷新器
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		ctx := r.Context()
		content := strings.NewReader("<html><body><h1>Streaming Content</h1></body></html>")

		err := renderer.Render(ctx, content, &flushWriterWrapper{w, flusher}, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	// 创建测试服务器
	server := httptest.NewServer(handler)
	defer server.Close()

	// 发送请求
	resp, err := http.Get(server.URL)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/html", resp.Header.Get("Content-Type"))

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "Streaming Content")
}

type flushWriterWrapper struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func (f *flushWriterWrapper) Write(p []byte) (int, error) {
	return f.w.Write(p)
}

func (f *flushWriterWrapper) Flush() error {
	f.flusher.Flush()
	return nil
}
