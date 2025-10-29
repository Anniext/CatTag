package device

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/Anniext/CatTag/internal/adapter"
	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// ManagerStats 设备管理器统计信息
type ManagerStats struct {
	TotalDevicesFound    int           `json:"total_devices_found"`    // 总发现设备数
	ActiveDevices        int           `json:"active_devices"`         // 活跃设备数
	ConnectedDevices     int           `json:"connected_devices"`      // 已连接设备数
	PairedDevices        int           `json:"paired_devices"`         // 已配对设备数
	ScanCount            int           `json:"scan_count"`             // 扫描次数
	LastScanTime         time.Time     `json:"last_scan_time"`         // 最后扫描时间
	AverageDiscoveryTime time.Duration `json:"average_discovery_time"` // 平均发现时间
	CacheHitRate         float64       `json:"cache_hit_rate"`         // 缓存命中率
}

// ManagerConfig 设备管理器配置
type ManagerConfig struct {
	ScanTimeout      time.Duration `json:"scan_timeout"`       // 扫描超时时间
	CacheTimeout     time.Duration `json:"cache_timeout"`      // 缓存超时时间
	MaxDevices       int           `json:"max_devices"`        // 最大设备缓存数量
	CleanupInterval  time.Duration `json:"cleanup_interval"`   // 清理间隔
	EnableAutoScan   bool          `json:"enable_auto_scan"`   // 启用自动扫描
	AutoScanInterval time.Duration `json:"auto_scan_interval"` // 自动扫描间隔
}

// DefaultManagerConfig 返回默认设备管理器配置
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		ScanTimeout:      bluetooth.DefaultScanTimeout,
		CacheTimeout:     5 * time.Minute,
		MaxDevices:       100,
		CleanupInterval:  1 * time.Minute,
		EnableAutoScan:   false,
		AutoScanInterval: 30 * time.Second,
	}
}

// Manager 设备管理器实现
type Manager struct {
	mu               sync.RWMutex                 // 读写锁
	devices          map[string]*bluetooth.Device // 设备缓存
	connectedDevices map[string]*bluetooth.Device // 已连接设备
	pairedDevices    map[string]*bluetooth.Device // 已配对设备
	callbacks        []bluetooth.DeviceCallback   // 设备回调列表
	adapter          adapter.BluetoothAdapter     // 蓝牙适配器
	discoveryService *DiscoveryService            // 发现服务
	pairingService   *PairingService              // 配对服务
	scanTimeout      time.Duration                // 扫描超时时间
	cacheTimeout     time.Duration                // 缓存超时时间
	maxDevices       int                          // 最大设备缓存数量
	running          bool                         // 运行状态
	scanning         bool                         // 扫描状态
	ctx              context.Context              // 上下文
	cancel           context.CancelFunc           // 取消函数
	stats            ManagerStats                 // 管理器统计信息
}

// NewManager 创建新的设备管理器
func NewManager(adapter adapter.BluetoothAdapter) *Manager {
	return NewManagerWithConfig(adapter, DefaultManagerConfig())
}

// NewManagerWithConfig 使用配置创建新的设备管理器
func NewManagerWithConfig(adapter adapter.BluetoothAdapter, config ManagerConfig) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	manager := &Manager{
		devices:          make(map[string]*bluetooth.Device),
		connectedDevices: make(map[string]*bluetooth.Device),
		pairedDevices:    make(map[string]*bluetooth.Device),
		callbacks:        make([]bluetooth.DeviceCallback, 0),
		adapter:          adapter,
		scanTimeout:      config.ScanTimeout,
		cacheTimeout:     config.CacheTimeout,
		maxDevices:       config.MaxDevices,
		running:          false,
		scanning:         false,
		ctx:              ctx,
		cancel:           cancel,
		stats:            ManagerStats{},
	}

	// 创建发现和配对服务
	manager.discoveryService = NewDiscoveryService(adapter, manager)
	manager.pairingService = NewPairingService(adapter, manager)

	return manager
}

