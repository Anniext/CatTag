package bluetooth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

// DefaultSecurityManager 默认安全管理器实现
type DefaultSecurityManager struct {
	config         SecurityConfig
	deviceSessions map[string]*SecuritySession
	trustedDevices map[string]bool
	accessPolicies map[string]*AccessPolicy
	mu             sync.RWMutex
}

// NewDefaultSecurityManager 创建新的默认安全管理器
func NewDefaultSecurityManager(config SecurityConfig) *DefaultSecurityManager {
	return &DefaultSecurityManager{
		config:         config,
		deviceSessions: make(map[string]*SecuritySession),
		trustedDevices: make(map[string]bool),
		accessPolicies: make(map[string]*AccessPolicy),
	}
}

// Authenticate 设备认证
func (sm *DefaultSecurityManager) Authenticate(deviceID string, credentials Credentials) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 检查设备是否在信任列表中
	if sm.isTrustedDevice(deviceID) {
		return sm.createSession(deviceID)
	}

	// 获取或创建会话
	session := sm.getOrCreateSession(deviceID)

	// 检查失败尝试次数
	if session.FailedAttempts >= sm.config.MaxFailedAttempts {
		return NewBluetoothError(ErrCodeAuthFailed, "认证失败次数过多")
	}

	// 根据凭据类型进行认证
	var err error
	switch credentials.Type {
	case CredentialTypePassword:
		err = sm.authenticateWithPassword(deviceID, credentials.Password)
	case CredentialTypeToken:
		err = sm.authenticateWithToken(deviceID, credentials.Token)
	case CredentialTypeCertificate:
		err = sm.authenticateWithCertificate(deviceID, credentials.KeyData)
	case CredentialTypeKey:
		err = sm.authenticateWithKey(deviceID, credentials.KeyData)
	default:
		return NewBluetoothError(ErrCodeNotSupported, "不支持的认证类型")
	}

	if err != nil {
		session.FailedAttempts++
		return fmt.Errorf("认证失败: %w", err)
	}

	// 认证成功
	session.IsAuthenticated = true
	session.FailedAttempts = 0
	session.LastActivity = time.Now()

	return nil
}

// Authorize 设备授权
func (sm *DefaultSecurityManager) Authorize(deviceID string, operation Operation) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 检查会话是否存在且已认证
	session, exists := sm.deviceSessions[deviceID]
	if !exists || !session.IsAuthenticated {
		return NewBluetoothError(ErrCodeAuthFailed, "设备未认证")
	}

	// 检查会话是否过期
	if time.Since(session.LastActivity) > sm.config.SessionTimeout {
		return NewBluetoothError(ErrCodeAuthFailed, "会话已过期")
	}

	// 检查访问策略
	policy, exists := sm.accessPolicies[deviceID]
	if exists {
		if !sm.isOperationAllowed(operation, policy) {
			return NewBluetoothError(ErrCodePermissionDenied, "操作未授权")
		}
	}

	// 更新最后活动时间
	session.LastActivity = time.Now()

	return nil
}

// Encrypt 加密数据
func (sm *DefaultSecurityManager) Encrypt(data []byte, deviceID string) ([]byte, error) {
	if !sm.config.EnableEncryption {
		return data, nil
	}

	sm.mu.RLock()
	session, exists := sm.deviceSessions[deviceID]
	sm.mu.RUnlock()

	if !exists || !session.IsAuthenticated {
		return nil, NewBluetoothError(ErrCodeAuthFailed, "设备未认证")
	}

	// 使用AES-GCM加密
	block, err := aes.NewCipher(session.SessionKey)
	if err != nil {
		return nil, fmt.Errorf("创建加密器失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建GCM失败: %w", err)
	}

	// 生成随机nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("生成nonce失败: %w", err)
	}

	// 加密数据
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// Decrypt 解密数据
func (sm *DefaultSecurityManager) Decrypt(data []byte, deviceID string) ([]byte, error) {
	if !sm.config.EnableEncryption {
		return data, nil
	}

	sm.mu.RLock()
	session, exists := sm.deviceSessions[deviceID]
	sm.mu.RUnlock()

	if !exists || !session.IsAuthenticated {
		return nil, NewBluetoothError(ErrCodeAuthFailed, "设备未认证")
	}

	// 使用AES-GCM解密
	block, err := aes.NewCipher(session.SessionKey)
	if err != nil {
		return nil, fmt.Errorf("创建解密器失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建GCM失败: %w", err)
	}

	// 检查数据长度
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, NewBluetoothError(ErrCodeEncryptionFailed, "加密数据格式错误")
	}

	// 提取nonce和密文
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]

	// 解密数据
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %w", err)
	}

	return plaintext, nil
}

