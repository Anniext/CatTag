package bluetooth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TimeoutManager 超时管理器，管理连接超时和资源清理
type TimeoutManager struct {
	timeouts map[string]*TimeoutEntry
	config   TimeoutConfig
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// TimeoutConfig 超时配置
type TimeoutConfig struct {
	ConnectTimeout    time.Duration `json:"connect_timeout"`    // 连接超时时间
	ReadTimeout       time.Duration `json:"read_timeout"`       // 读取超时时间
	WriteTimeout      time.Duration `json:"write_timeout"`      // 写入超时时间
	IdleTimeout       time.Duration `json:"idle_timeout"`       // 空闲超时时间
	KeepAliveInterval time.Duration `json:"keepalive_interval"` // 保活间隔
	CheckInterval     time.Duration `json:"check_interval"`     // 检查间隔
}

// DefaultTimeoutConfig 默认超时配置
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		ConnectTimeout:    DefaultConnectTimeout,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       5 * time.Minute,
		KeepAliveInterval: DefaultHeartbeatInterval,
		CheckInterval:     1 * time.Minute,
	}
}

// TimeoutEntry 超时条目
type TimeoutEntry struct {
	ID           string                 `json:"id"`            // 条目ID
	DeviceID     string                 `json:"device_id"`     // 设备ID
	Type         TimeoutType            `json:"type"`          // 超时类型
	Deadline     time.Time              `json:"deadline"`      // 截止时间
	Callback     TimeoutCallback        `json:"-"`             // 超时回调
	Context      map[string]interface{} `json:"context"`       // 上下文信息
	IsActive     bool                   `json:"is_active"`     // 是否活跃
	CreatedAt    time.Time              `json:"created_at"`    // 创建时间
	LastActivity time.Time              `json:"last_activity"` // 最后活动时间
}

// TimeoutType 超时类型
type TimeoutType int

const (
	TimeoutTypeConnect   TimeoutType = iota // 连接超时
	TimeoutTypeRead                         // 读取超时
	TimeoutTypeWrite                        // 写入超时
	TimeoutTypeIdle                         // 空闲超时
	TimeoutTypeKeepAlive                    // 保活超时
	TimeoutTypeCustom                       // 自定义超时
)

// String 返回超时类型的字符串表示
func (tt TimeoutType) String() string {
	switch tt {
	case TimeoutTypeConnect:
		return "connect"
	case TimeoutTypeRead:
		return "read"
	case TimeoutTypeWrite:
		return "write"
	case TimeoutTypeIdle:
		return "idle"
	case TimeoutTypeKeepAlive:
		return "keepalive"
	case TimeoutTypeCustom:
		return "custom"
	default:
		return "unknown"
	}
}

// TimeoutCallback 超时回调函数类型
type TimeoutCallback func(entry *TimeoutEntry) error

// NewTimeoutManager 创建新的超时管理器
func NewTimeoutManager(config TimeoutConfig) *TimeoutManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &TimeoutManager{
		timeouts: make(map[string]*TimeoutEntry),
		config:   config,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start 启动超时管理器
func (tm *TimeoutManager) Start() error {
	// 启动超时检查器
	tm.wg.Add(1)
	go tm.timeoutChecker()

	return nil
}

// Stop 停止超时管理器
func (tm *TimeoutManager) Stop() error {
	tm.cancel()
	tm.wg.Wait()

	// 清理所有超时条目
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.timeouts = make(map[string]*TimeoutEntry)

	return nil
}

// AddTimeout 添加超时条目
func (tm *TimeoutManager) AddTimeout(deviceID string, timeoutType TimeoutType, duration time.Duration, callback TimeoutCallback) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	entryID := generateTimeoutID(deviceID, timeoutType)
	deadline := time.Now().Add(duration)

	entry := &TimeoutEntry{
		ID:           entryID,
		DeviceID:     deviceID,
		Type:         timeoutType,
		Deadline:     deadline,
		Callback:     callback,
		Context:      make(map[string]interface{}),
		IsActive:     true,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	tm.timeouts[entryID] = entry
	return entryID, nil
}

// RemoveTimeout 移除超时条目
func (tm *TimeoutManager) RemoveTimeout(entryID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	entry, exists := tm.timeouts[entryID]
	if !exists {
		return NewBluetoothError(ErrCodeNotFound, "超时条目不存在")
	}

	entry.IsActive = false
	delete(tm.timeouts, entryID)
	return nil
}

// UpdateTimeout 更新超时条目
func (tm *TimeoutManager) UpdateTimeout(entryID string, duration time.Duration) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	entry, exists := tm.timeouts[entryID]
	if !exists {
		return NewBluetoothError(ErrCodeNotFound, "超时条目不存在")
	}

	entry.Deadline = time.Now().Add(duration)
	entry.LastActivity = time.Now()
	return nil
}

