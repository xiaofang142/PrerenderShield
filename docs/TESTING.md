# 测试指南 (Testing Guide)

## 1. 测试策略

本项目采用分层测试策略：
*   **单元测试 (Unit Tests)**: 针对各个模块的核心逻辑进行测试，如 WAF 规则匹配、工具函数、配置解析等。
*   **控制器测试 (Controller Tests)**: 针对 API 接口进行测试，模拟 HTTP 请求，验证输入输出。
*   **集成测试 (Integration Tests)**: 验证多个组件协同工作的情况（通常在 CI/CD 流程中运行）。
*   **端到端测试 (E2E Tests)**: 使用 Playwright 进行前端 UI 测试，验证完整用户流程。

## 2. 运行测试

### 2.1 运行所有测试

在项目根目录下执行：

```bash
go test ./...
```

### 2.2 查看测试覆盖率

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

这将生成一个 HTML 报告，展示代码覆盖率详情。

### 2.3 运行特定模块测试

```bash
# 运行 monitoring 包测试
go test ./internal/monitoring/... -v

# 运行单个测试文件
go test ./internal/monitoring/metrics_test.go -v

# 运行特定测试函数
go test ./internal/monitoring/... -run TestMetricsCollector
```

### 2.4 运行 Playwright 前端测试

```bash
cd web
npx playwright test
npx playwright test --reporter=html
npx playwright show-report
```

## 3. 测试模块说明

### 3.1 API 接口测试 (`internal/api/controllers`)
测试各个 Controller 的 HTTP 接口，使用 `httptest` 模拟请求。
*   `sites_controller_test.go`: 测试站点增删改查、静态文件操作。
*   `firewall_controller_test.go`: 测试 WAF 配置、GeoIP 规则。
*   `preheat_controller_test.go`: 测试预渲染任务触发。

### 3.2 核心逻辑测试
*   `internal/firewall`: 测试 SQL 注入、XSS 检测引擎的准确性。
*   `internal/config`: 测试配置文件的加载与热更新。
*   `internal/redis`: 测试 Redis 客户端的封装与数据操作。

### 3.3 监控模块测试 (`internal/monitoring`)
*   `metrics_test.go`: 测试指标收集器（计数器、仪表盘、计时器）。
*   `health_checker_test.go`: 测试健康检查器（系统、内存、Goroutine、SSL）。
*   `alerting_test.go`: 测试告警规则和通知。
*   `dashboard_test.go`: 测试仪表板数据生成。
*   `telemetry_test.go`: 测试遥测数据采集和处理。

### 3.4 安全模块测试 (`internal/security`)
*   `ratelimit/schema_test.go`: 测试 Schema 验证器、输入清理、速率限制。

### 3.5 任务队列测试 (`internal/task`)
*   `queue_test.go`: 测试任务队列的入队、出队、状态更新。

## 4. 编写测试规范

*   测试文件必须以 `_test.go` 结尾。
*   测试函数名必须以 `Test` 开头，如 `TestAddSite`。
*   推荐使用 Table-Driven Tests (表格驱动测试) 模式，以覆盖更多测试用例。
*   对于外部依赖（如 Redis、文件系统），尽量使用 Interface 或 Mock 进行隔离，或在测试环境中提供真实的测试实例。

## 5. 测试最佳实践

### 5.1 单元测试原则

1. **独立性**: 每个测试应该独立运行，不依赖其他测试的状态
2. **可重复性**: 测试结果不应该受运行顺序或环境影响
3. **快速执行**: 单元测试应该快速执行，理想情况下小于 100ms
4. **单一职责**: 每个测试只验证一个行为或场景

### 5.2 并发测试

```go
func TestMetricsCollector_Concurrent(t *testing.T) {
    collector := NewMetricsCollector(nil)

    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            collector.IncrementCounter("concurrent", 1)
        }()
    }

    wg.Wait()

    // 验证结果
    metrics := collector.GetMetrics()
    assert.Equal(t, int64(100), metrics.Counters["concurrent"])
}
```

### 5.3 HTTP Handler 测试

```go
func TestHealthChecker_ServeHTTP(t *testing.T) {
    checker := NewHealthChecker(nil)

    req := httptest.NewRequest(http.MethodGet, "/health", nil)
    rr := httptest.NewRecorder()

    checker.ServeHTTP(rr, req)

    assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
    assert.Contains(t, rr.Body.String(), `"status"`)
}
```

### 5.4 表格驱动测试

```go
func TestInjectionDetector(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected bool
    }{
        {"SQL injection", "SELECT * FROM users", true},
        {"XSS attack", "<script>alert(1)</script>", true},
        {"Normal text", "Hello World", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := detector.Detect(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

## 6. 当前测试覆盖率

| 模块 | 覆盖率 | 状态 |
|------|-------|------|
| internal/monitoring | 78.5% | ✅ 良好 |
| internal/monitoring/alerting | 89.6% | ✅ 良好 |
| internal/monitoring/dashboard | 95.9% | ✅ 优秀 |
| internal/monitoring/telemetry | 58.6% | ⚠️ 需改进 |
| internal/security/ratelimit | 92% | ✅ 优秀 |
| internal/task | 75% | ✅ 良好 |

**目标**: 所有核心模块覆盖率 >80%
