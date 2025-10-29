package bluetooth

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// DefaultBluetoothComponent 默认蓝牙组件实现，集成所有子组件
type DefaultBluetoothComponent[T any] struct {
	// 配置
	config BluetoothConfig

	// 核心组件
	server          BluetoothServer[T]
	client          BluetoothClient[T]
	deviceManager   DeviceManager
	connectionPool  ConnectionPool
	healthChecker   HealthChecker
	securityManager SecurityManager
	messageHandler  MessageHandler[T]

	// 内部组件
	adapter  BluetoothAdapter
	eventBus *EventBus
	logger   *slog.Logger

	// 状态管理
	status      ComponentStatus
	receiveChan chan Message[T]
	eventChan   chan ComponentEvent
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup

	// 组件协调
	componentStates map[string]ComponentStatus
	startupOrder    []string
	shutdownOrder   []string
}

// ComponentEvent 组件事件
type ComponentEvent struct {
	Type      EventType       `json:"type"`      // 事件类型
	Component string          `json:"component"` // 组件名称
	Status    ComponentStatus `json:"status"`    // 组件状态
	Message   string          `json:"message"`   // 事件消息
	Timestamp time.Time       `json:"timestamp"` // 事件时间戳
	Data      interface{}     `json:"data"`      // 事件数据
}

// EventType 事件类型
type EventType int

const (
	EventTypeStartup EventType = iota
	EventTypeShutdown
	EventTypeError
	EventTypeConnection
	EventTypeMessage
	EventTypeHealth
	EventTypeSecurity
)

// EventBus 事件总线，用于组件间通信
type EventBus struct {
	subscribers map[EventType][]chan ComponentEvent
	mu          sync.RWMutex
}

// NewEventBus 创建新的事件总线
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[EventType][]chan ComponentEvent),
	}
}

// Subscribe 订阅事件
func (eb *EventBus) Subscribe(eventType EventType) <-chan ComponentEvent {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	ch := make(chan ComponentEvent, 100)
	eb.subscribers[eventType] = append(eb.subscribers[eventType], ch)
	return ch
}

// Publish 发布事件
func (eb *EventBus) Publish(event ComponentEvent) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	if subscribers, exists := eb.subscribers[event.Type]; exists {
		for _, ch := range subscribers {
			select {
			case ch <- event:
			default:
				// 如果通道满了，跳过这个订阅者
			}
		}
	}
}

// NewBluetoothComponent 创建新的蓝牙组件实例
func NewBluetoothComponent[T any](config BluetoothConfig) (*DefaultBluetoothComponent[T], error) {
	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 创建日志记录器
	logger := slog.Default().With("component", "bluetooth")

	// 创建事件总线
	eventBus := NewEventBus()

	// 创建蓝牙适配器 - 使用模拟适配器进行演示
	bluetoothAdapter := NewMockAdapter()

	// 创建设备管理器 - 使用默认实现
	deviceMgr := NewDefaultDeviceManager(bluetoothAdapter)

	// 创建安全管理器 - 使用默认实现
	securityMgr := NewDefaultSecurityManager(config.SecurityConfig)

	// 创建连接池
	connPool := NewConnectionPool(config.PoolConfig.MaxConnections)

	// 创建健康检查器
	healthChecker := NewDefaultHealthChecker(connPool)

	// 创建消息处理器
	messageHandler := NewConcurrentMessageHandler[T](
		DefaultMessageQueueSize, // queueSize
		DefaultWorkerPoolSize,   // workerCount
		100.0,                   // maxRate (消息/秒)
		10,                      // maxConcurrency
	)

	// 创建服务端和客户端
	server := NewBluetoothServer[T](config.ServerConfig, bluetoothAdapter)
	client := NewBluetoothClient[T](config.ClientConfig, bluetoothAdapter)

	component := &DefaultBluetoothComponent[T]{
		config:          config,
		server:          server,
		client:          client,
		deviceManager:   deviceMgr,
		connectionPool:  connPool,
		healthChecker:   healthChecker,
		securityManager: securityMgr,
		messageHandler:  messageHandler,
		adapter:         bluetoothAdapter,
		eventBus:        eventBus,
		logger:          logger,
		status:          StatusStopped,
		receiveChan:     make(chan Message[T], DefaultMessageQueueSize),
		eventChan:       make(chan ComponentEvent, 100),
		ctx:             ctx,
		cancel:          cancel,
		componentStates: make(map[string]ComponentStatus),
		startupOrder:    []string{"adapter", "security", "device", "pool", "health", "message", "server", "client"},
		shutdownOrder:   []string{"client", "server", "message", "health", "pool", "device", "security", "adapter"},
	}

	// 初始化组件状态
	for _, name := range component.startupOrder {
		component.componentStates[name] = StatusStopped
	}

	return component, nil
}

