# Prerender Shield 国际化中小项目功能与商业模式论证

**日期:** 2026-06-16  
**适用范围:** 中小型网站、企业官网、SaaS 官网、文档站、小型电商展示站、轻量级管理后台  
**不适用范围:** 大型 CDN、云 WAF、跨区域边缘网络、大型多租户安全平台、企业级 SIEM/APM 平台

---

## 1. 产品应该如何

### 1.1 产品定位

Prerender Shield 应定位为：

> 面向中小项目的自托管 SEO 预渲染 + 基础 WAF + 多站点托管网关。

它的核心价值不是替代 Cloudflare、Akamai、Fastly 这类全球网络，也不是替代企业级 WAF，而是解决中小团队最常见的三个问题：

1. SPA / 前后端分离网站 SEO 收录差。
2. 小团队没有精力组合 Nginx、WAF、Prerender、Redis、SSL、监控等多套工具。
3. 商业 SaaS 预渲染和安全服务持续订阅成本较高，且数据和配置不完全自主。

### 1.2 产品原则

| 原则 | 说明 |
|---|---|
| 简单部署 | 一键安装、Docker、本机二进制三条路径即可，不把 Kubernetes 作为主路径 |
| 简单配置 | 控制台完成 80% 以上配置，YAML 作为高级配置入口 |
| 中小项目够用 | 重点覆盖 1-20 个站点、低到中等访问量、有限运维人力 |
| 国际化可用 | 至少保证中文、英文后台和官网可用，SEO 输出支持多语言站点 |
| 可解释 | 每一次预渲染、拦截、缓存命中、失败都应该能在控制台看到原因 |
| 可收费 | 1 个站点免费全功能，超过 1 个站点按站点授权收费，源码私有化部署单独收费 |

### 1.3 不做大型项目能力

以下能力不作为当前产品主线：

- 全球边缘节点、Anycast、CDN 调度。
- 大规模 DDoS 清洗。
- 大型企业多租户组织、复杂 RBAC、审批流。
- 海量日志湖、SIEM、长周期安全合规报表。
- 大型 SaaS 计费平台。
- Kubernetes Operator 作为默认部署形态。
- 企业级 Bot Management 全量替代方案。

这些能力可以在商业合作或私有定制中单独评估，但不应拖慢中小项目版本。

---

## 2. 当前项目已有功能

本节基于现有代码清单整理。

### 2.1 后端基础

| 模块 | 路径 | 已有能力 |
|---|---|---|
| 应用入口 | `cmd/api/main.go` | 启动参数、系统信号、调用 bootstrap |
| 启动编排 | `internal/bootstrap/` | 加载配置、初始化 Redis、启动 API、控制台、站点服务 |
| 依赖容器 | `internal/di/container.go` | 组装认证、WAF、缓存、预渲染、日志、监控、SSL、站点服务 |
| 配置系统 | `internal/config/` | YAML 配置结构、站点配置、SEO、SSL、WAF、缓存、监控配置 |
| Redis | `internal/redis/` | Redis 客户端、订阅、操作封装、断路器 |

### 2.2 多站点与请求处理

| 模块 | 路径 | 已有能力 |
|---|---|---|
| 站点服务器 | `internal/site-server/` | 按站点端口启动 HTTP/HTTPS 服务 |
| 站点处理器 | `internal/site-handler/` | 支持 `proxy`、`static`、`redirect` 三种模式 |
| 反向代理 | `internal/proxy/` | 上游代理基础能力 |
| 静态资源 | API controller + site handler | 静态文件上传、删除、解压、SPA fallback |

### 2.3 SEO 与预渲染

| 模块 | 路径 | 已有能力 |
|---|---|---|
| 预渲染引擎 | `internal/prerender/engine.go` | 基于 chromedp 渲染页面 |
| 浏览器池 | `internal/prerender/pool/` | Chromium 实例池、获取与释放 |
| 并发管理 | `internal/prerender/concurrency_manager.go` | 动态并发控制 |
| 预热任务 | `internal/prerender/preheat.go`、`internal/scheduler/` | 预热任务、定时任务 |
| 缓存 | `internal/cache/`、`internal/prerender/cache/` | Redis 缓存、多级缓存结构 |
| 搜索引擎推送 | `internal/prerender/push/` | 百度、Bing 推送配置与接口 |
| SEO 文件 | `internal/seo/sitemap.go`、`internal/seo/robots.go` | sitemap.xml、robots.txt 生成 |
| SEO 注入 | `internal/prerender/seo_injector.go` | Meta、结构化数据、LLM SEO 配置入口 |

