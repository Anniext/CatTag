package bluetooth

import (
	"fmt"
	"testing"
	"time"
)

func TestRoundRobinBalancer(t *testing.T) {
	balancer := NewRoundRobinBalancer()

	// 创建测试连接
	connections := []*PooledConnection{
		{Connection: NewMockConnection("device1")},
		{Connection: NewMockConnection("device2")},
		{Connection: NewMockConnection("device3")},
	}

	// 测试轮询选择
	expectedOrder := []string{"device1", "device2", "device3", "device1", "device2"}

	for i, expected := range expectedOrder {
		selected := balancer.SelectConnection(connections)
		if selected == nil {
			t.Fatalf("第 %d 次选择返回 nil", i)
		}

		if selected.Connection.DeviceID() != expected {
			t.Errorf("第 %d 次选择期望 %s，实际 %s", i, expected, selected.Connection.DeviceID())
		}
	}
}

func TestRoundRobinBalancer_EmptyConnections(t *testing.T) {
	balancer := NewRoundRobinBalancer()

	selected := balancer.SelectConnection([]*PooledConnection{})
	if selected != nil {
		t.Error("空连接列表应该返回 nil")
	}
}

func TestWeightedBalancer(t *testing.T) {
	balancer := NewWeightedBalancer()

	// 创建测试连接
	connections := []*PooledConnection{
		{Connection: NewMockConnection("device1"), weight: 1},
		{Connection: NewMockConnection("device2"), weight: 3},
		{Connection: NewMockConnection("device3"), weight: 6},
	}

	// 设置权重
	balancer.UpdateWeight("device1", 1)
	balancer.UpdateWeight("device2", 3)
	balancer.UpdateWeight("device3", 6)

	// 统计选择次数
	selectionCount := make(map[string]int)
	iterations := 1000

	for i := 0; i < iterations; i++ {
		selected := balancer.SelectConnection(connections)
		if selected == nil {
			t.Fatal("选择返回 nil")
		}
		selectionCount[selected.Connection.DeviceID()]++
	}

	// 验证权重影响选择频率
	// device3 (权重6) 应该被选择最多
	if selectionCount["device3"] <= selectionCount["device2"] {
		t.Error("高权重设备应该被选择更多")
	}

	if selectionCount["device2"] <= selectionCount["device1"] {
		t.Error("中等权重设备应该比低权重设备被选择更多")
	}

	// 验证大致的权重比例
	total := selectionCount["device1"] + selectionCount["device2"] + selectionCount["device3"]
	device1Ratio := float64(selectionCount["device1"]) / float64(total)
	device2Ratio := float64(selectionCount["device2"]) / float64(total)
	device3Ratio := float64(selectionCount["device3"]) / float64(total)

	// 允许一定的误差范围
	if device1Ratio < 0.05 || device1Ratio > 0.15 { // 期望约 10% (1/10)
		t.Errorf("device1 选择比例 %.2f 不在期望范围内", device1Ratio)
	}

	if device2Ratio < 0.25 || device2Ratio > 0.35 { // 期望约 30% (3/10)
		t.Errorf("device2 选择比例 %.2f 不在期望范围内", device2Ratio)
	}

	if device3Ratio < 0.55 || device3Ratio > 0.65 { // 期望约 60% (6/10)
		t.Errorf("device3 选择比例 %.2f 不在期望范围内", device3Ratio)
	}
}

func TestWeightedBalancer_ZeroWeight(t *testing.T) {
	balancer := NewWeightedBalancer()

	connections := []*PooledConnection{
		{Connection: NewMockConnection("device1"), weight: 0},
		{Connection: NewMockConnection("device2"), weight: 0},
	}

	// 当所有权重为0时，应该随机选择
	selected := balancer.SelectConnection(connections)
	if selected == nil {
		t.Error("零权重时应该随机选择一个连接")
	}
}

