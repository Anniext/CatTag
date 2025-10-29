package security

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFileKeyStore_StoreAndGetKey 测试密钥存储和获取
func TestFileKeyStore_StoreAndGetKey(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "keystore_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := KeyStoreConfig{
		StorePath:    filepath.Join(tempDir, "keystore.json"),
		Password:     "test-password-123",
		AutoSave:     false,
		SaveInterval: 0,
	}

	// 创建密钥存储
	fks, err := NewFileKeyStore(config)
	if err != nil {
		t.Fatalf("创建文件密钥存储失败: %v", err)
	}
	defer fks.Close()

	deviceID := "test-device-001"
	originalKey := []byte("这是一个测试密钥，用于验证存储和获取功能")

	// 存储密钥
	err = fks.StoreKey(deviceID, originalKey)
	if err != nil {
		t.Fatalf("存储密钥失败: %v", err)
	}

	// 获取密钥
	retrievedKey, err := fks.GetKey(deviceID)
	if err != nil {
		t.Fatalf("获取密钥失败: %v", err)
	}

	// 验证密钥内容
	if !bytes.Equal(originalKey, retrievedKey) {
		t.Errorf("获取的密钥与原密钥不匹配\n期望: %s\n实际: %s", string(originalKey), string(retrievedKey))
	}
}

// TestFileKeyStore_DeleteKey 测试密钥删除
func TestFileKeyStore_DeleteKey(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "keystore_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := KeyStoreConfig{
		StorePath: filepath.Join(tempDir, "keystore.json"),
		Password:  "test-password-123",
		AutoSave:  false,
	}

	fks, err := NewFileKeyStore(config)
	if err != nil {
		t.Fatalf("创建文件密钥存储失败: %v", err)
	}
	defer fks.Close()

	deviceID := "test-device-002"
	testKey := []byte("测试密钥")

	// 存储密钥
	err = fks.StoreKey(deviceID, testKey)
	if err != nil {
		t.Fatalf("存储密钥失败: %v", err)
	}

	// 验证密钥存在
	_, err = fks.GetKey(deviceID)
	if err != nil {
		t.Fatalf("获取密钥失败: %v", err)
	}

	// 删除密钥
	err = fks.DeleteKey(deviceID)
	if err != nil {
		t.Fatalf("删除密钥失败: %v", err)
	}

	// 验证密钥已删除
	_, err = fks.GetKey(deviceID)
	if err == nil {
		t.Error("删除后仍能获取密钥")
	}

	// 测试删除不存在的密钥
	err = fks.DeleteKey("nonexistent-device")
	if err == nil {
		t.Error("删除不存在的密钥应该返回错误")
	}
}

// TestFileKeyStore_ListKeys 测试密钥列表功能
func TestFileKeyStore_ListKeys(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "keystore_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := KeyStoreConfig{
		StorePath: filepath.Join(tempDir, "keystore.json"),
		Password:  "test-password-123",
		AutoSave:  false,
	}

	fks, err := NewFileKeyStore(config)
	if err != nil {
		t.Fatalf("创建文件密钥存储失败: %v", err)
	}
	defer fks.Close()

	// 存储多个密钥
	deviceIDs := []string{"device-001", "device-002", "device-003"}
	testKey := []byte("测试密钥")

	for _, deviceID := range deviceIDs {
		err = fks.StoreKey(deviceID, testKey)
		if err != nil {
			t.Fatalf("存储密钥 %s 失败: %v", deviceID, err)
		}
	}

	// 获取密钥列表
	keys, err := fks.ListKeys()
	if err != nil {
		t.Fatalf("获取密钥列表失败: %v", err)
	}

	// 验证密钥数量
	if len(keys) != len(deviceIDs) {
		t.Errorf("密钥数量不匹配，期望: %d，实际: %d", len(deviceIDs), len(keys))
	}

	// 验证所有设备ID都在列表中
	keyMap := make(map[string]bool)
	for _, key := range keys {
		keyMap[key] = true
	}

	for _, deviceID := range deviceIDs {
		if !keyMap[deviceID] {
			t.Errorf("设备ID %s 不在密钥列表中", deviceID)
		}
	}
}

