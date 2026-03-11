# Prerender Shield 优化开发计划

**项目 ID:** OPT-DEV-20260311-001
**制定日期:** 2026-03-11
**版本:** 1.0
**总工期:** 12 周（94 小时）

---

## 一、项目概览

### 优化目标
1. 实现 SSL 证书自动申请和续签（Let's Encrypt）
2. 性能提升 40%（渲染加速、缓存优化）
3. 安全加固（2FA、WAF 增强）
4. 代码质量提升（测试覆盖率 80%+）
5. 可观测性增强（健康检查、结构化日志）

### 技术栈
- **Go:** 1.24.0
- **ACME 库:** github.com/go-acme/lego/v4
- **Web 框架:** gin-gonic/gin v1.9.0
- **Redis:** go-redis/redis/v8
- **渲染:** chromedp/chromedp

---

## 二、任务清单（18 项）

### Phase 1: SSL 自动签署 + 稳定性（4 周 / 40 小时）

| 序号 | 任务 ID | 任务名称 | 优先级 | 工时 | 状态 |
|------|--------|----------|--------|------|------|
| 1 | SSL-01 | 完整 ACME 流程实现 | P0 | 8h | ⏳ 待开始 |
| 2 | SSL-02 | HTTP-01 挑战服务器 | P0 | 4h | ⏳ 待开始 |
| 3 | CQ-01-P1 | 测试覆盖率 Phase 1 | P0 | 8h | ⏳ 待开始 |
| 4 | OB-01 | 健康检查增强 | P0 | 4h | ⏳ 待开始 |
| 5 | SSL-04 | 自动续签 + 通知 | P1 | 4h | ⏳ 待开始 |
| 6 | SSL-03 | DNS-01 挑战集成 | P1 | 4h | ⏳ 待开始 |
| 7 | CQ-01-P2 | 测试覆盖率 Phase 2 | P1 | 8h | ⏳ 待开始 |

### Phase 2: 性能 + 安全（4 周 / 38 小时）

| 序号 | 任务 ID | 任务名称 | 优先级 | 工时 | 状态 |
|------|--------|----------|--------|------|------|
| 8 | PF-01 | Chromium 实例池优化 | P1 | 8h | ⏳ 待开始 |
| 9 | PF-02 | 渲染队列优先级 | P1 | 8h | ⏳ 待开始 |
| 10 | SC-01 | 双因素认证 (2FA) | P1 | 8h | ⏳ 待开始 |
| 11 | SC-02 | WAF 规则增强 | P1 | 6h | ⏳ 待开始 |
| 12 | CQ-02 | 错误处理标准化 | P1 | 6h | ⏳ 待开始 |
| 13 | CQ-03 | 配置热重载回滚 | P1 | 4h | ⏳ 待开始 |

### Phase 3: 完善优化（4 周 / 16 小时）

| 序号 | 任务 ID | 任务名称 | 优先级 | 工时 | 状态 |
|------|--------|----------|--------|------|------|
| 14 | OB-02 | 结构化日志 | P2 | 8h | ⏳ 待开始 |
| 15 | PF-03 | 缓存策略优化 | P2 | 6h | ⏳ 待开始 |
| 16 | PF-04 | 动态并发限制 | P2 | 4h | ⏳ 待开始 |
| 17 | SC-03 | API 速率限制 | P2 | 4h | ⏳ 待开始 |
| 18 | SC-04 | 敏感数据加密 | P2 | 4h | ⏳ 待开始 |
| 19 | SSL-05 | SSL 管理 API | P2 | 4h | ⏳ 待开始 |
| 20 | SSL-06 | 多域名支持 | P2 | 2h | ⏳ 待开始 |

---

## 三、详细开发计划

### Week 1: ACME 基础 + 依赖安装

**目标:** 完成 SSL 自动签署核心功能

#### 任务 1: SSL-01 完整 ACME 流程 (8h)

**工作内容:**
1. 添加 LEGO 依赖
2. 创建 `internal/ssl/acme_client.go`
3. 实现 ACME 账户注册
4. 实现证书申请流程
5. 测试（使用 Let's Encrypt Staging）

**文件清单:**
- `internal/ssl/acme_client.go` (新增)
- `internal/ssl/account.go` (新增)
- `go.mod` (修改)

