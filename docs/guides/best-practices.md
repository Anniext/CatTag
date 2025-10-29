# 最佳实践指南

本指南提供了使用 CatTag 蓝牙通信库的最佳实践和推荐模式。

## 架构设计最佳实践

### 1. 组件生命周期管理

**推荐做法：**

```go
type BluetoothService struct {
    component bluetooth.BluetoothComponent[MyMessage]
    ctx       context.Context
    cancel    context.CancelFunc
    wg        sync.WaitGroup
}

func NewBluetoothService() *BluetoothService {
    ctx, cancel := context.WithCancel(context.Background())

    component, err := bluetooth.NewServerBuilder("service-uuid").
        EnableSecurity().
        EnableHealthCheck().
        Build()

    if err != nil {
        cancel()
        return nil
    }

    return &BluetoothService{
        component: component,
        ctx:       ctx,
        cancel:    cancel,
    }
}

func (s *BluetoothService) Start() error {
    if err := s.component.Start(s.ctx); err != nil {
        return err
    }

    // 启动消息处理
    s.wg.Add(1)
    go s.messageHandler()

    return nil
}

func (s *BluetoothService) Stop() error {
    s.cancel()
    s.wg.Wait()
    return s.component.Stop(context.Background())
}
```

### 2. 错误处理策略

**推荐做法：**

```go
func (s *BluetoothService) sendWithRetry(deviceID string, message MyMessage) error {
    const maxRetries = 3
    const retryDelay = time.Second

    for i := 0; i < maxRetries; i++ {
        err := s.component.Send(s.ctx, deviceID, message)
        if err == nil {
            return nil
        }

        // 检查错误类型
        var btErr *bluetooth.BluetoothError
        if errors.As(err, &btErr) {
            switch btErr.Code {
            case bluetooth.ErrCodeDeviceNotFound:
                // 设备不存在，不需要重试
                return err
            case bluetooth.ErrCodeConnectionFailed:
                // 连接失败，可以重试
                if i < maxRetries-1 {
                    time.Sleep(retryDelay * time.Duration(i+1))
                    continue
                }
            }
        }

        return err
    }

    return fmt.Errorf("发送失败，已重试 %d 次", maxRetries)
}
```

### 3. 资源管理

**推荐做法：**

```go
type ResourceManager struct {
    connections map[string]bluetooth.Connection
    mu          sync.RWMutex
    maxIdle     time.Duration
    cleanup     *time.Ticker
}

func NewResourceManager() *ResourceManager {
    rm := &ResourceManager{
        connections: make(map[string]bluetooth.Connection),
        maxIdle:     5 * time.Minute,
        cleanup:     time.NewTicker(1 * time.Minute),
    }

    go rm.cleanupLoop()
    return rm
}

func (rm *ResourceManager) cleanupLoop() {
    for range rm.cleanup.C {
        rm.cleanupIdleConnections()
    }
}

func (rm *ResourceManager) cleanupIdleConnections() {
    rm.mu.Lock()
    defer rm.mu.Unlock()

    for id, conn := range rm.connections {
        if !conn.IsActive() {
            conn.Close()
            delete(rm.connections, id)
        }
    }
}
```

## 性能优化最佳实践

### 1. 连接池优化

**推荐配置：**

```go
config := bluetooth.DefaultConfig()

// 根据应用需求调整连接池大小
config.PoolConfig.MaxConnections = 20
config.PoolConfig.MinConnections = 2
config.PoolConfig.MaxIdleTime = 5 * time.Minute
config.PoolConfig.EnableLoadBalance = true
config.PoolConfig.BalanceStrategy = "round_robin"
```

### 2. 消息批处理

**推荐做法：**

```go
type MessageBatcher struct {
    messages []MyMessage
    mu       sync.Mutex
    timer    *time.Timer
    batchSize int
    timeout   time.Duration
}

func (mb *MessageBatcher) AddMessage(msg MyMessage) {
    mb.mu.Lock()
    defer mb.mu.Unlock()

    mb.messages = append(mb.messages, msg)

    if len(mb.messages) >= mb.batchSize {
        mb.flush()
    } else if mb.timer == nil {
        mb.timer = time.AfterFunc(mb.timeout, mb.flush)
    }
}

func (mb *MessageBatcher) flush() {
    if mb.timer != nil {
        mb.timer.Stop()
        mb.timer = nil
    }

    if len(mb.messages) > 0 {
        // 批量发送消息
        mb.sendBatch(mb.messages)
        mb.messages = mb.messages[:0]
    }
}
```

