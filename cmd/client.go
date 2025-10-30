package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// clientCmd 代表客户端命令
var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "启动蓝牙客户端",
	Long: `启动蓝牙客户端模式，连接到蓝牙服务端。

客户端将扫描可用的蓝牙设备并尝试连接到指定的服务。
支持自动重连和连接失败重试机制。`,
	Run: runClient,
}

var (
	// 客户端特定参数
	targetDevice   string
	scanTimeout    time.Duration
	connectTimeout time.Duration
	retryAttempts  int
	retryInterval  time.Duration
	autoReconnect  bool
)

func init() {
	rootCmd.AddCommand(clientCmd)

	// 客户端特定标志
	clientCmd.Flags().StringVar(&targetDevice, "target-device", "", "目标设备地址或名称")
	clientCmd.Flags().DurationVar(&scanTimeout, "scan-timeout", 10*time.Second, "设备扫描超时时间")
	clientCmd.Flags().DurationVar(&connectTimeout, "connect-timeout", 15*time.Second, "连接超时时间")
	clientCmd.Flags().IntVar(&retryAttempts, "retry-attempts", 3, "连接重试次数")
	clientCmd.Flags().DurationVar(&retryInterval, "retry-interval", 2*time.Second, "重试间隔时间")
	clientCmd.Flags().BoolVar(&autoReconnect, "auto-reconnect", true, "是否自动重连")

	// 绑定标志到 viper
	viper.BindPFlag("client.target_device", clientCmd.Flags().Lookup("target-device"))
	viper.BindPFlag("client.scan_timeout", clientCmd.Flags().Lookup("scan-timeout"))
	viper.BindPFlag("client.connect_timeout", clientCmd.Flags().Lookup("connect-timeout"))
	viper.BindPFlag("client.retry_attempts", clientCmd.Flags().Lookup("retry-attempts"))
	viper.BindPFlag("client.retry_interval", clientCmd.Flags().Lookup("retry-interval"))
	viper.BindPFlag("client.auto_reconnect", clientCmd.Flags().Lookup("auto-reconnect"))
}

func runClient(cmd *cobra.Command, args []string) {
	// 初始化日志
	initLogger(viper.GetString("log_level"))

	log.Printf("启动 %s v%s - 客户端模式", AppName, AppVersion)

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动客户端
	go func() {
		if err := startBluetoothClient(ctx); err != nil && err != context.Canceled {
			log.Printf("客户端运行错误: %v", err)
			cancel()
		}
	}()

	// 等待信号
	select {
	case sig := <-sigChan:
		log.Printf("收到信号 %v，正在关闭客户端...", sig)
		cancel()
	case <-ctx.Done():
		log.Println("客户端上下文已取消")
	}

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := gracefulShutdownClient(shutdownCtx); err != nil {
		log.Printf("客户端优雅关闭失败: %v", err)
	} else {
		log.Println("客户端已成功关闭")
	}
}

// startBluetoothClient 启动蓝牙客户端
func startBluetoothClient(ctx context.Context) error {
	log.Println("正在启动蓝牙客户端...")

	// 从配置中获取参数
	config := bluetooth.DefaultConfig()

	// 更新客户端配置
	if timeout := viper.GetDuration("client.scan_timeout"); timeout > 0 {
		config.ClientConfig.ScanTimeout = timeout
	}
	if timeout := viper.GetDuration("client.connect_timeout"); timeout > 0 {
		config.ClientConfig.ConnectTimeout = timeout
	}
	if attempts := viper.GetInt("client.retry_attempts"); attempts >= 0 {
		config.ClientConfig.RetryAttempts = attempts
	}
	if interval := viper.GetDuration("client.retry_interval"); interval > 0 {
		config.ClientConfig.RetryInterval = interval
	}
	config.ClientConfig.AutoReconnect = viper.GetBool("client.auto_reconnect")

	targetDev := viper.GetString("client.target_device")

	log.Printf("客户端配置:")
	log.Printf("  目标设备: %s", targetDev)
	log.Printf("  扫描超时: %v", config.ClientConfig.ScanTimeout)
	log.Printf("  连接超时: %v", config.ClientConfig.ConnectTimeout)
	log.Printf("  重试次数: %d", config.ClientConfig.RetryAttempts)
	log.Printf("  重试间隔: %v", config.ClientConfig.RetryInterval)
	log.Printf("  自动重连: %v", config.ClientConfig.AutoReconnect)

	// TODO: 实现实际的蓝牙客户端启动逻辑
	// 1. 创建蓝牙适配器
	// 2. 扫描设备
	// 3. 连接到目标设备
	// 4. 处理数据通信

	// 模拟客户端运行
	if targetDev != "" {
		log.Printf("正在连接到设备: %s", targetDev)
	} else {
		log.Println("开始扫描可用设备...")
	}

	<-ctx.Done()
	return ctx.Err()
}

// gracefulShutdownClient 优雅关闭客户端
func gracefulShutdownClient(ctx context.Context) error {
	log.Println("开始优雅关闭客户端...")

	// TODO: 实现客户端优雅关闭逻辑
	// 1. 断开现有连接
	// 2. 停止扫描
	// 3. 清理客户端资源

	select {
	case <-time.After(1 * time.Second):
		log.Println("客户端优雅关闭完成")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
