package device

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/Anniext/CatTag/internal/adapter"
	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// PairingService 设备配对服务
type PairingService struct {
	mu             sync.RWMutex
	adapter        adapter.BluetoothAdapter
	manager        *Manager
	activePairings map[string]*PairingSession
	pairingConfig  PairingConfig
	running        bool
	ctx            context.Context
	cancel         context.CancelFunc
}

// PairingConfig 配对服务配置
type PairingConfig struct {
	DefaultTimeout      time.Duration `json:"default_timeout"`      // 默认配对超时时间
	MaxConcurrentPairs  int           `json:"max_concurrent_pairs"` // 最大并发配对数
	RequireConfirmation bool          `json:"require_confirmation"` // 需要确认
	AutoAccept          bool          `json:"auto_accept"`          // 自动接受配对
	PINLength           int           `json:"pin_length"`           // PIN码长度
	EnableEncryption    bool          `json:"enable_encryption"`    // 启用加密
}

// DefaultPairingConfig 返回默认配对服务配置
func DefaultPairingConfig() PairingConfig {
	return PairingConfig{
		DefaultTimeout:      30 * time.Second,
		MaxConcurrentPairs:  3,
		RequireConfirmation: true,
		AutoAccept:          false,
		PINLength:           6,
		EnableEncryption:    true,
	}
}

// PairingSession 配对会话
type PairingSession struct {
	ID           string             `json:"id"`            // 会话ID
	DeviceID     string             `json:"device_id"`     // 设备ID
	DeviceName   string             `json:"device_name"`   // 设备名称
	Method       PairingMethod      `json:"method"`        // 配对方法
	Status       PairingStatus      `json:"status"`        // 配对状态
	PIN          string             `json:"pin,omitempty"` // PIN码
	Passkey      uint32             `json:"passkey"`       // 数字密钥
	StartTime    time.Time          `json:"start_time"`    // 开始时间
	Timeout      time.Duration      `json:"timeout"`       // 超时时间
	ErrorMessage string             `json:"error_message"` // 错误信息
	ctx          context.Context    `json:"-"`             // 上下文
	cancel       context.CancelFunc `json:"-"`             // 取消函数
	confirmChan  chan bool          `json:"-"`             // 确认通道
}

// PairingMethod 配对方法
type PairingMethod int

const (
	PairingMethodPIN          PairingMethod = iota // PIN码配对
	PairingMethodPasskey                           // 数字密钥配对
	PairingMethodConfirmation                      // 确认配对
	PairingMethodOOB                               // 带外配对
)

// String 返回配对方法的字符串表示
func (pm PairingMethod) String() string {
	switch pm {
	case PairingMethodPIN:
		return "pin"
	case PairingMethodPasskey:
		return "passkey"
	case PairingMethodConfirmation:
		return "confirmation"
	case PairingMethodOOB:
		return "oob"
	default:
		return "unknown"
	}
}

// PairingStatus 配对状态
type PairingStatus int

const (
	PairingStatusPending     PairingStatus = iota // 等待中
	PairingStatusInProgress                       // 进行中
	PairingStatusWaitingAuth                      // 等待认证
	PairingStatusCompleted                        // 已完成
	PairingStatusFailed                           // 失败
	PairingStatusCancelled                        // 已取消
	PairingStatusTimeout                          // 超时
)

// String 返回配对状态的字符串表示
func (ps PairingStatus) String() string {
	switch ps {
	case PairingStatusPending:
		return "pending"
	case PairingStatusInProgress:
		return "in_progress"
	case PairingStatusWaitingAuth:
		return "waiting_auth"
	case PairingStatusCompleted:
		return "completed"
	case PairingStatusFailed:
		return "failed"
	case PairingStatusCancelled:
		return "cancelled"
	case PairingStatusTimeout:
		return "timeout"
	default:
		return "unknown"
	}
}

// PairingEvent 配对事件
type PairingEvent struct {
	Type      PairingEventType `json:"type"`       // 事件类型
	SessionID string           `json:"session_id"` // 会话ID
	DeviceID  string           `json:"device_id"`  // 设备ID
	Data      interface{}      `json:"data"`       // 事件数据
	Timestamp time.Time        `json:"timestamp"`  // 时间戳
}

// PairingEventType 配对事件类型
type PairingEventType int

const (
	PairingEventStarted      PairingEventType = iota // 配对开始
	PairingEventPINRequired                          // 需要PIN码
	PairingEventPasskeyShow                          // 显示数字密钥
	PairingEventConfirmation                         // 需要确认
	PairingEventCompleted                            // 配对完成
	PairingEventFailed                               // 配对失败
	PairingEventCancelled                            // 配对取消
)

