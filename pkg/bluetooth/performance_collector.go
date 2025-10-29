package bluetooth

import (
	"context"
	"runtime"
	"sync"
	"time"
)

// PerformanceCollector 性能指标收集器接口
type PerformanceCollector interface {
	// StartCollection 开始收集性能指标
	StartCollection(ctx context.Context, interval time.Duration)
	// StopCollection 停止收集
	StopCollection()
	// GetMetrics 获取当前性能指标
	GetMetrics() PerformanceMetrics
	// GetHistoricalMetrics 获取历史性能指标
	GetHistoricalMetrics(duration time.Duration) []PerformanceMetrics
	// RegisterMetricsCallback 注册指标回调
	RegisterMetricsCallback(callback MetricsCallback)
}

// PerformanceMetrics 性能指标数据结构
type PerformanceMetrics struct {
	Timestamp         time.Time                    `json:"timestamp"`          // 时间戳
	SystemMetrics     SystemMetrics                `json:"system_metrics"`     // 系统指标
	ConnectionMetrics map[string]ConnectionMetrics `json:"connection_metrics"` // 连接指标
	PoolMetrics       PoolStats                    `json:"pool_metrics"`       // 连接池指标
	RSSIMetrics       map[string]RSSIData          `json:"rssi_metrics"`       // RSSI指标
	LatencyMetrics    map[string]LatencyData       `json:"latency_metrics"`    // 延迟指标
	ThroughputMetrics ThroughputData               `json:"throughput_metrics"` // 吞吐量指标
}

// RSSIData RSSI信号强度数据
type RSSIData struct {
	DeviceID    string    `json:"device_id"`    // 设备ID
	Current     int       `json:"current"`      // 当前RSSI值
	Average     float64   `json:"average"`      // 平均RSSI值
	Min         int       `json:"min"`          // 最小RSSI值
	Max         int       `json:"max"`          // 最大RSSI值
	Samples     []int     `json:"samples"`      // 样本数据
	LastUpdated time.Time `json:"last_updated"` // 最后更新时间
}

// LatencyData 延迟数据
type LatencyData struct {
	DeviceID    string          `json:"device_id"`    // 设备ID
	Current     time.Duration   `json:"current"`      // 当前延迟
	Average     time.Duration   `json:"average"`      // 平均延迟
	Min         time.Duration   `json:"min"`          // 最小延迟
	Max         time.Duration   `json:"max"`          // 最大延迟
	P95         time.Duration   `json:"p95"`          // 95百分位延迟
	P99         time.Duration   `json:"p99"`          // 99百分位延迟
	Samples     []time.Duration `json:"samples"`      // 样本数据
	LastUpdated time.Time       `json:"last_updated"` // 最后更新时间
}

// ThroughputData 吞吐量数据
type ThroughputData struct {
	BytesPerSecond    float64   `json:"bytes_per_second"`    // 每秒字节数
	MessagesPerSecond float64   `json:"messages_per_second"` // 每秒消息数
	TotalBytes        uint64    `json:"total_bytes"`         // 总字节数
	TotalMessages     uint64    `json:"total_messages"`      // 总消息数
	LastUpdated       time.Time `json:"last_updated"`        // 最后更新时间
}

// MetricsCallback 指标回调函数类型
type MetricsCallback func(metrics PerformanceMetrics)

// DefaultPerformanceCollector 默认性能指标收集器实现
type DefaultPerformanceCollector struct {
	mu                 sync.RWMutex
	connectionPool     ConnectionPool
	isCollecting       bool
	collectionCtx      context.Context
	collectionCancel   context.CancelFunc
	interval           time.Duration
	metricsHistory     []PerformanceMetrics
	maxHistorySize     int
	callbacks          []MetricsCallback
	rssiData           map[string]*RSSIData
	latencyData        map[string]*LatencyData
	throughputData     *ThroughputData
	lastCollectionTime time.Time
}

// NewDefaultPerformanceCollector 创建新的默认性能指标收集器
func NewDefaultPerformanceCollector(pool ConnectionPool) *DefaultPerformanceCollector {
	return &DefaultPerformanceCollector{
		connectionPool: pool,
		maxHistorySize: 1000, // 保留最近1000个指标记录
		callbacks:      make([]MetricsCallback, 0),
		rssiData:       make(map[string]*RSSIData),
		latencyData:    make(map[string]*LatencyData),
		throughputData: &ThroughputData{
			LastUpdated: time.Now(),
		},
		metricsHistory: make([]PerformanceMetrics, 0),
	}
}

