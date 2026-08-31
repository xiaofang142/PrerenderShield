# Prerender Shield 故障排查手册

> 按症状索引的处置手册。日常操作见 [OPERATIONS_MANUAL.md](OPERATIONS_MANUAL.md)，产品说明见 [OFFICIAL_DOCUMENTATION.md](OFFICIAL_DOCUMENTATION.md)。

## 目录

1. [快速诊断流程](#快速诊断流程)
2. [启动与配置问题](#启动与配置问题)
3. [Redis 问题](#redis-问题)
4. [浏览器与渲染问题](#浏览器与渲染问题)
5. [缓存与预热问题](#缓存与预热问题)
6. [WAF 与 GeoIP 问题](#waf-与-geoip-问题)
7. [SSL 证书问题](#ssl-证书问题)
8. [控制台与认证问题](#控制台与认证问题)
9. [性能问题](#性能问题)
10. [日志分析与诊断命令](#日志分析与诊断命令)

---

## 快速诊断流程

```bash
# 1. 服务活着吗？
./start.sh status
curl -fsS http://127.0.0.1:9598/api/v1/health   # code=200 存活

# 2. Redis 通吗？
redis-cli ping                                   # PONG 即通

# 3. 浏览器可用吗？（看启动自检行）
grep -i "chromium\|browser" data/prerender-shield.log | tail -5

# 4. 最近错误？
grep ERROR data/prerender-shield.log | tail -20

# 5. 端口冲突？
lsof -i :9598 -i :9597 -i :8082
```

按命中位置跳转对应章节。

## 启动与配置问题

### 服务启动失败 / 启动后立即退出

| 日志特征 | 原因 | 处置 |
|----------|------|------|
| `invalid API port: 0, must be between 1 and 65535` | 端口缺失或为 0，配置校验拒绝启动 | 在 `config/config.yml` 设置合法 `api_port`/`console_port`（默认 9598/9597） |
| `load config: ... validation failed` | YAML 语法或必填项错误 | `yamllint config/config.yml`；对照 `configs/config.example.yml` 补齐 |
| `Failed to open MaxMind DB ... falling back to HTTP API` | MMDB 路径不存在（非致命） | 放置 GeoLite2-Country.mmdb 或忽略（走 API 兜底） |
| `GeoIP disk cache corrupted at ... starting with empty cache` | geoip_cache.json 损坏（非致命） | 可删除该文件自动重建 |

### 端口被占用

```bash
lsof -i :9598            # 找到占用进程
kill <pid>               # 或改 config 中端口
```

端口也可用环境变量覆盖：`SERVER_API_PORT` / `SERVER_CONSOLE_PORT`。

### 配置热更新不生效

1. 确认改的是对应层级：站点级配置存 **Redis**（控制台改），主配置走文件 watcher
2. 日志中找配置加载错误：`grep -i "config" data/prerender-shield.log | tail`
3. 个别字段（如端口、地址）需重启才生效——这是预期行为

## Redis 问题

**症状**：启动失败、站点列表加载不出、缓存全部未命中、日志出现 `redis pool` 相关 WARN。

```bash
redis-cli ping                          # 无响应 → redis-server 未启动
redis-cli -a <密码> ping                # 有密码时
grep -i redis data/prerender-shield.log | tail -10
```

常见原因：

1. **未启动**：`systemctl start redis` 或 `redis-server --daemonize yes`
2. **连接串错误**：核对 `cache.redis_url` 与 `CACHE_REDIS_URL` 环境变量（后者覆盖前者；`REDIS_HOST/PORT/PASSWORD/DB` 组合亦可。`REDIS_URL` 仅 Docker 入口脚本生效）
3. **密码/绑定问题**：Redis 开了 `requirepass` 但配置未带密码；或 bind 了内网 IP 而应用在另一台机器
4. **数据丢失导致站点消失**：Redis 无持久化被重启清空 → 恢复 RDB/AOF；生产必须 `appendonly yes`

> 站点/WAF 配置的唯一事实源是 Redis。**先保 Redis，再排查其他。**

## 浏览器与渲染问题

### Chromium 未找到 / 渲染全失败

**诊断**：启动日志有浏览器自检结果；运行期看 `created new chromium instance` 是否出现。

```bash
grep -i "chromium path\|ExecPath\|browser" data/prerender-shield.log | tail -5
which chromium chromium-browser google-chrome google-chrome-stable
```

**处置（优先级从高到低）**：
1. 设 `PRERENDER_CHROMIUM_PATH=/usr/bin/chromium`（或 `CHROME_PATH`）后重启
2. 重跑安装脚本的浏览器检测：`sudo ./install.sh`
3. macOS 默认探测 Chrome.app；Linux 探测常见包名路径

### 进程上限触顶："chromium process cap reached"

这是**保护机制**：系统内 chromedp 进程数达到硬上限（默认 `MaxInstances*8+16`）时拒绝新建实例。

```bash
ps -eo pid,args | grep chromedp-runner | grep -v grep | wc -l   # 当前进程数
grep "chromium process cap" data/prerender-shield.log           # 触顶记录
```

| 场景 | 处置 |
|------|------|
| 大量僵尸进程（历史崩溃遗留） | 重启服务——启动期 Janitor 自动清扫孤儿 |
| 真实业务需要更多实例 | 提高内存并调大 `PRERENDER_MAX_INSTANCES`；特殊场景可用 `PRERENDER_PROCESS_CAP` 显式覆盖上限 |
| 内存不足导致的连环崩溃重启 | 先加内存/降实例数，见"高内存使用" |

### 渲染超时 / 返回空页面

1. 目标页面本身慢（异步数据多）→ 调大站点 `prerender.timeout`
2. 实例池耗尽 → 观察 Prometheus 实例数指标，调大 `PRERENDER_MAX_INSTANCES`
3. 单页 JS 崩溃 → 用无头模式手动复现：`chromium --headless --dump-dom <url>`

### 高内存使用

- 每实例 V8 堆已限 512MB，实例数 × 500MB 估算总量
- 降低 `PRERENDER_MAX_INSTANCES`（Docker <4GB 建议 2~3）
- 实例有使用计数回收（MaxUseCount），若怀疑泄漏抓 heap profile 并提 Issue

## 缓存与预热问题

### 爬虫拿到的还是 SPA 空壳

1. 确认 UA 触发识别：`curl -A "Mozilla/5.0 (compatible; Googlebot/2.1)" http://站点域名/`
2. 该 URL 是否已被预热/缓存：控制台 → 预热任务状态
3. 站点 `prerender.enabled` 是否开启

### 预热失败或不执行

- sitemap 解析失败：确认 `sitemap.xml` 可达且格式合法（每轮预热对 URL 去重，重复触发不会重复渲染）
- 定时任务未跑：检查调度器日志 `grep -i "schedul\|preheat" data/prerender-shield.log`

### 服务重启后缓存丢失

渲染缓存在 Redis 中，依赖 Redis 持久化（AOF/RDB）。若 Redis 数据丢了，参考上文恢复；缓存可通过预热任务重建。

## WAF 与 GeoIP 问题

### 规则不生效

1. 站点 `firewall.enabled: true` 已开启
2. 规则引擎支持热加载——控制台保存即生效；若走 API 更新，确认返回 code=200
3. 规则语法错误会在日志中有加载告警

### GeoIP 判定异常（误封/漏放）

判定链路：本地 MMDB → 外部 API（ip-api/ipinfo/ipapi-co）→ 磁盘缓存兜底 → UNKNOWN（fail-safe：BlockList 放行 / AllowList 拒绝）。

```bash
grep -i geoip data/prerender-shield.log | tail -10
cat data/geoip_cache.json | python3 -m json.tool | head    # 查看磁盘缓存
```

| 现象 | 原因 | 处置 |
|------|------|------|
| 全部 UNKNOWN + Error 日志 | 无 MMDB 且外部 API 全部不可达（断网/限频） | 放置 MMDB 到 `geoip.database_path`；临时可删 `data/geoip_cache.json` 后重试触发解析 |
| AllowList 模式大量误杀 | UNKNOWN fail-safe 特性 | 必须配 MMDB；检查磁盘缓存是否过期（7 天 TTL） |
| 使用陈旧归属地 | 磁盘缓存兜底（7 天内旧结果） | 属预期防误杀行为，API 恢复后新解析自动覆盖 |

### CC 防护误伤正常用户

调高频率阈值或添加白名单；观察拦截日志确认命中的是哪条检测器。

## SSL 证书问题

- 申请失败：确认 80/443 可达且域名解析正确（ACME HTTP 校验）
- 续期失败告警：控制台 SSL 模块查看证书状态；证书落 `certs/` 目录，检查其权限
- 手动验证：`openssl s_client -connect 域名:443 | openssl x509 -noout -dates`

## 控制台与认证问题

### 首次登录

本产品**没有默认账号**。首次访问控制台会进入建号引导（`CheckFirstRun`）；若引导没出现而直接要求登录，说明 Redis 里已有历史账号数据。

### 登录报 401

- 凭据错误或 JWT 过期 → 重新登录
- 服务器时间漂移会导致 JWT 校验失败 → `ntpdate`/chrony 对时
- `JWT_SECRET` 变更后所有旧 token 失效 → 重新登录即可

### 控制台打不开 / WebSocket 不实时

```bash
curl -I http://127.0.0.1:9597        # 控制台静态页
curl http://127.0.0.1:9598/api/v1/health
```

- 反代部署时确保 `/ws` 升级头透传（`Upgrade`/`Connection`）
- WebSocket 需要 JWT：`ws://host:9597/ws?token=<JWT>`，token 过期会断流，刷新页面重取

### 商业授权相关 402

触发了站点数/授权校验（licensing）。核对 `commercial` 配置段与当前授权的站点数量；开源自托管场景下如误配了商业限制，清理该配置段。

## 性能问题

| 症状 | 诊断 | 处置 |
|------|------|------|
| 高内存 | `top`；实例数 × ~500MB 对照 | 降 `PRERENDER_MAX_INSTANCES`；实例按使用计数自动退役（MaxUseCount），若怀疑泄漏抓 heap profile 并提 Issue |
| 高 CPU | 区分渲染进程与应用进程 | 限并发、优化复杂页面、预热错峰 |
| 响应慢 | 缓存命中率？源站延迟？ | 提高预热覆盖；检查源站健康 |
| Redis 慢 | `redis-cli --latency`；日志观察 `redis pool` 相关 WARN | 调大 `redis_pool.max_active`；独立 Redis 主机 |

基线参考：平均响应 < 500ms，P95 < 2s，CPU < 80%，内存 < 85%。

## 日志分析与诊断命令

日志级别：DEBUG / INFO / WARN / ERROR / FATAL。主日志：`data/prerender-shield.log`。

**关键日志速查**：

| 标识 | 含义 | 动作 |
|------|------|------|
| `Server location initialized` / `MaxMind GeoIP database loaded` | GeoIP 数据源就绪 | — |
| `GeoIP enabled WITHOUT local MMDB` | GeoIP 依赖外部 API，有限频风险 | 配置 MMDB |
| `using stale cached result` | API 失败，回退磁盘缓存旧值 | 关注网络连通性 |
| `chromium process cap reached` | 进程硬上限触顶 | 见浏览器章节 |
| `sweeping orphaned headless browsers` | 启动清扫孤儿浏览器 | 属正常自愈 |
| `Failed to open MaxMind DB` | MMDB 缺失，降级 API | 可忽略/补库 |

**常用命令集**：

```bash
./start.sh status                                  # 服务状态
curl -s http://127.0.0.1:9598/api/v1/health | jq   # 健康详情(Redis/内存/goroutine)
curl -s http://127.0.0.1:9598/api/v1/version | jq  # 版本
lsof -i :9598 -i :9597                             # 端口占用
ps aux | grep prerender                            # 进程
ps -eo args | grep -c chromedp-runner              # 浏览器进程数
redis-cli ping && redis-cli dbsize                 # Redis 连通与数据量
yamllint config/config.yml                         # 配置语法
```

## 联系支持

自行排查无果时，请收集以下信息提 Issue（GitHub/Gitee）：

1. `./start.sh status` 输出与 `/api/v1/version` 结果
2. 相关时间段的完整日志片段（脱敏后）
3. 配置文件（隐藏密码/密钥/JWT_SECRET）
4. 系统环境（OS/内存/Chromium 版本 `chromium --version`）
5. 重现步骤与期望行为