// RefreshTimeout 刷新超时条目活动时间
func (tm *TimeoutManager) RefreshTimeout(entryID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	entry, exists := tm.timeouts[entryID]
	if !exists {
		return NewBluetoothError(ErrCodeNotFound, "超时条目不存在")
	}

	entry.LastActivity = time.Now()
	return nil
}

// GetTimeout 获取超时条目
func (tm *TimeoutManager) GetTimeout(entryID string) (*TimeoutEntry, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	entry, exists := tm.timeouts[entryID]
	if !exists {
		return nil, NewBluetoothError(ErrCodeNotFound, "超时条目不存在")
	}

	return entry, nil
}

// GetTimeoutsByDevice 获取设备的所有超时条目
func (tm *TimeoutManager) GetTimeoutsByDevice(deviceID string) []*TimeoutEntry {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	entries := make([]*TimeoutEntry, 0)
	for _, entry := range tm.timeouts {
		if entry.DeviceID == deviceID && entry.IsActive {
			entries = append(entries, entry)
		}
	}
	return entries
}

// ClearTimeoutsByDevice 清理设备的所有超时条目
func (tm *TimeoutManager) ClearTimeoutsByDevice(deviceID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	toRemove := make([]string, 0)
	for entryID, entry := range tm.timeouts {
		if entry.DeviceID == deviceID {
			entry.IsActive = false
			toRemove = append(toRemove, entryID)
		}
	}

	for _, entryID := range toRemove {
		delete(tm.timeouts, entryID)
	}

	return nil
}

// AddConnectTimeout 添加连接超时
func (tm *TimeoutManager) AddConnectTimeout(deviceID string, callback TimeoutCallback) (string, error) {
	return tm.AddTimeout(deviceID, TimeoutTypeConnect, tm.config.ConnectTimeout, callback)
}

// AddReadTimeout 添加读取超时
func (tm *TimeoutManager) AddReadTimeout(deviceID string, callback TimeoutCallback) (string, error) {
	return tm.AddTimeout(deviceID, TimeoutTypeRead, tm.config.ReadTimeout, callback)
}

// AddWriteTimeout 添加写入超时
func (tm *TimeoutManager) AddWriteTimeout(deviceID string, callback TimeoutCallback) (string, error) {
	return tm.AddTimeout(deviceID, TimeoutTypeWrite, tm.config.WriteTimeout, callback)
}

// AddIdleTimeout 添加空闲超时
func (tm *TimeoutManager) AddIdleTimeout(deviceID string, callback TimeoutCallback) (string, error) {
	return tm.AddTimeout(deviceID, TimeoutTypeIdle, tm.config.IdleTimeout, callback)
}

// AddKeepAliveTimeout 添加保活超时
func (tm *TimeoutManager) AddKeepAliveTimeout(deviceID string, callback TimeoutCallback) (string, error) {
	return tm.AddTimeout(deviceID, TimeoutTypeKeepAlive, tm.config.KeepAliveInterval, callback)
}

// GetStats 获取超时管理器统计信息
func (tm *TimeoutManager) GetStats() TimeoutStats {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	stats := TimeoutStats{
		TotalTimeouts:   len(tm.timeouts),
		ActiveTimeouts:  0,
		ExpiredTimeouts: 0,
		TypeCounts:      make(map[TimeoutType]int),
	}

	now := time.Now()
	for _, entry := range tm.timeouts {
		if entry.IsActive {
			stats.ActiveTimeouts++
			stats.TypeCounts[entry.Type]++

			if now.After(entry.Deadline) {
				stats.ExpiredTimeouts++
			}
		}
	}

	return stats
}

