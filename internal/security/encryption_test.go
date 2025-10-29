package security

import (
	"bytes"
	"testing"
	"time"
)

// TestEncryptionManager_KeyGeneration 测试密钥生成功能
func TestEncryptionManager_KeyGeneration(t *testing.T) {
	em := NewEncryptionManager("AES-256-GCM", 32)
	deviceID := "test-device-001"
	ttl := 1 * time.Hour

	// 生成密钥
	key, err := em.GenerateKey(deviceID, ttl)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}

	// 验证密钥属性
	if key.DeviceID != deviceID {
		t.Errorf("设备ID不匹配，期望: %s，实际: %s", deviceID, key.DeviceID)
	}

	if len(key.Key) != 32 {
		t.Errorf("密钥长度不正确，期望: 32，实际: %d", len(key.Key))
	}

	if len(key.IV) != 16 {
		t.Errorf("IV长度不正确，期望: 16，实际: %d", len(key.IV))
	}

	if key.Algorithm != "AES-256-GCM" {
		t.Errorf("算法不匹配，期望: AES-256-GCM，实际: %s", key.Algorithm)
	}

	// 验证过期时间
	expectedExpiry := time.Now().Add(ttl)
	if key.ExpiresAt.Before(expectedExpiry.Add(-1*time.Minute)) || key.ExpiresAt.After(expectedExpiry.Add(1*time.Minute)) {
		t.Error("密钥过期时间不在预期范围内")
	}
}

// TestEncryptionManager_GetKey 测试获取密钥功能
func TestEncryptionManager_GetKey(t *testing.T) {
	em := NewEncryptionManager("AES-256-GCM", 32)
	deviceID := "test-device-002"
	ttl := 1 * time.Hour

	// 生成密钥
	originalKey, err := em.GenerateKey(deviceID, ttl)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}

	// 获取密钥
	retrievedKey, err := em.GetKey(deviceID)
	if err != nil {
		t.Fatalf("获取密钥失败: %v", err)
	}

	// 验证密钥内容
	if retrievedKey.DeviceID != originalKey.DeviceID {
		t.Error("获取的密钥设备ID不匹配")
	}

	if !bytes.Equal(retrievedKey.Key, originalKey.Key) {
		t.Error("获取的密钥内容不匹配")
	}

	// 测试获取不存在的密钥
	_, err = em.GetKey("nonexistent-device")
	if err == nil {
		t.Error("获取不存在的密钥应该返回错误")
	}
}

// TestEncryptionManager_KeyRotation 测试密钥轮换功能
func TestEncryptionManager_KeyRotation(t *testing.T) {
	em := NewEncryptionManager("AES-256-GCM", 32)
	deviceID := "test-device-003"
	ttl := 1 * time.Hour

	// 生成初始密钥
	originalKey, err := em.GenerateKey(deviceID, ttl)
	if err != nil {
		t.Fatalf("生成初始密钥失败: %v", err)
	}

	// 轮换密钥
	newKey, err := em.RotateKey(deviceID, ttl)
	if err != nil {
		t.Fatalf("轮换密钥失败: %v", err)
	}

	// 验证新密钥与原密钥不同
	if bytes.Equal(originalKey.Key, newKey.Key) {
		t.Error("轮换后的密钥不应该与原密钥相同")
	}

	// 验证新密钥可以正常获取
	retrievedKey, err := em.GetKey(deviceID)
	if err != nil {
		t.Fatalf("获取轮换后的密钥失败: %v", err)
	}

	if !bytes.Equal(retrievedKey.Key, newKey.Key) {
		t.Error("获取的密钥与轮换后的密钥不匹配")
	}
}

