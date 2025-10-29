package device

import (
	"context"
	"testing"
	"time"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// TestNewPairingService 测试创建配对服务
func TestNewPairingService(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)
	service := NewPairingService(adapter, manager)

	if service == nil {
		t.Fatal("创建配对服务失败")
	}

	if service.adapter != adapter {
		t.Error("适配器设置不正确")
	}

	if service.manager != manager {
		t.Error("管理器设置不正确")
	}

	if service.running {
		t.Error("新创建的配对服务不应该处于运行状态")
	}
}

// TestPairingServiceStartStop 测试配对服务启动和停止
func TestPairingServiceStartStop(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)
	service := NewPairingService(adapter, manager)

	// 测试启动
	err := service.Start()
	if err != nil {
		t.Fatalf("启动配对服务失败: %v", err)
	}

	if !service.running {
		t.Error("配对服务应该处于运行状态")
	}

	// 测试重复启动
	err = service.Start()
	if err == nil {
		t.Error("重复启动应该返回错误")
	}

	// 测试停止
	err = service.Stop()
	if err != nil {
		t.Fatalf("停止配对服务失败: %v", err)
	}

	if service.running {
		t.Error("配对服务应该处于停止状态")
	}

	// 测试重复停止
	err = service.Stop()
	if err == nil {
		t.Error("重复停止应该返回错误")
	}
}

// TestStartPairing 测试开始配对
func TestStartPairing(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)
	service := NewPairingService(adapter, manager)

	// 启动服务
	err := service.Start()
	if err != nil {
		t.Fatalf("启动配对服务失败: %v", err)
	}

	// 添加测试设备
	device := bluetooth.Device{
		ID:       "pairing_test_device",
		Name:     "Pairing Test Device",
		Address:  "00:11:22:33:44:89",
		RSSI:     -50,
		LastSeen: time.Now(),
	}
	manager.AddDevice(device)

	// 开始配对
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := service.StartPairing(ctx, device.ID, PairingMethodConfirmation)
	if err != nil {
		t.Fatalf("开始配对失败: %v", err)
	}

	if session == nil {
		t.Fatal("配对会话不应该为空")
	}

	if session.DeviceID != device.ID {
		t.Errorf("配对会话设备ID不匹配: 期望 %s, 实际 %s", device.ID, session.DeviceID)
	}

	if session.Method != PairingMethodConfirmation {
		t.Errorf("配对方法不匹配: 期望 %s, 实际 %s",
			PairingMethodConfirmation.String(), session.Method.String())
	}
}

// TestPairingWithNonExistentDevice 测试与不存在设备的配对
func TestPairingWithNonExistentDevice(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)
	service := NewPairingService(adapter, manager)

	// 启动服务
	err := service.Start()
	if err != nil {
		t.Fatalf("启动配对服务失败: %v", err)
	}

	// 尝试与不存在的设备配对
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = service.StartPairing(ctx, "non_existent_device", PairingMethodConfirmation)
	if err == nil {
		t.Error("与不存在设备的配对应该失败")
	}
}

// TestPairingAlreadyPairedDevice 测试与已配对设备的配对
func TestPairingAlreadyPairedDevice(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)
	service := NewPairingService(adapter, manager)

	// 启动服务
	err := service.Start()
	if err != nil {
		t.Fatalf("启动配对服务失败: %v", err)
	}

	// 添加并配对设备
	device := bluetooth.Device{
		ID:       "already_paired_device",
		Name:     "Already Paired Device",
		Address:  "00:11:22:33:44:90",
		RSSI:     -50,
		LastSeen: time.Now(),
	}
	manager.AddDevice(device)
	manager.PairDevice(device.ID)

	// 尝试再次配对
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = service.StartPairing(ctx, device.ID, PairingMethodConfirmation)
	if err == nil {
		t.Error("与已配对设备的配对应该失败")
	}
}

