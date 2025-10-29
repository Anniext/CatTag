package device

import (
	"context"
	"testing"
	"time"

	"github.com/Anniext/CatTag/internal/adapter"
	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// MockAdapter 模拟蓝牙适配器
type MockAdapter struct {
	enabled     bool
	devices     []bluetooth.Device
	scanChan    chan bluetooth.Device
	scanTimeout time.Duration
}

// NewMockAdapter 创建模拟适配器
func NewMockAdapter() *MockAdapter {
	return &MockAdapter{
		enabled:     true,
		devices:     make([]bluetooth.Device, 0),
		scanChan:    make(chan bluetooth.Device, 10),
		scanTimeout: 5 * time.Second,
	}
}

// Initialize 初始化适配器
func (ma *MockAdapter) Initialize(ctx context.Context) error {
	return nil
}

// Shutdown 关闭适配器
func (ma *MockAdapter) Shutdown(ctx context.Context) error {
	return nil
}

// IsEnabled 检查是否启用
func (ma *MockAdapter) IsEnabled() bool {
	return ma.enabled
}

// Enable 启用适配器
func (ma *MockAdapter) Enable(ctx context.Context) error {
	ma.enabled = true
	return nil
}

// Disable 禁用适配器
func (ma *MockAdapter) Disable(ctx context.Context) error {
	ma.enabled = false
	return nil
}

// Scan 扫描设备
func (ma *MockAdapter) Scan(ctx context.Context, timeout time.Duration) (<-chan bluetooth.Device, error) {
	if !ma.enabled {
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "适配器未启用")
	}

	deviceChan := make(chan bluetooth.Device, 10)

	go func() {
		defer close(deviceChan)

		// 添加延迟，模拟真实的扫描过程
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		deviceIndex := 0
		for {
			select {
			case <-ticker.C:
				if deviceIndex < len(ma.devices) {
					select {
					case deviceChan <- ma.devices[deviceIndex]:
						deviceIndex++
					case <-ctx.Done():
						return
					}
				}
			case <-ctx.Done():
				return
			case <-time.After(timeout):
				return
			}
		}
	}()

	return deviceChan, nil
}

// StopScan 停止扫描
func (ma *MockAdapter) StopScan() error {
	return nil
}

// Connect 连接设备
func (ma *MockAdapter) Connect(ctx context.Context, deviceID string) (bluetooth.Connection, error) {
	return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "模拟适配器不支持连接")
}

// Disconnect 断开连接
func (ma *MockAdapter) Disconnect(deviceID string) error {
	return nil
}

// Listen 监听连接
func (ma *MockAdapter) Listen(serviceUUID string) error {
	return nil
}

// StopListen 停止监听
func (ma *MockAdapter) StopListen() error {
	return nil
}

// AcceptConnection 接受连接
func (ma *MockAdapter) AcceptConnection(ctx context.Context) (bluetooth.Connection, error) {
	return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "模拟适配器不支持接受连接")
}

// GetLocalInfo 获取本地信息
func (ma *MockAdapter) GetLocalInfo() adapter.AdapterInfo {
	return adapter.AdapterInfo{
		ID:      "mock_adapter",
		Name:    "Mock Adapter",
		Address: "00:11:22:33:44:55",
	}
}

// SetDiscoverable 设置可发现性
func (ma *MockAdapter) SetDiscoverable(discoverable bool, timeout time.Duration) error {
	return nil
}

// GetConnectedDevices 获取已连接设备
func (ma *MockAdapter) GetConnectedDevices() []bluetooth.Device {
	return []bluetooth.Device{}
}

// AddMockDevice 添加模拟设备
func (ma *MockAdapter) AddMockDevice(device bluetooth.Device) {
	ma.devices = append(ma.devices, device)
}

// TestNewManager 测试创建设备管理器
func TestNewManager(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)

	if manager == nil {
		t.Fatal("创建设备管理器失败")
	}

	if manager.adapter != adapter {
		t.Error("适配器设置不正确")
	}

	if manager.running {
		t.Error("新创建的管理器不应该处于运行状态")
	}
}

// TestManagerStartStop 测试管理器启动和停止
func TestManagerStartStop(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)

	// 测试启动
	err := manager.Start()
	if err != nil {
		t.Fatalf("启动管理器失败: %v", err)
	}

	if !manager.running {
		t.Error("管理器应该处于运行状态")
	}

	// 测试重复启动
	err = manager.Start()
	if err == nil {
		t.Error("重复启动应该返回错误")
	}

	// 测试停止
	err = manager.Stop()
	if err != nil {
		t.Fatalf("停止管理器失败: %v", err)
	}

	if manager.running {
		t.Error("管理器应该处于停止状态")
	}

	// 测试重复停止
	err = manager.Stop()
	if err == nil {
		t.Error("重复停止应该返回错误")
	}
}

