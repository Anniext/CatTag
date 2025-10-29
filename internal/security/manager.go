package security

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// Manager 安全管理器实现
type Manager struct {
	mu              sync.RWMutex                       // 读写锁
	sessions        map[string]*SecuritySession        // 安全会话
	trustedDevices  map[string]bool                    // 信任设备列表
	accessPolicies  map[string]*bluetooth.AccessPolicy // 访问策略
	deviceWhitelist map[string]bool                    // 设备白名单
	config          SecurityConfig                     // 安全配置
	keyStore        KeyStore                           // 密钥存储
	encryptionMgr   *EncryptionManager                 // 加密管理器
	authCallbacks   []AuthCallback                     // 认证回调函数
	securityEvents  chan SecurityEvent                 // 安全事件通道
}

// SecuritySession 安全会话
type SecuritySession struct {
	DeviceID        string                `json:"device_id"`        // 设备ID
	SessionKey      []byte                `json:"session_key"`      // 会话密钥
	CreatedAt       time.Time             `json:"created_at"`       // 创建时间
	LastActivity    time.Time             `json:"last_activity"`    // 最后活动时间
	IsAuthenticated bool                  `json:"is_authenticated"` // 是否已认证
	Permissions     []bluetooth.Operation `json:"permissions"`      // 权限列表
	FailedAttempts  int                   `json:"failed_attempts"`  // 失败尝试次数
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	EnableAuth        bool          `json:"enable_auth"`         // 启用认证
	EnableEncryption  bool          `json:"enable_encryption"`   // 启用加密
	EnableWhitelist   bool          `json:"enable_whitelist"`    // 启用白名单
	KeySize           int           `json:"key_size"`            // 密钥大小
	Algorithm         string        `json:"algorithm"`           // 加密算法
	SessionTimeout    time.Duration `json:"session_timeout"`     // 会话超时时间
	MaxFailedAttempts int           `json:"max_failed_attempts"` // 最大失败尝试次数
	RequireAuth       bool          `json:"require_auth"`        // 强制认证
	LogSecurityEvents bool          `json:"log_security_events"` // 记录安全事件
}

// AuthCallback 认证回调函数类型
type AuthCallback func(deviceID string, event AuthEvent) error

// AuthEvent 认证事件
type AuthEvent struct {
	Type      AuthEventType `json:"type"`      // 事件类型
	DeviceID  string        `json:"device_id"` // 设备ID
	Timestamp time.Time     `json:"timestamp"` // 时间戳
	Success   bool          `json:"success"`   // 是否成功
	Message   string        `json:"message"`   // 事件消息
}

// AuthEventType 认证事件类型
type AuthEventType int

const (
	AuthEventLogin   AuthEventType = iota // 登录事件
	AuthEventLogout                       // 登出事件
	AuthEventFailed                       // 认证失败
	AuthEventBlocked                      // 设备被阻止
)

// SecurityEvent 安全事件
type SecurityEvent struct {
	ID        string            `json:"id"`        // 事件ID
	Type      SecurityEventType `json:"type"`      // 事件类型
	DeviceID  string            `json:"device_id"` // 设备ID
	Timestamp time.Time         `json:"timestamp"` // 时间戳
	Severity  EventSeverity     `json:"severity"`  // 严重程度
	Message   string            `json:"message"`   // 事件消息
	Details   map[string]string `json:"details"`   // 事件详情
}

// SecurityEventType 安全事件类型
type SecurityEventType int

const (
	SecurityEventAuthSuccess        SecurityEventType = iota // 认证成功
	SecurityEventAuthFailed                                  // 认证失败
	SecurityEventUnauthorized                                // 未授权访问
	SecurityEventEncryptionFailed                            // 加密失败
	SecurityEventSuspiciousActivity                          // 可疑活动
	SecurityEventDeviceBlocked                               // 设备被阻止
	SecurityEventError                                       // 错误事件
)

// EventSeverity 事件严重程度
type EventSeverity int

const (
	SeverityInfo     EventSeverity = iota // 信息
	SeverityWarning                       // 警告
	SeverityError                         // 错误
	SeverityCritical                      // 严重
)

// KeyStore 密钥存储接口
type KeyStore interface {
	// StoreKey 存储密钥
	StoreKey(deviceID string, key []byte) error
	// GetKey 获取密钥
	GetKey(deviceID string) ([]byte, error)
	// DeleteKey 删除密钥
	DeleteKey(deviceID string) error
	// ListKeys 列出所有密钥
	ListKeys() ([]string, error)
}

