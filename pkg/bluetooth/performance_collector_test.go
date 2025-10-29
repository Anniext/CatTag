package bluetooth

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestDefaultPerformanceCollector_StartStopCollection(t *testing.T) {
	// 创建连接池和性能收集器
	pool := NewConnectionPool(5)
	defer pool.Close()

	collector := NewDefaultPerformanceCollector(pool)

	// 测试开始收集
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	collector.StartCollection(ctx, 50*time.Millisecond)

	// 验证收集状态
	if !collector.isCollecting {
		t.Error("期望性能收集器处于收集状态")
	}

	// 测试停止收集
	collector.StopCollection()

	// 等待一小段时间确保停止完成
	time.Sleep(25 * time.Millisecond)

	if collector.isCollecting {
		t.Error("期望性能收集器停止收集")
	}
}

func TestDefaultPerformanceCollector_GetMetrics(t *testing.T) {
	// 创建连接池和性能收集器
	pool := NewConnectionPool(5)
	defer pool.Close()

	collector := NewDefaultPerformanceCollector(pool)

	// 添加一些模拟连接
	conn1 := NewMockConnection("device1")
	conn2 := NewMockConnection("device2")
	pool.AddConnection(conn1)
	pool.AddConnection(conn2)

	// 获取性能指标
	metrics := collector.GetMetrics()

	// 验证指标内容
	if metrics.Timestamp.IsZero() {
		t.Error("期望性能指标有有效的时间戳")
	}

	if len(metrics.ConnectionMetrics) != 2 {
		t.Errorf("期望连接指标数量为 2，实际为 %d", len(metrics.ConnectionMetrics))
	}

	// 验证连接指标包含正确的设备
	if _, exists := metrics.ConnectionMetrics["device1"]; !exists {
		t.Error("期望连接指标包含 device1")
	}

	if _, exists := metrics.ConnectionMetrics["device2"]; !exists {
		t.Error("期望连接指标包含 device2")
	}
}

func TestDefaultPerformanceCollector_HistoricalMetrics(t *testing.T) {
	// 创建连接池和性能收集器
	pool := NewConnectionPool(5)
	defer pool.Close()

	collector := NewDefaultPerformanceCollector(pool)

	// 开始收集
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	collector.StartCollection(ctx, 20*time.Millisecond)

	// 等待收集一些历史数据
	time.Sleep(100 * time.Millisecond)

	// 获取历史指标
	historical := collector.GetHistoricalMetrics(1 * time.Minute)

	if len(historical) == 0 {
		t.Error("期望有历史性能指标数据")
	}

	// 验证历史数据的时间顺序
	for i := 1; i < len(historical); i++ {
		if historical[i].Timestamp.Before(historical[i-1].Timestamp) {
			t.Error("期望历史指标按时间顺序排列")
		}
	}

	collector.StopCollection()
}

func TestDefaultPerformanceCollector_MetricsCallback(t *testing.T) {
	// 创建连接池和性能收集器
	pool := NewConnectionPool(5)
	defer pool.Close()

	collector := NewDefaultPerformanceCollector(pool)

	// 测试回调注册
	callbackCalled := false
	var receivedMetrics PerformanceMetrics

	callback := func(metrics PerformanceMetrics) {
		callbackCalled = true
		receivedMetrics = metrics
	}

	collector.RegisterMetricsCallback(callback)

	// 开始收集以触发回调
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	collector.StartCollection(ctx, 30*time.Millisecond)

	// 等待回调执行
	time.Sleep(50 * time.Millisecond)

	if !callbackCalled {
		t.Error("期望性能指标回调被调用")
	}

	if receivedMetrics.Timestamp.IsZero() {
		t.Error("期望接收到的指标有有效的时间戳")
	}

	collector.StopCollection()
}

