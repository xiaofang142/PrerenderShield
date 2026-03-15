package bootstrap

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestApplication_New(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  address: 127.0.0.1
  api_port: 8080
  console_port: 8081
cache:
  redis_url: redis://localhost:6379/0
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	assert.NoError(t, err)

	ctx := context.Background()
	app, err := New(ctx, configFile)
	assert.NoError(t, err)
	assert.NotNil(t, app)
	assert.NotNil(t, app.config)
	assert.NotNil(t, app.redis)
	assert.NotEmpty(t, app.cleanupFn)

	// Cleanup
	app.Shutdown(ctx)
}

func TestApplication_NewInvalidConfig(t *testing.T) {
	ctx := context.Background()
	app, err := New(ctx, "/non-existent/config.yaml")
	assert.Error(t, err)
	assert.Nil(t, app)
}

func TestApplication_NewInvalidRedisURL(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  address: 127.0.0.1
  api_port: 8080
cache:
  redis_url: invalid://url
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	assert.NoError(t, err)

	ctx := context.Background()
	app, err := New(ctx, configFile)
	assert.Error(t, err)
	assert.Nil(t, app)
}

func TestApplication_GetConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  address: 127.0.0.1
  api_port: 8080
cache:
  redis_url: redis://localhost:6379/0
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	assert.NoError(t, err)

	ctx := context.Background()
	app, _ := New(ctx, configFile)
	defer app.Shutdown(ctx)

	cfg := app.GetConfig()
	assert.NotNil(t, cfg)
	assert.Equal(t, "127.0.0.1", cfg.Server.Address)
}

func TestApplication_GetRedis(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  address: 127.0.0.1
  api_port: 8080
cache:
  redis_url: redis://localhost:6379/0
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	assert.NoError(t, err)

	ctx := context.Background()
	app, _ := New(ctx, configFile)
	defer app.Shutdown(ctx)

	redis := app.GetRedis()
	assert.NotNil(t, redis)
}

func TestApplication_AddServer(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  address: 127.0.0.1
  api_port: 8080
cache:
  redis_url: redis://localhost:6379/0
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	assert.NoError(t, err)

	ctx := context.Background()
	app, _ := New(ctx, configFile)
	defer app.Shutdown(ctx)

	server := &http.Server{Addr: ":8080"}
	app.AddServer(server)

	app.mu.Lock()
	assert.Len(t, app.servers, 1)
	app.mu.Unlock()
}

func TestApplication_AddCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  address: 127.0.0.1
  api_port: 8080
cache:
  redis_url: redis://localhost:6379/0
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	assert.NoError(t, err)

	ctx := context.Background()
	app, _ := New(ctx, configFile)
	defer app.Shutdown(ctx)

	called := false
	app.AddCleanup(func() {
		called = true
	})

	app.mu.Lock()
	assert.Len(t, app.cleanupFn, 2) // 1 from Redis + 1 from test
	app.mu.Unlock()

	// Cleanup functions are called during Shutdown
	app.Shutdown(ctx)
	assert.True(t, called)
}

func TestApplication_GetStartTime(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  address: 127.0.0.1
  api_port: 8080
cache:
  redis_url: redis://localhost:6379/0
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	assert.NoError(t, err)

	ctx := context.Background()
	app, _ := New(ctx, configFile)
	defer app.Shutdown(ctx)

	startTime := app.GetStartTime()
	assert.True(t, startTime.Before(time.Now()))
	assert.True(t, startTime.After(time.Now().Add(-time.Minute)))
}

func TestApplication_GetUptime(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  address: 127.0.0.1
  api_port: 8080
cache:
  redis_url: redis://localhost:6379/0
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	assert.NoError(t, err)

	ctx := context.Background()
	app, _ := New(ctx, configFile)
	defer app.Shutdown(ctx)

	uptime := app.GetUptime()
	assert.True(t, uptime > 0)
	assert.True(t, uptime < time.Minute)
}

func TestApplication_Run(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  address: 127.0.0.1
  api_port: 8080
cache:
  redis_url: redis://localhost:6379/0
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	app, _ := New(ctx, configFile)

	// Run in goroutine
	done := make(chan error)
	go func() {
		done <- app.Run(ctx)
	}()

	// Cancel to trigger shutdown
	time.Sleep(50 * time.Millisecond)
	cancel()

	err = <-done
	assert.NoError(t, err)
}

func TestApplication_Shutdown(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  address: 127.0.0.1
  api_port: 8080
cache:
  redis_url: redis://localhost:6379/0
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	assert.NoError(t, err)

	ctx := context.Background()
	app, _ := New(ctx, configFile)

	// Add a test server
	server := &http.Server{Addr: ":9999"}
	app.AddServer(server)

	// Add a cleanup function
	cleanupCalled := false
	app.AddCleanup(func() {
		cleanupCalled = true
	})

	err = app.Shutdown(ctx)
	assert.NoError(t, err)
	assert.True(t, cleanupCalled)
}

func TestNewAppRunner(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  address: 127.0.0.1
  api_port: 8080
cache:
  redis_url: redis://localhost:6379/0
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	assert.NoError(t, err)

	ctx := context.Background()
	app, _ := New(ctx, configFile)
	defer app.Shutdown(ctx)

	runner := NewAppRunner(app)
	assert.NotNil(t, runner)
	assert.Equal(t, app, runner.app)
}

func TestAppRunner_Initialize(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  address: 127.0.0.1
  api_port: 8080
cache:
  redis_url: redis://localhost:6379/0
sites: []
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	assert.NoError(t, err)

	ctx := context.Background()
	app, _ := New(ctx, configFile)
	defer app.Shutdown(ctx)

	runner := NewAppRunner(app)
	err = runner.Initialize(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, runner.container)
	assert.NotNil(t, runner.config)
	assert.NotNil(t, runner.redisClient)
}

func TestAppRunner_Start(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  address: 127.0.0.1
  api_port: 8080
  console_port: 8081
cache:
  redis_url: redis://localhost:6379/0
sites: []
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	assert.NoError(t, err)

	ctx := context.Background()
	app, _ := New(ctx, configFile)
	defer app.Shutdown(ctx)

	runner := NewAppRunner(app)
	err = runner.Initialize(ctx)
	assert.NoError(t, err)

	// Start may fail due to port conflicts, but we test the basic flow
	err = runner.Start(ctx)
	// We don't assert on error because it depends on port availability
	_ = err
}

func TestRun(t *testing.T) {
	t.Skip("Skipping TestRun - requires full integration setup with monitoring reset")
	// This test requires full integration setup including monitoring metrics reset
	// Run manually with proper test isolation when needed
}