// Start 启动蓝牙组件
func (c *DefaultBluetoothComponent[T]) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status == StatusRunning {
		return NewBluetoothError(ErrCodeAlreadyExists, "组件已在运行中")
	}

	c.logger.Info("开始启动蓝牙组件")
	c.status = StatusStarting

	// 发布启动事件
	c.publishEvent(ComponentEvent{
		Type:      EventTypeStartup,
		Component: "main",
		Status:    StatusStarting,
		Message:   "开始启动蓝牙组件",
		Timestamp: time.Now(),
	})

	// 按顺序启动各个组件
	for _, componentName := range c.startupOrder {
		if err := c.startComponent(ctx, componentName); err != nil {
			c.logger.Error("启动组件失败", "component", componentName, "error", err)
			c.status = StatusError
			return fmt.Errorf("启动组件 %s 失败: %w", componentName, err)
		}
		c.componentStates[componentName] = StatusRunning
		c.logger.Info("组件启动成功", "component", componentName)
	}

	// 启动事件处理器
	c.wg.Add(1)
	go c.eventHandler()

	// 启动消息路由器
	c.wg.Add(1)
	go c.messageRouter()

	c.status = StatusRunning
	c.logger.Info("蓝牙组件启动完成")

	// 发布启动完成事件
	c.publishEvent(ComponentEvent{
		Type:      EventTypeStartup,
		Component: "main",
		Status:    StatusRunning,
		Message:   "蓝牙组件启动完成",
		Timestamp: time.Now(),
	})

	return nil
}

// Stop 停止蓝牙组件
func (c *DefaultBluetoothComponent[T]) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status == StatusStopped {
		return nil
	}

	c.logger.Info("开始停止蓝牙组件")
	c.status = StatusStopping

	// 发布停止事件
	c.publishEvent(ComponentEvent{
		Type:      EventTypeShutdown,
		Component: "main",
		Status:    StatusStopping,
		Message:   "开始停止蓝牙组件",
		Timestamp: time.Now(),
	})

	// 取消上下文
	c.cancel()

	// 按逆序停止各个组件
	for _, componentName := range c.shutdownOrder {
		if err := c.stopComponent(ctx, componentName); err != nil {
			c.logger.Error("停止组件失败", "component", componentName, "error", err)
		}
		c.componentStates[componentName] = StatusStopped
		c.logger.Info("组件停止成功", "component", componentName)
	}

	// 等待所有 goroutine 完成
	c.wg.Wait()

	// 关闭通道
	close(c.receiveChan)
	close(c.eventChan)

	c.status = StatusStopped
	c.logger.Info("蓝牙组件停止完成")

	return nil
}

// Send 发送数据到指定设备
func (c *DefaultBluetoothComponent[T]) Send(ctx context.Context, deviceID string, data T) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.status != StatusRunning {
		return NewBluetoothError(ErrCodeResourceBusy, "组件未运行")
	}

	// 检查连接是否存在
	_, err := c.connectionPool.GetConnection(deviceID)
	if err != nil {
		return fmt.Errorf("获取连接失败: %w", err)
	}

	// 通过消息处理器发送
	return c.messageHandler.Send(ctx, deviceID, data)
}

// Receive 接收数据通道
func (c *DefaultBluetoothComponent[T]) Receive(ctx context.Context) <-chan Message[T] {
	return c.receiveChan
}

// GetStatus 获取组件状态
func (c *DefaultBluetoothComponent[T]) GetStatus() ComponentStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// GetComponentStates 获取所有组件状态
func (c *DefaultBluetoothComponent[T]) GetComponentStates() map[string]ComponentStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	states := make(map[string]ComponentStatus)
	for name, status := range c.componentStates {
		states[name] = status
	}
	return states
}

// GetServer 获取服务端实例
func (c *DefaultBluetoothComponent[T]) GetServer() BluetoothServer[T] {
	return c.server
}

// GetClient 获取客户端实例
func (c *DefaultBluetoothComponent[T]) GetClient() BluetoothClient[T] {
	return c.client
}

// GetDeviceManager 获取设备管理器
func (c *DefaultBluetoothComponent[T]) GetDeviceManager() DeviceManager {
	return c.deviceManager
}

// GetConnectionPool 获取连接池
func (c *DefaultBluetoothComponent[T]) GetConnectionPool() ConnectionPool {
	return c.connectionPool
}

// GetHealthChecker 获取健康检查器
func (c *DefaultBluetoothComponent[T]) GetHealthChecker() HealthChecker {
	return c.healthChecker
}

// GetSecurityManager 获取安全管理器
func (c *DefaultBluetoothComponent[T]) GetSecurityManager() SecurityManager {
	return c.securityManager
}

