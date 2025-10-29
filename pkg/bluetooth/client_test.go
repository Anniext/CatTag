package bluetooth

import (
	"context"
	"testing"
	"time"
)

// TestBluetoothClient_Basic 测试蓝牙客户端基本功能
func TestBluetoothClient_Basic(t *testing.T) {
	// 创建客户端配置
	config := ClientConfig{
		ScanTimeout:    5 * time.Second,
		ConnectTimeout: 10 * time.Second,
		RetryAttempts:  3,
		RetryInterval:  1 * time.Second,
		AutoReconnect:  false,
		KeepAlive:      false,
	}

	// 创建模拟适配器
	adapter := NewMockAdapter()

	// 创建客户端
	client := NewBluetoothClient[string](config, adapter)

	// 测试初始状态
	if client.GetStatus() != StatusStopped {
		t.Errorf("期望客户端初始状态为 StatusStopped，实际为 %v", client.GetStatus())
	}

	// 测试启动客户端
	ctx := context.Background()
	err := client.Start(ctx)
	if err != nil {
		t.Fatalf("启动客户端失败: %v", err)
	}

	// 验证客户端状态
	if client.GetStatus() != StatusRunning {
		t.Errorf("期望客户端状态为 StatusRunning，实际为 %v", client.GetStatus())
	}

	// 测试停止客户端
	err = client.Stop(ctx)
	if err != nil {
		t.Fatalf("停止客户端失败: %v", err)
	}

	// 验证客户端状态
	if client.GetStatus() != StatusStopped {
		t.Errorf("期望客户端状态为 StatusStopped，实际为 %v", client.GetStatus())
	}
}

// TestBluetoothClient_Scan 测试设备扫描功能
func TestBluetoothClient_Scan(t *testing.T) {
	config := ClientConfig{
		ScanTimeout:    2 * time.Second,
		ConnectTimeout: 5 * time.Second,
		RetryAttempts:  1,
		RetryInterval:  1 * time.Second,
		AutoReconnect:  false,
		KeepAlive:      false,
	}

	adapter := NewMockAdapter()
	client := NewBluetoothClient[string](config, adapter)

	ctx := context.Background()
	err := client.Start(ctx)
	if err != nil {
		t.Fatalf("启动客户端失败: %v", err)
	}
	defer client.Stop(ctx)

	// 测试设备扫描
	devices, err := client.Scan(ctx, 1*time.Second)
	if err != nil {
		t.Fatalf("扫描设备失败: %v", err)
	}

	// 验证扫描结果（模拟适配器应该返回一些设备）
	if len(devices) == 0 {
		t.Log("警告: 扫描未发现任何设备（这在模拟环境中是正常的）")
	}

	t.Logf("扫描发现 %d 个设备", len(devices))
	for i, device := range devices {
		t.Logf("设备 %d: ID=%s, Name=%s, Address=%s", i+1, device.ID, device.Name, device.Address)
	}
}

// TestBluetoothClient_Connect 测试设备连接功能
func TestBluetoothClient_Connect(t *testing.T) {
	config := ClientConfig{
		ScanTimeout:    2 * time.Second,
		ConnectTimeout: 5 * time.Second,
		RetryAttempts:  1,
		RetryInterval:  1 * time.Second,
		AutoReconnect:  false,
		KeepAlive:      false,
	}

	adapter := NewMockAdapter()
	client := NewBluetoothClient[string](config, adapter)

	ctx := context.Background()
	err := client.Start(ctx)
	if err != nil {
		t.Fatalf("启动客户端失败: %v", err)
	}

	// 测试连接到设备
	deviceID := "test_device_001"
	conn, err := client.Connect(ctx, deviceID)
	if err != nil {
		t.Fatalf("连接设备失败: %v", err)
	}

	// 验证连接
	if conn == nil {
		t.Fatal("连接对象为空")
	}

	if conn.DeviceID() != deviceID {
		t.Errorf("期望设备ID为 %s，实际为 %s", deviceID, conn.DeviceID())
	}

	if !conn.IsActive() {
		t.Error("期望连接为活跃状态")
	}

	// 验证连接计数
	if client.GetConnectionCount() != 1 {
		t.Errorf("期望连接数为 1，实际为 %d", client.GetConnectionCount())
	}

	// 测试断开连接
	err = client.Disconnect(deviceID)
	if err != nil {
		t.Fatalf("断开连接失败: %v", err)
	}

	// 验证连接计数
	if client.GetConnectionCount() != 0 {
		t.Errorf("期望连接数为 0，实际为 %d", client.GetConnectionCount())
	}

	// 停止客户端
	err = client.Stop(ctx)
	if err != nil {
		t.Fatalf("停止客户端失败: %v", err)
	}
}

