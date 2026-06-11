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

# 显示配置信息
echo "启动 Prerender Shield 服务..."
echo "API服务端口: 9598"
echo "管理控制台端口: 9597"
echo "Chrome 最大实例数: ${PRERENDER_MAX_INSTANCES:-10}"
echo "Chrome 最小实例数: ${PRERENDER_MIN_INSTANCES:-2}"
echo "渲染工作线程数: ${PRERENDER_WORKER_COUNT:-5}"

# 执行应用
./api --config ./config.yml