// MemoryKeyStore 内存密钥存储实现
type MemoryKeyStore struct {
	mu   sync.RWMutex
	keys map[string][]byte
}

// NewManager 创建新的安全管理器
func NewManager(config SecurityConfig) *Manager {
	return &Manager{
		sessions:        make(map[string]*SecuritySession),
		trustedDevices:  make(map[string]bool),
		accessPolicies:  make(map[string]*bluetooth.AccessPolicy),
		deviceWhitelist: make(map[string]bool),
		config:          config,
		keyStore:        NewMemoryKeyStore(),
		encryptionMgr:   NewEncryptionManager(config.Algorithm, config.KeySize),
		authCallbacks:   make([]AuthCallback, 0),
		securityEvents:  make(chan SecurityEvent, 100),
	}
}

// NewMemoryKeyStore 创建内存密钥存储
func NewMemoryKeyStore() *MemoryKeyStore {
	return &MemoryKeyStore{
		keys: make(map[string][]byte),
	}
}

// Authenticate 设备认证
func (m *Manager) Authenticate(deviceID string, credentials bluetooth.Credentials) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 首先检查白名单
	if !m.IsWhitelisted(deviceID) {
		m.logSecurityEvent(SecurityEventAuthFailed, deviceID, SeverityError, "设备不在白名单中")
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodePermissionDenied,
			"设备不在白名单中",
			deviceID,
			"authenticate",
		)
	}

	if !m.config.EnableAuth {
		// 即使未启用认证，也要创建会话
		session := m.getOrCreateSession(deviceID)
		session.IsAuthenticated = true
		session.LastActivity = time.Now()
		m.logSecurityEvent(SecurityEventAuthSuccess, deviceID, SeverityInfo, "认证已禁用，自动通过")
		return nil
	}

	// 获取或创建会话
	session := m.getOrCreateSession(deviceID)

	// 检查失败尝试次数
	if session.FailedAttempts >= m.config.MaxFailedAttempts {
		m.logSecurityEvent(SecurityEventAuthFailed, deviceID, SeverityError, "认证失败次数过多")
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeAuthFailed,
			"认证失败次数过多",
			deviceID,
			"authenticate",
		)
	}

	// 根据凭据类型进行认证
	var err error
	switch credentials.Type {
	case bluetooth.CredentialTypePassword:
		err = m.authenticateWithPassword(deviceID, credentials.Password)
	case bluetooth.CredentialTypeToken:
		err = m.authenticateWithToken(deviceID, credentials.Token)
	case bluetooth.CredentialTypeKey:
		err = m.authenticateWithKey(deviceID, credentials.KeyData)
	default:
		err = bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "不支持的认证类型")
	}

	if err != nil {
		session.FailedAttempts++
		m.logSecurityEvent(SecurityEventAuthFailed, deviceID, SeverityError, "认证失败: "+err.Error())

		// 触发认证失败回调
		m.triggerAuthCallback(deviceID, AuthEvent{
			Type:      AuthEventFailed,
			DeviceID:  deviceID,
			Timestamp: time.Now(),
			Success:   false,
			Message:   err.Error(),
		})

		return err
	}

	// 认证成功
	session.IsAuthenticated = true
	session.LastActivity = time.Now()
	session.FailedAttempts = 0

	// 设置默认权限
	if len(session.Permissions) == 0 {
		session.Permissions = []bluetooth.Operation{
			bluetooth.OperationRead,
			bluetooth.OperationWrite,
		}
	}

	m.logSecurityEvent(SecurityEventAuthSuccess, deviceID, SeverityInfo, "认证成功")

	// 触发认证成功回调
	m.triggerAuthCallback(deviceID, AuthEvent{
		Type:      AuthEventLogin,
		DeviceID:  deviceID,
		Timestamp: time.Now(),
		Success:   true,
		Message:   "认证成功",
	})

	return nil
}

