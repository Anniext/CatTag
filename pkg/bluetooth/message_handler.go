package bluetooth

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// MessageHandler 消息处理器接口，支持泛型消息类型
type MessageHandler[T any] interface {
	// Send 发送消息
	Send(ctx context.Context, deviceID string, payload T) error
	// SendWithPriority 发送带优先级的消息
	SendWithPriority(ctx context.Context, deviceID string, payload T, priority Priority) error
	// Receive 接收消息通道
	Receive() <-chan Message[T]
	// RegisterCallback 注册消息回调
	RegisterCallback(messageType MessageType, callback MessageHandlerCallback[T])
	// Start 启动消息处理器
	Start(ctx context.Context) error
	// Stop 停止消息处理器
	Stop() error
	// GetStats 获取消息处理统计信息
	GetStats() MessageHandlerStats
}

// MessageHandlerCallback 消息处理器回调函数类型
type MessageHandlerCallback[T any] func(ctx context.Context, message Message[T]) error

// MessageHandlerStats 消息处理器统计信息
type MessageHandlerStats struct {
	MessagesSent     uint64        `json:"messages_sent"`     // 发送消息数
	MessagesReceived uint64        `json:"messages_received"` // 接收消息数
	MessagesQueued   uint64        `json:"messages_queued"`   // 队列中的消息数
	ProcessingTime   time.Duration `json:"processing_time"`   // 平均处理时间
	ErrorCount       uint64        `json:"error_count"`       // 错误计数
	LastActivity     time.Time     `json:"last_activity"`     // 最后活动时间
}

// MessageQueue 消息队列，利用 Go 1.25 泛型特性
type MessageQueue[T any] struct {
	messages chan Message[T] // 消息通道
	capacity int             // 队列容量
	metrics  *QueueMetrics   // 队列指标
	mu       sync.RWMutex    // 读写锁
	closed   bool            // 是否已关闭
}

// QueueMetrics 队列指标
type QueueMetrics struct {
	Enqueued    uint64        `json:"enqueued"`      // 入队消息数
	Dequeued    uint64        `json:"dequeued"`      // 出队消息数
	Dropped     uint64        `json:"dropped"`       // 丢弃消息数
	MaxSize     int           `json:"max_size"`      // 最大队列大小
	CurrentSize int           `json:"current_size"`  // 当前队列大小
	AvgWaitTime time.Duration `json:"avg_wait_time"` // 平均等待时间
}

// NewMessageQueue 创建新的消息队列
func NewMessageQueue[T any](capacity int) *MessageQueue[T] {
	return &MessageQueue[T]{
		messages: make(chan Message[T], capacity),
		capacity: capacity,
		metrics: &QueueMetrics{
			MaxSize: capacity,
		},
	}
}

// Enqueue 入队消息
func (mq *MessageQueue[T]) Enqueue(message Message[T]) error {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	if mq.closed {
		return NewBluetoothError(ErrCodeGeneral, "消息队列已关闭")
	}

	select {
	case mq.messages <- message:
		mq.metrics.Enqueued++
		mq.metrics.CurrentSize = len(mq.messages)
		return nil
	default:
		// 队列已满，根据优先级决定是否丢弃
		if message.Metadata.Priority >= PriorityHigh {
			// 高优先级消息，尝试丢弃一个低优先级消息
			if mq.tryDropLowPriorityMessage() {
				mq.messages <- message
				mq.metrics.Enqueued++
				mq.metrics.CurrentSize = len(mq.messages)
				return nil
			}
		}
		mq.metrics.Dropped++
		return NewBluetoothError(ErrCodeResourceBusy, "消息队列已满")
	}
}

// Dequeue 出队消息
func (mq *MessageQueue[T]) Dequeue() <-chan Message[T] {
	return mq.messages
}

// Close 关闭队列
func (mq *MessageQueue[T]) Close() {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	if !mq.closed {
		close(mq.messages)
		mq.closed = true
	}
}

// GetMetrics 获取队列指标
func (mq *MessageQueue[T]) GetMetrics() QueueMetrics {
	mq.mu.RLock()
	defer mq.mu.RUnlock()

	metrics := *mq.metrics
	metrics.CurrentSize = len(mq.messages)
	return metrics
}