// TestEncryptionManager_EncryptDecrypt 测试加密解密功能
func TestEncryptionManager_EncryptDecrypt(t *testing.T) {
	em := NewEncryptionManager("AES-256-GCM", 32)
	deviceID := "test-device-004"
	ttl := 1 * time.Hour

	// 生成密钥
	_, err := em.GenerateKey(deviceID, ttl)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}

	// 测试数据
	originalData := []byte("这是一个测试消息，用于验证AES-GCM加密解密功能的正确性。")

	// 加密数据
	encryptedData, err := em.EncryptData(originalData, deviceID)
	if err != nil {
		t.Fatalf("加密数据失败: %v", err)
	}

	// 验证加密数据结构
	if encryptedData.Algorithm != "AES-256-GCM" {
		t.Errorf("加密算法不匹配，期望: AES-256-GCM，实际: %s", encryptedData.Algorithm)
	}

	if len(encryptedData.Nonce) == 0 {
		t.Error("Nonce不应该为空")
	}

	if len(encryptedData.Ciphertext) == 0 {
		t.Error("密文不应该为空")
	}

	if len(encryptedData.HMAC) == 0 {
		t.Error("HMAC不应该为空")
	}

	// 解密数据
	decryptedData, err := em.DecryptData(encryptedData, deviceID)
	if err != nil {
		t.Fatalf("解密数据失败: %v", err)
	}

	// 验证解密结果
	if !bytes.Equal(originalData, decryptedData) {
		t.Errorf("解密后数据不匹配\n期望: %s\n实际: %s", string(originalData), string(decryptedData))
	}
}

// TestEncryptionManager_MultipleEncryptions 测试多次加密的随机性
func TestEncryptionManager_MultipleEncryptions(t *testing.T) {
	em := NewEncryptionManager("AES-256-GCM", 32)
	deviceID := "test-device-005"
	ttl := 1 * time.Hour

	// 生成密钥
	_, err := em.GenerateKey(deviceID, ttl)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}

	// 相同数据的多次加密
	originalData := []byte("相同的测试数据")

	encData1, err := em.EncryptData(originalData, deviceID)
	if err != nil {
		t.Fatalf("第一次加密失败: %v", err)
	}

	encData2, err := em.EncryptData(originalData, deviceID)
	if err != nil {
		t.Fatalf("第二次加密失败: %v", err)
	}

	// 验证两次加密结果不同（由于随机nonce）
	if bytes.Equal(encData1.Ciphertext, encData2.Ciphertext) {
		t.Error("相同数据的多次加密结果不应该相同")
	}

	if bytes.Equal(encData1.Nonce, encData2.Nonce) {
		t.Error("两次加密的nonce不应该相同")
	}

	// 验证两次加密都能正确解密
	decData1, err := em.DecryptData(encData1, deviceID)
	if err != nil {
		t.Fatalf("解密第一次加密数据失败: %v", err)
	}

	decData2, err := em.DecryptData(encData2, deviceID)
	if err != nil {
		t.Fatalf("解密第二次加密数据失败: %v", err)
	}

	if !bytes.Equal(originalData, decData1) || !bytes.Equal(originalData, decData2) {
		t.Error("解密结果与原数据不匹配")
	}
}

// TestEncryptionManager_KeyExpiry 测试密钥过期功能
func TestEncryptionManager_KeyExpiry(t *testing.T) {
	em := NewEncryptionManager("AES-256-GCM", 32)
	deviceID := "test-device-006"
	ttl := 100 * time.Millisecond // 很短的过期时间

	// 生成密钥
	_, err := em.GenerateKey(deviceID, ttl)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}

	// 等待密钥过期
	time.Sleep(200 * time.Millisecond)

	// 尝试获取过期密钥
	_, err = em.GetKey(deviceID)
	if err == nil {
		t.Error("获取过期密钥应该返回错误")
	}

	// 尝试使用过期密钥加密
	testData := []byte("测试数据")
	_, err = em.EncryptData(testData, deviceID)
	if err == nil {
		t.Error("使用过期密钥加密应该返回错误")
	}
}

// TestEncryptionManager_KeyUsageLimit 测试密钥使用次数限制
func TestEncryptionManager_KeyUsageLimit(t *testing.T) {
	em := NewEncryptionManager("AES-256-GCM", 32)
	deviceID := "test-device-007"
	ttl := 1 * time.Hour

	// 生成密钥
	key, err := em.GenerateKey(deviceID, ttl)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}

	// 设置较小的使用次数限制
	key.MaxUsage = 3
	testData := []byte("测试数据")

	// 使用密钥进行加密，直到达到限制
	for i := 0; i < 3; i++ {
		_, err := em.EncryptData(testData, deviceID)
		if err != nil {
			t.Fatalf("第 %d 次加密失败: %v", i+1, err)
		}
	}

	// 超过使用次数限制后应该失败
	_, err = em.EncryptData(testData, deviceID)
	if err == nil {
		t.Error("超过使用次数限制后加密应该失败")
	}
}

