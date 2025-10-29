package bluetooth

import (
	"fmt"
	"sync"
	"time"
)

// ServerConnectionHandler 服务端连接处理器，集成会话管理、超时管理和安全检查
type ServerConnectionHandler[T any] struct {
	sessionManager  *SessionManager
	timeoutManager  *TimeoutManager
	resourceManager *ResourceManager
	securityManager SecurityManager
	config          ServerConnectionConfig
	callbacks       *ServerCallbacks[T]
	mu              sync.RWMutex
}

// ServerConnectionConfig 服务端连接配置
type ServerConnectionConfig struct {
	RequireAuth       bool          `json:"require_auth"`         // 是否需要认证
	EnableEncryption  bool          `json:"enable_encryption"`    // 是否启用加密
	AuthTimeout       time.Duration `json:"auth_timeout"`         // 认证超时时间
	MaxAuthAttempts   int           `json:"max_auth_attempts"`    // 最大认证尝试次数
	EnableRateLimit   bool          `json:"enable_rate_limit"`    // 启用速率限制
	MaxRequestsPerSec int           `json:"max_requests_per_sec"` // 每秒最大请求数
	EnableLogging     bool          `json:"enable_logging"`       // 启用日志记录
	LogLevel          string        `json:"log_level"`            // 日志级别
}

// DefaultServerConnectionConfig 默认服务端连接配置
func DefaultServerConnectionConfig() ServerConnectionConfig {
	return ServerConnectionConfig{
		RequireAuth:       true,
		EnableEncryption:  true,
		AuthTimeout:       30 * time.Second,
		MaxAuthAttempts:   3,
		EnableRateLimit:   true,
		MaxRequestsPerSec: 100,
		EnableLogging:     true,
		LogLevel:          "info",
	}
}

// ServerCallbacks 服务端回调函数集合
type ServerCallbacks[T any] struct {
	OnConnectionEstablished func(session *ClientSession) error
	OnConnectionClosed      func(session *ClientSession, err error)
	OnAuthenticationSuccess func(session *ClientSession) error
	OnAuthenticationFailed  func(session *ClientSession, err error)
	OnMessageReceived       func(session *ClientSession, message Message[T]) error
	OnMessageSent           func(session *ClientSession, message Message[T]) error
	OnError                 func(session *ClientSession, err error)
	OnTimeout               func(session *ClientSession, timeoutType TimeoutType) error
	OnResourceExhausted     func(session *ClientSession, resourceType ResourceType) error
}

// NewServerConnectionHandler 创建新的服务端连接处理器
func NewServerConnectionHandler[T any](
	sessionManager *SessionManager,
	timeoutManager *TimeoutManager,
	resourceManager *ResourceManager,
	securityManager SecurityManager,
	config ServerConnectionConfig,
) *ServerConnectionHandler[T] {
	return &ServerConnectionHandler[T]{
		sessionManager:  sessionManager,
		timeoutManager:  timeoutManager,
		resourceManager: resourceManager,
		securityManager: securityManager,
		config:          config,
		callbacks:       &ServerCallbacks[T]{},
	}
}

// SetCallbacks 设置回调函数
func (sch *ServerConnectionHandler[T]) SetCallbacks(callbacks *ServerCallbacks[T]) {
	sch.mu.Lock()
	defer sch.mu.Unlock()
	sch.callbacks = callbacks
}

