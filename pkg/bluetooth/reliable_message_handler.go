package bluetooth

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ReliableMessageHandler 可靠消息处理器，支持确认和重传机制
type ReliableMessageHandler[T any] struct {
	*DefaultMessageHandler[T]                               // 嵌入默认消息处理器
	pendingMessages           map[string]*PendingMessage[T] // 待确认消息映射
	ackTimeout                time.Duration                 // 确认超时时间
	maxRetries                int                           // 最大重试次数
	retryInterval             time.Duration                 // 重试间隔
	deduplicator              *MessageDeduplicator[T]       // 消息去重器
	mu                        sync.RWMutex                  // 读写锁
}

// PendingMessage 待确认消息
type PendingMessage[T any] struct {
	Message    Message[T]  // 原始消息
	DeviceID   string      // 目标设备ID
	SentAt     time.Time   // 发送时间
	RetryCount int         // 重试次数
	Timer      *time.Timer // 超时定时器
	OnAck      func()      // 确认回调
	OnTimeout  func(error) // 超时回调
}

// MessageDeduplicator 消息去重器
type MessageDeduplicator[T any] struct {
	receivedMessages map[string]time.Time // 已接收消息ID映射
	cleanupInterval  time.Duration        // 清理间隔
	messageTimeout   time.Duration        // 消息超时时间
	mu               sync.RWMutex         // 读写锁
	ctx              context.Context      // 上下文
	cancel           context.CancelFunc   // 取消函数
}

// NewMessageDeduplicator 创建消息去重器
func NewMessageDeduplicator[T any](cleanupInterval, messageTimeout time.Duration) *MessageDeduplicator[T] {
	return &MessageDeduplicator[T]{
		receivedMessages: make(map[string]time.Time),
		cleanupInterval:  cleanupInterval,
		messageTimeout:   messageTimeout,
	}
}

// Start 启动去重器
func (md *MessageDeduplicator[T]) Start(ctx context.Context) {
	md.ctx, md.cancel = context.WithCancel(ctx)
	go md.cleanupLoop()
}

// Stop 停止去重器
func (md *MessageDeduplicator[T]) Stop() {
	if md.cancel != nil {
		md.cancel()
	}
}

// IsDuplicate 检查消息是否重复
func (md *MessageDeduplicator[T]) IsDuplicate(messageID string) bool {
	md.mu.RLock()
	defer md.mu.RUnlock()

	_, exists := md.receivedMessages[messageID]
	return exists
}

// MarkReceived 标记消息已接收
func (md *MessageDeduplicator[T]) MarkReceived(messageID string) {
	md.mu.Lock()
	defer md.mu.Unlock()

	md.receivedMessages[messageID] = time.Now()
}

// cleanupLoop 清理过期消息的循环
func (md *MessageDeduplicator[T]) cleanupLoop() {
	ticker := time.NewTicker(md.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-md.ctx.Done():
			return
		case <-ticker.C:
			md.cleanup()
		}
	}
}

// cleanup 清理过期消息
func (md *MessageDeduplicator[T]) cleanup() {
	md.mu.Lock()
	defer md.mu.Unlock()

	now := time.Now()
	for messageID, receivedAt := range md.receivedMessages {
		if now.Sub(receivedAt) > md.messageTimeout {
			delete(md.receivedMessages, messageID)
		}
	}
}

// NewReliableMessageHandler 创建可靠消息处理器
func NewReliableMessageHandler[T any](queueSize int, ackTimeout time.Duration, maxRetries int, retryInterval time.Duration) *ReliableMessageHandler[T] {
	return &ReliableMessageHandler[T]{
		DefaultMessageHandler: NewDefaultMessageHandler[T](queueSize),
		pendingMessages:       make(map[string]*PendingMessage[T]),
		ackTimeout:            ackTimeout,
		maxRetries:            maxRetries,
		retryInterval:         retryInterval,
		deduplicator:          NewMessageDeduplicator[T](5*time.Minute, 30*time.Minute),
	}
}

// Start 启动可靠消息处理器
func (rmh *ReliableMessageHandler[T]) Start(ctx context.Context) error {
	err := rmh.DefaultMessageHandler.Start(ctx)
	if err != nil {
		return err
	}

	// 启动去重器
	rmh.deduplicator.Start(ctx)

	// 注册ACK消息回调
	rmh.RegisterCallback(MessageTypeAck, func(ctx context.Context, message Message[T]) error {
		return rmh.handleAckMessage(ctx, message)
	})

	return nil
}