// Authorize 设备授权
func (m *Manager) Authorize(deviceID string, operation bluetooth.Operation) error {
	// 首先进行访问验证
	if err := m.ValidateAccess(deviceID, operation); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查会话是否存在且已认证
	session, exists := m.sessions[deviceID]
	if !exists {
		m.logSecurityEvent(SecurityEventUnauthorized, deviceID, SeverityError, "会话不存在")
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeAuthFailed,
			"设备未认证",
			deviceID,
			"authorize",
		)
	}

	if !session.IsAuthenticated && m.config.RequireAuth {
		m.logSecurityEvent(SecurityEventUnauthorized, deviceID, SeverityError, "设备未认证")
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeAuthFailed,
			"设备未认证",
			deviceID,
			"authorize",
		)
	}

	// 检查会话是否过期
	if time.Since(session.LastActivity) > m.config.SessionTimeout {
		m.logSecurityEvent(SecurityEventUnauthorized, deviceID, SeverityWarning, "会话已过期")
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeAuthFailed,
			"会话已过期",
			deviceID,
			"authorize",
		)
	}

	// 检查操作权限
	hasPermission := false
	for _, perm := range session.Permissions {
		if perm == operation {
			hasPermission = true
			break
		}
	}

	if !hasPermission {
		m.logSecurityEvent(SecurityEventUnauthorized, deviceID, SeverityError, "操作权限不足: "+operation.String())
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodePermissionDenied,
			"操作权限不足",
			deviceID,
			"authorize",
		)
	}

	// 更新最后活动时间
	session.LastActivity = time.Now()

	return nil
}

// Encrypt 加密数据
func (m *Manager) Encrypt(data []byte, deviceID string) ([]byte, error) {
	if !m.config.EnableEncryption {
		return data, nil // 如果未启用加密，直接返回原数据
	}

	// 使用加密管理器进行加密
	encData, err := m.encryptionMgr.EncryptData(data, deviceID)
	if err != nil {
		return nil, err
	}

	// 序列化加密数据结构
	serialized, err := json.Marshal(encData)
	if err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "序列化加密数据失败", deviceID, "encrypt")
	}

	return serialized, nil
}

// Decrypt 解密数据
func (m *Manager) Decrypt(data []byte, deviceID string) ([]byte, error) {
	if !m.config.EnableEncryption {
		return data, nil // 如果未启用加密，直接返回原数据
	}

	// 反序列化加密数据结构
	var encData EncryptedData
	if err := json.Unmarshal(data, &encData); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "反序列化加密数据失败", deviceID, "decrypt")
	}

	// 使用加密管理器进行解密
	plaintext, err := m.encryptionMgr.DecryptData(&encData, deviceID)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// GenerateSessionKey 生成会话密钥
func (m *Manager) GenerateSessionKey(deviceID string) ([]byte, error) {
	// 使用加密管理器生成密钥
	encKey, err := m.encryptionMgr.GenerateKey(deviceID, m.config.SessionTimeout)
	if err != nil {
		return nil, err
	}

	// 存储密钥到密钥存储
	if err := m.keyStore.StoreKey(deviceID, encKey.Key); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "存储会话密钥失败", deviceID, "generate_key")
	}

	// 更新会话
	m.mu.Lock()
	session := m.getOrCreateSession(deviceID)
	session.SessionKey = encKey.Key
	m.mu.Unlock()

	return encKey.Key, nil
}

// getOrCreateSession 获取或创建会话
func (m *Manager) getOrCreateSession(deviceID string) *SecuritySession {
	session, exists := m.sessions[deviceID]
	if !exists {
		session = &SecuritySession{
			DeviceID:        deviceID,
			CreatedAt:       time.Now(),
			LastActivity:    time.Now(),
			IsAuthenticated: false,
			Permissions:     make([]bluetooth.Operation, 0),
			FailedAttempts:  0,
		}
		m.sessions[deviceID] = session
	}
	return session
}

// getSessionKey 获取会话密钥
func (m *Manager) getSessionKey(deviceID string) ([]byte, error) {
	m.mu.RLock()
	session, exists := m.sessions[deviceID]
	m.mu.RUnlock()

	if !exists {
		return nil, bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeAuthFailed,
			"会话不存在",
			deviceID,
			"get_session_key",
		)
	}

	if len(session.SessionKey) == 0 {
		// 尝试从密钥存储中获取
		key, err := m.keyStore.GetKey(deviceID)
		if err != nil {
			return nil, bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "获取会话密钥失败", deviceID, "get_session_key")
		}

		m.mu.Lock()
		session.SessionKey = key
		m.mu.Unlock()
	}

	return session.SessionKey, nil
}

