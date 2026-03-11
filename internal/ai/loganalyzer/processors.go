package loganalyzer

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// FieldNormalizerProcessor 字段标准化处理器
type FieldNormalizerProcessor struct {
	name string
}

// NewFieldNormalizerProcessor 创建字段标准化处理器
func NewFieldNormalizerProcessor() *FieldNormalizerProcessor {
	return &FieldNormalizerProcessor{
		name: "field_normalizer",
	}
}

// Name 返回处理器名称
func (p *FieldNormalizerProcessor) Name() string {
	return p.name
}

// Process 处理日志条目
func (p *FieldNormalizerProcessor) Process(ctx context.Context, entry *LogEntry) (*LogEntry, error) {
	if entry == nil {
		return nil, nil
	}

	// 标准化时间字段
	if ts, ok := entry.Fields["time_local"]; ok {
		if t, err := parseTimeLogFormat(toString(ts)); err == nil {
			entry.Timestamp = t
		}
	}

	// 标准化状态码
	if status, ok := entry.Fields["status"]; ok {
		entry.Fields["status_int"] = toInt(status)
		statusStr := toString(status)
		if len(statusStr) == 3 {
			switch statusStr[0] {
			case '2':
				entry.Fields["status_category"] = "success"
			case '3':
				entry.Fields["status_category"] = "redirect"
			case '4':
				entry.Fields["status_category"] = "client_error"
			case '5':
				entry.Fields["status_category"] = "server_error"
			}
		}
	}

	// 标准化字节数
	if bytes, ok := entry.Fields["body_bytes"]; ok {
		entry.Fields["body_bytes_int"] = toInt64(bytes)
	}

	// 标准化请求时间
	if reqTime, ok := entry.Fields["request_time"]; ok {
		entry.Fields["request_time_ms"] = toFloat64(reqTime) * 1000
	}

	// 提取 IP 信息
	if ip, ok := entry.Fields["remote_addr"]; ok {
		ipStr := toString(ip)
		entry.Fields["ip"] = ipStr
		if parsedIP := net.ParseIP(ipStr); parsedIP != nil {
			entry.Fields["ip_valid"] = true
		} else {
			entry.Fields["ip_valid"] = false
		}
	}

	// 标准化 User-Agent
	if ua, ok := entry.Fields["user_agent"]; ok {
		uaStr := toString(ua)
		entry.Fields["user_agent"] = uaStr
		entry.Fields["is_bot"] = isBot(uaStr)
		entry.Fields["is_search_engine"] = isSearchEngine(uaStr)
	}

	return entry, nil
}

// SecurityEnrichmentProcessor 安全日志增强处理器
type SecurityEnrichmentProcessor struct {
	name string
}

// NewSecurityEnrichmentProcessor 创建安全日志增强处理器
func NewSecurityEnrichmentProcessor() *SecurityEnrichmentProcessor {
	return &SecurityEnrichmentProcessor{
		name: "security_enrichment",
	}
}

// Name 返回处理器名称
func (p *SecurityEnrichmentProcessor) Name() string {
	return p.name
}

// Process 处理日志条目
func (p *SecurityEnrichmentProcessor) Process(ctx context.Context, entry *LogEntry) (*LogEntry, error) {
	if entry == nil || entry.SourceType != "security" {
		return entry, nil
	}

	// 威胁等级评分
	if threatLevel, ok := entry.Fields["threat_level"]; ok {
		level := toString(threatLevel)
		score := p.threatLevelToScore(level)
		entry.Fields["threat_score"] = score
	}

	// 提取攻击特征
	if matchedData, ok := entry.Fields["matched_data"]; ok {
		attackPatterns := p.extractAttackPatterns(toString(matchedData))
		entry.Fields["attack_patterns"] = attackPatterns
	}

	// 关联分析标记
	if sessionID, ok := entry.Fields["session_id"]; ok {
		entry.Fields["session_key"] = fmt.Sprintf("session:%s", toString(sessionID))
	}

	if userID, ok := entry.Fields["user_id"]; ok {
		entry.Fields["user_key"] = fmt.Sprintf("user:%s", toString(userID))
	}

	// IP 关联
	if ip, ok := entry.Fields["remote_addr"]; ok {
		entry.Fields["ip_key"] = fmt.Sprintf("ip:%s", toString(ip))
	}

	return entry, nil
}

func (p *SecurityEnrichmentProcessor) threatLevelToScore(level string) float64 {
	switch strings.ToLower(level) {
	case "critical":
		return 100.0
	case "high":
		return 75.0
	case "medium":
		return 50.0
	case "low":
		return 25.0
	default:
		return 0.0
	}
}

