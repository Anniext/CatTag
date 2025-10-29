# 快速入门指南

本指南将在 5 分钟内帮您上手 CatTag 蓝牙通信库。

## 创建第一个蓝牙应用

### 1. 基础设置

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/Anniext/CatTag/pkg/bluetooth"
)

func main() {
    // 创建蓝牙组件
    component, err := bluetooth.NewServerBuilder("12345678-1234-1234-1234-123456789abc").
        SetMaxConnections(5).
        EnableSecurity().
        Build()

    if err != nil {
        log.Fatal("创建蓝牙组件失败:", err)
    }

    // 启动组件
    ctx := context.Background()
    if err := component.Start(ctx); err != nil {
        log.Fatal("启动组件失败:", err)
    }

    fmt.Println("蓝牙服务已启动！")

    // 保持运行
    time.Sleep(30 * time.Second)

    // 停止组件
    component.Stop(ctx)
}
```

### 2. 发送和接收消息

```go
// 定义消息类型
type MyMessage struct {
    Content string `json:"content"`
    Sender  string `json:"sender"`
}

// 发送消息
msg := MyMessage{
    Content: "Hello, Bluetooth!",
    Sender:  "MyApp",
}

err := component.Send(ctx, "device_id", msg)
if err != nil {
    log.Printf("发送消息失败: %v", err)
}

// 接收消息
go func() {
    msgChan := component.Receive(ctx)
    for msg := range msgChan {
        fmt.Printf("收到消息: %+v\n", msg.Payload)
    }
}()
```

### 3. 客户端连接

```go
// 创建客户端
client, err := bluetooth.NewClientBuilder().
    EnableSecurity().
    Build()

if err != nil {
    log.Fatal("创建客户端失败:", err)
}

// 启动客户端
if err := client.Start(ctx); err != nil {
    log.Fatal("启动客户端失败:", err)
}

// 扫描设备
devices, err := client.GetClient().Scan(ctx, 10*time.Second)
if err != nil {
    log.Printf("扫描设备失败: %v", err)
    return
}

fmt.Printf("发现 %d 个设备\n", len(devices))

// 连接到第一个设备
if len(devices) > 0 {
    conn, err := client.GetClient().Connect(ctx, devices[0].ID)
    if err != nil {
        log.Printf("连接失败: %v", err)
    } else {
        fmt.Printf("已连接到设备: %s\n", devices[0].Name)
    }
}
```

## 常用模式

### 构建器模式

```go
// 服务端构建器
server := bluetooth.NewServerBuilder("service-uuid").
    SetMaxConnections(10).
    EnableSecurity().
    EnableHealthCheck().
    SetLogLevel("debug").
    Build()

// 客户端构建器
client := bluetooth.NewClientBuilder().
    EnableSecurity().
    EnableHealthCheck().
    Build()

// 完整功能构建器
full := bluetooth.NewFullBuilder("service-uuid").
    SetMaxConnections(20).
    EnableSecurity().
    Build()
```

### 配置管理

```go
// 使用默认配置
config := bluetooth.DefaultConfig()

// 自定义配置
config.ServerConfig.MaxConnections = 15
config.ClientConfig.AutoReconnect = true
config.SecurityConfig.EnableEncryption = true

// 使用配置创建组件
component, err := bluetooth.NewBluetoothComponent[MyMessage](config)
```

## 下一步

现在您已经了解了基础用法，可以继续学习：

- [基础概念](concepts.md) - 了解核心概念
- [API 文档](api/core-interfaces.md) - 详细 API 参考
- [示例应用](../examples/README.md) - 完整示例代码
- [配置指南](guides/configuration.md) - 高级配置选项
