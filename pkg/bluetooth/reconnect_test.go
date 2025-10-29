package bluetooth

import (
	"context"
	"testing"
	"time"
)

// TestReconnectManager_ExponentialBackoff 测试指数退避重连策略
func TestReconnectManager_ExponentialBackoff(t *testing.T) {
	config := ClientConfig{
		RetryAttempts: 5,
		RetryInterval: 100 * time.Millisecond,
		AutoReconnect: true,
	}

	manager := NewReconnectManager(config)
	deviceID := "test_device_backoff"

	// 第一次安排重连
	manager.ScheduleReconnect(deviceID)
	status1 := manager.GetReconnectStatus(deviceID)
	if status1 == nil {
		t.Fatal("重连任务未创建")
	}

	if status1.Attempts != 1 {
		t.Errorf("期望尝试次数为 1，实际为 %d", status1.Attempts)
	}

	if status1.BackoffDelay != config.RetryInterval {
		t.Errorf("期望第一次退避延迟为 %v，实际为 %v", config.RetryInterval, status1.BackoffDelay)
	}

	// 第二次安排重连（模拟失败）
	manager.ScheduleReconnect(deviceID)
	status2 := manager.GetReconnectStatus(deviceID)

	if status2.Attempts != 2 {
		t.Errorf("期望尝试次数为 2，实际为 %d", status2.Attempts)
	}

	expectedDelay2 := 2 * config.RetryInterval
	if status2.BackoffDelay != expectedDelay2 {
		t.Errorf("期望第二次退避延迟为 %v，实际为 %v", expectedDelay2, status2.BackoffDelay)
	}

	// 第三次安排重连
	manager.ScheduleReconnect(deviceID)
	status3 := manager.GetReconnectStatus(deviceID)

	expectedDelay3 := 4 * config.RetryInterval
	if status3.BackoffDelay != expectedDelay3 {
		t.Errorf("期望第三次退避延迟为 %v，实际为 %v", expectedDelay3, status3.BackoffDelay)
	}

	// 验证统计信息
	stats := manager.GetStatistics()
	if stats.TotalTasks != 1 {
		t.Errorf("期望总任务数为 1，实际为 %d", stats.TotalTasks)
	}

	if stats.ActiveTasks != 1 {
		t.Errorf("期望活跃任务数为 1，实际为 %d", stats.ActiveTasks)
	}

	if stats.TotalAttempts != 3 {
		t.Errorf("期望总尝试次数为 3，实际为 %d", stats.TotalAttempts)
	}
}

// TestReconnectManager_MaxRetries 测试最大重试次数限制
func TestReconnectManager_MaxRetries(t *testing.T) {
	config := ClientConfig{
		RetryAttempts: 3,
		RetryInterval: 50 * time.Millisecond,
		AutoReconnect: true,
	}

	manager := NewReconnectManager(config)
	deviceID := "test_device_max_retries"

	// 执行重连直到达到最大重试次数
	for i := 0; i < config.RetryAttempts+1; i++ {
		manager.ScheduleReconnect(deviceID)
		status := manager.GetReconnectStatus(deviceID)
		if status != nil && !status.Enabled {
			// 任务已被禁用，停止重试
			break
		}
	}

	status := manager.GetReconnectStatus(deviceID)
	if status == nil {
		t.Fatal("重连任务未创建")
	}

	// 验证任务被禁用
	if status.Enabled {
		t.Error("期望重连任务被禁用，因为超过了最大重试次数")
	}

	if status.Attempts <= config.RetryAttempts {
		t.Errorf("期望尝试次数超过最大重试次数 %d，实际为 %d", config.RetryAttempts, status.Attempts)
	}
}