// tryDropLowPriorityMessage 尝试丢弃低优先级消息
func (mq *MessageQueue[T]) tryDropLowPriorityMessage() bool {
	// 这是一个简化实现，实际应该检查队列中的消息优先级
	// 由于 Go 的 channel 不支持随机访问，这里只是示例
	return false
}

// MessageSerializer 消息序列化器接口
type MessageSerializer[T any] interface {
	// Serialize 序列化消息
	Serialize(message Message[T]) ([]byte, error)
	// Deserialize 反序列化消息
	Deserialize(data []byte) (Message[T], error)
}

// JSONMessageSerializer JSON消息序列化器
type JSONMessageSerializer[T any] struct{}

// NewJSONMessageSerializer 创建JSON消息序列化器
func NewJSONMessageSerializer[T any]() *JSONMessageSerializer[T] {
	return &JSONMessageSerializer[T]{}
}

// Serialize 序列化消息为JSON
func (jms *JSONMessageSerializer[T]) Serialize(message Message[T]) ([]byte, error) {
	data, err := json.Marshal(message)
	if err != nil {
		return nil, WrapError(err, ErrCodeGeneral, "消息序列化失败", "", "serialize")
	}
	return data, nil
}

// Deserialize 从JSON反序列化消息
func (jms *JSONMessageSerializer[T]) Deserialize(data []byte) (Message[T], error) {
	var message Message[T]
	err := json.Unmarshal(data, &message)
	if err != nil {
		return message, WrapError(err, ErrCodeGeneral, "消息反序列化失败", "", "deserialize")
	}
	return message, nil
}

// DefaultMessageHandler 默认消息处理器实现
type DefaultMessageHandler[T any] struct {
	queue       *MessageQueue[T]                          // 消息队列
	serializer  MessageSerializer[T]                      // 消息序列化器
	callbacks   map[MessageType]MessageHandlerCallback[T] // 消息回调映射
	connections map[string]Connection                     // 连接映射
	stats       MessageHandlerStats                       // 统计信息
	ctx         context.Context                           // 上下文
	cancel      context.CancelFunc                        // 取消函数
	mu          sync.RWMutex                              // 读写锁
	running     bool                                      // 是否运行中
}

// NewDefaultMessageHandler 创建默认消息处理器
func NewDefaultMessageHandler[T any](queueSize int) *DefaultMessageHandler[T] {
	return &DefaultMessageHandler[T]{
		queue:       NewMessageQueue[T](queueSize),
		serializer:  NewJSONMessageSerializer[T](),
		callbacks:   make(map[MessageType]MessageHandlerCallback[T]),
		connections: make(map[string]Connection),
	}
}

// Start 启动消息处理器
func (dmh *DefaultMessageHandler[T]) Start(ctx context.Context) error {
	dmh.mu.Lock()
	defer dmh.mu.Unlock()

	if dmh.running {
		return NewBluetoothError(ErrCodeAlreadyExists, "消息处理器已在运行")
	}

	dmh.ctx, dmh.cancel = context.WithCancel(ctx)
	dmh.running = true

	// 启动消息处理协程
	go dmh.processMessages()

	return nil
}

// Stop 停止消息处理器
func (dmh *DefaultMessageHandler[T]) Stop() error {
	dmh.mu.Lock()
	defer dmh.mu.Unlock()

	if !dmh.running {
		return NewBluetoothError(ErrCodeGeneral, "消息处理器未运行")
	}

	dmh.cancel()
	dmh.queue.Close()
	dmh.running = false

	return nil
}

// Send 发送消息
func (dmh *DefaultMessageHandler[T]) Send(ctx context.Context, deviceID string, payload T) error {
	return dmh.SendWithPriority(ctx, deviceID, payload, PriorityNormal)
}

