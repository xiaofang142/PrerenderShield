# 使用官方Go 1.20镜像作为构建环境
FROM golang:1.20-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制go.mod和go.sum文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制应用源代码
COPY . .

# 构建应用程序
RUN go build -o prerender-shield ./cmd/api

# 使用alpine作为运行时环境
FROM alpine:latest

# 设置工作目录
WORKDIR /app

# 复制构建好的二进制文件
COPY --from=builder /app/prerender-shield .

# 复制配置文件模板
COPY configs/config.example.yml ./configs/config.example.yml

# 创建必要的目录
RUN mkdir -p /etc/prerender-shield/rules /etc/prerender-shield/certs

# 暴露端口
EXPOSE 9597 9598

# 设置环境变量
ENV CONFIG_PATH=/etc/prerender-shield/config.yml

# 启动应用程序
CMD ["./prerender-shield", "--config", "/etc/prerender-shield/config.yml"]
