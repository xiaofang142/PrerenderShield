# Phase 1 迁移指南

## 概述

Phase 1 完成了架构优化的基础工作，主要包括：
1. 合并重复安全模块
2. AI 模块重构
3. 核心接口定义
4. main.go 重构

## 主要变更

### 1. 目录结构变更

#### 新增目录
```
internal/
├── bootstrap/              # 新的应用启动和生命周期管理
│   ├── application.go      # 应用容器
│   └── runner.go           # 模块初始化和服务启动
├── security/waf/           # 统一的 WAF 安全模块
│   ├── engine.go           # WAF 引擎实现
│   ├── types/              # 类型定义
│   │   ├── threat.go       # 威胁类型
│   │   └── action.go       # 动作类型
│   ├── rules.go            # 规则管理
│   ├── action.go           # 动作处理
│   └── detectors/          # 检测器
│       ├── injection.go    # SQL 注入检测
│       ├── xss.go          # XSS 检测
│       ├── csrf.go         # CSRF 检测
│       ├── sensitive.go    # 敏感数据检测
│       ├── common.go       # 通用检测器
│       └── advanced/       # 高级检测器（AI 驱动）
│           ├── baseline.go
│           └── ueba.go
├── interfaces.go           # 核心接口定义
└── logging/analyzer/       # 日志分析模块（从 ai/loganalyzer 迁移）
    ├── aggregation.go
    ├── collector.go
    ├── processors.go
    └── ...

pkg/
├── errors/                 # 统一错误处理
│   └── errors.go
├── event/                  # 事件总线
│   └── bus.go
└── observability/          # 可观测性
    ├── logging/
    │   └── logger.go
    └── metrics/
        └── recorder.go
```

#### 删除目录
```
internal/ai/                # 已拆分迁移
├── loganalyzer/            → internal/logging/analyzer/
└── behavioranalyzer/       → internal/security/waf/detectors/advanced/
```

### 2. 包导入路径变更

| 原路径 | 新路径 |
|--------|--------|
| `internal/ai/loganalyzer` | `internal/logging/analyzer` |
| `internal/ai/behavioranalyzer` | `internal/security/waf/detectors/advanced` |
| - | `pkg/errors` (新增) |
| - | `pkg/event` (新增) |
| - | `pkg/observability/logging` (新增) |
| - | `pkg/observability/metrics` (新增) |

### 3. main.go 简化

**之前**: 490 行，包含所有初始化和启动逻辑
**之后**: 30 行，仅包含信号处理和调用 bootstrap

```go
// 之前 (main.go - 490 行)
func main() {
    // 加载配置
    // 初始化 Redis
    // 初始化认证模块
    // 初始化防火墙
    // 初始化缓存
    // 初始化渲染引擎
    // ... 大量初始化代码
    // 启动所有服务
    // 等待信号
    // 关闭服务
}

// 之后 (main.go - 30 行)
func main() {
    var configPath string
    flag.StringVar(&configPath, "config", "", "Path to config")
    flag.Parse()

    ctx, cancel := signal.NotifyContext(context.Background(),
        syscall.SIGINT, syscall.SIGTERM)
    defer cancel()

    if err := bootstrap.Run(ctx, configPath); err != nil {
        log.Fatalf("Application error: %v", err)
    }
}
```

### 4. Bootstrap 框架

新增 `internal/bootstrap/` 包，负责：
- 应用容器管理 (`Application`)
- 模块初始化 (`AppRunner.Initialize`)
- 服务启动 (`AppRunner.Start`)
- 优雅关闭 (`Application.Shutdown`)

使用示例：
```go
// 一站式运行
if err := bootstrap.Run(ctx, configPath); err != nil {
    log.Fatalf("Application error: %v", err)
}

// 或者分步控制
app, err := bootstrap.New(ctx, configPath)
runner := bootstrap.NewAppRunner(app)
if err := runner.Initialize(ctx); err != nil {
    return err
}
if err := runner.Start(ctx); err != nil {
    return err
}
return app.Run(ctx)
```

### 5. 统一错误处理

新增 `pkg/errors/errors.go`：
```go
// 使用示例
import "prerender-shield/pkg/errors"

// 创建错误
err := errors.New(errors.ErrInternal, "something went wrong")

// 包装错误
err := errors.Wrap(someErr, errors.ErrRenderTimeout, "rendering failed")

// 添加上下文
err := errors.New(errors.ErrWAFBlocked, "blocked by firewall").
    WithContext("site_id", siteID).
    WithContext("threat_type", "sql_injection")
```