func TestDefaultPerformanceCollector_RSSIDataCollection(t *testing.T) {
	// 创建连接池和性能收集器
	pool := NewConnectionPool(5)
	defer pool.Close()

	collector := NewDefaultPerformanceCollector(pool)

	// 添加模拟连接
	conn := NewMockConnection("device1")
	pool.AddConnection(conn)

	// 模拟多次RSSI更新
	connectionMetrics := map[string]ConnectionMetrics{
		"device1": {
			BytesSent:     100,
			BytesReceived: 200,
			LastActivity:  time.Now(),
		},
	}

	// 更新RSSI数据多次
	for i := 0; i < 5; i++ {
		collector.updateRSSIData(connectionMetrics)
		time.Sleep(10 * time.Millisecond)
	}

	// 获取指标并验证RSSI数据
	metrics := collector.GetMetrics()

	if rssiData, exists := metrics.RSSIMetrics["device1"]; exists {
		if len(rssiData.Samples) == 0 {
			t.Error("期望RSSI数据包含样本")
		}

		if rssiData.DeviceID != "device1" {
			t.Errorf("期望RSSI数据设备ID为 device1，实际为 %s", rssiData.DeviceID)
		}
	} else {
		t.Error("期望指标包含 device1 的RSSI数据")
	}
}

func TestDefaultPerformanceCollector_LatencyDataCollection(t *testing.T) {
	// 创建连接池和性能收集器
	pool := NewConnectionPool(5)
	defer pool.Close()

	collector := NewDefaultPerformanceCollector(pool)

	// 添加模拟连接
	conn := NewMockConnection("device1")
	pool.AddConnection(conn)

	// 模拟延迟数据
	connectionMetrics := map[string]ConnectionMetrics{
		"device1": {
			Latency:      100 * time.Millisecond,
			LastActivity: time.Now(),
		},
	}

	// 更新延迟数据
	collector.updateLatencyData(connectionMetrics)

	// 获取指标并验证延迟数据
	metrics := collector.GetMetrics()

	if latencyData, exists := metrics.LatencyMetrics["device1"]; exists {
		if latencyData.Current != 100*time.Millisecond {
			t.Errorf("期望当前延迟为 100ms，实际为 %v", latencyData.Current)
		}

		if latencyData.DeviceID != "device1" {
			t.Errorf("期望延迟数据设备ID为 device1，实际为 %s", latencyData.DeviceID)
		}
	} else {
		t.Error("期望指标包含 device1 的延迟数据")
	}
}

func TestDefaultPerformanceCollector_ThroughputCalculation(t *testing.T) {
	// 创建连接池和性能收集器
	pool := NewConnectionPool(5)
	defer pool.Close()

	collector := NewDefaultPerformanceCollector(pool)

	// 设置初始时间
	collector.lastCollectionTime = time.Now().Add(-1 * time.Second)

	// 模拟连接指标
	connectionMetrics := map[string]ConnectionMetrics{
		"device1": {
			BytesSent:     1000,
			BytesReceived: 2000,
			MessagesSent:  10,
			MessagesRecv:  20,
		},
	}

	// 更新吞吐量数据
	collector.updateThroughputData(connectionMetrics)

	// 验证吞吐量计算
	if collector.throughputData.BytesPerSecond <= 0 {
		t.Error("期望字节吞吐量大于0")
	}

	if collector.throughputData.MessagesPerSecond <= 0 {
		t.Error("期望消息吞吐量大于0")
	}

	if collector.throughputData.TotalBytes != 3000 {
		t.Errorf("期望总字节数为 3000，实际为 %d", collector.throughputData.TotalBytes)
	}

	if collector.throughputData.TotalMessages != 30 {
		t.Errorf("期望总消息数为 30，实际为 %d", collector.throughputData.TotalMessages)
	}
}

