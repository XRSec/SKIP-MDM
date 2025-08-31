#!/bin/bash

# MDM日志收集系统测试脚本

echo "MDM Log Collection System Test"
echo "=============================="

# 检查必要的工具
check_command() {
    if ! command -v $1 &> /dev/null; then
        echo "Error: $1 is not installed"
        return 1
    fi
    return 0
}

echo "Checking prerequisites..."
check_command "go" || exit 1
check_command "mysql" || echo "Warning: MySQL client not found"

# 测试服务器编译
echo ""
echo "Testing server compilation..."
cd server
if go build -o test-server log_server.go; then
    echo "✓ Server compiles successfully"
    rm -f test-server
else
    echo "✗ Server compilation failed"
    exit 1
fi

# 测试分析器编译
if go build -o test-analyzer log_analyzer.go; then
    echo "✓ Log analyzer compiles successfully"
    rm -f test-analyzer
else
    echo "✗ Log analyzer compilation failed"
    exit 1
fi

# 测试客户端编译
echo ""
echo "Testing client compilation..."
cd ../mdm
if go build -o test-mdm mdm.go; then
    echo "✓ Client compiles successfully"
    rm -f test-mdm
else
    echo "✗ Client compilation failed"
    exit 1
fi

echo ""
echo "All tests passed! ✓"
echo ""
echo "Next steps:"
echo "1. Set up MySQL database: mysql -u root -p < server/init_db.sql"
echo "2. Start log server: cd server && ./start_server.sh"
echo "3. Run client with logs: sudo ./run_with_logs.sh YOUR_SERIAL"
echo ""
echo "For more information, see DEPLOYMENT_GUIDE.md"