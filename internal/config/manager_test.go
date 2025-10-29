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

// TestConfigManager_LoadConfig 测试配置加载功能
func TestConfigManager_LoadConfig(t *testing.T) {
	logger := &testLogger{}
	manager := NewConfigManager(logger)

	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.json")

	// 测试用例1: 加载不存在的配置文件（应该返回默认配置）
	t.Run("LoadNonExistentConfig", func(t *testing.T) {
		config, err := manager.LoadConfig(configPath)
		if err != nil {
			t.Fatalf("加载不存在的配置文件失败: %v", err)
		}
		if config == nil {
			t.Fatal("配置不应该为空")
		}
		// 验证是否为默认配置
		defaultConfig := bluetooth.DefaultConfig()
		if config.ServerConfig.ServiceUUID != defaultConfig.ServerConfig.ServiceUUID {
			t.Errorf("期望默认ServiceUUID %s, 实际得到 %s",
				defaultConfig.ServerConfig.ServiceUUID, config.ServerConfig.ServiceUUID)
		}
	})

	// 测试用例2: 加载有效的配置文件
	t.Run("LoadValidConfig", func(t *testing.T) {
		// 创建测试配置
		testConfig := bluetooth.DefaultConfig()
		testConfig.ServerConfig.ServiceName = "Test Service"
		testConfig.ClientConfig.RetryAttempts = 5

		// 保存配置到文件
		data, err := json.MarshalIndent(testConfig, "", "  ")
		if err != nil {
			t.Fatalf("序列化配置失败: %v", err)
		}
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			t.Fatalf("写入配置文件失败: %v", err)
		}

		// 加载配置
		config, err := manager.LoadConfig(configPath)
		if err != nil {
			t.Fatalf("加载配置文件失败: %v", err)
		}

		// 验证配置内容
		if config.ServerConfig.ServiceName != "Test Service" {
			t.Errorf("期望ServiceName 'Test Service', 实际得到 '%s'",
				config.ServerConfig.ServiceName)
		}
		if config.ClientConfig.RetryAttempts != 5 {
			t.Errorf("期望RetryAttempts 5, 实际得到 %d",
				config.ClientConfig.RetryAttempts)
		}
	})

	// 测试用例3: 加载无效的JSON文件
	t.Run("LoadInvalidJSON", func(t *testing.T) {
		invalidPath := filepath.Join(tempDir, "invalid.json")
		if err := os.WriteFile(invalidPath, []byte("invalid json"), 0644); err != nil {
			t.Fatalf("写入无效JSON文件失败: %v", err)
		}

		_, err := manager.LoadConfig(invalidPath)
		if err == nil {
			t.Fatal("期望加载无效JSON文件时返回错误")
		}
	})
}

// TestConfigManager_SaveConfig 测试配置保存功能
func TestConfigManager_SaveConfig(t *testing.T) {
	logger := &testLogger{}
	manager := NewConfigManager(logger)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "save_test.json")

	// 测试用例1: 保存有效配置
	t.Run("SaveValidConfig", func(t *testing.T) {
		config := bluetooth.DefaultConfig()
		config.ServerConfig.ServiceName = "Saved Service"

		err := manager.SaveConfig(configPath, &config)
		if err != nil {
			t.Fatalf("保存配置失败: %v", err)
		}

		// 验证文件是否存在
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Fatal("配置文件未创建")
		}

		// 重新加载并验证内容
		loadedConfig, err := manager.LoadConfig(configPath)
		if err != nil {
			t.Fatalf("重新加载配置失败: %v", err)
		}

		if loadedConfig.ServerConfig.ServiceName != "Saved Service" {
			t.Errorf("期望ServiceName 'Saved Service', 实际得到 '%s'",
				loadedConfig.ServerConfig.ServiceName)
		}
	})

	// 测试用例2: 保存无效配置
	t.Run("SaveInvalidConfig", func(t *testing.T) {
		invalidConfig := bluetooth.DefaultConfig()
		invalidConfig.ServerConfig.MaxConnections = -1 // 无效值

		err := manager.SaveConfig(configPath, &invalidConfig)
		if err == nil {
			t.Fatal("期望保存无效配置时返回错误")
		}
	})
}

// TestConfigManager_ValidateConfig 测试配置验证功能
func TestConfigManager_ValidateConfig(t *testing.T) {
	logger := &testLogger{}
	manager := NewConfigManager(logger)

	// 测试用例1: 验证有效配置
	t.Run("ValidateValidConfig", func(t *testing.T) {
		config := bluetooth.DefaultConfig()
		err := manager.ValidateConfig(&config)
		if err != nil {
			t.Fatalf("验证有效配置失败: %v", err)
		}
	})

	// 测试用例2: 验证无效配置
	t.Run("ValidateInvalidConfig", func(t *testing.T) {
		config := bluetooth.DefaultConfig()
		config.ServerConfig.MaxConnections = 0 // 无效值

		err := manager.ValidateConfig(&config)
		if err == nil {
			t.Fatal("期望验证无效配置时返回错误")
		}
	})

	// 测试用例3: 验证空配置
	t.Run("ValidateNilConfig", func(t *testing.T) {
		err := manager.ValidateConfig(nil)
		if err == nil {
			t.Fatal("期望验证空配置时返回错误")
		}
	})
}