// DiscoverDevices 发现设备
func (m *Manager) DiscoverDevices(ctx context.Context, filter bluetooth.DeviceFilter) <-chan bluetooth.Device {
	deviceChan := make(chan bluetooth.Device, 100)

	go func() {
		defer close(deviceChan)

		// 更新统计信息
		m.mu.Lock()
		m.stats.ScanCount++
		m.stats.LastScanTime = time.Now()
		m.scanning = true
		m.mu.Unlock()

		defer func() {
			m.mu.Lock()
			m.scanning = false
			m.mu.Unlock()
		}()

		// 首先返回缓存中符合条件的设备
		m.mu.RLock()
		for _, device := range m.devices {
			if m.matchesFilter(*device, filter) {
				select {
				case deviceChan <- *device:
				case <-ctx.Done():
					m.mu.RUnlock()
					return
				}
			}
		}
		m.mu.RUnlock()

		// 如果适配器可用，进行实际扫描
		if m.adapter != nil && m.adapter.IsEnabled() {
			scanChan, err := m.adapter.Scan(ctx, m.scanTimeout)
			if err != nil {
				// 记录错误但不中断，继续使用缓存数据
				return
			}

			// 处理扫描结果
			for {
				select {
				case device, ok := <-scanChan:
					if !ok {
						return // 扫描完成
					}

					// 更新设备缓存
					m.AddDevice(device)

					// 检查设备是否符合过滤条件
					if m.matchesFilter(device, filter) {
						select {
						case deviceChan <- device:
						case <-ctx.Done():
							return
						}
					}

				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return deviceChan
}

// GetDevice 获取指定设备信息
func (m *Manager) GetDevice(deviceID string) (bluetooth.Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.devices[deviceID]
	if !exists {
		return bluetooth.Device{}, bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeDeviceNotFound,
			"设备未找到",
			deviceID,
			"get_device",
		)
	}

	// 检查设备是否过期
	if time.Since(device.LastSeen) > m.cacheTimeout {
		return bluetooth.Device{}, bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeDeviceOffline,
			"设备缓存已过期",
			deviceID,
			"get_device",
		)
	}

	return *device, nil
}

// GetConnectedDevices 获取已连接的设备列表
func (m *Manager) GetConnectedDevices() []bluetooth.Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]bluetooth.Device, 0, len(m.connectedDevices))
	for _, device := range m.connectedDevices {
		devices = append(devices, *device)
	}

	return devices
}

// RegisterDeviceCallback 注册设备回调
func (m *Manager) RegisterDeviceCallback(callback bluetooth.DeviceCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callbacks = append(m.callbacks, callback)
}

// AddDevice 添加设备到缓存
func (m *Manager) AddDevice(device bluetooth.Device) {
	m.mu.Lock()
	defer m.mu.Unlock()

	device.LastSeen = time.Now()

	// 检查设备是否已存在
	if existingDevice, exists := m.devices[device.ID]; exists {
		// 更新设备信息
		*existingDevice = device
		m.notifyDeviceEvent(bluetooth.DeviceEventUpdated, device)
	} else {
		// 检查是否超过最大设备数量限制
		if len(m.devices) >= m.maxDevices {
			// 移除最旧的未连接且未配对的设备
			m.removeOldestDevice()
		}

		// 添加新设备
		m.devices[device.ID] = &device
		m.stats.TotalDevicesFound++
		m.notifyDeviceEvent(bluetooth.DeviceEventDiscovered, device)
	}
}

// removeOldestDevice 移除最旧的设备（内部方法，需要持有锁）
func (m *Manager) removeOldestDevice() {
	var oldestID string
	var oldestTime time.Time = time.Now()

	// 查找最旧的未连接且未配对的设备
	for deviceID, device := range m.devices {
		// 跳过已连接或已配对的设备
		if _, connected := m.connectedDevices[deviceID]; connected {
			continue
		}
		if _, paired := m.pairedDevices[deviceID]; paired {
			continue
		}

		if device.LastSeen.Before(oldestTime) {
			oldestTime = device.LastSeen
			oldestID = deviceID
		}
	}

	// 移除找到的最旧设备
	if oldestID != "" {
		if device, exists := m.devices[oldestID]; exists {
			delete(m.devices, oldestID)
			m.notifyDeviceEvent(bluetooth.DeviceEventLost, *device)
		}
	}
}

