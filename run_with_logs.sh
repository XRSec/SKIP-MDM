#!/bin/bash

# MDM客户端日志收集运行脚本

echo "MDM Assistant with Log Collection"
echo "================================="

# 检查是否为root用户
if [ "$EUID" -ne 0 ]; then
    echo "Error: Please run as root user"
    echo "Usage: sudo ./run_with_logs.sh [SERIAL_NUMBER]"
    exit 1
fi

# 获取序列号参数
SERIAL_NUMBER=""
if [ ! -z "$1" ]; then
    SERIAL_NUMBER="$1"
else
    echo "Please enter your device serial number:"
    read SERIAL_NUMBER
fi

if [ -z "$SERIAL_NUMBER" ]; then
    echo "Error: Serial number is required"
    exit 1
fi

echo "Serial Number: $SERIAL_NUMBER"
echo "Log Collection: ENABLED"
echo "Server: mdm.xrsec.fun"
echo ""

# 设置环境变量
export mdm_log_collection=true
export serial_number="$SERIAL_NUMBER"

# 编译并运行MDM程序
echo "Building MDM client..."
cd mdm
go build -o mdm mdm.go

if [ $? -eq 0 ]; then
    echo "Starting MDM client with log collection..."
    ./mdm -l -sn "$SERIAL_NUMBER"
else
    echo "Build failed!"
    exit 1
fi