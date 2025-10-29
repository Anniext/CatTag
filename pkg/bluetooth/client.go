package bluetooth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DefaultBluetoothClient 默认蓝牙客户端实现，支持泛型消息类型
type DefaultBluetoothClient[T any] struct {
	config            ClientConfig
	adapter           BluetoothAdapter
	connectionHandler ConnectionHandler[T]
	connections       map[string]Connection
	status            ComponentStatus
	receiveChan       chan Message[T]
	mu                sync.RWMutex
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	callbackManager   *CallbackManager
	reconnectManager  *ReconnectManager
}

// NewBluetoothClient 创建新的蓝牙客户端实例
func NewBluetoothClient[T any](config ClientConfig, adapter BluetoothAdapter) *DefaultBluetoothClient[T] {
	ctx, cancel := context.WithCancel(context.Background())

	return &DefaultBluetoothClient[T]{
		config:           config,
		adapter:          adapter,
		connections:      make(map[string]Connection),
		status:           StatusStopped,
		receiveChan:      make(chan Message[T], DefaultMessageQueueSize),
		ctx:              ctx,
		cancel:           cancel,
		callbackManager:  NewCallbackManager(),
		reconnectManager: NewReconnectManager(config),
	}
}

// Start 启动蓝牙客户端
func (c *DefaultBluetoothClient[T]) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status == StatusRunning {
		return NewBluetoothError(ErrCodeAlreadyExists, "客户端已在运行中")
	}

	c.status = StatusStarting

	// 初始化适配器
	if err := c.adapter.Initialize(ctx); err != nil {
		c.status = StatusError
		return WrapError(err, ErrCodeConnectionFailed, "初始化蓝牙适配器失败", "", "start")
	}

	// 启用蓝牙适配器
	if !c.adapter.IsEnabled() {
		if err := c.adapter.Enable(ctx); err != nil {
			c.status = StatusError
			return WrapError(err, ErrCodeConnectionFailed, "启用蓝牙适配器失败", "", "start")
		}
	}

	c.status = StatusRunning

	// 启动消息处理器
	c.wg.Add(1)
	go c.messageProcessor()

	// 启动连接管理器
	c.wg.Add(1)
	go c.connectionManager()

	// 如果启用自动重连，启动重连管理器
	if c.config.AutoReconnect {
		c.reconnectManager.Start(c.ctx, c)
	}

	return nil
}

// Stop 停止蓝牙客户端
func (c *DefaultBluetoothClient[T]) Stop(ctx context.Context) error {
	// 检查状态并设置停止状态
	func() {
		c.mu.Lock()
		defer c.mu.Unlock()

		if c.status == StatusStopped {
			return
		}
		c.status = StatusStopping
	}()

	// 停止重连管理器
	if c.config.AutoReconnect {
		c.reconnectManager.Stop()
	}

	// 断开所有连接
	func() {
		c.mu.Lock()
		defer c.mu.Unlock()

		for deviceID, conn := range c.connections {
			if c.connectionHandler != nil {
				c.connectionHandler.OnDisconnected(conn, NewBluetoothError(ErrCodeGeneral, "客户端停止"))
			}
			if err := conn.Close(); err != nil {
				c.callbackManager.NotifyError(WrapError(err, ErrCodeGeneral, "关闭连接失败", deviceID, "stop"))
			}
		}
		c.connections = make(map[string]Connection)
	}()

	// 关闭适配器
	if err := c.adapter.Shutdown(ctx); err != nil {
		c.callbackManager.NotifyError(WrapError(err, ErrCodeGeneral, "关闭适配器失败", "", "stop"))
	}

	// 取消上下文并等待goroutine结束
	c.cancel()
	c.wg.Wait()

	// 关闭通道并设置最终状态
	func() {
		c.mu.Lock()
		defer c.mu.Unlock()

		close(c.receiveChan)
		c.status = StatusStopped
	}()

	return nil
}

