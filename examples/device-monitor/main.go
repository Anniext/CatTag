package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// MonitorMessage 监控消息结构
type MonitorMessage struct {
	ID        string                 `json:"id"`        // 消息ID
	Type      string                 `json:"type"`      // 消息类型 (ping, pong, status, alert)
	DeviceID  string                 `json:"device_id"` // 设备ID
	Data      map[string]interface{} `json:"data"`      // 监控数据
	Timestamp time.Time              `json:"timestamp"` // 时间戳
}

// DeviceMonitorApp 设备监控应用结构
type DeviceMonitorApp struct {
	component        bluetooth.BluetoothComponent[MonitorMessage]
	mode             string
	serviceUUID      string
	monitoredDevices map[string]*DeviceStatus
	scanInterval     time.Duration
	pingInterval     time.Duration
	ctx              context.Context
	cancel           context.CancelFunc
}

// DeviceStatus 设备状态信息
type DeviceStatus struct {
	Device           bluetooth.Device
	Connection       bluetooth.Connection
	IsConnected      bool
	LastSeen         time.Time
	LastPing         time.Time
	LastPong         time.Time
	RSSI             int
	Latency          time.Duration
	PacketLoss       float64
	ConnectionTime   time.Time
	BytesSent        int64
	BytesReceived    int64
	MessagesSent     int64
	MessagesReceived int64
	ErrorCount       int64
	Status           string // online, offline, connecting, error
	Alerts           []string
}

var (
	mode         = flag.String("mode", "monitor", "运行模式 (monitor, device)")
	serviceUUID  = flag.String("service", "11111111-2222-3333-4444-555555555555", "服务UUID")
	scanInterval = flag.Duration("scan-interval", 30*time.Second, "设备扫描间隔")
	pingInterval = flag.Duration("ping-interval", 5*time.Second, "心跳检测间隔")
	logLevel     = flag.String("log-level", "info", "日志级别")
)

func main() {
	flag.Parse()

	fmt.Printf("=== CatTag 蓝牙设备监控应用 ===\n")
	fmt.Printf("模式: %s\n", *mode)
	fmt.Printf("服务UUID: %s\n", *serviceUUID)
	fmt.Printf("扫描间隔: %v\n", *scanInterval)
	fmt.Printf("心跳间隔: %v\n", *pingInterval)
	fmt.Println("输入 'help' 查看帮助信息")
	fmt.Println("输入 'quit' 退出应用")
	fmt.Println("==================================")

	// 创建设备监控应用
	app, err := NewDeviceMonitorApp(*mode, *serviceUUID, *scanInterval, *pingInterval)
	if err != nil {
		log.Fatalf("创建设备监控应用失败: %v", err)
	}

	// 启动应用
	if err := app.Start(); err != nil {
		log.Fatalf("启动设备监控应用失败: %v", err)
	}

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动用户输入处理
	go app.handleUserInput()

	// 启动监控界面更新
	go app.updateMonitorDisplay()

	// 等待退出信号
	<-sigChan
	fmt.Println("\n正在退出设备监控应用...")

	// 停止应用
	if err := app.Stop(); err != nil {
		log.Printf("停止设备监控应用失败: %v", err)
	}

	fmt.Println("设备监控应用已退出")
}

// NewDeviceMonitorApp 创建新的设备监控应用
func NewDeviceMonitorApp(mode, serviceUUID string, scanInterval, pingInterval time.Duration) (*DeviceMonitorApp, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建蓝牙组件配置
	var builder *bluetooth.ComponentBuilder
	switch mode {
	case "monitor":
		// 监控模式：既是服务端也是客户端
		builder = bluetooth.NewFullBuilder(serviceUUID)
	case "device":
		// 设备模式：主要作为服务端
		builder = bluetooth.NewServerBuilder(serviceUUID)
	default:
		cancel()
		return nil, fmt.Errorf("不支持的模式: %s", mode)
	}

	// 构建蓝牙组件
	component, err := builder.
		SetLogLevel(*logLevel).
		SetMaxConnections(20).
		Build()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("创建蓝牙组件失败: %w", err)
	}

	app := &DeviceMonitorApp{
		component:        component,
		mode:             mode,
		serviceUUID:      serviceUUID,
		monitoredDevices: make(map[string]*DeviceStatus),
		scanInterval:     scanInterval,
		pingInterval:     pingInterval,
		ctx:              ctx,
		cancel:           cancel,
	}

	return app, nil
}