// String 返回配对事件类型的字符串表示
func (pet PairingEventType) String() string {
	switch pet {
	case PairingEventStarted:
		return "started"
	case PairingEventPINRequired:
		return "pin_required"
	case PairingEventPasskeyShow:
		return "passkey_show"
	case PairingEventConfirmation:
		return "confirmation"
	case PairingEventCompleted:
		return "completed"
	case PairingEventFailed:
		return "failed"
	case PairingEventCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// NewPairingService 创建新的配对服务
func NewPairingService(adapter adapter.BluetoothAdapter, manager *Manager) *PairingService {
	return NewPairingServiceWithConfig(adapter, manager, DefaultPairingConfig())
}

// NewPairingServiceWithConfig 使用配置创建配对服务
func NewPairingServiceWithConfig(adapter adapter.BluetoothAdapter, manager *Manager, config PairingConfig) *PairingService {
	ctx, cancel := context.WithCancel(context.Background())

	return &PairingService{
		adapter:        adapter,
		manager:        manager,
		activePairings: make(map[string]*PairingSession),
		pairingConfig:  config,
		running:        false,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start 启动配对服务
func (ps *PairingService) Start() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.running {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeAlreadyExists, "配对服务已在运行")
	}

	if !ps.adapter.IsEnabled() {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "蓝牙适配器未启用")
	}

	ps.running = true
	return nil
}

// Stop 停止配对服务
func (ps *PairingService) Stop() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if !ps.running {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeNotFound, "配对服务未运行")
	}

	// 取消所有活跃的配对会话
	for _, session := range ps.activePairings {
		session.cancel()
	}

	ps.running = false
	ps.cancel()

	return nil
}

// StartPairing 开始配对设备
func (ps *PairingService) StartPairing(ctx context.Context, deviceID string, method PairingMethod) (*PairingSession, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if !ps.running {
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "配对服务未运行")
	}

	// 检查设备是否存在
	device, err := ps.manager.GetDevice(deviceID)
	if err != nil {
		return nil, err
	}

	// 检查设备是否已配对
	if ps.manager.IsDevicePaired(deviceID) {
		return nil, bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeAlreadyExists, "设备已配对", deviceID, "start_pairing")
	}

	// 检查并发配对限制
	if len(ps.activePairings) >= ps.pairingConfig.MaxConcurrentPairs {
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeResourceBusy, "已达到最大并发配对数限制")
	}

	// 创建配对会话
	sessionID := generatePairingSessionID()
	sessionCtx, sessionCancel := context.WithTimeout(ctx, ps.pairingConfig.DefaultTimeout)

	session := &PairingSession{
		ID:          sessionID,
		DeviceID:    deviceID,
		DeviceName:  device.Name,
		Method:      method,
		Status:      PairingStatusPending,
		StartTime:   time.Now(),
		Timeout:     ps.pairingConfig.DefaultTimeout,
		ctx:         sessionCtx,
		cancel:      sessionCancel,
		confirmChan: make(chan bool, 1),
	}

	ps.activePairings[sessionID] = session

	// 启动配对goroutine
	go ps.performPairing(session)

	return session, nil
}

// CancelPairing 取消配对
func (ps *PairingService) CancelPairing(sessionID string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	session, exists := ps.activePairings[sessionID]
	if !exists {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeNotFound, "配对会话不存在")
	}

	session.cancel()
	session.Status = PairingStatusCancelled
	delete(ps.activePairings, sessionID)

	return nil
}

// ConfirmPairing 确认配对
func (ps *PairingService) ConfirmPairing(sessionID string, confirm bool) error {
	ps.mu.RLock()
	session, exists := ps.activePairings[sessionID]
	ps.mu.RUnlock()

	if !exists {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeNotFound, "配对会话不存在")
	}

	if session.Status != PairingStatusWaitingAuth {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeInvalidParameter, "配对会话不在等待确认状态")
	}

	select {
	case session.confirmChan <- confirm:
		return nil
	case <-time.After(1 * time.Second):
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeTimeout, "确认超时")
	}
}

// ProvidePIN 提供PIN码
func (ps *PairingService) ProvidePIN(sessionID string, pin string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	session, exists := ps.activePairings[sessionID]
	if !exists {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeNotFound, "配对会话不存在")
	}

	if session.Method != PairingMethodPIN {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeInvalidParameter, "配对方法不是PIN码配对")
	}

	session.PIN = pin
	return nil
}

// GetActivePairingSessions 获取活跃的配对会话
func (ps *PairingService) GetActivePairingSessions() []*PairingSession {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	sessions := make([]*PairingSession, 0, len(ps.activePairings))
	for _, session := range ps.activePairings {
		sessions = append(sessions, session)
	}

	return sessions
}

// performPairing 执行配对操作
func (ps *PairingService) performPairing(session *PairingSession) {
	defer func() {
		ps.mu.Lock()
		delete(ps.activePairings, session.ID)
		ps.mu.Unlock()
	}()

	session.Status = PairingStatusInProgress

	// 根据配对方法执行不同的配对流程
	switch session.Method {
	case PairingMethodPIN:
		ps.performPINPairing(session)
	case PairingMethodPasskey:
		ps.performPasskeyPairing(session)
	case PairingMethodConfirmation:
		ps.performConfirmationPairing(session)
	case PairingMethodOOB:
		ps.performOOBPairing(session)
	default:
		session.Status = PairingStatusFailed
		session.ErrorMessage = "不支持的配对方法"
	}
}

