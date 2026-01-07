#!/bin/bash

# Prerender Shield 双仓库同步脚本
# 用于将代码同步到 Gitee 和 GitHub

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查当前目录是否为 Git 仓库
check_git_repo() {
    if [ ! -d .git ]; then
        log_error "当前目录不是 Git 仓库"
        exit 1
    fi
}

# 检查远程仓库配置
check_remotes() {
    log_info "检查远程仓库配置..."
    
    # 检查 Gitee 远程仓库
    if ! git remote get-url origin &> /dev/null; then
        log_error "未找到 origin 远程仓库"
        exit 1
    fi
    
    GITEE_URL=$(git remote get-url origin)
    log_info "Gitee 仓库: $GITEE_URL"
    
    # 检查 GitHub 远程仓库
    if git remote get-url github &> /dev/null; then
        GITHUB_URL=$(git remote get-url github)
        log_info "GitHub 仓库: $GITHUB_URL"
    else
        log_warn "未找到 GitHub 远程仓库"
        read -p "是否要添加 GitHub 远程仓库？(y/n): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            read -p "请输入 GitHub 仓库 URL (例如: git@github.com:username/repo.git): " GITHUB_URL
            if [ -n "$GITHUB_URL" ]; then
                git remote add github "$GITHUB_URL"
                log_info "已添加 GitHub 远程仓库: $GITHUB_URL"
            else
                log_error "未提供 GitHub 仓库 URL"
                exit 1
            fi
        else
            log_info "跳过 GitHub 仓库配置"
        fi
    fi
}

# 配置双仓库推送
configure_dual_push() {
    log_info "配置双仓库推送..."
    
    # 检查当前推送配置
    CURRENT_PUSH_URL=$(git config --get remote.origin.pushurl 2>/dev/null || git config --get remote.origin.url)
    
    # 如果已经配置了多个推送 URL，跳过
    if git config --get-all remote.origin.pushurl 2>/dev/null | grep -q ","; then
        log_info "双仓库推送已配置"
        return
    fi
    
    # 获取 GitHub 仓库 URL
    if git remote get-url github &> /dev/null; then
        GITHUB_URL=$(git remote get-url github)
        
        # 配置同时推送到 origin (Gitee) 和 github
        git remote set-url --add --push origin "$GITEE_URL"
        git remote set-url --add --push origin "$GITHUB_URL"
        
        log_info "已配置双仓库推送:"
        log_info "  - Gitee:  $GITEE_URL"
        log_info "  - GitHub: $GITHUB_URL"
        log_info "现在执行 'git push' 会自动推送到两个仓库"
    else
        log_warn "未配置 GitHub 仓库，跳过双仓库推送配置"
    fi
}

# 拉取最新代码
pull_latest() {
    log_info "从 Gitee 拉取最新代码..."
    git pull origin master
    
    if git remote get-url github &> /dev/null; then
        log_info "从 GitHub 拉取最新代码..."
        git pull github master
    fi
}

# 推送代码到所有仓库
push_all() {
    log_info "推送代码到所有仓库..."
    
    # 检查是否有未提交的更改
    if [ -n "$(git status --porcelain)" ]; then
        log_warn "检测到未提交的更改"
        read -p "是否要提交所有更改？(y/n): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            git add .
            read -p "请输入提交信息: " COMMIT_MSG
            if [ -z "$COMMIT_MSG" ]; then
                COMMIT_MSG="Update $(date '+%Y-%m-%d %H:%M:%S')"
            fi
            git commit -m "$COMMIT_MSG"
        else
            log_error "请先提交更改"
            exit 1
        fi
    fi
    
    # 推送到所有配置的远程仓库
    log_info "执行 git push..."
    git push
    
    # 检查推送结果
    if [ $? -eq 0 ]; then
        log_info "代码推送成功"
    else
        log_error "代码推送失败"
        exit 1
    fi
}

# 手动分别推送
push_manual() {
    log_info "手动分别推送..."
    
    # 推送到 Gitee
    log_info "推送到 Gitee..."
    git push origin master
    
    # 推送到 GitHub
    if git remote get-url github &> /dev/null; then
        log_info "推送到 GitHub..."
        git push github master
    fi
}

# 显示仓库状态
show_status() {
    log_info "仓库状态:"
    echo "当前分支: $(git branch --show-current)"
    echo ""
    
    echo "远程仓库:"
    git remote -v
    echo ""
    
    echo "最近提交:"
    git log --oneline -5
    echo ""
    
    echo "未提交的更改:"
    git status --short
}

# 主菜单
main_menu() {
    echo "========================================"
    echo "  Prerender Shield 双仓库同步管理"
    echo "========================================"
    echo "1. 检查仓库配置"
    echo "2. 配置双仓库推送"
    echo "3. 拉取最新代码"
    echo "4. 推送代码到所有仓库"
    echo "5. 手动分别推送"
    echo "6. 显示仓库状态"
    echo "7. 退出"
    echo "========================================"
    
    read -p "请选择操作 (1-7): " choice
    
    case $choice in
        1)
            check_remotes
            ;;
        2)
            check_remotes
            configure_dual_push
            ;;
        3)
            check_remotes
            pull_latest
            ;;
        4)
            check_remotes
            push_all
            ;;
        5)
            check_remotes
            push_manual
            ;;
        6)
            show_status
            ;;
        7)
            log_info "退出"
            exit 0
            ;;
        *)
            log_error "无效的选择"
            ;;
    esac
    
    echo ""
    read -p "按 Enter 键继续..."
    main_menu
}

# 脚本使用说明
usage() {
    echo "使用方法:"
    echo "  $0 [command]"
    echo ""
    echo "命令:"
    echo "  setup     配置双仓库同步"
    echo "  pull      拉取最新代码"
    echo "  push      推送代码"
    echo "  status    显示仓库状态"
    echo "  menu      显示交互式菜单"
    echo "  help      显示帮助信息"
    echo ""
    echo "示例:"
    echo "  $0 setup  配置双仓库同步"
    echo "  $0 push   推送代码到所有仓库"
}

# 命令行参数处理
if [ $# -eq 0 ]; then
    # 如果没有参数，显示菜单
    check_git_repo
    main_menu
else
    check_git_repo
    
    case $1 in
        setup)
            check_remotes
            configure_dual_push
            ;;
        pull)
            check_remotes
            pull_latest
            ;;
        push)
            check_remotes
            push_all
            ;;
        status)
            show_status
            ;;
        menu)
            main_menu
            ;;
        help|--help|-h)
            usage
            ;;
        *)
            log_error "未知命令: $1"
            usage
            exit 1
            ;;
    esac
fi