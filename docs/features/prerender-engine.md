# 功能详细文档 — 预渲染引擎体系

---

## 1. 核心渲染引擎

### 结构

```
internal/prerender/
├── engine.go              # 渲染引擎 (主入口)
│   ├── Render(url, timeout) → HTML
│   ├── renderWithRetry()  # 指数退避重试
│   ├── renderOnce()       # 单次渲染
│   └── RenderWithGzip()   # Gzip 压缩输出
├── engine_manager.go      # 多站点引擎管理器
│   ├── NewEngineManager()
│   ├── GetEngine(siteID)
│   └── Close()
├── engine_test.go
└── engine_bench_test.go
```

### 数据流

```
调用: Render(url, timeout)
  │
  ├── browserPool.AcquireWithTimeout(timeout)
  │    └── 从实例池获取 Chromium 实例
  │
  ├── chromedp.Run(ctx, actions...)
  │    ├── chromedp.Navigate(url)     # 导航到页面
  │    ├── chromedp.WaitReady("body") # 等待页面就绪
  │    └── chromedp.OuterHTML("html") # 获取 HTML
  │
  ├── 成功 → []byte(html)
  │
  └── 失败 → 重试 (最多3次, 指数退避)
       └── 全部失败 → error
```

### 业务流

```
1. 调用方传入 URL + 超时时间
2. AcquireWithTimeout 从池获取实例
   - 池为空且未达上限 → 创建新实例
   - 池为空且已达上限 → 等待其他任务释放
   - 等待超时 → 返回错误
3. Chromium 导航到 URL
4. 等待 body 就绪
5. 获取完整 HTML
6. 释放实例回池
7. 返回 HTML / 错误
8. 失败时自动重试 (500ms → 1s → 2s)
```

---

## 2. 浏览器实例池

### 结构

```
internal/prerender/pool/
├── pool.go     # 实例池
│   ├── NewPool(config)
│   ├── AcquireWithTimeout(timeout) → Instance
│   ├── Release(instance)
│   └── Close()
├── worker.go   # 实例工作者
│   ├── worker.start()
│   └── worker.stop()
├── pool_test.go
├── pool_integration_test.go
└── worker_test.go
```

### 结构说明

```
Pool
├── config: Config {Min, Max, IdleTimeout, MaxUseCount, ...}
├── instances: []*Instance    # 所有实例
├── available: chan *Instance # 可用实例队列
├── mu: sync.RWMutex
└── wg: sync.WaitGroup

Instance
├── ID: string
├── AllocCtx / ChromeCtx     # Chromium 上下文
├── CreatedAt / LastUsedAt
├── UseCount / MaxUseCount
└── IsHealthy: bool
```

### 数据流

```
Acquire:
  → available 通道读取 (阻塞/超时)
  → 无可用实例且 < MaxInstances?
       ├── 是 → 创建新实例
       └── 否 → 等待直到超时
  → 返回 Instance

Release:
  → 实例状态检查
  → UseCount++ > MaxUseCount?
       ├── 是 → 关闭实例, 创建新实例
       └── 否 → 放回 available 通道
  → IdleTimeout 超时 → 关闭多余空闲实例

健康检查:
  → 每 HealthCheckInterval 执行
  → 检查实例是否可用
  → 不可用 → 关闭并替换
```

### 业务流

```
启动:
  → 创建 MinInstances (默认2) 个实例

运行:
  → 按需获取/释放实例
  → 高峰时扩容至 MaxInstances (默认10)
  → 空闲时缩容至 MinInstances
  → 单个实例使用 100 次后重建 (防内存泄漏)

关闭:
  → 停止所有实例
  → 清理 Chromium 进程
```

---

## 3. 渲染优先级队列

### 结构

```
internal/prerender/
├── queue.go      # 优先级队列
│   ├── NewRenderQueue(maxSize)
│   ├── Enqueue(task)
│   ├── Dequeue() → RenderTask
│   └── Len() → int
├── queue_test.go
└── persistent_queue.go  # Redis 持久化包装
    ├── NewPersistentQueue(redis, prefix)
    ├── Enqueue(task)
    └── Dequeue() → RenderTask
```

