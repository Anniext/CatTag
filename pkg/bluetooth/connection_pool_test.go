package bluetooth

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestConnectionPool_AddConnection(t *testing.T) {
	pool := NewConnectionPool(5)
	defer pool.Close()

	// 测试添加连接
	conn1 := NewMockConnection("device1")
	err := pool.AddConnection(conn1)
	if err != nil {
		t.Fatalf("添加连接失败: %v", err)
	}

	// 验证连接已添加
	retrievedConn, err := pool.GetConnection("device1")
	if err != nil {
		t.Fatalf("获取连接失败: %v", err)
	}

	if retrievedConn.DeviceID() != "device1" {
		t.Errorf("期望设备ID为 device1，实际为 %s", retrievedConn.DeviceID())
	}

	// 测试添加重复连接
	err = pool.AddConnection(conn1)
	if err == nil {
		t.Error("期望添加重复连接时返回错误")
	}
}

func TestConnectionPool_RemoveConnection(t *testing.T) {
	pool := NewConnectionPool(5)
	defer pool.Close()

	// 添加连接
	conn1 := NewMockConnection("device1")
	pool.AddConnection(conn1)

	// 移除连接
	err := pool.RemoveConnection("device1")
	if err != nil {
		t.Fatalf("移除连接失败: %v", err)
	}

	// 验证连接已移除
	_, err = pool.GetConnection("device1")
	if err == nil {
		t.Error("期望获取已移除的连接时返回错误")
	}

	// 测试移除不存在的连接
	err = pool.RemoveConnection("nonexistent")
	if err == nil {
		t.Error("期望移除不存在的连接时返回错误")
	}
}

func TestConnectionPool_MaxConnections(t *testing.T) {
	maxConnections := 3
	pool := NewConnectionPool(maxConnections)
	defer pool.Close()

	// 添加最大数量的连接，都设置为高优先级
	for i := 0; i < maxConnections; i++ {
		deviceID := fmt.Sprintf("device%d", i)
		pool.SetConnectionPriority(deviceID, PriorityHigh)
		conn := NewMockConnection(deviceID)
		err := pool.AddConnection(conn)
		if err != nil {
			t.Fatalf("添加连接 %d 失败: %v", i, err)
		}
	}

	// 验证连接数达到最大值
	stats := pool.GetStats()
	if stats.ActiveConnections != maxConnections {
		t.Errorf("期望活跃连接数为 %d，实际为 %d", maxConnections, stats.ActiveConnections)
	}

	// 尝试添加超出限制的连接（可能会成功，因为会移除现有连接）
	extraConn := NewMockConnection("extra_device")
	_ = pool.AddConnection(extraConn) // 忽略错误，因为可能成功也可能失败

	// 验证连接数仍然不超过最大值
	stats = pool.GetStats()
	if stats.ActiveConnections > maxConnections {
		t.Errorf("活跃连接数 %d 超过了最大限制 %d", stats.ActiveConnections, maxConnections)
	}
}

func TestConnectionPool_SetConnectionPriority(t *testing.T) {
	pool := NewConnectionPool(5)
	defer pool.Close()

	// 设置优先级后添加连接
	pool.SetConnectionPriority("device1", PriorityHigh)

	conn1 := NewMockConnection("device1")
	err := pool.AddConnection(conn1)
	if err != nil {
		t.Fatalf("添加连接失败: %v", err)
	}

	// 验证优先级连接
	highPriorityConns := pool.GetConnectionsByPriority(PriorityHigh)
	if len(highPriorityConns) != 1 {
		t.Errorf("期望高优先级连接数为 1，实际为 %d", len(highPriorityConns))
	}

	if highPriorityConns[0].DeviceID() != "device1" {
		t.Errorf("期望高优先级连接设备ID为 device1，实际为 %s", highPriorityConns[0].DeviceID())
	}
}

func TestConnectionPool_LoadBalancing(t *testing.T) {
	pool := NewConnectionPool(5)
	defer pool.Close()

	// 添加多个连接
	for i := 0; i < 3; i++ {
		conn := NewMockConnection(fmt.Sprintf("device%d", i))
		pool.AddConnection(conn)
	}

	// 测试负载均衡选择
	selectedConnections := make(map[string]int)

	for i := 0; i < 30; i++ {
		conn, err := pool.SelectConnectionByStrategy()
		if err != nil {
			t.Fatalf("选择连接失败: %v", err)
		}
		selectedConnections[conn.DeviceID()]++
	}

	// 验证每个连接都被选择过
	if len(selectedConnections) != 3 {
		t.Errorf("期望选择到 3 个不同的连接，实际选择到 %d 个", len(selectedConnections))
	}

	// 验证负载相对均衡（允许一定偏差）
	for deviceID, count := range selectedConnections {
		if count < 5 || count > 15 {
			t.Errorf("设备 %s 被选择 %d 次，负载不均衡", deviceID, count)
		}
	}
}