// StartCollection 开始收集性能指标
func (pc *DefaultPerformanceCollector) StartCollection(ctx context.Context, interval time.Duration) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.isCollecting {
		return // 已经在收集中
	}

	pc.interval = interval
	pc.collectionCtx, pc.collectionCancel = context.WithCancel(ctx)
	pc.isCollecting = true
	pc.lastCollectionTime = time.Now()

	// 启动收集 goroutine
	go pc.collectionLoop()
}

// StopCollection 停止收集
func (pc *DefaultPerformanceCollector) StopCollection() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if !pc.isCollecting {
		return
	}

	if pc.collectionCancel != nil {
		pc.collectionCancel()
	}
	pc.isCollecting = false
}

// GetMetrics 获取当前性能指标
func (pc *DefaultPerformanceCollector) GetMetrics() PerformanceMetrics {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	return pc.collectCurrentMetrics()
}

// GetHistoricalMetrics 获取历史性能指标
func (pc *DefaultPerformanceCollector) GetHistoricalMetrics(duration time.Duration) []PerformanceMetrics {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	result := make([]PerformanceMetrics, 0)

	for _, metrics := range pc.metricsHistory {
		if metrics.Timestamp.After(cutoff) {
			result = append(result, metrics)
		}
	}

	return result
}

// RegisterMetricsCallback 注册指标回调
func (pc *DefaultPerformanceCollector) RegisterMetricsCallback(callback MetricsCallback) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.callbacks = append(pc.callbacks, callback)
}

// collectionLoop 收集循环
func (pc *DefaultPerformanceCollector) collectionLoop() {
	ticker := time.NewTicker(pc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-pc.collectionCtx.Done():
			return
		case <-ticker.C:
			pc.performCollection()
		}
	}
}

// performCollection 执行指标收集
func (pc *DefaultPerformanceCollector) performCollection() {
	metrics := pc.collectCurrentMetrics()

	pc.mu.Lock()
	// 添加到历史记录
	pc.metricsHistory = append(pc.metricsHistory, metrics)

	// 限制历史记录大小
	if len(pc.metricsHistory) > pc.maxHistorySize {
		pc.metricsHistory = pc.metricsHistory[1:]
	}

	// 更新RSSI和延迟数据
	pc.updateRSSIData(metrics.ConnectionMetrics)
	pc.updateLatencyData(metrics.ConnectionMetrics)
	pc.updateThroughputData(metrics.ConnectionMetrics)

	callbacks := make([]MetricsCallback, len(pc.callbacks))
	copy(callbacks, pc.callbacks)
	pc.mu.Unlock()

	// 异步调用回调函数
	for _, callback := range callbacks {
		go callback(metrics)
	}
}

// collectCurrentMetrics 收集当前指标
func (pc *DefaultPerformanceCollector) collectCurrentMetrics() PerformanceMetrics {
	now := time.Now()

	// 收集系统指标
	systemMetrics := pc.collectSystemMetrics()

	// 收集连接指标
	connectionMetrics := pc.collectConnectionMetrics()

	// 收集连接池指标
	poolMetrics := pc.connectionPool.GetStats()

	// 构建RSSI指标
	rssiMetrics := make(map[string]RSSIData)
	for deviceID, data := range pc.rssiData {
		rssiMetrics[deviceID] = *data
	}

	// 构建延迟指标
	latencyMetrics := make(map[string]LatencyData)
	for deviceID, data := range pc.latencyData {
		latencyMetrics[deviceID] = *data
	}

	return PerformanceMetrics{
		Timestamp:         now,
		SystemMetrics:     systemMetrics,
		ConnectionMetrics: connectionMetrics,
		PoolMetrics:       poolMetrics,
		RSSIMetrics:       rssiMetrics,
		LatencyMetrics:    latencyMetrics,
		ThroughputMetrics: *pc.throughputData,
	}
}

// collectSystemMetrics 收集系统指标
func (pc *DefaultPerformanceCollector) collectSystemMetrics() SystemMetrics {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return SystemMetrics{
		CPUUsage:    pc.getCPUUsage(),
		MemoryUsage: pc.getMemoryUsage(&memStats),
		Goroutines:  runtime.NumGoroutine(),
		GCPauses:    time.Duration(memStats.PauseNs[(memStats.NumGC+255)%256]),
	}
}

// getCPUUsage 获取CPU使用率（简化实现）
func (pc *DefaultPerformanceCollector) getCPUUsage() float64 {
	// 这里应该实现实际的CPU使用率计算
	// 目前返回模拟值，实际实现需要使用系统调用或第三方库
	return 0.0
}

// getMemoryUsage 获取内存使用率
func (pc *DefaultPerformanceCollector) getMemoryUsage(memStats *runtime.MemStats) float64 {
	// 计算堆内存使用率
	return float64(memStats.HeapInuse) / float64(memStats.HeapSys) * 100
}

