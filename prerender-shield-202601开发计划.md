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
| 插件系统 | 4周 | 生态扩展 |

---

**文档完成时间:** 2026-03-11  
**建议评审:** 技术团队评审后启动实施