// authenticateWithPassword 使用密码认证
func (m *Manager) authenticateWithPassword(deviceID, password string) error {
	// TODO: 实现密码认证逻辑
	// 这里应该与存储的密码哈希进行比较
	if password == "" {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeAuthFailed, "密码不能为空")
	}

	// 简单的示例实现
	expectedHash := m.getStoredPasswordHash(deviceID)
	actualHash := m.hashPassword(password)

	if expectedHash != actualHash {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeAuthFailed, "密码错误")
	}

	return nil
}

// authenticateWithToken 使用令牌认证
func (m *Manager) authenticateWithToken(deviceID, token string) error {
	// TODO: 实现令牌认证逻辑
	if token == "" {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeAuthFailed, "令牌不能为空")
	}

	// 简单的示例实现
	return nil
}

// authenticateWithKey 使用密钥认证
func (m *Manager) authenticateWithKey(deviceID string, keyData []byte) error {
	// TODO: 实现密钥认证逻辑
	if len(keyData) == 0 {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeAuthFailed, "密钥数据不能为空")
	}

	// 简单的示例实现
	return nil
}

// getStoredPasswordHash 获取存储的密码哈希
func (m *Manager) getStoredPasswordHash(deviceID string) string {
	// TODO: 从安全存储中获取密码哈希
	return ""
}

// hashPassword 计算密码哈希
func (m *Manager) hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// MemoryKeyStore 实现

// StoreKey 存储密钥
func (mks *MemoryKeyStore) StoreKey(deviceID string, key []byte) error {
	mks.mu.Lock()
	defer mks.mu.Unlock()

	mks.keys[deviceID] = make([]byte, len(key))
	copy(mks.keys[deviceID], key)

	return nil
}

// GetKey 获取密钥
func (mks *MemoryKeyStore) GetKey(deviceID string) ([]byte, error) {
	mks.mu.RLock()
	defer mks.mu.RUnlock()

	key, exists := mks.keys[deviceID]
	if !exists {
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeNotFound, "密钥不存在")
	}

	// 返回密钥的副本
	result := make([]byte, len(key))
	copy(result, key)

	return result, nil
}

// DeleteKey 删除密钥
func (mks *MemoryKeyStore) DeleteKey(deviceID string) error {
	mks.mu.Lock()
	defer mks.mu.Unlock()

	delete(mks.keys, deviceID)
	return nil
}

// ListKeys 列出所有密钥
func (mks *MemoryKeyStore) ListKeys() ([]string, error) {
	mks.mu.RLock()
	defer mks.mu.RUnlock()

	keys := make([]string, 0, len(mks.keys))
	for deviceID := range mks.keys {
		keys = append(keys, deviceID)
	}

	return keys, nil
}

// 访问控制和设备白名单方法

// AddToWhitelist 添加设备到白名单
func (m *Manager) AddToWhitelist(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deviceWhitelist[deviceID] = true
	m.logSecurityEvent(SecurityEventAuthSuccess, deviceID, SeverityInfo, "设备已添加到白名单")
	return nil
}

// RemoveFromWhitelist 从白名单移除设备
func (m *Manager) RemoveFromWhitelist(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.deviceWhitelist, deviceID)
	m.logSecurityEvent(SecurityEventDeviceBlocked, deviceID, SeverityWarning, "设备已从白名单移除")
	return nil
}

// IsWhitelisted 检查设备是否在白名单中
func (m *Manager) IsWhitelisted(deviceID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.config.EnableWhitelist {
		return true // 如果未启用白名单，所有设备都被允许
	}

	return m.deviceWhitelist[deviceID]
}

// GetWhitelist 获取白名单设备列表
func (m *Manager) GetWhitelist() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]string, 0, len(m.deviceWhitelist))
	for deviceID := range m.deviceWhitelist {
		devices = append(devices, deviceID)
	}

	return devices
}

// SetAccessPolicy 设置设备访问策略
func (m *Manager) SetAccessPolicy(deviceID string, policy *bluetooth.AccessPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.accessPolicies[deviceID] = policy
	m.logSecurityEvent(SecurityEventAuthSuccess, deviceID, SeverityInfo, "访问策略已更新")
	return nil
}

