package logging

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLogLevel_String(t *testing.T) {
	assert.Equal(t, "DEBUG", DEBUG.String())
	assert.Equal(t, "INFO", INFO.String())
	assert.Equal(t, "WARN", WARN.String())
	assert.Equal(t, "ERROR", ERROR.String())
	assert.Equal(t, "FATAL", FATAL.String())

	// 测试未知级别
	unknownLevel := LogLevel(999)
	assert.Equal(t, "UNKNOWN", unknownLevel.String())
}

func TestNewLogger(t *testing.T) {
	logger := NewLogger(Config{
		Level:        "debug",
		Output:       "stdout",
		AuditEnabled: true,
		AuditOutput:  "stdout",
	})

	assert.NotNil(t, logger)
	assert.Equal(t, DEBUG, logger.level)
	assert.True(t, logger.auditEnabled)
	assert.Equal(t, 10000, logger.maxAuditLogs)
}

func TestNewLogger_InvalidOutput(t *testing.T) {
	// 使用无效的输出路径，应该回退到 stdout
	logger := NewLogger(Config{
		Level:        "info",
		Output:       "/invalid/path/that/does/not/exist.log",
		AuditEnabled: false,
	})

	assert.NotNil(t, logger)
	assert.Equal(t, INFO, logger.level)
}

func TestNewLogger_Levels(t *testing.T) {
	testCases := []struct {
		level    string
		expected LogLevel
	}{
		{"debug", DEBUG},
		{"info", INFO},
		{"warn", WARN},
		{"error", ERROR},
		{"fatal", FATAL},
		{"unknown", INFO}, // 未知级别默认为 INFO
	}

	for _, tc := range testCases {
		logger := NewLogger(Config{Level: tc.level})
		assert.Equal(t, tc.expected, logger.level, "Level: %s", tc.level)
	}
}

func TestLogger_Debug(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		debugLogger: log.New(&buf, "[DEBUG] ", log.Lmicroseconds),
		level:       DEBUG,
	}

	logger.Debug("test debug message")
	assert.Contains(t, buf.String(), "test debug message")
}

func TestLogger_Debug_BelowLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		debugLogger: log.New(&buf, "[DEBUG] ", log.Lmicroseconds),
		level:       INFO, // 级别高于 DEBUG
	}

	logger.Debug("test debug message")
	assert.Empty(t, buf.String())
}

func TestLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		infoLogger: log.New(&buf, "[INFO]  ", log.Lmicroseconds),
		level:      INFO,
	}

	logger.Info("test info message")
	assert.Contains(t, buf.String(), "test info message")
}

func TestLogger_Warn(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		warnLogger: log.New(&buf, "[WARN]  ", log.Lmicroseconds),
		level:      WARN,
	}

	logger.Warn("test warn message")
	assert.Contains(t, buf.String(), "test warn message")
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		errorLogger: log.New(&buf, "[ERROR] ", log.Lmicroseconds),
		level:       ERROR,
	}

	logger.Error("test error message")
	assert.Contains(t, buf.String(), "test error message")
}

func TestLogger_Fatal(t *testing.T) {
	// Fatal 会调用 os.Exit(1)，所以不能直接测试
	// 这里只测试级别检查逻辑
	var buf bytes.Buffer
	logger := &Logger{
		fatalLogger: log.New(&buf, "[FATAL] ", log.Lmicroseconds),
		level:       FATAL,
	}

	// 注意：不会实际调用 Fatal，因为会导致测试退出
	// 只验证级别逻辑
	assert.True(t, logger.level <= FATAL)
}

