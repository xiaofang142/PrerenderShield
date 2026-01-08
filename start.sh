#!/bin/bash

# PrerenderShield 一键安装部署脚本

APP_NAME="prerender-shield"
CONFIG_FILE="configs/config.yml"

# 根据当前平台选择合适的二进制文件
get_platform_binary() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    
    # 转换架构名称
    if [[ $arch == "x86_64" ]]; then
        arch="amd64"
    elif [[ $arch == "arm64" ]]; then
        arch="arm64"
    fi
    
    # 构建二进制文件路径
    local binary_path="bin/${os}-${arch}/api"
    
    # 如果是Windows系统，添加.exe后缀
    if [[ $os == "windows" ]]; then
        binary_path="${binary_path}.exe"
    fi
    
    echo "$binary_path"
}

# 获取当前平台的二进制文件路径
APP_BINARY=$(get_platform_binary)

# 如果当前平台的二进制文件不存在，使用当前目录下的api
if [ ! -f "$APP_BINARY" ]; then
    APP_BINARY="./api"
fi

PID_FILE="./data/${APP_NAME}.pid"
LOG_FILE="./data/${APP_NAME}.log"

# 彩色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 打印彩色信息
info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

usage() {
    echo "========================================"
    echo -e "${GREEN}PrerenderShield 一键安装部署脚本${NC}"
    echo "========================================"
    echo "用法: $0 {start|check|restart|stop|reinstall}"
    echo ""
    echo "选项:"
    echo "  start      检查依赖并启动应用程序"
    echo "  check      仅检查系统依赖（不启动应用）"
    echo "  restart    重启应用程序"
    echo "  stop       停止应用程序"
    echo "  reinstall  重新安装应用程序（清除数据）"
    echo ""
    exit 1
}

check_root() {
    if [ "$EUID" -eq 0 ]; then
        warning "正在以root用户运行，这可能不是最佳实践"
    fi
}

