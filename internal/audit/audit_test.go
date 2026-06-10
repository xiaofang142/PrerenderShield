package audit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestActionConstants(t *testing.T) {
	assert.Equal(t, Action("login"), ActionLogin)
	assert.Equal(t, Action("logout"), ActionLogout)
	assert.Equal(t, Action("config.update"), ActionConfigUpdate)
	assert.Equal(t, Action("cert.request"), ActionCertRequest)
}

func TestSeverityConstants(t *testing.T) {
	assert.Equal(t, Severity("info"), SeverityInfo)
	assert.Equal(t, Severity("warning"), SeverityWarning)
	assert.Equal(t, Severity("critical"), SeverityCritical)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, "audit", cfg.Prefix)
	assert.Equal(t, 90*24*time.Hour, cfg.TTL)
}

func TestNewLoggerDefaults(t *testing.T) {
	l := NewLogger(nil, Config{})
	assert.NotNil(t, l)
	assert.Equal(t, "audit", l.prefix)
	assert.Equal(t, 90*24*time.Hour, l.ttl)
}

func TestLogNoRedis(t *testing.T) {
	l := NewLogger(nil, DefaultConfig())
	err := l.Log(Entry{UserID: "u1", Action: ActionLogin, Status: "success"})
	assert.NoError(t, err)
}

func TestLogfNoRedis(t *testing.T) {
	l := NewLogger(nil, DefaultConfig())
	assert.NotPanics(t, func() {
		l.Logf("u1", ActionLogin, "system", "login", "success")
	})
}

func TestQueryNoRedis(t *testing.T) {
	l := NewLogger(nil, DefaultConfig())
	entries, err := l.Query(QueryOptions{})
	assert.NoError(t, err)
	assert.Nil(t, entries)
}

func TestMatchesFilter(t *testing.T) {
	e := Entry{UserID: "admin", Action: ActionLogin, Status: "success"}
	assert.True(t, matchesFilter(e, QueryOptions{}))
	assert.True(t, matchesFilter(e, QueryOptions{UserID: "admin"}))
	assert.False(t, matchesFilter(e, QueryOptions{UserID: "nobody"}))
	assert.True(t, matchesFilter(e, QueryOptions{Action: ActionLogin}))
	assert.False(t, matchesFilter(e, QueryOptions{Action: ActionLogout}))
	assert.True(t, matchesFilter(e, QueryOptions{Status: "success"}))
	assert.False(t, matchesFilter(e, QueryOptions{Status: "failed"}))
}

func TestMatchesFilterTime(t *testing.T) {
	now := time.Now()
	e := Entry{UserID: "u1", Timestamp: now}
	assert.True(t, matchesFilter(e, QueryOptions{Since: now.Add(-time.Hour)}))
	assert.False(t, matchesFilter(e, QueryOptions{Since: now.Add(time.Hour)}))
	assert.True(t, matchesFilter(e, QueryOptions{Until: now.Add(time.Hour)}))
	assert.False(t, matchesFilter(e, QueryOptions{Until: now.Add(-time.Hour)}))
}

func TestNilLoggerLog(t *testing.T) {
	var l *Logger
	err := l.Log(Entry{UserID: "u1"})
	assert.NoError(t, err)
}

func TestEntryStruct(t *testing.T) {
	e := Entry{
		UserID:    "admin",
		Action:    ActionConfigUpdate,
		Resource:  "system",
		Detail:    "updated log retention",
		Severity:  SeverityWarning,
		ClientIP:  "10.0.0.1",
		Status:    "success",
		Timestamp: time.Now(),
	}
	assert.Equal(t, "admin", e.UserID)
	assert.Equal(t, ActionConfigUpdate, e.Action)
	assert.Equal(t, SeverityWarning, e.Severity)
}

func TestQueryOptionsStruct(t *testing.T) {
	opts := QueryOptions{UserID: "u1", Action: ActionLogin, Limit: 10}
	assert.Equal(t, "u1", opts.UserID)
	assert.Equal(t, ActionLogin, opts.Action)
	assert.Equal(t, 10, opts.Limit)
}

func TestLoggerNewWithPrefix(t *testing.T) {
	l := NewLogger(nil, Config{Prefix: "myapp", TTL: time.Hour})
	assert.Equal(t, "myapp", l.prefix)
	assert.Equal(t, time.Hour, l.ttl)
}

func TestLogSetsDefaults(t *testing.T) {
	l := NewLogger(nil, Config{})
	err := l.Log(Entry{UserID: "u1", Action: ActionLogin})
	assert.NoError(t, err)
}