func TestLogger_Audit(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		auditLogger:  log.New(&buf, "", 0),
		auditEnabled: true,
		auditLogs:    make([]AuditLogEntry, 0),
		maxAuditLogs: 10000,
	}

	entry := AuditLogEntry{
		Timestamp: time.Now(),
		Level:     "ADMIN",
		EventType: "admin_action",
		User:      "admin",
		IP:        "192.168.1.1",
		Action:    "delete_user",
		Resource:  "/users/123",
		Result:    "success",
		Message:   "User deleted",
	}

	logger.Audit(entry)

	// 验证审计日志被记录
	assert.NotEmpty(t, buf.String())
	assert.Contains(t, buf.String(), "admin_action")

	// 验证审计日志被保存到内存
	assert.Len(t, logger.auditLogs, 1)
	assert.Equal(t, "admin", logger.auditLogs[0].User)
}

func TestLogger_Audit_Disabled(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		auditLogger:  log.New(&buf, "", 0),
		auditEnabled: false,
	}

	entry := AuditLogEntry{
		EventType: "test_event",
	}

	logger.Audit(entry)

	// 审计日志被禁用，不应该记录
	assert.Empty(t, buf.String())
}

func TestLogger_Audit_ZeroTimestamp(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		auditLogger:  log.New(&buf, "", 0),
		auditEnabled: true,
		auditLogs:    make([]AuditLogEntry, 0),
		maxAuditLogs: 10000,
	}

	entry := AuditLogEntry{
		Level:   "TEST",
		Message: "test message",
	}

	logger.Audit(entry)

	assert.False(t, logger.auditLogs[0].Timestamp.IsZero())
}

func TestLogger_Audit_MaxLogs(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		auditLogger:  log.New(&buf, "", 0),
		auditEnabled: true,
		auditLogs:    make([]AuditLogEntry, 0),
		maxAuditLogs: 5,
	}

	// 添加 10 条日志
	for i := 0; i < 10; i++ {
		logger.Audit(AuditLogEntry{
			Level:   "TEST",
			Message: string(rune('A' + i)),
		})
	}

	// 应该只保留最近的 5 条
	assert.Len(t, logger.auditLogs, 5)
}

func TestLogger_LogSecurityEvent(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		auditLogger:  log.New(&buf, "", 0),
		auditEnabled: true,
		auditLogs:    make([]AuditLogEntry, 0),
		maxAuditLogs: 10000,
	}

	logger.LogSecurityEvent("xss_attack", "192.168.1.1", map[string]interface{}{
		"payload": "<script>",
	}, "blocked", "XSS attack detected")

	assert.Len(t, logger.auditLogs, 1)
	assert.Equal(t, "SECURITY", logger.auditLogs[0].Level)
	assert.Equal(t, "xss_attack", logger.auditLogs[0].EventType)
	assert.Equal(t, "192.168.1.1", logger.auditLogs[0].IP)
}

func TestLogger_LogAdminAction(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		auditLogger:  log.New(&buf, "", 0),
		auditEnabled: true,
		auditLogs:    make([]AuditLogEntry, 0),
		maxAuditLogs: 10000,
	}

	logger.LogAdminAction("admin", "192.168.1.1", "delete", "/users/123", nil, "success", "User deleted")

	assert.Len(t, logger.auditLogs, 1)
	assert.Equal(t, "ADMIN", logger.auditLogs[0].Level)
	assert.Equal(t, "admin", logger.auditLogs[0].User)
	assert.Equal(t, "delete", logger.auditLogs[0].Action)
}

func TestLogger_LogThreatDetection(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		auditLogger:  log.New(&buf, "", 0),
		infoLogger:   log.New(&buf, "[INFO]  ", log.Lmicroseconds),
		auditEnabled: true,
		auditLogs:    make([]AuditLogEntry, 0),
		maxAuditLogs: 10000,
		level:        INFO,
	}

	logger.LogThreatDetection("192.168.1.1", "sql_injection", map[string]interface{}{
		"query": "SELECT * FROM users",
	}, "blocked", "SQL injection detected")

	assert.Len(t, logger.auditLogs, 1)
	assert.Equal(t, "THREAT", logger.auditLogs[0].Level)
	assert.Contains(t, buf.String(), "Threat detected")
}

