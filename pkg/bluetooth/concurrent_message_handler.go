package bluetooth

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// ConcurrentMessageHandler 并发消息处理器，利用 Go 1.25 并发特性
type ConcurrentMessageHandler[T any] struct {
	*ReliableMessageHandler[T]                     // 嵌入可靠消息处理器
	workerPool                 *WorkerPool[T]      // 工作池
	priorityQueues             *PriorityQueues[T]  // 优先级队列
	flowController             *FlowController     // 流控制器
	concurrencyLimiter         *ConcurrencyLimiter // 并发限制器
	processingStats            *ProcessingStats    // 处理统计
	ctx                        context.Context     // 上下文
	cancel                     context.CancelFunc  // 取消函数
	mu                         sync.RWMutex        // 读写锁
}

// WorkerPool 工作池，利用 Go 1.25 优化的 goroutine 调度
type WorkerPool[T any] struct {
	workers     int                   // 工作协程数量
	workQueue   chan Message[T]       // 工作队列
	resultQueue chan ProcessResult[T] // 结果队列
	processor   MessageProcessor[T]   // 消息处理器
	ctx         context.Context       // 上下文
	cancel      context.CancelFunc    // 取消函数
	wg          sync.WaitGroup        // 等待组
	stats       WorkerPoolStats       // 工作池统计
	mu          sync.RWMutex          // 读写锁
}

// MessageProcessor 消息处理器接口
type MessageProcessor[T any] interface {
	Process(ctx context.Context, message Message[T]) (ProcessResult[T], error)
}

// ProcessResult 处理结果
type ProcessResult[T any] struct {
	MessageID   string        `json:"message_id"`   // 消息ID
	Success     bool          `json:"success"`      // 是否成功
	ProcessTime time.Duration `json:"process_time"` // 处理时间
	Error       error         `json:"error"`        // 错误信息
	Result      interface{}   `json:"result"`       // 处理结果
}

// WorkerPoolStats 工作池统计
type WorkerPoolStats struct {
	ActiveWorkers    int32         `json:"active_workers"`     // 活跃工作协程数
	QueuedMessages   int32         `json:"queued_messages"`    // 队列中的消息数
	ProcessedTotal   uint64        `json:"processed_total"`    // 总处理消息数
	SuccessCount     uint64        `json:"success_count"`      // 成功处理数
	ErrorCount       uint64        `json:"error_count"`        // 错误处理数
	AvgProcessTime   time.Duration `json:"avg_process_time"`   // 平均处理时间
	MaxProcessTime   time.Duration `json:"max_process_time"`   // 最大处理时间
	ThroughputPerSec float64       `json:"throughput_per_sec"` // 每秒吞吐量
}

// PriorityQueues 优先级队列系统
type PriorityQueues[T any] struct {
	queues    map[Priority]*MessageQueue[T] // 优先级队列映射
	scheduler *PriorityScheduler[T]         // 优先级调度器
	mu        sync.RWMutex                  // 读写锁
}

// PriorityScheduler 优先级调度器
type PriorityScheduler[T any] struct {
	queues      map[Priority]*MessageQueue[T] // 队列映射
	weights     map[Priority]int              // 优先级权重
	counters    map[Priority]int              // 计数器
	outputQueue chan Message[T]               // 输出队列
	ctx         context.Context               // 上下文
	cancel      context.CancelFunc            // 取消函数
	mu          sync.RWMutex                  // 读写锁
}

// FlowController 流控制器
type FlowController struct {
	maxRate      float64       // 最大速率（消息/秒）
	currentRate  float64       // 当前速率
	windowSize   time.Duration // 时间窗口大小
	messageCount int64         // 消息计数
	lastReset    time.Time     // 上次重置时间
	rateLimiter  *RateLimiter  // 速率限制器
	mu           sync.RWMutex  // 读写锁
}

// RateLimiter 速率限制器
type RateLimiter struct {
	tokens     float64    // 令牌数
	maxTokens  float64    // 最大令牌数
	refillRate float64    // 补充速率
	lastRefill time.Time  // 上次补充时间
	mu         sync.Mutex // 互斥锁
}

