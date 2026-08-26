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


---

# 第三部分：2026年开发计划

> 以下内容合并自 prerender-shield-202601开发计划.md


# Prerender Shield 具体扩展方案

**生成时间:** 2026-03-11  
**任务 ID:** JJC-20260311-004  
**项目位置:** `/Users/xiaofang/Documents/www/prerender/prerender-shield/`

---

## 一、AI 威胁检测引擎扩展

### 1.1 架构设计

```
internal/firewall/detectors/
├── ai/                      # 新增 AI 检测模块
│   ├── detector.go          # AI 检测器主文件
│   ├── model.go             # 模型加载和管理
│   ├── features.go          # 特征提取
│   ├── tokenizer.go         # 文本分词
│   └── config.go            # AI 配置
```

### 1.2 核心代码实现

#### detector.go - AI 检测器

```go
package ai

import (
	"context"
	"net/http"
	"sync"
	"time"

	"prerender-shield/internal/firewall/types"
)

// AIDetector AI威胁检测器
type AIDetector struct {
	model         *ThreatModel
	featureCache  *FeatureCache
	config        *Config
	predictChan   chan *PredictRequest
	resultChan    chan *PredictResult
	workerPool    int
	mu            sync.RWMutex
}

// PredictRequest 预测请求
type PredictRequest struct {
	RequestID string
	Features  []float32
	Response  chan *PredictResult
}

// PredictResult 预测结果
type PredictResult struct {
	RequestID  string
	ThreatType string
	Confidence float32
	IsMalicious bool
}

// NewAIDetector 创建AI检测器
func NewAIDetector(config *Config) (*AIDetector, error) {
	// 加载预训练模型
	model, err := LoadThreatModel(config.ModelPath)
	if err != nil {
		return nil, err
	}

	detector := &AIDetector{
		model:        model,
		featureCache: NewFeatureCache(10000, 5*time.Minute),
		config:       config,
		predictChan:  make(chan *PredictRequest, 1000),
		resultChan:   make(chan *PredictResult, 1000),
		workerPool:   config.WorkerPool,
	}

	// 启动预测工作池
	for i := 0; i < config.WorkerPool; i++ {
		go detector.predictWorker()
	}

	return detector, nil
}

// Detect 实现OWASPDetector接口
func (d *AIDetector) Detect(req *http.Request) ([]types.Threat, error) {
	// 提取特征
	features, err := d.extractFeatures(req)
	if err != nil {
		return nil, err
	}

	// 发送预测请求
	resultChan := make(chan *PredictResult, 1)
	d.predictChan <- &PredictRequest{
		RequestID: generateRequestID(req),
		Features:  features,
		Response:  resultChan,
	}

	// 等待结果（带超时）
	select {
	case result := <-resultChan:
		if result.IsMalicious && result.Confidence > d.config.ConfidenceThreshold {
			return []types.Threat{{
				Type:        result.ThreatType,
				Severity:    d.getSeverity(result.Confidence),
				Description: "AI检测到潜在威胁",
				Confidence:  result.Confidence,
				Source:      "ai_detector",
			}}, nil
		}
	case <-time.After(d.config.PredictTimeout):
		// 超时则返回空，继续其他检测
		return nil, nil
	}

	return nil, nil
}

// predictWorker 预测工作协程
func (d *AIDetector) predictWorker() {
	for req := range d.predictChan {
		// 模型推理
		prediction := d.model.Predict(req.Features)
		
		req.Response <- &PredictResult{
			RequestID:   req.RequestID,
			ThreatType:  prediction.ThreatType,
			Confidence:  prediction.Confidence,
			IsMalicious: prediction.IsMalicious,
		}
	}
}

// extractFeatures 特征提取
func (d *AIDetector) extractFeatures(req *http.Request) ([]float32, error) {
	// 检查缓存
	cacheKey := generateRequestID(req)
	if cached, ok := d.featureCache.Get(cacheKey); ok {
		return cached, nil
	}

	features := make([]float32, 0, 128)

	// URL特征
	features = append(features, extractURLFeatures(req.URL)...)

	// Header特征
	features = append(features, extractHeaderFeatures(req.Header)...)

	// Body特征（如果有）
	if req.Body != nil {
		bodyFeatures, err := extractBodyFeatures(req.Body)
		if err == nil {
			features = append(features, bodyFeatures...)
		}
	}

	// 行为特征
	features = append(features, extractBehaviorFeatures(req)...)

	// 缓存特征
	d.featureCache.Set(cacheKey, features)

	return features, nil
}

// Name 实现接口
func (d *AIDetector) Name() string {
	return "ai_detector"
}
```

#### model.go - 模型管理

```go
package ai

import (
	"encoding/json"
	"os"
	"sync"

	tf "github.com/tensorflow/tensorflow/tensorflow/go"
)

// ThreatModel 威胁检测模型
type ThreatModel struct {
	session    *tf.Session
	graph      *tf.Graph
	labels     []string
	mu         sync.RWMutex
	version    string
	inputSize  int
}

// Prediction 预测结果
type Prediction struct {
	ThreatType  string
	Confidence  float32
	IsMalicious bool
}

// LoadThreatModel 加载模型
func LoadThreatModel(modelPath string) (*ThreatModel, error) {
	// 加载模型文件
	modelBytes, err := os.ReadFile(modelPath + "/model.pb")
	if err != nil {
		return nil, err
	}

	// 创建图
	graph := tf.NewGraph()
	if err := graph.Import(modelBytes, ""); err != nil {
		return nil, err
	}

	// 创建会话
	session, err := tf.NewSession(graph, nil)
	if err != nil {
		return nil, err
	}

	// 加载标签
	labels, err := loadLabels(modelPath + "/labels.json")
	if err != nil {
		return nil, err
	}

	return &ThreatModel{
		session:   session,
		graph:     graph,
		labels:    labels,
		inputSize: 128, // 特征向量大小
	}, nil
}

// Predict 执行预测
func (m *ThreatModel) Predict(features []float32) *Prediction {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 创建输入张量
	tensor, err := tf.NewTensor([][]float32{features})
	if err != nil {
		return &Prediction{IsMalicious: false}
	}

	// 执行推理
	output, err := m.session.Run(
		map[tf.Output]*tf.Tensor{
			m.graph.Operation("input").Output(0): tensor,
		},
		[]tf.Output{
			m.graph.Operation("output").Output(0),
		},
		nil,
	)
	if err != nil {
		return &Prediction{IsMalicious: false}
	}

	// 解析输出
	predictions := output[0].Value().([][]float32)[0]
	
	// 找到最大概率的类别
	maxIdx := 0
	maxProb := predictions[0]
	for i, prob := range predictions {
		if prob > maxProb {
			maxProb = prob
			maxIdx = i
		}
	}

	return &Prediction{
		ThreatType:  m.labels[maxIdx],
		Confidence:  maxProb,
		IsMalicious: maxProb > 0.5 && m.labels[maxIdx] != "benign",
	}
}

// Close 关闭模型
func (m *ThreatModel) Close() {
	m.session.Close()
}
```