// Start 启动设备监控应用
func (app *DeviceMonitorApp) Start() error {
	// 启动蓝牙组件
	if err := app.component.Start(app.ctx); err != nil {
		return fmt.Errorf("启动蓝牙组件失败: %w", err)
	}

	// 根据模式启动相应功能
	switch app.mode {
	case "monitor":
		return app.startMonitor()
	case "device":
		return app.startDevice()
	default:
		return fmt.Errorf("不支持的模式: %s", app.mode)
	}
}

// Stop 停止设备监控应用
func (app *DeviceMonitorApp) Stop() error {
	app.cancel()
	return app.component.Stop(context.Background())
}

// startMonitor 启动监控模式
func (app *DeviceMonitorApp) startMonitor() error {
	fmt.Printf("[系统] 设备监控器已启动\n")

	// 启动服务端监听
	server := app.component.GetServer()
	if err := server.Listen(app.serviceUUID); err != nil {
		return fmt.Errorf("服务端监听失败: %w", err)
	}

	// 处理连接
	go app.handleConnections(server.AcceptConnections())

	// 处理消息
	go app.handleMessages()

	// 启动设备扫描
	go app.deviceScanner()

	// 启动心跳检测
	go app.heartbeatMonitor()

	// 启动健康检查
	go app.healthMonitor()

	fmt.Printf("[系统] 监控服务已启动，开始扫描设备...\n")

	return nil
}

// startDevice 启动设备模式
func (app *DeviceMonitorApp) startDevice() error {
	fmt.Printf("[系统] 设备代理已启动\n")

	// 启动服务端监听
	server := app.component.GetServer()
	if err := server.Listen(app.serviceUUID); err != nil {
		return fmt.Errorf("服务端监听失败: %w", err)
	}

	// 处理连接
	go app.handleConnections(server.AcceptConnections())

	// 处理消息
	go app.handleMessages()

	// 启动状态报告
	go app.statusReporter()

	fmt.Printf("[系统] 设备代理已就绪，等待监控器连接...\n")

	return nil
}

// deviceScanner 设备扫描器
func (app *DeviceMonitorApp) deviceScanner() {
	ticker := time.NewTicker(app.scanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			app.scanDevices()
		case <-app.ctx.Done():
			return
		}
	}
}

// scanDevices 扫描设备
func (app *DeviceMonitorApp) scanDevices() {
	client := app.component.GetClient()

	devices, err := client.Scan(app.ctx, 10*time.Second)
	if err != nil {
		fmt.Printf("[错误] 设备扫描失败: %v\n", err)
		return
	}

	// 更新设备列表
	for _, device := range devices {
		if _, exists := app.monitoredDevices[device.ID]; !exists {
			// 新发现的设备
			status := &DeviceStatus{
				Device:      device,
				IsConnected: false,
				LastSeen:    time.Now(),
				RSSI:        device.RSSI,
				Status:      "offline",
				Alerts:      make([]string, 0),
			}
			app.monitoredDevices[device.ID] = status

			// 尝试连接新设备
			go app.connectToDevice(device)
		} else {
			// 更新已知设备信息
			app.monitoredDevices[device.ID].LastSeen = time.Now()
			app.monitoredDevices[device.ID].RSSI = device.RSSI
		}
	}

	// 检查离线设备
	app.checkOfflineDevices()
}