// ConcurrencyLimiter 并发限制器
type ConcurrencyLimiter struct {
	maxConcurrency int32         // 最大并发数
	current        int32         // 当前并发数
	semaphore      chan struct{} // 信号量
}

// ProcessingStats 处理统计信息
type ProcessingStats struct {
	TotalMessages     uint64        `json:"total_messages"`     // 总消息数
	ProcessedMessages uint64        `json:"processed_messages"` // 已处理消息数
	FailedMessages    uint64        `json:"failed_messages"`    // 失败消息数
	AvgLatency        time.Duration `json:"avg_latency"`        // 平均延迟
	MaxLatency        time.Duration `json:"max_latency"`        // 最大延迟
	Throughput        float64       `json:"throughput"`         // 吞吐量
	CPUUsage          float64       `json:"cpu_usage"`          // CPU使用率
	MemoryUsage       uint64        `json:"memory_usage"`       // 内存使用量
	GoroutineCount    int           `json:"goroutine_count"`    // Goroutine数量
}

// NewConcurrentMessageHandler 创建并发消息处理器
func NewConcurrentMessageHandler[T any](
	queueSize int,
	workerCount int,
	maxRate float64,
	maxConcurrency int,
) *ConcurrentMessageHandler[T] {

	reliableHandler := NewReliableMessageHandler[T](
		queueSize,
		5*time.Second, // ACK超时
		3,             // 最大重试
		1*time.Second, // 重试间隔
	)

	return &ConcurrentMessageHandler[T]{
		ReliableMessageHandler: reliableHandler,
		workerPool:             NewWorkerPool[T](workerCount, queueSize),
		priorityQueues:         NewPriorityQueues[T](queueSize),
		flowController:         NewFlowController(maxRate, time.Second),
		concurrencyLimiter:     NewConcurrencyLimiter(int32(maxConcurrency)),
		processingStats:        &ProcessingStats{},
	}
}

// NewWorkerPool 创建工作池
func NewWorkerPool[T any](workers, queueSize int) *WorkerPool[T] {
	return &WorkerPool[T]{
		workers:     workers,
		workQueue:   make(chan Message[T], queueSize),
		resultQueue: make(chan ProcessResult[T], queueSize),
	}
}

// NewPriorityQueues 创建优先级队列
func NewPriorityQueues[T any](queueSize int) *PriorityQueues[T] {
	queues := make(map[Priority]*MessageQueue[T])
	queues[PriorityCritical] = NewMessageQueue[T](queueSize / 4)
	queues[PriorityHigh] = NewMessageQueue[T](queueSize / 4)
	queues[PriorityNormal] = NewMessageQueue[T](queueSize / 2)
	queues[PriorityLow] = NewMessageQueue[T](queueSize / 4)

	return &PriorityQueues[T]{
		queues:    queues,
		scheduler: NewPriorityScheduler[T](queues, queueSize),
	}
}

// NewPriorityScheduler 创建优先级调度器
func NewPriorityScheduler[T any](queues map[Priority]*MessageQueue[T], outputSize int) *PriorityScheduler[T] {
	weights := map[Priority]int{
		PriorityCritical: 8, // 关键优先级权重最高
		PriorityHigh:     4, // 高优先级
		PriorityNormal:   2, // 普通优先级
		PriorityLow:      1, // 低优先级权重最低
	}

	return &PriorityScheduler[T]{
		queues:      queues,
		weights:     weights,
		counters:    make(map[Priority]int),
		outputQueue: make(chan Message[T], outputSize),
	}
}