// performPINPairing 执行PIN码配对
func (ps *PairingService) performPINPairing(session *PairingSession) {
	// 生成PIN码
	if session.PIN == "" {
		session.PIN = ps.generatePIN()
	}

	session.Status = PairingStatusWaitingAuth

	// 模拟PIN码验证过程
	select {
	case <-time.After(2 * time.Second): // 模拟验证延迟
		// 在实际实现中，这里应该与系统蓝牙API交互
		if ps.validatePIN(session.PIN) {
			ps.completePairing(session)
		} else {
			session.Status = PairingStatusFailed
			session.ErrorMessage = "PIN码验证失败"
		}
	case <-session.ctx.Done():
		session.Status = PairingStatusTimeout
	}
}

// performPasskeyPairing 执行数字密钥配对
func (ps *PairingService) performPasskeyPairing(session *PairingSession) {
	// 生成6位数字密钥
	session.Passkey = ps.generatePasskey()
	session.Status = PairingStatusWaitingAuth

	// 等待用户确认
	select {
	case confirm := <-session.confirmChan:
		if confirm {
			ps.completePairing(session)
		} else {
			session.Status = PairingStatusCancelled
		}
	case <-session.ctx.Done():
		session.Status = PairingStatusTimeout
	}
}

// performConfirmationPairing 执行确认配对
func (ps *PairingService) performConfirmationPairing(session *PairingSession) {
	session.Status = PairingStatusWaitingAuth

	// 如果启用自动接受，直接完成配对
	if ps.pairingConfig.AutoAccept {
		ps.completePairing(session)
		return
	}

	// 等待用户确认
	select {
	case confirm := <-session.confirmChan:
		if confirm {
			ps.completePairing(session)
		} else {
			session.Status = PairingStatusCancelled
		}
	case <-session.ctx.Done():
		session.Status = PairingStatusTimeout
	}
}

// performOOBPairing 执行带外配对
func (ps *PairingService) performOOBPairing(session *PairingSession) {
	// 带外配对的实现
	// 在实际实现中，这里需要处理NFC或其他带外通信
	session.Status = PairingStatusFailed
	session.ErrorMessage = "带外配对暂未实现"
}

// completePairing 完成配对
func (ps *PairingService) completePairing(session *PairingSession) {
	// 将设备标记为已配对
	err := ps.manager.PairDevice(session.DeviceID)
	if err != nil {
		session.Status = PairingStatusFailed
		session.ErrorMessage = err.Error()
		return
	}

	session.Status = PairingStatusCompleted
}

// generatePIN 生成PIN码
func (ps *PairingService) generatePIN() string {
	length := ps.pairingConfig.PINLength
	if length <= 0 {
		length = 6
	}

	pin := ""
	for i := 0; i < length; i++ {
		pin += fmt.Sprintf("%d", ps.randomInt(10))
	}

	return pin
}

// generatePasskey 生成数字密钥
func (ps *PairingService) generatePasskey() uint32 {
	return uint32(ps.randomInt(1000000)) // 6位数字
}

// validatePIN 验证PIN码
func (ps *PairingService) validatePIN(pin string) bool {
	// 在实际实现中，这里应该与设备进行PIN码验证
	// 这里简单模拟验证成功
	return len(pin) >= 4
}

// randomInt 生成随机整数
func (ps *PairingService) randomInt(max int) int {
	b := make([]byte, 1)
	rand.Read(b)
	return int(b[0]) % max
}

// generatePairingSessionID 生成配对会话ID
func generatePairingSessionID() string {
	return time.Now().Format("20060102150405") + "_pair"
}

// IsActive 检查配对会话是否活跃
func (ps *PairingSession) IsActive() bool {
	return ps.Status == PairingStatusPending ||
		ps.Status == PairingStatusInProgress ||
		ps.Status == PairingStatusWaitingAuth
}

// Cancel 取消配对会话
func (ps *PairingSession) Cancel() {
	if ps.cancel != nil {
		ps.cancel()
	}
}

// GetStats 获取配对会话统计信息
func (ps *PairingSession) GetStats() map[string]interface{} {
	duration := time.Since(ps.StartTime)

	return map[string]interface{}{
		"session_id":    ps.ID,
		"device_id":     ps.DeviceID,
		"device_name":   ps.DeviceName,
		"method":        ps.Method.String(),
		"status":        ps.Status.String(),
		"duration":      duration,
		"start_time":    ps.StartTime,
		"timeout":       ps.Timeout,
		"error_message": ps.ErrorMessage,
	}
}
