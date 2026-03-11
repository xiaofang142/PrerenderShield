package loganalyzer

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sync"
	"time"

	"go.uber.org/zap"
)

// LogEntry 日志条目
type LogEntry struct {
	ID         string                 `json:"id"`
	Timestamp  time.Time              `json:"timestamp"`
	SourceType string                 `json:"source_type"` // access, security, render, error
	Raw        string                 `json:"raw"`
	Fields     map[string]interface{} `json:"fields"`
	Level      string                 `json:"level"` // info, warn, error
	Module     string                 `json:"module"`
	Host       string                 `json:"host"`
}

// AccessLogFields 访问日志字段
type AccessLogFields struct {
	RemoteAddr   string `json:"remote_addr"`
	Method       string `json:"method"`
	URI          string `json:"uri"`
	Protocol     string `json:"protocol"`
	Status       int    `json:"status"`
	BodyBytes    int64  `json:"body_bytes"`
	Referer      string `json:"referer"`
	UserAgent    string `json:"user_agent"`
	RequestTime  float64 `json:"request_time"`
	UpstreamTime float64 `json:"upstream_time"`
	SiteID       string `json:"site_id"`
	CacheStatus  string `json:"cache_status"` // HIT, MISS, BYPASS
	IsCrawler    bool   `json:"is_crawler"`
	IsBot        bool   `json:"is_bot"`
	Country      string `json:"country"`
}

// SecurityLogFields 安全日志字段
type SecurityLogFields struct {
	RemoteAddr    string   `json:"remote_addr"`
	Method        string   `json:"method"`
	URI           string   `json:"uri"`
	ThreatType    string   `json:"threat_type"`
	ThreatLevel   string   `json:"threat_level"` // low, medium, high, critical
	RuleID        string   `json:"rule_id"`
	Action        string   `json:"action"` // blocked, logged, challenged
	MatchedData   string   `json:"matched_data"`
	UserAgent     string   `json:"user_agent"`
	Country       string   `json:"country"`
	SessionID     string   `json:"session_id"`
	UserID        string   `json:"user_id"`
	RelatedEvents []string `json:"related_events"`
}

// RenderLogFields 渲染日志字段
type RenderLogFields struct {
	URL           string  `json:"url"`
	SiteID        string  `json:"site_id"`
	RenderTime    float64 `json:"render_time"`
	WaitTime      float64 `json:"wait_time"`
	TotalTime     float64 `json:"total_time"`
	StatusCode    int     `json:"status_code"`
	ContentLength int64   `json:"content_length"`
	CacheHit      bool    `json:"cache_hit"`
	WorkerID      string  `json:"worker_id"`
	Error         string  `json:"error"`
	UserAgent     string  `json:"user_agent"`
}

// Collector 日志采集器
type Collector struct {
	config     *CollectorConfig
	sources    []LogSource
	outputChan chan *LogEntry
	logger     *zap.Logger
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex
	running    bool
}

// CollectorConfig 采集器配置
type CollectorConfig struct {
	BufferSize     int           // 缓冲区大小
	FlushInterval  time.Duration // 刷新间隔
	MaxBatchSize   int           // 最大批量大小
	SourceConfigs  []SourceConfig
	EnableMetrics  bool
}

// SourceConfig 日志源配置
type SourceConfig struct {
	Name   string `json:"name"`
	Type   string `json:"type"` // file, stdin, tcp, udp
	Path   string `json:"path"`
	Format string `json:"format"` // json, nginx, apache, custom
	Parse  string `json:"parse"`  // 自定义解析正则
}

// LogSource 日志源接口
type LogSource interface {
	Name() string
	Read(ctx context.Context, output chan<- string) error
	Close() error
}

// DefaultCollectorConfig 返回默认配置
func DefaultCollectorConfig() *CollectorConfig {
	return &CollectorConfig{
		BufferSize:    10000,
		MaxBatchSize:  1000,
		FlushInterval: 5 * time.Second,
	}
}