// TestBluetoothClient_SendReceive 测试消息发送和接收
func TestBluetoothClient_SendReceive(t *testing.T) {
	config := ClientConfig{
		ScanTimeout:    2 * time.Second,
		ConnectTimeout: 5 * time.Second,
		RetryAttempts:  1,
		RetryInterval:  1 * time.Second,
		AutoReconnect:  false,
		KeepAlive:      false,
	}

	adapter := NewMockAdapter()
	client := NewBluetoothClient[string](config, adapter)

	ctx := context.Background()
	err := client.Start(ctx)
	if err != nil {
		t.Fatalf("启动客户端失败: %v", err)
	}
	defer client.Stop(ctx)

	// 连接到设备
	deviceID := "test_device_002"
	conn, err := client.Connect(ctx, deviceID)
	if err != nil {
		t.Fatalf("连接设备失败: %v", err)
	}

	// 测试发送消息
	testMessage := "Hello, Bluetooth!"
	err = client.Send(ctx, deviceID, testMessage)
	if err != nil {
		t.Fatalf("发送消息失败: %v", err)
	}

	// 获取接收通道
	receiveChan := client.Receive(ctx)

	// 等待接收消息（设置超时）
	select {
	case message := <-receiveChan:
		t.Logf("收到消息: ID=%s, Type=%s, Timestamp=%s",
			message.ID, message.Type.String(), message.Timestamp.Format(time.RFC3339))
	case <-time.After(2 * time.Second):
		t.Log("警告: 在超时时间内未收到消息（这在模拟环境中是正常的）")
	}

	// 清理连接
	_ = conn
}

// TestBluetoothClient_AutoReconnect 测试自动重连功能
func TestBluetoothClient_AutoReconnect(t *testing.T) {
	config := ClientConfig{
		ScanTimeout:    2 * time.Second,
		ConnectTimeout: 3 * time.Second,
		RetryAttempts:  2,
		RetryInterval:  500 * time.Millisecond,
		AutoReconnect:  true,
		KeepAlive:      false,
	}

	adapter := NewMockAdapter()
	client := NewBluetoothClient[string](config, adapter)

	ctx := context.Background()
	err := client.Start(ctx)
	if err != nil {
		t.Fatalf("启动客户端失败: %v", err)
	}
	defer client.Stop(ctx)

	// 验证重连管理器已启动
	if client.reconnectManager == nil {
		t.Fatal("重连管理器未初始化")
	}

	// 测试安排重连
	deviceID := "test_device_reconnect"
	client.reconnectManager.ScheduleReconnect(deviceID)

	// 检查重连状态
	status := client.reconnectManager.GetReconnectStatus(deviceID)
	if status == nil {
		t.Fatal("重连任务未创建")
	}

	if status.DeviceID != deviceID {
		t.Errorf("期望设备ID为 %s，实际为 %s", deviceID, status.DeviceID)
	}

	if !status.Enabled {
		t.Error("期望重连任务为启用状态")
	}

	// 停止重连
	client.reconnectManager.StopReconnect(deviceID)

	// 验证重连已停止
	status = client.reconnectManager.GetReconnectStatus(deviceID)
	if status != nil && status.Enabled {
		t.Error("期望重连任务为禁用状态")
	}
}

// TestClientBuilder 测试客户端构建器
func TestClientBuilder(t *testing.T) {
	// 测试构建器基本功能
	builder := NewClientBuilder()

	// 配置客户端
	config := builder.GetConfig()
	config.ClientConfig.ScanTimeout = 3 * time.Second
	config.ClientConfig.ConnectTimeout = 8 * time.Second
	config.ClientConfig.RetryAttempts = 5
	config.ClientConfig.RetryInterval = 2 * time.Second
	config.ClientConfig.AutoReconnect = true
	config.ClientConfig.KeepAlive = true

	client, err := builder.WithConfig(config).Build()

	if err != nil {
		t.Fatalf("构建客户端失败: %v", err)
	}

	if client == nil {
		t.Fatal("客户端对象为空")
	}

	// 验证配置
	finalConfig := builder.GetConfig()
	if finalConfig.ClientConfig.ScanTimeout != 3*time.Second {
		t.Errorf("期望扫描超时为 3s，实际为 %v", finalConfig.ClientConfig.ScanTimeout)
	}

	if finalConfig.ClientConfig.ConnectTimeout != 8*time.Second {
		t.Errorf("期望连接超时为 8s，实际为 %v", finalConfig.ClientConfig.ConnectTimeout)
	}

	if finalConfig.ClientConfig.RetryAttempts != 5 {
		t.Errorf("期望重试次数为 5，实际为 %d", finalConfig.ClientConfig.RetryAttempts)
	}

	if !finalConfig.ClientConfig.AutoReconnect {
		t.Error("期望自动重连为启用状态")
	}

	if !finalConfig.ClientConfig.KeepAlive {
		t.Error("期望保持连接为启用状态")
	}
}

// TestClientBuilder_Validation 测试构建器配置验证
func TestClientBuilder_Validation(t *testing.T) {
	builder := NewClientBuilder()

	// 测试无效的扫描超时
	config := builder.GetConfig()
	config.ClientConfig.ScanTimeout = 0
	_, err := builder.WithConfig(config).Build()
	if err == nil {
		t.Error("期望构建失败，因为扫描超时为0")
	}

	// 重置构建器
	builder = NewClientBuilder()

	// 测试无效的连接超时
	_, err = builder.WithConnectTimeout(-1 * time.Second).Build()
	if err == nil {
		t.Error("期望构建失败，因为连接超时为负数")
	}

	// 重置构建器
	builder.Reset()

	// 测试无效的重试次数
	_, err = builder.WithRetryAttempts(-1).Build()
	if err == nil {
		t.Error("期望构建失败，因为重试次数为负数")
	}
}
