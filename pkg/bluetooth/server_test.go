package bluetooth

import (
	"context"
	"testing"
	"time"
)

// MockBluetoothAdapter 模拟蓝牙适配器，用于测试
type MockBluetoothAdapter struct {
	initialized  bool
	enabled      bool
	listening    bool
	discoverable bool
	connections  map[string]*MockConnection
	acceptChan   chan Connection
	localInfo    AdapterInfo
}

// NewMockBluetoothAdapter 创建新的模拟蓝牙适配器
func NewMockBluetoothAdapter() *MockBluetoothAdapter {
	return &MockBluetoothAdapter{
		connections: make(map[string]*MockConnection),
		acceptChan:  make(chan Connection, 10),
		localInfo: AdapterInfo{
			ID:      "mock_adapter_001",
			Name:    "Mock Bluetooth Adapter",
			Address: "00:11:22:33:44:55",
		},
	}
}

// Initialize 初始化适配器
func (m *MockBluetoothAdapter) Initialize(ctx context.Context) error {
	m.initialized = true
	return nil
}

// Shutdown 关闭适配器
func (m *MockBluetoothAdapter) Shutdown(ctx context.Context) error {
	m.initialized = false
	m.enabled = false
	m.listening = false
	return nil
}

// IsEnabled 检查蓝牙是否启用
func (m *MockBluetoothAdapter) IsEnabled() bool {
	return m.enabled
}

// Enable 启用蓝牙
func (m *MockBluetoothAdapter) Enable(ctx context.Context) error {
	m.enabled = true
	return nil
}

// Listen 监听连接请求
func (m *MockBluetoothAdapter) Listen(serviceUUID string) error {
	m.listening = true
	return nil
}

// StopListen 停止监听
func (m *MockBluetoothAdapter) StopListen() error {
	m.listening = false
	return nil
}

// AcceptConnection 接受连接请求
func (m *MockBluetoothAdapter) AcceptConnection(ctx context.Context) (Connection, error) {
	select {
	case conn := <-m.acceptChan:
		return conn, nil
	case <-ctx.Done():
		return nil, NewBluetoothError(ErrCodeTimeout, "接受连接超时")
	}
}

// SetDiscoverable 设置可发现性
func (m *MockBluetoothAdapter) SetDiscoverable(discoverable bool, timeout time.Duration) error {
	m.discoverable = discoverable
	return nil
}

// GetLocalInfo 获取本地适配器信息
func (m *MockBluetoothAdapter) GetLocalInfo() AdapterInfo {
	return m.localInfo
}

// AddMockConnection 添加模拟连接
func (m *MockBluetoothAdapter) AddMockConnection(deviceID string) *MockConnection {
	mockConn := NewMockConnectionPtr(deviceID)
	conn := Connection(mockConn)
	m.connections[deviceID] = mockConn

	// 发送到接受通道
	select {
	case m.acceptChan <- conn:
	default:
		// 通道满了
	}

	return mockConn
}

// TestServerCreation 测试服务端创建
func TestServerCreation(t *testing.T) {
	config := DefaultConfig().ServerConfig
	adapter := NewMockBluetoothAdapter()

	server := NewBluetoothServer[string](config, adapter)

	if server == nil {
		t.Fatal("服务端创建失败")
	}

	if server.GetStatus() != StatusStopped {
		t.Errorf("期望状态为 %v, 实际为 %v", StatusStopped, server.GetStatus())
	}
}

// TestServerStartStop 测试服务端启动和停止
func TestServerStartStop(t *testing.T) {
	config := DefaultConfig().ServerConfig
	adapter := NewMockBluetoothAdapter()

	server := NewBluetoothServer[string](config, adapter)

	ctx := context.Background()

	// 测试启动
	err := server.Start(ctx)
	if err != nil {
		t.Fatalf("启动服务端失败: %v", err)
	}

	if server.GetStatus() != StatusRunning {
		t.Errorf("期望状态为 %v, 实际为 %v", StatusRunning, server.GetStatus())
	}

	// 测试重复启动
	err = server.Start(ctx)
	if err == nil {
		t.Error("重复启动应该返回错误")
	}

	// 测试停止
	err = server.Stop(ctx)
	if err != nil {
		t.Fatalf("停止服务端失败: %v", err)
	}

	if server.GetStatus() != StatusStopped {
		t.Errorf("期望状态为 %v, 实际为 %v", StatusStopped, server.GetStatus())
	}
}