// NewCollector 创建日志采集器
func NewCollector(config *CollectorConfig, logger *zap.Logger) *Collector {
	if config == nil {
		config = DefaultCollectorConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())

	collector := &Collector{
		config:     config,
		outputChan: make(chan *LogEntry, config.BufferSize),
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
		sources:    make([]LogSource, 0),
	}

	return collector
}

// AddSource 添加日志源
func (c *Collector) AddSource(source LogSource) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if source == nil {
		return fmt.Errorf("日志源不能为空")
	}

	c.sources = append(c.sources, source)
	c.logger.Info("添加日志源", zap.String("name", source.Name()))

	return nil
}

// Start 启动采集器
func (c *Collector) Start() error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("采集器已在运行")
	}
	c.running = true
	c.mu.Unlock()

	// 启动所有日志源
	for _, source := range c.sources {
		c.wg.Add(1)
		go c.readSource(source)
	}

	c.logger.Info("日志采集器已启动", zap.Int("sources", len(c.sources)))
	return nil
}

// Stop 停止采集器
func (c *Collector) Stop() error {
	c.cancel()

	// 关闭所有日志源
	for _, source := range c.sources {
		if err := source.Close(); err != nil {
			c.logger.Warn("关闭日志源失败", zap.String("name", source.Name()), zap.Error(err))
		}
	}

	// 等待所有协程退出
	c.wg.Wait()

	c.mu.Lock()
	c.running = false
	c.mu.Unlock()

	c.logger.Info("日志采集器已停止")
	return nil
}

// OutputChan 返回输出通道
func (c *Collector) OutputChan() <-chan *LogEntry {
	return c.outputChan
}

// readSource 从日志源读取
func (c *Collector) readSource(source LogSource) {
	defer c.wg.Done()

	rawChan := make(chan string, 1000)
	errChan := make(chan error, 1)

	go func() {
		if err := source.Read(c.ctx, rawChan); err != nil {
			errChan <- err
		}
		close(rawChan)
	}()

	for {
		select {
		case raw, ok := <-rawChan:
			if !ok {
				return
			}
			entry := ParseLogEntry(raw, source.Name())
			if entry != nil {
				select {
				case c.outputChan <- entry:
				case <-c.ctx.Done():
					return
				}
			}

		case err := <-errChan:
			c.logger.Error("读取日志源错误", zap.String("name", source.Name()), zap.Error(err))
			return

		case <-c.ctx.Done():
			return
		}
	}
}

// FileLogSource 文件日志源
type FileLogSource struct {
	name     string
	path     string
	format   string
	parser   *regexp.Regexp
	startPos int64
}

// NewFileLogSource 创建文件日志源
func NewFileLogSource(name, path, format string) (*FileLogSource, error) {
	// 检查文件是否存在
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("文件不存在：%w", err)
	}

	source := &FileLogSource{
		name:   name,
		path:   path,
		format: format,
	}

	// 根据格式创建解析器
	switch format {
	case "nginx":
		source.parser = getNginxLogPattern()
	case "apache":
		source.parser = getApacheLogPattern()
	case "json":
		source.parser = nil // JSON 不需要正则
	default:
		source.parser = nil
	}

	return source, nil
}

// Name 返回日志源名称
func (s *FileLogSource) Name() string {
	return s.name
}

// Read 读取日志文件
func (s *FileLogSource) Read(ctx context.Context, output chan<- string) error {
	file, err := os.Open(s.path)
	if err != nil {
		return err
	}
	defer file.Close()

	// 定位到文件末尾（实时模式）或开头（历史模式）
	if s.startPos > 0 {
		if _, err := file.Seek(s.startPos, io.SeekStart); err != nil {
			return err
		}
	}

	reader := bufio.NewReader(file)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					continue
				}
				return err
			}

			if len(line) > 0 {
				select {
				case output <- line:
				case <-ctx.Done():
					return nil
				}
			}
		}
	}
}

