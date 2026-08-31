# 无头浏览器进程治理方案

> 适用版本：v3.0.0+　|　更新：2026-08-26

## 一、问题背景

无头浏览器（chromedp 驱动的 Chromium）是渲染引擎的核心。每个浏览器实例在 OS 层面
派生多个进程（browser / gpu / renderer / utility，约 4–8 个），并创建
`$TMPDIR/chromedp-runner*` 临时 user-data-dir。

**故障模式**：宿主进程被 `SIGKILL`、崩溃或 `kill -9` 时，chromedp 的 cancel
链路失效——子浏览器进程变为孤儿继续运行，临时目录永久残留。
实测一次开发机排查发现 **42 个孤儿 chromium 进程 + 343 个残留临时目录**，
累计占用数 GB 内存与磁盘。

## 二、防护体系（已内置）

| 层 | 机制 | 位置 |
|----|------|------|
| ① 启动清扫 | 服务启动时回收上次遗留的孤儿进程（仅启动早期执行，此刻池未建，存在的 chromedp 进程均为遗留）与 mtime >1h 的临时目录 | `pool.SweepOrphans()`，bootstrap.Initialize 调用 |
| ② 进程数硬上限 | 每次创建实例前统计全局 chromedp 进程数，超过 `MaxInstances×8+16` 拒绝创建并报错提示 | `Pool.HardProcessCap()` / `createInstance` |
| ③ 同步关闭 | `Pool.Close()` 由异步改同步：cancel 必须完成后才允许进程退出 | pool.go Close |
| ④ 实例退役 | 按使用次数/健康度退役，退役即 cancel 进程树 | retireInstance/closeInstance |

### 启动日志示例

```
[WARN] sweeping orphaned headless browsers from previous runs: 12 killed
[INFO] janitor: removed 232 stale chromedp temp directories
[INFO] Chromium resolved: /usr/bin/chromium
```

## 三、人工处置手册

```bash
# 查看孤儿进程（带 chromedp-runner 标记的为本产品拉起）
ps -eo pid,args | grep chromedp-runner | grep -v grep

# 批量终止（确认无误后）
ps -eo pid,args | grep 'chromedp-runner' | grep -v grep \
  | awk '{print $1}' | xargs -r kill -15

# 清理残留临时目录（保留最近 1 小时内的，避免误删活跃实例）
find ${TMPDIR:-/tmp} -maxdepth 1 -name 'chromedp-runner*' -mmin +60 \
  -exec rm -rf {} +

# Docker 环境额外注意：
# 容器内 PID 1 收到 SIGKILL 时同样会产生孤儿；compose 当前未配置 init，
# 建议自行加上 init: true，让 tini 兜底收割僵尸子进程
```

## 四、Docker 建议

`docker-compose.yml` 建议补充（当前 compose 已有 `shm_size: 256m` 与 `mem_limit: 4g`，缺 `init`）：

```yaml
services:
  app:
    init: true          # tini 作为 PID 1，自动收割孤儿/僵尸
    shm_size: '1gb'     # chromium /dev/shm 需求
    deploy:
      resources:
        limits:
          memory: 4g    # 与 MaxInstances 匹配，防 OOM 连坐
```

## 五、容量规划参考

| 参数 | 默认值 | 说明 |
|------|--------|------|
| PRERENDER_MAX_INSTANCES | 10 | 浏览器实例上限 |
| 进程硬上限 | MAX_INSTANCES×8+16 | 全局 chromedp 进程数熔断线 |
| 单实例内存 | ~512MB（js-flags 已限堆） | renderer 另计 |
