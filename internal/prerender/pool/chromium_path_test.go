package pool

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveChromiumPath_ExplicitValid(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "chromium")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveChromiumPath(bin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != bin {
		t.Fatalf("expected %s, got %s", bin, got)
	}
}

func TestResolveChromiumPath_ExplicitMissing(t *testing.T) {
	if _, err := ResolveChromiumPath("/nonexistent/chromium-binary-xyz"); err == nil {
		t.Fatal("expected error for missing explicit path")
	}
}

func TestResolveChromiumPath_ExplicitIsDir(t *testing.T) {
	if _, err := ResolveChromiumPath(t.TempDir()); err == nil {
		t.Fatal("expected error for directory path")
	}
}

func TestResolveChromiumPath_EmptyNoBrowser(t *testing.T) {
	// 不强制断言环境有/无浏览器，只验证函数可调用且返回一致行为
	path, err := ResolveChromiumPath("")
	if err != nil {
		if path != "" {
			t.Fatal("path should be empty on error")
		}
		return
	}
	if path == "" {
		t.Fatal("path should not be empty on success")
	}
}