# 系统依赖安装
install_deps() {
    echo "========================================"
    echo -e "${GREEN}安装系统依赖...${NC}"
    echo "======================================="
    
    local os_type=$(uname -s)
    local install_cmd
    
    # 检测包管理器
    if [ "$os_type" = "Linux" ]; then
        if command -v apt-get &> /dev/null; then
            install_cmd="apt-get"
            sudo apt-get update
        elif command -v yum &> /dev/null; then
            install_cmd="yum"
        elif command -v dnf &> /dev/null; then
            install_cmd="dnf"
        else
            error "不支持的Linux发行版，无法自动安装依赖"
            return 1
        fi
    elif [ "$os_type" = "Darwin" ]; then
        # macOS
        if ! command -v brew &> /dev/null; then
            info "安装Homebrew..."
            /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
            if [ $? -ne 0 ]; then
                error "Homebrew安装失败"
                return 1
            fi
            # 添加Homebrew到PATH
            if [ -f "/opt/homebrew/bin/brew" ]; then
                # Apple Silicon
                export PATH="/opt/homebrew/bin:$PATH"
            elif [ -f "/usr/local/bin/brew" ]; then
                # Intel
                export PATH="/usr/local/bin:$PATH"
            fi
        fi
        install_cmd="brew"
    else
        error "不支持的操作系统，无法自动安装依赖"
        return 1
    fi
    
    # 安装Go环境
    local go_installed=false
    if command -v go &> /dev/null; then
        go_installed=true
    elif [ "$os_type" = "Linux" ]; then
        # Linux额外检查包管理器
        case "$install_cmd" in
            apt-get)
                dpkg -l | grep -q golang && go_installed=true
                ;;
            yum|dnf)
                rpm -q golang &> /dev/null && go_installed=true
                ;;
        esac
    elif [ "$os_type" = "Darwin" ] && brew list go &> /dev/null; then
        go_installed=true
    fi
    
    if [ "$go_installed" = false ]; then
        info "安装Go环境..."
        if [ "$os_type" = "Linux" ]; then
            case "$install_cmd" in
                apt-get)
                    sudo apt-get install -y golang
                    if [ $? -ne 0 ]; then
                        error "Go环境安装失败"
                        return 1
                    fi
                    ;;
                yum|dnf)
                    sudo $install_cmd install -y golang
                    if [ $? -ne 0 ]; then
                        error "Go环境安装失败"
                        return 1
                    fi
                    ;;
            esac
        elif [ "$os_type" = "Darwin" ]; then
            brew install go
            if [ $? -ne 0 ]; then
                error "Go环境安装失败"
                return 1
            fi
        fi
        info "✓ Go环境安装完成"
        
        # 验证Go是否可用
        if ! command -v go &> /dev/null; then
            error "Go环境安装后验证失败，无法找到go命令"
            return 1
        fi
    else
        info "✓ Go环境已安装: $(go version)"
    fi
    
    # 安装Redis
    local redis_installed=false
    if command -v redis-server &> /dev/null; then
        redis_installed=true
    elif [ "$os_type" = "Linux" ]; then
        # Linux额外检查包管理器
        case "$install_cmd" in
            apt-get)
                dpkg -l | grep -q redis-server && redis_installed=true
                ;;
            yum|dnf)
                rpm -q redis &> /dev/null && redis_installed=true
                ;;
        esac
    elif [ "$os_type" = "Darwin" ] && brew list redis &> /dev/null; then
        redis_installed=true
    fi
    
    if [ "$redis_installed" = false ]; then
        info "安装Redis..."
        if [ "$os_type" = "Linux" ]; then
            case "$install_cmd" in
                apt-get)
                    sudo apt-get install -y redis-server
                    if [ $? -ne 0 ]; then
                        error "Redis安装失败"
                        return 1
                    fi
                    sudo systemctl enable --now redis-server
                    ;;
                yum|dnf)
                    sudo $install_cmd install -y redis
                    if [ $? -ne 0 ]; then
                        error "Redis安装失败"
                        return 1
                    fi
                    sudo systemctl enable --now redis
                    ;;
            esac
        elif [ "$os_type" = "Darwin" ]; then
            brew install redis
            if [ $? -ne 0 ]; then
                error "Redis安装失败"
                return 1
            fi
            brew services start redis
        fi
        info "✓ Redis安装完成"
        
        # 验证Redis是否可用
        if ! command -v redis-server &> /dev/null; then
            error "Redis安装后验证失败，无法找到redis-server命令"
            return 1
        fi
    else
        info "✓ Redis已安装"
        # 启动Redis如果它没有运行
        if ! redis-cli ping &> /dev/null; then
            info "启动Redis..."
            if [ "$os_type" = "Linux" ]; then
                sudo systemctl start redis-server 2>/dev/null || sudo systemctl start redis
            elif [ "$os_type" = "Darwin" ]; then
                brew services start redis
            fi
        fi
    fi
    
    # 安装Node.js和npm
    local node_installed=false
    if command -v npm &> /dev/null; then
        node_installed=true
    elif [ "$os_type" = "Linux" ]; then
        # Linux额外检查包管理器
        case "$install_cmd" in
            apt-get)
                dpkg -l | grep -q nodejs && node_installed=true
                ;;
            yum|dnf)
                rpm -q nodejs &> /dev/null && node_installed=true
                ;;
        esac
    elif [ "$os_type" = "Darwin" ] && brew list node &> /dev/null; then
        node_installed=true
    fi
    
    if [ "$node_installed" = false ]; then
        info "安装Node.js和npm..."
        if [ "$os_type" = "Linux" ]; then
            case "$install_cmd" in
                apt-get)
                    sudo apt-get install -y nodejs npm
                    if [ $? -ne 0 ]; then
                        error "Node.js和npm安装失败"
                        return 1
                    fi
                    ;;
                yum|dnf)
                    sudo $install_cmd install -y nodejs npm
                    if [ $? -ne 0 ]; then
                        error "Node.js和npm安装失败"
                        return 1
                    fi
                    ;;
            esac
        elif [ "$os_type" = "Darwin" ]; then
            brew install node
            if [ $? -ne 0 ]; then
                error "Node.js和npm安装失败"
                return 1
            fi
        fi
        info "✓ Node.js和npm安装完成"
        
        # 验证Node.js和npm是否可用
        if ! command -v npm &> /dev/null; then
            error "Node.js和npm安装后验证失败，无法找到npm命令"
            return 1
        fi
    else
        info "✓ Node.js和npm已安装: $(npm --version)"
    fi
    
    # 安装Chrome/Chromium浏览器（用于预渲染）
    local chrome_installed=false
    
    # 检查Chrome是否已安装（Linux检查命令行工具和包管理器，macOS检查应用目录和brew）
    if [ "$os_type" = "Linux" ]; then
        if command -v google-chrome &> /dev/null || command -v chromium &> /dev/null || command -v chromium-browser &> /dev/null; then
            chrome_installed=true
        else
            # Linux额外检查包管理器
            case "$install_cmd" in
                apt-get)
                    dpkg -l | grep -q chromium-browser && chrome_installed=true
                    ;;
                yum|dnf)
                    rpm -q chromium &> /dev/null && chrome_installed=true
                    ;;
            esac
        fi
    elif [ "$os_type" = "Darwin" ]; then
        if [ -d "/Applications/Google Chrome.app" ] || [ -d "/Applications/Chromium.app" ]; then
            chrome_installed=true
        elif brew list --cask google-chrome &> /dev/null || brew list --cask chromium &> /dev/null; then
            chrome_installed=true
        fi
    fi
    
    if [ "$chrome_installed" = false ]; then
        info "安装Chrome/Chromium浏览器..."
        if [ "$os_type" = "Linux" ]; then
            case "$install_cmd" in
                apt-get)
                    sudo apt-get install -y chromium-browser
                    if [ $? -ne 0 ]; then
                        error "浏览器安装失败"
                        return 1
                    fi
                    ;;
                yum|dnf)
                    sudo $install_cmd install -y chromium
                    if [ $? -ne 0 ]; then
                        error "浏览器安装失败"
                        return 1
                    fi
                    ;;
            esac
        elif [ "$os_type" = "Darwin" ]; then
            brew install --cask google-chrome
            if [ $? -ne 0 ]; then
                error "浏览器安装失败"
                return 1
            fi
        fi
        info "✓ 浏览器安装完成"
        
        # 验证浏览器是否可用
        local chrome_verified=false
        if [ "$os_type" = "Linux" ]; then
            if command -v google-chrome &> /dev/null || command -v chromium &> /dev/null || command -v chromium-browser &> /dev/null; then
                chrome_verified=true
            fi
        elif [ "$os_type" = "Darwin" ]; then
            if [ -d "/Applications/Google Chrome.app" ] || [ -d "/Applications/Chromium.app" ]; then
                chrome_verified=true
            fi
        fi
        
        if [ "$chrome_verified" = false ]; then
            error "浏览器安装后验证失败，无法找到浏览器"
            return 1
        fi
    else
        if [ "$os_type" = "Linux" ]; then
            if command -v google-chrome &> /dev/null; then
                info "✓ Chrome浏览器已安装: $(google-chrome --version)"
            elif command -v chromium &> /dev/null; then
                info "✓ Chromium浏览器已安装: $(chromium --version)"
            elif command -v chromium-browser &> /dev/null; then
                info "✓ Chromium浏览器已安装: $(chromium-browser --version)"
            else
                info "✓ 浏览器已安装"
            fi
        else
            if [ -d "/Applications/Google Chrome.app" ]; then
                info "✓ Chrome浏览器已安装"
            else
                info "✓ Chromium浏览器已安装"
            fi
        fi
    fi
    
    echo "========================================"
    info "所有依赖安装完成！"
    echo "======================================="
    return 0
}