// RemoveDevice 从缓存中移除设备
func (m *Manager) RemoveDevice(deviceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device, exists := m.devices[deviceID]; exists {
		delete(m.devices, deviceID)
		delete(m.connectedDevices, deviceID)
		m.notifyDeviceEvent(bluetooth.DeviceEventLost, *device)
	}
}

// MarkDeviceConnected 标记设备为已连接
func (m *Manager) MarkDeviceConnected(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[deviceID]
	if !exists {
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeDeviceNotFound,
			"设备未找到",
			deviceID,
			"mark_connected",
		)
	}

	m.connectedDevices[deviceID] = device
	m.notifyDeviceEvent(bluetooth.DeviceEventConnected, *device)
	return nil
}

// MarkDeviceDisconnected 标记设备为已断开
func (m *Manager) MarkDeviceDisconnected(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.connectedDevices[deviceID]
	if !exists {
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeDeviceNotFound,
			"已连接设备中未找到指定设备",
			deviceID,
			"mark_disconnected",
		)
	}

	delete(m.connectedDevices, deviceID)
	m.notifyDeviceEvent(bluetooth.DeviceEventDisconnected, *device)
	return nil
}

// IsDeviceConnected 检查设备是否已连接
func (m *Manager) IsDeviceConnected(deviceID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.connectedDevices[deviceID]
	return exists
}

// Start 启动设备管理器
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeAlreadyExists, "设备管理器已在运行")
	}

	m.running = true

	// 启动清理过期设备的goroutine
	go m.cleanupExpiredDevices()

	// 启动发现和配对服务
	if m.discoveryService != nil {
		if err := m.discoveryService.Start(); err != nil {
			// 记录错误但不中断启动
		}
	}

	if m.pairingService != nil {
		if err := m.pairingService.Start(); err != nil {
			// 记录错误但不中断启动
		}
	}

	return nil
}

// Stop 停止设备管理器
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeNotFound, "设备管理器未运行")
	}

	// 停止发现和配对服务
	if m.discoveryService != nil {
		m.discoveryService.Stop()
	}

	if m.pairingService != nil {
		m.pairingService.Stop()
	}

	m.running = false
	m.cancel()

	// 清空设备缓存
	m.devices = make(map[string]*bluetooth.Device)
	m.connectedDevices = make(map[string]*bluetooth.Device)

	return nil
}