// TestServerListen 测试服务端监听功能
func TestServerListen(t *testing.T) {
	config := DefaultConfig().ServerConfig
	adapter := NewMockBluetoothAdapter()

	server := NewBluetoothServer[string](config, adapter)

	ctx := context.Background()

	// 启动服务端
	err := server.Start(ctx)
	if err != nil {
		t.Fatalf("启动服务端失败: %v", err)
	}
	defer server.Stop(ctx)

	// 测试监听
	serviceUUID := DefaultServiceUUID
	err = server.Listen(serviceUUID)
	if err != nil {
		t.Fatalf("开始监听失败: %v", err)
	}

	if !adapter.listening {
		t.Error("适配器应该处于监听状态")
	}
}

// TestServerAcceptConnections 测试服务端接受连接
func TestServerAcceptConnections(t *testing.T) {
	config := DefaultConfig().ServerConfig
	config.MaxConnections = 2
	adapter := NewMockBluetoothAdapter()

	server := NewBluetoothServer[string](config, adapter)

	ctx := context.Background()

	// 启动服务端
	err := server.Start(ctx)
	if err != nil {
		t.Fatalf("启动服务端失败: %v", err)
	}
	defer server.Stop(ctx)

	// 开始监听
	err = server.Listen(DefaultServiceUUID)
	if err != nil {
		t.Fatalf("开始监听失败: %v", err)
	}

	// 模拟连接请求
	mockConn1 := adapter.AddMockConnection("device_001")
	mockConn2 := adapter.AddMockConnection("device_002")

	// 等待连接被接受
	time.Sleep(200 * time.Millisecond)

	connections := server.GetConnections()
	if len(connections) != 2 {
		t.Errorf("期望连接数为 2, 实际为 %d", len(connections))
	}

	// 验证连接
	found1, found2 := false, false
	for _, conn := range connections {
		if conn.DeviceID() == "device_001" {
			found1 = true
		}
		if conn.DeviceID() == "device_002" {
			found2 = true
		}
	}

	if !found1 || !found2 {
		t.Error("未找到期望的连接")
	}

	// 测试连接数限制
	mockConn3 := adapter.AddMockConnection("device_003")
	time.Sleep(200 * time.Millisecond)

	// 第三个连接应该被拒绝
	if len(server.GetConnections()) > 2 {
		t.Error("超过最大连接数限制")
	}

	// 清理
	mockConn1.Close()
	mockConn2.Close()
	mockConn3.Close()
}

// TestServerSendReceive 测试服务端发送和接收消息
func TestServerSendReceive(t *testing.T) {
	config := DefaultConfig().ServerConfig
	adapter := NewMockBluetoothAdapter()

	server := NewBluetoothServer[string](config, adapter)

	ctx := context.Background()

	// 启动服务端
	err := server.Start(ctx)
	if err != nil {
		t.Fatalf("启动服务端失败: %v", err)
	}
	defer server.Stop(ctx)

	// 开始监听
	err = server.Listen(DefaultServiceUUID)
	if err != nil {
		t.Fatalf("开始监听失败: %v", err)
	}

	// 添加模拟连接
	mockConn := adapter.AddMockConnection("device_001")
	time.Sleep(100 * time.Millisecond)

	// 测试发送消息
	testData := "Hello, Bluetooth!"
	err = server.Send(ctx, "device_001", testData)
	if err != nil {
		t.Fatalf("发送消息失败: %v", err)
	}

	// 验证消息已发送
	sentData := mockConn.GetSentData()
	if len(sentData) == 0 {
		t.Error("没有发送任何数据")
	}

	// 测试接收消息
	receiveChan := server.Receive(ctx)

	// 模拟接收数据
	testReceiveData := []byte("Hello from client!")
	mockConn.SimulateReceiveData(testReceiveData)

	// 等待消息处理
	select {
	case message := <-receiveChan:
		if message.Metadata.SenderID != "device_001" {
			t.Errorf("期望发送者为 device_001, 实际为 %s", message.Metadata.SenderID)
		}
	case <-time.After(1 * time.Second):
		t.Error("接收消息超时")
	}

	mockConn.Close()
}

