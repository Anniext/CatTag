package bluetooth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MockAdapter 模拟蓝牙适配器实现，用于测试和开发
type MockAdapter struct {
	enabled     bool
	initialized bool
	listening   bool
	serviceUUID string
	localInfo   AdapterInfo
	mu          sync.RWMutex
}

// NewMockAdapter 创建新的模拟适配器
func NewMockAdapter() *MockAdapter {
	return &MockAdapter{
		localInfo: AdapterInfo{
			ID:      "mock_adapter_001",
			Name:    "Mock Bluetooth Adapter",
			Address: "00:11:22:33:44:55",
		},
	}
}

// Initialize 初始化适配器
func (ma *MockAdapter) Initialize(ctx context.Context) error {
	ma.mu.Lock()
	defer ma.mu.Unlock()

	if ma.initialized {
		return NewBluetoothError(ErrCodeAlreadyExists, "适配器已初始化")
	}

	// 模拟初始化延迟
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
	}

	ma.initialized = true
	return nil
}

// Shutdown 关闭适配器
func (ma *MockAdapter) Shutdown(ctx context.Context) error {
	ma.mu.Lock()
	defer ma.mu.Unlock()

	if !ma.initialized {
		return NewBluetoothError(ErrCodeNotSupported, "适配器未初始化")
	}

	// 模拟关闭延迟
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(50 * time.Millisecond):
	}

	ma.initialized = false
	ma.enabled = false
	ma.listening = false
	return nil
}

// IsEnabled 检查适配器是否启用
func (ma *MockAdapter) IsEnabled() bool {
	ma.mu.RLock()
	defer ma.mu.RUnlock()
	return ma.enabled
}

// Enable 启用适配器
func (ma *MockAdapter) Enable(ctx context.Context) error {
	ma.mu.Lock()
	defer ma.mu.Unlock()

	if !ma.initialized {
		return NewBluetoothError(ErrCodeNotSupported, "适配器未初始化")
	}

	if ma.enabled {
		return nil
	}

	// 模拟启用延迟
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(200 * time.Millisecond):
	}

	ma.enabled = true
	return nil
}

// Listen 开始监听连接请求
func (ma *MockAdapter) Listen(serviceUUID string) error {
	ma.mu.Lock()
	defer ma.mu.Unlock()

	if !ma.enabled {
		return NewBluetoothError(ErrCodeNotSupported, "适配器未启用")
	}

	if ma.listening {
		return NewBluetoothError(ErrCodeAlreadyExists, "已在监听中")
	}

	ma.serviceUUID = serviceUUID
	ma.listening = true
	return nil
}

// StopListen 停止监听
func (ma *MockAdapter) StopListen() error {
	ma.mu.Lock()
	defer ma.mu.Unlock()

	if !ma.listening {
		return nil
	}

	ma.listening = false
	ma.serviceUUID = ""
	return nil
}

// AcceptConnection 接受连接请求
func (ma *MockAdapter) AcceptConnection(ctx context.Context) (Connection, error) {
	ma.mu.RLock()
	defer ma.mu.RUnlock()

	if !ma.listening {
		return nil, NewBluetoothError(ErrCodeNotSupported, "未在监听状态")
	}

	// 模拟等待连接
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(1 * time.Second):
		// 模拟有客户端连接
		deviceID := fmt.Sprintf("mock_device_%d", time.Now().UnixNano())
		return NewMockConnection(deviceID), nil
	}
}

// SetDiscoverable 设置可发现性
func (ma *MockAdapter) SetDiscoverable(discoverable bool, timeout time.Duration) error {
	ma.mu.Lock()
	defer ma.mu.Unlock()

	if !ma.enabled {
		return NewBluetoothError(ErrCodeNotSupported, "适配器未启用")
	}

	// 模拟设置可发现性（这里只是记录，实际不做任何操作）
	_ = discoverable
	_ = timeout

	return nil
}

// GetLocalInfo 获取本地适配器信息
func (ma *MockAdapter) GetLocalInfo() AdapterInfo {
	ma.mu.RLock()
	defer ma.mu.RUnlock()
	return ma.localInfo
}

