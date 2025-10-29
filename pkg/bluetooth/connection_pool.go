package bluetooth

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ConnectionPoolImpl 连接池实现
type ConnectionPoolImpl struct {
	mu               sync.RWMutex
	connections      map[string]*PooledConnection // 连接映射，key为设备ID
	maxConnections   int                          // 最大连接数
	stats            PoolStats                    // 连接池统计信息
	priorities       map[string]Priority          // 设备优先级映射
	loadBalancer     LoadBalancer                 // 负载均衡器
	lifecycleManager *ConnectionLifecycleManager  // 连接生命周期管理器
	ctx              context.Context              // 上下文
	cancel           context.CancelFunc           // 取消函数
	monitorInterval  time.Duration                // 监控间隔
}

// PooledConnection 池化连接，包装原始连接并添加池管理信息
type PooledConnection struct {
	Connection               // 嵌入原始连接接口
	priority    Priority     // 连接优先级
	createdAt   time.Time    // 创建时间
	lastUsed    time.Time    // 最后使用时间
	useCount    uint64       // 使用次数
	weight      int          // 负载均衡权重
	healthScore float64      // 健康评分 (0-1)
	mu          sync.RWMutex // 连接级别的锁
}

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	// SelectConnection 根据负载均衡策略选择连接
	SelectConnection(connections []*PooledConnection) *PooledConnection
	// UpdateWeight 更新连接权重
	UpdateWeight(deviceID string, weight int)
	// GetStrategy 获取负载均衡策略
	GetStrategy() LoadBalanceStrategy
}

// LoadBalanceStrategy 负载均衡策略
type LoadBalanceStrategy int

const (
	StrategyRoundRobin  LoadBalanceStrategy = iota // 轮询策略
	StrategyWeighted                               // 加权策略
	StrategyLeastUsed                              // 最少使用策略
	StrategyHealthBased                            // 基于健康状态策略
)

// ConnectionLifecycleManager 连接生命周期管理器
type ConnectionLifecycleManager struct {
	mu                  sync.RWMutex
	maxIdleTime         time.Duration // 最大空闲时间
	maxLifetime         time.Duration // 最大生存时间
	healthCheckInterval time.Duration // 健康检查间隔
	cleanupInterval     time.Duration // 清理间隔
}

// NewConnectionPool 创建新的连接池
func NewConnectionPool(maxConnections int) *ConnectionPoolImpl {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &ConnectionPoolImpl{
		connections:    make(map[string]*PooledConnection),
		maxConnections: maxConnections,
		priorities:     make(map[string]Priority),
		loadBalancer:   NewRoundRobinBalancer(),
		lifecycleManager: &ConnectionLifecycleManager{
			maxIdleTime:         30 * time.Minute,
			maxLifetime:         2 * time.Hour,
			healthCheckInterval: 1 * time.Minute,
			cleanupInterval:     5 * time.Minute,
		},
		ctx:             ctx,
		cancel:          cancel,
		monitorInterval: 30 * time.Second,
		stats: PoolStats{
			MaxConnections: maxConnections,
		},
	}

	// 启动后台监控
	go pool.startMonitoring()

	return pool
}

// AddConnection 添加连接到池中
func (p *ConnectionPoolImpl) AddConnection(conn Connection) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	deviceID := conn.DeviceID()

	// 检查连接是否已存在
	if _, exists := p.connections[deviceID]; exists {
		return NewBluetoothError(ErrCodeAlreadyExists, fmt.Sprintf("设备 %s 的连接已存在", deviceID))
	}

	// 检查是否达到最大连接数
	if len(p.connections) >= p.maxConnections {
		// 尝试移除低优先级或不健康的连接
		if !p.makeRoomForNewConnection() {
			return NewBluetoothError(ErrCodeResourceBusy, "连接池已满，无法添加新连接")
		}
	}

	// 获取设备优先级
	priority, exists := p.priorities[deviceID]
	if !exists {
		priority = PriorityNormal // 默认优先级
	}

	// 创建池化连接
	pooledConn := &PooledConnection{
		Connection:  conn,
		priority:    priority,
		createdAt:   time.Now(),
		lastUsed:    time.Now(),
		useCount:    0,
		weight:      1,
		healthScore: 1.0,
	}

	p.connections[deviceID] = pooledConn
	p.stats.TotalConnections++
	p.updateActiveConnections()

	return nil
}

