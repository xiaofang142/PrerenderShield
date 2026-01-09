#!/bin/bash

# PrerenderShield 脚本测试工具

APP_NAME="prerender-shield"

# 彩色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 测试结果统计
PASSED=0
FAILED=0
TOTAL=0

# 打印彩色信息
print_header() {
    echo -e "${BLUE}"
    echo "===================================================================="
    echo "PrerenderShield 脚本测试工具"
    echo "===================================================================="
    echo -e "${NC}"
}

print_success() {
    echo -e "${GREEN}[✓] $1${NC}"
    ((PASSED++))
    ((TOTAL++))
}

print_info() {
    echo -e "${BLUE}[i] $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}[!] $1${NC}"
}

print_error() {
    echo -e "${RED}[✗] $1${NC}" >&2
    ((FAILED++))
    ((TOTAL++))
}

print_test_result() {
    echo -e "${BLUE}"
    echo "--------------------------------------------------------------------"
    echo "测试结果汇总"
    echo "--------------------------------------------------------------------"
    echo -e "${NC}"
    echo -e "${GREEN}通过: $PASSED${NC}"
    echo -e "${RED}失败: $FAILED${NC}"
    echo -e "${BLUE}总计: $TOTAL${NC}"
    echo ""
    
    if [ $FAILED -eq 0 ]; then
        echo -e "${GREEN}✅ 所有测试通过！脚本体系兼容良好${NC}"
    else
        echo -e "${RED}❌ 部分测试失败，需要进一步修复${NC}"
        return 1
    fi
    
    return 0
}

# 测试脚本语法
run_syntax_test() {
    local script=$1
    local script_name=$(basename "$script")
    
    print_info "测试脚本语法: $script_name"
    
    if bash -n "$script" 2>/dev/null; then
        print_success "脚本语法检查通过: $script_name"
        return 0
    else
        print_error "脚本语法检查失败: $script_name"
        bash -n "$script" >&2
        return 1
    fi
}

# 测试脚本帮助信息
run_help_test() {
    local script=$1
    local script_name=$(basename "$script")
    
    print_info "测试脚本帮助信息: $script_name"
    
    if "$script" --help 2>&1 | grep -q "Usage\|使用方法"; then
        print_success "脚本帮助信息测试通过: $script_name"
        return 0
    elif "$script" -h 2>&1 | grep -q "Usage\|使用方法"; then
        print_success "脚本帮助信息测试通过: $script_name"
        return 0
    else
        print_warning "脚本帮助信息不规范: $script_name"
        # 非致命错误，继续测试
        return 0
    fi
}

# 测试脚本基本功能
run_basic_test() {
    local script=$1
    local script_name=$(basename "$script")
    
    print_info "测试脚本基本功能: $script_name"
    
    case "$script_name" in
        "build.sh")
            # 测试build.sh的--help或-h参数
            if "$script" --help 2>&1 | grep -q "构建" || \
               "$script" -h 2>&1 | grep -q "构建"; then
                print_success "build.sh 帮助功能测试通过"
            else
                print_warning "build.sh 帮助功能不规范"
            fi
            ;;
            
        "install.sh")
            # 测试install.sh的--help或-h参数
            if "$script" --help 2>&1 | grep -q "安装" || \
               "$script" -h 2>&1 | grep -q "安装"; then
                print_success "install.sh 帮助功能测试通过"
            else
                print_warning "install.sh 帮助功能不规范"
            fi
            ;;
            
        "start.sh")
            # 测试start.sh的--help或-h参数
            if "$script" --help 2>&1 | grep -q "启动" || \
               "$script" -h 2>&1 | grep -q "启动"; then
                print_success "start.sh 帮助功能测试通过"
            else
                print_warning "start.sh 帮助功能不规范"
            fi
            
            # 测试start.sh的status命令
            local status_output=$(./$script status 2>&1)
            echo "调试信息: start.sh status输出为: $status_output"
            if echo "$status_output" | grep -q "没有在运行中\|正在运行\|prerender-shield"; then
                print_success "start.sh status命令测试通过"
            else
                print_error "start.sh status命令测试失败"
                echo "调试信息: 输出中没有匹配到预期内容"
            fi
            ;;
            
        *)
            print_warning "未知脚本类型: $script_name"
            ;;
    esac
    
    return 0
}

# 测试脚本跨平台兼容性
run_platform_test() {
    local script=$1
    local script_name=$(basename "$script")
    
    print_info "测试脚本跨平台兼容性: $script_name"
    
    # 检查脚本中是否使用了跨平台不兼容的命令
    local incompatible_commands=(
        "apt-get\|yum\|dnf\|pacman\|zypper\|apk"  # 包管理器命令
        "systemctl\|service\|rc-update"              # 服务管理命令
        "rm -rf /tmp/test"                            # 危险命令
    )
    
    local has_incompatible=false
    
    for cmd in "${incompatible_commands[@]}"; do
        if grep -q -E "$cmd" "$script" 2>/dev/null; then
            print_warning "脚本 $script_name 包含潜在的跨平台不兼容命令: $cmd"
            has_incompatible=true
        fi
    done
    
    if [ "$has_incompatible" = false ]; then
        print_success "脚本 $script_name 跨平台兼容性检查通过"
    else
        print_warning "脚本 $script_name 包含潜在的跨平台不兼容命令，需要进一步检查"
        # 非致命错误，继续测试
        return 0
    fi
    
    return 0
}

# 测试脚本错误处理
run_error_test() {
    local script=$1
    local script_name=$(basename "$script")
    
    print_info "测试脚本错误处理: $script_name"
    
    # 检查脚本中是否包含错误处理机制
    local error_handling_commands=(
        "set -e\|set -o errexit"           # 错误退出
        "set -u\|set -o nounset"           # 未定义变量退出
        "set -o pipefail"                  # 管道错误退出
        "trap"                              # 信号处理
        "exit 1"                            # 错误退出
        "return 1"                          # 函数错误返回
    )
    
    local has_error_handling=false
    
    for cmd in "${error_handling_commands[@]}"; do
        if grep -q -E "$cmd" "$script" 2>/dev/null; then
            has_error_handling=true
            break
        fi
    done
    
    if [ "$has_error_handling" = true ]; then
        print_success "脚本 $script_name 包含错误处理机制"
    else
        print_warning "脚本 $script_name 缺少错误处理机制，建议添加"
        # 非致命错误，继续测试
        return 0
    fi
    
    return 0
}

# 主测试函数
run_tests() {
    local scripts=($@)
    
    print_header
    
    # 获取当前平台信息
    local os=$(uname -s)
    local arch=$(uname -m)
    print_info "当前平台: $os $arch"
    echo ""
    
    # 测试每个脚本
    for script in "${scripts[@]}"; do
        if [ -f "$script" ]; then
            echo -e "${BLUE}"
            echo "--------------------------------------------------------------------"
            echo "测试脚本: $script"
            echo "--------------------------------------------------------------------"
            echo -e "${NC}"
            
            run_syntax_test "$script"
            run_help_test "$script"
            run_basic_test "$script"
            run_platform_test "$script"
            run_error_test "$script"
            
            echo ""
        else
            print_error "脚本不存在: $script"
        fi
    done
    
    # 测试结果汇总
    print_test_result
    
    return $?
}

# 主程序
if [ $# -eq 0 ]; then
    # 默认测试所有脚本
    run_tests "build.sh" "install.sh" "start.sh"
else
    # 测试指定脚本
    run_tests "$@"
fi
