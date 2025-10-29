package bluetooth

import (
	"fmt"
	"time"
)

// BluetoothConfig 蓝牙组件配置
type BluetoothConfig struct {
	ServerConfig   ServerConfig   `json:"server_config"`   // 服务端配置
	ClientConfig   ClientConfig   `json:"client_config"`   // 客户端配置
	SecurityConfig SecurityConfig `json:"security_config"` // 安全配置
	HealthConfig   HealthConfig   `json:"health_config"`   // 健康检查配置
	PoolConfig     PoolConfig     `json:"pool_config"`     // 连接池配置
	LogConfig      LogConfig      `json:"log_config"`      // 日志配置
}

// ServerConfig 服务端配置
type ServerConfig struct {
	ServiceUUID      string        `json:"service_uuid"`      // 服务UUID
	ServiceName      string        `json:"service_name"`      // 服务名称
	MaxConnections   int           `json:"max_connections"`   // 最大连接数
	AcceptTimeout    time.Duration `json:"accept_timeout"`    // 接受连接超时时间
	RequireAuth      bool          `json:"require_auth"`      // 是否需要认证
	EnableEncryption bool          `json:"enable_encryption"` // 是否启用加密
	ListenAddress    string        `json:"listen_address"`    // 监听地址
	Port             int           `json:"port"`              // 监听端口
}

// ClientConfig 客户端配置
type ClientConfig struct {
	ScanTimeout      time.Duration `json:"scan_timeout"`      // 扫描超时时间
	ConnectTimeout   time.Duration `json:"connect_timeout"`   // 连接超时时间
	RetryAttempts    int           `json:"retry_attempts"`    // 重试次数
	RetryInterval    time.Duration `json:"retry_interval"`    // 重试间隔
	AutoReconnect    bool          `json:"auto_reconnect"`    // 自动重连
	KeepAlive        bool          `json:"keep_alive"`        // 保持连接
	PreferredDevices []string      `json:"preferred_devices"` // 首选设备列表
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	EnableAuth        bool          `json:"enable_auth"`         // 启用认证
	EnableEncryption  bool          `json:"enable_encryption"`   // 启用加密
	EnableWhitelist   bool          `json:"enable_whitelist"`    // 启用白名单
	KeySize           int           `json:"key_size"`            // 密钥大小
	Algorithm         string        `json:"algorithm"`           // 加密算法
	CertificatePath   string        `json:"certificate_path"`    // 证书路径
	PrivateKeyPath    string        `json:"private_key_path"`    // 私钥路径
	TrustedDevices    []string      `json:"trusted_devices"`     // 信任设备列表
	SessionTimeout    time.Duration `json:"session_timeout"`     // 会话超时时间
	MaxFailedAttempts int           `json:"max_failed_attempts"` // 最大失败尝试次数
	RequireAuth       bool          `json:"require_auth"`        // 强制认证
	LogSecurityEvents bool          `json:"log_security_events"` // 记录安全事件
}