**验收标准:**
- [ ] 能够成功注册 Let's Encrypt 账户
- [ ] 能够申请单域名证书
- [ ] 证书保存到正确目录
- [ ] 单元测试通过

**实现代码框架:**
```go
// internal/ssl/acme_client.go
package ssl

import (
    "github.com/go-acme/lego/v4/certcrypto"
    "github.com/go-acme/lego/v4/certificate"
    "github.com/go-acme/lego/v4/lego"
    "github.com/go-acme/lego/v4/registration"
)

type ACMEClient struct {
    client     *lego.Client
    certDir    string
    email      string
    account    *Account
    production bool
}

type Account struct {
    Email        string
    Registration *registration.Resource
    key          crypto.PrivateKey
}

func NewACMEClient(config ACMEConfig) (*ACMEClient, error) {
    // 实现账户注册和客户端创建
}

func (c *ACMEClient) RequestCertificate(domains []string) (*certificate.Resource, error) {
    // 实现证书申请
}

func (c *ACMEClient) RenewCertificate(cert *certificate.Resource) (*certificate.Resource, error) {
    // 实现证书续签
}
```

---

#### 任务 2: SSL-02 HTTP-01 挑战服务器 (4h)

**工作内容:**
1. 创建 `internal/ssl/http_challenge.go`
2. 实现 HTTP 挑战响应服务器
3. 集成到 ACME 客户端
4. 测试域名验证流程

**文件清单:**
- `internal/ssl/http_challenge.go` (新增)
- `internal/api/routes/acme.go` (新增路由)

**验收标准:**
- [ ] HTTP-01 挑战自动响应
- [ ] Let's Encrypt 验证通过
- [ ] 挑战服务器可启动/停止

**实现代码框架:**
```go
// internal/ssl/http_challenge.go
package ssl

import (
    "context"
    "fmt"
    "net/http"
    "sync"
)

type HTTPProvider struct {
    port   int
    server *http.Server
    tokens map[string]string
    mu     sync.RWMutex
}

func NewHTTPProvider(port int) *HTTPProvider {
    h := &HTTPProvider{
        port:   port,
        tokens: make(map[string]string),
    }
    mux := http.NewServeMux()
    mux.HandleFunc("/.well-known/acme-challenge/", h.handleChallenge)
    h.server = &http.Server{
        Addr:    fmt.Sprintf(":%d", port),
        Handler: mux,
    }
    return h
}

func (h *HTTPProvider) Present(domain, token, keyAuth string) error {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.tokens[token] = keyAuth
    return nil
}

func (h *HTTPProvider) CleanUp(domain, token, keyAuth string) error {
    h.mu.Lock()
    defer h.mu.Unlock()
    delete(h.tokens, token)
    return nil
}

func (h *HTTPProvider) handleChallenge(w http.ResponseWriter, r *http.Request) {
    token := r.URL.Path[len("/.well-known/acme-challenge/"):]
    h.mu.RLock()
    keyAuth, exists := h.tokens[token]
    h.mu.RUnlock()
    if !exists {
        http.NotFound(w, r)
        return
    }
    w.Header().Set("Content-Type", "text/plain")
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(keyAuth))
}

func (h *HTTPProvider) Start() error {
    go func() {
        if err := h.server.ListenAndServe(); err != http.ErrServerClosed {
            log.Printf("HTTP challenge server error: %v", err)
        }
    }()
    return nil
}

func (h *HTTPProvider) Stop(ctx context.Context) error {
    return h.server.Shutdown(ctx)
}
```

---

#### 任务 3: CQ-01-P1 测试覆盖率 Phase 1 (8h)

**工作内容:**
1. 为 `internal/prerender` 创建测试
2. 为 `internal/cache` 创建测试
3. 为 `internal/ssl` 创建测试
4. 运行覆盖率检查

**文件清单:**
- `internal/prerender/engine_test.go` (新增)
- `internal/cache/manager_test.go` (新增)
- `internal/ssl/acme_client_test.go` (新增)

**验收标准:**
- [ ] 核心模块覆盖率达标 30%+
- [ ] 所有测试通过
- [ ] 生成覆盖率报告

---

#### 任务 4: OB-01 健康检查增强 (4h)

**工作内容:**
1. 增强 `/api/v1/health` 端点
2. 添加 SSL 证书状态检查
3. 添加各模块健康状态
4. 添加 Prometheus 指标

