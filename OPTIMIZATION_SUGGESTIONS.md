# Prerender Shield 优化建议与实施计划

**任务 ID:** JJC-20260308-012  
**制定日期:** 2026-03-08  
**版本:** 1.0

---

## 一、代码质量优化

### 1.1 测试覆盖率提升

**现状:** 30.0%  
**目标:** 80.0%+  
**优先级:** P0

#### 实施计划

**Phase 1: 核心模块 (Week 1)**

| 模块 | 当前 | 目标 | 测试重点 | 工时 |
|------|------|------|---------|------|
| internal/prerender | 0% | 70% | 渲染队列、超时处理、错误恢复 | 8h |
| internal/cache | 0% | 60% | 缓存读写、TTL、统计 | 4h |
| internal/middleware | 0% | 70% | 认证、限流、CORS | 4h |

**Phase 2: 重要模块 (Week 2)**

| 模块 | 当前 | 目标 | 测试重点 | 工时 |
|------|------|------|---------|------|
| internal/logging | 0% | 60% | 日志输出、轮转、审计 | 4h |
| internal/monitoring | 0% | 60% | 指标收集、告警触发 | 4h |
| internal/api | 0% | 70% | 路由注册、请求处理 | 4h |

**Phase 3: 辅助模块 (Week 3)**

| 模块 | 当前 | 目标 | 测试重点 | 工时 |
|------|------|------|---------|------|
| internal/crawler | 0% | 50% | 爬虫识别规则 | 2h |
| internal/services | 0% | 50% | 业务逻辑 | 4h |
| internal/routing | 0% | 50% | 路由匹配 | 2h |

#### 实施步骤

```bash
# 1. 为模块创建测试文件
touch internal/prerender/engine_test.go

# 2. 编写测试用例
# 参考 internal/auth/jwt_test.go 的模板

# 3. 运行测试并检查覆盖率
go test -coverprofile=coverage.out ./internal/prerender/...
go tool cover -html=coverage.out

# 4. 迭代改进，直到达到目标覆盖率
```

---

### 1.2 错误处理标准化

**现状:** 部分错误直接打印，未统一处理  
**目标:** 使用错误码和错误包装  
**优先级:** P1

#### 建议方案

**步骤 1: 定义错误码**

```go
// internal/errors/errors.go
package errors

import "errors"

// ErrorCode 错误码类型
type ErrorCode string

// 通用错误码
const (
    ErrUnknown          ErrorCode = "UNKNOWN"
    ErrInternal         ErrorCode = "INTERNAL_ERROR"
    ErrInvalidParam     ErrorCode = "INVALID_PARAM"
    ErrNotFound         ErrorCode = "NOT_FOUND"
    ErrUnauthorized     ErrorCode = "UNAUTHORIZED"
    ErrForbidden        ErrorCode = "FORBIDDEN"
    ErrTimeout          ErrorCode = "TIMEOUT"
)

// 业务错误码
const (
    ErrSiteNotFound     ErrorCode = "SITE_NOT_FOUND"
    ErrSiteExists       ErrorCode = "SITE_EXISTS"
    ErrRenderTimeout    ErrorCode = "RENDER_TIMEOUT"
    ErrRenderFailed     ErrorCode = "RENDER_FAILED"
    ErrCacheMiss        ErrorCode = "CACHE_MISS"
    ErrCacheWrite       ErrorCode = "CACHE_WRITE_FAILED"
    ErrWafBlocked       ErrorCode = "WAF_BLOCKED"
    ErrAuthFailed       ErrorCode = "AUTH_FAILED"
    ErrTokenExpired     ErrorCode = "TOKEN_EXPIRED"
    ErrConfigInvalid    ErrorCode = "CONFIG_INVALID"
)

// AppError 应用错误
type AppError struct {
    Code    ErrorCode
    Message string
    Cause   error
    Context map[string]interface{}
}

func (e *AppError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
    }
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
    return e.Cause
}

// NewError 创建应用错误
func NewError(code ErrorCode, message string, cause error) *AppError {
    return &AppError{
        Code:    code,
        Message: message,
        Cause:   cause,
        Context: make(map[string]interface{}),
    }
}

// WithContext 添加上下文信息
func (e *AppError) WithContext(key string, value interface{}) *AppError {
    e.Context[key] = value
    return e
}
```

**步骤 2: 使用错误包装**

```go
// 修复前
if err != nil {
    log.Printf("Failed to render: %v", err)
    return nil, err
}

// 修复后
if err != nil {
    return nil, errors.NewError(
        errors.ErrRenderFailed,
        "failed to render page",
        err,
    ).WithContext("url", url)
}
```