// collectConnectionMetrics 收集连接指标
func (pc *DefaultPerformanceCollector) collectConnectionMetrics() map[string]ConnectionMetrics {
	connections := pc.connectionPool.GetAllConnections()
	metrics := make(map[string]ConnectionMetrics)

	for _, conn := range connections {
		if conn.IsActive() {
			deviceID := conn.DeviceID()
			metrics[deviceID] = conn.GetMetrics()
		}
	}

	return metrics
}

// updateRSSIData 更新RSSI数据
func (pc *DefaultPerformanceCollector) updateRSSIData(connectionMetrics map[string]ConnectionMetrics) {
	for deviceID := range connectionMetrics {
		// 获取当前RSSI值（这里需要与蓝牙适配器集成）
		currentRSSI := pc.getCurrentRSSI(deviceID)

		if data, exists := pc.rssiData[deviceID]; exists {
			// 更新现有数据
			data.Current = currentRSSI
			data.Samples = append(data.Samples, currentRSSI)

			// 限制样本数量
			if len(data.Samples) > 100 {
				data.Samples = data.Samples[1:]
			}

			// 计算统计值
			data.Average = pc.calculateAverage(data.Samples)
			data.Min = pc.calculateMin(data.Samples)
			data.Max = pc.calculateMax(data.Samples)
			data.LastUpdated = time.Now()
		} else {
			// 创建新数据
			pc.rssiData[deviceID] = &RSSIData{
				DeviceID:    deviceID,
				Current:     currentRSSI,
				Average:     float64(currentRSSI),
				Min:         currentRSSI,
				Max:         currentRSSI,
				Samples:     []int{currentRSSI},
				LastUpdated: time.Now(),
			}
		}
	}
}

// updateLatencyData 更新延迟数据
func (pc *DefaultPerformanceCollector) updateLatencyData(connectionMetrics map[string]ConnectionMetrics) {
	for deviceID, metrics := range connectionMetrics {
		currentLatency := metrics.Latency

		if data, exists := pc.latencyData[deviceID]; exists {
			// 更新现有数据
			data.Current = currentLatency
			data.Samples = append(data.Samples, currentLatency)

			// 限制样本数量
			if len(data.Samples) > 100 {
				data.Samples = data.Samples[1:]
			}

			// 计算统计值
			data.Average = pc.calculateLatencyAverage(data.Samples)
			data.Min = pc.calculateLatencyMin(data.Samples)
			data.Max = pc.calculateLatencyMax(data.Samples)
			data.P95 = pc.calculateLatencyPercentile(data.Samples, 0.95)
			data.P99 = pc.calculateLatencyPercentile(data.Samples, 0.99)
			data.LastUpdated = time.Now()
		} else {
			// 创建新数据
			pc.latencyData[deviceID] = &LatencyData{
				DeviceID:    deviceID,
				Current:     currentLatency,
				Average:     currentLatency,
				Min:         currentLatency,
				Max:         currentLatency,
				P95:         currentLatency,
				P99:         currentLatency,
				Samples:     []time.Duration{currentLatency},
				LastUpdated: time.Now(),
			}
		}
	}
}

// updateThroughputData 更新吞吐量数据
func (pc *DefaultPerformanceCollector) updateThroughputData(connectionMetrics map[string]ConnectionMetrics) {
	now := time.Now()
	duration := now.Sub(pc.lastCollectionTime).Seconds()

	if duration <= 0 {
		return
	}

	var totalBytes, totalMessages uint64
	for _, metrics := range connectionMetrics {
		totalBytes += metrics.BytesSent + metrics.BytesReceived
		totalMessages += metrics.MessagesSent + metrics.MessagesRecv
	}

	// 计算增量
	bytesDelta := totalBytes - pc.throughputData.TotalBytes
	messagesDelta := totalMessages - pc.throughputData.TotalMessages

	// 更新吞吐量
	pc.throughputData.BytesPerSecond = float64(bytesDelta) / duration
	pc.throughputData.MessagesPerSecond = float64(messagesDelta) / duration
	pc.throughputData.TotalBytes = totalBytes
	pc.throughputData.TotalMessages = totalMessages
	pc.throughputData.LastUpdated = now

	pc.lastCollectionTime = now
}

// getCurrentRSSI 获取当前RSSI值（模拟实现）
func (pc *DefaultPerformanceCollector) getCurrentRSSI(deviceID string) int {
	// 这里应该调用蓝牙适配器获取实际RSSI值
	// 目前返回模拟值
	conn, err := pc.connectionPool.GetConnection(deviceID)
	if err != nil {
		return -100 // 连接不存在时返回最低信号强度
	}

	metrics := conn.GetMetrics()
	// 基于错误率估算信号强度
	if metrics.ErrorCount > 10 {
		return -90
	} else if metrics.ErrorCount > 5 {
		return -70
	} else {
		return -50
	}
}