// TestAddDevice 测试添加设备
func TestAddDevice(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)

	device := bluetooth.Device{
		ID:       "test_device_1",
		Name:     "Test Device 1",
		Address:  "00:11:22:33:44:66",
		RSSI:     -50,
		LastSeen: time.Now(),
	}

	// 添加设备
	manager.AddDevice(device)

	// 验证设备已添加
	retrievedDevice, err := manager.GetDevice(device.ID)
	if err != nil {
		t.Fatalf("获取设备失败: %v", err)
	}

	if retrievedDevice.ID != device.ID {
		t.Errorf("设备ID不匹配: 期望 %s, 实际 %s", device.ID, retrievedDevice.ID)
	}

	if retrievedDevice.Name != device.Name {
		t.Errorf("设备名称不匹配: 期望 %s, 实际 %s", device.Name, retrievedDevice.Name)
	}
}

// TestRemoveDevice 测试移除设备
func TestRemoveDevice(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)

	device := bluetooth.Device{
		ID:       "test_device_2",
		Name:     "Test Device 2",
		Address:  "00:11:22:33:44:67",
		RSSI:     -60,
		LastSeen: time.Now(),
	}

	// 添加设备
	manager.AddDevice(device)

	// 验证设备存在
	_, err := manager.GetDevice(device.ID)
	if err != nil {
		t.Fatalf("设备应该存在: %v", err)
	}

	// 移除设备
	manager.RemoveDevice(device.ID)

	// 验证设备已移除
	_, err = manager.GetDevice(device.ID)
	if err == nil {
		t.Error("设备应该已被移除")
	}
}

// TestDeviceConnection 测试设备连接管理
func TestDeviceConnection(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)

	device := bluetooth.Device{
		ID:       "test_device_3",
		Name:     "Test Device 3",
		Address:  "00:11:22:33:44:68",
		RSSI:     -40,
		LastSeen: time.Now(),
	}

	// 添加设备
	manager.AddDevice(device)

	// 测试标记为已连接
	err := manager.MarkDeviceConnected(device.ID)
	if err != nil {
		t.Fatalf("标记设备连接失败: %v", err)
	}

	// 验证设备已连接
	if !manager.IsDeviceConnected(device.ID) {
		t.Error("设备应该处于已连接状态")
	}

	// 获取已连接设备列表
	connectedDevices := manager.GetConnectedDevices()
	if len(connectedDevices) != 1 {
		t.Errorf("已连接设备数量不正确: 期望 1, 实际 %d", len(connectedDevices))
	}

	// 测试标记为已断开
	err = manager.MarkDeviceDisconnected(device.ID)
	if err != nil {
		t.Fatalf("标记设备断开失败: %v", err)
	}

	// 验证设备已断开
	if manager.IsDeviceConnected(device.ID) {
		t.Error("设备应该处于已断开状态")
	}
}

// TestDevicePairing 测试设备配对
func TestDevicePairing(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)

	device := bluetooth.Device{
		ID:       "test_device_4",
		Name:     "Test Device 4",
		Address:  "00:11:22:33:44:69",
		RSSI:     -30,
		LastSeen: time.Now(),
	}

	// 添加设备
	manager.AddDevice(device)

	// 测试配对设备
	err := manager.PairDevice(device.ID)
	if err != nil {
		t.Fatalf("配对设备失败: %v", err)
	}

	// 验证设备已配对
	if !manager.IsDevicePaired(device.ID) {
		t.Error("设备应该处于已配对状态")
	}

	// 获取已配对设备列表
	pairedDevices := manager.GetPairedDevices()
	if len(pairedDevices) != 1 {
		t.Errorf("已配对设备数量不正确: 期望 1, 实际 %d", len(pairedDevices))
	}

	// 测试取消配对
	err = manager.UnpairDevice(device.ID)
	if err != nil {
		t.Fatalf("取消配对失败: %v", err)
	}

	// 验证设备已取消配对
	if manager.IsDevicePaired(device.ID) {
		t.Error("设备应该处于未配对状态")
	}
}