// Send 发送数据到指定设备
func (c *DefaultBluetoothClient[T]) Send(ctx context.Context, deviceID string, data T) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.status != StatusRunning {
		return NewBluetoothErrorWithDevice(ErrCodeNotSupported, "客户端未运行", deviceID, "send")
	}

	conn, exists := c.connections[deviceID]
	if !exists {
		return NewBluetoothErrorWithDevice(ErrCodeDeviceNotFound, "设备连接不存在", deviceID, "send")
	}

	if !conn.IsActive() {
		// 如果启用自动重连，尝试重连
		if c.config.AutoReconnect {
			c.reconnectManager.ScheduleReconnect(deviceID)
		}
		return NewBluetoothErrorWithDevice(ErrCodeDisconnected, "设备连接已断开", deviceID, "send")
	}

	// 创建消息
	message := Message[T]{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		Type:      MessageTypeData,
		Payload:   data,
		Timestamp: time.Now(),
		Metadata: MessageMetadata{
			SenderID:   c.adapter.GetLocalInfo().ID,
			ReceiverID: deviceID,
			Priority:   PriorityNormal,
			TTL:        DefaultSessionTimeout,
			Encrypted:  false, // 客户端加密配置需要从安全管理器获取
		},
	}

	// 序列化消息（这里简化处理，实际应该使用JSON或其他序列化方式）
	messageData := []byte(fmt.Sprintf("%+v", message))

	// 发送数据
	if err := conn.Send(messageData); err != nil {
		// 如果发送失败且启用自动重连，安排重连
		if c.config.AutoReconnect {
			c.reconnectManager.ScheduleReconnect(deviceID)
		}
		return WrapError(err, ErrCodeConnectionFailed, "发送数据失败", deviceID, "send")
	}

	return nil
}

// Receive 接收数据通道
func (c *DefaultBluetoothClient[T]) Receive(ctx context.Context) <-chan Message[T] {
	return c.receiveChan
}

// GetStatus 获取组件状态
func (c *DefaultBluetoothClient[T]) GetStatus() ComponentStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// Scan 扫描蓝牙设备
func (c *DefaultBluetoothClient[T]) Scan(ctx context.Context, timeout time.Duration) ([]Device, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.status != StatusRunning {
		return nil, NewBluetoothError(ErrCodeNotSupported, "客户端未运行")
	}

	// 创建带超时的上下文
	scanCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 检查适配器是否支持扫描（类型断言）
	if clientAdapter, ok := c.adapter.(*MockAdapter); ok {
		devices, err := clientAdapter.Scan(scanCtx, timeout)
		if err != nil {
			return nil, err
		}
		// 通知设备发现事件
		for _, device := range devices {
			c.callbackManager.NotifyDeviceEvent(DeviceEventDiscovered, device)
		}
		return devices, nil
	}

	// 如果适配器不支持扫描，返回空列表
	return make([]Device, 0), nil
}

// Connect 连接到指定设备
func (c *DefaultBluetoothClient[T]) Connect(ctx context.Context, deviceID string) (Connection, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status != StatusRunning {
		return nil, NewBluetoothErrorWithDevice(ErrCodeNotSupported, "客户端未运行", deviceID, "connect")
	}

	// 检查是否已存在连接
	if existingConn, exists := c.connections[deviceID]; exists {
		if existingConn.IsActive() {
			return existingConn, nil
		}
		// 清理无效连接
		delete(c.connections, deviceID)
	}

	// 创建带超时的上下文
	connectCtx, cancel := context.WithTimeout(ctx, c.config.ConnectTimeout)
	defer cancel()

	// 尝试连接设备
	conn, err := c.connectToDevice(connectCtx, deviceID)
	if err != nil {
		// 如果启用自动重连，安排重连
		if c.config.AutoReconnect {
			c.reconnectManager.ScheduleReconnect(deviceID)
		}
		return nil, WrapError(err, ErrCodeConnectionFailed, "连接设备失败", deviceID, "connect")
	}

	// 添加到连接映射
	c.connections[deviceID] = conn

	// 通知连接处理器
	if c.connectionHandler != nil {
		if err := c.connectionHandler.OnConnected(conn); err != nil {
			c.callbackManager.NotifyError(WrapError(err, ErrCodeConnectionFailed, "连接处理器处理失败", deviceID, "connect"))
			conn.Close()
			delete(c.connections, deviceID)
			return nil, err
		}
	}

	// 通知设备连接事件
	device := Device{
		ID:       deviceID,
		LastSeen: time.Now(),
	}
	c.callbackManager.NotifyDeviceEvent(DeviceEventConnected, device)

	// 启动连接监控
	c.wg.Add(1)
	go c.monitorConnection(conn)

	return conn, nil
}