// TestReconnectManager_StopReconnect 测试停止重连功能
func TestReconnectManager_StopReconnect(t *testing.T) {
	config := ClientConfig{
		RetryAttempts: 5,
		RetryInterval: 100 * time.Millisecond,
		AutoReconnect: true,
	}

	manager := NewReconnectManager(config)
	deviceID := "test_device_stop"

	// 安排重连
	manager.ScheduleReconnect(deviceID)
	status1 := manager.GetReconnectStatus(deviceID)
	if status1 == nil || !status1.Enabled {
		t.Fatal("重连任务未正确创建或启用")
	}

	// 停止重连
	manager.StopReconnect(deviceID)
	status2 := manager.GetReconnectStatus(deviceID)
	if status2 == nil {
		t.Fatal("重连任务被意外删除")
	}

	if status2.Enabled {
		t.Error("期望重连任务被禁用")
	}
}

// TestReconnectManager_ResetTask 测试重置重连任务
func TestReconnectManager_ResetTask(t *testing.T) {
	config := ClientConfig{
		RetryAttempts: 5,
		RetryInterval: 100 * time.Millisecond,
		AutoReconnect: true,
	}

	manager := NewReconnectManager(config)
	deviceID := "test_device_reset"

	// 执行几次重连
	for i := 0; i < 3; i++ {
		manager.ScheduleReconnect(deviceID)
	}

	status1 := manager.GetReconnectStatus(deviceID)
	if status1.Attempts != 3 {
		t.Errorf("期望尝试次数为 3，实际为 %d", status1.Attempts)
	}

	// 重置任务
	manager.ResetReconnectTask(deviceID)
	status2 := manager.GetReconnectStatus(deviceID)

	if status2.Attempts != 0 {
		t.Errorf("期望重置后尝试次数为 0，实际为 %d", status2.Attempts)
	}

	if status2.BackoffDelay != config.RetryInterval {
		t.Errorf("期望重置后退避延迟为 %v，实际为 %v", config.RetryInterval, status2.BackoffDelay)
	}

	if !status2.Enabled {
		t.Error("期望重置后任务为启用状态")
	}

	if status2.LastError != nil {
		t.Error("期望重置后错误为空")
	}
}

// TestReconnectManager_ClearTask 测试清除重连任务
func TestReconnectManager_ClearTask(t *testing.T) {
	config := ClientConfig{
		RetryAttempts: 5,
		RetryInterval: 100 * time.Millisecond,
		AutoReconnect: true,
	}

	manager := NewReconnectManager(config)
	deviceID := "test_device_clear"

	// 安排重连
	manager.ScheduleReconnect(deviceID)
	status1 := manager.GetReconnectStatus(deviceID)
	if status1 == nil {
		t.Fatal("重连任务未创建")
	}

	// 清除任务
	manager.ClearReconnectTask(deviceID)
	status2 := manager.GetReconnectStatus(deviceID)
	if status2 != nil {
		t.Error("期望任务被清除，但仍然存在")
	}
}

// TestReconnectManager_MultipleDevices 测试多设备重连管理
func TestReconnectManager_MultipleDevices(t *testing.T) {
	config := ClientConfig{
		RetryAttempts: 3,
		RetryInterval: 50 * time.Millisecond,
		AutoReconnect: true,
	}

	manager := NewReconnectManager(config)
	devices := []string{"device_1", "device_2", "device_3"}

	// 为多个设备安排重连
	for _, deviceID := range devices {
		manager.ScheduleReconnect(deviceID)
	}

	// 验证所有任务都被创建
	allTasks := manager.GetAllReconnectTasks()
	if len(allTasks) != len(devices) {
		t.Errorf("期望任务数为 %d，实际为 %d", len(devices), len(allTasks))
	}

	for _, deviceID := range devices {
		if task, exists := allTasks[deviceID]; !exists {
			t.Errorf("设备 %s 的重连任务未找到", deviceID)
		} else if !task.Enabled {
			t.Errorf("设备 %s 的重连任务未启用", deviceID)
		}
	}

	// 验证统计信息
	stats := manager.GetStatistics()
	if stats.TotalTasks != len(devices) {
		t.Errorf("期望总任务数为 %d，实际为 %d", len(devices), stats.TotalTasks)
	}

	if stats.ActiveTasks != len(devices) {
		t.Errorf("期望活跃任务数为 %d，实际为 %d", len(devices), stats.ActiveTasks)
	}
}

