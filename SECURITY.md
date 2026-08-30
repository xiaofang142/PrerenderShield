# 安全策略（SECURITY）

感谢你负责任地披露安全问题。Prerender Shield 是自托管的安全防护与预渲染网关，我们非常重视漏洞报告。

## 🐞 如何报告漏洞

**请勿在公开 Issue / Discussion / 群组中讨论未修复的安全问题。**

请选择以下任一私密渠道：

| 渠道 | 地址 | 说明 |
|------|------|------|
| GitHub 私密漏洞报告 | [Security Advisories → Report a vulnerability](https://github.com/xiaofang142/PrerenderShield/security/advisories/new) | 首选 |
| 邮件 | `myloveisphp@126.com` | 邮件主题请注明 **`[SECURITY] PrerenderShield`** |

报告时请尽量包含：

1. 受影响版本（`git rev-parse HEAD` 或 Release 版本号）
2. 漏洞类型与影响面（如：WAF 绕过、认证绕过、SSRF、信息泄露）
3. 复现步骤 / PoC（最小化即可）
4. 缓解措施建议（如有）

## ⏱ 响应时限

| 阶段 | 目标时限 |
|------|---------|
| 确认收到报告 | 48 小时内 |
| 评估定级（CVSS）与影响范围 | 7 天内 |
| 修复或给出缓解方案 | 高危 30 天 / 中低危 90 天内 |
| 发布修复版本并公开披露 | 与修复版本同步，经报告人同意后披露致谢 |

在修复发布前，请对漏洞信息保密；我们会在披露时征得你的同意对报告人致谢。

## 🧩 支持版本

| 版本 | 支持状态 |
|------|---------|
| v3.0.x（最新 Release） | ✅ 支持安全修复 |
| < v3.0 | ❌ 停止支持，请升级 |

## ✅ 支持范围（报告范围内）

- `cmd/api` 主程序及其 REST API（`:9598`）与管理控制台（`:9597`）
- WAF 规则引擎（注入/XSS/CC/GeoIP/Bot 验证）的绕过
- 配置加密（`PRERENDER_MASTER_KEY`）、JWT 认证、API Token 机制
- `install.sh` / `build.sh` / Docker 镜像中的安全问题

## ❌ 不在范围内

- 需要物理访问或对自部署主机已有 root 权限的攻击
- 社会工程学、对基础设施的 DDoS/流量压测
- 对官方在线演示站点（prerender.websitetool.cn）的实测攻击——请改为在本地实例复现
- 依赖库自身已知漏洞且无项目侧可利用路径的报告（欢迎提普通 Issue 讨论升级）

## 🔐 安全加固参考

生产环境加固建议见 [docs/OPERATIONS_MANUAL.md](docs/OPERATIONS_MANUAL.md)（安全加固章节）与环境变量说明 [docs/ENV_VARS.md](docs/ENV_VARS.md)（`JWT_SECRET`、`PRERENDER_MASTER_KEY` 等）。
