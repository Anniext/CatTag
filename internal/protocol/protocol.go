package protocol

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"sync"
	"time"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// ProtocolHandler 协议处理器接口
type ProtocolHandler interface {
	// GetProtocolName 获取协议名称
	GetProtocolName() string
	// Initialize 初始化协议处理器
	Initialize(config ProtocolConfig) error
	// Shutdown 关闭协议处理器
	Shutdown() error
	// EncodeMessage 编码消息
	EncodeMessage(message any) ([]byte, error)
	// DecodeMessage 解码消息
	DecodeMessage(data []byte) (any, error)
	// ValidateMessage 验证消息格式
	ValidateMessage(data []byte) error
	// GetMTU 获取最大传输单元
	GetMTU() int
	// SetMTU 设置最大传输单元
	SetMTU(mtu int) error
}

// RFCOMMHandler RFCOMM协议处理器
type RFCOMMHandler struct {
	config   ProtocolConfig
	mtu      int
	sequence uint32
	mu       sync.Mutex
}

// L2CAPHandler L2CAP协议处理器
type L2CAPHandler struct {
	config   ProtocolConfig
	mtu      int
	sequence uint32
	mu       sync.Mutex
}

// ProtocolConfig 协议配置
type ProtocolConfig struct {
	ProtocolType    string `json:"protocol_type"`    // 协议类型
	MaxMessageSize  int    `json:"max_message_size"` // 最大消息大小
	CompressionType string `json:"compression_type"` // 压缩类型
	EnableChecksum  bool   `json:"enable_checksum"`  // 启用校验和
	Timeout         int    `json:"timeout"`          // 超时时间(秒)
}

// MessageFrame 消息帧结构
type MessageFrame struct {
	Header  FrameHeader `json:"header"`  // 帧头
	Payload []byte      `json:"payload"` // 载荷数据
	Footer  FrameFooter `json:"footer"`  // 帧尾
}

// FrameHeader 帧头结构
type FrameHeader struct {
	Version     uint8  `json:"version"`      // 协议版本
	MessageType uint8  `json:"message_type"` // 消息类型
	Length      uint32 `json:"length"`       // 载荷长度
	Sequence    uint32 `json:"sequence"`     // 序列号
	Flags       uint16 `json:"flags"`        // 标志位
	Reserved    uint16 `json:"reserved"`     // 保留字段
}

// FrameFooter 帧尾结构
type FrameFooter struct {
	Checksum uint32 `json:"checksum"` // 校验和
	Magic    uint32 `json:"magic"`    // 魔数
}

// ProtocolFactory 协议工厂
type ProtocolFactory struct{}

// CreateHandler 创建协议处理器
func (pf *ProtocolFactory) CreateHandler(protocolType string, config ProtocolConfig) (ProtocolHandler, error) {
	switch protocolType {
	case bluetooth.ProtocolRFCOMM:
		handler := &RFCOMMHandler{
			config: config,
			mtu:    bluetooth.DefaultMTU,
		}
		return handler, handler.Initialize(config)
	case bluetooth.ProtocolL2CAP:
		handler := &L2CAPHandler{
			config: config,
			mtu:    bluetooth.DefaultMTU,
		}
		return handler, handler.Initialize(config)
	default:
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeNotSupported, "不支持的协议类型: "+protocolType)
	}
}

// RFCOMM协议处理器实现

// GetProtocolName 获取协议名称
func (r *RFCOMMHandler) GetProtocolName() string {
	return bluetooth.ProtocolRFCOMM
}

// Initialize 初始化RFCOMM处理器
func (r *RFCOMMHandler) Initialize(config ProtocolConfig) error {
	r.config = config
	if config.MaxMessageSize > 0 {
		r.mtu = config.MaxMessageSize
	}
	return nil
}

// Shutdown 关闭RFCOMM处理器
func (r *RFCOMMHandler) Shutdown() error {
	// 清理资源
	return nil
}

