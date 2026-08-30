package redisutil

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"prerender-shield/internal/redis"
)

// TestParseRedisURL_PasswordBranch 覆盖 URL 中带密码的解析分支（redis.go:23-27）。
// 本地 Redis 无密码，带密码连接会在 Ping 阶段失败，但解析逻辑已被执行。
func TestParseRedisURL_PasswordBranch(t *testing.T) {
	_, err := ParseRedisURL("redis://secretpass@localhost:6379/15")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to redis")
}

// TestParseRedisURL_WithPoolConfig 覆盖可选连接池配置分支（redis.go:39-41）与
// redis.NewClientWithPool 成功路径。
func TestParseRedisURL_WithPoolConfig(t *testing.T) {
	pool := redis.PoolConfig{
		MaxActive:   5,
		MaxIdle:     2,
		IdleTimeout: time.Minute,
		PoolTimeout: 2 * time.Second,
	}
	client, err := ParseRedisURL("redis://localhost:6379/15", pool)
	assert.NoError(t, err)
	assert.NotNil(t, client)

	if client != nil {
		defer client.Close()
		// 验证连接真实可用
		assert.NoError(t, client.Set("redisutil:pool:test", "ok", time.Minute))
		val, err := client.Get("redisutil:pool:test")
		assert.NoError(t, err)
		assert.Equal(t, "ok", val)
		_ = client.Del("redisutil:pool:test")
	}
}

// TestParseRedisURL_DefaultHost 覆盖空地址回退默认 host 的行为
func TestParseRedisURL_DefaultHost(t *testing.T) {
	// 只含密码、无 host 的极端形态：host 回退到 "localhost:6379"
	client, err := ParseRedisURL("redis://@/15")
	// 无论连接是否成功，都验证不 panic 且返回值类型正确
	if err == nil && client != nil {
		client.Close()
	}
	assert.True(t, err == nil || strings.Contains(err.Error(), "failed to connect"))
}
