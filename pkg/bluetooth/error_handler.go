package bluetooth

import (
	"fmt"
	"time"
)

// NewUnifiedErrorHandler 创建新的统一错误处理器
func NewUnifiedErrorHandler() *UnifiedErrorHandler {
	return &UnifiedErrorHandler{
		errorHandlers:   make(map[int]ErrorHandlerFunc),
		errorCallbacks:  make([]ErrorCallback, 0),
		errorHistory:    make([]ErrorRecord, 0),
		maxHistorySize:  1000,
		retryStrategies: make(map[int]*ErrorRecoveryStrategy),
	}
}

// HandleError 处理错误
func (ueh *UnifiedErrorHandler) HandleError(err *BluetoothError) error {
	ueh.mu.Lock()
	defer ueh.mu.Unlock()

	// 记录错误
	record := ErrorRecord{
		Error:     err,
		Component: err.DeviceID, // 使用设备ID作为组件标识
		Timestamp: time.Now(),
		Handled:   false,
	}

	// 添加到历史记录
	ueh.addToHistory(record)

	// 触发错误回调
	ueh.triggerErrorCallbacks(err)

	// 查找对应的错误处理器
	handler, exists := ueh.errorHandlers[err.Code]
	if exists {
		if handlerErr := handler(err); handlerErr != nil {
			return fmt.Errorf("错误处理器执行失败: %w", handlerErr)
		}
		record.Handled = true
		record.Recovery = "使用自定义处理器"
	} else {
		// 使用默认处理逻辑
		if handlerErr := ueh.defaultErrorHandler(err); handlerErr != nil {
			return fmt.Errorf("默认错误处理失败: %w", handlerErr)
		}
		record.Handled = true
		record.Recovery = "使用默认处理器"
	}

	// 更新记录
	ueh.updateHistoryRecord(record)

	return nil
}

// AddHandler 添加错误处理器
func (ueh *UnifiedErrorHandler) AddHandler(errorCode int, handler ErrorHandlerFunc) {
	ueh.mu.Lock()
	defer ueh.mu.Unlock()

	ueh.errorHandlers[errorCode] = handler
}

// AddCallback 添加错误回调
func (ueh *UnifiedErrorHandler) AddCallback(callback ErrorCallback) {
	ueh.mu.Lock()
	defer ueh.mu.Unlock()

	ueh.errorCallbacks = append(ueh.errorCallbacks, callback)
}

// SetRetryStrategy 设置重试策略
func (ueh *UnifiedErrorHandler) SetRetryStrategy(errorCode int, strategy *ErrorRecoveryStrategy) {
	ueh.mu.Lock()
	defer ueh.mu.Unlock()

	ueh.retryStrategies[errorCode] = strategy
}

// GetErrorHistory 获取错误历史
func (ueh *UnifiedErrorHandler) GetErrorHistory() []ErrorRecord {
	ueh.mu.RLock()
	defer ueh.mu.RUnlock()

	// 返回副本
	history := make([]ErrorRecord, len(ueh.errorHistory))
	copy(history, ueh.errorHistory)
	return history
}

// GetErrorStatistics 获取错误统计
func (ueh *UnifiedErrorHandler) GetErrorStatistics() ErrorStatistics {
	ueh.mu.RLock()
	defer ueh.mu.RUnlock()

	stats := ErrorStatistics{
		TotalErrors:       len(ueh.errorHistory),
		HandledErrors:     0,
		ErrorsByCode:      make(map[int]int),
		ErrorsByComponent: make(map[string]int),
	}

	for _, record := range ueh.errorHistory {
		if record.Handled {
			stats.HandledErrors++
		}
		stats.ErrorsByCode[record.Error.Code]++
		stats.ErrorsByComponent[record.Component]++
	}

	return stats
}

