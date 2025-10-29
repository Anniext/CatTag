package adapter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// DefaultConnection 默认连接实现
type DefaultConnection struct {
	id          string
	deviceID    string
	active      bool
	sendChan    chan []byte
	receiveChan chan []byte
	metrics     bluetooth.ConnectionMetrics
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewDefaultConnection 创建新的默认连接实例
func NewDefaultConnection(deviceID string, bufferSize int) *DefaultConnection {
	ctx, cancel := context.WithCancel(context.Background())

	conn := &DefaultConnection{
		id:          fmt.Sprintf("conn_%s_%d", deviceID, time.Now().UnixNano()),
		deviceID:    deviceID,
		active:      true,
		sendChan:    make(chan []byte, bufferSize),
		receiveChan: make(chan []byte, bufferSize),
		metrics: bluetooth.ConnectionMetrics{
			ConnectedAt:  time.Now(),
			LastActivity: time.Now(),
		},
		ctx:    ctx,
		cancel: cancel,
	}

	// 启动数据处理goroutine
	conn.wg.Add(2)
	go conn.sendHandler()
	go conn.receiveHandler()

	return conn
}

// ID 连接唯一标识
func (dc *DefaultConnection) ID() string {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return dc.id
}

// DeviceID 设备标识
func (dc *DefaultConnection) DeviceID() string {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return dc.deviceID
}

// IsActive 连接是否活跃
func (dc *DefaultConnection) IsActive() bool {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return dc.active
}

// Send 发送数据
func (dc *DefaultConnection) Send(data []byte) error {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	if !dc.active {
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeDisconnected, "连接已断开", dc.deviceID, "send")
	}

	if len(data) == 0 {
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeInvalidParameter, "数据不能为空", dc.deviceID, "send")
	}

	select {
	case dc.sendChan <- data:
		// 更新指标
		dc.metrics.BytesSent += uint64(len(data))
		dc.metrics.MessagesSent++
		dc.metrics.LastActivity = time.Now()
		return nil
	case <-dc.ctx.Done():
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeDisconnected, "连接已关闭", dc.deviceID, "send")
	default:
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeResourceBusy, "发送缓冲区已满", dc.deviceID, "send")
	}
}

// Receive 接收数据通道
func (dc *DefaultConnection) Receive() <-chan []byte {
	return dc.receiveChan
}

// Close 关闭连接
func (dc *DefaultConnection) Close() error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if !dc.active {
		return nil
	}

	dc.active = false
	dc.cancel()

	// 关闭通道
	close(dc.sendChan)
	close(dc.receiveChan)

	// 等待goroutine结束
	dc.wg.Wait()

	return nil
}

// GetMetrics 获取连接指标
func (dc *DefaultConnection) GetMetrics() bluetooth.ConnectionMetrics {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	// 计算延迟（模拟）
	dc.metrics.Latency = 10 * time.Millisecond

	return dc.metrics
}

// sendHandler 发送数据处理器
func (dc *DefaultConnection) sendHandler() {
	defer dc.wg.Done()

	for {
		select {
		case <-dc.ctx.Done():
			return
		case data, ok := <-dc.sendChan:
			if !ok {
				return
			}

			// 模拟数据发送过程
			// 在实际实现中，这里会调用系统蓝牙API发送数据
			time.Sleep(1 * time.Millisecond) // 模拟发送延迟

			// 模拟发送成功，可以在这里处理发送确认等逻辑
			_ = data
		}
	}
}

// receiveHandler 接收数据处理器
func (dc *DefaultConnection) receiveHandler() {
	defer dc.wg.Done()

	// 模拟接收数据
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	messageCount := 0
	for {
		select {
		case <-dc.ctx.Done():
			return
		case <-ticker.C:
			// 模拟接收到数据
			if messageCount < 10 { // 限制模拟消息数量
				data := []byte(fmt.Sprintf("模拟接收数据 %d 来自设备 %s", messageCount+1, dc.deviceID))

				select {
				case dc.receiveChan <- data:
					// 更新指标
					dc.mu.Lock()
					dc.metrics.BytesReceived += uint64(len(data))
					dc.metrics.MessagesRecv++
					dc.metrics.LastActivity = time.Now()
					dc.mu.Unlock()
					messageCount++
				case <-dc.ctx.Done():
					return
				default:
					// 接收缓冲区满了
					dc.mu.Lock()
					dc.metrics.ErrorCount++
					dc.mu.Unlock()
				}
			}
		}
	}
}