// matchesFilter 检查设备是否匹配过滤条件
func (m *Manager) matchesFilter(device bluetooth.Device, filter bluetooth.DeviceFilter) bool {
	// 检查名称模式（支持正则表达式）
	if filter.NamePattern != "" {
		matched, err := regexp.MatchString(filter.NamePattern, device.Name)
		if err != nil || !matched {
			return false
		}
	}

	// 检查服务UUID
	if len(filter.ServiceUUIDs) > 0 {
		found := false
		for _, filterUUID := range filter.ServiceUUIDs {
			for _, deviceUUID := range device.ServiceUUIDs {
				if deviceUUID == filterUUID {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查最小信号强度
	if filter.MinRSSI != 0 && device.RSSI < filter.MinRSSI {
		return false
	}

	// 检查设备年龄
	if filter.MaxAge > 0 && time.Since(device.LastSeen) > filter.MaxAge {
		return false
	}

	// 检查设备类型
	if len(filter.DeviceTypes) > 0 {
		deviceType := m.getDeviceType(device)
		found := false
		for _, filterType := range filter.DeviceTypes {
			if deviceType == filterType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查是否需要认证
	if filter.RequireAuth {
		if !m.IsDevicePaired(device.ID) {
			return false
		}
	}

	return true
}

// notifyDeviceEvent 通知设备事件
func (m *Manager) notifyDeviceEvent(event bluetooth.DeviceEvent, device bluetooth.Device) {
	for _, callback := range m.callbacks {
		go callback(event, device) // 异步调用回调
	}
}

// cleanupExpiredDevices 清理过期设备
func (m *Manager) cleanupExpiredDevices() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.mu.Lock()
			now := time.Now()
			expiredDevices := make([]string, 0)

			for deviceID, device := range m.devices {
				// 跳过已连接的设备
				if _, connected := m.connectedDevices[deviceID]; connected {
					continue
				}

				// 跳过已配对的设备（使用更长的超时时间）
				if _, paired := m.pairedDevices[deviceID]; paired {
					if now.Sub(device.LastSeen) > m.cacheTimeout*3 {
						expiredDevices = append(expiredDevices, deviceID)
					}
					continue
				}

				// 普通设备使用标准超时时间
				if now.Sub(device.LastSeen) > m.cacheTimeout {
					expiredDevices = append(expiredDevices, deviceID)
				}
			}

			// 移除过期设备
			for _, deviceID := range expiredDevices {
				if device, exists := m.devices[deviceID]; exists {
					delete(m.devices, deviceID)

					// 如果是已连接设备，也从连接列表中移除
					if _, connected := m.connectedDevices[deviceID]; connected {
						delete(m.connectedDevices, deviceID)
						m.notifyDeviceEvent(bluetooth.DeviceEventDisconnected, *device)
					}

					m.notifyDeviceEvent(bluetooth.DeviceEventLost, *device)
				}
			}
			m.mu.Unlock()

		case <-m.ctx.Done():
			return
		}
	}
}

// GetDeviceCount 获取设备数量
func (m *Manager) GetDeviceCount() (total int, connected int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.devices), len(m.connectedDevices)
}

// GetDevicesByType 根据类型获取设备
func (m *Manager) GetDevicesByType(deviceType string) []bluetooth.Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]bluetooth.Device, 0)
	for _, device := range m.devices {
		if m.getDeviceType(*device) == deviceType {
			devices = append(devices, *device)
		}
	}

	return devices
}

// getDeviceType 根据设备信息推断设备类型
func (m *Manager) getDeviceType(device bluetooth.Device) string {
	// 根据设备名称模式推断类型
	name := device.Name

	// 手机设备模式
	phonePatterns := []string{
		"iPhone", "iPad", "Android", "Samsung", "Huawei", "Xiaomi", "OnePlus",
	}
	for _, pattern := range phonePatterns {
		if matched, _ := regexp.MatchString("(?i)"+pattern, name); matched {
			return bluetooth.DeviceTypePhone
		}
	}

	// 音频设备模式
	audioPatterns := []string{
		"Headset", "Headphone", "Earbuds", "Speaker", "AirPods", "Beats",
	}
	for _, pattern := range audioPatterns {
		if matched, _ := regexp.MatchString("(?i)"+pattern, name); matched {
			return bluetooth.DeviceTypeHeadset
		}
	}

	// 输入设备模式
	if matched, _ := regexp.MatchString("(?i)(keyboard|kbd)", name); matched {
		return bluetooth.DeviceTypeKeyboard
	}
	if matched, _ := regexp.MatchString("(?i)mouse", name); matched {
		return bluetooth.DeviceTypeMouse
	}

	// 计算机设备模式
	computerPatterns := []string{
		"MacBook", "ThinkPad", "Laptop", "Desktop", "PC",
	}
	for _, pattern := range computerPatterns {
		if matched, _ := regexp.MatchString("(?i)"+pattern, name); matched {
			return bluetooth.DeviceTypeLaptop
		}
	}

	return bluetooth.DeviceTypeUnknown
}

