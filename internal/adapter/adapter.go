package adapter

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Anniext/CatTag/pkg/bluetooth"
	tinybt "tinygo.org/x/bluetooth"
)

// BluetoothAdapter 蓝牙适配器接口，封装系统蓝牙API
type BluetoothAdapter interface {
	// Initialize 初始化适配器
	Initialize(ctx context.Context) error
	// Shutdown 关闭适配器
	Shutdown(ctx context.Context) error
	// IsEnabled 检查蓝牙是否启用
	IsEnabled() bool
	// Enable 启用蓝牙
	Enable(ctx context.Context) error
	// Disable 禁用蓝牙
	Disable(ctx context.Context) error
	// Scan 扫描蓝牙设备
	Scan(ctx context.Context, timeout time.Duration) (<-chan bluetooth.Device, error)
	// StopScan 停止扫描
	StopScan() error
	// Connect 连接到设备
	Connect(ctx context.Context, deviceID string) (bluetooth.Connection, error)
	// Disconnect 断开设备连接
	Disconnect(deviceID string) error
	// Listen 监听连接请求
	Listen(serviceUUID string) error
	// StopListen 停止监听
	StopListen() error
	// AcceptConnection 接受连接请求
	AcceptConnection(ctx context.Context) (bluetooth.Connection, error)
	// GetLocalInfo 获取本地适配器信息
	GetLocalInfo() AdapterInfo
	// SetDiscoverable 设置可发现性
	SetDiscoverable(discoverable bool, timeout time.Duration) error
	// GetConnectedDevices 获取已连接的设备
	GetConnectedDevices() []bluetooth.Device
}

// AdapterInfo 适配器信息
type AdapterInfo struct {
	ID           string   `json:"id"`           // 适配器ID
	Name         string   `json:"name"`         // 适配器名称
	Address      string   `json:"address"`      // 适配器地址
	Version      string   `json:"version"`      // 蓝牙版本
	Manufacturer string   `json:"manufacturer"` // 制造商
	Powered      bool     `json:"powered"`      // 是否启用
	Discoverable bool     `json:"discoverable"` // 是否可发现
	Pairable     bool     `json:"pairable"`     // 是否可配对
	Class        uint32   `json:"class"`        // 设备类别
	UUIDs        []string `json:"uuids"`        // 支持的UUID列表
}

// AdapterEvent 适配器事件
type AdapterEvent struct {
	Type      AdapterEventType `json:"type"`       // 事件类型
	AdapterID string           `json:"adapter_id"` // 适配器ID
	Data      any              `json:"data"`       // 事件数据
	Timestamp time.Time        `json:"timestamp"`  // 事件时间戳
}

// AdapterEventType 适配器事件类型
type AdapterEventType int

const (
	AdapterEventPoweredOn         AdapterEventType = iota // 适配器启用
	AdapterEventPoweredOff                                // 适配器禁用
	AdapterEventDiscoverableOn                            // 开始可发现
	AdapterEventDiscoverableOff                           // 停止可发现
	AdapterEventScanStarted                               // 开始扫描
	AdapterEventScanStopped                               // 停止扫描
	AdapterEventDeviceFound                               // 发现设备
	AdapterEventDeviceLost                                // 设备丢失
	AdapterEventConnectionRequest                         // 连接请求
)

// String 返回适配器事件类型的字符串表示
func (aet AdapterEventType) String() string {
	switch aet {
	case AdapterEventPoweredOn:
		return "powered_on"
	case AdapterEventPoweredOff:
		return "powered_off"
	case AdapterEventDiscoverableOn:
		return "discoverable_on"
	case AdapterEventDiscoverableOff:
		return "discoverable_off"
	case AdapterEventScanStarted:
		return "scan_started"
	case AdapterEventScanStopped:
		return "scan_stopped"
	case AdapterEventDeviceFound:
		return "device_found"
	case AdapterEventDeviceLost:
		return "device_lost"
	case AdapterEventConnectionRequest:
		return "connection_request"
	default:
		return "unknown"
	}
}

// AdapterFactory 适配器工厂接口
type AdapterFactory interface {
	// CreateAdapter 创建适配器实例
	CreateAdapter(config AdapterConfig) (BluetoothAdapter, error)
	// GetAvailableAdapters 获取可用的适配器列表
	GetAvailableAdapters() ([]AdapterInfo, error)
	// GetDefaultAdapter 获取默认适配器
	GetDefaultAdapter() (BluetoothAdapter, error)
}

