package bluetooth

import (
	"fmt"
	"time"
)

// NewComponentMonitorManager 创建新的组件监控管理器
func NewComponentMonitorManager() *ComponentMonitorManager {
	return &ComponentMonitorManager{
		monitors:         make(map[string]*ComponentMonitor),
		alertRules:       make([]AlertRule, 0),
		alertHandlers:    make([]AlertHandler, 0),
		metricsCollector: NewMetricsCollector(),
	}
}

// AddMonitor 添加组件监控器
func (cmm *ComponentMonitorManager) AddMonitor(componentName string, monitor *ComponentMonitor) {
	cmm.mu.Lock()
	defer cmm.mu.Unlock()
	cmm.monitors[componentName] = monitor
}

// RemoveMonitor 移除组件监控器
func (cmm *ComponentMonitorManager) RemoveMonitor(componentName string) {
	cmm.mu.Lock()
	defer cmm.mu.Unlock()
	delete(cmm.monitors, componentName)
}

// AddAlertRule 添加告警规则
func (cmm *ComponentMonitorManager) AddAlertRule(rule AlertRule) {
	cmm.mu.Lock()
	defer cmm.mu.Unlock()
	cmm.alertRules = append(cmm.alertRules, rule)
}

// AddAlertHandler 添加告警处理器
func (cmm *ComponentMonitorManager) AddAlertHandler(handler AlertHandler) {
	cmm.mu.Lock()
	defer cmm.mu.Unlock()
	cmm.alertHandlers = append(cmm.alertHandlers, handler)
}

// CollectMetrics 收集指标
func (cmm *ComponentMonitorManager) CollectMetrics() {
	cmm.mu.RLock()
	monitors := make(map[string]*ComponentMonitor)
	for k, v := range cmm.monitors {
		monitors[k] = v
	}
	cmm.mu.RUnlock()

	for componentName, monitor := range monitors {
		cmm.collectComponentMetrics(componentName, monitor)
	}
}

// CheckAlerts 检查告警
func (cmm *ComponentMonitorManager) CheckAlerts() {
	cmm.mu.RLock()
	rules := make([]AlertRule, len(cmm.alertRules))
	copy(rules, cmm.alertRules)
	cmm.mu.RUnlock()

	metrics := cmm.metricsCollector.GetAllMetrics()

	for componentName, componentMetrics := range metrics {
		for _, rule := range rules {
			cmm.checkAlertRule(componentName, componentMetrics, rule)
		}
	}
}

// collectComponentMetrics 收集组件指标
func (cmm *ComponentMonitorManager) collectComponentMetrics(componentName string, monitor *ComponentMonitor) {
	// 执行性能检查
	for _, perfCheck := range monitor.PerformanceChecks {
		metrics := perfCheck(nil) // 简化实现，实际需要传入组件实例
		cmm.metricsCollector.UpdateMetrics(componentName, metrics)
	}

	// 更新监控器状态
	monitor.LastCheck = time.Now()
	monitor.Status = MonitorStatusHealthy // 简化实现
}

// checkAlertRule 检查告警规则
func (cmm *ComponentMonitorManager) checkAlertRule(componentName string, metrics ComponentMetrics, rule AlertRule) {
	// 检查冷却时间
	if time.Since(rule.LastTriggered) < rule.Cooldown {
		return
	}

	// 检查告警条件
	if rule.Condition(metrics) {
		alert := Alert{
			Rule:      rule,
			Component: componentName,
			Message:   fmt.Sprintf("组件 %s 触发告警规则 %s", componentName, rule.Name),
			Severity:  rule.Severity,
			Timestamp: time.Now(),
			Metrics:   metrics,
			Metadata:  make(map[string]interface{}),
		}

		// 触发告警处理器
		cmm.triggerAlertHandlers(alert)

		// 更新规则触发时间
		cmm.updateRuleLastTriggered(rule.Name)
	}
}

// triggerAlertHandlers 触发告警处理器
func (cmm *ComponentMonitorManager) triggerAlertHandlers(alert Alert) {
	cmm.mu.RLock()
	handlers := make([]AlertHandler, len(cmm.alertHandlers))
	copy(handlers, cmm.alertHandlers)
	cmm.mu.RUnlock()

	for _, handler := range handlers {
		go func(h AlertHandler) {
			if err := h.Handle(alert); err != nil {
				// 记录告警处理错误
			}
		}(handler)
	}
}

// updateRuleLastTriggered 更新规则最后触发时间
func (cmm *ComponentMonitorManager) updateRuleLastTriggered(ruleName string) {
	cmm.mu.Lock()
	defer cmm.mu.Unlock()

	for i, rule := range cmm.alertRules {
		if rule.Name == ruleName {
			cmm.alertRules[i].LastTriggered = time.Now()
			break
		}
	}
}

// GetMonitorStatus 获取监控状态
func (cmm *ComponentMonitorManager) GetMonitorStatus() map[string]MonitorStatus {
	cmm.mu.RLock()
	defer cmm.mu.RUnlock()

	status := make(map[string]MonitorStatus)
	for componentName, monitor := range cmm.monitors {
		status[componentName] = monitor.Status
	}

	return status
}

// NewMetricsCollector 创建新的指标收集器
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics:    make(map[string]ComponentMetrics),
		collectors: make(map[string]MetricsCollectorFunc),
		interval:   30 * time.Second,
	}
}

// UpdateMetrics 更新指标
func (mc *MetricsCollector) UpdateMetrics(component string, metrics ComponentMetrics) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.metrics[component] = metrics
}

// GetMetrics 获取组件指标
func (mc *MetricsCollector) GetMetrics(component string) (ComponentMetrics, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	metrics, exists := mc.metrics[component]
	return metrics, exists
}

// GetAllMetrics 获取所有指标
func (mc *MetricsCollector) GetAllMetrics() map[string]ComponentMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make(map[string]ComponentMetrics)
	for component, metrics := range mc.metrics {
		result[component] = metrics
	}

	return result
}

// RegisterCollector 注册指标收集器
func (mc *MetricsCollector) RegisterCollector(component string, collector MetricsCollectorFunc) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.collectors[component] = collector
}

// CollectAll 收集所有指标
func (mc *MetricsCollector) CollectAll() {
	mc.mu.RLock()
	collectors := make(map[string]MetricsCollectorFunc)
	for k, v := range mc.collectors {
		collectors[k] = v
	}
	mc.mu.RUnlock()

	for component, collector := range collectors {
		metrics := collector()
		mc.UpdateMetrics(component, metrics)
	}
}

// 预定义的告警条件

// HighErrorRateCondition 高错误率条件
func HighErrorRateCondition(threshold float64) AlertCondition {
	return func(metrics ComponentMetrics) bool {
		if metrics.MessagesSent == 0 {
			return false
		}
		errorRate := float64(metrics.ErrorCount) / float64(metrics.MessagesSent)
		return errorRate > threshold
	}
}

// HighMemoryUsageCondition 高内存使用率条件
func HighMemoryUsageCondition(threshold uint64) AlertCondition {
	return func(metrics ComponentMetrics) bool {
		return metrics.MemoryUsage > threshold
	}
}

// HighCPUUsageCondition 高CPU使用率条件
func HighCPUUsageCondition(threshold float64) AlertCondition {
	return func(metrics ComponentMetrics) bool {
		return metrics.CPUUsage > threshold
	}
}

// TooManyRestartsCondition 重启次数过多条件
func TooManyRestartsCondition(threshold int) AlertCondition {
	return func(metrics ComponentMetrics) bool {
		return metrics.RestartCount > threshold
	}
}