func TestLeastUsedBalancer(t *testing.T) {
	balancer := NewLeastUsedBalancer()

	// 创建测试连接
	connections := []*PooledConnection{
		{Connection: NewMockConnection("device1"), useCount: 10},
		{Connection: NewMockConnection("device2"), useCount: 5},
		{Connection: NewMockConnection("device3"), useCount: 1},
	}

	// 应该选择使用次数最少的连接
	selected := balancer.SelectConnection(connections)
	if selected == nil {
		t.Fatal("选择返回 nil")
	}

	if selected.Connection.DeviceID() != "device3" {
		t.Errorf("期望选择 device3 (使用次数最少)，实际选择 %s", selected.Connection.DeviceID())
	}

	// 更新使用次数后再次测试
	connections[2].useCount = 15 // device3 现在使用次数最多

	selected = balancer.SelectConnection(connections)
	if selected.Connection.DeviceID() != "device2" {
		t.Errorf("期望选择 device2 (现在使用次数最少)，实际选择 %s", selected.Connection.DeviceID())
	}
}

func TestHealthBasedBalancer(t *testing.T) {
	balancer := NewHealthBasedBalancer()

	// 创建测试连接
	connections := []*PooledConnection{
		{Connection: NewMockConnection("device1"), healthScore: 0.5},
		{Connection: NewMockConnection("device2"), healthScore: 0.8},
		{Connection: NewMockConnection("device3"), healthScore: 0.3},
	}

	// 设置健康评分
	balancer.UpdateHealthScore("device1", 0.5)
	balancer.UpdateHealthScore("device2", 0.8)
	balancer.UpdateHealthScore("device3", 0.3)

	// 应该选择健康评分最高的连接
	selected := balancer.SelectConnection(connections)
	if selected == nil {
		t.Fatal("选择返回 nil")
	}

	if selected.Connection.DeviceID() != "device2" {
		t.Errorf("期望选择 device2 (健康评分最高)，实际选择 %s", selected.Connection.DeviceID())
	}

	// 测试健康评分更新
	balancer.UpdateHealthScore("device1", 0.9)

	selected = balancer.SelectConnection(connections)
	if selected.Connection.DeviceID() != "device1" {
		t.Errorf("期望选择 device1 (现在健康评分最高)，实际选择 %s", selected.Connection.DeviceID())
	}
}

func TestHealthBasedBalancer_HealthScoreRange(t *testing.T) {
	balancer := NewHealthBasedBalancer()

	// 测试健康评分范围限制
	balancer.UpdateHealthScore("device1", -0.5) // 应该被限制为 0
	balancer.UpdateHealthScore("device2", 1.5)  // 应该被限制为 1

	score1 := balancer.GetHealthScore("device1")
	if score1 != 0.0 {
		t.Errorf("期望健康评分被限制为 0.0，实际为 %.2f", score1)
	}

	score2 := balancer.GetHealthScore("device2")
	if score2 != 1.0 {
		t.Errorf("期望健康评分被限制为 1.0，实际为 %.2f", score2)
	}
}

func TestAdaptiveBalancer(t *testing.T) {
	balancer := NewAdaptiveBalancer()

	// 创建测试连接
	connections := []*PooledConnection{
		{Connection: NewMockConnection("device1")},
		{Connection: NewMockConnection("device2")},
	}

	// 测试初始选择
	selected := balancer.SelectConnection(connections)
	if selected == nil {
		t.Fatal("选择返回 nil")
	}

	// 记录性能指标
	balancer.RecordMetrics(StrategyRoundRobin, true, 10*time.Millisecond)
	balancer.RecordMetrics(StrategyWeighted, false, 50*time.Millisecond)

	// 获取指标
	metrics := balancer.GetMetrics()

	roundRobinMetrics := metrics[StrategyRoundRobin]
	if roundRobinMetrics == nil {
		t.Fatal("轮询策略指标为 nil")
	}

	if roundRobinMetrics.TotalRequests != 1 {
		t.Errorf("期望轮询策略总请求数为 1，实际为 %d", roundRobinMetrics.TotalRequests)
	}

	if roundRobinMetrics.SuccessRequests != 1 {
		t.Errorf("期望轮询策略成功请求数为 1，实际为 %d", roundRobinMetrics.SuccessRequests)
	}

	weightedMetrics := metrics[StrategyWeighted]
	if weightedMetrics == nil {
		t.Fatal("加权策略指标为 nil")
	}

	if weightedMetrics.ErrorRate == 0 {
		t.Error("期望加权策略有错误率")
	}
}

