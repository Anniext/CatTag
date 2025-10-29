# 常见问题解答 (FAQ)

本文档收集了 CatTag 蓝牙通信库使用过程中的常见问题和解决方案。

## 安装和配置问题

### Q1: 安装 CatTag 库时出现依赖错误

**问题描述：**

```bash
go get github.com/Anniext/CatTag
# 出现依赖解析错误
```

**解决方案：**

1. 确保使用 Go 1.25.1 或更高版本
2. 清理模块缓存：`go clean -modcache`
3. 重新初始化模块：`go mod tidy`
4. 如果问题持续，尝试使用代理：`GOPROXY=https://proxy.golang.org go get github.com/Anniext/CatTag`

### Q2: 在 Windows 上运行时提示权限错误

**问题描述：**

```
错误: 访问蓝牙适配器被拒绝
```

**解决方案：**

1. 以管理员身份运行应用程序
2. 确保蓝牙服务已启动
3. 检查 Windows 防火墙设置
4. 在应用程序属性中启用蓝牙权限

### Q3: Linux 上无法访问蓝牙设备

**问题描述：**

```
错误: 蓝牙设备访问权限不足
```

**解决方案：**

1. 将用户添加到蓝牙组：
   ```bash
   sudo usermod -a -G bluetooth $USER
   ```
2. 安装蓝牙开发库：
   ```bash
   sudo apt-get install libbluetooth-dev
   ```
3. 重新登录或重启系统
4. 确保蓝牙服务运行：`sudo systemctl start bluetooth`

## 连接问题

### Q4: 设备扫描不到任何设备

**问题描述：**
客户端扫描时返回空的设备列表。

**解决方案：**

1. 确保目标设备处于可发现模式
2. 检查设备距离（建议在 10 米内）
3. 增加扫描超时时间：
   ```go
   devices, err := client.Scan(ctx, 30*time.Second) // 增加到30秒
   ```
4. 检查蓝牙适配器是否正常工作
5. 尝试重启蓝牙适配器

### Q5: 连接建立后立即断开

**问题描述：**

```
[连接] 已连接到设备: device_001
[断开] 连接已断开: device_001
```

**解决方案：**

1. 检查设备兼容性和蓝牙版本
2. 增加连接超时时间：
   ```go
   config.ClientConfig.ConnectTimeout = 60 * time.Second
   ```
3. 启用自动重连：
   ```go
   config.ClientConfig.AutoReconnect = true
   ```
4. 检查设备电源管理设置
5. 确保服务 UUID 匹配

### Q6: 连接频繁中断

**问题描述：**
连接建立后经常出现意外断开。

**解决方案：**

1. 检查信号强度（RSSI 值）
2. 减少设备间距离
3. 避免蓝牙干扰源（WiFi、微波炉等）
4. 启用健康检查：
   ```go
   config.HealthConfig.EnableHealthCheck = true
   config.HealthConfig.CheckInterval = 5 * time.Second
   ```
5. 调整心跳间隔：
   ```go
   config.HealthConfig.HeartbeatInterval = 3 * time.Second
   ```

## 消息传输问题

### Q7: 消息发送失败

**问题描述：**

```go
err := component.Send(ctx, deviceID, message)
// 返回发送失败错误
```

**解决方案：**

1. 检查连接状态：
   ```go
   status := component.GetStatus()
   if status != bluetooth.StatusRunning {
       log.Printf("组件未运行: %v", status)
   }
   ```
2. 验证设备 ID 是否正确
3. 检查消息大小是否超过限制
4. 确保消息类型可以序列化
5. 启用重试机制

### Q8: 消息接收不到

**问题描述：**
发送方显示发送成功，但接收方收不到消息。

**解决方案：**

1. 确保接收方正在监听：
   ```go
   go func() {
       msgChan := component.Receive(ctx)
       for msg := range msgChan {
           log.Printf("收到消息: %+v", msg)
       }
   }()
   ```
2. 检查消息类型匹配
3. 验证序列化/反序列化是否正确
4. 检查消息队列是否满了
5. 启用调试日志查看详细信息

### Q9: 大文件传输失败

**问题描述：**
传输大文件时出现超时或失败。

**解决方案：**

1. 使用分块传输：
   ```go
   chunkSize := 1024 // 1KB分块
   ```
2. 增加传输超时时间
3. 实现断点续传机制
4. 添加传输进度监控
5. 使用文件校验确保完整性

## 性能问题

### Q10: 连接建立速度慢

**问题描述：**
设备连接需要很长时间才能建立。

