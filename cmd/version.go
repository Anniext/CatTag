package cmd

import (
	"fmt"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

// versionCmd 代表版本命令
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Long: `显示 CatTag 的详细版本信息，包括：
• 应用程序版本
• Go 版本
• 构建信息
• 系统信息`,
	Run: showVersion,
}

var (
	// 构建时注入的变量
	buildTime = "unknown"
	gitCommit = "unknown"
	gitBranch = "unknown"
	buildUser = "unknown"
	buildHost = "unknown"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

func showVersion(cmd *cobra.Command, args []string) {
	fmt.Printf("%s v%s\n", AppName, AppVersion)
	fmt.Printf("%s\n\n", AppDesc)

	fmt.Println("版本信息:")
	fmt.Printf("  应用版本: %s\n", AppVersion)
	fmt.Printf("  Go 版本:  %s\n", runtime.Version())
	fmt.Printf("  系统架构: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  CPU 核心: %d\n", runtime.NumCPU())

	fmt.Println("\n构建信息:")
	if buildTime != "unknown" {
		fmt.Printf("  构建时间: %s\n", buildTime)
	} else {
		fmt.Printf("  构建时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	}
	fmt.Printf("  Git 提交: %s\n", gitCommit)
	fmt.Printf("  Git 分支: %s\n", gitBranch)
	fmt.Printf("  构建用户: %s\n", buildUser)
	fmt.Printf("  构建主机: %s\n", buildHost)

	fmt.Println("\n技术栈:")
	fmt.Println("  • Go 1.25.1 - 现代化 Go 语言特性")
	fmt.Println("  • 蓝牙协议 - RFCOMM 和 L2CAP 支持")
	fmt.Println("  • AES-256-GCM - 企业级加密算法")
	fmt.Println("  • Cobra CLI - 强大的命令行框架")
	fmt.Println("  • Viper - 灵活的配置管理")

	fmt.Println("\n核心功能:")
	fmt.Println("  • 蓝牙设备管理和连接")
	fmt.Println("  • 双向通信支持")
	fmt.Println("  • 泛型消息处理")
	fmt.Println("  • 连接池管理")
	fmt.Println("  • 安全通信和认证")
	fmt.Println("  • 健康监控和故障恢复")
}
