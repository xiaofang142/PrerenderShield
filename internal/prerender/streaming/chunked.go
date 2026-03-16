package streaming

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ChunkedRenderer 流式渲染器
type ChunkedRenderer struct {
	config  *ChunkedRendererConfig
	logger  *zap.Logger
	chunks  chan *RenderChunk
	mu      sync.RWMutex
	stats   *RendererStats
	flusher FlushWriter
	closed  bool
	closeMu sync.Mutex
}

// ChunkedRendererConfig 流式渲染配置
type ChunkedRendererConfig struct {
	ChunkSize       int           `json:"chunk_size"`        // 每个分块大小（字节）
	FlushInterval   time.Duration `json:"flush_interval"`    // 强制刷新间隔
	EnableGzip      bool          `json:"enable_gzip"`       // 启用 Gzip 压缩
	MinChunkSize    int           `json:"min_chunk_size"`    // 最小分块大小
	MaxBufferChunks int           `json:"max_buffer_chunks"` // 最大缓冲分块数
	Timeout         time.Duration `json:"timeout"`           // 渲染超时
}

// DefaultChunkedRendererConfig 返回默认配置
func DefaultChunkedRendererConfig() *ChunkedRendererConfig {
	return &ChunkedRendererConfig{
		ChunkSize:       4096, // 4KB
		FlushInterval:   100 * time.Millisecond,
		EnableGzip:      false,
		MinChunkSize:    512, // 512 字节
		MaxBufferChunks: 100,
		Timeout:         30 * time.Second,
	}
}

// RenderChunk 渲染分块
type RenderChunk struct {
	ID        int       `json:"id"`
	Data      []byte    `json:"data"`
	IsFirst   bool      `json:"is_first"`
	IsLast    bool      `json:"is_last"`
	Timestamp time.Time `json:"timestamp"`
	Latency   int64     `json:"latency_ms"` // 从开始到现在的毫秒数
}

// FlushWriter 支持刷新的写入器
type FlushWriter interface {
	io.Writer
	Flush() error
}

// RendererStats 渲染统计
type RendererStats struct {
	TotalChunks     int64 `json:"total_chunks"`
	TotalBytes      int64 `json:"total_bytes"`
	AvgChunkLatency int64 `json:"avg_chunk_latency"`
	MaxChunkLatency int64 `json:"max_chunk_latency"`
	FlushCount      int64 `json:"flush_count"`
	Errors          int64 `json:"errors"`
}

