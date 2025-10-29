package bluetooth

// ConnectionHandler 连接处理器接口，利用 Go 1.25 泛型特性
type ConnectionHandler[T any] interface {
	// OnConnected 连接建立时的回调
	OnConnected(conn Connection) error
	// OnDisconnected 连接断开时的回调
	OnDisconnected(conn Connection, err error)
	// OnMessage 接收到消息时的回调
	OnMessage(conn Connection, message Message[T]) error
	// OnError 发生错误时的回调
	OnError(conn Connection, err error)
}

// DeviceCallback 设备回调函数类型
type DeviceCallback func(event DeviceEvent, device Device)

// HealthCallback 健康状态回调函数类型
type HealthCallback func(status HealthStatus)

// MessageCallback 消息回调函数类型，支持泛型
type MessageCallback[T any] func(message Message[T]) error

// ErrorCallback 错误回调函数类型
type ErrorCallback func(err *BluetoothError)

// DeviceEvent 设备事件类型
type DeviceEvent int

const (
	DeviceEventDiscovered   DeviceEvent = iota // 设备被发现
	DeviceEventConnected                       // 设备已连接
	DeviceEventDisconnected                    // 设备已断开
	DeviceEventPaired                          // 设备已配对
	DeviceEventUnpaired                        // 设备已取消配对
	DeviceEventUpdated                         // 设备信息已更新
	DeviceEventLost                            // 设备丢失
)

// String 返回设备事件的字符串表示
func (de DeviceEvent) String() string {
	switch de {
	case DeviceEventDiscovered:
		return "discovered"
	case DeviceEventConnected:
		return "connected"
	case DeviceEventDisconnected:
		return "disconnected"
	case DeviceEventPaired:
		return "paired"
	case DeviceEventUnpaired:
		return "unpaired"
	case DeviceEventUpdated:
		return "updated"
	case DeviceEventLost:
		return "lost"
	default:
		return "unknown"
	}
}

// CallbackManager 回调管理器，用于管理各种回调函数
type CallbackManager struct {
	deviceCallbacks []DeviceCallback
	healthCallbacks []HealthCallback
	errorCallbacks  []ErrorCallback
}

// NewCallbackManager 创建新的回调管理器
func NewCallbackManager() *CallbackManager {
	return &CallbackManager{
		deviceCallbacks: make([]DeviceCallback, 0),
		healthCallbacks: make([]HealthCallback, 0),
		errorCallbacks:  make([]ErrorCallback, 0),
	}
}

// RegisterDeviceCallback 注册设备回调
func (cm *CallbackManager) RegisterDeviceCallback(callback DeviceCallback) {
	cm.deviceCallbacks = append(cm.deviceCallbacks, callback)
}

// RegisterHealthCallback 注册健康状态回调
func (cm *CallbackManager) RegisterHealthCallback(callback HealthCallback) {
	cm.healthCallbacks = append(cm.healthCallbacks, callback)
}

// RegisterErrorCallback 注册错误回调
func (cm *CallbackManager) RegisterErrorCallback(callback ErrorCallback) {
	cm.errorCallbacks = append(cm.errorCallbacks, callback)
}

// NotifyDeviceEvent 通知设备事件
func (cm *CallbackManager) NotifyDeviceEvent(event DeviceEvent, device Device) {
	for _, callback := range cm.deviceCallbacks {
		go callback(event, device) // 异步调用回调函数
	}
}

// NotifyHealthStatus 通知健康状态
func (cm *CallbackManager) NotifyHealthStatus(status HealthStatus) {
	for _, callback := range cm.healthCallbacks {
		go callback(status) // 异步调用回调函数
	}
}

// NotifyError 通知错误
func (cm *CallbackManager) NotifyError(err *BluetoothError) {
	for _, callback := range cm.errorCallbacks {
		go callback(err) // 异步调用回调函数
	}
}
