package bluetooth

import (
	"math/rand"
	"sync"
	"time"
)

// RoundRobinBalancer 轮询负载均衡器
type RoundRobinBalancer struct {
	mu      sync.Mutex
	current int
}

// WeightedBalancer 加权负载均衡器
type WeightedBalancer struct {
	mu      sync.RWMutex
	weights map[string]int
}

// LeastUsedBalancer 最少使用负载均衡器
type LeastUsedBalancer struct{}

// HealthBasedBalancer 基于健康状态的负载均衡器
type HealthBasedBalancer struct {
	mu           sync.RWMutex
	healthScores map[string]float64
}

// NewRoundRobinBalancer 创建轮询负载均衡器
func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{
		current: 0,
	}
}

// SelectConnection 轮询选择连接
func (rb *RoundRobinBalancer) SelectConnection(connections []*PooledConnection) *PooledConnection {
	if len(connections) == 0 {
		return nil
	}

	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.current >= len(connections) {
		rb.current = 0
	}

	selected := connections[rb.current]
	rb.current++

	return selected
}

// UpdateWeight 更新权重（轮询策略不使用权重）
func (rb *RoundRobinBalancer) UpdateWeight(deviceID string, weight int) {
	// 轮询策略不需要权重
}

// GetStrategy 获取策略类型
func (rb *RoundRobinBalancer) GetStrategy() LoadBalanceStrategy {
	return StrategyRoundRobin
}

// NewWeightedBalancer 创建加权负载均衡器
func NewWeightedBalancer() *WeightedBalancer {
	return &WeightedBalancer{
		weights: make(map[string]int),
	}
}

// SelectConnection 根据权重选择连接
func (wb *WeightedBalancer) SelectConnection(connections []*PooledConnection) *PooledConnection {
	if len(connections) == 0 {
		return nil
	}

	wb.mu.RLock()
	defer wb.mu.RUnlock()

	// 计算总权重
	totalWeight := 0
	for _, conn := range connections {
		weight := wb.getConnectionWeight(conn)
		totalWeight += weight
	}

	if totalWeight == 0 {
		// 如果没有权重，随机选择
		return connections[rand.Intn(len(connections))]
	}

	// 根据权重随机选择
	random := rand.Intn(totalWeight)
	currentWeight := 0

	for _, conn := range connections {
		weight := wb.getConnectionWeight(conn)
		currentWeight += weight
		if random < currentWeight {
			return conn
		}
	}

	// 默认返回第一个连接
	return connections[0]
}

// UpdateWeight 更新设备权重
func (wb *WeightedBalancer) UpdateWeight(deviceID string, weight int) {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	if weight <= 0 {
		delete(wb.weights, deviceID)
	} else {
		wb.weights[deviceID] = weight
	}
}

// GetStrategy 获取策略类型
func (wb *WeightedBalancer) GetStrategy() LoadBalanceStrategy {
	return StrategyWeighted
}

// getConnectionWeight 获取连接权重
func (wb *WeightedBalancer) getConnectionWeight(conn *PooledConnection) int {
	deviceID := conn.Connection.DeviceID()
	if weight, exists := wb.weights[deviceID]; exists {
		return weight
	}
	return conn.weight // 使用连接自身的权重
}

// NewLeastUsedBalancer 创建最少使用负载均衡器
func NewLeastUsedBalancer() *LeastUsedBalancer {
	return &LeastUsedBalancer{}
}

// SelectConnection 选择使用次数最少的连接
func (lub *LeastUsedBalancer) SelectConnection(connections []*PooledConnection) *PooledConnection {
	if len(connections) == 0 {
		return nil
	}

	var selected *PooledConnection
	minUseCount := uint64(^uint64(0)) // 最大值

	for _, conn := range connections {
		conn.mu.RLock()
		useCount := conn.useCount
		conn.mu.RUnlock()

		if useCount < minUseCount {
			minUseCount = useCount
			selected = conn
		}
	}

	return selected
}

// UpdateWeight 更新权重（最少使用策略不使用权重）
func (lub *LeastUsedBalancer) UpdateWeight(deviceID string, weight int) {
	// 最少使用策略不需要权重
}

// GetStrategy 获取策略类型
func (lub *LeastUsedBalancer) GetStrategy() LoadBalanceStrategy {
	return StrategyLeastUsed
}

// NewHealthBasedBalancer 创建基于健康状态的负载均衡器
func NewHealthBasedBalancer() *HealthBasedBalancer {
	return &HealthBasedBalancer{
		healthScores: make(map[string]float64),
	}
}

// SelectConnection 根据健康状态选择连接
func (hbb *HealthBasedBalancer) SelectConnection(connections []*PooledConnection) *PooledConnection {
	if len(connections) == 0 {
		return nil
	}

	hbb.mu.RLock()
	defer hbb.mu.RUnlock()

	var selected *PooledConnection
	maxHealthScore := float64(-1)

	for _, conn := range connections {
		conn.mu.RLock()
		healthScore := conn.healthScore
		conn.mu.RUnlock()

		// 更新健康评分
		deviceID := conn.Connection.DeviceID()
		if storedScore, exists := hbb.healthScores[deviceID]; exists {
			healthScore = storedScore
		}

		if healthScore > maxHealthScore {
			maxHealthScore = healthScore
			selected = conn
		}
	}

	return selected
}

// UpdateWeight 更新权重（健康状态策略不使用权重）
func (hbb *HealthBasedBalancer) UpdateWeight(deviceID string, weight int) {
	// 健康状态策略不需要权重
}