### 1.3 集成到现有系统

修改 `internal/firewall/engine.go`:

```go
// 在NewEngine中添加AI检测器
func NewEngine(cfg *config.Config, logger Logger) *Engine {
	engine := &Engine{
		// ... 现有代码 ...
	}

	// 添加AI检测器（如果启用）
	if cfg.Security.AIDetector.Enabled {
		aiDetector, err := ai.NewAIDetector(&ai.Config{
			ModelPath:           cfg.Security.AIDetector.ModelPath,
			WorkerPool:          cfg.Security.AIDetector.WorkerPool,
			ConfidenceThreshold: cfg.Security.AIDetector.ConfidenceThreshold,
			PredictTimeout:      time.Duration(cfg.Security.AIDetector.TimeoutMs) * time.Millisecond,
		})
		if err != nil {
			logger.Warn("AI检测器初始化失败，将使用传统检测", "error", err)
		} else {
			engine.owaspDetectors["ai"] = aiDetector
			logger.Info("AI检测器已启用")
		}
	}

	return engine
}
```

---

## 二、DDoS 防护模块扩展

### 2.1 架构设计

```
internal/firewall/detectors/
├── ddos/                    # 新增 DDoS 防护模块
│   ├── detector.go          # DDoS 检测器
│   ├── rate_limiter.go      # 速率限制
│   ├── challenge.go         # 人机验证
│   ├── ip_tracker.go        # IP 追踪
│   └── syn_flood.go         # SYN Flood 防护
```

### 2.2 核心代码实现

#### detector.go - DDoS 检测器

```go
package ddos

import (
	"context"
	"net/http"
	"sync"
	"time"

	"prerender-shield/internal/firewall/types"
)

// DDoSDetector DDoS检测器
type DDoSDetector struct {
	rateLimiter  *RateLimiter
	ipTracker    *IPTracker
	challenge    *Challenge
	blacklist    *Blacklist
	config       *Config
	mu           sync.RWMutex
}

// Config DDoS配置
type Config struct {
	// 速率限制
	RequestsPerSecond   int           // 每秒请求数阈值
	BurstSize          int           // 突发大小
	RateLimitWindow    time.Duration // 窗口时间

	// IP追踪
	MaxIPConnections   int           // 最大连接数
	IPBlockDuration    time.Duration // 封禁时长

	// 挑战验证
	ChallengeEnabled   bool          // 启用人机验证
	ChallengeTimeout   time.Duration // 挑战超时

	// SYN Flood
	SYNCookieEnabled   bool          // 启用SYN Cookie
}

// NewDDoSDetector 创建DDoS检测器
func NewDDoSDetector(config *Config) *DDoSDetector {
	detector := &DDoSDetector{
		rateLimiter: NewRateLimiter(config.RequestsPerSecond, config.BurstSize),
		ipTracker:   NewIPTracker(config.MaxIPConnections),
		challenge:   NewChallenge(config.ChallengeTimeout),
		blacklist:   NewBlacklist(),
		config:      config,
	}

	// 启动后台清理任务
	go detector.cleanupWorker()

	return detector
}

// Detect 实现接口
func (d *DDoSDetector) Detect(req *http.Request) ([]types.Threat, error) {
	clientIP := getClientIP(req)

	// 1. 检查黑名单
	if d.blacklist.IsBlocked(clientIP) {
		return []types.Threat{{
			Type:        "ddos_blacklist",
			Severity:    "high",
			Description: "IP在黑名单中",
			Source:      "ddos_detector",
		}}, nil
	}

	// 2. 速率限制检查
	if !d.rateLimiter.Allow(clientIP) {
		// 触发人机验证
		if d.config.ChallengeEnabled {
			if !d.challenge.Verify(req) {
				d.blacklist.Block(clientIP, d.config.IPBlockDuration)
				return []types.Threat{{
					Type:        "ddos_rate_limit",
					Severity:    "medium",
					Description: "请求频率过高，需要人机验证",
					Source:      "ddos_detector",
				}}, nil
			}
		} else {
			return []types.Threat{{
				Type:        "ddos_rate_limit",
				Severity:    "medium",
				Description: "请求频率过高",
				Source:      "ddos_detector",
			}}, nil
		}
	}

	// 3. 连接数检查
	if d.ipTracker.ExceedsLimit(clientIP) {
		d.blacklist.Block(clientIP, d.config.IPBlockDuration)
		return []types.Threat{{
			Type:        "ddos_connection_limit",
			Severity:    "high",
			Description: "连接数超过限制",
			Source:      "ddos_detector",
		}}, nil
	}

	return nil, nil
}

// Name 实现接口
func (d *DDoSDetector) Name() string {
	return "ddos_detector"
}
```

#### rate_limiter.go - 速率限制器

```go
package ddos

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter 速率限制器
type RateLimiter struct {
	limiters  map[string]*rate.Limiter
	mu        sync.RWMutex
	rps       int           // 每秒请求数
	burst     int           // 突发大小
	cleanup   time.Duration // 清理间隔
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(rps, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rps:      rps,
		burst:    burst,
		cleanup:  time.Minute,
	}

	go rl.cleanupWorker()

	return rl
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[ip]
	if !exists {
		limiter = rate.NewLimiter(rate.Limit(rl.rps), rl.burst)
		rl.limiters[ip] = limiter
	}

	return limiter.Allow()
}

// GetLimiter 获取限制器
func (rl *RateLimiter) GetLimiter(ip string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[ip]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		limiter = rate.NewLimiter(rate.Limit(rl.rps), rl.burst)
		rl.limiters[ip] = limiter
		rl.mu.Unlock()
	}

	return limiter
}

// cleanupWorker 清理过期限制器
func (rl *RateLimiter) cleanupWorker() {
	ticker := time.NewTicker(rl.cleanup)
	for range ticker.C {
		rl.mu.Lock()
		for ip, limiter := range rl.limiters {
			// 如果限制器已经允许请求（不活跃），则删除
			if limiter.Allow() {
				delete(rl.limiters, ip)
			}
		}
		rl.mu.Unlock()
	}
}
```

#### challenge.go - 人机验证