// AdapterConfig 适配器配置
type AdapterConfig struct {
	AdapterID    string        `json:"adapter_id"`    // 适配器ID
	Name         string        `json:"name"`          // 适配器名称
	Timeout      time.Duration `json:"timeout"`       // 操作超时时间
	BufferSize   int           `json:"buffer_size"`   // 缓冲区大小
	EnableEvents bool          `json:"enable_events"` // 启用事件
	LogLevel     string        `json:"log_level"`     // 日志级别
}

// DefaultAdapterConfig 返回默认适配器配置
func DefaultAdapterConfig() AdapterConfig {
	return AdapterConfig{
		AdapterID:    "",
		Name:         "CatTag Adapter",
		Timeout:      30 * time.Second,
		BufferSize:   1024,
		EnableEvents: true,
		LogLevel:     "info",
	}
}

// DefaultAdapter 默认蓝牙适配器实现
type DefaultAdapter struct {
	config      AdapterConfig                   // 适配器配置
	info        AdapterInfo                     // 适配器信息
	enabled     bool                            // 是否启用
	scanning    bool                            // 是否正在扫描
	listening   bool                            // 是否正在监听连接
	connections map[string]bluetooth.Connection // 活动连接
	deviceCache map[string]*CachedDevice        // 设备缓存
	eventChan   chan AdapterEvent               // 适配器事件通道
	scanChan    chan bluetooth.Device           // 扫描设备通道
	connChan    chan bluetooth.Connection       // 连接请求通道
	mu          sync.RWMutex                    // 适配器级别的锁
	ctx         context.Context                 // 适配器上下文
	cancel      context.CancelFunc              // 取消函数
	wg          sync.WaitGroup                  // 等待组
	realAdapter *tinybt.Adapter                 // 真实的蓝牙适配器
}

// CachedDevice 缓存的设备信息
type CachedDevice struct {
	Device      bluetooth.Device `json:"device"`       // 设备信息
	FirstSeen   time.Time        `json:"first_seen"`   // 首次发现时间
	UpdateCount int              `json:"update_count"` // 更新次数
	mu          sync.RWMutex     // 设备级别的锁
}

