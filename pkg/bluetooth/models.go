package bluetooth

import (
	"time"
)

// Message 泛型消息结构，利用 Go 1.25 泛型特性
type Message[T any] struct {
	ID        string          `json:"id"`        // 消息唯一标识
	Type      MessageType     `json:"type"`      // 消息类型
	Payload   T               `json:"payload"`   // 消息载荷
	Metadata  MessageMetadata `json:"metadata"`  // 消息元数据
	Timestamp time.Time       `json:"timestamp"` // 时间戳
}

// MessageMetadata 消息元数据
type MessageMetadata struct {
	SenderID   string        `json:"sender_id"`   // 发送者ID
	ReceiverID string        `json:"receiver_id"` // 接收者ID
	Priority   Priority      `json:"priority"`    // 消息优先级
	TTL        time.Duration `json:"ttl"`         // 生存时间
	Encrypted  bool          `json:"encrypted"`   // 是否加密
	Compressed bool          `json:"compressed"`  // 是否压缩
}

// Device 蓝牙设备信息
type Device struct {
	ID           string             `json:"id"`            // 设备唯一标识
	Name         string             `json:"name"`          // 设备名称
	Address      string             `json:"address"`       // 设备地址
	ServiceUUIDs []string           `json:"service_uuids"` // 服务UUID列表
	RSSI         int                `json:"rssi"`          // 信号强度
	LastSeen     time.Time          `json:"last_seen"`     // 最后发现时间
	Capabilities DeviceCapabilities `json:"capabilities"`  // 设备能力
}

// DeviceCapabilities 设备能力
type DeviceCapabilities struct {
	SupportsEncryption bool     `json:"supports_encryption"` // 支持加密
	MaxConnections     int      `json:"max_connections"`     // 最大连接数
	SupportedProtocols []string `json:"supported_protocols"` // 支持的协议
	BatteryLevel       int      `json:"battery_level"`       // 电池电量
}

// DeviceFilter 设备过滤器
type DeviceFilter struct {
	NamePattern  string        `json:"name_pattern"`  // 名称模式
	ServiceUUIDs []string      `json:"service_uuids"` // 服务UUID过滤
	MinRSSI      int           `json:"min_rssi"`      // 最小信号强度
	RequireAuth  bool          `json:"require_auth"`  // 需要认证
	DeviceTypes  []string      `json:"device_types"`  // 设备类型
	MaxAge       time.Duration `json:"max_age"`       // 最大发现时间
}

// ConnectionMetrics 连接指标
type ConnectionMetrics struct {
	BytesSent     uint64        `json:"bytes_sent"`     // 发送字节数
	BytesReceived uint64        `json:"bytes_received"` // 接收字节数
	MessagesSent  uint64        `json:"messages_sent"`  // 发送消息数
	MessagesRecv  uint64        `json:"messages_recv"`  // 接收消息数
	Latency       time.Duration `json:"latency"`        // 延迟
	ErrorCount    uint64        `json:"error_count"`    // 错误计数
	ConnectedAt   time.Time     `json:"connected_at"`   // 连接时间
	LastActivity  time.Time     `json:"last_activity"`  // 最后活动时间
}

// PoolStats 连接池统计信息
type PoolStats struct {
	ActiveConnections     int           `json:"active_connections"`      // 活跃连接数
	TotalConnections      int           `json:"total_connections"`       // 总连接数
	MaxConnections        int           `json:"max_connections"`         // 最大连接数
	AverageLatency        time.Duration `json:"average_latency"`         // 平均延迟
	TotalBytesTransferred uint64        `json:"total_bytes_transferred"` // 总传输字节数
	ErrorRate             float64       `json:"error_rate"`              // 错误率
}

// HealthStatus 健康状态
type HealthStatus struct {
	DeviceID      string        `json:"device_id"`      // 设备ID
	IsHealthy     bool          `json:"is_healthy"`     // 是否健康
	LastHeartbeat time.Time     `json:"last_heartbeat"` // 最后心跳时间
	RSSI          int           `json:"rssi"`           // 信号强度
	Latency       time.Duration `json:"latency"`        // 延迟
	ErrorCount    int           `json:"error_count"`    // 错误计数
	Status        string        `json:"status"`         // 状态描述
}

// HealthReport 健康报告
type HealthReport struct {
	Timestamp       time.Time      `json:"timestamp"`       // 报告时间戳
	OverallHealth   bool           `json:"overall_health"`  // 整体健康状态
	DeviceStatuses  []HealthStatus `json:"device_statuses"` // 设备状态列表
	SystemMetrics   SystemMetrics  `json:"system_metrics"`  // 系统指标
	Recommendations []string       `json:"recommendations"` // 建议
}

// SystemMetrics 系统指标
type SystemMetrics struct {
	CPUUsage    float64       `json:"cpu_usage"`    // CPU使用率
	MemoryUsage float64       `json:"memory_usage"` // 内存使用率
	Goroutines  int           `json:"goroutines"`   // Goroutine数量
	GCPauses    time.Duration `json:"gc_pauses"`    // GC暂停时间
}

// Credentials 认证凭据
type Credentials struct {
	Type     CredentialType `json:"type"`     // 凭据类型
	Username string         `json:"username"` // 用户名
	Password string         `json:"password"` // 密码
	Token    string         `json:"token"`    // 令牌
	KeyData  []byte         `json:"key_data"` // 密钥数据
}

// AccessPolicy 访问策略
type AccessPolicy struct {
	DeviceWhitelist   []string      `json:"device_whitelist"`   // 设备白名单
	AllowedOperations []Operation   `json:"allowed_operations"` // 允许的操作
	RequireEncryption bool          `json:"require_encryption"` // 需要加密
	SessionTimeout    time.Duration `json:"session_timeout"`    // 会话超时
	MaxRetries        int           `json:"max_retries"`        // 最大重试次数
}
