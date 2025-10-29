package bluetooth

import (
	"context"
	"math"
	"sync"
	"time"
)

// ReconnectClient 重连客户端接口，避免泛型类型问题
type ReconnectClient interface {
	Connect(ctx context.Context, deviceID string) (Connection, error)
}

// ReconnectManager 重连管理器，实现指数退避重连策略
type ReconnectManager struct {
	config          ClientConfig
	reconnectTasks  map[string]*ReconnectTask
	mu              sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	callbackManager *CallbackManager
}

// ReconnectTask 重连任务
type ReconnectTask struct {
	DeviceID     string        // 设备ID
	Attempts     int           // 重连尝试次数
	NextAttempt  time.Time     // 下次重连时间
	BackoffDelay time.Duration // 当前退避延迟
	MaxDelay     time.Duration // 最大退避延迟
	Enabled      bool          // 是否启用重连
	LastError    error         // 最后一次错误
}

// NewReconnectManager 创建新的重连管理器
func NewReconnectManager(config ClientConfig) *ReconnectManager {
	return &ReconnectManager{
		config:          config,
		reconnectTasks:  make(map[string]*ReconnectTask),
		callbackManager: NewCallbackManager(),
	}
}

// Start 启动重连管理器
func (rm *ReconnectManager) Start(ctx context.Context, client ReconnectClient) {
	rm.ctx, rm.cancel = context.WithCancel(ctx)

	rm.wg.Add(1)
	go rm.reconnectWorker(client)
}

// Stop 停止重连管理器
func (rm *ReconnectManager) Stop() {
	if rm.cancel != nil {
		rm.cancel()
	}
	rm.wg.Wait()
}

// ScheduleReconnect 安排设备重连
func (rm *ReconnectManager) ScheduleReconnect(deviceID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	task, exists := rm.reconnectTasks[deviceID]
	if !exists {
		// 创建新的重连任务
		task = &ReconnectTask{
			DeviceID:     deviceID,
			Attempts:     0,
			BackoffDelay: rm.config.RetryInterval,
			MaxDelay:     30 * time.Minute, // 最大退避延迟30分钟
			Enabled:      true,
		}
		rm.reconnectTasks[deviceID] = task
	}

	// 如果任务已禁用，重新启用
	if !task.Enabled {
		task.Enabled = true
		task.Attempts = 0
		task.BackoffDelay = rm.config.RetryInterval
	}

	// 计算下次重连时间（指数退避）
	task.Attempts++
	if task.Attempts > rm.config.RetryAttempts && rm.config.RetryAttempts > 0 {
		// 达到最大重试次数，禁用重连
		task.Enabled = false
		return
	}

	// 指数退避算法：delay = base * 2^attempts，但不超过最大延迟
	backoffMultiplier := math.Pow(2, float64(task.Attempts-1))
	newDelay := time.Duration(float64(rm.config.RetryInterval) * backoffMultiplier)
	if newDelay > task.MaxDelay {
		newDelay = task.MaxDelay
	}
	task.BackoffDelay = newDelay
	task.NextAttempt = time.Now().Add(task.BackoffDelay)
}

// StopReconnect 停止指定设备的重连
func (rm *ReconnectManager) StopReconnect(deviceID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if task, exists := rm.reconnectTasks[deviceID]; exists {
		task.Enabled = false
	}
}

// GetReconnectStatus 获取设备的重连状态
func (rm *ReconnectManager) GetReconnectStatus(deviceID string) *ReconnectTask {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if task, exists := rm.reconnectTasks[deviceID]; exists {
		// 返回任务的副本，避免并发修改
		return &ReconnectTask{
			DeviceID:     task.DeviceID,
			Attempts:     task.Attempts,
			NextAttempt:  task.NextAttempt,
			BackoffDelay: task.BackoffDelay,
			MaxDelay:     task.MaxDelay,
			Enabled:      task.Enabled,
			LastError:    task.LastError,
		}
	}
	return nil
}

// GetAllReconnectTasks 获取所有重连任务
func (rm *ReconnectManager) GetAllReconnectTasks() map[string]*ReconnectTask {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	tasks := make(map[string]*ReconnectTask)
	for deviceID, task := range rm.reconnectTasks {
		tasks[deviceID] = &ReconnectTask{
			DeviceID:     task.DeviceID,
			Attempts:     task.Attempts,
			NextAttempt:  task.NextAttempt,
			BackoffDelay: task.BackoffDelay,
			MaxDelay:     task.MaxDelay,
			Enabled:      task.Enabled,
			LastError:    task.LastError,
		}
	}
	return tasks
}