// TestFileKeyStore_Persistence 测试持久化功能
func TestFileKeyStore_Persistence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "keystore_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storePath := filepath.Join(tempDir, "keystore.json")
	config := KeyStoreConfig{
		StorePath: storePath,
		Password:  "test-password-123",
		AutoSave:  false,
	}

	deviceID := "test-device-003"
	originalKey := []byte("持久化测试密钥")

	// 第一个密钥存储实例
	fks1, err := NewFileKeyStore(config)
	if err != nil {
		t.Fatalf("创建第一个文件密钥存储失败: %v", err)
	}

	// 存储密钥
	err = fks1.StoreKey(deviceID, originalKey)
	if err != nil {
		t.Fatalf("存储密钥失败: %v", err)
	}

	// 关闭第一个实例
	err = fks1.Close()
	if err != nil {
		t.Fatalf("关闭第一个密钥存储失败: %v", err)
	}

	// 创建第二个密钥存储实例（应该加载之前的数据）
	fks2, err := NewFileKeyStore(config)
	if err != nil {
		t.Fatalf("创建第二个文件密钥存储失败: %v", err)
	}
	defer fks2.Close()

	// 获取密钥
	retrievedKey, err := fks2.GetKey(deviceID)
	if err != nil {
		t.Fatalf("从第二个实例获取密钥失败: %v", err)
	}

	// 验证密钥内容
	if !bytes.Equal(originalKey, retrievedKey) {
		t.Error("持久化后获取的密钥与原密钥不匹配")
	}
}

// TestFileKeyStore_AutoSave 测试自动保存功能
func TestFileKeyStore_AutoSave(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "keystore_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := KeyStoreConfig{
		StorePath:    filepath.Join(tempDir, "keystore.json"),
		Password:     "test-password-123",
		AutoSave:     true,
		SaveInterval: 100 * time.Millisecond,
	}

	fks, err := NewFileKeyStore(config)
	if err != nil {
		t.Fatalf("创建文件密钥存储失败: %v", err)
	}
	defer fks.Close()

	deviceID := "test-device-004"
	testKey := []byte("自动保存测试密钥")

	// 存储密钥
	err = fks.StoreKey(deviceID, testKey)
	if err != nil {
		t.Fatalf("存储密钥失败: %v", err)
	}

	// 等待自动保存
	time.Sleep(200 * time.Millisecond)

	// 验证文件已创建
	if _, err := os.Stat(config.StorePath); os.IsNotExist(err) {
		t.Error("自动保存应该创建存储文件")
	}
}

// TestFileKeyStore_KeyInfo 测试密钥信息功能
func TestFileKeyStore_KeyInfo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "keystore_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := KeyStoreConfig{
		StorePath: filepath.Join(tempDir, "keystore.json"),
		Password:  "test-password-123",
		AutoSave:  false,
	}

	fks, err := NewFileKeyStore(config)
	if err != nil {
		t.Fatalf("创建文件密钥存储失败: %v", err)
	}
	defer fks.Close()

	deviceID := "test-device-005"
	testKey := []byte("密钥信息测试")

	// 存储密钥
	err = fks.StoreKey(deviceID, testKey)
	if err != nil {
		t.Fatalf("存储密钥失败: %v", err)
	}

	// 获取密钥信息
	keyInfo, err := fks.GetKeyInfo(deviceID)
	if err != nil {
		t.Fatalf("获取密钥信息失败: %v", err)
	}

	// 验证密钥信息
	if keyInfo.DeviceID != deviceID {
		t.Errorf("设备ID不匹配，期望: %s，实际: %s", deviceID, keyInfo.DeviceID)
	}

	if keyInfo.AccessCount != 0 {
		t.Errorf("初始访问次数应该为0，实际: %d", keyInfo.AccessCount)
	}

	// 访问密钥以增加访问次数
	_, err = fks.GetKey(deviceID)
	if err != nil {
		t.Fatalf("访问密钥失败: %v", err)
	}

	// 再次获取密钥信息
	keyInfo, err = fks.GetKeyInfo(deviceID)
	if err != nil {
		t.Fatalf("再次获取密钥信息失败: %v", err)
	}

	if keyInfo.AccessCount != 1 {
		t.Errorf("访问次数应该为1，实际: %d", keyInfo.AccessCount)
	}
}

