package bluetooth

import (
	"context"
	"fmt"
	"time"
)

// Example_healthCheckerBasicUsage 展示健康检查器的基本使用方法
func Example_healthCheckerBasicUsage() {
	// 创建连接池
	pool := NewConnectionPool(5)
	defer pool.Close()

	// 创建健康检查器
	healthChecker := NewDefaultHealthChecker(pool)

	// 添加一些模拟连接
	conn1 := NewMockConnection("device1")
	conn2 := NewMockConnection("device2")
	pool.AddConnection(conn1)
	pool.AddConnection(conn2)

	// 注册健康状态回调
	healthChecker.RegisterHealthCallback(func(status HealthStatus) {
		fmt.Printf("设备 %s 健康状态: %t - %s\n",
			status.DeviceID, status.IsHealthy, status.Status)
	})

	// 开始监控
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	healthChecker.StartMonitoring(ctx, 50*time.Millisecond)

	// 检查单个连接的健康状态
	status := healthChecker.CheckConnection("device1")
	fmt.Printf("Device1 健康状态: %t\n", status.IsHealthy)

	// 获取完整的健康报告
	report := healthChecker.GetHealthReport()
	fmt.Printf("整体健康状态: %t\n", report.OverallHealth)
	fmt.Printf("监控设备数量: %d\n", len(report.DeviceStatuses))

	// 等待一些监控数据
	time.Sleep(100 * time.Millisecond)

	// 停止监控
	healthChecker.StopMonitoring()

	// Output:
	// Device1 健康状态: true
	// 整体健康状态: true
	// 监控设备数量: 2
}

// Example_performanceCollectorBasicUsage 展示性能收集器的基本使用方法
func Example_performanceCollectorBasicUsage() {
	// 创建连接池
	pool := NewConnectionPool(5)
	defer pool.Close()

	// 创建性能收集器
	collector := NewDefaultPerformanceCollector(pool)

	// 添加一些模拟连接
	conn1 := NewMockConnection("device1")
	conn2 := NewMockConnection("device2")
	pool.AddConnection(conn1)
	pool.AddConnection(conn2)

	// 注册性能指标回调
	collector.RegisterMetricsCallback(func(metrics PerformanceMetrics) {
		fmt.Printf("收集到性能指标，连接数: %d\n", len(metrics.ConnectionMetrics))
	})

	// 开始收集性能指标
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	collector.StartCollection(ctx, 30*time.Millisecond)

	// 等待收集一些数据
	time.Sleep(80 * time.Millisecond)

	// 获取当前性能指标
	metrics := collector.GetMetrics()
	fmt.Printf("当前连接数: %d\n", len(metrics.ConnectionMetrics))
	fmt.Printf("系统Goroutine数: %d\n", metrics.SystemMetrics.Goroutines)

	// 获取历史指标
	historical := collector.GetHistoricalMetrics(1 * time.Minute)
	fmt.Printf("历史记录数量: %d\n", len(historical))

	// 停止收集
	collector.StopCollection()

	// Output:
	// 当前连接数: 2
	// 系统Goroutine数: 1
	// 历史记录数量: 2
}

// Example_healthCheckerIntegratedUsage 展示健康检查器与性能收集器的集成使用
func Example_healthCheckerIntegratedUsage() {
	// 创建连接池
	pool := NewConnectionPool(5)
	defer pool.Close()

	// 创建健康检查器（自动包含性能收集器）
	healthChecker := NewDefaultHealthChecker(pool)

	// 添加模拟连接
	conn := NewMockConnection("device1")
	pool.AddConnection(conn)

	// 配置健康检查参数
	healthChecker.SetHeartbeatTimeout(1 * time.Second)
	healthChecker.SetRSSIThreshold(-70)
	healthChecker.SetLatencyThreshold(500 * time.Millisecond)

	// 同时启动健康监控和性能收集
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	healthChecker.StartMonitoring(ctx, 50*time.Millisecond)
	healthChecker.StartPerformanceCollection(ctx, 40*time.Millisecond)

	// 等待收集数据
	time.Sleep(100 * time.Millisecond)

	// 获取综合报告
	healthReport := healthChecker.GetHealthReport()
	performanceMetrics := healthChecker.GetPerformanceMetrics()

	fmt.Printf("健康设备数: %d\n", len(healthReport.DeviceStatuses))
	fmt.Printf("性能指标连接数: %d\n", len(performanceMetrics.ConnectionMetrics))
	fmt.Printf("建议数量: %d\n", len(healthReport.Recommendations))

	// 停止所有监控
	healthChecker.StopMonitoring()
	healthChecker.StopPerformanceCollection()

	// Output:
	// 健康设备数: 1
	// 性能指标连接数: 1
	// 建议数量: 1
}
