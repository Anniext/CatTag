package device

import (
	"context"
	"sync"
	"time"

	"github.com/Anniext/CatTag/internal/adapter"
	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// DiscoveryService 设备发现服务
type DiscoveryService struct {
	mu              sync.RWMutex
	adapter         adapter.BluetoothAdapter
	manager         *Manager
	activeScans     map[string]*ScanSession
	discoveryConfig DiscoveryConfig
	running         bool
	ctx             context.Context
	cancel          context.CancelFunc
}

// DiscoveryConfig 发现服务配置
type DiscoveryConfig struct {
	DefaultTimeout     time.Duration `json:"default_timeout"`      // 默认扫描超时时间
	MaxConcurrentScans int           `json:"max_concurrent_scans"` // 最大并发扫描数
	ScanInterval       time.Duration `json:"scan_interval"`        // 扫描间隔
	EnableCaching      bool          `json:"enable_caching"`       // 启用缓存
	CacheTimeout       time.Duration `json:"cache_timeout"`        // 缓存超时时间
}

// DefaultDiscoveryConfig 返回默认发现服务配置
func DefaultDiscoveryConfig() DiscoveryConfig {
	return DiscoveryConfig{
		DefaultTimeout:     bluetooth.DefaultScanTimeout,
		MaxConcurrentScans: 3,
		ScanInterval:       100 * time.Millisecond,
		EnableCaching:      true,
		CacheTimeout:       5 * time.Minute,
	}
}

// ScanSession 扫描会话
type ScanSession struct {
	ID          string                 `json:"id"`           // 会话ID
	Filter      bluetooth.DeviceFilter `json:"filter"`       // 过滤条件
	DeviceChan  chan bluetooth.Device  `json:"-"`            // 设备通道
	StartTime   time.Time              `json:"start_time"`   // 开始时间
	Timeout     time.Duration          `json:"timeout"`      // 超时时间
	DeviceCount int                    `json:"device_count"` // 发现设备数量
	Status      ScanStatus             `json:"status"`       // 扫描状态
	ctx         context.Context        `json:"-"`            // 上下文
	cancel      context.CancelFunc     `json:"-"`            // 取消函数
}

// ScanStatus 扫描状态
type ScanStatus int

const (
	ScanStatusPending   ScanStatus = iota // 等待中
	ScanStatusActive                      // 活跃中
	ScanStatusCompleted                   // 已完成
	ScanStatusCancelled                   // 已取消
	ScanStatusError                       // 错误状态
)

// String 返回扫描状态的字符串表示
func (ss ScanStatus) String() string {
	switch ss {
	case ScanStatusPending:
		return "pending"
	case ScanStatusActive:
		return "active"
	case ScanStatusCompleted:
		return "completed"
	case ScanStatusCancelled:
		return "cancelled"
	case ScanStatusError:
		return "error"
	default:
		return "unknown"
	}
}

// NewDiscoveryService 创建新的设备发现服务
func NewDiscoveryService(adapter adapter.BluetoothAdapter, manager *Manager) *DiscoveryService {
	return NewDiscoveryServiceWithConfig(adapter, manager, DefaultDiscoveryConfig())
}

// NewDiscoveryServiceWithConfig 使用配置创建设备发现服务
func NewDiscoveryServiceWithConfig(adapter adapter.BluetoothAdapter, manager *Manager, config DiscoveryConfig) *DiscoveryService {
	ctx, cancel := context.WithCancel(context.Background())

	return &DiscoveryService{
		adapter:         adapter,
		manager:         manager,
		activeScans:     make(map[string]*ScanSession),
		discoveryConfig: config,
		running:         false,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start 启动发现服务
func (ds *DiscoveryService) Start() error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if ds.running {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeAlreadyExists, "发现服务已在运行")
	}

	if !ds.adapter.IsEnabled() {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "蓝牙适配器未启用")
	}

	ds.running = true
	return nil
}

// Stop 停止发现服务
func (ds *DiscoveryService) Stop() error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if !ds.running {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeNotFound, "发现服务未运行")
	}

	// 取消所有活跃的扫描会话
	for _, session := range ds.activeScans {
		session.cancel()
	}

	ds.running = false
	ds.cancel()

	return nil
}