// GetAccessPolicy 获取设备访问策略
func (m *Manager) GetAccessPolicy(deviceID string) (*bluetooth.AccessPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, exists := m.accessPolicies[deviceID]
	if !exists {
		return nil, bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeNotFound,
			"访问策略不存在",
			deviceID,
			"get_access_policy",
		)
	}

	return policy, nil
}

// AddTrustedDevice 添加信任设备
func (m *Manager) AddTrustedDevice(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.trustedDevices[deviceID] = true
	m.logSecurityEvent(SecurityEventAuthSuccess, deviceID, SeverityInfo, "设备已添加到信任列表")
	return nil
}

// RemoveTrustedDevice 移除信任设备
func (m *Manager) RemoveTrustedDevice(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.trustedDevices, deviceID)
	m.logSecurityEvent(SecurityEventDeviceBlocked, deviceID, SeverityWarning, "设备已从信任列表移除")
	return nil
}

// IsTrustedDevice 检查设备是否为信任设备
func (m *Manager) IsTrustedDevice(deviceID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.trustedDevices[deviceID]
}

// GetTrustedDevices 获取信任设备列表
func (m *Manager) GetTrustedDevices() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]string, 0, len(m.trustedDevices))
	for deviceID := range m.trustedDevices {
		devices = append(devices, deviceID)
	}

	return devices
}

// RegisterAuthCallback 注册认证回调
func (m *Manager) RegisterAuthCallback(callback bluetooth.AuthCallback) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 将公共类型的回调转换为内部类型
	internalCallback := func(deviceID string, event AuthEvent) error {
		// 转换事件类型
		bluetoothEvent := bluetooth.AuthEvent{
			Type:      bluetooth.AuthEventType(event.Type),
			DeviceID:  event.DeviceID,
			Timestamp: event.Timestamp,
			Success:   event.Success,
			Message:   event.Message,
		}
		return callback(deviceID, bluetoothEvent)
	}

	m.authCallbacks = append(m.authCallbacks, internalCallback)
	return nil
}

// RevokeSession 撤销会话
func (m *Manager) RevokeSession(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.sessions[deviceID]
	if !exists {
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeNotFound,
			"会话不存在",
			deviceID,
			"revoke_session",
		)
	}

	// 清除会话密钥
	if err := m.keyStore.DeleteKey(deviceID); err != nil {
		// 记录错误但不阻止会话撤销
		m.logSecurityEvent(SecurityEventError, deviceID, SeverityWarning, "删除会话密钥失败: "+err.Error())
	}

	// 删除会话
	delete(m.sessions, deviceID)
	m.logSecurityEvent(SecurityEventAuthSuccess, deviceID, SeverityInfo, "会话已撤销")

	// 触发认证回调
	m.triggerAuthCallback(deviceID, AuthEvent{
		Type:      AuthEventLogout,
		DeviceID:  deviceID,
		Timestamp: time.Now(),
		Success:   true,
		Message:   "会话已撤销",
	})

	return nil
}

// GetActiveSession 获取活跃会话
func (m *Manager) GetActiveSession(deviceID string) (bluetooth.SecuritySession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[deviceID]
	if !exists {
		return bluetooth.SecuritySession{}, bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeNotFound,
			"会话不存在",
			deviceID,
			"get_active_session",
		)
	}

	// 检查会话是否过期
	if time.Since(session.LastActivity) > m.config.SessionTimeout {
		return bluetooth.SecuritySession{}, bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeAuthFailed,
			"会话已过期",
			deviceID,
			"get_active_session",
		)
	}

	// 转换为公共类型
	return bluetooth.SecuritySession{
		DeviceID:        session.DeviceID,
		SessionKey:      session.SessionKey,
		CreatedAt:       session.CreatedAt,
		LastActivity:    session.LastActivity,
		IsAuthenticated: session.IsAuthenticated,
		Permissions:     session.Permissions,
		FailedAttempts:  session.FailedAttempts,
	}, nil
}

// GetSecurityEvents 获取安全事件通道
func (m *Manager) GetSecurityEvents() <-chan bluetooth.SecurityEvent {
	// 创建一个转换通道
	bluetoothEvents := make(chan bluetooth.SecurityEvent, 100)

	go func() {
		defer close(bluetoothEvents)
		for event := range m.securityEvents {
			// 转换为公共类型
			bluetoothEvent := bluetooth.SecurityEvent{
				ID:        event.ID,
				Type:      bluetooth.SecurityEventType(event.Type),
				DeviceID:  event.DeviceID,
				Timestamp: event.Timestamp,
				Severity:  bluetooth.EventSeverity(event.Severity),
				Message:   event.Message,
				Details:   event.Details,
			}

			select {
			case bluetoothEvents <- bluetoothEvent:
			default:
				// 如果通道满了，丢弃事件
			}
		}
	}()

	return bluetoothEvents
}

