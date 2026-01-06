# 开发指南 (Development Guide)

## 1. 环境准备

*   **Go**: 1.21 或更高版本
*   **Node.js**: 18.0 或更高版本 (用于前端)
*   **Redis**: 6.0 或更高版本
*   **Git**

## 2. 后端开发

### 2.1 依赖安装

```bash
go mod download
```

### 2.2 运行 API Server

```bash
# 设置环境变量（可选，有默认值）
export REDIS_ADDR="localhost:6379"
export API_PORT=9598

# 启动服务
go run cmd/api/main.go
```

服务启动后，API 将监听 `http://localhost:9598`。

### 2.3 运行测试

```bash
# 运行所有单元测试
go test ./internal/...

# 运行特定包的测试
go test ./internal/firewall/... -v
```

## 3. 前端开发

### 3.1 依赖安装

```bash
cd web
npm install
```

### 3.2 开发模式启动

```bash
npm run dev
```

前端服务将监听 `http://localhost:5173`（默认），并自动代理 API 请求到后端。

### 3.3 构建生产版本

```bash
npm run build
```

构建产物将输出到 `web/dist` 目录。

## 4. 目录规范

*   `internal/` 下的代码为私有代码，不应被外部项目导入。
*   `api/controllers` 处理 HTTP 请求逻辑。
*   `services` 处理核心业务逻辑。
*   `models` 定义数据结构。
*   `utils` 存放通用工具函数。

## 5. 常见问题

*   **Redis 连接失败**: 请检查 `config.yaml` 或环境变量中的 Redis 地址配置。
*   **端口冲突**: 默认 API 端口为 9598，可通过环境变量 `API_PORT` 修改。