// TestDiscoverDevices 测试设备发现
func TestDiscoverDevices(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)

	// 添加模拟设备到适配器
	mockDevice1 := bluetooth.Device{
		ID:       "mock_device_1",
		Name:     "Mock Device 1",
		Address:  "00:11:22:33:44:70",
		RSSI:     -50,
		LastSeen: time.Now(),
	}

	mockDevice2 := bluetooth.Device{
		ID:       "mock_device_2",
		Name:     "Mock Device 2",
		Address:  "00:11:22:33:44:71",
		RSSI:     -60,
		LastSeen: time.Now(),
	}

	adapter.AddMockDevice(mockDevice1)
	adapter.AddMockDevice(mockDevice2)

	// 测试设备发现
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	filter := bluetooth.DeviceFilter{
		MinRSSI: -70, // 接受信号强度大于-70的设备
	}

	deviceChan := manager.DiscoverDevices(ctx, filter)

	discoveredDevices := make([]bluetooth.Device, 0)
	for device := range deviceChan {
		discoveredDevices = append(discoveredDevices, device)
	}

	if len(discoveredDevices) < 2 {
		t.Errorf("发现的设备数量不足: 期望至少 2, 实际 %d", len(discoveredDevices))
	}
}

// TestDeviceFiltering 测试设备过滤
func TestDeviceFiltering(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)

	// 添加不同类型的设备
	phoneDevice := bluetooth.Device{
		ID:       "phone_device",
		Name:     "iPhone 12",
		Address:  "00:11:22:33:44:72",
		RSSI:     -40,
		LastSeen: time.Now(),
	}

	headsetDevice := bluetooth.Device{
		ID:       "headset_device",
		Name:     "AirPods Pro",
		Address:  "00:11:22:33:44:73",
		RSSI:     -30,
		LastSeen: time.Now(),
	}

	manager.AddDevice(phoneDevice)
	manager.AddDevice(headsetDevice)

	// 测试按设备类型过滤
	phoneDevices := manager.GetDevicesByType(bluetooth.DeviceTypePhone)
	if len(phoneDevices) != 1 {
		t.Errorf("手机设备数量不正确: 期望 1, 实际 %d", len(phoneDevices))
	}

	headsetDevices := manager.GetDevicesByType(bluetooth.DeviceTypeHeadset)
	if len(headsetDevices) != 1 {
		t.Errorf("耳机设备数量不正确: 期望 1, 实际 %d", len(headsetDevices))
	}
}

// TestSearchDevices 测试设备搜索
func TestSearchDevices(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)

	// 添加测试设备
	devices := []bluetooth.Device{
		{
			ID:       "search_device_1",
			Name:     "Samsung Galaxy",
			Address:  "00:11:22:33:44:74",
			RSSI:     -50,
			LastSeen: time.Now(),
		},
		{
			ID:       "search_device_2",
			Name:     "Apple Watch",
			Address:  "00:11:22:33:44:75",
			RSSI:     -40,
			LastSeen: time.Now(),
		},
		{
			ID:       "search_device_3",
			Name:     "Bluetooth Speaker",
			Address:  "00:11:22:33:44:76",
			RSSI:     -60,
			LastSeen: time.Now(),
		},
	}

	for _, device := range devices {
		manager.AddDevice(device)
	}

	// 测试按名称搜索
	results := manager.SearchDevices("Samsung")
	if len(results) != 1 {
		t.Errorf("搜索结果数量不正确: 期望 1, 实际 %d", len(results))
	}

	results = manager.SearchDevices("Apple")
	if len(results) != 1 {
		t.Errorf("搜索结果数量不正确: 期望 1, 实际 %d", len(results))
	}

	results = manager.SearchDevices("Bluetooth")
	if len(results) != 1 {
		t.Errorf("搜索结果数量不正确: 期望 1, 实际 %d", len(results))
	}
}