// TestConfigManager_WatchConfig 测试配置文件监控功能
func TestConfigManager_WatchConfig(t *testing.T) {
	logger := &testLogger{}
	manager := NewConfigManager(logger)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "watch_test.json")

	// 创建初始配置文件
	initialConfig := bluetooth.DefaultConfig()
	initialConfig.ServerConfig.ServiceName = "Initial Service"
	data, _ := json.MarshalIndent(initialConfig, "", "  ")
	os.WriteFile(configPath, data, 0644)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 开始监控配置文件
	configChan, err := manager.WatchConfig(ctx, configPath)
	if err != nil {
		t.Fatalf("开始监控配置文件失败: %v", err)
	}

	// 修改配置文件
	go func() {
		time.Sleep(100 * time.Millisecond)

		updatedConfig := bluetooth.DefaultConfig()
		updatedConfig.ServerConfig.ServiceName = "Updated Service"
		data, _ := json.MarshalIndent(updatedConfig, "", "  ")
		os.WriteFile(configPath, data, 0644)
	}()

	// 等待配置变更通知
	select {
	case newConfig := <-configChan:
		if newConfig == nil {
			t.Fatal("接收到空配置")
		}
		if newConfig.ServerConfig.ServiceName != "Updated Service" {
			t.Errorf("期望ServiceName 'Updated Service', 实际得到 '%s'",
				newConfig.ServerConfig.ServiceName)
		}
	case <-ctx.Done():
		t.Fatal("等待配置变更通知超时")
	}
}

// TestConfigManager_UpdateConfig 测试配置更新功能
func TestConfigManager_UpdateConfig(t *testing.T) {
	logger := &testLogger{}
	manager := NewConfigManager(logger)

	// 注册配置变更回调
	var callbackCalled bool

	callback := func(oldConfig, newConfig *bluetooth.BluetoothConfig) error {
		callbackCalled = true
		return nil
	}

	manager.RegisterCallback(callback)

	// 测试用例1: 更新有效配置
	t.Run("UpdateValidConfig", func(t *testing.T) {
		newConfig := bluetooth.DefaultConfig()
		newConfig.ServerConfig.ServiceName = "Updated Service"

		err := manager.UpdateConfig(&newConfig)
		if err != nil {
			t.Fatalf("更新配置失败: %v", err)
		}

		// 验证回调是否被调用
		if !callbackCalled {
			t.Fatal("配置变更回调未被调用")
		}

		// 验证当前配置是否更新
		currentConfig := manager.GetCurrentConfig()
		if currentConfig.ServerConfig.ServiceName != "Updated Service" {
			t.Errorf("期望ServiceName 'Updated Service', 实际得到 '%s'",
				currentConfig.ServerConfig.ServiceName)
		}
	})

	// 测试用例2: 更新无效配置
	t.Run("UpdateInvalidConfig", func(t *testing.T) {
		invalidConfig := bluetooth.DefaultConfig()
		invalidConfig.ServerConfig.MaxConnections = -1

		err := manager.UpdateConfig(&invalidConfig)
		if err == nil {
			t.Fatal("期望更新无效配置时返回错误")
		}
	})
}

// TestConfigManager_Callbacks 测试配置变更回调功能
func TestConfigManager_Callbacks(t *testing.T) {
	logger := &testLogger{}
	manager := NewConfigManager(logger)

	var callback1Called, callback2Called bool

	callback1 := func(oldConfig, newConfig *bluetooth.BluetoothConfig) error {
		callback1Called = true
		return nil
	}

	callback2 := func(oldConfig, newConfig *bluetooth.BluetoothConfig) error {
		callback2Called = true
		return nil
	}

	// 注册回调
	manager.RegisterCallback(callback1)
	manager.RegisterCallback(callback2)

	// 更新配置
	config := bluetooth.DefaultConfig()
	manager.UpdateConfig(&config)

	// 验证所有回调都被调用
	if !callback1Called {
		t.Error("回调1未被调用")
	}
	if !callback2Called {
		t.Error("回调2未被调用")
	}

	// 重置标志
	callback1Called = false
	callback2Called = false

	// 取消注册回调1
	manager.UnregisterCallback(callback1)

	// 再次更新配置
	config.ServerConfig.ServiceName = "Test"
	manager.UpdateConfig(&config)

	// 验证只有回调2被调用
	if callback1Called {
		t.Error("回调1不应该被调用")
	}
	if !callback2Called {
		t.Error("回调2应该被调用")
	}
}

// testLogger 测试用日志实现
type testLogger struct {
	logs []string
}

func (l *testLogger) Info(msg string, args ...interface{}) {
	l.logs = append(l.logs, "INFO: "+msg)
}

func (l *testLogger) Warn(msg string, args ...interface{}) {
	l.logs = append(l.logs, "WARN: "+msg)
}

func (l *testLogger) Error(msg string, args ...interface{}) {
	l.logs = append(l.logs, "ERROR: "+msg)
}

func (l *testLogger) Debug(msg string, args ...interface{}) {
	l.logs = append(l.logs, "DEBUG: "+msg)
}