// NewFlowController 创建流控制器
func NewFlowController(maxRate float64, windowSize time.Duration) *FlowController {
	return &FlowController{
		maxRate:     maxRate,
		windowSize:  windowSize,
		lastReset:   time.Now(),
		rateLimiter: NewRateLimiter(maxRate, maxRate),
	}
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(maxTokens, refillRate float64) *RateLimiter {
	return &RateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// NewConcurrencyLimiter 创建并发限制器
func NewConcurrencyLimiter(maxConcurrency int32) *ConcurrencyLimiter {
	return &ConcurrencyLimiter{
		maxConcurrency: maxConcurrency,
		semaphore:      make(chan struct{}, maxConcurrency),
	}
}

// Start 启动并发消息处理器
func (cmh *ConcurrentMessageHandler[T]) Start(ctx context.Context) error {
	cmh.mu.Lock()
	defer cmh.mu.Unlock()

	// 启动可靠消息处理器
	err := cmh.ReliableMessageHandler.Start(ctx)
	if err != nil {
		return err
	}

	cmh.ctx, cmh.cancel = context.WithCancel(ctx)

	// 启动工作池
	err = cmh.workerPool.Start(cmh.ctx, &DefaultMessageProcessor[T]{})
	if err != nil {
		return err
	}

	// 启动优先级调度器
	err = cmh.priorityQueues.Start(cmh.ctx)
	if err != nil {
		return err
	}

	// 启动统计收集
	go cmh.collectStats()

	// 启动消息分发
	go cmh.distributeMessages()

	return nil
}

// Stop 停止并发消息处理器
func (cmh *ConcurrentMessageHandler[T]) Stop() error {
	cmh.mu.Lock()
	defer cmh.mu.Unlock()

	if cmh.cancel != nil {
		cmh.cancel()
	}

	// 停止工作池
	cmh.workerPool.Stop()

	// 停止优先级队列
	cmh.priorityQueues.Stop()

	// 停止可靠消息处理器
	return cmh.ReliableMessageHandler.Stop()
}

// SendConcurrent 并发发送消息
func (cmh *ConcurrentMessageHandler[T]) SendConcurrent(ctx context.Context, deviceID string, payload T, priority Priority) error {
	// 流控制检查
	if !cmh.flowController.AllowMessage() {
		return NewBluetoothError(ErrCodeResourceBusy, "消息发送速率超限")
	}

	// 并发限制检查
	if !cmh.concurrencyLimiter.Acquire() {
		return NewBluetoothError(ErrCodeResourceBusy, "并发处理数量超限")
	}
	defer cmh.concurrencyLimiter.Release()

	// 发送消息
	return cmh.ReliableMessageHandler.SendReliable(ctx, deviceID, payload, priority)
}

// ProcessMessageConcurrent 并发处理消息
func (cmh *ConcurrentMessageHandler[T]) ProcessMessageConcurrent(message Message[T]) error {
	startTime := time.Now()

	// 根据优先级将消息加入相应队列
	err := cmh.priorityQueues.Enqueue(message)
	if err != nil {
		atomic.AddUint64(&cmh.processingStats.FailedMessages, 1)
		return err
	}

	// 更新统计信息
	atomic.AddUint64(&cmh.processingStats.TotalMessages, 1)

	// 计算延迟
	latency := time.Since(startTime)
	cmh.updateLatencyStats(latency)

	return nil
}

// Start 启动工作池
func (wp *WorkerPool[T]) Start(ctx context.Context, processor MessageProcessor[T]) error {
	wp.ctx, wp.cancel = context.WithCancel(ctx)
	wp.processor = processor

	// 启动工作协程
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}

	// 启动结果处理协程
	go wp.handleResults()

	return nil
}

// Stop 停止工作池
func (wp *WorkerPool[T]) Stop() {
	if wp.cancel != nil {
		wp.cancel()
	}
	wp.wg.Wait()
}

// worker 工作协程
func (wp *WorkerPool[T]) worker(id int) {
	defer wp.wg.Done()

	atomic.AddInt32(&wp.stats.ActiveWorkers, 1)
	defer atomic.AddInt32(&wp.stats.ActiveWorkers, -1)

	for {
		select {
		case <-wp.ctx.Done():
			return
		case message := <-wp.workQueue:
			wp.processMessage(message)
		}
	}
}

// processMessage 处理消息
func (wp *WorkerPool[T]) processMessage(message Message[T]) {
	startTime := time.Now()

	result, err := wp.processor.Process(wp.ctx, message)

	processTime := time.Since(startTime)
	result.ProcessTime = processTime
	result.MessageID = message.ID

	if err != nil {
		result.Success = false
		result.Error = err
		atomic.AddUint64(&wp.stats.ErrorCount, 1)
	} else {
		result.Success = true
		atomic.AddUint64(&wp.stats.SuccessCount, 1)
	}

	atomic.AddUint64(&wp.stats.ProcessedTotal, 1)
	wp.updateProcessTimeStats(processTime)

	// 发送结果
	select {
	case wp.resultQueue <- result:
	case <-wp.ctx.Done():
		return
	}
}

// handleResults 处理结果
func (wp *WorkerPool[T]) handleResults() {
	for {
		select {
		case <-wp.ctx.Done():
			return
		case result := <-wp.resultQueue:
			// 这里可以添加结果处理逻辑
			_ = result
		}
	}
}

// updateProcessTimeStats 更新处理时间统计
func (wp *WorkerPool[T]) updateProcessTimeStats(processTime time.Duration) {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	// 更新平均处理时间
	if wp.stats.AvgProcessTime == 0 {
		wp.stats.AvgProcessTime = processTime
	} else {
		wp.stats.AvgProcessTime = (wp.stats.AvgProcessTime + processTime) / 2
	}

	// 更新最大处理时间
	if processTime > wp.stats.MaxProcessTime {
		wp.stats.MaxProcessTime = processTime
	}
}

// Start 启动优先级队列
func (pq *PriorityQueues[T]) Start(ctx context.Context) error {
	return pq.scheduler.Start(ctx)
}

// Stop 停止优先级队列
func (pq *PriorityQueues[T]) Stop() {
	pq.scheduler.Stop()

	pq.mu.Lock()
	defer pq.mu.Unlock()

	for _, queue := range pq.queues {
		queue.Close()
	}
}

// Enqueue 根据优先级入队消息
func (pq *PriorityQueues[T]) Enqueue(message Message[T]) error {
	pq.mu.RLock()
	queue, exists := pq.queues[message.Metadata.Priority]
	pq.mu.RUnlock()

	if !exists {
		return NewBluetoothError(ErrCodeInvalidParameter, "无效的消息优先级")
	}

	return queue.Enqueue(message)
}

// Start 启动优先级调度器
func (ps *PriorityScheduler[T]) Start(ctx context.Context) error {
	ps.ctx, ps.cancel = context.WithCancel(ctx)
	go ps.schedule()
	return nil
}

// Stop 停止优先级调度器
func (ps *PriorityScheduler[T]) Stop() {
	if ps.cancel != nil {
		ps.cancel()
	}
}

// schedule 调度消息
func (ps *PriorityScheduler[T]) schedule() {
	ticker := time.NewTicker(10 * time.Millisecond) // 高频调度
	defer ticker.Stop()

	for {
		select {
		case <-ps.ctx.Done():
			return
		case <-ticker.C:
			ps.scheduleNext()
		}
	}
}

// scheduleNext 调度下一个消息
func (ps *PriorityScheduler[T]) scheduleNext() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// 按优先级顺序检查队列
	priorities := []Priority{PriorityCritical, PriorityHigh, PriorityNormal, PriorityLow}

	for _, priority := range priorities {
		queue := ps.queues[priority]
		weight := ps.weights[priority]
		counter := ps.counters[priority]

		// 检查是否应该处理此优先级的消息
		if counter < weight {
			select {
			case message := <-queue.Dequeue():
				ps.counters[priority]++

				// 发送到输出队列
				select {
				case ps.outputQueue <- message:
				case <-ps.ctx.Done():
					return
				}
				return
			default:
				// 队列为空，继续检查下一个优先级
			}
		}
	}

	// 重置计数器
	for priority := range ps.counters {
		ps.counters[priority] = 0
	}
}

