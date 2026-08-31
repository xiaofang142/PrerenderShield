# 功能详细文档 — WAF 防火墙体系

---

## 1. WAF 检测引擎

### 结构

检测引擎由两层组成:

**Layer 1: 检测器模块 (10 个独立模块)**

```
firewall/detectors/          # 独立检测器模块
├── injection.go             # 注入检测 (SQL/NoSQL/命令)
├── xss.go                   # XSS 检测 (反射/存储/DOM)
├── csrf.go                  # CSRF 检测 (Token/Origin)
├── deserialization.go       # 反序列化检测
├── sensitive_data.go        # 敏感数据检测
├── file_integrity.go        # 文件完整性检测
├── blacklist.go             # IP 黑白名单
├── rate_limit.go            # 速率限制
├── geoip.go                 # GeoIP 地理位置
├── owasp_top10.go           # OWASP CRS 规则集合
│
firewall/detectors/ddos/     # DDoS 检测 (IP跟踪/速率/黑名单)
firewall/detectors/ai/       # AI 行为检测 (模型/特征/检测器)

firewall/engine.go           # 防火墙引擎 (检测器链编排)
firewall/action.go           # 动作处理 (Allow/Block/Challenge)
firewall/detector_manager.go # 检测器管理器
```

**Layer 2: 规则引擎 (5 种规则类型, 内置)**

```
规则类型:
  ├── RuleTypeUserAgent  → UA 检测
  ├── RuleTypeHeader     → 请求头检测
  ├── RuleTypeMethod     → 请求方法检测
  ├── RuleTypePath       → URL 路径检测
  └── RuleTypeBehavior   → 爬虫行为检测
```

### 数据流

```
HTTP Request
  │
  ├──→ firewall.Engine.CheckRequest()
  │      ├── Layer 1: 10 检测器模块 (并发执行)
  │      │    ├── OWASP 6: injection / xss / csrf / deserialization / sensitive-data / ai
  │      │    └── Core 4:  geoip / rate_limit / file_integrity / blacklist
  │      │
  │      ├── Layer 2: 规则引擎 (串行匹配)
  │      │    ├── UA 规则 → 评分
  │      │    ├── 请求头规则 → 评分
  │      │    ├── 请求方法规则 → 评分
  │      │    ├── URL 路径规则 → 评分
  │      │    └── 行为规则 → 评分
  │      │
  │      ├── 综合评分 → 动作判定
  │      │    ├── Allow  (分数 < 阈值)
  │      │    ├── Block  (分数 ≥ 阈值)
  │      │    └── Challenge (可疑)
  │      │
  │      └── 结果缓存 (相同请求快速放行)
  │
  ├──→ middleware.WafMiddleware (Gin 中间件)
  │      ├── 请求拦截 → 403
  │      └── 攻击日志写入 Redis
  │
  └──→ security/waf (独立 WAF 规则引擎)
         ├── YAML 规则加载
         ├── 规则匹配
         └── 动作执行
```

### 业务流

```
用户配置:
  管理员通过 Web UI / API
  → 配置防火墙规则
  → 规则保存到 Redis/YAML
  → 引擎热加载 (无需重启)

请求处理:
  1. 请求到达 Site Server
  2. WAF 中间件捕获
  3. Engine.RunChecks() 执行全部检测器
  4. 检测器返回 Threat 评分
  5. 综合评分判定：
     - Allow: 继续后续处理
     - Block: 返回 403 页面 + 记录攻击日志
     - Challenge: 返回验证挑战
  6. 攻击日志写入 Redis (7天保留)
  7. (可选) 告警通知

规则管理:
  新增 → 验证 → 生效 (热加载)
  修改 → 实时更新
  删除 → 立即失效
```

---

## 2. 速率限制 (Rate Limit)

### 结构

```
security/ratelimit/
├── ratelimit.go   # 限流器实现
├── schema.go      # Schema 验证
├── ratelimit_test.go
└── schema_test.go

middleware/ratelimit.go  # Gin 中间件
```

### 数据流

```
Request → middleware.ratelimit
  ├── 提取限流维度 (IP/API Key/用户)
  ├── Redis INCR 计数
  ├── 检查是否超限
  │    ├── 未超限 → 继续
  │    └── 超限 → 429 Too Many Requests
  └── 记录限流日志
```

### 业务流

```
配置:
  → 时间窗口 (秒)
  → 最大请求数
  → 超限处理策略 (拒绝/延迟)
  → 维度 (IP/API Key)

执行:
  1. 每个请求到达, 提取维度
  2. Redis 计数器 +1
  3. 当前计数 > 阈值? → 阻断
  4. 窗口过期 → 计数器自动重置
```

---

## 4. IP 黑白名单

### 结构

```
firewall/detectors/blacklist.go
```

### 数据流

```
Request → 提取 Client IP
  ├── 检查 Redis: blacklist:{site_id}
  │    ├── 匹配 → Block
  │    └── 不匹配 → 继续
  ├── 检查 Redis: whitelist:{site_id}
  │    ├── 匹配 → Allow (跳过所有后续检测)
  │    └── 不匹配 → 继续常规检测
```

### 业务流

