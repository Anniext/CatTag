package main

import (
	"bufio"
	"context"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// FileTransferMessage 文件传输消息结构
type FileTransferMessage struct {
	ID          string            `json:"id"`           // 消息ID
	Type        string            `json:"type"`         // 消息类型 (file_info, chunk, ack, error)
	FileName    string            `json:"file_name"`    // 文件名
	FileSize    int64             `json:"file_size"`    // 文件大小
	ChunkIndex  int               `json:"chunk_index"`  // 分块索引
	TotalChunks int               `json:"total_chunks"` // 总分块数
	Data        []byte            `json:"data"`         // 数据内容
	Checksum    string            `json:"checksum"`     // 校验和
	Metadata    map[string]string `json:"metadata"`     // 元数据
	Timestamp   time.Time         `json:"timestamp"`    // 时间戳
}

// FileTransferApp 文件传输应用结构
type FileTransferApp struct {
	component   bluetooth.BluetoothComponent[FileTransferMessage]
	mode        string
	serviceUUID string
	downloadDir string
	chunkSize   int
	transfers   map[string]*FileTransfer // 传输会话
	ctx         context.Context
	cancel      context.CancelFunc
}

// FileTransfer 文件传输会话
type FileTransfer struct {
	ID             string
	FileName       string
	FileSize       int64
	TotalChunks    int
	ReceivedChunks map[int]bool
	Data           []byte
	Checksum       string
	StartTime      time.Time
	LastActivity   time.Time
	Progress       float64
	Status         string // pending, transferring, completed, failed
}

var (
	mode        = flag.String("mode", "server", "运行模式 (server, client)")
	serviceUUID = flag.String("service", "87654321-4321-4321-4321-210987654321", "服务UUID")
	downloadDir = flag.String("download", "./downloads", "下载目录")
	chunkSize   = flag.Int("chunk-size", 1024, "分块大小（字节）")
	logLevel    = flag.String("log-level", "info", "日志级别")
)

func main() {
	flag.Parse()

	fmt.Printf("=== CatTag 蓝牙文件传输应用 ===\n")
	fmt.Printf("模式: %s\n", *mode)
	fmt.Printf("服务UUID: %s\n", *serviceUUID)
	fmt.Printf("下载目录: %s\n", *downloadDir)
	fmt.Printf("分块大小: %d 字节\n", *chunkSize)
	fmt.Println("输入 '/help' 查看帮助信息")
	fmt.Println("输入 '/quit' 退出应用")
	fmt.Println("==================================")

	// 创建下载目录
	if err := os.MkdirAll(*downloadDir, 0755); err != nil {
		log.Fatalf("创建下载目录失败: %v", err)
	}

	// 创建文件传输应用
	app, err := NewFileTransferApp(*mode, *serviceUUID, *downloadDir, *chunkSize)
	if err != nil {
		log.Fatalf("创建文件传输应用失败: %v", err)
	}

	// 启动应用
	if err := app.Start(); err != nil {
		log.Fatalf("启动文件传输应用失败: %v", err)
	}

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动用户输入处理
	go app.handleUserInput()

	// 等待退出信号
	<-sigChan
	fmt.Println("\n正在退出文件传输应用...")

	// 停止应用
	if err := app.Stop(); err != nil {
		log.Printf("停止文件传输应用失败: %v", err)
	}

	fmt.Println("文件传输应用已退出")
}

// NewFileTransferApp 创建新的文件传输应用
func NewFileTransferApp(mode, serviceUUID, downloadDir string, chunkSize int) (*FileTransferApp, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建蓝牙组件配置
	var builder *bluetooth.ComponentBuilder
	switch mode {
	case "server":
		builder = bluetooth.NewServerBuilder(serviceUUID)
	case "client":
		builder = bluetooth.NewClientBuilder()
	default:
		cancel()
		return nil, fmt.Errorf("不支持的模式: %s", mode)
	}

	// 构建蓝牙组件
	component, err := builder.
		SetLogLevel(*logLevel).
		SetMaxConnections(5).
		Build()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("创建蓝牙组件失败: %w", err)
	}

	app := &FileTransferApp{
		component:   component,
		mode:        mode,
		serviceUUID: serviceUUID,
		downloadDir: downloadDir,
		chunkSize:   chunkSize,
		transfers:   make(map[string]*FileTransfer),
		ctx:         ctx,
		cancel:      cancel,
	}

	return app, nil
}