```go
package ddos

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"
)

// Challenge 人机验证
type Challenge struct {
	timeout  time.Duration
	secret   string
	challenges map[string]*ChallengeData
}

// ChallengeData 挑战数据
type ChallengeData struct {
	Token     string
	Timestamp int64
	IP        string
	Attempts  int
}

// NewChallenge 创建挑战
func NewChallenge(timeout time.Duration) *Challenge {
	return &Challenge{
		timeout:    timeout,
		secret:     generateSecret(),
		challenges: make(map[string]*ChallengeData),
	}
}

// GenerateChallenge 生成挑战
func (c *Challenge) GenerateChallenge(ip string) map[string]interface{} {
	token := c.generateToken(ip)
	timestamp := time.Now().Unix()

	// 计算挑战答案（客户端需要计算）
	challenge := token + string(timestamp) + c.secret
	hash := sha256.Sum256([]byte(challenge))
	answer := hex.EncodeToString(hash[:])

	// 存储挑战
	c.challenges[ip] = &ChallengeData{
		Token:     token,
		Timestamp: timestamp,
		IP:        ip,
		Attempts:  0,
	}

	return map[string]interface{}{
		"token":     token,
		"timestamp": timestamp,
		"script":    c.generateChallengeScript(token, timestamp),
		"answer":    answer, // 仅用于测试，生产环境不返回
	}
}

// Verify 验证挑战
func (c *Challenge) Verify(req *http.Request) bool {
	ip := getClientIP(req)

	// 检查是否有挑战
	data, exists := c.challenges[ip]
	if !exists {
		return false
	}

	// 检查是否过期
	if time.Now().Unix()-data.Timestamp > int64(c.timeout.Seconds()) {
		delete(c.challenges, ip)
		return false
	}

	// 从请求中获取答案
	answer := req.Header.Get("X-Challenge-Answer")
	if answer == "" {
		answer = req.URL.Query().Get("challenge_answer")
	}

	// 验证答案
	challenge := data.Token + string(data.Timestamp) + c.secret
	hash := sha256.Sum256([]byte(challenge))
	expectedAnswer := hex.EncodeToString(hash[:])

	if answer == expectedAnswer {
		delete(c.challenges, ip)
		return true
	}

	// 增加尝试次数
	data.Attempts++
	if data.Attempts > 3 {
		delete(c.challenges, ip)
	}

	return false
}

// generateChallengeScript 生成挑战脚本
func (c *Challenge) generateChallengeScript(token string, timestamp int64) string {
	return `
<script>
(function() {
    var token = "` + token + `";
    var timestamp = ` + string(timestamp) + `;
    var secret = ""; // 客户端不知道secret
    
    // 发送请求携带验证信息
    var xhr = new XMLHttpRequest();
    xhr.open('GET', window.location.href, true);
    xhr.setRequestHeader('X-Challenge-Token', token);
    xhr.setRequestHeader('X-Challenge-Timestamp', timestamp);
    xhr.send();
})();
</script>
`
}
```

---

## 三、可观测性增强扩展

### 3.1 架构设计

```
internal/monitoring/
├── telemetry/               # OpenTelemetry 集成
│   ├── tracer.go            # 分布式追踪
│   ├── metrics.go           # 指标收集
│   ├── exporter.go          # 导出器
│   └── middleware.go        # 中间件
├── dashboard/               # 仪表板
│   ├── handler.go           # HTTP处理器
│   └── templates/           # 模板
└── alerting/                # 告警
    ├── rules.go             # 告警规则
    ├── notifier.go          # 通知器
    └── webhook.go           # Webhook
```

### 3.2 核心代码实现

#### tracer.go - 分布式追踪

```go
package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// Tracer 追踪器
type Tracer struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
}

// TracerConfig 追踪器配置
type TracerConfig struct {
	ServiceName string
	Environment string
	JaegerURL   string
	SampleRate  float64
}

// NewTracer 创建追踪器
func NewTracer(config *TracerConfig) (*Tracer, error) {
	// 创建Jaeger导出器
	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(
		jaeger.WithEndpoint(config.JaegerURL),
	))
	if err != nil {
		return nil, err
	}

	// 创建资源
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(config.ServiceName),
		semconv.DeploymentEnvironmentKey.String(config.Environment),
	)

	// 创建追踪提供者
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(config.SampleRate)),
	)

	// 设置全局提供者
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Tracer{
		provider: provider,
		tracer:   provider.Tracer(config.ServiceName),
	}, nil
}

// StartSpan 开始追踪
func (t *Tracer) StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name)
}

// RecordError 记录错误
func (t *Tracer) RecordError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// Close 关闭追踪器
func (t *Tracer) Close() error {
	return t.provider.Shutdown(context.Background())
}
```

#### metrics.go - 指标收集

```go
package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
)

// Metrics 指标收集器
type Metrics struct {
	// 请求计数
	RequestTotal      metric.Int64Counter
	RequestDuration   metric.Float64Histogram
	RequestActive     metric.Int64UpDownCounter

	// 安全指标
	ThreatDetected    metric.Int64Counter
	ThreatBlocked     metric.Int64Counter
	RateLimitExceeded metric.Int64Counter

	// 渲染指标
	RenderTotal       metric.Int64Counter
	RenderDuration    metric.Float64Histogram
	RenderSuccess     metric.Int64Counter
	RenderFailed      metric.Int64Counter
	CacheHit          metric.Int64Counter
	CacheMiss         metric.Int64Counter

	// 系统指标
	GoroutineCount    metric.Int64ObservableGauge
	MemoryUsage       metric.Int64ObservableGauge
}

// NewMetrics 创建指标收集器
func NewMetrics() (*Metrics, error) {
	meter := global.MeterProvider().Meter("prerender-shield")

	m := &Metrics{}

	// 请求计数
	m.RequestTotal, _ = meter.Int64Counter(
		"request_total",
		metric.WithDescription("Total number of requests"),
		metric.WithUnit("1"),
	)

	m.RequestDuration, _ = meter.Float64Histogram(
		"request_duration_ms",
		metric.WithDescription("Request duration in milliseconds"),
		metric.WithUnit("ms"),
	)

	m.RequestActive, _ = meter.Int64UpDownCounter(
		"request_active",
		metric.WithDescription("Number of active requests"),
		metric.WithUnit("1"),
	)

	// 安全指标
	m.ThreatDetected, _ = meter.Int64Counter(
		"threat_detected_total",
		metric.WithDescription("Total number of threats detected"),
		metric.WithUnit("1"),
	)

	m.ThreatBlocked, _ = meter.Int64Counter(
		"threat_blocked_total",
		metric.WithDescription("Total number of threats blocked"),
		metric.WithUnit("1"),
	)

	// 渲染指标
	m.RenderTotal, _ = meter.Int64Counter(
		"render_total",
		metric.WithDescription("Total number of render operations"),
		metric.WithUnit("1"),
	)

	m.RenderDuration, _ = meter.Float64Histogram(
		"render_duration_ms",
		metric.WithDescription("Render duration in milliseconds"),
		metric.WithUnit("ms"),
	)

	m.CacheHit, _ = meter.Int64Counter(
		"cache_hit_total",
		metric.WithDescription("Total number of cache hits"),
		metric.WithUnit("1"),
	)

	m.CacheMiss, _ = meter.Int64Counter(
		"cache_miss_total",
		metric.WithDescription("Total number of cache misses"),
		metric.WithUnit("1"),
	)

	return m, nil
}

// RecordRequest 记录请求
func (m *Metrics) RecordRequest(ctx context.Context, method, path string, duration float64) {
	m.RequestTotal.Add(ctx, 1,
		attribute.String("method", method),
		attribute.String("path", path),
	)
	m.RequestDuration.Record(ctx, duration,
		attribute.String("method", method),
		attribute.String("path", path),
	)
}

// RecordThreat 记录威胁
func (m *Metrics) RecordThreat(ctx context.Context, threatType, severity string, blocked bool) {
	m.ThreatDetected.Add(ctx, 1,
		attribute.String("type", threatType),
		attribute.String("severity", severity),
	)
	if blocked {
		m.ThreatBlocked.Add(ctx, 1,
			attribute.String("type", threatType),
		)
	}
}

// RecordRender 记录渲染
func (m *Metrics) RecordRender(ctx context.Context, siteID string, duration float64, success, cacheHit bool) {
	m.RenderTotal.Add(ctx, 1,
		attribute.String("site_id", siteID),
	)
	m.RenderDuration.Record(ctx, duration,
		attribute.String("site_id", siteID),
	)
	if success {
		m.RenderSuccess.Add(ctx, 1)
	} else {
		m.RenderFailed.Add(ctx, 1)
	}
	if cacheHit {
		m.CacheHit.Add(ctx, 1)
	} else {
		m.CacheMiss.Add(ctx, 1)
	}
}
```

