package bluetooth

import (
	"context"
	"sync"
	"time"
)

// DefaultDeviceManager 默认设备管理器实现
type DefaultDeviceManager struct {
	adapter          BluetoothAdapter
	devices          map[string]*Device
	connectedDevices map[string]*Device
	callbacks        []DeviceCallback
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
}

// NewDefaultDeviceManager 创建新的默认设备管理器
func NewDefaultDeviceManager(adapter BluetoothAdapter) *DefaultDeviceManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &DefaultDeviceManager{
		adapter:          adapter,
		devices:          make(map[string]*Device),
		connectedDevices: make(map[string]*Device),
		callbacks:        make([]DeviceCallback, 0),
		ctx:              ctx,
		cancel:           cancel,
	}
}

// DiscoverDevices 发现设备
func (dm *DefaultDeviceManager) DiscoverDevices(ctx context.Context, filter DeviceFilter) <-chan Device {
	deviceChan := make(chan Device, 100)

	go func() {
		defer close(deviceChan)

		// 模拟设备发现过程
		// 在实际实现中，这里会调用蓝牙适配器的扫描功能
		mockDevices := []Device{
			{
				ID:           "device_001",
				Name:         "Test Device 1",
				Address:      "00:11:22:33:44:55",
				RSSI:         -45,
				LastSeen:     time.Now(),
				ServiceUUIDs: []string{DefaultServiceUUID},
				Capabilities: DeviceCapabilities{
					SupportsEncryption: true,
					MaxConnections:     1,
					SupportedProtocols: []string{ProtocolRFCOMM},
					BatteryLevel:       85,
				},
			},
			{
				ID:           "device_002",
				Name:         "Test Device 2",
				Address:      "00:11:22:33:44:66",
				RSSI:         -60,
				LastSeen:     time.Now(),
				ServiceUUIDs: []string{DefaultServiceUUID},
				Capabilities: DeviceCapabilities{
					SupportsEncryption: true,
					MaxConnections:     1,
					SupportedProtocols: []string{ProtocolRFCOMM},
					BatteryLevel:       72,
				},
			},
		}

		for _, device := range mockDevices {
			// 应用过滤器
			if dm.matchesFilter(device, filter) {
				dm.mu.Lock()
				dm.devices[device.ID] = &device
				dm.mu.Unlock()

				select {
				case deviceChan <- device:
					// 触发设备发现回调
					dm.triggerCallbacks(device, DeviceEventDiscovered)
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return deviceChan
}

// GetDevice 获取指定设备信息
func (dm *DefaultDeviceManager) GetDevice(deviceID string) (Device, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	device, exists := dm.devices[deviceID]
	if !exists {
		return Device{}, NewBluetoothError(ErrCodeDeviceNotFound, "设备未找到")
	}

	return *device, nil
}

// GetConnectedDevices 获取已连接的设备列表
func (dm *DefaultDeviceManager) GetConnectedDevices() []Device {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	devices := make([]Device, 0, len(dm.connectedDevices))
	for _, device := range dm.connectedDevices {
		devices = append(devices, *device)
	}

	return devices
}

// RegisterDeviceCallback 注册设备回调
func (dm *DefaultDeviceManager) RegisterDeviceCallback(callback DeviceCallback) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.callbacks = append(dm.callbacks, callback)
}

// AddConnectedDevice 添加已连接设备
func (dm *DefaultDeviceManager) AddConnectedDevice(device Device) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.connectedDevices[device.ID] = &device
	dm.triggerCallbacks(device, DeviceEventConnected)
}

// RemoveConnectedDevice 移除已连接设备
func (dm *DefaultDeviceManager) RemoveConnectedDevice(deviceID string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if device, exists := dm.connectedDevices[deviceID]; exists {
		delete(dm.connectedDevices, deviceID)
		dm.triggerCallbacks(*device, DeviceEventDisconnected)
	}
}

// matchesFilter 检查设备是否匹配过滤器
func (dm *DefaultDeviceManager) matchesFilter(device Device, filter DeviceFilter) bool {
	// 检查名称模式
	if filter.NamePattern != "" {
		// 简单的字符串包含检查，实际实现可能需要正则表达式
		if device.Name != filter.NamePattern {
			return false
		}
	}

	// 检查服务UUID
	if len(filter.ServiceUUIDs) > 0 {
		hasMatchingUUID := false
		for _, filterUUID := range filter.ServiceUUIDs {
			for _, deviceUUID := range device.ServiceUUIDs {
				if filterUUID == deviceUUID {
					hasMatchingUUID = true
					break
				}
			}
			if hasMatchingUUID {
				break
			}
		}
		if !hasMatchingUUID {
			return false
		}
	}

	// 检查最小信号强度
	if filter.MinRSSI != 0 && device.RSSI < filter.MinRSSI {
		return false
	}

	// 检查最大发现时间
	if filter.MaxAge > 0 && time.Since(device.LastSeen) > filter.MaxAge {
		return false
	}

	return true
}

// triggerCallbacks 触发设备回调
func (dm *DefaultDeviceManager) triggerCallbacks(device Device, event DeviceEvent) {
	for _, callback := range dm.callbacks {
		go callback(event, device)
	}
}

// Shutdown 关闭设备管理器
func (dm *DefaultDeviceManager) Shutdown() error {
	dm.cancel()
	return nil
}
