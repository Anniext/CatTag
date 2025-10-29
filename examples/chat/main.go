package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// ChatMessage 聊天消息结构
type ChatMessage struct {
	ID        string    `json:"id"`        // 消息ID
	Sender    string    `json:"sender"`    // 发送者
	Content   string    `json:"content"`   // 消息内容
	Timestamp time.Time `json:"timestamp"` // 时间戳
	Type      string    `json:"type"`      // 消息类型 (text, system, file)
}

// ChatApp 聊天应用结构
type ChatApp struct {
	component   bluetooth.BluetoothComponent[ChatMessage]
	username    string
	mode        string
	serviceUUID string
	messages    []ChatMessage
	ctx         context.Context
	cancel      context.CancelFunc
}

var (
	mode        = flag.String("mode", "server", "运行模式 (server, client)")
	username    = flag.String("username", "用户", "用户名")
	serviceUUID = flag.String("service", "12345678-1234-1234-1234-123456789abc", "服务UUID")
	logLevel    = flag.String("log-level", "info", "日志级别")
)

func main() {
	flag.Parse()

	fmt.Printf("=== CatTag 蓝牙聊天应用 ===\n")
	fmt.Printf("模式: %s\n", *mode)
	fmt.Printf("用户名: %s\n", *username)
	fmt.Printf("服务UUID: %s\n", *serviceUUID)
	fmt.Println("输入 '/help' 查看帮助信息")
	fmt.Println("输入 '/quit' 退出应用")
	fmt.Println("================================")

	// 创建聊天应用
	app, err := NewChatApp(*mode, *username, *serviceUUID)
	if err != nil {
		log.Fatalf("创建聊天应用失败: %v", err)
	}

	// 启动应用
	if err := app.Start(); err != nil {
		log.Fatalf("启动聊天应用失败: %v", err)
	}

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动用户输入处理
	go app.handleUserInput()

	// 等待退出信号
	<-sigChan
	fmt.Println("\n正在退出聊天应用...")

	// 停止应用
	if err := app.Stop(); err != nil {
		log.Printf("停止聊天应用失败: %v", err)
	}

	fmt.Println("聊天应用已退出")
}

// NewChatApp 创建新的聊天应用
func NewChatApp(mode, username, serviceUUID string) (*ChatApp, error) {
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
		SetMaxConnections(10).
		Build()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("创建蓝牙组件失败: %w", err)
	}

	app := &ChatApp{
		component:   component,
		username:    username,
		mode:        mode,
		serviceUUID: serviceUUID,
		messages:    make([]ChatMessage, 0),
		ctx:         ctx,
		cancel:      cancel,
	}

	return app, nil
}

// Start 启动聊天应用
func (app *ChatApp) Start() error {
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

// Stop 停止聊天应用
func (app *ChatApp) Stop() error {
	app.cancel()
	return app.component.Stop(context.Background())
}

// startServer 启动服务端模式
func (app *ChatApp) startServer() error {
	fmt.Printf("[系统] 聊天服务端已启动，等待客户端连接...\n")

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

	// 发送欢迎消息
	app.addSystemMessage("聊天室已创建，等待用户加入...")

	return nil
}

// startClient 启动客户端模式
func (app *ChatApp) startClient() error {
	fmt.Printf("[系统] 聊天客户端已启动，正在搜索服务端...\n")

	// 获取客户端实例
	client := app.component.GetClient()

	// 扫描设备
	devices, err := client.Scan(app.ctx, 10*time.Second)
	if err != nil {
		return fmt.Errorf("扫描设备失败: %w", err)
	}

	if len(devices) == 0 {
		fmt.Println("[系统] 未发现可用的聊天服务端")
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

		fmt.Printf("[系统] 已连接到 %s\n", device.Name)
		app.addSystemMessage(fmt.Sprintf("已加入聊天室 (%s)", device.Name))
	}

	// 处理消息
	go app.handleMessages()

	return nil
}

// handleConnections 处理连接事件
func (app *ChatApp) handleConnections(connChan <-chan bluetooth.Connection) {
	for {
		select {
		case conn := <-connChan:
			if conn != nil {
				fmt.Printf("[系统] 新用户连接: %s\n", conn.DeviceID())
				app.addSystemMessage(fmt.Sprintf("用户 %s 加入了聊天室", conn.DeviceID()))
			}
		case <-app.ctx.Done():
			return
		}
	}
}

// handleMessages 处理接收到的消息
func (app *ChatApp) handleMessages() {
	msgChan := app.component.Receive(app.ctx)

	for {
		select {
		case msg := <-msgChan:
			app.displayMessage(msg.Payload)
			app.messages = append(app.messages, msg.Payload)
		case <-app.ctx.Done():
			return
		}
	}
}

// handleUserInput 处理用户输入
func (app *ChatApp) handleUserInput() {
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

		// 发送消息
		app.sendMessage(input)
	}
}

