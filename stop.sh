#!/bin/bash

# PrerenderShield 停止脚本

echo "========================================"
echo "PrerenderShield 停止脚本"
echo "========================================"

# 查找进程ID
PID=$(pgrep -f "prerender-shield")

if [ -z "$PID" ]; then
    echo "PrerenderShield 未运行"
    exit 0
fi

echo "找到运行中的 PrerenderShield 进程: $PID"
echo "正在停止进程..."

# 发送终止信号
kill -TERM $PID

# 等待进程退出
wait $PID 2>/dev/null

if [ $? -eq 0 ]; then
    echo "PrerenderShield 已成功停止"
else
    echo "PrerenderShield 停止失败，尝试强制终止..."
    kill -KILL $PID
    wait $PID 2>/dev/null
    if [ $? -eq 0 ]; then
        echo "PrerenderShield 已强制停止"
    else
        echo "无法停止 PrerenderShield 进程"
        exit 1
    fi
fi

echo "========================================"
echo "停止完成"
echo "========================================"
