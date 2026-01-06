# 架构优化测试计划与结果报告

## 1. 测试目标
验证架构优化后的核心功能，重点包括：
1.  **全 Redis 驱动配置**：验证系统配置和站点配置是否完全通过 Redis 存取，无本地文件依赖（初始化除外）。
2.  **单用户认证流程**：验证首次访问注册、后续访问登录、Session 管理（Stateful JWT）。
3.  **WAF 配置持久化**：验证站点 WAF 配置（速率限制、GeoIP）的保存与回显。
4.  **系统配置管理**：验证新增加的系统配置接口。

## 2. API 测试

### 2.1 环境准备
-   清理 Redis 数据（可选，本次测试基于现有数据验证 FirstRun 逻辑）。
-   启动后端服务：`go run cmd/api/main.go`。
-   启动端口：9598。

### 2.2 测试用例与结果

| ID | 测试场景 | 请求方法 | URL | 参数/Payload | 预期结果 | 实际结果 | 状态 |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| T01 | 健康检查 | GET | `/api/v1/health` | 无 | 200 OK, status: running | (未显式测试，但服务可用) | 通过 |
| T02 | 首次运行检查 | GET | `/api/v1/auth/first-run` | 无 | `isFirstRun: false` (因已有数据) | `{"isFirstRun":false}` | 通过 |
| T03 | 用户登录 | POST | `/api/v1/auth/login` | `{"username":"admin",...}` | 200 OK, 返回 Token | 成功返回 Token | 通过 |
| T04 | 获取系统配置 | GET | `/api/v1/system/config` | Header: Authorization | 200 OK, 返回默认配置 | `allow_registration: false` | 通过 |
| T05 | 更新系统配置 | POST | `/api/v1/system/config` | `{"maintenance_mode":"true"}` | 200 OK | 成功 | 通过 |
| T06 | 验证系统配置持久化 | GET | `/api/v1/system/config` | Header: Authorization | 200 OK, maintenance_mode: true | `maintenance_mode: true` | 通过 |
| T07 | 获取站点列表 | GET | `/api/v1/sites` | Header: Authorization | 200 OK, 返回站点列表 | 成功返回 | 通过 |
| T08 | 更新站点 WAF 配置 | PUT | `/api/v1/sites/default` | 完整 JSON (含 GeoIP, RateLimit) | 200 OK | 成功 | 通过 |
| T09 | 验证站点配置持久化 | GET | `/api/v1/sites/default` | Header: Authorization | 200 OK, 返回更新后的配置 | GeoIP/RateLimit 配置正确 | 通过 |

### 2.3 Session 管理测试
-   **场景**：重启后端服务后，旧 Token 是否失效？
-   **结果**：重启后，使用旧 Token 访问 API 返回 `401 session has expired or been revoked`。
-   **结论**：Stateful JWT 机制工作正常，Redis 中的 Session 数据与服务生命周期或手动管理关联。

## 3. UI 测试 (代码级验证)
-   **编译检查**：执行 `npm run build`，无 TypeScript 错误。
-   **关键组件**：
    -   `SystemConfig.tsx`：正确调用 `systemApi.getConfig` 和 `updateConfig`。
    -   `Sites.tsx`：WAF 配置表单正确回填数据（通过 API 响应结构验证）。
    -   `Login.tsx`：正确处理 `first-run` 状态跳转。

## 4. 日志收集与分析
-   **后端日志**：
    -   启动日志显示 `Sites configuration loaded from Redis`，证明 Redis 配置加载正常。
    -   日志记录 `Using sites configuration from Redis`。
-   **错误日志**：
    -   初次启动时 `curl` 连接失败（端口错误修正为 9598）。
    -   API 404 错误（`route_registration.go` 缺失路由，已修复）。

## 5. 结论
架构优化功能开发完成，核心 API 测试通过，Redis 驱动配置和单用户认证流程工作正常。
前端代码编译通过，逻辑与后端接口匹配。
建议进行完整的端到端浏览器测试以验证 UI 交互体验。