**步骤 3: 统一错误处理**

```go
// internal/api/middleware/error_handler.go
func ErrorHandler(c *gin.Context, err error) {
    var appErr *errors.AppError
    
    if errors.As(err, &appErr) {
        // 应用错误
        c.JSON(http.StatusBadRequest, gin.H{
            "error": appErr.Code,
            "message": appErr.Message,
            "context": appErr.Context,
        })
    } else {
        // 未知错误
        logging.DefaultLogger.Error("Unhandled error", zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "INTERNAL_ERROR",
            "message": "An unexpected error occurred",
        })
    }
}
```

**预计工时:** 6 小时

---

### 1.3 模块接口标准化

**现状:** 部分模块直接依赖具体实现  
**目标:** 面向接口编程，便于测试和扩展  
**优先级:** P1

#### 建议方案

**定义核心接口:**

```go
// internal/interfaces/interfaces.go
package interfaces

import "context"

// Cache 缓存接口
type Cache interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
    Clear(ctx context.Context, pattern string) error
}

// RenderEngine 渲染引擎接口
type RenderEngine interface {
    Render(ctx context.Context, url string, options RenderOptions) ([]byte, error)
    Warmup(ctx context.Context, urls []string) error
    GetStatus() RenderStatus
    Shutdown(ctx context.Context) error
}

// Firewall 防火墙接口
type Firewall interface {
    CheckRequest(ctx context.Context, req *Request) (*CheckResult, error)
    UpdateRules(rules []Rule) error
    GetStats() FirewallStats
}

// Repository 数据仓库接口
type Repository interface {
    Save(ctx context.Context, entity interface{}) error
    FindByID(ctx context.Context, id string) (interface{}, error)
    FindAll(ctx context.Context) ([]interface{}, error)
    Delete(ctx context.Context, id string) error
}
```

**重构依赖:**

```go
// 修复前
type PrerenderService struct {
    cache *redis.Client  // 具体类型
    engine *chromedp.Engine
}

// 修复后
type PrerenderService struct {
    cache interfaces.Cache      // 接口
    engine interfaces.RenderEngine
}

// 依赖注入
func NewPrerenderService(cache interfaces.Cache, engine interfaces.RenderEngine) *PrerenderService {
    return &PrerenderService{
        cache: cache,
        engine: engine,
    }
}
```

**收益:**
- 便于单元测试（可以使用 mock）
- 支持多种实现（Redis/Memcached, chromedp/playwright）
- 降低模块耦合

**预计工时:** 8 小时

---

## 二、性能优化

### 2.1 Chromium 实例池优化

**现状:** 实例复用但缺少健康检查  
**目标:** 动态扩缩容、健康检查  
**优先级:** P1

#### 建议方案

```go
// internal/prerender/pool.go
type BrowserPool struct {
    instances []*BrowserInstance
    mu        sync.RWMutex
    minSize   int
    maxSize   int
    healthCheckInterval time.Duration
}

type BrowserInstance struct {
    browser *chromedp.Browser
    lastUsed time.Time
    healthy  bool
    requestsHandled int64
}

// 健康检查
func (p *BrowserPool) StartHealthChecker() {
    go func() {
        ticker := time.NewTicker(p.healthCheckInterval)
        for range ticker.C {
            p.checkHealth()
        }
    }()
}

func (p *BrowserPool) checkHealth() {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    for _, inst := range p.instances {
        if !inst.IsHealthy() {
            inst.Close()
            p.removeInstance(inst)
        }
    }
    
    // 自动扩容
    if len(p.instances) < p.minSize {
        p.scaleUp(p.minSize - len(p.instances))
    }
}

// 动态扩缩容
func (p *BrowserPool) GetInstance() (*BrowserInstance, error) {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    // 查找可用实例
    for _, inst := range p.instances {
        if inst.IsAvailable() {
            inst.lastUsed = time.Now()
            return inst, nil
        }
    }
    
    // 需要扩容
    if len(p.instances) < p.maxSize {
        inst, err := p.createInstance()
        if err != nil {
            return nil, err
        }
        p.instances = append(p.instances, inst)
        return inst, nil
    }
    
    // 等待可用实例
    return p.waitForAvailableInstance()
}
```

**预计工时:** 8 小时

---

### 2.2 缓存策略优化

**现状:** 固定 TTL，缺少智能过期  
**目标:** 基于内容哈希的缓存失效、热点数据预加载  
**优先级:** P2

#### 建议方案

