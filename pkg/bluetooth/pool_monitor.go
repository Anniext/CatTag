package bluetooth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PoolMonitor 连接池监控器
type PoolMonitor struct {
	mu               sync.RWMutex
	pool             *ConnectionPoolImpl
	metrics          *PoolMetrics
	config           *MonitorConfig
	ctx              context.Context
	cancel           context.CancelFunc
	collectors       []MetricsCollector
	alertHandlers    []AlertHandler
	reportGenerators []ReportGenerator
	isRunning        bool
}

// PoolMetrics 连接池详细指标
type PoolMetrics struct {
	mu                    sync.RWMutex
	StartTime             time.Time                 `json:"start_time"`              // 监控开始时间
	TotalConnections      uint64                    `json:"total_connections"`       // 总连接数
	ActiveConnections     int                       `json:"active_connections"`      // 当前活跃连接数
	PeakConnections       int                       `json:"peak_connections"`        // 峰值连接数
	ConnectionsCreated    uint64                    `json:"connections_created"`     // 创建的连接数
	ConnectionsClosed     uint64                    `json:"connections_closed"`      // 关闭的连接数
	ConnectionsRejected   uint64                    `json:"connections_rejected"`    // 拒绝的连接数
	TotalBytesTransferred uint64                    `json:"total_bytes_transferred"` // 总传输字节数
	TotalMessages         uint64                    `json:"total_messages"`          // 总消息数
	AverageLatency        time.Duration             `json:"average_latency"`         // 平均延迟
	ErrorRate             float64                   `json:"error_rate"`              // 错误率
	Throughput            float64                   `json:"throughput"`              // 吞吐量 (messages/second)
	ConnectionsByPriority map[Priority]int          `json:"connections_by_priority"` // 按优先级分组的连接数
	ConnectionsByDevice   map[string]*DeviceMetrics `json:"connections_by_device"`   // 按设备分组的指标
	PerformanceHistory    []PerformanceSnapshot     `json:"performance_history"`     // 性能历史记录
	Alerts                []Alert                   `json:"alerts"`                  // 告警记录
}

// DeviceMetrics 设备级别的指标
type DeviceMetrics struct {
	DeviceID         string        `json:"device_id"`         // 设备ID
	ConnectionTime   time.Time     `json:"connection_time"`   // 连接时间
	LastActivity     time.Time     `json:"last_activity"`     // 最后活动时间
	BytesSent        uint64        `json:"bytes_sent"`        // 发送字节数
	BytesReceived    uint64        `json:"bytes_received"`    // 接收字节数
	MessagesSent     uint64        `json:"messages_sent"`     // 发送消息数
	MessagesReceived uint64        `json:"messages_received"` // 接收消息数
	ErrorCount       uint64        `json:"error_count"`       // 错误计数
	AverageLatency   time.Duration `json:"average_latency"`   // 平均延迟
	RSSI             int           `json:"rssi"`              // 信号强度
	Priority         Priority      `json:"priority"`          // 连接优先级
	HealthScore      float64       `json:"health_score"`      // 健康评分
	UseCount         uint64        `json:"use_count"`         // 使用次数
}

// PerformanceSnapshot 性能快照
type PerformanceSnapshot struct {
	Timestamp         time.Time     `json:"timestamp"`          // 时间戳
	ActiveConnections int           `json:"active_connections"` // 活跃连接数
	Throughput        float64       `json:"throughput"`         // 吞吐量
	AverageLatency    time.Duration `json:"average_latency"`    // 平均延迟
	ErrorRate         float64       `json:"error_rate"`         // 错误率
	MemoryUsage       uint64        `json:"memory_usage"`       // 内存使用量
	CPUUsage          float64       `json:"cpu_usage"`          // CPU使用率
}

// AlertType 告警类型
type AlertType int

const (
	AlertTypeConnectionLimit  AlertType = iota // 连接数限制
	AlertTypeHighLatency                       // 高延迟
	AlertTypeHighErrorRate                     // 高错误率
	AlertTypeDeviceOffline                     // 设备离线
	AlertTypeMemoryUsage                       // 内存使用
	AlertTypeCPUUsage                          // CPU使用
	AlertTypeConnectionFailed                  // 连接失败
)