### 6. 事件总线系统

新增 `pkg/event/bus.go`：
```go
import "prerender-shield/pkg/event"

// 创建事件总线
bus := event.NewInMemoryBus()

// 订阅事件
sub, err := bus.Subscribe(ctx, event.TopicSiteCreated,
    func(ctx context.Context, e event.Event) error {
        // 处理事件
        return nil
    })

// 发布事件
bus.Publish(ctx, event.TopicSiteUpdated,
    event.NewEvent(event.TopicSiteUpdated, "api",
        map[string]interface{}{"site_id": "xxx"}))

// 取消订阅
sub.Unsubscribe()
```

### 7. 可观测性接口

新增 `pkg/observability/`：

**Logging**:
```go
import "prerender-shield/pkg/observability/logging"

logger := logging.NewContextLogger(baseLogger, ctx)
logger.Info("Request processed", "duration", 100*time.Millisecond)
```

**Metrics**:
```go
import "prerender-shield/pkg/observability/metrics"

recorder := metrics.NewInMemoryRecorder()
recorder.RecordRequestDuration("GET", "/api/v1/sites", 200, 50*time.Millisecond)
recorder.RecordWAFBlock("site1", "sql_injection")

// 获取指标
m := recorder.GetMetrics()
```

## 迁移步骤

### 1. 备份现有代码
```bash
git checkout -b backup-before-phase1-migration
```

### 2. 更新导入路径
在所有 Go 文件中替换：
```bash
# 替换 AI 模块导入
find . -name "*.go" -type f -exec sed -i '' \
    's|internal/ai/loganalyzer|internal/logging/analyzer|g' {} \;
find . -name "*.go" -type f -exec sed -i '' \
    's|internal/ai/behavioranalyzer|internal/security/waf/detectors/advanced|g' {} \;
```

### 3. 更新 main.go
原来的 `cmd/api/main.go` 已重命名为 `cmd/api/main.go.bak`，使用新的简化版本。

### 4. 验证编译
```bash
go build ./...
```

## 破坏性变更

### 1. main.go API 变更
- 原 `main.go` 中的所有初始化逻辑已移至 `internal/bootstrap/`
- 如需自定义初始化流程，请修改 `bootstrap/runner.go`

### 2. Redis 连接字符串格式
```go
// 之前
redis.NewClientWithURL("redis://localhost:6379/0")

// 之后 (内部解析)
// 支持格式：redis://password@host:port/db
// 或：host:port, password, db 分别传参
redis.NewClient("localhost:6379", "password", 0)
```

### 3. HealthChecker 接口变更
```go
// 之前返回 *monitoring.HealthChecker (具体类型)
// 之后返回 monitoring.HealthChecker (接口)
```

## 测试验证

运行以下测试确保迁移成功：
```bash
# 编译测试
go build ./cmd/api/main.go

# 单元测试
go test ./internal/bootstrap/...
go test ./pkg/...
go test ./internal/security/waf/...
go test ./internal/logging/analyzer/...
```

## 回滚方案

如需回滚：
```bash
# 恢复 main.go
mv cmd/api/main.go cmd/api/main.go.new
mv cmd/api/main.go.bak cmd/api/main.go

# 删除新增目录
rm -rf internal/bootstrap/
rm -rf internal/security/waf/
rm -rf internal/logging/analyzer/
rm -rf pkg/errors/
rm -rf pkg/event/
rm -rf pkg/observability/

# 恢复 ai 目录 (从备份)
git checkout internal/ai/
```

## 后续工作

- [ ] Phase 1.5: 更新所有测试文件
- [ ] Phase 2: 引入 Google Wire 依赖注入
- [ ] Phase 2: 配置系统重构
- [ ] Phase 3: 事件驱动架构实现
- [ ] Phase 3: 可观测性增强

## 参考文档

- [ARCHITECTURE_OPTIMIZATION_PLAN.md](./ARCHITECTURE_OPTIMIZATION_PLAN.md) - 完整架构优化计划
- [internal/interfaces.go](./internal/interfaces.go) - 核心接口定义
- [internal/bootstrap/application.go](./internal/bootstrap/application.go) - 应用容器实现
