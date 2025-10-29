package bluetooth

import (
	"time"
)

// SecurityManagerExtended 扩展的安全管理器接口，包含访问控制功能
type SecurityManagerExtended interface {
	SecurityManager

	// 白名单管理
	AddToWhitelist(deviceID string) error
	RemoveFromWhitelist(deviceID string) error
	IsWhitelisted(deviceID string) bool
	GetWhitelist() []string

	// 访问策略管理
	SetAccessPolicy(deviceID string, policy *AccessPolicy) error
	GetAccessPolicy(deviceID string) (*AccessPolicy, error)

	// 信任设备管理
	AddTrustedDevice(deviceID string) error
	RemoveTrustedDevice(deviceID string) error
	IsTrustedDevice(deviceID string) bool
	GetTrustedDevices() []string

	// 会话管理
	RevokeSession(deviceID string) error
	GetActiveSession(deviceID string) (SecuritySession, error)
	CleanupExpiredSessions()

	// 访问验证
	ValidateAccess(deviceID string, operation Operation) error

	// 事件和回调
	RegisterAuthCallback(callback AuthCallback) error
	GetSecurityEvents() <-chan SecurityEvent
}

// SecuritySession 安全会话信息
type SecuritySession struct {
	DeviceID        string      `json:"device_id"`        // 设备ID
	SessionKey      []byte      `json:"session_key"`      // 会话密钥
	CreatedAt       time.Time   `json:"created_at"`       // 创建时间
	LastActivity    time.Time   `json:"last_activity"`    // 最后活动时间
	IsAuthenticated bool        `json:"is_authenticated"` // 是否已认证
	Permissions     []Operation `json:"permissions"`      // 权限列表
	FailedAttempts  int         `json:"failed_attempts"`  // 失败尝试次数
}

// AuthCallback 认证回调函数类型
type AuthCallback func(deviceID string, event AuthEvent) error

// AuthEvent 认证事件
type AuthEvent struct {
	Type      AuthEventType `json:"type"`      // 事件类型
	DeviceID  string        `json:"device_id"` // 设备ID
	Timestamp time.Time     `json:"timestamp"` // 时间戳
	Success   bool          `json:"success"`   // 是否成功
	Message   string        `json:"message"`   // 事件消息
}

// AuthEventType 认证事件类型
type AuthEventType int

const (
	AuthEventLogin   AuthEventType = iota // 登录事件
	AuthEventLogout                       // 登出事件
	AuthEventFailed                       // 认证失败
	AuthEventBlocked                      // 设备被阻止
)

// String 返回认证事件类型的字符串表示
func (aet AuthEventType) String() string {
	switch aet {
	case AuthEventLogin:
		return "login"
	case AuthEventLogout:
		return "logout"
	case AuthEventFailed:
		return "failed"
	case AuthEventBlocked:
		return "blocked"
	default:
		return "unknown"
	}
}

// SecurityEvent 安全事件
type SecurityEvent struct {
	ID        string            `json:"id"`        // 事件ID
	Type      SecurityEventType `json:"type"`      // 事件类型
	DeviceID  string            `json:"device_id"` // 设备ID
	Timestamp time.Time         `json:"timestamp"` // 时间戳
	Severity  EventSeverity     `json:"severity"`  // 严重程度
	Message   string            `json:"message"`   // 事件消息
	Details   map[string]string `json:"details"`   // 事件详情
}

// SecurityEventType 安全事件类型
type SecurityEventType int

const (
	SecurityEventAuthSuccess        SecurityEventType = iota // 认证成功
	SecurityEventAuthFailed                                  // 认证失败
	SecurityEventUnauthorized                                // 未授权访问
	SecurityEventEncryptionFailed                            // 加密失败
	SecurityEventSuspiciousActivity                          // 可疑活动
	SecurityEventDeviceBlocked                               // 设备被阻止
	SecurityEventError                                       // 错误事件
)

// String 返回安全事件类型的字符串表示
func (set SecurityEventType) String() string {
	switch set {
	case SecurityEventAuthSuccess:
		return "auth_success"
	case SecurityEventAuthFailed:
		return "auth_failed"
	case SecurityEventUnauthorized:
		return "unauthorized"
	case SecurityEventEncryptionFailed:
		return "encryption_failed"
	case SecurityEventSuspiciousActivity:
		return "suspicious_activity"
	case SecurityEventDeviceBlocked:
		return "device_blocked"
	case SecurityEventError:
		return "error"
	default:
		return "unknown"
	}
}

// EventSeverity 事件严重程度
type EventSeverity int

const (
	SeverityInfo     EventSeverity = iota // 信息
	SeverityWarning                       // 警告
	SeverityError                         // 错误
	SeverityCritical                      // 严重
)

// String 返回事件严重程度的字符串表示
func (es EventSeverity) String() string {
	switch es {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}
