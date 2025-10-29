package adapter

import (
	"context"
	"testing"
	"time"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// TestDefaultAdapter_Initialize 测试适配器初始化
func TestDefaultAdapter_Initialize(t *testing.T) {
	config := DefaultAdapterConfig()
	adapter := NewDefaultAdapter(config)

	ctx := context.Background()
	err := adapter.Initialize(ctx)
	if err != nil {
		t.Fatalf("适配器初始化失败: %v", err)
	}

	// 验证适配器信息
	info := adapter.GetLocalInfo()
	if info.ID == "" {
		t.Error("适配器ID不能为空")
	}
	if info.Name != config.Name {
		t.Errorf("适配器名称不匹配，期望: %s, 实际: %s", config.Name, info.Name)
	}

	// 清理
	err = adapter.Shutdown(ctx)
	if err != nil {
		t.Errorf("适配器关闭失败: %v", err)
	}
}

// TestDefaultAdapter_EnableDisable 测试适配器启用和禁用
func TestDefaultAdapter_EnableDisable(t *testing.T) {
	config := DefaultAdapterConfig()
	adapter := NewDefaultAdapter(config)

	ctx := context.Background()
	err := adapter.Initialize(ctx)
	if err != nil {
		t.Fatalf("适配器初始化失败: %v", err)
	}
	defer adapter.Shutdown(ctx)

	// 初始状态应该是禁用的
	if adapter.IsEnabled() {
		t.Error("适配器初始状态应该是禁用的")
	}

	// 启用适配器
	err = adapter.Enable(ctx)
	if err != nil {
		t.Fatalf("启用适配器失败: %v", err)
	}

	if !adapter.IsEnabled() {
		t.Error("适配器应该是启用状态")
	}

	// 禁用适配器
	err = adapter.Disable(ctx)
	if err != nil {
		t.Fatalf("禁用适配器失败: %v", err)
	}

	if adapter.IsEnabled() {
		t.Error("适配器应该是禁用状态")
	}
}

// TestDefaultAdapter_Scan 测试设备扫描功能
func TestDefaultAdapter_Scan(t *testing.T) {
	config := DefaultAdapterConfig()
	adapter := NewDefaultAdapter(config)

	ctx := context.Background()
	err := adapter.Initialize(ctx)
	if err != nil {
		t.Fatalf("适配器初始化失败: %v", err)
	}
	defer adapter.Shutdown(ctx)

	// 测试未启用时扫描
	_, err = adapter.Scan(ctx, 1*time.Second)
	if err == nil {
		t.Error("未启用适配器时扫描应该失败")
	}

	// 启用适配器
	err = adapter.Enable(ctx)
	if err != nil {
		t.Fatalf("启用适配器失败: %v", err)
	}

	// 扫描设备
	deviceChan, err := adapter.Scan(ctx, 2*time.Second)
	if err != nil {
		t.Fatalf("扫描设备失败: %v", err)
	}

	// 在扫描进行中测试重复扫描
	_, err = adapter.Scan(ctx, 1*time.Second)
	if err == nil {
		t.Error("扫描进行中时重复扫描应该失败")
	}

	// 收集扫描到的设备
	var devices []bluetooth.Device
	timeout := time.After(3 * time.Second)

	for {
		select {
		case device, ok := <-deviceChan:
			if !ok {
				// 通道已关闭，扫描结束
				goto scanComplete
			}
			devices = append(devices, device)
			t.Logf("发现设备: %s (%s)", device.Name, device.ID)
		case <-timeout:
			// 扫描超时，停止扫描
			adapter.StopScan()
			goto scanComplete
		}
	}

scanComplete:
	// 验证扫描结果
	t.Logf("总共发现 %d 个设备", len(devices))

	// 停止扫描
	err = adapter.StopScan()
	if err != nil {
		t.Errorf("停止扫描失败: %v", err)
	}
}

// TestDefaultAdapter_Connect 测试设备连接功能
func TestDefaultAdapter_Connect(t *testing.T) {
	config := DefaultAdapterConfig()
	adapter := NewDefaultAdapter(config)

	ctx := context.Background()
	err := adapter.Initialize(ctx)
	if err != nil {
		t.Fatalf("适配器初始化失败: %v", err)
	}
	defer adapter.Shutdown(ctx)

	// 启用适配器
	err = adapter.Enable(ctx)
	if err != nil {
		t.Fatalf("启用适配器失败: %v", err)
	}

	// 测试连接到模拟设备
	deviceID := "test_device_001"
	conn, err := adapter.Connect(ctx, deviceID)
	if err != nil {
		t.Fatalf("连接设备失败: %v", err)
	}

	// 验证连接
	if conn.DeviceID() != deviceID {
		t.Errorf("连接设备ID不匹配，期望: %s, 实际: %s", deviceID, conn.DeviceID())
	}

	if !conn.IsActive() {
		t.Error("连接应该是活跃状态")
	}

	// 测试重复连接
	conn2, err := adapter.Connect(ctx, deviceID)
	if err != nil {
		t.Fatalf("重复连接失败: %v", err)
	}

	if conn.ID() != conn2.ID() {
		t.Error("重复连接应该返回相同的连接实例")
	}

	// 断开连接
	err = adapter.Disconnect(deviceID)
	if err != nil {
		t.Errorf("断开连接失败: %v", err)
	}

	// 验证连接已断开
	if conn.IsActive() {
		t.Error("连接应该已断开")
	}
}

// TestDefaultAdapter_Listen 测试监听功能
func TestDefaultAdapter_Listen(t *testing.T) {
	config := DefaultAdapterConfig()
	adapter := NewDefaultAdapter(config)

	ctx := context.Background()
	err := adapter.Initialize(ctx)
	if err != nil {
		t.Fatalf("适配器初始化失败: %v", err)
	}
	defer adapter.Shutdown(ctx)

	// 启用适配器
	err = adapter.Enable(ctx)
	if err != nil {
		t.Fatalf("启用适配器失败: %v", err)
	}

	// 开始监听
	serviceUUID := bluetooth.DefaultServiceUUID
	err = adapter.Listen(serviceUUID)
	if err != nil {
		t.Fatalf("开始监听失败: %v", err)
	}

	// 测试重复监听
	err = adapter.Listen(serviceUUID)
	if err == nil {
		t.Error("重复监听应该失败")
	}

	// 测试接受连接（带超时）
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = adapter.AcceptConnection(ctx)
	if err == nil {
		t.Log("接受到连接（在模拟环境中可能不会发生）")
	} else {
		// 超时是预期的，因为这是模拟环境
		t.Logf("接受连接超时（预期行为）: %v", err)
	}

	// 停止监听
	err = adapter.StopListen()
	if err != nil {
		t.Errorf("停止监听失败: %v", err)
	}
}

// TestDefaultAdapter_SetDiscoverable 测试可发现性设置
func TestDefaultAdapter_SetDiscoverable(t *testing.T) {
	config := DefaultAdapterConfig()
	adapter := NewDefaultAdapter(config)

	ctx := context.Background()
	err := adapter.Initialize(ctx)
	if err != nil {
		t.Fatalf("适配器初始化失败: %v", err)
	}
	defer adapter.Shutdown(ctx)

	// 启用适配器
	err = adapter.Enable(ctx)
	if err != nil {
		t.Fatalf("启用适配器失败: %v", err)
	}

	// 设置为可发现
	err = adapter.SetDiscoverable(true, 5*time.Second)
	if err != nil {
		t.Fatalf("设置可发现失败: %v", err)
	}

	info := adapter.GetLocalInfo()
	if !info.Discoverable {
		t.Error("适配器应该是可发现状态")
	}

	// 等待超时自动关闭可发现性
	time.Sleep(6 * time.Second)

	info = adapter.GetLocalInfo()
	if info.Discoverable {
		t.Error("适配器可发现性应该已超时关闭")
	}

	// 手动设置为不可发现
	adapter.SetDiscoverable(true, 0) // 无超时
	err = adapter.SetDiscoverable(false, 0)
	if err != nil {
		t.Fatalf("设置不可发现失败: %v", err)
	}

	info = adapter.GetLocalInfo()
	if info.Discoverable {
		t.Error("适配器应该是不可发现状态")
	}
}

// TestDefaultAdapterFactory 测试适配器工厂
func TestDefaultAdapterFactory(t *testing.T) {
	factory := NewDefaultAdapterFactory()

	// 测试获取可用适配器
	adapters, err := factory.GetAvailableAdapters()
	if err != nil {
		t.Fatalf("获取可用适配器失败: %v", err)
	}

	if len(adapters) == 0 {
		t.Error("应该至少有一个可用适配器")
	}

	// 测试创建适配器
	config := DefaultAdapterConfig()
	adapter, err := factory.CreateAdapter(config)
	if err != nil {
		t.Fatalf("创建适配器失败: %v", err)
	}

	if adapter == nil {
		t.Error("创建的适配器不能为空")
	}

	// 测试获取默认适配器
	defaultAdapter, err := factory.GetDefaultAdapter()
	if err != nil {
		t.Fatalf("获取默认适配器失败: %v", err)
	}

	if defaultAdapter == nil {
		t.Error("默认适配器不能为空")
	}
}

// TestAdapterConfig 测试适配器配置
func TestAdapterConfig(t *testing.T) {
	config := DefaultAdapterConfig()

	// 验证默认配置
	if config.Name == "" {
		t.Error("默认配置名称不能为空")
	}

	if config.Timeout <= 0 {
		t.Error("默认超时时间应该大于0")
	}

	if config.BufferSize <= 0 {
		t.Error("默认缓冲区大小应该大于0")
	}

	// 测试自定义配置
	customConfig := AdapterConfig{
		AdapterID:    "custom_adapter",
		Name:         "Custom Adapter",
		Timeout:      60 * time.Second,
		BufferSize:   2048,
		EnableEvents: false,
		LogLevel:     "debug",
	}

	adapter := NewDefaultAdapter(customConfig)
	if adapter.config.Name != customConfig.Name {
		t.Errorf("自定义配置名称不匹配，期望: %s, 实际: %s",
			customConfig.Name, adapter.config.Name)
	}
}