// TimeoutStats 超时统计信息
type TimeoutStats struct {
	TotalTimeouts   int                 `json:"total_timeouts"`   // 总超时数
	ActiveTimeouts  int                 `json:"active_timeouts"`  // 活跃超时数
	ExpiredTimeouts int                 `json:"expired_timeouts"` // 过期超时数
	TypeCounts      map[TimeoutType]int `json:"type_counts"`      // 类型计数
}

// 内部方法

// timeoutChecker 超时检查器goroutine
func (tm *TimeoutManager) timeoutChecker() {
	defer tm.wg.Done()

	ticker := time.NewTicker(tm.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tm.ctx.Done():
			return
		case <-ticker.C:
			tm.checkTimeouts()
		}
	}
}

// checkTimeouts 检查超时
func (tm *TimeoutManager) checkTimeouts() {
	tm.mu.Lock()
	expiredEntries := make([]*TimeoutEntry, 0)
	now := time.Now()

	for entryID, entry := range tm.timeouts {
		if entry.IsActive && now.After(entry.Deadline) {
			entry.IsActive = false
			expiredEntries = append(expiredEntries, entry)
			delete(tm.timeouts, entryID)
		}
	}
	tm.mu.Unlock()

	// 执行超时回调
	for _, entry := range expiredEntries {
		if entry.Callback != nil {
			go func(e *TimeoutEntry) {
				if err := e.Callback(e); err != nil {
					// 记录回调执行错误
				}
			}(entry)
		}
	}
}

// generateTimeoutID 生成超时条目ID
func generateTimeoutID(deviceID string, timeoutType TimeoutType) string {
	return fmt.Sprintf("timeout_%s_%s_%d", deviceID, timeoutType.String(), time.Now().UnixNano())
}