// TestClient_AutoReconnectIntegration 测试客户端自动重连集成
func TestClient_AutoReconnectIntegration(t *testing.T) {
	config := ClientConfig{
		ScanTimeout:    1 * time.Second,
		ConnectTimeout: 2 * time.Second,
		RetryAttempts:  2,
		RetryInterval:  200 * time.Millisecond,
		AutoReconnect:  true,
		KeepAlive:      false,
	}

	adapter := NewMockAdapter()
	client := NewBluetoothClient[string](config, adapter)

	ctx := context.Background()
	err := client.Start(ctx)
	if err != nil {
		t.Fatalf("启动客户端失败: %v", err)
	}
	defer client.Stop(ctx)

	deviceID := "test_device_integration"

	// 验证重连管理器已启动
	if client.reconnectManager == nil {
		t.Fatal("重连管理器未初始化")
	}

	// 直接测试重连管理器功能
	client.reconnectManager.ScheduleReconnect(deviceID)

	// 验证重连任务被创建
	status := client.reconnectManager.GetReconnectStatus(deviceID)
	if status == nil {
		t.Fatal("期望创建重连任务")
	}

	if !status.Enabled {
		t.Error("期望重连任务为启用状态")
	}

	if status.Attempts != 1 {
		t.Errorf("期望尝试次数为 1，实际为 %d", status.Attempts)
	}

	// 手动停止重连
	client.reconnectManager.StopReconnect(deviceID)

	// 验证重连被停止
	status = client.reconnectManager.GetReconnectStatus(deviceID)
	if status != nil && status.Enabled {
		t.Error("期望重连任务被禁用")
	}
}

// TestClient_ConnectionQualityAssessment 测试连接质量评估
func TestClient_ConnectionQualityAssessment(t *testing.T) {
	config := ClientConfig{
		ScanTimeout:    1 * time.Second,
		ConnectTimeout: 2 * time.Second,
		RetryAttempts:  1,
		RetryInterval:  100 * time.Millisecond,
		AutoReconnect:  false,
		KeepAlive:      true,
	}

	adapter := NewMockAdapter()
	client := NewBluetoothClient[string](config, adapter)

	ctx := context.Background()
	err := client.Start(ctx)
	if err != nil {
		t.Fatalf("启动客户端失败: %v", err)
	}
	defer client.Stop(ctx)

	deviceID := "test_device_quality"

	// 连接到设备
	conn, err := client.Connect(ctx, deviceID)
	if err != nil {
		t.Fatalf("连接设备失败: %v", err)
	}

	// 获取初始指标
	initialMetrics := conn.GetMetrics()
	if initialMetrics.ConnectedAt.IsZero() {
		t.Error("期望连接时间不为零")
	}

	// 模拟一些网络活动
	if mockConn, ok := conn.(*MockConnection); ok {
		// 模拟延迟
		mockConn.SimulateLatency(50 * time.Millisecond)

		// 模拟一些错误
		mockConn.IncrementErrorCount()
		mockConn.IncrementErrorCount()

		// 模拟发送数据
		testData := []byte("test message")
		err = mockConn.Send(testData)
		if err != nil {
			t.Errorf("发送测试数据失败: %v", err)
		}
	}

	// 获取更新后的指标
	updatedMetrics := conn.GetMetrics()

	if updatedMetrics.Latency != 50*time.Millisecond {
		t.Errorf("期望延迟为 50ms，实际为 %v", updatedMetrics.Latency)
	}

	if updatedMetrics.ErrorCount != 2 {
		t.Errorf("期望错误计数为 2，实际为 %d", updatedMetrics.ErrorCount)
	}

	if updatedMetrics.MessagesSent == 0 {
		t.Error("期望发送消息数大于 0")
	}

	t.Logf("连接指标: 延迟=%v, 错误数=%d, 发送消息=%d, 发送字节=%d",
		updatedMetrics.Latency, updatedMetrics.ErrorCount,
		updatedMetrics.MessagesSent, updatedMetrics.BytesSent)
}