// TestCancelPairing 测试取消配对
func TestCancelPairing(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)
	service := NewPairingService(adapter, manager)

	// 启动服务
	err := service.Start()
	if err != nil {
		t.Fatalf("启动配对服务失败: %v", err)
	}

	// 添加测试设备
	device := bluetooth.Device{
		ID:       "cancel_pairing_device",
		Name:     "Cancel Pairing Device",
		Address:  "00:11:22:33:44:91",
		RSSI:     -50,
		LastSeen: time.Now(),
	}
	manager.AddDevice(device)

	// 开始配对
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := service.StartPairing(ctx, device.ID, PairingMethodConfirmation)
	if err != nil {
		t.Fatalf("开始配对失败: %v", err)
	}

	// 取消配对
	err = service.CancelPairing(session.ID)
	if err != nil {
		t.Fatalf("取消配对失败: %v", err)
	}

	// 验证配对已取消
	time.Sleep(100 * time.Millisecond)
	if session.Status != PairingStatusCancelled && session.Status != PairingStatusTimeout {
		t.Errorf("配对状态应该是已取消或超时: 期望 %s 或 %s, 实际 %s",
			PairingStatusCancelled.String(), PairingStatusTimeout.String(), session.Status.String())
	}
}

// TestConfirmPairing 测试确认配对
func TestConfirmPairing(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)

	// 创建自动接受配对的配置
	config := DefaultPairingConfig()
	config.AutoAccept = false // 需要手动确认
	service := NewPairingServiceWithConfig(adapter, manager, config)

	// 启动服务
	err := service.Start()
	if err != nil {
		t.Fatalf("启动配对服务失败: %v", err)
	}

	// 添加测试设备
	device := bluetooth.Device{
		ID:       "confirm_pairing_device",
		Name:     "Confirm Pairing Device",
		Address:  "00:11:22:33:44:92",
		RSSI:     -50,
		LastSeen: time.Now(),
	}
	manager.AddDevice(device)

	// 开始配对
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := service.StartPairing(ctx, device.ID, PairingMethodConfirmation)
	if err != nil {
		t.Fatalf("开始配对失败: %v", err)
	}

	// 等待配对进入等待确认状态
	time.Sleep(100 * time.Millisecond)

	// 确认配对
	err = service.ConfirmPairing(session.ID, true)
	if err != nil {
		t.Fatalf("确认配对失败: %v", err)
	}

	// 等待配对完成
	time.Sleep(500 * time.Millisecond)

	// 验证配对已完成
	if session.Status != PairingStatusCompleted {
		t.Errorf("配对状态应该是已完成: 期望 %s, 实际 %s",
			PairingStatusCompleted.String(), session.Status.String())
	}

	// 验证设备已配对
	if !manager.IsDevicePaired(device.ID) {
		t.Error("设备应该处于已配对状态")
	}
}

// TestRejectPairing 测试拒绝配对
func TestRejectPairing(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)

	// 创建需要手动确认的配置
	config := DefaultPairingConfig()
	config.AutoAccept = false
	service := NewPairingServiceWithConfig(adapter, manager, config)

	// 启动服务
	err := service.Start()
	if err != nil {
		t.Fatalf("启动配对服务失败: %v", err)
	}

	// 添加测试设备
	device := bluetooth.Device{
		ID:       "reject_pairing_device",
		Name:     "Reject Pairing Device",
		Address:  "00:11:22:33:44:93",
		RSSI:     -50,
		LastSeen: time.Now(),
	}
	manager.AddDevice(device)

	// 开始配对
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := service.StartPairing(ctx, device.ID, PairingMethodConfirmation)
	if err != nil {
		t.Fatalf("开始配对失败: %v", err)
	}

	// 等待配对进入等待确认状态
	time.Sleep(100 * time.Millisecond)

	// 拒绝配对
	err = service.ConfirmPairing(session.ID, false)
	if err != nil {
		t.Fatalf("拒绝配对失败: %v", err)
	}

	// 等待配对处理完成
	time.Sleep(500 * time.Millisecond)

	// 验证配对已取消
	if session.Status != PairingStatusCancelled {
		t.Errorf("配对状态应该是已取消: 期望 %s, 实际 %s",
			PairingStatusCancelled.String(), session.Status.String())
	}

	// 验证设备未配对
	if manager.IsDevicePaired(device.ID) {
		t.Error("设备不应该处于已配对状态")
	}
}

