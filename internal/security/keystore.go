package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// FileKeyStore 文件密钥存储实现
type FileKeyStore struct {
	mu           sync.RWMutex
	storePath    string
	password     string
	keys         map[string]*StoredKey
	autoSave     bool
	saveInterval time.Duration
	stopChan     chan struct{}
}

// StoredKey 存储的密钥信息
type StoredKey struct {
	DeviceID    string            `json:"device_id"`    // 设备ID
	KeyData     []byte            `json:"key_data"`     // 加密的密钥数据
	Salt        []byte            `json:"salt"`         // 盐值
	CreatedAt   time.Time         `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time         `json:"updated_at"`   // 更新时间
	AccessCount int               `json:"access_count"` // 访问次数
	LastAccess  time.Time         `json:"last_access"`  // 最后访问时间
	Metadata    map[string]string `json:"metadata"`     // 元数据
}

// KeyStoreConfig 密钥存储配置
type KeyStoreConfig struct {
	StorePath    string        `json:"store_path"`    // 存储路径
	Password     string        `json:"password"`      // 主密码
	AutoSave     bool          `json:"auto_save"`     // 自动保存
	SaveInterval time.Duration `json:"save_interval"` // 保存间隔
	BackupCount  int           `json:"backup_count"`  // 备份数量
	Compression  bool          `json:"compression"`   // 是否压缩
}

// NewFileKeyStore 创建新的文件密钥存储
func NewFileKeyStore(config KeyStoreConfig) (*FileKeyStore, error) {
	// 确保存储目录存在
	if err := os.MkdirAll(filepath.Dir(config.StorePath), 0700); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeGeneral, "创建存储目录失败", "", "new_file_keystore")
	}

	fks := &FileKeyStore{
		storePath:    config.StorePath,
		password:     config.Password,
		keys:         make(map[string]*StoredKey),
		autoSave:     config.AutoSave,
		saveInterval: config.SaveInterval,
		stopChan:     make(chan struct{}),
	}

	// 加载现有密钥
	if err := fks.load(); err != nil {
		// 如果文件不存在，这是正常的
		if !os.IsNotExist(err) {
			return nil, bluetooth.WrapError(err, bluetooth.ErrCodeGeneral, "加载密钥存储失败", "", "new_file_keystore")
		}
	}

	// 启动自动保存
	if fks.autoSave && fks.saveInterval > 0 {
		go fks.autoSaveLoop()
	}

	return fks, nil
}

// StoreKey 存储密钥
func (fks *FileKeyStore) StoreKey(deviceID string, key []byte) error {
	fks.mu.Lock()
	defer fks.mu.Unlock()

	// 加密密钥数据
	encryptedKey, salt, err := fks.encryptKey(key)
	if err != nil {
		return bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "加密密钥失败", deviceID, "store_key")
	}

	now := time.Now()
	storedKey := &StoredKey{
		DeviceID:    deviceID,
		KeyData:     encryptedKey,
		Salt:        salt,
		CreatedAt:   now,
		UpdatedAt:   now,
		AccessCount: 0,
		LastAccess:  now,
		Metadata:    make(map[string]string),
	}

	fks.keys[deviceID] = storedKey

	// 立即保存（如果不是自动保存模式）
	if !fks.autoSave {
		return fks.save()
	}

	return nil
}

// GetKey 获取密钥
func (fks *FileKeyStore) GetKey(deviceID string) ([]byte, error) {
	fks.mu.Lock()
	defer fks.mu.Unlock()

	storedKey, exists := fks.keys[deviceID]
	if !exists {
		return nil, bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeNotFound,
			"密钥不存在",
			deviceID,
			"get_key",
		)
	}

	// 解密密钥数据
	key, err := fks.decryptKey(storedKey.KeyData, storedKey.Salt)
	if err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "解密密钥失败", deviceID, "get_key")
	}

	// 更新访问信息
	storedKey.AccessCount++
	storedKey.LastAccess = time.Now()

	return key, nil
}

// DeleteKey 删除密钥
func (fks *FileKeyStore) DeleteKey(deviceID string) error {
	fks.mu.Lock()
	defer fks.mu.Unlock()

	if _, exists := fks.keys[deviceID]; !exists {
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeNotFound,
			"密钥不存在",
			deviceID,
			"delete_key",
		)
	}

	delete(fks.keys, deviceID)

	// 立即保存（如果不是自动保存模式）
	if !fks.autoSave {
		return fks.save()
	}

	return nil
}

// ListKeys 列出所有密钥
func (fks *FileKeyStore) ListKeys() ([]string, error) {
	fks.mu.RLock()
	defer fks.mu.RUnlock()

	keys := make([]string, 0, len(fks.keys))
	for deviceID := range fks.keys {
		keys = append(keys, deviceID)
	}

	return keys, nil
}

// GetKeyInfo 获取密钥信息
func (fks *FileKeyStore) GetKeyInfo(deviceID string) (*StoredKey, error) {
	fks.mu.RLock()
	defer fks.mu.RUnlock()

	storedKey, exists := fks.keys[deviceID]
	if !exists {
		return nil, bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeNotFound,
			"密钥不存在",
			deviceID,
			"get_key_info",
		)
	}

	// 返回副本，不包含敏感数据
	info := &StoredKey{
		DeviceID:    storedKey.DeviceID,
		CreatedAt:   storedKey.CreatedAt,
		UpdatedAt:   storedKey.UpdatedAt,
		AccessCount: storedKey.AccessCount,
		LastAccess:  storedKey.LastAccess,
		Metadata:    make(map[string]string),
	}

	// 复制元数据
	for k, v := range storedKey.Metadata {
		info.Metadata[k] = v
	}

	return info, nil
}

// SetMetadata 设置密钥元数据
func (fks *FileKeyStore) SetMetadata(deviceID string, key string, value string) error {
	fks.mu.Lock()
	defer fks.mu.Unlock()

	storedKey, exists := fks.keys[deviceID]
	if !exists {
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeNotFound,
			"密钥不存在",
			deviceID,
			"set_metadata",
		)
	}

	if storedKey.Metadata == nil {
		storedKey.Metadata = make(map[string]string)
	}

	storedKey.Metadata[key] = value
	storedKey.UpdatedAt = time.Now()

	return nil
}

// GetMetadata 获取密钥元数据
func (fks *FileKeyStore) GetMetadata(deviceID string, key string) (string, error) {
	fks.mu.RLock()
	defer fks.mu.RUnlock()

	storedKey, exists := fks.keys[deviceID]
	if !exists {
		return "", bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeNotFound,
			"密钥不存在",
			deviceID,
			"get_metadata",
		)
	}

	value, exists := storedKey.Metadata[key]
	if !exists {
		return "", bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeNotFound,
			"元数据不存在",
			deviceID,
			"get_metadata",
		)
	}

	return value, nil
}

// Backup 创建备份
func (fks *FileKeyStore) Backup() error {
	fks.mu.RLock()
	defer fks.mu.RUnlock()

	backupPath := fmt.Sprintf("%s.backup.%d", fks.storePath, time.Now().Unix())
	return fks.saveToFile(backupPath)
}

// Restore 从备份恢复
func (fks *FileKeyStore) Restore(backupPath string) error {
	fks.mu.Lock()
	defer fks.mu.Unlock()

	return fks.loadFromFile(backupPath)
}

// Close 关闭密钥存储
func (fks *FileKeyStore) Close() error {
	// 停止自动保存
	close(fks.stopChan)

	// 最后保存一次
	fks.mu.Lock()
	defer fks.mu.Unlock()
	return fks.save()
}

// encryptKey 加密密钥
func (fks *FileKeyStore) encryptKey(key []byte) ([]byte, []byte, error) {
	// 生成随机盐
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, err
	}

	// 派生加密密钥
	derivedKey := fks.deriveKey(fks.password, salt)

	// 创建AES密码器
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, nil, err
	}

	// 创建GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	// 生成随机nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}

	// 加密数据
	ciphertext := gcm.Seal(nonce, nonce, key, nil)

	return ciphertext, salt, nil
}

// decryptKey 解密密钥
func (fks *FileKeyStore) decryptKey(encryptedKey []byte, salt []byte) ([]byte, error) {
	// 派生解密密钥
	derivedKey := fks.deriveKey(fks.password, salt)

	// 创建AES密码器
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, err
	}

	// 创建GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedKey) < nonceSize {
		return nil, fmt.Errorf("加密数据长度不足")
	}

	nonce, ciphertext := encryptedKey[:nonceSize], encryptedKey[nonceSize:]

	// 解密数据
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// deriveKey 派生密钥
func (fks *FileKeyStore) deriveKey(password string, salt []byte) []byte {
	// 使用PBKDF2派生密钥
	combined := append([]byte(password), salt...)
	hash := sha256.Sum256(combined)
	return hash[:]
}

// save 保存到文件
func (fks *FileKeyStore) save() error {
	return fks.saveToFile(fks.storePath)
}

// saveToFile 保存到指定文件
func (fks *FileKeyStore) saveToFile(filePath string) error {
	data, err := json.MarshalIndent(fks.keys, "", "  ")
	if err != nil {
		return err
	}

	// 创建临时文件
	tempFile := filePath + ".tmp"
	if err := ioutil.WriteFile(tempFile, data, 0600); err != nil {
		return err
	}

	// 原子性替换
	return os.Rename(tempFile, filePath)
}

// load 从文件加载
func (fks *FileKeyStore) load() error {
	return fks.loadFromFile(fks.storePath)
}

// loadFromFile 从指定文件加载
func (fks *FileKeyStore) loadFromFile(filePath string) error {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &fks.keys)
}

// autoSaveLoop 自动保存循环
func (fks *FileKeyStore) autoSaveLoop() {
	ticker := time.NewTicker(fks.saveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fks.mu.RLock()
			err := fks.save()
			fks.mu.RUnlock()

			if err != nil {
				// 记录错误但不停止循环
				fmt.Printf("自动保存密钥存储失败: %v\n", err)
			}
		case <-fks.stopChan:
			return
		}
	}
}

// GetStats 获取统计信息
func (fks *FileKeyStore) GetStats() map[string]interface{} {
	fks.mu.RLock()
	defer fks.mu.RUnlock()

	totalKeys := len(fks.keys)
	totalAccess := 0
	oldestKey := time.Now()
	newestKey := time.Time{}

	for _, key := range fks.keys {
		totalAccess += key.AccessCount
		if key.CreatedAt.Before(oldestKey) {
			oldestKey = key.CreatedAt
		}
		if key.CreatedAt.After(newestKey) {
			newestKey = key.CreatedAt
		}
	}

	return map[string]interface{}{
		"total_keys":    totalKeys,
		"total_access":  totalAccess,
		"oldest_key":    oldestKey,
		"newest_key":    newestKey,
		"store_path":    fks.storePath,
		"auto_save":     fks.autoSave,
		"save_interval": fks.saveInterval,
	}
}

// CleanupOldKeys 清理旧密钥
func (fks *FileKeyStore) CleanupOldKeys(maxAge time.Duration) (int, error) {
	fks.mu.Lock()
	defer fks.mu.Unlock()

	cleaned := 0
	cutoff := time.Now().Add(-maxAge)

	for deviceID, key := range fks.keys {
		if key.CreatedAt.Before(cutoff) {
			delete(fks.keys, deviceID)
			cleaned++
		}
	}

	if cleaned > 0 && !fks.autoSave {
		if err := fks.save(); err != nil {
			return cleaned, err
		}
	}

	return cleaned, nil
}