// ClearHistory 清空错误历史
func (ueh *UnifiedErrorHandler) ClearHistory() {
	ueh.mu.Lock()
	defer ueh.mu.Unlock()

	ueh.errorHistory = make([]ErrorRecord, 0)
}

// defaultErrorHandler 默认错误处理器
func (ueh *UnifiedErrorHandler) defaultErrorHandler(err *BluetoothError) error {
	// 根据错误代码进行默认处理
	switch err.Code {
	case ErrCodeTimeout:
		return ueh.handleTimeoutError(err)
	case ErrCodeConnectionFailed:
		return ueh.handleConnectionError(err)
	case ErrCodeAuthFailed:
		return ueh.handleAuthError(err)
	case ErrCodeDeviceNotFound:
		return ueh.handleDeviceError(err)
	default:
		return ueh.handleGenericError(err)
	}
}

// handleTimeoutError 处理超时错误
func (ueh *UnifiedErrorHandler) handleTimeoutError(err *BluetoothError) error {
	// 检查是否有重试策略
	strategy, exists := ueh.retryStrategies[err.Code]
	if exists && IsRetryableError(err) {
		// 实现重试逻辑
		return ueh.scheduleRetry(err, strategy)
	}
	return nil
}

// handleConnectionError 处理连接错误
func (ueh *UnifiedErrorHandler) handleConnectionError(err *BluetoothError) error {
	// 连接错误的默认处理逻辑
	// 可以尝试重新连接或清理资源
	return nil
}

// handleAuthError 处理认证错误
func (ueh *UnifiedErrorHandler) handleAuthError(err *BluetoothError) error {
	// 认证错误的默认处理逻辑
	// 可以清除认证缓存或要求重新认证
	return nil
}

// handleDeviceError 处理设备错误
func (ueh *UnifiedErrorHandler) handleDeviceError(err *BluetoothError) error {
	// 设备错误的默认处理逻辑
	// 可以从设备列表中移除设备或标记为离线
	return nil
}

// handleGenericError 处理通用错误
func (ueh *UnifiedErrorHandler) handleGenericError(err *BluetoothError) error {
	// 通用错误的默认处理逻辑
	// 记录日志并继续执行
	return nil
}

// scheduleRetry 安排重试
func (ueh *UnifiedErrorHandler) scheduleRetry(err *BluetoothError, strategy *ErrorRecoveryStrategy) error {
	// 简化的重试逻辑
	// 实际实现中可能需要更复杂的调度机制
	return nil
}

// addToHistory 添加到历史记录
func (ueh *UnifiedErrorHandler) addToHistory(record ErrorRecord) {
	// 检查历史记录大小
	if len(ueh.errorHistory) >= ueh.maxHistorySize {
		// 移除最旧的记录
		ueh.errorHistory = ueh.errorHistory[1:]
	}

	ueh.errorHistory = append(ueh.errorHistory, record)
}

// updateHistoryRecord 更新历史记录
func (ueh *UnifiedErrorHandler) updateHistoryRecord(record ErrorRecord) {
	// 查找并更新对应的记录
	for i := len(ueh.errorHistory) - 1; i >= 0; i-- {
		if ueh.errorHistory[i].Timestamp == record.Timestamp &&
			ueh.errorHistory[i].Error.Code == record.Error.Code {
			ueh.errorHistory[i] = record
			break
		}
	}
}

// triggerErrorCallbacks 触发错误回调
func (ueh *UnifiedErrorHandler) triggerErrorCallbacks(err *BluetoothError) {
	for _, callback := range ueh.errorCallbacks {
		go callback(err)
	}
}

// ErrorStatistics 错误统计信息
type ErrorStatistics struct {
	TotalErrors       int            `json:"total_errors"`        // 总错误数
	HandledErrors     int            `json:"handled_errors"`      // 已处理错误数
	ErrorsByCode      map[int]int    `json:"errors_by_code"`      // 按错误代码分组
	ErrorsByComponent map[string]int `json:"errors_by_component"` // 按组件分组
}
