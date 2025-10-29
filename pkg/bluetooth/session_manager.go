package bluetooth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SessionManager 会话管理器，管理客户端会话
type SessionManager struct {
	sessions        map[string]*ClientSession
	securityManager SecurityManager
	config          SessionConfig
	mu              sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

// SessionConfig 会话配置
type SessionConfig struct {
	SessionTimeout    time.Duration `json:"session_timeout"`    // 会话超时时间
	MaxSessions       int           `json:"max_sessions"`       // 最大会话数
	RequireAuth       bool          `json:"require_auth"`       // 是否需要认证
	EnableEncryption  bool          `json:"enable_encryption"`  // 是否启用加密
	HeartbeatInterval time.Duration `json:"heartbeat_interval"` // 心跳间隔
	MaxIdleTime       time.Duration `json:"max_idle_time"`      // 最大空闲时间
	CleanupInterval   time.Duration `json:"cleanup_interval"`   // 清理间隔
}

// DefaultSessionConfig 默认会话配置
func DefaultSessionConfig() SessionConfig {
	return SessionConfig{
		SessionTimeout:    DefaultSessionTimeout,
		MaxSessions:       DefaultMaxConnections,
		RequireAuth:       true,
		EnableEncryption:  true,
		HeartbeatInterval: DefaultHeartbeatInterval,
		MaxIdleTime:       10 * time.Minute,
		CleanupInterval:   1 * time.Minute,
	}
}

// ClientSession 客户端会话
type ClientSession struct {
	ID              string                 `json:"id"`               // 会话ID
	DeviceID        string                 `json:"device_id"`        // 设备ID
	Connection      Connection             `json:"-"`                // 连接对象
	Status          SessionStatus          `json:"status"`           // 会话状态
	CreatedAt       time.Time              `json:"created_at"`       // 创建时间
	LastActivity    time.Time              `json:"last_activity"`    // 最后活动时间
	LastHeartbeat   time.Time              `json:"last_heartbeat"`   // 最后心跳时间
	IsAuthenticated bool                   `json:"is_authenticated"` // 是否已认证
	IsEncrypted     bool                   `json:"is_encrypted"`     // 是否已加密
	Metadata        map[string]interface{} `json:"metadata"`         // 会话元数据
	mu              sync.RWMutex           `json:"-"`                // 会话锁
}

// SessionStatus 会话状态
type SessionStatus int

const (
	SessionStatusCreated        SessionStatus = iota // 已创建
	SessionStatusAuthenticating                      // 认证中
	SessionStatusAuthenticated                       // 已认证
	SessionStatusActive                              // 活跃
	SessionStatusIdle                                // 空闲
	SessionStatusExpired                             // 已过期
	SessionStatusClosed                              // 已关闭
)

// String 返回会话状态的字符串表示
func (ss SessionStatus) String() string {
	switch ss {
	case SessionStatusCreated:
		return "created"
	case SessionStatusAuthenticating:
		return "authenticating"
	case SessionStatusAuthenticated:
		return "authenticated"
	case SessionStatusActive:
		return "active"
	case SessionStatusIdle:
		return "idle"
	case SessionStatusExpired:
		return "expired"
	case SessionStatusClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// NewSessionManager 创建新的会话管理器
func NewSessionManager(config SessionConfig, securityManager SecurityManager) *SessionManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &SessionManager{
		sessions:        make(map[string]*ClientSession),
		securityManager: securityManager,
		config:          config,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start 启动会话管理器
func (sm *SessionManager) Start() error {
	// 启动会话清理器
	sm.wg.Add(1)
	go sm.sessionCleaner()

	// 启动心跳检查器
	sm.wg.Add(1)
	go sm.heartbeatChecker()

	return nil
}

// Stop 停止会话管理器
func (sm *SessionManager) Stop() error {
	sm.cancel()
	sm.wg.Wait()

	// 关闭所有会话
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, session := range sm.sessions {
		sm.closeSessionInternal(session)
	}
	sm.sessions = make(map[string]*ClientSession)

	return nil
}

// CreateSession 创建新会话
func (sm *SessionManager) CreateSession(conn Connection) (*ClientSession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	deviceID := conn.DeviceID()

	// 检查会话数限制
	if len(sm.sessions) >= sm.config.MaxSessions {
		return nil, NewBluetoothErrorWithDevice(ErrCodeResourceBusy, "达到最大会话数限制", deviceID, "create_session")
	}

	// 检查是否已存在会话
	if existingSession, exists := sm.sessions[deviceID]; exists {
		if existingSession.Status != SessionStatusClosed && existingSession.Status != SessionStatusExpired {
			return nil, NewBluetoothErrorWithDevice(ErrCodeAlreadyExists, "设备会话已存在", deviceID, "create_session")
		}
		// 清理旧会话
		sm.closeSessionInternal(existingSession)
	}

	// 创建新会话
	session := &ClientSession{
		ID:           generateSessionID(),
		DeviceID:     deviceID,
		Connection:   conn,
		Status:       SessionStatusCreated,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Metadata:     make(map[string]interface{}),
	}

	sm.sessions[deviceID] = session

	// 启动会话处理
	sm.wg.Add(1)
	go sm.handleSession(session)

	return session, nil
}

// GetSession 获取会话
func (sm *SessionManager) GetSession(deviceID string) (*ClientSession, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[deviceID]
	if !exists {
		return nil, NewBluetoothErrorWithDevice(ErrCodeNotFound, "会话不存在", deviceID, "get_session")
	}

	return session, nil
}

// CloseSession 关闭会话
func (sm *SessionManager) CloseSession(deviceID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[deviceID]
	if !exists {
		return NewBluetoothErrorWithDevice(ErrCodeNotFound, "会话不存在", deviceID, "close_session")
	}

	sm.closeSessionInternal(session)
	delete(sm.sessions, deviceID)

	return nil
}

// GetAllSessions 获取所有会话
func (sm *SessionManager) GetAllSessions() []*ClientSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*ClientSession, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// GetSessionCount 获取会话数量
func (sm *SessionManager) GetSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// AuthenticateSession 认证会话
func (sm *SessionManager) AuthenticateSession(deviceID string, credentials Credentials) error {
	session, err := sm.GetSession(deviceID)
	if err != nil {
		return err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Status == SessionStatusClosed || session.Status == SessionStatusExpired {
		return NewBluetoothErrorWithDevice(ErrCodeNotSupported, "会话已关闭或过期", deviceID, "authenticate")
	}

	session.Status = SessionStatusAuthenticating

	// 执行认证
	if sm.securityManager != nil {
		if err := sm.securityManager.Authenticate(deviceID, credentials); err != nil {
			session.Status = SessionStatusCreated
			return WrapError(err, ErrCodeAuthFailed, "会话认证失败", deviceID, "authenticate")
		}
	}

	session.IsAuthenticated = true
	session.Status = SessionStatusAuthenticated
	session.LastActivity = time.Now()

	return nil
}

// EnableEncryption 启用会话加密
func (sm *SessionManager) EnableEncryption(deviceID string) error {
	session, err := sm.GetSession(deviceID)
	if err != nil {
		return err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if !session.IsAuthenticated {
		return NewBluetoothErrorWithDevice(ErrCodeAuthFailed, "会话未认证，无法启用加密", deviceID, "enable_encryption")
	}

	if sm.securityManager != nil {
		// 生成会话密钥
		if _, err := sm.securityManager.GenerateSessionKey(deviceID); err != nil {
			return WrapError(err, ErrCodeEncryptionFailed, "生成会话密钥失败", deviceID, "enable_encryption")
		}
	}

	session.IsEncrypted = true
	session.LastActivity = time.Now()

	return nil
}

// UpdateActivity 更新会话活动时间
func (sm *SessionManager) UpdateActivity(deviceID string) error {
	session, err := sm.GetSession(deviceID)
	if err != nil {
		return err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	session.LastActivity = time.Now()
	if session.Status == SessionStatusIdle {
		session.Status = SessionStatusActive
	}

	return nil
}

// UpdateHeartbeat 更新会话心跳时间
func (sm *SessionManager) UpdateHeartbeat(deviceID string) error {
	session, err := sm.GetSession(deviceID)
	if err != nil {
		return err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	session.LastHeartbeat = time.Now()
	session.LastActivity = time.Now()

	return nil
}

// 内部方法

// handleSession 处理会话
func (sm *SessionManager) handleSession(session *ClientSession) {
	defer sm.wg.Done()

	deviceID := session.DeviceID
	receiveChan := session.Connection.Receive()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case data, ok := <-receiveChan:
			if !ok {
				// 连接已关闭
				sm.handleSessionClosed(session)
				return
			}

			// 更新活动时间
			sm.UpdateActivity(deviceID)

			// 处理接收到的数据
			sm.handleSessionData(session, data)
		}
	}
}

// handleSessionClosed 处理会话关闭
func (sm *SessionManager) handleSessionClosed(session *ClientSession) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session.mu.Lock()
	session.Status = SessionStatusClosed
	session.mu.Unlock()

	delete(sm.sessions, session.DeviceID)
}

// handleSessionData 处理会话数据
func (sm *SessionManager) handleSessionData(session *ClientSession, data []byte) {
	// 这里可以添加数据处理逻辑，比如解密、协议解析等
	_ = data // 避免未使用变量警告
}

// closeSessionInternal 内部关闭会话方法（需要持有锁）
func (sm *SessionManager) closeSessionInternal(session *ClientSession) {
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Status != SessionStatusClosed {
		session.Status = SessionStatusClosed
		if session.Connection != nil {
			session.Connection.Close()
		}
	}
}

// sessionCleaner 会话清理器goroutine
func (sm *SessionManager) sessionCleaner() {
	defer sm.wg.Done()

	ticker := time.NewTicker(sm.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			sm.cleanupExpiredSessions()
		}
	}
}

// cleanupExpiredSessions 清理过期会话
func (sm *SessionManager) cleanupExpiredSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	expiredSessions := make([]*ClientSession, 0)

	for deviceID, session := range sm.sessions {
		session.mu.RLock()
		isExpired := false

		// 检查会话超时
		if now.Sub(session.CreatedAt) > sm.config.SessionTimeout {
			isExpired = true
		}

		// 检查空闲超时
		if now.Sub(session.LastActivity) > sm.config.MaxIdleTime {
			session.Status = SessionStatusIdle
			if now.Sub(session.LastActivity) > sm.config.SessionTimeout {
				isExpired = true
			}
		}

		session.mu.RUnlock()

		if isExpired {
			expiredSessions = append(expiredSessions, session)
			delete(sm.sessions, deviceID)
		}
	}

	// 关闭过期会话
	for _, session := range expiredSessions {
		session.mu.Lock()
		session.Status = SessionStatusExpired
		session.mu.Unlock()
		sm.closeSessionInternal(session)
	}
}

// heartbeatChecker 心跳检查器goroutine
func (sm *SessionManager) heartbeatChecker() {
	defer sm.wg.Done()

	ticker := time.NewTicker(sm.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			sm.checkHeartbeats()
		}
	}
}

// checkHeartbeats 检查心跳
func (sm *SessionManager) checkHeartbeats() {
	sm.mu.RLock()
	sessions := make([]*ClientSession, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, session)
	}
	sm.mu.RUnlock()

	now := time.Now()
	for _, session := range sessions {
		session.mu.RLock()
		lastHeartbeat := session.LastHeartbeat
		deviceID := session.DeviceID
		session.mu.RUnlock()

		// 检查心跳超时
		if !lastHeartbeat.IsZero() && now.Sub(lastHeartbeat) > sm.config.HeartbeatInterval*2 {
			// 心跳超时，标记会话为不健康
			sm.handleHeartbeatTimeout(deviceID)
		}
	}
}

// handleHeartbeatTimeout 处理心跳超时
func (sm *SessionManager) handleHeartbeatTimeout(deviceID string) {
	// 可以在这里添加心跳超时处理逻辑，比如发送心跳请求或关闭会话
}

// generateSessionID 生成会话ID
func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}
