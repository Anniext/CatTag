package bluetooth

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestDefaultHealthChecker_StartStopMonitoring(t *testing.T) {
	// 创建连接池和健康检查器
	pool := NewConnectionPool(5)
	defer pool.Close()

	healthChecker := NewDefaultHealthChecker(pool)

	// 测试开始监控
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	healthChecker.StartMonitoring(ctx, 100*time.Millisecond)

	// 验证监控状态
	if !healthChecker.isMonitoring {
		t.Error("期望健康检查器处于监控状态")
	}

	// 测试停止监控
	healthChecker.StopMonitoring()

	// 等待一小段时间确保停止完成
	time.Sleep(50 * time.Millisecond)

	if healthChecker.isMonitoring {
		t.Error("期望健康检查器停止监控")
	}
}

func TestDefaultHealthChecker_CheckConnection(t *testing.T) {
	// 创建连接池和健康检查器
	pool := NewConnectionPool(5)
	defer pool.Close()

	healthChecker := NewDefaultHealthChecker(pool)

	// 测试检查不存在的连接
	status := healthChecker.CheckConnection("nonexistent")
	if status.IsHealthy {
		t.Error("期望不存在的连接状态为不健康")
	}
	if status.Status != "连接不存在" {
		t.Errorf("期望状态为 '连接不存在'，实际为 '%s'", status.Status)
	}

	// 添加一个模拟连接
	conn := NewMockConnection("device1")
	pool.AddConnection(conn)

	// 测试检查存在的连接
	status = healthChecker.CheckConnection("device1")
	if status.DeviceID != "device1" {
		t.Errorf("期望设备ID为 device1，实际为 %s", status.DeviceID)
	}
}

func TestDefaultHealthChecker_GetHealthReport(t *testing.T) {
	// 创建连接池和健康检查器
	pool := NewConnectionPool(5)
	defer pool.Close()

	healthChecker := NewDefaultHealthChecker(pool)

	// 添加一些模拟连接
	conn1 := NewMockConnection("device1")
	conn2 := NewMockConnection("device2")
	pool.AddConnection(conn1)
	pool.AddConnection(conn2)

	// 模拟一些健康状态
	healthChecker.deviceStatuses["device1"] = &HealthStatus{
		DeviceID:  "device1",
		IsHealthy: true,
		Status:    "连接健康",
	}
	healthChecker.deviceStatuses["device2"] = &HealthStatus{
		DeviceID:  "device2",
		IsHealthy: false,
		Status:    "信号强度过低",
	}

	// 获取健康报告
	report := healthChecker.GetHealthReport()

	// 验证报告内容
	if report.OverallHealth {
		t.Error("期望整体健康状态为 false（因为有不健康的设备）")
	}

	if len(report.DeviceStatuses) != 2 {
		t.Errorf("期望设备状态数量为 2，实际为 %d", len(report.DeviceStatuses))
	}

	if len(report.Recommendations) == 0 {
		t.Error("期望有健康建议")
	}
}

func TestDefaultHealthChecker_RegisterHealthCallback(t *testing.T) {
	// 创建连接池和健康检查器
	pool := NewConnectionPool(5)
	defer pool.Close()

	healthChecker := NewDefaultHealthChecker(pool)

	// 测试回调注册
	callbackCalled := false
	var receivedStatus HealthStatus

	callback := func(status HealthStatus) {
		callbackCalled = true
		receivedStatus = status
	}

	healthChecker.RegisterHealthCallback(callback)

	// 触发回调
	testStatus := HealthStatus{
		DeviceID:  "test-device",
		IsHealthy: true,
		Status:    "测试状态",
	}

	healthChecker.callbackManager.NotifyHealthStatus(testStatus)

	// 等待回调执行
	time.Sleep(10 * time.Millisecond)

	if !callbackCalled {
		t.Error("期望健康状态回调被调用")
	}

	if receivedStatus.DeviceID != "test-device" {
		t.Errorf("期望接收到的设备ID为 test-device，实际为 %s", receivedStatus.DeviceID)
	}
}

func TestDefaultHealthChecker_HeartbeatMechanism(t *testing.T) {
	// 创建连接池和健康检查器
	pool := NewConnectionPool(5)
	defer pool.Close()

	healthChecker := NewDefaultHealthChecker(pool)
	healthChecker.SetHeartbeatTimeout(100 * time.Millisecond)

	// 创建一个响应心跳的模拟连接
	mockConn := NewMockConnectionPtr("device1")
	mockConn.SetHeartbeatResponse(true) // 设置为响应心跳
	pool.AddConnection(mockConn)

	// 执行心跳检查
	result := healthChecker.performHeartbeat(mockConn)

	if !result.success {
		t.Error("期望心跳检查成功")
	}

	if result.latency <= 0 {
		t.Error("期望心跳延迟大于0")
	}

	// 测试心跳超时
	mockConn.SetHeartbeatResponse(false) // 设置为不响应心跳
	result = healthChecker.performHeartbeat(mockConn)

	if result.success {
		t.Error("期望心跳检查失败（超时）")
	}
}

