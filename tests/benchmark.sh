#!/bin/bash
# Prerender Shield 压力测试脚本
# 使用方法: ./benchmark.sh [目标URL] [并发数] [请求数]

set -e

# 默认参数
TARGET_URL="${1:-http://localhost:9598}"
CONCURRENT="${2:-50}"
REQUESTS="${3:-1000}"
DURATION="${4:-60}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Prerender Shield 压力测试${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${YELLOW}目标 URL:${NC} $TARGET_URL"
echo -e "${YELLOW}并发数:${NC} $CONCURRENT"
echo -e "${YELLOW}请求数:${NC} $REQUESTS"
echo -e "${YELLOW}测试时长:${NC} ${DURATION}s"
echo ""

# 检查依赖
check_dependencies() {
    echo -e "${YELLOW}检查依赖...${NC}"
    
    if ! command -v curl &> /dev/null; then
        echo -e "${RED}错误: 需要安装 curl${NC}"
        exit 1
    fi
    
    if ! command -v jq &> /dev/null; then
        echo -e "${YELLOW}警告: 建议安装 jq 以获得更好的输出格式${NC}"
    fi
    
    echo -e "${GREEN}依赖检查完成${NC}"
    echo ""
}

# 健康检查
health_check() {
    echo -e "${YELLOW}执行健康检查...${NC}"
    
    response=$(curl -s -o /dev/null -w "%{http_code}" "$TARGET_URL/api/v1/health" 2>/dev/null || echo "000")
    
    if [ "$response" = "200" ]; then
        echo -e "${GREEN}健康检查通过${NC}"
    else
        echo -e "${RED}健康检查失败 (HTTP $response)${NC}"
        echo -e "${YELLOW}请确保服务正在运行${NC}"
        exit 1
    fi
    echo ""
}

# 测试 API 响应时间
test_api_latency() {
    echo -e "${YELLOW}测试 API 响应时间...${NC}"
    
    local total_time=0
    local count=10
    local min_time=999999
    local max_time=0
    
    for i in $(seq 1 $count); do
        time_result=$(curl -s -o /dev/null -w "%{time_total}" "$TARGET_URL/api/v1/health" 2>/dev/null)
        time_ms=$(echo "$time_result * 1000" | bc 2>/dev/null || echo "0")
        
        total_time=$(echo "$total_time + $time_ms" | bc 2>/dev/null || echo "0")
        
        if (( $(echo "$time_ms < $min_time" | bc -l 2>/dev/null || echo 0) )); then
            min_time=$time_ms
        fi
        if (( $(echo "$time_ms > $max_time" | bc -l 2>/dev/null || echo 0) )); then
            max_time=$time_ms
        fi
    done
    
    avg_time=$(echo "scale=2; $total_time / $count" | bc 2>/dev/null || echo "0")
    
    echo -e "${GREEN}平均响应时间: ${avg_time}ms${NC}"
    echo -e "${GREEN}最小响应时间: ${min_time}ms${NC}"
    echo -e "${GREEN}最大响应时间: ${max_time}ms${NC}"
    echo ""
}

# 并发压力测试
concurrent_test() {
    echo -e "${YELLOW}执行并发压力测试...${NC}"
    
    local start_time=$(date +%s)
    local success_count=0
    local fail_count=0
    local total_requests=$REQUESTS
    local batch_size=100
    
    # 创建临时文件存储结果
    local tmp_file=$(mktemp)
    
    # 执行压力测试
    for ((i=1; i<=total_requests; i+=batch_size)); do
        local end=$((i + batch_size - 1))
        if [ $end -gt $total_requests ]; then
            end=$total_requests
        fi
        
        for ((j=i; j<=end; j++)); do
            (
                response=$(curl -s -o /dev/null -w "%{http_code}" "$TARGET_URL/api/v1/health" 2>/dev/null)
                echo "$response" >> "$tmp_file"
            ) &
        done
        
        # 等待当前批次完成
        wait
        
        # 进度显示
        progress=$((i * 100 / total_requests))
        echo -ne "\r进度: ${progress}%"
    done
    
    echo ""
    echo ""
    
    # 统计结果
    if [ -f "$tmp_file" ]; then
        success_count=$(grep -c "^200$" "$tmp_file" 2>/dev/null || echo "0")
        fail_count=$(grep -cv "^200$" "$tmp_file" 2>/dev/null || echo "0")
        rm "$tmp_file"
    fi
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    local rps=0
    if [ $duration -gt 0 ]; then
        rps=$((total_requests / duration))
    fi
    
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  压力测试结果${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}总请求数:${NC} $total_requests"
    echo -e "${GREEN}成功请求数:${NC} $success_count"
    echo -e "${RED}失败请求数:${NC} $fail_count"
    echo -e "${GREEN}测试时长:${NC} ${duration}s"
    echo -e "${GREEN}每秒请求数 (RPS):${NC} $rps"
    echo -e "${GREEN}成功率:${NC} $(echo "scale=2; $success_count * 100 / $total_requests" | bc 2>/dev/null || echo "0")%"
    echo ""
}

