# MDM 日志收集系统

## 功能概述

这个系统为 MDM 助手添加了完整的日志收集功能，可以收集用户的系统信息并存储到数据库中。

## 客户端功能

### 启用日志收集

有三种方式启用日志收集：

1. **命令行参数**：
   ```bash
   ./mdm -l -sn YOUR_SERIAL_NUMBER
   ```

2. **环境变量**：
   ```bash
   export mdm_log_collection=true
   ./mdm -sn YOUR_SERIAL_NUMBER
   ```

3. **代码中设置**：
   ```go
   enableLogCollection = true
   ```

### 收集的信息

客户端会收集以下系统信息：
- 设备序列号和系统版本
- `/Volumes/` 目录列表
- `LibraryPath/LaunchAgents` 文件列表
- `LibraryPath/LaunchDaemons` 文件列表  
- `LibraryPath/Application Support` 文件列表
- `UserLibraryPath/Preferences/` 文件列表
- `/Applications` 应用程序列表
- `MDMPath/Settings` MDM配置文件列表
- `MDMPath/Settings/.cloudConfigRecordFound` 内容
- 系统日志（最近50行）
- MDM相关进程列表
- 网络配置信息

### 认证机制

客户端使用基于时间戳和序列号的SHA256哈希认证：
```
token = sha256("mdm_log_auth" + serial_number + timestamp + "m'd'm")[:32]
```

## 服务器端功能

### 安装和配置

1. **安装依赖**：
   ```bash
   cd server
   go mod tidy
   ```

2. **配置数据库**：
   ```bash
   mysql -u root -p < init_db.sql
   ```

3. **启动服务器**：
   ```bash
   ./start_server.sh
   ```

### API 端点

#### POST /log
接收客户端日志数据
- **认证**: Bearer token 必需
- **Content-Type**: application/json

#### GET /query
查询日志记录
- **参数**: 
  - `serial` (可选): 设备序列号
  - `limit` (可选): 返回记录数量，最大1000

#### GET /detail
获取详细日志信息
- **参数**: 
  - `id` (必需): 日志记录ID

#### GET /health
健康检查端点

### 数据库结构

```sql
CREATE TABLE mdm_logs (
    id INT AUTO_INCREMENT PRIMARY KEY,
    serial_number VARCHAR(255) NOT NULL,
    os_version VARCHAR(100),
    client_timestamp VARCHAR(50),
    server_timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    raw_data LONGTEXT,
    ip_address VARCHAR(45),
    user_agent VARCHAR(500)
);
```

## 使用示例

### 启用日志收集的客户端运行
```bash
# 方式1: 命令行参数
./mdm -l -sn ABC123DEF456

# 方式2: 环境变量
export mdm_log_collection=true
export serial_number=ABC123DEF456
./mdm
```

### 查询日志
```bash
# 查询所有日志
curl "http://localhost:8080/query"

# 查询特定设备日志
curl "http://localhost:8080/query?serial=ABC123DEF456&limit=50"

# 获取详细日志
curl "http://localhost:8080/detail?id=1"
```

## 安全说明

1. **认证机制**: 使用时间窗口认证，防止重放攻击
2. **数据加密**: 建议在生产环境中使用HTTPS
3. **访问控制**: 建议配置防火墙限制访问
4. **数据保护**: 敏感信息已做脱敏处理

## 注意事项

1. 日志收集是可选功能，默认关闭
2. 所有网络请求都是异步的，不会影响主程序运行
3. 认证失败时日志会被丢弃，不会影响程序正常功能
4. 建议定期清理旧日志数据以节省存储空间

## 监控和维护

### 日志轮转
建议设置定期清理旧日志：
```sql
DELETE FROM mdm_logs WHERE server_timestamp < DATE_SUB(NOW(), INTERVAL 30 DAY);
```

### 性能监控
使用提供的视图查看统计信息：
```sql
SELECT * FROM mdm_stats;
SELECT * FROM recent_devices;
```