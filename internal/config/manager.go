package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// ConfigManager 配置管理器接口
type ConfigManager interface {
	// LoadConfig 从文件加载配置
	LoadConfig(path string) (*bluetooth.BluetoothConfig, error)
	// SaveConfig 保存配置到文件
	SaveConfig(path string, config *bluetooth.BluetoothConfig) error
	// ValidateConfig 验证配置
	ValidateConfig(config *bluetooth.BluetoothConfig) error
	// WatchConfig 监控配置文件变化
	WatchConfig(ctx context.Context, path string) (<-chan *bluetooth.BluetoothConfig, error)
	// RegisterCallback 注册配置变更回调
	RegisterCallback(callback ConfigCallback)
	// UnregisterCallback 取消注册配置变更回调
	UnregisterCallback(callback ConfigCallback)
	// GetCurrentConfig 获取当前配置
	GetCurrentConfig() *bluetooth.BluetoothConfig
	// UpdateConfig 更新配置
	UpdateConfig(config *bluetooth.BluetoothConfig) error
}

// ConfigCallback 配置变更回调函数类型
type ConfigCallback func(oldConfig, newConfig *bluetooth.BluetoothConfig) error

// ConfigManagerImpl 配置管理器实现
type ConfigManagerImpl struct {
	mu            sync.RWMutex                  // 读写锁
	currentConfig *bluetooth.BluetoothConfig    // 当前配置
	callbacks     []ConfigCallback              // 配置变更回调列表
	watchers      map[string]context.CancelFunc // 文件监控器
	logger        Logger                        // 日志记录器
}

// Logger 日志接口
type Logger interface {
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// NewConfigManager 创建新的配置管理器
func NewConfigManager(logger Logger) ConfigManager {
	if logger == nil {
		logger = &defaultLogger{}
	}

	return &ConfigManagerImpl{
		currentConfig: nil,
		callbacks:     make([]ConfigCallback, 0),
		watchers:      make(map[string]context.CancelFunc),
		logger:        logger,
	}
}

// LoadConfig 从文件加载配置
func (cm *ConfigManagerImpl) LoadConfig(path string) (*bluetooth.BluetoothConfig, error) {
	cm.logger.Info("开始加载配置文件: %s", path)

	// 检查文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cm.logger.Warn("配置文件不存在，使用默认配置: %s", path)
		defaultConfig := bluetooth.DefaultConfig()
		return &defaultConfig, nil
	}

	// 打开配置文件
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("无法打开配置文件 %s: %w", path, err)
	}
	defer file.Close()

	// 读取文件内容
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("无法读取配置文件 %s: %w", path, err)
	}

	// 解析JSON配置
	var config bluetooth.BluetoothConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("无法解析配置文件 %s: %w", path, err)
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	cm.logger.Info("配置文件加载成功: %s", path)
	return &config, nil
}

// SaveConfig 保存配置到文件
func (cm *ConfigManagerImpl) SaveConfig(path string, config *bluetooth.BluetoothConfig) error {
	cm.logger.Info("开始保存配置文件: %s", path)

	// 验证配置
	if err := config.Validate(); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	// 创建目录（如果不存在）
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("无法创建配置目录 %s: %w", dir, err)
	}

	// 序列化配置为JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("无法序列化配置: %w", err)
	}

	// 写入临时文件
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("无法写入临时配置文件 %s: %w", tempPath, err)
	}

	// 原子性替换配置文件
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath) // 清理临时文件
		return fmt.Errorf("无法替换配置文件 %s: %w", path, err)
	}

	cm.logger.Info("配置文件保存成功: %s", path)
	return nil
}

// ValidateConfig 验证配置
func (cm *ConfigManagerImpl) ValidateConfig(config *bluetooth.BluetoothConfig) error {
	if config == nil {
		return fmt.Errorf("配置不能为空")
	}

	return config.Validate()
}

// WatchConfig 监控配置文件变化
func (cm *ConfigManagerImpl) WatchConfig(ctx context.Context, path string) (<-chan *bluetooth.BluetoothConfig, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 如果已经在监控该文件，先停止之前的监控
	if cancel, exists := cm.watchers[path]; exists {
		cancel()
	}

	// 创建配置变更通道
	configChan := make(chan *bluetooth.BluetoothConfig, 1)

	// 创建监控上下文
	watchCtx, cancel := context.WithCancel(ctx)
	cm.watchers[path] = cancel

	// 启动文件监控goroutine
	go cm.watchFile(watchCtx, path, configChan)

	cm.logger.Info("开始监控配置文件: %s", path)
	return configChan, nil
}