# 测试登录接口
test_login() {
    echo -e "${YELLOW}测试登录接口...${NC}"
    
    local start_time=$(date +%s%N)
    
    response=$(curl -s -X POST "$TARGET_URL/api/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","password":"123456"}' \
        -w "\n%{http_code}" 2>/dev/null)
    
    local end_time=$(date +%s%N)
    local duration=$(( (end_time - start_time) / 1000000 ))
    
    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n-1)
    
    echo -e "${GREEN}登录接口响应时间: ${duration}ms${NC}"
    echo -e "${GREEN}HTTP 状态码: $http_code${NC}"
    echo ""
}

# 测试渲染预热接口
test_preheat() {
    echo -e "${YELLOW}测试渲染预热接口...${NC}"
    
    # 先获取 token
    token=$(curl -s -X POST "$TARGET_URL/api/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","password":"123456"}' 2>/dev/null | jq -r '.data.token' 2>/dev/null)
    
    if [ -z "$token" ] || [ "$token" = "null" ]; then
        echo -e "${RED}获取 token 失败${NC}"
        return
    fi
    
    local start_time=$(date +%s%N)
    
    response=$(curl -s -X GET "$TARGET_URL/api/v1/preheat/stats" \
        -H "Authorization: Bearer $token" \
        -w "\n%{http_code}" 2>/dev/null)
    
    local end_time=$(date +%s%N)
    local duration=$(( (end_time - start_time) / 1000000 ))
    
    local http_code=$(echo "$response" | tail -n1)
    
    echo -e "${GREEN}预热接口响应时间: ${duration}ms${NC}"
    echo -e "${GREEN}HTTP 状态码: $http_code${NC}"
    echo ""
}

# 测试站点管理接口
test_sites() {
    echo -e "${YELLOW}测试站点管理接口...${NC}"
    
    # 先获取 token
    token=$(curl -s -X POST "$TARGET_URL/api/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","password":"123456"}' 2>/dev/null | jq -r '.data.token' 2>/dev/null)
    
    if [ -z "$token" ] || [ "$token" = "null" ]; then
        echo -e "${RED}获取 token 失败${NC}"
        return
    fi
    
    local start_time=$(date +%s%N)
    
    response=$(curl -s -X GET "$TARGET_URL/api/v1/sites" \
        -H "Authorization: Bearer $token" \
        -w "\n%{http_code}" 2>/dev/null)
    
    local end_time=$(date +%s%N)
    local duration=$(( (end_time - start_time) / 1000000 ))
    
    local http_code=$(echo "$response" | tail -n1)
    
    echo -e "${GREEN}站点管理接口响应时间: ${duration}ms${NC}"
    echo -e "${GREEN}HTTP 状态码: $http_code${NC}"
    echo ""
}

# 内存使用检查
check_memory() {
    echo -e "${YELLOW}检查系统内存使用...${NC}"
    
    if command -v free &> /dev/null; then
        free -h
    elif [ "$(uname)" = "Darwin" ]; then
        # macOS
        vm_stat | head -5
    fi
    echo ""
}

# 主函数
main() {
    check_dependencies
    health_check
    test_api_latency
    test_login
    test_preheat
    test_sites
    concurrent_test
    
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  测试完成${NC}"
    echo -e "${GREEN}========================================${NC}"
}

# 运行主函数
main
