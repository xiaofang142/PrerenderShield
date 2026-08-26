package analyzer

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// LogEntry 日志条目（简化版，适配多种日志格式）
type LogEntry struct {
	Timestamp time.Time
	Level     string
	Message   string
	Source    string
	Fields    map[string]interface{}
}

// LogAnalyzer 日志分析器
type LogAnalyzer struct {
	mu           sync.RWMutex
	entries      []LogEntry
	maxEntries   int
	levelCounts  map[string]int64
	sourceCounts map[string]int64
}

// NewLogAnalyzer 创建新的日志分析器
func NewLogAnalyzer(maxEntries int) *LogAnalyzer {
	if maxEntries <= 0 {
		maxEntries = 10000
	}
	return &LogAnalyzer{
		entries:      make([]LogEntry, 0, maxEntries),
		maxEntries:   maxEntries,
		levelCounts:  make(map[string]int64),
		sourceCounts: make(map[string]int64),
	}
}

// AddEntry 添加日志条目
func (a *LogAnalyzer) AddEntry(entry LogEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 超出最大数量时移除最旧的
	if len(a.entries) >= a.maxEntries {
		a.entries = a.entries[1:]
	}

	a.entries = append(a.entries, entry)
	a.levelCounts[strings.ToUpper(entry.Level)]++
	a.sourceCounts[entry.Source]++
}

// AnomalyResult 异常检测结果
type AnomalyResult struct {
	Type        string    `json:"type"`
	Severity    string    `json:"severity"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
	Count       int64     `json:"count"`
}

// DetectAnomalies 检测异常
func (a *LogAnalyzer) DetectAnomalies(window time.Duration) []AnomalyResult {
	a.mu.RLock()
	defer a.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	var anomalies []AnomalyResult

	// 1. 错误率突增检测
	errorCount := int64(0)
	totalCount := int64(0)
	for _, e := range a.entries {
		if e.Timestamp.After(cutoff) {
			totalCount++
			if strings.ToUpper(e.Level) == "ERROR" || strings.ToUpper(e.Level) == "FATAL" {
				errorCount++
			}
		}
	}

	if totalCount > 10 {
		errorRate := float64(errorCount) / float64(totalCount)
		if errorRate > 0.1 { // 错误率超过 10%
			anomalies = append(anomalies, AnomalyResult{
				Type:        "high_error_rate",
				Severity:    "high",
				Description: fmt.Sprintf("Error rate %.1f%% in the last %v (%d errors out of %d)", errorRate*100, window, errorCount, totalCount),
				Timestamp:   time.Now(),
				Count:       errorCount,
			})
		}
	}

	// 2. 重复错误检测
	errorMessages := make(map[string]int64)
	for _, e := range a.entries {
		if e.Timestamp.After(cutoff) && (strings.ToUpper(e.Level) == "ERROR" || strings.ToUpper(e.Level) == "FATAL") {
			errorMessages[e.Message]++
		}
	}

	for msg, count := range errorMessages {
		if count >= 10 { // 同一错误出现 10 次以上
			anomalies = append(anomalies, AnomalyResult{
				Type:        "repeated_error",
				Severity:    "medium",
				Description: fmt.Sprintf("Repeated error (%d times): %s", count, truncateString(msg, 100)),
				Timestamp:   time.Now(),
				Count:       count,
			})
		}
	}

	return anomalies
}

// StatsReport 统计报告
type StatsReport struct {
	TotalEntries int64            `json:"total_entries"`
	LevelCounts  map[string]int64 `json:"level_counts"`
	SourceCounts map[string]int64 `json:"source_counts"`
	TimeRange    string           `json:"time_range"`
	TopErrors    []ErrorSummary   `json:"top_errors"`
}

// ErrorSummary 错误摘要
type ErrorSummary struct {
	Message  string    `json:"message"`
	Count    int64     `json:"count"`
	LastSeen time.Time `json:"last_seen"`
}

// GenerateReport 生成统计报告
func (a *LogAnalyzer) GenerateReport() StatsReport {
	a.mu.RLock()
	defer a.mu.RUnlock()

	report := StatsReport{
		TotalEntries: int64(len(a.entries)),
		LevelCounts:  make(map[string]int64),
		SourceCounts: make(map[string]int64),
	}

	// 复制计数
	for k, v := range a.levelCounts {
		report.LevelCounts[k] = v
	}
	for k, v := range a.sourceCounts {
		report.SourceCounts[k] = v
	}

	// 时间范围
	if len(a.entries) > 0 {
		oldest := a.entries[0].Timestamp
		newest := a.entries[len(a.entries)-1].Timestamp
		report.TimeRange = fmt.Sprintf("%s ~ %s", oldest.Format(time.RFC3339), newest.Format(time.RFC3339))
	}

	// Top 错误
	errorMap := make(map[string]*ErrorSummary)
	for _, e := range a.entries {
		if strings.ToUpper(e.Level) == "ERROR" || strings.ToUpper(e.Level) == "FATAL" {
			if existing, ok := errorMap[e.Message]; ok {
				existing.Count++
				if e.Timestamp.After(existing.LastSeen) {
					existing.LastSeen = e.Timestamp
				}
			} else {
				errorMap[e.Message] = &ErrorSummary{
					Message:  truncateString(e.Message, 200),
					Count:    1,
					LastSeen: e.Timestamp,
				}
			}
		}
	}

	for _, v := range errorMap {
		report.TopErrors = append(report.TopErrors, *v)
	}

	// 按计数排序
	sort.Slice(report.TopErrors, func(i, j int) bool {
		return report.TopErrors[i].Count > report.TopErrors[j].Count
	})

	// 限制 Top 10
	if len(report.TopErrors) > 10 {
		report.TopErrors = report.TopErrors[:10]
	}

	return report
}

// GetEntriesByLevel 按级别获取日志条目
func (a *LogAnalyzer) GetEntriesByLevel(level string, limit int) []LogEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	var result []LogEntry
	for i := len(a.entries) - 1; i >= 0 && len(result) < limit; i-- {
		if strings.EqualFold(a.entries[i].Level, level) {
			result = append(result, a.entries[i])
		}
	}
	return result
}

// Clear 清除所有日志
func (a *LogAnalyzer) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = a.entries[:0]
	a.levelCounts = make(map[string]int64)
	a.sourceCounts = make(map[string]int64)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