**解决方案：**

1. 优化扫描参数：
   ```go
   config.ClientConfig.ScanTimeout = 5 * time.Second
   ```
2. 使用设备缓存避免重复扫描
3. 实现并行连接：
   ```go
   for _, device := range devices {
       go connectToDevice(device)
   }
   ```
4. 预连接常用设备
5. 优化蓝牙适配器设置

### Q11: 内存使用过高

**问题描述：**
应用程序运行时内存使用持续增长。

**解决方案：**

1. 检查是否有内存泄漏：
   ```go
   import _ "net/http/pprof"
   go func() {
       log.Println(http.ListenAndServe("localhost:6060", nil))
   }()
   ```
2. 限制连接池大小：
   ```go
   config.PoolConfig.MaxConnections = 10
   ```
3. 定期清理过期连接
4. 优化消息缓冲区大小
5. 使用对象池复用对象

### Q12: CPU 使用率过高

**问题描述：**
应用程序 CPU 使用率异常高。

**解决方案：**

1. 减少扫描频率：
   ```go
   scanInterval := 30 * time.Second
   ```
2. 优化心跳间隔：
   ```go
   config.HealthConfig.HeartbeatInterval = 10 * time.Second
   ```
3. 使用 goroutine 池限制并发
4. 避免频繁的设备状态检查
5. 启用性能分析找出热点

## 安全问题

### Q13: 设备认证失败

**问题描述：**

```
错误: 设备认证失败，连接被拒绝
```

**解决方案：**

1. 检查认证配置：
   ```go
   config.SecurityConfig.EnableAuth = true
   ```
2. 确保设备已配对
3. 清除旧的配对信息重新配对
4. 检查认证凭据是否正确
5. 验证设备是否在信任列表中

### Q14: 数据加密失败

**问题描述：**
启用加密后无法正常通信。

**解决方案：**

1. 检查加密配置：
   ```go
   config.SecurityConfig.EnableEncryption = true
   config.SecurityConfig.Algorithm = "AES-256-GCM"
   ```
2. 确保双方使用相同的加密算法
3. 验证密钥交换是否成功
4. 检查密钥大小设置
5. 查看加密相关的错误日志

## 调试和诊断

### Q15: 如何启用详细日志

**解决方案：**

```go
config := bluetooth.DefaultConfig()
config.LogConfig.Level = "debug"
config.LogConfig.EnableFile = true
config.LogConfig.FilePath = "./bluetooth.log"

component, err := bluetooth.NewBluetoothComponent[MyMessage](config)
```

### Q16: 如何监控连接状态

**解决方案：**

```go
// 获取组件状态
status := component.GetStatus()
log.Printf("组件状态: %v", status)

// 获取连接池统计
pool := component.GetConnectionPool()
stats := pool.GetStats()
log.Printf("活跃连接: %d", stats.ActiveConnections)

// 获取健康报告
healthChecker := component.GetHealthChecker()
report := healthChecker.GetHealthReport()
log.Printf("整体健康: %v", report.OverallHealth)
```

### Q17: 如何进行性能分析

**解决方案：**

```go
import (
    _ "net/http/pprof"
    "net/http"
)

// 启动pprof服务器
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()

// 使用go tool pprof分析
// go tool pprof http://localhost:6060/debug/pprof/profile
// go tool pprof http://localhost:6060/debug/pprof/heap
```

## 平台特定问题

### Q18: macOS 上权限问题

**问题描述：**
macOS 上无法访问蓝牙功能。

**解决方案：**

1. 在"系统偏好设置" > "安全性与隐私" > "隐私"中授权蓝牙访问
2. 确保应用程序已签名
3. 添加蓝牙使用说明到 Info.plist
4. 重启应用程序

### Q19: Android 设备兼容性问题

**问题描述：**
某些 Android 设备无法正常连接。

**解决方案：**

1. 检查 Android 版本兼容性
2. 尝试不同的蓝牙协议（RFCOMM/L2CAP）
3. 调整连接参数
4. 查看设备特定的蓝牙限制
5. 使用设备白名单

## 获取更多帮助

如果以上解决方案无法解决您的问题，请：

1. 查看 [故障诊断指南](diagnostics.md)
2. 查看 [GitHub Issues](https://github.com/Anniext/CatTag/issues)
3. 提交新的 Issue 并提供：

   - 详细的错误信息
   - 完整的日志输出
   - 系统环境信息
   - 重现问题的最小代码示例

4. 参考 [示例应用](../../examples/) 中的实现