// Disconnect 断开与指定设备的连接
func (c *DefaultBluetoothClient[T]) Disconnect(deviceID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, exists := c.connections[deviceID]
	if !exists {
		return NewBluetoothErrorWithDevice(ErrCodeDeviceNotFound, "设备连接不存在", deviceID, "disconnect")
	}

	// 停止该设备的自动重连
	if c.config.AutoReconnect {
		c.reconnectManager.StopReconnect(deviceID)
	}

	// 通知连接处理器
	if c.connectionHandler != nil {
		c.connectionHandler.OnDisconnected(conn, NewBluetoothError(ErrCodeGeneral, "主动断开连接"))
	}

	// 关闭连接
	if err := conn.Close(); err != nil {
		return WrapError(err, ErrCodeConnectionFailed, "关闭连接失败", deviceID, "disconnect")
	}

	// 从连接映射中移除
	delete(c.connections, deviceID)

	// 通知设备断开事件
	device := Device{
		ID:       deviceID,
		LastSeen: time.Now(),
	}
	c.callbackManager.NotifyDeviceEvent(DeviceEventDisconnected, device)

	return nil
}

// GetConnections 获取当前连接列表
func (c *DefaultBluetoothClient[T]) GetConnections() []Connection {
	c.mu.RLock()
	defer c.mu.RUnlock()

	connections := make([]Connection, 0, len(c.connections))
	for _, conn := range c.connections {
		connections = append(connections, conn)
	}
	return connections
}

// GetConnectionCount 获取当前连接数
func (c *DefaultBluetoothClient[T]) GetConnectionCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.connections)
}

// SetConnectionHandler 设置连接处理器
func (c *DefaultBluetoothClient[T]) SetConnectionHandler(handler ConnectionHandler[T]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connectionHandler = handler
}

// RegisterCallback 注册回调函数
func (c *DefaultBluetoothClient[T]) RegisterCallback(callback any) {
	switch cb := callback.(type) {
	case DeviceCallback:
		c.callbackManager.RegisterDeviceCallback(cb)
	case HealthCallback:
		c.callbackManager.RegisterHealthCallback(cb)
	case ErrorCallback:
		c.callbackManager.RegisterErrorCallback(cb)
	}
}

// 内部方法

// connectToDevice 连接到指定设备的内部实现
func (c *DefaultBluetoothClient[T]) connectToDevice(ctx context.Context, deviceID string) (Connection, error) {
	// 检查适配器是否支持客户端连接（类型断言）
	if clientAdapter, ok := c.adapter.(*MockAdapter); ok {
		return clientAdapter.Connect(ctx, deviceID)
	}

	// 如果适配器不支持客户端连接，返回模拟连接
	// 模拟连接延迟
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(100 * time.Millisecond):
		// 返回模拟连接
		return NewMockConnection(deviceID), nil
	}
}

// monitorConnection 监控连接状态
func (c *DefaultBluetoothClient[T]) monitorConnection(conn Connection) {
	defer c.wg.Done()

	receiveChan := conn.Receive()

	for {
		select {
		case <-c.ctx.Done():
			return
		case data, ok := <-receiveChan:
			if !ok {
				// 连接已关闭
				c.handleConnectionClosed(conn)
				return
			}

			// 处理接收到的数据
			c.handleReceivedData(conn, data)
		}
	}
}