#### middleware.go - 追踪中间件

```go
package telemetry

import (
	"time"

	"github.com/gin-gonic/gin"
)

// TracingMiddleware 追踪中间件
func (t *Tracer) TracingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		ctx, span := t.StartSpan(c.Request.Context(), c.FullPath())
		defer span.End()

		// 设置追踪上下文
		c.Request = c.Request.WithContext(ctx)

		// 记录请求属性
		span.SetAttributes(
			attribute.String("http.method", c.Request.Method),
			attribute.String("http.url", c.Request.URL.String()),
			attribute.String("http.host", c.Request.Host),
			attribute.String("http.scheme", c.Request.URL.Scheme),
			attribute.String("http.remote_addr", c.ClientIP()),
			attribute.String("http.user_agent", c.Request.UserAgent()),
		)

		// 处理请求
		c.Next()

		// 记录响应属性
		span.SetAttributes(
			attribute.Int("http.status_code", c.Writer.Status()),
			attribute.Int("http.response_size", c.Writer.Size()),
		)
		span.SetStatus(httpStatusToCode(c.Writer.Status()), "")

		// 记录请求耗时
		duration := time.Since(start).Milliseconds()
		span.SetAttributes(
			attribute.Int64("http.duration_ms", duration),
		)
	}
}

// MetricsMiddleware 指标中间件
func (m *Metrics) MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 活跃请求+1
		m.RequestActive.Add(c.Request.Context(), 1)

		// 处理请求
		c.Next()

		// 活跃请求-1
		m.RequestActive.Add(c.Request.Context(), -1)

		// 记录请求
		duration := float64(time.Since(start).Milliseconds())
		m.RecordRequest(
			c.Request.Context(),
			c.Request.Method,
			c.FullPath(),
			duration,
		)
	}
}
```

---

## 四、渲染引擎性能优化扩展

### 4.1 架构设计

```
internal/prerender/
├── pool/                    # 渲染池管理
│   ├── pool.go              # Chromium实例池
│   ├── worker.go            # 工作进程
│   └── scheduler.go         # 调度器
├── cache/                   # 缓存优化
│   ├── tiered.go            # 多级缓存
│   ├── preheater.go         # 预热器
│   └── invalidator.go       # 缓存失效
└── optimizer/               # 渲染优化
    ├── lazy_loader.go       # 懒加载
    ├── resource_blocker.go  # 资源阻止
    └── memory_monitor.go    # 内存监控
```

### 4.2 核心代码实现

#### pool.go - Chromium 实例池

```go
package pool

import (
	"context"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// Worker Chromium工作进程
type Worker struct {
	ID           string
	ctx          context.Context
	cancel       context.CancelFunc
	allocatorCtx context.Context
	allocator    chromedp.ContextOption
	lastUsed     time.Time
	inUse        bool
	mu           sync.Mutex
}

// Pool Chromium实例池
type Pool struct {
	workers     []*Worker
	available   chan *Worker
	maxWorkers  int
	minWorkers  int
	idleTimeout time.Duration
	mu          sync.RWMutex
}

// PoolConfig 池配置
type PoolConfig struct {
	MinWorkers  int           // 最小工作进程数
	MaxWorkers  int           // 最大工作进程数
	IdleTimeout time.Duration // 空闲超时
	InitTimeout time.Duration // 初始化超时
}

// NewPool 创建实例池
func NewPool(config *PoolConfig) (*Pool, error) {
	pool := &Pool{
		workers:     make([]*Worker, 0, config.MaxWorkers),
		available:   make(chan *Worker, config.MaxWorkers),
		maxWorkers:  config.MaxWorkers,
		minWorkers:  config.MinWorkers,
		idleTimeout: config.IdleTimeout,
	}

	// 预热最小数量的工作进程
	for i := 0; i < config.MinWorkers; i++ {
		worker, err := pool.createWorker()
		if err != nil {
			pool.Close()
			return nil, err
		}
		pool.workers = append(pool.workers, worker)
		pool.available <- worker
	}

	// 启动清理协程
	go pool.cleanupWorker()

	return pool, nil
}

// createWorker 创建工作进程
func (p *Pool) createWorker() (*Worker, error) {
	// 创建分配器上下文
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-setuid-sandbox", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.WindowSize(1920, 1080),
	)

	allocatorCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)

	// 创建浏览器上下文
	ctx, _ := chromedp.NewContext(allocatorCtx)

	worker := &Worker{
		ID:           generateID(),
		ctx:          ctx,
		cancel:       cancel,
		allocatorCtx: allocatorCtx,
		lastUsed:     time.Now(),
	}

	return worker, nil
}

// Acquire 获取工作进程
func (p *Pool) Acquire(ctx context.Context) (*Worker, error) {
	select {
	case worker := <-p.available:
		worker.mu.Lock()
		worker.inUse = true
		worker.lastUsed = time.Now()
		worker.mu.Unlock()
		return worker, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// 没有可用的工作进程，尝试创建新的
		p.mu.Lock()
		if len(p.workers) < p.maxWorkers {
			worker, err := p.createWorker()
			if err != nil {
				p.mu.Unlock()
				return nil, err
			}
			p.workers = append(p.workers, worker)
			p.mu.Unlock()
			
			worker.mu.Lock()
			worker.inUse = true
			worker.lastUsed = time.Now()
			worker.mu.Unlock()
			return worker, nil
		}
		p.mu.Unlock()

		// 等待可用的工作进程
		select {
		case worker := <-p.available:
			worker.mu.Lock()
			worker.inUse = true
			worker.lastUsed = time.Now()
			worker.mu.Unlock()
			return worker, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// Release 释放工作进程
func (p *Pool) Release(worker *Worker) {
	worker.mu.Lock()
	worker.inUse = false
	worker.lastUsed = time.Now()
	worker.mu.Unlock()
	
	p.available <- worker
}

// cleanupWorker 清理空闲工作进程
func (p *Pool) cleanupWorker() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		p.mu.Lock()
		now := time.Now()
		activeWorkers := make([]*Worker, 0, len(p.workers))
		
		for _, worker := range p.workers {
			worker.mu.Lock()
			if !worker.inUse && 
			   now.Sub(worker.lastUsed) > p.idleTimeout && 
			   len(activeWorkers) > p.minWorkers {
				// 关闭空闲工作进程
				worker.cancel()
			} else {
				activeWorkers = append(activeWorkers, worker)
			}
			worker.mu.Unlock()
		}
		
		p.workers = activeWorkers
		p.mu.Unlock()
	}
}

// Close 关闭池
func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, worker := range p.workers {
		worker.cancel()
	}
	p.workers = nil
	close(p.available)
}
```