### 数据结构

```
RenderTask
├── ID: string
├── URL: string
├── SiteID: string
├── Priority: Priority {Low=1, Normal=5, High=8, VIP=10}
├── CreatedAt: time.Time
├── Timeout: time.Duration
├── UserAgent: string
└── Callback: chan<- RenderResult

PriorityQueue (heap.Interface)
├── Len() → int
├── Less(i,j) → bool  # 优先级高/创建早的优先
├── Push(x) / Pop() → RenderTask
└── Swap(i,j)
```

### 数据流

```
Enqueue:
  → 入 memQueue (heap)
  → 入 Redis (持久化备份)
  → Dequeue:
     → 从 memQueue 取出最高优先级任务
     → 删除 Redis 备份

崩溃恢复:
  → 启动时从 Redis 扫描未完成任务
  → 重新入队
```

### 业务流

```
1. 渲染请求到来
2. 创建 RenderTask (含优先级)
3. Enqueue:
   - 爬虫请求 → Priority.Normal
   - 管理手动触发 → Priority.High
   - 预先批量预热 → Priority.Low
   - VIP 客户 → Priority.VIP
4. 工作线程 Dequeue 获取任务
5. 按优先级处理 (VIP > High > Normal > Low)
6. 同优先级按创建时间 (FIFO)
7. 完成后回调通知结果
```

---

## 4. 爬虫识别

### 结构

```
internal/prerender/
├── crawler.go          # 爬虫检测 (渲染引擎用)
└── crawler_test.go

```

### 数据结构

```
CrawlerResult
├── IsCrawler: bool
├── IsAICrawler: bool
├── CrawlerName: string    # googlebot, gptbot 等
├── CrawlerType: string    # search, ai, social, tool
├── Confidence: float64    # 置信度
└── UserAgent: string
```

### 数据流

```
Request → CrawlerDetector
  ├── UA 匹配?
  │    ├── 已知爬虫库 → 返回 CrawlerResult
  │    └── 未知 → 行为分析
  │
  └── 行为分析
       ├── 请求频率
       ├── 路径模式
       ├── JS 执行能力
       └── Cookie 支持
       → 综合判定
```

### 业务流

```
1. 请求进入, 提取 User-Agent
2. 匹配内置爬虫库:
   - 搜索引擎: Googlebot, Bingbot, Baiduspider
   - AI 爬虫: GPTBot, ClaudeBot, PerplexityBot
   - 社交: Facebook, Twitter, LinkedIn
3. 匹配成功 → 标记类型 + 置信度
4. 匹配失败 → 行为分析
5. 判定结果决定后续处理:
   - 爬虫 → 预渲染
   - AI 爬虫 → 预渲染 + AEO 处理
   - 用户 → 直接代理转发
```

---

## 5. 缓存系统

### 结构

```
internal/cache/
└── manager.go    # 缓存管理器
    ├── Manager 接口
    │   ├── Set(siteID, key, value, ttl)
    │   ├── Get(siteID, key) → []byte
    │   ├── Delete(siteID, key)
    │   ├── Clear(siteID)
    │   └── GetStats(siteID)
    ├── manager struct (实现)
    │   ├── redisClient: RedisClientInterface
    │   ├── SetWithPriority()
    │   ├── GetCacheEntry()
    │   └── EvictLowPriority()
    └── RedisClientInterface
        ├── Get/Set/Del
        ├── Keys/Exists
        ├── HashSet/HashGet
        └── Incr/Expire/TTL
```

### Key 设计

```
缓存数据: cache:{site_id}:{key}
元数据:   cache:{site_id}:{key}:meta  (Hash)

meta Hash 字段:
  ├── created_at: 创建时间戳
  ├── expires_at: 过期时间戳
  ├── priority:   优先级 (1-5)
  ├── hit_count:  命中次数
  └── last_hit_at: 最后命中时间
```

### 数据流

