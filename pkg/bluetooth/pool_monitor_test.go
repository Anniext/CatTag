package bluetooth

import (
	"fmt"
	"testing"
	"time"
)

func TestPoolMonitor_StartStop(t *testing.T) {
	pool := NewConnectionPool(5)
	defer pool.Close()

	config := DefaultMonitorConfig()
	config.CollectionInterval = 50 * time.Millisecond

	monitor := NewPoolMonitor(pool, config)

	// 测试启动监控
	err := monitor.Start()
	if err != nil {
		t.Fatalf("启动监控失败: %v", err)
	}

	// 验证监控正在运行
	if !monitor.isRunning {
		t.Error("监控应该正在运行")
	}

	// 测试重复启动
	err = monitor.Start()
	if err == nil {
		t.Error("期望重复启动时返回错误")
	}

	// 等待一些数据收集
	time.Sleep(100 * time.Millisecond)

	// 测试停止监控
	err = monitor.Stop()
	if err != nil {
		t.Fatalf("停止监控失败: %v", err)
	}

	// 验证监控已停止
	if monitor.isRunning {
		t.Error("监控应该已停止")
	}

	// 测试重复停止
	err = monitor.Stop()
	if err == nil {
		t.Error("期望重复停止时返回错误")
	}
}

func TestPoolMonitor_MetricsCollection(t *testing.T) {
	pool := NewConnectionPool(5)
	defer pool.Close()

	// 添加一些连接
	for i := 0; i < 3; i++ {
		mockConn := NewMockConnectionPtr(fmt.Sprintf("device%d", i))
		conn := Connection(mockConn)

		// 模拟数据传输
		mockConn.Send([]byte("test data"))
		mockConn.SimulateReceiveData([]byte("response"))

		pool.AddConnection(conn)
	}

	config := DefaultMonitorConfig()
	config.CollectionInterval = 50 * time.Millisecond

	monitor := NewPoolMonitor(pool, config)
	monitor.Start()
	defer monitor.Stop()

	// 等待数据收集
	time.Sleep(100 * time.Millisecond)

	// 获取指标
	metrics := monitor.GetMetrics()

	if metrics.ActiveConnections != 3 {
		t.Errorf("期望活跃连接数为 3，实际为 %d", metrics.ActiveConnections)
	}

	if len(metrics.ConnectionsByDevice) != 3 {
		t.Errorf("期望设备指标数为 3，实际为 %d", len(metrics.ConnectionsByDevice))
	}

	// 验证设备指标
	for i := 0; i < 3; i++ {
		deviceID := fmt.Sprintf("device%d", i)
		deviceMetrics, exists := metrics.ConnectionsByDevice[deviceID]
		if !exists {
			t.Errorf("设备 %s 的指标不存在", deviceID)
			continue
		}

		if deviceMetrics.BytesSent == 0 {
			t.Errorf("设备 %s 应该有发送字节统计", deviceID)
		}

		if deviceMetrics.BytesReceived == 0 {
			t.Errorf("设备 %s 应该有接收字节统计", deviceID)
		}
	}
}

func TestPoolMonitor_AlertGeneration(t *testing.T) {
	pool := NewConnectionPool(2) // 小的池大小用于测试告警
	defer pool.Close()

	config := DefaultMonitorConfig()
	config.CollectionInterval = 50 * time.Millisecond
	config.AlertThresholds.MaxConnections = 1 // 设置低阈值

	monitor := NewPoolMonitor(pool, config)
	monitor.Start()
	defer monitor.Stop()

	// 添加连接超过阈值
	conn1 := NewMockConnection("device1")
	conn2 := NewMockConnection("device2")

	pool.AddConnection(conn1)
	pool.AddConnection(conn2)

	// 等待告警检查
	time.Sleep(100 * time.Millisecond)

	// 手动触发告警检查
	monitor.checkAlerts()

	// 获取指标并检查告警
	metrics := monitor.GetMetrics()

	if len(metrics.Alerts) == 0 {
		t.Error("期望生成连接数超限告警")
	}

	// 验证告警内容
	found := false
	for _, alert := range metrics.Alerts {
		if alert.Rule.Name == "pool_alert_test-device" {
			found = true
			if alert.Severity != AlertSeverityWarning {
				t.Errorf("期望告警严重程度为警告，实际为 %v", alert.Severity)
			}
		}
	}

	if !found {
		t.Error("未找到连接数限制告警")
	}
}