#### tiered.go - 多级缓存

```go
package cache

import (
	"context"
	"sync"
	"time"
)

// TieredCache 多级缓存
type TieredCache struct {
	l1         *MemoryCache  // 一级缓存（内存）
	l2         *RedisCache   // 二级缓存（Redis）
	l1TTL      time.Duration
	l2TTL      time.Duration
	mu         sync.RWMutex
}

// MemoryCache 内存缓存
type MemoryCache struct {
	data    map[string]*CacheItem
	mu      sync.RWMutex
	maxSize int
}

// CacheItem 缓存项
type CacheItem struct {
	Value     []byte
	ExpiresAt time.Time
}

// RedisCache Redis缓存
type RedisCache struct {
	client *redis.Client
}

// NewTieredCache 创建多级缓存
func NewTieredCache(l1Size int, l1TTL, l2TTL time.Duration, redisClient *redis.Client) *TieredCache {
	return &TieredCache{
		l1:    NewMemoryCache(l1Size),
		l2:    &RedisCache{client: redisClient},
		l1TTL: l1TTL,
		l2TTL: l2TTL,
	}
}

// Get 获取缓存
func (tc *TieredCache) Get(ctx context.Context, key string) ([]byte, bool) {
	// 先查L1
	if value, ok := tc.l1.Get(key); ok {
		return value, true
	}

	// 再查L2
	if value, ok := tc.l2.Get(ctx, key); ok {
		// 回填L1
		tc.l1.Set(key, value, tc.l1TTL)
		return value, true
	}

	return nil, false
}

// Set 设置缓存
func (tc *TieredCache) Set(ctx context.Context, key string, value []byte) error {
	// 同时写入L1和L2
	tc.l1.Set(key, value, tc.l1TTL)
	return tc.l2.Set(ctx, key, value, tc.l2TTL)
}

// Delete 删除缓存
func (tc *TieredCache) Delete(ctx context.Context, key string) error {
	tc.l1.Delete(key)
	return tc.l2.Delete(ctx, key)
}

// MemoryCache 方法
func NewMemoryCache(maxSize int) *MemoryCache {
	return &MemoryCache{
		data:    make(map[string]*CacheItem),
		maxSize: maxSize,
	}
}

func (mc *MemoryCache) Get(key string) ([]byte, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	item, exists := mc.data[key]
	if !exists || time.Now().After(item.ExpiresAt) {
		return nil, false
	}

	return item.Value, true
}

func (mc *MemoryCache) Set(key string, value []byte, ttl time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// LRU淘汰
	if len(mc.data) >= mc.maxSize {
		mc.evictLRU()
	}

	mc.data[key] = &CacheItem{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}
}

func (mc *MemoryCache) Delete(key string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	delete(mc.data, key)
}

func (mc *MemoryCache) evictLRU() {
	// 简单实现：删除最早过期的项
	var oldestKey string
	oldestTime := time.Now()

	for key, item := range mc.data {
		if item.ExpiresAt.Before(oldestTime) {
			oldestTime = item.ExpiresAt
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(mc.data, oldestKey)
	}
}

// RedisCache 方法
func (rc *RedisCache) Get(ctx context.Context, key string) ([]byte, bool) {
	result, err := rc.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}
	return result, true
}

func (rc *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return rc.client.Set(ctx, key, value, ttl).Err()
}

func (rc *RedisCache) Delete(ctx context.Context, key string) error {
	return rc.client.Del(ctx, key).Err()
}
```

---

## 五、配置文件扩展

### 5.1 新增配置项

```yaml
# configs/config.yml

# AI 威胁检测配置
ai_detector:
  enabled: true
  model_path: "./data/models/threat_detection"
  worker_pool: 4
  confidence_threshold: 0.85
  predict_timeout_ms: 50
  cache_size: 10000
  auto_update: true
  update_interval: 24h

# DDoS 防护配置
ddos_protection:
  enabled: true
  rate_limit:
    requests_per_second: 100
    burst_size: 200
    window: 1s
  ip_tracking:
    max_connections: 50
    block_duration: 10m
  challenge:
    enabled: true
    timeout: 30s
    algorithm: "sha256"

# 可观测性配置
observability:
  tracing:
    enabled: true
    exporter: "jaeger"
    jaeger_url: "http://localhost:14268/api/traces"
    sample_rate: 0.1
  metrics:
    enabled: true
    exporter: "prometheus"
    listen_address: ":9090"
  logging:
    format: "json"
    level: "info"
    output: "/var/log/prerender-shield/app.log"

# 渲染池配置
render_pool:
  min_workers: 2
  max_workers: 10
  idle_timeout: 5m
  init_timeout: 30s

# 多级缓存配置
cache:
  l1:
    enabled: true
    max_size: 10000
    ttl: 5m
  l2:
    enabled: true
    ttl: 1h
    redis_url: "localhost:6379"
```

---

## 六、实施优先级

### Phase 1（立即实施）

| 模块 | 工期 | 价值 |
|------|------|------|
| AI 威胁检测引擎 | 4周 | 技术壁垒提升 |
| 可观测性增强 | 2周 | 运维效率提升 |
| 渲染池优化 | 2周 | 性能提升 |

### Phase 2（3个月内）

| 模块 | 工期 | 价值 |
|------|------|------|
| DDoS 防护模块 | 3周 | 安全能力补全 |
| 多级缓存优化 | 2周 | 性能提升 |
| 文档完善 | 1周 | 用户体验 |

### Phase 3（6个月内）

| 模块 | 工期 | 价值 |
|------|------|------|
| 云服务版本 | 8周 | 商业化 |

---

**文档完成时间:** 2026-03-11  
**建议评审:** 技术团队评审后启动实施

---

# 第二部分：Phase 2 开发规划

> 以下内容合并自 PHASE2_DEVELOPMENT_PLAN.md


# Prerender Shield 第二阶段开发规划

**版本:** 2.0
**制定日期:** 2026-03-11
**规划周期:** 12 周 (Phase 2)
**架构师:** AI Assistant

---

## 一、现状分析

### 1.1 已完成功能 (Phase 1)

| 模块 | 状态 | 完成度 |
|------|------|--------|
| SSL 自动续签 | ✅ | 100% |
| AI 威胁检测 | ✅ | 90% |
| DDoS 防护 | ✅ | 85% |
| 可观测性 (Telemetry) | ✅ | 80% |
| 渲染池优化 | ✅ | 85% |
| 多级缓存 | ✅ | 90% |
| 仪表板系统 | ✅ | 70% |
| 告警系统 | ✅ | 75% |

