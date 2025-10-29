package bluetooth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BluetoothAdapter 蓝牙适配器接口（避免循环导入）
type BluetoothAdapter interface {
	Initialize(ctx context.Context) error
	Shutdown(ctx context.Context) error
	IsEnabled() bool
	Enable(ctx context.Context) error
	Listen(serviceUUID string) error
	StopListen() error
	AcceptConnection(ctx context.Context) (Connection, error)
	SetDiscoverable(discoverable bool, timeout time.Duration) error
	GetLocalInfo() AdapterInfo
}

// AdapterInfo 适配器信息（避免循环导入）
type AdapterInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

// DefaultBluetoothServer 默认蓝牙服务端实现，支持泛型消息类型
type DefaultBluetoothServer[T any] struct {
	config            ServerConfig
	adapter           BluetoothAdapter
	connectionHandler ConnectionHandler[T]
	connections       map[string]Connection
	status            ComponentStatus
	serviceUUID       string
	receiveChan       chan Message[T]
	connectionChan    chan Connection
	mu                sync.RWMutex
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	callbackManager   *CallbackManager
}

// NewBluetoothServer 创建新的蓝牙服务端实例
func NewBluetoothServer[T any](config ServerConfig, adapter BluetoothAdapter) *DefaultBluetoothServer[T] {
	ctx, cancel := context.WithCancel(context.Background())

	return &DefaultBluetoothServer[T]{
		config:          config,
		adapter:         adapter,
		connections:     make(map[string]Connection),
		status:          StatusStopped,
		receiveChan:     make(chan Message[T], DefaultMessageQueueSize),
		connectionChan:  make(chan Connection, config.MaxConnections),
		ctx:             ctx,
		cancel:          cancel,
		callbackManager: NewCallbackManager(),
	}
}

// Start 启动蓝牙服务端
func (s *DefaultBluetoothServer[T]) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == StatusRunning {
		return NewBluetoothError(ErrCodeAlreadyExists, "服务端已在运行中")
	}

	s.status = StatusStarting

	// 初始化适配器
	if err := s.adapter.Initialize(ctx); err != nil {
		s.status = StatusError
		return WrapError(err, ErrCodeConnectionFailed, "初始化蓝牙适配器失败", "", "start")
	}

	// 启用蓝牙适配器
	if !s.adapter.IsEnabled() {
		if err := s.adapter.Enable(ctx); err != nil {
			s.status = StatusError
			return WrapError(err, ErrCodeConnectionFailed, "启用蓝牙适配器失败", "", "start")
		}
	}

	// 设置可发现性
	if err := s.adapter.SetDiscoverable(true, 0); err != nil {
		s.status = StatusError
		return WrapError(err, ErrCodeConnectionFailed, "设置蓝牙可发现性失败", "", "start")
	}

	s.status = StatusRunning

	// 启动消息处理器
	s.wg.Add(1)
	go s.messageProcessor()

	// 启动连接管理器
	s.wg.Add(1)
	go s.connectionManager()

	return nil
}

// Stop 停止蓝牙服务端
func (s *DefaultBluetoothServer[T]) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == StatusStopped {
		return nil
	}

	s.status = StatusStopping

	// 停止监听
	if err := s.adapter.StopListen(); err != nil {
		// 记录错误但继续停止流程
		s.callbackManager.NotifyError(WrapError(err, ErrCodeGeneral, "停止监听失败", "", "stop"))
	}

	// 断开所有连接
	for deviceID, conn := range s.connections {
		if s.connectionHandler != nil {
			s.connectionHandler.OnDisconnected(conn, NewBluetoothError(ErrCodeGeneral, "服务端停止"))
		}
		if err := conn.Close(); err != nil {
			s.callbackManager.NotifyError(WrapError(err, ErrCodeGeneral, "关闭连接失败", deviceID, "stop"))
		}
	}
	s.connections = make(map[string]Connection)

	// 关闭适配器
	if err := s.adapter.Shutdown(ctx); err != nil {
		s.callbackManager.NotifyError(WrapError(err, ErrCodeGeneral, "关闭适配器失败", "", "stop"))
	}

	// 取消上下文并等待goroutine结束
	s.cancel()
	s.wg.Wait()

	// 关闭通道
	close(s.receiveChan)
	close(s.connectionChan)

	s.status = StatusStopped
	return nil
}