// connectToDevice 连接到设备
func (app *DeviceMonitorApp) connectToDevice(device bluetooth.Device) {
	client := app.component.GetClient()
	status := app.monitoredDevices[device.ID]

	status.Status = "connecting"

	conn, err := client.Connect(app.ctx, device.ID)
	if err != nil {
		status.Status = "error"
		status.ErrorCount++
		status.Alerts = append(status.Alerts, fmt.Sprintf("连接失败: %v", err))
		return
	}

	status.Connection = conn
	status.IsConnected = true
	status.ConnectionTime = time.Now()
	status.Status = "online"

	fmt.Printf("[连接] 已连接到设备: %s (%s)\n", device.Name, device.ID)
}

// checkOfflineDevices 检查离线设备
func (app *DeviceMonitorApp) checkOfflineDevices() {
	timeout := 2 * app.scanInterval
	now := time.Now()

	for deviceID, status := range app.monitoredDevices {
		if now.Sub(status.LastSeen) > timeout && status.Status != "offline" {
			status.Status = "offline"
			status.IsConnected = false
			status.Alerts = append(status.Alerts, "设备离线")

			fmt.Printf("[离线] 设备离线: %s\n", deviceID)
		}
	}
}

// heartbeatMonitor 心跳监控
func (app *DeviceMonitorApp) heartbeatMonitor() {
	ticker := time.NewTicker(app.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			app.sendHeartbeats()
		case <-app.ctx.Done():
			return
		}
	}
}

// sendHeartbeats 发送心跳
func (app *DeviceMonitorApp) sendHeartbeats() {
	for deviceID, status := range app.monitoredDevices {
		if status.IsConnected {
			go app.sendPing(deviceID)
		}
	}
}

// sendPing 发送心跳包
func (app *DeviceMonitorApp) sendPing(deviceID string) {
	status := app.monitoredDevices[deviceID]

	pingMsg := MonitorMessage{
		ID:       app.generateMessageID(),
		Type:     "ping",
		DeviceID: deviceID,
		Data: map[string]interface{}{
			"timestamp": time.Now().UnixNano(),
		},
		Timestamp: time.Now(),
	}

	status.LastPing = time.Now()

	if err := app.component.Send(app.ctx, deviceID, pingMsg); err != nil {
		status.ErrorCount++
		status.Alerts = append(status.Alerts, fmt.Sprintf("心跳发送失败: %v", err))
	} else {
		status.MessagesSent++
	}
}

// healthMonitor 健康监控
func (app *DeviceMonitorApp) healthMonitor() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			app.performHealthCheck()
		case <-app.ctx.Done():
			return
		}
	}
}

// performHealthCheck 执行健康检查
func (app *DeviceMonitorApp) performHealthCheck() {
	healthChecker := app.component.GetHealthChecker()

	for deviceID, status := range app.monitoredDevices {
		if status.IsConnected {
			healthStatus := healthChecker.CheckConnection(deviceID)

			// 更新健康状态
			if !healthStatus.IsHealthy {
				status.Alerts = append(status.Alerts, "连接健康状态异常")
			}

			// 更新延迟信息
			status.Latency = healthStatus.Latency
		}
	}
}

// statusReporter 状态报告器（设备模式）
func (app *DeviceMonitorApp) statusReporter() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			app.reportStatus()
		case <-app.ctx.Done():
			return
		}
	}
}

// reportStatus 报告设备状态
func (app *DeviceMonitorApp) reportStatus() {
	pool := app.component.GetConnectionPool()
	connections := pool.GetAllConnections()

	for _, conn := range connections {
		statusMsg := MonitorMessage{
			ID:       app.generateMessageID(),
			Type:     "status",
			DeviceID: "self",
			Data: map[string]interface{}{
				"cpu_usage":    app.getCPUUsage(),
				"memory_usage": app.getMemoryUsage(),
				"uptime":       time.Since(time.Now()).Seconds(),
				"connections":  len(connections),
			},
			Timestamp: time.Now(),
		}

		app.component.Send(app.ctx, conn.DeviceID(), statusMsg)
	}
}

