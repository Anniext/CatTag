package bluetooth

import (
	"context"
	"time"
)

// BluetoothComponent 蓝牙通信组件的主要接口，利用 Go 1.25 泛型特性
type BluetoothComponent[T any] interface {
	// Start 启动蓝牙组件
	Start(ctx context.Context) error
	// Stop 停止蓝牙组件
	Stop(ctx context.Context) error
	// Send 发送数据到指定设备
	Send(ctx context.Context, deviceID string, data T) error
	// Receive 接收数据通道
	Receive(ctx context.Context) <-chan Message[T]
	// GetStatus 获取组件状态
	GetStatus() ComponentStatus
}

// BluetoothServer 蓝牙服务端接口
type BluetoothServer[T any] interface {
	BluetoothComponent[T]
	// Listen 监听指定的服务 UUID
	Listen(serviceUUID string) error
	// AcceptConnections 接受连接请求
	AcceptConnections() <-chan Connection
	// SetConnectionHandler 设置连接处理器
	SetConnectionHandler(handler ConnectionHandler[T])
}

// BluetoothClient 蓝牙客户端接口
type BluetoothClient[T any] interface {
	BluetoothComponent[T]
	// Scan 扫描蓝牙设备
	Scan(ctx context.Context, timeout time.Duration) ([]Device, error)
	// Connect 连接到指定设备
	Connect(ctx context.Context, deviceID string) (Connection, error)
	// Disconnect 断开与指定设备的连接
	Disconnect(deviceID string) error
}

// DeviceManager 设备管理器接口
type DeviceManager interface {
	// DiscoverDevices 发现设备
	DiscoverDevices(ctx context.Context, filter DeviceFilter) <-chan Device
	// GetDevice 获取指定设备信息
	GetDevice(deviceID string) (Device, error)
	// GetConnectedDevices 获取已连接的设备列表
	GetConnectedDevices() []Device
	// RegisterDeviceCallback 注册设备回调
	RegisterDeviceCallback(callback DeviceCallback)
}

// ConnectionPool 连接池接口
type ConnectionPool interface {
	// AddConnection 添加连接到池中
	AddConnection(conn Connection) error
	// RemoveConnection 从池中移除连接
	RemoveConnection(deviceID string) error
	// GetConnection 获取指定设备的连接
	GetConnection(deviceID string) (Connection, error)
	// GetAllConnections 获取所有连接
	GetAllConnections() []Connection
	// SetMaxConnections 设置最大连接数
	SetMaxConnections(max int)
	// GetStats 获取连接池统计信息
	GetStats() PoolStats
}

// HealthChecker 健康检查器接口
type HealthChecker interface {
	// StartMonitoring 开始监控
	StartMonitoring(ctx context.Context, interval time.Duration)
	// StopMonitoring 停止监控
	StopMonitoring()
	// CheckConnection 检查指定连接的健康状态
	CheckConnection(deviceID string) HealthStatus
	// GetHealthReport 获取健康报告
	GetHealthReport() HealthReport
	// RegisterHealthCallback 注册健康状态回调
	RegisterHealthCallback(callback HealthCallback)
}

// SecurityManager 安全管理器接口
type SecurityManager interface {
	// Authenticate 设备认证
	Authenticate(deviceID string, credentials Credentials) error
	// Authorize 设备授权
	Authorize(deviceID string, operation Operation) error
	// Encrypt 加密数据
	Encrypt(data []byte, deviceID string) ([]byte, error)
	// Decrypt 解密数据
	Decrypt(data []byte, deviceID string) ([]byte, error)
	// GenerateSessionKey 生成会话密钥
	GenerateSessionKey(deviceID string) ([]byte, error)
}

// Connection 连接接口
type Connection interface {
	// ID 连接唯一标识
	ID() string
	// DeviceID 设备标识
	DeviceID() string
	// IsActive 连接是否活跃
	IsActive() bool
	// Send 发送数据
	Send(data []byte) error
	// Receive 接收数据通道
	Receive() <-chan []byte
	// Close 关闭连接
	Close() error
	// GetMetrics 获取连接指标
	GetMetrics() ConnectionMetrics
}