### 2.4 安全能力

| 模块 | 路径 | 已有能力 |
|---|---|---|
| WAF 引擎 | `internal/firewall/` | 规则加载、检测器编排、动作处理 |
| WAF 中间件 | `internal/middleware/waf.go` | 请求进入站点处理前进行安全检查 |
| 检测器 | `internal/firewall/detectors/` | SQLi、XSS、XXE、GeoIP、CC、威胁情报等检测结构 |
| 黑白名单 | repository + WAF controller | IP 黑名单、白名单 |
| Rate Limit | `internal/middleware/ratelimit.go` | 基础限流能力 |
| 审计日志 | `internal/audit/` | 审计事件记录 |
| 零信任/机器人 | `internal/security/` | 设备、连续验证、Bot manager 原型 |

### 2.5 SSL 与运维

| 模块 | 路径 | 已有能力 |
|---|---|---|
| ACME 客户端 | `internal/ssl/acme_client.go` | Let's Encrypt 申请路径 |
| 自动续签 | `internal/ssl/auto_renew.go` | 续签检查和记录 |
| HTTP-01 | `internal/ssl/http_challenge.go` | HTTP challenge provider |
| DNS-01 | `internal/ssl/dns_challenge.go` | DNS provider 配置入口 |
| 站点 TLS | `internal/site-server/manager.go` | 按站点启用 HTTPS、HSTS、HTTP 跳转 |
| 构建部署 | `build.sh`、`start.sh`、`deploy.sh`、`install.sh`、`docker/` | 本机、脚本、Docker 部署路径 |

### 2.6 管理后台与官网

| 项目 | 技术栈 | 已有能力 |
|---|---|---|
| `web/` 管理后台 | React 18 + TypeScript + Ant Design | 登录、概览、站点、WAF、预渲染、预热、推送、监控、日志、SSL、系统设置 |
| `prerender-offcial-website` 官网 | Vue 3 + Vite | 首页、痛点、技术原理、竞品对比、功能、技术文档、安装指南、文章页 |

### 2.7 国际化现状

| 区域 | 现状 |
|---|---|
| 管理后台 | 已有 `zh`、`en`、`ja`、`ko` locale 文件；Ant Design locale 中还引用了 `ar`、`fr`、`ru`、`es` |
| 后端错误 | `internal/i18n/` 已有基础结构，站点请求中部分错误使用 `i18n.T` |
| 官网 | 主要仍是中文内容，尚未形成 `/zh`、`/en` 路由体系 |
| SEO | 已有 sitemap、robots、meta、LLM SEO 结构，但多语言 SEO、hreflang 尚未体系化 |

---

## 3. 需要完善的功能与模块

### 3.1 P0：先把“中小项目可上线”做扎实

| 模块 | 改进项 | 原因 | 验收标准 |
|---|---|---|---|
| 首次运行 | 增加安装/初始化向导 | 中小团队不应手写复杂 YAML | 首次访问控制台可完成管理员、Redis、默认站点配置 |
| 站点管理 | 增加站点健康检查与诊断 | 添加站点后要知道是否可用 | 显示 DNS、端口、上游、SSL、静态首页状态 |
| 预渲染 | 增加 URL 渲染测试工具 | SEO 问题需要可验证 | 输入 URL + crawler UA，显示 HTML、耗时、缓存状态、错误原因 |
| 缓存 | 增加缓存规则 UI | 控制成本和性能 | 可配置 TTL、忽略参数、排除路径、清理缓存 |
| WAF | 管理 API 限流 | 登录接口和管理 API 自身需要保护 | `/api/v1/auth/login` 和管理 API 支持限流 |
| SSL | 统一 ACME 与站点证书路径 | 证书申请后必须能被站点服务实际使用 | 控制台申请证书后站点 HTTPS 可直接加载真实证书 |
| 日志 | 增加失败原因分类 | 中小团队需要看得懂 | 渲染失败、WAF 拦截、SSL 失败均有分类和建议 |

### 3.2 P1：国际化产品化能力

| 模块 | 改进项 | 原因 | 验收标准 |
|---|---|---|---|
| 管理后台 i18n | 收敛首批语言到中文、英文 | 先保证质量，不盲目堆语言 | 所有菜单、按钮、表单、错误提示完整中英文 |
| 官网 i18n | 增加 `/zh`、`/en` | 面向国际用户必须有英文官网 | 首页、安装、功能、价格、文档均有英文版本 |
| SEO i18n | 支持 hreflang | 多语言站点需要正确被搜索引擎理解 | sitemap 或页面 head 可输出 hreflang |
| 时区 | 控制台支持时区设置 | 海外用户看日志不能只按服务器时间 | 日志、图表、证书过期时间按用户时区展示 |
| 国家地区 | GeoIP 国家名国际化 | 不能只显示中文国家名 | 中英文国家名可切换 |
| 错误响应 | 后端统一 error code | API 不应直接返回固定语言 message | 后端返回 code，前端按 locale 翻译 |