// TestFileKeyStore_Metadata 测试元数据功能
func TestFileKeyStore_Metadata(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "keystore_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := KeyStoreConfig{
		StorePath: filepath.Join(tempDir, "keystore.json"),
		Password:  "test-password-123",
		AutoSave:  false,
	}

	fks, err := NewFileKeyStore(config)
	if err != nil {
		t.Fatalf("创建文件密钥存储失败: %v", err)
	}
	defer fks.Close()

	deviceID := "test-device-006"
	testKey := []byte("元数据测试密钥")

	// 存储密钥
	err = fks.StoreKey(deviceID, testKey)
	if err != nil {
		t.Fatalf("存储密钥失败: %v", err)
	}

	// 设置元数据
	metaKey := "device_type"
	metaValue := "smartphone"
	err = fks.SetMetadata(deviceID, metaKey, metaValue)
	if err != nil {
		t.Fatalf("设置元数据失败: %v", err)
	}

	// 获取元数据
	retrievedValue, err := fks.GetMetadata(deviceID, metaKey)
	if err != nil {
		t.Fatalf("获取元数据失败: %v", err)
	}

	if retrievedValue != metaValue {
		t.Errorf("元数据值不匹配，期望: %s，实际: %s", metaValue, retrievedValue)
	}

	// 测试获取不存在的元数据
	_, err = fks.GetMetadata(deviceID, "nonexistent_key")
	if err == nil {
		t.Error("获取不存在的元数据应该返回错误")
	}

	// 测试为不存在的设备设置元数据
	err = fks.SetMetadata("nonexistent_device", metaKey, metaValue)
	if err == nil {
		t.Error("为不存在的设备设置元数据应该返回错误")
	}
}

// TestFileKeyStore_Backup 测试备份功能
func TestFileKeyStore_Backup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "keystore_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := KeyStoreConfig{
		StorePath: filepath.Join(tempDir, "keystore.json"),
		Password:  "test-password-123",
		AutoSave:  false,
	}

	fks, err := NewFileKeyStore(config)
	if err != nil {
		t.Fatalf("创建文件密钥存储失败: %v", err)
	}
	defer fks.Close()

	deviceID := "test-device-007"
	testKey := []byte("备份测试密钥")

	// 存储密钥
	err = fks.StoreKey(deviceID, testKey)
	if err != nil {
		t.Fatalf("存储密钥失败: %v", err)
	}

	// 创建备份
	err = fks.Backup()
	if err != nil {
		t.Fatalf("创建备份失败: %v", err)
	}

	// 验证备份文件存在
	backupPattern := config.StorePath + ".backup.*"
	matches, err := filepath.Glob(backupPattern)
	if err != nil {
		t.Fatalf("查找备份文件失败: %v", err)
	}

	if len(matches) == 0 {
		t.Error("应该创建备份文件")
	}

	// 删除原密钥
	err = fks.DeleteKey(deviceID)
	if err != nil {
		t.Fatalf("删除原密钥失败: %v", err)
	}

	// 从备份恢复
	if len(matches) > 0 {
		err = fks.Restore(matches[0])
		if err != nil {
			t.Fatalf("从备份恢复失败: %v", err)
		}

		// 验证密钥已恢复
		retrievedKey, err := fks.GetKey(deviceID)
		if err != nil {
			t.Fatalf("恢复后获取密钥失败: %v", err)
		}

		if !bytes.Equal(testKey, retrievedKey) {
			t.Error("恢复的密钥与原密钥不匹配")
		}
	}
}

// TestFileKeyStore_Stats 测试统计信息功能
func TestFileKeyStore_Stats(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "keystore_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := KeyStoreConfig{
		StorePath:    filepath.Join(tempDir, "keystore.json"),
		Password:     "test-password-123",
		AutoSave:     true,
		SaveInterval: 1 * time.Second,
	}

	fks, err := NewFileKeyStore(config)
	if err != nil {
		t.Fatalf("创建文件密钥存储失败: %v", err)
	}
	defer fks.Close()

	// 存储一些密钥
	deviceIDs := []string{"device-001", "device-002", "device-003"}
	testKey := []byte("统计测试密钥")

	for _, deviceID := range deviceIDs {
		err = fks.StoreKey(deviceID, testKey)
		if err != nil {
			t.Fatalf("存储密钥 %s 失败: %v", deviceID, err)
		}
	}

	// 访问一些密钥
	for i := 0; i < 2; i++ {
		_, err = fks.GetKey(deviceIDs[0])
		if err != nil {
			t.Fatalf("访问密钥失败: %v", err)
		}
	}

	// 获取统计信息
	stats := fks.GetStats()

	// 验证统计信息
	if totalKeys, ok := stats["total_keys"].(int); !ok || totalKeys != len(deviceIDs) {
		t.Errorf("总密钥数不正确，期望: %d，实际: %v", len(deviceIDs), stats["total_keys"])
	}

	if totalAccess, ok := stats["total_access"].(int); !ok || totalAccess != 2 {
		t.Errorf("总访问次数不正确，期望: 2，实际: %v", stats["total_access"])
	}

	if storePath, ok := stats["store_path"].(string); !ok || storePath != config.StorePath {
		t.Errorf("存储路径不正确，期望: %s，实际: %v", config.StorePath, stats["store_path"])
	}

	if autoSave, ok := stats["auto_save"].(bool); !ok || autoSave != config.AutoSave {
		t.Errorf("自动保存设置不正确，期望: %t，实际: %v", config.AutoSave, stats["auto_save"])
	}
}

