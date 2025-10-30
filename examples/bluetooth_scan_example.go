package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Anniext/CatTag/internal/adapter"
	"github.com/Anniext/CatTag/pkg/bluetooth"
)

func main() {
	fmt.Println("CatTag 蓝牙设备扫描示例")
	fmt.Println("========================")

	// 创建适配器配置
	config := adapter.DefaultAdapterConfig()
	config.Name = "扫描示例适配器"
	config.Timeout = 30 * time.Second

	// 创建蓝牙适配器
	btAdapter := adapter.NewDefaultAdapter(config)

	// 初始化适配器
	ctx := context.Background()
	if err := btAdapter.Initialize(ctx); err != nil {
		log.Fatalf("初始化蓝牙适配器失败: %v", err)
	}
	defer btAdapter.Shutdown(ctx)

	// 启用蓝牙
	if err := btAdapter.Enable(ctx); err != nil {
		log.Fatalf("启用蓝牙失败: %v", err)
	}

	fmt.Println("开始扫描蓝牙设备...")
	fmt.Println("提示: 如果设备名称显示为 'Unknown Device'，这是正常的")
	fmt.Println("     某些设备不会在广播中包含名称信息")
	fmt.Println()

	// 开始扫描
	scanTimeout := 30 * time.Second
	deviceChan, err := btAdapter.Scan(ctx, scanTimeout)
	if err != nil {
		log.Fatalf("开始扫描失败: %v", err)
	}

	// 用于跟踪发现的设备
	discoveredDevices := make(map[string]bluetooth.Device)
	scanStartTime := time.Now()

	// 监听扫描结果
	fmt.Printf("扫描中... (超时: %v)\n", scanTimeout)
	fmt.Println("发现的设备:")
	fmt.Println("----------------------------------------")

scanLoop:
	for {
		select {
		case device, ok := <-deviceChan:
			if !ok {
				fmt.Println("扫描完成")
				break scanLoop
			}

			// 检查是否是新设备或设备信息有更新
			if existingDevice, exists := discoveredDevices[device.Address]; !exists ||
				existingDevice.Name != device.Name ||
				existingDevice.RSSI != device.RSSI {

				discoveredDevices[device.Address] = device

				fmt.Printf("设备: %s\n", device.Name)
				fmt.Printf("  地址: %s\n", device.Address)
				fmt.Printf("  信号强度: %d dBm\n", device.RSSI)
				fmt.Printf("  服务UUID: %v\n", device.ServiceUUIDs)
				fmt.Printf("  最后发现: %s\n", device.LastSeen.Format("15:04:05"))

				// 显示设备能力信息
				if device.Capabilities.SupportsEncryption {
					fmt.Printf("  支持加密: 是\n")
				}
				if len(device.Capabilities.SupportedProtocols) > 0 {
					fmt.Printf("  支持协议: %v\n", device.Capabilities.SupportedProtocols)
				}

				fmt.Println("----------------------------------------")
			}

		case <-time.After(scanTimeout):
			fmt.Println("扫描超时")
			break scanLoop

		case <-ctx.Done():
			fmt.Println("扫描被取消")
			break scanLoop
		}
	}

	// 停止扫描
	if err := btAdapter.StopScan(); err != nil {
		log.Printf("停止扫描时出错: %v", err)
	}

	// 显示扫描统计信息
	scanDuration := time.Since(scanStartTime)
	fmt.Printf("\n扫描统计:\n")
	fmt.Printf("  扫描时长: %v\n", scanDuration.Round(time.Second))
	fmt.Printf("  发现设备数: %d\n", len(discoveredDevices))

	// 显示设备名称获取情况
	namedDevices := 0
	unknownDevices := 0
	generatedNames := 0

	for _, device := range discoveredDevices {
		if device.Name == "Unknown Device" {
			unknownDevices++
		} else if isGeneratedName(device.Name) {
			generatedNames++
		} else {
			namedDevices++
		}
	}

	fmt.Printf("  有真实名称的设备: %d\n", namedDevices)
	fmt.Printf("  生成名称的设备: %d\n", generatedNames)
	fmt.Printf("  未知名称的设备: %d\n", unknownDevices)

	if unknownDevices > 0 {
		fmt.Println("\n关于设备名称为空的说明:")
		fmt.Println("1. 许多蓝牙设备为了节省电量，不会在广播包中包含设备名称")
		fmt.Println("2. 某些设备只有在连接后才会提供完整的设备信息")
		fmt.Println("3. iOS 设备通常不会广播设备名称以保护隐私")
		fmt.Println("4. 可以尝试连接到设备来获取更多信息")
	}

	fmt.Println("\n扫描完成!")
}

// isGeneratedName 判断是否是生成的设备名称
func isGeneratedName(name string) bool {
	if name == "Unknown Device" {
		return true
	}

	// 检查是否是制造商生成的名称格式
	if len(name) > 7 && name[len(name)-7:] == " Device" {
		return true
	}

	// 检查是否是地址生成的名称格式
	if len(name) > 7 && name[:7] == "Device_" {
		return true
	}

	return false
}