func TestDefaultHealthChecker_HealthEvaluation(t *testing.T) {
	// 创建连接池和健康检查器
	pool := NewConnectionPool(5)
	defer pool.Close()

	healthChecker := NewDefaultHealthChecker(pool)

	// 测试健康连接的评估
	healthyHeartbeat := heartbeatResult{
		success:   true,
		timestamp: time.Now(),
		latency:   50 * time.Millisecond,
		rssi:      -60, // 良好信号
	}

	healthyMetrics := ConnectionMetrics{
		ErrorCount:   2,
		LastActivity: time.Now().Add(-1 * time.Minute),
	}

	isHealthy := healthChecker.evaluateHealth(healthyHeartbeat, healthyMetrics)
	if !isHealthy {
		t.Error("期望健康连接被评估为健康")
	}

	// 测试不健康连接的评估（信号弱）
	weakSignalHeartbeat := heartbeatResult{
		success:   true,
		timestamp: time.Now(),
		latency:   50 * time.Millisecond,
		rssi:      -90, // 弱信号
	}

	isHealthy = healthChecker.evaluateHealth(weakSignalHeartbeat, healthyMetrics)
	if isHealthy {
		t.Error("期望弱信号连接被评估为不健康")
	}

	// 测试不健康连接的评估（高延迟）
	highLatencyHeartbeat := heartbeatResult{
		success:   true,
		timestamp: time.Now(),
		latency:   2 * time.Second, // 高延迟
		rssi:      -60,
	}

	isHealthy = healthChecker.evaluateHealth(highLatencyHeartbeat, healthyMetrics)
	if isHealthy {
		t.Error("期望高延迟连接被评估为不健康")
	}
}

func TestDefaultHealthChecker_PerformanceIntegration(t *testing.T) {
	// 创建连接池和健康检查器
	pool := NewConnectionPool(5)
	defer pool.Close()

	healthChecker := NewDefaultHealthChecker(pool)

	// 测试性能指标集成
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 开始性能收集
	healthChecker.StartPerformanceCollection(ctx, 50*time.Millisecond)

	// 添加一些连接
	conn1 := NewMockConnection("device1")
	conn2 := NewMockConnection("device2")
	pool.AddConnection(conn1)
	pool.AddConnection(conn2)

	// 等待一些性能数据收集
	time.Sleep(100 * time.Millisecond)

	// 获取性能指标
	metrics := healthChecker.GetPerformanceMetrics()

	if metrics.Timestamp.IsZero() {
		t.Error("期望性能指标有有效的时间戳")
	}

	// 停止性能收集
	healthChecker.StopPerformanceCollection()
}

func TestDefaultHealthChecker_ConfigurationMethods(t *testing.T) {
	// 创建连接池和健康检查器
	pool := NewConnectionPool(5)
	defer pool.Close()

	healthChecker := NewDefaultHealthChecker(pool)

	// 测试配置方法
	healthChecker.SetHeartbeatTimeout(5 * time.Second)
	healthChecker.SetMaxErrorCount(10)
	healthChecker.SetRSSIThreshold(-70)
	healthChecker.SetLatencyThreshold(500 * time.Millisecond)

	// 验证配置已设置（通过内部状态检查）
	if healthChecker.heartbeatTimeout != 5*time.Second {
		t.Errorf("期望心跳超时为 5s，实际为 %v", healthChecker.heartbeatTimeout)
	}

	if healthChecker.maxErrorCount != 10 {
		t.Errorf("期望最大错误计数为 10，实际为 %d", healthChecker.maxErrorCount)
	}

	if healthChecker.rssiThreshold != -70 {
		t.Errorf("期望RSSI阈值为 -70，实际为 %d", healthChecker.rssiThreshold)
	}

	if healthChecker.latencyThreshold != 500*time.Millisecond {
		t.Errorf("期望延迟阈值为 500ms，实际为 %v", healthChecker.latencyThreshold)
	}
}

func TestDefaultHealthChecker_ConcurrentAccess(t *testing.T) {
	// 创建连接池和健康检查器
	pool := NewConnectionPool(10)
	defer pool.Close()

	healthChecker := NewDefaultHealthChecker(pool)

	// 添加一些连接
	for i := 0; i < 5; i++ {
		conn := NewMockConnection(fmt.Sprintf("device%d", i))
		pool.AddConnection(conn)
	}

	// 并发测试
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动监控
	healthChecker.StartMonitoring(ctx, 10*time.Millisecond)

	// 并发执行多个操作
	done := make(chan bool, 3)

	// 并发检查连接健康状态
	go func() {
		for i := 0; i < 10; i++ {
			healthChecker.CheckConnection("device0")
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	// 并发获取健康报告
	go func() {
		for i := 0; i < 10; i++ {
			healthChecker.GetHealthReport()
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	// 并发注册回调
	go func() {
		for i := 0; i < 5; i++ {
			healthChecker.RegisterHealthCallback(func(status HealthStatus) {
				// 空回调
			})
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	// 等待所有并发操作完成
	for i := 0; i < 3; i++ {
		select {
		case <-done:
			// 操作完成
		case <-time.After(1 * time.Second):
			t.Fatal("并发测试超时")
		}
	}

	// 停止监控
	healthChecker.StopMonitoring()
}