func (p *SecurityEnrichmentProcessor) extractAttackPatterns(data string) []string {
	patterns := make([]string, 0)

	// SQL 注入特征
	sqlPatterns := []string{"SELECT", "UNION", "INSERT", "UPDATE", "DELETE", "DROP", "--", "/*", "*/"}
	for _, pattern := range sqlPatterns {
		if strings.Contains(strings.ToUpper(data), pattern) {
			patterns = append(patterns, "sql_injection")
			break
		}
	}

	// XSS 特征
	xssPatterns := []string{"<script", "javascript:", "onerror=", "onload=", "onclick="}
	for _, pattern := range xssPatterns {
		if strings.Contains(strings.ToLower(data), pattern) {
			patterns = append(patterns, "xss")
			break
		}
	}

	// 路径遍历
	if strings.Contains(data, "../") || strings.Contains(data, "..\\") {
		patterns = append(patterns, "path_traversal")
	}

	// 命令注入
	cmdPatterns := []string{"|", ";", "`", "$(", "&&", "||"}
	for _, pattern := range cmdPatterns {
		if strings.Contains(data, pattern) {
			patterns = append(patterns, "command_injection")
			break
		}
	}

	return patterns
}

// RenderEnrichmentProcessor 渲染日志增强处理器
type RenderEnrichmentProcessor struct {
	name string
}

// NewRenderEnrichmentProcessor 创建渲染日志增强处理器
func NewRenderEnrichmentProcessor() *RenderEnrichmentProcessor {
	return &RenderEnrichmentProcessor{
		name: "render_enrichment",
	}
}

// Name 返回处理器名称
func (p *RenderEnrichmentProcessor) Name() string {
	return p.name
}

// Process 处理日志条目
func (p *RenderEnrichmentProcessor) Process(ctx context.Context, entry *LogEntry) (*LogEntry, error) {
	if entry == nil || entry.SourceType != "render" {
		return entry, nil
	}

	// 计算渲染性能等级
	if renderTime, ok := entry.Fields["render_time"]; ok {
		rt := toFloat64(renderTime)
		if rt < 1000 {
			entry.Fields["performance_level"] = "excellent"
		} else if rt < 3000 {
			entry.Fields["performance_level"] = "good"
		} else if rt < 5000 {
			entry.Fields["performance_level"] = "fair"
		} else {
			entry.Fields["performance_level"] = "poor"
		}
	}

	// 缓存命中标记
	if cacheHit, ok := entry.Fields["cache_hit"]; ok {
		if toBool(cacheHit) {
			entry.Fields["cache_result"] = "HIT"
		} else {
			entry.Fields["cache_result"] = "MISS"
		}
	}

	// 错误分类
	if errMsg, ok := entry.Fields["error"]; ok {
		errStr := toString(errMsg)
		if errStr != "" {
			entry.Fields["has_error"] = true
			entry.Fields["error_category"] = p.classifyError(errStr)
		}
	}

	return entry, nil
}

func (p *RenderEnrichmentProcessor) classifyError(errMsg string) string {
	errLower := strings.ToLower(errMsg)

	if strings.Contains(errLower, "timeout") {
		return "timeout"
	}
	if strings.Contains(errLower, "context") {
		return "context_cancelled"
	}
	if strings.Contains(errLower, "connection") {
		return "network"
	}
	if strings.Contains(errLower, "dns") {
		return "dns"
	}
	if strings.Contains(errLower, "ssl") || strings.Contains(errLower, "tls") {
		return "ssl"
	}
	if strings.Contains(errLower, "javascript") {
		return "javascript"
	}

	return "unknown"
}

// AnomalyDetectionProcessor 异常检测处理器
type AnomalyDetectionProcessor struct {
	name        string
	thresholds  *AnomalyThresholds
	stats       *RequestStats
	mu          sync.RWMutex
}

// AnomalyThresholds 异常阈值
type AnomalyThresholds struct {
	RPMThreshold      int     // 每分钟请求数阈值
	ErrorRateThreshold float64 // 错误率阈值
	LatencyThreshold  float64 // 延迟阈值
}

// RequestStats 请求统计
type RequestStats struct {
	mu        sync.RWMutex
	ipStats   map[string]*IPWindowStats
	siteStats map[string]*SiteWindowStats
}

// IPWindowStats IP 窗口统计
type IPWindowStats struct {
	Count       int64
	Errors      int64
	TotalLatency float64
	FirstSeen   time.Time
	LastSeen    time.Time
}

// SiteWindowStats 站点窗口统计
type SiteWindowStats struct {
	Count       int64
	Errors      int64
	TotalLatency float64
}

// NewAnomalyDetectionProcessor 创建异常检测处理器
func NewAnomalyDetectionProcessor(thresholds *AnomalyThresholds) *AnomalyDetectionProcessor {
	if thresholds == nil {
		thresholds = &AnomalyThresholds{
			RPMThreshold:      1000,
			ErrorRateThreshold: 0.1,
			LatencyThreshold:  5000,
		}
	}

	return &AnomalyDetectionProcessor{
		name:       "anomaly_detection",
		thresholds: thresholds,
		stats: &RequestStats{
			ipStats:   make(map[string]*IPWindowStats),
			siteStats: make(map[string]*SiteWindowStats),
		},
	}
}