// CleanupExpiredSessions 清理过期会话
func (m *Manager) CleanupExpiredSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for deviceID, session := range m.sessions {
		if now.Sub(session.LastActivity) > m.config.SessionTimeout {
			// 清除过期会话
			delete(m.sessions, deviceID)
			m.keyStore.DeleteKey(deviceID)
			m.logSecurityEvent(SecurityEventAuthSuccess, deviceID, SeverityInfo, "过期会话已清理")
		}
	}
}

// ValidateAccess 验证设备访问权限
func (m *Manager) ValidateAccess(deviceID string, operation bluetooth.Operation) error {
	// 检查白名单
	if !m.IsWhitelisted(deviceID) {
		m.logSecurityEvent(SecurityEventUnauthorized, deviceID, SeverityError, "设备不在白名单中")
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodePermissionDenied,
			"设备不在白名单中",
			deviceID,
			"validate_access",
		)
	}

	// 检查访问策略
	policy, err := m.GetAccessPolicy(deviceID)
	if err != nil {
		// 如果没有特定策略，检查是否为信任设备
		if !m.IsTrustedDevice(deviceID) && m.config.RequireAuth {
			m.logSecurityEvent(SecurityEventUnauthorized, deviceID, SeverityError, "设备未授权且无访问策略")
			return bluetooth.NewBluetoothErrorWithDevice(
				bluetooth.ErrCodePermissionDenied,
				"设备未授权且无访问策略",
				deviceID,
				"validate_access",
			)
		}
		return nil // 信任设备允许所有操作
	}

	// 检查操作是否被允许
	allowed := false
	for _, allowedOp := range policy.AllowedOperations {
		if allowedOp == operation {
			allowed = true
			break
		}
	}

	if !allowed {
		m.logSecurityEvent(SecurityEventUnauthorized, deviceID, SeverityError, "操作未被授权: "+operation.String())
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodePermissionDenied,
			"操作未被授权: "+operation.String(),
			deviceID,
			"validate_access",
		)
	}

	return nil
}

// logSecurityEvent 记录安全事件
func (m *Manager) logSecurityEvent(eventType SecurityEventType, deviceID string, severity EventSeverity, message string) {
	if !m.config.LogSecurityEvents {
		return
	}

	event := SecurityEvent{
		ID:        generateEventID(),
		Type:      eventType,
		DeviceID:  deviceID,
		Timestamp: time.Now(),
		Severity:  severity,
		Message:   message,
		Details:   make(map[string]string),
	}

	// 非阻塞发送事件
	select {
	case m.securityEvents <- event:
	default:
		// 如果通道满了，丢弃事件以避免阻塞
	}
}

// triggerAuthCallback 触发认证回调
func (m *Manager) triggerAuthCallback(deviceID string, event AuthEvent) {
	for _, callback := range m.authCallbacks {
		go func(cb AuthCallback) {
			if err := cb(deviceID, event); err != nil {
				m.logSecurityEvent(SecurityEventError, deviceID, SeverityWarning, "认证回调执行失败: "+err.Error())
			}
		}(callback)
	}
}

// generateEventID 生成事件ID
func generateEventID() string {
	timestamp := time.Now().Format("20060102150405.000000")
	return fmt.Sprintf("sec_%s", timestamp)
}

// 密钥管理相关方法

// RotateSessionKey 轮换会话密钥
func (m *Manager) RotateSessionKey(deviceID string) ([]byte, error) {
	// 使用加密管理器轮换密钥
	encKey, err := m.encryptionMgr.RotateKey(deviceID, m.config.SessionTimeout)
	if err != nil {
		return nil, err
	}

	// 更新密钥存储
	if err := m.keyStore.StoreKey(deviceID, encKey.Key); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "存储轮换密钥失败", deviceID, "rotate_session_key")
	}

	// 更新会话
	m.mu.Lock()
	if session, exists := m.sessions[deviceID]; exists {
		session.SessionKey = encKey.Key
		session.LastActivity = time.Now()
	}
	m.mu.Unlock()

	m.logSecurityEvent(SecurityEventAuthSuccess, deviceID, SeverityInfo, "会话密钥已轮换")
	return encKey.Key, nil
}

