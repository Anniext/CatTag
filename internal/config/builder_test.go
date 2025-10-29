package config

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// TestAdvancedComponentBuilder_Basic 测试基本构建器功能
func TestAdvancedComponentBuilder_Basic(t *testing.T) {
	// 测试用例1: 创建构建器
	t.Run("CreateBuilder", func(t *testing.T) {
		builder := NewAdvancedComponentBuilder()
		if builder == nil {
			t.Fatal("构建器不应该为空")
		}

		// 验证默认配置
		config := builder.GetConfig()
		if config != nil {
			t.Error("初始配置应该为空")
		}
	})

	// 测试用例2: 设置配置
	t.Run("WithConfig", func(t *testing.T) {
		builder := NewAdvancedComponentBuilder()
		testConfig := bluetooth.DefaultConfig()
		testConfig.ServerConfig.ServiceName = "Test Service"

		builder.WithConfig(&testConfig)

		config := builder.GetConfig()
		if config == nil {
			t.Fatal("配置不应该为空")
		}
		if config.ServerConfig.ServiceName != "Test Service" {
			t.Errorf("期望ServiceName 'Test Service', 实际得到 '%s'",
				config.ServerConfig.ServiceName)
		}
	})

	// 测试用例3: 从文件加载配置
	t.Run("WithConfigFile", func(t *testing.T) {
		builder := NewAdvancedComponentBuilder()

		// 创建临时配置文件
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "test_config.json")

		testConfig := bluetooth.DefaultConfig()
		testConfig.ServerConfig.ServiceName = "File Service"

		data, _ := json.MarshalIndent(testConfig, "", "  ")
		os.WriteFile(configPath, data, 0644)

		builder.WithConfigFile(configPath)

		config := builder.GetConfig()
		if config == nil {
			t.Fatal("配置不应该为空")
		}
		if config.ServerConfig.ServiceName != "File Service" {
			t.Errorf("期望ServiceName 'File Service', 实际得到 '%s'",
				config.ServerConfig.ServiceName)
		}
	})
}

// TestAdvancedComponentBuilder_Dependencies 测试依赖注入功能
func TestAdvancedComponentBuilder_Dependencies(t *testing.T) {
	builder := NewAdvancedComponentBuilder()

	// 测试用例1: 注册依赖
	t.Run("WithDependency", func(t *testing.T) {
		testDep := "test dependency"
		builder.WithDependency("test_dep", testDep)

		// 获取依赖
		dep, err := builder.GetComponent("test_dep")
		if err != nil {
			t.Fatalf("获取依赖失败: %v", err)
		}
		if dep != testDep {
			t.Errorf("期望依赖 '%s', 实际得到 '%v'", testDep, dep)
		}
	})

	// 测试用例2: 注册依赖工厂
	t.Run("WithDependencyFactory", func(t *testing.T) {
		factoryCallCount := 0
		factory := func() (interface{}, error) {
			factoryCallCount++
			return "factory result", nil
		}

		builder.WithDependencyFactory("factory_dep", factory)

		// 第一次获取依赖
		dep1, err := builder.GetComponent("factory_dep")
		if err != nil {
			t.Fatalf("获取工厂依赖失败: %v", err)
		}
		if dep1 != "factory result" {
			t.Errorf("期望工厂结果 'factory result', 实际得到 '%v'", dep1)
		}

		// 第二次获取依赖（应该使用缓存）
		dep2, err := builder.GetComponent("factory_dep")
		if err != nil {
			t.Fatalf("第二次获取工厂依赖失败: %v", err)
		}
		if dep2 != dep1 {
			t.Error("第二次获取应该返回缓存的实例")
		}

		// 验证工厂只被调用一次
		if factoryCallCount != 1 {
			t.Errorf("期望工厂被调用1次, 实际调用了%d次", factoryCallCount)
		}
	})
}

