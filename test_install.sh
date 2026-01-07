#!/bin/bash

# PrerenderShield 安装脚本测试
# 此脚本用于验证安装脚本的基本功能，不实际执行安装

set -euo pipefail

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

print_success() {
    echo -e "${GREEN}[✓] $1${NC}"
}

print_error() {
    echo -e "${RED}[✗] $1${NC}" >&2
}

print_info() {
    echo -e "[i] $1"
}

# 测试1: 语法检查
test_syntax() {
    print_info "测试1: 检查脚本语法..."
    
    for script in install.sh uninstall.sh start.sh; do
        if [[ -f "$script" ]]; then
            if bash -n "$script"; then
                print_success "$script 语法检查通过"
            else
                print_error "$script 语法检查失败"
                return 1
            fi
        else
            print_error "脚本 $script 不存在"
            return 1
        fi
    done
    
    return 0
}

# 测试2: 函数定义检查
test_functions() {
    print_info "测试2: 检查关键函数定义..."
    
    local required_functions=(
        "print_header"
        "print_success"
        "print_error"
        "detect_os"
        "install_dependencies"
        "setup_configuration"
    )
    
    for func in "${required_functions[@]}"; do
        if grep -q "^$func()" install.sh; then
            print_success "函数 $func 已定义"
        else
            print_error "函数 $func 未定义"
            return 1
        fi
    done
    
    return 0
}

# 测试3: 配置文件检查
test_config_files() {
    print_info "测试3: 检查配置文件模板..."
    
    local required_files=(
        "configs/config.example.yml"
    )
    
    for file in "${required_files[@]}"; do
        if [[ -f "$file" ]]; then
            print_success "配置文件 $file 存在"
        else
            print_error "配置文件 $file 不存在"
            return 1
        fi
    done
    
    return 0
}

# 测试4: 安装目录结构验证
test_directory_structure() {
    print_info "测试4: 验证目录结构定义..."
    
    # 检查install.sh中定义的目录
    if grep -q 'INSTALL_DIR="/opt/${APP_NAME}"' install.sh; then
        print_success "安装目录定义正确"
    else
        print_error "安装目录定义错误"
        return 1
    fi
    
    if grep -q 'CONFIG_DIR="/etc/${APP_NAME}"' install.sh; then
        print_success "配置目录定义正确"
    else
        print_error "配置目录定义错误"
        return 1
    fi
    
    return 0
}

# 测试5: 卸载脚本功能检查
test_uninstall_functions() {
    print_info "测试5: 检查卸载脚本功能..."
    
    local required_functions=(
        "stop_services"
        "remove_installation_files"
        "confirm_uninstall"
    )
    
    for func in "${required_functions[@]}"; do
        if grep -q "^$func()" uninstall.sh; then
            print_success "卸载函数 $func 已定义"
        else
            print_error "卸载函数 $func 未定义"
            return 1
        fi
    done
    
    return 0
}

# 主测试函数
main() {
    echo "========================================"
    echo "PrerenderShield 安装脚本测试"
    echo "========================================"
    
    local tests_passed=0
    local tests_failed=0
    
    # 运行所有测试
    if test_syntax; then
        ((tests_passed++))
    else
        ((tests_failed++))
    fi
    
    if test_functions; then
        ((tests_passed++))
    else
        ((tests_failed++))
    fi
    
    if test_config_files; then
        ((tests_passed++))
    else
        ((tests_failed++))
    fi
    
    if test_directory_structure; then
        ((tests_passed++))
    else
        ((tests_failed++))
    fi
    
    if test_uninstall_functions; then
        ((tests_passed++))
    else
        ((tests_failed++))
    fi
    
    # 输出测试结果
    echo "========================================"
    echo "测试完成"
    echo "通过: $tests_passed"
    echo "失败: $tests_failed"
    echo "========================================"
    
    if [[ $tests_failed -eq 0 ]]; then
        print_success "所有测试通过！安装脚本准备就绪。"
        echo ""
        echo "下一步："
        echo "1. 查看安装文档: cat INSTALL.md"
        echo "2. 运行安装测试: sudo ./install.sh (实际安装)"
        echo "3. 或运行dry-run: sudo ./install.sh --dry-run"
        return 0
    else
        print_error "部分测试失败，请检查安装脚本。"
        return 1
    fi
}

# 执行测试
main "$@"