// ClearReconnectTask 清除指定设备的重连任务
func (rm *ReconnectManager) ClearReconnectTask(deviceID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	delete(rm.reconnectTasks, deviceID)
}

// ResetReconnectTask 重置指定设备的重连任务
func (rm *ReconnectManager) ResetReconnectTask(deviceID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if task, exists := rm.reconnectTasks[deviceID]; exists {
		task.Attempts = 0
		task.BackoffDelay = rm.config.RetryInterval
		task.Enabled = true
		task.LastError = nil
	}
}

// reconnectWorker 重连工作器goroutine
func (rm *ReconnectManager) reconnectWorker(client ReconnectClient) {
	defer rm.wg.Done()

	ticker := time.NewTicker(1 * time.Second) // 每秒检查一次重连任务
	defer ticker.Stop()

	for {
		select {
		case <-rm.ctx.Done():
			return
		case <-ticker.C:
			rm.processReconnectTasks(client)
		}
	}
}

// processReconnectTasks 处理重连任务
func (rm *ReconnectManager) processReconnectTasks(client ReconnectClient) {
	now := time.Now()

	rm.mu.RLock()
	tasksToProcess := make([]*ReconnectTask, 0)

	for _, task := range rm.reconnectTasks {
		if task.Enabled && now.After(task.NextAttempt) {
			tasksToProcess = append(tasksToProcess, task)
		}
	}
	rm.mu.RUnlock()

	// 处理需要重连的任务
	for _, task := range tasksToProcess {
		rm.attemptReconnect(client, task)
	}
}

// attemptReconnect 尝试重连
func (rm *ReconnectManager) attemptReconnect(client ReconnectClient, task *ReconnectTask) {
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(rm.ctx, rm.config.ConnectTimeout)
	defer cancel()

	// 尝试连接
	conn, err := client.Connect(ctx, task.DeviceID)
	if err != nil {
		// 连接失败，更新任务状态
		rm.mu.Lock()
		task.LastError = err
		rm.mu.Unlock()

		// 安排下次重连
		rm.ScheduleReconnect(task.DeviceID)

		// 通知错误回调
		if bluetoothErr, ok := err.(*BluetoothError); ok {
			rm.callbackManager.NotifyError(bluetoothErr)
		} else {
			rm.callbackManager.NotifyError(WrapError(err, ErrCodeConnectionFailed, "重连失败", task.DeviceID, "reconnect"))
		}
		return
	}

	// 连接成功，清理重连任务
	rm.mu.Lock()
	task.Enabled = false
	task.Attempts = 0
	task.LastError = nil
	rm.mu.Unlock()

	// 通知连接成功
	device := Device{
		ID:       task.DeviceID,
		LastSeen: time.Now(),
	}
	rm.callbackManager.NotifyDeviceEvent(DeviceEventConnected, device)

	// 记录连接成功（可以添加日志）
	_ = conn // 避免未使用变量警告
}

// RegisterCallback 注册回调函数
func (rm *ReconnectManager) RegisterCallback(callback any) {
	switch cb := callback.(type) {
	case DeviceCallback:
		rm.callbackManager.RegisterDeviceCallback(cb)
	case ErrorCallback:
		rm.callbackManager.RegisterErrorCallback(cb)
	}
}

// GetStatistics 获取重连统计信息
func (rm *ReconnectManager) GetStatistics() ReconnectStatistics {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	stats := ReconnectStatistics{
		TotalTasks:    len(rm.reconnectTasks),
		ActiveTasks:   0,
		DisabledTasks: 0,
		TotalAttempts: 0,
	}

	for _, task := range rm.reconnectTasks {
		if task.Enabled {
			stats.ActiveTasks++
		} else {
			stats.DisabledTasks++
		}
		stats.TotalAttempts += task.Attempts
	}

	return stats
}

// ReconnectStatistics 重连统计信息
type ReconnectStatistics struct {
	TotalTasks    int `json:"total_tasks"`    // 总任务数
	ActiveTasks   int `json:"active_tasks"`   // 活跃任务数
	DisabledTasks int `json:"disabled_tasks"` // 禁用任务数
	TotalAttempts int `json:"total_attempts"` // 总尝试次数
}