// AlertLevel 告警级别
type AlertLevel int

const (
	AlertLevelInfo     AlertLevel = iota // 信息
	AlertLevelWarn                       // 警告
	AlertLevelError                      // 错误
	AlertLevelCritical                   // 严重
)

// MonitorConfig 监控配置
type MonitorConfig struct {
	CollectionInterval    time.Duration   `json:"collection_interval"`     // 数据收集间隔
	ReportInterval        time.Duration   `json:"report_interval"`         // 报告生成间隔
	HistoryRetention      time.Duration   `json:"history_retention"`       // 历史数据保留时间
	MaxHistoryEntries     int             `json:"max_history_entries"`     // 最大历史记录数
	AlertThresholds       AlertThresholds `json:"alert_thresholds"`        // 告警阈值
	EnablePerformanceLog  bool            `json:"enable_performance_log"`  // 启用性能日志
	EnableDetailedMetrics bool            `json:"enable_detailed_metrics"` // 启用详细指标
}

// AlertThresholds 告警阈值配置
type AlertThresholds struct {
	MaxConnections    int           `json:"max_connections"`    // 最大连接数阈值
	MaxLatency        time.Duration `json:"max_latency"`        // 最大延迟阈值
	MaxErrorRate      float64       `json:"max_error_rate"`     // 最大错误率阈值
	MaxMemoryUsage    uint64        `json:"max_memory_usage"`   // 最大内存使用阈值
	MaxCPUUsage       float64       `json:"max_cpu_usage"`      // 最大CPU使用阈值
	MinHealthScore    float64       `json:"min_health_score"`   // 最小健康评分阈值
	ConnectionTimeout time.Duration `json:"connection_timeout"` // 连接超时阈值
}

// ReportGenerator 报告生成器接口
type ReportGenerator interface {
	GenerateReport(metrics *PoolMetrics) ([]byte, error)
	GetReportFormat() string
}

// NewPoolMonitor 创建连接池监控器
func NewPoolMonitor(pool *ConnectionPoolImpl, config *MonitorConfig) *PoolMonitor {
	if config == nil {
		config = DefaultMonitorConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	monitor := &PoolMonitor{
		pool:   pool,
		config: config,
		ctx:    ctx,
		cancel: cancel,
		metrics: &PoolMetrics{
			StartTime:             time.Now(),
			ConnectionsByPriority: make(map[Priority]int),
			ConnectionsByDevice:   make(map[string]*DeviceMetrics),
			PerformanceHistory:    make([]PerformanceSnapshot, 0),
			Alerts:                make([]Alert, 0),
		},
		collectors:       make([]MetricsCollector, 0),
		alertHandlers:    make([]AlertHandler, 0),
		reportGenerators: make([]ReportGenerator, 0),
	}

	// 默认收集器已在结构体中初始化

	// 添加默认告警处理器
	monitor.AddAlertHandler(NewLogAlertHandler())

	// 添加默认报告生成器
	monitor.AddReportGenerator(NewJSONReportGenerator())

	return monitor
}

// DefaultMonitorConfig 默认监控配置
func DefaultMonitorConfig() *MonitorConfig {
	return &MonitorConfig{
		CollectionInterval:    30 * time.Second,
		ReportInterval:        5 * time.Minute,
		HistoryRetention:      24 * time.Hour,
		MaxHistoryEntries:     1000,
		EnablePerformanceLog:  true,
		EnableDetailedMetrics: true,
		AlertThresholds: AlertThresholds{
			MaxConnections:    100,
			MaxLatency:        5 * time.Second,
			MaxErrorRate:      0.05,               // 5%
			MaxMemoryUsage:    1024 * 1024 * 1024, // 1GB
			MaxCPUUsage:       0.8,                // 80%
			MinHealthScore:    0.7,                // 70%
			ConnectionTimeout: 30 * time.Second,
		},
	}
}

// Start 启动监控
func (pm *PoolMonitor) Start() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.isRunning {
		return NewBluetoothError(ErrCodeAlreadyExists, "监控器已在运行")
	}

	pm.isRunning = true

	// 启动数据收集协程
	go pm.startDataCollection()

	// 启动报告生成协程
	go pm.startReportGeneration()

	// 启动告警检查协程
	go pm.startAlertChecking()

	return nil
}

