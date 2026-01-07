# 使用官方Node.js镜像构建前端
FROM node:18-alpine AS frontend-builder

# 接受构建参数
ARG PUBLIC_IP

# 设置工作目录
WORKDIR /app/web

# 复制前端代码
COPY web/package*.json ./
COPY web/tsconfig*.json ./
COPY web/vite.config.ts ./
COPY web/index.html ./
COPY web/src ./src

# 安装依赖并构建前端
# 配置npm镜像加速
RUN npm config set registry https://registry.npmmirror.com && \
    npm install
# 使用构建参数或默认值作为API地址
RUN API_IP=${PUBLIC_IP:-$(curl -s ifconfig.me 2>/dev/null || echo "localhost")} && \
    export VITE_API_BASE_URL="http://${API_IP}:9598/api/v1" && \
    echo "使用API地址: ${VITE_API_BASE_URL}" && \
    npm run build

# 使用官方Go镜像构建后端
FROM golang:1.24-alpine AS backend-builder

# 设置工作目录
WORKDIR /app

# 配置Go代理，解决依赖下载超时问题
ENV GOPROXY=https://goproxy.cn,direct
ENV GO111MODULE=on

# 复制go.mod和go.sum文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制应用源代码
COPY . .

# 构建应用程序
RUN go build -o ./prerender-shield ./cmd/api

# 查看构建结果
RUN ls -la /app

# 使用alpine作为运行时环境
FROM alpine:latest

# 设置工作目录
WORKDIR /app

# 复制构建好的二进制文件（兼容Go默认行为）
COPY --from=backend-builder /app/prerender-shield .

# 查看复制结果
RUN ls -la /app

# 复制前端构建文件
COPY --from=frontend-builder /app/web/dist ./web/dist

# 复制配置文件模板
COPY configs/config.example.yml ./configs/config.example.yml

# 创建必要的目录
RUN mkdir -p /etc/prerender-shield/rules /etc/prerender-shield/certs /app/data /app/certs

# 从示例配置复制配置文件到应用期望的位置
RUN cp ./configs/config.example.yml /etc/prerender-shield/config.yml

# 暴露端口
EXPOSE 9597 9598

# 设置环境变量
ENV CONFIG_PATH=/etc/prerender-shield/config.yml

# 启动应用程序，使用正确的配置文件路径
CMD ./api --config ${CONFIG_PATH:-/etc/prerender-shield/config.yml}
