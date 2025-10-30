package main

import (
	"os"
	"testing"
	"time"

	"github.com/Anniext/CatTag/cmd"
	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// TestDefaultConfig 测试默认配置
func TestDefaultConfig(t *testing.T) {
	config := bluetooth.DefaultConfig()

	// 验证服务端配置
	if config.ServerConfig.ServiceUUID == "" {
		t.Error("服务端ServiceUUID不能为空")
	}
	if config.ServerConfig.MaxConnections <= 0 {
		t.Error("服务端最大连接数必须大于0")
	}
	if config.ServerConfig.AcceptTimeout <= 0 {
		t.Error("服务端接受超时时间必须大于0")
	}

	// 验证客户端配置
	if config.ClientConfig.ScanTimeout <= 0 {
		t.Error("客户端扫描超时时间必须大于0")
	}
	if config.ClientConfig.ConnectTimeout <= 0 {
		t.Error("客户端连接超时时间必须大于0")
	}
	if config.ClientConfig.RetryAttempts < 0 {
		t.Error("客户端重试次数不能为负数")
	}

	// 验证安全配置
	if config.SecurityConfig.KeySize <= 0 {
		t.Error("密钥大小必须大于0")
	}
	if config.SecurityConfig.SessionTimeout <= 0 {
		t.Error("会话超时时间必须大于0")
	}

	// 验证健康检查配置
	if config.HealthConfig.CheckInterval <= 0 {
		t.Error("健康检查间隔必须大于0")
	}
	if config.HealthConfig.HeartbeatInterval <= 0 {
		t.Error("心跳间隔必须大于0")
	}

	// 验证连接池配置
	if config.PoolConfig.MaxConnections <= 0 {
		t.Error("连接池最大连接数必须大于0")
	}
	if config.PoolConfig.MinConnections < 0 {
		t.Error("连接池最小连接数不能为负数")
	}
}

// TestConfigValidation 测试配置验证
func TestConfigValidation(t *testing.T) {
	// 测试有效配置
	validConfig := bluetooth.DefaultConfig()
	if err := validConfig.Validate(); err != nil {
		t.Errorf("有效配置验证失败: %v", err)
	}

	// 测试无效的服务端配置
	invalidServerConfig := bluetooth.DefaultConfig()
	invalidServerConfig.ServerConfig.MaxConnections = 0
	if err := invalidServerConfig.Validate(); err == nil {
		t.Error("应该检测到无效的服务端最大连接数")
	}

	// 测试无效的客户端配置
	invalidClientConfig := bluetooth.DefaultConfig()
	invalidClientConfig.ClientConfig.ScanTimeout = 0
	if err := invalidClientConfig.Validate(); err == nil {
		t.Error("应该检测到无效的客户端扫描超时时间")
	}

	// 测试无效的安全配置
	invalidSecurityConfig := bluetooth.DefaultConfig()
	invalidSecurityConfig.SecurityConfig.KeySize = 0
	if err := invalidSecurityConfig.Validate(); err == nil {
		t.Error("应该检测到无效的密钥大小")
	}
}

// TestBluetoothError 测试蓝牙错误类型
func TestBluetoothError(t *testing.T) {
	// 测试基本错误创建
	err := bluetooth.NewBluetoothError(bluetooth.ErrCodeDeviceNotFound, "设备未找到")
	if err.Code != bluetooth.ErrCodeDeviceNotFound {
		t.Errorf("错误代码不匹配，期望: %d, 实际: %d", bluetooth.ErrCodeDeviceNotFound, err.Code)
	}
	if err.Message != "设备未找到" {
		t.Errorf("错误消息不匹配，期望: %s, 实际: %s", "设备未找到", err.Message)
	}

	// 测试带设备信息的错误
	deviceErr := bluetooth.NewBluetoothErrorWithDevice(
		bluetooth.ErrCodeConnectionFailed,
		"连接失败",
		"device123",
		"connect",
	)
	if deviceErr.DeviceID != "device123" {
		t.Errorf("设备ID不匹配，期望: %s, 实际: %s", "device123", deviceErr.DeviceID)
	}
	if deviceErr.Operation != "connect" {
		t.Errorf("操作类型不匹配，期望: %s, 实际: %s", "connect", deviceErr.Operation)
	}

	// 测试错误字符串格式
	expectedMsg := "bluetooth error [2000]: 连接失败 (device: device123, operation: connect)"
	if deviceErr.Error() != expectedMsg {
		t.Errorf("错误字符串格式不匹配，期望: %s, 实际: %s", expectedMsg, deviceErr.Error())
	}
}

// TestMessageTypes 测试消息类型
func TestMessageTypes(t *testing.T) {
	// 测试消息类型字符串表示
	testCases := []struct {
		msgType  bluetooth.MessageType
		expected string
	}{
		{bluetooth.MessageTypeData, "data"},
		{bluetooth.MessageTypeControl, "control"},
		{bluetooth.MessageTypeHeartbeat, "heartbeat"},
		{bluetooth.MessageTypeAuth, "auth"},
		{bluetooth.MessageTypeError, "error"},
		{bluetooth.MessageTypeAck, "ack"},
	}

	for _, tc := range testCases {
		if tc.msgType.String() != tc.expected {
			t.Errorf("消息类型字符串不匹配，类型: %d, 期望: %s, 实际: %s",
				tc.msgType, tc.expected, tc.msgType.String())
		}
	}
}

// TestPriority 测试优先级类型
func TestPriority(t *testing.T) {
	// 测试优先级字符串表示
	testCases := []struct {
		priority bluetooth.Priority
		expected string
	}{
		{bluetooth.PriorityLow, "low"},
		{bluetooth.PriorityNormal, "normal"},
		{bluetooth.PriorityHigh, "high"},
		{bluetooth.PriorityCritical, "critical"},
	}

	for _, tc := range testCases {
		if tc.priority.String() != tc.expected {
			t.Errorf("优先级字符串不匹配，优先级: %d, 期望: %s, 实际: %s",
				tc.priority, tc.expected, tc.priority.String())
		}
	}
}

// TestComponentStatus 测试组件状态
func TestComponentStatus(t *testing.T) {
	// 测试组件状态字符串表示
	testCases := []struct {
		status   bluetooth.ComponentStatus
		expected string
	}{
		{bluetooth.StatusStopped, "stopped"},
		{bluetooth.StatusStarting, "starting"},
		{bluetooth.StatusRunning, "running"},
		{bluetooth.StatusStopping, "stopping"},
		{bluetooth.StatusError, "error"},
	}

	for _, tc := range testCases {
		if tc.status.String() != tc.expected {
			t.Errorf("组件状态字符串不匹配，状态: %d, 期望: %s, 实际: %s",
				tc.status, tc.expected, tc.status.String())
		}
	}
}

// TestMessage 测试泛型消息结构
func TestMessage(t *testing.T) {
	// 测试字符串消息
	stringMsg := bluetooth.Message[string]{
		ID:      "msg001",
		Type:    bluetooth.MessageTypeData,
		Payload: "Hello, Bluetooth!",
		Metadata: bluetooth.MessageMetadata{
			SenderID:   "sender001",
			ReceiverID: "receiver001",
			Priority:   bluetooth.PriorityNormal,
			TTL:        30 * time.Second,
			Encrypted:  false,
			Compressed: false,
		},
		Timestamp: time.Now(),
	}

	if stringMsg.Payload != "Hello, Bluetooth!" {
		t.Errorf("字符串消息载荷不匹配，期望: %s, 实际: %s",
			"Hello, Bluetooth!", stringMsg.Payload)
	}

	// 测试结构体消息
	type CustomData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	structMsg := bluetooth.Message[CustomData]{
		ID:   "msg002",
		Type: bluetooth.MessageTypeData,
		Payload: CustomData{
			Name:  "test",
			Value: 42,
		},
		Metadata: bluetooth.MessageMetadata{
			SenderID:   "sender002",
			ReceiverID: "receiver002",
			Priority:   bluetooth.PriorityHigh,
			TTL:        60 * time.Second,
			Encrypted:  true,
			Compressed: true,
		},
		Timestamp: time.Now(),
	}

	if structMsg.Payload.Name != "test" || structMsg.Payload.Value != 42 {
		t.Errorf("结构体消息载荷不匹配，期望: {Name: test, Value: 42}, 实际: %+v",
			structMsg.Payload)
	}
}

// TestComponentBuilder 测试组件构建器
func TestComponentBuilder(t *testing.T) {
	builder := bluetooth.NewComponentBuilder()

	// 测试链式配置
	serverConfig := bluetooth.ServerConfig{
		ServiceUUID:    "test-uuid",
		ServiceName:    "Test Service",
		MaxConnections: 5,
		AcceptTimeout:  10 * time.Second,
		RequireAuth:    true,
	}

	clientConfig := bluetooth.ClientConfig{
		ScanTimeout:    5 * time.Second,
		ConnectTimeout: 15 * time.Second,
		RetryAttempts:  2,
		RetryInterval:  1 * time.Second,
		AutoReconnect:  false,
	}

	config := builder.
		WithServerConfig(serverConfig).
		WithClientConfig(clientConfig).
		GetConfig()

	// 验证配置
	if config.ServerConfig.ServiceUUID != "test-uuid" {
		t.Errorf("服务端UUID不匹配，期望: %s, 实际: %s",
			"test-uuid", config.ServerConfig.ServiceUUID)
	}
	if config.ClientConfig.ScanTimeout != 5*time.Second {
		t.Errorf("客户端扫描超时不匹配，期望: %v, 实际: %v",
			5*time.Second, config.ClientConfig.ScanTimeout)
	}

	// 测试配置验证
	if err := builder.Validate(); err != nil {
		t.Errorf("构建器配置验证失败: %v", err)
	}
}

// TestCobraCommands 测试 Cobra 命令结构
func TestCobraCommands(t *testing.T) {
	// 测试根命令是否正确初始化
	rootCmd := cmd.GetRootCommand()
	if rootCmd == nil {
		t.Error("根命令不应该为 nil")
	}

	// 检查命令名称
	if rootCmd.Use != "cattag" {
		t.Errorf("根命令名称不匹配，期望: cattag, 实际: %s", rootCmd.Use)
	}

	// 检查是否有子命令
	subCommands := rootCmd.Commands()
	expectedCommands := []string{"server", "client", "both", "scan", "version"}

	if len(subCommands) < len(expectedCommands) {
		t.Errorf("子命令数量不足，期望至少: %d, 实际: %d", len(expectedCommands), len(subCommands))
	}

	// 检查特定子命令是否存在
	commandMap := make(map[string]bool)
	for _, cmd := range subCommands {
		commandMap[cmd.Use] = true
	}

	for _, expectedCmd := range expectedCommands {
		if !commandMap[expectedCmd] {
			t.Errorf("缺少子命令: %s", expectedCmd)
		}
	}
}

// TestCommandLineArgs 测试命令行参数解析
func TestCommandLineArgs(t *testing.T) {
	// 保存原始命令行参数
	originalArgs := os.Args

	// 测试版本命令
	os.Args = []string{"cattag", "version"}
	// 这里不能直接调用 Execute()，因为它会退出程序
	// 在实际测试中，可能需要使用更复杂的测试策略

	// 恢复原始参数
	os.Args = originalArgs
}

// TestCobraConfigValidation 测试 Cobra 命令的配置验证
func TestCobraConfigValidation(t *testing.T) {
	// 测试有效配置
	validConfig := bluetooth.DefaultConfig()
	if err := validConfig.Validate(); err != nil {
		t.Errorf("有效配置验证失败: %v", err)
	}

	// 测试无效的服务端配置
	invalidServerConfig := bluetooth.DefaultConfig()
	invalidServerConfig.ServerConfig.MaxConnections = 0
	if err := invalidServerConfig.Validate(); err == nil {
		t.Error("应该检测到无效的服务端最大连接数")
	}

	// 测试无效的客户端配置
	invalidClientConfig := bluetooth.DefaultConfig()
	invalidClientConfig.ClientConfig.ScanTimeout = 0
	if err := invalidClientConfig.Validate(); err == nil {
		t.Error("应该检测到无效的客户端扫描超时时间")
	}

	// 测试无效的安全配置
	invalidSecurityConfig := bluetooth.DefaultConfig()
	invalidSecurityConfig.SecurityConfig.KeySize = 0
	if err := invalidSecurityConfig.Validate(); err == nil {
		t.Error("应该检测到无效的密钥大小")
	}
}