// OnConnected 连接建立时的回调
func (sch *ServerConnectionHandler[T]) OnConnected(conn Connection) error {
	deviceID := conn.DeviceID()

	// 创建会话
	session, err := sch.sessionManager.CreateSession(conn)
	if err != nil {
		return WrapError(err, ErrCodeConnectionFailed, "创建会话失败", deviceID, "on_connected")
	}

	// 分配连接资源
	resourceID, err := sch.resourceManager.AllocateResource(deviceID, ResourceTypeConnection, 1, conn)
	if err != nil {
		sch.sessionManager.CloseSession(deviceID)
		return WrapError(err, ErrCodeResourceBusy, "分配连接资源失败", deviceID, "on_connected")
	}

	// 添加到会话元数据
	session.Metadata["resource_id"] = resourceID

	// 设置连接超时
	connectTimeoutID, err := sch.timeoutManager.AddConnectTimeout(deviceID, func(entry *TimeoutEntry) error {
		return sch.handleConnectionTimeout(session, TimeoutTypeConnect)
	})
	if err != nil {
		sch.cleanup(session)
		return WrapError(err, ErrCodeTimeout, "设置连接超时失败", deviceID, "on_connected")
	}
	session.Metadata["connect_timeout_id"] = connectTimeoutID

	// 如果需要认证，设置认证超时
	if sch.config.RequireAuth {
		authTimeoutID, err := sch.timeoutManager.AddTimeout(deviceID, TimeoutTypeCustom, sch.config.AuthTimeout, func(entry *TimeoutEntry) error {
			return sch.handleAuthTimeout(session)
		})
		if err != nil {
			sch.cleanup(session)
			return WrapError(err, ErrCodeTimeout, "设置认证超时失败", deviceID, "on_connected")
		}
		session.Metadata["auth_timeout_id"] = authTimeoutID

		// 开始认证流程
		if err := sch.startAuthentication(session); err != nil {
			sch.cleanup(session)
			return WrapError(err, ErrCodeAuthFailed, "启动认证流程失败", deviceID, "on_connected")
		}
	} else {
		// 不需要认证，直接激活会话
		session.Status = SessionStatusActive
	}

	// 调用用户回调
	if sch.callbacks.OnConnectionEstablished != nil {
		if err := sch.callbacks.OnConnectionEstablished(session); err != nil {
			sch.cleanup(session)
			return WrapError(err, ErrCodeConnectionFailed, "连接建立回调失败", deviceID, "on_connected")
		}
	}

	sch.logInfo("连接建立成功: 设备=%s, 会话=%s", deviceID, session.ID)
	return nil
}

// OnDisconnected 连接断开时的回调
func (sch *ServerConnectionHandler[T]) OnDisconnected(conn Connection, err error) {
	deviceID := conn.DeviceID()

	// 获取会话
	session, sessionErr := sch.sessionManager.GetSession(deviceID)
	if sessionErr != nil {
		sch.logError("获取会话失败: 设备=%s, 错误=%v", deviceID, sessionErr)
		return
	}

	// 调用用户回调
	if sch.callbacks.OnConnectionClosed != nil {
		sch.callbacks.OnConnectionClosed(session, err)
	}

	// 清理资源
	sch.cleanup(session)

	if err != nil {
		sch.logWarn("连接断开: 设备=%s, 原因=%v", deviceID, err)
	} else {
		sch.logInfo("连接正常断开: 设备=%s", deviceID)
	}
}

