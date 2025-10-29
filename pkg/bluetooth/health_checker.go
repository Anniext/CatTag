package bluetooth

import (
	"context"
	"sync"
	"time"
)

// DefaultHealthChecker 默认健康检查器实现
type DefaultHealthChecker struct {
	mu                   sync.RWMutex
	connectionPool       ConnectionPool
	performanceCollector PerformanceCollector
	callbackManager      *CallbackManager
	monitoringCtx        context.Context
	monitoringCancel     context.CancelFunc
	interval             time.Duration
	deviceStatuses       map[string]*HealthStatus
	isMonitoring         bool
	heartbeatTimeout     time.Duration
	maxErrorCount        int
	rssiThreshold        int
	latencyThreshold     time.Duration
}

// NewDefaultHealthChecker 创建新的默认健康检查器
func NewDefaultHealthChecker(pool ConnectionPool) *DefaultHealthChecker {
	hc := &DefaultHealthChecker{
		connectionPool:   pool,
		callbackManager:  NewCallbackManager(),
		deviceStatuses:   make(map[string]*HealthStatus),
		heartbeatTimeout: 30 * time.Second,
		maxErrorCount:    5,
		rssiThreshold:    -80, // dBm
		latencyThreshold: 1 * time.Second,
	}

	// 创建性能收集器
	hc.performanceCollector = NewDefaultPerformanceCollector(pool)

	return hc
}

// StartMonitoring 开始监控连接健康状态
func (hc *DefaultHealthChecker) StartMonitoring(ctx context.Context, interval time.Duration) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if hc.isMonitoring {
		return // 已经在监控中
	}

	hc.interval = interval
	hc.monitoringCtx, hc.monitoringCancel = context.WithCancel(ctx)
	hc.isMonitoring = true

	// 启动监控 goroutine
	go hc.monitoringLoop()
}

// StopMonitoring 停止监控
func (hc *DefaultHealthChecker) StopMonitoring() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if !hc.isMonitoring {
		return
	}

	if hc.monitoringCancel != nil {
		hc.monitoringCancel()
	}
	hc.isMonitoring = false
}

// CheckConnection 检查指定连接的健康状态
func (hc *DefaultHealthChecker) CheckConnection(deviceID string) HealthStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	if status, exists := hc.deviceStatuses[deviceID]; exists {
		return *status
	}

	// 如果没有缓存状态，执行实时检查
	return hc.performHealthCheck(deviceID)
}

// GetHealthReport 获取完整的健康报告
func (hc *DefaultHealthChecker) GetHealthReport() HealthReport {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	statuses := make([]HealthStatus, 0, len(hc.deviceStatuses))
	overallHealth := true

	for _, status := range hc.deviceStatuses {
		statuses = append(statuses, *status)
		if !status.IsHealthy {
			overallHealth = false
		}
	}

	return HealthReport{
		Timestamp:       time.Now(),
		OverallHealth:   overallHealth,
		DeviceStatuses:  statuses,
		SystemMetrics:   hc.collectSystemMetrics(),
		Recommendations: hc.generateRecommendations(statuses),
	}
}

// RegisterHealthCallback 注册健康状态回调
func (hc *DefaultHealthChecker) RegisterHealthCallback(callback HealthCallback) {
	hc.callbackManager.RegisterHealthCallback(callback)
}

// monitoringLoop 监控循环
func (hc *DefaultHealthChecker) monitoringLoop() {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-hc.monitoringCtx.Done():
			return
		case <-ticker.C:
			hc.performPeriodicHealthCheck()
		}
	}
}

// performPeriodicHealthCheck 执行周期性健康检查
func (hc *DefaultHealthChecker) performPeriodicHealthCheck() {
	connections := hc.connectionPool.GetAllConnections()

	for _, conn := range connections {
		deviceID := conn.DeviceID()
		status := hc.performHealthCheck(deviceID)

		hc.mu.Lock()
		oldStatus, exists := hc.deviceStatuses[deviceID]
		hc.deviceStatuses[deviceID] = &status
		hc.mu.Unlock()

		// 如果健康状态发生变化，触发回调
		if !exists || oldStatus.IsHealthy != status.IsHealthy {
			hc.callbackManager.NotifyHealthStatus(status)
		}
	}
}

