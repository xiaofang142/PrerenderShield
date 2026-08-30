package audit

import (
	"testing"
	"time"

	"prerender-shield/internal/redis"
)

func newTestLogger(t *testing.T) *Logger {
	t.Helper()
	client, err := redis.NewClient("localhost:6379", "", 0)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	return NewLogger(client, Config{Enabled: true, TTL: 24 * time.Hour})
}

func TestAuditLogger_LogAndQuery(t *testing.T) {
	l := newTestLogger(t)
	uid := "audit-user-e2e"
	l.Logf(uid, ActionSiteUpdate, "site", "details payload", "success")

	events, err := l.Query(QueryOptions{UserID: uid, Limit: 10})
	if err != nil {
		t.Fatalf("Query err: %v", err)
	}
	found := false
	for _, e := range events {
		if e.Action == ActionSiteUpdate && e.UserID == uid {
			found = true
		}
	}
	if !found {
		t.Fatalf("logged event not found: %+v", events)
	}
}

func TestAuditLogger_LogNilRedisNoPanic(t *testing.T) {
	l := NewLogger(nil, Config{Enabled: true})
	_ = l.Log(Entry{UserID: "u", Action: ActionSiteUpdate, Resource: "res"}) // 不得 panic
}

func TestAuditLogger_QueryEmpty(t *testing.T) {
	l := newTestLogger(t)
	events, err := l.Query(QueryOptions{UserID: "audit-user-empty-e2e", Limit: 10})
	if err != nil {
		t.Fatalf("Query err: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("empty user must return no events, got %d", len(events))
	}
}