// 统计计算辅助函数
func (pc *DefaultPerformanceCollector) calculateAverage(samples []int) float64 {
	if len(samples) == 0 {
		return 0
	}

	sum := 0
	for _, sample := range samples {
		sum += sample
	}
	return float64(sum) / float64(len(samples))
}

func (pc *DefaultPerformanceCollector) calculateMin(samples []int) int {
	if len(samples) == 0 {
		return 0
	}

	min := samples[0]
	for _, sample := range samples[1:] {
		if sample < min {
			min = sample
		}
	}
	return min
}

func (pc *DefaultPerformanceCollector) calculateMax(samples []int) int {
	if len(samples) == 0 {
		return 0
	}

	max := samples[0]
	for _, sample := range samples[1:] {
		if sample > max {
			max = sample
		}
	}
	return max
}

func (pc *DefaultPerformanceCollector) calculateLatencyAverage(samples []time.Duration) time.Duration {
	if len(samples) == 0 {
		return 0
	}

	var sum time.Duration
	for _, sample := range samples {
		sum += sample
	}
	return sum / time.Duration(len(samples))
}

func (pc *DefaultPerformanceCollector) calculateLatencyMin(samples []time.Duration) time.Duration {
	if len(samples) == 0 {
		return 0
	}

	min := samples[0]
	for _, sample := range samples[1:] {
		if sample < min {
			min = sample
		}
	}
	return min
}

func (pc *DefaultPerformanceCollector) calculateLatencyMax(samples []time.Duration) time.Duration {
	if len(samples) == 0 {
		return 0
	}

	max := samples[0]
	for _, sample := range samples[1:] {
		if sample > max {
			max = sample
		}
	}
	return max
}

func (pc *DefaultPerformanceCollector) calculateLatencyPercentile(samples []time.Duration, percentile float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}

	// 简化的百分位计算，实际实现应该对样本进行排序
	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)

	// 简单的冒泡排序（实际应该使用更高效的排序算法）
	for i := 0; i < len(sorted); i++ {
		for j := 0; j < len(sorted)-1-i; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	index := int(float64(len(sorted)-1) * percentile)
	return sorted[index]
}

// SetMaxHistorySize 设置最大历史记录大小
func (pc *DefaultPerformanceCollector) SetMaxHistorySize(size int) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.maxHistorySize = size
}

// ClearHistory 清空历史记录
func (pc *DefaultPerformanceCollector) ClearHistory() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.metricsHistory = make([]PerformanceMetrics, 0)
}

// GetRSSITrend 获取RSSI趋势分析
func (pc *DefaultPerformanceCollector) GetRSSITrend(deviceID string, duration time.Duration) *RSSITrend {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	data, exists := pc.rssiData[deviceID]
	if !exists {
		return nil
	}

	// 分析RSSI趋势
	trend := &RSSITrend{
		DeviceID:    deviceID,
		TimeRange:   duration,
		CurrentRSSI: data.Current,
		AverageRSSI: data.Average,
		Trend:       pc.calculateRSSITrend(data.Samples),
		Quality:     pc.evaluateSignalQuality(data.Current),
	}

	return trend
}

// RSSITrend RSSI趋势分析结果
type RSSITrend struct {
	DeviceID    string        `json:"device_id"`    // 设备ID
	TimeRange   time.Duration `json:"time_range"`   // 时间范围
	CurrentRSSI int           `json:"current_rssi"` // 当前RSSI
	AverageRSSI float64       `json:"average_rssi"` // 平均RSSI
	Trend       string        `json:"trend"`        // 趋势（improving/stable/degrading）
	Quality     string        `json:"quality"`      // 信号质量（excellent/good/fair/poor）
}

// calculateRSSITrend 计算RSSI趋势
func (pc *DefaultPerformanceCollector) calculateRSSITrend(samples []int) string {
	if len(samples) < 2 {
		return "stable"
	}

	// 简单的趋势分析：比较前半部分和后半部分的平均值
	mid := len(samples) / 2
	firstHalf := samples[:mid]
	secondHalf := samples[mid:]

	firstAvg := pc.calculateAverage(firstHalf)
	secondAvg := pc.calculateAverage(secondHalf)

	diff := secondAvg - firstAvg
	if diff > 5 {
		return "improving"
	} else if diff < -5 {
		return "degrading"
	} else {
		return "stable"
	}
}

// evaluateSignalQuality 评估信号质量
func (pc *DefaultPerformanceCollector) evaluateSignalQuality(rssi int) string {
	if rssi >= -50 {
		return "excellent"
	} else if rssi >= -60 {
		return "good"
	} else if rssi >= -70 {
		return "fair"
	} else {
		return "poor"
	}
}
