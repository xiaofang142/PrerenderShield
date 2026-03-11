package monitoring

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"prerender-shield/internal/redis"
	"prerender-shield/internal/ssl"
)

// HealthChecker 健康检查器接口
type HealthChecker interface {
	Check() map[string]interface{}
	IsHealthy() bool
	RegisterCheck(name string, check func() (bool, string))
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// healthChecker 健康检查器实现
type healthChecker struct {
	redisClient     *redis.Client
	checks          map[string]func() (bool, string)
	lastCheckTime   time.Time
	lastCheckResult map[string]interface{}
	mutex           sync.RWMutex
	acmeClient      *ssl.ACMEClient
}

// NewHealthChecker 创建新的健康检查器
func NewHealthChecker(redisClient *redis.Client) HealthChecker {
	checker := &healthChecker{
		redisClient:     redisClient,
		checks:          make(map[string]func() (bool, string)),
		lastCheckResult: make(map[string]interface{}),
	}

	// 注册默认检查
	checker.RegisterCheck("redis", checker.checkRedis)
	checker.RegisterCheck("system", checker.checkSystem)
	checker.RegisterCheck("memory", checker.checkMemory)

	return checker
}

// SetACMEClient 设置 ACME 客户端用于 SSL 证书检查
func (h *healthChecker) SetACMEClient(client *ssl.ACMEClient) {
	h.acmeClient = client
	h.RegisterCheck("ssl", h.checkSSL)
}

// Check 执行健康检查
func (h *healthChecker) Check() map[string]interface{} {
	results := make(map[string]interface{})
	healthy := true

	for name, check := range h.checks {
		isHealthy, message := check()
		results[name] = map[string]interface{}{
			"healthy": isHealthy,
			"message": message,
		}
		if !isHealthy {
			healthy = false
		}
	}

	results["healthy"] = healthy
	results["timestamp"] = time.Now().Unix()
	results["uptime"] = time.Since(h.lastCheckTime).Seconds()

	// 添加系统资源信息
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	results["memory"] = map[string]interface{}{
		"allocated":   m.Alloc,
		"total_alloc": m.TotalAlloc,
		"sys":         m.Sys,
		"num_gc":      m.NumGC,
	}
	results["goroutines"] = runtime.NumGoroutine()

	h.mutex.Lock()
	h.lastCheckResult = results
	h.lastCheckTime = time.Now()
	h.mutex.Unlock()

	return results
}

// checkSSL 检查 SSL 证书状态
func (h *healthChecker) checkSSL() (bool, string) {
	if h.acmeClient == nil {
		return true, "SSL not configured"
	}

	// 获取即将过期的证书
	expiringCerts, err := h.acmeClient.GetExpiringCertificates(30)
	if err != nil {
		return false, fmt.Sprintf("Failed to check SSL certificates: %v", err)
	}

	if len(expiringCerts) > 0 {
		return false, fmt.Sprintf("%d certificate(s) expiring soon", len(expiringCerts))
	}

	return true, "SSL certificates are healthy"
}

// checkMemory 检查内存使用情况
func (h *healthChecker) checkMemory() (bool, string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 如果内存使用超过 1GB，认为不健康
	if m.Alloc > 1024*1024*1024 {
		return false, fmt.Sprintf("High memory usage: %d MB", m.Alloc/1024/1024)
	}

	return true, fmt.Sprintf("Memory usage: %d MB", m.Alloc/1024/1024)
}

// IsHealthy 检查系统是否健康
func (h *healthChecker) IsHealthy() bool {
	results := h.Check()
	return results["healthy"].(bool)
}

// RegisterCheck 注册健康检查
func (h *healthChecker) RegisterCheck(name string, check func() (bool, string)) {
	h.mutex.Lock()
	h.checks[name] = check
	h.mutex.Unlock()
}

// checkRedis 检查Redis健康状态
func (h *healthChecker) checkRedis() (bool, string) {
	if h.redisClient == nil {
		return false, "Redis client not initialized"
	}

	// 尝试执行PING命令
	_, err := h.redisClient.Get("ping")
	if err != nil {
		return false, fmt.Sprintf("Redis connection failed: %v", err)
	}

	return true, "Redis is healthy"
}

// checkSystem 检查系统健康状态
func (h *healthChecker) checkSystem() (bool, string) {
	// 检查系统状态
	// 这里可以添加更详细的系统检查，如CPU、内存、磁盘等
	return true, "System is healthy"
}

// ServeHTTP 处理健康检查请求
func (h *healthChecker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	results := h.Check()

	w.Header().Set("Content-Type", "application/json")
	if !results["healthy"].(bool) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	// 构建JSON响应
	response := `{
	"status": "`
	if results["healthy"].(bool) {
		response += "ok"
	} else {
		response += "error"
	}
	response += `",
	"checks": {
`

	first := true
	for name, result := range results {
		if name == "healthy" || name == "timestamp" || name == "uptime" {
			continue
		}
		if !first {
			response += `,`
		}
		first = false
		checkResult := result.(map[string]interface{})
		response += `		"` + name + `": {
			"healthy": `
		if checkResult["healthy"].(bool) {
			response += "true"
		} else {
			response += "false"
		}
		response += `,
			"message": "` + checkResult["message"].(string) + `"
		}`
	}

	response += `
	},
	"timestamp": ` + fmt.Sprintf("%d", results["timestamp"].(int64)) + `,
	"uptime": ` + fmt.Sprintf("%f", results["uptime"].(float64)) + `
}`

	w.Write([]byte(response))
}