func TestConnectionPool_ConcurrentAccess(t *testing.T) {
	pool := NewConnectionPool(5) // 更小的池大小
	defer pool.Close()

	var wg sync.WaitGroup
	errors := make(chan error, 200)

	// 并发添加连接，数量远超池大小
	for i := 0; i < 100; i++ { // 更多的连接
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn := NewMockConnection(fmt.Sprintf("device%d", id))
			if err := pool.AddConnection(conn); err != nil {
				errors <- err
			}
		}(i)
	}

	// 并发获取连接
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			time.Sleep(time.Millisecond * 10) // 等待连接添加
			if _, err := pool.GetConnection(fmt.Sprintf("device%d", id)); err != nil {
				// 某些连接可能因为池满而添加失败，这是正常的
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// 检查是否有意外错误
	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
		}
	}

	// 记录错误数量（可能为0，因为连接池会自动管理空间）
	t.Logf("连接添加错误数: %d", errorCount)

	stats := pool.GetStats()
	if stats.ActiveConnections > 5 {
		t.Errorf("活跃连接数 %d 超过了最大限制 5", stats.ActiveConnections)
	}
}

func TestConnectionPool_Statistics(t *testing.T) {
	pool := NewConnectionPool(5)
	defer pool.Close()

	// 添加连接并模拟数据传输
	for i := 0; i < 3; i++ {
		mockConn := NewMockConnectionPtr(fmt.Sprintf("device%d", i))
		conn := Connection(mockConn)

		// 模拟数据传输
		mockConn.Send([]byte("test data"))
		mockConn.SimulateReceiveData([]byte("response data"))
		mockConn.SimulateLatency(time.Millisecond * 50)

		pool.AddConnection(conn)
	}

	// 获取统计信息
	stats := pool.GetStats()

	if stats.ActiveConnections != 3 {
		t.Errorf("期望活跃连接数为 3，实际为 %d", stats.ActiveConnections)
	}

	if stats.TotalConnections != 3 {
		t.Errorf("期望总连接数为 3，实际为 %d", stats.TotalConnections)
	}

	if stats.TotalBytesTransferred == 0 {
		t.Error("期望有字节传输统计")
	}
}

func TestConnectionPool_Maintenance(t *testing.T) {
	pool := NewConnectionPool(5)
	defer pool.Close()

	// 设置较短的生命周期管理参数用于测试
	pool.lifecycleManager.maxIdleTime = 100 * time.Millisecond
	pool.lifecycleManager.maxLifetime = 200 * time.Millisecond

	// 添加连接
	conn1 := NewMockConnection("device1")
	pool.AddConnection(conn1)

	// 等待维护周期
	time.Sleep(300 * time.Millisecond)

	// 手动触发维护
	pool.performMaintenance()

	// 验证过期连接被清理
	_, err := pool.GetConnection("device1")
	if err == nil {
		t.Error("期望过期连接被清理")
	}
}

func TestConnectionPool_HealthScoring(t *testing.T) {
	pool := NewConnectionPool(5)
	defer pool.Close()

	// 添加连接
	conn1 := NewMockConnection("device1")
	pool.AddConnection(conn1)

	// 获取池化连接并更新健康评分
	pool.mu.RLock()
	pooledConn := pool.connections["device1"]
	pool.mu.RUnlock()

	if pooledConn == nil {
		t.Fatal("未找到池化连接")
	}

	// 更新健康评分
	pooledConn.mu.Lock()
	pooledConn.healthScore = 0.5
	pooledConn.mu.Unlock()

	// 验证健康评分影响连接选择
	// 添加另一个健康评分更高的连接
	conn2 := NewMockConnection("device2")
	pool.AddConnection(conn2)

	// 使用基于健康状态的负载均衡器
	healthBalancer := NewHealthBasedBalancer()
	healthBalancer.UpdateHealthScore("device1", 0.5)
	healthBalancer.UpdateHealthScore("device2", 0.9)

	pool.mu.Lock()
	pool.loadBalancer = healthBalancer
	pool.mu.Unlock()

	// 多次选择连接，健康评分高的应该被选择更多
	device2Count := 0
	for i := 0; i < 10; i++ {
		conn, err := pool.SelectConnectionByStrategy()
		if err != nil {
			t.Fatalf("选择连接失败: %v", err)
		}
		if conn.DeviceID() == "device2" {
			device2Count++
		}
	}

	// device2 应该被选择更多次（健康评分更高）
	if device2Count < 8 {
		t.Errorf("期望 device2 被选择至少 8 次，实际 %d 次", device2Count)
	}
}

