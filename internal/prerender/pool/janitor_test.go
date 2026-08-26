package pool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

// nopJanitorLogger 静默日志实现
type nopJanitorLogger struct{}

func (nopJanitorLogger) Info(format string, v ...interface{})  {}
func (nopJanitorLogger) Warn(format string, v ...interface{})  {}
func (nopJanitorLogger) Error(format string, v ...interface{}) {}

func TestListChromedpProcesses_NoFalsePositives(t *testing.T) {
	// 正常环境下可能为 0（无残留）也可能有历史遗留，但绝不应把
	// 自身进程/桌面 Chrome 计入——桌面 Chrome 无 chromedp-runner 标记
	pids, err := listChromedpProcesses()
	if err != nil {
		t.Skipf("ps unavailable: %v", err)
	}
	for _, pid := range pids {
		if pid == os.Getpid() {
			t.Fatal("sweep list should never contain self")
		}
	}
}

func TestSweepOrphans_RemovesStaleTempDirs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)

	// 陈旧目录（mtime 2 小时前）
	stale := filepath.Join(dir, chromedpMarker+"-stale123")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stale, past, past); err != nil {
		t.Fatal(err)
	}
	// 新鲜目录（本运行可能仍在用）
	fresh := filepath.Join(dir, chromedpMarker+"-fresh456")
	if err := os.MkdirAll(fresh, 0o755); err != nil {
		t.Fatal(err)
	}
	// 非 chromedp 目录不受影响
	other := filepath.Join(dir, "other-app-dir")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}

	killed, removed := SweepOrphans(nopJanitorLogger{})

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("stale chromedp dir should be removed")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatal("fresh chromedp dir must be preserved")
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatal("non-chromedp dir must be preserved")
	}
	_ = killed
	_ = removed
}

func TestHardProcessCap(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxInstances = 10
	p := NewPool(cfg, zap.NewNop())
	defer p.Close()
	cap := p.HardProcessCap()
	if cap <= cfg.MaxInstances {
		t.Fatalf("hard cap (%d) must exceed MaxInstances (%d)", cap, cfg.MaxInstances)
	}
}

func TestChromedpMarkerNotSubstringOfCommonNames(t *testing.T) {
	// 确保标记不会误匹配用户桌面 Chrome 的常规参数
	for _, arg := range []string{"--type=renderer", "Google Chrome", "chrome_crashpad"} {
		if strings.Contains(arg, chromedpMarker) {
			t.Fatalf("marker would false-positive on %q", arg)
		}
	}
}
