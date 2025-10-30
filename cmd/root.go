package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// 应用程序版本信息
const (
	AppName    = "CatTag"
	AppVersion = "1.0.0"
	AppDesc    = "Go 蓝牙通信库"
)

var (
	// 全局配置文件路径
	cfgFile string
	// 全局日志级别
	logLevel string
)

// rootCmd 代表基础命令，当不带任何子命令调用时执行
var rootCmd = &cobra.Command{
	Use:   "cattag",
	Short: "CatTag - Go 蓝牙通信库",
	Long: `CatTag 是一个用 Go 语言开发的现代化蓝牙通信库，
专注于提供高性能、类型安全的蓝牙设备通信解决方案。

支持的功能：
• 蓝牙设备管理：设备发现、连接管理、状态监控
• 双向通信：支持服务端和客户端模式的蓝牙通信
• 消息处理：泛型消息处理，支持自定义消息类型
• 连接池管理：高效的多连接管理和负载均衡
• 安全通信：AES-256-GCM 加密和设备认证
• 健康监控：连接质量监控和自动故障恢复`,
	Version: AppVersion,
}

// Execute 添加所有子命令到根命令并设置标志
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// 全局标志，在这里定义标志并绑定到配置
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件路径 (默认为 $HOME/.cattag.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "日志级别 (debug, info, warn, error)")

	// Cobra 也支持本地标志，只在直接调用此操作时运行
	rootCmd.Flags().BoolP("version", "v", false, "显示版本信息")
}

// initConfig 读取配置文件和环境变量
func initConfig() {
	if cfgFile != "" {
		// 使用命令行指定的配置文件
		viper.SetConfigFile(cfgFile)
	} else {
		// 查找主目录
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// 在主目录中搜索名为 ".cattag" 的配置文件（不带扩展名）
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
		viper.SetConfigType("yaml")
		viper.SetConfigName("cattag")
	}

	// 读取环境变量
	viper.AutomaticEnv()

	// 如果找到配置文件，则读取它
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "使用配置文件:", viper.ConfigFileUsed())
	}
}

// GetRootCommand 返回根命令，主要用于测试
func GetRootCommand() *cobra.Command {
	return rootCmd
}