// Start 启动文件传输应用
func (app *FileTransferApp) Start() error {
	// 启动蓝牙组件
	if err := app.component.Start(app.ctx); err != nil {
		return fmt.Errorf("启动蓝牙组件失败: %w", err)
	}

	// 根据模式启动相应功能
	switch app.mode {
	case "server":
		return app.startServer()
	case "client":
		return app.startClient()
	default:
		return fmt.Errorf("不支持的模式: %s", app.mode)
	}
}

// Stop 停止文件传输应用
func (app *FileTransferApp) Stop() error {
	app.cancel()
	return app.component.Stop(context.Background())
}

// startServer 启动服务端模式
func (app *FileTransferApp) startServer() error {
	fmt.Printf("[系统] 文件传输服务端已启动，等待客户端连接...\n")

	// 获取服务端实例
	server := app.component.GetServer()

	// 开始监听
	if err := server.Listen(app.serviceUUID); err != nil {
		return fmt.Errorf("服务端监听失败: %w", err)
	}

	// 处理连接
	go app.handleConnections(server.AcceptConnections())

	// 处理消息
	go app.handleMessages()

	fmt.Printf("[系统] 文件传输服务已就绪，可以接收文件\n")

	return nil
}

// startClient 启动客户端模式
func (app *FileTransferApp) startClient() error {
	fmt.Printf("[系统] 文件传输客户端已启动，正在搜索服务端...\n")

	// 获取客户端实例
	client := app.component.GetClient()

	// 扫描设备
	devices, err := client.Scan(app.ctx, 10*time.Second)
	if err != nil {
		return fmt.Errorf("扫描设备失败: %w", err)
	}

	if len(devices) == 0 {
		fmt.Println("[系统] 未发现可用的文件传输服务端")
		return nil
	}

	// 显示发现的设备
	fmt.Printf("[系统] 发现 %d 个设备:\n", len(devices))
	for i, device := range devices {
		fmt.Printf("  %d. %s (%s)\n", i+1, device.Name, device.ID)
	}

	// 连接到第一个设备（简化处理）
	if len(devices) > 0 {
		device := devices[0]
		fmt.Printf("[系统] 正在连接到 %s...\n", device.Name)

		_, err := client.Connect(app.ctx, device.ID)
		if err != nil {
			return fmt.Errorf("连接设备失败: %w", err)
		}

		fmt.Printf("[系统] 已连接到 %s，可以开始传输文件\n", device.Name)
	}

	// 处理消息
	go app.handleMessages()

	return nil
}

// handleConnections 处理连接事件
func (app *FileTransferApp) handleConnections(connChan <-chan bluetooth.Connection) {
	for {
		select {
		case conn := <-connChan:
			if conn != nil {
				fmt.Printf("[系统] 新设备连接: %s\n", conn.DeviceID())
			}
		case <-app.ctx.Done():
			return
		}
	}
}

// handleMessages 处理接收到的消息
func (app *FileTransferApp) handleMessages() {
	msgChan := app.component.Receive(app.ctx)

	for {
		select {
		case msg := <-msgChan:
			app.processMessage(msg.Payload)
		case <-app.ctx.Done():
			return
		}
	}
}

// processMessage 处理文件传输消息
func (app *FileTransferApp) processMessage(msg FileTransferMessage) {
	switch msg.Type {
	case "file_info":
		app.handleFileInfo(msg)
	case "chunk":
		app.handleChunk(msg)
	case "ack":
		app.handleAck(msg)
	case "error":
		app.handleError(msg)
	default:
		fmt.Printf("[错误] 未知消息类型: %s\n", msg.Type)
	}
}