**文件清单:**
- `internal/api/controllers/system_controller.go` (修改)
- `internal/monitoring/health_checker.go` (修改)

**验收标准:**
- [ ] 健康检查返回详细状态
- [ ] SSL 证书状态包含在内
- [ ] Prometheus 可抓取指标

---

### Week 2-4: Phase 1 剩余任务

（详细开发内容将在每个任务开始时展开）

---

### Week 5-8: Phase 2 性能 + 安全

（详细开发内容将在 Phase 1 完成后展开）

---

### Week 9-12: Phase 3 完善优化

（详细开发内容将在 Phase 2 完成后展开）

---

## 四、开发环境准备

### 4.1 依赖安装

```bash
# 进入项目目录
cd /Users/xiaofang/Documents/www/prerender/prerender-shield

# 添加 ACME 依赖
go get github.com/go-acme/lego/v4@latest

# 添加其他依赖
go get go.uber.org/zap@latest
go get github.com/pquerna/otp@latest  # 用于 2FA

# 更新依赖
go mod tidy
```

### 4.2 开发分支

```bash
# 创建开发分支
git checkout -b feature/optimization-phase1

# Phase 1 完成后合并
git checkout main
git merge feature/optimization-phase1
```

### 4.3 测试环境配置

```yaml
# configs/config.test.yml
ssl:
  enabled: true
  email: test@example.com
  production: false  # 使用 Let's Encrypt Staging
  http_challenge_port: 80
```

---

## 五、任务执行流程

### 每个任务的开发流程

```
1. 阅读任务详情 → 2. 创建开发分支 → 3. 编写代码
       ↓
7. 关闭任务 ← 6. 代码审查 ← 5. 运行测试 ← 4. 提交代码
```

### 提交规范

```bash
# SSL 相关
git commit -m "feat(ssl): implement ACME client for Let's Encrypt integration

- Add LEGO library integration
- Implement account registration
- Add certificate request flow
- Support both staging and production environments"

# 测试相关
git commit -m "test(prerender): add unit tests for render engine

- Add tests for NewEngine function
- Add tests for Render method
- Add tests for timeout handling
- Achieve 70% coverage for prerender package"

# 性能优化
git commit -m "perf(chromium): optimize browser pool with dynamic scaling

- Implement health check for browser instances
- Add auto-scaling based on demand
- Reduce memory usage by 30%"
```

---

## 六、进度跟踪

### 总体进度

```
Phase 1: SSL + 稳定性 (4 周)
├─ SSL-01 ACME 流程      [        ] 0%
├─ SSL-02 HTTP 挑战      [        ] 0%
├─ CQ-01-P1 测试覆盖率 P1 [        ] 0%
├─ OB-01 健康检查       [        ] 0%
├─ SSL-04 自动续签      [        ] 0%
├─ SSL-03 DNS 挑战      [        ] 0%
└─ CQ-01-P2 测试覆盖率 P2 [        ] 0%

Phase 2: 性能 + 安全 (4 周)
├─ PF-01 Chromium 池    [        ] 0%
├─ PF-02 渲染队列       [        ] 0%
├─ SC-01 双因素认证     [        ] 0%
├─ SC-02 WAF 增强       [        ] 0%
├─ CQ-02 错误处理       [        ] 0%
└─ CQ-03 配置回滚       [        ] 0%

Phase 3: 完善优化 (4 周)
├─ OB-02 结构化日志     [        ] 0%
├─ PF-03 缓存优化       [        ] 0%
├─ PF-04 并发限制       [        ] 0%
├─ SC-03 API 限流       [        ] 0%
├─ SC-04 数据加密       [        ] 0%
├─ SSL-05 SSL 管理 API  [        ] 0%
└─ SSL-06 多域名支持    [        ] 0%
```

---

## 七、现在开始：任务 1 - SSL-01

### 准备执行

**任务详情:**
- **名称:** 完整 ACME 流程实现
- **工时:** 8 小时
- **优先级:** P0
- **依赖:** 无

**下一步:** 请确认是否开始任务 1，我将：
1. 添加 LEGO 依赖到 `go.mod`
2. 创建 `internal/ssl/acme_client.go`
3. 实现完整的 ACME 证书申请流程
4. 编写测试用例

**输入 "开始任务 1" 或 "yes" 立即启动**

---

**文档状态:** ✅ 开发计划已就绪
**最后更新:** 2026-03-11