// TestGetNearbyDevices 测试获取附近设备
func TestGetNearbyDevices(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)

	// 添加不同信号强度的设备
	devices := []bluetooth.Device{
		{
			ID:       "near_device_1",
			Name:     "Near Device 1",
			Address:  "00:11:22:33:44:77",
			RSSI:     -20, // 很近
			LastSeen: time.Now(),
		},
		{
			ID:       "near_device_2",
			Name:     "Near Device 2",
			Address:  "00:11:22:33:44:78",
			RSSI:     -50, // 中等距离
			LastSeen: time.Now(),
		},
		{
			ID:       "far_device_1",
			Name:     "Far Device 1",
			Address:  "00:11:22:33:44:79",
			RSSI:     -80, // 很远
			LastSeen: time.Now(),
		},
	}

	for _, device := range devices {
		manager.AddDevice(device)
	}

	// 测试获取附近设备（RSSI > -60）
	nearbyDevices := manager.GetNearbyDevices(-60)
	if len(nearbyDevices) != 2 {
		t.Errorf("附近设备数量不正确: 期望 2, 实际 %d", len(nearbyDevices))
	}

	// 验证设备按信号强度排序
	if nearbyDevices[0].RSSI < nearbyDevices[1].RSSI {
		t.Error("设备应该按信号强度降序排列")
	}
}

// TestManagerStats 测试管理器统计信息
func TestManagerStats(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)

	// 添加一些设备
	device1 := bluetooth.Device{
		ID:       "stats_device_1",
		Name:     "Stats Device 1",
		Address:  "00:11:22:33:44:80",
		RSSI:     -40,
		LastSeen: time.Now(),
	}

	device2 := bluetooth.Device{
		ID:       "stats_device_2",
		Name:     "Stats Device 2",
		Address:  "00:11:22:33:44:81",
		RSSI:     -50,
		LastSeen: time.Now(),
	}

	manager.AddDevice(device1)
	manager.AddDevice(device2)

	// 连接一个设备
	manager.MarkDeviceConnected(device1.ID)

	// 配对一个设备
	manager.PairDevice(device2.ID)

	// 获取统计信息
	stats := manager.GetManagerStats()

	if stats.ActiveDevices != 2 {
		t.Errorf("活跃设备数量不正确: 期望 2, 实际 %d", stats.ActiveDevices)
	}

	if stats.ConnectedDevices != 1 {
		t.Errorf("已连接设备数量不正确: 期望 1, 实际 %d", stats.ConnectedDevices)
	}

	if stats.PairedDevices != 1 {
		t.Errorf("已配对设备数量不正确: 期望 1, 实际 %d", stats.PairedDevices)
	}
}

// TestUpdateDeviceRSSI 测试更新设备信号强度
func TestUpdateDeviceRSSI(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)

	device := bluetooth.Device{
		ID:       "rssi_device",
		Name:     "RSSI Device",
		Address:  "00:11:22:33:44:82",
		RSSI:     -50,
		LastSeen: time.Now(),
	}

	manager.AddDevice(device)

	// 更新RSSI
	newRSSI := -30
	err := manager.UpdateDeviceRSSI(device.ID, newRSSI)
	if err != nil {
		t.Fatalf("更新设备RSSI失败: %v", err)
	}

	// 验证RSSI已更新
	updatedDevice, err := manager.GetDevice(device.ID)
	if err != nil {
		t.Fatalf("获取更新后的设备失败: %v", err)
	}

	if updatedDevice.RSSI != newRSSI {
		t.Errorf("设备RSSI未正确更新: 期望 %d, 实际 %d", newRSSI, updatedDevice.RSSI)
	}
}

// TestClearDeviceCache 测试清空设备缓存
func TestClearDeviceCache(t *testing.T) {
	adapter := NewMockAdapter()
	manager := NewManager(adapter)

	// 添加设备
	device1 := bluetooth.Device{
		ID:       "cache_device_1",
		Name:     "Cache Device 1",
		Address:  "00:11:22:33:44:83",
		RSSI:     -40,
		LastSeen: time.Now(),
	}

	device2 := bluetooth.Device{
		ID:       "cache_device_2",
		Name:     "Cache Device 2",
		Address:  "00:11:22:33:44:84",
		RSSI:     -50,
		LastSeen: time.Now(),
	}

	manager.AddDevice(device1)
	manager.AddDevice(device2)

	// 连接一个设备
	manager.MarkDeviceConnected(device1.ID)

	// 清空缓存
	manager.ClearDeviceCache()

	// 验证只有已连接的设备保留
	stats := manager.GetManagerStats()
	if stats.ActiveDevices != 1 {
		t.Errorf("清空缓存后活跃设备数量不正确: 期望 1, 实际 %d", stats.ActiveDevices)
	}

	// 验证未连接的设备被移除
	_, err := manager.GetDevice(device2.ID)
	if err == nil {
		t.Error("未连接的设备应该被移除")
	}

	// 验证已连接的设备保留
	_, err = manager.GetDevice(device1.ID)
	if err != nil {
		t.Error("已连接的设备应该被保留")
	}
}
