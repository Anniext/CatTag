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

// serverCmd 代表服务端命令
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "启动蓝牙服务端",
	Long: `启动蓝牙服务端模式，监听客户端连接。

服务端将创建蓝牙服务并等待客户端连接，支持多个并发连接。
可以通过配置文件或命令行参数自定义服务端行为。`,
	Run: runServer,
}

var (
	// 服务端特定参数
	serviceUUID    string
	serviceName    string
	maxConnections int
	acceptTimeout  time.Duration
	requireAuth    bool
)

func init() {
	rootCmd.AddCommand(serverCmd)

	// 服务端特定标志
	serverCmd.Flags().StringVar(&serviceUUID, "service-uuid", "", "蓝牙服务 UUID")
	serverCmd.Flags().StringVar(&serviceName, "service-name", "CatTag Service", "蓝牙服务名称")
	serverCmd.Flags().IntVar(&maxConnections, "max-connections", 10, "最大并发连接数")
	serverCmd.Flags().DurationVar(&acceptTimeout, "accept-timeout", 30*time.Second, "接受连接超时时间")
	serverCmd.Flags().BoolVar(&requireAuth, "require-auth", true, "是否需要身份验证")

	// 绑定标志到 viper
	viper.BindPFlag("server.service_uuid", serverCmd.Flags().Lookup("service-uuid"))
	viper.BindPFlag("server.service_name", serverCmd.Flags().Lookup("service-name"))
	viper.BindPFlag("server.max_connections", serverCmd.Flags().Lookup("max-connections"))
	viper.BindPFlag("server.accept_timeout", serverCmd.Flags().Lookup("accept-timeout"))
	viper.BindPFlag("server.require_auth", serverCmd.Flags().Lookup("require-auth"))
}

func runServer(cmd *cobra.Command, args []string) {
	// 初始化日志
	initLogger(viper.GetString("log_level"))

	log.Printf("启动 %s v%s - 服务端模式", AppName, AppVersion)

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动服务端
	go func() {
		if err := startBluetoothServer(ctx); err != nil && err != context.Canceled {
			log.Printf("服务端运行错误: %v", err)
			cancel()
		}
	}()

	// 等待信号
	select {
	case sig := <-sigChan:
		log.Printf("收到信号 %v，正在关闭服务端...", sig)
		cancel()
	case <-ctx.Done():
		log.Println("服务端上下文已取消")
	}

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := gracefulShutdownServer(shutdownCtx); err != nil {
		log.Printf("服务端优雅关闭失败: %v", err)
	} else {
		log.Println("服务端已成功关闭")
	}
}

// startBluetoothServer 启动蓝牙服务端
func startBluetoothServer(ctx context.Context) error {
	log.Println("正在启动蓝牙服务端...")

	// 从配置中获取参数
	config := bluetooth.DefaultConfig()

	// 更新服务端配置
	if uuid := viper.GetString("server.service_uuid"); uuid != "" {
		config.ServerConfig.ServiceUUID = uuid
	}
	if name := viper.GetString("server.service_name"); name != "" {
		config.ServerConfig.ServiceName = name
	}
	if maxConn := viper.GetInt("server.max_connections"); maxConn > 0 {
		config.ServerConfig.MaxConnections = maxConn
	}
	if timeout := viper.GetDuration("server.accept_timeout"); timeout > 0 {
		config.ServerConfig.AcceptTimeout = timeout
	}
	config.ServerConfig.RequireAuth = viper.GetBool("server.require_auth")

	log.Printf("服务端配置:")
	log.Printf("  服务UUID: %s", config.ServerConfig.ServiceUUID)
	log.Printf("  服务名称: %s", config.ServerConfig.ServiceName)
	log.Printf("  最大连接数: %d", config.ServerConfig.MaxConnections)
	log.Printf("  接受超时: %v", config.ServerConfig.AcceptTimeout)
	log.Printf("  需要认证: %v", config.ServerConfig.RequireAuth)

	// TODO: 实现实际的蓝牙服务端启动逻辑
	// 1. 创建蓝牙适配器
	// 2. 注册服务
	// 3. 开始监听连接
	// 4. 处理客户端连接

	// 模拟服务端运行
	log.Println("蓝牙服务端已启动，等待客户端连接...")
	<-ctx.Done()
	return ctx.Err()
}

// gracefulShutdownServer 优雅关闭服务端
func gracefulShutdownServer(ctx context.Context) error {
	log.Println("开始优雅关闭服务端...")

	// TODO: 实现服务端优雅关闭逻辑
	// 1. 停止接受新连接
	// 2. 等待现有连接完成
	// 3. 清理服务端资源

	select {
	case <-time.After(1 * time.Second):
		log.Println("服务端优雅关闭完成")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
