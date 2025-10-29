package config

import (
	"context"
	"sync"
	"time"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// ConfigNotifier 配置变更通知器接口
type ConfigNotifier interface {
	// Subscribe 订阅配置变更通知
	Subscribe(ctx context.Context) <-chan ConfigChangeEvent
	// Notify 发送配置变更通知
	Notify(event ConfigChangeEvent) error
	// Close 关闭通知器
	Close() error
}

// ConfigChangeEvent 配置变更事件
type ConfigChangeEvent struct {
	Type      ChangeType                 `json:"type"`       // 变更类型
	OldConfig *bluetooth.BluetoothConfig `json:"old_config"` // 旧配置
	NewConfig *bluetooth.BluetoothConfig `json:"new_config"` // 新配置
	Timestamp time.Time                  `json:"timestamp"`  // 变更时间
	Source    string                     `json:"source"`     // 变更源
	Reason    string                     `json:"reason"`     // 变更原因
}

// ChangeType 配置变更类型
type ChangeType int

const (
	ChangeTypeUpdate ChangeType = iota // 配置更新
	ChangeTypeReload                   // 配置重载
	ChangeTypeReset                    // 配置重置
)

// String 返回变更类型的字符串表示
func (ct ChangeType) String() string {
	switch ct {
	case ChangeTypeUpdate:
		return "update"
	case ChangeTypeReload:
		return "reload"
	case ChangeTypeReset:
		return "reset"
	default:
		return "unknown"
	}
}

// ConfigNotifierImpl 配置变更通知器实现
type ConfigNotifierImpl struct {
	mu          sync.RWMutex                      // 读写锁
	subscribers map[string]chan ConfigChangeEvent // 订阅者通道映射
	closed      bool                              // 是否已关闭
	logger      Logger                            // 日志记录器
}

// NewConfigNotifier 创建新的配置变更通知器
func NewConfigNotifier(logger Logger) ConfigNotifier {
	if logger == nil {
		logger = &defaultLogger{}
	}

	return &ConfigNotifierImpl{
		subscribers: make(map[string]chan ConfigChangeEvent),
		closed:      false,
		logger:      logger,
	}
}

// Subscribe 订阅配置变更通知
func (cn *ConfigNotifierImpl) Subscribe(ctx context.Context) <-chan ConfigChangeEvent {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	if cn.closed {
		// 如果通知器已关闭，返回一个立即关闭的通道
		ch := make(chan ConfigChangeEvent)
		close(ch)
		return ch
	}

	// 生成唯一的订阅者ID
	subscriberID := generateSubscriberID()

	// 创建订阅者通道
	ch := make(chan ConfigChangeEvent, 10) // 缓冲10个事件
	cn.subscribers[subscriberID] = ch

	cn.logger.Debug("新增配置变更订阅者: %s", subscriberID)

	// 启动清理goroutine，当上下文取消时清理订阅者
	go func() {
		<-ctx.Done()
		cn.unsubscribe(subscriberID)
	}()

	return ch
}

// unsubscribe 取消订阅（内部方法）
func (cn *ConfigNotifierImpl) unsubscribe(subscriberID string) {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	if ch, exists := cn.subscribers[subscriberID]; exists {
		close(ch)
		delete(cn.subscribers, subscriberID)
		cn.logger.Debug("移除配置变更订阅者: %s", subscriberID)
	}
}

// Notify 发送配置变更通知
func (cn *ConfigNotifierImpl) Notify(event ConfigChangeEvent) error {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	if cn.closed {
		return ErrNotifierClosed
	}

	cn.logger.Info("发送配置变更通知: 类型=%s, 源=%s", event.Type.String(), event.Source)

	// 向所有订阅者发送事件
	for subscriberID, ch := range cn.subscribers {
		select {
		case ch <- event:
			cn.logger.Debug("配置变更通知已发送给订阅者: %s", subscriberID)
		default:
			cn.logger.Warn("订阅者通道已满，跳过通知: %s", subscriberID)
		}
	}

	return nil
}

// Close 关闭通知器
func (cn *ConfigNotifierImpl) Close() error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	if cn.closed {
		return nil
	}

	cn.closed = true

	// 关闭所有订阅者通道
	for subscriberID, ch := range cn.subscribers {
		close(ch)
		cn.logger.Debug("关闭订阅者通道: %s", subscriberID)
	}

	// 清空订阅者映射
	cn.subscribers = make(map[string]chan ConfigChangeEvent)

	cn.logger.Info("配置变更通知器已关闭")
	return nil
}