// OnMessage 接收到消息时的回调
func (sch *ServerConnectionHandler[T]) OnMessage(conn Connection, message Message[T]) error {
	deviceID := conn.DeviceID()

	// 获取会话
	session, err := sch.sessionManager.GetSession(deviceID)
	if err != nil {
		return WrapError(err, ErrCodeNotFound, "获取会话失败", deviceID, "on_message")
	}

	// 更新会话活动时间
	sch.sessionManager.UpdateActivity(deviceID)

	// 检查会话状态
	if session.Status == SessionStatusClosed || session.Status == SessionStatusExpired {
		return NewBluetoothErrorWithDevice(ErrCodeNotSupported, "会话已关闭或过期", deviceID, "on_message")
	}

	// 如果需要认证但未认证，拒绝消息
	if sch.config.RequireAuth && !session.IsAuthenticated {
		// 检查是否是认证消息
		if message.Type != MessageTypeAuth {
			return NewBluetoothErrorWithDevice(ErrCodeAuthFailed, "会话未认证", deviceID, "on_message")
		}

		// 处理认证消息
		return sch.handleAuthMessage(session, message)
	}

	// 处理心跳消息
	if message.Type == MessageTypeHeartbeat {
		return sch.handleHeartbeatMessage(session, message)
	}

	// 速率限制检查
	if sch.config.EnableRateLimit {
		if err := sch.checkRateLimit(session); err != nil {
			return WrapError(err, ErrCodeResourceBusy, "速率限制", deviceID, "on_message")
		}
	}

	// 解密消息（如果启用加密）
	if sch.config.EnableEncryption && session.IsEncrypted {
		if err := sch.decryptMessage(&message, session); err != nil {
			return WrapError(err, ErrCodeEncryptionFailed, "消息解密失败", deviceID, "on_message")
		}
	}

	// 调用用户回调
	if sch.callbacks.OnMessageReceived != nil {
		if err := sch.callbacks.OnMessageReceived(session, message); err != nil {
			return WrapError(err, ErrCodeProtocolError, "消息处理回调失败", deviceID, "on_message")
		}
	}

	sch.logDebug("收到消息: 设备=%s, 消息ID=%s, 类型=%s", deviceID, message.ID, message.Type.String())
	return nil
}

// OnError 发生错误时的回调
func (sch *ServerConnectionHandler[T]) OnError(conn Connection, err error) {
	deviceID := conn.DeviceID()

	// 获取会话
	session, sessionErr := sch.sessionManager.GetSession(deviceID)
	if sessionErr != nil {
		sch.logError("获取会话失败: 设备=%s, 错误=%v", deviceID, sessionErr)
		return
	}

	// 调用用户回调
	if sch.callbacks.OnError != nil {
		sch.callbacks.OnError(session, err)
	}

	sch.logError("连接错误: 设备=%s, 错误=%v", deviceID, err)
}

// SendMessage 发送消息到会话
func (sch *ServerConnectionHandler[T]) SendMessage(session *ClientSession, message Message[T]) error {
	deviceID := session.DeviceID

	// 检查会话状态
	if session.Status == SessionStatusClosed || session.Status == SessionStatusExpired {
		return NewBluetoothErrorWithDevice(ErrCodeNotSupported, "会话已关闭或过期", deviceID, "send_message")
	}

	// 加密消息（如果启用加密）
	if sch.config.EnableEncryption && session.IsEncrypted {
		if err := sch.encryptMessage(&message, session); err != nil {
			return WrapError(err, ErrCodeEncryptionFailed, "消息加密失败", deviceID, "send_message")
		}
	}

	// 序列化消息
	messageData := []byte(fmt.Sprintf("%+v", message))

	// 发送消息
	if err := session.Connection.Send(messageData); err != nil {
		return WrapError(err, ErrCodeConnectionFailed, "发送消息失败", deviceID, "send_message")
	}

	// 更新会话活动时间
	sch.sessionManager.UpdateActivity(deviceID)

	// 调用用户回调
	if sch.callbacks.OnMessageSent != nil {
		sch.callbacks.OnMessageSent(session, message)
	}

	sch.logDebug("发送消息: 设备=%s, 消息ID=%s, 类型=%s", deviceID, message.ID, message.Type.String())
	return nil
}

// 内部方法

// startAuthentication 开始认证流程
func (sch *ServerConnectionHandler[T]) startAuthentication(session *ClientSession) error {
	session.Status = SessionStatusAuthenticating

	// 发送认证请求消息
	authRequest := Message[T]{
		ID:        fmt.Sprintf("auth_req_%d", time.Now().UnixNano()),
		Type:      MessageTypeAuth,
		Timestamp: time.Now(),
		Metadata: MessageMetadata{
			Priority: PriorityHigh,
		},
	}

	return sch.SendMessage(session, authRequest)
}

