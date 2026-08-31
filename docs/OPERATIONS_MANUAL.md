# Prerender Shield 运维手册

> 面向运维/SRE 人员。覆盖日常操作、监控告警、备份恢复、安全加固、容量规划与维护清单。
> 产品功能说明见 [OFFICIAL_DOCUMENTATION.md](OFFICIAL_DOCUMENTATION.md)，故障处置见 [TROUBLESHOOTING_GUIDE.md](TROUBLESHOOTING_GUIDE.md)。

## 目录

1. [部署拓扑](#部署拓扑)
2. [日常操作](#日常操作)
3. [升级与回滚](#升级与回滚)
4. [备份与恢复](#备份与恢复)
5. [监控与告警](#监控与告警)
6. [日志管理](#日志管理)
7. [容量规划](#容量规划)
8. [安全加固](#安全加固)
9. [定期维护清单](#定期维护清单)

---

## 部署拓扑

### 单机一体化（默认）

```
[用户/爬虫] → :8082(站点端口) → PrerenderShield → 源站
                                    ├── Redis (本机, AOF)
                                    └── Chromium 池 (min2/max10)
管理面: :9597 控制台 / :9598 API / :9090 Prometheus（建议不对公网暴露）
```

### 生产建议

| 关注点 | 建议 |
|--------|------|
| 管理端口 | `server.address` 保持对公网关闭，9597/9598/9090 仅内网或经 VPN/防火墙白名单访问 |
| Redis | 独立实例或本机均可；**必须开启 AOF**（`appendonly yes`，docker-compose 已内置）；设置 `requirepass` |
| Chromium | 通过系统包安装，或用 `PRERENDER_CHROMIUM_PATH` 指向固定版本路径 |
| GeoIP | 放置 GeoLite2-Country.mmdb 到 `./rules/GeoLite2-Country.mmdb`，避免依赖限频的外部 API |
| TLS | 站点证书走控制台 SSL 模块（ACME 自动申请续期），证书落 `./certs` |

## 日常操作

所有命令在部署目录执行：

```bash
./start.sh start      # 启动（内置健康探测，输出 API/控制台就绪状态）
./start.sh restart    # 重启
./start.sh stop       # 停止
./start.sh status     # 状态 + 健康检查
```

**进程模型**：单二进制后台运行，PID 写入 `data/prerender-shield.pid`，日志写入 `data/prerender-shield.log`。

**健康检查**：

```bash
curl -fsS http://127.0.0.1:9598/api/v1/health    # code=200 即存活
curl -fsS http://127.0.0.1:9598/api/v1/version   # 版本信息
```

**配置变更**：

- 站点/WAF 规则 → 控制台修改即时生效（存 Redis）
- 主配置 YAML → 保存后 watcher 自动加载；改端口等少数项仍需重启

**启动期自动维护（无需人工干预）**：

- Janitor 清扫上次异常退出遗留的孤儿 Chromium 进程与超过 1 小时的临时目录
- Chromium 路径自检：配置 → 环境变量 → 系统常见路径逐级探测，结果打印在启动日志
- GeoIP 磁盘缓存加载（`data/geoip_cache.json`），损坏时告警并从空缓存开始

## 升级与回滚

```bash
# 1. 备份当前版本与数据
cp bin/prerender-shield bin/prerender-shield.bak
tar czf backup-$(date +%F).tgz config/ data/ certs/

# 2. 构建或解压新版本二进制
./build.sh        # 或下载 release 替换 bin/prerender-shield

# 3. 滚动重启
./start.sh restart && ./start.sh status

# 4. 验证
curl -fsS http://127.0.0.1:9598/api/v1/version
tail -50 data/prerender-shield.log   # 确认无 ERROR 刷屏、Chromium 自检通过

# 回滚：stop → 用 .bak 二进制覆盖 → start
```

升级注意：

- 新版本首次启动会执行孤儿清扫，属正常日志
- 若新版调整了配置结构，参照 `configs/config.example.yml` diff 你的 `config/config.yml`

## 备份与恢复

| 数据 | 位置 | 备份方式 | 频率建议 |
|------|------|----------|----------|
| 站点/WAF 配置 | **Redis** | `redis-cli SAVE` 后拷贝 `dump.rdb`，或 BGREWRITEAOF 后备份 AOF | 每日 |
| 渲染缓存 | Redis | 可不备（可由预热任务重建） | — |
| 证书 | `certs/` | tar 归档 | 变更时 + 每月 |
| 主配置 | `config/config.yml` | git 或 tar | 变更时 |
| 日志 | `data/*.log` | 按需归档 | — |
| GeoIP 缓存 | `data/geoip_cache.json` | 可不备（自动重建） | — |

恢复流程：停服 → 还原 Redis 数据集 → 还原 `certs/` 与 `config/` → 启动 → `status` 验证。

> Redis 是唯一的"事实源"（站点配置），**Redis 的可用性 = 服务的可用性**，务必对其做持久化与监控。

## 监控与告警

### Prometheus 指标

- 地址：`monitoring.prometheus_address`（默认 `:9090/metrics`）
- 关键指标：渲染请求 QPS/耗时（`prerender_requests_total` / `prerender_response_time_seconds` / `prerender_avg_render_time_seconds`）、Chromium 实例数（`prerender_active_browsers`）、WAF 拦截（`prerender_blocked_requests_total` / `prerender_waf_block_rate`）、缓存命中率与渲染成功率（`prerender_cache_hit_rate` / `prerender_render_success_rate`）

### WebSocket 实时流（内部消费）

- `/ws/realtime?token=<JWT>`：告警事件实时推送 + 每 10s 指标广播（控制台 Dashboard 已接入；控制台 `/ws/` 前缀反向代理至管理 API）

### 示例告警规则

参考 `configs/alert-rules.example.json`，最小集建议：

| 告警 | 条件（示例） | 处置入口 |
|------|--------------|----------|
| 服务不可用 | `/api/v1/health` 连续 3 次 5xx/超时 | TROUBLESHOOTING §服务启动失败 |
| Chromium 进程暴涨 | chromedp-runner 进程数 > HardProcessCap×0.8 | TROUBLESHOOTING §浏览器相关 |
| Redis 连接失败 | 日志出现 redis pool 错误 | TROUBLESHOOTING §Redis 相关 |
| WAF 拦截激增 | 分钟拦截数 > 基线 5 倍 | 控制台日志模块确认是否攻击 |
| 磁盘水位 | 分区使用率 > 85% | 清理日志/文件缓存 |

## 日志管理

| 文件 | 内容 |
|------|------|
| `data/prerender-shield.log` | 主日志（API/渲染/WAF/调度） |

```bash
# 高频排查命令
tail -f data/prerender-shield.log
grep -c ERROR data/prerender-shield.log
grep "chromium process cap" data/prerender-shield.log   # 进程上限触顶
grep "GeoIP" data/prerender-shield.log                  # GeoIP 数据源状态
```

轮转：主日志为追加写，请用 logrotate 或定时任务切割归档，示例 cron：

```cron
0 3 * * * cd /opt/prerender-shield && mv data/prerender-shield.log logs/shield-$(date +\%F).log && gzip logs/shield-*.log && ./start.sh restart >/dev/null
```

## 容量规划

| 维度 | 经验值 | 说明 |
|------|--------|------|
| 内存/Chromium 实例 | ~300–500MB | `js-flags` 已限制 V8 堆 512MB；小内存机器用 `PRERENDER_MAX_INSTANCES=2~3` |
| 实例数公式 | max ≥ 峰值并发爬虫渲染数 | min 为常驻热实例，保证首请求低延迟 |
| Chromium 进程硬上限 | `MaxInstances*8+16` | 每实例派生约 8 个 OS 进程；可用 `PRERENDER_PROCESS_CAP` 覆盖（一般不需要） |
| Redis 内存 | 缓存条目 × 平均页大小 | 渲染缓存为纯 Redis 存储（无本地 L1；`cache.memory_size` 为预留参数未参与链路） |
| 磁盘 | 日志 + 证书 + Redis 持久化文件 | 预留 20GB SSD |

扩容路径：垂直加内存提高实例数 → 水平多实例（每台独立 Redis 或共享）→ 前置 LB 按域名分流。

## 安全加固

1. **必做**
   - 设置强随机 `JWT_SECRET`
   - Redis 设置密码并绑定内网地址
   - 管理端口（9597/9598/9090）不对公网开放
   - 首次登录后妥善保管管理员凭据（无默认账号）
2. **推荐**
   - 开启配置加密：设置 `PRERENDER_MASTER_KEY`（AES-256-GCM），密钥离机保管
   - GeoIP 使用本地 MMDB，减少外部依赖
   - 定期更新 Chromium（安全补丁随浏览器发布）
   - WAF 规则保持默认开启，新站点先观察模式再切拦截
3. **禁止**
   - 将 `data/`、`certs/`、含密钥的配置提交到代码仓库
   - 以 root 长期运行（install.sh 已按普通服务用户部署）

## 定期维护清单

| 周期 | 项目 |
|------|------|
| 每日 | `start.sh status`；检查告警通道是否静默；磁盘水位 |
| 每周 | 巡检 ERROR 日志趋势；验证预热任务成功率；Redis RDB/AOF 备份有效性抽查 |
| 每月 | 升级浏览器与新版本；轮换 `JWT_SECRET`（滚动重启）；演练一次备份恢复；清理过期日志归档 |
| 每季 | 容量复盘（内存/磁盘/实例峰值）；GeoLite2 数据库更新；故障演练（kill -9 后启动自愈验证 Janitor） |