// NewDefaultAdapter 创建新的默认适配器实例
func NewDefaultAdapter(config AdapterConfig) *DefaultAdapter {
	ctx, cancel := context.WithCancel(context.Background())

	return &DefaultAdapter{
		config:      config,
		enabled:     false,
		scanning:    false,
		listening:   false,
		connections: make(map[string]bluetooth.Connection),
		deviceCache: make(map[string]*CachedDevice),
		eventChan:   make(chan AdapterEvent, 100),
		scanChan:    make(chan bluetooth.Device, 100),
		connChan:    make(chan bluetooth.Connection, 10),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Initialize 初始化适配器
func (da *DefaultAdapter) Initialize(ctx context.Context) error {
	da.mu.Lock()
	defer da.mu.Unlock()

	// 获取默认蓝牙适配器
	da.realAdapter = tinybt.DefaultAdapter

	// 启用蓝牙适配器
	err := da.realAdapter.Enable()
	if err != nil {
		return fmt.Errorf("启用蓝牙适配器失败: %v", err)
	}

	// 获取真实的适配器信息
	da.info = AdapterInfo{
		ID:           "adapter_001",
		Name:         da.config.Name,
		Address:      "00:11:22:33:44:55", // 在实际实现中应该获取真实地址
		Version:      "5.0",
		Manufacturer: "CatTag",
		Powered:      false,
		Discoverable: false,
		Pairable:     true,
		Class:        0x000000,
		UUIDs:        []string{bluetooth.DefaultServiceUUID},
	}

	// 启动事件处理器
	if da.config.EnableEvents {
		da.wg.Add(1)
		go da.eventHandler()
	}

	return nil
}

// Shutdown 关闭适配器
func (da *DefaultAdapter) Shutdown(ctx context.Context) error {
	da.mu.Lock()
	defer da.mu.Unlock()

	// 停止所有操作
	if da.scanning {
		da.stopScanInternal()
	}

	if da.listening {
		da.stopListenInternal()
	}

	// 断开所有连接
	for deviceID := range da.connections {
		da.disconnectInternal(deviceID)
	}

	// 取消上下文并等待goroutine结束
	da.cancel()
	da.wg.Wait()

	da.enabled = false
	da.info.Powered = false

	return nil
}

// IsEnabled 检查蓝牙是否启用
func (da *DefaultAdapter) IsEnabled() bool {
	da.mu.RLock()
	defer da.mu.RUnlock()
	return da.enabled
}

// Enable 启用蓝牙
func (da *DefaultAdapter) Enable(ctx context.Context) error {
	da.mu.Lock()
	defer da.mu.Unlock()

	if da.enabled {
		return nil
	}

	// 模拟启用蓝牙适配器
	da.enabled = true
	da.info.Powered = true

	// 发送启用事件
	da.sendEvent(AdapterEventPoweredOn, nil)

	return nil
}

// Disable 禁用蓝牙
func (da *DefaultAdapter) Disable(ctx context.Context) error {
	da.mu.Lock()
	defer da.mu.Unlock()

	if !da.enabled {
		return nil
	}

	// 停止所有操作
	if da.scanning {
		da.stopScanInternal()
	}

	if da.listening {
		da.stopListenInternal()
	}

	// 断开所有连接
	for deviceID := range da.connections {
		da.disconnectInternal(deviceID)
	}

	da.enabled = false
	da.info.Powered = false

	// 发送禁用事件
	da.sendEvent(AdapterEventPoweredOff, nil)

	return nil
}

// Scan 扫描蓝牙设备
func (da *DefaultAdapter) Scan(ctx context.Context, timeout time.Duration) (<-chan bluetooth.Device, error) {
	da.mu.Lock()
	defer da.mu.Unlock()

	if !da.enabled {
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "蓝牙适配器未启用")
	}

	if da.scanning {
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeResourceBusy, "正在扫描中")
	}

	da.scanning = true
	da.scanChan = make(chan bluetooth.Device, 100)

	// 发送扫描开始事件
	da.sendEvent(AdapterEventScanStarted, nil)

	// 启动扫描goroutine
	da.wg.Add(1)
	go da.scanDevices(ctx, timeout)

	return da.scanChan, nil
}

// StopScan 停止扫描
func (da *DefaultAdapter) StopScan() error {
	da.mu.Lock()
	defer da.mu.Unlock()

	if !da.scanning {
		return nil
	}

	return da.stopScanInternal()
}

// Connect 连接到设备
func (da *DefaultAdapter) Connect(ctx context.Context, deviceID string) (bluetooth.Connection, error) {
	da.mu.Lock()
	defer da.mu.Unlock()

	if !da.enabled {
		return nil, bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeNotSupported, "蓝牙适配器未启用", deviceID, "connect")
	}

	// 检查是否已经连接
	if conn, exists := da.connections[deviceID]; exists {
		if conn.IsActive() {
			return conn, nil
		}
		// 清理无效连接
		delete(da.connections, deviceID)
	}

	// 创建新连接
	conn := NewDefaultConnection(deviceID, da.config.BufferSize)

	// 模拟连接过程
	select {
	case <-ctx.Done():
		return nil, bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeTimeout, "连接超时", deviceID, "connect")
	case <-time.After(100 * time.Millisecond): // 模拟连接延迟
		// 连接成功
	}

	da.connections[deviceID] = conn
	return conn, nil
}

// Disconnect 断开设备连接
func (da *DefaultAdapter) Disconnect(deviceID string) error {
	da.mu.Lock()
	defer da.mu.Unlock()

	return da.disconnectInternal(deviceID)
}

// Listen 监听连接请求
func (da *DefaultAdapter) Listen(serviceUUID string) error {
	da.mu.Lock()
	defer da.mu.Unlock()

	if !da.enabled {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "蓝牙适配器未启用")
	}

	if da.listening {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeResourceBusy, "正在监听中")
	}

	da.listening = true
	da.connChan = make(chan bluetooth.Connection, 10)

	// 启动监听goroutine
	da.wg.Add(1)
	go da.listenForConnections(serviceUUID)

	return nil
}

// StopListen 停止监听
func (da *DefaultAdapter) StopListen() error {
	da.mu.Lock()
	defer da.mu.Unlock()

	if !da.listening {
		return nil
	}

	return da.stopListenInternal()
}

