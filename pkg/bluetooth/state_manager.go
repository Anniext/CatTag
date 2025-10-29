package bluetooth

import (
	"fmt"
	"time"
)

// NewComponentStateManager 创建新的组件状态管理器
func NewComponentStateManager() *ComponentStateManager {
	return &ComponentStateManager{
		states:          make(map[string]ComponentStatus),
		stateHistory:    make(map[string][]StateChange),
		stateCallbacks:  make(map[ComponentStatus][]StateCallback),
		transitionRules: make(map[ComponentStatus][]ComponentStatus),
	}
}

// SetState 设置组件状态
func (csm *ComponentStateManager) SetState(component string, status ComponentStatus) error {
	csm.mu.Lock()
	defer csm.mu.Unlock()

	oldStatus, exists := csm.states[component]
	if exists && oldStatus == status {
		return nil // 状态未变化
	}

	// 检查状态转换是否合法
	if exists && !csm.isValidTransition(oldStatus, status) {
		return fmt.Errorf("非法状态转换: %s -> %s", oldStatus, status)
	}

	// 记录状态变更
	change := StateChange{
		Component:  component,
		FromStatus: oldStatus,
		ToStatus:   status,
		Timestamp:  time.Now(),
		Reason:     "状态更新",
	}

	// 更新状态
	csm.states[component] = status

	// 记录历史
	csm.stateHistory[component] = append(csm.stateHistory[component], change)

	// 触发回调
	csm.triggerStateCallbacks(status, change)

	return nil
}

// GetState 获取组件状态
func (csm *ComponentStateManager) GetState(component string) (ComponentStatus, error) {
	csm.mu.RLock()
	defer csm.mu.RUnlock()

	status, exists := csm.states[component]
	if !exists {
		return StatusStopped, fmt.Errorf("组件 %s 不存在", component)
	}

	return status, nil
}

// GetAllStates 获取所有组件状态
func (csm *ComponentStateManager) GetAllStates() map[string]ComponentStatus {
	csm.mu.RLock()
	defer csm.mu.RUnlock()

	states := make(map[string]ComponentStatus)
	for component, status := range csm.states {
		states[component] = status
	}

	return states
}

// GetStateHistory 获取组件状态历史
func (csm *ComponentStateManager) GetStateHistory(component string) []StateChange {
	csm.mu.RLock()
	defer csm.mu.RUnlock()

	history, exists := csm.stateHistory[component]
	if !exists {
		return []StateChange{}
	}

	// 返回副本
	result := make([]StateChange, len(history))
	copy(result, history)
	return result
}

// RegisterStateCallback 注册状态回调
func (csm *ComponentStateManager) RegisterStateCallback(status ComponentStatus, callback StateCallback) {
	csm.mu.Lock()
	defer csm.mu.Unlock()

	csm.stateCallbacks[status] = append(csm.stateCallbacks[status], callback)
}

// SetTransitionRule 设置状态转换规则
func (csm *ComponentStateManager) SetTransitionRule(from ComponentStatus, allowedTo []ComponentStatus) {
	csm.mu.Lock()
	defer csm.mu.Unlock()

	csm.transitionRules[from] = allowedTo
}

// isValidTransition 检查状态转换是否合法
func (csm *ComponentStateManager) isValidTransition(from, to ComponentStatus) bool {
	allowedStates, exists := csm.transitionRules[from]
	if !exists {
		// 如果没有定义规则，允许所有转换
		return true
	}

	for _, allowed := range allowedStates {
		if allowed == to {
			return true
		}
	}

	return false
}

// triggerStateCallbacks 触发状态回调
func (csm *ComponentStateManager) triggerStateCallbacks(status ComponentStatus, change StateChange) {
	callbacks, exists := csm.stateCallbacks[status]
	if !exists {
		return
	}

	for _, callback := range callbacks {
		go func(cb StateCallback) {
			if err := cb(change); err != nil {
				// 记录回调错误，但不影响状态变更
				// 在实际实现中可能需要更复杂的错误处理
			}
		}(callback)
	}
}

// InitializeDefaultTransitionRules 初始化默认状态转换规则
func (csm *ComponentStateManager) InitializeDefaultTransitionRules() {
	csm.SetTransitionRule(StatusStopped, []ComponentStatus{StatusStarting})
	csm.SetTransitionRule(StatusStarting, []ComponentStatus{StatusRunning, StatusError})
	csm.SetTransitionRule(StatusRunning, []ComponentStatus{StatusStopping, StatusError})
	csm.SetTransitionRule(StatusStopping, []ComponentStatus{StatusStopped, StatusError})
	csm.SetTransitionRule(StatusError, []ComponentStatus{StatusStopped, StatusStarting})
}