// Close 关闭日志源
func (s *FileLogSource) Close() error {
	return nil
}

// ChannelLogSource 通道日志源（用于内部集成）
type ChannelLogSource struct {
	name   string
	input  <-chan string
	closed bool
	mu     sync.RWMutex
}

// NewChannelLogSource 创建通道日志源
func NewChannelLogSource(name string, input <-chan string) *ChannelLogSource {
	return &ChannelLogSource{
		name:  name,
		input: input,
	}
}

// Name 返回日志源名称
func (s *ChannelLogSource) Name() string {
	return s.name
}

// Read 读取日志
func (s *ChannelLogSource) Read(ctx context.Context, output chan<- string) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case line, ok := <-s.input:
			if !ok {
				return nil
			}
			select {
			case output <- line:
			case <-ctx.Done():
				return nil
			}
		}
	}
}

// Close 关闭日志源
func (s *ChannelLogSource) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// 获取 Nginx 日志正则
func getNginxLogPattern() *regexp.Regexp {
	// Nginx combined 格式
	return regexp.MustCompile(`(?P<remote_addr>\S+) - (?P<remote_user>\S+) \[(?P<time_local>[^\]]+)\] "(?P<method>\S+) (?P<uri>\S+) (?P<protocol>[^"]*)" (?P<status>\d+) (?P<body_bytes>\d+) "(?P<referer>[^"]*)" "(?P<user_agent>[^"]*)"( (?P<request_time>\S+))?`)
}

// 获取 Apache 日志正则
func getApacheLogPattern() *regexp.Regexp {
	// Apache combined 格式
	return regexp.MustCompile(`(?P<remote_addr>\S+) \S+ (?P<remote_user>\S+) \[(?P<time_local>[^\]]+)\] "(?P<method>\S+) (?P<uri>\S+) (?P<protocol>[^"]*)" (?P<status>\d+) (?P<body_bytes>\d+) "(?P<referer>[^"]*)" "(?P<user_agent>[^"]*)"`)
}

// ParseLogEntry 解析日志条目
func ParseLogEntry(raw string, sourceName string) *LogEntry {
	raw = trimNewline(raw)
	if len(raw) == 0 {
		return nil
	}

	entry := &LogEntry{
		ID:        generateLogID(),
		Timestamp: time.Now(),
		Raw:       raw,
		Fields:    make(map[string]interface{}),
		Module:    "loganalyzer",
	}

	// 尝试 JSON 解析
	if raw[0] == '{' {
		if err := json.Unmarshal([]byte(raw), &entry.Fields); err == nil {
			// 根据字段判断类型
			if _, ok := entry.Fields["threat_type"]; ok {
				entry.SourceType = "security"
			} else if _, ok := entry.Fields["render_time"]; ok {
				entry.SourceType = "render"
			} else {
				entry.SourceType = "access"
			}
			return entry
		}
	}

	// 尝试 Nginx/Apache 格式解析
	nginxParser := getNginxLogPattern()
	if matches := nginxParser.FindStringSubmatch(raw); matches != nil {
		entry.SourceType = "access"
		entry.Fields = extractNamedGroups(nginxParser, matches)
		return entry
	}

	// 未知格式，存储原始内容
	entry.SourceType = "unknown"
	entry.Fields["message"] = raw

	return entry
}

func trimNewline(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\n' {
		return s[:len(s)-1]
	}
	return s
}

func extractNamedGroups(re *regexp.Regexp, matches []string) map[string]interface{} {
	result := make(map[string]interface{})
	names := re.SubexpNames()

	for i, match := range matches {
		if i > 0 && i <= len(names) {
			name := names[i]
			if name != "" {
				result[name] = match
			}
		}
	}

	return result
}

func generateLogID() string {
	return fmt.Sprintf("log-%d", time.Now().UnixNano())
}
