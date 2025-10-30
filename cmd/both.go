package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// bothCmd 代表同时运行服务端和客户端的命令
var bothCmd = &cobra.Command{
	Use:   "both",
	Short: "同时启动蓝牙服务端和客户端",
	Long: `同时启动蓝牙服务端和客户端模式。

这个模式适用于需要同时提供服务和连接其他设备的场景。
服务端和客户端将在独立的 goroutine 中运行，共享相同的蓝牙适配器。`,
	Run: runBoth,
}

func init() {
	rootCmd.AddCommand(bothCmd)

	// both 命令继承 server 和 client 的所有标志
	// 服务端标志
	bothCmd.Flags().StringVar(&serviceUUID, "service-uuid", "", "蓝牙服务 UUID")
	bothCmd.Flags().StringVar(&serviceName, "service-name", "CatTag Service", "蓝牙服务名称")
	bothCmd.Flags().IntVar(&maxConnections, "max-connections", 10, "最大并发连接数")
	bothCmd.Flags().DurationVar(&acceptTimeout, "accept-timeout", 30*time.Second, "接受连接超时时间")
	bothCmd.Flags().BoolVar(&requireAuth, "require-auth", true, "是否需要身份验证")

	// 客户端标志
	bothCmd.Flags().StringVar(&targetDevice, "target-device", "", "目标设备地址或名称")
	bothCmd.Flags().DurationVar(&scanTimeout, "scan-timeout", 10*time.Second, "设备扫描超时时间")
	bothCmd.Flags().DurationVar(&connectTimeout, "connect-timeout", 15*time.Second, "连接超时时间")
	bothCmd.Flags().IntVar(&retryAttempts, "retry-attempts", 3, "连接重试次数")
	bothCmd.Flags().DurationVar(&retryInterval, "retry-interval", 2*time.Second, "重试间隔时间")
	bothCmd.Flags().BoolVar(&autoReconnect, "auto-reconnect", true, "是否自动重连")

	// 绑定标志到 viper
	viper.BindPFlag("server.service_uuid", bothCmd.Flags().Lookup("service-uuid"))
	viper.BindPFlag("server.service_name", bothCmd.Flags().Lookup("service-name"))
	viper.BindPFlag("server.max_connections", bothCmd.Flags().Lookup("max-connections"))
	viper.BindPFlag("server.accept_timeout", bothCmd.Flags().Lookup("accept-timeout"))
	viper.BindPFlag("server.require_auth", bothCmd.Flags().Lookup("require-auth"))

	viper.BindPFlag("client.target_device", bothCmd.Flags().Lookup("target-device"))
	viper.BindPFlag("client.scan_timeout", bothCmd.Flags().Lookup("scan-timeout"))
	viper.BindPFlag("client.connect_timeout", bothCmd.Flags().Lookup("connect-timeout"))
	viper.BindPFlag("client.retry_attempts", bothCmd.Flags().Lookup("retry-attempts"))
	viper.BindPFlag("client.retry_interval", bothCmd.Flags().Lookup("retry-interval"))
	viper.BindPFlag("client.auto_reconnect", bothCmd.Flags().Lookup("auto-reconnect"))
}

func runBoth(cmd *cobra.Command, args []string) {
	// 初始化日志
	initLogger(viper.GetString("log_level"))

	log.Printf("启动 %s v%s - 混合模式（服务端+客户端）", AppName, AppVersion)

	// 创建主上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 创建等待组来管理 goroutine
	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	// 启动服务端
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("正在启动服务端组件...")
		if err := startBluetoothServer(ctx); err != nil && err != context.Canceled {
			log.Printf("服务端组件错误: %v", err)
			errChan <- err
		}
	}()

	// 启动客户端
	wg.Add(1)
	go func() {
		defer wg.Done()
		// 稍微延迟启动客户端，让服务端先初始化
		time.Sleep(1 * time.Second)
		log.Println("正在启动客户端组件...")
		if err := startBluetoothClient(ctx); err != nil && err != context.Canceled {
			log.Printf("��户端组件错误: %v", err)
			errChan <- err
		}
	}()

	// 监控错误和信号
	go func() {
		select {
		case err := <-errChan:
			log.Printf("组件运行错误: %v", err)
			cancel()
		case sig := <-sigChan:
			log.Printf("收到信号 %v，正在关闭应用...", sig)
			cancel()
		case <-ctx.Done():
			log.Println("应用上下文已取消")
		}
	}()

	// 等待所有组件完成
	wg.Wait()

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	log.Println("开始优雅关闭所有组件...")

	// 并行关闭服务端和客户端
	var shutdownWg sync.WaitGroup
	shutdownErrors := make(chan error, 2)

	shutdownWg.Add(1)
	go func() {
		defer shutdownWg.Done()
		if err := gracefulShutdownServer(shutdownCtx); err != nil {
			shutdownErrors <- err
		}
	}()

	shutdownWg.Add(1)
	go func() {
		defer shutdownWg.Done()
		if err := gracefulShutdownClient(shutdownCtx); err != nil {
			shutdownErrors <- err
		}
	}()

	// 等待关闭完成
	shutdownWg.Wait()
	close(shutdownErrors)

	// 检查关闭错误
	var hasErrors bool
	for err := range shutdownErrors {
		if err != nil {
			log.Printf("组件关闭错误: %v", err)
			hasErrors = true
		}
	}

	if !hasErrors {
		log.Println("所有组件已成功关闭")
	}
}