// generateSubscriberID 生成唯一的订阅者ID
func generateSubscriberID() string {
	return time.Now().Format("20060102150405.000000")
}

// 预定义错误
var (
	ErrNotifierClosed = &ConfigError{
		Code:    "NOTIFIER_CLOSED",
		Message: "配置变更通知器已关闭",
	}
)

// ConfigError 配置相关错误
type ConfigError struct {
	Code    string `json:"code"`    // 错误代码
	Message string `json:"message"` // 错误消息
}

// Error 实现error接口
func (e *ConfigError) Error() string {
	return e.Message
}

// HotReloadManager 热更新管理器
type HotReloadManager struct {
	configManager ConfigManager                 // 配置管理器
	notifier      ConfigNotifier                // 配置通知器
	mu            sync.RWMutex                  // 读写锁
	watchers      map[string]context.CancelFunc // 文件监控器
	logger        Logger                        // 日志记录器
}

// NewHotReloadManager 创建新的热更新管理器
func NewHotReloadManager(configManager ConfigManager, notifier ConfigNotifier, logger Logger) *HotReloadManager {
	if logger == nil {
		logger = &defaultLogger{}
	}

	return &HotReloadManager{
		configManager: configManager,
		notifier:      notifier,
		watchers:      make(map[string]context.CancelFunc),
		logger:        logger,
	}
}

// StartWatching 开始监控配置文件
func (hrm *HotReloadManager) StartWatching(ctx context.Context, configPath string) error {
	hrm.mu.Lock()
	defer hrm.mu.Unlock()

	// 如果已经在监控该文件，先停止之前的监控
	if cancel, exists := hrm.watchers[configPath]; exists {
		cancel()
	}

	// 开始监控配置文件
	configChan, err := hrm.configManager.WatchConfig(ctx, configPath)
	if err != nil {
		return err
	}

	// 创建监控上下文
	watchCtx, cancel := context.WithCancel(ctx)
	hrm.watchers[configPath] = cancel

	// 启动配置变更处理goroutine
	go hrm.handleConfigChanges(watchCtx, configPath, configChan)

	hrm.logger.Info("开始热更新监控: %s", configPath)
	return nil
}

// StopWatching 停止监控配置文件
func (hrm *HotReloadManager) StopWatching(configPath string) {
	hrm.mu.Lock()
	defer hrm.mu.Unlock()

	if cancel, exists := hrm.watchers[configPath]; exists {
		cancel()
		delete(hrm.watchers, configPath)
		hrm.logger.Info("停止热更新监控: %s", configPath)
	}
}

// handleConfigChanges 处理配置变更
func (hrm *HotReloadManager) handleConfigChanges(ctx context.Context, configPath string, configChan <-chan *bluetooth.BluetoothConfig) {
	for {
		select {
		case <-ctx.Done():
			hrm.logger.Info("配置变更处理器已停止: %s", configPath)
			return
		case newConfig, ok := <-configChan:
			if !ok {
				hrm.logger.Info("配置变更通道已关闭: %s", configPath)
				return
			}

			// 获取当前配置
			oldConfig := hrm.configManager.GetCurrentConfig()

			// 更新配置
			if err := hrm.configManager.UpdateConfig(newConfig); err != nil {
				hrm.logger.Error("更新配置失败: %v", err)
				continue
			}

			// 发送配置变更通知
			event := ConfigChangeEvent{
				Type:      ChangeTypeReload,
				OldConfig: oldConfig,
				NewConfig: newConfig,
				Timestamp: time.Now(),
				Source:    configPath,
				Reason:    "文件变更触发热更新",
			}

			if err := hrm.notifier.Notify(event); err != nil {
				hrm.logger.Error("发送配置变更通知失败: %v", err)
			}
		}
	}
}

// Close 关闭热更新管理器
func (hrm *HotReloadManager) Close() error {
	hrm.mu.Lock()
	defer hrm.mu.Unlock()

	// 停止所有文件监控
	for configPath, cancel := range hrm.watchers {
		cancel()
		hrm.logger.Debug("停止监控配置文件: %s", configPath)
	}

	// 清空监控器映射
	hrm.watchers = make(map[string]context.CancelFunc)

	hrm.logger.Info("热更新管理器已关闭")
	return nil
}