// GenerateSessionKey 生成会话密钥
func (sm *DefaultSecurityManager) GenerateSessionKey(deviceID string) ([]byte, error) {
	key := make([]byte, sm.config.KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("生成会话密钥失败: %w", err)
	}

	// 存储会话密钥
	sm.mu.Lock()
	session := sm.getOrCreateSession(deviceID)
	session.SessionKey = key
	sm.mu.Unlock()

	return key, nil
}

// AddTrustedDevice 添加信任设备
func (sm *DefaultSecurityManager) AddTrustedDevice(deviceID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.trustedDevices[deviceID] = true
}

// RemoveTrustedDevice 移除信任设备
func (sm *DefaultSecurityManager) RemoveTrustedDevice(deviceID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.trustedDevices, deviceID)
}

// SetAccessPolicy 设置访问策略
func (sm *DefaultSecurityManager) SetAccessPolicy(deviceID string, policy *AccessPolicy) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.accessPolicies[deviceID] = policy
}

// isTrustedDevice 检查是否为信任设备
func (sm *DefaultSecurityManager) isTrustedDevice(deviceID string) bool {
	return sm.trustedDevices[deviceID]
}

// getOrCreateSession 获取或创建会话
func (sm *DefaultSecurityManager) getOrCreateSession(deviceID string) *SecuritySession {
	session, exists := sm.deviceSessions[deviceID]
	if !exists {
		session = &SecuritySession{
			DeviceID:        deviceID,
			CreatedAt:       time.Now(),
			LastActivity:    time.Now(),
			IsAuthenticated: false,
			FailedAttempts:  0,
		}
		sm.deviceSessions[deviceID] = session
	}
	return session
}

// createSession 创建认证会话
func (sm *DefaultSecurityManager) createSession(deviceID string) error {
	session := sm.getOrCreateSession(deviceID)
	session.IsAuthenticated = true
	session.FailedAttempts = 0
	session.LastActivity = time.Now()

	// 生成会话密钥
	if sm.config.EnableEncryption {
		_, err := sm.GenerateSessionKey(deviceID)
		if err != nil {
			return fmt.Errorf("生成会话密钥失败: %w", err)
		}
	}

	return nil
}

// authenticateWithPassword 密码认证
func (sm *DefaultSecurityManager) authenticateWithPassword(deviceID, password string) error {
	// 简单的密码验证逻辑，实际实现应该更安全
	if password == "default_password" {
		return nil
	}
	return NewBluetoothError(ErrCodeAuthFailed, "密码错误")
}

// authenticateWithToken 令牌认证
func (sm *DefaultSecurityManager) authenticateWithToken(deviceID, token string) error {
	// 简单的令牌验证逻辑
	if token != "" && len(token) >= 8 {
		return nil
	}
	return NewBluetoothError(ErrCodeAuthFailed, "令牌无效")
}

// authenticateWithCertificate 证书认证
func (sm *DefaultSecurityManager) authenticateWithCertificate(deviceID string, certData []byte) error {
	// 简单的证书验证逻辑
	if len(certData) > 0 {
		return nil
	}
	return NewBluetoothError(ErrCodeAuthFailed, "证书无效")
}

// authenticateWithKey 密钥认证
func (sm *DefaultSecurityManager) authenticateWithKey(deviceID string, keyData []byte) error {
	// 简单的密钥验证逻辑
	if len(keyData) >= 16 {
		return nil
	}
	return NewBluetoothError(ErrCodeAuthFailed, "密钥无效")
}

// isOperationAllowed 检查操作是否被允许
func (sm *DefaultSecurityManager) isOperationAllowed(operation Operation, policy *AccessPolicy) bool {
	for _, allowedOp := range policy.AllowedOperations {
		if allowedOp == operation {
			return true
		}
	}
	return false
}
