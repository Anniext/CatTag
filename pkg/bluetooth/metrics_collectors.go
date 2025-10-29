package bluetooth

import (
	"runtime"
	"time"
)

// BasicMetricsCollector 基础指标收集器
type BasicMetricsCollector struct {
	name string
}

// PerformanceMetricsCollector 性能指标收集器
type PerformanceMetricsCollector struct {
	name             string
	lastCollection   time.Time
	lastMessageCount uint64
}

// SystemMetricsCollector 系统指标收集器
type SystemMetricsCollector struct {
	name string
}

// NewBasicMetricsCollector 创建基础指标收集器
func NewBasicMetricsCollector() *BasicMetricsCollector {
	return &BasicMetricsCollector{
		name: "BasicMetricsCollector",
	}
}

// CollectMetrics 收集基础指标
func (bmc *BasicMetricsCollector) CollectMetrics(pool *ConnectionPoolImpl) (*PoolMetrics, error) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	metrics := &PoolMetrics{
		ConnectionsByPriority: make(map[Priority]int),
		ConnectionsByDevice:   make(map[string]*DeviceMetrics),
	}

	// 收集连接池基础统计
	stats := pool.stats
	metrics.ActiveConnections = stats.ActiveConnections
	metrics.TotalConnections = uint64(stats.TotalConnections)
	metrics.TotalBytesTransferred = stats.TotalBytesTransferred
	metrics.AverageLatency = stats.AverageLatency
	metrics.ErrorRate = stats.ErrorRate

	// 按优先级统计连接数
	priorityCount := make(map[Priority]int)

	// 收集设备级别指标
	for deviceID, pooledConn := range pool.connections {
		pooledConn.mu.RLock()

		// 统计优先级
		priorityCount[pooledConn.priority]++

		// 获取连接指标
		connMetrics := pooledConn.Connection.GetMetrics()

		deviceMetrics := &DeviceMetrics{
			DeviceID:         deviceID,
			ConnectionTime:   pooledConn.createdAt,
			LastActivity:     pooledConn.lastUsed,
			BytesSent:        connMetrics.BytesSent,
			BytesReceived:    connMetrics.BytesReceived,
			MessagesSent:     connMetrics.MessagesSent,
			MessagesReceived: connMetrics.MessagesRecv,
			ErrorCount:       connMetrics.ErrorCount,
			AverageLatency:   connMetrics.Latency,
			Priority:         pooledConn.priority,
			HealthScore:      pooledConn.healthScore,
			UseCount:         pooledConn.useCount,
		}

		metrics.ConnectionsByDevice[deviceID] = deviceMetrics
		pooledConn.mu.RUnlock()
	}

	metrics.ConnectionsByPriority = priorityCount

	return metrics, nil
}

// GetCollectorName 获取收集器名称
func (bmc *BasicMetricsCollector) GetCollectorName() string {
	return bmc.name
}

// NewPerformanceMetricsCollector 创建性能指标收集器
func NewPerformanceMetricsCollector() *PerformanceMetricsCollector {
	return &PerformanceMetricsCollector{
		name:           "PerformanceMetricsCollector",
		lastCollection: time.Now(),
	}
}

// CollectMetrics 收集性能指标
func (pmc *PerformanceMetricsCollector) CollectMetrics(pool *ConnectionPoolImpl) (*PoolMetrics, error) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	now := time.Now()
	timeDiff := now.Sub(pmc.lastCollection)

	metrics := &PoolMetrics{
		ConnectionsByPriority: make(map[Priority]int),
		ConnectionsByDevice:   make(map[string]*DeviceMetrics),
	}

	// 计算吞吐量
	var totalMessages uint64
	var totalLatency time.Duration
	var totalErrors uint64
	activeConnections := 0

	for _, pooledConn := range pool.connections {
		if pooledConn.Connection.IsActive() {
			activeConnections++
			connMetrics := pooledConn.Connection.GetMetrics()
			totalMessages += connMetrics.MessagesSent + connMetrics.MessagesRecv
			totalLatency += connMetrics.Latency
			totalErrors += connMetrics.ErrorCount
		}
	}

	// 计算吞吐量 (messages per second)
	if timeDiff.Seconds() > 0 {
		messageDiff := totalMessages - pmc.lastMessageCount
		metrics.Throughput = float64(messageDiff) / timeDiff.Seconds()
	}

	// 计算平均延迟
	if activeConnections > 0 {
		metrics.AverageLatency = totalLatency / time.Duration(activeConnections)
	}

	// 计算错误率
	if totalMessages > 0 {
		metrics.ErrorRate = float64(totalErrors) / float64(totalMessages)
	}

	metrics.ActiveConnections = activeConnections
	metrics.TotalMessages = totalMessages

	// 更新收集状态
	pmc.lastCollection = now
	pmc.lastMessageCount = totalMessages

	return metrics, nil
}

// GetCollectorName 获取收集器名称
func (pmc *PerformanceMetricsCollector) GetCollectorName() string {
	return pmc.name
}

// NewSystemMetricsCollector 创建系统指标收集器
func NewSystemMetricsCollector() *SystemMetricsCollector {
	return &SystemMetricsCollector{
		name: "SystemMetricsCollector",
	}
}