// PairDevice 配对设备
func (m *Manager) PairDevice(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[deviceID]
	if !exists {
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeDeviceNotFound,
			"设备未找到",
			deviceID,
			"pair_device",
		)
	}

	// 检查设备是否已配对
	if _, paired := m.pairedDevices[deviceID]; paired {
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeAlreadyExists,
			"设备已配对",
			deviceID,
			"pair_device",
		)
	}

	// 模拟配对过程
	// 在实际实现中，这里应该调用系统蓝牙API进行配对
	m.pairedDevices[deviceID] = device
	m.stats.PairedDevices = len(m.pairedDevices)

	// 通知配对事件
	m.notifyDeviceEvent(bluetooth.DeviceEventPaired, *device)

	return nil
}

// UnpairDevice 取消配对设备
func (m *Manager) UnpairDevice(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.pairedDevices[deviceID]
	if !exists {
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeDeviceNotFound,
			"已配对设备中未找到指定设备",
			deviceID,
			"unpair_device",
		)
	}

	// 如果设备已连接，先断开连接
	if _, connected := m.connectedDevices[deviceID]; connected {
		delete(m.connectedDevices, deviceID)
		m.notifyDeviceEvent(bluetooth.DeviceEventDisconnected, *device)
	}

	// 移除配对信息
	delete(m.pairedDevices, deviceID)
	m.stats.PairedDevices = len(m.pairedDevices)

	// 通知取消配对事件
	m.notifyDeviceEvent(bluetooth.DeviceEventUnpaired, *device)

	return nil
}

// IsDevicePaired 检查设备是否已配对
func (m *Manager) IsDevicePaired(deviceID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.pairedDevices[deviceID]
	return exists
}

// GetPairedDevices 获取已配对的设备列表
func (m *Manager) GetPairedDevices() []bluetooth.Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]bluetooth.Device, 0, len(m.pairedDevices))
	for _, device := range m.pairedDevices {
		devices = append(devices, *device)
	}

	return devices
}

// SearchDevices 搜索设备（支持模糊匹配）
func (m *Manager) SearchDevices(query string) []bluetooth.Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]bluetooth.Device, 0)
	queryLower := fmt.Sprintf("(?i)%s", regexp.QuoteMeta(query))

	for _, device := range m.devices {
		// 在设备名称、地址中搜索
		if matched, _ := regexp.MatchString(queryLower, device.Name); matched {
			devices = append(devices, *device)
			continue
		}
		if matched, _ := regexp.MatchString(queryLower, device.Address); matched {
			devices = append(devices, *device)
			continue
		}
		// 在服务UUID中搜索
		for _, uuid := range device.ServiceUUIDs {
			if matched, _ := regexp.MatchString(queryLower, uuid); matched {
				devices = append(devices, *device)
				break
			}
		}
	}

	return devices
}

// GetNearbyDevices 获取附近的设备（按信号强度排序）
func (m *Manager) GetNearbyDevices(maxDistance int) []bluetooth.Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]bluetooth.Device, 0)
	for _, device := range m.devices {
		// RSSI值越大表示信号越强，距离越近
		if device.RSSI >= maxDistance {
			devices = append(devices, *device)
		}
	}

	// 按RSSI降序排序（信号强度从强到弱）
	for i := 0; i < len(devices)-1; i++ {
		for j := i + 1; j < len(devices); j++ {
			if devices[i].RSSI < devices[j].RSSI {
				devices[i], devices[j] = devices[j], devices[i]
			}
		}
	}

	return devices
}

// GetManagerStats 获取管理器统计信息
func (m *Manager) GetManagerStats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := m.stats
	stats.ActiveDevices = len(m.devices)
	stats.ConnectedDevices = len(m.connectedDevices)
	stats.PairedDevices = len(m.pairedDevices)

	return stats
}

// IsScanning 检查是否正在扫描
func (m *Manager) IsScanning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.scanning
}

// ClearDeviceCache 清空设备缓存
func (m *Manager) ClearDeviceCache() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 保留已连接和已配对的设备
	newDevices := make(map[string]*bluetooth.Device)

	for deviceID, device := range m.devices {
		if _, connected := m.connectedDevices[deviceID]; connected {
			newDevices[deviceID] = device
		} else if _, paired := m.pairedDevices[deviceID]; paired {
			newDevices[deviceID] = device
		}
	}

	m.devices = newDevices
}

