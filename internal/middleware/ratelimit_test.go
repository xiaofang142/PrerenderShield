package middleware

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseTime(t *testing.T) {
	now := time.Now()
	formatted := now.Format(time.RFC3339)
	parsed := parseTime(formatted)
	assert.WithinDuration(t, now, parsed, time.Second)
}

func TestParseTime_Invalid(t *testing.T) {
	parsed := parseTime("invalid")
	assert.True(t, parsed.IsZero())
}