// watchFile 监控文件变化的内部方法
func (cm *ConfigManagerImpl) watchFile(ctx context.Context, path string, configChan chan<- *bluetooth.BluetoothConfig) {
	defer close(configChan)

	// 获取初始文件信息
	var lastModTime time.Time
	if stat, err := os.Stat(path); err == nil {
		lastModTime = stat.ModTime()
	}

	// 定期检查文件变化
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			cm.logger.Info("停止监控配置文件: %s", path)
			return
		case <-ticker.C:
			// 检查文件是否有变化
			stat, err := os.Stat(path)
			if err != nil {
				if !os.IsNotExist(err) {
					cm.logger.Error("检查配置文件状态失败: %s, 错误: %v", path, err)
				}
				continue
			}

			// 如果文件修改时间发生变化
			if stat.ModTime().After(lastModTime) {
				lastModTime = stat.ModTime()
				cm.logger.Info("检测到配置文件变化: %s", path)

				// 重新加载配置
				newConfig, err := cm.LoadConfig(path)
				if err != nil {
					cm.logger.Error("重新加载配置失败: %v", err)
					continue
				}

				// 发送新配置到通道
				select {
				case configChan <- newConfig:
					cm.logger.Info("配置变更通知已发送")
				case <-ctx.Done():
					return
				default:
					cm.logger.Warn("配置变更通道已满，跳过本次通知")
				}
			}
		}
	}
}

// RegisterCallback 注册配置变更回调
func (cm *ConfigManagerImpl) RegisterCallback(callback ConfigCallback) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.callbacks = append(cm.callbacks, callback)
	cm.logger.Debug("注册配置变更回调，当前回调数量: %d", len(cm.callbacks))
}

// UnregisterCallback 取消注册配置变更回调
func (cm *ConfigManagerImpl) UnregisterCallback(callback ConfigCallback) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 查找并移除回调
	for i, cb := range cm.callbacks {
		// 比较函数指针（注意：这种比较在Go中有限制）
		if fmt.Sprintf("%p", cb) == fmt.Sprintf("%p", callback) {
			cm.callbacks = append(cm.callbacks[:i], cm.callbacks[i+1:]...)
			cm.logger.Debug("取消注册配置变更回调，当前回调数量: %d", len(cm.callbacks))
			return
		}
	}

	cm.logger.Warn("未找到要取消注册的配置变更回调")
}

// GetCurrentConfig 获取当前配置
func (cm *ConfigManagerImpl) GetCurrentConfig() *bluetooth.BluetoothConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.currentConfig == nil {
		defaultConfig := bluetooth.DefaultConfig()
		return &defaultConfig
	}

	// 返回配置的副本，避免外部修改
	configCopy := *cm.currentConfig
	return &configCopy
}

// UpdateConfig 更新配置
func (cm *ConfigManagerImpl) UpdateConfig(config *bluetooth.BluetoothConfig) error {
	if config == nil {
		return fmt.Errorf("配置不能为空")
	}

	// 验证新配置
	if err := config.Validate(); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	cm.mu.Lock()
	oldConfig := cm.currentConfig
	cm.currentConfig = config
	callbacks := make([]ConfigCallback, len(cm.callbacks))
	copy(callbacks, cm.callbacks)
	cm.mu.Unlock()

	// 执行配置变更回调
	for _, callback := range callbacks {
		if err := callback(oldConfig, config); err != nil {
			cm.logger.Error("配置变更回调执行失败: %v", err)
			// 注意：这里不回滚配置，因为可能只是某个回调失败
		}
	}

	cm.logger.Info("配置更新成功")
	return nil
}

// defaultLogger 默认日志实现
type defaultLogger struct{}

func (l *defaultLogger) Info(msg string, args ...interface{}) {
	fmt.Printf("[INFO] "+msg+"\n", args...)
}

func (l *defaultLogger) Warn(msg string, args ...interface{}) {
	fmt.Printf("[WARN] "+msg+"\n", args...)
}

func (l *defaultLogger) Error(msg string, args ...interface{}) {
	fmt.Printf("[ERROR] "+msg+"\n", args...)
}

func (l *defaultLogger) Debug(msg string, args ...interface{}) {
	fmt.Printf("[DEBUG] "+msg+"\n", args...)
}