// EncodeMessage 编码消息
func (r *RFCOMMHandler) EncodeMessage(message any) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 序列化消息载荷
	payload, err := json.Marshal(message)
	if err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeProtocolError,
			"RFCOMM消息序列化失败", "", "encode")
	}

	// 检查载荷大小 (帧头14字节 + 帧尾8字节 = 22字节开销)
	const frameOverhead = 14 + 8
	if len(payload) > r.mtu-frameOverhead {
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeInvalidParameter,
			fmt.Sprintf("消息载荷大小 %d 超过MTU限制 %d", len(payload), r.mtu-frameOverhead))
	}

	// 创建消息帧
	frame := MessageFrame{
		Header: FrameHeader{
			Version:     1,
			MessageType: uint8(bluetooth.MessageTypeData),
			Length:      uint32(len(payload)),
			Sequence:    r.sequence,
			Flags:       0,
			Reserved:    0,
		},
		Payload: payload,
		Footer: FrameFooter{
			Magic: 0xDEADBEEF,
		},
	}

	r.sequence++

	// 计算校验和
	if r.config.EnableChecksum {
		frame.Footer.Checksum = r.calculateChecksum(payload)
	}

	// 编码帧
	return r.encodeFrame(&frame)
}

// DecodeMessage 解码消息
func (r *RFCOMMHandler) DecodeMessage(data []byte) (any, error) {
	// 解码帧
	frame, err := r.decodeFrame(data)
	if err != nil {
		return nil, err
	}

	// 验证校验和
	if r.config.EnableChecksum {
		expectedChecksum := r.calculateChecksum(frame.Payload)
		if frame.Footer.Checksum != expectedChecksum {
			return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeProtocolError,
				"RFCOMM消息校验和验证失败")
		}
	}

	// 验证魔数
	if frame.Footer.Magic != 0xDEADBEEF {
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeProtocolError,
			"RFCOMM消息魔数验证失败")
	}

	// 反序列化载荷
	var message any
	if err := json.Unmarshal(frame.Payload, &message); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeProtocolError,
			"RFCOMM消息反序列化失败", "", "decode")
	}

	return message, nil
}

// ValidateMessage 验证消息格式
func (r *RFCOMMHandler) ValidateMessage(data []byte) error {
	if len(data) == 0 {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeInvalidParameter, "消息数据为空")
	}
	if len(data) > r.mtu {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeInvalidParameter, "消息大小超过MTU限制")
	}
	return nil
}

// GetMTU 获取最大传输单元
func (r *RFCOMMHandler) GetMTU() int {
	return r.mtu
}

// SetMTU 设置最大传输单元
func (r *RFCOMMHandler) SetMTU(mtu int) error {
	if mtu <= 0 {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeInvalidParameter, "MTU必须大于0")
	}
	r.mtu = mtu
	return nil
}

// L2CAP协议处理器实现

// GetProtocolName 获取协议名称
func (l *L2CAPHandler) GetProtocolName() string {
	return bluetooth.ProtocolL2CAP
}

// Initialize 初始化L2CAP处理器
func (l *L2CAPHandler) Initialize(config ProtocolConfig) error {
	l.config = config
	if config.MaxMessageSize > 0 {
		l.mtu = config.MaxMessageSize
	}
	return nil
}

// Shutdown 关闭L2CAP处理器
func (l *L2CAPHandler) Shutdown() error {
	// 清理资源
	return nil
}

// EncodeMessage 编码消息
func (l *L2CAPHandler) EncodeMessage(message any) ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 序列化消息载荷
	payload, err := json.Marshal(message)
	if err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeProtocolError,
			"L2CAP消息序列化失败", "", "encode")
	}

	// 检查载荷大小 (帧头14字节 + 帧尾8字节 = 22字节开销)
	const frameOverhead = 14 + 8
	if len(payload) > l.mtu-frameOverhead {
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeInvalidParameter,
			fmt.Sprintf("消息载荷大小 %d 超过MTU限制 %d", len(payload), l.mtu-frameOverhead))
	}

	// 创建消息帧
	frame := MessageFrame{
		Header: FrameHeader{
			Version:     1,
			MessageType: uint8(bluetooth.MessageTypeData),
			Length:      uint32(len(payload)),
			Sequence:    l.sequence,
			Flags:       0x0001, // L2CAP特定标志
			Reserved:    0,
		},
		Payload: payload,
		Footer: FrameFooter{
			Magic: 0xCAFEBABE, // L2CAP使用不同的魔数
		},
	}

	l.sequence++

	// 计算校验和
	if l.config.EnableChecksum {
		frame.Footer.Checksum = l.calculateChecksum(payload)
	}

	// 编码帧
	return l.encodeFrame(&frame)
}