// performHealthCheck 执行单个设备的健康检查
func (hc *DefaultHealthChecker) performHealthCheck(deviceID string) HealthStatus {
	conn, err := hc.connectionPool.GetConnection(deviceID)
	if err != nil {
		return HealthStatus{
			DeviceID:      deviceID,
			IsHealthy:     false,
			LastHeartbeat: time.Time{},
			RSSI:          0,
			Latency:       0,
			ErrorCount:    -1,
			Status:        "连接不存在",
		}
	}

	// 检查连接是否活跃
	if !conn.IsActive() {
		return HealthStatus{
			DeviceID:      deviceID,
			IsHealthy:     false,
			LastHeartbeat: time.Time{},
			RSSI:          0,
			Latency:       0,
			ErrorCount:    0,
			Status:        "连接不活跃",
		}
	}

	// 获取连接指标
	metrics := conn.GetMetrics()

	// 执行心跳检查
	heartbeatResult := hc.performHeartbeat(conn)

	// 评估健康状态
	isHealthy := hc.evaluateHealth(heartbeatResult, metrics)

	status := HealthStatus{
		DeviceID:      deviceID,
		IsHealthy:     isHealthy,
		LastHeartbeat: heartbeatResult.timestamp,
		RSSI:          heartbeatResult.rssi,
		Latency:       heartbeatResult.latency,
		ErrorCount:    int(metrics.ErrorCount),
		Status:        hc.generateStatusMessage(isHealthy, heartbeatResult, metrics),
	}

	return status
}

// heartbeatResult 心跳检查结果
type heartbeatResult struct {
	success   bool
	timestamp time.Time
	latency   time.Duration
	rssi      int
	error     error
}

// performHeartbeat 执行心跳检查
func (hc *DefaultHealthChecker) performHeartbeat(conn Connection) heartbeatResult {
	start := time.Now()

	// 创建心跳消息
	heartbeatData := []byte("HEARTBEAT")

	// 发送心跳包
	err := conn.Send(heartbeatData)
	if err != nil {
		return heartbeatResult{
			success:   false,
			timestamp: start,
			latency:   0,
			rssi:      0,
			error:     err,
		}
	}

	// 等待响应或超时
	select {
	case <-conn.Receive():
		latency := time.Since(start)
		return heartbeatResult{
			success:   true,
			timestamp: time.Now(),
			latency:   latency,
			rssi:      hc.getRSSI(conn), // 获取信号强度
			error:     nil,
		}
	case <-time.After(hc.heartbeatTimeout):
		return heartbeatResult{
			success:   false,
			timestamp: start,
			latency:   hc.heartbeatTimeout,
			rssi:      0,
			error:     NewBluetoothErrorWithDevice(ErrCodeTimeout, "心跳超时", conn.DeviceID(), "heartbeat"),
		}
	}
}

// getRSSI 获取连接的信号强度
func (hc *DefaultHealthChecker) getRSSI(conn Connection) int {
	// 这里应该调用底层蓝牙适配器获取实际的 RSSI 值
	// 目前返回模拟值，实际实现需要与蓝牙适配器集成
	metrics := conn.GetMetrics()

	// 基于连接质量估算 RSSI
	if metrics.ErrorCount > 10 {
		return -90 // 信号很弱
	} else if metrics.ErrorCount > 5 {
		return -70 // 信号一般
	} else {
		return -50 // 信号良好
	}
}

// evaluateHealth 评估连接健康状态
func (hc *DefaultHealthChecker) evaluateHealth(heartbeat heartbeatResult, metrics ConnectionMetrics) bool {
	// 心跳失败
	if !heartbeat.success {
		return false
	}

	// 信号强度过低
	if heartbeat.rssi < hc.rssiThreshold {
		return false
	}

	// 延迟过高
	if heartbeat.latency > hc.latencyThreshold {
		return false
	}

	// 错误计数过高
	if int(metrics.ErrorCount) > hc.maxErrorCount {
		return false
	}

	// 检查最后活动时间
	if time.Since(metrics.LastActivity) > 5*time.Minute {
		return false
	}

	return true
}