// Send 发送数据到指定设备
func (s *DefaultBluetoothServer[T]) Send(ctx context.Context, deviceID string, data T) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.status != StatusRunning {
		return NewBluetoothErrorWithDevice(ErrCodeNotSupported, "服务端未运行", deviceID, "send")
	}

	conn, exists := s.connections[deviceID]
	if !exists {
		return NewBluetoothErrorWithDevice(ErrCodeDeviceNotFound, "设备连接不存在", deviceID, "send")
	}

	if !conn.IsActive() {
		return NewBluetoothErrorWithDevice(ErrCodeDisconnected, "设备连接已断开", deviceID, "send")
	}

	// 创建消息
	message := Message[T]{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		Type:      MessageTypeData,
		Payload:   data,
		Timestamp: time.Now(),
		Metadata: MessageMetadata{
			SenderID:   s.adapter.GetLocalInfo().ID,
			ReceiverID: deviceID,
			Priority:   PriorityNormal,
			TTL:        DefaultSessionTimeout,
			Encrypted:  s.config.EnableEncryption,
		},
	}

	// 序列化消息（这里简化处理，实际应该使用JSON或其他序列化方式）
	messageData := []byte(fmt.Sprintf("%+v", message))

	// 发送数据
	if err := conn.Send(messageData); err != nil {
		return WrapError(err, ErrCodeConnectionFailed, "发送数据失败", deviceID, "send")
	}

	return nil
}

// Receive 接收数据通道
func (s *DefaultBluetoothServer[T]) Receive(ctx context.Context) <-chan Message[T] {
	return s.receiveChan
}

// GetStatus 获取组件状态
func (s *DefaultBluetoothServer[T]) GetStatus() ComponentStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// Listen 监听指定的服务UUID
func (s *DefaultBluetoothServer[T]) Listen(serviceUUID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status != StatusRunning {
		return NewBluetoothError(ErrCodeNotSupported, "服务端未运行")
	}

	s.serviceUUID = serviceUUID

	// 开始监听连接请求
	if err := s.adapter.Listen(serviceUUID); err != nil {
		return WrapError(err, ErrCodeConnectionFailed, "开始监听失败", "", "listen")
	}

	// 启动连接接受器
	s.wg.Add(1)
	go s.connectionAcceptor()

	return nil
}

// AcceptConnections 接受连接请求
func (s *DefaultBluetoothServer[T]) AcceptConnections() <-chan Connection {
	return s.connectionChan
}

// SetConnectionHandler 设置连接处理器
func (s *DefaultBluetoothServer[T]) SetConnectionHandler(handler ConnectionHandler[T]) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connectionHandler = handler
}

// GetConnections 获取当前连接列表
func (s *DefaultBluetoothServer[T]) GetConnections() []Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()

	connections := make([]Connection, 0, len(s.connections))
	for _, conn := range s.connections {
		connections = append(connections, conn)
	}
	return connections
}

// GetConnectionCount 获取当前连接数
func (s *DefaultBluetoothServer[T]) GetConnectionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.connections)
}

// DisconnectDevice 断开指定设备的连接
func (s *DefaultBluetoothServer[T]) DisconnectDevice(deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conn, exists := s.connections[deviceID]
	if !exists {
		return NewBluetoothErrorWithDevice(ErrCodeDeviceNotFound, "设备连接不存在", deviceID, "disconnect")
	}

	// 通知连接处理器
	if s.connectionHandler != nil {
		s.connectionHandler.OnDisconnected(conn, NewBluetoothError(ErrCodeGeneral, "主动断开连接"))
	}

	// 关闭连接
	if err := conn.Close(); err != nil {
		return WrapError(err, ErrCodeConnectionFailed, "关闭连接失败", deviceID, "disconnect")
	}

	// 从连接映射中移除
	delete(s.connections, deviceID)

	return nil
}

// RegisterCallback 注册回调函数
func (s *DefaultBluetoothServer[T]) RegisterCallback(callback interface{}) {
	switch cb := callback.(type) {
	case DeviceCallback:
		s.callbackManager.RegisterDeviceCallback(cb)
	case HealthCallback:
		s.callbackManager.RegisterHealthCallback(cb)
	case ErrorCallback:
		s.callbackManager.RegisterErrorCallback(cb)
	}
}

// 内部方法

// connectionAcceptor 连接接受器goroutine
func (s *DefaultBluetoothServer[T]) connectionAcceptor() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// 创建带超时的上下文
			acceptCtx, cancel := context.WithTimeout(s.ctx, s.config.AcceptTimeout)

			// 接受连接
			conn, err := s.adapter.AcceptConnection(acceptCtx)
			cancel()

			if err != nil {
				// 检查是否是上下文取消
				if s.ctx.Err() != nil {
					return
				}
				// 其他错误，记录并继续
				s.callbackManager.NotifyError(WrapError(err, ErrCodeConnectionFailed, "接受连接失败", "", "accept"))
				continue
			}

			// 处理新连接
			s.handleNewConnection(conn)
		}
	}
}

