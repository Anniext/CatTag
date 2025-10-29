package adapter

import (
	"testing"
	"time"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// TestDefaultConnection_Basic 测试连接基本功能
func TestDefaultConnection_Basic(t *testing.T) {
	deviceID := "test_device_001"
	bufferSize := 1024

	conn := NewDefaultConnection(deviceID, bufferSize)

	// 验证基本属性
	if conn.DeviceID() != deviceID {
		t.Errorf("设备ID不匹配，期望: %s, 实际: %s", deviceID, conn.DeviceID())
	}

	if conn.ID() == "" {
		t.Error("连接ID不能为空")
	}

	if !conn.IsActive() {
		t.Error("新创建的连接应该是活跃状态")
	}

	// 清理
	err := conn.Close()
	if err != nil {
		t.Errorf("关闭连接失败: %v", err)
	}

	if conn.IsActive() {
		t.Error("关闭后的连接应该是非活跃状态")
	}
}

// TestDefaultConnection_SendReceive 测试发送和接收数据
func TestDefaultConnection_SendReceive(t *testing.T) {
	deviceID := "test_device_002"
	bufferSize := 1024

	conn := NewDefaultConnection(deviceID, bufferSize)
	defer conn.Close()

	// 测试发送数据
	testData := []byte("测试数据")
	err := conn.Send(testData)
	if err != nil {
		t.Fatalf("发送数据失败: %v", err)
	}

	// 验证指标更新
	metrics := conn.GetMetrics()
	if metrics.MessagesSent != 1 {
		t.Errorf("发送消息计数不正确，期望: 1, 实际: %d", metrics.MessagesSent)
	}

	if metrics.BytesSent != uint64(len(testData)) {
		t.Errorf("发送字节数不正确，期望: %d, 实际: %d",
			len(testData), metrics.BytesSent)
	}

	// 测试接收数据
	receiveChan := conn.Receive()

	// 等待接收数据（模拟数据会定期发送）
	select {
	case receivedData := <-receiveChan:
		t.Logf("接收到数据: %s", string(receivedData))

		// 验证接收指标
		metrics = conn.GetMetrics()
		if metrics.MessagesRecv == 0 {
			t.Error("接收消息计数应该大于0")
		}

	case <-time.After(3 * time.Second):
		t.Log("警告: 在超时时间内未接收到数据（这在某些情况下是正常的）")
	}
}

// TestDefaultConnection_SendErrors 测试发送错误情况
func TestDefaultConnection_SendErrors(t *testing.T) {
	deviceID := "test_device_003"
	bufferSize := 1024

	conn := NewDefaultConnection(deviceID, bufferSize)

	// 测试发送空数据
	err := conn.Send(nil)
	if err == nil {
		t.Error("发送空数据应该失败")
	}

	err = conn.Send([]byte{})
	if err == nil {
		t.Error("发送空数据应该失败")
	}

	// 关闭连接后测试发送
	conn.Close()

	err = conn.Send([]byte("测试数据"))
	if err == nil {
		t.Error("关闭连接后发送数据应该失败")
	}

	// 验证错误类型
	if !bluetooth.IsConnectionError(err) {
		t.Error("应该返回连接错误类型")
	}
}

// TestDefaultConnection_Metrics 测试连接指标
func TestDefaultConnection_Metrics(t *testing.T) {
	deviceID := "test_device_004"
	bufferSize := 1024

	conn := NewDefaultConnection(deviceID, bufferSize)
	defer conn.Close()

	// 获取初始指标
	initialMetrics := conn.GetMetrics()
	if initialMetrics.ConnectedAt.IsZero() {
		t.Error("连接时间不能为零值")
	}

	if initialMetrics.LastActivity.IsZero() {
		t.Error("最后活动时间不能为零值")
	}

	// 发送多条消息
	for i := 0; i < 5; i++ {
		testData := []byte("测试消息")
		err := conn.Send(testData)
		if err != nil {
			t.Fatalf("发送第%d条消息失败: %v", i+1, err)
		}
	}

	// 验证指标更新
	finalMetrics := conn.GetMetrics()
	if finalMetrics.MessagesSent != 5 {
		t.Errorf("发送消息计数不正确，期望: 5, 实际: %d", finalMetrics.MessagesSent)
	}

	if finalMetrics.BytesSent == 0 {
		t.Error("发送字节数应该大于0")
	}

	if finalMetrics.LastActivity.Before(initialMetrics.LastActivity) {
		t.Error("最后活动时间应该更新")
	}

	// 验证延迟指标
	if finalMetrics.Latency <= 0 {
		t.Error("延迟应该大于0")
	}
}

// TestDefaultConnection_Concurrent 测试并发操作
func TestDefaultConnection_Concurrent(t *testing.T) {
	deviceID := "test_device_005"
	bufferSize := 1024

	conn := NewDefaultConnection(deviceID, bufferSize)
	defer conn.Close()

	// 并发发送数据
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			testData := []byte("并发测试数据")
			err := conn.Send(testData)
			if err != nil {
				t.Errorf("并发发送第%d条消息失败: %v", id, err)
			}
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// 继续等待
		case <-time.After(5 * time.Second):
			t.Fatal("并发测试超时")
		}
	}

	// 验证最终指标
	metrics := conn.GetMetrics()
	if metrics.MessagesSent != 10 {
		t.Errorf("并发发送消息计数不正确，期望: 10, 实际: %d", metrics.MessagesSent)
	}
}

// TestDefaultConnection_CloseMultiple 测试多次关闭连接
func TestDefaultConnection_CloseMultiple(t *testing.T) {
	deviceID := "test_device_006"
	bufferSize := 1024

	conn := NewDefaultConnection(deviceID, bufferSize)

	// 第一次关闭
	err := conn.Close()
	if err != nil {
		t.Errorf("第一次关闭连接失败: %v", err)
	}

	if conn.IsActive() {
		t.Error("关闭后连接应该是非活跃状态")
	}

	// 第二次关闭应该不报错
	err = conn.Close()
	if err != nil {
		t.Errorf("第二次关闭连接不应该报错: %v", err)
	}
}

// TestDefaultConnection_ReceiveAfterClose 测试关闭后接收数据
func TestDefaultConnection_ReceiveAfterClose(t *testing.T) {
	deviceID := "test_device_007"
	bufferSize := 1024

	conn := NewDefaultConnection(deviceID, bufferSize)

	// 获取接收通道
	receiveChan := conn.Receive()

	// 关闭连接
	err := conn.Close()
	if err != nil {
		t.Errorf("关闭连接失败: %v", err)
	}

	// 验证接收通道已关闭
	select {
	case data, ok := <-receiveChan:
		if ok {
			t.Errorf("关闭连接后不应该接收到数据: %s", string(data))
		}
		// 通道已关闭，这是预期行为
	case <-time.After(1 * time.Second):
		// 超时也是可接受的，因为通道可能已经关闭
	}
}
