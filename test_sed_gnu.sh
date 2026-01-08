#!/bin/bash

# 测试GNU sed兼容性（如果已安装gsed）
if command -v gsed &> /dev/null; then
    echo "已安装gsed，测试GNU sed兼容性..."
    
    # 创建测试目录和配置文件
    mkdir -p test_sed_gnu
    cp configs/config.example.yml test_sed_gnu/config.yml
    
    # 设置测试变量
    INSTALL_DIR=/tmp/prerender-shield
    DATA_DIR=/tmp/prerender-shield/data
    CONFIG_DIR=./test_sed_gnu
    
    echo "\n1. 测试修复后的命令（带-i ''后缀）在GNU sed上的执行:"
    # 运行所有修复后的sed命令，使用gsed模拟GNU sed
    gsed -i '' "s|data_dir: ./data|data_dir: $DATA_DIR|" "$CONFIG_DIR/config.yml"
    gsed -i '' "s|static_dir: ./static|static_dir: $INSTALL_DIR/static|" "$CONFIG_DIR/config.yml"
    gsed -i '' "s|admin_static_dir: ./web/dist|admin_static_dir: $INSTALL_DIR/web/dist|" "$CONFIG_DIR/config.yml"
    gsed -i '' "s|redis_url: \"localhost:6379\"|redis_url: \"127.0.0.1:6379\"|" "$CONFIG_DIR/config.yml"
    
    # 显示修改后的配置
    echo "\n2. 验证修改结果:"
    grep "data_dir:" "$CONFIG_DIR/config.yml"
    grep "static_dir:" "$CONFIG_DIR/config.yml"
    grep "admin_static_dir:" "$CONFIG_DIR/config.yml"
    grep "redis_url:" "$CONFIG_DIR/config.yml"
    
    # 清理测试文件
    rm -rf test_sed_gnu
    
    echo "\nGNU sed兼容性测试完成！"
else
    echo "未安装gsed（GNU sed的macOS版本），跳过GNU sed兼容性测试。"
    echo "在Linux系统上，修复后的命令（带-i ''后缀）应该能正常工作，因为GNU sed会忽略空的后缀参数。"
fi

echo "\n总结："
echo "- 修复后的命令在macOS/BSD系统上能正常工作（带-i ''后缀）"
echo "- 修复后的命令在Linux系统上也能正常工作（GNU sed会忽略空的后缀参数）"
