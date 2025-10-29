package config

import (
	"context"
	"fmt"
	"sync"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// ComponentFactory 组件工厂接口
type ComponentFactory interface {
	// CreateBluetoothComponent 创建蓝牙组件
	CreateBluetoothComponent(config *bluetooth.BluetoothConfig) (bluetooth.BluetoothComponent[[]byte], error)
	// CreateBluetoothServer 创建蓝牙服务端
	CreateBluetoothServer(config *bluetooth.BluetoothConfig) (bluetooth.BluetoothServer[[]byte], error)
	// CreateBluetoothClient 创建蓝牙客户端
	CreateBluetoothClient(config *bluetooth.BluetoothConfig) (bluetooth.BluetoothClient[[]byte], error)
	// CreateDeviceManager 创建设备管理器
	CreateDeviceManager(config *bluetooth.BluetoothConfig) (bluetooth.DeviceManager, error)
	// CreateConnectionPool 创建连接池
	CreateConnectionPool(config *bluetooth.BluetoothConfig) (bluetooth.ConnectionPool, error)
	// CreateHealthChecker 创建健康检查器
	CreateHealthChecker(config *bluetooth.BluetoothConfig) (bluetooth.HealthChecker, error)
	// CreateSecurityManager 创建安全管理器
	CreateSecurityManager(config *bluetooth.BluetoothConfig) (bluetooth.SecurityManager, error)
}

// DependencyContainer 依赖容器接口
type DependencyContainer interface {
	// Register 注册依赖
	Register(name string, instance interface{}) error
	// Resolve 解析依赖
	Resolve(name string) (interface{}, error)
	// RegisterFactory 注册工厂函数
	RegisterFactory(name string, factory func() (interface{}, error)) error
	// Clear 清空容器
	Clear()
}

// ComponentLifecycle 组件生命周期管理接口
type ComponentLifecycle interface {
	// Start 启动组件
	Start(ctx context.Context) error
	// Stop 停止组件
	Stop(ctx context.Context) error
	// IsRunning 检查组件是否运行中
	IsRunning() bool
	// GetStatus 获取组件状态
	GetStatus() ComponentStatus
}

// ComponentStatus 组件状态
type ComponentStatus int

const (
	StatusStopped  ComponentStatus = iota // 已停止
	StatusStarting                        // 启动中
	StatusRunning                         // 运行中
	StatusStopping                        // 停止中
	StatusError                           // 错误状态
)

// String 返回状态的字符串表示
func (cs ComponentStatus) String() string {
	switch cs {
	case StatusStopped:
		return "stopped"
	case StatusStarting:
		return "starting"
	case StatusRunning:
		return "running"
	case StatusStopping:
		return "stopping"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// AdvancedComponentBuilder 高级组件构建器
type AdvancedComponentBuilder struct {
	config           *bluetooth.BluetoothConfig // 配置
	factory          ComponentFactory           // 组件工厂
	container        DependencyContainer        // 依赖容器
	configManager    ConfigManager              // 配置管理器
	notifier         ConfigNotifier             // 配置通知器
	hotReloadManager *HotReloadManager          // 热更新管理器
	lifecycle        ComponentLifecycle         // 生命周期管理
	logger           Logger                     // 日志记录器
	mu               sync.RWMutex               // 读写锁
	components       map[string]interface{}     // 已创建的组件
	configCallbacks  []ConfigCallback           // 配置变更回调
}

// NewAdvancedComponentBuilder 创建高级组件构建器
func NewAdvancedComponentBuilder() *AdvancedComponentBuilder {
	logger := &defaultLogger{}
	container := NewDependencyContainer()
	configManager := NewConfigManager(logger)
	notifier := NewConfigNotifier(logger)
	hotReloadManager := NewHotReloadManager(configManager, notifier, logger)

	return &AdvancedComponentBuilder{
		config:           nil,
		factory:          NewDefaultComponentFactory(),
		container:        container,
		configManager:    configManager,
		notifier:         notifier,
		hotReloadManager: hotReloadManager,
		lifecycle:        NewComponentLifecycleManager(),
		logger:           logger,
		components:       make(map[string]interface{}),
		configCallbacks:  make([]ConfigCallback, 0),
	}
}

// WithConfig 设置配置
func (acb *AdvancedComponentBuilder) WithConfig(config *bluetooth.BluetoothConfig) *AdvancedComponentBuilder {
	acb.mu.Lock()
	defer acb.mu.Unlock()

	acb.config = config
	return acb
}

// WithConfigFile 从文件加载配置
func (acb *AdvancedComponentBuilder) WithConfigFile(configPath string) *AdvancedComponentBuilder {
	config, err := acb.configManager.LoadConfig(configPath)
	if err != nil {
		acb.logger.Error("加载配置文件失败: %v", err)
		return acb
	}

	return acb.WithConfig(config)
}

// WithFactory 设置组件工厂
func (acb *AdvancedComponentBuilder) WithFactory(factory ComponentFactory) *AdvancedComponentBuilder {
	acb.mu.Lock()
	defer acb.mu.Unlock()

	acb.factory = factory
	return acb
}

// WithLogger 设置日志记录器
func (acb *AdvancedComponentBuilder) WithLogger(logger Logger) *AdvancedComponentBuilder {
	acb.mu.Lock()
	defer acb.mu.Unlock()

	acb.logger = logger
	return acb
}

// WithDependency 注册依赖
func (acb *AdvancedComponentBuilder) WithDependency(name string, instance interface{}) *AdvancedComponentBuilder {
	if err := acb.container.Register(name, instance); err != nil {
		acb.logger.Error("注册依赖失败: %s, 错误: %v", name, err)
	}
	return acb
}

// WithDependencyFactory 注册依赖工厂
func (acb *AdvancedComponentBuilder) WithDependencyFactory(name string, factory func() (interface{}, error)) *AdvancedComponentBuilder {
	if err := acb.container.RegisterFactory(name, factory); err != nil {
		acb.logger.Error("注册依赖工厂失败: %s, 错误: %v", name, err)
	}
	return acb
}

// WithConfigCallback 添加配置变更回调
func (acb *AdvancedComponentBuilder) WithConfigCallback(callback ConfigCallback) *AdvancedComponentBuilder {
	acb.mu.Lock()
	defer acb.mu.Unlock()

	acb.configCallbacks = append(acb.configCallbacks, callback)
	acb.configManager.RegisterCallback(callback)
	return acb
}

// EnableHotReload 启用配置热更新
func (acb *AdvancedComponentBuilder) EnableHotReload(ctx context.Context, configPath string) *AdvancedComponentBuilder {
	if err := acb.hotReloadManager.StartWatching(ctx, configPath); err != nil {
		acb.logger.Error("启用配置热更新失败: %v", err)
	}
	return acb
}

// Build 构建蓝牙组件
func (acb *AdvancedComponentBuilder) Build() (bluetooth.BluetoothComponent[[]byte], error) {
	acb.mu.Lock()
	defer acb.mu.Unlock()

	// 检查配置
	if acb.config == nil {
		acb.logger.Info("未设置配置，使用默认配置")
		defaultConfig := bluetooth.DefaultConfig()
		acb.config = &defaultConfig
	}

	// 验证配置
	if err := acb.config.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	// 更新配置管理器中的配置
	if err := acb.configManager.UpdateConfig(acb.config); err != nil {
		return nil, fmt.Errorf("更新配置管理器失败: %w", err)
	}

	// 创建蓝牙组件
	component, err := acb.factory.CreateBluetoothComponent(acb.config)
	if err != nil {
		return nil, fmt.Errorf("创建蓝牙组件失败: %w", err)
	}

	// 注册组件到依赖容器
	if err := acb.container.Register("bluetooth_component", component); err != nil {
		acb.logger.Warn("注册蓝牙组件到依赖容器失败: %v", err)
	}

	// 保存组件引用
	acb.components["bluetooth_component"] = component

	acb.logger.Info("蓝牙组件构建成功")
	return component, nil
}

// BuildServer 构建蓝牙服务端
func (acb *AdvancedComponentBuilder) BuildServer() (bluetooth.BluetoothServer[[]byte], error) {
	acb.mu.Lock()
	defer acb.mu.Unlock()

	if acb.config == nil {
		return nil, fmt.Errorf("未设置配置")
	}

	server, err := acb.factory.CreateBluetoothServer(acb.config)
	if err != nil {
		return nil, fmt.Errorf("创建蓝牙服务端失败: %w", err)
	}

	acb.container.Register("bluetooth_server", server)
	acb.components["bluetooth_server"] = server

	acb.logger.Info("蓝牙服务端构建成功")
	return server, nil
}

// BuildClient 构建蓝牙客户端
func (acb *AdvancedComponentBuilder) BuildClient() (bluetooth.BluetoothClient[[]byte], error) {
	acb.mu.Lock()
	defer acb.mu.Unlock()

	if acb.config == nil {
		return nil, fmt.Errorf("未设置配置")
	}

	client, err := acb.factory.CreateBluetoothClient(acb.config)
	if err != nil {
		return nil, fmt.Errorf("创建蓝牙客户端失败: %w", err)
	}

	acb.container.Register("bluetooth_client", client)
	acb.components["bluetooth_client"] = client

	acb.logger.Info("蓝牙客户端构建成功")
	return client, nil
}

// GetComponent 获取已构建的组件
func (acb *AdvancedComponentBuilder) GetComponent(name string) (interface{}, error) {
	acb.mu.RLock()
	defer acb.mu.RUnlock()

	if component, exists := acb.components[name]; exists {
		return component, nil
	}

	// 尝试从依赖容器解析
	return acb.container.Resolve(name)
}

// GetConfig 获取当前配置
func (acb *AdvancedComponentBuilder) GetConfig() *bluetooth.BluetoothConfig {
	acb.mu.RLock()
	defer acb.mu.RUnlock()

	return acb.config
}

// GetConfigManager 获取配置管理器
func (acb *AdvancedComponentBuilder) GetConfigManager() ConfigManager {
	return acb.configManager
}

// GetNotifier 获取配置通知器
func (acb *AdvancedComponentBuilder) GetNotifier() ConfigNotifier {
	return acb.notifier
}

// Start 启动所有组件
func (acb *AdvancedComponentBuilder) Start(ctx context.Context) error {
	acb.mu.Lock()
	defer acb.mu.Unlock()

	acb.logger.Info("开始启动所有组件")

	// 启动生命周期管理器
	if err := acb.lifecycle.Start(ctx); err != nil {
		return fmt.Errorf("启动生命周期管理器失败: %w", err)
	}

	// 启动所有组件
	for name, component := range acb.components {
		if starter, ok := component.(interface{ Start(context.Context) error }); ok {
			if err := starter.Start(ctx); err != nil {
				acb.logger.Error("启动组件失败: %s, 错误: %v", name, err)
				return fmt.Errorf("启动组件 %s 失败: %w", name, err)
			}
			acb.logger.Info("组件启动成功: %s", name)
		}
	}

	acb.logger.Info("所有组件启动完成")
	return nil
}

// Stop 停止所有组件
func (acb *AdvancedComponentBuilder) Stop(ctx context.Context) error {
	acb.mu.Lock()
	defer acb.mu.Unlock()

	acb.logger.Info("开始停止所有组件")

	// 停止所有组件
	for name, component := range acb.components {
		if stopper, ok := component.(interface{ Stop(context.Context) error }); ok {
			if err := stopper.Stop(ctx); err != nil {
				acb.logger.Error("停止组件失败: %s, 错误: %v", name, err)
			} else {
				acb.logger.Info("组件停止成功: %s", name)
			}
		}
	}

	// 停止生命周期管理器
	if err := acb.lifecycle.Stop(ctx); err != nil {
		acb.logger.Error("停止生命周期管理器失败: %v", err)
	}

	// 停止热更新管理器
	if err := acb.hotReloadManager.Close(); err != nil {
		acb.logger.Error("关闭热更新管理器失败: %v", err)
	}

	// 关闭配置通知器
	if err := acb.notifier.Close(); err != nil {
		acb.logger.Error("关闭配置通知器失败: %v", err)
	}

	acb.logger.Info("所有组件停止完成")
	return nil
}

// Validate 验证构建器配置
func (acb *AdvancedComponentBuilder) Validate() error {
	acb.mu.RLock()
	defer acb.mu.RUnlock()

	if acb.config == nil {
		return fmt.Errorf("配置不能为空")
	}

	return acb.config.Validate()
}

// Reset 重置构建器
func (acb *AdvancedComponentBuilder) Reset() *AdvancedComponentBuilder {
	acb.mu.Lock()
	defer acb.mu.Unlock()

	// 清空组件
	acb.components = make(map[string]interface{})

	// 清空依赖容器
	acb.container.Clear()

	// 重置配置
	acb.config = nil

	// 清空配置回调
	for _, callback := range acb.configCallbacks {
		acb.configManager.UnregisterCallback(callback)
	}
	acb.configCallbacks = make([]ConfigCallback, 0)

	acb.logger.Info("构建器已重置")
	return acb
}