// Name 返回处理器名称
func (p *AnomalyDetectionProcessor) Name() string {
	return p.name
}

// Process 处理日志条目
func (p *AnomalyDetectionProcessor) Process(ctx context.Context, entry *LogEntry) (*LogEntry, error) {
	if entry == nil {
		return nil, nil
	}

	// 更新统计
	p.updateStats(entry)

	// 检测异常
	anomalies := p.detectAnomalies(entry)

	if len(anomalies) > 0 {
		entry.Fields["anomalies"] = anomalies
		entry.Fields["is_anomaly"] = true
		entry.Level = "warn"
	}

	return entry, nil
}

func (p *AnomalyDetectionProcessor) updateStats(entry *LogEntry) {
	p.stats.mu.Lock()
	defer p.stats.mu.Unlock()

	now := time.Now()

	// 更新 IP 统计
	if ip, ok := entry.Fields["remote_addr"]; ok {
		ipStr := toString(ip)
		stats, exists := p.stats.ipStats[ipStr]
		if !exists {
			stats = &IPWindowStats{
				FirstSeen: now,
			}
			p.stats.ipStats[ipStr] = stats
		}
		stats.Count++
		stats.LastSeen = now

		if status, ok := entry.Fields["status_int"]; ok {
			if statusInt := toInt(status); statusInt >= 400 {
				stats.Errors++
			}
		}

		if latency, ok := entry.Fields["request_time_ms"]; ok {
			stats.TotalLatency += toFloat64(latency)
		}
	}

	// 更新站点统计
	if siteID, ok := entry.Fields["site_id"]; ok {
		siteStr := toString(siteID)
		stats, exists := p.stats.siteStats[siteStr]
		if !exists {
			stats = &SiteWindowStats{}
			p.stats.siteStats[siteStr] = stats
		}
		stats.Count++

		if status, ok := entry.Fields["status_int"]; ok {
			if statusInt := toInt(status); statusInt >= 400 {
				stats.Errors++
			}
		}
	}
}

func (p *AnomalyDetectionProcessor) detectAnomalies(entry *LogEntry) []string {
	anomalies := make([]string, 0)

	// IP 频率异常
	if ip, ok := entry.Fields["remote_addr"]; ok {
		ipStr := toString(ip)
		p.stats.mu.RLock()
		stats, exists := p.stats.ipStats[ipStr]
		p.stats.mu.RUnlock()

		if exists && stats.Count > int64(p.thresholds.RPMThreshold) {
			anomalies = append(anomalies, "high_request_rate")
		}
	}

	// 错误率异常
	if siteID, ok := entry.Fields["site_id"]; ok {
		siteStr := toString(siteID)
		p.stats.mu.RLock()
		stats, exists := p.stats.siteStats[siteStr]
		p.stats.mu.RUnlock()

		if exists && stats.Count > 0 {
			errorRate := float64(stats.Errors) / float64(stats.Count)
			if errorRate > p.thresholds.ErrorRateThreshold {
				anomalies = append(anomalies, "high_error_rate")
			}
		}
	}

	// 延迟异常
	if latency, ok := entry.Fields["request_time_ms"]; ok {
		if toFloat64(latency) > p.thresholds.LatencyThreshold {
			anomalies = append(anomalies, "high_latency")
		}
	}

	return anomalies
}

// 辅助函数

func isBot(userAgent string) bool {
	botPatterns := []string{
		"bot", "crawler", "spider", "scraper", "curl", "wget",
		"python", "go-http", "java", "php", "ruby",
	}
	uaLower := strings.ToLower(userAgent)
	for _, pattern := range botPatterns {
		if strings.Contains(uaLower, pattern) {
			return true
		}
	}
	return false
}

func isSearchEngine(userAgent string) bool {
	searchEngines := []string{
		"googlebot", "bingbot", "yandex", "baiduspider",
		"duckduckbot", "slurp", "sogou", "exabot",
	}
	uaLower := strings.ToLower(userAgent)
	for _, se := range searchEngines {
		if strings.Contains(uaLower, se) {
			return true
		}
	}
	return false
}

func parseTimeLogFormat(timeStr string) (time.Time, error) {
	// Nginx/Apache 日志格式：10/Oct/2024:13:55:36 +0800
	layout := "02/Jan/2006:15:04:05 -0700"
	return time.Parse(layout, timeStr)
}

func toBool(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return strings.ToLower(val) == "true" || val == "1"
	case int:
		return val != 0
	case int64:
		return val != 0
	case float64:
		return val != 0
	default:
		return false
	}
}