// DecodeMessage 解码消息
func (l *L2CAPHandler) DecodeMessage(data []byte) (any, error) {
	// 解码帧
	frame, err := l.decodeFrame(data)
	if err != nil {
		return nil, err
	}

	// 验证校验和
	if l.config.EnableChecksum {
		expectedChecksum := l.calculateChecksum(frame.Payload)
		if frame.Footer.Checksum != expectedChecksum {
			return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeProtocolError,
				"L2CAP消息校验和验证失败")
		}
	}

	// 验证魔数
	if frame.Footer.Magic != 0xCAFEBABE {
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeProtocolError,
			"L2CAP消息魔数验证失败")
	}

	// 反序列化载荷
	var message any
	if err := json.Unmarshal(frame.Payload, &message); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeProtocolError,
			"L2CAP消息反序列化失败", "", "decode")
	}

	return message, nil
}

// ValidateMessage 验证消息格式
func (l *L2CAPHandler) ValidateMessage(data []byte) error {
	if len(data) == 0 {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeInvalidParameter, "消息数据为空")
	}
	if len(data) > l.mtu {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeInvalidParameter, "消息大小超过MTU限制")
	}
	return nil
}

// GetMTU 获取最大传输单元
func (l *L2CAPHandler) GetMTU() int {
	return l.mtu
}

// SetMTU 设置最大传输单元
func (l *L2CAPHandler) SetMTU(mtu int) error {
	if mtu <= 0 {
		return bluetooth.NewBluetoothError(bluetooth.ErrCodeInvalidParameter, "MTU必须大于0")
	}
	l.mtu = mtu
	return nil
}

// ProtocolStream 协议流接口
type ProtocolStream interface {
	io.ReadWriteCloser
	// GetProtocol 获取协议类型
	GetProtocol() string
	// GetMTU 获取MTU
	GetMTU() int
	// SetDeadline 设置读写截止时间
	SetDeadline(ctx context.Context) error
}

// DefaultProtocolConfig 返回默认协议配置
func DefaultProtocolConfig(protocolType string) ProtocolConfig {
	return ProtocolConfig{
		ProtocolType:    protocolType,
		MaxMessageSize:  bluetooth.DefaultMTU,
		CompressionType: "none",
		EnableChecksum:  true,
		Timeout:         30,
	}
}

// RFCOMM协议处理器的辅助方法

// calculateChecksum 计算校验和
func (r *RFCOMMHandler) calculateChecksum(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

// encodeFrame 编码消息帧
func (r *RFCOMMHandler) encodeFrame(frame *MessageFrame) ([]byte, error) {
	buf := new(bytes.Buffer)

	// 写入帧头
	if err := binary.Write(buf, binary.LittleEndian, frame.Header); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeProtocolError,
			"RFCOMM帧头编码失败", "", "encode_frame")
	}

	// 写入载荷
	if _, err := buf.Write(frame.Payload); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeProtocolError,
			"RFCOMM载荷编码失败", "", "encode_frame")
	}

	// 写入帧尾
	if err := binary.Write(buf, binary.LittleEndian, frame.Footer); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeProtocolError,
			"RFCOMM帧尾编码失败", "", "encode_frame")
	}

	return buf.Bytes(), nil
}