// AllowMessage 检查是否允许发送消息
func (fc *FlowController) AllowMessage() bool {
	return fc.rateLimiter.Allow()
}

// Allow 检查是否允许操作
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()

	// 补充令牌
	rl.tokens += elapsed * rl.refillRate
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}
	rl.lastRefill = now

	// 检查是否有足够的令牌
	if rl.tokens >= 1.0 {
		rl.tokens -= 1.0
		return true
	}

	return false
}

// Acquire 获取并发许可
func (cl *ConcurrencyLimiter) Acquire() bool {
	select {
	case cl.semaphore <- struct{}{}:
		atomic.AddInt32(&cl.current, 1)
		return true
	default:
		return false
	}
}

// Release 释放并发许可
func (cl *ConcurrencyLimiter) Release() {
	select {
	case <-cl.semaphore:
		atomic.AddInt32(&cl.current, -1)
	default:
		// 不应该发生
	}
}

// collectStats 收集统计信息
func (cmh *ConcurrentMessageHandler[T]) collectStats() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cmh.ctx.Done():
			return
		case <-ticker.C:
			cmh.updateSystemStats()
		}
	}
}

// updateSystemStats 更新系统统计信息
func (cmh *ConcurrentMessageHandler[T]) updateSystemStats() {
	cmh.mu.Lock()
	defer cmh.mu.Unlock()

	// 更新 Goroutine 数量
	cmh.processingStats.GoroutineCount = runtime.NumGoroutine()

	// 计算吞吐量
	processed := atomic.LoadUint64(&cmh.processingStats.ProcessedMessages)
	total := atomic.LoadUint64(&cmh.processingStats.TotalMessages)

	if total > 0 {
		cmh.processingStats.Throughput = float64(processed) / float64(total) * 100
	}
}