// TestServerConnectionHandler 测试服务端连接处理器
func TestServerConnectionHandler(t *testing.T) {
	config := DefaultConfig().ServerConfig
	adapter := NewMockBluetoothAdapter()

	server := NewBluetoothServer[string](config, adapter)

	// 创建测试连接处理器
	var connectedCalled, disconnectedCalled, messageCalled bool

	handler := NewDefaultConnectionHandler[string]()
	handler.SetOnConnected(func(conn Connection) error {
		connectedCalled = true
		return nil
	})
	handler.SetOnDisconnected(func(conn Connection, err error) {
		disconnectedCalled = true
	})
	handler.SetOnMessage(func(conn Connection, message Message[string]) error {
		messageCalled = true
		return nil
	})
	handler.SetOnError(func(conn Connection, err error) {
		// 错误处理
	})

	server.SetConnectionHandler(handler)

	ctx := context.Background()

	// 启动服务端
	err := server.Start(ctx)
	if err != nil {
		t.Fatalf("启动服务端失败: %v", err)
	}
	defer server.Stop(ctx)

	// 开始监听
	err = server.Listen(DefaultServiceUUID)
	if err != nil {
		t.Fatalf("开始监听失败: %v", err)
	}

	// 添加模拟连接
	mockConn := adapter.AddMockConnection("device_001")
	time.Sleep(100 * time.Millisecond)

	// 验证连接回调被调用
	if !connectedCalled {
		t.Error("连接回调未被调用")
	}

	// 模拟接收数据触发消息回调
	mockConn.SimulateReceiveData([]byte("test message"))
	time.Sleep(100 * time.Millisecond)

	if !messageCalled {
		t.Error("消息回调未被调用")
	}

	// 关闭连接触发断开回调
	mockConn.Close()
	time.Sleep(100 * time.Millisecond)

	if !disconnectedCalled {
		t.Error("断开连接回调未被调用")
	}
}

// TestServerDisconnectDevice 测试服务端断开设备连接
func TestServerDisconnectDevice(t *testing.T) {
	config := DefaultConfig().ServerConfig
	adapter := NewMockBluetoothAdapter()

	server := NewBluetoothServer[string](config, adapter)

	ctx := context.Background()

	// 启动服务端
	err := server.Start(ctx)
	if err != nil {
		t.Fatalf("启动服务端失败: %v", err)
	}
	defer server.Stop(ctx)

	// 开始监听
	err = server.Listen(DefaultServiceUUID)
	if err != nil {
		t.Fatalf("开始监听失败: %v", err)
	}

	// 添加模拟连接
	mockConn := adapter.AddMockConnection("device_001")
	time.Sleep(100 * time.Millisecond)

	// 验证连接存在
	if server.GetConnectionCount() != 1 {
		t.Errorf("期望连接数为 1, 实际为 %d", server.GetConnectionCount())
	}

	// 断开设备连接
	err = server.DisconnectDevice("device_001")
	if err != nil {
		t.Fatalf("断开设备连接失败: %v", err)
	}

	// 验证连接已断开
	if server.GetConnectionCount() != 0 {
		t.Errorf("期望连接数为 0, 实际为 %d", server.GetConnectionCount())
	}

	if mockConn.IsActive() {
		t.Error("连接应该已关闭")
	}

	// 测试断开不存在的设备
	err = server.DisconnectDevice("nonexistent_device")
	if err == nil {
		t.Error("断开不存在的设备应该返回错误")
	}
}

// TestServerErrorHandling 测试服务端错误处理
func TestServerErrorHandling(t *testing.T) {
	config := DefaultConfig().ServerConfig
	adapter := NewMockBluetoothAdapter()

	server := NewBluetoothServer[string](config, adapter)

	ctx := context.Background()

	// 测试未启动时的操作
	err := server.Listen(DefaultServiceUUID)
	if err == nil {
		t.Error("未启动时监听应该返回错误")
	}

	err = server.Send(ctx, "device_001", "test")
	if err == nil {
		t.Error("未启动时发送应该返回错误")
	}

	// 启动服务端
	err = server.Start(ctx)
	if err != nil {
		t.Fatalf("启动服务端失败: %v", err)
	}
	defer server.Stop(ctx)

	// 测试发送到不存在的设备
	err = server.Send(ctx, "nonexistent_device", "test")
	if err == nil {
		t.Error("发送到不存在的设备应该返回错误")
	}
}