// handleCommand 处理命令
func (app *ChatApp) handleCommand(command string) {
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
	case "/users":
		app.showUsers()
	case "/history":
		app.showHistory()
	case "/clear":
		app.clearScreen()
	case "/status":
		app.showStatus()
	default:
		fmt.Printf("未知命令: %s，输入 /help 查看帮助\n", cmd)
	}
}

// sendMessage 发送消息
func (app *ChatApp) sendMessage(content string) {
	message := ChatMessage{
		ID:        app.generateMessageID(),
		Sender:    app.username,
		Content:   content,
		Timestamp: time.Now(),
		Type:      "text",
	}

	// 获取连接池中的所有连接
	pool := app.component.GetConnectionPool()
	connections := pool.GetAllConnections()

	if len(connections) == 0 {
		fmt.Println("[系统] 没有可用的连接，消息未发送")
		return
	}

	// 向所有连接发送消息
	for _, conn := range connections {
		if err := app.component.Send(app.ctx, conn.DeviceID(), message); err != nil {
			fmt.Printf("[错误] 发送消息到 %s 失败: %v\n", conn.DeviceID(), err)
		}
	}

	// 显示自己发送的消息
	app.displayMessage(message)
	app.messages = append(app.messages, message)
}

// addSystemMessage 添加系统消息
func (app *ChatApp) addSystemMessage(content string) {
	message := ChatMessage{
		ID:        app.generateMessageID(),
		Sender:    "系统",
		Content:   content,
		Timestamp: time.Now(),
		Type:      "system",
	}

	app.displayMessage(message)
	app.messages = append(app.messages, message)
}

// displayMessage 显示消息
func (app *ChatApp) displayMessage(msg ChatMessage) {
	timestamp := msg.Timestamp.Format("15:04:05")

	switch msg.Type {
	case "system":
		fmt.Printf("[%s] [系统] %s\n", timestamp, msg.Content)
	case "text":
		if msg.Sender == app.username {
			fmt.Printf("[%s] 我: %s\n", timestamp, msg.Content)
		} else {
			fmt.Printf("[%s] %s: %s\n", timestamp, msg.Sender, msg.Content)
		}
	default:
		fmt.Printf("[%s] %s: %s\n", timestamp, msg.Sender, msg.Content)
	}
}

// showHelp 显示帮助信息
func (app *ChatApp) showHelp() {
	fmt.Println("=== 聊天应用命令帮助 ===")
	fmt.Println("/help     - 显示此帮助信息")
	fmt.Println("/quit     - 退出聊天应用")
	fmt.Println("/users    - 显示在线用户")
	fmt.Println("/history  - 显示消息历史")
	fmt.Println("/clear    - 清屏")
	fmt.Println("/status   - 显示连接状态")
	fmt.Println("直接输入文本发送消息")
	fmt.Println("========================")
}

// showUsers 显示在线用户
func (app *ChatApp) showUsers() {
	pool := app.component.GetConnectionPool()
	connections := pool.GetAllConnections()

	fmt.Printf("=== 在线用户 (%d) ===\n", len(connections)+1)
	fmt.Printf("* %s (我)\n", app.username)

	for _, conn := range connections {
		status := "在线"
		if !conn.IsActive() {
			status = "离线"
		}
		fmt.Printf("* %s (%s)\n", conn.DeviceID(), status)
	}
	fmt.Println("==================")
}

// showHistory 显示消息历史
func (app *ChatApp) showHistory() {
	fmt.Println("=== 消息历史 ===")
	for _, msg := range app.messages {
		app.displayMessage(msg)
	}
	fmt.Println("===============")
}

// clearScreen 清屏
func (app *ChatApp) clearScreen() {
	fmt.Print("\033[2J\033[H")
}

// showStatus 显示连接状态
func (app *ChatApp) showStatus() {
	status := app.component.GetStatus()
	pool := app.component.GetConnectionPool()
	stats := pool.GetStats()

	fmt.Println("=== 连接状态 ===")
	fmt.Printf("组件状态: %v\n", status)
	fmt.Printf("运行模式: %s\n", app.mode)
	fmt.Printf("活跃连接: %d\n", stats.ActiveConnections)
	fmt.Printf("总连接数: %d\n", stats.TotalConnections)
	fmt.Printf("服务UUID: %s\n", app.serviceUUID)

	// 显示健康检查信息
	healthChecker := app.component.GetHealthChecker()
	healthReport := healthChecker.GetHealthReport()
	fmt.Printf("健康状态: %v\n", healthReport.OverallHealth)
	fmt.Println("================")
}

// generateMessageID 生成消息ID
func (app *ChatApp) generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}