// handleFileInfo 处理文件信息
func (app *FileTransferApp) handleFileInfo(msg FileTransferMessage) {
	fmt.Printf("[系统] 接收文件信息: %s (%.2f KB)\n",
		msg.FileName, float64(msg.FileSize)/1024)

	// 创建传输会话
	transfer := &FileTransfer{
		ID:             msg.ID,
		FileName:       msg.FileName,
		FileSize:       msg.FileSize,
		TotalChunks:    msg.TotalChunks,
		ReceivedChunks: make(map[int]bool),
		Data:           make([]byte, msg.FileSize),
		Checksum:       msg.Checksum,
		StartTime:      time.Now(),
		LastActivity:   time.Now(),
		Progress:       0.0,
		Status:         "transferring",
	}

	app.transfers[msg.ID] = transfer

	// 发送确认消息
	ackMsg := FileTransferMessage{
		ID:        app.generateMessageID(),
		Type:      "ack",
		Metadata:  map[string]string{"transfer_id": msg.ID, "status": "ready"},
		Timestamp: time.Now(),
	}

	app.sendMessage(ackMsg)
	fmt.Printf("[系统] 开始接收文件: %s\n", msg.FileName)
}

// handleChunk 处理文件分块
func (app *FileTransferApp) handleChunk(msg FileTransferMessage) {
	transferID := msg.Metadata["transfer_id"]
	transfer, exists := app.transfers[transferID]
	if !exists {
		fmt.Printf("[错误] 未找到传输会话: %s\n", transferID)
		return
	}

	// 检查分块是否已接收
	if transfer.ReceivedChunks[msg.ChunkIndex] {
		return // 重复分块，忽略
	}

	// 计算数据在文件中的位置
	offset := msg.ChunkIndex * app.chunkSize
	copy(transfer.Data[offset:], msg.Data)

	// 标记分块已接收
	transfer.ReceivedChunks[msg.ChunkIndex] = true
	transfer.LastActivity = time.Now()

	// 更新进度
	receivedChunks := len(transfer.ReceivedChunks)
	transfer.Progress = float64(receivedChunks) / float64(transfer.TotalChunks) * 100

	fmt.Printf("\r[传输] %s: %.1f%% (%d/%d 分块)",
		transfer.FileName, transfer.Progress, receivedChunks, transfer.TotalChunks)

	// 发送确认消息
	ackMsg := FileTransferMessage{
		ID:   app.generateMessageID(),
		Type: "ack",
		Metadata: map[string]string{
			"transfer_id": transferID,
			"chunk_index": strconv.Itoa(msg.ChunkIndex),
			"status":      "received",
		},
		Timestamp: time.Now(),
	}

	app.sendMessage(ackMsg)

	// 检查是否接收完成
	if receivedChunks == transfer.TotalChunks {
		fmt.Printf("\n[系统] 文件接收完成: %s\n", transfer.FileName)
		app.completeTransfer(transfer)
	}
}

// handleAck 处理确认消息
func (app *FileTransferApp) handleAck(msg FileTransferMessage) {
	status := msg.Metadata["status"]
	transferID := msg.Metadata["transfer_id"]

	switch status {
	case "ready":
		fmt.Printf("[系统] 对方已准备接收文件，开始传输...\n")
	case "received":
		chunkIndex := msg.Metadata["chunk_index"]
		fmt.Printf("\r[传输] 分块 %s 已确认接收", chunkIndex)
	case "completed":
		fmt.Printf("\n[系统] 文件传输完成: %s\n", transferID)
	}
}

// handleError 处理错误消息
func (app *FileTransferApp) handleError(msg FileTransferMessage) {
	fmt.Printf("[错误] 传输错误: %s\n", msg.Metadata["error"])
}

