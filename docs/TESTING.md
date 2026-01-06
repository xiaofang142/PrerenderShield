# 测试指南 (Testing Guide)

## 1. 测试策略

本项目采用分层测试策略：
*   **单元测试 (Unit Tests)**: 针对各个模块的核心逻辑进行测试，如 WAF 规则匹配、工具函数、配置解析等。
*   **控制器测试 (Controller Tests)**: 针对 API 接口进行测试，模拟 HTTP 请求，验证输入输出。
*   **集成测试 (Integration Tests)**: 验证多个组件协同工作的情况（通常在 CI/CD 流程中运行）。

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

## 4. 编写测试规范

*   测试文件必须以 `_test.go` 结尾。
*   测试函数名必须以 `Test` 开头，如 `TestAddSite`。
*   推荐使用 Table-Driven Tests (表格驱动测试) 模式，以覆盖更多测试用例。
*   对于外部依赖（如 Redis、文件系统），尽量使用 Interface 或 Mock 进行隔离，或在测试环境中提供真实的测试实例。