// Connect 连接到指定设备（扩展方法，用于客户端）
func (ma *MockAdapter) Connect(ctx context.Context, deviceID string) (Connection, error) {
	ma.mu.RLock()
	defer ma.mu.RUnlock()

	if !ma.enabled {
		return nil, NewBluetoothErrorWithDevice(ErrCodeNotSupported, "适配器未启用", deviceID, "connect")
	}

	// 模拟连接延迟
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(500 * time.Millisecond):
		// 模拟连接成功
		return NewMockConnection(deviceID), nil
	}
}

// Scan 扫描蓝牙设备（扩展方法，用于客户端）
func (ma *MockAdapter) Scan(ctx context.Context, timeout time.Duration) ([]Device, error) {
	ma.mu.RLock()
	defer ma.mu.RUnlock()

	if !ma.enabled {
		return nil, NewBluetoothError(ErrCodeNotSupported, "适配器未启用")
	}

	// 创建带超时的上下文
	scanCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 模拟扫描过程
	devices := make([]Device, 0)

	// 模拟发现几个设备
	mockDevices := []Device{
		{
			ID:       "device_001",
			Name:     "Mock Phone",
			Address:  "AA:BB:CC:DD:EE:01",
			RSSI:     -45,
			LastSeen: time.Now(),
			ServiceUUIDs: []string{
				"6E400001-B5A3-F393-E0A9-E50E24DCCA9E",
			},
			Capabilities: DeviceCapabilities{
				SupportsEncryption: true,
				MaxConnections:     1,
				SupportedProtocols: []string{"RFCOMM", "L2CAP"},
				BatteryLevel:       85,
			},
		},
		{
			ID:       "device_002",
			Name:     "Mock Headset",
			Address:  "AA:BB:CC:DD:EE:02",
			RSSI:     -60,
			LastSeen: time.Now(),
			ServiceUUIDs: []string{
				"0000110B-0000-1000-8000-00805F9B34FB", // Audio Sink
			},
			Capabilities: DeviceCapabilities{
				SupportsEncryption: true,
				MaxConnections:     2,
				SupportedProtocols: []string{"RFCOMM"},
				BatteryLevel:       70,
			},
		},
		{
			ID:       "device_003",
			Name:     "Mock Keyboard",
			Address:  "AA:BB:CC:DD:EE:03",
			RSSI:     -35,
			LastSeen: time.Now(),
			ServiceUUIDs: []string{
				"00001812-0000-1000-8000-00805F9B34FB", // HID over GATT
			},
			Capabilities: DeviceCapabilities{
				SupportsEncryption: true,
				MaxConnections:     1,
				SupportedProtocols: []string{"L2CAP", "GATT"},
				BatteryLevel:       95,
			},
		},
	}

	// 模拟扫描延迟和逐步发现设备
	for i, device := range mockDevices {
		select {
		case <-scanCtx.Done():
			return devices, nil
		case <-time.After(time.Duration(i+1) * 200 * time.Millisecond):
			devices = append(devices, device)
		}
	}

	return devices, nil
}

// SetLocalInfo 设置本地适配器信息（用于测试）
func (ma *MockAdapter) SetLocalInfo(info AdapterInfo) {
	ma.mu.Lock()
	defer ma.mu.Unlock()
	ma.localInfo = info
}

// SimulateError 模拟适配器错误（用于测试）
func (ma *MockAdapter) SimulateError(operation string) error {
	return NewBluetoothError(ErrCodeGeneral, fmt.Sprintf("模拟%s操作错误", operation))
}

// GetStatus 获取适配器状态（用于测试）
func (ma *MockAdapter) GetStatus() map[string]interface{} {
	ma.mu.RLock()
	defer ma.mu.RUnlock()

	return map[string]interface{}{
		"initialized": ma.initialized,
		"enabled":     ma.enabled,
		"listening":   ma.listening,
		"serviceUUID": ma.serviceUUID,
		"localInfo":   ma.localInfo,
	}
}
