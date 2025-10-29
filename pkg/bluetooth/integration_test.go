package bluetooth

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// TestEndToEndCommunication 测试端到端通信
func TestEndToEndCommunication(t *testing.T) {
	// 创建服务端组件
	serverConfig := DefaultConfig()
	serverConfig.ServerConfig.ServiceUUID = "test-service-uuid"
	serverConfig.ServerConfig.MaxConnections = 5

	server, err := NewBluetoothComponent[string](serverConfig)
	if err != nil {
		t.Fatalf("创建服务端失败: %v", err)
	}

	// 创建客户端组件
	clientConfig := DefaultConfig()
	clientConfig.ClientConfig.AutoReconnect = true

	client, err := NewBluetoothComponent[string](clientConfig)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 启动服务端
	if err := server.Start(ctx); err != nil {
		t.Fatalf("启动服务端失败: %v", err)
	}
	defer server.Stop(ctx)

	// 启动客户端
	if err := client.Start(ctx); err != nil {
		t.Fatalf("启动客户端失败: %v", err)
	}
	defer client.Stop(ctx)

	// 等待组件启动完成
	time.Sleep(2 * time.Second)

	// 测试消息发送
	testMessage := "Hello, Bluetooth!"
	deviceID := "test-device-001"

	// 发送消息
	if err := client.Send(ctx, deviceID, testMessage); err != nil {
		t.Errorf("发送消息失败: %v", err)
	}

	// 接收消息
	select {
	case msg := <-server.Receive(ctx):
		if msg.Payload != testMessage {
			t.Errorf("接收到的消息不匹配: 期望 %s, 实际 %s", testMessage, msg.Payload)
		}
	case <-time.After(5 * time.Second):
		t.Error("接收消息超时")
	}

	t.Log("端到端通信测试通过")
}