### 3.3 P2：商业版增强能力

| 模块 | 改进项 | 商业价值 |
|---|---|---|
| 一键诊断报告 | 输出 SEO、SSL、WAF、缓存综合报告 | 适合付费版和交付服务 |
| 定时巡检 | 每天自动检查站点可用性、证书、渲染结果 | 适合专业版 |
| 告警通知 | Webhook、邮件、飞书、钉钉、Slack | 中小团队刚需 |
| 商业授权 | 账号、账单、站点数授权、离线授权文件 | 支持按站点收费和源码私有化部署 |
| 自动升级 | 检查版本、下载更新包、回滚 | 减少服务成本 |
| 多搜索引擎策略 | Google、Bing、Baidu、Yandex crawler profile | 国际化 SEO 亮点 |

### 3.4 不建议优先完善的模块

| 模块 | 暂缓原因 |
|---|---|
| 企业级 RBAC | 当前单管理员/少量成员即可，复杂权限后置 |
| 大型 DDoS | 自托管中小项目无法低成本承担清洗能力 |
| 大规模 WebSocket 实时推送 | 先用轮询或轻量刷新即可 |
| AI WAF | 当前更应强化规则、日志、可解释性，而不是包装 AI |
| 大规模多租户计费 | 本项目优先卖软件授权和部署服务，不急着做 SaaS 平台 |

---

## 4. 持续深入检查论证机制

### 4.1 持续检查目标

持续检查不是单纯找 bug，而是持续回答四个问题：

1. 代码里声称有的功能是否真的可用。
2. 控制台里的开关是否能影响真实运行链路。
3. 文档中的卖点是否有对应代码和测试。
4. 功能是否符合中小项目场景，是否过度设计。

### 4.2 每轮检查清单

| 检查项 | 检查方式 | 产物 |
|---|---|---|
| 功能真实性 | 从 API、配置、运行链路追到最终执行代码 | 真功能/假功能/半成品标注 |
| 国际化完整性 | 扫描硬编码中文、英文、日期、国家名、错误提示 | i18n 缺口列表 |
| 中小项目适用性 | 检查是否需要复杂依赖、复杂部署、复杂运维 | 简化建议 |
| 商业化可卖性 | 检查是否能形成免费版/专业版/企业版差异 | 收费功能矩阵 |
| 测试可信度 | 核对测试是否覆盖真实路径 | 测试补齐计划 |
| 文档一致性 | README、官网、配置样例、控制台功能是否一致 | 文档修订清单 |

### 4.3 建议审计节奏

| 周期 | 动作 |
|---|---|
| 每周 | 检查一个核心模块：预渲染、WAF、SSL、SEO、站点管理轮换 |
| 每两周 | 更新一次未完成功能清单和商业功能矩阵 |
| 每月 | 做一次端到端验收：新增站点、启用 SSL、模拟爬虫、触发 WAF、查看日志 |
| 每个版本发布前 | 对 README、官网、配置样例、后台菜单做一致性检查 |

### 4.4 现阶段优先核验路径

1. SSL 控制台申请证书后，站点 HTTPS 是否真的使用该证书。
2. WAF 开关、CC 防护、威胁情报配置是否进入真实请求链路。
3. 预渲染缓存是否按站点隔离，是否可清理，是否能解释命中/未命中原因。
4. sitemap、robots 生成是否按站点和语言输出，而不是只写默认目录。
5. 管理后台中英文是否完整，新增 SEO/WAF/SSL 字段是否漏翻译。

---

## 5. 国际化功能清单

### 5.1 后台国际化

| 功能 | 免费版 | 专业版 | 说明 |
|---|---:|---:|---|
| 中文后台 | 是 | 是 | 默认支持 |
| 英文后台 | 是 | 是 | 国际化最低要求 |
| 日文/韩文 | 否 | 是 | 第二阶段支持 |
| 时区显示 | 是 | 是 | 日志、监控、证书时间 |
| 多语言错误提示 | 是 | 是 | 后端 code + 前端翻译 |
| RTL 语言 | 否 | 否 | 暂不支持阿拉伯语等 RTL，避免成本过高 |

### 5.2 SEO 国际化