func TestDefaultPerformanceCollector_SystemMetrics(t *testing.T) {
	// 创建连接池和性能收集器
	pool := NewConnectionPool(5)
	defer pool.Close()

	collector := NewDefaultPerformanceCollector(pool)

	// 收集系统指标
	systemMetrics := collector.collectSystemMetrics()

	// 验证系统指标
	if systemMetrics.Goroutines <= 0 {
		t.Error("期望Goroutine数量大于0")
	}

	// 内存使用率应该在合理范围内
	if systemMetrics.MemoryUsage < 0 || systemMetrics.MemoryUsage > 100 {
		t.Errorf("期望内存使用率在0-100之间，实际为 %f", systemMetrics.MemoryUsage)
	}
}

func TestDefaultPerformanceCollector_Configuration(t *testing.T) {
	// 创建连接池和性能收集器
	pool := NewConnectionPool(5)
	defer pool.Close()

	collector := NewDefaultPerformanceCollector(pool)

	// 测试配置方法
	collector.SetMaxHistorySize(500)

	if collector.maxHistorySize != 500 {
		t.Errorf("期望最大历史记录大小为 500，实际为 %d", collector.maxHistorySize)
	}

	// 测试清空历史记录
	collector.metricsHistory = append(collector.metricsHistory, PerformanceMetrics{})
	collector.ClearHistory()

	if len(collector.metricsHistory) != 0 {
		t.Error("期望历史记录被清空")
	}
}

func TestDefaultPerformanceCollector_StatisticalCalculations(t *testing.T) {
	// 创建连接池和性能收集器
	pool := NewConnectionPool(5)
	defer pool.Close()

	collector := NewDefaultPerformanceCollector(pool)

	// 测试整数样本统计计算
	samples := []int{-60, -65, -70, -55, -80}

	avg := collector.calculateAverage(samples)
	if avg != -66.0 {
		t.Errorf("期望平均值为 -66.0，实际为 %f", avg)
	}

	min := collector.calculateMin(samples)
	if min != -80 {
		t.Errorf("期望最小值为 -80，实际为 %d", min)
	}

	max := collector.calculateMax(samples)
	if max != -55 {
		t.Errorf("期望最大值为 -55，实际为 %d", max)
	}

	// 测试延迟样本统计计算
	latencySamples := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		150 * time.Millisecond,
		300 * time.Millisecond,
		50 * time.Millisecond,
	}

	avgLatency := collector.calculateLatencyAverage(latencySamples)
	expectedAvg := 160 * time.Millisecond
	if avgLatency != expectedAvg {
		t.Errorf("期望平均延迟为 %v，实际为 %v", expectedAvg, avgLatency)
	}

	minLatency := collector.calculateLatencyMin(latencySamples)
	if minLatency != 50*time.Millisecond {
		t.Errorf("期望最小延迟为 50ms，实际为 %v", minLatency)
	}

	maxLatency := collector.calculateLatencyMax(latencySamples)
	if maxLatency != 300*time.Millisecond {
		t.Errorf("期望最大延迟为 300ms，实际为 %v", maxLatency)
	}
}

func TestDefaultPerformanceCollector_ConcurrentAccess(t *testing.T) {
	// 创建连接池和性能收集器
	pool := NewConnectionPool(10)
	defer pool.Close()

	collector := NewDefaultPerformanceCollector(pool)

	// 添加一些连接
	for i := 0; i < 5; i++ {
		conn := NewMockConnection(fmt.Sprintf("device%d", i))
		pool.AddConnection(conn)
	}

	// 并发测试
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动收集
	collector.StartCollection(ctx, 10*time.Millisecond)

	// 并发执行多个操作
	done := make(chan bool, 3)

	// 并发获取指标
	go func() {
		for i := 0; i < 10; i++ {
			collector.GetMetrics()
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	// 并发获取历史指标
	go func() {
		for i := 0; i < 10; i++ {
			collector.GetHistoricalMetrics(1 * time.Minute)
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	// 并发注册回调
	go func() {
		for i := 0; i < 5; i++ {
			collector.RegisterMetricsCallback(func(metrics PerformanceMetrics) {
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

	// 停止收集
	collector.StopCollection()
}
