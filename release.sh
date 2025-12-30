#!/bin/bash

# PrerenderShield 一键发布脚本

echo "========================================"
echo "PrerenderShield 一键发布脚本"
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

# 检查Node.js是否安装
if ! command -v node &> /dev/null; then
    echo "错误: 未安装Node.js，无法编译前端代码"
    exit 1
fi

# 检查npm是否安装
if ! command -v npm &> /dev/null; then
    echo "错误: 未安装npm，无法编译前端代码"
    exit 1
fi

# 定义构建输出目录
BUILD_DIR="./build"

# 清理旧的构建文件
echo "清理旧的构建文件..."
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"/backend "$BUILD_DIR"/frontend

# 构建多端Go代码
echo "========================================"
echo "构建多端Go代码..."
echo "========================================"

# 定义要构建的平台和架构
PLATFORMS=("linux/amd64" "linux/arm64" "darwin/amd64" "darwin/arm64" "windows/amd64")

for PLATFORM in "${PLATFORMS[@]}"; do
    # 分割平台和架构
    IFS="/" read -r OS ARCH <<< "$PLATFORM"
    
    # 构建输出文件名
    OUTPUT_NAME="prerender-shield"
    if [ "$OS" = "windows" ]; then
        OUTPUT_NAME="$OUTPUT_NAME.exe"
    fi
    
    # 构建输出路径
    OUTPUT_PATH="$BUILD_DIR/backend/$OS-$ARCH/$OUTPUT_NAME"
    
    echo "构建 $OS/$ARCH..."
    
    # 设置Go环境变量
    export GOOS="$OS"
    export GOARCH="$ARCH"
    export CGO_ENABLED=0
    
    # 创建输出目录
    mkdir -p "$(dirname "$OUTPUT_PATH")"
    
    # 构建应用程序
    go build -o "$OUTPUT_PATH" ./cmd/api
    
    if [ $? -ne 0 ]; then
        echo "错误: 构建 $OS/$ARCH 失败"
        exit 1
    fi
    
    echo "构建 $OS/$ARCH 成功: $OUTPUT_PATH"
done

# 编译前端代码
echo "========================================"
echo "编译前端代码..."
echo "========================================"

# 进入前端目录
cd ./web || exit 1

# 安装前端依赖
echo "安装前端依赖..."
npm install

if [ $? -ne 0 ]; then
    echo "错误: 安装前端依赖失败"
    exit 1
fi

# 编译前端代码
echo "编译前端代码..."
npm run build

if [ $? -ne 0 ]; then
    echo "错误: 编译前端代码失败"
    exit 1
fi

# 复制编译后的前端代码到构建目录
cd ..
echo "复制前端构建文件到 $BUILD_DIR/frontend..."
cp -r ./web/dist "$BUILD_DIR/frontend"

echo "========================================"
echo "一键发布脚本执行成功！"
echo "========================================"
echo "构建结果："
echo "  后端多端构建文件: $BUILD_DIR/backend/"
echo "  前端构建文件: $BUILD_DIR/frontend/"
echo ""
echo "你可以使用以下命令启动应用："
echo "  ./start.sh start    # 启动应用"
echo "  docker-compose up   # 使用Docker启动应用"
echo "========================================"
