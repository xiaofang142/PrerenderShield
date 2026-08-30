package analyzer

import (
	"testing"
	"time"
)

func TestNewLogAnalyzer_Defaults(t *testing.T) {
	a := NewLogAnalyzer(0)
	if a.maxEntries != 10000 {
		t.Fatalf("default maxEntries = %d, want 10000", a.maxEntries)
	}
	a2 := NewLogAnalyzer(-5)
	if a2.maxEntries != 10000 {
		t.Fatalf("negative maxEntries must default, got %d", a2.maxEntries)
	}
}

func TestAddEntry_RingBuffer(t *testing.T) {
	a := NewLogAnalyzer(3)
	for i := 0; i < 5; i++ {
		a.AddEntry(LogEntry{Timestamp: time.Now(), Level: "info", Message: "m"})
	}
	if len(a.entries) != 3 {
		t.Fatalf("ring buffer cap: len=%d want 3", len(a.entries))
	}
	// 保留最新 3 条
	if a.entries[0].Message != "m" {
		t.Fatal("entries are ring-evicted; oldest dropped")
	}
}

func TestDetectAnomalies_HighErrorRate(t *testing.T) {
	a := NewLogAnalyzer(100)
	now := time.Now()
	// 20 条日志，5 条 ERROR = 25% > 10% 阈值，且总数 >10
	for i := 0; i < 15; i++ {
		a.AddEntry(LogEntry{Timestamp: now, Level: "info", Message: "ok"})
	}
	for i := 0; i < 5; i++ {
		a.AddEntry(LogEntry{Timestamp: now, Level: "error", Message: "boom"})
	}
	anomalies := a.DetectAnomalies(time.Hour)
	found := false
	for _, an := range anomalies {
		if an.Type == "high_error_rate" && an.Severity == "high" && an.Count == 5 {
			found = true
		}
	}
	if !found {
		t.Fatalf("high_error_rate not detected: %+v", anomalies)
	}
}

func TestDetectAnomalies_RepeatedError(t *testing.T) {
	a := NewLogAnalyzer(100)
	now := time.Now()
	for i := 0; i < 12; i++ {
		a.AddEntry(LogEntry{Timestamp: now, Level: "error", Message: "same failure X"})
	}
	anomalies := a.DetectAnomalies(time.Hour)
	found := false
	for _, an := range anomalies {
		if an.Type == "repeated_error" && an.Count == 12 {
			found = true
		}
	}
	if !found {
		t.Fatalf("repeated_error not detected: %+v", anomalies)
	}
}

func TestDetectAnomalies_NoFalsePositive(t *testing.T) {
	a := NewLogAnalyzer(100)
	now := time.Now()
	// 20 条全 info，且错误数 0
	for i := 0; i < 20; i++ {
		a.AddEntry(LogEntry{Timestamp: now, Level: "info", Message: "ok"})
	}
	if anomalies := a.DetectAnomalies(time.Hour); len(anomalies) != 0 {
		t.Fatalf("no anomalies expected, got %+v", anomalies)
	}
	// 总数 <=10 时不评估错误率（样本不足）
	a2 := NewLogAnalyzer(100)
	for i := 0; i < 5; i++ {
		a2.AddEntry(LogEntry{Timestamp: now, Level: "error", Message: "x"})
	}
	if anomalies := a2.DetectAnomalies(time.Hour); len(anomalies) != 0 {
		t.Fatalf("small sample must not trigger error-rate anomaly, got %+v", anomalies)
	}
}

func TestDetectAnomalies_WindowFilter(t *testing.T) {
	a := NewLogAnalyzer(100)
	// 2 小时前的错误不计入 1 小时窗口
	a.AddEntry(LogEntry{Timestamp: time.Now().Add(-2 * time.Hour), Level: "error", Message: "old"})
	if anomalies := a.DetectAnomalies(time.Hour); len(anomalies) != 0 {
		t.Fatalf("out-of-window entries must be ignored, got %+v", anomalies)
	}
}

func TestGenerateReport(t *testing.T) {
	a := NewLogAnalyzer(100)
	now := time.Now()
	a.AddEntry(LogEntry{Timestamp: now.Add(-time.Minute), Level: "info", Source: "api", Message: "a"})
	a.AddEntry(LogEntry{Timestamp: now, Level: "ERROR", Source: "api", Message: "err one"})
	a.AddEntry(LogEntry{Timestamp: now, Level: "error", Source: "render", Message: "err one"})
	a.AddEntry(LogEntry{Timestamp: now, Level: "fatal", Source: "render", Message: "err two"})

	rep := a.GenerateReport()
	if rep.TotalEntries != 4 {
		t.Fatalf("total=%d want 4", rep.TotalEntries)
	}
	if rep.LevelCounts["ERROR"] != 2 || rep.LevelCounts["FATAL"] != 1 || rep.LevelCounts["INFO"] != 1 {
		t.Fatalf("level counts wrong: %+v (levels大小写归一)", rep.LevelCounts)
	}
	if rep.SourceCounts["api"] != 2 || rep.SourceCounts["render"] != 2 {
		t.Fatalf("source counts wrong: %+v", rep.SourceCounts)
	}
	if rep.TimeRange == "" {
		t.Fatal("time range empty")
	}
	if len(rep.TopErrors) != 2 {
		t.Fatalf("top errors=%d want 2", len(rep.TopErrors))
	}
	// 按计数降序：err one(2) 在前
	if rep.TopErrors[0].Message != "err one" || rep.TopErrors[0].Count != 2 {
		t.Fatalf("top errors not sorted: %+v", rep.TopErrors)
	}
}

func TestGetEntriesByLevel(t *testing.T) {
	a := NewLogAnalyzer(100)
	now := time.Now()
	a.AddEntry(LogEntry{Timestamp: now, Level: "error", Message: "e1"})
	a.AddEntry(LogEntry{Timestamp: now, Level: "info", Message: "i1"})
	a.AddEntry(LogEntry{Timestamp: now, Level: "ERROR", Message: "e2"})

	// 大小写不敏感 + 最新在前
	got := a.GetEntriesByLevel("error", 10)
	if len(got) != 2 || got[0].Message != "e2" {
		t.Fatalf("GetEntriesByLevel = %+v", got)
	}
	// limit 截断
	if got := a.GetEntriesByLevel("error", 1); len(got) != 1 {
		t.Fatalf("limit broken: %+v", got)
	}
	// limit<=0 默认 100
	if got := a.GetEntriesByLevel("info", 0); len(got) != 1 {
		t.Fatalf("default limit broken: %+v", got)
	}
}

func TestClear(t *testing.T) {
	a := NewLogAnalyzer(10)
	a.AddEntry(LogEntry{Timestamp: time.Now(), Level: "error", Message: "x"})
	a.Clear()
	rep := a.GenerateReport()
	if rep.TotalEntries != 0 || len(rep.LevelCounts) != 0 || len(rep.SourceCounts) != 0 {
		t.Fatalf("clear incomplete: %+v", rep)
	}
}

func TestTruncateString(t *testing.T) {
	if got := truncateString("short", 10); got != "short" {
		t.Fatalf("short string altered: %q", got)
	}
	long := string(make([]byte, 50))
	got := truncateString(long, 10)
	if len(got) != 10 || got[7:] != "..." {
		t.Fatalf("truncate format wrong: %q", got)
	}
}
