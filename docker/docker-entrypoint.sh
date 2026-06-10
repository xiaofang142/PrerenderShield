#!/bin/sh

set -e

# 检查是否存在配置文件
if [ ! -f "./config.yml" ]; then
    echo "配置文件 ./config.yml 不存在，使用默认配置模板"
    if [ -f "./config.example.yml" ]; then
        cp ./config.example.yml ./config.yml
        echo "已从 config.example.yml 创建配置文件"
    else
        echo "错误：默认配置模板 ./config.example.yml 不存在"
        exit 1
    fi
fi

# 检查Redis连接
if [ -n "$REDIS_URL" ]; then
    echo "使用自定义Redis连接: $REDIS_URL"
    # 替换配置文件中的Redis URL
    if [ -f "./config.yml" ]; then
        sed -i "s|redis_url:.*|redis_url: \"$REDIS_URL\"|" ./config.yml
    fi
else
    echo "使用默认Redis连接: localhost:6379"
fi

# 启动应用
echo "启动 Prerender Shield 服务..."
echo "API服务端口: 9598"
echo "管理控制台端口: 9597"
echo "静态站点端口: 由配置决定"

# 执行应用
./api --config ./config.yml