// TestPINPairing 测试PIN码配对
func TestPINPairing(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)
	service := NewPairingService(adapter, manager)

	// 启动服务
	err := service.Start()
	if err != nil {
		t.Fatalf("启动配对服务失败: %v", err)
	}

	// 添加测试设备
	device := bluetooth.Device{
		ID:       "pin_pairing_device",
		Name:     "PIN Pairing Device",
		Address:  "00:11:22:33:44:94",
		RSSI:     -50,
		LastSeen: time.Now(),
	}
	manager.AddDevice(device)

	// 开始PIN码配对
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := service.StartPairing(ctx, device.ID, PairingMethodPIN)
	if err != nil {
		t.Fatalf("开始PIN码配对失败: %v", err)
	}

	// 提供PIN码
	err = service.ProvidePIN(session.ID, "123456")
	if err != nil {
		t.Fatalf("提供PIN码失败: %v", err)
	}

	// 等待配对完成
	time.Sleep(3 * time.Second)

	// 验证配对已完成
	if session.Status != PairingStatusCompleted {
		t.Errorf("配对状态应该是已完成: 期望 %s, 实际 %s",
			PairingStatusCompleted.String(), session.Status.String())
	}
}

// TestPasskeyPairing 测试数字密钥配对
func TestPasskeyPairing(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)

	// 创建需要手动确认的配置
	config := DefaultPairingConfig()
	config.AutoAccept = false
	service := NewPairingServiceWithConfig(adapter, manager, config)

	// 启动服务
	err := service.Start()
	if err != nil {
		t.Fatalf("启动配对服务失败: %v", err)
	}

	// 添加测试设备
	device := bluetooth.Device{
		ID:       "passkey_pairing_device",
		Name:     "Passkey Pairing Device",
		Address:  "00:11:22:33:44:95",
		RSSI:     -50,
		LastSeen: time.Now(),
	}
	manager.AddDevice(device)

	// 开始数字密钥配对
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := service.StartPairing(ctx, device.ID, PairingMethodPasskey)
	if err != nil {
		t.Fatalf("开始数字密钥配对失败: %v", err)
	}

	// 等待生成密钥
	time.Sleep(100 * time.Millisecond)

	// 验证生成了密钥
	if session.Passkey == 0 {
		t.Error("应该生成数字密钥")
	}

	// 确认配对
	err = service.ConfirmPairing(session.ID, true)
	if err != nil {
		t.Fatalf("确认配对失败: %v", err)
	}

	// 等待配对完成
	time.Sleep(500 * time.Millisecond)

	// 验证配对已完成
	if session.Status != PairingStatusCompleted {
		t.Errorf("配对状态应该是已完成: 期望 %s, 实际 %s",
			PairingStatusCompleted.String(), session.Status.String())
	}
}

// TestConcurrentPairings 测试并发配对限制
func TestConcurrentPairings(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)

	// 创建有限制的配对服务配置
	config := DefaultPairingConfig()
	config.MaxConcurrentPairs = 1            // 设置为1，更容易测试
	config.DefaultTimeout = 30 * time.Second // 使用长超时时间
	service := NewPairingServiceWithConfig(adapter, manager, config)

	// 启动服务
	err := service.Start()
	if err != nil {
		t.Fatalf("启动配对服务失败: %v", err)
	}

	// 添加测试设备
	devices := []bluetooth.Device{
		{
			ID:       "concurrent_device_1",
			Name:     "Concurrent Device 1",
			Address:  "00:11:22:33:44:96",
			RSSI:     -50,
			LastSeen: time.Now(),
		},
		{
			ID:       "concurrent_device_2",
			Name:     "Concurrent Device 2",
			Address:  "00:11:22:33:44:97",
			RSSI:     -50,
			LastSeen: time.Now(),
		},
		{
			ID:       "concurrent_device_3",
			Name:     "Concurrent Device 3",
			Address:  "00:11:22:33:44:98",
			RSSI:     -50,
			LastSeen: time.Now(),
		},
	}

	for _, device := range devices {
		manager.AddDevice(device)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 启动第一个配对
	session1, err := service.StartPairing(ctx, devices[0].ID, PairingMethodConfirmation)
	if err != nil {
		t.Fatalf("启动第一个配对失败: %v", err)
	}

	// 立即尝试启动第二个配对（应该失败）
	_, err = service.StartPairing(ctx, devices[1].ID, PairingMethodConfirmation)
	if err == nil {
		t.Error("第二个配对应该因为达到并发限制而失败")
	}

	// 验证错误类型
	if btErr, ok := err.(*bluetooth.BluetoothError); ok {
		if btErr.Code != bluetooth.ErrCodeResourceBusy {
			t.Errorf("错误代码不正确: 期望 %d, 实际 %d", bluetooth.ErrCodeResourceBusy, btErr.Code)
		}
	}

	// 取消第一个配对
	err = service.CancelPairing(session1.ID)
	if err != nil {
		t.Fatalf("取消配对失败: %v", err)
	}

	// 等待清理
	time.Sleep(100 * time.Millisecond)

	// 现在应该可以启动新的配对
	_, err = service.StartPairing(ctx, devices[1].ID, PairingMethodConfirmation)
	if err != nil {
		t.Fatalf("取消一个配对后应该可以启动新配对: %v", err)
	}
}