// StartScan 开始扫描设备
func (ds *DiscoveryService) StartScan(ctx context.Context, filter bluetooth.DeviceFilter, timeout time.Duration) (*ScanSession, error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if !ds.running {
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "发现服务未运行")
	}

	// 检查并发扫描限制
	if len(ds.activeScans) >= ds.discoveryConfig.MaxConcurrentScans {
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeResourceBusy, "已达到最大并发扫描数限制")
	}

	// 创建扫描会话
	sessionID := generateSessionID()
	sessionCtx, sessionCancel := context.WithTimeout(ctx, timeout)

	session := &ScanSession{
		ID:          sessionID,
		Filter:      filter,
		DeviceChan:  make(chan bluetooth.Device, 100),
		StartTime:   time.Now(),
		Timeout:     timeout,
		DeviceCount: 0,
		Status:      ScanStatusPending,
		ctx:         sessionCtx,
		cancel:      sessionCancel,
	}

	ds.activeScans[sessionID] = session

	// 启动扫描goroutine
	go ds.performScan(session)

	return session, nil
}

// StopScan 停止指定的扫描会话
func (ds *DiscoveryService) StopScan(sessionID string) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	session, exists := ds.activeScans[sessionID]
	if !exists {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeNotFound, "扫描会话不存在")
	}

	session.cancel()
	session.Status = ScanStatusCancelled
	delete(ds.activeScans, sessionID)

	return nil
}

// GetActiveScanSessions 获取活跃的扫描会话
func (ds *DiscoveryService) GetActiveScanSessions() []*ScanSession {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	sessions := make([]*ScanSession, 0, len(ds.activeScans))
	for _, session := range ds.activeScans {
		sessions = append(sessions, session)
	}

	return sessions
}

// performScan 执行扫描操作
func (ds *DiscoveryService) performScan(session *ScanSession) {
	defer func() {
		close(session.DeviceChan)

		ds.mu.Lock()
		delete(ds.activeScans, session.ID)
		ds.mu.Unlock()
	}()

	session.Status = ScanStatusActive

	// 如果启用缓存，首先返回缓存中的设备
	if ds.discoveryConfig.EnableCaching {
		cachedDevices := ds.manager.DiscoverDevices(session.ctx, session.Filter)
		for device := range cachedDevices {
			select {
			case session.DeviceChan <- device:
				session.DeviceCount++
			case <-session.ctx.Done():
				session.Status = ScanStatusCancelled
				return
			}
		}
	}

	// 执行实际的蓝牙扫描
	scanChan, err := ds.adapter.Scan(session.ctx, session.Timeout)
	if err != nil {
		session.Status = ScanStatusError
		return
	}

	// 处理扫描结果
	for {
		select {
		case device, ok := <-scanChan:
			if !ok {
				session.Status = ScanStatusCompleted
				return
			}

			// 将设备添加到管理器缓存
			ds.manager.AddDevice(device)

			// 检查设备是否符合过滤条件
			if ds.manager.matchesFilter(device, session.Filter) {
				select {
				case session.DeviceChan <- device:
					session.DeviceCount++
				case <-session.ctx.Done():
					session.Status = ScanStatusCancelled
					return
				}
			}

		case <-session.ctx.Done():
			session.Status = ScanStatusCancelled
			return
		}
	}
}

// generateSessionID 生成会话ID
func generateSessionID() string {
	return time.Now().Format("20060102150405") + "_scan"
}

// GetDevices 获取扫描会话发现的设备
func (ss *ScanSession) GetDevices() <-chan bluetooth.Device {
	return ss.DeviceChan
}

// IsActive 检查扫描会话是否活跃
func (ss *ScanSession) IsActive() bool {
	return ss.Status == ScanStatusActive || ss.Status == ScanStatusPending
}

// Cancel 取消扫描会话
func (ss *ScanSession) Cancel() {
	if ss.cancel != nil {
		ss.cancel()
	}
}

// GetStats 获取扫描会话统计信息
func (ss *ScanSession) GetStats() map[string]interface{} {
	duration := time.Since(ss.StartTime)

	return map[string]interface{}{
		"session_id":   ss.ID,
		"status":       ss.Status.String(),
		"device_count": ss.DeviceCount,
		"duration":     duration,
		"start_time":   ss.StartTime,
		"timeout":      ss.Timeout,
	}
}