// TestFileKeyStore_CleanupOldKeys 测试清理旧密钥功能
func TestFileKeyStore_CleanupOldKeys(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "keystore_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := KeyStoreConfig{
		StorePath: filepath.Join(tempDir, "keystore.json"),
		Password:  "test-password-123",
		AutoSave:  false,
	}

	fks, err := NewFileKeyStore(config)
	if err != nil {
		t.Fatalf("创建文件密钥存储失败: %v", err)
	}
	defer fks.Close()

	// 存储一些密钥
	deviceIDs := []string{"old-device-001", "old-device-002", "new-device-001"}
	testKey := []byte("清理测试密钥")

	for _, deviceID := range deviceIDs {
		err = fks.StoreKey(deviceID, testKey)
		if err != nil {
			t.Fatalf("存储密钥 %s 失败: %v", deviceID, err)
		}
	}

	// 手动修改前两个密钥的创建时间为旧时间
	fks.mu.Lock()
	oldTime := time.Now().Add(-2 * time.Hour)
	for i := 0; i < 2; i++ {
		if key, exists := fks.keys[deviceIDs[i]]; exists {
			key.CreatedAt = oldTime
		}
	}
	fks.mu.Unlock()

	// 清理1小时前的密钥
	cleaned, err := fks.CleanupOldKeys(1 * time.Hour)
	if err != nil {
		t.Fatalf("清理旧密钥失败: %v", err)
	}

	// 验证清理结果
	if cleaned != 2 {
		t.Errorf("期望清理2个旧密钥，实际清理了%d个", cleaned)
	}

	// 验证新密钥仍然存在
	_, err = fks.GetKey(deviceIDs[2])
	if err != nil {
		t.Error("新密钥不应该被清理")
	}

	// 验证旧密钥已被清理
	for i := 0; i < 2; i++ {
		_, err = fks.GetKey(deviceIDs[i])
		if err == nil {
			t.Errorf("旧密钥 %s 应该已被清理", deviceIDs[i])
		}
	}
}

// TestMemoryKeyStore_BasicOperations 测试内存密钥存储的基本操作
func TestMemoryKeyStore_BasicOperations(t *testing.T) {
	mks := NewMemoryKeyStore()
	deviceID := "test-device-memory"
	testKey := []byte("内存存储测试密钥")

	// 测试存储密钥
	err := mks.StoreKey(deviceID, testKey)
	if err != nil {
		t.Fatalf("存储密钥失败: %v", err)
	}

	// 测试获取密钥
	retrievedKey, err := mks.GetKey(deviceID)
	if err != nil {
		t.Fatalf("获取密钥失败: %v", err)
	}

	if !bytes.Equal(testKey, retrievedKey) {
		t.Error("获取的密钥与原密钥不匹配")
	}

	// 测试列出密钥
	keys, err := mks.ListKeys()
	if err != nil {
		t.Fatalf("列出密钥失败: %v", err)
	}

	if len(keys) != 1 || keys[0] != deviceID {
		t.Error("密钥列表不正确")
	}

	// 测试删除密钥
	err = mks.DeleteKey(deviceID)
	if err != nil {
		t.Fatalf("删除密钥失败: %v", err)
	}

	// 验证密钥已删除
	_, err = mks.GetKey(deviceID)
	if err == nil {
		t.Error("删除后仍能获取密钥")
	}
}
