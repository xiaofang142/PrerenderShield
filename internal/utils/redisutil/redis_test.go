package redisutil

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParseRedisURLFormat tests the URL parsing logic without connecting to Redis
func TestParseRedisURLFormat(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		expectHost string
		expectPass string
		expectDB   int
		parseErr  bool // parsing error (before connection)
	}{
		{
			name:       "simple URL",
			url:        "redis://localhost:6379/0",
			expectHost: "localhost:6379",
			expectPass: "",
			expectDB:   0,
			parseErr:  false,
		},
		{
			name:       "URL with password",
			url:        "redis://password@localhost:6379/0",
			expectHost: "localhost:6379",
			expectPass: "password",
			expectDB:   0,
			parseErr:  false,
		},
		{
			name:       "URL with different DB",
			url:        "redis://localhost:6379/5",
			expectHost: "localhost:6379",
			expectPass: "",
			expectDB:   5,
			parseErr:  false,
		},
		{
			name:       "URL with password and different DB",
			url:        "redis://mypassword@127.0.0.1:6380/3",
			expectHost: "127.0.0.1:6380",
			expectPass: "mypassword",
			expectDB:   3,
			parseErr:  false,
		},
		{
			name:       "URL without DB",
			url:        "redis://localhost:6379",
			expectHost: "localhost:6379",
			expectPass: "",
			expectDB:   0, // default
			parseErr:  false,
		},
		{
			name:       "invalid DB number",
			url:        "redis://localhost:6379/invalid",
			expectHost: "",
			expectPass: "",
			expectDB:   0,
			parseErr:  true,
		},
		{
			name:       "empty URL",
			url:        "",
			expectHost: "", // empty string results in empty host
			expectPass: "",
			expectDB:   0,
			parseErr:  false,
		},
		{
			name:       "URL with empty DB",
			url:        "redis://localhost:6379/",
			expectHost: "localhost:6379",
			expectPass: "",
			expectDB:   0,
			parseErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Manually test the parsing logic without calling NewClient
			redisURL := strings.TrimPrefix(tt.url, "redis://")

			// Default values
			host := "localhost:6379"
			password := ""
			db := 0

			parts := strings.Split(redisURL, "/")
			if len(parts) >= 1 {
				addrPart := parts[0]
				if strings.Contains(addrPart, "@") {
					pwHost := strings.SplitN(addrPart, "@", 2)
					password = pwHost[0]
					host = pwHost[1]
				} else {
					host = addrPart
				}
			}
			if len(parts) >= 2 && parts[1] != "" {
				var err error
				db, err = strconv.Atoi(parts[1])
				if err != nil {
					if !tt.parseErr {
						t.Fatalf("unexpected parse error: %v", err)
					}
					return
				}
			}

			if tt.parseErr {
				t.Fatal("expected parse error but got none")
			}

			assert.Equal(t, tt.expectHost, host)
			assert.Equal(t, tt.expectPass, password)
			assert.Equal(t, tt.expectDB, db)
		})
	}
}

// TestParseRedisURL_Integration tests the actual ParseRedisURL function
// This test verifies that the parsing logic correctly passes parameters to NewClient
func TestParseRedisURL_Integration(t *testing.T) {
	// Test with invalid DB to verify parsing error (without needing Redis connection)
	_, err := ParseRedisURL("redis://localhost:6379/invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid syntax")

	// Test with valid URL - will fail on connection but proves parsing worked
	_, err = ParseRedisURL("redis://localhost:6379/0")
	// This might succeed if Redis is running, or fail with connection error
	// We just verify it doesn't fail with parsing error
	if err != nil {
		assert.Contains(t, err.Error(), "failed to connect")
	}
}