// handleNewConnection 处理新连接
func (s *DefaultBluetoothServer[T]) handleNewConnection(conn Connection) {
	s.mu.Lock()
	defer s.mu.Unlock()

	deviceID := conn.DeviceID()

	// 检查连接数限制
	if len(s.connections) >= s.config.MaxConnections {
		s.callbackManager.NotifyError(NewBluetoothErrorWithDevice(
			ErrCodeResourceBusy, "达到最大连接数限制", deviceID, "accept"))
		conn.Close()
		return
	}

	// 检查是否已存在连接
	if existingConn, exists := s.connections[deviceID]; exists {
		if existingConn.IsActive() {
			s.callbackManager.NotifyError(NewBluetoothErrorWithDevice(
				ErrCodeAlreadyExists, "设备已连接", deviceID, "accept"))
			conn.Close()
			return
		}
		// 清理无效连接
		delete(s.connections, deviceID)
	}

	// 添加到连接映射
	s.connections[deviceID] = conn

	// 通知连接处理器
	if s.connectionHandler != nil {
		if err := s.connectionHandler.OnConnected(conn); err != nil {
			s.callbackManager.NotifyError(WrapError(err, ErrCodeConnectionFailed, "连接处理器处理失败", deviceID, "accept"))
			conn.Close()
			delete(s.connections, deviceID)
			return
		}
	}

	// 发送到连接通道
	select {
	case s.connectionChan <- conn:
	default:
		// 连接通道满了，记录错误
		s.callbackManager.NotifyError(NewBluetoothErrorWithDevice(
			ErrCodeResourceBusy, "连接通道已满", deviceID, "accept"))
	}

	// 启动连接监控
	s.wg.Add(1)
	go s.monitorConnection(conn)
}

// monitorConnection 监控连接状态
func (s *DefaultBluetoothServer[T]) monitorConnection(conn Connection) {
	defer s.wg.Done()

	receiveChan := conn.Receive()

	for {
		select {
		case <-s.ctx.Done():
			return
		case data, ok := <-receiveChan:
			if !ok {
				// 连接已关闭
				s.handleConnectionClosed(conn)
				return
			}

			// 处理接收到的数据
			s.handleReceivedData(conn, data)
		}
	}
}

// handleConnectionClosed 处理连接关闭
func (s *DefaultBluetoothServer[T]) handleConnectionClosed(conn Connection) {
	s.mu.Lock()
	defer s.mu.Unlock()

	deviceID := conn.DeviceID()

	// 从连接映射中移除
	delete(s.connections, deviceID)

	// 通知连接处理器
	if s.connectionHandler != nil {
		s.connectionHandler.OnDisconnected(conn, NewBluetoothError(ErrCodeDisconnected, "连接已断开"))
	}
}

// handleReceivedData 处理接收到的数据
func (s *DefaultBluetoothServer[T]) handleReceivedData(conn Connection, data []byte) {
	// 这里简化处理，实际应该反序列化消息
	// 创建一个模拟消息（实际实现中应该从data反序列化）
	var zeroValue T
	message := Message[T]{
		ID:        fmt.Sprintf("recv_%d", time.Now().UnixNano()),
		Type:      MessageTypeData,
		Payload:   zeroValue, // 实际应该从data反序列化
		Timestamp: time.Now(),
		Metadata: MessageMetadata{
			SenderID:   conn.DeviceID(),
			ReceiverID: s.adapter.GetLocalInfo().ID,
			Priority:   PriorityNormal,
		},
	}

	// 通知连接处理器
	if s.connectionHandler != nil {
		if err := s.connectionHandler.OnMessage(conn, message); err != nil {
			s.callbackManager.NotifyError(WrapError(err, ErrCodeProtocolError, "消息处理失败", conn.DeviceID(), "receive"))
			return
		}
	}

	// 发送到接收通道
	select {
	case s.receiveChan <- message:
	default:
		// 接收通道满了，记录错误
		s.callbackManager.NotifyError(NewBluetoothErrorWithDevice(
			ErrCodeResourceBusy, "接收通道已满", conn.DeviceID(), "receive"))
	}
}

// messageProcessor 消息处理器goroutine
func (s *DefaultBluetoothServer[T]) messageProcessor() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		case message := <-s.receiveChan:
			// 处理消息（这里可以添加消息路由、过滤等逻辑）
			_ = message // 避免未使用变量警告
		}
	}
}

// connectionManager 连接管理器goroutine
func (s *DefaultBluetoothServer[T]) connectionManager() {
	defer s.wg.Done()

	// 定期清理无效连接
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanupInactiveConnections()
		}
	}
}

// cleanupInactiveConnections 清理无效连接
func (s *DefaultBluetoothServer[T]) cleanupInactiveConnections() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for deviceID, conn := range s.connections {
		if !conn.IsActive() {
			// 通知连接处理器
			if s.connectionHandler != nil {
				s.connectionHandler.OnDisconnected(conn, NewBluetoothError(ErrCodeDisconnected, "连接不活跃"))
			}

			// 关闭连接
			conn.Close()

			// 从映射中移除
			delete(s.connections, deviceID)
		}
	}
}
