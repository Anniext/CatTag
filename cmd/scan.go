package cmd

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// scanCmd 代表设备扫描命令
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "扫描可用的蓝牙设备",
	Long: `扫描周围可用的蓝牙设备并显示设备信息。

这个命令用于发现附近的蓝牙设备，显示设备名称、地址、信号强度等信息。
可以用于调试和设备发现。`,
	Run: runScan,
}

var (
	// 扫描特定参数
	scanDuration    time.Duration
	showRSSI        bool
	filterByName    string
	filterByService string
	continuous      bool
)

func init() {
	rootCmd.AddCommand(scanCmd)

	// 扫描特定标志
	scanCmd.Flags().DurationVar(&scanDuration, "duration", 10*time.Second, "扫描持续时间")
	scanCmd.Flags().BoolVar(&showRSSI, "show-rssi", true, "显示信号强度 (RSSI)")
	scanCmd.Flags().StringVar(&filterByName, "filter-name", "", "按设备名称过滤")
	scanCmd.Flags().StringVar(&filterByService, "filter-service", "", "按服务 UUID 过滤")
	scanCmd.Flags().BoolVar(&continuous, "continuous", false, "持续扫描模式")

	// 绑定标志到 viper
	viper.BindPFlag("scan.duration", scanCmd.Flags().Lookup("duration"))
	viper.BindPFlag("scan.show_rssi", scanCmd.Flags().Lookup("show-rssi"))
	viper.BindPFlag("scan.filter_name", scanCmd.Flags().Lookup("filter-name"))
	viper.BindPFlag("scan.filter_service", scanCmd.Flags().Lookup("filter-service"))
	viper.BindPFlag("scan.continuous", scanCmd.Flags().Lookup("continuous"))
}

func runScan(cmd *cobra.Command, args []string) {
	// 初始化日志
	initLogger(viper.GetString("log_level"))

	log.Printf("启动 %s v%s - 设备扫描模式", AppName, AppVersion)

	// 从配置中获取参数
	duration := viper.GetDuration("scan.duration")
	showRSSI := viper.GetBool("scan.show_rssi")
	filterName := viper.GetString("scan.filter_name")
	filterService := viper.GetString("scan.filter_service")
	continuous := viper.GetBool("scan.continuous")

	fmt.Printf("扫描配置:\n")
	fmt.Printf("  扫描时长: %v\n", duration)
	fmt.Printf("  显示信号强度: %v\n", showRSSI)
	if filterName != "" {
		fmt.Printf("  名称过滤: %s\n", filterName)
	}
	if filterService != "" {
		fmt.Printf("  服务过滤: %s\n", filterService)
	}
	fmt.Printf("  持续扫描: %v\n", continuous)
	fmt.Println()

	// 创建上下文
	ctx := context.Background()
	if !continuous {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, duration)
		defer cancel()
	}

	// 开始扫描
	if err := startDeviceScan(ctx, showRSSI, filterName, filterService, continuous); err != nil {
		log.Fatalf("设备扫描失败: %v", err)
	}
}

// startDeviceScan 开始设备扫描
func startDeviceScan(ctx context.Context, showRSSI bool, filterName, filterService string, continuous bool) error {
	fmt.Println("开始扫描蓝牙设备...")
	fmt.Println("按 Ctrl+C 停止扫描")
	fmt.Println(strings.Repeat("-", 60))

	// TODO: 实现实际的蓝牙设备扫描逻辑
	// 1. 创建蓝牙适配器
	// 2. 开始扫描
	// 3. 处理发现的设备
	// 4. 应用过滤条件

	// 模拟设备扫描
	devices := []mockDevice{
		{
			Name:     "CatTag Device 1",
			Address:  "AA:BB:CC:DD:EE:01",
			RSSI:     -45,
			Services: []string{"12345678-1234-1234-1234-123456789abc"},
		},
		{
			Name:     "CatTag Device 2",
			Address:  "AA:BB:CC:DD:EE:02",
			RSSI:     -67,
			Services: []string{"87654321-4321-4321-4321-cba987654321"},
		},
		{
			Name:     "Unknown Device",
			Address:  "AA:BB:CC:DD:EE:03",
			RSSI:     -89,
			Services: []string{},
		},
	}

	deviceCount := 0
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("\n扫描完成，共发现 %d 个设备\n", deviceCount)
			return ctx.Err()
		case <-ticker.C:
			// 模拟发现设备
			for _, device := range devices {
				if shouldShowDevice(device, filterName, filterService) {
					displayDevice(device, showRSSI)
					deviceCount++
				}
			}

			if !continuous {
				// 非持续模式下，显示一轮后退出
				fmt.Printf("\n扫描完成，共发现 %d 个设备\n", deviceCount)
				return nil
			}
		}
	}
}

// mockDevice 模拟设备结构
type mockDevice struct {
	Name     string
	Address  string
	RSSI     int
	Services []string
}

// shouldShowDevice 检查是否应该显示设备
func shouldShowDevice(device mockDevice, filterName, filterService string) bool {
	// 名称过滤
	if filterName != "" && !strings.Contains(strings.ToLower(device.Name), strings.ToLower(filterName)) {
		return false
	}

	// 服务过滤
	if filterService != "" {
		found := false
		for _, service := range device.Services {
			if strings.Contains(strings.ToLower(service), strings.ToLower(filterService)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// displayDevice 显示设备信息
func displayDevice(device mockDevice, showRSSI bool) {
	fmt.Printf("发现设备: %s\n", device.Name)
	fmt.Printf("  地址: %s\n", device.Address)

	if showRSSI {
		fmt.Printf("  信号强度: %d dBm\n", device.RSSI)
	}

	if len(device.Services) > 0 {
		fmt.Printf("  服务:\n")
		for _, service := range device.Services {
			fmt.Printf("    - %s\n", service)
		}
	}

	fmt.Println()
}
