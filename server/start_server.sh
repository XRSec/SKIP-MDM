#!/bin/bash

# MDM日志收集服务器启动脚本

echo "Starting MDM Log Collection Server..."

# 检查Go是否安装
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed"
    exit 1
fi

# 检查MySQL是否运行
if ! command -v mysql &> /dev/null; then
    echo "Warning: MySQL client not found. Please ensure MySQL server is running."
fi

# 设置环境变量
export CGO_ENABLED=1

# 进入服务器目录
cd "$(dirname "$0")"

# 初始化数据库（如果需要）
echo "Initializing database..."
# mysql -u root -p < init_db.sql

# 下载依赖
echo "Downloading dependencies..."
go mod tidy

# 编译并运行服务器
echo "Building and starting server..."
go build -o mdm-log-server log_server.go

if [ $? -eq 0 ]; then
    echo "Server built successfully. Starting..."
    ./mdm-log-server
else
    echo "Build failed!"
    exit 1
fi