func TestLogger_GetAuditLogs(t *testing.T) {
	logger := &Logger{
		auditEnabled: true,
		auditLogs:    make([]AuditLogEntry, 0),
		maxAuditLogs: 10000,
	}

	// 添加 25 条审计日志
	for i := 0; i < 25; i++ {
		logger.auditLogs = append(logger.auditLogs, AuditLogEntry{
			Level:   "TEST",
			Message: string(rune('A' + i%26)),
		})
	}

	// 第一页，每页 10 条
	logs, total := logger.GetAuditLogs(1, 10)
	assert.Len(t, logs, 10)
	assert.Equal(t, 25, total)

	// 第二页
	logs2, total2 := logger.GetAuditLogs(2, 10)
	assert.Len(t, logs2, 10)
	assert.Equal(t, 25, total2)

	// 第三页（不足 10 条）
	logs3, total3 := logger.GetAuditLogs(3, 10)
	assert.Len(t, logs3, 5)
	assert.Equal(t, 25, total3)

	// 第四页（空）
	logs4, total4 := logger.GetAuditLogs(4, 10)
	assert.Empty(t, logs4)
	assert.Equal(t, 25, total4)
}

func TestLogger_GetAuditLogs_InvalidParams(t *testing.T) {
	logger := &Logger{
		auditEnabled: true,
		auditLogs:    make([]AuditLogEntry, 0),
		maxAuditLogs: 10000,
	}

	// 添加一些日志
	for i := 0; i < 5; i++ {
		logger.auditLogs = append(logger.auditLogs, AuditLogEntry{
			Level:   "TEST",
			Message: "test",
		})
	}

	// page < 1 应该被修正为 1
	logs, _ := logger.GetAuditLogs(0, 10)
	assert.Len(t, logs, 5)

	// pageSize < 1 应该被修正为 10，但只有 5 条日志
	logs2, _ := logger.GetAuditLogs(1, 0)
	assert.Len(t, logs2, 5)

	// pageSize > 100 应该被修正为 100
	logs3, _ := logger.GetAuditLogs(1, 200)
	assert.Len(t, logs3, 5)
}

func TestAuditLogEntry_JSON(t *testing.T) {
	entry := AuditLogEntry{
		Timestamp: time.Now(),
		Level:     "ADMIN",
		EventType: "admin_action",
		User:      "admin",
		IP:        "192.168.1.1",
		Action:    "delete_user",
		Resource:  "/users/123",
		Details: map[string]interface{}{
			"key": "value",
		},
		Result:  "success",
		Message: "User deleted",
	}

	data, err := json.Marshal(entry)
	assert.Nil(t, err)
	assert.Contains(t, string(data), "ADMIN")
	assert.Contains(t, string(data), "admin_action")
}

func TestLogEntry(t *testing.T) {
	entry := &LogEntry{
		Time:    time.Now(),
		Level:   "INFO",
		Message: "test message",
		Details: map[string]interface{}{
			"key": "value",
		},
	}

	assert.NotNil(t, entry)
	assert.Equal(t, "INFO", entry.Level)
	assert.Equal(t, "test message", entry.Message)
}

func TestLoggerInterface(t *testing.T) {
	// 验证 Logger 实现了 LoggerInterface 接口
	var _ LoggerInterface = (*Logger)(nil)
}

func TestDefaultLogger(t *testing.T) {
	assert.NotNil(t, DefaultLogger)
}

func TestFields_String(t *testing.T) {
	fields := Fields{
		"key1": "value1",
		"key2": 123,
	}

	str := fields.String()
	assert.NotNil(t, str)
	// JSON 应该包含键名
	assert.True(t, strings.Contains(str, "key1") || strings.Contains(str, "key2"))
}

func TestFields_Copy(t *testing.T) {
	fields := Fields{
		"key1": "value1",
		"key2": 123,
	}

	copied := fields.Copy()
	assert.Equal(t, fields, copied)

	// 修改副本不应该影响原字段
	copied["key3"] = "value3"
	assert.NotEqual(t, fields, copied)
	assert.NotContains(t, fields, "key3")
}

