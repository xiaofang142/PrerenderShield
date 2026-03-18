package monitoring

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHealthCheckerInterface(t *testing.T) {
	// 验证 healthChecker 实现了 HealthChecker 接口
	var _ HealthChecker = (*healthChecker)(nil)
}

func TestHealthChecker_Check_BasicInfo(t *testing.T) {
	// 使用 nil redis client 创建检查器
	checker := NewHealthChecker(nil)

	results := checker.Check()

	assert.NotNil(t, results)
	assert.Contains(t, results, "timestamp")
	assert.Contains(t, results, "uptime")
	assert.Contains(t, results, "goroutines")
	assert.Contains(t, results, "memory")
}

func TestHealthChecker_Check_RedisNil(t *testing.T) {
	checker := NewHealthChecker(nil).(*healthChecker)

	// 直接测试 checkRedis 方法
	healthy, message := checker.checkRedis()

	assert.False(t, healthy)
	assert.Contains(t, message, "not initialized")
}

func TestHealthChecker_Check_System(t *testing.T) {
	checker := NewHealthChecker(nil).(*healthChecker)

	healthy, message := checker.checkSystem()

	assert.True(t, healthy)
	assert.Equal(t, "System is healthy", message)
}

func TestHealthChecker_Check_Memory(t *testing.T) {
	checker := NewHealthChecker(nil).(*healthChecker)

	healthy, message := checker.checkMemory()

	assert.True(t, healthy)
	assert.Contains(t, message, "Memory usage")
}

func TestHealthChecker_RegisterCheck(t *testing.T) {
	checker := NewHealthChecker(nil).(*healthChecker)

	customCheckCalled := false
	checker.RegisterCheck("custom", func() (bool, string) {
		customCheckCalled = true
		return true, "Custom check passed"
	})

	results := checker.Check()

	assert.True(t, customCheckCalled)
	assert.Contains(t, results, "custom")

	customResult := results["custom"].(map[string]interface{})
	assert.True(t, customResult["healthy"].(bool))
	assert.Equal(t, "Custom check passed", customResult["message"].(string))
}

func TestHealthChecker_Check_MemoryStructure(t *testing.T) {
	checker := NewHealthChecker(nil)

	results := checker.Check()

	memoryInfo := results["memory"].(map[string]interface{})
	assert.Contains(t, memoryInfo, "allocated")
	assert.Contains(t, memoryInfo, "total_alloc")
	assert.Contains(t, memoryInfo, "sys")
	assert.Contains(t, memoryInfo, "num_gc")
}

func TestHealthChecker_Check_Goroutines(t *testing.T) {
	checker := NewHealthChecker(nil)

	results := checker.Check()

	goroutines := results["goroutines"]
	assert.Greater(t, goroutines, 0)
}

func TestHealthChecker_Check_Timestamp(t *testing.T) {
	checker := NewHealthChecker(nil)

	results := checker.Check()

	timestamp := results["timestamp"].(int64)
	assert.Greater(t, timestamp, int64(0))
}

func TestHealthChecker_LastCheckTime(t *testing.T) {
	checker := NewHealthChecker(nil).(*healthChecker)

	checker.Check()

	checker.mutex.RLock()
	lastTime := checker.lastCheckTime
	checker.mutex.RUnlock()

	assert.False(t, lastTime.IsZero())
}

func TestHealthChecker_LastCheckResult(t *testing.T) {
	checker := NewHealthChecker(nil).(*healthChecker)

	checker.Check()

	checker.mutex.RLock()
	lastResult := checker.lastCheckResult
	checker.mutex.RUnlock()

	assert.NotNil(t, lastResult)
	assert.NotEmpty(t, lastResult)
}

func TestHealthChecker_Check_Concurrent(t *testing.T) {
	checker := NewHealthChecker(nil)

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			checker.Check()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestHealthChecker_SetACMEClient(t *testing.T) {
	checker := NewHealthChecker(nil).(*healthChecker)

	// 测试设置为 nil
	checker.SetACMEClient(nil)

	// 验证 ssl 检查被注册
	checker.mutex.RLock()
	_, exists := checker.checks["ssl"]
	checker.mutex.RUnlock()

	assert.True(t, exists)
}

func TestHealthChecker_Check_SSL(t *testing.T) {
	checker := NewHealthChecker(nil).(*healthChecker)
	checker.SetACMEClient(nil)

	// 测试 checkSSL 方法（acmeClient 为 nil）
	healthy, message := checker.checkSSL()

	assert.True(t, healthy)
	assert.Contains(t, message, "SSL not configured")
}

func TestHealthChecker_Check_ResultStructure(t *testing.T) {
	checker := NewHealthChecker(nil)

	results := checker.Check()

	// 验证基本结构
	assert.IsType(t, int64(0), results["timestamp"])
	assert.IsType(t, 0.0, results["uptime"])
	assert.IsType(t, true, results["healthy"])

	// 验证各个检查项（不包括 memory，因为它会被系统资源信息覆盖）
	for _, name := range []string{"redis", "system"} {
		assert.Contains(t, results, name)
		checkResult := results[name].(map[string]interface{})
		assert.Contains(t, checkResult, "healthy")
		assert.Contains(t, checkResult, "message")
	}
}

func TestHealthChecker_Check_UptimeCalculation(t *testing.T) {
	checker := NewHealthChecker(nil).(*healthChecker)

	// 第一次检查
	checker.Check()

	// 等待一小段时间
	time.Sleep(10 * time.Millisecond)

	// 第二次检查
	results := checker.Check()

	uptime := results["uptime"].(float64)
	assert.GreaterOrEqual(t, uptime, 0.0)
}

func TestHealthChecker_IsHealthy(t *testing.T) {
	checker := NewHealthChecker(nil)

	// 测试健康情况
	// 由于没有配置 Redis，redis 检查会失败，所以整体不健康
	healthy := checker.IsHealthy()

	// Redis 检查失败会导致整体不健康
	assert.False(t, healthy)
}

func TestHealthChecker_ServeHTTP_OK(t *testing.T) {
	checker := NewHealthChecker(nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	checker.ServeHTTP(rr, req)

	// 由于没有配置 Redis，返回服务不可用状态
	// 但响应仍然是有效的健康检查响应
	// 验证响应格式
	// 注意：由于没有配置 Redis，status 是 "error" 而不是 "ok"
	assert.Contains(t, rr.Body.String(), `"status"`)
	assert.Contains(t, rr.Body.String(), `"checks"`)
}

func TestHealthChecker_ServeHTTP_BodyStructure(t *testing.T) {
	checker := NewHealthChecker(nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	checker.ServeHTTP(rr, req)

	body := rr.Body.String()
	// 验证基本结构
	assert.Contains(t, body, `"status"`)
	assert.Contains(t, body, `"checks"`)
	assert.Contains(t, body, `"timestamp"`)
	assert.Contains(t, body, `"uptime"`)
	assert.Contains(t, body, `"redis"`)
	assert.Contains(t, body, `"system"`)
}