// handleConnections 处理连接事件
func (app *DeviceMonitorApp) handleConnections(connChan <-chan bluetooth.Connection) {
	for {
		select {
		case conn := <-connChan:
			if conn != nil {
				fmt.Printf("[连接] 新设备连接: %s\n", conn.DeviceID())

				// 更新设备状态
				if status, exists := app.monitoredDevices[conn.DeviceID()]; exists {
					status.Connection = conn
					status.IsConnected = true
					status.ConnectionTime = time.Now()
					status.Status = "online"
				}
			}
		case <-app.ctx.Done():
			return
		}
	}
}

// handleMessages 处理接收到的消息
func (app *DeviceMonitorApp) handleMessages() {
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

// processMessage 处理监控消息
func (app *DeviceMonitorApp) processMessage(msg MonitorMessage) {
	switch msg.Type {
	case "ping":
		app.handlePing(msg)
	case "pong":
		app.handlePong(msg)
	case "status":
		app.handleStatus(msg)
	case "alert":
		app.handleAlert(msg)
	default:
		fmt.Printf("[消息] 未知消息类型: %s\n", msg.Type)
	}
}

// handlePing 处理心跳请求
func (app *DeviceMonitorApp) handlePing(msg MonitorMessage) {
	// 回复心跳响应
	pongMsg := MonitorMessage{
		ID:       app.generateMessageID(),
		Type:     "pong",
		DeviceID: "self",
		Data: map[string]interface{}{
			"ping_id":   msg.ID,
			"timestamp": msg.Data["timestamp"],
		},
		Timestamp: time.Now(),
	}

	pool := app.component.GetConnectionPool()
	connections := pool.GetAllConnections()

	for _, conn := range connections {
		app.component.Send(app.ctx, conn.DeviceID(), pongMsg)
	}
}

// handlePong 处理心跳响应
func (app *DeviceMonitorApp) handlePong(msg MonitorMessage) {
	deviceID := msg.DeviceID
	if status, exists := app.monitoredDevices[deviceID]; exists {
		status.LastPong = time.Now()
		status.MessagesReceived++

		// 计算延迟
		if pingTimestamp, ok := msg.Data["timestamp"].(float64); ok {
			pingTime := time.Unix(0, int64(pingTimestamp))
			status.Latency = time.Since(pingTime)
		}
	}
}

// handleStatus 处理状态报告
func (app *DeviceMonitorApp) handleStatus(msg MonitorMessage) {
	deviceID := msg.DeviceID
	if status, exists := app.monitoredDevices[deviceID]; exists {
		// 更新设备状态信息
		status.LastSeen = time.Now()

		// 可以在这里处理更多的状态信息
		fmt.Printf("[状态] 设备 %s 状态更新\n", deviceID)
	}
}

// handleAlert 处理告警消息
func (app *DeviceMonitorApp) handleAlert(msg MonitorMessage) {
	deviceID := msg.DeviceID
	alertMsg := fmt.Sprintf("告警: %v", msg.Data["message"])

	if status, exists := app.monitoredDevices[deviceID]; exists {
		status.Alerts = append(status.Alerts, alertMsg)
	}

	fmt.Printf("[告警] 设备 %s: %s\n", deviceID, alertMsg)
}

// handleUserInput 处理用户输入
func (app *DeviceMonitorApp) handleUserInput() {
	for {
		var input string
		fmt.Scanln(&input)

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		app.handleCommand(input)
	}
}

// handleCommand 处理命令
func (app *DeviceMonitorApp) handleCommand(command string) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return
	}

	cmd := parts[0]
	switch cmd {
	case "help":
		app.showHelp()
	case "quit", "exit":
		fmt.Println("正在退出...")
		app.cancel()
	case "list":
		app.listDevices()
	case "status":
		app.showStatus()
	case "connect":
		if len(parts) > 1 {
			app.connectDevice(parts[1])
		} else {
			fmt.Println("用法: connect <设备ID>")
		}
	case "disconnect":
		if len(parts) > 1 {
			app.disconnectDevice(parts[1])
		} else {
			fmt.Println("用法: disconnect <设备ID>")
		}
	case "ping":
		if len(parts) > 1 {
			app.pingDevice(parts[1])
		} else {
			fmt.Println("用法: ping <设备ID>")
		}
	case "clear":
		app.clearScreen()
	case "refresh":
		app.scanDevices()
	default:
		fmt.Printf("未知命令: %s，输入 help 查看帮助\n", cmd)
	}
}

