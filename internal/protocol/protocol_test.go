package protocol

import (
	"context"
	"testing"
	"time"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// TestProtocolFactory 测试协议工厂
func TestProtocolFactory(t *testing.T) {
	factory := &ProtocolFactory{}

	// 测试创建RFCOMM处理器
	rfcommConfig := DefaultProtocolConfig(bluetooth.ProtocolRFCOMM)
	rfcommHandler, err := factory.CreateHandler(bluetooth.ProtocolRFCOMM, rfcommConfig)
	if err != nil {
		t.Fatalf("创建RFCOMM处理器失败: %v", err)
	}

	if rfcommHandler.GetProtocolName() != bluetooth.ProtocolRFCOMM {
		t.Errorf("RFCOMM协议名称不匹配，期望: %s, 实际: %s",
			bluetooth.ProtocolRFCOMM, rfcommHandler.GetProtocolName())
	}

	// 测试创建L2CAP处理器
	l2capConfig := DefaultProtocolConfig(bluetooth.ProtocolL2CAP)
	l2capHandler, err := factory.CreateHandler(bluetooth.ProtocolL2CAP, l2capConfig)
	if err != nil {
		t.Fatalf("创建L2CAP处理器失败: %v", err)
	}

	if l2capHandler.GetProtocolName() != bluetooth.ProtocolL2CAP {
		t.Errorf("L2CAP协议名称不匹配，期望: %s, 实际: %s",
			bluetooth.ProtocolL2CAP, l2capHandler.GetProtocolName())
	}

	// 测试不支持的协议
	_, err = factory.CreateHandler("UNKNOWN", rfcommConfig)
	if err == nil {
		t.Error("创建不支持的协议处理器应该失败")
	}
}

// TestRFCOMMHandler 测试RFCOMM协议处理器
func TestRFCOMMHandler(t *testing.T) {
	config := DefaultProtocolConfig(bluetooth.ProtocolRFCOMM)
	handler := &RFCOMMHandler{}

	// 测试初始化
	err := handler.Initialize(config)
	if err != nil {
		t.Fatalf("RFCOMM处理器初始化失败: %v", err)
	}

	// 测试MTU设置
	originalMTU := handler.GetMTU()
	newMTU := 1024
	err = handler.SetMTU(newMTU)
	if err != nil {
		t.Fatalf("设置MTU失败: %v", err)
	}

	if handler.GetMTU() != newMTU {
		t.Errorf("MTU设置不正确，期望: %d, 实际: %d", newMTU, handler.GetMTU())
	}

	// 测试无效MTU
	err = handler.SetMTU(0)
	if err == nil {
		t.Error("设置无效MTU应该失败")
	}

	// 恢复MTU
	handler.SetMTU(originalMTU)

	// 测试消息验证
	validData := []byte("有效的测试数据")
	err = handler.ValidateMessage(validData)
	if err != nil {
		t.Errorf("验证有效消息失败: %v", err)
	}

	// 测试空消息验证
	err = handler.ValidateMessage(nil)
	if err == nil {
		t.Error("验证空消息应该失败")
	}

	err = handler.ValidateMessage([]byte{})
	if err == nil {
		t.Error("验证空消息应该失败")
	}

	// 测试超大消息验证
	largeData := make([]byte, handler.GetMTU()+1)
	err = handler.ValidateMessage(largeData)
	if err == nil {
		t.Error("验证超大消息应该失败")
	}

	// 测试关闭
	err = handler.Shutdown()
	if err != nil {
		t.Errorf("RFCOMM处理器关闭失败: %v", err)
	}
}

// TestL2CAPHandler 测试L2CAP协议处理器
func TestL2CAPHandler(t *testing.T) {
	config := DefaultProtocolConfig(bluetooth.ProtocolL2CAP)
	handler := &L2CAPHandler{}

	// 测试初始化
	err := handler.Initialize(config)
	if err != nil {
		t.Fatalf("L2CAP处理器初始化失败: %v", err)
	}

	// 测试MTU设置
	newMTU := 2048
	err = handler.SetMTU(newMTU)
	if err != nil {
		t.Fatalf("设置MTU失败: %v", err)
	}

	if handler.GetMTU() != newMTU {
		t.Errorf("MTU设置不正确，期望: %d, 实际: %d", newMTU, handler.GetMTU())
	}

	// 测试消息验证
	validData := []byte("有效的L2CAP测试数据")
	err = handler.ValidateMessage(validData)
	if err != nil {
		t.Errorf("验证有效消息失败: %v", err)
	}

	// 测试关闭
	err = handler.Shutdown()
	if err != nil {
		t.Errorf("L2CAP处理器关闭失败: %v", err)
	}
}

// TestRFCOMMEncodeDecodeMessage 测试RFCOMM消息编码解码
func TestRFCOMMEncodeDecodeMessage(t *testing.T) {
	config := DefaultProtocolConfig(bluetooth.ProtocolRFCOMM)
	handler := &RFCOMMHandler{}
	err := handler.Initialize(config)
	if err != nil {
		t.Fatalf("RFCOMM处理器初始化失败: %v", err)
	}

	// 测试消息编码解码
	testMessage := map[string]any{
		"type":    "test",
		"content": "这是一条测试消息",
		"number":  42,
	}

	// 编码消息
	encodedData, err := handler.EncodeMessage(testMessage)
	if err != nil {
		t.Fatalf("编码消息失败: %v", err)
	}

	if len(encodedData) == 0 {
		t.Error("编码后的数据不能为空")
	}

	// 解码消息
	decodedMessage, err := handler.DecodeMessage(encodedData)
	if err != nil {
		t.Fatalf("解码消息失败: %v", err)
	}

	// 验证解码结果
	decodedMap, ok := decodedMessage.(map[string]any)
	if !ok {
		t.Fatal("解码后的消息类型不正确")
	}

	if decodedMap["type"] != testMessage["type"] {
		t.Errorf("解码后的type字段不匹配，期望: %v, 实际: %v",
			testMessage["type"], decodedMap["type"])
	}

	if decodedMap["content"] != testMessage["content"] {
		t.Errorf("解码后的content字段不匹配，期望: %v, 实际: %v",
			testMessage["content"], decodedMap["content"])
	}
}

// TestL2CAPEncodeDecodeMessage 测试L2CAP消息编码解码
func TestL2CAPEncodeDecodeMessage(t *testing.T) {
	config := DefaultProtocolConfig(bluetooth.ProtocolL2CAP)
	handler := &L2CAPHandler{}
	err := handler.Initialize(config)
	if err != nil {
		t.Fatalf("L2CAP处理器初始化失败: %v", err)
	}

	// 测试消息编码解码
	testMessage := map[string]any{
		"protocol":  "L2CAP",
		"data":      []byte("二进制数据"),
		"timestamp": time.Now().Unix(),
	}

	// 编码消息
	encodedData, err := handler.EncodeMessage(testMessage)
	if err != nil {
		t.Fatalf("编码消息失败: %v", err)
	}

	// 解码消息
	decodedMessage, err := handler.DecodeMessage(encodedData)
	if err != nil {
		t.Fatalf("解码消息失败: %v", err)
	}

	// 验证解码结果
	decodedMap, ok := decodedMessage.(map[string]any)
	if !ok {
		t.Fatal("解码后的消息类型不正确")
	}

	if decodedMap["protocol"] != testMessage["protocol"] {
		t.Errorf("解码后的protocol字段不匹配，期望: %v, 实际: %v",
			testMessage["protocol"], decodedMap["protocol"])
	}
}

// TestProtocolProcessor 测试协议处理器
func TestProtocolProcessor(t *testing.T) {
	config := DefaultProtocolConfig(bluetooth.ProtocolRFCOMM)
	handler := &RFCOMMHandler{}
	err := handler.Initialize(config)
	if err != nil {
		t.Fatalf("RFCOMM处理器初始化失败: %v", err)
	}

	processor := NewProtocolProcessor(handler)

	// 测试获取处理器
	if processor.GetHandler() != handler {
		t.Error("获取的处理器不匹配")
	}

	// 测试设置重试策略
	customStrategy := bluetooth.ErrorRecoveryStrategy{
		MaxRetries:    5,
		RetryInterval: 500 * time.Millisecond,
		BackoffFactor: 1.5,
		MaxInterval:   10 * time.Second,
	}
	processor.SetRetryStrategy(customStrategy)

	// 测试消息处理
	testMessage := map[string]any{
		"test": "processor message",
	}

	ctx := context.Background()

	// 编码消息
	encodedData, err := processor.EncodeMessageWithRetry(ctx, testMessage)
	if err != nil {
		t.Fatalf("编码消息失败: %v", err)
	}

	// 处理消息
	decodedMessage, err := processor.ProcessMessage(ctx, encodedData)
	if err != nil {
		t.Fatalf("处理消息失败: %v", err)
	}

	// 验证结果
	decodedMap, ok := decodedMessage.(map[string]any)
	if !ok {
		t.Fatal("处理后的消息类型不正确")
	}

	if decodedMap["test"] != testMessage["test"] {
		t.Errorf("处理后的消息内容不匹配，期望: %v, 实际: %v",
			testMessage["test"], decodedMap["test"])
	}
}

// TestProtocolProcessor_InvalidData 测试协议处理器处理无效数据
func TestProtocolProcessor_InvalidData(t *testing.T) {
	config := DefaultProtocolConfig(bluetooth.ProtocolRFCOMM)
	handler := &RFCOMMHandler{}
	err := handler.Initialize(config)
	if err != nil {
		t.Fatalf("RFCOMM处理器初始化失败: %v", err)
	}

	processor := NewProtocolProcessor(handler)
	ctx := context.Background()

	// 测试处理无效数据
	invalidData := []byte("这不是有效的协议数据")
	_, err = processor.ProcessMessage(ctx, invalidData)
	if err == nil {
		t.Error("处理无效数据应该失败")
	}

	// 测试处理空数据
	_, err = processor.ProcessMessage(ctx, nil)
	if err == nil {
		t.Error("处理空数据应该失败")
	}
}

// TestDefaultProtocolConfig 测试默认协议配置
func TestDefaultProtocolConfig(t *testing.T) {
	// 测试RFCOMM默认配置
	rfcommConfig := DefaultProtocolConfig(bluetooth.ProtocolRFCOMM)
	if rfcommConfig.ProtocolType != bluetooth.ProtocolRFCOMM {
		t.Errorf("RFCOMM协议类型不匹配，期望: %s, 实际: %s",
			bluetooth.ProtocolRFCOMM, rfcommConfig.ProtocolType)
	}

	if rfcommConfig.MaxMessageSize <= 0 {
		t.Error("最大消息大小应该大于0")
	}

	if rfcommConfig.Timeout <= 0 {
		t.Error("超时时间应该大于0")
	}

	// 测试L2CAP默认配置
	l2capConfig := DefaultProtocolConfig(bluetooth.ProtocolL2CAP)
	if l2capConfig.ProtocolType != bluetooth.ProtocolL2CAP {
		t.Errorf("L2CAP协议类型不匹配，期望: %s, 实际: %s",
			bluetooth.ProtocolL2CAP, l2capConfig.ProtocolType)
	}
}

// TestMessageFrame 测试消息帧结构
func TestMessageFrame(t *testing.T) {
	// 创建测试消息帧
	testPayload := []byte("测试载荷数据")
	frame := MessageFrame{
		Header: FrameHeader{
			Version:     1,
			MessageType: uint8(bluetooth.MessageTypeData),
			Length:      uint32(len(testPayload)),
			Sequence:    123,
			Flags:       0x0001,
			Reserved:    0,
		},
		Payload: testPayload,
		Footer: FrameFooter{
			Checksum: 0x12345678,
			Magic:    0xDEADBEEF,
		},
	}

	// 验证帧结构
	if frame.Header.Version != 1 {
		t.Errorf("帧版本不正确，期望: 1, 实际: %d", frame.Header.Version)
	}

	if frame.Header.MessageType != uint8(bluetooth.MessageTypeData) {
		t.Errorf("消息类型不正确，期望: %v, 实际: %v",
			uint8(bluetooth.MessageTypeData), frame.Header.MessageType)
	}

	if frame.Header.Length != uint32(len(testPayload)) {
		t.Errorf("载荷长度不正确，期望: %d, 实际: %d",
			len(testPayload), frame.Header.Length)
	}

	if string(frame.Payload) != string(testPayload) {
		t.Errorf("载荷数据不匹配，期望: %s, 实际: %s",
			string(testPayload), string(frame.Payload))
	}

	if frame.Footer.Magic != 0xDEADBEEF {
		t.Errorf("魔数不正确，期望: 0xDEADBEEF, 实际: 0x%X", frame.Footer.Magic)
	}
}