// decodeFrame 解码消息帧
func (r *RFCOMMHandler) decodeFrame(data []byte) (*MessageFrame, error) {
	// 帧头实际二进制大小：1+1+4+4+2+2 = 14字节
	// 帧尾实际二进制大小：4+4 = 8字节
	const headerSize = 14
	const footerSize = 8
	const minFrameSize = headerSize + footerSize

	if len(data) < minFrameSize {
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeProtocolError,
			"RFCOMM数据长度不足")
	}

	buf := bytes.NewReader(data)
	frame := &MessageFrame{}

	// 读取帧头
	if err := binary.Read(buf, binary.LittleEndian, &frame.Header); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeProtocolError,
			"RFCOMM帧头解码失败", "", "decode_frame")
	}

	// 验证载荷长度
	maxPayloadSize := uint32(len(data) - headerSize - footerSize)
	if frame.Header.Length > maxPayloadSize {
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeProtocolError,
			fmt.Sprintf("RFCOMM载荷长度无效: 声明长度=%d, 最大允许长度=%d, 总数据长度=%d",
				frame.Header.Length, maxPayloadSize, len(data)))
	}

	// 读取载荷
	frame.Payload = make([]byte, frame.Header.Length)
	if _, err := buf.Read(frame.Payload); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeProtocolError,
			"RFCOMM载荷解码失败", "", "decode_frame")
	}

	// 读取帧尾
	if err := binary.Read(buf, binary.LittleEndian, &frame.Footer); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeProtocolError,
			"RFCOMM帧尾解码失败", "", "decode_frame")
	}

	return frame, nil
}

// L2CAP协议处理器的辅助方法

// calculateChecksum 计算校验和
func (l *L2CAPHandler) calculateChecksum(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

// encodeFrame 编码消息帧
func (l *L2CAPHandler) encodeFrame(frame *MessageFrame) ([]byte, error) {
	buf := new(bytes.Buffer)

	// 写入帧头
	if err := binary.Write(buf, binary.LittleEndian, frame.Header); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeProtocolError,
			"L2CAP帧头编码失败", "", "encode_frame")
	}

	// 写入载荷
	if _, err := buf.Write(frame.Payload); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeProtocolError,
			"L2CAP载荷编码失败", "", "encode_frame")
	}

	// 写入帧尾
	if err := binary.Write(buf, binary.LittleEndian, frame.Footer); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeProtocolError,
			"L2CAP帧尾编码失败", "", "encode_frame")
	}

	return buf.Bytes(), nil
}

// decodeFrame 解码消息帧
func (l *L2CAPHandler) decodeFrame(data []byte) (*MessageFrame, error) {
	// 帧头实际二进制大小：1+1+4+4+2+2 = 14字节
	// 帧尾实际二进制大小：4+4 = 8字节
	const headerSize = 14
	const footerSize = 8
	const minFrameSize = headerSize + footerSize

	if len(data) < minFrameSize {
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeProtocolError,
			"L2CAP数据长度不足")
	}

	buf := bytes.NewReader(data)
	frame := &MessageFrame{}

	// 读取帧头
	if err := binary.Read(buf, binary.LittleEndian, &frame.Header); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeProtocolError,
			"L2CAP帧头解码失败", "", "decode_frame")
	}

	// 验证载荷长度
	maxPayloadSize := uint32(len(data) - headerSize - footerSize)
	if frame.Header.Length > maxPayloadSize {
		return nil, bluetooth.NewBluetoothError(bluetooth.ErrCodeProtocolError,
			fmt.Sprintf("L2CAP载荷长度无效: 声明长度=%d, 最大允许长度=%d, 总数据长度=%d",
				frame.Header.Length, maxPayloadSize, len(data)))
	}

	// 读取载荷
	frame.Payload = make([]byte, frame.Header.Length)
	if _, err := buf.Read(frame.Payload); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeProtocolError,
			"L2CAP载荷解码失败", "", "decode_frame")
	}

	// 读取帧尾
	if err := binary.Read(buf, binary.LittleEndian, &frame.Footer); err != nil {
		return nil, bluetooth.WrapError(err, bluetooth.ErrCodeProtocolError,
			"L2CAP帧尾解码失败", "", "decode_frame")
	}

	return frame, nil
}

// ProtocolProcessor 协议处理器，提供错误处理和重试机制
type ProtocolProcessor struct {
	handler       ProtocolHandler
	retryStrategy bluetooth.ErrorRecoveryStrategy
	mu            sync.RWMutex
}

// NewProtocolProcessor 创建新的协议处理器
func NewProtocolProcessor(handler ProtocolHandler) *ProtocolProcessor {
	return &ProtocolProcessor{
		handler:       handler,
		retryStrategy: bluetooth.DefaultRecoveryStrategy,
	}
}