// UpdateDeviceRSSI 更新设备信号强度
func (m *Manager) UpdateDeviceRSSI(deviceID string, rssi int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[deviceID]
	if !exists {
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeDeviceNotFound,
			"设备未找到",
			deviceID,
			"update_rssi",
		)
	}

	device.RSSI = rssi
	device.LastSeen = time.Now()

	// 通知设备更新事件
	m.notifyDeviceEvent(bluetooth.DeviceEventUpdated, *device)

	return nil
}

// StartDiscoveryService 启动设备发现服务
func (m *Manager) StartDiscoveryService() error {
	if m.discoveryService == nil {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "发现服务未初始化")
	}
	return m.discoveryService.Start()
}

// StopDiscoveryService 停止设备发现服务
func (m *Manager) StopDiscoveryService() error {
	if m.discoveryService == nil {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "发现服务未初始化")
	}
	return m.discoveryService.Stop()
}

// StartPairingService 启动设备配对服务
func (m *Manager) StartPairingService() error {
	if m.pairingService == nil {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "配对服务未初始化")
	}
	return m.pairingService.Start()
}

// StopPairingService 停止设备配对服务
func (m *Manager) StopPairingService() error {
	if m.pairingService == nil {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "配对服务未初始化")
	}
	return m.pairingService.Stop()
}

// StartAdvancedScan 启动高级扫描（使用发现服务）
func (m *Manager) StartAdvancedScan(ctx context.Context, filter bluetooth.DeviceFilter, timeout time.Duration) (*ScanSession, error) {
	if m.discoveryService == nil {
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "发现服务未初始化")
	}
	return m.discoveryService.StartScan(ctx, filter, timeout)
}

// StopAdvancedScan 停止高级扫描
func (m *Manager) StopAdvancedScan(sessionID string) error {
	if m.discoveryService == nil {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "发现服务未初始化")
	}
	return m.discoveryService.StopScan(sessionID)
}

// GetActiveScanSessions 获取活跃的扫描会话
func (m *Manager) GetActiveScanSessions() []*ScanSession {
	if m.discoveryService == nil {
		return []*ScanSession{}
	}
	return m.discoveryService.GetActiveScanSessions()
}

// StartDevicePairing 启动设备配对
func (m *Manager) StartDevicePairing(ctx context.Context, deviceID string, method PairingMethod) (*PairingSession, error) {
	if m.pairingService == nil {
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "配对服务未初始化")
	}
	return m.pairingService.StartPairing(ctx, deviceID, method)
}

// CancelDevicePairing 取消设备配对
func (m *Manager) CancelDevicePairing(sessionID string) error {
	if m.pairingService == nil {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "配对服务未初始化")
	}
	return m.pairingService.CancelPairing(sessionID)
}

// ConfirmDevicePairing 确认设备配对
func (m *Manager) ConfirmDevicePairing(sessionID string, confirm bool) error {
	if m.pairingService == nil {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "配对服务未初始化")
	}
	return m.pairingService.ConfirmPairing(sessionID, confirm)
}

// ProvidePairingPIN 提供配对PIN码
func (m *Manager) ProvidePairingPIN(sessionID string, pin string) error {
	if m.pairingService == nil {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "配对服务未初始化")
	}
	return m.pairingService.ProvidePIN(sessionID, pin)
}

// GetActivePairingSessions 获取活跃的配对会话
func (m *Manager) GetActivePairingSessions() []*PairingSession {
	if m.pairingService == nil {
		return []*PairingSession{}
	}
	return m.pairingService.GetActivePairingSessions()
}

// GetDiscoveryService 获取发现服务实例
func (m *Manager) GetDiscoveryService() *DiscoveryService {
	return m.discoveryService
}

// GetPairingService 获取配对服务实例
func (m *Manager) GetPairingService() *PairingService {
	return m.pairingService
}