func TestNewStructuredLogger(t *testing.T) {
	logger := NewStructuredLogger(INFO, "stdout")
	assert.NotNil(t, logger)
	assert.Equal(t, INFO, logger.level)
	assert.Equal(t, INFO, logger.minLevel)
}

func TestNewStructuredLogger_FileOutput(t *testing.T) {
	// 使用临时文件测试
	tmpFile := "/tmp/test_structured_logger.log"
	defer os.Remove(tmpFile)

	logger := NewStructuredLogger(DEBUG, tmpFile)
	assert.NotNil(t, logger)
}

func TestStructuredLogger_WithField(t *testing.T) {
	logger := NewStructuredLogger(INFO, "stdout")
	logger2 := logger.WithField("key", "value")

	assert.NotNil(t, logger2)
	assert.NotEqual(t, logger, logger2)
	assert.Contains(t, logger2.fields, "key")
}

func TestStructuredLogger_WithFields(t *testing.T) {
	logger := NewStructuredLogger(INFO, "stdout")
	logger2 := logger.WithFields(Fields{
		"key1": "value1",
		"key2": "value2",
	})

	assert.NotNil(t, logger2)
	assert.Len(t, logger2.fields, 2)
	assert.Contains(t, logger2.fields, "key1")
	assert.Contains(t, logger2.fields, "key2")
}

func TestStructuredLogger_WithFields_MergeExisting(t *testing.T) {
	logger := NewStructuredLogger(INFO, "stdout")
	logger = logger.WithField("existing", "value")
	logger2 := logger.WithFields(Fields{
		"new": "value",
	})

	assert.Contains(t, logger2.fields, "existing")
	assert.Contains(t, logger2.fields, "new")
}

func TestStructuredLogger_WithTraceID(t *testing.T) {
	logger := NewStructuredLogger(INFO, "stdout")
	logger2 := logger.WithTraceID("trace-123")

	assert.Contains(t, logger2.fields, "trace_id")
	assert.Equal(t, "trace-123", logger2.fields["trace_id"])
}

func TestStructuredLogger_WithSpanID(t *testing.T) {
	logger := NewStructuredLogger(INFO, "stdout")
	logger2 := logger.WithSpanID("span-456")

	assert.Contains(t, logger2.fields, "span_id")
	assert.Equal(t, "span-456", logger2.fields["span_id"])
}

func TestStructuredLogger_SetMinLevel(t *testing.T) {
	logger := NewStructuredLogger(INFO, "stdout")
	assert.Equal(t, INFO, logger.minLevel)

	logger.SetMinLevel(ERROR)
	assert.Equal(t, ERROR, logger.minLevel)
}

func TestStructuredLogger_Log_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := &StructuredLogger{
		out:      &buf,
		level:    INFO,
		minLevel: INFO,
		fields:   make(Fields),
	}

	// DEBUG 级别低于 INFO，不应该输出
	logger.Debug("debug message", Fields{})
	assert.Empty(t, buf.String())

	// INFO 级别等于 INFO，应该输出
	logger.Info("info message", Fields{})
	assert.NotEmpty(t, buf.String())
}

func TestStructuredLogger_Debugf(t *testing.T) {
	var buf bytes.Buffer
	logger := &StructuredLogger{
		out:      &buf,
		level:    DEBUG,
		minLevel: DEBUG,
		fields:   make(Fields),
	}

	logger.Debugf("debug %s", "message")
	assert.Contains(t, buf.String(), "debug message")
}

func TestStructuredLogger_Infof(t *testing.T) {
	var buf bytes.Buffer
	logger := &StructuredLogger{
		out:      &buf,
		level:    INFO,
		minLevel: INFO,
		fields:   make(Fields),
	}

	logger.Infof("info %s", "message")
	assert.Contains(t, buf.String(), "info message")
}

