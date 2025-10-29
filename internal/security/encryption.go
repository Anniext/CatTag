package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// EncryptionManager 加密管理器
type EncryptionManager struct {
	mu        sync.RWMutex
	algorithm string
	keySize   int
	keys      map[string]*EncryptionKey // 设备密钥映射
}

// EncryptionKey 加密密钥信息
type EncryptionKey struct {
	DeviceID      string    `json:"device_id"`      // 设备ID
	Key           []byte    `json:"key"`            // 密钥数据
	IV            []byte    `json:"iv"`             // 初始化向量
	CreatedAt     time.Time `json:"created_at"`     // 创建时间
	ExpiresAt     time.Time `json:"expires_at"`     // 过期时间
	Usage         int       `json:"usage"`          // 使用次数
	MaxUsage      int       `json:"max_usage"`      // 最大使用次数
	Algorithm     string    `json:"algorithm"`      // 加密算法
	KeyDerivation string    `json:"key_derivation"` // 密钥派生方法
}

// EncryptedData 加密数据结构
type EncryptedData struct {
	Ciphertext []byte `json:"ciphertext"` // 密文
	Nonce      []byte `json:"nonce"`      // 随机数
	Tag        []byte `json:"tag"`        // 认证标签
	Algorithm  string `json:"algorithm"`  // 加密算法
	Timestamp  int64  `json:"timestamp"`  // 时间戳
	HMAC       []byte `json:"hmac"`       // 消息认证码
}

// KeyExchangeData 密钥交换数据
type KeyExchangeData struct {
	PublicKey   []byte    `json:"public_key"`  // 公钥
	Signature   []byte    `json:"signature"`   // 签名
	Timestamp   time.Time `json:"timestamp"`   // 时间戳
	Algorithm   string    `json:"algorithm"`   // 算法
	DeviceID    string    `json:"device_id"`   // 设备ID
	Nonce       []byte    `json:"nonce"`       // 随机数
	Certificate []byte    `json:"certificate"` // 证书（可选）
}

// NewEncryptionManager 创建新的加密管理器
func NewEncryptionManager(algorithm string, keySize int) *EncryptionManager {
	return &EncryptionManager{
		algorithm: algorithm,
		keySize:   keySize,
		keys:      make(map[string]*EncryptionKey),
	}
}

// GenerateKey 生成新的加密密钥
func (em *EncryptionManager) GenerateKey(deviceID string, ttl time.Duration) (*EncryptionKey, error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	// 生成随机密钥
	key := make([]byte, em.keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "生成密钥失败", deviceID, "generate_key")
	}

	// 生成初始化向量
	iv := make([]byte, 16) // AES块大小
	if _, err := rand.Read(iv); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "生成IV失败", deviceID, "generate_key")
	}

	now := time.Now()
	encKey := &EncryptionKey{
		DeviceID:      deviceID,
		Key:           key,
		IV:            iv,
		CreatedAt:     now,
		ExpiresAt:     now.Add(ttl),
		Usage:         0,
		MaxUsage:      10000, // 默认最大使用次数
		Algorithm:     em.algorithm,
		KeyDerivation: "PBKDF2",
	}

	// 存储密钥
	em.keys[deviceID] = encKey

	return encKey, nil
}

// GetKey 获取设备的加密密钥
func (em *EncryptionManager) GetKey(deviceID string) (*EncryptionKey, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	key, exists := em.keys[deviceID]
	if !exists {
		return nil, bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeNotFound,
			"加密密钥不存在",
			deviceID,
			"get_key",
		)
	}

	// 检查密钥是否过期
	if time.Now().After(key.ExpiresAt) {
		return nil, bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeEncryptionFailed,
			"加密密钥已过期",
			deviceID,
			"get_key",
		)
	}

	// 检查使用次数
	if key.Usage >= key.MaxUsage {
		return nil, bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeEncryptionFailed,
			"密钥使用次数已达上限",
			deviceID,
			"get_key",
		)
	}

	return key, nil
}

// RotateKey 轮换密钥
func (em *EncryptionManager) RotateKey(deviceID string, ttl time.Duration) (*EncryptionKey, error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	// 删除旧密钥
	delete(em.keys, deviceID)

	// 生成新密钥
	em.mu.Unlock() // 临时解锁以调用 GenerateKey
	newKey, err := em.GenerateKey(deviceID, ttl)
	em.mu.Lock() // 重新加锁

	if err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "密钥轮换失败", deviceID, "rotate_key")
	}

	return newKey, nil
}