```
写入 (WriteThrough):
  Set(siteID, key, value, ttl)
  → SET cache:{site}:{key} {value} EX {ttl}
  → HSET cache:{site}:{key}:meta created_at/expires_at/priority

读取 (ReadThrough):
  Get(siteID, key)
  → GET cache:{site}:{key}
  → 命中? → 返回
  → 未命中 → 渲染 → 写入缓存

淘汰:
  EvictLowPriority(siteID, count)
  → KEYS cache:{site}:*:meta
  → 逐条检查 priority
  → priority=1 或 2 → DEL

统计:
  GetStats(siteID)
  → KEYS cache:{site}:*
  → 统计 key 数量 (排除 :meta)
```

### 业务流

```
1. 渲染完成 → 写入 Redis
   Key: cache:{site}:{url_hash}
   TTL: 3600s (可配置)
   元数据: 优先级/创建时间

2. 爬虫请求 → 读 Redis
   命中 → 直接返回 (10ms)
   未命中 → Chromium 渲染 → 写入 → 返回

3. 空间不足 → 淘汰低优先级条目
   先淘汰 priority=1, 再 priority=2

4. TTL 到期 → Redis 自动删除
```

---

## 6. 预热系统

### 结构

```
internal/prerender/
├── preheat.go           # 预热管理
│   ├── CreatePreheatTask(siteID, urls)
│   ├── GetPreheatTaskStatus(taskID)
│   └── ListPreheatTasks(siteID)
├── preheat/             # 预热子模块
└── push/               # 搜索引擎推送
    └── manager.go
        ├── PushManager
        ├── PushToBaidu(urls)
        └── PushToBing(urls)
```

### 数据流

```
触发预热:
  → API: POST /preheat/trigger {siteId, urls}
  → 创建预热任务
  → 生成渲染任务 → 入队列
  → 并发渲染 (可配置并发数)
  → 写入缓存
  → 更新任务状态

定时预热:
  → Cron 表达式 (如 "0 0 * * *")
  → 解析 Sitemap
  → 批量渲染
  → 推送搜索引擎
```

### 业务流

```
1. 管理员配置 Sitemap URL + Cron 表达式
2. 定时任务触发:
   - 下载 sitemap.xml
   - 提取所有 URL
   - 生成预热任务 → 渲染队列
   - 批量渲染 → 更新缓存
3. 推送搜索引擎 (百度/必应):
   - 收集最新 URL
   - POST 到搜索引擎推送 API
   - 记录推送结果
4. 手动触发:
   - 管理员点击 "一键预热"
   - 同上流程
```

---

## 7. 并发控制

### 结构

```
internal/prerender/
├── concurrency_manager.go
│   ├── NewConcurrencyManager(min, max, initial)
│   ├── Acquire() → bool
│   ├── Release()
│   ├── RecordSuccess(duration)
│   └── RecordFailure(duration)
└── concurrency_manager_test.go
```

### 数据流

```
Acquire:
  → activeCount < currentLimit?
       ├── 是 → activeCount++, 返回 true
       └── 否 → waitingCount++, 返回 false

Release:
  → activeCount--
  → waitingCount > 0? → waitingCount--

动态调整 (每30s):
  → 计算成功率
  → 成功率高 + 延迟低 → 增加限制
  → 成功率低 + 延迟高 → 降低限制
```

---

## 8. 高级渲染功能

### 流式渲染 (Streaming)

```
结构: prerender/streaming/chunked.go, first_screen.go

流程:
  Chromium 渲染 → 首屏 HTML 先输出
  → 剩余内容分块输出
  → 浏览器逐步渲染

优势:
  TTFB 从 5s → 500ms
  爬虫不再因等待超时放弃
```

### DOM 差异更新 (Incremental)

```
结构: prerender/incremental/dom_diff.go, selective.go

流程:
  1. 首次渲染完整页面
  2. 后续请求:
     - 对比新旧 DOM 树
     - 仅传输变更部分
     - 合并到缓存

优势:
  第二次渲染提速 60%+
  减少 Chromium 计算量
```

### 渲染优化器 (Optimizer)

```
结构: prerender/optimizer/optimizer.go

功能:
  - 根据页面类型选择渲染策略
  - 静态页面绕过 Chromium 直接返回
  - 动态页面优化等待策略
```
