package bluetooth

import (
	"fmt"
)

// ComponentFactory 组件工厂，提供便捷的组件创建方法
type ComponentFactory struct{}

// NewFactory 创建新的组件工厂
func NewFactory() *ComponentFactory {
	return &ComponentFactory{}
}

// CreateServer 创建蓝牙服务端组件
func (f *ComponentFactory) CreateServer(serviceUUID string, options ...ServerOption) (*DefaultBluetoothComponent[any], error) {
	builder := NewServerBuilder(serviceUUID)

	// 应用选项
	for _, option := range options {
		if err := option(builder); err != nil {
			return nil, fmt.Errorf("应用服务端选项失败: %w", err)
		}
	}

	return builder.Build()
}

// CreateClient 创建蓝牙客户端组件
func (f *ComponentFactory) CreateClient(options ...ClientOption) (*DefaultBluetoothComponent[any], error) {
	builder := NewClientBuilder()

	// 应用选项
	for _, option := range options {
		if err := option(builder); err != nil {
			return nil, fmt.Errorf("应用客户端选项失败: %w", err)
		}
	}

	return builder.Build()
}

// CreateFullComponent 创建完整功能的蓝牙组件
func (f *ComponentFactory) CreateFullComponent(serviceUUID string, options ...ComponentOption) (*DefaultBluetoothComponent[any], error) {
	builder := NewFullBuilder(serviceUUID)

	// 应用选项
	for _, option := range options {
		if err := option(builder); err != nil {
			return nil, fmt.Errorf("应用组件选项失败: %w", err)
		}
	}

	return builder.Build()
}

// CreateCustomComponent 使用自定义配置创建组件
func (f *ComponentFactory) CreateCustomComponent(config BluetoothConfig, options ...ComponentOption) (*DefaultBluetoothComponent[any], error) {
	builder := NewComponentBuilder().WithConfig(config)

	// 应用选项
	for _, option := range options {
		if err := option(builder); err != nil {
			return nil, fmt.Errorf("应用自定义选项失败: %w", err)
		}
	}

	return builder.Build()
}

// 选项函数类型定义

// ServerOption 服务端选项函数
type ServerOption func(*ComponentBuilder) error

// ClientOption 客户端选项函数
type ClientOption func(*ComponentBuilder) error

// ComponentOption 组件选项函数
type ComponentOption func(*ComponentBuilder) error

// 预定义的选项函数

// WithMaxConnections 设置最大连接数选项
func WithMaxConnections(max int) ComponentOption {
	return func(builder *ComponentBuilder) error {
		builder.SetMaxConnections(max)
		return nil
	}
}

// WithLogLevel 设置日志级别选项
func WithLogLevel(level string) ComponentOption {
	return func(builder *ComponentBuilder) error {
		builder.SetLogLevel(level)
		return nil
	}
}

// WithSecurityDisabled 禁用安全功能选项
func WithSecurityDisabled() ComponentOption {
	return func(builder *ComponentBuilder) error {
		config := builder.GetConfig()
		config.SecurityConfig.EnableAuth = false
		config.SecurityConfig.EnableEncryption = false
		builder.WithSecurityConfig(config.SecurityConfig)
		return nil
	}
}

// WithHealthCheckDisabled 禁用健康检查选项
func WithHealthCheckDisabled() ComponentOption {
	return func(builder *ComponentBuilder) error {
		config := builder.GetConfig()
		config.HealthConfig.EnableHealthCheck = false
		builder.WithHealthConfig(config.HealthConfig)
		return nil
	}
}

// WithCustomServerConfig 自定义服务端配置选项
func WithCustomServerConfig(config ServerConfig) ComponentOption {
	return func(builder *ComponentBuilder) error {
		builder.WithServerConfig(config)
		return nil
	}
}

// WithCustomClientConfig 自定义客户端配置选项
func WithCustomClientConfig(config ClientConfig) ComponentOption {
	return func(builder *ComponentBuilder) error {
		builder.WithClientConfig(config)
		return nil
	}
}

// WithCustomSecurityConfig 自定义安全配置选项
func WithCustomSecurityConfig(config SecurityConfig) ComponentOption {
	return func(builder *ComponentBuilder) error {
		builder.WithSecurityConfig(config)
		return nil
	}
}

// WithCustomHealthConfig 自定义健康检查配置选项
func WithCustomHealthConfig(config HealthConfig) ComponentOption {
	return func(builder *ComponentBuilder) error {
		builder.WithHealthConfig(config)
		return nil
	}
}

// WithCustomPoolConfig 自定义连接池配置选项
func WithCustomPoolConfig(config PoolConfig) ComponentOption {
	return func(builder *ComponentBuilder) error {
		builder.WithPoolConfig(config)
		return nil
	}
}

// WithCustomLogConfig 自定义日志配置选项
func WithCustomLogConfig(config LogConfig) ComponentOption {
	return func(builder *ComponentBuilder) error {
		builder.WithLogConfig(config)
		return nil
	}
}

// 全局工厂实例
var DefaultFactory = NewFactory()

// 便捷函数

// CreateBluetoothServer 创建蓝牙服务端（便捷函数）
func CreateBluetoothServer(serviceUUID string, options ...ServerOption) (*DefaultBluetoothComponent[any], error) {
	return DefaultFactory.CreateServer(serviceUUID, options...)
}

// CreateBluetoothClient 创建蓝牙客户端（便捷函数）
func CreateBluetoothClient(options ...ClientOption) (*DefaultBluetoothComponent[any], error) {
	return DefaultFactory.CreateClient(options...)
}

// CreateBluetoothComponent 创建完整蓝牙组件（便捷函数）
func CreateBluetoothComponent(serviceUUID string, options ...ComponentOption) (*DefaultBluetoothComponent[any], error) {
	return DefaultFactory.CreateFullComponent(serviceUUID, options...)
}

// CreateCustomBluetoothComponent 使用自定义配置创建组件（便捷函数）
func CreateCustomBluetoothComponent(config BluetoothConfig, options ...ComponentOption) (*DefaultBluetoothComponent[any], error) {
	return DefaultFactory.CreateCustomComponent(config, options...)
}