// handleConnectionClosed 处理连接关闭
func (c *DefaultBluetoothClient[T]) handleConnectionClosed(conn Connection) {
	deviceID := conn.DeviceID()

	// 使用独立的锁作用域来避免死锁
	func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		// 从连接映射中移除
		delete(c.connections, deviceID)
	}()

	// 如果启用自动重连，安排重连
	if c.config.AutoReconnect {
		c.reconnectManager.ScheduleReconnect(deviceID)
	}

	// 通知连接处理器
	if c.connectionHandler != nil {
		c.connectionHandler.OnDisconnected(conn, NewBluetoothError(ErrCodeDisconnected, "连接已断开"))
	}

	// 通知设备断开事件
	device := Device{
		ID:       deviceID,
		LastSeen: time.Now(),
	}
	c.callbackManager.NotifyDeviceEvent(DeviceEventDisconnected, device)
}

// handleReceivedData 处理接收到的数据
func (c *DefaultBluetoothClient[T]) handleReceivedData(conn Connection, data []byte) {
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
			ReceiverID: c.adapter.GetLocalInfo().ID,
			Priority:   PriorityNormal,
		},
	}

	// 通知连接处理器
	if c.connectionHandler != nil {
		if err := c.connectionHandler.OnMessage(conn, message); err != nil {
			c.callbackManager.NotifyError(WrapError(err, ErrCodeProtocolError, "消息处理失败", conn.DeviceID(), "receive"))
			return
		}
	}

	// 发送到接收通道
	select {
	case c.receiveChan <- message:
	default:
		// 接收通道满了，记录错误
		c.callbackManager.NotifyError(NewBluetoothErrorWithDevice(
			ErrCodeResourceBusy, "接收通道已满", conn.DeviceID(), "receive"))
	}
}

// messageProcessor 消息处理器goroutine
func (c *DefaultBluetoothClient[T]) messageProcessor() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case message := <-c.receiveChan:
			// 处理消息（这里可以添加消息路由、过滤等逻辑）
			_ = message // 避免未使用变量警告
		}
	}
}

// connectionManager 连接管理器goroutine
func (c *DefaultBluetoothClient[T]) connectionManager() {
	defer c.wg.Done()

	// 定期清理无效连接和发送保活消息
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.cleanupInactiveConnections()
			if c.config.KeepAlive {
				c.sendKeepAliveMessages()
			}
		}
	}
}

// cleanupInactiveConnections 清理无效连接
func (c *DefaultBluetoothClient[T]) cleanupInactiveConnections() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for deviceID, conn := range c.connections {
		if !conn.IsActive() {
			// 通知连接处理器
			if c.connectionHandler != nil {
				c.connectionHandler.OnDisconnected(conn, NewBluetoothError(ErrCodeDisconnected, "连接不活跃"))
			}

			// 关闭连接
			conn.Close()

			// 从映射中移除
			delete(c.connections, deviceID)

			// 如果启用自动重连，安排重连
			if c.config.AutoReconnect {
				c.reconnectManager.ScheduleReconnect(deviceID)
			}

			// 通知设备断开事件
			device := Device{
				ID:       deviceID,
				LastSeen: time.Now(),
			}
			c.callbackManager.NotifyDeviceEvent(DeviceEventDisconnected, device)
		}
	}
}

// sendKeepAliveMessages 发送保活消息
func (c *DefaultBluetoothClient[T]) sendKeepAliveMessages() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for deviceID, conn := range c.connections {
		if conn.IsActive() {
			// 发送心跳消息
			heartbeatData := []byte("heartbeat")
			if err := conn.Send(heartbeatData); err != nil {
				c.callbackManager.NotifyError(WrapError(err, ErrCodeConnectionFailed, "发送心跳失败", deviceID, "keepalive"))
			}
		}
	}
}