// TestGetActivePairingSessions 测试获取活跃配对会话
func TestGetActivePairingSessions(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)
	service := NewPairingService(adapter, manager)

	// 启动服务
	err := service.Start()
	if err != nil {
		t.Fatalf("启动配对服务失败: %v", err)
	}

	// 初始应该没有活跃会话
	sessions := service.GetActivePairingSessions()
	if len(sessions) != 0 {
		t.Errorf("初始活跃配对会话数量应该为0: 实际 %d", len(sessions))
	}

	// 添加测试设备
	device := bluetooth.Device{
		ID:       "active_sessions_device",
		Name:     "Active Sessions Device",
		Address:  "00:11:22:33:44:99",
		RSSI:     -50,
		LastSeen: time.Now(),
	}
	manager.AddDevice(device)

	// 启动配对
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := service.StartPairing(ctx, device.ID, PairingMethodConfirmation)
	if err != nil {
		t.Fatalf("启动配对失败: %v", err)
	}

	// 应该有一个活跃会话
	sessions = service.GetActivePairingSessions()
	if len(sessions) != 1 {
		t.Errorf("活跃配对会话数量应该为1: 实际 %d", len(sessions))
	}

	if sessions[0].ID != session.ID {
		t.Error("返回的会话ID不匹配")
	}

	// 取消配对
	service.CancelPairing(session.ID)

	// 等待会话清理
	time.Sleep(100 * time.Millisecond)

	// 应该没有活跃会话
	sessions = service.GetActivePairingSessions()
	if len(sessions) != 0 {
		t.Errorf("取消配对后活跃会话数量应该为0: 实际 %d", len(sessions))
	}
}

// TestPairingSessionStats 测试配对会话统计信息
func TestPairingSessionStats(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)
	service := NewPairingService(adapter, manager)

	// 启动服务
	err := service.Start()
	if err != nil {
		t.Fatalf("启动配对服务失败: %v", err)
	}

	// 添加测试设备
	device := bluetooth.Device{
		ID:       "stats_pairing_device",
		Name:     "Stats Pairing Device",
		Address:  "00:11:22:33:44:100",
		RSSI:     -50,
		LastSeen: time.Now(),
	}
	manager.AddDevice(device)

	// 开始配对
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := service.StartPairing(ctx, device.ID, PairingMethodConfirmation)
	if err != nil {
		t.Fatalf("开始配对失败: %v", err)
	}

	// 获取统计信息
	stats := session.GetStats()

	if stats["session_id"] != session.ID {
		t.Error("统计信息中的会话ID不匹配")
	}

	if stats["device_id"] != device.ID {
		t.Error("统计信息中的设备ID不匹配")
	}

	if stats["device_name"] != device.Name {
		t.Error("统计信息中的设备名称不匹配")
	}

	if stats["method"] != session.Method.String() {
		t.Error("统计信息中的配对方法不匹配")
	}

	if stats["status"] == "" {
		t.Error("统计信息中应该包含状态")
	}
}
