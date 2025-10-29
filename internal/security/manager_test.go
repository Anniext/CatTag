package security

import (
	"testing"
	"time"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// TestManager_Authenticate 测试设备认证功能
func TestManager_Authenticate(t *testing.T) {
	config := SecurityConfig{
		EnableAuth:        true,
		EnableEncryption:  true,
		EnableWhitelist:   true,
		KeySize:           32,
		Algorithm:         "AES-256-GCM",
		SessionTimeout:    30 * time.Minute,
		MaxFailedAttempts: 3,
		RequireAuth:       true,
		LogSecurityEvents: true,
	}

	manager := NewManager(config)
	deviceID := "test-device-001"

	// 首先添加设备到白名单
	err := manager.AddToWhitelist(deviceID)
	if err != nil {
		t.Fatalf("添加设备到白名单失败: %v", err)
	}

	// 测试密码认证
	credentials := bluetooth.Credentials{
		Type:     bluetooth.CredentialTypePassword,
		Username: "testuser",
		Password: "testpassword",
	}

	err = manager.Authenticate(deviceID, credentials)
	if err != nil {
		t.Fatalf("设备认证失败: %v", err)
	}

	// 验证会话已创建
	session, err := manager.GetActiveSession(deviceID)
	if err != nil {
		t.Fatalf("获取活跃会话失败: %v", err)
	}

	if !session.IsAuthenticated {
		t.Error("会话应该已认证")
	}

	if session.DeviceID != deviceID {
		t.Errorf("期望设备ID %s，实际 %s", deviceID, session.DeviceID)
	}
}

// TestManager_Authorize 测试设备授权功能
func TestManager_Authorize(t *testing.T) {
	config := SecurityConfig{
		EnableAuth:        true,
		EnableWhitelist:   false, // 禁用白名单以简化测试
		RequireAuth:       true,
		SessionTimeout:    30 * time.Minute,
		MaxFailedAttempts: 3,
		LogSecurityEvents: false,
	}

	manager := NewManager(config)
	deviceID := "test-device-002"

	// 先进行认证
	credentials := bluetooth.Credentials{
		Type:     bluetooth.CredentialTypePassword,
		Password: "testpassword",
	}

	err := manager.Authenticate(deviceID, credentials)
	if err != nil {
		t.Fatalf("设备认证失败: %v", err)
	}

	// 测试授权读操作
	err = manager.Authorize(deviceID, bluetooth.OperationRead)
	if err != nil {
		t.Fatalf("授权读操作失败: %v", err)
	}

	// 测试授权写操作
	err = manager.Authorize(deviceID, bluetooth.OperationWrite)
	if err != nil {
		t.Fatalf("授权写操作失败: %v", err)
	}
}

// TestManager_WhitelistManagement 测试白名单管理功能
func TestManager_WhitelistManagement(t *testing.T) {
	config := SecurityConfig{
		EnableWhitelist: true,
	}

	manager := NewManager(config)
	deviceID := "test-device-003"

	// 测试添加到白名单
	err := manager.AddToWhitelist(deviceID)
	if err != nil {
		t.Fatalf("添加设备到白名单失败: %v", err)
	}

	// 验证设备在白名单中
	if !manager.IsWhitelisted(deviceID) {
		t.Error("设备应该在白名单中")
	}

	// 获取白名单
	whitelist := manager.GetWhitelist()
	found := false
	for _, id := range whitelist {
		if id == deviceID {
			found = true
			break
		}
	}
	if !found {
		t.Error("设备应该在白名单列表中")
	}

	// 测试从白名单移除
	err = manager.RemoveFromWhitelist(deviceID)
	if err != nil {
		t.Fatalf("从白名单移除设备失败: %v", err)
	}

	// 验证设备不在白名单中
	if manager.IsWhitelisted(deviceID) {
		t.Error("设备不应该在白名单中")
	}
}

// TestManager_TrustedDeviceManagement 测试信任设备管理功能
func TestManager_TrustedDeviceManagement(t *testing.T) {
	manager := NewManager(SecurityConfig{})
	deviceID := "test-device-004"

	// 测试添加信任设备
	err := manager.AddTrustedDevice(deviceID)
	if err != nil {
		t.Fatalf("添加信任设备失败: %v", err)
	}

	// 验证设备是信任设备
	if !manager.IsTrustedDevice(deviceID) {
		t.Error("设备应该是信任设备")
	}

	// 获取信任设备列表
	trustedDevices := manager.GetTrustedDevices()
	found := false
	for _, id := range trustedDevices {
		if id == deviceID {
			found = true
			break
		}
	}
	if !found {
		t.Error("设备应该在信任设备列表中")
	}

	// 测试移除信任设备
	err = manager.RemoveTrustedDevice(deviceID)
	if err != nil {
		t.Fatalf("移除信任设备失败: %v", err)
	}

	// 验证设备不是信任设备
	if manager.IsTrustedDevice(deviceID) {
		t.Error("设备不应该是信任设备")
	}
}

// TestManager_AccessPolicyManagement 测试访问策略管理功能
func TestManager_AccessPolicyManagement(t *testing.T) {
	manager := NewManager(SecurityConfig{})
	deviceID := "test-device-005"

	// 创建访问策略
	policy := &bluetooth.AccessPolicy{
		DeviceWhitelist:   []string{deviceID},
		AllowedOperations: []bluetooth.Operation{bluetooth.OperationRead, bluetooth.OperationWrite},
		RequireEncryption: true,
		SessionTimeout:    15 * time.Minute,
		MaxRetries:        3,
	}

	// 设置访问策略
	err := manager.SetAccessPolicy(deviceID, policy)
	if err != nil {
		t.Fatalf("设置访问策略失败: %v", err)
	}

	// 获取访问策略
	retrievedPolicy, err := manager.GetAccessPolicy(deviceID)
	if err != nil {
		t.Fatalf("获取访问策略失败: %v", err)
	}

	// 验证策略内容
	if retrievedPolicy.RequireEncryption != policy.RequireEncryption {
		t.Error("访问策略的加密要求不匹配")
	}

	if len(retrievedPolicy.AllowedOperations) != len(policy.AllowedOperations) {
		t.Error("访问策略的允许操作数量不匹配")
	}
}

// TestManager_SessionManagement 测试会话管理功能
func TestManager_SessionManagement(t *testing.T) {
	config := SecurityConfig{
		EnableAuth:     true,
		SessionTimeout: 1 * time.Second, // 短超时时间用于测试
		RequireAuth:    true,
	}

	manager := NewManager(config)
	deviceID := "test-device-006"

	// 创建会话
	credentials := bluetooth.Credentials{
		Type:     bluetooth.CredentialTypePassword,
		Password: "testpassword",
	}

	err := manager.Authenticate(deviceID, credentials)
	if err != nil {
		t.Fatalf("设备认证失败: %v", err)
	}

	// 验证会话存在
	session, err := manager.GetActiveSession(deviceID)
	if err != nil {
		t.Fatalf("获取活跃会话失败: %v", err)
	}

	if !session.IsAuthenticated {
		t.Error("会话应该已认证")
	}

	// 撤销会话
	err = manager.RevokeSession(deviceID)
	if err != nil {
		t.Fatalf("撤销会话失败: %v", err)
	}

	// 验证会话已撤销
	_, err = manager.GetActiveSession(deviceID)
	if err == nil {
		t.Error("会话应该已被撤销")
	}
}

// TestManager_EncryptionDecryption 测试加密解密功能
func TestManager_EncryptionDecryption(t *testing.T) {
	config := SecurityConfig{
		EnableEncryption: true,
		KeySize:          32,
		Algorithm:        "AES-256-GCM",
		SessionTimeout:   30 * time.Minute,
	}

	manager := NewManager(config)
	deviceID := "test-device-007"

	// 生成会话密钥
	_, err := manager.GenerateSessionKey(deviceID)
	if err != nil {
		t.Fatalf("生成会话密钥失败: %v", err)
	}

	// 测试数据
	originalData := []byte("这是一个测试消息，用于验证加密解密功能")

	// 加密数据
	encryptedData, err := manager.Encrypt(originalData, deviceID)
	if err != nil {
		t.Fatalf("加密数据失败: %v", err)
	}

	// 验证加密后数据不同
	if string(encryptedData) == string(originalData) {
		t.Error("加密后的数据不应该与原数据相同")
	}

	// 解密数据
	decryptedData, err := manager.Decrypt(encryptedData, deviceID)
	if err != nil {
		t.Fatalf("解密数据失败: %v", err)
	}

	// 验证解密后数据正确
	if string(decryptedData) != string(originalData) {
		t.Errorf("解密后数据不匹配，期望: %s，实际: %s", string(originalData), string(decryptedData))
	}
}

// TestManager_KeyRotation 测试密钥轮换功能
func TestManager_KeyRotation(t *testing.T) {
	config := SecurityConfig{
		EnableEncryption: true,
		KeySize:          32,
		Algorithm:        "AES-256-GCM",
		SessionTimeout:   30 * time.Minute,
	}

	manager := NewManager(config)
	deviceID := "test-device-008"

	// 生成初始密钥
	originalKey, err := manager.GenerateSessionKey(deviceID)
	if err != nil {
		t.Fatalf("生成初始会话密钥失败: %v", err)
	}

	// 轮换密钥
	newKey, err := manager.RotateSessionKey(deviceID)
	if err != nil {
		t.Fatalf("轮换会话密钥失败: %v", err)
	}

	// 验证密钥已更改
	if string(originalKey) == string(newKey) {
		t.Error("轮换后的密钥不应该与原密钥相同")
	}

	// 验证新密钥可以正常使用
	testData := []byte("测试轮换后的密钥")
	encryptedData, err := manager.Encrypt(testData, deviceID)
	if err != nil {
		t.Fatalf("使用新密钥加密失败: %v", err)
	}

	decryptedData, err := manager.Decrypt(encryptedData, deviceID)
	if err != nil {
		t.Fatalf("使用新密钥解密失败: %v", err)
	}

	if string(decryptedData) != string(testData) {
		t.Error("使用新密钥加密解密的数据不匹配")
	}
}

// TestManager_SecurityEvents 测试安全事件功能
func TestManager_SecurityEvents(t *testing.T) {
	config := SecurityConfig{
		LogSecurityEvents: true,
	}

	manager := NewManager(config)
	deviceID := "test-device-009"

	// 获取安全事件通道
	eventChan := manager.GetSecurityEvents()

	// 触发一个安全事件
	err := manager.AddToWhitelist(deviceID)
	if err != nil {
		t.Fatalf("添加设备到白名单失败: %v", err)
	}

	// 等待并验证事件
	select {
	case event := <-eventChan:
		if event.DeviceID != deviceID {
			t.Errorf("事件设备ID不匹配，期望: %s，实际: %s", deviceID, event.DeviceID)
		}
		if event.Type != bluetooth.SecurityEventAuthSuccess {
			t.Errorf("事件类型不匹配，期望: %v，实际: %v", bluetooth.SecurityEventAuthSuccess, event.Type)
		}
	case <-time.After(1 * time.Second):
		t.Error("未收到预期的安全事件")
	}
}

// TestManager_AuthenticationFailures 测试认证失败处理
func TestManager_AuthenticationFailures(t *testing.T) {
	config := SecurityConfig{
		EnableAuth:        true,
		MaxFailedAttempts: 2,
		RequireAuth:       true,
		LogSecurityEvents: false,
	}

	manager := NewManager(config)
	deviceID := "test-device-010"

	// 错误的认证凭据
	wrongCredentials := bluetooth.Credentials{
		Type:     bluetooth.CredentialTypePassword,
		Password: "wrongpassword",
	}

	// 第一次失败尝试
	err := manager.Authenticate(deviceID, wrongCredentials)
	if err == nil {
		t.Error("应该认证失败")
	}

	// 第二次失败尝试
	err = manager.Authenticate(deviceID, wrongCredentials)
	if err == nil {
		t.Error("应该认证失败")
	}

	// 第三次尝试应该被阻止
	err = manager.Authenticate(deviceID, wrongCredentials)
	if err == nil {
		t.Error("应该因为失败次数过多而被阻止")
	}

	// 验证错误类型
	if bluetoothErr, ok := err.(*bluetooth.BluetoothError); ok {
		if bluetoothErr.Code != bluetooth.ErrCodeAuthFailed {
			t.Errorf("期望错误代码 %d，实际 %d", bluetooth.ErrCodeAuthFailed, bluetoothErr.Code)
		}
	} else {
		t.Error("应该返回 BluetoothError 类型的错误")
	}
}

// TestManager_AccessValidation 测试访问验证功能
func TestManager_AccessValidation(t *testing.T) {
	config := SecurityConfig{
		EnableWhitelist: true,
		RequireAuth:     true,
	}

	manager := NewManager(config)
	deviceID := "test-device-011"
	unauthorizedDeviceID := "unauthorized-device"

	// 设置访问策略
	policy := &bluetooth.AccessPolicy{
		AllowedOperations: []bluetooth.Operation{bluetooth.OperationRead},
		RequireEncryption: false,
	}
	manager.SetAccessPolicy(deviceID, policy)
	manager.AddToWhitelist(deviceID)

	// 测试授权设备的允许操作
	err := manager.ValidateAccess(deviceID, bluetooth.OperationRead)
	if err != nil {
		t.Fatalf("授权设备的允许操作验证失败: %v", err)
	}

	// 测试授权设备的不允许操作
	err = manager.ValidateAccess(deviceID, bluetooth.OperationWrite)
	if err == nil {
		t.Error("授权设备的不允许操作应该验证失败")
	}

	// 测试未授权设备
	err = manager.ValidateAccess(unauthorizedDeviceID, bluetooth.OperationRead)
	if err == nil {
		t.Error("未授权设备应该验证失败")
	}
}

// TestManager_CleanupExpiredSessions 测试清理过期会话功能
func TestManager_CleanupExpiredSessions(t *testing.T) {
	config := SecurityConfig{
		EnableAuth:     true,
		SessionTimeout: 100 * time.Millisecond, // 很短的超时时间
		RequireAuth:    false,                  // 简化测试
	}

	manager := NewManager(config)
	deviceID := "test-device-012"

	// 创建会话
	credentials := bluetooth.Credentials{
		Type:     bluetooth.CredentialTypePassword,
		Password: "testpassword",
	}

	err := manager.Authenticate(deviceID, credentials)
	if err != nil {
		t.Fatalf("设备认证失败: %v", err)
	}

	// 验证会话存在
	_, err = manager.GetActiveSession(deviceID)
	if err != nil {
		t.Fatalf("获取活跃会话失败: %v", err)
	}

	// 等待会话过期
	time.Sleep(200 * time.Millisecond)

	// 清理过期会话
	manager.CleanupExpiredSessions()

	// 验证会话已被清理
	_, err = manager.GetActiveSession(deviceID)
	if err == nil {
		t.Error("过期会话应该已被清理")
	}
}

// TestManager_SecurityStats 测试安全统计功能
func TestManager_SecurityStats(t *testing.T) {
	manager := NewManager(SecurityConfig{})

	// 添加一些测试数据
	manager.AddToWhitelist("device1")
	manager.AddToWhitelist("device2")
	manager.AddTrustedDevice("device3")

	// 获取统计信息
	stats := manager.GetSecurityStats()

	// 验证统计信息
	if whitelistedCount, ok := stats["whitelisted_devices"].(int); !ok || whitelistedCount != 2 {
		t.Errorf("期望白名单设备数量为 2，实际 %v", stats["whitelisted_devices"])
	}

	if trustedCount, ok := stats["trusted_devices"].(int); !ok || trustedCount != 1 {
		t.Errorf("期望信任设备数量为 1，实际 %v", stats["trusted_devices"])
	}
}

// TestManager_SecurityFeatureToggle 测试安全功能开关
func TestManager_SecurityFeatureToggle(t *testing.T) {
	manager := NewManager(SecurityConfig{
		EnableAuth:        true,
		EnableEncryption:  true,
		EnableWhitelist:   true,
		LogSecurityEvents: true,
	})

	// 测试禁用认证
	err := manager.EnableSecurityFeature("auth", false)
	if err != nil {
		t.Fatalf("禁用认证功能失败: %v", err)
	}

	if manager.config.EnableAuth {
		t.Error("认证功能应该已被禁用")
	}

	// 测试启用认证
	err = manager.EnableSecurityFeature("auth", true)
	if err != nil {
		t.Fatalf("启用认证功能失败: %v", err)
	}

	if !manager.config.EnableAuth {
		t.Error("认证功能应该已被启用")
	}

	// 测试不支持的功能
	err = manager.EnableSecurityFeature("unsupported", true)
	if err == nil {
		t.Error("不支持的功能应该返回错误")
	}
}
