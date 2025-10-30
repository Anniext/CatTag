package cmd

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/Anniext/CatTag/internal/adapter"
	"github.com/Anniext/CatTag/pkg/bluetooth"
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

	// 实现实际的蓝牙设备扫描逻辑
	// 1. 创建蓝牙适配器
	adapterFactory := adapter.NewDefaultAdapterFactory()
	bluetoothAdapter, err := adapterFactory.GetDefaultAdapter()
	if err != nil {
		return fmt.Errorf("创建蓝牙适配器失败: %v", err)
	}

	// 2. 初始化适配器
	if err := bluetoothAdapter.Initialize(ctx); err != nil {
		return fmt.Errorf("初始化蓝牙适配器失败: %v", err)
	}
	defer bluetoothAdapter.Shutdown(ctx)

	// 3. 启用蓝牙适配器
	if !bluetoothAdapter.IsEnabled() {
		if err := bluetoothAdapter.Enable(ctx); err != nil {
			return fmt.Errorf("启用蓝牙适配器失败: %v", err)
		}
	}

	// 4. 创建设备过滤器
	filter := bluetooth.DeviceFilter{
		NamePattern:  filterName,
		ServiceUUIDs: []string{},
		MinRSSI:      -100, // 默认最小信号强度
		RequireAuth:  false,
		DeviceTypes:  []string{},
		MaxAge:       5 * time.Minute,
	}

	// 如果指定了服务过滤，添加到过滤器中
	if filterService != "" {
		filter.ServiceUUIDs = []string{filterService}
	}

	deviceCount := 0
	scannedDevices := make(map[string]bool) // 用于去重

	// 5. 开始扫描循环
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("\n扫描完成，共发现 %d 个设备\n", deviceCount)
			return ctx.Err()
		default:
			// 执行一次扫描
			scanTimeout := 5 * time.Second
			if continuous {
				scanTimeout = 2 * time.Second // 持续模式使用较短的扫描间隔
			}

			deviceChan, err := bluetoothAdapter.Scan(ctx, scanTimeout)
			if err != nil {
				log.Printf("扫描设备失败: %v", err)
				if !continuous {
					return fmt.Errorf("扫描设备失败: %v", err)
				}
				// 持续模式下，等待一段时间后重试
				time.Sleep(1 * time.Second)
				continue
			}

			// 6. 处理发现的设备
			scanComplete := false
			for !scanComplete {
				select {
				case <-ctx.Done():
					fmt.Printf("\n扫描完成，共发现 %d 个设备\n", deviceCount)
					return ctx.Err()
				case device, ok := <-deviceChan:
					if !ok {
						scanComplete = true
						break
					}

					// 应用过滤条件并去重
					if shouldShowBluetoothDevice(device, filterName, filterService) {
						// 使用设备地址作为唯一标识进行去重
						if !scannedDevices[device.Address] {
							displayBluetoothDevice(device, showRSSI)
							scannedDevices[device.Address] = true
							deviceCount++
						}
					}
				}
			}

			// 停止当前扫描
			bluetoothAdapter.StopScan()

			if !continuous {
				// 非持续模式下，扫描一轮后退出
				fmt.Printf("\n扫描完成，共发现 %d 个设备\n", deviceCount)
				return nil
			}

			// 持续模式下，等待一段时间后继续下一轮扫描
			time.Sleep(1 * time.Second)
		}
	}
}

// shouldShowBluetoothDevice 检查是否应该显示蓝牙设备
func shouldShowBluetoothDevice(device bluetooth.Device, filterName, filterService string) bool {
	// 名称过滤
	if filterName != "" && !strings.Contains(strings.ToLower(device.Name), strings.ToLower(filterName)) {
		return false
	}

	// 服务过滤
	if filterService != "" {
		found := false
		for _, service := range device.ServiceUUIDs {
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

// displayBluetoothDevice 显示蓝牙设备信息
func displayBluetoothDevice(device bluetooth.Device, showRSSI bool) {
	fmt.Printf("发现设备: %s\n", device.Name)
	fmt.Printf("  设备ID: %s\n", device.ID)
	fmt.Printf("  地址: %s\n", device.Address)

	if showRSSI {
		fmt.Printf("  信号强度: %d dBm\n", device.RSSI)
	}

	fmt.Printf("  最后发现: %s\n", device.LastSeen.Format("2006-01-02 15:04:05"))

	if len(device.ServiceUUIDs) > 0 {
		fmt.Printf("  服务UUID:\n")
		for _, service := range device.ServiceUUIDs {
			fmt.Printf("    - %s\n", service)
		}
	}

	// 显示设备能力信息
	if device.Capabilities.SupportsEncryption {
		fmt.Printf("  支持加密: 是\n")
	}

	if device.Capabilities.BatteryLevel > 0 {
		fmt.Printf("  电池电量: %d%%\n", device.Capabilities.BatteryLevel)
	}

	if len(device.Capabilities.SupportedProtocols) > 0 {
		fmt.Printf("  支持协议: %s\n", strings.Join(device.Capabilities.SupportedProtocols, ", "))
	}

	fmt.Println()
}
