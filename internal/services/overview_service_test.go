package services

import (
	"testing"

	"prerender-shield/internal/repository"
)

// TestOverviewService_NilRepository 验证 nil 仓储防护：Controller 层
// 以 nil WafRepository 构造服务时不应 panic，而是返回错误
func TestOverviewService_NilRepository(t *testing.T) {
	svc := NewOverviewService(nil)
	stats, err := svc.GetWafGlobalStats("2026-08-25 00:00:00", "2026-08-26 00:00:00")
	if err == nil {
		t.Fatal("expected error when repository is nil")
	}
	if stats != nil {
		t.Fatalf("expected nil stats, got %+v", stats)
	}
}

// TestOverviewService_NilReceiver 验证零值/nil 接收者安全
func TestOverviewService_NilReceiver(t *testing.T) {
	var svc *OverviewService
	_, err := svc.GetWafGlobalStats("2026-08-25 00:00:00", "2026-08-26 00:00:00")
	if err == nil {
		t.Fatal("expected error on nil receiver")
	}
}

// TestNewOverviewService_WrapsRepository 验证构造器正确持有仓储引用
func TestNewOverviewService_WrapsRepository(t *testing.T) {
	repo := &repository.WafRepository{}
	svc := NewOverviewService(repo)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.wafRepo != repo {
		t.Fatal("service should hold the given repository reference")
	}
}