// RemoveConnection 从池中移除连接
func (p *ConnectionPoolImpl) RemoveConnection(deviceID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	pooledConn, exists := p.connections[deviceID]
	if !exists {
		return NewBluetoothError(ErrCodeNotFound, fmt.Sprintf("设备 %s 的连接不存在", deviceID))
	}

	// 关闭连接
	if err := pooledConn.Connection.Close(); err != nil {
		// 记录错误但继续移除
	}

	delete(p.connections, deviceID)
	p.updateActiveConnections()

	return nil
}

// GetConnection 获取指定设备的连接
func (p *ConnectionPoolImpl) GetConnection(deviceID string) (Connection, error) {
	p.mu.RLock()
	pooledConn, exists := p.connections[deviceID]
	p.mu.RUnlock()

	if !exists {
		return nil, NewBluetoothError(ErrCodeNotFound, fmt.Sprintf("设备 %s 的连接不存在", deviceID))
	}

	// 更新使用统计
	pooledConn.mu.Lock()
	pooledConn.lastUsed = time.Now()
	pooledConn.useCount++
	pooledConn.mu.Unlock()

	return pooledConn.Connection, nil
}

// GetAllConnections 获取所有连接
func (p *ConnectionPoolImpl) GetAllConnections() []Connection {
	p.mu.RLock()
	defer p.mu.RUnlock()

	connections := make([]Connection, 0, len(p.connections))
	for _, pooledConn := range p.connections {
		connections = append(connections, pooledConn.Connection)
	}

	return connections
}

// SetMaxConnections 设置最大连接数
func (p *ConnectionPoolImpl) SetMaxConnections(max int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.maxConnections = max
	p.stats.MaxConnections = max

	// 如果当前连接数超过新的最大值，移除多余连接
	if len(p.connections) > max {
		p.removeExcessConnections(len(p.connections) - max)
	}
}

// GetStats 获取连接池统计信息
func (p *ConnectionPoolImpl) GetStats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 更新统计信息
	p.updateStats()
	return p.stats
}

// SetConnectionPriority 设置连接优先级
func (p *ConnectionPoolImpl) SetConnectionPriority(deviceID string, priority Priority) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.priorities[deviceID] = priority

	// 如果连接已存在，更新其优先级
	if pooledConn, exists := p.connections[deviceID]; exists {
		pooledConn.mu.Lock()
		pooledConn.priority = priority
		pooledConn.mu.Unlock()
	}
}

// GetConnectionsByPriority 按优先级获取连接
func (p *ConnectionPoolImpl) GetConnectionsByPriority(priority Priority) []Connection {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var connections []Connection
	for _, pooledConn := range p.connections {
		pooledConn.mu.RLock()
		if pooledConn.priority == priority {
			connections = append(connections, pooledConn.Connection)
		}
		pooledConn.mu.RUnlock()
	}

	return connections
}

// SelectConnectionByStrategy 根据负载均衡策略选择连接
func (p *ConnectionPoolImpl) SelectConnectionByStrategy() (Connection, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.connections) == 0 {
		return nil, NewBluetoothError(ErrCodeNotFound, "连接池中没有可用连接")
	}

	// 获取所有活跃连接
	var activeConnections []*PooledConnection
	for _, pooledConn := range p.connections {
		if pooledConn.Connection.IsActive() {
			activeConnections = append(activeConnections, pooledConn)
		}
	}

	if len(activeConnections) == 0 {
		return nil, NewBluetoothError(ErrCodeNotFound, "连接池中没有活跃连接")
	}

	// 使用负载均衡器选择连接
	selected := p.loadBalancer.SelectConnection(activeConnections)
	if selected == nil {
		return nil, NewBluetoothError(ErrCodeGeneral, "负载均衡器未能选择连接")
	}

	// 更新使用统计
	selected.mu.Lock()
	selected.lastUsed = time.Now()
	selected.useCount++
	selected.mu.Unlock()

	return selected.Connection, nil
}

// Close 关闭连接池
func (p *ConnectionPoolImpl) Close() error {
	p.cancel() // 停止监控

	p.mu.Lock()
	defer p.mu.Unlock()

	// 关闭所有连接
	for deviceID, pooledConn := range p.connections {
		if err := pooledConn.Connection.Close(); err != nil {
			// 记录错误但继续关闭其他连接
		}
		delete(p.connections, deviceID)
	}

	return nil
}