create_dirs() {
    info "创建必要的目录..."
    mkdir -p data/rules data/certs data/redis data/grafana
    info "✓ 目录创建完成"
    
    # 创建Redis配置文件
    if [ ! -f "data/redis/redis.conf" ]; then
        info "创建Redis配置文件..."
        cat > data/redis/redis.conf << EOF
# Redis配置文件
bind 0.0.0.0
protected-mode no
port 6379
dir /data
dbfilename dump.rdb
save 900 1
save 300 10
save 60 10000
appendonly yes
appendfilename "appendonly.aof"
EOF
        info "✓ Redis配置文件创建完成"
    fi
}

check_config() {
    if [ ! -f "$CONFIG_FILE" ]; then
        info "配置文件不存在，从模板复制..."
        if [ -f configs/config.example.yml ]; then
            cp configs/config.example.yml "$CONFIG_FILE"
            info "✓ 已从config.example.yml复制到$CONFIG_FILE"
            
            # 修改默认配置，使用内存存储和缓存
            info "优化默认配置..."
            sed -i '' 's/type: redis/type: memory/' "$CONFIG_FILE"
            sed -i '' 's/type: postgres/type: memory/' "$CONFIG_FILE"
            sed -i '' 's/redis_url: 127.0.0.1:6379/redis_url: /' "$CONFIG_FILE"
            info "✓ 配置优化完成"
        else
            error "配置文件模板configs/config.example.yml不存在"
            exit 1
        fi
    fi
}