// UpdateHealthScore 更新设备健康评分
func (hbb *HealthBasedBalancer) UpdateHealthScore(deviceID string, score float64) {
	hbb.mu.Lock()
	defer hbb.mu.Unlock()

	if score < 0 {
		score = 0
	} else if score > 1 {
		score = 1
	}

	hbb.healthScores[deviceID] = score
}

// GetHealthScore 获取设备健康评分
func (hbb *HealthBasedBalancer) GetHealthScore(deviceID string) float64 {
	hbb.mu.RLock()
	defer hbb.mu.RUnlock()

	if score, exists := hbb.healthScores[deviceID]; exists {
		return score
	}
	return 1.0 // 默认健康评分
}

// GetStrategy 获取策略类型
func (hbb *HealthBasedBalancer) GetStrategy() LoadBalanceStrategy {
	return StrategyHealthBased
}

// AdaptiveBalancer 自适应负载均衡器，结合多种策略
type AdaptiveBalancer struct {
	mu                 sync.RWMutex
	strategies         []LoadBalancer
	currentStrategy    int
	performanceMetrics map[LoadBalanceStrategy]*StrategyMetrics
	evaluationInterval time.Duration
	lastEvaluation     time.Time
}

// StrategyMetrics 策略性能指标
type StrategyMetrics struct {
	TotalRequests   uint64
	SuccessRequests uint64
	AverageLatency  time.Duration
	ErrorRate       float64
}

// NewAdaptiveBalancer 创建自适应负载均衡器
func NewAdaptiveBalancer() *AdaptiveBalancer {
	strategies := []LoadBalancer{
		NewRoundRobinBalancer(),
		NewWeightedBalancer(),
		NewLeastUsedBalancer(),
		NewHealthBasedBalancer(),
	}

	metrics := make(map[LoadBalanceStrategy]*StrategyMetrics)
	for _, strategy := range strategies {
		metrics[strategy.GetStrategy()] = &StrategyMetrics{}
	}

	return &AdaptiveBalancer{
		strategies:         strategies,
		currentStrategy:    0,
		performanceMetrics: metrics,
		evaluationInterval: 5 * time.Minute,
		lastEvaluation:     time.Now(),
	}
}

// SelectConnection 自适应选择连接
func (ab *AdaptiveBalancer) SelectConnection(connections []*PooledConnection) *PooledConnection {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	// 检查是否需要重新评估策略
	if time.Since(ab.lastEvaluation) > ab.evaluationInterval {
		ab.evaluateStrategies()
		ab.lastEvaluation = time.Now()
	}

	// 使用当前最优策略
	if ab.currentStrategy < len(ab.strategies) {
		return ab.strategies[ab.currentStrategy].SelectConnection(connections)
	}

	// 默认使用轮询策略
	return ab.strategies[0].SelectConnection(connections)
}

// UpdateWeight 更新权重
func (ab *AdaptiveBalancer) UpdateWeight(deviceID string, weight int) {
	ab.mu.RLock()
	defer ab.mu.RUnlock()

	// 更新所有支持权重的策略
	for _, strategy := range ab.strategies {
		strategy.UpdateWeight(deviceID, weight)
	}
}

// GetStrategy 获取当前策略类型
func (ab *AdaptiveBalancer) GetStrategy() LoadBalanceStrategy {
	ab.mu.RLock()
	defer ab.mu.RUnlock()

	if ab.currentStrategy < len(ab.strategies) {
		return ab.strategies[ab.currentStrategy].GetStrategy()
	}
	return StrategyRoundRobin
}

// evaluateStrategies 评估各种策略的性能
func (ab *AdaptiveBalancer) evaluateStrategies() {
	bestStrategy := 0
	bestScore := float64(-1)

	for i, strategy := range ab.strategies {
		metrics := ab.performanceMetrics[strategy.GetStrategy()]
		if metrics == nil {
			continue
		}

		// 计算策略评分（成功率 - 错误率 + 延迟因子）
		successRate := float64(metrics.SuccessRequests) / float64(metrics.TotalRequests)
		latencyFactor := 1.0 / (1.0 + float64(metrics.AverageLatency.Milliseconds())/1000.0)
		score := successRate - metrics.ErrorRate + latencyFactor*0.1

		if score > bestScore {
			bestScore = score
			bestStrategy = i
		}
	}

	ab.currentStrategy = bestStrategy
}

// RecordMetrics 记录策略性能指标
func (ab *AdaptiveBalancer) RecordMetrics(strategy LoadBalanceStrategy, success bool, latency time.Duration) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	metrics := ab.performanceMetrics[strategy]
	if metrics == nil {
		return
	}

	metrics.TotalRequests++
	if success {
		metrics.SuccessRequests++
	}

	// 更新平均延迟（简单移动平均）
	if metrics.TotalRequests == 1 {
		metrics.AverageLatency = latency
	} else {
		metrics.AverageLatency = (metrics.AverageLatency + latency) / 2
	}

	// 更新错误率
	metrics.ErrorRate = 1.0 - (float64(metrics.SuccessRequests) / float64(metrics.TotalRequests))
}

// GetMetrics 获取策略性能指标
func (ab *AdaptiveBalancer) GetMetrics() map[LoadBalanceStrategy]*StrategyMetrics {
	ab.mu.RLock()
	defer ab.mu.RUnlock()

	// 返回指标的副本
	result := make(map[LoadBalanceStrategy]*StrategyMetrics)
	for strategy, metrics := range ab.performanceMetrics {
		result[strategy] = &StrategyMetrics{
			TotalRequests:   metrics.TotalRequests,
			SuccessRequests: metrics.SuccessRequests,
			AverageLatency:  metrics.AverageLatency,
			ErrorRate:       metrics.ErrorRate,
		}
	}

	return result
}