// SendWithPriority 发送带优先级的消息
func (dmh *DefaultMessageHandler[T]) SendWithPriority(ctx context.Context, deviceID string, payload T, priority Priority) error {
	// 创建消息
	message := Message[T]{
		ID:        generateMessageID(),
		Type:      MessageTypeData,
		Payload:   payload,
		Timestamp: time.Now(),
		Metadata: MessageMetadata{
			ReceiverID: deviceID,
			Priority:   priority,
			TTL:        DefaultSessionTimeout,
		},
	}

	// 获取连接
	dmh.mu.RLock()
	conn, exists := dmh.connections[deviceID]
	dmh.mu.RUnlock()

	if !exists {
		return NewBluetoothErrorWithDevice(ErrCodeDeviceNotFound, "设备连接未找到", deviceID, "send")
	}

	// 序列化消息
	data, err := dmh.serializer.Serialize(message)
	if err != nil {
		return err
	}

	// 发送数据
	err = conn.Send(data)
	if err != nil {
		dmh.mu.Lock()
		dmh.stats.ErrorCount++
		dmh.mu.Unlock()
		return WrapError(err, ErrCodeConnectionFailed, "消息发送失败", deviceID, "send")
	}

	// 更新统计信息
	dmh.mu.Lock()
	dmh.stats.MessagesSent++
	dmh.stats.LastActivity = time.Now()
	dmh.mu.Unlock()

	return nil
}

// Receive 接收消息通道
func (dmh *DefaultMessageHandler[T]) Receive() <-chan Message[T] {
	return dmh.queue.Dequeue()
}

// RegisterCallback 注册消息回调
func (dmh *DefaultMessageHandler[T]) RegisterCallback(messageType MessageType, callback MessageHandlerCallback[T]) {
	dmh.mu.Lock()
	defer dmh.mu.Unlock()

	dmh.callbacks[messageType] = callback
}

// AddConnection 添加连接
func (dmh *DefaultMessageHandler[T]) AddConnection(conn Connection) {
	dmh.mu.Lock()
	defer dmh.mu.Unlock()

	dmh.connections[conn.DeviceID()] = conn

	// 启动连接消息接收协程
	go dmh.handleConnectionMessages(conn)
}

// RemoveConnection 移除连接
func (dmh *DefaultMessageHandler[T]) RemoveConnection(deviceID string) {
	dmh.mu.Lock()
	defer dmh.mu.Unlock()

	delete(dmh.connections, deviceID)
}

// GetStats 获取统计信息
func (dmh *DefaultMessageHandler[T]) GetStats() MessageHandlerStats {
	dmh.mu.RLock()
	defer dmh.mu.RUnlock()

	stats := dmh.stats
	stats.MessagesQueued = uint64(dmh.queue.GetMetrics().CurrentSize)
	return stats
}

// processMessages 处理消息的主循环
func (dmh *DefaultMessageHandler[T]) processMessages() {
	for {
		select {
		case <-dmh.ctx.Done():
			return
		case message := <-dmh.queue.Dequeue():
			dmh.handleMessage(message)
		}
	}
}

// handleMessage 处理单个消息
func (dmh *DefaultMessageHandler[T]) handleMessage(message Message[T]) {
	startTime := time.Now()

	dmh.mu.RLock()
	callback, exists := dmh.callbacks[message.Type]
	dmh.mu.RUnlock()

	if exists {
		err := callback(dmh.ctx, message)
		if err != nil {
			dmh.mu.Lock()
			dmh.stats.ErrorCount++
			dmh.mu.Unlock()
		}
	}

	// 更新处理时间统计
	processingTime := time.Since(startTime)
	dmh.mu.Lock()
	dmh.stats.MessagesReceived++
	dmh.stats.ProcessingTime = (dmh.stats.ProcessingTime + processingTime) / 2
	dmh.stats.LastActivity = time.Now()
	dmh.mu.Unlock()
}

// handleConnectionMessages 处理连接消息
func (dmh *DefaultMessageHandler[T]) handleConnectionMessages(conn Connection) {
	for {
		select {
		case <-dmh.ctx.Done():
			return
		case data := <-conn.Receive():
			if data == nil {
				// 连接已关闭
				dmh.RemoveConnection(conn.DeviceID())
				return
			}

			// 反序列化消息
			message, err := dmh.serializer.Deserialize(data)
			if err != nil {
				dmh.mu.Lock()
				dmh.stats.ErrorCount++
				dmh.mu.Unlock()
				continue
			}

			// 将消息加入队列
			err = dmh.queue.Enqueue(message)
			if err != nil {
				dmh.mu.Lock()
				dmh.stats.ErrorCount++
				dmh.mu.Unlock()
			}
		}
	}
}

// generateMessageID 生成消息ID
func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}