// Stop 停止可靠消息处理器
func (rmh *ReliableMessageHandler[T]) Stop() error {
	// 停止去重器
	rmh.deduplicator.Stop()

	// 取消所有待确认消息
	rmh.mu.Lock()
	for _, pending := range rmh.pendingMessages {
		if pending.Timer != nil {
			pending.Timer.Stop()
		}
	}
	rmh.pendingMessages = make(map[string]*PendingMessage[T])
	rmh.mu.Unlock()

	return rmh.DefaultMessageHandler.Stop()
}

// SendReliable 可靠发送消息，需要确认
func (rmh *ReliableMessageHandler[T]) SendReliable(ctx context.Context, deviceID string, payload T, priority Priority) error {
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

	// 创建待确认消息
	pending := &PendingMessage[T]{
		Message:    message,
		DeviceID:   deviceID,
		SentAt:     time.Now(),
		RetryCount: 0,
	}

	// 添加到待确认列表
	rmh.mu.Lock()
	rmh.pendingMessages[message.ID] = pending
	rmh.mu.Unlock()

	// 发送消息
	err := rmh.sendMessageWithRetry(pending)
	if err != nil {
		rmh.mu.Lock()
		delete(rmh.pendingMessages, message.ID)
		rmh.mu.Unlock()
		return err
	}

	return nil
}

// sendMessageWithRetry 发送消息并设置重试
func (rmh *ReliableMessageHandler[T]) sendMessageWithRetry(pending *PendingMessage[T]) error {
	// 发送消息
	err := rmh.DefaultMessageHandler.SendWithPriority(rmh.ctx, pending.DeviceID, pending.Message.Payload, pending.Message.Metadata.Priority)
	if err != nil {
		return err
	}

	// 设置确认超时定时器
	pending.Timer = time.AfterFunc(rmh.ackTimeout, func() {
		rmh.handleMessageTimeout(pending.Message.ID)
	})

	return nil
}

// handleMessageTimeout 处理消息超时
func (rmh *ReliableMessageHandler[T]) handleMessageTimeout(messageID string) {
	rmh.mu.Lock()
	pending, exists := rmh.pendingMessages[messageID]
	if !exists {
		rmh.mu.Unlock()
		return
	}

	pending.RetryCount++

	// 检查是否超过最大重试次数
	if pending.RetryCount >= rmh.maxRetries {
		// 删除待确认消息
		delete(rmh.pendingMessages, messageID)
		rmh.mu.Unlock()

		// 调用超时回调
		if pending.OnTimeout != nil {
			pending.OnTimeout(NewBluetoothErrorWithDevice(ErrCodeTimeout, "消息发送超时", pending.DeviceID, "send_reliable"))
		}
		return
	}
	rmh.mu.Unlock()

	// 等待重试间隔后重新发送
	time.AfterFunc(rmh.retryInterval, func() {
		err := rmh.sendMessageWithRetry(pending)
		if err != nil {
			rmh.mu.Lock()
			delete(rmh.pendingMessages, messageID)
			rmh.mu.Unlock()

			if pending.OnTimeout != nil {
				pending.OnTimeout(err)
			}
		}
	})
}

// handleAckMessage 处理确认消息
func (rmh *ReliableMessageHandler[T]) handleAckMessage(ctx context.Context, message Message[T]) error {
	// 从消息载荷中提取原始消息ID
	// 首先尝试作为JSON字符串解析
	var ackData map[string]interface{}
	payloadStr := fmt.Sprintf("%v", message.Payload)

	err := json.Unmarshal([]byte(payloadStr), &ackData)
	if err != nil {
		// 如果不是JSON字符串，尝试直接类型断言
		ackData, ok := any(message.Payload).(map[string]interface{})
		if !ok {
			return NewBluetoothError(ErrCodeProtocolError, "无效的ACK消息格式")
		}
		originalMessageID, ok := ackData["message_id"].(string)
		if !ok {
			return NewBluetoothError(ErrCodeProtocolError, "ACK消息缺少原始消息ID")
		}
		return rmh.processAckMessage(originalMessageID)
	}

	originalMessageID, ok := ackData["message_id"].(string)
	if !ok {
		return NewBluetoothError(ErrCodeProtocolError, "ACK消息缺少原始消息ID")
	}

	return rmh.processAckMessage(originalMessageID)
}

