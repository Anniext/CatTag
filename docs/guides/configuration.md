# 配置管理指南

本指南详细介绍 CatTag 蓝牙通信库的配置系统使用方法。

## 配置概述

CatTag 提供了灵活的配置系统，支持多种配置方式和层次化配置管理。

### 配置结构

```go
type BluetoothConfig struct {
    ServerConfig   ServerConfig   `json:"server_config"`
    ClientConfig   ClientConfig   `json:"client_config"`
    SecurityConfig SecurityConfig `json:"security_config"`
    HealthConfig   HealthConfig   `json:"health_config"`
    PoolConfig     PoolConfig     `json:"pool_config"`
    LogConfig      LogConfig      `json:"log_config"`
}
```

## 默认配置

### 获取默认配置

```go
config := bluetooth.DefaultConfig()
```

### 默认配置值

```go
// 服务端配置
ServerConfig{
    ServiceUUID:      "12345678-1234-1234-1234-123456789abc",
    ServiceName:      "CatTag Bluetooth Service",
    MaxConnections:   10,
    AcceptTimeout:    30 * time.Second,
    RequireAuth:      true,
    EnableEncryption: true,
}

// 客户端配置
ClientConfig{
    ScanTimeout:    10 * time.Second,
    ConnectTimeout: 30 * time.Second,
    RetryAttempts:  3,
    RetryInterval:  5 * time.Second,
    AutoReconnect:  true,
}

// 安全配置
SecurityConfig{
    EnableAuth:       true,
    EnableEncryption: true,
    KeySize:          256,
    Algorithm:        "AES-256-GCM",
    SessionTimeout:   1 * time.Hour,
}
```

## 配置方式

### 1. 代码配置

```go
config := bluetooth.DefaultConfig()

// 修改服务端配置
config.ServerConfig.MaxConnections = 20
config.ServerConfig.ServiceName = "我的蓝牙服务"

// 修改客户端配置
config.ClientConfig.AutoReconnect = false
config.ClientConfig.RetryAttempts = 5

// 修改安全配置
config.SecurityConfig.EnableEncryption = false

// 创建组件
component, err := bluetooth.NewBluetoothComponent[MyMessage](config)
```

### 2. 构建器模式

```go
component, err := bluetooth.NewServerBuilder("service-uuid").
    SetMaxConnections(15).
    EnableSecurity().
    EnableHealthCheck().
    SetLogLevel("debug").
    Build()
```

### 3. JSON 配置文件

创建配置文件 `bluetooth.json`：

```json
{
  "server_config": {
    "service_uuid": "12345678-1234-1234-1234-123456789abc",
    "service_name": "我的蓝牙服务",
    "max_connections": 15,
    "accept_timeout": "30s",
    "require_auth": true,
    "enable_encryption": true
  },
  "client_config": {
    "scan_timeout": "15s",
    "connect_timeout": "30s",
    "retry_attempts": 5,
    "retry_interval": "3s",
    "auto_reconnect": true
  },
  "security_config": {
    "enable_auth": true,
    "enable_encryption": true,
    "key_size": 256,
    "algorithm": "AES-256-GCM",
    "session_timeout": "1h"
  }
}
```

加载配置文件：

```go
func loadConfig(filename string) (bluetooth.BluetoothConfig, error) {
    data, err := os.ReadFile(filename)
    if err != nil {
        return bluetooth.BluetoothConfig{}, err
    }

    var config bluetooth.BluetoothConfig
    err = json.Unmarshal(data, &config)
    return config, err
}

// 使用
config, err := loadConfig("bluetooth.json")
if err != nil {
    log.Fatal(err)
}

component, err := bluetooth.NewBluetoothComponent[MyMessage](config)
```

## 配置详解

### 服务端配置 (ServerConfig)

```go
type ServerConfig struct {
    ServiceUUID      string        `json:"service_uuid"`      // 服务UUID
    ServiceName      string        `json:"service_name"`      // 服务名称
    MaxConnections   int           `json:"max_connections"`   // 最大连接数
    AcceptTimeout    time.Duration `json:"accept_timeout"`    // 接受连接超时
    RequireAuth      bool          `json:"require_auth"`      // 是否需要认证
    EnableEncryption bool          `json:"enable_encryption"` // 是否启用加密
    ListenAddress    string        `json:"listen_address"`    // 监听地址
    Port             int           `json:"port"`              // 监听端口
}
```

**配置说明：**

- `ServiceUUID`: 蓝牙服务的唯一标识符
- `MaxConnections`: 同时支持的最大客户端连接数
- `AcceptTimeout`: 等待客户端连接的超时时间
- `RequireAuth`: 是否要求客户端进行身份认证

### 客户端配置 (ClientConfig)

```go
type ClientConfig struct {
    ScanTimeout      time.Duration `json:"scan_timeout"`      // 扫描超时
    ConnectTimeout   time.Duration `json:"connect_timeout"`   // 连接超时
    RetryAttempts    int           `json:"retry_attempts"`    // 重试次数
    RetryInterval    time.Duration `json:"retry_interval"`    // 重试间隔
    AutoReconnect    bool          `json:"auto_reconnect"`    // 自动重连
    KeepAlive        bool          `json:"keep_alive"`        // 保持连接
    PreferredDevices []string      `json:"preferred_devices"` // 首选设备
}
```

**配置说明：**

- `ScanTimeout`: 设备扫描的最大等待时间
- `AutoReconnect`: 连接断开后是否自动重连
- `PreferredDevices`: 优先连接的设备 ID 列表

### 安全配置 (SecurityConfig)

