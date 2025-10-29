package bluetooth

import (
	"log"
	"sync"
	"time"
)

// DefaultConnectionHandler 默认连接处理器实现
type DefaultConnectionHandler[T any] struct {
	onConnectedFunc    func(Connection) error
	onDisconnectedFunc func(Connection, error)
	onMessageFunc      func(Connection, Message[T]) error
	onErrorFunc        func(Connection, error)
	mu                 sync.RWMutex
}

// NewDefaultConnectionHandler 创建新的默认连接处理器
func NewDefaultConnectionHandler[T any]() *DefaultConnectionHandler[T] {
	return &DefaultConnectionHandler[T]{
		onConnectedFunc: func(conn Connection) error {
			log.Printf("设备连接: %s", conn.DeviceID())
			return nil
		},
		onDisconnectedFunc: func(conn Connection, err error) {
			log.Printf("设备断开连接: %s, 原因: %v", conn.DeviceID(), err)
		},
		onMessageFunc: func(conn Connection, message Message[T]) error {
			log.Printf("收到消息: 来自设备 %s, 消息ID: %s", conn.DeviceID(), message.ID)
			return nil
		},
		onErrorFunc: func(conn Connection, err error) {
			log.Printf("连接错误: 设备 %s, 错误: %v", conn.DeviceID(), err)
		},
	}
}

// OnConnected 连接建立时的回调
func (h *DefaultConnectionHandler[T]) OnConnected(conn Connection) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.onConnectedFunc != nil {
		return h.onConnectedFunc(conn)
	}
	return nil
}

// OnDisconnected 连接断开时的回调
func (h *DefaultConnectionHandler[T]) OnDisconnected(conn Connection, err error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.onDisconnectedFunc != nil {
		h.onDisconnectedFunc(conn, err)
	}
}

// OnMessage 接收到消息时的回调
func (h *DefaultConnectionHandler[T]) OnMessage(conn Connection, message Message[T]) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.onMessageFunc != nil {
		return h.onMessageFunc(conn, message)
	}
	return nil
}

// OnError 发生错误时的回调
func (h *DefaultConnectionHandler[T]) OnError(conn Connection, err error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.onErrorFunc != nil {
		h.onErrorFunc(conn, err)
	}
}

// SetOnConnected 设置连接建立回调
func (h *DefaultConnectionHandler[T]) SetOnConnected(fn func(Connection) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onConnectedFunc = fn
}

// SetOnDisconnected 设置连接断开回调
func (h *DefaultConnectionHandler[T]) SetOnDisconnected(fn func(Connection, error)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onDisconnectedFunc = fn
}

// SetOnMessage 设置消息接收回调
func (h *DefaultConnectionHandler[T]) SetOnMessage(fn func(Connection, Message[T]) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onMessageFunc = fn
}

// SetOnError 设置错误回调
func (h *DefaultConnectionHandler[T]) SetOnError(fn func(Connection, error)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onErrorFunc = fn
}

// LoggingConnectionHandler 带日志记录的连接处理器
type LoggingConnectionHandler[T any] struct {
	*DefaultConnectionHandler[T]
	logger Logger
}

// Logger 日志接口
type Logger interface {
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// DefaultLogger 默认日志实现
type DefaultLogger struct{}

// Info 记录信息日志
func (l *DefaultLogger) Info(msg string, args ...interface{}) {
	log.Printf("[INFO] "+msg, args...)
}

// Warn 记录警告日志
func (l *DefaultLogger) Warn(msg string, args ...interface{}) {
	log.Printf("[WARN] "+msg, args...)
}

// Error 记录错误日志
func (l *DefaultLogger) Error(msg string, args ...interface{}) {
	log.Printf("[ERROR] "+msg, args...)
}

// Debug 记录调试日志
func (l *DefaultLogger) Debug(msg string, args ...interface{}) {
	log.Printf("[DEBUG] "+msg, args...)
}

// NewLoggingConnectionHandler 创建带日志记录的连接处理器
func NewLoggingConnectionHandler[T any](logger Logger) *LoggingConnectionHandler[T] {
	if logger == nil {
		logger = &DefaultLogger{}
	}

	handler := &LoggingConnectionHandler[T]{
		DefaultConnectionHandler: NewDefaultConnectionHandler[T](),
		logger:                   logger,
	}

	// 设置带日志的回调函数
	handler.SetOnConnected(func(conn Connection) error {
		handler.logger.Info("设备连接建立: ID=%s, 地址=%s", conn.DeviceID(), conn.ID())
		return nil
	})

	handler.SetOnDisconnected(func(conn Connection, err error) {
		if err != nil {
			handler.logger.Warn("设备连接断开: ID=%s, 原因=%v", conn.DeviceID(), err)
		} else {
			handler.logger.Info("设备连接正常断开: ID=%s", conn.DeviceID())
		}
	})

	handler.SetOnMessage(func(conn Connection, message Message[T]) error {
		handler.logger.Debug("收到消息: 设备=%s, 消息ID=%s, 类型=%s, 时间=%s",
			conn.DeviceID(), message.ID, message.Type.String(), message.Timestamp.Format(time.RFC3339))
		return nil
	})

	handler.SetOnError(func(conn Connection, err error) {
		handler.logger.Error("连接错误: 设备=%s, 错误=%v", conn.DeviceID(), err)
	})

	return handler
}

// MetricsConnectionHandler 带指标收集的连接处理器
type MetricsConnectionHandler[T any] struct {
	*DefaultConnectionHandler[T]
	metrics *ConnectionMetrics
	mu      sync.RWMutex
}

// NewMetricsConnectionHandler 创建带指标收集的连接处理器
func NewMetricsConnectionHandler[T any]() *MetricsConnectionHandler[T] {
	handler := &MetricsConnectionHandler[T]{
		DefaultConnectionHandler: NewDefaultConnectionHandler[T](),
		metrics: &ConnectionMetrics{
			ConnectedAt: time.Now(),
		},
	}

	// 设置带指标收集的回调函数
	handler.SetOnConnected(func(conn Connection) error {
		handler.mu.Lock()
		handler.metrics.ConnectedAt = time.Now()
		handler.metrics.LastActivity = time.Now()
		handler.mu.Unlock()
		return nil
	})

	handler.SetOnMessage(func(conn Connection, message Message[T]) error {
		handler.mu.Lock()
		handler.metrics.MessagesRecv++
		handler.metrics.LastActivity = time.Now()
		handler.mu.Unlock()
		return nil
	})

	handler.SetOnError(func(conn Connection, err error) {
		handler.mu.Lock()
		handler.metrics.ErrorCount++
		handler.mu.Unlock()
	})

	return handler
}

// GetMetrics 获取连接指标
func (h *MetricsConnectionHandler[T]) GetMetrics() ConnectionMetrics {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return *h.metrics
}

// ResetMetrics 重置指标
func (h *MetricsConnectionHandler[T]) ResetMetrics() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.metrics = &ConnectionMetrics{
		ConnectedAt: time.Now(),
	}
}