// ProcessMessage 处理消息，包含重试机制
func (pp *ProtocolProcessor) ProcessMessage(ctx context.Context, data []byte) (any, error) {
	var lastErr error

	for attempt := 0; attempt < pp.retryStrategy.MaxRetries; attempt++ {
		// 验证消息
		if err := pp.handler.ValidateMessage(data); err != nil {
			return nil, err // 验证错误不重试
		}

		// 解码消息
		message, err := pp.handler.DecodeMessage(data)
		if err == nil {
			return message, nil
		}

		lastErr = err

		// 检查是否应该重试
		if !pp.retryStrategy.ShouldRetry(err, attempt) {
			break
		}

		// 等待重试间隔
		delay := pp.retryStrategy.CalculateRetryDelay(attempt)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			// 继续重试
		}
	}

	return nil, lastErr
}

// EncodeMessageWithRetry 编码消息，包含重试机制
func (pp *ProtocolProcessor) EncodeMessageWithRetry(ctx context.Context, message any) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt < pp.retryStrategy.MaxRetries; attempt++ {
		// 编码消息
		data, err := pp.handler.EncodeMessage(message)
		if err == nil {
			return data, nil
		}

		lastErr = err

		// 检查是否应该重试
		if !pp.retryStrategy.ShouldRetry(err, attempt) {
			break
		}

		// 等待重试间隔
		delay := pp.retryStrategy.CalculateRetryDelay(attempt)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			// 继续重试
		}
	}

	return nil, lastErr
}

// SetRetryStrategy 设置重试策略
func (pp *ProtocolProcessor) SetRetryStrategy(strategy bluetooth.ErrorRecoveryStrategy) {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	pp.retryStrategy = strategy
}

// GetHandler 获取协议处理器
func (pp *ProtocolProcessor) GetHandler() ProtocolHandler {
	pp.mu.RLock()
	defer pp.mu.RUnlock()
	return pp.handler
}

// DefaultProtocolStream 默认协议流实现
type DefaultProtocolStream struct {
	protocol string
	mtu      int
	conn     io.ReadWriteCloser
	mu       sync.RWMutex
}

// NewDefaultProtocolStream 创建默认协议流
func NewDefaultProtocolStream(protocol string, mtu int, conn io.ReadWriteCloser) *DefaultProtocolStream {
	return &DefaultProtocolStream{
		protocol: protocol,
		mtu:      mtu,
		conn:     conn,
	}
}

// Read 读取数据
func (dps *DefaultProtocolStream) Read(p []byte) (n int, err error) {
	dps.mu.RLock()
	defer dps.mu.RUnlock()

	if dps.conn == nil {
		return 0, bluetooth.NewBluetoothError(bluetooth.ErrCodeDisconnected, "协议流已关闭")
	}

	return dps.conn.Read(p)
}

// Write 写入数据
func (dps *DefaultProtocolStream) Write(p []byte) (n int, err error) {
	dps.mu.RLock()
	defer dps.mu.RUnlock()

	if dps.conn == nil {
		return 0, bluetooth.NewBluetoothError(bluetooth.ErrCodeDisconnected, "协议流已关闭")
	}

	// 检查数据大小
	if len(p) > dps.mtu {
		return 0, bluetooth.NewBluetoothError(bluetooth.ErrCodeInvalidParameter,
			fmt.Sprintf("数据大小 %d 超过MTU限制 %d", len(p), dps.mtu))
	}

	return dps.conn.Write(p)
}

// Close 关闭流
func (dps *DefaultProtocolStream) Close() error {
	dps.mu.Lock()
	defer dps.mu.Unlock()

	if dps.conn == nil {
		return nil
	}

	err := dps.conn.Close()
	dps.conn = nil
	return err
}

// GetProtocol 获取协议类型
func (dps *DefaultProtocolStream) GetProtocol() string {
	dps.mu.RLock()
	defer dps.mu.RUnlock()
	return dps.protocol
}

// GetMTU 获取MTU
func (dps *DefaultProtocolStream) GetMTU() int {
	dps.mu.RLock()
	defer dps.mu.RUnlock()
	return dps.mtu
}

// SetDeadline 设置读写截止时间
func (dps *DefaultProtocolStream) SetDeadline(ctx context.Context) error {
	// 在实际实现中，这里应该设置底层连接的截止时间
	// 由于我们使用的是通用的 io.ReadWriteCloser，这里只是一个占位实现
	return nil
}