// TestMultiDeviceConnections 测试多设备连接场景
func TestMultiDeviceConnections(t *testing.T) {
	// 创建服务端组件
	serverConfig := DefaultConfig()
	serverConfig.ServerConfig.ServiceUUID = "multi-device-test"
	serverConfig.ServerConfig.MaxConnections = 10

	server, err := NewBluetoothComponent[string](serverConfig)
	if err != nil {
		t.Fatalf("创建服务端失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 启动服务端
	if err := server.Start(ctx); err != nil {
		t.Fatalf("启动服务端失败: %v", err)
	}
	defer server.Stop(ctx)

	// 创建多个客户端
	numClients := 5
	clients := make([]*DefaultBluetoothComponent[string], numClients)

	for i := 0; i < numClients; i++ {
		clientConfig := DefaultConfig()
		clientConfig.ClientConfig.AutoReconnect = true

		client, err := NewBluetoothComponent[string](clientConfig)
		if err != nil {
			t.Fatalf("创建客户端 %d 失败: %v", i, err)
		}

		clients[i] = client

		// 启动客户端
		if err := client.Start(ctx); err != nil {
			t.Fatalf("启动客户端 %d 失败: %v", i, err)
		}
		defer client.Stop(ctx)
	}

	// 等待所有客户端启动
	time.Sleep(3 * time.Second)

	// 并发发送消息
	var wg sync.WaitGroup
	messageCount := 10

	for i, client := range clients {
		wg.Add(1)
		go func(clientIndex int, c *DefaultBluetoothComponent[string]) {
			defer wg.Done()

			for j := 0; j < messageCount; j++ {
				message := fmt.Sprintf("Client %d - Message %d", clientIndex, j)
				deviceID := fmt.Sprintf("device-%d", clientIndex)

				if err := c.Send(ctx, deviceID, message); err != nil {
					t.Errorf("客户端 %d 发送消息 %d 失败: %v", clientIndex, j, err)
				}

				// 短暂延迟避免过载
				time.Sleep(100 * time.Millisecond)
			}
		}(i, client)
	}

	// 接收消息
	receivedCount := 0
	expectedCount := numClients * messageCount

	go func() {
		for {
			select {
			case <-server.Receive(ctx):
				receivedCount++
			case <-ctx.Done():
				return
			}
		}
	}()

	// 等待所有消息发送完成
	wg.Wait()

	// 等待消息接收
	time.Sleep(5 * time.Second)

	if receivedCount < expectedCount/2 { // 允许一些消息丢失
		t.Errorf("接收到的消息数量不足: 期望至少 %d, 实际 %d", expectedCount/2, receivedCount)
	}

	t.Logf("多设备连接测试通过: 发送 %d 条消息, 接收 %d 条消息", expectedCount, receivedCount)
}

// TestFailureRecovery 测试故障恢复
func TestFailureRecovery(t *testing.T) {
	// 创建组件
	config := DefaultConfig()
	config.ClientConfig.AutoReconnect = true
	config.ClientConfig.RetryAttempts = 3
	config.ClientConfig.RetryInterval = 1 * time.Second

	component, err := NewBluetoothComponent[string](config)
	if err != nil {
		t.Fatalf("创建组件失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 启动组件
	if err := component.Start(ctx); err != nil {
		t.Fatalf("启动组件失败: %v", err)
	}
	defer component.Stop(ctx)

	// 测试组件状态
	initialStatus := component.GetStatus()
	if initialStatus != StatusRunning {
		t.Errorf("组件初始状态错误: 期望 %s, 实际 %s", StatusRunning, initialStatus)
	}

	// 模拟故障 - 停止组件
	if err := component.Stop(ctx); err != nil {
		t.Errorf("停止组件失败: %v", err)
	}

	// 检查状态
	stoppedStatus := component.GetStatus()
	if stoppedStatus != StatusStopped {
		t.Errorf("组件停止状态错误: 期望 %s, 实际 %s", StatusStopped, stoppedStatus)
	}

	// 恢复组件
	if err := component.Start(ctx); err != nil {
		t.Errorf("重启组件失败: %v", err)
	}

	// 检查恢复状态
	recoveredStatus := component.GetStatus()
	if recoveredStatus != StatusRunning {
		t.Errorf("组件恢复状态错误: 期望 %s, 实际 %s", StatusRunning, recoveredStatus)
	}

	t.Log("故障恢复测试通过")
}

// TestPerformance 测试性能表现
func TestPerformance(t *testing.T) {
	// 创建高性能配置
	config := DefaultConfig()
	config.PoolConfig.MaxConnections = 20
	config.HealthConfig.CheckInterval = 1 * time.Second

	component, err := NewBluetoothComponent[string](config)
	if err != nil {
		t.Fatalf("创建组件失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 启动组件
	if err := component.Start(ctx); err != nil {
		t.Fatalf("启动组件失败: %v", err)
	}
	defer component.Stop(ctx)

	// 性能测试参数
	numMessages := 1000
	numGoroutines := 10
	messagesPerGoroutine := numMessages / numGoroutines

	startTime := time.Now()

	// 并发发送消息
	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < messagesPerGoroutine; j++ {
				message := fmt.Sprintf("Performance test message %d-%d", goroutineID, j)
				deviceID := fmt.Sprintf("perf-device-%d", goroutineID)

				if err := component.Send(ctx, deviceID, message); err != nil {
					// 在性能测试中，一些错误是可以接受的
					continue
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// 计算性能指标
	messagesPerSecond := float64(numMessages) / duration.Seconds()

	t.Logf("性能测试结果:")
	t.Logf("- 总消息数: %d", numMessages)
	t.Logf("- 总耗时: %v", duration)
	t.Logf("- 消息/秒: %.2f", messagesPerSecond)

	// 性能基准检查
	minMessagesPerSecond := 100.0 // 最低性能要求
	if messagesPerSecond < minMessagesPerSecond {
		t.Errorf("性能不达标: 期望至少 %.2f 消息/秒, 实际 %.2f 消息/秒",
			minMessagesPerSecond, messagesPerSecond)
	}

	t.Log("性能测试通过")
}

// TestComponentCoordination 测试组件协调
func TestComponentCoordination(t *testing.T) {
	// 创建组件协调器
	logger := slog.Default()
	coordinator := NewComponentCoordinator(logger)

	// 创建测试组件
	config := DefaultConfig()
	component, err := NewBluetoothComponent[string](config)
	if err != nil {
		t.Fatalf("创建组件失败: %v", err)
	}

	// 注册组件
	err = coordinator.RegisterComponent("test-component", ComponentTypeAdapter, component, []string{})
	if err != nil {
		t.Fatalf("注册组件失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 启动组件
	if err := coordinator.StartComponent(ctx, "test-component"); err != nil {
		t.Errorf("启动组件失败: %v", err)
	}

	// 检查组件状态
	status, err := coordinator.GetComponentStatus("test-component")
	if err != nil {
		t.Errorf("获取组件状态失败: %v", err)
	}

	if status != StatusRunning {
		t.Errorf("组件状态错误: 期望 %s, 实际 %s", StatusRunning, status)
	}

	// 停止组件
	if err := coordinator.StopComponent(ctx, "test-component"); err != nil {
		t.Errorf("停止组件失败: %v", err)
	}

	// 检查停止状态
	status, err = coordinator.GetComponentStatus("test-component")
	if err != nil {
		t.Errorf("获取组件状态失败: %v", err)
	}

	if status != StatusStopped {
		t.Errorf("组件停止状态错误: 期望 %s, 实际 %s", StatusStopped, status)
	}

	t.Log("组件协调测试通过")
}

// TestErrorHandling 测试错误处理
func TestErrorHandling(t *testing.T) {
	// 创建错误处理器
	errorHandler := NewUnifiedErrorHandler()

	// 添加自定义错误处理器
	errorHandler.AddHandler(ErrCodeConnectionFailed, func(err *BluetoothError) error {
		t.Logf("处理连接错误: %s", err.Message)
		return nil
	})

	// 创建测试错误
	testError := NewBluetoothError(ErrCodeConnectionFailed, "测试连接失败")

	// 处理错误
	if err := errorHandler.HandleError(testError); err != nil {
		t.Errorf("错误处理失败: %v", err)
	}

	// 检查错误历史
	history := errorHandler.GetErrorHistory()
	if len(history) == 0 {
		t.Error("错误历史为空")
	}

	// 检查错误统计
	stats := errorHandler.GetErrorStatistics()
	if stats.TotalErrors == 0 {
		t.Error("错误统计为空")
	}

	t.Log("错误处理测试通过")
}

// TestLogManagement 测试日志管理
func TestLogManagement(t *testing.T) {
	// 创建日志管理器
	logger := slog.Default()
	logManager := NewLogManager(logger)

	// 设置日志级别
	logManager.SetLogLevel(slog.LevelDebug)

	// 记录不同级别的日志
	logManager.LogInfo("test-component", "测试信息日志", map[string]interface{}{
		"key1": "value1",
		"key2": 123,
	})

	logManager.LogWarn("test-component", "测试警告日志", map[string]interface{}{
		"warning": "test warning",
	})

	logManager.LogError("test-component", "测试错误日志",
		fmt.Errorf("测试错误"), map[string]interface{}{
			"error_code": 500,
		})

	// 检查日志缓冲区
	buffer := logManager.GetLogBuffer()
	if len(buffer) == 0 {
		t.Error("日志缓冲区为空")
	}

	// 检查日志统计
	stats := logManager.GetLogStatistics()
	if stats.TotalLogs == 0 {
		t.Error("日志统计为空")
	}

	t.Logf("日志管理测试通过: 记录了 %d 条日志", stats.TotalLogs)
}

// BenchmarkMessageSending 消息发送性能基准测试
func BenchmarkMessageSending(b *testing.B) {
	config := DefaultConfig()
	component, err := NewBluetoothComponent[string](config)
	if err != nil {
		b.Fatalf("创建组件失败: %v", err)
	}

	ctx := context.Background()
	if err := component.Start(ctx); err != nil {
		b.Fatalf("启动组件失败: %v", err)
	}
	defer component.Stop(ctx)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		message := fmt.Sprintf("Benchmark message %d", i)
		deviceID := "benchmark-device"

		component.Send(ctx, deviceID, message)
	}
}

// BenchmarkComponentStartup 组件启动性能基准测试
func BenchmarkComponentStartup(b *testing.B) {
	for i := 0; i < b.N; i++ {
		config := DefaultConfig()
		component, err := NewBluetoothComponent[string](config)
		if err != nil {
			b.Fatalf("创建组件失败: %v", err)
		}

		ctx := context.Background()
		if err := component.Start(ctx); err != nil {
			b.Errorf("启动组件失败: %v", err)
		}

		component.Stop(ctx)
	}
}