// AcceptConnection 接受连接请求
func (da *DefaultAdapter) AcceptConnection(ctx context.Context) (bluetooth.Connection, error) {
	if !da.listening {
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "未在监听状态")
	}

	select {
	case conn := <-da.connChan:
		return conn, nil
	case <-ctx.Done():
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeTimeout, "接受连接超时")
	}
}

// GetLocalInfo 获取本地适配器信息
func (da *DefaultAdapter) GetLocalInfo() AdapterInfo {
	da.mu.RLock()
	defer da.mu.RUnlock()
	return da.info
}

// SetDiscoverable 设置可发现性
func (da *DefaultAdapter) SetDiscoverable(discoverable bool, timeout time.Duration) error {
	da.mu.Lock()
	defer da.mu.Unlock()

	if !da.enabled {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "蓝牙适配器未启用")
	}

	da.info.Discoverable = discoverable

	if discoverable {
		da.sendEvent(AdapterEventDiscoverableOn, timeout)

		// 如果设置了超时，启动定时器
		if timeout > 0 {
			da.wg.Add(1)
			go func() {
				defer da.wg.Done()
				select {
				case <-time.After(timeout):
					da.mu.Lock()
					da.info.Discoverable = false
					da.mu.Unlock()
					da.sendEvent(AdapterEventDiscoverableOff, nil)
				case <-da.ctx.Done():
					return
				}
			}()
		}
	} else {
		da.sendEvent(AdapterEventDiscoverableOff, nil)
	}

	return nil
}

// GetConnectedDevices 获取已连接的设备
func (da *DefaultAdapter) GetConnectedDevices() []bluetooth.Device {
	da.mu.RLock()
	defer da.mu.RUnlock()

	devices := make([]bluetooth.Device, 0, len(da.connections))
	for deviceID, conn := range da.connections {
		if conn.IsActive() {
			// 创建设备信息（在实际实现中应该从连接中获取）
			device := bluetooth.Device{
				ID:       deviceID,
				Name:     fmt.Sprintf("Device_%s", deviceID),
				Address:  fmt.Sprintf("00:11:22:33:44:%02d", len(devices)+1),
				RSSI:     -50,
				LastSeen: time.Now(),
			}
			devices = append(devices, device)
		}
	}

	return devices
}

// 内部方法

// stopScanInternal 内部停止扫描方法（需要持有锁）
func (da *DefaultAdapter) stopScanInternal() error {
	if !da.scanning {
		return nil
	}

	da.scanning = false
	close(da.scanChan)
	da.sendEvent(AdapterEventScanStopped, nil)
	return nil
}

// stopListenInternal 内部停止监听方法（需要持有锁）
func (da *DefaultAdapter) stopListenInternal() error {
	if !da.listening {
		return nil
	}

	da.listening = false
	close(da.connChan)
	return nil
}

// disconnectInternal 内部断开连接方法（需要持有锁）
func (da *DefaultAdapter) disconnectInternal(deviceID string) error {
	conn, exists := da.connections[deviceID]
	if !exists {
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeNotFound, "连接不存在", deviceID, "disconnect")
	}

	err := conn.Close()
	delete(da.connections, deviceID)
	return err
}

// sendEvent 发送适配器事件
func (da *DefaultAdapter) sendEvent(eventType AdapterEventType, data any) {
	if !da.config.EnableEvents {
		return
	}

	event := AdapterEvent{
		Type:      eventType,
		AdapterID: da.info.ID,
		Data:      data,
		Timestamp: time.Now(),
	}

	select {
	case da.eventChan <- event:
	default:
		// 事件通道满了，丢弃事件
	}
}

