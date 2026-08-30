package services

import (
	"context"
	"testing"
	"time"

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

// mockWafRedisClientForStats 概览统计专用 WafRedisClient 模拟
type mockWafRedisClientForStats struct{}

func (m *mockWafRedisClientForStats) Context() context.Context { return context.Background() }

func (m *mockWafRedisClientForStats) Get(ctx context.Context, key string) (string, error) {
	switch key {
	case "waf:stats:global:total":
		return "120", nil
	case "waf:stats:global:blocked":
		return "7", nil
	}
	return "", nil
}

func (m *mockWafRedisClientForStats) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return nil
}

func (m *mockWafRedisClientForStats) LPush(ctx context.Context, key string, value interface{}) error {
	return nil
}

func (m *mockWafRedisClientForStats) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return nil, nil
}

func (m *mockWafRedisClientForStats) LLen(ctx context.Context, key string) (int64, error) {
	return 0, nil
}

func (m *mockWafRedisClientForStats) LTrim(ctx context.Context, key string, start, stop int64) error {
	return nil
}

func (m *mockWafRedisClientForStats) HIncrBy(ctx context.Context, key, field string, incr int64) error {
	return nil
}

func (m *mockWafRedisClientForStats) Incr(ctx context.Context, key string) error { return nil }

func (m *mockWafRedisClientForStats) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (m *mockWafRedisClientForStats) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return nil
}

// TestOverviewService_GetWafGlobalStats 验证成功路径：透传仓储统计结果
func TestOverviewService_GetWafGlobalStats(t *testing.T) {
	repo := repository.NewWafRepository(&mockWafRedisClientForStats{})
	svc := NewOverviewService(repo)

	stats, err := svc.GetWafGlobalStats("2026-08-25 00:00:00", "2026-08-26 00:00:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}
	if stats.TotalRequests != 120 || stats.BlockedRequests != 7 || stats.AttackRequests != 7 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}