```
管理员添加 IP → API: POST /firewall/whitelist
  → Redis SADD whitelist:{site_id} {ip}
  → 实时生效

管理员添加 IP → API: POST /firewall/blacklist
  → Redis SADD blacklist:{site_id} {ip}
  → 实时生效
```

---

## 5. GeoIP 地理位置控制

### 结构

```
services/geoip.go   # GeoIP 解析服务
firewall/detectors/geoip.go  # WAF GeoIP 检测器
services/geoip_test.go
```

### 数据流

```
Request → GeoIP Detector
  ├── 提取 Client IP
  ├── GeoIPService.Lookup(ip)
  │    ├── 内存缓存命中 → 直接返回
  │    ├── HTTP API 查询 → 缓存结果
  │    └── 内网IP → 返回服务器位置
  ├── 匹配 BlockList?
  │    ├── 是 → Block
  │    └── 否 → Allow
  └── 日志记录附加国家/城市信息
```

### 业务流

```
配置:
  管理员设置允许/禁止的国家列表

执行:
  1. 请求到达
  2. GeoIP 解析客户端 IP
  3. 返回国家代码 (CN/US/JP 等)
  4. 匹配:
     - AllowList: 不在列表中则 Block
     - BlockList: 在列表中则 Block
  5. 阻断 → 返回 403
```

---

## 6. DDoS 检测

### 结构

```
firewall/detectors/ddos/
├── (检测器实现)
└── *_test.go
```

### 数据流

```
Request → DDoS Detector
  ├── 统计: 同 IP/路径 请求频率
  ├── 检查: 频率是否超过基线
  │    ├── 是 → 标记 DDoS 攻击
  │    │    ├── 记录日志
  │    │    ├── 可选: 触发告警
  │    │    └── Block
  │    └── 否 → Allow
```

### 业务流

```
1. 持续监控请求速率
2. 对每个 IP 建立基线
3. 偏离基线 3x+ 标准差 → 判定攻击
4. 自动阻断 + 告警
```

---

## 7. 文件完整性 (网页防篡改)

### 结构

```
firewall/detectors/file_integrity.go
```

### 数据流

```
定时 (或请求触发)
  → 计算静态文件哈希
  → 对比 Redis 存储的基准哈希
  → 不匹配?
       ├── 是 → 标记篡改 → 告警
       └── 否 → 正常

请求进入
  → 检查请求文件完整性
  → 异常? → Block
```

### 业务流

```
1. 站点部署时生成文件哈希基准
2. 定时检查 (可配置间隔)
3. 哈希不匹配 → 判定文件被篡改
4. 告警通知管理员
5. 可疑请求直接拦截
```

---

## 9. WAF 规则引擎 (Security/WAF)

### 结构

```
security/waf/
├── engine.go     # WAF 引擎
├── engine_test.go
├── action.go     # 动作处理
├── rules.go      # 规则管理
├── types/        # 类型定义
│   ├── action.go
│   └── threat.go
└── detectors/    # 检测器
    ├── xss.go
    ├── injection.go
    ├── csrf.go
    └── sensitive.go
    └── common.go
```

### 数据流

```
请求 → WAF 规则引擎
  ├── 加载规则 (YAML)
  ├── 规则匹配
  │    ├── 条件满足? → 执行动作
  │    └── 不满足 → 下一条规则
  ├── 动作:
  │    ├── ALLOW
  │    ├── BLOCK
  │    ├── LOG
  │    └── CUSTOM
  └── 日志记录
```

---

## 10. 安全检测器详解

### 10.1 注入检测

```
检测位置: URL参数 / POST Body / Headers / Cookies
检测方法:
  ├── 正则匹配攻击模式 (UNION/SELECT/OR 1=1 等)
  ├── SQL 语法分析
  ├── NoSQL 操作符检测 ($ne/$gt/$where 等)
  └── 命令注入字符检测 (|;&`$())
输出: Threat{Type: Injection, Severity: high/critical}
```

### 10.2 XSS 检测

```
检测位置: URL参数 / POST Body / Referer
检测方法:
  ├── 反射型: <script> onerror= alert() 等
  ├── 存储型: 同上, 检查 POST Body
  └── DOM型: URL Fragment / 输入点
输出: Threat{Type: XSS, Severity: high}
```

### 10.3 CSRF 检测

```
检测位置: 请求头
检测方法:
  ├── Origin 检查 (同源/跨域)
  ├── Referer 检查
  └── CSRF Token 验证
输出: Threat{Type: CSRF, Severity: medium}
```

### 10.4 反序列化检测

```
检测位置: POST Body / URL 参数
检测方法:
  ├── Java: 魔法字节 ac ed 00 05
  ├── PHP: 序列化格式检测
  └── Python: pickle 格式检测
输出: Threat{Type: Deserialization, Severity: critical}
```

### 10.5 敏感数据检测

```
检测位置: URL / POST Body / 响应
检测内容:
  ├── 身份证号 (18位, 含校验)
  ├── 手机号 (中国 1xx 开头)
  ├── 银行卡号 (16-19位)
  ├── 密码字段
  ├── API Key / Token
  └── 内网 IP 泄露
输出: Threat{Type: SensitiveData, Severity: medium}
```