// TestAdvancedComponentBuilder_Build 测试组件构建功能
func TestAdvancedComponentBuilder_Build(t *testing.T) {
	// 测试用例1: 构建蓝牙组件（使用默认配置）
	t.Run("BuildWithDefaultConfig", func(t *testing.T) {
		builder := NewAdvancedComponentBuilder()

		component, err := builder.Build()
		if err != nil {
			t.Fatalf("构建蓝牙组件失败: %v", err)
		}
		if component == nil {
			t.Fatal("构建的组件不应该为空")
		}

		// 验证组件是否被注册到依赖容器
		registeredComponent, err := builder.GetComponent("bluetooth_component")
		if err != nil {
			t.Fatalf("获取注册的组件失败: %v", err)
		}
		if registeredComponent != component {
			t.Error("注册的组件应该与构建的组件相同")
		}
	})

	// 测试用例2: 构建蓝牙服务端
	t.Run("BuildServer", func(t *testing.T) {
		builder := NewAdvancedComponentBuilder()
		config := bluetooth.DefaultConfig()
		builder.WithConfig(&config)

		server, err := builder.BuildServer()
		if err != nil {
			t.Fatalf("构建蓝牙服务端失败: %v", err)
		}
		if server == nil {
			t.Fatal("构建的服务端不应该为空")
		}

		// 验证服务端是否被注册
		registeredServer, err := builder.GetComponent("bluetooth_server")
		if err != nil {
			t.Fatalf("获取注册的服务端失败: %v", err)
		}
		if registeredServer != server {
			t.Error("注册的服务端应该与构建的服务端相同")
		}
	})

	// 测试用例3: 构建蓝牙客户端
	t.Run("BuildClient", func(t *testing.T) {
		builder := NewAdvancedComponentBuilder()
		config := bluetooth.DefaultConfig()
		builder.WithConfig(&config)

		client, err := builder.BuildClient()
		if err != nil {
			t.Fatalf("构建蓝牙客户端失败: %v", err)
		}
		if client == nil {
			t.Fatal("构建的客户端不应该为空")
		}

		// 验证客户端是否被注册
		registeredClient, err := builder.GetComponent("bluetooth_client")
		if err != nil {
			t.Fatalf("获取注册的客户端失败: %v", err)
		}
		if registeredClient != client {
			t.Error("注册的客户端应该与构建的客户端相同")
		}
	})

	// 测试用例4: 构建时配置验证失败
	t.Run("BuildWithInvalidConfig", func(t *testing.T) {
		builder := NewAdvancedComponentBuilder()

		// 设置无效配置
		invalidConfig := bluetooth.DefaultConfig()
		invalidConfig.ServerConfig.MaxConnections = -1
		builder.WithConfig(&invalidConfig)

		_, err := builder.Build()
		if err == nil {
			t.Fatal("期望构建无效配置时返回错误")
		}
	})
}

// TestAdvancedComponentBuilder_Lifecycle 测试组件生命周期管理
func TestAdvancedComponentBuilder_Lifecycle(t *testing.T) {
	builder := NewAdvancedComponentBuilder()
	config := bluetooth.DefaultConfig()
	builder.WithConfig(&config)

	// 构建组件
	component, err := builder.Build()
	if err != nil {
		t.Fatalf("构建组件失败: %v", err)
	}

	ctx := context.Background()

	// 测试用例1: 启动组件
	t.Run("StartComponents", func(t *testing.T) {
		err := builder.Start(ctx)
		if err != nil {
			t.Fatalf("启动组件失败: %v", err)
		}

		// 验证组件状态（这里使用模拟组件，所以状态检查可能有限）
		if mockComponent, ok := component.(*MockBluetoothComponent); ok {
			if mockComponent.status != StatusRunning {
				t.Errorf("期望组件状态为运行中, 实际状态: %v", mockComponent.status)
			}
		}
	})

	// 测试用例2: 停止组件
	t.Run("StopComponents", func(t *testing.T) {
		err := builder.Stop(ctx)
		if err != nil {
			t.Fatalf("停止组件失败: %v", err)
		}

		// 验证组件状态
		if mockComponent, ok := component.(*MockBluetoothComponent); ok {
			if mockComponent.status != StatusStopped {
				t.Errorf("期望组件状态为已停止, 实际状态: %v", mockComponent.status)
			}
		}
	})
}

// TestAdvancedComponentBuilder_ConfigCallbacks 测试配置变更回调
func TestAdvancedComponentBuilder_ConfigCallbacks(t *testing.T) {
	builder := NewAdvancedComponentBuilder()

	var callbackCalled bool
	var receivedNewConfig *bluetooth.BluetoothConfig

	callback := func(oldConfig, newConfig *bluetooth.BluetoothConfig) error {
		callbackCalled = true
		receivedNewConfig = newConfig
		return nil
	}

	// 注册配置变更回调
	builder.WithConfigCallback(callback)

	// 设置初始配置
	initialConfig := bluetooth.DefaultConfig()
	initialConfig.ServerConfig.ServiceName = "Initial"
	builder.WithConfig(&initialConfig)

	// 构建组件（这会触发配置更新）
	_, err := builder.Build()
	if err != nil {
		t.Fatalf("构建组件失败: %v", err)
	}

	// 验证回调是否被调用
	if !callbackCalled {
		t.Fatal("配置变更回调未被调用")
	}

	// 验证接收到的配置
	if receivedNewConfig == nil {
		t.Fatal("新配置不应该为空")
	}
	if receivedNewConfig.ServerConfig.ServiceName != "Initial" {
		t.Errorf("期望新配置ServiceName 'Initial', 实际得到 '%s'",
			receivedNewConfig.ServerConfig.ServiceName)
	}
}