// Stop 停止监控
func (pm *PoolMonitor) Stop() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.isRunning {
		return NewBluetoothError(ErrCodeGeneral, "监控器未在运行")
	}

	pm.cancel()
	pm.isRunning = false

	return nil
}

// GetMetrics 获取当前指标
func (pm *PoolMonitor) GetMetrics() *PoolMetrics {
	pm.metrics.mu.RLock()
	defer pm.metrics.mu.RUnlock()

	// 返回指标的深拷贝
	return pm.copyMetrics()
}

// AddCollector 添加指标收集器
func (pm *PoolMonitor) AddCollector(collector MetricsCollector) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.collectors = append(pm.collectors, collector)
}

// AddAlertHandler 添加告警处理器
func (pm *PoolMonitor) AddAlertHandler(handler AlertHandler) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.alertHandlers = append(pm.alertHandlers, handler)
}

// AddReportGenerator 添加报告生成器
func (pm *PoolMonitor) AddReportGenerator(generator ReportGenerator) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.reportGenerators = append(pm.reportGenerators, generator)
}

// GenerateReport 生成监控报告
func (pm *PoolMonitor) GenerateReport(format string) ([]byte, error) {
	metrics := pm.GetMetrics()

	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, generator := range pm.reportGenerators {
		if generator.GetReportFormat() == format {
			return generator.GenerateReport(metrics)
		}
	}

	return nil, NewBluetoothError(ErrCodeNotFound, fmt.Sprintf("未找到格式为 %s 的报告生成器", format))
}

// TriggerAlert 触发告警
func (pm *PoolMonitor) TriggerAlert(alertType AlertType, level AlertLevel, message string, deviceID string, metadata interface{}) {
	alert := Alert{
		Rule: AlertRule{
			Name: fmt.Sprintf("pool_alert_%s", deviceID),
		},
		Component: deviceID,
		Message:   message,
		Severity:  AlertSeverityWarning, // 默认警告级别
		Timestamp: time.Now(),
		Metrics:   ComponentMetrics{}, // 空的组件指标
		Metadata:  metadata.(map[string]interface{}),
	}

	pm.metrics.mu.Lock()
	pm.metrics.Alerts = append(pm.metrics.Alerts, alert)
	pm.metrics.mu.Unlock()

	// 通知所有告警处理器
	pm.mu.RLock()
	handlers := pm.alertHandlers
	pm.mu.RUnlock()

	for _, handler := range handlers {
		go func(h AlertHandler) {
			if err := h.Handle(alert); err != nil {
				// 记录处理错误
			}
		}(handler)
	}
}

// startDataCollection 启动数据收集
func (pm *PoolMonitor) startDataCollection() {
	ticker := time.NewTicker(pm.config.CollectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			pm.collectMetrics()
		}
	}
}

// startReportGeneration 启动报告生成
func (pm *PoolMonitor) startReportGeneration() {
	ticker := time.NewTicker(pm.config.ReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			pm.generatePeriodicReports()
		}
	}
}

// startAlertChecking 启动告警检查
func (pm *PoolMonitor) startAlertChecking() {
	ticker := time.NewTicker(pm.config.CollectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			pm.checkAlerts()
		}
	}
}

// collectMetrics 收集指标
func (pm *PoolMonitor) collectMetrics() {
	// 简化实现，直接更新基础指标
	// 在实际实现中，这里会调用真实的收集器

	// 添加性能快照
	pm.addPerformanceSnapshot()

	// 清理过期数据
	pm.cleanupOldData()
}