| 功能 | 优先级 | 说明 |
|---|---|---|
| 多语言 sitemap | P1 | 支持 `/zh/`、`/en/`、`/ja/` 等 URL |
| hreflang | P1 | 防止多语言页面互相竞争 |
| 多语言 title/description | P1 | 控制台按语言维护 SEO 字段 |
| 多搜索引擎 crawler profile | P1 | Googlebot、Bingbot、Baiduspider、YandexBot |
| 地区化推送策略 | P2 | 中国站偏百度，海外站偏 Google/Bing |
| LLM SEO 翻译/改写 | P2 | 商业版增强，不作为核心依赖 |

### 5.3 官网国际化

官网至少需要以下页面具备中英文：

- 首页。
- 功能特性。
- 安装指南。
- 价格页。
- 技术原理。
- 常见问题。
- 与 Prerender.io / Rendertron / Cloudflare 组合方案的对比。

---

## 6. 商务模式论证

### 6.1 市场参照

公开市场信息显示，Prerender.io 面向 JavaScript SEO 提供托管预渲染服务，其 Starter 试用为 30 天，试用包含 25,000 renders，试用后 Starter 为 $49/month；其文档说明 2025-10-15 已停止 1,000 renders/month 的 Free plan。[Prerender Pricing](https://prerender.io/pricing/) [Changes to Prerender pricing](https://docs.prerender.io/docs/changes-to-prerender-pricing)

Prerender.io 的计费核心是 render 次数：每次启动 headless browser 处理页面会计为一次 render，缓存命中不计费；移动端优化渲染默认可能让同一页面产生 desktop 与 mobile 两次 render。[How does Prerender.io count renders?](https://docs.prerender.io/docs/what-you-pay-for)

Cloudflare WAF 官方文档说明 WAF 可在所有计划中使用，并提供自动漏洞防护和自定义规则能力；其 Rate limiting rules 用于保护登录接口、限制 API 调用等滥用场景。[Cloudflare WAF docs](https://developers.cloudflare.com/waf/) [Cloudflare Rate limiting rules](https://developers.cloudflare.com/waf/rate-limiting-rules/)

第三方价格整理通常把 Cloudflare Free / Pro / Business / Enterprise 作为主要分层，并提示 Pro、Business、Enterprise 在价格和能力上逐级提升；实际收费应以 Cloudflare 官方价格页为准。[Cloudflare Pricing Explained](https://www.spendbase.co/blog/cost-optimization/cloudflare-pricing-explained-free-pro-business-and-enterprise-plans/) [Cloudflare Pricing](https://www.cloudflare.com/plans/)

### 6.2 本项目差异化

| 维度 | 托管预渲染服务 | 云 WAF / CDN | Prerender Shield |
|---|---|---|---|
| 部署 | 云端托管 | 云端托管 | 自托管 |
| 数据控制 | 页面内容经过第三方 | 流量经过第三方 | 数据留在自有服务器 |
| 计费 | 常按 render / 套餐 | 常按域名 / 套餐 / 企业合同 | 可按授权、站点数、服务收费 |
| SEO | 强 | 弱或非重点 | 强 |
| WAF | 弱 | 强 | 中小项目够用 |
| 适合对象 | SEO 问题明显的网站 | 需要全球网络和安全能力的网站 | 想低成本自托管的一体化中小项目 |

结论：本项目不应直接和 Cloudflare 正面竞争，而应卖“自托管的一体化、可控、低成本、懂中文和国际化 SEO”的价值。

### 6.3 收费方式建议

只保留一种清晰策略，避免用户理解成本：

| 类型 | 价格 | 功能边界 | 适用对象 |
|---|---:|---|---|
| Free | $0 | 1 个站点，全部功能开放 | 个人开发者、小公司单官网、试用用户 |
| Additional Site | $99 / 站点 / 年 | 每新增 1 个站点增加 1 份授权，全部功能开放 | 建站公司、多官网团队、SaaS 多落地页团队 |
| Private Source Deployment | $9999 | 源码私有化部署交付软件费用，可约定不限站点或较高站点数 | 需要源码、内网部署、二次开发、私有化交付的客户 |

中国大陆用户可以展示人民币参考价，但内部仍以美元锚定，便于国际化：

| 类型 | 人民币参考展示 |
|---|---:|
| Additional Site | 约 ¥699 / 站点 / 年 |
| Private Source Deployment | 约 ¥69,999 |

首付款策略：

| 类型 | 收款建议 |
|---|---|
| Additional Site | 金额低，建议一次性全款 |
| Private Source Deployment | 50% 首付款 + 50% 验收款 |

### 6.4 推荐商业模式

采用简单、足够有吸引力的站点授权模式：

1. **1 个站点永久免费，全部功能开放**：不阉割 WAF、预渲染、SSL、SEO、日志能力。
2. **超过 1 个站点后按站点数量收费**：99 美元 / 站点 / 年。
3. **源码私有化部署交付**：9999 美元软件费用，适合建站公司、小型集团、内网客户和二次开发客户。
4. **中国大陆用户可用人民币等值收款**：官网展示美元锚点，实际可提供人民币支付和对公转账。

推荐首发价格：

| 产品 | 首发价 | 原因 |
|---|---:|---|
| Free | $0 | 1 个站点，全功能，最大化冷启动传播 |
| Additional Site | $99 / 站点 / 年 | 足够便宜，建站公司和中小企业容易决策 |
| Private Source Deployment | $9999 | 源码私有化部署交付，仅收软件费用，便于快速成交 |
| 部署协助 | 可选服务费 | 软件费用之外，按实际工作量报价 |

### 6.5 免费版与付费版功能矩阵

| 功能 | Free | Additional Sites | Private Source Deployment |
|---|---:|---:|---:|
| 站点数量 | 1 个 | 按购买数量 | 授权约定，可不限 |
| 全部功能 | 是 | 是 | 是 |
| 单机部署 | 是 | 是 | 是 |
| Docker 部署 | 是 | 是 | 是 |
| 基础预渲染 | 是 | 是 | 是 |
| Redis 缓存 | 是 | 是 | 是 |
| sitemap / robots | 是 | 是 | 是 |
| 多语言 SEO | 是 | 是 | 是 |
| hreflang | 是 | 是 | 是 |
| WAF / CC 防护 | 是 | 是 | 是 |
| 威胁情报 | 是 | 是 | 是 |
| 告警通知 | 是 | 是 | 是 |
| 自动诊断报告 | 是 | 是 | 是 |
| 授权方式 | 默认免费授权 | 在线 license | 离线 license / 源码交付 |
| 源码交付 | 否 | 否 | 是 |
| 技术支持 | 社区 | 基础支持 | 交付支持 |

### 6.6 适用客户

| 客户类型 | 适用性 | 典型诉求 |
|---|---|---|
| 企业官网 | 高 | Vue/React 官网 SEO、HTTPS、基础防护 |
| SaaS 官网 | 高 | Google/Bing 收录、落地页预渲染、多语言 SEO |
| 文档站 | 高 | 静态站托管、robots、sitemap、访问日志 |
| 小型电商展示站 | 中高 | 商品页 SEO、缓存、基础防刷 |
| 建站公司 | 高 | 多客户站点统一管理 |
| 政企内网站点 | 中 | 自托管、数据可控、内网部署 |
| 高流量大型平台 | 低 | 应选择 CDN/WAF/专用渲染集群 |
| 强合规金融/大型电商 | 低 | 需要企业安全平台和专业清洗能力 |

---

## 7. 阶段路线图

### Phase 1：可信可用版

目标：让中小项目能放心上线。

- 统一 SSL 真证书链路。
- 管理 API 限流。
- 预渲染测试工具。
- 站点健康检查。
- 缓存规则 UI。
- 中英文后台补齐。
- README 和官网功能描述与真实代码一致。

### Phase 2：国际化专业版

目标：形成按站点收费的商业闭环。

- 官网 `/zh`、`/en`。
- 多语言 SEO、hreflang。
- Google/Bing/Baidu/Yandex crawler profile。
- 告警通知。
- 自动诊断报告。
- 备份恢复。
- 用户注册、账单、站点数授权、license key。

### Phase 3：商业交付版

目标：支持代理商、小型集团和私有化客户。

- 离线授权。
- 自动升级和回滚。
- 部署巡检工具。
- 商业支持文档。
- 价格页、购买页、授权说明、首付款和私有化交付说明。

---

## 8. 文档更新建议

后续应同步更新以下文档：

| 文档 | 更新内容 |
|---|---|
| `README.md` | 收敛卖点，明确中小项目定位和不做大型平台 |
| `configs/config.example.yml` | 增加国际化 SEO、时区、crawler profile 示例 |
| 官网首页 | 增加英文版本和中小项目价值主张 |
| 官网安装页 | 区分国内服务器、海外服务器、Docker、本地开发 |
| 官网价格页 | 增加 Free / Additional Sites / Private Source Deployment |
| `未完成功能清单.md` | 按 P0/P1/P2 和商业价值重新排序 |
| `DEVELOPMENT_PLAN.md` | 用 Phase 1-3 替换过度大型化任务 |

---

## 9. 最终结论

Prerender Shield 最合理的方向不是做“大而全”的安全云平台，而是成为：

> 中小项目能自己部署、自己掌控、低成本解决 SEO 预渲染和基础安全防护的一体化工具。

商业化也不应从复杂 SaaS 计费开始，而应先卖：

1. 1 个站点免费全功能，降低试用门槛。
2. 超过 1 个站点按站点数量收费。
3. 99 美元 / 站点 / 年，足够便宜，足够容易决策。
4. 9999 美元源码私有化部署交付，覆盖高价值客户。
5. SEO / SSL / WAF 配置交付服务和后续维护支持。

只要把“真实可用、配置简单、国际化 SEO、可解释诊断”做好，本项目就能避开大型云厂商的正面竞争，在中小企业和建站服务市场形成清晰价值。


---

# 附录：中国大陆商业化上线方案

> 以下内容合并自 CHINA_COMMERCIAL_LAUNCH_PLAN.md


# Prerender Shield 中国大陆商业化上线方案

**日期:** 2026-06-16  
**目标:** 以中国大陆开发者身份，用最低复杂度打通收款、发行、安装、注册、账单、授权、私有化交付和冷启动。  
**核心价格:** 1 个站点免费全功能；超过 1 个站点，99 美元 / 站点 / 年；源码私有化部署交付，9999 美元软件费用。

---

## 1. 首付款与收款问题

### 1.1 收费原则

不要一开始做复杂 SaaS 订阅平台。第一阶段只需要支持三类交易：

| 交易类型 | 建议收款方式 | 交付方式 |
|---|---|---|
| 单站点免费 | 无需支付 | 注册账号后自动获得 1 个免费站点授权 |
| 追加站点授权 | 在线支付或人工收款 | 支付后后台增加授权站点数 |
| 源码私有化部署 | 30%-50% 首付款 + 验收尾款 | 合同、发票、源码包、部署服务 |

### 1.2 首付款建议

#### 追加站点授权

金额较低，建议直接全款：

- 价格：99 美元 / 站点 / 年。
- 中国大陆用户可展示人民币折算价，例如：¥699 / 站点 / 年。
- 支付成功后立即开通授权。
- 不建议做首付款，避免流程复杂。

#### 源码私有化部署

金额较高，建议采用阶段付款：

| 阶段 | 比例 | 触发条件 |
|---|---:|---|
| 首付款 | 50% | 合同签署后支付，开始交付 |
| 验收款 | 50% | 源码交付、部署文档、授权文件、一次远程部署完成 |

也可以给保守客户：

| 阶段 | 比例 |
|---|---:|
| 首付款 | 30% |
| 部署完成 | 40% |
| 验收通过 | 30% |

推荐默认采用 **50% + 50%**，简单、清楚、现金流好。

### 1.3 支付通道建议

| 阶段 | 支付方案 | 原因 |
|---|---|---|
| MVP | 微信 / 支付宝转账 + 人工开授权 | 最快上线，不等待支付资质 |
| 第一版商业化 | 微信支付商户号 + 支付宝电脑网站支付 | 覆盖中国大陆主流用户 |
| 海外用户 | Stripe / Paddle / Lemon Squeezy | 支持美元信用卡和国际账单 |
| 私有化大单 | 对公转账 | 便于合同、发票、验收 |

微信支付和支付宝网站支付通常需要企业主体、商户资料、网站或应用信息；正式上线前应准备营业执照、备案域名、客服联系方式、隐私政策、用户协议和退款政策。

### 1.4 发票与税务

面向中国大陆企业客户，建议：

- 个人开发阶段：先验证市场，不主动承诺专票。
- 公司化后：提供普通发票或专票。
- 私有化 9999 美元订单：建议必须走公司主体合同。
- 站点授权 99 美元 / 年：可先以人民币收款，后续再统一开票。

---

## 2. 项目冷启动详细建议

### 2.1 冷启动定位

不要一开始面向“大企业安全平台”。最容易成交的是：

1. Vue / React 官网 SEO 不收录的小公司。
2. 外贸官网需要 Google 收录的小团队。
3. 建站公司手里有多个客户站点。
4. SaaS 官网、文档站、落地页项目。
5. 不想把页面内容交给海外预渲染 SaaS 的团队。

### 2.2 冷启动卖点

首页和销售话术必须反复强调：

- **1 个站点永久免费，全部功能可用。**
- **超过 1 个站点才收费，99 美元 / 站点 / 年。**
- **自托管，数据留在自己的服务器。**
- **一个二进制解决 SEO 预渲染 + 基础 WAF + SSL + 日志。**
- **源码私有化交付仅 9999 美元。**

### 2.3 冷启动渠道

| 渠道 | 动作 | 目标 |
|---|---|---|
| GitHub / Gitee | README 加价格和免费策略 | 获取开发者信任 |
| 官网 | 增加价格页、案例页、安装页 | 让用户看完能下载试用 |
| 掘金 / 知乎 / CSDN | 写 SPA SEO 实战文章 | 获取中文开发者流量 |
| V2EX / SegmentFault | 发布开源项目和问题解决帖 | 获取早期反馈 |
| 小红书 / B站 | 用“Vue 官网不收录怎么办”做短内容 | 获取非纯开发流量 |
| 建站公司 | 直接私信或邮件合作 | 获取多站点付费用户 |

### 2.4 冷启动内容选题

优先写这些：

1. Vue / React 单页应用为什么 Google 收录差。
2. 不用 Nuxt / Next，怎么给现有 SPA 做 SEO。
3. Prerender.io 太贵？自托管预渲染方案。
4. 企业官网如何同时做 SEO、HTTPS、基础防护。
5. 从 0 部署 Prerender Shield：10 分钟让爬虫看到 HTML。
6. 建站公司如何统一管理多个客户官网 SEO。

### 2.5 冷启动转化路径

用户路径必须短：

1. 访问官网。
2. 看到“1 个站点永久免费全功能”。
3. 复制安装命令。
4. 注册平台账号。
5. 自动获得 1 个站点授权。
6. 添加第 2 个站点时提示购买 99 美元 / 年授权。

---

## 3. 编译、构建、发行、安装、注册、账单、授权闭环

### 3.1 总体闭环

```
开发者提交代码
  → CI 构建多平台二进制和前端 dist
  → 生成 release 包
  → 官网展示版本和安装命令
  → 用户平台注册账号
  → 获得免费 1 站点授权
  → 安装脚本下载 release 包
  → 本地实例绑定账号或导入授权文件
  → 控制台添加站点
  → 超过 1 个站点触发购买
  → 支付成功后平台更新授权
  → 本地实例同步授权
```

### 3.2 需要建设的平台模块

| 模块 | 必要性 | 说明 |
|---|---|---|
| 用户账号 | P0 | 邮箱/手机号注册、登录、找回密码 |
| 组织/个人账户 | P0 | 账单归属，后续支持公司客户 |
| 授权记录 | P0 | 记录 max_sites、到期时间、plan |
| 账单订单 | P0 | 记录购买站点数、金额、支付状态 |
| 支付回调 | P0 | 支付成功后自动开通 |
| 授权 API | P0 | 本地实例按 license key 拉取授权 |
| 离线授权文件 | P1 | 私有化部署和内网客户使用 |
| 发票信息 | P1 | 中国大陆企业客户需要 |
| 设备/实例绑定 | P1 | 限制授权滥用，但不要过度打扰用户 |

### 3.3 本地软件授权逻辑

本地实例需要支持两种方式：

| 授权方式 | 场景 | 逻辑 |
|---|---|---|
| 在线授权 | 普通用户 | 输入 license key，定期向平台同步 max_sites 和 expires_at |
| 离线授权 | 私有化/内网 | 上传签名授权文件，本地验证签名和到期时间 |

当前代码已加入第一步商业策略：

- 默认 `commercial.max_sites: 1`。
- `commercial.site_price_usd_per_year: 99`。
- `commercial.private_deploy_price_usd: 9999`。
- 添加第 2 个站点时返回 402 Payment Required。

下一步应增加：

```yaml
commercial:
  license_key: ""
  license_server_url: "https://account.prerendershield.com"
  max_sites: 1
  plan: "free"
```

### 3.4 平台端最小数据表

第一阶段只需要这些表：

| 表 | 字段 |
|---|---|
| users | id, email, phone, password_hash, created_at |
| accounts | id, owner_user_id, name, country, created_at |
| licenses | id, account_id, license_key_hash, plan, max_sites, expires_at, status |
| orders | id, account_id, license_id, amount, currency, site_count, status, payment_provider |
| payments | id, order_id, provider, provider_trade_no, paid_at, raw_payload |
| instances | id, license_id, machine_fingerprint, version, last_seen_at |

### 3.5 发行包建议

| 包 | 用途 |
|---|---|
| `prerender-shield-linux-amd64.tar.gz` | 主力 Linux 服务器 |
| `prerender-shield-linux-arm64.tar.gz` | ARM 云服务器 |
| `prerender-shield-darwin-arm64.tar.gz` | macOS 本地测试 |
| `docker-compose.yml` | Docker 用户 |
| `install.sh` | 一键安装入口 |

发行包应包含：

- 后端二进制。
- 管理后台 `web/dist`。
- `configs/config.example.yml`。
- `start.sh`。
- `LICENSE`。
- `README`。
- 版本号和 checksum。

### 3.6 用户安装体验

安装脚本应该问最少的问题：

1. 选择安装目录，默认 `/opt/prerender-shield`。
2. 是否连接平台账号。
3. 输入 license key，可跳过。
4. 自动生成配置。
5. 自动启动服务。
6. 输出控制台地址。

免费用户不应该被注册流程卡住：

- 可先离线使用 1 个站点。
- 添加第 2 个站点时再要求登录/购买。

---

## 4. 域名选取建议

### 4.1 域名原则

域名要同时满足：

- 国际用户能读懂。
- 中国大陆用户能访问。
- 不容易拼错。
- 和产品关键词相关。
- 能拆分官网、账号平台、下载、文档。

### 4.2 推荐域名结构

如果已有 `prerender.websitetool.cn`，可以短期继续用，但商业化建议准备独立品牌域名。

推荐优先级：

| 类型 | 示例 | 说明 |
|---|---|---|
| 国际主域 | `prerendershield.com` | 最适合商业化和海外用户 |
| 产品短域 | `pshield.dev` | 开发者友好，但品牌解释成本稍高 |
| 中国主域 | `prerendershield.cn` | 面向大陆备案和支付审核 |
| 公司工具域 | `websitetool.cn` | 可保留为公司工具矩阵 |

### 4.3 子域规划

| 子域 | 用途 |
|---|---|
| `www.prerendershield.com` | 官网 |
| `account.prerendershield.com` | 用户注册、账单、授权 |
| `docs.prerendershield.com` | 文档 |
| `download.prerendershield.com` | 安装包下载 |
| `api.prerendershield.com` | 授权 API、支付回调 |
| `status.prerendershield.com` | 服务状态页 |

中国大陆可同步：

| 子域 | 用途 |
|---|---|
| `www.prerendershield.cn` | 备案后的国内官网 |
| `account.prerendershield.cn` | 国内账号平台 |
| `download.prerendershield.cn` | 国内下载加速 |

### 4.4 域名选择结论

建议采购：

1. `prerendershield.com`
2. `prerendershield.cn`
3. `prerendershield.com.cn`，可选

短期可继续使用 `prerender.websitetool.cn`，但官网、授权平台和价格页应逐步迁移到独立品牌域名。

---

## 5. 官网必须体现的内容

官网第一屏必须出现：

- Prerender Shield。
- 1 个站点永久免费，全功能开放。
- 超过 1 个站点，99 美元 / 站点 / 年。
- 源码私有化部署，9999 美元。
- 一键安装命令。
- 自托管，数据在自己的服务器。

价格页必须有：

| 套餐 | 价格 | 说明 |
|---|---:|---|
| Free | $0 | 1 个站点，全部功能 |
| Per-site | $99 / site / year | 超过 1 个站点后按站点购买 |
| Private Source Deployment | $9999 | 源码私有化部署软件费用 |

FAQ 必须解释：

- 免费版是否限制功能：不限制。
- 是否限制站点：免费 1 个。
- 是否必须联网：单站点可离线使用，多站点建议在线授权；私有化可离线授权。
- 是否支持中国大陆付款：支持人民币支付和对公转账。
- 是否提供源码：仅私有化部署套餐提供。

---

## 6. 下一步执行顺序

1. 官网新增价格页。
2. README 和商业文档全部改成新价格策略。
3. 后端补齐 license key 配置、授权状态 API。
4. 管理后台新增授权状态页。
5. 添加第 2 个站点时引导购买。
6. 搭建最小账号平台。
7. 接入微信/支付宝或先人工收款。
8. 发布第一个商业版安装包。

---

## 7. 参考信息

- 微信支付商户平台：适合中国大陆微信生态用户收款。
- 支付宝开放平台电脑网站支付：适合官网在线支付。
- Stripe / Paddle / Lemon Squeezy：适合海外用户美元付款。
- 中国大陆正式经营网站通常需要准备备案、隐私政策、用户协议、退款政策、联系方式等材料。

具体支付资质、备案和税务要求会随主体类型、地区、业务形态变化，正式上线前应以支付平台、云厂商、工信部备案系统和当地税务要求为准。