```go
type SecurityConfig struct {
    EnableAuth        bool          `json:"enable_auth"`         // 启用认证
    EnableEncryption  bool          `json:"enable_encryption"`   // 启用加密
    EnableWhitelist   bool          `json:"enable_whitelist"`    // 启用白名单
    KeySize           int           `json:"key_size"`            // 密钥大小
    Algorithm         string        `json:"algorithm"`           // 加密算法
    TrustedDevices    []string      `json:"trusted_devices"`     // 信任设备
    SessionTimeout    time.Duration `json:"session_timeout"`     // 会话超时
    MaxFailedAttempts int           `json:"max_failed_attempts"` // 最大失败次数
}
```

**配置说明：**

- `EnableAuth`: 是否启用设备认证
- `EnableEncryption`: 是否启用数据加密
- `Algorithm`: 支持的加密算法（AES-256-GCM, AES-128-GCM 等）
- `TrustedDevices`: 信任设备白名单

### 健康检查配置 (HealthConfig)

```go
type HealthConfig struct {
    EnableHealthCheck bool          `json:"enable_health_check"` // 启用健康检查
    CheckInterval     time.Duration `json:"check_interval"`      // 检查间隔
    HeartbeatInterval time.Duration `json:"heartbeat_interval"`  // 心跳间隔
    TimeoutThreshold  time.Duration `json:"timeout_threshold"`   // 超时阈值
    MaxMissedBeats    int           `json:"max_missed_beats"`    // 最大丢失心跳
    EnableMetrics     bool          `json:"enable_metrics"`      // 启用指标收集
}
```

## 配置验证

### 自动验证

所有配置在使用前都会自动验证：

```go
config := bluetooth.DefaultConfig()
config.ServerConfig.MaxConnections = -1 // 无效值

err := config.Validate()
if err != nil {
    log.Printf("配置验证失败: %v", err)
}
```

### 验证规则

- 连接数必须大于 0
- 超时时间必须大于 0
- 密钥大小必须是有效值（128, 192, 256）
- UUID 格式必须正确

## 环境变量配置

支持通过环境变量覆盖配置：

```bash
export CATTAG_SERVER_MAX_CONNECTIONS=20
export CATTAG_CLIENT_AUTO_RECONNECT=true
export CATTAG_SECURITY_ENABLE_ENCRYPTION=false
```

```go
func loadConfigFromEnv() bluetooth.BluetoothConfig {
    config := bluetooth.DefaultConfig()

    if maxConn := os.Getenv("CATTAG_SERVER_MAX_CONNECTIONS"); maxConn != "" {
        if n, err := strconv.Atoi(maxConn); err == nil {
            config.ServerConfig.MaxConnections = n
        }
    }

    if autoReconnect := os.Getenv("CATTAG_CLIENT_AUTO_RECONNECT"); autoReconnect != "" {
        if b, err := strconv.ParseBool(autoReconnect); err == nil {
            config.ClientConfig.AutoReconnect = b
        }
    }

    return config
}
```

## 配置最佳实践

### 1. 分环境配置

```go
func getConfig(env string) bluetooth.BluetoothConfig {
    switch env {
    case "development":
        return getDevelopmentConfig()
    case "production":
        return getProductionConfig()
    case "testing":
        return getTestingConfig()
    default:
        return bluetooth.DefaultConfig()
    }
}

func getDevelopmentConfig() bluetooth.BluetoothConfig {
    config := bluetooth.DefaultConfig()
    config.LogConfig.Level = "debug"
    config.ServerConfig.MaxConnections = 5
    return config
}

func getProductionConfig() bluetooth.BluetoothConfig {
    config := bluetooth.DefaultConfig()
    config.LogConfig.Level = "warn"
    config.ServerConfig.MaxConnections = 100
    config.SecurityConfig.EnableAuth = true
    return config
}
```

### 2. 配置热更新

```go
type ConfigManager struct {
    config   bluetooth.BluetoothConfig
    mu       sync.RWMutex
    watchers []chan bluetooth.BluetoothConfig
}

func (cm *ConfigManager) UpdateConfig(newConfig bluetooth.BluetoothConfig) error {
    if err := newConfig.Validate(); err != nil {
        return err
    }

    cm.mu.Lock()
    cm.config = newConfig
    cm.mu.Unlock()

    // 通知所有观察者
    for _, watcher := range cm.watchers {
        select {
        case watcher <- newConfig:
        default:
        }
    }

    return nil
}
```

### 3. 配置加密

对于敏感配置信息，建议进行加密存储：

```go
func encryptConfig(config bluetooth.BluetoothConfig, key []byte) ([]byte, error) {
    data, err := json.Marshal(config)
    if err != nil {
        return nil, err
    }

    // 使用AES加密
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }

    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }

    ciphertext := gcm.Seal(nonce, nonce, data, nil)
    return ciphertext, nil
}
```

## 故障排除

### 常见配置错误

1. **UUID 格式错误**

   ```
   错误: 无效的UUID格式
   解决: 使用标准UUID格式 "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
   ```

2. **超时时间过短**

   ```
   错误: 连接超时时间过短导致连接失败
   解决: 增加ConnectTimeout和AcceptTimeout值
   ```

3. **连接数限制**
   ```
   错误: 达到最大连接数限制
   解决: 增加MaxConnections值或实现连接池管理
   ```

### 配置调试

启用配置调试模式：

```go
config := bluetooth.DefaultConfig()
config.LogConfig.Level = "debug"

// 打印配置信息
configJSON, _ := json.MarshalIndent(config, "", "  ")
log.Printf("当前配置:\n%s", configJSON)
```