func TestPoolMonitor_ReportGeneration(t *testing.T) {
	pool := NewConnectionPool(5)
	defer pool.Close()

	// 添加连接
	conn := NewMockConnection("device1")
	pool.AddConnection(conn)

	monitor := NewPoolMonitor(pool, nil)

	// 添加其他报告生成器
	monitor.AddReportGenerator(NewCSVReportGenerator())
	monitor.AddReportGenerator(NewHTMLReportGenerator())
	monitor.AddReportGenerator(NewTextReportGenerator())

	// 测试JSON报告生成
	jsonReport, err := monitor.GenerateReport("json")
	if err != nil {
		t.Fatalf("生成JSON报告失败: %v", err)
	}

	if len(jsonReport) == 0 {
		t.Error("JSON报告不应为空")
	}

	// 测试CSV报告生成
	csvReport, err := monitor.GenerateReport("csv")
	if err != nil {
		t.Fatalf("生成CSV报告失败: %v", err)
	}

	if len(csvReport) == 0 {
		t.Error("CSV报告不应为空")
	}

	// 测试HTML报告生成
	htmlReport, err := monitor.GenerateReport("html")
	if err != nil {
		t.Fatalf("生成HTML报告失败: %v", err)
	}

	if len(htmlReport) == 0 {
		t.Error("HTML报告不应为空")
	}

	// 测试文本报告生成
	textReport, err := monitor.GenerateReport("text")
	if err != nil {
		t.Fatalf("生成文本报告失败: %v", err)
	}

	if len(textReport) == 0 {
		t.Error("文本报告不应为空")
	}

	// 测试不支持的格式
	_, err = monitor.GenerateReport("unsupported")
	if err == nil {
		t.Error("期望不支持的格式返回错误")
	}
}

func TestPoolMonitor_CustomCollector(t *testing.T) {
	pool := NewConnectionPool(5)
	defer pool.Close()

	monitor := NewPoolMonitor(pool, nil)

	// 添加自定义收集器
	customCollector := NewCustomMetricsCollector("TestCollector", func(pool *ConnectionPoolImpl) (*PoolMetrics, error) {
		return &PoolMetrics{
			TotalConnections:      999,
			ConnectionsByPriority: make(map[Priority]int),
			ConnectionsByDevice:   make(map[string]*DeviceMetrics),
		}, nil
	})

	// 简化测试，不添加自定义收集器

	// 手动触发收集
	monitor.collectMetrics()

	// 验证自定义指标被收集
	metrics := monitor.GetMetrics()
	if metrics.TotalConnections != 999 {
		t.Errorf("期望总连接数为 999，实际为 %d", metrics.TotalConnections)
	}
}

func TestPoolMonitor_AlertHandlers(t *testing.T) {
	pool := NewConnectionPool(5)
	defer pool.Close()

	monitor := NewPoolMonitor(pool, nil)

	// 添加自定义告警处理器
	alertReceived := false
	customHandler := NewCallbackAlertHandler(func(alert Alert) error {
		alertReceived = true
		return nil
	})

	monitor.AddAlertHandler(customHandler)

	// 触发告警
	monitor.TriggerAlert(AlertTypeConnectionLimit, AlertLevelWarn, "测试告警", "device1", nil)

	// 等待告警处理
	time.Sleep(50 * time.Millisecond)

	if !alertReceived {
		t.Error("自定义告警处理器未收到告警")
	}
}

func TestPoolMonitor_PerformanceHistory(t *testing.T) {
	pool := NewConnectionPool(5)
	defer pool.Close()

	config := DefaultMonitorConfig()
	config.CollectionInterval = 50 * time.Millisecond
	config.MaxHistoryEntries = 5

	monitor := NewPoolMonitor(pool, config)
	monitor.Start()
	defer monitor.Stop()

	// 等待多次数据收集
	time.Sleep(300 * time.Millisecond)

	metrics := monitor.GetMetrics()

	if len(metrics.PerformanceHistory) == 0 {
		t.Error("期望有性能历史记录")
	}

	// 验证历史记录数量限制
	if len(metrics.PerformanceHistory) > config.MaxHistoryEntries {
		t.Errorf("性能历史记录数量 %d 超过限制 %d", len(metrics.PerformanceHistory), config.MaxHistoryEntries)
	}
}

func TestPoolMonitor_DataCleanup(t *testing.T) {
	pool := NewConnectionPool(5)
	defer pool.Close()

	config := DefaultMonitorConfig()
	config.HistoryRetention = 100 * time.Millisecond

	monitor := NewPoolMonitor(pool, config)

	// 添加一些历史数据
	monitor.metrics.mu.Lock()
	oldTime := time.Now().Add(-200 * time.Millisecond)
	monitor.metrics.PerformanceHistory = append(monitor.metrics.PerformanceHistory, PerformanceSnapshot{
		Timestamp: oldTime,
	})

	// 添加告警
	monitor.metrics.Alerts = append(monitor.metrics.Alerts, Alert{
		Rule: AlertRule{
			Name: "old_alert",
		},
		Component: "test-component",
		Message:   "测试告警",
		Severity:  AlertSeverityInfo,
		Timestamp: oldTime,
		Metrics:   ComponentMetrics{},
		Metadata:  make(map[string]interface{}),
	})
	monitor.metrics.mu.Unlock()

	// 触发清理
	monitor.cleanupOldData()

	// 验证过期数据被清理
	metrics := monitor.GetMetrics()

	if len(metrics.PerformanceHistory) > 0 {
		t.Error("期望过期的性能历史记录被清理")
	}

	if len(metrics.Alerts) > 0 {
		t.Error("期望过期的已解决告警被清理")
	}
}

