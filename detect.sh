#!/bin/bash

# PrerenderShield 部署检测脚本

# 彩色输出定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_success() {
    echo -e "${GREEN}[✓] $1${NC}"
}

print_info() {
    echo -e "${BLUE}[i] $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}[!] $1${NC}"
}

print_error() {
    echo -e "${RED}[✗] $1${NC}" >&2
}

echo "========================================"
echo -e "${BLUE}PrerenderShield 部署检测脚本${NC}"
echo "========================================"

# 检查脚本是否存在且可执行
check_scripts() {
    print_info "检查部署脚本..."
    
    local scripts=("./build.sh" "./install.sh" "./start.sh")
    local all_good="true"
    
    for script in "${scripts[@]}"; do
        if [ -f "$script" ]; then
            if [ -x "$script" ]; then
                print_success "$script 存在且可执行"
            else
                print_warning "$script 存在但不可执行，正在添加执行权限..."
                chmod +x "$script"
                if [ $? -eq 0 ]; then
                    print_success "已添加执行权限"
                else
                    print_error "无法添加执行权限"
                    all_good="false"
                fi
            fi
        else
            print_error "$script 不存在"
            all_good="false"
        fi
    done
    
    if [ "$all_good" = "true" ]; then
        return 0
    else
        return 1
    fi
}

# 检查系统依赖
check_dependencies() {
    print_info "检查系统依赖..."
    
    local dependencies=()
    local all_good="true"
    
    # 检查Go环境
    if command -v go &> /dev/null; then
        print_success "Go环境已安装: $(go version)"
    else
        print_error "Go环境未安装"
        all_good="false"
    fi
    
    # 检查Node.js环境
    if command -v node &> /dev/null; then
        print_success "Node.js已安装: $(node --version)"
    else
        print_error "Node.js未安装"
        all_good="false"
    fi
    
    # 检查npm环境
    if command -v npm &> /dev/null; then
        print_success "npm已安装: $(npm --version)"
    else
        print_error "npm未安装"
        all_good="false"
    fi
    
    # 检查Redis环境
    if command -v redis-server &> /dev/null; then
        print_success "Redis已安装: $(redis-server --version 2>&1 | head -1)"
        # 检查Redis是否正在运行
        if redis-cli ping &> /dev/null; then
            print_success "Redis正在运行"
        else
            print_warning "Redis已安装但未运行"
        fi
    else
        print_error "Redis未安装"
        all_good="false"
    fi
    
    # 检查浏览器环境
    local chrome_available="false"
    if command -v google-chrome &> /dev/null || command -v chromium &> /dev/null || command -v chromium-browser &> /dev/null; then
        chrome_available="true"
    elif [ "$(uname -s)" = "Darwin" ]; then
        if [ -d "/Applications/Google Chrome.app" ] || [ -d "/Applications/Chromium.app" ]; then
            chrome_available="true"
        fi
    fi
    
    if [ "$chrome_available" = "true" ]; then
        print_success "Chrome/Chromium浏览器已安装"
    else
        print_error "Chrome/Chromium浏览器未安装"
        all_good="false"
    fi
    
    if [ "$all_good" = "true" ]; then
        return 0
    else
        return 1
    fi
}

# 检查构建过程
check_build() {
    print_info "检查构建过程..."
    
    if [ -f "./api" ]; then
        print_success "后端二进制文件已存在"
    else
        print_warning "后端二进制文件不存在，需要运行构建脚本"
    fi
    
    if [ -d "./web/dist" ]; then
        print_success "前端构建文件已存在"
    else
        print_warning "前端构建文件不存在，需要运行构建脚本"
    fi
    
    return 0
}

# 检查配置文件
check_config() {
    print_info "检查配置文件..."
    
    if [ -f "./configs/config.yml" ]; then
        print_success "配置文件已存在"
    else
        print_warning "配置文件不存在，启动时会从模板复制"
    fi
    
    return 0
}

# 检查服务状态
check_service() {
    print_info "检查服务状态..."
    
    local pid_file="./data/prerender-shield.pid"
    
    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if ps -p "$pid" > /dev/null 2>&1; then
            print_success "服务正在运行，PID: $pid"
            
            # 检查API服务
            if curl -s http://localhost:9598/api/v1/health > /dev/null 2>&1; then
                print_success "API服务运行正常: http://localhost:9598"
            else
                print_warning "API服务可能存在问题，无法访问健康检查接口"
            fi
            
            # 检查管理控制台
            if curl -s http://localhost:9597 > /dev/null 2>&1; then
                print_success "管理控制台运行正常: http://localhost:9597"
            else
                print_warning "管理控制台可能存在问题，无法访问"
            fi
        else
            print_warning "PID文件存在但进程不存在，服务可能已停止"
            rm -f "$pid_file"
        fi
    else
        print_warning "服务未运行，PID文件不存在"
    fi
    
    return 0
}

# 主函数
main() {
    print_info "开始检测PrerenderShield部署环境..."
    
    check_scripts
    local scripts_status=$?
    
    check_dependencies
    local dependencies_status=$?
    
    check_build
    local build_status=$?
    
    check_config
    local config_status=$?
    
    check_service
    local service_status=$?
    
    echo "========================================"
    echo -e "${BLUE}检测结果汇总${NC}"
    echo "========================================"
    
    # 服务状态不是部署环境的必要条件，只检查脚本、依赖、构建和配置
    if [ $scripts_status -eq 0 ] && [ $dependencies_status -eq 0 ] && [ $build_status -eq 0 ] && [ $config_status -eq 0 ]; then
        print_success "✅ 部署环境检测通过，可以开始部署"
        echo ""
        print_info "部署步骤："
        print_info "1. 运行构建脚本：./build.sh"
        print_info "2. 运行安装脚本：./install.sh"
        print_info "3. 运行启动脚本：./start.sh start"
        echo ""
        
        # 显示服务状态信息
        if [ -f "./data/prerender-shield.pid" ]; then
            local pid=$(cat "./data/prerender-shield.pid")
            if ps -p "$pid" > /dev/null 2>&1; then
                print_success "服务正在运行，PID: $pid"
            else
                print_warning "注意：PID文件存在但进程不存在，启动服务前会自动清理"
            fi
        else
            print_info "服务未运行，启动脚本将创建新的服务实例"
        fi
        echo ""
    else
        print_error "❌ 部署环境检测未通过，请检查上述错误"
    fi
    
    echo "========================================"
}

# 执行主函数
main