package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// 应用程序版本信息
const (
	AppName    = "CatTag"
	AppVersion = "1.0.0"
	AppDesc    = "Go 蓝牙通信库"
)

// 命令行参数
var (
	configPath = flag.String("config", "./config/bluetooth.json", "配置文件路径")
	logLevel   = flag.String("log-level", "info", "日志级别 (debug, info, warn, error)")
	mode       = flag.String("mode", "server", "运行模式 (server, client, both)")
	version    = flag.Bool("version", false, "显示版本信息")
	help       = flag.Bool("help", false, "显示帮助信息")
)

func main() {
	// 解析命令行参数
	flag.Parse()

	// 显示版本信息
	if *version {
		fmt.Printf("%s v%s - %s\n", AppName, AppVersion, AppDesc)
		fmt.Printf("Go版本: %s\n", "1.25.1")
		fmt.Printf("构建时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
		return
	}

	// 显示帮助信息
	if *help {
		showHelp()
		return
	}

	// 初始化日志
	initLogger(*logLevel)

	log.Printf("启动 %s v%s", AppName, AppVersion)
	log.Printf("配置文件: %s", *configPath)
	log.Printf("运行模式: %s", *mode)

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 根据模式启动应用
	var err error
	switch *mode {
	case "server":
		err = runServer(ctx, *configPath)
	case "client":
		err = runClient(ctx, *configPath)
	case "both":
		err = runBoth(ctx, *configPath)
	default:
		log.Fatalf("不支持的运行模式: %s", *mode)
	}

	if err != nil {
		log.Fatalf("启动应用失败: %v", err)
	}

	// 等待信号
	select {
	case sig := <-sigChan:
		log.Printf("收到信号 %v，正在关闭应用...", sig)
		cancel()
	case <-ctx.Done():
		log.Println("应用上下文已取消")
	}

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := gracefulShutdown(shutdownCtx); err != nil {
		log.Printf("优雅关闭失败: %v", err)
	} else {
		log.Println("应用已成功关闭")
	}
}

// runServer 运行服务端模式
func runServer(ctx context.Context, configPath string) error {
	log.Println("启动蓝牙服务端...")

	// TODO: 实现服务端启动逻辑
	// 1. 加载配置
	// 2. 创建蓝牙服务端组件
	// 3. 启动服务端

	// 示例实现
	config := bluetooth.DefaultConfig()
	log.Printf("服务端配置: ServiceUUID=%s, MaxConnections=%d",
		config.ServerConfig.ServiceUUID,
		config.ServerConfig.MaxConnections)

	// 模拟服务端运行
	<-ctx.Done()
	return ctx.Err()
}

// runClient 运行客户端模式
func runClient(ctx context.Context, configPath string) error {
	log.Println("启动蓝牙客户端...")

	// TODO: 实现客户端启动逻辑
	// 1. 加载配置
	// 2. 创建蓝牙客户端组件
	// 3. 启动客户端

	// 示例实现
	config := bluetooth.DefaultConfig()
	log.Printf("客户端配置: ScanTimeout=%v, ConnectTimeout=%v",
		config.ClientConfig.ScanTimeout,
		config.ClientConfig.ConnectTimeout)

	// 模拟客户端运行
	<-ctx.Done()
	return ctx.Err()
}

// runBoth 同时运行服务端和客户端
func runBoth(ctx context.Context, configPath string) error {
	log.Println("启动蓝牙服务端和客户端...")

	// 创建子上下文
	serverCtx, serverCancel := context.WithCancel(ctx)
	clientCtx, clientCancel := context.WithCancel(ctx)

	defer func() {
		serverCancel()
		clientCancel()
	}()

	// 启动服务端
	go func() {
		if err := runServer(serverCtx, configPath); err != nil && err != context.Canceled {
			log.Printf("服务端运行错误: %v", err)
		}
	}()

	// 启动客户端
	go func() {
		if err := runClient(clientCtx, configPath); err != nil && err != context.Canceled {
			log.Printf("客户端运行错误: %v", err)
		}
	}()

	// 等待上下文取消
	<-ctx.Done()
	return ctx.Err()
}

// gracefulShutdown 优雅关闭
func gracefulShutdown(ctx context.Context) error {
	log.Println("开始优雅关闭...")

	// TODO: 实现优雅关闭逻辑
	// 1. 停止接受新连接
	// 2. 等待现有连接完成
	// 3. 清理资源

	select {
	case <-time.After(1 * time.Second):
		log.Println("优雅关闭完成")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// initLogger 初始化日志
func initLogger(level string) {
	// TODO: 实现更完善的日志初始化
	// 可以使用 slog 或其他结构化日志库

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	switch level {
	case "debug":
		log.SetOutput(os.Stdout)
	case "info":
		log.SetOutput(os.Stdout)
	case "warn":
		log.SetOutput(os.Stderr)
	case "error":
		log.SetOutput(os.Stderr)
	default:
		log.SetOutput(os.Stdout)
	}
}

// showHelp 显示帮助信息
func showHelp() {
	fmt.Printf("%s v%s - %s\n\n", AppName, AppVersion, AppDesc)
	fmt.Println("用法:")
	fmt.Printf("  %s [选项]\n\n", os.Args[0])
	fmt.Println("选项:")
	flag.PrintDefaults()
	fmt.Println("\n示例:")
	fmt.Printf("  %s -mode=server -config=./config/server.json\n", os.Args[0])
	fmt.Printf("  %s -mode=client -log-level=debug\n", os.Args[0])
	fmt.Printf("  %s -mode=both\n", os.Args[0])
	fmt.Println("\n支持的运行模式:")
	fmt.Println("  server  - 仅运行蓝牙服务端")
	fmt.Println("  client  - 仅运行蓝牙客户端")
	fmt.Println("  both    - 同时运行服务端和客户端")
	fmt.Println("\n支持的日志级别:")
	fmt.Println("  debug   - 调试级别，显示详细信息")
	fmt.Println("  info    - 信息级别，显示一般信息")
	fmt.Println("  warn    - 警告级别，显示警告信息")
	fmt.Println("  error   - 错误级别，仅显示错误信息")
}