// handleAuthMessage 处理认证消息
func (sch *ServerConnectionHandler[T]) handleAuthMessage(session *ClientSession, message Message[T]) error {
	deviceID := session.DeviceID

	// 从消息中提取认证信息（这里简化处理）
	credentials := Credentials{
		Type: CredentialTypeKey,
		// 实际实现中应该从消息载荷中解析认证信息
	}

	// 执行认证
	if err := sch.sessionManager.AuthenticateSession(deviceID, credentials); err != nil {
		// 认证失败，增加失败计数
		attempts, _ := session.Metadata["auth_attempts"].(int)
		attempts++
		session.Metadata["auth_attempts"] = attempts

		if attempts >= sch.config.MaxAuthAttempts {
			// 达到最大尝试次数，关闭会话
			sch.cleanup(session)
			return NewBluetoothErrorWithDevice(ErrCodeAuthFailed, "认证失败次数过多", deviceID, "auth")
		}

		// 调用认证失败回调
		if sch.callbacks.OnAuthenticationFailed != nil {
			sch.callbacks.OnAuthenticationFailed(session, err)
		}

		return WrapError(err, ErrCodeAuthFailed, "认证失败", deviceID, "auth")
	}

	// 认证成功，移除认证超时
	if authTimeoutID, exists := session.Metadata["auth_timeout_id"].(string); exists {
		sch.timeoutManager.RemoveTimeout(authTimeoutID)
		delete(session.Metadata, "auth_timeout_id")
	}

	// 启用加密（如果配置了）
	if sch.config.EnableEncryption {
		if err := sch.sessionManager.EnableEncryption(deviceID); err != nil {
			return WrapError(err, ErrCodeEncryptionFailed, "启用加密失败", deviceID, "auth")
		}
	}

	session.Status = SessionStatusActive

	// 调用认证成功回调
	if sch.callbacks.OnAuthenticationSuccess != nil {
		if err := sch.callbacks.OnAuthenticationSuccess(session); err != nil {
			return WrapError(err, ErrCodeConnectionFailed, "认证成功回调失败", deviceID, "auth")
		}
	}

	sch.logInfo("认证成功: 设备=%s", deviceID)
	return nil
}

// handleHeartbeatMessage 处理心跳消息
func (sch *ServerConnectionHandler[T]) handleHeartbeatMessage(session *ClientSession, message Message[T]) error {
	deviceID := session.DeviceID

	// 更新心跳时间
	sch.sessionManager.UpdateHeartbeat(deviceID)

	// 发送心跳响应
	heartbeatResponse := Message[T]{
		ID:        fmt.Sprintf("heartbeat_resp_%d", time.Now().UnixNano()),
		Type:      MessageTypeHeartbeat,
		Timestamp: time.Now(),
		Metadata: MessageMetadata{
			Priority: PriorityNormal,
		},
	}

	return sch.SendMessage(session, heartbeatResponse)
}

// handleConnectionTimeout 处理连接超时
func (sch *ServerConnectionHandler[T]) handleConnectionTimeout(session *ClientSession, timeoutType TimeoutType) error {
	deviceID := session.DeviceID

	// 调用超时回调
	if sch.callbacks.OnTimeout != nil {
		if err := sch.callbacks.OnTimeout(session, timeoutType); err != nil {
			sch.logError("超时回调失败: 设备=%s, 类型=%s, 错误=%v", deviceID, timeoutType.String(), err)
		}
	}

	// 关闭会话
	sch.cleanup(session)

	sch.logWarn("连接超时: 设备=%s, 类型=%s", deviceID, timeoutType.String())
	return nil
}

