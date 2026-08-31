package monitoring

import (
	"context"
	"testing"

	"prerender-shield/internal/constants"
	"prerender-shield/internal/monitoring/alerting"
	"prerender-shield/internal/redis"
	"prerender-shield/internal/repository"
)

// TestAlertHistoryHandler_WritesHistory 回归测试（R11-BUG-4）：
// 引擎触发的告警必须落告警历史，否则 UI Alert History 恒空。
func TestAlertHistoryHandler_WritesHistory(t *testing.T) {
	client, err := redis.NewClient("localhost:6379", "", 15)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	defer client.Close()
	client.Del(constants.RedisKeyAlertHistory)

	repo := repository.NewAlertRepository(client)
	h := &alertHistoryHandler{repo: repo}

	alert := &alerting.Alert{
		ID:       "alert-test-1",
		RuleID:   "rule-1",
		RuleName: "r11-history-check",
		Severity: "warning",
		Message:  "r11-history-check: system_cpu_usage = 26.00",
		Metric:   "system_cpu_usage",
		Value:    26,
		Details:  map[string]interface{}{"threshold": 0.0},
	}
	if err := h.Send(context.Background(), alert); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if h.Name() != "history" {
		t.Fatalf("unexpected handler name: %s", h.Name())
	}

	records := repo.GetAlertHistory(10)
	if len(records) == 0 {
		t.Fatal("alert history empty after handler.Send — history chain broken")
	}
	found := false
	for _, r := range records {
		if r.Rule == "r11-history-check" && r.Status == "firing" && r.Value == 26 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected r11-history-check firing record, got %+v", records)
	}
}