// HealthConfig 健康检查配置
type HealthConfig struct {
	EnableHealthCheck bool          `json:"enable_health_check"` // 启用健康检查
	CheckInterval     time.Duration `json:"check_interval"`      // 检查间隔
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`  // 心跳间隔
	TimeoutThreshold  time.Duration `json:"timeout_threshold"`   // 超时阈值
	MaxMissedBeats    int           `json:"max_missed_beats"`    // 最大丢失心跳数
	EnableMetrics     bool          `json:"enable_metrics"`      // 启用指标收集
	MetricsInterval   time.Duration `json:"metrics_interval"`    // 指标收集间隔
}

// PoolConfig 连接池配置
type PoolConfig struct {
	MaxConnections    int           `json:"max_connections"`     // 最大连接数
	MinConnections    int           `json:"min_connections"`     // 最小连接数
	MaxIdleTime       time.Duration `json:"max_idle_time"`       // 最大空闲时间
	ConnectionTimeout time.Duration `json:"connection_timeout"`  // 连接超时时间
	EnableLoadBalance bool          `json:"enable_load_balance"` // 启用负载均衡
	BalanceStrategy   string        `json:"balance_strategy"`    // 负载均衡策略
	EnablePriority    bool          `json:"enable_priority"`     // 启用优先级
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `json:"level"`       // 日志级别
	Format     string `json:"format"`      // 日志格式
	Output     string `json:"output"`      // 输出目标
	EnableFile bool   `json:"enable_file"` // 启用文件输出
	FilePath   string `json:"file_path"`   // 文件路径
	MaxSize    int    `json:"max_size"`    // 最大文件大小(MB)
	MaxBackups int    `json:"max_backups"` // 最大备份数
	MaxAge     int    `json:"max_age"`     // 最大保存天数
	Compress   bool   `json:"compress"`    // 是否压缩
}

// DefaultConfig 返回默认配置
func DefaultConfig() BluetoothConfig {
	return BluetoothConfig{
		ServerConfig: ServerConfig{
			ServiceUUID:      DefaultServiceUUID,
			ServiceName:      "CatTag Bluetooth Service",
			MaxConnections:   DefaultMaxConnections,
			AcceptTimeout:    DefaultConnectTimeout,
			RequireAuth:      true,
			EnableEncryption: true,
			ListenAddress:    "0.0.0.0",
			Port:             0, // 系统分配端口
		},
		ClientConfig: ClientConfig{
			ScanTimeout:      DefaultScanTimeout,
			ConnectTimeout:   DefaultConnectTimeout,
			RetryAttempts:    DefaultMaxRetries,
			RetryInterval:    DefaultRetryInterval,
			AutoReconnect:    true,
			KeepAlive:        true,
			PreferredDevices: make([]string, 0),
		},
		SecurityConfig: SecurityConfig{
			EnableAuth:        true,
			EnableEncryption:  true,
			EnableWhitelist:   false,
			KeySize:           DefaultEncryptionKeySize,
			Algorithm:         "AES-256-GCM",
			SessionTimeout:    DefaultSessionTimeout,
			MaxFailedAttempts: 3,
			RequireAuth:       true,
			LogSecurityEvents: true,
			TrustedDevices:    make([]string, 0),
		},
		HealthConfig: HealthConfig{
			EnableHealthCheck: true,
			CheckInterval:     DefaultHeartbeatInterval,
			HeartbeatInterval: DefaultHeartbeatInterval,
			TimeoutThreshold:  DefaultConnectTimeout,
			MaxMissedBeats:    3,
			EnableMetrics:     true,
			MetricsInterval:   30 * time.Second,
		},
		PoolConfig: PoolConfig{
			MaxConnections:    DefaultMaxConnections,
			MinConnections:    1,
			MaxIdleTime:       5 * time.Minute,
			ConnectionTimeout: DefaultConnectTimeout,
			EnableLoadBalance: true,
			BalanceStrategy:   "round_robin",
			EnablePriority:    true,
		},
		LogConfig: LogConfig{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			EnableFile: false,
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     7,
			Compress:   true,
		},
	}
}

// Validate 验证配置的有效性
func (c *BluetoothConfig) Validate() error {
	// 验证服务端配置
	if c.ServerConfig.MaxConnections <= 0 {
		return NewBluetoothError(ErrCodeInvalidParameter, "服务端最大连接数必须大于0")
	}
	if c.ServerConfig.AcceptTimeout <= 0 {
		return NewBluetoothError(ErrCodeInvalidParameter, "服务端接受超时时间必须大于0")
	}

	// 验证客户端配置
	if c.ClientConfig.ScanTimeout <= 0 {
		return NewBluetoothError(ErrCodeInvalidParameter, "客户端扫描超时时间必须大于0")
	}
	if c.ClientConfig.ConnectTimeout <= 0 {
		return NewBluetoothError(ErrCodeInvalidParameter, "客户端连接超时时间必须大于0")
	}
	if c.ClientConfig.RetryAttempts < 0 {
		return NewBluetoothError(ErrCodeInvalidParameter, "客户端重试次数不能为负数")
	}

	// 验证安全配置
	if c.SecurityConfig.EnableEncryption && c.SecurityConfig.KeySize <= 0 {
		return NewBluetoothError(ErrCodeInvalidParameter, "启用加密时密钥大小必须大于0")
	}
	if c.SecurityConfig.SessionTimeout <= 0 {
		return NewBluetoothError(ErrCodeInvalidParameter, "会话超时时间必须大于0")
	}

	// 验证健康检查配置
	if c.HealthConfig.EnableHealthCheck {
		if c.HealthConfig.CheckInterval <= 0 {
			return NewBluetoothError(ErrCodeInvalidParameter, "健康检查间隔必须大于0")
		}
		if c.HealthConfig.HeartbeatInterval <= 0 {
			return NewBluetoothError(ErrCodeInvalidParameter, "心跳间隔必须大于0")
		}
	}

	// 验证连接池配置
	if c.PoolConfig.MaxConnections <= 0 {
		return NewBluetoothError(ErrCodeInvalidParameter, "连接池最大连接数必须大于0")
	}
	if c.PoolConfig.MinConnections < 0 {
		return NewBluetoothError(ErrCodeInvalidParameter, "连接池最小连接数不能为负数")
	}
	if c.PoolConfig.MinConnections > c.PoolConfig.MaxConnections {
		return NewBluetoothError(ErrCodeInvalidParameter, "连接池最小连接数不能大于最大连接数")
	}

	return nil
}

// ComponentBuilder 组件构建器，支持链式配置
type ComponentBuilder struct {
	config BluetoothConfig
}

// NewComponentBuilder 创建新的组件构建器
func NewComponentBuilder() *ComponentBuilder {
	return &ComponentBuilder{
		config: DefaultConfig(),
	}
}

// EnableServerMode 启用服务端模式
func (b *ComponentBuilder) EnableServerMode(serviceUUID string) *ComponentBuilder {
	b.config.ServerConfig.ServiceUUID = serviceUUID
	return b
}

// EnableClientMode 启用客户端模式
func (b *ComponentBuilder) EnableClientMode() *ComponentBuilder {
	b.config.ClientConfig.AutoReconnect = true
	return b
}

// EnableSecurity 启用安全功能
func (b *ComponentBuilder) EnableSecurity() *ComponentBuilder {
	b.config.SecurityConfig.EnableAuth = true
	b.config.SecurityConfig.EnableEncryption = true
	return b
}

// EnableHealthCheck 启用健康检查
func (b *ComponentBuilder) EnableHealthCheck() *ComponentBuilder {
	b.config.HealthConfig.EnableHealthCheck = true
	return b
}

// SetMaxConnections 设置最大连接数
func (b *ComponentBuilder) SetMaxConnections(max int) *ComponentBuilder {
	b.config.ServerConfig.MaxConnections = max
	b.config.PoolConfig.MaxConnections = max
	return b
}

// SetLogLevel 设置日志级别
func (b *ComponentBuilder) SetLogLevel(level string) *ComponentBuilder {
	b.config.LogConfig.Level = level
	return b
}

// WithConfig 设置完整配置
func (b *ComponentBuilder) WithConfig(config BluetoothConfig) *ComponentBuilder {
	b.config = config
	return b
}

// WithServerConfig 设置服务端配置
func (b *ComponentBuilder) WithServerConfig(config ServerConfig) *ComponentBuilder {
	b.config.ServerConfig = config
	return b
}

// WithClientConfig 设置客户端配置
func (b *ComponentBuilder) WithClientConfig(config ClientConfig) *ComponentBuilder {
	b.config.ClientConfig = config
	return b
}

// WithSecurityConfig 设置安全配置
func (b *ComponentBuilder) WithSecurityConfig(config SecurityConfig) *ComponentBuilder {
	b.config.SecurityConfig = config
	return b
}

// WithHealthConfig 设置健康检查配置
func (b *ComponentBuilder) WithHealthConfig(config HealthConfig) *ComponentBuilder {
	b.config.HealthConfig = config
	return b
}

// WithPoolConfig 设置连接池配置
func (b *ComponentBuilder) WithPoolConfig(config PoolConfig) *ComponentBuilder {
	b.config.PoolConfig = config
	return b
}

// WithLogConfig 设置日志配置
func (b *ComponentBuilder) WithLogConfig(config LogConfig) *ComponentBuilder {
	b.config.LogConfig = config
	return b
}

// GetConfig 获取当前配置
func (b *ComponentBuilder) GetConfig() BluetoothConfig {
	return b.config
}

// Validate 验证构建器配置
func (b *ComponentBuilder) Validate() error {
	return b.config.Validate()
}

// Build 构建蓝牙组件实例
func (b *ComponentBuilder) Build() (*DefaultBluetoothComponent[any], error) {
	// 验证配置
	if err := b.Validate(); err != nil {
		return nil, fmt.Errorf("构建验证失败: %w", err)
	}

	// 创建组件实例
	return NewBluetoothComponent[any](b.config)
}

// 预定义的构建器配置

// NewServerBuilder 创建服务端构建器
func NewServerBuilder(serviceUUID string) *ComponentBuilder {
	return NewComponentBuilder().
		EnableServerMode(serviceUUID).
		EnableSecurity().
		EnableHealthCheck()
}

// NewClientBuilder 创建客户端构建器
func NewClientBuilder() *ComponentBuilder {
	return NewComponentBuilder().
		EnableClientMode().
		EnableSecurity().
		EnableHealthCheck()
}

// NewFullBuilder 创建完整功能构建器
func NewFullBuilder(serviceUUID string) *ComponentBuilder {
	return NewComponentBuilder().
		EnableServerMode(serviceUUID).
		EnableClientMode().
		EnableSecurity().
		EnableHealthCheck().
		SetMaxConnections(DefaultMaxConnections)
}
