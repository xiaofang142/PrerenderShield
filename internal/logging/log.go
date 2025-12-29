package logging

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	// DEBUG 调试级别
	DEBUG LogLevel = iota
	// INFO 信息级别
	INFO
	// WARN 警告级别
	WARN
	// ERROR 错误级别
	ERROR
	// FATAL 致命级别
	FATAL
)

// Logger 日志记录器
type Logger struct {
	debugLogger *log.Logger
	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errorLogger *log.Logger
	fatalLogger *log.Logger
	auditLogger *log.Logger
	level       LogLevel
	auditEnabled bool
}

// Config 日志配置
type Config struct {
	Level       string
	Output      string
	AuditEnabled bool
	AuditOutput  string
}

// AuditLogEntry 审计日志条目
type AuditLogEntry struct {
	Timestamp    time.Time              `json:"timestamp"`
	Level        string                `json:"level"`
	EventType    string                `json:"event_type"`
	User         string                `json:"user,omitempty"`
	IP           string                `json:"ip,omitempty"`
	Action       string                `json:"action"`
	Resource     string                `json:"resource,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
	Result       string                `json:"result"`
	Message      string                `json:"message"`
}

// NewLogger 创建新的日志记录器
func NewLogger(config Config) *Logger {
	// 设置日志级别
	level := INFO
	switch config.Level {
	case "debug":
		level = DEBUG
	case "info":
		level = INFO
	case "warn":
		level = WARN
	case "error":
		level = ERROR
	case "fatal":
		level = FATAL
	}

	// 设置输出
	var output *os.File
	if config.Output == "stdout" || config.Output == "" {
		output = os.Stdout
	} else {
		var err error
		output, err = os.OpenFile(config.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Printf("Failed to open log file: %v, using stdout instead\n", err)
			output = os.Stdout
		}
	}

	// 创建基本日志记录器
	flags := log.Ldate | log.Ltime | log.Lmicroseconds
	logger := &Logger{
		debugLogger: log.New(output, "[DEBUG] ", flags),
		infoLogger:  log.New(output, "[INFO]  ", flags),
		warnLogger:  log.New(output, "[WARN]  ", flags),
		errorLogger: log.New(output, "[ERROR] ", flags),
		fatalLogger: log.New(output, "[FATAL] ", flags),
		level:       level,
		auditEnabled: config.AuditEnabled,
	}

	// 初始化审计日志记录器
	if config.AuditEnabled {
		var auditOutput *os.File
		if config.AuditOutput == "stdout" || config.AuditOutput == "" {
			auditOutput = os.Stdout
		} else {
			var err error
			auditOutput, err = os.OpenFile(config.AuditOutput, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Printf("Failed to open audit log file: %v, using stdout instead\n", err)
				auditOutput = os.Stdout
			}
		}
		logger.auditLogger = log.New(auditOutput, "", 0) // 审计日志使用JSON格式，不需要前缀和时间戳
	}

	return logger
}

// Debug 记录调试日志
func (l *Logger) Debug(format string, v ...interface{}) {
	if l.level <= DEBUG {
		l.debugLogger.Printf(format, v...)
	}
}

// Info 记录信息日志
func (l *Logger) Info(format string, v ...interface{}) {
	if l.level <= INFO {
		l.infoLogger.Printf(format, v...)
	}
}

// Warn 记录警告日志
func (l *Logger) Warn(format string, v ...interface{}) {
	if l.level <= WARN {
		l.warnLogger.Printf(format, v...)
	}
}

// Error 记录错误日志
func (l *Logger) Error(format string, v ...interface{}) {
	if l.level <= ERROR {
		l.errorLogger.Printf(format, v...)
	}
}

// Fatal 记录致命日志并退出程序
func (l *Logger) Fatal(format string, v ...interface{}) {
	if l.level <= FATAL {
		l.fatalLogger.Printf(format, v...)
		os.Exit(1)
	}
}

// Audit 记录审计日志
func (l *Logger) Audit(entry AuditLogEntry) {
	if !l.auditEnabled {
		return
	}

	// 设置默认值
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// 转换为JSON格式
	jsonData, err := json.Marshal(entry)
	if err != nil {
		l.Error("Failed to marshal audit log: %v", err)
		return
	}

	// 写入日志
	l.auditLogger.Println(string(jsonData))
}

// LogSecurityEvent 记录安全事件
func (l *Logger) LogSecurityEvent(eventType string, ip string, details map[string]interface{}, result string, message string) {
	l.Audit(AuditLogEntry{
		Level:     "SECURITY",
		EventType: eventType,
		IP:        ip,
		Action:    "security_event",
		Details:   details,
		Result:    result,
		Message:   message,
	})
}

// LogAdminAction 记录管理员操作
func (l *Logger) LogAdminAction(user string, ip string, action string, resource string, details map[string]interface{}, result string, message string) {
	l.Audit(AuditLogEntry{
		Level:     "ADMIN",
		EventType: "admin_action",
		User:      user,
		IP:        ip,
		Action:    action,
		Resource:  resource,
		Details:   details,
		Result:    result,
		Message:   message,
	})
}

// LogThreatDetection 记录威胁检测结果
func (l *Logger) LogThreatDetection(ip string, threatType string, details map[string]interface{}, result string, message string) {
	l.Audit(AuditLogEntry{
		Level:     "THREAT",
		EventType: "threat_detection",
		IP:        ip,
		Action:    "detect_threat",
		Details:   details,
		Result:    result,
		Message:   message,
	})
	l.Info("Threat detected: %s from %s, result: %s", threatType, ip, result)
}

// LogEntry 日志条目
type LogEntry struct {
	Time    time.Time
	Level   string
	Message string
	Details map[string]interface{}
}

// LoggerInterface 日志接口
type LoggerInterface interface {
	Debug(format string, v ...interface{})
	Info(format string, v ...interface{})
	Warn(format string, v ...interface{})
	Error(format string, v ...interface{})
	Fatal(format string, v ...interface{})
	Audit(entry AuditLogEntry)
	LogSecurityEvent(eventType string, ip string, details map[string]interface{}, result string, message string)
	LogAdminAction(user string, ip string, action string, resource string, details map[string]interface{}, result string, message string)
	LogThreatDetection(ip string, threatType string, details map[string]interface{}, result string, message string)
}

// DefaultLogger 默认日志记录器
var DefaultLogger *Logger

func init() {
	DefaultLogger = NewLogger(Config{
		Level:       "info",
		Output:      "stdout",
		AuditEnabled: true,
		AuditOutput:  "stdout",
	})
}