### 1.2 核心架构现状

```
当前架构层次:
├── 接入层 (SSL/TLS, 请求解析)
├── 核心处理层 (防火墙引擎，渲染引擎，缓存管理)
├── 服务层 (配置中心，存储服务，消息队列)
└── 管理监控层 (Web 界面，API, 监控告警，日志)
```

---

## 二、产品差距分析 (Gap Analysis)

### 2.1 预渲染引擎缺失功能

| 功能 | 重要性 | 当前状态 | 差距说明 |
|------|--------|----------|----------|
| **增量渲染** | 🔴 P0 | ❌ 未实现 | 仅支持全量渲染，无法针对页面变化部分渲染 |
| **流式渲染** | 🔴 P0 | ❌ 未实现 | 无法边渲染边返回，首屏延迟高 |
| **智能等待策略** | 🔴 P0 | ⚠️ 部分实现 | 仅支持固定 waitUntil，无法智能判断内容加载完成 |
| **AB 测试支持** | 🟡 P1 | ❌ 未实现 | 无法支持多版本渲染对比 |
| **SEO 元数据优化** | 🟡 P1 | ⚠️ 部分实现 | 缺少动态 meta 标签优化 |
| **结构化数据生成** | 🟡 P1 | ❌ 未实现 | 无法自动生成 Schema.org 数据 |
| **多浏览器引擎** | 🟡 P1 | ❌ 未实现 | 仅支持 Chromium，缺少 WebKit/Firefox |
| **截图/PDF 生成** | 🟢 P2 | ❌ 未实现 | 无法生成页面快照 |
| **视觉回归测试** | 🟢 P2 | ❌ 未实现 | 无法检测 UI 变化 |

### 2.2 WAF 防火墙缺失功能

| 功能 | 重要性 | 当前状态 | 差距说明 |
|------|--------|----------|----------|
| **API 安全防护** | 🔴 P0 | ❌ 未实现 | 缺少 API 速率限制、Schema 验证 |
| **Bot 管理** | 🔴 P0 | ⚠️ 部分实现 | 仅基础爬虫检测，缺少高级 Bot 识别 |
| **CC 攻击防护** | 🔴 P0 | ⚠️ 部分实现 | DDoS 模块有基础功能，需增强 |
| **SQL 注入深度检测** | 🟡 P1 | ⚠️ 基础实现 | 基于规则，缺少语义分析 |
| **RCE 检测** | 🟡 P1 | ⚠️ 基础实现 | 需要增强命令注入检测 |
| **文件上传安全** | 🟡 P1 | ❌ 未实现 | 缺少文件类型/内容检测 |
| **敏感数据防泄漏** | 🟡 P1 | ⚠️ 基础实现 | 需增强 DLP 功能 |
| **地理位置封锁** | 🟢 P2 | ✅ 已实现 | GeoIP 检测已完成 |
| **会话安全** | 🟢 P2 | ❌ 未实现 | 缺少会话劫持检测 |
| **零信任访问** | 🟢 P2 | ❌ 未实现 | 缺少持续身份验证 |

### 2.3 AI 能力差距分析

| 功能 | 重要性 | 当前状态 | 差距说明 |
|------|--------|----------|----------|
| **实时日志分析** | 🔴 P0 | ❌ 未实现 | 日志仅存储，无实时分析 |
| **威胁情报库** | 🔴 P0 | ❌ 未实现 | 无外部情报集成 |
| **异常行为检测** | 🔴 P0 | ⚠️ 部分实现 | AI 检测器有基础模型，需在线学习 |
| **自动规则生成** | 🟡 P1 | ❌ 未实现 | 无法从日志自动学习生成规则 |
| **攻击溯源** | 🟡 P1 | ❌ 未实现 | 无法追踪攻击来源和路径 |
| **风险评分** | 🟡 P1 | ❌ 未实现 | 无用户/IP 风险画像 |
| **预测性防护** | 🟢 P2 | ❌ 未实现 | 无法预测潜在攻击 |

---

## 三、第二阶段架构设计

### 3.1 整体架构升级

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                                    智能安全预渲染平台 2.0                                │
├─────────────────────────────────────────────────────────────────────────────────────────┤
│
│  ┌─────────────────────────────────────────────────────────────────────────────────┐   │
│  │                              AI 智能层 (新增)                                    │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐        │   │
│  │  │ 日志分析引擎 │  │ 威胁情报中心 │  │ 行为分析引擎 │  │ 自动学习系统 │        │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘        │   │
│  └─────────────────────────────────────────────────────────────────────────────────┘   │
│                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────┐   │
│  │                              安全增强层 (增强)                                   │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐        │   │
│  │  │  API 安全网关  │  │  Bot 管理器   │  │ 零信任引擎   │  │ 数据防泄漏  │        │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘        │   │
│  └─────────────────────────────────────────────────────────────────────────────────┘   │
│                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────┐   │
│  │                              渲染增强层 (增强)                                   │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐        │   │
│  │  │ 流式渲染引擎 │  │ 增量渲染器   │  │ SEO 优化器    │  │ 智能等待器  │        │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘        │   │
│  └─────────────────────────────────────────────────────────────────────────────────┘   │
│                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────┐   │
│  │                              现有核心层 (保持)                                   │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐        │   │
│  │  │  WAF 引擎     │  │ 预渲染引擎   │  │ 缓存管理     │  │ SSL 管理      │        │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘        │   │
│  └─────────────────────────────────────────────────────────────────────────────────┘   │
│
└─────────────────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 新增核心模块

#### 3.2.1 AI 智能层

**1. 日志分析引擎 (`internal/ai/log_analyzer/`)**
```
功能:
- 实时日志流处理 (Apache Kafka / Pulsar 集成)
- 日志模式识别与聚类
- 异常检测与告警
- 日志压缩与归档

技术栈:
- 流处理：Apache Flink / Kafka Streams
- 模式识别：Isolation Forest, LSTM
- 存储：ClickHouse / Elasticsearch
```

**2. 威胁情报中心 (`internal/ai/intelligence/`)**
```
功能:
- 外部威胁情报集成 (AlienVault OTX, VirusTotal, MISP)
- IP 信誉评分
- 恶意域名/URL 库
- 攻击指纹库
- 情报共享与订阅

数据源:
- AlienVault OTX API
- VirusTotal API
- AbuseIPDB API
- 自研情报库
```

**3. 行为分析引擎 (`internal/ai/behavior/`)**
```
功能:
- 用户行为基线建立
- 异常行为检测 (UEBA)
- 会话分析
- 风险评分引擎

算法:
- 用户画像：协同过滤 + 图神经网络
- 异常检测：One-Class SVM, Autoencoder
- 时序分析：Prophet, ARIMA
```

**4. 自动学习系统 (`internal/ai/autolearn/`)**
```
功能:
- 自动规则生成
- 模型在线更新
- 反馈学习循环
- A/B 测试框架

流程:
攻击日志 → 特征提取 → 模式识别 → 规则生成 → 验证测试 → 部署上线
```