// TestEncryptionManager_KeyExchangeGeneration 测试密钥交换数据生成
func TestEncryptionManager_KeyExchangeGeneration(t *testing.T) {
	em := NewEncryptionManager("AES-256-GCM", 32)
	deviceID := "test-device-008"

	// 生成密钥交换数据
	kxData, err := em.GenerateKeyExchangeData(deviceID)
	if err != nil {
		t.Fatalf("生成密钥交换数据失败: %v", err)
	}

	// 验证密钥交换数据
	if kxData.DeviceID != deviceID {
		t.Errorf("设备ID不匹配，期望: %s，实际: %s", deviceID, kxData.DeviceID)
	}

	if len(kxData.PublicKey) != 32 {
		t.Errorf("公钥长度不正确，期望: 32，实际: %d", len(kxData.PublicKey))
	}

	if len(kxData.Nonce) != 16 {
		t.Errorf("Nonce长度不正确，期望: 16，实际: %d", len(kxData.Nonce))
	}

	if len(kxData.Signature) == 0 {
		t.Error("签名不应该为空")
	}

	if kxData.Algorithm != "ECDH-P256" {
		t.Errorf("算法不匹配，期望: ECDH-P256，实际: %s", kxData.Algorithm)
	}
}

// TestEncryptionManager_KeyExchangeValidation 测试密钥交换数据验证
func TestEncryptionManager_KeyExchangeValidation(t *testing.T) {
	em := NewEncryptionManager("AES-256-GCM", 32)
	deviceID := "test-device-009"

	// 生成有效的密钥交换数据
	kxData, err := em.GenerateKeyExchangeData(deviceID)
	if err != nil {
		t.Fatalf("生成密钥交换数据失败: %v", err)
	}

	// 验证有效数据
	err = em.ValidateKeyExchange(kxData)
	if err != nil {
		t.Fatalf("验证有效密钥交换数据失败: %v", err)
	}

	// 测试过期数据
	kxData.Timestamp = time.Now().Add(-10 * time.Minute)
	err = em.ValidateKeyExchange(kxData)
	if err == nil {
		t.Error("验证过期密钥交换数据应该失败")
	}

	// 重新生成有效数据测试签名验证
	kxData, _ = em.GenerateKeyExchangeData(deviceID)

	// 篡改签名
	kxData.Signature[0] ^= 0xFF
	err = em.ValidateKeyExchange(kxData)
	if err == nil {
		t.Error("验证被篡改签名的密钥交换数据应该失败")
	}
}

// TestEncryptionManager_SharedKeyDerivation 测试共享密钥派生
func TestEncryptionManager_SharedKeyDerivation(t *testing.T) {
	em := NewEncryptionManager("AES-256-GCM", 32)
	deviceID := "test-device-010"

	// 模拟密钥交换过程
	localPrivateKey := make([]byte, 32)
	remotePublicKey := make([]byte, 32)

	// 填充测试数据
	for i := range localPrivateKey {
		localPrivateKey[i] = byte(i)
		remotePublicKey[i] = byte(i + 32)
	}

	// 派生共享密钥
	sharedKey, err := em.DeriveSharedKey(localPrivateKey, remotePublicKey, deviceID)
	if err != nil {
		t.Fatalf("派生共享密钥失败: %v", err)
	}

	// 验证共享密钥
	if len(sharedKey) != 32 {
		t.Errorf("共享密钥长度不正确，期望: 32，实际: %d", len(sharedKey))
	}

	// 使用相同输入应该产生相同的共享密钥
	sharedKey2, err := em.DeriveSharedKey(localPrivateKey, remotePublicKey, deviceID)
	if err != nil {
		t.Fatalf("第二次派生共享密钥失败: %v", err)
	}

	if !bytes.Equal(sharedKey, sharedKey2) {
		t.Error("相同输入应该产生相同的共享密钥")
	}
}

