# 使用官方Go镜像作为构建环境
FROM golang:1.20-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制go.mod和go.sum文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建后端二进制文件
RUN CGO_ENABLED=0 GOOS=linux go build -o api ./cmd/api

# 构建前端
FROM node:18-alpine AS frontend-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm install --legacy-peer-deps
COPY web ./
RUN npm run build

# 使用轻量级的Alpine Linux作为运行环境
FROM alpine:3.18

# 安装必要的依赖
RUN apk add --no-cache tzdata ca-certificates chromium

# 设置时区
ENV TZ=Asia/Shanghai

# 创建工作目录
WORKDIR /app

# 复制后端二进制文件
COPY --from=builder /app/api ./

# 复制前端构建文件
COPY --from=frontend-builder /app/web/dist ./web/dist

# 复制配置文件模板
COPY configs/config.example.yml ./config.example.yml

# 复制启动脚本
COPY docker-entrypoint.sh ./
RUN chmod +x docker-entrypoint.sh

# 暴露端口
EXPOSE 9598 9597 8081

# 设置环境变量
ENV GIN_MODE=release

# 启动应用
ENTRYPOINT ["./docker-entrypoint.sh"]
CMD ["--config", "./config.example.yml"]