#### 3.2.2 安全增强层

**5. API 安全网关 (`internal/api/gateway/`)**
```
功能:
- API 速率限制 (令牌桶/漏桶)
- Schema 验证 (JSON Schema, OpenAPI)
- JWT/OAuth2 验证
- GraphQL 安全
- gRPC 安全

特性:
- 细粒度限流 (用户/IP/API 维度)
- 请求/响应验证
- 敏感数据脱敏
```

**6. Bot 管理器 (`internal/firewall/bot_manager/`)**
```
功能:
- Bot 分类 (搜索引擎/监控/恶意)
- 指纹识别 (TLS 指纹，JA3)
- 行为挑战 (JavaScript 挑战，Proof of Work)
- 好 Bot 管理 (Googlebot, Bingbot 验证)

检测技术:
- TLS 指纹识别
- HTTP/2 指纹
- TCP 栈指纹
- JavaScript 执行能力
```

**7. 零信任引擎 (`internal/auth/zero_trust/`)**
```
功能:
- 持续身份验证
- 设备指纹
- 上下文感知访问
- 最小权限控制

验证因素:
- 用户身份
- 设备健康
- 网络位置
- 行为特征
- 时间/地点异常
```

#### 3.2.3 渲染增强层

**8. 流式渲染引擎 (`internal/prerender/streaming/`)**
```
功能:
- 边渲染边返回
- 分块传输 (Chunked Transfer)
- 首屏优先渲染
- 关键 CSS 内联

技术:
- HTTP/2 Server Push
- Server-Sent Events (SSE)
- WebTransport
```

**9. 增量渲染器 (`internal/prerender/incremental/`)**
```
功能:
- 页面差异检测
- 局部内容更新
- 选择性重新渲染
- 版本对比

算法:
- DOM Diff
- 视觉差异检测
- 内容哈希对比
```

**10. SEO 优化器 (`internal/prerender/seo/`)**
```
功能:
- 动态 Meta 标签优化
- 结构化数据生成 (Schema.org, JSON-LD)
- Open Graph 标签
- Twitter Card
- 规范链接 (Canonical URL)
- Robots 管理
```

**11. 智能等待器 (`internal/prerender/smart_wait/`)**
```
功能:
- 网络空闲检测
- 关键元素可见
- XHR/Fetch 完成
- 自定义条件等待

策略:
- networkidle0 / networkidle2
- 元素选择器等待
- 时间智能预测
```

---

## 四、Phase 2 任务清单

### Phase 2A: AI 智能层 (6 周)

| ID | 任务 | 优先级 | 工时 | 依赖 |
|----|------|--------|------|------|
| AI-01 | 日志分析引擎 - 流处理框架 | P0 | 16h | 无 |
| AI-02 | 日志分析引擎 - 异常检测 | P0 | 12h | AI-01 |
| AI-03 | 威胁情报中心 - 外部 API 集成 | P0 | 8h | 无 |
| AI-04 | 威胁情报中心 - IP 信誉评分 | P0 | 8h | AI-03 |
| AI-05 | 行为分析引擎 - 用户基线 | P1 | 12h | 无 |
| AI-06 | 行为分析引擎 - UEBA | P1 | 16h | AI-05 |
| AI-07 | 自动学习系统 - 规则生成 | P1 | 16h | AI-01,AI-02 |
| AI-08 | 自动学习系统 - 在线学习 | P2 | 12h | AI-07 |

### Phase 2B: 安全增强层 (4 周)

| ID | 任务 | 优先级 | 工时 | 依赖 |
|----|------|--------|------|------|
| SEC-01 | API 安全网关 - 速率限制 | P0 | 12h | 无 |
| SEC-02 | API 安全网关 - Schema 验证 | P0 | 8h | SEC-01 |
| SEC-03 | Bot 管理器 - 指纹识别 | P0 | 16h | 无 |
| SEC-04 | Bot 管理器 - 行为挑战 | P0 | 12h | SEC-03 |
| SEC-05 | 零信任引擎 - 设备指纹 | P1 | 12h | 无 |
| SEC-06 | 零信任引擎 - 持续验证 | P1 | 12h | SEC-05 |

### Phase 2C: 渲染增强层 (4 周)

| ID | 任务 | 优先级 | 工时 | 依赖 |
|----|------|--------|------|------|
| REN-01 | 流式渲染引擎 - 分块传输 | P0 | 12h | 无 |
| REN-02 | 流式渲染引擎 - 首屏优先 | P0 | 16h | REN-01 |
| REN-03 | 增量渲染器 - DOM Diff | P1 | 16h | 无 |
| REN-04 | 增量渲染器 - 选择性渲染 | P1 | 12h | REN-03 |
| REN-05 | SEO 优化器 - Meta 标签 | P1 | 8h | 无 |
| REN-06 | SEO 优化器 - 结构化数据 | P1 | 8h | REN-05 |
| REN-07 | 智能等待器 - 网络检测 | P2 | 8h | 无 |
| REN-08 | 智能等待器 - 元素等待 | P2 | 8h | REN-07 |

---

## 五、AI 驱动的实时情报库设计

### 5.1 架构设计

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              实时情报库架构                                         │
├─────────────────────────────────────────────────────────────────────────────────────┤
│
│  数据摄入层                                                                         │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐                       │
│  │ 访问日志  │  │ 安全日志  │  │ 渲染日志  │  │ 外部情报  │                       │
│  └─────┬─────┘  └─────┬─────┘  └─────┬─────┘  └─────┬─────┘                       │
│        │              │              │              │                                │
│        └──────────────┴──────────────┴──────────────┘                                │
│                                   │                                                   │
│                          ┌────────▼────────┐                                         │
│                          │   Apache Kafka  │  消息队列                              │
│                          └────────┬────────┘                                         │
│                                   │                                                   │
│  流处理层                            │                                                │
│                          ┌────────▼────────┐                                         │
│                          │ Apache Flink /  │  实时计算                              │
│                          │  Kafka Streams  │                                         │
│                          └────────┬────────┘                                         │
│                                   │                                                   │
│        ┌──────────────────────────┼──────────────────────────┐                       │
│        │                          │                          │                       │
│  ┌─────▼──────┐          ┌───────▼───────┐         ┌────────▼────────┐              │
│  │ 实时告警   │          │  特征存储     │         │  模型推理       │              │
│  └────────────┘          └───────┬───────┘         └────────┬────────┘              │
│                                  │                          │                        │
│  存储层                          │                          │                        │
│        ┌─────────────────────────┼──────────────────────────┘                       │
│        │                         │                                                   │
│  ┌─────▼──────┐         ┌────────▼───────┐         ┌────────────────┐               │
│  │ ClickHouse │         │   Redis      │         │  特征数据库    │               │
│  │ 日志存储   │         │   缓存       │         │  (Feast)       │               │
│  └────────────┘         └────────────────┘         └────────────────┘               │
│
│  服务层                                                                             │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐                        │
│  │ 情报 API  │  │ 查询引擎  │  │ 订阅服务  │  │ 导出服务  │                        │
│  └───────────┘  └───────────┘  └───────────┘  └───────────┘                        │
│
└─────────────────────────────────────────────────────────────────────────────────────┘
```

### 5.2 数据模型

```go
// 情报条目
type IntelligenceEntry struct {
    ID          string            `json:"id"`
    Type        string            `json:"type"` // ip, domain, url, fingerprint
    Value       string            `json:"value"`
    ThreatLevel int               `json:"threat_level"` // 0-100
    Confidence  float64           `json:"confidence"`
    Category    string            `json:"category"` // scanner, botnet, spam, malware
    Source      string            `json:"source"`
    Tags        []string          `json:"tags"`
    FirstSeen   time.Time         `json:"first_seen"`
    LastSeen    time.Time         `json:"last_seen"`
    Occurrences int64             `json:"occurrences"`
    Metadata    map[string]interface{} `json:"metadata"`
}

