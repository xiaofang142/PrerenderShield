#!/bin/bash

# PrerenderShield 启动脚本

echo "========================================"
echo "PrerenderShield 启动脚本"
echo "========================================"

# 检查是否为root用户
if [ "$EUID" -eq 0 ]; then
  echo "警告: 正在以root用户运行，这可能不是最佳实践"
fi

# 检查Go是否安装
if ! command -v go &> /dev/null; then
    echo "错误: 未安装Go，无法构建应用程序"
    exit 1
fi

# 检查Go版本
GO_VERSION=$(go version | grep -o 'go1\.[0-9]*')
if [[ "$GO_VERSION" != "go1.20" && "$GO_VERSION" != "go1.21" && "$GO_VERSION" != "go1.22" ]]; then
    echo "警告: Go版本可能不兼容，建议使用Go 1.20+，当前版本: $GO_VERSION"
fi

# 创建必要的目录
echo "创建必要的目录..."
mkdir -p data/rules data/certs data/redis data/grafana

# 检查配置文件是否存在
if [ ! -f configs/config.yml ]; then
    echo "配置文件不存在，从模板复制..."
    if [ -f configs/config.example.yml ]; then
        cp configs/config.example.yml configs/config.yml
        echo "已从config.example.yml复制到configs/config.yml"
    else
        echo "错误: 配置文件模板configs/config.example.yml不存在"
        exit 1
    fi
fi

# 安装依赖
echo "安装Go依赖..."
go mod tidy

# 构建应用程序
echo "构建应用程序..."
go build -o prerender-shield ./cmd/api

if [ $? -ne 0 ]; then
    echo "错误: 构建失败"
    exit 1
fi

echo "构建成功！"

# 启动应用程序
echo "启动PrerenderShield..."
echo "========================================"
echo "应用程序将在 http://0.0.0.0:8080 上运行"
echo "健康检查接口: http://0.0.0.0:8080/api/v1/health"
echo "版本信息接口: http://0.0.0.0:8080/api/v1/version"
echo "========================================"

# 启动应用程序
./prerender-shield --config configs/config.yml