// updateLatencyStats 更新延迟统计
func (cmh *ConcurrentMessageHandler[T]) updateLatencyStats(latency time.Duration) {
	cmh.mu.Lock()
	defer cmh.mu.Unlock()

	// 更新平均延迟
	if cmh.processingStats.AvgLatency == 0 {
		cmh.processingStats.AvgLatency = latency
	} else {
		cmh.processingStats.AvgLatency = (cmh.processingStats.AvgLatency + latency) / 2
	}

	// 更新最大延迟
	if latency > cmh.processingStats.MaxLatency {
		cmh.processingStats.MaxLatency = latency
	}
}

// distributeMessages 分发消息到工作池
func (cmh *ConcurrentMessageHandler[T]) distributeMessages() {
	for {
		select {
		case <-cmh.ctx.Done():
			return
		case message := <-cmh.priorityQueues.scheduler.outputQueue:
			select {
			case cmh.workerPool.workQueue <- message:
			case <-cmh.ctx.Done():
				return
			}
		}
	}
}

// GetConcurrentStats 获取并发处理统计信息
func (cmh *ConcurrentMessageHandler[T]) GetConcurrentStats() ConcurrentStats {
	cmh.mu.RLock()
	defer cmh.mu.RUnlock()

	return ConcurrentStats{
		ProcessingStats: *cmh.processingStats,
		WorkerPoolStats: cmh.workerPool.stats,
		ConcurrentCount: atomic.LoadInt32(&cmh.concurrencyLimiter.current),
		MaxConcurrency:  cmh.concurrencyLimiter.maxConcurrency,
		FlowControlRate: cmh.flowController.currentRate,
		MaxFlowRate:     cmh.flowController.maxRate,
	}
}

// ConcurrentStats 并发统计信息
type ConcurrentStats struct {
	ProcessingStats ProcessingStats `json:"processing_stats"`  // 处理统计
	WorkerPoolStats WorkerPoolStats `json:"worker_pool_stats"` // 工作池统计
	ConcurrentCount int32           `json:"concurrent_count"`  // 当前并发数
	MaxConcurrency  int32           `json:"max_concurrency"`   // 最大并发数
	FlowControlRate float64         `json:"flow_control_rate"` // 流控速率
	MaxFlowRate     float64         `json:"max_flow_rate"`     // 最大流控速率
}

// DefaultMessageProcessor 默认消息处理器
type DefaultMessageProcessor[T any] struct{}

// Process 处理消息
func (dmp *DefaultMessageProcessor[T]) Process(ctx context.Context, message Message[T]) (ProcessResult[T], error) {
	// 这里是默认的消息处理逻辑
	// 实际应用中应该根据消息类型进行不同的处理

	return ProcessResult[T]{
		MessageID: message.ID,
		Success:   true,
		Result:    "processed",
	}, nil
}