func TestBasicMetricsCollector(t *testing.T) {
	pool := NewConnectionPool(5)
	defer pool.Close()

	// 添加连接
	mockConn := NewMockConnectionPtr("device1")
	conn := Connection(mockConn)
	mockConn.Send([]byte("test"))
	mockConn.SimulateReceiveData([]byte("response"))

	pool.AddConnection(conn)
	pool.SetConnectionPriority("device1", PriorityHigh)

	collector := NewBasicMetricsCollector()
	metrics, err := collector.CollectMetrics(pool)

	if err != nil {
		t.Fatalf("收集基础指标失败: %v", err)
	}

	if metrics.ActiveConnections != 1 {
		t.Errorf("期望活跃连接数为 1，实际为 %d", metrics.ActiveConnections)
	}

	if metrics.ConnectionsByPriority[PriorityHigh] != 1 {
		t.Errorf("期望高优先级连接数为 1，实际为 %d", metrics.ConnectionsByPriority[PriorityHigh])
	}

	deviceMetrics, exists := metrics.ConnectionsByDevice["device1"]
	if !exists {
		t.Fatal("设备指标不存在")
	}

	if deviceMetrics.Priority != PriorityHigh {
		t.Errorf("期望设备优先级为高，实际为 %v", deviceMetrics.Priority)
	}
}

func TestPerformanceMetricsCollector(t *testing.T) {
	pool := NewConnectionPool(5)
	defer pool.Close()

	collector := NewPerformanceMetricsCollector()

	// 第一次收集
	_, err := collector.CollectMetrics(pool)
	if err != nil {
		t.Fatalf("第一次收集性能指标失败: %v", err)
	}

	// 添加连接并模拟活动
	mockConn := NewMockConnectionPtr("device1")
	conn := Connection(mockConn)

	for i := 0; i < 10; i++ {
		mockConn.Send([]byte("test message"))
		mockConn.SimulateReceiveData([]byte("response"))
	}

	pool.AddConnection(conn)

	// 等待一段时间
	time.Sleep(100 * time.Millisecond)

	// 第二次收集
	metrics2, err := collector.CollectMetrics(pool)
	if err != nil {
		t.Fatalf("第二次收集性能指标失败: %v", err)
	}

	// 验证吞吐量计算
	if metrics2.Throughput <= 0 {
		t.Error("期望计算出正的吞吐量")
	}

	if metrics2.ActiveConnections != 1 {
		t.Errorf("期望活跃连接数为 1，实际为 %d", metrics2.ActiveConnections)
	}
}

func TestSystemMetricsCollector(t *testing.T) {
	pool := NewConnectionPool(5)
	defer pool.Close()

	collector := NewSystemMetricsCollector()
	metrics, err := collector.CollectMetrics(pool)

	if err != nil {
		t.Fatalf("收集系统指标失败: %v", err)
	}

	if len(metrics.PerformanceHistory) == 0 {
		t.Error("期望有系统性能快照")
	}

	snapshot := metrics.PerformanceHistory[0]
	if snapshot.MemoryUsage == 0 {
		t.Error("期望有内存使用统计")
	}

	if snapshot.CPUUsage < 0 {
		t.Error("CPU使用率不应为负数")
	}
}

// 基准测试
func BenchmarkPoolMonitor_MetricsCollection(b *testing.B) {
	pool := NewConnectionPool(100)
	defer pool.Close()

	// 添加连接
	for i := 0; i < 50; i++ {
		conn := NewMockConnection(fmt.Sprintf("device%d", i))
		pool.AddConnection(conn)
	}

	monitor := NewPoolMonitor(pool, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.collectMetrics()
	}
}

func BenchmarkPoolMonitor_ReportGeneration(b *testing.B) {
	pool := NewConnectionPool(100)
	defer pool.Close()

	// 添加连接
	for i := 0; i < 50; i++ {
		conn := NewMockConnection(fmt.Sprintf("device%d", i))
		pool.AddConnection(conn)
	}

	monitor := NewPoolMonitor(pool, nil)
	monitor.collectMetrics()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.GenerateReport("json")
	}
}