// makeRoomForNewConnection 为新连接腾出空间
func (p *ConnectionPoolImpl) makeRoomForNewConnection() bool {
	// 按优先级和健康状态排序，移除最不重要的连接
	var candidates []*PooledConnection
	for _, pooledConn := range p.connections {
		candidates = append(candidates, pooledConn)
	}

	// 排序：优先级低的、健康状态差的、使用频率低的排在前面
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]

		// 首先按优先级排序
		if a.priority != b.priority {
			return a.priority < b.priority
		}

		// 然后按健康状态排序
		if a.healthScore != b.healthScore {
			return a.healthScore < b.healthScore
		}

		// 最后按使用频率排序
		return a.useCount < b.useCount
	})

	// 移除第一个（最不重要的）连接
	if len(candidates) > 0 {
		deviceID := candidates[0].Connection.DeviceID()
		candidates[0].Connection.Close()
		delete(p.connections, deviceID)
		return true
	}

	return false
}

// removeExcessConnections 移除多余的连接
func (p *ConnectionPoolImpl) removeExcessConnections(count int) {
	// 获取所有连接并按重要性排序
	var candidates []*PooledConnection
	for _, pooledConn := range p.connections {
		candidates = append(candidates, pooledConn)
	}

	// 排序逻辑同 makeRoomForNewConnection
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.priority != b.priority {
			return a.priority < b.priority
		}
		if a.healthScore != b.healthScore {
			return a.healthScore < b.healthScore
		}
		return a.useCount < b.useCount
	})

	// 移除指定数量的连接
	for i := 0; i < count && i < len(candidates); i++ {
		deviceID := candidates[i].Connection.DeviceID()
		candidates[i].Connection.Close()
		delete(p.connections, deviceID)
	}
}

// updateActiveConnections 更新活跃连接数
func (p *ConnectionPoolImpl) updateActiveConnections() {
	activeCount := 0
	for _, pooledConn := range p.connections {
		if pooledConn.Connection.IsActive() {
			activeCount++
		}
	}
	p.stats.ActiveConnections = activeCount
}

// updateStats 更新统计信息
func (p *ConnectionPoolImpl) updateStats() {
	var totalLatency time.Duration
	var totalBytes uint64
	var totalErrors uint64
	activeCount := 0

	for _, pooledConn := range p.connections {
		if pooledConn.Connection.IsActive() {
			activeCount++
			metrics := pooledConn.Connection.GetMetrics()
			totalLatency += metrics.Latency
			totalBytes += metrics.BytesSent + metrics.BytesReceived
			totalErrors += metrics.ErrorCount
		}
	}

	p.stats.ActiveConnections = activeCount
	p.stats.TotalBytesTransferred = totalBytes

	if activeCount > 0 {
		p.stats.AverageLatency = totalLatency / time.Duration(activeCount)
		totalMessages := uint64(0)
		for _, pooledConn := range p.connections {
			if pooledConn.Connection.IsActive() {
				metrics := pooledConn.Connection.GetMetrics()
				totalMessages += metrics.MessagesSent + metrics.MessagesRecv
			}
		}
		if totalMessages > 0 {
			p.stats.ErrorRate = float64(totalErrors) / float64(totalMessages)
		}
	}
}

// startMonitoring 启动后台监控
func (p *ConnectionPoolImpl) startMonitoring() {
	ticker := time.NewTicker(p.monitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.performMaintenance()
		}
	}
}

// performMaintenance 执行维护任务
func (p *ConnectionPoolImpl) performMaintenance() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	var toRemove []string

	for deviceID, pooledConn := range p.connections {
		pooledConn.mu.RLock()

		// 检查连接是否超时
		idleTime := now.Sub(pooledConn.lastUsed)
		lifetime := now.Sub(pooledConn.createdAt)

		shouldRemove := false

		// 检查空闲时间
		if idleTime > p.lifecycleManager.maxIdleTime {
			shouldRemove = true
		}

		// 检查生存时间
		if lifetime > p.lifecycleManager.maxLifetime {
			shouldRemove = true
		}

		// 检查连接是否仍然活跃
		if !pooledConn.Connection.IsActive() {
			shouldRemove = true
		}

		pooledConn.mu.RUnlock()

		if shouldRemove {
			toRemove = append(toRemove, deviceID)
		}
	}

	// 移除需要清理的连接
	for _, deviceID := range toRemove {
		if pooledConn, exists := p.connections[deviceID]; exists {
			pooledConn.Connection.Close()
			delete(p.connections, deviceID)
		}
	}

	// 更新统计信息
	p.updateActiveConnections()
}