build_app() {
    info "配置Go镜像加速..."
    export GOPROXY=https://goproxy.cn,direct
    export GO111MODULE=on

    info "安装Go依赖..."
    go mod tidy
    if [ $? -ne 0 ]; then
        error "Go依赖安装失败"
        exit 1
    fi
    info "✓ Go依赖安装完成"

    info "构建应用程序..."
    go build -o "$APP_BINARY" ./cmd/api
    if [ $? -ne 0 ]; then
        error "应用程序构建失败"
        exit 1
    fi
    
    # 添加执行权限
    chmod +x "$APP_BINARY"
    info "✓ 应用程序构建成功，并添加了执行权限"
}

is_running() {
    if [ -f "$PID_FILE" ]; then
        local pid=$(cat "$PID_FILE")
        if ps -p "$pid" > /dev/null 2>&1; then
            return 0 # 运行中
        else
            rm -f "$PID_FILE" # PID文件存在但进程不存在，删除PID文件
        fi
    fi
    return 1 # 未运行
}

start() {
    if is_running; then
        info "$APP_NAME 已经在运行中"
        return 0
    fi

    info "启动$APP_NAME..."
    echo "========================================"
    echo -e "${GREEN}应用程序服务启动信息${NC}"
    echo "======================================="
    echo "管理控制台: http://0.0.0.0:9597"
    echo "API服务: http://0.0.0.0:9598"
    echo "健康检查接口: http://0.0.0.0:9598/api/v1/health"
    echo "版本信息接口: http://0.0.0.0:9598/api/v1/version"
    echo "======================================="

    # 启动应用程序，确保工作目录正确
    cd "$(dirname "$0")" && nohup "$APP_BINARY" --config "$CONFIG_FILE" > "$LOG_FILE" 2>&1 &
    echo $! > "$PID_FILE"
    info "$APP_NAME 启动成功，PID: $(cat "$PID_FILE")"
    info "日志文件: $LOG_FILE"
    info "查看日志: tail -f $LOG_FILE"
}

stop() {
    if ! is_running; then
        info "$APP_NAME 没有在运行中"
        return 0
    fi

    local pid=$(cat "$PID_FILE")
    info "停止$APP_NAME，PID: $pid..."
    kill "$pid"

    # 等待进程退出
    local count=0
    while is_running && [ $count -lt 10 ]; do
        sleep 1
        count=$((count + 1))
    done

    if is_running; then
        warning "进程未正常退出，尝试强制终止..."
        kill -9 "$pid"
        sleep 1
    fi

    if ! is_running; then
        info "$APP_NAME 已停止"
        rm -f "$PID_FILE"
    else
        error "无法停止$APP_NAME"
        exit 1
    fi
}

restart() {
    stop
    sleep 2
    start
}

reinstall() {
    info "重新安装$APP_NAME..."
    stop
    
    # 清除数据（保留配置文件）
    info "清除数据..."
    rm -rf ./data/*
    mkdir -p data/rules data/certs data/redis data/grafana
    
    # 重新构建
    build_app
    
    # 启动
    start
}

# 主程序
if [ $# -eq 0 ]; then
    usage
fi

check_root

case "$1" in
    check)
        install_deps || exit 1
        ;;
    start)
        install_deps || exit 1
        create_dirs
        check_config
        build_app
        start
        ;;
    restart)
        restart
        ;;
    stop)
        stop
        ;;
    reinstall)
        install_deps || exit 1
        reinstall
        ;;
    *)
        usage
        ;;
esac