// updateMonitorDisplay 更新监控显示
func (app *DeviceMonitorApp) updateMonitorDisplay() {
	if app.mode != "monitor" {
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			app.displayMonitorInfo()
		case <-app.ctx.Done():
			return
		}
	}
}

// displayMonitorInfo 显示监控信息
func (app *DeviceMonitorApp) displayMonitorInfo() {
	// 清屏并显示监控面板
	fmt.Print("\033[2J\033[H")

	fmt.Println("=== CatTag 蓝牙设备监控面板 ===")
	fmt.Printf("时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("监控设备数: %d\n", len(app.monitoredDevices))

	// 统计信息
	online, offline, connecting := 0, 0, 0
	for _, status := range app.monitoredDevices {
		switch status.Status {
		case "online":
			online++
		case "offline":
			offline++
		case "connecting":
			connecting++
		}
	}

	fmt.Printf("在线: %d, 离线: %d, 连接中: %d\n", online, offline, connecting)
	fmt.Println("================================")

	// 设备列表
	fmt.Println("设备ID          | 名称           | 状态      | RSSI | 延迟    | 最后活动")
	fmt.Println("----------------|----------------|-----------|------|---------|------------------")

	// 按设备ID排序
	var deviceIDs []string
	for deviceID := range app.monitoredDevices {
		deviceIDs = append(deviceIDs, deviceID)
	}
	sort.Strings(deviceIDs)

	for _, deviceID := range deviceIDs {
		status := app.monitoredDevices[deviceID]

		name := status.Device.Name
		if len(name) > 14 {
			name = name[:11] + "..."
		}

		rssiStr := fmt.Sprintf("%d dBm", status.RSSI)
		latencyStr := fmt.Sprintf("%.1fms", float64(status.Latency.Nanoseconds())/1000000)
		lastActivity := status.LastSeen.Format("15:04:05")

		fmt.Printf("%-15s | %-14s | %-9s | %-4s | %-7s | %s\n",
			deviceID[:15], name, status.Status, rssiStr, latencyStr, lastActivity)
	}

	fmt.Println("================================")
	fmt.Println("命令: list, status, connect <ID>, disconnect <ID>, ping <ID>, refresh, clear, quit")
}

// showHelp 显示帮助信息
func (app *DeviceMonitorApp) showHelp() {
	fmt.Println("=== 设备监控应用命令帮助 ===")
	fmt.Println("help         - 显示此帮助信息")
	fmt.Println("quit/exit    - 退出应用")
	fmt.Println("list         - 列出所有设备")
	fmt.Println("status       - 显示系统状态")
	fmt.Println("connect <ID> - 连接指定设备")
	fmt.Println("disconnect <ID> - 断开指定设备")
	fmt.Println("ping <ID>    - 向指定设备发送心跳")
	fmt.Println("refresh      - 刷新设备列表")
	fmt.Println("clear        - 清屏")
	fmt.Println("==============================")
}

// listDevices 列出设备
func (app *DeviceMonitorApp) listDevices() {
	fmt.Printf("=== 监控设备列表 (%d) ===\n", len(app.monitoredDevices))

	for deviceID, status := range app.monitoredDevices {
		fmt.Printf("设备ID: %s\n", deviceID)
		fmt.Printf("  名称: %s\n", status.Device.Name)
		fmt.Printf("  状态: %s\n", status.Status)
		fmt.Printf("  RSSI: %d dBm\n", status.RSSI)
		fmt.Printf("  延迟: %v\n", status.Latency)
		fmt.Printf("  最后活动: %s\n", status.LastSeen.Format("2006-01-02 15:04:05"))
		fmt.Printf("  错误计数: %d\n", status.ErrorCount)

		if len(status.Alerts) > 0 {
			fmt.Printf("  告警: %s\n", strings.Join(status.Alerts, ", "))
		}
		fmt.Println("---")
	}
	fmt.Println("========================")
}

// showStatus 显示系统状态
func (app *DeviceMonitorApp) showStatus() {
	componentStatus := app.component.GetStatus()
	pool := app.component.GetConnectionPool()
	stats := pool.GetStats()

	fmt.Println("=== 系统状态 ===")
	fmt.Printf("组件状态: %v\n", componentStatus)
	fmt.Printf("运行模式: %s\n", app.mode)
	fmt.Printf("活跃连接: %d\n", stats.ActiveConnections)
	fmt.Printf("总连接数: %d\n", stats.TotalConnections)
	fmt.Printf("监控设备: %d\n", len(app.monitoredDevices))
	fmt.Printf("扫描间隔: %v\n", app.scanInterval)
	fmt.Printf("心跳间隔: %v\n", app.pingInterval)

	// 健康检查信息
	healthChecker := app.component.GetHealthChecker()
	healthReport := healthChecker.GetHealthReport()
	fmt.Printf("整体健康: %v\n", healthReport.OverallHealth)

	fmt.Println("================")
}

// connectDevice 连接设备
func (app *DeviceMonitorApp) connectDevice(deviceID string) {
	if status, exists := app.monitoredDevices[deviceID]; exists {
		if status.IsConnected {
			fmt.Printf("设备 %s 已连接\n", deviceID)
			return
		}

		go app.connectToDevice(status.Device)
		fmt.Printf("正在连接设备 %s...\n", deviceID)
	} else {
		fmt.Printf("未找到设备: %s\n", deviceID)
	}
}

// disconnectDevice 断开设备
func (app *DeviceMonitorApp) disconnectDevice(deviceID string) {
	if status, exists := app.monitoredDevices[deviceID]; exists {
		if !status.IsConnected {
			fmt.Printf("设备 %s 未连接\n", deviceID)
			return
		}

		client := app.component.GetClient()
		if err := client.Disconnect(deviceID); err != nil {
			fmt.Printf("断开设备 %s 失败: %v\n", deviceID, err)
		} else {
			status.IsConnected = false
			status.Status = "offline"
			fmt.Printf("已断开设备 %s\n", deviceID)
		}
	} else {
		fmt.Printf("未找到设备: %s\n", deviceID)
	}
}

// pingDevice 向设备发送心跳
func (app *DeviceMonitorApp) pingDevice(deviceID string) {
	if status, exists := app.monitoredDevices[deviceID]; exists {
		if !status.IsConnected {
			fmt.Printf("设备 %s 未连接\n", deviceID)
			return
		}

		app.sendPing(deviceID)
		fmt.Printf("已向设备 %s 发送心跳\n", deviceID)
	} else {
		fmt.Printf("未找到设备: %s\n", deviceID)
	}
}

// clearScreen 清屏
func (app *DeviceMonitorApp) clearScreen() {
	fmt.Print("\033[2J\033[H")
}

// getCPUUsage 获取CPU使用率（模拟）
func (app *DeviceMonitorApp) getCPUUsage() float64 {
	// 这里应该实现真实的CPU使用率获取
	return 15.5 // 模拟值
}

// getMemoryUsage 获取内存使用率（模拟）
func (app *DeviceMonitorApp) getMemoryUsage() float64 {
	// 这里应该实现真实的内存使用率获取
	return 45.2 // 模拟值
}

// generateMessageID 生成消息ID
func (app *DeviceMonitorApp) generateMessageID() string {
	return fmt.Sprintf("monitor_msg_%d", time.Now().UnixNano())
}