// ResourceManager 资源管理器，管理连接资源
type ResourceManager struct {
	resources map[string]*ResourceEntry
	config    ResourceConfig
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// ResourceConfig 资源配置
type ResourceConfig struct {
	MaxConnections     int           `json:"max_connections"`      // 最大连接数
	MaxMemoryUsage     int64         `json:"max_memory_usage"`     // 最大内存使用量(字节)
	MaxBufferSize      int           `json:"max_buffer_size"`      // 最大缓冲区大小
	CleanupInterval    time.Duration `json:"cleanup_interval"`     // 清理间隔
	ResourceTimeout    time.Duration `json:"resource_timeout"`     // 资源超时时间
	EnableAutoCleanup  bool          `json:"enable_auto_cleanup"`  // 启用自动清理
	EnableResourcePool bool          `json:"enable_resource_pool"` // 启用资源池
}

// DefaultResourceConfig 默认资源配置
func DefaultResourceConfig() ResourceConfig {
	return ResourceConfig{
		MaxConnections:     DefaultMaxConnections,
		MaxMemoryUsage:     100 * 1024 * 1024, // 100MB
		MaxBufferSize:      DefaultBufferSize,
		CleanupInterval:    5 * time.Minute,
		ResourceTimeout:    10 * time.Minute,
		EnableAutoCleanup:  true,
		EnableResourcePool: true,
	}
}

// ResourceEntry 资源条目
type ResourceEntry struct {
	ID           string                 `json:"id"`            // 资源ID
	DeviceID     string                 `json:"device_id"`     // 设备ID
	Type         ResourceType           `json:"type"`          // 资源类型
	Size         int64                  `json:"size"`          // 资源大小
	CreatedAt    time.Time              `json:"created_at"`    // 创建时间
	LastAccessed time.Time              `json:"last_accessed"` // 最后访问时间
	RefCount     int                    `json:"ref_count"`     // 引用计数
	Metadata     map[string]interface{} `json:"metadata"`      // 资源元数据
	Resource     interface{}            `json:"-"`             // 实际资源对象
}

// ResourceType 资源类型
type ResourceType int

const (
	ResourceTypeConnection ResourceType = iota // 连接资源
	ResourceTypeBuffer                         // 缓冲区资源
	ResourceTypeSession                        // 会话资源
	ResourceTypeMemory                         // 内存资源
	ResourceTypeCustom                         // 自定义资源
)

// String 返回资源类型的字符串表示
func (rt ResourceType) String() string {
	switch rt {
	case ResourceTypeConnection:
		return "connection"
	case ResourceTypeBuffer:
		return "buffer"
	case ResourceTypeSession:
		return "session"
	case ResourceTypeMemory:
		return "memory"
	case ResourceTypeCustom:
		return "custom"
	default:
		return "unknown"
	}
}

// NewResourceManager 创建新的资源管理器
func NewResourceManager(config ResourceConfig) *ResourceManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &ResourceManager{
		resources: make(map[string]*ResourceEntry),
		config:    config,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start 启动资源管理器
func (rm *ResourceManager) Start() error {
	if rm.config.EnableAutoCleanup {
		// 启动资源清理器
		rm.wg.Add(1)
		go rm.resourceCleaner()
	}

	return nil
}

// Stop 停止资源管理器
func (rm *ResourceManager) Stop() error {
	rm.cancel()
	rm.wg.Wait()

	// 清理所有资源
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.resources = make(map[string]*ResourceEntry)

	return nil
}

// AllocateResource 分配资源
func (rm *ResourceManager) AllocateResource(deviceID string, resourceType ResourceType, size int64, resource interface{}) (string, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// 检查资源限制
	if err := rm.checkResourceLimits(resourceType, size); err != nil {
		return "", err
	}

	resourceID := generateResourceID(deviceID, resourceType)
	entry := &ResourceEntry{
		ID:           resourceID,
		DeviceID:     deviceID,
		Type:         resourceType,
		Size:         size,
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		RefCount:     1,
		Metadata:     make(map[string]interface{}),
		Resource:     resource,
	}

	rm.resources[resourceID] = entry
	return resourceID, nil
}

// ReleaseResource 释放资源
func (rm *ResourceManager) ReleaseResource(resourceID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	entry, exists := rm.resources[resourceID]
	if !exists {
		return NewBluetoothError(ErrCodeNotFound, "资源不存在")
	}

	entry.RefCount--
	if entry.RefCount <= 0 {
		delete(rm.resources, resourceID)
	}

	return nil
}

// GetResource 获取资源
func (rm *ResourceManager) GetResource(resourceID string) (*ResourceEntry, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	entry, exists := rm.resources[resourceID]
	if !exists {
		return nil, NewBluetoothError(ErrCodeNotFound, "资源不存在")
	}

	entry.LastAccessed = time.Now()
	return entry, nil
}

// checkResourceLimits 检查资源限制
func (rm *ResourceManager) checkResourceLimits(resourceType ResourceType, size int64) error {
	// 检查连接数限制
	if resourceType == ResourceTypeConnection {
		connectionCount := 0
		for _, entry := range rm.resources {
			if entry.Type == ResourceTypeConnection {
				connectionCount++
			}
		}
		if connectionCount >= rm.config.MaxConnections {
			return NewBluetoothError(ErrCodeResourceBusy, "达到最大连接数限制")
		}
	}

	// 检查内存使用限制
	totalMemory := int64(0)
	for _, entry := range rm.resources {
		totalMemory += entry.Size
	}
	if totalMemory+size > rm.config.MaxMemoryUsage {
		return NewBluetoothError(ErrCodeResourceBusy, "达到最大内存使用限制")
	}

	return nil
}

// resourceCleaner 资源清理器goroutine
func (rm *ResourceManager) resourceCleaner() {
	defer rm.wg.Done()

	ticker := time.NewTicker(rm.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rm.ctx.Done():
			return
		case <-ticker.C:
			rm.cleanupExpiredResources()
		}
	}
}

// cleanupExpiredResources 清理过期资源
func (rm *ResourceManager) cleanupExpiredResources() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	now := time.Now()
	toRemove := make([]string, 0)

	for resourceID, entry := range rm.resources {
		if now.Sub(entry.LastAccessed) > rm.config.ResourceTimeout {
			toRemove = append(toRemove, resourceID)
		}
	}

	for _, resourceID := range toRemove {
		delete(rm.resources, resourceID)
	}
}

// generateResourceID 生成资源ID
func generateResourceID(deviceID string, resourceType ResourceType) string {
	return fmt.Sprintf("resource_%s_%s_%d", deviceID, resourceType.String(), time.Now().UnixNano())
}
