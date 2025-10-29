package device

import (
	"context"
	"testing"
	"time"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// TestNewDiscoveryService 测试创建发现服务
func TestNewDiscoveryService(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)
	service := NewDiscoveryService(adapter, manager)

	if service == nil {
		t.Fatal("创建发现服务失败")
	}

	if service.adapter != adapter {
		t.Error("适配器设置不正确")
	}

	if service.manager != manager {
		t.Error("管理器设置不正确")
	}

	if service.running {
		t.Error("新创建的发现服务不应该处于运行状态")
	}
}

// TestDiscoveryServiceStartStop 测试发现服务启动和停止
func TestDiscoveryServiceStartStop(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)
	service := NewDiscoveryService(adapter, manager)

	// 测试启动
	err := service.Start()
	if err != nil {
		t.Fatalf("启动发现服务失败: %v", err)
	}

	if !service.running {
		t.Error("发现服务应该处于运行状态")
	}

	// 测试重复启动
	err = service.Start()
	if err == nil {
		t.Error("重复启动应该返回错误")
	}

	// 测试停止
	err = service.Stop()
	if err != nil {
		t.Fatalf("停止发现服务失败: %v", err)
	}

	if service.running {
		t.Error("发现服务应该处于停止状态")
	}

	// 测试重复停止
	err = service.Stop()
	if err == nil {
		t.Error("重复停止应该返回错误")
	}
}

// TestStartScan 测试开始扫描
func TestStartScan(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)
	service := NewDiscoveryService(adapter, manager)

	// 启动服务
	err := service.Start()
	if err != nil {
		t.Fatalf("启动发现服务失败: %v", err)
	}

	// 添加模拟设备
	mockDevice := bluetooth.Device{
		ID:       "scan_test_device",
		Name:     "Scan Test Device",
		Address:  "00:11:22:33:44:85",
		RSSI:     -50,
		LastSeen: time.Now(),
	}
	adapter.AddMockDevice(mockDevice)

	// 开始扫描
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	filter := bluetooth.DeviceFilter{
		MinRSSI: -70,
	}

	session, err := service.StartScan(ctx, filter, 2*time.Second)
	if err != nil {
		t.Fatalf("开始扫描失败: %v", err)
	}

	if session == nil {
		t.Fatal("扫描会话不应该为空")
	}

	if !session.IsActive() {
		t.Error("扫描会话应该处于活跃状态")
	}

	// 等待扫描完成并收集结果
	discoveredDevices := make([]bluetooth.Device, 0)
	timeout := time.After(2 * time.Second)

	for {
		select {
		case device, ok := <-session.GetDevices():
			if !ok {
				// 通道已关闭，扫描完成
				goto checkResults
			}
			discoveredDevices = append(discoveredDevices, device)
		case <-timeout:
			// 超时，检查结果
			goto checkResults
		}
	}

checkResults:
	// 检查扫描结果
	if len(discoveredDevices) == 0 {
		t.Error("应该发现至少一个设备")
	}
}

// TestStopScan 测试停止扫描
func TestStopScan(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)
	service := NewDiscoveryService(adapter, manager)

	// 启动服务
	err := service.Start()
	if err != nil {
		t.Fatalf("启动发现服务失败: %v", err)
	}

	// 开始扫描
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bluetooth.DeviceFilter{}
	session, err := service.StartScan(ctx, filter, 5*time.Second)
	if err != nil {
		t.Fatalf("开始扫描失败: %v", err)
	}

	// 停止扫描
	err = service.StopScan(session.ID)
	if err != nil {
		t.Fatalf("停止扫描失败: %v", err)
	}

	// 验证扫描已停止
	time.Sleep(100 * time.Millisecond)
	if session.Status != ScanStatusCancelled {
		t.Errorf("扫描状态应该是已取消: 期望 %s, 实际 %s",
			ScanStatusCancelled.String(), session.Status.String())
	}
}