// AuthConnectionHandler 带认证的连接处理器
type AuthConnectionHandler[T any] struct {
	*DefaultConnectionHandler[T]
	securityManager      SecurityManager
	requireAuth          bool
	authenticatedDevices map[string]bool
	mu                   sync.RWMutex
}

// NewAuthConnectionHandler 创建带认证的连接处理器
func NewAuthConnectionHandler[T any](securityManager SecurityManager, requireAuth bool) *AuthConnectionHandler[T] {
	handler := &AuthConnectionHandler[T]{
		DefaultConnectionHandler: NewDefaultConnectionHandler[T](),
		securityManager:          securityManager,
		requireAuth:              requireAuth,
		authenticatedDevices:     make(map[string]bool),
	}

	// 设置带认证的回调函数
	handler.SetOnConnected(func(conn Connection) error {
		if !handler.requireAuth {
			return nil
		}

		deviceID := conn.DeviceID()

		// 检查设备是否已认证
		handler.mu.RLock()
		authenticated := handler.authenticatedDevices[deviceID]
		handler.mu.RUnlock()

		if authenticated {
			return nil
		}

		// 执行设备认证
		credentials := Credentials{
			Type: CredentialTypeKey,
			// 实际实现中应该从连接中获取认证信息
		}

		if err := handler.securityManager.Authenticate(deviceID, credentials); err != nil {
			return WrapError(err, ErrCodeAuthFailed, "设备认证失败", deviceID, "authenticate")
		}

		// 标记设备为已认证
		handler.mu.Lock()
		handler.authenticatedDevices[deviceID] = true
		handler.mu.Unlock()

		return nil
	})

	handler.SetOnDisconnected(func(conn Connection, err error) {
		// 清除认证状态
		handler.mu.Lock()
		delete(handler.authenticatedDevices, conn.DeviceID())
		handler.mu.Unlock()
	})

	return handler
}

// IsAuthenticated 检查设备是否已认证
func (h *AuthConnectionHandler[T]) IsAuthenticated(deviceID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.authenticatedDevices[deviceID]
}

// CompositeConnectionHandler 复合连接处理器，支持多个处理器链式调用
type CompositeConnectionHandler[T any] struct {
	handlers []ConnectionHandler[T]
	mu       sync.RWMutex
}

// NewCompositeConnectionHandler 创建复合连接处理器
func NewCompositeConnectionHandler[T any](handlers ...ConnectionHandler[T]) *CompositeConnectionHandler[T] {
	return &CompositeConnectionHandler[T]{
		handlers: handlers,
	}
}

// AddHandler 添加处理器
func (h *CompositeConnectionHandler[T]) AddHandler(handler ConnectionHandler[T]) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handlers = append(h.handlers, handler)
}

// OnConnected 连接建立时的回调
func (h *CompositeConnectionHandler[T]) OnConnected(conn Connection) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, handler := range h.handlers {
		if err := handler.OnConnected(conn); err != nil {
			return err
		}
	}
	return nil
}

// OnDisconnected 连接断开时的回调
func (h *CompositeConnectionHandler[T]) OnDisconnected(conn Connection, err error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, handler := range h.handlers {
		handler.OnDisconnected(conn, err)
	}
}

// OnMessage 接收到消息时的回调
func (h *CompositeConnectionHandler[T]) OnMessage(conn Connection, message Message[T]) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, handler := range h.handlers {
		if err := handler.OnMessage(conn, message); err != nil {
			return err
		}
	}
	return nil
}

// OnError 发生错误时的回调
func (h *CompositeConnectionHandler[T]) OnError(conn Connection, err error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, handler := range h.handlers {
		handler.OnError(conn, err)
	}
}
