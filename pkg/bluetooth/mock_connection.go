package bluetooth

import (
	"sync"
	"time"
)

// MockConnection 模拟连接实现
type MockConnection struct {
	mu                sync.RWMutex
	id                string
	deviceID          string
	active            bool
	receiveChan       chan []byte
	sentData          [][]byte
	metrics           ConnectionMetrics
	heartbeatResponse bool
	heartbeatDelay    time.Duration
}

// NewMockConnection 创建新的模拟连接
func NewMockConnection(deviceID string) Connection {
	return &MockConnection{
		id:                "conn_" + deviceID,
		deviceID:          deviceID,
		active:            true,
		receiveChan:       make(chan []byte, 100),
		sentData:          make([][]byte, 0),
		heartbeatResponse: true,
		heartbeatDelay:    10 * time.Millisecond,
		metrics: ConnectionMetrics{
			ConnectedAt:  time.Now(),
			LastActivity: time.Now(),
		},
	}
}

// NewMockConnectionPtr 创建新的模拟连接指针（用于测试）
func NewMockConnectionPtr(deviceID string) *MockConnection {
	return &MockConnection{
		id:                "conn_" + deviceID,
		deviceID:          deviceID,
		active:            true,
		receiveChan:       make(chan []byte, 100),
		sentData:          make([][]byte, 0),
		heartbeatResponse: true,
		heartbeatDelay:    10 * time.Millisecond,
		metrics: ConnectionMetrics{
			ConnectedAt:  time.Now(),
			LastActivity: time.Now(),
		},
	}
}

// ID 连接唯一标识
func (m *MockConnection) ID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.id
}

// DeviceID 设备标识
func (m *MockConnection) DeviceID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.deviceID
}

// IsActive 连接是否活跃
func (m *MockConnection) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

// Send 发送数据
func (m *MockConnection) Send(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.active {
		return NewBluetoothError(ErrCodeDisconnected, "连接已断开")
	}

	// 记录发送的数据
	m.sentData = append(m.sentData, data)
	m.metrics.BytesSent += uint64(len(data))
	m.metrics.MessagesSent++
	m.metrics.LastActivity = time.Now()

	// 如果是心跳包且配置为响应心跳，则自动回复
	if string(data) == "HEARTBEAT" && m.heartbeatResponse {
		go func() {
			time.Sleep(m.heartbeatDelay)
			m.SimulateReceiveData([]byte("HEARTBEAT_ACK"))
		}()
	}

	return nil
}

// Receive 接收数据通道
func (m *MockConnection) Receive() <-chan []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.receiveChan
}

// Close 关闭连接
func (m *MockConnection) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.active {
		m.active = false
		close(m.receiveChan)
	}
	return nil
}

// GetMetrics 获取连接指标
func (m *MockConnection) GetMetrics() ConnectionMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.metrics
}

// SimulateReceiveData 模拟接收数据
func (m *MockConnection) SimulateReceiveData(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.active {
		select {
		case m.receiveChan <- data:
			m.metrics.BytesReceived += uint64(len(data))
			m.metrics.MessagesRecv++
			m.metrics.LastActivity = time.Now()
		default:
			// 通道已满，丢弃数据
		}
	}
}

// GetSentData 获取已发送的数据
func (m *MockConnection) GetSentData() [][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sentData
}

// SimulateDisconnect 模拟连接断开（用于测试）
func (m *MockConnection) SimulateDisconnect() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.active = false
	// 不关闭通道，让监控器检测到连接断开
}

// SimulateLatency 模拟网络延迟（用于测试）
func (m *MockConnection) SimulateLatency(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics.Latency = latency
}

// IncrementErrorCount 增加错误计数（用于测试）
func (m *MockConnection) IncrementErrorCount() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics.ErrorCount++
}

// SetHeartbeatResponse 设置是否响应心跳（用于测试）
func (m *MockConnection) SetHeartbeatResponse(respond bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.heartbeatResponse = respond
}

// SetHeartbeatDelay 设置心跳响应延迟（用于测试）
func (m *MockConnection) SetHeartbeatDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.heartbeatDelay = delay
}
