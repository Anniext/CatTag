package bluetooth

import (
	"fmt"
	"time"
)

// BluetoothError 蓝牙错误类型，利用 Go 1.25 改进的错误处理
type BluetoothError struct {
	Code      int                    `json:"code"`      // 错误代码
	Message   string                 `json:"message"`   // 错误消息
	DeviceID  string                 `json:"device_id"` // 设备ID
	Operation string                 `json:"operation"` // 操作类型
	Timestamp time.Time              `json:"timestamp"` // 错误时间戳
	Cause     error                  `json:"-"`         // 原始错误
	Context   map[string]interface{} `json:"context"`   // 错误上下文
}

// Error 实现 error 接口
func (e *BluetoothError) Error() string {
	if e.DeviceID != "" {
		return fmt.Sprintf("bluetooth error [%d]: %s (device: %s, operation: %s)",
			e.Code, e.Message, e.DeviceID, e.Operation)
	}
	return fmt.Sprintf("bluetooth error [%d]: %s (operation: %s)",
		e.Code, e.Message, e.Operation)
}

// Unwrap 实现错误包装接口，支持 Go 1.25 错误处理
func (e *BluetoothError) Unwrap() error {
	return e.Cause
}

// Is 实现错误比较接口
func (e *BluetoothError) Is(target error) bool {
	if t, ok := target.(*BluetoothError); ok {
		return e.Code == t.Code
	}
	return false
}

// WithContext 添加错误上下文
func (e *BluetoothError) WithContext(key string, value interface{}) *BluetoothError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// NewBluetoothError 创建新的蓝牙错误
func NewBluetoothError(code int, message string) *BluetoothError {
	return &BluetoothError{
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// NewBluetoothErrorWithDevice 创建带设备信息的蓝牙错误
func NewBluetoothErrorWithDevice(code int, message, deviceID, operation string) *BluetoothError {
	return &BluetoothError{
		Code:      code,
		Message:   message,
		DeviceID:  deviceID,
		Operation: operation,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// WrapError 包装现有错误
func WrapError(err error, code int, message, deviceID, operation string) *BluetoothError {
	return &BluetoothError{
		Code:      code,
		Message:   message,
		DeviceID:  deviceID,
		Operation: operation,
		Timestamp: time.Now(),
		Cause:     err,
		Context:   make(map[string]interface{}),
	}
}

// 预定义的错误变量
var (
	// 通用错误
	ErrInvalidParameter = NewBluetoothError(ErrCodeInvalidParameter, "无效参数")
	ErrTimeout          = NewBluetoothError(ErrCodeTimeout, "操作超时")
	ErrNotFound         = NewBluetoothError(ErrCodeNotFound, "资源未找到")
	ErrAlreadyExists    = NewBluetoothError(ErrCodeAlreadyExists, "资源已存在")
	ErrPermissionDenied = NewBluetoothError(ErrCodePermissionDenied, "权限被拒绝")
	ErrResourceBusy     = NewBluetoothError(ErrCodeResourceBusy, "资源忙")
	ErrNotSupported     = NewBluetoothError(ErrCodeNotSupported, "操作不支持")

	// 连接相关错误
	ErrConnectionFailed = NewBluetoothError(ErrCodeConnectionFailed, "连接失败")
	ErrDisconnected     = NewBluetoothError(ErrCodeDisconnected, "连接已断开")
	ErrAuthFailed       = NewBluetoothError(ErrCodeAuthFailed, "认证失败")
	ErrEncryptionFailed = NewBluetoothError(ErrCodeEncryptionFailed, "加密失败")
	ErrProtocolError    = NewBluetoothError(ErrCodeProtocolError, "协议错误")

	// 设备相关错误
	ErrDeviceNotFound = NewBluetoothError(ErrCodeDeviceNotFound, "设备未找到")
	ErrDeviceBusy     = NewBluetoothError(ErrCodeDeviceBusy, "设备忙")
	ErrDeviceOffline  = NewBluetoothError(ErrCodeDeviceOffline, "设备离线")
	ErrPairingFailed  = NewBluetoothError(ErrCodePairingFailed, "设备配对失败")
)

// IsConnectionError 检查是否为连接相关错误
func IsConnectionError(err error) bool {
	if btErr, ok := err.(*BluetoothError); ok {
		return btErr.Code >= 2000 && btErr.Code < 3000
	}
	return false
}

// IsDeviceError 检查是否为设备相关错误
func IsDeviceError(err error) bool {
	if btErr, ok := err.(*BluetoothError); ok {
		return btErr.Code >= 3000 && btErr.Code < 4000
	}
	return false
}

// IsRetryableError 检查错误是否可重试
func IsRetryableError(err error) bool {
	if btErr, ok := err.(*BluetoothError); ok {
		switch btErr.Code {
		case ErrCodeTimeout, ErrCodeResourceBusy, ErrCodeConnectionFailed, ErrCodeDeviceBusy:
			return true
		default:
			return false
		}
	}
	return false
}

// ErrorRecoveryStrategy 错误恢复策略
type ErrorRecoveryStrategy struct {
	MaxRetries    int           `json:"max_retries"`    // 最大重试次数
	RetryInterval time.Duration `json:"retry_interval"` // 重试间隔
	BackoffFactor float64       `json:"backoff_factor"` // 退避因子
	MaxInterval   time.Duration `json:"max_interval"`   // 最大间隔
}

// DefaultRecoveryStrategy 默认恢复策略
var DefaultRecoveryStrategy = ErrorRecoveryStrategy{
	MaxRetries:    3,
	RetryInterval: 1 * time.Second,
	BackoffFactor: 2.0,
	MaxInterval:   30 * time.Second,
}

// CalculateRetryDelay 计算重试延迟时间
func (ers *ErrorRecoveryStrategy) CalculateRetryDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return ers.RetryInterval
	}

	delay := ers.RetryInterval
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * ers.BackoffFactor)
		if delay > ers.MaxInterval {
			return ers.MaxInterval
		}
	}
	return delay
}

// ShouldRetry 判断是否应该重试
func (ers *ErrorRecoveryStrategy) ShouldRetry(err error, attempt int) bool {
	if attempt >= ers.MaxRetries {
		return false
	}
	return IsRetryableError(err)
}