```go
// internal/cache/manager.go
type SmartCacheManager struct {
    cache Cache
    index *CacheIndex
}

type CacheIndex struct {
    urlToHash map[string]string
    hashToKeys map[string][]string
    hotKeys map[string]int  // 访问计数
}

// 基于内容哈希的缓存失效
func (m *SmartCacheManager) Set(url string, content []byte, ttl time.Duration) error {
    // 计算内容哈希
    hash := m.calculateHash(content)
    key := fmt.Sprintf("cache:%s", hash)
    
    // 检查是否有旧缓存
    if oldHash, exists := m.index.urlToHash[url]; exists && oldHash != hash {
        // 内容变化，使旧缓存失效
        m.invalidateByHash(oldHash)
    }
    
    // 存储新缓存
    return m.cache.Set(key, content, ttl)
}

// 热点数据检测
func (m *SmartCacheManager) Get(url string) ([]byte, error) {
    hash := m.index.urlToHash[url]
    key := fmt.Sprintf("cache:%s", hash)
    
    data, err := m.cache.Get(key)
    if err == nil {
        // 增加访问计数
        m.index.hotKeys[key]++
        
        // 如果是热点数据，预加载到内存
        if m.index.hotKeys[key] > 100 {
            m.prefetchToMemory(key, data)
        }
    }
    
    return data, err
}
```

**预计工时:** 6 小时

---

### 2.3 并发控制优化

**现状:** 固定并发数限制  
**目标:** 基于系统负载的动态并发调整  
**优先级:** P2

#### 建议方案

```go
// internal/prerender/limiter.go
type DynamicLimiter struct {
    baseLimit     int
    currentLimit  int32
    mu            sync.RWMutex
    checkInterval time.Duration
}

func NewDynamicLimiter(baseLimit int) *DynamicLimiter {
    l := &DynamicLimiter{
        baseLimit: baseLimit,
        currentLimit: int32(baseLimit),
        checkInterval: 10 * time.Second,
    }
    l.startAutoAdjust()
    return l
}

func (l *DynamicLimiter) startAutoAdjust() {
    go func() {
        ticker := time.NewTicker(l.checkInterval)
        for range ticker.C {
            l.adjust()
        }
    }()
}

func (l *DynamicLimiter) adjust() {
    // 获取系统负载
    loadAvg, _ := syscall.Getloadavg()
    cpuPercent, _ := getCPUPercent()
    memPercent, _ := getMemPercent()
    
    l.mu.Lock()
    defer l.mu.Unlock()
    
    // 根据负载调整
    if cpuPercent > 80 || memPercent > 80 {
        // 高负载，减少并发
        l.currentLimit = int32(float64(l.currentLimit) * 0.8)
        if l.currentLimit < 1 {
            l.currentLimit = 1
        }
    } else if cpuPercent < 50 && memPercent < 50 {
        // 低负载，增加并发
        l.currentLimit = int32(float64(l.currentLimit) * 1.2)
        if l.currentLimit > int32(l.baseLimit * 2) {
            l.currentLimit = int32(l.baseLimit * 2)
        }
    }
}

func (l *DynamicLimiter) Acquire() {
    semaphore <- struct{}{}
}

func (l *DynamicLimiter) Release() {
    <-semaphore
}
```

**预计工时:** 4 小时

---

## 三、安全加固

### 3.1 认证安全增强

**现状:** 基础 JWT 认证  
**目标:** 双因素认证、密码策略、登录限制  
**优先级:** P1

#### 建议方案

**1. 密码强度校验:**

```go
// internal/auth/password.go
func ValidatePasswordStrength(password string) error {
    if len(password) < 12 {
        return errors.New("password must be at least 12 characters")
    }
    
    hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
    hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
    hasDigit := regexp.MustCompile(`[0-9]`).MatchString(password)
    hasSpecial := regexp.MustCompile(`[!@#$%^&*]`).MatchString(password)
    
    if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
        return errors.New("password must contain uppercase, lowercase, digit, and special character")
    }
    
    return nil
}
```

**2. 登录失败限制:**

```go
// internal/auth/rate_limiter.go
type LoginRateLimiter struct {
    attempts map[string][]time.Time  // IP -> 尝试时间
    mu       sync.Mutex
    maxAttempts int
    lockoutDuration time.Duration
}

func (l *LoginRateLimiter) CheckLimit(ip string) error {
    l.mu.Lock()
    defer l.mu.Unlock()
    
    now := time.Now()
    windowStart := now.Add(-15 * time.Minute)
    
    // 清理过期记录
    attempts := l.attempts[ip]
    validAttempts := []time.Time{}
    for _, t := range attempts {
        if t.After(windowStart) {
            validAttempts = append(validAttempts, t)
        }
    }
    l.attempts[ip] = validAttempts
    
    // 检查是否超过限制
    if len(validAttempts) >= l.maxAttempts {
        return errors.New("too many login attempts, please try again later")
    }
    
    return nil
}