// 用户行为基线
type UserBaseline struct {
    UserID        string                 `json:"user_id"`
    IPAddresses   []string              `json:"ip_addresses"`
    UserAgents    []string              `json:"user_agents"`
    TypicalHours  []int                 `json:"typical_hours"`
    TypicalPaths  []string              `json:"typical_paths"`
    AvgRPM        float64               `json:"avg_requests_per_minute"`
    DeviceFingerprint string            `json:"device_fingerprint"`
    LastUpdated   time.Time             `json:"last_updated"`
}

// 实时威胁事件
type ThreatEvent struct {
    ID            string                 `json:"id"`
    Timestamp     time.Time             `json:"timestamp"`
    SourceIP      string                `json:"source_ip"`
    EventType     string                `json:"event_type"`
    Severity      string                `json:"severity"`
    Description   string                `json:"description"`
    Evidence      []Evidence            `json:"evidence"`
    RiskScore     float64               `json:"risk_score"`
    RelatedEvents []string              `json:"related_events"`
}
```

### 5.3 实时更新流程

```
1. 日志采集 → Kafka Topic
   - 访问日志 (access_log)
   - 安全日志 (security_log)
   - 渲染日志 (render_log)

2. 流处理 (Flink/Streams)
   - 窗口聚合 (1分钟/5 分钟/1 小时)
   - 异常检测 (统计异常，模式异常)
   - 关联分析 (多日志源关联)

3. 特征提取
   - IP 维度：请求频率，路径分布，时间分布
   - 用户维度：行为模式，设备指纹
   - 会话维度：持续时间，交互深度

4. 模型推理
   - 加载在线模型
   - 实时评分
   - 结果写入 Redis

5. 告警触发
   - 超过阈值 → 触发告警
   - 更新威胁情报库
   - 通知 WAF 引擎
```

### 5.4 技术选型

| 组件 | 推荐方案 | 备选方案 |
|------|----------|----------|
| 消息队列 | Apache Kafka | Redpanda, Pulsar |
| 流处理 | Apache Flink | Kafka Streams, Spark |
| 日志存储 | ClickHouse | Elasticsearch |
| 缓存 | Redis Cluster | Dragonfly |
| 特征存储 | Feast (自建) | Redis + 自定义 |
| 模型服务 | ONNX Runtime | TensorFlow Serving |
| 时序数据 | Prometheus | VictoriaMetrics |

---

## 六、实施优先级

### 第一阶段 (Week 1-4): AI 基础 + 安全增强

```
Week 1-2:
├── AI-01 日志分析引擎 - 流处理框架
├── AI-03 威胁情报中心 - 外部 API 集成
└── SEC-01 API 安全网关 - 速率限制

Week 3-4:
├── AI-02 日志分析引擎 - 异常检测
├── AI-04 威胁情报中心 - IP 信誉评分
├── SEC-02 API 安全网关 - Schema 验证
└── SEC-03 Bot 管理器 - 指纹识别
```

### 第二阶段 (Week 5-8): AI 进阶 + 渲染增强

```
Week 5-6:
├── AI-05 行为分析引擎 - 用户基线
├── REN-01 流式渲染引擎 - 分块传输
└── REN-02 流式渲染引擎 - 首屏优先

Week 7-8:
├── AI-06 行为分析引擎 - UEBA
├── REN-03 增量渲染器 - DOM Diff
└── SEC-04 Bot 管理器 - 行为挑战
```

### 第三阶段 (Week 9-12): 完善优化

```
Week 9-10:
├── AI-07 自动学习系统 - 规则生成
├── REN-04 增量渲染器 - 选择性渲染
└── SEC-05 零信任引擎 - 设备指纹

Week 11-12:
├── AI-08 自动学习系统 - 在线学习
├── REN-05/06 SEO 优化器
├── REN-07/08 智能等待器
└── SEC-06 零信任引擎 - 持续验证
```

---

## 七、预期成果

### 7.1 性能指标

| 指标 | 当前 | 目标 | 提升 |
|------|------|------|------|
| 首屏渲染时间 | 2.5s | 0.8s | 68% |
| 缓存命中率 | 75% | 95% | 27% |
| 威胁检测准确率 | 85% | 98% | 15% |
| 误报率 | 5% | <1% | 80% |
| 异常检测延迟 | N/A | <100ms | - |

### 7.2 安全能力

| 能力 | 当前 | 目标 |
|------|------|------|
| OWASP Top 10 覆盖 | 90% | 100% |
| API 安全防护 | ❌ | ✅ |
| Bot 管理 | ⚠️ 基础 | ✅ 高级 |
| 零信任访问 | ❌ | ✅ |
| 威胁情报 | ❌ | ✅ 实时 |
| 自动响应 | ❌ | ✅ |

### 7.3 AI 能力

| 能力 | 当前 | 目标 |
|------|------|------|
| 日志分析 | ❌ | ✅ 实时 |
| 异常检测 | ⚠️ 离线 | ✅ 在线 |
| 自动学习 | ❌ | ✅ |
| 威胁情报 | ❌ | ✅ 集成 |
| 风险评分 | ❌ | ✅ |

---

## 八、风险与缓解

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|----------|
| 技术复杂度高 | 高 | 中 | 分阶段实施，优先 P0 功能 |
| 性能开销 | 中 | 中 | 性能基准测试，优化关键路径 |
| 数据隐私 | 高 | 低 | 数据脱敏，合规审查 |
| 模型准确性 | 中 | 中 | A/B 测试，人工审核 |
| 外部依赖 | 中 | 低 | 降级方案，本地缓存 |

---

## 九、下一步行动

1. **确认规划** - 评审本规划文档，确认优先级和范围
2. **环境准备** - 搭建开发/测试环境，安装依赖
3. **启动 Phase 2A** - 从 AI-01 日志分析引擎开始实施
4. **持续迭代** - 每两周评审进度，调整优先级

---

**文档状态:** 📋 待评审
**最后更新:** 2026-03-11


---

# 第四部分：未完成功能清单

> 以下内容合并自 未完成功能清单.md


