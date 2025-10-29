# 核心接口文档

本文档详细介绍 CatTag 蓝牙通信库的核心接口和类型定义。

## BluetoothComponent[T]

蓝牙组件的主要接口，提供完整的蓝牙通信功能。

```go
type BluetoothComponent[T any] interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Send(ctx context.Context, deviceID string, data T) error
    Receive(ctx context.Context) <-chan Message[T]
    GetStatus() ComponentStatus
}
```

### 方法说明

#### Start(ctx context.Context) error

启动蓝牙组件，初始化所有子组件。

**参数：**

- `ctx` - 上下文，用于控制启动过程的取消和超时

**返回值：**

- `error` - 启动失败时返回错误信息

**示例：**

```go
ctx := context.Background()
err := component.Start(ctx)
if err != nil {
    log.Fatal("启动失败:", err)
}
```

#### Stop(ctx context.Context) error

停止蓝牙组件，清理所有资源。

**参数：**

- `ctx` - 上下文，用于控制停止过程的超时

**返回值：**

- `error` - 停止失败时返回错误信息

#### Send(ctx context.Context, deviceID string, data T) error

向指定设备发送数据。

**参数：**

- `ctx` - 上下文
- `deviceID` - 目标设备 ID
- `data` - 要发送的数据（泛型类型）

**返回值：**

- `error` - 发送失败时返回错误信息

#### Receive(ctx context.Context) <-chan Message[T]

获取接收消息的通道。

**参数：**

- `ctx` - 上下文

**返回值：**

- `<-chan Message[T]` - 消息接收通道

#### GetStatus() ComponentStatus

获取组件当前状态。

**返回值：**

- `ComponentStatus` - 组件状态枚举值

## BluetoothServer[T]

蓝牙服务端接口，继承自 BluetoothComponent。

```go
type BluetoothServer[T any] interface {
    BluetoothComponent[T]
    Listen(serviceUUID string) error
    AcceptConnections() <-chan Connection
    SetConnectionHandler(handler ConnectionHandler[T])
}
```

### 方法说明

#### Listen(serviceUUID string) error

开始监听指定的蓝牙服务。

**参数：**

- `serviceUUID` - 蓝牙服务 UUID

**返回值：**

- `error` - 监听失败时返回错误信息

#### AcceptConnections() <-chan Connection

获取新连接的通道。

**返回值：**

- `<-chan Connection` - 连接通道

#### SetConnectionHandler(handler ConnectionHandler[T])

设置连接处理器。

**参数：**

- `handler` - 连接处理器实例

## BluetoothClient[T]

蓝牙客户端接口，继承自 BluetoothComponent。

```go
type BluetoothClient[T any] interface {
    BluetoothComponent[T]
    Scan(ctx context.Context, timeout time.Duration) ([]Device, error)
    Connect(ctx context.Context, deviceID string) (Connection, error)
    Disconnect(deviceID string) error
}
```

### 方法说明

#### Scan(ctx context.Context, timeout time.Duration) ([]Device, error)

扫描可用的蓝牙设备。

**参数：**

- `ctx` - 上下文
- `timeout` - 扫描超时时间

**返回值：**

- `[]Device` - 发现的设备列表
- `error` - 扫描失败时返回错误信息

#### Connect(ctx context.Context, deviceID string) (Connection, error)

连接到指定设备。

**参数：**

- `ctx` - 上下文
- `deviceID` - 目标设备 ID

**返回值：**

- `Connection` - 连接实例
- `error` - 连接失败时返回错误信息

#### Disconnect(deviceID string) error

断开与指定设备的连接。

**参数：**

- `deviceID` - 设备 ID

**返回值：**

- `error` - 断开失败时返回错误信息

## 核心类型定义

### Message[T]

消息结构，支持泛型类型。

```go
type Message[T any] struct {
    ID        string          `json:"id"`
    Type      MessageType     `json:"type"`
    Payload   T               `json:"payload"`
    Metadata  MessageMetadata `json:"metadata"`
    Timestamp time.Time       `json:"timestamp"`
}
```

### Device

蓝牙设备信息。

```go
type Device struct {
    ID           string              `json:"id"`
    Name         string              `json:"name"`
    Address      string              `json:"address"`
    ServiceUUIDs []string            `json:"service_uuids"`
    RSSI         int                 `json:"rssi"`
    LastSeen     time.Time           `json:"last_seen"`
    Capabilities DeviceCapabilities  `json:"capabilities"`
}
```

### Connection

蓝牙连接接口。

```go
type Connection interface {
    ID() string
    DeviceID() string
    IsActive() bool
    Send(data []byte) error
    Receive() <-chan []byte
    Close() error
    GetMetrics() ConnectionMetrics
}
```

### ComponentStatus

组件状态枚举。

```go
type ComponentStatus int

const (
    StatusStopped ComponentStatus = iota
    StatusStarting
    StatusRunning
    StatusStopping
    StatusError
)
```

## 错误类型

### BluetoothError

蓝牙相关错误的统一类型。

```go
type BluetoothError struct {
    Code      ErrorCode `json:"code"`
    Message   string    `json:"message"`
    DeviceID  string    `json:"device_id"`
    Timestamp time.Time `json:"timestamp"`
    Cause     error     `json:"cause"`
}
```

### ErrorCode

错误代码枚举。

```go
type ErrorCode int

const (
    ErrCodeDeviceNotFound ErrorCode = iota
    ErrCodeConnectionFailed
    ErrCodeAuthenticationFailed
    ErrCodeEncryptionFailed
    ErrCodeMessageTimeout
    ErrCodeInvalidData
    ErrCodeResourceExhausted
)
```

## 使用示例

### 基本使用

```go
// 创建组件
component, err := bluetooth.NewServerBuilder("service-uuid").Build()
if err != nil {
    log.Fatal(err)
}

// 启动组件
ctx := context.Background()
if err := component.Start(ctx); err != nil {
    log.Fatal(err)
}

// 发送消息
type MyMessage struct {
    Content string `json:"content"`
}

msg := MyMessage{Content: "Hello"}
err = component.Send(ctx, "device-id", msg)

// 接收消息
go func() {
    msgChan := component.Receive(ctx)
    for msg := range msgChan {
        fmt.Printf("收到消息: %+v\n", msg.Payload)
    }
}()

// 停止组件
defer component.Stop(ctx)
```

### 错误处理

```go
err := component.Send(ctx, deviceID, data)
if err != nil {
    var btErr *bluetooth.BluetoothError
    if errors.As(err, &btErr) {
        switch btErr.Code {
        case bluetooth.ErrCodeDeviceNotFound:
            log.Printf("设备未找到: %s", btErr.DeviceID)
        case bluetooth.ErrCodeConnectionFailed:
            log.Printf("连接失败: %s", btErr.Message)
        default:
            log.Printf("其他错误: %s", btErr.Message)
        }
    }
}
```