// startComponent 启动指定组件
func (c *DefaultBluetoothComponent[T]) startComponent(ctx context.Context, name string) error {
	switch name {
	case "adapter":
		return c.adapter.Initialize(ctx)
	case "security":
		// 安全管理器可能没有 Initialize 方法，跳过
		return nil
	case "device":
		// 设备管理器可能没有 Initialize 方法，跳过
		return nil
	case "pool":
		// 连接池可能没有 Initialize 方法，跳过
		return nil
	case "health":
		// 健康检查器启动监控
		c.healthChecker.StartMonitoring(ctx, c.config.HealthConfig.CheckInterval)
		return nil
	case "message":
		return c.messageHandler.Start(ctx)
	case "server":
		return c.server.Start(ctx)
	case "client":
		return c.client.Start(ctx)
	default:
		return fmt.Errorf("未知组件: %s", name)
	}
}

// stopComponent 停止指定组件
func (c *DefaultBluetoothComponent[T]) stopComponent(ctx context.Context, name string) error {
	switch name {
	case "adapter":
		return c.adapter.Shutdown(ctx)
	case "security":
		// 安全管理器可能没有 Shutdown 方法，跳过
		return nil
	case "device":
		// 设备管理器可能没有 Shutdown 方法，跳过
		return nil
	case "pool":
		// 连接池可能没有 Shutdown 方法，跳过
		return nil
	case "health":
		// 停止健康检查器监控
		c.healthChecker.StopMonitoring()
		return nil
	case "message":
		return c.messageHandler.Stop()
	case "server":
		return c.server.Stop(ctx)
	case "client":
		return c.client.Stop(ctx)
	default:
		return fmt.Errorf("未知组件: %s", name)
	}
}

// eventHandler 事件处理器
func (c *DefaultBluetoothComponent[T]) eventHandler() {
	defer c.wg.Done()

	for {
		select {
		case event := <-c.eventChan:
			c.handleEvent(event)
		case <-c.ctx.Done():
			return
		}
	}
}

// messageRouter 消息路由器
func (c *DefaultBluetoothComponent[T]) messageRouter() {
	defer c.wg.Done()

	// 订阅各组件的消息
	serverChan := c.server.Receive(c.ctx)
	clientChan := c.client.Receive(c.ctx)

	for {
		select {
		case msg := <-serverChan:
			c.routeMessage(msg)
		case msg := <-clientChan:
			c.routeMessage(msg)
		case <-c.ctx.Done():
			return
		}
	}
}

// routeMessage 路由消息
func (c *DefaultBluetoothComponent[T]) routeMessage(msg Message[T]) {
	select {
	case c.receiveChan <- msg:
	default:
		c.logger.Warn("消息队列已满，丢弃消息", "message_id", msg.ID)
	}
}

// handleEvent 处理事件
func (c *DefaultBluetoothComponent[T]) handleEvent(event ComponentEvent) {
	c.logger.Debug("处理组件事件",
		"type", event.Type,
		"component", event.Component,
		"status", event.Status,
		"message", event.Message)

	// 根据事件类型进行处理
	switch event.Type {
	case EventTypeError:
		c.handleErrorEvent(event)
	case EventTypeConnection:
		c.handleConnectionEvent(event)
	case EventTypeHealth:
		c.handleHealthEvent(event)
	case EventTypeSecurity:
		c.handleSecurityEvent(event)
	}

	// 发布到事件总线
	c.eventBus.Publish(event)
}

// handleErrorEvent 处理错误事件
func (c *DefaultBluetoothComponent[T]) handleErrorEvent(event ComponentEvent) {
	c.logger.Error("组件错误事件",
		"component", event.Component,
		"message", event.Message)

	// 根据错误类型决定是否需要重启组件
	if event.Component != "main" {
		c.componentStates[event.Component] = StatusError
	}
}

// handleConnectionEvent 处理连接事件
func (c *DefaultBluetoothComponent[T]) handleConnectionEvent(event ComponentEvent) {
	c.logger.Info("连接事件",
		"component", event.Component,
		"message", event.Message)
}

// handleHealthEvent 处理健康事件
func (c *DefaultBluetoothComponent[T]) handleHealthEvent(event ComponentEvent) {
	c.logger.Info("健康检查事件",
		"component", event.Component,
		"message", event.Message)
}

// handleSecurityEvent 处理安全事件
func (c *DefaultBluetoothComponent[T]) handleSecurityEvent(event ComponentEvent) {
	c.logger.Warn("安全事件",
		"component", event.Component,
		"message", event.Message)
}

// publishEvent 发布事件
func (c *DefaultBluetoothComponent[T]) publishEvent(event ComponentEvent) {
	select {
	case c.eventChan <- event:
	default:
		c.logger.Warn("事件队列已满，丢弃事件", "event_type", event.Type)
	}
}

// generateComponentMessageID 生成组件消息ID
func generateComponentMessageID() string {
	return fmt.Sprintf("comp_msg_%d", time.Now().UnixNano())
}