### 3. 内存优化

**推荐做法：**

```go
// 使用对象池复用消息对象
var messagePool = sync.Pool{
    New: func() interface{} {
        return &MyMessage{}
    },
}

func getMessage() *MyMessage {
    return messagePool.Get().(*MyMessage)
}

func putMessage(msg *MyMessage) {
    // 重置消息内容
    msg.Content = ""
    msg.Sender = ""
    messagePool.Put(msg)
}

// 使用缓冲区池
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 1024)
    },
}
```

## 安全最佳实践

### 1. 认证和授权

**推荐做法：**

```go
type SecurityHandler struct {
    trustedDevices map[string]bool
    deviceKeys     map[string][]byte
    mu             sync.RWMutex
}

func (sh *SecurityHandler) AuthenticateDevice(deviceID string, credentials []byte) error {
    sh.mu.RLock()
    defer sh.mu.RUnlock()

    // 检查设备是否在信任列表中
    if !sh.trustedDevices[deviceID] {
        return fmt.Errorf("设备未授权: %s", deviceID)
    }

    // 验证设备凭据
    expectedKey := sh.deviceKeys[deviceID]
    if !bytes.Equal(credentials, expectedKey) {
        return fmt.Errorf("认证失败: %s", deviceID)
    }

    return nil
}

func (sh *SecurityHandler) AddTrustedDevice(deviceID string, key []byte) {
    sh.mu.Lock()
    defer sh.mu.Unlock()

    sh.trustedDevices[deviceID] = true
    sh.deviceKeys[deviceID] = key
}
```

### 2. 数据加密

**推荐做法：**

```go
type EncryptionManager struct {
    cipher cipher.AEAD
}

func NewEncryptionManager(key []byte) (*EncryptionManager, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }

    return &EncryptionManager{cipher: gcm}, nil
}

func (em *EncryptionManager) Encrypt(data []byte) ([]byte, error) {
    nonce := make([]byte, em.cipher.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }

    ciphertext := em.cipher.Seal(nonce, nonce, data, nil)
    return ciphertext, nil
}

func (em *EncryptionManager) Decrypt(ciphertext []byte) ([]byte, error) {
    nonceSize := em.cipher.NonceSize()
    if len(ciphertext) < nonceSize {
        return nil, fmt.Errorf("密文太短")
    }

    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    return em.cipher.Open(nil, nonce, ciphertext, nil)
}
```

## 测试最佳实践

### 1. 单元测试

**推荐做法：**

```go
func TestBluetoothService_SendMessage(t *testing.T) {
    // 使用模拟适配器
    mockAdapter := bluetooth.NewMockAdapter()

    // 创建测试组件
    component, err := bluetooth.NewBluetoothComponent[TestMessage](
        bluetooth.DefaultConfig(),
    )
    require.NoError(t, err)

    // 启动组件
    ctx := context.Background()
    err = component.Start(ctx)
    require.NoError(t, err)
    defer component.Stop(ctx)

    // 测试消息发送
    message := TestMessage{Content: "test"}
    err = component.Send(ctx, "test-device", message)
    assert.NoError(t, err)
}
```

### 2. 集成测试

**推荐做法：**

```go
func TestBluetoothIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("跳过集成测试")
    }

    // 创建服务端
    server, err := bluetooth.NewServerBuilder("test-uuid").Build()
    require.NoError(t, err)

    // 创建客户端
    client, err := bluetooth.NewClientBuilder().Build()
    require.NoError(t, err)

    // 启动服务端
    ctx := context.Background()
    err = server.Start(ctx)
    require.NoError(t, err)
    defer server.Stop(ctx)

    // 启动客户端
    err = client.Start(ctx)
    require.NoError(t, err)
    defer client.Stop(ctx)

    // 测试端到端通信
    testEndToEndCommunication(t, server, client)
}
```

### 3. 性能测试

**推荐做法：**

```go
func BenchmarkMessageSending(b *testing.B) {
    component, err := bluetooth.NewServerBuilder("bench-uuid").Build()
    if err != nil {
        b.Fatal(err)
    }

    ctx := context.Background()
    component.Start(ctx)
    defer component.Stop(ctx)

    message := TestMessage{Content: "benchmark"}

    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            component.Send(ctx, "bench-device", message)
        }
    })
}
```

## 监控和日志最佳实践

### 1. 结构化日志

**推荐做法：**