// NewChunkedRenderer 创建流式渲染器
func NewChunkedRenderer(config *ChunkedRendererConfig, logger *zap.Logger) *ChunkedRenderer {
	if config == nil {
		config = DefaultChunkedRendererConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	renderer := &ChunkedRenderer{
		config: config,
		logger: logger,
		chunks: make(chan *RenderChunk, config.MaxBufferChunks),
		stats:  &RendererStats{},
	}

	return renderer
}

// RenderContext 渲染上下文
type RenderContext struct {
	Context    context.Context
	Cancel     context.CancelFunc
	StartTime  time.Time
	ChunkCount int
	TotalBytes int
	Metadata   map[string]interface{}
}

// NewRenderContext 创建渲染上下文
func (r *ChunkedRenderer) NewRenderContext(ctx context.Context) *RenderContext {
	renderCtx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	return &RenderContext{
		Context:   renderCtx,
		Cancel:    cancel,
		StartTime: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
}

// Render 渲染并分块传输
func (r *ChunkedRenderer) Render(
	ctx context.Context,
	content io.Reader,
	writer FlushWriter,
	onChunk func(*RenderChunk),
) error {
	startTime := time.Now()
	renderCtx := r.NewRenderContext(ctx)
	defer renderCtx.Cancel()

	chunkID := 0
	buffer := make([]byte, r.config.ChunkSize)
	totalBytes := 0

	for {
		select {
		case <-renderCtx.Context.Done():
			return fmt.Errorf("渲染超时：%w", renderCtx.Context.Err())
		default:
		}

		// 读取数据
		n, err := content.Read(buffer)
		if n > 0 {
			chunkID++
			chunk := &RenderChunk{
				ID:        chunkID,
				Data:      append([]byte(nil), buffer[:n]...),
				IsFirst:   chunkID == 1,
				IsLast:    false,
				Timestamp: time.Now(),
				Latency:   time.Since(startTime).Milliseconds(),
			}

			// 写入分块
			if _, werr := writer.Write(chunk.Data); werr != nil {
				r.stats.Errors++
				return fmt.Errorf("写入分块失败：%w", werr)
			}

			// 刷新
			if ferr := writer.Flush(); ferr != nil {
				r.stats.Errors++
				return fmt.Errorf("刷新失败：%w", ferr)
			}

			r.stats.TotalChunks++
			r.stats.TotalBytes += int64(n)
			r.stats.FlushCount++
			totalBytes += n

			// 回调
			if onChunk != nil {
				onChunk(chunk)
			}

			// 发送到通道（非阻塞）
			select {
			case r.chunks <- chunk:
			default:
				// 通道已满，跳过
			}
		}

		if err == io.EOF {
			// 发送最后一个分块标记
			if chunkID > 0 {
				lastChunk := &RenderChunk{
					ID:        chunkID,
					IsLast:    true,
					Timestamp: time.Now(),
					Latency:   time.Since(startTime).Milliseconds(),
				}
				if onChunk != nil {
					onChunk(lastChunk)
				}
			}
			break
		}
		if err != nil {
			r.stats.Errors++
			return fmt.Errorf("读取内容失败：%w", err)
		}
	}

	r.logger.Debug("流式渲染完成",
		zap.Int("chunks", chunkID),
		zap.Int("total_bytes", totalBytes),
		zap.Int64("duration_ms", time.Since(startTime).Milliseconds()),
	)

	return nil
}

// RenderHTML 渲染 HTML 内容（带首屏优化）
func (r *ChunkedRenderer) RenderHTML(
	ctx context.Context,
	html string,
	writer FlushWriter,
) error {
	startTime := time.Now()

	// 立即发送 HTML 头部
	header := "<!DOCTYPE html><html><head>"
	if _, err := writer.Write([]byte(header)); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	r.stats.TotalChunks++
	r.stats.FlushCount++

	// 分块发送剩余内容
	chunkSize := r.config.ChunkSize
	for i := 0; i < len(html); i += chunkSize {
		end := i + chunkSize
		if end > len(html) {
			end = len(html)
		}

		chunk := html[i:end]
		if _, err := writer.Write([]byte(chunk)); err != nil {
			r.stats.Errors++
			return err
		}
		if err := writer.Flush(); err != nil {
			r.stats.Errors++
			return err
		}

		r.stats.TotalChunks++
		r.stats.FlushCount++
		r.stats.TotalBytes += int64(len(chunk))

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	// 发送 HTML 尾部
	footer := "</body></html>"
	if _, err := writer.Write([]byte(footer)); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	r.logger.Debug("HTML 流式渲染完成",
		zap.Int("length", len(html)),
		zap.Int64("duration_ms", time.Since(startTime).Milliseconds()),
	)

	return nil
}

// Stream 获取分块流
func (r *ChunkedRenderer) Stream() <-chan *RenderChunk {
	return r.chunks
}

// GetStats 获取统计
func (r *ChunkedRenderer) GetStats() *RendererStats {
	return r.stats
}

// Close 关闭渲染器
func (r *ChunkedRenderer) Close() {
	r.closeMu.Lock()
	defer r.closeMu.Unlock()

	if r.closed {
		return
	}
	r.closed = true
	close(r.chunks)
}

// ChunkedTransferEncoder 分块传输编码器
type ChunkedTransferEncoder struct {
	writer io.Writer
	mu     sync.Mutex
}

// NewChunkedTransferEncoder 创建分块传输编码器
func NewChunkedTransferEncoder(writer io.Writer) *ChunkedTransferEncoder {
	return &ChunkedTransferEncoder{
		writer: writer,
	}
}

// Encode 编码分块
func (e *ChunkedTransferEncoder) Encode(data []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 写入分块大小（16 进制）+ \r\n
	if _, err := fmt.Fprintf(e.writer, "%x\r\n", len(data)); err != nil {
		return err
	}

	// 写入数据
	if _, err := e.writer.Write(data); err != nil {
		return err
	}

	// 写入 \r\n
	if _, err := e.writer.Write([]byte("\r\n")); err != nil {
		return err
	}

	return nil
}

// Finalize 完成编码
func (e *ChunkedTransferEncoder) Finalize() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, err := e.writer.Write([]byte("0\r\n\r\n"))
	return err
}

// BufferPool 缓冲区池
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool 创建缓冲区池
func NewBufferPool(size int) *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, size))
			},
		},
	}
}

// Get 获取缓冲区
func (p *BufferPool) Get() *bytes.Buffer {
	return p.pool.Get().(*bytes.Buffer)
}

// Put 放回缓冲区
func (p *BufferPool) Put(buf *bytes.Buffer) {
	buf.Reset()
	p.pool.Put(buf)
}

// StreamingResponseWriter 流式响应写入器
type StreamingResponseWriter struct {
	writer      io.Writer
	flushFunc   func() error
	buffer      *bytes.Buffer
	chunkSize   int
	writtenSize int
	mu          sync.Mutex
}

// NewStreamingResponseWriter 创建流式响应写入器
func NewStreamingResponseWriter(
	writer io.Writer,
	flushFunc func() error,
	chunkSize int,
) *StreamingResponseWriter {
	if flushFunc == nil {
		flushFunc = func() error { return nil }
	}
	if chunkSize <= 0 {
		chunkSize = 4096
	}

	return &StreamingResponseWriter{
		writer:    writer,
		flushFunc: flushFunc,
		buffer:    bytes.NewBuffer(make([]byte, 0, chunkSize)),
		chunkSize: chunkSize,
	}
}

// Write 写入数据
func (w *StreamingResponseWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	n, err := w.buffer.Write(p)
	if err != nil {
		return 0, err
	}

	w.writtenSize += n

	// 如果缓冲区满了，刷新（调用内部无锁版本）
	if w.buffer.Len() >= w.chunkSize {
		if err := w.flushLocked(); err != nil {
			return n, err
		}
	}

	return n, nil
}

// Flush 刷新缓冲区
func (w *StreamingResponseWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushLocked()
}

// flushLocked 内部刷新方法（假设已持有锁）
func (w *StreamingResponseWriter) flushLocked() error {
	if w.buffer.Len() > 0 {
		if _, err := w.writer.Write(w.buffer.Bytes()); err != nil {
			return err
		}
		w.buffer.Reset()
	}

	return w.flushFunc()
}

// Close 关闭
func (w *StreamingResponseWriter) Close() error {
	return w.Flush()
}

// WrittenBytes 已写入字节数
func (w *StreamingResponseWriter) WrittenBytes() int {
	return w.writtenSize
}