// GetKeyInfo 获取密钥信息
func (m *Manager) GetKeyInfo(deviceID string) (map[string]interface{}, error) {
	return m.encryptionMgr.GetKeyInfo(deviceID)
}

// ExportKey 导出密钥
func (m *Manager) ExportKey(deviceID string, password string) (string, error) {
	return m.encryptionMgr.ExportKey(deviceID, password)
}

// ImportKey 导入密钥
func (m *Manager) ImportKey(deviceID string, encryptedKey string, password string) error {
	return m.encryptionMgr.ImportKey(deviceID, encryptedKey, password)
}

// GenerateKeyExchange 生成密钥交换数据
func (m *Manager) GenerateKeyExchange(deviceID string) (*KeyExchangeData, error) {
	return m.encryptionMgr.GenerateKeyExchangeData(deviceID)
}

// ValidateKeyExchange 验证密钥交换数据
func (m *Manager) ValidateKeyExchange(kxData *KeyExchangeData) error {
	return m.encryptionMgr.ValidateKeyExchange(kxData)
}

// DeriveSharedKey 派生共享密钥
func (m *Manager) DeriveSharedKey(localPrivateKey, remotePublicKey []byte, deviceID string) ([]byte, error) {
	return m.encryptionMgr.DeriveSharedKey(localPrivateKey, remotePublicKey, deviceID)
}

// CleanupExpiredKeys 清理过期密钥
func (m *Manager) CleanupExpiredKeys() (int, error) {
	// 清理加密管理器中的过期密钥
	encCleaned := m.encryptionMgr.CleanupExpiredKeys()

	// 清理密钥存储中的旧密钥（如果支持）
	var storeCleaned int
	if fileStore, ok := m.keyStore.(*FileKeyStore); ok {
		cleaned, err := fileStore.CleanupOldKeys(m.config.SessionTimeout * 2)
		if err != nil {
			return encCleaned, err
		}
		storeCleaned = cleaned
	}

	total := encCleaned + storeCleaned
	if total > 0 {
		m.logSecurityEvent(SecurityEventAuthSuccess, "", SeverityInfo, fmt.Sprintf("已清理 %d 个过期密钥", total))
	}

	return total, nil
}

// GetSecurityStats 获取安全统计信息
func (m *Manager) GetSecurityStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"active_sessions":     len(m.sessions),
		"trusted_devices":     len(m.trustedDevices),
		"whitelisted_devices": len(m.deviceWhitelist),
		"access_policies":     len(m.accessPolicies),
		"config":              m.config,
	}

	// 添加密钥存储统计
	if fileStore, ok := m.keyStore.(*FileKeyStore); ok {
		stats["keystore"] = fileStore.GetStats()
	}

	return stats
}

// EnableSecurityFeature 启用安全功能
func (m *Manager) EnableSecurityFeature(feature string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch feature {
	case "auth":
		m.config.EnableAuth = enabled
	case "encryption":
		m.config.EnableEncryption = enabled
	case "whitelist":
		m.config.EnableWhitelist = enabled
	case "security_events":
		m.config.LogSecurityEvents = enabled
	default:
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "不支持的安全功能: "+feature)
	}

	action := "禁用"
	if enabled {
		action = "启用"
	}
	m.logSecurityEvent(SecurityEventAuthSuccess, "", SeverityInfo, fmt.Sprintf("安全功能 %s 已%s", feature, action))

	return nil
}

// SetSessionTimeout 设置会话超时时间
func (m *Manager) SetSessionTimeout(timeout time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.SessionTimeout = timeout
	m.logSecurityEvent(SecurityEventAuthSuccess, "", SeverityInfo, fmt.Sprintf("会话超时时间已设置为 %v", timeout))
}

// SetMaxFailedAttempts 设置最大失败尝试次数
func (m *Manager) SetMaxFailedAttempts(maxAttempts int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.MaxFailedAttempts = maxAttempts
	m.logSecurityEvent(SecurityEventAuthSuccess, "", SeverityInfo, fmt.Sprintf("最大失败尝试次数已设置为 %d", maxAttempts))
}