// completeTransfer 完成文件传输
func (app *FileTransferApp) completeTransfer(transfer *FileTransfer) {
	// 验证文件完整性
	if transfer.Checksum != "" {
		hash := md5.Sum(transfer.Data)
		actualChecksum := hex.EncodeToString(hash[:])

		if actualChecksum != transfer.Checksum {
			fmt.Printf("[错误] 文件校验失败: %s\n", transfer.FileName)
			transfer.Status = "failed"
			return
		}
	}

	// 保存文件
	filePath := filepath.Join(app.downloadDir, transfer.FileName)
	if err := os.WriteFile(filePath, transfer.Data, 0644); err != nil {
		fmt.Printf("[错误] 保存文件失败: %v\n", err)
		transfer.Status = "failed"
		return
	}

	transfer.Status = "completed"
	duration := time.Since(transfer.StartTime)
	speed := float64(transfer.FileSize) / duration.Seconds() / 1024 // KB/s

	fmt.Printf("[系统] 文件已保存: %s\n", filePath)
	fmt.Printf("[统计] 传输时间: %.2fs, 平均速度: %.2f KB/s\n", duration.Seconds(), speed)

	// 发送完成确认
	ackMsg := FileTransferMessage{
		ID:   app.generateMessageID(),
		Type: "ack",
		Metadata: map[string]string{
			"transfer_id": transfer.ID,
			"status":      "completed",
		},
		Timestamp: time.Now(),
	}

	app.sendMessage(ackMsg)
}

// handleUserInput 处理用户输入
func (app *FileTransferApp) handleUserInput() {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// 处理命令
		if strings.HasPrefix(input, "/") {
			app.handleCommand(input)
			continue
		}

		// 如果是客户端模式，尝试发送文件
		if app.mode == "client" {
			app.sendFile(input)
		} else {
			fmt.Println("服务端模式下请使用命令，输入 /help 查看帮助")
		}
	}
}

// handleCommand 处理命令
func (app *FileTransferApp) handleCommand(command string) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return
	}

	cmd := parts[0]
	switch cmd {
	case "/help":
		app.showHelp()
	case "/quit", "/exit":
		fmt.Println("正在退出...")
		app.cancel()
	case "/status":
		app.showStatus()
	case "/transfers":
		app.showTransfers()
	case "/send":
		if len(parts) > 1 {
			app.sendFile(parts[1])
		} else {
			fmt.Println("用法: /send <文件路径>")
		}
	case "/clear":
		app.clearScreen()
	default:
		fmt.Printf("未知命令: %s，输入 /help 查看帮助\n", cmd)
	}
}

// sendFile 发送文件
func (app *FileTransferApp) sendFile(filePath string) {
	// 检查文件是否存在
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		fmt.Printf("[错误] 文件不存在: %s\n", filePath)
		return
	}

	if fileInfo.IsDir() {
		fmt.Printf("[错误] 不能发送目录: %s\n", filePath)
		return
	}

	// 读取文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("[错误] 读取文件失败: %v\n", err)
		return
	}

	// 计算校验和
	hash := md5.Sum(data)
	checksum := hex.EncodeToString(hash[:])

	// 计算分块数
	totalChunks := (len(data) + app.chunkSize - 1) / app.chunkSize
	transferID := app.generateMessageID()

	fmt.Printf("[系统] 开始发送文件: %s (%.2f KB, %d 分块)\n",
		filepath.Base(filePath), float64(len(data))/1024, totalChunks)

	// 发送文件信息
	fileInfoMsg := FileTransferMessage{
		ID:          transferID,
		Type:        "file_info",
		FileName:    filepath.Base(filePath),
		FileSize:    int64(len(data)),
		TotalChunks: totalChunks,
		Checksum:    checksum,
		Metadata:    map[string]string{"transfer_id": transferID},
		Timestamp:   time.Now(),
	}

	if err := app.sendMessage(fileInfoMsg); err != nil {
		fmt.Printf("[错误] 发送文件信息失败: %v\n", err)
		return
	}

	// 创建传输会话
	transfer := &FileTransfer{
		ID:          transferID,
		FileName:    filepath.Base(filePath),
		FileSize:    int64(len(data)),
		TotalChunks: totalChunks,
		Data:        data,
		Checksum:    checksum,
		StartTime:   time.Now(),
		Status:      "transferring",
	}

	app.transfers[transferID] = transfer

	// 发送文件分块
	go app.sendFileChunks(transferID, data)
}