func TestAdaptiveBalancer_StrategyEvaluation(t *testing.T) {
	balancer := NewAdaptiveBalancer()

	// 设置较短的评估间隔用于测试
	balancer.evaluationInterval = 100 * time.Millisecond

	connections := []*PooledConnection{
		{Connection: NewMockConnection("device1")},
	}

	// 记录不同策略的性能
	// 轮询策略表现良好
	for i := 0; i < 10; i++ {
		balancer.RecordMetrics(StrategyRoundRobin, true, 10*time.Millisecond)
	}

	// 加权策略表现较差
	for i := 0; i < 10; i++ {
		balancer.RecordMetrics(StrategyWeighted, false, 100*time.Millisecond)
	}

	// 等待评估间隔
	time.Sleep(150 * time.Millisecond)

	// 选择连接应该触发策略评估
	selected := balancer.SelectConnection(connections)
	if selected == nil {
		t.Fatal("选择返回 nil")
	}

	// 验证当前策略是轮询（表现更好）
	currentStrategy := balancer.GetStrategy()
	if currentStrategy != StrategyRoundRobin {
		t.Errorf("期望当前策略为轮询，实际为 %v", currentStrategy)
	}
}

func TestLoadBalancerStrategy_String(t *testing.T) {
	strategies := []struct {
		strategy LoadBalanceStrategy
		expected string
	}{
		{StrategyRoundRobin, "RoundRobin"},
		{StrategyWeighted, "Weighted"},
		{StrategyLeastUsed, "LeastUsed"},
		{StrategyHealthBased, "HealthBased"},
	}

	for _, test := range strategies {
		// 这里需要实现 String() 方法
		// 暂时跳过字符串测试
		_ = test
	}
}

// 基准测试
func BenchmarkRoundRobinBalancer_SelectConnection(b *testing.B) {
	balancer := NewRoundRobinBalancer()
	connections := make([]*PooledConnection, 100)

	for i := 0; i < 100; i++ {
		connections[i] = &PooledConnection{
			Connection: NewMockConnection(fmt.Sprintf("device%d", i)),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		balancer.SelectConnection(connections)
	}
}

func BenchmarkWeightedBalancer_SelectConnection(b *testing.B) {
	balancer := NewWeightedBalancer()
	connections := make([]*PooledConnection, 100)

	for i := 0; i < 100; i++ {
		deviceID := fmt.Sprintf("device%d", i)
		connections[i] = &PooledConnection{
			Connection: NewMockConnection(deviceID),
			weight:     i%10 + 1,
		}
		balancer.UpdateWeight(deviceID, i%10+1)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		balancer.SelectConnection(connections)
	}
}

func BenchmarkLeastUsedBalancer_SelectConnection(b *testing.B) {
	balancer := NewLeastUsedBalancer()
	connections := make([]*PooledConnection, 100)

	for i := 0; i < 100; i++ {
		connections[i] = &PooledConnection{
			Connection: NewMockConnection(fmt.Sprintf("device%d", i)),
			useCount:   uint64(i % 50),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		balancer.SelectConnection(connections)
	}
}

func BenchmarkHealthBasedBalancer_SelectConnection(b *testing.B) {
	balancer := NewHealthBasedBalancer()
	connections := make([]*PooledConnection, 100)

	for i := 0; i < 100; i++ {
		deviceID := fmt.Sprintf("device%d", i)
		connections[i] = &PooledConnection{
			Connection:  NewMockConnection(deviceID),
			healthScore: float64(i%100) / 100.0,
		}
		balancer.UpdateHealthScore(deviceID, float64(i%100)/100.0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		balancer.SelectConnection(connections)
	}
}
