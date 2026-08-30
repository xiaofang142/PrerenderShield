# PrerenderShield 多阶段构建镜像
# 构建前端 → 构建后端 → 最小运行镜像（alpine + chromium）
# 用法见 docs/DOCKER.md

# ---------- 阶段 1：构建管理控制台前端 ----------
FROM node:20-alpine AS frontend-builder
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---------- 阶段 2：构建后端二进制 ----------
FROM golang:1.25-alpine AS backend-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -trimpath -o /out/api ./cmd/api

# ---------- 阶段 3：最小运行镜像 ----------
FROM alpine:3.20

# tzdata 时区 / ca-certificates HTTPS / chromium 渲染引擎
RUN apk add --no-cache tzdata ca-certificates chromium \
    && adduser -D -h /app -s /sbin/nologin appuser

ENV TZ=Asia/Shanghai \
    GIN_MODE=release \
    CHROME_PATH=/usr/bin/chromium-browser

WORKDIR /app

COPY --from=backend-builder /out/api ./api
COPY --from=frontend-builder /src/web/dist ./web/dist
COPY configs/config.example.yml ./config.example.yml

RUN mkdir -p /app/data /app/static /app/certs \
    && chown -R appuser:appuser /app

USER appuser

EXPOSE 9597 9598

HEALTHCHECK --interval=15s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -qO- http://127.0.0.1:9598/api/v1/health >/dev/null || exit 1

ENTRYPOINT ["/app/api"]
CMD ["--config", "/app/config.example.yml"]
