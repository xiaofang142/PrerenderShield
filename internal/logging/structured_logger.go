package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

// String 实现 fmt.Stringer 接口
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// StructuredLogger 结构化日志记录器
type StructuredLogger struct {
	mu       sync.Mutex
	out      io.Writer
	level    LogLevel
	fields   Fields
	minLevel LogLevel
}

// Fields 字段的类型别名
type Fields map[string]interface{}

// structuredLogEntry 结构化日志条目
type structuredLogEntry struct {
	Timestamp string      `json:"timestamp"`
	Level     string      `json:"level"`
	Message   string      `json:"message"`
	Service   string      `json:"service,omitempty"`
	File      string      `json:"file,omitempty"`
	Line      int         `json:"line,omitempty"`
	TraceID   string      `json:"trace_id,omitempty"`
	SpanID    string      `json:"span_id,omitempty"`
	Details   Fields      `json:"details,omitempty"`
}

// NewStructuredLogger 创建结构化日志记录器
func NewStructuredLogger(level LogLevel, output string) *StructuredLogger {
	var out io.Writer = os.Stdout

	if output != "" && output != "stdout" {
		f, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open log file: %v, using stdout\n", err)
		} else {
			out = f
		}
	}

	return &StructuredLogger{
		out:      out,
		level:    level,
		minLevel: level,
		fields:   make(Fields),
	}
}

// WithField 添加字段
func (l *StructuredLogger) WithField(key string, value interface{}) *StructuredLogger {
	return l.WithFields(Fields{key: value})
}

// WithFields 添加多个字段
func (l *StructuredLogger) WithFields(fields Fields) *StructuredLogger {
	newFields := make(Fields)
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}
	return &StructuredLogger{
		out:      l.out,
		level:    l.level,
		minLevel: l.minLevel,
		fields:   newFields,
	}
}

// WithTraceID 添加追踪 ID
func (l *StructuredLogger) WithTraceID(traceID string) *StructuredLogger {
	return l.WithField("trace_id", traceID)
}

// WithSpanID 添加跨度 ID
func (l *StructuredLogger) WithSpanID(spanID string) *StructuredLogger {
	return l.WithField("span_id", spanID)
}

// SetMinLevel 设置最低日志级别
func (l *StructuredLogger) SetMinLevel(level LogLevel) {
	l.minLevel = level
}

// log 记录日志（内部方法）
func (l *StructuredLogger) log(level LogLevel, msg string, fields Fields) {
	if level < l.minLevel {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 获取调用位置
	_, file, line, ok := runtime.Caller(2)
	if ok {
		// 简化文件路径
		for i := len(file) - 1; i > 0; i-- {
			if file[i] == '/' {
				file = file[i-10 : i+1]
				break
			}
		}
	}

	// 合并字段
	allFields := make(Fields)
	for k, v := range l.fields {
		allFields[k] = v
	}
	for k, v := range fields {
		allFields[k] = v
	}

	entry := structuredLogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level.String(),
		Message:   msg,
		Service:   "prerender-shield",
		Details:   allFields,
	}

	if ok {
		entry.File = file
		entry.Line = line
	}

	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(l.out, "{\"error\":\"failed to marshal log entry: %v\"}", err)
		return
	}

	fmt.Fprintln(l.out, string(data))
}

