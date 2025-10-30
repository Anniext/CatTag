# CatTag CLI 使用指南

CatTag 现在使用 Cobra 框架提供强大的命令行界面，支持多种运行模式和丰富的配置选项。

## 安装和构建

### 从源码构建

```bash
# 克隆仓库
git clone https://github.com/Anniext/CatTag.git
cd CatTag

# 构建应用程序
make build

# 或者直接使用 go build
go build -o cattag ./cmd/app
```

### 安装到系统

```bash
# 构建并安装到系统
make install

# 现在可以在任何地方使用 cattag 命令
cattag --help
```

## 基本用法

### 查看帮助信息

```bash
# 查看主帮助
cattag --help

# 查看特定命令的帮助
cattag server --help
cattag client --help
cattag scan --help
```

### 查看版本信息

```bash
cattag version
```

## 运行模式

### 服务端模式

启动蓝牙服务端，等待客户端连接：

```bash
# 使用默认配置启动服务端
cattag server

# 自定义服务端配置
cattag server \
  --service-uuid "12345678-1234-1234-1234-123456789abc" \
  --service-name "我的蓝牙服务" \
  --max-connections 20 \
  --accept-timeout 60s \
  --require-auth=false \
  --log-level debug
```

### 客户端模式

启动蓝牙客户端，连接到服务端：

```bash
# 扫描并连接到可用设备
cattag client

# 连接到特定设备
cattag client \
  --target-device "AA:BB:CC:DD:EE:FF" \
  --scan-timeout 15s \
  --connect-timeout 30s \
  --retry-attempts 5 \
  --auto-reconnect=true \
  --log-level debug
```

### 混合模式

同时运行服务端和客户端：

```bash
# 同时启动服务端和客户端
cattag both

# 使用自定义配置
cattag both \
  --service-uuid "12345678-1234-1234-1234-123456789abc" \
  --target-device "AA:BB:CC:DD:EE:FF" \
  --max-connections 10 \
  --log-level info
```

### 设备扫描模式

扫描周围的蓝牙设备：

```bash
# 基本扫描
cattag scan

# 自定义扫描参数
cattag scan \
  --duration 30s \
  --show-rssi=true \
  --filter-name "CatTag" \
  --continuous=false

# 持续扫描模式
cattag scan --continuous --log-level debug
```

## 配置管理

### 配置文件

CatTag 支持 YAML 格式的配置文件，默认搜索路径：

1. `--config` 参数指定的路径
2. `$HOME/.cattag.yaml`
3. `./cattag.yaml`
4. `./config/cattag.yaml`

### 示例配置文件

```yaml
# 全局配置
log_level: "info"

# 服务端配置
server:
  service_uuid: "12345678-1234-1234-1234-123456789abc"
  service_name: "CatTag Service"
  max_connections: 10
  accept_timeout: "30s"
  require_auth: true

# 客户端配置
client:
  target_device: ""
  scan_timeout: "10s"
  connect_timeout: "15s"
  retry_attempts: 3
  retry_interval: "2s"
  auto_reconnect: true

# 扫描配置
scan:
  duration: "10s"
  show_rssi: true
  filter_name: ""
  filter_service: ""
  continuous: false
```

### 环境变量

所有配置选项都可以通过环境变量设置，使用 `CATTAG_` 前缀：

```bash
export CATTAG_LOG_LEVEL=debug
export CATTAG_SERVER_MAX_CONNECTIONS=20
export CATTAG_CLIENT_AUTO_RECONNECT=true

cattag server
```

## 日志配置

支持四个日志级别：

- `debug`: 详细调试信息
- `info`: 一般信息（默认）
- `warn`: 警告信息
- `error`: 仅错误信息

```bash
# 设置日志级别
cattag server --log-level debug
```

## 实用示例

### 开发和调试

```bash
# 启动调试模式的服务端
cattag server --log-level debug --require-auth=false

# 在另一个终端启动客户端
cattag client --log-level debug --auto-reconnect=true

# 扫描设备进行调试
cattag scan --duration 5s --show-rssi
```

### 生产环境

```bash
# 使用配置文件启动服务端
cattag server --config /etc/cattag/production.yaml

# 后台运行
nohup cattag server --config /etc/cattag/production.yaml > /var/log/cattag.log 2>&1 &
```

### 性能测试

```bash
# 高并发服务端
cattag server --max-connections 100 --log-level warn

# 多客户端连接测试
for i in {1..10}; do
  cattag client --log-level error &
done
```

## 故障排除

### 常见问题

1. **权限问题**: 在某些系统上，蓝牙操作需要管理员权限

   ```bash
   sudo cattag server
   ```

2. **设备不可见**: 确保蓝牙设备已启用且可发现

   ```bash
   cattag scan --duration 30s --log-level debug
   ```

3. **连接失败**: 检查设备地址和服务 UUID
   ```bash
   cattag client --target-device "AA:BB:CC:DD:EE:FF" --log-level debug
   ```

### 调试技巧

```bash
# 启用详细日志
cattag server --log-level debug

# 使用配置文件进行复杂配置
cattag server --config debug.yaml

# 检查版本和构建信息
cattag version
```

## 开发工具

### Makefile 命令

```bash
# 构建项目
make build

# 运行测试
make test

# 代码检查
make lint

# 快速开发测试
make run-server    # 运行服务端
make run-client    # 运行客户端
make run-scan      # 扫描设备

# 查看所有可用命令
make help
```

这个新的 CLI 结构提供了更好的用户体验、更灵活的配置选项和更强大的功能。