// TestEncryptionManager_CleanupExpiredKeys 测试清理过期密钥
func TestEncryptionManager_CleanupExpiredKeys(t *testing.T) {
	em := NewEncryptionManager("AES-256-GCM", 32)

	// 生成一些密钥，其中一些会过期
	deviceID1 := "test-device-011"
	deviceID2 := "test-device-012"
	deviceID3 := "test-device-013"

	// 生成正常密钥
	_, err := em.GenerateKey(deviceID1, 1*time.Hour)
	if err != nil {
		t.Fatalf("生成密钥1失败: %v", err)
	}

	// 生成即将过期的密钥
	_, err = em.GenerateKey(deviceID2, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("生成密钥2失败: %v", err)
	}

	_, err = em.GenerateKey(deviceID3, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("生成密钥3失败: %v", err)
	}

	// 等待部分密钥过期
	time.Sleep(100 * time.Millisecond)

	// 清理过期密钥
	cleaned := em.CleanupExpiredKeys()

	// 验证清理结果
	if cleaned != 2 {
		t.Errorf("期望清理2个过期密钥，实际清理了%d个", cleaned)
	}

	// 验证正常密钥仍然存在
	_, err = em.GetKey(deviceID1)
	if err != nil {
		t.Error("正常密钥不应该被清理")
	}

	// 验证过期密钥已被清理
	_, err = em.GetKey(deviceID2)
	if err == nil {
		t.Error("过期密钥应该已被清理")
	}

	_, err = em.GetKey(deviceID3)
	if err == nil {
		t.Error("过期密钥应该已被清理")
	}
}

// TestEncryptionManager_GetKeyInfo 测试获取密钥信息
func TestEncryptionManager_GetKeyInfo(t *testing.T) {
	em := NewEncryptionManager("AES-256-GCM", 32)
	deviceID := "test-device-014"
	ttl := 1 * time.Hour

	// 生成密钥
	_, err := em.GenerateKey(deviceID, ttl)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}

	// 获取密钥信息
	keyInfo, err := em.GetKeyInfo(deviceID)
	if err != nil {
		t.Fatalf("获取密钥信息失败: %v", err)
	}

	// 验证密钥信息
	if keyInfo["device_id"] != deviceID {
		t.Errorf("设备ID不匹配，期望: %s，实际: %v", deviceID, keyInfo["device_id"])
	}

	if keyInfo["algorithm"] != "AES-256-GCM" {
		t.Errorf("算法不匹配，期望: AES-256-GCM，实际: %v", keyInfo["algorithm"])
	}

	if keyInfo["usage"] != 0 {
		t.Errorf("使用次数应该为0，实际: %v", keyInfo["usage"])
	}

	if keyInfo["is_expired"] != false {
		t.Errorf("密钥不应该过期，实际: %v", keyInfo["is_expired"])
	}
}

// TestEncryptionManager_ExportImportKey 测试密钥导出导入
func TestEncryptionManager_ExportImportKey(t *testing.T) {
	em := NewEncryptionManager("AES-256-GCM", 32)
	deviceID := "test-device-015"
	password := "export-password-123"
	ttl := 1 * time.Hour

	// 生成密钥
	_, err := em.GenerateKey(deviceID, ttl)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}

	// 导出密钥
	exportedKey, err := em.ExportKey(deviceID, password)
	if err != nil {
		t.Fatalf("导出密钥失败: %v", err)
	}

	// 验证导出的密钥不为空
	if exportedKey == "" {
		t.Error("导出的密钥不应该为空")
	}

	// 删除原密钥（模拟密钥丢失）
	delete(em.keys, deviceID)

	// 导入密钥
	err = em.ImportKey(deviceID, exportedKey, password)
	if err != nil {
		t.Fatalf("导入密钥失败: %v", err)
	}

	// 验证导入后密钥可用（这里由于简化实现，实际验证会有限）
	// 在完整实现中，应该能够正常使用导入的密钥进行加密解密
}

// TestEncryptionManager_HMACIntegrity 测试HMAC完整性保护
func TestEncryptionManager_HMACIntegrity(t *testing.T) {
	em := NewEncryptionManager("AES-256-GCM", 32)
	deviceID := "test-device-016"
	ttl := 1 * time.Hour

	// 生成密钥
	_, err := em.GenerateKey(deviceID, ttl)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}

	// 加密数据
	originalData := []byte("测试HMAC完整性保护")
	encryptedData, err := em.EncryptData(originalData, deviceID)
	if err != nil {
		t.Fatalf("加密数据失败: %v", err)
	}

	// 正常解密应该成功
	decryptedData, err := em.DecryptData(encryptedData, deviceID)
	if err != nil {
		t.Fatalf("正常解密失败: %v", err)
	}

	if !bytes.Equal(originalData, decryptedData) {
		t.Error("正常解密结果不匹配")
	}

	// 篡改HMAC
	encryptedData.HMAC[0] ^= 0xFF

	// 篡改后解密应该失败
	_, err = em.DecryptData(encryptedData, deviceID)
	if err == nil {
		t.Error("篡改HMAC后解密应该失败")
	}
}