func (l *LoginRateLimiter) RecordAttempt(ip string) {
    l.mu.Lock()
    defer l.mu.Unlock()
    l.attempts[ip] = append(l.attempts[ip], time.Now())
}
```

**3. 双因素认证 (2FA):**

```go
// internal/auth/2fa.go
type TwoFactorAuth struct {
    redisClient *redis.Client
}

func (t *TwoFactorAuth) Enable2FA(userID string) (string, error) {
    // 生成 TOTP 密钥
    secret := otp.GenerateSecret()
    
    // 保存到 Redis
    t.redisClient.Set(fmt.Sprintf("2fa:%s", userID), secret, 0)
    
    // 返回二维码 URL
    qrURL := otp.GenerateQRCode(userID, secret)
    return qrURL, nil
}

func (t *TwoFactorAuth) Verify2FA(userID string, code string) error {
    secret, err := t.redisClient.Get(fmt.Sprintf("2fa:%s", userID))
    if err != nil {
        return errors.New("2FA not enabled")
    }
    
    if !otp.VerifyCode(secret, code) {
        return errors.New("invalid 2FA code")
    }
    
    return nil
}
```

**预计工时:** 8 小时

---

### 3.2 API 安全增强

**现状:** 基础认证  
**目标:** API 签名、速率限制、审计日志  
**优先级:** P2

#### 建议方案

**1. API 请求签名:**

```go
// internal/middleware/signature.go
func SignatureVerification(secret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        timestamp := c.GetHeader("X-Timestamp")
        signature := c.GetHeader("X-Signature")
        
        // 检查时间戳（防止重放攻击）
        ts, err := strconv.ParseInt(timestamp, 10, 64)
        if err != nil || time.Now().Unix()-ts > 300 {
            c.JSON(401, gin.H{"error": "invalid timestamp"})
            c.Abort()
            return
        }
        
        // 验证签名
        expectedSig := hmacSHA256(c.Request.URL.Path, secret, timestamp)
        if signature != expectedSig {
            c.JSON(401, gin.H{"error": "invalid signature"})
            c.Abort()
            return
        }
        
        c.Next()
    }
}
```

**2. 速率限制:**

```go
// internal/middleware/rate_limit.go
func RateLimit(perSecond int) gin.HandlerFunc {
    limiter := rate.NewLimiter(rate.Limit(perSecond), perSecond*2)
    
    return func(c *gin.Context) {
        if !limiter.Allow() {
            c.JSON(429, gin.H{"error": "too many requests"})
            c.Abort()
            return
        }
        c.Next()
    }
}
```

**预计工时:** 6 小时

---

## 四、可观测性增强

### 4.1 结构化日志

**现状:** 文本日志  
**目标:** JSON 格式、链路追踪、日志采样  
**优先级:** P2

#### 建议方案

```go
// internal/logging/structured.go
import "go.uber.org/zap"

var logger *zap.Logger

func InitStructuredLogger() {
    config := zap.NewProductionConfig()
    config.OutputPaths = []string{"stdout", "/var/log/app.log"}
    config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
    
    logger, _ = config.Build()
}

// 使用示例
logger.Info("site started",
    zap.String("site_id", site.ID),
    zap.Int("port", site.Port),
    zap.String("mode", site.Mode),
    zap.Duration("startup_time", startupTime),
)

logger.Error("render failed",
    zap.String("url", url),
    zap.Error(err),
    zap.Int("retry_count", retryCount),
)
```

**预计工时:** 4 小时

---

### 4.2 分布式追踪

**现状:** 无  
**目标:** OpenTelemetry 集成  
**优先级:** P2

#### 建议方案

```go
// internal/tracing/tracing.go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/sdk/trace"
)

func InitTracing() (*trace.TracerProvider, error) {
    exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(
        jaeger.WithEndpoint("http://jaeger:14268/api/traces"),
    ))
    if err != nil {
        return nil, err
    }
    
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithSampler(trace.AlwaysSample()),
    )
    
    otel.SetTracerProvider(tp)
    return tp, nil
}

// 使用示例
func HandleRequest(ctx context.Context, url string) error {
    ctx, span := otel.Tracer("prerender-shield").Start(ctx, "HandleRequest")
    defer span.End()
    
    span.SetAttributes(
        attribute.String("url", url),
        attribute.String("method", "GET"),
    )
    
    // 业务逻辑
    return nil
}
```

**预计工时:** 8 小时

---

## 五、部署优化

### 5.1 Docker 化

**现状:** 二进制部署  
**目标:** 官方 Docker 镜像  
**优先级:** P2

#### Dockerfile

```dockerfile
# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /api ./cmd/api

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/
COPY --from=builder /api .
COPY --from=builder /app/configs/config.yml ./config.yml

