package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActionConstants(t *testing.T) {
	assert.Equal(t, Action("login"), ActionLogin)
	assert.Equal(t, Action("logout"), ActionLogout)
	assert.Equal(t, Action("config.update"), ActionConfigUpdate)
	assert.Equal(t, Action("site.create"), ActionSiteCreate)
	assert.Equal(t, Action("cert.request"), ActionCertRequest)
	assert.Equal(t, Action("preheat.trigger"), ActionPreheat)
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
}

func TestLoggerNewWithDefaults(t *testing.T) {
	logger := NewLogger(nil, DefaultConfig())
	assert.NotNil(t, logger)
}

func TestLoggerLogfNoRedis(t *testing.T) {
	logger := NewLogger(nil, DefaultConfig())
	assert.NotPanics(t, func() {
		logger.Log(Entry{
			UserID: "test",
			Action: ActionLogin,
			Status: "success",
		})
	})
}

func TestNewEvent(t *testing.T) {
	entry := Entry{
		UserID:   "admin",
		Action:   ActionLogin,
		Resource: "system",
		Detail:   "login",
		Severity: SeverityInfo,
		Status:   "success",
	}
	assert.Equal(t, "admin", entry.UserID)
	assert.Equal(t, ActionLogin, entry.Action)
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

func TestQueryOptionsEmpty(t *testing.T) {
	opts := QueryOptions{}
	assert.Empty(t, opts.UserID)
	assert.Empty(t, opts.Action)
	assert.Empty(t, opts.Resource)
}