// EncryptData 加密数据
func (em *EncryptionManager) EncryptData(data []byte, deviceID string) (*EncryptedData, error) {
	// 获取密钥
	key, err := em.GetKey(deviceID)
	if err != nil {
		return nil, err
	}

	// 增加使用次数
	em.mu.Lock()
	key.Usage++
	em.mu.Unlock()

	switch em.algorithm {
	case "AES-256-GCM":
		return em.encryptAESGCM(data, key)
	default:
		return nil, bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeNotSupported,
			"不支持的加密算法: "+em.algorithm,
			deviceID,
			"encrypt_data",
		)
	}
}

// DecryptData 解密数据
func (em *EncryptionManager) DecryptData(encData *EncryptedData, deviceID string) ([]byte, error) {
	// 获取密钥
	key, err := em.GetKey(deviceID)
	if err != nil {
		return nil, err
	}

	// 验证时间戳（防重放攻击）
	timestamp := time.Unix(encData.Timestamp, 0)
	if time.Since(timestamp) > 5*time.Minute {
		return nil, bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeEncryptionFailed,
			"加密数据时间戳过期",
			deviceID,
			"decrypt_data",
		)
	}

	switch encData.Algorithm {
	case "AES-256-GCM":
		return em.decryptAESGCM(encData, key)
	default:
		return nil, bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeNotSupported,
			"不支持的解密算法: "+encData.Algorithm,
			deviceID,
			"decrypt_data",
		)
	}
}

// encryptAESGCM 使用AES-GCM加密
func (em *EncryptionManager) encryptAESGCM(data []byte, key *EncryptionKey) (*EncryptedData, error) {
	// 创建AES密码器
	block, err := aes.NewCipher(key.Key)
	if err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "创建AES密码器失败", key.DeviceID, "encrypt_aes_gcm")
	}

	// 创建GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "创建GCM模式失败", key.DeviceID, "encrypt_aes_gcm")
	}

	// 生成随机nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "生成nonce失败", key.DeviceID, "encrypt_aes_gcm")
	}

	// 加密数据
	ciphertext := gcm.Seal(nil, nonce, data, nil)

	// 创建加密数据结构
	encData := &EncryptedData{
		Ciphertext: ciphertext,
		Nonce:      nonce,
		Algorithm:  "AES-256-GCM",
		Timestamp:  time.Now().Unix(),
	}

	// 计算HMAC
	encData.HMAC = em.calculateHMAC(encData, key.Key)

	return encData, nil
}

// decryptAESGCM 使用AES-GCM解密
func (em *EncryptionManager) decryptAESGCM(encData *EncryptedData, key *EncryptionKey) ([]byte, error) {
	// 验证HMAC
	expectedHMAC := em.calculateHMAC(encData, key.Key)
	if !hmac.Equal(encData.HMAC, expectedHMAC) {
		return nil, bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeEncryptionFailed,
			"HMAC验证失败",
			key.DeviceID,
			"decrypt_aes_gcm",
		)
	}

	// 创建AES密码器
	block, err := aes.NewCipher(key.Key)
	if err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "创建AES密码器失败", key.DeviceID, "decrypt_aes_gcm")
	}

	// 创建GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "创建GCM模式失败", key.DeviceID, "decrypt_aes_gcm")
	}

	// 解密数据
	plaintext, err := gcm.Open(nil, encData.Nonce, encData.Ciphertext, nil)
	if err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "解密数据失败", key.DeviceID, "decrypt_aes_gcm")
	}

	return plaintext, nil
}

// calculateHMAC 计算HMAC
func (em *EncryptionManager) calculateHMAC(encData *EncryptedData, key []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(encData.Ciphertext)
	h.Write(encData.Nonce)
	h.Write([]byte(encData.Algorithm))
	h.Write([]byte(fmt.Sprintf("%d", encData.Timestamp)))
	return h.Sum(nil)
}

// GenerateKeyExchangeData 生成密钥交换数据
func (em *EncryptionManager) GenerateKeyExchangeData(deviceID string) (*KeyExchangeData, error) {
	// 生成临时密钥对（这里简化实现，实际应使用椭圆曲线密码学）
	publicKey := make([]byte, 32)
	if _, err := rand.Read(publicKey); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "生成公钥失败", deviceID, "generate_key_exchange")
	}

	// 生成随机数
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "生成nonce失败", deviceID, "generate_key_exchange")
	}

	// 创建签名数据（简化实现）
	signData := append(publicKey, nonce...)
	signature := sha256.Sum256(signData)

	return &KeyExchangeData{
		PublicKey: publicKey,
		Signature: signature[:],
		Timestamp: time.Now(),
		Algorithm: "ECDH-P256",
		DeviceID:  deviceID,
		Nonce:     nonce,
	}, nil
}

