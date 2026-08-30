package controllers

import (
	"testing"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/redis"
)

// ginNewRouter 统一测试路由构造（TestMode）
func ginNewRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// newTestRedisDB15 获取 DB15 测试客户端（绝不触碰 DB0），不可用则跳过
func newTestRedisDB15(t *testing.T) *redis.Client {
	t.Helper()
	client, err := redis.NewClient("localhost:6379", "", 15)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// closedTestRedisDB15 返回已关闭的 Redis 客户端，用于触发 Redis 错误分支
func closedTestRedisDB15(t *testing.T) *redis.Client {
	t.Helper()
	client := newTestRedisDB15(t)
	_ = client.Close()
	return client
}