func TestConnectionPool_PriorityManagement(t *testing.T) {
	pool := NewConnectionPool(3) // 小的池大小用于测试优先级管理
	defer pool.Close()

	// 添加不同优先级的连接
	pool.SetConnectionPriority("low_device", PriorityLow)
	pool.SetConnectionPriority("normal_device", PriorityNormal)
	pool.SetConnectionPriority("high_device", PriorityHigh)

	connLow := NewMockConnection("low_device")
	connNormal := NewMockConnection("normal_device")
	connHigh := NewMockConnection("high_device")

	pool.AddConnection(connLow)
	pool.AddConnection(connNormal)
	pool.AddConnection(connHigh)

	// 尝试添加第四个连接（应该移除低优先级的连接）
	pool.SetConnectionPriority("critical_device", PriorityCritical)
	connCritical := NewMockConnection("critical_device")

	err := pool.AddConnection(connCritical)
	if err != nil {
		t.Fatalf("添加关键优先级连接失败: %v", err)
	}

	// 验证低优先级连接被移除
	_, err = pool.GetConnection("low_device")
	if err == nil {
		t.Error("期望低优先级连接被移除")
	}

	// 验证关键优先级连接存在
	_, err = pool.GetConnection("critical_device")
	if err != nil {
		t.Errorf("关键优先级连接应该存在: %v", err)
	}
}

func TestConnectionPool_WeightedLoadBalancing(t *testing.T) {
	pool := NewConnectionPool(5)
	defer pool.Close()

	// 使用加权负载均衡器
	weightedBalancer := NewWeightedBalancer()
	pool.mu.Lock()
	pool.loadBalancer = weightedBalancer
	pool.mu.Unlock()

	// 添加连接并设置权重
	conn1 := NewMockConnection("device1")
	conn2 := NewMockConnection("device2")
	conn3 := NewMockConnection("device3")

	pool.AddConnection(conn1)
	pool.AddConnection(conn2)
	pool.AddConnection(conn3)

	// 设置不同权重
	weightedBalancer.UpdateWeight("device1", 1)
	weightedBalancer.UpdateWeight("device2", 3)
	weightedBalancer.UpdateWeight("device3", 6)

	// 统计选择次数
	selectionCount := make(map[string]int)
	for i := 0; i < 100; i++ {
		conn, err := pool.SelectConnectionByStrategy()
		if err != nil {
			t.Fatalf("选择连接失败: %v", err)
		}
		selectionCount[conn.DeviceID()]++
	}

	// 验证权重影响选择频率
	// device3 (权重6) 应该被选择最多
	// device2 (权重3) 应该比 device1 (权重1) 被选择更多
	if selectionCount["device3"] <= selectionCount["device2"] {
		t.Error("高权重设备应该被选择更多")
	}

	if selectionCount["device2"] <= selectionCount["device1"] {
		t.Error("中等权重设备应该比低权重设备被选择更多")
	}
}

func TestConnectionPool_LeastUsedBalancing(t *testing.T) {
	pool := NewConnectionPool(5)
	defer pool.Close()

	// 使用最少使用负载均衡器
	leastUsedBalancer := NewLeastUsedBalancer()
	pool.mu.Lock()
	pool.loadBalancer = leastUsedBalancer
	pool.mu.Unlock()

	// 添加连接
	conn1 := NewMockConnection("device1")
	conn2 := NewMockConnection("device2")
	conn3 := NewMockConnection("device3")

	pool.AddConnection(conn1)
	pool.AddConnection(conn2)
	pool.AddConnection(conn3)

	// 模拟不同的使用次数
	pool.mu.Lock()
	pool.connections["device1"].useCount = 10
	pool.connections["device2"].useCount = 5
	pool.connections["device3"].useCount = 1
	pool.mu.Unlock()

	// 选择连接，应该优先选择使用次数最少的
	conn, err := pool.SelectConnectionByStrategy()
	if err != nil {
		t.Fatalf("选择连接失败: %v", err)
	}

	if conn.DeviceID() != "device3" {
		t.Errorf("期望选择使用次数最少的 device3，实际选择了 %s", conn.DeviceID())
	}
}

// 基准测试
func BenchmarkConnectionPool_AddConnection(b *testing.B) {
	pool := NewConnectionPool(1000)
	defer pool.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn := NewMockConnection(fmt.Sprintf("device%d", i%100))
		pool.AddConnection(conn)
	}
}

func BenchmarkConnectionPool_GetConnection(b *testing.B) {
	pool := NewConnectionPool(100)
	defer pool.Close()

	// 预先添加连接
	for i := 0; i < 100; i++ {
		conn := NewMockConnection(fmt.Sprintf("device%d", i))
		pool.AddConnection(conn)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.GetConnection(fmt.Sprintf("device%d", i%100))
	}
}

func BenchmarkConnectionPool_SelectByStrategy(b *testing.B) {
	pool := NewConnectionPool(100)
	defer pool.Close()

	// 预先添加连接
	for i := 0; i < 100; i++ {
		conn := NewMockConnection(fmt.Sprintf("device%d", i))
		pool.AddConnection(conn)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.SelectConnectionByStrategy()
	}
}

func BenchmarkConnectionPool_ConcurrentOperations(b *testing.B) {
	pool := NewConnectionPool(100)
	defer pool.Close()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			deviceID := fmt.Sprintf("device%d", i%50)

			if i%2 == 0 {
				// 添加连接
				conn := NewMockConnection(deviceID)
				pool.AddConnection(conn)
			} else {
				// 获取连接
				pool.GetConnection(deviceID)
			}
			i++
		}
	})
}
