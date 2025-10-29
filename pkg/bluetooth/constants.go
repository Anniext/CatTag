package bluetooth

import "time"

// MessageType 消息类型枚举
type MessageType int

const (
	MessageTypeData      MessageType = iota // 数据消息
	MessageTypeControl                      // 控制消息
	MessageTypeHeartbeat                    // 心跳消息
	MessageTypeAuth                         // 认证消息
	MessageTypeError                        // 错误消息
	MessageTypeAck                          // 确认消息
)

// String 返回消息类型的字符串表示
func (mt MessageType) String() string {
	switch mt {
	case MessageTypeData:
		return "data"
	case MessageTypeControl:
		return "control"
	case MessageTypeHeartbeat:
		return "heartbeat"
	case MessageTypeAuth:
		return "auth"
	case MessageTypeError:
		return "error"
	case MessageTypeAck:
		return "ack"
	default:
		return "unknown"
	}
}

// Priority 消息优先级枚举
type Priority int

const (
	PriorityLow      Priority = iota // 低优先级
	PriorityNormal                   // 普通优先级
	PriorityHigh                     // 高优先级
	PriorityCritical                 // 关键优先级
)

// String 返回优先级的字符串表示
func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// ComponentStatus 组件状态枚举
type ComponentStatus int

const (
	StatusStopped  ComponentStatus = iota // 已停止
	StatusStarting                        // 启动中
	StatusRunning                         // 运行中
	StatusStopping                        // 停止中
	StatusError                           // 错误状态
)

// String 返回组件状态的字符串表示
func (cs ComponentStatus) String() string {
	switch cs {
	case StatusStopped:
		return "stopped"
	case StatusStarting:
		return "starting"
	case StatusRunning:
		return "running"
	case StatusStopping:
		return "stopping"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// CredentialType 凭据类型枚举
type CredentialType int

const (
	CredentialTypePassword    CredentialType = iota // 密码认证
	CredentialTypeToken                             // 令牌认证
	CredentialTypeCertificate                       // 证书认证
	CredentialTypeKey                               // 密钥认证
)

// String 返回凭据类型的字符串表示
func (ct CredentialType) String() string {
	switch ct {
	case CredentialTypePassword:
		return "password"
	case CredentialTypeToken:
		return "token"
	case CredentialTypeCertificate:
		return "certificate"
	case CredentialTypeKey:
		return "key"
	default:
		return "unknown"
	}
}

// Operation 操作类型枚举
type Operation int

const (
	OperationRead       Operation = iota // 读取操作
	OperationWrite                       // 写入操作
	OperationConnect                     // 连接操作
	OperationDisconnect                  // 断开操作
	OperationScan                        // 扫描操作
	OperationPair                        // 配对操作
)

// String 返回操作类型的字符串表示
func (op Operation) String() string {
	switch op {
	case OperationRead:
		return "read"
	case OperationWrite:
		return "write"
	case OperationConnect:
		return "connect"
	case OperationDisconnect:
		return "disconnect"
	case OperationScan:
		return "scan"
	case OperationPair:
		return "pair"
	default:
		return "unknown"
	}
}

// 默认配置常量
const (
	// 连接相关常量
	DefaultConnectTimeout    = 30 * time.Second // 默认连接超时时间
	DefaultScanTimeout       = 10 * time.Second // 默认扫描超时时间
	DefaultHeartbeatInterval = 5 * time.Second  // 默认心跳间隔
	DefaultRetryInterval     = 2 * time.Second  // 默认重试间隔
	DefaultSessionTimeout    = 30 * time.Minute // 默认会话超时时间

	// 连接池相关常量
	DefaultMaxConnections   = 10   // 默认最大连接数
	DefaultMaxRetries       = 3    // 默认最大重试次数
	DefaultMessageQueueSize = 1000 // 默认消息队列大小
	DefaultWorkerPoolSize   = 5    // 默认工作池大小

	// 安全相关常量
	DefaultEncryptionKeySize = 32 // 默认加密密钥大小（AES-256）
	DefaultNonceSize         = 12 // 默认随机数大小（GCM）
	DefaultTagSize           = 16 // 默认认证标签大小

	// 协议相关常量
	DefaultMTU                = 512                                    // 默认最大传输单元
	DefaultProtocolVersion    = "1.0"                                  // 默认协议版本
	DefaultServiceUUID        = "6E400001-B5A3-F393-E0A9-E50E24DCCA9E" // 默认服务UUID
	DefaultCharacteristicUUID = "6E400002-B5A3-F393-E0A9-E50E24DCCA9E" // 默认特征UUID

	// 性能相关常量
	DefaultBufferSize    = 4096            // 默认缓冲区大小
	DefaultBatchSize     = 100             // 默认批处理大小
	DefaultFlushInterval = 1 * time.Second // 默认刷新间隔
)

// 蓝牙协议常量
const (
	ProtocolRFCOMM = "RFCOMM" // RFCOMM协议
	ProtocolL2CAP  = "L2CAP"  // L2CAP协议
	ProtocolATT    = "ATT"    // ATT协议
	ProtocolGATT   = "GATT"   // GATT协议
)

// 设备类型常量
const (
	DeviceTypePhone    = "phone"    // 手机设备
	DeviceTypeTablet   = "tablet"   // 平板设备
	DeviceTypeLaptop   = "laptop"   // 笔记本电脑
	DeviceTypeDesktop  = "desktop"  // 台式电脑
	DeviceTypeHeadset  = "headset"  // 耳机设备
	DeviceTypeSpeaker  = "speaker"  // 扬声器设备
	DeviceTypeKeyboard = "keyboard" // 键盘设备
	DeviceTypeMouse    = "mouse"    // 鼠标设备
	DeviceTypeUnknown  = "unknown"  // 未知设备
)

// 错误代码常量
const (
	ErrCodeSuccess          = 0    // 成功
	ErrCodeGeneral          = 1000 // 通用错误
	ErrCodeInvalidParameter = 1001 // 无效参数
	ErrCodeTimeout          = 1002 // 超时错误
	ErrCodeNotFound         = 1003 // 未找到
	ErrCodeAlreadyExists    = 1004 // 已存在
	ErrCodePermissionDenied = 1005 // 权限拒绝
	ErrCodeResourceBusy     = 1006 // 资源忙
	ErrCodeNotSupported     = 1007 // 不支持
	ErrCodeConnectionFailed = 2000 // 连接失败
	ErrCodeDisconnected     = 2001 // 连接断开
	ErrCodeAuthFailed       = 2002 // 认证失败
	ErrCodeEncryptionFailed = 2003 // 加密失败
	ErrCodeProtocolError    = 2004 // 协议错误
	ErrCodeDeviceNotFound   = 3000 // 设备未找到
	ErrCodeDeviceBusy       = 3001 // 设备忙
	ErrCodeDeviceOffline    = 3002 // 设备离线
	ErrCodePairingFailed    = 3003 // 配对失败
)