// TestConcurrentScans 测试并发扫描限制
func TestConcurrentScans(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)

	// 创建有限制的发现服务配置
	config := DefaultDiscoveryConfig()
	config.MaxConcurrentScans = 1 // 设置为1，更容易测试
	service := NewDiscoveryServiceWithConfig(adapter, manager, config)

	// 启动服务
	err := service.Start()
	if err != nil {
		t.Fatalf("启动发现服务失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bluetooth.DeviceFilter{}

	// 启动第一个扫描
	session1, err := service.StartScan(ctx, filter, 10*time.Second)
	if err != nil {
		t.Fatalf("启动第一个扫描失败: %v", err)
	}

	// 立即尝试启动第二个扫描（应该失败）
	_, err = service.StartScan(ctx, filter, 10*time.Second)
	if err == nil {
		t.Error("第二个扫描应该因为达到并发限制而失败")
	}

	// 验证错误类型
	if btErr, ok := err.(*bluetooth.BluetoothError); ok {
		if btErr.Code != bluetooth.ErrCodeResourceBusy {
			t.Errorf("错误代码不正确: 期望 %d, 实际 %d", bluetooth.ErrCodeResourceBusy, btErr.Code)
		}
	}

	// 停止第一个扫描
	err = service.StopScan(session1.ID)
	if err != nil {
		t.Fatalf("停止扫描失败: %v", err)
	}

	// 等待清理
	time.Sleep(100 * time.Millisecond)

	// 现在应该可以启动新的扫描
	_, err = service.StartScan(ctx, filter, 10*time.Second)
	if err != nil {
		t.Fatalf("停止一个扫描后应该可以启动新扫描: %v", err)
	}
}

// TestGetActiveScanSessions 测试获取活跃扫描会话
func TestGetActiveScanSessions(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)
	service := NewDiscoveryService(adapter, manager)

	// 启动服务
	err := service.Start()
	if err != nil {
		t.Fatalf("启动发现服务失败: %v", err)
	}

	// 初始应该没有活跃会话
	sessions := service.GetActiveScanSessions()
	if len(sessions) != 0 {
		t.Errorf("初始活跃会话数量应该为0: 实际 %d", len(sessions))
	}

	// 启动扫描
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bluetooth.DeviceFilter{}
	session, err := service.StartScan(ctx, filter, 5*time.Second)
	if err != nil {
		t.Fatalf("启动扫描失败: %v", err)
	}

	// 应该有一个活跃会话
	sessions = service.GetActiveScanSessions()
	if len(sessions) != 1 {
		t.Errorf("活跃会话数量应该为1: 实际 %d", len(sessions))
	}

	if sessions[0].ID != session.ID {
		t.Error("返回的会话ID不匹配")
	}

	// 停止扫描
	service.StopScan(session.ID)

	// 等待会话清理
	time.Sleep(100 * time.Millisecond)

	// 应该没有活跃会话
	sessions = service.GetActiveScanSessions()
	if len(sessions) != 0 {
		t.Errorf("停止扫描后活跃会话数量应该为0: 实际 %d", len(sessions))
	}
}

// TestScanSessionStats 测试扫描会话统计信息
func TestScanSessionStats(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)
	service := NewDiscoveryService(adapter, manager)

	// 启动服务
	err := service.Start()
	if err != nil {
		t.Fatalf("启动发现服务失败: %v", err)
	}

	// 添加模拟设备
	mockDevice := bluetooth.Device{
		ID:       "stats_test_device",
		Name:     "Stats Test Device",
		Address:  "00:11:22:33:44:86",
		RSSI:     -40,
		LastSeen: time.Now(),
	}
	adapter.AddMockDevice(mockDevice)

	// 开始扫描
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	filter := bluetooth.DeviceFilter{}
	session, err := service.StartScan(ctx, filter, 1*time.Second)
	if err != nil {
		t.Fatalf("开始扫描失败: %v", err)
	}

	// 等待扫描完成
	time.Sleep(1500 * time.Millisecond)

	// 获取统计信息
	stats := session.GetStats()

	if stats["session_id"] != session.ID {
		t.Error("统计信息中的会话ID不匹配")
	}

	if stats["device_count"].(int) == 0 {
		t.Error("统计信息中应该显示发现了设备")
	}

	if stats["status"] == "" {
		t.Error("统计信息中应该包含状态")
	}
}

// TestScanWithFilter 测试带过滤条件的扫描
func TestScanWithFilter(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)
	service := NewDiscoveryService(adapter, manager)

	// 启动服务
	err := service.Start()
	if err != nil {
		t.Fatalf("启动发现服务失败: %v", err)
	}

	// 添加不同信号强度的模拟设备
	strongDevice := bluetooth.Device{
		ID:       "strong_device",
		Name:     "Strong Device",
		Address:  "00:11:22:33:44:87",
		RSSI:     -30, // 强信号
		LastSeen: time.Now(),
	}

	weakDevice := bluetooth.Device{
		ID:       "weak_device",
		Name:     "Weak Device",
		Address:  "00:11:22:33:44:88",
		RSSI:     -80, // 弱信号
		LastSeen: time.Now(),
	}

	adapter.AddMockDevice(strongDevice)
	adapter.AddMockDevice(weakDevice)

	// 使用RSSI过滤器扫描
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	filter := bluetooth.DeviceFilter{
		MinRSSI: -50, // 只接受信号强度大于-50的设备
	}

	session, err := service.StartScan(ctx, filter, 1*time.Second)
	if err != nil {
		t.Fatalf("开始扫描失败: %v", err)
	}

	// 收集扫描结果
	discoveredDevices := make([]bluetooth.Device, 0)
	for device := range session.GetDevices() {
		discoveredDevices = append(discoveredDevices, device)
	}

	// 验证只发现了强信号设备
	foundStrong := false
	foundWeak := false

	for _, device := range discoveredDevices {
		if device.ID == strongDevice.ID {
			foundStrong = true
		}
		if device.ID == weakDevice.ID {
			foundWeak = true
		}
	}

	if !foundStrong {
		t.Error("应该发现强信号设备")
	}

	if foundWeak {
		t.Error("不应该发现弱信号设备")
	}
}

// TestScanTimeout 测试扫描超时
func TestScanTimeout(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)
	service := NewDiscoveryService(adapter, manager)

	// 启动服务
	err := service.Start()
	if err != nil {
		t.Fatalf("启动发现服务失败: %v", err)
	}

	// 开始短时间扫描
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	filter := bluetooth.DeviceFilter{}
	session, err := service.StartScan(ctx, filter, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("开始扫描失败: %v", err)
	}

	// 等待超时
	time.Sleep(300 * time.Millisecond)

	// 验证扫描已完成或取消
	if session.IsActive() {
		t.Error("扫描应该已经超时完成")
	}
}