// scanDevices 扫描设备的goroutine
func (da *DefaultAdapter) scanDevices(ctx context.Context, timeout time.Duration) {
	defer da.wg.Done()
	defer func() {
		da.mu.Lock()
		if da.scanning {
			da.scanning = false
			close(da.scanChan)
			da.sendEvent(AdapterEventScanStopped, nil)
		}
		da.mu.Unlock()
	}()

	// 创建超时上下文
	scanCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 使用真实的蓝牙扫描
	if da.realAdapter == nil {
		log.Fatalln("蓝牙适配器扫描错误")
		return
	}

	err := da.realAdapter.Scan(func(adapter *tinybt.Adapter, result tinybt.ScanResult) {
		// 检查上下文是否已取消
		select {
		case <-scanCtx.Done():
			return
		case <-da.ctx.Done():
			return
		default:
		}

		// 转换扫描结果为我们的设备格式
		device := da.convertScanResult(result)

		// 更新设备缓存并获取最佳设备信息
		finalDevice := da.updateDeviceCache(device)

		// 发送设备到通道
		select {
		case da.scanChan <- finalDevice:
			da.sendEvent(AdapterEventDeviceFound, finalDevice)
		default:
			// 扫描通道满了，跳过这个设备
		}
	})

	if err != nil {
		log.Fatalln("蓝牙适配器扫描失败:", err)
		return
	}

	// 等待扫描完成或超时
	<-scanCtx.Done()

	// 停止扫描
	da.realAdapter.StopScan()
}

// convertScanResult 将 tinygo 蓝牙扫描结果转换为我们的设备格式
func (da *DefaultAdapter) convertScanResult(result tinybt.ScanResult) bluetooth.Device {
	// 获取服务 UUID
	var serviceUUIDs []string
	for _, uuid := range result.AdvertisementPayload.ServiceUUIDs() {
		serviceUUIDs = append(serviceUUIDs, uuid.String())
	}

	// 如果没有服务 UUID，添加默认的
	if len(serviceUUIDs) == 0 {
		serviceUUIDs = []string{bluetooth.DefaultServiceUUID}
	}

	return bluetooth.Device{
		ID:           result.Address.String(),
		Name:         result.LocalName(),
		Address:      result.Address.String(),
		RSSI:         int(result.RSSI),
		LastSeen:     time.Now(),
		ServiceUUIDs: serviceUUIDs,
		Capabilities: bluetooth.DeviceCapabilities{
			SupportsEncryption: true, // 假设支持加密
			MaxConnections:     1,
			SupportedProtocols: []string{bluetooth.ProtocolRFCOMM},
			BatteryLevel:       0, // 无法从扫描结果获取电池信息
		},
	}
}

// updateDeviceCache 更新设备缓存并返回最佳设备信息
func (da *DefaultAdapter) updateDeviceCache(newDevice bluetooth.Device) bluetooth.Device {
	da.mu.Lock()
	defer da.mu.Unlock()

	deviceID := newDevice.Address
	now := time.Now()

	// 检查设备是否已在缓存中
	if cached, exists := da.deviceCache[deviceID]; exists {
		cached.mu.Lock()
		defer cached.mu.Unlock()

		// 更新设备信息
		cached.Device.LastSeen = now
		cached.Device.RSSI = newDevice.RSSI
		cached.UpdateCount++

		// 如果新设备有更好的名称，更新名称
		if da.isBetterName(newDevice.Name, cached.Device.Name) {
			cached.Device.Name = newDevice.Name
		}

		// 合并服务UUID
		cached.Device.ServiceUUIDs = da.mergeServiceUUIDs(cached.Device.ServiceUUIDs, newDevice.ServiceUUIDs)

		return cached.Device
	}

	// 新设备，添加到缓存
	da.deviceCache[deviceID] = &CachedDevice{
		Device:      newDevice,
		FirstSeen:   now,
		UpdateCount: 1,
	}

	return newDevice
}

// isBetterName 判断新名称是否比旧名称更好
func (da *DefaultAdapter) isBetterName(newName, oldName string) bool {
	// 如果旧名称为空或是默认名称，新名称更好
	if oldName == "" || oldName == "Unknown Device" {
		return newName != "" && newName != "Unknown Device"
	}

	// 如果新名称更长且不是生成的名称，可能更好
	if len(newName) > len(oldName) && !da.isGeneratedName(newName) && da.isGeneratedName(oldName) {
		return true
	}

	// 如果新名称不是生成的，而旧名称是生成的，新名称更好
	if !da.isGeneratedName(newName) && da.isGeneratedName(oldName) {
		return true
	}

	return false
}

// isGeneratedName 判断是否是生成的名称
func (da *DefaultAdapter) isGeneratedName(name string) bool {
	if name == "Unknown Device" {
		return true
	}

	// 检查是否是制造商生成的名称格式
	if len(name) > 7 && name[len(name)-7:] == " Device" {
		return true
	}

	// 检查是否是地址生成的名称格式
	if len(name) > 7 && name[:7] == "Device_" {
		return true
	}

	return false
}