// TestAdvancedComponentBuilder_HotReload 测试配置热更新
func TestAdvancedComponentBuilder_HotReload(t *testing.T) {
	builder := NewAdvancedComponentBuilder()

	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "hot_reload_test.json")

	// 创建初始配置
	initialConfig := bluetooth.DefaultConfig()
	initialConfig.ServerConfig.ServiceName = "Initial Service"
	data, _ := json.MarshalIndent(initialConfig, "", "  ")
	os.WriteFile(configPath, data, 0644)

	// 从文件加载配置
	builder.WithConfigFile(configPath)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 启用热更新
	builder.EnableHotReload(ctx, configPath)

	// 构建组件
	_, err := builder.Build()
	if err != nil {
		t.Fatalf("构建组件失败: %v", err)
	}

	// 订阅配置变更通知
	notifier := builder.GetNotifier()
	eventChan := notifier.Subscribe(ctx)

	// 修改配置文件
	go func() {
		time.Sleep(100 * time.Millisecond)

		updatedConfig := bluetooth.DefaultConfig()
		updatedConfig.ServerConfig.ServiceName = "Updated Service"
		data, _ := json.MarshalIndent(updatedConfig, "", "  ")
		os.WriteFile(configPath, data, 0644)
	}()

	// 等待配置变更事件
	select {
	case event := <-eventChan:
		if event.Type != ChangeTypeReload {
			t.Errorf("期望变更类型为reload, 实际得到 %s", event.Type.String())
		}
		if event.NewConfig == nil {
			t.Fatal("新配置不应该为空")
		}
		if event.NewConfig.ServerConfig.ServiceName != "Updated Service" {
			t.Errorf("期望新配置ServiceName 'Updated Service', 实际得到 '%s'",
				event.NewConfig.ServerConfig.ServiceName)
		}
	case <-ctx.Done():
		t.Fatal("等待配置变更事件超时")
	}
}

// TestAdvancedComponentBuilder_Validation 测试构建器验证功能
func TestAdvancedComponentBuilder_Validation(t *testing.T) {
	// 测试用例1: 验证有效配置
	t.Run("ValidateValidConfig", func(t *testing.T) {
		builder := NewAdvancedComponentBuilder()
		config := bluetooth.DefaultConfig()
		builder.WithConfig(&config)

		err := builder.Validate()
		if err != nil {
			t.Fatalf("验证有效配置失败: %v", err)
		}
	})

	// 测试用例2: 验证无效配置
	t.Run("ValidateInvalidConfig", func(t *testing.T) {
		builder := NewAdvancedComponentBuilder()

		invalidConfig := bluetooth.DefaultConfig()
		invalidConfig.ServerConfig.MaxConnections = -1
		builder.WithConfig(&invalidConfig)

		err := builder.Validate()
		if err == nil {
			t.Fatal("期望验证无效配置时返回错误")
		}
	})

	// 测试用例3: 验证空配置
	t.Run("ValidateNilConfig", func(t *testing.T) {
		builder := NewAdvancedComponentBuilder()

		err := builder.Validate()
		if err == nil {
			t.Fatal("期望验证空配置时返回错误")
		}
	})
}

// TestAdvancedComponentBuilder_Reset 测试构建器重置功能
func TestAdvancedComponentBuilder_Reset(t *testing.T) {
	builder := NewAdvancedComponentBuilder()

	// 设置一些状态
	config := bluetooth.DefaultConfig()
	builder.WithConfig(&config)
	builder.WithDependency("test_dep", "test value")

	callback := func(oldConfig, newConfig *bluetooth.BluetoothConfig) error {
		return nil
	}
	builder.WithConfigCallback(callback)

	// 构建组件
	_, err := builder.Build()
	if err != nil {
		t.Fatalf("构建组件失败: %v", err)
	}

	// 验证状态已设置
	if builder.GetConfig() == nil {
		t.Fatal("配置应该已设置")
	}

	// 重置构建器
	builder.Reset()

	// 验证状态已清空
	if builder.GetConfig() != nil {
		t.Error("重置后配置应该为空")
	}

	// 验证组件已清空
	_, err = builder.GetComponent("bluetooth_component")
	if err == nil {
		t.Error("重置后组件应该不存在")
	}

	// 验证依赖已清空
	_, err = builder.GetComponent("test_dep")
	if err == nil {
		t.Error("重置后依赖应该不存在")
	}
}