// generateStatusMessage 生成状态描述消息
func (hc *DefaultHealthChecker) generateStatusMessage(isHealthy bool, heartbeat heartbeatResult, metrics ConnectionMetrics) string {
	if !isHealthy {
		if !heartbeat.success {
			return "心跳检查失败"
		}
		if heartbeat.rssi < hc.rssiThreshold {
			return "信号强度过低"
		}
		if heartbeat.latency > hc.latencyThreshold {
			return "延迟过高"
		}
		if int(metrics.ErrorCount) > hc.maxErrorCount {
			return "错误率过高"
		}
		if time.Since(metrics.LastActivity) > 5*time.Minute {
			return "连接不活跃"
		}
		return "健康状态异常"
	}
	return "连接健康"
}

// collectSystemMetrics 收集系统指标
func (hc *DefaultHealthChecker) collectSystemMetrics() SystemMetrics {
	// 使用性能收集器获取系统指标
	metrics := hc.performanceCollector.GetMetrics()
	return metrics.SystemMetrics
}

// generateRecommendations 生成健康建议
func (hc *DefaultHealthChecker) generateRecommendations(statuses []HealthStatus) []string {
	recommendations := make([]string, 0)

	unhealthyCount := 0
	lowRSSICount := 0
	highLatencyCount := 0

	for _, status := range statuses {
		if !status.IsHealthy {
			unhealthyCount++

			if status.RSSI < hc.rssiThreshold {
				lowRSSICount++
			}
			if status.Latency > hc.latencyThreshold {
				highLatencyCount++
			}
		}
	}

	if unhealthyCount > 0 {
		recommendations = append(recommendations,
			"检测到不健康的连接，建议检查设备状态")
	}

	if lowRSSICount > 0 {
		recommendations = append(recommendations,
			"多个设备信号强度较低，建议调整设备位置或检查环境干扰")
	}

	if highLatencyCount > 0 {
		recommendations = append(recommendations,
			"检测到高延迟连接，建议检查网络环境或重新连接")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "所有连接状态良好")
	}

	return recommendations
}

// SetHeartbeatTimeout 设置心跳超时时间
func (hc *DefaultHealthChecker) SetHeartbeatTimeout(timeout time.Duration) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.heartbeatTimeout = timeout
}

// SetMaxErrorCount 设置最大错误计数阈值
func (hc *DefaultHealthChecker) SetMaxErrorCount(count int) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.maxErrorCount = count
}

// SetRSSIThreshold 设置信号强度阈值
func (hc *DefaultHealthChecker) SetRSSIThreshold(threshold int) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.rssiThreshold = threshold
}

// SetLatencyThreshold 设置延迟阈值
func (hc *DefaultHealthChecker) SetLatencyThreshold(threshold time.Duration) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.latencyThreshold = threshold
}

// GetPerformanceMetrics 获取性能指标
func (hc *DefaultHealthChecker) GetPerformanceMetrics() PerformanceMetrics {
	return hc.performanceCollector.GetMetrics()
}

// GetHistoricalPerformanceMetrics 获取历史性能指标
func (hc *DefaultHealthChecker) GetHistoricalPerformanceMetrics(duration time.Duration) []PerformanceMetrics {
	return hc.performanceCollector.GetHistoricalMetrics(duration)
}

// StartPerformanceCollection 开始性能指标收集
func (hc *DefaultHealthChecker) StartPerformanceCollection(ctx context.Context, interval time.Duration) {
	hc.performanceCollector.StartCollection(ctx, interval)
}

// StopPerformanceCollection 停止性能指标收集
func (hc *DefaultHealthChecker) StopPerformanceCollection() {
	hc.performanceCollector.StopCollection()
}

// RegisterMetricsCallback 注册性能指标回调
func (hc *DefaultHealthChecker) RegisterMetricsCallback(callback MetricsCallback) {
	hc.performanceCollector.RegisterMetricsCallback(callback)
}