// processAckMessage 处理ACK消息的核心逻辑
func (rmh *ReliableMessageHandler[T]) processAckMessage(originalMessageID string) error {

	// 查找待确认消息
	rmh.mu.Lock()
	pending, exists := rmh.pendingMessages[originalMessageID]
	if exists {
		// 停止超时定时器
		if pending.Timer != nil {
			pending.Timer.Stop()
		}
		// 删除待确认消息
		delete(rmh.pendingMessages, originalMessageID)
	}
	rmh.mu.Unlock()

	if exists && pending.OnAck != nil {
		pending.OnAck()
	}

	return nil
}

// SendAck 发送确认消息
func (rmh *ReliableMessageHandler[T]) SendAck(ctx context.Context, deviceID string, originalMessageID string) error {
	// 创建ACK载荷作为JSON字符串
	ackPayload := fmt.Sprintf(`{"message_id":"%s","timestamp":"%s"}`, originalMessageID, time.Now().Format(time.RFC3339))

	// 发送ACK消息（不需要确认）
	return rmh.DefaultMessageHandler.SendWithPriority(ctx, deviceID, any(ackPayload).(T), PriorityHigh)
}

// ProcessIncomingMessage 处理接收到的消息
func (rmh *ReliableMessageHandler[T]) ProcessIncomingMessage(message Message[T]) error {
	// 检查消息是否重复
	if rmh.deduplicator.IsDuplicate(message.ID) {
		// 重复消息，发送ACK但不处理
		if message.Type == MessageTypeData {
			rmh.SendAck(rmh.ctx, message.Metadata.SenderID, message.ID)
		}
		return nil
	}

	// 标记消息已接收
	rmh.deduplicator.MarkReceived(message.ID)

	// 如果是数据消息，发送ACK
	if message.Type == MessageTypeData {
		err := rmh.SendAck(rmh.ctx, message.Metadata.SenderID, message.ID)
		if err != nil {
			// ACK发送失败不影响消息处理
			rmh.mu.Lock()
			rmh.stats.ErrorCount++
			rmh.mu.Unlock()
		}
	}

	// 将消息加入处理队列
	return rmh.queue.Enqueue(message)
}

// SetMessageCallbacks 设置消息回调
func (rmh *ReliableMessageHandler[T]) SetMessageCallbacks(messageID string, onAck func(), onTimeout func(error)) {
	rmh.mu.Lock()
	defer rmh.mu.Unlock()

	if pending, exists := rmh.pendingMessages[messageID]; exists {
		pending.OnAck = onAck
		pending.OnTimeout = onTimeout
	}
}

// GetPendingMessageCount 获取待确认消息数量
func (rmh *ReliableMessageHandler[T]) GetPendingMessageCount() int {
	rmh.mu.RLock()
	defer rmh.mu.RUnlock()

	return len(rmh.pendingMessages)
}

// GetReliabilityStats 获取可靠性统计信息
func (rmh *ReliableMessageHandler[T]) GetReliabilityStats() ReliabilityStats {
	rmh.mu.RLock()
	defer rmh.mu.RUnlock()

	var totalRetries int
	var oldestPending time.Time
	now := time.Now()

	for _, pending := range rmh.pendingMessages {
		totalRetries += pending.RetryCount
		if oldestPending.IsZero() || pending.SentAt.Before(oldestPending) {
			oldestPending = pending.SentAt
		}
	}

	var maxPendingTime time.Duration
	if !oldestPending.IsZero() {
		maxPendingTime = now.Sub(oldestPending)
	}

	return ReliabilityStats{
		PendingMessages:      len(rmh.pendingMessages),
		TotalRetries:         totalRetries,
		MaxPendingTime:       maxPendingTime,
		DeduplicatedMessages: len(rmh.deduplicator.receivedMessages),
	}
}

// ReliabilityStats 可靠性统计信息
type ReliabilityStats struct {
	PendingMessages      int           `json:"pending_messages"`      // 待确认消息数
	TotalRetries         int           `json:"total_retries"`         // 总重试次数
	MaxPendingTime       time.Duration `json:"max_pending_time"`      // 最长待确认时间
	DeduplicatedMessages int           `json:"deduplicated_messages"` // 去重消息数
}
