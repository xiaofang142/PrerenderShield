# 贡献指南（CONTRIBUTING）

感谢关注 **Prerender Shield**！本文档说明如何搭建开发环境、提交代码与参与社区。提交 Issue / PR 前，请先阅读本指南。

- 行为准则：[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- 安全漏洞：**请勿**通过公开 Issue 报告安全问题，见 [SECURITY.md](SECURITY.md)
- 问题反馈：[GitHub Issues](https://github.com/xiaofang142/PrerenderShield/issues) · [Gitee Issues](https://gitee.com/xhpmayun/prerender-shield/issues)

---

## 1. 开发环境搭建

### 系统要求

| 组件 | 版本要求 | 说明 |
|------|---------|------|
| Go | 1.25+ | 后端编译与测试 |
| Node.js | 20.x | 管理控制台前端（React 18 + TypeScript） |
| Redis | 7.x | 缓存与配置存储（集成测试依赖本地 `redis-server`） |
| Git | 2.x | 版本管理 |
| Chromium/Chrome | 可选 | 渲染预热联调时需要；可通过 `CHROME_PATH` 指定路径 |

### 搭建步骤

```bash
# 1. 克隆仓库
git clone https://github.com/xiaofang142/PrerenderShield.git
cd PrerenderShield

# 2. 启动本地 Redis（已装可跳过）
redis-server &

# 3. 启动后端（开发模式，默认 API :9598 / 控制台 :9597）
cd cmd/api && go run main.go

# 4. 启动前端开发服务（Vite，默认 :5173）
cd web && npm install && npm run dev
```

首次访问控制台时，使用登录页提交的账号密码创建管理员（无预置默认账号）。

### 常用命令

| 命令 | 作用 |
|------|------|
| `make build` | 一键构建前后端（调用 `build.sh`，产物 `bin/api` + `bin/web`） |
| `make test` | Go 全量单元测试（本地 Redis 不可用时集成用例自动跳过） |
| `make test-web` | 前端 vitest 单测 + i18n locale 完整性守护 |
| `make test-cover` | Go 测试并生成 `coverage.html` |
| `make lint` | `go vet` + gofmt 检查 |
| `make fmt` | gofmt 格式化 |
| `make verify-e2e` | E2E 验证（隔离实例 + API 端点全验证 + 浏览器巡检） |

---

## 2. 分支与提交规范

### 分支模型

- `master`：主开发分支，保持可构建、可测试
- 功能分支：`feat/<功能名>`，如 `feat/bot-verify`
- 修复分支：`fix/<问题名>`，如 `fix/redis-reconnect`
- 从最新 `master` 切出，合并前完成 rebase

### 提交信息（Conventional Commits）

格式：`<type>: <简要描述>`（英文小写 type + 中文描述均可）

| type | 用途 |
|------|------|
| `feat:` | 新功能 |
| `fix:` | 缺陷修复 |
| `docs:` | 文档变更 |
| `refactor:` | 重构（不改行为） |
| `test:` | 测试补充/修正 |
| `chore:` | 构建、依赖、脚本等杂项 |
| `ci:` | CI/CD 流水线变更 |

示例：

```text
feat: 站点级 TTL 缓存分级规则（ttl_rules）
fix: Redis 连接池 idle_timeout 配置未生效
docs: 补充 INSTALL.md 的 systemd 升级流程
```

一次提交只做一件事；大改动请拆分为多个提交，便于审查与回溯。

---

## 3. 测试要求（覆盖率政策）

**新代码必须携带测试，且不得使整体覆盖率下降。** CI 会以 `go test -race -coverprofile=coverage.out ./...` 作为门禁。

- 新增函数/逻辑：至少一条覆盖正常路径的表驱动测试；涉及错误分支的补错误路径用例
- 修复缺陷：先写一条能复现问题的失败测试，再修复使其通过
- 配置类改动（`internal/config/`）：同步更新 `configs/config.example.yml` 并补充校验用例
- 前端改动：`make test-web` 必须通过；i18n 文案变更需通过 locale 完整性守护
- 本地验证：`go test ./... -count=1` 与 `go tool cover -func=coverage.out | grep total` 自查

---

## 4. PR 流程

1. **Fork / 切分支**：从 `master` 切出功能或修复分支
2. **开发与自测**：按第 3 节完成测试；`make lint && make test` 全绿
3. **提交 PR**：填写 PR 描述（改了什么、为什么、如何验证）；关联相关 Issue（`Closes #123`）
4. **CI 门禁**：等待 [ci.yml](.github/workflows/ci.yml) 通过（Go 测试 + lint + 前端测试 + E2E）
5. **代码审查**：至少一名维护者审查通过后合并；按审查意见修改时请追加提交而非强推覆盖
6. **合并**：由维护者执行 squash 或 merge；合并后分支可删除

### PR 检查清单

- [ ] 提交信息符合 Conventional Commits
- [ ] 新代码带测试，覆盖率不下降
- [ ] `make lint`、`make test`、`make test-web` 通过
- [ ] 涉及配置/接口变更时同步更新了 `docs/` 相关文档
- [ ] 未引入新的硬编码密钥或敏感信息

---

## 5. 其他贡献方式

- **文档**：改进 `docs/` 下的指南、翻译、最佳实践示例
- **Issue 三角**：复现他人报告的问题、补充信息、确认修复
- **生态**：编写部署模板（systemd / compose / 反向代理配置）分享到 Discussions

---

再次感谢你的贡献！