```go
import "log/slog"

type BluetoothLogger struct {
    logger *slog.Logger
}

func NewBluetoothLogger() *BluetoothLogger {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))

    return &BluetoothLogger{logger: logger}
}

func (bl *BluetoothLogger) LogConnection(deviceID string, status string) {
    bl.logger.Info("连接状态变化",
        "device_id", deviceID,
        "status", status,
        "timestamp", time.Now(),
    )
}

func (bl *BluetoothLogger) LogMessage(messageID string, deviceID string, size int) {
    bl.logger.Info("消息传输",
        "message_id", messageID,
        "device_id", deviceID,
        "size_bytes", size,
        "timestamp", time.Now(),
    )
}
```

### 2. 指标收集

**推荐做法：**

```go
type MetricsCollector struct {
    connectionCount   int64
    messagesSent      int64
    messagesReceived  int64
    errorCount        int64
    mu                sync.RWMutex
}

func (mc *MetricsCollector) IncrementConnections() {
    atomic.AddInt64(&mc.connectionCount, 1)
}

func (mc *MetricsCollector) IncrementMessagesSent() {
    atomic.AddInt64(&mc.messagesSent, 1)
}

func (mc *MetricsCollector) GetMetrics() map[string]int64 {
    return map[string]int64{
        "connections":        atomic.LoadInt64(&mc.connectionCount),
        "messages_sent":      atomic.LoadInt64(&mc.messagesSent),
        "messages_received":  atomic.LoadInt64(&mc.messagesReceived),
        "errors":            atomic.LoadInt64(&mc.errorCount),
    }
}
```

## 部署最佳实践

### 1. 配置管理

**推荐做法：**

```go
type ConfigLoader struct {
    env string
}

func (cl *ConfigLoader) LoadConfig() bluetooth.BluetoothConfig {
    config := bluetooth.DefaultConfig()

    // 从环境变量加载
    cl.loadFromEnv(&config)

    // 从配置文件加载
    cl.loadFromFile(&config)

    // 验证配置
    if err := config.Validate(); err != nil {
        log.Fatal("配置验证失败:", err)
    }

    return config
}

func (cl *ConfigLoader) loadFromEnv(config *bluetooth.BluetoothConfig) {
    if maxConn := os.Getenv("BT_MAX_CONNECTIONS"); maxConn != "" {
        if n, err := strconv.Atoi(maxConn); err == nil {
            config.ServerConfig.MaxConnections = n
        }
    }
}
```

### 2. 健康检查端点

**推荐做法：**

```go
type HealthCheckHandler struct {
    component bluetooth.BluetoothComponent[any]
}

func (h *HealthCheckHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    status := h.component.GetStatus()

    health := map[string]interface{}{
        "status":    status.String(),
        "timestamp": time.Now(),
    }

    if status == bluetooth.StatusRunning {
        w.WriteHeader(http.StatusOK)
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
    }

    json.NewEncoder(w).Encode(health)
}

// 注册健康检查端点
http.Handle("/health", &HealthCheckHandler{component: component})
```

### 3. 优雅关闭

**推荐做法：**

```go
func main() {
    component, err := bluetooth.NewServerBuilder("service-uuid").Build()
    if err != nil {
        log.Fatal(err)
    }

    // 启动组件
    ctx := context.Background()
    if err := component.Start(ctx); err != nil {
        log.Fatal(err)
    }

    // 设置信号处理
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    // 等待信号
    <-sigChan
    log.Println("收到关闭信号，开始优雅关闭...")

    // 创建关闭上下文
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // 优雅关闭
    if err := component.Stop(shutdownCtx); err != nil {
        log.Printf("关闭失败: %v", err)
    } else {
        log.Println("应用已成功关闭")
    }
}
```

## 常见反模式

### 避免的做法

1. **不处理错误**

   ```go
   // 错误做法
   component.Send(ctx, deviceID, message) // 忽略错误

   // 正确做法
   if err := component.Send(ctx, deviceID, message); err != nil {
       log.Printf("发送失败: %v", err)
   }
   ```

2. **阻塞主线程**

   ```go
   // 错误做法
   for msg := range component.Receive(ctx) {
       processMessage(msg) // 可能阻塞
   }

   // 正确做法
   go func() {
       for msg := range component.Receive(ctx) {
           go processMessage(msg) // 异步处理
       }
   }()
   ```

3. **资源泄漏**

   ```go
   // 错误做法
   component.Start(ctx)
   // 忘记调用 Stop()

   // 正确做法
   component.Start(ctx)
   defer component.Stop(ctx)
   ```

遵循这些最佳实践将帮助您构建更稳定、高性能和可维护的蓝牙应用程序。