// mergeServiceUUIDs 合并服务UUID列表
func (da *DefaultAdapter) mergeServiceUUIDs(existing, new []string) []string {
	uuidSet := make(map[string]bool)

	// 添加现有的UUID
	for _, uuid := range existing {
		uuidSet[uuid] = true
	}

	// 添加新的UUID
	for _, uuid := range new {
		uuidSet[uuid] = true
	}

	// 转换回切片
	result := make([]string, 0, len(uuidSet))
	for uuid := range uuidSet {
		result = append(result, uuid)
	}

	return result
}

// cleanupDeviceCache 清理过期的设备缓存
func (da *DefaultAdapter) cleanupDeviceCache(maxAge time.Duration) {
	da.mu.Lock()
	defer da.mu.Unlock()

	now := time.Now()
	for deviceID, cached := range da.deviceCache {
		cached.mu.RLock()
		lastSeen := cached.Device.LastSeen
		cached.mu.RUnlock()

		if now.Sub(lastSeen) > maxAge {
			delete(da.deviceCache, deviceID)
		}
	}
}

// GetCachedDevices 获取缓存的设备列表
func (da *DefaultAdapter) GetCachedDevices() []bluetooth.Device {
	da.mu.RLock()
	defer da.mu.RUnlock()

	devices := make([]bluetooth.Device, 0, len(da.deviceCache))
	for _, cached := range da.deviceCache {
		cached.mu.RLock()
		devices = append(devices, cached.Device)
		cached.mu.RUnlock()
	}

	return devices
}

// listenForConnections 监听连接请求的goroutine
func (da *DefaultAdapter) listenForConnections(serviceUUID string) {
	defer da.wg.Done()

	// 模拟接收连接请求
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	connectionCount := 0
	for {
		select {
		case <-da.ctx.Done():
			return
		case <-ticker.C:
			// 模拟接收到连接请求
			if connectionCount < 3 { // 最多接受3个连接
				deviceID := fmt.Sprintf("incoming_device_%d", connectionCount+1)
				conn := NewDefaultConnection(deviceID, da.config.BufferSize)

				da.mu.Lock()
				da.connections[deviceID] = conn
				da.mu.Unlock()

				da.sendEvent(AdapterEventConnectionRequest, deviceID)

				select {
				case da.connChan <- conn:
					connectionCount++
				default:
					// 连接通道满了
				}
			}
		}
	}
}

// eventHandler 事件处理器goroutine
func (da *DefaultAdapter) eventHandler() {
	defer da.wg.Done()

	for {
		select {
		case <-da.ctx.Done():
			return
		case event := <-da.eventChan:
			// 在实际实现中，这里可以处理事件，比如记录日志、通知上层等
			_ = event // 避免未使用变量警告
		}
	}
}

// DefaultAdapterFactory 默认适配器工厂
type DefaultAdapterFactory struct{}

// NewDefaultAdapterFactory 创建默认适配器工厂
func NewDefaultAdapterFactory() *DefaultAdapterFactory {
	return &DefaultAdapterFactory{}
}

// CreateAdapter 创建适配器实例
func (daf *DefaultAdapterFactory) CreateAdapter(config AdapterConfig) (BluetoothAdapter, error) {
	return NewDefaultAdapter(config), nil
}

// GetAvailableAdapters 获取可用的适配器列表
func (daf *DefaultAdapterFactory) GetAvailableAdapters() ([]AdapterInfo, error) {
	// 模拟返回可用适配器列表
	adapters := []AdapterInfo{
		{
			ID:           "adapter_001",
			Name:         "Default Bluetooth Adapter",
			Address:      "00:11:22:33:44:55",
			Version:      "5.0",
			Manufacturer: "CatTag",
			Powered:      false,
			Discoverable: false,
			Pairable:     true,
			Class:        0x000000,
			UUIDs:        []string{bluetooth.DefaultServiceUUID},
		},
	}
	return adapters, nil
}

// GetDefaultAdapter 获取默认适配器
func (daf *DefaultAdapterFactory) GetDefaultAdapter() (BluetoothAdapter, error) {
	config := DefaultAdapterConfig()
	return daf.CreateAdapter(config)
}