// Debug 记录调试日志
func (l *StructuredLogger) Debug(msg string, fields ...Fields) {
	f := Fields{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(DEBUG, msg, f)
}

// Debugf 格式化调试日志
func (l *StructuredLogger) Debugf(format string, args ...interface{}) {
	l.Debug(fmt.Sprintf(format, args...))
}

// Info 记录信息日志
func (l *StructuredLogger) Info(msg string, fields ...Fields) {
	f := Fields{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(INFO, msg, f)
}

// Infof 格式化信息日志
func (l *StructuredLogger) Infof(format string, args ...interface{}) {
	l.Info(fmt.Sprintf(format, args...))
}

// Warn 记录警告日志
func (l *StructuredLogger) Warn(msg string, fields ...Fields) {
	f := Fields{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(WARN, msg, f)
}

// Warnf 格式化警告日志
func (l *StructuredLogger) Warnf(format string, args ...interface{}) {
	l.Warn(fmt.Sprintf(format, args...))
}

// Error 记录错误日志
func (l *StructuredLogger) Error(msg string, fields ...Fields) {
	f := Fields{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ERROR, msg, f)
}

// Errorf 格式化错误日志
func (l *StructuredLogger) Errorf(format string, args ...interface{}) {
	l.Error(fmt.Sprintf(format, args...))
}

// Fatal 记录致命日志
func (l *StructuredLogger) Fatal(msg string, fields ...Fields) {
	f := Fields{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(FATAL, msg, f)
	os.Exit(1)
}

// Fatalf 格式化致命日志
func (l *StructuredLogger) Fatalf(format string, args ...interface{}) {
	l.Fatal(fmt.Sprintf(format, args...))
}

// HTTPLogFields HTTP 请求日志字段
type HTTPLogFields struct {
	Method     string `json:"method"`
	Path       string `json:"path"`
	StatusCode int    `json:"status_code"`
	Duration   int64  `json:"duration_ms"`
	ClientIP   string `json:"client_ip"`
	UserAgent  string `json:"user_agent"`
	Referer    string `json:"referer,omitempty"`
}

// LogHTTP 记录 HTTP 请求日志
func (l *StructuredLogger) LogHTTP(fields HTTPLogFields, err error) {
	logFields := Fields{
		"method":      fields.Method,
		"path":        fields.Path,
		"status_code": fields.StatusCode,
		"duration_ms": fields.Duration,
		"client_ip":   fields.ClientIP,
		"user_agent":  fields.UserAgent,
	}

	if fields.Referer != "" {
		logFields["referer"] = fields.Referer
	}

	if err != nil {
		logFields["error"] = err.Error()
		l.Error("HTTP request failed", logFields)
	} else if fields.StatusCode >= 500 {
		l.Error("HTTP server error", logFields)
	} else if fields.StatusCode >= 400 {
		l.Warn("HTTP client error", logFields)
	} else {
		l.Info("HTTP request completed", logFields)
	}
}

// DBLogFields 数据库操作日志字段
type DBLogFields struct {
	Operation string `json:"operation"`
	Table     string `json:"table"`
	Duration  int64  `json:"duration_ms"`
	Query     string `json:"query,omitempty"`
	Error     string `json:"error,omitempty"`
}

// LogDB 记录数据库操作日志
func (l *StructuredLogger) LogDB(fields DBLogFields) {
	logFields := Fields{
		"operation":   fields.Operation,
		"table":       fields.Table,
		"duration_ms": fields.Duration,
	}

	if fields.Query != "" {
		logFields["query"] = fields.Query
	}

	if fields.Error != "" {
		logFields["error"] = fields.Error
		l.Error("Database operation failed", logFields)
	} else {
		l.Info("Database operation completed", logFields)
	}
}

// CacheLogFields 缓存操作日志字段
type CacheLogFields struct {
	Operation string `json:"operation"`
	Key       string `json:"key"`
	Hit       bool   `json:"hit"`
	Duration  int64  `json:"duration_ms"`
}

// LogCache 记录缓存操作日志
func (l *StructuredLogger) LogCache(fields CacheLogFields) {
	logFields := Fields{
		"operation":   fields.Operation,
		"key":         fields.Key,
		"hit":         fields.Hit,
		"duration_ms": fields.Duration,
	}

	if fields.Hit {
		l.Info("Cache hit", logFields)
	} else {
		l.Info("Cache miss", logFields)
	}
}

// String 实现 fmt.Stringer 接口
func (f Fields) String() string {
	data, _ := json.Marshal(f)
	return string(data)
}

// Copy 复制字段
func (f Fields) Copy() Fields {
	newFields := make(Fields)
	for k, v := range f {
		newFields[k] = v
	}
	return newFields
}