// sendFileChunks 发送文件分块
func (app *FileTransferApp) sendFileChunks(transferID string, data []byte) {
	transfer := app.transfers[transferID]

	for i := 0; i < transfer.TotalChunks; i++ {
		start := i * app.chunkSize
		end := start + app.chunkSize
		if end > len(data) {
			end = len(data)
		}

		chunkData := data[start:end]

		chunkMsg := FileTransferMessage{
			ID:         app.generateMessageID(),
			Type:       "chunk",
			ChunkIndex: i,
			Data:       chunkData,
			Metadata:   map[string]string{"transfer_id": transferID},
			Timestamp:  time.Now(),
		}

		if err := app.sendMessage(chunkMsg); err != nil {
			fmt.Printf("[错误] 发送分块 %d 失败: %v\n", i, err)
			return
		}

		// 更新进度
		progress := float64(i+1) / float64(transfer.TotalChunks) * 100
		fmt.Printf("\r[发送] %s: %.1f%% (%d/%d 分块)",
			transfer.FileName, progress, i+1, transfer.TotalChunks)

		// 添加小延迟避免过快发送
		time.Sleep(10 * time.Millisecond)
	}

	fmt.Printf("\n[系统] 文件发送完成: %s\n", transfer.FileName)
}

// sendMessage 发送消息
func (app *FileTransferApp) sendMessage(msg FileTransferMessage) error {
	// 获取连接池中的所有连接
	pool := app.component.GetConnectionPool()
	connections := pool.GetAllConnections()

	if len(connections) == 0 {
		return fmt.Errorf("没有可用的连接")
	}

	// 向第一个连接发送消息（简化处理）
	conn := connections[0]
	return app.component.Send(app.ctx, conn.DeviceID(), msg)
}

// showHelp 显示帮助信息
func (app *FileTransferApp) showHelp() {
	fmt.Println("=== 文件传输应用命令帮助 ===")
	fmt.Println("/help        - 显示此帮助信息")
	fmt.Println("/quit        - 退出应用")
	fmt.Println("/status      - 显示连接状态")
	fmt.Println("/transfers   - 显示传输列表")
	fmt.Println("/send <文件> - 发送文件（客户端模式）")
	fmt.Println("/clear       - 清屏")

	if app.mode == "client" {
		fmt.Println("\n直接输入文件路径也可以发送文件")
	}
	fmt.Println("==============================")
}

// showStatus 显示连接状态
func (app *FileTransferApp) showStatus() {
	status := app.component.GetStatus()
	pool := app.component.GetConnectionPool()
	stats := pool.GetStats()

	fmt.Println("=== 连接状态 ===")
	fmt.Printf("组件状态: %v\n", status)
	fmt.Printf("运行模式: %s\n", app.mode)
	fmt.Printf("活跃连接: %d\n", stats.ActiveConnections)
	fmt.Printf("下载目录: %s\n", app.downloadDir)
	fmt.Printf("分块大小: %d 字节\n", app.chunkSize)
	fmt.Printf("传输会话: %d\n", len(app.transfers))
	fmt.Println("================")
}

// showTransfers 显示传输列表
func (app *FileTransferApp) showTransfers() {
	fmt.Printf("=== 传输列表 (%d) ===\n", len(app.transfers))

	if len(app.transfers) == 0 {
		fmt.Println("暂无传输记录")
		fmt.Println("====================")
		return
	}

	for _, transfer := range app.transfers {
		duration := time.Since(transfer.StartTime)
		fmt.Printf("文件: %s\n", transfer.FileName)
		fmt.Printf("  大小: %.2f KB\n", float64(transfer.FileSize)/1024)
		fmt.Printf("  状态: %s\n", transfer.Status)
		fmt.Printf("  进度: %.1f%%\n", transfer.Progress)
		fmt.Printf("  用时: %.2fs\n", duration.Seconds())
		fmt.Println("---")
	}
	fmt.Println("====================")
}

// clearScreen 清屏
func (app *FileTransferApp) clearScreen() {
	fmt.Print("\033[2J\033[H")
}

// generateMessageID 生成消息ID
func (app *FileTransferApp) generateMessageID() string {
	return fmt.Sprintf("ft_msg_%d", time.Now().UnixNano())
}
