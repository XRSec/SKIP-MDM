# MDM 日志收集系统部署指南

## 系统架构

```
┌─────────────────┐    HTTP POST     ┌─────────────────┐    SQL     ┌─────────────────┐
│   MDM Client    │ ──────────────▶  │   Log Server    │ ─────────▶ │   MySQL DB      │
│   (macOS)       │                  │   (Go HTTP)     │            │   (mdm_logs)    │
└─────────────────┘                  └─────────────────┘            └─────────────────┘
```

## 快速部署

### 1. 服务器端部署

```bash
# 1. 安装MySQL
sudo apt-get install mysql-server  # Ubuntu/Debian
# 或
brew install mysql                  # macOS

# 2. 创建数据库
mysql -u root -p < server/init_db.sql

# 3. 配置数据库连接
cp server/config.example.json server/config.json
# 编辑 config.json 中的数据库连接信息

# 4. 启动日志服务器
cd server
./start_server.sh
```

### 2. 客户端使用

```bash
# 方式1: 使用脚本启动（推荐）
sudo ./run_with_logs.sh YOUR_SERIAL_NUMBER

# 方式2: 直接运行
cd mdm
sudo ./mdm -l -sn YOUR_SERIAL_NUMBER

# 方式3: 环境变量
export mdm_log_collection=true
export serial_number=YOUR_SERIAL_NUMBER
sudo ./mdm
```

## 详细配置

### 服务器配置

修改 `server/log_server.go` 中的数据库连接字符串：
```go
dsn := "username:password@tcp(host:port)/database?charset=utf8mb4&parseTime=True&loc=Local"
```

### 客户端配置

在 `mdm/mdm.go` 中可以修改以下配置：
- `serverHost`: 日志服务器地址
- `serverPort`: 日志服务器端口
- 日志收集的具体路径和内容

## 收集的数据详情

### 系统基础信息
- 设备序列号
- macOS 版本
- 时间戳

### 文件系统信息
- `/Volumes/` - 挂载的卷
- `/Applications` - 已安装应用程序
- `Library/LaunchAgents` - 用户启动代理
- `Library/LaunchDaemons` - 系统启动守护进程
- `Library/Application Support` - 应用程序支持文件
- `Users/*/Library/Preferences` - 用户偏好设置

### MDM 相关信息
- `var/db/ConfigurationProfiles/Settings` - MDM配置文件
- `.cloudConfigRecordFound` 内容 - 云配置记录
- MDM相关进程列表
- 已知MDM域名列表

### 系统状态
- 系统日志（最近50行）
- 网络配置信息
- 路由表信息

## API 使用

### 提交日志
```bash
curl -X POST http://localhost:8080/log \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d @log_data.json
```

### 查询日志
```bash
# 查询所有日志
curl "http://localhost:8080/query"

# 查询特定设备
curl "http://localhost:8080/query?serial=ABC123&limit=100"

# 获取详细信息
curl "http://localhost:8080/detail?id=1"
```

## 数据分析

### 使用分析工具
```bash
cd server

# 分析特定设备
go run log_analyzer.go analyze ABC123DEF456

# 生成总体报告
go run log_analyzer.go report

# 搜索关键词
go run log_analyzer.go search "jamf"
```

### SQL 查询示例
```sql
-- 查看最近活跃的设备
SELECT * FROM recent_devices LIMIT 10;

-- 查看每日统计
SELECT * FROM mdm_stats ORDER BY log_date DESC LIMIT 7;

-- 查找包含特定MDM软件的设备
SELECT DISTINCT serial_number 
FROM mdm_logs 
WHERE raw_data LIKE '%jamf%' 
   OR raw_data LIKE '%mosyle%';

-- 分析清理效果
SELECT 
    serial_number,
    COUNT(*) as attempts,
    MIN(server_timestamp) as first_attempt,
    MAX(server_timestamp) as last_attempt
FROM mdm_logs 
WHERE raw_data LIKE '%cleaning%'
GROUP BY serial_number;
```

## 安全考虑

### 认证机制
- 使用基于时间戳的SHA256哈希认证
- 时间窗口为±5分钟，防止重放攻击
- 认证密钥: `m'd'm`

### 数据保护
- 建议在生产环境使用HTTPS
- 定期轮换认证密钥
- 设置适当的防火墙规则

### 隐私保护
- 不收集用户个人文件内容
- 只收集文件名和目录结构
- 系统日志已过滤敏感信息

## 故障排除

### 常见问题

1. **数据库连接失败**
   - 检查MySQL服务是否运行
   - 验证连接字符串和凭据
   - 确认数据库和表已创建

2. **认证失败**
   - 检查客户端和服务器时间同步
   - 验证序列号是否正确
   - 确认认证密钥匹配

3. **日志收集不工作**
   - 确认 `enableLogCollection` 已设置为 true
   - 检查网络连接
   - 查看调试输出（使用 -d 参数）

### 调试模式

启用调试模式查看详细信息：
```bash
sudo ./mdm -d -l -sn YOUR_SERIAL_NUMBER
```

## 性能优化

### 数据库优化
- 定期清理旧日志
- 添加适当的索引
- 考虑分表存储

### 服务器优化
- 使用连接池
- 实现缓存机制
- 添加负载均衡

## 监控建议

### 服务器监控
- 监控数据库连接数
- 跟踪API响应时间
- 监控磁盘空间使用

### 客户端监控
- 跟踪日志提交成功率
- 监控网络延迟
- 记录认证失败次数