// CollectMetrics 收集系统指标
func (smc *SystemMetricsCollector) CollectMetrics(pool *ConnectionPoolImpl) (*PoolMetrics, error) {
	metrics := &PoolMetrics{
		ConnectionsByPriority: make(map[Priority]int),
		ConnectionsByDevice:   make(map[string]*DeviceMetrics),
	}

	// 收集Go运行时统计信息
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 创建性能快照（包含系统指标）
	snapshot := PerformanceSnapshot{
		Timestamp:   time.Now(),
		MemoryUsage: memStats.Alloc,
		CPUUsage:    smc.getCPUUsage(),
	}

	metrics.PerformanceHistory = []PerformanceSnapshot{snapshot}

	return metrics, nil
}

// getCPUUsage 获取CPU使用率（简化实现）
func (smc *SystemMetricsCollector) getCPUUsage() float64 {
	// 这里是一个简化的CPU使用率计算
	// 在实际应用中，可能需要使用更精确的方法
	return float64(runtime.NumGoroutine()) / float64(runtime.NumCPU()) * 0.1
}

// GetCollectorName 获取收集器名称
func (smc *SystemMetricsCollector) GetCollectorName() string {
	return smc.name
}

// CustomMetricsCollector 自定义指标收集器
type CustomMetricsCollector struct {
	name      string
	collector func(*ConnectionPoolImpl) (*PoolMetrics, error)
}

// NewCustomMetricsCollector 创建自定义指标收集器
func NewCustomMetricsCollector(name string, collector func(*ConnectionPoolImpl) (*PoolMetrics, error)) *CustomMetricsCollector {
	return &CustomMetricsCollector{
		name:      name,
		collector: collector,
	}
}

// CollectMetrics 收集自定义指标
func (cmc *CustomMetricsCollector) CollectMetrics(pool *ConnectionPoolImpl) (*PoolMetrics, error) {
	if cmc.collector != nil {
		return cmc.collector(pool)
	}

	return &PoolMetrics{
		ConnectionsByPriority: make(map[Priority]int),
		ConnectionsByDevice:   make(map[string]*DeviceMetrics),
	}, nil
}

// GetCollectorName 获取收集器名称
func (cmc *CustomMetricsCollector) GetCollectorName() string {
	return cmc.name
}

// AggregatedMetricsCollector 聚合指标收集器
type AggregatedMetricsCollector struct {
	name       string
	collectors []MetricsCollector
}

// NewAggregatedMetricsCollector 创建聚合指标收集器
func NewAggregatedMetricsCollector(collectors ...MetricsCollector) *AggregatedMetricsCollector {
	return &AggregatedMetricsCollector{
		name:       "AggregatedMetricsCollector",
		collectors: collectors,
	}
}

// CollectMetrics 收集聚合指标
func (amc *AggregatedMetricsCollector) CollectMetrics(pool *ConnectionPoolImpl) (*PoolMetrics, error) {
	aggregated := &PoolMetrics{
		ConnectionsByPriority: make(map[Priority]int),
		ConnectionsByDevice:   make(map[string]*DeviceMetrics),
		PerformanceHistory:    make([]PerformanceSnapshot, 0),
		Alerts:                make([]Alert, 0),
	}

	// 简化实现，直接返回基础指标
	// 在实际实现中，这里会收集真实的指标数据

	return aggregated, nil
}

// mergeMetrics 合并指标数据
func (amc *AggregatedMetricsCollector) mergeMetrics(target, source *PoolMetrics) {
	// 合并基础指标（取最新值）
	if source.ActiveConnections > 0 {
		target.ActiveConnections = source.ActiveConnections
	}
	if source.TotalConnections > target.TotalConnections {
		target.TotalConnections = source.TotalConnections
	}
	if source.TotalBytesTransferred > target.TotalBytesTransferred {
		target.TotalBytesTransferred = source.TotalBytesTransferred
	}
	if source.TotalMessages > target.TotalMessages {
		target.TotalMessages = source.TotalMessages
	}
	if source.AverageLatency > 0 {
		target.AverageLatency = source.AverageLatency
	}
	if source.ErrorRate > 0 {
		target.ErrorRate = source.ErrorRate
	}
	if source.Throughput > 0 {
		target.Throughput = source.Throughput
	}

	// 合并按优先级分组的连接数
	for priority, count := range source.ConnectionsByPriority {
		if count > target.ConnectionsByPriority[priority] {
			target.ConnectionsByPriority[priority] = count
		}
	}

	// 合并设备指标
	for deviceID, metrics := range source.ConnectionsByDevice {
		if existing, exists := target.ConnectionsByDevice[deviceID]; exists {
			// 合并设备指标（取最新值）
			if metrics.LastActivity.After(existing.LastActivity) {
				target.ConnectionsByDevice[deviceID] = metrics
			}
		} else {
			target.ConnectionsByDevice[deviceID] = metrics
		}
	}

	// 合并性能历史
	target.PerformanceHistory = append(target.PerformanceHistory, source.PerformanceHistory...)

	// 合并告警
	target.Alerts = append(target.Alerts, source.Alerts...)
}

// GetCollectorName 获取收集器名称
func (amc *AggregatedMetricsCollector) GetCollectorName() string {
	return amc.name
}

// AddCollector 添加子收集器
func (amc *AggregatedMetricsCollector) AddCollector(collector MetricsCollector) {
	amc.collectors = append(amc.collectors, collector)
}

// RemoveCollector 移除子收集器
func (amc *AggregatedMetricsCollector) RemoveCollector(index int) {
	if index >= 0 && index < len(amc.collectors) {
		amc.collectors = append(amc.collectors[:index], amc.collectors[index+1:]...)
	}
}

// GetCollectors 获取所有子收集器
func (amc *AggregatedMetricsCollector) GetCollectors() []MetricsCollector {
	return amc.collectors
}