// updateMetrics 更新指标
func (pm *PoolMonitor) updateMetrics(newMetrics *PoolMetrics) {
	pm.metrics.mu.Lock()
	defer pm.metrics.mu.Unlock()

	// 更新基本指标
	pm.metrics.ActiveConnections = newMetrics.ActiveConnections
	if newMetrics.TotalConnections > 0 {
		pm.metrics.TotalConnections = newMetrics.TotalConnections
	}
	pm.metrics.TotalBytesTransferred = newMetrics.TotalBytesTransferred
	pm.metrics.AverageLatency = newMetrics.AverageLatency
	pm.metrics.ErrorRate = newMetrics.ErrorRate
	pm.metrics.Throughput = newMetrics.Throughput

	// 更新峰值连接数
	if newMetrics.ActiveConnections > pm.metrics.PeakConnections {
		pm.metrics.PeakConnections = newMetrics.ActiveConnections
	}

	// 更新按优先级分组的连接数
	for priority, count := range newMetrics.ConnectionsByPriority {
		pm.metrics.ConnectionsByPriority[priority] = count
	}

	// 更新设备级别指标
	for deviceID, deviceMetrics := range newMetrics.ConnectionsByDevice {
		pm.metrics.ConnectionsByDevice[deviceID] = deviceMetrics
	}
}

// addPerformanceSnapshot 添加性能快照
func (pm *PoolMonitor) addPerformanceSnapshot() {
	pm.metrics.mu.Lock()
	defer pm.metrics.mu.Unlock()

	snapshot := PerformanceSnapshot{
		Timestamp:         time.Now(),
		ActiveConnections: pm.metrics.ActiveConnections,
		Throughput:        pm.metrics.Throughput,
		AverageLatency:    pm.metrics.AverageLatency,
		ErrorRate:         pm.metrics.ErrorRate,
		// TODO: 添加内存和CPU使用率收集
	}

	pm.metrics.PerformanceHistory = append(pm.metrics.PerformanceHistory, snapshot)

	// 限制历史记录数量
	if len(pm.metrics.PerformanceHistory) > pm.config.MaxHistoryEntries {
		pm.metrics.PerformanceHistory = pm.metrics.PerformanceHistory[1:]
	}
}

// cleanupOldData 清理过期数据
func (pm *PoolMonitor) cleanupOldData() {
	pm.metrics.mu.Lock()
	defer pm.metrics.mu.Unlock()

	cutoff := time.Now().Add(-pm.config.HistoryRetention)

	// 清理性能历史记录
	var validHistory []PerformanceSnapshot
	for _, snapshot := range pm.metrics.PerformanceHistory {
		if snapshot.Timestamp.After(cutoff) {
			validHistory = append(validHistory, snapshot)
		}
	}
	pm.metrics.PerformanceHistory = validHistory

	// 简化实现，清理过期告警
	// 在实际实现中，这里会根据告警的时间戳进行清理
	var activeAlerts []Alert
	for _, alert := range pm.metrics.Alerts {
		if time.Since(alert.Timestamp) < 24*time.Hour { // 保留24小时内的告警
			activeAlerts = append(activeAlerts, alert)
		}
	}
	pm.metrics.Alerts = activeAlerts
}