// handleAuthTimeout 处理认证超时
func (sch *ServerConnectionHandler[T]) handleAuthTimeout(session *ClientSession) error {
	deviceID := session.DeviceID

	// 调用认证失败回调
	if sch.callbacks.OnAuthenticationFailed != nil {
		authErr := NewBluetoothError(ErrCodeTimeout, "认证超时")
		sch.callbacks.OnAuthenticationFailed(session, authErr)
	}

	// 关闭会话
	sch.cleanup(session)

	sch.logWarn("认证超时: 设备=%s", deviceID)
	return nil
}

// checkRateLimit 检查速率限制
func (sch *ServerConnectionHandler[T]) checkRateLimit(session *ClientSession) error {
	// 这里简化实现，实际应该实现令牌桶或滑动窗口算法
	now := time.Now()
	lastRequest, exists := session.Metadata["last_request_time"].(time.Time)
	if exists && now.Sub(lastRequest) < time.Second/time.Duration(sch.config.MaxRequestsPerSec) {
		return NewBluetoothError(ErrCodeResourceBusy, "请求频率过高")
	}
	session.Metadata["last_request_time"] = now
	return nil
}

// encryptMessage 加密消息
func (sch *ServerConnectionHandler[T]) encryptMessage(message *Message[T], session *ClientSession) error {
	if sch.securityManager == nil {
		return NewBluetoothError(ErrCodeNotSupported, "安全管理器未配置")
	}

	// 这里简化处理，实际应该序列化消息载荷并加密
	data := []byte(fmt.Sprintf("%+v", message.Payload))
	encryptedData, err := sch.securityManager.Encrypt(data, session.DeviceID)
	if err != nil {
		return err
	}

	// 将加密数据存储在元数据中
	message.Metadata.Encrypted = true
	message.Metadata.Compressed = false // 简化处理
	_ = encryptedData                   // 避免未使用变量警告

	return nil
}

// decryptMessage 解密消息
func (sch *ServerConnectionHandler[T]) decryptMessage(message *Message[T], session *ClientSession) error {
	if sch.securityManager == nil {
		return NewBluetoothError(ErrCodeNotSupported, "安全管理器未配置")
	}

	if !message.Metadata.Encrypted {
		return nil // 消息未加密
	}

	// 这里简化处理，实际应该从消息中提取加密数据并解密
	data := []byte(fmt.Sprintf("%+v", message.Payload))
	decryptedData, err := sch.securityManager.Decrypt(data, session.DeviceID)
	if err != nil {
		return err
	}

	// 反序列化解密数据到消息载荷
	_ = decryptedData // 避免未使用变量警告

	return nil
}

// cleanup 清理会话资源
func (sch *ServerConnectionHandler[T]) cleanup(session *ClientSession) {
	deviceID := session.DeviceID

	// 清理超时条目
	sch.timeoutManager.ClearTimeoutsByDevice(deviceID)

	// 释放资源
	if resourceID, exists := session.Metadata["resource_id"].(string); exists {
		sch.resourceManager.ReleaseResource(resourceID)
	}

	// 关闭会话
	sch.sessionManager.CloseSession(deviceID)
}

// 日志方法

func (sch *ServerConnectionHandler[T]) logInfo(format string, args ...interface{}) {
	if sch.config.EnableLogging && (sch.config.LogLevel == "info" || sch.config.LogLevel == "debug") {
		fmt.Printf("[INFO] "+format+"\n", args...)
	}
}

func (sch *ServerConnectionHandler[T]) logWarn(format string, args ...interface{}) {
	if sch.config.EnableLogging {
		fmt.Printf("[WARN] "+format+"\n", args...)
	}
}

func (sch *ServerConnectionHandler[T]) logError(format string, args ...interface{}) {
	if sch.config.EnableLogging {
		fmt.Printf("[ERROR] "+format+"\n", args...)
	}
}

func (sch *ServerConnectionHandler[T]) logDebug(format string, args ...interface{}) {
	if sch.config.EnableLogging && sch.config.LogLevel == "debug" {
		fmt.Printf("[DEBUG] "+format+"\n", args...)
	}
}