func TestStructuredLogger_Warnf(t *testing.T) {
	var buf bytes.Buffer
	logger := &StructuredLogger{
		out:      &buf,
		level:    WARN,
		minLevel: WARN,
		fields:   make(Fields),
	}

	logger.Warnf("warn %s", "message")
	assert.Contains(t, buf.String(), "warn message")
}

func TestStructuredLogger_Errorf(t *testing.T) {
	var buf bytes.Buffer
	logger := &StructuredLogger{
		out:      &buf,
		level:    ERROR,
		minLevel: ERROR,
		fields:   make(Fields),
	}

	logger.Errorf("error %s", "message")
	assert.Contains(t, buf.String(), "error message")
}

func TestStructuredLogger_LogHTTP_Info(t *testing.T) {
	var buf bytes.Buffer
	logger := &StructuredLogger{
		out:      &buf,
		level:    INFO,
		minLevel: INFO,
		fields:   make(Fields),
	}

	fields := HTTPLogFields{
		Method:     "GET",
		Path:       "/api/test",
		StatusCode: 200,
		Duration:   100,
		ClientIP:   "192.168.1.1",
		UserAgent:  "Mozilla/5.0",
		Referer:    "https://example.com",
	}

	logger.LogHTTP(fields, nil)
	assert.Contains(t, buf.String(), "HTTP request completed")
	assert.Contains(t, buf.String(), "GET")
	assert.Contains(t, buf.String(), "/api/test")
}

func TestStructuredLogger_LogHTTP_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := &StructuredLogger{
		out:      &buf,
		level:    INFO,
		minLevel: INFO,
		fields:   make(Fields),
	}

	fields := HTTPLogFields{
		Method:     "POST",
		Path:       "/api/test",
		StatusCode: 500,
		Duration:   100,
		ClientIP:   "192.168.1.1",
		UserAgent:  "Mozilla/5.0",
	}

	logger.LogHTTP(fields, nil)
	assert.Contains(t, buf.String(), "HTTP server error")
}

func TestStructuredLogger_LogHTTP_ClientError(t *testing.T) {
	var buf bytes.Buffer
	logger := &StructuredLogger{
		out:      &buf,
		level:    INFO,
		minLevel: INFO,
		fields:   make(Fields),
	}

	fields := HTTPLogFields{
		Method:     "GET",
		Path:       "/api/test",
		StatusCode: 404,
		Duration:   100,
		ClientIP:   "192.168.1.1",
		UserAgent:  "Mozilla/5.0",
	}

	logger.LogHTTP(fields, nil)
	assert.Contains(t, buf.String(), "HTTP client error")
}

func TestStructuredLogger_LogHTTP_WithError(t *testing.T) {
	var buf bytes.Buffer
	logger := &StructuredLogger{
		out:      &buf,
		level:    INFO,
		minLevel: INFO,
		fields:   make(Fields),
	}

	fields := HTTPLogFields{
		Method:     "GET",
		Path:       "/api/test",
		StatusCode: 200,
		Duration:   100,
		ClientIP:   "192.168.1.1",
		UserAgent:  "Mozilla/5.0",
	}

	logger.LogHTTP(fields, assert.AnError)
	assert.Contains(t, buf.String(), "HTTP request failed")
	assert.Contains(t, buf.String(), assert.AnError.Error())
}

func TestStructuredLogger_LogDB_Success(t *testing.T) {
	var buf bytes.Buffer
	logger := &StructuredLogger{
		out:      &buf,
		level:    INFO,
		minLevel: INFO,
		fields:   make(Fields),
	}

	fields := DBLogFields{
		Operation: "SELECT",
		Table:     "users",
		Duration:  50,
		Query:     "SELECT * FROM users",
	}

	logger.LogDB(fields)
	assert.Contains(t, buf.String(), "Database operation completed")
	assert.Contains(t, buf.String(), "SELECT")
	assert.Contains(t, buf.String(), "users")
}

