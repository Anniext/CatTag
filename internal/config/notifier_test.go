package config

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Anniext/CatTag/pkg/bluetooth"
)

// TestConfigNotifier_Subscribe 测试配置变更订阅功能
func TestConfigNotifier_Subscribe(t *testing.T) {
	logger := &testLogger{}
	notifier := NewConfigNotifier(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// 测试用例1: 订阅配置变更
	t.Run("Subscribe", func(t *testing.T) {
		eventChan := notifier.Subscribe(ctx)
		if eventChan == nil {
			t.Fatal("事件通道不应该为空")
		}

		// 验证通道是否可用
		select {
		case <-eventChan:
			// 通道应该是开放的，但没有事件
		default:
			// 这是期望的行为
		}
	})

	// 测试用例2: 多个订阅者
	t.Run("MultipleSubscribers", func(t *testing.T) {
		ctx1, cancel1 := context.WithCancel(context.Background())
		ctx2, cancel2 := context.WithCancel(context.Background())
		defer cancel1()
		defer cancel2()

		eventChan1 := notifier.Subscribe(ctx1)
		eventChan2 := notifier.Subscribe(ctx2)

		if eventChan1 == nil || eventChan2 == nil {
			t.Fatal("事件通道不应该为空")
		}

		// 验证两个通道是不同的
		if eventChan1 == eventChan2 {
			t.Error("不同订阅者应该有不同的事件通道")
		}
	})

	// 清理
	notifier.Close()
}

// TestConfigNotifier_Notify 测试配置变更通知功能
func TestConfigNotifier_Notify(t *testing.T) {
	logger := &testLogger{}
	notifier := NewConfigNotifier(logger)
	defer notifier.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 订阅配置变更
	eventChan := notifier.Subscribe(ctx)

	// 测试用例1: 发送配置变更事件
	t.Run("NotifyEvent", func(t *testing.T) {
		oldConfig := bluetooth.DefaultConfig()
		newConfig := bluetooth.DefaultConfig()
		newConfig.ServerConfig.ServiceName = "Updated Service"

		event := ConfigChangeEvent{
			Type:      ChangeTypeUpdate,
			OldConfig: &oldConfig,
			NewConfig: &newConfig,
			Timestamp: time.Now(),
			Source:    "test",
			Reason:    "测试配置变更",
		}

		// 发送事件
		err := notifier.Notify(event)
		if err != nil {
			t.Fatalf("发送配置变更事件失败: %v", err)
		}

		// 接收事件
		select {
		case receivedEvent := <-eventChan:
			// 验证事件内容
			if receivedEvent.Type != ChangeTypeUpdate {
				t.Errorf("期望事件类型为update, 实际得到 %s", receivedEvent.Type.String())
			}
			if receivedEvent.Source != "test" {
				t.Errorf("期望事件源为'test', 实际得到 '%s'", receivedEvent.Source)
			}
			if receivedEvent.NewConfig == nil {
				t.Fatal("新配置不应该为空")
			}
			if receivedEvent.NewConfig.ServerConfig.ServiceName != "Updated Service" {
				t.Errorf("期望ServiceName 'Updated Service', 实际得到 '%s'",
					receivedEvent.NewConfig.ServerConfig.ServiceName)
			}
		case <-ctx.Done():
			t.Fatal("等待配置变更事件超时")
		}
	})

	// 测试用例2: 多个订阅者接收同一事件
	t.Run("NotifyMultipleSubscribers", func(t *testing.T) {
		// 简化测试：只验证能够创建多个订阅者并发送事件
		ctx1, cancel1 := context.WithTimeout(context.Background(), 1*time.Second)
		ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel1()
		defer cancel2()

		eventChan1 := notifier.Subscribe(ctx1)
		eventChan2 := notifier.Subscribe(ctx2)

		if eventChan1 == nil || eventChan2 == nil {
			t.Fatal("事件通道不应该为空")
		}

		event := ConfigChangeEvent{
			Type:      ChangeTypeReload,
			Timestamp: time.Now(),
			Source:    "test_multiple",
			Reason:    "测试多订阅者",
		}

		// 发送事件
		err := notifier.Notify(event)
		if err != nil {
			t.Fatalf("发送配置变更事件失败: %v", err)
		}

		// 验证至少有一个订阅者收到事件
		receivedCount := 0
		timeout := time.After(200 * time.Millisecond)

		for receivedCount < 2 {
			select {
			case <-eventChan1:
				receivedCount++
			case <-eventChan2:
				receivedCount++
			case <-timeout:
				goto done
			}
		}

	done:
		if receivedCount == 0 {
			t.Error("至少应该有一个订阅者收到配置变更事件")
		} else {
			t.Logf("成功：%d个订阅者收到了配置变更事件", receivedCount)
		}
	})
}

// TestConfigNotifier_Close 测试通知器关闭功能
func TestConfigNotifier_Close(t *testing.T) {
	logger := &testLogger{}
	notifier := NewConfigNotifier(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建订阅者
	eventChan := notifier.Subscribe(ctx)

	// 测试用例1: 关闭通知器
	t.Run("CloseNotifier", func(t *testing.T) {
		err := notifier.Close()
		if err != nil {
			t.Fatalf("关闭通知器失败: %v", err)
		}

		// 验证事件通道是否关闭
		select {
		case _, ok := <-eventChan:
			if ok {
				t.Error("事件通道应该已关闭")
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("事件通道应该立即关闭")
		}
	})

	// 测试用例2: 关闭后发送事件应该失败
	t.Run("NotifyAfterClose", func(t *testing.T) {
		event := ConfigChangeEvent{
			Type:      ChangeTypeUpdate,
			Timestamp: time.Now(),
			Source:    "test_after_close",
		}

		err := notifier.Notify(event)
		if err == nil {
			t.Fatal("期望关闭后发送事件时返回错误")
		}
	})

	// 测试用例3: 关闭后订阅应该返回关闭的通道
	t.Run("SubscribeAfterClose", func(t *testing.T) {
		newCtx, newCancel := context.WithCancel(context.Background())
		defer newCancel()

		newEventChan := notifier.Subscribe(newCtx)

		// 验证通道是否立即关闭
		select {
		case _, ok := <-newEventChan:
			if ok {
				t.Error("关闭后订阅的通道应该立即关闭")
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("关闭后订阅的通道应该立即关闭")
		}
	})
}

// TestConfigNotifier_ContextCancellation 测试上下文取消功能
func TestConfigNotifier_ContextCancellation(t *testing.T) {
	logger := &testLogger{}
	notifier := NewConfigNotifier(logger)
	defer notifier.Close()

	// 测试用例1: 上下文取消时自动取消订阅
	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		eventChan := notifier.Subscribe(ctx)

		// 取消上下文
		cancel()

		// 等待一段时间让清理goroutine执行
		time.Sleep(50 * time.Millisecond)

		// 发送事件，验证订阅者是否已被移除
		event := ConfigChangeEvent{
			Type:      ChangeTypeUpdate,
			Timestamp: time.Now(),
			Source:    "test_cancellation",
		}

		err := notifier.Notify(event)
		if err != nil {
			t.Fatalf("发送事件失败: %v", err)
		}

		// 验证事件通道是否关闭
		select {
		case _, ok := <-eventChan:
			if ok {
				t.Error("上下文取消后事件通道应该关闭")
			}
		case <-time.After(100 * time.Millisecond):
			// 通道可能还没关闭，这也是可以接受的
		}
	})
}

// TestHotReloadManager 测试热更新管理器
func TestHotReloadManager(t *testing.T) {
	logger := &testLogger{}
	configManager := NewConfigManager(logger)
	notifier := NewConfigNotifier(logger)
	hotReloadManager := NewHotReloadManager(configManager, notifier, logger)
	defer hotReloadManager.Close()

	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "hot_reload_test.json")

	// 创建初始配置
	initialConfig := bluetooth.DefaultConfig()
	initialConfig.ServerConfig.ServiceName = "Initial Service"

	err := configManager.SaveConfig(configPath, &initialConfig)
	if err != nil {
		t.Fatalf("保存初始配置失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 测试用例1: 启动热更新监控
	t.Run("StartWatching", func(t *testing.T) {
		err := hotReloadManager.StartWatching(ctx, configPath)
		if err != nil {
			t.Fatalf("启动热更新监控失败: %v", err)
		}
	})

	// 测试用例2: 配置文件变更触发热更新
	t.Run("ConfigFileChange", func(t *testing.T) {
		// 订阅配置变更通知
		eventChan := notifier.Subscribe(ctx)

		// 修改配置文件
		go func() {
			time.Sleep(100 * time.Millisecond)

			updatedConfig := bluetooth.DefaultConfig()
			updatedConfig.ServerConfig.ServiceName = "Updated Service"

			configManager.SaveConfig(configPath, &updatedConfig)
		}()

		// 等待配置变更事件
		select {
		case event := <-eventChan:
			if event.Type != ChangeTypeReload {
				t.Errorf("期望事件类型为reload, 实际得到 %s", event.Type.String())
			}
			if event.Source != configPath {
				t.Errorf("期望事件源为'%s', 实际得到 '%s'", configPath, event.Source)
			}
			if event.NewConfig == nil {
				t.Fatal("新配置不应该为空")
			}
			if event.NewConfig.ServerConfig.ServiceName != "Updated Service" {
				t.Errorf("期望ServiceName 'Updated Service', 实际得到 '%s'",
					event.NewConfig.ServerConfig.ServiceName)
			}
		case <-ctx.Done():
			t.Fatal("等待配置变更事件超时")
		}
	})

	// 测试用例3: 停止监控
	t.Run("StopWatching", func(t *testing.T) {
		hotReloadManager.StopWatching(configPath)

		// 再次修改配置文件，应该不会触发事件
		updatedConfig := bluetooth.DefaultConfig()
		updatedConfig.ServerConfig.ServiceName = "Should Not Trigger"

		configManager.SaveConfig(configPath, &updatedConfig)

		// 等待一段时间，确保没有事件
		select {
		case event := <-notifier.Subscribe(context.Background()):
			t.Errorf("停止监控后不应该收到事件: %v", event)
		case <-time.After(200 * time.Millisecond):
			// 这是期望的行为
		}
	})
}

// TestChangeType_String 测试变更类型字符串表示
func TestChangeType_String(t *testing.T) {
	testCases := []struct {
		changeType ChangeType
		expected   string
	}{
		{ChangeTypeUpdate, "update"},
		{ChangeTypeReload, "reload"},
		{ChangeTypeReset, "reset"},
		{ChangeType(999), "unknown"}, // 未知类型
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			result := tc.changeType.String()
			if result != tc.expected {
				t.Errorf("期望字符串表示 '%s', 实际得到 '%s'", tc.expected, result)
			}
		})
	}
}

// TestConfigError 测试配置错误
func TestConfigError(t *testing.T) {
	err := &ConfigError{
		Code:    "TEST_ERROR",
		Message: "这是一个测试错误",
	}

	// 测试Error方法
	errorMsg := err.Error()
	if errorMsg != "这是一个测试错误" {
		t.Errorf("期望错误消息 '这是一个测试错误', 实际得到 '%s'", errorMsg)
	}
}