// checkAlerts 检查告警条件
func (pm *PoolMonitor) checkAlerts() {
	metrics := pm.GetMetrics()
	thresholds := pm.config.AlertThresholds

	// 检查连接数告警
	if metrics.ActiveConnections > thresholds.MaxConnections {
		pm.TriggerAlert(
			AlertTypeConnectionLimit,
			AlertLevelWarn,
			fmt.Sprintf("活跃连接数 (%d) 超过阈值 (%d)", metrics.ActiveConnections, thresholds.MaxConnections),
			"",
			map[string]interface{}{
				"current_connections": metrics.ActiveConnections,
				"max_connections":     thresholds.MaxConnections,
			},
		)
	}

	// 检查延迟告警
	if metrics.AverageLatency > thresholds.MaxLatency {
		pm.TriggerAlert(
			AlertTypeHighLatency,
			AlertLevelWarn,
			fmt.Sprintf("平均延迟 (%v) 超过阈值 (%v)", metrics.AverageLatency, thresholds.MaxLatency),
			"",
			map[string]interface{}{
				"current_latency": metrics.AverageLatency,
				"max_latency":     thresholds.MaxLatency,
			},
		)
	}

	// 检查错误率告警
	if metrics.ErrorRate > thresholds.MaxErrorRate {
		pm.TriggerAlert(
			AlertTypeHighErrorRate,
			AlertLevelError,
			fmt.Sprintf("错误率 (%.2f%%) 超过阈值 (%.2f%%)", metrics.ErrorRate*100, thresholds.MaxErrorRate*100),
			"",
			map[string]interface{}{
				"current_error_rate": metrics.ErrorRate,
				"max_error_rate":     thresholds.MaxErrorRate,
			},
		)
	}

	// 检查设备级别告警
	for deviceID, deviceMetrics := range metrics.ConnectionsByDevice {
		if deviceMetrics.HealthScore < thresholds.MinHealthScore {
			pm.TriggerAlert(
				AlertTypeDeviceOffline,
				AlertLevelWarn,
				fmt.Sprintf("设备 %s 健康评分 (%.2f) 低于阈值 (%.2f)", deviceID, deviceMetrics.HealthScore, thresholds.MinHealthScore),
				deviceID,
				map[string]interface{}{
					"current_health_score": deviceMetrics.HealthScore,
					"min_health_score":     thresholds.MinHealthScore,
				},
			)
		}
	}
}

// generatePeriodicReports 生成周期性报告
func (pm *PoolMonitor) generatePeriodicReports() {
	pm.mu.RLock()
	generators := pm.reportGenerators
	pm.mu.RUnlock()

	for _, generator := range generators {
		if report, err := pm.GenerateReport(generator.GetReportFormat()); err == nil {
			// 这里可以将报告保存到文件或发送到外部系统
			_ = report
		}
	}
}

// copyMetrics 复制指标数据
func (pm *PoolMonitor) copyMetrics() *PoolMetrics {
	// 创建深拷贝
	result := &PoolMetrics{
		StartTime:             pm.metrics.StartTime,
		TotalConnections:      pm.metrics.TotalConnections,
		ActiveConnections:     pm.metrics.ActiveConnections,
		PeakConnections:       pm.metrics.PeakConnections,
		ConnectionsCreated:    pm.metrics.ConnectionsCreated,
		ConnectionsClosed:     pm.metrics.ConnectionsClosed,
		ConnectionsRejected:   pm.metrics.ConnectionsRejected,
		TotalBytesTransferred: pm.metrics.TotalBytesTransferred,
		TotalMessages:         pm.metrics.TotalMessages,
		AverageLatency:        pm.metrics.AverageLatency,
		ErrorRate:             pm.metrics.ErrorRate,
		Throughput:            pm.metrics.Throughput,
		ConnectionsByPriority: make(map[Priority]int),
		ConnectionsByDevice:   make(map[string]*DeviceMetrics),
		PerformanceHistory:    make([]PerformanceSnapshot, len(pm.metrics.PerformanceHistory)),
		Alerts:                make([]Alert, len(pm.metrics.Alerts)),
	}

	// 复制映射数据
	for priority, count := range pm.metrics.ConnectionsByPriority {
		result.ConnectionsByPriority[priority] = count
	}

	for deviceID, metrics := range pm.metrics.ConnectionsByDevice {
		result.ConnectionsByDevice[deviceID] = &DeviceMetrics{
			DeviceID:         metrics.DeviceID,
			ConnectionTime:   metrics.ConnectionTime,
			LastActivity:     metrics.LastActivity,
			BytesSent:        metrics.BytesSent,
			BytesReceived:    metrics.BytesReceived,
			MessagesSent:     metrics.MessagesSent,
			MessagesReceived: metrics.MessagesReceived,
			ErrorCount:       metrics.ErrorCount,
			AverageLatency:   metrics.AverageLatency,
			RSSI:             metrics.RSSI,
			Priority:         metrics.Priority,
			HealthScore:      metrics.HealthScore,
			UseCount:         metrics.UseCount,
		}
	}

	// 复制切片数据
	copy(result.PerformanceHistory, pm.metrics.PerformanceHistory)
	copy(result.Alerts, pm.metrics.Alerts)

	return result
}