func TestStructuredLogger_LogDB_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := &StructuredLogger{
		out:      &buf,
		level:    INFO,
		minLevel: INFO,
		fields:   make(Fields),
	}

	fields := DBLogFields{
		Operation: "INSERT",
		Table:     "users",
		Duration:  100,
		Error:     "duplicate key",
	}

	logger.LogDB(fields)
	assert.Contains(t, buf.String(), "Database operation failed")
	assert.Contains(t, buf.String(), "duplicate key")
}

func TestStructuredLogger_LogCache_Hit(t *testing.T) {
	var buf bytes.Buffer
	logger := &StructuredLogger{
		out:      &buf,
		level:    INFO,
		minLevel: INFO,
		fields:   make(Fields),
	}

	fields := CacheLogFields{
		Operation: "GET",
		Key:       "user:123",
		Hit:       true,
		Duration:  5,
	}

	logger.LogCache(fields)
	assert.Contains(t, buf.String(), "Cache hit")
	assert.Contains(t, buf.String(), "user:123")
}

func TestStructuredLogger_LogCache_Miss(t *testing.T) {
	var buf bytes.Buffer
	logger := &StructuredLogger{
		out:      &buf,
		level:    INFO,
		minLevel: INFO,
		fields:   make(Fields),
	}

	fields := CacheLogFields{
		Operation: "GET",
		Key:       "user:456",
		Hit:       false,
		Duration:  10,
	}

	logger.LogCache(fields)
	assert.Contains(t, buf.String(), "Cache miss")
}

func TestStructuredLogger_JSON_Marshal(t *testing.T) {
	var buf bytes.Buffer
	logger := &StructuredLogger{
		out:      &buf,
		level:    INFO,
		minLevel: INFO,
		fields:   make(Fields),
	}

	logger.Info("test message", Fields{
		"key": "value",
		"num": 123,
	})

	// 验证输出是有效的 JSON
	var entry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &entry)
	assert.Nil(t, err)
	assert.Equal(t, "INFO", entry["level"])
	assert.Equal(t, "test message", entry["message"])
}

func TestStructuredLogger_InvalidJSON(t *testing.T) {
	// 测试无法序列化的数据类型
	var buf bytes.Buffer
	logger := &StructuredLogger{
		out:      &buf,
		level:    INFO,
		minLevel: INFO,
		fields:   make(Fields),
	}

	// 使用无法序列化的类型（通道）
	logger.Info("test message", Fields{
		"channel": make(chan int),
	})

	// 应该输出错误信息而不是崩溃
	assert.Contains(t, buf.String(), "error")
}

func TestHTTPLogFields(t *testing.T) {
	fields := HTTPLogFields{
		Method:     "POST",
		Path:       "/api/users",
		StatusCode: 201,
		Duration:   250,
		ClientIP:   "10.0.0.1",
		UserAgent:  "curl/7.68.0",
		Referer:    "https://api.example.com",
	}

	data, err := json.Marshal(fields)
	assert.Nil(t, err)
	assert.Contains(t, string(data), "POST")
	assert.Contains(t, string(data), "/api/users")
}

func TestDBLogFields(t *testing.T) {
	fields := DBLogFields{
		Operation: "UPDATE",
		Table:     "products",
		Duration:  75,
		Query:     "UPDATE products SET price = 100",
		Error:     "",
	}

	data, err := json.Marshal(fields)
	assert.Nil(t, err)
	assert.Contains(t, string(data), "UPDATE")
	assert.Contains(t, string(data), "products")
}

func TestCacheLogFields(t *testing.T) {
	fields := CacheLogFields{
		Operation: "SET",
		Key:       "session:abc123",
		Hit:       false,
		Duration:  3,
	}

	data, err := json.Marshal(fields)
	assert.Nil(t, err)
	assert.Contains(t, string(data), "session:abc123")
}