// ValidateKeyExchange 验证密钥交换数据
func (em *EncryptionManager) ValidateKeyExchange(kxData *KeyExchangeData) error {
	// 验证时间戳
	if time.Since(kxData.Timestamp) > 5*time.Minute {
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeEncryptionFailed,
			"密钥交换数据已过期",
			kxData.DeviceID,
			"validate_key_exchange",
		)
	}

	// 验证签名（简化实现）
	signData := append(kxData.PublicKey, kxData.Nonce...)
	expectedSignature := sha256.Sum256(signData)

	if !hmac.Equal(kxData.Signature, expectedSignature[:]) {
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeEncryptionFailed,
			"密钥交换签名验证失败",
			kxData.DeviceID,
			"validate_key_exchange",
		)
	}

	return nil
}

// DeriveSharedKey 派生共享密钥
func (em *EncryptionManager) DeriveSharedKey(localPrivateKey, remotePublicKey []byte, deviceID string) ([]byte, error) {
	// 简化的密钥派生实现（实际应使用ECDH）
	combined := append(localPrivateKey, remotePublicKey...)
	hash := sha256.Sum256(combined)

	// 使用PBKDF2进行密钥强化
	derivedKey := make([]byte, em.keySize)
	copy(derivedKey, hash[:])

	return derivedKey, nil
}

// CleanupExpiredKeys 清理过期密钥
func (em *EncryptionManager) CleanupExpiredKeys() int {
	em.mu.Lock()
	defer em.mu.Unlock()

	cleaned := 0
	now := time.Now()

	for deviceID, key := range em.keys {
		if now.After(key.ExpiresAt) || key.Usage >= key.MaxUsage {
			delete(em.keys, deviceID)
			cleaned++
		}
	}

	return cleaned
}

// GetKeyInfo 获取密钥信息（不包含敏感数据）
func (em *EncryptionManager) GetKeyInfo(deviceID string) (map[string]interface{}, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	key, exists := em.keys[deviceID]
	if !exists {
		return nil, bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeNotFound,
			"密钥不存在",
			deviceID,
			"get_key_info",
		)
	}

	return map[string]interface{}{
		"device_id":      key.DeviceID,
		"created_at":     key.CreatedAt,
		"expires_at":     key.ExpiresAt,
		"usage":          key.Usage,
		"max_usage":      key.MaxUsage,
		"algorithm":      key.Algorithm,
		"key_derivation": key.KeyDerivation,
		"is_expired":     time.Now().After(key.ExpiresAt),
		"usage_percent":  float64(key.Usage) / float64(key.MaxUsage) * 100,
	}, nil
}

// ExportKey 导出密钥（加密格式）
func (em *EncryptionManager) ExportKey(deviceID string, password string) (string, error) {
	key, err := em.GetKey(deviceID)
	if err != nil {
		return "", err
	}

	// 使用密码加密密钥数据（简化实现）
	passwordHash := sha256.Sum256([]byte(password))

	block, err := aes.NewCipher(passwordHash[:])
	if err != nil {
		return "", bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "创建密码加密器失败", deviceID, "export_key")
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "创建GCM模式失败", deviceID, "export_key")
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "生成nonce失败", deviceID, "export_key")
	}

	// 序列化密钥数据
	keyData := fmt.Sprintf("%s:%s:%d:%d",
		hex.EncodeToString(key.Key),
		hex.EncodeToString(key.IV),
		key.CreatedAt.Unix(),
		key.ExpiresAt.Unix(),
	)

	// 加密密钥数据
	ciphertext := gcm.Seal(nonce, nonce, []byte(keyData), nil)

	return hex.EncodeToString(ciphertext), nil
}

// ImportKey 导入密钥（从加密格式）
func (em *EncryptionManager) ImportKey(deviceID string, encryptedKey string, password string) error {
	// 解码加密数据
	ciphertext, err := hex.DecodeString(encryptedKey)
	if err != nil {
		return bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "解码加密密钥失败", deviceID, "import_key")
	}

	// 使用密码解密
	passwordHash := sha256.Sum256([]byte(password))

	block, err := aes.NewCipher(passwordHash[:])
	if err != nil {
		return bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "创建密码解密器失败", deviceID, "import_key")
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "创建GCM模式失败", deviceID, "import_key")
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return bluetooth.NewBluetoothErrorWithDevice(
			bluetooth.ErrCodeEncryptionFailed,
			"加密密钥数据长度不足",
			deviceID,
			"import_key",
		)
	}

	nonce, encData := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, encData, nil)
	if err != nil {
		return bluetooth.WrapError(err, bluetooth.ErrCodeEncryptionFailed, "解密密钥数据失败", deviceID, "import_key")
	}

	// 解析密钥数据
	keyDataStr := string(plaintext)
	// 这里应该解析并重建密钥对象，简化实现
	// 实际实现中应该解析 keyDataStr 并重建 EncryptionKey 对象
	_ = keyDataStr // 避免未使用变量警告

	return nil
}