EXPOSE 9597 9598

HEALTHCHECK --interval=30s --timeout=3s \
    CMD wget -qO- http://localhost:9598/api/v1/health || exit 1

CMD ["./api", "-config", "config.yml"]
```

**预计工时:** 4 小时

---

### 5.2 Kubernetes 支持

**现状:** 无  
**目标:** Helm Chart  
**优先级:** P3

#### Helm Chart 结构

```
charts/prerender-shield/
├── Chart.yaml
├── values.yaml
├── templates/
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   ├── ingress.yaml
│   └── hpa.yaml
```

**预计工时:** 8 小时

---

## 六、测试改进

### 6.1 压力测试

**现状:** 无  
**目标:** k6 压力测试脚本  
**优先级:** P1

#### k6 脚本

```javascript
// tests/performance/load_test.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const errorRate = new Rate('errors');

export const options = {
  stages: [
    { duration: '30s', target: 100 },   // 热身
    { duration: '1m', target: 500 },    // 中负载
    { duration: '30s', target: 1000 },  // 高负载
    { duration: '1m', target: 1000 },   // 稳定
    { duration: '30s', target: 0 },     // 冷却
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],   // 95% 请求 < 500ms
    http_req_failed: ['rate<0.01'],     // 错误率 < 1%
    errors: ['rate<0.1'],               // 自定义错误率 < 10%
  },
};

export default function () {
  // 健康检查
  const healthRes = http.get('http://localhost:9598/api/v1/health');
  check(healthRes, {
    'health status is 200': (r) => r.status === 200,
  });
  errorRate.add(healthRes.status !== 200);
  
  sleep(1);
  
  // 站点列表
  const sitesRes = http.get('http://localhost:9598/api/v1/sites');
  check(sitesRes, {
    'sites status is 200': (r) => r.status === 200,
  });
  errorRate.add(sitesRes.status !== 200);
  
  sleep(1);
}
```

**运行命令:**
```bash
k6 run tests/performance/load_test.js
k6 run --out influxdb=http://localhost:8086/k6 tests/performance/load_test.js
```

**预计工时:** 4 小时

---

## 七、实施优先级总结

| 优先级 | 优化项 | 预计工时 | 收益 |
|--------|--------|---------|------|
| P0 | 测试覆盖率提升 | 24h | 高 - 质量保障 |
| P0 | 配置热重载回滚 | 4h | 高 - 稳定性 |
| P1 | 错误处理标准化 | 6h | 中 - 可维护性 |
| P1 | 模块接口标准化 | 8h | 中 - 可测试性 |
| P1 | 压力测试 | 4h | 高 - 性能保障 |
| P1 | 认证安全增强 | 8h | 高 - 安全性 |
| P2 | Chromium 池优化 | 8h | 中 - 性能 |
| P2 | 缓存策略优化 | 6h | 中 - 性能 |
| P2 | 并发控制优化 | 4h | 中 - 性能 |
| P2 | API 安全增强 | 6h | 中 - 安全性 |
| P2 | 结构化日志 | 4h | 中 - 可观测性 |
| P2 | 分布式追踪 | 8h | 中 - 可观测性 |
| P2 | Docker 化 | 4h | 中 - 部署便利 |
| P3 | Kubernetes 支持 | 8h | 低 - 扩展性 |

**总预计工时:** 106 小时（约 13 个工作日）

---

## 八、实施路线图

### Phase 1 (Month 1): 质量基础
- [x] WaitGroup 并发修复
- [x] Auth 模块测试
- [ ] 测试覆盖率提升至 50%
- [ ] 配置热重载回滚
- [ ] 压力测试

### Phase 2 (Month 2): 安全加固
- [ ] 错误处理标准化
- [ ] 认证安全增强 (2FA)
- [ ] API 安全增强
- [ ] 模块接口标准化

### Phase 3 (Month 3): 性能优化
- [ ] Chromium 池优化
- [ ] 缓存策略优化
- [ ] 并发控制优化
- [ ] 结构化日志

### Phase 4 (Month 4): 可观测性
- [ ] 分布式追踪
- [ ] 监控指标增强
- [ ] 告警规则完善

### Phase 5 (Month 5): 部署优化
- [ ] Docker 化
- [ ] Kubernetes 支持
- [ ] CI/CD 集成

---

**文档维护:** 每次实施优化后更新进度  
**最后更新:** 2026-03-08 15:45
